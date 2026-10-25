<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import PermissionGate from '@/components/PermissionGate.vue';
import { useI18n } from '@/i18n';
import {
  routeApi,
  walletApi,
  configApi,
  agentApi,
  paymentApi
} from '@/api/client';
import type { Route } from '@/types/route';
import type { Wallet } from '@/types/wallet';
import type { ConfigVersion } from '@/types/config';
import type { Agent } from '@/types/agent';
import type { PaymentAuditRecord, DisplayStatus } from '@/types/payment';

const authStore = useAuthStore();
const tenantStore = useTenantStore();
const { t } = useI18n();

const loading = ref(false);
const routes = ref<Route[]>([]);
const wallet = ref<Wallet | null>(null);
const versions = ref<ConfigVersion[]>([]);
const agents = ref<Agent[]>([]);
const payments = ref<PaymentAuditRecord[]>([]);

async function loadDashboardData() {
  const tenantId = tenantStore.currentTenantId;
  const projectId = tenantStore.currentProjectId;
  if (!tenantId) return;

  loading.value = true;
  try {
    const [routesRes, walletRes, versionsRes, agentsRes, paymentsRes] = await Promise.allSettled([
      routeApi.getRoutes(tenantId, projectId),
      walletApi.getTenantWallet(tenantId),
      configApi.getVersions(tenantId),
      agentApi.listAgents(tenantId, projectId),
      paymentApi.listPayments(tenantId, { limit: 5, project_id: projectId })
    ]);

    if (routesRes.status === 'fulfilled') {
      routes.value = routesRes.value || [];
    }
    if (walletRes.status === 'fulfilled') {
      wallet.value = walletRes.value || null;
    }
    if (versionsRes.status === 'fulfilled') {
      versions.value = versionsRes.value || [];
    }
    if (agentsRes.status === 'fulfilled') {
      agents.value = agentsRes.value || [];
    }
    if (paymentsRes.status === 'fulfilled') {
      payments.value = paymentsRes.value?.payments || [];
    }
  } catch (err) {
    console.error('Failed to load dashboard telemetry:', err);
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  loadDashboardData();
});

watch(
  () => [tenantStore.currentTenantId, tenantStore.currentProjectId],
  () => {
    loadDashboardData();
  }
);

// Route Metrics
const activeRoutes = computed(() => routes.value.filter((r) => r.status === 'active'));
const activeRoutesCount = computed(() => activeRoutes.value.length);
const totalRoutesCount = computed(() => routes.value.length);

// Wallet Metrics
const walletAvailable = computed(() => wallet.value?.available_cache || 0);
const walletHeld = computed(() => wallet.value?.held_cache || 0);

// Config Version Metrics
const activeVersion = computed(() => {
  return versions.value.find((v) => v.is_active) || versions.value[0] || null;
});

// Agent Metrics
const activeAgents = computed(() => agents.value.filter((a) => a.status === 'active'));
const activeAgentsCount = computed(() => activeAgents.value.length);
const totalAgentsCount = computed(() => agents.value.length);
const totalAgentRequests = computed(() => agents.value.reduce((acc, a) => acc + (a.requests_count || 0), 0));

// Project Budget Computations
const currentProject = computed(() => tenantStore.currentProject);
const projectBudgetCap = computed(() => currentProject.value?.budget_cap_base_units || 0);
const projectSpent = computed(() => {
  const pId = currentProject.value?.project_id;
  const projectAgents = pId ? agents.value.filter((a) => a.project_id === pId) : agents.value;
  return projectAgents.reduce((acc, a) => acc + (a.total_spend_base_units || 0), 0);
});
const projectBudgetPercent = computed(() => {
  if (!projectBudgetCap.value || projectBudgetCap.value <= 0) return 0;
  return Math.min(100, Math.round((projectSpent.value / projectBudgetCap.value) * 1000) / 10);
});
const budgetProgressStatus = computed(() => {
  if (projectBudgetPercent.value >= 90) return 'exception';
  if (projectBudgetPercent.value >= 75) return 'warning';
  return 'success';
});

// Formatter Helpers
function formatUSDC(baseUnits: number | undefined): string {
  if (baseUnits === undefined || isNaN(baseUnits)) return '0.00';
  const val = baseUnits / 1000000;
  return val.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 4
  });
}

