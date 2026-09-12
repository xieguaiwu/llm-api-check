# CONTEXT_FOR_NEXT_AGENT.md

## 最后一次完成的工作（2026-09-13：provider=longcat）
- **provider=longcat（美团龙猫）**：`llm-api-check longcat` 看余额状态 + 模型清单
  （OpenAI 兼容 API，无公开配额 API，靠小额推理探活间接判断余额）。
  - 🔑 **通道**：`GET https://api.longcat.chat/openai/v1/models`（模型清单，Bearer 认证）+
    `POST https://api.longcat.chat/openai/v1/chat/completions`（余额探活，max_tokens=1）。
    200=余额充足，402=余额不足，401=key 无效。
  - **额度语义**：LongCat 无公开配额 API。余额靠小额推理探活间接推断
    （402=余额为 0/已耗尽，200=余额 >0）。模型清单走 /v1/models（OpenAI 兼容）。
  - **错误语义**：401 `invalid_api_key`（key 无效/缺失）；403 `insufficient_quota`
    （key 有效但余额不足，实测 /v1/models 此时仍回 200）；429 `rate_limit_exceeded`。
  - 施工：models 三型（LongCatAccount/LongCatModel/LongCatPlan/LongCatUsage）；
    parsers.ParseLongCatModels（OpenAI 兼容信封）+ ErrLongCatAuth；
    repo.LongCatRepo（Models + ProbeBalance 两路并发）；
    app.LongCatResult + refreshLongCat + RefreshAll 并发接入；
    render 详情/总览；main cmdLongCat + accounts add/list/remove/rename 全接。
  - 质量：gofmt 0 / vet 0 / 7 包 -race 全绿；用例 **346**（longcat 新增 ~25 个）；
    真机冒烟待用户（cpu1/cpu2 实测）。未 commit。

## 最后一次完成的工作（2026-09-07 凌晨：provider=gptzero）
- **provider=gptzero（GPTZero AI 检测额度）**：`llm-api-check gptzero` 看月度词数额度
  （论文扫 AI 率的配额盯梢），单端点只读，真实 key 真机全链通。
  - 🔑 **通道**：`GET https://api.gptzero.me/v2/users/me`，头 **`x-api-key`**（非 Bearer），
    key 为 32 位 hex（app.gptzero.me 登录 → API 订阅页创建）。直连可通无需代理；
    请求必须带浏览器 UA（历史教训：Python urllib 裸 UA 被 Cloudflare 403 error 1010）。
  - **额度语义**：计费单位是**词**。`monthly_input_words`（已用，实测 587）/
    `full_plan.word_limit`（内含 300000）/ `full_plan.overage_word_limit`（超额上限 1000000，
    官方口径 300k+0.7M=1M 总顶，不加总）/ `priceData.unit_amount`=4500 美分=$45/月 /
    `last_time_usage_reset`=周期起点（重置≈+1 月，渲染标 ≈）/ `char_limit`=150000 是
    **单文档**上限（非月度）。实测快照：587/300000 词、历史 1,675,389 词 861 文档。
  - **错误语义（实测）**：无 key→401 `{"error":"Require valid cookie"}`；坏 key→403
    `{"error":"API key has no owner","apiKey":"<原样回显>"}`——**回显带 key**，doGet 层
    归一 `parsers.ErrGptzeroAuth`，原文不上抛；404 Express `Cannot GET` HTML。
  - 施工：models 四型（GptzeroAccount/Counters/Plan/Usage + GptzeroPercentUsed，
    limit≤0→-1 不画条）；parsers.ParseGptzeroUsage 两段拆包（缺 data/缺
    monthly_input_words/缺 full_plan.word_limit 显式失败，不得显示 0 误报）；
    **白名单不接 api_key 字段（响应含明文 key）**；repo.GptzeroRepo（x-api-key+
    浏览器 UA）；app 单 lane + RefreshAll 并发；render 详情/总览（超额 ≥100% 红标
    「已进入超额计费区」）；main cmdGptzero + accounts add/list/remove/rename 全接。
  - 质量：gofmt 0 / vet 0 / 7 包 -race 全绿；用例 **291→322**（gptzero 31 个：
    parsers 8/repo 5/app 5/render 7/main 6）；**反向验证 7 做**（缺 data 检查 /
    月用量必须项 / UA 头 / 403 归一 / 超额红标 / LastReset 守卫 / status json 键——
    拆除后对应测例全 FAIL）。README/README_zh 六处双语同步；契约
    docs/plans/2026-09-07-gptzero-provider.md。真机：gptzero / --json 掩码 /
    --no-refresh / 过滤 / 坏 key exit 1 / accounts 全生命周期 / status 总览全通过。
  - 真实 config 已录账号「论文扫」（真 key）；**未 commit**（同上轮 A-F 惯例，等用户确认）。

## 最后一次完成的工作（2026-09-06 下午，本日第二轮：A-F 六任务）
- **本轮范围**：用户指「根据 Coding/index.md 完成剩下工作」→ CONTEXT 待办全盘动工：
  A bai --stats / B 全仓错误消毒 / C P3 四小修 / D 免费通道探活 / E bai 告警阈值 /
  F Android bai 对等（切仓 pocket-llm-api-checker，commit 9d0c9c2）。资源评估
  RISK=CRITICAL（defer_or_direct）→ 全程直接施工未派 subagent。
