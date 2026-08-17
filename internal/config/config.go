// Package config 配置存储，对应 Android 版 SecureSettings.kt。
//
// 与 Android 版的差异（重要）：Android 版用 Keystore AES-GCM 加密凭据后落
// SharedPreferences；CLI 无法使用 Android Keystore，等价方案 = 文件权限 0600
// + 启动时权限检查警告（与 gh/aws cli 同模式）。不实现假加密：加密 key 与
// 密文同盘存放没有安全增益。
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/models"
)

// SecurityWarning 非致命安全问题提示（配置文件权限过宽等）。
// 对应 Android 版 SecureSettings.securityWarning 的 UI 提示逻辑。
var SecurityWarning string

// Config 配置文件结构（账号列表 + 最近更新时间）。
type Config struct {
	DeepSeekAccounts []models.DeepSeekAccount `json:"deepseek_accounts"`
	Accounts         []models.Account         `json:"accounts"`
	LastUpdate       map[string]int64         `json:"last_update"`
}

// DefaultPath 配置路径：$XDG_CONFIG_HOME/llm-api-check/config.json，
// 未设 XDG_CONFIG_HOME 时用 ~/.config/llm-api-check/config.json。
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "llm-api-check", "config.json")
}

// Load 读取配置。文件不存在 → 空 Config（不报错）；JSON 损坏 → 返回可读错误。
// 检查文件权限：向组/其他开放时置 SecurityWarning。
func Load(path string) (*Config, error) {
	cfg := &Config{LastUpdate: map[string]int64{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("配置 JSON 损坏: %w", err)
	}
	if cfg.LastUpdate == nil {
		cfg.LastUpdate = map[string]int64{}
	}
	checkPermissions(path)
	return cfg, nil
}

// checkPermissions 配置文件含明文凭据：权限含组/其他可读位时置警告
func checkPermissions(path string) {
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	if fi.Mode().Perm()&0o077 != 0 {
		SecurityWarning = fmt.Sprintf(
			"配置文件权限过宽（当前 %04o），凭据为明文存储，建议执行 chmod 600 %s",
			fi.Mode().Perm(), path)
	} else {
		SecurityWarning = ""
	}
}

// Save 保存配置：写同目录临时文件 + rename（防半写），chmod 0600。
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后该文件已不存在，Remove 静默失败无副作用
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入配置失败: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("设置配置权限失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	SecurityWarning = ""
	return nil
}

// SaveDeepSeekAccount 按 id upsert（对应 SecureSettings.saveDeepSeekAccount 的 indexOfFirst 逻辑）
func (c *Config) SaveDeepSeekAccount(a models.DeepSeekAccount) {
	for i, x := range c.DeepSeekAccounts {
		if x.ID == a.ID {
			c.DeepSeekAccounts[i] = a
			return
		}
	}
	c.DeepSeekAccounts = append(c.DeepSeekAccounts, a)
}

// DeleteDeepSeekAccount 按 id 删除（对应 SecureSettings.deleteDeepSeekAccount）
func (c *Config) DeleteDeepSeekAccount(id string) {
	kept := c.DeepSeekAccounts[:0]
	for _, x := range c.DeepSeekAccounts {
		if x.ID != id {
			kept = append(kept, x)
		}
	}
	c.DeepSeekAccounts = kept
}

// SaveAccount 按 id upsert（对应 SecureSettings.saveAccount）
func (c *Config) SaveAccount(a models.Account) {
	for i, x := range c.Accounts {
		if x.ID == a.ID {
			c.Accounts[i] = a
			return
		}
	}
	c.Accounts = append(c.Accounts, a)
}

// DeleteAccount 按 id 删除（对应 SecureSettings.deleteAccount）
func (c *Config) DeleteAccount(id string) {
	kept := c.Accounts[:0]
	for _, x := range c.Accounts {
		if x.ID != id {
			kept = append(kept, x)
		}
	}
	c.Accounts = kept
}

// SetLastUpdate 记录最近更新时间（对应 SecureSettings.setLastUpdate）
func (c *Config) SetLastUpdate(key string, t int64) {
	if c.LastUpdate == nil {
		c.LastUpdate = map[string]int64{}
	}
	c.LastUpdate[key] = t
}

// LastUpdateAt 读取最近更新时间（对应 SecureSettings.lastUpdate）
func (c *Config) LastUpdateAt(key string) time.Time {
	if c.LastUpdate == nil {
		return time.Time{}
	}
	t, ok := c.LastUpdate[key]
	if !ok || t <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(t)
}

// NewID 生成 32 字符 hex 随机账号 id（等价 Android 版 UUID，零第三方依赖：
// 16 字节 crypto/rand → hex 编码）。
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败属于系统级异常（熵源不可用），无法继续
		panic(fmt.Sprintf("生成随机 id 失败: %v", err))
	}
	return hex.EncodeToString(b)
}
