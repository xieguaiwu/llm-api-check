package models

import (
	"encoding/json"
	"testing"
	"time"
)

// Commandai 套餐显示名：Er/Mr 映射 + 最长前缀匹配。
func TestCommandaiPlanDisplayName(t *testing.T) {
	cases := map[string]string{
		"individual-provider": "Provider",
		"individual-go":       "Go",
		"individual-go-v1":    "Go",
		"individual-goat":     "GOAT",
		"individual-pro-v1":   "Pro",
		"individual-max":      "Max",
		"individual-ultra":    "Ultra",
		"teams-pro":           "Teams Pro",
	}
	for in, want := range cases {
		if got := CommandaiPlanDisplayName(in); got != want {
			t.Errorf("CommandaiPlanDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommandaiPlanDisplayNameUnknown(t *testing.T) {
	// 未知套餐回原文，不吞
	if got := CommandaiPlanDisplayName("mystery-plan"); got != "mystery-plan" {
		t.Errorf("unknown plan should return raw: %q", got)
	}
}

func TestCommandaiPlanDisplayNameNormalization(t *testing.T) {
	// 小写化 + 下划线转连字符
	if got := CommandaiPlanDisplayName("Individual_Provider"); got != "Provider" {
		t.Errorf("normalization failed: %q", got)
	}
}

func TestCommandaiPlanMonthlyCredits(t *testing.T) {
	cases := map[string]float64{
		"individual-go":       10,
		"individual-go-v1":    10,
		"individual-goat":     70,
		"individual-pro-v1":   80,
		"individual-provider": 15,
		"individual-max":      150,
		"individual-ultra":    300,
		"teams-pro":           40,
	}
	for in, want := range cases {
		got, ok := CommandaiPlanMonthlyCredits(in)
		if !ok {
			t.Errorf("CommandaiPlanMonthlyCredits(%q) ok=false, want true", in)
			continue
		}
		if got != want {
			t.Errorf("CommandaiPlanMonthlyCredits(%q) = %v, want %v", in, got, want)
		}
	}
	if _, ok := CommandaiPlanMonthlyCredits("unknown"); ok {
		t.Error("unknown plan should return ok=false")
	}
}

// CommandaiBillingOf：active 套餐走 max(planMonthly, monthly) 分支。
func TestCommandaiBillingOfActivePlan(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	end := now.Add(12 * 24 * time.Hour)
	c := &CommandaiCredits{MonthlyCredits: 10, PurchasedCredits: 3.2, FreeCredits: 0}
	s := &CommandaiSubscription{PlanID: "individual-provider", Status: "active", CurrentPeriodEnd: &end}
	u := &CommandaiUsageSummary{TotalCost: 1.8, TotalCount: 42}
	b := CommandaiBillingOf(c, s, u, now)
	if !b.HasBilling {
		t.Fatal("HasBilling should be true")
	}
	// totalRemaining = 10 + 3.2 + 0 = 13.2
	if b.TotalRemaining != 13.2 {
		t.Errorf("TotalRemaining = %v, want 13.2", b.TotalRemaining)
	}
	// totalPool = max(15, 10) + 3.2 + 0 = 18.2
	if b.TotalPool != 18.2 {
		t.Errorf("TotalPool = %v, want 18.2", b.TotalPool)
	}
	// usagePercent = (18.2-13.2)/18.2*100 ≈ 27.47
	if b.UsagePercent < 27 || b.UsagePercent > 28 {
		t.Errorf("UsagePercent = %v, want ~27.47", b.UsagePercent)
	}
	if b.Spent != 1.8 {
		t.Errorf("Spent = %v, want 1.8", b.Spent)
	}
	if b.DaysLeft != 12 {
		t.Errorf("DaysLeft = %d, want 12", b.DaysLeft)
	}
}

// 非 active 套餐（past_due/trialing/未知）走 spent+remaining 分支。
func TestCommandaiBillingOfNoPlan(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	c := &CommandaiCredits{MonthlyCredits: 5, PurchasedCredits: 2, FreeCredits: 1}
	s := &CommandaiSubscription{PlanID: "individual-provider", Status: "past_due"}
	u := &CommandaiUsageSummary{TotalCost: 0.5}
	b := CommandaiBillingOf(c, s, u, now)
	// totalPool = spent + remaining = 0.5 + 8 = 8.5
	if b.TotalPool != 8.5 {
		t.Errorf("TotalPool = %v, want 8.5", b.TotalPool)
	}
	// usagePercent = (8.5-8)/8.5*100 ≈ 5.88
	if b.UsagePercent < 5 || b.UsagePercent > 6 {
		t.Errorf("UsagePercent = %v, want ~5.88", b.UsagePercent)
	}
	if b.DaysLeft != -1 {
		t.Errorf("DaysLeft = %d, want -1 (周期未知)", b.DaysLeft)
	}
}

func TestCommandaiBillingOfNegativeClamped(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	// 负池值按 0 处理（max(0,·)）
	c := &CommandaiCredits{MonthlyCredits: -5, PurchasedCredits: -1, FreeCredits: 0}
	u := &CommandaiUsageSummary{TotalCost: 2}
	b := CommandaiBillingOf(c, nil, u, now)
	if b.TotalRemaining != 0 {
		t.Errorf("TotalRemaining = %v, want 0", b.TotalRemaining)
	}
	if b.TotalPool != 2 {
		t.Errorf("TotalPool = %v, want 2", b.TotalPool)
	}
	// totalPool>0 且 remaining=0 → 100%
	if b.UsagePercent != 100 {
		t.Errorf("UsagePercent = %v, want 100", b.UsagePercent)
	}
}

func TestCommandaiBillingOfNilCredits(t *testing.T) {
	b := CommandaiBillingOf(nil, nil, nil, time.Now())
	if b.HasBilling {
		t.Error("nil credits should have HasBilling=false")
	}
}

// CommandaiAccount JSON tag 与线协议一致（--json 输出依赖）。
func TestCommandaiAccountJSONTags(t *testing.T) {
	a := CommandaiAccount{ID: "id1", Name: "n", ApiKey: "k"}
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{`"id":"id1"`, `"name":"n"`, `"api_key":"k"`} {
		if !contains(got, want) {
			t.Errorf("序列化结果 %s 缺少 %s", got, want)
		}
	}
}

// 窗口与沙箱 JSON tag 与线协议一致。
func TestCommandaiCreditsJSONTags(t *testing.T) {
	c := CommandaiCredits{
		PlanID:           "individual-provider",
		MonthlyCredits:   10,
		PurchasedCredits: 3.2,
		FreeCredits:      0,
		SandboxAccess:    true,
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		`"planId":"individual-provider"`, `"monthlyCredits":10`,
		`"purchasedCredits":3.2`, `"freeCredits":0`, `"sandboxAccess":true`,
	} {
		if !contains(got, want) {
			t.Errorf("序列化结果 %s 缺少 %s", got, want)
		}
	}
}

func TestCommandaiBillingOfMaxPoolEdge(t *testing.T) {
	// 反向验证⑤配套：credits.monthlyCredits > planMonthly 时 totalPool 取二者较大值
	// （npm projectUsageView: Math.max(planMonthly, monthlyCredits)）。
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	c := &CommandaiCredits{MonthlyCredits: 20, PurchasedCredits: 3.2}
	s := &CommandaiSubscription{PlanID: "individual-provider", Status: "active"}
	b := CommandaiBillingOf(c, s, nil, now)
	// totalPool = max(15, 20) + 3.2 = 23.2；若无 max 分支则为 15+3.2=18.2
	if b.TotalPool != 23.2 {
		t.Errorf("TotalPool = %v, want 23.2 (max(15,20)+3.2)", b.TotalPool)
	}
}
