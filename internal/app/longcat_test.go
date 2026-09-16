package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

// newLongCatAppTestServer 启动一个模拟 LongCat API 的服务器，返回 httptest.Server。
// balanceState: "ok" | "empty" | "bad"
func newLongCatAppTestServer(balanceState string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "LongCat-2.0", "owned_by": "meituan"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			switch balanceState {
			case "ok":
				w.WriteHeader(200)
				json.NewEncoder(w).Encode(map[string]any{
					"choices": []map[string]any{{"message": map[string]any{"content": "pong"}}},
				})
			case "empty":
				w.WriteHeader(402)
			case "bad":
				w.WriteHeader(401)
			}
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestRefreshLongCatHappy(t *testing.T) {
	srv := newLongCatAppTestServer("ok")
	defer srv.Close()

	cfg := &config.Config{
		LongCatAccounts: []models.LongCatAccount{
			{ID: "lc1", Name: "测试号", ApiKey: "test-key"},
		},
	}
	app := New(cfg)
	app.Repos.LongCat.BaseURL = srv.URL
	app.Repos.LongCat.Client = srv.Client()

	res, err := app.RefreshLongCat("lc1")
	if err != nil {
		t.Fatalf("RefreshLongCat: %v", err)
	}
	if res.Plan == nil {
		t.Fatal("Plan should not be nil")
	}
	if len(res.Plan.Models) != 1 {
		t.Errorf("Models: got %d, want 1", len(res.Plan.Models))
	}
	if res.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if res.Usage.BalanceOK == nil || !*res.Usage.BalanceOK {
		t.Error("BalanceOK should be true")
	}
}

func TestRefreshLongCatEmptyBalance(t *testing.T) {
	srv := newLongCatAppTestServer("empty")
	defer srv.Close()

	cfg := &config.Config{
		LongCatAccounts: []models.LongCatAccount{
			{ID: "lc1", Name: "测试号", ApiKey: "test-key"},
		},
	}
	app := New(cfg)
	app.Repos.LongCat.BaseURL = srv.URL
	app.Repos.LongCat.Client = srv.Client()

	res, err := app.RefreshLongCat("lc1")
	if err != nil {
		t.Fatalf("RefreshLongCat: %v", err)
	}
	if res.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if res.Usage.BalanceOK == nil || *res.Usage.BalanceOK {
		t.Error("BalanceOK should be false for 402")
	}
}

func TestRefreshLongCatBadKey(t *testing.T) {
	srv := newLongCatAppTestServer("bad")
	defer srv.Close()

	cfg := &config.Config{
		LongCatAccounts: []models.LongCatAccount{
			{ID: "lc1", Name: "测试号", ApiKey: "bad-key"},
		},
	}
	app := New(cfg)
	app.Repos.LongCat.BaseURL = srv.URL
	app.Repos.LongCat.Client = srv.Client()

	res, err := app.RefreshLongCat("lc1")
	if err != nil {
		t.Fatalf("RefreshLongCat: %v", err)
	}
	// 401 认证错误应该出现在 res.Error
	if res.Error == "" {
		t.Error("Error should be set for 401")
	}
}

func TestRefreshLongCatAccountNotFound(t *testing.T) {
	srv := newLongCatAppTestServer("ok")
	defer srv.Close()

	cfg := &config.Config{}
	app := New(cfg)
	app.Repos.LongCat.BaseURL = srv.URL
	app.Repos.LongCat.Client = srv.Client()

	_, err := app.RefreshLongCat("nonexistent")
	if err == nil {
		t.Fatal("should error for nonexistent account")
	}
}

func TestRefreshLongCatNoRefresh(t *testing.T) {
	// 无账号时 RefreshAll 不应 panic
	cfg := &config.Config{}
	app := New(cfg)
	res, err := app.RefreshAll()
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if len(res.LongCat) != 0 {
		t.Errorf("LongCat results: got %d, want 0", len(res.LongCat))
	}
}

// 控制台配额接口测试 fixture（2026-09-16 实测形状，假 Cookie）。
const (
	lcTokenPacksHappy = `{"code":0,"msg":"success","data":{` +
		`"currentLot":{"remainingToken":1234567,"totalToken":5000000,"consumedToken":3765433,` +
		`"consumedRatio":0.753,"expireTime":1760342400000,"remainSeconds":2332800,"grantCategory":"GIFT"},` +
		`"otherLots":[],` +
		`"estimate":{"windowDays":7,"dailyAverageToken":12345,"exhaustedAfterDays":100}}}`
	lcTokenPacksNoPack = `{"code":0,"msg":"success","data":{` +
		`"currentLot":null,"estimate":{"windowDays":7,"dailyAverageToken":0,"exhaustedAfterDays":0},"otherLots":[]}}`
	lcConsole401 = `{"code":401,"msg":"登录状态无效，请重新登录","data":null}`
	lcPaygoHappy = `{"code":0,"msg":"success","data":{` +
		`"paygoBalanceCent":0,"paygoStatus":"NORMAL","rechargeEnabled":true,` +
		`"statusTip":"账户余额已耗尽，请及时充值以确保 API 正常使用",` +
		`"paygoBalance":{"primary":{"currency":"CNY","amount":"0.00"},"secondary":null},"exchangeRate":6.8}}`
)

// newLongCatConsoleAppTestServer 模拟 LongCat 全通道（API + 控制台）。
// balanceState: "ok" | "empty" | "bad"；consoleState: "ok" | "no-pack" | "http401" | "http500"。
// consoleHits 记录控制台配额接口被请求次数（无 Cookie 时必须为 0）。
func newLongCatConsoleAppTestServer(balanceState, consoleState string, consoleHits *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "LongCat-2.0", "owned_by": "meituan", "display_name": "LongCat 2.0", "context_window": 1048576, "max_output_tokens": 131072},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions":
			switch balanceState {
			case "ok":
				w.WriteHeader(200)
				json.NewEncoder(w).Encode(map[string]any{
					"choices": []map[string]any{{"message": map[string]any{"content": "pong"}}},
				})
			case "empty":
				w.WriteHeader(402)
			case "bad":
				w.WriteHeader(401)
			}
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/pay/quota/metering/"):
			atomic.AddInt32(consoleHits, 1)
			if strings.HasSuffix(r.URL.Path, "/api-usage/summary") {
				// 按量余额端点：复用 consoleState 的故障注入，成功时返回 paygo 信封
				switch consoleState {
				case "ok", "no-pack":
					w.WriteHeader(200)
					w.Write([]byte(lcPaygoHappy))
				case "http401":
					w.WriteHeader(401)
					w.Write([]byte(lcConsole401))
				case "http500":
					w.WriteHeader(500)
					w.Write([]byte(`{"code":500,"msg":"服务器开小差了","data":null}`))
				}
				return
			}
			switch consoleState {
			case "ok":
				w.WriteHeader(200)
				w.Write([]byte(lcTokenPacksHappy))
			case "no-pack":
				w.WriteHeader(200)
				w.Write([]byte(lcTokenPacksNoPack))
			case "http401":
				w.WriteHeader(401)
				w.Write([]byte(lcConsole401))
			case "http500":
				w.WriteHeader(500)
				w.Write([]byte(`{"code":500,"msg":"服务器开小差了","data":null}`))
			}
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestRefreshLongCatNoCookieSkipsConsole(t *testing.T) {
	var hits int32
	srv := newLongCatConsoleAppTestServer("ok", "ok", &hits)
	defer srv.Close()

	cfg := &config.Config{
		LongCatAccounts: []models.LongCatAccount{
			{ID: "lc1", Name: "测试号", ApiKey: "test-key"}, // 无 Cookie
		},
	}
	app := New(cfg)
	app.Repos.LongCat.BaseURL = srv.URL
	app.Repos.LongCat.ConsoleURL = srv.URL
	app.Repos.LongCat.Client = srv.Client()

	res, err := app.RefreshLongCat("lc1")
	if err != nil {
		t.Fatalf("RefreshLongCat: %v", err)
	}
	if hits != 0 {
		t.Errorf("无 Cookie 不应请求控制台配额接口, got %d", hits)
	}
	if res.Quota != nil || res.Paygo != nil {
		t.Errorf("无 Cookie 不应有 Quota/Paygo: %+v %+v", res.Quota, res.Paygo)
	}
	if res.Usage == nil || res.Usage.BalanceOK == nil || !*res.Usage.BalanceOK {
		t.Error("探活结论应正常")
	}
}

func TestRefreshLongCatConsoleOK(t *testing.T) {
	var hits int32
	srv := newLongCatConsoleAppTestServer("ok", "ok", &hits)
	defer srv.Close()

	cfg := &config.Config{
		LongCatAccounts: []models.LongCatAccount{
			{ID: "lc1", Name: "测试号", ApiKey: "test-key", ConsoleCookie: "passport_token_key=fake-cookie-for-test"},
		},
	}
	app := New(cfg)
	app.Repos.LongCat.BaseURL = srv.URL
	app.Repos.LongCat.ConsoleURL = srv.URL
	app.Repos.LongCat.Client = srv.Client()

	res, err := app.RefreshLongCat("lc1")
	if err != nil {
		t.Fatalf("RefreshLongCat: %v", err)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Errorf("有 Cookie 应并发请求两个控制台端点, got %d", hits)
	}
	if res.Quota == nil || res.Quota.CurrentLot == nil {
		t.Fatalf("Quota 应非空: %+v", res.Quota)
	}
	if res.Quota.CurrentLot.RemainingToken != 1234567 {
		t.Errorf("RemainingToken: %+v", res.Quota.CurrentLot)
	}
	if res.Paygo == nil || res.Paygo.PaygoBalance == nil {
		t.Fatalf("Paygo 应非空: %+v", res.Paygo)
	}
	if res.Error != "" {
		t.Errorf("不应有错误: %s", res.Error)
	}
}

func TestRefreshLongCatConsoleAuthError(t *testing.T) {
	// Cookie 失效：错误出现，但不污染探活与清单结论
	var hits int32
	srv := newLongCatConsoleAppTestServer("ok", "http401", &hits)
	defer srv.Close()

	cfg := &config.Config{
		LongCatAccounts: []models.LongCatAccount{
			{ID: "lc1", Name: "测试号", ApiKey: "test-key", ConsoleCookie: "passport_token_key=stale-cookie"},
		},
	}
	app := New(cfg)
	app.Repos.LongCat.BaseURL = srv.URL
	app.Repos.LongCat.ConsoleURL = srv.URL
	app.Repos.LongCat.Client = srv.Client()

	res, err := app.RefreshLongCat("lc1")
	if err != nil {
		t.Fatalf("RefreshLongCat: %v", err)
	}
	if !strings.Contains(res.Error, "控制台会话已失效") {
		t.Errorf("Error 应含控制台会话失效: %q", res.Error)
	}
	if res.Plan == nil || len(res.Plan.Models) != 1 {
		t.Error("模型清单不应被 Cookie 通道失败污染")
	}
	if res.Usage == nil || res.Usage.BalanceOK == nil || !*res.Usage.BalanceOK {
		t.Error("探活结论不应被 Cookie 通道失败污染")
	}
}

func TestRefreshLongCatConsoleHTTP500(t *testing.T) {
	var hits int32
	srv := newLongCatConsoleAppTestServer("ok", "http500", &hits)
	defer srv.Close()

	cfg := &config.Config{
		LongCatAccounts: []models.LongCatAccount{
			{ID: "lc1", Name: "测试号", ApiKey: "test-key", ConsoleCookie: "passport_token_key=fake-cookie-for-test"},
		},
	}
	app := New(cfg)
	app.Repos.LongCat.BaseURL = srv.URL
	app.Repos.LongCat.ConsoleURL = srv.URL
	app.Repos.LongCat.Client = srv.Client()

	res, err := app.RefreshLongCat("lc1")
	if err != nil {
		t.Fatalf("RefreshLongCat: %v", err)
	}
	if !strings.Contains(res.Error, "HTTP 500") {
		t.Errorf("Error 应含 HTTP 500: %q", res.Error)
	}
	if res.Plan == nil || res.Usage == nil {
		t.Error("清单与探活不应被控制台 500 污染")
	}
}
