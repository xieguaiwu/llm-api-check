package parsers

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

// ── ParseCodebuddyAccounts ───────────────────────────────────

func TestParseCodebuddyAccountsHappy(t *testing.T) {
	raw := `{"code":0,"msg":"OK","data":{"accounts":[
		{"uid":"u-1","nickname":"alice","uin":"450703308768","type":"personal",
		 "accountType":"","isAmbassador":false},
		{"uid":"u-2","nickname":"bob","uin":"1","type":"enterprise",
		 "accountType":"enterprise","isAmbassador":true}]}}`
	ids, err := ParseCodebuddyAccounts(raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("got %d accounts, want 2", len(ids))
	}
	if ids[0].UID != "u-1" || ids[0].Nickname != "alice" || ids[0].UIN != "450703308768" {
		t.Errorf("首个账号解析错: %+v", ids[0])
	}
	if ids[1].AccountType != "enterprise" || !ids[1].IsAmbassador {
		t.Errorf("第二个账号解析错: %+v", ids[1])
	}
}

func TestParseCodebuddyAccountsMissingData(t *testing.T) {
	// data.accounts 缺席必须显式失败：静默当 0 账号会把「响应异常」误报成「无账号」
	if _, err := ParseCodebuddyAccounts(`{"code":0,"msg":"OK","data":{}}`); err == nil {
		t.Fatal("缺 data.accounts 应报错")
	}
	if _, err := ParseCodebuddyAccounts(`{"code":0,"data":{"accounts":null}}`); err == nil {
		t.Fatal("accounts:null 应报错")
	}
}

func TestParseCodebuddyAccountsDedupAndSanitize(t *testing.T) {
	raw := `{"code":0,"data":{"accounts":[
		{"uid":"u-1","nickname":"a\u001b[31mred"},
		{"uid":"u-1","nickname":"dup"},
		{"uid":"","nickname":"nouid"}]}}`
	ids, err := ParseCodebuddyAccounts(raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("去重 + 空 uid 过滤后应剩 1 个，got %d", len(ids))
	}
	if strings.ContainsRune(ids[0].Nickname, 0x1b) {
		t.Errorf("nickname 未消毒: %q", ids[0].Nickname)
	}
}

func TestParseCodebuddyAccountsBusinessError(t *testing.T) {
	_, err := ParseCodebuddyAccounts(`{"code":10001,"msg":"only temporary api keys are supported"}`)
	if err == nil {
		t.Fatal("code!=0 应报错")
	}
	if !strings.Contains(err.Error(), "10001") {
		t.Errorf("错误文本应含业务码: %v", err)
	}
}

// ── ParseCodebuddyCatalog ────────────────────────────────────

const cbCatalogHappy = `{"code":0,"msg":"OK","data":{
	"agents":[{"name":"cli","models":["default-model"]}],"enterpriseId":"",
	"models":[
	 {"id":"default-model","name":"Auto","vendor":"e","maxInputTokens":176000,
	  "maxOutputTokens":24000,"credits":"","isDefault":true},
	 {"id":"fast-model","name":"Fast","vendor":"i","maxInputTokens":200000,
	  "maxOutputTokens":32000,"credits":"x0.34 credits"},
	 {"id":"gpt-6-astra","name":"GPT-6-Astra","maxInputTokens":1000000,
	  "maxOutputTokens":128000,"credits":"x6.67"}],
	"productFeatures":{}}}`

func TestParseCodebuddyCatalogHappy(t *testing.T) {
	cat, err := ParseCodebuddyCatalog(cbCatalogHappy)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(cat.Models) != 3 {
		t.Fatalf("got %d models, want 3", len(cat.Models))
	}
	// 按 id 排序（排序契约）
	if cat.Models[0].ID != "default-model" || cat.Models[1].ID != "fast-model" || cat.Models[2].ID != "gpt-6-astra" {
		t.Errorf("未按 id 排序: %v %v %v", cat.Models[0].ID, cat.Models[1].ID, cat.Models[2].ID)
	}
	def := cat.Models[0]
	if !def.IsDefault {
		t.Error("default-model 应标记 IsDefault")
	}
	if def.HasMultiplier {
		t.Errorf("空 credits 应为未标注，got multiplier=%v", def.CreditsMultiplier)
	}
	fast := cat.Models[1]
	if !fast.HasMultiplier || fast.CreditsMultiplier != 0.34 {
		t.Errorf("fast-model 倍率解析错: %+v", fast)
	}
	if fast.MaxInputTokens != 200000 || fast.MaxOutputTokens != 32000 {
		t.Errorf("fast-model 上下文解析错: %+v", fast)
	}
	astra := cat.Models[2]
	if !astra.HasMultiplier || astra.CreditsMultiplier != 6.67 {
		t.Errorf("gpt-6-astra 倍率解析错: %+v", astra)
	}
}

