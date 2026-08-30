// Package render 终端文本渲染：用量条/倒计时/颜色，对应 Android 版
// HomeScreen.kt / DetailScreen.kt 的信息结构（纯文本版）。
package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// ── 颜色 ───────────────────────────────────────────────────────

// Color 输出颜色分类（对应 Android usageColor 的颜色规则）
type Color int

const (
	ColorBlue Color = iota
	ColorYellow
	ColorRed
	ColorGreen
	ColorGray
)

// Colorizer 颜色渲染：NO_COLOR / --no-color 时返回原文（不加 ANSI）
type Colorizer struct {
	Disabled bool
}

func (c Colorizer) wrap(code, s string) string {
	if c.Disabled {
		return s
	}
	return code + s + "\x1b[0m"
}

// Blue 蓝色
func (c Colorizer) Blue(s string) string { return c.wrap("\x1b[34m", s) }

// Yellow 黄色
func (c Colorizer) Yellow(s string) string { return c.wrap("\x1b[33m", s) }

// Red 红色
func (c Colorizer) Red(s string) string { return c.wrap("\x1b[31m", s) }

// Green 绿色
func (c Colorizer) Green(s string) string { return c.wrap("\x1b[32m", s) }

// Gray 灰色
func (c Colorizer) Gray(s string) string { return c.wrap("\x1b[90m", s) }

// apply 按颜色类别渲染
func (c Colorizer) apply(col Color, s string) string {
	switch col {
	case ColorBlue:
		return c.Blue(s)
	case ColorYellow:
		return c.Yellow(s)
	case ColorRed:
		return c.Red(s)
	case ColorGreen:
		return c.Green(s)
	default:
		return c.Gray(s)
	}
}

// ColorForPercent 用量颜色规则：<70 蓝；70-89 黄；≥90 红；限流强制红
// （对应 HomeScreen.usageColor）
func ColorForPercent(percent int, rateLimited bool) Color {
	switch {
	case rateLimited:
		return ColorRed
	case percent >= 90:
		return ColorRed
	case percent >= 70:
		return ColorYellow
	default:
		return ColorBlue
	}
}

// ── 基础渲染原语 ──────────────────────────────────────────────

// Fmt 金额格式化：两位小数（对应 Android fmt = String.format("%.2f")）
func Fmt(v float64) string { return fmt.Sprintf("%.2f", v) }

// CurrencySymbol 货币符号映射：CNY→¥、USD→$、EUR→€，其他显示代码
// （对应 HomeScreen.currencySymbol）
func CurrencySymbol(code string) string {
	switch strings.ToUpper(code) {
	case "CNY", "RMB":
		return "¥"
	case "USD":
		return "$"
	case "EUR":
		return "€"
	default:
		return code + " "
	}
}

// UsageBar 文本用量条：宽度 width 格，四舍五入填充
// （对应 UsageBar LinearProgressIndicator 的 percent/100f 进度）
func UsageBar(percent int, width int) string {
	p := percent
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	filled := int(math.Round(float64(p) * float64(width) / 100))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// FormatCountdown 重置倒计时（对应 DetailScreen.countdownText）：
// 解析失败 / 已过期 / 不足 1 分钟 → 「即将重置」；
// <60min → 「N分钟后重置」；≥1h → 「N小时M分后重置」（整小时 → 「N小时后重置」）。
func FormatCountdown(resetsAt string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, resetsAt)
	if err != nil {
		return "即将重置"
	}
	dur := t.Sub(now)
	if dur <= 0 {
		return "即将重置"
	}
	mins := int64(dur / time.Minute) // 向下取整，等价 Duration.toMinutes
	switch {
	case mins < 1:
		return "即将重置"
	case mins < 60:
		return fmt.Sprintf("%d分钟后重置", mins)
	default:
		h := mins / 60
		m := mins % 60
		if m == 0 {
			return fmt.Sprintf("%d小时后重置", h)
		}
		return fmt.Sprintf("%d小时%d分后重置", h, m)
	}
}

// ── 总览视图（对应 HomeScreen） ───────────────────────────────

