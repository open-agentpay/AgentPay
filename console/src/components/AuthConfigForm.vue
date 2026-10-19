<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { routeApi } from '@/api/client';
import type { AuthTypeSchema, UpstreamAuthConfig, TestUpstreamResult } from '@/types/auth_config';

const props = defineProps<{
  modelValue?: UpstreamAuthConfig;
  upstreamUrl?: string;
  tenantId?: string;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: UpstreamAuthConfig): void;
}>();

const authTypes = ref<AuthTypeSchema[]>([]);
const loadingTypes = ref(false);

const selectedType = ref<string>('none');
const params = ref<Record<string, any>>({});
const secrets = ref<Record<string, string>>({});
const hasSecret = ref<boolean>(false);

const testing = ref(false);
const testResult = ref<TestUpstreamResult | null>(null);

const currentSchema = computed(() => {
  return authTypes.value.find((t) => t.type === selectedType.value);
});

// Load auth type schemas from gateway backend
async function loadSchemas() {
  loadingTypes.value = true;
  try {
    const list = await routeApi.getAuthTypes();
    authTypes.value = list;
  } catch (err: unknown) {
    console.error('Failed to load auth types:', err);
  } finally {
    loadingTypes.value = false;
  }
}

// Sync from modelValue
function syncFromProps() {
  if (props.modelValue) {
    selectedType.value = props.modelValue.type || 'none';
    params.value = { ...(props.modelValue.params || {}) };
    secrets.value = { ...(props.modelValue.secrets || {}) };
    hasSecret.value = !!props.modelValue.has_secret;
  } else {
    selectedType.value = 'none';
    params.value = {};
    secrets.value = {};
    hasSecret.value = false;
  }
}

watch(
  () => props.modelValue,
  () => {
    syncFromProps();
  },
  { deep: true, immediate: true }
);

// Emit updates to parent
function emitChange() {
  const cfg: UpstreamAuthConfig = {
    type: selectedType.value,
    params: { ...params.value },
    has_secret: hasSecret.value || Object.keys(secrets.value).some((k) => !!secrets.value[k])
  };

  // Only include non-empty secrets in write payload
  const activeSecrets: Record<string, string> = {};
  for (const k in secrets.value) {
    if (secrets.value[k]) {
      activeSecrets[k] = secrets.value[k];
    }
  }
  if (Object.keys(activeSecrets).length > 0) {
    cfg.secrets = activeSecrets;
  }

  emit('update:modelValue', cfg);
}

function handleTypeChange(val: string) {
  selectedType.value = val;
  testResult.value = null;

  // Initialize default params for the selected schema
  const schema = authTypes.value.find((t) => t.type === val);
  if (schema) {
    const defaultParams: Record<string, any> = {};
    schema.fields.forEach((f) => {
      if (!f.secret && f.default_value !== undefined) {
        defaultParams[f.key] = f.default_value;
      }
    });
    params.value = defaultParams;
  }
  emitChange();
}

function handleParamChange() {
  emitChange();
}

function handleSecretChange() {
  emitChange();
}

