"""
Gateway State Coordinator for AgentPay.

Coordinates SQLite atomic transactions between:
  - Gateway Settlement (ap_gateway_settlement)
  - Upstream Fulfillment (ap_gateway_fulfillment)
  - Idempotency Cache (ap_idempotency_cache)

Enforces:
  - Non-Rollback Invariant: Upstream 5xx never voids valid settlement
  - P0-7 Replay Defense: Unique nonce constraint
  - P1-5 Idempotency lease locking & free upstream replays
"""

from __future__ import annotations

import sqlite3
import time
import uuid
from collections.abc import Callable

from agentpay.core.state import (
    FulfillmentState,
    GatewayFulfillmentRecord,
    GatewaySettlementRecord,
    SettlementState,
)
from agentpay.state.guards import (
    validate_fulfillment_transition,
    validate_settlement_transition,
)
from pydantic import BaseModel, ConfigDict


class IdempotencyEvaluation(BaseModel):
    """Result of checking an inbound (payer, idempotency_key) pair."""

    model_config = ConfigDict(frozen=True)

    is_fresh: bool
    is_in_flight: bool = False
    is_cache_hit: bool = False
    is_free_replay: bool = False
    settlement_id: str | None = None
    fulfillment_id: str | None = None
    cached_status_code: int | None = None
    cached_headers_json: str | None = None
    cached_body_blob: bytes | None = None


