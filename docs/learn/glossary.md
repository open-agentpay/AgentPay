---
title: 术语表
description: AgentPay 与 AI Agent 支付领域的关键术语定义与对照。
---

# 术语表

### Agent API Key
租户签发给特定 Agent 实例的鉴权凭据，格式统一为 `apk_live_<hex>`，网关根据此 Key 检索关联的钱包、策略与租户边界。

### Financial Firewall (金融防火墙)
部署在 Agent 与付费网络服务之间的受控代理层，执行单笔限额、24小时预算、收款方白名单以及异常熔断。

### Hold (资金冻结)
预付钱包模式下，网关在转发上游请求前对账户可用余额进行的原子锁定。如果请求成功则执行 Capture（确认扣费），若上游明确失败则执行 Release（退回可用余额）。

### Hold TTL (冻结超时)
资金被锁定处于冻结状态的最大有效时长。AgentPay 默认设定为 `upstream_timeout + 60s`（默认 120 秒）。

### Capture (扣除冻结)
在上游服务确认履约交付后，网关将已冻结的金额转为实际支出，不可回滚。

### x402 协议
基于 HTTP 402 Payment Required 扩展的机器可读开放支付协议标准。

### Facilitator
链上结算路径中负责构建、签名、广播区块链交易并监听收据的异步服务组件。
