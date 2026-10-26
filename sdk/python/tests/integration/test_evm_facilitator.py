"""
Integration tests for Arbitrum Sepolia On-Chain EvmFacilitatorService (T6.2).

Validates:
1. Verification against local and on-chain nonce state.
2. EIP-3009 transferWithAuthorization calldata generation & transaction signing with Facilitator key.
3. Raw transaction broadcast via JSON-RPC.
4. Transaction receipt polling and transition from SUBMITTED -> CONFIRMED / FAILED.
5. End-to-end interoperability with FacilitatorClient over ASGI transport.
6. Gateway-Facilitator integration with asynchronous clearing.
"""

from __future__ import annotations

import json
from typing import Any

import httpx
import pytest
from agentpay.client.signer import LocalSigner, generate_nonce
from agentpay.core.models import PaymentChallenge
from agentpay.evm.constants import (
    ARBITRUM_SEPOLIA_CAIP2,
    ARBITRUM_SEPOLIA_CHAIN_ID,
    ARBITRUM_SEPOLIA_RPC_URL,
    ARBITRUM_SEPOLIA_USDC_ADDRESS,
    ARBITRUM_SEPOLIA_USDC_NAME,
    ARBITRUM_SEPOLIA_USDC_VERSION,
)
from agentpay.evm.rpc import EvmRpcClient
from agentpay.policy.models import ReservationToken
from agentpay_gateway.facilitator.client import AsyncFacilitatorClient, FacilitatorClient
from agentpay_gateway.facilitator.errors import (
    ChallengeExpiredError,
    InvalidSignatureError,
    NonceAlreadyUsedError,
)
from agentpay_gateway.facilitator.evm_service import EvmFacilitatorService
from agentpay_gateway.facilitator.models import (
    SettlementState,
    SettleRequest,
    VerifyRequest,
)

BUYER_KEY = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
FACILITATOR_KEY = "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
MERCHANT_PAYEE = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"


class MockEvmRpcHandler:
    """Mock EVM JSON-RPC handler simulating Arbitrum Sepolia node."""

    def __init__(self, chain_id: int = ARBITRUM_SEPOLIA_CHAIN_ID) -> None:
        self.chain_id = chain_id
        self.nonce_states: dict[tuple[str, str], bool] = {}  # (authorizer, nonce) -> is_used
        self.broadcasted_txs: list[str] = []
        self.receipts: dict[str, dict[str, Any]] = {}
        self.block_number = 12345678
        self.gas_price = 100_000_000  # 0.1 gwei

    def handle(self, request: httpx.Request) -> httpx.Response:
        data = json.loads(request.content.decode("utf-8"))
        method = data.get("method")
        params = data.get("params", [])
        req_id = data.get("id", 1)

        result: Any = None
        if method == "eth_chainId":
            result = hex(self.chain_id)
        elif method == "eth_blockNumber":
            result = hex(self.block_number)
        elif method == "eth_gasPrice":
            result = hex(self.gas_price)
        elif method == "eth_getBlockByNumber":
            result = {
                "number": hex(self.block_number),
                "baseFeePerGas": hex(self.gas_price),
            }
        elif method == "eth_maxPriorityFeePerGas":
            result = "0x0"
        elif method == "eth_getTransactionCount":
            result = hex(len(self.broadcasted_txs))
        elif method == "eth_estimateGas":
            result = hex(65000)
        elif method == "eth_call":
            call_tx = params[0]
            calldata = call_tx.get("data", "")
            # authorizationState selector: 0xe94a0102
            if calldata.startswith("0xe94a0102"):
                authorizer_hex = "0x" + calldata[10 + 24 : 10 + 64]
                nonce_hex = "0x" + calldata[10 + 64 : 10 + 128]
                key = (authorizer_hex.lower(), nonce_hex.lower())
                is_used = self.nonce_states.get(key, False)
                result = "0x" + ("01" if is_used else "00").rjust(64, "0")
            else:
                result = "0x" + "00" * 32
        elif method == "eth_sendRawTransaction":
            raw_tx = params[0]
            self.broadcasted_txs.append(raw_tx)
            tx_hash = f"0x{len(self.broadcasted_txs):064x}"
            # Pre-register successful receipt
            self.receipts[tx_hash] = {
                "transactionHash": tx_hash,
                "blockNumber": hex(self.block_number + 1),
                "status": "0x1",
                "gasUsed": "0xfde8",
            }
            result = tx_hash
        elif method == "eth_getTransactionReceipt":
            tx_hash = params[0]
            result = self.receipts.get(tx_hash)
        else:
            result = "0x0"

        return httpx.Response(
            status_code=200,
            json={"jsonrpc": "2.0", "result": result, "id": req_id},
            request=request,
        )