- **A. bai --stats（usage.records）**：models.BaiRecord/BaiUsageStats +
  AggregateBaiUsage（requests 降序/同数字典序，窗口取 min/max created_at）；
  parsers.ParseBaiRecords（信封拆包重构为 baiEnvelopeOf 三端点共用；data 缺席=显式
  失败，空数组=合法零记录）；repo.Records/Stats（串行 10 页×100=1000 条封顶，
  has_more 自然终止 complete=true，中途页失败保部分+标中断页——不并行翻页：
  新请求持续入队推移窗口有页间重复竞态）；app.RefreshBaiStats；render.renderBaiStats
  （截断必标「数据不完整」）；cmdBai --stats（moveFlags 已认 --stats）+ publicBaiResult
  stats 键。真机：1000 条 7.6s，glm/qwen/deepseek 三模型聚合，边界 09-05~09-06。
- **B. 全仓错误文本消毒（momus P2 留档项兑现）**：parsers.SanitizeText 原语
  （CSI/OSC/两字符转义字节态机 + 控制字符剥离，保留 \n\t，UTF-8 按字节处理，
  序列有字节上限防无终止符拖死）；接入 6 处——BAI 信封×2、qwenErrorOf、
  repo.truncate200（先消毒后截断，覆盖 doGet/doPost/galaxy 5 个 HTTP 错误体出口）、
  qwenCLIErrorEnvelope message/hint、qwenCLIStderrTail（顺带修 08-30 留档的字节截断
  P2 → rune 安全）。**教训**：先截断后消毒会把 ESC 序列拦腰切断留下残序列——
  必须先消毒后截断（探活测试反向抓到该顺序缺陷）。
- **C. P3 四小修**：①NewID→NewIDE (string, error)，cmdAccountsAdd 统一预生成 accID
  （五处调用点收敛一处）②writeJSON 编码失败写 w 本身 error 信封 + os.Stderr 保底
  ③promptTTY 进程级共享 bufio（stdinSourceFor，源变重建）——互动机连续 prompt
  不再吞缓冲行④--json --version 出 JSON 信封。
- **D. 免费通道探活**：models.BaiProbe + BaiPlan.Probes；repo.ProbeFreeFlash
  （POST chat/completions max_tokens=8，200=alive / 401·403 归一认证文案 / 其他
  带消毒摘要）；refreshBai 模型 lane 成功后自动探盯梢清单内存在模型（缺失项不探）；
  render 免费通道四态（✓存活/✓未探/⚠运行时故障+摘要/✗缺失）+ 总览故障注记。
  真机：4 路全绿（deepseek-v4-flash 今晨 503 已自愈——间歇故障只有真发推理才查得出，
  正是探活的价值）；status 全链 6.7s。
- **E. bai 告警阈值（口径=用户选「只警过期部分」）**：models.BaiExpiringWarnPoints=
  1_000_000（≈$1，低于不扰）；详情「过期提醒 …不花就没了」黄色 + 总览注记；
  耗尽态（≤0）红色已最高级不叠提醒；低于阈值不提醒。真机 7,166,086 触发正确。
- **F. Android bai 对等**：pocket-llm-api-checker commit 9d0c9c2——数据层三件
  （Models/Parsers/Repositories，aggregateBaiUsage 等口径逐条对齐）+ BaiUi/BaiCard/
  BaiDetailScreen（导航 bai/{id}）/Settings 分区 + refreshBaiNow（探活+stats
  best-effort，§12 调用点自查）+ sanitizeServerText。122 测试 0 失败，lintDebug 绿
  （顺手修 themes.xml forceDarkAllowed 基线 error），assembleDebug 17.1MB。
  ⚠️ 真机冒烟待用户（本机无 adb）；Android commit 未 push。
- **质量门（Go 仓）**：gofmt 0 / vet 0 / 7 包 -race 全绿；用例 **258 → 291**；
  **反向验证 11 做**（rawInt64 严格化 / data 缺席静默 / 翻页封顶拆除 /
  中途失败改整体失败 / 聚合排序反转 / 截断标注拆除 / cmdBai stats 接线拆除 /
  publicBaiResult stats 键拆除 / 消毒原语拆除×2 / 探活接线拆除——各自对应测例
  FAIL）。新二进制已装 ~/.local/bin，真机冒烟：bai --stats / bai --json / bai /
  status / --json --version 全通过。**未 commit**（见下方待办）。

