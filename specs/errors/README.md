# AgentPay Canonical Protocol Errors Specification

This specification defines language-agnostic canonical error codes for the AgentPay protocol across buyer agents, seller gateways, and facilitators.

## Error Taxonomy

| Error Code | HTTP Status | Domain | Description | Actionable Guidance |
| :--- | :--- | :--- | :--- | :--- |
| `ERR_PAYMENT_REQUIRED` | 402 | Protocol | Resource requires x402 V2 payment. | Parse challenge and attach `PAYMENT-SIGNATURE`. |
| `ERR_POLICY_BUDGET_EXCEEDED` | 403 | Policy | 24-hour rolling or lifetime budget exhausted. | Increase policy budget or await window roll-forward. |
| `ERR_POLICY_MAX_PER_TX` | 403 | Policy | Transaction amount exceeds single-tx limit. | Review transaction amount or adjust policy configuration. |
| `ERR_POLICY_CIRCUIT_BROKEN` | 429 | Policy | High frequency or runaway loop detected. | Cool down agent loop or wait for automatic probe recovery. |
| `ERR_NONCE_REPLAY` | 409 | Settlement | Nonce has already been submitted or settled. | Generate fresh cryptographic nonce. |
| `ERR_SIGNATURE_INVALID` | 401 | Cryptography | EIP-712/3009 signature verification failed. | Check private key, domain separator, and signer address. |
| `ERR_UPSTREAM_TIMEOUT` | 504 | Gateway | Gateway timed out awaiting upstream API response. | Retry with same `Idempotency-Key` (free replay). |
| `ERR_UPSTREAM_FAILED` | 502 / 500 | Gateway | Upstream API returned 5xx error. | Retry with same `Idempotency-Key` (free replay). |
| `ERR_IDEMPOTENCY_CONFLICT` | 409 | Idempotency | Active request with same key is currently in-flight. | Wait for in-flight request to resolve or backoff. |
