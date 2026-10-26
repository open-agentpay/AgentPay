<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import { routeApi, configApi } from '@/api/client';
import type { Route, CreateRoutePayload, UpdateRoutePayload, PaymentMethod, RouteStatus, RouteParameter } from '@/types/route';
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
    const msg = err instanceof Error ? t(err.message) : t('加载路由配置失败');
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
    const msg = err instanceof Error ? t(err.message) : t('配置草稿校验失败');
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
    const msg = err instanceof Error ? t(err.message) : t('获取草稿差异比对失败');
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
      const msg = err instanceof Error ? t(err.message) : t('发布配置失败');
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
const pathPatternRegex = /^\/[a-zA-Z0-9_\-/{}*]*$/;

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

const marketplaceCategories = [
  { label: 'AI & LLM (模型与推理)', value: 'AI & LLM' },
  { label: 'Crypto & On-Chain Data (链上与行情数据)', value: 'Crypto & On-Chain Data' },
  { label: 'Web Scraping & Search (网页抓取与搜索)', value: 'Web Scraping & Search' },
  { label: 'Financial & Market Intel (金融与研报)', value: 'Financial & Market Intel' },
  { label: 'Compute & Developer Tools (算力与开发工具)', value: 'Compute & Developer Tools' }
];

const marketplaceNetworks = [
  { label: 'Arbitrum Sepolia (eip155:421614)', value: 'eip155:421614' },
  { label: 'Arbitrum One (eip155:42161)', value: 'eip155:42161' },
  { label: 'Base Sepolia (eip155:84532)', value: 'eip155:84532' },
  { label: 'Base Mainnet (eip155:8453)', value: 'eip155:8453' },
  { label: 'Solana Mainnet (solana:mainnet)', value: 'solana:mainnet' },
  { label: 'Solana Devnet (solana:devnet)', value: 'solana:devnet' }
];

function addParameter(form: typeof createForm | typeof editForm, parentName?: string) {
  const defaultName = parentName ? `${parentName}.` : '';
  form.parameters.push({
    name: defaultName,
    type: 'string',
    required: true,
    description: '',
    example: ''
  });
}

function removeParameter(form: typeof createForm | typeof editForm, index: number) {
  form.parameters.splice(index, 1);
}

function parseJsonField(raw: string, fieldName: string): Record<string, unknown> | undefined {
  const trimmed = raw.trim();
  if (!trimmed) return undefined;
  try {
    const parsed = JSON.parse(trimmed);
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      throw new Error(`${fieldName} ${t('必须为标准 JSON 对象格式 (如 {"key": "val"})')}`);
    }
    return parsed as Record<string, unknown>;
  } catch (err: unknown) {
    const msg = err instanceof Error ? t(err.message) : t('JSON 格式无效');
    throw new Error(`${fieldName} ${t('解析失败:')} ${msg}`);
  }
}

// ----------------------------------------------------------------------------
// Create Route Dialog
// ----------------------------------------------------------------------------
const createDialogVisible = ref(false);
const createLoading = ref(false);
const createActiveStep = ref(0);
const createFormRef = ref<FormInstance>();
const createForm = reactive({
  project_id: '',
  slug: '',
  title: '',
  summary: '',
  provider_name: '',
  is_public: true,
  category: 'AI & LLM',
  tags: ['x402', 'USDC'] as string[],
  description_md: '',
  network: 'eip155:421614',
  network_label: '',
  path_pattern: '',
  methods: ['POST'] as string[],
  upstream_url: '',
  price_usdc: 0.05,
  onchain_price_usdc: 0.05,
  asset: 'USD',
  payee_address: '',
  allowed_payment_methods: 'both' as PaymentMethod,
  rate_limit_rpm: 600,
  timeout_ms: 30000,
  upstream_auth: { type: 'none', params: {}, secrets: {} } as UpstreamAuthConfig,
  parameters: [] as RouteParameter[],
  sample_params_raw: '',
  sample_response_raw: ''
});

function openCreateDialog() {
  createActiveStep.value = 0;
  createForm.project_id = tenantStore.currentProject?.project_id || (tenantStore.projects[0]?.project_id ?? '');
  createForm.slug = '';
  createForm.title = '';
  createForm.summary = '';
  createForm.provider_name = '';
  createForm.is_public = true;
  createForm.category = 'AI & LLM';
  createForm.tags = ['x402', 'USDC'];
  createForm.description_md = '';
  createForm.network = 'eip155:421614';
  createForm.network_label = '';
  createForm.path_pattern = '';
  createForm.methods = ['POST'];
  createForm.upstream_url = '';
  createForm.price_usdc = 0.05;
  createForm.onchain_price_usdc = 0.05;
  createForm.asset = tenantStore.currentCurrency;
  createForm.payee_address = '';
  createForm.allowed_payment_methods = 'both';
  createForm.rate_limit_rpm = 600;
  createForm.timeout_ms = 30000;
  createForm.upstream_auth = { type: 'none', params: {}, secrets: {} };
  createForm.parameters = [];
  createForm.sample_params_raw = '';
  createForm.sample_response_raw = '';
  createDialogVisible.value = true;
}

watch(createDialogVisible, (visible) => {
  if (!visible) {
    createActiveStep.value = 0;
  }
});

async function handleNextStep() {
  if (createActiveStep.value === 0) {
    if (!createFormRef.value) return;
    try {
      await createFormRef.value.validateField(['project_id', 'path_pattern', 'upstream_url']);
      createActiveStep.value = 1;
    } catch {
      return;
    }
    return;
  }

  if (createActiveStep.value === 1) {
    createActiveStep.value = 2;
    return;
  }

  if (createActiveStep.value === 2) {
    // Validate JSON formatting if entered
    try {
      parseJsonField(createForm.sample_params_raw, t('调用入参样例'));
      parseJsonField(createForm.sample_response_raw, t('响应出参样例'));
      createActiveStep.value = 3;
    } catch (e: unknown) {
      ElMessage.error(e instanceof Error ? e.message : t('JSON 格式错误'));
      return;
    }
    return;
  }
}

function handlePrevStep() {
  if (createActiveStep.value > 0) {
    createActiveStep.value--;
  }
}