## 最后一次完成的工作（2026-09-06 12:5x）
- **bai 积分额度显示（推翻 09-04「配额不可得」结论）**：`llm-api-check bai` 现在显示
  `积分余额 27,166,591（≈ $27.17） · 其中 7,166,591 即将过期` + `本月消耗 2,833,409（≈ $2.83）`。
  - 🔑 **通道**：`GET https://chat.b.ai/trpc/lambda/usage.points` 与 `…/usage.summary`，
    **认证头就是同一把 `Authorization: Bearer sk-…`**（无需 Cookie/会话）。假 key/无头 → 401
    tRPC 信封 `error.json.data.code=UNAUTHORIZED`。
  - 🩹 **旧结论为何错**：09-04 只探了 `api.b.ai` 一个域名（那里确实只开放推理路径，403 原文可查）。
    推理面与控制台是两个域名、两套网关——`chat.b.ai` 是 LobeChat 分支，自带 tRPC 数据 API。
    **教训：「平台不提供 X」这类否定结论必须逐域名/逐子域取证，单域 403 不能外推。**
  - 取证路径：页面 chunk → webpack 异步 chunk 表（`u.u=c=>…` + `{id:"hash"}`，89 个）→
    chunk 193735 找到 `usage.points/summary/records` 与 `url:"/trpc/lambda"`。
  - 单位：`1 积分 = 1e-6 USD`（`basicConfig.creditsPerDollar=1000000`，与
    `getRechargeBonusStat` 1000 美分 ↔ 10 000 000 积分互证）。`points_expiring` = 赠送池余额
    （购买积分不过期），逐笔对账成立：30 000 000 − 2 833 409 = 27 166 591 = balance，
    10 000 000 − 2 833 409 = 7 166 591 = expiring。详见 docs/plans/2026-09-04-bai-provider.md §二-b。
  - 施工：`BaiRepo` 新增 `ConsoleURL`；`Points()` 串接 points+summary（summary 失败不致命，
    `HasMonthly=false`）；`app.refreshBai` 两路并发（模型路 api.b.ai / 额度路 chat.b.ai 不同主机）；
    `parsers.ErrBaiAuth` 成两路共用常量 + `app.joinErrors` 整行去重（同 key 坏掉时只读一句）；
    `rawInt64` 容忍整数/浮点/字符串三形状，`points_balance` 缺席 = 显式失败（不得显示 0 误報额度用尽）；
    标签列统一 `baiLabel`=`padTo(s,8)`（旧 `%-12s` 配中文按字节补位，与硬编码空格行不同列）。
  - 质量门：gofmt 0 / vet 0 / 7 包 `-race` 全绿；用例 **242 → 258**；**反向验证 10 做**
    （删 tRPC 信封分支 / rawInt64 去容错 / summary 改致命×2 / 去错误去重 / 退出码漏 Points /
    标签退回 %-12s / 「暂无数据」漏 Points / tRPC 路径写错——对应测例均 FAIL）；
    新二进制已装 `~/.local/bin`，**真机四路冒烟**：有效 key（1.2s，额度+48 模型）/ 坏 key
    （单行错误 + exit 1）/ `--json`（points 对象 + key 掩码）/ `--no-refresh`（6ms 不联网）。
  - **momus 审查轮（2026-09-06）**：结论「无 P0」，7 项重点逐条裁定通过（并发无竞争、
    joinErrors 去重对 DeepSeek/Qwen/Galaxy 无回归——全仓无「依赖重复错误」的断言、
    退出码口径与 qwen/galaxy 同构、凭据不外泄、对齐与 rune 安全、三处文档逐字一致）。
    - 🔴 **P1-1 已修（真缺陷，先复现再修）**：`rawInt64` 的浮点/字符串回退路径缺 int64
      范围与 Inf 守卫——`1e30` / `9.3e18` / `"1e30"` / `"Inf"` 全部返回
      `(-9223372036854775808, ok=true)`（float→int64 越界转换结果由实现定义，amd64 得 MinInt64；
      `strconv.ParseFloat("Inf")` 还不报错）。后果正是本次要消灭的那类误导：垃圾值进 `Balance`
      → 触发 `≤0` 红警「额度已耗尽，推理请求会失败」，把解析失败伪装成额度用尽。
      修法：`math.IsNaN/IsInf` + `f >= 2^63 || f < -2^63` 拒为解析失败（`int64FloatUpper/Lower`
      常量，上界严格小于因 float64 恰能量表示 2^63）；新增 `TestParseBaiPointsRejectsOutOfRange`
      （8 个越界形状 + 2^53-1 边界内不得误拒 + expiring 越界按缺席处理），反向验证第 10 做。
    - P2 已修 3 条：plan §施工约束「总时长不超模型路」算术错（额度路串行两次往返 ≈1.2s
      才是主导，与冒烟 1.2s 自相矛盾）、`RefreshBai` 注释漏积分路、render 测例双重否定
      `!Contains(...) == false` 拆成两条独立断言。
    - P2 **未修（记此防忘）**：tRPC 错误原文只截断未消毒，服务器可控文本里的 ANSI/控制字符
      会进终端与 `--json.error`。与 `ParseBaiModels`/galaxy/qwen 全部同模式——要修就全仓
      一处统一修（新增 sanitize 原语 + 各 provider 接入），单点加固反而制造不一致。
  - ⚠️ **`chat.b.ai` 本机直连超时**（`--noproxy '*'` 15s 无响应），`api.b.ai` 直连可通——
    Go 默认 `ProxyFromEnvironment`，跟系统 `HTTPS_PROXY`（127.0.0.1:7897），无需特殊处理。
  - 📌 附带观测（本轮未加功能）：`bai/deepseek-v4-flash` 推理当前回 **503
    `pre_consume_token_quota_failed`**（网关侧 subscription billing-info 查询失败），
    `glm-5.3-flash` / `qwen3.8-flash` 正常——上游通道故障，非本工具缺陷；免费通道盯梢只查
    模型清单存在性，查不出这种运行时 503。

## 最后一次完成的工作（2026-09-04 17:5x）
- **provider=bai（白B.AI api.b.ai，commit 1df753b）**：免费 0-Credits flash 通道盯梢（qwen3.8-flash / deepseek-v4-flash / vision-exp / glm-5.3-flash，缺失红警——pi-subagent 默认免费模型源）。one-api 系网关（x-oneapi-request-id），**仅开放推理路径**（billing 403 原文「HTTP node only allows access to inference API paths」），v1 只有模型清单。⚠️ **该「无配额数据」结论已于 2026-09-06 推翻**（漏探 chat.b.ai 控制台域，见上方新记录），「仅开放推理路径」只对 api.b.ai 成立。凭据 = ~/.config/fish/config.fish L274 BAI_API_KEY（chat.b.ai 侧栏创建）。实测：直连已通（config.fish「必须走代理」注释已过时）、10 路并发未复现 429、假 key 401 `Invalid token`、max_tokens≤2 拒 400。`llm-api-check bai` / `accounts add --type bai`（env LLM_API_CHECK_BAI_API_KEY）；测试 222→236，反向验证三做，真机 47 模型+4/4 绿 ✓。契约 docs/plans/2026-09-04-bai-provider.md。⚠️ 同 commit 还含 joinText 按行去重修复（见下）。
- **qwen provider 重新验证（全闭环）**：① API Key 通道实时 12 模型；② Bailian CLI 会话过期实测（config 停在 08-30）→ 错误文案/绝对路径/`bl` 改写防线全部按 08-30 修复生效；③ **发现并修掉 --stats 重复错误缺陷**：joinText 全串比对漏「上层已拼 Plan 错误」的行内重复 → 改按行去重（main.go joinText + TestJoinTextDedupesLines + e2e 计数断言，反向验证 FAIL 确认）；④ 用户浏览器 OAuth 重登后配额恢复：7天 96% · 8小时1分后重置（红 ≥90 正确）+ --stats 全通（45,351 tokens）；⑤ bailian-cli 1.18.1→1.20.0 漂移检查：输出字段名零变化（对比 bailian-cli-commands 包），升级安全暂不升。
- **Android 对等实现待办**：pocket-llm-api-checker 同名 provider 未施工（bai）；Galaxy 的 Android 侧真机冒烟仍欠（见下）。

