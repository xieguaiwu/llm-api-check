package render

import (
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

// ── FormatCountdown（对照 Android countdownText 全分支） ───────

func baseTime() time.Time {
	return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
}

func TestCountdown(t *testing.T) {
	now := baseTime()
	cases := []struct {
		resetsAt string
		want     string
	}{
		// 解析失败 → 即将重置
		{"not-a-time", "即将重置"},
		// 已过期 → 即将重置
		{now.Add(-1 * time.Minute).Format(time.RFC3339), "即将重置"},
		// 30 秒后（不足 1 分钟）→ 即将重置
		{now.Add(30 * time.Second).Format(time.RFC3339), "即将重置"},
		// 52 分钟后
		{now.Add(52 * time.Minute).Format(time.RFC3339), "52分钟后重置"},
		// 4 小时 20 分后
		{now.Add(4*time.Hour + 20*time.Minute).Format(time.RFC3339), "4小时20分后重置"},
		// 整 2 小时 → 省略分钟
		{now.Add(2 * time.Hour).Format(time.RFC3339), "2小时后重置"},
		// 2 天后（>24h 仍用小时/分钟格式）
		{now.Add(48 * time.Hour).Format(time.RFC3339), "48小时后重置"},
	}
	for _, tc := range cases {
		if got := FormatCountdown(tc.resetsAt, now); got != tc.want {
			t.Errorf("FormatCountdown(%q) = %q, want %q", tc.resetsAt, got, tc.want)
		}
	}
	// 59 分钟 → 分钟格式
	if got := FormatCountdown(now.Add(59*time.Minute).Format(time.RFC3339), now); got != "59分钟后重置" {
		t.Errorf("59min: got %q", got)
	}
}

func TestCountdownMillisecondsAccepted(t *testing.T) {
	// fixture 带毫秒（.884Z），Go RFC3339 解析容忍小数秒
	got := FormatCountdown("2026-08-14T16:20:08.884Z", baseTime())
	if got == "" {
		t.Error("空输出")
	}
}

// ── CurrencySymbol ────────────────────────────────────────────

func TestCurrencySymbol(t *testing.T) {
	cases := map[string]string{
		"CNY": "¥", "cny": "¥", "RMB": "¥", "USD": "$", "EUR": "€", "GBP": "GBP ",
	}
	for in, want := range cases {
		if got := CurrencySymbol(in); got != want {
			t.Errorf("CurrencySymbol(%q) = %q, want %q", in, got, want)
		}
	}
}

// ── ColorForPercent ───────────────────────────────────────────

func TestColorForPercent(t *testing.T) {
	if ColorForPercent(42, false) != ColorBlue {
		t.Error("<70 应为蓝")
	}
	if ColorForPercent(70, false) != ColorYellow {
		t.Error("70 应为黄")
	}
	if ColorForPercent(89, false) != ColorYellow {
		t.Error("70-89 应为黄")
	}
	if ColorForPercent(90, false) != ColorRed {
		t.Error("≥90 应为红")
	}
	if ColorForPercent(10, true) != ColorRed {
		t.Error("限流强制红")
	}
}

// ── UsageBar ──────────────────────────────────────────────────

func TestUsageBar(t *testing.T) {
	cases := []struct {
		percent int
		want    string
	}{
		{0, "[░░░░░░░░░░]"},
		{42, "[████░░░░░░]"},
		{100, "[██████████]"},
		{150, "[██████████]"}, // 超限截断
		{-5, "[░░░░░░░░░░]"},
	}
	for _, tc := range cases {
		if got := UsageBar(tc.percent, 10); got != tc.want {
			t.Errorf("UsageBar(%d, 10) = %q, want %q", tc.percent, got, tc.want)
		}
	}
}

// ── Colorizer ─────────────────────────────────────────────────

func TestColorizerDisabledNoANSI(t *testing.T) {
	c := Colorizer{Disabled: true}
	for _, s := range []string{c.Blue("x"), c.Yellow("x"), c.Red("x"), c.Green("x"), c.Gray("x")} {
		if strings.Contains(s, "\x1b") {
			t.Errorf("禁用颜色时不应含 ANSI: %q", s)
		}
	}
}

func TestColorizerEnabledHasANSI(t *testing.T) {
	c := Colorizer{}
	if !strings.Contains(c.Red("x"), "\x1b[31m") {
		t.Errorf("启用颜色时应含 ANSI 红色码: %q", c.Red("x"))
	}
}

// ── 总览 / 详情渲染 ───────────────────────────────────────────

func TestRenderOverviewEmpty(t *testing.T) {
	got := RenderOverview(nil, nil, time.Time{}, Colorizer{Disabled: true})
	if !strings.Contains(got, "未配置任何账号") {
		t.Errorf("空配置应提示添加账号: %q", got)
	}
}

func TestRenderOverviewWithData(t *testing.T) {
	ds := []app.DeepSeekResult{{
		Account: models.DeepSeekAccount{ID: "d1", Name: "测试", ApiKey: "k", PlatformToken: "t"},
		Balance: &models.DeepSeekBalance{IsAvailable: true, Infos: []models.DeepSeekBalanceInfo{
			{Currency: "CNY", TotalBalance: 120, ToppedUpBalance: 120, GrantedBalance: 0},
		}},
		Cost: &models.DeepSeekCost{Today: 1.2, Last7d: 3.0, Last30d: 5.5},
	}}
	accs := []app.AccountResult{{
		Account: models.Account{ID: "a1", Name: "opencode", GoApiKey: "g", WorkspaceId: "w", AuthCookie: "c"},
		GoUsage: &models.GoUsage{
			Rolling: &models.GoWindow{Status: "ok", Percent: 42},
			Weekly:  &models.GoWindow{Status: "ok", Percent: 17},
			Monthly: &models.GoWindow{Status: "rate-limited", Percent: 100},
		},
		ZenBilling: &models.ZenBilling{BalanceUsd: 19.99},
	}}
	got := RenderOverview(ds, accs, baseTime(), Colorizer{Disabled: true})
	for _, want := range []string{
		"LLM API Check — 更新于 12:00",
		"DeepSeek (测试)",
		"余额 ¥120.00 · 充值 ¥120.00 · 赠送 ¥0.00",
		"今日 ¥1.20 · 7日 ¥3.00 · 30日 ¥5.50",
		"OpenCode (opencode)",
		"R 42% [████░░░░░░]",
		"W 17% [██░░░░░░░░]",
		"已限流",
		"Zen $19.99",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("总览缺少 %q:\n%s", want, got)
		}
	}
}

