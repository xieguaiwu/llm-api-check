# LLM API Check CLI Implementation Plan

> **For agentic workers:** 按本计划任务逐项实施。步骤用 checkbox（`- [ ]`）跟踪。

**Goal:** 一个 Go CLI，复刻 Android app「API Checkers」（`~/Desktop/android-projects/api-checkers/`）的数据层逻辑：查看 DeepSeek API（余额 + 消费明细）与 OpenCode（Go usage 三窗口 + Zen billing）的使用情况，支持多账号。不复制 Android UI，只复刻逻辑，输出为终端文本/JSON。

**Architecture:** 单二进制 CLI。数据层 = Go stdlib `net/http`（15s 超时）+ 手写 JSON/正则解析（复刻 Android 版 `Parsers.kt`）。配置 = `~/.config/llm-api-check/config.json`（chmod 0600，CLI 无 Android Keystore，用文件权限保护）。命令式 CLI（非交互 TUI）：`llm-api-check` 显示总览，子命令管理账号。

**Tech Stack:**
- Go 1.25（stdlib only：`net/http`、`encoding/json`、`regexp`、`testing`，零第三方依赖）
- 测试: `go test`（纯 JVM→纯 Go 单测，fixtures 从 Android 项目复制）

**Spec:** 需求来自用户对话 + Android 源码（`~/Desktop/android-projects/api-checkers/` 下 `data/Models.kt`、`data/Parsers.kt`、`data/Repositories.kt`、`data/ApiClient.kt`、`data/SecureSettings.kt`、`AppViewModel.kt`、`ui/DetailScreen.kt` 的 countdownText）。执行者**必须**先读这些源文件再动手，本计划是移植规格，源文件是权威实现。

## Global Constraints

1. 模块路径 `github.com/xieguiawu/llm-api-check`，二进制名 `llm-api-check`
2. 项目目录：`/home/xieguiawu/Desktop/go-projects/LLM-api-check/`（当前为空，git init 开始）
3. 零第三方依赖（stdlib only）；Go 版本 ≥ 1.22
4. 所有网络超时 15s；HTTP 401/403 必须返回中文错误（如「API Key 无效或已过期」），不崩溃
5. 中文 UI（用户是中文使用者），代码注释中文
6. **禁止硬编码任何 API key / cookie / token**（开发质量关卡 9）。凭据只来自：命令行 flag / 环境变量 / 交互提示（仅 TTY）→ 存配置文件
7. 每个解析器必须有 Go 单元测试（fixtures 放 `testdata/fixtures/`，从 Android `app/src/test/resources/fixtures/` 复制）
8. 账号数据模型固定（与 Android 一致）：
   - OpenCode: `Account{id, name, goApiKey, workspaceId, authCookie}`——cookie 与 workspace 可选（留空则只显示 Go plan）；`hasZen = workspaceId 非空 && authCookie 非空`
   - DeepSeek: `DeepSeekAccount{id, name, apiKey, platformToken}`——token 可选（留空则只显示余额）；`hasToken = platformToken 非空`
9. 颜色输出：ANSI 色码；`NO_COLOR` 环境变量存在时禁用；`--no-color` flag 同效
10. 错误消息文案必须与 Android 版逐字一致（见下方「错误消息对照表」）
11. 产出物：`~/.local/bin/llm-api-check`（关卡 11 本地部署）+ `README.md`/`README_zh.md` 双语 + `CONTEXT_FOR_NEXT_AGENT.md`

## 外部 API 契约（全部已实测/验证，来自 Android 计划文档）

### A. OpenCode Go usage（官方 API，API key 认证，无需 cookie）

```
GET https://opencode.ai/zen/go/v1/usage
Authorization: Bearer {goApiKey}
→ 200:
{"usage":{"rolling":{"status":"ok","percent":0,"resetsAt":"2026-08-14T16:20:08.884Z"},
          "weekly":{"status":"ok","percent":0,"resetsAt":"2026-08-17T00:00:00.884Z"},
          "monthly":{"status":"rate-limited","percent":100,"resetsAt":"2026-08-15T16:02:00.884Z"}}}
```
字段：每个窗口 `status`(ok|rate-limited)、`percent`(0-100)、`resetsAt`(ISO8601 UTC)。401 = key 无效。

