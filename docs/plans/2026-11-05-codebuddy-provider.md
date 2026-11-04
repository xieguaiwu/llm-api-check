# CodeBuddy Provider Implementation Plan

> **For agentic workers:** 按本计划任务逐项实施。步骤用 checkbox（`- [ ]`）跟踪。

**Goal:** 新增 `provider=codebuddy`（腾讯 CodeBuddy 国际版，codebuddy.ai）——身份 + 模型目录（含 credits 倍率）+ **本地用量台账额度计算**（按模型聚合积分、时间窗、缓存命中）。照 longcat/commandai 模板全链接入 Go CLI。

**Architecture:** 三路并行——① 身份面 `GET /v2/accounts`（`X-API-Key`，取 uid/nickname/uin）② 目录面 `GET /v3/config`（需 `X-User-Id`，返回 28 模型与每模型 `credits` 倍率）③ 台账面：读 `~/.codebuddy/projects/*/*.jsonl`，聚合 `providerData.rawUsage.credit`。**服务端无额度接口**（见 §1.4 穷举证据），额度只能由本地台账算出——这是本 provider 与 longcat/commandai 的根本差异，渲染层必须显式标注数据来源。

**Tech Stack:** Go 标准库（net/http + httptest + encoding/json + bufio 流式扫描），零第三方依赖。

**Spec:** 本文件即规格。线协议来自四源交叉验证（2026-11-05）：① 真 key 实测（本机 `~/.config/codebuddy/api-key`）；② npm `@tencent-ai/codebuddy-code@2.179.3` 的 `dist/codebuddy.js` 逆向（header 名、路径、错误码表）；③ `product.json`（`API_KEY_HEADER="X-API-Key"`、endpoint、认证属性）；④ 本机台账 663 条真实计费记录（`~/.codebuddy/projects/`）。

## Global Constraints

- 模块路径 `github.com/xieguiawu/llm-api-check`——import 段必须与 go.mod 逐字节一致。**近形标识符陷阱**：每次写完 Go 文件脚本校验 import 前缀（`xieguaiwu` 拼写）。
- 凭据真值：key 只存 config.json（0600）+ 环境变量 `CODEBUDDY_API_KEY`（官方）→ `LLM_API_CHECK_CODEBUDDY_API_KEY`（回退）。`--json` 输出全部掩码；错误消息不带 key 原文。
- 错误文本：先 `parsers.SanitizeText` 后截断（顺序反了会切断 ESC 序列——CONTEXT 铁律）。
- 对齐：中文标签用 `render.padTo`（禁 `%-Ns` 对中文）。
- 颜色阈值 <70 蓝 / 70-89 黄 / ≥90 红。
- **不做推理探活**：额度无服务端真值通道，探活烧真钱且得不出额度。key 有效性由 `/v2/accounts` 判定。
- **不得虚构额度**：服务端不返回余额。渲染必须写明「本地台账」，禁止把台账求和呈现为「账户余额」。
- 质量门：gofmt 空 / go vet 干净 / 7 包 `go test ./... -race` 全绿；反向验证 ≥4 做（见 §九）。
- 构建部署：`go build -trimpath -ldflags="-s -w -X main.version=…" -o ~/.local/bin/llm-api-check .`（关卡 11）。
- 文档：README.md 与 README_zh.md 同 commit 同步（关卡 5/8）。
- 用户可见文本 ASD-STE100：短句、祈使、术语一致（关卡 13）。

## 一、线协议（四源验证，2026-11-05）

### 1.1 Base 与认证

