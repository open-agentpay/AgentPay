# ADR-002: 数据库选型全面升级至 PostgreSQL

> **状态**：**已采纳 (Accepted)**  
> **决策者**：AgentPay 架构团队 & 业务主管  
> **日期**：2026-10-18（更新于 2026-10-26）  
> **前置任务**：T1.2 设计多租户逻辑数据模型  
> **废弃/演进**：取代 Phase 0 仅依赖 SQLite 单机本地文件的做法  
> **关联规范**：
> - [`specs/multitenant-data-model.md`](./multitenant-data-model.md)（多租户逻辑数据模型规范）
> - [`specs/wallet-ledger-spec.md`](./wallet-ledger-spec.md)（虚拟钱包与只追加账本规范）
> - [`infra/postgres/schema.sql`](../../infra/postgres/schema.sql)（PostgreSQL 权威 DDL 脚本）

---

## 1. 上下文与问题陈述 (Context & Problem Statement)

在 AgentPay Phase 0（Python 最小原型闭环阶段），系统为了追求极简的本地零依赖快速验证，采用了 Python 标准库内置的 SQLite（`sqlite3`）结合 WAL（`Write-Ahead Logging`）模式和 `BEGIN IMMEDIATE` 悲观锁。

然而，随着 AgentPay 正式向**生产级多租户支付网关、Go 云原生高并发数据面网关、Web 管理控制台以及双轨虚拟钱包账本（T1.7）**演进，基于单机嵌入式文件的数据层面临根本性的架构瓶颈：

1. **多节点无状态集群水平扩展**：Go Gateway 数据面与 Python/Web 控制面均需在多实例（Kubernetes Pod / 容器副本）下无状态部署，以实现高可用与弹性横向扩容。SQLite 依赖本地磁盘的操作系统文件锁，无法在分布式容器集群中跨节点安全地共享并发读写。
2. **虚拟钱包的高并发行级锁要求**：平台内部虚拟钱包引入了资金两阶段预留冻结（`Hold`）、确权实扣（`Capture`）与失败释放（`Release`）。SQLite 在写入事务时执行粗粒度的库级写锁（Database-level Lock），高并发下极易触发 `sqlite3.OperationalError: database is locked`。而金融级记账要求不同租户、乃至同一租户下的独立钱包操作互不阻塞，必须依赖成熟的**行级锁（Row-level Locking，如 `SELECT ... FOR UPDATE`）**。
3. **跨网关实例的挑战与随机数防重放（Anti-Replay）**：在多副本 Go Gateway 部署下，虚拟钱包挑战状态（`ap_wallet_challenge`）与链上 EIP-3009 结算随机数（`ap_settlement_record`）必须在所有网关实例间原子申领与核销，防止客户端重试路由到其他网关副本时绕过单次消费限制。
4. **海量审计事件与只追加流水分区存储**：网关请求追踪（`ap_request_record`）、Agent 行为时间线（`ap_agent_audit_event`）、链上结算明细（`ap_settlement_record`）与复式记账流水（`ap_ledger_entry`）均属于高频只追加（Append-Only）时间序列数据，需要部分索引（Partial B-Tree Index）、复合唯一约束及原生表分区能力。
5. **不可变配置版本快照与 JSONB 高性能检索**：路由规则（`ap_route`）与安全策略（`ap_policy`）的草稿发布、RCU 内存热切换与一键回滚依赖不可变快照存储（`ap_config_version.snapshot_json`），PostgreSQL 的二进制 `JSONB` 存储与 GIN 倒排索引在此场景下具备显著优势。

---

## 2. 决策结果 (Decision)

我们正式决定：**将 AgentPay 的生产核心关系型与事务存储全面选型为 PostgreSQL（15+，生产环境与 Docker Compose 标准基线为 PostgreSQL 16）**。

### 2.1 核心落地架构规范

1. **全栈统一数据中心**：
   - Python 控制面管理 API、数据面 Go Gateway、Go Facilitator 结算服务、虚拟钱包与账本引擎、以及消费审计与自动对账子系统统一接入 PostgreSQL 实例或主从高可用集群（权威 Schema 定义见 `infra/postgres/schema.sql`）。
