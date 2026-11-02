package parsers

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// cmModelsHappy /provider/v1/models 实测形状截取样本（OpenAI 兼容信封）。
const cmModelsHappy = `{"object":"list","data":[` +
	`{"id":"claude-sonnet-5-5","object":"model","created":1792901766,"owned_by":"command-code",` +
	`"name":"Claude Sonnet 5.5","context_length":1000000,"supported_endpoints":["/messages"]},` +
	`{"id":"gpt-5","object":"model","created":1792901767,"owned_by":"command-code",` +
	`"name":"GPT-5","context_length":400000,"supported_endpoints":["/chat/completions","/responses"]},` +
	`{"id":"claude-opus-4-5","object":"model","created":1792901768,"owned_by":"command-code",` +
	`"name":"Claude Opus 4.5","context_length":1000000,"supported_endpoints":["/messages","/chat/completions"]}` +
	`]}`

// cmModelsEmpty 空模型清单（合法 0 模型）。
const cmModelsEmpty = `{"object":"list","data":[]}`

// cmUnauthorized 实测 401 信封原文。
const cmUnauthorized = `{"success":false,"error":{"code":"UNAUTHORIZED","status":401,` +
	`"message":"Invalid 'Authorization' header or token.",` +
	`"docs":"https://commandcode.ai/docs/reference/errors/unauthorized"}}`

// cmUpgradeRequired 403 套餐不含 API 信封。
const cmUpgradeRequired = `{"success":false,"error":{"code":"upgrade_required","status":403,` +
	`"message":"Current plan does not include API access"}}`

func TestParseCommandaiModelsHappy(t *testing.T) {
	ms, err := ParseCommandaiModels(cmModelsHappy)
	if err != nil {
		t.Fatalf("ParseCommandaiModels: %v", err)
	}
	if len(ms) != 3 {
		t.Fatalf("len: got %d, want 3", len(ms))
	}
	// 排序后首项 claude-opus-4-5
	if ms[0].ID != "claude-opus-4-5" {
		t.Errorf("ms[0].ID: %q", ms[0].ID)
	}
	var sonnet *struct {
		ID            string
		ContextLength int64
		Endpoints     []string
	}
	for i := range ms {
		if ms[i].ID == "claude-sonnet-5-5" {
			m := ms[i]
			sonnet = &struct {
				ID            string
				ContextLength int64
				Endpoints     []string
			}{m.ID, m.ContextLength, m.SupportedEndpoints}
		}
	}
	if sonnet == nil {
		t.Fatal("claude-sonnet-5-5 missing")
	}
	if sonnet.ContextLength != 1000000 {
		t.Errorf("sonnet context: %d", sonnet.ContextLength)
	}
	if len(sonnet.Endpoints) != 1 || sonnet.Endpoints[0] != "/messages" {
		t.Errorf("sonnet endpoints: %v", sonnet.Endpoints)
	}
}

func TestParseCommandaiModelsEmpty(t *testing.T) {
	ms, err := ParseCommandaiModels(cmModelsEmpty)
	if err != nil {
		t.Fatalf("empty models should be legal: %v", err)
	}
	if len(ms) != 0 {
		t.Errorf("len: got %d, want 0", len(ms))
	}
}

func TestParseCommandaiModelsDataAbsent(t *testing.T) {
	_, err := ParseCommandaiModels(`{"object":"list"}`)
	if err == nil {
		t.Fatal("missing data should error")
	}
	if !strings.Contains(err.Error(), "data") {
		t.Errorf("error should mention data: %v", err)
	}
}

func TestParseCommandaiModelsUnauthorized(t *testing.T) {
	_, err := ParseCommandaiModels(cmUnauthorized)
	if !errors.Is(err, ErrCommandaiAuth) {
		t.Errorf("401 envelope should map to ErrCommandaiAuth: %v", err)
	}
}

func TestParseCommandaiModelsInvalidJSON(t *testing.T) {
	_, err := ParseCommandaiModels("not json")
	if err == nil {
		t.Fatal("invalid JSON should error")
	}
}

