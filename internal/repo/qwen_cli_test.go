package repo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

// qwenUsageServer 返回只应答用量 RPC 的假控制台网关（单窗口 20%）。
func qwenUsageServer(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data/api.json" {
			t.Errorf("[%s] 意外路径: %s", tag, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		api := r.URL.Query().Get("api")
		switch api {
		case qwenAPIUsage:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"code":"200","data":{"success":true,"data":{"per5HourPercentage":0.2,"per5HourResetTime":1790000000000}}}`)
		case qwenAPISubscription:
			// 档位 best-effort：返回空对象即可（解析器找不到 specCode 返回空串无错）
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"code":"200","data":{"success":true,"data":{}}}`)
		default:
			t.Errorf("[%s] 意外 api = %q", tag, api)
			http.Error(w, "bad api", http.StatusBadRequest)
		}
	}))
}

// helperCLI 构造指向 TestQwenCLIHelperProcess 的 QwenCLI（不依赖真实文件系统）。
func helperCLI(t *testing.T, mode string) *QwenCLI {
	t.Helper()
	return &QwenCLI{
		BinPath: "bailian-test",
		Timeout: 5 * time.Second,
		Command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestQwenCLIHelperProcess", "--", name)
			cmd.Args = append(cmd.Args, args...)
			cmd.Env = append(os.Environ(),
				"GO_WANT_HELPER_PROCESS=1",
				"GO_HELPER_MODE="+mode)
			return cmd
		},
	}
}

// TestQwenCLIHelperProcess 是被 exec 的子进程入口（标准 TestHelperProcess 模式）。
// 它模拟官方 bailian CLI 的输出形状；"ok" 模式还校验 region/site 参数。
func TestQwenCLIHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	args = args[1:] // 去掉 "--"
	// args[0]=bin 名, args[1:]=usage token-plan --console-region … --console-site …
	join := strings.Join(args, " ")
	switch os.Getenv("GO_HELPER_MODE") {
	case "ok":
		if !strings.Contains(join, "--console-region") || !strings.Contains(join, "--console-site") {
			os.Stderr.WriteString("missing region/site flags: " + join + "\n")
			os.Exit(2)
		}
		if !strings.Contains(join, "--output json") {
			os.Stderr.WriteString("missing --output json: " + join + "\n")
			os.Exit(2)
		}
		os.Stdout.WriteString(`{"per5HourPercentage":0.5,"per5HourResetTime":1790000000000,"per1WeekPercentage":0.7,"per1WeekResetTime":1791000000000}`)
	case "intl-site":
		if !strings.Contains(join, "ap-southeast-1") || !strings.Contains(join, "international") {
			os.Stderr.WriteString("wrong region/site for intl: " + join + "\n")
			os.Exit(2)
		}
		os.Stdout.WriteString(`{"per1WeekPercentage":0.2,"per1WeekResetTime":1791000000000}`)
	case "notloggedin":
		os.Stdout.WriteString(`{"error":{"code":3,"message":"No console access token found.","hint":"Run bl auth login --console."}}`)
	case "expired":
		// 真实上游文案（2026-08-30 实测 exit 3 + stderr 信封）
		os.Stdout.WriteString("{\"error\":{\"code\":3,\"message\":\"Console session is not logged in or has expired.\",\"hint\":\"Run `bl auth login --console` to sign in or refresh your console session.\"}}")
	case "gateway-error":
		// 非会话类错误：不得误报成「重新登录就好」，但 `bl` 仍须改写
		os.Stdout.WriteString(`{"error":{"code":5,"message":"BailianGateway.Workspace.NotAuthorised","hint":"Run bl auth login --console."}}`)
	case "stdout-envelope-nonzero":
		// 改版形状：失败信封改到 stdout、仍 exit 非零（旧实现会退化成 exit status）
		os.Stdout.WriteString(`{"error":{"code":3,"message":"Console session is not logged in or has expired."}}`)
		os.Exit(1)
	case "stderr-envelope-zero":
		// 改版形状：失败信封在 stderr、exit 0（旧实现会拿到空 stdout 去解析）
		os.Stderr.WriteString("(node:1) [UNDICI-EHPA] Warning: experimental\n")
		os.Stderr.WriteString(`{"error":{"code":3,"message":"No console access token found."}}` + "\n")
	case "ok-with-stderr-noise":
		// 成功路径：stderr 有非信封警告不得误判为错误
		os.Stderr.WriteString("bailian: deprecated flag --foo\n")
		os.Stdout.WriteString(`{"per1WeekPercentage":0.3,"per1WeekResetTime":1791000000000}`)
	case "stderr-exit1":
		os.Stderr.WriteString("bailian: unknown command\n")
		os.Exit(1)
	case "stderr-with-warnings":
		os.Stderr.WriteString("(node:1361669) [UNDICI-EHPA] Warning: EnvHttpProxyAgent is experimental, expect them to change at any time.\n(Use `node --trace-warnings ...` to show where the warning was created)\n{\"error\":{\"code\":3,\"message\":\"No console access token found.\"}}\n")
		os.Exit(3)
	case "badjson":
		os.Stdout.WriteString("<html>gateway error</html>")
	case "summary":
		if !strings.Contains(join, "usage summary") {
			os.Stderr.WriteString("wrong subcommand: " + join + "\n")
			os.Exit(2)
		}
		os.Stdout.WriteString(`{"period":{"start":"2026-08-22","end":"2026-08-29","days":7},"freeTier":[{"model":"qwen3.8-flash","type":"Text","remaining":986768,"total":1000000,"remainingPercent":98.7,"expires":"2026-11-28"},{"model":"qwen3.8-max","type":"Text","remaining":1000000,"total":1000000,"remainingPercent":100,"expires":"2026-11-28"}],"usage":{"modelsCalled":2,"successfulCalls":14,"usages":[{"key":"input_token","value":5034,"unit":"tokens","label":"Input Tokens"},{"key":"output_token","value":9751,"unit":"tokens","label":"Output Tokens"},{"key":"total_token","value":14785,"unit":"tokens","label":"Total Tokens"}]}}`)
	}
	os.Exit(0)
}