// RenderOverview 总览：DeepSeek / OpenCode / Qwen / 智星云 四类账号卡片列表。
func RenderOverview(res app.Result, now time.Time, c Colorizer) string {
	ds, accs, qwen, galaxy := res.DeepSeek, res.Accounts, res.Qwen, res.Galaxy
	lastUpdated := res.LastUpdated
	var b strings.Builder
	if lastUpdated.IsZero() {
		b.WriteString("LLM API Check — 尚未更新\n")
	} else {
		fmt.Fprintf(&b, "LLM API Check — 更新于 %s\n", lastUpdated.Format("15:04"))
	}
	if len(ds) == 0 && len(accs) == 0 && len(qwen) == 0 && len(galaxy) == 0 {
		b.WriteString("\n未配置任何账号，运行 llm-api-check accounts add --help 添加\n")
		return b.String()
	}
	for _, r := range ds {
		fmt.Fprintf(&b, "\nDeepSeek (%s)\n", r.Account.Name)
		writeDeepSeekOverview(&b, r, c)
	}
	for _, r := range accs {
		fmt.Fprintf(&b, "\nOpenCode (%s)\n", r.Account.Name)
		writeAccountOverview(&b, r, c)
	}
	for _, r := range qwen {
		fmt.Fprintf(&b, "\nQwen (%s)\n", r.Account.Name)
		writeQwenOverview(&b, r, c)
	}
	for _, r := range galaxy {
		fmt.Fprintf(&b, "\n智星云 (%s)\n", r.Account.Name)
		writeGalaxyOverview(&b, r, now, c)
	}
	return b.String()
}

func writeDeepSeekOverview(b *strings.Builder, r app.DeepSeekResult, c Colorizer) {
	if strings.TrimSpace(r.Account.ApiKey) == "" {
		b.WriteString(c.Gray("  未配置 API Key，运行 llm-api-check accounts add --type deepseek --help 添加") + "\n")
		return
	}
	info := firstBalanceInfo(r.Balance)
	if info != nil {
		sym := CurrencySymbol(info.Currency)
		fmt.Fprintf(b, "  余额 %s%s · 充值 %s%s · 赠送 %s%s\n",
			sym, Fmt(info.TotalBalance), sym, Fmt(info.ToppedUpBalance), sym, Fmt(info.GrantedBalance))
		if r.Cost != nil {
			fmt.Fprintf(b, "  今日 ¥%s · 7日 ¥%s · 30日 ¥%s\n",
				Fmt(r.Cost.Today), Fmt(r.Cost.Last7d), Fmt(r.Cost.Last30d))
		}
	} else if r.Error == "" {
		b.WriteString(c.Gray("  暂无数据") + "\n")
	}
	if r.Error != "" {
		b.WriteString(c.Red("  "+r.Error) + "\n")
	}
}

func writeAccountOverview(b *strings.Builder, r app.AccountResult, c Colorizer) {
	goU := r.GoUsage
	if goU != nil {
		var parts []string
		for _, w := range goWindows(goU) {
			rateLimited := w.win.Status == "rate-limited"
			col := ColorForPercent(w.win.Percent, rateLimited)
			seg := fmt.Sprintf("%s %d%% %s", w.label, w.win.Percent, UsageBar(w.win.Percent, 10))
			if rateLimited {
				// 整段已是红色（ColorForPercent 对 rate-limited 强制红），不再嵌套上色
				seg += " 已限流"
			}
			parts = append(parts, c.apply(col, seg))
		}
		if len(parts) > 0 {
			b.WriteString("  " + strings.Join(parts, " · ") + "\n")
		}
	}
	if r.ZenBilling != nil {
		fmt.Fprintf(b, "  Zen $%s\n", Fmt(r.ZenBilling.BalanceUsd))
	}
	if r.Error != "" {
		b.WriteString(c.Red("  "+r.Error) + "\n")
	} else if goU == nil {
		b.WriteString(c.Gray("  暂无数据") + "\n")
	}
}

// ── OpenCode 账号详情（对应 DetailScreen） ────────────────────

