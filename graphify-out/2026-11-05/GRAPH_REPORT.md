# Graph Report - LLM-api-check  (2026-10-25)

## Corpus Check
- 79 files · ~85,582 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1318 nodes · 3727 edges · 54 communities (49 shown, 5 thin omitted)
- Extraction: 83% EXTRACTED · 17% INFERRED · 0% AMBIGUOUS · INFERRED: 638 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d4c92cd1`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- render.go
- parsers_test.go
- main.go
- QwenRepo
- models_test.go
- HandlerFunc
- NewWithRepos
- parsers/galaxy_test.go
- runCLI
- llm-api-check
- LLM API Check CLI Implementation Plan
- repo/bai_test.go
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
- ParseGptzeroUsage
- GPTZero provider 设计（AI 检测额度）—— 2026-09-07
- cmdStatus
- qwen_cli_test.go
- parsers/longcat_test.go
- baseTime
- parsers/bai_test.go
- Load
- LongCat 控制台配额通道实施计划（Phase 1 = Go CLI）
- app/bai_test.go
- Commandai Provider Implementation Plan
- CommandaiResult
- models/commandai_test.go
- app/longcat_test.go
- NewGptzeroRepo
- LongCat Provider Implementation Plan
- doGet
- AggregateBaiUsage
- render/commandai_test.go
- newCommandaiApp
- defaultClient
- GptzeroResult
- Account
- DeepSeekAccount
- CommandaiBillingOf
- isTTY
- isTTY
- 2026-10-25.md

## God Nodes (most connected - your core abstractions)
1. `New()` - 87 edges
2. `runCLI()` - 62 edges
3. `withConfigDir()` - 51 edges
4. `Config` - 36 edges
5. `Colorizer` - 32 edges
6. `RenderOverview()` - 31 edges
7. `baseTime()` - 29 edges
8. `RenderLongCatDetail()` - 28 edges
9. `NewWithRepos()` - 26 edges
10. `RenderCommandaiDetail()` - 26 edges

## Surprising Connections (you probably didn't know these)
- `TestBaiJSONPointsAndExitCode()` --calls--> `exitCodeForResults()`  [INFERRED]
  bai_main_test.go → main.go
- `TestBaiJSONPointsAndExitCode()` --calls--> `publicBaiResult()`  [INFERRED]
  bai_main_test.go → main.go
- `cmdBai()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdCommandai()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdDeepSeek()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go

## Import Cycles
- None detected.

## Communities (54 total, 5 thin omitted)

### Community 0 - "render.go"
Cohesion: 0.05
Nodes (122): LongCatResult, Builder, GptzeroPercentUsed(), PlanDisplayName(), alignedLabelColumn(), baiAllFlash(), baiWithPoints(), T (+114 more)

### Community 1 - "parsers_test.go"
Cohesion: 0.10
Nodes (50): AggregateCost(), Time, normalizeRefDate(), ParseDeepSeekBalance(), ParseDeepSeekCost(), ParseGoUsage(), ParseQwenModels(), ParseQwenUsage() (+42 more)

### Community 2 - "main.go"
Cohesion: 0.10
Nodes (34): cmdAccountsAdd(), cmdAccountsList(), filterBai(), filterCommandai(), filterGalaxy(), filterLongCat(), T, Time (+26 more)

### Community 3 - "QwenRepo"
Cohesion: 0.15
Nodes (17): ExtractQwenSECToken(), galaxyError(), cookieValue(), Duration, joinErrors(), NewQwenRepo(), normalizeCookieHeader(), QwenEndpointsFor() (+9 more)

### Community 4 - "models_test.go"
Cohesion: 0.33
Nodes (9): TestCommandaiCreditsJSONTags(), QwenRegionDisplayName(), contains(), T, TestAccountHelpers(), TestAccountJSONTags(), TestBalanceJSONTags(), TestNormalizeQwenRegion() (+1 more)

### Community 5 - "HandlerFunc"
Cohesion: 0.08
Nodes (77): HandlerFunc, NewCommandaiRepo(), Mutex, Server, T, newCMQuotaServer(), TestCommandaiModelsDedupSort(), TestCommandaiModelsEmptyKeyAllowed() (+69 more)

### Community 6 - "NewWithRepos"
Cohesion: 0.12
Nodes (43): Repos, NewWithRepos(), Duration, Int32, Server, T, newTestServer(), qwenRepos() (+35 more)