func TestQwenCLIUsageOK(t *testing.T) {
	cli := helperCLI(t, "ok")
	u, err := cli.Usage(models.QwenAccount{Region: "cn-beijing"})
	if err != nil {
		t.Fatalf("Usage err: %v", err)
	}
	if u.FiveHour == nil || u.FiveHour.Percent != 50 {
		t.Errorf("FiveHour = %+v, want 50%%", u.FiveHour)
	}
	if u.Weekly == nil || u.Weekly.Percent != 70 {
		t.Errorf("Weekly = %+v, want 70%%", u.Weekly)
	}
	if u.FiveHour.ResetsAt == "" || u.Weekly.ResetsAt == "" {
		t.Errorf("重置时间不应为空: %+v", u)
	}
}

func TestQwenCLIUsageIntlSite(t *testing.T) {
	cli := helperCLI(t, "intl-site")
	u, err := cli.Usage(models.QwenAccount{Region: "ap-southeast-1"})
	if err != nil {
		t.Fatalf("Usage err: %v", err)
	}
	if u.FiveHour != nil {
		t.Errorf("国际站单窗口响应不应产生 5 小时窗口: %+v", u.FiveHour)
	}
	if u.Weekly == nil || u.Weekly.Percent != 20 {
		t.Errorf("Weekly = %+v, want 20%%", u.Weekly)
	}
}

func TestQwenCLIUsageNotLoggedIn(t *testing.T) {
	cli := helperCLI(t, "notloggedin")
	_, err := cli.Usage(models.QwenAccount{Region: "cn-beijing"})
	if err == nil {
		t.Fatal("未登录应报错")
	}
	if !strings.Contains(err.Error(), "auth login --console") {
		t.Errorf("错误应提示登录: %v", err)
	}
}

// 会话过期类错误：给可直接复制的真实 bin 路径，不透传上游的 `bl` 指令
// （本机 bl 是另一套同名翻译 CLI，照做必然失败）。
func TestQwenCLIUsageExpiredGivesRealBinPath(t *testing.T) {
	cli := helperCLI(t, "expired")
	_, err := cli.Usage(models.QwenAccount{Region: "cn-beijing"})
	if err == nil {
		t.Fatal("会话过期应报错")
	}
	msg := err.Error()
	if !strings.Contains(msg, "未登录或会话已过期") {
		t.Errorf("应报会话失效: %v", msg)
	}
	if !strings.Contains(msg, "bailian-test auth login --console") {
		t.Errorf("应含探测到的 bin 路径: %v", msg)
	}
	if strings.Contains(msg, "bl auth") {
		t.Errorf("不得出现裸 bl 指令: %v", msg)
	}
}

