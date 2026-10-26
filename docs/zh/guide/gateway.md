---
title: 网关配置指南
description: gateway.yaml 配置文件规格说明与运行时路由设置。
---

# 网关配置指南

AgentPay 网关使用 YAML 配置文件进行服务启动与路由策略声明。默认配置文件模板位于 `services/gateway/configs/gateway.example.yaml`。

## 基础配置示例

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

完整配置请参考仓库中的 `services/gateway/configs/gateway.example.yaml`。