## 最后一次完成的工作（2026-08-30 15:0x）
- **momus 审查轮收尾：3 个 P2 全修（审查结论本身是「通过、无阻塞」）**
  - ① 会话关键字集合收窄：删 `unauthorized` / `not authorised` / `not authorized`（权限类错误误归会话失效 = 让用户白跑一轮浏览器登录），补 `needlogin`（`BailianGateway.Login.NeedLogin` 是 cookie 路径已文档化的登录码，旧集合漏它）
  - ② `runJSON` 信封改为**两条流都查**（新增 `qwenCLIEnvelopeOf`）：旧实现 exit≠0 只看 stderr、exit 0 只看 stdout，上游改版捐走信封就退化成 `exit status 1` / 「JSON 解析失败: unexpected end of JSON input」（两种退化均有测例锁定）；同时验证「成功输出 + stderr 有非信封噪声」不得误判为错误
  - ③ 未装 CLI 不再给照做必失败的命令：默认独立 prefix 安装下 `bailian` **不在 PATH**。新增 `QwenRepo.CLILoginCmd()`（探测到→真实路径；未探测→`~/.local/share/bailian-cli/bin/bailian …`）+ `CLIInstallCmd()`，`QwenResult` 多两个 `json:"-"` 字段（`CLILoginCmd`/`CLIInstallCmd`，不过 `publicQwenResult`，`--json` 不泄），详情页未探测到时多打安装+登录两行，`--stats` 错误文案同口径
  - 质量门：gofmt 0 / vet 0 / 7 包 `-race` 全绿 / 用例 218 → 222；**反向验证 5 做**（加回 `not authorized`、去掉 `needlogin`、去掉双流信封、丢弃 `CLILoginCmd` 传递、不打印安装行——各自对应测例均 FAIL）；新二进制已装 `~/.local/bin`，真机三路径冒烟通过（配额有效 / `LLM_API_CHECK_QWEN_CLI=off` 两行指引 / `--stats` 未探测文案）
  - 文档：plan §一-b.1 重写（关键字集合理由 + 双流 + 安装指引）、README/README_zh 常见问题各补一条
  - ⚠️ 未做（非阻塞）：`qwenCLIStderrTail` 的 `tail[len(tail)-300:]` 仍是**字节截断**（上游若打中文错误可能切出乱码；同型问题在 `parsers.go` 的 `redactURL`/`truncateBody` 也存在）——下一位动这块时可改成 rune 安全并补测

## 最后一次完成的工作（2026-08-30 14:35）
- **修复「qwen 额度看不到但提示没用」（会话失效文案缺陷）**
  - 复现根因（实测，非推断）：`~/.bailian/config.json` 控制台会话于 08-29 13:58 写入，08-30 已过期 → `bailian usage token-plan` 回 exit 3 + 信封 `Console session is not logged in or has expired.`，hint 写 **`Run \`bl auth login --console\``**
  - 🔴 **真缺陷**：旧文案把上游 hint 原样透给用户。本机 `bl` = `~/.local/bin/bl`（用户自研翻译 CLI，无 `auth` 子命令）→ **照提示做会调错程序**，正是本文件铁律 ①（禁 `bl`）的同型陷阱，只是从「我们调」变成「我们告知」
  - 修（`internal/repo/qwen_cli.go`）：`runJSON` 错误分**三档**——① 会话失效（`not logged in`/`no console access token`/`has expired`/`notlogin`/`login required`/`unauthorized`/`not authorised`）→ 中文单行提示 + **探测到的 bin 绝对路径**（可直接粘贴）；② 其他信封错误 → `Bailian CLI 返回错误: <原文>`，不谎报「登录就好」；③ 无信封 exit 非零 → 原文洗 Node 噪音。新增 `qwenCLIRewriteBin`（正则 `(^|[\s\x60])(bl)(\s+auth|usage|…)` → 真实 bin；退化用 `bailian`，绝不退 `bl`；`ReplaceAllStringFunc` 避开 `$` 转义）
  - 副修：`cmdQwen --no-refresh` 不回填 `CLIEnabled` → 装了 CLI 仍显示「需控制台 Cookie 或 Bailian CLI」，把用户推向不必要的 Cookie 抓取
  - 文档：README/README_zh 新增「常见问题 / Troubleshooting」节（含 `bailian auth status` **不校验会话有效性**、只看 config 存了什么这一实测结论）；顺带补 README_zh 缺失的「Bailian CLI（配额首选通道）」凭据行（中英双语漂移）；契约见 docs/plans/2026-08-29-qwen-provider.md §一-b.1
  - 质量门：gofmt 0 / vet 0 / 7 包 `go test -race` 全绿；用例 212 → 218（+6：会话分档×2、bl 改写×1、分类器×1、main 级 e2e×2）；**反向验证三做**（去掉 bl 改写 / 去掉会话判定 / --no-refresh 不回填 → 对应测试均 FAIL）
  - 真机证据（新二进制已装 `~/.local/bin`，会话仍过期状态）：旧 `Bailian CLI 不可用: Console session is not logged in or has expired.（Run \`bl auth login --console\` …）` → 新 `Bailian CLI 未登录或会话已过期（会话通常数小时失效）：运行 /home/xieguiawu/.local/share/bailian-cli/bin/bailian auth login --console 在浏览器中重新登录后重试`
  - ✅ **已端到端验证（2026-08-30 14:36，用户浏览器 OAuth 后）**：`llm-api-check qwen` → `7天 [█████████░] 85% · 130小时44分后重置`、exit 0、无错误行；`--stats` 同时正常（14 次调用 · Total 14,785 tokens · qwen3.8-max 免费额度 99.8%）。登录流程本身无变动（CLI 起本地回调 `127.0.0.1:43375`，Firefox 弹页）
  - 📌 当日实测：Qwen Token Plan **7 天窗口已用 85%**（峰时段主力模型路由到 qwen3.8-max 会继续消耗）；5 小时窗口字段仍缺席（官方限时取消）

