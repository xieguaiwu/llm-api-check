package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
	"github.com/xieguiawu/llm-api-check/internal/repo"
)

// cbAppServer 模拟 CodeBuddy 两路端点。
func cbAppServer(t *testing.T, accountsCode int, accountsBody, configBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/accounts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(accountsCode)
		_, _ = w.Write([]byte(accountsBody))
	})
	mux.HandleFunc("/v3/config", func(w http.ResponseWriter, r *http.Request) {
		// 目录面必须带 X-User-Id：缺了回 models:null（模拟服务端行为）
		if r.Header.Get("X-User-Id") == "" {
			_, _ = w.Write([]byte(`{"code":0,"data":{"models":null}}`))
			return
		}
		_, _ = w.Write([]byte(configBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

const (
	cbAppAccountsOK = `{"code":0,"data":{"accounts":[{"uid":"u-1","nickname":"alice",` +
		`"uin":"9","type":"personal","accountType":""}]}}`
	cbAppConfigOK = `{"code":0,"data":{"models":[` +
		`{"id":"default-model","name":"Auto","maxInputTokens":176000,"maxOutputTokens":24000,` +
		`"credits":"","isDefault":true},` +
		`{"id":"fast-model","name":"Fast","maxInputTokens":200000,"maxOutputTokens":32000,` +
		`"credits":"x0.34 credits"}]}}`
)

// cbWriteLedger 写一条台账记录到临时根目录。
func cbWriteLedger(t *testing.T, root, model string, credit string) {
	t.Helper()
	dir := filepath.Join(root, "slug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"id":"r1","timestamp":` + itoaMs(time.Now()) + `,"providerData":{` +
		`"conversationRequestId":"c1","messageId":"m1","requestModelId":"` + model + `",` +
		`"rawUsage":{"prompt_tokens":10,"completion_tokens":2,"credit":` + credit + `}}}`
	if err := os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

func cbTestApp(t *testing.T, srv *httptest.Server, ledgerRoot string, acc models.CodebuddyAccount) *App {
	t.Helper()
	cfg := &config.Config{CodebuddyAccounts: []models.CodebuddyAccount{acc}}
	return NewWithRepos(cfg, &Repos{Codebuddy: &repo.CodebuddyRepo{
		BaseURL:    srv.URL,
		Client:     srv.Client(),
		LedgerRoot: ledgerRoot,
	}})
}

func TestRefreshCodebuddyHappy(t *testing.T) {
	ledgerRoot := t.TempDir()
	cbWriteLedger(t, ledgerRoot, "model-a", "1.25")
	srv := cbAppServer(t, 200, cbAppAccountsOK, cbAppConfigOK)
	a := cbTestApp(t, srv, ledgerRoot, models.CodebuddyAccount{ID: "id1", Name: "cb", ApiKey: "ck-x"})

	res, err := a.RefreshCodebuddy("id1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.Error != "" {
		t.Fatalf("不应有错误: %s", res.Error)
	}
	if res.Identity == nil || res.Identity.UID != "u-1" || res.Identity.Nickname != "alice" {
		t.Errorf("身份解析错: %+v", res.Identity)
	}
	if res.Catalog == nil || len(res.Catalog.Models) != 2 {
		t.Errorf("目录解析错: %+v", res.Catalog)
	}
	if res.Ledger == nil || res.Ledger.Total.Calls != 1 || res.Ledger.Total.Credits != 1.25 {
		t.Errorf("台账解析错: %+v", res.Ledger)
	}
}

func TestRefreshCodebuddyAuthFailureKeepsLedger(t *testing.T) {
	// 认证失败：Error 单行化，但台账（不依赖 key）必须照常返回
	ledgerRoot := t.TempDir()
	cbWriteLedger(t, ledgerRoot, "model-a", "0.75")
	srv := cbAppServer(t, 401, `{"message":"not_found"}`, cbAppConfigOK)
	a := cbTestApp(t, srv, ledgerRoot, models.CodebuddyAccount{ID: "id1", Name: "cb", ApiKey: "ck-bad"})

	res, err := a.RefreshCodebuddy("id1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !res.AuthFailed() {
		t.Errorf("应判为认证失败: %q", res.Error)
	}
	if res.Error != parsers.ErrCodebuddyAuth.Error() {
		t.Errorf("认证失败应单行化（不叠目录/台账错误）, got %q", res.Error)
	}
	if res.Identity != nil {
		t.Error("认证失败不应有身份")
	}
	if res.Catalog != nil {
		t.Error("认证失败不应有目录（无 uid 无法取）")
	}
	if res.Ledger == nil || res.Ledger.Total.Credits != 0.75 {
		t.Errorf("台账应不受认证失败影响: %+v", res.Ledger)
	}
}

func TestRefreshCodebuddyNoKey(t *testing.T) {
	// 无 key：身份与目录都不发请求，台账照跑
	ledgerRoot := t.TempDir()
	cbWriteLedger(t, ledgerRoot, "model-a", "0.10")
	srv := cbAppServer(t, 200, cbAppAccountsOK, cbAppConfigOK)
	a := cbTestApp(t, srv, ledgerRoot, models.CodebuddyAccount{ID: "id1", Name: "cb"})

	res, err := a.RefreshCodebuddy("id1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.Identity != nil || res.Catalog != nil {
		t.Error("无 key 时不应有身份/目录")
	}
	if res.Ledger == nil || res.Ledger.Total.Calls != 1 {
		t.Errorf("台账应照跑: %+v", res.Ledger)
	}
	if res.Error == "" {
		t.Error("无 key 应有提示")
	}
	if res.AuthFailed() {
		t.Error("无 key 不是认证失败（没发请求）")
	}
}

func TestRefreshCodebuddyUnknownID(t *testing.T) {
	srv := cbAppServer(t, 200, cbAppAccountsOK, cbAppConfigOK)
	a := cbTestApp(t, srv, t.TempDir(), models.CodebuddyAccount{ID: "id1", Name: "cb", ApiKey: "ck-x"})
	if _, err := a.RefreshCodebuddy("nope"); err == nil {
		t.Fatal("未知 id 应报错")
	}
}

func TestRefreshCodebuddyRepoMissing(t *testing.T) {
	a := NewWithRepos(&config.Config{CodebuddyAccounts: []models.CodebuddyAccount{{ID: "id1"}}}, &Repos{})
	res, err := a.RefreshCodebuddy("id1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(res.Error, "未初始化") {
		t.Errorf("仓库缺失应提示: %q", res.Error)
	}
}

func TestRefreshCodebuddyLedgerErrorDoesNotMaskIdentity(t *testing.T) {
	// 台账目录不可读（权限）→ 身份/目录仍应成功，错误合并显示
	srv := cbAppServer(t, 200, cbAppAccountsOK, cbAppConfigOK)
	// 用一个文件充当台账根目录 → ReadDir 报 ENOTDIR
	badRoot := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(badRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := cbTestApp(t, srv, badRoot, models.CodebuddyAccount{ID: "id1", Name: "cb", ApiKey: "ck-x"})
	res, err := a.RefreshCodebuddy("id1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.Identity == nil || res.Catalog == nil {
		t.Error("台账失败不应拖垮身份/目录")
	}
	if res.Ledger != nil {
		t.Error("台账失败不应返回 Ledger")
	}
	if !strings.Contains(res.Error, "台账") {
		t.Errorf("错误应提到台账: %q", res.Error)
	}
}

func TestRefreshAllIncludesCodebuddy(t *testing.T) {
	srv := cbAppServer(t, 200, cbAppAccountsOK, cbAppConfigOK)
	ledgerRoot := t.TempDir()
	a := cbTestApp(t, srv, ledgerRoot, models.CodebuddyAccount{ID: "id1", Name: "cb", ApiKey: "ck-x"})
	res, err := a.RefreshAll()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(res.Codebuddy) != 1 {
		t.Fatalf("RefreshAll 应含 1 个 Codebuddy 账号, got %d", len(res.Codebuddy))
	}
	if res.Codebuddy[0].Identity == nil {
		t.Error("RefreshAll 的 Codebuddy 段应有身份")
	}
}

func TestCodebuddyAuthFailedEmpty(t *testing.T) {
	if (CodebuddyResult{}).AuthFailed() {
		t.Error("空结果不应判为认证失败")
	}
	if !(CodebuddyResult{Error: parsers.ErrCodebuddyAuth.Error()}).AuthFailed() {
		t.Error("含认证错误文本应判为认证失败")
	}
	// 不能与 Commandai 的错误串混淆
	if (CodebuddyResult{Error: parsers.ErrCommandaiAuth.Error()}).AuthFailed() {
		t.Error("Commandai 错误不应判为 CodeBuddy 认证失败")
	}
}

var _ = errors.Is