### B. OpenCode Zen billing（页面 scrape，workspaceId + auth cookie）

```
GET https://opencode.ai/workspace/{workspaceId}/billing
Cookie: auth={authCookie}
User-Agent: Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 ...
→ 200 HTML（SolidJS SSR hydration）
```
解析算法（移植自 MIT 项目 4cya/pi-go-bars core.ts，已授权复用；**逐行对照 Android `Parsers.kt` 的 parseZenBilling**）：
1. `strings.Index(html, "customerID:\"cus_")` 找锚点；向前找对象起点 `{`（**必须跳过字符串字面量**——字符串内可能含 `{`）
2. 从 `{` 起深度计数到匹配 `}`（同样跳过字符串字面量），取子串 obj
3. 在 obj 内正则（字段顺序可变，逐个匹配）：
   - `(?:^|,)balance:(-?\d+(?:\.\d+)?)` → ÷1e8 = USD（microcents）
   - `monthlyUsage:(-?\d+(?:\.\d+)?)` → ÷1e8 = USD
   - `monthlyLimit:(-?\d+(?:\.\d+)?)` → 整 USD
   - `reload:(!0|!1|true|false|null)` → boolean（!0=true, !1=false）
   - `reloadAmount:(-?\d+(?:\.\d+)?)` → 整 USD
   - `reloadTrigger:(-?\d+(?:\.\d+)?)` → 整 USD
4. 找不到 `customerID:"cus_` → 错误「会话已过期，请更新 Cookie」
5. 找到对象但 balance/monthlyUsage/monthlyLimit 全 null → 错误「账单页面结构已变化，请更新应用」

### C. DeepSeek 余额（官方 API，API key 认证）

```
GET https://api.deepseek.com/user/balance
Authorization: Bearer {apiKey}
→ 200:
{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"120.00","granted_balance":"0.00","topped_up_balance":"120.00"}]}
```
字段：`is_available`(bool)、`balance_infos[]`：`currency`、`total_balance`、`granted_balance`、`topped_up_balance`（**字符串**金额，需 toDoubleOrNull 等价转换）。401 = key 无效。

### D. DeepSeek 消费明细（platform 页面 API，浏览器登录 token）

```
GET https://platform.deepseek.com/api/v0/usage/cost?month={m}&year={y}
Authorization: Bearer {platformToken}   ← 浏览器登录 token，非 API key
Accept: application/json
x-app-version: 1.0.0
Referer: https://platform.deepseek.com/usage
User-Agent: Mozilla/5.0 ... Chrome/126 Safari/537.36
→ 200:
{"code":0,"data":{"biz_data":[{"days":[{"date":"2026-08-14","data":[{"model":"deepseek-chat",
  "usage":[{"type":"input","amount":0.123},{"type":"output","amount":0.456}]}]}]}]}}
```
- `code` 40003 = token 失效（错误「DeepSeek 平台登录已失效，请更新平台 Token」）；非 0 = 失败（「DeepSeek 平台接口错误（code=N）」）
- 用法：拉本月+上月两个月（跨年由 `now.AddDate(0,-1,0)` 自动处理），按 date 汇总每日金额（sum 所有 type 的 amount），算今天/近7天/近30天
- 金额单位：人民币元
- **两个月都无数据（含非认证错误）→ 显式失败**，不返回误导性的零数据（失败文案：tokenInvalid→「DeepSeek 平台登录已失效，请更新平台 Token」；有 lastError→「消费数据获取失败：{msg}」；否则「消费数据为空」）

## 错误消息对照表（必须逐字一致）

| 场景 | 消息 |
|---|---|
| DeepSeek 官方 API 401/403（balance） | `API Key 无效或已过期` |
| DeepSeek platform API 401/403（cost） | `平台 Token 已失效，请更新平台 Token` |
| OpenCode Go API 401/403 | `Go API Key 无效或已过期` |
| OpenCode Zen 页面 401/403 | `Cookie 已过期，请更新 Auth Cookie` |
| 非 2xx 通用 | `HTTP {code}: {body前200字符}` |
| Zen 无 customerID 锚点 | `会话已过期，请更新 Cookie` |
| Zen 对象内三字段全 null | `账单页面结构已变化，请更新应用` |
| Zen 未配置 workspace/cookie | `未配置 Workspace/Cookie` |
| DeepSeek cost code=40003 | `DeepSeek 平台登录已失效，请更新平台 Token` |
| DeepSeek cost code 其他非 0 | `DeepSeek 平台接口错误（code={code}）` |

