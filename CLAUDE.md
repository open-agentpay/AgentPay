# CLAUDE.md — AgentPay

本文件是 Claude Code 在本仓库工作的最高优先级约定。开始任何任务前先读完。
与用户沟通使用中文；代码、标识符、注释、commit message、文档字符串使用英文。

## 1. 项目是什么

AgentPay 是面向自主 AI Agent 的开源支付基础设施与“财务防火墙”，基于 HTTP 402 / x402 V2 双轨架构（链上 EIP-3009 无 Gas 结算 + 企业内部预付虚拟账本）。
核心价值有四点：

1. **独立策略引擎（Financial Firewall）**：LLM 只能提议调用，永远不能自己批准预算（单笔上限、24h 滚动预算、收款方/资产/网络/域名白名单、RPM 熔断器）。
2. **三态正交解耦状态机**：支付意图（`PaymentIntent`）、资金结算（`Settlement`）、业务交付（`Fulfillment`）分开建模；网络超时不等于失败；通过 `unknown` 状态、幂等锁与后台对账（`ReconcileSettlements` / `SweepExpiredHolds`）保证不重复扣款、不静默吞钱。
3. **双轨自托管网关（Go Cloud-Native Gateway）**：支持非托管链上 x402（`EIP-3009`）与企业预付虚拟钱包（`agentpay-wallet` 两阶段 `Hold -> Fulfill -> Capture/Release`），支持 RCU 零停机配置热更新、多租户 RBAC、上游凭证 AES-256-GCM 版本化加密与 SSRF 防护。
4. **全链路客观行为审计**：严格区分网关观测事实（Observed）与 Agent 自报上下文（Declared），提供多租户审计事件流与行为时间线。

## 2. 当前仓库已实现范围

当前 Monorepo 已包含并维护以下生产与开发组件：

- **Go 生产网关 (`services/gateway`)**：x402 V2 双轨处理器、RCU 无锁热配置、虚拟钱包双式记账账本、JWT + 租户隔离 RBAC 管理平面、上游鉴权加密（支持密钥轮换）与 SSRF 防护、PostgreSQL 与内存双模式存储。
- **Go Facilitator 服务 (`services/facilitator`)**：EIP-712 / EIP-3009 签名验证、调用方 API Key 鉴权、收款地址白名单、Gas 余额监控告警、EIP-1559 动态 Gas 缓冲广播与链上回执轮询对账。
- **Python SDK 与参考实现 (`sdk/python`)**：
  - `packages/agentpay`：买方 SDK（`SQLitePolicyEngine`、x402 编解码、EIP-712/3009 签名、`httpx` 自动 402 拦截器、LangChain `@paid_tool` 集成）。
  - `packages/agentpay-gateway`：轻量级 Python 参考网关与本地模拟环境。
- **TypeScript SDK (`sdk/typescript`, `@open-agentpay/sdk`)**：双轨买方客户端（Wallet + On-Chain）、本地策略引擎、Express / Fastify / Next.js 卖方中间件、MCP 工具包装器。
- **多租户管理控制台 (`apps/console`)**：Vue 3 + Element Plus + Pinia 控制台前端。
- **API 市场与文档门户 (`apps/marketplace-web`, `docs`)**：Next.js 15 公开路由市场与 VitePress 双语文档站。
- **跨语言规范与测试向量 (`specs/`)**：架构、状态机、账本、审计规范及 `specs/conformance/test-vectors` 语言无关测试向量。

尚未实现（处于 `Planned / 规划中` 阶段）：飞书/企微/钉钉 HITL 审批卡片运行时、Langfuse 集成插件、国内法币第三方支付（微信/支付宝/对公水单）自动入金对接。

## 3. 目录结构

```text
specs/                              规范与跨语言测试向量（事实来源）
  conformance/test-vectors/         JSON 协议与签名测试向量
services/
  gateway/                          Go 云原生网关服务
  facilitator/                      Go 链上验证与结算协调器服务
apps/
  console/                          多租户管理控制台前端 (Vue 3 + Vite + Vitest)
  marketplace-web/                  公开 API 市场与演示前端 (Next.js 15)
sdk/
  python/
    packages/agentpay/              Python 买方核心 SDK 与防火墙
    packages/agentpay-gateway/      Python 参考网关
  typescript/                       TypeScript 买方/卖方 SDK (@open-agentpay/sdk)
infra/                              PostgreSQL 初始化 SQL 与部署配置
examples/                           端到端示例与失控智能体熔断 Demo
```

## 4. 技术栈与常用命令

Python 3.10+，Go 1.22+，Node.js 20+ (pnpm)，SQLite / PostgreSQL，httpx，pytest，ruff，vitest。
密码学一律使用成熟库（Python `eth-account`、Go `go-ethereum/crypto`、TS `viem`），**严禁自行实现底层椭圆曲线或对称加密算法**。

