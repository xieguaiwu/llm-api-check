package render

import (
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

func gzResult() app.GptzeroResult {
	return app.GptzeroResult{
		Account: models.GptzeroAccount{ID: "g1", Name: "论文扫", ApiKey: "adfc"},
		Usage: &models.GptzeroUsage{
			Email:     "xieguaiwu@163.com",
			PlanName:  "API (300k words/month)",
			Monthly:   models.GptzeroCounters{Words: 587, Chars: 3952, Documents: 1},
			AllTime:   models.GptzeroCounters{Words: 1675389, Chars: 11680324, Documents: 861},
			CharLimit: 150000,
			LastReset: "2026-09-06T16:28:34.943+00:00",
			Plan: models.GptzeroPlan{
				Name:             "API (300k words/month)",
				DurationType:     "monthly",
				WordLimit:        300000,
				OverageWordLimit: 1000000,
				PriceCents:       4500,
			},
		},
	}
}

func TestRenderGptzeroDetailHappy(t *testing.T) {
	got := RenderGptzeroDetail(gzResult(), time.Now(), Colorizer{Disabled: true})
	for _, want := range []string{
		"论文扫 (GPTZero)",
		"xieguaiwu@163.com",
		"API (300k words/month)",
		"$45.00/月",
		"587 / 300,000 词",
		"0%",
		"超额上限 1,000,000 词",
		"≈2026-10-07 00:28 重置",
		"1,675,389 词",
		"150,000 字符",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("详情缺 %q:\n%s", want, got)
		}
	}
}

func TestRenderGptzeroDetailOverageRed(t *testing.T) {
	r := gzResult()
	r.Usage.Monthly.Words = 350000 // 超过 300000 内含额度
	got := RenderGptzeroDetail(r, time.Now(), Colorizer{Disabled: true})
	if !strings.Contains(got, "已进入超额计费区") {
		t.Errorf("超额区应红标:\n%s", got)
	}
}

func TestRenderGptzeroDetailUnknownLimitNoBar(t *testing.T) {
	r := gzResult()
	r.Usage.Plan.WordLimit = 0
	got := RenderGptzeroDetail(r, time.Now(), Colorizer{Disabled: true})
	if strings.Contains(got, "[█") || strings.Contains(got, "[░") {
		t.Errorf("上限未知不应画条:\n%s", got)
	}
	if !strings.Contains(got, "上限未知") {
		t.Errorf("应注明上限未知:\n%s", got)
	}
}

func TestRenderGptzeroDetailBadLastResetNoCrash(t *testing.T) {
	r := gzResult()
	r.Usage.LastReset = "not-a-time"
	got := RenderGptzeroDetail(r, time.Now(), Colorizer{Disabled: true})
	if strings.Contains(got, "重置") {
		t.Errorf("LastReset 不可解析不应渲染周期行:\n%s", got)
	}
}

func TestRenderGptzeroDetailNoKey(t *testing.T) {
	r := app.GptzeroResult{Account: models.GptzeroAccount{ID: "g1", Name: "x"}}
	got := RenderGptzeroDetail(r, time.Now(), Colorizer{Disabled: true})
	if !strings.Contains(got, "未配置 API Key") || !strings.Contains(got, "--type gptzero") {
		t.Errorf("未配置提示不符:\n%s", got)
	}
}

func TestRenderGptzeroDetailErrorOnly(t *testing.T) {
	r := app.GptzeroResult{
		Account: models.GptzeroAccount{ID: "g1", Name: "x", ApiKey: "k"},
		Error:   "GPTZero API Key 无效或已过期，请到 app.gptzero.me 的 API 订阅页核对",
	}
	got := RenderGptzeroDetail(r, time.Now(), Colorizer{Disabled: true})
	if !strings.Contains(got, "GPTZero API Key 无效") {
		t.Errorf("错误应展示:\n%s", got)
	}
	if strings.Contains(got, "暂无数据") {
		t.Errorf("有错误时不应再打暂无数据:\n%s", got)
	}
}

func TestWriteGptzeroOverviewPercentLine(t *testing.T) {
	var b strings.Builder
	writeGptzeroOverview(&b, gzResult(), Colorizer{Disabled: true})
	got := b.String()
	if !strings.Contains(got, "月额度 587 / 300,000 词 · 0%") {
		t.Errorf("总览行不符:\n%s", got)
	}
}

func TestWriteGptzeroOverviewError(t *testing.T) {
	var b strings.Builder
	writeGptzeroOverview(&b, app.GptzeroResult{
		Account: models.GptzeroAccount{ID: "g1", Name: "x", ApiKey: "k"},
		Error:   "HTTP 500: boom",
	}, Colorizer{Disabled: true})
	if !strings.Contains(b.String(), "HTTP 500: boom") {
		t.Errorf("总览应展示错误:\n%s", b.String())
	}
}
