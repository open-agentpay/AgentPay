---
layout: home

hero:
  name: "AgentPay"
  text: "AI Agent 的金融防火墙与自主支付网关"
  tagline: 让 AI 可以自主花钱，但不能突破资金权限。基于 HTTP 402 / x402 与平台虚拟钱包的双轨支付。
  image:
    src: /logo.png
    alt: AgentPay Logo
  actions:
    - theme: brand
      text: 5 分钟快速开始
      link: /zh/guide/quickstart
    - theme: alt
      text: 了解 Agent 支付
      link: /zh/learn/
    - theme: alt
      text: 核心规范
      link: /zh/specs/
    - theme: alt
      text: GitHub
      link: https://github.com/open-agentpay/AgentPay

features:
  - title: 🛡️ 独立财务防火墙
    details: LLM 只能提议付款，不能自行批准。单笔上限、24 小时滚动预算、白名单、频率熔断与循环检测，全部在请求路径内同步执行。
  - title: ⚡ 双轨支付
    details: 链上 x402（EIP-3009 离线签名，买方无需持有 Gas）与平台虚拟钱包（冻结 → 履约 → 确认扣款/释放）。余额不足绝不静默降级到链上。
  - title: 🔄 三态正交状态机
    details: 支付意图、结算、履约严格分离。超时与异常不直接判为失败，幂等对账，避免重复扣款。
  - title: 🏢 多租户管控
    details: 租户、项目、成员 RBAC 与 API Key 生命周期隔离；配置草稿、不可变快照发布与秒级回滚。
  - title: 👁️ Agent 身份与行为审计
    details: 身份只由凭据解析，观测与自报字段严格分离；行为时间线、一键停用（Kill Switch）与只追加审计。
  - title: 📒 只追加账本
    details: 金额一律整数最小单位；流水不可修改，多重对账平账，可追溯、可导出。
---

::: warning 实验性项目
AgentPay 处于早期阶段，**尚未经过独立安全审计**，请仅在测试网与测试代币上评估，不要用于生产资金。
:::
