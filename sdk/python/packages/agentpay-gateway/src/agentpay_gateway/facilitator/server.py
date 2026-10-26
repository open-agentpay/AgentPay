"""
ASGI Server Application Entrypoint for AgentPay Facilitator.

Supports launching via:
  uvicorn agentpay_gateway.facilitator.server:app --host 0.0.0.0 --port 8001

Environment variables:
  - RPC_URL: EVM JSON-RPC URL (defaults to Arbitrum Sepolia)
  - CHAIN_ID: EVM Chain ID (defaults to 421614)
  - USDC_ADDRESS: Token contract address (defaults to Circle USDC on Arbitrum Sepolia)
  - FACILITATOR_PRIVATE_KEY: Hex private key for transaction signing and gas sponsorship
"""

from __future__ import annotations

import os

from agentpay.evm.constants import (
    ARBITRUM_SEPOLIA_CHAIN_ID,
    ARBITRUM_SEPOLIA_RPC_URL,
    ARBITRUM_SEPOLIA_USDC_ADDRESS,
)

from agentpay_gateway.facilitator.evm_service import EvmFacilitatorService


def create_facilitator_service() -> EvmFacilitatorService:
    """Instantiate EvmFacilitatorService configured from environment variables."""
    rpc_url = os.environ.get("RPC_URL", ARBITRUM_SEPOLIA_RPC_URL)
    chain_id = int(os.environ.get("CHAIN_ID", str(ARBITRUM_SEPOLIA_CHAIN_ID)))
    token_address = os.environ.get("USDC_ADDRESS", ARBITRUM_SEPOLIA_USDC_ADDRESS)
    private_key = os.environ.get("FACILITATOR_PRIVATE_KEY")

    return EvmFacilitatorService(
        rpc_url=rpc_url,
        chain_id=chain_id,
        token_address=token_address,
        private_key=private_key,
    )


# Default singleton service and ASGI app instance
service = create_facilitator_service()
app = service.asgi_app
