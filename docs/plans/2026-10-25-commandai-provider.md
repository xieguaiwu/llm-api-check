# Commandai Provider Implementation Plan

> **For agentic workers:** 按本计划任务逐项实施。步骤用 checkbox（`- [ ]`）跟踪。

**Goal:** 新增 `provider=commandai`（Command Code 平台，commandcode.ai）——模型清单 + 额度盯梢（积分三池 + 5小时/每周滚动窗口 + 套餐周期 + 本期消耗），照 longcat/bai 模板全链接入 Go CLI。

**Architecture:** 双通道单 repo——模型面 `GET /provider/v1/models`（OpenAI 兼容信封，实测免认证可用，仍带 Bearer）与额度面 `/alpha/*`（CLI 私有端点，Bearer 认证，whoami 取 orgId 后 credits ∥ subscriptions 并发，再 summary）。额度语义=美元浮点数；窗口倒计时遵循 index.md §六「已限流徽章与重置倒计时并存」。

**Tech Stack:** Go 标准库（net/http + httptest + encoding/json），零第三方依赖（house 惯例）。

**Spec:** 本文件即规格。线协议来自三源交叉验证（2026-10-25）：① 官方文档 commandcode.ai/docs/provider + /docs/resources/pricing-limits + /docs/headless；② npm 包 command-code@1.88.0 的 dist/cli.mjs 逆向（usage 通道与字段名 ground truth）；③ 本机无认证探测（401 信封原文、models 免认证实测 93 模型）。

## Global Constraints

