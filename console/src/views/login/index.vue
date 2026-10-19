<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import type { UserRole } from '@/types/auth';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

const activeTab = ref<'login' | 'register'>('login');
const loading = ref(false);

// Login Form State
const loginFormRef = ref<FormInstance>();
const loginForm = reactive({
  email: 'admin@autonomous.ai',
  password: 'SecurePassword123!',
  role: 'admin' as UserRole
});

const loginRules: FormRules = {
  email: [
    { required: true, message: '请输入登录邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入访问密码', trigger: 'blur' },
    { min: 8, message: '密码不能少于 8 位字符', trigger: 'blur' }
  ]
};

// Register Form State
const registerFormRef = ref<FormInstance>();
const registerForm = reactive({
  tenant_name: 'DeepSeek Autonomous Labs',
  email: 'founder@deepseek.com',
  password: 'SecurePassword123!',
  confirm_password: 'SecurePassword123!'
});

const registerRules: FormRules = {
  tenant_name: [
    { required: true, message: '请输入组织 / 租户名称', trigger: 'blur' },
    { min: 2, max: 64, message: '租户名称长度应为 2~64 位', trigger: 'blur' }
  ],
  email: [
    { required: true, message: '请输入管理员邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请设置初始访问密码', trigger: 'blur' },
    { min: 8, message: '密码安全要求最少 8 位字符', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请确认访问密码', trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (value !== registerForm.password) {
          callback(new Error('两次输入的密码不一致'));
        } else {
          callback();
        }
      },
      trigger: 'blur'
    }
  ]
};

async function handleLoginSubmit() {
  if (!loginFormRef.value) return;
  await loginFormRef.value.validate(async (valid) => {
    if (!valid) return;

    loading.value = true;
    try {
      await authStore.login({
        email: loginForm.email,
        password: loginForm.password
      });
      ElMessage.success('登录成功，欢迎进入 AgentPay 工作台');
      const redirect = (route.query.redirect as string) || '/';
      router.push(redirect);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '登录失败，请检查账号密码';
      ElMessage.error(msg);
    } finally {
      loading.value = false;
    }
  });
}

async function handleRegisterSubmit() {
  if (!registerFormRef.value) return;
  await registerFormRef.value.validate(async (valid) => {
    if (!valid) return;

    loading.value = true;
    try {
      const res = await authStore.register({
        tenant_name: registerForm.tenant_name,
        email: registerForm.email,
        password: registerForm.password
      });
      ElMessage.success(`开户成功！已为您初始化租户「${res.tenant.name}」与主钱包`);
      router.push('/dashboard');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '开户失败，请重试';
      ElMessage.error(msg);
    } finally {
      loading.value = false;
    }
  });
}

