package render

import (
	"strings"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

func baiAllFlash() app.BaiResult {
	ms := make([]models.BaiModel, 0, len(models.BaiFreeFlashModels)+1)
	for _, id := range models.BaiFreeFlashModels {
		ms = append(ms, models.BaiModel{ID: id, OwnedBy: "x"})
	}
	ms = append(ms, models.BaiModel{ID: "gpt-5.5", OwnedBy: "openai"})
	return app.BaiResult{
		Account: models.BaiAccount{ID: "b1", Name: "免费通道", ApiKey: "sk-bai"},
		Plan:    &models.BaiPlan{Models: ms},
	}
}

func TestRenderBaiDetailAllFlashPresent(t *testing.T) {
	got := RenderBaiDetail(baiAllFlash(), Colorizer{Disabled: true})
	if !strings.Contains(got, "免费通道 (BAI)") || !strings.Contains(got, "免费 0-Credits flash 通道") {
		t.Errorf("标题/副题不符:\n%s", got)
	}
	if !strings.Contains(got, "模型     5 个：") {
		t.Errorf("模型数不符:\n%s", got)
	}
	if !strings.Contains(got, "免费通道 ✓ deepseek-v4-flash / deepseek-v4-flash-vision-exp / glm-5.3-flash / qwen3.8-flash") {
		t.Errorf("免费通道全在应显示 ✓ 行:\n%s", got)
	}
	if strings.Contains(got, "缺失") {
		t.Errorf("不应有缺失提示:\n%s", got)
	}
}

func TestRenderBaiDetailMissingFlashWarns(t *testing.T) {
	r := baiAllFlash()
	r.Plan.Models = r.Plan.Models[:2] // 只留 deepseek 两项
	got := RenderBaiDetail(r, Colorizer{Disabled: true})
	if !strings.Contains(got, "✓ deepseek-v4-flash / deepseek-v4-flash-vision-exp · ") {
		t.Errorf("应在的部分仍显示 ✓:\n%s", got)
	}
	if !strings.Contains(got, "⚠ 缺失：glm-5.3-flash、qwen3.8-flash（pi-subagent 默认免费模型源受影响）") {
		t.Errorf("缺失应红色提示:\n%s", got)
	}
}

func TestRenderBaiDetailErrorAndEmpty(t *testing.T) {
	c := Colorizer{Disabled: true}
	r := app.BaiResult{Account: models.BaiAccount{ID: "b1", Name: "免费通道", ApiKey: "sk-bai"},
		Error: "BAI API Key 无效、已过期或额度用尽，请到 chat.b.ai 核对"}
	got := RenderBaiDetail(r, c)
	if !strings.Contains(got, "无效、已过期或额度用尽") {
		t.Errorf("错误行应显示:\n%s", got)
	}
	// 错误与「暂无数据」互斥（对齐 qwen 详情语义）
	if strings.Contains(got, "暂无数据") {
		t.Errorf("有错误时不应再显示暂无数据:\n%s", got)
	}
	// 空 key → 指引行
	empty := app.BaiResult{Account: models.BaiAccount{ID: "b1", Name: "免费通道"}}
	if got := RenderBaiDetail(empty, c); !strings.Contains(got, "accounts add --type bai --help") {
		t.Errorf("空 key 应给添加指引:\n%s", got)
	}
	// no-refresh（无 Plan 无错误）→ 暂无数据
	nr := app.BaiResult{Account: models.BaiAccount{ID: "b1", Name: "免费通道", ApiKey: "sk-bai"}}
	if got := RenderBaiDetail(nr, c); !strings.Contains(got, "暂无数据") {
		t.Errorf("--no-refresh 无数据应显示暂无数据:\n%s", got)
	}
}

func TestRenderOverviewIncludesBai(t *testing.T) {
	got := RenderOverview(app.Result{Bai: []app.BaiResult{baiAllFlash()}, LastUpdated: baseTime()}, baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got, "白B.AI (免费通道)") {
		t.Errorf("总览应含白B.AI 卡片:\n%s", got)
	}
	if !strings.Contains(got, "模型 5 个 · 免费通道 4/4") {
		t.Errorf("总览应含模型数与免费通道计数:\n%s", got)
	}
}

// ── 积分额度 ──────────────────────────────────────────────────

func baiWithPoints() app.BaiResult {
	r := baiAllFlash()
	r.Points = &models.BaiPoints{Balance: 27166591, Expiring: 7166591, MonthlySpent: 2833409, HasMonthly: true}
	return r
}

func TestRenderBaiDetailPoints(t *testing.T) {
	got := RenderBaiDetail(baiWithPoints(), Colorizer{Disabled: true})
	for _, want := range []string{
		"积分余额 27,166,591（≈ $27.17） · 其中 7,166,591 即将过期",
		"本月消耗 2,833,409（≈ $2.83）",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("详情缺少 %q:\n%s", want, got)
		}
	}
	// 标签列按显示宽度对齐：四行内容起始列一致（中文 2 列/字）
	if !alignedLabelColumn(got, []string{"积分余额", "本月消耗", "模型", "免费通道"}) {
		t.Errorf("标签列未对齐:\n%s", got)
	}
}

