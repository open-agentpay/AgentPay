# HTTP 402 and the x402 Protocol

## What does HTTP 402 mean?

HTTP status `402 Payment Required` is reserved for payment-related use. For years it had no broadly adopted standard payment flow. A service can use it to tell a client that a requested resource requires payment, but the status code alone does not define how to pay or prove that payment occurred.

## The basic x402 pattern

x402 uses HTTP request and response conventions to communicate payment requirements and payment evidence. A simplified flow is:

```text
Client requests a resource
        ↓
Server returns 402 with payment requirements
        ↓
Client evaluates the terms and creates payment data
        ↓
Client retries with payment information
        ↓
Server or facilitator verifies/processes it
        ↓
Server returns the resource
```

The exact fields and lifecycle depend on the protocol version and implementation. A 402 response is a request for payment, not proof that the terms are safe or authorized.

## What x402 provides—and what it does not

x402 standardizes a way for web services and clients to negotiate machine-readable payment requirements and continue a request with payment information. It can make pay-per-request services easier to integrate without a conventional account or subscription.

It does not decide whether an agent should pay, how much it may spend, which providers are trusted, or how an application should recover from uncertain outcomes. The buyer still needs to validate network, asset, amount, and recipient and apply independent spending policy. Payment verification and resource fulfillment must also be tracked carefully.

## How AgentPay relates

The roles are complementary: x402 describes a payment interaction; AgentPay provides controls around whether an agent is allowed to proceed and how budgets, identity, retries, and audit are handled. A protocol can carry a payment request without being the financial authority for the agent.
