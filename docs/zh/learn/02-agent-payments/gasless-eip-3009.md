# EIP-712 / EIP-3009：为什么 Agent 不必持有 Gas

> **一句话**：买方只需**离线签一份授权**，由别人（Facilitator）代为提交上链并垫付 Gas——所以 Agent 的钱包里只放 USDC 就够了。

**本篇你会了解**：Gas 是什么、传统转账为何麻烦、EIP-3009 如何让转账"无 Gas"、EIP-712 为什么让签名更安全、签名里的每个字段起什么作用，以及在实战中如何应对异常与风险。

---

## 1. 先理解 Gas：为什么它是 AI Agent 的运维噩梦？

在以太坊系区块链（如 Ethereum、Arbitrum、Base）上，每笔改变链上状态的交易都要支付 **Gas（燃料费）**，用于补偿矿工或验证者的计算资源。关键规则在于：**Gas 必须用该链的原生代币支付**（例如在 Arbitrum 或 Base 上必须用 ETH）。

这对自主运行的 AI Agent 意味着一个极其尴尬的局面：

```text
Agent 钱包：100 USDC（用来付 API 费）
想付一笔 0.001 USDC 的调用费 → 还得另外备 ETH 付 Gas
```

如果把场景放大到企业级应用，问题会接踵而来：

- **双资产管理负担**：如果你部署了 1,000 个并发工作的 Agent，你不仅要给它们分配业务资金（USDC），还要为这 1,000 个钱包分别充值极小额的 ETH。
- **资金搁浅与服务中断**：一旦某个 Agent 的 ETH 耗尽，哪怕它账户里还有 10,000 USDC，它也会立刻瘫痪，连一笔 0.001 USDC 的 API 调用都无法完成。
- **跨链与运维复杂性**：不同链的原生代币不同（Polygon 用 POL，BNB Chain 用 BNB），运维系统需要维护一套极其复杂的“Gas 监控 + 自动跨链补给（Gas Tank）”系统。

**为了付 0.001 USDC 的微支付，却要管理两种资产并搭建一整套防断油监控系统，这在工程上显然是不合理的。**

---

## 2. 传统 ERC-20 转账的两步痛点

为什么不能直接让收款方（或第三方）从 Agent 钱包里扣 USDC，并由对方出 Gas 呢？

在标准 ERC-20 协议中，有两种转账方式：
1. **直接转账（`transfer`）**：由持币人（Agent）自己发起交易并支付 ETH Gas。
2. **授权划转（`approve` + `transferFrom`）**：想让第三方代为划转，持币人必须先发送一笔链上交易调用 `approve`（授权额度），再由第三方调用 `transferFrom`（实际划转）。

```text
传统 ERC-20 委托划转流程：
步骤 1：买方 ──► 调用 approve(第三方, 额度) ──► 【上链交易，买方必须付 ETH Gas】
步骤 2：第三方 ──► 调用 transferFrom(买方, 卖方, 金额) ──► 【上链交易，第三方付 ETH Gas】
```

**核心痛点**：
- **死循环**：每一步都是链上交易，每一步都要 Gas，且第一步 `approve` 依然要求 Agent 自己发交易、自己付 ETH Gas，没有解决“零 ETH 启动”的问题。
- **安全隐患**：为了省事，很多应用会让用户一次性 `approve` 无限额度（Infinite Approval）。一旦第三方合约被黑，Agent 钱包里的所有 USDC 都会被瞬间洗劫一空。

---

## 3. EIP-3009：转账授权（`transferWithAuthorization`）

为了彻底解决上述痛点，USDC 的发行方 Circle 提出了 **EIP-3009** 标准，并在各条主流链的原版 USDC 合约中内置了该功能。

EIP-3009 给代币合约增加了一个核心函数 `transferWithAuthorization`：**允许持币人签一份离线授权（Off-chain Signature），任何人拿着这份授权都可以代为提交上链并完成转账**。

```text
        ┌──────────────────────────┐  ① 离线签名授权（纯本地计算，零 Gas）
 买方 ──►│ 授权：付 0.1 USDC 给卖方， │
(Agent) │ 有效期 5 分钟，nonce=0x4a..│
        └────────────┬─────────────┘
                     │ ② 通过 HTTP 请求交给 Facilitator
                     ▼
              Facilitator ── ③ 调用 transferWithAuthorization(授权参数, 签名)
                     │         并用 Facilitator 自己的 ETH 垫付 Gas
                     ▼
              链上 USDC 合约：校验签名与参数通过后，买方 USDC −0.1，卖方 USDC +0.1
```

**关键突破点**：

- **买方彻底告别 ETH**：买方**只做本地密码学签名**，不发链上交易，不花一分钱 Gas，**钱包里完全不需要持有 ETH**；
- **Gas 责任转移**：链上交易的 Gas 由实际提交交易的人（Facilitator / 卖方中继器）垫付；
- **智能合约级安全**：USDC 合约在链上通过 `ecrecover` **直接校验签名**，只要授权参数被篡改或签名不合法，整笔转账就会回滚失败。

