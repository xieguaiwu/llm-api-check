package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
