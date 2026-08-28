# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态
llm-api-check v1.1.0 — Go CLI，复刻 Android app「API Checkers」（现名 pocket-llm-api-checker）的数据层逻辑：查看 DeepSeek（余额 + 消费）、OpenCode（Go 三窗口 + Zen billing）与 **Qwen Token Plan（套餐模型 + 5 小时/7 天 配额窗口）** 用量，多账号。**Qwen provider 开发完成、7 包测试全绿（含 -race）、真实凭据冒烟通过（API Key 通道）、已部署 `~/.local/bin/llm-api-check`。**

🔴 **Android 侧唯一权威 clone（2026-08-29 取证）= `~/Desktop/go-projects/pocket-llm-api-checker/`**（HEAD e1c1568，含 fastlane 元数据 + tag v1.0.0 + scripts/ 可复现构建 + docs/fdroid 草稿）。`~/Desktop/android-projects/api-checkers/` 是落后一提交的旧副本（HEAD cec6ef7，无 fastlane、无 tag），只做历史参考，**勿在其上开发**。

**公开 repo：https://github.com/xieguaiwu/llm-api-check（PUBLIC）**
发布物：GitHub Release v1.1.0（linux amd64/arm64 + darwin amd64/arm64 tarball + sha256sums.txt）

## 最后一次完成的工作（2026-08-29 02:40）
- **provider=qwen（commit 7c51bfa + 收尾 commit）**：新增第三个数据源，与 Android 侧逐条对等
  - 数据模型：`QwenAccount{ID,Name,ApiKey,ConsoleCookie,Region}`、`QwenWindow{Percent,ResetsAt,Exhausted}`、`QwenUsage{PlanCode,FiveHour,Weekly}`、`QwenPlan{Models}`
  - 两个通道：**模型清单**走 API Key（`/compatible-mode/v1/models`）；**配额窗口**走百炼控制台 RPC（只认 Cookie，实测 API Key 回 `BailianGateway.Login.NotLogined`）
  - CLI：`llm-api-check qwen [名称|ID]`、`accounts add --type qwen`（`--api-key` / `--console-cookie` / `--region`）、list/remove/rename 跨三类账号、`--json` 新增 `qwen` 段并掩码新凭据
  - 测试：parsers/repo/app/config/render/main 均新增用例，6 个 fixture `testdata/fixtures/qwen_*.json`；`go test ./... -race` 7 包绿、`go vet` 干净
  - 附带修一个既有缺陷：`moveNoRefresh` 使 `llm-api-check qwen 名称 --no-refresh` 可用（Go flag 包遇首个位置参数即停止解析，deepseek/opencode 同样受益）
  - 详细契约、实测矩阵、未验证项见 **docs/plans/2026-08-29-qwen-provider.md**
- **本机配置完成**：`~/.config/llm-api-check/config.json` 已加 Qwen 账号「Token Plan 订阅号」（密钥取自 `~/.config/fish/config.fish` 的 `QWEN_API_KEY`，区域 cn-beijing，未配 Cookie）。实测 `llm-api-check qwen` → `模型 12 个：…`，exit 0
- **发版 v1.1.0**：`scripts/build-dist.sh` 出四平台 tarball + sha256sums.txt → tag v1.1.0 → GitHub Release

## 遗留问题 / 待办
- [ ] **Qwen 配额窗口未做端到端实跑**：需用户提供百炼控制台 Cookie（`accounts add --type qwen … --console-cookie '...'` 重新录入，或直接改 config.json）。RPC 契约已用未登录请求实测确认（回 NotLogined），但 sec_token 抓取、x-xsrf-token、空信封重试分支待真 Cookie 验证
- [ ] **国际区域（ap-southeast-1）无凭据、未实跑**：代码按公开契约实现（`QwenEndpointsFor`）；启用前先验 `/tool/user/info.json` 的 sec_token 字段名
- [ ] **DeepSeek 平台 token / workspace ID + auth cookie 未配置**（config.fish 无）：DeepSeek 消费明细与 Zen billing 不可用。平台 token 需浏览器登录 platform.deepseek.com 后从 DevTools 抓；Zen 需 opencode.ai 的 workspace ID + auth cookie（`Fe26.2` 开头）
- [ ] songjieshi/xieguaiwu 的 key 来自 config.fish 注释行（非当前生效），可能已过期（xieguaiwu 实测 M 100% 已限流属正常用量而非 key 失效）
- [ ] P3 未修（非阻塞）：NewID panic 改返回错误、writeJSON stderr 注入、promptTTY bufio.Reader 复用、`--json --version` 文本输出；Qwen 额度绝对值（quota-config 接口的 `five_hour`/`weekly` credits）未接入，现只显示百分比
- [ ] Zen billing 解析依赖 opencode.ai 页面结构，改版需更新 `internal/parsers/parsers.go` 的 ParseZenBilling；Qwen 同理依赖百炼控制台 RPC（信封形状变化时改 `qwenFindObject` 目标键）
- [ ] 图形知识图谱 graphify-out/ 未生成（可选，`graphify update . --no-llm`）

