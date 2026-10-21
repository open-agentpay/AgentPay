# Stablecoins and USDC: A Primer

A stablecoin is a digital token designed to track the value of an asset, commonly the US dollar. It can provide a familiar unit of account for on-chain payments, but stability is a design goal rather than a mathematical guarantee. Issuer, reserve, operational, regulatory, and blockchain risks remain.

## Why use a stable-value asset?

If an API is priced in a volatile token, the dollar cost of a call can change quickly, making budgets and pricing harder to reason about. A dollar-linked token gives buyers and providers a more predictable denomination. Stablecoins can also be transferred by software and used across borders without the same account setup as many traditional payment services.

These properties can be useful for frequent, small machine payments. They do not eliminate the need for wallet security, recipient verification, policy enforcement, or settlement monitoring.

## Why USDC in this learning material?

USDC is a widely used dollar-denominated stablecoin. On supported EVM networks, its implementation of EIP-3009 can allow a holder to sign an authorization that another party submits, so the buyer does not necessarily need to hold the network's gas token. See [Gasless Authorization](./gasless-eip-3009.md).

Support depends on the specific token deployment and network. Verify contract addresses and current network support using authoritative project documentation; do not assume every token or chain supports the same behavior. AgentPay examples should use testnets and test tokens, not production funds.

## Use integer base units

USDC commonly uses six decimal places. Applications should represent amounts as integers in base units rather than binary floating-point values:

| Display amount | Base units |
|---:|---:|
| 1 USDC | 1,000,000 |
| 0.1 USDC | 100,000 |
| 0.000001 USDC | 1 |

Floating-point calculations can introduce rounding errors. Parse and validate decimal input, convert it to integer units, and keep it integer-valued through policy checks and payment construction.
