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
	out := RenderLongCatDetail(r, c)
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
	out := RenderLongCatDetail(r, c)
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
	out := RenderLongCatDetail(r, c)
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
	out := RenderLongCatDetail(r, c)
	if !strings.Contains(out, "API Key 无效") {
		t.Errorf("output should show error: %q", out)
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
