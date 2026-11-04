package render

import (
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

func cbRenderFixture() (app.CodebuddyResult, time.Time) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	return app.CodebuddyResult{
		Account: models.CodebuddyAccount{Name: "腾讯CB", ApiKey: "ck-test"},
		Identity: &models.CodebuddyIdentity{
			UID: "e54c80ec-06fe-4be5-bac3-b74020337e7b", Nickname: "xieguaiwu", UIN: "450703308768",
			Type: "personal", AccountType: "",
		},
		Catalog: &models.CodebuddyCatalog{Models: []models.CodebuddyModel{
			{ID: "default-model", Name: "Auto", MaxInputTokens: 176000, MaxOutputTokens: 24000,
				CreditsRaw: "", HasMultiplier: false, IsDefault: true},
			{ID: "fast-model", Name: "Fast", MaxInputTokens: 200000, MaxOutputTokens: 32000,
				CreditsRaw: "x0.34 credits", CreditsMultiplier: 0.34, HasMultiplier: true},
			{ID: "gpt-6-astra", Name: "GPT-6-Astra", MaxInputTokens: 1000000, MaxOutputTokens: 128000,
				CreditsRaw: "x6.67", CreditsMultiplier: 6.67, HasMultiplier: true},
		}},
		Ledger: &models.CodebuddyLedger{
			Root: "/home/u/.codebuddy/projects", Files: 27, Sessions: 27, Lines: 2264, UnbilledLines: 1601,
			Total: models.CodebuddyWindow{Label: "全部", Calls: 663, Credits: 52.85,
				Prompt: 42196674, Completion: 420556, CacheHit: 41561344},
			Windows: []models.CodebuddyWindow{
				{Label: "今日", Days: 1, Calls: 663, Credits: 52.85},
				{Label: "近7天", Days: 7, Calls: 663, Credits: 52.85},
				{Label: "近30天", Days: 30, Calls: 663, Credits: 52.85},
				{Label: "全部", Days: 0, Calls: 663, Credits: 52.85},
			},
			ByModel: []models.CodebuddyModelUsage{
				{Model: "deepseek-v4.1-flash", Calls: 652, Credits: 43.57},
				{Model: "default-model", Calls: 11, Credits: 9.28},
			},
			LastUsed: now.Add(-75 * time.Minute).UnixMilli(),
		},
	}, now
}

func TestRenderCodebuddyDetailHappy(t *testing.T) {
	r, now := cbRenderFixture()
	out := RenderCodebuddyDetail(r, now, Colorizer{Disabled: true})

	// 标题与数据源声明（后者是防止把台账误读成账户余额的关键文案）
	for _, want := range []string{
		"腾讯CB (CodeBuddy)",
		"服务端不提供额度接口",
		"账号", "xieguaiwu", "e54c80ec-06fe-4be5-bac3-b74020337e7b", "个人",
		"模型", "3 个", "默认 Auto", "倍率 x0.34–x6.67",
		"本地台账", "52.85", "663",
		"今日", "近7天", "近30天",
		"最近",
		"按模型", "deepseek-v4.1-flash", "43.57", "default-model", "9.28",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("详情缺少 %q:\n%s", want, out)
		}
	}
	// 不得出现伪造的余额口径
	for _, banned := range []string{"余额", "剩余额度", "已用"} {
		if strings.Contains(out, banned) {
			t.Errorf("详情不得出现 %q（服务端无此数据）:\n%s", banned, out)
		}
	}
}

func TestRenderCodebuddyDetailEmptyLedger(t *testing.T) {
	r, now := cbRenderFixture()
	r.Ledger = &models.CodebuddyLedger{Root: "/tmp/x"}
	out := RenderCodebuddyDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "暂无记录") {
		t.Errorf("空台账应显式提示（不显示 0）:\n%s", out)
	}
	if strings.Contains(out, "0.00 积分") {
		t.Errorf("空台账不应显示 0.00:\n%s", out)
	}
}

func TestRenderCodebuddyDetailNilLedger(t *testing.T) {
	// 台账路失败（Ledger=nil）：不应 panic，也不应伪造数据
	r, now := cbRenderFixture()
	r.Ledger = nil
	out := RenderCodebuddyDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "腾讯CB (CodeBuddy)") {
		t.Errorf("标题应在:\n%s", out)
	}
	if strings.Contains(out, "本地台账") {
		t.Errorf("nil 台账不应渲染台账段:\n%s", out)
	}
}

func TestRenderCodebuddyDetailNoKey(t *testing.T) {
	r := app.CodebuddyResult{Account: models.CodebuddyAccount{Name: "nokey"}}
	out := RenderCodebuddyDetail(r, time.Now(), Colorizer{Disabled: true})
	if !strings.Contains(out, "未配置 API Key") {
		t.Errorf("缺 key 应提示:\n%s", out)
	}
	if !strings.Contains(out, "accounts add --type codebuddy") {
		t.Errorf("提示应给出添加命令:\n%s", out)
	}
}