### Community 7 - "parsers/galaxy_test.go"
Cohesion: 0.07
Nodes (60): GalaxyResult, AggregateGalaxyCost(), GalaxyDeadlineUnix(), galaxyMissingField(), GalaxyRFC3339(), GalaxySign(), GalaxyStatusActive(), GalaxyStatusText() (+52 more)

### Community 8 - "runCLI"
Cohesion: 0.08
Nodes (77): T, TestBaiAccountsLifecycle(), TestBaiAddFromEnv(), TestBaiJSONPointsAndExitCode(), TestUsageTextMentionsBai(), T, TestCommandaiAccountsListMasked(), TestCommandaiAddAndDetail() (+69 more)

### Community 9 - "llm-api-check"
Cohesion: 0.09
Nodes (20): Commands, Data sources, Features, Install, License, llm-api-check, Quick start, Security (+12 more)

### Community 10 - "LLM API Check CLI Implementation Plan"
Cohesion: 0.11
Nodes (18): A. OpenCode Go usage（官方 API，API key 认证，无需 cookie）, Acceptance（验收标准）, B. OpenCode Zen billing（页面 scrape，workspaceId + auth cookie）, C. DeepSeek 余额（官方 API，API key 认证）, D. DeepSeek 消费明细（platform 页面 API，浏览器登录 token）, Global Constraints, LLM API Check CLI Implementation Plan, Task 1: 项目脚手架 + 数据模型 (+10 more)

### Community 11 - "repo/bai_test.go"
Cohesion: 0.14
Nodes (30): baiRecordsEnvelope(), baiRepoAt(), fmtInt(), Request, Server, T, itoaB(), newBaiConsole() (+22 more)

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
Cohesion: 0.10
Nodes (18): 历史工作记录, 待办：Android 对等实现, 技术要点（下一位 Agent 必读）, 最后一次完成的工作（2026-08-29 14:10）, 最后一次完成的工作（2026-08-29 18:50）, 最后一次完成的工作（2026-08-30 14:35）, 最后一次完成的工作（2026-08-30 15:0x）, 最后一次完成的工作（2026-09-04 17:5x） (+10 more)

### Community 16 - "parsers/commandai_test.go"
Cohesion: 0.13
Nodes (36): T, TestCommandaiEnvelopeAbsent(), TestCommandaiEnvelopeMessageSanitized(), TestCommandaiTimeOfShapes(), TestErrCommandaiAuthMessage(), TestParseCommandaiCreditsAbsent(), TestParseCommandaiCreditsHappy(), TestParseCommandaiCreditsPoolsAbsent() (+28 more)

### Community 17 - "New"
Cohesion: 0.19
Nodes (13): AccountResult, App, BaiResult, DeepSeekResult, QwenResult, Result, errMsg(), Mutex (+5 more)

### Community 22 - "models.go"
Cohesion: 0.14
Nodes (21): BaiDollar(), BaiModel, BaiPlan, BaiProbe, DeepSeekBalance, DeepSeekBalanceInfo, DeepSeekCostDay, GptzeroCounters (+13 more)

### Community 23 - "BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04"
Cohesion: 0.12
Nodes (15): BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04, 一、背景与需求, 七、测试与质量门, 三、数据模型（models）, 二-b、积分额度通道（chat.b.ai tRPC，2026-09-06 真实 key 实测）, 二、契约（2026-09-04 真实 key 实测）, 五、渲染, 八、用量分析 --stats（usage.records，2026-09-06 真实 key 实测） (+7 more)

### Community 24 - "parsers.go"
Cohesion: 0.12
Nodes (31): baiEnvelopeOf(), baiPayloadOf(), clampPercent(), RawMessage, gptzeroCountersOf(), int64Of(), parseAmount(), ParseBaiMonthlySpent() (+23 more)

### Community 25 - "Config"
Cohesion: 0.09
Nodes (9): Config, Time, filterQwen(), publicQwenAccount(), publicQwenAccounts(), publicQwenResult(), publicQwenResults(), resultsFromAccounts() (+1 more)

### Community 26 - "ParseGptzeroUsage"
Cohesion: 0.44
Nodes (10): T, TestParseGptzeroUsageBrokenJSON(), TestParseGptzeroUsageDropsAPIKey(), TestParseGptzeroUsageHappy(), TestParseGptzeroUsageMissingData(), TestParseGptzeroUsageMissingMonthlyWords(), TestParseGptzeroUsageMissingWordLimit(), TestParseGptzeroUsageOptionalFieldsAbsent() (+2 more)

