// 智星云 AI Galaxy OpenAPI v2 仓库（AccessKey + SecretKey + MD5 签名）。
//
// 平台特点（实测，见 docs/plans/2026-08-29-ai-galaxy-provider.md）：
//   - 统一 POST + application/x-www-form-urlencoded，所有参数（含 sign）走 body
//   - HTTP 状态码恒为 200，错误在信封里（{success, code:"4000", message}）
//     ——与 Qwen 控制台网关同坑，只看状态码会把失败读成成功
//   - page_size 硬上限 100（超限回 code=4000 "page_size参数超限!"）
//   - 实例列表响应内含实例 root/桌面明文口令，本仓库只把解析后的白名单结构
//     交给上层，原始响应体绝不外传（错误消息同样只带 message，不带 data）
package repo

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// GalaxyBaseURL 官方 OpenAPI v2 前缀（文档「开始使用」）
const GalaxyBaseURL = "https://app.ai-galaxy.cn/openapi/v2"

// GalaxyMaxPageSize 平台 page_size 上限（实测 >100 报「page_size参数超限!」）
const GalaxyMaxPageSize = 100

// 实例分组过滤值（平台语义）
const (
	GalaxyStatusDefault = "statusDefault" // 1,4,5,-1,7,8
	GalaxyStatusRunning = "statusRunning" // 1,4,5
	GalaxyStatusAll     = "statusAll"     // 不过滤
)

// galaxyCodeOK 成功状态码（字符串，不是整型——契约明确）
const galaxyCodeOK = "2000"

// GalaxyRepo 智星云数据仓库。BaseURL/Client/Now/Nonce 测试可注入。
type GalaxyRepo struct {
	BaseURL string
	Client  *http.Client
	// Now 时钟注入（到期时间折算与请求 timestamp）
	Now func() time.Time
	// Nonce 随机串注入（测试确定性）；nil 时用 crypto/rand
	Nonce func() string
	// CostPages 余额变更明细最大翻页数（≤0 → 默认 8 页 = 800 条：实测该账号
	// 每天约 100 条变更，5 页覆盖不满 7 天窗口，只能标「≥」下限）
	CostPages int
	// InstancePages 实例列表最大翻页数（≤0 → 默认 3 页）
	InstancePages int
}

// NewGalaxyRepo 默认端点 + 15s 超时 client
func NewGalaxyRepo() *GalaxyRepo {
	return &GalaxyRepo{BaseURL: GalaxyBaseURL, Client: defaultClient()}
}

func (r *GalaxyRepo) baseURL() string {
	if strings.TrimSpace(r.BaseURL) != "" {
		return strings.TrimRight(r.BaseURL, "/")
	}
	return GalaxyBaseURL
}

func (r *GalaxyRepo) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return defaultClient()
}

func (r *GalaxyRepo) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *GalaxyRepo) nonce() string {
	if r.Nonce != nil {
		return r.Nonce()
	}
	return galaxyRandomNonce(12)
}

// galaxyRandomNonce 生成 n 位字母数字随机串（平台要求 ≥8 位且一段时间内不可重复）。
// crypto/rand 不可用时回落时间戳+固定前缀——宁可退化也不 panic（读路径不该崩）。
func galaxyRandomNonce(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return fmt.Sprintf("%d%08d", time.Now().UnixNano(), i)[:n]
		}
		b[i] = alphabet[v.Int64()]
	}
	return string(b)
}

// ── 签名请求 ───────────────────────────────────────────────────

// galaxyEnvelope OpenAPI 统一响应信封。Code 用 RawMessage 宽容接收
// （文档强调是字符串，但平台偶发回数字时不该整体解析失败）。
type galaxyEnvelope struct {
	Success bool            `json:"success"`
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// call 发一次签名请求，返回 data 节点的原始 JSON。
// 认证类错误（AccessKey/SecretKey/实名）映射成可操作中文提示，供上层直接展示。
func (r *GalaxyRepo) call(acc models.GalaxyAccount, path string, params map[string]string) (string, error) {
	if strings.TrimSpace(acc.AccessKey) == "" || strings.TrimSpace(acc.SecretKey) == "" {
		return "", errors.New("未配置 AccessKey / SecretKey，运行 llm-api-check accounts add --type galaxy --help 添加")
	}
	all := make(map[string]string, len(params)+3)
	for k, v := range params {
		if v != "" {
			all[k] = v
		}
	}
	all["apikey"] = acc.AccessKey
	all["timestamp"] = fmt.Sprintf("%d", r.now().Unix())
	all["nonce"] = r.nonce()
	all["sign"] = parsers.GalaxySign(all, acc.SecretKey)

	form := url.Values{}
	for k, v := range all {
		form.Set(k, v)
	}
	req, err := http.NewRequest(http.MethodPost, r.baseURL()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate200(string(body)))
	}
	var env galaxyEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("智星云响应格式错误: %s", truncate200(string(body)))
	}
	code := galaxyCodeString(env.Code)
	if !env.Success || code != galaxyCodeOK {
		return "", galaxyError(code, env.Message)
	}
	data := strings.TrimSpace(string(env.Data))
	if data == "" || data == "null" {
		return "", errors.New("智星云响应缺少 data 字段")
	}
	return data, nil
}

