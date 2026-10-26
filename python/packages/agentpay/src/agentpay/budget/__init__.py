"""
AgentPay Budget sub-package.

Exposes budget models, ledger records, and reservation tokens for buyer spending limits.
"""

from __future__ import annotations

from agentpay.core.state import BudgetLedgerState
from agentpay.policy.models import (
    BreakerState,
    LedgerEntryState,
    PaymentRequest,
    PolicyConfig,
    PolicyDecision,
    ReservationToken,
    WhitelistMode,
)

# Backward compatibility / convenient domain aliases
BudgetReservation = ReservationToken
LedgerEntry = LedgerEntryState
PolicyEvaluationResult = PolicyDecision
PolicyStatus = BreakerState

__all__ = [
    "BreakerState",
    "BudgetLedgerState",
    "BudgetReservation",
    "LedgerEntry",
    "LedgerEntryState",
    "PaymentRequest",
    "PolicyConfig",
    "PolicyDecision",
    "PolicyEvaluationResult",
    "PolicyStatus",
    "ReservationToken",
    "WhitelistMode",
]
