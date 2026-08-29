package repo

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
)

// galaxyTestAccount 测试凭据（假值，非真实密钥）
func galaxyTestAccount() models.GalaxyAccount {
	return models.GalaxyAccount{ID: "g1", Name: "测试", AccessKey: "ak-test", SecretKey: "sk-test"}
}

// galaxyFixedNow 固定时钟：timestamp 与到期折算都依赖它。
// 必须用 time.Local —— 余额变更的 CreateTime 按本地时区解析，跨区比较会把
// 「今天」的记录算进昨天。
var galaxyFixedNow = time.Date(2026, 8, 29, 18, 0, 0, 0, time.Local)

// galaxyOK 包一层成功信封
func galaxyOK(data string) string {
	return `{"success":true,"code":"2000","message":"","data":` + data + `}`
}

// mustHandler 校验签名与公共参数后回业务 body（服务端复算 = 与平台同一套算法）
func mustHandler(t *testing.T, body func(r *http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("表单解析失败: %v", err)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			t.Errorf("Content-Type 应为表单编码: %s", ct)
		}
		for _, k := range []string{"apikey", "timestamp", "nonce", "sign"} {
			if r.Form.Get(k) == "" {
				t.Errorf("缺少公共参数 %s", k)
			}
		}
		params := map[string]string{}
		for k, v := range r.Form {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}
		got := params["sign"]
		delete(params, "sign")
		if want := parsers.GalaxySign(params, "sk-test"); got != want {
			t.Errorf("签名校验失败: got %s want %s (params=%v)", got, want, params)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body(r)))
	}
}

func newGalaxyRepo(t *testing.T, srv *httptest.Server) *GalaxyRepo {
	t.Helper()
	return &GalaxyRepo{BaseURL: srv.URL, Client: srv.Client(), Now: func() time.Time { return galaxyFixedNow }}
}

// ── 余额 ───────────────────────────────────────────────────────

func TestGalaxyBalanceOK(t *testing.T) {
	srv := httptest.NewServer(mustHandler(t, func(r *http.Request) string {
		if r.URL.Path != "/account/get_main_account_info" {
			t.Errorf("路径不符: %s", r.URL.Path)
		}
		return galaxyOK(readFixture(t, "galaxy_account_info.json"))
	}))
	defer srv.Close()

	bal, err := newGalaxyRepo(t, srv).Balance(galaxyTestAccount())
	if err != nil {
		t.Fatal(err)
	}
	if bal.Money != 96.2805 || bal.Phone != "138****1111" {
		t.Errorf("余额解析不符: %+v", bal)
	}
}

// TestGalaxyEnvelopeErrors HTTP 恒 200、错误在信封里 —— 逐条映射成可操作中文
func TestGalaxyEnvelopeErrors(t *testing.T) {
	cases := []struct {
		Envelope string
		Want     string
	}{
		{`{"success":false,"code":"4000","message":"accesskey不存在!","data":null}`, "AccessKey 无效或已删除"},
		{`{"success":false,"code":"4000","message":"sign验证失败!","data":null}`, "签名校验失败"},
		{`{"success":false,"code":"4000","message":"请先完成实名认证","data":null}`, "实名认证"},
		{`{"success":false,"code":"4000","message":"时间戳错误","data":null}`, "本机时钟不准"},
		{`{"success":false,"code":"4000","message":"page_size参数超限!","data":null}`, "page_size参数超限"},
		{`{"success":false,"code":"4000","message":"","data":null}`, "code=4000"},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tc.Envelope))
		}))
		_, err := newGalaxyRepo(t, srv).Balance(galaxyTestAccount())
		srv.Close()
		if err == nil {
			t.Fatalf("信封 success=false 应报错: %s", tc.Envelope)
		}
		if !strings.Contains(err.Error(), tc.Want) {
			t.Errorf("错误文案应含 %q，实得 %q", tc.Want, err.Error())
		}
	}
}

func TestGalaxyCodeNumeric(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"code":2000,"message":"","data":{"Money":1,"Name":"n","Phone":""}}`))
	}))
	defer srv.Close()
	if _, err := newGalaxyRepo(t, srv).Balance(galaxyTestAccount()); err != nil {
		t.Errorf("code 回数字形式应容忍: %v", err)
	}
}

func TestGalaxyNilData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"code":"2000","message":"","data":null}`))
	}))
	defer srv.Close()
	_, err := newGalaxyRepo(t, srv).Balance(galaxyTestAccount())
	if err == nil || !strings.Contains(err.Error(), "data") {
		t.Errorf("data=null 应报错，实得 %v", err)
	}
}

func TestGalaxyHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<html>gateway</html>"))
	}))
	defer srv.Close()
	_, err := newGalaxyRepo(t, srv).Balance(galaxyTestAccount())
	if err == nil || !strings.HasPrefix(err.Error(), "HTTP 502") {
		t.Errorf("非 2xx 应报 HTTP 码，实得 %v", err)
	}
}

func TestGalaxyBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()
	_, err := newGalaxyRepo(t, srv).Balance(galaxyTestAccount())
	if err == nil || !strings.Contains(err.Error(), "响应格式错误") {
		t.Errorf("非 JSON 响应应报格式错误，实得 %v", err)
	}
}

// TestGalaxyMissingCredentials 缺凭据必须直接失败且不发请求（别拿空 ak 打平台）
func TestGalaxyMissingCredentials(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	r := newGalaxyRepo(t, srv)
	if _, err := r.Balance(models.GalaxyAccount{AccessKey: "only-ak"}); err == nil {
		t.Error("缺 SecretKey 应失败")
	}
	if _, err := r.StatusCount(models.GalaxyAccount{}); err == nil {
		t.Error("空凭据应失败")
	}
	if hits != 0 {
		t.Errorf("缺凭据不该发请求，实发 %d 次", hits)
	}
}

// ── 实例列表 ───────────────────────────────────────────────────

func TestGalaxyStatusCountOK(t *testing.T) {
	srv := httptest.NewServer(mustHandler(t, func(r *http.Request) string {
		if r.URL.Path != "/instance/get_instance_status_count" {
			t.Errorf("路径不符: %s", r.URL.Path)
		}
		return galaxyOK(readFixture(t, "galaxy_status_count.json"))
	}))
	defer srv.Close()
	s, err := newGalaxyRepo(t, srv).StatusCount(galaxyTestAccount())
	if err != nil {
		t.Fatal(err)
	}
	if s.All != 85 || s.Running != 4 {
		t.Errorf("统计不符: %+v", s)
	}
}

// TestGalaxyInstancesLimit pageSize 取 limit，够数即停，不再翻第二页
func TestGalaxyInstancesLimit(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(mustHandler(t, func(r *http.Request) string {
		if r.URL.Path != "/instance/get_instance_list" {
			t.Errorf("路径不符: %s", r.URL.Path)
		}
		pages = append(pages, r.FormValue("page")+"/"+r.FormValue("page_size")+"/"+r.FormValue("status_type"))
		return galaxyOK(readFixture(t, "galaxy_instance_list.json"))
	}))
	defer srv.Close()
	list, err := newGalaxyRepo(t, srv).Instances(galaxyTestAccount(), GalaxyStatusDefault, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("应截断到 limit=2: got %d", len(list))
	}
	if len(pages) != 1 || pages[0] != "1/2/statusDefault" {
		t.Errorf("应只请求一页且带上过滤: %v", pages)
	}
}

// TestGalaxyInstancesPageSizeClamped 平台 page_size>100 直接报错，客户端必须自己夹住
func TestGalaxyInstancesPageSizeClamped(t *testing.T) {
	var seen string
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if hits > 6 {
			w.Write([]byte(`{"success":true,"code":"2000","message":"","data":{"list":[],"has_more":false,"total_count":0}}`))
			return
		}
		seen = r.FormValue("page_size")
		w.Write([]byte(galaxyOK(`{"list":[],"has_more":true,"total_count":999}`)))
	}))
	defer srv.Close()
	if _, err := newGalaxyRepo(t, srv).Instances(galaxyTestAccount(), GalaxyStatusAll, 500); err != nil {
		t.Fatal(err)
	}
	if seen != "100" {
		t.Errorf("page_size 应夹到 100，实得 %q", seen)
	}
	if hits > 3 {
		t.Errorf("翻页数应受 InstancePages 限制，实发 %d", hits)
	}
}

// ── 消耗 ───────────────────────────────────────────────────────

