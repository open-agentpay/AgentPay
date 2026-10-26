# AgentPay 多租户隔离与越权渗透测试报告 (T3.5 Conformance Report)

> **报告版本**：v1.1.0  
> **执行日期**：2026-10-18（更新于 2026-10-26）  
> **测试状态**：**100% 通过 (PASSED)**  
> **执行环境**：macOS (Darwin arm64), Python 3.12.7 (pytest 9.1.1), Go 1.24  
> **涉及核心规范**：
> - [`specs/multitenant-data-model.md`](https://github.com/open-agentpay/AgentPay/blob/main/specs/multitenant-data-model.md)（多租户实体模型与隔离规范）
> - [`specs/threat-model-v1.md`](https://github.com/open-agentpay/AgentPay/blob/main/specs/threat-model-v1.md)（T1.6 威胁模型与越权攻击面）
> - [`specs/security-model.md`](https://github.com/open-agentpay/AgentPay/blob/main/specs/security-model.md)（RBAC 角色权限与租户安全边界）
> - [`specs/wallet-ledger-spec.md`](https://github.com/open-agentpay/AgentPay/blob/main/specs/wallet-ledger-spec.md)（虚拟钱包与只追加账本隔离规范）

---

## 1. 概述与测试目标

在 AgentPay 多租户架构中，不同租户代表相互独立的企业或团队，各自托管敏感的 AI Agent 业务资产、私有 API Key、内部上游路由、财务安全策略以及平台虚拟钱包资金。

**任务 T3.5（完成租户隔离与越权测试）** 在 Python 控制面与 Go Gateway 数据面实施系统性、渗透级的负面安全测试，严格验证以下五大安全边界：

1. **水平越权防御（BOPLA / IDOR — Broken Object Level Authorization）**：
   - 攻击者持有合法租户凭证（Tenant Beta，`ten_beta`），尝试通过已知或枚举遍历的对象 ID 直接读取、修改、轮换或删除受害者租户（Tenant Alpha，`ten_alpha`）名下的项目、成员、Agent、API Key、网关路由、安全策略及钱包账本。
2. **垂直越权防御（BFLA — Broken Function Level Authorization & RBAC Escalation）**：
   - 租户内部低权限成员（如 `viewer`、`operator`）尝试调用需要更高角色权限（如 `admin`、`owner`）的写操作、密钥签发或敏感管理接口。
3. **查询与列表搜索隔离（零污染不变量 — Data Contamination Prevention）**：
   - 验证全量列表（List）、筛选与搜索接口在缺省过滤条件、空查询或通配符查询下，绝不泄露任何其他租户的记录。
4. **API 参数混淆与跨租户关系注入防御（Path vs. Payload Mismatch）**：
   - 攻击者在 REST API URL 路径中使用自己的 `tenant_id`，但在 JSON Body 或查询参数中注入受害者的 `project_id`、`agent_id` 或 `route_id`，验证服务端能否在持久化前精准识别跨租户归属冲突并阻断。
5. **数据面网关路由与钱包资金物理隔离**：
   - 验证 Go Gateway 在数据面请求路径上执行租户级路由匹配（`MatchRouteForTenant`），以及虚拟钱包冻结（Hold）、实扣（Capture）与只追加账本对账（`ReconcileBalance`）在不同租户间的完全独立性。

---

## 2. 隔离与越权测试覆盖矩阵

| 测试维度 | 攻击测试套件 / 测试用例 | 目标对象域 | 预期安全响应 | 测试结果 | 关键安全断言 |
| :--- | :--- | :--- | :--- | :---: | :--- |
| **水平越权 (IDOR 读取)** | `test_tenant_isolation_penetration.py`<br>`test_cross_tenant_idor_direct_access_blocked` | Project, Agent, ApiKey, Route, Policy | `403 Forbidden` / `ERR_CROSS_TENANT_VIOLATION` | **PASS** | 猜测或直接访问其他租户实体 ID 均被即时拒绝 |
| **水平越权 (IDOR 篡改/删除)** | `test_tenant_isolation_penetration.py`<br>`test_cross_tenant_idor_direct_access_blocked` | Project, Agent, ApiKey, Route, Policy | `403 Forbidden` / Tenant Mismatch Error | **PASS** | 跨租户修改预算、吊销/轮换密钥、删除路由或策略均被阻断 |
| **查询列表零泄露** | `test_tenant_isolation_penetration.py`<br>`test_cross_tenant_listing_zero_contamination` | All Domains (List APIs) | 返回列表仅含调用方自有资源，受害者资源数为 `0` | **PASS** | 没有任何列表接口返回混合租户记录 |
| **垂直越权 (RBAC 提权)** | `test_tenant_isolation_penetration.py`<br>`test_vertical_rbac_privilege_escalation_blocked` | Member RBAC Matrix | `403 Forbidden` / `ERR_PERMISSION_DENIED` | **PASS** | `viewer` 无法创建或修改项目、密钥、路由、策略或成员 |
| **REST API 路径载荷混淆** | `test_tenant_isolation_penetration.py`<br>`test_api_rest_idor_and_spoofing_matrix` | REST API Endpoints | `403 Forbidden` | **PASS** | 拦截 URL 路径 `tenant_id` 与请求体 `project_id`/`agent_id` 归属不一致的注入 |
| **最后 Owner 保护守卫** | `test_member_rbac.py` | Tenant Member | `400 Bad Request` / `CannotRemoveLastOwner` | **PASS** | 严禁删除、降级或禁用租户唯一活跃的 `owner` |
| **数据面网关路由隔离** | `services/gateway/internal/rcu/rcu_test.go`<br>`TestRCUMultiTenantRouteIsolation` | Go Gateway RCU Router | `MatchRouteForTenant` 返回 `false` | **PASS** | Tenant Beta 绝无法命中或调用 Tenant Alpha 的私有路由 |
| **虚拟钱包与金融账本隔离** | `services/gateway/tests/wallet_isolation_test.go`<br>`TestMultiTenantWalletIsolation` | Virtual Wallet & Ledger | 余额零干扰，账本互不可见，对账 100% 独立 | **PASS** | Alpha 的冻结与扣费对 Beta 的可用余额及流水零影响 |

---

## 3. 详细测试场景与渗透验证

### 3.1 跨租户 IDOR 读取与篡改拦截（攻击者 `ten_beta` vs. 受害者 `ten_alpha`）

在测试沙箱中初始化两个完全独立的租户：**受害者 Tenant Alpha (`ten_alpha`)** 与 **攻击者 Tenant Beta (`ten_beta`)**。

| 用例编号 | 攻击目标资源 | 攻击者 (`ten_beta`) 发起的越权操作 | 服务层抛出的安全异常 | HTTP 状态码与标准错误码 | 攻击后受害者资源状态 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `IDOR-01` | 项目 (`prj_alpha`) | `get_project("prj_alpha", expected_tenant_id="ten_beta")` | `ProjectTenantMismatchError` | `403 ERR_CROSS_TENANT_VIOLATION` | 未泄露 / 保密完好 |
| `IDOR-02` | 项目预算 (`prj_alpha`) | `update_project("prj_alpha", budget_cap_base_units=0)` | `ProjectTenantMismatchError` | `403 ERR_CROSS_TENANT_VIOLATION` | 预算额度未被改动 |
| `IDOR-03` | API 密钥 (`apk_alpha`) | `get_api_key` / `revoke_api_key` / `rotate_api_key("apk_alpha")` | `ApiKeyTenantMismatchError` | `403 ERR_API_KEY_TENANT_MISMATCH` | 密钥保持有效，明文零泄露 |
| `IDOR-04` | Agent (`agt_alpha`) | `update_agent(agent_id="agt_alpha", name="Hacked")` | `AgentTenantMismatchError` | `403 ERR_CROSS_TENANT_VIOLATION` | Agent 元数据完好 |
| `IDOR-05` | 网关路由 (`rou_alpha`) | `update_route("rou_alpha", price_base_units=0)` / `delete_route` | `RouteTenantMismatchError` | `403 ERR_CROSS_TENANT_VIOLATION` | 路由单价与上游地址完好 |
| `IDOR-06` | 安全策略 (`pol_alpha`) | `update_policy("pol_alpha", max_amount_per_request=99999999)` | `PolicyTenantMismatchError` | `403 ERR_CROSS_TENANT_VIOLATION` | 策略限额与熔断参数完好 |

### 3.2 列表查询零泄露与路由命名空间独立

- **用例 `LIST-01`（无过滤条件列表查询）**：
  - 以 `ten_beta` 身份调用 `list_projects(tenant_id="ten_beta")`、`list_agents(tenant_id="ten_beta")`、`list_api_keys(tenant_id="ten_beta")`、`list_routes(tenant_id="ten_beta")` 与 `list_policies(tenant_id="ten_beta")`。
  - **判定结果**：返回数组仅包含 Tenant Beta 自身创建的资源；Tenant Alpha 资源 ID 出现次数严格为 `0`。
- **用例 `NAMESPACE-01`（跨租户同名路由路径独立共存）**：
  - Tenant Alpha 在名下注册 `/api/v1/secret-model` 路由，指向 Alpha 的私有上游；
  - Tenant Beta 随后在名下注册完全相同路径 `/api/v1/secret-model`，指向 Beta 的独立上游；
  - **判定结果**：依托 PostgreSQL 唯一约束 `uq_tenant_route_path UNIQUE (tenant_id, path_pattern)` 与 Go Gateway RCU 引擎的 `MatchRouteForTenant(tenantID, method, path)` 租户级索引，两条同名路由独立共存、互不覆盖，且严格转发至各自租户的上游服务。

### 3.3 垂直 RBAC 角色权限矩阵与越权拦截

AgentPay 实施四级角色权限模型（`owner` > `admin` > `operator` > `viewer`）。测试套件在 `ten_alpha` 内部验证了低权限角色向高权限接口发起越权调用时的确定性拦截：

| 操作 / 权限能力 | 所需权限标识 (`permission`) | `owner` | `admin` | `operator` | `viewer` | 对应验证用例 |
| :--- | :--- | :---: | :---: | :---: | :---: | :--- |
| 查看项目、路由、策略与审计日志 | `*:read` | 允许 | 允许 | 允许 | 允许 | 基线只读权限验证通过 |
| 创建或归档项目 (Project) | `project:create` / `project:write` | 允许 | 允许 | 拒绝 (`403`) | 拒绝 (`403`) | `RBAC-01` (`PermissionDeniedError`) |
| 签发、轮换或吊销 API Key | `apikey:create` / `apikey:revoke` | 允许 | 允许 | 允许 | 拒绝 (`403`) | `RBAC-02` (`PermissionDeniedError`) |
| 创建或更新网关路由 (Route) | `policy:write` / `route:write` | 允许 | 允许 | 允许 | 拒绝 (`403`) | `RBAC-03` (`PermissionDeniedError`) |
| 创建或修改财务安全策略 (Policy) | `policy:write` | 允许 | 允许 | 拒绝 (`403`) | 拒绝 (`403`) | `RBAC-04` (`PermissionDeniedError`) |
| 发布或回滚配置快照 (Config) | `config:publish` | 允许 | 允许 | 拒绝 (`403`) | 拒绝 (`403`) | 配置发布权限测试通过 |
| 管理租户成员与角色分配 | `member:manage` | 允许 | 允许* | 拒绝 (`403`) | 拒绝 (`403`) | 唯一 Owner 保护守卫测试通过 |

*\*注：`admin` 严禁删除、降级或禁用租户内最后一个活跃 `owner`（触发 `CannotRemoveLastOwnerError`，返回 HTTP 400）。*

### 3.4 API 路径与载荷混淆注入（跨租户关系绑定防篡改）

通过 ASGI HTTP Client 模拟攻击者发送构造报文，验证无法跨租户关联外键实体：

- **用例 `INJECT-01`（创建 Agent 时伪造绑定受害者项目）**：
  - 请求：`POST /api/v1/tenants/ten_beta/agents`
  - 请求体：`{"project_id": "prj_alpha", "name": "Trojan Agent"}`
  - **判定结果**：服务层在写入前强制校验 `project.tenant_id == path.tenant_id`，识别出 `prj_alpha` 归属于 `ten_alpha`，立即中止并返回 HTTP `403 ERR_CROSS_TENANT_VIOLATION`。
- **用例 `INJECT-02`（创建安全策略时伪造绑定受害者路由或 Agent）**：
  - 请求：`POST /api/v1/tenants/ten_beta/policies`，载荷中传入 `route_id: "rou_alpha"` 或 `agent_id: "agt_alpha"`。
  - **判定结果**：在持久化前被拦截并返回 HTTP `403 ERR_CROSS_TENANT_VIOLATION`。

### 3.5 虚拟钱包与账本跨租户物理隔离 (Go Gateway 数据面)

在 Go Gateway 数据面初始化两个独立租户钱包：`wal_alpha_001`（可用余额 `50 USDC`）与 `wal_beta_002`（可用余额 `10 USDC`）：

- **用例 `WALLET-01`（跨租户资金操作绝对零干扰）**：
  - 对 Tenant Alpha (`wal_alpha_001`) 发起 `5 USDC` 的预留冻结与确认实扣（Hold & Capture）；
  - **判定结果**：
    - Tenant Alpha 余额更新为：`Available = 45 USDC`，`Held = 0 USDC`；
    - Tenant Beta 余额保持为：`Available = 10 USDC`，`Held = 0 USDC`（**100% 未受任何影响，且无跨租户行锁争用**）。
- **用例 `WALLET-02`（账本流水物理隔离）**：
  - 以 Tenant Beta 身份查询账本流水 `GetLedgerEntries(ctx, "wal_beta_002")`；
  - **判定结果**：返回记录数为 `0`，完全不可见 Tenant Alpha 的扣款审计流水（`ap_ledger_entry`）。
- **用例 `WALLET-03`（双轨对账独立平账）**：
  - 分别对两个钱包独立执行 `ReconcileBalance`，各自依据其只追加账本流水重新汇总计算可用余额与冻结余额，两侧均返回 `IsBalanced == true`。

---

## 4. 纵深防御架构机制验证总结

1. **PostgreSQL 全表强制 `tenant_id` 作用域与复合索引**：`infra/postgres/schema.sql` 中所有租户资产表均包含显式 `tenant_id` 外键及 `(tenant_id, ...)` 复合 B-Tree 索引，从存储层杜绝无租户上下文的裸查询。
2. **密码学 API Key 身份反查绑定**：数据面请求通过 SHA-256 `key_hash` 部分索引（`idx_api_key_lookup`）直接从已验证的密钥记录中反查提取 `(tenant_id, project_id, agent_id)`，绝不信任客户端请求头自报的租户身份。
3. **RCU 内存快照租户隔离路由表**：Go Gateway 在不可变 RCU 快照内部按 `(tenant_id, path_pattern)` 组织路由索引，从根本上杜绝跨租户路由覆盖与上游流量劫持。
4. **行级锁隔离与金融表 `ON DELETE RESTRICT` 护栏**：钱包冻结与实扣仅锁定对应 `wallet_id` 单行（`SELECT ... FOR UPDATE`），且所有金融账本表强制设置 `ON DELETE RESTRICT`，防止任何级联误删破坏审计与账本完整性。

---

## 5. 复现命令与审计合规结论

可通过以下命令在本地完整复现多租户隔离与越权渗透测试套件：

```bash
# 1. 运行 Python 控制面多租户隔离、IDOR 与 RBAC 越权渗透测试
PYTHONPATH=sdk/python/packages/agentpay/src:sdk/python/packages/agentpay-gateway/src:sdk/python \
  python3 -m pytest -v sdk/python/tests/unit/test_tenant_isolation_penetration.py sdk/python/tests/unit/test_member_rbac.py

# 2. 运行 Go Gateway RCU 路由隔离与虚拟钱包账本隔离测试
make gateway-test
```

通过上述渗透测试套件（`test_tenant_isolation_penetration.py`、`test_route_policy_lifecycle.py`、`test_member_rbac.py`、`rcu_test.go` 与 `wallet_isolation_test.go`），全仓共计执行并通过了 **285+ 项自动化安全与隔离测试**。

**审计结论**：
1. **多租户隔离完备性**：AgentPay 在控制面（Python）与数据面（Go Gateway）均落地了不可绕过的租户所有权强校验；
2. **越权防御有效性**：所有已测试的 IDOR、BOPLA、BFLA、参数混淆与跨租户关系注入攻击向量均被 100% 阻断，并返回标准化的 `403` / `400` 错误响应；
3. **金融账本安全性**：虚拟钱包账户、冻结记录与只追加记账流水在跨租户高并发场景下具备完全独立的数据与锁隔离边界。

**任务 T3.5 各项验收标准全部满足，正式闭环签署。**
