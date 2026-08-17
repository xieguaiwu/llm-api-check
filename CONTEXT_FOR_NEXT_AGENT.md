# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态
llm-api-check — Go CLI，复刻 Android app「API Checkers」（现名 pocket-llm-api-checker）的数据层逻辑：查看 DeepSeek（余额+消费）与 OpenCode（Go 三窗口 + Zen billing）用量，多账号。**开发完成 + 真实凭据冒烟通过 + 已发布公开 GitHub，已部署 `~/.local/bin/llm-api-check`。**

**公开 repo：https://github.com/xieguaiwu/llm-api-check（PUBLIC，13 commits，发布前按 System_Fix git 审计：历史零真实 key）**

## 最后一次完成的工作（2026-08-18 01:55）
- **发布公开 repo（01:55）**：System_Fix → dotfiles-sync-and-audit 流程审计（历史全量扫描真实 key 前缀 sk-uZY/sk-2a37/sk-rGB/sk-vrIY/nvapi- 零命中；Fe26.2 命中均为 README 格式示例；fixture 为 TEST 占位符；config.json/.env 未入库；工作区干净）→ gh repo create llm-api-check --public → push 完成（main 同步 2b55ca2）
- **真实冒烟完成（01:26）**：从 ~/.config/fish/config.fish 提取 4 个 key 配置 4 个账号并验证真实 API：DeepSeek 主号（余额 ¥62.45）、opencode 主号（M 70% 黄）、xieguaiwu（M 100% 已限流红）、songjieshi（W 77% 黄）。直连无代理成功。
- 修复总览页已限流嵌套 ANSI 颜色（f2dbf22）

## 最后一次完成的工作（2026-08-18 01:40）
- CLI 全功能实现：models/parsers/repo/config/app/render 六包 + main.go，零第三方依赖
- **momus 审查修复（61b1e4b）**：P1-1 Cost 月末溢出（AddDate 归一化 → day=1 构造，补 5 回归用例）、P1-2 isTTY ioctl TCGETS（/dev/null 不误判）、P1-3 --json 凭据掩码（首尾4+****）、P2-2 rune 截断、P2-3 warning 快照、P2-4 warnSecurity no-color、P2-5 rawFloat 字符串金额；P3 filter 注释/README 版本/accounts list 警告
- main_test.go 新增 8 个命令级测试（--json 合法性/退出码/凭据掩码/非 TTY 降级/EOF 跳过/NO_COLOR）
- 测试全绿（7 包，含 -race）；`go vet` 干净
- 部署：`~/.local/bin/llm-api-check`（--version = 1.0.0）
- 文档：README.md（英文）+ README_zh.md（中文）双语互链、CONTEXT 本文档
- Git：10 个 commit

## 遗留问题 / 待办
- [ ] **DeepSeek 平台 token / workspace ID + auth cookie 未配置**（config.fish 无）：DeepSeek 消费明细与 Zen billing 不可用。平台 token 需浏览器登录 platform.deepseek.com 后从 DevTools 抓；Zen 需 opencode.ai 的 workspace ID + auth cookie（Fe26.2 开头）
- [ ] songjieshi/xieguaiwu 的 key 来自 config.fish 注释行（非当前生效），key 可能已过期（xieguaiwu 实测 M 100% 已限流为正常用量而非 key 失效）
- [ ] P3 未修（非阻塞）：NewID panic 改返回错误、writeJSON stderr 注入、promptTTY bufio.Reader 复用、`--json --version` 文本输出
- [ ] Zen billing 解析依赖 opencode.ai 页面结构，改版需更新 `internal/parsers/parsers.go` 的 ParseZenBilling
- [ ] 图形知识图谱 graphify-out/ 未生成（可选，`graphify update . --no-llm`）

## 技术要点（下一位 Agent 必读）
- **数据源**：Go usage = `GET https://opencode.ai/zen/go/v1/usage`（API key）；Zen billing = `GET https://opencode.ai/workspace/{id}/billing`（cookie，SolidJS SSR HTML，锚点 `customerID:"cus_`，balance 单位 1e-8 USD）；DeepSeek 余额 = `api.deepseek.com/user/balance`（API key，金额为字符串）；消费 = `platform.deepseek.com/api/v0/usage/cost?month=&year=`（浏览器 token，code 40003 = 失效，拉本月+上月聚合 30 天）
- **关键移植点**：ZenBilling 字符串字面量感知括号匹配（inStr/esc 状态机）、`(?:^|,)balance:` 正则、microcents÷1e8、cost 两月都无数据显式失败（不返回误导零数据）
- **错误消息**与 Android 版逐字一致（对照表在 docs/plans/2026-08-18-llm-api-check-cli.md「错误消息对照表」节）
- **安全**：凭据存 `~/.config/llm-api-check/config.json` chmod 0600（CLI 无 Keystore，文件权限替代 + 权限过宽警告）；凭据来源顺序 flag → 环境变量（LLM_API_CHECK_*）→ TTY 提示
- **构建**：`go build -trimpath -ldflags="-s -w" -o ~/.local/bin/llm-api-check .`；测试 `go test ./...`（6 包）
- **渲染**：中文输出、ANSI 颜色（NO_COLOR/--no-color 禁用）、用量条 10 格、倒计时「4小时20分后重置/52分钟后重置/即将重置」、颜色阈值 <70 蓝 / 70-89 黄 / ≥90 红、rate-limited 强制红

## 知识图谱
- graphify-out/: 不存在（可选生成）

## 最后更新时间
2026-08-18 01:20
