# AgentPay 研发任务清单 v1.1

- **版本**：v1.1（在 v1.0 基础上新增平台内部虚拟钱包与 T0 状态）
- **规划日期**：2026-10-18
- **用途**：将《AgentPay 后续研发计划：多租户支付网关与管理平台》拆分为可执行、可验收的任务。
- **任务编号规则**：`T阶段.序号`，例如 `T0.1`。
- **当前状态约定**：`待开始` / `进行中` / `阻塞中` / `已完成` / `已取消`。
- **当前已知状态**：核心代码与链上支付验证已完成；T0（CI 修复、基线冻结）已完成；T1.1（统一支付状态机）与 T1.7（钱包与账本契约）已完成正式规范与测试向量。当前全面推进 **T1.2 设计多租户逻辑数据模型**。

## v1.1 变更摘要

- **新增 T0 章节**：CI 修复与基线冻结，均已完成并打 Git Tag `v0.1.0-poc-baseline`。
- **新增任务**：T1.7、T2.8、T3.6、T3.7、T3.8、T4.6、T5.7、T6.7（平台内部虚拟钱包相关）。
- **修改任务**：T1.1、T1.2、T1.3、T1.5、T1.6、T3.4、T3.5、T5.2、T5.5、T6.2、T6.3。
- **状态更新**：T0（T0.1, T0.2）与 T1（T1.1, T1.2, T1.3, T1.4, T1.5, T1.6, T1.7）全部完成并 100% 通过自动化契约测试。
- **依赖与执行顺序**：已更新，并收敛核心决策 W1–W6。
- **充值渠道分阶段**：v1 管理员发放（T3.6）；v1.x 链上充值（T3.7）；之后法币充值（T3.8）。

---

## T0 — 前置：CI 与基线（已完成）

### T0.1 修复 CI
- **优先级**：P0
- **状态**：已完成（2026-10-18 确认）
- **任务内容**
  - [x] 修复 CI 失败问题（Python 3.10 类型兼容性）。
- **验收标准**
  - [x] CI 通过。
- **验收证据**：`make check` 全绿，所有测试通过。
- **交付物**：可通过的 CI 流水线。

### T0.2 冻结当前基线
- **优先级**：P0
- **状态**：已完成（2026-10-18 确认）
- **任务内容**
  - [x] 固化当前已验证的行为（含链上支付验证）作为回归基线。
- **验收标准**
  - [x] 后续改动可对照该基线回归。
- **验收证据**：Git 标签 `v0.1.0-poc-baseline`（Commit `338fc0a`），详见 `docs/baseline-freeze-v0.1.0.md`。
- **交付物**：行为基线与冻结报告。

---

## T1 — 协议、状态机与多租户数据契约

### T1.1 统一支付状态机

- **优先级**：P0
- **状态**：已完成（2026-10-18 确认）
- **任务内容**
  - [x] 明确支付意图状态（created, challenged, authorized, rejected, expired, completed, failed, needs_review）。
  - [x] 明确结算状态（not_started, held, submitted, pending, confirmed, released, failed, unknown）。
  - [x] 明确上游履约状态（not_started, in_progress, succeeded, failed, unknown）。
  - [x] 定义各状态的合法迁移和禁止迁移。
  - [x] 明确超时、未知结果、进程重启和恢复流程。
  - [x] 增加支付方式维度（链上 / 钱包双轨制），并入钱包的冻结、确认、释放流程。
  - [x] 确认 D1：链上路径采用方案 A（先结算后履约）；钱包路径采用冻结 → 履约 → 确认扣款/释放。
- **验收标准**
  - [x] 状态迁移规则有文档（`specs/payment-state-machine-v2.md`）。
  - [x] 支付状态与履约状态分离。
  - [x] 未知结算状态不会被直接当作成功或失败。
  - [x] 钱包路径的冻结不会永久占用余额，履约结果未知时不自动扣款或释放（转 needs_review）。
- **交付物**：状态机规范（`specs/payment-state-machine-v2.md`）、机器可读迁移表（`specs/conformance/state-machine.json`）与表驱动测试套件（`python/tests/unit/test_state_machine_matrix.py`，11 项测试全部通过）。

### T1.2 设计多租户逻辑数据模型
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 定义 Tenant、Project、Member、Role。
  - [x] 定义 ApiKey、Route、Policy、ConfigVersion。
  - [x] 定义 RequestRecord、UsageEvent、PaymentIntent。
  - [x] 定义 SettlementRecord、FulfillmentRecord、AuditEvent。
  - [x] 定义 Wallet、LedgerEntry、Hold、TopUpOrder。
  - [x] 明确实体归属、主键和关联 ID（统一采用 `ten_`, `prj_`, `wal_` 等前缀标识符）。
  - [x] 评估数据库选型与数据迁移策略（已产出 ADR-002，正式采纳 PostgreSQL 15+ 替代单机 SQLite）。
- **验收标准**
  - [x] 核心实体、归属关系和关联方式有书面定义（`specs/multitenant-data-model.md`）。
  - [x] 数据库选型有决策记录（`specs/adr-002-postgresql-database-selection.md`）。
  - [x] 钱包相关实体、余额定义和归属关系有书面定义（17 张核心表 DDL 已固化于 `infra/postgres/schema.sql`）。
- **交付物**：多租户逻辑数据模型（`specs/multitenant-data-model.md`）、数据库选型 ADR（`specs/adr-002-postgresql-database-selection.md`）与完整 PostgreSQL DDL（`infra/postgres/schema.sql`）。

### T1.3 定义 Python/Go 协议一致性契约
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 确认 x402 版本与挑战格式（`PAYMENT-REQUIRED` 与 RFC 6648 兼容 Base64URL 载荷）。
  - [x] 明确签名、验证和支付请求的输入输出（EIP-712/3009 授权转账载荷）。
  - [x] 统一错误码和错误响应（17 项全量错误码，含 `ERR_WALLET_INSUFFICIENT_BALANCE` 等，统一 JSON 格式）。
  - [x] 统一请求 ID、幂等键、支付意图 ID、结算 ID 和履约 ID 的规则（统一前缀与强幂等校验）。
  - [x] 建立跨语言测试向量（`specs/test-vectors/dual-path-protocol.json` 与 `specs/conformance/protocol-errors.json`）。
  - [x] 明确钱包支付（API Key）与链上支付（x402 载荷）的选择规则，**余额不足绝对不自动降级到链上**。
- **验收标准**
  - [x] Python 与 Go 可以运行同一组测试向量（自动化测试 `python/tests/unit/test_protocol_conformance_vectors.py` 验证通过）。
- **交付物**：协议规范（`specs/protocol-consistency-spec.md`）、标准错误码表（`specs/conformance/protocol-errors.json`）、跨语言测试向量（`specs/test-vectors/dual-path-protocol.json`）与自动化测试套件。

### T1.4 定义配置发布契约
- **优先级**：P1
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 定义配置草稿、校验、发布、生效和回滚状态（draft, validated, active, superseded, rolled_back）。
  - [x] 定义配置版本号和不可变快照（递增版本号与 SHA-256 确定性内容指纹）。
  - [x] 定义 Gateway 获取、缓存和刷新配置的机制（Push + Pull 混合刷新与 RCU 零停机原子指针替换）。
  - [x] 定义配置缺失、过期或控制面不可用时的安全行为（本地内存/磁盘镜像继续服务，默认拒绝防穿透）。