- Base：`https://www.codebuddy.ai`（`product.json` `endpoint`）。DNS 与 `api.codebuddy.ai` 同 IP（43.160.158.125），无独立 API 域。
- **API key 头是 `X-API-Key`**（`product.json` 无关；来自 `dist/codebuddy.js` 的 `API_KEY_HEADER = "X-API-Key"`）。`Authorization: Bearer <key>` 对 `/v2/accounts` 同样有效（两条都实测 200），但 `/v3/config` 的鉴权走 `X-API-Key`。
- key 形态：`ck_<base36>.<base36>`（实测前缀 `ck_g3ad3g…`，总长 59）。创建入口 `https://www.codebuddy.ai/profile/keys`。
- 环境变量（官方）：`CODEBUDDY_API_KEY`。**注意**：设了它之后 CLI 还会要求 `CODEBUDDY_INTERNET_ENVIRONMENT`（海外版不设、中国版 `internal`）——本工具只发 HTTP，不需要该变量，但错误提示里要提醒用户别混淆。
- **`X-User-Id` 是 `/v3/config` 返回模型的硬前提**（实测：任意非空值 `x`/`123`/随机 UUID 均 → 28 模型；缺失 → `models:null`）。真实 uid 从 `/v2/accounts` 取。

### 1.2 身份面（`GET /v2/accounts`）

实测原样（`X-API-Key` 或 Bearer 均可）：

```json
{"code":0,"msg":"OK","requestId":"…","data":{"accounts":[
  {"uid":"e54c80ec-06fe-4be5-bac3-b74020337e7b","nickname":"xieguaiwu","uin":"450703308768",
   "type":"personal","lastLogin":true,"pluginEnabled":true,
   "deployStatus":{"statusCode":0,"statusMsg":"","detailMsg":""},
   "accountType":"","sso":{"domain":"","domainModifiedTimes":0},
   "oneidAccountId":"","isCurrentOneIdEnterprise":false,"isAmbassador":false}]}}
```

- **无 key → 401 + APISIX HTML**（`<html>…401 Authorization Required…`）。
- **坏 key → 401 + `{"message":"not_found"}`**（JSON 信封，与无 key 形状不同——两者都要归一为 `ErrCodebuddyAuth`）。
- `code != 0` → 业务错误（`msg` 进错误文本，消毒后截断）。

### 1.3 目录面（`GET /v3/config`）

必需头：`X-API-Key: <key>` + `X-User-Id: <uid>` + `User-Agent: CLI/<ver> CodeBuddy/<ver>`（UA 非必需，实测缺 UA 也 200）。

实测原样（截断，28 模型）：

```json
{"code":0,"msg":"OK","requestId":"…","data":{
  "agents":[{"name":"cli","models":["default-model",…]}],
  "enterpriseId":"",
  "models":[{"id":"default-model","name":"Auto","vendor":"e",
             "maxInputTokens":176000,"maxOutputTokens":24000,
             "maxAllowedSize":200000,"credits":"","isDefault":true,
             "supportsToolCall":true,"supportsImages":true,"temperature":1,
             "descriptionEn":"…","descriptionZh":"…"},
            {"id":"fast-model","name":"Fast","credits":"x0.34 credits",…}, …],
  "productFeatures":{…},"productFeaturesConfig":{…}}}
```

- `credits` 三形状：`""`（空，default-model）、`"x0.34 credits"`、`"x0.00"`（无 `credits` 后缀）。统一解析为**倍率 float**（`""` → 0 且标记 `HasMultiplier=false`，不显示为「x0.00」以免误读成免费）。
- `models:null` 且 `agents:null` = `X-User-Id` 缺失（实测）→ 显式报错，不静默当 0 模型。
- `/v3/config` **对 key 不校验**（坏 key 也 200）——所以它不能当鉴权判据，只能当目录源。

### 1.4 额度面：**服务端不存在**（穷举证据，2026-11-05）

本机实测穷举，全部失败：