// cmWhoamiHappy whoami 成功响应（团队账号：带 org）。
const cmWhoamiHappy = `{"org":{"id":"org_abc123","login":"acme"},` +
	`"user":{"userName":"alice"},"orgLimits":[{"key":"x"}]}`

// cmWhoamiLive 个人账号实测响应（2026-10-25）：org 为 null。
const cmWhoamiLive = `{"success":true,"user":{"id":"ef176952-9032-40f2-91e2-ebaa7050820a",` +
	`"name":"xieguaiwu","email":"xieguaiwu@163.com","userName":"xieguaiwu"},"org":null}`

func TestParseCommandaiWhoamiHappy(t *testing.T) {
	w, err := ParseCommandaiWhoami(cmWhoamiHappy)
	if err != nil {
		t.Fatalf("ParseCommandaiWhoami: %v", err)
	}
	if w.OrgID != "org_abc123" {
		t.Errorf("OrgID: %q", w.OrgID)
	}
	if w.OrgLogin != "acme" {
		t.Errorf("OrgLogin: %q", w.OrgLogin)
	}
	if w.UserName != "alice" {
		t.Errorf("UserName: %q", w.UserName)
	}
}

// 实测个人账号 org 为 null：orgId 只是可选查询参数，不得当作必需字段报错
// （额度端点不带 orgId 即可；带空 orgId 反而 400 Invalid UUID）。
func TestParseCommandaiWhoamiOrgNull(t *testing.T) {
	w, err := ParseCommandaiWhoami(cmWhoamiLive)
	if err != nil {
		t.Fatalf("org:null should be legal: %v", err)
	}
	if w.OrgID != "" {
		t.Errorf("OrgID should be empty: %q", w.OrgID)
	}
	if w.OrgLogin != "" {
		t.Errorf("OrgLogin should be empty: %q", w.OrgLogin)
	}
	if w.UserName != "xieguaiwu" {
		t.Errorf("UserName: %q", w.UserName)
	}
}

func TestParseCommandaiWhoamiPartialFields(t *testing.T) {
	// org.login / user.userName 缺席容错空串
	w, err := ParseCommandaiWhoami(`{"org":{"id":"org_x"}}`)
	if err != nil {
		t.Fatalf("partial whoami should be legal: %v", err)
	}
	if w.OrgID != "org_x" || w.OrgLogin != "" || w.UserName != "" {
		t.Errorf("whoami: %+v", w)
	}
}
func TestParseCommandaiWhoamiUnauthorized(t *testing.T) {
	_, err := ParseCommandaiWhoami(cmUnauthorized)
	if !errors.Is(err, ErrCommandaiAuth) {
		t.Errorf("401 should map to ErrCommandaiAuth: %v", err)
	}
}

func TestParseCommandaiWhoamiUpgradeRequired(t *testing.T) {
	_, err := ParseCommandaiWhoami(cmUpgradeRequired)
	if !errors.Is(err, ErrCommandaiUpgrade) {
		t.Errorf("upgrade_required should map to ErrCommandaiUpgrade: %v", err)
	}
}

// cmCreditsHappy credits 成功响应（实测顶层结构：三池在 credits 内，
// windowLimits/sandboxAccess/sandboxMinutes 与 credits 平级；窗口带 exceeded）。
const cmCreditsHappy = `{"credits":{"belowThreshold":false,"creditThreshold":0,` +
	`"monthlyCredits":10,"purchasedCredits":3.2,"freeCredits":0},` +
	`"windowLimits":{"limited":true,"exceeded":null,` +
	`"fiveHour":{"used":14.2,"cap":14,"exceeded":true,"resetAt":1792901766000},` +
	`"weekly":{"used":30,"cap":35,"exceeded":false,"resetAt":"2026-10-30T00:00:00Z"}},` +
	`"sandboxAccess":true,` +
	`"sandboxMinutes":{"limitMinutes":120,"usedMinutes":47,"resetAt":1792901766}}`

