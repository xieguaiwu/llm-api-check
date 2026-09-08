package repo

// GPTZero（AI 检测服务）仓库：账号与月度额度（GET /v2/users/me，x-api-key 认证）。
// 契约见 docs/plans/2026-09-07-gptzero-provider.md（2026-09-07 真实 key 实测）。

import (
	"errors"
	"net/http"
	"strings"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// gptzeroBaseURL 默认端点。直连可通（实测 200，无需代理）；历史教训：Python
// urllib 裸 UA 会被 Cloudflare 403（error 1010），故请求头显式带浏览器 UA。
const gptzeroBaseURL = "https://api.gptzero.me"

// GptzeroRepo GPTZero 数据仓库。BaseURL/Client 测试可注入（httptest）。
type GptzeroRepo struct {
	BaseURL string
	Client  *http.Client
}

// NewGptzeroRepo 默认端点 + 15s 超时 client
func NewGptzeroRepo() *GptzeroRepo {
	return &GptzeroRepo{BaseURL: gptzeroBaseURL, Client: defaultClient()}
}

func (r *GptzeroRepo) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return defaultClient()
}

// gptzeroHeaders 请求头：x-api-key（非 Bearer）+ 浏览器 UA（Cloudflare 对裸
// 客户端 UA 有 403 前科，显式规避）。
func gptzeroHeaders(apiKey string) map[string]string {
	return map[string]string{
		"x-api-key":  apiKey,
		"Accept":     "application/json",
		"User-Agent": browserUA,
	}
}

// Usage 查询账号与月度词数额度（单端点，只读）。
// 401/403 归一 ErrGptzeroAuth——403 响应体会原样回显 key（apiKey 字段），
// 原文不得进终端或 --json，doGet 层统一文案。
func (r *GptzeroRepo) Usage(apiKey string) (models.GptzeroUsage, error) {
	if strings.TrimSpace(apiKey) == "" {
		return models.GptzeroUsage{}, errors.New("未配置 API Key")
	}
	base := r.BaseURL
	if strings.TrimSpace(base) == "" {
		base = gptzeroBaseURL
	}
	body, err := doGet(r.client(), base+"/v2/users/me",
		gptzeroHeaders(apiKey), parsers.ErrGptzeroAuth.Error())
	if err != nil {
		return models.GptzeroUsage{}, err
	}
	return parsers.ParseGptzeroUsage(body)
}
