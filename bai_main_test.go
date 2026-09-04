package main

import (
	"strings"
	"testing"
)

// BAI 全链（hermetic，不联网）：add → list → 详情 --no-refresh → rename → remove。
func TestBaiAccountsLifecycle(t *testing.T) {
	withConfigDir(t)

	// add
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "bai",
		"--name", "免费通道", "--api-key", "sk-baitest1234567890")
	if code != 0 {
		t.Fatalf("add exit=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, "已添加白B.AI 账号「免费通道」") {
		t.Errorf("add 输出不符: %s", out)
	}

	// list
	_, out, _ = runCLI(t, "", "accounts", "list")
	if !strings.Contains(out, "白B.AI 账号 (1)") || !strings.Contains(out, "API Key 已配置") {
		t.Errorf("list 应含白B.AI 段: %s", out)
	}

	// 详情 --no-refresh（不联网）：显示副题 + 暂无数据
	code, out, errOut = runCLI(t, "", "bai", "免费通道", "--no-refresh")
	if code != 0 {
		t.Fatalf("bai --no-refresh exit=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, "免费通道 (BAI)") || !strings.Contains(out, "暂无数据") {
		t.Errorf("详情输出不符:\n%s", out)
	}
	if !strings.Contains(out, "免费 0-Credits flash 通道") {
		t.Errorf("详情应含通道副题:\n%s", out)
	}

	// rename
	code, _, _ = runCLI(t, "", "accounts", "rename", "--name", "免费通道", "--new-name", "改名通道")
	if code != 0 {
		t.Fatalf("rename exit=%d", code)
	}
	_, out, _ = runCLI(t, "", "accounts", "list")
	if !strings.Contains(out, "改名通道") {
		t.Errorf("rename 未生效: %s", out)
	}

	// --json 掩码
	code, out, _ = runCLI(t, "", "--json", "accounts", "list")
	if code != 0 || !strings.Contains(out, "sk-b****7890") {
		t.Fatalf("--json list 应含掩码 key: exit=%d out=%s", code, out)
	}
	if strings.Contains(out, "sk-baitest1234567890") {
		t.Errorf("--json 泄漏明文 key: %s", out)
	}

	// remove（跨类型删除路径收编 bai）
	code, out, _ = runCLI(t, "", "accounts", "remove", "--name", "改名通道")
	if code != 0 || !strings.Contains(out, "已删除 1 个账号") {
		t.Errorf("remove 不符: exit=%d out=%s", code, out)
	}
}

// BAI 用法帮助收编
func TestUsageTextMentionsBai(t *testing.T) {
	code, out, _ := runCLI(t, "", "help")
	if code != 0 {
		t.Fatalf("help exit=%d", code)
	}
	for _, want := range []string{"llm-api-check bai", "opencode|deepseek|qwen|galaxy|bai", "LLM_API_CHECK_BAI_API_KEY"} {
		if !strings.Contains(out, want) {
			t.Errorf("帮助文本缺少 %q", want)
		}
	}
}

// add 校验：未知 type 拒绝 bai 以外的值时不得误伤；--type bai 缺 key 走 env。
func TestBaiAddFromEnv(t *testing.T) {
	withConfigDir(t)
	t.Setenv("LLM_API_CHECK_BAI_API_KEY", "sk-fromenv1234567890")
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "bai", "--name", "环境变量号")
	if code != 0 {
		t.Fatalf("add(exit env) exit=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, "已添加白B.AI 账号「环境变量号」") {
		t.Errorf("add 输出不符: %s", out)
	}
	// 清理，防串扰其它用例
	runCLI(t, "", "accounts", "remove", "--name", "环境变量号")
}