// cmCreditsLive 个人账号 Go 套餐实测响应（2026-10-25）：limited=true 但 exceeded=false
// ——正是「limited 不等于已限流」的真实样本。
const cmCreditsLive = `{"credits":{"belowThreshold":false,"creditThreshold":0,` +
	`"monthlyCredits":9.4306827317,"purchasedCredits":0,"freeCredits":0},` +
	`"windowLimits":{"limited":true,"exceeded":null,` +
	`"fiveHour":{"used":0.054105,"cap":3,"exceeded":false,"resetAt":1792948252329},` +
	`"weekly":{"used":0.5693172683,"cap":6,"exceeded":false,"resetAt":1793511581667}},` +
	`"sandboxAccess":false,"sandboxMinutes":null}`

func TestParseCommandaiCreditsHappy(t *testing.T) {
	c, err := ParseCommandaiCredits(cmCreditsHappy)
	if err != nil {
		t.Fatalf("ParseCommandaiCredits: %v", err)
	}
	if c.PlanID != "" {
		t.Errorf("PlanID: 实测 credits 内无 planId，应为空串，got %q", c.PlanID)
	}
	if c.MonthlyCredits != 10 || c.PurchasedCredits != 3.2 || c.FreeCredits != 0 {
		t.Errorf("pools: %+v", c)
	}
	if !c.SandboxAccess {
		t.Error("SandboxAccess should be true")
	}
	if c.WindowLimits == nil || !c.WindowLimits.Limited {
		t.Fatalf("WindowLimits: %+v", c.WindowLimits)
	}
	fh := c.WindowLimits.FiveHour
	if fh == nil || fh.Used != 14.2 || fh.Cap != 14 {
		t.Errorf("fiveHour: %+v", fh)
	}
	if !fh.Exceeded {
		t.Error("fiveHour.Exceeded should be true")
	}
	if fh.ResetAt == nil {
		t.Error("fiveHour.ResetAt should be parsed (unix ms)")
	}
	wk := c.WindowLimits.Weekly
	if wk == nil || wk.Used != 30 || wk.Cap != 35 {
		t.Errorf("weekly: %+v", wk)
	}
	if wk.Exceeded {
		t.Error("weekly.Exceeded should be false")
	}
	if wk.ResetAt == nil {
		t.Error("weekly.ResetAt should be parsed (ISO)")
	}
	if c.SandboxMinutes == nil || c.SandboxMinutes.LimitMinutes != 120 || c.SandboxMinutes.UsedMinutes != 47 {
		t.Errorf("sandbox: %+v", c.SandboxMinutes)
	}
}

// 实测个人账号样本：windowLimits 在顶层（不在 credits 内），且两个窗口均未 exceeded。
func TestParseCommandaiCreditsLiveShape(t *testing.T) {
	c, err := ParseCommandaiCredits(cmCreditsLive)
	if err != nil {
		t.Fatalf("ParseCommandaiCredits(live): %v", err)
	}
	if c.MonthlyCredits != 9.4306827317 {
		t.Errorf("monthlyCredits: %v", c.MonthlyCredits)
	}
	if c.SandboxAccess {
		t.Error("SandboxAccess should be false")
	}
	if c.SandboxMinutes != nil {
		t.Errorf("sandboxMinutes should be nil: %+v", c.SandboxMinutes)
	}
	if c.WindowLimits == nil {
		t.Fatal("windowLimits must be read from the top level, got nil")
	}
	if !c.WindowLimits.Limited {
		t.Error("limited should be true")
	}
	if c.WindowLimits.FiveHour == nil || c.WindowLimits.Weekly == nil {
		t.Fatalf("both windows should parse: %+v", c.WindowLimits)
	}
	if c.WindowLimits.FiveHour.Exceeded || c.WindowLimits.Weekly.Exceeded {
		t.Error("live sample: no window is exceeded")
	}
	if c.WindowLimits.FiveHour.ResetAt == nil {
		t.Error("fiveHour.ResetAt (unix ms) should parse")
	}
}

