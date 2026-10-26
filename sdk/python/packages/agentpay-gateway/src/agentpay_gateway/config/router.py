"""
Route Matching and Upstream URL Construction Engine.

Implements:
- Longest Prefix Match (LPM) priority.
- HTTP method validation and 405 MethodNotAllowed signaling.
- Robust URL path concatenation and query string preservation.
"""

from __future__ import annotations

from urllib.parse import urlsplit, urlunsplit

from pydantic import BaseModel, ConfigDict

from agentpay_gateway.config.errors import MethodNotAllowedError, RouteNotFoundError
from agentpay_gateway.config.models import RouteConfig


class MatchResult(BaseModel):
    """Result of matching an inbound request against gateway route rules."""

    model_config = ConfigDict(frozen=True)

    route: RouteConfig
    upstream_url: str
    subpath: str


class RouteMatcher:
    """
    High-performance Route Matcher using Longest Prefix Matching (LPM).

    Guarantees:
    - More specific paths (e.g. '/v1/data/live') take precedence over prefixes ('/v1/data').
    - Exact path match takes precedence over prefix.
    - Path normalization prevents trailing slash mismatches.
    """

    def __init__(self, routes: list[RouteConfig]) -> None:
        # Sort routes by length of path descending for longest prefix match
        self.routes: list[RouteConfig] = sorted(
            routes,
            key=lambda r: len(r.path),
            reverse=True,
        )

    def match(
        self,
        method: str,
        path: str,
        query_string: str | None = None,
    ) -> MatchResult:
        """
        Match an incoming request path and method against configured routes.

        Args:
            method: HTTP method (e.g. 'GET', 'POST').
            path: Request URI path (e.g. '/v1/market-data/ticker').
            query_string: Optional raw query parameters string (e.g. 'symbol=BTC&limit=10').

        Returns:
            MatchResult containing matched RouteConfig and constructed upstream target URL.

        Raises:
            RouteNotFoundError: If no configured route matches the path.
            MethodNotAllowedError: If path matches but requested method is rejected.
        """
        norm_method = method.strip().upper()
        norm_path = path.strip()
        if not norm_path.startswith("/"):
            norm_path = "/" + norm_path

        # Normalize trailing slash (unless it is root "/")
        clean_path = norm_path.rstrip("/") if len(norm_path) > 1 else norm_path

        matched_path_routes: list[RouteConfig] = []

        for route in self.routes:
            route_path = route.path
            # Check exact match or subpath prefix match
            if clean_path == route_path or clean_path.startswith(route_path + "/"):
                matched_path_routes.append(route)

        if not matched_path_routes:
            raise RouteNotFoundError(clean_path)

        # Check method allowed
        target_route: RouteConfig | None = None
        all_allowed_methods: set[str] = set()

        for route in matched_path_routes:
            all_allowed_methods.update(route.methods)
            if norm_method in route.methods:
                target_route = route
                break

        if target_route is None:
            raise MethodNotAllowedError(
                method=norm_method,
                path=clean_path,
                allowed_methods=sorted(all_allowed_methods),
            )

        # Calculate subpath relative to route.path
        if clean_path == target_route.path:
            subpath = ""
        else:
            subpath = clean_path[len(target_route.path) :]

        # Construct upstream URL
        upstream_url = self._build_upstream_url(
            target_route=target_route,
            clean_path=clean_path,
            subpath=subpath,
            query_string=query_string,
        )

        return MatchResult(
            route=target_route,
            upstream_url=upstream_url,
            subpath=subpath,
        )

    def _build_upstream_url(
        self,
        target_route: RouteConfig,
        clean_path: str,
        subpath: str,
        query_string: str | None = None,
    ) -> str:
        parts = urlsplit(target_route.upstream)
        base_path = parts.path.rstrip("/")

        if base_path:
            # Upstream specifies an explicit target path prefix (e.g. /market-data or /internal)
            final_path = f"{base_path}{subpath}" if subpath else base_path
        else:
            # Upstream has no path prefix (e.g. http://upstream:8080)
            if target_route.strip_path_prefix:
                final_path = subpath if subpath else "/"
            else:
                final_path = clean_path

        # Preserve / combine query strings
        if parts.query and query_string:
            final_query = f"{parts.query}&{query_string}"
        else:
            final_query = query_string if query_string else parts.query

        return urlunsplit((parts.scheme, parts.netloc, final_path, final_query, ""))