- **验收标准**
  - [x] 配置生命周期和运行时行为有明确规范（`specs/config-publishing-spec.md`）。
  - [x] 自动化测试通过（`python/tests/unit/test_config_publishing.py` 6 项测试全部通过）。
- **交付物**：配置发布协议规范（`specs/config-publishing-spec.md`）、快照 Schema（`specs/conformance/config-snapshot.schema.json`）、跨语言测试向量（`specs/test-vectors/config-publishing.json`）与自动化测试。

### T1.5 定义消费审计事件契约
- **优先级**：P1
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 定义请求、消费、支付、结算、履约和管理审计事件。
  - [x] 定义充值、钱包流水、冻结/确认/释放和人工调账事件。
  - [x] 定义事件 ID 与关联 ID（`evt_`, `req_`, `pi_`, `set_`, `ful_`, `wlt_`, `hld_`, `led_`, `top_`）。
  - [x] 定义事件幂等写入和重复事件处理规则（`ON CONFLICT (audit_id) DO NOTHING`）。
  - [x] 定义敏感字段脱敏、数据保留和导出规则（API Key 仅记录 Hash 与 Prefix，body_sha256 指纹，Hot 90d / Warm 1y / Cold 7y 分层）。
- **验收标准**
  - [x] 每类事件的字段、关联方式和写入规则明确。
  - [x] 自动化测试通过（`python/tests/unit/test_audit_events.py` 8 项测试全部通过）。
- **交付物**：审计事件规范（`specs/audit-events-spec.md`）、事件信封 Schema（`specs/conformance/audit-events.schema.json`）、测试向量（`specs/test-vectors/audit-events.json`）与自动化测试。

### T1.6 完成威胁模型
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 标记控制面、数据面、上游服务和支付组件的信任边界（5 层边界矩阵与 STRIDE 分析）。
  - [x] 识别租户越权、密钥泄漏、重复支付和配置错误等风险。
  - [x] 识别钱包相关风险：并发双花（W-T1）、负余额（W-T2）、越权调账（W-T3）、充值重放（W-T4）、链上重组回滚（W-T5）、冻结泄漏（W-T6）与严禁自动降级（W-T7）。
  - [x] 为主要风险定义缓解措施和验证方式（Fail-Closed、行级锁、CAS 状态机、多重确认数）。
- **验收标准**
  - [x] 关键威胁、边界和测试方法有文档记录。
  - [x] 核心安全红线与不可变量明确收敛。
- **交付物**：全面威胁模型 v1.0（`specs/threat-model-v1.md`）。

### T1.7 定义钱包与账本契约
- **优先级**：P0
- **状态**：已完成（2026-10-18 确认）
- **任务内容**
  - [x] 定义 Wallet、LedgerEntry（只追加）、Hold、TopUpOrder。
  - [x] 定义余额口径（available / held）和对账基准（以账本为准）。
  - [x] 定义冻结、确认、释放及到期规则。
  - [x] 定义幂等键与并发控制方式（行锁 / 乐观锁条件更新）。
  - [x] 定义三种充值渠道（管理员发放、链上充值、法币）的统一抽象。
  - [x] 定义对账规则（五重对账平账体系）。
  - [x] 确认钱包决策 W1–W6（W1 租户级、W2 整数最小单位 USDC 单一资产、W3 TTL、W4 履约未知保持冻结、W5 不自动降级、W6 欠费冻结）。
- **验收标准**
  - [x] 账本不变量有书面定义（余额不为负、流水只追加、金额为整数、幂等）。
  - [x] 冻结、确认、释放迁移可被测试。
  - [x] W1–W6 有决策记录，并已合入测试与状态机。
- **交付物**：钱包与账本规范（`specs/wallet-ledger-spec.md`）。

---

## T2 — Go Gateway

### T2.1 建立 Go Gateway 项目骨架
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 建立项目结构和构建流程（`gateway-go/` 标准分层架构，集成根目录 `Makefile` 的 `gateway-build` 与 `gateway-test`）。
  - [x] 增加配置加载（`internal/config`，支持 YAML 规范加载、环境变量覆盖与边界强校验）。
  - [x] 增加结构化日志（`internal/logger`，基于 Go 1.21 `log/slog`，支持 JSON/Text 格式与敏感凭证脱敏）。
  - [x] 增加健康检查和基础指标（`/healthz` 存活探针、`/readyz` 就绪探针、`/metrics` Prometheus 文本格式导出、`/version` 编译元数据）。
  - [x] 增加 RCU 零停机配置快照缓存（`internal/rcu`，基于 `sync/atomic.Pointer` 实现微秒级无锁路由匹配与热更新）。
  - [x] 增加优雅退出（处理 SIGINT/SIGTERM 信号并支持超时上下文退出）。
- **验收标准**
  - [x] 服务可以在本地及 CI 环境构建、启动和测试（`go build ./cmd/gateway` 成功，单元测试与 E2E 集成测试 100% 通过）。
- **交付物**：Go Gateway 项目骨架（`gateway-go/` 全量源码、单元测试、E2E 测试与示例配置）。

### T2.2 实现 HTTP 反向代理
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 实现路由匹配和上游转发（`internal/proxy`，基于 `httputil.ReverseProxy`，支持路径映射、方法限制与响应改写）。
  - [x] 配置上游超时和请求大小限制（支持 per-route `timeout_ms` 与 `http.MaxBytesReader` 请求体限制，超限返回 413，超时返回 504）。
  - [x] 清理不可信请求头（严格清洗 RFC 7230 跳跃标头及 `X-Payer-*`、`X-Payment-*`、`X-AgentPay-*` 伪造身份标头，并透传标准 `X-Forwarded-*`）。
  - [x] 隔离上游 API Key 等敏感凭证（通过 `upstream_headers` 注入后端凭据，且绝不反向泄露给客户端或进入普通日志）。
- **验收标准**
  - [x] 代理行为有自动化测试（`internal/proxy/proxy_test.go` 7 项测试与 `tests/e2e_test.go` 全部通过）。
  - [x] 上游凭证不会泄漏给客户端或普通日志（已通过隔离与脱敏断言验证）。
- **交付物**：基础反向代理模块（`gateway-go/internal/proxy/` 全量源码与单元测试）。

### T2.3 实现 x402 支付处理
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 实现挑战生成与解析（`internal/x402/codecs.go`，支持 `PAYMENT-REQUIRED` 标头的 Base64URL 与 JSON 格式无损编解码）。
  - [x] 实现支付验证流程（`internal/x402/verifier.go`，支持 EIP-712/EIP-3009 签名有效性验证与 Nonce 防重放检查）。
  - [x] 实现双轨支付分流与策略执行（`internal/x402/handler.go`，支持 API Key 虚拟钱包冻结实扣与 x402 链上验签，严格遵循 W5 余额不足拒绝自动降级规则）。
  - [x] 统一支付相关错误响应（严格遵循 `specs/conformance/protocol-errors.json` 规范定义与 17 个标准化错误码）。