// 非会话类错误：保留上游原文（避免误导向），但 bin 名仍须改写。
func TestQwenCLIUsageOtherEnvelopeKeepsDetail(t *testing.T) {
	cli := helperCLI(t, "gateway-error")
	_, err := cli.Usage(models.QwenAccount{Region: "cn-beijing"})
	if err == nil {
		t.Fatal("网关错误应报错")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Bailian CLI 返回错误") || !strings.Contains(msg, "NotAuthorised") {
		t.Errorf("应保留上游错误原文: %v", msg)
	}
	if strings.Contains(msg, "未登录或会话已过期") {
		t.Errorf("非会话错误不应说会话失效: %v", msg)
	}
	if strings.Contains(msg, "bl auth") || !strings.Contains(msg, "bailian-test auth") {
		t.Errorf("hint 中的 bl 应改写为真实 bin: %v", msg)
	}
}

// 无 bin 路径时退化到 bailian（绝不退到 bl）。
func TestQwenCLIRewriteBinFallsBackToName(t *testing.T) {
	got := qwenCLIRewriteBin("Run `bl auth login --console` to sign in.", "")
	want := "Run `bailian auth login --console` to sign in."
	if got != want {
		t.Errorf("改写结果 = %q, want %q", got, want)
	}
	// 幂等且不误伤：已是 bailian / 单词内含 bl 都不动
	for _, s := range []string{
		"Run `bailian auth login --console`.",
		"enable blurring in model",
		"unable auth failed",
	} {
		if out := qwenCLIRewriteBin(s, "/usr/bin/bailian"); out != s {
			t.Errorf("不应改写 %q → %q", s, out)
		}
	}
	// 多个子命令都改写，且路径含特殊字符也不变样
	if out := qwenCLIRewriteBin("bl usage token-plan; bl auth logout", "/x/y$b/z"); out != "/x/y$b/z usage token-plan; /x/y$b/z auth logout" {
		t.Errorf("多命中改写结果不符: %q", out)
	}
}

func TestQwenCLISessionExpiredClassifies(t *testing.T) {
	hit := []string{
		"Console session is not logged in or has expired.",
		"No console access token found.",
		"BailianGateway.Login.NotLogined（Run `bl auth login --console`）",
		"BailianGateway.Login.NeedLogin",
		"token has expired",
	}
	for _, s := range hit {
		if !qwenCLISessionExpired(s) {
			t.Errorf("应归为会话失效: %q", s)
		}
	}
	// 权限/参数/网络类错误不得归为会话失效（否则让用户白跑一轮重新登录）
	miss := []string{
		"BailianGateway.Workspace.NotAuthorised",
		"You are not authorized to access this workspace",
		"page_size 参数超限!",
		"upstream timeout",
	}
	for _, s := range miss {
		if qwenCLISessionExpired(s) {
			t.Errorf("不应归为会话失效: %q", s)
		}
	}
}

// 信封在两条流上的四种组合都要能识别（防上游改版把失败信封捐走）。
func TestQwenCLIEnvelopeOnBothStreams(t *testing.T) {
	for _, mode := range []string{"stdout-envelope-nonzero", "stderr-envelope-zero"} {
		_, err := helperCLI(t, mode).Usage(models.QwenAccount{Region: "cn-beijing"})
		if err == nil {
			t.Fatalf("%s 应报错", mode)
		}
		if !strings.Contains(err.Error(), "未登录或会话已过期") {
			t.Errorf("%s 应归为会话失效文案: %v", mode, err)
		}
	}
	// 反向：成功输出 + stderr 有噪声 不得被判为错误
	u, err := helperCLI(t, "ok-with-stderr-noise").Usage(models.QwenAccount{Region: "cn-beijing"})
	if err != nil {
		t.Fatalf("不应误判为错误: %v", err)
	}
	if u.Weekly == nil || u.Weekly.Percent != 30 {
		t.Errorf("应正常解析窗口: %+v", u)
	}
}

