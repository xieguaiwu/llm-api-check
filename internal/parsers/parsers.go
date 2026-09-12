// Package parsers 三个外部数据源的解析器，逐行对照 Android 版
// com.xieguiawu.apicheckers.data.Parsers.kt 移植。
package parsers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

// ── OpenCode Go usage：官方 API JSON ────────────────────────────

// ParseGoUsage 解析 Go usage 官方 API 响应（对应 parseGoUsage）。
// Go 的 json.Unmarshal 默认忽略未知字段，等价 Kotlin ignoreUnknownKeys = true。
func ParseGoUsage(raw string) (models.GoUsage, error) {
	var payload struct {
		Usage models.GoUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return models.GoUsage{}, fmt.Errorf("Go usage JSON 解析失败: %w", err)
	}
	return payload.Usage, nil
}

// ── OpenCode Zen billing：SSR HTML 正则解析 ─────────────────────
// 算法移植自 MIT 项目 4cya/pi-go-bars core.ts（已授权复用）：
// 1) 以 customerID:"cus_ 为锚点；2) 向前找对象起始 {（跳过字符串字面量——
//    字符串内可能含 { 字符，这是对旧算法的修复）；3) 深度计数取匹配 }（同样
//    跳过字符串字面量）；4) 对象内按字段正则逐个匹配（字段顺序可变）。

var (
	// 前导逗号/行首断言防止误匹配 xxxbalance:
	reBalance       = regexp.MustCompile(`(?:^|,)balance:(-?\d+(?:\.\d+)?)`)
	reMonthlyUsage  = regexp.MustCompile(`monthlyUsage:(-?\d+(?:\.\d+)?)`)
	reMonthlyLimit  = regexp.MustCompile(`monthlyLimit:(-?\d+(?:\.\d+)?)`)
	reReload        = regexp.MustCompile(`reload:(!0|!1|true|false|null)`)
	reReloadAmount  = regexp.MustCompile(`reloadAmount:(-?\d+(?:\.\d+)?)`)
	reReloadTrigger = regexp.MustCompile(`reloadTrigger:(-?\d+(?:\.\d+)?)`)
)

// ParseZenBilling 解析 Zen billing SSR 页面（对应 parseZenBilling）。
// 错误消息与 Android 版逐字一致（见实施计划错误消息对照表）。
func ParseZenBilling(html string) (models.ZenBilling, error) {
	start := strings.Index(html, `customerID:"cus_`)
	if start == -1 {
		return models.ZenBilling{}, errors.New("会话已过期，请更新 Cookie")
	}
	// 从锚点向前找对象起始 {：跳过字符串字面量（字符串内可能含 { 字符）
	braceStart := -1
	inStr, esc := false, false
scanBack:
	for i := start - 1; i >= 0; i-- {
		c := html[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			braceStart = i
			break scanBack
		}
	}
	if braceStart == -1 {
		return models.ZenBilling{}, errors.New("账单页面结构异常")
	}
	// 深度计数到匹配 }：同样跳过字符串字面量
	depth := 0
	end := -1
	inStr, esc = false, false
scanForward:
	for i := braceStart; i < len(html); i++ {
		c := html[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
				break scanForward
			}
		}
	}
	if end == -1 {
		return models.ZenBilling{}, errors.New("账单页面结构异常")
	}
	obj := html[braceStart : end+1]
	// num：正则匹配失败或非数字 → (0, false)，等价 Kotlin toDoubleOrNull 判空
	num := func(re *regexp.Regexp) (float64, bool) {
		m := re.FindStringSubmatch(obj)
		if m == nil {
			return 0, false
		}
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	balance, hasBalance := num(reBalance)
	monthlyUsage, hasMonthlyUsage := num(reMonthlyUsage)
	monthlyLimit, hasMonthlyLimit := num(reMonthlyLimit)
	if !hasBalance && !hasMonthlyUsage && !hasMonthlyLimit {
		return models.ZenBilling{}, errors.New("账单页面结构已变化，请更新应用")
	}
	reloadAmount, _ := num(reReloadAmount)
	reloadTrigger, _ := num(reReloadTrigger)
	autoReload := false
	if m := reReload.FindStringSubmatch(obj); m != nil {
		autoReload = m[1] == "!0" || m[1] == "true"
	}
	return models.ZenBilling{
		BalanceUsd:       balance / 1e8,      // microcents → USD
		MonthlyUsageUsd:  monthlyUsage / 1e8, // microcents → USD
		MonthlyLimitUsd:  monthlyLimit,       // 整 USD
		AutoReload:       autoReload,
		ReloadAmountUsd:  reloadAmount,  // 整 USD
		ReloadTriggerUsd: reloadTrigger, // 整 USD
	}, nil
}

// ── DeepSeek 余额：官方 API JSON ───────────────────────────────