- **验收标准**
  - [x] 通过 T1.3 定义的协议测试向量（`internal/x402/handler_test.go` 10 项测试与 `tests/e2e_test.go` 全部通过）。
- **交付物**：x402 处理模块（`gateway-go/internal/x402/` 全量源码与单元测试）。

### T2.4 实现幂等与异常恢复
- **优先级**：P0
- **状态**：待开始
- **任务内容**
  - [ ] 实现请求关联和幂等控制。
  - [ ] 处理重复请求。
  - [ ] 处理上游 5xx 和网络超时。
  - [ ] 处理结算结果未知。
  - [ ] 实现必要的状态查询与恢复流程。
- **验收标准**
  - [ ] 故障注入测试证明不会因盲目重试造成重复支付。
- **交付物**：恢复流程及测试。

### T2.5 实现路由、价格和策略读取
- **优先级**：P0
- **状态**：待开始
- **任务内容**
  - [ ] 读取已发布的路由配置。
  - [ ] 读取定价、支付地址和策略。
  - [ ] 记录请求使用的配置版本。
- **验收标准**
  - [ ] 请求可以追溯到对应配置版本。
- **交付物**：配置读取模块。

### T2.6 完成 Python/Go 一致性测试
- **优先级**：P0
- **状态**：待开始
- **任务内容**
  - [ ] 使用相同输入执行 Python 与 Go 测试。
  - [ ] 比较支付结果、错误码、状态迁移和恢复行为。
  - [ ] 修复行为差异。
- **验收标准**
  - [ ] 约定的核心测试向量全部通过。
- **交付物**：一致性测试报告。

### T2.7 建立性能基准
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 设计可重复的负载测试。
  - [ ] 记录吞吐量、P95/P99 延迟和错误率。
  - [ ] 记录 CPU、内存和并发配置。
  - [ ] 对比 Python 与 Go 的结果。
- **验收标准**
  - [ ] 测试方法和结果可复现。
- **交付物**：性能基准报告。

### T2.8 实现钱包支付路径
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **依赖**：T1.1、T1.7、T3.3、T3.6
- **任务内容**
  - [x] 通过 API Key 识别租户，并按路由策略选择钱包支付（`gateway-go/internal/wallet/`，支持 API Key 租户解析、`allowed_payment_methods` 校验与 403 `ERR_PAYMENT_METHOD_NOT_ALLOWED` 拦截）。
  - [x] 实现“冻结 → 履约 → 确认扣款 / 释放”（基于行级悲观锁与原子状态迁移，2xx 上游自动执行 Capture 写入只追加不可变流水 `ap_ledger_entry`，4xx 自动执行 Release 恢复额度）。
  - [x] 余额不足时返回明确错误，不自动降级到链上支付（严格遵守 W5 决策，余额不足返回 402 `ERR_WALLET_INSUFFICIENT_BALANCE` 且绝对不透传 `PAYMENT-REQUIRED` 挑战标头）。
  - [x] 复用策略检查与幂等逻辑（与 `idempotencyStore` 联动，相同幂等键直接命中缓存 `X-Cache: HIT` 零重复扣款，并发冲突与参数篡改返回 409 `ERR_IDEMPOTENCY_CONFLICT`）。
  - [x] 履约结果未知时保持冻结并转人工核查；冻结到期后由恢复流程处理（严格遵守 W4 决策，上游 5xx/504 时额度维持 `held` 并标记 `needs_review`，相同幂等键重试触发 Free Replay 复用原冻结；提供 `ReleaseExpiredHolds` 超时解冻与双轨对账 `ReconcileBalance`）。
- **验收标准**
  - [x] 并发请求不会透支余额（100 个并发高争抢协程压测断言：`TestWalletConcurrencyNonNegativeOverdraft` 验证可用余额绝不透支至负数，严格无负余额）。
  - [x] 重复请求只扣费一次（`TestWalletPaymentPathIdempotencyAndFreeReplay` 证明 5xx 故障重试与 200 重放均仅产生唯单一笔金融扣费流水）。
  - [x] 故障注入后账本、结算与履约记录保持一致（双轨记账对账报告 `ReconcileBalance` 证明 `available + held` 与不可变金融账本流水 100% 平账）。
- **交付物**：钱包支付模块（`gateway-go/internal/wallet/` 支持 Memory 与 Postgres 生产持久化实现）、数据面集成（`gateway-go/internal/server/`）、并发竞态测试（`gateway-go/internal/wallet/concurrency_test.go`）、端到端集成测试（`gateway-go/tests/wallet_e2e_test.go`）。

### T2.9 Gateway Agent 级策略执行与行为审计
- **优先级**：P1
- **状态**：已完成
- **完成时间**：2026-10-18
- **依赖**：T1.8、T2.2、T2.4、T3.3
- **任务内容**
  - [x] 解析 API Key 绑定的 `agent_id`，并严格剥离/重写客户端传入的伪造 `X-Agent-ID`（`gateway-go/internal/agent/sanitizer.go`，并在 `proxy.go` 中强制清理所有入站 `x-agent-*` 标头，由网关注入可信权威 `X-Agent-ID` 透传上游）。
  - [x] 在网关请求路径内同步执行 Agent 级策略：预算检查（与钱包 Hold 联动）、单 Agent 限流（RPM/QPS）、路由白名单与单笔上限（`gateway-go/internal/agent/enforcer.go`，超限或未授权时拦截并返回 403 `ERR_POLICY_MAX_PER_TX`、`ERR_ROUTE_NOT_ALLOWED`、429 `ERR_RATE_LIMIT_EXCEEDED` 或 402 `ERR_WALLET_INSUFFICIENT_BALANCE`）。
  - [x] 实现 Agent 一键停用（Kill Switch）秒级生效通道（`gateway-go/internal/agent/killswitch.go`，结合 RCU / 内存注册表管理，立即阻断已被 suspended / revoked 的 Agent，返回 403 `ERR_AGENT_SUSPENDED` / `ERR_AGENT_REVOKED`）。
  - [x] 提取并清洗客户端声明字段（`session_id`, `task_id`, `tool_name` 等），执行 128 字符截断与白名单字符集防注入过滤。
  - [x] 产生符合 T1.8 / T1.5 规范的结构化 `agent.request`、`agent.policy_decision` 与 `agent.spend` 审计事件流（`gateway-go/internal/agent/audit.go`，区分 `observed` 权威观测数据与 `declared` 自报数据）。
- **验收标准**
  - [x] 伪造请求头无法篡改 `agent_id`（端到端测试 `TestAgentE2ESpoofedHeaderStrippingAndIdentityResolution` 验证通过）。
  - [x] Agent 停用后新请求在数秒内被拒绝，进行中事务按状态机安全收敛（测试 `TestAgentE2EKillSwitchFastPath` 验证通过）。
  - [x] Agent 预算超限或路由未授权时同步被拦截并记录明确策略拒绝事件（测试 `TestAgentE2EPolicyEnforcementAndAuditStream` 验证通过）。
- **交付物**：Gateway Agent 策略与审计发射模块（`gateway-go/internal/agent/` 全量源码、单元测试 `gateway-go/internal/agent/agent_test.go` 与端到端测试 `gateway-go/tests/agent_e2e_test.go`）。

