<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage } from 'element-plus';
import { useTenantStore } from '@/stores/tenant';
import { paymentApi, agentApi } from '@/api/client';
import type { PaymentAuditRecord, DisplayStatus } from '@/types/payment';
import type { Agent } from '@/types/agent';
import { t } from '@/i18n';

const route = useRoute();
const tenantStore = useTenantStore();

// State
const loading = ref(false);
const payments = ref<PaymentAuditRecord[]>([]);
const totalCount = ref(0);
const pageSize = ref(10);
const currentPage = ref(1);

// Agents list for filter
const agentList = ref<Agent[]>([]);

// Filters
const selectedProject = ref<string>('all');
const selectedAgent = ref<string>('all');
const selectedPaymentMethod = ref<string>('all');
const selectedDisplayStatus = ref<string>('all');
const searchRouteOrReq = ref<string>('');
const dateRange = ref<[Date, Date] | null>(null);

// Detail Drawer State
const detailDrawerVisible = ref(false);
const activeRecord = ref<PaymentAuditRecord | null>(null);

// Summary KPI Metrics
const metrics = computed(() => {
  const list = payments.value;
  let totalAmount = 0;
  let completedCount = 0;
  let recoveryCount = 0;
  let unbilledSavedAmount = 0;

  for (const item of list) {
    totalAmount += item.amount_base_units;
    if (item.display_status === 'COMPLETED') {
      completedCount++;
    } else if (item.is_recovery_required || item.display_status === 'RECOVERY_REQUIRED') {
      recoveryCount++;
    } else if (item.display_status === 'RELEASED_UNBILLED') {
      unbilledSavedAmount += item.amount_base_units;
    }
  }

  const successRate = list.length > 0 ? Math.round((completedCount / list.length) * 100) : 100;

  return {
    totalTransactions: totalCount.value,
    totalSpentUsdc: (totalAmount / 1_000_000).toFixed(4),
    successRate,
    recoveryCount,
    unbilledSavedUsdc: (unbilledSavedAmount / 1_000_000).toFixed(4)
  };
});

// Format amount with tenant base currency (or USDC for onchain)
function formatUSDC(baseUnits: number, paymentMethod?: string): string {
  if (paymentMethod === 'onchain') {
    if (baseUnits === 0) return '0.00 USDC';
    return `${(baseUnits / 1_000_000).toFixed(4)} USDC`;
  }
  const sym = tenantStore.currencySymbol;
  const code = tenantStore.currentCurrency;
  if (baseUnits === 0) return `${sym}0.00 ${code}`;
  return `${sym}${(baseUnits / 1_000_000).toFixed(4)} ${code}`;
}

