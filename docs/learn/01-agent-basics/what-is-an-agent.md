# From Chat to Action: What Is an AI Agent?

## From answering questions to completing tasks

Most people use AI by asking a question and waiting for an answer. A language model can summarize what it already knows, but without connected tools it cannot search for current information or take actions in other systems.

Consider a more involved request: search the past week's AI news, select five important stories, read the original sources, compare their perspectives, and produce a briefing. Completing it may require searching, fetching pages, checking relevance, comparing sources, searching again when evidence is missing, and writing the report. This is a multi-step task, not a single text generation.

**An AI agent works toward a goal by combining model reasoning, tool use, and feedback from actions.**

## LLM, workflow, and agent

An LLM interprets input and generates text or recommendations. A workflow follows steps defined in advance, which is useful when the control flow is predictable. An agent uses a model to choose or adapt actions based on what it observes, often calling tools and inspecting their results before deciding what to do next. Real systems can combine workflows and agent decisions.

```text
Receive goal → observe context → choose an action → call a tool
      ↑                                         ↓
      └──────────── inspect result ─────────────┘
```

The loop stops when the goal is met, a limit is reached, or human input is needed.

## Common components

- **Model:** interprets the task and proposes decisions.
- **Instructions and context:** describe goals, constraints, and relevant information.
- **Tools:** expose capabilities such as search, databases, and APIs.
- **Orchestrator:** manages the action loop, tool results, and stopping conditions.
- **State or memory:** preserves task progress where needed.
- **External controls:** enforce permissions, budgets, validation, and approvals.

The model is only one component. Reliable agents depend on surrounding software to validate inputs, restrict actions, handle errors, and record events.

## Why the distinction matters

An ordinary text response usually has no direct external effect. An agent can send messages, modify records, trigger deployments, or make purchases through tools. The more consequential the tools, the more important it is to use narrow permissions, explicit limits, and recovery paths.

For payment-capable agents, the model may propose a purchase, but deterministic software should independently decide whether it satisfies the user's authorization and spending policy.
