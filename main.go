// Command llm-api-check 复刻 Android app「API Checkers」数据逻辑的 Go CLI：
// 查看 DeepSeek API（余额 + 消费明细）、OpenCode（Go usage 三窗口 + Zen
// billing）、Qwen Token Plan（模型清单 + 5 小时/7 天 配额窗口）与智星云
// AI Galaxy 算力云（余额 + 云主机实例状态）的使用情况，支持多账号。
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
var version = "1.3.0"

// 凭据环境变量（flag 缺失时回退，再回退 TTY 交互提示）
const (
	envGoAPIKey    = "LLM_API_CHECK_GO_API_KEY"
	envWorkspaceID = "LLM_API_CHECK_WORKSPACE_ID"
	envAuthCookie  = "LLM_API_CHECK_AUTH_COOKIE"
	envDsAPIKey    = "LLM_API_CHECK_DEEPSEEK_API_KEY"
	envPlatformTok = "LLM_API_CHECK_PLATFORM_TOKEN"
	envQwenAPIKey  = "LLM_API_CHECK_QWEN_API_KEY"
	envQwenCookie  = "LLM_API_CHECK_QWEN_COOKIE"
	envQwenRegion  = "LLM_API_CHECK_QWEN_REGION"
	envGalaxyAK    = "LLM_API_CHECK_GALAXY_ACCESS_KEY"
	envGalaxySK    = "LLM_API_CHECK_GALAXY_SECRET_KEY"
	envBaiAPIKey   = "LLM_API_CHECK_BAI_API_KEY"
)

