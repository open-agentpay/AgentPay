# Common Agent Failure Modes

Agents combine probabilistic model output with tools that can affect external systems. Failures can arise from reasoning, input data, orchestration, integrations, or permissions.

## Reasoning errors

A model may misunderstand a request, make an unsupported inference, overlook a constraint, or state uncertainty as fact. Early mistakes can compound through otherwise valid tool calls. Check sources, validate intermediate results, and make uncertainty visible.

## Prompt injection and untrusted content

Webpages, documents, and tool results may contain instructions that attempt to redirect the agent. Treat retrieved content as data, isolate it from trusted instructions, and never let model text modify its own permissions. Enforce sensitive actions in deterministic code.

## Tool and integration failures

Tools can return stale or malformed data, fail partially, or time out after completing an action. Validate schemas and business rules at integration boundaries. For writes and payments, reconcile uncertain results before retrying.

## Loops and runaway execution

An agent may repeat calls because it fails to recognize completion or receives unexpected results. Limit steps, time, retries, concurrency, and resource use. Detect repeated calls and provide a stop path. For paid calls, enforce both per-call and cumulative budgets.

## Excessive permissions and unclear identity

Broad credentials can turn a small model error into a serious incident. Bind identity to authenticated credentials, grant least privilege, and check each sensitive action against server-side policy rather than trusting model instructions.

## Unclear outcomes and accountability

A timeout does not prove an operation failed. Represent pending or unknown results explicitly and query the original operation before creating a duplicate. Keep an audit trail of relevant inputs and actions while protecting secrets and sensitive data.

Assume models, tools, networks, and orchestration can each fail. The more impact a tool can have, the narrower its permissions and the stronger its validation and recovery path should be.
