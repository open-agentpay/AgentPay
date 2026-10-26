# 规范总览

AgentPay 的协议、状态机、数据模型与安全设计都以规范文档形式定义，并配有跨语言测试向量（Python 与 Go 运行同一组向量）。

::: info 单一事实来源
本站规范由仓库根目录 [`specs/`](https://github.com/open-agentpay/AgentPay/tree/main/specs) 自动同步生成。
机器可读的 JSON Schema 与测试向量在 [`specs/conformance`](https://github.com/open-agentpay/AgentPay/tree/main/specs/conformance) 与 [`specs/test-vectors`](https://github.com/open-agentpay/AgentPay/tree/main/specs/test-vectors)。
:::

## 核心契约与架构规范

| 主题 | 说明 |
|---|---|
| [支付状态机](./state-machine) | 意图 / 结算 / 履约三态正交状态机与合法迁移 |
| [钱包与账本](./wallet-ledger) | Hold / Capture / Release、只追加复式账本与自动对账 |
| [协议一致性](./protocol-consistency) | 请求标头、统一错误码信封与双轨支付裁定 |
| [配置发布](./config-publishing) | 草稿校验、不可变快照发布、RCU 内存热切换与回滚 |
| [审计事件](./audit-events) | 事件信封、敏感信息脱敏、WORM 保留与 CSV/JSON 导出 |
| [Agent 身份与行为审计](./agent-audit) | 密码学身份提取、网关观测 vs 客户端自报维度、紧急熔断停用 |
| [多租户数据模型](./multitenant-data-model) | PostgreSQL 16 表结构、行级租户隔离与外键拓扑 |
| [系统架构规范](./architecture) | 控制面与数据面双平面架构、组件边界与核心数据流 |
| [Gas 与结算模型](./gas-models) | EIP-3009 免 Gas 签名授权经济学与 Facilitator 结算模型 |
| [安全模型](./security-model) | 信任边界、STRIDE 威胁目录与金融防火墙缓解措施 |

## 架构决策记录 (ADR)

| 编号 | 决策摘要 |
|---|---|
| [ADR-001: x402 V2 协议与官方 SDK 架构策略](./adr/001) | 分层适配架构 + 原生 Pydantic V2 防火墙核心编排 |
| [ADR-002: 数据库选型全面升级至 PostgreSQL](./adr/002) | 从 Phase 0 单机 SQLite 升级至 PostgreSQL 16 行级锁与 `JSONB` 快照存储 |

## 一致性与安全测试报告

| 报告名称 | 覆盖范围说明 |
|---|---|
| [Python / Go 协议一致性测试报告](./reports/protocol-conformance-report) | 19 项标准错误码、双轨协议向量、35 条状态机迁移与进程级 HTTP 互操作验证 |
| [多租户隔离与越权渗透测试报告](./reports/tenant-isolation-test-report) | 水平 IDOR、垂直 RBAC 提权、路径载荷混淆、RCU 路由与虚拟钱包账本隔离验证 |