## 最后一次完成的工作（2026-08-29 18:50）
- **provider=galaxy（智星云 AI Galaxy 算力云，v1.3.0）**：`llm-api-check galaxy` 看账户余额 + 租用的云主机实例状态（只读，无写操作）
  - 通道 = **官方 OpenAPI v2**（AccessKey + SecretKey + MD5 签名，`POST https://app.ai-galaxy.cn/openapi/v2/…`，表单体带 `apikey/timestamp/nonce/sign`）；**不走控制台 session**（会随登录过期，且无官方支持）
  - 真机联调通过（2026-08-29 18:17 起多次实跑）：余额 ¥96.28、statusAll 85 / 运行中 4、四台实例与 `~/.config/train-watch/servers.json` 逐台对上（js1/js4.blockelite.cn = 223.109.239.11/.36）
  - 数据面四路并发：`account/get_main_account_info`、`instance/get_instance_status_count`、`instance/get_instance_list`（翻页 + page_size 夹 100）、`billing/get_balance_change_list`（今日/近7天净消耗，双窗口各自完整性标记 → 未翻完数字前加 `≥`）
  - 🔴 **口令屏蔽**：实例列表响应含 `Init_passwd/LastInitPasswd/RdpPasswd/VncPasswd` 明文口令 → 解析层显式白名单结构体，`--json` 与渲染层不可能带出（有单测断言 SECRET_PWD 哨兵不出现）；刻意不调 `account/get_apikey_info`（会回吐 SecretKey）
  - 🔴 **弃用 statusDefault**：实测统计端点回 9、同 status_type 列表只回 4，两数互相矛盾 → 统计只显示自洽的 4 个字段（契约 §2.4）
  - 到期倒计时用 `Due_time - ServerTime` 折算（与本机时钟无关），状态异常徽章与倒计时并存（§六 要求）；`padTo` 按显示宽度对齐（`%-10s` 对中文按 rune 计数会错位）
  - 凭据：`--access-key/--secret-key` → `LLM_API_CHECK_GALAXY_ACCESS_KEY/…_SECRET_KEY` → TTY；配置 `galaxy_accounts`；`--json` 里 AccessKey 也掩码
  - **顺带修**：`moveNoRefresh` → `moveFlags`（旧实现每类 flag 只搬一个，`名称 --limit 3 --no-refresh` 会漏搬 `--limit`）
  - 测试：新增 64 个（parsers 16 / repo 15 / app 8 / render 13 / main 12），共 206 个用例 7 包 `-race` 全绿；反向验证：把签名字典序改成逆序后 `TestGalaxyRequestWireFormat`（独立复算 MD5，不复用被测函数）与 parsers 用例同时失败
  - 契约文档：`docs/plans/2026-08-29-ai-galaxy-provider.md`（含调查取证：Vue bundle 分析 → Apifox llms.txt → 真实 AK/SK 实测矩阵）

### 待办：Android 对等实现
`~/Desktop/go-projects/pocket-llm-api-checker` 的同名 provider 正在并行施工（GalaxyRepo/GalaxyCard/GalaxyDetailScreen/SettingsScreen 录入 + 单测），契约以 Go 侧 plan 文档为准。真机冒烟与凭据录入由用户完成。

## 项目当前状态
llm-api-check v1.3.0 — Go CLI，复刻 Android app「API Checkers」（现名 pocket-llm-api-checker）的数据层逻辑：查看 DeepSeek（余额 + 消费）、OpenCode（Go 三窗口 + Zen billing）、**Qwen Token Plan（套餐模型 + 5 小时/7 天 配额窗口）**、**智星云 AI Galaxy（算力云余额 + 云主机实例状态）**、**白B.AI（积分额度 + 免费 flash 通道模型清单）** 与 **GPTZero（AI 检测月度词数额度，2026-09-07 新增）** 用量，多账号。**Qwen 配额已通：官方 Bailian CLI 通道（浏览器 OAuth 一次登录）已落地并实跑成功——最近一次验证 2026-09-04（7天 96% · 8小时1分后重置）；CLI 优先、控制台 Cookie 兜底。**BAI 积分额度已通（2026-09-06）：走 `chat.b.ai` 控制台 tRPC，同一把 sk- key 作 Bearer，无需 Cookie；`api.b.ai` 本身仍只开放推理路径（其余 403）——不要再拿 api.b.ai 的 403 当「bai 无额度数据」的依据。**GPTZero 已通（2026-09-07）：`GET /v2/users/me` x-api-key 认证，计费单位是词。****

