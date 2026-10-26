"""
Unit tests for Gateway Route Matcher (Longest Prefix Match) and Upstream URL Construction.
"""

from __future__ import annotations

import pytest
from agentpay_gateway.config.errors import MethodNotAllowedError, RouteNotFoundError
from agentpay_gateway.config.models import RouteConfig
from agentpay_gateway.config.router import RouteMatcher

VALID_PAY_TO = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"


@pytest.fixture
def sample_routes() -> list[RouteConfig]:
    return [
        RouteConfig(
            path="/v1/market-data",
            upstream="https://api.internal/market-data",
            price="0.001 USDC",
            pay_to=VALID_PAY_TO,
            methods=["GET"],
        ),
        RouteConfig(
            path="/v1/market-data/depth",
            upstream="https://api.internal/depth-engine",
            price="0.005 USDC",
            pay_to=VALID_PAY_TO,
            methods=["GET"],
        ),
        RouteConfig(
            path="/v1/trade",
            upstream="https://trading.internal",
            price="0.01 USDC",
            pay_to=VALID_PAY_TO,
            methods=["POST"],
            strip_path_prefix=True,
        ),
        RouteConfig(
            path="/v1/echo",
            upstream="https://echo.internal",
            price="0.0001 USDC",
            pay_to=VALID_PAY_TO,
            methods=["GET", "POST"],
            strip_path_prefix=False,
        ),
        RouteConfig(
            path="/v1/search",
            upstream="https://search.internal/query?source=agentpay",
            price="0.002 USDC",
            pay_to=VALID_PAY_TO,
            methods=["GET"],
        ),
    ]


class TestRouteMatcher:
    """Tests for RouteMatcher routing logic."""

    def test_longest_prefix_match_precedence(self, sample_routes: list[RouteConfig]) -> None:
        matcher = RouteMatcher(sample_routes)

        # /v1/market-data/depth must match the more specific route (/v1/market-data/depth)
        res_depth = matcher.match(method="GET", path="/v1/market-data/depth")
        assert res_depth.route.path == "/v1/market-data/depth"
        assert res_depth.upstream_url == "https://api.internal/depth-engine"
        assert res_depth.subpath == ""

        # /v1/market-data/ticker must match /v1/market-data with subpath /ticker
        res_ticker = matcher.match(method="GET", path="/v1/market-data/ticker")
        assert res_ticker.route.path == "/v1/market-data"
        assert res_ticker.upstream_url == "https://api.internal/market-data/ticker"
        assert res_ticker.subpath == "/ticker"

    def test_exact_path_match(self, sample_routes: list[RouteConfig]) -> None:
        matcher = RouteMatcher(sample_routes)
        res = matcher.match(method="GET", path="/v1/market-data")
        assert res.route.path == "/v1/market-data"
        assert res.upstream_url == "https://api.internal/market-data"
        assert res.subpath == ""

    def test_trailing_slash_normalization(self, sample_routes: list[RouteConfig]) -> None:
        matcher = RouteMatcher(sample_routes)
        res = matcher.match(method="GET", path="/v1/market-data/")
        assert res.route.path == "/v1/market-data"
        assert res.upstream_url == "https://api.internal/market-data"

    def test_method_not_allowed(self, sample_routes: list[RouteConfig]) -> None:
        matcher = RouteMatcher(sample_routes)
        with pytest.raises(MethodNotAllowedError) as exc_info:
            matcher.match(method="DELETE", path="/v1/market-data")

        assert exc_info.value.method == "DELETE"
        assert exc_info.value.path == "/v1/market-data"
        assert exc_info.value.allowed_methods == ["GET"]

    def test_route_not_found(self, sample_routes: list[RouteConfig]) -> None:
        matcher = RouteMatcher(sample_routes)
        with pytest.raises(RouteNotFoundError) as exc_info:
            matcher.match(method="GET", path="/v2/unknown")

        assert exc_info.value.path == "/v2/unknown"

    def test_strip_path_prefix_true(self, sample_routes: list[RouteConfig]) -> None:
        matcher = RouteMatcher(sample_routes)

        # /v1/trade/submit -> strip prefix /v1/trade -> /submit
        res = matcher.match(method="POST", path="/v1/trade/submit")
        assert res.route.path == "/v1/trade"
        assert res.upstream_url == "https://trading.internal/submit"
        assert res.subpath == "/submit"

        # /v1/trade (exact match) -> strip prefix -> /
        res_root = matcher.match(method="POST", path="/v1/trade")
        assert res_root.upstream_url == "https://trading.internal/"
        assert res_root.subpath == ""

    def test_strip_path_prefix_false(self, sample_routes: list[RouteConfig]) -> None:
        matcher = RouteMatcher(sample_routes)

        # Upstream has no base_path, strip_path_prefix is False -> full path passed
        res = matcher.match(method="POST", path="/v1/echo/test")
        assert res.upstream_url == "https://echo.internal/v1/echo/test"
        assert res.subpath == "/test"

    def test_query_parameters_forwarding_and_merging(self, sample_routes: list[RouteConfig]) -> None:
        matcher = RouteMatcher(sample_routes)

        # Standard query forwarding
        res = matcher.match(
            method="GET",
            path="/v1/market-data/ticker",
            query_string="symbol=ETH&interval=1h",
        )
        assert res.upstream_url == "https://api.internal/market-data/ticker?symbol=ETH&interval=1h"

        # Query merging with upstream predefined query
        res_merged = matcher.match(
            method="GET",
            path="/v1/search",
            query_string="q=antigravity",
        )
        assert res_merged.upstream_url == "https://search.internal/query?source=agentpay&q=antigravity"

    def test_disjoint_method_matching_same_path(self) -> None:
        routes = [
            RouteConfig(
                path="/v1/items",
                upstream="https://api.internal/items-read",
                price="0.001 USDC",
                pay_to=VALID_PAY_TO,
                methods=["GET"],
            ),
            RouteConfig(
                path="/v1/items",
                upstream="https://api.internal/items-write",
                price="0.005 USDC",
                pay_to=VALID_PAY_TO,
                methods=["POST"],
            ),
        ]
        matcher = RouteMatcher(routes)

        res_get = matcher.match(method="GET", path="/v1/items")
        assert res_get.upstream_url == "https://api.internal/items-read"
        assert res_get.route.price_base_units == 1000

        res_post = matcher.match(method="POST", path="/v1/items")
        assert res_post.upstream_url == "https://api.internal/items-write"
        assert res_post.route.price_base_units == 5000