// API 原始响应 DTO：金额为字符串（直接对应 JSON，同 Android DeepSeekBalancePayload）
type deepSeekBalancePayload struct {
	IsAvailable  bool                         `json:"is_available"`
	BalanceInfos []deepSeekBalanceInfoPayload `json:"balance_infos"`
}

type deepSeekBalanceInfoPayload struct {
	Currency        string `json:"currency"`
	TotalBalance    string `json:"total_balance"`
	GrantedBalance  string `json:"granted_balance"`
	ToppedUpBalance string `json:"topped_up_balance"`
}

// ParseDeepSeekBalance 解析余额响应（对应 parseDeepSeekBalance）。
// 字符串金额转换失败兜底 0.0，等价 Kotlin toDoubleOrNull ?: 0.0。
func ParseDeepSeekBalance(raw string) (models.DeepSeekBalance, error) {
	var p deepSeekBalancePayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return models.DeepSeekBalance{}, fmt.Errorf("DeepSeek 余额 JSON 解析失败: %w", err)
	}
	infos := make([]models.DeepSeekBalanceInfo, 0, len(p.BalanceInfos))
	for _, it := range p.BalanceInfos {
		infos = append(infos, models.DeepSeekBalanceInfo{
			Currency:        it.Currency,
			TotalBalance:    parseAmount(it.TotalBalance),
			GrantedBalance:  parseAmount(it.GrantedBalance),
			ToppedUpBalance: parseAmount(it.ToppedUpBalance),
		})
	}
	return models.DeepSeekBalance{IsAvailable: p.IsAvailable, Infos: infos}, nil
}

// parseAmount 字符串金额 → float64，失败兜底 0.0（等价 toDoubleOrNull ?: 0.0）
func parseAmount(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return f
}

// ── DeepSeek 消费明细：platform 页面 API JSON ──────────────────

