package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 用隔离 XDG_CONFIG_HOME 跑命令级测试，避免污染真实配置
func withConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	// 默认关闭 Bailian CLI 通道：保证既有用例 hermetic（不依赖本机是否安装 bailian-cli）。
	// 专门测 CLI 通道的用例需显式 t.Setenv("LLM_API_CHECK_QWEN_CLI", "on")。
	if os.Getenv("LLM_API_CHECK_QWEN_CLI") == "" {
		t.Setenv("LLM_API_CHECK_QWEN_CLI", "off")
	}
	var out, errB bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errB)
	return code, out.String(), errB.String()
}

func TestVersion(t *testing.T) {
	code, out, _ := runCLI(t, "", "--version")
	if code != 0 {
		t.Fatalf("--version exit=%d, want 0", code)
	}
	if !strings.Contains(out, "llm-api-check "+version) {
		t.Errorf("版本输出不符: %q", out)
	}
}

func TestUnknownCommandExit2(t *testing.T) {
	code, _, errOut := runCLI(t, "", "frobnicate")
	if code != 2 {
		t.Fatalf("未知命令 exit=%d, want 2", code)
	}
	if !strings.Contains(errOut, "未知命令") {
		t.Errorf("stderr 应含未知命令提示: %q", errOut)
	}
}

func TestStatusEmptyConfigExit0(t *testing.T) {
	withConfigDir(t)
	code, out, _ := runCLI(t, "", "status", "--no-refresh")
	if code != 0 {
		t.Fatalf("空配置 status exit=%d, want 0", code)
	}
	if !strings.Contains(out, "未配置任何账号") {
		t.Errorf("应提示未配置账号: %q", out)
	}
}

