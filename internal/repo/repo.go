// Package repo 数据仓库：DeepSeek（余额 + 消费明细）与 OpenCode（Go usage + Zen billing）。
// 逐行对照 Android 版 com.xieguiawu.apicheckers.data.Repositories.kt + ApiClient.kt 移植。
package repo

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// UA 移动端 UA，Zen billing 页面按浏览器解析（与 Android ApiClient.UA 一致）
const UA = "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"

// browserUA 桌面浏览器 UA：阿里云控制台网关对移动端 UA 会降级处理（sec_token 不渲染）
const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// 所有网络超时 15s（全局约束，等价 OkHttp connect/read/write 15s）
const timeout = 15 * time.Second

// truncate200 错误响应体截断 200 字符（rune 安全，防切断中文多字节，momus P2-2）
func truncate200(s string) string {
	r := []rune(s)
	if len(r) > 200 {
		return string(r[:200])
	}
	return s
}

// doGet 执行 GET 请求：
//
//	401/403 → 中文认证错误（authErrMsg，按凭据区分）；
//	其他非 2xx → "HTTP {code}: {body前200字符}"；
//	网络异常 → 中文包装「网络请求失败: {err}」。
//
// 对应 Android Repositories.get。
func doGet(client *http.Client, url string, headers map[string]string, authErrMsg string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", errors.New(authErrMsg)
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate200(string(body)))
	default:
		return string(body), nil
	}
}

// defaultClient 15s 超时 client（对应 ApiClient.client）
func defaultClient() *http.Client {
	return &http.Client{Timeout: timeout}
}

// ── DeepSeek 仓库 ──────────────────────────────────────────────

// DeepSeekRepo DeepSeek 数据仓库：余额（官方 API）+ 消费明细（platform 页面 API）。
// BaseBalanceURL/BaseCostURL/Client 测试可注入（httptest）。
type DeepSeekRepo struct {
	BaseBalanceURL string
	BaseCostURL    string
	Client         *http.Client
	// Now 时钟注入（测试固定日期保证确定性）；nil 时用 time.Now
	Now func() time.Time
}

// NewDeepSeekRepo 默认端点 + 15s 超时 client
func NewDeepSeekRepo() *DeepSeekRepo {
	return &DeepSeekRepo{
		BaseBalanceURL: "https://api.deepseek.com",
		BaseCostURL:    "https://platform.deepseek.com",
		Client:         defaultClient(),
	}
}

func (r *DeepSeekRepo) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return defaultClient()
}

func (r *DeepSeekRepo) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Balance 查询余额（对应 balance）。
// 401/403 → 「API Key 无效或已过期」。
func (r *DeepSeekRepo) Balance(apiKey string) (models.DeepSeekBalance, error) {
	body, err := doGet(r.client(), r.BaseBalanceURL+"/user/balance",
		map[string]string{"Authorization": "Bearer " + apiKey},
		"API Key 无效或已过期")
	if err != nil {
		return models.DeepSeekBalance{}, err
	}
	return parsers.ParseDeepSeekBalance(body)
}

// Cost 拉本月 + 上月两个月消费，汇总算 today/7d/30d（对应 cost）。
// 跨年由 time.AddDate(0,-1,0) 自动处理。
// 两个月都无数据（含非认证错误）→ 显式失败，不返回误导性的零数据。
func (r *DeepSeekRepo) Cost(platformToken string) (models.DeepSeekCost, error) {
	now := r.now()
	// 上月用 day=1 构造，避免 AddDate 的日溢出归一化（3/31 → 3/03 等）导致请求错月份（momus P1-1）
	prev := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	months := []time.Time{now, prev}
	dayMap := map[string]float64{}
	tokenInvalid := false
	var lastError string
	for _, d := range months {
		url := fmt.Sprintf("%s/api/v0/usage/cost?month=%d&year=%d", r.BaseCostURL, int(d.Month()), d.Year())
		body, err := doGet(r.client(), url, map[string]string{
			"Authorization": "Bearer " + platformToken,
			"Accept":        "application/json",
			"x-app-version": "1.0.0",
			"Referer":       "https://platform.deepseek.com/usage",
			"User-Agent":    UA,
		}, "平台 Token 已失效，请更新平台 Token")
		if err != nil {
			// HTTP 401/403 的认证错误含「失效」→ 标记 token 失效
			if strings.Contains(err.Error(), "失效") {
				tokenInvalid = true
			}
			lastError = err.Error()
			continue
		}
		parsed, err := parsers.ParseDeepSeekCost(body, now)
		if err != nil {
			// code 40003 等业务错误也标记 token 失效
			if strings.Contains(err.Error(), "失效") {
				tokenInvalid = true
			}
			lastError = err.Error()
			continue
		}
		for _, day := range parsed.Days {
			dayMap[day.Date] = dayMap[day.Date] + day.Total
		}
	}
	// 两个月都无数据（含非认证错误）：显式失败，不返回误导性的零数据
	if len(dayMap) == 0 {
		switch {
		case tokenInvalid:
			return models.DeepSeekCost{}, errors.New("DeepSeek 平台登录已失效，请更新平台 Token")
		case lastError != "":
			return models.DeepSeekCost{}, fmt.Errorf("消费数据获取失败：%s", lastError)
		default:
			return models.DeepSeekCost{}, errors.New("消费数据为空")
		}
	}
	return parsers.AggregateCost(dayMap, now), nil
}

