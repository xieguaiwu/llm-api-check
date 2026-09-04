// Package models 定义全部数据模型，逐类型移植自 Android 版
// com.xieguiawu.apicheckers.data.Models.kt。
//
// JSON tag 与 Android 版 kotlinx.serialization 序列化名一致：
//   - 账号列表（Account / DeepSeekAccount / QwenAccount）用 camelCase（Kotlin 属性名）
//   - DeepSeek 原始 API 字段用 snake_case（total_balance 等）
package models

import (
	"errors"
	"strings"
)

// ── Qwen Token Plan（阿里云百炼订阅）区域常量 ─────────────────

const (
	// RegionQwenCN 中国大陆（北京）：网关 token-plan.cn-beijing.maas.aliyuncs.com
	RegionQwenCN = "cn-beijing"
	// RegionQwenIntl 国际（新加坡）：网关 token-plan.ap-southeast-1.maas.aliyuncs.com
	RegionQwenIntl = "ap-southeast-1"
)

// NormalizeQwenRegion 归一化区域取值。空串 → 默认中国大陆。
// 别名：cn/domestic/beijing → cn-beijing；intl/singapore/international → ap-southeast-1。
func NormalizeQwenRegion(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "cn", "cn-beijing", "domestic", "beijing", "china":
		return RegionQwenCN, nil
	case "intl", "international", "singapore", "ap-southeast-1", "sg":
		return RegionQwenIntl, nil
	default:
		return "", errors.New("Qwen 区域不支持：" + s + "（可用值 cn-beijing / ap-southeast-1）")
	}
}

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

// ── 白B.AI（api.b.ai 免费通道） ───────────────────────────────

// BaiAccount 白B.AI 账号。apiKey 为 chat.b.ai 侧栏创建的 sk- 密钥；
// 平台只开放推理路径（/v1/models 等），无余额/配额端点可查（实测 403）。
type BaiAccount struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	ApiKey string `json:"apiKey"`
}

// BaiModel 单个模型（/v1/models 列表项白名单）。
type BaiModel struct {
	ID        string   `json:"id"`
	OwnedBy   string   `json:"owned_by"`
	Endpoints []string `json:"supported_endpoint_types"`
}

// BaiPlan 模型清单（API Key 认证）。
type BaiPlan struct {
	Models []BaiModel `json:"models"`
}

// BaiFreeFlashModels 免费 0-Credits flash 通道盯梢清单——pi-subagent 默认免费
// 模型源（bai/qwen3.8-flash、bai/deepseek-v4-flash）依赖它们在线。
// 清单缺失时 CLI 提示，不参与断言（快照 2026-09-04）。
var BaiFreeFlashModels = []string{
	"deepseek-v4-flash",
	"deepseek-v4-flash-vision-exp",
	"glm-5.3-flash",
	"qwen3.8-flash",
}

// MissingFreeFlash 返回免费通道清单中不在模型列表里的项。
func (p BaiPlan) MissingFreeFlash() []string {
	have := make(map[string]bool, len(p.Models))
	for _, m := range p.Models {
		have[m.ID] = true
	}
	var missing []string
	for _, want := range BaiFreeFlashModels {
		if !have[want] {
			missing = append(missing, want)
		}
	}
	return missing
}

// ── Qwen Token Plan（订阅） ────────────────────────────────────

// QwenAccount Qwen 账号。apiKey 为 Token Plan 订阅密钥（sk-sp- 前缀）；
// consoleCookie 可选：阿里云百炼控制台 Cookie，缺失时只能显示套餐模型清单，
// 无法显示配额窗口（用量接口只认控制台会话，实测 API Key 返回
// BailianGateway.Login.NotLogined）。
type QwenAccount struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ApiKey        string `json:"apiKey"`
	ConsoleCookie string `json:"consoleCookie"`
	Region        string `json:"region"`
}

// HasCookie 是否配置了控制台 Cookie（对应 Kotlin hasCookie）。
func (a QwenAccount) HasCookie() bool { return strings.TrimSpace(a.ConsoleCookie) != "" }

// QwenRegion 归一化后的区域（非法/空值回落中国大陆）。
func (a QwenAccount) QwenRegion() string {
	r, err := NormalizeQwenRegion(a.Region)
	if err != nil {
		return RegionQwenCN
	}
	return r
}

// QwenRegionDisplayName 区域展示名（供 CLI 与 UI 共用）。
func QwenRegionDisplayName(region string) string {
	if r, err := NormalizeQwenRegion(region); err == nil && r == RegionQwenIntl {
		return "国际（新加坡）"
	}
	return "中国大陆（北京）"
}

// QwenWindow 订阅滚动窗口（5 小时 / 7 天）。
// Percent 由接口返回的比例值（0-1）截断取整；ResetsAt 为 RFC3339；
// Exhausted 由原始比例 ≥ 1 推导（官方规则：窗口内配额用尽则暂停服务）。
type QwenWindow struct {
	Percent   int    `json:"percent"`
	ResetsAt  string `json:"resetsAt"`
	Exhausted bool   `json:"exhausted"`
}

// QwenUsage 控制台用量接口结果（Cookie 认证）。窗口可能缺失（指针为 nil）。
type QwenUsage struct {
	PlanCode string      `json:"plan_code"`
	FiveHour *QwenWindow `json:"five_hour"`
	Weekly   *QwenWindow `json:"weekly"`
}

// QwenPlan 网关模型清单（API Key 认证）。
type QwenPlan struct {
	Models []string `json:"models"`
}

