module github.com/open-agentpay/agentpay-gateway-go

go 1.21.6

require (
	github.com/agent-core-sdk/gateway v0.1.0
	github.com/lib/pq v1.12.3
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/agent-core-sdk/gateway => ../../../agent-core-sdk/gateway
