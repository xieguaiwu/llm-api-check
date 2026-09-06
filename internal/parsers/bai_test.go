package parsers

import (
	"strconv"
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

// ── 积分额度（chat.b.ai tRPC） ─────────────────────────────────

// 真实响应快照（2026-09-06 实测，同一把 sk- key）
const (
	baiPointsFixture  = `{"result":{"data":{"json":{"points_balance":27166591,"points_expiring":7166591}}}}`
	baiSummaryFixture = `{"result":{"data":{"json":{"monthly_chart":[{"month":"2026-08","points":0},` +
		`{"month":"2026-09","points":2833409}],"monthly_spent":2833409,"points_balance":27166591}}}}`
	// tRPC 未授权信封：HTTP 401 + 该 body（doGet 先按状态码拦截，此形状在 200 时也要能识别）
	baiUnauthorizedFixture = `{"error":{"json":{"message":"UNAUTHORIZED","code":-32001,` +
		`"data":{"code":"UNAUTHORIZED","httpStatus":401,"path":"usage.points"}}}}`
)

func TestParseBaiPointsHappy(t *testing.T) {
	got, err := ParseBaiPoints(baiPointsFixture)
	if err != nil {
		t.Fatalf("ParseBaiPoints: %v", err)
	}
	if got.Balance != 27166591 || got.Expiring != 7166591 {
		t.Errorf("额度不符: %+v", got)
	}
	if got.HasMonthly {
		t.Errorf("points 通道不带月度消耗，HasMonthly 应为 false: %+v", got)
	}
}

func TestParseBaiPointsTolerantShapes(t *testing.T) {
	// JS 侧数字可能是浮点形状或字符串形状，严格解析会静默归零
	cases := []struct{ raw, want string }{
		{`{"result":{"data":{"json":{"points_balance":2.7e6}}}}`, "2700000"},
		{`{"result":{"data":{"json":{"points_balance":"12345"}}}}`, "12345"},
		{`{"result":{"data":{"json":{"points_balance":100,"points_expiring":null}}}}`, "100"},
	}
	for _, c := range cases {
		got, err := ParseBaiPoints(c.raw)
		if err != nil {
			t.Errorf("%s: 应解析成功: %v", c.raw, err)
			continue
		}
		if fmtInt(got.Balance) != c.want {
			t.Errorf("%s: 余额应为 %s，实际 %d", c.raw, c.want, got.Balance)
		}
	}
	// points_expiring 缺席 → 0（不是错误）
	got, err := ParseBaiPoints(`{"result":{"data":{"json":{"points_balance":500}}}}`)
	if err != nil || got.Balance != 500 || got.Expiring != 0 {
		t.Errorf("缺席的 points_expiring 应按 0 处理: %+v err=%v", got, err)
	}
}

func TestParseBaiPointsErrors(t *testing.T) {
	if _, err := ParseBaiPoints(baiUnauthorizedFixture); err == nil ||
		!strings.Contains(err.Error(), "无效、已过期或额度用尽") {
		t.Errorf("UNAUTHORIZED 信封应归一到凭据口径: %v", err)
	}
	// 非授权类错误保留原文，不谎报「key 有问题」
	if _, err := ParseBaiPoints(`{"error":{"json":{"message":"Internal server error","code":-32603,"data":{"code":"INTERNAL_SERVER_ERROR"}}}}`); err == nil ||
		!strings.Contains(err.Error(), "Internal server error") || strings.Contains(err.Error(), "无效") {
		t.Errorf("其他 tRPC 错误应带原文且不提 key: %v", err)
	}
	if _, err := ParseBaiPoints(`{"result":{"data":{"json":{"points_expiring":1}}}}`); err == nil ||
		!strings.Contains(err.Error(), "points_balance") {
		t.Errorf("缺 points_balance 应显式失败（不得显示 0 误導额度用尽）: %v", err)
	}
	if _, err := ParseBaiPoints(`{"result":{"data":{"json":null}}}`); err == nil {
		t.Error("空负载应显式失败")
	}
	if _, err := ParseBaiPoints(`{oops`); err == nil {
		t.Error("非法 JSON 应显式失败")
	}
}

// 越界 / Inf / NaN 形状必须显式失败：float→int64 越界转换在 amd64 上得 MinInt64，
// 放行会把解析失败伪装成「额度已耗尽」红警（momus P1-1，实测复现后修复）。
func TestParseBaiPointsRejectsOutOfRange(t *testing.T) {
	for _, raw := range []string{`1e30`, `9.3e18`, `"1e30"`, `"Inf"`, `"-Inf"`, `-1e30`, `1e400`, `"NaN"`} {
		body := `{"result":{"data":{"json":{"points_balance":` + raw + `}}}}`
		got, err := ParseBaiPoints(body)
		if err == nil {
			t.Errorf("越界形状 %s 应显式失败，实际 Balance=%d", raw, got.Balance)
		}
		if got.Balance == -9223372036854775808 || got.Balance < -1<<62 {
			t.Errorf("越界形状 %s 不得产出 MinInt64 型垃圾值: %d", raw, got.Balance)
		}
	}
	// 边界内的大整数仍要正常接受（2^53 内 JS 精确）
	if got, err := ParseBaiPoints(`{"result":{"data":{"json":{"points_balance":9007199254740991}}}}`); err != nil || got.Balance != 9007199254740991 {
		t.Errorf("int64 范围内的合法大整数不应被拒: %+v err=%v", got, err)
	}
	// points_expiring 越界按缺席处理（不连带否掉余额）
	got, err := ParseBaiPoints(`{"result":{"data":{"json":{"points_balance":500,"points_expiring":"Inf"}}}}`)
	if err != nil || got.Balance != 500 || got.Expiring != 0 {
		t.Errorf("越界的 expiring 应按缺席处理: %+v err=%v", got, err)
	}
}

func TestParseBaiMonthlySpent(t *testing.T) {
	got, err := ParseBaiMonthlySpent(baiSummaryFixture)
	if err != nil {
		t.Fatalf("ParseBaiMonthlySpent: %v", err)
	}
	if got != 2833409 {
		t.Errorf("本月消耗不符: %d", got)
	}
	if _, err := ParseBaiMonthlySpent(baiPointsFixture); err == nil ||
		!strings.Contains(err.Error(), "monthly_spent") {
		t.Errorf("缺 monthly_spent 应显式失败: %v", err)
	}
	if _, err := ParseBaiMonthlySpent(baiUnauthorizedFixture); err == nil ||
		!strings.Contains(err.Error(), "无效、已过期或额度用尽") {
		t.Errorf("summary 通道的 UNAUTHORIZED 应同口径: %v", err)
	}
}

// fmtInt 测试辅助：int64 → 十进制字符串（避免为断言引入 render 包）
func fmtInt(v int64) string { return strconv.FormatInt(v, 10) }

// usage.records 负载真实形状（2026-09-06 实测截取，superjson meta 忽略）
const baiRecordsFixture = `{"result":{"data":{"json":{"data":[` +
	`{"cache_tokens":{"cache_read_input_tokens":72064,"cache_creation_input_tokens":0},"cost_points":0,` +
	`"created_at":"2026-09-06T05:27:37.000Z","duration_sec":20.219,"id":"api_CygR4X0iGOuP7BaN","image_usage":null,` +
	`"input_tokens":73238,"model":"glm-5.3-flash","output_tokens":660,"request_id":"20260906x","source_type":"api",` +
	`"total_tokens":73898,"router_difficulty":null},` +
	`{"cost_points":"12","created_at":"2026-09-06T05:09:11.000Z","id":"api_2nd","input_tokens":2.5e3,` +
	`"model":"qwen3.8-flash","output_tokens":10,"source_type":"api","total_tokens":2510}` +
	`],"has_more":true,"next_cursor":"eyJ2Ijox","page":1,"pageSize":2}}}}`

func TestParseBaiRecordsHappy(t *testing.T) {
	recs, more, err := ParseBaiRecords(baiRecordsFixture)
	if err != nil {
		t.Fatalf("ParseBaiRecords: %v", err)
	}
	if !more {
		t.Errorf("has_more=true 应透传")
	}
	if len(recs) != 2 {
		t.Fatalf("应 2 条: %+v", recs)
	}
	r0 := recs[0]
	if r0.ID != "api_CygR4X0iGOuP7BaN" || r0.Model != "glm-5.3-flash" || r0.SourceType != "api" {
		t.Errorf("字段不符: %+v", r0)
	}
	if r0.InputTokens != 73238 || r0.OutputTokens != 660 || r0.TotalTokens != 73898 || r0.CostPoints != 0 {
		t.Errorf("token 数不符: %+v", r0)
	}
	if r0.CreatedAt != "2026-09-06T05:27:37.000Z" {
		t.Errorf("时间不符: %s", r0.CreatedAt)
	}
	// 宽容形状：字符串积分 + 浮点 token（rawInt64 通道）
	r1 := recs[1]
	if r1.CostPoints != 12 || r1.InputTokens != 2500 {
		t.Errorf("宽容解析不符: %+v", r1)
	}
}

func TestParseBaiRecordsHasMoreFalse(t *testing.T) {
	raw := `{"result":{"data":{"json":{"data":[{"model":"m","created_at":"2026-09-06T05:00:00.000Z","input_tokens":1,"output_tokens":2,"total_tokens":3,"cost_points":0}],"has_more":false}}}}`
	recs, more, err := ParseBaiRecords(raw)
	if err != nil || more {
		t.Fatalf("err=%v more=%v", err, more)
	}
	if len(recs) != 1 || recs[0].TotalTokens != 3 {
		t.Errorf("记录不符: %+v", recs)
	}
}

func TestParseBaiRecordsErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"UNAUTHORIZED", baiUnauthorizedFixture, "无效、已过期或额度用尽"},
		{"非授权错误带原文", `{"error":{"json":{"message":"boom","data":{"code":"BAD"}}}}`, "BAI 用量明细 返回错误: boom"},
		{"JSON 坏", `{not json`, "JSON 解析失败"},
		{"负载缺 data", `{"result":{"data":{"json":{"has_more":false}}}}`, "响应缺少 data"},
		{"data 为 null", `{"result":{"data":{"json":{"data":null,"has_more":false}}}}`, "响应缺少 data"},
	}
	for _, c := range cases {
		_, _, err := ParseBaiRecords(c.raw)
		if err == nil {
			t.Errorf("%s: 期望错误", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: 错误文案缺 %q: %v", c.name, c.want, err)
		}
	}
	// 空数组是合法零记录，不是错误
	recs, more, err := ParseBaiRecords(`{"result":{"data":{"json":{"data":[],"has_more":false}}}}`)
	if err != nil || more || len(recs) != 0 {
		t.Errorf("空数组应为零记录: err=%v more=%v recs=%v", err, more, recs)
	}
}

