---
title: 策略配置指南
description: 配置 AgentPay 金融防火墙——单笔上限、24 小时滚动预算、白名单、频率限制、熔断器与策略继承，附配置示例、容量估算与故障排查。
---

# 策略配置指南

策略是 AgentPay 的核心。它决定 **Agent 能不能付款、最多付多少、付给谁、付得多快**，并且由*独立于模型*的确定性代码强制执行。Agent 可以提议付款，但永远无法修改、读取策略，也无法用话术绕过策略。

::: tip 请记住一条规则
策略是**默认拒绝（fail-closed）**的。规则被违反、锁超时、解析错误或任何意外异常，结果都是**拒绝请求**，绝不会悄悄放行。
:::

## 策略在哪里执行

AgentPay 在两个位置执行策略，二者互补：第一层保护买方自己的钱包，第二层保护平台及其租户。

| 层级 | 运行位置 | 保护对象 | 配置方式 |
| --- | --- | --- | --- |
| **本地防火墙** | 买方 SDK 进程内（`SQLitePolicyEngine` 内存模式或持久化文件模式） | 你自己的签名密钥与预算，防止循环调用、提示词注入与被抬高的报价 | Python 代码或 YAML 策略文件 |
| **网关策略** | AgentPay 网关，位于请求路径中 | 租户钱包与平台：按 Agent 的预算、频率限制、路由访问、Kill Switch | [管理控制台](/zh/guide/console)（草稿 → 校验 → 发布） |

两层都在**资金流动之前**评估。任何一层拒绝，都不会产生签名、冻结或扣费。

---

## 快速示例

适用于调用约 0.05 USDC API 的 Agent 的最小保守策略：

```yaml
policy_id: "pol_weather_default"
tenant_id: "ten_default"

max_amount_per_tx: 500000          # 单笔 0.5 USDC
rolling_budget_24h: 5000000        # 滚动 24 小时 5.0 USDC

allowed_payees:
  - "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

circuit_breaker:
  consecutive_failures_limit: 3
  cooldown_seconds: 60
```

在 Python SDK 中直接应用同样的限额：

```python
from agentpay import AgentPayClient, LocalSigner
from agentpay.policy import PolicyConfig, SQLitePolicyEngine

signer = LocalSigner(...)
policy = SQLitePolicyEngine.create_in_memory(
    PolicyConfig(
        id="pol_weather_default",
        name="Weather Policy",
        wallet_address=signer.address,
        max_per_tx_base_units=500_000,          # 0.5 USDC
        rolling_budget_24h_base_units=5_000_000, # 5.0 USDC
        whitelist_payees=frozenset(["0x742d35Cc6634C0532925a3b844Bc454e4438f44e"]),
    )
)
client = AgentPayClient(signer=signer, policy=policy)
```

---

## 金额与单位

**所有金额都是最小单位整数。** 金额永远不接受也不使用浮点数。USDC 为 6 位小数：

| 人类可读金额 | 最小单位 |
| --- | --- |
| 0.001 USDC | `1000` |
| 0.01 USDC | `10000` |
| 0.10 USDC | `100000` |
| 1 USDC | `1000000` |
| 100 USDC | `100000000` |

::: warning
`500000` 表示 **0.5 USDC**，而不是 500,000 USDC。数量级差 10 倍是最常见的配置错误，请仔细核对零的个数。
:::

---

## 规则参考

### 单笔上限

| | |
| --- | --- |
| **YAML 字段** | `max_amount_per_tx` |
| **SDK 参数** | `max_per_tx` |
| **网关字段** | `max_amount_per_request` |
| **类型** | 整数，最小单位，`> 0` |

Agent 为**单次**请求愿意支付的最高价格。用于防御恶意或被攻破的上游突然把 0.05 美元的报价改成 500 美元。如果挑战或路由价格超过上限，请求会立即被拒绝并返回 `ERR_POLICY_MAX_PER_TX`，且**不会产生任何签名**。

### 24 小时滚动预算

| | |
| --- | --- |
| **YAML 字段** | `rolling_budget_24h` |
| **SDK 参数** | `daily_budget` |
| **网关字段** | `rolling_24h_budget_cap` |
| **类型** | 整数，最小单位，`> 0` |

这是**滑动窗口**，而不是自然日：任何时刻，引擎都会汇总过去 24 小时内已花费与已预留的金额，若新请求会使总额超限，则拒绝（`ERR_POLICY_BUDGET_EXCEEDED`）。已预留金额会立即计入，因此并发请求不会合计超支；已释放的预留不再计入。

