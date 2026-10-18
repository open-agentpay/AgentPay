"""
AgentPay Python SDK
The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents.
"""

__version__ = "0.1.0"

from agentpay.client import (
    AgentPayClient,
    AgentPayTransport,
    AsyncAgentPayClient,
    AsyncAgentPayTransport,
    ISigner,
    LocalSigner,
)
from agentpay.core import (
    HEADER_PAYMENT_REQUIRED,
    HEADER_PAYMENT_SIGNATURE,
    PaymentAuthorization,
    PaymentChallenge,
    SignatureData,
    decode_authorization,
    decode_challenge,
    encode_authorization,
    encode_challenge,
)

__all__ = [
    "HEADER_PAYMENT_REQUIRED",
    "HEADER_PAYMENT_SIGNATURE",
    "AgentPayClient",
    "AgentPayTransport",
    "AsyncAgentPayClient",
    "AsyncAgentPayTransport",
    "ISigner",
    "LocalSigner",
    "PaymentAuthorization",
    "PaymentChallenge",
    "SignatureData",
    "__version__",
    "decode_authorization",
    "decode_challenge",
    "encode_authorization",
    "encode_challenge",
]
