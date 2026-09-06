package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
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

// --json 载荷与退出码：积分额度要出得来，且「有额度无清单」不算完全失败。
func TestBaiJSONPointsAndExitCode(t *testing.T) {
	res := app.BaiResult{
		Account: models.BaiAccount{ID: "b1", Name: "免费通道", ApiKey: "sk-secretvalue1234"},
		Points:  &models.BaiPoints{Balance: 27166591, Expiring: 7166591, MonthlySpent: 2833409, HasMonthly: true},
	}
	m := publicBaiResult(res)
	p, ok := m["points"].(*models.BaiPoints)
	if !ok || p == nil {
		t.Fatalf("--json 应含 points: %+v", m)
	}
	if p.Balance != 27166591 || p.Expiring != 7166591 || p.MonthlySpent != 2833409 {
		t.Errorf("points 数值不符: %+v", p)
	}
	if _, has := m["plan"]; has {
		t.Errorf("Plan 为 nil 时不应出现 plan 键: %+v", m)
	}
	// 掩码不外泄：account 段走 publicBaiAccount
	if acct, ok := m["account"].(map[string]any); !ok || strings.Contains(fmt.Sprint(acct), "sk-secretvalue1234") {
		t.Errorf("--json 泄漏明文 key: %+v", m["account"])
	}

	// 退出码：有额度数据 → 0；两路皆空 + 错误 → 1
	if got := exitCodeForResults(app.Result{Bai: []app.BaiResult{
		{Account: res.Account, Points: res.Points, Error: "模型清单失败"}}}); got != 0 {
		t.Errorf("额度已到手时部分失败不应 exit 1，实际 %d", got)
	}
	if got := exitCodeForResults(app.Result{Bai: []app.BaiResult{
		{Account: res.Account, Error: "BAI API Key 无效"}}}); got != 1 {
		t.Errorf("无任何数据且有错误应 exit 1，实际 %d", got)
	}
}