// ③ 登录/安装命令必须含真实路径（默认独立 prefix 安装下裸 bailian 不在 PATH）。
func TestQwenRepoCLILoginAndInstallCmd(t *testing.T) {
	detected := (&QwenRepo{CLI: &QwenCLI{BinPath: "/opt/bailian"}}).CLILoginCmd()
	if detected != "/opt/bailian auth login --console" {
		t.Errorf("探测到 CLI 应用其路径: %q", detected)
	}
	notDetected := (&QwenRepo{}).CLILoginCmd()
	if !strings.HasSuffix(notDetected, " auth login --console") ||
		!strings.Contains(notDetected, "/.local/share/bailian-cli/bin/bailian") {
		t.Errorf("未探测到时应给默认安装位全路径: %q", notDetected)
	}
	if strings.HasPrefix(notDetected, "bailian ") {
		t.Errorf("不得退为裸名 bailian: %q", notDetected)
	}
	if got := (&QwenRepo{}).CLIInstallCmd(); !strings.Contains(got, "--prefix ~/.local/share/bailian-cli") {
		t.Errorf("安装命令应与探测路径一致: %q", got)
	}
}

func TestQwenCLIUsageStderrExit(t *testing.T) {
	cli := helperCLI(t, "stderr-exit1")
	_, err := cli.Usage(models.QwenAccount{Region: "cn-beijing"})
	if err == nil {
		t.Fatal("exit 非零应报错")
	}
	if !strings.Contains(err.Error(), "Bailian CLI 调用失败") || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("错误应含调用失败与 stderr: %v", err)
	}
}

func TestQwenCLIUsageBadJSON(t *testing.T) {
	cli := helperCLI(t, "badjson")
	_, err := cli.Usage(models.QwenAccount{Region: "cn-beijing"})
	if err == nil {
		t.Fatal("非 JSON 输出应报错")
	}
	if !strings.Contains(err.Error(), "Qwen 用量 JSON 解析失败") {
		t.Errorf("错误应指向用量解析: %v", err)
	}
}

func TestQwenCLIUsageEmptyBin(t *testing.T) {
	cli := &QwenCLI{BinPath: "", Timeout: time.Second}
	_, err := cli.Usage(models.QwenAccount{Region: "cn-beijing"})
	if err == nil || !strings.Contains(err.Error(), "未找到 Bailian CLI") {
		t.Errorf("空 BinPath 应报未找到: %v", err)
	}
}

func TestQwenCLISummaryOK(t *testing.T) {
	cli := helperCLI(t, "summary")
	s, err := cli.Summary(models.QwenAccount{Region: "cn-beijing"})
	if err != nil {
		t.Fatalf("Summary err: %v", err)
	}
	if s.Period.Days != 7 || s.Period.Start != "2026-08-22" || s.Period.End != "2026-08-29" {
		t.Errorf("周期不符: %+v", s.Period)
	}
	if s.Usage.ModelsCalled != 2 || s.Usage.SuccessfulCalls != 14 {
		t.Errorf("调用汇总不符: %+v", s.Usage)
	}
	if len(s.Usage.Usages) != 3 || s.Usage.Usages[0].Label != "Input Tokens" || s.Usage.Usages[2].Value != 14785 {
		t.Errorf("token 统计不符: %+v", s.Usage.Usages)
	}
	if len(s.FreeTier) != 2 || s.FreeTier[0].Model != "qwen3.8-flash" || s.FreeTier[0].RemainingPercent != 98.7 {
		t.Errorf("免费额度不符: %+v", s.FreeTier)
	}
}

func TestQwenCLISummaryNotLoggedIn(t *testing.T) {
	cli := helperCLI(t, "notloggedin")
	_, err := cli.Summary(models.QwenAccount{Region: "cn-beijing"})
	if err == nil || !strings.Contains(err.Error(), "auth login --console") {
		t.Errorf("未登录应提示登录: %v", err)
	}
}

// ── repo.Usage 通道优先级（CLI 优先、Cookie 兜底） ────────────

func TestQwenCLIUsageStderrWithWarnings(t *testing.T) {
	cli := helperCLI(t, "stderr-with-warnings")
	_, err := cli.Usage(models.QwenAccount{Region: "cn-beijing"})
	if err == nil {
		t.Fatal("应报错")
	}
	if strings.Contains(err.Error(), "UNDICI") || strings.Contains(err.Error(), "trace-warnings") {
		t.Errorf("错误不应含 Node 噪音: %v", err)
	}
	// 含 UNDICI 噪音的 stderr 信封仍要被识别并归到会话失效文案
	if !strings.Contains(err.Error(), "未登录或会话已过期") {
		t.Errorf("应识别出会话失效信封: %v", err)
	}
}