2. **领域化 Schema 划分（5 大业务域、18 张核心表）**：
   - **身份、多租户与 RBAC 域**：`ap_tenant`、`ap_project`、`ap_member`、`ap_agent`、`ap_api_key`（对活跃 `key_hash` 建立部分索引）。
   - **路由、策略与配置版本域**：`ap_route`、`api_auth_configs`（上游接口加密凭据）、`ap_policy`、`ap_config_version`（不可变 `JSONB` 配置快照）。
   - **虚拟钱包与只追加账本域**：`ap_wallet`、`ap_ledger_entry`、`ap_hold`、`ap_topup_order`。
   - **支付意图、挑战、链上结算与履约域**：`ap_payment_intent`、`ap_wallet_challenge`、`ap_settlement_record`、`ap_fulfillment_record`。
   - **请求追踪、计量统计与安全审计域**：`ap_request_record`、`ap_usage_event`、`ap_audit_event`、`ap_agent_audit_event`。
3. **强事务隔离与金融级行级锁不变量**：
   - 所有货币与代币金额统一使用精确整数最小单位（`BIGINT` Base Units，例如 `1 USDC = 1,000,000` base units），从根源杜绝浮点数精度丢失。
   - 钱包资金变动（`Hold` / `Capture` / `Release` / `TopUp`）必须在显式 ACID 事务中对目标钱包行加排他锁：
     ```sql
     SELECT wallet_id, status, version, available_cache, held_cache
     FROM ap_wallet
     WHERE wallet_id = $1
     FOR UPDATE;
     ```
   - 在存储引擎层通过 `CHECK` 约束强制守护账户余额不变量（如 `CHECK (available_cache >= 0 OR status = 'frozen_arrears')` 与 `CHECK (held_cache >= 0)`），即使应用层出现极端并发竞态，数据库底层也会拒绝任何导致非法负余额的写入。
4. **连接池与高并发超时治理**：
   - **Go Gateway 数据面**：采用 `jackc/pgx/v5/pgxpool` 原生二进制协议连接池，严格配置最大/最小连接数、健康检查，以及事务级 `lock_timeout`（如 `3000ms`）与 `statement_timeout`（如 `5000ms`），防止慢查询阻塞网关协程池。
   - **Python 控制面**：采用 `psycopg` / `asyncpg` 异步连接池，配合预编译语句缓存与租户级过滤守卫。
5. **本地开发与分层测试策略**：
   - 仓库提供标准的 `docker-compose.yml` 与 `docker-compose.prod.yml`，基于 `postgres:16-alpine` 自动挂载 `infra/postgres/schema.sql` 初始化并配置 `pg_isready` 健康检查。
   - 纯算法与密码学单元测试（如 x402 标头编解码、EIP-712/EIP-3009 签名校验、状态机转移矩阵）保持零外部依赖以维持毫秒级反馈；多租户隔离、钱包并发记账与跨语言端到端一致性测试统一通过 PostgreSQL 验证。

---

## 3. 备选方案对比与评估 (Options Considered)

| 评估维度 | SQLite 3 (Phase 0 原方案) | MySQL 8.0+ (InnoDB) | **PostgreSQL 16 (最终选择)** |
| :--- | :--- | :--- | :--- |
| **并发锁粒度** | 库级写锁（全局单写串行） | 行级锁（InnoDB 索引记录锁） | **行级锁（MVCC + `FOR UPDATE` / `SKIP LOCKED` 严密事务）** |
| **多节点集群共享** | 不支持（仅限单机本地文件锁） | 原生支持主从与集群访问 | **原生支持云原生高可用（Patroni / CloudNativePG / 读写分离 / PgBouncer）** |
| **半结构化数据 (`JSONB`)** | 基础 `JSON1` 文本函数 | `JSON` 类型（索引能力有限） | **二进制优化的 `JSONB`，支持 GIN 倒排索引与高效路径检索** |
| **金融不变量与高级索引** | 基础 `CHECK` 约束，部分索引受限 | 8.0.16+ 支持 `CHECK`，不支持原生部分索引 | **完备支持 `CHECK` 约束、部分索引（Partial Index）与 `ON DELETE RESTRICT` 护栏** |
| **跨语言驱动吞吐性能** | 需内嵌 C 绑定（Go 侧有 CGo 开销） | 驱动成熟（`go-sql-driver`, `aiomysql`） | **Go (`pgx/v5` 零 CGo 二进制协议) 与 Python (`psycopg3`/`asyncpg`) 极高吞吐** |
| **云生态与运维成熟度** | 仅适合嵌入式或本地单机工具 | 云厂商广泛支持 | **金融账本与 AI 基础设施首选（RDS、Supabase、Neon、CloudSQL、Docker）** |

---

## 4. 核心技术实现模式 (Technical Architecture & Key Patterns)

### 4.1 虚拟钱包两阶段原子记账 (`Hold -> Capture / Release`)

