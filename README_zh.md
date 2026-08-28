# llm-api-check

> [**English**](README.md) | [**中文版**](#)

查看 LLM API 使用情况的终端 CLI——DeepSeek（余额 + 消费明细）、OpenCode（Go plan 三窗口 + Zen billing）与 Qwen Token Plan（套餐模型 + 5 小时/7 天 配额窗口），支持多账号。复刻自 Android app [API Checkers](https://github.com/xieguaiwu/pocket-llm-api-checker)：数据层逻辑一致，输出改为终端文本/JSON 而非 App 界面。

## 功能

- **DeepSeek** — 官方 API 查余额（`api.deepseek.com/user/balance`）；平台 API 查每日消费（今日 / 7日 / 30日，需浏览器登录 token）
- **OpenCode Go plan** — 三个用量窗口：Rolling 5h / Weekly 7d / Monthly 30d，含用量条、重置倒计时、限流状态
- **OpenCode Zen plan** — 余额、本月用量/限额、自动充值设置（从 billing 页面解析，需 workspace ID + auth cookie）
- **Qwen Token Plan** — 订阅网关查套餐可用模型；百炼控制台查 5 小时/7 天 配额窗口与重置倒计时（配额接口不认 API Key，需控制台 Cookie）
- **多账号** — DeepSeek、OpenCode、Qwen 账号数量不限，各自独立展示
- 机器可读 `--json` 输出、`NO_COLOR` 支持
- 零第三方依赖（Go stdlib only）

## 安装

```bash
go build -trimpath -ldflags="-s -w" -o ~/.local/bin/llm-api-check .
```

需要 Go ≥ 1.25。

## 快速开始

```bash
llm-api-check                              # 刷新全部账号，显示总览
llm-api-check accounts add --type deepseek --name "DeepSeek" --api-key sk-xxx
llm-api-check accounts add --type opencode --name "账号 1" --go-api-key sk-xxx \
  --workspace-id wrk_xxx --auth-cookie Fe26.2xxx
llm-api-check accounts add --type qwen --name "订阅号" --api-key sk-sp-xxx \
  --console-cookie 'login_aliyunid_csrf=...; cna=...' --region cn-beijing
llm-api-check opencode "账号 1"           # 账号详情（Go 窗口 + Zen 卡片）
llm-api-check qwen                         # Qwen 详情（套餐模型 + 配额窗口）
```

可从 [Releases](https://github.com/xieguaiwu/llm-api-check/releases) 下载预编译二进制（Linux / macOS，amd64 / arm64），与 `sha256sums.txt` 对校后放入 `PATH`：

```bash
tar -xzf llm-api-check_1.1.0_linux_amd64.tar.gz -C ~/.local/bin
llm-api-check --version
```

## 命令参考

| 命令 | 说明 |
|---|---|
| `llm-api-check` | 刷新全部账号并显示总览（等同 `status`） |
| `llm-api-check status [--no-refresh]` | 总览；`--no-refresh` 只读配置不联网 |
| `llm-api-check deepseek [名称\|ID] [--no-refresh]` | DeepSeek 账号详情 |
| `llm-api-check opencode [名称\|ID] [--no-refresh]` | OpenCode 账号详情 |
| `llm-api-check qwen [名称\|ID] [--no-refresh]` | Qwen 账号详情 |
| `llm-api-check accounts list` | 列出所有账号 |
| `llm-api-check accounts add --type opencode\|deepseek\|qwen --name 名称 [凭据 flags]` | 添加账号 |
| `llm-api-check accounts remove --id ID \| --name 名称` | 删除账号 |
| `llm-api-check accounts rename --id ID \| --name 名称 --new-name 新名称` | 重命名账号 |
| `llm-api-check config path` | 打印配置文件路径 |
| `llm-api-check --version` | 打印版本 |

全局 flags：`--json`（所有输出为 JSON）、`--no-color`（等同 `NO_COLOR` 环境变量）。

凭据 flags 缺失时按此顺序回退：flag → 环境变量 → TTY 交互提示（非 TTY 报错）。环境变量：`LLM_API_CHECK_GO_API_KEY`、`LLM_API_CHECK_WORKSPACE_ID`、`LLM_API_CHECK_AUTH_COOKIE`、`LLM_API_CHECK_DEEPSEEK_API_KEY`、`LLM_API_CHECK_PLATFORM_TOKEN`、`LLM_API_CHECK_QWEN_API_KEY`、`LLM_API_CHECK_QWEN_COOKIE`、`LLM_API_CHECK_QWEN_REGION`。

## 凭据获取方式

| 凭据 | 获取位置 |
|---|---|
| DeepSeek API key | platform.deepseek.com → API Keys |
| DeepSeek 平台 token | 浏览器 DevTools → Network → 任意 `api/v0` 请求的 `Authorization` 头（几天到几周过期） |
| OpenCode Go API key | opencode.ai 账号设置 |
| OpenCode workspace ID + auth cookie | 浏览器 DevTools → Application → Cookies → opencode.ai → `auth`（以 `Fe26.2` 开头） |
| Qwen API key | 阿里云百炼 → Token Plan → API Key（`sk-sp-…`）。订阅密钥**与区域绑定**：北京 key 只能打北京网关 |
| Qwen 控制台 Cookie | 登录 `bailian.console.aliyun.com` → Token Plan 页 → DevTools → Network → 任意 `data/api.json` 请求 → 复制整个 `Cookie` 请求头（连 `Cookie:` 前缀一起粘贴也可以，工具会自动剥除） |

Qwen 配额属于控制台会话数据。未配 Cookie 时，工具仍会验证密钥并列出套餐模型，并明说缺什么，不会给出臆造的数字。

## 数据来源

| 数据 | 接口 | 认证 |
|---|---|---|
| DeepSeek 余额 | `GET https://api.deepseek.com/user/balance` | API key |
| DeepSeek 消费 | `GET https://platform.deepseek.com/api/v0/usage/cost?month=&year=` | 浏览器 token |
| OpenCode Go usage | `GET https://opencode.ai/zen/go/v1/usage` | API key |
| OpenCode Zen billing | `GET https://opencode.ai/workspace/{id}/billing` | auth cookie |
| Qwen 套餐模型 | `GET https://token-plan.<region>.maas.aliyuncs.com/compatible-mode/v1/models` | API key |
| Qwen 配额窗口 | `POST https://bailian-cs.console.aliyun.com/data/api.json`（`zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/usage`） | 控制台 Cookie + `sec_token` |

## 安全说明

- 不硬编码、不提交任何凭据。全部密钥存于 `~/.config/llm-api-check/config.json`，创建时权限 `0600`。
- Android 原版用 Keystore 加密凭据；CLI 无系统级 keystore，本工具用文件权限保护（与 `gh` / `aws` CLI 同模式）。配置文件权限过宽时会在 stderr 给出警告。
- Zen billing 数据来自页面解析，若 opencode.ai 改版可能失效。Qwen 配额窗口来自百炼控制台 RPC，随控制台会话过期。
- Qwen 控制台 Cookie 包含你的阿里云登录会话。请把配置文件当敏感文件对待，不再需要时用 `accounts remove` 删除该账号。

## 许可证

MIT — 见 [LICENSE](LICENSE)。