func TestQwenRepoUsageCLIOKWithoutCookie(t *testing.T) {
	repo := &QwenRepo{CLI: helperCLI(t, "ok")}
	u, err := repo.Usage(models.QwenAccount{ApiKey: "sk-sp-x", Region: "cn-beijing"})
	if err != nil {
		t.Fatalf("CLI 通道应无需 Cookie: %v", err)
	}
	if u.Weekly == nil || u.Weekly.Percent != 70 {
		t.Errorf("Weekly = %+v, want 70%%", u.Weekly)
	}
}

func TestQwenRepoUsageCLIBeatsCookie(t *testing.T) {
	srv := qwenUsageServer(t, "cli-beats")
	defer srv.Close()
	repo := &QwenRepo{
		Client:    srv.Client(),
		Endpoints: qwenTestEndpoints(srv.URL),
		CLI:       helperCLI(t, "ok"),
	}
	u, err := repo.Usage(models.QwenAccount{ApiKey: "sk-sp-x", ConsoleCookie: "sec_token=tok"})
	if err != nil {
		t.Fatalf("Usage err: %v", err)
	}
	if u.Weekly == nil || u.Weekly.Percent != 70 {
		t.Errorf("应优先 CLI 结果: %+v", u)
	}
}

func TestQwenRepoUsageCLIFailFallsBackToCookie(t *testing.T) {
	srv := qwenUsageServer(t, "fallback")
	defer srv.Close()
	repo := &QwenRepo{
		Client:          srv.Client(),
		Endpoints:       qwenTestEndpoints(srv.URL),
		UsageRetryDelay: time.Millisecond,
		CLI:             helperCLI(t, "stderr-exit1"),
	}
	u, err := repo.Usage(models.QwenAccount{ApiKey: "sk-sp-x", ConsoleCookie: "sec_token=tok"})
	if err != nil {
		t.Fatalf("CLI 失败应降级 Cookie: %v", err)
	}
	if u.FiveHour == nil || u.FiveHour.Percent != 20 {
		t.Errorf("Cookie 结果 FiveHour = %+v, want 20%%", u.FiveHour)
	}
}

func TestQwenRepoUsageCLIErrorNoCookie(t *testing.T) {
	repo := &QwenRepo{CLI: helperCLI(t, "notloggedin")}
	_, err := repo.Usage(models.QwenAccount{ApiKey: "sk-sp-x", Region: "cn-beijing"})
	if err == nil {
		t.Fatal("CLI 未登录且无 Cookie 应报错")
	}
	if !strings.Contains(err.Error(), "auth login --console") {
		t.Errorf("错误应提示 CLI 登录: %v", err)
	}
}

// ── 探测 ────────────────────────────────────────────────────

func TestDetectQwenCLIFromEnv(t *testing.T) {
	t.Setenv("LLM_API_CHECK_QWEN_CLI", "")
	t.Setenv("LLM_API_CHECK_BL_BIN", os.Args[0]) // 本测试二进制即真实可执行文件
	cli, err := DetectQwenCLI()
	if err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	if cli.BinPath != os.Args[0] {
		t.Errorf("BinPath = %q, want %q", cli.BinPath, os.Args[0])
	}
}

func TestDetectQwenCLIDisabledByEnv(t *testing.T) {
	t.Setenv("LLM_API_CHECK_QWEN_CLI", "off")
	t.Setenv("LLM_API_CHECK_BL_BIN", os.Args[0])
	if _, err := DetectQwenCLI(); err == nil {
		t.Fatal("LLM_API_CHECK_QWEN_CLI=off 应禁用 CLI 通道")
	}
}

func TestDetectQwenCLIEnvBinMissing(t *testing.T) {
	t.Setenv("LLM_API_CHECK_QWEN_CLI", "")
	t.Setenv("LLM_API_CHECK_BL_BIN", "/nonexistent/bailian-"+time.Now().Format("150405"))
	if cli, err := DetectQwenCLI(); err == nil {
		t.Fatalf("不存在的 bin 不应命中: %+v", cli)
	}
}
