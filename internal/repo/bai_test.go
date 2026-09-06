package repo

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// baiModelsBody /v1/models 成功响应（真实形状截取）
const baiModelsBody = `{"data":[{"id":"qwen3.8-flash","object":"model","owned_by":"qwen","supported_endpoint_types":["openai"]},` +
	`{"id":"deepseek-v4-flash","object":"model","owned_by":"deepseek","supported_endpoint_types":null}],"object":"list","success":true}`

func TestBaiModelsWireFormat(t *testing.T) {
	var gotAuth, gotAccept, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotPath = r.URL.Path
		w.Write([]byte(baiModelsBody))
	}))
	defer ts.Close()

	r := NewBaiRepo()
	r.BaseURL = ts.URL
	plan, err := r.Models("sk-baitest123")
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if gotPath != "/v1/models" {
		t.Errorf("请求路径不符: %s", gotPath)
	}
	if gotAuth != "Bearer sk-baitest123" {
		t.Errorf("Bearer 头不符: %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept 头不符: %q", gotAccept)
	}
	if len(plan.Models) != 2 || plan.Models[0].ID != "deepseek-v4-flash" {
		t.Errorf("解析结果不符: %+v", plan.Models)
	}
}

func TestBaiModelsAuthErrors(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			w.Write([]byte(`{"message":"Invalid token","success":false}`))
		}))
		r := NewBaiRepo()
		r.BaseURL = ts.URL
		_, err := r.Models("sk-expired")
		ts.Close()
		if err == nil {
			t.Fatalf("HTTP %d 应报错", code)
		}
		// 401/403 统一口径：key 无效、过期、额度用尽都在文案内
		for _, want := range []string{"无效", "过期", "额度用尽"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("HTTP %d 错误文案缺 %q: %v", code, want, err)
			}
		}
	}
}

func TestBaiModelsEmptyKey(t *testing.T) {
	r := NewBaiRepo()
	if _, err := r.Models("  "); err == nil || !strings.Contains(err.Error(), "未配置 API Key") {
		t.Errorf("空 key 应显式失败: %v", err)
	}
}

