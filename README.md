# llm-api-check

> [**中文版**](README_zh.md) | [**English**](#)

A terminal CLI that checks LLM API usage — DeepSeek (balance + spend history) and OpenCode (Go plan windows + Zen billing), with multi-account support. Ported from the Android app [API Checkers](https://github.com/xieguaiwu/pocket-llm-api-checker): same data-layer logic, terminal output instead of an app UI.

## Features

- **DeepSeek** — balance via official API (`api.deepseek.com/user/balance`); daily spend (today / 7d / 30d) via the platform API (requires browser login token)
- **OpenCode Go plan** — 3 usage windows: Rolling 5h / Weekly 7d / Monthly 30d, with percent bars, reset countdown, and rate-limit status
- **OpenCode Zen plan** — balance, monthly usage/limit, auto-reload settings (parsed from the billing page; requires workspace ID + auth cookie)
- **Multi-account** — unlimited DeepSeek and OpenCode accounts, shown separately
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
llm-api-check opencode "Account 1"         # account detail (Go windows + Zen card)
```

## Commands

| Command | Description |
|---|---|
| `llm-api-check` | Refresh all accounts, show overview (same as `status`) |
| `llm-api-check status [--no-refresh]` | Overview; `--no-refresh` reads config only, no network |
| `llm-api-check deepseek [name\|ID] [--no-refresh]` | DeepSeek account detail |
| `llm-api-check opencode [name\|ID] [--no-refresh]` | OpenCode account detail |
| `llm-api-check accounts list` | List all accounts |
| `llm-api-check accounts add --type opencode\|deepseek --name NAME [credential flags]` | Add account |
| `llm-api-check accounts remove --id ID \| --name NAME` | Remove account |
| `llm-api-check accounts rename --id ID \| --name NAME --new-name NEW` | Rename account |
| `llm-api-check config path` | Print config file path |
| `llm-api-check --version` | Print version |

Global flags: `--json` (all output as JSON), `--no-color` (same as `NO_COLOR` env).

Credential flags fall back in this order: flag → env var → TTY prompt (non-TTY errors out). Env vars: `LLM_API_CHECK_GO_API_KEY`, `LLM_API_CHECK_WORKSPACE_ID`, `LLM_API_CHECK_AUTH_COOKIE`, `LLM_API_CHECK_DEEPSEEK_API_KEY`, `LLM_API_CHECK_PLATFORM_TOKEN`.

## Where to get credentials

| Credential | Where to get it |
|---|---|
| DeepSeek API key | platform.deepseek.com → API Keys |
| DeepSeek platform token | Browser DevTools → Network → any `api/v0` request → `Authorization` header (expires in days/weeks) |
| OpenCode Go API key | opencode.ai account settings |
| OpenCode workspace ID + auth cookie | Browser DevTools → Application → Cookies → opencode.ai → `auth` (starts with `Fe26.2`) |

## Data sources

| Source | Endpoint | Auth |
|---|---|---|
| DeepSeek balance | `GET https://api.deepseek.com/user/balance` | API key |
| DeepSeek spend | `GET https://platform.deepseek.com/api/v0/usage/cost?month=&year=` | browser token |
| OpenCode Go usage | `GET https://opencode.ai/zen/go/v1/usage` | API key |
| OpenCode Zen billing | `GET https://opencode.ai/workspace/{id}/billing` | auth cookie |

## Security

- No credentials are hardcoded or committed. All secrets live in `~/.config/llm-api-check/config.json`, created with mode `0600`.
- The Android original encrypts credentials with the Android Keystore; a CLI has no OS keystore, so this tool uses file permissions (same model as `gh` / `aws` CLI). The tool warns on stderr if the config file permissions are too open.
- Zen billing data is parsed from the billing page HTML — it may break if opencode.ai changes its page structure.

## License

MIT — see [LICENSE](LICENSE).
