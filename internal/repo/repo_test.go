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

// TestCostMonthEndOverflow 月末溢出回归（momus P1-1）：3/31、5/31 等日 AddDate 会归一化到
// 下月日期导致请求错月份（旧实现 5/31 → 上月请求 month=5 重复当月、金额翻倍）。
// 修复后用 day=1 构造上月，请求必须为正确的上月月份。
func TestCostMonthEndOverflow(t *testing.T) {
	cases := []struct {
		now   time.Time
		prevQ string // 上月请求参数
		curQ  string // 当月请求参数
	}{
		{time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), "month=2&year=2026", "month=3&year=2026"},
		{time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC), "month=4&year=2026", "month=5&year=2026"},
		{time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC), "month=6&year=2026", "month=7&year=2026"},
		{time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), "month=11&year=2026", "month=12&year=2026"},
		{time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), "month=12&year=2025", "month=1&year=2026"},
	}
	for _, tc := range cases {
		var requests []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.RawQuery)
			w.Write([]byte(`{"code":0,"data":{"biz_data":[]}}`))
		}))
		repo := &DeepSeekRepo{BaseBalanceURL: srv.URL, BaseCostURL: srv.URL, Client: srv.Client()}
		repo.Now = func() time.Time { return tc.now }
		if _, err := repo.Cost("tok"); err == nil || err.Error() != "消费数据为空" {
			t.Fatalf("%s: 空数据应报「消费数据为空」，got %v", tc.now.Format("2006-01-02"), err)
		}
		if len(requests) != 2 || requests[0] != tc.curQ || requests[1] != tc.prevQ {
			t.Errorf("%s: 请求参数不符，want [%s %s] got %v", tc.now.Format("2006-01-02"), tc.curQ, tc.prevQ, requests)
		}
		srv.Close()
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

// ── Qwen Token Plan ────────────────────────────────────────────

// qwenTestEndpoints 把三组端点全指向同一个 httptest 服务
func qwenTestEndpoints(srvURL string) *QwenEndpoints {
	return &QwenEndpoints{
		Gateway:       srvURL,
		Dashboard:     srvURL + "/cn-beijing?tab=plan",
		Quota:         srvURL + "/data/api.json",
		Action:        "BroadScopeAspnGateway",
		Region:        "cn-beijing",
		ConsoleSite:   "BAILIAN_ALIYUN",
		Domain:        "bailian.console.aliyun.com",
		Lang:          "zh-CN",
		CommodityCode: "sfm_tokenplansolo_public_cn",
		Origin:        srvURL,
	}
}

func TestQwenPlanOK(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/compatible-mode/v1/models" {
			t.Errorf("路径不符: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(readFixture(t, "qwen_models.json")))
	}))
	defer srv.Close()
	repo := &QwenRepo{Client: srv.Client(), Endpoints: &QwenEndpoints{Gateway: srv.URL}}
	plan, err := repo.Plan(models.QwenAccount{ApiKey: "sk-sp-test"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sk-sp-test" {
		t.Errorf("Authorization 头不符: %q", gotAuth)
	}
	if len(plan.Models) != 4 || plan.Models[0] != "deepseek-v4-flash-0731" {
		t.Errorf("模型清单不符: %v", plan.Models)
	}
}

func TestQwenPlan401MentionsRegion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	repo := &QwenRepo{Client: srv.Client(), Endpoints: &QwenEndpoints{Gateway: srv.URL}}
	_, err := repo.Plan(models.QwenAccount{ApiKey: "sk-sp-bad"})
	if err == nil || !strings.Contains(err.Error(), "区域") {
		t.Errorf("401 需提示区域绑定（实测同 key 换区域即 200）: %v", err)
	}
}

func TestQwenPlanNoKey(t *testing.T) {
	repo := &QwenRepo{Endpoints: &QwenEndpoints{Gateway: "https://example.invalid"}}
	if _, err := repo.Plan(models.QwenAccount{}); err == nil || !strings.Contains(err.Error(), "未配置 API Key") {
		t.Errorf("空 key 应直接提示未配置: %v", err)
	}
}

func TestQwenUsageRequiresCookie(t *testing.T) {
	repo := &QwenRepo{Endpoints: qwenTestEndpoints("https://example.invalid")}
	_, err := repo.Usage(models.QwenAccount{ApiKey: "sk-sp-x"})
	if err == nil || err.Error() != "未配置控制台 Cookie" {
		t.Errorf("未配 Cookie 应显式提示: %v", err)
	}
}

func TestQwenUsageOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/cn-beijing"):
			w.Write([]byte(`window.ALIYUN_CONSOLE_CONFIG = { SEC_TOKEN: "tok-from-html" };`))
		case r.URL.Path == "/data/api.json":
			_ = r.ParseForm()
			api := r.URL.Query().Get("api")
			switch api {
			case qwenAPIUsage:
				if st := r.PostForm.Get("sec_token"); st != "tok-from-html" {
					t.Errorf("sec_token 未从页面透传: %q", st)
				}
				if r.PostForm.Get("product") != "sfm_bailian" || r.PostForm.Get("action") != "BroadScopeAspnGateway" ||
					r.PostForm.Get("region") != "cn-beijing" {
					t.Errorf("表单体不符: %s", r.PostForm.Encode())
				}
				params := r.PostForm.Get("params")
				if !strings.Contains(params, `"Api":"`+qwenAPIUsage+`"`) {
					t.Errorf("params 缺 Api: %s", params)
				}
				if strings.Contains(params, "switchAgent") {
					t.Errorf("params 不得含 switchAgent（会绑死他人工作区）: %s", params)
				}
				if r.Header.Get("Cookie") != "login_aliyunid_csrf=csrf-1; cna=anon-1" {
					t.Errorf("Cookie 头不符: %q", r.Header.Get("Cookie"))
				}
				if r.Header.Get("x-xsrf-token") != "csrf-1" {
					t.Errorf("x-xsrf-token 应取 login_aliyunid_csrf: %q", r.Header.Get("x-xsrf-token"))
				}
				if r.Header.Get("Origin") == "" || r.Header.Get("Referer") == "" {
					t.Error("缺 Origin/Referer（网关同源校验）")
				}
				if !strings.Contains(params, "anon-1") {
					t.Errorf("cna 需作为 X-Anonymous-Id 进入 cornerstoneParam: %s", params)
				}
				w.Write([]byte(readFixture(t, "qwen_usage.json")))
			case qwenAPISubscription:
				if !strings.Contains(r.PostForm.Get("params"), "sfm_tokenplansolo_public_cn") {
					t.Errorf("订阅接口需带 commodityCode: %s", r.PostForm.Get("params"))
				}
				w.Write([]byte(readFixture(t, "qwen_subscription.json")))
			}
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	repo := &QwenRepo{Client: srv.Client(), Endpoints: qwenTestEndpoints(srv.URL), UsageRetryDelay: time.Millisecond}
	u, err := repo.Usage(models.QwenAccount{
		ApiKey: "sk-sp-x", ConsoleCookie: "Cookie: login_aliyunid_csrf=csrf-1; cna=anon-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if u.FiveHour == nil || u.FiveHour.Percent != 79 {
		t.Errorf("5 小时窗口不符: %+v", u.FiveHour)
	}
	if u.PlanCode != "lite" {
		t.Errorf("套餐档位不符: %q", u.PlanCode)
	}
}

func TestQwenUsageSecTokenFromCookieSkipsDashboard(t *testing.T) {
	dashboard := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/cn-beijing") {
			dashboard++
			t.Error("Cookie 内已有 sec_token 时不应再抓页面")
			return
		}
		w.Write([]byte(readFixture(t, "qwen_usage.json")))
	}))
	defer srv.Close()
	repo := &QwenRepo{Client: srv.Client(), Endpoints: qwenTestEndpoints(srv.URL), UsageRetryDelay: time.Millisecond}
	if _, err := repo.Usage(models.QwenAccount{ConsoleCookie: "sec_token=tok-ck; cna=a"}); err != nil {
		t.Fatal(err)
	}
	if dashboard != 0 {
		t.Errorf("不应请求 dashboard，实得 %d", dashboard)
	}
}

// 网关偶发返回「200 Success 但无窗口」，重试后成功
func TestQwenUsageRetriesEmptyEnvelope(t *testing.T) {
	usageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/data/api.json" && r.URL.Query().Get("api") == qwenAPIUsage:
			usageCalls++
			if usageCalls < 3 {
				w.Write([]byte(readFixture(t, "qwen_usage_empty.json")))
				return
			}
			w.Write([]byte(readFixture(t, "qwen_usage.json")))
		case r.URL.Path == "/data/api.json":
			w.Write([]byte(readFixture(t, "qwen_subscription.json")))
		default:
			w.Write([]byte(`SEC_TOKEN: "t";`))
		}
	}))
	defer srv.Close()
	repo := &QwenRepo{Client: srv.Client(), Endpoints: qwenTestEndpoints(srv.URL), UsageAttempts: 3, UsageRetryDelay: time.Millisecond}
	if _, err := repo.Usage(models.QwenAccount{ConsoleCookie: "sec_token=tok-ck"}); err != nil {
		t.Fatal(err)
	}
	if usageCalls != 3 {
		t.Errorf("应重试至第 3 次成功，实得 %d", usageCalls)
	}
}

