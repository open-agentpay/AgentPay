# AgentPay 统一支付状态机与故障恢复规范 (T1.1 Specification v2.1)

> **版本**：v2.1.0 (T1.1 统一规范：双轨结算架构)  
> **状态**：**已定稿 (Approved Specification)**  
> **前置任务**：T0.1 / T0.2 (基线已冻结：`v0.1.0-poc-baseline`)  
> **相关规范**：[specs/wallet-ledger-spec.md](./wallet-ledger-spec.md) (T1.7 虚拟钱包与账本规范)  
> **机器可读向量**：[`specs/conformance/state-machine.json`](../conformance/state-machine.json)

---

## 1. 核心设计原则

1. **三条状态线严格正交分离**：
   - **支付意图（PaymentIntent）**：业务层端到端请求生命周期；
   - **结算（Settlement）**：资金划转状态，支持**双轨支付方式（Payment Method）**：
     - `onchain`（链上结算）：x402 挑战签名、区块链广播与确认；
     - `wallet`（平台内部虚拟钱包）：强事务资金冻结（Hold）、确权扣费（Capture）与解冻释放（Release）。
   - **业务履约（Fulfillment）**：上游目标 API 资源/结果实际交付状态。
2. **`unknown` 为一等状态**：
   - 结算或履约结果不确定时，**严禁直接归为成功，也严禁直接归为失败**。
   - 钱包扣费在数据库本地强事务中完成，本身无结算 unknown；但**若履约 unknown，钱包冻结必须保持锁定并转 `needs_review`**，严禁私自释放或扣款。
3. **终态绝对不可变（Terminal State Immutability）**：
   - 进入终态（`completed`, `failed`, `expired`, `rejected`, `confirmed`, `released`）后状态只读，只允许追加不可变审计事件。
4. **禁止盲目重试与禁止静默降级**：
   - 对 `unknown` 状态严禁重复发起扣款或重新广播；
   - **严禁自动降级**：若钱包余额不足，必须直接返回 `402 Insufficient Balance`，绝对不能静默降级为链上扣费，防止用户产生非预期的链上资金支出。

---

## 2. 标识与多租户幂等契约

| 标识符 | 作用域与责任 | 约束规则 |
|---|---|---|
| `request_id` | 单次 HTTP 请求链路追踪 | 网关在入站时生成（UUIDv4），贯穿全链路结构化日志 |
| `idempotency_key` | 客户端业务级幂等重试 | 租户级唯一。同一租户内相同的 key 严格映射到同一个 `payment_intent_id` |
| `payment_intent_id` | 一笔高层支付意图 | 一个 `idempotency_key` 唯一对应一个 intent（格式：`pi_...`） |
| `hold_id` | 虚拟钱包单笔冻结单号 | 钱包路径专属，一个 intent 唯一绑定一个有效 Hold（格式：`hld_...`） |
| `settlement_id` | 一次结算记录 | 一个 intent 在同一时刻最多对应一个有效结算（格式：`stl_...`） |
| `fulfillment_id` | 一次上游业务交付尝试 | 一个 intent 严格对应一个履约记录（格式：`ful_...`） |

---

## 3. 双轨结算与履约状态流转

### 3.1 链上结算路径 (On-Chain Path)
遵循 **方案 A（先结算后履约）**：
```text
authorized ──▶ submit ──▶ [submitted] ──▶ [pending] ──▶ [confirmed]
                                                             │ (dispatch)
                                                             ▼
                                                    [fulfillment: in_progress]
                                                             ├─ 2xx ─▶ completed
                                                             ├─ 4xx ─▶ refund_required
                                                             └─ 5xx ─▶ needs_review
```

### 3.2 虚拟钱包结算路径 (Internal Wallet Path)
遵循 **“冻结 → 履约 → 确认扣费/释放”** 闭环：
```text
authorized ──▶ hold ──▶ [held] ──(dispatch)──▶ [fulfillment: in_progress]
                                                        ├─ 2xx ─▶ capture ──▶ confirmed (完成)
                                                        ├─ 4xx ─▶ release ──▶ released (未扣费释放)
                                                        └─ 5xx ─▶ 保持 held ──▶ needs_review (人工核实)
```

**双轨对比优势**：
- 在钱包路径中，上游明确失败（4xx）时直接在库内事务中 `release` 释放冻结，**完全不需要复杂的退款/补偿流程**，买方零资金损失，卖方亦不会白送服务。

---

## 4. 三维状态定义与合法迁移矩阵

### 4.1 支付意图 (PaymentIntent)

