// Package render 终端文本渲染：用量条/倒计时/颜色，对应 Android 版
// HomeScreen.kt / DetailScreen.kt 的信息结构（纯文本版）。
package render

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
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

// RenderOverview 总览：DeepSeek 账号 + OpenCode 账号卡片列表。
func RenderOverview(ds []app.DeepSeekResult, accs []app.AccountResult, lastUpdated time.Time, c Colorizer) string {
	var b strings.Builder
	if lastUpdated.IsZero() {
		b.WriteString("LLM API Check — 尚未更新\n")
	} else {
		fmt.Fprintf(&b, "LLM API Check — 更新于 %s\n", lastUpdated.Format("15:04"))
	}
	if len(ds) == 0 && len(accs) == 0 {
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
// 窗口行尾：正常显示重置倒计时；status=rate-limited → 红「已限流」替代。
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
				suffix = c.Red("已限流")
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

// ── 小工具 ────────────────────────────────────────────────────

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