## 文件结构

```
LLM-api-check/
├── go.mod / go.sum（无第三方，可能无 go.sum）
├── main.go                       (CLI 入口：flag 解析 + 命令分发 + 文本渲染)
├── internal/
│   ├── models/models.go          (全部数据模型，对应 Models.kt)
│   ├── parsers/parsers.go        (GoUsage/ZenBilling/DeepSeek 解析，对应 Parsers.kt)
│   ├── repo/repo.go              (DeepSeekRepo/OpenCodeRepo，对应 Repositories.kt + ApiClient.kt)
│   ├── config/config.go          (配置存储，对应 SecureSettings.kt)
│   ├── app/app.go                (刷新编排，对应 AppViewModel.kt)
│   └── render/render.go          (终端文本渲染：用量条/倒计时/颜色，对应 HomeScreen/DetailScreen 逻辑)
├── internal/.../*_test.go        (各包单测)
├── testdata/fixtures/            (从 Android app/src/test/resources/fixtures/ 复制 4 个文件)
├── docs/plans/2026-08-18-llm-api-check-cli.md   (本计划)
├── README.md / README_zh.md      (中英双语，关卡 8)
├── CONTEXT_FOR_NEXT_AGENT.md
├── LICENSE                       (MIT，与 Android 版一致)
├── .gitignore
└── graphify-out/                 (可选，最后生成)
```

## Task 1: 项目脚手架 + 数据模型

**Files:**
- Create: `go.mod`、`.gitignore`、`LICENSE`
- Create: `internal/models/models.go`
- Test: `internal/models/models_test.go`（如需要）

**Interfaces:**
- Consumes: 无
- Produces:
  - `type GoWindow struct { Status string; Percent int; ResetsAt string }`
  - `type GoUsage struct { Rolling, Weekly, Monthly *GoWindow }`
  - `type ZenBilling struct { BalanceUsd, MonthlyUsageUsd, MonthlyLimitUsd, ReloadAmountUsd, ReloadTriggerUsd float64; AutoReload bool }`
  - `type DeepSeekBalanceInfo struct { Currency string; TotalBalance, GrantedBalance, ToppedUpBalance float64 }`
  - `type DeepSeekBalance struct { IsAvailable bool; Infos []DeepSeekBalanceInfo }`
  - `type DeepSeekCostDay struct { Date string; Total float64 }`
  - `type DeepSeekCost struct { Today, Last7d, Last30d float64; Days []DeepSeekCostDay }`
  - `type DeepSeekAccount struct { ID, Name, ApiKey, PlatformToken string }` + `func (a DeepSeekAccount) HasToken() bool`
  - `type Account struct { ID, Name, GoApiKey, WorkspaceId, AuthCookie string }` + `func (a Account) HasZen() bool`
  - JSON tag 与 Android 序列化名一致（`total_balance` 等 snake_case；accounts 列表存完整 struct）

- [ ] **Step 1: 脚手架**

```bash
cd /home/xieguiawu/Desktop/go-projects/LLM-api-check
git init
go mod init github.com/xieguiawu/llm-api-check
```

`.gitignore`（参考 Android 版 + Go 惯例）:
```gitignore
bin/
dist/
*.exe
llm-api-check
.DS_Store
.idea/
graphify-out/
```

`LICENSE`: 从 `~/Desktop/android-projects/api-checkers/LICENSE` 复制（MIT）。

- [ ] **Step 2: 实现 models.go**

对照 `Models.kt` 逐类型移植。注意 DeepSeek 原始响应 DTO 与域模型分离（Android 有 `DeepSeekBalanceInfoPayload`（字符串金额）→ `DeepSeekBalanceInfo`（float）），Go 中可合并：解析时手动 `strconv.ParseFloat` 兜底 0.0（等价 toDoubleOrNull ?: 0.0）。

