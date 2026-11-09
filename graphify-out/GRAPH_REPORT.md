# Graph Report - LLM-api-check  (2026-11-08)

## Corpus Check
- 90 files · ~102,652 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1549 nodes · 4352 edges · 63 communities (56 shown, 7 thin omitted)
- Extraction: 83% EXTRACTED · 17% INFERRED · 0% AMBIGUOUS · INFERRED: 735 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `635fadef`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- RenderOverview
- parsers_test.go
- main.go
- CodebuddyRepo
- models_test.go
- HandlerFunc
- NewWithRepos
- parsers/galaxy_test.go
- runCLI
- llm-api-check
- LLM API Check CLI Implementation Plan
- RenderLongCatDetail
- 二、契约（2026-08-29 真实凭据实测通过）
- F-Droid 发布调查与安卓发表计划（2026-08-24）
- Qwen Token Plan provider 设计（2026-08-29）
- CONTEXT_FOR_NEXT_AGENT.md
- parsers/commandai_test.go
- New
- build-dist.sh
- github.com/xieguiawu/llm-api-check
- models.go
- BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04
- parsers.go
- Config
- QwenRepo
- GPTZero provider 设计（AI 检测额度）—— 2026-09-07
- cmdStatus
- qwen_cli_test.go
- parsers/longcat_test.go
- baseTime
- RenderGalaxyDetail
- Load
- LongCat 控制台配额通道实施计划（Phase 1 = Go CLI）
- app/bai_test.go
- Commandai Provider Implementation Plan
- doGet
- CommandaiBillingOf
- app/longcat_test.go
- ParseGptzeroUsage
- LongCat Provider Implementation Plan
- render.go
- AggregateBaiUsage
- RenderCommandaiDetail
- app/commandai_test.go
- CodeBuddy Provider Implementation Plan
- RenderBaiDetail
- defaultClient
- app/codebuddy_test.go
- GptzeroResult
- isTTY
- isTTY
- 2026-10-25.md
- RenderCodebuddyDetail
- CodebuddyLedger
- render/gptzero_test.go
- repo/codebuddy_test.go
- provider=codebuddy：服务端无额度接口 → 本地台账计算
- commandai / codebuddy 额度可见性修复 + 同类 provider 排查
- Result
- unixMillisOrZero
- Taste

## God Nodes (most connected - your core abstractions)
1. `New()` - 94 edges
2. `runCLI()` - 70 edges
3. `withConfigDir()` - 59 edges
4. `Config` - 40 edges
5. `Colorizer` - 36 edges
6. `RenderOverview()` - 36 edges
7. `RenderCommandaiDetail()` - 30 edges
8. `baseTime()` - 29 edges
9. `NewWithRepos()` - 28 edges
10. `RenderLongCatDetail()` - 28 edges

## Surprising Connections (you probably didn't know these)
- `TestBaiJSONPointsAndExitCode()` --calls--> `exitCodeForResults()`  [INFERRED]
  bai_main_test.go → main.go
- `TestBaiJSONPointsAndExitCode()` --calls--> `publicBaiResult()`  [INFERRED]
  bai_main_test.go → main.go
