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
- ~~**配额/余额不可得**~~ → **2026-09-06 推翻，见 §二-b**。旧结论只覆盖了 `api.b.ai`
  一个域名（推理面确实只开放推理路径），未探 `chat.b.ai` 控制台自己的 API 面。
  同一把 sk- key 在控制台 tRPC 上能直读积分，无需浏览器 Cookie。
- **限流时限不可得**：推理响应无限流头；10 路并发 max_tokens=3 未复现 429
  （memory 里 429 见于长请求场景）。§六「限流时限可见」对 BAI 数据源不适用，
  非实现缺陷。

## 二-b、积分额度通道（chat.b.ai tRPC，2026-09-06 真实 key 实测）

### 取证路径

`chat.b.ai` 是 LobeChat 分支（路由表 `oauth:/api/auth` + `chat:/webapi/chat/{id}` +
`/webapi/models/{id}` 为 LobeChat 指纹）。从页面初始 chunk 反推 webpack 异步 chunk 表
（`u.u=c=>…` + `{id:"hash"}` 映射，共 89 个），在客户端 bundle 里找到数据层：

```js
// chunk 193735：额度服务类
class i {
  async getSummary(){ return a.du.usage.summary.query() }
  async getPoints() { return a.du.usage.points.query() }
  async records(e)  { return a.du.usage.records.query(e) }
}
// chunk 93731：store 消费方
let {points_balance:s, points_expiring:a} = await I.B.getPoints();
e({bonus:a??0, isPointsInit:!0, points:s})
// tRPC 客户端配置：url:"/trpc/lambda"
```

推理面 403 不代表控制台 403——两个域名、两套网关。实测：

| 端点 | 认证 | 实测结果 |
|:---|:---|:---|
| `GET https://chat.b.ai/trpc/lambda/usage.points` | `Authorization: Bearer sk-…` | 200 `{"result":{"data":{"json":{"points_balance":27166591,"points_expiring":7166591}}}}` |
| `GET …/trpc/lambda/usage.summary` | 同上 | 200 `{points_balance, monthly_spent:2833409, monthly_chart:[{month,points}×12]}` |
| `GET …/trpc/lambda/usage.records?input={"page":1,"pageSize":100}` | 同上 | 200 逐请求明细（model/tokens/cost_points/created_at/source_type/request_id） |
| `GET …/trpc/lambda/basicConfig.getBasicConfig` | 无需认证 | 200 `{creditsPerDollar:1000000, …}` |
| 以上全部 | 假 key / 无头 | 401 `{"error":{"json":{"message":"UNAUTHORIZED","code":-32001,"data":{"code":"UNAUTHORIZED","httpStatus":401,"path":"usage.points"}}}}` |

### 数值语义

- **单位**：平台积分（point）。`1 000 000 积分 = $1`，三个独立证据交叉：
  ① `basicConfig.creditsPerDollar = 1000000`；② `user.getRechargeBonusStat` 的
  `claimedAmountCents:1000` ↔ `claimedAmountPoints:10000000`；③ `order.listOrders`
  一笔 `points:10000000` 的 alipay 充值 `quantityDisplay:"$10.78"`（含支付手续费，
  故名义值取 ①② 的 1e6，渲染用 `≈ $` 标记提醒不是账单金额）。
- **`points_expiring` = 赠送额度余额**（购买积分不过期）。逐笔对账成立：
  账户共 2 笔 5 000 000 赠送 + 2 笔 10 000 000 购买 = 30 000 000；
  `30 000 000 − 2 833 409（monthly_spent）= 27 166 591 = points_balance`，
  `10 000 000 − 2 833 409 = 7 166 591 = points_expiring`——本月消耗全部取自赠送池。
  控制台 store 把该字段直接当 `bonus` 用，与上述算式一致。CLI 文案取字段本义
  「即将过期」（对用户更可操作：这部分不花就没了）。
