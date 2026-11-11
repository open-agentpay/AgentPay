"""
AgentPay Python SDK
The Open-Source Payment Infrastructure & Financial Firewall for Autonomous AI Agents.
"""

from __future__ import annotations

import sys
import agent_core

__version__ = "0.4.0"

# 1. 动态将 agent_core 的核心子模块映射到 agentpay.* 命名空间
_CORE_MODULE_MAP = {
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
    "client.transport": "agent_core.client.transport",
    "state": "agent_core.state",
    "state.buyer_coordinator": "agent_core.state.buyer_coordinator",
    "state.errors": "agent_core.state.errors",
    "state.guards": "agent_core.state.guards",
    "state.reconciliation": "agent_core.state.reconciliation",
    "budget": "agent_core.budget",
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
}

for _sub_name, _target_path in _CORE_MODULE_MAP.items():
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

# 2. 导出/注册 EVM Signer 基础设施到 agentpay.signer 与 agentpay.client.signer
from agentpay.evm import signer as _evm_signer

sys.modules["agentpay.client.signer"] = _evm_signer
sys.modules["agentpay.signer"] = _evm_signer
signer = _evm_signer

if "client" in globals():
    setattr(client, "signer", _evm_signer)

# 3. 映射依赖 signer 的 testing 子模块
_TESTING_MODULE_MAP = {
    "testing": "agent_core.testing",
    "testing.mock_seller": "agent_core.testing.mock_seller",
}

for _sub_name, _target_path in _TESTING_MODULE_MAP.items():
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

_MODULE_MAP = {**_CORE_MODULE_MAP, **_TESTING_MODULE_MAP}

if "signer" in globals():
    setattr(signer, "SignerInterface", getattr(signer, "ISigner", None))
    setattr(signer, "TRANSFER_WITH_AUTHORIZATION_TYPE", getattr(signer, "EIP3009_MESSAGE_TYPES", None))


def __getattr__(name: str):
    if name in _MODULE_MAP:
        return sys.modules.get(f"agentpay.{name}")
    raise AttributeError(f"module 'agentpay' has no attribute '{name}'")


from agent_core.client import (
    AgentPayClient,
    AgentPayTransport,
    AsyncAgentPayClient,
    AsyncAgentPayTransport,
    ISigner,
)
from agentpay.evm.signer import (
    EIP3009_MESSAGE_TYPES,
    LocalSigner,
    SignatureData,
    _validate_evm_address,
    generate_nonce,
    parse_caip2_chain_id,
)
from agent_core.core import (
    HEADER_PAYMENT_REQUIRED,
    HEADER_PAYMENT_SIGNATURE,
    PaymentAuthorization,
    PaymentChallenge,
    decode_authorization,
    decode_challenge,
    encode_authorization,
    encode_challenge,
)
from agent_core.policy import (
    BreakerState,
    IPolicyEngine,
    LedgerEntryState,
    PaymentRequest,
    PolicyConfig,
    PolicyDecision,
    PolicyErrorCode,
    PolicyViolationError,
    ReservationToken,
    SQLitePolicyEngine,
    StaticEvaluationResult,
    WhitelistMode,
    create_buyer_connection,
    evaluate_static_rules,
    init_buyer_database,
    load_policy_config,
)
from collections.abc import Mapping

# Register SignatureData factory with core PaymentAuthorization
PaymentAuthorization.register_signature_factory(
    lambda v: SignatureData(**v) if isinstance(v, Mapping) else v
)

for _mod_key in ("agentpay.core.models", "agentpay.protocol.models"):
    if _mod_key in sys.modules:
        setattr(sys.modules[_mod_key], "SignatureData", SignatureData)
        setattr(sys.modules[_mod_key], "_validate_evm_address", _validate_evm_address)

__all__ = [
    "HEADER_PAYMENT_REQUIRED",
    "HEADER_PAYMENT_SIGNATURE",
    "AgentPayClient",
    "AgentPayTransport",
    "AsyncAgentPayClient",
    "AsyncAgentPayTransport",
    "BreakerState",
    "IPolicyEngine",
    "ISigner",
    "LedgerEntryState",
    "LocalSigner",
    "PaymentAuthorization",
    "PaymentChallenge",
    "PaymentRequest",
    "PolicyConfig",
    "PolicyDecision",
    "PolicyErrorCode",
    "PolicyViolationError",
    "ReservationToken",
    "SQLitePolicyEngine",
    "SignatureData",
    "StaticEvaluationResult",
    "WhitelistMode",
    "__version__",
    "create_buyer_connection",
    "decode_authorization",
    "decode_challenge",
    "encode_authorization",
    "encode_challenge",
    "evaluate_static_rules",
    "init_buyer_database",
    "load_policy_config",
]
