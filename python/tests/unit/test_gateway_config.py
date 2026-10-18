"""
Unit tests for Gateway Configuration, Pricing Parser, and Environment Interpolation.
"""

from pathlib import Path

import pytest
from agentpay_gateway.config.errors import (
    EnvInterpolationError,
    GatewayConfigFileNotFoundError,
    GatewayConfigValidationError,
    InvalidPriceError,
)
from agentpay_gateway.config.loader import (
    interpolate_env_vars,
    load_gateway_config_from_dict,
    load_gateway_config_from_yaml,
)
from agentpay_gateway.config.price_parser import parse_price_to_base_units

VALID_PAY_TO = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"


# ==============================================================================
# Price Parser Tests
# ==============================================================================


class TestPriceParser:
    """Tests for parse_price_to_base_units."""

    def test_parse_human_readable_usdc_decimal(self) -> None:
        # 0.001 USDC with 6 decimals = 1000 atomic units
        assert parse_price_to_base_units("0.001 USDC", decimals=6, expected_symbol="USDC") == 1000

    def test_parse_human_readable_case_insensitive(self) -> None:
        assert parse_price_to_base_units("0.5 usdc", decimals=6, expected_symbol="USDC") == 500000

    def test_parse_whole_token(self) -> None:
        # 1.0 USDC = 1,000,000 units
        assert parse_price_to_base_units("1 USDC", decimals=6, expected_symbol="USDC") == 1000000
        assert parse_price_to_base_units("1.000000 USDC", decimals=6, expected_symbol="USDC") == 1000000

    def test_parse_integer_direct(self) -> None:
        assert parse_price_to_base_units(1000, decimals=6) == 1000
        assert parse_price_to_base_units(0, decimals=6) == 0

    def test_parse_raw_integer_string(self) -> None:
        # Without ticker symbol, string integer is treated as raw base units
        assert parse_price_to_base_units("5000", decimals=6) == 5000

    def test_parse_symbol_optional_matching(self) -> None:
        # If expected_symbol is None, accepts any symbol
        assert parse_price_to_base_units("0.01 DAI", decimals=18, expected_symbol=None) == 10000000000000000

    def test_reject_mismatched_symbol(self) -> None:
        with pytest.raises(InvalidPriceError, match="Currency symbol mismatch"):
            parse_price_to_base_units("0.001 DAI", decimals=6, expected_symbol="USDC")

    def test_reject_negative_price(self) -> None:
        with pytest.raises(InvalidPriceError, match="Price cannot be negative"):
            parse_price_to_base_units("-0.001 USDC", decimals=6)
        with pytest.raises(InvalidPriceError, match="Price cannot be negative"):
            parse_price_to_base_units(-100, decimals=6)

    def test_reject_exceeding_decimal_precision(self) -> None:
        # 7 decimal places for 6 decimals token must fail
        with pytest.raises(InvalidPriceError, match="exceeds token decimal precision"):
            parse_price_to_base_units("0.0000001 USDC", decimals=6)

    def test_reject_invalid_strings(self) -> None:
        with pytest.raises(InvalidPriceError):
            parse_price_to_base_units("free", decimals=6)
        with pytest.raises(InvalidPriceError):
            parse_price_to_base_units("0.00.1 USDC", decimals=6)
        with pytest.raises(InvalidPriceError):
            parse_price_to_base_units("", decimals=6)


# ==============================================================================
# Environment Interpolation Tests
# ==============================================================================