// RenderAccountDetail OpenCode 账号详情：Go 三窗口 + Zen 账单 + 错误卡。
// 窗口行尾：始终显示重置倒计时（限流时限）；status=rate-limited → 红「已限流」徽章与倒计时并存
// （对照 Android DetailScreen.WindowRow：CountdownText 恒显 + 已限流徽章）。
func RenderAccountDetail(r app.AccountResult, now time.Time, c Colorizer) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (OpenCode)\n", r.Account.Name)
	b.WriteString("Go Plan · 订阅\n")
	goU := r.GoUsage
	if goU == nil {
		b.WriteString(c.Gray("  暂无数据") + "\n")
	} else {
		rows := []struct {
			label string
			win   *models.GoWindow
		}{
			{"Rolling 5h", goU.Rolling},
			{"Weekly 7d", goU.Weekly},
			{"Monthly 30d", goU.Monthly},
		}
		for _, row := range rows {
			if row.win == nil {
				continue
			}
			rateLimited := row.win.Status == "rate-limited"
			col := ColorForPercent(row.win.Percent, rateLimited)
			pct := c.apply(col, fmt.Sprintf("%d%%", row.win.Percent))
			suffix := FormatCountdown(row.win.ResetsAt, now)
			if rateLimited {
				// 限流时限必须直接可见：已限流徽章 + 重置倒计时并存（对照 Android WindowRow）
				suffix = c.Red("已限流") + " · " + suffix
			}
			fmt.Fprintf(&b, "  %-12s %s %s · %s\n", row.label, UsageBar(row.win.Percent, 10), pct, suffix)
		}
	}
	b.WriteString("Zen Plan · 按量\n")
	switch {
	case !r.Account.HasZen():
		b.WriteString(c.Gray("  未配置 Workspace ID / Cookie") + "\n")
	case r.ZenBilling == nil:
		b.WriteString(c.Gray("  暂无数据") + "\n")
	default:
		z := r.ZenBilling
		fmt.Fprintf(&b, "  余额 $%s\n", Fmt(z.BalanceUsd))
		fmt.Fprintf(&b, "  本月 $%s / $%s", Fmt(z.MonthlyUsageUsd), Fmt(z.MonthlyLimitUsd))
		if z.MonthlyLimitUsd > 0 {
			pct := int(z.MonthlyUsageUsd / z.MonthlyLimitUsd * 100) // 对应 Kotlin toInt() 截断
			col := ColorForPercent(pct, false)
			fmt.Fprintf(&b, " %s %s", UsageBar(pct, 10), c.apply(col, fmt.Sprintf("%d%%", pct)))
		}
		b.WriteString("\n")
		if z.AutoReload {
			fmt.Fprintf(&b, "  自动充值 开 · 低于 $%s 充 $%s\n",
				Fmt(z.ReloadTriggerUsd), Fmt(z.ReloadAmountUsd))
		} else {
			b.WriteString("  自动充值 关\n")
		}
	}
	if r.Error != "" {
		b.WriteString(c.Red(r.Error) + "\n")
	}
	return b.String()
}

// ── DeepSeek 账号详情 ─────────────────────────────────────────

// RenderDeepSeekDetail DeepSeek 账号详情：余额大字 + 充值/赠送 + 消费明细。
func RenderDeepSeekDetail(r app.DeepSeekResult, c Colorizer) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (DeepSeek)\n", r.Account.Name)
	if strings.TrimSpace(r.Account.ApiKey) == "" {
		b.WriteString(c.Gray("  未配置 API Key，运行 llm-api-check accounts add --type deepseek --help 添加") + "\n")
		return b.String()
	}
	info := firstBalanceInfo(r.Balance)
	if info != nil {
		sym := CurrencySymbol(info.Currency)
		fmt.Fprintf(&b, "  余额 %s%s\n", sym, Fmt(info.TotalBalance))
		fmt.Fprintf(&b, "  充值 %s%s · 赠送 %s%s\n", sym, Fmt(info.ToppedUpBalance), sym, Fmt(info.GrantedBalance))
	} else if r.Error == "" {
		b.WriteString(c.Gray("  暂无数据") + "\n")
	}
	if r.Cost != nil {
		fmt.Fprintf(&b, "  今日 ¥%s · 7日 ¥%s · 30日 ¥%s\n",
			Fmt(r.Cost.Today), Fmt(r.Cost.Last7d), Fmt(r.Cost.Last30d))
	} else if !r.Account.HasToken() && r.Error == "" {
		b.WriteString(c.Gray("  未配置平台 Token，仅显示余额") + "\n")
	}
	if r.Error != "" {
		b.WriteString(c.Red(r.Error) + "\n")
	}
	return b.String()
}

// ── 智星云 AI Galaxy ────────────────────────────────────

