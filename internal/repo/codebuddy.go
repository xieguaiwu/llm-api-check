package repo

// Codebuddy（腾讯 CodeBuddy，codebuddy.ai）数据仓库。
//
// 平台特点（线协议见 docs/plans/2026-11-05-codebuddy-provider.md §一）：
//   - Base：https://www.codebuddy.ai
//   - 认证：X-API-Key（product.json 的 API_KEY_HEADER；Bearer 对 /v2/accounts 也通，
//     但 /v3/config 的鉴权走 X-API-Key）
//   - 身份面：GET /v2/accounts
//   - 目录面：GET /v3/config —— 必须带 X-User-Id，否则 data.models 为 null
//   - 额度面：**服务端不存在**（§1.4 穷举证据）。额度只能从本机台账聚合：
//     ~/.codebuddy/projects/<slug>/<sessionId>.jsonl 的 providerData.rawUsage.credit
//   - 不做推理探活（无真值通道，探活烧真钱且得不出额度）

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// codebuddyBaseURL CodeBuddy 国际版端点（product.json endpoint）。
const codebuddyBaseURL = "https://www.codebuddy.ai"

// codebuddyUA 客户端 UA。实测任意 UA 均可通过；固定此值便于服务端日志追溯。
const codebuddyUA = "CLI/2.179.3 CodeBuddy/2.179.3"

// CodebuddyRepo CodeBuddy 数据仓库。BaseURL/Client/LedgerRoot 测试可注入（httptest / t.TempDir）。
type CodebuddyRepo struct {
	BaseURL string
	Client  *http.Client
	// LedgerRoot 台账根目录；空 → models.CodebuddyLedgerRoot()（跟随 CODEBUDDY_CONFIG_DIR）
	LedgerRoot string
}

// NewCodebuddyRepo 默认端点 + 15s 超时 client。
func NewCodebuddyRepo() *CodebuddyRepo {
	return &CodebuddyRepo{BaseURL: codebuddyBaseURL, Client: defaultClient()}
}

func (r *CodebuddyRepo) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return defaultClient()
}

func (r *CodebuddyRepo) baseURL() string {
	if s := strings.TrimSpace(r.BaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	return codebuddyBaseURL
}

// get 执行 GET：X-API-Key 认证 + CLI UA。
//
// 401 有两种形状，都要归一为 ErrCodebuddyAuth：
//   - 无 key → APISIX HTML（<html>…401 Authorization Required…）
//   - 坏 key → JSON {"message":"not_found"}
func (r *CodebuddyRepo) get(path, apiKey, uid string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, r.baseURL()+path, nil)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	if strings.TrimSpace(uid) != "" {
		req.Header.Set("X-User-Id", uid)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", codebuddyUA)
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
		return "", parsers.ErrCodebuddyAuth
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate200(string(body)))
	default:
		return string(body), nil
	}
}

// Accounts 拉取账号身份（GET /v2/accounts）。uid 供目录面的 X-User-Id 使用。
func (r *CodebuddyRepo) Accounts(apiKey string) ([]models.CodebuddyIdentity, error) {
	body, err := r.get("/v2/accounts", apiKey, "")
	if err != nil {
		return nil, err
	}
	return parsers.ParseCodebuddyAccounts(body)
}

// Catalog 拉取模型目录（GET /v3/config）。
// uid 必填：缺失时服务端返回 data.models=null（实测），解析层会显式报错。
func (r *CodebuddyRepo) Catalog(apiKey, uid string) (models.CodebuddyCatalog, error) {
	body, err := r.get("/v3/config", apiKey, uid)
	if err != nil {
		return models.CodebuddyCatalog{}, err
	}
	return parsers.ParseCodebuddyCatalog(body)
}

// Ledger 扫描本地台账（纯文件 IO，不联网）。
// now 注入用于时间窗切分（生产调用传 time.Now()）。
func (r *CodebuddyRepo) Ledger(now time.Time) (models.CodebuddyLedger, error) {
	root := strings.TrimSpace(r.LedgerRoot)
	if root == "" {
		root = models.CodebuddyLedgerRoot()
	}
	return parsers.ParseCodebuddyLedger(root, now)
}
