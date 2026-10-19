# AgentPay 消费与操作审计事件契约规范 (T1.5)

> **规范版本**：1.0.0  
> **状态**：正式规范 (Draft Approved)  
> **适用范围**：Go Gateway 数据面、控制面 API、虚拟钱包账本引擎、离线对账与合规导出  
> **任务追踪**：研发任务清单 T1.5 (P1)

---

## 1. 契约目标与核心原则

在 AgentPay 的双轨支付（虚拟钱包与链上 x402）和多租户高并发架构中，审计事件（Audit Events）是平台实现**资金安全追溯、纠纷争议定责、高频计量计费、不可抵赖审计以及跨服务可观测性**的唯一信任事实来源（Single Source of Truth）。

### 1.1 设计原则
1. **只追加、永不可变 (Append-Only & Immutable)**：所有审计事件一经产生并持久化，严禁任何形式的更新（UPDATE）或物理删除（DELETE）。
2. **统一信封封装 (Unified Event Envelope)**：所有网关请求、支付意图、结算履约、钱包流水与管理操作事件必须使用统一标准信封包裹，具备一致的追踪上下文。
3. **因果与全链路关联 (Correlation & Causality)**：每个事件必须携带结构化追踪标头（`request_id`, `idempotency_key`, `intent_id`, `hold_id` 等），支持从单次网关 HTTP 调用一直追溯到链上交易或底层记账流水。
4. **安全脱敏与最小特权 (Privacy & Redaction by Default)**：默认剥离或掩码所有凭据、敏感密钥（API Key 仅记录 Hash/Prefix）、私钥载荷与上游内部堆栈。
5. **幂等存储与去重 (At-Least-Once Delivery, Exactly-Once Persistence)**：事件流产生遵循至少投递一次，数据库与存储层通过全局 `event_id` 及业务指纹实现严格幂等写入。

---

## 2. 标识体系与 ID 命名规范

所有实体与事件标识符均采用统一的 **`前缀 + 全局唯一标识`** 格式，避免字符串类型混淆：

| 前缀 | 实体 / 事件对象 | 示例 | 作用与生命周期 |
| :--- | :--- | :--- | :--- |
| `evt_` | 审计事件 ID | `evt_01j7b8z4m0k9x8y7` | 单条审计事件全局唯一标识（ULID 或 UUIDv4 Hex） |
| `req_` | 入站请求 ID | `req_01j7b8z4m0k9x8y1` | 网关接收到客户端 HTTP 请求时分配，透传至上游 |
| `pi_` | 支付意图 ID | `pi_01j7b8z4m0k9x8y2` | 对应一次 402 挑战或钱包扣款意图 |
| `set_` | 链上结算记录 ID | `set_01j7b8z4m0k9x8y3` | 对应一次链上 EIP-3009 授权打包或 Facilitator 提交 |
| `ful_` | 上游履约记录 ID | `ful_01j7b8z4m0k9x8y4` | 对应网关反向代理向上游发起的调用 |
| `wlt_` | 租户钱包 ID | `wlt_01j7b8z4m0k9x8y5` | 租户虚拟钱包主账户 |
| `hld_` | 资金冻结 ID | `hld_01j7b8z4m0k9x8y6` | 履约前在钱包中预冻结的配额记录 |
| `led_` | 账本流水 ID | `led_01j7b8z4m0k9x8y7` | 钱包资金变动的唯一不可变流水号 |
| `top_` | 充值订单 ID | `top_01j7b8z4m0k9x8y8` | 管理员发放、链上充值或法币充值单号 |
| `key_` | API Key ID | `key_01j7b8z4m0k9x8y9` | 访问凭证元数据标识 |
| `adm_` | 管理员操作 ID | `adm_01j7b8z4m0k9x8ya` | 控制面操作审计追踪 |

---

## 3. 标准事件信封 (Event Envelope)

所有事件由外层标准信封（Envelope）与内层业务载荷（Payload）组成。