🔴 **Android 侧唯一权威 clone（2026-08-29 取证）= `~/Desktop/go-projects/pocket-llm-api-checker/`**（HEAD e1c1568，含 fastlane 元数据 + tag v1.0.0 + scripts/ 可复现构建 + docs/fdroid 草稿）。`~/Desktop/android-projects/api-checkers/` 是落后一提交的旧副本（HEAD cec6ef7，无 fastlane、无 tag），只做历史参考，**勿在其上开发**。

**公开 repo：https://github.com/xieguaiwu/llm-api-check（PUBLIC）**
发布物：GitHub Release v1.1.0（linux amd64/arm64 + darwin amd64/arm64 tarball + sha256sums.txt）

## 最后一次完成的工作（2026-08-29 14:10）
- **provider=qwen --stats 用量分析（v1.2.0 追加）**：`llm-api-check qwen [名称|ID] --stats` 显示 7 天 token 统计 + 免费额度
  - 数据源：`bailian usage summary --output json`（一个命令含 period/freeTier/usage 三块）；`QwenCLI.runJSON` 抽取共用（Usage/Summary 同走 argv+超时+stderr 过滤+信封识别）
  - 渲染：周期、调用模型数/成功次数、Input/Output/Total/Avg Tokens（千分位 formatInt）、免费额度只列已用模型（剩余<100%，全未用提示「未使用」）
  - `moveNoRefresh` 扩展：`--stats` 在位置参数后也能被 flag 解析（同 `--no-refresh` 的坑）
  - 错误文案统一「Bailian CLI 不可用: …」；测试 +7（repo Summary×2、render Stats×2+formatInt、main e2e --stats）；7 包 -race 全绿
  - 实跑：`llm-api-check qwen --stats` → 2 模型 · 14 次调用 · Total 14,785 tokens · qwen3.8-flash 98.7% 免费额度
  - ⚠️ 会话短寿命实测：OAuth 登录约几小时即过期（13:46 登录 14:0x 已报 Console session not logged in or has expired）→ 过期重跑 `bailian auth login --console` 即可（首次登录进程若卡住先 kill 再重开）

## 遗留问题 / 待办
- [ ] **两轮改动未 commit**（2026-09-06 A-F 五任务 + 2026-09-07 gptzero）：均过全部质量门，等用户确认后提交（公开仓 push 须用户同意）
- [ ] **Android 真机冒烟（bai）**：本机无 adb 设备，用户手机装 9d0c9c2 包后录入 sk- key 验证 bai 卡/详情页/探活红行；Android commit 同样未 push
- [ ] **bai order.listOrders / records 逐条视图（未做）**：充值流水与逐请求明细列表 UI 未接（--stats 已覆盖聚合视图）；需要时再加
- [x] ~~**Bailian CLI 会话过期维护**：差异化文案未做~~ → **2026-08-30 已做**：会话失效单独立档，给出可复制的 `bailian auth login --console`（绝对路径）；不透传上游 `bl` hint。运维动作不变：过期重跑 `~/.local/share/bailian-cli/bin/bailian auth login --console`（浏览器 OAuth；登录进程卡住先 kill 再重开）。⚠️ `bailian auth status` 不校验会话有效性，判活看 `bailian usage token-plan`
- [ ] **国际区域（ap-southeast-1）无凭据、未实跑**：代码按公开契约实现（`QwenEndpointsFor` + CLI `--console-site international`，CLI 通道单测覆盖了 intl 参数）；启用前先验证 `/tool/user/info.json` 的 sec_token 字段名
- [ ] **DeepSeek 平台 token / workspace ID + auth cookie 未配置**（config.fish 无）：DeepSeek 消费明细与 Zen billing 不可用。平台 token 需浏览器登录 platform.deepseek.com 后从 DevTools 抓；Zen 需 opencode.ai 的 workspace ID + auth cookie（`Fe26.2` 开头）
- [ ] songjieshi/xieguaiwu 的 key 来自 config.fish 注释行（非当前生效），可能已过期（xieguaiwu 实测 M 100% 已限流属正常用量而非 key 失效）
- [x] ~~P3 未修（非阻塞）：NewID panic 改返回错误、writeJSON stderr 注入、promptTTY bufio.Reader 复用、`--json --version` 文本输出~~ 仍未修（本轮只做了 momus 的 3 个 P2）；Qwen 额度绝对值（quota-config 接口的 `five_hour`/`weekly` credits）未接入，现只显示百分比（CodexBar 已接 quota-config，可参考）
- [ ] Zen billing 解析依赖 opencode.ai 页面结构，改版需更新 `internal/parsers/parsers.go` 的 ParseZenBilling；Qwen 同理依赖百炼控制台 RPC（信封形状变化时改 `qwenFindObject` 目标键）或 bailian-cli 输出（字段变化时改 `qwenCLIErrorEnvelope`/`ParseQwenUsage`）
- [x] ~~**发版 v1.3.0**~~ → **2026-09-06 已发**：`VERSION=1.3.0 scripts/build-dist.sh`（四平台 tarball + sha256sums）→ tag v1.3.0 → Release https://github.com/xieguaiwu/llm-api-check/releases/tag/v1.3.0（覆盖 v1.2.0+v1.3.0 全部内容：galaxy provider、bai 积分、qwen --stats、moveFlags/joinText 修复）。**下载回验通过**：从 Release 拉回 linux_amd64 包 `sha256sum -c` OK、解包 `--version` → 1.3.0；v1.3.0 已接管 Latest 标记。main 与 tag 均已 push
- [ ] **智星云可选增强**（未做，需要时再加）：`billing/get_instance_cost_summary` 单实例费用分解、`instance/get_instance_detail` 深看、`/store/*` 显卡价格与库存、自动续费开关状态细化、余额低于阈值告警（可接 belater 定时跑 `galaxy --json`）
- [ ] 图形知识图谱 graphify-out/ 未生成（可选，`graphify update . --no-llm`）

