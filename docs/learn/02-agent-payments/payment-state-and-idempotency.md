# Payment State Machines, Idempotency, and Recovery

## Why success and failure are not enough

Suppose an agent purchases data for 0.10 USDC. The client sends a payment request and the network connection drops. The only known fact is that the client did not receive a complete response. It does not know whether the request reached the server, payment was verified, a transaction was submitted or confirmed, or the data was delivered.

A single `SUCCESS / FAILED` field cannot represent these intermediate and uncertain outcomes. A payment system needs explicit states and rules for moving between them.

## What is a state machine?

A state machine describes a process using named states and allowed transitions. A simplified payment intent might move through:

```text
CREATED → AUTHORIZED → VERIFIED → COMPLETED
                 ↘ REJECTED
                 ↘ EXPIRED
                 ↘ CANCELLED
```

These names are examples. An implementation must define each state's precise meaning, entry conditions, valid next states, and recovery behavior. In particular, `COMPLETED` must not ambiguously mean authorized, delivered, and settled all at once.

## Keep intent, settlement, and fulfillment distinct

### Payment intent

Tracks the business request and its authorization lifecycle, such as created, authorized, verified, rejected, expired, or cancelled.

### Settlement

Tracks movement of funds. A simplified lifecycle might be:

```text
NOT_STARTED → SUBMITTED → PENDING → CONFIRMED
                              ↘ FAILED
                              ↘ REORGED
                              ↘ UNKNOWN
```

`UNKNOWN` means the system cannot yet determine the result. `REORGED` means a previously observed confirmation needs rechecking after a chain reorganization. Actual transitions depend on the payment rail.

### Fulfillment

Tracks whether the purchased service was delivered:

```text
NOT_STARTED → PROCESSING → DELIVERED
                        ↘ FAILED
                        ↘ RECOVERY_REQUIRED
```

For example, payment may be confirmed while an upstream data service times out. In that case settlement is confirmed but fulfillment needs recovery. The system should resume the incomplete service step rather than blindly replaying payment.

## Idempotency prevents duplicate effects

Idempotency means that submitting the same business request again does not create an unintended second effect. If a client times out and retries a 0.10 USDC payment without idempotency controls, the system could charge twice for the same intent.

An idempotency key identifies one business operation. The same intent should reuse the same key across retries. The server can then return the existing result, continue processing, or reject an invalid duplicate rather than creating a new payment.

For example, a database can enforce uniqueness within a tenant:

```sql
CREATE TABLE payment_intent (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    amount TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (tenant_id, idempotency_key)
);
```

This SQLite example illustrates a uniqueness constraint; it is not a complete production payment schema.

## Retry and recovery rules

- Reuse the original idempotency key when retrying the same intent.
- Query or reconcile an unknown settlement before creating another payment.
- Do not equate a timeout with failure.
- Recover fulfillment independently when settlement has already succeeded.
- Make state transitions conditional and auditable so stale workers cannot overwrite newer results.

## Key takeaway

Represent what the system knows, including uncertainty. Separate the business intent from settlement and service delivery, and make retries refer to the same intent. This makes recovery safer and prevents ambiguous network failures from turning into duplicate charges.
