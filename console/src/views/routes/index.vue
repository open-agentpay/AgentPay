<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import { routeApi, configApi } from '@/api/client';
import type { Route, CreateRoutePayload, UpdateRoutePayload, PaymentMethod, RouteStatus } from '@/types/route';
import type { UpstreamAuthConfig } from '@/types/auth_config';
import type { ConfigDraftSummary, ValidationResult, ConfigDraftDiff } from '@/types/config';
import PermissionGate from '@/components/PermissionGate.vue';
import ValidationDiagnosticModal from '@/components/ValidationDiagnosticModal.vue';
import DraftDiffDrawer from '@/components/DraftDiffDrawer.vue';
import AuthConfigForm from '@/components/AuthConfigForm.vue';
import { t } from '@/i18n';

const authStore = useAuthStore();
const tenantStore = useTenantStore();

const loading = ref(false);
const routes = ref<Route[]>([]);
const selectedProjectFilter = ref<string>('');

// Draft validation & diff state
const draftSummary = ref<ConfigDraftSummary | null>(null);
const validating = ref(false);
const lastValidationResult = ref<ValidationResult | null>(null);
const validationModalVisible = ref(false);

const diffLoading = ref(false);
const draftDiff = ref<ConfigDraftDiff | null>(null);
const diffDrawerVisible = ref(false);

// Role check
const canManage = computed(() => authStore.hasRole(['owner', 'admin']));

// Filtered Routes
const filteredRoutes = computed(() => {
  if (!selectedProjectFilter.value) {
    return routes.value;
  }
  return routes.value.filter((r) => r.project_id === selectedProjectFilter.value);
});

// Load Routes from API
async function loadRoutes() {
  if (!tenantStore.currentTenantId) return;
  loading.value = true;
  try {
    const list = await routeApi.getRoutes(tenantStore.currentTenantId);
    routes.value = list;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('加载路由配置失败');
    ElMessage.error(msg);
  } finally {
    loading.value = false;
  }
}

// Load Draft Summary
async function loadDraftSummary() {
  if (!tenantStore.currentTenantId) return;
  try {
    const summary = await configApi.getDraftSummary(tenantStore.currentTenantId);
    draftSummary.value = summary;
    if (summary.last_result) {
      lastValidationResult.value = summary.last_result;
    }
  } catch {
    // Silent fallback
  }
}