```json
{
  "event_id": "evt_01j7b8z4m0k9x8y7a1b2c3d4",
  "event_type": "wallet.hold.created",
  "version": "1.0",
  "timestamp": "2026-10-18T07:15:00.123456Z",
  "tenant_id": "tenant_enterprise_01",
  "project_id": "proj_production",
  "correlation": {
    "request_id": "req_01j7b8z4m0k9x8y100000001",
    "idempotency_key": "tx_client_uuid_998877",
    "intent_id": "pi_01j7b8z4m0k9x8y200000002",
    "hold_id": "hld_01j7b8z4m0k9x8y600000003",
    "settlement_id": null,
    "fulfillment_id": null,
    "ledger_entry_id": null,
    "topup_id": null
  },
  "actor": {
    "type": "api_key",
    "id": "key_01j7b8z4m0k9x8y900000004",
    "client_ip": "198.51.100.42",
    "user_agent": "AgentPay-Python/0.1.0"
  },
  "data": {
    "amount_base_units": 10000,
    "asset": "USDC",
    "wallet_id": "wlt_01j7b8z4m0k9x8y500000005",
    "expires_at": "2026-10-18T07:15:30.123456Z",
    "balance_available_before": 500000,
    "balance_available_after": 490000,
    "balance_held_before": 0,
    "balance_held_after": 10000
  }
}
```

### 3.1 信封字段规范

| 字段名 | 类型 | 必填 | 说明 |
| :--- | :--- | :---: | :--- |
| `event_id` | String | 是 | `evt_` 开头的全局唯一 ID，单调递增推荐 |
| `event_type` | String | 是 | 点分层级事件类型，如 `intent.authorized` |
| `version` | String | 是 | 事件信封与载荷模式版本，当前固定为 `"1.0"` |
| `timestamp` | String | 是 | ISO 8601 UTC 微秒精度的生成时间戳 |
| `tenant_id` | String | 是 | 归属租户 ID |
| `project_id` | String | 否 | 归属项目 ID，若租户级全局操作可为空 |
| `correlation` | Object | 是 | 结构化关联追踪上下文（见 3.2） |
| `actor` | Object | 是 | 操作触发主体信息（见 3.3） |
| `data` | Object | 是 | 事件类型特异的强类型业务载荷 |

### 3.2 关联追踪上下文 (`correlation`)
- `request_id` (String, 可选): 网关 HTTP 请求 ID。
- `idempotency_key` (String, 可选): 客户端传入的业务幂等键。
- `intent_id` (String, 可选): 支付意图 ID。
- `settlement_id` (String, 可选): 链上结算明细 ID。
- `fulfillment_id` (String, 可选): 上游履约明细 ID。
- `hold_id` (String, 可选): 钱包预冻结 ID。
- `ledger_entry_id` (String, 可选): 账本流水 ID。
- `topup_id` (String, 可选): 充值单 ID。

### 3.3 主体模型 (`actor`)
- `type` (String, 必填): 主体类型，枚举值：
  - `api_key`: 通过 API 密钥发起请求的客户端 Agent
  - `client_signer`: 具备链上钱包地址的去中心化调用者
  - `admin_user`: 控制面控制台管理员 / 成员
  - `system_worker`: 平台内部定时任务或异步协调 Worker
- `id` (String, 必填): 对应主体的 ID（API Key ID、Signer Address、Member ID 或 Worker 名称）
- `client_ip` (String, 必填): 发起方 IPv4/IPv6 地址（经可信反代标头提取）
- `user_agent` (String, 否): HTTP 客户端标识字符串

---

## 4. 事件分类与事件类型定义

审计事件划分为六大核心领域：

```mermaid
graph TD
    AuditEvents[AgentPay 审计事件体系]
    AuditEvents --> Traffic[1. 网关与请求事件]
    AuditEvents --> Intent[2. 支付意图事件]
    AuditEvents --> Settle[3. 链上结算事件]
    AuditEvents --> Fulfill[4. 上游履约事件]
    AuditEvents --> Wallet[5. 钱包账本与充值事件]
    AuditEvents --> Admin[6. 管理与配置事件]
```

