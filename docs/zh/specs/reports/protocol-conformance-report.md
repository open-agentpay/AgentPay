# AgentPay Python / Go 协议一致性测试报告 (T2.6 Conformance Report)

> **报告版本**：v2.1.0  
> **执行日期**：2026-10-18（更新于 2026-10-26）  
> **测试状态**：**100% 通过 (PASSED)**  
> **执行环境**：macOS (Darwin arm64), Python 3.12.7 (pytest 9.1.1), Go 1.24  
> **涉及核心规范**：
> - [`specs/protocol-consistency-spec.md`](https://github.com/open-agentpay/AgentPay/blob/main/specs/protocol-consistency-spec.md)（T1.3 统一协议契约）
> - [`specs/conformance/protocol-errors.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/conformance/protocol-errors.json)（统一错误码分类表与响应 Schema）
> - [`specs/test-vectors/dual-path-protocol.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/test-vectors/dual-path-protocol.json)（双轨协议一致性测试向量）
> - [`specs/test-vectors/config-publishing.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/test-vectors/config-publishing.json)（配置发布与 RCU 快照一致性向量）
> - [`specs/conformance/state-machine.json`](https://github.com/open-agentpay/AgentPay/blob/main/specs/conformance/state-machine.json)（统一三态状态机模型 v2.1.0）

---

## 1. 概述与测试目标

为确保 AgentPay 的 **Python 买方客户端（Buyer SDK，`sdk/python`）** 与 **Go 云原生网关（Go Gateway，`services/gateway`）** 在数据面与控制面交互时行为 100% 同构，**任务 T2.6（完成 Python/Go 一致性测试）** 系统性验证了以下五个维度的跨语言等价性：

1. **统一错误分类与响应信封（Canonical Error Taxonomy & Wire Envelope）**：验证 Go 网关与 Python 客户端所使用的全量 19 个标准化错误码（`specs/conformance/protocol-errors.json`）、HTTP 状态码、业务领域划分（Domain）、可操作修复指引（Actionable）与 JSON 响应信封 1:1 对齐。
2. **双轨协议测试向量（Dual-Path Protocol Test Vectors）**：跨语言执行 `TC_PROTO_01` 至 `TC_PROTO_06`，验证在 x402 链上出题、虚拟钱包扣费、余额不足严禁静默降级、欠费冻结阻断、路由支付方式限制及幂等载荷冲突等场景下的确定性裁定。
3. **状态机矩阵流转与安全不变量守卫（State Machine & Invariants）**：对比验证 Go 状态机引擎（`services/gateway/internal/statemachine`）与 Python 状态机在全量 35 条合法迁移规则、终态不可变性、6 项核心安全不变量（`INV_1` 至 `INV_6`）以及 8 种复合视图推导上的 100% 同构。
4. **配置发布与 RCU 容灾降级测试向量（Config Publishing Vectors）**：跨语言执行 `TC_CFG_01` 至 `TC_CFG_05`，验证路由与策略静态校验、Canonical SHA-256 快照签名计算、版本回滚一致性以及控制面不可用时的网关本地缓存容灾降级。
5. **端到端跨语言网络互操作性（Live HTTP Cross-Language Interoperability）**：拉起独立的 Go Gateway 进程与 Python 模拟上游后端，通过 Python `httpx` 客户端向 Go 网关发起真实 HTTP 请求，端到端验证挑战解析、Bearer Token 虚拟钱包扣费、配置版本追踪标头注入与防双花/防篡改冲突拦截。

---

## 2. 跨语言测试覆盖矩阵

| 测试维度 | 测试套件与文件 | 测试用例数 | 结果 | 重点验证项 |
| :--- | :--- | :---: | :---: | :--- |
| **全量错误码对齐** | `services/gateway/internal/x402/conformance_test.go`<br>`sdk/python/tests/unit/test_protocol_conformance_vectors.py` | 19 项 | **PASS** | 全量 19 个 Canonical Error Code 的 HTTP 状态码、Domain、Message、Actionable 及标准 Wire Format Envelope 校验 |
| **双轨协议测试向量** | `specs/test-vectors/dual-path-protocol.json`<br>`services/gateway/internal/x402/conformance_test.go` | 6 项 | **PASS** | `TC_PROTO_01` ~ `TC_PROTO_06` 向量驱动测试，覆盖出题、选路、防静默降级、欠费冻结、幂等冲突 |
| **状态机矩阵与守卫** | `services/gateway/internal/statemachine/engine_test.go`<br>`sdk/python/tests/unit/test_state_machine_matrix.py` | 42 项 | **PASS** | 35 个合法状态迁移、非法迁移拦截、终态不可变性、双轨调度守卫、未知防重守卫、复合视图映射 |
| **配置发布一致性** | `specs/test-vectors/config-publishing.json`<br>`services/gateway/internal/rcu/snapshot_test.go`<br>`sdk/python/tests/unit/test_config_publishing.py` | 5 项 | **PASS** | `TC_CFG_01` ~ `TC_CFG_05` 静态校验矩阵、Canonical SHA-256 指纹计算、版本回滚与断连容灾降级 |
| **端到端进程级互操作** | `sdk/python/tests/integration/test_cross_language_conformance.py` | 6 项 | **PASS** | 真实 Python 客户端向独立 Go Gateway 发起 HTTP 请求，验证挑战解析、钱包扣费、标头溯源、幂等与故障注入 |

---

## 3. 测试用例执行与结果明细

### 3.1 双轨协议测试向量结果 (`specs/test-vectors/dual-path-protocol.json`)

| 用例 ID | 类别 | 输入特征 | 预期行为 | Go 网关验证 | Python SDK 验证 | 结论 |
| :--- | :--- | :--- | :--- | :---: | :---: | :---: |
| `TC_PROTO_01` | `ONCHAIN_X402` | 匿名未认证 GET 请求 | 返回 `402`，带 `PAYMENT-REQUIRED` 标头，载荷符合 EIP-3009 结构 | PASS (402, EIP-3009) | PASS (`from_header` 解析成功) | **一致** |
| `TC_PROTO_02` | `WALLET_PAYMENT` | 携带有效 API Key | 选用虚拟钱包路径，执行 Hold 预留冻结，不下发链上出题 | PASS (200, 扣除余额) | PASS (选路 wallet) | **一致** |
| `TC_PROTO_03` | `WALLET_PAYMENT` | 虚拟钱包可用余额不足 | 返回 `402 ERR_WALLET_INSUFFICIENT_BALANCE`，**严禁下发 `PAYMENT-REQUIRED`** | PASS (402, 无降级头) | PASS (402, 拒绝自动降级) | **一致** |
| `TC_PROTO_04` | `WALLET_PAYMENT` | 钱包处于欠费冻结 (`frozen_arrears`) | 拦截请求并返回 `403 ERR_WALLET_FROZEN` | PASS (403, 阻断) | PASS (403, 阻断) | **一致** |
| `TC_PROTO_05` | `ROUTING` | 仅允许 `wallet` 的路由收到链上签名 | 拦截请求并返回 `403 ERR_PAYMENT_METHOD_NOT_ALLOWED` | PASS (403, 阻断) | PASS (403, 阻断) | **一致** |
| `TC_PROTO_06` | `IDEMPOTENCY` | 相同幂等键附带不同请求载荷摘要 | 拦截请求并返回 `409 ERR_IDEMPOTENCY_CONFLICT` | PASS (409, 阻断) | PASS (409, 阻断) | **一致** |

### 3.2 错误分类全量对齐 (`specs/conformance/protocol-errors.json` 共 19 项)

Go 网关输出与 Python SDK 解析的所有错误响应均严格遵循统一的 JSON 信封格式（Wire Format Envelope）：

```json
{
  "error": {
    "code": "ERR_WALLET_INSUFFICIENT_BALANCE",
    "message": "Wallet available balance is insufficient to cover this request.",
    "domain": "WALLET",
    "status": 402,
    "requestId": "req_01j7b8z4m0k9x8y100000001",
    "details": {}
  }
}
```

通过 `TestCanonicalErrorsIntegrityAgainstTaxonomy` 自动对齐全部 19 个标准错误码：

| 错误码 (`code`) | HTTP 状态码 | 所属领域 (`domain`) | 标准语义与处置指引 (`actionable`) |
| :--- | :---: | :--- | :--- |
| `ERR_PAYMENT_REQUIRED` | `402` | `PROTOCOL` | 资源需要 x402 支付授权；解析 `PAYMENT-REQUIRED` 挑战并附带合法 `PAYMENT-SIGNATURE` 重试。 |
| `ERR_WALLET_INSUFFICIENT_BALANCE` | `402` | `WALLET` | 虚拟钱包可用余额不足；请充值后再试，系统严禁静默降级为链上支付。 |
| `ERR_WALLET_FROZEN` | `403` | `WALLET` | 虚拟钱包处于冻结或欠费冻结状态；请联系管理员或结清欠款以恢复账户。 |
| `ERR_WALLET_AUTH_INVALID` | `401` | `WALLET` | 虚拟钱包授权无效、已过期、参数不匹配或已被消费；请获取新挑战后重试。 |
| `ERR_WALLET_AUTH_REQUIRED` | `401` | `WALLET` | 当前路由要求提供有效的虚拟钱包 API Key 以获取钱包计价挑战。 |
| `ERR_HOLD_EXPIRED` | `408` | `WALLET` | 内部钱包冻结租约（Hold）在上游完成前已过期；请使用新的冻结租约重试。 |
| `ERR_PAYMENT_METHOD_NOT_ALLOWED` | `403` | `ROUTING` | 当前路由策略不允许所请求的支付方式（请检查 `wallet`、`onchain` 或 `both` 配置）。 |
| `ERR_POLICY_BUDGET_EXCEEDED` | `403` | `POLICY` | 24 小时滚动预算或项目总预算已耗尽；请上调预算上限或等待滚动窗口释放。 |
| `ERR_POLICY_MAX_PER_TX` | `403` | `POLICY` | 单笔交易金额超出安全策略的 `max_amount_per_request` 限额。 |
| `ERR_POLICY_CIRCUIT_BROKEN` | `429` | `POLICY` | 检测到失控 Agent 循环调用或高频异常请求，已触发熔断器；请冷却后等待半开探测恢复。 |
| `ERR_API_KEY_INVALID` | `401` | `AUTH` | 提供的 API Key 无效、已吊销或已过期；请核对 Bearer Token 或在控制台签发新密钥。 |
| `ERR_API_KEY_SCOPE_DENIED` | `403` | `AUTH` | API Key 缺少当前操作所需的权限范围（如 `gateway:call`）。 |
| `ERR_TENANT_SUSPENDED` | `403` | `TENANT` | 租户组织账户已被暂停；请联系平台超级管理员恢复。 |
| `ERR_SIGNATURE_INVALID` | `401` | `CRYPTOGRAPHY` | EIP-712 / EIP-3009 支付签名校验失败或载荷被篡改。 |
| `ERR_NONCE_REPLAY` | `409` | `SETTLEMENT` | 密码学随机数（Nonce）已被消费或提交；请生成全新的 32 字节随机 Nonce。 |
| `ERR_IDEMPOTENCY_CONFLICT` | `409` | `IDEMPOTENCY` | 幂等键冲突（相同幂等键携带了不同请求载荷哈希，或同键请求正在并发执行中）。 |
| `ERR_UPSTREAM_TIMEOUT` | `504` | `GATEWAY` | 网关等待上游 API 响应超时；请携带完全相同的 `Idempotency-Key` 发起免费幂等重放。 |
| `ERR_UPSTREAM_FAILED` | `502` | `GATEWAY` | 上游服务返回 5xx 错误；请排查上游可用性并携带相同 `Idempotency-Key` 重试。 |
| `ERR_CONFIG_VERSION_EXPIRED` | `409` | `CONFIG` | 路由配置版本在执行期间已过期或被回滚；请刷新网关本地路由快照后重试。 |

### 3.3 配置发布与 RCU 快照一致性 (`specs/test-vectors/config-publishing.json`)

| 用例 ID | 类别 | 测试场景说明 | 预期裁定结果 | Go RCU 引擎 | Python 控制面 |
| :--- | :--- | :--- | :--- | :---: | :---: |
| `TC_CFG_01` | `PUBLISHING` | 合法路由与策略序列化并生成确定性 SHA-256 指纹 | `is_valid = true`, `target_status = active`, 包含 `sha256:` 前缀 | PASS | PASS |
| `TC_CFG_02` | `VALIDATION` | 路由包含非法上游 URL（`not_a_valid_http_uri`） | 发布前静态校验拦截，返回 `INVALID_UPSTREAM_URL` | PASS | PASS |
| `TC_CFG_03` | `VALIDATION` | 策略单笔限额（2,000,000）大于 24h 滚动预算上限（1,000,000） | 发布前静态校验拦截，返回 `MAX_AMOUNT_EXCEEDS_ROLLING_BUDGET` | PASS | PASS |
| `TC_CFG_04` | `ROLLBACK` | 从 `v2` 回滚至 `v1` 生成新版本 `v3`，且快照哈希与 `v1` 完全一致 | `new_version_number = 3`, `active_hash == v1_hash` (`ROLLBACK_SUCCESS`) | PASS | PASS |
| `TC_CFG_05` | `DEGRADATION` | 控制面返回 `500 UNAVAILABLE` 时，网关继续使用内存不可变快照路由 | `CONTINUE_SERVICE_WITH_LOCAL_CACHE`, `degradation_alert_logged = true` | PASS | PASS |

### 3.4 状态机矩阵、安全不变量守卫与复合视图 (`specs/conformance/state-machine.json`)

在 Go 侧 `services/gateway/internal/statemachine` 与 Python 侧共同执行 `specs/conformance/state-machine.json`（v2.1.0）：

1. **合法状态转移矩阵（35 条迁移规则）**：
   - `PaymentIntent`（8 个状态，13 条合法迁移）
   - `Settlement`（8 个状态，14 条合法迁移，完整覆盖链上与虚拟钱包双路径）
   - `Fulfillment`（5 个状态，8 条合法迁移）
   - 全部 35 个 `(from, event, to)` 状态迁移在 Go 与 Python 中产出完全一致的目标状态，未列入矩阵的非法迁移均被确定性拦截。
2. **六大核心安全不变量守卫（`INV_1` ~ `INV_6`）**：
   - `INV_1_DISPATCH_GUARD`（双轨调度守卫）：除非满足 `(path == onchain AND settlement == confirmed)` 或 `(path == wallet AND settlement == held)`，否则严禁向上游分发履约请求。
   - `INV_2_UNKNOWN_SUBMIT_GUARD`（未知状态防重提守卫）：当链上 `settlement == unknown` 时，严禁盲目重新提交交易，仅允许轮询链上状态。
   - `INV_3_TERMINAL_IMMUTABLE`（终态不可变守卫）：所有标记为 `is_terminal == true` 的状态（`payment_intent: rejected/expired/completed/failed`、`settlement: confirmed/released/failed`、`fulfillment: succeeded/failed`）严格拒绝任何后续状态变更事件。
   - `INV_4_UPSTREAM_5XX_UNKNOWN`（上游 5xx/超时归入未知态守卫）：上游返回 `5xx` 或网络超时必须使 `fulfillment` 进入 `unknown` 状态，严禁直接判定为 `failed`。
   - `INV_5_WALLET_UNKNOWN_HOLD_GUARD`（履约未知态冻结保持守卫）：当 `path == wallet` 且 `fulfillment == unknown` 时，钱包冻结记录必须保持在 `held` 状态，在恢复协程确认真实结果前严禁自动实扣或自动释放。
   - `INV_6_NO_AUTO_DOWNGRADE`（严禁静默降级守卫）：当虚拟钱包余额不足时，网关必须立即返回 `402 ERR_WALLET_INSUFFICIENT_BALANCE`，严禁静默降级为链上支付挑战。
3. **复合状态视图映射（Composite Views）**：

| `Settlement` 状态 | `Fulfillment` 状态 | 推导出的用户端复合状态 (`display`) |
| :--- | :--- | :--- |
| `held` | `not_started`, `in_progress` | `WALLET_HOLD_PROCESSING` |
| `unknown`, `pending`, `submitted` | `not_started` | `SETTLEMENT_IN_PROGRESS` |
| `confirmed` | `in_progress` | `FULFILLMENT_IN_PROGRESS` |
| `confirmed` | `succeeded` | `COMPLETED` |
| `confirmed` | `failed` | `SETTLED_FULFILLMENT_FAILED_REFUND_REQUIRED` |
| `confirmed` | `unknown` | `SETTLED_FULFILLMENT_UNKNOWN_NEEDS_REVIEW` |
| `released` | `failed` | `RELEASED_UNBILLED` |
| `failed` | `not_started` | `PAYMENT_FAILED` |

---

## 4. 发现并修复的行为差异清单

在跨语言一致性测试落地过程中，共识别并修复了以下 4 项跨语言实现差异：

1. **Canonical Error Taxonomy 命名对齐与向后兼容别名**：
   - *差异*：Go 网关早期定义了 `ERR_NONCE_ALREADY_USED` 与 `ERR_UPSTREAM_BAD_GATEWAY`，而规范标准定义为 `ERR_NONCE_REPLAY` (409, `SETTLEMENT`) 与 `ERR_UPSTREAM_FAILED` (502, `GATEWAY`)，且早期缺少 `ERR_HOLD_EXPIRED`、`ERR_CONFIG_VERSION_EXPIRED`、`ERR_WALLET_AUTH_INVALID` 与 `ERR_WALLET_AUTH_REQUIRED`。
   - *修复*：完善 `services/gateway/internal/x402/errors.go`，补齐全量 19 个标准错误码及其 Actionable 指引字段，并保留向后兼容常量别名以确保既有调用零破坏。
2. **402 出题 Network 动态化与 Fallback 机制**：
   - *差异*：Go 网关原先在下发 402 Challenge 时硬编码了 `"eip155:84532"`，未优先读取路由定义中的 `route.Network`。
   - *修复*：在 `rcu.RouteConfig` 与 `config.RouteDefinition` 中增加 `Network` 字段，出题时优先采用路由配置的 `route.Network`，空值时回退至全局 `cfg.Settlement.DefaultNetwork`（如 `eip155:421614` Arbitrum Sepolia）。
3. **大小写不敏感 HTTP 标头与 EVM 地址规范化比较**：
   - *差异*：Python 字典对 HTTP Header 大小写敏感导致读取 `x-agentpay-config-version` 偶发漏检；EVM 地址在 Python SDK 中规范化为全小写，而 Go 保留了 EIP-55 混合大小写校验和。
   - *修复*：端到端测试与客户端 SDK 统一采用符合 RFC 7230 的大小写不敏感标头提取与十六进制以太坊地址标准化比对。
4. **预设测试钱包账户与隔离初始化**：
   - *优化*：在 `services/gateway/internal/server` 默认初始化构造中内置正常激活（`active`）、余额不足（`insufficient`）与欠费冻结（`frozen_arrears`）三类预设测试钱包，支持本地开发与跨语言端到端集成测试即开即测。

---

## 5. 复现命令与验收结论

可通过以下标准命令在本地完整复现跨语言一致性测试套件：

```bash
# 1. 校验 JSON Schema 与跨语言测试向量完整性
make vectors

# 2. 运行 Python 单元、状态机矩阵及跨语言进程级集成测试
make test

# 3. 运行 Go Gateway 单元、RCU 快照及 x402 协议一致性测试
make gateway-test
```

全仓验证结论：
- **Python 测试**：373+ 项测试（含 6 项真实进程级跨语言 HTTP 互操作测试）— **100% 通过 (PASS)**；
- **Go 测试**：全量单元、状态机矩阵、RCU 快照与 x402 协议测试 — **100% 通过 (PASS)**；
- **测试向量校验**：`specs/test-vectors` 与 `specs/conformance` 下所有跨语言 JSON 测试向量全部通过；
- **代码质量检查**：`ruff check` 与 `ruff format --check` 零警告通过。

**任务 T2.6（Python/Go 协议一致性测试）各项验收标准全部达成，正式交付。**
