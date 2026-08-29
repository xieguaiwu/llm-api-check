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

结论：**配额只有控制台会话一条路，API Key 不行**——但控制台会话可以由官方 Bailian CLI 代持（浏览器 OAuth 一次），不必人工抓 Cookie。
因此 `consoleCookie` 设计成可选字段，缺失时优先走 CLI 通道，其次只显示套餐模型清单 + 灰字提示，与既有 Zen billing 的 `workspaceId + authCookie` 同模式。

## 一-b、Bailian CLI 配额通道（2026-08-29 实测落地，v1.2.0）

官方 Model Studio CLI（npm `bailian-cli`，bin 名 `bailian`/`bl`，≥1.16.0）提供：

```sh
bl usage token-plan --console-region cn-beijing --console-site domestic --output json
```

- 认证模式 **Console**：`bailian auth login --console` 浏览器 OAuth 一次，token 存 `~/.bailian/config.json`，之后免交互
- API key 登录（`auth login --api-key`）**不能**解锁 token-plan（实测未登录报 `No console access token found`）
- 实测（2026-08-29，cn-beijing 已登录）：返回 `{"per1WeekPercentage":0.418, "per1WeekResetTime":1788542460000}`——**5 小时窗口字段缺席**（官方「5 小时限额限时取消」），解析器单窗口独立判有，正好覆盖
- 成功 JSON 与 Cookie RPC 负载字段同名同义（分数型 used 比例 + Unix 毫秒重置时间）

**本机集成要点**：

1. 独立 prefix 安装 `npm install -g --prefix ~/.local/share/bailian-cli bailian-cli`，只用 `bailian` 名——避免与本机自研翻译 CLI `bl`（~/.local/bin/bl）同名冲突；`exec.LookPath` 也刻意查 `bailian` 不查 `bl`
2. 探测顺序：env `LLM_API_CHECK_BL_BIN`（显式指定且不可执行则报错，不静默 fallback）→ 独立安装位 → PATH 中的 `bailian`
3. `LLM_API_CHECK_QWEN_CLI=off|0|false|no` 禁用 CLI 通道（main 测试默认置 off 保持 hermetic）
4. 调用：argv 数组（非 shell）、20s 超时（exec.CommandContext）、stdout/stderr 分离（Node UNDICI 警告在 stderr，过滤后再截断 300 字符）、JSON 错误信封识别（exit 非零时信封在 stderr，实测 exit 3）
5. 通道优先级（同 CodexBar Auto）：CLI 优先、Cookie 兜底；CLI 成功不拉 subscription（无 PlanCode），Cookie 路径保留原 subscription 逻辑
6. CLI 会话过期 → `bailian auth login --console` 重新登录即可，无需重抓 Cookie

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
| `cn-beijing`（默认） | token-plan.cn-beijing.maas | bailian-cs.console.aliyun.com | BroadScopeAspnGateway | BAILIAN_ALIYUN | sfm_tokenplansolo_public_cn | 模型清单 ✅ / 配额 ✅（Bailian CLI 已登录实测 2026-08-29） |
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

- 单元测试：`go test ./... -race` 7 包全绿；fixture 为 6 个 `qwen_*.json`；CLI 通道 10 个用例（TestHelperProcess 模式，无真实 CLI 依赖），含 CLI 优先/Cookie 兜底/参数校验/噪音过滤/探测逻辑
- 真实冒烟（API Key 通道）：`llm-api-check qwen` → `模型 12 个：deepseek-v4-flash-0731, …`
- **真实冒烟（CLI 通道，2026-08-29 完成）**：`bailian auth login --console` 浏览器登录后，`llm-api-check qwen` → `7天 [████░░░░░░] 41% · 155小时32分后重置`，exit 0
- 配额通道演进史：未登录 API Key 实测回 NotLogined → CLI 未登录回 `No console access token found` → 登录后返回真实窗口

## 七、用量分析（--stats，2026-08-29 追加，v1.2.0）

`llm-api-check qwen [名称|ID] --stats` 附加 7 天用量分析：

- 数据源：`bailian usage summary --output json`（Console 认证，**一个命令同时含 period + freeTier + usage**，无需单独调 stats）
- JSON 形状（实测）：`{"period":{start,end,days}, "freeTier":[{model,type,remaining,total,remainingPercent,expires}], "usage":{modelsCalled,successfulCalls,usages:[{key,value,unit,label}]}}`
- 渲染：周期、调用模型数/成功次数、Input/Output/Total/Avg Tokens（千分位）、免费额度**只列已用过的模型**（剩余 <100%，全未用提示「免费额度未使用」）
- 免费额度剩余百分比颜色沿用已用比例语义（ColorForPercent(100-剩余)）
- 实现：`QwenCLI.runJSON` 抽取共用（Usage/Summary 同走：argv + 超时 + stderr 噪音过滤 + 信封识别）；app.RefreshQwenStats；main `--stats` flag（moveNoRefresh 扩展把 `--stats` 也提前，否则 `qwen 名称 --stats` exit 2）
- 错误文案统一「Bailian CLI 不可用: …（运行 bailian auth login --console 登录后重试）」（Usage/Stats 共用）
- 实测（2026-08-29 重新登录后）：`qwen --stats` → 2 模型 · 14 次调用 · Total 14,785 tokens · qwen3.8-flash 免费额度 98.7%
