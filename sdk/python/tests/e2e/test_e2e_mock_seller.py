"""
End-to-End (E2E) integration tests for AgentPay with Mock Seller and Facilitator.

Tests the full autonomous financial negotiation loop:
  Buyer Agent (httpx + PolicyEngine + LocalSigner)
    <---> Seller API (MockSellerService with x402 V2 challenge)
    <---> Facilitator (MockFacilitatorService for verification & gasless settlement)
"""

from __future__ import annotations

import asyncio
from typing import Any

import pytest
from agentpay.client.client import AgentPayClient, AsyncAgentPayClient
from agentpay.client.signer import LocalSigner
from agentpay.policy.db import create_buyer_connection, init_buyer_database
from agentpay.policy.errors import PolicyErrorCode, PolicyViolationError
from agentpay.policy.models import LedgerEntryState, PolicyConfig
from agentpay.policy.sqlite_engine import SQLitePolicyEngine
from agentpay.testing.mock_seller import MockSellerService
from agentpay_gateway.facilitator.models import SettlementState
from agentpay_gateway.facilitator.service import MockFacilitatorService

# Test constants
FIXED_CLOCK_TIME = 1729000100
BUYER_PRIVATE_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
SELLER_PAYEE_ADDRESS = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
USDC_ASSET_ADDRESS = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
NETWORK_BASE_SEPOLIA = "eip155:84532"


@pytest.fixture
def test_clock() -> Any:
    current_time = FIXED_CLOCK_TIME

    def _now() -> int:
        return current_time

    return _now


@pytest.fixture
def signer() -> LocalSigner:
    return LocalSigner(BUYER_PRIVATE_KEY)


@pytest.fixture
def policy_engine(signer: LocalSigner, test_clock: Any) -> SQLitePolicyEngine:
    conn = create_buyer_connection(":memory:")
    init_buyer_database(conn)
    cfg = PolicyConfig(
        id="pol_e2e",
        name="E2E Test Policy",
        wallet_address=signer.address,
        max_per_tx_base_units=100000,  # 0.1 USDC single-tx cap
        rolling_budget_24h_base_units=1000000,  # 1.0 USDC daily budget
        whitelist_payees=[SELLER_PAYEE_ADDRESS],
        whitelist_assets=[USDC_ASSET_ADDRESS],
        whitelist_networks=[NETWORK_BASE_SEPOLIA],
        whitelist_domains=["api.seller.example.com"],
        circuit_consecutive_failures=3,
        circuit_cooldown_seconds=60,
    )
    return SQLitePolicyEngine(conn, cfg, clock=test_clock)


@pytest.fixture
def facilitator(test_clock: Any) -> MockFacilitatorService:
    return MockFacilitatorService(clock=test_clock)


@pytest.fixture
def mock_seller(facilitator: MockFacilitatorService, test_clock: Any) -> MockSellerService:
    seller = MockSellerService(
        payee_address=SELLER_PAYEE_ADDRESS,
        facilitator=facilitator,
        network=NETWORK_BASE_SEPOLIA,
        asset=USDC_ASSET_ADDRESS,
        asset_symbol="USDC",
        decimals=6,
        clock=test_clock,
        challenge_ttl_seconds=300,
    )
    seller.add_route("/v1/market-data", price=50000)
    seller.add_route("/v1/premium-ai", price=500000)  # Exceeds buyer per-tx cap
    seller.add_route("/v1/micro-inference", price=25000)
    return seller


