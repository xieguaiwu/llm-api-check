package main

import (
	"strings"
	"testing"
)

func TestUsageTextMentionsGptzero(t *testing.T) {
	if !strings.Contains(usageText, "gptzero") {
		t.Errorf("usageText 应提及 gptzero")
	}
	if !strings.Contains(usageText, "LLM_API_CHECK_GPTZERO_API_KEY") {
		t.Errorf("usageText 应提及环境变量")
	}
}

func TestGptzeroAccountsLifecycle(t *testing.T) {
	withConfigDir(t)

	// add
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "gptzero",
		"--name", "论文扫", "--api-key", "adfc62c318f14a16a8843c952b56ea5b")
	if code != 0 {
		t.Fatalf("add exit=%d err=%q", code, errOut)
	}
	if !strings.Contains(out, "已添加 GPTZero 账号「论文扫」") {
		t.Errorf("add 输出不符: %s", out)
	}

	// list（文本）
	_, out, _ = runCLI(t, "", "accounts", "list")
	if !strings.Contains(out, "GPTZero 账号 (1)") || !strings.Contains(out, "API Key 已配置") {
		t.Errorf("list 输出不符: %s", out)
	}

	// list --json 掩码
	code, out, _ = runCLI(t, "", "--json", "accounts", "list")
	if code != 0 {
		t.Fatalf("list --json exit=%d", code)
	}
	if strings.Contains(out, "adfc62c318f14a16a8843c952b56ea5b") {
		t.Errorf("list --json 泄漏明文 key:\n%s", out)
	}
	if !strings.Contains(out, "gptzero_accounts") {
		t.Errorf("list --json 应含 gptzero_accounts:\n%s", out)
	}

	// add --json 掩码
	code, out, _ = runCLI(t, "", "--json", "accounts", "add", "--type", "gptzero",
		"--name", "第二把", "--api-key", "adfc62c318f14a16a8843c952b56ea5b")
	if code != 0 {
		t.Fatalf("add --json exit=%d err=%q", code, errOut)
	}
	if strings.Contains(out, "adfc62c318f14a16a8843c952b56ea5b") {
		t.Errorf("add --json 泄漏明文 key:\n%s", out)
	}
	if !strings.Contains(out, "gptzero_account") || !strings.Contains(out, "adfc****") {
		t.Errorf("add --json 应含掩码账号:\n%s", out)
	}

	// rename
	code, out, _ = runCLI(t, "", "accounts", "rename", "--name", "第二把", "--new-name", "备用")
	if code != 0 {
		t.Fatalf("rename exit=%d err=%q", code, out)
	}

	// remove（按新名）
	code, out, _ = runCLI(t, "", "accounts", "remove", "--name", "备用")
	if code != 0 || !strings.Contains(out, "已删除 1 个账号") {
		t.Fatalf("remove exit=%d out=%q", code, out)
	}

	// remove 后只剩 1 个
	_, out, _ = runCLI(t, "", "accounts", "list")
	if !strings.Contains(out, "GPTZero 账号 (1)") {
		t.Errorf("remove 后应剩 1 个 GPTZero 账号:\n%s", out)
	}
}

func TestGptzeroAddBadType(t *testing.T) {
	withConfigDir(t)
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "gptzero2", "--name", "x")
	if code != 2 {
		t.Fatalf("非法 type 应 exit 2: %d", code)
	}
	if !strings.Contains(errOut, "gptzero") {
		t.Errorf("错误提示应列 gptzero: %s", errOut)
	}
}

func TestGptzeroNoRefreshNoAccounts(t *testing.T) {
	withConfigDir(t)
	// 未配置账号：--no-refresh 空输出 exit 0（无账号不是错误）
	code, out, errOut := runCLI(t, "", "gptzero", "--no-refresh")
	if code != 0 {
		t.Fatalf("空账号 --no-refresh exit=%d err=%q out=%q", code, errOut, out)
	}
}

func TestGptzeroCommandWired(t *testing.T) {
	// 命令存在性：不配置账号直接调用应 exit 0 而非「未知命令」（exit 2）
	withConfigDir(t)
	code, _, errOut := runCLI(t, "", "gptzero")
	if code == 2 && strings.Contains(errOut, "未知命令") {
		t.Fatalf("gptzero 命令未接线: %s", errOut)
	}
}