- **额度耗尽会直接打断推理**：客户端有 `Insufficient points balance. Please recharge
  your account.`（403）分支，故余额 ≤0 时详情整行红色告警。

### 施工约束

- **必须走代理**：`chat.b.ai` 本机直连超时（`--noproxy '*'` 15s 无响应），`api.b.ai` 直连可通。
  Go 默认 `http.ProxyFromEnvironment` 与 curl 一致，不做特殊处理（同 §一）。
- 单次往返 ≈0.6 s（代理链路）。积分与模型清单两路在 app 层并发，但额度路内部
  `usage.points` → `usage.summary` 是串行两次往返 ≈1.2 s，**由额度路主导总时长**
  （真机冒烟 `bai` 全链 1.2 s 即此值）。要再快可把 summary 也并发，代价是多一处
  并发状态，当前收益不值。
- 只读：三个端点全是 tRPC query，无任何 mutate 接入（`user.claimSignupBonus` 等
  写操作刻意不碰）。
- 输入参数宽松得不可信：`usage.records` 对 `page:"x"`、未知过滤键一律忽略不报错，
  不能拿它做参数校验（同智星云 `status_type` 的教训）。

## 三、数据模型（models）

```go
type BaiAccount struct { ID, Name, ApiKey string }          // json: id/name/apiKey
type BaiModel  struct { ID, OwnedBy string; Endpoints []string } // json: id/owned_by/supported_endpoint_types
type BaiPlan   struct { Models []BaiModel }
type BaiPoints struct { Balance, Expiring, MonthlySpent int64; HasMonthly bool }
const BaiPointsPerDollar = 1_000_000                        // 积分→美元换算率
```

免费通道盯梢清单（`models.BaiFreeFlashModels`，2026-09-04 用户侧快照，仅提示不参与
断言）：`deepseek-v4-flash`、`deepseek-v4-flash-vision-exp`、`glm-5.3-flash`、
`qwen3.8-flash`。`BaiPlan.MissingFreeFlash()` 返回缺失项。

## 四、通道与错误

- repo：`BaiRepo{BaseURL="https://api.b.ai", ConsoleURL="https://chat.b.ai", Client}`；
  `Models(apiKey)` 走推理面 doGet（Bearer + Accept: application/json）→ `parsers.ParseBaiModels`；
  `Points(apiKey)` 走控制台 tRPC `usage.points` + `usage.summary` → `parsers.ParseBaiPoints` /
  `ParseBaiMonthlySpent`。
- 401/403 统一 → `parsers.ErrBaiAuth`（`BAI API Key 无效、已过期或额度用尽，请到 chat.b.ai 核对`）。
  one-api 对额度用尽/令牌过期也回 403，doGet 不区分状态码，文案须覆盖三种。
  **两路共用这一条常量**：同一把 key 坏掉时两路都会报，app 层 `joinErrors` 按整行
  去重，用户只读到一句（同 main.go `joinText` 的教训）。
- tRPC 信封错误优先于状态码：`error.json.data.code == UNAUTHORIZED` → 归一到
  `ErrBaiAuth`；其他 code 带原文上抛，不谎报「key 有问题」。
- 解析：信封 `success=false` 且无 data → 显式错误；data 空 → 「未获取到 BAI 模型」；
  id 去重 + 按 id 排序（对齐 ParseQwenModels 惯例）；owned_by 原样保留。
- 数字形状：`rawInt64` 容忍整数 / 浮点（`2.7e6`）/ 字符串（`"12345"`）三种 JSON 形状，
  `null` 视为缺席。tRPC 负载由 JS 序列化，严格解析会把大整数误归零（同 galaxy `rawInt` 的
  oracle 对照教训）。`points_balance` 缺席 = 显式失败，**不得显示 0**（0 会被读成「额度用尽」）。

## 五、渲染

