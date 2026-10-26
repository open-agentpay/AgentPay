"""
AgentPay EVM & Arbitrum Sepolia Integration Module.

Provides:
- Authoritative Arbitrum Sepolia Testnet parameters and Circle USDC contracts
- EIP-3009 / FiatTokenV2_2 calldata encoding and result decoding
- Fast, dependency-light EVM JSON-RPC client
"""

from __future__ import annotations

from agentpay.evm.constants import (
    ARBITRUM_SEPOLIA,
    ARBITRUM_SEPOLIA_CAIP2,
    ARBITRUM_SEPOLIA_CHAIN_ID,
    ARBITRUM_SEPOLIA_EXPLORER_URL,
    ARBITRUM_SEPOLIA_RPC_URL,
    ARBITRUM_SEPOLIA_USDC_ADDRESS,
    ARBITRUM_SEPOLIA_USDC_DECIMALS,
    ARBITRUM_SEPOLIA_USDC_NAME,
    ARBITRUM_SEPOLIA_USDC_SYMBOL,
    ARBITRUM_SEPOLIA_USDC_VERSION,
    AUTHORIZATION_STATE_SELECTOR,
    CANCEL_AUTHORIZATION_SELECTOR,
    KNOWN_NETWORKS,
    RECEIVE_WITH_AUTHORIZATION_SELECTOR,
    TRANSFER_WITH_AUTHORIZATION_SELECTOR,
    NetworkSpec,
    get_network_spec,
)
from agentpay.evm.eip3009 import (
    decode_authorization_state_result,
    decode_balance_of_result,
    decode_transfer_with_authorization,
    encode_authorization_state,
    encode_balance_of,
    encode_transfer_from_authorization,
    encode_transfer_with_authorization,
)
from agentpay.evm.rpc import (
    EvmRpcClient,
    EvmRpcConnectionError,
    EvmRpcError,
    EvmRpcRequestError,
)

__all__ = [
    # Network & Chain Constants
    "ARBITRUM_SEPOLIA",
    "ARBITRUM_SEPOLIA_CHAIN_ID",
    "ARBITRUM_SEPOLIA_CAIP2",
    "ARBITRUM_SEPOLIA_RPC_URL",
    "ARBITRUM_SEPOLIA_EXPLORER_URL",
    "ARBITRUM_SEPOLIA_USDC_ADDRESS",
    "ARBITRUM_SEPOLIA_USDC_SYMBOL",
    "ARBITRUM_SEPOLIA_USDC_DECIMALS",
    "ARBITRUM_SEPOLIA_USDC_NAME",
    "ARBITRUM_SEPOLIA_USDC_VERSION",
    "TRANSFER_WITH_AUTHORIZATION_SELECTOR",
    "RECEIVE_WITH_AUTHORIZATION_SELECTOR",
    "CANCEL_AUTHORIZATION_SELECTOR",
    "AUTHORIZATION_STATE_SELECTOR",
    "NetworkSpec",
    "KNOWN_NETWORKS",
    "get_network_spec",
    # EIP-3009 ABI
    "encode_transfer_with_authorization",
    "encode_transfer_from_authorization",
    "decode_transfer_with_authorization",
    "encode_authorization_state",
    "decode_authorization_state_result",
    "encode_balance_of",
    "decode_balance_of_result",
    # RPC Client
    "EvmRpcClient",
    "EvmRpcError",
    "EvmRpcRequestError",
    "EvmRpcConnectionError",
]
