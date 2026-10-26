"""
Unit & Integration Tests for AgentPay LangChain Paid Tool Integration.

Covers:
- @paid_tool decorator usage (standard, closure, auto client injection)
- PaidTool StructuredTool subclass invocation
- Async coroutine support
- Financial firewall / Policy violation interception
- PaidHttpTool pre-built LangChain tool for paid endpoints
"""

from __future__ import annotations

import asyncio
from typing import Any

import httpx
import pytest
from agentpay.client.client import AgentPayClient, AsyncAgentPayClient
from agentpay.client.signer import LocalSigner
from agentpay.core.codecs import (
    HEADER_PAYMENT_REQUIRED,
    HEADER_PAYMENT_SIGNATURE,
    decode_authorization,
    encode_challenge,
)
from agentpay.core.models import PaymentChallenge
from agentpay.integrations.langchain import PaidHttpInput, PaidHttpTool, PaidTool, paid_tool
from agentpay.policy.config import PolicyConfig
from agentpay.policy.db import create_buyer_connection, init_buyer_database
from agentpay.policy.sqlite_engine import SQLitePolicyEngine
from pydantic import BaseModel, Field


class MockPaidServerTransport(httpx.BaseTransport):
    """Simulates a protected upstream API demanding x402 payment."""

    def __init__(self, challenge: PaymentChallenge) -> None:
        super().__init__()
        self.challenge = challenge
        self.payments_received: list[dict[str, Any]] = []

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        sig_header = request.headers.get(HEADER_PAYMENT_SIGNATURE)
        if not sig_header:
            ch_encoded = encode_challenge(self.challenge)
            headers = {
                HEADER_PAYMENT_REQUIRED: ch_encoded,
                "Content-Type": "application/json",
            }
            return httpx.Response(
                402,
                headers=headers,
                json={"error": "Payment Required", "amount": self.challenge.amount},
                request=request,
            )

        auth = decode_authorization(sig_header)
        is_valid = LocalSigner.verify_authorization(auth, self.challenge)
        if not is_valid:
            return httpx.Response(401, json={"error": "Invalid signature"}, request=request)

        self.payments_received.append({"payer": auth.payer, "amount": self.challenge.amount})
        return httpx.Response(
            200,
            json={"status": "success", "data": f"Data for {request.url.path}", "paid": self.challenge.amount},
            request=request,
        )


class AsyncMockPaidServerTransport(httpx.AsyncBaseTransport):
    """Async variant for AsyncAgentPayClient."""

    def __init__(self, challenge: PaymentChallenge) -> None:
        super().__init__()
        self._sync = MockPaidServerTransport(challenge)

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        return self._sync.handle_request(request)


@pytest.fixture
def signer() -> LocalSigner:
    return LocalSigner("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")


@pytest.fixture
def standard_challenge() -> PaymentChallenge:
    return PaymentChallenge(
        version="2.0",
        challengeId="ch_langchain_01",
        network="eip155:84532",
        payee="0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
        asset="0x036cbd53842c5426634e7929541ec2318f3dcf7e",
        amount="50000",  # 0.05 USDC
        validAfter=1729000000,
        validBefore=1729000300,
        facilitatorUrl="https://facilitator.example.com",
        resourceUri="/v1/quote",
    )


@pytest.fixture
def policy_engine(signer: LocalSigner) -> SQLitePolicyEngine:
    conn = create_buyer_connection(":memory:")
    init_buyer_database(conn)
    cfg = PolicyConfig(
        id="pol_langchain",
        name="LangChain Test Policy",
        wallet_address=signer.address,
        max_per_tx_base_units=100000,  # 0.1 USDC cap
        rolling_budget_24h_base_units=1000000,  # 1.0 USDC 24h budget
        whitelist_payees=["0x70997970c51812dc3a010c7d01b50e0d17dc79c8"],
        whitelist_assets=["0x036cbd53842c5426634e7929541ec2318f3dcf7e"],
        whitelist_networks=["eip155:84532"],
        whitelist_domains=["api.example.com"],
    )
    return SQLitePolicyEngine(conn, cfg, clock=lambda: 1729000100)


@pytest.fixture
def agent_pay_client(
    signer: LocalSigner,
    policy_engine: SQLitePolicyEngine,
    standard_challenge: PaymentChallenge,
) -> AgentPayClient:
    transport = MockPaidServerTransport(standard_challenge)
    return AgentPayClient(
        signer=signer,
        policy=policy_engine,
        transport=transport,
        raise_on_policy_rejection=True,
    )


