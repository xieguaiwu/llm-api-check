package repo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// Bailian CLI 配额通道。
//
// 认证模式 Console：`bailian auth login --console` 一次浏览器 OAuth 后，
// `bailian usage token-plan --console-region <region> --console-site <site> --output json`
// 直接返回配额窗口 JSON，无需人工抓取控制台 Cookie。
// CLI 不可用/未登录时调用方降级到 Cookie 路径（CodexBar 同策略：CLI 优先、Cookie 兜底）。
const (
	// envQwenCLIBin 覆盖 Bailian CLI 可执行文件路径
	envQwenCLIBin = "LLM_API_CHECK_BL_BIN"
	// envQwenCLIOff 非空且为 off/0/false/no 时禁用 CLI 通道
	envQwenCLIOff = "LLM_API_CHECK_QWEN_CLI"
)

// defaultQwenCLIPath 本机 npm 独立安装位（--prefix 安装，避免与用户自研 bl 同名冲突）。
var defaultQwenCLIPath = filepath.Join(os.Getenv("HOME"), ".local", "share", "bailian-cli", "bin", "bailian")

// QwenCLI 官方 Bailian CLI 的配额通道。
type QwenCLI struct {
	// BinPath 可执行文件路径（空串表示未探测到）
	BinPath string
	// Timeout 单次调用超时（≤0 → 20s）
	Timeout time.Duration
	// Command 命令构造注入点（测试用）；nil → exec.CommandContext
	Command func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// DetectQwenCLI 探测可用的官方 Bailian CLI。
//
// 探测顺序：env LLM_API_CHECK_BL_BIN（显式指定）→
// 本机独立安装位 ~/.local/share/bailian-cli/bin/bailian →
// PATH 中的 "bailian"（刻意不用 "bl"，避免误调用户自研同名翻译 CLI）。
// env LLM_API_CHECK_QWEN_CLI=off|0|false|no 时返回禁用错误。
func DetectQwenCLI() (*QwenCLI, error) {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv(envQwenCLIOff))); v == "off" || v == "0" || v == "false" || v == "no" {
		return nil, errors.New("Qwen CLI 通道已被 " + envQwenCLIOff + " 禁用")
	}
	// 显式 env 指定：存在即用，不存在即报错（不静默 fallback，暴露拼写/路径错误）
	if p := strings.TrimSpace(os.Getenv(envQwenCLIBin)); p != "" {
		if st, err := os.Stat(p); err != nil || st.IsDir() || st.Mode()&0o111 == 0 {
			return nil, fmt.Errorf(envQwenCLIBin+" 指定的 Bailian CLI 不可执行: %s", p)
		}
		return &QwenCLI{BinPath: p}, nil
	}
	candidates := make([]string, 0, 2)
	if defaultQwenCLIPath != "" {
		candidates = append(candidates, defaultQwenCLIPath)
	}
	if p, err := exec.LookPath("bailian"); err == nil {
		candidates = append(candidates, p)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return &QwenCLI{BinPath: p}, nil
		}
	}
	return nil, errors.New("未找到 Bailian CLI（bailian-cli，npm install -g --prefix ~/.local/share/bailian-cli bailian-cli）")
}

func (c *QwenCLI) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	if c.Command != nil {
		return c.Command(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...)
}

func (c *QwenCLI) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 20 * time.Second
}

// Usage 调用 `bailian usage token-plan` 拉配额窗口。
// 返回的错误可直接展示（含 CLI 修复提示）。
func (c *QwenCLI) Usage(acc models.QwenAccount) (models.QwenUsage, error) {
	region, err := models.NormalizeQwenRegion(acc.Region)
	if err != nil {
		return models.QwenUsage{}, err
	}
	site := qwenCLISite(region)
	raw, err := c.runJSON("usage", "token-plan",
		"--console-region", region, "--console-site", site, "--output", "json")
	if err != nil {
		return models.QwenUsage{}, err
	}
	return parsers.ParseQwenUsage(raw, time.Now())
}

