package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/repo"
)

// newGalaxyServer 四类端点各自的响应；failPath 非空时该路径回信封错误。
func newGalaxyServer(t *testing.T, count *atomic.Int32, failPath string) *httptest.Server {
	t.Helper()
	ok := func(data string) string {
		return `{"success":true,"code":"2000","message":"","data":` + data + `}`
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		count.Add(1)
		if failPath != "" && r.URL.Path == failPath {
			w.Write([]byte(`{"success":false,"code":"4000","message":"sign验证失败!","data":null}`))
			return
		}
		switch r.URL.Path {
		case "/account/get_main_account_info":
			w.Write([]byte(ok(readFixture(t, "galaxy_account_info.json"))))
		case "/instance/get_instance_status_count":
			w.Write([]byte(ok(readFixture(t, "galaxy_status_count.json"))))
		case "/instance/get_instance_list":
			// 单页返回全部：fixture 里 has_more=true 会让仓库继续翻页
			list := strings.Replace(readFixture(t, "galaxy_instance_list.json"), `"has_more": true`, `"has_more": false`, 1)
			w.Write([]byte(ok(list)))
		case "/billing/get_balance_change_list":
			bill := strings.Replace(readFixture(t, "galaxy_balance_changes.json"), `"has_more": true`, `"has_more": false`, 1)
			w.Write([]byte(ok(bill)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func galaxyApp(srv *httptest.Server, cfg *config.Config) *App {
	return NewWithRepos(cfg, &Repos{
		Galaxy: &repo.GalaxyRepo{BaseURL: srv.URL, Client: srv.Client()},
	})
}

var galaxyCfg = &config.Config{GalaxyAccounts: []models.GalaxyAccount{
	{ID: "g1", Name: "训练集群", AccessKey: "ak-test", SecretKey: "sk-test"},
}}

func TestRefreshGalaxyAllSections(t *testing.T) {
	var count atomic.Int32
	srv := newGalaxyServer(t, &count, "")
	defer srv.Close()

	res, err := galaxyApp(srv, galaxyCfg).RefreshGalaxy("g1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Errorf("全绿时不该有错误: %q", res.Error)
	}
	if res.Balance == nil || res.Balance.Money != 96.2805 {
		t.Errorf("余额缺失: %+v", res.Balance)
	}
	if res.Status == nil || res.Status.Running != 4 {
		t.Errorf("统计缺失: %+v", res.Status)
	}
	if len(res.Instances) != 4 {
		t.Errorf("实例列表应为 4 条: %d", len(res.Instances))
	}
	if res.Cost == nil {
		t.Error("消耗缺失")
	}
}

// TestRefreshGalaxyPartialFailure 只有一路失败时，其余数据照常展示，
// 错误合并进 Error（同 DeepSeek balance/cost 的处理口径）
func TestRefreshGalaxyPartialFailure(t *testing.T) {
	var count atomic.Int32
	srv := newGalaxyServer(t, &count, "/instance/get_instance_status_count")
	defer srv.Close()

	res, err := galaxyApp(srv, galaxyCfg).RefreshGalaxy("g1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Balance == nil {
		t.Error("统计失败不该连带影响余额")
	}
	if res.Status != nil {
		t.Error("失败的那路不该有数据")
	}
	if !strings.Contains(res.Error, "签名校验失败") {
		t.Errorf("错误应合并失败路文案: %q", res.Error)
	}
}

// TestRefreshGalaxyAllFailureKeepsAccountInfo 全失败：无数据 + 有错误（上层据此 exit 1）
func TestRefreshGalaxyAllFailure(t *testing.T) {
	var count atomic.Int32
	srv := newGalaxyServer(t, &count, "/account/get_main_account_info")
	defer srv.Close()

	cfg := &config.Config{GalaxyAccounts: []models.GalaxyAccount{
		{ID: "g1", Name: "n", AccessKey: "ak", SecretKey: ""}, // 缺 SecretKey → 四路全失败
	}}
	res, err := galaxyApp(srv, cfg).RefreshGalaxy("g1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Balance != nil || res.Status != nil || len(res.Instances) != 0 || res.Cost != nil {
		t.Errorf("全失败仍拿到数据: %+v", res)
	}
	if res.Error == "" {
		t.Error("全失败必须带错误")
	}
	if count.Load() != 0 {
		t.Errorf("缺凭据时四路都不该发请求，实发 %d", count.Load())
	}
}

func TestRefreshGalaxyUnknownID(t *testing.T) {
	var count atomic.Int32
	srv := newGalaxyServer(t, &count, "")
	defer srv.Close()
	if _, err := galaxyApp(srv, galaxyCfg).RefreshGalaxy("nope", 10); err == nil {
		t.Error("未知 id 应报错")
	}
}

// TestRefreshGalaxyNilRepo 仓库未初始化不能 panic（其他 provider 单测复用同一 App）
func TestRefreshGalaxyNilRepo(t *testing.T) {
	a := NewWithRepos(galaxyCfg, &Repos{})
	res, err := a.RefreshGalaxy("g1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error == "" {
		t.Error("仓库缺失应转成错误文案而非崩溃")
	}
}

// TestRefreshAllIncludesGalaxy status 总览必须带上智星云账号
func TestRefreshAllIncludesGalaxy(t *testing.T) {
	var count atomic.Int32
	srv := newGalaxyServer(t, &count, "")
	defer srv.Close()

	cfg := &config.Config{
		DeepSeekAccounts: []models.DeepSeekAccount{{ID: "ds1", Name: "DS", ApiKey: "k"}},
		GalaxyAccounts:   galaxyCfg.GalaxyAccounts,
	}
	a := NewWithRepos(cfg, &Repos{
		DeepSeek: &repo.DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()},
		Galaxy:   &repo.GalaxyRepo{BaseURL: srv.URL, Client: srv.Client()},
	})
	res, err := a.RefreshAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Galaxy) != 1 || res.Galaxy[0].Balance == nil {
		t.Errorf("总览缺智星云数据: %+v", res.Galaxy)
	}
	if len(res.DeepSeek) != 1 {
		t.Errorf("总览缺 DeepSeek 数据: %+v", res.DeepSeek)
	}
}

// TestGalaxyHourlyCost 时价只累计仍占资源的实例（已结束的不该算进「还能撑多久」）
func TestGalaxyHourlyCost(t *testing.T) {
	r := GalaxyResult{Instances: []models.GalaxyInstance{
		{Status: 1, TotalCost: 0.325},
		{Status: 4, TotalCost: 0.1},
		{Status: 8, TotalCost: 0.0009}, // 磁盘保留按磁盘价计费，不算运行时时价
		{Status: 0, TotalCost: 9},
	}}
	if got := r.HourlyCost(); got-0.425 > 1e-9 || got-0.425 < -1e-9 {
		t.Errorf("合计时价 got %v want 0.425", got)
	}
}

func TestGalaxyResultTimeFieldsParseable(t *testing.T) {
	var count atomic.Int32
	srv := newGalaxyServer(t, &count, "")
	defer srv.Close()
	res, err := galaxyApp(srv, galaxyCfg).RefreshGalaxy("g1", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range res.Instances {
		if in.DueAt == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, in.DueAt); err != nil {
			t.Errorf("到期时间必须是 RFC3339: %q", in.DueAt)
		}
	}
}
