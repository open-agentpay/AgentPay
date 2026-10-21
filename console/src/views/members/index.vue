<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import { memberApi, type AddMemberPayload } from '@/api/client';
import type { UserProfile, UserRole } from '@/types/auth';
import PermissionGate from '@/components/PermissionGate.vue';
import { t } from '@/i18n';

const authStore = useAuthStore();
const tenantStore = useTenantStore();

const loading = ref(false);
const members = ref<UserProfile[]>([]);

// Dialog States
const addDialogVisible = ref(false);
const addLoading = ref(false);
const addFormRef = ref<FormInstance>();
const addForm = reactive<AddMemberPayload>({
  email: '',
  role: 'operator',
  password: ''
});

const addRules: FormRules = {
  email: [
    { required: true, message: t('请输入成员邮箱'), trigger: 'blur' },
    { type: 'email', message: t('请输入有效邮箱格式'), trigger: 'blur' }
  ],
  role: [{ required: true, message: t('请选择分配角色'), trigger: 'change' }],
  password: [
    {
      validator: (_rule, value, callback) => {
        if (value && value.length < 8) {
          callback(new Error(t('初始密码长度不能少于 8 位')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ]
};

// Edit Role Dialog
const editDialogVisible = ref(false);
const editLoading = ref(false);
const currentEditingMember = ref<UserProfile | null>(null);
const selectedNewRole = ref<UserRole>('operator');

async function loadMembers() {
  if (!tenantStore.currentTenantId) return;
  loading.value = true;
  try {
    const list = await memberApi.getMembers(tenantStore.currentTenantId);
    members.value = list;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('加载成员失败');
    ElMessage.error(msg);
  } finally {
    loading.value = false;
  }
}

// Watch tenant switch to reload members
watch(
  () => tenantStore.currentTenantId,
  () => {
    loadMembers();
  }
);

onMounted(() => {
  loadMembers();
});

function openAddDialog() {
  addForm.email = '';
  addForm.role = 'operator';
  addForm.password = '';
  addDialogVisible.value = true;
}

async function handleAddMemberSubmit() {
  if (!addFormRef.value) return;
  await addFormRef.value.validate(async (valid) => {
    if (!valid) return;
    addLoading.value = true;
    try {
      await memberApi.addMember(tenantStore.currentTenantId, {
        email: addForm.email,
        role: addForm.role,
        password: addForm.password || undefined
      });
      ElMessage.success(`${t('成功添加成员')}「${addForm.email}」`);
      addDialogVisible.value = false;
      await loadMembers();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('添加成员失败');
      ElMessage.error(msg);
    } finally {
      addLoading.value = false;
    }
  });
}

function openEditRoleDialog(member: UserProfile) {
  currentEditingMember.value = member;
  selectedNewRole.value = member.role;
  editDialogVisible.value = true;
}

async function handleEditRoleSubmit() {
  if (!currentEditingMember.value) return;
  editLoading.value = true;
  try {
    await memberApi.updateMember(tenantStore.currentTenantId, currentEditingMember.value.member_id, {
      role: selectedNewRole.value
    });
    ElMessage.success(t('成员角色已更新'));
    editDialogVisible.value = false;
    await loadMembers();
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('修改角色失败');
    ElMessage.error(msg);
  } finally {
    editLoading.value = false;
  }
}

async function handleToggleStatus(member: UserProfile) {
  const newStatus = member.status === 'active' ? 'disabled' : 'active';
  const actionText = newStatus === 'disabled' ? t('禁用') : t('重新启用');

  try {
    await ElMessageBox.confirm(`${t('确定要')}${actionText}${t('成员')}「${member.email}」${t('吗？')}`, t('提示'), {
      confirmButtonText: t('确定'),
      cancelButtonText: t('取消'),
      type: 'warning'
    });
  } catch {
    return;
  }

  try {
    await memberApi.updateMember(tenantStore.currentTenantId, member.member_id, {
      status: newStatus
    });
    ElMessage.success(newStatus === 'disabled' ? t('已禁用该成员') : t('已启用该成员'));
    await loadMembers();
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('操作失败');
    ElMessage.error(msg);
  }
}

async function handleRemoveMember(member: UserProfile) {
  try {
    await ElMessageBox.confirm(
      `${t('确定要将成员')}「${member.email}」${t('从当前租户中彻底移除吗？此操作不可撤销。')}`,
      t('危险操作警告'),
      {
        confirmButtonText: t('确认移除'),
        cancelButtonText: t('取消'),
        type: 'warning'
      }
    );
  } catch {
    return;
  }

  try {
    await memberApi.removeMember(tenantStore.currentTenantId, member.member_id);
    ElMessage.success(t('成员已被移除'));
    await loadMembers();
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : t('移除成员失败');
    ElMessage.error(msg);
  }
}

function getRoleTagType(role: UserRole) {
  switch (role) {
    case 'owner':
      return 'danger';
    case 'admin':
      return 'warning';
    case 'operator':
      return 'success';
    case 'viewer':
      return 'info';
    default:
      return 'info';
  }
}
</script>

<template>
  <div class="members-page">
    <el-card shadow="never">
      <template #header>
        <div class="header-box">
          <div>
            <span class="title">{{ $t('组织团队与成员权限管理') }}</span>
            <span class="subtitle">{{ $t('所属租户:') }} {{ tenantStore.currentTenant?.name }} ({{ tenantStore.currentTenantId }})</span>
          </div>
          <PermissionGate :roles="['owner', 'admin']">
            <el-button type="primary" icon="Plus" @click="openAddDialog">
              {{ $t('添加团队成员') }}
            </el-button>
            <template #fallback>
              <el-tag type="info">{{ $t('当前角色') }} ({{ authStore.currentRole }}) {{ $t('为只读视图') }}</el-tag>
            </template>
          </PermissionGate>
        </div>
      </template>

      <!-- Members Table -->
      <el-table v-loading="loading" :data="members" style="width: 100%">
        <el-table-column prop="email" :label="$t('成员邮箱')" min-width="220">
          <template #default="{ row }">
            <div class="member-email-cell">
              <span class="email-text">{{ row.email }}</span>
              <el-tag v-if="row.member_id === authStore.currentUser?.member_id" size="small" type="primary" effect="plain">
                {{ $t('当前账号') }}
              </el-tag>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="member_id" :label="$t('成员 ID')" width="180" />

        <el-table-column :label="$t('分配角色 (RBAC)')" width="150">
          <template #default="{ row }">
            <el-tag :type="getRoleTagType(row.role)" effect="dark" size="small">
              {{ row.role.toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="$t('账号状态')" width="130">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
              {{ row.status === 'active' ? $t('正常 (Active)') : $t('已禁用 (Disabled)') }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="$t('加入时间')" width="180">
          <template #default="{ row }">
            {{ row.created_at ? new Date(row.created_at * 1000).toLocaleString() : '—' }}
          </template>
        </el-table-column>

        <el-table-column :label="$t('操作')" width="220" fixed="right">
          <template #default="{ row }">
            <!-- Edit Role (Owner Only) -->
            <PermissionGate :roles="['owner']">
              <el-button
                link
                type="primary"
                size="small"
                @click="openEditRoleDialog(row)"
              >
                {{ $t('修改角色') }}
              </el-button>
            </PermissionGate>

            <!-- Toggle Disable / Enable (Owner & Admin) -->
            <PermissionGate :roles="['owner', 'admin']">
              <el-button
                link
                :type="row.status === 'active' ? 'warning' : 'success'"
                size="small"
                :disabled="row.role === 'owner'"
                @click="handleToggleStatus(row)"
              >
                {{ row.status === 'active' ? $t('禁用') : $t('启用') }}
              </el-button>
            </PermissionGate>

            <!-- Remove Member (Owner & Admin) -->
            <PermissionGate :roles="['owner', 'admin']">
              <el-button
                link
                type="danger"
                size="small"
                :disabled="row.role === 'owner'"
                @click="handleRemoveMember(row)"
              >
                {{ $t('移除') }}
              </el-button>
            </PermissionGate>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Dialog: Add Member -->
    <el-dialog v-model="addDialogVisible" :title="$t('添加团队成员')" width="500px">
      <el-form ref="addFormRef" :model="addForm" :rules="addRules" label-position="top">
        <el-form-item :label="$t('成员邮箱')" prop="email">
          <el-input v-model="addForm.email" placeholder="colleague@company.com" prefix-icon="Message" />
        </el-form-item>

        <el-form-item :label="$t('授权角色')" prop="role">
          <el-select v-model="addForm.role" style="width: 100%">
            <el-option :label="$t('Admin (租户管理员 - 可管理路由与项目)')" value="admin" />
            <el-option :label="$t('Operator (运维操作员 - 可编辑草稿但不可发布)')" value="operator" />
            <el-option :label="$t('Viewer (只读审计员 - 仅可查看配置与指标)')" value="viewer" />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('初始访问密码 (可选)')" prop="password">
          <el-input
            v-model="addForm.password"
            type="password"
            :placeholder="$t('若填写则不少于 8 位，留空表示由成员后续设置')"
            prefix-icon="Lock"
            show-password
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="addDialogVisible = false">{{ $t('取消') }}</el-button>
        <el-button type="primary" :loading="addLoading" @click="handleAddMemberSubmit">
          {{ $t('确认添加') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Dialog: Edit Role -->
    <el-dialog v-model="editDialogVisible" :title="$t('修改成员角色')" width="460px">
      <div v-if="currentEditingMember" style="margin-bottom: 16px">
        <span>{{ $t('当前成员:') }} <b>{{ currentEditingMember.email }}</b></span>
      </div>
      <el-form label-position="top">
        <el-form-item :label="$t('新角色设定')">
          <el-select v-model="selectedNewRole" style="width: 100%">
            <el-option :label="$t('Owner (组织所有者 / 最高权限)')" value="owner" />
            <el-option :label="$t('Admin (租户管理员)')" value="admin" />
            <el-option :label="$t('Operator (运维操作员)')" value="operator" />
            <el-option :label="$t('Viewer (只读审计员)')" value="viewer" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">{{ $t('取消') }}</el-button>
        <el-button type="primary" :loading="editLoading" @click="handleEditRoleSubmit">
          {{ $t('确认修改') }}
        </el-button>
      </template>
    </el-dialog>
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
.member-email-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}
.email-text {
  font-weight: 500;
  color: #303133;
}
</style>