## 技术要点（下一位 Agent 必读）
- **白B.AI 铁律**：① 两个域名两套网关——`api.b.ai` 只开放推理路径（`/v1/chat/completions|/v1/messages|/v1/responses|/v1/models|/v1/images/*`，其余 403），积分/额度在 `chat.b.ai` 控制台 tRPC；② 同一把 `sk-` key 两处通用（Bearer），无需浏览器 Cookie；③ `chat.b.ai` 本机直连超时、必须走代理（`api.b.ai` 直连可通）；④ tRPC 错误信封优先于状态码，`UNAUTHORIZED` 归一到 `parsers.ErrBaiAuth`，其他 code 带原文上抛；⑤ `points_balance` 缺席必须显式失败（显示 0 = 误報额度用尽）；⑥ 1 积分 = 1e-6 USD（`creditsPerDollar`），渲染用 `≈ $` 标记提醒是名义换算；⑦ `usage.records` 对未知过滤键与非法 `page` 一律忽略不报错，不能拿它做参数校验（同智星云 `status_type`）。详见 docs/plans/2026-09-04-bai-provider.md §二-b
- **智星云 OpenAPI 铁律**：① 签名 = 非空参数字典序拼 `k=v&…` + 末尾 `&secret=<SecretKey>` → MD5 小写 hex，`sign` 进 body（**不入串**）；② HTTP 恒 200，错误在信封 `{success,code:"2000"}` 里，`code` 是**字符串**；③ `page_size` 上限 100（超限报 `page_size参数超限!`）；④ `status_type` 传非法值不报错、按不过滤处理，不能拿它做参数校验；⑤ 实例响应含明文口令（见上）；⑥ 到期时刻用 `Due_time-ServerTime` 折算；⑦ 平台错误码只有 "2000"（成功）/"4000"（客户端错误，message 说明原因）。详见 docs/plans/2026-08-29-ai-galaxy-provider.md
- **数据源**：Go usage = `GET https://opencode.ai/zen/go/v1/usage`（API key）；Zen billing = `GET https://opencode.ai/workspace/{id}/billing`（cookie，SolidJS SSR HTML，锚点 `customerID:"cus_`，balance 单位 1e-8 USD）；DeepSeek 余额 = `api.deepseek.com/user/balance`（API key，金额为字符串）；消费 = `platform.deepseek.com/api/v0/usage/cost?month=&year=`（浏览器 token，code 40003 = 失效，拉本月+上月聚合 30 天）；**智星云 = `POST https://app.ai-galaxy.cn/openapi/v2/{account/get_main_account_info,instance/get_instance_status_count,instance/get_instance_list,billing/get_balance_change_list}`（AccessKey+SecretKey 签名，表单编码）；**Qwen 模型 = `https://token-plan.<region>.maas.aliyuncs.com/compatible-mode/v1/models`（API key）；Qwen 配额 = Bailian CLI `bailian usage token-plan --output json`（Console 认证，首选）或 `POST https://bailian-cs.console.aliyun.com/data/api.json`（Cookie + sec_token，信封 `data.DataV2.data.data` 或内嵌 JSON 字符串）；BAI 模型 = `GET https://api.b.ai/v1/models`（API key，one-api 系信封 `{data:[{id,owned_by,supported_endpoint_types}],success}`）**；**BAI 积分 = `GET https://chat.b.ai/trpc/lambda/usage.points`（余额/即将过期）+ `…/usage.summary`（本月消耗），同一把 sk- key 作 Bearer，tRPC v11 信封 `{result:{data:{json:…}}}`，错误走 `{error:{json:{data:{code:"UNAUTHORIZED"}}}}`；只读 query，不接任何 mutate；**GPTZero = `GET https://api.gptzero.me/v2/users/me`（x-api-key 头，信封 `{data:{…}}` 含明文 api_key 字段必须白名单丢弃；计费单位词；坏 key 403 回显 key）**
- **Bailian CLI 通道铁律**：① 本机 `bl` 是用户自研翻译 CLI——探测与文档一律用 `bailian` 名/绝对路径，`exec.LookPath("bl")` 是被禁止的（会误调）；② `usage token-plan` 认证模式 Console，API key 无效；③ Node 的 UNDICI 警告在 stderr，必须过滤；④ exit 非零时 JSON 错误信封在 stderr（实测 exit 3）；⑤ CLI 会话存 `~/.bailian/config.json`（敏感文件）；⑥ 会话短寿命（几小时），过期重跑 `bailian auth login --console`；⑦ 实现文件 `internal/repo/qwen_cli.go`（QwenCLI.runJSON 共用 Usage/Summary 通道）
- **Qwen 三个不能踩的坑**（详见 docs/plans/2026-08-29-qwen-provider.md）：① `cornerstoneParam` 绝不得硬编码 `switchAgent`（网关会绑死该工作区 → 他人账号全部 NotAuthorised）② 抓 `SEC_TOKEN` 必须带 `Sec-Fetch-*` 浏览器导航头 + 桌面 UA，否则 OneConsole shell 不渲染该 token ③ 登录失效仍回 HTTP 200，错误在信封 `data.errorCode` 里，不能只看状态码
- **关键移植点**：ZenBilling 字符串字面量感知括号匹配（inStr/esc 状态机）、`(?:^|,)balance:` 正则、microcents÷1e8、cost 两月都无数据显式失败（不返回误导零数据）；QwenUsage 信封 BFS + 内嵌 JSON 字符串展开（深度上限 12）、`qwenPercent` 比例/百分数双域判定（>2 才当百分数）、空窗口重试 3 次而认证类错误不重试、CLI 单窗口响应独立判有（5 小时限时取消期间字段缺席）
- **错误消息**与 Android 版逐字一致（对照表在 docs/plans/2026-08-18-llm-api-check-cli.md「错误消息对照表」节；Qwen 新错文见 docs/plans/2026-08-29-qwen-provider.md）
- **安全**：凭据存 `~/.config/llm-api-check/config.json` chmod 0600（智星云 AccessKey 也算长期凭据——它能签名发起扣费请求，`--json` 同样掩码）（CLI 无 Keystore，文件权限替代 + 权限过宽警告）；凭据来源顺序 flag → 环境变量（`LLM_API_CHECK_*`）→ TTY 提示；`--json` 输出全部掩码凭据；Qwen 控制台 Cookie 含阿里云登录会话，当敏感文件对待；Bailian CLI 会话在 `~/.bailian/config.json`
- **2026-09-06 下午新增要点**：①stats 翻页串行 10 页×100（并行翻页有页间重复竞态——新请求持续入队推移窗口）；②消毒铁律「先 SanitizeText 后 truncate」——顺序反了会把 ESC 序列拦腰切断；③探活只探盯梢清单内存在模型（缺失项再探是浪费）；④过期告警口径=只警 expiring ≥100 万（用户定），耗尽态红色已最高级不叠；⑤「平台不提供 X」的否定结论要逐子域取证（chat.b.ai 推翻 api.b.ai 403 外推，本轮 §八 records 再证）
- **构建**：`go build -trimpath -ldflags="-s -w -X main.version=1.3.0" -o ~/.local/bin/llm-api-check .`；测试 `go test ./... -race`（7 包）；发版打包 `VERSION=x.y.z scripts/build-dist.sh`（四平台 tarball + sha256sums.txt → dist/，dist/ 不入库）
- **渲染**：中文输出、ANSI 颜色（NO_COLOR/--no-color 禁用）、用量条 10 格；中英混排对齐用 `render.padTo`（按显示宽度，中文 2 列），不要用 `%-Ns`、倒计时「4小时20分后重置 / 52分钟后重置 / 即将重置」、颜色阈值 <70 蓝 / 70-89 黄 / ≥90 红、限流与配额用尽强制红且**「已限流」徽章与重置倒计时必须并存**（index.md §六 项目专属要求）
- **本机环境**：密钥真值在 `~/.config/fish/config.fish`（非 dotfiles 符号链接、不入库）；该文件首行有 `if not status is-interactive; return; end` 守卫，`fish -c 'source …'` 取值会静默得空值——用 python 正则直读文件（见 System_Fix/dotfiles-sync-and-audit.md 附录 B.5）；订阅密钥与区域强绑定（北京 key 打新加坡端点 401，同 key 换区域即 200）；Bailian CLI 已装 `~/.local/share/bailian-cli/bin/bailian`（独立 prefix，不 shadow 自研 bl）

