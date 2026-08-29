# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态
llm-api-check v1.2.0 — Go CLI，复刻 Android app「API Checkers」（现名 pocket-llm-api-checker）的数据层逻辑：查看 DeepSeek（余额 + 消费）、OpenCode（Go 三窗口 + Zen billing）与 **Qwen Token Plan（套餐模型 + 5 小时/7 天 配额窗口）** 用量，多账号。**Qwen 配额已通：官方 Bailian CLI 通道（浏览器 OAuth 一次登录）已落地并实跑成功——`llm-api-check qwen` 显示 `7天 41% · 155小时32分后重置`（2026-08-29 实测）；CLI 优先、控制台 Cookie 兜底。**

🔴 **Android 侧唯一权威 clone（2026-08-29 取证）= `~/Desktop/go-projects/pocket-llm-api-checker/`**（HEAD e1c1568，含 fastlane 元数据 + tag v1.0.0 + scripts/ 可复现构建 + docs/fdroid 草稿）。`~/Desktop/android-projects/api-checkers/` 是落后一提交的旧副本（HEAD cec6ef7，无 fastlane、无 tag），只做历史参考，**勿在其上开发**。

**公开 repo：https://github.com/xieguaiwu/llm-api-check（PUBLIC）**
发布物：GitHub Release v1.1.0（linux amd64/arm64 + darwin amd64/arm64 tarball + sha256sums.txt）

## 最后一次完成的工作（2026-08-29 13:50）
- **provider=qwen 配额 CLI 通道（v1.2.0）**：官方 Bailian CLI（npm `bailian-cli`）接入，配额终于可查
  - 安装：`npm install -g --prefix ~/.local/share/bailian-cli bailian-cli`（**bin 用 `bailian` 名，避免与本机自研翻译 CLI `bl` 冲突**；~/.local/share/bailian-cli/bin/bailian，v1.18.1）
  - 登录：`bailian auth login --console` 浏览器 OAuth 一次 → `~/.bailian/config.json`（0600）；**API key 登录解不开配额**（`usage token-plan` 认证模式 Console，实测未登录报 `No console access token found`）
  - 代码：新 `internal/repo/qwen_cli.go`（argv 数组 + 20s 超时 + stdout/stderr 分离过滤 Node 噪音 + JSON 错误信封识别）；`QwenRepo.CLI` 注入点（nil=禁用，测试 hermetic）；`Usage()` 通道优先级 **CLI → Cookie**；探测顺序 env `LLM_API_CHECK_BL_BIN`（显式指定不可执行则报错）→ 独立安装位 → PATH `bailian`（不查 `bl`）；`LLM_API_CHECK_QWEN_CLI=off` 禁用
  - app/render：`CLIEnabled` 进 QwenResult（json:"-"）；灰字提示改为「需控制台 Cookie 或 Bailian CLI（bailian auth login --console）」
  - 测试：repo 包 +10 用例（TestHelperProcess 模式、CLI 优先/Cookie 兜底/单窗口/噪音过滤/探测），main 包 +1 端到端（fake CLI 脚本）；`go test ./... -race -count=1` 7 包全绿、`go vet` 干净
  - 实跑：`llm-api-check qwen` → `7天 [████░░░░░░] 41% · 155小时32分后重置`（5 小时字段缺席——官方「5 小时限额限时取消」，解析器单窗口独立判有已覆盖）
  - 已部署 `~/.local/bin/llm-api-check`（v1.2.0）；README/README_zh/计划文档同步

## 遗留问题 / 待办
- [ ] **Bailian CLI 会话过期维护**：`~/.bailian/config.json` 会话过期后 `llm-api-check qwen` 会报 `No console access token found` → 重跑 `~/.local/share/bailian-cli/bin/bailian auth login --console`（浏览器 OAuth）。暂未做自动检测提示的差异化文案（现在是通用「调用失败」路径，2026-08-29 实测 exit 3 信封已识别并提示登录）
- [ ] **国际区域（ap-southeast-1）无凭据、未实跑**：代码按公开契约实现（`QwenEndpointsFor` + CLI `--console-site international`，CLI 通道单测覆盖了 intl 参数）；启用前先验证 `/tool/user/info.json` 的 sec_token 字段名
- [ ] **DeepSeek 平台 token / workspace ID + auth cookie 未配置**（config.fish 无）：DeepSeek 消费明细与 Zen billing 不可用。平台 token 需浏览器登录 platform.deepseek.com 后从 DevTools 抓；Zen 需 opencode.ai 的 workspace ID + auth cookie（`Fe26.2` 开头）
- [ ] songjieshi/xieguaiwu 的 key 来自 config.fish 注释行（非当前生效），可能已过期（xieguaiwu 实测 M 100% 已限流属正常用量而非 key 失效）
- [ ] P3 未修（非阻塞）：NewID panic 改返回错误、writeJSON stderr 注入、promptTTY bufio.Reader 复用、`--json --version` 文本输出；Qwen 额度绝对值（quota-config 接口的 `five_hour`/`weekly` credits）未接入，现只显示百分比（CodexBar 已接 quota-config，可参考）
- [ ] Zen billing 解析依赖 opencode.ai 页面结构，改版需更新 `internal/parsers/parsers.go` 的 ParseZenBilling；Qwen 同理依赖百炼控制台 RPC（信封形状变化时改 `qwenFindObject` 目标键）或 bailian-cli 输出（字段变化时改 `qwenCLIErrorEnvelope`/`ParseQwenUsage`）
- [ ] 图形知识图谱 graphify-out/ 未生成（可选，`graphify update . --no-llm`）