| 当前状态 | 触发事件 | 目标新状态 | 说明与前置守卫 |
|---|---|---|---|
| `created` | `challenge_issued` | `challenged` | 仅链上路径：402 响应送出 |
| `created` | `payload_verified` | `authorized` | 仅钱包路径：API Key 及预算校验通过 |
| `created` | `policy_denied` | `rejected` | 策略拒绝、风控拦截或钱包欠费冻结 |
| `challenged` | `payload_verified` | `authorized` | 链上路径：买方携带合法签名重新请求 |
| `challenged` | `payload_invalid` | `rejected` | 签名无效、篡改或格式错误 |
| `challenged` | `timeout` | `expired` | 超过挑战 TTL（默认 300 秒）未提交 |
| `authorized` | `settlement_confirmed` | `completed` | 链上 confirmed 且履约成功，或钱包 capture 成功 |
| `authorized` | `settlement_failed` | `failed` | 链上结算失败，或钱包冻结额度不足 |
| `authorized` | `settlement_released` | `failed` | 钱包路径：上游失败解冻，意图正常关闭未计费 |
| `authorized` | `authorization_expired`| `expired` | 链上授权过期且结算尚未开始 |
| `authorized` | `recovery_deadline_exceeded`| `needs_review` | 结算或履约持续 unknown 超过恢复期限 |
| `needs_review`| `manual_resolved_confirmed`| `completed`| 人工介入核实已到账并交付 |
| `needs_review`| `manual_resolved_failed` | `failed` | 人工介入核实未交付且关闭意图 |

### 4.2 结算 (Settlement)

| 当前状态 | 触发事件 | 目标新状态 | 对应支付方式与规则 |
|---|---|---|---|
| `not_started` | `hold_acquired` | `held` | 【钱包】在数据库事务中成功冻结额度 |
| `not_started` | `hold_failed` | `failed` | 【钱包】可用余额不足或账户冻结 |
| `held` | `capture` | `confirmed` | 【钱包】上游 2xx，确认扣费入账（终态） |
| `held` | `release` | `released` | 【钱包】上游 4xx，解冻恢复可用余额（终态） |
| `not_started` | `submit_ok` | `submitted` | 【链上】广播成功获取 tx_hash |
| `not_started` | `submit_rejected_definitely` | `failed` | 【链上】节点直接拒绝 |
| `not_started` | `submit_timeout` | `unknown` | 【链上】提交超时，状态未知 |
| `submitted` | `seen_on_chain` | `pending` | 【链上】交易进入内存池或已出块 |
| `submitted` | `query_timeout` | `unknown` | 【链上】查询超时 |
| `pending` | `confirmations_reached` | `confirmed` | 【链上】达到 1 个区块确认（终态） |
| `pending` | `reverted` | `failed` | 【链上】交易执行 Revert 回滚（终态） |
| `pending` | `query_timeout` | `unknown` | 【链上】状态轮询超时 |
| `unknown` | `query_found_confirmed` | `confirmed` | 【链上】对账核实已打包确认 |
| `unknown` | `query_found_failed` | `failed` | 【链上】对账核实已失败且过期未花费 |
| `unknown` | `query_found_pending` | `pending` | 【链上】重新查到待确认交易 |
| `unknown` | `query_still_unknown` | `unknown` | 【链上】状态依然未知 |

### 4.3 业务履约 (Fulfillment)

| 当前状态 | 触发事件 | 目标新状态 | 说明与安全守卫 |
|---|---|---|---|
| `not_started` | `dispatch` | `in_progress` | 链上要求 `settlement==confirmed`；钱包要求 `settlement==held` |
| `in_progress` | `upstream_2xx` | `succeeded` | 上游交付成功（触发钱包 capture） |
| `in_progress` | `upstream_4xx` | `failed` | 上游客户端入参错误未执行（触发钱包 release） |
| `in_progress` | `upstream_5xx` | `unknown` | 上游服务端异常，不可假定未执行（钱包保持 held） |
| `in_progress` | `upstream_timeout`| `unknown` | 连接断开或读超时（钱包保持 held） |
| `unknown` | `upstream_status_query_ok` | `succeeded` | 幂等查询证实已成功（补发 capture） |
| `unknown` | `upstream_status_query_failed` | `failed` | 幂等查询证实未执行（补发 release） |
| `unknown` | `recovery_deadline_exceeded` | `unknown` | 超过最大恢复期，标记待人工核查 |

---

## 5. 对外派生组合视图（Composite View）

| 结算状态 (Settlement) | 履约状态 (Fulfillment) | 派生展示状态 (Display Status) | 业务解释 |
|---|---|---|---|
| `held` | `not_started` / `in_progress` | `WALLET_HOLD_PROCESSING` | 钱包已预留额度，正在调用上游服务 |
| `unknown` / `pending` / `submitted` | `not_started` | `SETTLEMENT_IN_PROGRESS` | 链上支付确认中 |
| `confirmed` | `in_progress` | `FULFILLMENT_IN_PROGRESS` | 扣费已确认，服务交付中 |
| `confirmed` | `succeeded` | `COMPLETED` | 交易成功完成 |
| `confirmed` | `failed` | `SETTLED_FULFILLMENT_FAILED_REFUND_REQUIRED` | 链上已扣款但履约失败，待补偿退款 |
| `confirmed` | `unknown` | `SETTLED_FULFILLMENT_UNKNOWN_NEEDS_REVIEW` | 扣款已确认，履约异常待排查 |
| `released` | `failed` | `RELEASED_UNBILLED` | 钱包额度已释放，未产生任何实际扣款 |
| `failed` | `not_started` | `PAYMENT_FAILED` | 支付失败，未扣款 |