func TestStatusJSONEmptyLists(t *testing.T) {
	withConfigDir(t)
	code, out, _ := runCLI(t, "", "--json", "status", "--no-refresh")
	if code != 0 {
		t.Fatalf("exit=%d, want 0", code)
	}
	var v struct {
		DeepSeek []any `json:"deepseek"`
		Accounts []any `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json 输出非法: %v\n%s", err, out)
	}
	if v.DeepSeek == nil || v.Accounts == nil {
		t.Errorf("空列表应为 [] 而非 null: %s", out)
	}
}

func TestAccountsAddAndListJSONMasksSecrets(t *testing.T) {
	dir := withConfigDir(t)
	code, out, _ := runCLI(t, "", "accounts", "add", "--type", "deepseek",
		"--name", "测试", "--api-key", "sk-testsecret123456", "--platform-token", "tok-secret-abc")
	if code != 0 {
		t.Fatalf("add exit=%d, want 0; out=%s", code, out)
	}
	// 配置文件真实保存（未掩码），JSON 输出掩码
	raw, err := os.ReadFile(filepath.Join(dir, "llm-api-check", "config.json"))
	if err != nil {
		t.Fatalf("配置文件未写入: %v", err)
	}
	if !strings.Contains(string(raw), "sk-testsecret123456") {
		t.Errorf("配置文件应存明文凭据: %s", raw)
	}
	code, out, _ = runCLI(t, "", "--json", "accounts", "list")
	if code != 0 {
		t.Fatalf("list exit=%d, want 0", code)
	}
	if strings.Contains(out, "sk-testsecret123456") {
		t.Errorf("JSON 输出泄漏明文凭据: %s", out)
	}
	if !strings.Contains(out, "sk-t****3456") {
		t.Errorf("JSON 输出应含掩码凭据: %s", out)
	}
	if strings.Contains(out, "tok-secret-abc") {
		t.Errorf("JSON 输出泄漏平台 token: %s", out)
	}
}

func TestAccountsAddMissingCredentialNonTTY(t *testing.T) {
	withConfigDir(t)
	// stdin 为 /dev/null 语义：非 TTY 且必填缺失 → 明确报错（momus P1-2 回归）
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "opencode", "--name", "X")
	if code != 2 {
		t.Fatalf("exit=%d, want 2", code)
	}
	if !strings.Contains(errOut, "缺少凭据 --go-api-key") {
		t.Errorf("应提示缺少凭据: %q", errOut)
	}
}

func TestAccountsAddOptionalFieldEOFSkip(t *testing.T) {
	withConfigDir(t)
	code, out, _ := runCLI(t, "", "accounts", "add", "--type", "deepseek",
		"--name", "T2", "--api-key", "sk-abcdefgh1234")
	if code != 0 {
		t.Fatalf("可选字段 EOF 应跳过（exit=%d, out=%s）", code, out)
	}
	if !strings.Contains(out, "已添加 DeepSeek 账号") {
		t.Errorf("应成功添加: %q", out)
	}
}

func TestNoColorFlagNoANSI(t *testing.T) {
	withConfigDir(t)
	// 无 NO_COLOR 环境变量但传 --no-color：输出不含 ESC
	t.Setenv("NO_COLOR", "")
	code, out, _ := runCLI(t, "", "--no-color", "status", "--no-refresh")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("--no-color 输出含 ANSI 转义: %q", out)
	}
}

// ── Qwen provider ──────────────────────────────────────────────

func TestQwenAddAndListMasksSecrets(t *testing.T) {
	dir := withConfigDir(t)
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "qwen",
		"--name", "订阅号", "--api-key", "sk-sp-qwen1234567890",
		"--console-cookie", "login_aliyunid_csrf=csrfabc123456; cna=anon", "--region", "cn")
	if code != 0 {
		t.Fatalf("add exit=%d, want 0; out=%s err=%s", code, out, errOut)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "llm-api-check", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "sk-sp-qwen1234567890") || !strings.Contains(string(raw), "csrfabc123456") {
		t.Errorf("配置文件应存真实凭据: %s", raw)
	}
	if !strings.Contains(string(raw), `"region": "cn-beijing"`) {
		t.Errorf("--region cn 应归一化为 cn-beijing: %s", raw)
	}
	code, out, _ = runCLI(t, "", "--json", "accounts", "list")
	if code != 0 {
		t.Fatalf("list exit=%d", code)
	}
	for _, leak := range []string{"sk-sp-qwen1234567890", "csrfabc123456"} {
		if strings.Contains(out, leak) {
			t.Errorf("JSON 输出泄漏 %q: %s", leak, out)
		}
	}
	if !strings.Contains(out, "sk-s****7890") {
		t.Errorf("应含掩码 API Key: %s", out)
	}
	if !strings.Contains(out, `"qwen_accounts"`) {
		t.Errorf("JSON 应含 qwen_accounts: %s", out)
	}
	// 文本总览提示 Cookie 已配置
	_, out, _ = runCLI(t, "", "accounts", "list")
	if !strings.Contains(out, "已配置 Cookie") {
		t.Errorf("list 应显示配额能力: %s", out)
	}
}

func TestQwenAddWithoutCookieHints(t *testing.T) {
	withConfigDir(t)
	t.Setenv("LLM_API_CHECK_QWEN_API_KEY", "sk-sp-envkey123456")
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "qwen", "--name", "仅密钥")
	if code != 0 {
		t.Fatalf("add exit=%d; out=%s err=%s", code, out, errOut)
	}
	if !strings.Contains(out, "未配控制台 Cookie") {
		t.Errorf("无 Cookie 时应提示能力受限: %s", out)
	}
	if !strings.Contains(out, "中国大陆（北京）") && !strings.Contains(out, "id=") {
		t.Errorf("应打印新增结果: %s", out)
	}
}

func TestQwenAddBadRegionExit2(t *testing.T) {
	withConfigDir(t)
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "qwen",
		"--name", "X", "--api-key", "sk-sp-x", "--region", "mars")
	if code != 2 {
		t.Fatalf("非法区域 exit=%d, want 2", code)
	}
	if !strings.Contains(errOut, "区域") {
		t.Errorf("应提示区域不支持: %q", errOut)
	}
}

func TestQwenAddMissingKeyNonTTY(t *testing.T) {
	withConfigDir(t)
	t.Setenv("LLM_API_CHECK_QWEN_API_KEY", "")
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "qwen", "--name", "X")
	if code != 2 || !strings.Contains(errOut, "缺少凭据 --api-key") {
		t.Errorf("应提示缺少 Qwen API Key: code=%d err=%q", code, errOut)
	}
}

func TestStatusJSONIncludesQwenSection(t *testing.T) {
	withConfigDir(t)
	code, out, errOut := runCLI(t, "", "--json", "status", "--no-refresh")
	if code != 0 {
		t.Fatalf("status exit=%d err=%s", code, errOut)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("--json status 非法 JSON: %v\n%s", err, out)
	}
	q, ok := parsed["qwen"].([]any)
	if !ok || len(q) != 0 {
		t.Errorf("qwen 字段应为空数组: %v", parsed["qwen"])
	}
}

func TestQwenDetailNoRefreshAndFilter(t *testing.T) {
	withConfigDir(t)
	if _, out, errOut := runCLI(t, "", "accounts", "add", "--type", "qwen", "--name", "订阅号",
		"--api-key", "sk-sp-a1234567890", "--console-cookie", "sec_token=tok", "--region", "intl"); errOut != "" ||
		!strings.Contains(out, "已添加") {
		t.Fatalf("add 失败: %s %s", out, errOut)
	}
	code, out, _ := runCLI(t, "", "qwen", "订阅号", "--no-refresh")
	if code != 0 {
		t.Fatalf("qwen --no-refresh exit=%d", code)
	}
	if !strings.Contains(out, "订阅号 (Qwen · 国际（新加坡）)") {
		t.Errorf("应显示国际区域标题: %s", out)
	}
	if !strings.Contains(out, "Token Plan · 订阅") {
		t.Errorf("应显示订阅分区: %s", out)
	}
	// 未知名称 → exit 1
	if code, _, _ := runCLI(t, "", "qwen", "查无此人"); code != 1 {
		t.Errorf("未知账号 exit=%d, want 1", code)
	}
	// --json 掩码
	code, out, _ = runCLI(t, "", "--json", "qwen", "--no-refresh")
	if code != 0 {
		t.Fatalf("--json qwen exit=%d", code)
	}
	if strings.Contains(out, "sk-sp-a1234567890") || strings.Contains(out, `"tok"`) {
		t.Errorf("--json 泄漏明文凭据: %s", out)
	}
}

// 缺 API Key 的账号刷新时无网络调用即失败 → 退出码 1
func TestQwenRefreshMissingKeyExit1(t *testing.T) {
	dir := withConfigDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "llm-api-check"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw := `{"deepseek_accounts":[],"accounts":[],"qwen_accounts":[{"id":"q1","name":"空密钥","apiKey":"","consoleCookie":"","region":"cn-beijing"}],"last_update":{}}`
	if err := os.WriteFile(filepath.Join(dir, "llm-api-check", "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "", "qwen")
	if code != 1 {
		t.Fatalf("完全失败应 exit=1，实得 %d; out=%s", code, out)
	}
	if !strings.Contains(out+errOut, "未配置 API Key") {
		t.Errorf("应提示未配置 API Key: out=%q err=%q", out, errOut)
	}
}

func TestQwenRemoveAndRenameAcrossTypes(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "qwen", "--name", "同号", "--api-key", "sk-sp-q1234567890")
	runCLI(t, "", "accounts", "add", "--type", "deepseek", "--name", "同号", "--api-key", "sk-d1234567890")
	code, out, _ := runCLI(t, "", "accounts", "rename", "--name", "同号", "--new-name", "改名")
	if code != 0 || !strings.Contains(out, "已重命名 2 个账号") {
		t.Errorf("跨类型重命名不符: %s", out)
	}
	_, out, _ = runCLI(t, "", "accounts", "list")
	if !strings.Contains(out, "改名") {
		t.Errorf("重命名未生效: %s", out)
	}
	code, out, _ = runCLI(t, "", "accounts", "remove", "--name", "改名")
	if code != 0 || !strings.Contains(out, "已删除 2 个账号") {
		t.Errorf("跨类型删除不符: %s", out)
	}
}

// 位置参数在 flag 之前也要能识别（三个详情命令共用）
func TestDetailFlagAfterName(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "deepseek", "--name", "DS", "--api-key", "sk-d1234567890")
	for _, c := range []string{"deepseek", "opencode", "qwen"} {
		code, _, errOut := runCLI(t, "", c, "DS", "--no-refresh")
		if code != 0 && c == "deepseek" {
			t.Errorf("%s 名称后置 flag 解析失败: exit=%d err=%q", c, code, errOut)
		}
	}
}

func TestUsageTextMentionsQwen(t *testing.T) {
	code, out, _ := runCLI(t, "", "help")
	if code != 0 {
		t.Fatalf("help exit=%d", code)
	}
	for _, want := range []string{"llm-api-check qwen", "--type opencode|deepseek|qwen", "LLM_API_CHECK_QWEN_API_KEY", "--console-cookie"} {
		if !strings.Contains(out, want) {
			t.Errorf("帮助文本缺少 %q", want)
		}
	}
}

// Bailian CLI 通道端到端：env 指定 fake CLI + 显式开启 → qwen 刷新走 CLI 拿配额。
// fake CLI 脚本只回配额 JSON；API Key 用无效占位（Plan 会 401，但配额行应出现）。
func TestQwenCLIChannelEndToEnd(t *testing.T) {
	dir := withConfigDir(t)
	fake := filepath.Join(dir, "fake-bailian")
	script := `#!/bin/sh
if [ "$1" = "usage" ] && [ "$2" = "token-plan" ]; then
  echo '{"per5HourPercentage":0.5,"per5HourResetTime":1790000000000,"per1WeekPercentage":0.7,"per1WeekResetTime":1791000000000}'
  exit 0
fi
if [ "$1" = "usage" ] && [ "$2" = "summary" ]; then
  echo '{"period":{"start":"2026-08-22","end":"2026-08-29","days":7},"freeTier":[{"model":"qwen3.8-flash","type":"Text","remaining":986768,"total":1000000,"remainingPercent":98.7,"expires":"2026-11-28"}],"usage":{"modelsCalled":2,"successfulCalls":14,"usages":[{"key":"total_token","value":14785,"unit":"tokens","label":"Total Tokens"}]}}'
  exit 0
fi
echo '{"error":{"code":3,"message":"No console access token found."}}'
exit 0
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LLM_API_CHECK_QWEN_CLI", "on")
	t.Setenv("LLM_API_CHECK_BL_BIN", fake)

	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "qwen",
		"--name", "订阅号", "--api-key", "sk-sp-clitest1234567890", "--region", "cn-beijing")
	if code != 0 {
		t.Fatalf("add exit=%d err=%q", code, errOut)
	}
	code, out, errOut = runCLI(t, "", "qwen", "订阅号")
	if code != 0 {
		t.Fatalf("qwen exit=%d err=%q out=%s", code, errOut, out)
	}
	for _, want := range []string{"50%", "70%", "5小时", "7天"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q: %s", want, out)
		}
	}
	if strings.Contains(out, "需控制台 Cookie") {
		t.Errorf("CLI 通道可用时不应提示缺 Cookie: %s", out)
	}

	// --stats 附加用量分析（fake CLI 的 summary 分支）
	code, out, errOut = runCLI(t, "", "qwen", "订阅号", "--stats")
	if code != 0 {
		t.Fatalf("qwen --stats exit=%d err=%q", code, errOut)
	}
	for _, want := range []string{"用量分析", "14,785", "qwen3.8-flash", "98.7%", "2026-08-22 ~ 2026-08-29（7 天）"} {
		if !strings.Contains(out, want) {
			t.Errorf("--stats 输出缺少 %q: %s", want, out)
		}
	}
}
