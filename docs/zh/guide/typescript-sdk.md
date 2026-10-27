---
title: TypeScript SDK 指南
description: 使用 AgentPay TypeScript / Node.js SDK——基于原生 fetch 的客户端、本地 SQLite 金融防火墙、两种支付模式（API Key 钱包与链上 x402）、Agent 审计元数据、幂等重试，以及 LangChain.js、Vercel AI SDK 与 MCP 集成。
---

# TypeScript SDK 指南

`open-agentpay`（位于 `sdk/typescript/packages/agentpay`）是面向自主 AI Agent 与工具开发者的官方 TypeScript / Node.js SDK。它提供开箱即用的 HTTP 客户端与原生 `fetch` 拦截器，自动处理付费 API 的 402 握手与签名，同时由**本地金融防火墙**决定每一笔支付是否被允许。

::: warning 实验性项目
AgentPay 尚未经过审计，仅适用于测试网与沙箱环境。切勿向与其配合使用的密钥转入真实资金。
:::

## SDK 提供什么

| 能力 | 说明 |
| --- | --- |
| **`AgentPayClient` 与 `createAgentPayFetch`** | 基于原生 `fetch` 的 HTTP 客户端（`get`、`post`、`put`、`delete`、`request`），以及可无缝替换标准 `fetch` 的适配器（`client.asFetch()`）。 |
| **两种支付模式** | *价格感知钱包模式*（API Key、x402 风格的 `agentpay-wallet` 挑战、租户虚拟钱包）与*链上模式*（基于 `viem` 的本地签名器，x402 + EIP-3009）。 |
| **本地金融防火墙** | 基于 `better-sqlite3` 的本地策略引擎（单笔上限、24 小时滚动预算、终身额度、白名单矩阵、熔断器）；开启钱包挑战模式后，会在网关冻结余额前校验报价。 |
| **三态对账与线安全** | `open-agentpay/state` 中的 `BuyerCoordinator` 与 `Reconciler` 保证签名一旦发出，网络超时或上游 `5xx` 绝不提前释放预算预留。 |
| **审计元数据** | 可选请求头（`X-Session-ID`、`X-Task-ID`、`X-Tool-Name`），用于在控制台时间线中按会话与任务聚合 Agent 调用。 |
| **AI 框架集成** | 面向 LangChain.js 的 `createPaidTool`（`open-agentpay/langchain`）、面向 Vercel AI SDK 的 `paidVercelTool`（`open-agentpay/vercel-ai`），以及面向 MCP 的 `createPaidMcpToolHandler` / `createPaidMcpFetch`（`open-agentpay/mcp`）。 |

## 安装

需要 **Node.js 20.0+**。

```bash
# 从 npm 安装标准 SDK 包
npm install open-agentpay
# 或使用 pnpm / yarn
pnpm add open-agentpay
yarn add open-agentpay
```

如果在仓库根目录进行本地开发构建：

```bash
pnpm --filter open-agentpay build
```

---

## 模块化子路径导出 (Subpath Exports)

SDK 提供按需导出的子路径模块：

| 导入路径 | 核心导出内容 |
| --- | --- |
| `open-agentpay` | `AgentPayClient`、`createAgentPayFetch`、`LocalSigner` 以及全部核心、策略、状态与集成导出 |
| `open-agentpay/client` | `AgentPayClient`、`AgentPayTransport`、`createAgentPayFetch`、`LocalSigner` |
| `open-agentpay/policy` | `PolicyConfig`、`SQLitePolicyEngine`、`WhitelistMode`、`BreakerState`、`createBuyerConnection`、`initBuyerDatabase`、`PolicyViolationError`、`PolicyErrorCode` |
| `open-agentpay/core` | `PaymentChallenge`、`PaymentAuthorization`、`WalletAuthorization`、x402 头部编解码器、`X402Error` |
| `open-agentpay/evm` | EIP-712 / EIP-3009 类型化数据构建器、CAIP-2 网络常量 |
| `open-agentpay/state` | `BuyerCoordinator`、`Reconciler`、`IntentTransitionValidator` |
| `open-agentpay/langchain` | `createPaidTool`、`PaidStructuredTool`、`ToolException` |
| `open-agentpay/vercel-ai` | `paidVercelTool`、`createVercelPaidTool` |
| `open-agentpay/mcp` | `createPaidMcpToolHandler`、`createPaidMcpFetch` |

