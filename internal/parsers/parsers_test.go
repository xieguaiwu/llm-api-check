package parsers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixturePath 读取项目根 testdata/fixtures 下的文件（从 Android 项目复制）。
// go test 的工作目录是包目录，故相对路径为 ../../testdata/fixtures。
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "fixtures", name)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture 缺失（请先复制 Android fixtures）: %v", err)
	}
	return p
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// ── Go Usage ──────────────────────────────────────────────────
// 对照 Android GoUsageParserTest

func TestParseGoUsageReal(t *testing.T) {
	u, err := ParseGoUsage(readFixture(t, "go_usage.json"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Rolling == nil || u.Rolling.Status != "ok" {
		t.Errorf("rolling.status 应为 ok，got %+v", u.Rolling)
	}
	if u.Rolling.Percent != 0 {
		t.Errorf("rolling.percent 应为 0，got %d", u.Rolling.Percent)
	}
	if u.Rolling.ResetsAt != "2026-08-14T16:20:08.884Z" {
		t.Errorf("rolling.resetsAt 不符: %s", u.Rolling.ResetsAt)
	}
	if u.Monthly == nil || u.Monthly.Percent != 100 || u.Monthly.Status != "rate-limited" {
		t.Errorf("monthly 应为 100/rate-limited，got %+v", u.Monthly)
	}
	if u.Weekly == nil || u.Weekly.ResetsAt != "2026-08-17T00:00:00.884Z" || u.Weekly.Percent != 0 {
		t.Errorf("weekly 不符: %+v", u.Weekly)
	}
}

func TestParseGoUsageInvalidJSON(t *testing.T) {
	if _, err := ParseGoUsage("{bad"); err == nil {
		t.Error("非法 JSON 应报错")
	}
}

func TestParseGoUsageMissingWindows(t *testing.T) {
	u, err := ParseGoUsage(`{"usage":{"rolling":{"status":"ok","percent":1,"resetsAt":"x"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if u.Weekly != nil || u.Monthly != nil {
		t.Error("缺失窗口应为 nil")
	}
	if u.Rolling == nil || u.Rolling.Percent != 1 {
		t.Errorf("rolling 应保留: %+v", u.Rolling)
	}
}

// ── Zen Billing ───────────────────────────────────────────────
// 对照 Android ZenBillingParserTest

func TestParseZenBillingReal(t *testing.T) {
	b, err := ParseZenBilling(readFixture(t, "billing.html"))
	if err != nil {
		t.Fatal(err)
	}
	// fixture 实测值：balance:1999960750 → $19.9996075；monthlyUsage:39250 → $0.0003925；monthlyLimit:50
	if d := b.BalanceUsd - 19.9996075; d > 1e-6 || d < -1e-6 {
		t.Errorf("balanceUsd 应为 19.9996075，got %v", b.BalanceUsd)
	}
	if d := b.MonthlyUsageUsd - 0.0003925; d > 1e-9 || d < -1e-9 {
		t.Errorf("monthlyUsageUsd 应为 0.0003925，got %v", b.MonthlyUsageUsd)
	}
	if d := b.MonthlyLimitUsd - 50.0; d > 1e-6 || d < -1e-6 {
		t.Errorf("monthlyLimitUsd 应为 50，got %v", b.MonthlyLimitUsd)
	}
	if !b.AutoReload {
		t.Error("reload:!0 应解析为 autoReload=true")
	}
	if d := b.ReloadAmountUsd - 10.0; d > 1e-6 || d < -1e-6 {
		t.Errorf("reloadAmountUsd 应为 10，got %v", b.ReloadAmountUsd)
	}
	if d := b.ReloadTriggerUsd - 5.0; d > 1e-6 || d < -1e-6 {
		t.Errorf("reloadTriggerUsd 应为 5，got %v", b.ReloadTriggerUsd)
	}
}

func TestParseZenBillingNoCustomerID(t *testing.T) {
	_, err := ParseZenBilling("<html>login page</html>")
	if err == nil || !strings.Contains(err.Error(), "会话") {
		t.Errorf("应返回会话过期错误，got %v", err)
	}
}

func TestParseZenBillingMissingFieldTolerant(t *testing.T) {
	html2 := strings.Replace(readFixture(t, "billing.html"), "monthlyUsage:39250", "monthlyUsage:null", 1)
	b, err := ParseZenBilling(html2)
	if err != nil {
		t.Fatal(err)
	}
	if b.MonthlyUsageUsd != 0.0 {
		t.Errorf("monthlyUsageUsd 应为 0.0，got %v", b.MonthlyUsageUsd)
	}
	if d := b.BalanceUsd - 19.9996075; d > 1e-6 || d < -1e-6 {
		t.Errorf("balanceUsd 应为 19.9996075，got %v", b.BalanceUsd)
	}
}

func TestParseZenBillingAllCoreFieldsMissing(t *testing.T) {
	html2 := readFixture(t, "billing.html")
	html2 = strings.Replace(html2, "balance:1999960750", "balance:null", 1)
	html2 = strings.Replace(html2, "monthlyUsage:39250", "monthlyUsage:null", 1)
	html2 = strings.Replace(html2, "monthlyLimit:50", "monthlyLimit:null", 1)
	_, err := ParseZenBilling(html2)
	if err == nil || !strings.Contains(err.Error(), "结构") {
		t.Errorf("应返回页面结构变化错误，got %v", err)
	}
}

// 字符串字面量感知括号匹配：customerID 之前的字符串值内含 { 与 } 时，
// 解析器必须跳过字符串内容（Android 版对旧算法的修复）
func TestParseZenBillingBracesInsideStringLiteral(t *testing.T) {
	html := `window.__SSR={weird:"{not-a-brace",note:"}",customerID:"cus_TEST",balance:100000000,monthlyLimit:1};`
	b, err := ParseZenBilling(html)
	if err != nil {
		t.Fatal(err)
	}
	if d := b.BalanceUsd - 1.0; d > 1e-9 || d < -1e-9 {
		t.Errorf("balanceUsd 应为 1.0（100000000 microcents），got %v", b.BalanceUsd)
	}
}

// ── DeepSeek 余额 ─────────────────────────────────────────────
// 对照 Android DeepSeekParserTest

func TestParseDeepSeekBalanceReal(t *testing.T) {
	b, err := ParseDeepSeekBalance(readFixture(t, "deepseek_balance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !b.IsAvailable {
		t.Error("is_available 应为 true")
	}
	if len(b.Infos) != 1 {
		t.Fatalf("infos 应有 1 条，got %d", len(b.Infos))
	}
	info := b.Infos[0]
	if info.Currency != "CNY" {
		t.Errorf("currency 应为 CNY，got %s", info.Currency)
	}
	if d := info.TotalBalance - 120.0; d > 1e-6 || d < -1e-6 {
		t.Errorf("totalBalance 应为 120.0，got %v", info.TotalBalance)
	}
	if d := info.GrantedBalance - 0.0; d > 1e-6 || d < -1e-6 {
		t.Errorf("grantedBalance 应为 0.0，got %v", info.GrantedBalance)
	}
	if d := info.ToppedUpBalance - 120.0; d > 1e-6 || d < -1e-6 {
		t.Errorf("toppedUpBalance 应为 120.0，got %v", info.ToppedUpBalance)
	}
}

func TestParseDeepSeekBalanceUnavailable(t *testing.T) {
	b, err := ParseDeepSeekBalance(`{"is_available":false,"balance_infos":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	if b.IsAvailable || len(b.Infos) != 0 {
		t.Errorf("is_available=false 且 infos 空，got %+v", b)
	}
}

func TestParseDeepSeekBalanceBadAmountTolerant(t *testing.T) {
	b, err := ParseDeepSeekBalance(`{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"abc","granted_balance":"0","topped_up_balance":"1.5"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if b.Infos[0].TotalBalance != 0.0 || b.Infos[0].ToppedUpBalance != 1.5 {
		t.Errorf("非数字金额应兜底 0.0: %+v", b.Infos[0])
	}
}

// ── DeepSeek 消费明细 ─────────────────────────────────────────

func refDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// diff 两个浮点数差的绝对值
func diff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func TestParseDeepSeekCostReal(t *testing.T) {
	c, err := ParseDeepSeekCost(readFixture(t, "deepseek_cost.json"), refDate(t, "2026-08-14"))
	if err != nil {
		t.Fatal(err)
	}
	if d := c.Today - 4.5; d > 1e-6 || d < -1e-6 {
		t.Errorf("today 应为 4.5（1.5+2.5+0.5），got %v", c.Today)
	}
	if d := c.Last7d - 4.8; d > 1e-6 || d < -1e-6 {
		t.Errorf("last7d 应为 4.8（+0.3），got %v", c.Last7d)
	}
	if d := c.Last30d - 4.8; d > 1e-6 || d < -1e-6 {
		t.Errorf("last30d 应为 4.8，got %v", c.Last30d)
	}
	if len(c.Days) != 2 {
		t.Fatalf("days 应有 2 条，got %d", len(c.Days))
	}
	if c.Days[0].Date != "2026-08-14" || diff(c.Days[0].Total, 4.5) > 1e-6 {
		t.Errorf("days[0] 不符: %+v", c.Days[0])
	}
	if c.Days[1].Date != "2026-08-13" || diff(c.Days[1].Total, 0.3) > 1e-6 {
		t.Errorf("days[1] 不符: %+v", c.Days[1])
	}
}

func TestParseDeepSeekCostCode40003(t *testing.T) {
	_, err := ParseDeepSeekCost(`{"code":40003}`, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "失效") {
		t.Errorf("code 40003 应报 token 失效错误，got %v", err)
	}
}

func TestParseDeepSeekCostCodeNonZero(t *testing.T) {
	_, err := ParseDeepSeekCost(`{"code":5}`, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "code=5") {
		t.Errorf("code=5 应报接口错误（code=5），got %v", err)
	}
}

func TestParseDeepSeekCostMissingCodeOK(t *testing.T) {
	// code 字段缺失（对应 intOrNull → null）→ 不报错，正常解析
	raw := `{"data":{"biz_data":[{"days":[{"date":"2026-08-14","data":[{"model":"m","usage":[{"type":"input","amount":1.0}]}]}]}]}}`
	c, err := ParseDeepSeekCost(raw, refDate(t, "2026-08-14"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Today != 1.0 {
		t.Errorf("today 应为 1.0，got %v", c.Today)
	}
}

// ── AggregateCost ─────────────────────────────────────────────
// 对照 Android DeepSeekAggregationTest

func TestAggregateCost30DaysNotTruncated(t *testing.T) {
	ref := refDate(t, "2026-08-14")
	m := map[string]float64{}
	for i := 0; i < 30; i++ {
		m[ref.AddDate(0, 0, -i).Format("2006-01-02")] = 1.0
	}
	c := AggregateCost(m, ref)
	if c.Today != 1.0 || c.Last7d != 7.0 || c.Last30d != 30.0 {
		t.Errorf("聚合不符: today=%v last7d=%v last30d=%v", c.Today, c.Last7d, c.Last30d)
	}
	if len(c.Days) != 30 {
		t.Errorf("days 不应被截断到 7 条，got %d", len(c.Days))
	}
}

func TestParseDeepSeekCostKeepsAllDays(t *testing.T) {
	// 构造 20 天数据：7 月 26 日 - 8 月 14 日（跨月 JSON）
	ref := refDate(t, "2026-08-14")
	var sb strings.Builder
	sb.WriteString(`{"code":0,"data":{"biz_data":[{"days":[`)
	for i := 19; i >= 0; i-- {
		if i != 19 {
			sb.WriteString(",")
		}
		d := ref.AddDate(0, 0, -i).Format("2006-01-02")
		sb.WriteString(`{"date":"` + d + `","data":[{"model":"deepseek-chat","usage":[{"type":"input","amount":0.5}]}]}`)
	}
	sb.WriteString(`]}]}}`)
	c, err := ParseDeepSeekCost(sb.String(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Days) != 20 {
		t.Errorf("解析器应保留全部天数，got %d", len(c.Days))
	}
	if d := c.Last30d - 10.0; d > 1e-6 || d < -1e-6 {
		t.Errorf("last30d 应为 10.0（0.5×20 天），got %v", c.Last30d)
	}
	if d := c.Last7d - 3.5; d > 1e-6 || d < -1e-6 {
		t.Errorf("last7d 应为 3.5（0.5×7 天），got %v", c.Last7d)
	}
}

func TestAggregateCostEmpty(t *testing.T) {
	c := AggregateCost(map[string]float64{}, refDate(t, "2026-08-14"))
	if c.Today != 0.0 || c.Last30d != 0.0 || len(c.Days) != 0 {
		t.Errorf("空消费应返回空聚合不崩溃: %+v", c)
	}
}

func TestParseDeepSeekCostZeroRefDateUsesToday(t *testing.T) {
	// refDate 零值 → 今天：今天日期键有数据时 today 非零
	key := time.Now().Format("2006-01-02")
	raw := `{"code":0,"data":{"biz_data":[{"days":[{"date":"` + key + `","data":[{"model":"m","usage":[{"type":"input","amount":2.0}]}]}]}]}}`
	c, err := ParseDeepSeekCost(raw, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Today != 2.0 {
		t.Errorf("零值 refDate 应取今天：today=%v", c.Today)
	}
}
