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

func newBaiServer(t *testing.T, count *atomic.Int32, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func TestRefreshBaiHappy(t *testing.T) {
	var count atomic.Int32
	ts := newBaiServer(t, &count, http.StatusOK, baiModelsResp)
	defer ts.Close()

	cfg := &config.Config{BaiAccounts: []models.BaiAccount{{ID: "b1", Name: "免费通道", ApiKey: "sk-bai1"}}}
	apps := NewWithRepos(cfg, &Repos{Bai: &repo.BaiRepo{BaseURL: ts.URL}})
	res, err := apps.RefreshBai("b1")
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
}

func TestRefreshBaiAuthError(t *testing.T) {
	var count atomic.Int32
	ts := newBaiServer(t, &count, http.StatusUnauthorized, `{"error":{"message":"Invalid token"}}`)
	defer ts.Close()

	cfg := &config.Config{BaiAccounts: []models.BaiAccount{{ID: "b1", Name: "免费通道", ApiKey: "sk-bai1"}}}
	apps := NewWithRepos(cfg, &Repos{Bai: &repo.BaiRepo{BaseURL: ts.URL}})
	res, err := apps.RefreshBai("b1")
	if err != nil {
		t.Fatalf("RefreshBai 不应向上抛账号级错误: %v", err)
	}
	if res.Error == "" || !strings.Contains(res.Error, "无效") {
		t.Errorf("错误应合并进结果: %q", res.Error)
	}
	if res.Plan != nil {
		t.Errorf("失败时不应有 Plan: %+v", res.Plan)
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
	ts := newBaiServer(t, &count, http.StatusOK, baiModelsResp)
	defer ts.Close()

	cfg := &config.Config{
		BaiAccounts: []models.BaiAccount{{ID: "b1", Name: "免费通道", ApiKey: "sk-bai1"}},
	}
	apps := NewWithRepos(cfg, &Repos{
		DeepSeek: repo.NewDeepSeekRepo(),
		OpenCode: repo.NewOpenCodeRepo(),
		Qwen:     repo.NewQwenRepo(),
		Galaxy:   repo.NewGalaxyRepo(),
		Bai:      &repo.BaiRepo{BaseURL: ts.URL},
	})
	res, err := apps.RefreshAll()
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if len(res.Bai) != 1 || res.Bai[0].Plan == nil {
		t.Fatalf("RefreshAll 应收编 bai 账号: %+v", res.Bai)
	}
}
