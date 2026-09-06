package repo

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
