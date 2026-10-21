# Payment Options for Agents: Subscriptions, Prepaid API Keys, Cards, and Stablecoins

No single payment method fits every agent workload. Compare options by how easily an agent can discover a provider, pay per use, control spending, recover from errors, and operate across providers.

| Method | Strengths | Limitations for autonomous use |
|---|---|---|
| Subscription | Predictable recurring price; simple for steady usage | May charge for unused capacity; does not fit dynamic discovery or per-call settlement |
| Prepaid API balance/key | Easy to integrate with one provider; spend is bounded by the balance | Requires separate accounts and keys; balances and billing are fragmented |
| Credit or debit card | Broad merchant acceptance and familiar dispute processes | Credential exposure risk; fixed fees can dominate tiny transactions; often designed around human checkout |
| Stablecoin payment | Programmable, globally accessible, and suitable for small machine-to-machine transfers | Requires wallet and chain infrastructure; network, asset, custody, and settlement risks remain |

## Choose for the workload

A subscription can be best for a known service with steady demand. A provider-specific prepaid key may be practical when using only a few APIs. Cards remain useful for ordinary commerce and established merchant flows. Stablecoins can support open, pay-per-request services, but do not by themselves provide authorization policy or recovery guarantees.

These approaches can coexist. A production system may use subscriptions for predictable baseline capacity and per-request payment for long-tail services. Whichever rail is selected, keep secrets out of model context and enforce limits in trusted code.

## What payment rails do not solve

The transfer mechanism does not decide whether a purchase matches the user's intent, whether the recipient is trusted, whether the budget remains available, or what to do after a timeout. Those are control and state-management responsibilities that sit above the payment method.
