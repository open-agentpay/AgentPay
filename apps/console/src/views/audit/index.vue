<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { useTenantStore } from '@/stores/tenant';
import { auditApi } from '@/api/client';
import type { AuditEvent } from '@/types/audit';

const tenantStore = useTenantStore();

// State
const loading = ref(false);
const autoRefresh = ref(false);
let refreshTimer: any = null;

const auditEvents = ref<AuditEvent[]>([]);
const searchQuery = ref('');
const selectedTargetType = ref<string>('all');
const selectedActionCategory = ref<string>('all');

// Target types supported
const targetTypes = [
  { label: '全部类型', value: 'all' },
  { label: '智能体 (Agent)', value: 'Agent' },
  { label: '网关路由 (Route)', value: 'Route' },
  { label: '安全策略 (Policy)', value: 'Policy' },
  { label: '账户钱包 (Wallet)', value: 'Wallet' },
  { label: '发布配置 (Config)', value: 'Config' },
  { label: '空间项目 (Project)', value: 'Project' },
  { label: '成员权限 (Member)', value: 'Member' },
  { label: '租户设置 (Tenant)', value: 'Tenant' }
];

// Action categories
const actionCategories = [
  { label: '全部动作', value: 'all' },
  { label: '新建 (Create)', value: 'create' },
  { label: '修改 (Update)', value: 'update' },
  { label: '熔断/冻结 (Status)', value: 'status' },
  { label: '删除/注销 (Delete)', value: 'delete' }
];

// Modals / Drawers
const diffDrawerVisible = ref(false);
const rawJsonDialogVisible = ref(false);
const activeEvent = ref<AuditEvent | null>(null);

// Pagination
const currentPage = ref(1);
const pageSize = ref(15);

// Fetch events from API
async function fetchAuditLogs() {
  const tenantId = tenantStore.currentTenantId;
  if (!tenantId) return;

  loading.value = true;
  try {
    const params: { target_type?: string; limit?: number } = { limit: 100 };
    if (selectedTargetType.value !== 'all') {
      params.target_type = selectedTargetType.value;
    }
    const data = await auditApi.listEvents(tenantId, params);
    auditEvents.value = data;
  } catch (err: any) {
    ElMessage.error(err.message || '获取审计日志失败');
  } finally {
    loading.value = false;
  }
}

// Watch filters & tenant
watch(
  () => [tenantStore.currentTenantId, selectedTargetType.value],
  () => {
    currentPage.value = 1;
    fetchAuditLogs();
  }
);

// Toggle auto refresh
watch(autoRefresh, (enabled) => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
  if (enabled) {
    refreshTimer = setInterval(() => {
      fetchAuditLogs();
    }, 10000);
  }
});

onMounted(() => {
  fetchAuditLogs();
});

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
  }
});

// Filtered logs
const filteredEvents = computed(() => {
  let list = auditEvents.value;

  // Filter by category
  if (selectedActionCategory.value !== 'all') {
    const cat = selectedActionCategory.value.toLowerCase();
    list = list.filter((e) => {
      const act = e.action.toLowerCase();
      if (cat === 'create') return act.includes('create') || act.includes('add');
      if (cat === 'update') return act.includes('update') || act.includes('edit') || act.includes('patch');
      if (cat === 'status') return act.includes('status') || act.includes('suspend') || act.includes('freeze') || act.includes('kill');
      if (cat === 'delete') return act.includes('delete') || act.includes('revoke') || act.includes('remove');
      return true;
    });
  }

  // Filter by search query
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.trim().toLowerCase();
    list = list.filter(
      (e) =>
        e.audit_id.toLowerCase().includes(q) ||
        e.action.toLowerCase().includes(q) ||
        e.operator_id.toLowerCase().includes(q) ||
        e.target_id.toLowerCase().includes(q) ||
        e.target_type.toLowerCase().includes(q) ||
        (e.client_ip && e.client_ip.toLowerCase().includes(q))
    );
  }

  return list;
});

// Paginated logs
const paginatedEvents = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value;
  return filteredEvents.value.slice(start, start + pageSize.value);
});

// Metrics
const totalEventsCount = computed(() => auditEvents.value.length);
const criticalOpsCount = computed(() =>
  auditEvents.value.filter((e) => {
    const act = e.action.toLowerCase();
    return act.includes('suspend') || act.includes('freeze') || act.includes('kill') || act.includes('revoke');
  }).length
);
const distinctOperatorsCount = computed(() => {
  const set = new Set(auditEvents.value.map((e) => e.operator_id));
  return set.size;
});

