package config

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

func tmpPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "config.json")
}

func TestRoundtrip(t *testing.T) {
	path := tmpPath(t)
	cfg := &Config{
		DeepSeekAccounts: []models.DeepSeekAccount{
			{ID: "ds1", Name: "DS", ApiKey: "k1", PlatformToken: "t1"},
		},
		Accounts: []models.Account{
			{ID: "a1", Name: "OC", GoApiKey: "gk1", WorkspaceId: "w1", AuthCookie: "c1"},
		},
		LastUpdate: map[string]int64{"all": 12345},
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.DeepSeekAccounts) != 1 || loaded.DeepSeekAccounts[0] != cfg.DeepSeekAccounts[0] {
		t.Errorf("DeepSeek 账号往返不一致: %+v", loaded.DeepSeekAccounts)
	}
	if len(loaded.Accounts) != 1 || loaded.Accounts[0] != cfg.Accounts[0] {
		t.Errorf("OpenCode 账号往返不一致: %+v", loaded.Accounts)
	}
	if loaded.LastUpdate["all"] != 12345 {
		t.Errorf("LastUpdate 往返不一致: %+v", loaded.LastUpdate)
	}
}

func TestUpsertAndDelete(t *testing.T) {
	cfg := &Config{}
	cfg.SaveAccount(models.Account{ID: "a1", Name: "old", GoApiKey: "k"})
	cfg.SaveAccount(models.Account{ID: "a1", Name: "new", GoApiKey: "k2"}) // 同 id upsert
	if len(cfg.Accounts) != 1 || cfg.Accounts[0].Name != "new" {
		t.Errorf("upsert 应覆盖: %+v", cfg.Accounts)
	}
	cfg.SaveAccount(models.Account{ID: "a2", Name: "second"})
	cfg.DeleteAccount("a1")
	if len(cfg.Accounts) != 1 || cfg.Accounts[0].ID != "a2" {
		t.Errorf("delete 后应剩 a2: %+v", cfg.Accounts)
	}

	cfg.SaveDeepSeekAccount(models.DeepSeekAccount{ID: "d1", Name: "ds1", ApiKey: "k"})
	cfg.SaveDeepSeekAccount(models.DeepSeekAccount{ID: "d1", Name: "ds1b", ApiKey: "k2"})
	if len(cfg.DeepSeekAccounts) != 1 || cfg.DeepSeekAccounts[0].Name != "ds1b" {
		t.Errorf("DeepSeek upsert 应覆盖: %+v", cfg.DeepSeekAccounts)
	}
	cfg.DeleteDeepSeekAccount("d1")
	if len(cfg.DeepSeekAccounts) != 0 {
		t.Errorf("DeepSeek delete 后应空: %+v", cfg.DeepSeekAccounts)
	}
}

func TestSaveFilePermission0600(t *testing.T) {
	path := tmpPath(t)
	cfg := &Config{}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("配置文件权限应为 0600，got %04o", fi.Mode().Perm())
	}
}

func TestLoadWidePermissionWarns(t *testing.T) {
	path := tmpPath(t)
	cfg := &Config{Accounts: []models.Account{{ID: "a1", Name: "n", GoApiKey: "k"}}}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if SecurityWarning == "" {
		t.Error("0644 权限应置 SecurityWarning")
	}
	if !strings.Contains(SecurityWarning, "权限过宽") {
		t.Errorf("警告文案应含「权限过宽」: %s", SecurityWarning)
	}
	// 0600 文件应清除警告
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if SecurityWarning != "" {
		t.Errorf("0600 不应有警告: %s", SecurityWarning)
	}
}

func TestLoadMissingFileEmpty(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("文件不存在应返回空 Config 不报错: %v", err)
	}
	if len(cfg.Accounts) != 0 || len(cfg.DeepSeekAccounts) != 0 {
		t.Errorf("应为空 Config: %+v", cfg)
	}
}

func TestLoadCorruptJSON(t *testing.T) {
	path := tmpPath(t)
	if err := os.WriteFile(path, []byte("not json {"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Errorf("损坏 JSON 应返回可读错误，got %v", err)
	}
}

func TestNewID(t *testing.T) {
	id := NewID()
	if len(id) != 32 {
		t.Errorf("id 长度应为 32，got %d", len(id))
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Errorf("id 应为合法 hex: %v", err)
	}
	if NewID() == id {
		t.Error("两次 NewID 不应相同")
	}
}

func TestDefaultPathXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	if got := DefaultPath(); got != "/tmp/xdg-test/llm-api-check/config.json" {
		t.Errorf("DefaultPath 应尊重 XDG_CONFIG_HOME，got %s", got)
	}
}

func TestDefaultPathHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "llm-api-check", "config.json")
	if got := DefaultPath(); got != want {
		t.Errorf("DefaultPath 应回退 ~/.config，got %s want %s", got, want)
	}
}
