package repo

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// newLongCatTestServer 启动一个模拟 LongCat API 的 httptest 服务器。
//   - GET /openai/v1/models → 模型清单
//   - POST /openai/v1/chat/completions → 根据 balanceState 返回：
//     "ok" → 200, "empty" → 402, "bad" → 401
func newLongCatTestServer(balanceState string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "LongCat-2.0", "owned_by": "meituan"},
					{"id": "LongCat-Flash-Chat", "owned_by": "meituan"},
				},
			}
			json.NewEncoder(w).Encode(resp)
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
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"code": "insufficient_quota", "message": "余额不足"},
				})
			case "bad":
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"code": "invalid_api_key", "message": "incorrect api key"},
				})
			}
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestLongCatModelsHappy(t *testing.T) {
	srv := newLongCatTestServer("ok")
	defer srv.Close()
	repo := NewLongCatRepo()
	repo.BaseURL = srv.URL
	repo.Client = srv.Client()

	plan, err := repo.Models("test-key")
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(plan.Models) != 2 {
		t.Fatalf("len: got %d, want 2", len(plan.Models))
	}
	if plan.Models[0].ID != "LongCat-2.0" {
		t.Errorf("Models[0].ID: %q", plan.Models[0].ID)
	}
}

func TestLongCatModelsEmptyKey(t *testing.T) {
	srv := newLongCatTestServer("ok")
	defer srv.Close()
	repo := NewLongCatRepo()
	repo.BaseURL = srv.URL
	repo.Client = srv.Client()

	_, err := repo.Models("")
	if err == nil {
		t.Fatal("empty key should error")
	}
}

func TestLongCatProbeBalanceOK(t *testing.T) {
	srv := newLongCatTestServer("ok")
	defer srv.Close()
	repo := NewLongCatRepo()
	repo.BaseURL = srv.URL
	repo.Client = srv.Client()

	ok, err := repo.ProbeBalance("test-key")
	if err != nil {
		t.Fatalf("ProbeBalance: %v", err)
	}
	if !ok {
		t.Fatal("balance should be ok")
	}
}

func TestLongCatProbeBalanceEmpty(t *testing.T) {
	srv := newLongCatTestServer("empty")
	defer srv.Close()
	repo := NewLongCatRepo()
	repo.BaseURL = srv.URL
	repo.Client = srv.Client()

	ok, err := repo.ProbeBalance("test-key")
	if err != nil {
		t.Fatalf("ProbeBalance: %v", err)
	}
	if ok {
		t.Fatal("balance should be empty (402)")
	}
}

func TestLongCatProbeBalanceBadKey(t *testing.T) {
	srv := newLongCatTestServer("bad")
	defer srv.Close()
	repo := NewLongCatRepo()
	repo.BaseURL = srv.URL
	repo.Client = srv.Client()

	_, err := repo.ProbeBalance("bad-key")
	if err == nil {
		t.Fatal("bad key should error")
	}
	if !strings.Contains(err.Error(), "无效") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error should mention invalid key: %v", err)
	}
}

func TestLongCatModelsBadKey(t *testing.T) {
	// 401 服务器：/v1/models 也回 401
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "invalid_api_key", "message": "incorrect api key"},
		})
	}))
	defer srv.Close()
	repo := NewLongCatRepo()
	repo.BaseURL = srv.URL
	repo.Client = srv.Client()

	_, err := repo.Models("bad-key")
	if err == nil {
		t.Fatal("bad key should error")
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

// newLongCatConsoleTestServer 模拟 longcat.chat 控制台配额接口（httptest 注入）。
// state: "ok" | "no-pack" | "http401" | "http500" | "env401"
func newLongCatConsoleTestServer(state string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/pay/quota/metering/") {
			w.Header().Set("Content-Type", "application/json")
			switch state {
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
			case "env401":
				// 网关偶发把 401 放信封里回 200
				w.WriteHeader(200)
				w.Write([]byte(lcConsole401))
			}
			return
		}
		w.WriteHeader(404)
	}))
}

func newLongCatConsoleRepo(state string) (*LongCatRepo, *httptest.Server) {
	srv := newLongCatConsoleTestServer(state)
	r := NewLongCatRepo()
	r.ConsoleURL = srv.URL
	r.Client = srv.Client()
	return r, srv
}

