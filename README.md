# llm-api-check

> [**中文版**](README_zh.md) | [**English**](#)

A terminal CLI that checks LLM API usage — DeepSeek (balance + spend history), OpenCode (Go plan windows + Zen billing), Qwen Token Plan (plan models + 5-hour / 7-day credit windows), and AI Galaxy 智星云 GPU cloud (account balance + rented instance status), with multi-account support. Ported from the Android app [API Checkers](https://github.com/xieguaiwu/pocket-llm-api-checker): same data-layer logic, terminal output instead of an app UI.

## Features

- **DeepSeek** — balance via official API (`api.deepseek.com/user/balance`); daily spend (today / 7d / 30d) via the platform API (requires browser login token)
- **OpenCode Go plan** — 3 usage windows: Rolling 5h / Weekly 7d / Monthly 30d, with percent bars, reset countdown, and rate-limit status
- **OpenCode Zen plan** — balance, monthly usage/limit, auto-reload settings (parsed from the billing page; requires workspace ID + auth cookie)
- **Qwen Token Plan** — plan model list via the subscription gateway, plus 5-hour / 7-day credit windows with reset countdowns (quota comes from the official Bailian CLI when available, falling back to a console cookie; the quota API does not accept API keys)
- **AI Galaxy (智星云)** — cash balance, compute vouchers and credit quota kept apart (never summed), plus rented GPU/CVM instances: status badge, GPU model/count, vCPU/memory, region, SSH endpoint, hourly price, auto-renew flag and an always-visible expiry countdown
- **Multi-account** — unlimited DeepSeek, OpenCode, Qwen and AI Galaxy accounts, shown separately
- Machine-readable `--json` output, `NO_COLOR` support
- Zero third-party dependencies (Go stdlib only)

## Install

```bash
go build -trimpath -ldflags="-s -w" -o ~/.local/bin/llm-api-check .
```

Requires Go ≥ 1.25.

## Quick start

```bash
llm-api-check                              # refresh all accounts, show overview
llm-api-check accounts add --type deepseek --name "DeepSeek" --api-key sk-xxx
llm-api-check accounts add --type opencode --name "Account 1" --go-api-key sk-xxx \
  --workspace-id wrk_xxx --auth-cookie Fe26.2xxx
llm-api-check accounts add --type qwen --name "Token Plan" --api-key sk-sp-xxx \
  --console-cookie 'login_aliyunid_csrf=...; cna=...' --region cn-beijing
llm-api-check opencode "Account 1"         # account detail (Go windows + Zen card)
llm-api-check qwen --stats                 # Qwen detail + 7-day usage analysis (tokens + free tier)
llm-api-check accounts add --type galaxy --name "GPU cloud" \
  --access-key <your-access-key> --secret-key <your-secret-key>
llm-api-check galaxy                       # AI Galaxy balance + instance status
llm-api-check galaxy --limit 5             # top 5 active instances
```

Download a prebuilt binary from [Releases](https://github.com/xieguaiwu/llm-api-check/releases) (Linux and macOS, amd64 and arm64), verify it against `sha256sums.txt`, then put it on your `PATH`:

```bash
tar -xzf llm-api-check_1.1.0_linux_amd64.tar.gz -C ~/.local/bin
llm-api-check --version
```

## Commands

| Command | Description |
|---|---|
| `llm-api-check` | Refresh all accounts, show overview (same as `status`) |
| `llm-api-check status [--no-refresh]` | Overview; `--no-refresh` reads config only, no network |
| `llm-api-check deepseek [name\|ID] [--no-refresh]` | DeepSeek account detail |
| `llm-api-check opencode [name\|ID] [--no-refresh]` | OpenCode account detail |
| `llm-api-check qwen [name\|ID] [--no-refresh] [--stats]` | Qwen account detail (`--stats` adds 7-day token stats + free-tier quota) |
| `llm-api-check galaxy [name\|ID] [--no-refresh] [--limit N]` | AI Galaxy balance + active instances (`--limit` how many, default 10, max 100) |
| `llm-api-check accounts list` | List all accounts |
| `llm-api-check accounts add --type opencode\|deepseek\|qwen\|galaxy --name NAME [credential flags]` | Add account |
| `llm-api-check accounts remove --id ID \| --name NAME` | Remove account |
| `llm-api-check accounts rename --id ID \| --name NAME --new-name NEW` | Rename account |
| `llm-api-check config path` | Print config file path |
| `llm-api-check --version` | Print version |

Global flags: `--json` (all output as JSON), `--no-color` (same as `NO_COLOR` env).

Credential flags fall back in this order: flag → env var → TTY prompt (non-TTY errors out). Env vars: `LLM_API_CHECK_GO_API_KEY`, `LLM_API_CHECK_WORKSPACE_ID`, `LLM_API_CHECK_AUTH_COOKIE`, `LLM_API_CHECK_DEEPSEEK_API_KEY`, `LLM_API_CHECK_PLATFORM_TOKEN`, `LLM_API_CHECK_QWEN_API_KEY`, `LLM_API_CHECK_QWEN_COOKIE`, `LLM_API_CHECK_QWEN_REGION`, `LLM_API_CHECK_GALAXY_ACCESS_KEY`, `LLM_API_CHECK_GALAXY_SECRET_KEY`, `LLM_API_CHECK_BL_BIN` (Bailian CLI path), `LLM_API_CHECK_QWEN_CLI` (`off` disables the CLI quota channel).

## Where to get credentials

| Credential | Where to get it |
|---|---|
| DeepSeek API key | platform.deepseek.com → API Keys |
| DeepSeek platform token | Browser DevTools → Network → any `api/v0` request → `Authorization` header (expires in days/weeks) |
| OpenCode Go API key | opencode.ai account settings |
| OpenCode workspace ID + auth cookie | Browser DevTools → Application → Cookies → opencode.ai → `auth` (starts with `Fe26.2`) |
| Qwen API key | Alibaba Cloud Model Studio (Bailian) → Token Plan → API Key (`sk-sp-…`). Subscription keys are region-bound: a Beijing key only works on the Beijing gateway |
| Bailian CLI (preferred) | Install the official CLI: `npm install -g --prefix ~/.local/share/bailian-cli bailian-cli`, then log in once: `~/.local/share/bailian-cli/bin/bailian auth login --console`. The tool auto-detects it (or set `LLM_API_CHECK_BL_BIN`); CLI takes precedence over the cookie |
| Qwen console cookie (fallback) | Sign in to `bailian.console.aliyun.com` → Token Plan page → DevTools → Network → any `data/api.json` request → copy the whole `Cookie` request header (you can paste it with the `Cookie:` prefix; the tool strips it) |
| AI Galaxy AccessKey + SecretKey | gpu.ai-galaxy.cn console → 开放API → AccessKey管理 → create (requires real-name verification first). API signature is MD5 over sorted non-empty params plus `&secret=<SecretKey>` |

Qwen quota is console-session data. The tool tries the official Bailian CLI first (`bailian usage token-plan`; set `LLM_API_CHECK_QWEN_CLI=off` to disable), then falls back to a console cookie. Without either it still validates the key and lists plan models, and says so instead of showing invented numbers.

## Data sources

| Source | Endpoint | Auth |
|---|---|---|
| DeepSeek balance | `GET https://api.deepseek.com/user/balance` | API key |
| DeepSeek spend | `GET https://platform.deepseek.com/api/v0/usage/cost?month=&year=` | browser token |
| OpenCode Go usage | `GET https://opencode.ai/zen/go/v1/usage` | API key |
| OpenCode Zen billing | `GET https://opencode.ai/workspace/{id}/billing` | auth cookie |
| Qwen plan models | `GET https://token-plan.<region>.maas.aliyuncs.com/compatible-mode/v1/models` | API key |
| Qwen quota windows | `bailian usage token-plan --output json` (official CLI) or `POST https://bailian-cs.console.aliyun.com/data/api.json` (`zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/usage`) | CLI console session or cookie + `sec_token` |
| AI Galaxy balance | `POST https://app.ai-galaxy.cn/openapi/v2/account/get_main_account_info` | AccessKey + SecretKey signature |
| AI Galaxy instances | `POST .../instance/get_instance_status_count`, `.../instance/get_instance_list` | AccessKey + SecretKey signature |
| AI Galaxy spend | `POST .../billing/get_balance_change_list` | AccessKey + SecretKey signature |

## Security

- No credentials are hardcoded or committed. All secrets live in `~/.config/llm-api-check/config.json`, created with mode `0600`.
- The Android original encrypts credentials with the Android Keystore; a CLI has no OS keystore, so this tool uses file permissions (same model as `gh` / `aws` CLI). The tool warns on stderr if the config file permissions are too open.
- Zen billing data is parsed from the billing page HTML — it may break if opencode.ai changes its page structure. Qwen quota windows come from the Bailian CLI console session (preferred) or the console RPC — both expire with your console session; re-run `bailian auth login --console` to refresh.
- AI Galaxy returns instance root/RDP/VNC passwords inside the instance list. The decoder uses an explicit field whitelist, so those secrets never reach the data model, `--json` output or the UI. The tool also never calls `account/get_apikey_info`, which would echo your SecretKey back.
- The Qwen console cookie carries your Alibaba Cloud login session. Treat that config file as sensitive, and remove the account (`accounts remove`) when you no longer need it. The Bailian CLI stores its session under `~/.bailian/config.json` (0600).

## License

MIT — see [LICENSE](LICENSE).
