# AgentPay Virtual Wallet x402 Extension

> **Status:** Draft implementation contract  
> **Scope:** Python buyer SDK and Go Gateway virtual-wallet route  
> **Relationship to x402:** This document defines an AgentPay-specific payment scheme carried in the existing x402 V2 HTTP challenge and payment headers. `agentpay-wallet` is not the x402 EIP-3009 scheme and is not interoperable with an EIP-3009 facilitator.

## 1. Goal

Require the buyer SDK to inspect the seller's price and obtain local policy approval before a virtual-wallet request can cause a hold or debit. The lifecycle is:

```text
request with AgentPay API key
  -> 402 PAYMENT-REQUIRED (scheme=agentpay-wallet)
  -> SDK validates challenge and reserves local budget
  -> retry with the same API key and PAYMENT-SIGNATURE wallet authorization
  -> Gateway validates and atomically consumes the challenge
  -> wallet Hold -> upstream fulfillment -> Capture / Release / Needs review
  -> SDK commits its reservation on success
```

The client API key authenticates the wallet. The wallet scheme does not create an EIP-712 signature, initiate an on-chain transfer, or call a facilitator. The header name `PAYMENT-SIGNATURE` is retained for transport uniformity; the payload is an AgentPay wallet authorization, not a cryptographic signature.

## 2. Scheme and challenge

The scheme identifier is exactly `agentpay-wallet`. Header names and Base64URL JSON encoding remain those in the existing AgentPay protocol contract.

Example `PAYMENT-REQUIRED` payload:

```json
{
  "version": "2.0",
  "challengeId": "ch_01J7XYZ890ABCDEF12345678",
  "scheme": "agentpay-wallet",
  "network": "agentpay:internal-ledger",
  "asset": "USD",
  "assetSymbol": "USD",
  "decimals": 6,
  "amount": "50000",
  "payee": "agentpay:route:rt_01J7XYZ890ABCDEF12345678",
  "validAfter": 1792540800,
  "validBefore": 1792541100,
  "resourceUri": "/v1/llm/chat",
  "extensions": {
    "paymentMethod": "wallet",
    "routeId": "rt_01J7XYZ890ABCDEF12345678",
    "requestBinding": "sha256:..."
  }
}
```

Rules:

1. `amount` is a positive decimal integer string in the route's configured wallet minor units; floating-point values are forbidden. `decimals` is fixed at `6` across all supported tenant base fiat currencies (`CNY`, `USD`, `EUR`, where `1 unit = 1,000,000 base units`; see [`currency-and-multichain-spec.md`](./currency-and-multichain-spec.md)). The challenge amount is informational until the gateway rechecks it against the active route during authorization.
2. `network` is the AgentPay namespace `agentpay:internal-ledger`; it does not name a blockchain. `asset` is the tenant's configured base currency code (`CNY`, `USD`, or `EUR`; legacy `USDC` is accepted for backward compatibility), and `assetSymbol` is the corresponding display symbol or code (`¥`, `$`, `€`, or `CNY`/`USD`/`EUR`).
3. `payee` is the non-secret route resource identifier shown above; it is not an EVM address and must not be interpreted as one.
4. `resourceUri`, `routeId`, and `requestBinding` bind approval to the original request. Compute `bodyDigest = lowercase_hex(SHA256(raw_body_bytes))`, then `requestBinding = "sha256:" + lowercase_hex(SHA256(UTF8(method_upper + "\n" + escaped_path + "\n" + raw_query + "\n" + bodyDigest)))`. `escaped_path` is the path component with percent escapes preserved exactly as received (empty becomes `/`); `raw_query` excludes `?` and preserves original ordering/escaping (empty when absent). Go uses `URL.EscapedPath()` and `URL.RawQuery`; Python uses the raw path/query bytes from `httpx.URL.raw_path`, splitting at the first `?`. Both implementations must use the original request bytes before any body parsing or rewriting. Shared conformance vectors are required for encoded paths, repeated query keys, empty body, and non-ASCII body bytes.
5. Challenge lifetime is at most 300 seconds. Gateway time is authoritative. A missing, expired, unknown, or already consumed challenge fails closed.
6. The challenge is issued only when the route allows `wallet` or `both`. For an `onchain`-only route, API-key wallet authentication returns `ERR_PAYMENT_METHOD_NOT_ALLOWED`.

`extensions` is an AgentPay extension object. Implementations must preserve unknown extension values when decoding, while validating the required fields above.

## 3. Wallet authorization

The buyer retries the original HTTP request with its original `Authorization: Bearer <AGENT_KEY>` or `X-Agent-Key` and this `PAYMENT-SIGNATURE` payload:

```json
{
  "version": "2.0",
  "challengeId": "ch_01J7XYZ890ABCDEF12345678",
  "scheme": "agentpay-wallet",
  "nonce": "0x<32 random bytes as 64 lowercase hex characters>",
  "validAfter": 1792540800,
  "validBefore": 1792541100,
  "amount": "50000",
  "resourceUri": "/v1/llm/chat",
  "requestBinding": "sha256:..."
}
```

The wallet authorization intentionally has no `payer` or `signature`: wallet identity comes only from the authenticated API key, and the local SDK's approved reservation is the authorization decision. `nonce` is generated with a cryptographically secure RNG and is an additional replay key; it does not prove buyer identity.

The SDK must:

1. Select handling by `challenge.scheme` before invoking a signer. For `EIP-3009`, retain the existing signing path. For `agentpay-wallet`, do not invoke `ISigner` or produce EIP-712 data.
2. Validate scheme, amount, resource, validity window, and request binding; reserve exactly the challenge amount and bind the reservation to `challengeId`.
3. Build the wallet authorization only after approval, inject it into `PAYMENT-SIGNATURE`, and replay the same request while preserving its API key and idempotency key.
4. Commit the local reservation only after a successful 2xx response. On a definitive pre-fulfillment rejection, release according to the existing policy lifecycle. On transport ambiguity or 5xx after the authorization was sent, retain the reservation and record failure; never automatically release it.

## 4. Gateway validation and consumption

For an API-key request without a wallet authorization, the gateway must authenticate the key, resolve the route and exact server-side price, create a durable challenge/intent, and return 402 with `PAYMENT-REQUIRED`. It must not place a wallet hold or call upstream at this stage.

For the retry, the gateway must, in this order:

1. Authenticate the same API key and confirm that the route still permits wallet payment.
2. Parse `PAYMENT-SIGNATURE` as the `agentpay-wallet` payload. Reject an authorization using any other scheme on the wallet path.
3. Load the durable challenge by `challengeId`; bind it to tenant, API key identity (never store the raw key), route ID, request binding, server-computed price, asset, and validity window. Recompute price and request binding; reject any mismatch. Client-supplied amount is never the debit source.
4. Atomically transition the challenge from `issued` to `consuming` and reserve its nonce. Enforce one-time use across all Gateway instances using PostgreSQL uniqueness/transactional state, not an in-process map. A duplicate authorization must not create another hold or dispatch another upstream request.
5. In the same safe payment lifecycle, create the payment intent and wallet hold before upstream dispatch. On 2xx, capture once; on definitive upstream 4xx, release once; on 5xx, timeout, or ambiguous delivery, retain the hold and mark `needs_review` under the existing wallet-ledger invariant.
6. Return the challenge/authorization error without falling back to on-chain payment. Insufficient wallet balance remains `402 ERR_WALLET_INSUFFICIENT_BALANCE` and must not issue an EIP-3009 challenge.

### Durable state requirements

The implementation must add a durable wallet challenge record or an equivalent transactionally safe extension of the payment-intent model. At minimum it stores:

- challenge ID (unique primary key), tenant ID, API-key record ID, route ID, payment intent ID;
- request binding, server-computed amount and asset, creation/expiry timestamps;
- status `issued | consuming | consumed | rejected | expired`;
- nonce uniqueness scoped to wallet/network and challenge, with a database uniqueness constraint;
- resulting hold ID and idempotency association when consumption succeeds.

Challenge consumption, intent transition, nonce reservation, and hold creation must be atomic or recoverable through an explicit state machine. A process-local cache is only an optimization and cannot be the source of replay protection. Challenge records may be deleted only after expiry plus the configured audit/recovery retention period.

## 5. Compatibility and payment selection

1. Wallet API-key calls no longer debit directly on the first request. They receive a challenge and require a compliant SDK (or another client implementing this extension) to approve and replay.
2. Existing `EIP-3009` requests continue through the chain payment path. An API key plus an EIP-3009 authorization is never silently reinterpreted as wallet authorization; scheme and route policy decide the path deterministically.
3. `wallet`, `onchain`, and `both` remain the route configuration values. `both` means the client explicitly chooses a path by presenting wallet credentials or an EIP-3009 authorization; the gateway never downgrades between them.
4. Direct credential metering/allow modes in the Python SDK remain an explicit legacy escape hatch, are not x402 wallet payments, and must not be presented as price-aware wallet mode.

## 6. Error behavior

Use existing protocol error envelopes and codes where applicable:

| Condition | HTTP | Code |
|---|---:|---|
| First wallet request requires approval | 402 | `ERR_PAYMENT_REQUIRED` plus wallet challenge |
| Wallet-only route receives no API key | 401 | `ERR_WALLET_AUTH_REQUIRED` |
| Invalid, unknown, expired, or mismatched wallet authorization | 401/409 | `ERR_WALLET_AUTH_INVALID` or `ERR_NONCE_REPLAY` |
| Challenge already consuming/consumed | 409 | `ERR_IDEMPOTENCY_CONFLICT` or `ERR_NONCE_REPLAY` |
| Wallet lacks available balance | 402 | `ERR_WALLET_INSUFFICIENT_BALANCE` |
| Route disallows wallet | 403 | `ERR_PAYMENT_METHOD_NOT_ALLOWED` |
| Wallet frozen | 403 | `ERR_WALLET_FROZEN` |

Error responses for insufficient funds, frozen wallet, and policy rejection must not contain `PAYMENT-REQUIRED` for a different payment method.

`ERR_WALLET_AUTH_INVALID` must be added to the canonical protocol error taxonomy and shared conformance vectors before implementation is enabled.

## 7. Implementation and conformance gates

Before enabling the scheme in production, Python and Go must share vectors for: wallet challenge encoding/decoding; amount and request-binding equality; expired and unknown challenge rejection; wrong API-key/tenant/route rejection; one-time nonce/challenge consumption under concurrency and across instances; no hold before authorization; exactly-once hold/capture/release; insufficient balance without chain downgrade; idempotent replay; and ambiguous upstream outcome retaining the hold and local reservation.

The wallet path changes funds reservation, capture/release, idempotency, and state transitions. Those code changes require `SECURITY-REVIEW` comments and explicit human review. This draft is the source for subsequent implementation; do not infer omitted wire fields or weaken the durable replay requirements.