---

## T3 — 多租户身份、权限与隔离

### T3.1 实现租户与项目管理
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 实现 Tenant 管理（`agentpay.tenancy`，支持 `ten_` 标识符、唯一命名、创建、查询与列表筛选）。
  - [x] 实现 Project 管理（`prj_` 标识符、预算硬顶 `budget_cap_base_units`、状态控制、更新与删除）。
  - [x] 定义租户和项目的启用、禁用及删除策略（支持 `active -> suspended -> active`，`terminated` 终态锁定；租户挂起/终止时级联阻断项目新建与修改；项目支持 `active <-> archived` 归档隔离）。
  - [x] 实施多租户跨租户数据隔离防越权检测（`ProjectTenantMismatchError`，严格阻止通过伪造或竞态 ID 越权访问非属主项目）。
- **验收标准**
  - [x] 所有项目均归属明确租户（外键约束与应用层校验 `assert_tenant_active`）。
  - [x] 提供完整的纯 ASGI 3.0 REST API（`TenancyAPI`，覆盖租户与项目 CRUD 及状态流转，通过端到端 HTTP 测试）。
- **交付物**：多租户与项目管理模块（`python/packages/agentpay/src/agentpay/tenancy/`）、ASGI REST API（`agentpay.tenancy.api.TenancyAPI`）、单元与 API 测试套件（`python/tests/unit/test_tenancy_service.py`、`test_tenancy_api.py`）。

### T3.2 实现成员与角色权限
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 定义平台管理员与租户角色（`owner`, `admin`, `operator`, `viewer`）的完整 RBAC 权限矩阵（覆盖 22 项细粒度权限，涵盖租户、成员、项目、Agent、API Key、策略、钱包与审计）。
  - [x] 实现成员管理（`add_member`, `get_member`, `update_member_role`, `update_member_status`, `remove_member`, `list_members`，支持创建租户时自动初始化首位活跃 Owner）。
  - [x] 在服务端实施权限校验与最后 Owner 保护守卫（`enforce_member_permission` 强校验，严禁删除、降级或禁用租户最后一名活跃 Owner，抛出明确领域异常；成员被禁用时立即阻断一切操作）。
- **验收标准**
  - [x] 权限矩阵覆盖关键管理操作（通过 `python/tests/unit/test_member_rbac.py` 7 项全量单元与 API 集成测试，验证角色权限隔离、禁用阻断、最后 Owner 防御与跨租户隔离）。
- **交付物**：成员与 RBAC 模块（`python/packages/agentpay/src/agentpay/tenancy/rbac.py`、`models.py`、`service.py`、`api.py`）、单元与 API 测试套件（`python/tests/unit/test_member_rbac.py`）。

### T3.3 实现 API Key 生命周期与 Agent 绑定
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 创建 API Key 并关联明确租户、项目与指定 Agent（支持 Agent `active / suspended / revoked` 状态机）。
  - [x] 限制密钥权限范围（Scope 与权限授权，默认 `["gateway:call"]`）。
  - [x] 支持即时撤销和安全轮换（`revoke_api_key` 与 `rotate_api_key`）。
  - [x] 避免在日志或持久化中记录明文密钥（数据库仅存 SHA-256 哈希与安全前缀，明文仅单次暴露）。
  - [x] 运行时守卫拦截（Agent 停用或 Key 撤销即时阻断）。
- **验收标准**
  - [x] 撤销后的密钥与停用的 Agent 无法继续通过校验（`python/tests/unit/test_api_key_agent_lifecycle.py` 9 项测试全绿）。
  - [x] 密钥展示和存储符合安全要求。
- **交付物**：Agent 与 API Key 管理模块（`python/packages/agentpay/src/agentpay/tenancy/models.py`、`service.py`、`api.py`、`db.py`）、单元与 API 测试套件。


### T3.4 实施租户与 Agent 级路由策略
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **依赖**：T1.8、T3.1
- **任务内容**
  - [x] 为路由、策略和配置绑定租户、项目及 Agent（`Route` 与 `Policy` 实体在数据模型、PostgreSQL DDL 及 SQLite 中严格绑定 `tenant_id`、`project_id`、可选 `agent_id` 与 `route_id`）。
  - [x] 支持租户级与 Agent 级细粒度预算、限额、单笔上限和频率限制（RPM/QPS）（在 `service.py` 中实现 `resolve_effective_policy`，层级继承收敛计算最严格约束：Tenant → Project → Agent → Route → Policy Rules）。
  - [x] 支持 Agent 路由白名单（在 `Agent.policy_config` 与 `Policy.whitelisted_routes` 中支持路径模式与 Route ID 严格放行匹配，未授权访问判定为 `is_route_allowed = False`）。
  - [x] 路由策略增加“允许的支付方式”（支持 `wallet` / `onchain` / `both` 枚举，并在路由及有效策略解析中强制校验）。
  - [x] 确保 Gateway 执行时使用正确的租户与 Agent 上下文（在 Go Gateway `rcu.Manager` 中实现 `MatchRouteForTenant`，杜绝不同租户间路由串用泄漏；`RouteConfig` 与 `PolicyConfig` 注入 `TenantID` 与 `AgentID`）。
- **验收标准**
  - [x] 租户之间、Agent 之间的配置和策略互不串用（通过 `python/tests/unit/test_route_policy_lifecycle.py` 5 项全量测试与 Go Gateway `TestRCUMultiTenantRouteIsolation` 验证，跨租户路由/策略读写与访问严格返回 403 / 不匹配）。
- **交付物**：租户与 Agent 级配置与策略模块（`python/packages/agentpay/src/agentpay/tenancy/models.py`、`service.py`、`api.py`）、PostgreSQL DDL（`infra/postgres/schema.sql`）、Go 租户路由隔离器（`gateway-go/internal/rcu/rcu.go`）与全套单元/API 集成测试（`python/tests/unit/test_route_policy_lifecycle.py`）。

### T3.5 完成租户隔离与越权测试
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **依赖**：T3.1、T3.2、T3.3、T3.4
- **任务内容**
  - [x] 测试跨租户读取（覆盖 Projects, Members, Agents, API Keys, Routes, Policies, Wallets 均被隔离与拒绝）。
  - [x] 测试跨租户修改和删除（写操作与撤销拦截，返回 403 `ERR_CROSS_TENANT_VIOLATION` 与 `ERR_PERMISSION_DENIED`）。
  - [x] 测试跨租户查询和导出（各租户列表查询数据 0 污染、0 泄露）。
  - [x] 测试通过猜测 ID 绕过权限的场景（IDOR / Path 与 Payload 参数篡改注入严格被服务层与 REST API 阻断拦截）。
  - [x] 测试跨租户查看、冻结和调整钱包（Go 数据面与 Python 控制面钱包资金独立隔离，冻结/实扣无交叉影响，流水与对账 100% 独立）。
- **验收标准**
  - [x] 所有跨租户越权测试通过（`python/tests/unit/test_tenant_isolation_penetration.py` 4 大渗透测试套件 + `gateway-go/tests/wallet_isolation_test.go` 100% PASS）。
