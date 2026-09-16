package render

import (
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

func TestRenderLongCatDetail(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "测试号", ApiKey: "sk-test123"},
		Plan: &models.LongCatPlan{
			Models: []models.LongCatModel{
				{ID: "LongCat-2.0", OwnedBy: "meituan"},
			},
		},
		Usage: &models.LongCatUsage{
			BalanceOK: &bal,
		},
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if !strings.Contains(out, "LongCat") {
		t.Errorf("output should mention LongCat: %q", out)
	}
	if !strings.Contains(out, "充足") {
		t.Errorf("output should show balance ok: %q", out)
	}
	if !strings.Contains(out, "LongCat-2.0") {
		t.Errorf("output should list models: %q", out)
	}
}

func TestRenderLongCatDetailNoKey(t *testing.T) {
	c := Colorizer{Disabled: true}
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "测试号"},
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if !strings.Contains(out, "未配置 API Key") {
		t.Errorf("output should prompt for key: %q", out)
	}
}

func TestRenderLongCatDetailEmptyBalance(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := false
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "测试号", ApiKey: "sk-test"},
		Usage: &models.LongCatUsage{
			BalanceOK: &bal,
		},
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if !strings.Contains(out, "不足") {
		t.Errorf("output should show insufficient balance: %q", out)
	}
}

func TestRenderLongCatDetailError(t *testing.T) {
	c := Colorizer{Disabled: true}
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "测试号", ApiKey: "bad-key"},
		Error:   "API Key 无效",
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if !strings.Contains(out, "API Key 无效") {
		t.Errorf("output should show error: %q", out)
	}
}

func TestRenderOverviewLongCatOnly(t *testing.T) {
	// 缺陷回归：只配 LongCat 账号时总览必须显示 LongCat 段，不能显示「未配置任何账号」
	c := Colorizer{Disabled: true}
	res := app.Result{
		LongCat: []app.LongCatResult{
			{
				Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test"},
				Plan:    &models.LongCatPlan{Models: []models.LongCatModel{{ID: "LongCat-2.0"}}},
			},
		},
	}
	out := RenderOverview(res, time.Now(), c)
	if strings.Contains(out, "未配置任何账号") {
		t.Errorf("只配 LongCat 不应显示「未配置任何账号」: %q", out)
	}
	if !strings.Contains(out, "LongCat (龙猫)") {
		t.Errorf("总览应含 LongCat 段: %q", out)
	}
}

func TestRenderOverviewLongCat(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	res := app.Result{
		LongCat: []app.LongCatResult{
			{
				Account: models.LongCatAccount{Name: "测试号", ApiKey: "sk-test"},
				Plan: &models.LongCatPlan{
					Models: []models.LongCatModel{{ID: "LongCat-2.0"}},
				},
				Usage: &models.LongCatUsage{
					BalanceOK: &bal,
				},
			},
			// Add a non-longcat account so RenderOverview doesn't short-circuit
			// with "未配置任何账号" (it checks all account types)
		},
		DeepSeek: []app.DeepSeekResult{
			{Account: models.DeepSeekAccount{Name: "dummy", ApiKey: "sk-dummy"}},
		},
	}
	out := RenderOverview(res, time.Now(), c)
	if !strings.Contains(out, "LongCat") {
		t.Errorf("overview should include LongCat: %q", out)
	}
}

// 控制台配额测试 fixture（2026-09-16 实测形状）。
var (
	lcHappyQuota = &models.LongCatQuota{
		CurrentLot: &models.LongCatLot{
			RemainingToken: 1234567,
			TotalToken:     5000000,
			ConsumedToken:  3765433,
			ConsumedRatio:  0.753,
			ExpireTime:     1760342400000,
			RemainSeconds:  2332800,
			GrantCategory:  "GIFT",
		},
		Estimate: &models.LongCatEstimate{
			WindowDays:         7,
			DailyAverageToken:  12345,
			ExhaustedAfterDays: 100,
		},
	}
	lcZeroPaygo = &models.LongCatPaygo{
		PaygoBalance: &models.LongCatPaygoBalance{
			Primary: &models.LongCatPaygoAmount{Currency: "CNY", Amount: "0.00"},
		},
	}
)

