---
title: Python SDK 指南
description: 使用 AgentPay Python SDK——基于 httpx 的客户端、本地金融防火墙、两种支付模式（API Key 钱包与链上 x402）、Agent 审计元数据、幂等重试、LangChain 付费工具与 FastAPI 卖方中间件。
---

# Python SDK 指南

`agentpay` 是面向 AI Agent 与 API 提供方的 Python SDK。它为你的 Agent 提供一个开箱即用的 HTTP 客户端，自动处理付费 API，同时由**本地金融防火墙**决定每一笔支付是否被允许。

::: warning 实验性项目
AgentPay 尚未经过审计，仅适用于测试网与沙箱环境。切勿向与其配合使用的密钥转入真实资金。
:::

## SDK 提供什么

| 能力 | 说明 |
| --- | --- |
| **`AgentPayClient`** | `httpx.Client` 的子类，用法与 `httpx` 完全一致：`get`、`post`、超时、请求头等。 |
| **两种支付模式** | *钱包模式*（API Key，租户虚拟钱包）与*链上模式*（本地签名器，x402 + EIP-3009）。 |
| **本地金融防火墙** | 策略引擎（单笔上限、滚动预算、白名单、熔断器），在任何支付发生前先行评估。 |
| **审计元数据** | 可选请求头，用于在控制台时间线中按会话、任务聚合 Agent 的调用。 |
| **集成** | 面向 LangChain / CrewAI 的 `paid_tool`，以及 FastAPI 卖方中间件 `requires_payment`。 |