// Trigger static draft validation
async function handleValidateDraft() {
  if (!tenantStore.currentTenantId) return;
  validating.value = true;
  try {
    const result = await configApi.validateDraft(tenantStore.currentTenantId);
    lastValidationResult.value = result;
    await loadDraftSummary();
    if (result.valid) {
      ElMessage.success(t('配置草稿静态组合校验 100% 通过，已就绪可发布！'));
    } else {
      ElMessage.error(`${t('检测到')} ${result.summary.error_count} ${t('处错误')}，${t('已拦截发布！')}`);
      validationModalVisible.value = true;
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('配置草稿校验失败');
    ElMessage.error(msg);
  } finally {
    validating.value = false;
  }
}

// Open Diff Drawer
async function handleOpenDiff() {
  if (!tenantStore.currentTenantId) return;
  diffLoading.value = true;
  try {
    const diff = await configApi.getDraftDiff(tenantStore.currentTenantId);
    draftDiff.value = diff;
    diffDrawerVisible.value = true;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('获取草稿差异比对失败');
    ElMessage.error(msg);
  } finally {
    diffLoading.value = false;
  }
}

const publishing = ref(false);

function handleOpenDiagnostic() {
  validationModalVisible.value = true;
}

// Publish draft directly from Routes management view
async function handlePublishDraft() {
  if (!tenantStore.currentTenantId) return;
  publishing.value = true;
  try {
    const result = await configApi.validateDraft(tenantStore.currentTenantId);
    lastValidationResult.value = result;
    await loadDraftSummary();
    if (!result.valid) {
      ElMessage.error(`${t('草稿组合校验未通过')} (${t('发现')} ${result.summary.error_count} ${t('处错误')})，${t('系统已阻止发布！')}`);
      validationModalVisible.value = true;
      return;
    }

    const { value: changeSummary } = await ElMessageBox.prompt(
      t('配置草稿已通过完整静态校验矩阵。请输入本次发布变更摘要：'),
      t('发布配置快照版本'),
      {
        confirmButtonText: t('确认发布并热加载'),
        cancelButtonText: t('取消'),
        inputPlaceholder: t('例如：更新 LLM 上游路由与计费配置'),
        inputValue: t('更新网关路由配置')
      }
    );

    const newVersion = await configApi.publishConfig(tenantStore.currentTenantId, {
      change_summary: changeSummary
    });

    ElMessage.success(`${t('版本')} v${newVersion.version_number} ${t('发布成功，不可变快照已热加载至网关！')}`);
    await loadDraftSummary();
    await loadRoutes();
  } catch (err: unknown) {
    if (err !== 'cancel') {
      const msg = err instanceof Error ? err.message : t('发布配置失败');
      ElMessage.error(msg);
    }
  } finally {
    publishing.value = false;
  }
}

// Watch tenant switch
watch(
  () => tenantStore.currentTenantId,
  () => {
    loadRoutes();
    loadDraftSummary();
  }
);

// Auto-sync project filter if currentProject changes
watch(
  () => tenantStore.currentProject?.project_id,
  (newPid) => {
    if (newPid && !selectedProjectFilter.value) {
      selectedProjectFilter.value = newPid;
    }
  },
  { immediate: true }
);

onMounted(() => {
  if (tenantStore.currentProject?.project_id) {
    selectedProjectFilter.value = tenantStore.currentProject.project_id;
  }
  loadRoutes();
  loadDraftSummary();
});

// Form validation rules
const evmAddressRegex = /^0x[a-fA-F0-9]{40}$/;
const pathPatternRegex = /^\/[a-zA-Z0-9_\-/{}]*$/;

const routeRules: FormRules = {
  project_id: [{ required: true, message: t('请选择所属项目'), trigger: 'change' }],
  path_pattern: [
    { required: true, message: t('请输入暴露路径'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (!value.startsWith('/')) {
          callback(new Error(t('路径必须以 / 开头，例如 /api/v1/market-data')));
        } else if (!pathPatternRegex.test(value)) {
          callback(new Error(t('路径只允许字母、数字、下划线、中划线和斜杠')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ],
  upstream_url: [
    { required: true, message: t('请输入目标上游URL'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (!value.startsWith('http://') && !value.startsWith('https://')) {
          callback(new Error(t('上游地址必须以 http:// 或 https:// 开头')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ],
  allowed_payment_methods: [{ required: true, message: t('请选择支持的支付方式'), trigger: 'change' }],
  price_usdc: [
    { required: true, message: t('请输入单次调用单价'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (value === undefined || value === null || value < 0) {
          callback(new Error(t('价格不能为负数')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ],
  payee_address: [
    {
      validator: (_rule, value, callback) => {
        const isCreatingWallet = createDialogVisible.value && createForm.allowed_payment_methods === 'wallet';
        const isEditingWallet = editDialogVisible.value && editForm.allowed_payment_methods === 'wallet';
        if (isCreatingWallet || isEditingWallet) {
          callback();
          return;
        }
        if (!value) {
          callback(new Error(t('请输入链上收款地址')));
        } else if (!evmAddressRegex.test(value)) {
          callback(new Error(t('收款地址必须为标准的42位以太坊/EVM地址 (0x...)')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ],
  timeout_ms: [{ required: true, message: t('请输入超时时间'), trigger: 'blur' }],
  rate_limit_rpm: [{ required: true, message: t('请输入频控限制'), trigger: 'blur' }]
};

// ----------------------------------------------------------------------------
// Create Route Dialog
// ----------------------------------------------------------------------------
const createDialogVisible = ref(false);
const createLoading = ref(false);
const createFormRef = ref<FormInstance>();
const createForm = reactive({
  project_id: '',
  path_pattern: '',
  upstream_url: '',
  price_usdc: 0.05,
  asset: 'USDC',
  payee_address: '',
  allowed_payment_methods: 'both' as PaymentMethod,
  rate_limit_rpm: 600,
  timeout_ms: 30000,
  upstream_auth: { type: 'none', params: {}, secrets: {} } as UpstreamAuthConfig
});

function openCreateDialog() {
  createForm.project_id = tenantStore.currentProject?.project_id || (tenantStore.projects[0]?.project_id ?? '');
  createForm.path_pattern = '';
  createForm.upstream_url = '';
  createForm.price_usdc = 0.05;
  createForm.asset = 'USDC';
  createForm.payee_address = '';
  createForm.allowed_payment_methods = 'both';
  createForm.rate_limit_rpm = 600;
  createForm.timeout_ms = 30000;
  createForm.upstream_auth = { type: 'none', params: {}, secrets: {} };
  createDialogVisible.value = true;
}

async function handleCreateRoute() {
  if (!createFormRef.value || !tenantStore.currentTenantId) return;
  await createFormRef.value.validate(async (valid) => {
    if (!valid) return;

    createLoading.value = true;
    try {
      const isWalletOnly = createForm.allowed_payment_methods === 'wallet';
      const priceBaseUnits = Math.round(createForm.price_usdc * 1000000);
      const payload: CreateRoutePayload = {
        project_id: createForm.project_id,
        path_pattern: createForm.path_pattern.trim(),
        upstream_url: createForm.upstream_url.trim(),
        price_base_units: priceBaseUnits,
        asset: isWalletOnly ? 'USD' : createForm.asset,
        payee_address: isWalletOnly ? '' : createForm.payee_address.trim().toLowerCase(),
        allowed_payment_methods: createForm.allowed_payment_methods,
        rate_limit_rpm: createForm.rate_limit_rpm,
        timeout_ms: createForm.timeout_ms,
        upstream_auth: createForm.upstream_auth,
        status: 'active'
      };

      await routeApi.createRoute(tenantStore.currentTenantId, payload);
      ElMessage.success(t('代理路由创建成功'));
      createDialogVisible.value = false;
      await loadRoutes();
      await loadDraftSummary();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('创建代理路由失败');
      ElMessage.error(msg);
    } finally {
      createLoading.value = false;
    }
  });
}

// ----------------------------------------------------------------------------
// Edit Route Dialog
// ----------------------------------------------------------------------------
const editDialogVisible = ref(false);
const editLoading = ref(false);
const editFormRef = ref<FormInstance>();
const editingRouteId = ref('');
const editForm = reactive({
  project_id: '',
  path_pattern: '',
  upstream_url: '',
  price_usdc: 0.05,
  asset: 'USDC',
  payee_address: '',
  allowed_payment_methods: 'both' as PaymentMethod,
  rate_limit_rpm: 600,
  timeout_ms: 30000,
  upstream_auth: { type: 'none', params: {}, secrets: {} } as UpstreamAuthConfig,
  status: 'active' as RouteStatus
});

function openEditDialog(route: Route) {
  editingRouteId.value = route.route_id;
  editForm.project_id = route.project_id;
  editForm.path_pattern = route.path_pattern;
  editForm.upstream_url = route.upstream_url;
  editForm.price_usdc = route.price_base_units / 1000000;
  editForm.asset = route.asset || 'USDC';
  editForm.payee_address = route.payee_address;
  editForm.allowed_payment_methods = route.allowed_payment_methods;
  editForm.rate_limit_rpm = route.rate_limit_rpm || 600;
  editForm.timeout_ms = route.timeout_ms || 30000;
  editForm.upstream_auth = route.upstream_auth
    ? JSON.parse(JSON.stringify(route.upstream_auth))
    : { type: 'none', params: {}, secrets: {} };
  editForm.status = route.status;
  editDialogVisible.value = true;
}

async function handleUpdateRoute() {
  if (!editFormRef.value || !tenantStore.currentTenantId || !editingRouteId.value) return;
  await editFormRef.value.validate(async (valid) => {
    if (!valid) return;

    editLoading.value = true;
    try {
      const isWalletOnly = editForm.allowed_payment_methods === 'wallet';
      const priceBaseUnits = Math.round(editForm.price_usdc * 1000000);
      const payload: UpdateRoutePayload = {
        project_id: editForm.project_id,
        path_pattern: editForm.path_pattern.trim(),
        upstream_url: editForm.upstream_url.trim(),
        price_base_units: priceBaseUnits,
        asset: isWalletOnly ? 'USD' : editForm.asset,
        payee_address: isWalletOnly ? '' : editForm.payee_address.trim().toLowerCase(),
        allowed_payment_methods: editForm.allowed_payment_methods,
        rate_limit_rpm: editForm.rate_limit_rpm,
        timeout_ms: editForm.timeout_ms,
        upstream_auth: editForm.upstream_auth,
        status: editForm.status
      };

      await routeApi.updateRoute(tenantStore.currentTenantId, editingRouteId.value, payload);
      ElMessage.success(t('路由配置更新成功'));
      editDialogVisible.value = false;
      await loadRoutes();
      await loadDraftSummary();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('更新路由配置失败');
      ElMessage.error(msg);
    } finally {
      editLoading.value = false;
    }
  });
}

// ----------------------------------------------------------------------------
// Status Toggle & Delete
// ----------------------------------------------------------------------------
async function toggleStatus(route: Route) {
  if (!tenantStore.currentTenantId) return;
  const targetStatus: RouteStatus = route.status === 'active' ? 'disabled' : 'active';
  const actionText = targetStatus === 'active' ? t('启用') : t('禁用');

  try {
    await ElMessageBox.confirm(`${t('确定要')}${actionText}${t('路由')} "${route.path_pattern}" ${t('吗？')}`, t('状态变更确认'), {
      type: targetStatus === 'active' ? 'info' : 'warning',
      confirmButtonText: t('确定'),
      cancelButtonText: t('取消')
    });

    await routeApi.updateRoute(tenantStore.currentTenantId, route.route_id, {
      status: targetStatus
    });
    ElMessage.success(`${t('路由已')}${actionText}`);
    await loadRoutes();
    await loadDraftSummary();
  } catch (err: unknown) {
    if (err !== 'cancel') {
      const msg = err instanceof Error ? err.message : t('操作失败');
      ElMessage.error(msg);
    }
  }
}

async function handleDeleteRoute(route: Route) {
  if (!tenantStore.currentTenantId) return;

  try {
    await ElMessageBox.confirm(
      `${t('确定要删除路由')} "${route.path_pattern}" ${t('吗？')}${t('删除后下游调用将直接返回 404 错误。')}`,
      t('删除确认'),
      {
        type: 'error',
        confirmButtonText: t('确认删除'),
        cancelButtonText: t('取消')
      }
    );

    await routeApi.deleteRoute(tenantStore.currentTenantId, route.route_id);
    ElMessage.success(t('路由已删除'));
    await loadRoutes();
    await loadDraftSummary();
  } catch (err: unknown) {
    if (err !== 'cancel') {
      const msg = err instanceof Error ? err.message : t('删除失败');
      ElMessage.error(msg);
    }
  }
}

function getProjectName(projectId: string): string {
  const p = tenantStore.projects.find((item) => item.project_id === projectId);
  return p ? p.name : projectId;
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text);
  ElMessage.success(t('已复制到剪贴板'));
}
</script>

<template>
  <div class="routes-page">
    <el-card shadow="never">
      <template #header>
        <div class="header-box">
          <div class="header-left">
            <span class="title">{{ $t('网关路由与上游配置') }}</span>
            <el-select
              v-model="selectedProjectFilter"
              :placeholder="$t('按项目过滤')"
              clearable
              size="default"
              style="width: 220px; margin-left: 12px"
            >
              <el-option :label="$t('全部项目')" value="" />
              <el-option
                v-for="p in tenantStore.projects"
                :key="p.project_id"
                :label="p.name"
                :value="p.project_id"
              />
            </el-select>
          </div>
          <div class="header-right">
            <PermissionGate :roles="['owner', 'admin']">
              <el-button type="primary" icon="Plus" @click="openCreateDialog">
                {{ $t('新建代理路由') }}
              </el-button>
              <template #fallback>
                <el-tag type="info">{{ $t('当前角色') }} ({{ authStore.currentRole }}) {{ $t('为只读视图') }}</el-tag>
              </template>
            </PermissionGate>
          </div>
        </div>
      </template>

      <!-- Draft Control & Status Bar -->
      <div class="draft-control-bar">
        <div class="draft-info">
          <div class="draft-badge-group">
            <span class="draft-title">{{ $t('网关配置工作草稿') }}</span>
            <el-tag
              v-if="draftSummary?.is_validated && lastValidationResult?.valid"
              type="success"
              effect="dark"
              size="small"
            >
              <el-icon style="vertical-align: middle; margin-right: 2px"><Select /></el-icon>
              {{ $t('组合校验通过') }}
            </el-tag>
            <el-tag
              v-else-if="draftSummary?.is_validated && lastValidationResult && !lastValidationResult.valid"
              type="danger"
              effect="dark"
              size="small"
            >
              <el-icon style="vertical-align: middle; margin-right: 2px"><CloseBold /></el-icon>
              {{ $t('校验未通过') }} ({{ lastValidationResult.summary.error_count }} {{ $t('错误') }})
            </el-tag>
            <el-tag v-else type="warning" effect="plain" size="small">
              <el-icon style="vertical-align: middle; margin-right: 2px"><Clock /></el-icon>
              {{ $t('待静态校验') }}
            </el-tag>
          </div>
          <div class="draft-meta">
            <span>{{ $t('待发布变更:') }} <strong>{{ draftSummary?.pending_changes ?? 0 }}</strong></span>
            <span class="meta-dot">·</span>
            <span>{{ $t('路由数:') }} <strong>{{ draftSummary?.routes_count ?? routes.length }}</strong></span>
            <span class="meta-dot">·</span>
            <span>{{ $t('策略数:') }} <strong>{{ draftSummary?.policies_count ?? 0 }}</strong></span>
            <span class="meta-dot">·</span>
            <span class="isolation-note">{{ $t('草稿隔离保护中，发布新版本前不影响线上网关') }}</span>
          </div>
        </div>

        <div class="draft-actions">
          <el-button
            type="primary"
            plain
            size="small"
            icon="CircleCheck"
            :loading="validating"
            @click="handleValidateDraft"
          >
            {{ $t('静态组合校验') }}
          </el-button>
          <el-button
            v-if="lastValidationResult"
            type="info"
            plain
            size="small"
            icon="Document"
            @click="handleOpenDiagnostic"
          >
            {{ $t('校验诊断报告') }}
          </el-button>
          <el-button
            type="default"
            size="small"
            icon="Operation"
            :loading="diffLoading"
            @click="handleOpenDiff"
          >
            {{ $t('草稿差异 (Diff)') }}
          </el-button>
          <el-button
            type="success"
            size="small"
            icon="Upload"
            :loading="publishing"
            :disabled="!canManage"
            @click="handlePublishDraft"
          >
            {{ $t('发布新版本') }}
          </el-button>
        </div>
      </div>

      <!-- Routes Table -->
      <el-table v-loading="loading" :data="filteredRoutes" stripe style="width: 100%">
        <el-table-column prop="path_pattern" :label="$t('暴露路径')" min-width="200">
          <template #default="{ row }">
            <code class="code-path">{{ row.path_pattern }}</code>
          </template>
        </el-table-column>

        <el-table-column :label="$t('所属项目')" width="140">
          <template #default="{ row }">
            <el-tag size="small" type="info">{{ getProjectName(row.project_id) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="upstream_url" :label="$t('上游地址')" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="upstream-cell">
              <span>{{ row.upstream_url }}</span>
              <el-button
                link
                icon="CopyDocument"
                size="small"
                @click="copyToClipboard(row.upstream_url)"
                :title="$t('复制上游地址')"
              />
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="$t('上游鉴权')" width="130">
          <template #default="{ row }">
            <el-tag v-if="!row.upstream_auth || row.upstream_auth.type === 'none'" type="info" size="small">
              {{ $t('无鉴权') }}
            </el-tag>
            <el-tag v-else-if="row.upstream_auth.type === 'api_key'" type="primary" size="small">
              API Key
            </el-tag>
            <el-tag v-else-if="row.upstream_auth.type === 'bearer'" type="success" size="small">
              Bearer Token
            </el-tag>
            <el-tag v-else-if="row.upstream_auth.type === 'basic'" type="warning" size="small">
              {{ $t('Basic 认证') }}
            </el-tag>
            <el-tag v-else-if="row.upstream_auth.type === 'oauth2_client_credentials'" type="danger" size="small">
              OAuth 2.0
            </el-tag>
            <el-tag v-else-if="row.upstream_auth.type === 'hmac_sign'" type="warning" size="small">
              {{ $t('HMAC 签名') }}
            </el-tag>
            <el-tag v-else size="small">
              {{ row.upstream_auth.type }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="$t('支付方式')" width="160">
          <template #default="{ row }">
            <el-tag v-if="row.allowed_payment_methods === 'both'" type="success" size="small">
              {{ $t('双通道 (钱包/链上)') }}
            </el-tag>
            <el-tag v-else-if="row.allowed_payment_methods === 'wallet'" type="primary" size="small">
              {{ $t('仅虚拟钱包支付') }}
            </el-tag>
            <el-tag v-else-if="row.allowed_payment_methods === 'onchain'" type="warning" size="small">
              {{ $t('仅链上直接支付') }}
            </el-tag>
            <el-tag v-else size="small">{{ row.allowed_payment_methods }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="$t('单次定价')" width="140">
          <template #default="{ row }">
            <span v-if="row.allowed_payment_methods === 'wallet'" class="price-text">
              $ {{ (row.price_base_units / 1000000).toFixed(4) }} ({{ $t('美元') }})
            </span>
            <span v-else class="price-text">
              {{ (row.price_base_units / 1000000).toFixed(4) }} {{ row.asset || 'USDC' }}
            </span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('收款账户/地址')" width="170" show-overflow-tooltip>
          <template #default="{ row }">
            <div v-if="row.allowed_payment_methods === 'wallet'" class="wallet-cell">
              <el-tag size="small" type="primary">{{ $t('组织主虚拟钱包') }}</el-tag>
            </div>
            <div v-else-if="row.payee_address" class="address-cell">
              <span>{{ row.payee_address.slice(0, 6) }}...{{ row.payee_address.slice(-4) }}</span>
              <el-button
                link
                icon="CopyDocument"
                size="small"
                @click="copyToClipboard(row.payee_address)"
                :title="$t('复制收款地址')"
              />
            </div>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('超时/频控')" width="130">
          <template #default="{ row }">
            <div class="limit-cell">
              <div>{{ (row.timeout_ms || 30000) / 1000 }}{{ $t('s 超时') }}</div>
              <div class="text-muted">{{ row.rate_limit_rpm || 600 }} RPM</div>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="$t('状态')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
              {{ row.status === 'active' ? $t('启用') : $t('已禁用') }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="$t('操作')" width="180" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              size="small"
              :disabled="!canManage"
              @click="openEditDialog(row)"
            >
              {{ $t('编辑') }}
            </el-button>
            <el-button
              link
              :type="row.status === 'active' ? 'warning' : 'success'"
              size="small"
              :disabled="!canManage"
              @click="toggleStatus(row)"
            >
              {{ row.status === 'active' ? $t('禁用') : $t('启用') }}
            </el-button>
            <el-button
              link
              type="danger"
              size="small"
              :disabled="!canManage"
              @click="handleDeleteRoute(row)"
            >
              {{ $t('删除') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Route Dialog -->
    <el-dialog
      v-model="createDialogVisible"
      :title="$t('新建代理路由')"
      width="640px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="routeRules"
        label-width="120px"
        status-icon
      >
        <el-form-item :label="$t('所属项目')" prop="project_id">
          <el-select v-model="createForm.project_id" :placeholder="$t('请选择项目')" style="width: 100%">
            <el-option
              v-for="p in tenantStore.projects"
              :key="p.project_id"
              :label="p.name"
              :value="p.project_id"
            />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('暴露路径')" prop="path_pattern">
          <el-input
            v-model="createForm.path_pattern"
            :placeholder="$t('例如: /api/v1/llm/chat')"
            clearable
          >
            <template #prepend>{{ $t('路径') }}</template>
          </el-input>
          <div class="form-hint">{{ $t('客户端访问网关时的暴露相对路径，租户内全局唯一') }}</div>
        </el-form-item>

        <el-form-item :label="$t('目标上游URL')" prop="upstream_url">
          <el-input
            v-model="createForm.upstream_url"
            :placeholder="$t('例如: https://api.openai.com/v1/chat/completions')"
            clearable
          />
          <div class="form-hint">{{ $t('网关收到校验通过的请求后转发的目标真实后端地址') }}</div>
        </el-form-item>

        <!-- 上游鉴权方式配置 -->
        <AuthConfigForm
          v-model="createForm.upstream_auth"
          :upstream-url="createForm.upstream_url"
          :tenant-id="tenantStore.currentTenantId"
        />

        <!-- 支付通道 (置于单价之前) -->
        <el-form-item :label="$t('支付通道')" prop="allowed_payment_methods">
          <el-radio-group v-model="createForm.allowed_payment_methods">
            <el-radio value="both">{{ $t('双通道 (虚拟钱包 + 链上支付)') }}</el-radio>
            <el-radio value="wallet">{{ $t('仅虚拟钱包支付') }}</el-radio>
            <el-radio value="onchain">{{ $t('仅链上支付') }}</el-radio>
          </el-radio-group>
        </el-form-item>

        <!-- 单次调用单价 (置于支付通道之后，根据支付通道自动适应单位) -->
        <el-form-item :label="$t('单次调用单价')" prop="price_usdc">
          <el-input-number
            v-model="createForm.price_usdc"
            :min="0"
            :step="0.01"
            :precision="4"
            style="width: 200px"
          />
          <span style="margin-left: 8px; font-weight: 500">
            {{ createForm.allowed_payment_methods === 'wallet' ? $t('美元 ($)') : 'USDC' }}
          </span>
          <div class="form-hint">
            {{ createForm.allowed_payment_methods === 'wallet' ? $t('虚拟支付单价，以美元为结算单位') : $t('计费精度为 6 位小数，系统自动转为整数基础单位 (Base Units)') }}
          </div>
        </el-form-item>

        <!-- 链上收款地址 (仅在支持链上支付时显示) -->
        <el-form-item
          v-if="createForm.allowed_payment_methods !== 'wallet'"
          :label="$t('链上收款地址')"
          prop="payee_address"
        >
          <el-input
            v-model="createForm.payee_address"
            placeholder="0x..."
            clearable
          />
          <div class="form-hint">{{ $t('用于链上清算直接打款的 EVM 0x 格式收款账户') }}</div>
        </el-form-item>

        <!-- 虚拟账户钱包展示 (在仅虚拟支付时展示) -->
        <el-form-item
          v-else
          :label="$t('虚拟账户钱包')"
        >
          <el-input
            :model-value="`wlt_${tenantStore.currentTenantId || 'master'}`"
            disabled
          >
            <template #prepend>{{ $t('组织主钱包') }}</template>
          </el-input>
          <div class="form-hint">{{ $t('虚拟支付自动记账并结算至当前组织的虚拟主钱包中，无需配置链上地址') }}</div>
        </el-form-item>

        <el-form-item :label="$t('上游超时时间')" prop="timeout_ms">
          <el-input-number
            v-model="createForm.timeout_ms"
            :min="1000"
            :max="300000"
            :step="5000"
            style="width: 200px"
          />
          <span style="margin-left: 8px">{{ $t('毫秒 (ms)') }}</span>
        </el-form-item>

        <el-form-item :label="$t('频控速率上限')" prop="rate_limit_rpm">
          <el-input-number
            v-model="createForm.rate_limit_rpm"
            :min="10"
            :max="100000"
            :step="100"
            style="width: 200px"
          />
          <span style="margin-left: 8px">{{ $t('RPM (次/分钟)') }}</span>
        </el-form-item>
      </el-form>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="createDialogVisible = false">{{ $t('取消') }}</el-button>
          <el-button type="primary" :loading="createLoading" @click="handleCreateRoute">
            {{ $t('确认新建') }}
          </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- Edit Route Dialog -->
    <el-dialog
      v-model="editDialogVisible"
      :title="$t('编辑代理路由配置')"
      width="640px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="editFormRef"
        :model="editForm"
        :rules="routeRules"
        label-width="120px"
        status-icon
      >
        <el-form-item :label="$t('所属项目')" prop="project_id">
          <el-select v-model="editForm.project_id" :placeholder="$t('请选择项目')" style="width: 100%">
            <el-option
              v-for="p in tenantStore.projects"
              :key="p.project_id"
              :label="p.name"
              :value="p.project_id"
            />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('暴露路径')" prop="path_pattern">
          <el-input v-model="editForm.path_pattern" clearable>
            <template #prepend>{{ $t('路径') }}</template>
          </el-input>
        </el-form-item>

        <el-form-item :label="$t('目标上游URL')" prop="upstream_url">
          <el-input v-model="editForm.upstream_url" clearable />
        </el-form-item>

        <!-- 上游鉴权方式配置 -->
        <AuthConfigForm
          v-model="editForm.upstream_auth"
          :upstream-url="editForm.upstream_url"
          :tenant-id="tenantStore.currentTenantId"
        />

        <!-- 支付通道 (置于单价之前) -->
        <el-form-item :label="$t('支付通道')" prop="allowed_payment_methods">
          <el-radio-group v-model="editForm.allowed_payment_methods">
            <el-radio value="both">{{ $t('双通道 (虚拟钱包 + 链上支付)') }}</el-radio>
            <el-radio value="wallet">{{ $t('仅虚拟钱包支付') }}</el-radio>
            <el-radio value="onchain">{{ $t('仅链上支付') }}</el-radio>
          </el-radio-group>
        </el-form-item>

        <!-- 单次调用单价 (置于支付通道之后，根据支付通道自动适应单位) -->
        <el-form-item :label="$t('单次调用单价')" prop="price_usdc">
          <el-input-number
            v-model="editForm.price_usdc"
            :min="0"
            :step="0.01"
            :precision="4"
            style="width: 200px"
          />
          <span style="margin-left: 8px; font-weight: 500">
            {{ editForm.allowed_payment_methods === 'wallet' ? $t('美元 ($)') : 'USDC' }}
          </span>
          <div class="form-hint">
            {{ editForm.allowed_payment_methods === 'wallet' ? $t('虚拟支付单价，以美元为结算单位') : $t('计费精度为 6 位小数，系统自动转为整数基础单位 (Base Units)') }}
          </div>
        </el-form-item>

        <!-- 链上收款地址 (仅在包含链上支付时显示) -->
        <el-form-item
          v-if="editForm.allowed_payment_methods !== 'wallet'"
          :label="$t('链上收款地址')"
          prop="payee_address"
        >
          <el-input v-model="editForm.payee_address" clearable />
          <div class="form-hint">{{ $t('用于链上清算直接打款的 EVM 0x 格式收款账户') }}</div>
        </el-form-item>

        <!-- 虚拟账户钱包展示 -->
        <el-form-item
          v-else
          :label="$t('虚拟账户钱包')"
        >
          <el-input
            :model-value="`wlt_${tenantStore.currentTenantId || 'master'}`"
            disabled
          >
            <template #prepend>{{ $t('组织主钱包') }}</template>
          </el-input>
          <div class="form-hint">{{ $t('虚拟支付自动记账并结算至当前组织的虚拟主钱包中，无需配置链上地址') }}</div>
        </el-form-item>

        <el-form-item :label="$t('上游超时时间')" prop="timeout_ms">
          <el-input-number
            v-model="editForm.timeout_ms"
            :min="1000"
            :max="300000"
            :step="5000"
            style="width: 200px"
          />
          <span style="margin-left: 8px">{{ $t('毫秒 (ms)') }}</span>
        </el-form-item>

        <el-form-item :label="$t('频控速率上限')" prop="rate_limit_rpm">
          <el-input-number
            v-model="editForm.rate_limit_rpm"
            :min="10"
            :max="100000"
            :step="100"
            style="width: 200px"
          />
          <span style="margin-left: 8px">{{ $t('RPM (次/分钟)') }}</span>
        </el-form-item>

        <el-form-item :label="$t('路由状态')">
          <el-radio-group v-model="editForm.status">
            <el-radio value="active">{{ $t('启用 (Active)') }}</el-radio>
            <el-radio value="disabled">{{ $t('禁用 (Disabled)') }}</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="editDialogVisible = false">{{ $t('取消') }}</el-button>
          <el-button type="primary" :loading="editLoading" @click="handleUpdateRoute">
            {{ $t('保存变更') }}
          </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- Validation Diagnostic Modal -->
    <ValidationDiagnosticModal
      v-model="validationModalVisible"
      :result="lastValidationResult"
    />

    <!-- Draft Diff Drawer -->
    <DraftDiffDrawer
      v-model="diffDrawerVisible"
      :diff="draftDiff"
    />
  </div>
</template>

<style scoped>
.routes-page {
  padding: 0;
}
.header-box {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-left {
  display: flex;
  align-items: center;
}
.title {
  font-size: 16px;
  font-weight: 600;
}
.draft-control-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: linear-gradient(135deg, #f0f9ff 0%, #f8fafc 100%);
  border: 1px solid #bae6fd;
  border-radius: 6px;
  padding: 10px 16px;
  margin-bottom: 16px;
  gap: 16px;
}
.draft-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.draft-badge-group {
  display: flex;
  align-items: center;
  gap: 8px;
}
.draft-title {
  font-size: 14px;
  font-weight: 600;
  color: #0369a1;
}
.draft-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #64748b;
}
.meta-dot {
  color: #cbd5e1;
}
.isolation-note {
  color: #0284c7;
}
.draft-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.code-path {
  font-family: monospace;
  font-size: 13px;
  background-color: #f4f4f5;
  padding: 2px 6px;
  border-radius: 4px;
  color: #409eff;
}
.upstream-cell,
.address-cell {
  display: flex;
  align-items: center;
  gap: 4px;
}
.price-text {
  font-weight: 600;
  color: #303133;
}
.limit-cell {
  font-size: 12px;
  line-height: 1.4;
}
.text-muted {
  color: #909399;
}
.form-hint {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}
</style>
