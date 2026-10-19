# AgentPay 接口规格文档 (API Specifications)

## 1. x402 V2 HTTP 协议头规范

AgentPay 在 HTTP 应用层采用标准的 `402 Payment Required` 状态码与结构化自定义头部进行无缝交互。

### 1.1 `PAYMENT-REQUIRED` 响应头 (服务端返回)

当客户端未携带有效支付凭据访问受保护的端点时，服务端返回 `HTTP 402` 并在 `PAYMENT-REQUIRED` 响应头中携带 Base64URL 编码的 JSON 挑战载荷。

#### JSON 载荷 Schema:
```json
{
  "version": "2.0",
  "challengeId": "ch_01j7xyz...",
  "network": "eip155:84532",
  "asset": "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
  "assetSymbol": "USDC",
  "decimals": 6,
  "amount": "100000",
  "payee": "0xSellerPayoutAddress...",
  "scheme": "EIP-3009",
  "validAfter": 1729000000,
  "validBefore": 1729000300,
  "facilitatorUrl": "https://facilitator.agentpay.dev",
  "resourceUri": "/v1/market-data"
}
```

#### 字段详细说明:
| 字段 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `version` | string | 是 | 协议版本，固定为 `"2.0"`。 |
| `challengeId` | string | 是 | 本次支付挑战的全局唯一 ID (UUIDv7/CUID)。 |
| `network` | string | 是 | CAIP-2 格式的区块链网络 ID（如 `eip155:84532` 为 Base Sepolia）。 |
| `asset` | string | 是 | 支付资产的合约地址（EVM 0x 格式）。 |
| `assetSymbol` | string | 否 | 代币资产标识符号，便于展示与审计（如 `"USDC"`）。 |
| `decimals` | number | 是 | 资产精度位数（USDC 通常为 6）。 |
| `amount` | string | 是 | 需支付的最小基数单位（字符串避免精度丢失，如 `"100000"` 表示 0.1 USDC）。 |
| `payee` | string | 是 | 卖方收款的链上地址。 |
| `scheme` | string | 是 | 支付签名模式：`"EIP-3009"` 或 `"EIP-712"`。 |
| `validAfter` | number | 是 | 授权生效的 Unix 时间戳（秒）。 |
| `validBefore` | number | 是 | 授权失效的 Unix 时间戳（秒，通常有效窗口为 3~5 分钟）。 |
| `facilitatorUrl`| string | 是 | 推荐/指定的 Facilitator 清算节点地址。 |
| `resourceUri` | string | 否 | 被保护的目标资源路径。 |

---

### 1.2 `PAYMENT-SIGNATURE` 请求头 (买方客户端重放)

买方 Agent 在完成策略评估并生成 EIP-712/3009 授权签名后，将凭据 Base64URL 编码后放入请求的 `PAYMENT-SIGNATURE` 头部。

#### JSON 载荷 Schema:
```json
{
  "version": "2.0",
  "challengeId": "ch_01j7xyz...",
  "payer": "0xBuyerAgentAddress...",
  "scheme": "EIP-3009",
  "nonce": "0x4a9b...32bytesHex",
  "validAfter": 1729000000,
  "validBefore": 1729000300,
  "signature": {
    "v": 27,
    "r": "0x6f2b...",
    "s": "0x1a8c...",
    "compact": "0x6f2b...1a8c...1b"
  }
}
```

---

## 2. Facilitator 协调清算服务 REST API

Facilitator 作为无 Gas 清算的验证与中继枢纽，提供以下核心接口。

### 2.1 签名在线/离线验证接口

卖方中间件收到买方携带的 `PAYMENT-SIGNATURE` 后，调用该接口核验签名的真实性与未花费状态。

* **Endpoint**: `POST /v1/verify`
* **Content-Type**: `application/json`

#### 请求参数 (Request Body):
```json
{
  "challenge": {
    "challengeId": "ch_01j7xyz...",
    "network": "eip155:84532",
    "asset": "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
    "amount": "100000",
    "payee": "0xSellerPayoutAddress...",
    "scheme": "EIP-3009"
  },
  "authorization": {
    "challengeId": "ch_01j7xyz...",
    "payer": "0xBuyerAgentAddress...",
    "nonce": "0x4a9b...32bytesHex",
    "validAfter": 1729000000,
    "validBefore": 1729000300,
    "signature": "0xcompactSignature..."
  }
}
```

#### 成功响应 (HTTP 200 OK):
```json
{
  "valid": true,
  "verifiedPayer": "0xBuyerAgentAddress...",
  "verifiedAt": 1729000010,
  "verificationToken": "vtok_01j7abc..."
}
```