---

## 客户端初始化

```typescript
import { AgentPayClient, LocalSigner } from 'open-agentpay';
import { PolicyConfig, SQLitePolicyEngine } from 'open-agentpay/policy';

const client = new AgentPayClient({
  signer: new LocalSigner(process.env.AGENT_PRIVATE_KEY!), // 为链上授权签名
  policy: SQLitePolicyEngine.createInMemory(new PolicyConfig({ ... })), // 本地金融防火墙
  defaultHeaders: { ... },                                 // 默认请求头，如 Authorization
  walletChallengeMode: true,                               // API Key 钱包调用先报价、后重试
});
```

| 配置项 | 是否必填 | 作用 |
| --- | --- | --- |
| `signer` | 是 | 为**链上路径**签署 EIP-712 / EIP-3009 授权（底层使用 `viem`）。钱包模式不产生链上签名，可使用占位测试密钥。 |
| `policy` | 是 | 本地策略引擎（`IPolicyEngine`），在任何支付前执行评估。参见[策略配置](/zh/guide/policy)。 |
| `defaultHeaders` | 否 | 每次请求默认附带的请求头。`Authorization: Bearer <api_key>` 用于认证钱包请求。 |
| `baseUrl` | 否 | 可选的基础 URL，自动拼接相对请求路径。 |
| `walletChallengeMode` | 否 | 调用 AgentPay 网关钱包路由时设为 `true`。SDK 先获取 402 报价、评估本地策略，再携带 `PAYMENT-SIGNATURE` 重试；默认为 `false`。 |
| `raiseOnPolicyRejection` | 否 | 设为 `true` 时，若本地防火墙拒绝 402 挑战则直接抛出 `PolicyViolationError`；默认为 `false`（返回原始 402 `Response`）。 |
| `baseFetch` | 否 | 自定义底层 `fetch` 实现（默认使用 `globalThis.fetch`）。 |

### 标准 `fetch` 适配器

如果现有 SDK 或第三方库接收标准 `fetch` 函数，可使用 `client.asFetch()` 或 `createAgentPayFetch()`：

```typescript
import { createAgentPayFetch } from 'open-agentpay';

const paidFetch = client.asFetch();
// 或者：const paidFetch = createAgentPayFetch({ signer, policy });

const response = await paidFetch('https://gate.openagentpay.dev/v1/market-data?ticker=NVDA');
```

---

## 选择支付模式

支付模式由请求携带的凭证决定，模式之间**没有自动降级**。

| | **钱包模式** | **链上模式** |
| --- | --- | --- |
| **凭证** | `Authorization` 头中的 API Key | 本地私钥（`LocalSigner`） |
| **是否需要账号** | 需要：在[控制台](/zh/guide/console)创建租户、项目、Agent 与密钥 | 不需要 |
| **支付方式** | 网关返回 `agentpay-wallet` 价格挑战；SDK 策略批准后才进行钱包 Hold → 上游 → Capture | 客户端签署 EIP-3009 授权，由 Facilitator 上链结算 |
| **延迟 / 成本** | 内部账本，无 Gas | 需等待网络确认，Gas 由 Facilitator 代付 |
| **适用场景** | 同一组织内的 Agent 集群 | 跨组织调用、无事先合作关系 |

::: tip
钱包模式请求余额不足时，会收到 `402 ERR_WALLET_INSUFFICIENT_BALANCE`。SDK **不会**自动改走链上，切换模式始终是你在代码中显式做出的决定。
:::

### 价格感知钱包模式（API Key）

使用 AgentPay 网关 API Key 时，请开启 `walletChallengeMode: true`。网关首次返回包含确切路由价格的 `402`，此时不会冻结钱包余额，也不会调用上游。SDK 校验 `requestBinding` 与 `resourceUri` 后，先用本地策略检查报价并预留本地预算，通过后再携带一次性 `PAYMENT-SIGNATURE` 钱包授权重试。

