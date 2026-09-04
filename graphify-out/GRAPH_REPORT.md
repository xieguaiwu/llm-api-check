# Graph Report - LLM-api-check  (2026-09-04)

## Corpus Check
- 55 files · ~47,899 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 758 nodes · 2115 edges · 25 communities (21 shown, 4 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 324 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `99f7c98d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- render.go
- parsers_test.go
- main.go
- New
- QwenAccount
- HandlerFunc
- Config
- parsers/galaxy_test.go
- runCLI
- llm-api-check
- LLM API Check CLI Implementation Plan
- qwen_cli_test.go
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
- ParseBaiModels

## God Nodes (most connected - your core abstractions)
1. `New()` - 45 edges
2. `runCLI()` - 39 edges
3. `withConfigDir()` - 33 edges
4. `baseTime()` - 28 edges
5. `Config` - 25 edges
6. `RenderGalaxyDetail()` - 24 edges
7. `RenderQwenDetail()` - 24 edges
8. `NewWithRepos()` - 21 edges
9. `Colorizer` - 21 edges
10. `QwenRepo` - 21 edges

## Surprising Connections (you probably didn't know these)
- `cmdBai()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdDeepSeek()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdGalaxy()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdOpenCode()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdQwen()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go

## Import Cycles
- None detected.

## Communities (25 total, 4 thin omitted)

### Community 0 - "render.go"
Cohesion: 0.06
Nodes (92): DeepSeekResult, QwenResult, Builder, PlanDisplayName(), baiAllFlash(), T, TestRenderBaiDetailAllFlashPresent(), TestRenderBaiDetailErrorAndEmpty() (+84 more)

### Community 1 - "parsers_test.go"
Cohesion: 0.09
Nodes (57): AggregateCost(), clampPercent(), ExtractQwenSECToken(), Time, normalizeRefDate(), parseAmount(), ParseDeepSeekBalance(), ParseDeepSeekCost() (+49 more)

### Community 2 - "main.go"
Cohesion: 0.09
Nodes (73): checkPermissions(), DefaultPath(), Load(), NewID(), T, TestDefaultPathHome(), TestDefaultPathXDG(), TestLoadCorruptJSON() (+65 more)

### Community 3 - "New"
Cohesion: 0.10
Nodes (29): New(), ParseQwenModels(), T, TestBaiModelsAuthErrors(), TestBaiModelsEmptyKey(), TestBaiModelsNonJSON200(), TestBaiModelsWireFormat(), galaxyError() (+21 more)

### Community 4 - "QwenAccount"
Cohesion: 0.11
Nodes (21): Cmd, Context, NormalizeQwenRegion(), QwenRegionDisplayName(), contains(), T, TestAccountHelpers(), TestAccountJSONTags() (+13 more)

### Community 5 - "HandlerFunc"
Cohesion: 0.10
Nodes (62): HandlerFunc, galaxyOK(), galaxyTestAccount(), Server, T, md5Hex(), mustHandler(), newGalaxyRepo() (+54 more)

### Community 6 - "Config"
Cohesion: 0.06
Nodes (57): AccountResult, App, BaiResult, Repos, Result, Config, errMsg(), Time (+49 more)

### Community 7 - "parsers/galaxy_test.go"
Cohesion: 0.11
Nodes (44): AggregateGalaxyCost(), GalaxyDeadlineUnix(), galaxyMissingField(), GalaxyRFC3339(), GalaxySign(), GalaxyStatusActive(), GalaxyStatusText(), GalaxyStringToSign() (+36 more)

### Community 8 - "runCLI"
Cohesion: 0.14
Nodes (44): T, TestBaiAccountsLifecycle(), TestBaiAddFromEnv(), TestUsageTextMentionsBai(), galaxyEnv(), T, TestGalaxyAddAndListMasksSecrets(), TestGalaxyAddBadTypeExit2() (+36 more)

### Community 9 - "llm-api-check"
Cohesion: 0.09
Nodes (20): Commands, Data sources, Features, Install, License, llm-api-check, Quick start, Security (+12 more)

### Community 10 - "LLM API Check CLI Implementation Plan"
Cohesion: 0.11
Nodes (18): A. OpenCode Go usage（官方 API，API key 认证，无需 cookie）, Acceptance（验收标准）, B. OpenCode Zen billing（页面 scrape，workspaceId + auth cookie）, C. DeepSeek 余额（官方 API，API key 认证）, D. DeepSeek 消费明细（platform 页面 API，浏览器登录 token）, Global Constraints, LLM API Check CLI Implementation Plan, Task 1: 项目脚手架 + 数据模型 (+10 more)

### Community 11 - "qwen_cli_test.go"
Cohesion: 0.17
Nodes (28): DetectQwenCLI(), Server, T, helperCLI(), qwenUsageServer(), TestDetectQwenCLIDisabledByEnv(), TestDetectQwenCLIEnvBinMissing(), TestDetectQwenCLIFromEnv() (+20 more)

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
Cohesion: 0.15
Nodes (11): 历史工作记录, 待办：Android 对等实现, 技术要点（下一位 Agent 必读）, 最后一次完成的工作（2026-08-29 14:10）, 最后一次完成的工作（2026-08-29 18:50）, 最后一次完成的工作（2026-08-30 14:35）, 最后一次完成的工作（2026-08-30 15:0x）, 最后更新时间 (+3 more)

### Community 22 - "models.go"
Cohesion: 0.09
Nodes (31): GalaxyResult, goWindows(), costWindowCovered(), galaxyCodeString(), galaxyRandomNonce(), Client, RawMessage, Time (+23 more)

### Community 23 - "BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04"
Cohesion: 0.22
Nodes (8): BAI provider 设计（白B.AI · api.b.ai）—— 2026-09-04, 一、背景与需求, 七、测试与质量门, 三、数据模型（models）, 二、契约（2026-09-04 真实 key 实测）, 五、渲染, 六、CLI, 四、通道与错误

### Community 24 - "ParseBaiModels"
Cohesion: 0.43
Nodes (7): T, TestBaiPlanMissingFreeFlash(), TestParseBaiModelsEmptyData(), TestParseBaiModelsErrorEnvelope(), TestParseBaiModelsHappy(), ParseBaiModels(), truncate()

## Knowledge Gaps
- **82 isolated node(s):** `github.com/xieguiawu/llm-api-check`, `build-dist.sh script`, `最后一次完成的工作（2026-08-30 15:0x）`, `最后一次完成的工作（2026-08-30 14:35）`, `待办：Android 对等实现` (+77 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `render.go`, `parsers_test.go`, `main.go`, `QwenAccount`, `Config`, `parsers/galaxy_test.go`, `qwen_cli_test.go`, `models.go`, `ParseBaiModels`?**
  _High betweenness centrality (0.296) - this node is a cross-community bridge._
- **Why does `run()` connect `main.go` to `runCLI`?**
  _High betweenness centrality (0.089) - this node is a cross-community bridge._
- **Why does `runCLI()` connect `runCLI` to `main.go`?**
  _High betweenness centrality (0.087) - this node is a cross-community bridge._
- **Are the 36 inferred relationships involving `New()` (e.g. with `NewGalaxyRepo()` and `NewBaiRepo()`) actually correct?**
  _`New()` has 36 INFERRED edges - model-reasoned connections that need verification._
- **Are the 28 inferred relationships involving `HandlerFunc` (e.g. with `newTestServer()` and `qwenServer()`) actually correct?**
  _`HandlerFunc` has 28 INFERRED edges - model-reasoned connections that need verification._
- **Are the 15 inferred relationships involving `runCLI()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`runCLI()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `withConfigDir()` (e.g. with `TestBaiAccountsLifecycle()` and `TestBaiAddFromEnv()`) actually correct?**
  _`withConfigDir()` has 12 INFERRED edges - model-reasoned connections that need verification._