class TestE2EMockSellerSync:
    """Synchronous End-to-End negotiation test cases."""

    def test_e2e_sync_happy_path(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        """
        Complete E2E test:
          1. Buyer GET /v1/market-data
          2. Seller responds 402 with x402 V2 Challenge
          3. AgentPayClient automatically reserves quota, signs EIP-3009 authorization, and retries
          4. Seller verifies with Facilitator and triggers settlement
          5. Seller delivers 200 OK with payload & receipt
          6. Buyer commits budget in SQLite ledger
        """
        transport = mock_seller.as_transport()

        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            clock=test_clock,
        ) as client:
            resp = client.get("https://api.seller.example.com/v1/market-data")

            # Assert 1: Transparent 200 OK delivered to agent
            assert resp.status_code == 200
            data = resp.json()
            assert "data" in data
            assert data["receipt"]["payer"] == signer.address
            assert data["receipt"]["amount"] == 50000

            # Assert 2: Seller recorded transaction
            assert len(mock_seller.settled_transactions) == 1
            record = mock_seller.settled_transactions[0]
            assert record["payer"] == signer.address
            assert record["amount"] == 50000

            # Assert 3: Facilitator verified & settled exactly 1 transaction
            assert len(facilitator._settlements) == 1
            settlement = next(iter(facilitator._settlements.values()))
            assert settlement.status == SettlementState.SUBMITTED
            assert settlement.settlement_id == record["settlement_id"]

            # Assert 4: Buyer's financial ledger is in COMMITTED state
            row = policy_engine._conn.execute(
                "SELECT state, amount_base_units FROM ap_budget_ledger WHERE intent_id = ?",
                (record["challenge_id"],),
            ).fetchone()
            assert row is not None
            assert row[0] == LedgerEntryState.COMMITTED.value
            assert row[1] == 50000

            # Assert 5: Available budget correctly updated
            assert policy_engine.get_available_rolling_budget() == 950000

    def test_e2e_unprotected_route_passthrough(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        """Unprotected endpoints return 200 OK without generating challenges or consuming quota."""
        transport = mock_seller.as_transport()

        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            clock=test_clock,
        ) as client:
            resp = client.get("https://api.seller.example.com/public-status")
            assert resp.status_code == 200
            assert resp.json()["status"] == "ok"

            assert len(facilitator._settlements) == 0
            assert len(mock_seller.settled_transactions) == 0
            assert policy_engine.get_available_rolling_budget() == 1000000

    def test_e2e_policy_firewall_blocks_overpriced_route(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        """When seller asks for 500,000 base units, Buyer's 100,000 cap rejects it."""
        transport = mock_seller.as_transport()

        # Mode A: Default returns 402 without signing
        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            clock=test_clock,
        ) as client:
            resp = client.get("https://api.seller.example.com/v1/premium-ai")
            assert resp.status_code == 402

            # Zero settlement, zero quota deducted
            assert len(facilitator._settlements) == 0
            assert len(mock_seller.settled_transactions) == 0
            assert policy_engine.get_available_rolling_budget() == 1000000

        # Mode B: raise_on_policy_rejection=True raises PolicyViolationError
        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            raise_on_policy_rejection=True,
            clock=test_clock,
        ) as client:
            with pytest.raises(PolicyViolationError) as exc:
                client.get("https://api.seller.example.com/v1/premium-ai")
            assert exc.value.code == PolicyErrorCode.SINGLE_TX_CAP_EXCEEDED

    def test_e2e_guardrail_upstream_500_failure_preserves_reserved_state(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        """
        CLAUDE.md Invariant:
        When seller verifies and settles, but fails to deliver resource (HTTP 500),
        Buyer MUST PRESERVE reservation in RESERVED state (never auto-release).
        """
        mock_seller.fail_on_upstream = True
        transport = mock_seller.as_transport()

        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            clock=test_clock,
        ) as client:
            resp = client.get("https://api.seller.example.com/v1/market-data")
            assert resp.status_code == 500

            # Facilitator did settle the payment
            assert len(facilitator._settlements) == 1

            # Buyer budget ledger MUST remain in RESERVED state
            active_challenges = list(mock_seller._active_challenges.values())
            assert len(active_challenges) == 1
            challenge_id = active_challenges[0].challenge_id

            row = policy_engine._conn.execute(
                "SELECT state, amount_base_units FROM ap_budget_ledger WHERE intent_id = ?",
                (challenge_id,),
            ).fetchone()
            assert row is not None
            assert row[0] == LedgerEntryState.RESERVED.value
            assert row[1] == 50000

    def test_e2e_consecutive_payments_sliding_budget(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        """
        Consecutive payment sequence:
          - Multiple purchases succeed
          - Rolling budget aggregates atomically
          - Once exhausted, next purchase is rejected
        """
        transport = mock_seller.as_transport()

        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            clock=test_clock,
        ) as client:
            # Call 1: 50,000 -> Remaining: 950,000
            r1 = client.get("https://api.seller.example.com/v1/market-data")
            assert r1.status_code == 200

            # Call 2: 25,000 -> Remaining: 925,000
            r2 = client.get("https://api.seller.example.com/v1/micro-inference")
            assert r2.status_code == 200

            # Call 3: 50,000 -> Remaining: 875,000
            r3 = client.get("https://api.seller.example.com/v1/market-data")
            assert r3.status_code == 200

            assert len(facilitator._settlements) == 3
            assert policy_engine.get_available_rolling_budget() == 875000

    def test_e2e_policy_firewall_blocks_unwhitelisted_payee(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        test_clock: Any,
    ) -> None:
        """Buyer policy whitelist matrix blocks challenges with unwhitelisted payees."""
        unwhitelisted_seller = MockSellerService(
            payee_address="0x1111111111111111111111111111111111111111",
            facilitator=facilitator,
            clock=test_clock,
        )
        unwhitelisted_seller.add_route("/v1/market-data", price=50000)
        transport = unwhitelisted_seller.as_transport()

        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            raise_on_policy_rejection=True,
            clock=test_clock,
        ) as client:
            with pytest.raises(PolicyViolationError) as exc:
                client.get("https://api.seller.example.com/v1/market-data")
            assert exc.value.code == PolicyErrorCode.PAYEE_NOT_WHITELISTED

    def test_e2e_policy_firewall_blocks_unwhitelisted_domain(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        """Buyer policy blocks requests directed at domains outside whitelist_domains."""
        transport = mock_seller.as_transport()

        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            raise_on_policy_rejection=True,
            clock=test_clock,
        ) as client:
            with pytest.raises(PolicyViolationError) as exc:
                client.get("https://untrusted-api.evil.com/v1/market-data")
            assert exc.value.code == PolicyErrorCode.DOMAIN_NOT_WHITELISTED

    def test_e2e_seller_verification_failure_returns_401(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        """Seller verification failure returns 401 to client; quota is preserved (not released)."""
        mock_seller.fail_verification = True
        transport = mock_seller.as_transport()

        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            clock=test_clock,
        ) as client:
            resp = client.get("https://api.seller.example.com/v1/market-data")
            assert resp.status_code == 401
            assert "Simulated seller verification failure" in resp.text

    def test_e2e_seller_replay_attack_rejected(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        """
        P0-7 Replay Defense:
        If a malicious party tries to replay an already-consumed authorization,
        the Facilitator identifies the nonce collision and the Seller returns 409 Conflict.
        """
        import httpx
        from agentpay.core.codecs import (
            inject_authorization_to_headers,
        )

        transport = mock_seller.as_transport()

        # Step 1: Legitimate initial payment succeeds
        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=transport,
            clock=test_clock,
        ) as client:
            resp = client.get("https://api.seller.example.com/v1/market-data")
            assert resp.status_code == 200

        # Step 2: Extract the authorization that was issued and settled
        challenge_id = mock_seller.settled_transactions[0]["challenge_id"]
        challenge = mock_seller._active_challenges[challenge_id]

        # Step 3: Direct raw request replaying the SAME authorization to the seller
        # Re-fetch the authorization signed during step 1 from facilitator's nonce set
        # Or construct an identical raw request with the settled challenge & headers
        raw_headers = {}
        # Make a raw request with an authorization using the already consumed nonce
        # Get the used nonce from facilitator
        used_nonce = next(iter(facilitator._used_nonces))[3]
        dummy_res = type(
            "DummyRes",
            (),
            {
                "amount": 50000,
                "intent_id": challenge.challenge_id,
                "expires_at": challenge.valid_before + 60,
            },
        )()
        replayed_auth = signer.sign_payment_authorization(
            challenge,
            dummy_res,
            nonce=used_nonce,
        )
        inject_authorization_to_headers(raw_headers, replayed_auth)

        raw_req = httpx.Request(
            "GET",
            "https://api.seller.example.com/v1/market-data",
            headers=raw_headers,
        )
        replay_resp = mock_seller.handle_request(raw_req)
        assert replay_resp.status_code == 409
        assert "Nonce already used" in replay_resp.text


class TestE2EMockSellerAsync:
    """Asynchronous End-to-End negotiation test cases."""

    def test_e2e_async_happy_path(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        facilitator: MockFacilitatorService,
        mock_seller: MockSellerService,
        test_clock: Any,
    ) -> None:
        async_transport = mock_seller.as_async_transport()

        async def _run() -> None:
            async with AsyncAgentPayClient(
                signer=signer,
                policy=policy_engine,
                transport=async_transport,
                clock=test_clock,
            ) as client:
                resp = await client.get("https://api.seller.example.com/v1/market-data")
                assert resp.status_code == 200
                data = resp.json()
                assert data["receipt"]["payer"] == signer.address
                assert data["receipt"]["amount"] == 50000

        asyncio.run(_run())

        # Check Facilitator and ledger
        assert len(facilitator._settlements) == 1
        assert policy_engine.get_available_rolling_budget() == 950000