// cmCreditsNoPools 纯充值户：三池字段缺席按 0。
const cmCreditsNoPools = `{"credits":{"planId":"individual-provider"}}`

func TestParseCommandaiCreditsPoolsAbsent(t *testing.T) {
	c, err := ParseCommandaiCredits(cmCreditsNoPools)
	if err != nil {
		t.Fatalf("pools absent should be legal: %v", err)
	}
	if c.MonthlyCredits != 0 || c.PurchasedCredits != 0 || c.FreeCredits != 0 {
		t.Errorf("pools should default 0: %+v", c)
	}
	if c.WindowLimits != nil {
		t.Errorf("windowLimits should be nil: %+v", c.WindowLimits)
	}
	if c.SandboxMinutes != nil {
		t.Errorf("sandboxMinutes should be nil: %+v", c.SandboxMinutes)
	}
}

func TestParseCommandaiCreditsAbsent(t *testing.T) {
	_, err := ParseCommandaiCredits(`{"foo":1}`)
	if err == nil {
		t.Fatal("missing credits should error")
	}
}

func TestParseCommandaiCreditsUnauthorized(t *testing.T) {
	_, err := ParseCommandaiCredits(cmUnauthorized)
	if !errors.Is(err, ErrCommandaiAuth) {
		t.Errorf("401 should map to ErrCommandaiAuth: %v", err)
	}
}

// cmSubscriptionHappy subscriptions 成功响应。
const cmSubscriptionHappy = `{"data":{"planId":"individual-provider","status":"active",` +
	`"currentPeriodStart":"2026-10-01T00:00:00Z","currentPeriodEnd":"2026-11-01T00:00:00Z"}}`

func TestParseCommandaiSubscriptionHappy(t *testing.T) {
	s, err := ParseCommandaiSubscription(cmSubscriptionHappy)
	if err != nil {
		t.Fatalf("ParseCommandaiSubscription: %v", err)
	}
	if s.PlanID != "individual-provider" || s.Status != "active" {
		t.Errorf("subscription: %+v", s)
	}
	if s.CurrentPeriodStart == nil || s.CurrentPeriodEnd == nil {
		t.Fatalf("period should be parsed: %+v", s)
	}
	if s.PeriodStartRaw != "2026-10-01T00:00:00Z" {
		t.Errorf("PeriodStartRaw: %q", s.PeriodStartRaw)
	}
}

func TestParseCommandaiSubscriptionUnknownStatus(t *testing.T) {
	// status 不在合法集仍原样保留（渲染层标注）
	s, err := ParseCommandaiSubscription(`{"data":{"planId":"p","status":"weird"}}`)
	if err != nil {
		t.Fatalf("unknown status should be legal: %v", err)
	}
	if s.Status != "weird" {
		t.Errorf("status: %q", s.Status)
	}
}

func TestParseCommandaiSubscriptionPeriodUnparsable(t *testing.T) {
	s, err := ParseCommandaiSubscription(`{"data":{"planId":"p","status":"active","currentPeriodEnd":"garbage"}}`)
	if err != nil {
		t.Fatalf("unparsable period should be legal: %v", err)
	}
	if s.CurrentPeriodEnd != nil {
		t.Errorf("period should be nil: %v", s.CurrentPeriodEnd)
	}
	if s.PeriodEndRaw != "garbage" {
		t.Errorf("raw should be preserved: %q", s.PeriodEndRaw)
	}
}

func TestParseCommandaiSubscriptionDataAbsent(t *testing.T) {
	_, err := ParseCommandaiSubscription(`{"foo":1}`)
	if err == nil {
		t.Fatal("missing data should error")
	}
}

// cmSummaryHappy usage/summary 成功响应。
const cmSummaryHappy = `{"totalCost":1.8,"totalCount":42,"periodBasis":"billing-period"}`