class GatewayStateCoordinator:
    """
    Coordinates state mutations and SQLite transactions for the Gateway domain.
    """

    def __init__(
        self,
        conn: sqlite3.Connection,
        *,
        clock: Callable[[], int] | None = None,
        lease_duration_seconds: int = 60,
        cache_ttl_seconds: int = 86400,
    ) -> None:
        self._conn = conn
        self._clock = clock or (lambda: int(time.time()))
        self.lease_duration_seconds = lease_duration_seconds
        self.cache_ttl_seconds = cache_ttl_seconds

    def get_time(self) -> int:
        return self._clock()

    def check_idempotency(
        self,
        payer_address: str,
        idempotency_key: str,
    ) -> IdempotencyEvaluation:
        """
        Evaluate an incoming idempotency token to determine the required execution path.
        """
        now = self.get_time()
        cursor = self._conn.cursor()
        cursor.execute(
            """
            SELECT c.status, c.settlement_id, c.locked_until, c.response_status_code,
                   c.response_headers_json, c.response_body_blob,
                   s.status, f.status, f.fulfillment_id
            FROM ap_idempotency_cache c
            LEFT JOIN ap_gateway_settlement s ON c.settlement_id = s.settlement_id
            LEFT JOIN ap_gateway_fulfillment f ON s.settlement_id = f.settlement_id
            WHERE c.payer_address = ? AND c.idempotency_key = ?;
            """,
            (payer_address.lower(), idempotency_key),
        )
        row = cursor.fetchone()
        if not row:
            return IdempotencyEvaluation(is_fresh=True)

        cache_status = row[0]
        settlement_id = row[1]
        locked_until = row[2]
        resp_status = row[3]
        resp_headers = row[4]
        resp_blob = row[5]
        settlement_status = SettlementState(row[6]) if row[6] else None
        fulfillment_status = FulfillmentState(row[7]) if row[7] else None
        fulfillment_id = row[8]

        # Case 1: In-flight concurrent request with active lease
        if cache_status == "IN_FLIGHT" and now < locked_until:
            return IdempotencyEvaluation(
                is_fresh=False,
                is_in_flight=True,
                settlement_id=settlement_id,
            )

        # Case 2: Healthy complete delivery -> CACHE HIT!
        if fulfillment_status == FulfillmentState.DELIVERED and resp_blob is not None:
            return IdempotencyEvaluation(
                is_fresh=False,
                is_cache_hit=True,
                settlement_id=settlement_id,
                fulfillment_id=fulfillment_id,
                cached_status_code=resp_status,
                cached_headers_json=resp_headers,
                cached_body_blob=resp_blob,
            )

        # Case 3: Payment verified/settled, but fulfillment requires recovery -> FREE REPLAY ELIGIBLE!
        if (
            settlement_status
            in (
                SettlementState.SUBMITTED,
                SettlementState.PENDING,
                SettlementState.CONFIRMED,
                SettlementState.SUCCESS,
            )
            and fulfillment_status == FulfillmentState.RECOVERY_REQUIRED
        ):
            return IdempotencyEvaluation(
                is_fresh=False,
                is_free_replay=True,
                settlement_id=settlement_id,
                fulfillment_id=fulfillment_id,
            )

        # Expired lease without completion
        return IdempotencyEvaluation(is_fresh=True)

    def register_inbound_payment(
        self,
        settlement: GatewaySettlementRecord,
        request_hash: str = "hash_default",
    ) -> str:
        """
        Atomically register an inbound settlement record, create initial fulfillment,
        and acquire the idempotency lease.

        Enforces:
          P0-7: UNIQUE(network_id, asset_address, payer_address, nonce)
          P1-5: PRIMARY KEY(payer_address, idempotency_key)

        Returns:
            fulfillment_id (UUIDv7 string)
        """
        now = self.get_time()
        fulfillment_id = f"ful_{uuid.uuid4().hex}"
        locked_until = now + self.lease_duration_seconds
        expires_at = now + self.cache_ttl_seconds

        cursor = self._conn.cursor()
        if not self._conn.in_transaction:
            cursor.execute("BEGIN IMMEDIATE;")
        try:
            # Insert settlement record (enforces P0-7 replay defense)
            cursor.execute(
                """
                INSERT INTO ap_gateway_settlement (
                    settlement_id, idempotency_key, payer_address, payee_address,
                    asset_address, network_id, amount_base_units, nonce,
                    valid_after, valid_before, raw_signature, status,
                    tx_hash, block_number, error_message, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
                """,
                (
                    settlement.settlement_id,
                    settlement.idempotency_key,
                    settlement.payer_address.lower(),
                    settlement.payee_address.lower(),
                    settlement.asset_address.lower(),
                    settlement.network_id,
                    settlement.amount_base_units,
                    settlement.nonce,
                    settlement.valid_after,
                    settlement.valid_before,
                    settlement.raw_signature,
                    settlement.status.value,
                    settlement.tx_hash,
                    settlement.block_number,
                    settlement.error_message,
                    settlement.created_at or now,
                    settlement.updated_at or now,
                ),
            )

            # Insert initial fulfillment record
            cursor.execute(
                """
                INSERT INTO ap_gateway_fulfillment (
                    fulfillment_id, settlement_id, status, upstream_status_code,
                    response_hash, retry_count, delivered_at, created_at, updated_at
                ) VALUES (?, ?, 'NOT_DELIVERED', NULL, NULL, 0, NULL, ?, ?);
                """,
                (fulfillment_id, settlement.settlement_id, now, now),
            )

            # Acquire idempotency lease (enforces P1-5)
            cursor.execute(
                """
                INSERT INTO ap_idempotency_cache (
                    payer_address, idempotency_key, settlement_id, status,
                    request_hash, response_status_code, response_headers_json,
                    response_body_blob, locked_until, expires_at, created_at
                ) VALUES (?, ?, ?, 'IN_FLIGHT', ?, NULL, NULL, NULL, ?, ?, ?);
                """,
                (
                    settlement.payer_address.lower(),
                    settlement.idempotency_key,
                    settlement.settlement_id,
                    request_hash,
                    locked_until,
                    expires_at,
                    now,
                ),
            )
            self._conn.commit()
            return fulfillment_id
        except Exception:
            if self._conn.in_transaction:
                cursor.execute("ROLLBACK;")
            raise

    def transition_settlement(
        self,
        settlement_id: str,
        to_state: SettlementState,
        *,
        tx_hash: str | None = None,
        block_number: int | None = None,
        error_message: str | None = None,
    ) -> None:
        """
        Transition settlement state. Guarded against illegal mutations.
        """
        now = self.get_time()
        cursor = self._conn.cursor()
        if not self._conn.in_transaction:
            cursor.execute("BEGIN IMMEDIATE;")
        try:
            cursor.execute(
                "SELECT status FROM ap_gateway_settlement WHERE settlement_id = ?;",
                (settlement_id,),
            )
            row = cursor.fetchone()
            if not row:
                cursor.execute("ROLLBACK;")
                raise ValueError(f"Settlement '{settlement_id}' not found.")

            current_status = SettlementState(row[0])
            validate_settlement_transition(
                current_status,
                to_state,
                settlement_id=settlement_id,
            )

            cursor.execute(
                """
                UPDATE ap_gateway_settlement
                SET status = ?, tx_hash = COALESCE(?, tx_hash),
                    block_number = COALESCE(?, block_number),
                    error_message = COALESCE(?, error_message),
                    updated_at = ?
                WHERE settlement_id = ?;
                """,
                (
                    to_state.value,
                    tx_hash,
                    block_number,
                    error_message,
                    now,
                    settlement_id,
                ),
            )
            self._conn.commit()
        except Exception:
            if self._conn.in_transaction:
                cursor.execute("ROLLBACK;")
            raise

    def start_fulfillment(self, fulfillment_id: str) -> None:
        """Transition fulfillment state from NOT_DELIVERED to DELIVERING."""
        now = self.get_time()
        cursor = self._conn.cursor()
        if not self._conn.in_transaction:
            cursor.execute("BEGIN IMMEDIATE;")
        try:
            cursor.execute(
                "SELECT status FROM ap_gateway_fulfillment WHERE fulfillment_id = ?;",
                (fulfillment_id,),
            )
            row = cursor.fetchone()
            if not row:
                cursor.execute("ROLLBACK;")
                raise ValueError(f"Fulfillment '{fulfillment_id}' not found.")

            current = FulfillmentState(row[0])
            validate_fulfillment_transition(current, FulfillmentState.DELIVERING)

            cursor.execute(
                """
                UPDATE ap_gateway_fulfillment
                SET status = 'DELIVERING', updated_at = ?
                WHERE fulfillment_id = ?;
                """,
                (now, fulfillment_id),
            )
            self._conn.commit()
        except Exception:
            if self._conn.in_transaction:
                cursor.execute("ROLLBACK;")
            raise

    def complete_fulfillment(
        self,
        fulfillment_id: str,
        upstream_status_code: int,
        response_body_blob: bytes,
        response_headers_json: str | None = None,
        response_hash: str | None = None,
    ) -> None:
        """
        Record successful upstream delivery (2xx) and cache response payload.
        Transitions fulfillment to DELIVERED and idempotency cache to COMPLETED.
        """
        now = self.get_time()
        cursor = self._conn.cursor()
        if not self._conn.in_transaction:
            cursor.execute("BEGIN IMMEDIATE;")
        try:
            cursor.execute(
                """
                SELECT f.status, f.settlement_id, s.payer_address, s.idempotency_key
                FROM ap_gateway_fulfillment f
                JOIN ap_gateway_settlement s ON f.settlement_id = s.settlement_id
                WHERE f.fulfillment_id = ?;
                """,
                (fulfillment_id,),
            )
            row = cursor.fetchone()
            if not row:
                cursor.execute("ROLLBACK;")
                raise ValueError(f"Fulfillment '{fulfillment_id}' not found.")

            current = FulfillmentState(row[0])
            payer = row[2]
            idem_key = row[3]
            validate_fulfillment_transition(current, FulfillmentState.DELIVERED)

            cursor.execute(
                """
                UPDATE ap_gateway_fulfillment
                SET status = 'DELIVERED', upstream_status_code = ?,
                    response_hash = ?, delivered_at = ?, updated_at = ?
                WHERE fulfillment_id = ?;
                """,
                (upstream_status_code, response_hash, now, now, fulfillment_id),
            )

            # Update idempotency cache
            cursor.execute(
                """
                UPDATE ap_idempotency_cache
                SET status = 'COMPLETED', response_status_code = ?,
                    response_headers_json = ?, response_body_blob = ?,
                    locked_until = 0
                WHERE payer_address = ? AND idempotency_key = ?;
                """,
                (
                    upstream_status_code,
                    response_headers_json,
                    response_body_blob,
                    payer,
                    idem_key,
                ),
            )
            self._conn.commit()
        except Exception:
            if self._conn.in_transaction:
                cursor.execute("ROLLBACK;")
            raise

    def fail_fulfillment(
        self,
        fulfillment_id: str,
        upstream_status_code: int = 500,
        error_message: str | None = None,
    ) -> None:
        """
        Record upstream delivery failure (5xx or timeout).
        Transitions fulfillment to RECOVERY_REQUIRED.

        ANTI-ROLLBACK INVARIANT:
        Settlement status remains strictly UNTOUCHED (retains SUBMITTED / SUCCESS).
        """
        now = self.get_time()
        cursor = self._conn.cursor()
        if not self._conn.in_transaction:
            cursor.execute("BEGIN IMMEDIATE;")
        try:
            cursor.execute(
                """
                SELECT f.status, f.settlement_id, s.payer_address, s.idempotency_key
                FROM ap_gateway_fulfillment f
                JOIN ap_gateway_settlement s ON f.settlement_id = s.settlement_id
                WHERE f.fulfillment_id = ?;
                """,
                (fulfillment_id,),
            )
            row = cursor.fetchone()
            if not row:
                cursor.execute("ROLLBACK;")
                raise ValueError(f"Fulfillment '{fulfillment_id}' not found.")

            current = FulfillmentState(row[0])
            payer = row[2]
            idem_key = row[3]
            validate_fulfillment_transition(current, FulfillmentState.RECOVERY_REQUIRED)

            cursor.execute(
                """
                UPDATE ap_gateway_fulfillment
                SET status = 'RECOVERY_REQUIRED', upstream_status_code = ?,
                    retry_count = retry_count + 1, updated_at = ?
                WHERE fulfillment_id = ?;
                """,
                (upstream_status_code, now, fulfillment_id),
            )

            # Release lease lock so client can immediately replay
            cursor.execute(
                """
                UPDATE ap_idempotency_cache
                SET locked_until = 0
                WHERE payer_address = ? AND idempotency_key = ?;
                """,
                (payer, idem_key),
            )
            self._conn.commit()
        except Exception:
            if self._conn.in_transaction:
                cursor.execute("ROLLBACK;")
            raise

    def get_settlement(self, settlement_id: str) -> GatewaySettlementRecord | None:
        """Fetch GatewaySettlementRecord by settlement_id."""
        cursor = self._conn.cursor()
        cursor.execute(
            """
            SELECT settlement_id, idempotency_key, payer_address, payee_address,
                   asset_address, network_id, amount_base_units, nonce,
                   valid_after, valid_before, raw_signature, status, tx_hash,
                   block_number, error_message, created_at, updated_at
            FROM ap_gateway_settlement
            WHERE settlement_id = ?;
            """,
            (settlement_id,),
        )
        row = cursor.fetchone()
        if not row:
            return None
        return GatewaySettlementRecord(
            settlementId=row[0],
            idempotencyKey=row[1],
            payerAddress=row[2],
            payeeAddress=row[3],
            assetAddress=row[4],
            networkId=row[5],
            amountBaseUnits=row[6],
            nonce=row[7],
            validAfter=row[8],
            validBefore=row[9],
            rawSignature=row[10],
            status=SettlementState(row[11]),
            txHash=row[12],
            blockNumber=row[13],
            errorMessage=row[14],
            createdAt=row[15],
            updatedAt=row[16],
        )

    def get_fulfillment(self, fulfillment_id: str) -> GatewayFulfillmentRecord | None:
        """Fetch GatewayFulfillmentRecord by fulfillment_id."""
        cursor = self._conn.cursor()
        cursor.execute(
            """
            SELECT fulfillment_id, settlement_id, status, upstream_status_code,
                   response_hash, retry_count, delivered_at, created_at, updated_at
            FROM ap_gateway_fulfillment
            WHERE fulfillment_id = ?;
            """,
            (fulfillment_id,),
        )
        row = cursor.fetchone()
        if not row:
            return None
        return GatewayFulfillmentRecord(
            fulfillmentId=row[0],
            settlementId=row[1],
            status=FulfillmentState(row[2]),
            upstreamStatusCode=row[3],
            responseHash=row[4],
            retryCount=row[5],
            deliveredAt=row[6],
            createdAt=row[7],
            updatedAt=row[8],
        )

    def get_fulfillment_by_settlement(self, settlement_id: str) -> GatewayFulfillmentRecord | None:
        """Fetch GatewayFulfillmentRecord associated with a settlement_id."""
        cursor = self._conn.cursor()
        cursor.execute(
            """
            SELECT fulfillment_id, settlement_id, status, upstream_status_code,
                   response_hash, retry_count, delivered_at, created_at, updated_at
            FROM ap_gateway_fulfillment
            WHERE settlement_id = ?;
            """,
            (settlement_id,),
        )
        row = cursor.fetchone()
        if not row:
            return None
        return GatewayFulfillmentRecord(
            fulfillmentId=row[0],
            settlementId=row[1],
            status=FulfillmentState(row[2]),
            upstreamStatusCode=row[3],
            responseHash=row[4],
            retryCount=row[5],
            deliveredAt=row[6],
            createdAt=row[7],
            updatedAt=row[8],
        )

    def list_settlements(self, states: list[SettlementState] | None = None) -> list[GatewaySettlementRecord]:
        """Fetch GatewaySettlementRecords filtered optionally by state."""
        cursor = self._conn.cursor()
        if states:
            placeholders = ",".join("?" for _ in states)
            sql = f"""
            SELECT settlement_id, idempotency_key, payer_address, payee_address,
                   asset_address, network_id, amount_base_units, nonce,
                   valid_after, valid_before, raw_signature, status, tx_hash,
                   block_number, error_message, created_at, updated_at
            FROM ap_gateway_settlement
            WHERE status IN ({placeholders})
            ORDER BY created_at ASC;
            """
            cursor.execute(sql, tuple(s.value for s in states))
        else:
            sql = """
            SELECT settlement_id, idempotency_key, payer_address, payee_address,
                   asset_address, network_id, amount_base_units, nonce,
                   valid_after, valid_before, raw_signature, status, tx_hash,
                   block_number, error_message, created_at, updated_at
            FROM ap_gateway_settlement
            ORDER BY created_at ASC;
            """
            cursor.execute(sql)

        records: list[GatewaySettlementRecord] = []
        for row in cursor.fetchall():
            records.append(
                GatewaySettlementRecord(
                    settlementId=row[0],
                    idempotencyKey=row[1],
                    payerAddress=row[2],
                    payeeAddress=row[3],
                    assetAddress=row[4],
                    networkId=row[5],
                    amountBaseUnits=row[6],
                    nonce=row[7],
                    validAfter=row[8],
                    validBefore=row[9],
                    rawSignature=row[10],
                    status=SettlementState(row[11]),
                    txHash=row[12],
                    blockNumber=row[13],
                    errorMessage=row[14],
                    createdAt=row[15],
                    updatedAt=row[16],
                )
            )
        return records
