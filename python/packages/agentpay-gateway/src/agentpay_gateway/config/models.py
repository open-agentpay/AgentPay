"""
Data Models and Pydantic Schemas for Gateway Configuration.
"""

from __future__ import annotations

import re
from typing import Any

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

from agentpay_gateway.config.errors import (
    GatewayConfigValidationError,
    RouteConflictError,
)
from agentpay_gateway.config.price_parser import parse_price_to_base_units

EVM_ADDRESS_REGEX = re.compile(r"^0x[0-9a-fA-F]{40}$")
CAIP2_NETWORK_REGEX = re.compile(r"^[-a-z0-9]{3,8}:[-_a-zA-Z0-9]{1,32}$")
VALID_HTTP_METHODS = {"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}


class ServerConfig(BaseModel):
    """Server listener and process parameters."""

    model_config = ConfigDict(frozen=True)

    host: str = Field(default="0.0.0.0", description="Bind IP address")
    port: int = Field(default=8000, ge=1, le=65535, description="Bind port")
    db_path: str = Field(default="./data/gateway.db", description="Gateway SQLite database file path")
    default_timeout_seconds: float = Field(default=10.0, gt=0, description="Global default upstream timeout")
    audit_log_path: str | None = Field(default=None, description="Path to append structured audit events")


class FacilitatorConfig(BaseModel):
    """Facilitator settlement coordinator configuration."""

    model_config = ConfigDict(frozen=True)

    url: str = Field(default="http://127.0.0.1:8001", description="Facilitator HTTP endpoint")
    timeout_seconds: float = Field(default=5.0, gt=0, description="Facilitator client timeout")

    @field_validator("url")
    @classmethod
    def validate_url(cls, v: str) -> str:
        if not v.startswith(("http://", "https://")):
            raise GatewayConfigValidationError(f"Facilitator URL must begin with http:// or https://, got: {v}")
        return v.rstrip("/")


class SettlementDefaults(BaseModel):
    """Default blockchain settlement parameters across routes."""

    model_config = ConfigDict(frozen=True)

    default_network: str = Field(default="eip155:8453", description="Default CAIP-2 network ID (Base)")
    default_asset: str = Field(
        default="0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
        description="Default ERC-20 contract address (Base USDC)",
    )
    default_asset_symbol: str = Field(default="USDC", description="Default asset ticker symbol")
    default_asset_decimals: int = Field(default=6, ge=0, le=18, description="Default token decimals")
    default_challenge_ttl_seconds: int = Field(default=120, gt=0, description="Default 402 challenge expiration TTL")
    safe_buffer_seconds: int = Field(default=60, ge=0, description="Reconciliation safety window buffer")
    default_pay_to: str | None = Field(default=None, description="Default merchant receiving EVM address")

    @field_validator("default_network")
    @classmethod
    def validate_network(cls, v: str) -> str:
        if not CAIP2_NETWORK_REGEX.match(v):
            raise GatewayConfigValidationError(f"Invalid CAIP-2 network identifier: '{v}'")
        return v

    @field_validator("default_asset")
    @classmethod
    def validate_asset(cls, v: str) -> str:
        if not EVM_ADDRESS_REGEX.match(v):
            raise GatewayConfigValidationError(f"Invalid EVM asset address: '{v}'")
        return v.lower()

    @field_validator("default_pay_to")
    @classmethod
    def validate_pay_to(cls, v: str | None) -> str | None:
        if v is not None:
            if not EVM_ADDRESS_REGEX.match(v):
                raise GatewayConfigValidationError(f"Invalid default pay_to address: '{v}'")
            return v.lower()
        return None


class RouteConfig(BaseModel):
    """Configuration for an individual protected API route."""

    model_config = ConfigDict(frozen=True)

    path: str = Field(..., description="Route path prefix or exact path")
    upstream: str = Field(..., description="Target upstream base URL or full endpoint")
    price: str | int = Field(..., description="Route price (e.g. '0.001 USDC' or integer base units)")
    price_base_units: int = Field(default=0, description="Exact integer atomic units (computed)")
    pay_to: str = Field(default="", description="Merchant EVM receiving address")
    network: str = Field(default="", description="CAIP-2 network identifier")
    asset: str = Field(default="", description="ERC-20 token contract address")
    asset_symbol: str = Field(default="USDC", description="Token symbol")
    asset_decimals: int = Field(default=6, ge=0, le=18, description="Token decimals")
    methods: list[str] = Field(
        default_factory=lambda: ["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"],
        description="Allowed HTTP methods",
    )
    timeout_seconds: float = Field(default=10.0, gt=0, description="Per-route upstream timeout override")
    challenge_ttl_seconds: int = Field(default=120, gt=0, description="Per-route challenge validity window")
    upstream_headers: dict[str, str] = Field(
        default_factory=dict,
        description="Headers to inject when proxying to upstream (e.g. API keys)",
    )
    strip_path_prefix: bool = Field(
        default=False,
        description="Whether to strip the route path prefix before forwarding to upstream",
    )

    @field_validator("path")
    @classmethod
    def normalize_path(cls, v: str) -> str:
        clean = v.strip()
        if not clean.startswith("/"):
            clean = "/" + clean
        return clean.rstrip("/") if len(clean) > 1 else clean

    @field_validator("upstream")
    @classmethod
    def validate_upstream(cls, v: str) -> str:
        clean = v.strip()
        if not clean.startswith(("http://", "https://")):
            raise GatewayConfigValidationError(f"Upstream URL must begin with http:// or https://, got: '{v}'")
        return clean.rstrip("/")

    @field_validator("methods")
    @classmethod
    def validate_methods(cls, v: list[str]) -> list[str]:
        if not v:
            raise GatewayConfigValidationError("Route methods list cannot be empty")
        normalized = []
        for m in v:
            upper = m.strip().upper()
            if upper not in VALID_HTTP_METHODS:
                raise GatewayConfigValidationError(
                    f"Invalid HTTP method '{m}'. Allowed methods: {sorted(VALID_HTTP_METHODS)}"
                )
            if upper not in normalized:
                normalized.append(upper)
        return normalized

    @model_validator(mode="before")
    @classmethod
    def compute_pricing_and_defaults(cls, data: Any) -> Any:
        if not isinstance(data, dict):
            return data

        decimals = data.get("asset_decimals", 6)
        symbol = data.get("asset_symbol", "USDC")
        raw_price = data.get("price")

        if raw_price is not None:
            base_units = parse_price_to_base_units(raw_price, decimals=decimals, expected_symbol=symbol)
            data["price_base_units"] = base_units

        # Normalize address fields if present
        if "pay_to" in data and isinstance(data["pay_to"], str) and EVM_ADDRESS_REGEX.match(data["pay_to"]):
            data["pay_to"] = data["pay_to"].lower()
        if "asset" in data and isinstance(data["asset"], str) and EVM_ADDRESS_REGEX.match(data["asset"]):
            data["asset"] = data["asset"].lower()

        return data


class GatewayConfig(BaseModel):
    """Complete Top-Level Gateway Configuration Schema."""

    model_config = ConfigDict(frozen=True)

    version: str = Field(default="1.0", description="Configuration schema version")
    server: ServerConfig = Field(default_factory=ServerConfig)
    facilitator: FacilitatorConfig = Field(default_factory=FacilitatorConfig)
    settlement: SettlementDefaults = Field(default_factory=SettlementDefaults)
    routes: list[RouteConfig] = Field(default_factory=list)

    @model_validator(mode="before")
    @classmethod
    def propagate_settlement_defaults(cls, data: Any) -> Any:
        if not isinstance(data, dict):
            return data

        # Extract settlement defaults
        settlement_raw = data.get("settlement", {})
        if isinstance(settlement_raw, dict):
            def_pay_to = settlement_raw.get("default_pay_to")
            def_network = settlement_raw.get("default_network", "eip155:8453")
            def_asset = settlement_raw.get("default_asset", "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913")
            def_symbol = settlement_raw.get("default_asset_symbol", "USDC")
            def_decimals = settlement_raw.get("default_asset_decimals", 6)
            def_challenge_ttl = settlement_raw.get("default_challenge_ttl_seconds", 120)
        elif isinstance(settlement_raw, SettlementDefaults):
            def_pay_to = settlement_raw.default_pay_to
            def_network = settlement_raw.default_network
            def_asset = settlement_raw.default_asset
            def_symbol = settlement_raw.default_asset_symbol
            def_decimals = settlement_raw.default_asset_decimals
            def_challenge_ttl = settlement_raw.default_challenge_ttl_seconds
        else:
            def_pay_to = None
            def_network = "eip155:8453"
            def_asset = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
            def_symbol = "USDC"
            def_decimals = 6
            def_challenge_ttl = 120

        routes_raw = data.get("routes", [])
        if isinstance(routes_raw, list):
            new_routes = []
            for r in routes_raw:
                if isinstance(r, dict):
                    r_copy = dict(r)
                    if not r_copy.get("pay_to") and def_pay_to:
                        r_copy["pay_to"] = def_pay_to
                    if not r_copy.get("network") and def_network:
                        r_copy["network"] = def_network
                    if not r_copy.get("asset") and def_asset:
                        r_copy["asset"] = def_asset
                    if "asset_symbol" not in r_copy and def_symbol:
                        r_copy["asset_symbol"] = def_symbol
                    if "asset_decimals" not in r_copy and def_decimals is not None:
                        r_copy["asset_decimals"] = def_decimals
                    if "challenge_ttl_seconds" not in r_copy and def_challenge_ttl is not None:
                        r_copy["challenge_ttl_seconds"] = def_challenge_ttl
                    new_routes.append(r_copy)
                elif isinstance(r, RouteConfig):
                    updates: dict[str, Any] = {}
                    if not r.pay_to and def_pay_to:
                        updates["pay_to"] = def_pay_to
                    if not r.network and def_network:
                        updates["network"] = def_network
                    if not r.asset and def_asset:
                        updates["asset"] = def_asset
                    if updates:
                        new_routes.append(r.model_copy(update=updates))
                    else:
                        new_routes.append(r)
                else:
                    new_routes.append(r)
            data["routes"] = new_routes

        return data

    @model_validator(mode="after")
    def validate_routes(self) -> GatewayConfig:
        registered_methods_by_path: dict[str, set[str]] = {}

        for route in self.routes:
            if not route.pay_to:
                raise GatewayConfigValidationError(
                    f"Route '{route.path}' has no pay_to address and no settlement.default_pay_to is defined"
                )
            if not EVM_ADDRESS_REGEX.match(route.pay_to):
                raise GatewayConfigValidationError(f"Invalid pay_to address '{route.pay_to}' on route '{route.path}'")

            if not CAIP2_NETWORK_REGEX.match(route.network):
                raise GatewayConfigValidationError(f"Invalid network '{route.network}' on route '{route.path}'")

            if not EVM_ADDRESS_REGEX.match(route.asset):
                raise GatewayConfigValidationError(f"Invalid asset address '{route.asset}' on route '{route.path}'")

            existing_methods = registered_methods_by_path.setdefault(route.path, set())
            overlap = existing_methods.intersection(set(route.methods))
            if overlap:
                raise RouteConflictError(
                    f"Route conflict on path '{route.path}': methods {sorted(overlap)} already registered by an earlier route"
                )
            existing_methods.update(route.methods)

        return self
