# Qwen Token Plan provider 设计（2026-08-29）

> 目标：给 llm-api-check（Go CLI）与 pocket-llm-api-checker（Android）加第三个
> provider：阿里云百炼 Qwen Token Plan 订阅。本文记录实测契约与取舍，供后续
> 排障与 Android 对等实现引用。

## 一、凭据与端点矩阵（2026-08-29 本机实测）

| 凭据 | 端点 | 结果 |
|:---|:---|:---|
| `sk-sp-`（Token Plan 订阅，北京） | `GET https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/models` | 200，返回 12 个模型 |
| 同一把 `sk-sp-` | `.../token-plan.ap-southeast-1.../models` | 401 `invalid_api_key`（**密钥与区域绑定**） |
| 同一把 `sk-sp-` | `POST https://bailian-cs.console.aliyun.com/data/api.json`（配额 RPC） | HTTP 200 + 信封 `errorCode=BailianGateway.Login.NotLogined` |
| `sk-ws-`（按量 PAYG） | `GET https://dashscope.aliyuncs.com/compatible-mode/v1/models` | 200（本 provider 不使用按量通道） |

结论：**配额只有控制台会话一条路，API Key 不行**。因此 `consoleCookie` 设计成可选字段，
缺失时只显示套餐模型清单 + 灰字提示，与既有 Zen billing 的 `workspaceId + authCookie`
同模式。

## 二、配额 RPC 契约

```text
POST https://bailian-cs.console.aliyun.com/data/api.json
    ?action=BailianGateway 系 action 名 &product=sfm_bailian&api=<api>&_v=undefined
Content-Type: application/x-www-form-urlencoded
Cookie: <用户粘贴的控制台 Cookie>
X-Requested-With: XMLHttpRequest
Origin / Referer: <控制台页面>
x-xsrf-token / x-csrf-token: <Cookie 里的 login_aliyunid_csrf>

body: product=sfm_bailian & action=BroadScopeAspnGateway & region=cn-beijing
      & language=zh-CN & params=<JSON> & sec_token=<可选>
```

`params` = `{"Api":"<api>","V":"1.0","Data":{"cornerstoneParam":{...}}}`，
`<api>` 取 `zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/usage`（窗口）或
`.../v2/subscription`（档位；Data 里另带 `commodityCode`）。

两个必须遵守的细节：

1. **不得硬编码 `switchAgent`**。上游实测：网关把它绑到某个具体账号的工作区，
   其他账号全部回 `BailianGateway.Workspace.NotAuthorised`。省略它，网关会解析
   会话默认工作区。
2. **抓 `sec_token` 必须伪装成浏览器导航请求**。OneConsole shell 只对带
   `Sec-Fetch-Site: same-origin` / `Sec-Fetch-Mode: navigate` /
   `Sec-Fetch-Dest: document` 的同源文档导航服务端渲染 `SEC_TOKEN`，
   裸请求拿不到；中国大陆 Personal/Solo 网关缺它会被拒。
   三级降级：Cookie 内 `sec_token=` → 页面 HTML 抓 `SEC_TOKEN: "…"` →
   `/tool/user/info.json`（仅国际站有）。

响应负载（`per*Percentage` 为 0-1 比例，`per*ResetTime` 为 Unix 毫秒）：

```json
{"per5HourPercentage":0.7913113,"per5HourResetTime":1786716480000,
 "per1WeekPercentage":0.451,"per1WeekResetTime":1786975680000}
```

负载位置不固定，且有的层以「JSON 字符串」内嵌。解析器因此用 BFS：展开形如
JSON 的字符串值（最大深度 12），取第一个含目标键的对象。判错信封：
`data.success=false` 或 `data.errorCode` 非空。登录类错误（`NotLogined`）
映射为「控制台 Cookie 已过期或无效」；工作区类错误保留原 `errorCode`，
**不得**误报成 Cookie 问题（会把用户引向错误的修复动作）。

网关偶发返回「200 Success 但无窗口字段」→ 重试 3 次、间隔 400 ms；
认证类错误不重试。

## 三、数值语义

`qwenPercent(ratio)`：

- `ratio ≤ 2`：比例域。`percent = min(100, floor(ratio×100))`，`exhausted = ratio ≥ 1`。
  超额（1.0x）也判 100% + 已限流。
- `ratio > 2`：视为已是百分数尺度（防御：避免 7913% 与误判限流）。
  `percent = min(100, floor(ratio))`，`exhausted = ratio ≥ 100`。

`exhausted` 是推导值，依据官方规则「窗口内配额用尽则暂停服务」。渲染时按
index.md §六 的硬性要求：**「已限流」徽章与重置倒计时并存，禁止互相替代**
（该 bug 在 2026-08-18 的 OpenCode 侧已修过一次，见 commit c694d24）。

## 四、区域支持范围

| 区域 | 网关 | 配额主机 | action | consoleSite | commodityCode | 实跑验证 |
|:---|:---|:---|:---|:---|:---|:---|
| `cn-beijing`（默认） | token-plan.cn-beijing.maas | bailian-cs.console.aliyun.com | BroadScopeAspnGateway | BAILIAN_ALIYUN | sfm_tokenplansolo_public_cn | 模型清单 ✅ / 配额 ⚠️ 需用户提供 Cookie |
| `ap-southeast-1` | token-plan.ap-southeast-1.maas | cs-data.qwencloud.com | IntlBroadScopeAspnGateway | QWENCLOUD | sfm_tokenplansolo_public_intl | ❌ 本机无国际凭据，未实跑 |

国际路径代码按公开契约实现（端点/常量集中于 `QwenEndpointsFor`），但**未做真实
冒烟**——如后续启用，先验证 `/tool/user/info.json` 的 sec_token 字段名。

## 五、CLI 面

```text
llm-api-check qwen [名称|ID] [--no-refresh]
llm-api-check accounts add --type qwen --name X --api-key sk-sp-... \
    [--console-cookie '...'] [--region cn-beijing]
```

- 凭据回退顺序沿用既有约定：flag → 环境变量 → TTY 提示。新增
  `LLM_API_CHECK_QWEN_API_KEY` / `LLM_API_CHECK_QWEN_COOKIE` /
  `LLM_API_CHECK_QWEN_REGION`。
- `--json` 全部输出掩码 `apiKey` 与 `consoleCookie`（首尾 4 + `****`）。
- 旧 `config.json` 无 `qwen_accounts` 字段时正常加载（向后兼容）。
- 顺带修一个既有 CLI 缺陷：Go flag 包遇到首个非 flag 参数即停止解析，导致
  `qwen 名称 --no-refresh` 报「多余参数」。`moveNoRefresh` 把该 flag 提前，
  deepseek/opencode/qwen 三个详情命令共用。

## 六、验证记录

- 单元测试：`go test ./... -race` 7 包全绿；fixture 为 6 个 `qwen_*.json`。
- 真实冒烟（API Key 通道）：`llm-api-check qwen` → `模型 12 个：deepseek-v4-flash-0731, …`。
- 配额通道：契约经未登录请求实测确认（回 NotLogined），有 Cookie 的端到端跑通
  **待用户提供控制台 Cookie**。
