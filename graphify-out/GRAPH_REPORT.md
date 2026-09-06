# Graph Report - LLM-api-check  (2026-09-06)

## Corpus Check
- 56 files · ~57,208 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 872 nodes · 2480 edges · 30 communities (26 shown, 4 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 384 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `ca0b715a`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- render.go
- parsers.go
- main.go
- QwenRepo
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
- New
- Config
- .client
- defaultClient
- AggregateBaiUsage

## God Nodes (most connected - your core abstractions)
1. `New()` - 54 edges
2. `runCLI()` - 43 edges
3. `withConfigDir()` - 33 edges
4. `baseTime()` - 29 edges
5. `Config` - 26 edges
6. `Colorizer` - 24 edges
7. `RenderGalaxyDetail()` - 24 edges
8. `RenderQwenDetail()` - 24 edges
9. `RenderBaiDetail()` - 23 edges
10. `QwenRepo` - 21 edges

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

## Communities (30 total, 4 thin omitted)

### Community 0 - "render.go"
Cohesion: 0.06
Nodes (108): GalaxyResult, QwenResult, Builder, PlanDisplayName(), alignedLabelColumn(), baiAllFlash(), baiWithPoints(), T (+100 more)

### Community 1 - "parsers.go"
Cohesion: 0.06
Nodes (89): fmtInt(), T, TestBaiPlanMissingFreeFlash(), TestParseBaiErrorEnvelopeSanitized(), TestParseBaiModelsEmptyData(), TestParseBaiModelsErrorEnvelope(), TestParseBaiModelsHappy(), TestParseBaiMonthlySpent() (+81 more)

### Community 2 - "main.go"
Cohesion: 0.09
Nodes (76): checkPermissions(), DefaultPath(), Load(), NewIDE(), T, TestDefaultPathHome(), TestDefaultPathXDG(), TestLoadCorruptJSON() (+68 more)

### Community 3 - "QwenRepo"
Cohesion: 0.18
Nodes (12): cookieValue(), Duration, joinErrors(), normalizeCookieHeader(), QwenEndpointsFor(), qwenParamsJSON(), qwenTraceID(), TestCookieHelpers() (+4 more)

### Community 4 - "qwen_cli_test.go"
Cohesion: 0.07
Nodes (50): Cmd, Context, NormalizeQwenRegion(), QwenRegionDisplayName(), contains(), T, TestAccountHelpers(), TestAccountJSONTags() (+42 more)

### Community 5 - "HandlerFunc"
Cohesion: 0.10
Nodes (61): HandlerFunc, galaxyOK(), galaxyTestAccount(), Request, Server, T, md5Hex(), mustHandler() (+53 more)

### Community 6 - "NewWithRepos"
Cohesion: 0.16
Nodes (35): Repos, NewWithRepos(), Duration, Int32, Server, T, newTestServer(), qwenRepos() (+27 more)

### Community 7 - "parsers/galaxy_test.go"
Cohesion: 0.08
Nodes (54): AggregateGalaxyCost(), GalaxyDeadlineUnix(), galaxyMissingField(), GalaxyRFC3339(), GalaxySign(), GalaxyStatusActive(), GalaxyStatusText(), GalaxyStringToSign() (+46 more)

### Community 8 - "runCLI"
Cohesion: 0.12
Nodes (51): T, TestBaiAccountsLifecycle(), TestBaiAddFromEnv(), TestBaiJSONPointsAndExitCode(), TestUsageTextMentionsBai(), galaxyEnv(), T, TestGalaxyAddAndListMasksSecrets() (+43 more)

### Community 9 - "llm-api-check"
Cohesion: 0.09
Nodes (20): Commands, Data sources, Features, Install, License, llm-api-check, Quick start, Security (+12 more)

### Community 10 - "LLM API Check CLI Implementation Plan"
Cohesion: 0.11
Nodes (18): A. OpenCode Go usage（官方 API，API key 认证，无需 cookie）, Acceptance（验收标准）, B. OpenCode Zen billing（页面 scrape，workspaceId + auth cookie）, C. DeepSeek 余额（官方 API，API key 认证）, D. DeepSeek 消费明细（platform 页面 API，浏览器登录 token）, Global Constraints, LLM API Check CLI Implementation Plan, Task 1: 项目脚手架 + 数据模型 (+10 more)

### Community 11 - "repo/bai_test.go"
Cohesion: 0.15
Nodes (29): baiRecordsEnvelope(), baiRepoAt(), fmtInt(), Request, Server, T, itoaB(), newBaiConsole() (+21 more)

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
Cohesion: 0.13
Nodes (13): 历史工作记录, 待办：Android 对等实现, 技术要点（下一位 Agent 必读）, 最后一次完成的工作（2026-08-29 14:10）, 最后一次完成的工作（2026-08-29 18:50）, 最后一次完成的工作（2026-08-30 14:35）, 最后一次完成的工作（2026-08-30 15:0x）, 最后一次完成的工作（2026-09-04 17:5x） (+5 more)

### Community 22 - "models.go"
Cohesion: 0.10
Nodes (26): BaiDollar(), goWindows(), BaiModel, BaiModelUsage, BaiPlan, BaiPoints, BaiProbe, BaiUsageStats (+18 more)

### Community 23 - "BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04"
Cohesion: 0.12
Nodes (15): BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04, 一、背景与需求, 七、测试与质量门, 三、数据模型（models）, 二-b、积分额度通道（chat.b.ai tRPC，2026-09-06 真实 key 实测）, 二、契约（2026-09-04 真实 key 实测）, 五、渲染, 八、用量分析 --stats（usage.records，2026-09-06 真实 key 实测） (+7 more)

### Community 24 - "app/bai_test.go"
Cohesion: 0.28
Nodes (19): baiStubResp, baiAllOK(), baiAllOK2(), boolJSON(), Int32, Server, T, newBaiApp() (+11 more)

### Community 25 - "New"
Cohesion: 0.22
Nodes (11): AccountResult, App, BaiResult, DeepSeekResult, Result, errMsg(), Time, joinErrors() (+3 more)

### Community 26 - "Config"
Cohesion: 0.12
Nodes (3): Config, Time, Account

### Community 27 - ".client"
Cohesion: 0.27
Nodes (4): TestDoGetSanitizesHTTPErrorBody(), baiHeaders(), doGet(), BaiRepo

### Community 28 - "defaultClient"
Cohesion: 0.29
Nodes (8): defaultClient(), Client, Time, NewDeepSeekRepo(), NewOpenCodeRepo(), NewQwenRepo(), DeepSeekRepo, OpenCodeRepo

### Community 29 - "AggregateBaiUsage"
Cohesion: 0.52
Nodes (6): baiRec(), T, TestAggregateBaiUsage(), TestAggregateBaiUsageTiesAndEmpty(), AggregateBaiUsage(), BaiRecord

## Knowledge Gaps
- **89 isolated node(s):** `github.com/xieguiawu/llm-api-check`, `build-dist.sh script`, `最后一次完成的工作（2026-09-06 12:5x）`, `最后一次完成的工作（2026-09-04 17:5x）`, `最后一次完成的工作（2026-08-30 15:0x）` (+84 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `parsers.go`, `main.go`, `QwenRepo`, `qwen_cli_test.go`, `NewWithRepos`, `parsers/galaxy_test.go`, `repo/bai_test.go`, `app/bai_test.go`, `Config`, `.client`, `defaultClient`?**
  _High betweenness centrality (0.315) - this node is a cross-community bridge._
- **Why does `run()` connect `main.go` to `runCLI`?**
  _High betweenness centrality (0.078) - this node is a cross-community bridge._
- **Why does `runCLI()` connect `runCLI` to `main.go`?**
  _High betweenness centrality (0.077) - this node is a cross-community bridge._
- **Are the 44 inferred relationships involving `New()` (e.g. with `NewGalaxyRepo()` and `NewBaiRepo()`) actually correct?**
  _`New()` has 44 INFERRED edges - model-reasoned connections that need verification._
- **Are the 39 inferred relationships involving `HandlerFunc` (e.g. with `newTestServer()` and `qwenServer()`) actually correct?**
  _`HandlerFunc` has 39 INFERRED edges - model-reasoned connections that need verification._
- **Are the 15 inferred relationships involving `runCLI()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`runCLI()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `withConfigDir()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`withConfigDir()` has 12 INFERRED edges - model-reasoned connections that need verification._