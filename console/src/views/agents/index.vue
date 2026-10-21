<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import PermissionGate from '@/components/PermissionGate.vue';
import { agentApi } from '@/api/client';
import { useI18n } from '@/i18n';

const router = useRouter();
const { t } = useI18n();
import type {
  Agent,
  AgentAPIKey,
  AgentTimelineEvent,
  KYAProfile,
  KYAAutonomyLevel,
  KYARiskLevel,
  KYAVerificationStatus
} from '@/types/agent';

const authStore = useAuthStore();
const tenantStore = useTenantStore();

// State
const loading = ref(false);
const agents = ref<Agent[]>([]);

// Filters
const selectedProject = ref<string>('all');
const selectedStatus = ref<string>('all');
const selectedKyaStatus = ref<string>('all');
const searchQuery = ref<string>('');

// KYA Profile Drawer State
const kyaDrawerVisible = ref(false);
const activeKyaAgent = ref<Agent | null>(null);

// Timeline Drawer State
const timelineDrawerVisible = ref(false);
const timelineLoading = ref(false);
const activeTimelineAgent = ref<Agent | null>(null);
const timelineEvents = ref<AgentTimelineEvent[]>([]);
const timelineSessionFilter = ref<string>('all');
const timelineEventTypeFilter = ref<string>('all');

// Available distinct sessions in current timeline
const availableSessions = computed(() => {
  const set = new Set<string>();
  for (const e of timelineEvents.value) {
    if (e.declared?.session_id) {
      set.add(e.declared.session_id);
    }
  }
  return Array.from(set);
});

// Filtered timeline events
const filteredTimelineEvents = computed(() => {
  return timelineEvents.value.filter((e) => {
    if (timelineSessionFilter.value !== 'all' && e.declared?.session_id !== timelineSessionFilter.value) {
      return false;
    }
    if (timelineEventTypeFilter.value !== 'all' && e.event_type !== timelineEventTypeFilter.value) {
      return false;
    }
    return true;
  });
});

// Audit Detail Modal
const auditModalVisible = ref(false);
const activeAuditEvent = ref<AgentTimelineEvent | null>(null);

// Kill Switch Dialog State
const killDialogVisible = ref(false);
const killSubmitting = ref(false);
const targetKillAgent = ref<Agent | null>(null);
const killForm = ref({
  reason: '',
  revokeKeys: false
});
const killRules: FormRules = {
  reason: [
    { required: true, message: t('必须填写停用原因与依据 (Required reason)'), trigger: 'blur' },
    { min: 5, message: t('停用原因不能少于 5 个字符'), trigger: 'blur' }
  ]
};

// Create / Edit Agent Dialog State
const agentDialogVisible = ref(false);
const isEditing = ref(false);
const editingAgentId = ref('');
const agentSubmitting = ref(false);
const agentFormRef = ref<FormInstance>();
const agentActiveTab = ref('basic');

const agentForm = ref({
  name: '',
  project_id: '',
  budget_cap_usdc: 100,
  rate_limit_rpm: 60,
  // KYA Fields
  kya: {
    model_provider: 'DeepSeek',
    model_version: 'deepseek-v3',
    framework: 'LangChain v0.3',
    hosting_env: 'Cloud Worker',
    owner_name: '',
    owner_email: '',
    department: '',
    authorized_by: '',
    autonomy_level: 'L2_semi_autonomous' as KYAAutonomyLevel,
    risk_level: 'low' as KYARiskLevel,
    status: 'verified' as KYAVerificationStatus,
    max_single_spend_usdc: 5,
    allowed_tools_str: 'web_search, vector_store',
    notes: ''
  }
});

const agentRules: FormRules = {
  name: [{ required: true, message: t('请输入 Agent 名称'), trigger: 'blur' }],
  project_id: [{ required: true, message: t('请选择归属业务项目'), trigger: 'change' }]
};

// API Keys Management Dialog State
const keysDialogVisible = ref(false);
const activeKeysAgent = ref<Agent | null>(null);
const agentKeys = ref<AgentAPIKey[]>([]);
const keysLoading = ref(false);
const newKeyName = ref('');
const newlyCreatedRawKey = ref('');

// Formatters
function formatUSDC(baseUnits: number | undefined): string {
  if (baseUnits === undefined || isNaN(baseUnits)) return '0.00';
  return (baseUnits / 1000000).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 4
  });
}

function formatDate(timestamp: number | string | undefined): string {
  if (!timestamp) return '-';
  try {
    const ms = typeof timestamp === 'number' ? (timestamp < 1e11 ? timestamp * 1000 : timestamp) : timestamp;
    return new Date(ms).toLocaleString('zh-CN', { hour12: false });
  } catch {
    return String(timestamp);
  }
}

// Load Agents
async function loadAgents() {
  const tenantId = tenantStore.currentTenantId;
  if (!tenantId) return;

  loading.value = true;
  try {
    const list = await agentApi.listAgents(
      tenantId,
      selectedProject.value !== 'all' ? selectedProject.value : undefined,
      selectedStatus.value !== 'all' ? selectedStatus.value : undefined
    );
    agents.value = list;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('加载 Agent 列表失败');
    ElMessage.error(msg);
  } finally {
    loading.value = false;
  }
}

watch(
  () => tenantStore.currentTenantId,
  () => {
    loadAgents();
  }
);

onMounted(() => {
  loadAgents();
});

// Filtered agents
const filteredAgents = computed(() => {
  return agents.value.filter((a) => {
    if (selectedProject.value !== 'all' && a.project_id !== selectedProject.value) {
      return false;
    }
    if (selectedStatus.value !== 'all' && a.status !== selectedStatus.value) {
      return false;
    }
    if (selectedKyaStatus.value !== 'all' && a.kya_profile?.status !== selectedKyaStatus.value) {
      return false;
    }
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.trim().toLowerCase();
      const inName = a.name.toLowerCase().includes(q);
      const inId = a.agent_id.toLowerCase().includes(q);
      const inModel = (a.kya_profile?.model_version || '').toLowerCase().includes(q);
      const inOwner = (a.kya_profile?.owner_name || '').toLowerCase().includes(q);
      if (!inName && !inId && !inModel && !inOwner) return false;
    }
    return true;
  });
});

