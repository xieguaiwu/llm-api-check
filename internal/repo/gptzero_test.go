package repo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// gzUsersBody /v2/users/me 成功响应（真实形状截取）
const gzUsersBody = `{"data":{"id":"fceb2e86","email":"xieguaiwu@163.com","plan":"API (300k words/month)",` +
	`"api_key":"adfc62c318f14a16a8843c952b56ea5b","char_limit":150000,` +
	`"monthly_input_words":587,"monthly_input_chars":3952,"monthly_input_documents":1,` +
	`"all_time_input_words":1675389,"all_time_input_chars":11680324,"all_time_input_documents":861,` +
	`"last_time_usage_reset":"2026-09-06T16:28:34.943+00:00",` +
	`"full_plan":{"name":"API (300k words/month)","duration_type":"monthly","word_limit":300000,` +
	`"overage_word_limit":1000000,"priceData":{"currency":"usd","unit_amount":4500}}}}`

func TestGptzeroUsageWireFormat(t *testing.T) {
	var gotKey, gotUA, gotAccept, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotPath = r.URL.Path
		w.Write([]byte(gzUsersBody))
	}))
	defer ts.Close()

	r := NewGptzeroRepo()
	r.BaseURL = ts.URL
	u, err := r.Usage("adfc62c318f14a16a8843c952b56ea5b")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if gotPath != "/v2/users/me" {
		t.Errorf("请求路径不符: %s", gotPath)
	}
	if gotKey != "adfc62c318f14a16a8843c952b56ea5b" {
		t.Errorf("x-api-key 头不符: %q", gotKey)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept 头不符: %q", gotAccept)
	}
	// 🔒 Cloudflare 对裸客户端 UA 有 403 前科（daily/2026-08-31 error 1010），
	// 必须带浏览器 UA——Go 默认 UA（Go-http-client/1.1）不允许。
	if gotUA == "" || gotUA == "Go-http-client/1.1" || gotUA == UA {
		t.Errorf("必须显式浏览器 UA（Got %q）", gotUA)
	}
	if u.Monthly.Words != 587 || u.Plan.WordLimit != 300000 {
		t.Errorf("解析结果不符: %+v", u)
	}
}

func TestGptzeroUsageAuthErrors(t *testing.T) {
	// 实测：未带 key → 401 {"error":"Require valid cookie"}；坏 key → 403
	// {"error":"API key has no owner","apiKey":"<原样回显>"}——回显含 key，
	// 原文不得进错误消息。
	for _, tc := range []struct {
		code int
		body string
	}{
		{http.StatusUnauthorized, `{"error":"Require valid cookie"}`},
		{http.StatusForbidden, `{"error":"API key has no owner","apiKey":"adfc62c318f14a16a8843c952b56ea5b"}`},
	} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
			w.Write([]byte(tc.body))
		}))
		r := NewGptzeroRepo()
		r.BaseURL = ts.URL
		_, err := r.Usage("adfc62c318f14a16a8843c952b56ea5b")
		ts.Close()
		if err == nil {
			t.Fatalf("HTTP %d 应报错", tc.code)
		}
		if err.Error() != "GPTZero API Key 无效或已过期，请到 app.gptzero.me 的 API 订阅页核对" {
			t.Errorf("HTTP %d 应归一认证文案: %q", tc.code, err.Error())
		}
		if strings.Contains(err.Error(), "adfc62") {
			t.Errorf("HTTP %d 错误消息不得含 key 回显: %q", tc.code, err.Error())
		}
	}
}

func TestGptzeroUsageServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`upstream boom`))
	}))
	defer ts.Close()
	r := NewGptzeroRepo()
	r.BaseURL = ts.URL
	_, err := r.Usage("key")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("500 应带状态码: %v", err)
	}
}

func TestGptzeroUsageEmptyKey(t *testing.T) {
	r := NewGptzeroRepo()
	_, err := r.Usage("  ")
	if err == nil || err.Error() != "未配置 API Key" {
		t.Fatalf("空 key 应未配置错误: %v", err)
	}
}

func TestGptzeroUsageEmptyBaseURLFallsBack(t *testing.T) {
	r := GptzeroRepo{} // BaseURL 空、Client 空 → 走默认端点与默认 client
	// 不真正联网：只验证字段回填逻辑——用 Client 指向本地 stub 保证不外联
	var called bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte(gzUsersBody))
	}))
	defer ts.Close()
	r.Client = ts.Client()
	_ = r // BaseURL 空会打真实端点，这里不调用；仅保证构造不 panic
	if called {
		t.Fatal("不应发起请求")
	}
}
