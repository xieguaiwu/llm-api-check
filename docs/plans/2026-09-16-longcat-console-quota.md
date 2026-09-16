# LongCat 控制台配额通道实施计划（Phase 1 = Go CLI）

> 2026-09-16 — LongCat 控制台 Cookie 配额通道 + 三缺陷修复

## 一、背景与事实（上游实测确认，2026-09-16）

1. `api.longcat.chat`（App Key 通道）**没有任何配额接口**：仅 `/openai/v1/models`、
   `/openai/v1/chat/completions`、`/anthropic/v1/messages`；12 个 OpenAI 式账单/额度路径全 404；
   真实响应头无 quota/ratelimit 字段。
2. `longcat.chat`（平台控制台）有可用配额接口，**认证只需 Cookie `passport_token_key=<值>`**
   （无需 x-requested-with、无签名、无风控头）。
3. 平台未公开「每日免费额度」剩余，任何 `/free-quota` 类路径 404 —— 本功能**不承诺**显示免费额度，
   只显示资源包余量 + 按量余额。

## 二、契约（2026-09-16 实测）

### 2.1 端点（全部 POST，`Content-Type: application/json`，body `{}`）

| 端点 | 语义 |
|---|---|
| `https://longcat.chat/api/pay/quota/metering/token-packs/summary` | Token 资源包钱包：剩余/总量/已用/有效期/预计耗尽 |
| `https://longcat.chat/api/pay/quota/metering/api-usage/summary` | 按量计费余额 |

### 2.2 真实响应（2026-09-16 实测原样，测试 fixture 的 ground truth）

`token-packs/summary`（该账号无资源包）：

```json
{"code":0,"msg":"success","data":{"currentLot":null,"estimate":{"windowDays":7,"dailyAverageToken":0,"exhaustedAfterDays":0},"otherLots":[]}}
```

`api-usage/summary`：

```json
{"code":0,"msg":"success","data":{"paygoBalanceCent":0,"paygoStatus":"NORMAL","rechargeEnabled":true,"statusTip":"账户余额已耗尽，请及时充值以确保 API 正常使用","paygoBalance":{"primary":{"currency":"CNY","amount":"0.00"},"secondary":null},"exchangeRate":6.8}}
```

无效 Cookie（实测）：

```text
HTTP 401
{"code":401,"msg":"登录状态无效，请重新登录","data":null}
```

`currentLot` 非空时的字段形状（从前端 bundle 反查，字段名确定）：
`{remainingToken, totalToken, consumedToken, consumedRatio(0..1), expireTime(毫秒时间戳), remainSeconds, grantCategory("GIFT" 表示免费发放，其它为付费)}`；
`otherLots` 是同样对象的数组；`estimate.exhaustedAfterDays` 为按当前速率耗尽天数。

### 2.3 降级语义

| 场景 | 行为 |
|---|---|
| 无 Cookie | 保持现状：`Models` + `ProbeBalance`；不报错、不提示失败 |
| 有 Cookie，控制台成功 | 展示精确额度；探活仍并行执行（保留 `/v1/models` 权限校验语义） |
| 有 Cookie，Cookie 失效 | `ErrLongCatConsoleAuth` 文案，**不影响**探活与模型清单的展示 |
| 有 Cookie，网络失败 | 中文包装「网络请求失败: …」，其余通道正常 |
| `currentLot:null` | 合法语义 = 无资源包（**不是错误**），渲染「无 Token 资源包」 |
| 部分字段缺失 | 除 `data` 整体缺失外，允许字段缺省（前端可能裁剪），但 `code` 缺失/非 0 必须报错 |

## 三、施工（Go CLI）

