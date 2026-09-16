package repo

// LongCat（美团龙猫）API 仓库：模型清单（OpenAI 兼容 /v1/models）+ 余额探活。
//
// 平台特点（实测，见 docs/plans/2026-09-13-longcat-provider.md 与
// docs/plans/2026-09-16-longcat-console-quota.md）：
//   - OpenAI 兼容格式：基础端点 https://api.longcat.chat/openai
//   - 认证：Authorization: Bearer YOUR_APP_KEY
//   - App Key 通道无公开配额接口 —— 余额只能间接推断：
//     GET /v1/models 成功 = key 有效且账户正常
//     小额 POST /v1/chat/completions → 402 = 余额不足（error.code=insufficient_quota）
//   - 控制台通道（longcat.chat，Cookie passport_token_key）可读 Token 资源包
//     （token-packs/summary）与按量余额（api-usage/summary），详见 ConsoleQuota/ConsolePaygo
//   - 401 invalid_api_key = key 无效；403 insufficient_quota = key 有效但余额不足
//   - 免费额度每日北京时间 0 点重置（不累计），付费为预付费余额模式

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// longcatBaseURL OpenAI 兼容端点前缀（LongCat 只开放 /openai 路由）。
const longcatBaseURL = "https://api.longcat.chat/openai"

// longcatConsoleBaseURL 控制台配额接口基址（与 API 端点 api.longcat.chat 不同域）。
// 认证仅需 Cookie: passport_token_key=<值>（2026-09-16 实测，无需签名/风控头）。
const longcatConsoleBaseURL = "https://longcat.chat"

// LongCatRepo LongCat 数据仓库。BaseURL/ConsoleURL/Client 测试可注入（httptest）。
type LongCatRepo struct {
	BaseURL    string
	ConsoleURL string
	Client     *http.Client
}

// NewLongCatRepo 默认端点 + 15s 超时 client
func NewLongCatRepo() *LongCatRepo {
	return &LongCatRepo{BaseURL: longcatBaseURL, Client: defaultClient()}
}

func (r *LongCatRepo) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return defaultClient()
}

func (r *LongCatRepo) baseURL() string {
	if strings.TrimSpace(r.BaseURL) != "" {
		return strings.TrimRight(r.BaseURL, "/")
	}
	return longcatBaseURL
}

// consoleBaseURL 控制台地址（空值退默认，ConsoleQuota/ConsolePaygo 共用）。
func (r *LongCatRepo) consoleBaseURL() string {
	if s := strings.TrimSpace(r.ConsoleURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	return longcatConsoleBaseURL
}

// ConsoleQuota 拉取 Token 资源包钱包（POST /api/pay/quota/metering/token-packs/summary，
// body 恒为 {}）。认证仅需 Cookie（normalizeConsoleCookie 容错）。
// HTTP 401/403 或信封 code==401 → parsers.ErrLongCatConsoleAuth。
func (r *LongCatRepo) ConsoleQuota(cookie string) (models.LongCatQuota, error) {
	body, err := r.consolePOST("/api/pay/quota/metering/token-packs/summary", cookie)
	if err != nil {
		return models.LongCatQuota{}, err
	}
	return parsers.ParseLongCatTokenPacksSummary(body)
}

// ConsolePaygo 拉取按量计费余额（POST /api/pay/quota/metering/api-usage/summary）。
// 错误口径同 ConsoleQuota。
func (r *LongCatRepo) ConsolePaygo(cookie string) (models.LongCatPaygo, error) {
	body, err := r.consolePOST("/api/pay/quota/metering/api-usage/summary", cookie)
	if err != nil {
		return models.LongCatPaygo{}, err
	}
	return parsers.ParseLongCatPaygoSummary(body)
}

// consolePOST 控制台配额 POST：body 恒为 {}，认证头只有 Cookie。
// 401/403 → ErrLongCatConsoleAuth；其他非 2xx → 「HTTP {code}: {body}」。
func (r *LongCatRepo) consolePOST(path, cookie string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, r.consoleBaseURL()+path, strings.NewReader("{}"))
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Cookie", normalizeConsoleCookie(cookie))
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
		return "", parsers.ErrLongCatConsoleAuth
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate200(string(body)))
	default:
		return string(body), nil
	}
}

// normalizeConsoleCookie 控制台 Cookie 容错：
//   - 完整 Cookie 头（可含 Cookie: 前缀，normalizeCookieHeader 处理）原样透传
//   - 不含 = 的值视为裸会话值，自动补 passport_token_key= 前缀（实测唯一必需项）
func normalizeConsoleCookie(cookie string) string {
	c := normalizeCookieHeader(cookie)
	if !strings.Contains(c, "=") {
		return "passport_token_key=" + c
	}
	return c
}

// longCatHeaders 请求头：Bearer 认证 + 浏览器 UA（LongCat 网关对裸客户端
// UA 未观测到拦截，但保持统一防御）。
func longCatHeaders(apiKey string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Accept":        "application/json",
		"User-Agent":    browserUA,
	}
}

// Models 拉取可用模型清单（API Key 认证）。
// 401 → ErrLongCatAuth；403 → ErrLongCatAuth（余额不足时 key 仍有效，但
// /v1/models 仍会返回 200——实测 403 只在余额为 0 且平台策略拒绝时出现）。
func (r *LongCatRepo) Models(apiKey string) (models.LongCatPlan, error) {
	if strings.TrimSpace(apiKey) == "" {
		return models.LongCatPlan{}, errors.New("未配置 API Key")
	}
	body, err := doGet(r.client(), r.baseURL()+"/v1/models",
		longCatHeaders(apiKey), parsers.ErrLongCatAuth.Error())
	if err != nil {
		return models.LongCatPlan{}, err
	}
	ms, err := parsers.ParseLongCatModels(body)
	if err != nil {
		return models.LongCatPlan{}, err
	}
	return models.LongCatPlan{Models: ms}, nil
}

// ProbeBalance 用最小推理请求探测余额状态。
//   - 200 → BalanceOK=true（余额充足）
//   - 402 → BalanceOK=false（余额不足，error.code=insufficient_quota）
//   - 401 → 返回 ErrLongCatAuth（key 无效，余额无从谈起）
//   - 其他错误 → 返回错误（不设置 BalanceOK）
//
// max_tokens=1 把消耗压到最低（LongCat 按成功请求计费，失败不收费）。
func (r *LongCatRepo) ProbeBalance(apiKey string) (bool, error) {
	if strings.TrimSpace(apiKey) == "" {
		return false, errors.New("未配置 API Key")
	}
	payload := `{"model":"LongCat-2.0","messages":[{"role":"user","content":"ping"}],"max_tokens":1,"stream":false}`
	req, err := http.NewRequest(http.MethodPost, r.baseURL()+"/v1/chat/completions", strings.NewReader(payload))
	if err != nil {
		return false, fmt.Errorf("构造探活请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", browserUA)
	resp, err := r.client().Do(req)
	if err != nil {
		return false, fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
		return true, nil
	case 401:
		return false, parsers.ErrLongCatAuth
	case 402:
		// 余额不足：key 有效但余额为 0 或已耗尽
		return false, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate200(string(body)))
	}
}
