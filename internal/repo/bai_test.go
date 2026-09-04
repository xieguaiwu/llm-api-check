package repo

import (
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