- [ ] **Step 3: 编译验证**

```bash
go build ./...
```
Expected: 无输出（成功）

- [ ] **Step 4: 提交**

```bash
git add -A && git commit -m "chore: Go 脚手架 + 数据模型"
```

## Task 2: 解析器（纯 Go，带单测）

**Files:**
- Create: `internal/parsers/parsers.go`
- Create: `internal/parsers/parsers_test.go`
- Create: `testdata/fixtures/go_usage.json`、`billing.html`、`deepseek_balance.json`、`deepseek_cost.json`（**从 Android 项目复制**：`cp ~/Desktop/android-projects/api-checkers/app/src/test/resources/fixtures/* testdata/fixtures/`）

**Interfaces:**
- Consumes: Task 1 的 models
- Produces:
  - `func ParseGoUsage(raw string) (GoUsage, error)`
  - `func ParseZenBilling(html string) (ZenBilling, error)` — 错误消息见对照表
  - `func ParseDeepSeekBalance(raw string) (DeepSeekBalance, error)`
  - `func ParseDeepSeekCost(raw string, refDate time.Time) (DeepSeekCost, error)` — refDate 可传零值表示今天（测试传固定日期保证确定性）；Android 版 `refDate: LocalDate = LocalDate.now()`
  - `func AggregateCost(dayMap map[string]float64, refDate time.Time) DeepSeekCost` — 纯函数，today/7d/30d + 全部天按日期倒序
  - 解析失败返回 error，错误消息人类可读（中文）

- [ ] **Step 1: 复制 fixtures**

```bash
mkdir -p testdata/fixtures
cp ~/Desktop/android-projects/api-checkers/app/src/test/resources/fixtures/go_usage.json \
   ~/Desktop/android-projects/api-checkers/app/src/test/resources/fixtures/billing.html \
   ~/Desktop/android-projects/api-checkers/app/src/test/resources/fixtures/deepseek_balance.json \
   ~/Desktop/android-projects/api-checkers/app/src/test/resources/fixtures/deepseek_cost.json \
   testdata/fixtures/
```

- [ ] **Step 2: 写测试（先红）**

对照 Android 测试断言逐条移植（`GoUsageParserTest`、`ZenBillingParserTest`、`DeepSeekParserTest`）：
- go usage：rolling ok/0/resetsAt、monthly rate-limited/100、非法 JSON 报错、窗口缺失不崩溃（weekly/monthly 为 nil）
- zen billing：`balance:1999960750 → $19.9996075`、`monthlyUsage:39250 → $0.0003925`、`monthlyLimit:50`；无 customerID → 错误含「会话」；`monthlyUsage:null` 替换 → 容忍为 0.0
- deepseek balance：is_available、CNY、120.0/0.0/120.0
- deepseek cost（refDate 固定为 fixture 数据中的日期 + 1 天，确保断言确定）：today=4.5、last7d=4.8、last30d=4.8、days 倒序 2 条；code 40003 → 错误含「失效」；code=5 → 错误含「code=5」

- [ ] **Step 3: 实现 parsers.go**

**逐行对照 `Parsers.kt` 移植**，特别保留：
- ZenBilling 的字符串字面量感知括号匹配（inStr/esc 状态机，Kotlin 版有，Go 版同样实现——**这是 Android 版对旧算法的修复**）
- `(?:^|,)balance:` 正则的前导逗号/行首断言（防误匹配 `xxxbalance:`）
- microcents ÷1e8
- AggregateCost 的 today/7d/30d 循环（i in 0..29，key = refDate - i 天）
- 时间处理：Go 用 `time.Time`，日期键用 `t.Format("2006-01-02")`

- [ ] **Step 4: 测试转绿**

```bash
go test ./internal/parsers/ -v
```
Expected: 全绿

- [ ] **Step 5: 提交**

```bash
git add -A && git commit -m "feat: 解析器（GoUsage/ZenBilling/DeepSeek）+ 单元测试"
```

## Task 3: 网络层与仓库

