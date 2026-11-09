package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

// newCommandaiAppServer 模拟 Commandai 双通道：
//   - GET /provider/v1/models（模型面，免认证可用）
//   - GET /alpha/*（额度面，Bearer 认证）
//
// quotaState: "ok" | "auth"；quotaHits 记录额度面命中次数。
func newCommandaiAppServer(quotaState string, quotaHits *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/provider/v1/models":
			json.NewEncoder(w).Encode(map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "claude-sonnet-5-5", "name": "Claude Sonnet 5.5",
						"context_length": 1000000, "supported_endpoints": []string{"/messages"}},
				},
			})
		case "/alpha/whoami":
			if quotaState == "auth" {
				w.WriteHeader(401)
				w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","status":401,` +
					`"message":"Invalid 'Authorization' header or token."}}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"org":  map[string]any{"id": "org_abc", "login": "acme"},
				"user": map[string]any{"userName": "alice"},
			})
		case "/alpha/billing/credits":
			atomic.AddInt32(quotaHits, 1)
			json.NewEncoder(w).Encode(map[string]any{"credits": map[string]any{
				"planId":           "individual-provider",
				"monthlyCredits":   10,
				"purchasedCredits": 3.2,
				"freeCredits":      0,
				"sandboxAccess":    true,
				"windowLimits": map[string]any{
					"limited":  true,
					"fiveHour": map[string]any{"used": 14.2, "cap": 14, "resetAt": 1792901766000},
				},
			}})
		case "/alpha/billing/subscriptions":
			atomic.AddInt32(quotaHits, 1)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"planId":             "individual-provider",
				"status":             "active",
				"currentPeriodStart": "2026-10-01T00:00:00Z",
				"currentPeriodEnd":   "2026-11-01T00:00:00Z",
			}})
		case "/alpha/usage/summary":
			atomic.AddInt32(quotaHits, 1)
			json.NewEncoder(w).Encode(map[string]any{
				"totalCost": 1.8, "totalCount": 42, "periodBasis": "billing-period",
			})
		default:
			w.WriteHeader(404)
		}
	}))
}

func newCommandaiApp(t *testing.T, quotaState string) (*App, *httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := newCommandaiAppServer(quotaState, &hits)
	a := New(&config.Config{})
	a.Repos.Commandai.BaseURL = srv.URL
	a.Repos.Commandai.Client = srv.Client()
	// 隔离宿主机 ~/.commandcode/auth.json：否则开发机 CLI 已登录时，
	// 「配置 key 认证失败」用例会静默走回退成功（同 hostCredentialEnv 教训）。
	a.Repos.Commandai.CLIAuthPath = filepath.Join(t.TempDir(), "auth.json")
	return a, srv, &hits
}

func TestRefreshCommandaiHappy(t *testing.T) {
	a, srv, hits := newCommandaiApp(t, "ok")
	defer srv.Close()
	cfg := &config.Config{CommandaiAccounts: []models.CommandaiAccount{{ID: "cm1", Name: "测试号", ApiKey: "test-key"}}}
	a.Cfg = cfg

	res, err := a.RefreshCommandai("cm1")
	if err != nil {
		t.Fatalf("RefreshCommandai: %v", err)
	}
	if res.Plan == nil || len(res.Plan.Models) != 1 {
		t.Errorf("Plan: %+v", res.Plan)
	}
	if res.Credits == nil || res.Credits.MonthlyCredits != 10 {
		t.Errorf("Credits: %+v", res.Credits)
	}
	if res.Subscription == nil || res.Subscription.Status != "active" {
		t.Errorf("Subscription: %+v", res.Subscription)
	}
	if res.Summary == nil || res.Summary.TotalCost != 1.8 {
		t.Errorf("Summary: %+v", res.Summary)
	}
	if res.Whoami == nil || res.Whoami.OrgID != "org_abc" {
		t.Errorf("Whoami: %+v", res.Whoami)
	}
	if res.Error != "" {
		t.Errorf("Error should be empty: %q", res.Error)
	}
	if *hits != 3 {
		t.Errorf("quota hits: got %d, want 3 (credits+sub+summary)", *hits)
	}
}

func TestRefreshCommandaiAuthSingleLine(t *testing.T) {
	a, srv, _ := newCommandaiApp(t, "auth")
	defer srv.Close()
	cfg := &config.Config{CommandaiAccounts: []models.CommandaiAccount{{ID: "cm1", Name: "测试号", ApiKey: "bad-key"}}}
	a.Cfg = cfg

	res, err := a.RefreshCommandai("cm1")
	if err != nil {
		t.Fatalf("RefreshCommandai: %v", err)
	}
	// 认证失败：Error 单行，不叠 models 错误
	if strings.Contains(res.Error, "\n") {
		t.Errorf("auth error should be single line: %q", res.Error)
	}
	if !strings.Contains(res.Error, "commandcode.ai/studio") {
		t.Errorf("auth error should point to studio: %q", res.Error)
	}
	// 模型路不受额度路影响（models 免认证）
	if res.Plan == nil || len(res.Plan.Models) != 1 {
		t.Errorf("Plan should still be fetched: %+v", res.Plan)
	}
}

// newCommandaiKeyAwareServer 只接受 validKey 的额度请求；其它 key 回 401 信封。
func newCommandaiKeyAwareServer(t *testing.T, validKey string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/provider/v1/models" {
			json.NewEncoder(w).Encode(map[string]any{"object": "list",
				"data": []map[string]any{{"id": "m1", "name": "M1"}}})
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+validKey {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","status":401,` +
				`"message":"Your API key has expired."}}`))
			return
		}
		switch r.URL.Path {
		case "/alpha/whoami":
			json.NewEncoder(w).Encode(map[string]any{
				"org": nil, "user": map[string]any{"userName": "cli-user"},
			})
		case "/alpha/billing/credits":
			json.NewEncoder(w).Encode(map[string]any{"credits": map[string]any{
				"monthlyCredits": 3.05, "purchasedCredits": 0, "freeCredits": 0,
			}})
		case "/alpha/billing/subscriptions":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"planId": "individual-go-v1", "status": "active",
				"currentPeriodEnd": "2026-11-25T04:15:41.000Z",
			}})
		case "/alpha/usage/summary":
			json.NewEncoder(w).Encode(map[string]any{"totalCost": 7.65, "totalCount": 4835})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRefreshCommandaiFallsBackToCLIAuth(t *testing.T) {
	srv := newCommandaiKeyAwareServer(t, "cli-key")
	authPath := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"apiKey":"cli-key","userId":"u1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a := New(&config.Config{CommandaiAccounts: []models.CommandaiAccount{
		{ID: "cm1", Name: "测试号", ApiKey: "expired-key"}}})
	a.Repos.Commandai.BaseURL = srv.URL
	a.Repos.Commandai.Client = srv.Client()
	a.Repos.Commandai.CLIAuthPath = authPath

	res, err := a.RefreshCommandai("cm1")
	if err != nil {
		t.Fatalf("RefreshCommandai: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("回退成功后不应有错误: %s", res.Error)
	}
	if res.QuotaKeySource != "cli" {
		t.Errorf("QuotaKeySource=%q, want cli", res.QuotaKeySource)
	}
	if res.Credits == nil || res.Credits.MonthlyCredits != 3.05 {
		t.Errorf("应拿到 CLI 凭据的额度: %+v", res.Credits)
	}
	if res.Summary == nil || res.Summary.TotalCost != 7.65 {
		t.Errorf("应拿到 CLI 凭据的本期消耗: %+v", res.Summary)
	}
	if res.Plan == nil || len(res.Plan.Models) != 1 {
		t.Errorf("模型清单不受影响: %+v", res.Plan)
	}
}