// 登录失效重试无意义：必须只请求一次并抛 Cookie 错误
func TestQwenUsageLoginErrorNoRetry(t *testing.T) {
	usageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/data/api.json" && r.URL.Query().Get("api") == qwenAPIUsage {
			usageCalls++
		}
		w.Write([]byte(readFixture(t, "qwen_login_notlogined.json")))
	}))
	defer srv.Close()
	repo := &QwenRepo{Client: srv.Client(), Endpoints: qwenTestEndpoints(srv.URL), UsageAttempts: 3, UsageRetryDelay: time.Millisecond}
	_, err := repo.Usage(models.QwenAccount{ConsoleCookie: "sec_token=tok-ck"})
	if err == nil || !strings.Contains(err.Error(), "Cookie") {
		t.Errorf("应报 Cookie 失效: %v", err)
	}
	if usageCalls != 1 {
		t.Errorf("认证错误不得重试，实得 %d 次", usageCalls)
	}
}

func TestQwenUsagePersistentEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(readFixture(t, "qwen_usage_empty.json")))
	}))
	defer srv.Close()
	repo := &QwenRepo{Client: srv.Client(), Endpoints: qwenTestEndpoints(srv.URL), UsageAttempts: 2, UsageRetryDelay: time.Millisecond}
	_, err := repo.Usage(models.QwenAccount{ConsoleCookie: "sec_token=tok-ck"})
	if err == nil || !strings.Contains(err.Error(), "暂不可用") {
		t.Errorf("重试耗尽应抛「暂不可用」: %v", err)
	}
}

func TestQwenEndpointsFor(t *testing.T) {
	cn, err := QwenEndpointsFor("")
	if err != nil || cn.Gateway != "https://token-plan.cn-beijing.maas.aliyuncs.com" ||
		cn.Quota != "https://bailian-cs.console.aliyun.com/data/api.json" ||
		cn.Action != "BroadScopeAspnGateway" || cn.CommodityCode != "sfm_tokenplansolo_public_cn" {
		t.Errorf("默认区域端点不符: %+v %v", cn, err)
	}
	intl, err := QwenEndpointsFor("intl")
	if err != nil || intl.Gateway != "https://token-plan.ap-southeast-1.maas.aliyuncs.com" ||
		intl.Action != "IntlBroadScopeAspnGateway" || intl.Quota != "https://cs-data.qwencloud.com/data/api.json" {
		t.Errorf("国际区域端点不符: %+v %v", intl, err)
	}
	if _, err := QwenEndpointsFor("mars"); err == nil {
		t.Error("未知区域应报错")
	}
}

func TestCookieHelpers(t *testing.T) {
	if got := normalizeCookieHeader("  Cookie: a=1;  b=2\n"); got != "a=1; b=2" {
		t.Errorf("应剥掉 Cookie: 前缀并压平空白: %q", got)
	}
	// 大小写不敏感；但 `cookie=x` 形式的合法 Cookie（名为 cookie）不可误剥
	if got := normalizeCookieHeader("COOKIE: a=1"); got != "a=1" {
		t.Errorf("前缀大小写不敏感: %q", got)
	}
	if got := normalizeCookieHeader("cookie=a=1"); got != "cookie=a=1" {
		t.Errorf("名为 cookie 的条目应保持原样: %q", got)
	}
	h := "a=1; cna=xyz; sec_token=tok"
	if cookieValue(h, "cna") != "xyz" || cookieValue(h, "sec_token") != "tok" || cookieValue(h, "nope") != "" {
		t.Errorf("cookieValue 取值不符: %q", h)
	}
}

func TestQwenTraceIDShape(t *testing.T) {
	id := qwenTraceID()
	if len(id) != 36 || id[14] != '4' || !strings.Contains("89ab", string(id[19])) {
		t.Errorf("feTraceId 应为小写 UUIDv4: %q", id)
	}
	if qwenTraceID() == id {
		t.Error("两次 traceId 不应相同")
	}
}