### 4.1 网关与入站流量事件 (Gateway Traffic)

| 事件类型 (`event_type`) | 触发时机 | 核心载荷字段 (`data`) |
| :--- | :--- | :--- |
| `gateway.request.received` | 网关接收到入站 HTTP 请求并完成路由初筛 | `route_id`, `http_method`, `request_path`, `content_length` |
| `gateway.challenge.issued` | 买方无授权或余额不足，返回 HTTP 402 Payment Required | `route_id`, `price_base_units`, `asset`, `payee_address`, `challenge_id` |
| `gateway.request.completed` | 请求处理全流程终结，向买方返回最终 HTTP 响应 | `route_id`, `http_status`, `duration_ms`, `payment_method`, `bytes_sent` |
| `gateway.request.rejected` | 请求因黑白名单、预算超限、认证失败或速率限制被拒绝 | `route_id`, `rejection_code`, `rejection_reason`, `http_status` |

### 4.2 支付意图事件 (Payment Intent)

| 事件类型 (`event_type`) | 触发时机 | 核心载荷字段 (`data`) |
| :--- | :--- | :--- |
| `intent.created` | 意图初始化创建 | `amount_base_units`, `asset`, `payment_method`, `config_version_id` |
| `intent.authorized` | 买方支付载荷（签名或钱包授权）校验通过 | `payer_address`, `valid_after`, `valid_before`, `nonce` |
| `intent.completed` | 结算与履约双重达成，意图成功归档 | `settlement_status`, `fulfillment_status`, `total_duration_ms` |
| `intent.failed` | 支付授权失败、结算失败或非法输入导致的终态失败 | `failure_code`, `failure_message` |
| `intent.expired` | 402 挑战在有效窗口内未收到有效载荷 | `challenge_ttl_ms`, `expired_at` |
| `intent.needs_review` | 结算超时或履约结果未知转人工审查 | `review_reason`, `hold_status`, `settlement_status` |

### 4.3 链上结算事件 (On-Chain Settlement)

| 事件类型 (`event_type`) | 触发时机 | 核心载荷字段 (`data`) |
| :--- | :--- | :--- |
| `settlement.submitted` | 结算交易已通过 RPC 广播或提交至 Facilitator | `network_id`, `asset_address`, `payer_address`, `nonce`, `tx_hash` |
| `settlement.pending` | 交易已进入链上区块，等待确认数确认 | `network_id`, `tx_hash`, `block_number`, `current_confirmations` |
| `settlement.confirmed` | 交易区块达到预设安全确认数 | `network_id`, `tx_hash`, `block_number`, `gas_used_base_units` |
| `settlement.failed` | 链上交易明确 Revert、Nonce 过期或被置换 | `network_id`, `tx_hash`, `revert_reason` |
| `settlement.unknown` | RPC 连续超时或网络断开，进入未知挂起状态 | `network_id`, `tx_hash`, `last_probe_time`, `probe_retry_count` |

### 4.4 上游履约事件 (Fulfillment)

| 事件类型 (`event_type`) | 触发时机 | 核心载荷字段 (`data`) |
| :--- | :--- | :--- |
| `fulfillment.dispatched` | 网关开始向上游目标服务建立连接并代理请求 | `upstream_url`, `timeout_ms` |
| `fulfillment.succeeded` | 上游返回 HTTP 2xx 明确成功响应 | `upstream_status`, `upstream_latency_ms`, `response_body_hash` |
| `fulfillment.failed` | 上游返回 HTTP 4xx 或明确不可执行的错误 | `upstream_status`, `upstream_latency_ms`, `error_code` |
| `fulfillment.unknown` | 上游响应超时或连接非正常断开（必须保持冻结并保护现场） | `timeout_ms`, `elapsed_ms`, `error_message` |

### 4.5 平台虚拟钱包与账本事件 (Wallet & Ledger)