async function handleCreateRoute() {
  if (!createFormRef.value || !tenantStore.currentTenantId) return;
  await createFormRef.value.validate(async (valid, invalidFields) => {
    if (!valid) {
      if (invalidFields) {
        if (invalidFields.project_id || invalidFields.path_pattern || invalidFields.upstream_url) {
          createActiveStep.value = 0;
        } else if (invalidFields.allowed_payment_methods || invalidFields.price_usdc || invalidFields.payee_address) {
          createActiveStep.value = 3;
        }
      }
      return;
    }

    let sampleParams: Record<string, unknown> | undefined;
    let sampleResponse: Record<string, unknown> | undefined;
    try {
      sampleParams = parseJsonField(createForm.sample_params_raw, t('调用入参样例'));
      sampleResponse = parseJsonField(createForm.sample_response_raw, t('响应出参样例'));
    } catch (e: unknown) {
      ElMessage.error(e instanceof Error ? e.message : t('JSON 格式错误'));
      return;
    }

    createLoading.value = true;
    try {
      const isWalletOnly = createForm.allowed_payment_methods === 'wallet';
      const isOnchainOnly = createForm.allowed_payment_methods === 'onchain';
      const priceBaseUnits = Math.round(createForm.price_usdc * 1000000);
      const onchainPriceBaseUnits =
        createForm.allowed_payment_methods === 'both'
          ? Math.round((createForm.onchain_price_usdc || createForm.price_usdc) * 1000000)
          : priceBaseUnits;

      const cleanParameters = createForm.parameters
        .map((p) => ({
          name: p.name.trim(),
          type: p.type.trim() || 'string',
          required: Boolean(p.required),
          description: p.description.trim(),
          example: p.example?.trim() || undefined
        }))
        .filter((p) => p.name !== '');

      const payload: CreateRoutePayload = {
        project_id: createForm.project_id,
        slug: createForm.slug.trim() || undefined,
        title: createForm.title.trim() || undefined,
        summary: createForm.summary.trim() || undefined,
        provider_name: createForm.provider_name.trim() || undefined,
        is_public: createForm.is_public,
        category: createForm.category,
        tags: createForm.tags,
        description_md: createForm.description_md.trim() || undefined,
        network: createForm.network,
        network_label: createForm.network_label.trim() || undefined,
        path_pattern: createForm.path_pattern.trim(),
        methods: createForm.methods.length > 0 ? createForm.methods : ['POST'],
        upstream_url: createForm.upstream_url.trim(),
        price_base_units: priceBaseUnits,
        asset: isOnchainOnly ? 'USDC' : tenantStore.currentCurrency,
        onchain_price_base_units: isWalletOnly ? 0 : onchainPriceBaseUnits,
        onchain_asset: 'USDC',
        payee_address: isWalletOnly ? '' : createForm.payee_address.trim().toLowerCase(),
        allowed_payment_methods: createForm.allowed_payment_methods,
        rate_limit_rpm: createForm.rate_limit_rpm,
        timeout_ms: createForm.timeout_ms,
        upstream_auth: createForm.upstream_auth,
        parameters: cleanParameters,
        sample_params: sampleParams,
        sample_response: sampleResponse,
        status: 'active'
      };

      await routeApi.createRoute(tenantStore.currentTenantId, payload);
      ElMessage.success(t('代理路由创建成功'));
      createDialogVisible.value = false;
      await loadRoutes();
      await loadDraftSummary();
    } catch (err: unknown) {
      const msg = err instanceof Error ? t(err.message) : t('创建代理路由失败');
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
const editActiveStep = ref(0);
const editFormRef = ref<FormInstance>();
const editingRouteId = ref('');
const editForm = reactive({
  project_id: '',
  slug: '',
  title: '',
  summary: '',
  provider_name: '',
  is_public: false,
  category: 'AI & LLM',
  tags: [] as string[],
  description_md: '',
  network: 'eip155:421614',
  network_label: '',
  path_pattern: '',
  methods: ['POST'] as string[],
  upstream_url: '',
  price_usdc: 0.05,
  onchain_price_usdc: 0.05,
  asset: 'USD',
  payee_address: '',
  allowed_payment_methods: 'both' as PaymentMethod,
  rate_limit_rpm: 600,
  timeout_ms: 30000,
  upstream_auth: { type: 'none', params: {}, secrets: {} } as UpstreamAuthConfig,
  parameters: [] as RouteParameter[],
  sample_params_raw: '',
  sample_response_raw: '',
  status: 'active' as RouteStatus
});

watch(editDialogVisible, (visible) => {
  if (!visible) {
    editActiveStep.value = 0;
  }
});

async function handleEditNextStep() {
  if (editActiveStep.value === 0) {
    if (!editFormRef.value) return;
    try {
      await editFormRef.value.validateField(['project_id', 'path_pattern', 'upstream_url']);
      editActiveStep.value = 1;
    } catch {
      return;
    }
    return;
  }

  if (editActiveStep.value === 1) {
    editActiveStep.value = 2;
    return;
  }

  if (editActiveStep.value === 2) {
    // Validate JSON formatting if entered
    try {
      parseJsonField(editForm.sample_params_raw, t('调用入参样例'));
      parseJsonField(editForm.sample_response_raw, t('响应出参样例'));
      editActiveStep.value = 3;
    } catch (e: unknown) {
      ElMessage.error(e instanceof Error ? e.message : t('JSON 格式错误'));
      return;
    }
    return;
  }
}

function handleEditPrevStep() {
  if (editActiveStep.value > 0) {
    editActiveStep.value--;
  }
}

function flattenObjectToRouteParams(
  obj: Record<string, unknown>,
  prefix = ''
): RouteParameter[] {
  const result: RouteParameter[] = [];
  for (const [key, val] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (val !== null && typeof val === 'object' && !Array.isArray(val)) {
      result.push({
        name: fullKey,
        type: 'object',
        required: true,
        description: `Nested object ${fullKey}`,
      });
      result.push(...flattenObjectToRouteParams(val as Record<string, unknown>, fullKey));
    } else {
      let valType: string = typeof val;
      if (Array.isArray(val)) valType = 'array';
      else if (val === null) valType = 'null';
      result.push({
        name: fullKey,
        type: valType,
        required: true,
        description: `Parameter ${fullKey}`,
        example: typeof val === 'string' ? val : JSON.stringify(val),
      });
    }
  }
  return result;
}

function openEditDialog(route: Route) {
  editActiveStep.value = 0;
  editingRouteId.value = route.route_id;
  editForm.project_id = route.project_id;
  editForm.slug = route.slug || '';
  editForm.title = route.title || '';
  editForm.summary = route.summary || '';
  editForm.provider_name = route.provider_name || '';
  editForm.is_public = Boolean(route.is_public);
  editForm.category = route.category || 'AI & LLM';
  editForm.tags = Array.isArray(route.tags) ? [...route.tags] : [];
  editForm.description_md = route.description_md || '';
  editForm.network = route.network || 'eip155:421614';
  editForm.network_label = route.network_label || '';
  editForm.path_pattern = route.path_pattern;
  editForm.methods = Array.isArray(route.methods) && route.methods.length > 0 ? [...route.methods] : ['POST'];
  editForm.upstream_url = route.upstream_url;
  editForm.price_usdc = route.price_base_units / 1000000;
  editForm.onchain_price_usdc = (route.onchain_price_base_units || route.price_base_units) / 1000000;
  editForm.asset = route.asset || tenantStore.currentCurrency;
  editForm.payee_address = route.payee_address;
  editForm.allowed_payment_methods = route.allowed_payment_methods;
  editForm.rate_limit_rpm = route.rate_limit_rpm || 600;
  editForm.timeout_ms = route.timeout_ms || 30000;
  editForm.upstream_auth = route.upstream_auth
    ? JSON.parse(JSON.stringify(route.upstream_auth))
    : { type: 'none', params: {}, secrets: {} };

  const rawParams = route.parameters as unknown;
  if (Array.isArray(rawParams) && rawParams.length > 0) {
    editForm.parameters = JSON.parse(JSON.stringify(rawParams));
  } else if (rawParams && typeof rawParams === 'object' && !Array.isArray(rawParams) && Object.keys(rawParams).length > 0) {
    editForm.parameters = flattenObjectToRouteParams(rawParams as Record<string, unknown>);
  } else if (route.sample_params && typeof route.sample_params === 'object' && Object.keys(route.sample_params).length > 0) {
    editForm.parameters = flattenObjectToRouteParams(route.sample_params);
  } else {
    editForm.parameters = [];
  }

  const sampleObj =
    route.sample_params && Object.keys(route.sample_params).length > 0
      ? route.sample_params
      : (rawParams && typeof rawParams === 'object' && !Array.isArray(rawParams) && Object.keys(rawParams).length > 0)
      ? (rawParams as Record<string, unknown>)
      : null;

  editForm.sample_params_raw = sampleObj ? JSON.stringify(sampleObj, null, 2) : '';
  editForm.sample_response_raw = route.sample_response && Object.keys(route.sample_response).length > 0 ? JSON.stringify(route.sample_response, null, 2) : '';
  editForm.status = route.status;
  editDialogVisible.value = true;
}

async function handleUpdateRoute() {
  if (!editFormRef.value || !tenantStore.currentTenantId || !editingRouteId.value) return;
  await editFormRef.value.validate(async (valid, invalidFields) => {
    if (!valid) {
      if (invalidFields) {
        if (invalidFields.project_id || invalidFields.path_pattern || invalidFields.upstream_url) {
          editActiveStep.value = 0;
        } else if (invalidFields.allowed_payment_methods || invalidFields.price_usdc || invalidFields.payee_address) {
          editActiveStep.value = 3;
        }
      }
      return;
    }

    let sampleParams: Record<string, unknown> | undefined;
    let sampleResponse: Record<string, unknown> | undefined;
    try {
      sampleParams = parseJsonField(editForm.sample_params_raw, t('调用入参样例'));
      sampleResponse = parseJsonField(editForm.sample_response_raw, t('响应出参样例'));
    } catch (e: unknown) {
      ElMessage.error(e instanceof Error ? e.message : t('JSON 格式错误'));
      return;
    }

    editLoading.value = true;
    try {
      const isWalletOnly = editForm.allowed_payment_methods === 'wallet';
      const isOnchainOnly = editForm.allowed_payment_methods === 'onchain';
      const priceBaseUnits = Math.round(editForm.price_usdc * 1000000);
      const onchainPriceBaseUnits =
        editForm.allowed_payment_methods === 'both'
          ? Math.round((editForm.onchain_price_usdc || editForm.price_usdc) * 1000000)
          : priceBaseUnits;

      const cleanParameters = editForm.parameters
        .map((p) => ({
          name: p.name.trim(),
          type: p.type.trim() || 'string',
          required: Boolean(p.required),
          description: p.description.trim(),
          example: p.example?.trim() || undefined
        }))
        .filter((p) => p.name !== '');

      const payload: UpdateRoutePayload = {
        project_id: editForm.project_id,
        slug: editForm.slug.trim(),
        title: editForm.title.trim(),
        summary: editForm.summary.trim(),
        provider_name: editForm.provider_name.trim(),
        is_public: editForm.is_public,
        category: editForm.category,
        tags: editForm.tags,
        description_md: editForm.description_md.trim(),
        network: editForm.network,
        network_label: editForm.network_label.trim(),
        path_pattern: editForm.path_pattern.trim(),
        methods: editForm.methods.length > 0 ? editForm.methods : ['POST'],
        upstream_url: editForm.upstream_url.trim(),
        price_base_units: priceBaseUnits,
        asset: isOnchainOnly ? 'USDC' : tenantStore.currentCurrency,
        onchain_price_base_units: isWalletOnly ? 0 : onchainPriceBaseUnits,
        onchain_asset: 'USDC',
        payee_address: isWalletOnly ? '' : editForm.payee_address.trim().toLowerCase(),
        allowed_payment_methods: editForm.allowed_payment_methods,
        rate_limit_rpm: editForm.rate_limit_rpm,
        timeout_ms: editForm.timeout_ms,
        upstream_auth: editForm.upstream_auth,
        parameters: cleanParameters,
        sample_params: sampleParams,
        sample_response: sampleResponse,
        status: editForm.status
      };

      await routeApi.updateRoute(tenantStore.currentTenantId, editingRouteId.value, payload);
      ElMessage.success(t('路由配置更新成功'));
      editDialogVisible.value = false;
      await loadRoutes();
      await loadDraftSummary();
    } catch (err: unknown) {
      const msg = err instanceof Error ? t(err.message) : t('更新路由配置失败');
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
      const msg = err instanceof Error ? t(err.message) : t('操作失败');
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
      const msg = err instanceof Error ? t(err.message) : t('删除失败');
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
        <el-table-column prop="path_pattern" :label="$t('暴露路径')" min-width="220">
          <template #default="{ row }">
            <div>
              <div style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap">
                <el-tag
                  v-for="m in (row.methods && row.methods.length > 0 ? row.methods : ['POST'])"
                  :key="m"
                  size="small"
                  effect="dark"
                  :type="m === 'GET' ? 'success' : m === 'POST' ? 'primary' : m === 'DELETE' ? 'danger' : 'warning'"
                  style="font-weight: 700; font-size: 10px; padding: 0 4px; height: 18px; line-height: 18px"
                >
                  {{ m }}
                </el-tag>
                <code class="code-path">{{ row.path_pattern }}</code>
              </div>
              <div v-if="row.title" class="text-muted" style="font-size: 12px; margin-top: 2px">
                {{ row.title }}
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="$t('功能商店 (Marketplace)')" min-width="190">
          <template #default="{ row }">
            <div v-if="row.is_public" style="display: flex; flex-direction: column; gap: 4px">
              <div>
                <el-tag size="small" type="success" effect="light">{{ $t('已上架公开商店') }}</el-tag>
                <el-tag v-if="row.category" size="small" type="info" style="margin-left: 4px">
                  {{ row.category }}
                </el-tag>
              </div>
              <code v-if="row.slug" style="font-size: 11px; color: #0284c7">/hub/{{ row.slug }}</code>
            </div>
            <el-tag v-else size="small" type="info" effect="plain">{{ $t('仅私有内部调用') }}</el-tag>
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

        <el-table-column :label="$t('单次定价')" width="180">
          <template #default="{ row }">
            <span v-if="row.allowed_payment_methods === 'wallet'" class="price-text">
              {{ tenantStore.currencySymbol }} {{ (row.price_base_units / 1000000).toFixed(4) }} {{ tenantStore.currentCurrency }}
            </span>
            <span v-else-if="row.allowed_payment_methods === 'onchain'" class="price-text">
              {{ (row.price_base_units / 1000000).toFixed(4) }} USDC
            </span>
            <div v-else class="price-text">
              <div>{{ tenantStore.currencySymbol }} {{ (row.price_base_units / 1000000).toFixed(4) }} {{ tenantStore.currentCurrency }}</div>
              <div class="text-muted" style="font-size: 11px">
                {{ $t('链上:') }} {{ ((row.onchain_price_base_units || row.price_base_units) / 1000000).toFixed(4) }} USDC
              </div>
            </div>
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

    <!-- Create Route Dialog (Multi-Step Wizard) -->
    <el-dialog
      v-model="createDialogVisible"
      :title="$t('新建代理路由')"
      width="1000px"
      :close-on-click-modal="false"
      class="route-wizard-dialog"
    >
      <!-- 步骤导航条 -->
      <el-steps
        :active="createActiveStep"
        finish-status="success"
        align-center
        class="create-route-steps"
      >
        <el-step :title="$t('基础信息与鉴权')" :description="$t('路径与上游网关')" />
        <el-step :title="$t('商店展示与发现')" :description="$t('公开上架与发现')" />
        <el-step :title="$t('请求入参规范')" :description="$t('参数结构与样例')" />
        <el-step :title="$t('计费策略')" :description="$t('支付方式与定价')" />
      </el-steps>

      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="routeRules"
        label-width="190px"
        status-icon
      >
        <!-- 步骤 1: 基础信息（上游鉴权） -->
        <div v-show="createActiveStep === 0" class="step-panel">
          <div class="step-guide-box">
            <div class="step-guide-title">{{ $t('基础信息与上游服务接入') }}</div>
            <div class="step-guide-desc">{{ $t('配置网关对外暴露的访问路径、目标后端接口以及转发鉴权凭据') }}</div>
          </div>

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

          <el-form-item :label="$t('请求方式')">
            <el-checkbox-group v-model="createForm.methods">
              <el-checkbox value="POST">POST</el-checkbox>
              <el-checkbox value="GET">GET</el-checkbox>
              <el-checkbox value="PUT">PUT</el-checkbox>
              <el-checkbox value="DELETE">DELETE</el-checkbox>
            </el-checkbox-group>
            <div class="form-hint">{{ $t('允许客户端调用的 HTTP 方法，通常 AI 与大模型服务使用 POST') }}</div>
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
          <el-divider content-position="left">{{ $t('上游服务凭据与鉴权') }}</el-divider>
          <AuthConfigForm
            v-model="createForm.upstream_auth"
            :upstream-url="createForm.upstream_url"
            :tenant-id="tenantStore.currentTenantId"
          />

          <!-- 超时与限流防护 -->
          <el-divider content-position="left">{{ $t('网关超时与限流防护') }}</el-divider>
          <div style="display: flex; gap: 24px;">
            <el-form-item :label="$t('上游超时时间')" prop="timeout_ms" style="flex: 1;">
              <el-input-number
                v-model="createForm.timeout_ms"
                :min="1000"
                :max="300000"
                :step="5000"
                style="width: 100%"
              />
              <div class="form-hint">{{ $t('毫秒 (ms)，默认 30000ms') }}</div>
            </el-form-item>

            <el-form-item :label="$t('频控速率上限')" prop="rate_limit_rpm" style="flex: 1;">
              <el-input-number
                v-model="createForm.rate_limit_rpm"
                :min="10"
                :max="100000"
                :step="100"
                style="width: 100%"
              />
              <div class="form-hint">{{ $t('RPM (次/分钟)，默认 600 RPM') }}</div>
            </el-form-item>
          </div>
        </div>

        <!-- 步骤 2: 商店展示与发现 (原 GEO 展示配置) -->
        <div v-show="createActiveStep === 1" class="step-panel">
          <div class="step-guide-box">
            <div class="step-guide-title">{{ $t('功能商店与智能体发现配置') }}</div>
            <div class="step-guide-desc">{{ $t('配置该 API 在 OpenAgentPay 公开功能商店 (openagentpay.dev/marketplace) 与 /llms.txt 中的展示信息，便于人类开发者与 AI Agent 发现检索') }}</div>
          </div>

          <el-form-item :label="$t('上架公开商店')">
            <el-switch
              v-model="createForm.is_public"
              :active-text="$t('公开展示 (上架 openagentpay.dev/marketplace 与 /llms.txt)')"
              :inactive-text="$t('仅私有路由 (不对外公开展示)')"
            />
          </el-form-item>

          <template v-if="createForm.is_public">
            <el-form-item :label="$t('商品名称')">
              <el-input
                v-model="createForm.title"
                :placeholder="$t('例如: Solana Smart Money Tracker X402 API')"
                clearable
              />
            </el-form-item>

            <el-form-item :label="$t('服务商名称')">
              <el-input
                v-model="createForm.provider_name"
                :placeholder="$t('例如: Acme Autonomous AI Corp (留空默认使用组织名称)')"
                clearable
              />
              <div class="form-hint">{{ $t('在 Hub 详情页顶部及 SEO 结构化数据中展示的服务商机构背书') }}</div>
            </el-form-item>

            <el-form-item :label="$t('语义化别名')">
              <el-input
                v-model="createForm.slug"
                :placeholder="$t('例如: solana-smart-money-tracker-x402-api (留空将自动根据路径生成)')"
                clearable
              >
                <template #prepend>/hub/</template>
              </el-input>
              <div class="form-hint">{{ $t('用于生成易读的 Hub 详情页 URL 与 SDK route 标识') }}</div>
            </el-form-item>

            <el-form-item :label="$t('精炼摘要')">
              <el-input
                v-model="createForm.summary"
                type="textarea"
                :rows="2"
                :placeholder="$t('用于功能商店卡片及 /llms.txt 检索摘要 (建议 100~200 字)')"
                clearable
              />
              <div class="form-hint">{{ $t('AI 搜索引擎与智能体发现 API 核心功能的第一入口') }}</div>
            </el-form-item>

            <div style="display: flex; gap: 24px;">
              <el-form-item :label="$t('商店分类')" style="flex: 1;">
                <el-select v-model="createForm.category" style="width: 100%">
                  <el-option
                    v-for="cat in marketplaceCategories"
                    :key="cat.value"
                    :label="$t(cat.label)"
                    :value="cat.value"
                  />
                </el-select>
              </el-form-item>

              <el-form-item :label="$t('结算链网络')" style="flex: 1;">
                <el-select v-model="createForm.network" style="width: 100%">
                  <el-option
                    v-for="net in marketplaceNetworks"
                    :key="net.value"
                    :label="$t(net.label)"
                    :value="net.value"
                  />
                </el-select>
              </el-form-item>
            </div>

            <el-form-item :label="$t('网络易读标签')">
              <el-input
                v-model="createForm.network_label"
                :placeholder="$t('例如: Arbitrum / Base USDC (留空自动按网络推导)')"
                clearable
              />
              <div class="form-hint">{{ $t('在 Hub 页面展示的人性化网络名称，如 Arbitrum / Base USDC') }}</div>
            </el-form-item>

            <el-form-item :label="$t('检索标签')">
              <el-select
                v-model="createForm.tags"
                multiple
                filterable
                allow-create
                default-first-option
                :placeholder="$t('输入标签后回车，例如 Solana、x402、LLM')"
                style="width: 100%"
              >
                <el-option label="x402" value="x402" />
                <el-option label="USDC" value="USDC" />
                <el-option label="Solana" value="Solana" />
                <el-option label="Base" value="Base" />
                <el-option label="Arbitrum" value="Arbitrum" />
                <el-option label="LLM" value="LLM" />
                <el-option label="Web Scraping" value="Web Scraping" />
              </el-select>
            </el-form-item>

            <el-form-item :label="$t('详细功能描述')">
              <el-input
                v-model="createForm.description_md"
                type="textarea"
                :rows="4"
                :placeholder="$t('支持 Markdown：填写 API 功能说明、使用场景与约束条件')"
              />
            </el-form-item>
          </template>

          <template v-else>
            <el-alert
              :title="$t('当前为私有代理路由')"
              type="info"
              :description="$t('该路由仅在网关内部代理转发，不会展示在公开功能商店 (openagentpay.dev/marketplace) 与 /llms.txt 中。如需对外供 AI Agent 发现与付费采购，请开启上方【公开展示】开关。')"
              show-icon
              :closable="false"
              style="margin-top: 16px;"
            />
          </template>
        </div>

        <!-- 步骤 3: 请求入参规范 (支持嵌套参数) -->
        <div v-show="createActiveStep === 2" class="step-panel">
          <div class="step-guide-box">
            <div class="step-guide-title">{{ $t('请求入参规范与代码样例') }}</div>
            <div class="step-guide-desc">{{ $t('结构化定义 API 的输入参数字段与调用样例，系统将自动生成 AI Agent 一键集成的 SDK 代码与契约说明') }}</div>
          </div>

          <!-- 结构化请求入参定义 (Parameters) -->
          <div class="param-spec-box" style="width: 100%; margin-bottom: 20px;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
              <span style="font-size: 13px; font-weight: 600; color: #334155;">{{ $t('参数字段列表 (Input Parameters)') }}</span>
              <div style="display: flex; align-items: center; gap: 10px;">
                <span style="font-size: 12px; color: #94a3b8;">{{ $t('支持点号嵌套路径，如 filter.keyword 或 user.address.city') }}</span>
                <el-button type="primary" link icon="Plus" size="small" @click="addParameter(createForm)">
                  {{ $t('添加参数字段') }}
                </el-button>
              </div>
            </div>
            <el-table :data="createForm.parameters" border size="small" style="width: 100%">
              <el-table-column :label="$t('参数名')" min-width="160">
                <template #default="{ row }">
                  <div style="display: flex; align-items: center; gap: 4px;">
                    <span
                      v-if="row.name && row.name.includes('.')"
                      style="font-family: monospace; font-size: 11px; color: #4f46e5; background: #eef2ff; padding: 1px 5px; border-radius: 3px; white-space: nowrap;"
                    >
                      {{ '↳ '.repeat(Math.min(3, row.name.split('.').length - 1)) }}{{ $t('嵌套') }}
                    </span>
                    <el-input v-model="row.name" :placeholder="$t('例如: prompt 或 filter.keyword')" size="small" />
                  </div>
                </template>
              </el-table-column>
              <el-table-column :label="$t('类型')" width="110">
                <template #default="{ row }">
                  <el-select v-model="row.type" size="small">
                    <el-option label="string" value="string" />
                    <el-option label="integer" value="integer" />
                    <el-option label="number" value="number" />
                    <el-option label="boolean" value="boolean" />
                    <el-option label="object" value="object" />
                    <el-option label="array" value="array" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column :label="$t('必填')" width="70" align="center">
                <template #default="{ row }">
                  <el-switch v-model="row.required" size="small" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('描述')" min-width="160">
                <template #default="{ row }">
                  <el-input v-model="row.description" :placeholder="$t('用途说明')" size="small" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('示例值')" min-width="110">
                <template #default="{ row }">
                  <el-input v-model="row.example" :placeholder="$t('例如: 2048')" size="small" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('操作')" width="110" align="center">
                <template #default="{ row, $index }">
                  <el-button
                    link
                    type="primary"
                    size="small"
                    icon="CirclePlus"
                    :title="$t('添加子字段')"
                    @click="addParameter(createForm, row.name || 'nested')"
                  >
                    {{ $t('子字段') }}
                  </el-button>
                  <el-button link type="danger" size="small" icon="Delete" :title="$t('删除')" @click="removeParameter(createForm, $index)" />
                </template>
              </el-table-column>
            </el-table>
          </div>

          <!-- 调用入参样例与响应出参样例 -->
          <el-form-item :label="$t('调用入参样例')">
            <el-input
              v-model="createForm.sample_params_raw"
              type="textarea"
              :rows="3"
              :placeholder="`{\n  &quot;model&quot;: &quot;deepseek-r1&quot;,\n  &quot;prompt&quot;: &quot;Analyze invariants...&quot;\n}`"
            />
            <div class="form-hint">{{ $t('JSON 格式：用于在 Hub 详情页自动生成一键可运行的 Python SDK 示例代码') }}</div>
          </el-form-item>

          <el-form-item :label="$t('响应出参样例')">
            <el-input
              v-model="createForm.sample_response_raw"
              type="textarea"
              :rows="3"
              :placeholder="`{\n  &quot;id&quot;: &quot;chatcmpl_123&quot;,\n  &quot;content&quot;: &quot;...&quot;\n}`"
            />
            <div class="form-hint">{{ $t('JSON 格式：用于在 Hub 详情页展示标准 API 返回格式及 TEE 硬件证明样例') }}</div>
          </el-form-item>
        </div>

        <!-- 步骤 4: 计费策略 -->
        <div v-show="createActiveStep === 3" class="step-panel">
          <div class="step-guide-box">
            <div class="step-guide-title">{{ $t('X402 计费模式与清算收款') }}</div>
            <div class="step-guide-desc">{{ $t('设置客户端调用的支付通道、单次调用价格及收款钱包账户，网关将根据此配置自动下发 HTTP 402 Challenge') }}</div>
          </div>

          <!-- 支付通道 (置于单价之前) -->
          <el-form-item :label="$t('支付通道')" prop="allowed_payment_methods">
            <el-radio-group v-model="createForm.allowed_payment_methods">
              <el-radio value="both">{{ $t('双通道 (虚拟钱包 + 链上支付)') }}</el-radio>
              <el-radio value="wallet">{{ $t('仅虚拟钱包支付') }}</el-radio>
              <el-radio value="onchain">{{ $t('仅链上支付') }}</el-radio>
            </el-radio-group>
          </el-form-item>

          <!-- 单次调用单价 -->
          <el-form-item :label="createForm.allowed_payment_methods === 'both' ? $t('钱包结算单价') : $t('单次调用单价')" prop="price_usdc">
            <el-input-number
              v-model="createForm.price_usdc"
              :min="0"
              :step="0.01"
              :precision="4"
              style="width: 200px"
            />
            <span style="margin-left: 8px; font-weight: 500">
              {{ createForm.allowed_payment_methods === 'onchain' ? 'USDC' : `${tenantStore.currentCurrency} (${tenantStore.currencySymbol})` }}
            </span>
            <div class="form-hint">
              {{ createForm.allowed_payment_methods === 'onchain' ? $t('链上直付单价，计费精度为 6 位小数') : `${$t('内部虚拟钱包结算单价，以租户本位币')} ${tenantStore.currentCurrency} ${$t('计价 (6 位精度)')}` }}
            </div>
          </el-form-item>

          <!-- 双通道模式下的独立链上结算单价 (USDC) -->
          <el-form-item
            v-if="createForm.allowed_payment_methods === 'both'"
            :label="$t('链上结算单价')"
          >
            <el-input-number
              v-model="createForm.onchain_price_usdc"
              :min="0"
              :step="0.01"
              :precision="4"
              style="width: 200px"
            />
            <span style="margin-left: 8px; font-weight: 500">USDC</span>
            <div class="form-hint">
              {{ $t('双通道模式下链上 x402 EIP-3009 签名支付的独立 USDC 定价，网关不做实时汇率换算') }}
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
              :placeholder="$t('例如: 0x...')"
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
        </div>
      </el-form>

      <template #footer>
        <div class="step-dialog-footer">
          <el-button @click="createDialogVisible = false">{{ $t('取消') }}</el-button>
          <div class="step-nav-buttons">
            <el-button
              v-if="createActiveStep > 0"
              icon="ArrowLeft"
              @click="handlePrevStep"
            >
              {{ $t('上一步') }}
            </el-button>
            <el-button
              v-if="createActiveStep < 3"
              type="primary"
              @click="handleNextStep"
            >
              {{ $t('下一步') }}
              <el-icon class="el-icon--right"><ArrowRight /></el-icon>
            </el-button>
            <el-button
              v-else
              type="primary"
              :loading="createLoading"
              icon="Check"
              @click="handleCreateRoute"
            >
              {{ $t('确认新建代理路由') }}
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>

    <!-- Edit Route Dialog (4-Step Wizard) -->
    <el-dialog
      v-model="editDialogVisible"
      :title="$t('编辑代理路由配置')"
      width="1000px"
      :close-on-click-modal="false"
      class="route-wizard-dialog"
    >
      <!-- 步骤导航条 -->
      <el-steps
        :active="editActiveStep"
        finish-status="success"
        align-center
        class="create-route-steps"
      >
        <el-step :title="$t('基础信息与鉴权')" :description="$t('路径与上游网关')" />
        <el-step :title="$t('商店展示与发现')" :description="$t('公开上架与发现')" />
        <el-step :title="$t('请求入参规范')" :description="$t('参数结构与样例')" />
        <el-step :title="$t('计费策略')" :description="$t('支付方式与定价')" />
      </el-steps>

      <el-form
        ref="editFormRef"
        :model="editForm"
        :rules="routeRules"
        label-width="190px"
        status-icon
      >
        <!-- 步骤 1: 基础信息（上游鉴权） -->
        <div v-show="editActiveStep === 0" class="step-panel">
          <div class="step-guide-box">
            <div class="step-guide-title">{{ $t('基础信息与上游服务接入') }}</div>
            <div class="step-guide-desc">{{ $t('配置网关对外暴露的访问路径、目标后端接口以及转发鉴权凭据') }}</div>
          </div>

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
            <el-input
              v-model="editForm.path_pattern"
              :placeholder="$t('例如: /api/v1/llm/chat')"
              clearable
            >
              <template #prepend>{{ $t('路径') }}</template>
            </el-input>
            <div class="form-hint">{{ $t('客户端访问网关时的暴露相对路径，租户内全局唯一') }}</div>
          </el-form-item>

          <el-form-item :label="$t('请求方式')">
            <el-checkbox-group v-model="editForm.methods">
              <el-checkbox value="POST">POST</el-checkbox>
              <el-checkbox value="GET">GET</el-checkbox>
              <el-checkbox value="PUT">PUT</el-checkbox>
              <el-checkbox value="DELETE">DELETE</el-checkbox>
            </el-checkbox-group>
            <div class="form-hint">{{ $t('允许客户端调用的 HTTP 方法，通常 AI 与大模型服务使用 POST') }}</div>
          </el-form-item>

          <el-form-item :label="$t('目标上游URL')" prop="upstream_url">
            <el-input
              v-model="editForm.upstream_url"
              :placeholder="$t('例如: https://api.openai.com/v1/chat/completions')"
              clearable
            />
            <div class="form-hint">{{ $t('网关收到校验通过的请求后转发的目标真实后端地址') }}</div>
          </el-form-item>

          <!-- 上游鉴权方式配置 -->
          <el-divider content-position="left">{{ $t('上游服务凭据与鉴权') }}</el-divider>
          <AuthConfigForm
            v-model="editForm.upstream_auth"
            :upstream-url="editForm.upstream_url"
            :tenant-id="tenantStore.currentTenantId"
          />

          <!-- 超时与限流防护 -->
          <el-divider content-position="left">{{ $t('网关超时与限流防护') }}</el-divider>
          <div style="display: flex; gap: 24px;">
            <el-form-item :label="$t('上游超时时间')" prop="timeout_ms" style="flex: 1;">
              <el-input-number
                v-model="editForm.timeout_ms"
                :min="1000"
                :max="300000"
                :step="5000"
                style="width: 100%"
              />
              <div class="form-hint">{{ $t('毫秒 (ms)，默认 30000ms') }}</div>
            </el-form-item>

            <el-form-item :label="$t('频控速率上限')" prop="rate_limit_rpm" style="flex: 1;">
              <el-input-number
                v-model="editForm.rate_limit_rpm"
                :min="10"
                :max="100000"
                :step="100"
                style="width: 100%"
              />
              <div class="form-hint">{{ $t('RPM (次/分钟)，默认 600 RPM') }}</div>
            </el-form-item>
          </div>

          <el-form-item :label="$t('路由状态')">
            <el-radio-group v-model="editForm.status">
              <el-radio value="active">{{ $t('启用 (Active)') }}</el-radio>
              <el-radio value="disabled">{{ $t('禁用 (Disabled)') }}</el-radio>
            </el-radio-group>
          </el-form-item>
        </div>

        <!-- 步骤 2: 商店展示与发现 -->
        <div v-show="editActiveStep === 1" class="step-panel">
          <div class="step-guide-box">
            <div class="step-guide-title">{{ $t('功能商店与智能体发现配置') }}</div>
            <div class="step-guide-desc">{{ $t('配置该 API 在 OpenAgentPay 公开功能商店 (openagentpay.dev/marketplace) 与 /llms.txt 中的展示信息，便于人类开发者与 AI Agent 发现检索') }}</div>
          </div>

          <el-form-item :label="$t('上架公开商店')">
            <el-switch
              v-model="editForm.is_public"
              :active-text="$t('公开展示 (上架 openagentpay.dev/marketplace 与 /llms.txt)')"
              :inactive-text="$t('仅私有路由 (不对外公开展示)')"
            />
          </el-form-item>

          <template v-if="editForm.is_public">
            <el-form-item :label="$t('商品名称')">
              <el-input
                v-model="editForm.title"
                :placeholder="$t('例如: Solana Smart Money Tracker X402 API')"
                clearable
              />
            </el-form-item>

            <el-form-item :label="$t('服务商名称')">
              <el-input
                v-model="editForm.provider_name"
                :placeholder="$t('例如: Acme Autonomous AI Corp (留空默认使用组织名称)')"
                clearable
              />
              <div class="form-hint">{{ $t('在 Hub 详情页顶部及 SEO 结构化数据中展示的服务商机构背书') }}</div>
            </el-form-item>

            <el-form-item :label="$t('语义化别名')">
              <el-input
                v-model="editForm.slug"
                :placeholder="$t('例如: solana-smart-money-tracker-x402-api (留空将自动根据路径生成)')"
                clearable
              >
                <template #prepend>/hub/</template>
              </el-input>
              <div class="form-hint">{{ $t('用于生成易读的 Hub 详情页 URL 与 SDK route 标识') }}</div>
            </el-form-item>

            <el-form-item :label="$t('精炼摘要')">
              <el-input
                v-model="editForm.summary"
                type="textarea"
                :rows="2"
                :placeholder="$t('用于功能商店卡片及 /llms.txt 检索摘要 (建议 100~200 字)')"
                clearable
              />
              <div class="form-hint">{{ $t('AI 搜索引擎与智能体发现 API 核心功能的第一入口') }}</div>
            </el-form-item>

            <div style="display: flex; gap: 24px;">
              <el-form-item :label="$t('商店分类')" style="flex: 1;">
                <el-select v-model="editForm.category" style="width: 100%">
                  <el-option
                    v-for="cat in marketplaceCategories"
                    :key="cat.value"
                    :label="$t(cat.label)"
                    :value="cat.value"
                  />
                </el-select>
              </el-form-item>

              <el-form-item :label="$t('结算链网络')" style="flex: 1;">
                <el-select v-model="editForm.network" style="width: 100%">
                  <el-option
                    v-for="net in marketplaceNetworks"
                    :key="net.value"
                    :label="$t(net.label)"
                    :value="net.value"
                  />
                </el-select>
              </el-form-item>
            </div>

            <el-form-item :label="$t('网络易读标签')">
              <el-input
                v-model="editForm.network_label"
                :placeholder="$t('例如: Arbitrum / Base USDC (留空自动按网络推导)')"
                clearable
              />
              <div class="form-hint">{{ $t('在 Hub 页面展示的人性化网络名称，如 Arbitrum / Base USDC') }}</div>
            </el-form-item>

            <el-form-item :label="$t('检索标签')">
              <el-select
                v-model="editForm.tags"
                multiple
                filterable
                allow-create
                default-first-option
                :placeholder="$t('输入标签后回车，例如 Solana、x402、LLM')"
                style="width: 100%"
              >
                <el-option label="x402" value="x402" />
                <el-option label="USDC" value="USDC" />
                <el-option label="Solana" value="Solana" />
                <el-option label="Base" value="Base" />
                <el-option label="Arbitrum" value="Arbitrum" />
                <el-option label="LLM" value="LLM" />
                <el-option label="Web Scraping" value="Web Scraping" />
              </el-select>
            </el-form-item>

            <el-form-item :label="$t('详细功能描述')">
              <el-input
                v-model="editForm.description_md"
                type="textarea"
                :rows="4"
                :placeholder="$t('支持 Markdown：填写 API 功能说明、使用场景与约束条件')"
              />
            </el-form-item>
          </template>

          <template v-else>
            <el-alert
              :title="$t('当前为私有代理路由')"
              type="info"
              :description="$t('该路由仅在网关内部代理转发，不会展示在公开功能商店 (openagentpay.dev/marketplace) 与 /llms.txt 中。如需对外供 AI Agent 发现与付费采购，请开启上方【公开展示】开关。')"
              show-icon
              :closable="false"
              style="margin-top: 16px;"
            />
          </template>
        </div>

        <!-- 步骤 3: 请求入参规范 (支持嵌套参数) -->
        <div v-show="editActiveStep === 2" class="step-panel">
          <div class="step-guide-box">
            <div class="step-guide-title">{{ $t('请求入参规范与代码样例') }}</div>
            <div class="step-guide-desc">{{ $t('结构化定义 API 的输入参数字段与调用样例，系统将自动生成 AI Agent 一键集成的 SDK 代码与契约说明') }}</div>
          </div>

          <div class="param-spec-box" style="width: 100%; margin-bottom: 20px;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
              <span style="font-size: 13px; font-weight: 600; color: #334155;">{{ $t('参数字段列表 (Input Parameters)') }}</span>
              <div style="display: flex; align-items: center; gap: 10px;">
                <span style="font-size: 12px; color: #94a3b8;">{{ $t('支持点号嵌套路径，如 filter.keyword 或 user.address.city') }}</span>
                <el-button type="primary" link icon="Plus" size="small" @click="addParameter(editForm)">
                  {{ $t('添加参数字段') }}
                </el-button>
              </div>
            </div>
            <el-table :data="editForm.parameters" border size="small" style="width: 100%">
              <el-table-column :label="$t('参数名')" min-width="160">
                <template #default="{ row }">
                  <div style="display: flex; align-items: center; gap: 4px;">
                    <span
                      v-if="row.name && row.name.includes('.')"
                      style="font-family: monospace; font-size: 11px; color: #4f46e5; background: #eef2ff; padding: 1px 5px; border-radius: 3px; white-space: nowrap;"
                    >
                      {{ '↳ '.repeat(Math.min(3, row.name.split('.').length - 1)) }}{{ $t('嵌套') }}
                    </span>
                    <el-input v-model="row.name" :placeholder="$t('例如: prompt 或 filter.keyword')" size="small" />
                  </div>
                </template>
              </el-table-column>
              <el-table-column :label="$t('类型')" width="110">
                <template #default="{ row }">
                  <el-select v-model="row.type" size="small">
                    <el-option label="string" value="string" />
                    <el-option label="integer" value="integer" />
                    <el-option label="number" value="number" />
                    <el-option label="boolean" value="boolean" />
                    <el-option label="object" value="object" />
                    <el-option label="array" value="array" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column :label="$t('必填')" width="70" align="center">
                <template #default="{ row }">
                  <el-switch v-model="row.required" size="small" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('描述')" min-width="160">
                <template #default="{ row }">
                  <el-input v-model="row.description" :placeholder="$t('用途说明')" size="small" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('示例值')" min-width="110">
                <template #default="{ row }">
                  <el-input v-model="row.example" :placeholder="$t('例如: 2048')" size="small" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('操作')" width="110" align="center">
                <template #default="{ row, $index }">
                  <el-button
                    link
                    type="primary"
                    size="small"
                    icon="CirclePlus"
                    :title="$t('添加子字段')"
                    @click="addParameter(editForm, row.name || 'nested')"
                  >
                    {{ $t('子字段') }}
                  </el-button>
                  <el-button link type="danger" size="small" icon="Delete" :title="$t('删除')" @click="removeParameter(editForm, $index)" />
                </template>
              </el-table-column>
            </el-table>
          </div>

          <!-- 调用入参样例与响应出参样例 -->
          <el-form-item :label="$t('调用入参样例')">
            <el-input
              v-model="editForm.sample_params_raw"
              type="textarea"
              :rows="3"
              :placeholder="`{\n  &quot;model&quot;: &quot;deepseek-r1&quot;,\n  &quot;prompt&quot;: &quot;Analyze invariants...&quot;\n}`"
            />
            <div class="form-hint">{{ $t('JSON 格式：用于在 Hub 详情页自动生成一键可运行的 Python SDK 示例代码') }}</div>
          </el-form-item>

          <el-form-item :label="$t('响应出参样例')">
            <el-input
              v-model="editForm.sample_response_raw"
              type="textarea"
              :rows="3"
              :placeholder="`{\n  &quot;id&quot;: &quot;chatcmpl_123&quot;,\n  &quot;content&quot;: &quot;...&quot;\n}`"
            />
            <div class="form-hint">{{ $t('JSON 格式：用于在 Hub 详情页展示标准 API 返回格式及 TEE 硬件证明样例') }}</div>
          </el-form-item>
        </div>

        <!-- 步骤 4: 计费策略 -->
        <div v-show="editActiveStep === 3" class="step-panel">
          <div class="step-guide-box">
            <div class="step-guide-title">{{ $t('X402 计费模式与清算收款') }}</div>
            <div class="step-guide-desc">{{ $t('设置客户端调用的支付通道、单次调用价格及收款钱包账户，网关将根据此配置自动下发 HTTP 402 Challenge') }}</div>
          </div>

          <!-- 支付通道 (置于单价之前) -->
          <el-form-item :label="$t('支付通道')" prop="allowed_payment_methods">
            <el-radio-group v-model="editForm.allowed_payment_methods">
              <el-radio value="both">{{ $t('双通道 (虚拟钱包 + 链上支付)') }}</el-radio>
              <el-radio value="wallet">{{ $t('仅虚拟钱包支付') }}</el-radio>
              <el-radio value="onchain">{{ $t('仅链上支付') }}</el-radio>
            </el-radio-group>
          </el-form-item>

          <!-- 单次调用单价 -->
          <el-form-item :label="editForm.allowed_payment_methods === 'both' ? $t('钱包结算单价') : $t('单次调用单价')" prop="price_usdc">
            <el-input-number
              v-model="editForm.price_usdc"
              :min="0"
              :step="0.01"
              :precision="4"
              style="width: 200px"
            />
            <span style="margin-left: 8px; font-weight: 500">
              {{ editForm.allowed_payment_methods === 'onchain' ? 'USDC' : `${tenantStore.currentCurrency} (${tenantStore.currencySymbol})` }}
            </span>
            <div class="form-hint">
              {{ editForm.allowed_payment_methods === 'onchain' ? $t('链上直付单价，计费精度为 6 位小数') : `${$t('内部虚拟钱包结算单价，以租户本位币')} ${tenantStore.currentCurrency} ${$t('计价 (6 位精度)')}` }}
            </div>
          </el-form-item>

          <!-- 双通道模式下的独立链上结算单价 (USDC) -->
          <el-form-item
            v-if="editForm.allowed_payment_methods === 'both'"
            :label="$t('链上结算单价')"
          >
            <el-input-number
              v-model="editForm.onchain_price_usdc"
              :min="0"
              :step="0.01"
              :precision="4"
              style="width: 200px"
            />
            <span style="margin-left: 8px; font-weight: 500">USDC</span>
            <div class="form-hint">
              {{ $t('双通道模式下链上 x402 EIP-3009 签名支付的独立 USDC 定价，网关不做实时汇率换算') }}
            </div>
          </el-form-item>

          <!-- 链上收款地址 (仅在包含链上支付时显示) -->
          <el-form-item
            v-if="editForm.allowed_payment_methods !== 'wallet'"
            :label="$t('链上收款地址')"
            prop="payee_address"
          >
            <el-input
              v-model="editForm.payee_address"
              :placeholder="$t('例如: 0x...')"
              clearable
            />
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
        </div>
      </el-form>

      <template #footer>
        <div class="step-dialog-footer">
          <el-button @click="editDialogVisible = false">{{ $t('取消') }}</el-button>
          <div class="step-nav-buttons">
            <el-button
              v-if="editActiveStep > 0"
              icon="ArrowLeft"
              @click="handleEditPrevStep"
            >
              {{ $t('上一步') }}
            </el-button>
            <el-button
              v-if="editActiveStep < 3"
              type="primary"
              @click="handleEditNextStep"
            >
              {{ $t('下一步') }}
              <el-icon class="el-icon--right"><ArrowRight /></el-icon>
            </el-button>
            <el-button
              v-else
              type="primary"
              :loading="editLoading"
              icon="Check"
              @click="handleUpdateRoute"
            >
              {{ $t('保存变更') }}
            </el-button>
          </div>
        </div>
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
.create-route-steps {
  margin-bottom: 24px;
  padding: 0 12px;
}
.step-panel {
  min-height: 400px;
}
.step-guide-box {
  background: #f8fafc;
  border-left: 4px solid #6366f1;
  padding: 10px 14px;
  border-radius: 0 6px 6px 0;
  margin-bottom: 20px;
}
.step-guide-title {
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
  margin-bottom: 2px;
}
.step-guide-desc {
  font-size: 12px;
  color: #64748b;
  line-height: 1.4;
}
.step-dialog-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}
.step-nav-buttons {
  display: flex;
  align-items: center;
  gap: 10px;
}
.route-wizard-dialog :deep(.el-form-item__label),
.route-edit-dialog :deep(.el-form-item__label) {
  font-weight: 500;
  white-space: nowrap;
}
</style>