func TestRefreshCommandaiNoKeyUsesCLIAuth(t *testing.T) {
	srv := newCommandaiKeyAwareServer(t, "cli-key")
	authPath := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"apiKey":"cli-key"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a := New(&config.Config{CommandaiAccounts: []models.CommandaiAccount{
		{ID: "cm1", Name: "测试号"}}})
	a.Repos.Commandai.BaseURL = srv.URL
	a.Repos.Commandai.Client = srv.Client()
	a.Repos.Commandai.CLIAuthPath = authPath

	res, err := a.RefreshCommandai("cm1")
	if err != nil {
		t.Fatalf("RefreshCommandai: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("CLI 登录态可用时不应报错: %s", res.Error)
	}
	if res.QuotaKeySource != "cli" || res.Credits == nil {
		t.Errorf("应使用 CLI 凭据: source=%q credits=%+v", res.QuotaKeySource, res.Credits)
	}
}

func TestRefreshCommandaiNoKey(t *testing.T) {
	a, srv, hits := newCommandaiApp(t, "ok")
	defer srv.Close()
	cfg := &config.Config{CommandaiAccounts: []models.CommandaiAccount{{ID: "cm1", Name: "测试号"}}}
	a.Cfg = cfg

	res, err := a.RefreshCommandai("cm1")
	if err != nil {
		t.Fatalf("RefreshCommandai: %v", err)
	}
	// 无 key：不发额度请求
	if *hits != 0 {
		t.Errorf("quota hits: got %d, want 0", *hits)
	}
	// models 正常拉（免认证）
	if res.Plan == nil || len(res.Plan.Models) != 1 {
		t.Errorf("Plan should still be fetched: %+v", res.Plan)
	}
	// 额度不可用提示
	if !strings.Contains(res.Error, "未配置 API Key") {
		t.Errorf("Error should hint missing key: %q", res.Error)
	}
	if res.Credits != nil {
		t.Errorf("Credits should be nil: %+v", res.Credits)
	}
}

func TestRefreshCommandaiAccountNotFound(t *testing.T) {
	a, srv, _ := newCommandaiApp(t, "ok")
	defer srv.Close()
	a.Cfg = &config.Config{}
	if _, err := a.RefreshCommandai("nonexistent"); err == nil {
		t.Fatal("should error for nonexistent account")
	}
}

func TestRefreshAllIncludesCommandai(t *testing.T) {
	a, srv, _ := newCommandaiApp(t, "ok")
	defer srv.Close()
	cfg := &config.Config{CommandaiAccounts: []models.CommandaiAccount{{ID: "cm1", Name: "测试号", ApiKey: "test-key"}}}
	a.Cfg = cfg

	res, err := a.RefreshAll()
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if len(res.Commandai) != 1 {
		t.Fatalf("Commandai results: got %d, want 1", len(res.Commandai))
	}
	if res.Commandai[0].Credits == nil {
		t.Error("Commandai credits should be fetched in RefreshAll")
	}
}

func TestRefreshCommandaiQuotaPartialFailure(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/provider/v1/models":
			json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []map[string]any{{"id": "m1"}}})
		case "/alpha/whoami":
			json.NewEncoder(w).Encode(map[string]any{"org": map[string]any{"id": "org_x"}})
		case "/alpha/billing/credits":
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(500)
			w.Write([]byte(`boom`))
		case "/alpha/billing/subscriptions":
			atomic.AddInt32(&hits, 1)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"planId": "p", "status": "active"}})
		case "/alpha/usage/summary":
			atomic.AddInt32(&hits, 1)
			json.NewEncoder(w).Encode(map[string]any{"totalCost": 0, "totalCount": 0})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	a := New(&config.Config{CommandaiAccounts: []models.CommandaiAccount{{ID: "cm1", Name: "t", ApiKey: "k"}}})
	a.Repos.Commandai.BaseURL = srv.URL
	a.Repos.Commandai.Client = srv.Client()

	res, err := a.RefreshCommandai("cm1")
	if err != nil {
		t.Fatalf("RefreshCommandai: %v", err)
	}
	if res.Credits != nil {
		t.Errorf("Credits should be nil on failure: %+v", res.Credits)
	}
	if res.Subscription == nil {
		t.Error("Subscription should be returned on partial failure")
	}
	if res.Summary == nil {
		t.Error("Summary should be returned on partial failure")
	}
	if res.Error == "" {
		t.Error("partial failure should surface an error")
	}
}

func TestRefreshCommandaiRepoNil(t *testing.T) {
	// 仓库未初始化（NewWithRepos 注入空 Repos）→ 不 panic，给中文错误
	a := NewWithRepos(&config.Config{CommandaiAccounts: []models.CommandaiAccount{{ID: "cm1", Name: "t", ApiKey: "k"}}}, &Repos{})
	res := a.refreshCommandai(models.CommandaiAccount{ID: "cm1", Name: "t", ApiKey: "k"})
	if res.Error == "" {
		t.Error("nil repo should surface an error")
	}
	if !strings.Contains(res.Error, "仓库未初始化") {
		t.Errorf("error should mention repo: %q", res.Error)
	}
}