// TestGalaxyCostStopsWhenWindowCovered 第二页出现早于 7 天下界的记录 → 停止翻页
func TestGalaxyCostStopsWhenWindowCovered(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(mustHandler(t, func(r *http.Request) string {
		if r.URL.Path != "/billing/get_balance_change_list" {
			t.Errorf("路径不符: %s", r.URL.Path)
		}
		atomic.AddInt32(&hits, 1)
		page := r.FormValue("page")
		if page == "1" {
			return galaxyOK(`{"list":[{"CreateTime":"2026-08-29 10:00:00","DiffMoney":-1,"DiffPower":0,"MoneyLeft":9,"Remark":"续费"}],"has_more":true,"total_count":500}`)
		}
		return galaxyOK(`{"list":[{"CreateTime":"2026-08-20 10:00:00","DiffMoney":-2,"DiffPower":0,"MoneyLeft":7,"Remark":"很久以前"}],"has_more":true,"total_count":500}`)
	}))
	defer srv.Close()
	cost, err := newGalaxyRepo(t, srv).Cost(galaxyTestAccount())
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Errorf("窗口取完应停止翻页，实发 %d 次", hits)
	}
	if cost.Today != 1 {
		t.Errorf("今日消耗只该含今日条目: got %v", cost.Today)
	}
	if cost.TodayPartial || cost.WeekPartial {
		t.Errorf("已翻到窗口下界外仍标下限: %+v", cost)
	}
}

func TestGalaxyCostPageCap(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte(galaxyOK(`{"list":[{"CreateTime":"2026-08-29 10:00:00","DiffMoney":-1,"DiffPower":0,"MoneyLeft":9,"Remark":"x"}],"has_more":true,"total_count":9999}`)))
	}))
	defer srv.Close()
	r := newGalaxyRepo(t, srv)
	r.CostPages = 2
	cost, err := r.Cost(galaxyTestAccount())
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Errorf("应受 CostPages 上限约束: hits=%d", hits)
	}
	if !cost.TodayPartial || !cost.WeekPartial {
		t.Errorf("翻不完时两个窗口都该标下限: %+v", cost)
	}
}

// ── nonce 与时间戳 ─────────────────────────────────────────────

func TestGalaxyNonceUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		n := galaxyRandomNonce(12)
		if len(n) != 12 {
			t.Fatalf("nonce 长度不符: %q", n)
		}
		if seen[n] {
			t.Fatalf("nonce 重复（平台会拒绝重复随机串）: %s", n)
		}
		seen[n] = true
	}
}

func TestGalaxyTimestampFromInjectedClock(t *testing.T) {
	var ts string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		ts = r.FormValue("timestamp")
		w.Write([]byte(galaxyOK(`{"Money":1,"Name":"n","Phone":""}`)))
	}))
	defer srv.Close()
	if _, err := newGalaxyRepo(t, srv).Balance(galaxyTestAccount()); err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("%d", galaxyFixedNow.Unix()); ts != want {
		t.Errorf("timestamp 应取注入时钟: got %s want %s", ts, want)
	}
}

// TestGalaxyRequestWireFormat 线上格式金标准：固定时钟 + 固定 nonce 下，
// body 必须只含 4 个公共参数，且 sign = 独立复算（手工拼串 + MD5）的结果。
// 刻意不复用 parsers.GalaxySign——自证式的服务端复算只验自洽，拼串顺序变了也测不出。
func TestGalaxyRequestWireFormat(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body = readBodyString(t, r)
		w.Write([]byte(galaxyOK(`{"Money":1,"Name":"n","Phone":""}`)))
	}))
	defer srv.Close()

	r := newGalaxyRepo(t, srv)
	r.Nonce = func() string { return "testnonce0001" }
	if _, err := r.Balance(galaxyTestAccount()); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"apikey":    "ak-test",
		"nonce":     "testnonce0001",
		"timestamp": fmt.Sprintf("%d", galaxyFixedNow.Unix()),
	}
	form := parseFormString(t, body)
	if len(form) != len(want)+1 {
		t.Fatalf("body 参数集合不符（应只有 4 个公共参数）: %v", form)
	}
	for k, v := range want {
		if form[k] != v {
			t.Errorf("参数 %s = %q, want %q", k, form[k], v)
		}
	}
	independent := md5Hex("apikey=ak-test&nonce=testnonce0001&timestamp=" + want["timestamp"] + "&secret=sk-test")
	if form["sign"] != independent {
		t.Errorf("sign 与独立复算不符: got %s want %s", form["sign"], independent)
	}
}

// ── 测试小工具（独立实现，不复用被测代码） ────────────────────

func readBodyString(t *testing.T, r *http.Request) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func parseFormString(t *testing.T, raw string) map[string]string {
	t.Helper()
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("body 不是合法表单编码: %v (%s)", err, raw)
	}
	out := map[string]string{}
	for k := range v {
		out[k] = v.Get(k)
	}
	return out
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