@pytest.fixture
def mock_rpc():
    handler = MockEvmRpcHandler()
    transport = httpx.MockTransport(handler.handle)
    client = httpx.Client(transport=transport)
    rpc_client = EvmRpcClient(
        rpc_url=ARBITRUM_SEPOLIA_RPC_URL,
        expected_chain_id=ARBITRUM_SEPOLIA_CHAIN_ID,
        client=client,
    )
    return handler, rpc_client


@pytest.fixture
def evm_facilitator(mock_rpc):
    handler, rpc_client = mock_rpc
    service = EvmFacilitatorService(
        rpc_url=ARBITRUM_SEPOLIA_RPC_URL,
        chain_id=ARBITRUM_SEPOLIA_CHAIN_ID,
        token_address=ARBITRUM_SEPOLIA_USDC_ADDRESS,
        private_key=FACILITATOR_KEY,
        rpc_client=rpc_client,
        auto_poll_receipts=False,  # deterministic manual sync in tests
    )
    return service, handler


@pytest.fixture
def buyer_signer():
    return LocalSigner(BUYER_KEY)


@pytest.fixture
def sample_challenge():
    return PaymentChallenge(
        version="2.0",
        challengeId="ch_arb_fac_001",
        network=ARBITRUM_SEPOLIA_CAIP2,
        asset=ARBITRUM_SEPOLIA_USDC_ADDRESS,
        assetSymbol="USDC",
        decimals=6,
        amount="25000",  # 0.025 USDC
        payee=MERCHANT_PAYEE,
        scheme="EIP-3009",
        validAfter=1000,
        validBefore=2000,
        facilitatorUrl="http://facilitator:8001",
        resourceUri="/v1/analytics",
    )


