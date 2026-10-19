# AgentPay 内部研发文档中心 (Internal Docs)

> ⚠️ **注意**：本目录（`docs/`）为内部研发规划、技术方案及敏捷计划文档，已在 `.gitignore` 中配置排除，**不向 GitHub 公开同步**。

---

## 📚 文档索引

1. **[mvp-design.md](./mvp-design.md) - MVP 版本系统设计文档**
   * MVP 目标与范围界定
   * 极简端到端（Buyer + Seller + Mock Facilitator）架构拓扑
   * 核心模块接口与时序设计
   * 关键技术选型与实现细节

2. **[api-specs.md](./api-specs.md) - 核心接口规范文档**
   * x402 V2 HTTP 协议头（Challenge 与 Signature）全规范与 Payload 契约
   * Facilitator 协调清算服务 REST API 规范（`/v1/verify`、`/v1/settle` 等）
   * Buyer Agent Client SDK 方法签名与事件机制
   * Seller Middleware 配置与拦截器规范

3. **[development-plan-2026.md](./development-plan-2026.md) - 2026 年底研发计划（★ 当前研发执行唯一基准）**
   * **研发周期**：2026-10-17 至 2026-12-31（每周 10–15h）
   * **年底目标**：基础流程跑通（Python 买方 + 财务防火墙 + 自托管网关 + 三态恢复 + 失控循环 Demo）
   * **周计划与 WBS**：T0.1 ~ T9.1 颗粒度拆解与逐项验收标准
   * **执行纪律**：砍项优先级、无大班车原则、风险应对预案

4. **[development-plan.md](./development-plan.md) - 整体中长期研发规划（参考归档）**
   * 双栈中长期路线图与演进规划（Java、PostgreSQL、分布式集群等 2027 事项）

5. **[product-planning.md](./product-planning.md) - 技术方案与产品规划（★ 长期战略与终局愿景）**
   * **核心定位**：AI Agent 的 Financial Firewall（金融防火墙）与自主支付基础设施
   * **战略五阶段演进**：Phase 0 协议原型 → Phase 1 金融防火墙 → Phase 2 Agent 生态（MCP Paid Tool） → Phase 3 企业控制面 → Phase 4 Agent Wallet 资金账户 → Phase 5 A2A 商业网络
   * **商业模式与护城河**：开源防火墙内核 + SaaS 云端控制面 + 商业化 Facilitator 清算网络 + 行为信用评分壁垒
   * **安全六公理与威胁模型**：防 Prompt 注入、防递归死循环扣费、零信任预算原子预留与反向网关设计

6. **[development-tasks-v1.1.md](./development-tasks-v1.1.md) - AgentPay 研发任务清单 v1.1（★ 最新研发执行总表）**
   * **升级说明**：纳入平台内部虚拟钱包（T1.7、T2.8、T3.6~3.8、T4.6、T5.7、T6.7），确立双轨结算架构
   * **当前状态**：T0.1、T0.2、T1.1、T1.2、T1.3、T1.4、T1.7 全部完成并经过全量自动化测试验证；当前推进 T1.5 审计契约

7. **[baseline-freeze-v0.1.0.md](./baseline-freeze-v0.1.0.md) - T0.2 行为基线冻结记录**
   * **基线 Commit**：`338fc0a`
   * **Git Tag**：`v0.1.0-poc-baseline`
   * **测试证据**：全量 340 项自动化测试 100% 通过、跨语言测试向量验证无误

8. **[specs/wallet-ledger-spec.md](../specs/wallet-ledger-spec.md) - T1.7 内部虚拟钱包与账本规范**
   * **核心机制**：只追加不可变账本、租户级钱包、Hold 冻结/确认/释放闭环、五重对账平账
   * **拍定决策**：W1 租户级归属、W2 USDC 整数最小单位、v1 不支持提现、W6 欠费冻结

9. **[specs/multitenant-data-model.md](../specs/multitenant-data-model.md) - T1.2 多租户逻辑数据模型规范**
   * **覆盖范围**：租户/项目/RBAC、路由/策略、虚拟钱包/账本、支付/结算/履约、请求追踪/用量审计五大域（17 张核心表）
   * **数据底座**：PostgreSQL 15+ 权威 DDL（`infra/postgres/schema.sql`）与 `docker-compose.yml` 容器配置
   * **选型决策**：[ADR-002: 数据库选型全面升级至 PostgreSQL](../specs/adr-002-postgresql-database-selection.md)

10. **[specs/protocol-consistency-spec.md](../specs/protocol-consistency-spec.md) - T1.3 Python/Go 协议一致性契约**
    * **核心规范**：x402 V2 挑战/签名 Base64URL 规范、双轨支付裁定流（余额不足严禁自动降级）
    * **测试向量**：17 项标准错误码（`specs/conformance/protocol-errors.json`）与跨语言测试套件（`specs/test-vectors/dual-path-protocol.json`）

11. **[specs/config-publishing-spec.md](../specs/config-publishing-spec.md) - T1.4 配置发布与运行时缓存契约**
    * **核心规范**：配置生命周期状态机、不可变快照哈希指纹（`snapshotHash`）、微秒级 RCU 原子热替换、控制面宕机降级容灾
    * **测试向量**：快照 Schema（`specs/conformance/config-snapshot.schema.json`）与回滚测试用例（`specs/test-vectors/config-publishing.json`）