- 模块路径 `github.com/xieguiawu/llm-api-check`——import 段必须与 go.mod 逐字节一致。**近形标识符陷阱**：曾出现 `xieguiawu`→`xieguiawu` 字节转置，每次写完 Go 文件脚本校验所有 import 前缀。
- 凭据真值：key 只存 config.json（chmod 0600 已有）+ 环境变量 `LLM_API_CHECK_COMMANDAI_API_KEY`；`--json` 输出全部掩码；错误消息不带 key 原文。
- 错误文本：先 `parsers.SanitizeText` 后截断（顺序反了会切断 ESC 序列——CONTEXT 铁律）。
- 对齐：中文标签用 `render.padTo`/`longCatLabel` 同款显示宽度补位，禁 `%-Ns`。
- 倒计时文案「N小时M分后重置 / N分钟后重置 / 即将重置」（FormatCountdown 既有）。
- 颜色阈值 <70 蓝 / 70-89 黄 / ≥90 红；已限流/额度用尽强制红且徽章+倒计时并存（index.md §六）。
- 质量门：gofmt 空 / go vet 干净 / 7 包 `go test ./... -race` 全绿；反向验证至少 4 做（见 §六）。
- 构建部署：`go build -trimpath -ldflags="-s -w -X main.version=…" -o ~/.local/bin/llm-api-check .`（关卡 11）。
- 文档：README.md 与 README_zh.md 同 commit 同步（关卡 5/8）。
- 用户可见文本 ASD-STE100：短句、祈使、术语一致（关卡 13）。
- 探活纪律：**不做推理探活**——额度有真值通道（/alpha/*），探活烧真钱（pay-as-you-go）。key 有效性由 whoami 判定。

## 一、线协议（三源验证，2026-10-25）

### 1.1 Base 与认证

- Base：`https://api.commandcode.ai`（npm `In={prod:…,staging:…,local:…}`）。
- 认证：`Authorization: Bearer <key>`（官方文档：any route；`x-api-key` 仅 /messages 兼容 Anthropic SDK——本工具只用 Bearer）。key 在 Studio（commandcode.ai/studio）创建；env 名 `COMMAND_CODE_API_KEY`（官方），本工具 env 为 `LLM_API_CHECK_COMMANDAI_API_KEY`。
- 客户端 UA：用既有 `browserUA`（防御惯例，同 longcat）。

### 1.2 模型面（实测免认证，HTTP 200）

`GET /provider/v1/models` → 200，OpenAI 兼容信封（实测原样，93 模型）：

```json
{"object":"list","data":[
  {"id":"claude-sonnet-5-5","object":"model","created":1792901766,"owned_by":"command-code",
   "name":"Claude Sonnet 5.5","context_length":1000000,"supported_endpoints":["/messages"]}
]}
```

- `supported_endpoints` ∈ `/messages`（11 个 Claude 系）、`/chat/completions`（82）、`/responses`（72）。
- 401 信封（实测原样，chat/completions 无认证）：

```json
{"success":false,"error":{"code":"UNAUTHORIZED","status":401,
 "message":"Invalid 'Authorization' header or token.",
 "docs":"https://commandcode.ai/docs/reference/errors/unauthorized"}}
```

- 官方错误码表：400 `unsupported_model` / 400 `invalid_request_error` / 401 `authentication_error`（实际信封 code 为 `UNAUTHORIZED`）/ 403 `upgrade_required`（Go 套餐无 API）/ 422 `cmd_zdr_no_providers`（本工具不发 x-cmd-zdr，N/A）/ 429 `rate_limit_error` / 5xx `server_error|api_error`。
- **信封优先于状态码**（bai tRPC 先例）：能解析出信封时按 `error.code` 归类，不能才按 HTTP 状态。

### 1.3 额度面（/alpha/*，CLI 私有，全部 GET + Bearer）

调用序列（npm `fetchUsageData` 原样还原）：

```text
GET /alpha/whoami?limits=1                          → data.org?.id（orgId 参数源，可空）
GET /alpha/billing/credits[?orgId=<id>]             ┐ 并发
GET /alpha/billing/subscriptions[?orgId=<id>]       ┘
GET /alpha/usage/summary[?orgId=<id>][&since=<currentPeriodStart>]   （since 取 subscription.currentPeriodStart；无则省略）
```

响应字段（**2026-10-25 真 key 实测回填，一手样本**；与 npm 消费点 `e.credits?.windowLimits` 交叉核对一致）：

- whoami：`user.{id,name,email,userName}` + `org`（**个人账号实测 `org:null`**，团队账号才有 `{id,login}`）。另有 `orgLimits[]`（形状未知，v1 不接不渲染）。
  - **`org` 为 null 时额度端点必须省略 orgId**：实测 `?orgId=`（空串）→ 400 `BAD_REQUEST`「Invalid UUID at "orgId"」；省略则 200。故 `ParseCommandaiWhoami` 不再把 `org.id` 当必需字段。
- credits（**实测原文**）：
  ```json
  {"credits":{"belowThreshold":false,"creditThreshold":0,"monthlyCredits":9.4306827317,
              "purchasedCredits":0,"freeCredits":0},
   "windowLimits":{"limited":true,"exceeded":null,
              "fiveHour":{"used":0.054105,"cap":3,"exceeded":false,"resetAt":1792948252329},
              "weekly":{"used":0.5693172683,"cap":6,"exceeded":false,"resetAt":1793511581667}},
   "sandboxAccess":false,"sandboxMinutes":null}
  ```
  - **三池嵌在 `credits` 内**（`monthlyCredits`/`purchasedCredits`/`freeCredits`）；**`windowLimits`/`sandboxAccess`/`sandboxMinutes` 与 `credits` 平级**（初版误按嵌套实现，实测修正）。
  - **`credits` 内没有 `planId`**——套餐名在 `subscriptions.data.planId`（npm `createBilling` 同）。
- subscriptions：`data.{id,status,currentPeriodStart,currentPeriodEnd,planId,userId,orgId,priceId,metadata,quantity,cancelAtPeriodEnd,…}`（信封 `{success:true,data:{…}}`）。`currentPeriodStart/End` 为 ISO-8601（`"2026-10-25T04:15:41.000Z"`）。
- summary：`totalCost`（美元）、`totalCount`（请求数）、`periodBasis`（`"billing-period"`），另有 `averageCost/successRate/completedCount/failedCount/totalTokens*/totalCredits*/periodStart/daily`。
- windowLimits：`{limited: bool, exceeded: null|…, fiveHour?: {used, cap, exceeded, resetAt}, weekly?: {…}}`；**`resetAt` 实测为 unix 毫秒数**（1792948252329 > 1e12）。解析器仍按多形状容错（数值 >1e12 毫秒 / ≤1e12 秒 / 字符串 RFC3339；全失败=无倒计时，不影响 used/cap 渲染）。
  - **各窗口自带 `exceeded` 布尔**（初版遗漏）——「已限流」的唯一判据。
- sandboxMinutes：`{limitMinutes, usedMinutes, resetAt}`（`null` = 无沙箱权限；v1 渲染一行，缺省不显示）。

### 1.4 数值语义（npm `projectUsageView` 公式，逐条照抄）

- 积分单位 = 美元（CLI `formatCredits(x)=$x.toFixed(2)`）。浮点数直存，渲染两位小数。
- `totalRemaining = monthlyCredits + purchasedCredits + freeCredits`（各项 max(0,·)）。
- `spent = summary.totalCost`。
- `planMonthly`：`status=="active"` 时取套餐映射表值，否则 null。映射（npm `Er` 表）：`individual-go`=10、`individual-go-v1`=10、`individual-goat`=70、`individual-pro-v1`=80、`individual-provider`=15、`individual-max`=150、`individual-ultra`=300、`teams-pro`=40。`getPlanInfo` 匹配法：planId 小写化、`_`→`-`，按最长前缀命中。
- `totalPool = planMonthly!=null ? max(planMonthly, credits.monthlyCredits) + purchased + free : spent + totalRemaining`（Provider 套餐等无订阅场景走后者）。
- `usagePercent = totalPool>0 ? clamp((totalPool-totalRemaining)/totalPool*100, 0, 100) : 0`。
- `daysLeft = ceil((currentPeriodEnd - now)/86400)`，<0 归 0；renewal urgency：<3 critical 红 / <7 warning 黄 / 其余 normal。
- **已限流判据 = 窗口自身 `exceeded`（或 used ≥ cap 兜底）→ 徽章 + `resetAt` 倒计时并存**（§六铁律）。
  - ⚠️ 初版按 `windowLimits.limited` 判限流是**误读**：npm `projectUsageView` 只在 `S?.limited` 为真时决定是否渲染「Usage limits」段（即「本套餐配置了额度限制、值得展示」），实测个人 Go 套餐 `limited=true` 而两个窗口 `exceeded=false`、用量仅 3%/10%——拿它当限流判据会误报。
- 无窗口限制的套餐（Provider = pay-as-you-go，官方文档）：windowLimits 为 null/缺省 → 只渲染积分三池 + 已花，不渲染窗口段。

## 二、数据模型（Task 1a：internal/models/models.go 追加「── Commandai（Command Code）───」段）

```go
// CommandaiAccount 单账号。key 于 commandcode.ai/studio 创建（Provider API 用）。
type CommandaiAccount struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	ApiKey string `json:"api_key"`
}

