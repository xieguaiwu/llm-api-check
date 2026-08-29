// 智星云 AI Galaxy（OpenAPI v2）签名与解析。
// 契约与实测取证见 docs/plans/2026-08-29-ai-galaxy-provider.md。
package parsers

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

// ── 签名 ───────────────────────────────────────────────────────

// GalaxyStringToSign 拼接待签名字符串：参数名字典序升序、跳过空值、
// 排除 sign/secret 两个键。与官方 Golang 参考实现逐条对齐。
func GalaxyStringToSign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		if k == "sign" || k == "secret" {
			continue
		}
		if params[k] == "" {
			continue // 空值不参与签名（官方契约明确）
		}
		pairs = append(pairs, k+"="+params[k])
	}
	return strings.Join(pairs, "&")
}

// GalaxySign 计算签名：md5(stringToSign + "&secret=" + SecretKey) 小写 hex。
// secret 为空时不拼尾缀（对齐官方参考实现的 if secret != "" 分支）。
func GalaxySign(params map[string]string, secret string) string {
	s := GalaxyStringToSign(params)
	if secret != "" {
		s += "&secret=" + secret
	}
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ── 实例状态 ───────────────────────────────────────────────────

// galaxyStatusText 平台实例状态码 → 中文（文档「获取自主实例详情」的状态常量表）。
var galaxyStatusText = map[int]string{
	-2: "已退费",
	-1: "启动错误",
	0:  "已结束",
	1:  "运行中",
	4:  "启动中",
	5:  "重启中",
	7:  "重启失败",
	8:  "磁盘保留",
}

// GalaxyStatusText 状态码展示文案（未知码回「未知(N)」，不猜语义）。
func GalaxyStatusText(status int) string {
	if s, ok := galaxyStatusText[status]; ok {
		return s
	}
	return fmt.Sprintf("未知(%d)", status)
}

// GalaxyStatusActive 是否仍占用资源（需要用户关注的状态）。
// 已结束/已退费是终态，展示时降级为灰。
func GalaxyStatusActive(status int) bool {
	switch status {
	case -1, 1, 4, 5, 7, 8:
		return true
	default:
		return false
	}
}

// GalaxyDeadlineUnix 到期时刻（Unix 秒）。平台同响应里带回 ServerTime，
// 用 due-serverTime 折算可完全规避本机时钟偏移（文档要求服务器时间准确，
// 但本机偏移一分钟就会把「33分后到期」显示成「已到期」）。
func GalaxyDeadlineUnix(due, serverTime int64, now time.Time) int64 {
	if due <= 0 {
		return 0
	}
	if serverTime > 0 {
		return now.Unix() + (due - serverTime)
	}
	return due
}

// GalaxyRFC3339 Unix 秒 → 本地时区 RFC3339；0/负数 → 空串（无该时间）。
func GalaxyRFC3339(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}

// MaskPhone 手机号脱敏：183****2433。非 11 位号码只保留前 3 位。
func MaskPhone(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) == 0 {
		return ""
	}
	if len(r) <= 7 {
		return string(r[:1]) + "****"
	}
	return string(r[:3]) + "****" + string(r[len(r)-4:])
}

// ── 主账户信息 ─────────────────────────────────────────────────

// galaxyMissingField RawMessage 是否“缺失或显式 null”（两者都算缺，不把坏数据当 0 展示）
func galaxyMissingField(r json.RawMessage) bool {
	if len(r) == 0 {
		return true
	}
	s := strings.TrimSpace(string(r))
	return s == "null"
}

