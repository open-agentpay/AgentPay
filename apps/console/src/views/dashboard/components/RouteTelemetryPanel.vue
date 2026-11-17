<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter } from 'vue-router';
import { analyticsApi } from '@/api/client';
import type { RouteTelemetryOverviewResponse, RouteTelemetryItem } from '@/types/telemetry';

const props = defineProps<{
  tenantId: string;
  projectId?: string;
}>();

const router = useRouter();

const loading = ref(false);
const timeRange = ref('24h');
const telemetryData = ref<RouteTelemetryOverviewResponse | null>(null);

// 过滤、排序与分页状态
const searchKeyword = ref('');
const activeTab = ref<'all' | 'llm' | 'api' | 'warning'>('all');
const onlyActive = ref(true); // 默认隐藏 0 调用的冷路由
const sortBy = ref<'tokens_desc' | 'calls_desc' | 'latency_desc' | 'success_asc'>('tokens_desc');
const currentPage = ref(1);
const pageSize = ref(10);

const summary = computed(() => telemetryData.value?.summary || {
  total_calls: 0,
  success_rate: 100,
  avg_duration_ms: 0,
  p95_duration_ms: 0,
  total_tokens: 0,
  prompt_tokens: 0,
  completion_tokens: 0,
  failed_calls: 0,
});

const routes = computed<RouteTelemetryItem[]>(() => telemetryData.value?.routes || []);

async function fetchTelemetry() {
  if (!props.tenantId) return;
  loading.value = true;
  try {
    const res = await analyticsApi.getRouteTelemetry(props.tenantId, {
      time_range: timeRange.value,
      project_id: props.projectId
    });
    telemetryData.value = res;
  } catch (err) {
    console.error('Failed to load route telemetry:', err);
  } finally {
    loading.value = false;
  }
}

watch([() => props.tenantId, () => props.projectId, timeRange], () => {
  fetchTelemetry();
});

// 重置分页
watch([searchKeyword, activeTab, onlyActive, sortBy], () => {
  currentPage.value = 1;
});

onMounted(() => {
  fetchTelemetry();
});

function formatNumber(num: number): string {
  if (num >= 1_000_000) {
    return (num / 1_000_000).toFixed(2) + 'M';
  }
  if (num >= 1_000) {
    return (num / 1_000).toFixed(1) + 'k';
  }
  return num.toLocaleString();
}

function getSuccessRateType(rate: number): 'success' | 'warning' | 'danger' {
  if (rate >= 99.0) return 'success';
  if (rate >= 95.0) return 'warning';
  return 'danger';
}

function getLatencyType(ms: number): 'success' | 'warning' | 'danger' {
  if (ms <= 300) return 'success';
  if (ms <= 1000) return 'warning';
  return 'danger';
}

function navigateToRoute(routeId: string) {
  router.push({ path: '/routes', query: { search: routeId } });
}

function navigateToAllRoutes() {
  router.push({ path: '/routes' });
}

// 过滤与排序
const filteredAndSortedRoutes = computed(() => {
  let list = [...routes.value];

  // 1. 过滤仅活跃路由
  if (onlyActive.value) {
    list = list.filter(r => r.calls > 0 || r.total_tokens > 0);
  }

  // 2. 分类 Tab 过滤
  if (activeTab.value === 'llm') {
    list = list.filter(r => r.route_type === 'llm');
  } else if (activeTab.value === 'api') {
    list = list.filter(r => r.route_type !== 'llm');
  } else if (activeTab.value === 'warning') {
    list = list.filter(r => r.success_rate < 99.0 || (r.calls > 0 && r.avg_duration_ms > 1000));
  }

  // 3. 搜索过滤
  if (searchKeyword.value.trim()) {
    const q = searchKeyword.value.trim().toLowerCase();
    list = list.filter(r =>
      (r.title && r.title.toLowerCase().includes(q)) ||
      (r.model_name && r.model_name.toLowerCase().includes(q)) ||
      (r.route_id && r.route_id.toLowerCase().includes(q))
    );
  }

  // 4. 排序
  list.sort((a, b) => {
    switch (sortBy.value) {
      case 'tokens_desc':
        return (b.total_tokens || 0) - (a.total_tokens || 0);
      case 'calls_desc':
        return b.calls - a.calls;
      case 'latency_desc':
        return b.avg_duration_ms - a.avg_duration_ms;
      case 'success_asc':
        return a.success_rate - b.success_rate;
      default:
        return (b.total_tokens || 0) - (a.total_tokens || 0);
    }
  });

  return list;
});