单笔上限必须**小于或等于**滚动预算；单笔上限大于预算的策略无效，无法发布。

### 终身限额

| | |
| --- | --- |
| **YAML 字段** | `lifetime_limit` |
| **类型** | 整数，最小单位，`> 0` |

账本整个生命周期内的累计花费绝对上限，与滚动窗口无关。适合作为实验性 Agent 的最后一道兜底。

### 收款人、域名与网络白名单

| | |
| --- | --- |
| **YAML 字段** | `whitelist_mode`、`allowed_payees`、`allowed_networks` |
| **SDK 参数** | `whitelist_mode`（`WhitelistMode.SIMPLE` / `WhitelistMode.STRICT`）、`whitelist_payees`、`whitelist_assets`、`whitelist_networks`、`whitelist_domains` |
| **网关字段** | `whitelisted_payees`、`whitelisted_networks` |
| **类型** | `WhitelistMode` 枚举（`"simple"` \| `"strict"`）+ 字符串列表 / `frozenset` |

按所配置的白名单策略模式向允许的收款人、资产、域名和网络付款：

- **收款人**为 EVM 地址（`0x` + 40 位十六进制字符）。
- **网络**使用 [CAIP-2](https://github.com/ChainAgnostic/CAIPs/blob/master/CAIPs/caip-2.md) 标识，如 `eip155:421614`（Arbitrum Sepolia）。
- **Python SDK 链上路径支持两种白名单模式**（`whitelist_mode`）：
  - **简单模式（`WhitelistMode.SIMPLE`，默认）**：某个白名单维度为空集合时默认放行（不限制）；若该维度配置了白名单项，则仅放行列表内的目标。
  - **严格模式（`WhitelistMode.STRICT`）**：所有链上白名单维度（`whitelist_networks`、`whitelist_assets`、`whitelist_payees`，以及请求携带域名时的 `whitelist_domains`）均必须显式添加白名单且命中，任一维度为空即默认拒绝。
- 除本地快速实验外，生产或高价值 Agent 建议开启 `WhitelistMode.STRICT` 或显式设置 `whitelist_payees`，避免攻击者把付款重定向到自己的地址。

收款人、资产、域名或网络不在允许范围内的请求，会在签名之前被拒绝。

### 路由白名单（网关，按 Agent）

| | |
| --- | --- |
| **网关字段** | Agent 或策略上的 `whitelisted_routes`（路径模式或 Route ID） |

限制 Agent 可以调用哪些路由。调用白名单之外的路由会收到 `403 ERR_ROUTE_NOT_ALLOWED`，适合让研究型 Agent 访问数据 API、却无法访问昂贵的算力路由。

### 频率限制

| | |
| --- | --- |
| **网关字段** | 按 Agent、按路由的限制（每分钟 / 每秒请求数）；路由默认值 `rate_limit_rpm` |

限制请求频率，超出的请求会收到 `429 ERR_RATE_LIMIT_EXCEEDED`。频率限制与预算互补：一个由大量极低价调用组成的循环，可能在耗尽预算之前就已经造成破坏。

### 熔断器

| | |
| --- | --- |
| **YAML 字段** | `circuit_breaker.consecutive_failures_limit`、`circuit_breaker.cooldown_seconds` |
| **网关字段** | `circuit_breaker_max_rpm` |

熔断器用于识别失控行为，例如**高频支付**、**对同一收款人与资源的重复支付**以及**重复失败**。触发后进入 `OPEN` 状态并阻断后续支付（`429 ERR_POLICY_CIRCUIT_BROKEN`），直到 `cooldown_seconds` 冷却期结束。它专门针对"Agent 陷入工具调用死循环"的故障模式；可在[快速开始](/zh/guide/quickstart)的演示中看到它被触发。

<!-- TODO(author): 请确认熔断器是否有半开（half-open）探测状态，以及循环检测阈值在 YAML 中如何暴露。 -->

### 允许的支付方式（网关，按路由）

| | |
| --- | --- |
| **网关字段** | 路由上的 `allowed_payment_methods`：`wallet`、`onchain` 或 `both` |

决定路由接受哪条支付路径。使用路由不允许的支付方式的请求，会被拒绝并返回 `403 ERR_PAYMENT_METHOD_NOT_ALLOWED`。不同支付方式之间**没有自动降级**：钱包路径余额不足时，返回 `402 ERR_WALLET_INSUFFICIENT_BALANCE`，而不会改走链上扣费。

### 项目预算上限

项目可以设置硬性预算上限（`budget_cap_base_units`）。`0` 表示"继承租户，不单独设限"。用它避免某个项目耗尽整个租户钱包。

### Agent 状态与 Kill Switch

处于 `suspended` 或 `revoked` 状态的 Agent，会在数秒内被网关阻断（`403 ERR_AGENT_SUSPENDED` / `ERR_AGENT_REVOKED`），停用 Agent 同时会禁用其 API Key。这是制止异常 Agent 最快的方式，无需修改策略，也无需发布。

---

## 策略继承

在网关上，限额可以在多个层级设置。请求的有效策略按从宽到窄的层级解析，**始终取最严格的值**：

```text
租户  →  项目  →  Agent  →  路由  →  策略规则
（最宽）                              （最窄）
```

| 层级 | 典型用途 |
| --- | --- |
| **租户** | 组织级总上限与钱包 |
| **项目** | 按团队或业务线划分预算 |
| **Agent** | 单个 Agent 的限制（预算、频率限制、路由白名单） |
| **路由** | 与某个付费端点及其允许的支付方式绑定的限制 |

示例：租户每天允许 100 USDC，而某个 Agent 策略每天允许 5 USDC，则该 Agent 被限制在 5 USDC。较窄的层级永远不能放宽较宽层级的禁令；对于白名单，请求必须在每一层都被允许。

---

## 预算是如何预留的

如果先检查预算、再付款，两个并发请求可能同时看到"预算充足"而合计超支。因此 AgentPay 使用**原子预留**。

**买方侧（链上路径）**

```text
评估  →  RESERVE（原子预留）  →  签名  →  发送  →  COMMIT（成功）
                                                  →  RELEASE（明确失败）
```

- 预留在独占的数据库事务中执行，并发预留会被串行化。
- 只有**已批准的预留**才会调用签名器。
- 签名一旦发出，网络错误或 `5xx` **不会**释放预算。只有当授权已过期（`now > validBefore + 60s`）**并且**确认链上未花费后，才会释放，以防止双花。

**网关侧（钱包路径）**

```text
HOLD（行级锁）→ 上游 → 2xx：CAPTURE  |  4xx：RELEASE  |  5xx/超时：保持 HELD + needs_review
```

冻结在上游超时加 60 秒（默认 120 秒）后过期，由后台清理任务释放。如果上游结果未知，资金保持冻结，并标记为待核查，而不会被自动扣款或释放。

---

## 配置方式

### A. Python SDK（本地防火墙）

最适合开发阶段，以及保护 Agent 自身的密钥。

```python
from agentpay.policy import PolicyConfig, SQLitePolicyEngine

policy = SQLitePolicyEngine.create_in_memory(
    PolicyConfig(
        id="local_firewall",
        name="Local Firewall",
        wallet_address=signer.address,
        max_per_tx_base_units=100_000,        # 0.10 USDC
        rolling_budget_24h_base_units=5_000_000, # 5.00 USDC
        whitelist_payees=frozenset(["0x742d35Cc6634C0532925a3b844Bc454e4438f44e"]),
    )
)
```

`SQLitePolicyEngine.create_in_memory(...)` 将状态保存在内存 SQLite 数据库（`:memory:`）中，适合测试与短生命周期的 Agent。如需在重启后保留状态，请通过 `create_buyer_connection` + `init_buyer_database` 传入持久化连接初始化 `SQLitePolicyEngine(conn, config)`。

::: warning 一个密钥，一个账本
同一个私钥必须只对应**一个** SQLite 账本文件。多个进程使用同一密钥却各用独立账本，会使有效预算成倍放大，等于废掉防火墙。
:::

### B. YAML 策略文件

将限额放进版本控制，并加载到 SDK 的 `PolicyConfig` 模型中，由 Pydantic 校验。部署前先校验文件：

```python
import yaml
from agentpay.policy import PolicyConfig

with open("policy.yaml") as f:
    config = PolicyConfig.model_validate(yaml.safe_load(f))
print(config)
```

格式错误的 YAML、无效地址或负数金额，都会在这里抛出校验错误，远早于 Agent 运行之前。

<!-- TODO(author): 请确认 PolicyConfig 的导入路径 / 加载方式，以及上述 YAML 字段名与模型字段名一致。 -->

### C. 管理控制台（网关策略）

网关策略遵循**草稿 → 校验 → 发布 → 回滚**的生命周期：

1. **编辑草稿。** 修改以草稿形式保存，**不影响**线上流量。
2. **校验。** 控制台执行下列检查，并对错误给出诊断报告。
3. **查看差异**：对比草稿与当前生效版本。
4. **发布。** 生成不可变、带哈希指纹的快照，网关零停机原子切换。无效的草稿无法发布。
5. **回滚**：出问题时一步回滚到任意历史版本。

每个请求都会记录处理它的配置版本（`X-Config-Version`），因此你始终知道当时生效的是哪套策略。

**发布前校验**

| 检查项 | 规则 |
| --- | --- |
| 单笔上限与预算 | `max_amount_per_request` ≤ `rolling_24h_budget_cap` |
| 收款地址 | 匹配 `^0x[a-fA-F0-9]{40}$` |
| 价格 | `price_base_units` ≥ 0 |
| 支付方式 | 取值为 `wallet`、`onchain`、`both` 之一 |
| 路由路径 | 同一租户内唯一 |
| 上游 URL | 以 `http://` 或 `https://` 开头 |
| 频率 / 熔断值 | 非负 |

::: tip YAML 导入 / 导出
网关完整配置的 YAML 导入 / 导出已在规划中。导入的配置将经过与控制台相同的校验与权限检查。
:::

---

## 错误码参考

策略拦截请求时，网关会返回带标准错误码的统一 JSON 错误信封。客户端逻辑请依据**错误码**，不要依赖文案。

| 错误码 | HTTP | 含义 | 处理方式 |
| --- | --- | --- | --- |
| `ERR_POLICY_MAX_PER_TX` | 403 | 价格超过单笔上限 | 价格合法则调高上限；否则视上游为可疑 |
| `ERR_POLICY_BUDGET_EXCEEDED` | 403 | 将超出滚动预算 | 等待窗口滚动，或调高预算 |
| `ERR_POLICY_CIRCUIT_BROKEN` | 429 | 熔断器已打开 | 排查循环；冷却后重试 |
| `ERR_RATE_LIMIT_EXCEEDED` | 429 | 触发频率限制 | 降低请求速度或调高限制 |
| `ERR_ROUTE_NOT_ALLOWED` | 403 | Agent 无权调用该路由 | 将路由加入该 Agent 的白名单 |
| `ERR_PAYMENT_METHOD_NOT_ALLOWED` | 403 | 路由不接受该支付方式 | 使用允许的方式，或修改路由 |
| `ERR_WALLET_INSUFFICIENT_BALANCE` | 402 | 钱包余额不足（不会降级为链上） | 为钱包充值 |
| `ERR_WALLET_FROZEN` | 403 | 钱包被冻结（含欠费冻结） | 在控制台处理冻结 |
| `ERR_AGENT_SUSPENDED` / `ERR_AGENT_REVOKED` | 403 | Agent 已被停用 | 重新启用，或签发新的 Agent |

注意并非所有策略违规都是 `403`：熔断与频率限制返回 `429`，方便客户端退避重试。

---

## 配置示例

可在此基础上调整。金额为最小单位，人类可读金额写在注释中。

### 研究型 Agent（少量付费数据 API）

```yaml
max_amount_per_tx: 200000          # 0.20 USDC
rolling_budget_24h: 5000000        # 5.00 USDC
allowed_payees: ["0x...dataProviderA", "0x...dataProviderB"]
circuit_breaker: { consecutive_failures_limit: 3, cooldown_seconds: 60 }
```

### 高频微额 API Agent

```yaml
max_amount_per_tx: 5000            # 0.005 USDC
rolling_budget_24h: 20000000       # 20.00 USDC
allowed_payees: ["0x...apiVendor"]
circuit_breaker: { consecutive_failures_limit: 5, cooldown_seconds: 30 }
```

请搭配按正常高峰流量调整的网关**频率限制**使用。

### 严格受限的试验（"我只是想试一下"）

```yaml
max_amount_per_tx: 50000           # 0.05 USDC
rolling_budget_24h: 500000         # 0.50 USDC
lifetime_limit: 2000000            # 累计最多 2.00 USDC
allowed_payees: ["0x...onlyThisOne"]
circuit_breaker: { consecutive_failures_limit: 2, cooldown_seconds: 300 }
```

---

## 如何确定数值

1. **从价格出发。** 将 `max_amount_per_tx` 设为合法路由最高价格的约 **2–5 倍**。过低会拦截正常调用；过高则会放过漫天要价的上游。
2. **按预期用量定预算。** `rolling_budget_24h ≈ 日均调用次数 × 平均单价 × 2`（留出安全余量）。示例：每天 200 次、每次 0.02 USDC，正常花费为 4 USDC，预算设为 8 USDC 既有余量，又不会放任失控。
3. **按正常高峰定频率限制。** 将每分钟请求数限制设为实测高峰的约 2 倍。
4. **单笔上限应远低于预算。** 通常预算至少是单笔上限的 10 倍，确保单个请求不可能用掉一整天的额度。
5. **观察后再收紧。** 在沙箱中从严开始，观察审计时间线中合法流量被 `DENY` 的情况，再有意识地放宽。

---

## 测试你的策略

在限额真正起作用之前，主动测试它们。

- **超过单笔上限。** 调用价格高于 `max_amount_per_tx` 的路由，应返回 `ERR_POLICY_MAX_PER_TX`，且没有账本流水。
- **预算耗尽。** 将预算设得很小（例如只够两次调用），连续调用三次，第三次应返回 `ERR_POLICY_BUDGET_EXCEEDED`。
- **错误的收款人。** 将路由指向不在白名单的地址，确认请求在签名前被拒绝。
- **循环。** 运行 `make demo`，观察重复支付触发熔断器。
- **Kill Switch。** 在控制台停用某个 Agent，确认新请求在数秒内被拒绝。

由于被拒绝的请求既不产生签名也不产生扣费，这些测试可以放心反复运行。

---

## 监控与审计

- 每一次决策都会记录为 `agent.policy_decision` 事件，包含结果（`ALLOW` / `DENY`）、命中的规则与原因码，可在控制台的 **Agent 时间线**与**支付审计**页面查看。
- `agent.spend` 事件记录实际扣费，并与同一请求关联，便于核对花费与决策。
- 策略与配置变更会记录操作人与配置版本。
- 留意某个 Agent 出现**反复 `DENY`**：要么是策略设置不合理，要么是该 Agent 出了问题，可考虑将其停用。

---

## 故障排查

| 现象 | 可能原因 |
| --- | --- |
| 所有请求都返回 `ERR_POLICY_MAX_PER_TX` | 上限小于路由价格，或金额按整数 USDC 而非最小单位填写。 |
| `ERR_POLICY_BUDGET_EXCEEDED` 比预期更早出现 | 窗口是滚动的，且**已预留**（尚未确认）的金额也会计入。请检查是否有未完成或卡住的冻结。 |
| 重启后预算看起来"重置"或翻倍 | 内存引擎丢失了状态，或多个进程用同一密钥却各用独立账本。请使用 SQLite 引擎，并保证一个密钥一个账本。 |
| 保存了草稿但流量表现没变 | 草稿不会生效。请校验并**发布**。 |
| 无法发布 | 某条校验规则未通过，常见为 `max_amount_per_request` 大于预算或地址格式错误。请查看诊断报告。 |
| `429 ERR_POLICY_CIRCUIT_BROKEN` | 熔断器已打开。先排查循环，再等待冷却。 |
| 看不出请求命中了哪套策略 | 查看响应头 `X-Config-Version`，确认是哪份快照处理的请求。 |

---

## 最佳实践

1. **除本地实验外，始终设置 `allowed_payees`。** 这是防止付款被重定向最有效的单项措施。
2. **分层设限。** 同时使用单笔上限、滚动预算、频率限制与熔断器；每一项都能捕获其他项漏掉的故障。
3. **最小权限。** 为每个 Agent 单独分配密钥、预算与路由白名单，而不是共用一个宽泛的配置。
4. **让策略远离模型。** 永远不要把策略编辑暴露为 LLM 工具，也不要让提示词影响限额。
5. **版本化与评审。** 将 YAML 策略放入 Git 并像代码一样评审；网关策略使用发布与回滚。
6. **只用测试网。** AgentPay 是实验性且未经审计的项目，请只使用测试代币。

## 下一步

| 我想… | 阅读 |
| --- | --- |
| 看策略如何拦截失控 Agent | [5 分钟快速开始](/zh/guide/quickstart) |
| 在代码中使用策略引擎 | [Python SDK](/zh/guide/python-sdk) |
| 为路由和价格绑定策略 | [网关配置](/zh/guide/gateway) |
| 在界面中管理策略 | [管理控制台](/zh/guide/console) |
| 理解各项保证 | [状态机规范](/zh/specs/state-machine) |