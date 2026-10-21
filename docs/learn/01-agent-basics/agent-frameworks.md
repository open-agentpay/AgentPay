# Agent Frameworks: What They Provide

Agent applications often need to connect models to tools, preserve state, route requests, handle retries, and observe execution. Frameworks package some of these building blocks.

Common capabilities include model adapters, tool schemas and invocation, conversation state, workflow orchestration, structured output, tracing, and evaluation hooks. Frameworks vary: some focus on tool abstractions, while others provide graph workflows, multi-agent coordination, or hosted runtimes.

## Choose for the task

A predictable sequence may be easier to reason about as an explicit workflow. An agent loop is useful when the next step depends on intermediate findings. Multi-agent designs can divide work, but add coordination, latency, and failure points. Compare frameworks based on model support, integrations, state persistence, observability, deployment, testing, and how clearly you can understand their control flow.

## A framework is not a safety boundary

A framework does not guarantee correct model decisions, protect credentials, authorize payments, or make retries safe. The application still owns authentication, authorization, validation, resource and spending limits, idempotency, recovery, and privacy-aware audit logs. Keep critical policy in deterministic code outside the prompt.

```text
Application policy and credentials
             ↓
Framework: model, tools, task loop
             ↓
Narrowly scoped services and APIs
```

Treat the framework as an orchestration component, not the authority for every action.