// KYA Helpers
function getAutonomyLabel(level: KYAAutonomyLevel | undefined): string {
  switch (level) {
    case 'L1_assisted':
      return t('L1 辅助 (Assisted)');
    case 'L2_semi_autonomous':
      return t('L2 半自主 (Semi-Auto)');
    case 'L3_fully_autonomous':
      return t('L3 全自主 (Autonomous)');
    default:
      return t('L2 半自主');
  }
}

function getRiskTagType(risk: KYARiskLevel | undefined) {
  switch (risk) {
    case 'low':
      return 'success';
    case 'medium':
      return 'warning';
    case 'high':
    case 'critical':
      return 'danger';
    default:
      return 'info';
  }
}

function getKyaStatusTagType(status: KYAVerificationStatus | undefined) {
  switch (status) {
    case 'verified':
      return 'success';
    case 'pending_review':
      return 'warning';
    case 'unverified':
      return 'info';
    default:
      return 'info';
  }
}

// Open KYA Profile Drawer
function openKyaDrawer(agent: Agent) {
  activeKyaAgent.value = agent;
  kyaDrawerVisible.value = true;
}

// Open Timeline Drawer
async function openTimelineDrawer(agent: Agent) {
  activeTimelineAgent.value = agent;
  timelineDrawerVisible.value = true;
  timelineLoading.value = true;
  timelineSessionFilter.value = 'all';
  timelineEventTypeFilter.value = 'all';

  try {
    const tenantId = tenantStore.currentTenantId;
    const events = await agentApi.getAgentTimeline(tenantId, agent.agent_id);
    timelineEvents.value = events;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('获取审计时间线失败');
    ElMessage.error(msg);
  } finally {
    timelineLoading.value = false;
  }
}

// Navigate to payments query view with agent_id pre-filtered
function goToAgentPayments(agent: Agent) {
  router.push({ path: '/payments', query: { agent_id: agent.agent_id } });
}

// Open Audit Detail Modal
function openAuditDetail(event: AgentTimelineEvent) {
  activeAuditEvent.value = event;
  auditModalVisible.value = true;
}

// Open Kill Switch Dialog
function openKillDialog(agent: Agent) {
  targetKillAgent.value = agent;
  killForm.value = {
    reason: '',
    revokeKeys: false
  };
  killDialogVisible.value = true;
}

async function handleKillSubmit() {
  if (!targetKillAgent.value) return;
  if (!killForm.value.reason || killForm.value.reason.trim().length < 5) {
    ElMessage.warning(t('停用原因必须填写且不少于 5 个字符'));
    return;
  }

  killSubmitting.value = true;
  try {
    const tenantId = tenantStore.currentTenantId;
    await agentApi.killAgent(tenantId, targetKillAgent.value.agent_id, {
      reason: killForm.value.reason.trim(),
      revoke: killForm.value.revokeKeys
    });
    ElMessage.success(`Agent ${targetKillAgent.value.name} ${t('已成功执行一键熔断停用')}`);
    killDialogVisible.value = false;
    await loadAgents();
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('一键停用失败');
    ElMessage.error(msg);
  } finally {
    killSubmitting.value = false;
  }
}

// Resume Agent
async function handleResumeAgent(agent: Agent) {
  try {
    await ElMessageBox.confirm(
      `${t('确定要')} ${t('恢复')} Agent "${agent.name}" (${agent.agent_id}) ${t('的运行状态吗？')}`,
      t('恢复 Agent 运行确认'),
      { confirmButtonText: t('确认恢复'), cancelButtonText: t('取消'), type: 'info' }
    );
  } catch {
    return;
  }

  try {
    const tenantId = tenantStore.currentTenantId;
    await agentApi.updateAgentStatus(tenantId, agent.agent_id, 'active');
    ElMessage.success(`Agent ${agent.name} ${t('已成功恢复运行状态')}`);
    await loadAgents();
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('恢复状态失败');
    ElMessage.error(msg);
  }
}

// Open Create / Edit Agent Dialog
function openCreateAgentDialog() {
  isEditing.value = false;
  editingAgentId.value = '';
  agentActiveTab.value = 'basic';
  agentForm.value = {
    name: '',
    project_id: tenantStore.currentProjectId || tenantStore.availableProjects[0]?.project_id || '',
    budget_cap_usdc: 100,
    rate_limit_rpm: 60,
    kya: {
      model_provider: 'DeepSeek',
      model_version: 'deepseek-v3',
      framework: 'LangChain v0.3',
      hosting_env: 'Cloud Worker',
      owner_name: authStore.currentUser?.name || '',
      owner_email: authStore.currentUser?.email || '',
      department: 'Autonomous Intelligence Team',
      authorized_by: '',
      autonomy_level: 'L2_semi_autonomous',
      risk_level: 'low',
      status: 'verified',
      max_single_spend_usdc: 5,
      allowed_tools_str: 'web_search, vector_store',
      notes: ''
    }
  };
  agentDialogVisible.value = true;
}