```bash
# 1. Python 代码检查与测试
ruff check sdk/python/
ruff format --check sdk/python/
pytest sdk/python/tests -v
python3 scripts/validate_vectors.py specs/conformance/test-vectors

# 2. Go 网关与 Facilitator 构建及测试
make gateway-build
make gateway-test
make facilitator-test

# 3. TypeScript SDK 与前端控制台测试
pnpm --prefix sdk/typescript test
pnpm --prefix apps/console test

# 4. 失控 Agent 熔断演示
make demo
```

提交前必须确保被修改模块及跨语言一致性测试全部通过。

## 5. 事实来源与优先级

冲突时按下列顺序，**不要静默选择一方，发现冲突立即停下来告诉用户**：

1. 用户当前指令
2. 本文件（`CLAUDE.md`）
3. `specs/` 下的规范与测试向量
4. 其他文档（`README.md`、`README_zh.md`、`docs/` 等）

## 6. 安全红线（不可违反）

资金、鉴权与多租户相关代码必须严格遵守以下红线：

**金额与数据类型**
- 金额一律使用 6 位微单位整数（`int64` / Python `int` / TS `bigint`），**严禁使用 float 处理金额**；JSON 传输中 uint256 或大整数金额使用十进制字符串。
- SQLite 金额列使用 `INTEGER` 并加 `CHECK (typeof(col)='integer')`。
- EVM 地址与 32 字节 hex nonce 统一归一化为小写（lowercase）存储、索引和比较；网络标识符使用 CAIP-2（如 `eip155:84532`）。

**管理平面鉴权与多租户隔离（P0-1）**
- 除显式公开接口（`/api/v1/auth/login`、`/api/v1/auth/register`、`/api/v1/public/marketplace/*`、`/healthz`、`/readyz`、`/version`）外，所有 `/api/v1/*` 接口必须强制校验 JWT。
- 必须在认证中间件入口剥离客户端伪造的 `X-Tenant-Id` 与 `X-Actor-Member-Id` 请求头，身份与租户上下文仅能由已验签的 JWT Claims 注入。
- Service 层（Memory 与 Postgres）在 `actorMemberID == ""` 且非显式系统上下文（`auth.WithSystemContext`）时必须默认拒绝（返回 `ErrPermissionDenied`），严禁空身份跳过 RBAC。

**主密钥与上游鉴权安全（P0-2 & P1-1）**
- 网关启动必须配置 `auth.jwt_secret`（或 `AGENTPAY_JWT_SECRET`）与 `auth.upstream_master_key`（或 `AGENTPAY_UPSTREAM_MASTER_KEY`），严禁回退到硬编码默认密钥（Fail-Closed）。
- 上游 URL 配置、连通性测试（`test-upstream`）与反向代理拨号（`SafeDialContext`）必须校验并拦截回环地址、RFC1918 私网、链路本地元数据地址（`169.254.169.254`）、CGNAT 等内网 IP，防止 SSRF 与 DNS 重绑定攻击。

**策略引擎、Nonce 与链上对账（P0-3, P0-4, P0-5）**
- 默认拒绝（Fail-Closed）：数据库锁失败、序列化错误或未处理异常一律拒绝放行。
- Nonce 必须持久化去重（`UNIQUE(network_id, asset_address, payer_address, nonce)`），地址与 Nonce 入库前必须转小写，防止大小写变形重放与网关重启重放。
- Facilitator `/v1/settle` 返回 `202 submitted` 仅代表交易已广播，网关严禁立即将结算标记为 `confirmed`；必须保持 `submitted`/`pending`，由后台对账任务根据链上回执更新为 `confirmed` 或 `failed`。
- Facilitator 写接口必须启用 API Key 鉴权、在 `Settle` 阶段二次完整验证 EIP-712 签名，并监控代付账户 Gas 余额。收款方由各商户路由自主配置，不对收款方做硬编码限制。

**机密与日志**
- 私钥、助记词、主密钥、上游凭证明文绝不出现在日志、异常信息或 HTTP 响应里。
- 测试私钥仅限公开的 Hardhat/Anvil 测试账户，严禁用于主网。

## 7. 必须人工审查的代码

下列区域的改动，完成后需明确说明并在关键路径添加 `# SECURITY-REVIEW` 或 `// SECURITY-REVIEW` 注释：

- 签名构造与验证、Nonce 归一化与持久化、`valid_before` 判断
- 预留 / 冻结（Hold）/ 扣费（Capture）/ 释放（Release）及过期冻结回收
- 链上结算状态迁移与后台对账（`ReconcileSettlements`）
- 管理 API 认证中间件、多租户越权校验与 RBAC 权限检查
- 网关上游转发、SSRF 拨号校验与上游密钥加解密轮换