// Helpers
function formatDateTime(val: string | number | undefined): string {
  if (!val) return '-';
  const num = typeof val === 'string' ? (isNaN(Number(val)) ? Date.parse(val) : Number(val)) : val;
  const d = num > 1e11 ? new Date(num) : new Date(num * 1000);
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

function getActionTagType(action: string): 'success' | 'warning' | 'danger' | 'info' | 'primary' {
  const act = action.toLowerCase();
  if (act.includes('create') || act.includes('publish') || act.includes('grant')) return 'success';
  if (act.includes('suspend') || act.includes('freeze') || act.includes('kill') || act.includes('reject') || act.includes('revoke'))
    return 'danger';
  if (act.includes('update') || act.includes('adjust')) return 'warning';
  return 'primary';
}

function copyText(text: string) {
  navigator.clipboard.writeText(text);
  ElMessage.success('已复制到剪贴板');
}

function openDiffDrawer(ev: AuditEvent) {
  activeEvent.value = ev;
  diffDrawerVisible.value = true;
}

function openRawJson(ev: AuditEvent) {
  activeEvent.value = ev;
  rawJsonDialogVisible.value = true;
}

function formatJson(obj: any): string {
  if (!obj) return '无数据 (空或未变更)';
  try {
    return JSON.stringify(obj, null, 2);
  } catch {
    return String(obj);
  }
}
</script>

<template>
  <div class="audit-management-page">
    <!-- Header Banner -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="title">{{ $t('操作与安全审计') }}</h2>
        <span class="subtitle">
          {{ $t('全链路操作不可篡改审计追踪，精确记录管控平面实体生命周期变更与操作源 IP') }}
        </span>
      </div>
      <div class="header-right">
        <el-switch
          v-model="autoRefresh"
          inline-prompt
          active-text="实时刷新 (10s)"
          inactive-text="手动刷新"
          style="margin-right: 12px"
        />
        <el-button type="primary" plain icon="Refresh" :loading="loading" @click="fetchAuditLogs">
          {{ $t('立即刷新') }}
        </el-button>
        <el-button type="warning" plain icon="Tickets" @click="$router.push('/payments')">
          {{ $t('财务扣费审计') }} &rarr;
        </el-button>
      </div>
    </div>

    <!-- Overview Stat Cards -->
    <el-row :gutter="16" class="stats-row">
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="stat-card">
          <div class="stat-title">{{ $t('审计事件总数 (最近100条)') }}</div>
          <div class="stat-value text-primary">{{ totalEventsCount }}</div>
          <div class="stat-desc">{{ $t('当前租户最近管控平面操作记录') }}</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="stat-card">
          <div class="stat-title">{{ $t('敏感与熔断操作') }}</div>
          <div class="stat-value text-danger">{{ criticalOpsCount }}</div>
          <div class="stat-desc">{{ $t('含熔断、冻结、销毁等高风险变更') }}</div>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="8">
        <el-card shadow="never" class="stat-card">
          <div class="stat-title">{{ $t('活跃操作主体数') }}</div>
          <div class="stat-value text-success">{{ distinctOperatorsCount }}</div>
          <div class="stat-desc">{{ $t('操作员或系统引擎唯一标识数') }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Main Table Card -->
    <el-card shadow="never" class="main-card">
      <!-- Filter Bar -->
      <div class="filter-bar">
        <div class="filter-left">
          <el-select v-model="selectedTargetType" style="width: 170px" placeholder="作用实体">
            <el-option
              v-for="item in targetTypes"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>

          <el-select v-model="selectedActionCategory" style="width: 160px; margin-left: 10px" placeholder="动作分类">
            <el-option
              v-for="item in actionCategories"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>

          <el-input
            v-model="searchQuery"
            placeholder="搜索操作动作、主体 ID、目标 ID 或 IP..."
            clearable
            prefix-icon="Search"
            style="width: 320px; margin-left: 10px"
          />
        </div>

        <div class="filter-right">
          <span class="record-count">共 {{ filteredEvents.length }} 条记录</span>
        </div>
      </div>

      <!-- Events Table -->
      <el-table
        v-loading="loading"
        :data="paginatedEvents"
        stripe
        style="width: 100%; margin-top: 12px"
        empty-text="暂无匹配的操作审计事件"
      >
        <!-- Time -->
        <el-table-column label="发生时间" width="180">
          <template #default="{ row }">
            <span class="mono-text">{{ formatDateTime(row.created_at) }}</span>
          </template>
        </el-table-column>

        <!-- Action -->
        <el-table-column label="操作事件" min-width="190">
          <template #default="{ row }">
            <el-tag :type="getActionTagType(row.action)" size="small" effect="plain" class="action-tag">
              {{ row.action }}
            </el-tag>
          </template>
        </el-table-column>

        <!-- Target -->
        <el-table-column label="作用实体" min-width="210">
          <template #default="{ row }">
            <div class="target-cell">
              <el-tag size="small" type="info" style="margin-right: 6px">{{ row.target_type }}</el-tag>
              <span class="mono-text target-id" :title="row.target_id">{{ row.target_id }}</span>
              <el-button link type="primary" icon="DocumentCopy" size="small" @click="copyText(row.target_id)" />
            </div>
          </template>
        </el-table-column>

        <!-- Operator -->
        <el-table-column label="操作人 / 主体" width="180">
          <template #default="{ row }">
            <span class="mono-text">{{ row.operator_id || 'system' }}</span>
          </template>
        </el-table-column>

        <!-- Client IP -->
        <el-table-column label="来源 IP" width="140">
          <template #default="{ row }">
            <el-tag size="small" type="info" effect="light" class="mono-text">
              {{ row.client_ip || '127.0.0.1' }}
            </el-tag>
          </template>
        </el-table-column>

        <!-- Operation -->
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" icon="View" @click="openDiffDrawer(row)">
              {{ $t('状态差异') }}
            </el-button>
            <el-button link type="info" size="small" icon="Document" @click="openRawJson(row)">
              {{ $t('原始报文') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[15, 30, 50, 100]"
          :total="filteredEvents.length"
          layout="total, sizes, prev, pager, next, jumper"
          background
        />
      </div>
    </el-card>

    <!-- State Diff Drawer -->
    <el-drawer
      v-model="diffDrawerVisible"
      title="操作前后状态对比 (State Diff)"
      size="55%"
      destroy-on-close
    >
      <div v-if="activeEvent" class="drawer-content">
        <!-- Event Meta Info -->
        <el-descriptions :column="2" border size="small" class="meta-desc">
          <el-descriptions-item label="审计事件 ID">
            <span class="mono-text">{{ activeEvent.audit_id }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="操作动作">
            <el-tag :type="getActionTagType(activeEvent.action)" size="small">
              {{ activeEvent.action }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="作用对象">
            {{ activeEvent.target_type }} (<code>{{ activeEvent.target_id }}</code>)
          </el-descriptions-item>
          <el-descriptions-item label="触发主体">
            <code>{{ activeEvent.operator_id }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="客户端 IP">
            <code>{{ activeEvent.client_ip }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="发生时间">
            {{ formatDateTime(activeEvent.created_at) }}
          </el-descriptions-item>
        </el-descriptions>

        <!-- Diff Columns -->
        <div class="diff-container">
          <div class="diff-box">
            <div class="diff-header before-header">变更前状态 (Before State)</div>
            <pre class="diff-body json-viewer">{{ formatJson(activeEvent.before_state) }}</pre>
          </div>
          <div class="diff-box">
            <div class="diff-header after-header">变更后状态 (After State)</div>
            <pre class="diff-body json-viewer">{{ formatJson(activeEvent.after_state) }}</pre>
          </div>
        </div>
      </div>
    </el-drawer>

    <!-- Raw JSON Dialog -->
    <el-dialog
      v-model="rawJsonDialogVisible"
      title="审计事件原始 JSON 报文"
      width="600px"
      destroy-on-close
    >
      <div v-if="activeEvent">
        <pre class="json-viewer" style="max-height: 480px; overflow-y: auto">{{ JSON.stringify(activeEvent, null, 2) }}</pre>
      </div>
      <template #footer>
        <el-button type="primary" plain @click="copyText(JSON.stringify(activeEvent, null, 2))">
          复制完整 JSON
        </el-button>
        <el-button @click="rawJsonDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.audit-management-page {
  padding: 4px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.title {
  font-size: 20px;
  font-weight: 600;
  color: #1f2937;
  margin: 0 0 4px 0;
}

.subtitle {
  font-size: 13px;
  color: #6b7280;
}

.header-right {
  display: flex;
  align-items: center;
}

.stats-row {
  margin-bottom: 16px;
}

.stat-card {
  border-radius: 6px;
  background-color: #fff;
}

.stat-title {
  font-size: 13px;
  color: #6b7280;
  margin-bottom: 6px;
}

.stat-value {
  font-size: 26px;
  font-weight: 700;
  margin-bottom: 4px;
}

.stat-desc {
  font-size: 12px;
  color: #9ca3af;
}

.text-primary {
  color: #409eff;
}

.text-danger {
  color: #f56c6c;
}

.text-success {
  color: #67c23a;
}

.main-card {
  border-radius: 6px;
}

.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 8px;
}

.filter-left {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.record-count {
  font-size: 13px;
  color: #909399;
}

.mono-text {
  font-family: SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
}

.action-tag {
  font-weight: 600;
}

.target-cell {
  display: flex;
  align-items: center;
  overflow: hidden;
}

.target-id {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #4b5563;
}

.pagination-wrapper {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.drawer-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.meta-desc {
  margin-bottom: 8px;
}

.diff-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.diff-box {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  overflow: hidden;
}

.diff-header {
  padding: 8px 12px;
  font-size: 13px;
  font-weight: 600;
  border-bottom: 1px solid #e5e7eb;
}

.before-header {
  background-color: #fef0f0;
  color: #f56c6c;
}

.after-header {
  background-color: #f0f9eb;
  color: #67c23a;
}

.diff-body {
  margin: 0;
  padding: 12px;
  height: 380px;
  overflow-y: auto;
  background-color: #f9fafb;
}

.json-viewer {
  font-family: SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  line-height: 1.5;
  color: #374151;
  background-color: #f9fafb;
  padding: 12px;
  border-radius: 4px;
  border: 1px solid #e5e7eb;
}
</style>