// alignedLabelColumn 检查各标签行的值起始显示列一致（中文按 2 列计）。
func alignedLabelColumn(out string, labels []string) bool {
	want := -1
	for _, lb := range labels {
		col, ok := valueColumn(out, lb)
		if !ok {
			return false
		}
		if want < 0 {
			want = col
		} else if col != want {
			return false
		}
	}
	return true
}

// valueColumn 返回标签行里数值（或正文）开始的显示列。
func valueColumn(out, label string) (int, bool) {
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimPrefix(line, "  ")
		if !strings.HasPrefix(trimmed, label) {
			continue
		}
		rest := strings.TrimPrefix(trimmed, label)
		pad := strings.IndexFunc(rest, func(r rune) bool { return r != ' ' })
		if pad < 0 {
			return 0, false
		}
		return 2 + displayWidth(label) + pad, true
	}
	return 0, false
}

func TestRenderBaiDetailPointsVariants(t *testing.T) {
	c := Colorizer{Disabled: true}
	// 无赠送额度 → 不显示「即将过期」段
	r := baiWithPoints()
	r.Points.Expiring = 0
	if got := RenderBaiDetail(r, c); strings.Contains(got, "即将过期") {
		t.Errorf("Expiring=0 不应显示过期段:\n%s", got)
	}
	// summary 缺席 → 不显示本月消耗行
	r2 := baiWithPoints()
	r2.Points.HasMonthly = false
	r2.Points.MonthlySpent = 0
	if got := RenderBaiDetail(r2, c); strings.Contains(got, "本月消耗") {
		t.Errorf("HasMonthly=false 不应显示消耗行:\n%s", got)
	}
	// 额度耗尽 → 整行红色 + 后果提示
	r3 := baiWithPoints()
	r3.Points.Balance = 0
	r3.Points.MonthlySpent = 0
	colored := RenderBaiDetail(r3, Colorizer{})
	if !strings.Contains(colored, "\x1b[31m") || !strings.Contains(colored, "额度已耗尽") {
		t.Errorf("余额 0 应红色告警:\n%q", colored)
	}
	if plain := RenderBaiDetail(r3, c); !strings.Contains(plain, "积分余额 0（≈ $0.00）") {
		t.Errorf("余额 0 应如实显示 0:\n%s", plain)
	}
	// 额度通道失败但模型在 → 仍显示模型行 + 错误行，不显示「暂无数据」
	r4 := baiAllFlash()
	r4.Error = "BAI API Key 无效、已过期或额度用尽，请到 chat.b.ai 核对"
	got4 := RenderBaiDetail(r4, c)
	if strings.Contains(got4, "暂无数据") {
		t.Errorf("有模型数据时不应显示暂无数据:\n%s", got4)
	}
	if strings.Contains(got4, "积分") {
		t.Errorf("额度通道失败时不应凭空造出积分行:\n%s", got4)
	}
}

func TestRenderOverviewBaiPoints(t *testing.T) {
	got := RenderOverview(app.Result{Bai: []app.BaiResult{baiWithPoints()}, LastUpdated: baseTime()},
		baseTime(), Colorizer{Disabled: true})
	for _, want := range []string{"积分 27,166,591（≈ $27.17）", "本月消耗 2,833,409", "模型 5 个 · 免费通道 4/4"} {
		if !strings.Contains(got, want) {
			t.Errorf("总览缺少 %q:\n%s", want, got)
		}
	}
	// 只有额度、没有模型清单时也要出数据（不得显示暂无数据）
	only := app.BaiResult{
		Account: models.BaiAccount{ID: "b1", Name: "免费通道", ApiKey: "sk-bai"},
		Points:  &models.BaiPoints{Balance: 1234},
	}
	got2 := RenderOverview(app.Result{Bai: []app.BaiResult{only}, LastUpdated: baseTime()}, baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got2, "积分 1,234") || strings.Contains(got2, "暂无数据") {
		t.Errorf("仅额度时总览应显示额度:\n%s", got2)
	}
	if strings.Contains(got2, "本月消耗") {
		t.Errorf("HasMonthly=false 不应显示消耗:\n%s", got2)
	}
}