- **交付物**：租户隔离测试报告（`docs/specs/tenant-isolation-test-report.md`）。

### T3.6 实现钱包账户管理与管理员发放
- **优先级**：P0
- **状态**：已完成
- **完成时间**：2026-10-18
- **依赖**：T1.7、T3.1、T3.2
- **任务内容**
  - [x] 实现开户、余额与流水查询（租户创建自动开户 `get_or_create_wallet`，只追加不可变流水查询 `list_ledger_entries`）。
  - [x] 实现管理员发放充值（admin_grant），必须填写原因并写入审计（必填 `reason`、强制幂等键 `idempotency_key`，生成 `TopUpOrder` 与 `topup_credit` 流水并记录 `ap_audit_event`）。
  - [x] 实现人工调账，限定授权角色；大额调账是否二次审批按 T1.7 决策执行（限定 `Owner` / `Admin` 角色；大额调账 `>= 1,000 USDC` 强制要求独立审批人 `approved_by` 双人双控复核）。
  - [x] 实现钱包冻结 / 解冻（含欠费冻结）（`freeze_wallet` 支持 `frozen` 与 `frozen_arrears`，负余额补足前严格禁止解冻欠费账户）。
  - [x] 在服务端实施钱包权限校验（服务层与 REST API 强校验租户隔离边界，Operator 与 Viewer 读写分离，跨租户访问强拦截并返回 403）。
- **验收标准**
  - [x] 余额可由流水重新计算，并与缓存一致（`recalculate_balance` 支持双向流水平账对账，提供 `sync_cache` 自动同步）。
  - [x] 未授权角色无法发放或调账（通过 `test_wallet_lifecycle.py` 严格验证 Operator/Viewer 被 403 阻断）。
  - [x] 所有人工操作都有审计记录（自动向 `ap_audit_event` 写入操作人、IP、变更前后快照）。
- **交付物**：钱包管理 API（`python/packages/agentpay/src/agentpay/tenancy/models.py`、`service.py`、`api.py`、`db.py`，全套测试套件 `python/tests/unit/test_wallet_lifecycle.py` 10 项测试全部通过）。

### T3.7 实现链上充值
- **优先级**：P1（v1.x）
- **状态**：待开始
- **依赖**：T3.6
- **任务内容**
  - [ ] 确定充值地址方案（每租户独立地址或共用地址加备注匹配）。
  - [ ] 实现到账监听与确认数判断。
  - [ ] 处理错币种、金额不符、重复到账和链上重组回滚。
  - [ ] 以充值单 ID 作为幂等键入账。
- **验收标准**
  - [ ] 同一笔链上充值只入账一次。
  - [ ] 异常充值进入人工处理，不自动入账。
- **交付物**：链上充值模块。

### T3.8 评估并实现法币充值
- **优先级**：P2（之后）
- **状态**：待开始
- **依赖**：T3.6
- **任务内容**
  - [ ] 评估合规要求与运营地区限制。
  - [ ] 选择第三方支付渠道。
  - [ ] 实现回调验签和幂等入账。
  - [ ] 设计拒付 / 退款冲正和欠费处理。
- **验收标准**
  - [ ] 合规评估结论有记录。
  - [ ] 重复回调不会重复入账。
  - [ ] 拒付可通过冲正流水处理。
- **交付物**：评估结论；若进入开发，则交付法币充值模块。

---

## T4 — Web 管理控制台与配置发布

### T4.1 建立控制台基础框架
- **优先级**：P1
- **状态**：**已完成**
- **任务内容**
  - [x] 建立 Vue 3 + Element Plus 控制台。
  - [x] 实现登录和会话处理（支持开户注册、密码哈希、JWT 鉴权中间件与会话持久化）。
  - [x] 实现租户与项目切换（级联过滤与请求头注入）。
  - [x] 实现基础权限控制和导航（四级角色模型、路由全局拦截、团队成员管理、个人密码修改）。
- **验收标准**
  - [x] 不同角色只能访问授权页面和操作。
- **交付物**：控制台基础框架与团队成员管理模块（`console/`）。

### T4.2 实现路由与上游管理
- **优先级**：P1
- **状态**：已完成
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 创建、编辑和禁用路由。
  - [x] 配置上游地址和超时。
  - [x] 配置定价和支付地址（支持虚拟支付自动隐藏收款地址并切换主钱包与美元单位）。
  - [x] 配置上游 API 鉴权方式（`none`、`api_key`、`bearer`、`basic`、`oauth2_client_credentials`、`hmac_sign`、`custom_template`）：
    - 采用“类型 + 参数 + 密钥”三分离架构。
    - 敏感密钥（Secrets）使用 AES-256-GCM 信封加密存储且回显脱敏（只写不读）。
    - Go 网关反向代理转发时由 `Authenticator` 接口与策略注册表自动注入凭证。
    - 前端实现基于 Schema 驱动的动态鉴权表单组件 `<AuthConfigForm>` 与一键上游连通性/鉴权测试（Test Upstream）。
  - [x] 配置必要的访问策略。
- **验收标准**
  - [x] 管理员可通过 Web 完成基础路由与各类上游鉴权配置。
  - [x] 密钥明文永不回显，请求头安全脱敏展示。
- **交付物**：路由管理页面和 API（`console/src/views/routes/index.vue`、`console/src/components/AuthConfigForm.vue`、`gateway-go/internal/upstreamauth/`、`gateway-go/internal/admin/`、全套自动化测试）。

### T4.2.1 路由收款单位与多链/多代币高级配置
- **优先级**：P2
- **状态**：待开始（后续迭代规划）
- **任务内容**
  - [ ] 扩展路由收款单位（Asset）配置：支持虚拟支付通道自定义法币单位（USD、EUR、CNY 等）。
  - [ ] 扩展多链与网络选择（Network）：链上支付支持指定目标公链/网络（如 Ethereum Mainnet, Base, Arbitrum One, Polygon, Solana 等 CAIP-2 规范）。
  - [ ] 扩展结算代币（Token）：链上支付支持指定结算代币（USDC, USDT, DAI 等）及智能合约地址校验。
- **验收标准**
  - [ ] 路由创建与编辑表单可按网络及代币类型动态联动配置。
- **交付物**：多链多代币配置项扩展及数据面联动校验。

### T4.3 实现配置草稿与校验
- **优先级**：P1
- **状态**：**已完成**
- **完成时间**：2026-10-18
- **任务内容**
  - [x] 支持草稿编辑（工作草稿与在线生效网关完全解耦隔离，路由/策略修改即时保存于内存/数据库草稿态，未发布前绝不影响在线流量）。
  - [x] 校验必填字段、格式和配置组合（后端与前端全量校验矩阵：上游 URL 协议规范、路径全局唯一去重、EVM 40位收款地址十六进制校验、单价非负数、支付方式枚举限制、单次金额上限 $\le$ 24小时滚动预算、熔断阈值 RPM 非负校验）。
  - [x] 展示校验错误与可视化诊断报告（提供 `ValidationDiagnosticModal`，结构化输出错误码、异常实体、问题字段、错误信息与精准修复指导建议）。
  - [x] 实现草稿 vs 生效快照差异比对（提供 `DraftDiffDrawer`，支持路由与策略的新增、修改、删除可视化对比与前后差异高亮）。
  - [x] 强制发布门禁（在发布不可变快照前自动执行静态校验矩阵，校验失败严格阻断发布并弹窗报错）。