// 分页数据
const paginatedRoutes = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value;
  return filteredAndSortedRoutes.value.slice(start, start + pageSize.value);
});

// 统计标签角标
const tabCounts = computed(() => {
  const all = routes.value;
  const activeAll = onlyActive.value ? all.filter(r => r.calls > 0 || r.total_tokens > 0) : all;
  return {
    all: activeAll.length,
    llm: activeAll.filter(r => r.route_type === 'llm').length,
    api: activeAll.filter(r => r.route_type !== 'llm').length,
    warning: activeAll.filter(r => r.success_rate < 99.0 || (r.calls > 0 && r.avg_duration_ms > 1000)).length,
  };
});

// 计算 Token 占总量的百分比
function getTokenSharePercent(tokens: number): number {
  if (!summary.value.total_tokens || summary.value.total_tokens <= 0 || tokens <= 0) {
    return 0;
  }
  const pct = (tokens / summary.value.total_tokens) * 100;
  return Math.min(100, Math.round(pct * 10) / 10);
}
</script>

<template>
  <el-card shadow="never" class="telemetry-panel-card" v-loading="loading">
    <template #header>
      <div class="panel-header">
        <div class="panel-title-area">
          <el-icon class="panel-icon color-blue"><TrendCharts /></el-icon>
          <span class="panel-title">{{ $t('路由性能与 LLM 遥测大盘') }}</span>
          <el-tag size="small" type="info" effect="plain" class="ttl-badge">
            {{ $t('高性能聚合 (TTL 15s)') }}
          </el-tag>
        </div>
        <div class="panel-actions">
          <el-radio-group v-model="timeRange" size="small" class="range-selector">
            <el-radio-button label="1h">1h</el-radio-button>
            <el-radio-button label="24h">24h</el-radio-button>
            <el-radio-button label="7d">7d</el-radio-button>
          </el-radio-group>
          <el-button
            circle
            size="small"
            :loading="loading"
            class="refresh-btn"
            @click="fetchTelemetry"
          >
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>
      </div>
    </template>

    <!-- 4 Core KPI Cards -->
    <el-row :gutter="14" class="kpi-grid">
      <!-- 1. Total Calls -->
      <el-col :xs="24" :sm="12" :md="6">
        <div class="kpi-card bg-blue-tint">
          <div class="kpi-header">
            <span class="kpi-label">{{ $t('调用总次数') }}</span>
            <el-icon class="kpi-icon color-blue"><Connection /></el-icon>
          </div>
          <div class="kpi-value">
            {{ summary.total_calls.toLocaleString() }}
            <span class="kpi-unit">{{ $t('次') }}</span>
          </div>
          <div class="kpi-sub">
            <span v-if="summary.failed_calls > 0" class="color-red">
              {{ $t('失败') }}: {{ summary.failed_calls.toLocaleString() }}
            </span>
            <span v-else class="color-green">
              {{ $t('全部成功履约') }}
            </span>
          </div>
        </div>
      </el-col>

      <!-- 2. LLM Tokens -->
      <el-col :xs="24" :sm="12" :md="6">
        <div class="kpi-card bg-purple-tint">
          <div class="kpi-header">
            <span class="kpi-label">{{ $t('LLM Token 总消耗') }}</span>
            <el-icon class="kpi-icon color-purple"><Cpu /></el-icon>
          </div>
          <div class="kpi-value">
            {{ formatNumber(summary.total_tokens) }}
            <span class="kpi-unit">Tokens</span>
          </div>
          <div class="kpi-sub">
            <el-tooltip :content="`输入: ${summary.prompt_tokens.toLocaleString()} | 输出: ${summary.completion_tokens.toLocaleString()}`" placement="top">
              <span>输入: {{ formatNumber(summary.prompt_tokens) }} / 输出: {{ formatNumber(summary.completion_tokens) }}</span>
            </el-tooltip>
          </div>
        </div>
      </el-col>

      <!-- 3. Success Rate -->
      <el-col :xs="24" :sm="12" :md="6">
        <div class="kpi-card bg-green-tint">
          <div class="kpi-header">
            <span class="kpi-label">{{ $t('整体成功率') }}</span>
            <el-icon class="kpi-icon color-green"><Check /></el-icon>
          </div>
          <div class="kpi-value">
            {{ summary.success_rate.toFixed(2) }}%
            <el-tag size="small" :type="getSuccessRateType(summary.success_rate)" effect="dark" class="rate-tag">
              {{ summary.success_rate >= 99 ? $t('稳定') : $t('关注') }}
            </el-tag>
          </div>
          <div class="kpi-sub">
            <el-progress
              :percentage="Math.min(100, Math.max(0, summary.success_rate))"
              :status="getSuccessRateType(summary.success_rate)"
              :stroke-width="5"
              :show-text="false"
              class="rate-progress"
            />
          </div>
        </div>
      </el-col>

      <!-- 4. Avg Latency -->
      <el-col :xs="24" :sm="12" :md="6">
        <div class="kpi-card bg-orange-tint">
          <div class="kpi-header">
            <span class="kpi-label">{{ $t('平均响应时间') }}</span>
            <el-icon class="kpi-icon color-orange"><Timer /></el-icon>
          </div>
          <div class="kpi-value">
            {{ summary.avg_duration_ms }}
            <span class="kpi-unit">ms</span>
            <el-tag size="small" :type="getLatencyType(summary.avg_duration_ms)" class="rate-tag">
              P95: {{ summary.p95_duration_ms }}ms
            </el-tag>
          </div>
          <div class="kpi-sub">
            <span>{{ summary.avg_duration_ms <= 300 ? $t('极速响应') : $t('慢请求已监控') }}</span>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- Multi-Dimensional Toolbar for Large Scale Routes -->
    <div class="toolbar-section">
      <!-- Left Category Tabs -->
      <el-radio-group v-model="activeTab" size="small" class="tab-filters">
        <el-radio-button label="all">
          {{ $t('全部路由') }} ({{ tabCounts.all }})
        </el-radio-button>
        <el-radio-button label="llm">
          🧠 {{ $t('仅LLM模型') }} ({{ tabCounts.llm }})
        </el-radio-button>
        <el-radio-button label="api">
          🔌 {{ $t('仅API接口') }} ({{ tabCounts.api }})
        </el-radio-button>
        <el-radio-button label="warning">
          ⚠️ {{ $t('异常关注') }}
          <span v-if="tabCounts.warning > 0" class="warn-dot">({{ tabCounts.warning }})</span>
        </el-radio-button>
      </el-radio-group>

      <!-- Right Controls -->
      <div class="toolbar-right">
        <el-checkbox v-model="onlyActive" size="small" class="active-checkbox">
          {{ $t('仅显示有调用') }}
        </el-checkbox>

        <el-select v-model="sortBy" size="small" class="sort-select" style="width: 140px">
          <el-option label="Token消耗最高" value="tokens_desc" />
          <el-option label="调用量最高" value="calls_desc" />
          <el-option label="响应延迟最高" value="latency_desc" />
          <el-option label="成功率最低" value="success_asc" />
        </el-select>

        <el-input
          v-model="searchKeyword"
          size="small"
          :placeholder="$t('搜索路由/模型...')"
          clearable
          class="search-input"
          style="width: 160px"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <el-button link type="primary" size="small" class="link-routes" @click="navigateToAllRoutes">
          {{ $t('路由中心') }} &rarr;
        </el-button>
      </div>
    </div>

    <!-- Route Breakdown Detail Table -->
    <div class="breakdown-section">
      <el-table
        :data="paginatedRoutes"
        size="small"
        style="width: 100%"
        empty-text=""
      >
        <template #empty>
          <el-empty :description="$t('暂无匹配的路由遥测数据')" :image-size="60" />
        </template>

        <!-- Rank Column -->
        <el-table-column label="#" width="54" align="center">
          <template #default="{ $index }">
            <span
              class="rank-badge"
              :class="{
                'rank-1': (currentPage - 1) * pageSize + $index === 0,
                'rank-2': (currentPage - 1) * pageSize + $index === 1,
                'rank-3': (currentPage - 1) * pageSize + $index === 2,
              }"
            >
              {{ (currentPage - 1) * pageSize + $index + 1 }}
            </span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('路由 / 模型名称')" min-width="180">
          <template #default="{ row }">
            <div class="route-info-cell">
              <div class="route-name-row">
                <span class="route-title">{{ row.title || row.route_id }}</span>
                <el-tag size="small" :type="row.route_type === 'llm' ? 'primary' : 'info'" effect="plain" class="route-type-tag">
                  {{ (row.route_type || 'api').toUpperCase() }}
                </el-tag>
              </div>
              <div class="route-sub-code">
                {{ row.model_name || row.route_id }}
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="$t('调用次数')" min-width="100" align="right">
          <template #default="{ row }">
            <span class="num-cell">{{ row.calls.toLocaleString() }}</span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('成功率')" min-width="110" align="center">
          <template #default="{ row }">
            <div class="rate-cell">
              <span :class="'color-' + getSuccessRateType(row.success_rate)">
                {{ row.success_rate.toFixed(1) }}%
              </span>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="$t('平均延迟')" min-width="120" align="right">
          <template #default="{ row }">
            <div class="latency-cell">
              <span class="duration-main">{{ row.avg_duration_ms }} ms</span>
              <span class="duration-p95">P95: {{ row.p95_duration_ms }}ms</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="$t('Token 消耗与占比')" min-width="170">
          <template #default="{ row }">
            <div v-if="row.total_tokens > 0" class="token-cell-wrapper">
              <div class="token-main-row">
                <el-tooltip
                  :content="`输入: ${row.prompt_tokens.toLocaleString()} | 输出: ${row.completion_tokens.toLocaleString()}`"
                  placement="top"
                >
                  <span class="token-cell font-mono">{{ formatNumber(row.total_tokens) }}</span>
                </el-tooltip>
                <span class="token-pct">{{ getTokenSharePercent(row.total_tokens) }}%</span>
              </div>
              <el-progress
                :percentage="getTokenSharePercent(row.total_tokens)"
                :stroke-width="4"
                :show-text="false"
                color="#8e44ad"
                class="token-progress"
              />
            </div>
            <span v-else class="color-gray font-mono">-</span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('操作')" width="80" align="center">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="navigateToRoute(row.route_id)">
              {{ $t('详情') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination Footer -->
      <div v-if="filteredAndSortedRoutes.length > pageSize" class="pagination-footer">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="filteredAndSortedRoutes.length"
          :page-sizes="[5, 10, 20, 50]"
          size="small"
          layout="total, sizes, prev, pager, next"
          background
        />
      </div>
    </div>
  </el-card>
</template>

<style scoped>
.telemetry-panel-card {
  margin-top: 16px;
  border-radius: 8px;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.panel-title-area {
  display: flex;
  align-items: center;
  gap: 8px;
}

.panel-icon {
  font-size: 18px;
}

.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary, #303133);
}

.ttl-badge {
  font-size: 11px;
}

.panel-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.refresh-btn {
  color: var(--el-text-color-secondary, #909399);
}

/* 4 KPI Cards */
.kpi-grid {
  margin-bottom: 16px;
}

.kpi-card {
  border-radius: 6px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  transition: all 0.2s ease;
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
}

.kpi-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.bg-blue-tint {
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.04) 0%, rgba(64, 158, 255, 0.01) 100%);
}

.bg-purple-tint {
  background: linear-gradient(135deg, rgba(142, 68, 173, 0.04) 0%, rgba(142, 68, 173, 0.01) 100%);
}

.bg-green-tint {
  background: linear-gradient(135deg, rgba(103, 194, 58, 0.04) 0%, rgba(103, 194, 58, 0.01) 100%);
}

.bg-orange-tint {
  background: linear-gradient(135deg, rgba(230, 162, 60, 0.04) 0%, rgba(230, 162, 60, 0.01) 100%);
}

.kpi-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.kpi-label {
  font-size: 13px;
  color: var(--el-text-color-secondary, #909399);
  font-weight: 500;
}

.kpi-icon {
  font-size: 18px;
}

.kpi-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--el-text-color-primary, #303133);
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.kpi-unit {
  font-size: 12px;
  font-weight: 400;
  color: var(--el-text-color-secondary, #909399);
}

.rate-tag {
  font-size: 10px;
  padding: 0 4px;
  height: 18px;
  line-height: 18px;
}

.kpi-sub {
  margin-top: 6px;
  font-size: 12px;
  color: var(--el-text-color-placeholder, #a8abb2);
  display: flex;
  align-items: center;
}

.rate-progress {
  width: 100%;
}

/* Multi-dimensional Toolbar */
.toolbar-section {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  background-color: var(--el-fill-color-light, #f8f9fa);
  border-radius: 6px;
  margin-bottom: 12px;
}

.tab-filters {
  display: flex;
}

.warn-dot {
  color: #f56c6c;
  font-weight: bold;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.active-checkbox {
  margin-right: 2px;
}

.link-routes {
  font-size: 12px;
  padding: 0 4px;
}

/* Route breakdown */
.breakdown-section {
  margin-top: 4px;
}

.rank-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  font-size: 11px;
  font-weight: bold;
  color: var(--el-text-color-secondary, #909399);
  background: var(--el-fill-color-darker, #e4e7ed);
}

.rank-1 {
  background: linear-gradient(135deg, #ffd700, #ffae00);
  color: #fff;
  box-shadow: 0 2px 4px rgba(255, 174, 0, 0.3);
}

.rank-2 {
  background: linear-gradient(135deg, #d4d4d4, #a6a6a6);
  color: #fff;
}

.rank-3 {
  background: linear-gradient(135deg, #e6a23c, #cb7919);
  color: #fff;
}

.route-info-cell {
  display: flex;
  flex-direction: column;
}

.route-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.route-title {
  font-weight: 600;
  font-size: 13px;
  color: var(--el-text-color-primary, #303133);
}

.route-type-tag {
  font-size: 10px;
  padding: 0 4px;
  height: 18px;
  line-height: 16px;
}

.route-sub-code {
  font-family: monospace;
  font-size: 11px;
  color: var(--el-text-color-secondary, #909399);
  margin-top: 2px;
}

.num-cell {
  font-weight: 600;
  font-family: monospace;
  font-size: 13px;
}

.rate-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  font-size: 12px;
}

.latency-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.duration-main {
  font-weight: 600;
  font-family: monospace;
  font-size: 12px;
  color: var(--el-text-color-primary, #303133);
}

.duration-p95 {
  font-size: 11px;
  color: var(--el-text-color-secondary, #909399);
}

.token-cell-wrapper {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.token-main-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.token-cell {
  font-weight: 600;
  color: #8e44ad;
  cursor: pointer;
  font-size: 12px;
}

.token-pct {
  font-size: 11px;
  color: var(--el-text-color-secondary, #909399);
  font-family: monospace;
}

.token-progress {
  width: 100%;
}

.pagination-footer {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}

.font-mono {
  font-family: monospace;
}

.color-blue {
  color: #409eff;
}

.color-green {
  color: #67c23a;
}

.color-purple {
  color: #8e44ad;
}

.color-orange {
  color: #e6a23c;
}

.color-red {
  color: #f56c6c;
}

.color-gray {
  color: #909399;
}
</style>