func TestBaiModelsNonJSON200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>gateway busy</html>`))
	}))
	defer ts.Close()
	r := NewBaiRepo()
	r.BaseURL = ts.URL
	if _, err := r.Models("sk-x"); err == nil || !strings.Contains(err.Error(), "BAI 模型清单") {
		t.Errorf("非 JSON 200 应报解析错误: %v", err)
	}
}

// ── 积分额度通道（chat.b.ai tRPC） ──────────────────────────────

const (
	baiPointsOK   = `{"result":{"data":{"json":{"points_balance":27166591,"points_expiring":7166591}}}}`
	baiSummaryOK  = `{"result":{"data":{"json":{"monthly_spent":2833409,"points_balance":27166591}}}}`
	baiUnauthTRPC = `{"error":{"json":{"message":"UNAUTHORIZED","code":-32001,"data":{"code":"UNAUTHORIZED","httpStatus":401,"path":"usage.points"}}}}`
)

// newBaiConsole 按路径分发的控制台桩；statuses 缺省一律 200。
func newBaiConsole(t *testing.T, bodies map[string]string, statuses map[string]int, seen *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.Path+"|"+r.Header.Get("Authorization"))
		if s, ok := statuses[r.URL.Path]; ok {
			w.WriteHeader(s)
		}
		w.Write([]byte(bodies[r.URL.Path]))
	}))
}

func baiRepoAt(url string, seen *[]string) *BaiRepo {
	return &BaiRepo{BaseURL: url, ConsoleURL: url, Client: http.DefaultClient}
}

func TestBaiPointsWireFormat(t *testing.T) {
	var seen []string
	ts := newBaiConsole(t, map[string]string{
		"/trpc/lambda/usage.points":  baiPointsOK,
		"/trpc/lambda/usage.summary": baiSummaryOK,
	}, nil, &seen)
	defer ts.Close()

	pts, err := baiRepoAt(ts.URL, &seen).Points("sk-baitest123")
	if err != nil {
		t.Fatalf("Points: %v", err)
	}
	if pts.Balance != 27166591 || pts.Expiring != 7166591 || pts.MonthlySpent != 2833409 || !pts.HasMonthly {
		t.Errorf("额度合并结果不符: %+v", pts)
	}
	want := []string{
		"/trpc/lambda/usage.points|Bearer sk-baitest123",
		"/trpc/lambda/usage.summary|Bearer sk-baitest123",
	}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Errorf("请求路径/认证头不符:\n got=%v\nwant=%v", seen, want)
	}
}

func TestBaiPointsAuthError(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"HTTP 401", `{"error":{"message":"Invalid token"}}`},
		{"tRPC 信封", baiUnauthTRPC},
	} {
		var seen []string
		ts := newBaiConsole(t, map[string]string{"/trpc/lambda/usage.points": tc.body},
			map[string]int{"/trpc/lambda/usage.points": http.StatusUnauthorized}, &seen)
		_, err := baiRepoAt(ts.URL, &seen).Points("sk-bad")
		ts.Close()
		if err == nil || !strings.Contains(err.Error(), "无效、已过期或额度用尽") {
			t.Errorf("%s: 应归一到凭据口径，实际 %v", tc.name, err)
		}
	}
}

// summary 是可选信息：它失败不得抖掉已拿到的余额，也不得报错。
func TestBaiPointsSummaryFailureNotFatal(t *testing.T) {
	var seen []string
	ts := newBaiConsole(t, map[string]string{
		"/trpc/lambda/usage.points":  baiPointsOK,
		"/trpc/lambda/usage.summary": `{"error":{"json":{"message":"boom","data":{"code":"INTERNAL_SERVER_ERROR"}}}}`,
	}, map[string]int{"/trpc/lambda/usage.summary": http.StatusInternalServerError}, &seen)
	defer ts.Close()

	pts, err := baiRepoAt(ts.URL, &seen).Points("sk-ok")
	if err != nil {
		t.Fatalf("summary 失败不应报错: %v", err)
	}
	if pts.Balance != 27166591 || pts.HasMonthly {
		t.Errorf("应保留余额且不标记月度消耗可用: %+v", pts)
	}
}

func TestBaiPointsEmptyKeyAndMissingBalance(t *testing.T) {
	var seen []string
	if _, err := (&BaiRepo{}).Points("   "); err == nil || !strings.Contains(err.Error(), "未配置 API Key") {
		t.Errorf("空 key 应显式失败: %v", err)
	}
	ts := newBaiConsole(t, map[string]string{"/trpc/lambda/usage.points": `{"result":{"data":{"json":{}}}}`}, nil, &seen)
	defer ts.Close()
	if _, err := baiRepoAt(ts.URL, &seen).Points("sk-x"); err == nil || !strings.Contains(err.Error(), "points_balance") {
		t.Errorf("缺余额字段应显式失败（不得显示 0）: %v", err)
	}
}

// ConsoleURL 缺席时退回真实默认端点（生产路径由 NewBaiRepo 赋值，此处锁住退化行为）
func TestBaiPointsDefaultConsoleURL(t *testing.T) {
	var got string
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		got = req.URL.String()
		return nil, errors.New("stop before network")
	})
	r := &BaiRepo{BaseURL: "http://127.0.0.1:1", Client: &http.Client{Transport: rt}}
	if _, err := r.Points("sk-x"); err == nil {
		t.Fatal("桩 transport 应让请求失败")
	}
	if want := "https://chat.b.ai/trpc/lambda/usage.points"; got != want {
		t.Errorf("空 ConsoleURL 应退回默认端点:\n got=%s\nwant=%s", got, want)
	}
	if NewBaiRepo().ConsoleURL != "https://chat.b.ai" {
		t.Errorf("NewBaiRepo 应默认 chat.b.ai: %q", NewBaiRepo().ConsoleURL)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// baiRecordsEnvelope 单页 records 响应（n 条记录，指定 has_more）
func baiRecordsEnvelope(n int, more bool, startMin int) string {
	var rows []string
	for i := 0; i < n; i++ {
		min := startMin + i
		rows = append(rows, `{"id":"api_r`+strings.Repeat("x", 1)+`_`+fmtInt(min)+`","model":"glm-5.3-flash","created_at":"2026-09-06T05:`+twoDigit(min)+`:00.000Z","input_tokens":100,"output_tokens":10,"total_tokens":110,"cost_points":0,"source_type":"api"}`)
	}
	return `{"result":{"data":{"json":{"data":[` + strings.Join(rows, ",") + `],"has_more":` + itoaB(more) + `}}}}`
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
func itoaB(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
func twoDigit(n int) string {
	s := fmtInt(n)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func TestBaiRecordsWireFormat(t *testing.T) {
	var gotAuth, gotRawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotRawQuery = r.URL.RawQuery
		w.Write([]byte(baiRecordsEnvelope(1, false, 1)))
	}))
	defer ts.Close()

	r := &BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}
	recs, more, err := r.Records("sk-baitest123", 2)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if gotAuth != "Bearer sk-baitest123" {
		t.Errorf("认证头不符: %q", gotAuth)
	}
	// input 参数必须含 page=2&pageSize=100（URL 编码后）
	if !strings.Contains(gotRawQuery, "page") || !strings.Contains(gotRawQuery, "pageSize") {
		t.Errorf("input 参数不符: %q", gotRawQuery)
	}
	if strings.Contains(gotRawQuery, "%7B%22page%22%3A1") {
		t.Errorf("page 应为传入值 2: %q", gotRawQuery)
	}
	if len(recs) != 1 || more {
		t.Errorf("返回不符: %d 条, more=%v", len(recs), more)
	}
}

func TestBaiStatsPagesUntilDone(t *testing.T) {
	pages := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		// 每页 2 条，第 3 页终止 → 应翻 3 页共 6 条
		w.Write([]byte(baiRecordsEnvelope(2, pages < 3, pages)))
	}))
	defer ts.Close()

	r := &BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}
	s, err := r.Stats("sk-baitest123")
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if pages != 3 {
		t.Errorf("应翻 3 页，实际 %d", pages)
	}
	if !s.Complete || s.RecordsFetched != 6 || s.TotalRequests != 6 {
		t.Errorf("聚合不符: complete=%v fetched=%d total=%d", s.Complete, s.RecordsFetched, s.TotalRequests)
	}
	if len(s.PerModel) != 1 || s.PerModel[0].Model != "glm-5.3-flash" || s.PerModel[0].Requests != 6 {
		t.Errorf("模型聚合不符: %+v", s.PerModel)
	}
	if s.TotalTokens != 660 {
		t.Errorf("token 合计不符: %d", s.TotalTokens)
	}
}

func TestBaiStatsTruncatesAtMaxPages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 永远 has_more=true → 达上限截断
		w.Write([]byte(baiRecordsEnvelope(100, true, 1)))
	}))
	defer ts.Close()

	r := &BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}
	s, err := r.Stats("sk-baitest123")
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if s.Complete {
		t.Errorf("达上限应 Complete=false")
	}
	if s.RecordsFetched != baiStatsMaxPages*baiStatsPageSize {
		t.Errorf("应拉满 %d 条，实际 %d", baiStatsMaxPages*baiStatsPageSize, s.RecordsFetched)
	}
}

func TestBaiStatsMidwayFailureKeepsPartial(t *testing.T) {
	pages := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		if pages == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"json":{"message":"boom","data":{"code":"INTERNAL_SERVER_ERROR"}}}}`))
			return
		}
		w.Write([]byte(baiRecordsEnvelope(2, pages < 3, pages)))
	}))
	defer ts.Close()

	r := &BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}
	s, err := r.Stats("sk-baitest123")
	if err == nil {
		t.Fatalf("中途页失败应返回错误")
	}
	if s.RecordsFetched != 2 || s.Complete {
		t.Errorf("应保留第 1 页部分数据且标截断: fetched=%d complete=%v", s.RecordsFetched, s.Complete)
	}
	if !strings.Contains(err.Error(), "第 2 页") {
		t.Errorf("错误应标注中断页: %v", err)
	}
}

