# AgentPay 官网（openagentpay.dev）中英文双轨叙事分流改造方案

> **版本**：v1.0.0  
> **状态**：**已落地实现 (Implemented)**  
> **涉及文件**：  
> - [`apps/marketplace-web/src/i18n/locales/zh.ts`](../../apps/marketplace-web/src/i18n/locales/zh.ts)  
> - [`apps/marketplace-web/src/components/HomePageView.tsx`](../../apps/marketplace-web/src/components/HomePageView.tsx)  
> - [`apps/marketplace-web/src/i18n/locales/en.ts`](../../apps/marketplace-web/src/i18n/locales/en.ts)（保持 Global Web3 原味不变）  

---

## 1. 背景与改造目标

`openagentpay.dev` 作为统一对外门户，同时面向**海外 Web3 / 跨组织 Agent 开发者**与**国内 AI 应用 / 企业级智能体团队**。此前中文版（`locale === 'zh'`）在首屏、指标卡与代码示例中完全沿用了英文版的 `Circle USDC`、`EIP-3009 0 Gas` 与 `Solana 聪明钱` 叙事，容易给国内开发者造成“必须持有加密钱包与 USDC 才能使用”的误解，掩盖了系统已具备的**租户级 `CNY`（人民币 6 位微单位）虚拟钱包与企业预算防火墙**能力。

本次改造在不拆分站点、不影响英文版（`EN`）海外叙事的前提下，通过 i18n 语言环境感知实现**“中英文双轨叙事分流”**。

---

## 2. 核心改造内容对照表

| 模块位置 | 英文版（`locale === 'en'`，面向海外） | 中文版（`locale === 'zh'`，面向国内与出海双栖） |
| :--- | :--- | :--- |
| **首屏主标题 (`heroTitle`)** | `AgentPay — Safe Micro-Payments for Autonomous AI Agents` | `AgentPay — 让 AI 自主调用付费工具，但绝不突破预算边界` |
| **首屏副标题 (`heroSubtitle`)** | 强调 `Private keys stay local, zero gas friction, pay-per-request` | 强调 **原生支持「企业多币种虚拟钱包（CNY/USD）」与「x402 链上零 Gas」双轨按次微支付**，杜绝刷爆账单 |
| **核心指标卡 1** | `Circle USDC (EIP-3009)` / **`0 Gas`** | `双轨微支付 (CNY / USDC)` / **`¥0.000001`**（支持 6 位微单位 CNY/USD 虚拟账本与零 Gas 链上结算） |
| **核心指标卡 2** | `Non-Custodial` / **`100% Local`** | `零信任安全边界` / **`100% 本地拦截`**（受限 API Key 租户隔离与本地私钥不出域双重保护） |
| **买方极速上手 (`Quickstart`)** | 默认选中 **`On-Chain (EIP-3009)`**，展示 `LocalSigner` + `USDC` 调用示例 | 默认选中 **`虚拟钱包 (CNY/API Key)`**，展示免区块链私钥的 `AGENTPAY_API_KEY` + `CNY` 预算防火墙调用示例，并提供按钮一键切换查看 **`链上签名 (EIP-3009)`** |
| **卖方配置示例 (`gateway.yaml`)** | 展示 `0.001 USDC` + `eip155:8453` 链上路由配置 | 展示 `allowed_payment_methods: "both"` 双轨配置（同时包含 `CNY` 虚拟钱包微单位定价 `350000` = `¥0.35` 与链上 `USDC` 定价） |
| **API 商店首屏 (`hero`)** | 徽章：`EIP-3009 Zero-Gas Signing` | 徽章：**`支持虚拟钱包 (CNY/USD) 与 EIP-3009 双通道`**；检索词优先展示 `DeepSeek 推理、网页 Markdown 抓取、金融财报` |
