# Spending Policies: How to Limit Agent Spending

## Why use a spending policy engine?

Suppose a research agent receives a 10 USDC budget for purchasing data. It may use APIs appropriately, or repeatedly buy unnecessary data because of a model error, task loop, or malicious input. A wallet and payment API alone do not impose meaningful boundaries.

A spending policy engine applies independent rules before payment execution. It combines the role of account spending controls with gateway access controls.

## Basic evaluation flow

```text
Agent requests payment
        ↓
Authenticate agent
        ↓
Check provider and recipient
        ↓
Check per-payment amount
        ↓
Check budgets
        ↓
Check request rate
        ↓
Check risk and approval requirements
        ↓
Allow, reject, or wait for approval
```

An allowed request has passed policy evaluation; funds may still need authorization, payment, fulfillment, and settlement.

## Per-payment limits

A per-payment limit caps the amount of one request. For example, with `max_transaction_amount: 0.10` USDC, a 0.05 or 0.10 request passes this rule while a 0.20 request is rejected. Passing this check does not imply that other rules pass. A per-payment cap cannot control cumulative spending by itself.

## Time-window budgets

A time-window budget caps cumulative spending over a defined period. A policy might set `daily: 2.00` USDC. The implementation must specify whether “daily” means a calendar day or a rolling 24-hour window, along with timezone, accounting, expiry, and concurrency rules.

With a rolling 24-hour budget, if 1.80 USDC has been used and 0.20 remains, a 0.05 request can fit while a 0.25 request cannot. Budget checks should account for in-flight reservations as well as confirmed spend.

## Provider allowlists

An allowlist limits which services may receive payment, for example `search-api`, `market-data`, and `translation-api`. An unknown provider should be rejected even for a tiny amount. Names supplied freely by a remote service are not proof of identity; the implementation must bind provider identity to trustworthy payment details.

## Request-rate limits

A rule such as `requests_per_minute: 10` can limit runaway loops, duplicate traffic, and unusually frequent spending. Rate limits do not replace budgets: ten expensive calls may cost more than hundreds of cheap ones. Use rate controls together with amount and cumulative-spend limits.

## Lifetime limits

A lifetime limit caps total spending over an agent's authorization period. If an agent has a 20.00 USDC lifetime cap and has already spent 19.90, a new 0.20 request should be rejected even if the current daily budget is available. Define how refunds, failed attempts, and reservations affect this total.

## Human approval thresholds

Some requests should wait for a person. For example, a policy might require approval above 1.00 USDC. Bind approval to the exact request parameters. If the amount, recipient, or another important field changes after approval, obtain a new approval or revalidate the authorization.

## Reserve budget before proceeding

If two concurrent requests each need 0.70 USDC while only 1.00 remains, checking without reserving can approve both. Instead, atomically reserve the amount before continuing:

```text
Check available budget
        ↓
Atomically reserve amount
        ↓
Execute payment flow
        ↓
Commit spend or release reservation
```

A useful accounting model distinguishes:

- `available`: budget still free to allocate.
- `reserved`: budget held for in-flight requests.
- `spent`: confirmed consumption.

In a simple model:

```text
available = limit - spent - reserved
```

The reservation and policy decision must be concurrency-safe. Otherwise simultaneous requests can exceed the configured limit.

## Key takeaway

No single rule is enough. Combine per-payment limits, rolling budgets, provider allowlists, rate controls, lifetime limits, and approvals as appropriate. Enforce them in deterministic code and reserve budget atomically before authorizing payment.
