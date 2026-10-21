<script setup lang="ts">
import { computed, onMounted, ref, reactive } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import { routes } from '@/router';
import type { UserRole } from '@/types/auth';
import { MOCK_USERS } from '@/api/mockData';
import { authApi } from '@/api/client';
import { useI18n, type LocaleKey } from '@/i18n';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const tenantStore = useTenantStore();
const { locale, setLocale, t } = useI18n();

function handleLanguageChange(lang: string | number | object) {
  setLocale(lang as LocaleKey);
}

// Password Change Dialog State
const pwdDialogVisible = ref(false);
const pwdLoading = ref(false);
const pwdFormRef = ref<FormInstance>();
const pwdForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: ''
});

const pwdRules: FormRules = {
  old_password: [{ required: true, message: t('请输入当前密码'), trigger: 'blur' }],
  new_password: [
    { required: true, message: t('请输入新密码'), trigger: 'blur' },
    { min: 8, message: t('新密码不能少于 8 位字符'), trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: t('请确认新密码'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (value !== pwdForm.new_password) {
          callback(new Error(t('两次输入的新密码不一致')));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ]
};

function openPasswordDialog() {
  pwdForm.old_password = '';
  pwdForm.new_password = '';
  pwdForm.confirm_password = '';
  pwdDialogVisible.value = true;
}

async function handlePasswordSubmit() {
  if (!pwdFormRef.value) return;
  await pwdFormRef.value.validate(async (valid) => {
    if (!valid) return;
    pwdLoading.value = true;
    try {
      await authApi.changePassword(pwdForm.old_password, pwdForm.new_password);
      ElMessage.success(t('密码修改成功，请重新登录'));
      pwdDialogVisible.value = false;
      handleLogout();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('修改密码失败');
      ElMessage.error(msg);
    } finally {
      pwdLoading.value = false;
    }
  });
}

onMounted(async () => {
  if (authStore.isAuthenticated) {
    try {
      const me = await authApi.getCurrentUser();
      if (me && me.user) {
        authStore.setSession(authStore.token!, me.user, me.tenant, me.permissions);
        if (me.tenant?.tenant_id) {
          tenantStore.currentTenantId = me.tenant.tenant_id;
        }
      }
    } catch {
      // ignore
    }
  }
  await tenantStore.loadTenants();
  await tenantStore.loadProjects();
});

// Filter menu items by user role
const menuItems = computed(() => {
  const rootRoute = routes.find((r) => r.path === '/');
  if (!rootRoute || !rootRoute.children) return [];

  return rootRoute.children.filter((child) => {
    if (child.meta?.hideInMenu) return false;
    if (!child.meta?.roles || child.meta.roles.length === 0) return true;
    return authStore.hasRole(child.meta.roles);
  });
});

const activeMenu = computed(() => route.path);

const roleTagType = computed(() => {
  switch (authStore.currentRole) {
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
});

function handleTenantChange(val: string) {
  tenantStore.switchTenant(val);
}

function handleProjectChange(val: string) {
  tenantStore.switchProject(val);
}

function handleLogout() {
  authStore.logout();
  router.push('/login');
}

// Quick switch role utility for testing & demo purposes
function switchRole(role: UserRole) {
  const mockUser = MOCK_USERS[role];
  if (mockUser) {
    authStore.setSession(`mock_token_${role}`, mockUser);
    // Refresh current route permissions or redirect if 403
    if (route.meta?.roles && !authStore.hasRole(route.meta.roles)) {
      router.push('/dashboard');
    }
  }
}
</script>

<template>
  <el-container class="layout-container">
    <!-- Sidebar -->
    <el-aside width="240px" class="aside-menu">
      <div class="logo-box">
        <el-icon :size="24" class="logo-icon"><Coin /></el-icon>
        <span class="logo-text">AgentPay Console</span>
      </div>

      <el-menu
        :default-active="activeMenu"
        class="sidebar-el-menu"
        background-color="#141414"
        text-color="#c0c4cc"
        active-text-color="#409eff"
        router
      >
        <el-menu-item
          v-for="item in menuItems"
          :key="item.path"
          :index="'/' + item.path"
        >
          <el-icon v-if="item.meta?.icon">
            <component :is="item.meta.icon" />
          </el-icon>
          <template #title>
            <span>{{ item.meta?.titleKey ? t(String(item.meta.titleKey)) : item.meta?.title }}</span>
          </template>
        </el-menu-item>
      </el-menu>

      <div class="aside-footer">
        <div class="version-label">AgentPay v0.1.0-poc</div>
        <div class="sub-label">HTTP 402 Financial Firewall</div>
      </div>
    </el-aside>

    <!-- Main Container -->
    <el-container>
      <!-- Top Navigation Header -->
      <el-header height="60px" class="top-header">
        <div class="header-left">
          <!-- Tenant Switcher -->
          <span class="selector-label">{{ t('layout.tenant') }}:</span>
          <el-select
            v-model="tenantStore.currentTenantId"
            :placeholder="t('layout.selectTenant')"
            style="width: 220px; margin-right: 16px"
            @change="handleTenantChange"
          >
            <el-option
              v-for="tItem in tenantStore.tenants"
              :key="tItem.tenant_id"
              :label="tItem.name"
              :value="tItem.tenant_id"
            >
              <div class="tenant-option-item">
                <span>{{ tItem.name }}</span>
                <el-tag size="small" type="info" effect="plain">{{ tItem.tenant_id.slice(0, 8) }}</el-tag>
              </div>
            </el-option>
          </el-select>

          <!-- Project Switcher -->
          <span class="selector-label">{{ t('layout.project') }}:</span>
          <el-select
            v-model="tenantStore.currentProjectId"
            :placeholder="t('layout.selectProject')"
            style="width: 240px"
            @change="handleProjectChange"
          >
            <el-option
              v-for="p in tenantStore.availableProjects"
              :key="p.project_id"
              :label="p.name"
              :value="p.project_id"
            />
          </el-select>
        </div>

        <div class="header-right">
          <!-- Language Switcher -->
          <el-dropdown trigger="click" style="margin-right: 14px" @command="handleLanguageChange">
            <el-button size="small" plain>
              {{ locale === 'zh-cn' ? '🇨🇳 简体中文' : '🇺🇸 English' }}
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="zh-cn" :disabled="locale === 'zh-cn'">
                  🇨🇳 简体中文
                </el-dropdown-item>
                <el-dropdown-item command="en" :disabled="locale === 'en'">
                  🇺🇸 English
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <!-- Current Role Badge -->
          <el-tooltip :content="t('layout.roleTooltip')" placement="bottom">
            <el-tag :type="roleTagType" effect="dark" class="role-badge">
              {{ t('layout.role') }}: {{ authStore.currentRole?.toUpperCase() }}
            </el-tag>
          </el-tooltip>

          <!-- Fast Role Switcher for Demo -->
          <el-dropdown trigger="click" style="margin-right: 16px">
            <el-button size="small" type="primary" plain>
              {{ t('layout.quickRoleSwitch') }}
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="switchRole('owner')">{{ t('layout.switchRoleOwner') }}</el-dropdown-item>
                <el-dropdown-item @click="switchRole('admin')">{{ t('layout.switchRoleAdmin') }}</el-dropdown-item>
                <el-dropdown-item @click="switchRole('operator')">{{ t('layout.switchRoleOperator') }}</el-dropdown-item>
                <el-dropdown-item @click="switchRole('viewer')">{{ t('layout.switchRoleViewer') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <!-- User Profile Dropdown -->
          <el-dropdown trigger="click">
            <div class="user-avatar-wrap">
              <el-avatar :size="32" icon="UserFilled" class="user-avatar" />
              <span class="user-email">{{ authStore.currentUser?.name || authStore.currentUser?.email }}</span>
              <el-icon><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>
                  ID: {{ authStore.currentUser?.member_id }}
                </el-dropdown-item>
                <el-dropdown-item @click="openPasswordDialog">
                  <el-icon><Key /></el-icon>{{ t('layout.changePassword') }}
                </el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">
                  <el-icon><SwitchButton /></el-icon>{{ t('layout.logout') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- Main Content Area -->
      <el-main class="main-body">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>

    <!-- Dialog: Change Password -->
    <el-dialog v-model="pwdDialogVisible" :title="t('layout.changePassword')" width="460px">
      <el-form ref="pwdFormRef" :model="pwdForm" :rules="pwdRules" label-position="top">
        <el-form-item :label="t('layout.currentPassword')" prop="old_password">
          <el-input
            v-model="pwdForm.old_password"
            type="password"
            :placeholder="t('layout.currentPasswordPlaceholder')"
            prefix-icon="Lock"
            show-password
          />
        </el-form-item>

        <el-form-item :label="t('layout.newPassword')" prop="new_password">
          <el-input
            v-model="pwdForm.new_password"
            type="password"
            :placeholder="t('layout.newPasswordPlaceholder')"
            prefix-icon="Lock"
            show-password
          />
        </el-form-item>

        <el-form-item :label="t('layout.confirmPassword')" prop="confirm_password">
          <el-input
            v-model="pwdForm.confirm_password"
            type="password"
            :placeholder="t('layout.confirmPasswordPlaceholder')"
            prefix-icon="Check"
            show-password
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="pwdDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="pwdLoading" @click="handlePasswordSubmit">
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>
  </el-container>
</template>

<style scoped>
.layout-container {
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background-color: #f5f7fa;
}

.aside-menu {
  background-color: #141414;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #2b2b2b;
}

.logo-box {
  height: 60px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  background-color: #0d0d0d;
  color: #fff;
  border-bottom: 1px solid #222;
}

.logo-icon {
  color: #409eff;
  margin-right: 10px;
}

.logo-text {
  font-weight: 700;
  font-size: 16px;
  letter-spacing: 0.5px;
}

.sidebar-el-menu {
  border-right: none;
  flex: 1;
}

.aside-footer {
  padding: 16px 20px;
  border-top: 1px solid #222;
  font-size: 12px;
  color: #606266;
}

.sub-label {
  font-size: 11px;
  color: #909399;
  margin-top: 4px;
}

.top-header {
  background-color: #fff;
  border-bottom: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
}

.header-left {
  display: flex;
  align-items: center;
}

.selector-label {
  font-size: 13px;
  font-weight: 500;
  color: #606266;
  margin-right: 8px;
}

.tenant-option-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.header-right {
  display: flex;
  align-items: center;
}

.role-badge {
  margin-right: 16px;
  font-weight: 600;
  letter-spacing: 0.5px;
}

.user-avatar-wrap {
  display: flex;
  align-items: center;
  cursor: pointer;
  gap: 8px;
}

.user-email {
  font-size: 13px;
  color: #303133;
}

.main-body {
  padding: 24px;
  overflow-y: auto;
  height: calc(100vh - 60px);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
