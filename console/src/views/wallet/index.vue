<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import PermissionGate from '@/components/PermissionGate.vue';
import { t } from '@/i18n';
import { walletApi } from '@/api/client';
import type {
  Wallet,
  LedgerEntry,
  TopUpOrder,
  ReconciliationReport,
  LedgerEntryType,
  TopUpStatus
} from '@/types/wallet';

const authStore = useAuthStore();
const tenantStore = useTenantStore();

// Wallet State
const loading = ref(false);
const wallet = ref<Wallet | null>(null);
const ledgerEntries = ref<LedgerEntry[]>([]);
const topupOrders = ref<TopUpOrder[]>([]);
const activeTab = ref('ledger');

// Filters & Pagination for Ledger
const ledgerTypeFilter = ref<string>('all');
const ledgerSearchQuery = ref('');
const ledgerPage = ref(1);
const ledgerPageSize = ref(10);

// Filters & Pagination for TopUps
const topupChannelFilter = ref<string>('all');
const topupStatusFilter = ref<string>('all');
const topupPage = ref(1);
const topupPageSize = ref(10);

// Reconciliation State
const reconciling = ref(false);
const reconciliationReport = ref<ReconciliationReport | null>(null);

// Admin Grant Dialog State
const grantDialogVisible = ref(false);
const grantSubmitting = ref(false);
const grantFormRef = ref<FormInstance>();
const grantForm = ref({
  amountUsdc: 10,
  reason: '',
  idempotency_key: ''
});