```typescript
import { AgentPayClient, LocalSigner } from 'open-agentpay';
import { PolicyConfig, SQLitePolicyEngine } from 'open-agentpay/policy';

// 钱包模式不产生链上签名，因此 signer 只是占位。
// ⚠️ 仅限测试网开发：此私钥为 Hardhat/Anvil 默认公开测试密钥，切勿转入任何主网真实资产！
const devKey =
  process.env.AGENT_PRIVATE_KEY ??
  '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';
const signer = new LocalSigner(devKey);

const policy = SQLitePolicyEngine.createInMemory(
  new PolicyConfig({
    id: 'wallet_firewall',
    name: 'Wallet Firewall',
    walletAddress: signer.address,
    maxPerTxBaseUnits: 100_000n,         // 单笔上限 0.10 USD（钱包最小单位）
    rollingBudget24hBaseUnits: 5_000_000n, // 滚动 24 小时上限 5.00 USD
    whitelistDomains: ['gateway.example.com'],
  })
);

const client = new AgentPayClient({
  signer,
  policy,
  defaultHeaders: {
    Authorization: `Bearer ${process.env.AGENTPAY_API_KEY}`,
  },
  walletChallengeMode: true,
});

const resp = await client.get('https://gateway.example.com/v1/market-data', {
  params: { ticker: 'NVDA' },
});
console.log(resp.status, await resp.json());
```

### 链上模式（x402）

```typescript
import { AgentPayClient, LocalSigner } from 'open-agentpay';
import { PolicyConfig, SQLitePolicyEngine, WhitelistMode } from 'open-agentpay/policy';

const signer = new LocalSigner(process.env.AGENT_WALLET_KEY!);
const policy = SQLitePolicyEngine.createInMemory(
  new PolicyConfig({
    id: 'onchain_firewall',
    name: 'On-Chain Firewall',
    walletAddress: signer.address,
    whitelistMode: WhitelistMode.SIMPLE,
    maxPerTxBaseUnits: 1_000_000n,          // 1.00 USDC
    rollingBudget24hBaseUnits: 10_000_000n, // 10.00 USDC
    whitelistPayees: ['0x70997970C51812dc3A010C7d01b50e0d17dc79C8'],
  })
);

const client = new AgentPayClient({ signer, policy }); // 无需 API Key

const resp = await client.get('https://gate.openagentpay.dev/v1/solana/smart-money', {
  params: { timeframe: '24h' },
});
console.log(await resp.json());
```

当服务端返回携带 `PAYMENT-REQUIRED` 挑战的 `402` 时，客户端会透明地完成以下流程：

```text
1. 解码挑战（价格、收款人、资产、网络、有效期、scheme）
2. 策略：评估 → 在 SQLite 中原子地 RESERVE（预留）预算（此处被拒 = 不会发出授权）
3. 按 scheme 选择：使用 viem 签署 EIP-3009，或直接生成 `agentpay-wallet` 授权
4. 携带 `PAYMENT-SIGNATURE` 头重放原请求
5. 2xx 成功后 COMMIT 本地预留；超时或 `5xx` 等结果不明确时保留预留
```

::: warning 私钥安全与自动脱敏
`LocalSigner` 已重写 `toString()`、`toJSON()` 与 `nodejs.util.inspect.custom` 对私钥自动脱敏。请始终从环境变量或密钥管理服务加载私钥，切勿写在源码或提示词中，切勿将签名器或策略对象作为工具暴露给大模型。
:::

---

## 本地金融防火墙

`policy` 参数接收基于 `better-sqlite3` 的 `SQLitePolicyEngine`，支持内存与持久化文件两种存储模式：

| 模式 | 状态 | 适用场景 |
| --- | --- | --- |
| `SQLitePolicyEngine.createInMemory(config)` | 内存（`:memory:`，重启后丢失） | 测试、演示、短生命周期的 Agent |
| `new SQLitePolicyEngine(db, config)` | 持久化 SQLite 文件（开启 WAL 模式与 `BEGIN IMMEDIATE` 独占事务原子预留） | 长期运行的 Agent，以及预算必须在重启后保留的场景 |

