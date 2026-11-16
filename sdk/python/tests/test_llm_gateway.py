import pytest
from agentpay.llm import parse_stream_chunk, MidStreamBudgetExhaustedError, create_openai_client


def test_parse_stream_chunk_normal():
    line = 'data: {"choices":[{"delta":{"content":"Hello"}}]}'
    res = parse_stream_chunk(line)
    assert res is not None
    assert res["choices"][0]["delta"]["content"] == "Hello"


def test_parse_stream_chunk_done():
    line = "data: [DONE]"
    res = parse_stream_chunk(line)
    assert res == {"done": True}


def test_parse_stream_chunk_budget_exhausted():
    line = 'data: {"error":{"code":"STREAM_BUDGET_EXHAUSTED","message":"Tenant wallet budget cap reached mid-stream. Stream terminated.","accumulated_tokens":42}}'
    with pytest.raises(MidStreamBudgetExhaustedError) as exc_info:
        parse_stream_chunk(line)

    err = exc_info.value
    assert err.code == "STREAM_BUDGET_EXHAUSTED"
    assert err.accumulated_tokens == 42
    assert "Tenant wallet budget cap reached" in str(err)


def test_create_openai_client_url_construction():
    # If openai package is not installed in the testing environment, it should raise ImportError with clear message
    try:
        client = create_openai_client(gateway_url="http://127.0.0.1:8080", api_key="apk_test_123")
        assert str(client.base_url).startswith("http://127.0.0.1:8080/v1")
    except ImportError as e:
        assert "openai" in str(e)
