---
title: Gateway Configuration Guide
description: gateway.yaml configuration schema reference, reverse proxy settings, and upstream route definitions.
---

# Gateway Configuration Guide

The AgentPay reverse proxy gateway is configured via YAML. A reference template is provided at `services/gateway/configs/gateway.example.yaml`.

## Basic Configuration Example

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  admin_port: 8081

storage:
  driver: "postgres"
  dsn: "postgres://agentpay:agentpay_secret@localhost:5432/agentpay?sslmode=disable"

wallet:
  default_hold_ttl: "120s"

routes:
  - path: "/weather/*"
    upstream: "https://api.weather.example.com"
    policy_id: "pol_weather_default"
```

For advanced settings including RCU hot-reloads and Prometheus metrics, see `services/gateway/configs/gateway.example.yaml`.
