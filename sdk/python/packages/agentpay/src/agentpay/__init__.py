"""
AgentPay Python SDK
The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents.
"""

from __future__ import annotations

import sys
import agent_core

__version__ = "0.2.8"

# 1. 动态将 agent_core 的子模块映射到 agentpay.* 命名空间，完美向后兼容深层 import
_MODULE_MAP = {
    "core": "agent_core.core",
    "core.codecs": "agent_core.core.codecs",
    "core.errors": "agent_core.core.errors",
    "core.interop": "agent_core.core.interop",
    "core.models": "agent_core.core.models",
    "core.state": "agent_core.core.state",
    "policy": "agent_core.policy",
    "policy.config": "agent_core.policy.config",
    "policy.db": "agent_core.policy.db",
    "policy.errors": "agent_core.policy.errors",
    "policy.evaluator": "agent_core.policy.evaluator",
    "policy.interfaces": "agent_core.policy.interfaces",
    "policy.metered": "agent_core.policy.metered",
    "policy.models": "agent_core.policy.models",
    "policy.sqlite_engine": "agent_core.policy.sqlite_engine",
    "client": "agent_core.client",
    "client.client": "agent_core.client.client",
    "client.errors": "agent_core.client.errors",
    "client.interfaces": "agent_core.client.interfaces",
    "client.signer": "agent_core.client.signer",
    "client.transport": "agent_core.client.transport",
    "state": "agent_core.state",
    "state.buyer_coordinator": "agent_core.state.buyer_coordinator",
    "state.errors": "agent_core.state.errors",
    "state.guards": "agent_core.state.guards",
    "state.reconciliation": "agent_core.state.reconciliation",
    "budget": "agent_core.budget",
    "signer": "agent_core.client.signer",
    "protocol": "agent_core.protocol",
    "protocol.codecs": "agent_core.core.codecs",
    "protocol.errors": "agent_core.core.errors",
    "protocol.interop": "agent_core.core.interop",
    "protocol.models": "agent_core.core.models",
    "protocol.state": "agent_core.core.state",
    "integrations": "agent_core.integrations",
    "integrations.langchain": "agent_core.integrations.langchain",
    "integrations.langchain.http": "agent_core.integrations.langchain.http",
    "integrations.langchain.tool": "agent_core.integrations.langchain.tool",
    "testing": "agent_core.testing",
    "testing.mock_seller": "agent_core.testing.mock_seller",
}

for _sub_name, _target_path in _MODULE_MAP.items():
    try:
        _mod = __import__(_target_path, fromlist=["*"])
        sys.modules[f"agentpay.{_sub_name}"] = _mod
        if "." not in _sub_name:
            globals()[_sub_name] = _mod
        else:
            _parent, _child = _sub_name.split(".", 1)
            if _parent in globals() and "." not in _child:
                setattr(globals()[_parent], _child, _mod)
    except ImportError:
        pass

if "signer" in globals():
    setattr(signer, "SignerInterface", getattr(signer, "ISigner", None))
    setattr(signer, "TRANSFER_WITH_AUTHORIZATION_TYPE", getattr(signer, "EIP3009_MESSAGE_TYPES", None))


def __getattr__(name: str):
    if name in _MODULE_MAP:
        return sys.modules.get(f"agentpay.{name}")
    raise AttributeError(f"module 'agentpay' has no attribute '{name}'")


# 2. 导出顶层公共 API
from agent_core.client import (
    AgentPayClient,
    AgentPayTransport,
    AsyncAgentPayClient,
    AsyncAgentPayTransport,
    ISigner,
    LocalSigner,
)
from agent_core.core import (
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