function openEditAgentDialog(agent: Agent) {
  isEditing.value = true;
  editingAgentId.value = agent.agent_id;
  agentActiveTab.value = 'basic';
  const kya = agent.kya_profile;

  agentForm.value = {
    name: agent.name,
    project_id: agent.project_id,
    budget_cap_usdc: agent.policy_config?.budget_cap_base_units
      ? agent.policy_config.budget_cap_base_units / 1000000
      : 100,
    rate_limit_rpm: agent.policy_config?.rate_limit_rpm || 60,
    kya: {
      model_provider: kya?.model_provider || 'DeepSeek',
      model_version: kya?.model_version || 'deepseek-v3',
      framework: kya?.framework || 'Custom SDK',
      hosting_env: kya?.hosting_env || 'Cloud Worker',
      owner_name: kya?.owner_name || '',
      owner_email: kya?.owner_email || '',
      department: kya?.department || '',
      authorized_by: kya?.authorized_by || '',
      autonomy_level: kya?.autonomy_level || 'L2_semi_autonomous',
      risk_level: kya?.risk_level || 'low',
      status: kya?.status || 'verified',
      max_single_spend_usdc: kya?.max_single_spend ? kya.max_single_spend / 1000000 : 5,
      allowed_tools_str: kya?.allowed_tools?.join(', ') || '',
      notes: kya?.notes || ''
    }
  };
  agentDialogVisible.value = true;
}

async function handleAgentFormSubmit() {
  if (!agentFormRef.value) return;
  await agentFormRef.value.validate(async (valid) => {
    if (!valid) return;
    const tenantId = tenantStore.currentTenantId;
    if (!tenantId) return;

    agentSubmitting.value = true;
    try {
      const allowedTools = agentForm.value.kya.allowed_tools_str
        ? agentForm.value.kya.allowed_tools_str.split(',').map((s) => s.trim()).filter(Boolean)
        : [];

      const kyaPayload: KYAProfile = {
        model_provider: agentForm.value.kya.model_provider,
        model_version: agentForm.value.kya.model_version,
        framework: agentForm.value.kya.framework,
        hosting_env: agentForm.value.kya.hosting_env,
        owner_name: agentForm.value.kya.owner_name,
        owner_email: agentForm.value.kya.owner_email,
        department: agentForm.value.kya.department,
        authorized_by: agentForm.value.kya.authorized_by,
        autonomy_level: agentForm.value.kya.autonomy_level,
        risk_level: agentForm.value.kya.risk_level,
        status: agentForm.value.kya.status,
        max_single_spend: Math.round(agentForm.value.kya.max_single_spend_usdc * 1000000),
        allowed_tools: allowedTools,
        notes: agentForm.value.kya.notes,
        verified_at: new Date().toISOString()
      };

      const policyConfig = {
        budget_cap_base_units: Math.round(agentForm.value.budget_cap_usdc * 1000000),
        rate_limit_rpm: agentForm.value.rate_limit_rpm,
        kya: kyaPayload
      };

      if (isEditing.value) {
        await agentApi.updateAgent(tenantId, editingAgentId.value, {
          name: agentForm.value.name,
          policy_config: policyConfig,
          kya_profile: kyaPayload
        });
        ElMessage.success(`Agent ${agentForm.value.name} ${t('更新成功')}`);
      } else {
        await agentApi.createAgent(tenantId, {
          project_id: agentForm.value.project_id,
          name: agentForm.value.name,
          policy_config: policyConfig,
          kya_profile: kyaPayload
        });
        ElMessage.success(`Agent ${agentForm.value.name} ${t('注册并完成 KYA 登记')}`);
      }

      agentDialogVisible.value = false;
      await loadAgents();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('操作失败');
      ElMessage.error(msg);
    } finally {
      agentSubmitting.value = false;
    }
  });
}

// API Keys Management
async function openKeysDialog(agent: Agent) {
  activeKeysAgent.value = agent;
  keysDialogVisible.value = true;
  newKeyName.value = '';
  newlyCreatedRawKey.value = '';
  await loadAgentKeys();
}

async function loadAgentKeys() {
  if (!activeKeysAgent.value) return;
  keysLoading.value = true;
  try {
    const tenantId = tenantStore.currentTenantId;
    const keys = await agentApi.listAgentKeys(tenantId, activeKeysAgent.value.agent_id);
    agentKeys.value = keys;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('加载凭证失败');
    ElMessage.error(msg);
  } finally {
    keysLoading.value = false;
  }
}

async function handleCreateKey() {
  if (!activeKeysAgent.value) return;
  if (!newKeyName.value.trim()) {
    ElMessage.warning(t('请输入 Key 标识名称'));
    return;
  }

  try {
    const tenantId = tenantStore.currentTenantId;
    const res = await agentApi.createAgentKey(tenantId, activeKeysAgent.value.agent_id, {
      project_id: activeKeysAgent.value.project_id,
      name: newKeyName.value.trim()
    });
    ElMessage.success(t('API Key 创建成功'));
    if (res.raw_key) {
      newlyCreatedRawKey.value = res.raw_key;
    }
    newKeyName.value = '';
    await loadAgentKeys();
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('创建 API Key 失败');
    ElMessage.error(msg);
  }
}

async function handleRevokeKey(key: AgentAPIKey) {
  try {
    await ElMessageBox.confirm(
      `${t('确定要撤销 Key')} "${key.name}" (${key.key_prefix}...) ${t('吗？')}`,
      t('撤销凭证确认'),
      { confirmButtonText: t('确认撤销'), cancelButtonText: t('取消'), type: 'warning' }
    );
  } catch {
    return;
  }

  try {
    const tenantId = tenantStore.currentTenantId;
    await agentApi.revokeAgentKey(tenantId, key.key_id);
    ElMessage.success(t('API Key 撤销成功并已即时失效'));
    await loadAgentKeys();
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('撤销失败');
    ElMessage.error(msg);
  }
}
</script>

