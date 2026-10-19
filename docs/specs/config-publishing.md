# AgentPay 配置发布与运行时缓存契约规范 (T1.4 Specification)

> **版本**：v2.0.0 (T1.4 统一规范)  
> **状态**：**已定稿 (Approved Specification)**  
> **适用范围**：Web 管理控制台（控制面）、Go Gateway（数据面）、PostgreSQL 配置持久化  
> **机器可读模式**：  
> - [`specs/conformance/config-snapshot.schema.json`](../../specs/conformance/config-snapshot.schema.json) (不可变快照 JSON Schema)  
> - [`specs/test-vectors/config-publishing.json`](../../specs/test-vectors/config-publishing.json) (配置发布与回滚测试向量)  

---

## 1. 核心设计原则

1. **草稿绝对隔离（Draft Isolation）**：
   - 处于草稿态（`draft`）的路由或策略变更仅存在于管理控制台会话或草稿区，**严禁对在线 Gateway 请求产生任何实质影响**。
2. **不可变版本快照（Immutable Snapshot & Content Addressing）**：
   - 配置发布一旦完成，系统立刻生成一份完整的全局配置快照（包含该租户名下所有的 Routes、Policies 及绑定关系），赋予递增的 `version_number` 与确定性 SHA-256 内容指纹（`snapshotHash`），存入 `ap_config_version`；
   - 该快照**绝对只读、严禁更新或删除**。
3. **数据面微秒级原子热切换（Zero-Downtime RCU Swap）**：
   - Go Gateway 在内存中采用类似 Linux RCU（Read-Copy-Update）的原子指针替换机制。当新版本发布生效时，Gateway 瞬间将活跃配置指针切换到新快照，正在执行中的旧请求继续引用旧快照直至完成，新请求即刻使用新规则，全程零连接中断、零吞吐抖动。
4. **强健降级与高可用防御（Fail-Safe Degradation）**：
   - 若控制面服务宕机、数据库短时失联或网络分区，Gateway **坚决不停摆**，严格基于内存与本地磁盘落盘镜像（Local Cache）持续提供代理与验签服务。

---

## 2. 严格静态校验矩阵 (Validation Rules)

在发布前，控制面必须通过静态规则拦截，任何一条失败均禁止发布：

1. **URI 格式合规**：`upstream_url` 必须是合法 URI（以 `http://` 或 `https://` 开头）；
2. **PathPattern 唯一性**：同一租户内 `path_pattern` 绝对唯一，禁止歧义重叠；
3. **收款地址校验**：`payee_address` 必须符合有效以太坊地址规范（`^0x[a-fA-F0-9]{40}$`）；
4. **定价非负**：`price_base_units >= 0`；
5. **支付方式枚举**：`allowed_payment_methods` 严格限定于 `wallet`、`onchain`、`both` 之一；
6. **预算上限阶梯校验**：单笔上限必须严格小于或等于 24 小时滚动预算上限：
   $$\text{max\_amount\_per\_request} \le \text{rolling\_24h\_budget\_cap}$$

---

## 3. Gateway 运行时缓存与刷新协议 (Push + Pull)

- **Push 实时通知**：PostgreSQL `LISTEN/NOTIFY` 广播新版本事件；
- **Pull 巡检兜底**：Gateway 每 30 秒轮询比对活跃快照的 SHA-256 指纹；
- **原子热替换**：指纹校验通过后瞬间替换内存指针，旧请求优雅完成。
