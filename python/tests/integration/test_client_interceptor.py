"""
Unit tests for httpx transparent 402 interceptor (AgentPayTransport and AgentPayClient).
"""

from __future__ import annotations

import asyncio
from typing import Any

import httpx
import pytest
from agentpay.client.client import AgentPayClient, AsyncAgentPayClient
from agentpay.client.signer import LocalSigner
from agentpay.core.codecs import (
    HEADER_PAYMENT_REQUIRED,
    HEADER_PAYMENT_SIGNATURE,
    decode_authorization,
    encode_challenge,
)
from agentpay.core.models import PaymentChallenge
from agentpay.policy.db import create_buyer_connection, init_buyer_database
from agentpay.policy.errors import PolicyErrorCode, PolicyViolationError
from agentpay.policy.models import LedgerEntryState, PolicyConfig
from agentpay.policy.sqlite_engine import SQLitePolicyEngine


class MockServerTransport(httpx.BaseTransport):
    """
    Mock backend server that requires payment for protected endpoints.
    Verifies incoming PAYMENT-SIGNATURE header.
    """

    def __init__(self, challenge: PaymentChallenge) -> None:
        super().__init__()
        self.challenge = challenge
        self.payment_received: list[dict[str, Any]] = []
        self.fail_on_replay: bool = False

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        path = request.url.path

        # Unprotected endpoint
        if path == "/public":
            return httpx.Response(200, json={"message": "free"}, request=request)

        # Protected endpoint: check for PAYMENT-SIGNATURE header
        sig_header = request.headers.get(HEADER_PAYMENT_SIGNATURE)
        if not sig_header:
            # Emit 402 Payment Required with PAYMENT-REQUIRED header
            ch_encoded = encode_challenge(self.challenge)
            headers = {
                HEADER_PAYMENT_REQUIRED: ch_encoded,
                "Content-Type": "application/json",
            }
            return httpx.Response(
                402,
                headers=headers,
                json={"error": "Payment Required", "price": self.challenge.amount},
                request=request,
            )

        # Verify signature
        if self.fail_on_replay:
            return httpx.Response(500, json={"error": "Upstream error"}, request=request)

        auth = decode_authorization(sig_header)
        is_valid = LocalSigner.verify_authorization(auth, self.challenge)
        if not is_valid:
            return httpx.Response(401, json={"error": "Invalid signature"}, request=request)

        self.payment_received.append({"payer": auth.payer, "amount": self.challenge.amount})
        return httpx.Response(
            200,
            json={"data": "Protected Market Data", "receipt": auth.challenge_id},
            request=request,
        )


class AsyncMockServerTransport(httpx.AsyncBaseTransport):
    """Async variant of MockServerTransport for AsyncAgentPayClient."""

    def __init__(self, challenge: PaymentChallenge) -> None:
        super().__init__()
        self._sync = MockServerTransport(challenge)

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        return self._sync.handle_request(request)


@pytest.fixture
def signer() -> LocalSigner:
    return LocalSigner("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")


@pytest.fixture
def policy_engine(signer: LocalSigner) -> SQLitePolicyEngine:
    conn = create_buyer_connection(":memory:")
    init_buyer_database(conn)
    cfg = PolicyConfig(
        id="pol_test",
        name="Test Policy",
        wallet_address=signer.address,
        max_per_tx_base_units=100000,  # 0.1 USDC cap
        rolling_budget_24h_base_units=1000000,  # 1.0 USDC budget
        whitelist_payees=[
            "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
            "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc",
        ],
        whitelist_assets=["0x036CbD53842c5426634e7929541eC2318f3dCF7e"],
        whitelist_networks=["eip155:84532"],
        whitelist_domains=["api.example.com"],
    )
    return SQLitePolicyEngine(conn, cfg, clock=lambda: 1729000100)