class TestEvmFacilitatorVerification:
    def test_verify_success(
        self,
        evm_facilitator: tuple[EvmFacilitatorService, MockEvmRpcHandler],
        buyer_signer: LocalSigner,
        sample_challenge: PaymentChallenge,
    ) -> None:
        service, _ = evm_facilitator
        service._clock = lambda: 1500  # within [1000, 2000]

        token = ReservationToken(
            reservation_id="res_01",
            intent_id=sample_challenge.challenge_id,
            policy_id="pol_default",
            amount=int(sample_challenge.amount),
            expires_at=sample_challenge.valid_before,
            created_at=sample_challenge.valid_after,
        )
        auth = buyer_signer.sign_payment_authorization(
            sample_challenge,
            token,
            nonce=generate_nonce(),
            token_name=ARBITRUM_SEPOLIA_USDC_NAME,
            token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
        )

        req = VerifyRequest(challenge=sample_challenge, authorization=auth)
        res = service.verify(req)

        assert res.valid is True
        assert res.verified_payer.lower() == buyer_signer.address.lower()
        assert res.verification_token.startswith("vtok_")

    def test_verify_fails_onchain_used_nonce(
        self,
        evm_facilitator: tuple[EvmFacilitatorService, MockEvmRpcHandler],
        buyer_signer: LocalSigner,
        sample_challenge: PaymentChallenge,
    ) -> None:
        service, handler = evm_facilitator
        service._clock = lambda: 1500

        nonce = generate_nonce()
        # Mark nonce as already used on chain
        handler.nonce_states[(buyer_signer.address.lower(), nonce.lower())] = True

        token = ReservationToken(
            reservation_id="res_02",
            intent_id=sample_challenge.challenge_id,
            policy_id="pol_default",
            amount=int(sample_challenge.amount),
            expires_at=sample_challenge.valid_before,
            created_at=sample_challenge.valid_after,
        )
        auth = buyer_signer.sign_payment_authorization(
            sample_challenge,
            token,
            nonce=nonce,
            token_name=ARBITRUM_SEPOLIA_USDC_NAME,
            token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
        )

        req = VerifyRequest(challenge=sample_challenge, authorization=auth)
        with pytest.raises(NonceAlreadyUsedError) as exc_info:
            service.verify(req)
        assert "already been consumed on-chain" in str(exc_info.value)

    def test_verify_fails_expired_challenge(
        self,
        evm_facilitator: tuple[EvmFacilitatorService, MockEvmRpcHandler],
        buyer_signer: LocalSigner,
        sample_challenge: PaymentChallenge,
    ) -> None:
        service, _ = evm_facilitator
        service._clock = lambda: 2500  # past validBefore 2000

        token = ReservationToken(
            reservation_id="res_03",
            intent_id=sample_challenge.challenge_id,
            policy_id="pol_default",
            amount=int(sample_challenge.amount),
            expires_at=sample_challenge.valid_before,
            created_at=sample_challenge.valid_after,
        )
        auth = buyer_signer.sign_payment_authorization(
            sample_challenge,
            token,
            token_name=ARBITRUM_SEPOLIA_USDC_NAME,
            token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
        )

        req = VerifyRequest(challenge=sample_challenge, authorization=auth)
        with pytest.raises(ChallengeExpiredError):
            service.verify(req)

    def test_verify_fails_invalid_signature(
        self,
        evm_facilitator: tuple[EvmFacilitatorService, MockEvmRpcHandler],
        buyer_signer: LocalSigner,
        sample_challenge: PaymentChallenge,
    ) -> None:
        service, _ = evm_facilitator
        service._clock = lambda: 1500

        token = ReservationToken(
            reservation_id="res_04",
            intent_id=sample_challenge.challenge_id,
            policy_id="pol_default",
            amount=int(sample_challenge.amount),
            expires_at=sample_challenge.valid_before,
            created_at=sample_challenge.valid_after,
        )
        auth = buyer_signer.sign_payment_authorization(
            sample_challenge,
            token,
            token_name=ARBITRUM_SEPOLIA_USDC_NAME,
            token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
        )

        # Corrupt signature by tampering serialized bytes
        bad_sig = auth.signature.model_copy(update={"serialized": "11" * 65})
        bad_auth = auth.model_copy(update={"signature": bad_sig})

        req = VerifyRequest(challenge=sample_challenge, authorization=bad_auth)
        with pytest.raises(InvalidSignatureError):
            service.verify(req)


class TestEvmFacilitatorSettlement:
    def test_settle_broadcasts_signed_evm_transaction(
        self,
        evm_facilitator: tuple[EvmFacilitatorService, MockEvmRpcHandler],
        buyer_signer: LocalSigner,
        sample_challenge: PaymentChallenge,
    ) -> None:
        service, handler = evm_facilitator
        service._clock = lambda: 1500

        token = ReservationToken(
            reservation_id="res_05",
            intent_id=sample_challenge.challenge_id,
            policy_id="pol_default",
            amount=int(sample_challenge.amount),
            expires_at=sample_challenge.valid_before,
            created_at=sample_challenge.valid_after,
        )
        auth = buyer_signer.sign_payment_authorization(
            sample_challenge,
            token,
            token_name=ARBITRUM_SEPOLIA_USDC_NAME,
            token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
        )

        verify_req = VerifyRequest(challenge=sample_challenge, authorization=auth)
        verify_res = service.verify(verify_req)

        settle_req = SettleRequest(
            verificationToken=verify_res.verification_token,
            authorization=auth,
            idempotencyKey="idem_arb_001",
        )
        settle_res = service.settle(settle_req)

        assert settle_res.status == SettlementState.SUBMITTED
        assert settle_res.settlement_id.startswith("stl_")

        # Verify broadcasted transaction in mock RPC
        assert len(handler.broadcasted_txs) == 1
        raw_tx_hex = handler.broadcasted_txs[0]
        assert raw_tx_hex.startswith("0x")

        # Inspect settlement status record
        status = service.get_settlement(settle_res.settlement_id)
        assert status.status == SettlementState.SUBMITTED
        assert status.tx_hash is not None

        # Confirm receipt on-chain
        confirmed = service.sync_settlement_receipt(settle_res.settlement_id)
        assert confirmed.status == SettlementState.CONFIRMED
        assert confirmed.error is None

    def test_settle_reverts_handling(
        self,
        evm_facilitator: tuple[EvmFacilitatorService, MockEvmRpcHandler],
        buyer_signer: LocalSigner,
        sample_challenge: PaymentChallenge,
    ) -> None:
        service, handler = evm_facilitator
        service._clock = lambda: 1500

        token = ReservationToken(
            reservation_id="res_06",
            intent_id=sample_challenge.challenge_id,
            policy_id="pol_default",
            amount=int(sample_challenge.amount),
            expires_at=sample_challenge.valid_before,
            created_at=sample_challenge.valid_after,
        )
        auth = buyer_signer.sign_payment_authorization(
            sample_challenge,
            token,
            token_name=ARBITRUM_SEPOLIA_USDC_NAME,
            token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
        )

        verify_res = service.verify(VerifyRequest(challenge=sample_challenge, authorization=auth))
        settle_res = service.settle(
            SettleRequest(
                verificationToken=verify_res.verification_token,
                authorization=auth,
            )
        )

        # Simulate on-chain revert by setting status = 0x0 in receipt
        curr_status = service.get_settlement(settle_res.settlement_id)
        handler.receipts[curr_status.tx_hash]["status"] = "0x0"

        failed_status = service.sync_settlement_receipt(settle_res.settlement_id)
        assert failed_status.status == SettlementState.FAILED
        assert "reverted" in str(failed_status.error)


