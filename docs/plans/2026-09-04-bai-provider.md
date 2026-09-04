# BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04

## 一、背景与需求

用户要求新增 `provider=bai`：白B.AI（api.b.ai）免费 0-Credits flash 通道。凭据 =
`~/.config/fish/config.fish` 的 `BAI_API_KEY`（sk- 前缀，chat.b.ai 侧栏 API →
Create API Key 创建）。该通道是 pi-subagent 的默认免费模型源（bai/qwen3.8-flash、
bai/deepseek-v4-flash），模型上下架直接影响 subagent 可用性。

config.fish 注释称「本机直连超时必须走 7897 代理」——2026-09-04 实测**直连已通**
（200，3.5s），代理同样可通。Go 默认 `http.ProxyFromEnvironment` 与 curl 行为一致，
不做特殊处理。

## 二、契约（2026-09-04 真实 key 实测）

平台指纹：响应头 `x-oneapi-request-id` → one-api 系网关；控制台 chat.b.ai 为
Next.js 自建站。

| 端点 | 认证 | 实测结果 |
|:---|:---|:---|
| `GET https://api.b.ai/v1/models` | Bearer sk- | 200 `{"data":[{id,object,created,owned_by,supported_endpoint_types}…],"object":"list","success":true}`，47 模型 |
| `GET /v1/models/{id}` | Bearer sk- | 200 单对象；不存在的 id 未测（不接入） |
| `POST /v1/chat/completions` | Bearer sk- | 200 正常推理；**无任何 x-ratelimit 头** |
| `GET /v1/dashboard/billing/subscription`、`/v1/dashboard/billing/usage`、`/api/user/self`、`/api/status` | Bearer sk- | 403 `{"message":"HTTP node only allows access to inference API paths (…)","success":false}` |
| 无效 key（models / chat） | 假 key | 401 `{"error":{"code":"","message":"Invalid token (request id: …)","type":"api_error"}}` |

实测边界（v1 不做的事，均有证据）：
- **配额/余额不可得**：sk- key 只开放推理路径（403 信息原文），控制台会话通道
  （chat.b.ai）需浏览器登录，无公开 API——同 DeepSeek 平台 Token 的处境，留待
  用户日后取证再加。
- **限流时限不可得**：推理响应无限流头；10 路并发 max_tokens=3 未复现 429
  （memory 里 429 见于长请求场景）。§六「限流时限可见」对 BAI 数据源不适用，
  非实现缺陷。

## 三、数据模型（models）

```go
type BaiAccount struct { ID, Name, ApiKey string }          // json: id/name/apiKey
type BaiModel  struct { ID, OwnedBy string; Endpoints []string } // json: id/owned_by/supported_endpoint_types
type BaiPlan   struct { Models []BaiModel }
```

免费通道盯梢清单（`models.BaiFreeFlashModels`，2026-09-04 用户侧快照，仅提示不参与
断言）：`deepseek-v4-flash`、`deepseek-v4-flash-vision-exp`、`glm-5.3-flash`、
`qwen3.8-flash`。`BaiPlan.MissingFreeFlash()` 返回缺失项。

## 四、通道与错误

- repo：`BaiRepo{BaseURL="https://api.b.ai", Client}`，`Models(apiKey)` 走 doGet
  （Bearer + Accept: application/json）→ `parsers.ParseBaiModels`。
- 401/403 统一 → `BAI API Key 无效、已过期或额度用尽，请到 chat.b.ai 核对`
  （one-api 对额度用尽/令牌过期也回 403，doGet 不区分状态码，文案须覆盖两种）。
- 解析：信封 `success=false` 且无 data → 显式错误；data 空 → 「未获取到 BAI 模型」；
  id 去重 + 按 id 排序（对齐 ParseQwenModels 惯例）；owned_by 原样保留。

## 五、渲染

详情（RenderBaiDetail）：
```
白B.AI (BAI)
API · 免费 0-Credits flash 通道
  模型           47 个：…
  免费通道       ✓ deepseek-v4-flash / …（全在清单时只列名字）
  免费通道       ⚠ 缺失：qwen3.8-flash（pi-subagent 默认通道受影响）  ← 有缺失时
```
- 空 key → 灰色指引行（对齐 qwen）；--no-refresh 无数据 → 灰「暂无数据」。
- 总览（writeBaiOverview）：`模型 N 个 · 免费通道 k/4` + 错误行。

## 六、CLI

- `llm-api-check bai [名称|ID] [--no-refresh]`；`accounts add --type bai --name N --api-key sk-…`
  （env `LLM_API_CHECK_BAI_API_KEY` → TTY 回退）；list/remove/rename/config 收编。
- exit code：Error≠"" 且 Plan==nil → 1（对齐 exitCodeForResults 既有语义）。
- `--json`：apiKey 掩码（publicBaiAccount）；status JSON 增 `bai` 数组。

## 七、测试与质量门

- parsers：happy / success=false 信封 / 空 data / 去重排序 / 非法 JSON。
- repo：httptest wire format（Bearer 头、路径）、401、403、网络错误、非 JSON 200。
- app：RefreshBai 成功/失败/账号不存在；RefreshAll 收编。
- render：全在 / 缺失 / 空 key / 错误 / 总览行；显示宽度对齐抽查。
- main：add → list → --no-refresh → rename → remove 全链（不联网）。
- 质量门：gofmt 0 / vet 0 / 7 包 `-race` 全绿；反向验证（删实现对应测例必 FAIL）；
  新二进制装 `~/.local/bin` + 真机冒烟。