| 层 | 改动 |
|---|---|
| `internal/models/models.go` | `LongCatAccount` 加 `ConsoleCookie` + `HasCookie()`；`LongCatModel` 扩 `DisplayName/ContextWindow/MaxOutputTokens`；新增 `LongCatLot/LongCatEstimate/LongCatQuota/LongCatPaygo/LongCatPaygoBalance/LongCatPaygoAmount`（JSON 全 snake_case） |
| `internal/parsers/parsers.go` | `ParseLongCatTokenPacksSummary` / `ParseLongCatPaygoSummary`（信封 `{code,msg,data}`，code 用 `*int` 区分缺失与 0；401 → `ErrLongCatConsoleAuth`）；`ParseLongCatModels` 提取三个能力字段 |
| `internal/repo/longcat.go` | `longcatConsoleBaseURL = "https://longcat.chat"`；`LongCatRepo.ConsoleURL` 注入点 + `consoleBaseURL()`；`ConsoleQuota`/`ConsolePaygo`（POST body `{}`，Cookie 容错：无 `=` 自动补 `passport_token_key=`；401/403 或信封 401 → `ErrLongCatConsoleAuth`） |
| `internal/app/app.go` | `LongCatResult` 加 `Quota`/`Paygo`；`refreshLongCat` 有 Cookie 时追加两路并发（`hasCookie` 门控，无 Cookie 时 Quota/Paygo 保持 nil）；错误合并 `joinErrors(errorr, planErr, quotaErr, paygoErr)` |
| `internal/render/render.go` | 详情页控制台配额段（Token/有效期/日均消耗/按量余额 + 无资源包 + 免费额度灰字 + 未配 Cookie 灰字指引）；模型行带能力标注（`LongCat-2.0（上下文 1M · 输出 128K）`，1024 进制简写）；总览行加 `Token 剩余 …（已用 …%）` |
| `main.go` | env `LLM_API_CHECK_LONGCAT_COOKIE`；`accounts add --type longcat` 加 `--console-cookie`（未配打印提示）；`publicLongCatAccount` 掩码 `consoleCookie`；`accounts list` LongCat 行加 `· Cookie 已配置/未配置`；help 文本中立化 |

## 四、质量门

| 门 | 结果 |
|---|---|
| `gofmt -l .` | 空 ✅ |
| `go vet ./...` | 干净 ✅ |
| `go test ./... -race` | 7 包全绿 ✅ |
| 用例数 | 346 → **392**（+46） |
| 三缺陷端到端回归 | 临时 XDG_CONFIG_HOME + 真实二进制：① `status --no-refresh` 只配 LongCat 显示 LongCat 段 ② `accounts remove --name 龙猫` 成功 ③ `accounts rename --name 龙猫 --new-name 龙猫2` 成功 ④ `accounts list` 显示 Cookie 状态 ✅ |

## 五、已知边界与风险

1. **每日免费额度平台未公开**：控制台只暴露资源包（Token Pack）与按量余额；免费额度剩余无任何接口。
   无资源包且探活余额为 0 时详情页补灰字「每日免费额度平台未公开，不含在内」。
2. **Cookie 敏感度 = 登录态**：`passport_token_key` 是 longcat.chat 的会话 Cookie，同源写接口亦受其保护。
   泄露即等于账号登录态泄露，应立即在浏览器登出/轮换。配置文件按敏感文件对待（0600）。
3. **接口非公开**：`/api/pay/quota/metering/*` 未在公开文档出现，平台改版可能变更字段或路径。
   解析层对字段缺失显式失败（不静默当 0），改版时会在错误信息中暴露。
4. **go.mod 模块路径正确（非本期问题）**：`go.mod` 的 module 路径为
   `github.com/xieguiawu/llm-api-check`，与 git remote 一致，全仓 61 处 import 全部一致。
   本期施工中 Agent 新写的 import 行曾出现字节转置（输出 `xieguiawu` → 落地 `xiegui**a**wu`），
   构建报 `no required module provides package`；防范：写完 Go 文件后脚本校验所有
   `github.com/<org>/llm-api-check` 段与 go.mod 逐字节一致。
5. **Cookie 通道 e2e 未做实机验证**：硬约束禁止调用真实 longcat.chat，控制台通道仅由
   httptest 单测覆盖（cookie 透传/自动补名/401 哨兵/500/并发）。
