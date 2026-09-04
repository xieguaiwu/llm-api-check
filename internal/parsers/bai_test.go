package parsers

import (
	"strings"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

// BAI /v1/models 响应真实形状（2026-09-04 实测截取）
const baiModelsFixture = `{"data":[` +
	`{"id":"minimax-m3","object":"model","created":1626777600,"owned_by":"minimax","supported_endpoint_types":["openai","anthropic"]},` +
	`{"id":"deepseek-v4-flash","object":"model","created":1626777600,"owned_by":"deepseek","supported_endpoint_types":null},` +
	`{"id":"qwen3.8-flash","object":"model","created":1626777600,"owned_by":"qwen","supported_endpoint_types":["openai"]},` +
	`{"id":"deepseek-v4-flash","object":"model","created":1626777600,"owned_by":"deepseek","supported_endpoint_types":null}` +
	`],"object":"list","success":true}`

func TestParseBaiModelsHappy(t *testing.T) {
	got, err := ParseBaiModels(baiModelsFixture)
	if err != nil {
		t.Fatalf("ParseBaiModels: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("重复 id 应去重，期望 3 个，实际 %d: %+v", len(got), got)
	}
	// 排序：deepseek < minimax < qwen
	if got[0].ID != "deepseek-v4-flash" || got[1].ID != "minimax-m3" || got[2].ID != "qwen3.8-flash" {
		t.Errorf("排序不符: %v", got)
	}
	if got[0].OwnedBy != "deepseek" {
		t.Errorf("owned_by 应保留: %q", got[0].OwnedBy)
	}
	if got[2].Endpoints == nil || len(got[2].Endpoints) != 1 || got[2].Endpoints[0] != "openai" {
		t.Errorf("supported_endpoint_types 应透传: %v", got[2].Endpoints)
	}
}

func TestParseBaiModelsErrorEnvelope(t *testing.T) {
	// 403 网关信封（实测形状：无 error 键，message + success=false）
	_, err := ParseBaiModels(`{"message":"HTTP node only allows access to inference API paths (/v1/chat/completions, /v1/messages, /v1/responses, /v1/models, /v1/images/*)","success":false}`)
	if err == nil {
		t.Fatal("错误信封应返回错误")
	}
	if !strings.Contains(err.Error(), "HTTP node only allows access") {
		t.Errorf("错误应含网关原文: %v", err)
	}
}

func TestParseBaiModelsEmptyData(t *testing.T) {
	if _, err := ParseBaiModels(`{"data":[],"object":"list","success":true}`); err == nil {
		t.Fatal("空清单应显式失败")
	}
	if _, err := ParseBaiModels(`{not-json`); err == nil {
		t.Fatal("非法 JSON 应显式失败")
	}
}

func TestBaiPlanMissingFreeFlash(t *testing.T) {
	p := models.BaiPlan{Models: []models.BaiModel{
		{ID: "deepseek-v4-flash"}, {ID: "deepseek-v4-flash-vision-exp"}, {ID: "qwen3.8-flash"},
	}}
	missing := p.MissingFreeFlash()
	if len(missing) != 1 || missing[0] != "glm-5.3-flash" {
		t.Errorf("应缺 glm-5.3-flash: %v", missing)
	}
	full := models.BaiPlan{Models: []models.BaiModel{
		{ID: "deepseek-v4-flash"}, {ID: "deepseek-v4-flash-vision-exp"},
		{ID: "glm-5.3-flash"}, {ID: "qwen3.8-flash"},
	}}
	if m := full.MissingFreeFlash(); len(m) != 0 {
		t.Errorf("全在清单时不应有缺失: %v", m)
	}
}