const grantRules: FormRules = {
  amountUsdc: [
    { required: true, message: t('请输入充值发放金额'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (!value || Number(value) <= 0) {
          callback(new Error(t('充值金额必须大于 0')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ],
  reason: [
    { required: true, message: t('必须填写发放原因与依据 (Required reason)'), trigger: 'blur' },
    { min: 5, message: t('原因说明不能少于 5 个字符'), trigger: 'blur' }
  ]
};

// Manual Adjustment Dialog State
const adjustDialogVisible = ref(false);
const adjustSubmitting = ref(false);
const adjustFormRef = ref<FormInstance>();
const adjustForm = ref({
  direction: 'increase' as 'increase' | 'decrease',
  amountUsdc: 5,
  reason: '',
  approved_by: '',
  idempotency_key: ''
});

const adjustRules: FormRules = {
  amountUsdc: [
    { required: true, message: t('请输入调账金额'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (!value || Number(value) <= 0) {
          callback(new Error(t('调账金额必须大于 0')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ],
  reason: [
    { required: true, message: t('必须填写调账原因与审计说明 (Required reason)'), trigger: 'blur' },
    { min: 5, message: t('原因说明不能少于 5 个字符'), trigger: 'blur' }
  ],
  approved_by: [
    {
      validator: (_rule, value, callback) => {
        if (adjustForm.value.amountUsdc >= 1000 && (!value || !value.trim())) {
          callback(new Error(`${t('调账金额 >= 1,000')} ${currencyUnit.value} ${t('时，必须填写独立审批人进行双人双控复核')}`));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ]
};

// Freeze / Unfreeze Dialog State
const freezeDialogVisible = ref(false);
const freezeSubmitting = ref(false);
const freezeFormRef = ref<FormInstance>();
const freezeForm = ref({
  action: 'freeze' as 'freeze' | 'unfreeze',
  reason: ''
});

const freezeRules: FormRules = {
  reason: [
    { required: true, message: t('请填写冻结/解冻操作原因与说明'), trigger: 'blur' },
    { min: 4, message: t('原因说明不能少于 4 个字符'), trigger: 'blur' }
  ]
};

// Dynamic tenant / wallet base currency (6 decimals integer base units)
const currencyUnit = computed(() => {
  const wAsset = wallet.value?.asset;
  if (wAsset && wAsset !== 'USDC') return wAsset;
  return tenantStore.currentCurrency || wAsset || 'USD';
});

const currencySymbol = computed(() => {
  switch (currencyUnit.value) {
    case 'CNY':
      return '¥';
    case 'EUR':
      return '€';
    default:
      return '$';
  }
});

const canChangeCurrency = computed(() => {
  return ledgerEntries.value.length === 0;
});

async function handleCurrencyChange(newCurr: 'CNY' | 'USD' | 'EUR') {
  try {
    await tenantStore.updateTenantCurrency(newCurr);
    if (wallet.value) {
      wallet.value.asset = newCurr;
    }
    ElMessage.success(`${t('已将租户与虚拟钱包本位币切换为')} ${newCurr}`);
  } catch (err: unknown) {
    const msg = err instanceof Error ? t(err.message) : t('本位币切换失败');
    ElMessage.error(msg);
  }
}

function formatUSDC(baseUnits: number | undefined): string {
  if (baseUnits === undefined || isNaN(baseUnits)) return '0.0000';
  const val = baseUnits / 1000000;
  return val.toLocaleString('en-US', {
    minimumFractionDigits: 4,
    maximumFractionDigits: 6
  });
}

function formatBaseUnits(baseUnits: number | undefined): string {
  if (baseUnits === undefined || isNaN(baseUnits)) return '0';
  return baseUnits.toLocaleString('en-US');
}

function formatDate(isoStr: string | undefined): string {
  if (!isoStr) return '-';
  try {
    const d = new Date(isoStr);
    return d.toLocaleString('zh-CN', { hour12: false });
  } catch {
    return isoStr;
  }
}

// Data fetching
async function loadWalletData() {
  const tenantId = tenantStore.currentTenantId;
  if (!tenantId) return;

  loading.value = true;
  try {
    // 1. Load tenant master wallet
    const w = await walletApi.getTenantWallet(tenantId);
    wallet.value = w;

    // 2. Load ledger entries
    if (w && w.wallet_id) {
      const entries = await walletApi.getLedgerEntries(w.wallet_id);
      ledgerEntries.value = entries.sort(
        (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      );

      // 3. Load top-up orders
      const orders = await walletApi.getTopUpOrders(tenantId, w.wallet_id);
      topupOrders.value = orders.sort(
        (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      );
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? t(err.message) : t('加载钱包信息失败');
    ElMessage.error(msg);
  } finally {
    loading.value = false;
  }
}

// Watch tenant change to enforce tenant isolation
watch(
  () => tenantStore.currentTenantId,
  () => {
    wallet.value = null;
    ledgerEntries.value = [];
    topupOrders.value = [];
    reconciliationReport.value = null;
    loadWalletData();
  }
);

onMounted(() => {
  loadWalletData();
});

// Filtered and paginated ledger entries
const filteredLedgerEntries = computed(() => {
  return ledgerEntries.value.filter((item) => {
    if (ledgerTypeFilter.value !== 'all' && item.type !== ledgerTypeFilter.value) {
      return false;
    }
    if (ledgerSearchQuery.value.trim()) {
      const q = ledgerSearchQuery.value.trim().toLowerCase();
      const inId = item.entry_id.toLowerCase().includes(q);
      const inRef = item.ref_id.toLowerCase().includes(q);
      const inReason = (item.reason || '').toLowerCase().includes(q);
      const inOp = (item.operator_id || '').toLowerCase().includes(q);
      if (!inId && !inRef && !inReason && !inOp) return false;
    }
    return true;
  });
});

const paginatedLedgerEntries = computed(() => {
  const start = (ledgerPage.value - 1) * ledgerPageSize.value;
  return filteredLedgerEntries.value.slice(start, start + ledgerPageSize.value);
});

// Filtered and paginated topup orders
const filteredTopUpOrders = computed(() => {
  return topupOrders.value.filter((item) => {
    if (topupChannelFilter.value !== 'all' && item.channel !== topupChannelFilter.value) {
      return false;
    }
    if (topupStatusFilter.value !== 'all' && item.status !== topupStatusFilter.value) {
      return false;
    }
    return true;
  });
});

const paginatedTopUpOrders = computed(() => {
  const start = (topupPage.value - 1) * topupPageSize.value;
  return filteredTopUpOrders.value.slice(start, start + topupPageSize.value);
});

// Admin Grant Action
function openGrantDialog() {
  grantForm.value = {
    amountUsdc: 10,
    reason: '',
    idempotency_key: `idem_grant_${Date.now()}`
  };
  grantDialogVisible.value = true;
}

async function handleGrantSubmit() {
  if (!grantFormRef.value) return;
  await grantFormRef.value.validate(async (valid) => {
    if (!valid) return;
    const tenantId = tenantStore.currentTenantId;
    if (!tenantId) return;

    grantSubmitting.value = true;
    try {
      const baseUnits = Math.round(grantForm.value.amountUsdc * 1000000);
      await walletApi.adminGrant(tenantId, {
        amount: baseUnits,
        reason: grantForm.value.reason,
        idempotency_key: grantForm.value.idempotency_key
      });
      ElMessage.success(`${t('成功向租户发放')} ${grantForm.value.amountUsdc} ${currencyUnit.value} ${t('额度')}`);
      grantDialogVisible.value = false;
      await loadWalletData();
    } catch (err: unknown) {
      const msg = err instanceof Error ? t(err.message) : t('充值发放失败');
      ElMessage.error(msg);
    } finally {
      grantSubmitting.value = false;
    }
  });
}

// Manual Adjust Action
function openAdjustDialog() {
  adjustForm.value = {
    direction: 'increase',
    amountUsdc: 5,
    reason: '',
    approved_by: '',
    idempotency_key: `idem_adj_${Date.now()}`
  };
  adjustDialogVisible.value = true;
}

async function handleAdjustSubmit() {
  if (!adjustFormRef.value || !wallet.value) return;
  await adjustFormRef.value.validate(async (valid) => {
    if (!valid) return;

    const isLarge = adjustForm.value.amountUsdc >= 1000;
    if (isLarge && !adjustForm.value.approved_by) {
      ElMessage.warning(t('大额调账必须填写复核审批人 (approved_by)'));
      return;
    }

    try {
      await ElMessageBox.confirm(
        `${t('确定要为钱包')} ${wallet.value?.wallet_id} ${t('执行')} ${adjustForm.value.direction === 'increase' ? t('增加') : t('扣减')} ${adjustForm.value.amountUsdc} ${currencyUnit.value} ${t('的手工调账吗？此操作将记录永久审计流水。')}`,
        t('高风险调账操作确认'),
        {
          confirmButtonText: t('确认执行'),
          cancelButtonText: t('取消'),
          type: 'warning'
        }
      );
    } catch {
      return;
    }

    if (!wallet.value) return;
    adjustSubmitting.value = true;
    try {
      const factor = adjustForm.value.direction === 'increase' ? 1 : -1;
      const baseUnits = Math.round(adjustForm.value.amountUsdc * 1000000) * factor;

      await walletApi.adjustBalance(wallet.value.wallet_id, {
        amount: baseUnits,
        reason: `${adjustForm.value.reason}${adjustForm.value.approved_by ? ` (${t('复核人:')} ${adjustForm.value.approved_by})` : ''}`,
        idempotency_key: adjustForm.value.idempotency_key,
        approved_by: adjustForm.value.approved_by
      });
      ElMessage.success(t('调账成功完成并记入账本'));
      adjustDialogVisible.value = false;
      await loadWalletData();
    } catch (err: unknown) {
      const msg = err instanceof Error ? t(err.message) : t('调账失败');
      ElMessage.error(msg);
    } finally {
      adjustSubmitting.value = false;
    }
  });
}

// Freeze / Unfreeze Action
function openFreezeDialog(targetAction: 'freeze' | 'unfreeze') {
  freezeForm.value = {
    action: targetAction,
    reason: ''
  };
  freezeDialogVisible.value = true;
}

async function handleFreezeSubmit() {
  if (!freezeFormRef.value || !wallet.value) return;
  await freezeFormRef.value.validate(async (valid) => {
    if (!valid) return;

    // Check arrears restriction per W6
    if (freezeForm.value.action === 'unfreeze') {
      if (wallet.value?.status === 'frozen_arrears' && wallet.value.available_cache < 0) {
        ElMessage.error(t('当前钱包处于欠费状态 (余额小于 0)，根据 W6 规则严禁解冻！必须先充值补足负余额。'));
        return;
      }
    }

    if (!wallet.value) return;
    freezeSubmitting.value = true;
    try {
      if (freezeForm.value.action === 'freeze') {
        await walletApi.freezeWallet(wallet.value.wallet_id, {
          reason: freezeForm.value.reason
        });
        ElMessage.warning(t('钱包已被成功冻结'));
      } else {
        await walletApi.unfreezeWallet(wallet.value.wallet_id, {
          reason: freezeForm.value.reason
        });
        ElMessage.success(t('钱包已成功解冻恢复正常使用'));
      }
      freezeDialogVisible.value = false;
      await loadWalletData();
    } catch (err: unknown) {
      const msg = err instanceof Error ? t(err.message) : t('操作失败');
      ElMessage.error(msg);
    } finally {
      freezeSubmitting.value = false;
    }
  });
}

// Reconcile Balance Action
async function handleReconcile() {
  if (!wallet.value) return;
  reconciling.value = true;
  try {
    const rep = await walletApi.reconcileWallet(wallet.value.wallet_id);
    reconciliationReport.value = rep;
    if (rep.is_balanced) {
      ElMessage.success(t('账本双向平账核验通过：流水总和与可用+冻结缓存完全一致！'));
    } else {
      ElMessage.error(`${t('检出账本差异 (Discrepancy)')}: ${formatUSDC(rep.discrepancy)} ${currencyUnit.value}`);
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? t(err.message) : t('对账检查失败');
    ElMessage.error(msg);
  } finally {
    reconciling.value = false;
  }
}

// Tag styles
function getLedgerTagType(type: LedgerEntryType) {
  switch (type) {
    case 'topup_credit':
      return 'success';
    case 'capture':
      return 'warning';
    case 'adjustment':
      return 'primary';
    case 'reversal':
      return 'danger';
    case 'hold_release':
      return 'info';
    default:
      return 'info';
  }
}

function getTopupStatusType(status: TopUpStatus) {
  switch (status) {
    case 'confirmed':
      return 'success';
    case 'pending':
    case 'created':
      return 'warning';
    case 'failed':
    case 'expired':
      return 'danger';
    case 'reversed':
      return 'info';
    default:
      return 'info';
  }
}
</script>

<template>
  <div class="wallet-container" v-loading="loading">
    <!-- Top Context & Header -->
    <div class="page-header">
      <div>
        <h2 class="title">{{ $t('内部虚拟钱包与双轨记账账本') }}</h2>
        <div class="subtitle">
          {{ $t('所属租户:') }} <strong>{{ tenantStore.currentTenant?.name || tenantStore.currentTenantId }}</strong>
          <span class="divider">|</span>
          {{ $t('租户ID:') }} <code>{{ tenantStore.currentTenantId }}</code>
          <span class="divider">|</span>
          {{ $t('当前操作角色:') }} <el-tag size="small" effect="dark" type="info">{{ authStore.currentRole?.toUpperCase() }}</el-tag>
        </div>
      </div>
      <div class="header-actions">
        <el-button icon="Refresh" @click="loadWalletData">{{ $t('刷新') }}</el-button>

        <!-- Role-gated Reconcile -->
        <PermissionGate :roles="['owner', 'admin', 'operator']">
          <el-button type="info" plain icon="ScaleToOriginal" :loading="reconciling" @click="handleReconcile">
            {{ $t('双向平账核验') }}
          </el-button>
          <template #fallback>
            <el-tooltip :content="$t('仅 Owner / Admin / Operator 可发起对账核验')" placement="top">
              <el-button type="info" plain icon="ScaleToOriginal" disabled>{{ $t('双向平账核验') }}</el-button>
            </el-tooltip>
          </template>
        </PermissionGate>

        <!-- Role-gated Freeze / Unfreeze -->
        <PermissionGate :roles="['owner', 'admin']">
          <el-button
            v-if="wallet?.status === 'active'"
            type="danger"
            plain
            icon="Lock"
            @click="openFreezeDialog('freeze')"
          >
            {{ $t('冻结钱包') }}
          </el-button>
          <el-button
            v-else-if="wallet?.status === 'frozen' || wallet?.status === 'frozen_arrears'"
            type="success"
            plain
            icon="Unlock"
            @click="openFreezeDialog('unfreeze')"
          >
            {{ $t('解冻钱包') }}
          </el-button>
        </PermissionGate>

        <!-- Role-gated Adjustment -->
        <PermissionGate :roles="['owner', 'admin']">
          <el-button type="warning" plain icon="EditPen" @click="openAdjustDialog">
            {{ $t('人工调账') }}
          </el-button>
          <template #fallback>
            <el-tooltip :content="$t('人工调账需 Owner 或 Admin 权限')" placement="top">
              <el-button type="warning" plain icon="EditPen" disabled>{{ $t('人工调账') }}</el-button>
            </el-tooltip>
          </template>
        </PermissionGate>

        <!-- Role-gated Admin Grant -->
        <PermissionGate :roles="['owner', 'admin']">
          <el-button type="primary" icon="Money" @click="openGrantDialog">
            {{ $t('管理员发放充值') }}
          </el-button>
          <template #fallback>
            <el-tooltip :content="$t('管理员发放需 Owner 或 Admin 权限')" placement="top">
              <el-button type="primary" icon="Money" disabled>{{ $t('管理员发放充值') }}</el-button>
            </el-tooltip>
          </template>
        </PermissionGate>
      </div>
    </div>

    <!-- Prominent Status Alerts -->
    <div v-if="wallet?.status === 'frozen_arrears'" class="alert-box">
      <el-alert
        :title="$t('⚠️ 账户处于欠费冻结状态 (Frozen Arrears)')"
        type="error"
        show-icon
        :closable="false"
      >
        <template #default>
          <p class="alert-desc">
            {{ $t('当前可用余额已出现透支负值') }} (<strong>{{ currencySymbol }}{{ formatUSDC(wallet.available_cache) }} {{ currencyUnit }}</strong>)，{{ $t('系统已按') }} 
            <strong>{{ $t('W6 架构策略') }}</strong> {{ $t('自动执行欠费熔断保护。所有新支付冻结 (Hold) 请求均已被拦截。') }}<br />
            <strong>{{ $t('安全合规防线') }}</strong>{{ $t('：在通过充值或平账将负余额彻底补足至 &ge; 0 前，') }}<strong>{{ $t('严禁人工解冻账户') }}</strong>。
          </p>
        </template>
      </el-alert>
    </div>

    <div v-else-if="wallet?.status === 'frozen'" class="alert-box">
      <el-alert
        :title="$t('⚠️ 账户已被管理员手动冻结 (Frozen)')"
        type="warning"
        show-icon
        :closable="false"
      >
        <template #default>
          <p class="alert-desc">
            {{ $t('该租户钱包已被管理员手动冻结，暂停所有扣费、意图创建与额度申请。可在确认安全后由 Owner/Admin 执行解冻恢复。') }}
          </p>
        </template>
      </el-alert>
    </div>

    <!-- Balance & Status KPI Cards (6 Decimals Micro-Fiat Standard) -->
    <el-row :gutter="16" class="kpi-row">
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="never" class="kpi-card">
          <div class="kpi-label">
            {{ $t('可用余额 (Available Balance)') }}
            <el-tooltip :content="$t('可立即用于 Hold 冻结与 API 消费的净可用额度')" placement="top">
              <el-icon><QuestionFilled /></el-icon>
            </el-tooltip>
          </div>
          <div
            class="kpi-val"
            :class="{ 'text-danger': (wallet?.available_cache || 0) < 0, 'text-success': (wallet?.available_cache || 0) > 0 }"
          >
            {{ currencySymbol }}{{ formatUSDC(wallet?.available_cache) }}
            <span class="unit">{{ currencyUnit }}</span>
          </div>
          <div class="kpi-sub">
            {{ $t('基础最小单位:') }} <code>{{ formatBaseUnits(wallet?.available_cache) }}</code>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="never" class="kpi-card">
          <div class="kpi-label">
            {{ $t('冻结额度 (Held Reservations)') }}
            <el-tooltip :content="$t('进行中支付意图临时锁定的额度，上游交付成功后确收扣费，失败则解冻释放')" placement="top">
              <el-icon><QuestionFilled /></el-icon>
            </el-tooltip>
          </div>
          <div class="kpi-val text-warning">
            {{ currencySymbol }}{{ formatUSDC(wallet?.held_cache) }}
            <span class="unit">{{ currencyUnit }}</span>
          </div>
          <div class="kpi-sub">
            {{ $t('锁定中单位:') }} <code>{{ formatBaseUnits(wallet?.held_cache) }}</code>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="never" class="kpi-card">
          <div class="kpi-label">
            {{ $t('账户总资产 (Total Equity)') }}
            <el-tooltip :content="$t('可用余额 + 冻结额度总和')" placement="top">
              <el-icon><QuestionFilled /></el-icon>
            </el-tooltip>
          </div>
          <div class="kpi-val">
            {{ currencySymbol }}{{ formatUSDC((wallet?.available_cache || 0) + (wallet?.held_cache || 0)) }}
            <span class="unit">{{ currencyUnit }}</span>
          </div>
          <div class="kpi-sub" style="display: flex; align-items: center; justify-content: space-between; gap: 6px;">
            <span>{{ $t('本位币') }}: <strong>{{ currencyUnit }}</strong> ({{ $t('6位精度') }})</span>
            <PermissionGate :roles="['owner', 'admin']">
              <el-dropdown v-if="canChangeCurrency" trigger="click" @command="handleCurrencyChange">
                <el-button size="small" type="primary" link>
                  {{ $t('切换币种') }}
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="CNY" :disabled="currencyUnit === 'CNY'">CNY - 人民币 (¥)</el-dropdown-item>
                    <el-dropdown-item command="USD" :disabled="currencyUnit === 'USD'">USD - 美元 ($)</el-dropdown-item>
                    <el-dropdown-item command="EUR" :disabled="currencyUnit === 'EUR'">EUR - 欧元 (€)</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <el-tag v-else size="small" type="info" effect="plain">{{ $t('已锁定') }}</el-tag>
            </PermissionGate>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="never" class="kpi-card">
          <div class="kpi-label">{{ $t('钱包状态与安全约束') }}</div>
          <div class="kpi-status-box">
            <el-tag
              v-if="wallet?.status === 'active'"
              type="success"
              effect="dark"
              size="large"
            >
              {{ $t('正常 (Active)') }}
            </el-tag>
            <el-tag
              v-else-if="wallet?.status === 'frozen_arrears'"
              type="danger"
              effect="dark"
              size="large"
            >
              {{ $t('欠费冻结 (Arrears)') }}
            </el-tag>
            <el-tag
              v-else-if="wallet?.status === 'frozen'"
              type="warning"
              effect="dark"
              size="large"
            >
              {{ $t('手动冻结 (Frozen)') }}
            </el-tag>
            <el-tag
              v-else
              type="info"
              effect="dark"
              size="large"
            >
              {{ $t('已注销 (Closed)') }}
            </el-tag>
          </div>
          <div class="kpi-sub">
            {{ $t('乐观锁版本:') }} v{{ wallet?.version || 1 }} ({{ $t('W1 租户独立主钱包') }})
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Reconciliation Report Box (If triggered) -->
    <div v-if="reconciliationReport" class="reconcile-box">
      <el-card shadow="never" :class="reconciliationReport.is_balanced ? 'card-balanced' : 'card-discrepancy'">
        <template #header>
          <div class="reconcile-header">
            <span>
              <el-icon v-if="reconciliationReport.is_balanced" color="#67c23a"><CircleCheckFilled /></el-icon>
              <el-icon v-else color="#f56c6c"><WarningFilled /></el-icon>
              <strong>{{ $t('双向记账对账平账报告 (Reconciliation Report)') }}</strong>
            </span>
            <el-tag :type="reconciliationReport.is_balanced ? 'success' : 'danger'" effect="dark">
              {{ reconciliationReport.is_balanced ? $t('对账平账无差异 (Balanced)') : $t('检出账本差异 (Discrepancy)') }}
            </el-tag>
          </div>
        </template>
        <el-descriptions :column="4" border size="small">
          <el-descriptions-item :label="$t('钱包可用缓存')">
            {{ currencySymbol }}{{ formatUSDC(reconciliationReport.available_cache) }} {{ currencyUnit }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('钱包冻结缓存')">
            {{ currencySymbol }}{{ formatUSDC(reconciliationReport.held_cache) }} {{ currencyUnit }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('不可变流水总和')">
            {{ currencySymbol }}{{ formatUSDC(reconciliationReport.sum_ledger_entries) }} {{ currencyUnit }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('对账差额 (Discrepancy)')">
            <strong :style="{ color: reconciliationReport.discrepancy === 0 ? '#67c23a' : '#f56c6c' }">
              {{ currencySymbol }}{{ formatUSDC(reconciliationReport.discrepancy) }} {{ currencyUnit }}
            </strong>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>
    </div>

    <!-- Main Content: Tabs for Ledger, Topups, Consistency Checks -->
    <el-card shadow="never" class="main-card">
      <el-tabs v-model="activeTab">
        <!-- Tab 1: Ledger Entries -->
        <el-tab-pane :label="$t('只追加账本流水 (Ledger Entries)')" name="ledger">
          <div class="filter-bar">
            <div class="filter-left">
              <el-select v-model="ledgerTypeFilter" :placeholder="$t('交易类型')" style="width: 170px">
                <el-option :label="$t('全部类型')" value="all" />
                <el-option :label="$t('充值入账 (topup_credit)')" value="topup_credit" />
                <el-option :label="$t('结算扣费 (capture)')" value="capture" />
                <el-option :label="$t('调账变更 (adjustment)')" value="adjustment" />
                <el-option :label="$t('冲正回滚 (reversal)')" value="reversal" />
                <el-option :label="$t('解除冻结 (hold_release)')" value="hold_release" />
              </el-select>

              <el-input
                v-model="ledgerSearchQuery"
                :placeholder="$t('搜索流水号 / 业务ID / 原因...')"
                prefix-icon="Search"
                clearable
                style="width: 280px"
              />
            </div>
            <div class="filter-right">
              <span class="count-tag">{{ $t('共') }} {{ filteredLedgerEntries.length }} {{ $t('笔记录') }}</span>
            </div>
          </div>

          <el-table :data="paginatedLedgerEntries" stripe style="width: 100%" :empty-text="$t('暂无账本流水')">
            <el-table-column prop="entry_id" :label="$t('流水单号')" width="180">
              <template #default="{ row }">
                <code class="code-id">{{ row.entry_id }}</code>
              </template>
            </el-table-column>

            <el-table-column prop="type" :label="$t('交易类型')" width="140">
              <template #default="{ row }">
                <el-tag :type="getLedgerTagType(row.type)" size="small">
                  {{ row.type }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column :label="$t('发生金额')" width="150">
              <template #default="{ row }">
                <span
                  :style="{
                    color: row.amount >= 0 ? '#67c23a' : '#f56c6c',
                    fontWeight: 'bold',
                    fontSize: '14px'
                  }"
                >
                  {{ row.amount >= 0 ? '+' : '' }}{{ formatUSDC(row.amount) }} {{ currencyUnit }}
                </span>
              </template>
            </el-table-column>

            <el-table-column :label="$t('变动后结余')" width="140">
              <template #default="{ row }">
                <span v-if="row.balance_after !== undefined">
                  {{ formatUSDC(row.balance_after) }} {{ currencyUnit }}
                </span>
                <span v-else class="text-muted">-</span>
              </template>
            </el-table-column>

            <el-table-column :label="$t('业务关联 (Ref)')" width="180">
              <template #default="{ row }">
                <div v-if="row.ref_id">
                  <el-tag size="small" type="info">{{ row.ref_type }}</el-tag>
                  <div class="text-muted code-id" style="font-size: 11px">{{ row.ref_id }}</div>
                </div>
                <span v-else class="text-muted">-</span>
              </template>
            </el-table-column>

            <el-table-column prop="reason" :label="$t('摘要说明与审计原因')" min-width="220">
              <template #default="{ row }">
                <div>{{ row.reason || $t('自动结算') }}</div>
                <div v-if="row.operator_id" class="text-muted" style="font-size: 11px">
                  {{ $t('经办人:') }} {{ row.operator_id }}
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="$t('记账时间')" width="170">
              <template #default="{ row }">
                <span style="font-size: 12px">{{ formatDate(row.created_at) }}</span>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-box" v-if="filteredLedgerEntries.length > ledgerPageSize">
            <el-pagination
              v-model:current-page="ledgerPage"
              v-model:page-size="ledgerPageSize"
              :total="filteredLedgerEntries.length"
              layout="prev, pager, next"
            />
          </div>
        </el-tab-pane>

        <!-- Tab 2: TopUp Orders -->
        <el-tab-pane :label="$t('充值记录 (TopUp Orders)')" name="topup">
          <div class="filter-bar">
            <div class="filter-left">
              <el-select v-model="topupChannelFilter" :placeholder="$t('充值渠道')" style="width: 160px">
                <el-option :label="$t('全部渠道')" value="all" />
                <el-option :label="$t('管理员发放 (admin_grant)')" value="admin_grant" />
                <el-option :label="$t('链上充值 (onchain_deposit)')" value="onchain_deposit" />
                <el-option :label="$t('法币网关 (fiat)')" value="fiat" />
              </el-select>

              <el-select v-model="topupStatusFilter" :placeholder="$t('订单状态')" style="width: 140px">
                <el-option :label="$t('全部状态')" value="all" />
                <el-option :label="$t('已确认 (confirmed)')" value="confirmed" />
                <el-option :label="$t('处理中 (pending)')" value="pending" />
                <el-option :label="$t('失败 (failed)')" value="failed" />
                <el-option :label="$t('冲正回滚 (reversed)')" value="reversed" />
              </el-select>
            </div>
            <div class="filter-right">
              <span class="count-tag">{{ $t('共') }} {{ filteredTopUpOrders.length }} {{ $t('笔充值记录') }}</span>
            </div>
          </div>

          <el-table :data="paginatedTopUpOrders" stripe style="width: 100%" :empty-text="$t('暂无充值记录')">
            <el-table-column prop="topup_id" :label="$t('充值单号')" width="180">
              <template #default="{ row }">
                <code class="code-id">{{ row.topup_id }}</code>
              </template>
            </el-table-column>

            <el-table-column prop="channel" :label="$t('充值渠道')" width="160">
              <template #default="{ row }">
                <el-tag v-if="row.channel === 'admin_grant'" type="primary" size="small">
                  {{ $t('管理员发放') }}
                </el-tag>
                <el-tag v-else-if="row.channel === 'onchain_deposit'" type="success" size="small">
                  {{ $t('链上入账') }}
                </el-tag>
                <el-tag v-else type="info" size="small">
                  {{ $t('法币渠道') }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column :label="$t('充值金额')" width="150">
              <template #default="{ row }">
                <strong style="color: #67c23a; font-size: 14px">
                  +{{ formatUSDC(row.amount) }} {{ (row.asset && row.asset !== 'USDC') ? row.asset : currencyUnit }}
                </strong>
              </template>
            </el-table-column>

            <el-table-column prop="status" :label="$t('状态')" width="130">
              <template #default="{ row }">
                <el-tag :type="getTopupStatusType(row.status)" size="small">
                  {{ row.status }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column :label="$t('外部凭据 / TxHash')" min-width="200">
              <template #default="{ row }">
                <el-tooltip v-if="row.external_ref" :content="row.external_ref" placement="top">
                  <code class="code-id">{{ row.external_ref.slice(0, 18) }}...</code>
                </el-tooltip>
                <span v-else class="text-muted">-</span>
              </template>
            </el-table-column>

            <el-table-column prop="reason" :label="$t('发放原因与说明')" min-width="200">
              <template #default="{ row }">
                <div>{{ row.reason || '-' }}</div>
                <div v-if="row.operator_id" class="text-muted" style="font-size: 11px">
                  {{ $t('经办人:') }} {{ row.operator_id }}
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="$t('下单时间')" width="170">
              <template #default="{ row }">
                <span style="font-size: 12px">{{ formatDate(row.created_at) }}</span>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-box" v-if="filteredTopUpOrders.length > topupPageSize">
            <el-pagination
              v-model:current-page="topupPage"
              v-model:page-size="topupPageSize"
              :total="filteredTopUpOrders.length"
              layout="prev, pager, next"
            />
          </div>
        </el-tab-pane>

        <!-- Tab 3: Consistency Invariants -->
        <el-tab-pane :label="$t('账本一致性规则与安全保障 (Invariants)')" name="invariants">
          <div class="invariants-grid">
            <el-card shadow="never" class="inv-card">
              <template #header>
                <span class="inv-title">{{ $t('1. 可用余额非负守卫 (Non-Negative Invariant)') }}</span>
              </template>
              <p>{{ $t('任何时刻：') }}<code>{{ $t('available = SUM(入账) - SUM(扣费) - 当前Held总额') }}</code>{{ $t('。发起 Hold 冻结必须满足余额充足，严禁产生透支。') }}</p>
            </el-card>

            <el-card shadow="never" class="inv-card">
              <template #header>
                <span class="inv-title">{{ $t('2. 账本只追加不可篡改 (Append-Only Invariant)') }}</span>
              </template>
              <p><code>LedgerEntry</code> {{ $t('严禁 UPDATE 或 DELETE。更正历史错误必须通过生成') }} <code>reversal</code> {{ $t('或') }} <code>adjustment</code> {{ $t('补偿流水追加。') }}</p>
            </el-card>

            <el-card shadow="never" class="inv-card">
              <template #header>
                <span class="inv-title">{{ $t('3. 强幂等记账守卫 (Idempotency Invariant)') }}</span>
              </template>
              <p>{{ $t('同一') }} <code>wallet_id</code> {{ $t('内对相同的') }} <code>idempotency_key</code> {{ $t('具备唯一约束，重复调用直接返回原始记账记录，防止重复扣费或充值。') }}</p>
            </el-card>

            <el-card shadow="never" class="inv-card">
              <template #header>
                <span class="inv-title">{{ $t('4. W6 欠费自动熔断与解冻防御 (Arrears Circuit Breaker)') }}</span>
              </template>
              <p>{{ $t('因冲正退款导致可用余额为负时，账户自动进入') }} <code>frozen_arrears</code> {{ $t('状态。必须充值平账至非负后才允许解冻。') }}</p>
            </el-card>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- Dialog: Admin Grant -->
    <el-dialog
      v-model="grantDialogVisible"
      :title="$t('管理员发放测试与运营额度 (Admin Grant)')"
      width="540px"
      :close-on-click-modal="false"
    >
      <el-form ref="grantFormRef" :model="grantForm" :rules="grantRules" label-width="110px">
        <el-form-item :label="$t('归属租户')">
          <el-input :value="`${tenantStore.currentTenant?.name} (${tenantStore.currentTenantId})`" disabled />
        </el-form-item>

        <el-form-item :label="$t('发放金额')" prop="amountUsdc">
          <el-input-number
            v-model="grantForm.amountUsdc"
            :min="0.01"
            :max="100000"
            :step="10"
            :precision="2"
            style="width: 200px"
          />
          <span style="margin-left: 10px; font-weight: 500">{{ currencyUnit }}</span>
          <div class="form-tip">
            换算为{{ $t('基础最小单位:') }} <code>{{ formatBaseUnits(Math.round(grantForm.amountUsdc * 1000000)) }}</code>
          </div>
        </el-form-item>

        <el-form-item :label="$t('发放原因')" prop="reason">
          <el-input
            v-model="grantForm.reason"
            type="textarea"
            :rows="3"
            :placeholder="$t('必须填写发放原因与依据，该记录将永久记录在审计日志中 (必填)')"
          />
        </el-form-item>

        <el-form-item :label="$t('幂等键')">
          <el-input v-model="grantForm.idempotency_key" :placeholder="$t('自动生成幂等键')" />
          <div class="form-tip">{{ $t('防止网络抖动导致重复发放') }}</div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="grantDialogVisible = false">{{ $t('取消') }}</el-button>
        <el-button type="primary" :loading="grantSubmitting" @click="handleGrantSubmit">
          {{ $t('确认发放入账') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Dialog: Manual Adjustment -->
    <el-dialog
      v-model="adjustDialogVisible"
      :title="$t('人工调账与差错更正 (Manual Adjustment)')"
      width="560px"
      :close-on-click-modal="false"
    >
      <el-form ref="adjustFormRef" :model="adjustForm" :rules="adjustRules" label-width="110px">
        <el-alert
          v-if="adjustForm.amountUsdc >= 1000"
          :title="`${$t('⚠️ 大额调账警示')} (>= 1,000 ${currencyUnit})`"
          type="error"
          show-icon
          :closable="false"
          style="margin-bottom: 16px"
        >
          <template #default>
            {{ $t('根据内部安全风控策略，调账金额达到或超过 1,000') }} {{ currencyUnit }} {{ $t('时，必须填写独立复核审批人 (approved_by) 执行双人双控复核！') }}
          </template>
        </el-alert>

        <el-form-item :label="$t('调账方向')">
          <el-radio-group v-model="adjustForm.direction">
            <el-radio-button value="increase">{{ $t('增加余额 (+)') }}</el-radio-button>
            <el-radio-button value="decrease">{{ $t('扣减余额 (-)') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <el-form-item :label="$t('调账金额')" prop="amountUsdc">
          <el-input-number
            v-model="adjustForm.amountUsdc"
            :min="0.01"
            :max="50000"
            :step="5"
            :precision="2"
            style="width: 200px"
          />
          <span style="margin-left: 10px; font-weight: 500">{{ currencyUnit }}</span>
        </el-form-item>

        <el-form-item :label="$t('调账原因')" prop="reason">
          <el-input
            v-model="adjustForm.reason"
            type="textarea"
            :rows="3"
            :placeholder="$t('必须明确填写人工调账原因与审计说明 (必填)')"
          />
        </el-form-item>

        <el-form-item :label="$t('复核审批人')" prop="approved_by">
          <el-input
            v-model="adjustForm.approved_by"
            :placeholder="$t('大额调账时必填，如: risk_officer_bob')"
          />
        </el-form-item>

        <el-form-item :label="$t('幂等键')">
          <el-input v-model="adjustForm.idempotency_key" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="adjustDialogVisible = false">{{ $t('取消') }}</el-button>
        <el-button type="warning" :loading="adjustSubmitting" @click="handleAdjustSubmit">
          {{ $t('确认执行调账') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Dialog: Freeze / Unfreeze -->
    <el-dialog
      v-model="freezeDialogVisible"
      :title="freezeForm.action === 'freeze' ? $t('冻结租户虚拟钱包') : $t('解冻租户虚拟钱包')"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form ref="freezeFormRef" :model="freezeForm" :rules="freezeRules" label-width="90px">
        <div v-if="freezeForm.action === 'freeze'">
          <el-alert
            :title="$t('冻结后将暂停所有扣费、支付意图创建与配额申请，请谨慎操作。')"
            type="warning"
            show-icon
            :closable="false"
            style="margin-bottom: 16px"
          />
        </div>
        <div v-else>
          <el-alert
            v-if="wallet?.status === 'frozen_arrears' && (wallet?.available_cache || 0) < 0"
            :title="$t('⚠️ 欠费解冻限制')"
            type="error"
            show-icon
            :closable="false"
            style="margin-bottom: 16px"
          >
            <template #default>
              {{ $t('当前钱包可用余额为负值') }} ({{ formatUSDC(wallet?.available_cache) }} {{ currencyUnit }})，{{ $t('严禁执行解冻操作。请先充值平账！') }}
            </template>
          </el-alert>
          <el-alert
            v-else
            :title="$t('解冻后钱包将恢复正常状态，可继续进行 API 扣款消费。')"
            type="info"
            show-icon
            :closable="false"
            style="margin-bottom: 16px"
          />
        </div>

        <el-form-item :label="$t('操作原因')" prop="reason">
          <el-input
            v-model="freezeForm.reason"
            type="textarea"
            :rows="3"
            :placeholder="$t('请填写冻结或解冻的原因说明，纳入安全审计记录')"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="freezeDialogVisible = false">{{ $t('取消') }}</el-button>
        <el-button
          :type="freezeForm.action === 'freeze' ? 'danger' : 'success'"
          :loading="freezeSubmitting"
          :disabled="freezeForm.action === 'unfreeze' && wallet?.status === 'frozen_arrears' && (wallet?.available_cache || 0) < 0"
          @click="handleFreezeSubmit"
        >
          {{ freezeForm.action === 'freeze' ? $t('冻结') : $t('解冻') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.wallet-container {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.title {
  margin: 0 0 6px 0;
  font-size: 20px;
  font-weight: 600;
  color: #303133;
}

.subtitle {
  font-size: 13px;
  color: #909399;
}

.divider {
  margin: 0 8px;
  color: #dcdfe6;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.alert-box {
  margin-bottom: 16px;
}

.alert-desc {
  margin: 4px 0 0 0;
  font-size: 13px;
  line-height: 1.6;
}

.kpi-row {
  margin-bottom: 16px;
}

.kpi-card {
  border-radius: 8px;
  height: 100%;
}

.kpi-label {
  font-size: 13px;
  color: #909399;
  display: flex;
  align-items: center;
  gap: 4px;
}

.kpi-val {
  font-size: 24px;
  font-weight: 700;
  color: #303133;
  margin: 8px 0 4px 0;
}

.unit {
  font-size: 14px;
  font-weight: 500;
  color: #606266;
  margin-left: 4px;
}

.kpi-status-box {
  margin: 8px 0 4px 0;
  min-height: 36px;
  display: flex;
  align-items: center;
}

.kpi-sub {
  font-size: 12px;
  color: #909399;
}

.text-danger {
  color: #f56c6c !important;
}

.text-success {
  color: #67c23a !important;
}

.text-warning {
  color: #e6a23c !important;
}

.text-muted {
  color: #909399;
}

.code-id {
  font-family: monospace;
  font-size: 12px;
  background-color: #f4f4f5;
  padding: 2px 4px;
  border-radius: 4px;
  color: #606266;
}

.reconcile-box {
  margin-bottom: 16px;
}

.card-balanced {
  border-left: 4px solid #67c23a;
}

.card-discrepancy {
  border-left: 4px solid #f56c6c;
}

.reconcile-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.reconcile-header span {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
}

.main-card {
  border-radius: 8px;
}

.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.filter-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-right {
  font-size: 13px;
  color: #909399;
}

.count-tag {
  background: #f4f4f5;
  padding: 4px 8px;
  border-radius: 4px;
}

.pagination-box {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.invariants-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
  padding: 8px 0;
}

.inv-card {
  border-radius: 6px;
  background: #fafafa;
}

.inv-title {
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}

.inv-card p {
  margin: 0;
  font-size: 13px;
  color: #606266;
  line-height: 1.6;
}
</style>