// ParseGalaxyBalance 解析 /account/get_main_account_info 的 data 节点。
// 数值一律走 rawFloat/rawInt 宽容解析（平台偶发把金额序列化成字符串）。
func ParseGalaxyBalance(raw string) (models.GalaxyBalance, error) {
	var p struct {
		Name             string          `json:"Name"`
		Phone            string          `json:"Phone"`
		Money            json.RawMessage `json:"Money"`
		PowerMoney       json.RawMessage `json:"PowerMoney"`
		CreditMoneyQuota json.RawMessage `json:"CreditMoneyQuota"`
		CustomDiscount   json.RawMessage `json:"CustomDiscount"`
		VipLevel         json.RawMessage `json:"VipLevel"`
		LastLoginTime    json.RawMessage `json:"Last_login_time"`
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return models.GalaxyBalance{}, fmt.Errorf("智星云账户 JSON 解析失败: %w", err)
	}
	if galaxyMissingField(p.Money) {
		return models.GalaxyBalance{}, errors.New("智星云账户响应缺少 Money 字段")
	}
	bal := models.GalaxyBalance{
		Name:             strings.TrimSpace(p.Name),
		Phone:            MaskPhone(p.Phone),
		Money:            rawFloat(p.Money),
		PowerMoney:       rawFloat(p.PowerMoney),
		CreditMoneyQuota: rawFloat(p.CreditMoneyQuota),
		CustomDiscount:   rawFloat(p.CustomDiscount),
		VipLevel:         int(rawFloat(p.VipLevel)),
	}
	if v, ok := rawInt(p.LastLoginTime); ok {
		bal.LastLoginAt = GalaxyRFC3339(int64(v))
	}
	return bal, nil
}

// ── 实例状态统计 ───────────────────────────────────────────────

// ParseGalaxyStatusCount 解析 /instance/get_instance_status_count 的 data 节点。
// statusDefault 刻意不取：实测与列表条数不一致（契约 §2.4）。
func ParseGalaxyStatusCount(raw string) (models.GalaxyStatusCount, error) {
	var p struct {
		All          json.RawMessage `json:"statusAll"`
		Running      json.RawMessage `json:"statusRunning"`
		KeeppedDisk  json.RawMessage `json:"statusKeeppedDisk"`
		CreateError  json.RawMessage `json:"statusCreateError"`
		RunningError json.RawMessage `json:"statusRunningError"`
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return models.GalaxyStatusCount{}, fmt.Errorf("智星云实例统计 JSON 解析失败: %w", err)
	}
	if galaxyMissingField(p.All) && galaxyMissingField(p.Running) {
		return models.GalaxyStatusCount{}, errors.New("智星云实例统计响应为空")
	}
	i := func(r json.RawMessage) int {
		v, ok := rawInt(r)
		if !ok {
			return int(rawFloat(r))
		}
		return v
	}
	return models.GalaxyStatusCount{
		All:          i(p.All),
		Running:      i(p.Running),
		KeeppedDisk:  i(p.KeeppedDisk),
		CreateError:  i(p.CreateError),
		RunningError: i(p.RunningError),
	}, nil
}

// ── 实例列表 ───────────────────────────────────────────────────