#### 常见失败状态码:
* `400 Bad Request`：参数缺失或格式非法。
* `401 Unauthorized`：EIP-712 签名反解出的地址与 `payer` 不符。
* `409 Conflict`：该 `nonce` 已被提交或消费过（防重放触发）。
* `410 Gone`：当前时间已超过 `validBefore` 截止期。

---

### 2.2 提交链上结算接口

卖方在确认资源交付成功后，异步将签名提交至 Facilitator 请求上链。

* **Endpoint**: `POST /v1/settle`
* **Content-Type**: `application/json`

#### 请求参数 (Request Body):
```json
{
  "verificationToken": "vtok_01j7abc...",
  "authorization": { ... },
  "idempotencyKey": "idem_01j7def..."
}
```

#### 响应 (HTTP 202 Accepted):
```json
{
  "settlementId": "stl_01j7xyz...",
  "status": "SUBMITTED",
  "queuePosition": 4
}
```

---

## 3. SDK 客户端与中间件 API 契约

### 3.1 买方客户端 `AgentPayClient`
```typescript
interface AgentPayClientOptions {
  signer: ISigner;               // 签名器实例 (LocalSigner / KMSSigner)
  policy: IPolicyEngine;         // 策略引擎实例 (Memory / PostgreSQL)
  timeoutMs?: number;            // 超时时间 (默认 10000ms)
  autoRetryOn402?: boolean;      // 是否在 402 时透明签名并重试 (默认 true)
}

class AgentPayClient {
  constructor(options: AgentPayClientOptions);

  // 包装的原生 fetch 方法
  fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response>;

  // 快捷方法
  get<T = any>(url: string, init?: RequestInit): Promise<T>;
  post<T = any>(url: string, body?: any, init?: RequestInit): Promise<T>;
}
```

### 3.2 卖方中间件 `agentPayMiddleware` (Express / Fastify)
```typescript
interface AgentPayMiddlewareOptions {
  price: bigint;                 // 每次调用的最小单位金额
  payee: string;                 // 收款方链上地址
  asset: string;                 // 资产合约地址
  network: string;               // CAIP-2 网络标识
  facilitatorUrl: string;        // 校验 Facilitator 地址
  validitySeconds?: number;      // 挑战有效时长 (默认 300 秒)
}
```

---

### 3.3 Python SDK 客户端与服务端契约 (`agentpay-python`)

`AgentPayClient` 继承自标准 `httpx.Client`，支持两种主流支付模式：
1. **模式一：Web3 / 链上小额支付模式 (On-Chain x402)** —— 无需账号与 APIKey，利用本地私钥对 HTTP 402 挑战进行 EIP-712/3009 离线签名；
2. **模式二：企业虚拟钱包 / 平台网关模式 (Enterprise Virtual Wallet / API Key)** —— 面向企业级微秒级结算，携带控制台颁发的 APIKey，联动租户虚拟钱包余额并自动注入 T1.8 行为审计头。

#### 3.3.1 模式一：Web3 / 链上小额支付 (On-Chain x402)
```python
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

# 初始化签名器与策略防火墙 (免 APIKey)
signer = LocalSigner(private_key="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
policy = MemoryPolicyEngine(
    max_per_tx=1_000_000,      # 1 USDC
    daily_budget=10_000_000,   # 10 USDC
    whitelist_payees=["0xSellerAddress..."]
)

client = AgentPayClient(signer=signer, policy=policy)

# 1. 原生风格调用 (自动截获 402、本地策略校验、EIP-712 签名并重放)
response = client.get("https://gateway.example.com/v1/market-data")
data = response.json()

# 2. 异步客户端集成
async with client.async_client() as aclient:
    res = await aclient.get("https://gateway.example.com/v1/market-data")
```

#### 3.3.2 模式二：企业虚拟钱包 / 平台网关模式 (Enterprise Virtual Wallet / Platform API Key)
在控制台（T4.7）创建 Agent 并获取 API Key 后，通过 `headers` 参数注入。网关自动从租户虚拟钱包扣款，并记录全链路审计事实：