func TestRenderAccountDetail(t *testing.T) {
	now := baseTime()
	r := app.AccountResult{
		Account: models.Account{ID: "a1", Name: "主账号", GoApiKey: "g", WorkspaceId: "w", AuthCookie: "c"},
		GoUsage: &models.GoUsage{
			Rolling: &models.GoWindow{Status: "ok", Percent: 42, ResetsAt: now.Add(4*time.Hour + 20*time.Minute).Format(time.RFC3339)},
			Weekly:  &models.GoWindow{Status: "ok", Percent: 17, ResetsAt: now.Add(52 * time.Minute).Format(time.RFC3339)},
			Monthly: &models.GoWindow{Status: "rate-limited", Percent: 100, ResetsAt: now.Add(time.Hour).Format(time.RFC3339)},
		},
		ZenBilling: &models.ZenBilling{
			BalanceUsd: 19.99, MonthlyUsageUsd: 0.0, MonthlyLimitUsd: 50.0,
			AutoReload: true, ReloadAmountUsd: 20, ReloadTriggerUsd: 5,
		},
	}
	got := RenderAccountDetail(r, now, Colorizer{Disabled: true})
	for _, want := range []string{
		"主账号 (OpenCode)",
		"Go Plan · 订阅",
		"Rolling 5h",
		"4小时20分后重置",
		"52分钟后重置",
		"已限流", // rate-limited → 红「已限流」替代倒计时
		"Zen Plan · 按量",
		"余额 $19.99",
		"本月 $0.00 / $50.00 [░░░░░░░░░░] 0%",
		"自动充值 开 · 低于 $5.00 充 $20.00",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("详情缺少 %q:\n%s", want, got)
		}
	}
	// rate-limited 行不显示倒计时
	if strings.Contains(got, "1小时后重置") {
		t.Errorf("限流行应显示「已限流」而非倒计时:\n%s", got)
	}
}

func TestRenderAccountDetailNoZen(t *testing.T) {
	r := app.AccountResult{
		Account: models.Account{ID: "a1", Name: "无Zen", GoApiKey: "g"},
		GoUsage: &models.GoUsage{Rolling: &models.GoWindow{Status: "ok", Percent: 0, ResetsAt: "x"}},
	}
	got := RenderAccountDetail(r, baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got, "未配置 Workspace ID / Cookie") {
		t.Errorf("未配置 Zen 应灰字提示: %q", got)
	}
}

func TestRenderAccountDetailError(t *testing.T) {
	r := app.AccountResult{
		Account: models.Account{ID: "a1", Name: "坏账号", GoApiKey: "g"},
		Error:   "Go API Key 无效或已过期",
	}
	got := RenderAccountDetail(r, baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got, "Go API Key 无效或已过期") {
		t.Errorf("错误卡应显示错误: %q", got)
	}
}

func TestRenderDeepSeekDetail(t *testing.T) {
	r := app.DeepSeekResult{
		Account: models.DeepSeekAccount{ID: "d1", Name: "测试", ApiKey: "k", PlatformToken: "t"},
		Balance: &models.DeepSeekBalance{IsAvailable: true, Infos: []models.DeepSeekBalanceInfo{
			{Currency: "CNY", TotalBalance: 120, ToppedUpBalance: 120, GrantedBalance: 0},
		}},
		Cost: &models.DeepSeekCost{Today: 1.2, Last7d: 3.0, Last30d: 5.5},
	}
	got := RenderDeepSeekDetail(r, Colorizer{Disabled: true})
	for _, want := range []string{
		"测试 (DeepSeek)",
		"余额 ¥120.00",
		"充值 ¥120.00 · 赠送 ¥0.00",
		"今日 ¥1.20 · 7日 ¥3.00 · 30日 ¥5.50",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("详情缺少 %q:\n%s", want, got)
		}
	}

	// 无 token → 灰字提示
	r2 := app.DeepSeekResult{
		Account: models.DeepSeekAccount{ID: "d2", Name: "无Token", ApiKey: "k"},
		Balance: &models.DeepSeekBalance{IsAvailable: true, Infos: []models.DeepSeekBalanceInfo{
			{Currency: "CNY", TotalBalance: 1},
		}},
	}
	got2 := RenderDeepSeekDetail(r2, Colorizer{Disabled: true})
	if !strings.Contains(got2, "未配置平台 Token，仅显示余额") {
		t.Errorf("无 token 应提示仅显示余额: %q", got2)
	}
}