// ParseDeepSeekCost 解析消费明细（对应 parseDeepSeekCost）。
// refDate 传零值表示今天（测试可传固定日期保证确定性）。
func ParseDeepSeekCost(raw string, refDate time.Time) (models.DeepSeekCost, error) {
	// code 字段可能缺失或非整数（对应 Kotlin jsonPrimitive.intOrNull 的宽容语义）
	var head struct {
		Code json.RawMessage `json:"code"`
	}
	if err := json.Unmarshal([]byte(raw), &head); err != nil {
		return models.DeepSeekCost{}, fmt.Errorf("DeepSeek 消费 JSON 解析失败: %w", err)
	}
	if code, ok := rawInt(head.Code); ok {
		if code == 40003 {
			return models.DeepSeekCost{}, errors.New("DeepSeek 平台登录已失效，请更新平台 Token")
		}
		if code != 0 {
			return models.DeepSeekCost{}, fmt.Errorf("DeepSeek 平台接口错误（code=%d）", code)
		}
	}
	// amount 用 RawMessage 逐字段宽容解析（对应 doubleOrNull ?: 0.0）
	var payload struct {
		Data struct {
			BizData []struct {
				Days []struct {
					Date *string `json:"date"`
					Data []struct {
						Usage []struct {
							Amount json.RawMessage `json:"amount"`
						} `json:"usage"`
					} `json:"data"`
				} `json:"days"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return models.DeepSeekCost{}, fmt.Errorf("DeepSeek 消费 JSON 解析失败: %w", err)
	}
	dayMap := map[string]float64{}
	for _, biz := range payload.Data.BizData {
		for _, dayEl := range biz.Days {
			if dayEl.Date == nil {
				continue
			}
			var total float64
			for _, modelEl := range dayEl.Data {
				for _, u := range modelEl.Usage {
					total += rawFloat(u.Amount)
				}
			}
			// 同响应内重复日期取后者（对应 Kotlin associate 覆盖语义）
			dayMap[*dayEl.Date] = total
		}
	}
	// 今天/近7天/近30天：以 refDate 为基准
	return AggregateCost(dayMap, refDate), nil
}

// AggregateCost 按参考日期聚合消费：today/7d/30d + 全部天（按日期倒序）。
// 纯函数，供 ParseDeepSeekCost 与仓库跨月聚合复用（跨月、超 7 天数据不截断）。
// 对应 Kotlin aggregateCost（i in 0 until 30，key = refDate - i 天）。
func AggregateCost(dayMap map[string]float64, refDate time.Time) models.DeepSeekCost {
	ref := normalizeRefDate(refDate)
	var today, d7, d30 float64
	for i := 0; i < 30; i++ {
		key := ref.AddDate(0, 0, -i).Format("2006-01-02")
		v := dayMap[key]
		if i == 0 {
			today = v
		}
		if i < 7 {
			d7 += v
		}
		d30 += v
	}
	days := make([]models.DeepSeekCostDay, 0, len(dayMap))
	for k, v := range dayMap {
		days = append(days, models.DeepSeekCostDay{Date: k, Total: v})
	}
	// sortedByDescending { it.key }
	sort.Slice(days, func(i, j int) bool { return days[i].Date > days[j].Date })
	return models.DeepSeekCost{Today: today, Last7d: d7, Last30d: d30, Days: days}
}

// normalizeRefDate 零值时间表示「今天」（本地时区），并截断到日（不含时分秒）。
func normalizeRefDate(refDate time.Time) time.Time {
	if refDate.IsZero() {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	return time.Date(refDate.Year(), refDate.Month(), refDate.Day(), 0, 0, 0, 0, refDate.Location())
}

// rawInt 宽容解析 JSON 整数（对应 jsonPrimitive.intOrNull）
// rawInt 宽容解析 JSON 整数（对应 jsonPrimitive.intOrNull）。
// 与 rawFloat 同一宽容口径：JSON 字符串形式的整数（如 "1"）同样接受——
// 平台偶发把整型字段序列化成字符串，严格模式会把运行中实例（Status:"1"）
// 解析成 0（已结束），属于静默错误（oracle 双实现对照 P2-1）。
func rawInt(r json.RawMessage) (int, bool) {
	if len(r) == 0 {
		return 0, false
	}
	var i int
	if err := json.Unmarshal(r, &i); err == nil {
		return i, true
	}
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return v, true
		}
	}
	return 0, false
}

// rawFloat 宽容解析 JSON 浮点数，失败兜底 0.0（对应 doubleOrNull ?: 0.0）。
// Kotlin doubleOrNull 对 JSON 字符串形式的数字（如 "0.5"）同样解析为 0.5，这里对齐该语义。
func rawFloat(r json.RawMessage) float64 {
	var f float64
	if err := json.Unmarshal(r, &f); err == nil {
		return f
	}
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	return 0.0
}

// ── Qwen Token Plan：网关模型清单（API Key 认证） ───────────────

// ParseQwenModels 解析 GET /compatible-mode/v1/models 响应，返回排序后的模型 id 列表。
// 空清单视为失败：宁显示错误也不显示误導的空套餐。
func ParseQwenModels(raw string) ([]string, error) {
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("Qwen 模型清单 JSON 解析失败: %w", err)
	}
	seen := map[string]bool{}
	models := make([]string, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, id)
	}
	if len(models) == 0 {
		return nil, errors.New("未获取到 Qwen 可用模型")
	}
	sort.Strings(models)
	return models, nil
}

// truncate rune 安全截断（防切断中文多字节，与 repo.truncate200 同型）
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// SanitizeText 清洗服务器可控文本（错误消息、HTTP 响应体片段）——进终端与
// --json 前必经。剥离 ANSI 转义序列（CSI / OSC / 两字符转义）与除 \n、\t 外
// 的控制字符（含 \r、DEL、C1 区）：上游文本可携带 \x1b[31m 或 \r 伪造终端
// 输出、打断行结构，也可让 --json.error 混入垃圾字节。多行错误依赖 \n 故
// 保留；多字节 UTF-8 按字节态机处理，中文不受影响。momus 2026-09-06 P2：
// 须全仓统一接入而非单点修，避免各 provider 消毒口径不一致。
func SanitizeText(s string) string {
	dirty := false
	for _, r := range s {
		if r == 0x1b || (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			dirty = true
			break
		}
	}
	if !dirty {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c != 0x1b {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size <= 1 {
				i++ // 坏字节丢弃，不进输出
				continue
			}
			if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
				i += size
				continue
			}
			b.WriteString(s[i : i+size])
			i += size
			continue
		}
		// ESC 序列三型：CSI（ESC [ … 终止字节 0x40-0x7e）、OSC（ESC ] … BEL 或 ESC \）、
		// 两字符转义。均有字节上限，防服务器发无终止符序列拖死清洗。
		if i+1 >= len(s) {
			break // 尾部孤 ESC 丢弃
		}
		switch s[i+1] {
		case '[':
			j := i + 2
			for j < len(s) && j-i < 64 && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) && s[j] >= 0x40 && s[j] <= 0x7e {
				i = j + 1
			} else {
				i = j
			}
		case ']':
			j := i + 2
			end := -1
			for j < len(s) && j-i < 256 {
				if s[j] == 0x07 {
					end = j + 1
					break
				}
				if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
					end = j + 2
					break
				}
				j++
			}
			if end < 0 {
				if j > len(s) {
					j = len(s)
				}
				end = j
			}
			i = end
		default:
			i += 2
			// ESC ( B / ESC ) 1 / ESC # 3 等三字符形态：中间字符再吃一个
			if i-1 < len(s) && (s[i-1] == '(' || s[i-1] == ')' || s[i-1] == '#' || s[i-1] == '%') && i < len(s) {
				i++
			}
		}
	}
	return b.String()
}

// ── 白B.AI：网关模型清单（API Key 认证） ─────────────────────

// ParseBaiModels 解析 GET /v1/models 响应（one-api 系信封：data 数组 + 顶层
// success）。id 去重并按 id 排序；空清单/错误信封显式失败。
func ParseBaiModels(raw string) ([]models.BaiModel, error) {
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			ID        string   `json:"id"`
			OwnedBy   string   `json:"owned_by"`
			Endpoints []string `json:"supported_endpoint_types"`
		} `json:"data"`
		// 403 等错误信封（无 error 键）：{"message":"…","success":false}
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("BAI 模型清单 JSON 解析失败: %w", err)
	}
	// 刻意宽松：success=false 但 data 非空时仍接受数据——one-api 网关从未观测到
	// 该形状（实测 403 错误信封不含 data 键，上方分支已覆盖），严格拒绝反而在
	// 未验证形状上误报；若上游未来改版，此处是首个观察点。
	if !payload.Success && len(payload.Data) == 0 {
		if msg := strings.TrimSpace(payload.Message); msg != "" {
			return nil, fmt.Errorf("BAI 网关返回错误: %s", truncate(SanitizeText(msg), 200))
		}
		return nil, errors.New("未获取到 BAI 可用模型")
	}
	seen := map[string]bool{}
	out := make([]models.BaiModel, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, models.BaiModel{ID: id, OwnedBy: strings.TrimSpace(m.OwnedBy), Endpoints: m.Endpoints})
	}
	if len(out) == 0 {
		return nil, errors.New("未获取到 BAI 可用模型")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// int64 与 float64 的交界：float64 恰好能表示 2^63，int64 只能装到 2^63-1，
// 故上界用「严格小于」判定（越界的 float→int64 转换结果由实现定义，amd64 得 MinInt64）。
const (
	int64FloatUpper = 9223372036854775808.0  //  2^63
	int64FloatLower = -9223372036854775808.0 // -2^63
)

// rawInt64 宽容解析 JSON 整数（int64 / 字符串 / 浮点三种形状）。
// tRPC 负载由 JS 序列化，大整数理论上可能以 2.7e7 或 "27166591" 出现，
// 严格模式会静默归零（同 rawInt 的 oracle 对照教训）。
// 越界 / Inf / NaN 一律视为解析失败：放行会把垃圾值伪装成「额度已耗尽」红警。
func rawInt64(r json.RawMessage) (int64, bool) {
	if len(r) == 0 || strings.TrimSpace(string(r)) == "null" {
		return 0, false
	}
	var i int64
	if err := json.Unmarshal(r, &i); err == nil {
		return i, true
	}
	// 非整数字面量：先试浮点（JS 侧 2.7e7 形状），再试字符串里的数字。
	var f float64
	if err := json.Unmarshal(r, &f); err != nil {
		var s string
		if json.Unmarshal(r, &s) != nil {
			return 0, false
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return 0, false
		}
		f = v
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f >= int64FloatUpper || f < int64FloatLower {
		return 0, false
	}
	if f != math.Trunc(f) {
		return 0, false
	}
	return int64(f), true
}

// ── 白B.AI 积分额度（chat.b.ai tRPC，API Key 认证） ────────────────

// ErrBaiAuth BAI 凭据问题的统一口径：推理面（api.b.ai）401/403 与控制台
// tRPC UNAUTHORIZED 共用同一文案，避免两路失败时用户读到两句同义不同词的提示。
var ErrBaiAuth = errors.New("BAI API Key 无效、已过期或额度用尽，请到 chat.b.ai 核对")

// baiEnvelope tRPC v11 单查询信封：成功 {"result":{"data":{"json":…}}}，
// 失败 {"error":{"json":{"message":…, "data":{"code":"UNAUTHORIZED",…}}}}。
type baiEnvelope struct {
	Result struct {
		Data struct {
			JSON json.RawMessage `json:"json"`
		} `json:"data"`
	} `json:"result"`
	Error struct {
		JSON struct {
			Message string `json:"message"`
			Data    struct {
				Code string `json:"code"`
			} `json:"data"`
		} `json:"json"`
	} `json:"error"`
}

// baiPointsPayload usage.points / usage.summary 共用的字段集（两个端点都回
// points_balance，summary 额外回 monthly_spent，points 额外回 points_expiring）。
type baiPointsPayload struct {
	PointsBalance  json.RawMessage `json:"points_balance"`
	PointsExpiring json.RawMessage `json:"points_expiring"`
	MonthlySpent   json.RawMessage `json:"monthly_spent"`
}

// baiEnvelopeOf 拆 tRPC 信封并返回负载原文。错误信封优先：UNAUTHORIZED 归一到
// ErrBaiAuth，其他错误带原文（截 200 字）。points / summary / records 三端点共用。
func baiEnvelopeOf(raw, proc string) (json.RawMessage, error) {
	var env baiEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, fmt.Errorf("BAI %s JSON 解析失败: %w", proc, err)
	}
	if msg := strings.TrimSpace(env.Error.JSON.Message); msg != "" {
		if strings.EqualFold(strings.TrimSpace(env.Error.JSON.Data.Code), "UNAUTHORIZED") {
			return nil, ErrBaiAuth
		}
		return nil, fmt.Errorf("BAI %s 返回错误: %s", proc, truncate(SanitizeText(msg), 200))
	}
	if len(env.Result.Data.JSON) == 0 {
		return nil, fmt.Errorf("未获取到 BAI %s", proc)
	}
	return env.Result.Data.JSON, nil
}

// baiPayloadOf 拆 tRPC 信封并取出积分额度负载。
func baiPayloadOf(raw, proc string) (baiPointsPayload, error) {
	payload, err := baiEnvelopeOf(raw, proc)
	if err != nil {
		return baiPointsPayload{}, err
	}
	var p baiPointsPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return baiPointsPayload{}, fmt.Errorf("BAI %s 负载解析失败: %w", proc, err)
	}
	return p, nil
}

// baiRecordsRow usage.records 单条记录的白名单字段（其余 router_* / meta 未用字段
// 由 JSON 白名单结构体天然忽略；数字形状宽容交给 rawInt64）。
type baiRecordsRow struct {
	ID           string          `json:"id"`
	Model        string          `json:"model"`
	SourceType   string          `json:"source_type"`
	CreatedAt    string          `json:"created_at"`
	InputTokens  json.RawMessage `json:"input_tokens"`
	OutputTokens json.RawMessage `json:"output_tokens"`
	TotalTokens  json.RawMessage `json:"total_tokens"`
	CostPoints   json.RawMessage `json:"cost_points"`
}

// baiRecordsPayload usage.records 负载：data 数组 + 翻页游标。next_cursor 刻意不接
// （page/pageSize 已足够，游标格式属于平台内部实现，不把它当契约）。
type baiRecordsPayload struct {
	Data    json.RawMessage `json:"data"`
	HasMore bool            `json:"has_more"`
}

// ParseBaiRecords 解析 GET /trpc/lambda/usage.records：逐请求明细 + has_more。
// data 键缺席 = 显式失败（不显示 0 条冒充统计）；空数组 = 合法零记录。
func ParseBaiRecords(raw string) ([]models.BaiRecord, bool, error) {
	payload, err := baiEnvelopeOf(raw, "用量明细")
	if err != nil {
		return nil, false, err
	}
	var p baiRecordsPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, false, fmt.Errorf("BAI 用量明细负载解析失败: %w", err)
	}
	if len(p.Data) == 0 || strings.TrimSpace(string(p.Data)) == "null" {
		return nil, false, errors.New("未获取到 BAI 用量明细（响应缺少 data）")
	}
	var rows []baiRecordsRow
	if err := json.Unmarshal(p.Data, &rows); err != nil {
		return nil, false, fmt.Errorf("BAI 用量明细记录解析失败: %w", err)
	}
	recs := make([]models.BaiRecord, 0, len(rows))
	for _, r := range rows {
		rec := models.BaiRecord{
			ID:         r.ID,
			Model:      r.Model,
			SourceType: r.SourceType,
			CreatedAt:  r.CreatedAt,
		}
		rec.InputTokens, _ = rawInt64(r.InputTokens)
		rec.OutputTokens, _ = rawInt64(r.OutputTokens)
		rec.TotalTokens, _ = rawInt64(r.TotalTokens)
		rec.CostPoints, _ = rawInt64(r.CostPoints)
		recs = append(recs, rec)
	}
	return recs, p.HasMore, nil
}

// ParseBaiPoints 解析 GET /trpc/lambda/usage.points。
// points_balance 缺失视为失败（宁可不显示也不显示 0 误導用户「额度用尽」）；
// points_expiring 缺席按 0 处理（无赠送额度时平台就不回该键）。
func ParseBaiPoints(raw string) (models.BaiPoints, error) {
	p, err := baiPayloadOf(raw, "积分额度")
	if err != nil {
		return models.BaiPoints{}, err
	}
	balance, ok := rawInt64(p.PointsBalance)
	if !ok {
		return models.BaiPoints{}, errors.New("未获取到 BAI 积分额度（响应缺少 points_balance）")
	}
	out := models.BaiPoints{Balance: balance}
	if exp, ok := rawInt64(p.PointsExpiring); ok {
		out.Expiring = exp
	}
	return out, nil
}

// ParseBaiMonthlySpent 解析 GET /trpc/lambda/usage.summary 的 monthly_spent（本月已消耗积分）。
func ParseBaiMonthlySpent(raw string) (int64, error) {
	p, err := baiPayloadOf(raw, "本月消耗")
	if err != nil {
		return 0, err
	}
	spent, ok := rawInt64(p.MonthlySpent)
	if !ok {
		return 0, errors.New("未获取到 BAI 本月消耗（响应缺少 monthly_spent）")
	}
	return spent, nil
}

// ── Qwen Token Plan：控制台 RPC（Cookie 认证） ─────────────────
//
// 控制台网关信封形如 {code, data:{DataV2:{ret, data:{code, data:{...}}}}, successResponse}，
// 目标负载深度嵌套且部分层以「JSON 字符串」形式内嵌，因此解析器做两件事：
//  1. 先判错信封（data.success=false / data.errorCode 非空）；
//  2. BFS 遍历对象/数组（含展开形如 JSON 的字符串值），取第一个包含目标键的对象。
//
// 响应形状实测来源：百炼控制台 token-plan/personal/api/v2/usage（2026-08-29 抓包）。

// qwenWalkMaxDepth 内嵌 JSON 展开的最大深度（防御无限嵌套）
const qwenWalkMaxDepth = 12

// qwenFindObject BFS 查找含任一目标键的对象（内嵌 JSON 字符串会被展开后继续遍历）。
func qwenFindObject(node any, wants []string, depth int) (map[string]any, bool) {
	if depth > qwenWalkMaxDepth {
		return nil, false
	}
	switch v := node.(type) {
	case map[string]any:
		for _, want := range wants {
			if _, ok := v[want]; ok {
				return v, true
			}
		}
		for _, child := range v {
			if got, ok := qwenFindObject(child, wants, depth+1); ok {
				return got, true
			}
		}
	case []any:
		for _, child := range v {
			if got, ok := qwenFindObject(child, wants, depth+1); ok {
				return got, true
			}
		}
	case string:
		s := strings.TrimSpace(v)
		if len(s) >= 2 && (s[0] == '{' || s[0] == '[') {
			var inner any
			if err := json.Unmarshal([]byte(s), &inner); err == nil {
				return qwenFindObject(inner, wants, depth+1)
			}
		}
	}
	return nil, false
}

// qwenErrorOf 判错信封：返回可读错误（无错则 nil）。
// 登录类错误（NotLogined / NeedLogin）映射为 Cookie 过期提示，与 Zen billing 同语义。
func qwenErrorOf(raw string) error {
	var env struct {
		Data struct {
			Success   *json.RawMessage `json:"success"`
			ErrorCode string           `json:"errorCode"`
			ErrorMsg  string           `json:"errorMsg"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil // 非标准信封交给后续查找逻辑处理
	}
	code := strings.TrimSpace(env.Data.ErrorCode)
	msg := strings.TrimSpace(env.Data.ErrorMsg)
	if code == "" && msg == "" {
		return nil
	}
	if code == "" {
		code = msg
	}
	low := strings.ToLower(code + " " + msg)
	if strings.Contains(low, "notlogined") || strings.Contains(low, "needlogin") ||
		strings.Contains(low, "login") || strings.Contains(low, "unauthor") {
		return errors.New("控制台 Cookie 已过期或无效，请更新控制台 Cookie")
	}
	return fmt.Errorf("Qwen 控制台接口错误：%s", truncate(SanitizeText(code), 200))
}

// qwenNumber 宽容取数（数字或数字字符串）。
func qwenNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// qwenRatioToWindow 比例值 → 窗口。契约上接口返回 0-1 比例（实测 0.7913113）；
// 若 >1 则视为已是百分数（防御性处理，避免 7913% 与误判限流）。
// Exhausted 由原值达满推导（官方规则：窗口内配额用尽则暂停服务）。
func qwenRatioToWindow(ratio float64, resetsAt any, now time.Time) *models.QwenWindow {
	percent, exhausted := qwenPercent(ratio)
	return &models.QwenWindow{Percent: percent, ResetsAt: qwenResetTime(resetsAt, now), Exhausted: exhausted}
}

// qwenPercent 拆分「百分比 + 是否用尽」。
//
// 接口契约为 0-1 比例（实测 0.7913113）。取值域划分：
//   - ≤ 2：比例域。>1 为超额（配额用尽后网关仍可能给到 1.0x），一律上限 100% + 已限流；
//   - > 2：不可能是比例，视为已是百分数尺度（防御：避免显示 7913% 与误判限流）。
func qwenPercent(ratio float64) (percent int, exhausted bool) {
	if ratio > 2 {
		return clampPercent(int(ratio)), ratio >= 100
	}
	return clampPercent(int(ratio * 100)), ratio >= 1
}

// clampPercent 把百分比限到 0-100（用量条上限）
func clampPercent(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// qwenResetTime 重置时间 → RFC3339。数字按 Unix 毫秒；字符串原样传递
// （解析失败时渲染层降级为「即将重置」）。
func qwenResetTime(v any, now time.Time) string {
	if f, ok := qwenNumber(v); ok {
		return time.UnixMilli(int64(f)).In(now.Location()).Format(time.RFC3339)
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// ParseQwenUsage 解析 tokenplan/personal/api/v2/usage 响应。
// 两个窗口都缺失 → 报错（仓库层会重试，网关偶发返回空信封）。
func ParseQwenUsage(raw string, now time.Time) (models.QwenUsage, error) {
	if err := qwenErrorOf(raw); err != nil {
		return models.QwenUsage{}, err
	}
	var node any
	if err := json.Unmarshal([]byte(raw), &node); err != nil {
		return models.QwenUsage{}, fmt.Errorf("Qwen 用量 JSON 解析失败: %w", err)
	}
	obj, ok := qwenFindObject(node, []string{"per5HourPercentage", "per1WeekPercentage"}, 0)
	if !ok {
		return models.QwenUsage{}, errors.New("Qwen 用量数据暂不可用")
	}
	out := models.QwenUsage{}
	if v, has := obj["per5HourPercentage"]; has {
		if f, ok := qwenNumber(v); ok {
			out.FiveHour = qwenRatioToWindow(f, obj["per5HourResetTime"], now)
		}
	}
	if v, has := obj["per1WeekPercentage"]; has {
		if f, ok := qwenNumber(v); ok {
			out.Weekly = qwenRatioToWindow(f, obj["per1WeekResetTime"], now)
		}
	}
	if out.FiveHour == nil && out.Weekly == nil {
		return models.QwenUsage{}, errors.New("Qwen 用量数据暂不可用")
	}
	return out, nil
}

// ParseQwenSubscription 解析 tokenplan/personal/api/v2/subscription 响应，
// 取套餐档位（lite/standard/pro/max）。找不到档位不是错误（best-effort，返回空串）。
func ParseQwenSubscription(raw string) (string, error) {
	if err := qwenErrorOf(raw); err != nil {
		return "", err
	}
	var node any
	if err := json.Unmarshal([]byte(raw), &node); err != nil {
		return "", fmt.Errorf("Qwen 订阅 JSON 解析失败: %w", err)
	}
	keys := []string{"specCode", "spec_code", "planName", "plan_name", "planCode", "plan_code"}
	obj, ok := qwenFindObject(node, keys, 0)
	if !ok {
		return "", nil
	}
	for _, k := range keys {
		if s, ok := obj[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.ToLower(strings.TrimSpace(s)), nil
		}
	}
	return "", nil
}

// PlanDisplayName 套餐档位 → 展示名（未知那么原样输出）。
func PlanDisplayName(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "":
		return ""
	case "lite":
		return "Lite"
	case "standard":
		return "Standard"
	case "pro":
		return "Pro"
	case "max":
		return "Max"
	default:
		return code
	}
}

// reQwenSECToken 从控制台 HTML 提取 SEC_TOKEN（window.ALIYUN_CONSOLE_CONFIG 内）。
var reQwenSECToken = regexp.MustCompile(`SEC_TOKEN\s*[:=]\s*"([^"]+)"`)

// ExtractQwenSECToken 提取 sec_token；找不到返回空串（网关对部分账号接受无 token 请求）。
func ExtractQwenSECToken(html string) string {
	m := reQwenSECToken.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// ── LongCat（美团龙猫，OpenAI 兼容 /v1/models） ─────────────────

// ParseLongCatModels 解析 GET /openai/v1/models 响应。
// LongCat 响应形状与 OpenAI 一致：{"data":[{"id":"…","owned_by":"…"},…]}。
// 空清单视为失败。
func ParseLongCatModels(raw string) ([]models.LongCatModel, error) {
	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("LongCat 模型清单 JSON 解析失败: %w", err)
	}
	if len(payload.Data) == 0 {
		return nil, errors.New("未获取到 LongCat 可用模型")
	}
	seen := map[string]bool{}
	out := make([]models.LongCatModel, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, models.LongCatModel{ID: id, OwnedBy: strings.TrimSpace(m.OwnedBy)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ErrLongCatAuth LongCat 凭据问题统一口径。401 invalid_api_key / 403 insufficient_quota
// 共用文案（403 响应体不含 key 原文，无需担心泄露）。
var ErrLongCatAuth = errors.New("LongCat API Key 无效或已过期，请到 longcat.chat/platform/api_keys 核对")

// ── GPTZero（AI 检测额度，GET /v2/users/me，x-api-key 认证） ────────

// ErrGptzeroAuth GPTZero 凭据问题的统一口径。api.gptzero.me 对未带 key 回 401
// {"error":"Require valid cookie"}、对坏 key 回 403 {"error":"API key has no owner",
// "apiKey":"<原样回显>"}——回显里带 key，因此错误在 doGet 层归一，原文不上抛。
var ErrGptzeroAuth = errors.New("GPTZero API Key 无效或已过期，请到 app.gptzero.me 的 API 订阅页核对")

// gptzeroTopPayload 信封外层：data 以 RawMessage 拆出，才能区分「缺 data」与
// 「data 内字段缺席」（直接 Unmarshal 嵌套结构体会把缺 data 静默变零值）。
type gptzeroTopPayload struct {
	Data json.RawMessage `json:"data"`
}

// gptzeroDataPayload GET /v2/users/me data 白名单字段。刻意不接 api_key 字段：
// 响应含明文 key，白名单结构体天然丢弃它（渲染层与 --json 都不可能带出）。
type gptzeroDataPayload struct {
	Email              string          `json:"email"`
	Plan               string          `json:"plan"`
	CharLimit          json.RawMessage `json:"char_limit"`
	MonthlyInputWords  json.RawMessage `json:"monthly_input_words"`
	MonthlyInputChars  json.RawMessage `json:"monthly_input_chars"`
	MonthlyInputDocs   json.RawMessage `json:"monthly_input_documents"`
	AllTimeInputWords  json.RawMessage `json:"all_time_input_words"`
	AllTimeInputChars  json.RawMessage `json:"all_time_input_chars"`
	AllTimeInputDocs   json.RawMessage `json:"all_time_input_documents"`
	LastTimeUsageReset string          `json:"last_time_usage_reset"`
	FullPlan           struct {
		Name             string          `json:"name"`
		DurationType     string          `json:"duration_type"`
		WordLimit        json.RawMessage `json:"word_limit"`
		OverageWordLimit json.RawMessage `json:"overage_word_limit"`
		PriceData        struct {
			UnitAmount json.RawMessage `json:"unit_amount"`
		} `json:"priceData"`
	} `json:"full_plan"`
}

// gptzeroCountersOf 三元组装配：words/chars/documents 任一缺席按 0 计
// （缺席语义是「平台没回该键」而非「没用量」，只有 words 才是计费口径必须项，
// 由调用方单独判）。
func gptzeroCountersOf(words, chars, docs json.RawMessage) models.GptzeroCounters {
	var c models.GptzeroCounters
	c.Words, _ = rawInt64(words)
	c.Chars, _ = rawInt64(chars)
	c.Documents, _ = rawInt64(docs)
	return c
}

// int64Of 可选整数：缺席/非法 → 0（仅用于非必须项，必须项由调用方判 ok）。
func int64Of(r json.RawMessage) int64 {
	v, _ := rawInt64(r)
	return v
}

// ParseGptzeroUsage 解析 GET /v2/users/me 响应为额度快照。
// data 缺席 / monthly_input_words 缺席 / full_plan.word_limit 缺席 = 显式失败：
// 额度显示 0 会让用户误以为「没用量/没额度」，宁可不显示。
func ParseGptzeroUsage(raw string) (models.GptzeroUsage, error) {
	var top gptzeroTopPayload
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return models.GptzeroUsage{}, fmt.Errorf("GPTZero 响应 JSON 解析失败: %w", err)
	}
	if len(top.Data) == 0 || strings.TrimSpace(string(top.Data)) == "null" {
		return models.GptzeroUsage{}, errors.New("未获取到 GPTZero 账号数据（响应缺少 data）")
	}
	var d gptzeroDataPayload
	if err := json.Unmarshal(top.Data, &d); err != nil {
		return models.GptzeroUsage{}, fmt.Errorf("GPTZero 账号数据解析失败: %w", err)
	}
	if _, ok := rawInt64(d.MonthlyInputWords); !ok {
		return models.GptzeroUsage{}, errors.New("未获取到 GPTZero 月度用量（响应缺少 monthly_input_words）")
	}
	limit, ok := rawInt64(d.FullPlan.WordLimit)
	if !ok {
		return models.GptzeroUsage{}, errors.New("未获取到 GPTZero 套餐词数上限（响应缺少 full_plan.word_limit）")
	}
	out := models.GptzeroUsage{
		Email:     SanitizeText(strings.TrimSpace(d.Email)),
		PlanName:  SanitizeText(strings.TrimSpace(d.Plan)),
		Monthly:   gptzeroCountersOf(d.MonthlyInputWords, d.MonthlyInputChars, d.MonthlyInputDocs),
		AllTime:   gptzeroCountersOf(d.AllTimeInputWords, d.AllTimeInputChars, d.AllTimeInputDocs),
		CharLimit: int64Of(d.CharLimit),
		LastReset: strings.TrimSpace(d.LastTimeUsageReset),
	}
	out.Plan = models.GptzeroPlan{
		Name:         SanitizeText(strings.TrimSpace(d.FullPlan.Name)),
		DurationType: SanitizeText(strings.TrimSpace(d.FullPlan.DurationType)),
		WordLimit:    limit,
	}
	out.Plan.OverageWordLimit, _ = rawInt64(d.FullPlan.OverageWordLimit)
	out.Plan.PriceCents, _ = rawInt64(d.FullPlan.PriceData.UnitAmount)
	return out, nil
}
