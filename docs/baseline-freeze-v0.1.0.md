# AgentPay 行为基线冻结记录 (T0.2 Baseline Freeze)

> **基线版本**：`v0.1.0-poc-baseline`  
> **关联 Commit**：`338fc0a` (`fix(gateway): replace typing.Self with class annotations for Python 3.10 compatibility`)  
> **冻结日期**：2026-10-18  
> **执行人**：Antigravity Agent & Core Maintainers  
> **任务归属**：T0.2 冻结当前行为基线

---

## 1. 冻结目的

本记录用于在正式启动 **T1（协议、状态机与多租户数据契约）** 和 **T2（Go Gateway 云原生网关）** 前，对 AgentPay Phase 0 阶段（Python 最小原型闭环）的所有既有代码、测试用例与行为契约进行强制封箱与基准锚定。

后续任何针对协议扩展、跨语言 Go 重构或多租户演进的改动，均以此基线为正确性与一致性回归测试的唯一事实参照点。

---

## 2. 质量门禁与验证证据

在冻结执行时刻，通过项目级 Makefile 严格执行全量门禁检查（`make check`），结果全部通过：

### 2.1 静态检查与规范自检
- **代码 Lint (Ruff)**：通过（`python/` 目录下全部包无任何 lint 告警）
- **代码格式 (Format Check)**：通过（严格符合 PEP 8 与项目格式规范）
- **语言中立规范向量验证**：通过（`specs/test-vectors` 下全部 JSON 策略与签名测试向量通过结构校验）

### 2.2 自动化测试套件执行证据
- **测试框架**：`pytest 8.3.4`
- **执行命令**：`PYTHONPATH=python/packages/agentpay/src:python/packages/agentpay-gateway/src:python python3 -m pytest -v python/tests`
- **执行耗时**：3.70s
- **总用例数**：**340 个用例全部 PASSED** (0 failed, 0 error, 0 skipped)

```text
============================= 340 passed in 3.70s ==============================
- python/tests/unit/test_budget_models.py (PASSED)
- python/tests/unit/test_budget_storage.py (PASSED)
- python/tests/unit/test_circuit_breaker.py (PASSED)
- python/tests/unit/test_conformance_vectors.py (PASSED)
- python/tests/unit/test_engine.py (PASSED)
- python/tests/unit/test_mock_facilitator.py (PASSED)
- python/tests/unit/test_policy_models.py (PASSED)
- python/tests/unit/test_signer.py (PASSED)
- python/tests/unit/test_smoke.py (PASSED)
- python/tests/unit/test_subpackages.py (PASSED)
- python/tests/unit/test_x402_codecs.py (PASSED)
- python/tests/unit/test_x402_models.py (PASSED)
- python/tests/integration/ (PASSED)
- python/tests/e2e/ (PASSED)
```

---

## 3. 冻结的核心资产与架构范围

当前基线包含以下经过充分验证的核心模块与组件：

1. **Python 财务防火墙与策略引擎 (`agentpay-policy`)**：
   - 滚动窗口预算流水表（基于 SQLite `BEGIN IMMEDIATE` 悲观锁事务控制）；
   - 单笔上限、白名单（收款人、域名、网络 CAIP-2 校验）、终身限额；
   - 频次熔断与循环调用检测器。
2. **x402 V2 协议编解码与类型系统 (`agentpay-core`)**：
   - RFC 6648 兼容的 `PAYMENT-CHALLENGE` 与 `PAYMENT-SIGNATURE` Base64URL 序列化与反序列化；
   - EIP-712 / EIP-3009 授权转账数据签名校验；
   - 语言中立测试向量加载器。
3. **买方客户端拦截器 (`agentpay-client`)**：
   - `httpx` 异步/同步拦截器，自动捕获 402 响应、向策略引擎申请预留、签名并重放请求；
   - 异常自动回滚与预留安全释放守卫。
4. **参考自托管网关原型 (`agentpay-gateway`)**：
   - 基础 HTTP 反向代理原型；
   - 固定价格与收款地址路由分发；
   - 签名有效性核验与状态返回。
5. **本地开发环境与 Mock Facilitator**：
   - 内存级/本地仿真链上清算服务；
   - Arbitrum Sepolia 测试网 E2E 演示脚本与失控循环 Demo。

---

## 4. 冻结的不可违反安全契约 (Safety Redlines)

后续 T1/T2 开发中，必须继续严格遵守以下在基线中已确立的安全原则：

1. **金额存储与计算**：
   - 统一采用最小整数单位（如 USDC 6 位小数整数），**严禁使用 float 或包含浮点精度的数字类型**。
   - SQLite 金额列必须包含 `CHECK (typeof(col)='integer')` 约束。
2. **默认拒绝原则 (Fail-Closed)**：
   - 任何数据库锁竞争超时、网络异常、解析异常或未捕获错误，均统一裁定为拒绝支付。
3. **预留与签名安全守卫**：
   - 仅在策略引擎产生不可篡改的 `ApprovedReservation` 后方可调用签名器。
   - 签名一旦发出，上游网络超时或 5xx 异常**绝不立即自动释放预算**；必须等待授权过期（`now > valid_before + 安全余量`）且确认未上链结算后方可释放。
4. **状态三态隔离**：
   - 明确区分 `PaymentIntent`（意图）、`SettlementRecord`（链上结算）与 `FulfillmentRecord`（业务交付）。
   - 严禁将链上结算超时直接映射为业务失败。

---

## 5. Git 标签信息

- **Tag 名称**：`v0.1.0-poc-baseline`
- **目标 Commit**：`338fc0a25b380ebf2fd0aa99a5fec7886e...`
- **签名描述**：`chore(baseline): freeze Phase 0 Python PoC baseline with 340 passing tests`
- **检出指令**：
  ```bash
  git checkout v0.1.0-poc-baseline
  ```

---

## 6. 后续承接

随着本基线的冻结，**T0 前置工作正式闭环**，研发工作即刻全面切入 **T1 阶段**：
- **T1.1** 状态机规范将以本基线的三态模型为雏形，进行形式化定义；
- **T1.3** 跨语言契约将提炼本基线的所有输入输出结构，形成 Go/Python 共享测试用例；
- **T2.6** Go Gateway 开发完成后，将通过跨语言回放对比，确保 Go Gateway 与该基线的处理结果严格 100% 同构。