- **验收标准**
  - [x] 不合法配置无法发布（前后端自动化测试全面覆盖拦截非法 URL、非法地址、重复路径与倒挂预算）。
  - [x] 后端 `TestConfigDraftAndValidation` 单元与集成测试 100% 通过。
  - [x] 前端 `config_validation.spec.ts` 5 大测试用例全部通过，`npm run build` 零编译警告顺利打包。
- **交付物**：
  - 后端：Go 控制面校验引擎与草稿 API（`gateway-go/internal/admin/models.go`、`service.go`、`handlers.go`、`config_validation_test.go`）。
  - 前端：草稿控制栏（`console/src/views/routes/index.vue`）、发布门禁与差异比对（`console/src/views/versions/index.vue`）、校验诊断弹窗（`console/src/components/ValidationDiagnosticModal.vue`）、差异抽屉（`console/src/components/DraftDiffDrawer.vue`）、配置客户端（`console/src/api/client.ts`）与测试套件（`console/tests/config_validation.spec.ts`）。

### T4.4 实现发布、版本和回滚
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 发布时生成不可变配置快照。
  - [ ] 记录发布者、时间和版本。
  - [ ] 支持查看版本差异。
  - [ ] 支持回滚到已发布版本。
  - [ ] 确保请求记录可关联生效版本。
- **验收标准**
  - [ ] 发布、版本追踪和回滚均有自动化测试。
- **交付物**：配置版本管理模块。

### T4.5 保留 YAML 导入与导出
- **优先级**：P2
- **状态**：待开始
- **任务内容**
  - [ ] 评估 YAML 导入/导出格式。
  - [ ] 支持自托管和自动化部署场景。
  - [ ] 明确导入配置的校验和冲突处理规则。
- **验收标准**
  - [ ] YAML 不绕过配置校验和权限控制。
- **交付物**：YAML 导入/导出功能或设计决策。

### T4.6 实现钱包控制台页面
- **优先级**：P1
- **状态**：已完成
- **完成时间**：2026-10-19
- **依赖**：T3.6、T4.1
- **任务内容**
  - [x] 展示余额（可用 / 冻结 / 净资产）和流水（基于 6 位小数整数最小单位换算 USDC，支持流水单号、类型标签、发生金额、结余、关联业务与操作人）。
  - [x] 展示充值记录（支持管理员发放 `admin_grant`、链上充值 `onchain_deposit`、法币网关 `fiat` 渠道及到账状态、外部交易 Hash 筛选）。
  - [x] 提供管理员发放与调账入口（受 RBAC 权限控制；发放与调账强制必填原因并自动注入幂等键；调账金额 >= 1,000 USDC 强制启动双人双控审批复核流程）。
  - [x] 展示冻结 / 欠费状态（醒目 Alert 告警条；W6 欠费自动熔断；余额小于 0 严格禁止人工解冻欠费账户）。
  - [x] 提供双向平账核验功能（基于 T1.7/T5.7 五重一致性检查，一键生成对账平账报告与差额诊断）。
- **验收标准**
  - [x] 租户只能查看自己的钱包。
  - [x] 不同角色只能执行被授权的钱包操作。
- **交付物**：钱包管理页面（`console/src/views/wallet/index.vue`、`types/wallet.ts`、测试套件 `console/tests/wallet_management.spec.ts` 16 项测试全部通过）。

### T4.7 实现 Agent 管理与审计时间线页面
- **优先级**：P1
- **状态**：已完成
- **完成时间**：2026-10-19
- **依赖**：T1.8、T3.9、T4.1、T5.8（T5.8 时间线读模型先预留支持，前端完成高保真事件流与会话聚合）
- **任务内容**
  - [x] Agent 列表页面：展示 Agent 名称、客户端标识、归属项目、运行状态（active/suspended/revoked）、近期调用量、花费总额、策略拒绝率与 API Key 凭证管理。
  - [x] Agent 管理增加 KYA (Know Your Agent) 信用与风控档案：完整支持模型技术溯源（Provider、Version、Framework、Hosting、Attestation）、权责责任人映射（Owner、Email、Dept、AuthorizedBy）、自主度等级（L1 辅助 / L2 半自主 / L3 全自主）、风险评级、KYA 认证状态（verified/pending/unverified）与安全护栏。
  - [x] Agent 行为时间线（Timeline）：按时序倒序展示事件流，支持按 `session_id` / `task_id`（自报声明）聚合工作流会话（T5.8 预留读模型无缝对接）。
  - [x] 一键停用（Kill Switch）交互：弹窗二次确认输入停用原因（必填 $\ge 5$ 字符），支持级联作废绑定的所有 API Key 凭证，实时触发停用阻断后续请求；支持恢复启用。
  - [x] 审计详情弹窗：一键穿透展示请求关联的支付意图、结算单、冻结单、履约状态、命中策略判定结果（ALLOW/DENY）及请求哈希防篡改指纹。
  - [x] 声明字段标识：前端显式标记字段来源（`[观测 observed]` 客观事实防伪造 vs `[自报 declared]` Agent 自报声明仅用于关联，符合 T1.8 契约规范）。
- **验收标准**
  - [x] 租户管理员只能查看本租户的 Agent 和时间线，严格禁止跨租户访问。
  - [x] 一键停用操作实时生效并阻断后续请求。
  - [x] 前端 `agent_management.spec.ts` 6 组 16 项测试全部通过，`npm run build` 零错误零警告顺利打包。
- **交付物**：
  - 前端页面与组件：`console/src/views/agents/index.vue`、`console/src/types/agent.ts`、`console/src/api/client.ts`、`console/src/api/mockData.ts`。
  - 前端测试套件：`console/tests/agent_management.spec.ts`。

---

## T5 — 消费记录、支付审计与用量分析

### T5.1 建立请求与用量事件记录
- **优先级**：P1
- **状态**：已完成
- **任务内容**
  - [x] 记录租户、项目、路由、请求 ID 和配置版本（`tenant_id`、`project_id`、`agent_id`、`route_id`、`request_id`、`config_version`）。
  - [x] 记录请求时间、结果和必要的耗时指标（`created_at`、`status_code`、`duration_ms`）。
  - [x] 定义失败请求和重复请求的记录规则（`error_code` 规范映射；幂等重放记录 `is_replay=true`，严格防二次扣费不变量）。
- **验收标准**
  - [x] 请求记录可以关联到后续支付和履约记录（记录与用量事件均绑定一致的 `intent_id`）。
  - [x] 缓存重放（Cache Hit）与重试仅生成 `is_replay=true` 请求记录，不重复产生 `UsageEvent`（防二次计费）。
  - [x] 多租户严格边界隔离，控制台 Admin REST API 具备租户校验与 RBAC `audit:read` 权限防护。
  - [x] 网关双引擎支持（PostgreSQL 生产持久化 + 线程安全 In-Memory 开发测试降级）。
  - [x] 全套单元测试与端到端集成测试通过（`usage_test.go` 与 `usage_e2e_test.go` 100% PASS）。