| 路径 | 结果 | 说明 |
|:---|:---|:---|
| `POST /billing/meter/get-user-resource-summary` | 401 APISIX | 需网页登录态 Cookie，API key 不认 |
| `POST /billing/meter/get-user-resource` | 401 APISIX | 同上 |
| `POST /billing/meter/get-user-request-usage` | 401 APISIX | 同上 |
| `POST /billing/meter/get-user-daily-usage` | 401 APISIX | 同上 |
| `GET /v2/userinfo` | 400 `10001 only temporary api keys are supported` | 只接受临时 key |
| `/v2|/v3` × {usage, quota, credit(s), billing, plan, subscription, member, entitlement, balance, wallet, limits, resource} × {GET, POST} | 全 404 | 60+ 组合逐一实测 |
| `/console/*` | 302 → 登录页 | 网页会话域 |
| `/v3/config` | 200，**无余额字段** | 只有模型目录 |

`dist/codebuddy.js` 里也没有任何额度查询调用：`credit` 只在 SSE `usage.credit` 与 `model.credits`（倍率）出现；`fetchQuota`/`creditBalance`/`usageSummary` 等符号零命中。上游 CLI 自身也只显示「本轮消耗积分」（来自 SSE）与 `/cost`（会话级），从不显示账户余额。

**结论（写进代码注释与渲染文案）：额度只能从本地台账计算。**

### 1.5 台账面（本地用量真值源）

CodeBuddy CLI 每次模型调用都往 `~/.codebuddy/projects/<cwd-slug>/<sessionId>.jsonl` 追加一行，其中 `providerData.rawUsage` 与服务端 SSE 的 `usage` 对象同构，含 **`credit`（该次调用扣的积分，2 位小数）**。

实测样本（`~/.codebuddy/projects/home-xieguiawu/*.jsonl`，663 条带 credit 的记录）：

```json
{"type":"function_call","timestamp":1793803285707,
 "providerData":{"conversationRequestId":"01a1a70195817e9db0343647d56c5175",
  "messageId":"01a1a701a03c72b79ffb0442c636a34b","stepSeq":1,
  "model":"default-model","requestModelId":"default-model","requestModelName":"Auto",
  "rawUsage":{"prompt_tokens":22236,"completion_tokens":1,"total_tokens":22237,
    "prompt_cache_hit_tokens":512,"prompt_cache_miss_tokens":21724,
    "cache_read_input_tokens":0,"cache_creation_input_tokens":0,
    "prompt_cache_write_tokens":0,"completion_thinking_tokens":0,
    "credit":1.81,"cached_tokens":0}, …}}
```

**解析规则（必须逐条实现）**：

1. **只读 `providerData.rawUsage.credit`**；缺 `rawUsage` 或 `credit` 非有限数值 → 跳过该行（不补 0，不猜）。
2. **去重键 = `(providerData.conversationRequestId, providerData.messageId)`**。实测 663 行 → 663 唯一键，但两字段都缺失时以整行 `id` 兜底；同一键重复出现只计一次（首见胜出）。
3. **时间戳** `timestamp`（unix 毫秒）。缺失 → 计入「时间未知」桶，不进任何时间窗。
4. **模型名** 取 `providerData.requestModelId`，空则 `providerData.model`，再空 → `"(未知模型)"`。
5. **行格式容错**：空行跳过；JSON 损坏 → 计数 `SkippedLines++`，不中断扫描（台账是 CLI 追加写的，可能截断半行）。
6. **递归深度**：`projects/<slug>/<session>.jsonl` 一层（实测无更深）。文件不存在 → 空台账（不是错误）。
7. **只读**：绝不可写/改/删 `~/.codebuddy` 任何文件。

**实测台账数字（2026-11-05 快照，作 fixture 与验收对照）**：663 次调用 · 52.85 积分 ·
`deepseek-v4.1-flash` 652 次 43.57 积分 · `default-model` 11 次 9.28 积分 ·
时间跨度 2026-11-04 21:14 → 2026-11-05 00:47。

### 1.6 积分与倍率的关系（实测拟合，用于交叉校验而非展示预测）

按 `credit = a·(prompt−cache_hit) + b·cache_hit + c·completion` 最小二乘（单位：积分/百万 token）：