// Summary 调用 `bailian usage summary` 拉用量分析（token 统计 + 免费额度）。
func (c *QwenCLI) Summary(acc models.QwenAccount) (models.QwenSummary, error) {
	region, err := models.NormalizeQwenRegion(acc.Region)
	if err != nil {
		return models.QwenSummary{}, err
	}
	raw, err := c.runJSON("usage", "summary",
		"--console-region", region, "--console-site", qwenCLISite(region), "--output", "json")
	if err != nil {
		return models.QwenSummary{}, err
	}
	var out models.QwenSummary
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return models.QwenSummary{}, fmt.Errorf("Qwen 用量分析 JSON 解析失败: %w", err)
	}
	return out, nil
}

func qwenCLISite(region string) string {
	if region == models.RegionQwenIntl {
		return "international"
	}
	return "domestic"
}

// runJSON 执行 Bailian CLI 子命令并返回清洗后的 stdout JSON。
// 错误处理统一：BinPath 校验 → 超时 → stdout/stderr 分离（过滤 Node 噪音）
// → 错误信封识别（exit 非零时信封在 stderr，实测 exit 3）。
func (c *QwenCLI) runJSON(args ...string) (string, error) {
	if c.BinPath == "" {
		return "", errors.New("未找到 Bailian CLI（bailian-cli）")
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout())
	defer cancel()
	cmd := c.command(ctx, c.BinPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// stderr 尾部（截断防膨胀；过滤 Node 的 UNDICI/experimental 警告噪音）
		tail := qwenCLIStderrTail(stderr.String())
		// exit 非零时 CLI 可能把 JSON 错误信封打到 stderr（实测 exit 3 即如此）
		if envErr := qwenCLIErrorEnvelope(tail); envErr != "" {
			return "", fmt.Errorf("Bailian CLI 不可用: %s（运行 bailian auth login --console 登录后重试）", envErr)
		}
		if tail == "" {
			return "", fmt.Errorf("Bailian CLI 调用失败: %v", err)
		}
		return "", fmt.Errorf("Bailian CLI 调用失败: %v（%s）", err, tail)
	}
	raw := strings.TrimSpace(stdout.String())
	if envErr := qwenCLIErrorEnvelope(raw); envErr != "" {
		return "", fmt.Errorf("Bailian CLI 不可用: %s（运行 bailian auth login --console 登录后重试）", envErr)
	}
	return raw, nil
}

// qwenCLIStderrTail 清洗 Bailian CLI 的 stderr：过滤 Node 运行时噪音
// （UNDICI 代理实验警告、node 进程头注释），保留真实错误，截断到 300 字符。
func qwenCLIStderrTail(raw string) string {
	var kept []string
	for _, ln := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.Contains(t, "UNDICI") || strings.HasPrefix(t, "(node:") ||
			(strings.HasPrefix(t, "(Use `node") && strings.Contains(t, "trace-warnings")) {
			continue
		}
		kept = append(kept, t)
	}
	tail := strings.Join(kept, "\n")
	if len(tail) > 300 {
		tail = "…" + tail[len(tail)-300:]
	}
	return strings.TrimSpace(tail)
}

// qwenCLIErrorEnvelope 识别 Bailian CLI 的 JSON 错误信封 {"error":{code,message,hint}}。
// 命中返回可展示的消息（含 hint），未命中返回空串。
func qwenCLIErrorEnvelope(raw string) string {
	var env struct {
		Error struct {
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(raw), &env) == nil && strings.TrimSpace(env.Error.Message) != "" {
		if strings.TrimSpace(env.Error.Hint) != "" {
			return strings.TrimSpace(env.Error.Message) + "（" + strings.TrimSpace(env.Error.Hint) + "）"
		}
		return strings.TrimSpace(env.Error.Message)
	}
	return ""
}
