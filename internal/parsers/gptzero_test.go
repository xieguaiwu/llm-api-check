package parsers

import (
	"strings"
	"testing"
)

// gzUsersHappy GET /v2/users/me 成功响应（2026-09-07 真实形状截取，api_key 字段
// 刻意保留——白名单结构体必须把它丢掉）。
const gzUsersHappy = `{"data":{"id":"fceb2e86-31ef-4930-a201-a12bb4988b5e","email":"xieguaiwu@163.com",` +
	`"plan":"API (300k words/month)","api_key":"adfc62c318f14a16a8843c952b56ea5b","char_limit":150000,` +
	`"monthly_input_words":587,"monthly_input_chars":3952,"monthly_input_documents":1,` +
	`"all_time_input_words":1675389,"all_time_input_chars":11680324,"all_time_input_documents":861,` +
	`"last_time_usage_reset":"2026-09-06T16:28:34.943+00:00",` +
	`"full_plan":{"id":"a50d004f","name":"API (300k words/month)","tier":"api","duration_type":"monthly",` +
	`"word_limit":300000,"overage_word_limit":1000000,` +
	`"priceData":{"currency":"usd","unit_amount":4500}}}}`

func TestParseGptzeroUsageHappy(t *testing.T) {
	u, err := ParseGptzeroUsage(gzUsersHappy)
	if err != nil {
		t.Fatalf("ParseGptzeroUsage: %v", err)
	}
	if u.Email != "xieguaiwu@163.com" {
		t.Errorf("Email: %q", u.Email)
	}
	if u.PlanName != "API (300k words/month)" {
		t.Errorf("PlanName: %q", u.PlanName)
	}
	if u.Monthly.Words != 587 || u.Monthly.Chars != 3952 || u.Monthly.Documents != 1 {
		t.Errorf("Monthly: %+v", u.Monthly)
	}
	if u.AllTime.Words != 1675389 || u.AllTime.Documents != 861 {
		t.Errorf("AllTime: %+v", u.AllTime)
	}
	if u.CharLimit != 150000 {
		t.Errorf("CharLimit: %d", u.CharLimit)
	}
	if u.LastReset != "2026-09-06T16:28:34.943+00:00" {
		t.Errorf("LastReset: %q", u.LastReset)
	}
	if u.Plan.WordLimit != 300000 {
		t.Errorf("WordLimit: %d", u.Plan.WordLimit)
	}
	if u.Plan.OverageWordLimit != 1000000 {
		t.Errorf("OverageWordLimit: %d", u.Plan.OverageWordLimit)
	}
	if u.Plan.PriceCents != 4500 {
		t.Errorf("PriceCents: %d", u.Plan.PriceCents)
	}
	if u.Plan.DurationType != "monthly" {
		t.Errorf("DurationType: %q", u.Plan.DurationType)
	}
	// 🔒 api_key 明文不得进入任何模型字段
	if strings.Contains(u.PlanName, "adfc62") || strings.Contains(u.Email, "adfc62") {
		t.Fatalf("api_key 泄入模型字段")
	}
}

func TestParseGptzeroUsageDropsAPIKey(t *testing.T) {
	u, err := ParseGptzeroUsage(gzUsersHappy)
	if err != nil {
		t.Fatalf("ParseGptzeroUsage: %v", err)
	}
	for _, s := range []string{u.Email, u.PlanName, u.Plan.Name, u.LastReset} {
		if strings.Contains(s, "adfc62c318f14a16a8843c952b56ea5b") {
			t.Fatalf("api_key 明文透出: %q", s)
		}
	}
}

func TestParseGptzeroUsageMissingData(t *testing.T) {
	_, err := ParseGptzeroUsage(`{"error":"Require valid cookie"}`)
	if err == nil || !strings.Contains(err.Error(), "data") {
		t.Fatalf("缺 data 应显式失败: %v", err)
	}
	_, err = ParseGptzeroUsage(`{"data":null}`)
	if err == nil || !strings.Contains(err.Error(), "data") {
		t.Fatalf("data=null 应显式失败: %v", err)
	}
}

func TestParseGptzeroUsageMissingMonthlyWords(t *testing.T) {
	raw := `{"data":{"email":"a@b.c","plan":"API (300k words/month)","full_plan":{"word_limit":300000}}}`
	_, err := ParseGptzeroUsage(raw)
	if err == nil || !strings.Contains(err.Error(), "monthly_input_words") {
		t.Fatalf("缺 monthly_input_words 应显式失败（不得显示 0 冒充没用过）: %v", err)
	}
}

func TestParseGptzeroUsageMissingWordLimit(t *testing.T) {
	raw := `{"data":{"email":"a@b.c","monthly_input_words":10,"full_plan":{"name":"x"}}}`
	_, err := ParseGptzeroUsage(raw)
	if err == nil || !strings.Contains(err.Error(), "word_limit") {
		t.Fatalf("缺 full_plan.word_limit 应显式失败: %v", err)
	}
}

func TestParseGptzeroUsageStringNumbers(t *testing.T) {
	raw := `{"data":{"email":"a@b.c","monthly_input_words":"123","monthly_input_chars":"456",` +
		`"monthly_input_documents":"1","char_limit":"150000",` +
		`"full_plan":{"word_limit":"300000","overage_word_limit":"1000000","priceData":{"unit_amount":"4500"}}}}`
	u, err := ParseGptzeroUsage(raw)
	if err != nil {
		t.Fatalf("字符串数字形状应宽容: %v", err)
	}
	if u.Monthly.Words != 123 || u.Plan.WordLimit != 300000 || u.Plan.PriceCents != 4500 || u.CharLimit != 150000 {
		t.Errorf("字符串数字未转换: %+v %+v", u.Monthly, u.Plan)
	}
}

func TestParseGptzeroUsageBrokenJSON(t *testing.T) {
	_, err := ParseGptzeroUsage(`{not json`)
	if err == nil || !strings.Contains(err.Error(), "JSON 解析失败") {
		t.Fatalf("坏 JSON 应报解析失败: %v", err)
	}
}

func TestParseGptzeroUsageOptionalFieldsAbsent(t *testing.T) {
	// 可选键缺席（overage/price/last_reset/char_limit）按 0/空串，不失败
	raw := `{"data":{"email":"a@b.c","monthly_input_words":0,"full_plan":{"word_limit":100}}}`
	u, err := ParseGptzeroUsage(raw)
	if err != nil {
		t.Fatalf("可选键缺席不应失败: %v", err)
	}
	if u.Plan.OverageWordLimit != 0 || u.Plan.PriceCents != 0 || u.LastReset != "" || u.CharLimit != 0 {
		t.Errorf("可选键缺席应零值: %+v", u)
	}
}