| 模型 | n | R² | 非缓存输入 | 缓存输入 | 输出 |
|:---|---:|---:|---:|---:|---:|
| `deepseek-v4.1-flash` | 652 | 0.9981 | 14.09 | 0.301 | 54.94 |
| `default-model` | 11 | 1.0000 | 82.99 | 20.03 | 273.90 |

留出集误差 −1.0% / −0.3%。→ 台账 `credit` 是**服务端真实扣费值**，不是本地估算；拟合只作一致性自检（放测试里，不进用户可见文案）。

### 1.7 错误语义（实测）

| 场景 | 响应 | 归一 |
|:---|:---|:---|
| 无 key | 401 + APISIX HTML | `ErrCodebuddyAuth` |
| 坏 key | 401 + `{"message":"not_found"}` | `ErrCodebuddyAuth` |
| 模型不存在 | 400 `{"code":11102,"msg":"model [x] service info not found"}` | 原文（目录面用不到） |
| 首条非 system | 400 `{"code":11128,"msg":"first message is not system prompt"}` | 原文 |
| 非流式 | 400 `{"code":11101,"msg":"Non-stream chat request is currently not supported"}` | 原文 |
| 模型饱和 | 400 `{"code":14003,"extError":{"code":"no_capacity"}…}` | 「模型繁忙」（非额度） |
| 积分用尽 | 429 / `14019`（CLI 侧归类 `quota_balance_exhausted`） | 「积分已用尽」（非本工具发起） |

本工具**不发起 chat 请求**，故后四行只作错误文本字典，不进主链路。

## 二、数据模型（Task 1：`internal/models/models.go` 追加段）

```go
// ── Codebuddy（腾讯 CodeBuddy，codebuddy.ai） ─────────────────

// CodebuddyAccount 单账号。key 于 codebuddy.ai/profile/keys 创建（X-API-Key 头认证）。
type CodebuddyAccount struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	ApiKey string `json:"api_key"`
}

// CodebuddyIdentity 身份（GET /v2/accounts 白名单子集）。
type CodebuddyIdentity struct {
	UID         string `json:"uid"`
	Nickname    string `json:"nickname"`
	UIN         string `json:"uin"`
	Type        string `json:"type"`        // personal / enterprise
	AccountType string `json:"accountType"` // 空 = 个人
	IsAmbassador bool  `json:"isAmbassador"`
}

// CodebuddyModel 模型目录项（GET /v3/config data.models[]）。
// CreditsMultiplier 来自 "x0.34 credits" / "x0.00" / "" 三形状；
// HasMultiplier=false 表示原始为空串（不是免费，是未标注）。
type CodebuddyModel struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Vendor            string  `json:"vendor"`
	MaxInputTokens    int64   `json:"maxInputTokens"`
	MaxOutputTokens   int64   `json:"maxOutputTokens"`
	CreditsMultiplier float64 `json:"creditsMultiplier"`
	CreditsRaw        string  `json:"creditsRaw"`
	HasMultiplier     bool    `json:"hasMultiplier"`
	IsDefault         bool    `json:"isDefault"`
}

// CodebuddyCatalog 模型目录。
type CodebuddyCatalog struct {
	Models []CodebuddyModel `json:"models"`
}

// CodebuddyModelUsage 单模型台账聚合。
type CodebuddyModelUsage struct {
	Model      string  `json:"model"`
	Calls      int64   `json:"calls"`
	Credits    float64 `json:"credits"`
	Prompt     int64   `json:"promptTokens"`
	Completion int64   `json:"completionTokens"`
	CacheHit   int64   `json:"cacheHitTokens"`
	LastUsed   int64   `json:"lastUsed"` // unix 毫秒，0 = 未知
}

// CodebuddyWindow 单个时间窗的台账聚合。
type CodebuddyWindow struct {
	Label      string  `json:"label"` // 今日 / 近 7 天 / 近 30 天 / 全部
	Days       int     `json:"days"`  // 0 = 全部
	Calls      int64   `json:"calls"`
	Credits    float64 `json:"credits"`
	Prompt     int64   `json:"promptTokens"`
	Completion int64   `json:"completionTokens"`
	CacheHit   int64   `json:"cacheHitTokens"`
}

// CodebuddyLedger 本地台账（~/.codebuddy/projects/*/*.jsonl）。
// 这是本 provider 唯一的额度来源——服务端不提供余额接口（见计划 §1.4）。
type CodebuddyLedger struct {
	Root         string                `json:"root"`
	Files        int                   `json:"files"`
	Lines        int                   `json:"lines"`
	SkippedLines int                   `json:"skippedLines"`
	Duplicates   int                   `json:"duplicates"`
	Total        CodebuddyWindow       `json:"total"`
	Windows      []CodebuddyWindow     `json:"windows"`
	ByModel      []CodebuddyModelUsage `json:"byModel"`
	LastUsed     int64                 `json:"lastUsed"` // unix 毫秒，0 = 无记录
	Sessions     int                   `json:"sessions"`
}

// CodebuddyLedgerRoot 台账根目录：$CODEBUDDY_CONFIG_DIR/projects，
// 未设时 ~/.codebuddy/projects（与 CLI 自身的 CODEBUDDY_CONFIG_DIR 同源）。
func CodebuddyLedgerRoot() string { … }

// ParseCodebuddyMultiplier 解析 "x0.34 credits" / "x0.00" / "" → (倍率, 是否有标注)。
func ParseCodebuddyMultiplier(raw string) (float64, bool) { … }

// CodebuddyWindowOf 台账在 [now-days, now] 内的聚合（days<=0 = 全部）。
func CodebuddyWindowOf(l *CodebuddyLedger, days int, now time.Time) CodebuddyWindow { … }
```

