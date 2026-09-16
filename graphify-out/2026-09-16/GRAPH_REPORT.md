# Graph Report - LLM-api-check  (2026-09-07)

## Corpus Check
- 63 files · ~62,310 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 961 nodes · 2724 edges · 28 communities (24 shown, 4 thin omitted)
- Extraction: 84% EXTRACTED · 16% INFERRED · 0% AMBIGUOUS · INFERRED: 437 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d8ec97b3`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- render.go
- parsers.go
- main.go
- New
- qwen_cli_test.go
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
- isTTY
- isTTY
- build-dist.sh
- github.com/xieguiawu/llm-api-check
- models.go
- BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04
- app/bai_test.go
- RenderBaiDetail
- ParseGptzeroUsage
- GPTZero provider 设计（AI 检测额度）—— 2026-09-07

## God Nodes (most connected - your core abstractions)
1. `New()` - 59 edges
2. `runCLI()` - 47 edges
3. `withConfigDir()` - 37 edges
4. `Config` - 30 edges
5. `baseTime()` - 29 edges
6. `Colorizer` - 26 edges
7. `NewWithRepos()` - 25 edges
8. `RenderGalaxyDetail()` - 24 edges
9. `RenderQwenDetail()` - 24 edges
10. `RenderBaiDetail()` - 23 edges

## Surprising Connections (you probably didn't know these)
- `TestBaiJSONPointsAndExitCode()` --calls--> `exitCodeForResults()`  [INFERRED]
  bai_main_test.go → main.go
- `TestBaiJSONPointsAndExitCode()` --calls--> `publicBaiResult()`  [INFERRED]
  bai_main_test.go → main.go
- `cmdBai()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdDeepSeek()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdGalaxy()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go

## Import Cycles
- None detected.

## Communities (28 total, 4 thin omitted)

### Community 0 - "render.go"
Cohesion: 0.06
Nodes (104): GalaxyResult, QwenResult, Builder, GptzeroPercentUsed(), PlanDisplayName(), galaxyResult(), T, TestGalaxyOverviewLowBalanceRed() (+96 more)

### Community 1 - "parsers.go"
Cohesion: 0.05
Nodes (93): fmtInt(), T, TestBaiPlanMissingFreeFlash(), TestParseBaiErrorEnvelopeSanitized(), TestParseBaiModelsEmptyData(), TestParseBaiModelsErrorEnvelope(), TestParseBaiModelsHappy(), TestParseBaiMonthlySpent() (+85 more)

### Community 2 - "main.go"
Cohesion: 0.06
Nodes (91): AccountResult, DeepSeekResult, GptzeroResult, Result, Config, checkPermissions(), DefaultPath(), Time (+83 more)

### Community 3 - "New"
Cohesion: 0.08
Nodes (29): App, errMsg(), Time, joinErrors(), New(), baiHeaders(), cookieValue(), defaultClient() (+21 more)

### Community 4 - "qwen_cli_test.go"
Cohesion: 0.08
Nodes (49): Cmd, Context, NormalizeQwenRegion(), QwenRegionDisplayName(), contains(), T, TestAccountHelpers(), TestAccountJSONTags() (+41 more)

### Community 5 - "HandlerFunc"
Cohesion: 0.10
Nodes (61): HandlerFunc, galaxyOK(), galaxyTestAccount(), Request, Server, T, md5Hex(), mustHandler() (+53 more)

### Community 6 - "NewWithRepos"
Cohesion: 0.12
Nodes (43): Repos, NewWithRepos(), Duration, Int32, Server, T, newTestServer(), qwenRepos() (+35 more)

### Community 7 - "parsers/galaxy_test.go"
Cohesion: 0.08
Nodes (54): AggregateGalaxyCost(), GalaxyDeadlineUnix(), galaxyMissingField(), GalaxyRFC3339(), GalaxySign(), GalaxyStatusActive(), GalaxyStatusText(), GalaxyStringToSign() (+46 more)

### Community 8 - "runCLI"
Cohesion: 0.10
Nodes (57): T, TestBaiAccountsLifecycle(), TestBaiAddFromEnv(), TestBaiJSONPointsAndExitCode(), TestUsageTextMentionsBai(), galaxyEnv(), T, TestGalaxyAddAndListMasksSecrets() (+49 more)

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
Cohesion: 0.12
Nodes (15): 历史工作记录, 待办：Android 对等实现, 技术要点（下一位 Agent 必读）, 最后一次完成的工作（2026-08-29 14:10）, 最后一次完成的工作（2026-08-29 18:50）, 最后一次完成的工作（2026-08-30 14:35）, 最后一次完成的工作（2026-08-30 15:0x）, 最后一次完成的工作（2026-09-04 17:5x） (+7 more)

