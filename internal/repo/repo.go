// Package repo 数据仓库：DeepSeek（余额 + 消费明细）与 OpenCode（Go usage + Zen billing）。
// 逐行对照 Android 版 com.xieguiawu.apicheckers.data.Repositories.kt + ApiClient.kt 移植。
package repo

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// UA 移动端 UA，Zen billing 页面按浏览器解析（与 Android ApiClient.UA 一致）
const UA = "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"

// 所有网络超时 15s（全局约束，等价 OkHttp connect/read/write 15s）
const timeout = 15 * time.Second

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
		b := body
		if len(b) > 200 {
			b = b[:200]
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
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
	// (year/month 取自同一时间)：上月跨年时 year 必须取 AddDate 之后的 year
	months := []time.Time{now, now.AddDate(0, -1, 0)}
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