| 事件类型 (`event_type`) | 触发时机 | 核心载荷字段 (`data`) |
| :--- | :--- | :--- |
| `wallet.created` | 租户开户创建首个资产钱包 | `wallet_id`, `asset`, `initial_balance` |
| `wallet.hold.created` | 意图创建时成功锁定可用额度（Hold） | `wallet_id`, `amount_base_units`, `expires_at`, `available_balance_after` |
| `wallet.hold.captured` | 上游履约成功，冻结金额转化为实扣（Capture） | `wallet_id`, `hold_id`, `amount_base_units`, `ledger_entry_id` |
| `wallet.hold.released` | 履约失败或超时撤回，冻结金额原路归还可用余额 | `wallet_id`, `hold_id`, `amount_base_units`, `release_reason` |
| `wallet.ledger.appended` | 账本写入一条不可变会计流水记录 | `wallet_id`, `entry_id`, `type`, `amount`, `ref_type`, `ref_id` |
| `wallet.status.changed` | 钱包状态迁移（如活跃 -> 欠费冻结 `frozen_arrears`） | `wallet_id`, `old_status`, `new_status`, `reason` |
| `topup.order.created` | 充值单创建（管理员发放、链上充值预备、法币单） | `topup_id`, `wallet_id`, `channel`, `amount_base_units`, `asset` |
| `topup.order.confirmed` | 充值资金到账确认并入账账本 | `topup_id`, `wallet_id`, `amount_base_units`, `external_ref`, `entry_id` |
| `topup.order.failed` | 充值单失败或关闭 | `topup_id`, `failure_reason` |

### 4.6 管理与配置事件 (Admin & Management)

| 事件类型 (`event_type`) | 触发时机 | 核心载荷字段 (`data`) |
| :--- | :--- | :--- |
| `admin.adjustment.executed` | 管理员执行手动调账（入账/扣减/冲正） | `wallet_id`, `amount`, `reason`, `ledger_entry_id`, `operator_id` |
| `admin.config.published` | 管理员发布全新路由/策略快照版本 | `version_id`, `config_hash`, `routes_count`, `policies_count` |
| `admin.config.rolled_back` | 管理员一键回滚配置快照 | `active_version_id`, `target_version_id`, `reason` |
| `admin.apikey.created` | 管理员或系统签发新 API Key | `key_id`, `key_prefix`, `project_id`, `scopes` |
| `admin.apikey.revoked` | 管理员吊销指定 API Key | `key_id`, `key_prefix`, `reason` |
| `admin.tenant.updated` | 租户基本信息、限额或状态更新 | `target_tenant_id`, `changed_fields` |

---

## 5. 安全脱敏与隐私保护规范 (Redaction & Data Privacy)

审计事件作为长期存档和对外导出的合规载体，**严禁明文记录密码、私钥、完整 API Key 或用户隐私数据 (PII)**。

### 5.1 脱敏规则矩阵

| 字段类别 | 明文出现场景 | 审计事件记录标准与脱敏算法 | 严重性级别 |
| :--- | :--- | :--- | :---: |
| **API Key 凭据** | `Authorization: Bearer agp_live_12345678abcdef...` | **绝不明文存储**。<br>1. 提取前缀：`key_prefix` (前 8 位如 `agp_live`)<br>2. 密钥哈希：`key_hash = sha256(raw_key)`<br>3. 关联 ID：`api_key_id` | **P0 (禁止明文)** |
| **私钥与助记词** | 买方本地签名器私钥 | **物理隔离**。网关及审计系统绝不允许接触私钥。若在载荷中误传，立即抹除并抛出安全告警。 | **P0 (禁止接触)** |
| **加密签名 (Signature)** | `PAYMENT-SIGNATURE` 请求头 | 允许保留 compact 字符串或分解的 `(v, r, s)`，必须隐去客户端任何非公开私有参数。 | **P2** |
| **HTTP 请求/响应体** | Agent 与上游交互的 Body 载荷 | 1. 默认仅记录 `body_sha256` 摘要与 `content_length` 字节数。<br>2. 如需采样留存，执行 JSON 敏感键脱敏（将 `password`, `secret`, `token`, `credential`, `private_key` 的值替换为 `[REDACTED]`），且截断最大长度为 2048 字符。 | **P1** |
| **客户端 IP 地址** | 客户端请求 IP | 内部存完整 IP 用于安全风控。对外导出脱敏模式下，IPv4 隐藏末位段（如 `198.51.100.xxx`），IPv6 隐藏后 64 位（如 `2001:db8::xxxx`）。 | **P2** |
| **上游错误堆栈** | 上游 500 / 502 返回的 HTML/堆栈信息 | 截断至最多 1024 字符，并过滤掉数据库连接串、密码和内网 IP。 | **P1** |