func TestRenderLongCatDetailWithQuota(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		Plan: &models.LongCatPlan{
			Models: []models.LongCatModel{
				{ID: "LongCat-2.0", OwnedBy: "meituan", DisplayName: "LongCat 2.0", ContextWindow: 1048576, MaxOutputTokens: 131072},
			},
		},
		Usage: &models.LongCatUsage{BalanceOK: &bal},
		Quota: lcHappyQuota,
		Paygo: lcZeroPaygo,
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	for _, want := range []string{
		"余额", "充足",
		"Token", "剩余 1,234,567 / 共 5,000,000（已用 75.3%）",
		"有效期", "2025-10-13", "剩 27 天",
		"日均消耗", "12,345", "按当前速率约 100 天后耗尽",
		"按量余额", "¥0.00",
		"模型", "LongCat 2.0（上下文 1M · 输出 128K）",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("输出应含 %q:\n%s", want, out)
		}
	}
}

func TestRenderLongCatDetailNoPack(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := false
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		Usage:   &models.LongCatUsage{BalanceOK: &bal},
		Quota:   &models.LongCatQuota{CurrentLot: nil, Estimate: &models.LongCatEstimate{}},
		Paygo:   lcZeroPaygo,
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if !strings.Contains(out, "无 Token 资源包") {
		t.Errorf("无资源包应显示「无 Token 资源包」:\n%s", out)
	}
	if !strings.Contains(out, "每日免费额度平台未公开，不含在内") {
		t.Errorf("无资源包且余额为 0 应补免费额度灰字:\n%s", out)
	}
}

func TestRenderLongCatDetailNoCookie(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test"}, // 无 Cookie
		Plan:    &models.LongCatPlan{Models: []models.LongCatModel{{ID: "LongCat-2.0"}}},
		Usage:   &models.LongCatUsage{BalanceOK: &bal},
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if !strings.Contains(out, "--console-cookie") {
		t.Errorf("未配 Cookie 应给灰字指引:\n%s", out)
	}
	if strings.Contains(out, "Token 剩余") {
		t.Errorf("未配 Cookie 不应有 Token 段:\n%s", out)
	}
}

func TestRenderLongCatDetailConsoleError(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=stale"},
		Plan:    &models.LongCatPlan{Models: []models.LongCatModel{{ID: "LongCat-2.0"}}},
		Usage:   &models.LongCatUsage{BalanceOK: &bal},
		Error:   "LongCat 控制台会话已失效，请重新从浏览器复制 Cookie",
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if !strings.Contains(out, "控制台会话已失效") {
		t.Errorf("应显示控制台错误:\n%s", out)
	}
	if !strings.Contains(out, "LongCat-2.0") {
		t.Errorf("Cookie 通道失败不应抖掉模型清单:\n%s", out)
	}
}

func TestRenderLongCatOverviewWithQuota(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	res := app.Result{
		LongCat: []app.LongCatResult{
			{
				Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
				Plan:    &models.LongCatPlan{Models: []models.LongCatModel{{ID: "LongCat-2.0"}}},
				Usage:   &models.LongCatUsage{BalanceOK: &bal},
				Quota:   lcHappyQuota,
			},
		},
	}
	out := RenderOverview(res, time.Now(), c)
	if !strings.Contains(out, "Token 剩余 1,234,567（已用 75.3%）") {
		t.Errorf("总览应含 Token 剩余:\n%s", out)
	}
}

