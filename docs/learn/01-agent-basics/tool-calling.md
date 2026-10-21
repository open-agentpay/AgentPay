# Tool Calling: How Agents Interact with External Systems

A language model normally returns text. Tool calling lets an application offer named operations with descriptions and structured inputs. The model can request an operation; the host validates and executes it, then returns the result to the model.

```text
User request → model selects a tool → host validates and runs it
             ← tool result        ←
```

The model does not execute arbitrary code simply by proposing a call. The application controls which tools exist and whether a request is permitted.

For example, a weather lookup might be requested as `get_weather({"city":"Singapore"})`. The host validates the tool and arguments, calls the service, and passes its result back. Schema validation checks form; authorization checks whether this caller may perform the operation. Both are necessary.

Use narrowly scoped capabilities. A read-only lookup should not share credentials with an operation that can change records or spend money. Validate arguments at the boundary and use idempotency for operations that may be retried.

Tools can fail, partially complete, or time out after performing an action. A timeout after a write does not prove the write failed; query or reconcile before retrying. Treat tool output as untrusted data because webpages and API responses may contain prompt injection. Keep authorization independent of model-generated instructions.

**The model proposes an operation; the host owns validation, credentials, authorization, execution, and recovery.**