func TestParseCodebuddyCatalogNullModels(t *testing.T) {
	// models:null 是 X-User-Id 缺失的实测形状——必须显式失败并提示原因，
	// 静默当 0 模型会让用户以为账号没有可用模型。
	_, err := ParseCodebuddyCatalog(`{"code":0,"data":{"models":null}}`)
	if err == nil {
		t.Fatal("models:null 应报错")
	}
	if !strings.Contains(err.Error(), "X-User-Id") {
		t.Errorf("错误文本应提示 X-User-Id 原因: %v", err)
	}
	if _, err := ParseCodebuddyCatalog(`{"code":0,"data":{}}`); err == nil {
		t.Fatal("缺 models 字段应报错")
	}
}

func TestParseCodebuddyCatalogEmptyArray(t *testing.T) {
	// 空数组是合法 0 模型（与 null 区分）
	cat, err := ParseCodebuddyCatalog(`{"code":0,"data":{"models":[]}}`)
	if err != nil {
		t.Fatalf("空数组不应报错: %v", err)
	}
	if len(cat.Models) != 0 {
		t.Errorf("应 0 模型, got %d", len(cat.Models))
	}
}

// ── ParseCodebuddyLedger ─────────────────────────────────────

// cbLedgerLine 构造一行台账记录。
func cbLedgerLine(msgID, model string, ts int64, credit string) string {
	c := "null"
	if credit != "" {
		c = credit
	}
	return `{"id":"row-` + msgID + `","timestamp":` + itoa(ts) + `,"type":"message",` +
		`"providerData":{"conversationRequestId":"cr-` + msgID + `","messageId":"` + msgID + `",` +
		`"model":"` + model + `","requestModelId":"` + model + `",` +
		`"rawUsage":{"prompt_tokens":100,"completion_tokens":10,` +
		`"prompt_cache_hit_tokens":40,"credit":` + c + `}}}`
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// writeCbLedger 在临时目录写一个会话文件（slug 子目录 + sessionId.jsonl）。
func writeCbLedger(t *testing.T, root, slug, session string, lines []string) {
	t.Helper()
	dir := filepath.Join(root, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, session+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestParseCodebuddyLedgerHappy(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	// 三次调用：今日 1 次、5 天前 1 次、40 天前 1 次
	writeCbLedger(t, root, "home-u", "s1", []string{
		cbLedgerLine("m1", "model-a", now.Add(-1*time.Hour).UnixMilli(), "1.50"),
		cbLedgerLine("m2", "model-a", now.Add(-5*24*time.Hour).UnixMilli(), "2.25"),
	})
	writeCbLedger(t, root, "home-u", "s2", []string{
		cbLedgerLine("m3", "model-b", now.Add(-40*24*time.Hour).UnixMilli(), "0.25"),
	})

	l, err := ParseCodebuddyLedger(root, now)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if l.Files != 2 || l.Sessions != 2 {
		t.Errorf("files/sessions = %d/%d, want 2/2", l.Files, l.Sessions)
	}
	if l.Total.Calls != 3 {
		t.Errorf("Total.Calls=%d, want 3", l.Total.Calls)
	}
	if l.Total.Credits != 4.0 {
		t.Errorf("Total.Credits=%v, want 4.00", l.Total.Credits)
	}
	if l.SkippedLines != 0 {
		t.Errorf("SkippedLines=%d, want 0", l.SkippedLines)
	}
	// 时间窗：今日 1.50 / 近7天 3.75 / 近30天 3.75 / 全部 4.00
	want := map[string]struct {
		calls   int64
		credits float64
	}{
		"今日":   {1, 1.50},
		"近7天":  {2, 3.75},
		"近30天": {2, 3.75},
		"全部":   {3, 4.00},
	}
	for _, w := range l.Windows {
		exp, ok := want[w.Label]
		if !ok {
			t.Errorf("意外的时间窗 %q", w.Label)
			continue
		}
		if w.Calls != exp.calls || w.Credits != exp.credits {
			t.Errorf("%s: calls=%d credits=%v, want %d/%v", w.Label, w.Calls, w.Credits, exp.calls, exp.credits)
		}
	}
	// 按模型降序（model-a 3.75 > model-b 0.25）
	if len(l.ByModel) != 2 || l.ByModel[0].Model != "model-a" || l.ByModel[1].Model != "model-b" {
		t.Fatalf("ByModel 排序错: %+v", l.ByModel)
	}
	if l.ByModel[0].Calls != 2 || l.ByModel[0].Credits != 3.75 {
		t.Errorf("model-a 聚合错: %+v", l.ByModel[0])
	}
	if l.LastUsed != now.Add(-1*time.Hour).UnixMilli() {
		t.Errorf("LastUsed 应为最近一次调用时间")
	}
}

func TestParseCodebuddyLedgerMissingRoot(t *testing.T) {
	// 未用过 CLI 是合法状态：空台账 + 无错误
	l, err := ParseCodebuddyLedger(filepath.Join(t.TempDir(), "nope"), time.Now())
	if err != nil {
		t.Fatalf("目录不存在不应报错: %v", err)
	}
	if l.Total.Calls != 0 || l.Files != 0 {
		t.Errorf("应为空台账: %+v", l)
	}
}

func TestParseCodebuddyLedgerDedup(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	line := cbLedgerLine("m1", "model-a", now.UnixMilli(), "1.00")
	// 同一 (conversationRequestId, messageId) 出现三次 → 只计一次
	writeCbLedger(t, root, "home-u", "s1", []string{line, line, line})
	l, err := ParseCodebuddyLedger(root, now)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if l.Total.Calls != 1 {
		t.Errorf("去重后应 1 次, got %d", l.Total.Calls)
	}
	if l.Duplicates != 2 {
		t.Errorf("Duplicates=%d, want 2", l.Duplicates)
	}
	if l.Total.Credits != 1.0 {
		t.Errorf("重复行不应重复计费: %v", l.Total.Credits)
	}
}

func TestParseCodebuddyLedgerZeroCreditIsBilled(t *testing.T) {
	// credit=0 是合法值（免费模型实测回 0）——必须计次，不能被当成「字段缺席」
	root := t.TempDir()
	now := time.Now()
	writeCbLedger(t, root, "home-u", "s1", []string{
		cbLedgerLine("m1", "hy3", now.UnixMilli(), "0"),
	})
	l, err := ParseCodebuddyLedger(root, now)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if l.Total.Calls != 1 || l.Total.Credits != 0 {
		t.Errorf("0 积分行应计 1 次 0 积分, got calls=%d credits=%v", l.Total.Calls, l.Total.Credits)
	}
	if l.UnbilledLines != 0 {
		t.Errorf("credit=0 不应算 unbilled, got %d", l.UnbilledLines)
	}
}

func TestParseCodebuddyLedgerUnbilledVsCorrupt(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeCbLedger(t, root, "home-u", "s1", []string{
		// 无 rawUsage 的合法行（reasoning / 工具结果 / 快照）→ unbilled
		`{"id":"r1","timestamp":` + itoa(now.UnixMilli()) + `,"type":"reasoning"}`,
		// credit 字段缺席 → unbilled
		`{"id":"r2","timestamp":1,"providerData":{"rawUsage":{"prompt_tokens":1}}}`,
		// JSON 损坏（半行）→ corrupt
		`{"id":"r3","providerData":{"rawUsage":{"credit":0.5`,
		// credit 为非法值 → corrupt
		cbLedgerLine("m4", "model-a", now.UnixMilli(), `"oops"`),
		// 正常行
		cbLedgerLine("m5", "model-a", now.UnixMilli(), "0.75"),
	})
	l, err := ParseCodebuddyLedger(root, now)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if l.Total.Calls != 1 || l.Total.Credits != 0.75 {
		t.Errorf("应只计 1 次 0.75, got calls=%d credits=%v", l.Total.Calls, l.Total.Credits)
	}
	if l.UnbilledLines != 2 {
		t.Errorf("UnbilledLines=%d, want 2（合法但不计费）", l.UnbilledLines)
	}
	if l.SkippedLines != 2 {
		t.Errorf("SkippedLines=%d, want 2（损坏行）", l.SkippedLines)
	}
}

func TestParseCodebuddyLedgerMissingTimestamp(t *testing.T) {
	// timestamp 缺失 → 计入「全部」，但不进任何时间窗（不能当成 now 塞进今日）
	root := t.TempDir()
	now := time.Now()
	line := `{"id":"x1","providerData":{"conversationRequestId":"c","messageId":"m",` +
		`"requestModelId":"model-a","rawUsage":{"prompt_tokens":1,"credit":0.30}}}`
	writeCbLedger(t, root, "home-u", "s1", []string{line})
	l, err := ParseCodebuddyLedger(root, now)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if l.Total.Calls != 1 || l.Total.Credits != 0.30 {
		t.Errorf("应计入全部: calls=%d credits=%v", l.Total.Calls, l.Total.Credits)
	}
	if w := models.CodebuddyWindowOf(&l, "今日"); w.Calls != 0 {
		t.Errorf("无时间戳不应进今日窗, got %d", w.Calls)
	}
	if w := models.CodebuddyWindowOf(&l, "全部"); w.Calls != 1 {
		t.Errorf("无时间戳应进全部窗, got %d", w.Calls)
	}
}

func TestParseCodebuddyLedgerModelFallback(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	// requestModelId 空 → 退回 model；两者皆空 → "(未知模型)"
	writeCbLedger(t, root, "home-u", "s1", []string{
		`{"id":"a","timestamp":` + itoa(now.UnixMilli()) + `,"providerData":{"conversationRequestId":"c1",` +
			`"messageId":"m1","model":"fallback-model","rawUsage":{"credit":1}}}`,
		`{"id":"b","timestamp":` + itoa(now.UnixMilli()) + `,"providerData":{"conversationRequestId":"c2",` +
			`"messageId":"m2","rawUsage":{"credit":2}}}`,
	})
	l, err := ParseCodebuddyLedger(root, now)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	names := map[string]bool{}
	for _, m := range l.ByModel {
		names[m.Model] = true
	}
	if !names["fallback-model"] {
		t.Errorf("应退回 model 字段: %+v", l.ByModel)
	}
	if !names["(未知模型)"] {
		t.Errorf("两字段皆空应标 (未知模型): %+v", l.ByModel)
	}
}

func TestParseCodebuddyLedgerIgnoresNonJSONL(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeCbLedger(t, root, "home-u", "s1", []string{cbLedgerLine("m1", "model-a", now.UnixMilli(), "1.00")})
	// 同目录放非 .jsonl 文件与子目录 → 都应忽略
	if err := os.WriteFile(filepath.Join(root, "home-u", "notes.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "home-u", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := ParseCodebuddyLedger(root, now)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if l.Files != 1 {
		t.Errorf("应只扫 1 个 .jsonl, got %d", l.Files)
	}
	if l.Total.Calls != 1 {
		t.Errorf("应只 1 次调用, got %d", l.Total.Calls)
	}
}

// TestParseCodebuddyLedgerRealSnapshot 回归自检：本机真实台账快照（2026-11-05）。
// 若解析规则漂移（漏计/重复计入/时间窗错切），此用例立即失败。
func TestParseCodebuddyLedgerRealSnapshot(t *testing.T) {
	root := os.Getenv("LLM_API_CHECK_CB_LEDGER_ROOT")
	if root == "" {
		t.Skip("未设 LLM_API_CHECK_CB_LEDGER_ROOT（真实台账路径），跳过快照回归")
	}
	now := time.Date(2026, 11, 5, 1, 0, 0, 0, time.Local)
	l, err := ParseCodebuddyLedger(root, now)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if l.Total.Calls != 663 {
		t.Errorf("Total.Calls=%d, want 663", l.Total.Calls)
	}
	if l.Total.Credits < 52.84 || l.Total.Credits > 52.86 {
		t.Errorf("Total.Credits=%v, want ≈52.85", l.Total.Credits)
	}
	if l.SkippedLines != 0 {
		t.Errorf("真实台账不应有损坏行, got %d", l.SkippedLines)
	}
}

// TestCodebuddyAuthErrorText 保证 repo/parsers 共用同一句错误文案。
func TestCodebuddyAuthErrorText(t *testing.T) {
	if !strings.Contains(ErrCodebuddyAuth.Error(), "codebuddy.ai/profile/keys") {
		t.Errorf("认证错误文本应给创建入口: %v", ErrCodebuddyAuth)
	}
	if errors.Is(ErrCodebuddyAuth, ErrCommandaiAuth) {
		t.Error("CodeBuddy 与 Commandai 的认证错误必须不同实例")
	}
}