// CommandaiModel 模型清单项（/provider/v1/models，字段实测原样）。
type CommandaiModel struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	ContextLength      int64    `json:"context_length"`
	SupportedEndpoints []string `json:"supported_endpoints"`
	OwnedBy            string   `json:"owned_by"`
}

// CommandaiPlan 模型清单。
type CommandaiPlan struct{ Models []CommandaiModel `json:"models"` }

// CommandaiWindowLimit 单个滚动窗口（used/cap 美元；resetAt 多形状容错）。
type CommandaiWindowLimit struct {
	Used    float64    `json:"used"`
	Cap     float64    `json:"cap"`
	ResetAt *time.Time `json:"-"`      // 解析自原始 JSON（见 parsers.commandaiTimeOf）
	ResetAtRaw string   `json:"-"`     // 原文兜底（解析失败时渲染「未知」，不二次猜）
}

// CommandaiWindowLimits 5小时/每周滚动窗口（订阅套餐才有；Provider 套餐无）。
type CommandaiWindowLimits struct {
	Limited  bool                 `json:"limited"`
	FiveHour *CommandaiWindowLimit `json:"fiveHour"`
	Weekly   *CommandaiWindowLimit `json:"weekly"`
}

// CommandaiSandbox 云沙箱分钟数（可选）。
type CommandaiSandbox struct {
	LimitMinutes float64    `json:"limitMinutes"`
	UsedMinutes  float64    `json:"usedMinutes"`
	ResetAt      *time.Time `json:"-"`
	ResetAtRaw   string     `json:"-"`
}

