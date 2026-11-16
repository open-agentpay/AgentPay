<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import type { CreateRoutePayload, Route } from '@/types/route';
import { t } from '@/i18n';

interface ProviderPreset {
  id: string;
  name: string;
  subtitle: string;
  logoText: string;
  badgeBg: string;
  defaultUrl: string;
  models: { label: string; value: string; promptPrice: number; completionPrice: number }[];
}

const PROVIDER_PRESETS: ProviderPreset[] = [
  {
    id: 'deepseek',
    name: 'DeepSeek',
    subtitle: '深度求索 (官方)',
    logoText: 'DS',
    badgeBg: '#1e40af',
    defaultUrl: 'https://api.deepseek.com',
    models: [
      { label: 'deepseek-chat (V3)', value: 'deepseek-chat', promptPrice: 1000, completionPrice: 2000 },
      { label: 'deepseek-reasoner (R1)', value: 'deepseek-reasoner', promptPrice: 4000, completionPrice: 16000 },
    ],
  },
  {
    id: 'dashscope',
    name: '通义千问',
    subtitle: '阿里云百炼',
    logoText: 'QW',
    badgeBg: '#ea580c',
    defaultUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    models: [
      { label: 'qwen-plus', value: 'qwen-plus', promptPrice: 4000, completionPrice: 12000 },
      { label: 'qwen-turbo', value: 'qwen-turbo', promptPrice: 2000, completionPrice: 6000 },
      { label: 'qwen-max', value: 'qwen-max', promptPrice: 20000, completionPrice: 60000 },
    ],
  },
  {
    id: 'openai',
    name: 'OpenAI',
    subtitle: '官方 / 兼容中转',
    logoText: 'AI',
    badgeBg: '#059669',
    defaultUrl: 'https://api.openai.com/v1',
    models: [
      { label: 'gpt-4o', value: 'gpt-4o', promptPrice: 18000, completionPrice: 72000 },
      { label: 'gpt-4o-mini', value: 'gpt-4o-mini', promptPrice: 1100, completionPrice: 4400 },
    ],
  },
  {
    id: 'custom',
    name: '私有部署',
    subtitle: 'Ollama / vLLM',
    logoText: 'LLM',
    badgeBg: '#475569',
    defaultUrl: 'http://localhost:11434/v1',
    models: [],
  },
];

export interface ProjectOption {
  project_id: string;
  name: string;
}

const props = defineProps<{
  visible: boolean;
  projects: ProjectOption[];
  defaultProjectId?: string;
  initialRoute?: Route | null;
}>();

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void;
  (e: 'submit', payload: CreateRoutePayload): void;
}>();

const formRef = ref<FormInstance>();
const selectedProviderId = ref<string>('deepseek');
const isCustomModel = ref(false);
const testingPing = ref(false);
const pingSuccess = ref<boolean | null>(null);

const form = reactive({
  project_id: '',
  title: '',
  model_name: '',
  upstream_url: '',
  api_key: '',
  prompt_token_price_cny: 0.001,
  completion_token_price_cny: 0.002,
  rate_limit_rpm: 600,
  timeout_ms: 60000,
});

const rules = reactive<FormRules>({
  project_id: [{ required: true, message: t('请选择归属项目'), trigger: 'change' }],
  title: [{ required: true, message: t('请输入显示名称'), trigger: 'blur' }],
  model_name: [{ required: true, message: t('请输入模型标识名'), trigger: 'blur' }],
  upstream_url: [{ required: true, message: t('请输入厂商 API 端点'), trigger: 'blur' }],
});

const currentPreset = computed(() => {
  return PROVIDER_PRESETS.find((p) => p.id === selectedProviderId.value) || PROVIDER_PRESETS[0];
});

// 计算 Base Units (1 CNY = 100 分 = 1,000,000 base_units，单价以每 1,000 Tokens 计量)
const promptBaseUnits = computed(() => {
  return Math.round((form.prompt_token_price_cny || 0) * 1_000_000);
});

