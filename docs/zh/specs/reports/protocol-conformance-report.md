# AgentPay Python / Go 协议一致性测试报告 (T2.6 Conformance Report)

> **报告版本**：v2.0.0**执行日期**：2026-10-18**测试状态**：**100% 通过 (PASSED)\*\***执行环境**：macOS (Darwin arm64), Python 3.12.7 (pytest 9.1.1), Go 1.24**涉及核心规范\*\*：
>
> - [`specs/protocol-consistency-spec.md`](https://github.com/open-agentpay/AgentPay/blob/main/specs/protocol-consistency-spec.md) (T1.3 统一协议契约)
> - [`specs/conformance/protocol-errors.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/conformance/protocol-errors.json) (统一错误码表与 Schema)
> - [`specs/test-vectors/dual-path-protocol.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/test-vectors/dual-path-protocol.json) (双轨协议一致性测试向量)
> - [`specs/conformance/state-machine.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/conformance/state-machine.json) (统一状态机模型)

---

## 1. 概述与测试目标

为确保 AgentPay 的 **Python 买方客户端（Buyer SDK）** 与 **Go 云原生网关（Go Gateway）** 在数据面与控制面交互时行为完全同构，任务 **T2.6 完成 Python/Go 一致性测试** 对比了以下维度的跨语言同构性：

1. **统一错误分类与响应信封（Canonical Error Taxonomy）**：验证 Go 网关与 Python 客户端所使用的全量 17 个标准化错误码、HTTP 状态码、领域划分与 JSON 响应格式 1:1 对齐；
2. **双轨协议测试向量（Dual-Path Protocol Test Vectors）**：验证 `TC_PROTO_01` 至 `TC_PROTO_06` 跨语言执行，在 x402 链上出题、虚拟钱包扣费、余额不足无静默降级、欠费冻结、幂等冲突等场景下的确定性裁定；
3. **状态机矩阵流转与守卫（State Machine & Invariants）**：在 Go 内部实现状态机引擎，验证其与 Python 侧状态迁移规则、终端不可变性、调度守卫（`INV_1_DISPATCH_GUARD`）、未知防重守卫（`INV_2_UNKNOWN_SUBMIT_GUARD`）与复合视图推导 100% 相同；
4. **端到端跨语言网络互操作性（Live HTTP Cross-Language Conformance）**：启动 Go 网关独立进程与 Python 模拟后端，通过 Python `httpx` 客户端真实向 Go 网关发起网络请求，验证挑战解析、Bearer Token 虚拟钱包扣费、配置版本追踪标头注入、防双花防篡改冲突等全链路集成。

---

## 2. 跨语言测试覆盖矩阵

| 测试维度               | 测试套件与文件                                                                                                                             | 测试用例数 | 结果     | 重点验证项                                                                                             |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ---------- | -------- | ------------------------------------------------------------------------------------------------------ |
| **全量错误码对齐**     | `gateway-go/internal/x402/conformance_test.go`<br>`python/tests/unit/test_protocol_conformance_vectors.py`                                 | 18 项      | **PASS** | 17 个 Canonical Error Code 状态码、Domain、Message 及标准 Wire Format Envelope 校验                    |
| **双轨协议测试向量**   | `specs/test-vectors/dual-path-protocol.json`<br>`gateway-go/internal/x402/conformance_test.go`                                             | 6 项       | **PASS** | `TC_PROTO_01` ~ `TC_PROTO_06` 驱动测试，覆盖出题、选路、防降级、欠费、幂等冲突                         |
| **状态机矩阵与守卫**   | `gateway-go/internal/statemachine/engine_test.go`<br>`python/tests/unit/test_state_machine_matrix.py`                                      | 42 项      | **PASS** | 35 个合法转移、非法转移拦截、终端状态不可变、双轨调度守卫、未知防重守卫、复合视图映射                  |
| **配置发布一致性**     | `specs/test-vectors/config-publishing.json`<br>`gateway-go/internal/rcu/snapshot_test.go`<br>`python/tests/unit/test_config_publishing.py` | 5 项       | **PASS** | `TC_CFG_01` ~ `TC_CFG_05`，静态校验矩阵、Canonical SHA-256 签名计算、版本回滚与容灾降级                |
| **端到端进程级互操作** | `python/tests/integration/test_cross_language_conformance.py`                                                                              | 6 项       | **PASS** | 真实 Python 客户端向独立 Go Gateway 发起 HTTP 请求，解析 Challenge、钱包扣费、标头溯源、幂等与故障注入 |

---

## 3. 测试用例执行与结果明细

### 3.1 双轨协议测试向量结果 (Dual-Path Vectors)

| 用例 ID       | 类别           | 输入特征                        | 预期行为                                                                 | Go 网关验证          | Python SDK 验证             | 结论     |
| ------------- | -------------- | ------------------------------- | ------------------------------------------------------------------------ | -------------------- | --------------------------- | -------- |
| `TC_PROTO_01` | ONCHAIN_X402   | 匿名未认证 GET 请求             | 返回 402，带`PAYMENT-REQUIRED` 标头，载荷符合 EIP-3009 结构              | PASS (402, EIP-3009) | PASS (from_header 解析成功) | **一致** |
| `TC_PROTO_02` | WALLET_PAYMENT | 携带有效 API Key                | 选用虚拟钱包扣费，执行 Hold 冻结，不触发链上出题                         | PASS (200, 扣除余额) | PASS (选路 wallet)          | **一致** |
| `TC_PROTO_03` | WALLET_PAYMENT | 虚拟钱包余额不足                | 返回 402`ERR_WALLET_INSUFFICIENT_BALANCE`，**严禁下发 PAYMENT-REQUIRED** | PASS (402, 无降级头) | PASS (402, 拒绝自动降级)    | **一致** |
| `TC_PROTO_04` | WALLET_PAYMENT | 钱包欠费冻结 (`frozen_arrears`) | 拦截并返回 403`ERR_WALLET_FROZEN`                                        | PASS (403, 阻断)     | PASS (403, 阻断)            | **一致** |
| `TC_PROTO_05` | ROUTING        | wallet 路由收到链上签名         | 拦截并返回 403`ERR_PAYMENT_METHOD_NOT_ALLOWED`                           | PASS (403, 阻断)     | PASS (403, 阻断)            | **一致** |
| `TC_PROTO_06` | IDEMPOTENCY    | 相同幂等键附带不同载荷指纹      | 拦截并返回 409`ERR_IDEMPOTENCY_CONFLICT`                                 | PASS (409, 阻断)     | PASS (409, 阻断)            | **一致** |

### 3.2 错误分类全量对齐 (17 个错误码表)

通过 `TestCanonicalErrorsIntegrityAgainstTaxonomy` 自动读取 `specs/conformance/protocol-errors.json` 与 Go 网关 `CanonicalErrorRegistry`：

- `ERR_PAYMENT_REQUIRED` (402, PROTOCOL)
- `ERR_WALLET_INSUFFICIENT_BALANCE` (402, WALLET)
- `ERR_WALLET_FROZEN` (403, WALLET)
- `ERR_PAYMENT_METHOD_NOT_ALLOWED` (403, ROUTING)
- `ERR_POLICY_BUDGET_EXCEEDED` (403, POLICY)
- `ERR_POLICY_MAX_PER_TX` (403, POLICY)
- `ERR_POLICY_CIRCUIT_BROKEN` (429, POLICY)
- `ERR_API_KEY_INVALID` (401, AUTH)
- `ERR_API_KEY_SCOPE_DENIED` (403, AUTH)
- `ERR_TENANT_SUSPENDED` (403, TENANT)
- `ERR_SIGNATURE_INVALID` (401, CRYPTOGRAPHY)
- `ERR_NONCE_REPLAY` (409, SETTLEMENT)
- `ERR_IDEMPOTENCY_CONFLICT` (409, IDEMPOTENCY)
- `ERR_HOLD_EXPIRED` (408, WALLET)
- `ERR_UPSTREAM_TIMEOUT` (504, GATEWAY)
- `ERR_UPSTREAM_FAILED` (502, GATEWAY)
- `ERR_CONFIG_VERSION_EXPIRED` (409, CONFIG)

全部错误码的状态码（HTTP Status）、业务领域（Domain）与响应信封（Wire Format Envelope）跨语言 100% 对齐。

### 3.3 状态机矩阵与安全守卫对齐

在 Go 侧建立 `gateway-go/internal/statemachine` 引擎并执行 `specs/conformance/state-machine.json`：

1. **合法转移矩阵**：35 个 (from, event, to) 状态迁移在 Go 与 Python 中产出完全一致的目标状态；
2. **终态不可变性（Terminal Immutability）**：`settlement: confirmed/failed/released`、`fulfillment: succeeded/failed`、`payment_intent: settled/failed` 均严格拒绝后续修改事件；
3. **双轨调度守卫（`INV_1_DISPATCH_GUARD`）**：
   - 链上路径：必须满足 `settlement == confirmed` 才允许向上游履约分发；
   - 钱包路径：必须满足 `settlement == held`（已成功冻结额度）才允许向上游履约分发；
4. **未知防重提交守卫（`INV_2_UNKNOWN_SUBMIT_GUARD`）**：
   - 当 `settlement == unknown` 时，严格禁止盲目重新 submit；仅当处于 `not_started` 时才允许提交；
5. **复合视图映射（Composite Views）**：9 种状态组合推导出的用户端状态（`WALLET_HOLD_PROCESSING`, `SETTLEMENT_IN_PROGRESS`, `FULFILLMENT_IN_PROGRESS`, `COMPLETED`, `SETTLED_FULFILLMENT_FAILED_REFUND_REQUIRED`, `SETTLED_FULFILLMENT_UNKNOWN_NEEDS_REVIEW`, `RELEASED_UNBILLED`, `PAYMENT_FAILED`）与 Python 实现 100% 同构。

---

## 4. 发现并修复的行为差异清单

在一致性测试落地过程中，发现并修复了以下行为差异：

1. **Canonical Error Taxonomy 命名与别名同构**：
   - _差异_：Go 网关此前定义了 `ERR_NONCE_ALREADY_USED` 与 `ERR_UPSTREAM_BAD_GATEWAY`，而标准规范定义为 `ERR_NONCE_REPLAY` (409, SETTLEMENT) 与 `ERR_UPSTREAM_FAILED` (502, GATEWAY)，且缺少 `ERR_HOLD_EXPIRED` 与 `ERR_CONFIG_VERSION_EXPIRED`；
   - _修复_：完善 `gateway-go/internal/x402/errors.go`，补齐全量 17 个错误码及其 Actionable 字段，并设立向后兼容别名保证已有模块零破坏。
2. **出题 Network 动态化与 Fallback 机制**：
   - _差异_：Go 网关原先在下发 402 challenge 时写死了 `"eip155:84532"`，未读取路由定义中的 `route.Network`；
   - _修复_：在 `rcu.RouteConfig` 与 `config.RouteDefinition` 中增加 `Network` 字段，并在出题时优先取路由配置的 `route.Network`，空值回退至全局 `cfg.Settlement.DefaultNetwork`（如 `eip155:421614` Arbitrum Sepolia）。
3. **大小写不敏感标头与 EVM 地址比较**：
   - _差异_：Python 字典键查找对 HTTP Header 大小写敏感，导致断言 `x-agentpay-config-version` 偶现空指针；EVM 地址校验在 Python SDK 规范化为小写，而 Go 原样透传；
   - _修复_：端到端测试与客户端逻辑统一采用大小写不敏感（RFC 7230 兼容）标头提取与以太坊地址比较。
4. **测试钱包预设与隔离**：
   - _优化_：在 `gateway-go/internal/server` 默认构造中内置激活、余额不足、欠费冻结三类预设账户，便于任何本地开发工具与端到端跨语言测试即开即测。

---

## 5. 验收结论

全仓测试验证结果：

- **Python 测试**：373 项测试（含 6 项端到端跨语言测试）100% 通过（PASS）；
- **Go 测试**：全量单元、状态机矩阵与集成测试 100% 通过（PASS）；
- **测试向量验证**：`specs/test-vectors` 下各 JSON 跨语言测试向量全部绿灯；
- **代码规范**：`ruff check` 与 `ruff format --check` 无任何警告。

**T2.6 完成 Python/Go 一致性测试验收标准全部满足，正式交付完成。**