- **交付物**：
  - 数据模型与持久化：`infra/postgres/schema.sql`、`specs/multitenant-data-model.md`、`gateway-go/internal/usage/models.go`、`gateway-go/internal/usage/service.go`、`gateway-go/internal/usage/postgres.go`。
  - 控制平面查询与 RBAC：`gateway-go/internal/admin/handlers.go`、`gateway-go/internal/admin/service.go`、`gateway-go/internal/admin/postgres.go`。
  - 数据平面切入钩子：`gateway-go/internal/x402/handler.go`、`gateway-go/internal/server/server.go`。
  - 测试套件：`gateway-go/internal/usage/usage_test.go`、`gateway-go/tests/usage_e2e_test.go`。

### T5.2 建立支付、结算与履约关联
- **优先级**：P0
- **状态**：已完成
- **任务内容**
  - [x] 关联 PaymentIntent（记录业务意图、状态、租户/项目/Agent 归属、金额与幂等键）。
  - [x] 关联 SettlementRecord 和链上交易信息（记录资产扣费/结算状态、Nonce、TxHash、BlockNumber）。
  - [x] 关联 FulfillmentRecord（独立维护上游状态码、交付延迟、响应体哈希）。
  - [x] 钱包支付关联 Hold 与账本流水（联动 `ap_hold` 与 `ap_ledger_entry` 的 `capture` 与 `hold_release`）。
  - [x] 独立维护支付状态与履约状态（三维状态机：意图、结算、履约各自推进，防止上游 5xx 污染结算）。
  - [x] 明确未知状态和恢复中的展示方式（严格推导 11 种复合展示状态，5xx 保留冻结标记 `RECOVERY_REQUIRED`）。
- **验收标准**
  - [x] 每笔消费能够追踪请求、支付、结算和履约状态，并提供标准复合展示视图。
  - [x] 租户隔离审计与 RBAC 权限控制健全（跨租户访问硬隔离，返回 403 Forbidden）。
  - [x] 全套单元测试与端到端集成测试通过（`payment_test.go` 与 `payment_correlation_e2e_test.go` 100% PASS）。
- **交付物**：
  - 核心模型与领域服务：`gateway-go/internal/payment/models.go`、`gateway-go/internal/payment/service.go`、`gateway-go/internal/payment/postgres.go`。
  - 控制平面查询与 RBAC：`gateway-go/internal/admin/handlers.go`、`gateway-go/internal/admin/service.go`、`gateway-go/internal/admin/postgres.go`。
  - 网关核心状态机切入：`gateway-go/internal/x402/handler.go`、`gateway-go/internal/server/server.go`。
  - 数据库模式与多租户规约：`infra/postgres/schema.sql`、`specs/multitenant-data-model.md`。
  - 自动化测试套件：`gateway-go/internal/payment/payment_test.go`、`gateway-go/tests/payment_correlation_e2e_test.go`。

### T5.3 实现消费记录查询
- **优先级**：P1
- **状态**：已完成
- **任务内容**
  - [x] 支持按时间、状态、项目、Agent (`agent_id`)、路由和支付方式多维筛选。
  - [x] 支持高性能分页查询与时间范围（RFC3339）过滤。
  - [x] 展示请求、支付、结算、冻结、履约详情及 Agent 身份关联。
  - [x] 强制执行租户数据边界校验，禁止跨租户查询。
- **验收标准**
  - [x] 租户无法查看其他租户的数据（后端 RBAC 与数据边界硬隔离）。
  - [x] 可以按单 Agent 独立检索消费与履约历史（支持通过 URL 参数 `?agent_id=...` 和操作跳转直达检索）。
- **交付物**：
  - 前端路由与页面：`console/src/views/payments/index.vue`、`console/src/router/index.ts`。
  - 前端类型与 API 客户端：`console/src/types/payment.ts`、`console/src/api/client.ts`、`console/src/api/mockData.ts`。
  - Agent 联动跳转：`console/src/views/agents/index.vue`（Agent 列表与时间线抽屉增加一键跳转消费审计）。
  - 后端时间范围过滤增强：`gateway-go/internal/admin/handlers.go`（解析 `start_time` / `end_time`）。

### T5.4 实现管理操作审计
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 记录操作者、时间、对象和操作结果。
  - [ ] 记录路由、定价、策略和权限的变更。
  - [ ] 关联配置版本。
  - [ ] 对敏感字段进行脱敏。
- **验收标准**
  - [ ] 关键管理变更可以追溯。
- **交付物**：AuditEvent 与审计查询页面。

### T5.5 实现用量统计
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 按租户、项目、路由和时间汇总请求量。
  - [ ] 汇总成功、失败和待恢复数量。
  - [ ] 汇总支付金额（区分链上与钱包）和相关用量指标。
  - [ ] 防止重复事件导致重复统计。
- **验收标准**
  - [ ] 汇总口径有定义，并可与明细记录核对。
- **交付物**：用量统计 API 与页面。

### T5.6 定义审计保留与导出策略
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 定义数据保留周期。
  - [ ] 定义敏感字段脱敏规则。
  - [ ] 限制审计导出权限。
  - [ ] 记录导出操作。
- **验收标准**
  - [ ] 保留、脱敏、访问和导出规则有文档且经过测试。
- **交付物**：审计数据治理说明。

### T5.7 实现账本对账
- **优先级**：P0
- **状态**：待开始
- **依赖**：T2.8、T3.6
- **任务内容**
  - [ ] 核对流水汇总与余额缓存。
  - [ ] 核对已确认扣款的冻结、结算记录和履约记录。
  - [ ] 核对已确认充值单与入账流水（一对一）。
  - [ ] 核对链上充值与链上交易、法币充值与第三方账单。
  - [ ] 检查超时未处理的冻结（僵尸冻结）。
  - [ ] 对账差异告警。
- **验收标准**
  - [ ] 人为制造的差异能够被检出。
  - [ ] 对账规则和处理流程有文档。
- **交付物**：对账任务与对账报告。

### T5.8 实现 Agent 行为时间线与多维查询
- **优先级**：P1
- **状态**：已完成
- **依赖**：T1.8、T2.9、T5.1
- **任务内容**
  - [x] 构建准实时读模型，对事件流（`agent.request`, `agent.policy_decision`, `agent.spend`, `agent.control`, `agent.anomaly` 等）进行索引。
  - [x] 支持按 `session_id` / `task_id`（自报声明字段）进行会话聚合与独立过滤检索。
  - [x] 关联请求命中的策略规则、拒绝原因码、实际耗时、状态码、花费、请求哈希指纹与可读摘要。
  - [x] 严格执行多租户隔离与项目/Agent 级别权限过滤。
- **验收标准**
  - [x] 时间线按时序完整重现 Agent 行为全过程（倒序排列、完整还原观察事实 vs 自报声明）。
  - [x] 聚合口径与底层不可变审计事件完全一致，自报元数据缺失时安全降级，绝不影响鉴权与结算。
  - [x] 多租户硬边界防护，严禁跨租户检索 Agent 时间线。
  - [x] 单元测试与端到端集成测试通过（`timeline_test.go` 与 `agent_timeline_e2e_test.go` 100% PASS）。
