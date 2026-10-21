# The Agent Payment Flow

## A practical scenario

Suppose a user asks an agent to research market data and prepare a report. A data API provides the required information for 0.01 USDC per query. If the agent is connected to a payment system, it may purchase the data automatically when the request meets its authorization rules.

A complete payment flow involves more than sending a transaction. It can include requesting a resource, receiving payment terms, evaluating policy, reserving budget, creating authorization, verifying payment information, delivering the resource, settling, updating state, and recovering from errors. Exact ordering varies by protocol and implementation. The outline below uses an illustrative x402 buyer, gateway, and facilitator architecture.

## Overview

```text
Agent → Buyer SDK → Paid API request
  ← HTTP 402 payment requirement
  → Parse requirements and evaluate policy
  → Atomically reserve budget
  → Create payment authorization
  → Retry request with payment information
Gateway verifies payment → calls upstream service → delivers resource
Settlement is submitted or tracked → state is updated and reconciled
```

Payment authorization, resource delivery, and settlement are related, but they are not interchangeable events.

## 1. Request the resource

The agent first makes an ordinary HTTP request, for example:

```http
GET /v1/market-data
Host: api.example.com
```

The client may not know whether the resource is paid. The server may return the resource directly or respond with payment requirements.

## 2. Receive and inspect the payment requirement

An API that requires payment may return HTTP 402 with terms such as the service, price, network, asset, and recipient. These fields are illustrative, not a wire-format specification.

The client must check the requirements against its own policy. It should reject a request if, for example, the network is not allowed or the recipient is not an approved provider. It must not blindly accept arbitrary terms supplied by a server.

## 3. Evaluate policy

An illustrative policy might set a 0.10 USDC per-payment limit, a 2.00 USDC daily budget, and an allowlist containing `market-data`. A 0.01 USDC request can proceed only if the provider, amount, remaining budget, and request rate all pass their checks. A low price does not make an unapproved merchant acceptable.

Passing policy means the request may continue; it does not mean funds have settled.

## 4. Reserve budget atomically

Suppose 0.05 USDC remains and two requests each need 0.04 USDC. If both merely read the balance, both could proceed. An atomic reservation gives the first request 0.04 USDC and leaves 0.01 USDC available, so the second is rejected.

Reservation temporarily holds capacity for a request. The system later converts it to spend or releases it based on the outcome. The implementation must use a concurrency-safe transaction or equivalent mechanism.

## 5. Create payment authorization

After budget approval, the client creates authorization appropriate to the selected payment scheme. On some EVM paths this may use an EIP-712 structured signature and an EIP-3009 authorization.

```text
Payment requirement
   ↓ validate network, asset, recipient, and amount
Build authorization data
   ↓
Wallet or signer signs
   ↓
Payment credential
```

A signature proves that a key authorized particular data. It does not replace budget controls, merchant restrictions, or business rules. Before signing, the client should verify that the signed fields match the original payment requirement.

## 6. Submit and verify payment information

The client sends payment information as required by the protocol. The service or payment component should validate the signature, authorization conditions, asset, network, amount, recipient, expiry, and whether the authorization has already been used. The exact checks depend on the payment scheme. A client assertion that it has paid is not proof of verified payment.

## 7. Deliver the resource

After payment verification, the provider returns the requested resource. The system should record fulfillment separately from payment and settlement so an upstream service failure does not cause the client to pay again blindly.

## 8. Submit or track settlement

Some flows submit settlement before delivery; others may verify authorization first and track settlement asynchronously. A network timeout cannot establish whether settlement failed. Preserve a pending or unknown result, query the same operation, and reconcile before retrying.

## 9. Update state and recover

Record policy outcome, reservation, authorization, settlement, and fulfillment as distinct facts. If settlement is confirmed but fulfillment failed, recover delivery. If the payment outcome is unknown, reconcile it before creating another payment. Idempotency keys help ensure that repeated requests for the same business intent do not create duplicate effects.

## Key takeaway

Treat the flow as coordinated stages with explicit state transitions. Policy approval, cryptographic authorization, payment verification, settlement, and service delivery each answer a different question.
