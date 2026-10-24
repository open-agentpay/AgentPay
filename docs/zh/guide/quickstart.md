---
title: 5 分钟快速开始
description: 在本地运行 AgentPay 网关，用 Python SDK 完成第一次受策略管控的付费调用，并亲眼看到金融防火墙拦截失控 Agent。
---

# 5 分钟快速开始

本指南带你在本机运行 AgentPay 网关，用 Python Agent 完成一次付费 API 调用，然后观察防火墙如何工作。全程**不需要钱包、测试网资金或真实资金**。

::: warning 仅限沙箱
AgentPay 仍是实验性项目，尚未经过独立审计。本指南的所有操作都在本地使用开发数据运行，切勿接入主网资金。
:::

<!--
  视频占位 —— 录制完成后在此处嵌入，例如：
  <iframe src="…" width="100%" height="420" allowfullscreen></iframe>
-->

## 你将完成什么

| # | 步骤 | 用时 | 你会看到 |
| --- | --- | --- | --- |
| 1 | 获取代码并启动 PostgreSQL | 约 1 分钟 | 一个运行中的数据库容器 |
| 2 | 启动网关 | 约 1 分钟 | 健康检查返回正常 |
| 3 | 查看 `402 Payment Required` 挑战 | 约 30 秒 | 网关返回的原始 x402 挑战 |
| 4 | 安装 Python SDK | 约 1 分钟 | 可以 `import agentpay` |
| 5 | 发起第一次受管控请求 | 约 30 秒 | 一次 `200` 响应，费用从租户钱包扣除 |
| 6 | 用幂等键安全重试 | 约 30 秒 | 零额外费用的回放响应 |
| 7 | 观察防火墙拦截失控循环 | 约 1 分钟 | 先被批准，随后被阻断的支付 |
| 8 | 清理环境 | 约 10 秒 | 一切已停止 |

## 前置条件

| 工具 | 版本 | 检查命令 |
| --- | --- | --- |
| Git | 较新版本即可 | `git --version` |
| Docker + Compose v2 | Docker 24+ | `docker compose version` |
| Go | 1.21+ | `go version` |
| Python | 3.10+ | `python3 --version` |
| curl | 任意 | `curl --version` |

请确保端口 **5432**（PostgreSQL）、**8080**（网关数据面）和 **8081**（管理 API）未被占用。Windows 用户请使用 WSL 2。

---

## 1. 获取代码并启动 PostgreSQL

```bash
git clone https://github.com/open-agentpay/AgentPay.git
cd AgentPay

docker compose up -d postgres
```

确认容器已正常运行：

```bash
docker compose ps
```

你应当看到 `postgres` 服务状态为 `running`（或 `healthy`）。数据库表结构由 `infra/postgres/schema.sql` 创建。

## 2. 启动网关

打开**第二个终端**（网关会在前台运行）并启动：

```bash
cd AgentPay/gateway-go
go run ./cmd/gateway
```

首次运行需要下载 Go 依赖并编译，可能需要一分钟。就绪后网关会输出监听日志：

| 端口 | 用途 |
| --- | --- |
| `127.0.0.1:8080` | 数据面 —— Agent 发送的业务流量 |
| `127.0.0.1:8081` | 管理 API —— 供管理控制台使用 |

回到第一个终端，检查网关是否存活：

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/readyz
curl -s http://127.0.0.1:8080/version
```

`/healthz` 表示进程存活；`/readyz` 表示已加载配置、可以处理请求；`/version` 输出构建元数据。

::: details 网关的路由从哪里来？
启动时网关会加载一份配置快照。开发配置中包含一条示例付费路由 `/weather/current`，下文将使用它。网关会通过 `X-Config-Version` 响应头告知每个请求命中的配置快照版本。如需自定义路由，请参阅[网关配置](/zh/guide/gateway)。
:::

## 3. 查看 `402 Payment Required` 挑战

在使用 SDK 之前，先看看 Agent **不带任何凭证**调用付费路由时会看到什么：

```bash
curl -i "http://127.0.0.1:8080/weather/current?city=SanFrancisco"
```

预期结果：

```text
HTTP/1.1 402 Payment Required
PAYMENT-REQUIRED: eyJ2ZXJzaW9uIjoiMi4wIiwi...
```

`PAYMENT-REQUIRED` 头是 Base64URL 编码的 JSON 挑战，包含价格、收款人、资产、网络和有效期。解码查看：

```bash
curl -si "http://127.0.0.1:8080/weather/current?city=SanFrancisco" \
  | grep -i '^payment-required:' | cut -d' ' -f2 | tr -d '\r' \
  | python3 -c "import sys,base64,json; s=sys.stdin.read().strip(); s+='='*(-len(s)%4); print(json.dumps(json.loads(base64.urlsafe_b64decode(s)),indent=2))"
