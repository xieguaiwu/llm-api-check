# LongCat Provider Implementation Plan

> 2026-09-13 — provider=longcat（美团龙猫）

## 平台特征

LongCat（龙猫）是美团的大模型服务，提供 OpenAI/Anthropic 兼容的 API 格式。

- **API 端点**：
  - OpenAI 兼容：`https://api.longcat.chat/openai`
  - Anthropic 兼容：`https://api.longcat.chat/anthropic`
- **认证**：`Authorization: Bearer YOUR_APP_KEY`
- **模型**：LongCat-2.0（1M 上下文，128K 输出），LongCat-Flash-Chat
- **计费**：按 token 计费（预付费余额 + Token Pack）
  - 输入 $0.75/MTok（折扣 $0.30），输出 $2.95/MTok（折扣 $1.20）
  - 仅成功请求（HTTP 200）计费，失败不收费

## 关键设计决策

### 1. App Key 无配额接口；控制台 Cookie 可读资源包/余额

LongCat 不提供公开的余额/配额查询 API。App Key 通道（api.longcat.chat）唯一间接方式是：
- **模型清单**：`GET /openai/v1/models` → 验证 key 是否有效（401=无效）
- **余额探活**：`POST /openai/v1/chat/completions`（max_tokens=1）
  - 200 → 余额充足
  - 402 → 余额不足
  - 401 → key 无效

控制台通道（longcat.chat，Cookie `passport_token_key`）可读精确额度（2026-09-16 新增，
详见 docs/plans/2026-09-16-longcat-console-quota.md）：
- **Token 资源包**：`POST /api/pay/quota/metering/token-packs/summary`
- **按量余额**：`POST /api/pay/quota/metering/api-usage/summary`
- 每日免费额度平台未公开，不在展示范围内

### 2. 探活成本

LongCat 按成功请求计费。max_tokens=1 的请求约消耗 1-2 个 output token，
折扣价约 $0.000002（几乎为零）。每日刷新一次约 $0.00006/月。

### 3. 免费额度

每日北京时间 0 点重置（不累计）。免费额度不保证，平台可能随时调整。

## 错误映射

| HTTP | error.code | 含义 | 处理 |
|---|---|---|---|
| 200 | - | 成功 | 余额充足 |
| 401 | invalid_api_key | key 无效/缺失 | → ErrLongCatAuth |
| 402 | insufficient_quota | 余额不足 | → BalanceOK=false |
| 403 | permission_error | 权限不足 | → ErrLongCatAuth |
| 429 | rate_limit_exceeded | 速率限制 | → 错误但不致命 |
| 500 | internal_error | 服务器错误 | → 错误但不致命 |

## 数据模型

```go
type LongCatAccount struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    ApiKey string `json:"apiKey"`
}

type LongCatUsage struct {
    Models    []LongCatModel `json:"models,omitempty"`
    BalanceOK *bool          `json:"balance_ok,omitempty"` // nil = 未探活
}
```

## 验收标准

- [x] `llm-api-check accounts add --type longcat --name X --api-key KEY`
- [x] `llm-api-check accounts add --type longcat --name X --api-key KEY --console-cookie 'passport_token_key=…'`（控制台配额通道，2026-09-16 新增）
- [x] `llm-api-check longcat` 显示余额状态 + 模型清单（+ 控制台配额段，有 Cookie 时）
- [x] `llm-api-check longcat --no-refresh` 只读配置
- [x] `llm-api-check longcat --json` 输出 JSON（quota/paygo 对象 + 凭据掩码）
- [x] `llm-api-check status` 总览包含 LongCat（+ Token 剩余段，有 Cookie 时）
- [x] `llm-api-check accounts list` 列出 LongCat 账号（+ Cookie 状态后缀）
- [x] 无效 key → exit 1
- [x] 未知账号 → exit 1
- [x] --json 掩码 API Key / consoleCookie
- [x] 控制台 Cookie 失效 → ErrLongCatConsoleAuth 文案，不污染探活结论
- [x] 全量测试通过（346 → 383）
- [x] race 检测通过
- [ ] 真机冒烟（待用户 cpu1/cpu2 实测；控制台通道受硬约束限制不做真实调用）

## 已知限制

1. 余额信息是"快照"（探活时点的状态），不是实时精确值
2. 免费额度每日重置，探活结果只反映当日余额；**控制台不公开每日免费额度剩余**（2026-09-16 实测确认）
3. 探活请求会产生微量费用（约 $0.000002/次）
4. 控制台配额接口（`/api/pay/quota/metering/*`）非公开，平台改版可能变更字段/路径；解析层对字段缺失显式失败（不静默当 0）
5. 控制台 Cookie = 平台登录态，泄露即账号失守；配置文件按敏感文件对待
6. go.mod 模块路径拼写（`xiegui**a**wu`）与 git remote（`xiegu**ai**wu`）不一致——既有技术债，Go 1.25 对主模块近似拼写有容错，全仓统一用 go.mod 拼写即可（详见 docs/plans/2026-09-16-longcat-console-quota.md §五.4）
