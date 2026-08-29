# 智星云 AI Galaxy（gpu.ai-galaxy.cn）provider 设计（2026-08-29）

> 目标：给 llm-api-check（Go CLI）与 pocket-llm-api-checker（Android）加第四个
> provider——GPU 算力云「智星云 AI Galaxy」，查看**账户余额**与**云主机实例状态**。
> 本文记录调查取证、实测契约与取舍，是两侧实现的唯一契约源。

## 一、平台与调查路径（取证记录）

| 事实 | 取证方式 |
|:---|:---|
| 平台 = 智星云 AI Galaxy（上海亘聪信息科技有限公司），GPU 云服务器租用 | 控制台页面 `<title>` + JSON-LD `schema.org/Organization` |
| 控制台 = Vue3 + Vite SPA，接口前缀 `/api`，请求体 `application/x-www-form-urlencoded` | 静态分析 `/assets/index-*.js`：`baseUrlConfig={production:"/api"}`、`axios.create({...})` |
| 控制台会话凭证 = `localStorage.token`，作为**表单字段 `session`** 由请求拦截器注入（不是 Cookie、不是 Header） | 同上：`e.data&&!e.data.session&&(e.data.session=localStorage.getItem("token")??"")` |
| **官方开放接口 OpenAPI v2**（AccessKey + SecretKey + MD5 签名）存在且有正式文档 | 控制台路由 `/console/openapi/accessKey` + 文档站 `/docs_v2` + Apifox 共享文档「智星云开放接口」 |
| 创建 AccessKey 前置条件：完成实名认证 | 文档「开始使用」+ accessKey 页面文案「尚未完成实名认证，请认证后使用」 |

选路结论：**走官方 OpenAPI v2**，不碰控制台会话。理由——① 官方契约稳定、有签名与文档；
② 控制台 `session` 会随登录过期且无刷新端点（同 Qwen Cookie 通道那样脆弱）；
③ OpenAPI 一次创建长期可用，与本项目「API key 优先、Cookie 兜底」的既有取向一致。
本 provider **没有**兜底通道，因此也不需要 Cookie 字段。

Apifox 共享文档机器可读入口（排障时复用）：
`https://apifox.com/apidoc/shared-b0fc397f-c455-4c9a-9d82-875fc48ae106/llms.txt`
→ 每篇正文 = `https://s.apifox.cn/b0fc397f-…/<doc-id>.md`（含 OpenAPI yaml）。

## 二、契约（2026-08-29 真实凭据实测通过）

### 2.1 请求

- 前缀 `https://app.ai-galaxy.cn/openapi/v2`，**统一 POST**，`Content-Type: application/x-www-form-urlencoded`
- 公共参数：`apikey`（AccessKey）、`timestamp`（Unix 秒，整数）、`nonce`（≥8 位随机串，一定时间内不可重复）
- 签名：
  1. 取**非空**业务参数 + apikey/timestamp/nonce，按参数名字典序排序，拼 `k=v` 用 `&` 连接（`sign`、`secret` 不参与；值为空的参数不参与）
  2. 末尾直接拼 `&secret={SecretKey}`（`secret` 不参与排序）
  3. `sign = lowercase(hex(md5(stringSignTemp)))`
  4. `sign` 随其余参数一起放进 POST body
- 官方给过一份可直接运行的 Golang 参考实现（文档「参考代码（Golang版）」），本实现按同一逻辑重写为无第三方依赖版本（去掉 gjson / 拼接式 client）

### 2.2 响应

```json
{ "success": true, "code": "2000", "message": "", "data": ... }
```

- `code` 是**字符串**（"2000" 成功、"4000" 客户端错误）；HTTP 状态码恒为 200，**错误在信封里**（与 Qwen 控制台同坑，不能只看状态码）
- 分页 `data`：`{list, current_page, page_count, has_more, total_count}`
- 实测错误消息：`accesskey不存在!` / `sign验证失败!` / `nonce参数缺失!` / `page_size参数超限!`

### 2.3 用到的端点

