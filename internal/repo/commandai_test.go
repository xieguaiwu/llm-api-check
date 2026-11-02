package repo

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

const (
	cmWhoamiHappy  = `{"org":{"id":"org_abc","login":"acme"},"user":{"userName":"alice"}}`
	cmWhoamiNoOrg  = `{"success":true,"user":{"userName":"alice"},"org":null}`
	cmCreditsHappy = `{"credits":{"belowThreshold":false,"creditThreshold":0,"monthlyCredits":10,` +
		`"purchasedCredits":3.2,"freeCredits":0},` +
		`"windowLimits":{"limited":true,"exceeded":null,` +
		`"fiveHour":{"used":14.2,"cap":14,"exceeded":true,"resetAt":1792901766000}},` +
		`"sandboxAccess":true,"sandboxMinutes":null}`
	cmSubscriptionHappy = `{"data":{"planId":"individual-provider","status":"active",` +
		`"currentPeriodStart":"2026-10-01T00:00:00Z","currentPeriodEnd":"2026-11-01T00:00:00Z"}}`
	cmSummaryHappy = `{"totalCost":1.8,"totalCount":42,"periodBasis":"billing-period"}`
	cmUnauthorized = `{"success":false,"error":{"code":"UNAUTHORIZED","status":401,` +
		`"message":"Invalid 'Authorization' header or token."}}`
	cmUpgradeRequired = `{"success":false,"error":{"code":"upgrade_required","status":403,` +
		`"message":"Current plan does not include API access"}}`
)

// cmQuotaServer 模拟 Commandai 全链路（whoami → credits ∥ subscriptions → summary）。
// 记录 credits/subscriptions 并发命中与 summary 的 since 参数。
type cmQuotaServer struct {
	*httptest.Server
	mu            sync.Mutex
	creditsHits   int
	subHits       int
	summaryHits   int
	lastSince     string
	creditsStatus int
	subStatus     int
}

func newCMQuotaServer() *cmQuotaServer {
	s := &cmQuotaServer{creditsStatus: 200, subStatus: 200}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/alpha/whoami":
			w.Write([]byte(cmWhoamiHappy))
		case "/alpha/billing/credits":
			s.mu.Lock()
			s.creditsHits++
			code := s.creditsStatus
			s.mu.Unlock()
			w.WriteHeader(code)
			if code == 200 {
				w.Write([]byte(cmCreditsHappy))
			} else {
				w.Write([]byte(cmUnauthorized))
			}
		case "/alpha/billing/subscriptions":
			s.mu.Lock()
			s.subHits++
			code := s.subStatus
			s.mu.Unlock()
			w.WriteHeader(code)
			if code == 200 {
				w.Write([]byte(cmSubscriptionHappy))
			} else {
				w.Write([]byte(cmUnauthorized))
			}
		case "/alpha/usage/summary":
			s.mu.Lock()
			s.summaryHits++
			s.lastSince = r.URL.Query().Get("since")
			s.mu.Unlock()
			w.Write([]byte(cmSummaryHappy))
		default:
			w.WriteHeader(404)
		}
	}))
	return s
}

func TestCommandaiModelsHappy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/provider/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"object":"list","data":[{"id":"claude-sonnet-5-5","owned_by":"command-code",` +
				`"name":"Claude Sonnet 5.5","context_length":1000000,"supported_endpoints":["/messages"]}]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	plan, err := r.Models("test-key")
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(plan.Models) != 1 || plan.Models[0].ID != "claude-sonnet-5-5" {
		t.Errorf("plan: %+v", plan)
	}
}

func TestCommandaiModelsEmptyKeyAllowed(t *testing.T) {
	// /provider/v1/models 实测免认证：空 key 仍应发出请求
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	plan, err := r.Models("")
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(plan.Models) != 0 {
		t.Errorf("plan: %+v", plan)
	}
	if gotAuth != "" {
		t.Errorf("empty key should not send Authorization header: %q", gotAuth)
	}
}

func TestCommandaiModelsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(cmUnauthorized))
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	_, err := r.Models("bad-key")
	if !errors.Is(err, parsers.ErrCommandaiAuth) {
		t.Errorf("401 envelope should map to ErrCommandaiAuth: %v", err)
	}
}

func TestCommandaiModelsUpgradeRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(cmUpgradeRequired))
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	_, err := r.Models("bad-key")
	if !errors.Is(err, parsers.ErrCommandaiUpgrade) {
		t.Errorf("403 upgrade_required should map to ErrCommandaiUpgrade: %v", err)
	}
}

func TestCommandaiQuotaHappy(t *testing.T) {
	srv := newCMQuotaServer()
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	who, cred, sub, sum, err := r.Quota("test-key")
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}
	if who == nil || who.OrgID != "org_abc" {
		t.Errorf("whoami: %+v", who)
	}
	if cred == nil || cred.MonthlyCredits != 10 {
		t.Errorf("credits: %+v", cred)
	}
	if sub == nil || sub.Status != "active" {
		t.Errorf("subscription: %+v", sub)
	}
	if sum == nil || sum.TotalCost != 1.8 {
		t.Errorf("summary: %+v", sum)
	}
	// credits 与 subscriptions 都命中（并发）
	if srv.creditsHits != 1 || srv.subHits != 1 {
		t.Errorf("hits: credits=%d sub=%d, want 1/1", srv.creditsHits, srv.subHits)
	}
	// summary 的 since = currentPeriodStart 原文
	if srv.lastSince != "2026-10-01T00:00:00Z" {
		t.Errorf("summary since: %q", srv.lastSince)
	}
}

