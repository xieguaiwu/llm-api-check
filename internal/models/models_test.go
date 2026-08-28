package models

import (
	"encoding/json"
	"testing"
)

// 账号判定方法：对应 Kotlin hasToken / hasZen 语义（blank 视为未配置）
func TestAccountHelpers(t *testing.T) {
	acc := Account{ID: "1", Name: "n", GoApiKey: "k", WorkspaceId: "wrk_x", AuthCookie: "c"}
	if !acc.HasZen() {
		t.Error("workspace 与 cookie 都非空时 HasZen 应为 true")
	}
	acc.AuthCookie = "  "
	if acc.HasZen() {
		t.Error("cookie 为空白时 HasZen 应为 false")
	}
	acc.WorkspaceId = ""
	acc.AuthCookie = "c"
	if acc.HasZen() {
		t.Error("workspace 为空时 HasZen 应为 false")
	}

	ds := DeepSeekAccount{ID: "1", Name: "n", ApiKey: "k", PlatformToken: "t"}
	if !ds.HasToken() {
		t.Error("platformToken 非空时 HasToken 应为 true")
	}
	ds.PlatformToken = ""
	if ds.HasToken() {
		t.Error("platformToken 为空时 HasToken 应为 false")
	}
}

// JSON 序列化名与 Android 版一致（--json 输出依赖）
func TestAccountJSONTags(t *testing.T) {
	acc := Account{ID: "id1", Name: "n", GoApiKey: "gk", WorkspaceId: "w", AuthCookie: "c"}
	raw, err := json.Marshal(acc)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{`"id":"id1"`, `"goApiKey":"gk"`, `"workspaceId":"w"`, `"authCookie":"c"`} {
		if !contains(got, want) {
			t.Errorf("序列化结果 %s 缺少 %s", got, want)
		}
	}
	if contains(got, "go_api_key") {
		t.Errorf("Account 不应使用 snake_case tag: %s", got)
	}
}

func TestBalanceJSONTags(t *testing.T) {
	info := DeepSeekBalanceInfo{Currency: "CNY", TotalBalance: 1.5, GrantedBalance: 0, ToppedUpBalance: 1.5}
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{`"total_balance":1.5`, `"granted_balance":0`, `"topped_up_balance":1.5`, `"currency":"CNY"`} {
		if !contains(got, want) {
			t.Errorf("序列化结果 %s 缺少 %s", got, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── Qwen ───────────────────────────────────────────────────────

func TestNormalizeQwenRegion(t *testing.T) {
	ok := map[string]string{
		"":               RegionQwenCN,
		"cn":             RegionQwenCN,
		" CN-Beijing ":   RegionQwenCN,
		"beijing":        RegionQwenCN,
		"intl":           RegionQwenIntl,
		"sg":             RegionQwenIntl,
		"ap-southeast-1": RegionQwenIntl,
		"International":  RegionQwenIntl,
	}
	for in, want := range ok {
		got, err := NormalizeQwenRegion(in)
		if err != nil || got != want {
			t.Errorf("NormalizeQwenRegion(%q) = (%q,%v), want %q", in, got, err, want)
		}
	}
	if _, err := NormalizeQwenRegion("mars"); err == nil {
		t.Error("未知区域应报错")
	}
}

func TestQwenAccountHelpers(t *testing.T) {
	a := QwenAccount{ID: "q", Name: "n", ApiKey: "k", ConsoleCookie: "  "}
	if a.HasCookie() {
		t.Error("纯空白 Cookie 应视为未配置")
	}
	if a.QwenRegion() != RegionQwenCN {
		t.Errorf("空区域应回落中国大陆: %q", a.QwenRegion())
	}
	a.Region = "非法区域"
	if a.QwenRegion() != RegionQwenCN {
		t.Errorf("非法区域应回落中国大陆: %q", a.QwenRegion())
	}
	a.ConsoleCookie = "sec_token=tok"
	if !a.HasCookie() {
		t.Error("配置 Cookie 后 HasCookie 应为真")
	}
	if QwenRegionDisplayName("intl") != "国际（新加坡）" || QwenRegionDisplayName("") != "中国大陆（北京）" {
		t.Error("区域展示名不符")
	}
}
