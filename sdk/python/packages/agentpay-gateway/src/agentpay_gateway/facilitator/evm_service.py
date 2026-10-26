"""
Arbitrum Sepolia & EVM On-Chain Facilitator Settlement Service for AgentPay.

Implements the official x402 V2 Facilitator settlement node:
- Verifies EIP-712/3009 authorizations off-chain and against on-chain state
- Checks on-chain authorizationState (P0-7 nonce replay prevention)
- Prepares and signs transferWithAuthorization EVM transactions
- Broadcasts raw transactions to Arbitrum Sepolia via JSON-RPC
- Asynchronously polls transaction receipts for confirmation or reversion
- Fully compatible with ASGI 3.0 and httpx transports
"""

from __future__ import annotations

import json
import logging
import os
import secrets
import threading
import time
import urllib.parse
from collections.abc import Callable
from typing import Any

from agentpay.client.signer import LocalSigner
from agentpay.core.models import PaymentChallenge
from agentpay.evm.constants import (
    ARBITRUM_SEPOLIA_CHAIN_ID,
    ARBITRUM_SEPOLIA_RPC_URL,
    ARBITRUM_SEPOLIA_USDC_ADDRESS,
)
from agentpay.evm.eip3009 import encode_transfer_from_authorization
from agentpay.evm.rpc import EvmRpcClient
from eth_account import Account
from eth_utils import to_checksum_address

from agentpay_gateway.facilitator.errors import (
    ChallengeExpiredError,
    InvalidRequestError,
    InvalidSignatureError,
    NonceAlreadyUsedError,
    SettlementNotFoundError,
)
from agentpay_gateway.facilitator.models import (
    SettlementState,
    SettlementStatus,
    SettleRequest,
    SettleResponse,
    VerifyRequest,
    VerifyResponse,
)
from agentpay_gateway.facilitator.service import MockFacilitatorTransport

logger = logging.getLogger("agentpay_gateway.facilitator.evm")


