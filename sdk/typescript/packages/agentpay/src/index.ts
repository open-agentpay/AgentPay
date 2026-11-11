/**
 * AgentPay TypeScript SDK
 * The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents.
 */

export const VERSION = '0.4.0';

// Export all universal policy engine, budget firewall, x402 codecs and client from agentpay-core-sdk
export * from 'agentpay-core-sdk';

// Export AgentPay Web3 / EVM settlement infrastructure
export * from './evm/index.js';

// Explicitly re-export EVM signer members to resolve export ambiguity with legacy agentpay-core-sdk
export {
  LocalSigner,
  DEFAULT_TOKEN_NAMES,
  EIP3009_MESSAGE_TYPES,
  generateNonce,
  parseCaip2ChainId,
} from './evm/index.js';