`CodebuddyWindowOf` 由 `Windows` 预先算好，故本函数实际只做「按 Label 查表」；保留签名供渲染层按需重算。

## 三、解析器（Task 2：`internal/parsers/parsers.go` 追加 codebuddy 段）

```go
var ErrCodebuddyAuth = errors.New("CodeBuddy API Key 无效或已过期，请到 codebuddy.ai/profile/keys 创建或核对")

// ParseCodebuddyAccounts 解析 GET /v2/accounts。
// 坏 key 信封 {"message":"not_found"} 与 APISIX HTML 401 都在 repo 层归一，
// 此处只处理 code!=0 的业务错误与形状。
func ParseCodebuddyAccounts(raw string) ([]models.CodebuddyIdentity, error)

// ParseCodebuddyCatalog 解析 GET /v3/config 的 data.models。
// models 为 null → 显式错误（多为 X-User-Id 缺失）；空数组 → 合法 0 模型。
func ParseCodebuddyCatalog(raw string) (models.CodebuddyCatalog, error)

// ParseCodebuddyLedger 扫描台账根目录，返回聚合。
// now 用于时间窗切分；root 不存在 → 空台账 + 无错误。
func ParseCodebuddyLedger(root string, now time.Time) (models.CodebuddyLedger, error)
```

台账扫描用 `bufio.Scanner`（默认 64KB 行上限，实测最长行 ~1.5KB；`Buffer(make([]byte,0,1<<20), 4<<20)` 兜底大行）。

## 四、repo（Task 3：`internal/repo/codebuddy.go` 新文件）