```sql
BEGIN;

-- 1. 仅锁定当前租户的单个钱包行（不同租户之间完全零锁争用）
SELECT wallet_id, available_cache, held_cache, status
FROM ap_wallet
WHERE wallet_id = $1 AND tenant_id = $2
FOR UPDATE;

-- 2. 插入与 PaymentIntent 唯一绑定的冻结记录
INSERT INTO ap_hold (hold_id, wallet_id, payment_intent_id, amount, status, expires_at)
VALUES ($3, $1, $4, $5, 'held', CURRENT_TIMESTAMP + INTERVAL '120 seconds');

-- 3. 原子扣减可用余额并转入冻结余额（受数据库 CHECK 约束保护）
UPDATE ap_wallet
SET available_cache = available_cache - $5,
    held_cache      = held_cache + $5,
    version         = version + 1
WHERE wallet_id = $1;

COMMIT;
```

### 4.2 面向网关热路径的部分索引 (Partial Indexes)

为确保历史流水增长至千万级后，网关数据面的密钥鉴权、防重放校验与过期冻结回收耗时仍保持在 `1 ms` 以内，`infra/postgres/schema.sql` 广泛采用 PostgreSQL **部分索引（Partial Indexes）**：

- **活跃 API Key 极速鉴权**：`CREATE INDEX idx_api_key_lookup ON ap_api_key(key_hash) WHERE status = 'active';`
- **活跃 Hold 过期扫描**：`CREATE INDEX idx_hold_active_expires ON ap_hold(wallet_id, expires_at) WHERE status = 'held';`
- **未完结支付意图恢复巡检**：`CREATE INDEX idx_intent_active_status ON ap_payment_intent(status) WHERE status NOT IN ('completed', 'failed', 'rejected', 'expired');`
- **链上待确认/未知状态结算轮询**：`CREATE INDEX idx_settlement_pending_unknown ON ap_settlement_record(status) WHERE status IN ('submitted', 'pending', 'unknown');`

### 4.3 引用完整性与防篡改硬约束

- **金融核心表强制 `ON DELETE RESTRICT`**：`ap_wallet`、`ap_ledger_entry`、`ap_hold`、`ap_topup_order` 与 `ap_wallet_challenge` 的外键全部采用 `ON DELETE RESTRICT`，从数据库层严禁任何产生过资金流水或钱包账户的租户被级联误删。
- **全局幂等与防重放唯一约束**：
  - `uq_wallet_ledger_idempotency UNIQUE (wallet_id, idempotency_key)` 杜绝重复入账或重复扣款；
  - `uq_nonce_replay_guard UNIQUE (network_id, asset_address, payer_address, nonce)` 杜绝链上 EIP-3009 签名重放；
  - `uq_wallet_challenge_nonce UNIQUE (wallet_id, nonce)` 确保虚拟钱包挑战在多网关副本间严格单次核销。

---

## 5. 后续影响、风险与缓解措施 (Consequences & Mitigations)

### 5.1 正向收益
- **彻底解除高并发吞吐瓶颈**：消除 SQLite 单写锁限制，支撑多副本 Go Gateway 集群每秒数万次鉴权与记账读写。
- **可证明的金融账本一致性**：借助 PostgreSQL ACID 事务、`FOR UPDATE` 行锁、`BIGINT` 整数基本单位及 `CHECK`/`UNIQUE` 硬约束，实现零双花与 100% 确定性的日终对账（`ReconcileBalance`）。
- **配置零停机热切换**：`ap_config_version` 持久化带 SHA-256 指纹的不可变 `JSONB` 快照，支撑网关 RCU（Read-Copy-Update）无锁热更新与秒级回滚。

### 5.2 潜在风险与缓解措施
- **风险 — 突发流量下的数据库连接数耗尽**：多网关实例高并发可能占满 PostgreSQL `max_connections`。
  - *缓解措施*：在各网关实例中严格限制 `pgxpool` 最大连接数与空闲回收周期，生产环境支持前置 PgBouncer 事务级连接池。
- **风险 — 单一超高频租户钱包的热点行锁争用**：虽然不同租户完全并行，但单一租户对同一个 `wallet_id` 发起极端并发扣费时会在该行上排队。
  - *缓解措施*：严格遵循“短事务锁”原则——`Hold` 预留事务仅包含内存/索引级 SQL 操作（耗时 `< 1 ms`），绝不在持有数据库事务锁期间发起上游 HTTP 调用；上游履约完成后再开启第二个短事务执行 `Capture` 或 `Release`。
- **风险 — 本地开发启动门槛增加**：相比单文件 SQLite，本地开发需要运行 PostgreSQL 服务。
  - *缓解措施*：通过 `make compose-up`（`docker-compose.yml`）提供一键拉起与自动建表，并在测试套件中保留内存级 Repository 适配器以支持快速离线单元测试。