// galaxySpanShort 时长短语：不足 1分 / N分 / N小时M分 / N天M小时
func galaxySpanShort(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "不足1分"
	case d < time.Hour:
		return fmt.Sprintf("%d分", int64(d/time.Minute))
	case d < 48*time.Hour:
		m := int64(d/time.Minute) % 60
		h := int64(d / time.Hour)
		if m == 0 {
			return fmt.Sprintf("%d小时", h)
		}
		return fmt.Sprintf("%d小时%d分", h, m)
	default:
		h := int64(d.Hours()) % 24
		day := int64(d.Hours()) / 24
		if h == 0 {
			return fmt.Sprintf("%d天", day)
		}
		return fmt.Sprintf("%d天%d小时", day, h)
	}
}

// galaxyExpiryText 到期文案 + 紧急色。时间信息恒显：过期不抹掉时间，
// 而是给「已到期 N」（同 §六「已限流徐章与重置倒计时并存」的口径）。
func galaxyExpiryText(dueAt string, now time.Time) (string, Color) {
	if strings.TrimSpace(dueAt) == "" {
		return "无到期信息", ColorGray
	}
	t, err := time.Parse(time.RFC3339, dueAt)
	if err != nil {
		return "到期时间未知", ColorGray
	}
	d := t.Sub(now)
	switch {
	case d <= 0:
		return fmt.Sprintf("已到期 %s", galaxySpanShort(-d)), ColorRed
	case d < 30*time.Minute:
		return galaxySpanShort(d) + "后到期", ColorRed
	case d < 2*time.Hour:
		return galaxySpanShort(d) + "后到期", ColorYellow
	default:
		return galaxySpanShort(d) + "后到期", ColorBlue
	}
}

// GalaxyStatusColor 实例状态色（运行异常强制红，对照 ColorForPercent 的限流强制红）
func GalaxyStatusColor(status int, abnormal bool) Color {
	if abnormal && (status == 1 || status == 4 || status == 5) {
		return ColorRed
	}
	switch status {
	case 1:
		return ColorGreen
	case 4, 5:
		return ColorYellow
	case -1, 7:
		return ColorRed
	case 8:
		return ColorBlue
	default:
		return ColorGray
	}
}

// galaxyGpuLabel GPU 型号简写（去厂商前缀）；无卡实例回「CPU 实例」
func galaxyGpuLabel(in models.GalaxyInstance) string {
	if in.GpuNum <= 0 {
		return "CPU 实例"
	}
	g := strings.TrimSpace(in.GpuType)
	for _, pre := range []string{"GeForce ", "NVIDIA ", "Tesla "} {
		g = strings.TrimPrefix(g, pre)
	}
	return fmt.Sprintf("%s×%d", g, in.GpuNum)
}

// galaxyUnitPrice 时价文本（保留三位小数并去尾零：0.325 / 0.87）
func galaxyUnitPrice(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		s = "0"
	}
	return "¥" + s + "/时"
}

func writeGalaxyOverview(b *strings.Builder, r app.GalaxyResult, now time.Time, c Colorizer) {
	if strings.TrimSpace(r.Account.AccessKey) == "" || strings.TrimSpace(r.Account.SecretKey) == "" {
		b.WriteString(c.Gray("  未配置 AccessKey/SecretKey，运行 llm-api-check accounts add --type galaxy --help 添加") + "\n")
		if r.Error != "" {
			b.WriteString(c.Red("  "+r.Error) + "\n")
		}
		return
	}
	var parts []string
	if r.Balance != nil {
		col := ColorBlue
		if r.Balance.Money <= 0 {
			col = ColorRed
		} else if r.Balance.Money < 50 {
			col = ColorYellow
		}
		parts = append(parts, c.apply(col, fmt.Sprintf("余额 ¥%s", Fmt(r.Balance.Money))))
		if r.Balance.PowerMoney > 0 {
			parts = append(parts, fmt.Sprintf("算力券 ¥%s", Fmt(r.Balance.PowerMoney)))
		}
	}
	if r.Status != nil {
		seg := fmt.Sprintf("运行中 %d", r.Status.Running)
		if r.Status.CreateError > 0 {
			seg = c.Red(seg + fmt.Sprintf(" · 启动错误 %d", r.Status.CreateError))
		}
		parts = append(parts, seg)
		if r.Status.KeeppedDisk > 0 {
			parts = append(parts, fmt.Sprintf("磁盘保留 %d", r.Status.KeeppedDisk))
		}
	}
	if next, ok := galaxyNextExpiry(r.Instances); ok {
		txt, col := galaxyExpiryText(next, now)
		parts = append(parts, c.apply(col, "最近 "+txt))
	}
	if len(parts) > 0 {
		b.WriteString("  " + strings.Join(parts, " · ") + "\n")
	}
	if r.Error != "" {
		b.WriteString(c.Red("  "+r.Error) + "\n")
	}
	if len(parts) == 0 && r.Error == "" {
		b.WriteString(c.Gray("  暂无数据") + "\n")
	}
}