@pytest.fixture
def standard_challenge() -> PaymentChallenge:
    return PaymentChallenge(
        version="2.0",
        challengeId="ch_interceptor_01",
        network="eip155:84532",
        asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",
        assetSymbol="USDC",
        decimals=6,
        amount="50000",  # 0.05 USDC
        payee="0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
        scheme="EIP-3009",
        validAfter=1729000000,
        validBefore=1729000300,
        facilitatorUrl="https://facilitator.example.com",
        resourceUri="/v1/market-data",
    )


class TestAgentPayClientSync:
    def test_unprotected_endpoint_passthrough(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        standard_challenge: PaymentChallenge,
    ) -> None:
        mock_server = MockServerTransport(standard_challenge)
        with AgentPayClient(signer=signer, policy=policy_engine, transport=mock_server) as client:
            resp = client.get("https://api.example.com/public")
            assert resp.status_code == 200
            assert resp.json() == {"message": "free"}
            assert len(mock_server.payment_received) == 0

    def test_automatic_402_payment_and_commit(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        standard_challenge: PaymentChallenge,
    ) -> None:
        mock_server = MockServerTransport(standard_challenge)
        with AgentPayClient(signer=signer, policy=policy_engine, transport=mock_server) as client:
            # Client calls protected endpoint: 402 is intercepted, signed, and replayed automatically
            resp = client.get("https://api.example.com/v1/market-data")

            # Final response is transparently 200 OK!
            assert resp.status_code == 200
            assert resp.json()["data"] == "Protected Market Data"
            assert len(mock_server.payment_received) == 1
            assert mock_server.payment_received[0]["payer"] == signer.address

            # Check that budget was COMMITTED in policy ledger directly from database table
            row = policy_engine._conn.execute(
                "SELECT state, amount_base_units FROM ap_budget_ledger WHERE intent_id = ?",
                (standard_challenge.challenge_id,),
            ).fetchone()
            assert row is not None
            assert row[0] == LedgerEntryState.COMMITTED.value
            assert row[1] == 50000

    def test_policy_rejection_single_tx_cap(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
    ) -> None:
        # Challenge with amount 500,000 (> max_per_tx_base_units of 100,000)
        expensive_challenge = PaymentChallenge(
            version="2.0",
            challengeId="ch_expensive",
            network="eip155:84532",
            asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",
            assetSymbol="USDC",
            decimals=6,
            amount="500000",
            payee="0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
            scheme="EIP-3009",
            validAfter=1729000000,
            validBefore=1729000300,
            facilitatorUrl="https://facilitator.example.com",
        )
        mock_server = MockServerTransport(expensive_challenge)

        # Mode A: raise_on_policy_rejection=False (default) returns raw 402
        with AgentPayClient(signer=signer, policy=policy_engine, transport=mock_server) as client:
            resp = client.get("https://api.example.com/expensive")
            assert resp.status_code == 402
            assert len(mock_server.payment_received) == 0

        # Mode B: raise_on_policy_rejection=True raises PolicyViolationError
        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=mock_server,
            raise_on_policy_rejection=True,
        ) as client:
            with pytest.raises(PolicyViolationError) as exc:
                client.get("https://api.example.com/expensive")
            assert exc.value.code == PolicyErrorCode.SINGLE_TX_CAP_EXCEEDED

    def test_guardrail_signature_sent_failure_does_not_release(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        standard_challenge: PaymentChallenge,
    ) -> None:
        """
        CLAUDE.md Safety Rule:
        Once signature is transmitted on the wire, failure must NOT auto-release.
        Reservation stays in RESERVED state until expiry or reconcile().
        """
        mock_server = MockServerTransport(standard_challenge)
        mock_server.fail_on_replay = True  # Upstream returns 500 on retry

        with AgentPayClient(signer=signer, policy=policy_engine, transport=mock_server) as client:
            resp = client.get("https://api.example.com/v1/market-data")
            assert resp.status_code == 500

            # Inspect database: reservation must REMAIN in RESERVED state, NOT RELEASED!
            row = policy_engine._conn.execute(
                "SELECT state, amount_base_units FROM ap_budget_ledger WHERE intent_id = ?",
                (standard_challenge.challenge_id,),
            ).fetchone()
            assert row is not None
            assert row[0] == LedgerEntryState.RESERVED.value

    def test_auto_retry_disabled_returns_402(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        standard_challenge: PaymentChallenge,
    ) -> None:
        mock_server = MockServerTransport(standard_challenge)
        with AgentPayClient(
            signer=signer,
            policy=policy_engine,
            transport=mock_server,
            auto_retry_on_402=False,
        ) as client:
            resp = client.get("https://api.example.com/v1/market-data")
            assert resp.status_code == 402
            assert len(mock_server.payment_received) == 0

    def test_expired_challenge_passes_through(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
    ) -> None:
        # Challenge that expired before policy clock (clock is 1729000100)
        expired_ch = PaymentChallenge(
            version="2.0",
            challengeId="ch_expired",
            network="eip155:84532",
            asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",
            assetSymbol="USDC",
            decimals=6,
            amount="50000",
            payee="0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
            scheme="EIP-3009",
            validAfter=1728000000,
            validBefore=1728000100,  # Expired in past
            facilitatorUrl="https://facilitator.example.com",
        )
        mock_server = MockServerTransport(expired_ch)
        with AgentPayClient(signer=signer, policy=policy_engine, transport=mock_server) as client:
            resp = client.get("https://api.example.com/v1/market-data")
            # Should not attempt payment, passes raw 402 through
            assert resp.status_code == 402
            assert len(mock_server.payment_received) == 0

    def test_signing_failure_releases_quota_immediately(
        self,
        policy_engine: SQLitePolicyEngine,
        standard_challenge: PaymentChallenge,
    ) -> None:
        """If signer throws exception before transmission, quota must be immediately released."""

        class FailingSigner(LocalSigner):
            def sign_payment_authorization(self, challenge: Any, reservation: Any) -> Any:
                raise RuntimeError("Hardware security module timeout")

        failing_signer = FailingSigner("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
        mock_server = MockServerTransport(standard_challenge)

        with AgentPayClient(signer=failing_signer, policy=policy_engine, transport=mock_server) as client:
            with pytest.raises(RuntimeError, match="Hardware security module timeout"):
                client.get("https://api.example.com/v1/market-data")

            # Check database: reservation was created then immediately RELEASED
            row = policy_engine._conn.execute(
                "SELECT state FROM ap_budget_ledger WHERE intent_id = ?",
                (standard_challenge.challenge_id,),
            ).fetchone()
            assert row is not None
            assert row[0] == LedgerEntryState.RELEASED.value


class TestAgentPayClientAsync:
    def test_async_client_automatic_payment(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
        standard_challenge: PaymentChallenge,
    ) -> None:
        mock_server = AsyncMockServerTransport(standard_challenge)

        async def _run() -> None:
            async with AsyncAgentPayClient(signer=signer, policy=policy_engine, transport=mock_server) as client:
                resp = await client.get("https://api.example.com/v1/market-data")
                assert resp.status_code == 200
                assert resp.json()["data"] == "Protected Market Data"

        asyncio.run(_run())

    def test_async_client_factory_and_policy_rejection(
        self,
        signer: LocalSigner,
        policy_engine: SQLitePolicyEngine,
    ) -> None:
        expensive_challenge = PaymentChallenge(
            version="2.0",
            challengeId="ch_expensive_async",
            network="eip155:84532",
            asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",
            assetSymbol="USDC",
            decimals=6,
            amount="500000",
            payee="0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
            scheme="EIP-3009",
            validAfter=1729000000,
            validBefore=1729000300,
            facilitatorUrl="https://facilitator.example.com",
        )
        mock_server = AsyncMockServerTransport(expensive_challenge)

        sync_client = AgentPayClient(
            signer=signer,
            policy=policy_engine,
            raise_on_policy_rejection=True,
        )

        async def _run() -> None:
            async with sync_client.async_client(transport=mock_server) as async_client:
                with pytest.raises(PolicyViolationError) as exc:
                    await async_client.get("https://api.example.com/expensive")
                assert exc.value.code == PolicyErrorCode.SINGLE_TX_CAP_EXCEEDED

        asyncio.run(_run())
        sync_client.close()
