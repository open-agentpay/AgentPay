# 规范总览

AgentPay 的协议、状态机、数据模型与安全设计都以规范文档形式定义，并配有跨语言测试向量（Python 与 Go 运行同一组向量）。

::: info 单一事实来源
本站规范由仓库根目录 [`specs/`](https://github.com/july-whj/AgentPay/tree/main/specs) 自动同步生成。
机器可读的 JSON Schema 与测试向量在 [`specs/conformance`](https://github.com/july-whj/AgentPay/tree/main/specs/conformance) 与 [`specs/test-vectors`](https://github.com/july-whj/AgentPay/tree/main/specs/test-vectors)。
:::

| 主题 | 说明 |
|---|---|
| [支付状态机](./state-machine) | 意图 / 结算 / 履约三态与合法迁移 |
| [钱包与账本](./wallet-ledger) | Hold / Capture / Release、只追加账本、对账 |
| [协议一致性](./protocol-consistency) | 头部、错误码、双轨支付裁定 |
| [配置发布](./config-publishing) | 草稿、快照、RCU 热切换、回滚 |
| [审计事件](./audit-events) | 事件信封、脱敏、保留与导出 |
| [Agent 身份与行为审计](./agent-audit) | 身份来源、观测 vs 自报、停用 |
| [多租户数据模型](./multitenant-data-model) | 实体与表结构 |
| [安全模型](./security-model) | 信任边界与防御范围 |
