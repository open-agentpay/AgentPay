"""
EIP-3009 ABI Encoding and Decoding Utilities for AgentPay.

Implements calldata builders and result decoders for FiatTokenV2_2 / EIP-3009:
- transferWithAuthorization
- receiveWithAuthorization
- cancelAuthorization
- authorizationState
- balanceOf
"""

from __future__ import annotations

from typing import Any

import eth_abi
from eth_utils import to_checksum_address

from agentpay.core.models import PaymentAuthorization
from agentpay.evm.constants import (
    AUTHORIZATION_STATE_SELECTOR,
    BALANCE_OF_SELECTOR,
    TRANSFER_WITH_AUTHORIZATION_SELECTOR,
)

_TRANSFER_WITH_AUTH_TYPES = [
    "address",  # from
    "address",  # to
    "uint256",  # value
    "uint256",  # validAfter
    "uint256",  # validBefore
    "bytes32",  # nonce
    "uint8",  # v
    "bytes32",  # r
    "bytes32",  # s
]

_AUTH_STATE_TYPES = [
    "address",  # authorizer
    "bytes32",  # nonce
]


def _normalize_bytes32(val: str | bytes) -> bytes:
    """Normalize hex string or bytes to 32-byte binary."""
    if isinstance(val, str):
        hex_str = val[2:] if val.startswith("0x") else val
        return bytes.fromhex(hex_str.rjust(64, "0"))
    if isinstance(val, (bytes, bytearray)):
        if len(val) != 32:
            raise ValueError(f"Expected 32 bytes, got {len(val)}")
        return bytes(val)
    raise TypeError(f"Expected str or bytes, got {type(val)}")


def encode_transfer_with_authorization(
    from_address: str,
    to_address: str,
    value: int,
    valid_after: int,
    valid_before: int,
    nonce: str | bytes,
    v: int,
    r: str | bytes,
    s: str | bytes,
) -> bytes:
    """
    Encode calldata for ERC-3009 transferWithAuthorization.

    Selector: 0xe3ee160e (4 bytes)
    Calldata: 292 bytes total
    """
    checksum_from = to_checksum_address(from_address)
    checksum_to = to_checksum_address(to_address)
    nonce_b32 = _normalize_bytes32(nonce)
    r_b32 = _normalize_bytes32(r)
    s_b32 = _normalize_bytes32(s)

    payload = eth_abi.encode(
        _TRANSFER_WITH_AUTH_TYPES,
        [
            checksum_from,
            checksum_to,
            value,
            valid_after,
            valid_before,
            nonce_b32,
            v,
            r_b32,
            s_b32,
        ],
    )
    return TRANSFER_WITH_AUTHORIZATION_SELECTOR + payload


def encode_transfer_from_authorization(
    authorization: PaymentAuthorization,
    to_address: str,
    value: int,
) -> bytes:
    """
    Helper to encode transferWithAuthorization directly from PaymentAuthorization model.
    """
    sig = authorization.signature
    r = sig.r
    s = sig.s
    v = sig.v

    return encode_transfer_with_authorization(
        from_address=authorization.payer,
        to_address=to_address,
        value=value,
        valid_after=authorization.valid_after,
        valid_before=authorization.valid_before,
        nonce=authorization.nonce,
        v=v,
        r=r,
        s=s,
    )


def decode_transfer_with_authorization(calldata: bytes | str) -> dict[str, Any]:
    """
    Decode transferWithAuthorization calldata into its structured parameters.
    """
    raw_bytes = bytes.fromhex(calldata[2:]) if isinstance(calldata, str) and calldata.startswith("0x") else calldata
    if isinstance(raw_bytes, str):
        raw_bytes = bytes.fromhex(raw_bytes)

    if len(raw_bytes) < 4:
        raise ValueError("Calldata too short (less than 4 bytes)")

    selector = raw_bytes[:4]
    if selector != TRANSFER_WITH_AUTHORIZATION_SELECTOR:
        raise ValueError(
            f"Selector mismatch: expected {TRANSFER_WITH_AUTHORIZATION_SELECTOR.hex()}, got {selector.hex()}"
        )

    decoded = eth_abi.decode(_TRANSFER_WITH_AUTH_TYPES, raw_bytes[4:])
    return {
        "from": decoded[0],
        "to": decoded[1],
        "value": decoded[2],
        "validAfter": decoded[3],
        "validBefore": decoded[4],
        "nonce": "0x" + decoded[5].hex(),
        "v": decoded[6],
        "r": "0x" + decoded[7].hex(),
        "s": "0x" + decoded[8].hex(),
    }


def encode_authorization_state(authorizer: str, nonce: str | bytes) -> bytes:
    """
    Encode calldata for authorizationState(address authorizer, bytes32 nonce).
    Selector: 0xe94a0102 (4 bytes) + 64 bytes arguments.
    """
    checksum_authorizer = to_checksum_address(authorizer)
    nonce_b32 = _normalize_bytes32(nonce)
    payload = eth_abi.encode(_AUTH_STATE_TYPES, [checksum_authorizer, nonce_b32])
    return AUTHORIZATION_STATE_SELECTOR + payload


def decode_authorization_state_result(data: bytes | str) -> bool:
    """
    Decode boolean response from authorizationState eth_call.
    Returns True if nonce has already been used or cancelled, False if unused.
    """
    raw_bytes = bytes.fromhex(data[2:]) if isinstance(data, str) and data.startswith("0x") else data
    if isinstance(raw_bytes, str):
        raw_bytes = bytes.fromhex(raw_bytes)

    if len(raw_bytes) < 32:
        return False
    decoded = eth_abi.decode(["bool"], raw_bytes[:32])
    return bool(decoded[0])


def encode_balance_of(account: str) -> bytes:
    """Encode calldata for ERC-20 balanceOf(address)."""
    checksum_account = to_checksum_address(account)
    return BALANCE_OF_SELECTOR + eth_abi.encode(["address"], [checksum_account])


def decode_balance_of_result(data: bytes | str) -> int:
    """Decode integer balance from ERC-20 balanceOf eth_call."""
    raw_bytes = bytes.fromhex(data[2:]) if isinstance(data, str) and data.startswith("0x") else data
    if isinstance(raw_bytes, str):
        raw_bytes = bytes.fromhex(raw_bytes)

    if len(raw_bytes) < 32:
        return 0
    decoded = eth_abi.decode(["uint256"], raw_bytes[:32])
    return int(decoded[0])