---

## 6. 事件幂等写入与去重规则

### 6.1 存储唯一性保证
PostgreSQL 存储表 `ap_audit_event` 中采用两层约束：
1. **主键唯一性**：`audit_id` (即 `event_id`) 为全局 Primary Key。
2. **业务幂等联合唯一索引**：针对同一次业务请求重放，使用 `(tenant_id, idempotency_key, event_type)` 唯一性索引（允许 `idempotency_key IS NULL` 的管理类事件独立生成）。

```sql
-- 幂等写入保证：若事件 ID 重复，直接忽略返回成功
INSERT INTO ap_audit_event (
    audit_id, tenant_id, operator_id, action, target_type, target_id, before_state, after_state, client_ip, created_at
) VALUES (
    'evt_01j7b8z4m0k9x8y7', 'tenant_01', 'key_01', 'wallet.hold.created', 'Hold', 'hld_01', NULL, '{"amount": 10000}', '192.0.2.1', NOW()
) ON CONFLICT (audit_id) DO NOTHING;
```

### 6.2 乱序到达与因果重排
当使用异步消息队列（如 Kafka、SQS 或内建异步 Buffer）收集审计日志时，由于网络原因事件可能乱序到达：
- 消费端与查询端按 `timestamp` (微秒) 与实体内部递增序列（如 `version` 或状态机迁移序号）进行稳定排序。
- 终态事件（`intent.completed`, `intent.failed`）到达后，锁定对应意图的审计跟踪，后续到达的重复探针事件仅追加记录为只读探针记录，不再触发状态回调。

---

## 7. 数据保留周期与导出规范 (Retention & Export)

### 7.1 分层存储生命周期 (Storage Tiering)

| 存储层级 | 存储介质 | 保留期限 | 查询延迟与场景 |
| :--- | :--- | :---: | :--- |
| **热存 (Hot Tier)** | PostgreSQL 15+ (`ap_audit_event`, `ap_request_record`) | **90 天** | 毫秒级。控制台实时检索、最近账单排查、实时告警。 |
| **温存 (Warm Tier)** | 对象存储 (S3 / GCS) 标准存储，Parquet / NDJSON 格式 | **1 年** | 秒至分钟级。离线五重自动对账、月度运营汇总、财务分析。 |
| **冷存 (Cold Tier)** | 归档对象存储 (Glacier Deep Archive / Coldline) | **7 年** | 小时级。符合金融合规监管审计追溯要求。 |

### 7.2 导出格式与契约

系统提供租户级批量导出接口：
1. **NDJSON 格式 (Newline Delimited JSON)**：
   - 每行一条符合信封模式的独立 JSON 对象，UTF-8 编码，Gzip 压缩传输。
   - 文件命名规范：`audit_export_{tenant_id}_{start_date}_{end_date}.ndjson.gz`
2. **CSV 展平格式 (Flat CSV for Excel / BI)**：
   - 字段列名：
     `event_id,event_type,timestamp,tenant_id,project_id,request_id,idempotency_key,intent_id,actor_type,actor_id,client_ip,amount_base_units,asset,status`
   - 对嵌套复杂数据以 JSON 字符串形式保存在最后一列 `payload_json`。
