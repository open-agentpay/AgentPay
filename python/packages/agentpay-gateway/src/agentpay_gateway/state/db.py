"""
Gateway Database connection utilities and SQLite schema initialization for AgentPay.

Enforces:
- Foreign key constraints (PRAGMA foreign_keys = ON)
- WAL journal mode for concurrent read/write isolation (PRAGMA journal_mode = WAL)
- Busy timeout to gracefully queue during concurrent write contention
- Complete Gateway DDL schema (ap_gateway_settlement, ap_gateway_fulfillment, ap_idempotency_cache, ap_audit_event)
- P0-7 Unique nonce replay defense
- P1-5 Strict idempotency lease scoping
"""

from __future__ import annotations

import sqlite3
from pathlib import Path


def create_gateway_connection(
    db_path: str | Path,
    busy_timeout_ms: int = 5000,
    check_same_thread: bool = False,
) -> sqlite3.Connection:
    """
    Factory function to create a standardized SQLite connection to the Gateway database.

    Guarantees:
    - Connection-level ``PRAGMA foreign_keys = ON;``
    - Connection-level ``PRAGMA busy_timeout = {busy_timeout_ms};``
    - Database-level ``PRAGMA journal_mode = WAL;`` (for file-based databases)

    Args:
        db_path: File path or ':memory:' for in-memory databases.
        busy_timeout_ms: Timeout in milliseconds for locked database retries.
        check_same_thread: Whether SQLite restricts connection to the creator thread.

    Returns:
        Configured sqlite3.Connection.
    """
    str_path = str(db_path)
    conn = sqlite3.connect(
        str_path,
        timeout=float(busy_timeout_ms) / 1000.0,
        check_same_thread=check_same_thread,
        isolation_level=None,
    )
    conn.execute("PRAGMA foreign_keys = ON;")
    conn.execute(f"PRAGMA busy_timeout = {busy_timeout_ms};")
    if str_path != ":memory:":
        conn.execute("PRAGMA journal_mode = WAL;")
    return conn


def init_gateway_database(conn: sqlite3.Connection) -> None:
    """
    Initialize the gateway database tables, constraints, and indexes from schema DDL.
    """
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS ap_gateway_settlement (
            settlement_id TEXT PRIMARY KEY,
            idempotency_key TEXT NOT NULL,
            payer_address TEXT NOT NULL,
            payee_address TEXT NOT NULL,
            asset_address TEXT NOT NULL,
            network_id TEXT NOT NULL,
            amount_base_units INTEGER NOT NULL CHECK (amount_base_units > 0 AND typeof(amount_base_units) = 'integer'),
            nonce TEXT NOT NULL,
            valid_after INTEGER NOT NULL,
            valid_before INTEGER NOT NULL,
            raw_signature TEXT NOT NULL,
            status TEXT NOT NULL DEFAULT 'NOT_STARTED' CHECK (status IN (
                'NOT_STARTED', 'NOT_SUBMITTED', 'SUBMITTED', 'PENDING', 'CONFIRMED', 'SUCCESS', 'FAILED', 'REORGED', 'UNKNOWN'
            )),
            tx_hash TEXT,
            block_number INTEGER,
            error_message TEXT,
            created_at INTEGER NOT NULL,
            updated_at INTEGER NOT NULL,
            CONSTRAINT uq_network_asset_payer_nonce UNIQUE (network_id, asset_address, payer_address, nonce)
        );
        """
    )
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS ap_gateway_fulfillment (
            fulfillment_id TEXT PRIMARY KEY,
            settlement_id TEXT NOT NULL REFERENCES ap_gateway_settlement(settlement_id),
            status TEXT NOT NULL DEFAULT 'NOT_STARTED' CHECK (status IN (
                'NOT_STARTED', 'NOT_DELIVERED', 'DELIVERING', 'PROCESSING', 'DELIVERED', 'FAILED', 'RECOVERY_REQUIRED'
            )),
            upstream_status_code INTEGER,
            response_hash TEXT,
            retry_count INTEGER NOT NULL DEFAULT 0,
            delivered_at INTEGER,
            created_at INTEGER NOT NULL,
            updated_at INTEGER NOT NULL
        );
        """
    )
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS ap_idempotency_cache (
            payer_address TEXT NOT NULL,
            idempotency_key TEXT NOT NULL,
            settlement_id TEXT REFERENCES ap_gateway_settlement(settlement_id),
            status TEXT NOT NULL CHECK (status IN ('IN_FLIGHT', 'COMPLETED', 'FAILED')),
            request_hash TEXT NOT NULL,
            response_status_code INTEGER,
            response_headers_json TEXT,
            response_body_blob BLOB,
            locked_until INTEGER NOT NULL,
            expires_at INTEGER NOT NULL,
            created_at INTEGER NOT NULL,
            PRIMARY KEY (payer_address, idempotency_key)
        );
        """
    )
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS ap_audit_event (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            event_type TEXT NOT NULL,
            payer_address TEXT,
            payload_json TEXT NOT NULL,
            prev_hash TEXT NOT NULL,
            entry_hash TEXT NOT NULL,
            created_at INTEGER NOT NULL
        );
        """
    )
    conn.execute(
        """
        CREATE INDEX IF NOT EXISTS idx_settlement_lookup 
        ON ap_gateway_settlement (payer_address, idempotency_key);
        """
    )
    conn.execute(
        """
        CREATE INDEX IF NOT EXISTS idx_idempotency_lease 
        ON ap_idempotency_cache (locked_until, status);
        """
    )
    conn.commit()
