# AgentPay 接口规格文档 (API Specifications)

## 1. x402 V2 HTTP 协议头规范

AgentPay 在 HTTP 应用层采用标准的 `402 Payment Required` 状态码与结构化自定义头部进行无缝交互。

### 1.1 `PAYMENT-REQUIRED` 响应头 (服务端返回)

当客户端未携带有效支付凭据访问受保护的端点时，服务端返回 `HTTP 402` 并在 `PAYMENT-REQUIRED` 响应头中携带 Base64URL 编码的 JSON 挑战载荷。

#### JSON 载荷 Schema:
```json
{
  "version": "2.0",
  "challengeId": "ch_01j7xyz...",
  "network": "eip155:421614",
  "asset": "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d",
  "assetSymbol": "USDC",
  "decimals": 6,
  "amount": "100000",
  "payee": "0xSellerPayoutAddress...",
  "scheme": "EIP-3009",
  "validAfter": 1729000000,
  "validBefore": 1729000300,
  "facilitatorUrl": "https://facilitator.agentpay.dev",
  "resourceUri": "/v1/market-data"
}
```

#### 字段详细说明:
| 字段 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `version` | string | 是 | 协议版本，固定为 `"2.0"`。 |
| `challengeId` | string | 是 | 本次支付挑战的全局唯一 ID (UUIDv7/CUID)。 |
| `network` | string | 是 | CAIP-2 格式的区块链网络 ID（如 `eip155:421614` 为 Arbitrum Sepolia）。 |
| `asset` | string | 是 | 支付资产的合约地址（EVM 0x 格式）。 |
| `assetSymbol` | string | 否 | 代币资产标识符号，便于展示与审计（如 `"USDC"`）。 |
| `decimals` | number | 是 | 资产精度位数（USDC 通常为 6）。 |
| `amount` | string | 是 | 需支付的最小基数单位（字符串避免精度丢失，如 `"100000"` 表示 0.1 USDC）。 |
| `payee` | string | 是 | 卖方收款的链上地址。 |
| `scheme` | string | 是 | 支付签名模式：`"EIP-3009"` 或 `"EIP-712"`。 |
| `validAfter` | number | 是 | 授权生效的 Unix 时间戳（秒）。 |
| `validBefore` | number | 是 | 授权失效的 Unix 时间戳（秒，通常有效窗口为 3~5 分钟）。 |
| `facilitatorUrl`| string | 是 | 推荐/指定的 Facilitator 清算节点地址。 |
| `resourceUri` | string | 否 | 被保护的目标资源路径。 |

---

### 1.2 `PAYMENT-SIGNATURE` 请求头 (买方客户端重放)

买方 Agent 在完成策略评估并生成 EIP-712/3009 授权签名后，将凭据 Base64URL 编码后放入请求的 `PAYMENT-SIGNATURE` 头部。

#### JSON 载荷 Schema:
```json
{
  "version": "2.0",
  "challengeId": "ch_01j7xyz...",
  "payer": "0xBuyerAgentAddress...",
  "scheme": "EIP-3009",
  "nonce": "0x4a9b...32bytesHex",
  "validAfter": 1729000000,
  "validBefore": 1729000300,
  "signature": "0xcompactSignatureHex..."
}
```

---

## 2. Facilitator 协调清算服务 REST API

### 2.1 校验支付授权接口 (Verify Authorization)

提供给卖方网关快速核验买方提交的签名、余额及 Nonce 是否合法，无需直接广播上链即可确权。

* **Endpoint**: `POST /v1/verify`
* **Content-Type**: `application/json`

#### 请求参数 (Request Body):
```json
{
  "challengeId": "ch_01j7xyz...",
  "authorization": {
    "network": "eip155:421614",
    "asset": "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d",
    "payer": "0xBuyerAgentAddress...",
    "payee": "0xSellerPayoutAddress...",
    "amount": "100000",
    "validAfter": 1729000000,
    "validBefore": 1729000300,
    "signature": "0xcompactSignature..."
  }
}
```

#### 成功响应 (HTTP 200 OK):
```json
{
  "valid": true,
  "verifiedPayer": "0xBuyerAgentAddress...",
  "verifiedAt": 1729000010,
  "verificationToken": "vtok_01j7abc..."
}
```

#### 常见失败状态码:
* `400 Bad Request`：参数缺失或格式非法。
* `401 Unauthorized`：EIP-712 签名反解出的地址与 `payer` 不符。
* `409 Conflict`：该 `nonce` 已被提交或消费过（防重放触发）。
* `410 Gone`：当前时间已超过 `validBefore` 截止期。

---

### 2.2 提交链上结算接口 (Submit Settlement)

卖方在确认资源交付成功后，异步将签名提交至 Facilitator 请求上链。

* **Endpoint**: `POST /v1/settle`
* **Content-Type**: `application/json`

#### 请求参数 (Request Body):
```json
{
  "verificationToken": "vtok_01j7abc...",
  "authorization": { ... },
  "idempotencyKey": "idem_01j7def..."
}
```

#### 响应 (HTTP 202 Accepted):
```json
{
  "settlementId": "stl_01j7xyz...",
  "status": "SUBMITTED",
  "queuePosition": 4
}
```
