"""
AgentPay LLM Gateway Integration Module.
Provides native OpenAI client factory and mid-stream circuit-breaker exception handling
for AgentFinOps & AgentPay Enterprise Model Gateway.
"""

from __future__ import annotations

import json
from typing import Any, Dict, Optional


class MidStreamBudgetExhaustedError(RuntimeError):
    """
    Raised when an ongoing SSE token stream is interrupted by the Model Gateway
    due to tenant wallet balance exhaustion or project budget cap limit.
    """

    def __init__(self, message: str, payload: Optional[Dict[str, Any]] = None):
        super().__init__(message)
        self.payload = payload or {}
        self.code = self.payload.get("code", "STREAM_BUDGET_EXHAUSTED")
        self.accumulated_tokens = self.payload.get("accumulated_tokens", 0)


def parse_stream_chunk(raw_chunk: str) -> Optional[Dict[str, Any]]:
    """
    Parses a single SSE data line and checks for circuit breaker error frames.
    
    Raises:
        MidStreamBudgetExhaustedError: if the stream chunk indicates STREAM_BUDGET_EXHAUSTED.
    """
    clean_line = raw_chunk.strip()
    if not clean_line or not clean_line.startswith("data:"):
        return None

    data_payload = clean_line[5:].strip()
    if data_payload == "[DONE]":
        return {"done": True}

    try:
        obj = json.loads(data_payload)
    except Exception:
        return None

    if isinstance(obj, dict) and "error" in obj:
        err = obj["error"]
        if err.get("code") == "STREAM_BUDGET_EXHAUSTED":
            raise MidStreamBudgetExhaustedError(
                message=err.get("message", "Mid-stream budget exhausted; generation aborted by gateway circuit breaker"),
                payload=err,
            )

    return obj


def create_openai_client(
    gateway_url: str = "http://localhost:8080",
    api_key: str = "",
    **kwargs: Any,
) -> Any:
    """
    Creates an OpenAI-compatible client routed through AgentFinOps / AgentPay Model Gateway.
    Requires 'openai' python package to be installed.
    """
    try:
        from openai import OpenAI
    except ImportError as e:
        raise ImportError(
            "The 'openai' package is required to use create_openai_client. "
            "Please install it with: pip install openai"
        ) from e

    base_url = f"{gateway_url.rstrip('/')}/v1"
    return OpenAI(base_url=base_url, api_key=api_key or "apk_anonymous", **kwargs)
