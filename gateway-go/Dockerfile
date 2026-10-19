# ==============================================================================
# AgentPay Gateway Go Service Dockerfile
# Multi-stage build for minimal image size and maximum security
# ==============================================================================

# Stage 1: Build binary
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Cache go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static executable
ARG VERSION=2.0.0
ARG COMMIT=docker
ARG BUILD_DATE
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w \
      -X 'github.com/open-agentpay/agentpay-gateway-go/internal/server.Version=${VERSION}' \
      -X 'github.com/open-agentpay/agentpay-gateway-go/internal/server.GitCommit=${COMMIT}' \
      -X 'github.com/open-agentpay/agentpay-gateway-go/internal/server.BuildDate=${BUILD_DATE}'" \
    -o /build/bin/gateway ./cmd/gateway

# Stage 2: Runtime image
FROM alpine:3.19 AS runtime

# Install runtime dependencies: ca-certificates for TLS, tzdata, wget for healthcheck
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S agentpay && adduser -S -G agentpay agentpay

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bin/gateway /app/gateway

# Create default config directory
RUN mkdir -p /etc/agentpay && chown -R agentpay:agentpay /app /etc/agentpay

USER agentpay

EXPOSE 8000

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8000/healthz || exit 1

ENTRYPOINT ["/app/gateway"]
CMD ["-config", "/etc/agentpay/gateway.yaml"]