## 技术要点（下一位 Agent 必读）
- **数据源**：Go usage = `GET https://opencode.ai/zen/go/v1/usage`（API key）；Zen billing = `GET https://opencode.ai/workspace/{id}/billing`（cookie，SolidJS SSR HTML，锚点 `customerID:"cus_`，balance 单位 1e-8 USD）；DeepSeek 余额 = `api.deepseek.com/user/balance`（API key，金额为字符串）；消费 = `platform.deepseek.com/api/v0/usage/cost?month=&year=`（浏览器 token，code 40003 = 失效，拉本月+上月聚合 30 天）；**Qwen 模型 = `https://token-plan.<region>.maas.aliyuncs.com/compatible-mode/v1/models`（API key）；Qwen 配额 = `POST https://bailian-cs.console.aliyun.com/data/api.json`（控制台 Cookie + sec_token，信封 `data.DataV2.data.data` 或内嵌 JSON 字符串）**
- **Qwen 三个不能踩的坑**（详见 docs/plans/2026-08-29-qwen-provider.md）：① `cornerstoneParam` 绝不得硬编码 `switchAgent`（网关会绑死该工作区 → 他人账号全部 NotAuthorised）② 抓 `SEC_TOKEN` 必须带 `Sec-Fetch-*` 浏览器导航头 + 桌面 UA，否则 OneConsole shell 不渲染该 token ③ 登录失效仍回 HTTP 200，错误在信封 `data.errorCode` 里，不能只看状态码
- **关键移植点**：ZenBilling 字符串字面量感知括号匹配（inStr/esc 状态机）、`(?:^|,)balance:` 正则、microcents÷1e8、cost 两月都无数据显式失败（不返回误导零数据）；QwenUsage 信封 BFS + 内嵌 JSON 字符串展开（深度上限 12）、`qwenPercent` 比例/百分数双域判定（>2 才当百分数）、空窗口重试 3 次而认证类错误不重试
- **错误消息**与 Android 版逐字一致（对照表在 docs/plans/2026-08-18-llm-api-check-cli.md「错误消息对照表」节；Qwen 新错文见 docs/plans/2026-08-29-qwen-provider.md）
- **安全**：凭据存 `~/.config/llm-api-check/config.json` chmod 0600（CLI 无 Keystore，文件权限替代 + 权限过宽警告）；凭据来源顺序 flag → 环境变量（`LLM_API_CHECK_*`）→ TTY 提示；`--json` 输出全部掩码凭据；Qwen 控制台 Cookie 含阿里云登录会话，当敏感文件对待
- **构建**：`go build -trimpath -ldflags="-s -w -X main.version=1.1.0" -o ~/.local/bin/llm-api-check .`；测试 `go test ./... -race`（7 包）；发版打包 `VERSION=x.y.z scripts/build-dist.sh`（四平台 tarball + sha256sums.txt → dist/，dist/ 不入库）
- **渲染**：中文输出、ANSI 颜色（NO_COLOR/--no-color 禁用）、用量条 10 格、倒计时「4小时20分后重置 / 52分钟后重置 / 即将重置」、颜色阈值 <70 蓝 / 70-89 黄 / ≥90 红、限流与配额用尽强制红且**「已限流」徽章与重置倒计时必须并存**（index.md §六 项目专属要求）
- **本机环境**：密钥真值在 `~/.config/fish/config.fish`（非 dotfiles 符号链接、不入库）；该文件首行有 `if not status is-interactive; return; end` 守卫，`fish -c 'source …'` 取值会静默得空值——用 python 正则直读文件（见 System_Fix/dotfiles-sync-and-audit.md 附录 B.5）；订阅密钥与区域强绑定（北京 key 打新加坡端点 401，同 key 换区域即 200）

## 历史工作记录
- **2026-08-18 10:0x（c694d24）**：限流时限直接可见——对照 Android DetailScreen.WindowRow，rate-limited 行由「已限流」替代倒计时改为「已限流 · N小时M分后重置」并存；render_test 断言同步反转（限流行必须含倒计时）。实测 xieguaiwu Monthly → `已限流 · 175小时7分后重置`
- **2026-08-18 01:55**：发布公开 repo（System_Fix → dotfiles-sync-and-audit 审计：历史全量扫描真实 key 前缀零命中；fixture 为 TEST 占位符；config.json/.env 未入库）→ `gh repo create llm-api-check --public`；真实冒烟 4 账号通过；修复总览页已限流嵌套 ANSI 颜色（f2dbf22）
- **2026-08-18 01:40**：CLI 全功能实现（models/parsers/repo/config/app/render 六包 + main.go，零第三方依赖）；momus 审查修复（P1×3 + P2×4 + 8 个命令级测试）；v1.0.0 部署与双语 README

## 知识图谱
- graphify-out/: 不存在（可选生成）

## 最后更新时间
2026-08-29 02:45
