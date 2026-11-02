# Graph Report - llm-api-check  (2026-09-16)

## Corpus Check
- 70 files · ~71,776 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 2 file(s) not represented in the graph (top: (none) 2)

## Summary
- 1069 nodes · 3065 edges · 47 communities (42 shown, 5 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 325 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `f598f16d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Colorizer
- parsers_test.go
- main.go
- QwenRepo
- QwenCLI
- repo_test.go
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
- render.go
- App
- build-dist.sh
- github.com/xieguiawu/llm-api-check
- models.go
- BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04
- parsers.go
- RenderOverview
- ParseGptzeroUsage
- GPTZero provider 设计（AI 检测额度）—— 2026-09-07
- repo/galaxy_test.go
- qwen_cli_test.go
- parsers/longcat_test.go
- baseTime
- parsers/bai_test.go
- testing.T
- LongCat 控制台配额通道实施计划（Phase 1 = Go CLI）
- RenderGalaxyDetail
- RenderLongCatDetail
- main_galaxy_test.go
- QwenResult
- RenderGptzeroDetail
- ParseDeepSeekCost
- LongCat Provider Implementation Plan
- AccountResult
- AggregateBaiUsage
- SanitizeText
- qwenRatioToWindow
- ExtractQwenSECToken

## God Nodes (most connected - your core abstractions)
1. `runCLI()` - 52 edges
2. `withConfigDir()` - 42 edges
3. `Config` - 33 edges
4. `New()` - 29 edges
5. `Colorizer` - 29 edges
6. `baseTime()` - 29 edges
7. `App` - 27 edges
8. `NewWithRepos()` - 25 edges
9. `RenderOverview()` - 25 edges
10. `RenderQwenDetail()` - 24 edges

## Surprising Connections (you probably didn't know these)
- `runCLI()` --calls--> `run()`  [INFERRED]
  main_test.go → main.go
- `TestMoveFlags()` --calls--> `moveFlags()`  [INFERRED]
  main_galaxy_test.go → main.go
- `TestPromptTTYSharesBuffer()` --calls--> `promptTTY()`  [INFERRED]
  main_test.go → main.go
- `TestJoinTextDedupesLines()` --calls--> `joinText()`  [INFERRED]
  main_test.go → main.go
- `TestBaiStatsJSONKeyWired()` --calls--> `publicBaiResult()`  [INFERRED]
  main_test.go → main.go

## Import Cycles
- None detected.

## Communities (47 total, 5 thin omitted)

### Community 0 - "Colorizer"
Cohesion: 0.23
Nodes (21): strings.Builder, baiDollarText(), baiLabel(), ColorForPercent(), formatInt(), Colorizer, RenderAccountDetail(), renderBaiFlashLane() (+13 more)

### Community 1 - "parsers_test.go"
Cohesion: 0.14
Nodes (27): parseAmount(), ParseDeepSeekBalance(), ParseGoUsage(), ParseQwenModels(), ParseQwenUsage(), ParseZenBilling(), fixturePath(), readFixture() (+19 more)

### Community 2 - "main.go"
Cohesion: 0.05
Nodes (94): TestBaiJSONPointsAndExitCode(), bufio.Reader, io.Reader, io.Writer, New(), newLongCatAppTestServer(), newLongCatConsoleAppTestServer(), TestRefreshLongCatAccountNotFound() (+86 more)

### Community 3 - "QwenRepo"
Cohesion: 0.05
Nodes (53): baiStubResp, Repos, net/http.Client, TestRefreshQwenBadRegion(), baiAllOK(), baiAllOK2(), boolJSON(), newBaiApp() (+45 more)

### Community 4 - "QwenCLI"
Cohesion: 0.10
Nodes (23): context.Context, os/exec.Cmd, time.Duration, NormalizeQwenRegion(), QwenRegionDisplayName(), contains(), TestAccountHelpers(), TestAccountJSONTags() (+15 more)

### Community 5 - "repo_test.go"
Cohesion: 0.13
Nodes (28): costJSON(), costServer(), diffF(), formatFloat(), qwenTestEndpoints(), readFixture(), TestBalance401(), TestBalanceOK() (+20 more)

### Community 6 - "NewWithRepos"
Cohesion: 0.08
Nodes (53): net/http/httptest.Server, sync/atomic.Int32, NewWithRepos(), newTestServer(), qwenRepos(), qwenServer(), readFixture(), TestRefreshAccountErrorJoin() (+45 more)

### Community 7 - "parsers/galaxy_test.go"
Cohesion: 0.07
Nodes (49): GalaxyAccount, GalaxyBalance, GalaxyStatusCount, AggregateGalaxyCost(), GalaxyDeadlineUnix(), galaxyMissingField(), GalaxyRFC3339(), GalaxySign() (+41 more)

### Community 8 - "runCLI"
Cohesion: 0.12
Nodes (41): TestBaiAccountsLifecycle(), TestBaiAddFromEnv(), TestUsageTextMentionsBai(), TestGptzeroAccountsLifecycle(), TestGptzeroAddBadType(), TestGptzeroCommandWired(), TestGptzeroNoRefreshNoAccounts(), TestUsageTextMentionsGptzero() (+33 more)

### Community 9 - "llm-api-check"
Cohesion: 0.09
Nodes (20): Commands, Data sources, Features, Install, License, llm-api-check, Quick start, Security (+12 more)

### Community 10 - "LLM API Check CLI Implementation Plan"
Cohesion: 0.11
Nodes (18): A. OpenCode Go usage（官方 API，API key 认证，无需 cookie）, Acceptance（验收标准）, B. OpenCode Zen billing（页面 scrape，workspaceId + auth cookie）, C. DeepSeek 余额（官方 API，API key 认证）, D. DeepSeek 消费明细（platform 页面 API，浏览器登录 token）, Global Constraints, LLM API Check CLI Implementation Plan, Task 1: 项目脚手架 + 数据模型 (+10 more)

### Community 11 - "repo/bai_test.go"
Cohesion: 0.11
Nodes (28): net/http.Response, TestRefreshBaiAccountNotFound(), baiRecordsEnvelope(), baiRepoAt(), fmtInt(), itoaB(), newBaiConsole(), TestBaiModelsAuthErrors() (+20 more)

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
Cohesion: 0.11
Nodes (17): 历史工作记录, 待办：Android 对等实现, 技术要点（下一位 Agent 必读）, 最后一次完成的工作（2026-08-29 14:10）, 最后一次完成的工作（2026-08-29 18:50）, 最后一次完成的工作（2026-08-30 14:35）, 最后一次完成的工作（2026-08-30 15:0x）, 最后一次完成的工作（2026-09-04 17:5x） (+9 more)

### Community 16 - "render.go"
Cohesion: 0.13
Nodes (25): time.Time, LongCatResult, GalaxyInstance, TestGalaxySpanShort(), TestPadToUsesDisplayWidth(), baiDay(), CurrencySymbol(), displayWidth() (+17 more)

### Community 17 - "App"
Cohesion: 0.18
Nodes (9): App, sync.Mutex, errMsg(), BaiResult, DeepSeekResult, GalaxyResult, GptzeroResult, Result (+1 more)

### Community 22 - "models.go"
Cohesion: 0.10
Nodes (29): BaiDollar(), BaiModel, BaiPlan, BaiProbe, DeepSeekBalance, DeepSeekBalanceInfo, DeepSeekCost, GalaxyCost (+21 more)

### Community 23 - "BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04"
Cohesion: 0.12
Nodes (15): BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04, 一、背景与需求, 七、测试与质量门, 三、数据模型（models）, 二-b、积分额度通道（chat.b.ai tRPC，2026-09-06 真实 key 实测）, 二、契约（2026-09-04 真实 key 实测）, 五、渲染, 八、用量分析 --stats（usage.records，2026-09-06 真实 key 实测） (+7 more)

### Community 24 - "parsers.go"
Cohesion: 0.15
Nodes (25): encoding/json.RawMessage, baiEnvelopeOf(), baiPayloadOf(), estimateToModel(), gptzeroCountersOf(), int64Of(), ParseBaiMonthlySpent(), parseLongCatConsoleEnvelope() (+17 more)

### Community 25 - "RenderOverview"
Cohesion: 0.23
Nodes (20): alignedLabelColumn(), baiAllFlash(), baiWithPoints(), TestRenderBaiDetailAllFlashPresent(), TestRenderBaiDetailErrorAndEmpty(), TestRenderBaiDetailMissingFlashWarns(), TestRenderBaiDetailPoints(), TestRenderBaiDetailPointsVariants() (+12 more)

### Community 26 - "ParseGptzeroUsage"
Cohesion: 0.36
Nodes (9): TestParseGptzeroUsageBrokenJSON(), TestParseGptzeroUsageDropsAPIKey(), TestParseGptzeroUsageHappy(), TestParseGptzeroUsageMissingData(), TestParseGptzeroUsageMissingMonthlyWords(), TestParseGptzeroUsageMissingWordLimit(), TestParseGptzeroUsageOptionalFieldsAbsent(), TestParseGptzeroUsageStringNumbers() (+1 more)

### Community 27 - "GPTZero provider 设计（AI 检测额度）—— 2026-09-07"
Cohesion: 0.17
Nodes (11): 2.1 请求, 2.2 响应（白名单字段）, 2.3 错误语义（实测）, 2.4 额度口径, GPTZero provider 设计（AI 检测额度）—— 2026-09-07, 一、背景, 三、数据模型（models）, 二、契约（2026-09-07 真实 key 实测） (+3 more)

### Community 28 - "repo/galaxy_test.go"
Cohesion: 0.23
Nodes (24): net/http.HandlerFunc, net/http.Request, galaxyOK(), galaxyTestAccount(), md5Hex(), mustHandler(), newGalaxyRepo(), parseFormString() (+16 more)

### Community 29 - "qwen_cli_test.go"
Cohesion: 0.14
Nodes (24): DetectQwenCLI(), helperCLI(), qwenUsageServer(), TestDetectQwenCLIDisabledByEnv(), TestDetectQwenCLIEnvBinMissing(), TestDetectQwenCLIFromEnv(), TestQwenCLIEnvelopeOnBothStreams(), TestQwenCLIHelperProcess() (+16 more)

### Community 30 - "parsers/longcat_test.go"
Cohesion: 0.15
Nodes (22): TestErrLongCatAuth(), TestParseLongCatModelsCapabilityFields(), TestParseLongCatModelsCapabilityFieldsAbsent(), TestParseLongCatModelsDedup(), TestParseLongCatModelsEmpty(), TestParseLongCatModelsHappy(), TestParseLongCatModelsInvalidJSON(), TestParseLongCatModelsSorted() (+14 more)

### Community 31 - "baseTime"
Cohesion: 0.23
Nodes (21): FormatCountdown(), RenderQwenDetail(), baseTime(), qwenResult(), TestColorizerDisabledNoANSI(), TestColorizerEnabledHasANSI(), TestCountdown(), TestCountdownMillisecondsAccepted() (+13 more)

### Community 32 - "parsers/bai_test.go"
Cohesion: 0.16
Nodes (18): BaiPoints, fmtInt(), TestBaiPlanMissingFreeFlash(), TestParseBaiErrorEnvelopeSanitized(), TestParseBaiModelsEmptyData(), TestParseBaiModelsErrorEnvelope(), TestParseBaiModelsHappy(), TestParseBaiMonthlySpent() (+10 more)

### Community 33 - "testing.T"
Cohesion: 0.24
Nodes (17): testing.T, TestDefaultPathHome(), TestDefaultPathXDG(), TestLoadCorruptJSON(), TestLoadLegacyConfigWithoutQwen(), TestLoadMissingFileEmpty(), TestLoadWidePermissionWarns(), TestNewID() (+9 more)

### Community 34 - "LongCat 控制台配额通道实施计划（Phase 1 = Go CLI）"
Cohesion: 0.12
Nodes (15): 2.1 端点（全部 POST，`Content-Type: application/json`，body `{}`）, 2.2 真实响应（2026-09-16 实测原样，测试 fixture 的 ground truth）, 2.3 降级语义, LongCat 控制台配额通道实施计划（Phase 1 = Go CLI）, 一、背景与事实（上游实测确认，2026-09-16）, 三、施工（Go CLI）, 三缺陷端到端回归输出（临时 XDG_CONFIG_HOME + 真实二进制 v1.4.0）, 二、契约（2026-09-16 实测） (+7 more)

### Community 35 - "RenderGalaxyDetail"
Cohesion: 0.27
Nodes (15): galaxyResult(), TestGalaxyOverviewLowBalanceRed(), TestGalaxyStatusColor(), TestGalaxyUnitPriceTrims(), TestRenderGalaxyDetailSections(), TestRenderGalaxyErrorOnly(), TestRenderGalaxyExpiredKeepsTime(), TestRenderGalaxyExpiryAlwaysVisible() (+7 more)

### Community 36 - "RenderLongCatDetail"
Cohesion: 0.19
Nodes (15): TestFormatTokenCount(), TestRenderLongCatDetail(), TestRenderLongCatDetailConsoleError(), TestRenderLongCatDetailEmptyBalance(), TestRenderLongCatDetailError(), TestRenderLongCatDetailNoCookie(), TestRenderLongCatDetailNoKey(), TestRenderLongCatDetailNoPack() (+7 more)

### Community 37 - "main_galaxy_test.go"
Cohesion: 0.15
Nodes (13): galaxyEnv(), TestGalaxyAddAndListMasksSecrets(), TestGalaxyAddBadTypeExit2(), TestGalaxyAddFromEnvVars(), TestGalaxyAddMissingSecretNonTTY(), TestGalaxyFlagAfterName(), TestGalaxyNoRefreshListsAccount(), TestGalaxyRefreshMissingSecretExit1() (+5 more)

### Community 38 - "QwenResult"
Cohesion: 0.20
Nodes (11): QwenResult, QwenPlan, QwenUsage, QwenWindow, PlanDisplayName(), TestPlanDisplayName(), planSummary(), qwenLoginCmd() (+3 more)

### Community 39 - "RenderGptzeroDetail"
Cohesion: 0.32
Nodes (11): GptzeroPercentUsed(), gzResult(), TestRenderGptzeroDetailBadLastResetNoCrash(), TestRenderGptzeroDetailErrorOnly(), TestRenderGptzeroDetailHappy(), TestRenderGptzeroDetailNoKey(), TestRenderGptzeroDetailOverageRed(), TestRenderGptzeroDetailUnknownLimitNoBar() (+3 more)

### Community 40 - "ParseDeepSeekCost"
Cohesion: 0.23
Nodes (12): AggregateCost(), ParseDeepSeekCost(), diff(), refDate(), TestAggregateCost30DaysNotTruncated(), TestAggregateCostEmpty(), TestParseDeepSeekCostCode40003(), TestParseDeepSeekCostCodeNonZero() (+4 more)

### Community 41 - "LongCat Provider Implementation Plan"
Cohesion: 0.18
Nodes (10): 1. App Key 无配额接口；控制台 Cookie 可读资源包/余额, 2. 探活成本, 3. 免费额度, LongCat Provider Implementation Plan, 关键设计决策, 已知限制, 平台特征, 数据模型 (+2 more)

### Community 42 - "AccountResult"
Cohesion: 0.32
Nodes (6): AccountResult, GoUsage, GoWindow, ZenBilling, goWindows(), windowEntry

### Community 43 - "AggregateBaiUsage"
Cohesion: 0.39
Nodes (7): baiRec(), TestAggregateBaiUsage(), TestAggregateBaiUsageTiesAndEmpty(), AggregateBaiUsage(), BaiRecord, BaiUsageStats, BaiModelUsage

### Community 44 - "SanitizeText"
Cohesion: 0.33
Nodes (6): TestSanitizeText(), ParseQwenSubscription(), qwenErrorOf(), qwenFindObject(), SanitizeText(), TestParseQwenSubscription()

### Community 45 - "qwenRatioToWindow"
Cohesion: 0.33
Nodes (6): clampPercent(), qwenNumber(), qwenPercent(), qwenRatioToWindow(), qwenResetTime(), TestQwenPercent()

## Knowledge Gaps
- **122 isolated node(s):** `github.com/xieguiawu/llm-api-check`, `build-dist.sh script`, `最后一次完成的工作（2026-09-16：LongCat 控制台配额通道 + 三缺陷修复）`, `最后一次完成的工作（2026-09-13：provider=longcat）`, `最后一次完成的工作（2026-09-07 凌晨：provider=gptzero）` (+117 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 158 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Config` connect `main.go` to `App`, `QwenRepo`, `NewWithRepos`, `parsers/galaxy_test.go`?**
  _High betweenness centrality (0.040) - this node is a cross-community bridge._
- **Why does `New()` connect `main.go` to `repo/bai_test.go`, `App`, `QwenRepo`, `NewWithRepos`?**
  _High betweenness centrality (0.036) - this node is a cross-community bridge._
- **Why does `QwenAccount` connect `main.go` to `QwenRepo`, `QwenCLI`, `QwenResult`, `App`, `models.go`?**
  _High betweenness centrality (0.021) - this node is a cross-community bridge._
- **Are the 19 inferred relationships involving `runCLI()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`runCLI()` has 19 INFERRED edges - model-reasoned connections that need verification._
- **Are the 16 inferred relationships involving `withConfigDir()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`withConfigDir()` has 16 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/xieguiawu/llm-api-check`, `build-dist.sh script`, `最后一次完成的工作（2026-09-16：LongCat 控制台配额通道 + 三缺陷修复）` to the rest of the system?**
  _122 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `parsers_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.13756613756613756 - nodes in this community are weakly interconnected._