// ── OpenCode 仓库 ──────────────────────────────────────────────

// OpenCodeRepo OpenCode 数据仓库：Go usage（官方 API）+ Zen billing（页面解析）。
type OpenCodeRepo struct {
	BaseURL string
	Client  *http.Client
}

// NewOpenCodeRepo 默认端点 + 15s 超时 client
func NewOpenCodeRepo() *OpenCodeRepo {
	return &OpenCodeRepo{BaseURL: "https://opencode.ai", Client: defaultClient()}
}

func (r *OpenCodeRepo) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return defaultClient()
}

// GoUsage 查询 Go plan 三窗口用量（对应 goUsage）。
// 401/403 → 「Go API Key 无效或已过期」。
func (r *OpenCodeRepo) GoUsage(acc models.Account) (models.GoUsage, error) {
	body, err := doGet(r.client(), r.BaseURL+"/zen/go/v1/usage",
		map[string]string{"Authorization": "Bearer " + acc.GoApiKey},
		"Go API Key 无效或已过期")
	if err != nil {
		return models.GoUsage{}, err
	}
	return parsers.ParseGoUsage(body)
}

// ZenBilling 解析 Zen billing 页面（对应 zenBilling）。
// 未配置 workspace/cookie → 「未配置 Workspace/Cookie」；
// 401/403 → 「Cookie 已过期，请更新 Auth Cookie」。
func (r *OpenCodeRepo) ZenBilling(acc models.Account) (models.ZenBilling, error) {
	if !acc.HasZen() {
		return models.ZenBilling{}, errors.New("未配置 Workspace/Cookie")
	}
	body, err := doGet(r.client(), r.BaseURL+"/workspace/"+acc.WorkspaceId+"/billing",
		map[string]string{
			"Cookie":     "auth=" + acc.AuthCookie,
			"User-Agent": UA,
		},
		"Cookie 已过期，请更新 Auth Cookie")
	if err != nil {
		return models.ZenBilling{}, err
	}
	return parsers.ParseZenBilling(body)
}

// ── Qwen 仓库 ────────────────────────────────────────────────

// qwenAPI 控制台 RPC 的三个 zelda 接口名
const (
	qwenAPIUsage        = "zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/usage"
	qwenAPISubscription = "zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/subscription"
)

// QwenEndpoints 一套区域的端点与网关参数。
type QwenEndpoints struct {
	Gateway       string // token-plan.<region>.maas.aliyuncs.com（模型清单，API Key 认证）
	Dashboard     string // 订阅页面 HTML（拓 SEC_TOKEN）
	UserInfo      string // sec_token 兜底端点
	Quota         string // <host>/data/api.json（用量 RPC，Cookie 认证）
	Action        string // BroadScopeAspnGateway / IntlBroadScopeAspnGateway
	Region        string // RPC region 参数
	ConsoleSite   string // BAILIAN_ALIYUN / QWENCLOUD
	Domain        string // cornerstoneParam.domain
	Lang          string // xsp_lang
	CommodityCode string // 订阅接口的套餐商品码
	Origin        string // Origin 头
}

