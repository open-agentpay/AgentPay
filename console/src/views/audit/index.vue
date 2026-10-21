<script setup lang="ts">
import { ref } from 'vue';
import { useTenantStore } from '@/stores/tenant';

const tenantStore = useTenantStore();

const auditLogs = ref([
  {
    audit_id: 'aud_9b0c1d2e3f',
    action: 'POLICY_EVAL_REJECT',
    actor: 'system:policy_engine',
    target: 'agent_crawler_swarm_01',
    reason: 'Exceeded rolling window 1-hour cap (50 USDC limit reached)',
    created_at: '2026-10-18T19:45:12Z'
  },
  {
    audit_id: 'aud_1a2b3c4d5e',
    action: 'CONFIG_VERSION_PUBLISHED',
    actor: 'admin@agentpay.io',
    target: 'ver_20261018002',
    reason: 'Routine routing configuration update',
    created_at: '2026-10-18T18:00:00Z'
  }
]);
</script>

<template>
  <div class="audit-page">
    <el-card shadow="never">
      <template #header>
        <div class="header-box">
          <div>
            <span class="title">{{ $t('管理与策略审计日志') }}</span>
            <span class="subtitle">{{ $t('租户:') }} {{ tenantStore.currentTenant?.name }}</span>
          </div>
          <div class="header-right">
            <el-button size="small" type="primary" plain @click="$router.push('/payments')">
              {{ $t('前往消费与履约审计') }} &rarr;
            </el-button>
            <el-tag type="warning" class="ml-2">{{ $t('全链路不可篡改审计追踪') }}</el-tag>
          </div>
        </div>
      </template>

      <el-table :data="auditLogs" style="width: 100%">
        <el-table-column prop="audit_id" :label="$t('审计 ID')" width="160" />
        <el-table-column prop="action" :label="$t('事件类型')" width="220">
          <template #default="{ row }">
            <el-tag :type="row.action.includes('REJECT') ? 'danger' : 'info'" size="small">
              {{ row.action }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="actor" :label="$t('触发主体')" width="180" />
        <el-table-column prop="target" :label="$t('作用对象')" width="200" />
        <el-table-column prop="reason" :label="$t('审计详情与判定依据')" min-width="260" />
        <el-table-column prop="created_at" :label="$t('记录时间')" width="180" />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.header-box {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.title {
  font-size: 16px;
  font-weight: 600;
  margin-right: 12px;
}
.subtitle {
  font-size: 13px;
  color: #909399;
}
</style>