// Fast demo mock login for testing RBAC matrix
async function handleQuickRoleLogin(role: UserRole) {
  loading.value = true;
  try {
    await authStore.login({
      email: `${role}@agentpay.io`,
      role
    });
    ElMessage.info(`已以 ${role.toUpperCase()} 模式进入控制台`);
    router.push('/');
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="login-wrapper">
    <div class="login-card">
      <div class="login-header">
        <div class="brand-title">AgentPay Console</div>
        <div class="brand-subtitle">面向自主 AI Agent 的 HTTP 402 支付防火墙与管理控制台</div>
      </div>

      <!-- Tab Switcher -->
      <el-tabs v-model="activeTab" class="auth-tabs" stretch>
        <!-- 登录 Tab -->
        <el-tab-pane label="密码登录" name="login">
          <el-form
            ref="loginFormRef"
            :model="loginForm"
            :rules="loginRules"
            label-position="top"
            class="auth-form"
            @keyup.enter="handleLoginSubmit"
          >
            <el-form-item label="登录邮箱" prop="email">
              <el-input
                v-model="loginForm.email"
                placeholder="name@company.com"
                prefix-icon="Message"
                size="large"
              />
            </el-form-item>

            <el-form-item label="访问密码" prop="password">
              <el-input
                v-model="loginForm.password"
                type="password"
                placeholder="输入访问密码 (≥ 8 位)"
                prefix-icon="Lock"
                show-password
                size="large"
              />
            </el-form-item>

            <el-button
              type="primary"
              class="submit-btn"
              size="large"
              :loading="loading"
              @click="handleLoginSubmit"
            >
              登 录
            </el-button>
          </el-form>

          <el-divider>
            <span class="divider-text">本地联调快速体验不同 RBAC 权限</span>
          </el-divider>

          <div class="role-quick-cards">
            <div class="role-btn-item" @click="handleQuickRoleLogin('owner')">
              <div class="role-title">Owner 模式</div>
              <div class="role-desc">全部权限 (发布/调账/Kill)</div>
            </div>

            <div class="role-btn-item" @click="handleQuickRoleLogin('admin')">
              <div class="role-title">Admin 模式</div>
              <div class="role-desc">管理权限 (配置/发布/Kill)</div>
            </div>

            <div class="role-btn-item" @click="handleQuickRoleLogin('operator')">
              <div class="role-title">Operator 模式</div>
              <div class="role-desc">运维权限 (编辑草稿/不可发布)</div>
            </div>

            <div class="role-btn-item" @click="handleQuickRoleLogin('viewer')">
              <div class="role-title">Viewer 模式</div>
              <div class="role-desc">只读审计 (无写操作权限)</div>
            </div>
          </div>
        </el-tab-pane>

        <!-- 开户注册 Tab -->
        <el-tab-pane label="开户注册" name="register">
          <el-form
            ref="registerFormRef"
            :model="registerForm"
            :rules="registerRules"
            label-position="top"
            class="auth-form"
            @keyup.enter="handleRegisterSubmit"
          >
            <el-form-item label="组织 / 租户名称 (Tenant)" prop="tenant_name">
              <el-input
                v-model="registerForm.tenant_name"
                placeholder="例如: DeepSeek Autonomous Labs"
                prefix-icon="OfficeBuilding"
                size="large"
              />
            </el-form-item>

            <el-form-item label="管理员邮箱 (Owner Email)" prop="email">
              <el-input
                v-model="registerForm.email"
                placeholder="例如: founder@deepseek.com"
                prefix-icon="Message"
                size="large"
              />
            </el-form-item>

            <el-form-item label="设置初始访问密码" prop="password">
              <el-input
                v-model="registerForm.password"
                type="password"
                placeholder="至少 8 位包含数字与字母"
                prefix-icon="Lock"
                show-password
                size="large"
              />
            </el-form-item>

            <el-form-item label="再次确认访问密码" prop="confirm_password">
              <el-input
                v-model="registerForm.confirm_password"
                type="password"
                placeholder="再次输入设置的密码"
                prefix-icon="Check"
                show-password
                size="large"
              />
            </el-form-item>

            <el-button
              type="success"
              class="submit-btn"
              size="large"
              :loading="loading"
              @click="handleRegisterSubmit"
            >
              一键开户注册并进入工作台
            </el-button>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<style scoped>
.login-wrapper {
  height: 100vh;
  width: 100vw;
  display: flex;
  justify-content: center;
  align-items: center;
  background: radial-gradient(circle at 50% 50%, #1f2327 0%, #0d0f12 100%);
}

.login-card {
  width: 500px;
  background: #ffffff;
  border-radius: 14px;
  padding: 36px 36px 30px;
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.4);
}

.login-header {
  text-align: center;
  margin-bottom: 20px;
}

.brand-title {
  font-size: 24px;
  font-weight: 700;
  color: #1a1a1a;
  letter-spacing: 0.5px;
}

.brand-subtitle {
  font-size: 13px;
  color: #606266;
  margin-top: 6px;
  line-height: 1.4;
}

.auth-tabs {
  margin-top: 8px;
}

.auth-form {
  margin-top: 16px;
}

.submit-btn {
  width: 100%;
  font-weight: 600;
  margin-top: 10px;
}

.divider-text {
  font-size: 12px;
  color: #909399;
}

.role-quick-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 12px;
}

.role-btn-item {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 10px 12px;
  cursor: pointer;
  background-color: #fafafa;
  transition: all 0.2s;
}

.role-btn-item:hover {
  border-color: #409eff;
  background-color: #ecf5ff;
}

.role-title {
  font-size: 13px;
  font-weight: 600;
  color: #303133;
}

.role-desc {
  font-size: 11px;
  color: #909399;
  margin-top: 4px;
  line-height: 1.3;
}
</style>