// ── 智星云 AI Galaxy（GPU 算力云） ───────────────────────────

// GalaxyAccount 智星云账号。凭据 = 控制台「开放API → AccessKey管理」创建的
// AccessKey/SecretKey（需先完成实名认证）；两者缺一不可，故无 HasXxx 可选分支。
type GalaxyAccount struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

// GalaxyBalance 主账户余额。三项金额语义不同、平台各自扣费，不互相折算：
//   - Money            现金余额（充值所得，元）
//   - PowerMoney       算力券（活动/退租返还，只能抵扣实例费用）
//   - CreditMoneyQuota 信用额度（余额≤0 时可透支的上限）
//
// 控制台另有「可用余额 = Money + 信用额度 + 算力券 − 冻结」，OpenAPI 不返回冻结
// 金额，故本结构不加总、分列展示（见 docs/plans/2026-08-29-ai-galaxy-provider.md §二）。
type GalaxyBalance struct {
	Name             string  `json:"name"`
	Phone            string  `json:"phone"` // 已脱敏（183****2433）
	Money            float64 `json:"money"`
	PowerMoney       float64 `json:"power_money"`
	CreditMoneyQuota float64 `json:"credit_money_quota"`
	VipLevel         int     `json:"vip_level"`
	CustomDiscount   float64 `json:"custom_discount"`
	LastLoginAt      string  `json:"last_login_at"` // RFC3339，空串表示无记录
}

// GalaxyStatusCount 实例状态统计。刻意不含 statusDefault：实测统计端点回 9、
// 同 status_type 的列表只回 4，两个数同屏互相矛盾（契约 §2.4）。
type GalaxyStatusCount struct {
	All          int `json:"all"`
	Running      int `json:"running"`
	KeeppedDisk  int `json:"keepped_disk"`
	CreateError  int `json:"create_error"`
	RunningError int `json:"running_error"`
}

// GalaxyInstance 云主机实例。字段是显式白名单——接口响应里含 Init_passwd /
// LastInitPasswd / RdpPasswd / VncPasswd 明文口令，任何一层都不允许透传。
type GalaxyInstance struct {
	Name          string  `json:"name"` // Container_name（平台侧唯一名）
	Note          string  `json:"note"`
	Status        int     `json:"status"`
	StatusText    string  `json:"status_text"`
	Abnormal      bool    `json:"abnormal"`
	GpuType       string  `json:"gpu_type"`
	GpuNum        int     `json:"gpu_num"`
	CpuNum        int     `json:"cpu_num"`
	MemoryGB      int     `json:"memory_gb"`
	District      string  `json:"district"`
	Host          string  `json:"host"` // 平台内部机名（如 lyg2030）
	SSHHost       string  `json:"ssh_host"`
	SSHPort       int     `json:"ssh_port"`
	Image         string  `json:"image"`
	Kind          string  `json:"kind"` // kvm / docker
	DueAt         string  `json:"due_at"`
	DiskReleaseAt string  `json:"disk_release_at"`
	TotalCost     float64 `json:"total_cost"` // 小时单价（元/时）
	PayType       string  `json:"pay_type"`   // money / power
	AutoRenew     bool    `json:"auto_renew"`
	CreatedAt     string  `json:"created_at"`
}

// GalaxyCost 近期消耗（余额变更明细聚合）。净消耗 = −(ΔMoney+ΔPower)，净额为负的
// 返还/充值不计入。两个 *Partial 分别标记「今日」「近 7 天」窗口是否已翻完明细——
// 明细按时间倒序，只要取到早于窗口下界的一条，该窗口数值即为精确值。
type GalaxyCost struct {
	Today        float64           `json:"today"`
	Last7d       float64           `json:"last7d"`
	TodayPartial bool              `json:"today_partial"`
	WeekPartial  bool              `json:"week_partial"`
	Entries      []GalaxyCostEntry `json:"entries,omitempty"`
}

// GalaxyCostEntry 单条余额变更（只留展示需要的四项）。
type GalaxyCostEntry struct {
	Time   string  `json:"time"` // RFC3339
	Remark string  `json:"remark"`
	Spent  float64 `json:"spent"` // 正数＝扣费，负数＝返还
	Left   float64 `json:"left"`  // 变更后现金余额
}

// ── Qwen 用量分析（Bailian CLI `usage summary --output json`） ──

// QwenTokenStat 单项 token 统计（Input/Output/Total/Avg）。
type QwenTokenStat struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
	Unit  string `json:"unit"`
	Label string `json:"label"`
}

// QwenUsageStats 统计周期内的调用汇总。
type QwenUsageStats struct {
	ModelsCalled    int             `json:"modelsCalled"`
	SuccessfulCalls int             `json:"successfulCalls"`
	Usages          []QwenTokenStat `json:"usages"`
}

// QwenFreeTier 模型免费额度（无 Token Plan 抵扣）。
type QwenFreeTier struct {
	Model            string  `json:"model"`
	Type             string  `json:"type"`
	Remaining        int64   `json:"remaining"`
	Total            int64   `json:"total"`
	RemainingPercent float64 `json:"remainingPercent"`
	Expires          string  `json:"expires"`
}

// QwenSummaryPeriod 统计周期。
type QwenSummaryPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Days  int    `json:"days"`
}

// QwenSummary 用量分析：周期 + 免费额度 + token 统计（Bailian CLI Console 认证）。
type QwenSummary struct {
	Period   QwenSummaryPeriod `json:"period"`
	FreeTier []QwenFreeTier    `json:"freeTier"`
	Usage    QwenUsageStats    `json:"usage"`
}