// CommandaiCredits 积分三池 + 窗口 + 沙箱（/alpha/billing/credits）。
type CommandaiCredits struct {
	PlanID          string                `json:"planId"`
	MonthlyCredits  float64               `json:"monthlyCredits"`
	PurchasedCredits float64              `json:"purchasedCredits"`
	FreeCredits     float64               `json:"freeCredits"`
	WindowLimits    *CommandaiWindowLimits `json:"windowLimits"`
	SandboxMinutes  *CommandaiSandbox     `json:"sandboxMinutes"`
	SandboxAccess   bool                  `json:"sandboxAccess"`
}

// CommandaiSubscription 套餐与周期（/alpha/billing/subscriptions）。
type CommandaiSubscription struct {
	PlanID             string     `json:"planId"`
	Status             string     `json:"status"`
	CurrentPeriodStart *time.Time `json:"-"`
	CurrentPeriodEnd   *time.Time `json:"-"`
	// PeriodStartRaw 原文兜底 + repo summary 的 since 参数源（§四：解析成功也回传原文）。
	PeriodStartRaw string `json:"-"`
	PeriodEndRaw   string `json:"-"`
}

// CommandaiUsageSummary 本期消耗（/alpha/usage/summary，since=currentPeriodStart）。
type CommandaiUsageSummary struct {
	TotalCount  int64    `json:"totalCount"`
	PeriodBasis string   `json:"periodBasis"`
	TotalCost   float64  `json:"totalCost"`
}

// CommandaiWhoami 身份（/alpha/whoami，白名单字段）。
type CommandaiWhoami struct {
	OrgID    string `json:"orgId"`
	OrgLogin string `json:"orgLogin"`
	UserName string `json:"userName"`
}
```

Period 时间原始形状未一手确认（ISO 或 unix）→ 与 resetAt 同一 `commandaiTimeOf` 多形状容错；解析失败=nil、原文存 Raw 字段（渲染层显式「未知」而非 0 误导，repo 的 since 参数取 `PeriodStartRaw`）。

派生函数（models 层，纯函数，Android/其他端可对齐）：

```go
// CommandaiPlanDisplayName 套餐显示名（Er/Mr 映射 + 最长前缀匹配）。
func CommandaiPlanDisplayName(planID string) string  // "individual-provider"→"Provider"；未知→原文
// CommandaiPlanMonthlyCredits 套餐月度积分映射（Er 表）；未知套餐 → ok=false。
func CommandaiPlanMonthlyCredits(planID string) (float64, bool)
// CommandaiBilling 集中公式（见 §1.4），返回 totalPool/usagePercent/daysLeft。
type CommandaiBilling struct {
	TotalRemaining, TotalPool, Spent, UsagePercent float64
	DaysLeft int  // -1 = 周期未知
	HasBilling bool
}
func CommandaiBillingOf(c *CommandaiCredits, s *CommandaiSubscription, u *CommandaiUsageSummary, now time.Time) CommandaiBilling
```

## 三、解析器（Task 1b：internal/parsers/parsers.go 追加 commandai 段）

```go
var ErrCommandaiAuth = errors.New("Commandai API Key 无效或已过期，请到 commandcode.ai/studio 创建或核对 API key")
var ErrCommandaiUpgrade = errors.New("当前套餐不含 API 访问（Go 套餐），请升级到 GOAT 或更高")

func ParseCommandaiModels(raw string) ([]models.CommandaiModel, error)
    // {object:"list",data:[…]}；data 缺席=显式失败；空数组=合法 0 模型（不得静默吞）。
    // 信封 401（code=UNAUTHORIZED）→ ErrCommandaiAuth。
