package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/repo"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// newTestServer 单端点 httptest 服务：/user/balance、/api/v0/usage/cost、
// /zen/go/v1/usage、/workspace/{id}/billing 都返回对应 fixture 数据；
// 请求计数原子累加。
func newTestServer(t *testing.T, count *atomic.Int32, slow time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slow > 0 {
			time.Sleep(slow)
		}
		count.Add(1)
		switch r.URL.Path {
		case "/user/balance":
			w.Write([]byte(readFixture(t, "deepseek_balance.json")))
		case "/api/v0/usage/cost":
			w.Write([]byte(readFixture(t, "deepseek_cost.json")))
		case "/zen/go/v1/usage":
			w.Write([]byte(readFixture(t, "go_usage.json")))
		case "/workspace/wrk_TEST/billing":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(readFixture(t, "billing.html")))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func testRepos(srv *httptest.Server) *Repos {
	return &Repos{
		DeepSeek: &repo.DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()},
		OpenCode: &repo.OpenCodeRepo{BaseURL: srv.URL, Client: srv.Client()},
	}
}

func TestRefreshAllAggregatesAllAccounts(t *testing.T) {
	var count atomic.Int32
	srv := newTestServer(t, &count, 0)
	defer srv.Close()

	cfg := &config.Config{
		DeepSeekAccounts: []models.DeepSeekAccount{
			{ID: "ds1", Name: "DS1", ApiKey: "k1"},
			{ID: "ds2", Name: "DS2", ApiKey: "k2", PlatformToken: "t2"},
		},
		Accounts: []models.Account{
			{ID: "a1", Name: "OC1", GoApiKey: "g1"},
			{ID: "a2", Name: "OC2", GoApiKey: "g2", WorkspaceId: "wrk_TEST", AuthCookie: "c2"},
		},
	}
	a := NewWithRepos(cfg, testRepos(srv))
	res, err := a.RefreshAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.DeepSeek) != 2 || len(res.Accounts) != 2 {
		t.Fatalf("结果数量不符: %+v", res)
	}
	// DS1：无 token → 只刷余额
	if res.DeepSeek[0].Balance == nil || res.DeepSeek[0].Cost != nil || res.DeepSeek[0].Error != "" {
		t.Errorf("DS1 不符: %+v", res.DeepSeek[0])
	}
	// DS2：有 token → 余额 + 消费
	if res.DeepSeek[1].Balance == nil || res.DeepSeek[1].Cost == nil || res.DeepSeek[1].Error != "" {
		t.Errorf("DS2 不符: %+v", res.DeepSeek[1])
	}
	// OC1：无 zen → 只刷 go
	if res.Accounts[0].GoUsage == nil || res.Accounts[0].ZenBilling != nil || res.Accounts[0].Error != "" {
		t.Errorf("OC1 不符: %+v", res.Accounts[0])
	}
	// OC2：有 zen → go + zen
	if res.Accounts[1].GoUsage == nil || res.Accounts[1].ZenBilling == nil || res.Accounts[1].Error != "" {
		t.Errorf("OC2 不符: %+v", res.Accounts[1])
	}
	// 请求数：DS1 余额 1 + DS2 余额/消费两月 3 + OC1 go 1 + OC2 go/zen 2 = 7
	if count.Load() != 7 {
		t.Errorf("请求数应为 7，got %d", count.Load())
	}
	if a.LastUpdated().IsZero() {
		t.Error("RefreshAll 应写入 LastUpdate")
	}
}

func TestRefreshDeepSeekErrorMerge(t *testing.T) {
	// balance 401 失败 + cost 成功 → Error 非空但 Cost 有值
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/balance":
			w.WriteHeader(http.StatusUnauthorized)
		case "/api/v0/usage/cost":
			w.Write([]byte(readFixture(t, "deepseek_cost.json")))
		}
	}))
	defer srv.Close()

	cfg := &config.Config{DeepSeekAccounts: []models.DeepSeekAccount{
		{ID: "ds1", Name: "DS", ApiKey: "bad", PlatformToken: "t"},
	}}
	a := NewWithRepos(cfg, testRepos(srv))
	res, err := a.RefreshDeepSeek("ds1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Balance != nil {
		t.Error("balance 应失败为 nil")
	}
	if res.Cost == nil {
		t.Error("cost 应成功有值")
	}
	if res.Error != "API Key 无效或已过期" {
		t.Errorf("Error 应含 balance 错误，got %q", res.Error)
	}
}

func TestRefreshDeepSeekNotFound(t *testing.T) {
	a := New(&config.Config{})
	if _, err := a.RefreshDeepSeek("nope"); err == nil || err.Error() != "账号不存在或已被删除" {
		t.Errorf("不存在的账号应报错，got %v", err)
	}
}

func TestRefreshAccountErrorJoin(t *testing.T) {
	// go 401 + zen 401 → 错误用 \n 连接两条
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &config.Config{Accounts: []models.Account{
		{ID: "a1", Name: "OC", GoApiKey: "bad", WorkspaceId: "w", AuthCookie: "c"},
	}}
	a := NewWithRepos(cfg, testRepos(srv))
	res, err := a.RefreshAccount("a1")
	if err != nil {
		t.Fatal(err)
	}
	if res.GoUsage != nil || res.ZenBilling != nil {
		t.Error("两者都应失败")
	}
	want := "Go API Key 无效或已过期\nCookie 已过期，请更新 Auth Cookie"
	if res.Error != want {
		t.Errorf("Error 应合并两条，got %q", res.Error)
	}
}

func TestRefreshAllReentryProtection(t *testing.T) {
	var count atomic.Int32
	firstReq := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	// 第一个请求阻塞直到测试放行，保证第二次 RefreshAll 时第一次仍在刷新中
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		once.Do(func() { close(firstReq) })
		<-release
		w.Write([]byte(readFixture(t, "go_usage.json")))
	}))
	defer srv.Close()

	cfg := &config.Config{Accounts: []models.Account{{ID: "a1", Name: "OC", GoApiKey: "g"}}}
	a := NewWithRepos(cfg, testRepos(srv))

	done := make(chan struct{})
	go func() {
		a.RefreshAll()
		close(done)
	}()
	<-firstReq // 第一次刷新已发起请求并持有 refreshing
	res2, _ := a.RefreshAll()
	if len(res2.Accounts) != 0 {
		t.Errorf("重入调用应返回空结果，got %+v", res2)
	}
	close(release)
	<-done
	if count.Load() != 1 {
		t.Errorf("应只发起 1 次请求，got %d", count.Load())
	}
}

func TestRefreshAllNoAccounts(t *testing.T) {
	var count atomic.Int32
	srv := newTestServer(t, &count, 0)
	defer srv.Close()

	a := NewWithRepos(&config.Config{}, testRepos(srv))
	res, err := a.RefreshAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.DeepSeek) != 0 || len(res.Accounts) != 0 {
		t.Errorf("空配置应返回空结果: %+v", res)
	}
	if count.Load() != 0 {
		t.Errorf("不应发起请求，got %d", count.Load())
	}
}
