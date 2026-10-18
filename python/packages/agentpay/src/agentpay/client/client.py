"""
AgentPayClient: Drop-in httpx.Client with automatic 402 payment negotiation.

Wraps the financial firewall and EIP-712 cryptographic signer to automatically
satisfy HTTP 402 Payment Required challenges.
"""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

import httpx

from agentpay.client.interfaces import ISigner
from agentpay.client.transport import AgentPayTransport, AsyncAgentPayTransport
from agentpay.policy.interfaces import IPolicyEngine


class AgentPayClient(httpx.Client):
    """
    HTTP client for autonomous AI agents with integrated financial firewall.

    Inherits from httpx.Client: fully compatible with LangChain, LlamaIndex,
    and standard Python HTTP toolchains.
    """

    def __init__(
        self,
        signer: ISigner,
        policy: IPolicyEngine,
        *,
        transport: httpx.BaseTransport | None = None,
        auto_retry_on_402: bool = True,
        raise_on_policy_rejection: bool = False,
        clock: Callable[[], int] | None = None,
        **kwargs: Any,
    ) -> None:
        self.signer = signer
        self.policy = policy
        self.auto_retry_on_402 = auto_retry_on_402
        self.raise_on_policy_rejection = raise_on_policy_rejection
        self._clock = clock

        inner = transport or httpx.HTTPTransport()
        agent_transport = AgentPayTransport(
            inner_transport=inner,
            signer=signer,
            policy=policy,
            auto_retry_on_402=auto_retry_on_402,
            raise_on_policy_rejection=raise_on_policy_rejection,
            clock=clock,
        )

        super().__init__(transport=agent_transport, **kwargs)

    def async_client(
        self,
        *,
        transport: httpx.AsyncBaseTransport | None = None,
        **kwargs: Any,
    ) -> AsyncAgentPayClient:
        """Create an asynchronous AsyncAgentPayClient sharing the same signer and policy."""
        return AsyncAgentPayClient(
            signer=self.signer,
            policy=self.policy,
            transport=transport,
            auto_retry_on_402=self.auto_retry_on_402,
            raise_on_policy_rejection=self.raise_on_policy_rejection,
            clock=self._clock,
            **kwargs,
        )


class AsyncAgentPayClient(httpx.AsyncClient):
    """
    Asynchronous HTTP client for autonomous AI agents with integrated financial firewall.

    Inherits from httpx.AsyncClient.
    """

    def __init__(
        self,
        signer: ISigner,
        policy: IPolicyEngine,
        *,
        transport: httpx.AsyncBaseTransport | None = None,
        auto_retry_on_402: bool = True,
        raise_on_policy_rejection: bool = False,
        clock: Callable[[], int] | None = None,
        **kwargs: Any,
    ) -> None:
        self.signer = signer
        self.policy = policy
        self.auto_retry_on_402 = auto_retry_on_402
        self.raise_on_policy_rejection = raise_on_policy_rejection
        self._clock = clock

        inner = transport or httpx.AsyncHTTPTransport()
        agent_transport = AsyncAgentPayTransport(
            inner_transport=inner,
            signer=signer,
            policy=policy,
            auto_retry_on_402=auto_retry_on_402,
            raise_on_policy_rejection=raise_on_policy_rejection,
            clock=clock,
        )

        super().__init__(transport=agent_transport, **kwargs)