@pytest.fixture
def async_agent_pay_client(
    signer: LocalSigner,
    policy_engine: SQLitePolicyEngine,
    standard_challenge: PaymentChallenge,
) -> AsyncAgentPayClient:
    transport = AsyncMockPaidServerTransport(standard_challenge)
    return AsyncAgentPayClient(
        signer=signer,
        policy=policy_engine,
        transport=transport,
        raise_on_policy_rejection=True,
    )


class TestPaidToolDecorator:
    """Tests for the @paid_tool decorator."""

    def test_paid_tool_closure_usage(self, agent_pay_client: AgentPayClient) -> None:
        """Test decorator using closure over client (matching docs/api-specs.md §3.3.2)."""

        @paid_tool(
            name="get_stock_quote",
            description="获取实时美股行情数据 (按次扣费)",
            client=agent_pay_client,
        )
        def get_stock_quote(ticker: str) -> dict[str, Any]:
            return agent_pay_client.get(f"https://api.example.com/v1/quote?ticker={ticker}").json()

        assert isinstance(get_stock_quote, PaidTool)
        assert get_stock_quote.name == "get_stock_quote"
        assert "美股行情" in get_stock_quote.description

        # Verify LangChain argument schema
        schema = get_stock_quote.args_schema.model_json_schema()
        assert "ticker" in schema["properties"]

        # Run invocation
        result = get_stock_quote.invoke({"ticker": "AAPL"})
        assert result["status"] == "success"
        assert result["paid"] == "50000"

    def test_paid_tool_automatic_client_injection(self, agent_pay_client: AgentPayClient) -> None:
        """Test that 'client' parameter in function is injected and hidden from LangChain schema."""

        @paid_tool(
            name="get_weather",
            description="Query weather for city",
            client=agent_pay_client,
        )
        def get_weather(city: str, client: AgentPayClient) -> dict[str, Any]:
            return client.get(f"https://api.example.com/weather?city={city}").json()

        # 'client' must NOT be in the LLM tool schema
        schema = get_weather.args_schema.model_json_schema()
        assert "city" in schema["properties"]
        assert "client" not in schema["properties"]

        result = get_weather.invoke({"city": "Tokyo"})
        assert result["status"] == "success"

    def test_paid_tool_bare_decorator(self) -> None:
        """Test @paid_tool without arguments."""

        @paid_tool
        def calculate_sum(a: int, b: int) -> int:
            """Simple add."""
            return a + b

        assert isinstance(calculate_sum, PaidTool)
        assert calculate_sum.name == "calculate_sum"
        assert calculate_sum.invoke({"a": 2, "b": 3}) == 5

    def test_paid_tool_custom_args_schema(self, agent_pay_client: AgentPayClient) -> None:
        """Test @paid_tool with explicit Pydantic args_schema."""

        class CustomInput(BaseModel):
            symbol: str = Field(description="Asset symbol")
            interval: str = Field(default="1d", description="Time interval")

        @paid_tool(
            name="market_candles",
            client=agent_pay_client,
            args_schema=CustomInput,
        )
        def market_candles(symbol: str, interval: str = "1d") -> dict[str, Any]:
            return agent_pay_client.get(f"https://api.example.com/candles?s={symbol}&i={interval}").json()

        assert market_candles.args_schema is CustomInput
        res = market_candles.invoke({"symbol": "BTC", "interval": "1h"})
        assert res["status"] == "success"

    def test_paid_tool_async_coroutine(self, async_agent_pay_client: AsyncAgentPayClient) -> None:
        """Test @paid_tool on async coroutine."""

        @paid_tool(
            name="async_quote",
            client=async_agent_pay_client,
        )
        async def async_quote(ticker: str, client: AsyncAgentPayClient) -> dict[str, Any]:
            resp = await client.get(f"https://api.example.com/v1/quote?ticker={ticker}")
            return resp.json()

        res = asyncio.run(async_quote.ainvoke({"ticker": "ETH"}))
        assert res["status"] == "success"
        assert res["paid"] == "50000"


