"""
Arbitrum Sepolia Testnet & EIP-3009 Constants for AgentPay.

Authoritative reference for:
- Arbitrum Sepolia Testnet (Chain ID 421614, CAIP-2: eip155:421614)
- Native Circle FiatTokenV2_2 USDC contract
- Function selectors and EIP-712 / EIP-3009 constants
"""

from __future__ import annotations

from typing import Final

# -----------------------------------------------------------------------------
# Arbitrum Sepolia Network Parameters
# -----------------------------------------------------------------------------

ARBITRUM_SEPOLIA_CHAIN_ID: Final[int] = 421614
ARBITRUM_SEPOLIA_CAIP2: Final[str] = "eip155:421614"
ARBITRUM_SEPOLIA_RPC_URL: Final[str] = "https://sepolia-rollup.arbitrum.io/rpc"
ARBITRUM_SEPOLIA_EXPLORER_URL: Final[str] = "https://sepolia.arbiscan.io"

# -----------------------------------------------------------------------------
# Official Circle USDC Contract on Arbitrum Sepolia
# Standard: FiatTokenV2_2 (EIP-3009 transferWithAuthorization enabled)
# Verified on Arbiscan: https://sepolia.arbiscan.io/address/0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d
# -----------------------------------------------------------------------------

ARBITRUM_SEPOLIA_USDC_ADDRESS: Final[str] = "0x75faf114eafb1BDbe2F0316DF893fd58CE46AA4d"
ARBITRUM_SEPOLIA_USDC_SYMBOL: Final[str] = "USDC"
ARBITRUM_SEPOLIA_USDC_DECIMALS: Final[int] = 6
ARBITRUM_SEPOLIA_USDC_NAME: Final[str] = "USD Coin"
ARBITRUM_SEPOLIA_USDC_VERSION: Final[str] = "2"

# -----------------------------------------------------------------------------
# EIP-3009 4-byte Function Selectors
# -----------------------------------------------------------------------------

# transferWithAuthorization(address,address,uint256,uint256,uint256,bytes32,uint8,bytes32,bytes32)
TRANSFER_WITH_AUTHORIZATION_SELECTOR: Final[bytes] = bytes.fromhex("e3ee160e")

# receiveWithAuthorization(address,address,uint256,uint256,uint256,bytes32,uint8,bytes32,bytes32)
RECEIVE_WITH_AUTHORIZATION_SELECTOR: Final[bytes] = bytes.fromhex("ef55dec6")

# cancelAuthorization(address,bytes32,uint8,bytes32,bytes32)
CANCEL_AUTHORIZATION_SELECTOR: Final[bytes] = bytes.fromhex("5b34b966")

# authorizationState(address,bytes32)
AUTHORIZATION_STATE_SELECTOR: Final[bytes] = bytes.fromhex("e94a0102")

# balanceOf(address)
BALANCE_OF_SELECTOR: Final[bytes] = bytes.fromhex("70a08231")

# DOMAIN_SEPARATOR()
DOMAIN_SEPARATOR_SELECTOR: Final[bytes] = bytes.fromhex("3644e515")


# -----------------------------------------------------------------------------
# Network Registry
# -----------------------------------------------------------------------------


class NetworkSpec:
    """Immutable specification for an EVM network and its default settlement asset."""

    def __init__(
        self,
        name: str,
        chain_id: int,
        caip2: str,
        rpc_url: str,
        explorer_url: str,
        default_asset_address: str,
        default_asset_symbol: str = "USDC",
        default_asset_decimals: int = 6,
        token_name: str = "USD Coin",
        token_version: str = "2",
    ) -> None:
        self.name = name
        self.chain_id = chain_id
        self.caip2 = caip2
        self.rpc_url = rpc_url
        self.explorer_url = explorer_url
        self.default_asset_address = default_asset_address.lower()
        self.default_asset_symbol = default_asset_symbol
        self.default_asset_decimals = default_asset_decimals
        self.token_name = token_name
        self.token_version = token_version

    def __repr__(self) -> str:
        return f"<NetworkSpec {self.name} chain_id={self.chain_id} caip2={self.caip2}>"


