package repo

// Commandai（Command Code，commandcode.ai）数据仓库。
//
// 平台特点（线协议见 docs/plans/2026-10-25-commandai-provider.md §一）：
//   - Base：https://api.commandcode.ai
//   - 认证：Authorization: Bearer <key>（/provider/v1/models 实测免认证，仍带 Bearer）
//   - 模型面：GET /provider/v1/models（OpenAI 兼容信封）
//   - 额度面：CLI 私有端点 /alpha/*，调用序列
//     whoami →（credits ∥ subscriptions）→ usage/summary
//   - 错误信封优先于状态码（bai tRPC 先例）：{success:false,error:{code,...}}
//   - 不做推理探活（额度有真值通道，探活烧真钱）

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// commandaiBaseURL Commandai API 基址（npm prod 环境）。
const commandaiBaseURL = "https://api.commandcode.ai"

// CommandaiRepo Commandai 数据仓库。BaseURL/Client 测试可注入（httptest）。
type CommandaiRepo struct {
	BaseURL string
	Client  *http.Client
}

// NewCommandaiRepo 默认端点 + 15s 超时 client。
func NewCommandaiRepo() *CommandaiRepo {
	return &CommandaiRepo{BaseURL: commandaiBaseURL, Client: defaultClient()}
}

func (r *CommandaiRepo) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return defaultClient()
}

func (r *CommandaiRepo) baseURL() string {
	if s := strings.TrimSpace(r.BaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	return commandaiBaseURL
}

// get 执行 GET：Bearer 认证 + 浏览器 UA。
// 空 key 合法（/provider/v1/models 实测免认证）——仅当账号显式无 key 时不带 Authorization。
// 401/403 先解析错误信封（信封优先于状态码），再退 HTTP 码路径。
func (r *CommandaiRepo) get(path, apiKey string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, r.baseURL()+path, nil)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", browserUA)
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
		if e := parsers.CommandaiEnvelopeError(string(body)); e != nil {
			return "", e
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate200(string(body)))
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate200(string(body)))
	default:
		return string(body), nil
	}
}

// Models 拉取模型清单（GET /provider/v1/models，Bearer）。
// 空 key 合法（端点免认证实测）——但仅当账号显式无 key 时调用（app 层守卫）。
func (r *CommandaiRepo) Models(apiKey string) (models.CommandaiPlan, error) {
	body, err := r.get("/provider/v1/models", apiKey)
	if err != nil {
		return models.CommandaiPlan{}, err
	}
	ms, err := parsers.ParseCommandaiModels(body)
	if err != nil {
		return models.CommandaiPlan{}, err
	}
	return models.CommandaiPlan{Models: ms}, nil
}

// Quota 额度链：whoami →（credits ∥ subscriptions）→ summary(since)。
// 返回四元组；whoami 401 → ErrCommandaiAuth（额度路整体失败，模型路不受影响）。
// 单段失败不静默：返回已成功段 + 错误（app 层 joinErrors）。
func (r *CommandaiRepo) Quota(apiKey string) (*models.CommandaiWhoami, *models.CommandaiCredits, *models.CommandaiSubscription, *models.CommandaiUsageSummary, error) {
	who, err := r.whoami(apiKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var (
		cred    models.CommandaiCredits
		credErr error
		sub     models.CommandaiSubscription
		subErr  error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cred, credErr = r.credits(apiKey, who.OrgID)
	}()
	go func() {
		defer wg.Done()
		sub, subErr = r.subscription(apiKey, who.OrgID)
	}()
	wg.Wait()

	var credPtr *models.CommandaiCredits
	if credErr == nil {
		credPtr = &cred
	}
	var subPtr *models.CommandaiSubscription
	if subErr == nil {
		subPtr = &sub
	}

	// summary 的 since：currentPeriodStart 原文非空 → 回传原文（服务端按己方格式
	// 解释，解析成功也回传原文）；都空 → 省略参数。
	since := ""
	if subPtr != nil {
		since = subPtr.PeriodStartRaw
	}
	sum, sumErr := r.summary(apiKey, who.OrgID, since)
	var sumPtr *models.CommandaiUsageSummary
	if sumErr == nil {
		sumPtr = &sum
	}
	return who, credPtr, subPtr, sumPtr, joinErrors(credErr, subErr, sumErr)
}

func (r *CommandaiRepo) whoami(apiKey string) (*models.CommandaiWhoami, error) {
	body, err := r.get("/alpha/whoami?limits=1", apiKey)
	if err != nil {
		return nil, err
	}
	w, err := parsers.ParseCommandaiWhoami(body)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *CommandaiRepo) credits(apiKey, orgID string) (models.CommandaiCredits, error) {
	body, err := r.get(commandaiPath("/alpha/billing/credits", orgID, ""), apiKey)
	if err != nil {
		return models.CommandaiCredits{}, err
	}
	return parsers.ParseCommandaiCredits(body)
}

func (r *CommandaiRepo) subscription(apiKey, orgID string) (models.CommandaiSubscription, error) {
	body, err := r.get(commandaiPath("/alpha/billing/subscriptions", orgID, ""), apiKey)
	if err != nil {
		return models.CommandaiSubscription{}, err
	}
	return parsers.ParseCommandaiSubscription(body)
}

func (r *CommandaiRepo) summary(apiKey, orgID, since string) (models.CommandaiUsageSummary, error) {
	body, err := r.get(commandaiPath("/alpha/usage/summary", orgID, since), apiKey)
	if err != nil {
		return models.CommandaiUsageSummary{}, err
	}
	return parsers.ParseCommandaiUsageSummary(body)
}

// commandaiPath 拼 /alpha/* 查询串：orgId / since 为空则整个参数省略。
// 实测带空 orgId（?orgId=）会 400 BAD_REQUEST「Invalid UUID at \"orgId\"」，
// 而个人账号 org 本就是 null——对齐 npm buildUsageEndpoint 的 null 参数省略行为。
func commandaiPath(path, orgID, since string) string {
	q := url.Values{}
	if s := strings.TrimSpace(orgID); s != "" {
		q.Set("orgId", s)
	}
	if s := strings.TrimSpace(since); s != "" {
		q.Set("since", s)
	}
	if enc := q.Encode(); enc != "" {
		return path + "?" + enc
	}
	return path
}
