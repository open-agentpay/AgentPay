module github.com/open-agentpay/agentpay-gateway-go

go 1.21

require github.com/open-agentpay/agent-core-sdk/gateway v0.1.0

replace github.com/open-agentpay/agent-core-sdk/gateway => ../../../agent-core-sdk/gateway

require (
	github.com/lib/pq v1.12.3 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