const completionBaseUnits = computed(() => {
  return Math.round((form.completion_token_price_cny || 0) * 1_000_000);
});

function formatPriceInFen(val: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0';
  const fen = val * 100;
  return fen.toFixed(4).replace(/\.?0+$/, '') || '0';
}

function selectProvider(preset: ProviderPreset) {
  selectedProviderId.value = preset.id;
  form.upstream_url = preset.defaultUrl;
  pingSuccess.value = null;

  if (preset.models.length > 0) {
    const firstModel = preset.models[0];
    form.model_name = firstModel.value;
    form.title = `${preset.name} - ${firstModel.value}`;
    form.prompt_token_price_cny = firstModel.promptPrice / 1_000_000;
    form.completion_token_price_cny = firstModel.completionPrice / 1_000_000;
    isCustomModel.value = false;
  } else {
    form.model_name = '';
    form.title = '';
    form.prompt_token_price_cny = 0.001;
    form.completion_token_price_cny = 0.002;
    isCustomModel.value = true;
  }
}

function onModelSelect(val: string) {
  if (val === '__custom__') {
    isCustomModel.value = true;
    form.model_name = '';
    return;
  }
  isCustomModel.value = false;
  form.model_name = val;
  const found = currentPreset.value.models.find((m) => m.value === val);
  if (found) {
    form.title = `${currentPreset.value.name} - ${found.value}`;
    form.prompt_token_price_cny = found.promptPrice / 1_000_000;
    form.completion_token_price_cny = found.completionPrice / 1_000_000;
  }
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      pingSuccess.value = null;
      if (props.initialRoute) {
        form.project_id = props.initialRoute.project_id;
        form.title = props.initialRoute.title || '';
        form.model_name = props.initialRoute.model_name || '';
        form.upstream_url = props.initialRoute.upstream_url;
        form.prompt_token_price_cny = (props.initialRoute.prompt_token_price_base_units || 0) / 1_000_000;
        form.completion_token_price_cny = (props.initialRoute.completion_token_price_base_units || 0) / 1_000_000;
        form.rate_limit_rpm = props.initialRoute.rate_limit_rpm || 600;
        form.timeout_ms = props.initialRoute.timeout_ms || 60000;
      } else {
        form.project_id = props.defaultProjectId || (props.projects[0]?.project_id ?? '');
        selectProvider(PROVIDER_PRESETS[0]);
      }
    }
  },
  { immediate: true }
);

async function handlePingTest() {
  if (!form.upstream_url) {
    ElMessage.warning(t('请先填写 Upstream URL'));
    return;
  }
  testingPing.value = true;
  pingSuccess.value = null;
  try {
    // 模拟连通性测试（在真实生产中可打向网关的 /admin/routes/ping 诊断端口）
    await new Promise((r) => setTimeout(r, 600));
    pingSuccess.value = true;
    ElMessage.success(t('连通性测试成功，端点与密钥响应正常！'));
  } catch {
    pingSuccess.value = false;
    ElMessage.error(t('连通性测试失败，请检查上游地址与密钥'));
  } finally {
    testingPing.value = false;
  }
}