**Files:**
- Create: `internal/repo/repo.go`
- Create: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: Task 1 models + Task 2 parsers
- Produces:
  - `type DeepSeekRepo struct { BaseBalanceURL, BaseCostURL string; Client *http.Client }` — 默认 `https://api.deepseek.com` / `https://platform.deepseek.com`，测试可注入 httptest URL
  - `type OpenCodeRepo struct { BaseURL string; Client *http.Client }` — 默认 `https://opencode.ai`
  - `func NewDeepSeekRepo() *DeepSeekRepo` / `func NewOpenCodeRepo() *OpenCodeRepo` — 15s 超时 client（等价 ApiClient 15s connect/read/write）
  - `func (r *DeepSeekRepo) Balance(apiKey string) (DeepSeekBalance, error)`
  - `func (r *DeepSeekRepo) Cost(platformToken string) (DeepSeekCost, error)` — 拉本月+上月，聚合
  - `func (r *OpenCodeRepo) GoUsage(acc Account) (GoUsage, error)`
  - `func (r *OpenCodeRepo) ZenBilling(acc Account) (ZenBilling, error)` — 未配置 hasZen → 错误「未配置 Workspace/Cookie」
  - 内部 get 执行 401/403 → 对照表错误；非 2xx → `HTTP {code}: {body前200}`；网络异常 → 中文包装（如 `网络请求失败: {err}`）

- [ ] **Step 1: 实现 repo.go**（对照 Repositories.kt + ApiClient.kt）

关键点：
- 请求头完全一致（Bearer、UA、Accept、x-app-version、Referer、Cookie）
- Cost 的两月循环 + dayMap 聚合 + tokenInvalid/lastError 逻辑（**失败显式，不返回误导零数据**）
- Go 结构体用 `http.Client{Timeout: 15 * time.Second}`

- [ ] **Step 2: 写测试（httptest）**

- `httptest.NewServer` 分别模拟：go usage 200（真实 fixture）、401（断言「Go API Key 无效或已过期」）、500（断言 HTTP 500 前缀）
- balance 200（fixture）/401
- cost：模拟两个月端点返回不同月份数据 → 聚合正确；一个月 40003 一个月有数据 → 用有数据的结果（不整体失败）；两月都失败 → 断言失败
- zen billing 200（billing.html fixture）/401/未配置

- [ ] **Step 3: 测试 + 提交**

```bash
go test ./internal/repo/ -v && git add -A && git commit -m "feat: 网络层 + DeepSeek/OpenCode 仓库"
```

## Task 4: 配置存储

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Interfaces:**
- Consumes: Task 1 models
- Produces:
  - `type Config struct { DeepSeekAccounts []models.DeepSeekAccount; Accounts []models.Account; LastUpdate map[string]int64 }`
  - `func DefaultPath() string` — `$XDG_CONFIG_HOME/llm-api-check/config.json`，未设则 `~/.config/llm-api-check/config.json`
  - `func Load(path string) (*Config, error)` — 文件不存在 → 空 Config（不报错）；JSON 损坏 → 返回错误（或空 Config + warning，见下）；**检查文件权限，非 0600 时置 SecurityWarning**
  - `func (c *Config) Save(path string) error` — 写临时文件 + rename（防半写）；**chmod 0600**
  - `func (c *Config) SaveDeepSeekAccount(a models.DeepSeekAccount)` / `DeleteDeepSeekAccount(id)` / `SaveAccount(a models.Account)` / `DeleteAccount(id)` — 按 id upsert（对应 SecureSettings.saveAccount 的 indexOfFirst 逻辑）
  - `var SecurityWarning string` — 权限过宽/解密失败等非致命问题提示（对应 Android securityWarning UI 提示逻辑）
  - 账号 id 生成：`github.com/google/uuid` 不可用（零依赖）→ 用 `crypto/rand` 生成 16 字节 hex（32 字符）自写 `NewID()`

- [ ] **Step 1: 实现 config.go**

对比说明（写入代码注释）：Android 版用 Keystore AES-GCM 加密；CLI 无法用 Keystore，等价方案 = 文件权限 0600 + 启动时权限检查警告（与 gh/aws cli 同模式）。**不实现假加密**（key 与密文同盘无意义）。

- [ ] **Step 2: 写测试**

- 往返：Save → Load → 内容一致（含多账号 upsert/delete）
- 权限：Save 后 stat 权限 == 0600；Load 时文件为 0644 → SecurityWarning 非空
- 损坏 JSON → 不 panic，返回可读错误
- NewID 唯一性 + 长度 32