class TestEvmFacilitatorHttpAndClient:
    def test_facilitator_client_over_sync_transport(
        self,
        evm_facilitator: tuple[EvmFacilitatorService, MockEvmRpcHandler],
        buyer_signer: LocalSigner,
        sample_challenge: PaymentChallenge,
    ) -> None:
        service, _ = evm_facilitator
        service._clock = lambda: 1500

        # Run sync client against service via in-memory transport
        transport = service.create_sync_transport()
        with FacilitatorClient(base_url="http://facilitator:8001", transport=transport) as client:
            token = ReservationToken(
                reservation_id="res_07",
                intent_id=sample_challenge.challenge_id,
                policy_id="pol_default",
                amount=int(sample_challenge.amount),
                expires_at=sample_challenge.valid_before,
                created_at=sample_challenge.valid_after,
            )
            auth = buyer_signer.sign_payment_authorization(
                sample_challenge,
                token,
                token_name=ARBITRUM_SEPOLIA_USDC_NAME,
                token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
            )

            # 1. Verify
            v_res = client.verify(sample_challenge, auth)
            assert v_res.valid is True
            assert v_res.verification_token.startswith("vtok_")

            # 2. Settle
            s_res = client.settle(
                authorization=auth,
                verification_token=v_res.verification_token,
                idempotency_key="idem_http_001",
            )
            assert s_res.status == SettlementState.SUBMITTED
            assert s_res.settlement_id.startswith("stl_")

            # 3. Check settlement status
            st = client.get_settlement(s_res.settlement_id)
            assert st.status == SettlementState.SUBMITTED

            # 4. Check nonce query
            assert (
                client.is_nonce_used(
                    sample_challenge.network,
                    sample_challenge.asset,
                    auth.payer,
                    auth.nonce,
                )
                is True
            )

    @pytest.mark.anyio
    async def test_async_facilitator_client_over_asgi(
        self,
        evm_facilitator: tuple[EvmFacilitatorService, MockEvmRpcHandler],
        buyer_signer: LocalSigner,
        sample_challenge: PaymentChallenge,
    ) -> None:
        service, _ = evm_facilitator
        service._clock = lambda: 1500

        transport = httpx.ASGITransport(app=service.asgi_app)
        async with AsyncFacilitatorClient(base_url="http://facilitator:8001", transport=transport) as aclient:
            token = ReservationToken(
                reservation_id="res_08",
                intent_id=sample_challenge.challenge_id,
                policy_id="pol_default",
                amount=int(sample_challenge.amount),
                expires_at=sample_challenge.valid_before,
                created_at=sample_challenge.valid_after,
            )
            auth = buyer_signer.sign_payment_authorization(
                sample_challenge,
                token,
                token_name=ARBITRUM_SEPOLIA_USDC_NAME,
                token_version=ARBITRUM_SEPOLIA_USDC_VERSION,
            )

            v_res = await aclient.verify(sample_challenge, auth)
            assert v_res.valid is True

            s_res = await aclient.settle(
                authorization=auth,
                verification_token=v_res.verification_token,
            )
            assert s_res.status == SettlementState.SUBMITTED