// galaxyCodeString code 字段宽容取字符串形式（"2000" / 2000 都接受）。
// 数字形式只认整数值——code:2000.5 不是合法成功码（对齐 Kotlin 严格性，
// 避免把半截错误码截断成成功）。
func galaxyCodeString(r json.RawMessage) string {
	if len(r) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		return s
	}
	var f float64
	if err := json.Unmarshal(r, &f); err == nil && f == math.Trunc(f) {
		return fmt.Sprintf("%d", int64(f))
	}
	return strings.Trim(string(r), `"`)
}

// galaxyError 把平台 message 映射成中文可操作提示。
// 实测文案：accesskey不存在! / sign验证失败! / nonce参数缺失! / page_size参数超限!
func galaxyError(code, message string) error {
	msg := strings.TrimSpace(message)
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "accesskey"):
		return errors.New("AccessKey 无效或已删除，请在控制台「开放API → AccessKey管理」重新创建")
	case strings.Contains(low, "sign") || strings.Contains(msg, "签名"):
		return errors.New("签名校验失败：SecretKey 与 AccessKey 不匹配或已重置，请重新录入账号")
	case strings.Contains(msg, "实名"):
		return errors.New("账号未完成实名认证，OpenAPI 不可用（控制台 → 实名认证）")
	case strings.Contains(msg, "时间戳") || strings.Contains(low, "timestamp"):
		return errors.New("请求时间戳被拒绝：本机时钟不准，请同步系统时间后重试")
	case msg == "":
		return fmt.Errorf("智星云接口错误（code=%s）", code)
	default:
		return fmt.Errorf("智星云接口错误（code=%s）：%s", code, truncate200(msg))
	}
}

// ── 对外数据方法 ───────────────────────────────────────────────

// Balance 主账户余额（现金 / 算力券 / 信用额度 / VIP）。
func (r *GalaxyRepo) Balance(acc models.GalaxyAccount) (models.GalaxyBalance, error) {
	data, err := r.call(acc, "/account/get_main_account_info", nil)
	if err != nil {
		return models.GalaxyBalance{}, err
	}
	return parsers.ParseGalaxyBalance(data)
}

// StatusCount 实例状态统计。
func (r *GalaxyRepo) StatusCount(acc models.GalaxyAccount) (models.GalaxyStatusCount, error) {
	data, err := r.call(acc, "/instance/get_instance_status_count", nil)
	if err != nil {
		return models.GalaxyStatusCount{}, err
	}
	return parsers.ParseGalaxyStatusCount(data)
}

// Instances 实例列表（statusType 传 GalaxyStatusDefault 等）。
// limit ≤0 表示不限量（仍受翻页上限约束）；pageSize 自动夹到 [1,100]。
func (r *GalaxyRepo) Instances(acc models.GalaxyAccount, statusType string, limit int) ([]models.GalaxyInstance, error) {
	pageSize := limit
	switch {
	case pageSize <= 0 || pageSize > GalaxyMaxPageSize:
		pageSize = GalaxyMaxPageSize
	}
	maxPages := r.InstancePages
	if maxPages <= 0 {
		maxPages = 3
	}
	now := r.now()
	var out []models.GalaxyInstance
	for page := 1; page <= maxPages; page++ {
		data, err := r.call(acc, "/instance/get_instance_list", map[string]string{
			"page":        fmt.Sprintf("%d", page),
			"page_size":   fmt.Sprintf("%d", pageSize),
			"status_type": statusType,
		})
		if err != nil {
			return nil, err
		}
		list, _, hasMore, err := parsers.ParseGalaxyInstances(data, now)
		if err != nil {
			return nil, err
		}
		out = append(out, list...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
		if !hasMore || len(list) == 0 {
			return out, nil
		}
	}
	return out, nil
}

// Cost 今日 / 近 7 天净消耗（余额变更明细聚合）。明细按时间倒序，翻到出现
// 早于「近 7 天」下界的记录即停（两个窗口都取完），上限 CostPages 页。
func (r *GalaxyRepo) Cost(acc models.GalaxyAccount) (models.GalaxyCost, error) {
	maxPages := r.CostPages
	if maxPages <= 0 {
		maxPages = 8
	}
	now := r.now() // 只取一次时钟：跨午夜时翻页、窗口判定、聚合用同一基准（oracle P4-6）
	var all []parsers.GalaxyChange
	hasMore := false
	for page := 1; page <= maxPages; page++ {
		data, err := r.call(acc, "/billing/get_balance_change_list", map[string]string{
			"page":      fmt.Sprintf("%d", page),
			"page_size": fmt.Sprintf("%d", GalaxyMaxPageSize),
		})
		if err != nil {
			return models.GalaxyCost{}, err
		}
		items, more, err := parsers.ParseGalaxyChanges(data)
		if err != nil {
			return models.GalaxyCost{}, err
		}
		all = append(all, items...)
		hasMore = more
		if !more {
			break
		}
		if costWindowCovered(all, now) {
			break
		}
	}
	return parsers.AggregateGalaxyCost(all, hasMore, now), nil
}

// costWindowCovered 已取到的变更是否已跨过「近 7 天」窗口下界
// （跨过则两个窗口均已取完，可提前停止翻页）
func costWindowCovered(changes []parsers.GalaxyChange, now time.Time) bool {
	day7 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -6)
	for _, c := range changes {
		if c.At.Before(day7) {
			return true
		}
	}
	return false
}