// Test connection using backend gateway proxy tester
async function handleTestConnection() {
  if (!props.upstreamUrl) {
    ElMessage.warning('请先输入上游地址 (URL) 后再测试连通性');
    return;
  }

  testing.value = true;
  testResult.value = null;

  try {
    const tenantId = props.tenantId || 'ten_default';
    const cfg: UpstreamAuthConfig = {
      type: selectedType.value,
      params: { ...params.value },
      secrets: { ...secrets.value },
      has_secret: hasSecret.value
    };

    const res = await routeApi.testUpstream(tenantId, {
      upstream_url: props.upstreamUrl.trim(),
      method: 'GET',
      upstream_auth: cfg
    });

    testResult.value = res;
    if (res.success) {
      ElMessage.success(`上游测试连通成功 (HTTP ${res.status_code})`);
    } else {
      ElMessage.error(res.error || `上游返回异常状态 HTTP ${res.status_code}`);
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : '上游测试异常';
    testResult.value = {
      success: false,
      status_code: 0,
      status_text: 'NETWORK_ERROR',
      latency_ms: 0,
      sent_headers: {},
      error: msg
    };
    ElMessage.error(msg);
  } finally {
    testing.value = false;
  }
}

onMounted(() => {
  loadSchemas();
});
</script>

<template>
  <div class="auth-config-container">
    <!-- Auth Strategy Selector -->
    <el-form-item label="上游鉴权方式">
      <el-select
        v-model="selectedType"
        placeholder="选择上游 API 认证方式"
        style="width: 100%"
        @change="handleTypeChange"
      >
        <el-option
          v-for="t in authTypes"
          :key="t.type"
          :label="t.label"
          :value="t.type"
        >
          <div class="type-option">
            <span class="type-title">{{ t.label }}</span>
            <span class="type-desc">{{ t.description }}</span>
          </div>
        </el-option>
      </el-select>
      <div v-if="currentSchema" class="type-hint">
        {{ currentSchema.description }}
      </div>
    </el-form-item>

    <!-- Schema-Driven Fields (When not 'none') -->
    <div v-if="selectedType !== 'none' && currentSchema && currentSchema.fields.length > 0" class="schema-fields-box">
      <div v-for="field in currentSchema.fields" :key="field.key" class="field-item">
        <!-- Sensitive / Secret field -->
        <el-form-item
          v-if="field.secret"
          :label="field.label"
          :required="field.required && !hasSecret"
        >
          <div class="secret-input-wrap">
            <el-input
              v-model="secrets[field.key]"
              type="password"
              show-password
              clearable
              :placeholder="hasSecret ? '●●●●●● (已安全加密存储，留空保持不变)' : (field.description || '请输入密钥')"
              @input="handleSecretChange"
            />
            <el-tag v-if="hasSecret && !secrets[field.key]" type="success" size="small" effect="plain" class="secret-tag">
              已配置
            </el-tag>
          </div>
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- Non-sensitive field: Select -->
        <el-form-item
          v-else-if="field.kind === 'select'"
          :label="field.label"
          :required="field.required"
        >
          <el-select
            v-model="params[field.key]"
            style="width: 100%"
            @change="handleParamChange"
          >
            <el-option
              v-for="opt in field.options || []"
              :key="opt"
              :label="opt"
              :value="opt"
            />
          </el-select>
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- Non-sensitive field: Textarea -->
        <el-form-item
          v-else-if="field.kind === 'textarea'"
          :label="field.label"
          :required="field.required"
        >
          <el-input
            v-model="params[field.key]"
            type="textarea"
            :rows="3"
            :placeholder="field.description"
            @input="handleParamChange"
          />
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- Default: Text Input -->
        <el-form-item
          v-else
          :label="field.label"
          :required="field.required"
        >
          <el-input
            v-model="params[field.key]"
            :placeholder="field.description"
            @input="handleParamChange"
          />
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>
      </div>
    </div>

    <!-- Test Upstream Connectivity & Auth Section -->
    <div class="test-connectivity-box">
      <div class="test-actions">
        <el-button
          type="primary"
          plain
          size="small"
          icon="Connection"
          :loading="testing"
          @click="handleTestConnection"
        >
          测试上游鉴权连通性
        </el-button>
        <span class="test-tip">通过网关向目标上游发起探测请求，验证凭证有效性与脱敏后的请求头</span>
      </div>

      <!-- Test Result Report -->
      <div v-if="testResult" class="test-result-card">
        <el-alert
          v-if="testResult.success"
          :title="`连通性测试成功 - 状态码: ${testResult.status_code} (${testResult.status_text}) | 耗时: ${testResult.latency_ms}ms`"
          type="success"
          show-icon
          :closable="false"
        />
        <el-alert
          v-else
          :title="`连通性测试未通过 - ${testResult.error || testResult.status_text} (状态码: ${testResult.status_code}) | 耗时: ${testResult.latency_ms}ms`"
          type="error"
          show-icon
          :closable="false"
        />

        <!-- Sent headers preview -->
        <div v-if="testResult.sent_headers && Object.keys(testResult.sent_headers).length > 0" class="headers-preview">
          <div class="headers-title">网关转发请求头快照 (敏感凭据已自动脱敏):</div>
          <div class="headers-code">
            <div v-for="(val, key) in testResult.sent_headers" :key="key" class="header-line">
              <span class="hdr-key">{{ key }}:</span>
              <span class="hdr-val">{{ val }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-config-container {
  background-color: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 14px 16px;
  margin-bottom: 16px;
}
.type-option {
  display: flex;
  flex-direction: column;
}
.type-title {
  font-weight: 500;
  font-size: 13px;
  color: #1e293b;
}
.type-desc {
  font-size: 11px;
  color: #94a3b8;
}
.type-hint {
  font-size: 12px;
  color: #64748b;
  margin-top: 4px;
}
.schema-fields-box {
  background-color: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 12px 14px 2px 14px;
  margin-top: 8px;
  margin-bottom: 14px;
}
.secret-input-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}
.secret-tag {
  flex-shrink: 0;
}
.field-desc {
  font-size: 12px;
  color: #94a3b8;
  line-height: 1.3;
  margin-top: 3px;
}
.test-connectivity-box {
  border-top: 1px dashed #e2e8f0;
  padding-top: 12px;
  margin-top: 4px;
}
.test-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.test-tip {
  font-size: 12px;
  color: #64748b;
}
.test-result-card {
  margin-top: 10px;
}
.headers-preview {
  margin-top: 8px;
  background-color: #0f172a;
  color: #e2e8f0;
  border-radius: 4px;
  padding: 8px 12px;
  font-family: monospace;
  font-size: 12px;
}
.headers-title {
  color: #94a3b8;
  font-size: 11px;
  margin-bottom: 4px;
}
.headers-code {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.header-line {
  display: flex;
  gap: 6px;
}
.hdr-key {
  color: #38bdf8;
}
.hdr-val {
  color: #cbd5e1;
}
</style>