- [ ] **Step 3: 测试 + 提交**

```bash
go test ./internal/config/ -v && git add -A && git commit -m "feat: 配置存储（0600 权限 + 账号 CRUD）"
```

## Task 5: 刷新编排（App 逻辑）

**Files:**
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`

**Interfaces:**
- Consumes: Task 1-4 全部
- Produces:
  - `type App struct { Repos *Repos; Cfg *config.Config; ... }`（结构自定，职责对应 AppViewModel）
  - `func New(cfg *config.Config) *App`
  - `func (a *App) RefreshAll() (Result, error)` — 刷新全部（DeepSeek 全账号 + OpenCode 全账号，**并行**）；重入保护（refreshing 中忽略）
  - `func (a *App) RefreshDeepSeek(id string) (DeepSeekResult, error)` / `func (a *App) RefreshAccount(id string) (AccountResult, error)` — 对应 refreshDeepSeekNow/refreshAccountNow
  - 结果结构：`DeepSeekResult{Account, Balance *DeepSeekBalance, Cost *DeepSeekCost, Error string}`（error 合并 balance/cost 两者错误，`\n` 连接，空则 nil）；`AccountResult{Account, GoUsage *GoUsage, ZenBilling *ZenBilling, Error string}`
  - 刷新期间保留旧数据（对应 P2-15 的 copy(account=acc) 保留旧值逻辑——CLI 单次执行可简化：失败时 Error 非空但旧数据为 nil 即可，CLI 无持续 UI）
  - `func (a *App) LastUpdated() time.Time` — 从 Config.LastUpdate 读

- [ ] **Step 1: 实现 app.go**

并行用 goroutine + `sync.WaitGroup`（或 errgroup 但 stdlib 无 → 用 WaitGroup + channel 收集）。

- [ ] **Step 2: 写测试**

- 用 repo 的 httptest server 注入，验证 RefreshAll 聚合多个账号、错误合并（balance 失败 + cost 成功 → Error 非空但 Cost 有值）
- 重入保护：连续两次 RefreshAll，第二次不重复发起（可用原子计数验证）

- [ ] **Step 3: 测试 + 提交**

```bash
go test ./internal/app/ -v && git add -A && git commit -m "feat: 刷新编排（并行全量刷新 + 错误合并）"
```

## Task 6: CLI 入口 + 渲染

**Files:**
- Create: `main.go`
- Create: `internal/render/render.go`
- Create: `internal/render/render_test.go`

**Interfaces:**
- Consumes: Task 1-5 全部
- Produces:
  - `func FormatCountdown(resetsAt string, now time.Time) string` — 对应 countdownText：解析失败→「即将重置」；≤0→「即将重置」；<1min→「即将重置」；<60min→「N分钟后重置」；≥1h→「N小时M分后重置」（M==0 → 「N小时后重置」）
  - `func UsageBar(percent int, width int) string` — 文本条 `[██████░░░░]`，宽度 10 格
  - `func ColorForPercent(percent int, rateLimited bool) Color` — <70 蓝、70-89 黄、≥90 红、rate-limited 强制红（对应 Android 颜色规则）
  - `func CurrencySymbol(code string) string` — CNY→¥、USD→$、EUR→€、默认 `$code `
  - `func RenderOverview(ds []app.DeepSeekResult, accs []app.AccountResult, lastUpdated time.Time, c Colorizer) string` — 总览（对应 HomeScreen）
  - `func RenderAccountDetail(r app.AccountResult, now time.Time, c Colorizer) string` — 账号详情（对应 DetailScreen）
  - `type Colorizer struct{ Disabled bool }` + `func (c Colorizer) Blue/Yellow/Red/Green/Gray(s string) string` — NO_COLOR/--no-color 时返回原文

- [ ] **Step 1: 实现 render.go**

渲染规格（对应 HomeScreen/DetailScreen 的信息结构，纯文本版）：

**总览**：
```
LLM API Check — 更新于 14:35

DeepSeek (账号名)
  余额 ¥120.00 · 充值 ¥120.00 · 赠送 ¥0.00
  今日 ¥1.20 · 7日 ¥3.00 · 30日 ¥5.50
  [错误红色显示]

