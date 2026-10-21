# EIP-712 and EIP-3009: Why an Agent May Not Need Gas

## The gas problem

On EVM networks, sending a transaction normally requires the sender to pay network fees in the chain's native gas token. That creates friction for an agent whose payment balance contains USDC but no ETH or other native token: before using a paid service, it would first need to acquire gas.

## Authorization can be relayed

EIP-3009 defines USDC authorization methods such as `transferWithAuthorization`. The token holder signs structured authorization data off-chain; another party can submit it on-chain and pay the transaction fee. The holder can therefore authorize a transfer without personally submitting the transaction.

EIP-712 provides a structured-data signing format. Typed fields make the signed message easier for software and wallets to interpret than an opaque byte string. An authorization typically binds relevant values such as token domain, sender, recipient, amount, validity window, and a unique nonce. Exact fields depend on the token contract and method.

```text
Agent validates payment terms
        ↓
Wallet signs bounded authorization
        ↓
Facilitator submits the authorization and pays gas
        ↓
Token contract verifies and performs transfer
```

“Gasless” means the buyer need not pay gas for that transaction directly; the facilitator still pays network fees and may apply its own rules or costs.

## Security checks

Before signing, verify chain, token contract, recipient, amount, expiry, and nonce. Keep the private key outside the model's context and avoid granting broad approvals. The signer should sign only data that has already passed independent spending policy.

Authorization is not the same as confirmed settlement. Relayed submission can fail, remain pending, or have an uncertain result. Track the authorization and settlement state, and do not blindly sign a second transfer after a timeout. Use test networks and test tokens while developing.