// QwenEndpointsFor 按区域返回端点。region 空串 → 中国大陆。
// 国际端点形状来自上游公开资料，本机无国际 Cookie 未做实带验证（见 CONTEXT 遗留问题）。
func QwenEndpointsFor(region string) (QwenEndpoints, error) {
	r, err := models.NormalizeQwenRegion(region)
	if err != nil {
		return QwenEndpoints{}, err
	}
	if r == models.RegionQwenIntl {
		return QwenEndpoints{
			Gateway:       "https://token-plan.ap-southeast-1.maas.aliyuncs.com",
			Dashboard:     "https://home.qwencloud.com/billing/subscription/token-plan-individual",
			UserInfo:      "https://home.qwencloud.com/tool/user/info.json",
			Quota:         "https://cs-data.qwencloud.com/data/api.json",
			Action:        "IntlBroadScopeAspnGateway",
			Region:        models.RegionQwenIntl,
			ConsoleSite:   "QWENCLOUD",
			Domain:        "home.qwencloud.com",
			Lang:          "en-US",
			CommodityCode: "sfm_tokenplansolo_public_intl",
			Origin:        "https://home.qwencloud.com",
		}, nil
	}
	return QwenEndpoints{
		Gateway:       "https://token-plan.cn-beijing.maas.aliyuncs.com",
		Dashboard:     "https://bailian.console.aliyun.com/cn-beijing?tab=plan#/efm/subscription/token-plan/personal",
		UserInfo:      "",
		Quota:         "https://bailian-cs.console.aliyun.com/data/api.json",
		Action:        "BroadScopeAspnGateway",
		Region:        models.RegionQwenCN,
		ConsoleSite:   "BAILIAN_ALIYUN",
		Domain:        "bailian.console.aliyun.com",
		Lang:          "zh-CN",
		CommodityCode: "sfm_tokenplansolo_public_cn",
		Origin:        "https://bailian.console.aliyun.com",
	}, nil
}

// QwenRepo Qwen Token Plan 仓库：模型清单（API Key）+ 配额窗口
// （Bailian CLI 优先，控制台 Cookie 兜底）。
type QwenRepo struct {
	Client *http.Client
	// CLI 官方 Bailian CLI 配额通道；nil = 禁用（测试默认禁用，保持 hermetic）
	CLI *QwenCLI
	// Now 时钟注入（重置时间换 RFC3339 用同一时区）
	Now func() time.Time
	// Endpoints 测试注入：非 nil 时接管全部区域解析
	Endpoints *QwenEndpoints
	// UsageAttempts 用量空信封重试次数（≤0 → 默认 3）
	UsageAttempts int
	// UsageRetryDelay 重试间隔（≤0 → 默认 400ms）
	UsageRetryDelay time.Duration
}

// NewQwenRepo 默认构造（15s 超时 client + 自动探测 Bailian CLI）
func NewQwenRepo() *QwenRepo {
	r := &QwenRepo{Client: defaultClient()}
	if c, err := DetectQwenCLI(); err == nil {
		r.CLI = c
	}
	return r
}

// CLIEnabled CLI 配额通道是否可用（app 层据此决定是否尝试拉配额）
func (r *QwenRepo) CLIEnabled() bool { return r.CLI != nil }

// CLILoginCmd 给用户看的登录命令（含完整可执行路径）。
// 探测到 CLI → 探测到的路径；未探测到 → 文档默认安装位（避开了裸 `bailian`
// 在默认独立 prefix 安装下不在 PATH、照做会 command not found 的问题）。
func (r *QwenRepo) CLILoginCmd() string {
	if r.CLI != nil {
		return r.CLI.loginBin() + " auth login --console"
	}
	return QwenCLIDefaultLoginCmd()
}

// CLIInstallCmd 未探测到 CLI 时应展示的安装命令。
func (r *QwenRepo) CLIInstallCmd() string { return QwenCLIInstallCmd }

func (r *QwenRepo) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return defaultClient()
}

func (r *QwenRepo) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *QwenRepo) endpoints(region string) (QwenEndpoints, error) {
	if r.Endpoints != nil {
		return *r.Endpoints, nil
	}
	return QwenEndpointsFor(region)
}

// Plan 拉取套餐可用模型清单（API Key 认证）。
// 401/403 → 区域绑定提示（实测：同一把 sk-sp- key 打错区域同样回 invalid_api_key）。
func (r *QwenRepo) Plan(acc models.QwenAccount) (models.QwenPlan, error) {
	ep, err := r.endpoints(acc.Region)
	if err != nil {
		return models.QwenPlan{}, err
	}
	if strings.TrimSpace(acc.ApiKey) == "" {
		return models.QwenPlan{}, errors.New("未配置 API Key")
	}
	body, err := doGet(r.client(), ep.Gateway+"/compatible-mode/v1/models",
		map[string]string{"Authorization": "Bearer " + acc.ApiKey, "Accept": "application/json"},
		"Qwen API Key 无效或已过期（订阅密钥与区域绑定，请核对区域设置）")
	if err != nil {
		return models.QwenPlan{}, err
	}
	ids, err := parsers.ParseQwenModels(body)
	if err != nil {
		return models.QwenPlan{}, err
	}
	return models.QwenPlan{Models: ids}, nil
}