```

你会看到 `network`、`asset`、`amount`（最小单位 —— USDC 为 6 位小数，`100000` 即 0.1 USDC）、`payee`、`validBefore` 与 `scheme` 等字段。这个未认证请求展示的是**链上路径**。API Key 钱包路径同样先返回 x402 风格的 `402` 价格挑战：网关先报价，SDK 用本地策略检查价格，再携带钱包授权重试。这个流程不会生成 EIP-3009 签名，也不会调用 Facilitator。

## 4. 安装 Python SDK

在仓库根目录，使用虚拟环境：

```bash
python3 -m venv .venv
source .venv/bin/activate          # Windows（WSL）同上；PowerShell：.venv\Scripts\Activate.ps1

pip install -e python/packages/agentpay
python -c "import agentpay; print('agentpay OK')"
```

## 5. 发起第一次受管控请求

创建 `first_request.py`：

```python
import os

from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import MemoryPolicyEngine

# 1. 本地财务防火墙：防止 Agent 陷入循环刷爆预算。
#    金额为最小单位整数；钱包挑战使用 USD 计价。
policy = MemoryPolicyEngine(
    max_per_tx=1_000_000,       # 单笔最多 1 USD
    daily_budget=10_000_000,    # 24 小时最多 10 USD
)

# 2. 签名器。钱包模式不需要链上签名，这里仅作占位。
#    ⚠️ 仅限测试网开发：此公钥对应 Hardhat/Anvil 默认公开测试密钥，切勿转入任何主网真实资产！
dev_key = os.getenv("AGENT_PRIVATE_KEY", "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
signer = LocalSigner(dev_key)

# 3. 为 Agent 颁发的 API Key（开发密钥，已预置测试钱包余额）。
api_key = os.getenv("AGENTPAY_API_KEY", "apk_live_valid123")

# 4. 开启价格感知钱包模式。SDK 先接收 402 报价，用本地策略检查确切金额，
#    获准后再携带钱包授权重试请求。
client = AgentPayClient(
    signer=signer,
    policy=policy,
    headers={"Authorization": f"Bearer {api_key}"},
    wallet_challenge_mode=True,
)

# 首次请求会收到价格挑战。只有本地策略批准后 SDK 才重试；随后网关冻结租户钱包余额，
# 并调用上游。
response = client.get("http://127.0.0.1:8080/weather/current?city=SanFrancisco")
print("Response status:", response.status_code)
print("Data:", response.json())
```

`AgentPayClient` 继承自 `httpx.Client`，因此 `get`、`post`、超时设置等 `httpx` 用法都可以照常使用。主要 AgentPay 参数如下：

| 参数 | 作用 |
| --- | --- |
| `policy` | **本地**防火墙（单笔上限、滚动预算、收款人白名单）；钱包挑战模式会在网关冻结余额前评估报价。 |
| `signer` | 为**链上路径**签署 EIP-712 / EIP-3009 授权。钱包挑战模式不会调用它，但当前客户端构造函数仍要求传入 signer。 |
| `headers` | 默认请求头。`Authorization: Bearer <api_key>` 用于认证钱包请求。 |
| `wallet_challenge_mode` | 调用 AgentPay 网关 API Key 钱包路由时设为 `True`。SDK 等待价格挑战、评估本地策略，再携带 `PAYMENT-SIGNATURE` 重试；默认是 `False`。 |

如果配置了白名单，钱包挑战使用 `USD` 资产与 `agentpay:route:<route-id>` 形式的收款方；链上挑战仍使用代币合约地址。

更喜欢异步？可以从同一个对象获取异步客户端：

```python
import asyncio

async def main():
    async with client.async_client() as aclient:
        res = await aclient.get("http://127.0.0.1:8080/weather/current?city=SanFrancisco")
        print(res.status_code, res.json())

asyncio.run(main())
```

运行：

```bash
python first_request.py
```

预期输出：

```text
Response status: 200
Data: {...天气数据...}
```

这一次调用背后，网关依次完成了：

1. **报价** —— 通过 `402` `agentpay-wallet` 挑战返回路由价格，此时不会冻结资金。
2. **本地策略评估** —— 针对确切报价检查策略并预留 SDK 预算。
3. **授权** —— SDK 携带 `PAYMENT-SIGNATURE` 重试原请求；网关验证并一次性消费挑战。
4. **冻结（Hold）** —— 网关冻结租户钱包余额，再注入上游凭证并转发请求。
5. **确认扣款（Capture）** —— 上游返回 `2xx` 后确认扣款，并追加一条不可变的账本流水。

如果上游返回 `4xx`，冻结会被**释放**，不产生任何费用；如果上游返回 `5xx` 或超时，冻结会保持不动，支付被标记为待核查——既不会自动扣款，也不会自动释放。

::: tip 沙箱密钥
`apk_live_valid123` 是仅存在于本地开发配置中的**开发种子密钥**。真实部署中，请在[管理控制台](/zh/guide/console)创建 Agent 与密钥；密钥只会展示一次，且仅以哈希形式存储。
:::

## 6. 用幂等键安全重试

网络会出错，真正的风险是重试导致重复扣费。携带同一个 `Idempotency-Key` 发送两次相同请求：

```bash
for i in 1 2; do
  curl -si "http://127.0.0.1:8080/weather/current?city=SanFrancisco" \
    -H "Authorization: Bearer apk_live_valid123" \
    -H "Idempotency-Key: quickstart-demo-001" \
    | grep -iE '^(HTTP/|x-cache|x-config-version)'
  echo "---"
done
```

预期结果：

```text
HTTP/1.1 200 OK
X-Config-Version: <n>
---
HTTP/1.1 200 OK
X-Cache: HIT
X-Config-Version: <n>
---
```

第二次响应是对第一次结果的回放（`X-Cache: HIT`），**不产生任何额外费用**。如果用同一个 Key 发送*不同*的请求体，会被拒绝并返回 `409 ERR_IDEMPOTENCY_CONFLICT`，因此幂等键无法被用来夹带另一笔不同的交易。

## 7. 观察防火墙拦截失控循环

AgentPay 最重要的特性，是让行为失常的 Agent 被及时切断。仓库提供了一条命令即可运行的演示，包含三个场景：正常支付成功、超限支付被拒绝、重复支付循环触发熔断。

在仓库根目录执行：

```bash
make demo
```

演示既可以运行在本地 Mock 链上（无需钱包与测试网资金），也可以运行在 Arbitrum Sepolia 上；切换方式见仓库 README。请在输出中留意以下三种结果：

| 场景 | 预期结果 |
| --- | --- |
| 限额内的正常调用 | 批准并完成支付 |
| 单笔支付超过单笔上限 | 被拒绝 —— 不会签名，也不会扣费 |
| 同一笔支付在短时间内被循环重复 | 起初被批准，随后**熔断器打开**，后续支付被阻断 |

如需设置自己的限额，请参阅[策略配置](/zh/guide/policy)。

## 8. 清理环境

在网关所在终端按 `Ctrl+C` 停止网关，然后：

```bash
docker compose down          # 停止 PostgreSQL（数据保留在数据卷中）
docker compose down -v       # …或同时删除数据，得到完全干净的环境
```

---

## 刚才发生了什么

```text
                    ┌─ 钱包路径（步骤 5–6）──────┐
Agent ── API Key ──▶│  402 报价 → SDK 策略 →     │──▶ 上游 API
                    │  授权 → HOLD → CAPTURE     │
                    └────────────┬───────────────┘
                                 ▼
                       PostgreSQL 账本 + 审计轨迹

Agent ── 无凭证 ───▶ 网关 ──▶ 402 + PAYMENT-REQUIRED   （链上路径，步骤 3）
```

- Agent 使用**可撤销的 API Key** 认证，钱包路径下不持有任何私钥。
- SDK 钱包挑战模式会先用本地策略检查报价，再允许网关冻结钱包余额。请设置 `wallet_challenge_mode=True`。
- 每笔扣费都经过 **Hold →（履约）→ Capture 或 Release**，因此只为成功交付的调用付费。
- **幂等键**让重试变得安全。
- 花费限额与熔断器在**模型之外**执行，提示词无法绕过。

## 常见问题排查

| 现象 | 可能原因与解决办法 |
| --- | --- |
| `docker compose up` 报 `port is already allocated` | 5432 端口被占用。请停止占用进程，或修改 `docker-compose.yml` 中发布的端口。 |
| 网关因数据库连接错误退出 | PostgreSQL 尚未就绪。等待几秒，用 `docker compose ps` 检查后重启网关。 |
| 8080 / 8081 报 `bind: address already in use` | 其他进程占用了端口。用 `lsof -i :8080` 找到并停止它。 |
| `go: go.mod requires go >= …` | Go 版本过低，请升级到 1.21 或更高。 |
| `curl: (7) Failed to connect` | 网关未运行。请检查运行 `go run ./cmd/gateway` 的终端。 |
| Python 抛出 `ConnectError` | `gateway_url` 有误，或首次运行时网关仍在编译。 |
| `401 ERR_API_KEY_INVALID` | 网关不认识该密钥。请使用预置的开发密钥，或在控制台创建。 |
| `402 ERR_WALLET_INSUFFICIENT_BALANCE` | 钱包余额不足以支付路由价格。网关**不会**降级为链上支付，请在控制台为钱包充值。 |
| `403 ERR_POLICY_MAX_PER_TX` / `ERR_POLICY_BUDGET_EXCEEDED` | 这是防火墙在正常工作。若请求合法，请调高策略限额。 |
| Python 3.9 及以下 `pip install` 失败 | 请使用 Python 3.10+。 |

仍然无法解决？请[提交 issue](https://github.com/open-agentpay/AgentPay/issues)，并附上网关日志与失败请求的 `curl -i` 输出。

## 下一步

| 我想… | 阅读 |
| --- | --- |
| 为 Agent 设置花费限额 | [策略配置](/zh/guide/policy) |
| 在 LangChain 或自有代码中使用 AgentPay | [Python SDK](/zh/guide/python-sdk) |
| 给自己的 API 加上价格 | [网关配置](/zh/guide/gateway) |
| 在界面中创建租户、Agent、密钥与路由 | [管理控制台](/zh/guide/console) |
| 了解线上协议 | [API 规范](/zh/guide/api-specs) |
| 部署到 localhost 之外 | [部署指南](/zh/guide/deployment) |
