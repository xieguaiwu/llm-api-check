# Graph Report - LLM-api-check  (2026-08-30)

## Corpus Check
- 49 files · ~43,110 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 683 nodes · 1914 edges · 23 communities (19 shown, 4 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 283 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d92a78a6`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- render.go
- parsers_test.go
- main.go
- New
- qwen_cli_test.go
- HandlerFunc
- NewWithRepos
- parsers/galaxy_test.go
- runCLI
- llm-api-check
- LLM API Check CLI Implementation Plan
- GalaxyRepo
- 二、契约（2026-08-29 真实凭据实测通过）
- F-Droid 发布调查与安卓发表计划（2026-08-24）
- Qwen Token Plan provider 设计（2026-08-29）
- CONTEXT_FOR_NEXT_AGENT.md
- isTTY
- isTTY
- build-dist.sh
- github.com/xieguiawu/llm-api-check
- models.go

## God Nodes (most connected - your core abstractions)
1. `New()` - 40 edges
2. `runCLI()` - 35 edges
3. `withConfigDir()` - 30 edges
4. `baseTime()` - 26 edges
5. `RenderGalaxyDetail()` - 24 edges
6. `Config` - 22 edges
7. `RenderQwenDetail()` - 22 edges
8. `Load()` - 19 edges
9. `Colorizer` - 19 edges
10. `newGalaxyRepo()` - 19 edges

## Surprising Connections (you probably didn't know these)
- `cmdDeepSeek()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdGalaxy()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdOpenCode()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdQwen()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdStatus()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go

## Import Cycles
- None detected.

## Communities (23 total, 4 thin omitted)

### Community 0 - "render.go"
Cohesion: 0.07
Nodes (81): GalaxyResult, Builder, PlanDisplayName(), galaxyResult(), T, TestGalaxyOverviewLowBalanceRed(), TestGalaxySpanShort(), TestGalaxyStatusColor() (+73 more)

### Community 1 - "parsers_test.go"
Cohesion: 0.09
Nodes (58): AggregateCost(), clampPercent(), ExtractQwenSECToken(), Time, normalizeRefDate(), parseAmount(), ParseDeepSeekBalance(), ParseDeepSeekCost() (+50 more)

### Community 2 - "main.go"
Cohesion: 0.07
Nodes (72): Config, checkPermissions(), DefaultPath(), Time, Load(), NewID(), T, TestDefaultPathHome() (+64 more)

### Community 3 - "New"
Cohesion: 0.14
Nodes (22): New(), galaxyError(), NewGalaxyRepo(), cookieValue(), defaultClient(), doGet(), Client, Duration (+14 more)

### Community 4 - "qwen_cli_test.go"
Cohesion: 0.08
Nodes (45): Cmd, Context, NormalizeQwenRegion(), QwenRegionDisplayName(), contains(), T, TestAccountHelpers(), TestAccountJSONTags() (+37 more)

### Community 5 - "HandlerFunc"
Cohesion: 0.10
Nodes (62): HandlerFunc, galaxyOK(), galaxyTestAccount(), Server, T, md5Hex(), mustHandler(), newGalaxyRepo() (+54 more)

### Community 6 - "NewWithRepos"
Cohesion: 0.16
Nodes (35): Repos, NewWithRepos(), Duration, Int32, Server, T, newTestServer(), qwenRepos() (+27 more)

### Community 7 - "parsers/galaxy_test.go"
Cohesion: 0.11
Nodes (44): AggregateGalaxyCost(), GalaxyDeadlineUnix(), galaxyMissingField(), GalaxyRFC3339(), GalaxySign(), GalaxyStatusActive(), GalaxyStatusText(), GalaxyStringToSign() (+36 more)

### Community 8 - "runCLI"
Cohesion: 0.17
Nodes (38): galaxyEnv(), T, TestGalaxyAddAndListMasksSecrets(), TestGalaxyAddBadTypeExit2(), TestGalaxyAddFromEnvVars(), TestGalaxyAddMissingSecretNonTTY(), TestGalaxyFlagAfterName(), TestGalaxyNoRefreshListsAccount() (+30 more)

### Community 9 - "llm-api-check"
Cohesion: 0.09
Nodes (20): Commands, Data sources, Features, Install, License, llm-api-check, Quick start, Security (+12 more)

### Community 10 - "LLM API Check CLI Implementation Plan"
Cohesion: 0.11
Nodes (18): A. OpenCode Go usage（官方 API，API key 认证，无需 cookie）, Acceptance（验收标准）, B. OpenCode Zen billing（页面 scrape，workspaceId + auth cookie）, C. DeepSeek 余额（官方 API，API key 认证）, D. DeepSeek 消费明细（platform 页面 API，浏览器登录 token）, Global Constraints, LLM API Check CLI Implementation Plan, Task 1: 项目脚手架 + 数据模型 (+10 more)

### Community 11 - "GalaxyRepo"
Cohesion: 0.23
Nodes (9): costWindowCovered(), galaxyCodeString(), galaxyRandomNonce(), Client, RawMessage, Time, GalaxyAccount, galaxyEnvelope (+1 more)

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
Cohesion: 0.17
Nodes (10): 历史工作记录, 待办：Android 对等实现, 技术要点（下一位 Agent 必读）, 最后一次完成的工作（2026-08-29 14:10）, 最后一次完成的工作（2026-08-29 18:50）, 最后一次完成的工作（2026-08-30 14:35）, 最后更新时间, 知识图谱 (+2 more)

### Community 22 - "models.go"
Cohesion: 0.10
Nodes (27): AccountResult, App, DeepSeekResult, QwenResult, Result, errMsg(), Time, joinErrors() (+19 more)

## Knowledge Gaps
- **74 isolated node(s):** `github.com/xieguiawu/llm-api-check`, `build-dist.sh script`, `最后一次完成的工作（2026-08-30 14:35）`, `待办：Android 对等实现`, `项目当前状态` (+69 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `parsers_test.go`, `main.go`, `qwen_cli_test.go`, `NewWithRepos`, `parsers/galaxy_test.go`, `GalaxyRepo`, `models.go`?**
  _High betweenness centrality (0.277) - this node is a cross-community bridge._
- **Why does `run()` connect `main.go` to `runCLI`?**
  _High betweenness centrality (0.086) - this node is a cross-community bridge._
- **Why does `runCLI()` connect `runCLI` to `main.go`?**
  _High betweenness centrality (0.084) - this node is a cross-community bridge._
- **Are the 32 inferred relationships involving `New()` (e.g. with `NewGalaxyRepo()` and `NewDeepSeekRepo()`) actually correct?**
  _`New()` has 32 INFERRED edges - model-reasoned connections that need verification._
- **Are the 24 inferred relationships involving `HandlerFunc` (e.g. with `newTestServer()` and `qwenServer()`) actually correct?**
  _`HandlerFunc` has 24 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `runCLI()` (e.g. with `TestGalaxyAddAndListMasksSecrets()` and `TestGalaxyAddBadTypeExit2()`) actually correct?**
  _`runCLI()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **Are the 10 inferred relationships involving `withConfigDir()` (e.g. with `TestGalaxyAddAndListMasksSecrets()` and `TestGalaxyAddBadTypeExit2()`) actually correct?**
  _`withConfigDir()` has 10 INFERRED edges - model-reasoned connections that need verification._