"""
AgentPay Gateway Configuration and Routing Module.

Provides:
- Schema validation via Pydantic V2 (ServerConfig, RouteConfig, GatewayConfig, etc.)
- YAML configuration loading with ${ENV_VAR} dynamic interpolation
- Longest Prefix Match (LPM) route matching and upstream URL resolution
- Exact base-unit pricing parsing with zero floating-point arithmetic
"""

from __future__ import annotations

from agentpay_gateway.config.errors import (
    EnvInterpolationError,
    GatewayConfigError,
    GatewayConfigFileNotFoundError,
    GatewayConfigValidationError,
    InvalidPriceError,
    MethodNotAllowedError,
    RouteConflictError,
    RouteNotFoundError,
)
from agentpay_gateway.config.loader import (
    interpolate_env_vars,
    load_gateway_config_from_dict,
    load_gateway_config_from_yaml,
)
from agentpay_gateway.config.models import (
    FacilitatorConfig,
    GatewayConfig,
    RouteConfig,
    ServerConfig,
    SettlementDefaults,
)
from agentpay_gateway.config.price_parser import parse_price_to_base_units
from agentpay_gateway.config.router import MatchResult, RouteMatcher

__all__ = [
    # Schema Models
    "ServerConfig",
    "FacilitatorConfig",
    "SettlementDefaults",
    "RouteConfig",
    "GatewayConfig",
    # Loader & Interpolator
    "interpolate_env_vars",
    "load_gateway_config",
    "load_gateway_config_from_dict",
    "load_gateway_config_from_yaml",
    # Price Parser
    "parse_price_to_base_units",
    # Route Engine
    "RouteMatcher",
    "MatchResult",
    # Errors
    "GatewayConfigError",
    "GatewayConfigFileNotFoundError",
    "GatewayConfigValidationError",
    "InvalidPriceError",
    "EnvInterpolationError",
    "RouteConflictError",
    "RouteNotFoundError",
    "MethodNotAllowedError",
]
