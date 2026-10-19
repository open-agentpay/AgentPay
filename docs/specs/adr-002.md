# ADR-002: 数据库选型全面升级至 PostgreSQL

> **状态**：**已采纳 (Accepted)**  
> **决策者**：AgentPay 架构团队 & 业务主管  
> **日期**：2026-10-18  
> **前置任务**：T1.2 设计多租户逻辑数据模型  
> **废弃/演进**：取代 Phase 0 仅依赖 SQLite 单机本地文件的做法  

---

## 1. 上下文与背景 (Context)

在 AgentPay Phase 0（Python 最小原型闭环阶段），系统主要为了追求极简的本地零依赖快速验证，采用了标准库的 SQLite（`sqlite3`）结合 WAL 模式和 `BEGIN IMMEDIATE` 悲观锁。

然而，随着项目正式向 **多租户支付网关、Go 云原生高并发网关、Web 管理控制台与平台内部虚拟钱包账本（T1.7）** 演进，数据层面临根本性的架构挑战：

1. **多节点与集群水平扩展**：Go Gateway 数据面与 Web 控制面需要部署多个实例（Pod）以实现高可用和无缝横向扩容。SQLite 基于本地文件系统，无法在无状态分布式容器集群中跨节点共享读写；
2. **虚拟钱包的高并发行级锁要求**：平台内部虚拟钱包引入了资金冻结（Hold）与确权扣款（Capture）。SQLite 在写入事务时执行库级锁（Database-level Lock），高并发请求下极易产生 `sqlite3.OperationalError: database is locked`。而金融记账要求不同租户甚至同租户的不同账户之间互不阻塞，必须具备成熟的**行级锁（Row-level Locking，如 `FOR UPDATE`）**；
3. **海量审计事件与流水分区存储**：消费记录（RequestRecord）、链上结算明细（SettlementRecord）与账本流水（LedgerEntry）属于高频时间序列数据，需要成熟的分区表（Partitioning）、高效 B-Tree 索引及复合唯一约束支持；
4. **配置版本快照与 JSONB 高性能操作**：路由规则（Route）、安全策略（Policy）的发布需要记录不可变快照快查，PostgreSQL 的 `JSONB` 原生索引与检索能力极为出色。

---

## 2. 决策 (Decision)

我们正式决定：**将 AgentPay 的生产核心数据存储全面选型为 PostgreSQL (>= 15.0)**。

具体落地规范如下：
1. **统一数据中心**：控制面管理服务、数据面 Go Gateway、虚拟钱包记账与消费审计统一接入 PostgreSQL 实例或主从集群；
2. **连接池与高并发管理**：数据面 Go Gateway 采用 `pgxpool`，控制面 API 采用成熟的异步连接池，严格管理连接数与查询超时；
3. **强事务行级锁隔离**：
   - 钱包扣费与冻结必须使用：`SELECT ... FROM ap_wallet WHERE wallet_id = $1 FOR UPDATE;`
   - 确保同一租户的并发扣费按序列化精确校验，不同租户的并发扣费完全并行无锁争用；
4. **本地开发与轻量测试支持**：
   - 仓库提供标准的 `docker-compose.yml` 一键拉起本地开发用 PostgreSQL；
   - 纯算法/工具类的单元测试（如 x402 编解码、EIP-712 签名）保持零外部依赖，数据库相关集成测试在 CI/本地统一通过容器化 PostgreSQL 验证。

---

## 3. 方案对比与评估 (Options Considered)

| 评估维度 | SQLite (原方案) | MySQL (8.0+) | **PostgreSQL (最终选择)** |
|---|---|---|---|
| **并发锁粒度** | 库级写锁（全局串行） | 行级锁（InnoDB） | **行级锁（MVCC + FOR UPDATE 严密事务）** |
| **多节点共享** | 不支持（仅单机文件） | 原生支持 | **原生支持（云原生 Operator/主从/连接池）** |
| **半结构化/JSON** | JSON1 文本函数 | JSON 类型（支持局部操作） | **JSONB（二进制优化，支持 GIN 倒排索引）** |
| **金融记账与不变量** | 约束功能基础 | 事务与约束支持完备 | **极为严密（CHECK 约束、Partial Index、原生 UUID、强类型）** |
| **跨语言驱动支持** | 各语言内嵌 C 绑定 | 成熟 | **Go (`pgx`)、Python (`asyncpg`/`psycopg3`) 极高吞吐性能** |

---

## 4. 后续影响与执行要求 (Consequences)

1. **正向影响**：
   - 彻底解除高并发吞吐瓶颈，满足网关数万 QPS 的读写能力；
   - 虚拟钱包多租户并发安全从根本上得到数据库底层 ACID 保障；
   - 支持不可变配置快照、全量操作审计与日终自动对账任务的大规模批量处理。
2. **执行行动**：
   - 编写多租户领域数据模型规范：`specs/multitenant-data-model.md`；
   - 建立权威 DDL 脚本库：`infra/postgres/schema.sql`；
   - 更新本地 `docker-compose.yml` 包含 PostgreSQL 基础服务配置。
