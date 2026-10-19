<script setup lang="ts">
import { computed } from 'vue';
import type { ValidationResult, ValidationError } from '@/types/config';

const props = defineProps<{
  modelValue: boolean;
  result: ValidationResult | null;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
}>();

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (val: boolean) => emit('update:modelValue', val)
});

function getFixSuggestion(err: ValidationError): string {
  switch (err.code) {
    case 'INVALID_UPSTREAM_URL':
      return '请将上游地址修改为合法的 http:// 或 https:// 协议开头的完整 URL。';
    case 'DUPLICATE_PATH_PATTERN':
      return '检测到存在重复的暴露路径，请调整 path_pattern 确保全租户唯一。';
    case 'INVALID_PAYEE_ADDRESS':
      return '链上或双通道支付通道必须填写合法的 0x 开头 40 位 EVM 钱包收款地址。';
    case 'NEGATIVE_PRICE_UNITS':
      return '单次调用费用必须为非负整数（单位为 6 位精度基数）。';
    case 'INVALID_PAYMENT_METHOD_OPTION':
      return '支付通道只允许配置为 wallet (虚拟支付)、onchain (链上支付) 或 both (双通道)。';
    case 'MAX_AMOUNT_EXCEEDS_ROLLING_BUDGET':
      return '单次请求上限不得大于 24 小时滚动预算总限额。';
    case 'INVALID_CIRCUIT_BREAKER_RPM':
      return '熔断阈值 RPM 必须为大于或等于 0 的整数。';
    default:
      return '请检查该字段的输入格式，确保符合规范。';
  }
}

function formatTime(timestamp?: number): string {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    title="网关配置草稿 · 静态组合校验诊断报告"
    width="860px"
    destroy-on-close
  >
    <div v-if="result" class="diagnostic-container">
      <!-- Top Status Banner -->
      <el-alert
        v-if="result.valid"
        title="静态组合校验 100% 通过"
        type="success"
        description="所有路由 URI、EVM收款地址格式、多级预算阶梯、熔断阈值及支付通道配置均符合网关热加载发布规范！"
        show-icon
        :closable="false"
        style="margin-bottom: 20px"
      />
      <el-alert
        v-else
        :title="`发现 ${result.summary.error_count} 处配置规则冲突或格式错误`"
        type="error"
        description="系统已拦截非法草稿发布。请根据下方诊断表及修复建议逐项调整配置后重新校验。"
        show-icon
        :closable="false"
        style="margin-bottom: 20px"
      />

      <!-- Metrics Row -->
      <div class="metrics-bar">
        <div class="metric-item">
          <span class="label">校验时间</span>
          <span class="value">{{ formatTime(result.validated_at) }}</span>
        </div>
        <div class="metric-item">
          <span class="label">检查路由数</span>
          <span class="value">{{ result.summary.total_routes }}</span>
        </div>
        <div class="metric-item">
          <span class="label">检查策略数</span>
          <span class="value">{{ result.summary.total_policies }}</span>
        </div>
        <div class="metric-item">
          <span class="label">错误总数</span>
          <span class="value error-val">{{ result.summary.error_count }}</span>
        </div>
      </div>

      <!-- Errors Table -->
      <div v-if="!result.valid && result.errors.length > 0" class="error-table-box">
        <h4 class="section-title">错误诊断详情</h4>
        <el-table :data="result.errors" border stripe style="width: 100%">
          <el-table-column label="错误码 / 类别" width="220">
            <template #default="{ row }">
              <el-tag type="danger" size="small" effect="plain">{{ row.code }}</el-tag>
              <div class="target-sub">{{ row.target_type }}: {{ row.target_id }}</div>
            </template>
          </el-table-column>

          <el-table-column prop="field" label="异常字段" width="130">
            <template #default="{ row }">
              <code class="code-field">{{ row.field }}</code>
            </template>
          </el-table-column>

          <el-table-column prop="message" label="错误描述" min-width="200" />

          <el-table-column label="修复建议" min-width="250">
            <template #default="{ row }">
              <div class="suggestion-box">
                <el-icon color="#e6a23c" style="margin-right: 4px"><Warning /></el-icon>
                <span>{{ getFixSuggestion(row) }}</span>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- All passed state -->
      <div v-else-if="result.valid" class="passed-box">
        <el-result
          icon="success"
          title="校验通过，草稿就绪"
          sub-title="您可以在「快照版本」页面将其安全发布为不可变快照版本。"
        />
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="dialogVisible = false">关闭</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.diagnostic-container {
  padding: 4px 0;
}
.metrics-bar {
  display: flex;
  background-color: #f8fafc;
  border-radius: 6px;
  padding: 12px 20px;
  margin-bottom: 20px;
  border: 1px solid #e2e8f0;
  gap: 32px;
}
.metric-item {
  display: flex;
  flex-direction: column;
}
.metric-item .label {
  font-size: 12px;
  color: #64748b;
  margin-bottom: 4px;
}
.metric-item .value {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}
.metric-item .error-val {
  color: #ef4444;
}
.section-title {
  margin: 0 0 12px 0;
  font-size: 14px;
  color: #334155;
}
.target-sub {
  font-size: 11px;
  color: #94a3b8;
  margin-top: 4px;
}
.code-field {
  background-color: #f1f5f9;
  color: #0f172a;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 12px;
}
.suggestion-box {
  display: flex;
  align-items: flex-start;
  font-size: 13px;
  color: #475569;
  line-height: 1.4;
}
.passed-box {
  padding: 20px 0;
}
</style>
