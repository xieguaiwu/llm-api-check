# Graph Report - LLM-api-check  (2026-08-29)

## Corpus Check
- 37 files · ~29,511 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 488 nodes · 1296 edges · 22 communities (18 shown, 4 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 150 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `a2c59683`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- render.go
- parsers_test.go
- main.go
- New
- models.go
- repo_test.go
- app_test.go
- qwen_cli_test.go
- runCLI
- llm-api-check
- LLM API Check CLI Implementation Plan
- Load
- App
- F-Droid 发布调查与安卓发表计划（2026-08-24）
- Qwen Token Plan provider 设计（2026-08-29）
- CONTEXT_FOR_NEXT_AGENT.md
- isTTY
- isTTY
- build-dist.sh
- github.com/xieguiawu/llm-api-check

## God Nodes (most connected - your core abstractions)
1. `New()` - 32 edges
2. `RenderQwenDetail()` - 22 edges
3. `runCLI()` - 22 edges
4. `QwenRepo` - 19 edges
5. `Config` - 18 edges
6. `Load()` - 18 edges
7. `cmdStatus()` - 18 edges
8. `withConfigDir()` - 18 edges
9. `Colorizer` - 17 edges
10. `cmdQwen()` - 17 edges

## Surprising Connections (you probably didn't know these)
- `cmdDeepSeek()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdOpenCode()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdQwen()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdStatus()` --calls--> `New()`  [INFERRED]
  main.go → internal/app/app.go
- `cmdAccountsAdd()` --calls--> `Load()`  [INFERRED]
  main.go → internal/config/config.go

## Import Cycles
- None detected.

## Communities (22 total, 4 thin omitted)

### Community 0 - "render.go"
Cohesion: 0.10
Nodes (56): DeepSeekResult, QwenResult, Builder, PlanDisplayName(), ColorForPercent(), CurrencySymbol(), firstBalanceInfo(), Fmt() (+48 more)

### Community 1 - "parsers_test.go"
Cohesion: 0.09
Nodes (60): AggregateCost(), clampPercent(), ExtractQwenSECToken(), Time, normalizeRefDate(), parseAmount(), ParseDeepSeekBalance(), ParseDeepSeekCost() (+52 more)

### Community 2 - "main.go"
Cohesion: 0.10
Nodes (48): Config, DefaultPath(), Time, cmdAccounts(), cmdAccountsAdd(), cmdAccountsList(), cmdAccountsRemove(), cmdAccountsRename() (+40 more)

### Community 3 - "New"
Cohesion: 0.14
Nodes (22): Client, New(), ParseQwenModels(), cookieValue(), defaultClient(), doGet(), Duration, Time (+14 more)

### Community 4 - "models.go"
Cohesion: 0.09
Nodes (29): Cmd, Context, NormalizeQwenRegion(), QwenRegionDisplayName(), contains(), T, TestAccountHelpers(), TestAccountJSONTags() (+21 more)

### Community 5 - "repo_test.go"
Cohesion: 0.14
Nodes (34): costJSON(), costServer(), diffF(), formatFloat(), Duration, Server, T, qwenTestEndpoints() (+26 more)

### Community 6 - "app_test.go"
Cohesion: 0.27
Nodes (22): Repos, Int32, NewWithRepos(), Duration, Server, T, newTestServer(), qwenRepos() (+14 more)

### Community 7 - "qwen_cli_test.go"
Cohesion: 0.22
Nodes (22): DetectQwenCLI(), Server, T, helperCLI(), qwenUsageServer(), TestDetectQwenCLIDisabledByEnv(), TestDetectQwenCLIEnvBinMissing(), TestDetectQwenCLIFromEnv() (+14 more)

### Community 8 - "runCLI"
Cohesion: 0.30
Nodes (22): T, runCLI(), TestAccountsAddAndListJSONMasksSecrets(), TestAccountsAddMissingCredentialNonTTY(), TestAccountsAddOptionalFieldEOFSkip(), TestDetailFlagAfterName(), TestNoColorFlagNoANSI(), TestQwenAddAndListMasksSecrets() (+14 more)

### Community 9 - "llm-api-check"
Cohesion: 0.10
Nodes (18): Commands, Data sources, Features, Install, License, llm-api-check, Quick start, Security (+10 more)

### Community 10 - "LLM API Check CLI Implementation Plan"
Cohesion: 0.11
Nodes (18): A. OpenCode Go usage（官方 API，API key 认证，无需 cookie）, Acceptance（验收标准）, B. OpenCode Zen billing（页面 scrape，workspaceId + auth cookie）, C. DeepSeek 余额（官方 API，API key 认证）, D. DeepSeek 消费明细（platform 页面 API，浏览器登录 token）, Global Constraints, LLM API Check CLI Implementation Plan, Task 1: 项目脚手架 + 数据模型 (+10 more)

### Community 11 - "Load"
Cohesion: 0.26
Nodes (17): checkPermissions(), Load(), NewID(), T, TestDefaultPathHome(), TestDefaultPathXDG(), TestLoadCorruptJSON(), TestLoadLegacyConfigWithoutQwen() (+9 more)

### Community 12 - "App"
Cohesion: 0.26
Nodes (8): AccountResult, App, Result, errMsg(), Time, joinErrors(), ZenBilling, Mutex

### Community 13 - "F-Droid 发布调查与安卓发表计划（2026-08-24）"
Cohesion: 0.14
Nodes (13): F-Droid 发布调查与安卓发表计划（2026-08-24）, Phase 1 — repo 侧准备（我可直接实施）, Phase 2 — fdroiddata 提交（需用户 GitLab 账号，我可起草文件）, Phase 3 — 发布后维护纪律, 一、现状核查（2026-08-24 实测）, 三、实施计划, 二、F-Droid 发表限制审查, 🔑 可复现构建评估（强烈推荐，非强制） (+5 more)

### Community 14 - "Qwen Token Plan provider 设计（2026-08-29）"
Cohesion: 0.20
Nodes (9): Qwen Token Plan provider 设计（2026-08-29）, 一-b、Bailian CLI 配额通道（2026-08-29 实测落地，v1.2.0）, 一、凭据与端点矩阵（2026-08-29 本机实测）, 七、用量分析（--stats，2026-08-29 追加，v1.2.0）, 三、数值语义, 二、配额 RPC 契约, 五、CLI 面, 六、验证记录 (+1 more)

### Community 15 - "CONTEXT_FOR_NEXT_AGENT.md"
Cohesion: 0.22
Nodes (7): 历史工作记录, 技术要点（下一位 Agent 必读）, 最后一次完成的工作（2026-08-29 14:10）, 最后更新时间, 知识图谱, 遗留问题 / 待办, 项目当前状态

## Knowledge Gaps
- **59 isolated node(s):** `github.com/xieguiawu/llm-api-check`, `build-dist.sh script`, `项目当前状态`, `最后一次完成的工作（2026-08-29 14:10）`, `遗留问题 / 待办` (+54 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `New()` connect `New` to `render.go`, `parsers_test.go`, `main.go`, `models.go`, `app_test.go`, `qwen_cli_test.go`, `App`?**
  _High betweenness centrality (0.276) - this node is a cross-community bridge._
- **Why does `run()` connect `main.go` to `runCLI`?**
  _High betweenness centrality (0.076) - this node is a cross-community bridge._
- **Why does `runCLI()` connect `runCLI` to `main.go`?**
  _High betweenness centrality (0.071) - this node is a cross-community bridge._
- **Are the 25 inferred relationships involving `New()` (e.g. with `NewDeepSeekRepo()` and `NewOpenCodeRepo()`) actually correct?**
  _`New()` has 25 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `RenderQwenDetail()` (e.g. with `PlanDisplayName()` and `TestRenderQwenColorWhenExhausted()`) actually correct?**
  _`RenderQwenDetail()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/xieguiawu/llm-api-check`, `build-dist.sh script`, `项目当前状态` to the rest of the system?**
  _59 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `render.go` be split into smaller, more focused modules?**
  _Cohesion score 0.0979020979020979 - nodes in this community are weakly interconnected._