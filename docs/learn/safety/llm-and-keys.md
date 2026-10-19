---
title: Why LLMs Must Never Hold Private Keys
description: Analyzing the catastrophic risks of prompt injections, model hallucinations, and supply-chain attacks when LLMs hold master keys.
---

# Why LLMs Must Never Hold Private Keys

Directly injecting private keys or master payment secrets into an LLM context window is the most dangerous anti-pattern in agent engineering:

1. **Direct Prompt Injection**: Attackers can construct adversarial prompts or manipulate upstream tool outputs to coerce the model into leaking private keys.
2. **Model Hallucinations**: An LLM may misinterpret transaction amounts (e.g. confusing 100 USDC with 100 ETH) and sign transactions with unbounded values.
3. **Absence of Memory Isolation**: If third-party dependencies are compromised, plaintext keys in process memory are completely exposed.

**Core Axiom**: Private keys and signature authority must remain strictly within an out-of-band gateway, secure enclave, or KMS. Agents should only issue policy-governed "payment intents."

::: tip In AgentPay
AgentPay agents only hold scoped API keys (`apk_live_...`). Master signing keys reside behind the gateway security boundary. See the [Security Model Specification](/specs/security-model).
:::