## 历史工作记录
- **2026-08-29 13:50（edb6db0）**：Qwen 配额 Bailian CLI 通道——官方 bailian-cli（npm，独立 prefix ~/.local/share/bailian-cli）接入，`bailian auth login --console` 一次 OAuth 后 `llm-api-check qwen` 显示配额窗口（CLI 优先、Cookie 兜底）；探测 env LLM_API_CHECK_BL_BIN → 安装位 → PATH bailian（禁查 bl）；LLM_API_CHECK_QWEN_CLI=off 禁用。详见 docs/plans/2026-08-29-qwen-provider.md §一-b
- **2026-08-29 14:10（a2c5968）**：`qwen --stats` 用量分析——`bailian usage summary --output json`（period/freeTier/usage 三块）；QwenCLI.runJSON 抽取共用；token 千分位；免费额度只列已用模型；moveNoRefresh 扩展提前 --stats。详见 docs/plans/2026-08-29-qwen-provider.md §七
- **2026-08-18 10:0x（c694d24）**：限流时限直接可见——对照 Android DetailScreen.WindowRow，rate-limited 行由「已限流」替代倒计时改为「已限流 · N小时M分后重置」并存；render_test 断言同步反转（限流行必须含倒计时）。实测 xieguaiwu Monthly → `已限流 · 175小时7分后重置`
- **2026-08-18 01:55**：发布公开 repo（System_Fix → dotfiles-sync-and-audit 审计：历史全量扫描真实 key 前缀零命中；fixture 为 TEST 占位符；config.json/.env 未入库）→ `gh repo create llm-api-check --public`；真实冒烟 4 账号通过；修复总览页已限流嵌套 ANSI 颜色（f2dbf22）
- **2026-08-18 01:40**：CLI 全功能实现（models/parsers/repo/config/app/render 六包 + main.go，零第三方依赖）；momus 审查修复（P1×3 + P2×4 + 8 个命令级测试）；v1.0.0 部署与双语 README

## 知识图谱
- graphify-out/: 存在（每次代码变更后 `graphify update .` 重建，no-LLM 零成本）
- 683 节点 / 1914 边 / 23 社区（2026-08-30 bugfix 后重建；旧记录「319/736/24」为跳仓残留数字，与 GRAPH_REPORT 不一致，已以图谱为准）
- 图谱以最新 commit 为准；若 `graphify-out/needs_update` 存在说明已陈旧，先 update 再依赖它回答

## 最后更新时间
2026-09-07 01:1x