> 💡 **为什么不用 EIP-2612 (`permit`)？**
> 熟悉 Web3 的开发者可能会问：EIP-2612 也支持离线签名授权，为什么不用它？
> 因为 EIP-2612 的 `permit` 采用的是**递增序号（Sequential Nonce：0, 1, 2...）**。当一个 AI Agent 并发调用 5 个不同的 API 时，5 笔签名的 Nonce 必须严格排队上链；一旦第 1 笔因为网络原因卡住，后面 4 笔全部阻塞失效。
> 而 **EIP-3009 采用的是随机 32 字节 Nonce（Random 32-byte Nonce）**，各个签名之间完全独立，天生支持 AI Agent 的高并发异步调用！

> 这就是 [x402 流程](./http-402-and-x402.md)里"客户端签名 → 服务器验证 → Facilitator 结算"能成立的密码学基础。

---

## 4. EIP-712：让签名"看得懂"

如果只是对一串毫无规律的十六进制哈希（如 `0x8f3b...2a1c`）进行盲签（Blind Signing），人类用户或 Agent 的安全策略层根本看不出自己到底在签什么，极易被恶意服务端诱导签下倾家荡产的授权。

**EIP-712** 标准规定了**"类型化结构数据签名（Typed Structured Data Hashing and Signing）"**：把要签名的内容像 JSON 一样按**明确定义的字段和类型**组织起来，钱包或 Agent 的本地策略引擎可以逐字段解析、校验，确认无误后再放行签名。

### 4.1 核心字段解析

EIP-3009 授权的核心结构（`TransferWithAuthorization`）包含以下 6 个字段：

| 字段 | 类型 | 含义 | 安全与风控作用 |
|---|---|---|---|
| `from` | `address` | 付款人地址 | 链上恢复出的签名者地址必须严格等于它 |
| `to` | `address` | 收款人地址 | **锁死收款方**，Facilitator 无法在半路把收款人改成自己 |
| `value` | `uint256` | 转账金额（最小精度单位） | **锁死金额**（注意：USDC 为 6 位小数，`100000` 代表 `0.1 USDC`） |
| `validAfter` | `uint256` | 生效时间戳（Unix 秒） | 在此时间之前提交上链会直接报错，可用于预约生效 |
| `validBefore` | `uint256` | 过期时间戳（Unix 秒） | **有效时间窗**，过期授权在链上自动作废，防止"延迟扣款" |
| `nonce` | `bytes32` | 32 字节随机数 | **一次一用**，合约内部会记录已使用的 nonce，彻底防重放攻击 |

此外，EIP-712 强制要求包含一个**域分隔符（`domain` / Domain Separator）**，将签名与特定的 `chainId`（链 ID）和 `verifyingContract`（USDC 合约地址）死死绑定。这意味着：**Agent 为 Arbitrum Sepolia 测试网签出的授权，绝对无法被拿到 Base 主网上重放骗钱。**

### 4.2 实战示例：一份完整的 EIP-712 签名载荷

假设 Agent `0x1111...1111` 要在 Base 链（`chainId: 8453`）上向某个天气 API 服务商 `0x2222...2222` 支付 `0.05 USDC`。策略引擎在本地看到的待签名结构如下：

```json
{
  "types": {
    "EIP712Domain": [
      { "name": "name", "type": "string" },
      { "name": "version", "type": "string" },
      { "name": "chainId", "type": "uint256" },
      { "name": "verifyingContract", "type": "address" }
    ],
    "TransferWithAuthorization": [
      { "name": "from", "type": "address" },
      { "name": "to", "type": "address" },
      { "name": "value", "type": "uint256" },
      { "name": "validAfter", "type": "uint256" },
      { "name": "validBefore", "type": "uint256" },
      { "name": "nonce", "type": "bytes32" }
    ]
  },
  "primaryType": "TransferWithAuthorization",
  "domain": {
    "name": "USD Coin",
    "version": "2",
    "chainId": 8453,
    "verifyingContract": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
  },
  "message": {
    "from": "0x1111111111111111111111111111111111111111",
    "to": "0x2222222222222222222222222222222222222222",
    "value": "50000",
    "validAfter": 0,
    "validBefore": 1760000300,
    "nonce": "0xf3746613c2d920b5fdabc0856f2aeb2d4f88ee6037b8cc5d04a71a4462f13480"
  }
}
```

### 4.3 为什么 Facilitator（或恶意的 API 卖方）偷不走钱？

因为上述 JSON 中的 `to`（收款人）、`value`（金额 `50000`）等所有参数都参与了密码学哈希与私钥签名：

