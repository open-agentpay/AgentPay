# API & Protocol Specifications

## 1. x402 V2 HTTP Protocol Headers

AgentPay uses standard `HTTP 402 Payment Required` status codes and structured protocol headers for seamless, machine-readable payment negotiation.

### 1.1 `PAYMENT-REQUIRED` Response Header (Server -> Client)

When a client requests a protected endpoint without valid payment authorization, the server responds with `HTTP 402` containing a Base64URL-encoded JSON challenge in the `PAYMENT-REQUIRED` header.

#### JSON Challenge Schema:
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

#### Fields Description:
| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `version` | string | Yes | Protocol version, fixed to `"2.0"`. |
| `challengeId` | string | Yes | Globally unique challenge ID (UUIDv7/CUID). |
| `network` | string | Yes | CAIP-2 blockchain network identifier (e.g. `eip155:421614` for Arbitrum Sepolia). |
| `asset` | string | Yes | Payment token contract address (EVM 0x format). |
| `assetSymbol` | string | No | Human-readable token symbol (e.g. `"USDC"`). |
| `decimals` | number | Yes | Asset token decimals (typically 6 for USDC). |
| `amount` | string | Yes | Required amount in atomic units (e.g. `"100000"` = 0.10 USDC). |
| `payee` | string | Yes | Recipient payout address. |
| `scheme` | string | Yes | Authorization signing scheme: `"EIP-3009"` or `"EIP-712"`. |
| `validAfter` | number | Yes | Unix timestamp (seconds) when authorization becomes valid. |
| `validBefore` | number | Yes | Unix timestamp (seconds) when authorization expires (typically 3-5 min window). |
| `facilitatorUrl`| string | Yes | URL of recommended Facilitator node for verification and gas sponsorship. |
| `resourceUri` | string | No | Protected resource path. |

---

### 1.2 `PAYMENT-SIGNATURE` Request Header (Client -> Server)

After validating local policy rules and generating the EIP-712/3009 signature, the buyer agent encodes the authorization payload as Base64URL and supplies it in the `PAYMENT-SIGNATURE` header.

#### JSON Signature Payload Schema:
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

## 2. Facilitator Verification & Settlement API

### 2.1 Verify Authorization

Verifies cryptographic signatures, checks token balances, and prevents nonce replays without broadcasting to the mempool.

* **Endpoint**: `POST /v1/verify`
* **Content-Type**: `application/json`

#### Request Body:
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

#### Successful Response (`HTTP 200 OK`):
```json
{
  "valid": true,
  "verifiedPayer": "0xBuyerAgentAddress...",
  "verifiedAt": 1729000010,
  "verificationToken": "vtok_01j7abc..."
}
```

#### Common Error Codes:
* `400 Bad Request`: Malformed payload or missing required parameters.
* `401 Unauthorized`: Recovered signer address does not match `payer`.
* `409 Conflict`: Nonce has already been consumed (anti-replay guard).
* `410 Gone`: Signature expiration timestamp (`validBefore`) has passed.

---

### 2.2 Submit Settlement

Once fulfillment succeeds, the gateway asynchronously submits the verified authorization to the Facilitator for on-chain batching.

* **Endpoint**: `POST /v1/settle`
* **Content-Type**: `application/json`

#### Request Body:
```json
{
  "verificationToken": "vtok_01j7abc...",
  "authorization": { ... },
  "idempotencyKey": "idem_01j7def..."
}
```

#### Response (`HTTP 202 Accepted`):
```json
{
  "settlementId": "stl_01j7xyz...",
  "status": "SUBMITTED",
  "queuePosition": 4
}
```
