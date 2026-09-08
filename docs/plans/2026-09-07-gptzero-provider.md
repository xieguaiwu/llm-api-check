# GPTZero provider 设计（AI 检测额度）—— 2026-09-07

> 新增 provider `gptzero`：查看 GPTZero（AI 检测服务，论文扫 AI 率用的那个）账号的
> 月度词数额度。单端点、只读。契约 2026-09-07 00:4x 真实 key 实测。

## 一、背景

用户在 GK 论文降 AI 率流程中反复用 GPTZero API 扫描（curl 直调）。额度按「词/月」计，
现状只能登录网页看。纳入 llm-api-check 统一盯梢：`llm-api-check gptzero`。

## 二、契约（2026-09-07 真实 key 实测）

### 2.1 请求

```text
GET https://api.gptzero.me/v2/users/me
Header: x-api-key: <key>          # 32 位 hex，app.gptzero.me → API 订阅页创建
        Accept: application/json
```

- **直连可通，无需代理**（`--noproxy '*'` 实测 200）。Go 默认 `ProxyFromEnvironment`
  无影响。历史教训（daily/2026-08-31）：Python urllib 裸 UA 会被 Cloudflare 403
  （error 1010）——Go 侧显式带浏览器 UA 头保险（实测 curl UA 直连 200）。
- 响应顶层信封 `{"data": {...}}`；`data` 内含 `api_key` 明文字段——解析必须**白名单**，
  任何层不得透传。

### 2.2 响应（白名单字段）

| 字段 | 实测值 | 语义 |
|:--|:--|:--|
| `data.plan` | `"API (300k words/month)"` | 套餐显示名 |
| `data.email` | `xieguaiwu@163.com` | 账号邮箱 |
| `data.char_limit` | `150000` | **单文档**字符上限（非月度） |
| `data.monthly_input_words` | `587` | 本周期已用词数（计费口径） |
| `data.monthly_input_chars` / `monthly_input_documents` | `3952` / `1` | 本周期字符/文档数 |
| `data.all_time_input_words` / `all_time_input_documents` | `1675389` / `861` | 历史累计 |
| `data.last_time_usage_reset` | `2026-09-06T16:28:34.943+00:00` | 本周期起点（上次重置时刻） |
| `data.full_plan.word_limit` | `300000` | 订阅内含月词数 |
| `data.full_plan.overage_word_limit` | `1000000` | 超额上限（官方 msg 口径：300k 套餐 + 0.7M 超额 = 总顶 1M；字段语义=超额部分上限，渲染按「超额上限」展示，不做加总推断） |
| `data.full_plan.duration_type` | `"monthly"` | 周期类型 |
| `data.full_plan.priceData.unit_amount` | `4500` | 套餐月费（美分）→ $45/月 |
| `data.full_plan.word_limit_msg` | `"Monthly limit of 1 million words has been reached…"` | 到顶提示模板（静态文案，非当前状态，不采信为用量） |

### 2.3 错误语义（实测）

| 场景 | 状态 | 响应体 | 处理 |
|:--|:--|:--|:--|
| 未带 key | 401 | `{"error":"Require valid cookie"}` | doGet 401/403 统一归一认证文案 |
| 坏 key | 403 | `{"error":"API key has no owner","apiKey":"<原样回显>"}` | 同上；**回显 key 不得进终端**（错误在 doGet 层被归一，原文不上抛） |
| 错误路径 | 404 | Express `Cannot GET …` HTML | `HTTP 404: …`（truncate200 消毒） |

### 2.4 额度口径

- 计费单位是**词**（word），非 token、非字符。百分比 = `monthly_input_words / full_plan.word_limit`。
- 颜色阈值沿用全局：<70 蓝 / 70-89 黄 / ≥90 红；进入超额区（used > word_limit）红标
  「已进入超额计费区」。
- 下次重置 ≈ `last_time_usage_reset + 1 个月`（估算，渲染标「≈」；平台无显式重置时间字段）。
- `char_limit=150000` 是单文档上限，展示为参考行，与月度额度无关。

## 三、数据模型（models）

```go
GptzeroAccount{ID, Name, ApiKey}
GptzeroCounters{Words, Chars, Documents}
GptzeroPlan{Name, DurationType, WordLimit, OverageWordLimit, PriceCents}
GptzeroUsage{Email, PlanName, Monthly/AllTime GptzeroCounters, CharLimit, LastReset, Plan}
GptzeroPercentUsed(used, limit int64) int   // limit<=0 → -1（未知，不画条）
```

## 四、CLI 面

```text
llm-api-check gptzero [名称|ID] [--no-refresh]
accounts add --type gptzero --name 名称 --api-key <key>   # env LLM_API_CHECK_GPTZERO_API_KEY
```

- 详情：凭据（邮箱 · 套餐名）、月额度条（已用/内含词数）、超额上限、历史累计、
  周期起点与预计重置、单文档上限。
- `--json`：账号 apiKey 掩码；usage 白名单结构不含 key（解析层已保证）。
- 退出码：任一账号无数据且有错 → 1（同全家桶口径）。

## 五、施工约束

1. 解析白名单：`data` 缺席 = 显式失败；`monthly_input_words`/`full_plan.word_limit`
   缺席 = 显式失败（不得显示 0 误报「额度没用过」）。
2. 401/403 归一 `parsers.ErrGptzeroAuth`，原文（含 key 回显）不出 doGet。
3. 请求头必须带浏览器 UA（Cloudflare 对裸客户端 UA 有 403 前科）。
4. 响应 `api_key` 字段零透传——单测断言掩码哨兵不出现。
5. 渲染 label 用 `padTo` 显示宽度对齐；数字 `formatInt` 千分位；价格 `≈ $` 前缀标注
   名义换算（套餐价是实价，不加 ≈；用 Fmt 即可）。

## 六、测试与质量门

- parsers：happy / data 缺席 / monthly_input_words 缺席 / word_limit 缺席 / 数字字符串形状 / api_key 不透传。
- repo：httptest 200 happy、403 认证文案、401 认证文案、500 原文、x-api-key 与 UA 头断言、空 key。
- app：RefreshGptzero 成功 / 仓库未初始化 / RefreshAll 并发 lane。
- render：详情各段、超额红标、limit 未知不画条、LastReset 不可解析不崩。
- main：usageText 提及 gptzero、accounts add --type gptzero e2e、--json 掩码、remove/rename 覆盖 gptzero。
- 全部 `go test ./... -race`；关键分支反向验证（拆除断言 → 对应测例 FAIL）。
