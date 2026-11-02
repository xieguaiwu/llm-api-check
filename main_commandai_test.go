package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

func TestCommandaiAddAndDetail(t *testing.T) {
	withConfigDir(t)
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "commandai",
		"--name", "命令测试", "--api-key", "sk-cm-a1234567890")
	if code != 0 {
		t.Fatalf("add commandai exit=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "已添加 Commandai") {
		t.Errorf("应提示已添加 Commandai: %s", out)
	}
	// --no-refresh 详情
	code, out, _ = runCLI(t, "", "commandai", "--no-refresh")
	if code != 0 {
		t.Fatalf("commandai --no-refresh exit=%d", code)
	}
	if !strings.Contains(out, "命令测试 (Commandai)") {
		t.Errorf("应显示账号标题: %s", out)
	}
	if !strings.Contains(out, "积分额度") {
		t.Errorf("应显示计费模式: %s", out)
	}
	// 未知名称 → exit 1
	if code, _, _ := runCLI(t, "", "commandai", "查无此人"); code != 1 {
		t.Errorf("未知账号 exit=%d, want 1", code)
	}
	// --json 掩码
	code, out, _ = runCLI(t, "", "--json", "commandai", "--no-refresh")
	if code != 0 {
		t.Fatalf("--json commandai exit=%d", code)
	}
	if strings.Contains(out, "sk-cm-a1234567890") {
		t.Errorf("--json 泄漏明文 key: %s", out)
	}
	// 缺少 key → exit 2
	if code, _, _ := runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "无key"); code != 2 {
		t.Errorf("缺少 key exit=%d, want 2", code)
	}
}

func TestCommandaiRemoveAndRename(t *testing.T) {
	withConfigDir(t)
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "commandai",
		"--name", "命令", "--api-key", "sk-cm-a1234567890")
	if code != 0 {
		t.Fatalf("add commandai exit=%d err=%s", code, errOut)
	}
	code, out, errOut = runCLI(t, "", "accounts", "remove", "--name", "命令")
	if code != 0 {
		t.Fatalf("remove commandai exit=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "已删除 1") {
		t.Errorf("remove 应成功: %s", out)
	}
	runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "命令", "--api-key", "sk-cm-a1234567890")
	code, out, errOut = runCLI(t, "", "accounts", "rename", "--name", "命令", "--new-name", "命令2")
	if code != 0 {
		t.Fatalf("rename commandai exit=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "已重命名 1") {
		t.Errorf("rename 应成功: %s", out)
	}
	code, out, _ = runCLI(t, "", "accounts", "list")
	if code != 0 {
		t.Fatalf("accounts list exit=%d", code)
	}
	if !strings.Contains(out, "命令2") {
		t.Errorf("list 应显示改名后账号: %s", out)
	}
}

func TestCommandaiAddFromEnv(t *testing.T) {
	withConfigDir(t)
	t.Setenv("LLM_API_CHECK_COMMANDAI_API_KEY", "sk-cm-envkey123456")
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "env号")
	if code != 0 {
		t.Fatalf("add from env exit=%d err=%s", code, errOut)
	}
	code, out, _ = runCLI(t, "", "accounts", "list")
	if code != 0 {
		t.Fatalf("list exit=%d", code)
	}
	if !strings.Contains(out, "env号") {
		t.Errorf("list 应含 env 添加的账号: %s", out)
	}
	// 落盘配置里 key 应为环境变量值
	code, out, _ = runCLI(t, "", "accounts", "list", "--json")
	if code != 0 {
		t.Fatalf("list --json exit=%d", code)
	}
	if strings.Contains(out, "sk-cm-envkey123456") {
		t.Errorf("--json 泄漏明文 key: %s", out)
	}
}

// loadTestConfig 读回当前 XDG_CONFIG_HOME 下的落盘配置（命令级用例断言用）。
func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

// 官方环境变量名 COMMAND_CODE_API_KEY 优先于历史名 LLM_API_CHECK_COMMANDAI_API_KEY。
func TestCommandaiAddFromOfficialEnv(t *testing.T) {
	withConfigDir(t)
	t.Setenv("COMMAND_CODE_API_KEY", "sk-cm-official111")
	t.Setenv("LLM_API_CHECK_COMMANDAI_API_KEY", "sk-cm-legacy222")
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "官方env号")
	if code != 0 {
		t.Fatalf("add from env exit=%d err=%s", code, errOut)
	}
	// 读回落盘配置，确认取的是官方名
	cfg := loadTestConfig(t)
	if len(cfg.CommandaiAccounts) != 1 {
		t.Fatalf("accounts: %+v", cfg.CommandaiAccounts)
	}
	if got := cfg.CommandaiAccounts[0].ApiKey; got != "sk-cm-official111" {
		t.Errorf("应优先取 COMMAND_CODE_API_KEY，got %q", got)
	}
}