class TestEnvInterpolation:
    """Tests for interpolate_env_vars."""

    def test_simple_env_interpolation(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("TEST_GATEWAY_PORT", "9090")
        raw = "port: ${TEST_GATEWAY_PORT}"
        assert interpolate_env_vars(raw) == "port: 9090"

    def test_env_interpolation_with_default_dash(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.delenv("NON_EXISTENT_VAR", raising=False)
        raw = "timeout: ${NON_EXISTENT_VAR:-30}"
        assert interpolate_env_vars(raw) == "timeout: 30"

    def test_env_interpolation_with_default_colon(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.delenv("NON_EXISTENT_VAR", raising=False)
        raw = "host: ${NON_EXISTENT_VAR:127.0.0.1}"
        assert interpolate_env_vars(raw) == "host: 127.0.0.1"

    def test_env_present_overrides_default(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("OVERRIDE_VAR", "production_host")
        raw = "host: ${OVERRIDE_VAR:-localhost}"
        assert interpolate_env_vars(raw) == "host: production_host"

    def test_missing_env_without_default_raises(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.delenv("REQUIRED_SECRET_VAR", raising=False)
        raw = "api_key: ${REQUIRED_SECRET_VAR}"
        with pytest.raises(EnvInterpolationError, match="Missing required environment variable"):
            interpolate_env_vars(raw)


# ==============================================================================
# Gateway Config Schema & Loader Tests
# ==============================================================================


class TestGatewayConfig:
    """Tests for GatewayConfig model and YAML loading."""

    def test_minimal_gateway_config(self) -> None:
        config_dict = {
            "version": "1.0",
            "settlement": {
                "default_pay_to": VALID_PAY_TO,
            },
            "routes": [
                {
                    "path": "/v1/market-data",
                    "upstream": "http://127.0.0.1:9000/market-data",
                    "price": "0.001 USDC",
                }
            ],
        }
        cfg = load_gateway_config_from_dict(config_dict)
        assert cfg.version == "1.0"
        assert cfg.server.port == 8000
        assert len(cfg.routes) == 1
        route = cfg.routes[0]
        assert route.path == "/v1/market-data"
        assert route.price_base_units == 1000
        assert route.pay_to == VALID_PAY_TO.lower()
        assert route.network == "eip155:8453"
        assert route.asset == "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913"

    def test_load_from_yaml_string(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("MERCHANT_WALLET", VALID_PAY_TO)
        yaml_content = """
version: "1.0"
server:
  host: "127.0.0.1"
  port: 8080
settlement:
  default_pay_to: "${MERCHANT_WALLET}"
routes:
  - path: "/v1/orders"
    upstream: "http://127.0.0.1:9001"
    price: "0.005 USDC"
"""
        cfg = load_gateway_config_from_yaml(yaml_content)
        assert cfg.server.host == "127.0.0.1"
        assert cfg.server.port == 8080
        assert cfg.routes[0].price_base_units == 5000
        assert cfg.routes[0].pay_to == VALID_PAY_TO.lower()

    def test_load_from_yaml_file(self, tmp_path: Path) -> None:
        cfg_file = tmp_path / "gateway.yaml"
        cfg_file.write_text(
            f"""
version: "1.0"
settlement:
  default_pay_to: "{VALID_PAY_TO}"
routes:
  - path: "/v1/data"
    upstream: "http://127.0.0.1:8000"
    price: 100
"""
        )
        cfg = load_gateway_config_from_yaml(cfg_file)
        assert len(cfg.routes) == 1
        assert cfg.routes[0].price_base_units == 100

    def test_file_not_found_raises(self) -> None:
        with pytest.raises(GatewayConfigFileNotFoundError):
            load_gateway_config_from_yaml(Path("/non/existent/path/gateway.yaml"))

    def test_missing_pay_to_raises(self) -> None:
        config_dict = {
            "routes": [
                {
                    "path": "/v1/data",
                    "upstream": "http://127.0.0.1:9000",
                    "price": "0.001 USDC",
                }
            ]
        }
        with pytest.raises(GatewayConfigValidationError, match="has no pay_to address"):
            load_gateway_config_from_dict(config_dict)

    def test_invalid_pay_to_address_raises(self) -> None:
        config_dict = {
            "settlement": {
                "default_pay_to": "not-an-evm-address",
            },
            "routes": [
                {
                    "path": "/v1/data",
                    "upstream": "http://127.0.0.1:9000",
                    "price": "0.001 USDC",
                }
            ],
        }
        with pytest.raises(GatewayConfigValidationError, match="Invalid default pay_to address"):
            load_gateway_config_from_dict(config_dict)

    def test_invalid_network_caip2_raises(self) -> None:
        config_dict = {
            "settlement": {
                "default_network": "invalid_network_id",
                "default_pay_to": VALID_PAY_TO,
            },
            "routes": [
                {
                    "path": "/v1/data",
                    "upstream": "http://127.0.0.1:9000",
                    "price": "0.001 USDC",
                }
            ],
        }
        with pytest.raises(GatewayConfigValidationError, match="Invalid CAIP-2 network identifier"):
            load_gateway_config_from_dict(config_dict)

    def test_route_conflict_same_path_same_method_raises(self) -> None:
        config_dict = {
            "settlement": {
                "default_pay_to": VALID_PAY_TO,
            },
            "routes": [
                {
                    "path": "/v1/data",
                    "upstream": "http://upstream-1:9000",
                    "price": "0.001 USDC",
                    "methods": ["GET", "POST"],
                },
                {
                    "path": "/v1/data",
                    "upstream": "http://upstream-2:9000",
                    "price": "0.002 USDC",
                    "methods": ["POST", "DELETE"],
                },
            ],
        }
        with pytest.raises(GatewayConfigValidationError, match="Route conflict on path '/v1/data'"):
            load_gateway_config_from_dict(config_dict)

    def test_route_same_path_disjoint_methods_allowed(self) -> None:
        config_dict = {
            "settlement": {
                "default_pay_to": VALID_PAY_TO,
            },
            "routes": [
                {
                    "path": "/v1/data",
                    "upstream": "http://upstream-read:9000",
                    "price": "0.001 USDC",
                    "methods": ["GET"],
                },
                {
                    "path": "/v1/data",
                    "upstream": "http://upstream-write:9000",
                    "price": "0.005 USDC",
                    "methods": ["POST"],
                },
            ],
        }
        cfg = load_gateway_config_from_dict(config_dict)
        assert len(cfg.routes) == 2
        assert cfg.routes[0].methods == ["GET"]
        assert cfg.routes[1].methods == ["POST"]

    def test_example_paid_api_gateway_yaml(self) -> None:
        example_path = Path(__file__).resolve().parents[3] / "examples" / "paid-api" / "gateway.yaml"
        assert example_path.is_file(), f"Example gateway.yaml not found at {example_path}"
        cfg = load_gateway_config_from_yaml(example_path)
        assert cfg.version == "1.0"
        assert len(cfg.routes) == 3
        # Check market-data route
        r0 = cfg.routes[0]
        assert r0.path == "/v1/market-data"
        assert r0.price_base_units == 1000
        # Check depth route
        r1 = cfg.routes[1]
        assert r1.path == "/v1/market-data/depth"
        assert r1.price_base_units == 5000
        # Check infer route
        r2 = cfg.routes[2]
        assert r2.path == "/v1/ai/infer"
        assert r2.price_base_units == 20000
        assert r2.strip_path_prefix is True