### Community 27 - "GPTZero provider 设计（AI 检测额度）—— 2026-09-07"
Cohesion: 0.17
Nodes (11): 2.1 请求, 2.2 响应（白名单字段）, 2.3 错误语义（实测）, 2.4 额度口径, GPTZero provider 设计（AI 检测额度）—— 2026-09-07, 一、背景, 三、数据模型（models）, 二、契约（2026-09-07 真实 key 实测） (+3 more)

### Community 28 - "cmdStatus"
Cohesion: 0.41
Nodes (23): DefaultPath(), cmdAccounts(), cmdAccountsRemove(), cmdAccountsRename(), cmdBai(), cmdCommandai(), cmdConfig(), cmdDeepSeek() (+15 more)

### Community 29 - "qwen_cli_test.go"
Cohesion: 0.09
Nodes (41): Cmd, Context, NormalizeQwenRegion(), DetectQwenCLI(), Duration, QwenCLIDefaultLoginCmd(), qwenCLIEnvelopeOf(), qwenCLIErrorEnvelope() (+33 more)

### Community 30 - "parsers/longcat_test.go"
Cohesion: 0.07
Nodes (55): T, TestErrLongCatAuth(), TestParseLongCatModelsCapabilityFields(), TestParseLongCatModelsCapabilityFieldsAbsent(), TestParseLongCatModelsDedup(), TestParseLongCatModelsEmpty(), TestParseLongCatModelsHappy(), TestParseLongCatModelsInvalidJSON() (+47 more)

### Community 31 - "baseTime"
Cohesion: 0.13
Nodes (41): galaxyResult(), T, TestGalaxyOverviewLowBalanceRed(), TestGalaxySpanShort(), TestGalaxyStatusColor(), TestGalaxyUnitPriceTrims(), TestPadToUsesDisplayWidth(), TestRenderGalaxyDetailSections() (+33 more)

### Community 32 - "parsers/bai_test.go"
Cohesion: 0.20
Nodes (20): fmtInt(), T, TestBaiPlanMissingFreeFlash(), TestParseBaiErrorEnvelopeSanitized(), TestParseBaiModelsEmptyData(), TestParseBaiModelsErrorEnvelope(), TestParseBaiModelsHappy(), TestParseBaiMonthlySpent() (+12 more)

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

### Community 37 - "CommandaiResult"
Cohesion: 0.24
Nodes (7): CommandaiResult, Client, CommandaiModel, CommandaiPlan, CommandaiUsageSummary, CommandaiWhoami, CommandaiRepo

### Community 38 - "models/commandai_test.go"
Cohesion: 0.21
Nodes (16): T, TestCommandaiAccountJSONTags(), TestCommandaiBillingOfActivePlan(), TestCommandaiBillingOfMaxPoolEdge(), TestCommandaiBillingOfNegativeClamped(), TestCommandaiBillingOfNilCredits(), TestCommandaiBillingOfNoPlan(), TestCommandaiPlanDisplayName() (+8 more)

### Community 39 - "app/longcat_test.go"
Cohesion: 0.33
Nodes (13): Server, T, newLongCatAppTestServer(), newLongCatConsoleAppTestServer(), TestRefreshLongCatAccountNotFound(), TestRefreshLongCatBadKey(), TestRefreshLongCatConsoleAuthError(), TestRefreshLongCatConsoleHTTP500() (+5 more)

### Community 40 - "NewGptzeroRepo"
Cohesion: 0.26
Nodes (10): Client, gptzeroHeaders(), NewGptzeroRepo(), T, TestGptzeroUsageAuthErrors(), TestGptzeroUsageEmptyBaseURLFallsBack(), TestGptzeroUsageEmptyKey(), TestGptzeroUsageServerError() (+2 more)

### Community 41 - "LongCat Provider Implementation Plan"
Cohesion: 0.18
Nodes (10): 1. App Key 无配额接口；控制台 Cookie 可读资源包/余额, 2. 探活成本, 3. 免费额度, LongCat Provider Implementation Plan, 关键设计决策, 已知限制, 平台特征, 数据模型 (+2 more)

### Community 42 - "doGet"
Cohesion: 0.20
Nodes (7): goWindows(), baiHeaders(), doGet(), GoUsage, GoWindow, windowEntry, BaiRepo