// ParseGalaxyInstances 解析 /instance/get_instance_list 的 data 节点。
//
// 🔴 白名单解码：响应含 Init_passwd / LastInitPasswd / RdpPasswd / VncPasswd
// 明文口令，这里只声明要用的字段，其余被 encoding/json 直接丢弃——
// 口令不可能经数据类、--json、UI 或日志外泄。
//
// now 用于 ServerTime 时钟折算到期时刻（测试传固定值保证确定性）。
func ParseGalaxyInstances(raw string, now time.Time) (list []models.GalaxyInstance, total int, hasMore bool, err error) {
	// 先探 list 是否显式 null（与 Android 侧报错口径对齐；null 表示异常响应，
	// 空数组才是正常空态）
	var probe struct {
		List json.RawMessage `json:"list"`
	}
	if perr := json.Unmarshal([]byte(raw), &probe); perr != nil {
		return nil, 0, false, fmt.Errorf("智星云实例列表 JSON 解析失败: %w", perr)
	}
	if len(probe.List) > 0 && strings.TrimSpace(string(probe.List)) == "null" {
		return nil, 0, false, errors.New("智星云实例列表响应异常（list 为 null）")
	}
	var payload struct {
		List []struct {
			ContainerName   string          `json:"Container_name"`
			Note            string          `json:"Note"`
			Status          json.RawMessage `json:"Status"`
			IsAbnormal      json.RawMessage `json:"IsAbnormal"`
			GpuType         string          `json:"Gpu_type"`
			GpuNum          json.RawMessage `json:"Gpu_num"`
			CpuNum          json.RawMessage `json:"Cpu_num"`
			Memory          json.RawMessage `json:"Memory"`
			District        string          `json:"District"`
			Host            string          `json:"Host"`
			Url             string          `json:"Url"`
			SshPort         json.RawMessage `json:"SshPort"`
			Image           string          `json:"Image"`
			ContainerType   string          `json:"ContainerType"`
			DueTime         json.RawMessage `json:"Due_time"`
			DiskReleaseTime json.RawMessage `json:"DiskReleaseTime"`
			ServerTime      json.RawMessage `json:"ServerTime"`
			TotalCost       json.RawMessage `json:"Total_cost"`
			PayTypeFirst    string          `json:"PayTypeFirst"`
			Ctime           json.RawMessage `json:"Ctime"`
			Autorenew       *struct {
				SubscribeStatus   json.RawMessage `json:"SubscribeStatus"`
				CancelSubscribeAt json.RawMessage `json:"CancelSubscribeAt"`
			} `json:"InstanceAutorenew"`
		} `json:"list"`
		TotalCount json.RawMessage `json:"total_count"`
		HasMore    json.RawMessage `json:"has_more"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, 0, false, fmt.Errorf("智星云实例列表 JSON 解析失败: %w", err)
	}
	out := make([]models.GalaxyInstance, 0, len(payload.List))
	for _, it := range payload.List {
		status, _ := rawInt(it.Status)
		due, _ := rawInt(it.DueTime)
		srv, _ := rawInt(it.ServerTime)
		gpuNum, _ := rawInt(it.GpuNum)
		cpuNum, _ := rawInt(it.CpuNum)
		mem, _ := rawInt(it.Memory)
		sshPort, _ := rawInt(it.SshPort)
		abn, _ := rawInt(it.IsAbnormal)
		created, _ := rawInt(it.Ctime)
		diskRel, _ := rawInt(it.DiskReleaseTime)
		in := models.GalaxyInstance{
			Name:          strings.TrimSpace(it.ContainerName),
			Note:          strings.TrimSpace(it.Note),
			Status:        status,
			StatusText:    GalaxyStatusText(status),
			Abnormal:      abn != 0,
			GpuType:       strings.TrimSpace(it.GpuType),
			GpuNum:        gpuNum,
			CpuNum:        cpuNum,
			MemoryGB:      mem,
			District:      strings.TrimSpace(it.District),
			Host:          strings.TrimSpace(it.Host),
			SSHHost:       strings.TrimSpace(it.Url),
			SSHPort:       sshPort,
			Image:         strings.TrimSpace(it.Image),
			Kind:          strings.TrimSpace(it.ContainerType),
			TotalCost:     rawFloat(it.TotalCost),
			PayType:       strings.TrimSpace(it.PayTypeFirst),
			DueAt:         GalaxyRFC3339(GalaxyDeadlineUnix(int64(due), int64(srv), now)),
			DiskReleaseAt: GalaxyRFC3339(int64(diskRel)),
			CreatedAt:     GalaxyRFC3339(int64(created)),
		}
		if it.Autorenew != nil {
			ss, _ := rawInt(it.Autorenew.SubscribeStatus)
			cancelled := len(it.Autorenew.CancelSubscribeAt) > 0 &&
				string(it.Autorenew.CancelSubscribeAt) != "null"
			in.AutoRenew = ss == 1 && !cancelled
		}
		out = append(out, in)
	}
	total, _ = rawInt(payload.TotalCount)
	hasMore, _ = rawBool(payload.HasMore)
	return out, total, hasMore, nil
}

// rawBool 宽容解析 JSON 布尔（非布尔 → false）
func rawBool(r json.RawMessage) (bool, bool) {
	if len(r) == 0 {
		return false, false
	}
	var b bool
	if err := json.Unmarshal(r, &b); err != nil {
		return false, false
	}
	return b, true
}

// ── 余额变更明细 ───────────────────────────────────────────────

// GalaxyChange 单条余额变更（内部结构，保留 time.Time 供聚合）。
type GalaxyChange struct {
	At     time.Time
	Remark string
	Spent  float64 // 正数＝扣费，负数＝返还
	Left   float64
}

// ParseGalaxyChanges 解析 /billing/get_balance_change_list 的 data 节点。
// 消费额 = −(ΔMoney + ΔPower)：实测一条「复制启动」DiffMoney=-0.155 +
// DiffPower=-0.17 合计 0.325，与该实例 Total_cost 精确吻合（两种资金融合计费）。
func ParseGalaxyChanges(raw string) ([]GalaxyChange, bool, error) {
	var payload struct {
		List []struct {
			CreateTime string          `json:"CreateTime"`
			Remark     string          `json:"Remark"`
			DiffMoney  json.RawMessage `json:"DiffMoney"`
			DiffPower  json.RawMessage `json:"DiffPower"`
			MoneyLeft  json.RawMessage `json:"MoneyLeft"`
		} `json:"list"`
		HasMore json.RawMessage `json:"has_more"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, false, fmt.Errorf("智星云余额变更 JSON 解析失败: %w", err)
	}
	out := make([]GalaxyChange, 0, len(payload.List))
	for _, it := range payload.List {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(it.CreateTime), time.Local)
		if err != nil {
			continue // 单条时间格式异常不致命：跳过该条，不整批失败
		}
		out = append(out, GalaxyChange{
			At:     t,
			Remark: strings.TrimSpace(it.Remark),
			Spent:  -(rawFloat(it.DiffMoney) + rawFloat(it.DiffPower)),
			Left:   rawFloat(it.MoneyLeft),
		})
	}
	hasMore, _ := rawBool(payload.HasMore)
	return out, hasMore, nil
}

