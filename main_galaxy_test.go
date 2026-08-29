package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// galaxyEnv 设置智星云凭据环境变量（模拟 CI/脚本注入），返回 cleanup
func galaxyEnv(t *testing.T, ak, sk string) {
	t.Helper()
	if ak != "" {
		t.Setenv(envGalaxyAK, ak)
	}
	if sk != "" {
		t.Setenv(envGalaxySK, sk)
	}
}

func TestGalaxyAddAndListMasksSecrets(t *testing.T) {
	dir := withConfigDir(t)
	code, out, errOut := runCLI(t, "",
		"accounts", "add", "--type", "galaxy", "--name", "训练集群",
		"--access-key", "aabbccddeeff0011", "--secret-key", "2233445566778899deadbeef")
	if code != 0 {
		t.Fatalf("add exit=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "已添加智星云账号") {
		t.Errorf("输出异常: %q", out)
	}

	// 配置文件里保留明文（下次要用），但必须 0600
	raw, rerr := os.ReadFile(filepath.Join(dir, "llm-api-check", "config.json"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !strings.Contains(string(raw), "aabbccddeeff0011") {
		t.Error("配置应写入 AccessKey")
	}

	code, out, _ = runCLI(t, "", "accounts", "list", "--json")
	if code != 0 {
		t.Fatalf("list exit=%d", code)
	}
	if strings.Contains(out, "aabbccddeeff0011") || strings.Contains(out, "2233445566778899") {
		t.Errorf("--json 泄漏明文凭据: %s", out)
	}
	var parsed struct {
		GalaxyAccounts []map[string]any `json:"galaxy_accounts"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("JSON 解析失败: %v\n%s", err, out)
	}
	if len(parsed.GalaxyAccounts) != 1 {
		t.Fatalf("应列出 1 个智星云账号: %s", out)
	}
	got := parsed.GalaxyAccounts[0]
	if got["accessKey"] != "aabb****0011" || got["secretKey"] != "2233****beef" {
		t.Errorf("掩码不符: %v", got)
	}
	if got["name"] != "训练集群" {
		t.Errorf("名称丢失: %v", got)
	}
}

func TestGalaxyAddFromEnvVars(t *testing.T) {
	withConfigDir(t)
	galaxyEnv(t, "ak-from-env-1234567", "sk-from-env-7654321")
	code, out, errOut := runCLI(t, "", "accounts", "add", "--type", "galaxy", "--name", "env账号")
	if code != 0 {
		t.Fatalf("环境变量凭据 add exit=%d stderr=%s out=%s", code, errOut, out)
	}
	if !strings.Contains(out, "已添加智星云账号") {
		t.Errorf("输出异常: %q", out)
	}
}

func TestGalaxyAddMissingSecretNonTTY(t *testing.T) {
	withConfigDir(t)
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "galaxy",
		"--name", "n", "--access-key", "only-ak")
	if code != 2 {
		t.Fatalf("缺 SecretKey 且非 TTY 应 exit=2，实得 %d", code)
	}
	if !strings.Contains(errOut, "secret-key") {
		t.Errorf("应指明缺失的 flag: %q", errOut)
	}
}

func TestGalaxyAddBadTypeExit2(t *testing.T) {
	withConfigDir(t)
	code, _, errOut := runCLI(t, "", "accounts", "add", "--type", "galaxyx", "--name", "n")
	if code != 2 || !strings.Contains(errOut, "galaxy") {
		t.Errorf("未知 type 应在用法里列出 galaxy: exit=%d err=%q", code, errOut)
	}
}

// TestGalaxyRefreshMissingSecretExit1 缺凭据时 galaxy 命令必须报错并非零退出，
// 而不是静默显示空白卡（脚本用退出码判活）
func TestGalaxyRefreshMissingSecretExit1(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "galaxy", "--name", "half",
		"--access-key", "ak-only", "--secret-key", "placeholder")
	// 覆盖配置：直接删掉 SecretKey 更真实
	cfgPath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "llm-api-check", "config.json")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	empty := strings.Replace(string(raw), `"secretKey": "placeholder"`, `"secretKey": ""`, 1)
	if err := os.WriteFile(cfgPath, []byte(empty), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI(t, "", "galaxy")
	if code != 1 {
		t.Fatalf("完全失败应 exit=1，实得 %d，输出:\n%s", code, out)
	}
	if !strings.Contains(out, "未配置 AccessKey/SecretKey") {
		t.Errorf("应给出配置指引: %q", out)
	}
}

func TestGalaxyNoRefreshListsAccount(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "galaxy", "--name", "训练集群",
		"--access-key", "ak1234567890", "--secret-key", "sk1234567890")
	code, out, errOut := runCLI(t, "", "galaxy", "--no-refresh")
	if code != 0 {
		t.Fatalf("--no-refresh 不该联网失败: exit=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "训练集群 (智星云)") {
		t.Errorf("应显示账号标题: %q", out)
	}
}

func TestGalaxyFlagAfterName(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "galaxy", "--name", "a1",
		"--access-key", "ak1234567890", "--secret-key", "sk1234567890")
	// 全部补 --no-refresh：只断言 flag 解析，不依赖网络（不加的话会真的去打平台接口）
	for _, args := range [][]string{
		{"galaxy", "a1", "--limit", "3", "--no-refresh"},
		{"galaxy", "a1", "--no-refresh", "--limit", "3"},
		{"galaxy", "a1", "--no-refresh"},
	} {
		code, out, errOut := runCLI(t, "", args...)
		if code == 2 {
			t.Fatalf("%v 被误判为用法错误: %s", args, errOut)
		}
		if code != 0 {
			t.Errorf("%v exit=%d out=%s err=%s", args, code, out, errOut)
		}
	}
}

// TestMoveFlags flag 与位置参数混排时的归位规则
func TestMoveFlags(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"名称", "--no-refresh"}, []string{"--no-refresh", "名称"}},
		{[]string{"名称", "--limit", "3"}, []string{"--limit", "3", "名称"}},
		{[]string{"名称", "--limit", "3", "--no-refresh"}, []string{"--limit", "3", "--no-refresh", "名称"}},
		{[]string{"--stats", "名称"}, []string{"--stats", "名称"}},
		{[]string{"名称"}, []string{"名称"}},
		{[]string{"名称", "--limit"}, []string{"名称", "--limit"}}, // 缺值：交给 flag 包报错
	}
	for _, tc := range cases {
		got := moveFlags(tc.in)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("moveFlags(%v)=%v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestGalaxyUnknownNameExit1(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "galaxy", "--name", "a1",
		"--access-key", "ak1234567890", "--secret-key", "sk1234567890")
	code, _, errOut := runCLI(t, "", "galaxy", "不存在的账号")
	if code != 1 || !strings.Contains(errOut, "账号不存在") {
		t.Errorf("未知名称应 exit=1 并提示: exit=%d err=%q", code, errOut)
	}
}

func TestStatusJSONIncludesGalaxySection(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "galaxy", "--name", "g",
		"--access-key", "ak1234567890", "--secret-key", "sk1234567890")
	code, out, _ := runCLI(t, "", "status", "--no-refresh", "--json")
	if code != 0 {
		t.Fatalf("status exit=%d", code)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("JSON 解析失败: %v\n%s", err, out)
	}
	sec, ok := parsed["galaxy"].([]any)
	if !ok || len(sec) != 1 {
		t.Fatalf("status --json 应含 galaxy 段: %s", out)
	}
	// key 集合恒在：无数据时也保有 account 结构
	m := sec[0].(map[string]any)
	if _, has := m["account"]; !has {
		t.Errorf("缺 account 键: %v", m)
	}
	if strings.Contains(out, "ak1234567890") || strings.Contains(out, "sk1234567890") {
		t.Errorf("凭据泄漏: %s", out)
	}
}

func TestGalaxyRemoveAndRenameAcrossTypes(t *testing.T) {
	withConfigDir(t)
	runCLI(t, "", "accounts", "add", "--type", "galaxy", "--name", "共享名",
		"--access-key", "ak1234567890", "--secret-key", "sk1234567890")
	runCLI(t, "", "accounts", "add", "--type", "deepseek", "--name", "共享名",
		"--api-key", "sk-ds1234567890")
	code, out, _ := runCLI(t, "", "accounts", "rename", "--name", "共享名", "--new-name", "改名后")
	if code != 0 {
		t.Fatalf("rename exit=%d out=%s", code, out)
	}
	if !strings.Contains(out, "已重命名 2 个账号") {
		t.Errorf("应同时命中 galaxy 与 deepseek: %q", out)
	}
	code, out, _ = runCLI(t, "", "accounts", "remove", "--name", "改名后")
	if code != 0 || !strings.Contains(out, "已删除 2 个账号") {
		t.Errorf("remove 应跨类型删除: exit=%d out=%s", code, out)
	}
}

func TestUsageTextMentionsGalaxy(t *testing.T) {
	code, out, _ := runCLI(t, "", "help")
	if code != 0 {
		t.Fatalf("help exit=%d", code)
	}
	for _, want := range []string{"llm-api-check galaxy", "--access-key", "--secret-key",
		envGalaxyAK, envGalaxySK, "智星云"} {
		if !strings.Contains(out, want) {
			t.Errorf("帮助文本缺少 %q", want)
		}
	}
}
