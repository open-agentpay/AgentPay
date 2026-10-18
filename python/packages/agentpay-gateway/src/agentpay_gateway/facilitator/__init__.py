"""
AgentPay Gateway Facilitator Settlement Service and Client.

Provides:
- MockFacilitatorService: In-memory clearing & testing node
- EvmFacilitatorService: Arbitrum Sepolia on-chain EIP-3009 settlement clearing node
- FacilitatorClient / AsyncFacilitatorClient: HTTP clients for gateway payment verification
"""

from __future__ import annotations

from agentpay_gateway.facilitator.client import AsyncFacilitatorClient, FacilitatorClient
from agentpay_gateway.facilitator.errors import (
    ChallengeExpiredError,
    FacilitatorAPIError,
    FacilitatorConnectionError,
    FacilitatorError,
    InvalidRequestError,
    InvalidSignatureError,
    NonceAlreadyUsedError,
    SettlementNotFoundError,
)
from agentpay_gateway.facilitator.evm_service import EvmFacilitatorService
from agentpay_gateway.facilitator.models import (
    SettlementState,
    SettlementStatus,
    SettleRequest,
    SettleResponse,
    VerifyRequest,
    VerifyResponse,
)
from agentpay_gateway.facilitator.service import MockFacilitatorService, MockFacilitatorTransport

__all__ = [
    "AsyncFacilitatorClient",
    "ChallengeExpiredError",
    "EvmFacilitatorService",
    "FacilitatorAPIError",
    "FacilitatorClient",
    "FacilitatorConnectionError",
    "FacilitatorError",
    "InvalidRequestError",
    "InvalidSignatureError",
    "MockFacilitatorService",
    "MockFacilitatorTransport",
    "NonceAlreadyUsedError",
    "SettleRequest",
    "SettleResponse",
    "SettlementNotFoundError",
    "SettlementState",
    "SettlementStatus",
    "VerifyRequest",
    "VerifyResponse",
]
