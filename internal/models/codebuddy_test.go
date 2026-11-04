package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCodebuddyMultiplier(t *testing.T) {
	cases := []struct {
		raw    string
		want   float64
		wantOK bool
		desc   string
	}{
		{"x0.34 credits", 0.34, true, "带 credits 后缀"},
		{"x0.00", 0, true, "无后缀但显式标注 0"},
		{"", 0, false, "空串 = 未标注（不是免费）"},
		{"   ", 0, false, "纯空白同空串"},
		{"×0.59 credits", 0.59, true, "全角乘号容错"},
		{"X3.31 credits", 3.31, true, "大写 X 容错"},
		{"x0.08", 0.08, true, "两位小数"},
		{"x1.00 credit", 1.0, true, "单数 credit 容错"},
		{"x6.67 credits", 6.67, true, "目录最大值"},
		{"garbage", 0, false, "不可解析 → 未标注，不猜"},
		{"xNaN credits", 0, false, "NaN 拒绝"},
		{"x-1.5", 0, false, "负值拒绝（倍率不可能为负）"},
	}
	for _, c := range cases {
		got, ok := ParseCodebuddyMultiplier(c.raw)
		if ok != c.wantOK {
			t.Errorf("%s: ParseCodebuddyMultiplier(%q) ok=%v, want %v", c.desc, c.raw, ok, c.wantOK)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%s: ParseCodebuddyMultiplier(%q)=%v, want %v", c.desc, c.raw, got, c.want)
		}
	}
}

func TestCodebuddyLedgerRoot(t *testing.T) {
	// 显式 CODEBUDDY_CONFIG_DIR 优先（与 CLI 自身配置目录同源）
	t.Setenv("CODEBUDDY_CONFIG_DIR", "/tmp/cb-test-home")
	if got, want := CodebuddyLedgerRoot(), filepath.Join("/tmp/cb-test-home", "projects"); got != want {
		t.Errorf("CodebuddyLedgerRoot()=%q, want %q", got, want)
	}
	// 未设 → ~/.codebuddy/projects
	t.Setenv("CODEBUDDY_CONFIG_DIR", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("无 home 目录")
	}
	if got, want := CodebuddyLedgerRoot(), filepath.Join(home, ".codebuddy", "projects"); got != want {
		t.Errorf("CodebuddyLedgerRoot()=%q, want %q", got, want)
	}
}

func TestCodebuddyWindowOf(t *testing.T) {
	l := &CodebuddyLedger{Windows: []CodebuddyWindow{
		{Label: "今日", Days: 1, Calls: 3, Credits: 1.5},
		{Label: "全部", Days: 0, Calls: 9, Credits: 4.5},
	}}
	if w := CodebuddyWindowOf(l, "今日"); w.Calls != 3 || w.Credits != 1.5 {
		t.Errorf("今日窗口取错: %+v", w)
	}
	// 不存在的标签 → 零值，且标签回填（渲染层据此跳过该行）
	if w := CodebuddyWindowOf(l, "近7天"); w.Calls != 0 || w.Label != "近7天" {
		t.Errorf("未知窗口应为零值且回填标签: %+v", w)
	}
	// nil 台账不 panic
	if w := CodebuddyWindowOf(nil, "今日"); w.Calls != 0 {
		t.Errorf("nil 台账应返回零值: %+v", w)
	}
}

func TestCodebuddyHasUsage(t *testing.T) {
	if CodebuddyHasUsage(nil) {
		t.Error("nil 台账不应判为有用量")
	}
	if CodebuddyHasUsage(&CodebuddyLedger{}) {
		t.Error("空台账不应判为有用量")
	}
	if !CodebuddyHasUsage(&CodebuddyLedger{Total: CodebuddyWindow{Calls: 1}}) {
		t.Error("1 次调用应判为有用量")
	}
}
