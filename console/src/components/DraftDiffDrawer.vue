<script setup lang="ts">
import { computed, ref } from 'vue';
import type { ConfigDraftDiff } from '@/types/config';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const props = defineProps<{
  modelValue: boolean;
  diff: ConfigDraftDiff | null;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
}>();

const drawerVisible = computed({
  get: () => props.modelValue,
  set: (val: boolean) => emit('update:modelValue', val)
});

const activeTab = ref('routes');

function getActionLabel(action: string): string {
  switch (action) {
    case 'added':
      return t('新增');
    case 'modified':
      return t('修改');
    case 'deleted':
      return t('删除');
    default:
      return action;
  }
}

function getActionTagType(action: string): 'success' | 'warning' | 'danger' | 'info' {
  switch (action) {
    case 'added':
      return 'success';
    case 'modified':
      return 'warning';
    case 'deleted':
      return 'danger';
    default:
      return 'info';
  }
}

function formatPrice(units?: unknown): string {
  if (typeof units !== 'number') return '-';
  return (units / 1000000).toFixed(4);
}
</script>

<template>
  <el-drawer
    v-model="drawerVisible"
    :title="$t('网关配置草稿 vs 生效快照 差异比对')"
    size="720px"
    destroy-on-close
  >
    <div v-if="diff" class="diff-container">
      <!-- Overview Bar -->
      <div class="diff-overview">
        <div class="overview-item">
          <span class="label">{{ $t('基准生效版本') }}</span>
          <el-tag size="small" type="info">{{ diff.active_version_id || $t('初始空快照') }}</el-tag>
        </div>
        <div class="overview-item">
          <span class="label">{{ $t('草稿变更状态') }}</span>
          <el-tag v-if="diff.has_changes" type="warning" size="small">{{ $t('存在未发布变更') }}</el-tag>
          <el-tag v-else type="success" size="small">{{ $t('与已发布版本一致') }}</el-tag>
        </div>
        <div class="overview-item">
          <span class="label">{{ $t('路由变更数') }}</span>
          <span class="count">{{ diff.route_diffs?.length || 0 }}</span>
        </div>
        <div class="overview-item">
          <span class="label">{{ $t('策略变更数') }}</span>
          <span class="count">{{ diff.policy_diffs?.length || 0 }}</span>
        </div>
      </div>

      <!-- No changes -->
      <div v-if="!diff.has_changes" class="empty-diff">
        <el-empty :description="$t('当前草稿与已生效网关快照完全一致，暂无未发布的变更。')" />
      </div>

      <!-- Changes tabs -->
      <div v-else class="diff-tabs-wrap">
        <el-tabs v-model="activeTab">
          <el-tab-pane :label="`${$t('路由变更数')} (${diff.route_diffs?.length || 0})`" name="routes">
            <div v-if="!diff.route_diffs || diff.route_diffs.length === 0" class="sub-empty">
              {{ $t('暂无路由层级的变更') }}
            </div>
            <div v-else class="diff-list">
              <el-card
                v-for="item in diff.route_diffs"
                :key="item.route_id"
                shadow="hover"
                class="diff-card"
              >
                <div class="diff-card-header">
                  <div class="title-left">
                    <el-tag :type="getActionTagType(item.action)" size="small">
                      {{ getActionLabel(item.action) }}
                    </el-tag>
                    <code class="path-badge">{{ item.path_pattern }}</code>
                  </div>
                  <span class="route-id">{{ item.route_id }}</span>
                </div>

                <!-- Modified details comparison -->
                <div v-if="item.action === 'modified'" class="diff-details">
                  <div class="diff-row">
                    <span class="diff-key">{{ $t('上游地址:') }}</span>
                    <div class="diff-val">
                      <span class="old-val">{{ item.before?.upstream_url }}</span>
                      <el-icon class="arrow-icon"><Right /></el-icon>
                      <span class="new-val">{{ item.after?.upstream_url }}</span>
                    </div>
                  </div>

                  <div class="diff-row">
                    <span class="diff-key">{{ $t('支付方式:') }}</span>
                    <div class="diff-val">
                      <span class="old-val">{{ item.before?.allowed_payment_methods }}</span>
                      <el-icon class="arrow-icon"><Right /></el-icon>
                      <span class="new-val">{{ item.after?.allowed_payment_methods }}</span>
                    </div>
                  </div>

                  <div class="diff-row">
                    <span class="diff-key">{{ $t('单价金额:') }}</span>
                    <div class="diff-val">
                      <span class="old-val">{{ formatPrice(item.before?.price_base_units) }}</span>
                      <el-icon class="arrow-icon"><Right /></el-icon>
                      <span class="new-val">{{ formatPrice(item.after?.price_base_units) }}</span>
                    </div>
                  </div>

                  <div class="diff-row">
                    <span class="diff-key">{{ $t('收款地址:') }}</span>
                    <div class="diff-val">
                      <span class="old-val">{{ item.before?.payee_address || $t('(虚拟钱包)') }}</span>
                      <el-icon class="arrow-icon"><Right /></el-icon>
                      <span class="new-val">{{ item.after?.payee_address || $t('(虚拟钱包)') }}</span>
                    </div>
                  </div>
                </div>

                <!-- Added details -->
                <div v-else-if="item.action === 'added'" class="diff-details added-box">
                  <div class="field-item">
                    <span class="field-label">{{ $t('目标上游:') }}</span>
                    <span>{{ item.after?.upstream_url }}</span>
                  </div>
                  <div class="field-item">
                    <span class="field-label">{{ $t('支付方式:') }}</span>
                    <span>{{ item.after?.allowed_payment_methods }}</span>
                  </div>
                  <div class="field-item">
                    <span class="field-label">{{ $t('单价:') }}</span>
                    <span>{{ formatPrice(item.after?.price_base_units) }}</span>
                  </div>
                </div>

                <!-- Deleted details -->
                <div v-else-if="item.action === 'deleted'" class="diff-details deleted-box">
                  <div class="field-item">
                    <span class="field-label">{{ $t('已删除路由:') }}</span>
                    <span class="deleted-text">{{ item.before?.upstream_url }}</span>
                  </div>
                </div>
              </el-card>
            </div>
          </el-tab-pane>

          <el-tab-pane :label="`${$t('策略变更数')} (${diff.policy_diffs?.length || 0})`" name="policies">
            <div v-if="!diff.policy_diffs || diff.policy_diffs.length === 0" class="sub-empty">
              {{ $t('暂无策略层级的变更') }}
            </div>
            <div v-else class="diff-list">
              <el-card
                v-for="item in diff.policy_diffs"
                :key="item.policy_id"
                shadow="hover"
                class="diff-card"
              >
                <div class="diff-card-header">
                  <div class="title-left">
                    <el-tag :type="getActionTagType(item.action)" size="small">
                      {{ getActionLabel(item.action) }}
                    </el-tag>
                    <span class="policy-title">{{ $t('安全策略:') }} {{ item.policy_id }}</span>
                  </div>
                </div>

                <!-- Modified policy comparison -->
                <div v-if="item.action === 'modified'" class="diff-details">
                  <div class="diff-row">
                    <span class="diff-key">{{ $t('单次上限:') }}</span>
                    <div class="diff-val">
                      <span class="old-val">{{ formatPrice(item.before?.max_amount_per_request) }}</span>
                      <el-icon class="arrow-icon"><Right /></el-icon>
                      <span class="new-val">{{ formatPrice(item.after?.max_amount_per_request) }}</span>
                    </div>
                  </div>

                  <div class="diff-row">
                    <span class="diff-key">{{ $t('24h预算上限:') }}</span>
                    <div class="diff-val">
                      <span class="old-val">{{ formatPrice(item.before?.rolling_24h_budget_cap) }}</span>
                      <el-icon class="arrow-icon"><Right /></el-icon>
                      <span class="new-val">{{ formatPrice(item.after?.rolling_24h_budget_cap) }}</span>
                    </div>
                  </div>

                  <div class="diff-row">
                    <span class="diff-key">{{ $t('熔断阈值 RPM:') }}</span>
                    <div class="diff-val">
                      <span class="old-val">{{ item.before?.circuit_breaker_max_rpm }}</span>
                      <el-icon class="arrow-icon"><Right /></el-icon>
                      <span class="new-val">{{ item.after?.circuit_breaker_max_rpm }}</span>
                    </div>
                  </div>
                </div>
              </el-card>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>

    <template #footer>
      <div class="drawer-footer">
        <el-button @click="drawerVisible = false">{{ $t('关闭') }}</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<style scoped>