| 端点 | 参数 | 用途 / 关键字段 |
|:---|:---|:---|
| `POST /account/get_main_account_info` | — | `Money`（余额 元）、`PowerMoney`（算力券）、`CreditMoneyQuota`（信用额度）、`VipLevel`、`CustomDiscount`、`Name`、`Phone`、`Last_login_time`、`Create_time` |
| `POST /instance/get_instance_status_count` | — | `statusAll/statusRunning/statusKeeppedDisk/statusCreateError/statusRunningError/statusDefault` |
| `POST /instance/get_instance_list` | `page`、`page_size`（**≤100**）、`status_type` | 实例数组（见 2.4） |
| `POST /billing/get_balance_change_list` | `page`、`page_size`（≤100） | `CreateTime`（"2006-01-02 15:04:05"）、`DiffMoney`、`DiffPower`、`MoneyLeft`、`Remark`、`DataFrom` |
| `POST /billing/get_recharge_order_list` | `page`、`page_size` | `Amount`、`OrderTime`、`Status`（最近充值） |

刻意**不调** `account/get_apikey_info`——它会回吐 `SecretKey`，读一次就把长期凭据放进日志/内存，无收益。

### 2.4 实例状态语义（文档 + 实测）

```
Status: -2 启动错误已退费 | -1 启动错误 | 0 已结束 | 1 运行中
        4 启动中 | 5 重启中 | 7 重启失败 | 8 磁盘保留中
```

分组（`status_type`）：`statusDefault`=1,4,5,-1,7,8 · `statusRunning`=1,4,5 ·
`statusKeeppedDisk`=8 · `statusCreateError`=-1,-2,7 · `statusRunningError`=1,4,5 且 `IsAbnormal!=0` · `statusAll`=不过滤

**实测偏差记录**：统计端点回 `statusDefault: 9`，而 `status_type=statusDefault` 的列表只回 4 条（本机 4 台运行中）。
两侧都用**列表**得到的实例集合渲染，统计行只取 `statusAll/statusRunning/statusKeeppedDisk/statusCreateError`
这四个实测自洽的字段，**不显示 `statusDefault`**，避免同屏两个数字互相矛盾。
另：`status_type` 传非法值不报错、按未过滤处理（实测 `statusNope` 正常返回列表），所以不能拿它做参数校验。

实例里要用的字段（白名单）：`Container_name`、`Note`、`Status`、`IsAbnormal`、`Gpu_type`、`Gpu_num`、
`Cpu_num`、`Memory`、`District`、`Host`、`Url`、`SshPort`、`Image`、`ContainerType`、`Due_time`、
`End_time`、`DiskReleaseTime`、`ServerTime`、`Total_cost`、`PayTypeFirst`、`Performance`、
`InstanceAutorenew.SubscribeStatus`、`Ctime`。

🔴 **黑名单（实测存在于响应里，绝不落到 UI / JSON / 日志）**：
`Init_passwd`、`LastInitPasswd`、`RdpPasswd`、`VncPasswd`——实例 root/桌面明文口令。
实现用**显式字段白名单结构体**（未知字段自然丢弃），解析层不留 `map[string]any`，渲染/序列化层不可能带出。

### 2.5 到期时间口径

实例按小时租、自动续费，`Due_time` 是 Unix 秒。同响应里有平台侧 `ServerTime`，
`remaining = Due_time - ServerTime` 与本机时钟无关：

```go
deadline = now + (Due_time - ServerTime)   // ServerTime>0 时
deadline = Due_time                         // 否则
```

倒计时文案「33分后到期 / 1小时20分后到期 / 即将到期 / 已到期」——与 §六 项目专属要求同源：
**时间信息恒显**，异常徽章（如「已到期」「运行异常」）与倒计时并存，不得互相替代。

### 2.6 实测快照（2026-08-29 18:17）

```
余额 Money=97.1505 · PowerMoney=0 · CreditMoneyQuota=0 · VipLevel=2 · Name=用户#2433
统计 statusAll=85 statusRunning=4 statusKeeppedDisk=0 statusCreateError=0（statusDefault=9 与列表不符，弃用）
运行中 4 台（与 ~/.config/train-watch/servers.json 逐台对上）：
  js4.blockelite.cn:13024  lyg2030  GeForce RTX 3080×1  16C/48G  due 1788000414
  js1.blockelite.cn:28540  lyg0175  CPU                 8C/16G   due 1788000636
  js1.blockelite.cn:20812  lyg0098xh CPU                8C/16G   due 1788000681
  js1.blockelite.cn:27012  lyg0160  CPU                 2C/2G    due 1788004094
DNS 实测：js1.blockelite.cn → 223.109.239.11、js4.blockelite.cn → 223.109.239.36
```

