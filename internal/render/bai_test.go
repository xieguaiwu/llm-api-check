package render

import (
	"strings"
	"testing"
	"time"

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
	// 在的部分单独一行 ✓（无探活数据时维持旧语义）
	if !strings.Contains(got, "✓ deepseek-v4-flash / deepseek-v4-flash-vision-exp") {
		t.Errorf("应在的部分仍显示 ✓:\n%s", got)
	}
	if !strings.Contains(got, "⚠ 缺失：glm-5.3-flash、qwen3.8-flash（pi-subagent 默认免费模型源受影响）") {
		t.Errorf("缺失应红色提示:\n%s", got)
	}
}

// 探活故障模型：清单在但运行时挂 → 红色 ⚠ + 故障摘要；存活模型照常 ✓
func TestRenderBaiFlashProbeDead(t *testing.T) {
	r := baiAllFlash()
	r.Plan.Probes = []models.BaiProbe{
		{Model: "deepseek-v4-flash", Alive: false, Detail: "HTTP 503: pre_consume_token_quota_failed"},
		{Model: "deepseek-v4-flash-vision-exp", Alive: true},
		{Model: "glm-5.3-flash", Alive: true},
		{Model: "qwen3.8-flash", Alive: true},
	}
	got := RenderBaiDetail(r, Colorizer{Disabled: true})
	if !strings.Contains(got, "⚠ deepseek-v4-flash 运行时故障：HTTP 503: pre_consume_token_quota_failed") {
		t.Errorf("探活故障应显示模型名与摘要:\n%s", got)
	}
	if !strings.Contains(got, "✓ glm-5.3-flash / qwen3.8-flash / deepseek-v4-flash-vision-exp") &&
		!strings.Contains(got, "✓ deepseek-v4-flash-vision-exp / glm-5.3-flash / qwen3.8-flash") {
		t.Errorf("存活模型应显示 ✓:\n%s", got)
	}
	// 总览：alive 计数扣故障 + 故障注记
	ov := RenderOverview(app.Result{Bai: []app.BaiResult{r}}, time.Time{}, Colorizer{Disabled: true})
	if !strings.Contains(ov, "免费通道 3/4（1 故障）") {
		t.Errorf("总览应扣故障并注记:\n%s", ov)
	}
}

// 未探活（--no-refresh）时总览维持清单在位计数
func TestRenderBaiOverviewWithoutProbes(t *testing.T) {
	r := baiAllFlash()
	ov := RenderOverview(app.Result{Bai: []app.BaiResult{r}}, time.Time{}, Colorizer{Disabled: true})
	if !strings.Contains(ov, "免费通道 4/4") || strings.Contains(ov, "故障") {
		t.Errorf("无探活不应有故障注记:\n%s", ov)
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

func TestRenderBaiStatsSection(t *testing.T) {
	c := Colorizer{Disabled: true}
	stats := models.BaiUsageStats{
		RecordsFetched: 3,
		Complete:       true,
		WindowStart:    "2026-09-06T05:09:11.000Z",
		WindowEnd:      "2026-09-06T05:28:30.000Z",
		PerModel: []models.BaiModelUsage{
			{Model: "glm-5.3-flash", Requests: 2, InputTokens: 400, OutputTokens: 40, TotalTokens: 440},
			{Model: "qwen3.8-flash", Requests: 1, InputTokens: 2_500_000, OutputTokens: 10, TotalTokens: 2_500_010},
		},
		TotalRequests:   3,
		TotalTokens:     2_500_450,
		TotalCostPoints: 0,
	}
	r := baiAllFlash()
	r.Stats = &stats
	got := RenderBaiDetail(r, c)
	for _, want := range []string{
		"用量分析",
		"窗口", "2026-09-06 ~ 2026-09-06",
		"3 次", "2,500,450 tokens",
		"glm-5.3-flash", "2 次 · in 400 / out 40 / total 440 tokens",
		"qwen3.8-flash", "2,500,000",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stats 段缺 %q:\n%s", want, got)
		}
	}
	// 完整数据不标截断
	if strings.Contains(got, "数据不完整") {
		t.Errorf("complete=true 不应标截断:\n%s", got)
	}
}

func TestRenderBaiStatsTruncatedAndCost(t *testing.T) {
	c := Colorizer{Disabled: true}
	stats := models.BaiUsageStats{
		RecordsFetched:  1000,
		Complete:        false,
		WindowStart:     "2026-09-05T00:00:00.000Z",
		WindowEnd:       "2026-09-06T05:28:30.000Z",
		TotalRequests:   1000,
		TotalTokens:     110000,
		TotalCostPoints: 5_000_000,
	}
	r := baiAllFlash()
	r.Stats = &stats
	got := RenderBaiDetail(r, c)
	if !strings.Contains(got, "数据不完整") {
		t.Errorf("截断必须明示:\n%s", got)
	}
	if !strings.Contains(got, "5,000,000") || !strings.Contains(got, "≈ $5.00") {
		t.Errorf("消耗积分与美元换算应显示:\n%s", got)
	}
	if !strings.Contains(got, "2026-09-05 ~ 2026-09-06") {
		t.Errorf("跨日窗口应显示:\n%s", got)
	}
}

func TestRenderBaiDetailWithoutStatsUnchanged(t *testing.T) {
	c := Colorizer{Disabled: true}
	got := RenderBaiDetail(baiAllFlash(), c)
	if strings.Contains(got, "用量分析") {
		t.Errorf("无 Stats 不应显示用量分析段:\n%s", got)
	}
}

// 过期部分黄色告警（口径用户定：只警过期 ≥100 万；余额维持 ≤0 红不设阈值）
func TestRenderBaiExpiringWarn(t *testing.T) {
	c := Colorizer{Disabled: true}
	// 超阈值：详情 + 总览都提醒
	r := baiAllFlash()
	r.Points = &models.BaiPoints{Balance: 27_166_591, Expiring: 7_166_591, MonthlySpent: 2_833_409, HasMonthly: true}
	got := RenderBaiDetail(r, c)
	if !strings.Contains(got, "过期提醒") || !strings.Contains(got, "7,166,591") || !strings.Contains(got, "不花就没了") {
		t.Errorf("详情缺过期提醒:\n%s", got)
	}
	ov := RenderOverview(app.Result{Bai: []app.BaiResult{r}}, time.Time{}, c)
	if !strings.Contains(ov, "即将过期，不花就没了") {
		t.Errorf("总览缺过期提醒:\n%s", ov)
	}
	// 低于阈值：不提醒（免打扰）
	r2 := baiAllFlash()
	r2.Points = &models.BaiPoints{Balance: 900_000, Expiring: 900_000}
	if got := RenderBaiDetail(r2, c); strings.Contains(got, "过期提醒") {
		t.Errorf("低于阈值不应提醒:\n%s", got)
	}
	// 额度耗尽（≤0）时过期提醒退位——整行红色已是最高级
	r3 := baiAllFlash()
	r3.Points = &models.BaiPoints{Balance: 0, Expiring: 2_000_000}
	if got := RenderBaiDetail(r3, c); strings.Contains(got, "过期提醒") {
		t.Errorf("耗尽态不应再叠过期提醒:\n%s", got)
	}
}