.diff-container {
  padding: 0 4px;
}
.diff-overview {
  display: flex;
  background-color: #f8fafc;
  border-radius: 6px;
  padding: 12px 16px;
  margin-bottom: 20px;
  border: 1px solid #e2e8f0;
  gap: 24px;
}
.overview-item {
  display: flex;
  flex-direction: column;
}
.overview-item .label {
  font-size: 12px;
  color: #64748b;
  margin-bottom: 4px;
}
.overview-item .count {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}
.empty-diff {
  padding: 40px 0;
}
.sub-empty {
  text-align: center;
  color: #94a3b8;
  padding: 30px 0;
  font-size: 14px;
}
.diff-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.diff-card {
  border-radius: 6px;
}
.diff-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.title-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.path-badge {
  background-color: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 13px;
  color: #0f172a;
}
.route-id, .policy-title {
  font-size: 12px;
  color: #94a3b8;
}
.diff-details {
  background-color: #fafbfc;
  padding: 10px 14px;
  border-radius: 4px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
}
.diff-row {
  display: flex;
  align-items: center;
}
.diff-key {
  width: 90px;
  color: #64748b;
}
.diff-val {
  display: flex;
  align-items: center;
  gap: 8px;
}
.old-val {
  color: #ef4444;
  text-decoration: line-through;
  background-color: #fee2e2;
  padding: 1px 4px;
  border-radius: 2px;
}
.arrow-icon {
  color: #94a3b8;
}
.new-val {
  color: #16a34a;
  background-color: #dcfce7;
  padding: 1px 4px;
  border-radius: 2px;
}
.added-box {
  background-color: #f0fdf4;
}
.deleted-box {
  background-color: #fef2f2;
}
.deleted-text {
  color: #ef4444;
  text-decoration: line-through;
}
.field-item {
  display: flex;
  gap: 8px;
}
.field-label {
  color: #64748b;
  width: 90px;
}
</style>
