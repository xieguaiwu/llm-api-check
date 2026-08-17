package repo

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "fixtures", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// ── OpenCode Go usage ─────────────────────────────────────────

func TestGoUsageOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zen/go/v1/usage" {
			t.Errorf("路径不符: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-go-key" {
			t.Errorf("Authorization 头不符: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(readFixture(t, "go_usage.json")))
	}))
	defer srv.Close()

	repo := &OpenCodeRepo{BaseURL: srv.URL, Client: srv.Client()}
	u, err := repo.GoUsage(models.Account{ID: "1", Name: "n", GoApiKey: "test-go-key"})
	if err != nil {
		t.Fatal(err)
	}
	if u.Rolling == nil || u.Rolling.Status != "ok" || u.Monthly == nil || u.Monthly.Percent != 100 {
		t.Errorf("go usage 解析不符: %+v", u)
	}
}

func TestGoUsage401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	repo := &OpenCodeRepo{BaseURL: srv.URL, Client: srv.Client()}
	_, err := repo.GoUsage(models.Account{ID: "1", GoApiKey: "bad"})
	if err == nil || err.Error() != "Go API Key 无效或已过期" {
		t.Errorf("401 应返回逐字错误「Go API Key 无效或已过期」，got %v", err)
	}
}

func TestGoUsage500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	repo := &OpenCodeRepo{BaseURL: srv.URL, Client: srv.Client()}
	_, err := repo.GoUsage(models.Account{ID: "1", GoApiKey: "k"})
	if err == nil || !strings.HasPrefix(err.Error(), "HTTP 500") {
		t.Errorf("500 应返回 HTTP 500 前缀错误，got %v", err)
	}
}

// ── DeepSeek 余额 ─────────────────────────────────────────────

func TestBalanceOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/balance" {
			t.Errorf("路径不符: %s", r.URL.Path)
		}
		w.Write([]byte(readFixture(t, "deepseek_balance.json")))
	}))
	defer srv.Close()

	repo := &DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()}
	b, err := repo.Balance("k")
	if err != nil {
		t.Fatal(err)
	}
	if !b.IsAvailable || len(b.Infos) != 1 || b.Infos[0].TotalBalance != 120.0 {
		t.Errorf("余额解析不符: %+v", b)
	}
}

func TestBalance401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	repo := &DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()}
	_, err := repo.Balance("bad")
	if err == nil || err.Error() != "API Key 无效或已过期" {
		t.Errorf("401/403 应返回逐字错误「API Key 无效或已过期」，got %v", err)
	}
}

// ── DeepSeek 消费明细（两月聚合） ─────────────────────────────