### Community 22 - "models.go"
Cohesion: 0.08
Nodes (36): baiRec(), T, TestAggregateBaiUsage(), TestAggregateBaiUsageTiesAndEmpty(), AggregateBaiUsage(), BaiDollar(), goWindows(), BaiModel (+28 more)

### Community 23 - "BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04"
Cohesion: 0.12
Nodes (15): BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04, 一、背景与需求, 七、测试与质量门, 三、数据模型（models）, 二-b、积分额度通道（chat.b.ai tRPC，2026-09-06 真实 key 实测）, 二、契约（2026-09-04 真实 key 实测）, 五、渲染, 八、用量分析 --stats（usage.records，2026-09-06 真实 key 实测） (+7 more)

### Community 24 - "app/bai_test.go"
Cohesion: 0.28
Nodes (19): baiStubResp, baiAllOK(), baiAllOK2(), boolJSON(), Int32, Server, T, newBaiApp() (+11 more)

### Community 25 - "RenderBaiDetail"
Cohesion: 0.28
Nodes (20): BaiResult, alignedLabelColumn(), baiAllFlash(), baiWithPoints(), T, TestRenderBaiDetailAllFlashPresent(), TestRenderBaiDetailErrorAndEmpty(), TestRenderBaiDetailMissingFlashWarns() (+12 more)

### Community 26 - "ParseGptzeroUsage"
Cohesion: 0.16
Nodes (20): T, TestParseGptzeroUsageBrokenJSON(), TestParseGptzeroUsageDropsAPIKey(), TestParseGptzeroUsageHappy(), TestParseGptzeroUsageMissingData(), TestParseGptzeroUsageMissingMonthlyWords(), TestParseGptzeroUsageMissingWordLimit(), TestParseGptzeroUsageOptionalFieldsAbsent() (+12 more)

### Community 27 - "GPTZero provider 设计（AI 检测额度）—— 2026-09-07"
Cohesion: 0.17
Nodes (11): 2.1 请求, 2.2 响应（白名单字段）, 2.3 错误语义（实测）, 2.4 额度口径, GPTZero provider 设计（AI 检测额度）—— 2026-09-07, 一、背景, 三、数据模型（models）, 二、契约（2026-09-07 真实 key 实测） (+3 more)

## Knowledge Gaps
- **100 isolated node(s):** `github.com/xieguiawu/llm-api-check`, `build-dist.sh script`, `最后一次完成的工作（2026-09-07 凌晨：provider=gptzero）`, `最后一次完成的工作（2026-09-06 下午，本日第二轮：A-F 六任务）`, `最后一次完成的工作（2026-09-06 12:5x）` (+95 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `parsers.go`, `main.go`, `qwen_cli_test.go`, `NewWithRepos`, `parsers/galaxy_test.go`, `repo/bai_test.go`, `app/bai_test.go`, `ParseGptzeroUsage`?**
  _High betweenness centrality (0.322) - this node is a cross-community bridge._
- **Why does `run()` connect `main.go` to `runCLI`?**
  _High betweenness centrality (0.083) - this node is a cross-community bridge._
- **Why does `runCLI()` connect `runCLI` to `main.go`?**
  _High betweenness centrality (0.082) - this node is a cross-community bridge._
- **Are the 48 inferred relationships involving `New()` (e.g. with `NewGalaxyRepo()` and `NewGptzeroRepo()`) actually correct?**
  _`New()` has 48 INFERRED edges - model-reasoned connections that need verification._
- **Are the 46 inferred relationships involving `HandlerFunc` (e.g. with `newTestServer()` and `qwenServer()`) actually correct?**
  _`HandlerFunc` has 46 INFERRED edges - model-reasoned connections that need verification._
- **Are the 19 inferred relationships involving `runCLI()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`runCLI()` has 19 INFERRED edges - model-reasoned connections that need verification._
- **Are the 16 inferred relationships involving `withConfigDir()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`withConfigDir()` has 16 INFERRED edges - model-reasoned connections that need verification._