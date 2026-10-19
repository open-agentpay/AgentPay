# AgentPay Python/Go 协议一致性契约规范 (T1.3 Specification)

> **版本**：v2.0.0 (T1.3 统一协议契约)  
> **状态**：**已定稿 (Approved Specification)**  
> **适用范围**：Python Client/Buyer SDK、Go Gateway 数据面、控制面 API 与对账系统  
> **跨语言测试向量**：  
> - [`specs/conformance/protocol-errors.json`](../../specs/conformance/protocol-errors.json) (统一错误码表与 Schema)  
> - [`specs/test-vectors/dual-path-protocol.json`](../../specs/test-vectors/dual-path-protocol.json) (双轨协议一致性测试向量)  
> - [`specs/conformance/state-machine.json`](../../specs/conformance/state-machine.json) (状态机迁移向量)  

---

## 1. 目标与设计原则

为确保 **Python 买方客户端（Buyer SDK）** 与 **Go 云原生网关（Go Gateway）** 在网络交互、错误处理与状态裁定时行为 100% 同构，T1.3 规范制定统一的协议层通信契约：

1. **单点事实（Single Source of Truth）**：所有协议头、错误码、状态枚举与测试向量均以机器可读的 JSON 存储在 `specs/` 中，Python 与 Go 共同读取运行相同的测试套件；
2. **严密格式化（Case-Insensitive & Schema-Strict）**：HTTP 头部字段名称遵循大小写不敏感解析，载荷格式遵循严格 JSON Schema 验证，未知或非法字段一律安全拦截；
3. **确定性支付方式裁定**：明确区分链上 x402 签名支付与内部虚拟钱包扣费，**严禁在余额不足时静默或自动降级为链上扣费**；
4. **统一错误响应模型**：全系统遵循一致的 JSON 错误响应格式，避免客户端对不同后端的错误进行分支适配。

---

## 2. HTTP 协议头契约 (Headers Specification)

### 2.1 链上 x402 支付通道
| Header 名称 | 作用方 | 语法与编码 | 规范定义 |
|---|---|---|---|
| `PAYMENT-REQUIRED` | Gateway → Client (402) | `PAYMENT-REQUIRED: <Base64URL(JSON_Payload)>` | RFC 6648 兼容，携带出题方挑战明细 |
| `PAYMENT-SIGNATURE`| Client → Gateway (重发) | `PAYMENT-SIGNATURE: <Base64URL(JSON_Payload)>` | 包含买方钱包针对 EIP-3009 的签名凭证 |

### 2.2 虚拟钱包支付通道
| Header 名称 | 作用方 | 语法与格式 | 规范定义 |
|---|---|---|---|
| `Authorization` | Client → Gateway | `Authorization: Bearer apk_live_...` | 传递租户签发的 API Key |
| `Idempotency-Key` | Client → Gateway | `Idempotency-Key: <string>` | 客户端业务级唯一键（最大 128 字符） |

---

## 3. 双轨支付方式裁定与选择规则 (Payment Method Arbitration)

当 Gateway 收到请求时，遵循以下确定性状态机裁定流：

1. 若请求头包含 `Authorization: Bearer apk_...`：
   - 检查路由是否允许 `wallet` 或 `both`。若不允许，返回 `403 ERR_PAYMENT_METHOD_NOT_ALLOWED`；
   - 检查钱包状态。若为 `frozen_arrears`，返回 `403 ERR_WALLET_FROZEN`；
   - 检查可用余额。**若可用余额不足，返回 `402 ERR_WALLET_INSUFFICIENT_BALANCE`，绝对不自动降级为链上扣费**；
   - 额度充足时，执行 Hold 冻结额度，调用上游服务。
2. 若请求头不包含 API Key 但包含 `PAYMENT-SIGNATURE`：
   - 检查路由是否允许 `onchain` 或 `both`。若不允许，返回 `403 ERR_PAYMENT_METHOD_NOT_ALLOWED`；
   - 执行 EIP-712/3009 验签并走链上结算通道。
3. 若两者均无：
   - 签发 `402 PAYMENT-REQUIRED` 挑战，等待客户端签名。

---

## 4. 全量标准错误码表 (Canonical Error Taxonomy)

详见标准错误码规格：[`specs/conformance/protocol-errors.json`](../../specs/conformance/protocol-errors.json)。

---

## 5. 标识符规则与幂等一致性

- `request_id`：必须以 `req_` 为前缀或符合 UUIDv4 标准；
- `idempotency_key`：客户端提供的业务幂等键，以 `(tenant_id, idempotency_key)` 为联合租户唯一键；
- `payment_intent_id`：以 `pi_` 为前缀；
- `settlement_id`：以 `stl_` 为前缀；
- `fulfillment_id`：以 `ful_` 为前缀；
- `hold_id`：以 `hld_` 为前缀。