```go
const codebuddyBaseURL = "https://www.codebuddy.ai"
const codebuddyUA = "CLI/2.179.3 CodeBuddy/2.179.3" // 实测任意 UA 均可，固定值仅为可追溯

type CodebuddyRepo struct {
	BaseURL string
	Client  *http.Client
	// LedgerRoot 台账根（测试注入；空 → models.CodebuddyLedgerRoot()）
	LedgerRoot string
}

// Accounts GET /v2/accounts（X-API-Key）。401（HTML 或 {"message":"not_found"}）→ ErrCodebuddyAuth。
func (r *CodebuddyRepo) Accounts(apiKey string) ([]models.CodebuddyIdentity, error)

// Catalog GET /v3/config（X-API-Key + X-User-Id）。
func (r *CodebuddyRepo) Catalog(apiKey, uid string) (models.CodebuddyCatalog, error)

// Ledger 扫描本地台账（纯文件 IO，不联网）。now 注入便于测试。
func (r *CodebuddyRepo) Ledger(now time.Time) (models.CodebuddyLedger, error)
```

`get` 方法沿用 commandai 的形状：`truncate200(SanitizeText(body))`、`Accept: application/json`、15s 超时。

## 五、app（Task 4：`internal/app/app.go`）

```go
// CodebuddyResult 单个 CodeBuddy 账号的刷新结果。
// Ledger 与账号无关（同一台机器的台账），但按账号重复携带以便渲染层单点取值。
type CodebuddyResult struct {
	Account  models.CodebuddyAccount   `json:"account"`
	Identity *models.CodebuddyIdentity `json:"identity,omitempty"`
	Catalog  *models.CodebuddyCatalog  `json:"catalog,omitempty"`
	Ledger   *models.CodebuddyLedger   `json:"ledger,omitempty"`
	Error    string                    `json:"error,omitempty"`
}

func (r CodebuddyResult) AuthFailed() bool

func (a *App) RefreshCodebuddy(id string) (CodebuddyResult, error)
func (a *App) refreshCodebuddy(acc models.CodebuddyAccount) CodebuddyResult
```

`refreshCodebuddy` 三路：身份（串行，因为 uid 是目录面的输入）→ 目录 ∥ 台账（并发）。

- 身份失败（`ErrCodebuddyAuth`）→ 目录路不发（`Error` 单行化），台账路照跑（台账不依赖 key）。
- 台账失败 → 不覆盖身份/目录结果（`joinErrors`）。
- `RefreshAll` 加 `Codebuddy` 并发接入。

## 六、渲染（Task 5：`internal/render/render.go`）

详情页布局（`RenderCodebuddyDetail`）：

```
<名称> (CodeBuddy)
  账号    xieguaiwu · uid e54c80ec…7e7b · 个人
  模型    28 个（默认 Auto · 倍率 x0.00–x6.67）
  积分    本地台账累计 52.85 · 663 次调用（数据源：~/.codebuddy/projects）
  今日    0.05 · 6 次      ← 有记录才渲染
  近7天   52.85 · 663 次
  近30天  52.85 · 663 次
  最近    2026-11-05 00:47（21 个会话）
  按模型  deepseek-v4.1-flash 43.57 (652 次) · default-model 9.28 (11 次)
  <模型清单行>
```

- **「积分」行必须带「本地台账」字样**，禁止写成「余额」「剩余额度」（服务端无此数据）。
- 台账为空 → `暂无本地台账记录（未在 ~/.codebuddy 下运行过 CodeBuddy CLI）`，不显示 0。
- 时间窗 0 记录 → 该行不渲染（避免 0 误导）。
- 模型行：总数 + 倍率区间 + 默认模型；明细限前 6 个（超出「…等 N 个」）。
- 总览段 `writeCodebuddyOverview`：身份 + 模型数 + 台账累计积分 + 错误。
- **限流徽章/倒计时**：本 provider 无限流窗口信号（服务端不返回），`index.md §六` 不适用——渲染层不造倒计时。

## 七、main 接线（Task 6：`main.go` + `internal/config/config.go`）