<template>
  <div class="agents-page-container" v-loading="loading">
    <!-- Header -->
    <div class="header-banner">
      <div>
        <h2 class="title">{{ t('agents.title') }}</h2>
        <div class="subtitle">
          {{ $t('归属租户:') }} <strong>{{ tenantStore.currentTenant?.name || tenantStore.currentTenantId }}</strong>
          <span class="divider">|</span>
          {{ $t('租户ID:') }} <code>{{ tenantStore.currentTenantId }}</code>
          <span class="divider">|</span>
          {{ $t('当前角色:') }} <el-tag size="small" type="info">{{ authStore.currentRole?.toUpperCase() }}</el-tag>
        </div>
      </div>
      <div class="header-actions">
        <el-button icon="Refresh" @click="loadAgents">{{ $t('刷新') }}</el-button>
        <PermissionGate :roles="['owner', 'admin']">
          <el-button type="primary" icon="Plus" @click="openCreateAgentDialog">
            {{ $t('新建 Agent 实体 (含 KYA 登记)') }}
          </el-button>
        </PermissionGate>
      </div>
    </div>

    <!-- Filter Bar -->
    <el-card shadow="never" class="filter-card">
      <div class="filter-row">
        <div class="filter-items">
          <el-select v-model="selectedProject" :placeholder="$t('业务项目')" style="width: 180px">
            <el-option :label="$t('全部业务项目')" value="all" />
            <el-option
              v-for="p in tenantStore.availableProjects"
              :key="p.project_id"
              :label="p.name"
              :value="p.project_id"
            />
          </el-select>

          <el-select v-model="selectedStatus" :placeholder="$t('Agent 状态')" style="width: 140px">
            <el-option :label="$t('全部状态')" value="all" />
            <el-option :label="$t('运行中 (active)')" value="active" />
            <el-option :label="$t('已熔断 (suspended)')" value="suspended" />
            <el-option :label="$t('已作废 (revoked)')" value="revoked" />
          </el-select>

          <el-select v-model="selectedKyaStatus" :placeholder="$t('KYA 认证状态')" style="width: 160px">
            <el-option :label="$t('全部 KYA 状态')" value="all" />
            <el-option :label="$t('已认证 (verified)')" value="verified" />
            <el-option :label="$t('待审核 (pending)')" value="pending_review" />
            <el-option :label="$t('未认证 (unverified)')" value="unverified" />
          </el-select>

          <el-input
            v-model="searchQuery"
            :placeholder="$t('搜索 Agent 名称 / ID / 模型版本 / 负责人...')"
            prefix-icon="Search"
            clearable
            style="width: 300px"
          />
        </div>
        <div class="filter-summary">
          <span>{{ $t('共') }} <strong>{{ filteredAgents.length }}</strong> {{ $t('个 Agent 实体') }}</span>
        </div>
      </div>
    </el-card>

    <!-- Agent List Table -->
    <el-card shadow="never" class="table-card">
      <el-table :data="filteredAgents" stripe style="width: 100%" :empty-text="$t('当前租户下暂无 Agent 实体')">
        <!-- Name & ID -->
        <el-table-column :label="$t('Agent 实体 / 客户端标识')" min-width="230">
          <template #default="{ row }">
            <div class="agent-name-cell">
              <strong>{{ row.name }}</strong>
              <div class="code-id">{{ row.agent_id }}</div>
            </div>
          </template>
        </el-table-column>

        <!-- KYA (Know Your Agent) Summary Pill -->
        <el-table-column :label="$t('KYA 认证与自治等级')" min-width="260">
          <template #default="{ row }">
            <div class="kya-cell" v-if="row.kya_profile">
              <div class="kya-tags">
                <el-tag size="small" type="primary">
                  {{ row.kya_profile.model_provider }} / {{ row.kya_profile.model_version }}
                </el-tag>
                <el-tag size="small" :type="getRiskTagType(row.kya_profile.risk_level)">
                  {{ getAutonomyLabel(row.kya_profile.autonomy_level) }}
                </el-tag>
                <el-tag size="small" :type="getKyaStatusTagType(row.kya_profile.status)">
                  {{ row.kya_profile.status === 'verified' ? $t('已认证') : (row.kya_profile.status === 'pending_review' ? $t('待审核') : $t('未认证')) }}
                </el-tag>
              </div>
              <el-button link type="primary" size="small" @click="openKyaDrawer(row)">
                {{ $t('查看完整 KYA 档案') }}
              </el-button>
            </div>
            <span v-else class="text-muted">{{ $t('待补全 KYA 档案') }}</span>
          </template>
        </el-table-column>

        <!-- Metrics: Requests, Spend, Rejections -->
        <el-table-column :label="$t('近期调用量')" width="110" align="right">
          <template #default="{ row }">
            {{ (row.requests_count || 0).toLocaleString() }}
          </template>
        </el-table-column>

        <el-table-column :label="$t('累计花费 (USDC)')" width="140" align="right">
          <template #default="{ row }">
            <span style="font-weight: 600; color: #303133">
              {{ formatUSDC(row.total_spend_base_units) }} USDC
            </span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('策略拒绝率')" width="110" align="center">
          <template #default="{ row }">
            <span :class="{ 'text-danger': parseFloat(String(row.reject_rate || 0)) > 5 }">
              {{ row.reject_rate || '0.0%' }}
            </span>
          </template>
        </el-table-column>

        <!-- Status -->
        <el-table-column :label="$t('运行状态')" width="130" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'active'" type="success" size="small">
              {{ $t('运行中 (Active)') }}
            </el-tag>
            <el-tag v-else-if="row.status === 'suspended'" type="danger" size="small">
              {{ $t('已熔断 (Suspended)') }}
            </el-tag>
            <el-tag v-else type="info" size="small">
              {{ $t('已作废 (Revoked)') }}
            </el-tag>
          </template>
        </el-table-column>

        <!-- Actions -->
        <el-table-column :label="$t('操作与审计')" width="360" fixed="right">
          <template #default="{ row }">
            <!-- Timeline -->
            <el-button link type="primary" size="small" icon="Clock" @click="openTimelineDrawer(row)">
              {{ $t('行为时间线') }}
            </el-button>

            <!-- Payments -->
            <el-button link type="warning" size="small" icon="Tickets" @click="goToAgentPayments(row)">
              {{ $t('消费记录') }}
            </el-button>

            <!-- API Keys -->
            <el-button link type="info" size="small" icon="Key" @click="openKeysDialog(row)">
              {{ $t('凭证') }}
            </el-button>

            <!-- Kill Switch (Owner/Admin) -->
            <PermissionGate :roles="['owner', 'admin']">
              <el-button
                v-if="row.status === 'active'"
                link
                type="danger"
                size="small"
                icon="SwitchButton"
                @click="openKillDialog(row)"
              >
                {{ $t('一键熔断') }}
              </el-button>
              <el-button
                v-else-if="row.status === 'suspended'"
                link
                type="success"
                size="small"
                icon="VideoPlay"
                @click="handleResumeAgent(row)"
              >
                {{ $t('恢复运行') }}
              </el-button>
            </PermissionGate>

            <!-- Edit (Owner/Admin) -->
            <PermissionGate :roles="['owner', 'admin']">
              <el-button link type="primary" size="small" icon="Edit" @click="openEditAgentDialog(row)">
                {{ $t('编辑') }}
              </el-button>
            </PermissionGate>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- KYA Profile Drawer -->
    <el-drawer
      v-model="kyaDrawerVisible"
      :title="$t('Agent KYA (Know Your Agent) 信用与安全档案')"
      size="560px"
      direction="rtl"
    >
      <div v-if="activeKyaAgent" class="kya-drawer-content">
        <div class="kya-header-card">
          <div class="kya-header-main">
            <h3>{{ activeKyaAgent.name }}</h3>
            <code>{{ activeKyaAgent.agent_id }}</code>
          </div>
          <div class="kya-header-badge">
            <el-tag size="large" :type="getKyaStatusTagType(activeKyaAgent.kya_profile?.status)" effect="dark">
              {{ activeKyaAgent.kya_profile?.status === 'verified' ? $t('KYA 已认证') : $t('待人工复核') }}
            </el-tag>
          </div>
        </div>

        <el-divider content-position="left">{{ $t('1. 基础模型与技术溯源 (Model Provenance)') }}</el-divider>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="$t('模型供应商')">
            {{ activeKyaAgent.kya_profile?.model_provider || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('核心模型版本')">
            {{ activeKyaAgent.kya_profile?.model_version || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('Agent 开发框架')">
            {{ activeKyaAgent.kya_profile?.framework || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('部署运行环境')">
            {{ activeKyaAgent.kya_profile?.hosting_env || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('远程认证证明 (Attestation)')" :span="2">
            <div class="code-id" style="word-break: break-all;">
              {{ activeKyaAgent.kya_profile?.attestation_hash || 'SHA256:7f8a9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e' }}
            </div>
          </el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">{{ $t('2. 权责人与责任归属 (Responsibility & Accountability)') }}</el-divider>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="$t('技术负责人')">
            {{ activeKyaAgent.kya_profile?.owner_name || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('责任人邮箱')">
            {{ activeKyaAgent.kya_profile?.owner_email || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('归属业务部门')">
            {{ activeKyaAgent.kya_profile?.department || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('业务授权人')">
            {{ activeKyaAgent.kya_profile?.authorized_by || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('认证生效时间')" :span="2">
            {{ formatDate(activeKyaAgent.kya_profile?.verified_at) }}
          </el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">{{ $t('3. 自主度分级与风控评级 (Autonomy & Risk Classification)') }}</el-divider>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="$t('自治能力等级')">
            <el-tag size="small" type="primary">
              {{ getAutonomyLabel(activeKyaAgent.kya_profile?.autonomy_level) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('风险等级评定')">
            <el-tag size="small" :type="getRiskTagType(activeKyaAgent.kya_profile?.risk_level)">
              {{ activeKyaAgent.kya_profile?.risk_level?.toUpperCase() || 'LOW' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('单笔最大限额')" :span="2">
            {{ formatUSDC(activeKyaAgent.kya_profile?.max_single_spend) }} USDC
          </el-descriptions-item>
          <el-descriptions-item :label="$t('授权工具与能力')" :span="2">
            <div class="tool-tags">
              <el-tag
                v-for="t in (activeKyaAgent.kya_profile?.allowed_tools || [])"
                :key="t"
                size="small"
                effect="plain"
              >
                {{ t }}
              </el-tag>
            </div>
          </el-descriptions-item>
        </el-descriptions>

        <div v-if="activeKyaAgent.kya_profile?.notes" style="margin-top: 16px">
          <el-alert
            :title="`${$t('安全合规审计附注:')} ${activeKyaAgent.kya_profile.notes}`"
            type="info"
            show-icon
            :closable="false"
          />
        </div>
      </div>
    </el-drawer>

    <!-- Timeline Drawer -->
    <el-drawer
      v-model="timelineDrawerVisible"
      :title="`${$t('Agent 行为审计时间线: ${activeTimelineAgent?.name ||')}${activeTimelineAgent?.name || ''}`"
      size="700px"
      direction="rtl"
    >
      <div class="timeline-container" v-loading="timelineLoading">
        <!-- Invariant Legend Banner -->
        <el-alert
          type="info"
          show-icon
          :closable="false"
          style="margin-bottom: 16px"
        >
          <template #title>
            <strong>{{ t('agents.contractNotice') }}</strong>
          </template>
          <template #default>
            <div style="font-size: 12px; line-height: 1.6">
              <el-tag size="small" type="success" effect="dark">{{ $t('[网关观测 observed]') }}</el-tag>
              {{ $t('客观事实（路由、实际耗时、扣费金额、策略判定结果），由网关生成，防伪造；') }}<br />
              <el-tag size="small" type="warning" effect="dark">{{ $t('[Agent自报 declared]') }}</el-tag>
              {{ $t('客户端自报字段（会话 session_id、任务 task_id、工具名），仅用于关联归类，严禁用于授权或计费判定。') }}
            </div>
          </template>
        </el-alert>

        <!-- Filters: Session Filter & Event Type -->
        <div class="timeline-filter-bar">
          <el-select
            v-model="timelineSessionFilter"
            :placeholder="$t('按自报会话 (session_id) 聚合')"
            style="width: 240px"
          >
            <el-option :label="$t('全部工作流会话')" value="all" />
            <el-option
              v-for="s in availableSessions"
              :key="s"
              :label="`${$t('会话:')} ${s}`"
              :value="s"
            />
          </el-select>

          <el-select
            v-model="timelineEventTypeFilter"
            :placeholder="$t('事件类型')"
            style="width: 180px"
          >
            <el-option :label="$t('全部事件类型')" value="all" />
            <el-option :label="$t('请求调用 (agent.request)')" value="agent.request" />
            <el-option :label="$t('策略判定 (policy_decision)')" value="agent.policy_decision" />
            <el-option :label="$t('资金结算 (agent.spend)')" value="agent.spend" />
            <el-option :label="$t('管控熔断 (agent.control)')" value="agent.control" />
            <el-option :label="$t('风控告警 (agent.anomaly)')" value="agent.anomaly" />
          </el-select>

          <el-button
            type="warning"
            plain
            icon="Tickets"
            @click="activeTimelineAgent && goToAgentPayments(activeTimelineAgent)"
          >
            {{ t('agents.goToPayments') }}
          </el-button>
        </div>

        <!-- Vertical Timeline -->
        <div v-if="filteredTimelineEvents.length > 0" class="timeline-list">
          <el-timeline>
            <el-timeline-item
              v-for="e in filteredTimelineEvents"
              :key="e.event_id"
              :timestamp="formatDate(e.occurred_at)"
              placement="top"
              :type="e.event_type === 'agent.anomaly' || e.event_type === 'agent.control' ? 'danger' : 'primary'"
            >
              <el-card shadow="never" class="timeline-event-card">
                <div class="event-header">
                  <span class="event-title">{{ e.summary }}</span>
                  <el-tag size="small" type="info">{{ e.event_type }}</el-tag>
                </div>

                <!-- Observed vs Declared tags -->
                <div class="event-pill-row">
                  <!-- Observed Badges -->
                  <el-tag size="small" type="success" effect="plain" v-if="e.observed.status_code">
                    {{ $t('[观测] HTTP') }} {{ e.observed.status_code }}
                  </el-tag>
                  <el-tag size="small" type="success" effect="plain" v-if="e.observed.latency_ms">
                    {{ $t('[观测] 耗时') }} {{ e.observed.latency_ms }}ms
                  </el-tag>
                  <el-tag size="small" type="success" effect="plain" v-if="e.observed.cost_base_units">
                    {{ $t('[观测] 花费') }} {{ formatUSDC(e.observed.cost_base_units) }} USDC
                  </el-tag>
                  <el-tag
                    size="small"
                    :type="e.observed.policy_decision === 'allow' ? 'success' : 'danger'"
                    effect="plain"
                    v-if="e.observed.policy_decision"
                  >
                    {{ $t('[观测] 决策') }} {{ e.observed.policy_decision.toUpperCase() }}
                  </el-tag>

                  <!-- Declared Badges -->
                  <el-tag size="small" type="warning" effect="plain" v-if="e.declared?.session_id">
                    {{ $t('[自报会话]') }} {{ e.declared.session_id }}
                  </el-tag>
                  <el-tag size="small" type="warning" effect="plain" v-if="e.declared?.task_id">
                    {{ $t('[自报任务]') }} {{ e.declared.task_id }}
                  </el-tag>
                </div>

                <div class="event-action-row">
                  <el-button link type="primary" size="small" @click="openAuditDetail(e)">
                    {{ $t('查看详细审计穿透数据') }} &rarr;
                  </el-button>
                </div>
              </el-card>
            </el-timeline-item>
          </el-timeline>
        </div>

        <el-empty v-else :description="$t('所选会话或类型暂无行为事件')" />
      </div>
    </el-drawer>

    <!-- Audit Detail Modal -->
    <el-dialog
      v-model="auditModalVisible"
      :title="t('agents.auditEventDetailTitle')"
    >
      <div v-if="activeAuditEvent">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="$t('事件 ID (Event ID)')" :span="2">
            <code>{{ activeAuditEvent.event_id }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('事件类型')">
            <el-tag size="small">{{ activeAuditEvent.event_type }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('触发主体 (Actor)')">
            {{ activeAuditEvent.actor }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('发生时间')" :span="2">
            {{ formatDate(activeAuditEvent.occurred_at) }}
          </el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">{{ $t('网关客观观测 (Observed Invariants)') }}</el-divider>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="$t('请求链路 ID')">
            <code>{{ activeAuditEvent.observed.request_id }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('命中的路由 ID')">
            <code>{{ activeAuditEvent.observed.route_id || '-' }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('生效快照版本')">
            <code>{{ activeAuditEvent.observed.config_version || '-' }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('HTTP 状态码')">
            {{ activeAuditEvent.observed.status_code || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('实际耗时')">
            {{ activeAuditEvent.observed.latency_ms ? `${activeAuditEvent.observed.latency_ms} ms` : '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('计费扣除金额')">
            {{ formatUSDC(activeAuditEvent.observed.cost_base_units) }} USDC
          </el-descriptions-item>
          <el-descriptions-item :label="$t('关联支付意图 (PaymentIntent)')">
            <code>{{ activeAuditEvent.observed.payment_intent_id || '-' }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('资金冻结凭单 (Hold)')">
            <code>{{ activeAuditEvent.observed.hold_id || '-' }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('请求哈希指纹')" :span="2">
            <code style="word-break: break-all;">
              {{ activeAuditEvent.observed.request_hash || '-' }}
            </code>
          </el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">{{ $t('Agent 客户端自报声明 (Declared)') }}</el-divider>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="$t('自报会话 ID (session_id)')">
            <code>{{ activeAuditEvent.declared?.session_id || '-' }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('自报任务 ID (task_id)')">
            <code>{{ activeAuditEvent.declared?.task_id || '-' }}</code>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('自报调用工具名 (tool_name)')">
            {{ activeAuditEvent.declared?.tool_name || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('自报 Prompt 摘要')">
            {{ activeAuditEvent.declared?.user_prompt_summary || '-' }}
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-dialog>

    <!-- Kill Switch Dialog -->
    <el-dialog
      v-model="killDialogVisible"
      :title="$t('⚠️ 触发 Agent 一键熔断停用 (Kill Switch)')"
      width="500px"
      :close-on-click-modal="false"
    >
      <div v-if="targetKillAgent">
        <el-alert
          :title="$t('执行熔断后，网关数据面将立即生效并阻断该 Agent 后续的一切入站请求！')"
          type="error"
          show-icon
          :closable="false"
          style="margin-bottom: 16px"
        />

        <el-form ref="killFormRef" :model="killForm" :rules="killRules" label-width="100px">
          <el-form-item :label="$t('目标 Agent')">
            <strong>{{ targetKillAgent.name }}</strong>
            <div class="code-id">{{ targetKillAgent.agent_id }}</div>
          </el-form-item>

          <el-form-item :label="$t('停用原因')" prop="reason">
            <el-input
              v-model="killForm.reason"
              type="textarea"
              :rows="3"
              :placeholder="$t('必须填写停用原因与安全依据，该原因将记入 agent.control 不可变审计日志 (必填)')"
            />
          </el-form-item>

          <el-form-item :label="$t('作废凭证')">
            <el-checkbox v-model="killForm.revokeKeys">
              {{ $t('同时作废所有关联的 API Keys (Revoke Bound Keys)') }}
            </el-checkbox>
          </el-form-item>
        </el-form>
      </div>

      <template #footer>
        <el-button @click="killDialogVisible = false">{{ $t('取消') }}</el-button>
        <el-button type="danger" :loading="killSubmitting" @click="handleKillSubmit">
          {{ $t('确认执行熔断停用') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Create / Edit Agent Dialog (with KYA) -->
    <el-dialog
      v-model="agentDialogVisible"
      :title="isEditing ? $t('编辑 Agent 配置与 KYA 档案') : $t('新建 Agent 实体并完成 KYA 登记')"
      width="640px"
      :close-on-click-modal="false"
    >
      <el-form ref="agentFormRef" :model="agentForm" :rules="agentRules" label-width="110px">
        <el-tabs v-model="agentActiveTab">
          <!-- Tab 1: Basic -->
          <el-tab-pane :label="$t('基础信息')" name="basic">
            <el-form-item :label="$t('Agent 名称')" prop="name">
              <el-input v-model="agentForm.name" :placeholder="$t('例如: Autonomous Research Spider')" />
            </el-form-item>

            <el-form-item :label="$t('归属项目')" prop="project_id">
              <el-select v-model="agentForm.project_id" :placeholder="$t('请选择业务项目')" style="width: 100%">
                <el-option
                  v-for="p in tenantStore.availableProjects"
                  :key="p.project_id"
                  :label="p.name"
                  :value="p.project_id"
                />
              </el-select>
            </el-form-item>

            <el-form-item :label="$t('独立预算上限')">
              <el-input-number v-model="agentForm.budget_cap_usdc" :min="1" :max="50000" :step="10" />
              <span style="margin-left: 10px">USDC</span>
            </el-form-item>

            <el-form-item :label="$t('速率限制 (RPM)')">
              <el-input-number v-model="agentForm.rate_limit_rpm" :min="1" :max="10000" :step="10" />
              <span style="margin-left: 10px">{{ $t('次 / 分钟') }}</span>
            </el-form-item>
          </el-tab-pane>

          <!-- Tab 2: KYA Profile -->
          <el-tab-pane :label="$t('KYA 认证与自治档案')" name="kya">
            <el-alert
              :title="$t('KYA (Know Your Agent) 登记用于建立 Agent 的模型溯源、自主等级与责任人映射')"
              type="info"
              show-icon
              :closable="false"
              style="margin-bottom: 14px"
            />

            <el-form-item :label="$t('模型供应商')">
              <el-select v-model="agentForm.kya.model_provider" style="width: 100%">
                <el-option label="DeepSeek" value="DeepSeek" />
                <el-option label="OpenAI" value="OpenAI" />
                <el-option label="Anthropic" value="Anthropic" />
                <el-option label="Self-hosted / Private" value="Self-hosted / Private" />
              </el-select>
            </el-form-item>

            <el-form-item :label="$t('基础模型')">
              <el-input v-model="agentForm.kya.model_version" :placeholder="$t('例如: deepseek-v3 / claude-3-5-sonnet')" />
            </el-form-item>

            <el-form-item :label="$t('开发框架')">
              <el-input v-model="agentForm.kya.framework" :placeholder="$t('例如: LangChain v0.3 / CrewAI / AutoGen')" />
            </el-form-item>

            <el-form-item :label="$t('部署环境')">
              <el-input v-model="agentForm.kya.hosting_env" :placeholder="$t('例如: TEE Enclave / Cloud Worker')" />
            </el-form-item>

            <el-form-item :label="$t('技术负责人')">
              <el-input v-model="agentForm.kya.owner_name" :placeholder="$t('负责人姓名')" />
            </el-form-item>

            <el-form-item :label="$t('负责人邮箱')">
              <el-input v-model="agentForm.kya.owner_email" placeholder="owner@company.com" />
            </el-form-item>

            <el-form-item :label="$t('业务部门')">
              <el-input v-model="agentForm.kya.department" :placeholder="$t('业务团队')" />
            </el-form-item>

            <el-form-item :label="$t('自治能力等级')">
              <el-select v-model="agentForm.kya.autonomy_level" style="width: 100%">
                <el-option :label="$t('L1 - 辅助执行 (人工把关每一笔支出)')" value="L1_assisted" />
                <el-option :label="$t('L2 - 半自主代理 (限额内自动结算，超限审批)')" value="L2_semi_autonomous" />
                <el-option :label="$t('L3 - 全自主代理 (完全自动化工作流调用)')" value="L3_fully_autonomous" />
              </el-select>
            </el-form-item>

            <el-form-item :label="$t('风险评级')">
              <el-select v-model="agentForm.kya.risk_level" style="width: 100%">
                <el-option :label="$t('Low (低风险)')" value="low" />
                <el-option :label="$t('Medium (中风险)')" value="medium" />
                <el-option :label="$t('High (高风险)')" value="high" />
                <el-option :label="$t('Critical (极高风险)')" value="critical" />
              </el-select>
            </el-form-item>

            <el-form-item :label="$t('KYA 认证状态')">
              <el-select v-model="agentForm.kya.status" style="width: 100%">
                <el-option :label="$t('Verified (已认证通过)')" value="verified" />
                <el-option :label="$t('Pending Review (待人工审核)')" value="pending_review" />
                <el-option :label="$t('Unverified (未认证)')" value="unverified" />
              </el-select>
            </el-form-item>

            <el-form-item :label="$t('单笔限额上限')">
              <el-input-number v-model="agentForm.kya.max_single_spend_usdc" :min="0.1" :max="1000" :step="1" />
              <span style="margin-left: 10px">USDC</span>
            </el-form-item>

            <el-form-item :label="$t('授权工具列表')">
              <el-input
                v-model="agentForm.kya.allowed_tools_str"
                :placeholder="$t('以逗号分隔，如: web_search, vector_store')"
              />
            </el-form-item>
          </el-tab-pane>
        </el-tabs>
      </el-form>

      <template #footer>
        <el-button @click="agentDialogVisible = false">{{ $t('取消') }}</el-button>
        <el-button type="primary" :loading="agentSubmitting" @click="handleAgentFormSubmit">
          {{ isEditing ? $t('保存修改') : $t('确认创建') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- API Keys Management Dialog -->
    <el-dialog
      v-model="keysDialogVisible"
      :title="`${$t('Agent API Key 凭证管理: ${activeKeysAgent?.name ||')}${activeKeysAgent?.name || ''}`"
      width="640px"
    >
      <div v-loading="keysLoading">
        <div class="new-key-box">
          <el-input v-model="newKeyName" :placeholder="$t('新凭证名称，如: Spider Worker 01')" style="width: 320px" />
          <el-button type="primary" icon="Plus" @click="handleCreateKey">{{ $t('生成新 API Key') }}</el-button>
        </div>

        <div v-if="newlyCreatedRawKey" style="margin-bottom: 16px">
          <el-alert
            :title="$t('⚠️ 新生成凭证仅展示一次，请妥善保存：')"
            type="warning"
            :closable="false"
          >
            <template #default>
              <div class="raw-key-box">
                <code>{{ newlyCreatedRawKey }}</code>
              </div>
            </template>
          </el-alert>
        </div>

        <el-table :data="agentKeys" stripe size="small" style="width: 100%" :empty-text="$t('当前 Agent 暂无绑定的 API Key 凭证')">
          <el-table-column prop="name" :label="$t('凭证名称')" width="160" />
          <el-table-column :label="$t('Key 前缀')" width="160">
            <template #default="{ row }">
              <code>{{ row.key_prefix }}...</code>
            </template>
          </el-table-column>
          <el-table-column :label="$t('状态')" width="100">
            <template #default="{ row }">
              <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
                {{ row.status === 'active' ? $t('正常') : $t('已撤销') }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="$t('创建时间')" width="140">
            <template #default="{ row }">
              {{ formatDate(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('操作')" width="80" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="row.status === 'active'"
                link
                type="danger"
                size="small"
                @click="handleRevokeKey(row)"
              >
                {{ $t('撤销') }}
              </el-button>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.agents-page-container {
  padding: 0;
}

.header-banner {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.title {
  margin: 0 0 6px 0;
  font-size: 20px;
  font-weight: 600;
  color: #303133;
}

.subtitle {
  font-size: 13px;
  color: #909399;
}

.divider {
  margin: 0 8px;
  color: #dcdfe6;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-card {
  margin-bottom: 16px;
  border-radius: 8px;
}

.filter-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.filter-items {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-summary {
  font-size: 13px;
  color: #909399;
}

.table-card {
  border-radius: 8px;
}

.agent-name-cell strong {
  font-size: 14px;
  color: #303133;
}

.code-id {
  font-family: monospace;
  font-size: 11px;
  color: #909399;
  margin-top: 2px;
}

.kya-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.kya-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.text-danger {
  color: #f56c6c !important;
  font-weight: 600;
}

.text-muted {
  color: #909399;
  font-size: 12px;
}

.kya-drawer-content {
  padding: 0 8px;
}

.kya-header-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #f8fafc;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}

.kya-header-main h3 {
  margin: 0 0 4px 0;
  font-size: 16px;
}

.tool-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.timeline-container {
  padding: 0 8px;
}

.timeline-filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.timeline-event-card {
  border-radius: 6px;
  margin-bottom: 8px;
  background: #ffffff;
}

.event-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.event-title {
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}

.event-pill-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.event-action-row {
  display: flex;
  justify-content: flex-end;
}

.new-key-box {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.raw-key-box {
  background: #ffffff;
  padding: 8px 12px;
  border-radius: 4px;
  border: 1px dashed #e6a23c;
  margin-top: 6px;
  font-family: monospace;
  word-break: break-all;
}
</style>
