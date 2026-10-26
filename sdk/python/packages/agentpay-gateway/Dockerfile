FROM python:3.12-slim AS builder

WORKDIR /app

RUN pip install --no-cache-dir hatchling

# Install buyer SDK and gateway packages
COPY packages/agentpay /app/packages/agentpay
COPY packages/agentpay-gateway /app/packages/agentpay-gateway

RUN pip install --no-cache-dir /app/packages/agentpay /app/packages/agentpay-gateway

FROM python:3.12-slim AS runtime

WORKDIR /app

COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

EXPOSE 8000

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["uvicorn", "agentpay_gateway.server:app", "--host", "0.0.0.0", "--port", "8000"]
