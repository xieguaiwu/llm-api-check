# CONTEXT_FOR_NEXT_AGENT.md

## 项目当前状态
llm-api-check — Go CLI，复刻 Android app「API Checkers」（现名 pocket-llm-api-checker）的数据层逻辑：查看 DeepSeek（余额+消费）与 OpenCode（Go 三窗口 + Zen billing）用量，多账号。**开发完成，已部署 `~/.local/bin/llm-api-check`，待真实凭据冒烟测试。**

## 最后一次完成的工作（2026-08-18 01:20）
- CLI 全功能实现：models/parsers/repo/config/app/render 六包 + main.go，零第三方依赖
- 修复 2 个 main.go 问题：①`--json` 空列表输出 null → 改为 `[]`、零值时间戳 → 0（sliceOrEmpty/unixMillisOrZero）②非 TTY 下 accounts add 可选字段 EOF 报错 → 改为跳过；deepseek/opencode 补 `--no-refresh` flag
- 测试全绿（6 包 ok，63 个测试断言）；`go vet` 通过
- 部署：`~/.local/bin/llm-api-check`（6.4MB，`--version` = 1.0.0）
- 文档：README.md（英文）+ README_zh.md（中文）双语互链、CONTEXT 本文档
- Git：7 个 commit（脚手架→模型→解析器→仓库→配置→编排→CLI+渲染→文档待提交）

## 遗留问题 / 待办
- [ ] **真实凭据冒烟测试**：用户需提供 DeepSeek API key / 平台 token / opencode-go 三账号的 Go key + workspace ID + auth cookie（`llm-api-check accounts add` 输入），然后 `llm-api-check` 验证真实 API 响应
- [ ] 交互提示在非 TTY 下会打印可选字段提示行后直接跳过（行为正确，观感可优化为 TTY 检测前置）
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