// galaxyNextExpiry 活跃实例里最早到期的那一个的到期时间
func galaxyNextExpiry(list []models.GalaxyInstance) (string, bool) {
	best := ""
	var bestT time.Time
	for _, in := range list {
		if !parsers.GalaxyStatusActive(in.Status) || strings.TrimSpace(in.DueAt) == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, in.DueAt)
		if err != nil {
			continue
		}
		if best == "" || t.Before(bestT) {
			best, bestT = in.DueAt, t
		}
	}
	return best, best != ""
}

// RenderGalaxyDetail 智星云账号详情：余额三列 + 消耗 + 实例统计 + 活跃实例列表。
func RenderGalaxyDetail(r app.GalaxyResult, now time.Time, c Colorizer) string {
	var b strings.Builder
	owner := strings.TrimSpace(r.Account.Name)
	if r.Balance != nil && strings.TrimSpace(r.Balance.Name) != "" {
		owner = fmt.Sprintf("%s · %s", r.Account.Name, r.Balance.Name)
	}
	fmt.Fprintf(&b, "%s (智星云)\n", owner)
	if strings.TrimSpace(r.Account.AccessKey) == "" || strings.TrimSpace(r.Account.SecretKey) == "" {
		b.WriteString(c.Gray("  未配置 AccessKey/SecretKey，运行 llm-api-check accounts add --type galaxy --help 添加") + "\n")
		if r.Error != "" {
			b.WriteString(c.Red(r.Error) + "\n")
		}
		return b.String()
	}
	if r.Balance != nil {
		bal := r.Balance
		col := ColorBlue
		if bal.Money <= 0 {
			col = ColorRed
		} else if bal.Money < 50 {
			col = ColorYellow
		}
		fmt.Fprintf(&b, "  %s%s · 算力券 ¥%s · 信用额度 ¥%s\n",
			padTo("余额", 9), c.apply(col, "¥"+Fmt(bal.Money)), Fmt(bal.PowerMoney), Fmt(bal.CreditMoneyQuota))
		meta := fmt.Sprintf("VIP%d", bal.VipLevel)
		if bal.CustomDiscount > 0 && bal.CustomDiscount < 1 {
			meta += fmt.Sprintf(" · 折扣 %.2f", bal.CustomDiscount)
		}
		if bal.Phone != "" {
			meta += " · " + bal.Phone
		}
		fmt.Fprintf(&b, "  %s%s\n", padTo("账户", 9), meta)
	} else if r.Error == "" {
		b.WriteString(c.Gray("  "+padTo("余额", 9)+"暂无数据") + "\n")
	}
	if r.Cost != nil {
		// 明细没翻完的窗口只当「至少这么多」：数字前加 ≥，不让下限冒充精确值
		today, week := "¥"+Fmt(r.Cost.Today), "¥"+Fmt(r.Cost.Last7d)
		if r.Cost.TodayPartial {
			today = "≥" + today
		}
		if r.Cost.WeekPartial {
			week = "≥" + week
		}
		line := fmt.Sprintf("  %s%s · 近7天 %s", padTo("今日消耗", 9), today, week)
		if r.Cost.TodayPartial || r.Cost.WeekPartial {
			line += c.Gray("（明细未翻完）")
		}
		b.WriteString(line + "\n")
	}
	if r.Status != nil {
		s := r.Status
		fmt.Fprintf(&b, "  %s运行中 %d · 磁盘保留 %d · 启动错误 %d · 运行异常 %d · 全部 %d\n",
			padTo("实例", 9), s.Running, s.KeeppedDisk, s.CreateError, s.RunningError, s.All)
	}
	if hourly := r.HourlyCost(); hourly > 0 && r.Balance != nil {
		fund := r.Balance.Money + r.Balance.PowerMoney
		var runway string
		switch {
		case fund <= 0:
			runway = c.Red("余额不足")
		case fund/hourly < 24:
			runway = c.Yellow(fmt.Sprintf("约可支撑 %s", galaxySpanShort(time.Duration(fund/hourly*float64(time.Hour)))))
		default:
			runway = fmt.Sprintf("约 %d 天", int(fund/hourly/24))
		}
		fmt.Fprintf(&b, "  %s%s · %s\n", padTo("时价", 9), galaxyUnitPrice(hourly), runway)
	}
	if len(r.Instances) > 0 {
		fmt.Fprintf(&b, "\n  活跃实例（%d）\n", len(r.Instances))
		for i, in := range r.Instances {
			idx := fmt.Sprintf("%d)", i+1)
			label := in.Host
			if strings.TrimSpace(label) == "" {
				label = in.Name
			}
			endpoint := ""
			if in.SSHHost != "" || in.SSHPort > 0 {
				endpoint = fmt.Sprintf("  %s:%d", in.SSHHost, in.SSHPort)
			}
			fmt.Fprintf(&b, "  %s %s%s\n", idx, label, endpoint)
			col := GalaxyStatusColor(in.Status, in.Abnormal)
			badge := c.apply(col, in.StatusText)
			if in.Abnormal && (in.Status == 1 || in.Status == 4 || in.Status == 5) {
				badge = c.Red(in.StatusText + "·异常")
			}
			detail := []string{badge, galaxyGpuLabel(in), fmt.Sprintf("%d核/%dG", in.CpuNum, in.MemoryGB)}
			if in.District != "" {
				detail = append(detail, in.District)
			}
			if in.TotalCost > 0 {
				detail = append(detail, galaxyUnitPrice(in.TotalCost))
			}
			if in.AutoRenew {
				detail = append(detail, "自动续费")
			}
			if in.Note != "" {
				detail = append(detail, in.Note)
			}
			b.WriteString("     " + strings.Join(detail, " · ") + "\n")
			expText, expCol := galaxyExpiryText(in.DueAt, now)
			line := "     " + c.apply(expCol, expText)
			if t, err := time.Parse(time.RFC3339, in.DueAt); err == nil {
				line += c.Gray(fmt.Sprintf("（%s）", t.Format("01-02 15:04")))
			}
			if in.Status == 8 && in.DiskReleaseAt != "" {
				if t, err := time.Parse(time.RFC3339, in.DiskReleaseAt); err == nil {
					line += c.Gray(fmt.Sprintf(" · 磁盘 %s 释放", t.Format("01-02 15:04")))
				}
			}
			b.WriteString(line + "\n")
		}
	} else if r.Error == "" && r.Status != nil && r.Status.Running == 0 {
		b.WriteString(c.Gray("\n  无活跃实例") + "\n")
	}
	if r.Error != "" {
		b.WriteString(c.Red(r.Error) + "\n")
	}
	return b.String()
}