// Usage 拉配额窗口（5 小时 / 7 天）与套餐档位。
// 通道优先级：Bailian CLI（若启用）→ 控制台 Cookie（若配置）→ 显式错误。
func (r *QwenRepo) Usage(acc models.QwenAccount) (models.QwenUsage, error) {
	var cliErr error
	if r.CLI != nil {
		u, err := r.CLI.Usage(acc)
		if err == nil {
			return u, nil
		}
		cliErr = err
	}
	if !acc.HasCookie() {
		if cliErr != nil {
			return models.QwenUsage{}, cliErr
		}
		return models.QwenUsage{}, errors.New("未配置控制台 Cookie")
	}
	ep, err := r.endpoints(acc.Region)
	if err != nil {
		return models.QwenUsage{}, joinErrors(cliErr, err)
	}
	cookie := normalizeCookieHeader(acc.ConsoleCookie)
	secToken := r.resolveSECToken(ep, cookie)

	usage, err := r.fetchUsage(ep, cookie, secToken)
	if err != nil {
		return models.QwenUsage{}, joinErrors(cliErr, err)
	}
	// 套餐档位 best-effort：登录失效向上抛出，其他失败只记空档位
	code, subErr := r.fetchSubscription(ep, cookie, secToken)
	if subErr != nil && strings.Contains(subErr.Error(), "Cookie") {
		return models.QwenUsage{}, joinErrors(cliErr, subErr)
	}
	usage.PlanCode = code
	return usage, nil
}

// fetchUsage 拉窗口数据。网关偶发返回「200 Success 但无窗口字段」，
// 因此重试（上游实现同策略），重试后仍空则抛出「暂不可用」。
func (r *QwenRepo) fetchUsage(ep QwenEndpoints, cookie, secToken string) (models.QwenUsage, error) {
	attempts := r.UsageAttempts
	if attempts <= 0 {
		attempts = 3
	}
	delay := r.UsageRetryDelay
	if delay <= 0 {
		delay = 400 * time.Millisecond
	}
	now := r.now()
	var lastErr error = errors.New("Qwen 用量数据暂不可用")
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(delay)
		}
		body, err := r.rpc(ep, cookie, secToken, qwenAPIUsage, nil)
		if err != nil {
			lastErr = err
			continue
		}
		u, err := parsers.ParseQwenUsage(body, now)
		if err == nil {
			return u, nil
		}
		lastErr = err
		// 认证类错误不重试（重试不会改变结果）
		if strings.Contains(err.Error(), "Cookie") {
			return models.QwenUsage{}, err
		}
	}
	return models.QwenUsage{}, lastErr
}

// fetchSubscription 拉套餐档位（lite/standard/pro/max）。
func (r *QwenRepo) fetchSubscription(ep QwenEndpoints, cookie, secToken string) (string, error) {
	body, err := r.rpc(ep, cookie, secToken, qwenAPISubscription,
		map[string]string{"commodityCode": ep.CommodityCode})
	if err != nil {
		return "", err
	}
	return parsers.ParseQwenSubscription(body)
}

// rpc 调用控制台网关：POST <quota>?action=…&product=sfm_bailian&api=…&_v=undefined
// 表单体（application/x-www-form-urlencoded）携带 product/action/region/language/params/sec_token。
// 网关特点：登录失效仍回 HTTP 200，错误在信封里（errorCode=BailianGateway.Login.NotLogined），
// 因此非 2xx 之外的判错交给解析器。
func (r *QwenRepo) rpc(ep QwenEndpoints, cookie, secToken, api string, dataParams map[string]string) (string, error) {
	params, err := qwenParamsJSON(ep, api, dataParams, cookie)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("action", ep.Action)
	q.Set("product", "sfm_bailian")
	q.Set("api", api)
	q.Set("_v", "undefined")
	form := url.Values{}
	form.Set("product", "sfm_bailian")
	form.Set("action", ep.Action)
	form.Set("region", ep.Region)
	form.Set("language", ep.Lang)
	form.Set("params", params)
	if secToken != "" {
		form.Set("sec_token", secToken)
	}
	req, err := http.NewRequest(http.MethodPost, ep.Quota+"?"+q.Encode(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Origin", ep.Origin)
	req.Header.Set("Referer", ep.Dashboard)
	if csrf := cookieValue(cookie, "login_aliyunid_csrf"); csrf != "" {
		req.Header.Set("x-xsrf-token", csrf)
		req.Header.Set("x-csrf-token", csrf)
	} else if csrf := cookieValue(cookie, "csrf"); csrf != "" {
		req.Header.Set("x-xsrf-token", csrf)
		req.Header.Set("x-csrf-token", csrf)
	}
	resp, err := r.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", errors.New("控制台 Cookie 已过期或无效，请更新控制台 Cookie")
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate200(string(body)))
	default:
		return string(body), nil
	}
}