func TestLongCatConsoleQuotaHappy(t *testing.T) {
	r, srv := newLongCatConsoleRepo("ok")
	defer srv.Close()
	q, err := r.ConsoleQuota("passport_token_key=fake-cookie-for-test")
	if err != nil {
		t.Fatalf("ConsoleQuota: %v", err)
	}
	if q.CurrentLot == nil || q.CurrentLot.RemainingToken != 1234567 {
		t.Errorf("quota: %+v", q)
	}
}

func TestLongCatConsoleQuotaCookieHeader(t *testing.T) {
	// Cookie 头透传：完整 Cookie 头原样发送
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(lcTokenPacksNoPack))
	}))
	defer srv.Close()
	r := NewLongCatRepo()
	r.ConsoleURL = srv.URL
	r.Client = srv.Client()
	if _, err := r.ConsoleQuota("Cookie: passport_token_key=fake-cookie-for-test; other=1"); err != nil {
		t.Fatalf("ConsoleQuota: %v", err)
	}
	if gotCookie != "passport_token_key=fake-cookie-for-test; other=1" {
		t.Errorf("Cookie header not passed through: %q", gotCookie)
	}
}

func TestLongCatConsoleQuotaBareValue(t *testing.T) {
	// 不含 = 的值视为裸会话值，自动补 passport_token_key= 前缀
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(lcTokenPacksNoPack))
	}))
	defer srv.Close()
	r := NewLongCatRepo()
	r.ConsoleURL = srv.URL
	r.Client = srv.Client()
	if _, err := r.ConsoleQuota("fake-cookie-for-test"); err != nil {
		t.Fatalf("ConsoleQuota: %v", err)
	}
	if gotCookie != "passport_token_key=fake-cookie-for-test" {
		t.Errorf("bare value should get passport_token_key= prefix: %q", gotCookie)
	}
}

func TestLongCatConsoleQuotaHTTP401(t *testing.T) {
	r, srv := newLongCatConsoleRepo("http401")
	defer srv.Close()
	_, err := r.ConsoleQuota("passport_token_key=fake-cookie-for-test")
	if !errors.Is(err, parsers.ErrLongCatConsoleAuth) {
		t.Errorf("HTTP 401 should map to ErrLongCatConsoleAuth: %v", err)
	}
}

func TestLongCatConsoleQuotaEnvelope401(t *testing.T) {
	r, srv := newLongCatConsoleRepo("env401")
	defer srv.Close()
	_, err := r.ConsoleQuota("passport_token_key=fake-cookie-for-test")
	if !errors.Is(err, parsers.ErrLongCatConsoleAuth) {
		t.Errorf("envelope code=401 should map to ErrLongCatConsoleAuth: %v", err)
	}
}

func TestLongCatConsoleQuotaHTTP500(t *testing.T) {
	r, srv := newLongCatConsoleRepo("http500")
	defer srv.Close()
	_, err := r.ConsoleQuota("passport_token_key=fake-cookie-for-test")
	if err == nil {
		t.Fatal("HTTP 500 should error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error should mention HTTP 500: %v", err)
	}
}

func TestLongCatConsolePaygoHappy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/pay/quota/metering/api-usage/summary" {
			w.Write([]byte(lcPaygoHappy))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	r := NewLongCatRepo()
	r.ConsoleURL = srv.URL
	r.Client = srv.Client()
	p, err := r.ConsolePaygo("passport_token_key=fake-cookie-for-test")
	if err != nil {
		t.Fatalf("ConsolePaygo: %v", err)
	}
	if p.PaygoBalance == nil || p.PaygoBalance.Primary.Amount != "0.00" {
		t.Errorf("paygo: %+v", p)
	}
}

func TestLongCatConsolePaygo403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(lcConsole401))
	}))
	defer srv.Close()
	r := NewLongCatRepo()
	r.ConsoleURL = srv.URL
	r.Client = srv.Client()
	_, err := r.ConsolePaygo("passport_token_key=fake-cookie-for-test")
	if !errors.Is(err, parsers.ErrLongCatConsoleAuth) {
		t.Errorf("HTTP 403 should map to ErrLongCatConsoleAuth: %v", err)
	}
}