### 持久化文件账本示例

```typescript
import {
  createBuyerConnection,
  initBuyerDatabase,
  PolicyConfig,
  SQLitePolicyEngine,
  WhitelistMode,
} from 'open-agentpay/policy';

const db = createBuyerConnection('./agentpay-buyer.sqlite');
initBuyerDatabase(db);

const policy = new SQLitePolicyEngine(
  db,
  new PolicyConfig({
    id: 'agent_firewall',
    name: 'Agent Firewall',
    walletAddress: signer.address,
    maxPerTxBaseUnits: 500_000n,          // 0.50 USDC
    rollingBudget24hBaseUnits: 5_000_000n, // 5.00 USDC
    lifetimeBudgetBaseUnits: 50_000_000n,  // 可选终身总预算上限 (50.00 USDC)
    circuitMaxRpm: 15,
    circuitConsecutiveFailures: 5,
    circuitCooldownSeconds: 300,
    whitelistMode: WhitelistMode.SIMPLE,
    whitelistPayees: ['0x742d35Cc6634C0532925a3b844Bc454e4438f44e'],
  })
);
```

所有金额均为**最小单位整数**（支持 `bigint`、安全整数 `number` 或纯数字 `string`；传入浮点数或布尔值会直接抛出 `TypeError`）。USDC 为 6 位小数，`500_000n` 即 0.50 USDC。全部规则见[策略配置](/zh/guide/policy)。

---

## Agent 身份与审计元数据

网关根据 API Key 解析 `agent_id`，并自动剥离客户端发送的任何 `X-Agent-ID` 头。你可以添加可选的自报上下文请求头，用于在控制台时间线中聚合调用：

| 请求头 | 用途 |
| --- | --- |
| `X-Session-ID` | 聚合属于同一工作流会话的调用 |
| `X-Task-ID` | 聚合属于同一任务的调用 |
| `X-Tool-Name` | 发起调用的工具名称 |

```typescript
const client = new AgentPayClient({
  signer,
  policy,
  defaultHeaders: {
    Authorization: `Bearer ${apiKey}`,
    'X-Session-ID': 'session_nvda_analysis_001',
    'X-Task-ID': 'task_market_data_fetch',
    'X-Tool-Name': 'fetch_stock_quote',
  },
  walletChallengeMode: true,
});
```

---

## 幂等与安全重试

1. **对任何会产生费用的请求发送 `Idempotency-Key`**，并在重试同一逻辑请求时复用**同一个 Key**。
2. 相同 Key 且请求一致时，网关会回放已存结果（`X-Cache: HIT`），**不产生额外费用**。
3. **超时不等于失败。** 支付签名一旦在网络上发出，若遇到超时或上游 `5xx`，客户端**绝不会**提前释放本地预算预留。可通过 `open-agentpay/state` 中的 `Reconciler` 在授权过期且确认链上未消费后安全释放。

```typescript
import { randomUUID } from 'node:crypto';

const idempotencyKey = randomUUID();

const resp = await client.post(
  'https://gateway.example.com/v1/report',
  { ticker: 'NVDA' },
  { headers: { 'Idempotency-Key': idempotencyKey } }
);
```

---

## 错误处理

默认情况下（`raiseOnPolicyRejection: false`），本地策略拒绝会直接返回未签名的 `402` 响应；若希望在本地策略拦截时直接抛出异常，可设置 `raiseOnPolicyRejection: true` 并捕获 `PolicyViolationError`：

```typescript
import { PolicyViolationError } from 'open-agentpay/policy';

try {
  const resp = await client.get('https://gateway.example.com/v1/market-data');
  if (resp.status === 200) {
    const data = await resp.json();
  } else if (resp.status === 402) {
    console.error('支付被拒绝或挑战未通过:', await resp.text());
  } else if (resp.status === 403 || resp.status === 429) {
    console.error('被网关策略拦截:', await resp.json());
  }
} catch (err) {
  if (err instanceof PolicyViolationError) {
    console.error(`被本地防火墙拦截 [${err.code}]:`, err.message);
  } else {
    throw err;
  }
}
```

---