// Format date
function formatDate(dateStr: string): string {
  if (!dateStr) return '-';
  const d = new Date(dateStr);
  return d.toLocaleString('zh-CN', {
    hour12: false,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

// Tag types for composite status
function getCompositeTagType(status: DisplayStatus): 'success' | 'warning' | 'danger' | 'info' {
  switch (status) {
    case 'COMPLETED':
      return 'success';
    case 'RELEASED_UNBILLED':
      return 'info';
    case 'RECOVERY_REQUIRED':
    case 'SETTLED_FULFILLMENT_FAILED_REFUND_REQUIRED':
      return 'danger';
    case 'PAYMENT_FAILED':
    case 'REJECTED':
      return 'warning';
    default:
      return 'info';
  }
}

// Subsystem badge classes
function getSubsystemTagType(_subsystem: 'intent' | 'settlement' | 'fulfillment', status: string): 'success' | 'warning' | 'danger' | 'info' {
  if (status === 'completed' || status === 'confirmed' || status === 'succeeded') {
    return 'success';
  }
  if (status === 'held' || status === 'in_progress' || status === 'authorized' || status === 'submitted' || status === 'pending') {
    return 'warning';
  }
  if (status === 'failed' || status === 'needs_review' || status === 'unknown' || status === 'rejected') {
    return 'danger';
  }
  return 'info';
}

// Copy helper
function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text);
  ElMessage.success(t('已复制到剪贴板'));
}

// Load data
async function loadPayments() {
  const tenantId = tenantStore.currentTenantId;
  if (!tenantId) return;

  loading.value = true;
  try {
    const offset = (currentPage.value - 1) * pageSize.value;
    const query: Record<string, any> = {
      limit: pageSize.value,
      offset
    };

    if (selectedProject.value !== 'all') query.project_id = selectedProject.value;
    if (selectedAgent.value !== 'all') query.agent_id = selectedAgent.value;
    if (selectedPaymentMethod.value !== 'all') query.payment_method = selectedPaymentMethod.value;
    if (selectedDisplayStatus.value !== 'all') query.display_status = selectedDisplayStatus.value;
    if (searchRouteOrReq.value.trim()) query.route_id = searchRouteOrReq.value.trim();

    if (dateRange.value && dateRange.value.length === 2) {
      query.start_time = dateRange.value[0].toISOString();
      query.end_time = dateRange.value[1].toISOString();
    }

    const res = await paymentApi.listPayments(tenantId, query);
    payments.value = res.payments || [];
    totalCount.value = res.total || 0;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('获取消费记录失败');
    ElMessage.error(msg);
  } finally {
    loading.value = false;
  }
}

// Load agents for select dropdown
async function loadAgents() {
  const tenantId = tenantStore.currentTenantId;
  if (!tenantId) return;
  try {
    agentList.value = await agentApi.listAgents(tenantId);
  } catch {
    // ignore
  }
}

// Reset filters
function resetFilters() {
  selectedProject.value = 'all';
  selectedAgent.value = 'all';
  selectedPaymentMethod.value = 'all';
  selectedDisplayStatus.value = 'all';
  searchRouteOrReq.value = '';
  dateRange.value = null;
  currentPage.value = 1;
  loadPayments();
}

// Filter by recovery required directly
function filterRecoveryRequired() {
  selectedDisplayStatus.value = 'RECOVERY_REQUIRED';
  currentPage.value = 1;
  loadPayments();
}

// View details
function viewDetails(record: PaymentAuditRecord) {
  activeRecord.value = record;
  detailDrawerVisible.value = true;
}

// Pagination handles
function handlePageChange(page: number) {
  currentPage.value = page;
  loadPayments();
}

function handleSizeChange(size: number) {
  pageSize.value = size;
  currentPage.value = 1;
  loadPayments();
}

onMounted(async () => {
  // Check if router has agent_id query param
  if (route.query.agent_id) {
    selectedAgent.value = String(route.query.agent_id);
  }
  await loadAgents();
  await loadPayments();
});

watch(
  () => tenantStore.currentTenantId,
  async () => {
    await loadAgents();
    await loadPayments();
  }
);
</script>

<template>
  <div class="payments-page">
    <!-- Top KPI Cards -->
    <el-row :gutter="16" class="metrics-row">
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-header">
            <span class="label">{{ $t('消费交易总量') }}</span>
            <el-icon class="icon"><Tickets /></el-icon>
          </div>
          <div class="metric-value">{{ totalCount }} <span class="unit">{{ $t('笔') }}</span></div>
          <div class="metric-footer">
            <span class="sub-label">{{ $t('累计金额：') }}</span>
            <span class="sub-value">{{ formatUSDC(Number(metrics.totalSpentUsdc) * 1_000_000) }}</span>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-header">
            <span class="label">{{ $t('履约达成率 (COMPLETED)') }}</span>
            <el-icon class="icon success"><CircleCheck /></el-icon>
          </div>
          <div class="metric-value success">{{ metrics.successRate }}%</div>
          <div class="metric-footer">
            <span class="sub-label">{{ $t('确权交付保障') }}</span>
            <el-tag size="small" type="success" effect="plain">{{ $t('正交一致性验证') }}</el-tag>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card
          shadow="never"
          class="metric-card recovery-card"
          :class="{ 'has-alert': metrics.recoveryCount > 0 }"
          @click="filterRecoveryRequired"
        >
          <div class="metric-header">
            <span class="label">{{ $t('异常待恢复 (RECOVERY_REQUIRED)') }}</span>
            <el-icon class="icon danger"><Warning /></el-icon>
          </div>
          <div class="metric-value danger">{{ metrics.recoveryCount }} <span class="unit">{{ $t('笔') }}</span></div>
          <div class="metric-footer">
            <span class="sub-label">{{ $t('上游超时/5xx，钱包严格锁定中') }}</span>
            <el-button v-if="metrics.recoveryCount > 0" link type="danger" size="small">{{ $t('立即核验') }}</el-button>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="never" class="metric-card">
          <div class="metric-header">
            <span class="label">{{ $t('无损解冻挽回 (RELEASED_UNBILLED)') }}</span>
            <el-icon class="icon info"><Lock /></el-icon>
          </div>
          <div class="metric-value info">{{ formatUSDC(Number(metrics.unbilledSavedUsdc) * 1_000_000) }}</div>
          <div class="metric-footer">
            <span class="sub-label">{{ $t('上游 4xx 失败自动全额解冻') }}</span>
            <el-tag size="small" type="info" effect="plain">{{ $t('买方零扣费保障') }}</el-tag>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Filter Card -->
    <el-card shadow="never" class="filter-card">
      <el-form :inline="true" class="filter-form">
        <el-form-item :label="$t('时间范围')">
          <el-date-picker
            v-model="dateRange"
            type="datetimerange"
            :range-separator="$t('至')"
            :start-placeholder="$t('开始时间')"
            :end-placeholder="$t('结束时间')"
            format="YYYY-MM-DD HH:mm:ss"
            size="default"
            style="width: 320px"
          />
        </el-form-item>

        <el-form-item :label="$t('所属 Agent')">
          <el-select v-model="selectedAgent" :placeholder="$t('全部 Agent')" clearable filterable style="width: 200px">
            <el-option :label="$t('全部 Agent')" value="all" />
            <el-option
              v-for="a in agentList"
              :key="a.agent_id"
              :label="`${a.name} (${a.agent_id.slice(-6)})`"
              :value="a.agent_id"
            />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('支付方式')">
          <el-select v-model="selectedPaymentMethod" :placeholder="$t('全部方式')" style="width: 140px">
            <el-option :label="$t('全部方式')" value="all" />
            <el-option :label="$t('企业虚拟钱包')" value="wallet" />
            <el-option :label="$t('链上 x402')" value="onchain" />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('复合审计状态')">
          <el-select v-model="selectedDisplayStatus" :placeholder="$t('全部状态')" style="width: 220px">
            <el-option :label="$t('全部状态')" value="all" />
            <el-option :label="$t('COMPLETED (交易顺利完成)')" value="COMPLETED" />
            <el-option :label="$t('RELEASED_UNBILLED (无损解冻未计费)')" value="RELEASED_UNBILLED" />
            <el-option :label="$t('RECOVERY_REQUIRED (需人工对账恢复)')" value="RECOVERY_REQUIRED" />
            <el-option :label="$t('PAYMENT_FAILED (支付前失败/超额)')" value="PAYMENT_FAILED" />
            <el-option :label="$t('SETTLEMENT_IN_PROGRESS (链上广播确认中)')" value="SETTLEMENT_IN_PROGRESS" />
            <el-option :label="$t('WALLET_HOLD_PROCESSING (钱包锁定调用中)')" value="WALLET_HOLD_PROCESSING" />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('检索')">
          <el-input
            v-model="searchRouteOrReq"
            :placeholder="$t('搜索请求 ID / 路由 ID / 意图 ID')"
            clearable
            style="width: 240px"
            @keyup.enter="loadPayments"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="() => { currentPage = 1; loadPayments(); }">
            <el-icon><Search /></el-icon> {{ $t('查询') }}
          </el-button>
          <el-button @click="resetFilters">{{ $t('重置') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Payments Data Table -->
    <el-card shadow="never" class="table-card">
      <template #header>
        <div class="table-header">
          <div class="title-wrap">
            <span class="table-title">{{ $t('消费记录与三正交生命周期关联') }}</span>
            <span class="table-subtitle">{{ $t('（Intent 意图 / Settlement 结算 / Fulfillment 履约 / Virtual Wallet 联动）') }}</span>
          </div>
          <div class="header-actions">
            <el-button size="small" @click="loadPayments">
              <el-icon><Refresh /></el-icon> {{ $t('刷新数据') }}
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="payments" v-loading="loading" stripe style="width: 100%">
        <!-- Intent ID & Request ID -->
        <el-table-column :label="$t('支付意图 / 请求标识')" min-width="200">
          <template #default="{ row }">
            <div class="id-cell">
              <div class="primary-id">
                <el-link type="primary" :underline="false" @click="viewDetails(row)">
                  {{ row.intent_id }}
                </el-link>
                <el-icon class="copy-icon" @click.stop="copyToClipboard(row.intent_id)"><CopyDocument /></el-icon>
              </div>
              <div class="secondary-id">
                <span class="label">Req:</span> {{ row.request_id }}
                <el-icon class="copy-icon" @click.stop="copyToClipboard(row.request_id)"><CopyDocument /></el-icon>
              </div>
            </div>
          </template>
        </el-table-column>

        <!-- Agent Identification -->
        <el-table-column :label="$t('所属 Agent')" min-width="160">
          <template #default="{ row }">
            <div class="agent-cell" v-if="row.agent_id">
              <el-tag size="small" effect="light" class="agent-tag">
                <el-icon><Cpu /></el-icon> {{ row.agent_id }}
              </el-tag>
            </div>
            <span v-else class="text-muted">{{ $t('租户直接调用') }}</span>
          </template>
        </el-table-column>

        <!-- Route & Method -->
        <el-table-column :label="$t('路由 / 支付方式')" min-width="180">
          <template #default="{ row }">
            <div class="route-cell">
              <div class="route-id">{{ row.route_id }}</div>
              <div class="method-tag">
                <el-tag v-if="row.payment_method === 'wallet'" size="small" type="success" effect="plain">
                  <el-icon><Wallet /></el-icon> {{ $t('虚拟钱包') }}
                </el-tag>
                <el-tag v-else size="small" type="primary" effect="plain">
                  <el-icon><Connection /></el-icon> {{ $t('链上 x402') }}
                </el-tag>
              </div>
            </div>
          </template>
        </el-table-column>

        <!-- Amount -->
        <el-table-column :label="$t('消费金额')" width="140" align="right">
          <template #default="{ row }">
            <div class="amount-cell">
              <span class="amount-val">{{ formatUSDC(row.amount_base_units, row.payment_method) }}</span>
            </div>
          </template>
        </el-table-column>

        <!-- Three Orthogonal Lifecycles -->
        <el-table-column :label="$t('三大正交生命周期状态')" min-width="240">
          <template #default="{ row }">
            <div class="lifecycles-badges">
              <el-tooltip :content="$t('业务意图状态 (PaymentIntent)')" placement="top">
                <el-tag size="small" :type="getSubsystemTagType('intent', row.intent_status)">
                  {{ $t('意图:') }} {{ row.intent_status }}
                </el-tag>
              </el-tooltip>

              <el-tooltip :content="$t('资金划转结算状态 (SettlementRecord)')" placement="top">
                <el-tag size="small" :type="getSubsystemTagType('settlement', row.settlement_status)">
                  {{ $t('结算:') }} {{ row.settlement_status }}
                </el-tag>
              </el-tooltip>

              <el-tooltip :content="$t('上游 API 履约交付状态 (FulfillmentRecord)')" placement="top">
                <el-tag size="small" :type="getSubsystemTagType('fulfillment', row.fulfillment_status)">
                  {{ $t('履约:') }} {{ row.fulfillment_status }}
                </el-tag>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>

        <!-- Canonical Composite Display Status -->
        <el-table-column :label="$t('复合审计状态')" min-width="180">
          <template #default="{ row }">
            <div class="composite-status-cell">
              <el-tag :type="getCompositeTagType(row.display_status)" effect="dark" class="status-tag">
                {{ row.display_status }}
              </el-tag>
              <div class="status-desc">{{ row.display_note }}</div>
            </div>
          </template>
        </el-table-column>

        <!-- Created At -->
        <el-table-column prop="created_at" :label="$t('记录时间')" width="165">
          <template #default="{ row }">
            <span class="timestamp">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>

        <!-- Operations -->
        <el-table-column :label="$t('操作')" width="110" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="viewDetails(row)">
              {{ $t('全景审计') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <div class="pagination-box">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          :total="totalCount"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </div>
    </el-card>

    <!-- Detail Drawer: Full Three-Orthogonal-Lifecycle Audit View -->
    <el-drawer
      v-model="detailDrawerVisible"
      :title="$t('全生命周期证据链审计')"
      size="680px"
      direction="rtl"
      destroy-on-close
    >
      <template #default>
        <div class="drawer-content" v-if="activeRecord">
          <!-- Banner Alert for Status -->
          <el-alert
            :title="`${$t('复合判定状态：')}${activeRecord.display_status}`"
            :description="activeRecord.display_note"
            :type="getCompositeTagType(activeRecord.display_status)"
            show-icon
            :closable="false"
            class="status-alert"
          />

          <!-- Stage 1: Payment Intent -->
          <div class="stage-section">
            <div class="stage-header">
              <div class="stage-title">
                <el-icon class="icon"><Document /></el-icon>
                <span>{{ $t('1. 业务意图 (Payment Intent)') }}</span>
              </div>
              <el-tag :type="getSubsystemTagType('intent', activeRecord.intent_status)">
                {{ activeRecord.intent_status }}
              </el-tag>
            </div>
            <div class="stage-body">
              <el-descriptions :column="2" border size="small">
                <el-descriptions-item :label="$t('意图 ID (Intent ID)')" :span="2">
                  <span class="mono">{{ activeRecord.intent_id }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('请求 ID (Request ID)')">
                  <span class="mono">{{ activeRecord.request_id }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('幂等键 (Idempotency)')">
                  <span class="mono">{{ activeRecord.idempotency_key || $t('未指定') }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('所属 Agent')">
                  <span class="mono">{{ activeRecord.agent_id || $t('租户直连') }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('所属项目')">
                  <span class="mono">{{ activeRecord.project_id }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('意图金额')">
                  <span class="bold-text">{{ formatUSDC(activeRecord.amount_base_units, activeRecord.payment_method) }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('创建时间')">
                  {{ formatDate(activeRecord.created_at) }}
                </el-descriptions-item>
              </el-descriptions>
            </div>
          </div>

          <!-- Stage 2: Settlement Record -->
          <div class="stage-section">
            <div class="stage-header">
              <div class="stage-title">
                <el-icon class="icon"><Coin /></el-icon>
                <span>{{ $t('2. 资金结算与冻结 (Settlement Record)') }}</span>
              </div>
              <el-tag :type="getSubsystemTagType('settlement', activeRecord.settlement_status)">
                {{ activeRecord.settlement_status }}
              </el-tag>
            </div>
            <div class="stage-body">
              <el-descriptions :column="2" border size="small">
                <el-descriptions-item :label="$t('结算模式')">
                  <el-tag size="small" :type="activeRecord.payment_method === 'wallet' ? 'success' : 'primary'">
                    {{ activeRecord.payment_method === 'wallet' ? $t('虚拟钱包 (双式记账)') : $t('链上 x402 授权签名') }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('结算状态')">
                  <span class="mono">{{ activeRecord.settlement_status }}</span>
                </el-descriptions-item>

                <!-- Wallet specifics -->
                <template v-if="activeRecord.payment_method === 'wallet'">
                  <el-descriptions-item :label="$t('关联冻结 ID (Hold ID)')" :span="2">
                    <span class="mono">{{ activeRecord.hold?.hold_id || activeRecord.settlement?.settlement_id }}</span>
                    <el-tag v-if="activeRecord.hold" size="small" class="ml-2" type="info">
                      Hold: {{ activeRecord.hold.status }}
                    </el-tag>
                  </el-descriptions-item>
                  <el-descriptions-item :label="$t('账本流水 (Ledger Entry)')" :span="2" v-if="activeRecord.ledger">
                    <span class="mono">{{ activeRecord.ledger.entry_id }}</span>
                    <el-tag size="small" class="ml-2" :type="activeRecord.ledger.type === 'capture' ? 'success' : 'warning'">
                      {{ activeRecord.ledger.type }} ({{ activeRecord.ledger.amount > 0 ? '+' : '' }}{{ activeRecord.ledger.amount / 1000000 }} {{ tenantStore.currentCurrency }})
                    </el-tag>
                  </el-descriptions-item>
                </template>

                <!-- Onchain specifics -->
                <template v-else>
                  <el-descriptions-item :label="$t('目标网络')">
                    <span class="mono">{{ activeRecord.settlement?.network_id || 'eip155:84532' }}</span>
                  </el-descriptions-item>
                  <el-descriptions-item :label="$t('交易哈希 / Nonce')" :span="2">
                    <span class="mono">{{ activeRecord.settlement?.tx_hash || activeRecord.settlement?.nonce }}</span>
                  </el-descriptions-item>
                </template>
              </el-descriptions>
            </div>
          </div>

          <!-- Stage 3: Fulfillment Record -->
          <div class="stage-section">
            <div class="stage-header">
              <div class="stage-title">
                <el-icon class="icon"><Service /></el-icon>
                <span>{{ $t('3. 上游交付与履约 (Fulfillment Record)') }}</span>
              </div>
              <el-tag :type="getSubsystemTagType('fulfillment', activeRecord.fulfillment_status)">
                {{ activeRecord.fulfillment_status }}
              </el-tag>
            </div>
            <div class="stage-body">
              <el-descriptions :column="2" border size="small">
                <el-descriptions-item :label="$t('上游地址 (Upstream URL)')" :span="2">
                  <span class="mono">{{ activeRecord.fulfillment?.upstream_url || '-' }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('HTTP 响应码')">
                  <el-tag
                    size="small"
                    :type="(activeRecord.fulfillment?.upstream_status || 0) < 400 ? 'success' : 'danger'"
                  >
                    {{ activeRecord.fulfillment?.upstream_status || $t('未收到') }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('端到端耗时')">
                  <span class="mono">{{ activeRecord.fulfillment?.latency_ms || 0 }} ms</span>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('响应摘要 (SHA-256)')" :span="2" v-if="activeRecord.fulfillment?.response_body_hash">
                  <span class="mono hash-text">{{ activeRecord.fulfillment.response_body_hash }}</span>
                </el-descriptions-item>
              </el-descriptions>
            </div>
          </div>

          <!-- Raw JSON Audit Collapsible -->
          <el-collapse class="json-collapse">
            <el-collapse-item :title="$t('查看不可篡改原始审计 JSON (Raw Evidence)')" name="raw">
              <div class="json-box">
                <pre>{{ JSON.stringify(activeRecord, null, 2) }}</pre>
              </div>
            </el-collapse-item>
          </el-collapse>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped>
.payments-page {
  padding: 16px 20px;
}

.metrics-row {
  margin-bottom: 16px;
}

.metric-card {
  border-radius: 8px;
  border-color: #ebeef5;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.metric-header .label {
  font-size: 13px;
  color: #606266;
  font-weight: 500;
}

.metric-header .icon {
  font-size: 18px;
  color: #909399;
}

.metric-header .icon.success {
  color: #67c23a;
}
.metric-header .icon.danger {
  color: #f56c6c;
}
.metric-header .icon.info {
  color: #409eff;
}

.metric-value {
  font-size: 24px;
  font-weight: 700;
  color: #303133;
  line-height: 1.2;
  margin-bottom: 8px;
}

.metric-value.success {
  color: #67c23a;
}
.metric-value.danger {
  color: #f56c6c;
}
.metric-value.info {
  color: #409eff;
}

.metric-value .unit {
  font-size: 13px;
  font-weight: normal;
  color: #909399;
  margin-left: 4px;
}

.metric-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: #909399;
}

.recovery-card.has-alert {
  border-color: #fde2e2;
  background-color: #fef0f0;
  cursor: pointer;
}

.filter-card {
  margin-bottom: 16px;
  border-radius: 8px;
}

.filter-form {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
}

.filter-form :deep(.el-form-item) {
  margin-bottom: 8px;
  margin-right: 0;
}

.table-card {
  border-radius: 8px;
}

.table-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.table-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.table-subtitle {
  font-size: 12px;
  color: #909399;
  margin-left: 8px;
}

.id-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.primary-id {
  font-family: monospace;
  font-size: 13px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 4px;
}

.secondary-id {
  font-family: monospace;
  font-size: 11px;
  color: #909399;
  display: flex;
  align-items: center;
  gap: 4px;
}

.copy-icon {
  font-size: 12px;
  cursor: pointer;
  color: #909399;
}

.copy-icon:hover {
  color: #409eff;
}

.agent-cell {
  display: flex;
  align-items: center;
}

.agent-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-family: monospace;
}

.route-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.route-id {
  font-family: monospace;
  font-size: 12px;
  color: #303133;
  font-weight: 500;
}

.amount-cell {
  font-family: monospace;
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}

.lifecycles-badges {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.composite-status-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.status-tag {
  align-self: flex-start;
  font-size: 11px;
  font-family: monospace;
  letter-spacing: 0.5px;
}

.status-desc {
  font-size: 11px;
  color: #909399;
  line-height: 1.3;
}

.timestamp {
  font-family: monospace;
  font-size: 12px;
  color: #606266;
}

.pagination-box {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

/* Detail Drawer */
.drawer-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.status-alert {
  margin-bottom: 8px;
}

.stage-section {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 12px 16px;
  background-color: #fafafa;
}

.stage-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.stage-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.stage-title .icon {
  font-size: 16px;
  color: #409eff;
}

.mono {
  font-family: monospace;
  font-size: 12px;
}

.hash-text {
  word-break: break-all;
  font-size: 11px;
  color: #606266;
}

.bold-text {
  font-weight: 700;
  color: #303133;
}

.ml-2 {
  margin-left: 8px;
}

.json-collapse {
  margin-top: 8px;
}

.json-box {
  background-color: #282c34;
  color: #abb2bf;
  padding: 12px;
  border-radius: 6px;
  font-family: monospace;
  font-size: 11px;
  overflow-x: auto;
  max-height: 280px;
}
</style>