- `cmdBai()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdCodebuddy()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdCommandai()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go

## Import Cycles
- None detected.

## Communities (63 total, 7 thin omitted)

### Community 0 - "RenderOverview"
Cohesion: 0.25
Nodes (25): Builder, GptzeroPercentUsed(), codebuddyQuotaCountdown(), ColorForPercent(), firstBalanceInfo(), FormatCountdown(), Time, RenderAccountDetail() (+17 more)

### Community 1 - "parsers_test.go"
Cohesion: 0.09
Nodes (53): AggregateCost(), clampPercent(), Time, normalizeRefDate(), ParseDeepSeekBalance(), ParseDeepSeekCost(), ParseGoUsage(), ParseQwenModels() (+45 more)

### Community 2 - "main.go"
Cohesion: 0.11
Nodes (39): cmdAccountsAdd(), cmdAccountsList(), filterGalaxy(), filterLongCat(), T, main(), maskSecret(), promptTTY() (+31 more)

### Community 3 - "CodebuddyRepo"
Cohesion: 0.33
Nodes (4): Client, NewCodebuddyRepo(), CodebuddyIdentity, CodebuddyRepo

### Community 4 - "models_test.go"
Cohesion: 0.46
Nodes (7): contains(), T, TestAccountHelpers(), TestAccountJSONTags(), TestBalanceJSONTags(), TestNormalizeQwenRegion(), TestQwenAccountHelpers()

### Community 5 - "HandlerFunc"
Cohesion: 0.05
Nodes (110): HandlerFunc, baiRecordsEnvelope(), baiRepoAt(), fmtInt(), Request, Server, T, itoaB() (+102 more)

### Community 6 - "NewWithRepos"
Cohesion: 0.12
Nodes (43): Repos, NewWithRepos(), Duration, Int32, Server, T, newTestServer(), qwenRepos() (+35 more)

### Community 7 - "parsers/galaxy_test.go"
Cohesion: 0.07
Nodes (61): GalaxyResult, AggregateGalaxyCost(), GalaxyDeadlineUnix(), galaxyMissingField(), GalaxyRFC3339(), GalaxySign(), GalaxyStatusActive(), GalaxyStatusText() (+53 more)

### Community 8 - "runCLI"
Cohesion: 0.06
Nodes (92): T, TestBaiAccountsLifecycle(), TestBaiAddFromEnv(), TestBaiJSONPointsAndExitCode(), TestUsageTextMentionsBai(), codebuddyTestAccount(), T, TestCodebuddyAddAndDetail() (+84 more)

### Community 9 - "llm-api-check"
Cohesion: 0.09
Nodes (20): Commands, Data sources, Features, Install, License, llm-api-check, Quick start, Security (+12 more)

### Community 10 - "LLM API Check CLI Implementation Plan"
Cohesion: 0.11
Nodes (18): A. OpenCode Go usage（官方 API，API key 认证，无需 cookie）, Acceptance（验收标准）, B. OpenCode Zen billing（页面 scrape，workspaceId + auth cookie）, C. DeepSeek 余额（官方 API，API key 认证）, D. DeepSeek 消费明细（platform 页面 API，浏览器登录 token）, Global Constraints, LLM API Check CLI Implementation Plan, Task 1: 项目脚手架 + 数据模型 (+10 more)

### Community 11 - "RenderLongCatDetail"
Cohesion: 0.18
Nodes (26): T, TestFormatTokenCount(), TestRenderLongCatDetail(), TestRenderLongCatDetailClampConsumedRatio(), TestRenderLongCatDetailConsoleError(), TestRenderLongCatDetailEmptyBalance(), TestRenderLongCatDetailError(), TestRenderLongCatDetailExpiryFallback() (+18 more)

### Community 12 - "二、契约（2026-08-29 真实凭据实测通过）"
Cohesion: 0.14
Nodes (13): 2.1 请求, 2.2 响应, 2.3 用到的端点, 2.4 实例状态语义（文档 + 实测）, 2.5 到期时间口径, 2.6 实测快照（2026-08-29 18:17）, 一、平台与调查路径（取证记录）, 三、CLI 面（Go） (+5 more)

### Community 13 - "F-Droid 发布调查与安卓发表计划（2026-08-24）"
Cohesion: 0.14
Nodes (13): F-Droid 发布调查与安卓发表计划（2026-08-24）, Phase 1 — repo 侧准备（我可直接实施）, Phase 2 — fdroiddata 提交（需用户 GitLab 账号，我可起草文件）, Phase 3 — 发布后维护纪律, 一、现状核查（2026-08-24 实测）, 三、实施计划, 二、F-Droid 发表限制审查, 🔑 可复现构建评估（强烈推荐，非强制） (+5 more)

### Community 14 - "Qwen Token Plan provider 设计（2026-08-29）"
Cohesion: 0.18
Nodes (10): Qwen Token Plan provider 设计（2026-08-29）, 一-b.1 错误文案分档（2026-08-30 修复「额度看不到但提示没用」）, 一-b、Bailian CLI 配额通道（2026-08-29 实测落地，v1.2.0）, 一、凭据与端点矩阵（2026-08-29 本机实测）, 七、用量分析（--stats，2026-08-29 追加，v1.2.0）, 三、数值语义, 二、配额 RPC 契约, 五、CLI 面 (+2 more)

### Community 15 - "CONTEXT_FOR_NEXT_AGENT.md"
Cohesion: 0.09
Nodes (20): 历史工作记录, 待办：Android 对等实现, 技术要点（下一位 Agent 必读）, 最后一次完成的工作（2026-08-29 14:10）, 最后一次完成的工作（2026-08-29 18:50）, 最后一次完成的工作（2026-08-30 14:35）, 最后一次完成的工作（2026-08-30 15:0x）, 最后一次完成的工作（2026-09-04 17:5x） (+12 more)

### Community 16 - "parsers/commandai_test.go"
Cohesion: 0.08
Nodes (50): CommandaiResult, Time, T, TestCommandaiEnvelopeAbsent(), TestCommandaiEnvelopeMessageSanitized(), TestCommandaiTimeOfShapes(), TestErrCommandaiAuthMessage(), TestParseCommandaiCreditsAbsent() (+42 more)

### Community 17 - "New"
Cohesion: 0.22
Nodes (6): App, errMsg(), Mutex, Time, joinErrors(), New()

### Community 22 - "models.go"
Cohesion: 0.11
Nodes (26): BaiDollar(), estimateToModel(), BaiModel, BaiPlan, BaiProbe, CommandaiBilling, DeepSeekBalance, DeepSeekBalanceInfo (+18 more)

### Community 23 - "BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04"
Cohesion: 0.12
Nodes (15): BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04, 一、背景与需求, 七、测试与质量门, 三、数据模型（models）, 二-b、积分额度通道（chat.b.ai tRPC，2026-09-06 真实 key 实测）, 二、契约（2026-09-04 真实 key 实测）, 五、渲染, 八、用量分析 --stats（usage.records，2026-09-06 真实 key 实测） (+7 more)

### Community 24 - "parsers.go"
Cohesion: 0.05
Nodes (94): fmtInt(), T, TestBaiPlanMissingFreeFlash(), TestParseBaiErrorEnvelopeSanitized(), TestParseBaiModelsEmptyData(), TestParseBaiModelsErrorEnvelope(), TestParseBaiModelsHappy(), TestParseBaiMonthlySpent() (+86 more)

### Community 25 - "Config"
Cohesion: 0.06
Nodes (14): Config, Time, filterAccounts(), filterBai(), filterCodebuddy(), filterCommandai(), filterDeepSeek(), filterQwen() (+6 more)

### Community 26 - "QwenRepo"
Cohesion: 0.18
Nodes (14): cookieValue(), Duration, joinErrors(), NewQwenRepo(), normalizeCookieHeader(), QwenEndpointsFor(), qwenParamsJSON(), qwenTraceID() (+6 more)

### Community 27 - "GPTZero provider 设计（AI 检测额度）—— 2026-09-07"
Cohesion: 0.17
Nodes (11): 2.1 请求, 2.2 响应（白名单字段）, 2.3 错误语义（实测）, 2.4 额度口径, GPTZero provider 设计（AI 检测额度）—— 2026-09-07, 一、背景, 三、数据模型（models）, 二、契约（2026-09-07 真实 key 实测） (+3 more)

### Community 28 - "cmdStatus"
Cohesion: 0.41
Nodes (24): DefaultPath(), cmdAccounts(), cmdAccountsRemove(), cmdAccountsRename(), cmdBai(), cmdCodebuddy(), cmdCommandai(), cmdConfig() (+16 more)

### Community 29 - "qwen_cli_test.go"
Cohesion: 0.09
Nodes (42): Cmd, Context, NormalizeQwenRegion(), QwenRegionDisplayName(), DetectQwenCLI(), Duration, QwenCLIDefaultLoginCmd(), qwenCLIEnvelopeOf() (+34 more)

### Community 30 - "parsers/longcat_test.go"
Cohesion: 0.08
Nodes (48): T, TestErrLongCatAuth(), TestParseLongCatModelsCapabilityFields(), TestParseLongCatModelsCapabilityFieldsAbsent(), TestParseLongCatModelsDedup(), TestParseLongCatModelsEmpty(), TestParseLongCatModelsHappy(), TestParseLongCatModelsInvalidJSON() (+40 more)

### Community 31 - "baseTime"
Cohesion: 0.20
Nodes (26): baseTime(), T, Time, qwenResult(), TestColorForPercent(), TestColorizerDisabledNoANSI(), TestColorizerEnabledHasANSI(), TestCountdown() (+18 more)

### Community 32 - "RenderGalaxyDetail"
Cohesion: 0.22
Nodes (22): galaxyResult(), T, TestGalaxyOverviewLowBalanceRed(), TestGalaxySpanShort(), TestGalaxyStatusColor(), TestGalaxyUnitPriceTrims(), TestPadToUsesDisplayWidth(), TestRenderGalaxyDetailSections() (+14 more)

### Community 33 - "Load"
Cohesion: 0.26
Nodes (17): checkPermissions(), Load(), NewIDE(), T, TestDefaultPathHome(), TestDefaultPathXDG(), TestLoadCorruptJSON(), TestLoadLegacyConfigWithoutQwen() (+9 more)

### Community 34 - "LongCat 控制台配额通道实施计划（Phase 1 = Go CLI）"
Cohesion: 0.12
Nodes (16): 2.1 端点（全部 POST，`Content-Type: application/json`，body `{}`）, 2.2 真实响应（2026-09-16 实测原样，测试 fixture 的 ground truth）, 2.3 降级语义, LongCat 控制台配额通道实施计划（Phase 1 = Go CLI）, 一、背景与事实（上游实测确认，2026-09-16）, 七、独立复核结论（2026-09-16，fresh-context 只读）, 三、施工（Go CLI）, 三缺陷端到端回归输出（临时 XDG_CONFIG_HOME + 真实二进制 v1.4.0） (+8 more)

### Community 35 - "app/bai_test.go"
Cohesion: 0.28
Nodes (19): baiStubResp, baiAllOK(), baiAllOK2(), boolJSON(), Int32, Server, T, newBaiApp() (+11 more)

### Community 36 - "Commandai Provider Implementation Plan"
Cohesion: 0.11
Nodes (17): 1.1 Base 与认证, 1.2 模型面（实测免认证，HTTP 200）, 1.3 额度面（/alpha/*，CLI 私有，全部 GET + Bearer）, 1.4 数值语义（npm `projectUsageView` 公式，逐条照抄）, Commandai Provider Implementation Plan, Global Constraints, 一、线协议（三源验证，2026-10-25）, 七、main 接线（Task 5：main.go） (+9 more)

### Community 37 - "doGet"
Cohesion: 0.21
Nodes (6): baiHeaders(), doGet(), BaiPoints, QwenPlan, ZenBilling, BaiRepo

### Community 38 - "CommandaiBillingOf"
Cohesion: 0.21
Nodes (18): T, TestCommandaiAccountJSONTags(), TestCommandaiBillingOfActivePlan(), TestCommandaiBillingOfMaxPoolEdge(), TestCommandaiBillingOfNegativeClamped(), TestCommandaiBillingOfNilCredits(), TestCommandaiBillingOfNoPlan(), TestCommandaiCreditsJSONTags() (+10 more)

### Community 39 - "app/longcat_test.go"
Cohesion: 0.33
Nodes (13): Server, T, newLongCatAppTestServer(), newLongCatConsoleAppTestServer(), TestRefreshLongCatAccountNotFound(), TestRefreshLongCatBadKey(), TestRefreshLongCatConsoleAuthError(), TestRefreshLongCatConsoleHTTP500() (+5 more)

### Community 40 - "ParseGptzeroUsage"
Cohesion: 0.14
Nodes (24): T, TestParseGptzeroUsageBrokenJSON(), TestParseGptzeroUsageDropsAPIKey(), TestParseGptzeroUsageHappy(), TestParseGptzeroUsageMissingData(), TestParseGptzeroUsageMissingMonthlyWords(), TestParseGptzeroUsageMissingWordLimit(), TestParseGptzeroUsageOptionalFieldsAbsent() (+16 more)

### Community 41 - "LongCat Provider Implementation Plan"
Cohesion: 0.18
Nodes (10): 1. App Key 无配额接口；控制台 Cookie 可读资源包/余额, 2. 探活成本, 3. 免费额度, LongCat Provider Implementation Plan, 关键设计决策, 已知限制, 平台特征, 数据模型 (+2 more)

### Community 42 - "render.go"
Cohesion: 0.08
Nodes (54): LongCatResult, QwenResult, PlanDisplayName(), baiDay(), baiDollarText(), baiLabel(), clampPercent64(), codebuddyDefaultModel() (+46 more)

### Community 43 - "AggregateBaiUsage"
Cohesion: 0.36
Nodes (8): baiRec(), T, TestAggregateBaiUsage(), TestAggregateBaiUsageTiesAndEmpty(), AggregateBaiUsage(), BaiModelUsage, BaiRecord, BaiUsageStats

### Community 44 - "RenderCommandaiDetail"
Cohesion: 0.30
Nodes (18): cmFixture(), T, Time, TestRenderCommandaiCapReachedFallback(), TestRenderCommandaiCLIFallbackNote(), TestRenderCommandaiDetailHappy(), TestRenderCommandaiError(), TestRenderCommandaiExceededBadge() (+10 more)

### Community 45 - "app/commandai_test.go"
Cohesion: 0.32
Nodes (14): Server, T, newCommandaiApp(), newCommandaiAppServer(), newCommandaiKeyAwareServer(), TestRefreshAllIncludesCommandai(), TestRefreshCommandaiAccountNotFound(), TestRefreshCommandaiAuthSingleLine() (+6 more)

### Community 46 - "CodeBuddy Provider Implementation Plan"
Cohesion: 0.09
Nodes (21): 1.1 Base 与认证, 1.2 身份面（`GET /v2/accounts`）, 1.3 目录面（`GET /v3/config`）, 1.4 额度面：**服务端不存在**（穷举证据，2026-11-05）, 1.4b 额度面（2026-11-08 实测定稿）, 1.5 台账面（本地用量明细，非账户余额）, 1.6 积分与倍率的关系（实测拟合，用于交叉校验而非展示预测）, 1.7 错误语义（实测） (+13 more)

### Community 47 - "RenderBaiDetail"
Cohesion: 0.28
Nodes (20): BaiResult, alignedLabelColumn(), baiAllFlash(), baiWithPoints(), T, TestRenderBaiDetailAllFlashPresent(), TestRenderBaiDetailErrorAndEmpty(), TestRenderBaiDetailMissingFlashWarns() (+12 more)

### Community 48 - "defaultClient"
Cohesion: 0.29
Nodes (7): defaultClient(), Client, Time, NewDeepSeekRepo(), NewOpenCodeRepo(), DeepSeekRepo, OpenCodeRepo

### Community 49 - "app/codebuddy_test.go"
Cohesion: 0.35
Nodes (15): cbAppServer(), cbTestApp(), cbWriteLedger(), Server, T, Time, itoaMs(), TestCodebuddyAuthFailedEmpty() (+7 more)

### Community 50 - "GptzeroResult"
Cohesion: 0.36
Nodes (7): GptzeroResult, filterGptzero(), publicGptzeroAccount(), publicGptzeroAccounts(), publicGptzeroResult(), publicGptzeroResults(), GptzeroAccount

### Community 54 - "RenderCodebuddyDetail"
Cohesion: 0.31
Nodes (17): cbRenderFixture(), T, Time, TestCodebuddyMultiplierRange(), TestRenderCodebuddyDetailEmptyLedger(), TestRenderCodebuddyDetailHappy(), TestRenderCodebuddyDetailManyModels(), TestRenderCodebuddyDetailNilLedger() (+9 more)

### Community 55 - "CodebuddyLedger"
Cohesion: 0.15
Nodes (17): T, TestCodebuddyHasUsage(), TestCodebuddyLedgerRoot(), TestCodebuddyQuotaPercentUsed(), TestCodebuddyWindowOf(), TestParseCodebuddyMultiplier(), CodebuddyHasUsage(), CodebuddyLedgerRoot() (+9 more)

### Community 56 - "render/gptzero_test.go"
Cohesion: 0.40
Nodes (10): T, gzResult(), TestRenderGptzeroDetailBadLastResetNoCrash(), TestRenderGptzeroDetailErrorOnly(), TestRenderGptzeroDetailHappy(), TestRenderGptzeroDetailNoKey(), TestRenderGptzeroDetailOverageRed(), TestRenderGptzeroDetailUnknownLimitNoBar() (+2 more)

### Community 57 - "repo/codebuddy_test.go"
Cohesion: 0.27
Nodes (15): Server, T, Time, itoaMs(), newCbServer(), TestCodebuddyAccountsAuthHTML(), TestCodebuddyAccountsAuthJSON(), TestCodebuddyAccountsHappy() (+7 more)

### Community 58 - "provider=codebuddy：服务端无额度接口 → 本地台账计算"
Cohesion: 0.20
Nodes (9): 2026-11-05, provider=codebuddy：服务端无额度接口 → 本地台账计算, 修复：命令名大小写不敏感（`llm-api-check CodeBuddy` 报「未知命令」）, 关键发现（穷举实测，非猜测）, 协议细节（四源交叉验证）, 台账即真值源, 施工, 渲染铁律（写进代码注释 + 测试断言） (+1 more)

### Community 59 - "commandai / codebuddy 额度可见性修复 + 同类 provider 排查"
Cohesion: 0.25
Nodes (7): 2026-11-08, commandai / codebuddy 额度可见性修复 + 同类 provider 排查, 修复, 同类 provider 排查, 根因, 真实冒烟（真 key）, 质量门

### Community 60 - "Result"
Cohesion: 0.21
Nodes (11): AccountResult, CodebuddyResult, DeepSeekResult, Result, publicAccountResult(), publicAccountResults(), publicCodebuddyResult(), publicCodebuddyResults() (+3 more)

## Knowledge Gaps
- **179 isolated node(s):** `github.com/xieguiawu/llm-api-check`, `commandaiEnvelope`, `commandaiCreditsPayload`, `codebuddyErrorEnvelope`, `codebuddyQuotaAccountPayload` (+174 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **7 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `parsers_test.go`, `CodebuddyRepo`, `app/bai_test.go`, `HandlerFunc`, `NewWithRepos`, `parsers/galaxy_test.go`, `ParseGptzeroUsage`, `app/longcat_test.go`, `doGet`, `app/commandai_test.go`, `defaultClient`, `parsers/commandai_test.go`, `parsers.go`, `Config`, `QwenRepo`, `cmdStatus`, `qwen_cli_test.go`, `parsers/longcat_test.go`?**
  _High betweenness centrality (0.295) - this node is a cross-community bridge._
- **Why does `runCLI()` connect `runCLI` to `cmdStatus`?**
  _High betweenness centrality (0.072) - this node is a cross-community bridge._
- **Why does `run()` connect `cmdStatus` to `runCLI`, `main.go`?**
  _High betweenness centrality (0.066) - this node is a cross-community bridge._
- **Are the 80 inferred relationships involving `New()` (e.g. with `NewCodebuddyRepo()` and `NewCommandaiRepo()`) actually correct?**
  _`New()` has 80 INFERRED edges - model-reasoned connections that need verification._
- **Are the 71 inferred relationships involving `HandlerFunc` (e.g. with `newTestServer()` and `qwenServer()`) actually correct?**
  _`HandlerFunc` has 71 INFERRED edges - model-reasoned connections that need verification._
- **Are the 33 inferred relationships involving `runCLI()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`runCLI()` has 33 INFERRED edges - model-reasoned connections that need verification._
- **Are the 28 inferred relationships involving `withConfigDir()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`withConfigDir()` has 28 INFERRED edges - model-reasoned connections that need verification._