## AI 框架集成

### LangChain.js (`open-agentpay/langchain`)

使用 `createPaidTool`（或 `PaidStructuredTool`）包装工具函数。策略拦截与协议错误会自动转换为 `ToolException`，方便 Agent 感知原因并安全终止，而不会导致进程崩溃：

```typescript
import { createPaidTool } from 'open-agentpay/langchain';

const fetchStockQuote = createPaidTool({
  name: 'fetch_stock_quote',
  description: '获取实时美股行情（按次扣费）',
  client,
  func: async ({ ticker }: { ticker: string }, context) => {
    const resp = await context!.client!.get('https://gateway.example.com/v1/market-data', {
      params: { ticker },
    });
    return await resp.json();
  },
});

const result = await fetchStockQuote.invoke({ ticker: 'NVDA' });
```

### Vercel AI SDK (`open-agentpay/vercel-ai`)

使用 `paidVercelTool` 封装 Vercel AI SDK 工具。当本地防火墙拦截调用时，会向模型返回结构化的 `{ error: true, code, message }` 对象：

```typescript
import { paidVercelTool } from 'open-agentpay/vercel-ai';
import { z } from 'zod';

const marketTool = paidVercelTool({
  description: '获取受财务防火墙保护的 DEX 市场交易量数据',
  parameters: z.object({ chain: z.string() }),
  client,
  execute: async ({ chain }, context) => {
    const res = await context!.client!.get(
      'https://gate.openagentpay.dev/v1/free/defi/dex-volume',
      { params: { chain } }
    );
    return await res.json();
  },
});
```

### Model Context Protocol (`open-agentpay/mcp`)

在 MCP Server 中使用 `createPaidMcpToolHandler`，或在 MCP Client 传输层使用 `createPaidMcpFetch`：

```typescript
import { createPaidMcpFetch, createPaidMcpToolHandler } from 'open-agentpay/mcp';

// 1. 包装 MCP Server 工具处理函数（策略拦截时自动返回 isError: true 的 McpToolResponse）
const mcpHandler = createPaidMcpToolHandler(client, async (args: { ticker: string }, context) => {
  const res = await context!.client.get('https://gateway.example.com/v1/market-data', {
    params: { ticker: args.ticker },
  });
  return await res.json();
});

// 2. 或为 MCP Client 传输层提供原生支持 402 的 fetch 函数
const mcpFetch = createPaidMcpFetch(client);
```

---

## 故障排查

| 现象 | 可能原因 |
| --- | --- |
| `PolicyViolationError: UNMETERED_CREDENTIAL` | 请求携带了 `apk_...` API Key，但未开启 `walletChallengeMode: true` 或未设置 `unmeteredCredentialPolicy`。调用网关钱包路由时请开启 `walletChallengeMode: true`。 |
| `TypeError: maxPerTxBaseUnits must be an integer, got 0.05` | 金额必须为最小单位整数（如 `50_000n` 或 `50000` 表示 0.05 USDC），严禁使用浮点数。 |
| `InvalidPrivateKeyError` | 私钥必须是 32 字节 Hex 字符串（可带或不带 `0x` 前缀）或 32 字节 `Uint8Array`。 |
| 所有调用都被 `SINGLE_TX_LIMIT_EXCEEDED` 拦截 | `maxPerTxBaseUnits` 低于路由单价。 |
| 进程重启后预算被重置 | `SQLitePolicyEngine.createInMemory` 使用 `:memory:`；需要持久化时请使用 `createBuyerConnection('./buyer.sqlite')` + `initBuyerDatabase(db)`。 |

## 下一步

| 我想… | 阅读 |
| --- | --- |
| 在本地跑通全流程 | [5 分钟快速开始](/zh/guide/quickstart) |
| 使用 Python SDK | [Python SDK 指南](/zh/guide/python-sdk) |
| 设置花费限额 | [策略配置](/zh/guide/policy) |
| 为 API 加上价格 | [网关配置](/zh/guide/gateway) |
| 创建 Agent 与 API Key | [管理控制台](/zh/guide/console) |
| 查看线上协议 | [API 规范](/zh/guide/api-specs) |
