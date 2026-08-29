package parsers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

func galaxyFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// ── 签名 ───────────────────────────────────────────────────────

// TestGalaxyStringToSign 官方文档「接口签名」给出的 stringA 逐字比对：
// 字典序、空值剔除、sign/secret 排除。
func TestGalaxyStringToSign(t *testing.T) {
	params := map[string]string{
		"apikey":    "8b90cf872569460a",
		"nonce":     "CrpsYHp",
		"param1":    "iamcoolman",
		"param2":    "18",
		"param3":    "true",
		"param4":    "", // 空值不参与签名
		"timestamp": "1733814154",
		"sign":      "should-be-ignored",
	}
	want := "apikey=8b90cf872569460a&nonce=CrpsYHp&param1=iamcoolman&param2=18&param3=true&timestamp=1733814154"
	if got := GalaxyStringToSign(params); got != want {
		t.Errorf("待签名字符串不符:\n got %s\nwant %s", got, want)
	}
}

// TestGalaxySign 已知向量（MD5 由 Python 独立实现交叉验证）。
func TestGalaxySign(t *testing.T) {
	params := map[string]string{
		"apikey":    "8b90cf872569460a",
		"nonce":     "CrpsYHp",
		"param1":    "iamcoolman",
		"param2":    "18",
		"param3":    "true",
		"param4":    "",
		"timestamp": "1733814154",
		"sign":      "should-be-ignored",
	}
	const want = "883c5a86f9dab614490c6021da5f531c"
	if got := GalaxySign(params, "testsecretkey"); got != want {
		t.Errorf("签名不符: got %s want %s", got, want)
	}
}

// TestGalaxySignEmptySecret secret 为空时不拼尾缀（对齐官方参考实现的 if secret != "" 分支）
func TestGalaxySignEmptySecret(t *testing.T) {
	params := map[string]string{"apikey": "a"}
	if got, want := GalaxySign(params, ""), "58bd93007b141c0164c435502bed759b"; got != want {
		t.Errorf("空 secret 应只对参数取 MD5: got %s want %s", got, want)
	}
	if got, want := GalaxySign(params, "s"), "7711ef1481c3c6b3f0adfcc1af027d21"; got != want {
		t.Errorf("非空 secret 应末尾拼 &secret= 后取 MD5: got %s want %s", got, want)
	}
}

// ── 状态与时间 ─────────────────────────────────────────────────

func TestGalaxyStatusText(t *testing.T) {
	for status, want := range map[int]string{
		-2: "已退费", -1: "启动错误", 0: "已结束", 1: "运行中",
		4: "启动中", 5: "重启中", 7: "重启失败", 8: "磁盘保留",
	} {
		if got := GalaxyStatusText(status); got != want {
			t.Errorf("Status=%d 文案 %q, want %q", status, got, want)
		}
	}
	if got := GalaxyStatusText(42); got != "未知(42)" {
		t.Errorf("未知状态应回显码值不猜语义: %q", got)
	}
	if !GalaxyStatusActive(8) || GalaxyStatusActive(0) || GalaxyStatusActive(-2) {
		t.Error("活跃判定不符：磁盘保留算活跃，已结束/已退费算终态")
	}
}

// TestGalaxyDeadlineUnix 平台带回 ServerTime 时用差值折算，本机时钟偏移不影响倒计时
func TestGalaxyDeadlineUnix(t *testing.T) {
	now := time.Date(2026, 8, 29, 18, 0, 0, 0, time.Local)
	// due 比 server 晚 1 小时 → 到期时刻 = now + 1h
	if got := GalaxyDeadlineUnix(3700, 100, now); got != now.Add(time.Hour).Unix() {
		t.Errorf("ServerTime 折算错误: got %d want %d", got, now.Add(time.Hour).Unix())
	}
	// 无 ServerTime → 直接用 due
	if got := GalaxyDeadlineUnix(1234, 0, now); got != 1234 {
		t.Errorf("缺 ServerTime 应回退原值: got %d", got)
	}
	if got := GalaxyDeadlineUnix(0, 100, now); got != 0 {
		t.Errorf("无到期时间应为 0: got %d", got)
	}
}