- `usageText` 增 `llm-api-check codebuddy [名称|ID]` 行 + 凭据说明（`--api-key` / `CODEBUDDY_API_KEY`）。
- `case "codebuddy":` 分发 + `cmdCodebuddy`。
- `accounts add --type codebuddy`：key 解析顺序 `--api-key` → `CODEBUDDY_API_KEY` → `LLM_API_CHECK_CODEBUDDY_API_KEY` → TTY。
- `accounts list/remove/rename` 接 Codebuddy。
- `--json` 投影 `publicCodebuddyResult`（key 掩码）。
- `exitCodeForResults`：`AuthFailed()` → 1；全空 + 有 Error → 1。
- `config.Config` 增 `CodebuddyAccounts []models.CodebuddyAccount` + Save/Delete。

## 八、测试（各 Task 内嵌；目标 478 → ≥520）

必测（每层至少）：

| 层 | 用例 |
|:---|:---|
| models | `ParseCodebuddyMultiplier` 三形状 + 空串 + 异常串；`CodebuddyLedgerRoot` 环境变量覆盖 |
| parsers | `ParseCodebuddyAccounts` 正常/`code!=0`/缺 data；`ParseCodebuddyCatalog` 正常/`models:null`/空数组；`ParseCodebuddyLedger` 正常/坏行/去重/时间窗/空目录 |
| repo | httptest 两路 200/401(HTML)/401(JSON)；台账 root 注入 |
| app | 认证失败单行化；台账不依赖 key；目录并发 |
| render | 详情全字段/台账空/无 Identity/倍率空串；总览 |
| main | add/list/remove/rename/`--json` 掩码/未知账号 exit 1/缺 key exit 2 |

**回归自检（进测试，防漂移）**：用 §1.5 的 663 条真实快照做 fixture，断言 `Total.Credits==52.85 && Total.Calls==663`（浮点容差 0.01）。

## 九、反向验证清单（审查轮执行，≥4 做）

1. 拆掉 `credit` 数值校验（接受任意类型）→ 应有用例 FAIL。
2. 拆掉去重键 → 重复行用例 FAIL。
3. 拆掉 `X-User-Id` 头 → 目录用例 FAIL（`models:null` → 显式错误）。
4. 拆掉 401 的 JSON 信封分支（只留 HTML）→ 坏 key 用例 FAIL。
5. 拆掉 `--json` 掩码 → 泄漏用例 FAIL。

## 十、施工顺序与验收

1. Task 1 models → 2 parsers → 3 repo → 4 app → 5 render → 6 main/config。
2. 每步 `gofmt -l .` + `go build ./...`。
3. 全量 `go test ./... -race`（注意：本机 `COMMAND_CODE_API_KEY` 已导出会让既有用例失败，跑测试时 `env -u COMMAND_CODE_API_KEY`）。
4. 真机冒烟（真 key，隔离 XDG）：
   - `llm-api-check codebuddy` → 身份 + 28 模型 + 台账 52.85
   - `llm-api-check --json codebuddy` → 无明文 key
   - 假 key → 单行 401 exit 1
   - `accounts add/list/rename/remove` 生命周期
5. 构建安装到 `~/.local/bin`（关卡 11）+ README 双语同步 + CONTEXT 更新 + graphify 重建。

## 十一、已知未知

| 项 | 状态 | 处置 |
|:---|:---|:---|
| 服务端额度接口 | **确认不存在**（§1.4 穷举） | 台账计算；文案标注来源 |
| 台账 `credit` 是否含子代理/图片/视频生成 | 未验证 | 台账按行聚合，不区分来源；文案不声称覆盖全部计费项 |
| `credits` 倍率与实扣积分的确切换算 | 未验证（拟合 R²>0.99 但非官方公式） | 只展示倍率原文语义，不据此预测 |
| 5 小时/每周窗口是否存在 | **未证实** | 不渲染窗口 |
| 临时 API key（`/v2/userinfo` 唯一接受者）的额度语义 | 未探索 | 不实现；错误文本提到即可 |
| 台账跨机器 | 不存在（本地文件） | 文案写明「本机」 |
| `CODEBUDDY_INTERNET_ENVIRONMENT` 中国版端点 | 未测 | 只支持国际版 `www.codebuddy.ai` |
