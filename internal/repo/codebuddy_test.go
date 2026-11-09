package repo

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

const (
	cbAccountsHappy = `{"code":0,"msg":"OK","data":{"accounts":[
		{"uid":"u-1","nickname":"alice","uin":"123","type":"personal",
		 "accountType":"","isAmbassador":false}]}}`
	cbConfigHappy = `{"code":0,"msg":"OK","data":{"models":[
		{"id":"default-model","name":"Auto","maxInputTokens":176000,
		 "maxOutputTokens":24000,"credits":"","isDefault":true},
		{"id":"fast-model","name":"Fast","maxInputTokens":200000,
		 "maxOutputTokens":32000,"credits":"x0.34 credits"}]}}`
	// 实测形状：无 key → APISIX HTML
	cbApisixHTML = `<html>
<head><title>401 Authorization Required</title></head>
<body><center><h1>401 Authorization Required</h1></center>
<hr><center>openresty</center></body>
</html>`
	// 实测形状：坏 key → JSON
	cbNotFoundJSON = `{"message":"not_found"}`
)

// cbServer 模拟 CodeBuddy 端点，记录收到的头。
type cbServer struct {
	*httptest.Server
	mu          []map[string]string
	accountsHit int
	configHit   int
}

func newCbServer(t *testing.T, accountsBody, configBody string, accountsCode, configCode int) *cbServer {
	t.Helper()
	s := &cbServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/accounts", func(w http.ResponseWriter, r *http.Request) {
		s.accountsHit++
		s.mu = append(s.mu, map[string]string{
			"apiKey": r.Header.Get("X-API-Key"),
			"uid":    r.Header.Get("X-User-Id"),
			"ua":     r.Header.Get("User-Agent"),
		})
		w.WriteHeader(accountsCode)
		_, _ = w.Write([]byte(accountsBody))
	})
	mux.HandleFunc("/v3/config", func(w http.ResponseWriter, r *http.Request) {
		s.configHit++
		s.mu = append(s.mu, map[string]string{
			"apiKey": r.Header.Get("X-API-Key"),
			"uid":    r.Header.Get("X-User-Id"),
			"ua":     r.Header.Get("User-Agent"),
		})
		w.WriteHeader(configCode)
		_, _ = w.Write([]byte(configBody))
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func TestCodebuddyAccountsHappy(t *testing.T) {
	srv := newCbServer(t, cbAccountsHappy, cbConfigHappy, 200, 200)
	r := &CodebuddyRepo{BaseURL: srv.URL, Client: srv.Client()}
	ids, err := r.Accounts("ck-test")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(ids) != 1 || ids[0].UID != "u-1" || ids[0].Nickname != "alice" {
		t.Fatalf("账号解析错: %+v", ids)
	}
	// 头契约：X-API-Key 必带；身份路不带 X-User-Id（此时还没有 uid）
	h := srv.mu[0]
	if h["apiKey"] != "ck-test" {
		t.Errorf("X-API-Key 未发送: %q", h["apiKey"])
	}
	if h["uid"] != "" {
		t.Errorf("身份路不应带 X-User-Id: %q", h["uid"])
	}
	if !strings.Contains(h["ua"], "CodeBuddy") {
		t.Errorf("UA 应带产品标识: %q", h["ua"])
	}
}

func TestCodebuddyAccountsAuthHTML(t *testing.T) {
	// 无 key 的实测形状：APISIX HTML 401
	srv := newCbServer(t, cbApisixHTML, "", 401, 200)
	r := &CodebuddyRepo{BaseURL: srv.URL, Client: srv.Client()}
	_, err := r.Accounts("")
	if !errors.Is(err, parsers.ErrCodebuddyAuth) {
		t.Fatalf("HTML 401 应归一为 ErrCodebuddyAuth, got %v", err)
	}
}

func TestCodebuddyAccountsAuthJSON(t *testing.T) {
	// 坏 key 的实测形状：{"message":"not_found"} 401
	srv := newCbServer(t, cbNotFoundJSON, "", 401, 200)
	r := &CodebuddyRepo{BaseURL: srv.URL, Client: srv.Client()}
	_, err := r.Accounts("ck-bad")
	if !errors.Is(err, parsers.ErrCodebuddyAuth) {
		t.Fatalf("JSON 401 应归一为 ErrCodebuddyAuth, got %v", err)
	}
}

func TestCodebuddyCatalogSendsUID(t *testing.T) {
	// 目录面的硬契约：X-User-Id 必带（缺失时服务端 data.models=null）
	srv := newCbServer(t, cbAccountsHappy, cbConfigHappy, 200, 200)
	r := &CodebuddyRepo{BaseURL: srv.URL, Client: srv.Client()}
	cat, err := r.Catalog("ck-test", "u-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(cat.Models) != 2 {
		t.Fatalf("got %d models, want 2", len(cat.Models))
	}
	h := srv.mu[0]
	if h["uid"] != "u-1" {
		t.Errorf("X-User-Id 未发送: %q", h["uid"])
	}
	if h["apiKey"] != "ck-test" {
		t.Errorf("X-API-Key 未发送: %q", h["apiKey"])
	}
}

func TestCodebuddyCatalogNullModels(t *testing.T) {
	// 服务端在 X-User-Id 缺失时回 data.models=null → 解析层显式报错
	srv := newCbServer(t, cbAccountsHappy, `{"code":0,"data":{"models":null}}`, 200, 200)
	r := &CodebuddyRepo{BaseURL: srv.URL, Client: srv.Client()}
	_, err := r.Catalog("ck-test", "")
	if err == nil {
		t.Fatal("models:null 应报错")
	}
	if !strings.Contains(err.Error(), "X-User-Id") {
		t.Errorf("错误应提示 X-User-Id: %v", err)
	}
}

func TestCodebuddyCatalogHTTPError(t *testing.T) {
	srv := newCbServer(t, cbAccountsHappy, "boom", 200, 500)
	r := &CodebuddyRepo{BaseURL: srv.URL, Client: srv.Client()}
	_, err := r.Catalog("ck-test", "u-1")
	if err == nil {
		t.Fatal("500 应报错")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("错误应含状态码: %v", err)
	}
}

// cbQuotaBody 实测响应形状（只保留额度包字段）。
const cbQuotaBody = `{"code":0,"msg":"OK","data":{"Response":{"Data":{"Accounts":[
	{"PackageName":"Plan Bonus Quota","PackageCode":"TCACA_code_037","Status":0,
	 "CycleCapacityRemain":570,"CycleCapacitySize":1000,"CycleCapacityUsed":429,
	 "CycleCapacityRemainPrecise":"570.06000063","CycleCapacitySizePrecise":"1000",
	 "CycleCapacityUsedPrecise":"429.93999937","CycleEndTime":"2026-12-04 21:10:06"}]}}}}`

func TestCodebuddyQuotaHappy(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cbQuotaBody))
	}))
	defer srv.Close()
	r := &CodebuddyRepo{BaseURL: srv.URL, Client: srv.Client()}
	q, err := r.Quota("ck-test")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if q.Remaining != 570.06000063 || q.Total != 1000 {
		t.Errorf("额度解析错: %+v", q)
	}
	if gotMethod != http.MethodPost || gotPath != "/v2/billing/meter/get-user-resource" {
		t.Errorf("请求契约错: %s %s", gotMethod, gotPath)
	}
	if gotAuth != "Bearer ck-test" {
		t.Errorf("Authorization 头错: %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"ProductCode":"p_tcaca"`) || !strings.Contains(gotBody, `"Status":[0,3]`) {
		t.Errorf("请求体缺关键字段: %s", gotBody)
	}
}

func TestCodebuddyQuotaAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(cbApisixHTML))
	}))
	defer srv.Close()
	r := &CodebuddyRepo{BaseURL: srv.URL, Client: srv.Client()}
	if _, err := r.Quota("ck-bad"); !errors.Is(err, parsers.ErrCodebuddyAuth) {
		t.Fatalf("401 应归一为 ErrCodebuddyAuth, got %v", err)
	}
}

func TestCodebuddyLedgerReadsInjectedRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "slug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"id":"r1","timestamp":` + itoaMs(time.Now()) + `,"providerData":{` +
		`"conversationRequestId":"c1","messageId":"m1","requestModelId":"model-a",` +
		`"rawUsage":{"prompt_tokens":10,"completion_tokens":2,"credit":0.42}}}`
	if err := os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &CodebuddyRepo{LedgerRoot: root, Client: http.DefaultClient}
	l, err := r.Ledger(time.Now())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if l.Root != root {
		t.Errorf("Root=%q, want %q", l.Root, root)
	}
	if l.Total.Calls != 1 || l.Total.Credits != 0.42 {
		t.Errorf("台账聚合错: calls=%d credits=%v", l.Total.Calls, l.Total.Credits)
	}
}

// itoaMs 十进制毫秒时间戳（避免引 strconv 只为测试）。
func itoaMs(t time.Time) string {
	v := t.UnixMilli()
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
