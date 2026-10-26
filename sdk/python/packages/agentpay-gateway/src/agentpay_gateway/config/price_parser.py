"""
Financial Safety Pricing Parser for Gateway Configuration.

Enforces:
- Exact decimal-to-integer conversion using decimal.Decimal.
- Zero floating-point arithmetic.
- Strict sub-unit cutoff (rejects fractions smaller than 1 base unit).
- Rejection of negative values.
- Precision safety: zero amounts allowed (free routes), base units always integer.
"""

from __future__ import annotations

import re
from decimal import Decimal, InvalidOperation

from agentpay_gateway.config.errors import InvalidPriceError


def parse_price_to_base_units(
    price_val: str | int,
    decimals: int = 6,
    expected_symbol: str | None = "USDC",
) -> int:
    """
    Parse a human-readable price string into exact integer base units.

    Examples:
        - "0.001 USDC" (decimals=6) -> 1000
        - "1 USDC" (decimals=6) -> 1000000
        - "5000" (raw integer / string) -> 5000
        - 1000 (raw integer) -> 1000
        - 0 -> 0

    Args:
        price_val: Price string (e.g. "0.001 USDC") or direct integer base units.
        decimals: Token decimals (default 6 for USDC).
        expected_symbol: Expected token symbol (e.g. "USDC"), or None to accept any.

    Returns:
        Integer base units (>= 0).

    Raises:
        InvalidPriceError: If amount is negative, malformed, or exceeds decimal resolution.
    """
    if isinstance(price_val, int):
        if price_val < 0:
            raise InvalidPriceError(f"Price cannot be negative, got: {price_val}")
        return price_val

    if not isinstance(price_val, str):
        raise InvalidPriceError(f"Price must be a string or integer, got {type(price_val).__name__}")

    raw_str = price_val.strip()
    if not raw_str:
        raise InvalidPriceError("Price string cannot be empty")

    if raw_str.startswith("-"):
        raise InvalidPriceError(f"Price cannot be negative, got: {price_val}")

    # Match: number, optional space, optional symbol
    match = re.match(r"^([0-9]+(?:\.[0-9]+)?)(?:\s+([A-Za-z0-9_-]+))?$", raw_str)
    if not match:
        raise InvalidPriceError(
            f"Malformed price format '{price_val}'. Expected format: '<amount> [SYMBOL]', e.g., '0.001 USDC'"
        )

    amount_str, symbol = match.groups()

    if symbol and expected_symbol:
        symbol_upper = symbol.upper()
        expected_upper = expected_symbol.upper()
        if symbol_upper != expected_upper:
            raise InvalidPriceError(
                f"Currency symbol mismatch: expected '{expected_symbol}', but route specified '{symbol}'"
            )

    try:
        dec_amount = Decimal(amount_str)
    except InvalidOperation as exc:
        raise InvalidPriceError(f"Invalid decimal number in price: '{amount_str}'") from exc

    if dec_amount < 0:
        raise InvalidPriceError(f"Price cannot be negative, got: {dec_amount}")

    # If no symbol specified and no decimal point present, treat as raw base units
    if symbol is None and "." not in amount_str:
        return int(dec_amount)

    multiplier = Decimal(10**decimals)
    scaled = dec_amount * multiplier

    # Verify no fractional base units exist
    if scaled != scaled.to_integral_value():
        raise InvalidPriceError(f"Price '{price_val}' exceeds token decimal precision ({decimals} decimals resolution)")

    base_units = int(scaled)
    if base_units < 0:
        raise InvalidPriceError(f"Resulting base units cannot be negative, got: {base_units}")

    return base_units