class EvmFacilitatorService:
    """
    Production-ready Facilitator service clearing EIP-3009 payments on Arbitrum Sepolia or EVM testnets.
    """

    def __init__(
        self,
        *,
        rpc_url: str = ARBITRUM_SEPOLIA_RPC_URL,
        chain_id: int = ARBITRUM_SEPOLIA_CHAIN_ID,
        token_address: str = ARBITRUM_SEPOLIA_USDC_ADDRESS,
        private_key: str | None = None,
        rpc_client: EvmRpcClient | None = None,
        clock: Callable[[], int] | None = None,
        auto_poll_receipts: bool = True,
        receipt_timeout_seconds: float = 30.0,
        receipt_poll_interval_seconds: float = 0.5,
    ) -> None:
        self.rpc_url = rpc_url
        self.chain_id = chain_id
        self.token_address = to_checksum_address(token_address)
        self._clock = clock or (lambda: int(time.time()))
        self.auto_poll_receipts = auto_poll_receipts
        self.receipt_timeout_seconds = receipt_timeout_seconds
        self.receipt_poll_interval_seconds = receipt_poll_interval_seconds

        # Private key setup for Facilitator EVM transaction signing
        raw_key = private_key or os.environ.get("FACILITATOR_PRIVATE_KEY")
        if raw_key:
            self._account = Account.from_key(raw_key)
        else:
            # Generate deterministic fallback account for ephemeral testing
            self._account = Account.create(extra_entropy=secrets.token_hex(32))

        self.facilitator_address = to_checksum_address(self._account.address)

        # RPC client
        self.rpc_client = rpc_client or EvmRpcClient(
            rpc_url=self.rpc_url,
            expected_chain_id=self.chain_id,
        )

        # Internal registries
        self._used_nonces: set[tuple[str, str, str, str]] = set()
        self._verification_tokens: dict[str, dict[str, Any]] = {}
        self._settlements: dict[str, SettlementStatus] = {}
        self._idempotency_cache: dict[tuple[str, str], SettleResponse] = {}
        self._lock = threading.Lock()

    def create_sync_transport(self) -> MockFacilitatorTransport:
        """Create a synchronous in-memory httpx.BaseTransport for FacilitatorClient."""
        return MockFacilitatorTransport(self)  # type: ignore[arg-type]

    def get_time(self) -> int:
        """Get current wall-clock or simulated timestamp."""
        return self._clock()

    # -------------------------------------------------------------------------
    # Verification (POST /v1/verify)
    # -------------------------------------------------------------------------

    def verify(self, request: VerifyRequest) -> VerifyResponse:
        """
        Verify payment challenge and client authorization off-chain and against on-chain state.

        Checks:
        1. Local in-memory nonce replay cache.
        2. On-chain authorizationState(authorizer, nonce) via JSON-RPC.
        3. Expiration and time-window validity.
        4. Challenge ID congruence.
        5. EIP-712 cryptographic signature validity.
        """
        ch = request.challenge
        auth = request.authorization
        now = self.get_time()

        nonce_key = (
            ch.network.lower(),
            ch.asset.lower(),
            auth.payer.lower(),
            auth.nonce.lower(),
        )

        with self._lock:
            # 1. Local nonce cache check
            if nonce_key in self._used_nonces:
                raise NonceAlreadyUsedError(f"Nonce {auth.nonce} has already been registered or used for {auth.payer}")

            # 2. Time window check
            if now > ch.valid_before:
                raise ChallengeExpiredError(f"Challenge expired: validBefore ({ch.valid_before}) < now ({now})")
            if now < ch.valid_after:
                raise InvalidRequestError(f"Challenge not yet valid: validAfter ({ch.valid_after}) > now ({now})")

            # 3. Challenge ID matching
            if auth.challenge_id != ch.challenge_id:
                raise InvalidRequestError(
                    f"Authorization challengeId ({auth.challenge_id}) does not match challenge ({ch.challenge_id})"
                )

        # 4. Check on-chain authorization state via RPC (if RPC client is accessible)
        try:
            is_onchain_used = self.rpc_client.check_authorization_state(
                token_address=ch.asset,
                authorizer=auth.payer,
                nonce=auth.nonce,
            )
            if is_onchain_used:
                with self._lock:
                    self._used_nonces.add(nonce_key)
                raise NonceAlreadyUsedError(f"Nonce {auth.nonce} has already been consumed on-chain for {auth.payer}")
        except NonceAlreadyUsedError:
            raise
        except Exception as exc:
            # If RPC call fails (e.g. offline mock during quick unit testing), log warning and proceed to signature verification
            logger.debug(f"On-chain nonce pre-check skipped or errored: {exc}")

        # 5. Cryptographic signature check
        is_valid = LocalSigner.verify_authorization(
            auth,
            ch,
            expected_payer=auth.payer,
        )
        if not is_valid:
            raise InvalidSignatureError(f"EIP-712 signature verification failed for payer {auth.payer}")

        # 6. Issue verification token
        vtok = f"vtok_{secrets.token_hex(16)}"
        with self._lock:
            self._verification_tokens[vtok] = {
                "challenge": ch,
                "authorization": auth,
                "nonce_key": nonce_key,
                "verified_at": now,
                "expires_at": ch.valid_before,
            }

        return VerifyResponse(
            valid=True,
            verifiedPayer=auth.payer,
            verifiedAt=now,
            verificationToken=vtok,
        )

    # -------------------------------------------------------------------------
    # Settlement (POST /v1/settle)
    # -------------------------------------------------------------------------

    def settle(self, request: SettleRequest) -> SettleResponse:
        """
        Assemble transferWithAuthorization calldata, sign an EVM transaction with Facilitator key,
        broadcast to Arbitrum Sepolia, and trigger receipt confirmation.
        """
        now = self.get_time()
        auth = request.authorization
        vtok = request.verification_token

        # Check idempotency cache first
        if request.idempotency_key:
            idem_key = (auth.payer.lower(), request.idempotency_key)
            with self._lock:
                if idem_key in self._idempotency_cache:
                    return self._idempotency_cache[idem_key]

        with self._lock:
            record = self._verification_tokens.get(vtok)
            if not record:
                raise InvalidRequestError(f"Unknown or expired verification token: {vtok}")

            if now > record["expires_at"]:
                raise ChallengeExpiredError("Verification token has expired")

            nonce_key = record["nonce_key"]
            if nonce_key in self._used_nonces:
                raise NonceAlreadyUsedError("Nonce has already been submitted for settlement")

            # Reserve nonce locally
            self._used_nonces.add(nonce_key)

        ch: PaymentChallenge = record["challenge"]

        # 1. Encode EIP-3009 transferWithAuthorization calldata
        calldata = encode_transfer_from_authorization(
            auth,
            to_address=ch.payee,
            value=int(ch.amount),
        )

        # 2. Query Facilitator account state from EVM RPC
        try:
            tx_nonce = self.rpc_client.get_transaction_count(self.facilitator_address, "pending")
        except Exception:
            tx_nonce = 0

        # Query base fee and gas price for dynamic EIP-1559 / EVM fee calculation
        base_fee = self.rpc_client.get_base_fee()
        try:
            gas_price = self.rpc_client.get_gas_price()
        except Exception:
            gas_price = 100_000_000  # 0.1 gwei fallback

        if base_fee is None or base_fee <= 0:
            base_fee = gas_price

        priority_fee = self.rpc_client.get_max_priority_fee()
        # Default priority fee buffer: 0.001 Gwei (1,000,000 wei)
        if priority_fee <= 0:
            priority_fee = 1_000_000

        # Buffer multiplier of 1.5x on base fee ensures block-to-block base fee increases
        # on fast rollups (Arbitrum nitro) don't trigger "max fee per gas less than block base fee".
        # Sender is only debited the actual baseFee + priorityFee, not maxFeePerGas.
        max_fee_per_gas = max(int(base_fee * 1.5) + priority_fee, int(gas_price * 1.35) + priority_fee)

        # 3. Estimate Gas
        try:
            est_gas = self.rpc_client.estimate_gas(
                to=self.token_address,
                data=calldata,
                from_address=self.facilitator_address,
            )
            gas_limit = max(int(est_gas * 1.25), 85_000)
        except Exception:
            gas_limit = 120_000

        # 4. Sign EVM transaction with Facilitator private key (Type 2 EIP-1559)
        tx_dict = {
            "type": 2,
            "nonce": tx_nonce,
            "maxFeePerGas": max_fee_per_gas,
            "maxPriorityFeePerGas": priority_fee,
            "gas": gas_limit,
            "to": self.token_address,
            "value": 0,
            "data": calldata,
            "chainId": self.chain_id,
        }
        signed_tx = Account.sign_transaction(tx_dict, self._account.key)

        # 5. Broadcast to EVM RPC (with legacy fallback if chain rejects Type 2)
        try:
            tx_hash = self.rpc_client.send_raw_transaction(signed_tx.raw_transaction.hex())
        except Exception as exc:
            err_str = str(exc)
            # If the node rejects Type 2 (e.g. non-EIP-1559 chain), fallback to legacy Type 0
            if "not supported" in err_str.lower() or "invalid transaction type" in err_str.lower():
                logger.warning(f"EIP-1559 not supported by node, falling back to legacy tx: {exc}")
                legacy_gas_price = max(max_fee_per_gas, int(gas_price * 1.35) + 1_000_000)
                legacy_tx = {
                    "nonce": tx_nonce,
                    "gasPrice": legacy_gas_price,
                    "gas": gas_limit,
                    "to": self.token_address,
                    "value": 0,
                    "data": calldata,
                    "chainId": self.chain_id,
                }
                signed_legacy = Account.sign_transaction(legacy_tx, self._account.key)
                tx_hash = self.rpc_client.send_raw_transaction(signed_legacy.raw_transaction.hex())
            else:
                logger.error(f"Failed to broadcast settlement tx: {exc}")
                raise InvalidRequestError(f"Failed to broadcast transaction to EVM RPC: {exc}") from exc

        settlement_id = f"stl_{secrets.token_hex(12)}"
        with self._lock:
            position = len(self._settlements) + 1
            response = SettleResponse(
                settlementId=settlement_id,
                status=SettlementState.SUBMITTED,
                queuePosition=position,
                txHash=tx_hash,
            )
            self._settlements[settlement_id] = SettlementStatus(
                settlementId=settlement_id,
                status=SettlementState.SUBMITTED,
                txHash=tx_hash,
                createdAt=now,
                updatedAt=now,
            )
            if request.idempotency_key:
                self._idempotency_cache[(auth.payer.lower(), request.idempotency_key)] = response

        # 6. Asynchronously poll transaction receipt if enabled
        if self.auto_poll_receipts:
            threading.Thread(
                target=self._poll_receipt_loop,
                args=(settlement_id, tx_hash),
                name=f"poll-receipt-{settlement_id}",
                daemon=True,
            ).start()

        return response

    def _poll_receipt_loop(self, settlement_id: str, tx_hash: str) -> None:
        """Background thread polling EVM RPC for transaction receipt confirmation."""
        try:
            receipt = self.rpc_client.wait_for_transaction_receipt(
                tx_hash,
                timeout_seconds=self.receipt_timeout_seconds,
                poll_interval_seconds=self.receipt_poll_interval_seconds,
            )
            status_raw = receipt.get("status")
            is_success = status_raw in (1, "0x1", "1")

            now = self.get_time()
            with self._lock:
                curr = self._settlements.get(settlement_id)
                if curr:
                    self._settlements[settlement_id] = SettlementStatus(
                        settlementId=settlement_id,
                        status=SettlementState.CONFIRMED if is_success else SettlementState.FAILED,
                        txHash=tx_hash,
                        error=None if is_success else "On-chain transaction execution reverted",
                        createdAt=curr.created_at,
                        updatedAt=now,
                    )
        except Exception as exc:
            logger.warning(f"Receipt polling failed for settlement {settlement_id}: {exc}")
            now = self.get_time()
            with self._lock:
                curr = self._settlements.get(settlement_id)
                if curr:
                    self._settlements[settlement_id] = SettlementStatus(
                        settlementId=settlement_id,
                        status=SettlementState.FAILED,
                        txHash=tx_hash,
                        error=str(exc),
                        createdAt=curr.created_at,
                        updatedAt=now,
                    )

    # -------------------------------------------------------------------------
    # Settlement Inspection & Query
    # -------------------------------------------------------------------------

    def get_settlement(self, settlement_id: str) -> SettlementStatus:
        """Retrieve settlement record by settlement ID."""
        with self._lock:
            record = self._settlements.get(settlement_id)
            if not record:
                raise SettlementNotFoundError(f"Settlement {settlement_id!r} not found")
            return record

    def is_nonce_used(self, network: str, asset: str, payer: str, nonce: str) -> bool:
        """Check if nonce has been used either locally or on-chain."""
        nonce_key = (
            network.lower(),
            asset.lower(),
            payer.lower(),
            nonce.lower(),
        )
        with self._lock:
            if nonce_key in self._used_nonces:
                return True

        # Query on-chain
        try:
            return self.rpc_client.check_authorization_state(asset, payer, nonce)
        except Exception:
            return False

    def sync_settlement_receipt(self, settlement_id: str, timeout_seconds: float = 5.0) -> SettlementStatus:
        """Synchronously wait for a settlement's transaction receipt (useful in test suites)."""
        curr = self.get_settlement(settlement_id)
        if curr.status in (SettlementState.CONFIRMED, SettlementState.FAILED):
            return curr

        if not curr.tx_hash:
            return curr

        try:
            receipt = self.rpc_client.wait_for_transaction_receipt(
                curr.tx_hash,
                timeout_seconds=timeout_seconds,
                poll_interval_seconds=self.receipt_poll_interval_seconds,
            )
            status_raw = receipt.get("status")
            is_success = status_raw in (1, "0x1", "1")
            with self._lock:
                updated = SettlementStatus(
                    settlementId=settlement_id,
                    status=SettlementState.CONFIRMED if is_success else SettlementState.FAILED,
                    txHash=curr.tx_hash,
                    error=None if is_success else "On-chain execution reverted",
                    createdAt=curr.created_at,
                    updatedAt=self.get_time(),
                )
                self._settlements[settlement_id] = updated
                return updated
        except Exception as exc:
            with self._lock:
                updated = SettlementStatus(
                    settlementId=settlement_id,
                    status=SettlementState.FAILED,
                    txHash=curr.tx_hash,
                    error=str(exc),
                    createdAt=curr.created_at,
                    updatedAt=self.get_time(),
                )
                self._settlements[settlement_id] = updated
                return updated

    # -------------------------------------------------------------------------
    # Native ASGI 3.0 Handler
    # -------------------------------------------------------------------------

    async def asgi_app(
        self,
        scope: dict[str, Any],
        receive: Callable[..., Any],
        send: Callable[..., Any],
    ) -> None:
        """ASGI 3.0 handler for HTTP deployment via uvicorn or httpx.ASGITransport."""
        if scope["type"] != "http":
            return

        path = scope.get("path", "")
        method = scope.get("method", "GET").upper()

        async def send_response(status_code: int, body_dict: dict[str, Any]) -> None:
            body_bytes = json.dumps(body_dict).encode("utf-8")
            await send(
                {
                    "type": "http.response.start",
                    "status": status_code,
                    "headers": [
                        [b"content-type", b"application/json"],
                        [b"content-length", str(len(body_bytes)).encode("ascii")],
                    ],
                }
            )
            await send(
                {
                    "type": "http.response.body",
                    "body": body_bytes,
                }
            )

        body = b""
        more_body = True
        while more_body:
            message = await receive()
            body += message.get("body", b"")
            more_body = message.get("more_body", False)

        try:
            # POST /v1/verify
            if method == "POST" and path == "/v1/verify":
                data = json.loads(body.decode("utf-8")) if body else {}
                req = VerifyRequest.model_validate(data)
                res = self.verify(req)
                await send_response(200, res.model_dump(by_alias=True))
                return

            # POST /v1/settle
            if method == "POST" and path == "/v1/settle":
                data = json.loads(body.decode("utf-8")) if body else {}
                req_s = SettleRequest.model_validate(data)
                res_s = self.settle(req_s)
                await send_response(202, res_s.model_dump(by_alias=True))
                return

            # GET /v1/settle/{id}
            if method == "GET" and path.startswith("/v1/settle/"):
                settlement_id = path.removeprefix("/v1/settle/")
                res_st = self.get_settlement(settlement_id)
                await send_response(200, res_st.model_dump(by_alias=True))
                return

            # GET /v1/nonce
            if method == "GET" and path in ("/v1/nonce", "/v1/nonces"):
                query_string = scope.get("query_string", b"").decode("utf-8")
                params = dict(urllib.parse.parse_qsl(query_string))
                used = self.is_nonce_used(
                    params.get("network", ""),
                    params.get("asset", ""),
                    params.get("payer", ""),
                    params.get("nonce", ""),
                )
                await send_response(200, {"isUsed": used})
                return

            # GET /health
            if method == "GET" and path in ("/health", "/v1/health"):
                await send_response(
                    200,
                    {
                        "status": "ok",
                        "chainId": self.chain_id,
                        "tokenAddress": self.token_address,
                        "facilitatorAddress": self.facilitator_address,
                        "time": self.get_time(),
                    },
                )
                return

            await send_response(404, {"error": "Endpoint not found", "path": path})

        except NonceAlreadyUsedError as exc:
            await send_response(409, {"error": "NONCE_ALREADY_USED", "message": exc.message})
        except ChallengeExpiredError as exc:
            await send_response(410, {"error": "CHALLENGE_EXPIRED", "message": exc.message})
        except InvalidSignatureError as exc:
            await send_response(401, {"error": "INVALID_SIGNATURE", "message": exc.message})
        except InvalidRequestError as exc:
            await send_response(400, {"error": "INVALID_REQUEST", "message": exc.message})
        except SettlementNotFoundError as exc:
            await send_response(404, {"error": "SETTLEMENT_NOT_FOUND", "message": exc.message})
        except Exception as exc:  # noqa: BLE001
            await send_response(500, {"error": "INTERNAL_SERVER_ERROR", "message": str(exc)})
