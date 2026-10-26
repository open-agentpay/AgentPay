.PHONY: build test clean run

BINARY_NAME=bin/gateway

build:
	go build -ldflags "-X 'github.com/open-agentpay/agentpay-gateway-go/internal/server.Version=2.1.0' -X 'github.com/open-agentpay/agentpay-gateway-go/internal/server.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)'" -o $(BINARY_NAME) ./cmd/gateway

test:
	go test -v ./...

clean:
	rm -rf bin/

run: build
	./$(BINARY_NAME) -config configs/gateway.example.yaml