func TestSanitizeText(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"干净文本原样", "BAI API Key 无效，余额不足", "BAI API Key 无效，余额不足"},
		{"CSI 颜色序列剥离", "a\x1b[31m红\x1b[0mb", "a红b"},
		{"OSC 标题 BEL 终止", "a\x1b]0;evil\x07b", "ab"},
		{"OSC 标题 ST 终止", "a\x1b]2;t\x1b\\b", "ab"},
		{"\\r 剥离（防光标回行首伪造输出）", "行一\r伪造行", "行一伪造行"},
		{"\\n 与 \\t 保留（多行错误依赖）", "a\nb\tc", "a\nb\tc"},
		{"DEL 剥离", "a\x7fb", "ab"},
		{"C1 控制符剥离（U+009B）", "ab", "ab"},
		{"无终止符 CSI 有界吞掉", "a\x1b[31", "a"},
		{"尾部孤 ESC 丢弃", "a\x1b", "a"},
		{"中文多字节不受影响", "余额不足（≈ $0.01）", "余额不足（≈ $0.01）"},
		{"ESC 后中文正常", "\x1b[1m余额\x1b[m", "余额"},
		{"注入攻击组合", "\x1b]0;pwned\x07额度已用尽\x1b[31m", "额度已用尽"},
	}
	for _, c := range cases {
		if got := SanitizeText(c.in); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

// 消毒必须接进错误出口：服务器消息带 ANSI 时，解析错误里不得出现转义序列
func TestParseBaiErrorEnvelopeSanitized(t *testing.T) {
	raw := `{"error":{"json":{"message":"bad \u001b[31mtoken\u001b[0m inject\u0007","data":{"code":"BAD"}}}}`
	_, err := ParseBaiPoints(raw)
	if err == nil {
		t.Fatal("期望错误")
	}
	msg := err.Error()
	for _, bad := range []string{"\x1b", "\x07"} {
		if strings.Contains(msg, bad) {
			t.Errorf("错误消息含控制字符 %q: %q", bad, msg)
		}
	}
	if !strings.Contains(msg, "bad token inject") {
		t.Errorf("正文应保留: %q", msg)
	}
}
