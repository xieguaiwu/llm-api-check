package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/repo"
)

const gzUsersResp = `{"data":{"email":"xieguaiwu@163.com","plan":"API (300k words/month)",` +
	`"char_limit":150000,"monthly_input_words":587,"monthly_input_chars":3952,"monthly_input_documents":1,` +
	`"all_time_input_words":1675389,"all_time_input_chars":11680324,"all_time_input_documents":861,` +
	`"last_time_usage_reset":"2026-09-06T16:28:34.943+00:00",` +
	`"full_plan":{"name":"API (300k words/month)","duration_type":"monthly","word_limit":300000,` +
	`"overage_word_limit":1000000,"priceData":{"unit_amount":4500}}}}`

func newGzApp(ts *httptest.Server, cfg *config.Config) *App {
	return NewWithRepos(cfg, &Repos{Gptzero: &repo.GptzeroRepo{BaseURL: ts.URL}})
}

func TestRefreshGptzeroHappy(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.URL.Path != "/v2/users/me" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(gzUsersResp))
	}))
	defer ts.Close()

	cfg := &config.Config{GptzeroAccounts: []models.GptzeroAccount{{ID: "g1", Name: "论文扫", ApiKey: "adfc"}}}
	res, err := newGzApp(ts, cfg).RefreshGptzero("g1")
	if err != nil {
		t.Fatalf("RefreshGptzero: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("不应有错误: %s", res.Error)
	}
	if res.Usage == nil {
		t.Fatal("Usage 不应缺席")
	}
	if res.Usage.Monthly.Words != 587 || res.Usage.Plan.WordLimit != 300000 {
		t.Errorf("用量数据不符: %+v", res.Usage)
	}
	if count.Load() != 1 {
		t.Errorf("应恰好 1 次请求: %d", count.Load())
	}
}

func TestRefreshGptzeroRepoNotInit(t *testing.T) {
	cfg := &config.Config{GptzeroAccounts: []models.GptzeroAccount{{ID: "g1", Name: "x", ApiKey: "k"}}}
	res, err := NewWithRepos(cfg, &Repos{}).RefreshGptzero("g1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(res.Error, "GPTZero 仓库未初始化") {
		t.Errorf("应报仓库未初始化: %q", res.Error)
	}
}

func TestRefreshGptzeroAccountMissing(t *testing.T) {
	cfg := &config.Config{}
	app := NewWithRepos(cfg, &Repos{Gptzero: &repo.GptzeroRepo{BaseURL: "http://127.0.0.1:1"}})
	_, err := app.RefreshGptzero("nope")
	if err == nil || err.Error() != "账号不存在或已被删除" {
		t.Fatalf("应报账号不存在: %v", err)
	}
}

func TestRefreshAllGptzeroLane(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Write([]byte(gzUsersResp))
	}))
	defer ts.Close()

	cfg := &config.Config{
		GptzeroAccounts: []models.GptzeroAccount{
			{ID: "g1", Name: "a", ApiKey: "k1"},
			{ID: "g2", Name: "b", ApiKey: "k2"},
		},
	}
	app := NewWithRepos(cfg, &Repos{Gptzero: &repo.GptzeroRepo{BaseURL: ts.URL}})
	res, err := app.RefreshAll()
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if len(res.Gptzero) != 2 {
		t.Fatalf("应有 2 条 GPTZero 结果: %d", len(res.Gptzero))
	}
	for _, r := range res.Gptzero {
		if r.Usage == nil || r.Error != "" {
			t.Errorf("lane 应全绿: %+v err=%s", r.Usage, r.Error)
		}
	}
	if count.Load() != 2 {
		t.Errorf("应 2 次请求: %d", count.Load())
	}
}

func TestRefreshAllGptzeroErrorKeepsLane(t *testing.T) {
	// 认证失败 lane 只影响自己：Error 有值、Usage 缺席，其余 provider lane 不受影响
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"API key has no owner","apiKey":"k1"}`))
	}))
	defer ts.Close()

	cfg := &config.Config{GptzeroAccounts: []models.GptzeroAccount{{ID: "g1", Name: "a", ApiKey: "k1"}}}
	res, err := NewWithRepos(cfg, &Repos{Gptzero: &repo.GptzeroRepo{BaseURL: ts.URL}}).RefreshAll()
	if err != nil {
		t.Fatalf("RefreshAll 不应整体失败: %v", err)
	}
	r := res.Gptzero[0]
	if r.Error == "" || r.Usage != nil {
		t.Errorf("坏 key lane 应 Error 非空且 Usage 缺席: %+v", r)
	}
	if !strings.Contains(r.Error, "GPTZero API Key 无效") {
		t.Errorf("错误应归一: %q", r.Error)
	}
}
