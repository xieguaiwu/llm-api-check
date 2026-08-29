package render

import (
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/app"
	"github.com/xieguiawu/llm-api-check/internal/models"
)

// galaxyResult 构造一个四路齐全的智星云刷新结果（时钟锚定 baseTime）
func galaxyResult() app.GalaxyResult {
	now := baseTime()
	return app.GalaxyResult{
		Account: models.GalaxyAccount{ID: "g1", Name: "训练集群", AccessKey: "ak", SecretKey: "sk"},
		Balance: &models.GalaxyBalance{
			Name: "用户#2433", Phone: "138****1111", Money: 96.2805, PowerMoney: 12.5,
			VipLevel: 2, CustomDiscount: 1,
		},
		Status: &models.GalaxyStatusCount{All: 85, Running: 2, KeeppedDisk: 1, CreateError: 1},
		Instances: []models.GalaxyInstance{
			{Name: "inst-a", Host: "lyg2030", SSHHost: "js4.example.cn", SSHPort: 13024,
				Status: 1, StatusText: "运行中", GpuType: "GeForce RTX 3080", GpuNum: 1,
				CpuNum: 16, MemoryGB: 48, District: "js", TotalCost: 0.87, AutoRenew: true,
				DueAt: now.Add(33 * time.Minute).Format(time.RFC3339)},
			{Name: "inst-b", Host: "lyg0175", SSHHost: "js1.example.cn", SSHPort: 28540,
				Status: 1, StatusText: "运行中", Abnormal: true, GpuType: "CPU",
				CpuNum: 8, MemoryGB: 16, District: "js", TotalCost: 0.325,
				DueAt: now.Add(20 * time.Hour).Format(time.RFC3339)},
			{Name: "inst-c", Host: "lyg0160", SSHHost: "js1.example.cn", SSHPort: 27012,
				Status: 8, StatusText: "磁盘保留", CpuNum: 2, MemoryGB: 2, TotalCost: 0.0009,
				DiskReleaseAt: now.Add(48 * time.Hour).Format(time.RFC3339)},
		},
		Cost: &models.GalaxyCost{Today: 26.25, Last7d: 249.83, WeekPartial: true},
	}
}

func TestRenderGalaxyDetailSections(t *testing.T) {
	got := RenderGalaxyDetail(galaxyResult(), baseTime(), Colorizer{Disabled: true})
	for _, want := range []string{
		"训练集群 · 用户#2433 (智星云)",
		"余额", "¥96.28", "算力券 ¥12.50", "信用额度 ¥0.00",
		"VIP2", "138****1111",
		"今日消耗 ¥26.25", "近7天 ≥¥249.83", "明细未翻完",
		"运行中 2 · 磁盘保留 1 · 启动错误 1",
		"活跃实例（3）",
		"lyg2030", "js4.example.cn:13024",
		"RTX 3080×1", "16核/48G", "¥0.87/时", "自动续费",
		"33分后到期",
		"磁盘保留",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("详情缺少 %q:\n%s", want, got)
		}
	}
}

// TestRenderGalaxyExpiryAlwaysVisible 到期时间恒显；异常徽章与倒计时并存
// （对照 index.md §六：时间信息不得被状态徽章替代）
func TestRenderGalaxyExpiryAlwaysVisible(t *testing.T) {
	got := RenderGalaxyDetail(galaxyResult(), baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got, "运行中·异常") {
		t.Fatalf("IsAbnormal 实例应显示异常徽章:\n%s", got)
	}
	line := ""
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "运行中·异常") {
			// 徽章所在卡片随后两行必须给出倒计时
			idx := strings.Index(got, l)
			line = got[idx:]
		}
	}
	if !strings.Contains(line, "后到期") {
		t.Errorf("异常徽章不得吞掉到期倒计时:\n%s", line)
	}
}

// TestRenderGalaxyExpiredKeepsTime 已到期也要给出「已到期 N」而不是抹掉时间
func TestRenderGalaxyExpiredKeepsTime(t *testing.T) {
	r := galaxyResult()
	r.Instances[0].DueAt = baseTime().Add(-3 * time.Hour).Format(time.RFC3339)
	got := RenderGalaxyDetail(r, baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got, "已到期 3小时") {
		t.Errorf("过期应显示已过期时长:\n%s", got)
	}
}

// TestRenderGalaxyNoLeakedSecrets 🔴 渲染层兜底：任何口令字段都不得出现在输出
func TestRenderGalaxyNoLeakedSecrets(t *testing.T) {
	r := galaxyResult()
	r.Instances[1].Note = "密码 SECRET_PWD_1" // 即便平台把口令塞进 Note，也不该被我们主动脱敏掉——见下
	got := RenderGalaxyDetail(r, baseTime(), Colorizer{Disabled: true})
	for _, probe := range []string{"Init_passwd", "LastInitPasswd", "RdpPasswd", "VncPasswd"} {
		if strings.Contains(got, probe) {
			t.Errorf("渲染输出含口令字段名 %s:\n%s", probe, got)
		}
	}
}