// costServer 模拟 platform 页面 API：按 month/year 参数返回不同数据
func costServer(t *testing.T, monthData map[int]string, slow time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slow > 0 {
			time.Sleep(slow)
		}
		if r.URL.Path != "/api/v0/usage/cost" {
			t.Errorf("路径不符: %s", r.URL.Path)
			return
		}
		var month int
		if _, err := fmt.Sscan(r.URL.Query().Get("month"), &month); err != nil {
			t.Errorf("month 参数缺失")
			return
		}
		if r.Header.Get("Referer") != "https://platform.deepseek.com/usage" {
			t.Errorf("Referer 头不符: %q", r.Header.Get("Referer"))
		}
		if r.Header.Get("x-app-version") != "1.0.0" {
			t.Errorf("x-app-version 头不符: %q", r.Header.Get("x-app-version"))
		}
		body, ok := monthData[month]
		if !ok {
			body = `{"code":0,"data":{"biz_data":[]}}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
}

func costJSON(date string, amount float64) string {
	return `{"code":0,"data":{"biz_data":[{"days":[{"date":"` + date + `","data":[{"model":"deepseek-chat","usage":[{"type":"input","amount":` + formatFloat(amount) + `}]}]}]}]}}`
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func TestCostTwoMonthAggregation(t *testing.T) {
	// 2026-08 的 8 月 + 7 月：8 月含 08-14 一天 1.0，7 月含 07-31 一天 2.0
	srv := costServer(t, map[int]string{
		8: costJSON("2026-08-14", 1.0),
		7: costJSON("2026-07-31", 2.0),
	}, 0)
	defer srv.Close()

	repo := &DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()}
	// 固定 now = 2026-08-18（refDate 为当天）
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	repo.Now = func() time.Time { return now }

	c, err := repo.Cost("tok")
	if err != nil {
		t.Fatal(err)
	}
	// today(08-18)=0；last7d(08-12..08-18)=1.0（08-14）；last30d(07-20..08-18)=3.0
	if c.Today != 0.0 || c.Last7d != 1.0 || c.Last30d != 3.0 {
		t.Errorf("聚合不符: today=%v last7d=%v last30d=%v", c.Today, c.Last7d, c.Last30d)
	}
	if len(c.Days) != 2 || c.Days[0].Date != "2026-08-14" || c.Days[1].Date != "2026-07-31" {
		t.Errorf("days 应合并两月并倒序: %+v", c.Days)
	}
}

func TestCostOneMonthInvalidOtherHasData(t *testing.T) {
	// 一个月 40003、另一个月有数据 → 用有数据的结果（不整体失败）
	srv := costServer(t, map[int]string{
		8: `{"code":40003}`,
		7: costJSON("2026-07-31", 2.0),
	}, 0)
	defer srv.Close()

	repo := &DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()}
	repo.Now = func() time.Time { return time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC) }

	c, err := repo.Cost("tok")
	if err != nil {
		t.Fatalf("一个月失败一个月有数据不应整体失败: %v", err)
	}
	if len(c.Days) != 1 || c.Days[0].Total != 2.0 {
		t.Errorf("应保留有数据的月份: %+v", c.Days)
	}
}

func TestCostBothMonthsFailExplicit(t *testing.T) {
	srv := costServer(t, map[int]string{8: `{"code":40003}`, 7: `{"code":40003}`}, 0)
	defer srv.Close()

	repo := &DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()}
	repo.Now = func() time.Time { return time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC) }

	_, err := repo.Cost("tok")
	if err == nil || err.Error() != "DeepSeek 平台登录已失效，请更新平台 Token" {
		t.Errorf("两月 40003 应显式报 token 失效，got %v", err)
	}
}

func TestCostBothMonthsEmptyDataExplicit(t *testing.T) {
	// 两个月都成功但无任何天数 → 不返回误导零数据，报「消费数据为空」
	srv := costServer(t, map[int]string{
		8: `{"code":0,"data":{"biz_data":[]}}`,
		7: `{"code":0,"data":{"biz_data":[]}}`,
	}, 0)
	defer srv.Close()

	repo := &DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()}
	repo.Now = func() time.Time { return time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC) }

	_, err := repo.Cost("tok")
	if err == nil || err.Error() != "消费数据为空" {
		t.Errorf("两月无数据应报「消费数据为空」，got %v", err)
	}
}

func TestCostCrossYearMonthParams(t *testing.T) {
	// 1 月时上月为前一年 12 月：month/year 参数必须来自 minusMonths 之后的同一时间
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RawQuery)
		w.Write([]byte(`{"code":0,"data":{"biz_data":[]}}`))
	}))
	defer srv.Close()

	repo := &DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()}
	repo.Now = func() time.Time { return time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC) }

	if _, err := repo.Cost("tok"); err == nil || err.Error() != "消费数据为空" {
		t.Fatalf("空数据应报「消费数据为空」，got %v", err)
	}
	if len(requests) != 2 || requests[0] != "month=1&year=2026" || requests[1] != "month=12&year=2025" {
		t.Errorf("跨年月参数不符: %v", requests)
	}
}

// ── Zen billing ───────────────────────────────────────────────

func TestZenBillingOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspace/wrk_TEST/billing" {
			t.Errorf("路径不符: %s", r.URL.Path)
		}
		if r.Header.Get("Cookie") != "auth=test-cookie" {
			t.Errorf("Cookie 头不符: %s", r.Header.Get("Cookie"))
		}
		if r.Header.Get("User-Agent") != UA {
			t.Errorf("UA 头不符: %s", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(readFixture(t, "billing.html")))
	}))
	defer srv.Close()

	repo := &OpenCodeRepo{BaseURL: srv.URL, Client: srv.Client()}
	acc := models.Account{ID: "1", Name: "n", GoApiKey: "k", WorkspaceId: "wrk_TEST", AuthCookie: "test-cookie"}
	b, err := repo.ZenBilling(acc)
	if err != nil {
		t.Fatal(err)
	}
	if diffF(b.BalanceUsd, 19.9996075) > 1e-6 || b.MonthlyLimitUsd != 50.0 || !b.AutoReload {
		t.Errorf("zen billing 解析不符: %+v", b)
	}
}

func TestZenBilling401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	repo := &OpenCodeRepo{BaseURL: srv.URL, Client: srv.Client()}
	acc := models.Account{ID: "1", WorkspaceId: "w", AuthCookie: "c"}
	_, err := repo.ZenBilling(acc)
	if err == nil || err.Error() != "Cookie 已过期，请更新 Auth Cookie" {
		t.Errorf("401 应返回逐字错误「Cookie 已过期，请更新 Auth Cookie」，got %v", err)
	}
}

func TestZenBillingNotConfigured(t *testing.T) {
	repo := NewOpenCodeRepo()
	_, err := repo.ZenBilling(models.Account{ID: "1"})
	if err == nil || err.Error() != "未配置 Workspace/Cookie" {
		t.Errorf("未配置应返回逐字错误「未配置 Workspace/Cookie」，got %v", err)
	}
}

// 小工具：避免引入 strconv/fmt 重名干扰
func diffF(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