// 实测个人账号 org 为 null：orgId 必须整个省略——带空 orgId（?orgId=）实测 400
// BAD_REQUEST「Invalid UUID at "orgId"」。
func TestCommandaiQuotaOmitsEmptyOrgID(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		paths = append(paths, r.URL.RequestURI())
		switch r.URL.Path {
		case "/alpha/whoami":
			w.Write([]byte(cmWhoamiNoOrg))
		case "/alpha/billing/credits":
			w.Write([]byte(cmCreditsHappy))
		case "/alpha/billing/subscriptions":
			w.Write([]byte(cmSubscriptionHappy))
		case "/alpha/usage/summary":
			w.Write([]byte(cmSummaryHappy))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	who, cred, _, sum, err := r.Quota("test-key")
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}
	if who == nil || who.OrgID != "" {
		t.Errorf("whoami should parse with empty orgId: %+v", who)
	}
	if cred == nil || sum == nil {
		t.Fatalf("segments should all succeed: cred=%v sum=%v", cred, sum)
	}
	for _, p := range paths {
		if strings.Contains(p, "orgId=") {
			t.Errorf("orgId must be omitted when empty, got %q", p)
		}
	}
	// 额度端点确实被调用到了
	if len(paths) != 4 {
		t.Errorf("requests: got %d %v, want 4", len(paths), paths)
	}
}

// 有 org 时 orgId 仍须带上（团队账号路径不回归）。
func TestCommandaiQuotaKeepsOrgID(t *testing.T) {
	var uris []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uris = append(uris, r.URL.RequestURI())
		switch r.URL.Path {
		case "/alpha/whoami":
			w.Write([]byte(cmWhoamiHappy))
		case "/alpha/billing/credits":
			w.Write([]byte(cmCreditsHappy))
		case "/alpha/billing/subscriptions":
			w.Write([]byte(cmSubscriptionHappy))
		case "/alpha/usage/summary":
			w.Write([]byte(cmSummaryHappy))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	if _, _, _, _, err := r.Quota("test-key"); err != nil {
		t.Fatalf("Quota: %v", err)
	}
	for _, u := range uris[1:] {
		if !strings.Contains(u, "orgId=org_abc") {
			t.Errorf("orgId should be kept for team accounts, got %q", u)
		}
	}
}

func TestCommandaiQuotaWhoami401ShortCircuit(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(401)
		w.Write([]byte(cmUnauthorized))
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	who, cred, sub, sum, err := r.Quota("bad-key")
	if !errors.Is(err, parsers.ErrCommandaiAuth) {
		t.Errorf("whoami 401 should short-circuit to ErrCommandaiAuth: %v", err)
	}
	if who != nil || cred != nil || sub != nil || sum != nil {
		t.Errorf("short-circuit should return nil segments: %v %v %v %v", who, cred, sub, sum)
	}
	// 短路：只请求 whoami 一跳
	if hits != 1 {
		t.Errorf("hits: got %d, want 1", hits)
	}
}

func TestCommandaiQuotaPartialFailure(t *testing.T) {
	srv := newCMQuotaServer()
	srv.creditsStatus = 401 // credits 失败，subscription 成功
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	who, cred, sub, sum, err := r.Quota("test-key")
	if who == nil {
		t.Error("whoami should still be returned")
	}
	if cred != nil {
		t.Errorf("credits should be nil on failure: %+v", cred)
	}
	if sub == nil || sub.Status != "active" {
		t.Errorf("subscription should be returned: %+v", sub)
	}
	if sum == nil {
		t.Error("summary should still be returned")
	}
	if err == nil {
		t.Error("partial failure should surface an error")
	}
	if !strings.Contains(err.Error(), "Commandai") {
		t.Errorf("error should mention Commandai: %v", err)
	}
}

func TestCommandaiQuotaSummarySinceOmitted(t *testing.T) {
	// 无周期（subscription 无 currentPeriodStart）→ since 参数省略
	var gotSince string
	var gotHasSince bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/alpha/whoami":
			w.Write([]byte(cmWhoamiHappy))
		case "/alpha/billing/credits":
			w.Write([]byte(cmCreditsHappy))
		case "/alpha/billing/subscriptions":
			w.Write([]byte(`{"data":{"planId":"individual-provider","status":"active"}}`))
		case "/alpha/usage/summary":
			gotSince = r.URL.Query().Get("since")
			_, gotHasSince = r.URL.Query()["since"]
			w.Write([]byte(cmSummaryHappy))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	if _, _, _, _, err := r.Quota("test-key"); err != nil {
		t.Fatalf("Quota: %v", err)
	}
	if gotHasSince {
		t.Errorf("since should be omitted when no period start: %q", gotSince)
	}
}

func TestCommandaiModelsHTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`boom`))
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	_, err := r.Models("test-key")
	if err == nil {
		t.Fatal("HTTP 500 should error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error should mention HTTP 500: %v", err)
	}
}

func TestCommandaiModelsDedupSort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"z-model"},{"id":"a-model"},{"id":"a-model"}]}`))
	}))
	defer srv.Close()
	r := NewCommandaiRepo()
	r.BaseURL = srv.URL
	r.Client = srv.Client()

	plan, err := r.Models("test-key")
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(plan.Models) != 2 || plan.Models[0].ID != "a-model" {
		t.Errorf("dedup/sort: %+v", plan.Models)
	}
}
