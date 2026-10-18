"""
Regression test for AgentPay public subpackages.
Ensures that all convenience and canonical packages import cleanly without symbol errors.
"""

from __future__ import annotations

import importlib


def test_public_subpackages_import() -> None:
    modules = [
        "agentpay",
        "agentpay.budget",
        "agentpay.client",
        "agentpay.core",
        "agentpay.evm",
        "agentpay.integrations",
        "agentpay.integrations.langchain",
        "agentpay.policy",
        "agentpay.protocol",
        "agentpay.signer",
        "agentpay.state",
        "agentpay.testing",
        "agentpay_gateway",
        "agentpay_gateway.config",
        "agentpay_gateway.facilitator",
        "agentpay_gateway.payment",
        "agentpay_gateway.proxy",
        "agentpay_gateway.state",
    ]

    for mod in modules:
        imported = importlib.import_module(mod)
        assert imported is not None


def test_budget_subpackage_exports() -> None:
    from agentpay.budget import (
        BreakerState,
        BudgetLedgerState,
        BudgetReservation,
        LedgerEntry,
        LedgerEntryState,
        PaymentRequest,
        PolicyConfig,
        PolicyDecision,
        PolicyEvaluationResult,
        PolicyStatus,
        ReservationToken,
    )

    # Invariants and alias checks
    assert BudgetReservation is ReservationToken
    assert LedgerEntry is LedgerEntryState
    assert PolicyEvaluationResult is PolicyDecision
    assert PolicyStatus is BreakerState
    assert BudgetLedgerState.RESERVED.value == "RESERVED"
    assert PaymentRequest is not None
    assert PolicyConfig is not None
    assert BreakerState.CLOSED.value == "CLOSED"


def test_signer_subpackage_exports() -> None:
    from agentpay.signer import (
        EIP3009_MESSAGE_TYPES,
        TRANSFER_WITH_AUTHORIZATION_TYPE,
        InvalidPrivateKeyError,
        ISigner,
        LocalSigner,
        ReservationMismatchError,
        SignerError,
        SignerInterface,
        UnsupportedNetworkError,
        generate_nonce,
        parse_caip2_chain_id,
    )

    assert SignerInterface is ISigner
    assert TRANSFER_WITH_AUTHORIZATION_TYPE is EIP3009_MESSAGE_TYPES
    assert callable(generate_nonce)
    assert callable(parse_caip2_chain_id)
    assert LocalSigner is not None
    assert issubclass(InvalidPrivateKeyError, SignerError)
    assert issubclass(ReservationMismatchError, SignerError)
    assert issubclass(UnsupportedNetworkError, SignerError)