### Community 43 - "AggregateBaiUsage"
Cohesion: 0.36
Nodes (8): baiRec(), T, TestAggregateBaiUsage(), TestAggregateBaiUsageTiesAndEmpty(), AggregateBaiUsage(), BaiModelUsage, BaiRecord, BaiUsageStats

### Community 44 - "render/commandai_test.go"
Cohesion: 0.33
Nodes (12): cmFixture(), T, Time, TestRenderCommandaiDetailHappy(), TestRenderCommandaiError(), TestRenderCommandaiExhaustedRed(), TestRenderCommandaiLimitedBadgeAndCountdownCoexist(), TestRenderCommandaiModelsCount() (+4 more)

### Community 45 - "newCommandaiApp"
Cohesion: 0.38
Nodes (11): Server, T, newCommandaiApp(), newCommandaiAppServer(), TestRefreshAllIncludesCommandai(), TestRefreshCommandaiAccountNotFound(), TestRefreshCommandaiAuthSingleLine(), TestRefreshCommandaiHappy() (+3 more)

### Community 46 - "defaultClient"
Cohesion: 0.33
Nodes (7): defaultClient(), Client, Time, NewDeepSeekRepo(), NewOpenCodeRepo(), DeepSeekRepo, OpenCodeRepo

### Community 47 - "GptzeroResult"
Cohesion: 0.33
Nodes (7): GptzeroResult, filterGptzero(), publicGptzeroAccount(), publicGptzeroAccounts(), publicGptzeroResult(), publicGptzeroResults(), GptzeroAccount

### Community 48 - "Account"
Cohesion: 0.29
Nodes (6): filterAccounts(), publicAccount(), publicAccountResult(), publicAccountResults(), publicAccounts(), Account

### Community 49 - "DeepSeekAccount"
Cohesion: 0.29
Nodes (6): filterDeepSeek(), publicDeepSeekAccount(), publicDeepSeekAccounts(), publicDeepSeekResult(), publicDeepSeekResults(), DeepSeekAccount

### Community 50 - "CommandaiBillingOf"
Cohesion: 0.36
Nodes (8): CommandaiBillingOf(), Time, CommandaiBilling, CommandaiCredits, CommandaiSandbox, CommandaiSubscription, CommandaiWindowLimit, CommandaiWindowLimits

## Knowledge Gaps
- **141 isolated node(s):** `github.com/xieguiawu/llm-api-check`, `commandaiEnvelope`, `build-dist.sh script`, `最后一次完成的工作（2026-10-25：provider=commandai）`, `最后一次完成的工作（2026-09-16：LongCat 控制台配额通道 + 三缺陷修复）` (+136 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `parsers_test.go`, `QwenRepo`, `HandlerFunc`, `NewWithRepos`, `parsers/galaxy_test.go`, `repo/bai_test.go`, `parsers/commandai_test.go`, `parsers.go`, `Config`, `ParseGptzeroUsage`, `cmdStatus`, `qwen_cli_test.go`, `parsers/longcat_test.go`, `parsers/bai_test.go`, `app/bai_test.go`, `app/longcat_test.go`, `NewGptzeroRepo`, `doGet`, `newCommandaiApp`, `defaultClient`, `GptzeroResult`?**
  _High betweenness centrality (0.336) - this node is a cross-community bridge._
- **Why does `runCLI()` connect `runCLI` to `cmdStatus`?**
  _High betweenness centrality (0.068) - this node is a cross-community bridge._
- **Why does `run()` connect `cmdStatus` to `runCLI`, `main.go`?**
  _High betweenness centrality (0.068) - this node is a cross-community bridge._
- **Are the 74 inferred relationships involving `New()` (e.g. with `NewCommandaiRepo()` and `NewGalaxyRepo()`) actually correct?**
  _`New()` has 74 INFERRED edges - model-reasoned connections that need verification._
- **Are the 66 inferred relationships involving `HandlerFunc` (e.g. with `newTestServer()` and `qwenServer()`) actually correct?**
  _`HandlerFunc` has 66 INFERRED edges - model-reasoned connections that need verification._
- **Are the 26 inferred relationships involving `runCLI()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`runCLI()` has 26 INFERRED edges - model-reasoned connections that need verification._
- **Are the 22 inferred relationships involving `withConfigDir()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`withConfigDir()` has 22 INFERRED edges - model-reasoned connections that need verification._