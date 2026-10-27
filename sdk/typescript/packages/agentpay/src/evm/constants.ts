/**
 * Arbitrum Sepolia Testnet & EIP-3009 Constants for AgentPay.
 *
 * Authoritative reference for:
 * - Arbitrum Sepolia Testnet (Chain ID 421614, CAIP-2: eip155:421614)
 * - Native Circle FiatTokenV2_2 USDC contract
 * - Function selectors and EIP-712 / EIP-3009 constants
 */

export const ARBITRUM_SEPOLIA_CHAIN_ID = 421614;
export const ARBITRUM_SEPOLIA_CAIP2 = 'eip155:421614';
export const ARBITRUM_SEPOLIA_RPC_URL = 'https://sepolia-rollup.arbitrum.io/rpc';
export const ARBITRUM_SEPOLIA_EXPLORER_URL = 'https://sepolia.arbiscan.io';

export const ARBITRUM_SEPOLIA_USDC_ADDRESS = '0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d';
export const ARBITRUM_SEPOLIA_USDC_SYMBOL = 'USDC';
export const ARBITRUM_SEPOLIA_USDC_DECIMALS = 6;
export const ARBITRUM_SEPOLIA_USDC_NAME = 'USD Coin';
export const ARBITRUM_SEPOLIA_USDC_VERSION = '2';

// EIP-3009 4-byte Function Selectors
export const TRANSFER_WITH_AUTHORIZATION_SELECTOR = new Uint8Array([0xe3, 0xee, 0x16, 0x0e]);
export const RECEIVE_WITH_AUTHORIZATION_SELECTOR = new Uint8Array([0xef, 0x55, 0xde, 0xc6]);
export const CANCEL_AUTHORIZATION_SELECTOR = new Uint8Array([0x5b, 0x34, 0xb9, 0x66]);
export const AUTHORIZATION_STATE_SELECTOR = new Uint8Array([0xe9, 0x4a, 0x01, 0x02]);
export const BALANCE_OF_SELECTOR = new Uint8Array([0x70, 0xa0, 0x82, 0x31]);
export const DOMAIN_SEPARATOR_SELECTOR = new Uint8Array([0x36, 0x44, 0xe5, 0x15]);

export interface NetworkSpecInput {
  name: string;
  chainId: number;
  caip2: string;
  rpcUrl: string;
  explorerUrl: string;
  defaultAssetAddress: string;
  defaultAssetSymbol?: string;
  defaultAssetDecimals?: number;
  tokenName?: string;
  tokenVersion?: string;
}

export class NetworkSpec {
  readonly name: string;
  readonly chainId: number;
  readonly caip2: string;
  readonly rpcUrl: string;
  readonly explorerUrl: string;
  readonly defaultAssetAddress: `0x${string}`;
  readonly defaultAssetSymbol: string;
  readonly defaultAssetDecimals: number;
  readonly tokenName: string;
  readonly tokenVersion: string;

  constructor(input: NetworkSpecInput) {
    this.name = input.name;
    this.chainId = input.chainId;
    this.caip2 = input.caip2;
    this.rpcUrl = input.rpcUrl;
    this.explorerUrl = input.explorerUrl;
    this.defaultAssetAddress = input.defaultAssetAddress.toLowerCase() as `0x${string}`;
    this.defaultAssetSymbol = input.defaultAssetSymbol ?? 'USDC';
    this.defaultAssetDecimals = input.defaultAssetDecimals ?? 6;
    this.tokenName = input.tokenName ?? 'USD Coin';
    this.tokenVersion = input.tokenVersion ?? '2';
    Object.freeze(this);
  }
}

export const ARBITRUM_SEPOLIA = new NetworkSpec({
  name: 'arbitrum-sepolia',
  chainId: ARBITRUM_SEPOLIA_CHAIN_ID,
  caip2: ARBITRUM_SEPOLIA_CAIP2,
  rpcUrl: ARBITRUM_SEPOLIA_RPC_URL,
  explorerUrl: ARBITRUM_SEPOLIA_EXPLORER_URL,
  defaultAssetAddress: ARBITRUM_SEPOLIA_USDC_ADDRESS,
  defaultAssetSymbol: ARBITRUM_SEPOLIA_USDC_SYMBOL,
  defaultAssetDecimals: ARBITRUM_SEPOLIA_USDC_DECIMALS,
  tokenName: ARBITRUM_SEPOLIA_USDC_NAME,
  tokenVersion: ARBITRUM_SEPOLIA_USDC_VERSION,
});

export const KNOWN_NETWORKS: Readonly<Record<string, NetworkSpec>> = Object.freeze({
  [ARBITRUM_SEPOLIA_CAIP2]: ARBITRUM_SEPOLIA,
  [String(ARBITRUM_SEPOLIA_CHAIN_ID)]: ARBITRUM_SEPOLIA,
  'arbitrum-sepolia': ARBITRUM_SEPOLIA,
  'arb-sepolia': ARBITRUM_SEPOLIA,
});

export function getNetworkSpec(networkOrChainId: string | number): NetworkSpec | undefined {
  const key = String(networkOrChainId).trim().toLowerCase();
  return KNOWN_NETWORKS[key];
}
