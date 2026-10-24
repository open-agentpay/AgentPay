# ==============================================================================
# Multi-Stage Build for AgentPay Go Facilitator Node
# ==============================================================================

# Build Stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /build/bin/facilitator \
    ./cmd/facilitator/main.go

# Runtime Stage
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=builder /build/bin/facilitator /app/facilitator
COPY --from=builder /build/configs/facilitator.yaml /etc/agentpay/facilitator.yaml

EXPOSE 8001

USER nobody

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8001/health || exit 1

ENTRYPOINT ["/app/facilitator", "-config", "/etc/agentpay/facilitator.yaml"]