// 仅设历史名时仍可回退。
func TestCommandaiAddFromLegacyEnvFallback(t *testing.T) {
	withConfigDir(t)
	t.Setenv("COMMAND_CODE_API_KEY", "")
	t.Setenv("LLM_API_CHECK_COMMANDAI_API_KEY", "sk-cm-legacy333")
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "旧env号")
	if code != 0 {
		t.Fatalf("add from legacy env exit=%d err=%s", code, errOut)
	}
	cfg := loadTestConfig(t)
	if len(cfg.CommandaiAccounts) != 1 || cfg.CommandaiAccounts[0].ApiKey != "sk-cm-legacy333" {
		t.Fatalf("legacy env fallback: %+v", cfg.CommandaiAccounts)
	}
}

func TestCommandaiAccountsListMasked(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "掩码号", "--api-key", "sk-cm-secret9876543210")
	code, out, _ := runCLI(t, "", "accounts", "list", "--json")
	if code != 0 {
		t.Fatalf("list --json exit=%d", code)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json: %v", err)
	}
	accs, ok := parsed["commandai_accounts"].([]any)
	if !ok || len(accs) != 1 {
		t.Fatalf("commandai_accounts: %v", parsed["commandai_accounts"])
	}
	acc := accs[0].(map[string]any)
	if strings.Contains(acc["apiKey"].(string), "sk-cm-secret9876543210") {
		t.Errorf("apiKey 应掩码: %v", acc["apiKey"])
	}
	if acc["name"] != "掩码号" {
		t.Errorf("name: %v", acc["name"])
	}
}

func TestCommandaiStatusJSONKey(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "状态号", "--api-key", "sk-cm-a1234567890")
	code, out, _ := runCLI(t, "", "status", "--no-refresh", "--json")
	if code != 0 {
		t.Fatalf("status --json exit=%d", code)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json: %v", err)
	}
	cm, ok := parsed["commandai"].([]any)
	if !ok || len(cm) != 1 {
		t.Fatalf("status 缺 commandai 键: %v", parsed["commandai"])
	}
	acc := cm[0].(map[string]any)["account"].(map[string]any)
	if strings.Contains(acc["apiKey"].(string), "sk-cm-a1234567890") {
		t.Errorf("status --json 泄漏明文 key: %v", acc["apiKey"])
	}
}

func TestCommandaiUsageText(t *testing.T) {
	code, out, _ := runCLI(t, "", "help")
	if code != 0 {
		t.Fatalf("help exit=%d", code)
	}
	for _, want := range []string{
		"commandai [名称|ID]",
		"--type opencode|deepseek|qwen|galaxy|bai|gptzero|longcat|commandai",
		"COMMAND_CODE_API_KEY",
		"LLM_API_CHECK_COMMANDAI_API_KEY",
		"commandcode.ai/studio",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("帮助文本缺 %q:\n%s", want, out)
		}
	}
}

func TestCommandaiFilterByName(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "甲", "--api-key", "sk-cm-a1234567890")
	runCLI(t, "", "accounts", "add", "--type", "commandai", "--name", "乙", "--api-key", "sk-cm-b1234567890")
	code, out, _ := runCLI(t, "", "commandai", "甲", "--no-refresh")
	if code != 0 {
		t.Fatalf("filter exit=%d", code)
	}
	if !strings.Contains(out, "甲 (Commandai)") || strings.Contains(out, "乙 (Commandai)") {
		t.Errorf("应按名过滤: %s", out)
	}
}

// exitCodeForResults 直接测 Commandai 语义（不依赖网络注入）。
func TestExitCodeForResultsCommandai(t *testing.T) {
	// 认证失败 → exit 1（账号级凭据问题，即使清单成功也判失败）
	r0 := app.CommandaiResult{
		Error: parsers.ErrCommandaiAuth.Error(),
		Plan:  &models.CommandaiPlan{Models: []models.CommandaiModel{{ID: "m1"}}},
	}
	if code := exitCodeForResults(app.Result{Commandai: []app.CommandaiResult{r0}}); code != 1 {
		t.Errorf("认证失败应 exit=1, got %d", code)
	}

	// 全无数据 → exit 1
	r1 := app.CommandaiResult{Error: "网络请求失败: ..."}
	if code := exitCodeForResults(app.Result{Commandai: []app.CommandaiResult{r1}}); code != 1 {
		t.Errorf("无数据应 exit=1, got %d", code)
	}

	// 清单成功 + 额度段非认证错误 → exit 0
	r2 := app.CommandaiResult{
		Error: "HTTP 500: boom",
		Plan:  &models.CommandaiPlan{Models: []models.CommandaiModel{{ID: "m1"}}},
	}
	if code := exitCodeForResults(app.Result{Commandai: []app.CommandaiResult{r2}}); code != 0 {
		t.Errorf("清单成功应 exit=0, got %d", code)
	}

	// 有额度数据 → exit 0
	r3 := app.CommandaiResult{
		Error:   "HTTP 500: boom",
		Credits: &models.CommandaiCredits{MonthlyCredits: 10},
	}
	if code := exitCodeForResults(app.Result{Commandai: []app.CommandaiResult{r3}}); code != 0 {
		t.Errorf("有额度数据应 exit=0, got %d", code)
	}
}
