"""
Lightweight EVM JSON-RPC Client for AgentPay (Arbitrum Sepolia & EVM Chains).

Uses httpx for high-performance HTTP JSON-RPC 2.0 communication without web3.py overhead:
- Chain ID verification
- On-chain EIP-3009 authorization state inspection
- Token balance queries
- Gas estimation and fee queries
- Raw transaction broadcasting and receipt polling
"""

from __future__ import annotations

import time
from typing import Any

import httpx

from agentpay.evm.constants import ARBITRUM_SEPOLIA_CHAIN_ID, ARBITRUM_SEPOLIA_RPC_URL
from agentpay.evm.eip3009 import (
    decode_authorization_state_result,
    decode_balance_of_result,
    encode_authorization_state,
    encode_balance_of,
)


class EvmRpcError(Exception):
    """Base exception for EVM JSON-RPC failures."""


class EvmRpcRequestError(EvmRpcError):
    """Raised when JSON-RPC returns an error response object."""

    def __init__(self, code: int, message: str, data: Any = None) -> None:
        super().__init__(f"RPC error {code}: {message}")
        self.code = code
        self.rpc_message = message
        self.data = data


class EvmRpcConnectionError(EvmRpcError):
    """Raised when the RPC node is unreachable or times out."""


class EvmRpcClient:
    """
    Synchronous JSON-RPC 2.0 client for EVM node communication.
    """

    def __init__(
        self,
        rpc_url: str = ARBITRUM_SEPOLIA_RPC_URL,
        *,
        timeout_seconds: float = 10.0,
        expected_chain_id: int | None = ARBITRUM_SEPOLIA_CHAIN_ID,
        client: httpx.Client | None = None,
        transport: httpx.BaseTransport | None = None,
    ) -> None:
        self.rpc_url = rpc_url
        self.timeout_seconds = timeout_seconds
        self.expected_chain_id = expected_chain_id
        self._request_id = 0

        if client is not None:
            self._client = client
            self._owns_client = False
        else:
            self._client = httpx.Client(
                transport=transport,
                timeout=timeout_seconds,
                trust_env=False,
            )
            self._owns_client = True

    def close(self) -> None:
        """Close the underlying HTTP client if created internally."""
        if self._owns_client:
            self._client.close()

    def __enter__(self) -> EvmRpcClient:
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()

    def _next_id(self) -> int:
        self._request_id += 1
        return self._request_id

    def execute_rpc(self, method: str, params: list[Any] | None = None) -> Any:
        """
        Execute raw JSON-RPC 2.0 request.
        """
        req_id = self._next_id()
        payload = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params or [],
            "id": req_id,
        }

        try:
            resp = self._client.post(self.rpc_url, json=payload)
        except (httpx.ConnectError, httpx.TimeoutException) as exc:
            raise EvmRpcConnectionError(f"Failed to connect to EVM RPC at {self.rpc_url}: {exc}") from exc
        except Exception as exc:
            raise EvmRpcConnectionError(f"Unexpected error communicating with EVM RPC: {exc}") from exc

        if resp.status_code != 200:
            raise EvmRpcConnectionError(f"HTTP {resp.status_code} returned by RPC endpoint: {resp.text}")

        try:
            data = resp.json()
        except Exception as exc:
            raise EvmRpcError(f"Invalid JSON returned by RPC endpoint: {resp.text}") from exc

        if "error" in data and data["error"]:
            err = data["error"]
            raise EvmRpcRequestError(
                code=err.get("code", -32000),
                message=err.get("message", "Unknown RPC error"),
                data=err.get("data"),
            )

        return data.get("result")

    # -------------------------------------------------------------------------
    # Core EVM Inquiries
    # -------------------------------------------------------------------------

    def get_chain_id(self) -> int:
        """Query eth_chainId and return integer chain ID."""
        res = self.execute_rpc("eth_chainId")
        if isinstance(res, str):
            return int(res, 16)
        return int(res)

    def verify_network(self) -> bool:
        """Verify that connected RPC chain ID matches expected_chain_id."""
        if self.expected_chain_id is None:
            return True
        actual_id = self.get_chain_id()
        if actual_id != self.expected_chain_id:
            raise EvmRpcError(
                f"Chain ID mismatch: expected {self.expected_chain_id}, connected node returned {actual_id}"
            )
        return True

    def get_block_number(self) -> int:
        """Query eth_blockNumber and return integer block height."""
        res = self.execute_rpc("eth_blockNumber")
        return int(res, 16) if isinstance(res, str) else int(res)

    def get_transaction_count(self, address: str, block: str = "pending") -> int:
        """Query account nonce via eth_getTransactionCount."""
        res = self.execute_rpc("eth_getTransactionCount", [address, block])
        return int(res, 16) if isinstance(res, str) else int(res)

    def call(self, to: str, data: bytes | str, block: str = "latest") -> str:
        """Execute eth_call and return hex result."""
        data_hex = data if isinstance(data, str) else "0x" + data.hex()
        tx = {"to": to, "data": data_hex}
        res = self.execute_rpc("eth_call", [tx, block])
        return str(res)

    def get_gas_price(self) -> int:
        """Query eth_gasPrice in wei."""
        res = self.execute_rpc("eth_gasPrice")
        return int(res, 16) if isinstance(res, str) else int(res)

    def get_latest_block(self) -> dict[str, Any]:
        """Fetch latest block header via eth_getBlockByNumber."""
        try:
            res = self.execute_rpc("eth_getBlockByNumber", ["latest", False])
            if isinstance(res, dict):
                return res
        except Exception:
            pass
        return {}

    def get_base_fee(self) -> int | None:
        """Fetch latest baseFeePerGas in wei from latest block, or None if unavailable."""
        try:
            blk = self.get_latest_block()
            base_fee_hex = blk.get("baseFeePerGas")
            if base_fee_hex is not None:
                if isinstance(base_fee_hex, str) and base_fee_hex.startswith("0x"):
                    return int(base_fee_hex, 16)
                return int(base_fee_hex)
        except Exception:
            pass
        return None

    def get_max_priority_fee(self) -> int:
        """Fetch eth_maxPriorityFeePerGas in wei, or 0 if unsupported."""
        try:
            res = self.execute_rpc("eth_maxPriorityFeePerGas")
            if isinstance(res, str) and res.startswith("0x"):
                return int(res, 16)
            elif isinstance(res, (int, float)):
                return int(res)
        except Exception:
            pass
        return 0

    def estimate_gas(
        self,
        to: str,
        data: bytes | str,
        from_address: str | None = None,
        value: int = 0,
    ) -> int:
        """Estimate transaction gas via eth_estimateGas."""
        data_hex = data if isinstance(data, str) else "0x" + data.hex()
        tx: dict[str, Any] = {"to": to, "data": data_hex}
        if from_address:
            tx["from"] = from_address
        if value > 0:
            tx["value"] = hex(value)
        res = self.execute_rpc("eth_estimateGas", [tx])
        return int(res, 16) if isinstance(res, str) else int(res)

    # -------------------------------------------------------------------------
    # EIP-3009 On-Chain Verification
    # -------------------------------------------------------------------------

    def check_authorization_state(
        self,
        token_address: str,
        authorizer: str,
        nonce: str | bytes,
    ) -> bool:
        """
        Query authorizationState(authorizer, nonce) on the token contract.
        Returns True if the nonce was already consumed or cancelled, False if available.
        """
        calldata = encode_authorization_state(authorizer, nonce)
        res_hex = self.call(to=token_address, data=calldata)
        return decode_authorization_state_result(res_hex)

    def get_token_balance(self, token_address: str, account: str) -> int:
        """Query ERC-20 balanceOf(account) in atomic base units."""
        calldata = encode_balance_of(account)
        res_hex = self.call(to=token_address, data=calldata)
        return decode_balance_of_result(res_hex)

    # -------------------------------------------------------------------------
    # Transaction Broadcasting & Lifecycle
    # -------------------------------------------------------------------------

    def send_raw_transaction(self, raw_tx: bytes | str) -> str:
        """
        Broadcast a signed raw transaction via eth_sendRawTransaction.
        Returns 0x-prefixed transaction hash.
        """
        raw_hex = raw_tx if isinstance(raw_tx, str) else "0x" + raw_tx.hex()
        if not raw_hex.startswith("0x"):
            raw_hex = "0x" + raw_hex
        res = self.execute_rpc("eth_sendRawTransaction", [raw_hex])
        return str(res)

    def get_transaction_receipt(self, tx_hash: str) -> dict[str, Any] | None:
        """
        Fetch transaction receipt via eth_getTransactionReceipt.
        Returns receipt dict if mined, None if still pending.
        """
        norm_hash = tx_hash if tx_hash.startswith("0x") else "0x" + tx_hash
        res = self.execute_rpc("eth_getTransactionReceipt", [norm_hash])
        if not res or not isinstance(res, dict):
            return None
        return res

    def wait_for_transaction_receipt(
        self,
        tx_hash: str,
        *,
        timeout_seconds: float = 30.0,
        poll_interval_seconds: float = 0.5,
    ) -> dict[str, Any]:
        """
        Poll until transaction receipt is available or timeout expires.
        """
        deadline = time.time() + timeout_seconds
        while time.time() < deadline:
            receipt = self.get_transaction_receipt(tx_hash)
            if receipt is not None:
                return receipt
            time.sleep(poll_interval_seconds)

        raise TimeoutError(f"Transaction receipt for {tx_hash} not found within {timeout_seconds} seconds")