// qwenParamsJSON 组装 params 字段 JSON。
//
// 关键约束：cornerstoneParam 不能硬编码 switchAgent——网关会把它绑定到某个具体
// 账号的工作区，其他账号全部回 BailianGateway.Workspace.NotAuthorised； omission
// 使网关自行解析会话默认工作区。
func qwenParamsJSON(ep QwenEndpoints, api string, dataParams map[string]string, cookie string) (string, error) {
	cornerstone := map[string]any{
		"feTraceId":         qwenTraceID(),
		"feURL":             ep.Dashboard,
		"protocol":          "V2",
		"console":           "ONE_CONSOLE",
		"productCode":       "p_efm",
		"switchUserType":    3,
		"domain":            ep.Domain,
		"consoleSite":       ep.ConsoleSite,
		"userNickName":      "",
		"userPrincipalName": "",
		"xsp_lang":          ep.Lang,
	}
	if anon := cookieValue(cookie, "cna"); anon != "" {
		cornerstone["X-Anonymous-Id"] = anon
	}
	data := map[string]any{"cornerstoneParam": cornerstone}
	for k, v := range dataParams {
		data[k] = v
	}
	buf, err := json.Marshal(map[string]any{"Api": api, "V": "1.0", "Data": data})
	if err != nil {
		return "", fmt.Errorf("构造 Qwen 请求参数失败: %w", err)
	}
	return string(buf), nil
}

// qwenTraceID 生成 36 字符小写 UUIDv4（feTraceId，仅用于链路跟踪）
func qwenTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// resolveSECToken 解析 sec_token，三级降级：Cookie 内 sec_token →
// 控制台页面 HTML 抓取（需浏览器导航头，否则 shell 不渲染 SEC_TOKEN）→
// /tool/user/info.json。全失败返回空串（部分账号网关接受无 token 请求）。
func (r *QwenRepo) resolveSECToken(ep QwenEndpoints, cookie string) string {
	if t := cookieValue(cookie, "sec_token"); t != "" {
		return t
	}
	if ep.Dashboard != "" {
		if html, err := r.getPage(ep.Dashboard, cookie, ep.Origin, true); err == nil {
			if t := parsers.ExtractQwenSECToken(html); t != "" {
				return t
			}
		}
	}
	if ep.UserInfo != "" {
		if body, err := r.getPage(ep.UserInfo, cookie, ep.Origin, false); err == nil {
			if t := parsers.ExtractQwenSECToken(body); t != "" {
				return t
			}
			var info struct {
				Data struct {
					SECToken string `json:"secToken"`
				} `json:"data"`
			}
			if json.Unmarshal([]byte(body), &info) == nil && strings.TrimSpace(info.Data.SECToken) != "" {
				return strings.TrimSpace(info.Data.SECToken)
			}
		}
	}
	return ""
}

// getPage GET 页面（sec_token 专用）。navigate=true 时附加浏览器导航头：
// OneConsole shell 只对同源文档导航服务端渲染 SEC_TOKEN。
func (r *QwenRepo) getPage(rawURL, cookie, origin string, navigate bool) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if navigate {
		req.Header.Set("Referer", origin+"/")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	}
	resp, err := r.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", errors.New("页面获取失败")
	}
	return string(body), nil
}

// joinErrors 合并多个错误为多行文本（与 app 包 joinErrors 语义一致，repo 内自持）
func joinErrors(errs ...error) error {
	var msgs []string
	for _, e := range errs {
		if e != nil {
			msgs = append(msgs, e.Error())
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return errors.New(strings.Join(msgs, "\n"))
}

// normalizeCookieHeader 允许用户粘贴完整 `Cookie: xxx` 头，并保证单行
func normalizeCookieHeader(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) >= 7 && strings.EqualFold(s[:7], "cookie:") {
		s = strings.TrimSpace(s[7:])
	}
	return s
}

// cookieValue 从 Cookie 头取指定名（不存在返回空串）
func cookieValue(header, name string) string {
	for _, part := range strings.Split(header, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == name {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}
