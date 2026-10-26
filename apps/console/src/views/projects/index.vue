<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import type { Project } from '@/types/tenant';
import PermissionGate from '@/components/PermissionGate.vue';
import { t } from '@/i18n';

const authStore = useAuthStore();
const tenantStore = useTenantStore();

const loading = ref(false);
const canManage = computed(() => authStore.hasRole(['owner', 'admin']));

async function refreshProjects() {
  loading.value = true;
  try {
    await tenantStore.loadProjects();
  } catch (err: unknown) {
    const msg = err instanceof Error ? t(err.message) : t('加载项目列表失败');
    ElMessage.error(msg);
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  refreshProjects();
});

// ----------------------------------------------------------------------------
// Create Project Dialog
// ----------------------------------------------------------------------------
const createDialogVisible = ref(false);
const createLoading = ref(false);
const createFormRef = ref<FormInstance>();
const createForm = reactive({
  name: '',
  budget_cap_usdc: 1000
});

const projectRules: FormRules = {
  name: [
    { required: true, message: t('请输入项目名称'), trigger: 'blur' },
    { min: 2, max: 50, message: t('长度在 2 到 50 个字符'), trigger: 'blur' }
  ],
  budget_cap_usdc: [
    { required: true, message: t('请输入预算上限'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (value < 0) {
          callback(new Error(t('预算上限不能小于 0')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ]
};

function openCreateDialog() {
  createForm.name = '';
  createForm.budget_cap_usdc = 1000;
  createDialogVisible.value = true;
}

async function handleCreateProject() {
  if (!createFormRef.value) return;
  await createFormRef.value.validate(async (valid) => {
    if (!valid) return;
    createLoading.value = true;
    try {
      const budgetCapBaseUnits = Math.round(createForm.budget_cap_usdc * 1000000);
      await tenantStore.createProject({
        name: createForm.name.trim(),
        budget_cap_base_units: budgetCapBaseUnits
      });
      ElMessage.success(t('项目创建成功'));
      createDialogVisible.value = false;
      await refreshProjects();
    } catch (err: unknown) {
      const msg = err instanceof Error ? t(err.message) : t('创建项目失败');
      ElMessage.error(msg);
    } finally {
      createLoading.value = false;
    }
  });
}

// ----------------------------------------------------------------------------
// Edit Project Dialog
// ----------------------------------------------------------------------------
const editDialogVisible = ref(false);
const editLoading = ref(false);
const editFormRef = ref<FormInstance>();
const editingProjectId = ref('');
const editForm = reactive({
  name: '',
  budget_cap_usdc: 1000,
  status: 'active' as 'active' | 'archived'
});

function openEditDialog(prj: Project) {
  editingProjectId.value = prj.project_id;
  editForm.name = prj.name;
  editForm.budget_cap_usdc = (prj.budget_cap_base_units || 0) / 1000000;
  editForm.status = prj.status || 'active';
  editDialogVisible.value = true;
}

async function handleUpdateProject() {
  if (!editFormRef.value || !editingProjectId.value) return;
  await editFormRef.value.validate(async (valid) => {
    if (!valid) return;
    editLoading.value = true;
    try {
      const budgetCapBaseUnits = Math.round(editForm.budget_cap_usdc * 1000000);
      await tenantStore.updateProject(editingProjectId.value, {
        name: editForm.name.trim(),
        budget_cap_base_units: budgetCapBaseUnits,
        status: editForm.status
      });
      ElMessage.success(t('项目更新成功'));
      editDialogVisible.value = false;
      await refreshProjects();
    } catch (err: unknown) {
      const msg = err instanceof Error ? t(err.message) : t('更新项目失败');
      ElMessage.error(msg);
    } finally {
      editLoading.value = false;
    }
  });
}

// ----------------------------------------------------------------------------
// Toggle Archive / Delete
// ----------------------------------------------------------------------------
async function toggleArchive(prj: Project) {
  const targetStatus = prj.status === 'active' ? 'archived' : 'active';
  const actionText = targetStatus === 'active' ? t('激活') : t('归档');

  try {
    await ElMessageBox.confirm(`${t('确定要')}${actionText}${t('项目')} "${prj.name}" ${t('吗？')}`, t('项目状态确认'), {
      type: 'warning',
      confirmButtonText: t('确定'),
      cancelButtonText: t('取消')
    });

    await tenantStore.updateProject(prj.project_id, {
      status: targetStatus
    });
    ElMessage.success(`${t('项目已')}${actionText}`);
    await refreshProjects();
  } catch (err: unknown) {
    if (err !== 'cancel') {
      const msg = err instanceof Error ? t(err.message) : t('操作失败');
      ElMessage.error(msg);
    }
  }
}

async function handleDeleteProject(prj: Project) {
  try {
    await ElMessageBox.confirm(
      `${t('确定要删除项目')} "${prj.name}" ${t('吗？该操作不可逆，请谨慎操作。')}`,
      t('删除项目确认'),
      {
        type: 'error',
        confirmButtonText: t('确认删除'),
        cancelButtonText: t('取消')
      }
    );

    await tenantStore.deleteProject(prj.project_id);
    ElMessage.success(t('项目已删除'));
    await refreshProjects();
  } catch (err: unknown) {
    if (err !== 'cancel') {
      const msg = err instanceof Error ? t(err.message) : t('删除失败');
      ElMessage.error(msg);
    }
  }
}

function handleSetCurrentProject(projectId: string) {
  tenantStore.switchProject(projectId);
  ElMessage.success(t('已切换为当前工作项目'));
}
</script>

<template>
  <div class="projects-page">
    <el-card shadow="never">
      <template #header>
        <div class="header-box">
          <div>
            <span class="title">{{ $t('业务项目管理 (Project Management)') }}</span>
            <span class="subtitle">{{ $t('所属组织:') }} {{ tenantStore.currentTenant?.name }}</span>
          </div>
          <PermissionGate :roles="['owner', 'admin']">
            <el-button type="primary" icon="Plus" @click="openCreateDialog">
              {{ $t('新建业务项目') }}
            </el-button>
            <template #fallback>
              <el-tag type="info">{{ $t('当前角色') }} ({{ authStore.currentRole }}) {{ $t('为只读视图') }}</el-tag>
            </template>
          </PermissionGate>
        </div>
      </template>

      <!-- Projects Table -->
      <el-table v-loading="loading" :data="tenantStore.availableProjects" stripe style="width: 100%">
        <el-table-column prop="name" :label="$t('项目名称')" min-width="180">
          <template #default="{ row }">
            <div class="project-name-cell">
              <span class="name-text">{{ row.name }}</span>
              <el-tag
                v-if="row.project_id === tenantStore.currentProjectId"
                type="success"
                size="small"
                effect="dark"
              >
                {{ $t('当前工作项目') }}
              </el-tag>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="project_id" :label="$t('项目 ID')" width="200">
          <template #default="{ row }">
            <code class="code-id">{{ row.project_id }}</code>
          </template>
        </el-table-column>

        <el-table-column :label="$t('预算上限')" width="160">
          <template #default="{ row }">
            <span class="budget-text">
              {{ tenantStore.currencySymbol }}{{ ((row.budget_cap_base_units || 0) / 1000000).toLocaleString('en-US', { minimumFractionDigits: 2 }) }} {{ tenantStore.currentCurrency }}
            </span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('状态')" width="110">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
              {{ row.status === 'active' ? $t('正常') : $t('已归档') }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="$t('操作')" width="220" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.project_id !== tenantStore.currentProjectId"
              link
              type="primary"
              size="small"
              @click="handleSetCurrentProject(row.project_id)"
            >
              {{ $t('设为当前') }}
            </el-button>
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
              @click="toggleArchive(row)"
            >
              {{ row.status === 'active' ? $t('归档') : $t('恢复') }}
            </el-button>
            <el-button
              link
              type="danger"
              size="small"
              :disabled="!canManage"
              @click="handleDeleteProject(row)"
            >
              {{ $t('删除') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Project Dialog -->
    <el-dialog
      v-model="createDialogVisible"
      :title="$t('新建业务项目')"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="projectRules"
        label-width="130px"
        status-icon
      >
        <el-form-item :label="$t('项目名称')" prop="name">
          <el-input
            v-model="createForm.name"
            :placeholder="$t('例如: 智能投研业务线')"
            clearable
          />
        </el-form-item>

        <el-form-item :label="$t('预算上限')" prop="budget_cap_usdc">
          <el-input-number
            v-model="createForm.budget_cap_usdc"
            :min="0"
            :step="100"
            :precision="2"
            style="width: 220px"
          />
          <span style="margin-left: 8px; font-weight: 500">{{ tenantStore.currentCurrency }}</span>
          <div class="form-hint">{{ $t('项目在结算周期内允许消费的最大预算上限') }}</div>
        </el-form-item>
      </el-form>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="createDialogVisible = false">{{ $t('取消') }}</el-button>
          <el-button type="primary" :loading="createLoading" @click="handleCreateProject">
            {{ $t('确认新建') }}
          </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- Edit Project Dialog -->
    <el-dialog
      v-model="editDialogVisible"
      :title="$t('编辑业务项目')"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="editFormRef"
        :model="editForm"
        :rules="projectRules"
        label-width="130px"
        status-icon
      >
        <el-form-item :label="$t('项目名称')" prop="name">
          <el-input v-model="editForm.name" clearable />
        </el-form-item>

        <el-form-item :label="$t('预算上限')" prop="budget_cap_usdc">
          <el-input-number
            v-model="editForm.budget_cap_usdc"
            :min="0"
            :step="100"
            :precision="2"
            style="width: 220px"
          />
          <span style="margin-left: 8px; font-weight: 500">{{ tenantStore.currentCurrency }}</span>
        </el-form-item>

        <el-form-item :label="$t('项目状态')">
          <el-radio-group v-model="editForm.status">
            <el-radio value="active">{{ $t('正常运行 (Active)') }}</el-radio>
            <el-radio value="archived">{{ $t('已归档 (Archived)') }}</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="editDialogVisible = false">{{ $t('取消') }}</el-button>
          <el-button type="primary" :loading="editLoading" @click="handleUpdateProject">
            {{ $t('保存修改') }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.projects-page {
  padding: 0;
}
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
.project-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}
.name-text {
  font-weight: 500;
}
.code-id {
  font-family: monospace;
  font-size: 13px;
  background-color: #f4f4f5;
  padding: 2px 6px;
  border-radius: 4px;
  color: #909399;
}
.budget-text {
  font-weight: 600;
  color: #303133;
}
.form-hint {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}
</style>
