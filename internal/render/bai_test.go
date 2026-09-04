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
	if !strings.Contains(got, "模型           5 个：") {
		t.Errorf("模型数不符:\n%s", got)
	}
	if !strings.Contains(got, "✓ deepseek-v4-flash / deepseek-v4-flash-vision-exp / glm-5.3-flash / qwen3.8-flash") {
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