// ── 小工具 ────────────────────────────────────────────

// runeWidth 东亚宽字符按 2 列计（CJK 汉字、全角标点、假名、韩文音节）。
func runeWidth(r rune) int {
	switch {
	case r < 0x1100:
		return 1
	case r <= 0x115F, r >= 0x2E80 && r <= 0xA4CF && r != 0x303F,
		r >= 0xAC00 && r <= 0xD7A3, r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE6F, r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6:
		return 2
	default:
		return 1
	}
}

// displayWidth 字符串终端显示宽度（中文占两列）
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// padTo 按显示宽度右对齐补空格（%-10s 按 rune 计数，中英混排会错位）
func padTo(s string, width int) string {
	d := displayWidth(s)
	if d >= width {
		return s
	}
	return s + strings.Repeat(" ", width-d)
}

func firstBalanceInfo(b *models.DeepSeekBalance) *models.DeepSeekBalanceInfo {
	if b == nil || len(b.Infos) == 0 {
		return nil
	}
	return &b.Infos[0]
}

// goWindows 按 R/W/M 顺序收集非空窗口（对应 AccountCard 的 listOfNotNull）
type windowEntry struct {
	label string
	win   *models.GoWindow
}

func goWindows(u *models.GoUsage) []windowEntry {
	var ws []windowEntry
	if u.Rolling != nil {
		ws = append(ws, windowEntry{"R", u.Rolling})
	}
	if u.Weekly != nil {
		ws = append(ws, windowEntry{"W", u.Weekly})
	}
	if u.Monthly != nil {
		ws = append(ws, windowEntry{"M", u.Monthly})
	}
	return ws
}

