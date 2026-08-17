// Package models 定义全部数据模型，逐类型移植自 Android 版
// com.xieguiawu.apicheckers.data.Models.kt。
//
// JSON tag 与 Android 版 kotlinx.serialization 序列化名一致：
//   - 账号列表（Account / DeepSeekAccount）用 camelCase（Kotlin 属性名）
//   - DeepSeek 原始 API 字段用 snake_case（total_balance 等）
package models

import "strings"

// ── OpenCode Go usage（官方 API） ──────────────────────────────

// GoWindow 单个用量窗口（rolling/weekly/monthly 共用结构）。
// 对应 Kotlin: data class GoWindow(status, percent, resetsAt)
type GoWindow struct {
	Status   string `json:"status"`
	Percent  int    `json:"percent"`
	ResetsAt string `json:"resetsAt"`
}

// GoUsage 三窗口用量。窗口可能缺失（指针为 nil），对应 Kotlin 可空字段。
type GoUsage struct {
	Rolling *GoWindow `json:"rolling"`
	Weekly  *GoWindow `json:"weekly"`
	Monthly *GoWindow `json:"monthly"`
}

// ── OpenCode Zen billing（页面解析） ───────────────────────────

// ZenBilling Zen 账单（SSR HTML 正则解析结果）。
// balanceUsd/monthlyUsageUsd 由 microcents ÷1e8 得来。
type ZenBilling struct {
	BalanceUsd       float64 `json:"balance_usd"`
	MonthlyUsageUsd  float64 `json:"monthly_usage_usd"`
	MonthlyLimitUsd  float64 `json:"monthly_limit_usd"`
	AutoReload       bool    `json:"auto_reload"`
	ReloadAmountUsd  float64 `json:"reload_amount_usd"`
	ReloadTriggerUsd float64 `json:"reload_trigger_usd"`
}

// ── DeepSeek 余额 ──────────────────────────────────────────────

// DeepSeekBalanceInfo 域模型：金额已由字符串转 float64（对应 Kotlin 中
// DeepSeekBalanceInfoPayload → DeepSeekBalanceInfo 的转换，toDoubleOrNull ?: 0.0）。
type DeepSeekBalanceInfo struct {
	Currency        string  `json:"currency"`
	TotalBalance    float64 `json:"total_balance"`
	GrantedBalance  float64 `json:"granted_balance"`
	ToppedUpBalance float64 `json:"topped_up_balance"`
}

// DeepSeekBalance 余额（is_available + 按币种余额列表）。
type DeepSeekBalance struct {
	IsAvailable bool                  `json:"is_available"`
	Infos       []DeepSeekBalanceInfo `json:"balance_infos"`
}

// ── DeepSeek 消费明细 ──────────────────────────────────────────

// DeepSeekCostDay 单日消费（date 格式 2006-01-02）。
type DeepSeekCostDay struct {
	Date  string  `json:"date"`
	Total float64 `json:"total"`
}

// DeepSeekCost 消费聚合：今天/近7天/近30天 + 全部天（按日期倒序）。
type DeepSeekCost struct {
	Today   float64           `json:"today"`
	Last7d  float64           `json:"last7d"`
	Last30d float64           `json:"last30d"`
	Days    []DeepSeekCostDay `json:"days"`
}

// ── 账号 ───────────────────────────────────────────────────────

// DeepSeekAccount DeepSeek 账号（支持多个 API key，各自查看余额/消费）。
// platformToken 可选：留空则只显示余额。
type DeepSeekAccount struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ApiKey        string `json:"apiKey"`
	PlatformToken string `json:"platformToken"`
}

// HasToken 是否配置了平台 Token（对应 Kotlin hasToken）。
func (a DeepSeekAccount) HasToken() bool { return strings.TrimSpace(a.PlatformToken) != "" }

// Account OpenCode 账号。
// workspaceId 与 authCookie 可选：留空则只显示 Go plan。
type Account struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	GoApiKey    string `json:"goApiKey"`
	WorkspaceId string `json:"workspaceId"`
	AuthCookie  string `json:"authCookie"`
}

// HasZen 只有同时配置了 workspace 与 cookie 才展示 Zen plan（对应 Kotlin hasZen）。
func (a Account) HasZen() bool {
	return strings.TrimSpace(a.WorkspaceId) != "" && strings.TrimSpace(a.AuthCookie) != ""
}
