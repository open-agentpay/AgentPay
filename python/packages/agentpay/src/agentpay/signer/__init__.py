"""
AgentPay Signer sub-package.

Provides cryptographic signers for EIP-712 and EIP-3009 payment authorizations.
"""

from __future__ import annotations

from agentpay.client.errors import (
    InvalidPrivateKeyError,
    ReservationMismatchError,
    SignerError,
    UnsupportedNetworkError,
)
from agentpay.client.interfaces import ISigner
from agentpay.client.signer import (
    EIP3009_MESSAGE_TYPES,
    LocalSigner,
    generate_nonce,
    parse_caip2_chain_id,
)

# Aliases for convenience / naming consistency
SignerInterface = ISigner
TRANSFER_WITH_AUTHORIZATION_TYPE = EIP3009_MESSAGE_TYPES

__all__ = [
    "EIP3009_MESSAGE_TYPES",
    "ISigner",
    "InvalidPrivateKeyError",
    "LocalSigner",
    "ReservationMismatchError",
    "SignerError",
    "SignerInterface",
    "TRANSFER_WITH_AUTHORIZATION_TYPE",
    "UnsupportedNetworkError",
    "generate_nonce",
    "parse_caip2_chain_id",
]
