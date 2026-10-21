# Agent Autonomy: From Assistance to Independent Action

Autonomy describes how much of a task an agent can complete without a person selecting every next step. It is not a single measure of intelligence or safety: an agent may gather information independently but need approval before spending money.

| Mode | Agent role | Human role |
|---|---|---|
| **Assist** | Drafts or recommends | Reviews and performs the action |
| **Confirm each action** | Proposes an operation | Approves each consequential step |
| **Bounded autonomy** | Acts within explicit limits | Sets policy and handles exceptions |
| **Delegated execution** | Completes a multi-step task | Defines goal, constraints, and escalation points |

These are practical categories, not a universal formal standard. Match autonomy to potential harm, maximum loss, reversibility, effects on third parties, and the user's ability to inspect and stop the action. Too many prompts create approval fatigue; too few remove meaningful control.

## Define enforceable boundaries

Specify allowed tools and recipients, amount caps, budgets, rate limits, deadlines, and conditions that require review. Enforce these constraints outside the model. For uncertain outcomes, define a recovery path: a payment timeout should trigger reconciliation, not an automatic second payment.

Start with observation or recommendations, then enable narrow actions in a test environment. Review traces and failure cases before expanding permissions. Provide ways to pause tasks, revoke credentials, and inspect action history.

Autonomy should be granted per capability and bounded by the consequences of failure. The goal is useful execution within clear limits, not maximum independence.