- **防篡改**：如果 Facilitator 试图把 `value` 从 `50000`（0.05 USDC）改成 `5000000`（5 USDC），或者把 `to` 改成黑客的地址，链上合约根据新参数和原签名恢复出的公钥地址就会变成一串乱码，与 `from` 不匹配，交易直接 Revert（回滚）。
- **防二次扣款**：如果卖方拿到这笔签名后，想在链上提交两次收双倍的钱，第二次提交时 USDC 合约会发现该 `nonce` 已被标记为 `used`，直接拒绝执行。

因此，Facilitator 只能"原样提交"，改一个字节签名就失效。最坏情况是它**不提交**（拒绝服务，买方也没损失钱），而不是**偷钱**。

---

## 5. 对 AgentPay 架构意味着什么

基于 EIP-712 和 EIP-3009 的特性，AgentPay 在设计安全与资金网关时确立了以下三条核心铁律：

### 5.1 私钥只在买方本地（零托管风险）
签名操作严格发生在买方自己的 `LocalSigner`（在生产环境中应接入 AWS KMS / 硬件安全模块 HSM）。无论是 AgentPay 网关、云端服务器还是 Facilitator，永远只接触签好名的授权凭证，绝对接触不到买方私钥。

### 5.2 签名前必须过防火墙，签名后不可轻信“超时”
在区块链世界里，**离线签名一旦生成并发送给外部网络，就等同于一张已经盖章开出的支票——在有效期内不可撤销**。因此 AgentPay 执行硬性风控规则：

1. **先冻结，后签名**：只有本地策略引擎校验通过（检查单笔限额、每日总预算、收款人白名单），并成功在数据库中产出 `ApprovedReservation`（预算预留锁）之后，才允许调用底层签名器。
2. **超时不等于失败，严禁立即释放预算**：签名通过 HTTP 发出后，即使上游 API 返回 `504 Gateway Timeout` 或 `500 Internal Server Error`，**也绝对不能立刻把预留的预算退回可用池**！

> **🚨 实战反例：为什么超时不能立刻释放预算？**
> 1. **12:00:00**：Agent 每日预算剩余 **0.10 USDC**。它调用某付费接口，签出了一笔 **0.10 USDC** 的授权（有效期至 **12:05:00**），可用预算变为 **0 USDC**。
> 2. **12:00:10**：上游服务网络抖动，HTTP 请求超时断开。但实际上，卖方后台已经收到了这笔签名，并正在往链上广播。
> 3. **错误做法**：如果 AgentPay 看到 HTTP 超时就立刻释放这 0.10 USDC 预算，Agent 会在 **12:00:15** 发起重试，签出**第二张 0.10 USDC** 的有效支票。随后卖方将两张支票全部提交上链，Agent 实际被扣款 **0.20 USDC**，直接击穿每日预算上限！
> 4. **AgentPay 正确做法**：遇到超时或 5xx，这笔预算会进入 `PENDING_EXPIRY`（等待过期）状态。系统必须等到该签名的有效期彻底结束（即满足 `当前时间 now > validBefore + 区块时间安全余量`），且向链上节点确认该 `nonce` 确实未被上链消耗后，才安全释放这笔预算（详见 [timeout-is-not-failure](../04-agentpay-concepts/timeout-is-not-failure.md)）。

### 5.3 Gas 由谁出，是个经济问题
虽然买方不付 Gas，但天上不会掉馅饼，Facilitator 垫付的 ETH Gas 终究是真实成本：
- 在 `v0.1.x` 阶段，得益于 Arbitrum / Base 等 L2 网络极低的交易成本（单笔 Gas 通常低于 $0.001），系统采用**"卖方 / Facilitator 全额代付吸收"**模式；
- 针对超高频、极微额（如 $0.0001 级别）的场景，动态 Gas 加价分摊与链下批量聚合结算（Batch Settlement）将留到后续版本深入探讨——详见 [micropayment-economics.md](./micropayment-economics.md)。

---

## 小结

- **告别双资产负担**：链上 Gas 必须用原生代币（ETH）支付，若走传统转账，会让成百上千个 Agent 钱包背上极其沉重的双资产运维负担。
- **无 Gas 与高并发**：EIP-3009 让买方离线签名、他人代付 Gas，且采用随机 Nonce 原生支持并发调用；EIP-712 则让签名内容结构化、可被策略引擎逐字段审查。
- **密码学防盗**：签名死死锁定了收款人、金额、有效时间窗、随机 Nonce 以及所在链与合约地址，**中继者无法篡改偷钱，最坏只能拒绝提交**。
- **风控前置原则**：签名一旦发出在过期前不可撤销，**因此安全防火墙必须在签名之前介入，且在网络超时场景下必须严格等待授权过期后才能释放预算**。

**下一篇**：[micropayment-economics.md](./micropayment-economics.md) —— 微支付的成本问题