- **交付物**：
  - 数据库与数据规约：`infra/postgres/schema.sql`（`ap_agent_audit_event` 表与时序复合索引）、`specs/multitenant-data-model.md`。
  - 核心领域模型与读模型服务：`gateway-go/internal/timeline/models.go`、`gateway-go/internal/timeline/service.go`、`gateway-go/internal/timeline/postgres.go`、`gateway-go/internal/timeline/emitter.go`。
  - 控制平面查询与 RBAC：`gateway-go/internal/admin/handlers.go`、`gateway-go/internal/admin/service.go`、`gateway-go/internal/admin/postgres.go`。
  - 网关服务装配：`gateway-go/internal/server/server.go`（双引擎装配与 `PaymentProcessor` 审计发射器适配注入）。
  - 测试套件：`gateway-go/internal/timeline/timeline_test.go`、`gateway-go/tests/agent_timeline_e2e_test.go`。

### T5.9 实现异常规则检测与告警 (v1.x)
- **优先级**：P2（v1.x）
- **状态**：待开始
- **依赖**：T5.8
- **任务内容**
  - [ ] 实现阈值规则引擎：调用量激增、花费速率骤增、连续被策略拒绝、首次访问非常规路由、非工作时段突发调用。
  - [ ] 支持按 Agent 粒度自定义风险阈值。
  - [ ] 规则命中触发 `agent.anomaly` 事件。
  - [ ] 默认仅告警不自动阻断；自动阻断须显式开启并设置冷却期，防止误杀导致业务工作流中断。
  - [ ] 告警渠道支持控制台通知与 Webhook 回调推送。
- **验收标准**
  - [ ] 模拟突增或频繁拒绝场景能准实时触发告警。
- **交付物**：异常检测与告警引擎。

---

## T6 — 集成验收、试点与发布

### T6.1 完成干净环境部署验证
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 按文档在干净环境部署。
  - [ ] 验证配置、启动、健康检查和升级流程。
  - [ ] 验证失败时的恢复或回滚流程。
- **验收标准**
  - [ ] 非原开发环境可按文档完成部署。
- **交付物**：部署验证记录。

### T6.2 完成端到端支付测试
- **优先级**：P0
- **状态**：待开始
- **任务内容**
  - [ ] 验证 Python Buyer → Go Gateway → 上游 API。
  - [ ] 验证正常支付。
  - [ ] 验证超限拒绝。
  - [ ] 验证重复请求和超时恢复。
  - [ ] 验证上游 5xx 与结算未知。
  - [ ] 验证钱包支付：正常扣费、余额不足、重复请求、上游失败释放冻结。
- **验收标准**
  - [ ] 核心链路和故障注入测试全部通过。
- **交付物**：端到端测试报告。

### T6.3 完成多租户与配置安全验收
- **优先级**：P0
- **状态**：待开始
- **任务内容**
  - [ ] 验证跨租户访问被拒绝。
  - [ ] 验证 API Key 撤销生效。
  - [ ] 验证配置发布和回滚。
  - [ ] 验证请求关联正确的配置版本。
  - [ ] 验证钱包越权访问被拒绝、人工调账权限和账本不可变。
- **验收标准**
  - [ ] 权限、隔离和配置发布测试通过。
- **交付物**：安全验收记录。

### T6.4 完成日志与依赖安全检查
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 检查日志和错误信息中的敏感数据。
  - [ ] 执行依赖审计。
  - [ ] 更新威胁模型和已知限制。
  - [ ] 明确测试网络和真实资金使用边界。
- **验收标准**
  - [ ] 高优先级问题已修复或有明确阻断措施。
- **交付物**：安全检查报告。

### T6.5 开展小范围试点
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 选择少量可信测试租户或内部项目。
  - [ ] 记录部署、配置和使用中的问题。
  - [ ] 为问题分配优先级和负责人。
  - [ ] 根据反馈修复关键问题。
- **验收标准**
  - [ ] 试点问题均已解决或明确进入后续版本。
- **交付物**：试点复盘。

### T6.6 发布版本候选
- **优先级**：P1
- **状态**：待开始
- **任务内容**
  - [ ] 编写版本说明。
  - [ ] 编写升级与回滚说明。
  - [ ] 编写已知限制和安全说明。
  - [ ] 检查所有发布验收条件。
- **验收标准**
  - [ ] 文档、测试证据和发布流程完整。
  - [ ] 未经独立安全审计，不宣称产品已通过安全审计。
- **交付物**：版本候选发布包。

### T6.7 完成钱包并发与对账验收
- **优先级**：P0
- **状态**：待开始
- **依赖**：T2.8、T3.6、T5.7
- **任务内容**
  - [ ] 并发冻结测试（多个请求争用有限余额）。
  - [ ] 幂等测试（重复冻结、确认、充值入账）。
  - [ ] 冻结超时与崩溃恢复测试。
  - [ ] 履约结果未知场景测试。
  - [ ] 余额不足不自动降级到链上的测试。
  - [ ] 对账差异检出测试。
- **验收标准**
  - [ ] 全部测试通过，且不出现透支或重复扣费。
- **交付物**：钱包验收记录。

---

## 总体依赖顺序

```text
T0.1 CI 修复（已完成）
  └── T0.2 冻结当前基线（已完成）
       └── T1 协议、状态机、数据契约、钱包账本契约与威胁模型
            ├── T1.1 统一支付状态机（已完成）
            ├── T1.7 钱包与账本契约（已完成）
            ├── T1.2 多租户数据模型与数据库 ADR（进行中）
            ├── T2 Go Gateway（钱包支付路径 T2.8 依赖 T1.7、T3.3、T3.6）
            ├── T3 多租户身份、权限与隔离（钱包管理 T3.6 依赖 T1.7）
            ├── T4 Web 控制台与配置发布（依赖 T3 与 T1.4；钱包页面 T4.6 依赖 T3.6）
            └── T5 消费审计（依赖 T1.5，并与 T2/T3 集成；账本对账 T5.7 依赖 T2.8、T3.6）
                  └── T6 集成验收、试点与发布（钱包验收 T6.7 依赖 T2.8、T3.6、T5.7）
```

## 核心架构决策状态总览

- [x] **D1 链上结算/履约顺序**：方案 A（先结算后履约），履约失败走补偿退款。
- [x] **W1 钱包归属**：**租户级**一个钱包，项目级用限额与预算控制。
- [x] **W2 计价单位**：**整数最小单位 + 单一资产代码**（如 USDC 6 位小数），v1 只支持一种资产。
- [x] **提现策略**：**v1 不支持提现**（闭环消费，规避资金托管合规风险）。
- [x] **W3 冻结超时**：`upstream_timeout + 60s`（默认 120s）。
- [x] **W4 履约未知处理**：**保持冻结不动并转 `needs_review`**，严禁自动扣费或释放。
- [x] **W5 支付方式选择**：凭证路由，**余额不足严禁自动降级到链上**。
- [x] **W6 欠费处理**：允许账户进入“欠费冻结（`frozen_arrears`）”，禁止继续冻结新额度。