func TestFormatTokenCount(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{1048576, "1M"},
		{131072, "128K"},
		{150000, "150,000"}, // 非整数倍用原始数字
		{1024, "1K"},
		{0, "0"},
		{2097152, "2M"},
	}
	for _, tc := range cases {
		if got := formatTokenCount(tc.in); got != tc.want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// 复审修复测试：Fix 3 提示条件（无资源包 且 按量余额缺失或为 0 → 灰字；余额>0 → 灰字消失）
func TestRenderLongCatDetailHintNoLotZeroPaygo(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := false
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		Usage:   &models.LongCatUsage{BalanceOK: &bal},
		Quota:   &models.LongCatQuota{CurrentLot: nil},
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if !strings.Contains(out, "每日免费额度平台未公开，不含在内") {
		t.Errorf("无资源包+无 paygo 应显示免费额度灰字:\n%s", out)
	}
}

func TestRenderLongCatDetailHintNoLotWithPaygo(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		Usage:   &models.LongCatUsage{BalanceOK: &bal},
		Quota:   &models.LongCatQuota{CurrentLot: nil},
		Paygo: &models.LongCatPaygo{
			PaygoBalance: &models.LongCatPaygoBalance{Primary: &models.LongCatPaygoAmount{Currency: "CNY", Amount: "12.50"}},
		},
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if strings.Contains(out, "每日免费额度平台未公开") {
		t.Errorf("无资源包但 paygo>0 不应显示免费额度灰字:\n%s", out)
	}
}

func TestRenderLongCatDetailHintHasLot(t *testing.T) {
	c := Colorizer{Disabled: true}
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		Quota: &models.LongCatQuota{CurrentLot: &models.LongCatLot{
			RemainingToken: 1000000, TotalToken: 5000000, ConsumedRatio: 0.8,
		}},
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	if strings.Contains(out, "每日免费额度平台未公开") {
		t.Errorf("有资源包不应显示免费额度灰字:\n%s", out)
	}
}

// 复审修复测试：Fix 4 总览配额独立于探活（四种组合）
func TestRenderLongCatOverviewWithLot(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	res := app.Result{LongCat: []app.LongCatResult{{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		Usage:   &models.LongCatUsage{BalanceOK: &bal},
		Quota:   lcHappyQuota,
		Plan:    &models.LongCatPlan{Models: []models.LongCatModel{{ID: "LongCat-2.0"}}},
	}}}
	out := RenderOverview(res, time.Now(), c)
	if !strings.Contains(out, "Token 剩余 1,234,567（已用 75.3%）") {
		t.Errorf("有资源包应显示 Token 剩余:\n%s", out)
	}
}

func TestRenderLongCatOverviewNoLotWithPaygo(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	res := app.Result{LongCat: []app.LongCatResult{{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		Usage:   &models.LongCatUsage{BalanceOK: &bal},
		Quota:   &models.LongCatQuota{CurrentLot: nil},
		Plan:    &models.LongCatPlan{Models: []models.LongCatModel{{ID: "LongCat-2.0"}}},
	}}}
	out := RenderOverview(res, time.Now(), c)
	// 按量余额 0.00 → 不显示（hasUsablePaygo 返回 false，因为金额为 0）
	if strings.Contains(out, "按量余额") {
		t.Errorf("按量余额为 0 不应显示按量段:\n%s", out)
	}
}

func TestRenderLongCatOverviewNoLotPaygoWithAmount(t *testing.T) {
	c := Colorizer{Disabled: true}
	res := app.Result{LongCat: []app.LongCatResult{{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		// Usage==nil 但 paygo 有金额 → 配额段应照常显示
		Quota: &models.LongCatQuota{CurrentLot: nil},
		Paygo: &models.LongCatPaygo{
			PaygoBalance: &models.LongCatPaygoBalance{Primary: &models.LongCatPaygoAmount{Currency: "CNY", Amount: "8.88"}},
		},
		Plan: &models.LongCatPlan{Models: []models.LongCatModel{{ID: "LongCat-2.0"}}},
	}}}
	out := RenderOverview(res, time.Now(), c)
	if !strings.Contains(out, "按量余额 ¥8.88") {
		t.Errorf("Usage==nil 但有 paygo 金额应显示按量余额:\n%s", out)
	}
	if strings.Contains(out, "余额充足") || strings.Contains(out, "余额不足") {
		t.Errorf("Usage==nil 不应显示余额段:\n%s", out)
	}
}

func TestRenderLongCatOverviewNothing(t *testing.T) {
	c := Colorizer{Disabled: true}
	res := app.Result{LongCat: []app.LongCatResult{{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test", ConsoleCookie: "passport_token_key=fake"},
		Quota:   &models.LongCatQuota{CurrentLot: nil},
		Plan:    &models.LongCatPlan{Models: []models.LongCatModel{{ID: "LongCat-2.0"}}},
	}}}
	out := RenderOverview(res, time.Now(), c)
	if !strings.Contains(out, "模型 1 个") {
		t.Errorf("无配额数据应降级为模型数:\n%s", out)
	}
}

// 复审修复测试：Fix 2 标签对齐（"配额" 4 列中文，pad 后至少 2 空格）
func TestRenderLongCatDetailNoCookieLabelAlignment(t *testing.T) {
	c := Colorizer{Disabled: true}
	bal := true
	r := app.LongCatResult{
		Account: models.LongCatAccount{Name: "龙猫", ApiKey: "sk-test"},
		Usage:   &models.LongCatUsage{BalanceOK: &bal},
	}
	out := RenderLongCatDetail(r, time.Now(), c)
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  配额") && strings.Contains(line, "需控制台 Cookie") {
			// "配额" (4 列) + padTo 4 空格 + 单空格 = 标签后恰 5 空格（与「余额」「Token」同列）
			afterLabel := strings.TrimPrefix(line, "  配额")
			if !strings.HasPrefix(afterLabel, "     ") {
				t.Errorf("标签应对齐（配额后应恰为 5 空格）: %q", line)
			}
			return
		}
	}
	t.Errorf("未找到配额指引行:\n%s", out)
}