function formatTime(timeStr?: string | number): string {
  if (!timeStr) return '-';
  try {
    const d = typeof timeStr === 'number' ? new Date(timeStr * 1000) : new Date(timeStr);
    return d.toLocaleString('zh-CN', { hour12: false });
  } catch {
    return String(timeStr);
  }
}

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

// RBAC Matrix
const rolePermissionsList = computed(() => {
  const role = authStore.currentRole;
  switch (role) {
    case 'owner':
      return [
        { name: t('租户与组织管理'), allowed: true, detail: t('创建项目、邀请成员、转让所有权') },
        { name: t('dashboard.routesModule'), allowed: true, detail: t('dashboard.routesDetail') },
        { name: t('dashboard.versionsModule'), allowed: true, detail: t('dashboard.versionsDetail') },
        { name: t('dashboard.walletModule'), allowed: true, detail: t('dashboard.walletDetail') },
        { name: t('dashboard.agentsModule'), allowed: true, detail: t('dashboard.agentsDetail') }
      ];
    case 'admin':
      return [
        { name: t('租户与组织管理'), allowed: false, detail: t('只读租户基本信息，不能转让租户') },
        { name: t('dashboard.routesModule'), allowed: true, detail: t('dashboard.routesDetail') },
        { name: t('dashboard.versionsModule'), allowed: true, detail: t('dashboard.versionsDetail') },
        { name: t('dashboard.walletModule'), allowed: false, detail: t('仅可查看余额与流水，不可直接手动调账') },
        { name: t('dashboard.agentsModule'), allowed: true, detail: t('dashboard.agentsDetail') }
      ];
    case 'operator':
      return [
        { name: t('租户与组织管理'), allowed: false, detail: t('无权限') },
        { name: t('dashboard.routesModule'), allowed: true, detail: t('dashboard.routesDetail') },
        { name: t('dashboard.versionsModule'), allowed: false, detail: t('仅可查看版本差异，无权触发发布与回滚') },
        { name: t('dashboard.walletModule'), allowed: false, detail: t('只读钱包流水与余额') },
        { name: t('dashboard.agentsModule'), allowed: false, detail: t('只读 Agent 时间线，无权触发紧急停用') }
      ];
    case 'viewer':
    default:
      return [
        { name: t('租户与组织管理'), allowed: false, detail: t('无权限') },
        { name: t('dashboard.routesModule'), allowed: false, detail: t('只读已有路由配置，禁止修改') },
        { name: t('dashboard.versionsModule'), allowed: false, detail: t('无权进入发布管理页面 (403 阻断)') },
        { name: t('dashboard.walletModule'), allowed: false, detail: t('只读钱包余额与流水') },
        { name: t('dashboard.agentsModule'), allowed: false, detail: t('只读 Agent 列表，无权修改状态') }
      ];
  }
});
</script>