## 技术要点（下一位 Agent 必读）
- **数据源**：Go usage = `GET https://opencode.ai/zen/go/v1/usage`（API key）；Zen billing = `GET https://opencode.ai/workspace/{id}/billing`（cookie，SolidJS SSR HTML，锚点 `customerID:"cus_`，balance 单位 1e-8 USD）；DeepSeek 余额 = `api.deepseek.com/user/balance`（API key，金额为字符串）；消费 = `platform.deepseek.com/api/v0/usage/cost?month=&year=`（浏览器 token，code 40003 = 失效，拉本月+上月聚合 30 天）；**Qwen 模型 = `https://token-plan.<region>.maas.aliyuncs.com/compatible-mode/v1/models`（API key）；Qwen 配额 = Bailian CLI `bailian usage token-plan --output json`（Console 认证，首选）或 `POST https://bailian-cs.console.aliyun.com/data/api.json`（Cookie + sec_token，信封 `data.DataV2.data.data` 或内嵌 JSON 字符串）**
- **Bailian CLI 通道铁律**：① 本机 `bl` 是用户自研翻译 CLI——探测与文档一律用 `bailian` 名/绝对路径，`exec.LookPath("bl")` 是被禁止的（会误调）；② `usage token-plan` 认证模式 Console，API key 无效；③ Node 的 UNDICI 警告在 stderr，必须过滤；④ exit 非零时 JSON 错误信封在 stderr（实测 exit 3）；⑤ CLI 会话存 `~/.bailian/config.json`（敏感文件）
- **Qwen 三个不能踩的坑**（详见 docs/plans/2026-08-29-qwen-provider.md）：① `cornerstoneParam` 绝不得硬编码 `switchAgent`（网关会绑死该工作区 → 他人账号全部 NotAuthorised）② 抓 `SEC_TOKEN` 必须带 `Sec-Fetch-*` 浏览器导航头 + 桌面 UA，否则 OneConsole shell 不渲染该 token ③ 登录失效仍回 HTTP 200，错误在信封 `data.errorCode` 里，不能只看状态码
- **关键移植点**：ZenBilling 字符串字面量感知括号匹配（inStr/esc 状态机）、`(?:^|,)balance:` 正则、microcents÷1e8、cost 两月都无数据显式失败（不返回误导零数据）；QwenUsage 信封 BFS + 内嵌 JSON 字符串展开（深度上限 12）、`qwenPercent` 比例/百分数双域判定（>2 才当百分数）、空窗口重试 3 次而认证类错误不重试、CLI 单窗口响应独立判有（5 小时限时取消期间字段缺席）
- **错误消息**与 Android 版逐字一致（对照表在 docs/plans/2026-08-18-llm-api-check-cli.md「错误消息对照表」节；Qwen 新错文见 docs/plans/2026-08-29-qwen-provider.md）
- **安全**：凭据存 `~/.config/llm-api-check/config.json` chmod 0600（CLI 无 Keystore，文件权限替代 + 权限过宽警告）；凭据来源顺序 flag → 环境变量（`LLM_API_CHECK_*`）→ TTY 提示；`--json` 输出全部掩码凭据；Qwen 控制台 Cookie 含阿里云登录会话，当敏感文件对待；Bailian CLI 会话在 `~/.bailian/config.json`
- **构建**：`go build -trimpath -ldflags="-s -w -X main.version=1.2.0" -o ~/.local/bin/llm-api-check .`；测试 `go test ./... -race`（7 包）；发版打包 `VERSION=x.y.z scripts/build-dist.sh`（四平台 tarball + sha256sums.txt → dist/，dist/ 不入库）
- **渲染**：中文输出、ANSI 颜色（NO_COLOR/--no-color 禁用）、用量条 10 格、倒计时「4小时20分后重置 / 52分钟后重置 / 即将重置」、颜色阈值 <70 蓝 / 70-89 黄 / ≥90 红、限流与配额用尽强制红且**「已限流」徽章与重置倒计时必须并存**（index.md §六 项目专属要求）
- **本机环境**：密钥真值在 `~/.config/fish/config.fish`（非 dotfiles 符号链接、不入库）；该文件首行有 `if not status is-interactive; return; end` 守卫，`fish -c 'source …'` 取值会静默得空值——用 python 正则直读文件（见 System_Fix/dotfiles-sync-and-audit.md 附录 B.5）；订阅密钥与区域强绑定（北京 key 打新加坡端点 401，同 key 换区域即 200）；Bailian CLI 已装 `~/.local/share/bailian-cli/bin/bailian`（独立 prefix，不 shadow 自研 bl）

## 历史工作记录
- **2026-08-18 10:0x（c694d24）**：限流时限直接可见——对照 Android DetailScreen.WindowRow，rate-limited 行由「已限流」替代倒计时改为「已限流 · N小时M分后重置」并存；render_test 断言同步反转（限流行必须含倒计时）。实测 xieguaiwu Monthly → `已限流 · 175小时7分后重置`
- **2026-08-18 01:55**：发布公开 repo（System_Fix → dotfiles-sync-and-audit 审计：历史全量扫描真实 key 前缀零命中；fixture 为 TEST 占位符；config.json/.env 未入库）→ `gh repo create llm-api-check --public`；真实冒烟 4 账号通过；修复总览页已限流嵌套 ANSI 颜色（f2dbf22）
- **2026-08-18 01:40**：CLI 全功能实现（models/parsers/repo/config/app/render 六包 + main.go，零第三方依赖）；momus 审查修复（P1×3 + P2×4 + 8 个命令级测试）；v1.0.0 部署与双语 README

## 知识图谱
- graphify-out/: 不存在（可选生成）

## 最后更新时间
2026-08-29 13:52