OpenCode (账号名)
  R 42% [████░░░░░░] · W 17% [██░░░░░░░░] · M 8% [█░░░░░░░░░]
  Zen $19.99
  [错误红色显示]
```
- 未配置账号：`未配置任何账号，运行 llm-api-check accounts add --help 添加`
- 颜色规则：用量条 + 百分比按 ColorForPercent；rate-limited 状态加红色「已限流」标记（对应 HomeScreen 的 M 红色）
- 币种符号映射（DeepSeek 用 currency 字段，Zen 固定 $）
- 刷新时间 `更新于 HH:mm`（对应「更新于 HH:mm」小字）

**账号详情**（`llm-api-check opencode <name>`）：
```
账号名 (OpenCode)
Go Plan · 订阅
  Rolling 5h   [████░░░░░░] 42% · 4小时20分后重置
  Weekly 7d    [██░░░░░░░░] 17% · 2天后重置   ← 倒计时用小时/分钟格式（>24h 显示 N小时M分）
  Monthly 30d  [██████████] 100% · 已限流 (红)
Zen Plan · 按量
  余额 $19.99
  本月 $0.00 / $50.00 [░░░░░░░░░░] 0%
  自动充值 开 · 低于 $5 充 $20  (或「自动充值 关」)
  (未配置 → 灰色「未配置 Workspace ID / Cookie」)
```
- Go 窗口标签：Rolling 5h / Weekly 7d / Monthly 30d
- 窗口行尾：重置倒计时（TextSub 灰）；status=rate-limited → 红「已限流」替代倒计时（对照 DetailScreen：已限流优先）
- Zen 限额 0 时不显示条
- 错误卡：红色显示 error 内容

**DeepSeek 详情**（`llm-api-check deepseek [name]`）：余额大字 + 充值/赠送 + 今日/7日/30日消费；未配置 token 时消费行灰色「未配置平台 Token，仅显示余额」。

- [ ] **Step 2: 实现 main.go**

命令结构（stdlib `flag` 即可，多级子命令手写分发）：
```
llm-api-check                        # = status：刷新全部 + 总览
llm-api-check status [--no-refresh]  # 总览；默认刷新，--no-refresh 只读缓存
llm-api-check deepseek [name]        # DeepSeek 账号详情（可过滤名字/id）
llm-api-check opencode [name]        # OpenCode 账号详情
llm-api-check accounts list
llm-api-check accounts add --type opencode|deepseek --name X [凭据 flags]
llm-api-check accounts remove --id ID|--name X
llm-api-check accounts rename --id ID|--name X --new-name Y
llm-api-check config path            # 打印配置文件路径
llm-api-check --version
llm-api-check --json                 # 全局 flag：所有输出为 JSON（便于脚本）
llm-api-check --no-color             # 全局 flag：禁用 ANSI 颜色
```
- 凭据 flag 缺失时：先查环境变量（`LLM_API_CHECK_GO_API_KEY` / `LLM_API_CHECK_DEEPSEEK_API_KEY` / `LLM_API_CHECK_PLATFORM_TOKEN` / `LLM_API_CHECK_AUTH_COOKIE` / `LLM_API_CHECK_WORKSPACE_ID`），再尝试 TTY 交互提示（`golang.org/x/term` 不可用 → 用 `bufio` 简单读行，密码不回显可用 `stty -echo` 系统调用或省略回显控制，注明「输入不回显需终端支持」；非 TTY 且无 flag/env → 报错退出并提示用法）
- `--json` 输出结构：`{"deepseek":[{...}],"accounts":[{...}],"last_updated":...,"security_warning":...}`（JSON tag snake_case）
- 退出码：成功 0；任一账号全部失败 1；用法错误 2
- SecurityWarning 非空时输出到 stderr 警告行（黄色）
- 错误消息符合关卡 13（短句、先说条件再说动作）

- [ ] **Step 3: 写测试**

- render_test：countdown 全分支（<1min/52min/4h20m/整小时/解析失败/过期）、CurrencySymbol 映射、ColorForPercent 阈值、UsageBar 宽度
- main 级：`--json` 输出可解析（用 httptest 注入 repo? main 用全局 repo 构造 → 给 main 留 `var newRepos = ...` 测试可替换，或只测 render + 命令分发的小函数）
- 非交互降级（interactive-cli-design §4.3 #10）：`echo "" | llm-api-check status` 管道输入不 panic、不进交互模式

- [ ] **Step 4: 全量测试 + 构建**

```bash
go test ./... && go build -trimpath -ldflags="-s -w" -o llm-api-check .
./llm-api-check --version
./llm-api-check config path
./llm-api-check accounts list   # 空配置不崩溃
```
Expected: 全部成功

- [ ] **Step 5: 提交**

```bash
git add -A && git commit -m "feat: CLI 入口 + 终端渲染（总览/详情/颜色/倒计时）"
```

## Task 7: 本地部署 + 文档

**Files:**
- Create: `README.md`、`README_zh.md`、`CONTEXT_FOR_NEXT_AGENT.md`
- Modify: 无

- [ ] **Step 1: 部署到 ~/.local/bin（关卡 11）**

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ~/.local/bin/llm-api-check .
command -v llm-api-check && llm-api-check --version && ls -l ~/.local/bin/llm-api-check
```

