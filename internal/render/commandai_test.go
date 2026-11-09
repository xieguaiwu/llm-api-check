package render

import (
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

func cmFixture() (app.CommandaiResult, time.Time) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	resetAt := now.Add(2*time.Hour + 13*time.Minute)
	end := now.Add(12 * 24 * time.Hour)
	return app.CommandaiResult{
		Account: models.CommandaiAccount{Name: "测试号", ApiKey: "sk-test123"},
		Plan: &models.CommandaiPlan{Models: []models.CommandaiModel{
			{ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", ContextLength: 1000000,
				SupportedEndpoints: []string{"/messages"}},
			{ID: "gpt-5", ContextLength: 400000, SupportedEndpoints: []string{"/chat/completions", "/responses"}},
		}},
		Whoami: &models.CommandaiWhoami{OrgID: "org_abc", OrgLogin: "acme", UserName: "alice"},
		Credits: &models.CommandaiCredits{
			PlanID:           "individual-provider",
			MonthlyCredits:   10,
			PurchasedCredits: 3.2,
			FreeCredits:      0,
			SandboxAccess:    true,
			SandboxMinutes:   &models.CommandaiSandbox{LimitMinutes: 120, UsedMinutes: 47},
			WindowLimits: &models.CommandaiWindowLimits{
				Limited:  true,
				FiveHour: &models.CommandaiWindowLimit{Used: 14.2, Cap: 14, Exceeded: true, ResetAt: &resetAt},
			},
		},
		Subscription: &models.CommandaiSubscription{
			PlanID: "individual-provider", Status: "active", CurrentPeriodEnd: &end,
		},
		Summary: &models.CommandaiUsageSummary{TotalCost: 1.8, TotalCount: 42, PeriodBasis: "billing-period"},
	}, now
}

func TestRenderCommandaiDetailHappy(t *testing.T) {
	r, now := cmFixture()
	c := Colorizer{Disabled: true}
	out := RenderCommandaiDetail(r, now, c)
	for _, want := range []string{
		"Commandai", "测试号", "alice", "org acme", "Provider", "active", "12 天后重置",
		"13.20", "1.80", "42 次请求",
		"5小时窗口", "已限流", "2小时13分后重置",
		"沙箱", "47.00/120.00",
		"模型", "2 个", "claude-sonnet-5-5", "1M",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}
}

func TestRenderCommandaiLimitedBadgeAndCountdownCoexist(t *testing.T) {
	r, now := cmFixture()
	c := Colorizer{Disabled: true}
	out := RenderCommandaiDetail(r, now, c)
	if !strings.Contains(out, "已限流") {
		t.Errorf("limited badge missing:\n%s", out)
	}
	if !strings.Contains(out, "2小时13分后重置") {
		t.Errorf("countdown missing:\n%s", out)
	}
	// 徽章与倒计时必须同段并存（§六铁律）
	if !strings.Contains(out, "已限流 · 2小时13分后重置") {
		t.Errorf("badge and countdown should coexist on the window line:\n%s", out)
	}
}

// limited=true 但 exceeded=false 且 used<cap（实测 Go 套餐形态）→ 不得报「已限流」。
// 这是防误报回归：limited 的语义是「本套餐配置了额度限制、值得展示」，不是「当前已限流」。
func TestRenderCommandaiLimitedIsNotRateLimit(t *testing.T) {
	r, now := cmFixture()
	r.Credits.WindowLimits = &models.CommandaiWindowLimits{
		Limited:  true,
		FiveHour: &models.CommandaiWindowLimit{Used: 0.05, Cap: 3, Exceeded: false, ResetAt: nil},
	}
	out := RenderCommandaiDetail(r, now, Colorizer{Disabled: true})
	if strings.Contains(out, "已限流") {
		t.Errorf("limited=true alone must not raise the rate-limit badge:\n%s", out)
	}
	if !strings.Contains(out, "5小时窗口") {
		t.Errorf("window section should still render:\n%s", out)
	}
}

// 窗口自身 exceeded=true → 徽章 + 倒计时并存。
func TestRenderCommandaiExceededBadge(t *testing.T) {
	r, now := cmFixture()
	r.Credits.WindowLimits = &models.CommandaiWindowLimits{
		Limited:  true,
		FiveHour: &models.CommandaiWindowLimit{Used: 3, Cap: 3, Exceeded: true},
	}
	out := RenderCommandaiDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "已限流") {
		t.Errorf("exceeded badge missing:\n%s", out)
	}
}

