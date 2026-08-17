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