// AggregateGalaxyCost 聚合今日 / 近 7 天净消耗（近 7 天含今日，与 DeepSeek
// 侧 AggregateCost 同一口径）。明细按时间倒序返回，所以只要看到一条早于窗口
// 下界的记录，该窗口就取完了；否则只能给下限（*Partial=true，渲染层加「≥」）。
func AggregateGalaxyCost(changes []GalaxyChange, hasMore bool, now time.Time) models.GalaxyCost {
	ref := normalizeRefDate(now)
	day7 := ref.AddDate(0, 0, -6)
	var today, week float64
	oldest := ref.AddDate(0, 0, 1) // 哨兵：比今日更晚，保证有数据时会被下调
	for _, c := range changes {
		day := time.Date(c.At.Year(), c.At.Month(), c.At.Day(), 0, 0, 0, 0, c.At.Location())
		if day.Before(oldest) {
			oldest = day
		}
		if c.Spent < 0 {
			continue // 纯返还（充值/退款）不计入消耗
		}
		if !day.Before(ref) {
			today += c.Spent
		}
		if !day.Before(day7) {
			week += c.Spent
		}
	}
	var todayPartial, weekPartial bool
	if hasMore {
		// 还能往前翻：只有已经看到窗口下界之前的记录，才能断定窗口取完
		todayPartial = !oldest.Before(ref)
		weekPartial = !oldest.Before(day7)
	}
	entries := make([]models.GalaxyCostEntry, 0, 5)
	for i, c := range changes {
		if i >= 5 {
			break
		}
		entries = append(entries, models.GalaxyCostEntry{
			Time:   c.At.Format(time.RFC3339),
			Remark: c.Remark,
			Spent:  c.Spent,
			Left:   c.Left,
		})
	}
	return models.GalaxyCost{
		Today:        today,
		Last7d:       week,
		TodayPartial: todayPartial,
		WeekPartial:  weekPartial,
		Entries:      entries,
	}
}