// ── Qwen 总览（对应 HomeScreen 的 Qwen 卡片） ─────────────────

// qwenWindowEntry 带标签的 Qwen 窗口
type qwenWindowEntry struct {
	label string
	win   *models.QwenWindow
}

// qwenWindows 按 5 小时 / 7 天顺序收集非空窗口
// renderQwenStats 渲染用量分析段：周期 + 调用汇总 + token 统计 + 免费额度。
// 免费额度只列已用过的模型（剩余 < 100%），全未用则一句提示。
func renderQwenStats(b *strings.Builder, s *models.QwenSummary, c Colorizer) {
	b.WriteString("  用量分析\n")
	fmt.Fprintf(b, "  %-14s %s ~ %s（%d 天）\n", "周期", s.Period.Start, s.Period.End, s.Period.Days)
	fmt.Fprintf(b, "  %-14s %d 个模型 · %d 次成功调用\n", "调用", s.Usage.ModelsCalled, s.Usage.SuccessfulCalls)
	for _, u := range s.Usage.Usages {
		fmt.Fprintf(b, "  %-14s %s %s\n", u.Label, formatInt(u.Value), u.Unit)
	}
	used := 0
	for _, f := range s.FreeTier {
		if f.RemainingPercent < 100 {
			used++
		}
	}
	if used == 0 {
		b.WriteString(c.Gray("  免费额度       未使用") + "\n")
		return
	}
	b.WriteString("  免费额度\n")
	for _, f := range s.FreeTier {
		if f.RemainingPercent >= 100 {
			continue
		}
		usedPct := int(100 - f.RemainingPercent)
		col := ColorForPercent(usedPct, usedPct >= 100)
		pct := c.apply(col, fmt.Sprintf("%.1f%%", f.RemainingPercent))
		fmt.Fprintf(b, "  %-22s %s %s/%s · %s · %s 到期\n",
			f.Model, pct, formatInt(f.Remaining), formatInt(f.Total), f.Type, f.Expires)
	}
}

// formatInt 千分位格式化整数（如 14785 → 14,785）。
func formatInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:]
	}
	var out []byte
	for i, ch := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, ch)
	}
	if n < 0 {
		return "-" + string(out)
	}
	return string(out)
}

func qwenWindows(u *models.QwenUsage) []qwenWindowEntry {
	var ws []qwenWindowEntry
	if u.FiveHour != nil {
		ws = append(ws, qwenWindowEntry{"5小时", u.FiveHour})
	}
	if u.Weekly != nil {
		ws = append(ws, qwenWindowEntry{"7天", u.Weekly})
	}
	return ws
}

// RegionDisplayName 区域展示名（委托 models，保证 CLI/UI 与 Android 同源）
func RegionDisplayName(region string) string { return models.QwenRegionDisplayName(region) }

func writeQwenOverview(b *strings.Builder, r app.QwenResult, c Colorizer) {
	if strings.TrimSpace(r.Account.ApiKey) == "" {
		b.WriteString(c.Gray("  未配置 API Key，运行 llm-api-check accounts add --type qwen --help 添加") + "\n")
		if r.Error != "" {
			b.WriteString(c.Red("  "+r.Error) + "\n")
		}
		return
	}
	if r.Usage != nil {
		var parts []string
		for _, w := range qwenWindows(r.Usage) {
			col := ColorForPercent(w.win.Percent, w.win.Exhausted)
			seg := fmt.Sprintf("%s %d%% %s", w.label, w.win.Percent, UsageBar(w.win.Percent, 10))
			if w.win.Exhausted {
				// 整段已是红色（rate-limited 强制红），不再嵌套上色
				seg += " 已限流"
			}
			parts = append(parts, c.apply(col, seg))
		}
		if len(parts) > 0 {
			b.WriteString("  " + strings.Join(parts, " · ") + "\n")
		}
	}
	if plan := planSummary(r); plan != "" {
		b.WriteString("  " + plan + "\n")
	}
	if r.Error != "" {
		b.WriteString(c.Red("  "+r.Error) + "\n")
	} else if r.Usage == nil && r.Plan == nil {
		b.WriteString(c.Gray("  暂无数据") + "\n")
	}
}