`Url:SshPort` 与 servers.json 的 `endpoint` 同物（一个是域名、一个是 IP）——这正是本功能的价值点：
服务器被回收/到期重建时，这里能第一时间看到「已到期」与实例消失。

## 三、CLI 面（Go）

```
llm-api-check galaxy [名称|ID] [--no-refresh] [--limit N]
llm-api-check accounts add --type galaxy --name 名称 --access-key AK --secret-key SK
llm-api-check status                         # 总览含智星云卡片
```

- 凭据：flag `--access-key/--secret-key` → env `LLM_API_CHECK_GALAXY_ACCESS_KEY` / `LLM_API_CHECK_GALAXY_SECRET_KEY` → TTY 提示（同既有 `resolveSecret`）
- 配置文件：`galaxy_accounts`（与其余三类同权限 0600）
- `--json`：`accessKey`/`secretKey` 一律 `maskSecret`；`Phone` 脱敏为 `183****2433`
- `--limit` 默认 10，控制列出的实例条数（列表仍按 `statusDefault` 过滤，`--all-instances`? 不做——已结束实例对监控无价值，统计行已给 statusAll）

## 四、Android 面对等要求

同一份契约，命名沿用 Kotlin 风格：
`GalaxyAccount(id,name,accessKey,secretKey)` / `GalaxyRepo` / `galaxy_accounts_json`（SecureSettings 加密）/
`HomeScreen` GalaxyCard（余额 + 运行中 N + 最近到期倒计时）/ `DetailScreen` GalaxyDetailScreen（余额明细 + 实例卡列表）/
`SettingsScreen` 两个输入框（AccessKey、SecretKey，均可选粘贴 `ak=`/`sk=` 前缀清理）。
倒计时恒显 + 异常徽章并存（§六），口令字段不得进入数据类。

## 五、两侧共同的质量门

- Go：`gofmt` 零差异 / `go vet` / `go test ./... -race` / 四平台交叉编译 / 真机冒烟（真实 AK/SK）
- Android：`./gradlew :app:testDebugUnitTest` 全绿 + `assembleDebug`
- 凭据只进 `~/.config/llm-api-check/config.json`（0600、gitignored），**绝不写进仓库、README、plan 文档**

## 六、双实现对照审查与修复（2026-08-29，oracle/momus 并行双审）

oracle 对 Go/Kotlin 双侧做逐项等价性验证（含 kotlinx 1.7.3 字节码反汇编），
结论：**真实快照形态下两侧输出一致**；差异全部在防御路径（平台返回非契约形态时）。
已修复：

| # | 分叉点 | 修复 |
|---|---|---|
| 1 | Go `rawInt` 只认数字、Kotlin 认数字字符串 → `Status:"1"` 时 Go 显示已结束 | Go rawInt 加字符串宽容（顺带修 DeepSeek code:"40003" 漏判） |
| 2 | Kotlin `booleanOrNull` 认 `"false"` 字符串 → `has_more:"false"` 继续翻页 | Kotlin galaxyRawBool/信封 success 只认 JSON 布尔 |
| 3 | `Money:null`：Go 静默 0 成功 vs Kotlin 报错 | Go 显式报错（缺失或 null 同判） |
| 4 | `list:null`：Go 空列表成功 vs Kotlin 抛异常 | Go 显式报错 |
| 5 | `code:2000.5`：Go 截断判成功 | Go 只认整数值 |
| 6 | `Cost()` 三次取时钟，跨午夜口径不稳 | 循环外取一次 |

momus Android 侧 P2×10 已修复 7 项（toString 打码、错误消息不携 body、稳定
LazyColumn key、乱序签名向量、sorted() ASCII 前提注释、GalaxyCard 错误可见/
磁盘保留 0 不占位、详情页无活跃实例空态）；P2-7（函数组织）与 P2-9（Keystore
明文兜底，既有设计）按现状保留并记录。
