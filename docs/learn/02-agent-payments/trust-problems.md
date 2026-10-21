# Five Trust Problems in Agent Payments

Letting an agent spend money raises five questions: **Who authorized it? How much may it spend? Who receives the money? What happens when something fails? Who is accountable?** A payment protocol answers only part of this.

| Question | Failure if unanswered | Example control |
|---|---|---|
| Who authorized? | Impersonation or self-granted permissions | Authenticate identity and keep policy outside the model |
| How much? | Hallucinations or loops drain funds | Per-transaction limits, rolling budgets, circuit breakers, atomic reservations |
| To whom? | Funds go to a malicious or incorrect recipient | Verify merchant, domain, network, asset, and recipient against trusted policy |
| What if it fails? | Retry causes duplicate payment, or service is not delivered | Explicit state machine, idempotency, reconciliation, and recovery |
| Who is accountable? | No way to investigate or assign responsibility | Append-only audit events, attributable credentials, and emergency disablement |

## Authorization and identity

Prompt injection may try to convince an agent to increase a budget or pay an attacker. Do not expose policy changes as unrestricted model tools. Derive identity from authenticated credentials rather than trusting a caller-supplied `X-Agent-ID` header. Keep private keys outside model-visible context.

## Amount and concurrency

Per-call limits do not stop a large number of small charges. Use cumulative budgets and rate controls. Concurrent requests must reserve budget atomically; otherwise two requests can each observe the same remaining balance and jointly overspend it.

## Recipient validation

An approved display name alone is insufficient. Bind provider identity to trusted domains, payment addresses, networks, and assets as appropriate to the rail. Validate the amount and recipient in the signed authorization.

## Failure and recovery

A timeout can leave settlement unknown. Do not treat it as failure and repeat the payment automatically. Track intent, settlement, and fulfillment separately; use idempotency and reconciliation to resume only the unfinished step.

## Accountability

Record who or what requested an action, which policy decision applied, and what happened afterward. Keep an audit trail useful for investigation while protecting secrets. Provide a way to disable an agent or revoke its credentials quickly.

The core principle is that an LLM may propose a payment but must not grant itself financial authority. Deterministic controls should evaluate the request independently.