// used 达 cap 但 exceeded 字段缺席 → 兜底判据仍须报「已限流」。
func TestRenderCommandaiCapReachedFallback(t *testing.T) {
	r, now := cmFixture()
	r.Credits.WindowLimits = &models.CommandaiWindowLimits{
		Limited:  true,
		FiveHour: &models.CommandaiWindowLimit{Used: 3, Cap: 3},
	}
	out := RenderCommandaiDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "已限流") {
		t.Errorf("cap-reached fallback badge missing:\n%s", out)
	}
}

func TestRenderCommandaiWindowAbsent(t *testing.T) {
	r, now := cmFixture()
	r.Credits.WindowLimits = nil
	c := Colorizer{Disabled: true}
	out := RenderCommandaiDetail(r, now, c)
	if strings.Contains(out, "5小时窗口") || strings.Contains(out, "已限流") {
		t.Errorf("window section should not render when windowLimits absent:\n%s", out)
	}
	// 其他段仍在
	if !strings.Contains(out, "13.20") {
		t.Errorf("billing line should still render:\n%s", out)
	}
}

func TestRenderCommandaiExhaustedRed(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	r := app.CommandaiResult{
		Account: models.CommandaiAccount{Name: "t", ApiKey: "k"},
		Credits: &models.CommandaiCredits{PlanID: "individual-provider"},
		Summary: &models.CommandaiUsageSummary{TotalCost: 5},
	}
	out := RenderCommandaiDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "额度已耗尽") {
		t.Errorf("exhausted line missing:\n%s", out)
	}
}

func TestRenderCommandaiCLIFallbackNote(t *testing.T) {
	r, now := cmFixture()
	r.QuotaKeySource = "cli"
	out := RenderCommandaiDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "已回退 Command Code CLI 登录凭据") {
		t.Errorf("详情应说明额度凭据来源:\n%s", out)
	}
	overview := RenderOverview(app.Result{Commandai: []app.CommandaiResult{r}}, now, Colorizer{Disabled: true})
	if !strings.Contains(overview, "Command Code CLI 登录态") {
		t.Errorf("总览应说明额度凭据来源:\n%s", overview)
	}
}

func TestRenderCommandaiError(t *testing.T) {
	r := app.CommandaiResult{
		Account: models.CommandaiAccount{Name: "t", ApiKey: "bad"},
		Error:   "Commandai API Key 无效或已过期",
	}
	out := RenderCommandaiDetail(r, time.Now(), Colorizer{Disabled: true})
	if !strings.Contains(out, "Commandai API Key 无效") {
		t.Errorf("error line missing:\n%s", out)
	}
}

func TestRenderCommandaiNoKey(t *testing.T) {
	r := app.CommandaiResult{Account: models.CommandaiAccount{Name: "t"}}
	out := RenderCommandaiDetail(r, time.Now(), Colorizer{Disabled: true})
	if !strings.Contains(out, "未配置 API Key") {
		t.Errorf("no-key hint missing:\n%s", out)
	}
}

func TestRenderCommandaiNoColor(t *testing.T) {
	r, now := cmFixture()
	out := RenderCommandaiDetail(r, now, Colorizer{Disabled: true})
	if strings.Contains(out, "\x1b[") {
		t.Errorf("NO_COLOR output should have no ANSI:\n%s", out)
	}
}

func TestRenderOverviewCommandai(t *testing.T) {
	r, now := cmFixture()
	res := app.Result{Commandai: []app.CommandaiResult{r}}
	out := RenderOverview(res, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "Commandai (测试号)") {
		t.Errorf("overview missing commandai section:\n%s", out)
	}
	if strings.Contains(out, "未配置任何账号") {
		t.Errorf("commandai-only should not show empty hint:\n%s", out)
	}
	if !strings.Contains(out, "已限流") {
		t.Errorf("overview should show limited note:\n%s", out)
	}
	if !strings.Contains(out, "2小时13分后重置") {
		t.Errorf("overview limited note should carry countdown:\n%s", out)
	}
}

func TestRenderCommandaiModelsCount(t *testing.T) {
	r, now := cmFixture()
	r.Plan = &models.CommandaiPlan{Models: []models.CommandaiModel{
		{ID: "a", SupportedEndpoints: []string{"/messages"}},
		{ID: "b", SupportedEndpoints: []string{"/chat/completions"}},
		// 真实形态（实测 2026-10-25）：Claude 系只开 /messages；双端点 = chat∧responses。
		{ID: "c", SupportedEndpoints: []string{"/chat/completions", "/responses"}},
	}}
	out := RenderCommandaiDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "3 个") {
		t.Errorf("model count missing:\n%s", out)
	}
	if !strings.Contains(out, "Claude 系 1") || !strings.Contains(out, "OpenAI 兼容 2") || !strings.Contains(out, "双端点 1") {
		t.Errorf("endpoint counts missing:\n%s", out)
	}
}