func ParseCommandaiWhoami(raw string) (models.CommandaiWhoami, error)
    // org.id 缺席=显式失败（orgId 是后续链路参数源）；org.login/user.userName 缺席容错空串。
    // 信封 401 → ErrCommandaiAuth；code=upgrade_required → ErrCommandaiUpgrade。
func ParseCommandaiCredits(raw string) (models.CommandaiCredits, error)
    // credits 对象缺席=显式失败；三池字段缺席按 0（合法场景：纯充值户）；windowLimits/sandbox 缺席=nil。
func ParseCommandaiSubscription(raw string) (models.CommandaiSubscription, error)
    // data 缺席=显式失败；status 不在合法集仍原样保留（渲染层标注），period 解析失败=nil。
func ParseCommandaiUsageSummary(raw string) (models.CommandaiUsageSummary, error)
    // totalCost/totalCount 缺席=显式失败（不显示 0 误报——bai points_balance 先例）。
func commandaiEnvelopeOf(raw string) (code, message string, ok bool)
    // {success:false,error:{code,status,message,docs}} 提取；message 过 SanitizeText 后截断。
func commandaiTimeOf(raw json.RawMessage) *time.Time
    // 数值 >1e12 → unix 毫秒；≤1e12 → unix 秒；字符串 → RFC3339/ISO 宽容解析；失败 → nil。
```

命名统一 `Commandai` 前缀（与 provider 名一致）；错误信封 message 必须消毒（防 ANSI 注入，bai `ParseBaiErrorEnvelopeSanitized` 同款）。

## 四、repo（Task 2：internal/repo/commandai.go 新文件，197 行量级）

```go
const commandaiBaseURL = "https://api.commandcode.ai"

type CommandaiRepo struct {
	BaseURL string
	Client  *http.Client
}
func NewCommandaiRepo() *CommandaiRepo  // 默认端点 + defaultClient()

// Models 模型清单。GET {base}/provider/v1/models，Bearer。
// 空 key 合法（端点免认证实测）——但仅当账号显式无 key 时调用（app 层守卫）。
func (r *CommandaiRepo) Models(apiKey string) (models.CommandaiPlan, error)

