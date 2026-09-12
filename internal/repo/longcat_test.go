package repo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/models"
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

func TestLongCatAccountStruct(t *testing.T) {
	acc := models.LongCatAccount{
		ID:     "test-id",
		Name:   "测试号",
		ApiKey: "sk-test123",
	}
	if acc.ID != "test-id" {
		t.Errorf("ID: %q", acc.ID)
	}
}