const usageText = `llm-api-check — 查看 DeepSeek API、OpenCode、Qwen Token Plan、智星云算力云与白B.AI 使用情况（复刻 Android 版 API Checkers）

用法:
  llm-api-check                            刷新全部账号并显示总览（等同 status）
  llm-api-check status [--no-refresh]      总览；默认刷新，--no-refresh 只读配置不联网
  llm-api-check deepseek [名称|ID]         DeepSeek 账号详情（可过滤名字/id，缺省全部）
  llm-api-check opencode [名称|ID]         OpenCode 账号详情（可过滤名字/id，缺省全部）
  llm-api-check qwen [名称|ID] [--stats]   Qwen 账号详情（--stats 附加 7 天用量分析与免费额度）
  llm-api-check galaxy [名称|ID] [--limit N]   智星云余额 + 云主机实例状态（--limit 列出实例数，默认 10）
  llm-api-check bai [名称|ID]              白B.AI 账号详情：模型清单 + 免费通道状态（可过滤名字/id，缺省全部）
  llm-api-check accounts list              列出所有账号
  llm-api-check accounts add --type opencode|deepseek|qwen|galaxy|bai --name 名称 [凭据 flags]
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
  Qwen:     --api-key / --console-cookie / --region
            环境变量 LLM_API_CHECK_QWEN_API_KEY / LLM_API_CHECK_QWEN_COOKIE / LLM_API_CHECK_QWEN_REGION
            --api-key 为订阅密钥（sk-sp- 开头，与区域绑定）；--region 可选 cn-beijing（默认）/ap-southeast-1
            --console-cookie 可选：阿里云百炼控制台 Cookie，提供后才能看到 5 小时/7 天 配额窗口
            （Cookie 从已登录的 bailian.console.aliyun.com 订阅页网络请求里复制）
  智星云:   --access-key / --secret-key
            环境变量 LLM_API_CHECK_GALAXY_ACCESS_KEY / LLM_API_CHECK_GALAXY_SECRET_KEY
            控制台「开放API → AccessKey管理」创建（需先完成实名认证）；看余额 + 实例状态与到期时间
  BAI:      --api-key
            环境变量 LLM_API_CHECK_BAI_API_KEY
            白B.AI（api.b.ai）key，chat.b.ai 侧栏 API → Create API Key 创建；
            平台仅开放推理路径，只看模型清单与免费通道状态（无配额数据可查）

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
	case "qwen":
		return cmdQwen(rest[1:], stdin, stdout, stderr, jsonOut, noColor)
	case "galaxy":
		return cmdGalaxy(rest[1:], stdin, stdout, stderr, jsonOut, noColor)
	case "bai":
		return cmdBai(rest[1:], stdin, stdout, stderr, jsonOut, noColor)
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
	warnSecurity(stderr, noColor)
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
		// security_warning 在 Save 前快照（Save 会清空警告，momus P2-3）
		sw := config.SecurityWarning
		writeJSON(stdout, map[string]any{
			"deepseek":         sliceOrEmpty(publicDeepSeekResults(res.DeepSeek)),
			"accounts":         sliceOrEmpty(publicAccountResults(res.Accounts)),
			"qwen":             sliceOrEmpty(publicQwenResults(res.Qwen)),
			"galaxy":           sliceOrEmpty(publicGalaxyResults(res.Galaxy)),
			"bai":              sliceOrEmpty(publicBaiResults(res.Bai)),
			"last_updated":     unixMillisOrZero(res.LastUpdated),
			"security_warning": sw,
		})
		return exitCodeForResults(res)
	}
	fmt.Fprint(stdout, render.RenderOverview(res, time.Now(), colorizer(noColor)))
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
	for _, acc := range cfg.QwenAccounts {
		res.Qwen = append(res.Qwen, app.QwenResult{Account: acc})
	}
	for _, acc := range cfg.GalaxyAccounts {
		res.Galaxy = append(res.Galaxy, app.GalaxyResult{Account: acc})
	}
	for _, acc := range cfg.BaiAccounts {
		res.Bai = append(res.Bai, app.BaiResult{Account: acc})
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
	for _, r := range res.Qwen {
		if r.Error != "" && r.Plan == nil && r.Usage == nil {
			return 1
		}
	}
	for _, r := range res.Galaxy {
		if r.Error != "" && r.Balance == nil && r.Status == nil && len(r.Instances) == 0 {
			return 1
		}
	}
	for _, r := range res.Bai {
		if r.Error != "" && r.Plan == nil {
			return 1
		}
	}
	return 0
}

// ── deepseek / opencode 详情 ──────────────────────────────────

func cmdDeepSeek(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut, noColor bool) int {
	fs := flag.NewFlagSet("deepseek", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noRefresh := fs.Bool("no-refresh", false, "不刷新，只显示已配置账号")
	if err := fs.Parse(moveFlags(args)); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check deepseek [名称|ID] [--no-refresh]")
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "用法: llm-api-check deepseek [名称|ID] [--no-refresh]")
		return 2
	}
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	warnSecurity(stderr, noColor)
	accounts := cfg.DeepSeekAccounts
	if fs.NArg() == 1 {
		filtered, ok := filterDeepSeek(accounts, fs.Arg(0))
		if !ok {
			fmt.Fprintf(stderr, "账号不存在: %s\n", fs.Arg(0))
			return 1
		}
		accounts = filtered
	}
	a := app.New(cfg)
	results := make([]app.DeepSeekResult, 0, len(accounts))
	for _, acc := range accounts {
		var r app.DeepSeekResult
		if *noRefresh {
			r = app.DeepSeekResult{Account: acc}
		} else if r, err = a.RefreshDeepSeek(acc.ID); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		results = append(results, r)
	}
	res := app.Result{DeepSeek: results}
	if jsonOut {
		if len(results) == 1 {
			writeJSON(stdout, map[string]any{"deepseek": publicDeepSeekResult(results[0])})
		} else {
			writeJSON(stdout, map[string]any{"deepseek": publicDeepSeekResults(results)})
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
	fs := flag.NewFlagSet("opencode", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noRefresh := fs.Bool("no-refresh", false, "不刷新，只显示已配置账号")
	if err := fs.Parse(moveFlags(args)); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check opencode [名称|ID] [--no-refresh]")
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "用法: llm-api-check opencode [名称|ID] [--no-refresh]")
		return 2
	}
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	warnSecurity(stderr, noColor)
	accounts := cfg.Accounts
	if fs.NArg() == 1 {
		filtered, ok := filterAccounts(accounts, fs.Arg(0))
		if !ok {
			fmt.Fprintf(stderr, "账号不存在: %s\n", fs.Arg(0))
			return 1
		}
		accounts = filtered
	}
	a := app.New(cfg)
	results := make([]app.AccountResult, 0, len(accounts))
	now := time.Now()
	for _, acc := range accounts {
		var r app.AccountResult
		if *noRefresh {
			r = app.AccountResult{Account: acc}
		} else if r, err = a.RefreshAccount(acc.ID); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		results = append(results, r)
	}
	res := app.Result{Accounts: results}
	if jsonOut {
		if len(results) == 1 {
			writeJSON(stdout, map[string]any{"account": publicAccountResult(results[0])})
		} else {
			writeJSON(stdout, map[string]any{"accounts": publicAccountResults(results)})
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

// moveFlags 把散布在位置参数之后的 flag 提到参数列表最前：Go flag 包遇到首个
// 非 flag 参数即停止解析，否则 `llm-api-check qwen 名称 --no-refresh` /
// `--stats` / `galaxy 名称 --limit 5` 会被误判为多余参数（exit 2）。
// 带值 flag（--limit）连同紧跟的一个 token 一起移动；多个 flag 全部前置。
func moveFlags(args []string) []string {
	boolFlags := map[string]bool{"--no-refresh": true, "--stats": true}
	valueFlags := map[string]bool{"--limit": true}
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case boolFlags[a]:
			flags = append(flags, a)
		case valueFlags[a] && i+1 < len(args):
			flags = append(flags, a, args[i+1])
			i++
		default:
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

// filterDeepSeek 按 id 或 name 精确匹配（任一命中即包含）
func filterDeepSeek(list []models.DeepSeekAccount, q string) ([]models.DeepSeekAccount, bool) {
	var out []models.DeepSeekAccount
	for _, a := range list {
		if a.ID == q || a.Name == q {
			out = append(out, a)
		}
	}
	return out, len(out) > 0
}

// filterAccounts 按 id 或 name 精确匹配（任一命中即包含）
func filterAccounts(list []models.Account, q string) ([]models.Account, bool) {
	var out []models.Account
	for _, a := range list {
		if a.ID == q || a.Name == q {
			out = append(out, a)
		}
	}
	return out, len(out) > 0
}

// ── qwen 详情 ──────────────────────────────────────────────

func cmdQwen(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut, noColor bool) int {
	fs := flag.NewFlagSet("qwen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noRefresh := fs.Bool("no-refresh", false, "不刷新，只显示已配置账号")
	stats := fs.Bool("stats", false, "显示用量分析（7 天 token 统计 + 免费额度）")
	if err := fs.Parse(moveFlags(args)); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check qwen [名称|ID] [--no-refresh] [--stats]")
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "用法: llm-api-check qwen [名称|ID] [--no-refresh] [--stats]")
		return 2
	}
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	warnSecurity(stderr, noColor)
	accounts := cfg.QwenAccounts
	if fs.NArg() == 1 {
		filtered, ok := filterQwen(accounts, fs.Arg(0))
		if !ok {
			fmt.Fprintf(stderr, "账号不存在: %s\n", fs.Arg(0))
			return 1
		}
		accounts = filtered
	}
	a := app.New(cfg)
	results := make([]app.QwenResult, 0, len(accounts))
	for _, acc := range accounts {
		var r app.QwenResult
		if *noRefresh {
			// CLIEnabled/登录命令仍需如实上报：否则装了 CLI 也会显示
			// 「需控制台 Cookie 或 Bailian CLI」（误导用户去抓 Cookie）
			r = app.QwenResult{Account: acc,
				CLIEnabled:    a.Repos.Qwen.CLIEnabled(),
				CLILoginCmd:   a.Repos.Qwen.CLILoginCmd(),
				CLIInstallCmd: a.Repos.Qwen.CLIInstallCmd(),
			}
		} else if r, err = a.RefreshQwen(acc.ID); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		if *stats && !*noRefresh {
			sr, sErr := a.RefreshQwenStats(acc.ID)
			if sErr != nil {
				r.Error = joinText(r.Error, sErr.Error())
			} else {
				r.Stats = sr.Stats
			}
		}
		results = append(results, r)
	}
	res := app.Result{Qwen: results}
	if jsonOut {
		if len(results) == 1 {
			writeJSON(stdout, map[string]any{"qwen": publicQwenResult(results[0])})
		} else {
			writeJSON(stdout, map[string]any{"qwen": publicQwenResults(results)})
		}
		return exitCodeForResults(res)
	}
	c := colorizer(noColor)
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(render.RenderQwenDetail(r, time.Now(), c))
	}
	fmt.Fprint(stdout, b.String())
	return exitCodeForResults(res)
}

// filterQwen 按 id 或 name 精确匹配（任一命中即包含）
func filterQwen(list []models.QwenAccount, q string) ([]models.QwenAccount, bool) {
	var out []models.QwenAccount
	for _, a := range list {
		if a.ID == q || a.Name == q {
			out = append(out, a)
		}
	}
	return out, len(out) > 0
}

// ── 智星云详情 ───────────────────────────────────

func cmdGalaxy(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut, noColor bool) int {
	fs := flag.NewFlagSet("galaxy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noRefresh := fs.Bool("no-refresh", false, "不刷新，只显示已配置账号")
	limit := fs.Int("limit", 10, "列出的活跃实例数（≤100）")
	if err := fs.Parse(moveFlags(args)); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check galaxy [名称|ID] [--no-refresh] [--limit N]")
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "用法: llm-api-check galaxy [名称|ID] [--no-refresh] [--limit N]")
		return 2
	}
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	warnSecurity(stderr, noColor)
	accounts := cfg.GalaxyAccounts
	if fs.NArg() == 1 {
		filtered, ok := filterGalaxy(accounts, fs.Arg(0))
		if !ok {
			fmt.Fprintf(stderr, "账号不存在: %s\n", fs.Arg(0))
			return 1
		}
		accounts = filtered
	}
	a := app.New(cfg)
	results := make([]app.GalaxyResult, 0, len(accounts))
	for _, acc := range accounts {
		var r app.GalaxyResult
		if *noRefresh {
			r = app.GalaxyResult{Account: acc}
		} else if r, err = a.RefreshGalaxy(acc.ID, *limit); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		results = append(results, r)
	}
	res := app.Result{Galaxy: results}
	if jsonOut {
		if len(results) == 1 {
			writeJSON(stdout, map[string]any{"galaxy": publicGalaxyResult(results[0])})
		} else {
			writeJSON(stdout, map[string]any{"galaxy": publicGalaxyResults(results)})
		}
		return exitCodeForResults(res)
	}
	c := colorizer(noColor)
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(render.RenderGalaxyDetail(r, time.Now(), c))
	}
	fmt.Fprint(stdout, b.String())
	return exitCodeForResults(res)
}

// filterGalaxy 按 id 或 name 精确匹配（任一命中即包含）
func filterGalaxy(list []models.GalaxyAccount, q string) ([]models.GalaxyAccount, bool) {
	var out []models.GalaxyAccount
	for _, a := range list {
		if a.ID == q || a.Name == q {
			out = append(out, a)
		}
	}
	return out, len(out) > 0
}

// ── 白B.AI 详情 ──────────────────────────────────────────────

func cmdBai(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut, noColor bool) int {
	fs := flag.NewFlagSet("bai", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noRefresh := fs.Bool("no-refresh", false, "不刷新，只显示已配置账号")
	if err := fs.Parse(moveFlags(args)); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check bai [名称|ID] [--no-refresh]")
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "用法: llm-api-check bai [名称|ID] [--no-refresh]")
		return 2
	}
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	warnSecurity(stderr, noColor)
	accounts := cfg.BaiAccounts
	if fs.NArg() == 1 {
		filtered, ok := filterBai(accounts, fs.Arg(0))
		if !ok {
			fmt.Fprintf(stderr, "账号不存在: %s\n", fs.Arg(0))
			return 1
		}
		accounts = filtered
	}
	a := app.New(cfg)
	results := make([]app.BaiResult, 0, len(accounts))
	for _, acc := range accounts {
		var r app.BaiResult
		if *noRefresh {
			r = app.BaiResult{Account: acc}
		} else if r, err = a.RefreshBai(acc.ID); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		results = append(results, r)
	}
	res := app.Result{Bai: results}
	if jsonOut {
		if len(results) == 1 {
			writeJSON(stdout, map[string]any{"bai": publicBaiResult(results[0])})
		} else {
			writeJSON(stdout, map[string]any{"bai": publicBaiResults(results)})
		}
		return exitCodeForResults(res)
	}
	c := colorizer(noColor)
	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(render.RenderBaiDetail(r, c))
	}
	fmt.Fprint(stdout, b.String())
	return exitCodeForResults(res)
}

// filterBai 按 id 或 name 精确匹配（任一命中即包含）
func filterBai(list []models.BaiAccount, q string) ([]models.BaiAccount, bool) {
	var out []models.BaiAccount
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
	warnSecurity(stderr, false)
	if jsonOut {
		writeJSON(stdout, map[string]any{
			"deepseek_accounts": sliceOrEmpty(publicDeepSeekAccounts(cfg.DeepSeekAccounts)),
			"accounts":          sliceOrEmpty(publicAccounts(cfg.Accounts)),
			"qwen_accounts":     sliceOrEmpty(publicQwenAccounts(cfg.QwenAccounts)),
			"galaxy_accounts":   sliceOrEmpty(publicGalaxyAccounts(cfg.GalaxyAccounts)),
			"bai_accounts":      sliceOrEmpty(publicBaiAccounts(cfg.BaiAccounts)),
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
	fmt.Fprintf(stdout, "Qwen 账号 (%d):\n", len(cfg.QwenAccounts))
	for _, a := range cfg.QwenAccounts {
		quota := "未配置 Cookie（仅模型清单）"
		if a.HasCookie() {
			quota = "已配置 Cookie（含配额窗口）"
		}
		fmt.Fprintf(stdout, "  %s  %s  [%s · %s]\n", a.ID, a.Name, render.RegionDisplayName(a.Region), quota)
	}
	fmt.Fprintf(stdout, "智星云账号 (%d):\n", len(cfg.GalaxyAccounts))
	for _, a := range cfg.GalaxyAccounts {
		ready := "凭据不完整"
		if strings.TrimSpace(a.AccessKey) != "" && strings.TrimSpace(a.SecretKey) != "" {
			ready = "AccessKey 已配置"
		}
		fmt.Fprintf(stdout, "  %s  %s  [%s]\n", a.ID, a.Name, ready)
	}
	fmt.Fprintf(stdout, "白B.AI 账号 (%d):\n", len(cfg.BaiAccounts))
	for _, a := range cfg.BaiAccounts {
		ready := "未配置"
		if strings.TrimSpace(a.ApiKey) != "" {
			ready = "API Key 已配置"
		}
		fmt.Fprintf(stdout, "  %s  %s  [%s]\n", a.ID, a.Name, ready)
	}
	return 0
}

func cmdAccountsAdd(args []string, stdin io.Reader, stdout, stderr io.Writer, jsonOut bool) int {
	fs := flag.NewFlagSet("accounts add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	typ := fs.String("type", "", "账号类型: opencode|deepseek|qwen|galaxy|bai")
	name := fs.String("name", "", "账号名称")
	goKey := fs.String("go-api-key", "", "OpenCode Go API Key")
	wsID := fs.String("workspace-id", "", "OpenCode Workspace ID（可选）")
	cookie := fs.String("auth-cookie", "", "OpenCode Auth Cookie（可选）")
	apiKey := fs.String("api-key", "", "DeepSeek / Qwen API Key")
	ptok := fs.String("platform-token", "", "DeepSeek 平台 Token（可选）")
	qwenCookie := fs.String("console-cookie", "", "Qwen 控制台 Cookie（可选，配额窗口需要）")
	qwenRegion := fs.String("region", "", "Qwen 区域: cn-beijing（默认）|ap-southeast-1")
	galaxyAK := fs.String("access-key", "", "智星云 AccessKey")
	galaxySK := fs.String("secret-key", "", "智星云 SecretKey")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "用法: llm-api-check accounts add --type opencode|deepseek|qwen|galaxy|bai --name 名称 [凭据 flags]")
		return 2
	}
	if *typ != "opencode" && *typ != "deepseek" && *typ != "qwen" && *typ != "galaxy" && *typ != "bai" {
		fmt.Fprintln(stderr, "错误: --type 必须是 opencode、deepseek、qwen、galaxy 或 bai")
		fmt.Fprintln(stderr, "用法: llm-api-check accounts add --type opencode|deepseek|qwen|galaxy|bai --name 名称 [凭据 flags]")
		return 2
	}
	if strings.TrimSpace(*name) == "" {
		if !isTTY(stdin) {
			fmt.Fprintln(stderr, "错误: 缺少 --name，非交互终端无法提示")
			return 2
		}
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
			writeJSON(stdout, map[string]any{"account": publicAccount(acc)})
		} else {
			fmt.Fprintf(stdout, "已添加 OpenCode 账号「%s」(id=%s)\n", acc.Name, acc.ID)
		}
		return 0
	}
	// deepseek
	if *typ == "deepseek" {
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
			writeJSON(stdout, map[string]any{"deepseek_account": publicDeepSeekAccount(acc)})
		} else {
			fmt.Fprintf(stdout, "已添加 DeepSeek 账号「%s」(id=%s)\n", acc.Name, acc.ID)
		}
		return 0
	}
	// galaxy
	if *typ == "galaxy" {
		ak, err := resolveSecret(*galaxyAK, "access-key", envGalaxyAK, "智星云 AccessKey: ", true, stdin, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 2
		}
		sk, err := resolveSecret(*galaxySK, "secret-key", envGalaxySK, "智星云 SecretKey: ", true, stdin, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 2
		}
		acc := models.GalaxyAccount{
			ID:        config.NewID(),
			Name:      strings.TrimSpace(*name),
			AccessKey: ak,
			SecretKey: sk,
		}
		cfg.SaveGalaxyAccount(acc)
		if err := cfg.Save(config.DefaultPath()); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		if jsonOut {
			writeJSON(stdout, map[string]any{"galaxy_account": publicGalaxyAccount(acc)})
		} else {
			fmt.Fprintf(stdout, "已添加智星云账号「%s」(id=%s)\n", acc.Name, acc.ID)
		}
		return 0
	}
	// bai（白B.AI）：只用共享的 --api-key；放在 qwen 之前，qwen 是 fallthrough
	if *typ == "bai" {
		key, err := resolveSecret(*apiKey, "api-key", envBaiAPIKey, "BAI API Key: ", true, stdin, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 2
		}
		acc := models.BaiAccount{
			ID:     config.NewID(),
			Name:   strings.TrimSpace(*name),
			ApiKey: key,
		}
		cfg.SaveBaiAccount(acc)
		if err := cfg.Save(config.DefaultPath()); err != nil {
			fmt.Fprintf(stderr, "错误: %v\n", err)
			return 1
		}
		if jsonOut {
			writeJSON(stdout, map[string]any{"bai_account": publicBaiAccount(acc)})
		} else {
			fmt.Fprintf(stdout, "已添加白B.AI 账号「%s」(id=%s)\n", acc.Name, acc.ID)
		}
		return 0
	}
	// qwen
	key, err := resolveSecret(*apiKey, "api-key", envQwenAPIKey, "Qwen API Key（sk-sp- 开头）: ", true, stdin, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 2
	}
	ck, err := resolveSecret(*qwenCookie, "console-cookie", envQwenCookie, "控制台 Cookie（可选，回车跳过）: ", false, stdin, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 2
	}
	region, err := resolveSecret(*qwenRegion, "region", envQwenRegion, "区域（可选，回车取默认 cn-beijing）: ", false, stdin, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 2
	}
	region, err = models.NormalizeQwenRegion(region)
	if err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 2
	}
	acc := models.QwenAccount{
		ID:            config.NewID(),
		Name:          strings.TrimSpace(*name),
		ApiKey:        key,
		ConsoleCookie: ck,
		Region:        region,
	}
	cfg.SaveQwenAccount(acc)
	if err := cfg.Save(config.DefaultPath()); err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	if jsonOut {
		writeJSON(stdout, map[string]any{"qwen_account": publicQwenAccount(acc)})
	} else {
		fmt.Fprintf(stdout, "已添加 Qwen 账号「%s」(id=%s)\n", acc.Name, acc.ID)
		if !acc.HasCookie() {
			fmt.Fprintln(stdout, "提示: 未配控制台 Cookie，只能看套餐模型清单；配额窗口需重跑并传 --console-cookie")
		}
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
	var keptQwen []models.QwenAccount
	for _, a := range cfg.QwenAccounts {
		if match(a.ID, a.Name) {
			removed++
			continue
		}
		keptQwen = append(keptQwen, a)
	}
	cfg.QwenAccounts = keptQwen
	var keptGalaxy []models.GalaxyAccount
	for _, a := range cfg.GalaxyAccounts {
		if match(a.ID, a.Name) {
			removed++
			continue
		}
		keptGalaxy = append(keptGalaxy, a)
	}
	cfg.GalaxyAccounts = keptGalaxy
	var keptBai []models.BaiAccount
	for _, a := range cfg.BaiAccounts {
		if match(a.ID, a.Name) {
			removed++
			continue
		}
		keptBai = append(keptBai, a)
	}
	cfg.BaiAccounts = keptBai
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
	for i := range cfg.QwenAccounts {
		a := &cfg.QwenAccounts[i]
		if match(a.ID, a.Name) {
			a.Name = strings.TrimSpace(*newName)
			renamed++
		}
	}
	for i := range cfg.GalaxyAccounts {
		a := &cfg.GalaxyAccounts[i]
		if match(a.ID, a.Name) {
			a.Name = strings.TrimSpace(*newName)
			renamed++
		}
	}
	for i := range cfg.BaiAccounts {
		a := &cfg.BaiAccounts[i]
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
		// 可选字段在输入流提前关闭（EOF）时跳过，不视为错误
		if !required {
			return "", nil
		}
		return "", err
	}
	if !required && strings.TrimSpace(v) == "" {
		return "", nil
	}
	return strings.TrimSpace(v), nil
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

// warnSecurity SecurityWarning 非空时输出到 stderr（TTY 下黄色；noColor 时无色）
func warnSecurity(stderr io.Writer, noColor bool) {
	if config.SecurityWarning == "" {
		return
	}
	c := colorizer(noColor)
	msg := "警告: " + config.SecurityWarning
	if !noColor && os.Getenv("NO_COLOR") == "" {
		if f, ok := stderr.(*os.File); ok {
			if isTTY(f) {
				msg = c.Yellow(msg)
			}
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

// ── JSON 凭据掩码（momus P1-3）：明文凭据不落到 stdout（终端日志/CI 日志/录屏泄漏向量）──
// maskSecret 保留首尾 4 字符，中间掩码；短于 9 字符全掩码；空串原样。
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 8 {
		return "****"
	}
	return string(r[:4]) + "****" + string(r[len(r)-4:])
}

func publicDeepSeekAccount(a models.DeepSeekAccount) map[string]any {
	return map[string]any{
		"id":            a.ID,
		"name":          a.Name,
		"apiKey":        maskSecret(a.ApiKey),
		"platformToken": maskSecret(a.PlatformToken),
	}
}

func publicAccount(a models.Account) map[string]any {
	return map[string]any{
		"id":          a.ID,
		"name":        a.Name,
		"goApiKey":    maskSecret(a.GoApiKey),
		"workspaceId": maskSecret(a.WorkspaceId),
		"authCookie":  maskSecret(a.AuthCookie),
	}
}

func publicQwenAccount(a models.QwenAccount) map[string]any {
	return map[string]any{
		"id":            a.ID,
		"name":          a.Name,
		"apiKey":        maskSecret(a.ApiKey),
		"consoleCookie": maskSecret(a.ConsoleCookie),
		"region":        a.QwenRegion(),
	}
}

// publicGalaxyAccount AccessKey 也当凭据对待（可签名发起计费请求），一并掩码
func publicGalaxyAccount(a models.GalaxyAccount) map[string]any {
	return map[string]any{
		"id":        a.ID,
		"name":      a.Name,
		"accessKey": maskSecret(a.AccessKey),
		"secretKey": maskSecret(a.SecretKey),
	}
}

func publicDeepSeekResults(rs []app.DeepSeekResult) []map[string]any {
	out := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, publicDeepSeekResult(r))
	}
	return out
}

func publicDeepSeekResult(r app.DeepSeekResult) map[string]any {
	m := map[string]any{
		"account": publicDeepSeekAccount(r.Account),
	}
	if r.Balance != nil {
		m["balance"] = r.Balance
	}
	if r.Cost != nil {
		m["cost"] = r.Cost
	}
	if r.Error != "" {
		m["error"] = r.Error
	}
	return m
}

func publicAccountResults(rs []app.AccountResult) []map[string]any {
	out := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, publicAccountResult(r))
	}
	return out
}

func publicAccountResult(r app.AccountResult) map[string]any {
	m := map[string]any{
		"account": publicAccount(r.Account),
	}
	if r.GoUsage != nil {
		m["go_usage"] = r.GoUsage
	}
	if r.ZenBilling != nil {
		m["zen_billing"] = r.ZenBilling
	}
	if r.Error != "" {
		m["error"] = r.Error
	}
	return m
}

func publicQwenResults(rs []app.QwenResult) []map[string]any {
	out := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, publicQwenResult(r))
	}
	return out
}

func publicGalaxyResults(rs []app.GalaxyResult) []map[string]any {
	out := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, publicGalaxyResult(r))
	}
	return out
}

// publicGalaxyResult 实例数据已是白名单结构（口令在解析层就被丢弃），
// 这里直接透传不再二次包装字段集，保持 JSON key 集合稳定。
func publicGalaxyResult(r app.GalaxyResult) map[string]any {
	m := map[string]any{
		"account": publicGalaxyAccount(r.Account),
	}
	if r.Balance != nil {
		m["balance"] = r.Balance
	}
	if r.Status != nil {
		m["status"] = r.Status
	}
	if len(r.Instances) > 0 {
		m["instances"] = r.Instances
	}
	if r.Cost != nil {
		m["cost"] = r.Cost
	}
	if r.Error != "" {
		m["error"] = r.Error
	}
	return m
}

func publicQwenResult(r app.QwenResult) map[string]any {
	m := map[string]any{
		"account": publicQwenAccount(r.Account),
	}
	if r.Plan != nil {
		m["plan"] = r.Plan
	}
	if r.Usage != nil {
		m["usage"] = r.Usage
	}
	if r.Stats != nil {
		m["stats"] = r.Stats
	}
	if r.Error != "" {
		m["error"] = r.Error
	}
	return m
}

// joinText 合并两段错误文本（main 层轻量版，与 app.joinErrors 同语义）。
// 按行去重：qwen --stats 时配额窗口与用量分析走同一个 CLI 会话，会话过期会产生
// 逐字相同的错误（上层还可能已拼上 Plan 错误），不去重就在详情页里印两遍。
func joinText(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	lines := strings.Split(a, "\n")
	seen := make(map[string]bool, len(lines))
	for _, l := range lines {
		seen[l] = true
	}
	for _, l := range strings.Split(b, "\n") {
		if !seen[l] {
			lines = append(lines, l)
			seen[l] = true
		}
	}
	return strings.Join(lines, "\n")
}

func publicDeepSeekAccounts(as []models.DeepSeekAccount) []map[string]any {
	out := make([]map[string]any, 0, len(as))
	for _, a := range as {
		out = append(out, publicDeepSeekAccount(a))
	}
	return out
}

func publicAccounts(as []models.Account) []map[string]any {
	out := make([]map[string]any, 0, len(as))
	for _, a := range as {
		out = append(out, publicAccount(a))
	}
	return out
}

func publicQwenAccounts(as []models.QwenAccount) []map[string]any {
	out := make([]map[string]any, 0, len(as))
	for _, a := range as {
		out = append(out, publicQwenAccount(a))
	}
	return out
}

func publicGalaxyAccounts(as []models.GalaxyAccount) []map[string]any {
	out := make([]map[string]any, 0, len(as))
	for _, a := range as {
		out = append(out, publicGalaxyAccount(a))
	}
	return out
}

// publicBaiAccount apiKey 当凭据掩码（与 DeepSeek/Qwen 同口径）
func publicBaiAccount(a models.BaiAccount) map[string]any {
	return map[string]any{
		"id":     a.ID,
		"name":   a.Name,
		"apiKey": maskSecret(a.ApiKey),
	}
}

func publicBaiResults(rs []app.BaiResult) []map[string]any {
	out := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, publicBaiResult(r))
	}
	return out
}

func publicBaiResult(r app.BaiResult) map[string]any {
	m := map[string]any{
		"account": publicBaiAccount(r.Account),
	}
	if r.Plan != nil {
		m["plan"] = r.Plan
	}
	if r.Error != "" {
		m["error"] = r.Error
	}
	return m
}

func publicBaiAccounts(as []models.BaiAccount) []map[string]any {
	out := make([]map[string]any, 0, len(as))
	for _, a := range as {
		out = append(out, publicBaiAccount(a))
	}
	return out
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
