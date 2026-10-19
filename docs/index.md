---
layout: home

hero:
  name: "AgentPay"
  text: "AI Agent 金融防火墙与自主支付网关"
  tagline: "面向自主 AI Agent 的企业级开源支付基础设施与安全控制网关，基于 HTTP 402 与平台内部虚拟钱包双轨架构。"
  actions:
    - theme: brand
      text: 快速开始 (指南)
      link: /guide/overview
    - theme: alt
      text: 核心规范 (Specs)
      link: /specs/state-machine
    - theme: alt
      text: 研发任务清单 (v1.1)
      link: /planning/tasks-v1.1

features:
  - title: 🛡️ 独立财务防火墙
    details: LLM 只能提议付款，永远无法私自批准。单笔上限、24h 滚动窗口预算、白名单与调用频次熔断全面受控。
  - title: ⚡ 双轨清算架构 (双引擎)
    details: 支持区块链原生 x402 结算与平台内部虚拟钱包强事务扣费（Hold/Capture/Release），极速毫秒级响应，零 Gas 损耗。
  - title: 🔄 三态正交解耦状态机
    details: 支付意图、链上结算与上游履约严格三态分离。超时与异常绝不直接判为失败，幂等对账，杜绝重复扣款。
  - title: 🏢 多租户企业级管控
    details: 租户、项目、成员 RBAC 与 API Key 生命周期隔离。配置版本草稿发布与秒级无感回滚。
  - title: 📊 只追加不可变账本
    details: 严禁修改历史流水，全量操作与扣费均可追溯，五重自动对账平账，满足企业财务合规审计。
  - title: 🚀 Go 云原生高性能数据面
    details: 底层采用 Go 高并发反向代理与连接复用，数据底座支持 PostgreSQL 15+ 行级锁与高可用集群。
---