// Quota 额度链：whoami → (credits ∥ subscriptions) → summary(since)。
// 返回四元组；whoami 401 → ErrCommandaiAuth（额度路整体失败，模型路不受影响）。
// 单段失败不静默：返回已成功段 + 错误（app 层 joinErrors）。
func (r *CommandaiRepo) Quota(apiKey string) (*models.CommandaiWhoami, *models.CommandaiCredits, *models.CommandaiSubscription, *models.CommandaiUsageSummary, error)
```

- 全部 GET；头：`Authorization: Bearer`、`Accept: application/json`、`User-Agent: browserUA`。
- HTTP 错误处理：401/403 → 解析信封，`code=UNAUTHORIZED|authentication_error` → ErrCommandaiAuth；`upgrade_required` → ErrCommandaiUpgrade；其他 → `fmt.Errorf("HTTP %d: %s", code, truncate200(SanitizeText(body)))`（truncate200 既有，先消毒后截断已内建）。
- summary 的 since：`currentPeriodStart != nil` 或 `PeriodStartRaw != ""` → query 参数 `since=<PeriodStartRaw>`（原文回传，服务端按己方格式解释）；都空 → 省略参数。
- timeout：沿用 defaultClient()（15s）；全链 3 跳串行 ≈ 上限 45s 极端情形，可接受（longcat 控制台链同量级）。

## 五、app（Task 3：internal/app/app.go）

```go
// CommandaiResult 单账号刷新结果。额度四段指针 nil=该段不可用。
type CommandaiResult struct {
	Account      models.CommandaiAccount        `json:"account"`
	Plan         *models.CommandaiPlan          `json:"plan,omitempty"`
	Whoami       *models.CommandaiWhoami        `json:"whoami,omitempty"`
	Credits      *models.CommandaiCredits       `json:"credits,omitempty"`
	Subscription *models.CommandaiSubscription  `json:"subscription,omitempty"`
	Summary      *models.CommandaiUsageSummary  `json:"summary,omitempty"`
	Error        string                         `json:"error,omitempty"`
}
// App struct 追加：Commandai []CommandaiResult；Repos 追加 Commandai *repo.CommandaiRepo
// RefreshCommandai(id) (CommandaiResult, error)  按 id 刷新单账号
// refreshCommandai(acc) CommandaiResult          两路并发：Models ∥ Quota
// RefreshAll 追加 commandai 账号并发 lane（goroutine + WaitGroup，同 longcat 模式）
```

- 错误聚合：quota 路错误 + models 路错误 joinText 去重合并（既有 joinErrors/joinText 口径）；`errors.Is(err, parsers.ErrCommandaiAuth)` 时 Error 单行化，不再叠 models 错误（longcat 同款：认证失败主行只读一句）。
- 无 key 防御：ApiKey 空串 → 不发额度请求，Models 正常拉，Error 提示「未配置 API Key（额度信息不可用）」。
- Result struct 与 `New()`/`NewWithRepos()`/status 汇总接线同 longcat。

## 六、渲染（Task 4：internal/render/render.go）

```go
func RenderCommandaiDetail(r app.CommandaiResult, now time.Time, c Colorizer) string
// overview 追加：RenderOverview 的 commandai 行（一行/账号）
```

详情段（中文，标签列 `commandaiLabel=padTo(s,显示宽度)` 同 longcat 口径）：

```text
Commandai · <账号名>
  套餐      Provider（active）· 本期剩 12 天后重置          ← daysLeft；无周期→「周期未知」
  额度      $13.20 可用 / 总池 $15.00 [█████████░] 12% 已用   ← 颜色阈值；条 10 格
    月度    $10.00 · 充值 $3.20 · 免费 $0.00
  本期已花  $1.80 · 42 次请求                                ← summary；缺→不渲染
  5小时窗口 $14.20/$14.00 [██████████] 100% · 已限流 · 2小时13分后重置   ← limited=true 时红+徽章+倒计时并存
  每周窗口  $30.00/$35.00 [████████░░] 86% · 3天后重置
  沙箱      47/120 分钟                                     ← sandboxAccess 且有值才渲染
  模型      93 个（Claude 系 11 · OpenAI 兼容 82 · 双端点 72） ← 按 supported_endpoints 计数
    claude-sonnet-5-5（1M · /messages）… 列前 8 个 + 「…等 85 个」
