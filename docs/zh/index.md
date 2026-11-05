---
layout: home

hero:
  name: "AgentPay"
  text: "企业级 AI 行为审计与资金防火墙"
  tagline: "AI Agent Gateway & FinOps — 让 AI 自主调用付费工具与商业模型，但绝不突破企业预算边界。死循环秒级熔断、飞书卡片审批、等保行为审计、部门水表对账。"
  image:
    src: /logo.png
    alt: AgentPay Logo
  actions:
    - theme: brand
      text: 5 分钟快速上手
      link: /zh/guide/quickstart
    - theme: alt
      text: 架构与规范
      link: /zh/specs/
    - theme: alt
      text: GitHub 源码
      link: https://github.com/open-agentpay/AgentPay

features:
  - title: 🛡️ 防 · 资金防火墙与死循环熔断
    details: 单笔限额、24 小时滚动预算与 RPM 突发流量熔断。当 Agent 陷入死循环递归或遭遇 Prompt 注入盗刷时，网关第 3 秒自动拉下安全电闸，物理掐断网络连接，杜绝天价账单。
  - title: 📱 控 · 飞书/企微审批 (人在回路)
    details: 三级风控梯度（小额自动放行 / 中大额人工审批 / 超大额硬顶阻断）。超额请求触发 x402 异步挂起，自动向负责人的飞书、企业微信、钉钉推送审批卡片，手机一键放行。
  - title: 👁️ 审 · 不可篡改审计黑匣子
    details: 贯彻严格的网关客观事实（Observed）与自报声明（Declared）物理隔离。只追加（Append-Only）审计流水，支持哈希链防篡改指纹与一键断电（Kill Switch），满足等保 2.0 审计。
  - title: 📊 算 · 企业 AI FinOps 财务水表
    details: 租户 → 部门 → 项目 → Agent 树状配额水表。人民币（CNY）6 位微单位虚拟账本（1 元 = 1,000,000 微单位），支持微信/支付宝充值、对公银行汇款核销与增值税专票导出。
  - title: 🤝 联 · 原生集成 Langfuse 可观测性
    details: 打造“AI 运维双子星”——开源 MIT 的 Langfuse 负责看脑子（Prompt/Token/评测），AgentPay 负责管双手与钱包（物理防火墙/工具扣费/审批），Trace ID 全链路双向回写。
  - title: ⚡ 轨 · 双轨微支付与三态解耦
    details: 国内主推零牌照风险的虚拟账本与持牌机构空中分账；出海场景自适应切换海外 x402 链上零 Gas 结算（EIP-3009）。三态状态机确保网络超时绝不误扣款。
---

::: tip 企业级 AI 安全底座
AgentPay 不仅支持单机部署体验，更提供面向生产环境的高性能 Go 反向代理网关、Vue 3 多租户管控台与开箱即用的 Docker 容器编排套件。
:::