func TestBaiStatsFirstPageFailureIsFatal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":{"json":{"message":"UNAUTHORIZED","data":{"code":"UNAUTHORIZED"}}}}`))
	}))
	defer ts.Close()

	r := &BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}
	_, err := r.Stats("sk-baitest123")
	if err == nil || !strings.Contains(err.Error(), "无效、已过期或额度用尽") {
		t.Errorf("首页失败应整体失败并归一认证文案: %v", err)
	}
}

func TestBaiStatsEmptyKey(t *testing.T) {
	r := &BaiRepo{BaseURL: "http://x", ConsoleURL: "http://x"}
	if _, _, err := r.Records("  ", 1); err == nil || !strings.Contains(err.Error(), "未配置 API Key") {
		t.Errorf("空 key 应显式失败: %v", err)
	}
}

// 服务器响应体带 ANSI/控制字符 → doGet 错误消息必须已消毒（全仓统一 SanitizeText）
func TestDoGetSanitizesHTTPErrorBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom \x1b[31mred\x1b[0m inject\rFAKE\x07"))
	}))
	defer ts.Close()

	r := &BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}
	// 401/403 走固定文案，500 带响应体片段 → 用 500 触发
	body, err := doGet(r.client(), ts.URL+"/v1/models", map[string]string{}, "auth")
	if err == nil {
		t.Fatalf("期望错误，body=%q", body)
	}
	msg := err.Error()
	for _, bad := range []string{"\x1b", "\r", "\x07"} {
		if strings.Contains(msg, bad) {
			t.Errorf("HTTP 错误消息含控制字符 %q: %q", bad, msg)
		}
	}
	if !strings.Contains(msg, "boom red inject") {
		t.Errorf("正文应保留: %q", msg)
	}
}