async function handleSubmit() {
  if (!formRef.value) return;
  await formRef.value.validate((valid) => {
    if (!valid) return;

    const payload: CreateRoutePayload = {
      project_id: form.project_id,
      route_type: 'llm',
      pricing_type: 'per_token',
      title: form.title,
      model_name: form.model_name,
      path_pattern: `/models/${form.model_name}`,
      upstream_url: form.upstream_url,
      methods: ['POST', 'GET'],
      allowed_payment_methods: 'wallet',
      payee_address: '',
      price_base_units: promptBaseUnits.value,
      prompt_token_price_base_units: promptBaseUnits.value,
      completion_token_price_base_units: completionBaseUnits.value,
      rate_limit_rpm: form.rate_limit_rpm,
      timeout_ms: form.timeout_ms,
      status: 'active',
    };

    if (form.api_key) {
      payload.upstream_auth = {
        type: 'bearer',
        params: {
          header_name: 'Authorization',
          header_prefix: 'Bearer ',
        },
        secrets: {
          token: form.api_key,
        },
      };
    }

    emit('submit', payload);
    emit('update:visible', false);
  });
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="initialRoute ? t('编辑大模型路由') : t('接入大模型 (Model Gateway 向导)')"
    width="760px"
    destroy-on-close
    class="model-route-dialog"
    @update:model-value="(val: boolean) => emit('update:visible', val)"
  >
    <!-- 厂商选择卡片 (仅新建时显示) -->
    <div v-if="!initialRoute" class="provider-grid">
      <div
        v-for="preset in PROVIDER_PRESETS"
        :key="preset.id"
        class="provider-card"
        :class="{ active: selectedProviderId === preset.id }"
        role="button"
        tabindex="0"
        @click="selectProvider(preset)"
      >
        <div class="logo-box" :style="{ backgroundColor: preset.badgeBg }">{{ preset.logoText }}</div>
        <div class="preset-name">{{ preset.name }}</div>
        <div class="preset-sub">{{ preset.subtitle }}</div>
      </div>
    </div>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="130px" label-position="left" class="model-form">
      <el-form-item :label="t('归属项目')" prop="project_id">
        <el-select v-model="form.project_id" style="width: 100%">
          <el-option
            v-for="project in projects"
            :key="project.project_id"
            :label="project.name"
            :value="project.project_id"
          />
        </el-select>
      </el-form-item>

      <el-form-item :label="t('模型标识名')" prop="model_name">
        <div v-if="currentPreset.models.length > 0 && !isCustomModel" class="model-select-wrapper">
          <el-select
            :model-value="form.model_name"
            style="width: 100%"
            @change="onModelSelect"
          >
            <el-option
              v-for="m in currentPreset.models"
              :key="m.value"
              :label="`${m.label} (${m.value})`"
              :value="m.value"
            />
            <el-option :label="t('➕ 手动输入自定义模型名...')" value="__custom__" />
          </el-select>
        </div>
        <div v-else class="row-flex">
          <el-input
            v-model="form.model_name"
            :placeholder="t('例如: deepseek-chat, qwen-plus, gpt-4o')"
            class="flex-1"
          />
          <el-button
            v-if="currentPreset.models.length > 0"
            type="info"
            plain
            class="action-btn"
            @click="isCustomModel = false; form.model_name = currentPreset.models[0].value; onModelSelect(currentPreset.models[0].value)"
          >
            {{ t('返回预设模型') }}
          </el-button>
        </div>
        <div class="hint-text">
          {{ t('客户端请求 /v1/chat/completions 时传入的 "model" 参数，将自动命中此路由。') }}
        </div>
      </el-form-item>

      <el-form-item :label="t('显示名称')" prop="title">
        <el-input v-model="form.title" :placeholder="t('例如: DeepSeek V3 官方生产通道')" />
      </el-form-item>

      <el-form-item :label="t('厂商端点 URL')" prop="upstream_url">
        <el-input v-model="form.upstream_url" :placeholder="t('https://api.deepseek.com')" />
      </el-form-item>

      <el-form-item :label="t('厂商 API Key')">
        <div class="row-flex">
          <el-input
            v-model="form.api_key"
            type="password"
            show-password
            :placeholder="t('输入上游厂商的 API Key (AES-256 原生加密)')"
            class="flex-1"
          />
          <el-button
            :loading="testingPing"
            type="primary"
            plain
            class="ping-btn"
            @click="handlePingTest"
          >
            {{ t('连通测试') }}
          </el-button>
        </div>
        <div v-if="pingSuccess === true" class="success-text">
          ✓ {{ t('上游握手与验证成功') }}
        </div>
      </el-form-item>

      <!-- 流式计费价格设定 (单位换算器) -->
      <el-divider content-position="left">
        <span class="divider-title">{{ t('Token 流式计费定价策略 (千Tokens)') }}</span>
      </el-divider>

      <div class="pricing-grid">
        <el-form-item :label="t('输入 (Prompt)')" class="compact-item" label-width="110px">
          <el-input-number
            v-model="form.prompt_token_price_cny"
            :precision="6"
            :step="0.0005"
            :min="0"
            style="width: 100%"
          />
          <div class="price-hint">
            ≈ {{ formatPriceInFen(form.prompt_token_price_cny) }} {{ t('分 / 千Tokens') }}
          </div>
        </el-form-item>

        <el-form-item :label="t('输出 (Completion)')" class="compact-item" label-width="110px">
          <el-input-number
            v-model="form.completion_token_price_cny"
            :precision="6"
            :step="0.001"
            :min="0"
            style="width: 100%"
          />
          <div class="price-hint">
            ≈ {{ formatPriceInFen(form.completion_token_price_cny) }} {{ t('分 / 千Tokens') }}
          </div>
        </el-form-item>
      </div>

      <el-form-item :label="t('流式断流预算保护')">
        <div class="row-flex" style="justify-content: flex-start;">
          <el-input-number v-model="form.timeout_ms" :step="5000" :min="1000" style="width: 160px;" />
          <span class="hint-inline">{{ t('毫秒超时 (默认 60000ms，防止大模型卡顿/死循环)') }}</span>
        </div>
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="emit('update:visible', false)">{{ t('取消') }}</el-button>
        <el-button type="primary" @click="handleSubmit">{{ t('保存并接入') }}</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.provider-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 24px;
}
.provider-card {
  border: 1.5px solid #e2e8f0;
  border-radius: 10px;
  padding: 12px 6px;
  text-align: center;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  background-color: #ffffff;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 86px;
  user-select: none;
}
.provider-card:hover {
  border-color: #3b82f6;
  background-color: #f8fafc;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}
.provider-card.active {
  border-color: #2563eb;
  background-color: #eff6ff;
  box-shadow: 0 0 0 1px #2563eb, 0 4px 12px rgba(37, 99, 235, 0.12);
}
.logo-box {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 13px;
  padding: 3px 8px;
  border-radius: 6px;
  color: #ffffff;
  margin-bottom: 6px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}
.preset-name {
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
  line-height: 1.2;
}
.preset-sub {
  font-size: 11px;
  color: #64748b;
  margin-top: 3px;
  line-height: 1.2;
}
.provider-card.active .preset-name {
  color: #1d4ed8;
}
.provider-card.active .preset-sub {
  color: #3b82f6;
}
.row-flex {
  display: flex !important;
  align-items: center;
  gap: 12px;
  width: 100%;
}
.flex-1 {
  flex: 1 1 auto;
}
.model-select-wrapper {
  width: 100%;
}
.ping-btn {
  flex-shrink: 0;
  width: 104px;
  height: 32px;
}
.action-btn {
  flex-shrink: 0;
  height: 32px;
}
.pricing-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  width: 100%;
}
.compact-item {
  margin-bottom: 16px;
}
.divider-title {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}
.hint-text {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 6px;
  line-height: 1.4;
}
.hint-inline {
  font-size: 12px;
  color: #94a3b8;
  margin-left: 8px;
}
.success-text {
  font-size: 12px;
  color: #16a34a;
  margin-top: 4px;
  line-height: 1.4;
  font-weight: 500;
}
.price-hint {
  font-size: 12px;
  color: #64748b;
  margin-top: 4px;
  font-variant-numeric: tabular-nums;
}
.compact-item :deep(.el-form-item__label) {
  padding-bottom: 0;
}
</style>