// planSummary 套餐行：档位（未知则省略）+ 可用模型数
func planSummary(r app.QwenResult) string {
	plan := parsers.PlanDisplayName(usagePlanCode(r))
	count := 0
	if r.Plan != nil {
		count = len(r.Plan.Models)
	}
	switch {
	case plan != "" && count > 0:
		return fmt.Sprintf("套餐 %s · 模型 %d 个", plan, count)
	case count > 0:
		return fmt.Sprintf("模型 %d 个", count)
	case plan != "":
		return "套餐 " + plan
	default:
		return ""
	}
}

// usagePlanCode 取配额响应里的套餐档位（未拉到则为空）
func usagePlanCode(r app.QwenResult) string {
	if r.Usage == nil {
		return ""
	}
	return r.Usage.PlanCode
}

// ── Qwen 账号详情（对应 DetailScreen 的 Qwen 页） ──────────────

// qwenLoginCmd 登录命令：用 app 层传入的完整命令（含真实路径），无则退为裸名（仍不用 bl）。
func qwenLoginCmd(r app.QwenResult) string {
	if s := strings.TrimSpace(r.CLILoginCmd); s != "" {
		return s
	}
	return "bailian auth login --console"
}

// RenderQwenDetail Qwen 账号详情：套餐/模型 + 5小时/7天 配额窗口。
// 窗口行尾同 OpenCode：重置倒计时恒显，配额用尽时「已限流」徽章与倒计时并存
// （限流时限直接可见，见 ~/prompt_boilerplates/Coding/index.md §六）。
func RenderQwenDetail(r app.QwenResult, now time.Time, c Colorizer) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (Qwen · %s)\n", r.Account.Name, RegionDisplayName(r.Account.Region))
	if strings.TrimSpace(r.Account.ApiKey) == "" {
		b.WriteString(c.Gray("  未配置 API Key，运行 llm-api-check accounts add --type qwen --help 添加") + "\n")
		if r.Error != "" {
			b.WriteString(c.Red(r.Error) + "\n")
		}
		return b.String()
	}
	b.WriteString("Token Plan · 订阅\n")
	if tier := parsers.PlanDisplayName(usagePlanCode(r)); tier != "" {
		fmt.Fprintf(&b, "  %-12s %s\n", "套餐", tier)
	}
	if r.Usage == nil {
		if r.Account.HasCookie() || r.CLIEnabled {
			b.WriteString(c.Gray("  配额窗口       暂无数据") + "\n")
		} else {
			// 未探测到 CLI：不能只说「运行 bailian auth login --console」——
			// 默认独立 prefix 安装下 bailian 不在 PATH，照做会 command not found
			b.WriteString(c.Gray("  配额窗口       需控制台 Cookie 或 Bailian CLI") + "\n")
			if inst := strings.TrimSpace(r.CLIInstallCmd); inst != "" {
				b.WriteString(c.Gray("               安装："+inst) + "\n")
			}
			b.WriteString(c.Gray("               登录："+qwenLoginCmd(r)) + "\n")
		}
	} else {
		for _, w := range qwenWindows(r.Usage) {
			col := ColorForPercent(w.win.Percent, w.win.Exhausted)
			pct := c.apply(col, fmt.Sprintf("%d%%", w.win.Percent))
			suffix := FormatCountdown(w.win.ResetsAt, now)
			if w.win.Exhausted {
				// 限流时限直接可见：已限流徽章与重置倒计时并存（对照 Android WindowRow）
				suffix = c.Red("已限流") + " · " + suffix
			}
			fmt.Fprintf(&b, "  %-12s %s %s · %s\n", w.label, UsageBar(w.win.Percent, 10), pct, suffix)
		}
	}
	if r.Stats != nil {
		renderQwenStats(&b, r.Stats, c)
	}
	if r.Plan != nil && len(r.Plan.Models) > 0 {
		fmt.Fprintf(&b, "  %-12s %d 个：%s\n", "模型", len(r.Plan.Models), strings.Join(r.Plan.Models, ", "))
	}
	if r.Error != "" {
		b.WriteString(c.Red(r.Error) + "\n")
	}
	return b.String()
}