func TestMaskPhone(t *testing.T) {
	cases := map[string]string{
		"13800001111": "138****1111",
		"1380000":     "1****",
		"":            "",
	}
	for in, want := range cases {
		if got := MaskPhone(in); got != want {
			t.Errorf("MaskPhone(%q)=%q, want %q", in, got, want)
		}
	}
}

// ── 主账户信息 ─────────────────────────────────────────────────

func TestParseGalaxyBalance(t *testing.T) {
	bal, err := ParseGalaxyBalance(galaxyFixture(t, "galaxy_account_info.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bal.Money != 96.2805 || bal.PowerMoney != 12.5 {
		t.Errorf("金额不符: %+v", bal)
	}
	if bal.Name != "用户#2433" || bal.VipLevel != 2 {
		t.Errorf("账户信息不符: %+v", bal)
	}
	if bal.Phone != "138****1111" {
		t.Errorf("手机号必须脱敏，实得 %q", bal.Phone)
	}
	if bal.LastLoginAt == "" {
		t.Error("最后登录时间应解析出来")
	}
}

func TestParseGalaxyBalanceStringNumbers(t *testing.T) {
	// 平台偶发把金额序列化成字符串（DeepSeek 同款口径），不能整体失败
	raw := `{"Money":"12.34","PowerMoney":"0.5","CreditMoneyQuota":"0","VipLevel":"3","Name":"n","Phone":""}`
	bal, err := ParseGalaxyBalance(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bal.Money != 12.34 || bal.PowerMoney != 0.5 || bal.VipLevel != 3 {
		t.Errorf("字符串数值应宽容解析: %+v", bal)
	}
}

func TestParseGalaxyBalanceMissingMoney(t *testing.T) {
	if _, err := ParseGalaxyBalance(`{"Name":"x"}`); err == nil {
		t.Error("缺少 Money 字段应显式失败，不能当 0 元展示")
	}
}

// ── 实例统计 ───────────────────────────────────────────────────

func TestParseGalaxyStatusCount(t *testing.T) {
	s, err := ParseGalaxyStatusCount(galaxyFixture(t, "galaxy_status_count.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.All != 85 || s.Running != 4 || s.KeeppedDisk != 0 || s.CreateError != 0 || s.RunningError != 0 {
		t.Errorf("统计解析不符: %+v", s)
	}
}

func TestParseGalaxyStatusCountEmpty(t *testing.T) {
	if _, err := ParseGalaxyStatusCount(`{}`); err == nil {
		t.Error("空统计应失败，避免把「拉取失败」显示成 0 台")
	}
}

// ── 实例列表 ───────────────────────────────────────────────────

func TestParseGalaxyInstances(t *testing.T) {
	now := time.Date(2026, 8, 29, 18, 0, 0, 0, time.Local)
	list, total, hasMore, err := ParseGalaxyInstances(galaxyFixture(t, "galaxy_instance_list.json"), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 4 || total != 9 || !hasMore {
		t.Fatalf("列表规模不符: len=%d total=%d hasMore=%v", len(list), total, hasMore)
	}
	first := list[0]
	if first.Host != "lyg0098xh" || first.SSHHost != "js1.example.cn" || first.SSHPort != 20812 {
		t.Errorf("连接信息解析不符: %+v", first)
	}
	if first.StatusText != "运行中" || !first.AutoRenew {
		t.Errorf("状态/自动续费解析不符: %+v", first)
	}
	// ServerTime 1787998666、Due_time 1788000681 → 差 2015s，到期 = now + 2015s
	wantDue := time.Unix(now.Unix()+2015, 0)
	got, perr := time.Parse(time.RFC3339, first.DueAt)
	if perr != nil || !got.Equal(wantDue) {
		t.Errorf("到期时间折算错误: got %v want %v", got, wantDue)
	}

	gpu := list[1]
	if gpu.GpuType != "GeForce RTX 3080" || gpu.GpuNum != 1 || gpu.MemoryGB != 48 {
		t.Errorf("GPU 实例解析不符: %+v", gpu)
	}
	if gpu.AutoRenew {
		t.Error("SubscribeStatus=2 且已取消订阅 → 不应判为自动续费")
	}
	if gpu.Note != "训练机" {
		t.Errorf("备注应透传: %q", gpu.Note)
	}

	keepped := list[2]
	if keepped.Status != 8 || keepped.StatusText != "磁盘保留" || keepped.DiskReleaseAt == "" {
		t.Errorf("磁盘保留实例解析不符: %+v", keepped)
	}
	if keepped.DueAt != "" {
		t.Error("Due_time=0 表示无到期时间，应留空而非 1970")
	}

	abn := list[3]
	if !abn.Abnormal {
		t.Error("IsAbnormal!=0 应判为运行异常")
	}
}

// TestParseGalaxyInstancesDropsSecrets 🔴 白名单解码：响应里的实例明文口令
// 不得出现在数据类任何字段（含 JSON 序列化）。
func TestParseGalaxyInstancesDropsSecrets(t *testing.T) {
	raw := galaxyFixture(t, "galaxy_instance_list.json")
	for _, probe := range []string{"SECRET_PWD_1", "SECRET_PWD_2", "SECRET_PWD_3", "SECRET_PWD_4"} {
		if !strings.Contains(raw, probe) {
			t.Fatalf("fixture 应包含 %s 才能验证屏蔽", probe)
		}
	}
	now := time.Date(2026, 8, 29, 18, 0, 0, 0, time.Local)
	list, _, _, err := ParseGalaxyInstances(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range list {
		if err := assertNoSecret(in); err != nil {
			t.Error(err)
		}
	}
}

func assertNoSecret(in models.GalaxyInstance) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	for _, probe := range []string{"SECRET_PWD", "passwd", "Passwd"} {
		if strings.Contains(string(b), probe) {
			return fmt.Errorf("实例序列化结果含敏感字段 %s: %s", probe, b)
		}
	}
	return nil
}

// ── 余额变更 ───────────────────────────────────────────────────

func TestParseGalaxyChanges(t *testing.T) {
	raw := galaxyFixture(t, "galaxy_balance_changes.json")
	items, hasMore, err := ParseGalaxyChanges(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore {
		t.Error("fixture 声明 has_more=true")
	}
	// 5 条里 1 条 CreateTime 非法 → 跳过
	if len(items) != 4 {
		t.Fatalf("应跳过时间格式异常条目: got %d", len(items))
	}
	if items[1].Spent != 0.325 {
		t.Errorf("现金+算力券应合并计消耗: got %v want 0.325", items[1].Spent)
	}
	if items[2].Spent >= 0 {
		t.Errorf("退费条目净消耗应为负: got %v", items[2].Spent)
	}
}

// TestAggregateGalaxyCost 净消耗聚合 + 两个窗口各自的完整性标记
func TestAggregateGalaxyCost(t *testing.T) {
	now := time.Date(2026, 8, 29, 18, 0, 0, 0, time.Local)
	items, hasMore, err := ParseGalaxyChanges(galaxyFixture(t, "galaxy_balance_changes.json"))
	if err != nil {
		t.Fatal(err)
	}
	cost := AggregateGalaxyCost(items, hasMore, now)
	// 今日：0.87 + 0.325（退费 -0.17 不计入）
	if diff := cost.Today - 1.195; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("今日净消耗 got %v want 1.195", cost.Today)
	}
	// 近 7 天含 08-24 那条（+100 充值为净返还 → 不计入），仍 1.195
	if diff := cost.Last7d - 1.195; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("近 7 天净消耗 got %v want 1.195", cost.Last7d)
	}
	if cost.TodayPartial {
		t.Error("已取到 08-24（早于今日）的明细 → 今日窗口已翻完，不该标下限")
	}
	if !cost.WeekPartial {
		t.Error("最早只到 08-24，未跨过 7 天下界（08-23）→ 7 天值应标为下限")
	}
	if len(cost.Entries) != 4 {
		t.Errorf("最近变更条目数不符: %d", len(cost.Entries))
	}
}

// TestAggregateGalaxyCostCompleteWindows 翻到窗口下界之外 → 不再标下限
func TestAggregateGalaxyCostCompleteWindows(t *testing.T) {
	now := time.Date(2026, 8, 29, 18, 0, 0, 0, time.Local)
	items := []GalaxyChange{
		{At: now.Add(-time.Hour), Spent: 1},
		{At: now.AddDate(0, 0, -10), Spent: 2}, // 早于 7 天下界
	}
	cost := AggregateGalaxyCost(items, true, now)
	if cost.TodayPartial || cost.WeekPartial {
		t.Errorf("窗口已翻完仍标记下限: %+v", cost)
	}
	if cost.Today != 1 {
		t.Errorf("今日只含今日条目: got %v", cost.Today)
	}
	if cost.Last7d != 1 {
		t.Errorf("10 天前的条目不该进 7 天窗口: got %v", cost.Last7d)
	}
}

// ── 防御路径回归（oracle 双实现对照修复） ─────────────────────

// TestRawIntStringTolerant 平台偶发把整型序列化成字符串：严格模式会把
// Status:"1" 解析成 0（已结束），属于静默错误 → rawInt 必须双兼容。
func TestRawIntStringTolerant(t *testing.T) {
	if v, ok := rawInt(json.RawMessage(`"1"`)); !ok || v != 1 {
		t.Errorf("rawInt(\"1\") = %d,%v, want 1,true", v, ok)
	}
	if v, ok := rawInt(json.RawMessage(`" 2 "`)); !ok || v != 2 {
		t.Errorf("rawInt(\" 2 \") = %d,%v, want 2,true（带空白也容忍）", v, ok)
	}
	if v, ok := rawInt(json.RawMessage(`"abc"`)); ok || v != 0 {
		t.Errorf("rawInt(\"abc\") = %d,%v, want 0,false", v, ok)
	}
}

// TestParseGalaxyBalanceNullMoney Money 显式 null = 异常响应，不能当 0 元成功
func TestParseGalaxyBalanceNullMoney(t *testing.T) {
	if _, err := ParseGalaxyBalance(`{"Money":null,"Name":"x","Phone":""}`); err == nil {
		t.Error("Money:null 应显式失败（Android 侧同口径报错，两侧不得一静默一报错）")
	}
}

// TestParseGalaxyStatusCountNullFields 统计字段显式 null 同族
func TestParseGalaxyStatusCountNullFields(t *testing.T) {
	if _, err := ParseGalaxyStatusCount(`{"statusAll":null,"statusRunning":null}`); err == nil {
		t.Error("统计字段全 null 应显式失败")
	}
}

// TestParseGalaxyInstancesNullList list:null = 异常响应（空数组才是正常空态）
func TestParseGalaxyInstancesNullList(t *testing.T) {
	if _, _, _, err := ParseGalaxyInstances(`{"list":null}`, time.Now()); err == nil {
		t.Error("list:null 应显式失败")
	}
}

// TestParseGalaxyInstancesStringInts 字符串整型 → 不能把运行中实例读成已结束
func TestParseGalaxyInstancesStringInts(t *testing.T) {
	raw := `{"list":[{"Container_name":"c1","Status":"1","IsAbnormal":"0","Gpu_num":"0","Cpu_num":"8","Memory":"16","Due_time":"1788000681","ServerTime":"1787998666","SshPort":"20812"}],"has_more":false,"total_count":"1"}`
	now := time.Date(2026, 8, 29, 18, 0, 0, 0, time.Local)
	list, total, _, err := ParseGalaxyInstances(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || total != 1 {
		t.Fatalf("字符串整型解析失败: list=%d total=%d", len(list), total)
	}
	if list[0].Status != 1 || list[0].StatusText != "运行中" || list[0].CpuNum != 8 || list[0].SSHPort != 20812 {
		t.Errorf("字符串整型字段应宽容解析: %+v", list[0])
	}
}
