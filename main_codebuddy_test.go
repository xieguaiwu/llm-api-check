package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

func TestCodebuddyAddAndDetail(t *testing.T) {
	withConfigDir(t)
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "codebuddy",
		"--name", "腾讯CB", "--api-key", "ck_test1234567890")
	if code != 0 {
		t.Fatalf("add codebuddy exit=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "已添加 CodeBuddy") {
		t.Errorf("应提示已添加 CodeBuddy: %s", out)
	}
	// --no-refresh 详情（不联网，只验证标题与数据源声明）
	code, out, _ = runCLI(t, "", "codebuddy", "--no-refresh")
	if code != 0 {
		t.Fatalf("codebuddy --no-refresh exit=%d", code)
	}
	if !strings.Contains(out, "腾讯CB (CodeBuddy)") {
		t.Errorf("应显示账号标题: %s", out)
	}
	if !strings.Contains(out, "服务端不提供额度接口") {
		t.Errorf("应声明额度数据源: %s", out)
	}
	// 未知名称 → exit 1
	if code, _, _ := runCLI(t, "", "codebuddy", "查无此人"); code != 1 {
		t.Errorf("未知账号 exit=%d, want 1", code)
	}
	// --json 掩码
	code, out, _ = runCLI(t, "", "--json", "codebuddy", "--no-refresh")
	if code != 0 {
		t.Fatalf("--json codebuddy exit=%d", code)
	}
	if strings.Contains(out, "ck_test1234567890") {
		t.Errorf("--json 泄漏明文 key: %s", out)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("--json 输出非法 JSON: %v\n%s", err, out)
	}
	if _, ok := parsed["codebuddy"]; !ok {
		t.Errorf("--json 应含 codebuddy 键: %s", out)
	}
}

func TestCodebuddyAddMissingKey(t *testing.T) {
	withConfigDir(t)
	// 缺 key → exit 2（非交互终端无法提示）
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "codebuddy", "--name", "无key")
	if code != 2 {
		t.Errorf("缺少 key exit=%d, want 2 (err=%s)", code, errOut)
	}
	if !strings.Contains(errOut, "CODEBUDDY_API_KEY") {
		t.Errorf("错误应提示官方环境变量名: %s", errOut)
	}
}

func TestCodebuddyRemoveAndRename(t *testing.T) {
	withConfigDir(t)
	if code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "codebuddy",
		"--name", "腾讯", "--api-key", "ck_test1234567890"); code != 0 {
		t.Fatalf("add exit=%d err=%s", code, errOut)
	}
	code, out, errOut := runCLI(t, "", "accounts", "rename", "--name", "腾讯", "--new-name", "腾讯改")
	if code != 0 {
		t.Fatalf("rename exit=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "已重命名") {
		t.Errorf("rename 应确认成功: %s", out)
	}
	code, out, errOut = runCLI(t, "", "accounts", "list")
	if code != 0 {
		t.Fatalf("list exit=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "CodeBuddy 账号 (1)") || !strings.Contains(out, "腾讯改") {
		t.Errorf("list 应含 CodeBuddy 段与新名: %s", out)
	}
	code, _, errOut = runCLI(t, "", "accounts", "remove", "--name", "腾讯改")
	if code != 0 {
		t.Fatalf("remove exit=%d err=%s", code, errOut)
	}
	code, out, _ = runCLI(t, "", "--json", "accounts", "list")
	if code != 0 {
		t.Fatalf("list --json exit=%d", code)
	}
	if strings.Contains(out, "腾讯改") {
		t.Errorf("remove 后不应再出现: %s", out)
	}
}

func TestCodebuddyBadKeyExitCode(t *testing.T) {
	// 坏 key 会真连网（httptest 不可注入到 main 的 app.New），
	// 故只验证 --no-refresh 路径不联网、且 exit 0。
	withConfigDir(t)
	if code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "codebuddy",
		"--name", "x", "--api-key", "ck_bad"); code != 0 {
		t.Fatalf("add exit=%d err=%s", code, errOut)
	}
	if code, _, _ := runCLI(t, "", "codebuddy", "--no-refresh"); code != 0 {
		t.Errorf("--no-refresh exit=%d, want 0", code)
	}
}

func TestCodebuddyTypeInUsage(t *testing.T) {
	code, out, _ := runCLI(t, "", "--help")
	if code != 0 {
		t.Fatalf("--help exit=%d", code)
	}
	for _, want := range []string{"llm-api-check codebuddy", "CodeBuddy: --api-key", "CODEBUDDY_API_KEY"} {
		if !strings.Contains(out, want) {
			t.Errorf("--help 缺少 %q", want)
		}
	}
}

func TestPublicCodebuddyAccountMasksKey(t *testing.T) {
	m := publicCodebuddyAccount(codebuddyTestAccount())
	if got := m["apiKey"]; got != "ck_t****7890" {
		t.Errorf("apiKey 掩码错: %v", got)
	}
}

func codebuddyTestAccount() models.CodebuddyAccount {
	return models.CodebuddyAccount{ID: "id1", Name: "cb", ApiKey: "ck_test1234567890"}
}
