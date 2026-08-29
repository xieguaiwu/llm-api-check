// Package parsers 三个外部数据源的解析器，逐行对照 Android 版
// com.xieguiawu.apicheckers.data.Parsers.kt 移植。
package parsers

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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
	return fmt.Errorf("Qwen 控制台接口错误：%s", code)
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