- [ ] **Step 2: 写 README.md + README_zh.md**（关卡 8 双语，顶部互链）

内容：功能特性（DeepSeek 余额/消费、OpenCode Go 三窗口、OpenCode Zen 账单、多账号）、快速开始、命令参考表、凭据获取方式（四类）、数据来源 API 表、安全说明（凭据 0600 权限存 ~/.config/llm-api-check/，与 Android 版 Keystore 加密的差异说明）、免责声明（Zen 数据来自页面解析可能随网页改版失效）、与 Android 版的关系。语言切换链接：README.md 顶部 `[**中文版**](README_zh.md)`，README_zh.md 顶部 `[**English**](README.md)`。

- [ ] **Step 3: 写 CONTEXT_FOR_NEXT_AGENT.md**（阶段 B 文档：项目状态、已完成工作、遗留问题——真实凭据需用户配置、真机 API 验证）

- [ ] **Step 4: 安全检查（关卡 9）**

```bash
grep -rniE 'sk-[a-zA-Z0-9]{16,}|Fe26\.2\*|Bearer [a-zA-Z0-9]{20,}' . --include='*.go' --include='*.md' 2>/dev/null | grep -v testdata || echo "无密钥泄漏"
git diff --cached | grep -iE 'sk-|Fe26|nvapi' || echo "暂存区无密钥"
```

- [ ] **Step 5: 最终提交**

```bash
git add -A && git commit -m "docs: README 双语 + CONTEXT 文档 + 部署"
```

## Task 8: 验收

- [ ] `go test ./...` 全绿
- [ ] `llm-api-check --version` 输出版本
- [ ] `llm-api-check` 无配置不崩溃，提示添加账号
- [ ] 用 httptest 或真实 API 冒烟：配置一个测试账号后 `llm-api-check status` 正常输出
- [ ] `NO_COLOR=1 llm-api-check status` 无 ANSI 转义
- [ ] `llm-api-check --json` 输出合法 JSON
- [ ] `llm-api-check accounts add/remove/list` 全流程可用
- [ ] `~/.local/bin/llm-api-check` 已部署且为最新构建
- [ ] 安全 grep 无密钥泄漏
- [ ] README 双语 + CONTEXT 存在

## Acceptance（验收标准）

1. `go test ./...` 全绿（≥15 个测试：解析器 ≥10 + config ≥3 + render ≥5 + repo ≥4）
2. 数据逻辑与 Android 版一致：Go 三窗口、Zen microcents÷1e8、DeepSeek 余额字符串金额、cost 两月聚合 today/7d/30d
3. 错误消息与对照表逐字一致
4. 多账号（DeepSeek 多个 + OpenCode 多个）独立展示、互不混淆
5. 凭据不硬编码、不落 git；配置文件 0600
6. 总览/详情两种视图 + `--json` 机器输出 + NO_COLOR 降级
7. README.md + README_zh.md 双语存在且互相链接
8. 二进制部署到 `~/.local/bin/llm-api-check` 并验证