详情（RenderBaiDetail）：
```
白B.AI (BAI)
API · 免费 0-Credits flash 通道
  积分余额 27,166,591（≈ $27.17） · 其中 7,166,591 即将过期
  本月消耗 2,833,409（≈ $2.83）
  模型     48 个：…
  免费通道 ✓ deepseek-v4-flash / …（全在清单时只列名字）
  免费通道 ⚠ 缺失：qwen3.8-flash（pi-subagent 默认免费模型源受影响）  ← 有缺失时
```
- 标签列统一走 `baiLabel`（= `padTo(s, 8)`，按显示宽度）。旧实现用 `%-12s` 配中文标签
  按字节补齐，与硬编码空格的「免费通道」行不同列，本轮一并收敛。
- 余额 ≤0 → 整行红色 + 「额度已耗尽，推理请求会失败」（不设主观「偏低」阈值，
  0 是唯一有依据的红线）。
- `HasMonthly=false`（summary 通道失败）→ 不显示本月消耗行，也不报错。
- 空 key → 灰色指引行（对齐 qwen）；--no-refresh 无数据 → 灰「暂无数据」。
- 总览（writeBaiOverview）：`积分 N（≈ $x）· 本月消耗 M` + `模型 N 个 · 免费通道 k/4` + 错误行。

## 六、CLI

- `llm-api-check bai [名称|ID] [--no-refresh]`；`accounts add --type bai --name N --api-key sk-…`
  （env `LLM_API_CHECK_BAI_API_KEY` → TTY 回退）；list/remove/rename/config 收编。
- exit code：`Error != ""` 且 `Plan == nil` 且 `Points == nil` → 1（任一路有数据即 0）。
- `--json`：apiKey 掩码（publicBaiAccount）；`bai.points` = `{balance, expiring, monthlySpent, hasMonthly}`；
  status JSON 增 `bai` 数组。

## 七、测试与质量门

- parsers：happy / success=false 信封 / 空 data / 去重排序 / 非法 JSON；
  积分信封（happy / 三种数字形状 / 缺席 expiring / UNAUTHORIZED / 非授权错误带原文 /
  缺 points_balance / null 负载）/ 越界形状（`1e30`、`9.3e18`、`"Inf"`、`"NaN"` 等 8 例
  必须显式失败，2^53-1 边界内不得误拒）。
- repo：httptest wire format（Bearer 头、路径）、401、403、网络错误、非 JSON 200；
  积分两路路径+认证头逐条比对、summary 失败不致命、空 ConsoleURL 退回默认端点
  （RoundTrip 桩，不联网）。
- app：RefreshBai 成功/失败/账号不存在；模型路失败保额度、额度路失败保模型；
  summary 可选；三路同文本错误去重；RefreshAll 收编。
- render：全在 / 缺失 / 空 key / 错误 / 总览行；积分三态（有过期 / 无过期 / 耗尽红色）；
  标签列显示宽度对齐断言（`valueColumn`）。
- main：add → list → --no-refresh → rename → remove 全链（不联网）；
  `publicBaiResult` points 键 + 掩码；`exitCodeForResults` 三态。
- 质量门：gofmt 0 / vet 0 / 7 包 `-race` 全绿；用例 242 → 258；**反向验证 10 做**
  （删 tRPC 信封分支、rawInt64 去容错、**rawInt64 去越界守卫**、summary 改致命×2、
  去错误去重、退出码漏 Points、标签退回 %-12s、「暂无数据」漏 Points、tRPC 路径写错
  ——各自对应测例均 FAIL）；
- **momus 审查轮（2026-09-06）**：无 P0；P1-1 = `rawInt64` 越界/Inf 缺守卫会把解析失败
  伪装成「额度已耗尽」红警（先实测复现 `-9223372036854775808` 再修）；P2 修 3 条
  （§施工约束的时延算术、`RefreshBai` 注释、测例双重否定），1 条留档未修
  （tRPC 错误原文未消毒——与全仓同模式，须统一修而非单点修）。
  新二进制装 `~/.local/bin` + 真机四路冒烟（有效 key / 坏 key / --json / --no-refresh）。