```python
import os
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine
from agentpay.integrations.langchain import paid_tool

# 1. 初始化本地财务防火墙（防 Agent 内部循环调用刷爆企业预算）
policy = MemoryPolicyEngine(
    max_per_tx=100_000,        # 单笔上限 $0.10 USDC
    daily_budget=5_000_000,    # 24小时上限 $5.00 USDC
)
# 钱包模式下 signer 可为占位或从本地获取
signer = LocalSigner("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")

# 2. 获取控制台颁发给该 Agent 的 API Key
api_key = os.getenv("AGENTPAY_API_KEY", "apk_live_xxxxxxxxxxxxxxxxxxxx")

# 3. 创建客户端：配置 APIKey 与 T1.8 规范审计跟踪请求头
client = AgentPayClient(
    signer=signer,
    policy=policy,
    headers={
        # 认证凭证：网关据此识别 Agent 身份并扣除对应租户钱包余额
        "Authorization": f"Bearer {api_key}",
        # 或使用自定义头: "X-API-Key": api_key,

        # T1.8 规范自报元数据 (用于控制台按工作流会话聚合审计时间线)
        "X-Session-ID": "session_nvda_analysis_001",
        "X-Task-ID": "task_market_data_fetch",
        "X-Tool-Name": "fetch_stock_quote",
    }
)

# 4. LangChain / CrewAI 付费工具无感接入
@paid_tool(name="fetch_stock_quote", description="获取实时美股行情", client=client)
def fetch_stock_quote(ticker: str) -> dict:
    resp = client.get(f"https://gateway.example.com/v1/market-data?ticker={ticker}")
    return resp.json()

result = fetch_stock_quote.invoke({"ticker": "NVDA"})
```

> **安全与网络设计说明**：
> - **上游凭据隔离**：被调用的上游服务真实 API Key 由 AgentPay 网关安全托管，客户端代码绝不接触上游 Key；
> - **两阶段资金安全**：网关在数据面原子执行 `Hold -> 转发上游 -> Capture`，上游失败立即释放冻结，企业资金零损失；
> - **秒级停用 (Kill Switch)**：若 Agent 出现异常，在控制台一键停用 Agent 或撤销 API Key，网关毫秒级阻断后续所有请求。

##### 自报审计元数据（Declared Metadata）传递与缺省处理对照

客户端请求头中的 `X-Session-ID`、`X-Task-ID`、`X-Tool-Name` 等属于 **T1.8 规范自报元数据 (Declared Context)**。它们是 **100% 可选的**，系统针对传与不传的处理对比如下：

| 对比维度 | 传了自报 Header (`X-Session-ID` 等) | 未传自报 Header (缺省情况) |
| :--- | :--- | :--- |
| **网关请求处理** | 网关执行白名单过滤（防注入，限长 128 字符）后记录 | 网关提取为 `""`，**正常放行并转发上游，绝不报错或拦截** |
| **身份与计费判定** | **严禁参与计费**，网关只以 API Key 解析的 AgentID 为唯一事实 | **完全不受影响**，按 API Key 绑定的租户钱包正常原子扣款 |
| **风控与熔断** | 单笔限额、24h 预算、Kill Switch 由网关内部计数严格执行 | **完全不受影响**，风控逻辑严格在请求路径内同步执行 |
| **控制台会话聚合** | 下拉框可按 `Session ID` 筛选，将同一工作流的多次调用折叠聚合 | 事件归入“全部工作流会话”，按独立原子请求平铺展示 |
| **时间线徽标展示** | 同时展示绿色 `[网关观测]` 徽标与 **黄色 `[自报会话]` / `[自报任务]`** | **仅展示绿色 `[网关观测]` 徽标**（HTTP 状态、耗时、扣费、策略决策） |
| **审计穿透详情** | 穿透弹窗展示关联会话名、任务名与工具名 | 穿透弹窗展示 `PaymentIntent`、`Hold`、`Settlement` 与指纹哈希 |

**极简免配置代码示例（完全省略自报头，仅保留认证）：**
```python
# 极简模式：仅配一行 Authorization，业务完全正常，扣费与风控完全生效
client = AgentPayClient(
    policy=policy,
    signer=signer,
    headers={"Authorization": f"Bearer {api_key}"}
)
```

#### 3.3.3 LangChain / CrewAI 付费工具包装器 (`agentpay.integrations.langchain`)
```python
from langchain.tools import tool
from agentpay.integrations.langchain import paid_tool

@paid_tool(
    name="get_stock_quote",
    description="获取实时美股行情数据 (按次扣费)",
    client=client
)
def get_stock_quote(ticker: str) -> dict:
    return client.get(f"https://data.market.com/v1/quote?symbol={ticker}").json()
```

#### 3.3.3 FastAPI 卖方中间件 (`agentpay.fastapi`)
```python
from fastapi import FastAPI, Depends
from agentpay.fastapi import requires_payment

app = FastAPI()

@app.get("/v1/market-data")
@requires_payment(
    price=100_000,                                         # 0.1 USDC
    payee="0xSellerPayoutAddress...",
    asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",    # Base Sepolia USDC
    network="eip155:84532",
    facilitator_url="https://facilitator.agentpay.dev"
)
async def get_market_data():
    return {"status": "success", "data": "Alpha Market Data"}
```

