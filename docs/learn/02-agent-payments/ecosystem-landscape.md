# A Neutral Guide to the Agent Payment Ecosystem

Agent payments are an evolving field. The protocols often discussed together solve different layers of a commercial transaction, so they are not necessarily competing replacements.

## Classify by the problem they solve

| Layer | Question | Examples |
|---|---|---|
| Tool and communication | How do agents discover tools and collaborate? | MCP, A2A |
| Commerce workflow | How does an agent find products, build a cart, and check out? | ACP, UCP |
| Authorization and trust | How can a merchant verify human intent and accountability? | AP2, card-network agent programs |
| Financial control | Should this agent pay, how much, and how should failures be handled? | AgentPay's focus |
| Payment and fulfillment | How is a machine-readable payment presented and settled? | x402 and other machine-payment protocols |

## Protocols at a glance

- **x402** uses HTTP payment requirements and payment information for machine-facing services. It concerns the payment interaction; it does not by itself decide whether an agent is allowed to spend.
- **Machine Payments Protocol (MPP)** is another approach to machine-to-machine payments. Compare its current specification and supported rails with the intended integration before adopting it.
- **AP2** focuses on verifiable authorization and accountability for agent commerce, often described through signed mandates. It addresses evidence of user intent rather than every runtime budget-control problem.
- **Visa and Mastercard agent initiatives** build on existing card-network infrastructure, including identity, tokenization, and issuer controls.
- **OpenAI/Stripe ACP and Google-led UCP** address commerce workflows such as product discovery and checkout. In crypto contexts, “ACP” may also refer to an unrelated Agent Commerce Protocol; check the project and use case.
- **MCP and A2A** support tool connectivity or agent communication. They are complementary to payment and authorization mechanisms.

These categories can work together in one transaction. An agent might discover a tool over a communication protocol, select an offer through a commerce flow, prove user authorization, pass a payment request through a payment protocol, and rely on a separate control layer to enforce its budget.

## How AgentPay fits

AgentPay focuses on the buyer-side financial control layer: identity, spending rules, budget reservation, failure recovery, and audit. It can complement a payment protocol such as x402 rather than replace its wire-level payment interaction. Protocol names, governance, and product status change quickly; verify current details against each project's official documentation before implementation.
