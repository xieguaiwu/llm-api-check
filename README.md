# llm-api-check

查看 DeepSeek API（余额 + 消费明细）与 OpenCode（Go usage 三窗口 + Zen billing）使用情况的终端 CLI，支持多账号。

> **状态**：规划阶段 — 实现计划见 [docs/plans/2026-08-18-llm-api-check-cli.md](docs/plans/2026-08-18-llm-api-check-cli.md)。CLI 代码将按计划实现后提交到本仓库。

## 目标

复刻 Android app「API Checkers」的数据层逻辑，不复制 UI，输出为终端文本/JSON：

- DeepSeek：余额 + 消费明细
- OpenCode：Go usage 三窗口 + Zen billing
- 多账号支持
- 零第三方依赖（Go stdlib only），配置 `~/.config/llm-api-check/config.json`（chmod 0600）

## 技术栈

- Go ≥ 1.22，stdlib only（`net/http` / `encoding/json` / `regexp` / `testing`）
- 命令式 CLI：`llm-api-check` 显示总览，子命令管理账号
- 所有网络超时 15s；HTTP 401/403 返回中文错误提示

## 许可证

MIT