// 探活 wire format + 状态码语义
func TestBaiProbeFreeFlash(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	var auths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		auths = append(auths, r.Header.Get("Authorization"))
		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(string(b), "glm-5.3-flash") {
			// 正常推理 200
			w.Write([]byte(`{"id":"x","object":"chat.completion","choices":[{"message":{"role":"assistant","content":""}}],"usage":{"total_tokens":10}}`))
			return
		}
		// deepseek 模拟运行时 503（2026-09-06 实测故障形状）
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":{"message":"pre_consume_token_quota_failed","type":"gateway"}}`))
	}))
	defer ts.Close()

	r := &BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}
	probes, err := r.ProbeFreeFlash("sk-baitest123", []string{"deepseek-v4-flash", "glm-5.3-flash"})
	if err != nil {
		t.Fatalf("ProbeFreeFlash: %v", err)
	}
	if len(probes) != 2 {
		t.Fatalf("应 2 条: %+v", probes)
	}
	mu.Lock()
	defer mu.Unlock()
	for i, b := range bodies {
		if !strings.Contains(b, `"max_tokens":8`) || !strings.Contains(b, `"stream":false`) {
			t.Errorf("请求 %d 缺 max_tokens=8/stream=false: %s", i, b)
		}
		if auths[i] != "Bearer sk-baitest123" {
			t.Errorf("请求 %d 认证头不符: %q", i, auths[i])
		}
	}
	// deepseek 503 → dead + 摘要含上游 message
	if probes[0].Alive {
		t.Errorf("503 应不 Alive")
	}
	if !strings.Contains(probes[0].Detail, "HTTP 503") || !strings.Contains(probes[0].Detail, "pre_consume_token_quota_failed") {
		t.Errorf("故障摘要应含状态码与上游消息: %q", probes[0].Detail)
	}
	// glm 200 → alive
	if !probes[1].Alive || probes[1].Detail != "" {
		t.Errorf("200 应 Alive 无摘要: %+v", probes[1])
	}
	// 结果按入参顺序
	if probes[0].Model != "deepseek-v4-flash" || probes[1].Model != "glm-5.3-flash" {
		t.Errorf("顺序应与入参一致: %+v", probes)
	}
}

func TestBaiProbeAuthErrorAndEmptyKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid token"}}`))
	}))
	defer ts.Close()

	r := &BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}
	probes, err := r.ProbeFreeFlash("sk-x", []string{"qwen3.8-flash"})
	if err != nil {
		t.Fatalf("ProbeFreeFlash: %v", err)
	}
	if probes[0].Alive || !strings.Contains(probes[0].Detail, "无效、已过期或额度用尽") {
		t.Errorf("401 应归一认证文案: %+v", probes[0])
	}
	if _, err := r.ProbeFreeFlash("  ", []string{"m"}); err == nil {
		t.Errorf("空 key 应显式失败")
	}
}