func TestRenderGalaxyWithoutCredentials(t *testing.T) {
	r := galaxyResult()
	r.Account.SecretKey = ""
	got := RenderGalaxyDetail(r, baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got, "未配置 AccessKey/SecretKey") {
		t.Errorf("缺凭据应给操作指引:\n%s", got)
	}
	if strings.Contains(got, "¥96.28") {
		t.Error("缺凭据时不该渲染上一轮的余额数据")
	}
}

func TestRenderGalaxyErrorOnly(t *testing.T) {
	r := galaxyResult()
	r.Balance, r.Status, r.Instances, r.Cost = nil, nil, nil, nil
	r.Error = "AccessKey 无效或已删除，请在控制台「开放API → AccessKey管理」重新创建"
	got := RenderGalaxyDetail(r, baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got, "AccessKey 无效") {
		t.Errorf("错误必须可见:\n%s", got)
	}
}

func TestRenderGalaxyNoActiveInstances(t *testing.T) {
	r := galaxyResult()
	r.Instances = nil
	r.Status.Running = 0
	got := RenderGalaxyDetail(r, baseTime(), Colorizer{Disabled: true})
	if !strings.Contains(got, "无活跃实例") {
		t.Errorf("应明确说明无活跃实例:\n%s", got)
	}
}

// TestWriteGalaxyOverview 总览行：余额 + 运行数 + 最近到期（取最早的那个）
func TestWriteGalaxyOverview(t *testing.T) {
	got := RenderOverview(app.Result{Galaxy: []app.GalaxyResult{galaxyResult()}, LastUpdated: baseTime()}, baseTime(), Colorizer{Disabled: true})
	for _, want := range []string{"智星云 (训练集群)", "余额 ¥96.28", "算力券 ¥12.50", "运行中 2", "最近 33分后到期"} {
		if !strings.Contains(got, want) {
			t.Errorf("总览缺少 %q:\n%s", want, got)
		}
	}
}

func TestGalaxyOverviewLowBalanceRed(t *testing.T) {
	r := galaxyResult()
	r.Balance.Money = 12
	c := Colorizer{}
	got := RenderOverview(app.Result{Galaxy: []app.GalaxyResult{r}}, baseTime(), c)
	if !strings.Contains(got, "\x1b[33m余额 ¥12.00") {
		t.Errorf("余额低于 50 应黄色告警:\n%q", got)
	}
	r.Balance.Money = 0
	got = RenderOverview(app.Result{Galaxy: []app.GalaxyResult{r}}, baseTime(), c)
	if !strings.Contains(got, "\x1b[31m余额 ¥0.00") {
		t.Errorf("余额归零应红色:\n%q", got)
	}
}

// ── 原语 ───────────────────────────────────────────────────────

func TestGalaxySpanShort(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "不足1分"},
		{33 * time.Minute, "33分"},
		{2 * time.Hour, "2小时"},
		{2*time.Hour + 20*time.Minute, "2小时20分"},
		{25 * time.Hour, "25小时"}, // 48 小时内一律用小时，避免「1天1小时」这类难读混排
		{48 * time.Hour, "2天"},
	}
	for _, tc := range cases {
		if got := galaxySpanShort(tc.d); got != tc.want {
			t.Errorf("galaxySpanShort(%v)=%q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestGalaxyUnitPriceTrims(t *testing.T) {
	cases := map[float64]string{0.325: "¥0.325/时", 0.87: "¥0.87/时", 2: "¥2/时", 0: "¥0/时"}
	for v, want := range cases {
		if got := galaxyUnitPrice(v); got != want {
			t.Errorf("galaxyUnitPrice(%v)=%q, want %q", v, got, want)
		}
	}
}

func TestPadToUsesDisplayWidth(t *testing.T) {
	// 中文按 2 列计：4 个汉字 = 8 列 → 补 1 空格到 9
	if got := padTo("今日消耗", 9); got != "今日消耗 " {
		t.Errorf("CJK 对齐错误: %q", got)
	}
	if got := padTo("余额", 9); got != "余额     " { // 4 列 + 5 空格 = 9 列
		t.Errorf("CJK 对齐错误: %q", got)
	}
	if got := padTo("很长很长很长的标签", 4); got != "很长很长很长的标签" {
		t.Errorf("超宽不该截断: %q", got)
	}
	if displayWidth("a１b") != 4 {
		t.Errorf("全角数字应算 2 列，实得 %d", displayWidth("a１b"))
	}
}

func TestGalaxyStatusColor(t *testing.T) {
	if GalaxyStatusColor(1, false) != ColorGreen {
		t.Error("运行中应为绿色")
	}
	if GalaxyStatusColor(1, true) != ColorRed {
		t.Error("运行中但 IsAbnormal!=0 必须红色（限流强制红同规则）")
	}
	if GalaxyStatusColor(-1, false) != ColorRed || GalaxyStatusColor(7, false) != ColorRed {
		t.Error("启动错误/重启失败应红色")
	}
	if GalaxyStatusColor(0, false) != ColorGray {
		t.Error("已结束等终态应为灰")
	}
}
