package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/repo"
)

const baiModelsResp = `{"data":[{"id":"qwen3.8-flash","object":"model","owned_by":"qwen"},` +
	`{"id":"deepseek-v4-flash","object":"model","owned_by":"deepseek"}],"object":"list","success":true}`

const (
	baiPointsResp  = `{"result":{"data":{"json":{"points_balance":27166591,"points_expiring":7166591}}}}`
	baiSummaryResp = `{"result":{"data":{"json":{"monthly_spent":2833409,"points_balance":27166591}}}}`
	baiUnauthBody  = `{"error":{"json":{"message":"UNAUTHORIZED","code":-32001,"data":{"code":"UNAUTHORIZED","httpStatus":401,"path":"usage.points"}}}}`
)

type baiStubResp struct {
	status int
	body   string
}

// newBaiServer 按路径分发：/v1/models 走推理面，/trpc/lambda/usage.* 走控制台。
// 未列出的路径回 500（该通道视为失败），保证测试不隐式依赖真实网络。
func newBaiServer(t *testing.T, responses map[string]baiStubResp, count *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		resp, ok := responses[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"json":{"message":"unstubbed path","data":{"code":"INTERNAL_SERVER_ERROR"}}}}`))
			return
		}
		if resp.status != 0 {
			w.WriteHeader(resp.status)
		}
		w.Write([]byte(resp.body))
	}))
}

// baiAllOK 三路全部正常的桩响应
func baiAllOK() map[string]baiStubResp {
	return map[string]baiStubResp{
		"/v1/models":                 {body: baiModelsResp},
		"/trpc/lambda/usage.points":  {body: baiPointsResp},
		"/trpc/lambda/usage.summary": {body: baiSummaryResp},
	}
}

func newBaiApp(ts *httptest.Server, cfg *config.Config) *App {
	return NewWithRepos(cfg, &Repos{Bai: &repo.BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL}})
}

func TestRefreshBaiHappy(t *testing.T) {
	var count atomic.Int32
	ts := newBaiServer(t, baiAllOK(), &count)
	defer ts.Close()

	cfg := &config.Config{BaiAccounts: []models.BaiAccount{{ID: "b1", Name: "免费通道", ApiKey: "sk-bai1"}}}
	res, err := newBaiApp(ts, cfg).RefreshBai("b1")
	if err != nil {
		t.Fatalf("RefreshBai: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("不应有错误: %s", res.Error)
	}
	if res.Plan == nil || len(res.Plan.Models) != 2 {
		t.Fatalf("应有 2 个模型: %+v", res.Plan)
	}
	if m := res.Plan.MissingFreeFlash(); len(m) != 2 {
		t.Errorf("fixture 缺 vision-exp 与 glm-5.3-flash，应报 2 缺失: %v", m)
	}
	if res.Points == nil {
		t.Fatal("应有积分额度")
	}
	if *res.Points != (models.BaiPoints{Balance: 27166591, Expiring: 7166591, MonthlySpent: 2833409, HasMonthly: true}) {
		t.Errorf("积分额度不符: %+v", *res.Points)
	}
	if count.Load() < 3 {
		t.Errorf("模型/额度/消耗三路应各发一次请求，实际 %d 次", count.Load())
	}
}

func TestRefreshBaiAuthError(t *testing.T) {
	var count atomic.Int32
	auth := baiStubResp{status: http.StatusUnauthorized, body: `{"error":{"message":"Invalid token"}}`}
	ts := newBaiServer(t, map[string]baiStubResp{
		"/v1/models":                 auth,
		"/trpc/lambda/usage.points":  auth,
		"/trpc/lambda/usage.summary": auth,
	}, &count)
	defer ts.Close()

	cfg := &config.Config{BaiAccounts: []models.BaiAccount{{ID: "b1", Name: "免费通道", ApiKey: "sk-bai1"}}}
	res, err := newBaiApp(ts, cfg).RefreshBai("b1")
	if err != nil {
		t.Fatalf("RefreshBai 不应向上抛账号级错误: %v", err)
	}
	if res.Plan != nil || res.Points != nil {
		t.Errorf("失败时不应有数据: plan=%+v points=%+v", res.Plan, res.Points)
	}
	// 三路共用一把 key：同一句凭据错误只应出现一次（去重，同 joinText 教训）
	if n := strings.Count(res.Error, "无效、已过期或额度用尽"); n != 1 {
		t.Errorf("同文本错误应去重，实际出现 %d 次: %q", n, res.Error)
	}
}

// 一路失败不抖掉另一路：额度正常、模型清单挂 → 仍显示额度 + 保留错误
func TestRefreshBaiPartialFailureKeepsBothLanes(t *testing.T) {
	var count atomic.Int32
	cases := []struct {
		name       string
		responses  map[string]baiStubResp
		wantPlan   bool
		wantPoints bool
	}{
		{"模型失败", map[string]baiStubResp{
			"/v1/models":                 {status: http.StatusForbidden, body: `{"message":"nope","success":false}`},
			"/trpc/lambda/usage.points":  {body: baiPointsResp},
			"/trpc/lambda/usage.summary": {body: baiSummaryResp},
		}, false, true},
		{"额度失败", baiAllOK2(map[string]baiStubResp{
			"/trpc/lambda/usage.points": {status: http.StatusUnauthorized, body: baiUnauthBody},
		}), true, false},
	}
	for _, tc := range cases {
		cfg := &config.Config{BaiAccounts: []models.BaiAccount{{ID: "b1", Name: "免费通道", ApiKey: "sk-bai1"}}}
		res, err := newBaiApp(newBaiServer(t, tc.responses, &count), cfg).RefreshBai("b1")
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if (res.Plan != nil) != tc.wantPlan {
			t.Errorf("%s: plan 存在性应为 %v: %+v", tc.name, tc.wantPlan, res.Plan)
		}
		if (res.Points != nil) != tc.wantPoints {
			t.Errorf("%s: points 存在性应为 %v: %+v", tc.name, tc.wantPoints, res.Points)
		}
		if res.Error == "" {
			t.Errorf("%s: 失败通道应留下错误", tc.name)
		}
	}
}

// baiAllOK2 在全路正常的响应上覆盖指定路径
func baiAllOK2(overrides map[string]baiStubResp) map[string]baiStubResp {
	out := baiAllOK()
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

// summary 缺席（可选信息）不得产生错误行
func TestRefreshBaiSummaryOptional(t *testing.T) {
	var count atomic.Int32
	ts := newBaiServer(t, baiAllOK2(map[string]baiStubResp{
		"/trpc/lambda/usage.summary": {status: http.StatusInternalServerError, body: `{"message":"boom"}`},
	}), &count)
	defer ts.Close()

	cfg := &config.Config{BaiAccounts: []models.BaiAccount{{ID: "b1", Name: "免费通道", ApiKey: "sk-bai1"}}}
	res, err := newBaiApp(ts, cfg).RefreshBai("b1")
	if err != nil {
		t.Fatalf("RefreshBai: %v", err)
	}
	if res.Error != "" {
		t.Errorf("可选通道失败不应报错: %q", res.Error)
	}
	if res.Points == nil || res.Points.HasMonthly || res.Points.Balance != 27166591 {
		t.Errorf("应保留余额且不标记月度消耗: %+v", res.Points)
	}
}

func TestRefreshBaiAccountNotFound(t *testing.T) {
	apps := NewWithRepos(&config.Config{}, &Repos{Bai: repo.NewBaiRepo()})
	if _, err := apps.RefreshBai("ghost"); err == nil || !strings.Contains(err.Error(), "账号不存在") {
		t.Errorf("账号不存在应报错: %v", err)
	}
}

func TestRefreshAllIncludesBai(t *testing.T) {
	var count atomic.Int32
	ts := newBaiServer(t, baiAllOK(), &count)
	defer ts.Close()

	cfg := &config.Config{
		BaiAccounts: []models.BaiAccount{{ID: "b1", Name: "免费通道", ApiKey: "sk-bai1"}},
	}
	apps := NewWithRepos(cfg, &Repos{
		DeepSeek: repo.NewDeepSeekRepo(),
		OpenCode: repo.NewOpenCodeRepo(),
		Qwen:     repo.NewQwenRepo(),
		Galaxy:   repo.NewGalaxyRepo(),
		Bai:      &repo.BaiRepo{BaseURL: ts.URL, ConsoleURL: ts.URL},
	})
	res, err := apps.RefreshAll()
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if len(res.Bai) != 1 || res.Bai[0].Plan == nil || res.Bai[0].Points == nil {
		t.Fatalf("RefreshAll 应收编 bai 账号两路数据: %+v", res.Bai)
	}
}