func TestRenderCodebuddyDetailUnlabeledMultiplier(t *testing.T) {
	// credits 为空串 → 不显示倍率（显示 x0.00 会被读成免费）
	r, now := cbRenderFixture()
	r.Catalog = &models.CodebuddyCatalog{Models: []models.CodebuddyModel{
		{ID: "default-model", Name: "Auto", MaxInputTokens: 176000, CreditsRaw: "", HasMultiplier: false},
	}}
	out := RenderCodebuddyDetail(r, now, Colorizer{Disabled: true})
	if strings.Contains(out, "x0.00") {
		t.Errorf("未标注倍率不应渲染成 x0.00:\n%s", out)
	}
	if strings.Contains(out, "倍率") {
		t.Errorf("全部未标注时不应出现「倍率」字样:\n%s", out)
	}
	if !strings.Contains(out, "Auto（176K）") {
		t.Errorf("模型明细应显示上下文:\n%s", out)
	}
}

func TestRenderCodebuddyDetailSkippedLines(t *testing.T) {
	r, now := cbRenderFixture()
	r.Ledger.SkippedLines = 3
	out := RenderCodebuddyDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "3 行无法解析") {
		t.Errorf("坏行应提示（聚合可能偏低）:\n%s", out)
	}
}

func TestRenderCodebuddyDetailManyModels(t *testing.T) {
	r, now := cbRenderFixture()
	ms := make([]models.CodebuddyModel, 0, 10)
	for i := 0; i < 10; i++ {
		ms = append(ms, models.CodebuddyModel{
			ID: "m" + string(rune('a'+i)), Name: "M" + string(rune('A'+i)),
			MaxInputTokens: 100000, HasMultiplier: true, CreditsMultiplier: float64(i) * 0.5,
		})
	}
	r.Catalog = &models.CodebuddyCatalog{Models: ms}
	out := RenderCodebuddyDetail(r, now, Colorizer{Disabled: true})
	if !strings.Contains(out, "10 个") {
		t.Errorf("应显示总数 10:\n%s", out)
	}
	if !strings.Contains(out, "…等 4 个") {
		t.Errorf("超出 6 个应折叠为「…等 4 个」:\n%s", out)
	}
}

func TestRenderCodebuddyOverview(t *testing.T) {
	r, _ := cbRenderFixture()
	res := app.Result{Codebuddy: []app.CodebuddyResult{r}}
	out := RenderOverview(res, time.Now(), Colorizer{Disabled: true})
	for _, want := range []string{"CodeBuddy (腾讯CB)", "xieguaiwu", "模型 3 个", "台账累计 52.85 积分 / 663 次"} {
		if !strings.Contains(out, want) {
			t.Errorf("总览缺少 %q:\n%s", want, out)
		}
	}
}

func TestRenderCodebuddyOverviewEmptyLedger(t *testing.T) {
	r, _ := cbRenderFixture()
	r.Ledger = &models.CodebuddyLedger{Root: "/tmp/x"}
	res := app.Result{Codebuddy: []app.CodebuddyResult{r}}
	out := RenderOverview(res, time.Now(), Colorizer{Disabled: true})
	if !strings.Contains(out, "台账无记录") {
		t.Errorf("空台账总览应提示:\n%s", out)
	}
}

func TestRenderCodebuddyOverviewNoKey(t *testing.T) {
	r := app.CodebuddyResult{Account: models.CodebuddyAccount{Name: "nokey"}}
	res := app.Result{Codebuddy: []app.CodebuddyResult{r}}
	out := RenderOverview(res, time.Now(), Colorizer{Disabled: true})
	if !strings.Contains(out, "未配置 API Key") {
		t.Errorf("缺 key 应提示:\n%s", out)
	}
}

func TestCodebuddyMultiplierRange(t *testing.T) {
	cases := []struct {
		desc string
		ms   []models.CodebuddyModel
		want string
	}{
		{"无标注", []models.CodebuddyModel{{HasMultiplier: false}}, ""},
		{"单值", []models.CodebuddyModel{
			{HasMultiplier: true, CreditsMultiplier: 0.5},
			{HasMultiplier: false},
			{HasMultiplier: true, CreditsMultiplier: 0.5},
		}, "倍率 x0.50"},
		{"区间", []models.CodebuddyModel{
			{HasMultiplier: true, CreditsMultiplier: 6.67},
			{HasMultiplier: true, CreditsMultiplier: 0},
		}, "倍率 x0.00–x6.67"},
	}
	for _, c := range cases {
		if got := codebuddyMultiplierRange(c.ms); got != c.want {
			t.Errorf("%s: got %q, want %q", c.desc, got, c.want)
		}
	}
}
