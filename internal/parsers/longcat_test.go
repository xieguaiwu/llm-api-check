package parsers

import (
	"errors"
	"strings"
	"testing"
)

// lcModelsHappy GET /v1/models 成功响应（OpenAI 兼容格式，2026-09-13 实测形状）。
const lcModelsHappy = `{"object":"list","data":[` +
	`{"id":"LongCat-2.0","object":"model","created":1700000000,"owned_by":"meituan"},` +
	`{"id":"LongCat-Flash-Chat","object":"model","created":1700000001,"owned_by":"meituan"}` +
	`]}`

// lcModelsEmpty 空模型清单。
const lcModelsEmpty = `{"object":"list","data":[]}`

func TestParseLongCatModelsHappy(t *testing.T) {
	ms, err := ParseLongCatModels(lcModelsHappy)
	if err != nil {
		t.Fatalf("ParseLongCatModels: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("len: got %d, want 2", len(ms))
	}
	if ms[0].ID != "LongCat-2.0" {
		t.Errorf("ms[0].ID: %q", ms[0].ID)
	}
	if ms[1].ID != "LongCat-Flash-Chat" {
		t.Errorf("ms[1].ID: %q", ms[1].ID)
	}
	if ms[0].OwnedBy != "meituan" {
		t.Errorf("ms[0].OwnedBy: %q", ms[0].OwnedBy)
	}
}

func TestParseLongCatModelsEmpty(t *testing.T) {
	_, err := ParseLongCatModels(lcModelsEmpty)
	if err == nil {
		t.Fatal("empty models should error")
	}
}

func TestParseLongCatModelsDedup(t *testing.T) {
	dup := `{"data":[{"id":"LongCat-2.0","owned_by":"meituan"},{"id":"LongCat-2.0","owned_by":"meituan"}]}`
	ms, err := ParseLongCatModels(dup)
	if err != nil {
		t.Fatalf("ParseLongCatModels: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("dedup: got %d, want 1", len(ms))
	}
}

func TestParseLongCatModelsSorted(t *testing.T) {
	unsorted := `{"data":[{"id":"Z-Model","owned_by":"x"},{"id":"A-Model","owned_by":"x"}]}`
	ms, err := ParseLongCatModels(unsorted)
	if err != nil {
		t.Fatalf("ParseLongCatModels: %v", err)
	}
	if ms[0].ID != "A-Model" || ms[1].ID != "Z-Model" {
		t.Errorf("not sorted: %s, %s", ms[0].ID, ms[1].ID)
	}
}

func TestParseLongCatModelsInvalidJSON(t *testing.T) {
	_, err := ParseLongCatModels("not json")
	if err == nil {
		t.Fatal("invalid JSON should error")
	}
	if !strings.Contains(err.Error(), "LongCat") {
		t.Errorf("error should mention LongCat: %v", err)
	}
}

func TestErrLongCatAuth(t *testing.T) {
	// 401 错误响应不应包含 key 原文（与 GPTZero 不同），且文案应明确指向凭据问题
	msg := ErrLongCatAuth.Error()
	if msg == "" {
		t.Fatal("ErrLongCatAuth message empty")
	}
	want := "LongCat API Key 无效或已过期"
	if !strings.Contains(msg, want) {
		t.Errorf("ErrLongCatAuth 应含 %q, 实得 %q", want, msg)
	}
}

// lcTokenPacksHappy token-packs/summary 成功响应（含资源包 + 估算，2026-09-16 实测形状）。
const lcTokenPacksHappy = `{"code":0,"msg":"success","data":{` +
	`"currentLot":{"remainingToken":1234567,"totalToken":5000000,"consumedToken":3765433,` +
	`"consumedRatio":0.753,"expireTime":1760342400000,"remainSeconds":2332800,"grantCategory":"GIFT"},` +
	`"otherLots":[],` +
	`"estimate":{"windowDays":7,"dailyAverageToken":12345,"exhaustedAfterDays":100}}}`

// lcTokenPacksNoPack 无资源包（2026-09-16 实测原样：currentLot=null 是合法语义）。
const lcTokenPacksNoPack = `{"code":0,"msg":"success","data":{` +
	`"currentLot":null,"estimate":{"windowDays":7,"dailyAverageToken":0,"exhaustedAfterDays":0},"otherLots":[]}}`

// lcPaygoHappy api-usage/summary 成功响应（2026-09-16 实测原样）。
const lcPaygoHappy = `{"code":0,"msg":"success","data":{` +
	`"paygoBalanceCent":0,"paygoStatus":"NORMAL","rechargeEnabled":true,` +
	`"statusTip":"账户余额已耗尽，请及时充值以确保 API 正常使用",` +
	`"paygoBalance":{"primary":{"currency":"CNY","amount":"0.00"},"secondary":null},"exchangeRate":6.8}}`

// lcConsole401 控制台会话失效（实测原文）。
const lcConsole401 = `{"code":401,"msg":"登录状态无效，请重新登录","data":null}`

func TestParseLongCatTokenPacksSummaryHappy(t *testing.T) {
	q, err := ParseLongCatTokenPacksSummary(lcTokenPacksHappy)
	if err != nil {
		t.Fatalf("ParseLongCatTokenPacksSummary: %v", err)
	}
	if q.CurrentLot == nil {
		t.Fatal("CurrentLot should not be nil")
	}
	lot := q.CurrentLot
	if lot.RemainingToken != 1234567 || lot.TotalToken != 5000000 || lot.ConsumedToken != 3765433 {
		t.Errorf("lot tokens: %+v", lot)
	}
	if lot.ConsumedRatio != 0.753 {
		t.Errorf("ConsumedRatio: %v", lot.ConsumedRatio)
	}
	if lot.ExpireTime != 1760342400000 || lot.RemainSeconds != 2332800 {
		t.Errorf("lot expiry: %+v", lot)
	}
	if lot.GrantCategory != "GIFT" {
		t.Errorf("GrantCategory: %q", lot.GrantCategory)
	}
	if len(q.OtherLots) != 0 {
		t.Errorf("OtherLots: %v", q.OtherLots)
	}
	if q.Estimate == nil {
		t.Fatal("Estimate should not be nil")
	}
	if q.Estimate.WindowDays != 7 || q.Estimate.DailyAverageToken != 12345 || q.Estimate.ExhaustedAfterDays != 100 {
		t.Errorf("Estimate: %+v", q.Estimate)
	}
}

func TestParseLongCatTokenPacksSummaryNoPack(t *testing.T) {
	q, err := ParseLongCatTokenPacksSummary(lcTokenPacksNoPack)
	if err != nil {
		t.Fatalf("no-pack should be legal: %v", err)
	}
	if q.CurrentLot != nil {
		t.Errorf("CurrentLot should be nil for no-pack: %+v", q.CurrentLot)
	}
	if q.Estimate == nil || q.Estimate.DailyAverageToken != 0 {
		t.Errorf("Estimate: %+v", q.Estimate)
	}
}

func TestParseLongCatTokenPacksSummaryBadJSON(t *testing.T) {
	_, err := ParseLongCatTokenPacksSummary("not json")
	if err == nil {
		t.Fatal("invalid JSON should error")
	}
}

func TestParseLongCatTokenPacksSummaryErrorCode(t *testing.T) {
	_, err := ParseLongCatTokenPacksSummary(`{"code":500,"msg":"服务器开小差了","data":null}`)
	if err == nil {
		t.Fatal("code!=0 should error")
	}
	if !strings.Contains(err.Error(), "服务器开小差了") {
		t.Errorf("error should carry msg: %v", err)
	}
}

func TestParseLongCatTokenPacksSummaryCode401(t *testing.T) {
	_, err := ParseLongCatTokenPacksSummary(lcConsole401)
	if err == nil {
		t.Fatal("code=401 should error")
	}
	if !errors.Is(err, ErrLongCatConsoleAuth) {
		t.Errorf("code=401 should map to ErrLongCatConsoleAuth: %v", err)
	}
}

func TestParseLongCatTokenPacksSummaryMissingCode(t *testing.T) {
	_, err := ParseLongCatTokenPacksSummary(`{"msg":"success","data":{}}`)
	if err == nil {
		t.Fatal("missing code should error")
	}
}

func TestParseLongCatTokenPacksSummaryMissingData(t *testing.T) {
	if _, err := ParseLongCatTokenPacksSummary(`{"code":0,"msg":"success"}`); err == nil {
		t.Fatal("missing data should error")
	}
	if _, err := ParseLongCatTokenPacksSummary(`{"code":0,"msg":"success","data":null}`); err == nil {
		t.Fatal("null data should error")
	}
}

func TestParseLongCatPaygoHappy(t *testing.T) {
	p, err := ParseLongCatPaygoSummary(lcPaygoHappy)
	if err != nil {
		t.Fatalf("ParseLongCatPaygoSummary: %v", err)
	}
	if p.PaygoBalanceCent != 0 || p.PaygoStatus != "NORMAL" || !p.RechargeEnabled {
		t.Errorf("paygo fields: %+v", p)
	}
	if p.PaygoBalance == nil || p.PaygoBalance.Primary == nil {
		t.Fatal("PaygoBalance.Primary should not be nil")
	}
	if p.PaygoBalance.Primary.Currency != "CNY" || p.PaygoBalance.Primary.Amount != "0.00" {
		t.Errorf("primary: %+v", p.PaygoBalance.Primary)
	}
	if p.PaygoBalance.Secondary != nil {
		t.Errorf("secondary should be nil: %+v", p.PaygoBalance.Secondary)
	}
	if p.ExchangeRate != 6.8 {
		t.Errorf("ExchangeRate: %v", p.ExchangeRate)
	}
}

func TestParseLongCatPaygoCode401(t *testing.T) {
	_, err := ParseLongCatPaygoSummary(lcConsole401)
	if !errors.Is(err, ErrLongCatConsoleAuth) {
		t.Errorf("code=401 should map to ErrLongCatConsoleAuth: %v", err)
	}
}

func TestParseLongCatPaygoMissingData(t *testing.T) {
	if _, err := ParseLongCatPaygoSummary(`{"code":0,"msg":"success"}`); err == nil {
		t.Fatal("missing data should error")
	}
}

func TestParseLongCatPaygoMissingPrimary(t *testing.T) {
	_, err := ParseLongCatPaygoSummary(`{"code":0,"msg":"success","data":{"paygoBalance":{"secondary":null}}}`)
	if err == nil {
		t.Fatal("missing paygoBalance.primary should error")
	}
}

func TestParseLongCatModelsCapabilityFields(t *testing.T) {
	raw := `{"object":"list","data":[{` +
		`"id":"LongCat-2.0","object":"model","owned_by":"meituan",` +
		`"display_name":"LongCat 2.0","context_window":1048576,"max_output_tokens":131072}]}`
	ms, err := ParseLongCatModels(raw)
	if err != nil {
		t.Fatalf("ParseLongCatModels: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("len: got %d, want 1", len(ms))
	}
	m := ms[0]
	if m.DisplayName != "LongCat 2.0" || m.ContextWindow != 1048576 || m.MaxOutputTokens != 131072 {
		t.Errorf("capability fields: %+v", m)
	}
}

func TestParseLongCatModelsCapabilityFieldsAbsent(t *testing.T) {
	// 能力字段缺席 = 未知（0），不得失败
	ms, err := ParseLongCatModels(lcModelsHappy)
	if err != nil {
		t.Fatalf("ParseLongCatModels: %v", err)
	}
	if ms[0].ContextWindow != 0 || ms[0].MaxOutputTokens != 0 || ms[0].DisplayName != "" {
		t.Errorf("absent capability fields should default 0: %+v", ms[0])
	}
}
