// Command llm-api-check 复刻 Android app「API Checkers」数据逻辑的 Go CLI：
// 查看 DeepSeek API（余额 + 消费明细）与 OpenCode（Go usage 三窗口 + Zen
// billing）的使用情况，支持多账号。
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/render"
)

// version 编译期可注入（-ldflags "-X main.version=x.y.z"）
var version = "1.0.0"

// 凭据环境变量（flag 缺失时回退，再回退 TTY 交互提示）
const (
	envGoAPIKey    = "LLM_API_CHECK_GO_API_KEY"
	envWorkspaceID = "LLM_API_CHECK_WORKSPACE_ID"
	envAuthCookie  = "LLM_API_CHECK_AUTH_COOKIE"
	envDsAPIKey    = "LLM_API_CHECK_DEEPSEEK_API_KEY"
	envPlatformTok = "LLM_API_CHECK_PLATFORM_TOKEN"
)

const usageText = `llm-api-check — 查看 DeepSeek API 与 OpenCode 使用情况（复刻 Android 版 API Checkers）

用法:
  llm-api-check                            刷新全部账号并显示总览（等同 status）
  llm-api-check status [--no-refresh]      总览；默认刷新，--no-refresh 只读配置不联网
  llm-api-check deepseek [名称|ID]         DeepSeek 账号详情（可过滤名字/id，缺省全部）
  llm-api-check opencode [名称|ID]         OpenCode 账号详情（可过滤名字/id，缺省全部）
  llm-api-check accounts list              列出所有账号
  llm-api-check accounts add --type opencode|deepseek --name 名称 [凭据 flags]
  llm-api-check accounts remove --id ID | --name 名称
  llm-api-check accounts rename --id ID | --name 名称 --new-name 新名称
  llm-api-check config path                打印配置文件路径
  llm-api-check --version                  打印版本

全局 flags:
  --json        所有输出为 JSON（便于脚本）
  --no-color    禁用 ANSI 颜色（同 NO_COLOR 环境变量）

凭据 flags（缺失时按此顺序回退：环境变量 → TTY 交互提示；非 TTY 报错）:
  OpenCode: --go-api-key / --workspace-id / --auth-cookie
            环境变量 LLM_API_CHECK_GO_API_KEY / LLM_API_CHECK_WORKSPACE_ID / LLM_API_CHECK_AUTH_COOKIE
  DeepSeek: --api-key / --platform-token
            环境变量 LLM_API_CHECK_DEEPSEEK_API_KEY / LLM_API_CHECK_PLATFORM_TOKEN

退出码: 0 成功；1 任一账号完全失败；2 用法错误
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run 便于单测：参数与标准输入输出注入
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// 全局 flag 扫描（可出现在子命令前后）
	var jsonOut, noColor bool
	var rest []string
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--no-color":
			noColor = true
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		return cmdStatus(nil, stdin, stdout, stderr, jsonOut, noColor)
	}
	switch rest[0] {
	case "--version", "-V", "version":
		fmt.Fprintf(stdout, "llm-api-check %s\n", version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usageText)
		return 0
	case "status":
		return cmdStatus(rest[1:], stdin, stdout, stderr, jsonOut, noColor)
	case "deepseek":
		return cmdDeepSeek(rest[1:], stdin, stdout, stderr, jsonOut, noColor)
	case "opencode":
		return cmdOpenCode(rest[1:], stdin, stdout, stderr, jsonOut, noColor)
	case "accounts":
		return cmdAccounts(rest[1:], stdin, stdout, stderr, jsonOut)
	case "config":
		return cmdConfig(rest[1:], stdout, stderr, jsonOut)
	default:
		fmt.Fprintf(stderr, "未知命令：%s\n\n%s", rest[0], usageText)
		return 2
	}
}

// colorizer 根据 NO_COLOR 环境变量（存在即禁用，含空值）与 --no-color flag 决定
func colorizer(noColor bool) render.Colorizer {
	_, noColorEnv := os.LookupEnv("NO_COLOR")
	return render.Colorizer{Disabled: noColor || noColorEnv}
}

// ── status ────────────────────────────────────────────────────

func cmdStatus(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut, noColor bool) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noRefresh := fs.Bool("no-refresh", false, "不刷新，只显示已配置账号")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check status [--no-refresh]")
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "用法: llm-api-check status [--no-refresh]")
		return 2
	}
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	warnSecurity(stderr)
	a := app.New(cfg)
	var res app.Result
	if *noRefresh {
		res = resultsFromAccounts(cfg)
	} else {
		if res, err = a.RefreshAll(); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		if err := cfg.Save(path); err != nil {
			fmt.Fprintf(stderr, "警告: %v\n", err)
		}
	}
	if jsonOut {
		writeJSON(stdout, map[string]any{
			"deepseek":         sliceOrEmpty(res.DeepSeek),
			"accounts":         sliceOrEmpty(res.Accounts),
			"last_updated":     unixMillisOrZero(res.LastUpdated),
			"security_warning": config.SecurityWarning,
		})
		return exitCodeForResults(res)
	}
	fmt.Fprint(stdout, render.RenderOverview(res.DeepSeek, res.Accounts, res.LastUpdated, colorizer(noColor)))
	return exitCodeForResults(res)
}

// resultsFromAccounts 只读配置构造空结果骨架（--no-refresh，不发起网络）
func resultsFromAccounts(cfg *config.Config) app.Result {
	res := app.Result{}
	for _, acc := range cfg.DeepSeekAccounts {
		res.DeepSeek = append(res.DeepSeek, app.DeepSeekResult{Account: acc})
	}
	for _, acc := range cfg.Accounts {
		res.Accounts = append(res.Accounts, app.AccountResult{Account: acc})
	}
	return res
}

// exitCodeForResults 任一账号完全失败（无任何数据且有错误）→ 1
func exitCodeForResults(res app.Result) int {
	for _, r := range res.DeepSeek {
		if r.Error != "" && r.Balance == nil && r.Cost == nil {
			return 1
		}
	}
	for _, r := range res.Accounts {
		if r.Error != "" && r.GoUsage == nil && r.ZenBilling == nil {
			return 1
		}
	}
	return 0
}

// ── deepseek / opencode 详情 ──────────────────────────────────

func cmdDeepSeek(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut, noColor bool) int {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "用法: llm-api-check deepseek [名称|ID]")
		return 2
	}
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	warnSecurity(stderr)
	accounts := cfg.DeepSeekAccounts
	if len(args) == 1 {
		filtered, ok := filterDeepSeek(accounts, args[0])
		if !ok {
			fmt.Fprintf(stderr, "账号不存在: %s\n", args[0])
			return 1
		}
		accounts = filtered
	}
	a := app.New(cfg)
	results := make([]app.DeepSeekResult, 0, len(accounts))
	for _, acc := range accounts {
		r, err := a.RefreshDeepSeek(acc.ID)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		results = append(results, r)
	}
	res := app.Result{DeepSeek: results}
	if jsonOut {
		if len(results) == 1 {
			writeJSON(stdout, map[string]any{"deepseek": results[0]})
		} else {
			writeJSON(stdout, map[string]any{"deepseek": results})
		}
		return exitCodeForResults(res)
	}
	c := colorizer(noColor)
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(render.RenderDeepSeekDetail(r, c))
	}
	fmt.Fprint(stdout, b.String())
	return exitCodeForResults(res)
}

func cmdOpenCode(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut, noColor bool) int {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "用法: llm-api-check opencode [名称|ID]")
		return 2
	}
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	warnSecurity(stderr)
	accounts := cfg.Accounts
	if len(args) == 1 {
		filtered, ok := filterAccounts(accounts, args[0])
		if !ok {
			fmt.Fprintf(stderr, "账号不存在: %s\n", args[0])
			return 1
		}
		accounts = filtered
	}
	a := app.New(cfg)
	results := make([]app.AccountResult, 0, len(accounts))
	now := time.Now()
	for _, acc := range accounts {
		r, err := a.RefreshAccount(acc.ID)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		results = append(results, r)
	}
	res := app.Result{Accounts: results}
	if jsonOut {
		if len(results) == 1 {
			writeJSON(stdout, map[string]any{"account": results[0]})
		} else {
			writeJSON(stdout, map[string]any{"accounts": results})
		}
		return exitCodeForResults(res)
	}
	c := colorizer(noColor)
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(render.RenderAccountDetail(r, now, c))
	}
	fmt.Fprint(stdout, b.String())
	return exitCodeForResults(res)
}

// filterDeepSeek 按 id 精确匹配，否则按 name 精确匹配
func filterDeepSeek(list []models.DeepSeekAccount, q string) ([]models.DeepSeekAccount, bool) {
	var out []models.DeepSeekAccount
	for _, a := range list {
		if a.ID == q || a.Name == q {
			out = append(out, a)
		}
	}
	return out, len(out) > 0
}

// filterAccounts 按 id 精确匹配，否则按 name 精确匹配
func filterAccounts(list []models.Account, q string) ([]models.Account, bool) {
	var out []models.Account
	for _, a := range list {
		if a.ID == q || a.Name == q {
			out = append(out, a)
		}
	}
	return out, len(out) > 0
}

// ── accounts ──────────────────────────────────────────────────

func cmdAccounts(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut bool) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "用法: llm-api-check accounts <list|add|remove|rename>")
		return 2
	}
	switch args[0] {
	case "list":
		return cmdAccountsList(stdout, stderr, jsonOut)
	case "add":
		return cmdAccountsAdd(args[1:], stdin, stdout, stderr, jsonOut)
	case "remove":
		return cmdAccountsRemove(args[1:], stdout, stderr, jsonOut)
	case "rename":
		return cmdAccountsRename(args[1:], stdout, stderr, jsonOut)
	default:
		fmt.Fprintf(stderr, "未知子命令：%s\n用法: llm-api-check accounts <list|add|remove|rename>\n", args[0])
		return 2
	}
}

func cmdAccountsList(stdout, stderr io.Writer, jsonOut bool) int {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	if jsonOut {
		writeJSON(stdout, map[string]any{
			"deepseek_accounts": cfg.DeepSeekAccounts,
			"accounts":          cfg.Accounts,
		})
		return 0
	}
	fmt.Fprintf(stdout, "DeepSeek 账号 (%d):\n", len(cfg.DeepSeekAccounts))
	for _, a := range cfg.DeepSeekAccounts {
		tok := "未配置"
		if a.HasToken() {
			tok = "已配置"
		}
		fmt.Fprintf(stdout, "  %s  %s  [平台 Token: %s]\n", a.ID, a.Name, tok)
	}
	fmt.Fprintf(stdout, "OpenCode 账号 (%d):\n", len(cfg.Accounts))
	for _, a := range cfg.Accounts {
		zen := "未配置"
		if a.HasZen() {
			zen = "已配置"
		}
		fmt.Fprintf(stdout, "  %s  %s  [Zen: %s]\n", a.ID, a.Name, zen)
	}
	return 0
}

func cmdAccountsAdd(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut bool) int {
	fs := flag.NewFlagSet("accounts add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	typ := fs.String("type", "", "账号类型: opencode|deepseek")
	name := fs.String("name", "", "账号名称")
	goKey := fs.String("go-api-key", "", "OpenCode Go API Key")
	wsID := fs.String("workspace-id", "", "OpenCode Workspace ID（可选）")
	cookie := fs.String("auth-cookie", "", "OpenCode Auth Cookie（可选）")
	apiKey := fs.String("api-key", "", "DeepSeek API Key")
	ptok := fs.String("platform-token", "", "DeepSeek 平台 Token（可选）")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check accounts add --type opencode|deepseek --name 名称 [凭据 flags]")
		return 2
	}
	if *typ != "opencode" && *typ != "deepseek" {
		fmt.Fprintln(stderr, "错误: --type 必须是 opencode 或 deepseek")
		fmt.Fprintln(stderr, "用法: llm-api-check accounts add --type opencode|deepseek --name 名称 [凭据 flags]")
		return 2
	}
	if strings.TrimSpace(*name) == "" {
		v, err := promptTTY(stdin, stdout, "账号名称: ", false)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 2
		}
		if strings.TrimSpace(v) == "" {
			fmt.Fprintln(stderr, "错误: 账号名称不能为空")
			return 2
		}
		name = &v
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	if *typ == "opencode" {
		key, err := resolveSecret(*goKey, "go-api-key", envGoAPIKey, "Go API Key: ", true, stdin, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 2
		}
		ws, err := resolveSecret(*wsID, "workspace-id", envWorkspaceID, "Workspace ID（可选，回车跳过）: ", false, stdin, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 2
		}
		ck, err := resolveSecret(*cookie, "auth-cookie", envAuthCookie, "Auth Cookie（可选，回车跳过）: ", false, stdin, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 2
		}
		acc := models.Account{
			ID:          config.NewID(),
			Name:        strings.TrimSpace(*name),
			GoApiKey:    key,
			WorkspaceId: ws,
			AuthCookie:  ck,
		}
		cfg.SaveAccount(acc)
		if err := cfg.Save(config.DefaultPath()); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		if jsonOut {
			writeJSON(stdout, map[string]any{"account": acc})
		} else {
			fmt.Fprintf(stdout, "已添加 OpenCode 账号「%s」(id=%s)\n", acc.Name, acc.ID)
		}
		return 0
	}
	// deepseek
	key, err := resolveSecret(*apiKey, "api-key", envDsAPIKey, "DeepSeek API Key: ", true, stdin, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 2
	}
	tok, err := resolveSecret(*ptok, "platform-token", envPlatformTok, "平台 Token（可选，回车跳过）: ", false, stdin, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 2
	}
	acc := models.DeepSeekAccount{
		ID:            config.NewID(),
		Name:          strings.TrimSpace(*name),
		ApiKey:        key,
		PlatformToken: tok,
	}
	cfg.SaveDeepSeekAccount(acc)
	if err := cfg.Save(config.DefaultPath()); err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	if jsonOut {
		writeJSON(stdout, map[string]any{"deepseek_account": acc})
	} else {
		fmt.Fprintf(stdout, "已添加 DeepSeek 账号「%s」(id=%s)\n", acc.Name, acc.ID)
	}
	return 0
}

func cmdAccountsRemove(args []string, stdout, stderr io.Writer, jsonOut bool) int {
	fs := flag.NewFlagSet("accounts remove", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	id := fs.String("id", "", "按 id 删除")
	name := fs.String("name", "", "按名称删除")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check accounts remove --id ID | --name 名称")
		return 2
	}
	if *id == "" && *name == "" {
		fmt.Fprintln(stderr, "错误: 必须提供 --id 或 --name")
		fmt.Fprintln(stderr, "用法: llm-api-check accounts remove --id ID | --name 名称")
		return 2
	}
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	removed := 0
	// id 优先；否则按名称删除两类列表中所有匹配项
	match := func(accID, accName string) bool {
		if *id != "" {
			return accID == *id
		}
		return accName == *name
	}
	var keptDS []models.DeepSeekAccount
	for _, a := range cfg.DeepSeekAccounts {
		if match(a.ID, a.Name) {
			removed++
			continue
		}
		keptDS = append(keptDS, a)
	}
	cfg.DeepSeekAccounts = keptDS
	var kept []models.Account
	for _, a := range cfg.Accounts {
		if match(a.ID, a.Name) {
			removed++
			continue
		}
		kept = append(kept, a)
	}
	cfg.Accounts = kept
	if removed == 0 {
		fmt.Fprintln(stderr, "错误: 未找到匹配的账号")
		return 1
	}
	if err := cfg.Save(config.DefaultPath()); err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	if jsonOut {
		writeJSON(stdout, map[string]any{"removed": removed})
	} else {
		fmt.Fprintf(stdout, "已删除 %d 个账号\n", removed)
	}
	return 0
}

func cmdAccountsRename(args []string, stdout, stderr io.Writer, jsonOut bool) int {
	fs := flag.NewFlagSet("accounts rename", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	id := fs.String("id", "", "按 id 定位")
	name := fs.String("name", "", "按名称定位")
	newName := fs.String("new-name", "", "新名称")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check accounts rename --id ID | --name 名称 --new-name 新名称")
		return 2
	}
	if *id == "" && *name == "" {
		fmt.Fprintln(stderr, "错误: 必须提供 --id 或 --name")
		fmt.Fprintln(stderr, "用法: llm-api-check accounts rename --id ID | --name 名称 --new-name 新名称")
		return 2
	}
	if strings.TrimSpace(*newName) == "" {
		fmt.Fprintln(stderr, "错误: --new-name 不能为空")
		return 2
	}
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	renamed := 0
	match := func(accID, accName string) bool {
		if *id != "" {
			return accID == *id
		}
		return accName == *name
	}
	for i := range cfg.DeepSeekAccounts {
		a := &cfg.DeepSeekAccounts[i]
		if match(a.ID, a.Name) {
			a.Name = strings.TrimSpace(*newName)
			renamed++
		}
	}
	for i := range cfg.Accounts {
		a := &cfg.Accounts[i]
		if match(a.ID, a.Name) {
			a.Name = strings.TrimSpace(*newName)
			renamed++
		}
	}
	if renamed == 0 {
		fmt.Fprintln(stderr, "错误: 未找到匹配的账号")
		return 1
	}
	if err := cfg.Save(config.DefaultPath()); err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	if jsonOut {
		writeJSON(stdout, map[string]any{"renamed": renamed})
	} else {
		fmt.Fprintf(stdout, "已重命名 %d 个账号\n", renamed)
	}
	return 0
}

// ── config path ───────────────────────────────────────────────

func cmdConfig(args []string, stdout, stderr io.Writer, jsonOut bool) int {
	if len(args) == 0 || args[0] != "path" {
		fmt.Fprintln(stderr, "用法: llm-api-check config path")
		return 2
	}
	if jsonOut {
		writeJSON(stdout, map[string]any{"path": config.DefaultPath()})
	} else {
		fmt.Fprintln(stdout, config.DefaultPath())
	}
	return 0
}

// ── 凭据输入 ──────────────────────────────────────────────────

// resolveSecret 凭据解析顺序：flag → 环境变量 → TTY 交互提示。
// 非 TTY 且 required 凭据缺失 → 报错提示用法（interactive-cli-design 降级）。
func resolveSecret(flagVal, flagName, envName, prompt string, required bool, stdin io.Reader, stdout io.Writer) (string, error) {
	if strings.TrimSpace(flagVal) != "" {
		return strings.TrimSpace(flagVal), nil
	}
	if v := os.Getenv(envName); v != "" {
		return v, nil
	}
	if !isTTY(stdin) {
		if required {
			return "", fmt.Errorf("缺少凭据 --%s（或环境变量 %s）：非交互终端无法提示，请显式传入",
				flagName, envName)
		}
		return "", nil
	}
	v, err := promptTTY(stdin, stdout, prompt, true)
	if err != nil {
		return "", err
	}
	if !required && strings.TrimSpace(v) == "" {
		return "", nil
	}
	return strings.TrimSpace(v), nil
}

// isTTY stdin 是否为终端（ModeCharDevice）
func isTTY(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// promptTTY 从 TTY 读取一行；secret 时尝试 stty -echo 关闭回显
// （终端不支持 stty 时忽略——输入不回显需终端支持，可改用环境变量）。
func promptTTY(stdin io.Reader, stdout io.Writer, prompt string, secret bool) (string, error) {
	fmt.Fprint(stdout, prompt)
	var restore func()
	if secret {
		if f, ok := stdin.(*os.File); ok {
			off := exec.Command("stty", "-echo")
			off.Stdin = f
			if off.Run() == nil {
				restore = func() {
					on := exec.Command("stty", "echo")
					on.Stdin = f
					_ = on.Run()
				}
			}
		}
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if restore != nil {
		restore()
		fmt.Fprintln(stdout)
	}
	if err != nil && line == "" {
		return "", fmt.Errorf("读取输入失败: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// ── 输出小工具 ────────────────────────────────────────────────

// warnSecurity SecurityWarning 非空时输出到 stderr（TTY 下黄色）
func warnSecurity(stderr io.Writer) {
	if config.SecurityWarning == "" {
		return
	}
	c := colorizer(false)
	msg := "警告: " + config.SecurityWarning
	if f, ok := stderr.(*os.File); ok {
		if fi, err := f.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
			msg = c.Yellow(msg)
		}
	}
	fmt.Fprintln(stderr, msg)
}

// sliceOrEmpty nil slice 序列化为 [] 而非 null
func sliceOrEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// unixMillisOrZero 零值时间输出 0 而非负时间戳
func unixMillisOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "JSON 输出失败: %v\n", err)
	}
}