func TestParseCommandaiUsageSummaryHappy(t *testing.T) {
	s, err := ParseCommandaiUsageSummary(cmSummaryHappy)
	if err != nil {
		t.Fatalf("ParseCommandaiUsageSummary: %v", err)
	}
	if s.TotalCost != 1.8 || s.TotalCount != 42 || s.PeriodBasis != "billing-period" {
		t.Errorf("summary: %+v", s)
	}
}

func TestParseCommandaiUsageSummaryMissingCost(t *testing.T) {
	_, err := ParseCommandaiUsageSummary(`{"totalCount":42}`)
	if err == nil {
		t.Fatal("missing totalCost should error")
	}
}

func TestParseCommandaiUsageSummaryMissingCount(t *testing.T) {
	_, err := ParseCommandaiUsageSummary(`{"totalCost":1.8}`)
	if err == nil {
		t.Fatal("missing totalCount should error")
	}
}

func TestParseCommandaiUsageSummaryUnauthorized(t *testing.T) {
	_, err := ParseCommandaiUsageSummary(cmUnauthorized)
	if !errors.Is(err, ErrCommandaiAuth) {
		t.Errorf("401 should map to ErrCommandaiAuth: %v", err)
	}
}

func TestCommandaiTimeOfShapes(t *testing.T) {
	// 毫秒（>1e12）
	ms := commandaiTimeOf(json.RawMessage(`1792901766000`))
	if ms == nil || ms.Unix() != 1792901766 {
		t.Errorf("ms shape: %v", ms)
	}
	// 秒（≤1e12）
	sec := commandaiTimeOf(json.RawMessage(`1792901766`))
	if sec == nil || sec.Unix() != 1792901766 {
		t.Errorf("sec shape: %v", sec)
	}
	// ISO 字符串
	iso := commandaiTimeOf(json.RawMessage(`"2026-10-30T00:00:00Z"`))
	if iso == nil || iso.Unix() != time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC).Unix() {
		t.Errorf("iso shape: %v", iso)
	}
	// 垃圾 → nil
	if got := commandaiTimeOf(json.RawMessage(`"not-a-time"`)); got != nil {
		t.Errorf("garbage should be nil: %v", got)
	}
	// null / 空 → nil
	if got := commandaiTimeOf(json.RawMessage(`null`)); got != nil {
		t.Errorf("null should be nil: %v", got)
	}
	if got := commandaiTimeOf(nil); got != nil {
		t.Errorf("empty should be nil: %v", got)
	}
}

func TestCommandaiEnvelopeMessageSanitized(t *testing.T) {
	// 信封 message 含 ANSI 注入，必须剥离
	// 服务器把 ESC 转义为 \u001b（合法 JSON），清洗后不得残留
	raw := `{"success":false,"error":{"code":"UNAUTHORIZED","status":401,` +
		`"message":"\u001b[31mInvalid token\u001b[0m"}}`
	_, msg, ok := commandaiEnvelopeOf(raw)
	if !ok {
		t.Fatal("envelope should be detected")
	}
	if strings.Contains(msg, "\x1b") {
		t.Errorf("message should be sanitized: %q", msg)
	}
	if !strings.Contains(msg, "Invalid token") {
		t.Errorf("message should keep text: %q", msg)
	}
}

func TestCommandaiEnvelopeAbsent(t *testing.T) {
	if _, _, ok := commandaiEnvelopeOf(cmModelsHappy); ok {
		t.Error("success payload should not be treated as error envelope")
	}
	if _, _, ok := commandaiEnvelopeOf("not json"); ok {
		t.Error("invalid JSON should not be envelope")
	}
}

func TestErrCommandaiAuthMessage(t *testing.T) {
	if !strings.Contains(ErrCommandaiAuth.Error(), "commandcode.ai/studio") {
		t.Errorf("ErrCommandaiAuth should point to studio: %q", ErrCommandaiAuth.Error())
	}
	if strings.Contains(ErrCommandaiAuth.Error(), "sk-") {
		t.Errorf("error should not carry key: %q", ErrCommandaiAuth.Error())
	}
}