```

- 倒计时复用 `FormatCountdown`；`已限流`徽章与倒计时**必须并存**（§六/index.md 项目专属要求——测例锁定）。
- usagePercent=0 且 totalRemaining=0 且 spent>0 → 红色「额度已耗尽」。
- 错误行：`Error` 非空时红色单行；模型路成功时仍显示模型段。
- 总览行：`命令名 账号名 · 套餐 · [条] N% · 状态注记`，已限流注记带倒计时。

## 七、main 接线（Task 5：main.go）

- `usageText`：首行 provider 清单追加「Commandai」；新增 `llm-api-check commandai [名称|ID]` 行；accounts add --type 清单追加 `commandai`；凭据段追加 `Commandai: --api-key（commandcode.ai/studio 创建）`。
- `cmdCommandai`：flagset `--no-refresh`/`--limit N`（moveFlags 搬运两 flag——moveFlags 已认 flag 集合，需把 `--limit` 加进通用搬运表）；位置参数按 id/name 过滤（filterCommandai 同 filterLongCat）。
- accounts：add（`--api-key` flag / env `LLM_API_CHECK_COMMANDAI_API_KEY` / TTY prompt；缺凭据非 TTY → exit 2）/ list（掩码显示）/ remove / rename——照 cmdAccountsAdd 现有分支模式追加。
- `--json`：`publicCommandaiResult(s)`（key 掩码 `maskKey`，复用既有 mask 函数名——先 grep 确认真名）+ `publicCommandaiAccounts`；status 总览 `commandai` 键。
- `exitCodeForResults`：追加 commandai 口径——认证失败/无数据 → 1；探活成功仅额度段错误 → 0（qwen/longcat 同款）。
- config：`CommandaiAccounts []models.CommandaiAccount json:"commandai_accounts"` + `SaveCommandaiAccount`/`DeleteCommandaiAccount`。

## 八、测试（各 Task 内嵌；目标 399 → ≥445）

- parsers（~12）：models happy（用实测 93 模型截取样本做 fixture）/ data 缺席失败 / 空数组合法 / 信封 401 归一 / upgrade_required 归一 / whoami org.id 缺失失败 / credits happy+windowLimits 完整 / credits 三池缺席=0 / summary 字段缺失失败 / commandaiTimeOf 四形状（毫秒/秒/ISO/垃圾→nil）/ message 消毒断言（ESC 序列剥离）。
- repo（~8）：httptest models 200→解析 / 401 信封→ErrCommandaiAuth / 403 upgrade_required→ErrCommandaiUpgrade / Quota 全链 happy（断言 whoami→credits+subscriptions 并发命中、summary 的 since 参数=periodStart 原文）/ whoami 401 短路 / credits 失败但 subscription 成功=部分返回+错误。
- app（~6）：双路并发 happy / 认证失败单行 / 无 key 降级（models 有、额度 hint）/ 账号不存在错误 / RefreshAll 含 commandai。
- render（~8）：详情 happy 全段 / 已限流=徽章+倒计时并存（断言两串同时出现）/ 窗口缺席不渲染窗口段 / 额度耗尽红 / 错误行 / 总览行 / NO_COLOR 无 ANSI。
- main（~10）：add from env / add 缺凭据非 TTY exit 2 / list 掩码 / remove / rename / --json 掩码（key 不得出现）/ exit code 认证=1 / --no-refresh 不联网 / usage text 提及 commandai / status 总览键。

## 九、反向验证清单（审查轮执行，≥4 做）

1. 拆 `commandaiEnvelopeOf` 信封归类 → 401 用例 FAIL（退化为 HTTP 码路径）。
2. 拆「已限流徽章与倒计时并存」→ render 用例 FAIL。
3. 拆 summary since 参数 → repo 用例 FAIL（URL 无 since）。
4. 拆 `--json` 掩码 → main 用例 FAIL（key 泄漏断言）。
5. 拆 totalPool 公式的 max(planMonthly, monthlyCredits) 分支 → models 公式用例 FAIL。
6. 拆无 key 降级 → app 用例 FAIL（空 key 不发额度请求）。

## 十、施工顺序与验收

- [ ] Task 1 models + parsers（TDD：先测后码，逐段 -race 绿）
- [ ] Task 2 repo + httptest
- [ ] Task 3 app 接线
- [ ] Task 4 render
- [ ] Task 5 main 全接线（accounts 生命周期 + --json + exit code + usage text）
- [ ] Task 6 README.md + README_zh.md 同步（功能行/数据源/凭据段/常见问题）+ 契约回填（真 key 冒烟后）
- [ ] 质量门全绿：gofmt / vet / 7 包 -race / import 前缀校验脚本
- [ ] 构建 + 安装 ~/.local/bin + 冒烟：无账号提示 / 假 key 账号（models 实通 + 额度 401 单行错误 exit 1）/ --json 掩码 / --no-refresh
- [ ] 不得 commit（沿用惯例：等用户确认）

## 十一、已知未知（2026-10-25 真 key 冒烟后已回填）

1. ~~/alpha/* 真实 JSON 顶层形状~~ → **已回填，见 §1.3**。三处修正：`org` 可空（`?orgId=` 空值 400）、`windowLimits`/`sandbox*` 在顶层、各窗口带 `exceeded`。
2. ~~`resetAt` / `currentPeriod*` 原始形状~~ → **`resetAt` 实测 unix 毫秒；period 实测 ISO-8601**（commandaiTimeOf 多形状容错仍保留）。
3. `orgLimits[]` 形状 → 仍未一手取得（实测响应里根本没这个键）；v1 未接不渲染。
4. 429 信封是否带 `rateLimit{window,reset}` 重试提示 → 未触发（本期 390 次请求未限流）；v1 直通 message。