同时支持同步与异步用法（见[同步与异步](#同步与异步)）。

## 安装

需要 **Python 3.10+**。

```bash
# 从 PyPI 安装标准 SDK 包
pip install open-agentpay

# 或在仓库根目录以可编辑模式本地安装进行开发
pip install -e python/packages/agentpay
```

---

## 客户端

```python
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

client = AgentPayClient(
    signer=LocalSigner(...),          # 为链上授权签名
    policy=MemoryPolicyEngine(...),   # 本地金融防火墙
    headers={...},                    # 默认请求头，例如 Authorization
)
```

| 参数 | 是否必填 | 作用 |
| --- | --- | --- |
| `signer` | 是 | 为**链上路径**签署 EIP-712 / EIP-3009 授权。钱包模式不会产生签名，可以只是占位。 |
| `policy` | 是 | 本地策略引擎，在任何支付之前评估。参见[策略配置](/zh/guide/policy)。 |
| `headers` | 否 | 默认请求头。`Authorization: Bearer <api_key>` 用于选择**钱包模式**。 |

由于 `AgentPayClient` 继承自 `httpx.Client`，`httpx` 的超时等标准选项均可照常使用。

<!-- TODO(author): 请确认还有哪些额外关键字参数（timeout、transport、base_url 等）会透传给 httpx.Client。 -->

---

## 选择支付模式

支付模式由请求携带的凭证决定，模式之间**没有自动降级**。

| | **钱包模式** | **链上模式** |
| --- | --- | --- |
| **凭证** | `Authorization` 头中的 API Key | 本地私钥（`LocalSigner`） |
| **是否需要账号** | 需要：在[控制台](/zh/guide/console)创建租户、项目、Agent 与密钥 | 不需要 |
| **支付方式** | 网关从租户钱包扣款：Hold → 上游 → Capture | 客户端签署 EIP-3009 授权，由 Facilitator 上链结算 |
| **延迟 / 成本** | 内部账本，无 Gas | 需等待网络确认，Gas 由 Facilitator 代付 |
| **适用场景** | 同一组织内的 Agent 集群 | 跨组织调用、无事先合作关系 |

::: tip
钱包模式请求余额不足时，会收到 `402 ERR_WALLET_INSUFFICIENT_BALANCE`。SDK **不会**自动改走链上，切换模式始终是你在代码中显式做出的决定。
:::

### 钱包模式（API Key）

```python
import os

from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

policy = MemoryPolicyEngine(
    max_per_tx=100_000,         # 单笔 0.10 USDC
    daily_budget=5_000_000,     # 滚动 24 小时 5.00 USDC
)

# 钱包模式不产生链上签名，因此 signer 只是占位。
# 请使用一次性的公开开发私钥，切勿使用持有资金的密钥。
signer = LocalSigner(
    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
)

client = AgentPayClient(
    signer=signer,
    policy=policy,
    headers={"Authorization": f"Bearer {os.environ['AGENTPAY_API_KEY']}"},
)

resp = client.get("https://gateway.example.com/v1/market-data?ticker=NVDA")
print(resp.status_code, resp.json())
```

网关会校验密钥、据此解析租户、项目与 Agent，应用网关策略，在租户钱包中冻结价格，用**网关自己持有**的凭证调用上游，并在 `2xx` 后确认扣款。你的代码永远看不到上游的密钥。

除 `Authorization` 头外，也可使用 `X-API-Key`。

### 链上模式（x402）

```python
import os

from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

signer = LocalSigner(private_key=os.environ["AGENT_PRIVATE_KEY"])
policy = MemoryPolicyEngine(
    max_per_tx=1_000_000,                    # 1 USDC
    daily_budget=10_000_000,                 # 10 USDC
    whitelist_payees=["0xSellerAddress..."],
)

client = AgentPayClient(signer=signer, policy=policy)   # 无需 API Key

resp = client.get("https://gateway.example.com/v1/market-data")
print(resp.json())
```

当服务端返回携带 `PAYMENT-REQUIRED` 挑战的 `402` 时，客户端会透明地完成以下流程：

```text
1. 解码挑战（价格、收款人、资产、网络、有效期）
2. 策略：评估 → 原子地 RESERVE（预留）预算      （此处被拒 = 不会产生任何签名）
3. 签名器：生成 EIP-712 / EIP-3009 授权
4. 携带 PAYMENT-SIGNATURE 头重放请求
5. 成功后：COMMIT（提交）预留
```

**失败处理至关重要。** 签名一旦发出，超时或 `5xx` **不会**释放预算，因为该支付仍可能完成结算。只有授权已过期且确认未被花费后，预留才会被释放，这就是防止重复扣款的机制。

::: warning 私钥安全
请从环境变量或密钥管理服务加载私钥，切勿写在源码或提示词中。切勿把签名器或策略对象作为工具暴露给模型。生产环境请规划使用 KMS / HSM 签名（目前尚未提供）。
:::

---

## 同步与异步

客户端默认是同步的。异步代码中，可从同一个对象获取异步客户端：

```python
import asyncio

async def main():
    async with client.async_client() as aclient:
        resp = await aclient.get("https://gateway.example.com/v1/market-data")
        print(resp.status_code, resp.json())

asyncio.run(main())
```

请使用 `async with`，以确保连接被正确关闭。

---

## 本地金融防火墙

`policy` 参数接收一个策略引擎，目前提供两种：

| 引擎 | 状态 | 适用场景 |
| --- | --- | --- |
| `MemoryPolicyEngine` | 内存（重启后丢失） | 测试、演示、短生命周期的 Agent |
| 基于 SQLite 的引擎 | 持久化，在独占事务中原子预留 | 长期运行的 Agent，以及预算必须在重启后保留的场景 |

```python
from agentpay.policy import MemoryPolicyEngine

policy = MemoryPolicyEngine(
    max_per_tx=500_000,
    daily_budget=5_000_000,
    whitelist_payees=["0x742d35Cc6634C0532925a3b844Bc454e4438f44e"],
)
```

所有金额均为**最小单位整数**（USDC 为 6 位小数，`500_000` 即 0.5 USDC）。引擎采用默认拒绝：任何错误都意味着支付被拒绝。全部规则见[策略配置](/zh/guide/policy)。

::: warning 一个密钥，一个账本
一个私钥必须只对应一个 SQLite 账本。多个进程使用同一密钥却各用独立账本，会使有效预算成倍放大。
:::

---

## Agent 身份与审计元数据

**身份来自凭证，而不是请求头。** 网关根据 API Key 解析 `agent_id`。客户端发送的任何 `X-Agent-ID`（或其他 `X-Agent-*`）头都会被**剥离并忽略**，因此无法被用来冒充其他 Agent。你不需要、也不应该设置它。

你**可以**添加的是可选的自报上下文，仅用于在控制台时间线中聚合调用：

| 请求头 | 用途 |
| --- | --- |
| `X-Session-ID` | 聚合属于同一工作流会话的调用 |
| `X-Task-ID` | 聚合属于同一任务的调用 |
| `X-Tool-Name` | 发起调用的工具名称 |

```python
client = AgentPayClient(
    signer=signer,
    policy=policy,
    headers={
        "Authorization": f"Bearer {api_key}",
        "X-Session-ID": "session_nvda_analysis_001",
        "X-Task-ID": "task_market_data_fetch",
        "X-Tool-Name": "fetch_stock_quote",
    },
)
```

这些请求头 **100% 可选**，且绝不影响鉴权、计费与策略：

| | 传了这些头 | 没传 |
| --- | --- | --- |
| **请求处理** | 经过过滤（仅允许的字符，最长 128 字符）后记录 | 视为空值，请求正常处理 |
| **计费与限额** | 不受影响，仅以 API Key 对应的 Agent 为准 | 不受影响 |
| **控制台时间线** | 可按会话 / 任务筛选与聚合，并带有"自报"标识 | 每次调用单独展示，仅含网关观测到的事实 |

网关会把自己观测到的事实（状态、耗时、扣费、策略决策）与任何自报内容分开记录，因此审计轨迹始终能区分"观测到的"与"声明的"。

---

## 幂等与安全重试

网络会出错，而最危险的重试就是**重复付款**。请遵循以下规则：

1. **对任何会产生费用的请求发送 `Idempotency-Key`**，并在重试同一逻辑请求时使用**同一个 Key**。
2. 相同 Key 且请求相同，会回放已存结果（`X-Cache: HIT`），**不产生额外费用**；上游 `5xx` 之后也是如此（会复用原来的冻结）。
3. 相同 Key 但请求体**不同**，会返回 `409 ERR_IDEMPOTENCY_CONFLICT`，Key 不能用于不同的请求。
4. **超时不等于失败。** 请求超时意味着结果未知，请使用**同一个 Key** 重试，而不是换新 Key。

```python
import uuid

key = str(uuid.uuid4())          # 每个逻辑操作一个 Key

resp = client.post(
    "https://gateway.example.com/v1/report",
    json={"ticker": "NVDA"},
    headers={"Idempotency-Key": key},
)
# 超时后，请用同一个 Key 再次调用：
# resp = client.post(..., headers={"Idempotency-Key": key})
```

<!-- TODO(author): 请确认未提供时 SDK 是否会自动生成 Idempotency-Key，并补充其生成规则。 -->

---

## 错误处理

策略或支付被拒绝并不是崩溃：请求在资金流动**之前**就被拦下了。请根据 HTTP 状态码与 JSON 响应体中的标准错误码来判断，不要依赖文案。

```python
resp = client.get("https://gateway.example.com/v1/market-data")

if resp.status_code == 200:
    data = resp.json()
elif resp.status_code == 402:
    # 没有 PAYMENT-REQUIRED 挑战的 402 是终态错误
    # （例如钱包余额不足），不要盲目重试。
    print("支付被拒绝:", resp.json())
elif resp.status_code in (403, 429):
    # 策略、路由或频率限制拒绝
    print("被策略拦截:", resp.json())
else:
    resp.raise_for_status()
```

<!-- TODO(author): 请确认 JSON 错误信封的结构，以及本地防火墙拒绝时抛出的异常类型，然后替换上面的通用处理。 -->

| 错误码 | HTTP | 含义 | 常见处理 |
| --- | --- | --- | --- |
| `ERR_POLICY_MAX_PER_TX` | 403 | 价格超过单笔上限 | 不要重试，排查价格 |
| `ERR_POLICY_BUDGET_EXCEEDED` | 403 | 将超出滚动预算 | 停止；等待窗口滚动或调高预算 |
| `ERR_POLICY_CIRCUIT_BROKEN` | 429 | 熔断器已打开 | 排查循环，冷却后重试 |
| `ERR_RATE_LIMIT_EXCEEDED` | 429 | 请求过多 | 退避 |
| `ERR_ROUTE_NOT_ALLOWED` | 403 | Agent 无权调用该路由 | 修改白名单，而不是修改代码 |
| `ERR_WALLET_INSUFFICIENT_BALANCE` | 402 | 钱包余额不足 | 充值；不会自动改走链上 |
| `ERR_WALLET_FROZEN` | 403 | 钱包被冻结 | 在控制台处理 |
| `ERR_API_KEY_INVALID` | 401 | 密钥未知、已撤销或已过期 | 检查或轮换密钥 |
| `ERR_AGENT_SUSPENDED` / `ERR_AGENT_REVOKED` | 403 | Agent 已被停用 | 重新启用或重新签发 |
| `ERR_IDEMPOTENCY_CONFLICT` | 409 | 相同 Key，不同请求 | 新操作使用新 Key |
| `ERR_UPSTREAM_TIMEOUT` / `ERR_UPSTREAM_FAILED` | 504 / 502 | 上游问题，资金保持冻结待核查 | 使用**同一个**幂等键重试 |

完整列表见 [API 规范](/zh/guide/api-specs)。

---

## LangChain 与 CrewAI 付费工具

包装一个函数，使每次调用都是受管控的付费调用：

```python
from agentpay.integrations.langchain import paid_tool

@paid_tool(
    name="fetch_stock_quote",
    description="获取实时美股行情（按次扣费）",
    client=client,
)
def fetch_stock_quote(ticker: str) -> dict:
    resp = client.get(f"https://gateway.example.com/v1/market-data?ticker={ticker}")
    return resp.json()

result = fetch_stock_quote.invoke({"ticker": "NVDA"})
```

工具只能看到函数的输入与输出。模型可以调用该工具，但无法访问客户端的策略、签名器或 API Key。

---

## 卖方：FastAPI 中间件

为你自己的 FastAPI 接口收费：

```python
from fastapi import FastAPI
from agentpay.fastapi import requires_payment

app = FastAPI()

@app.get("/v1/market-data")
@requires_payment(
    price=100_000,                                         # 0.1 USDC
    payee="0xSellerPayoutAddress...",
    asset="0x036CbD53842c5426634e7929541eC2318f3dCF7e",    # Base Sepolia USDC
    network="eip155:84532",
    facilitator_url="https://facilitator.agentpay.dev",
)
async def get_market_data():
    return {"status": "success", "data": "Alpha Market Data"}
```

未付款的请求会收到携带 `PAYMENT-REQUIRED` 挑战的 `402`；合法的已签名请求会先经 Facilitator 校验，再进入你的处理函数。如果想在**不修改代码**的前提下为现有 API 收费，请改为将其置于[网关](/zh/guide/gateway)之后。

---

## 本地测试

- **内存沙箱。** 本地嵌入式拓扑通过内存传输将请求路由到网关代码与 Mock EVM RPC，不占用端口，也无需测试网资金。`make demo` 与测试套件正是以这种方式在数秒内完成运行的。
- **对接本地网关。** 按照[快速开始](/zh/guide/quickstart)在 Docker 中运行网关与 Postgres，并使用开发密钥调用。
- **做拒绝类测试。** 被策略拦截的请求既不产生签名，也不产生扣费，因此可以放心反复测试限额。

<!-- TODO(author): 如果内存传输（GatewayTransport）面向用户开放，请补充其公开导入路径。 -->

---

## 故障排查

| 现象 | 可能原因 |
| --- | --- |
| `TypeError: unexpected keyword argument 'gateway_url'` / `'api_key'` | 客户端没有这些参数。请传入 `signer`、`policy` 以及 `headers={"Authorization": ...}`。 |
| `401 ERR_API_KEY_INVALID` | 密钥错误、已撤销或已过期，或 `Bearer` 头有拼写错误。 |
| 所有调用都被 `ERR_POLICY_MAX_PER_TX` 拦截 | 上限低于路由价格，或金额按整数 USDC 而非最小单位填写。 |
| 重启后预算表现异常 | `MemoryPolicyEngine` 会丢失状态；请使用 SQLite 引擎，并保证一个密钥一个账本。 |
| `httpx.ConnectError` | 网关未运行，或 URL / 端口有误。 |
| `httpx.ReadTimeout` | 结果未知。请使用**同一个**幂等键重试。 |
| 出现疑似重复扣费 | 重试时使用了新的 `Idempotency-Key`。每个逻辑操作请固定使用同一个 Key。 |
| `X-Agent-ID` 似乎被忽略 | 这是设计使然，Agent 身份来自 API Key。 |

## 下一步

| 我想… | 阅读 |
| --- | --- |
| 在本地跑通全流程 | [5 分钟快速开始](/zh/guide/quickstart) |
| 设置花费限额 | [策略配置](/zh/guide/policy) |
| 为 API 加上价格 | [网关配置](/zh/guide/gateway) |
| 创建 Agent 与 API Key | [管理控制台](/zh/guide/console) |
| 查看线上协议 | [API 规范](/zh/guide/api-specs) |