"""
Gateway Configuration and Routing Exceptions.
"""

from __future__ import annotations


class GatewayConfigError(Exception):
    """Base exception for all Gateway configuration errors."""


class GatewayConfigFileNotFoundError(GatewayConfigError):
    """Raised when the specified gateway YAML configuration file does not exist."""


class GatewayConfigValidationError(GatewayConfigError):
    """Raised when the YAML configuration violates schema rules."""


class InvalidPriceError(GatewayConfigValidationError):
    """Raised when a route pricing string cannot be parsed or violates financial safety."""


class EnvInterpolationError(GatewayConfigError):
    """Raised when a referenced environment variable is missing without a default fallback."""


class RouteConflictError(GatewayConfigValidationError):
    """Raised when two routes define overlapping paths and HTTP methods."""


class RouteNotFoundError(GatewayConfigError):
    """Raised when an incoming HTTP request path does not match any configured route."""

    def __init__(self, path: str) -> None:
        super().__init__(f"No gateway route matches request path: '{path}'")
        self.path = path


class MethodNotAllowedError(GatewayConfigError):
    """Raised when a route matches the path but rejects the requested HTTP method."""

    def __init__(self, method: str, path: str, allowed_methods: list[str]) -> None:
        super().__init__(f"HTTP method '{method}' not allowed on route '{path}'. Allowed methods: {allowed_methods}")
        self.method = method
        self.path = path
        self.allowed_methods = allowed_methods