<template>
  <div class="dashboard-page" v-loading="loading">
    <!-- Top Welcome Banner -->
    <el-card shadow="never" class="welcome-card">
      <div class="welcome-inner">
        <div>
          <h2 class="welcome-title">{{ $t('欢迎使用 AgentPay 管理控制台') }}</h2>
          <p class="welcome-desc">
            {{ $t('面向自主 AI Agent 的开源 HTTP 402 / x402 V2 支付基础设施与财务防火墙。') }}
          </p>
        </div>
        <div class="scope-actions">
          <div class="current-scope-tags">
            <el-tag size="large" effect="plain" type="primary">
              {{ $t('当前租户:') }} {{ authStore.currentTenant?.name || tenantStore.currentTenant?.name || 'AgentPay Default' }} ({{ authStore.currentTenant?.tenant_id || tenantStore.currentTenantId }})
            </el-tag>
            <el-tag size="large" effect="plain" type="success" style="margin-left: 10px">
              {{ $t('当前项目:') }} {{ tenantStore.currentProject?.name || $t('默认项目') }} ({{ tenantStore.currentProjectId || 'prj_default' }})
            </el-tag>
          </div>
          <el-button icon="Refresh" circle :loading="loading" @click="loadDashboardData" :title="$t('刷新数据')" />
        </div>
      </div>
    </el-card>

    <!-- Metrics Grid -->
    <el-row :gutter="16" class="metrics-row">
      <!-- Active Routes -->
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-header">
            <span class="metric-title">{{ t('dashboard.activeRoutes') }}</span>
            <el-icon class="metric-icon color-blue"><Connection /></el-icon>
          </div>
          <div class="metric-val">
            {{ activeRoutesCount }}
            <span class="metric-unit">/ {{ totalRoutesCount }}</span>
          </div>
          <div class="metric-sub">
            <span :class="activeRoutesCount > 0 ? 'status-dot online' : 'status-dot offline'"></span>
            {{ activeRoutesCount > 0 ? $t('网关路由运行中') : $t('暂无活动路由') }}
          </div>
        </el-card>
      </el-col>

      <!-- Tenant Wallet Balance -->
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-header">
            <span class="metric-title">{{ $t('租户钱包') }} ({{ tenantStore.currentCurrency }})</span>
            <el-icon class="metric-icon color-green"><Wallet /></el-icon>
          </div>
          <div class="metric-val">
            {{ tenantStore.currencySymbol }}{{ formatUSDC(walletAvailable) }}
            <span class="metric-unit">{{ tenantStore.currentCurrency }}</span>
          </div>
          <div class="metric-sub">
            {{ $t('冻结额度') }}: {{ tenantStore.currencySymbol }}{{ formatUSDC(walletHeld) }}
          </div>
        </el-card>
      </el-col>

      <!-- Gateway Snapshot Version -->
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-header">
            <span class="metric-title">{{ t('dashboard.publishedVersion') }}</span>
            <el-icon class="metric-icon color-orange"><Files /></el-icon>
          </div>
          <div class="metric-val">
            {{ activeVersion ? `v${activeVersion.version_number}` : 'v1.0.0' }}
          </div>
          <div class="metric-sub">
            {{ activeVersion?.snapshot_hash ? `${$t('快照')}: ${activeVersion.snapshot_hash.slice(0, 8)}...` : $t('热加载原子生效') }}
          </div>
        </el-card>
      </el-col>

      <!-- Registered Agents -->
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-header">
            <span class="metric-title">{{ t('dashboard.registeredAgents') }}</span>
            <el-icon class="metric-icon color-purple"><Cpu /></el-icon>
          </div>
          <div class="metric-val">
            {{ totalAgentsCount }}
            <span class="metric-unit">{{ $t('个') }}</span>
          </div>
          <div class="metric-sub">
            {{ $t('活跃') }}: {{ activeAgentsCount }} | {{ $t('总调用') }}: {{ totalAgentRequests.toLocaleString() }}
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Project Budget & Consumption Bar -->
    <el-card shadow="never" class="budget-card" style="margin-top: 16px;">
      <div class="budget-card-header">
        <div class="budget-info">
          <span class="budget-title">{{ $t('当前项目月度预算与消耗进度') }}</span>
          <span class="budget-project-name">「{{ currentProject?.name || $t('默认项目') }}」</span>
        </div>
        <div class="budget-stats">
          <span>{{ $t('已消耗') }}: <strong>{{ tenantStore.currencySymbol }}{{ formatUSDC(projectSpent) }}</strong></span>
          <span class="divider">/</span>
          <span>{{ $t('预算上限') }}: <strong>{{ tenantStore.currencySymbol }}{{ formatUSDC(projectBudgetCap) }}</strong></span>
          <el-tag size="small" :type="budgetProgressStatus" style="margin-left: 12px">
            {{ projectBudgetPercent }}%
          </el-tag>
        </div>
      </div>
      <el-progress
        :percentage="projectBudgetPercent"
        :status="budgetProgressStatus"
        :stroke-width="10"
        :show-text="false"
        style="margin-top: 10px;"
      />
    </el-card>

    <!-- Telemetry Data Grid: Recent Payments & Proxy Routes -->
    <el-row :gutter="16" style="margin-top: 16px;">
      <!-- Recent Payments & Fulfillment Stream -->
      <el-col :xs="24" :lg="14">
        <el-card shadow="never" class="data-card">
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <span>{{ $t('近期交易与履约审计') }}</span>
                <el-tag size="small" type="info" style="margin-left: 8px;">
                  {{ payments.length }} {{ $t('条记录') }}
                </el-tag>
              </div>
              <el-button link type="primary" size="small" @click="$router.push('/payments')">
                {{ $t('查看完整审计') }} &rarr;
              </el-button>
            </div>
          </template>

          <el-table :data="payments" size="small" style="width: 100%" empty-text="">
            <template #empty>
              <el-empty :description="$t('暂无近期交易流水')" :image-size="60" />
            </template>
            <el-table-column :label="$t('请求 ID')" min-width="140">
              <template #default="{ row }">
                <el-tooltip :content="row.request_id || row.intent_id" placement="top">
                  <span class="code-badge">{{ (row.request_id || row.intent_id || '').slice(0, 14) }}...</span>
                </el-tooltip>
              </template>
            </el-table-column>
            <el-table-column :label="$t('路由 / 目标')" min-width="130">
              <template #default="{ row }">
                <span class="route-badge">{{ row.route_id || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('扣费金额')" min-width="110" align="right">
              <template #default="{ row }">
                <span class="amount-val">
                  {{ tenantStore.currencySymbol }}{{ formatUSDC(row.amount_base_units) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('方式')" width="100" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.payment_method === 'wallet' ? 'primary' : 'warning'" effect="plain">
                  {{ row.payment_method === 'wallet' ? $t('虚拟钱包') : $t('x402 链上直付') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('状态')" width="110" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="getCompositeTagType(row.display_status)" effect="dark">
                  {{ row.display_status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('时间')" width="140" align="right">
              <template #default="{ row }">
                <span class="time-text">{{ formatTime(row.created_at) }}</span>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>

      <!-- Key Proxy Routes Snapshot -->
      <el-col :xs="24" :lg="10">
        <el-card shadow="never" class="data-card">
          <template #header>
            <div class="card-header">
              <span>{{ $t('核心代理路由速览') }}</span>
              <el-button link type="primary" size="small" @click="$router.push('/routes')">
                {{ $t('配置路由') }} &rarr;
              </el-button>
            </div>
          </template>

          <el-table :data="routes.slice(0, 5)" size="small" style="width: 100%" empty-text="">
            <template #empty>
              <el-empty :description="$t('暂无配置路由')" :image-size="60" />
            </template>
            <el-table-column :label="$t('路径匹配')" min-width="150">
              <template #default="{ row }">
                <div class="route-item">
                  <span class="route-pattern">{{ row.path_pattern }}</span>
                  <span class="route-service" v-if="row.service_name">{{ row.service_name }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="$t('状态')" width="80" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.status === 'active' ? 'success' : 'info'">
                  {{ row.status === 'active' ? $t('启用') : $t('停用') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('单价')" width="100" align="right">
              <template #default="{ row }">
                <span class="price-val">
                  {{ tenantStore.currencySymbol }}{{ formatUSDC(row.pricing?.base_price_usdc) }}
                </span>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <!-- Role RBAC Authorization Overview & Quick Actions -->
    <el-row :gutter="16" style="margin-top: 16px">
      <!-- RBAC Matrix -->
      <el-col :xs="24" :lg="14">
        <el-card shadow="never">
          <template #header>
            <div class="card-header">
              <span>{{ t('dashboard.rbacMatrix') }}</span>
              <el-tag size="small" type="warning">
                {{ t('layout.role') }}: {{ authStore.currentRole?.toUpperCase() }}
              </el-tag>
            </div>
          </template>
          <el-table :data="rolePermissionsList" size="small" style="width: 100%">
            <el-table-column prop="name" :label="t('dashboard.module')" width="200" />
            <el-table-column :label="t('dashboard.permission')" width="100">
              <template #default="{ row }">
                <el-tag :type="row.allowed ? 'success' : 'danger'" size="small">
                  {{ row.allowed ? t('dashboard.allowed') : t('dashboard.forbidden') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="detail" :label="t('dashboard.permissionDetail')" />
          </el-table>
        </el-card>
      </el-col>

      <!-- Quick Navigation -->
      <el-col :xs="24" :lg="10">
        <el-card shadow="never">
          <template #header>
            <span>{{ t('dashboard.quickNav') }}</span>
          </template>
          <div class="quick-nav-list">
            <router-link to="/members" class="nav-item">
              <el-icon><User /></el-icon>
              <div>
                <div class="nav-title">{{ t('menu.members') }}</div>
                <div class="nav-desc">{{ $t('查看团队成员、分配 RBAC 角色与开户授权') }}</div>
              </div>
            </router-link>

            <router-link to="/routes" class="nav-item">
              <el-icon><Connection /></el-icon>
              <div>
                <div class="nav-title">{{ t('menu.routes') }}</div>
                <div class="nav-desc">{{ $t('配置 API 端点、上游目标与 x402 价格') }}</div>
              </div>
            </router-link>

            <PermissionGate :roles="['owner', 'admin']">
              <router-link to="/versions" class="nav-item">
                <el-icon><Files /></el-icon>
                <div>
                  <div class="nav-title">{{ t('menu.versions') }}</div>
                  <div class="nav-desc">{{ $t('生成配置快照，安全推送到网关或一键回退') }}</div>
                </div>
              </router-link>
              <template #fallback>
                <div class="nav-item disabled">
                  <el-icon><Lock /></el-icon>
                  <div>
                    <div class="nav-title">{{ t('menu.versions') }} [{{ t('dashboard.locked') }}]</div>
                    <div class="nav-desc">{{ $t('当前角色') }} ({{ authStore.currentRole }}) {{ $t('无权执行发布') }}</div>
                  </div>
                </div>
              </template>
            </PermissionGate>

            <router-link to="/wallet" class="nav-item">
              <el-icon><Wallet /></el-icon>
              <div>
                <div class="nav-title">{{ t('menu.wallet') }}</div>
                <div class="nav-desc">{{ $t('可用/冻结余额展示，多维度流水记账') }}</div>
              </div>
            </router-link>

            <router-link to="/agents" class="nav-item">
              <el-icon><Cpu /></el-icon>
              <div>
                <div class="nav-title">{{ t('menu.agents') }}</div>
                <div class="nav-desc">{{ $t('会话级请求下钻，紧急失控 Kill Switch') }}</div>
              </div>
            </router-link>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.dashboard-page {
  padding-bottom: 24px;
}
.welcome-card {
  margin-bottom: 16px;
  background: #ffffff;
  border-radius: 8px;
}
.welcome-inner {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}
.welcome-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 6px 0;
  color: #303133;
}
.welcome-desc {
  font-size: 13px;
  color: #909399;
  margin: 0;
}
.scope-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.current-scope-tags {
  display: flex;
  align-items: center;
}
.metrics-row {
  margin-bottom: 0;
}
.metric-card {
  border-radius: 8px;
  margin-bottom: 8px;
}
.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.metric-title {
  font-size: 13px;
  color: #909399;
}
.metric-icon {
  font-size: 18px;
}
.color-blue { color: #409eff; }
.color-green { color: #67c23a; }
.color-orange { color: #e6a23c; }
.color-purple { color: #9c27b0; }

.metric-val {
  font-size: 24px;
  font-weight: 700;
  color: #303133;
  line-height: 1.2;
}
.metric-unit {
  font-size: 14px;
  font-weight: 400;
  color: #909399;
  margin-left: 4px;
}
.metric-sub {
  font-size: 12px;
  color: #606266;
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
}
.status-dot.online { background-color: #67c23a; }
.status-dot.offline { background-color: #909399; }

/* Budget Card */
.budget-card {
  border-radius: 8px;
}
.budget-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.budget-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}
.budget-project-name {
  font-size: 13px;
  color: #409eff;
  margin-left: 4px;
}
.budget-stats {
  font-size: 13px;
  color: #606266;
  display: flex;
  align-items: center;
}
.budget-stats .divider {
  margin: 0 8px;
  color: #dcdfe6;
}

/* Data Tables */
.data-card {
  border-radius: 8px;
  margin-bottom: 8px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}
.header-left {
  display: flex;
  align-items: center;
}
.code-badge {
  font-family: monospace;
  font-size: 12px;
  background: #f4f4f5;
  padding: 2px 6px;
  border-radius: 4px;
  color: #303133;
}
.route-badge {
  font-family: monospace;
  font-size: 12px;
  color: #409eff;
}
.amount-val {
  font-family: monospace;
  font-weight: 600;
  color: #303133;
}
.time-text {
  font-size: 12px;
  color: #909399;
}
.route-item {
  display: flex;
  flex-direction: column;
}
.route-pattern {
  font-family: monospace;
  font-size: 12px;
  font-weight: 600;
  color: #303133;
}
.route-service {
  font-size: 11px;
  color: #909399;
}
.price-val {
  font-family: monospace;
  font-size: 12px;
  color: #67c23a;
  font-weight: 600;
}

.quick-nav-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid #e4e7ed;
  text-decoration: none;
  color: inherit;
  transition: all 0.2s;
}
.nav-item:hover {
  background-color: #f5f7fa;
  border-color: #dcdfe6;
}
.nav-item.disabled {
  opacity: 0.6;
  cursor: not-allowed;
  background-color: #f5f7fa;
}
.nav-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}
.nav-desc {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}
</style>