class TestPaidToolGuardrails:
    """Tests for Policy violation handling and firewall interception."""

    def test_policy_rejection_caught_as_tool_error(
        self,
        signer: LocalSigner,
        standard_challenge: PaymentChallenge,
    ) -> None:
        """When budget is exceeded, LangChain catches the error and provides clean LLM observation."""
        conn = create_buyer_connection(":memory:")
        init_buyer_database(conn)
        # Limit tx to 1000 base units, but challenge requires 50000
        cfg = PolicyConfig(
            id="pol_tiny",
            name="Tiny Budget Policy",
            wallet_address=signer.address,
            max_per_tx_base_units=1000,
            rolling_budget_24h_base_units=5000,
            whitelist_networks=["eip155:84532"],
            whitelist_assets=["0x036cbd53842c5426634e7929541ec2318f3dcf7e"],
            whitelist_payees=["0x70997970c51812dc3a010c7d01b50e0d17dc79c8"],
            whitelist_domains=["api.example.com"],
        )
        policy = SQLitePolicyEngine(conn, cfg, clock=lambda: 1729000100)
        client = AgentPayClient(
            signer=signer,
            policy=policy,
            transport=MockPaidServerTransport(standard_challenge),
            raise_on_policy_rejection=True,
        )

        @paid_tool(name="pricey_call", client=client, handle_tool_error=True)
        def pricey_call() -> str:
            return client.get("https://api.example.com/premium").text

        # In LangChain, with handle_tool_error=True, ToolException returns the string message!
        output = pricey_call.invoke({})
        assert isinstance(output, str)
        assert "AgentPay Firewall Rejected" in output
        assert "SINGLE_TX_CAP_EXCEEDED" in output

    def test_policy_rejection_raises_when_disabled(
        self,
        signer: LocalSigner,
        standard_challenge: PaymentChallenge,
    ) -> None:
        """When handle_policy_violation=False, raw PolicyViolationError is propagated."""
        from agentpay.policy.errors import PolicyViolationError

        conn = create_buyer_connection(":memory:")
        init_buyer_database(conn)
        cfg = PolicyConfig(
            id="pol_strict",
            name="Strict",
            wallet_address=signer.address,
            max_per_tx_base_units=1000,
            rolling_budget_24h_base_units=5000,
        )
        policy = SQLitePolicyEngine(conn, cfg, clock=lambda: 1729000100)
        client = AgentPayClient(
            signer=signer,
            policy=policy,
            transport=MockPaidServerTransport(standard_challenge),
            raise_on_policy_rejection=True,
        )

        @paid_tool(
            name="raw_call",
            client=client,
            handle_policy_violation=False,
            handle_tool_error=False,
        )
        def raw_call() -> str:
            return client.get("https://api.example.com/premium").text

        with pytest.raises(PolicyViolationError):
            raw_call.invoke({})


class TestPaidHttpTool:
    """Tests for the pre-built PaidHttpTool."""

    def test_paid_http_tool_sync_success(self, agent_pay_client: AgentPayClient) -> None:
        """Test out-of-the-box PaidHttpTool calling protected endpoint."""
        tool = PaidHttpTool(client=agent_pay_client)
        assert tool.name == "paid_http_request"
        assert tool.args_schema is PaidHttpInput

        response_str = tool.invoke(
            {
                "url": "https://api.example.com/v1/compute",
                "method": "POST",
                "json_data": {"query": "deep search"},
            }
        )

        assert "success" in response_str
        assert "Data for /v1/compute" in response_str

    def test_paid_http_tool_async_success(self, async_agent_pay_client: AsyncAgentPayClient) -> None:
        """Test PaidHttpTool async invocation."""
        tool = PaidHttpTool(async_client=async_agent_pay_client)

        response_str = asyncio.run(
            tool.ainvoke(
                {
                    "url": "https://api.example.com/v1/inference",
                    "method": "GET",
                }
            )
        )

        assert "success" in response_str
        assert "Data for /v1/inference" in response_str

    def test_paid_http_tool_missing_client(self) -> None:
        """Test ToolException raised when client is not configured."""
        tool = PaidHttpTool(client=None)
        output = tool.invoke({"url": "https://example.com"})
        assert "requires an AgentPayClient" in output

    def test_paid_http_tool_firewall_rejection(
        self,
        signer: LocalSigner,
        standard_challenge: PaymentChallenge,
    ) -> None:
        """Test that PaidHttpTool handles firewall policy rejection gracefully."""
        conn = create_buyer_connection(":memory:")
        init_buyer_database(conn)
        cfg = PolicyConfig(
            id="pol_blocked",
            name="Blocked Payee Policy",
            wallet_address=signer.address,
            max_per_tx_base_units=100000,
            rolling_budget_24h_base_units=1000000,
            whitelist_networks=["eip155:84532"],
            whitelist_assets=["0x036cbd53842c5426634e7929541ec2318f3dcf7e"],
            whitelist_payees=["0x1111111111111111111111111111111111111111"],  # Challenge payee is different
        )
        policy = SQLitePolicyEngine(conn, cfg, clock=lambda: 1729000100)
        client = AgentPayClient(
            signer=signer,
            policy=policy,
            transport=MockPaidServerTransport(standard_challenge),
            raise_on_policy_rejection=True,
        )

        tool = PaidHttpTool(client=client)
        output = tool.invoke({"url": "https://api.example.com/unauthorized"})

        assert "AgentPay Firewall Rejected" in output
        assert "PAYEE_NOT_WHITELISTED" in output