ARBITRUM_SEPOLIA = NetworkSpec(
    name="arbitrum-sepolia",
    chain_id=ARBITRUM_SEPOLIA_CHAIN_ID,
    caip2=ARBITRUM_SEPOLIA_CAIP2,
    rpc_url=ARBITRUM_SEPOLIA_RPC_URL,
    explorer_url=ARBITRUM_SEPOLIA_EXPLORER_URL,
    default_asset_address=ARBITRUM_SEPOLIA_USDC_ADDRESS,
    default_asset_symbol=ARBITRUM_SEPOLIA_USDC_SYMBOL,
    default_asset_decimals=ARBITRUM_SEPOLIA_USDC_DECIMALS,
    token_name=ARBITRUM_SEPOLIA_USDC_NAME,
    token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
)

BASE_SEPOLIA = NetworkSpec(
    name="base-sepolia",
    chain_id=84532,
    caip2="eip155:84532",
    rpc_url="https://sepolia.base.org",
    explorer_url="https://sepolia.basescan.org",
    default_asset_address="0x036CbD53842c5426634e7929541eC2318f3dCF7e",
    default_asset_symbol="USDC",
    default_asset_decimals=6,
    token_name="USD Coin",
    token_version="2",
)

BASE_MAINNET = NetworkSpec(
    name="base",
    chain_id=8453,
    caip2="eip155:8453",
    rpc_url="https://mainnet.base.org",
    explorer_url="https://basescan.org",
    default_asset_address="0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
    default_asset_symbol="USDC",
    default_asset_decimals=6,
    token_name="USD Coin",
    token_version="2",
)

ARBITRUM_ONE = NetworkSpec(
    name="arbitrum-one",
    chain_id=42161,
    caip2="eip155:42161",
    rpc_url="https://arb1.arbitrum.io/rpc",
    explorer_url="https://arbiscan.io",
    default_asset_address="0xaf88d065e77c8cC2239327C5EDb3A432268e5831",
    default_asset_symbol="USDC",
    default_asset_decimals=6,
    token_name="USD Coin",
    token_version="2",
)

ETHEREUM_MAINNET = NetworkSpec(
    name="ethereum",
    chain_id=1,
    caip2="eip155:1",
    rpc_url="https://cloudflare-eth.com",
    explorer_url="https://etherscan.io",
    default_asset_address="0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
    default_asset_symbol="USDC",
    default_asset_decimals=6,
    token_name="USD Coin",
    token_version="2",
)

ETHEREUM_SEPOLIA = NetworkSpec(
    name="ethereum-sepolia",
    chain_id=11155111,
    caip2="eip155:11155111",
    rpc_url="https://rpc.sepolia.org",
    explorer_url="https://sepolia.etherscan.io",
    default_asset_address="0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238",
    default_asset_symbol="USDC",
    default_asset_decimals=6,
    token_name="USD Coin",
    token_version="2",
)

KNOWN_NETWORKS: Final[dict[str, NetworkSpec]] = {
    ARBITRUM_SEPOLIA_CAIP2: ARBITRUM_SEPOLIA,
    str(ARBITRUM_SEPOLIA_CHAIN_ID): ARBITRUM_SEPOLIA,
    "arbitrum-sepolia": ARBITRUM_SEPOLIA,
    "arb-sepolia": ARBITRUM_SEPOLIA,
    "eip155:84532": BASE_SEPOLIA,
    "84532": BASE_SEPOLIA,
    "base-sepolia": BASE_SEPOLIA,
    "eip155:8453": BASE_MAINNET,
    "8453": BASE_MAINNET,
    "base": BASE_MAINNET,
    "eip155:42161": ARBITRUM_ONE,
    "42161": ARBITRUM_ONE,
    "arbitrum-one": ARBITRUM_ONE,
    "arbitrum": ARBITRUM_ONE,
    "eip155:1": ETHEREUM_MAINNET,
    "1": ETHEREUM_MAINNET,
    "ethereum": ETHEREUM_MAINNET,
    "mainnet": ETHEREUM_MAINNET,
    "eip155:11155111": ETHEREUM_SEPOLIA,
    "11155111": ETHEREUM_SEPOLIA,
    "sepolia": ETHEREUM_SEPOLIA,
}


def get_network_spec(network_or_chain_id: str | int) -> NetworkSpec | None:
    """Lookup network specification by CAIP-2 string, chain ID, or alias."""
    key = str(network_or_chain_id).strip().lower()
    return KNOWN_NETWORKS.get(key)
