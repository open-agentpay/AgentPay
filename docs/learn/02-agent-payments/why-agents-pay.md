# Why Agents Need to Pay: APIs, Data, Compute, and Other Agents

Traditional chat applications mainly incur model inference costs. An agent that acts in the world also consumes external resources: search and specialist APIs, commercial datasets, cloud compute, paid tools, and services provided by other agents. As the agent becomes more capable, payment becomes a runtime requirement rather than an occasional add-on.

## What agents may buy

| Category | Examples | Common pricing |
|---|---|---|
| APIs | Search, weather, maps, OCR | Per call or request volume |
| Commercial data | Market data, research, company records | Per record, query, or item |
| Elastic compute | Code sandboxes, model inference, browsers | Per second, token, or instance |
| Tools and agents | Specialist MCP tools, expert sub-agents | Per call or result |
| Real-world services | Travel, cloud services, online purchases | Per item or transaction |

Many agent-native purchases are small, frequent, and directed to providers discovered dynamically. A workflow may make dozens of calls, each to a different service. Traditional card fees, subscriptions, and manually managed API keys may be awkward for this pattern. The trade-offs are discussed in [Payment Options Compared](./payment-options.md).

## Why not ask a person every time?

Per-call approval can create alert fatigue. It also prevents unattended jobs from completing when no person is available. The answer is not unrestricted spending: set policy in advance, then enforce per-transaction limits, cumulative budgets, approved recipients, rate limits, and escalation rules independently of the model.

## A separate control layer

The developer should not have to pre-register for every long-tail service or expose reusable credentials to every agent. A payment system for agents needs to bind identity and authorization to each request, enforce limits, make retries safe, and preserve an audit trail. The model may propose that a resource is useful; trusted code decides whether the agent may pay for it.