6. **复审修复（2026-09-16）**：
   - `--json` 投影漏 quota/paygo（publicLongCatResult 已补）
   - 标签「控制台配额」超 8 列改为「配额」
   - 提示「每日免费额度平台未公开」依赖探活结论 → 改为依赖「无资源包 + 按量余额≤0」
   - 总览配额段嵌套在探活分支内 → 独立降级（有 lot / 有 paygo 余额 / 两者皆无 / Usage==nil 有 paygo 四组合）

## 六、实施记录

### 改了哪些文件（路径:行）

- `internal/models/models.go`：`LongCatAccount` 加 `ConsoleCookie`+`HasCookie()`；`LongCatModel` 扩三字段；新增 `LongCatLot/LongCatEstimate/LongCatQuota/LongCatPaygo/LongCatPaygoBalance/LongCatPaygoAmount`
- `internal/parsers/parsers.go`：`ErrLongCatConsoleAuth` 哨兵；`parseLongCatConsoleEnvelope`（code 用 `*int`）；`ParseLongCatTokenPacksSummary`/`ParseLongCatPaygoSummary`；`ParseLongCatModels` 提取能力字段
- `internal/repo/longcat.go`：`longcatConsoleBaseURL`；`ConsoleURL`/`consoleBaseURL()`；`ConsoleQuota`/`ConsolePaygo`/`consolePOST`/`normalizeConsoleCookie`
- `internal/app/app.go`：`LongCatResult.Quota/Paygo`；`refreshLongCat` 两路并发 + `hasCookie` 门控 + 错误合并
- `internal/render/render.go`：`writeLongCatConsoleQuota`/`tokenPackLot`/`formatTokenCount`/`longCatModelText`；`RenderLongCatDetail` 加 `now` 参数 + 配额段；`writeLongCatOverview` 加 Token 段
- `main.go`：`envLcCookie`；`--console-cookie` 中立 help；longcat add 分支加 Cookie；`publicLongCatAccount` 掩码；list Cookie 状态；usageText 更新
- 测试：`internal/parsers/longcat_test.go`（+12）、`internal/repo/longcat_test.go`（+10）、`internal/app/longcat_test.go`（+5）、`internal/render/longcat_test.go`（+7）、`main_test.go`（+3）

### 质量门命令与结果

```
$ gofmt -l .          # 空
$ go vet ./...        # 干净
$ go test ./... -race # 7 包全绿
$ grep -rh "^func Test" --include="*_test.go" . | wc -l   # 392（基线 346）
```

### 三缺陷端到端回归输出（临时 XDG_CONFIG_HOME + 真实二进制 v1.4.0）

```
$ llm-api-check accounts add --type longcat --name "龙猫" --api-key sk-lc-testkey123
已添加 LongCat 账号「龙猫」(id=47fd5d62...)
提示: 未配控制台 Cookie，余额只能探活；配额信息（Token 资源包/按量余额）需重跑并传 --console-cookie

$ llm-api-check status --no-refresh
LLM API Check — 尚未更新
LongCat (龙猫)
  暂无数据          ← 旧版此处显示「未配置任何账号」并 return（缺陷 1 已修）

$ llm-api-check accounts remove --name "龙猫"
已删除 1 个账号      ← 缺陷 2 已修

$ llm-api-check accounts rename --name "龙猫" --new-name "龙猫2"
已重命名 1 个账号    ← 缺陷 2 已修

$ llm-api-check accounts list
LongCat 账号 (1):
  906f5cda...  龙猫2  [API Key 已配置 · Cookie 未配置（仅探活）]   ← 缺陷 3 + list Cookie 状态
```

### 未决/已知风险

- 见「五、已知边界与风险」第 1-5 条。
- 控制台通道无真机 e2e（硬约束禁真实调用）。

### 提交

- `f598f16` fix: LongCat 账号三缺陷（总览空判定/remove/rename）+ README 去重
- 本期 feat 提交（见 git log）
