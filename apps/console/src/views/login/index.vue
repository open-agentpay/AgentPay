<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import type { UserRole } from '@/types/auth';
import type { CurrencyCode } from '@/types/tenant';
import { useI18n, type LocaleKey } from '@/i18n';
import { updateDocumentTitle } from '@/router';
import { useTheme, type ThemeMode } from '@/theme';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const { locale, setLocale, t } = useI18n();
const { theme, setTheme } = useTheme();

onMounted(() => {
  updateDocumentTitle();
});

function handleLanguageChange(lang: string | number | object) {
  setLocale(lang as LocaleKey);
}

function handleThemeChange(mode: string | number | object) {
  setTheme(mode as ThemeMode);
}

const activeTab = ref<'login' | 'register'>('login');
const loading = ref(false);

// Login Form State - Empty defaults
const loginFormRef = ref<FormInstance>();
const loginForm = reactive({
  email: '',
  password: '',
  role: 'admin' as UserRole
});

const loginRules: FormRules = {
  email: [
    { required: true, message: t('请输入登录邮箱'), trigger: 'blur' },
    { type: 'email', message: t('请输入有效的邮箱地址'), trigger: 'blur' }
  ],
  password: [
    { required: true, message: t('请输入访问密码'), trigger: 'blur' },
    { min: 8, message: t('密码不能少于 8 位字符'), trigger: 'blur' }
  ]
};

// Register Form State - Empty defaults
const registerFormRef = ref<FormInstance>();
const registerForm = reactive({
  tenant_name: '',
  email: '',
  currency: (locale.value === 'zh-cn' ? 'CNY' : 'USD') as CurrencyCode,
  password: '',
  confirm_password: ''
});

const registerRules: FormRules = {
  tenant_name: [
    { required: true, message: t('请输入组织 / 租户名称'), trigger: 'blur' },
    { min: 2, max: 64, message: t('租户名称长度应为 2~64 位'), trigger: 'blur' }
  ],
  email: [
    { required: true, message: t('请输入管理员邮箱'), trigger: 'blur' },
    { type: 'email', message: t('请输入有效的邮箱地址'), trigger: 'blur' }
  ],
  currency: [
    { required: true, message: t('请选择租户结算本位币'), trigger: 'change' }
  ],
  password: [
    { required: true, message: t('请设置初始访问密码'), trigger: 'blur' },
    { min: 8, message: t('密码安全要求最少 8 位字符'), trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: t('请确认访问密码'), trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (value !== registerForm.password) {
          callback(new Error(t('两次输入的密码不一致')));
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
      ElMessage.success(t('登录成功，欢迎进入 AgentPay 工作台'));
      const redirect = (route.query.redirect as string) || '/';
      router.push(redirect);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('登录失败，请检查账号密码');
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
        password: registerForm.password,
        currency: registerForm.currency
      });
      ElMessage.success(`${t('开户成功！已为您初始化租户')}「${res.tenant.name}」(${res.tenant.currency || registerForm.currency})`);
      router.push('/dashboard');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('开户失败，请重试');
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
    ElMessage.info(`${t('已以')} ${role.toUpperCase()} ${t('模式进入控制台')}`);
    router.push('/');
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div :class="['login-wrapper', theme === 'dark' ? 'theme-dark' : 'theme-light']">
    <!-- Ambient glowing backgrounds and grid -->
    <div class="bg-grid-overlay"></div>
    <div class="ambient-glow glow-top-left"></div>
    <div class="ambient-glow glow-bottom-right"></div>
    <div class="ambient-glow glow-center"></div>

    <!-- Top Navigation Bar -->
    <header class="top-nav">
      <div class="nav-left">
        <a href="https://openagentpay.dev" class="brand-link" title="OpenAgentPay">
          <div class="brand-logo-wrap">
            <img src="/logo.png" alt="AgentPay" class="brand-logo-img" />
          </div>
          <span class="brand-name">AgentPay</span>
          <span class="console-pill">Console</span>
        </a>
      </div>

      <div class="nav-right">
        <a
          href="https://openagentpay.dev"
          target="_blank"
          rel="noreferrer"
          class="nav-link-btn"
        >
          <el-icon><Shop /></el-icon>
          <span>{{ t('返回功能商店') }}</span>
        </a>
        <a
          href="https://openagentpay.dev/docs/"
          target="_blank"
          rel="noreferrer"
          class="nav-link-btn"
        >
          <el-icon><Document /></el-icon>
          <span>{{ t('开发文档') }}</span>
        </a>

        <!-- Theme Switcher: Light / Dark -->
        <el-dropdown trigger="click" @command="handleThemeChange">
          <el-button size="small" class="theme-dropdown-btn">
            <el-icon class="mr-1">
              <Sunny v-if="theme === 'light'" />
              <Moon v-else />
            </el-icon>
            {{ theme === 'dark' ? t('暗黑模式') : t('明亮模式') }}
            <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="light" :disabled="theme === 'light'">
                <el-icon><Sunny /></el-icon>
                <span>{{ t('明亮模式') }} (Light)</span>
              </el-dropdown-item>
              <el-dropdown-item command="dark" :disabled="theme === 'dark'">
                <el-icon><Moon /></el-icon>
                <span>{{ t('暗黑模式') }} (Dark)</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <el-dropdown trigger="click" @command="handleLanguageChange">
          <el-button size="small" class="lang-dropdown-btn">
            <el-icon class="mr-1"><Language /></el-icon>
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
      </div>
    </header>

    <!-- Main Content Container -->
    <main class="login-main-container">
      <div class="login-split-layout">
        <!-- Left Side: Value Props & Branding -->
        <section class="hero-showcase">
          <div class="hero-badge">
            <span class="pulse-dot"></span>
            <span>HTTP 402 MACHINE PAYMENT FIREWALL</span>
          </div>

          <h1 class="hero-title">
            {{ t('自主 AI Agent 机器经济支付防火墙') }}
          </h1>

          <p class="hero-desc">
            {{ t('首个专为 AI Agent 打造的非托管微支付与额度风控基础设施。支持原生 HTTP 402 握手、零 Gas 离线签名与毫秒级双通道清结算。') }}
          </p>

          <!-- Feature Cards Grid -->
          <div class="feature-cards-grid">
            <div class="feature-card">
              <div class="feature-icon-box icon-lightning">
                <el-icon :size="20"><Lightning /></el-icon>
              </div>
              <div class="feature-text">
                <div class="feature-heading">{{ t('原生 HTTP 402 协议') }}</div>
                <div class="feature-sub">{{ t('零摩擦机器对机器自动微支付与状态握手') }}</div>
              </div>
            </div>

            <div class="feature-card">
              <div class="feature-icon-box icon-shield">
                <el-icon :size="20"><ShieldCheck /></el-icon>
              </div>
              <div class="feature-text">
                <div class="feature-heading">{{ t('非托管额度防火墙') }}</div>
                <div class="feature-sub">{{ t('单次上限、24h 预算熔断与本地策略保护') }}</div>
              </div>
            </div>

            <div class="feature-card">
              <div class="feature-icon-box icon-network">
                <el-icon :size="20"><Connection /></el-icon>
              </div>
              <div class="feature-text">
                <div class="feature-heading">{{ t('双通道清结算网络') }}</div>
                <div class="feature-sub">{{ t('虚拟钱包账本 + 链上 USDC (Base / Arbitrum / Solana)') }}</div>
              </div>
            </div>

            <div class="feature-card">
              <div class="feature-icon-box icon-chart">
                <el-icon :size="20"><DataAnalysis /></el-icon>
              </div>
              <div class="feature-text">
                <div class="feature-heading">{{ t('实时 Agent 可观测性') }}</div>
                <div class="feature-sub">{{ t('毫秒级决策穿透、成本归属与全量审计') }}</div>
              </div>
            </div>
          </div>

          <!-- Bottom Telemetry Pill -->
          <div class="hero-footer-status">
            <div class="status-indicator">
              <span class="status-live-dot"></span>
              <span class="status-text">{{ t('网关状态: 100% 在线') }} (Verified SLA)</span>
            </div>
            <div class="status-divider">|</div>
            <div class="status-networks">
              <span>Arbitrum</span> • <span>Base</span> • <span>Solana USDC</span>
            </div>
          </div>
        </section>

        <!-- Right Side: Auth Card -->
        <section class="auth-card-section">
          <div class="login-card">
            <div class="login-header">
              <div class="card-logo-container">
                <img src="/logo.png" alt="AgentPay" class="card-logo-img" />
              </div>
              <h2 class="card-title">{{ t('欢迎登录 AgentPay 控制台') }}</h2>
              <p class="card-subtitle">{{ t('输入管理员账号以管理 AI Agent 财务安全与路由策略') }}</p>
            </div>

            <!-- Tab Switcher -->
            <el-tabs v-model="activeTab" class="auth-tabs" stretch>
              <!-- 登录 Tab -->
              <el-tab-pane :label="t('登录控制台')" name="login">
                <el-form
                  ref="loginFormRef"
                  :model="loginForm"
                  :rules="loginRules"
                  label-position="top"
                  class="auth-form"
                  @keyup.enter="handleLoginSubmit"
                >
                  <el-form-item :label="$t('登录邮箱')" prop="email">
                    <el-input
                      v-model="loginForm.email"
                      placeholder="name@company.com"
                      prefix-icon="Message"
                      size="large"
                      clearable
                    />
                  </el-form-item>

                  <el-form-item :label="$t('访问密码')" prop="password">
                    <el-input
                      v-model="loginForm.password"
                      type="password"
                      :placeholder="$t('输入访问密码 (≥ 8 位)')"
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
                    {{ t('login.signInButton') }}
                  </el-button>
                </el-form>

                <el-divider>
                  <span class="divider-text">{{ $t('本地联调快速体验不同 RBAC 权限') }}</span>
                </el-divider>

                <div class="role-quick-cards">
                  <div class="role-btn-item" @click="handleQuickRoleLogin('owner')">
                    <div class="role-title">{{ $t('Owner 模式') }}</div>
                    <div class="role-desc">{{ $t('全部权限 (发布/调账/Kill)') }}</div>
                  </div>

                  <div class="role-btn-item" @click="handleQuickRoleLogin('admin')">
                    <div class="role-title">{{ $t('Admin 模式') }}</div>
                    <div class="role-desc">{{ $t('管理权限 (配置/发布/Kill)') }}</div>
                  </div>

                  <div class="role-btn-item" @click="handleQuickRoleLogin('operator')">
                    <div class="role-title">{{ $t('Operator 模式') }}</div>
                    <div class="role-desc">{{ $t('运维权限 (编辑草稿/不可发布)') }}</div>
                  </div>

                  <div class="role-btn-item" @click="handleQuickRoleLogin('viewer')">
                    <div class="role-title">{{ $t('Viewer 模式') }}</div>
                    <div class="role-desc">{{ $t('只读审计 (无写操作权限)') }}</div>
                  </div>
                </div>
              </el-tab-pane>

              <!-- 开户注册 Tab -->
              <el-tab-pane :label="$t('开户注册')" name="register">
                <el-form
                  ref="registerFormRef"
                  :model="registerForm"
                  :rules="registerRules"
                  label-position="top"
                  class="auth-form"
                  @keyup.enter="handleRegisterSubmit"
                >
                  <el-form-item :label="$t('组织 / 租户名称 (Tenant)')" prop="tenant_name">
                    <el-input
                      v-model="registerForm.tenant_name"
                      :placeholder="$t('例如: DeepSeek Autonomous Labs')"
                      prefix-icon="OfficeBuilding"
                      size="large"
                      clearable
                    />
                  </el-form-item>

                  <el-form-item :label="$t('结算本位法币 (Base Currency)')" prop="currency">
                    <el-select
                      v-model="registerForm.currency"
                      size="large"
                      style="width: 100%"
                    >
                      <el-option label="USD - 美元 ($)" value="USD" />
                      <el-option label="CNY - 人民币 (¥)" value="CNY" />
                      <el-option label="EUR - 欧元 (€)" value="EUR" />
                    </el-select>
                  </el-form-item>

                  <el-form-item :label="$t('管理员邮箱 (Owner Email)')" prop="email">
                    <el-input
                      v-model="registerForm.email"
                      :placeholder="$t('例如: founder@deepseek.com')"
                      prefix-icon="Message"
                      size="large"
                      clearable
                    />
                  </el-form-item>

                  <el-form-item :label="$t('设置初始访问密码')" prop="password">
                    <el-input
                      v-model="registerForm.password"
                      type="password"
                      :placeholder="$t('至少 8 位包含数字与字母')"
                      prefix-icon="Lock"
                      show-password
                      size="large"
                    />
                  </el-form-item>

                  <el-form-item :label="$t('再次确认访问密码')" prop="confirm_password">
                    <el-input
                      v-model="registerForm.confirm_password"
                      type="password"
                      :placeholder="$t('再次输入设置的密码')"
                      prefix-icon="Check"
                      show-password
                      size="large"
                    />
                  </el-form-item>

                  <el-button
                    type="success"
                    class="submit-btn register-btn"
                    size="large"
                    :loading="loading"
                    @click="handleRegisterSubmit"
                  >
                    {{ $t('一键开户注册并进入工作台') }}
                  </el-button>
                </el-form>
              </el-tab-pane>
            </el-tabs>
          </div>
        </section>
      </div>
    </main>

    <!-- Bottom Footer -->
    <footer class="bottom-footer">
      <div class="footer-inner">
        <span>© 2026 OpenAgentPay. Apache-2.0 License.</span>
        <div class="footer-links">
          <a href="https://openagentpay.dev" target="_blank" rel="noreferrer">{{ t('返回功能商店') }}</a>
          <span class="dot">•</span>
          <a href="https://openagentpay.dev/docs/" target="_blank" rel="noreferrer">{{ t('开发文档') }}</a>
          <span class="dot">•</span>
          <a href="https://github.com/open-agentpay/AgentPay" target="_blank" rel="noreferrer">GitHub</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
/* Base Wrapper Layout */
.login-wrapper {
  position: relative;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  overflow-x: hidden;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  transition: background-color 0.25s ease, color 0.25s ease;
}

/* Background Cyber Grid Overlay */
.bg-grid-overlay {
  position: absolute;
  inset: 0;
  background-size: 36px 36px;
  pointer-events: none;
  z-index: 1;
}

/* Ambient Glowing Aurora Blobs */
.ambient-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(120px);
  pointer-events: none;
  z-index: 1;
  transition: opacity 0.3s ease;
}

.glow-top-left {
  top: -150px;
  left: -150px;
  width: 550px;
  height: 550px;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.25) 0%, transparent 70%);
}

.glow-bottom-right {
  bottom: -200px;
  right: -150px;
  width: 650px;
  height: 650px;
  background: radial-gradient(circle, rgba(79, 70, 229, 0.22) 0%, rgba(6, 182, 212, 0.15) 50%, transparent 70%);
}

.glow-center {
  top: 30%;
  left: 45%;
  width: 500px;
  height: 500px;
  background: radial-gradient(circle, rgba(168, 85, 247, 0.12) 0%, transparent 70%);
}

/* Top Navigation Bar */
.top-nav {
  position: relative;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 32px;
  max-width: 1400px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}

.brand-link {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
}

.brand-logo-wrap {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}

.brand-logo-img {
  width: 26px;
  height: 26px;
  object-fit: contain;
}

.brand-name {
  font-size: 20px;
  font-weight: 800;
  letter-spacing: -0.5px;
}

.console-pill {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 2px 7px;
  border-radius: 6px;
  border: 1px solid transparent;
}

.nav-right {
  display: flex;
  align-items: center;
  gap: 14px;
}

.nav-link-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  text-decoration: none;
  padding: 6px 12px;
  border-radius: 8px;
  border: 1px solid transparent;
  backdrop-filter: blur(8px);
  transition: all 0.2s ease;
}

.theme-dropdown-btn,
.lang-dropdown-btn {
  backdrop-filter: blur(8px);
  transition: all 0.2s ease;
}

/* Main Split Layout */
.login-main-container {
  position: relative;
  z-index: 5;
  width: 100%;
  max-width: 1240px;
  margin: 16px auto;
  padding: 0 24px;
  box-sizing: border-box;
}

.login-split-layout {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 48px;
}

/* Left Hero Showcase */
.hero-showcase {
  flex: 1;
  max-width: 580px;
}

.hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.8px;
  padding: 4px 12px;
  border-radius: 20px;
  margin-bottom: 20px;
  border: 1px solid transparent;
}

.pulse-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #6366f1;
  box-shadow: 0 0 10px #6366f1;
  animation: pulse-glow 2s infinite;
}

@keyframes pulse-glow {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(1.3); }
}

.hero-title {
  font-size: 34px;
  font-weight: 800;
  line-height: 1.25;
  letter-spacing: -0.8px;
  margin: 0 0 16px 0;
}

.hero-desc {
  font-size: 14px;
  line-height: 1.65;
  margin-bottom: 32px;
}

/* 4 Feature Cards */
.feature-cards-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin-bottom: 30px;
}

.feature-card {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px;
  border-radius: 12px;
  backdrop-filter: blur(12px);
  border: 1px solid transparent;
  transition: all 0.25s ease;
}

.feature-icon-box {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-lightning {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.icon-shield {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.icon-network {
  background: rgba(99, 102, 241, 0.15);
  color: #818cf8;
  border: 1px solid rgba(99, 102, 241, 0.3);
}

.icon-chart {
  background: rgba(6, 182, 212, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.feature-text {
  min-width: 0;
}

.feature-heading {
  font-size: 13px;
  font-weight: 700;
  margin-bottom: 3px;
}

.feature-sub {
  font-size: 11px;
  line-height: 1.4;
}

/* Telemetry pill */
.hero-footer-status {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-radius: 30px;
  font-size: 12px;
  border: 1px solid transparent;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-live-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 8px #10b981;
  animation: pulse-glow 1.8s infinite;
}

.status-text {
  font-weight: 600;
}

.status-divider {
  opacity: 0.5;
}

.status-networks {
  font-family: monospace;
  font-size: 11px;
}

/* Right Auth Card Section */
.auth-card-section {
  flex: 0 0 460px;
  max-width: 100%;
}

.login-card {
  width: 100%;
  border-radius: 18px;
  padding: 32px 28px;
  box-sizing: border-box;
  transition: all 0.25s ease;
}

.login-header {
  text-align: center;
  margin-bottom: 20px;
}

.card-logo-container {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  margin-bottom: 12px;
  border: 1px solid transparent;
}

.card-logo-img {
  width: 34px;
  height: 34px;
  object-fit: contain;
}

.card-title {
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.4px;
  margin: 0 0 6px 0;
}

.card-subtitle {
  font-size: 12px;
  margin: 0;
  line-height: 1.5;
}

.auth-tabs {
  margin-top: 14px;
}

.auth-tabs :deep(.el-tabs__header) {
  margin-bottom: 20px;
}

.auth-tabs :deep(.el-tabs__item) {
  font-size: 14px;
  font-weight: 600;
}

.auth-tabs :deep(.el-tabs__active-bar) {
  height: 3px;
  border-radius: 2px;
}

.auth-form {
  margin-top: 6px;
}

.auth-form :deep(.el-form-item) {
  margin-bottom: 16px;
}

.auth-form :deep(.el-form-item__label) {
  font-size: 12px;
  font-weight: 600;
  padding-bottom: 4px;
}

.auth-form :deep(.el-input__wrapper) {
  border-radius: 10px;
  padding: 4px 12px;
  transition: all 0.2s;
}

.submit-btn {
  width: 100%;
  margin-top: 10px;
  font-weight: 600;
  border-radius: 10px;
  padding: 12px;
  background: linear-gradient(135deg, #4f46e5 0%, #6366f1 100%);
  border: none;
  color: #ffffff !important;
  box-shadow: 0 4px 14px rgba(79, 70, 229, 0.35);
  transition: all 0.2s;
}

.submit-btn:hover {
  background: linear-gradient(135deg, #4338ca 0%, #4f46e5 100%);
  box-shadow: 0 6px 18px rgba(79, 70, 229, 0.45);
  transform: translateY(-1px);
}

.register-btn {
  background: linear-gradient(135deg, #059669 0%, #10b981 100%);
  color: #ffffff !important;
  box-shadow: 0 4px 14px rgba(16, 185, 129, 0.35);
}

.register-btn:hover {
  background: linear-gradient(135deg, #047857 0%, #059669 100%);
  box-shadow: 0 6px 18px rgba(16, 185, 129, 0.45);
}

.divider-text {
  font-size: 11px;
  font-weight: 500;
}

.role-quick-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: 10px;
}

.role-btn-item {
  border: 1px dashed transparent;
  border-radius: 8px;
  padding: 8px 10px;
  cursor: pointer;
  transition: all 0.2s;
}

.role-title {
  font-size: 12px;
  font-weight: 700;
}

.role-desc {
  font-size: 10px;
  margin-top: 2px;
}

/* Bottom Footer */
.bottom-footer {
  position: relative;
  z-index: 10;
  padding: 16px 32px;
  border-top: 1px solid transparent;
  backdrop-filter: blur(8px);
}

.footer-inner {
  max-width: 1400px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}

.footer-links {
  display: flex;
  align-items: center;
  gap: 12px;
}

.footer-links a {
  text-decoration: none;
  transition: color 0.2s;
}

/* ================= DARK THEME STYLES ================= */
.login-wrapper.theme-dark {
  background-color: #070b14;
  color: #f1f5f9;
}

.login-wrapper.theme-dark .bg-grid-overlay {
  background-image: 
    linear-gradient(to right, rgba(255, 255, 255, 0.03) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(255, 255, 255, 0.03) 1px, transparent 1px);
  mask-image: radial-gradient(ellipse at center, rgba(0, 0, 0, 1) 0%, rgba(0, 0, 0, 0.3) 70%, transparent 100%);
}

.login-wrapper.theme-dark .ambient-glow {
  opacity: 1;
}

.login-wrapper.theme-dark .brand-logo-wrap {
  background: rgba(99, 102, 241, 0.15);
  border-color: rgba(99, 102, 241, 0.3);
  box-shadow: 0 0 15px rgba(99, 102, 241, 0.25);
}

.login-wrapper.theme-dark .brand-name {
  color: #ffffff;
}

.login-wrapper.theme-dark .console-pill {
  background: rgba(99, 102, 241, 0.18);
  color: #818cf8;
  border-color: rgba(99, 102, 241, 0.35);
}

.login-wrapper.theme-dark .nav-link-btn {
  color: #94a3b8;
  border-color: rgba(148, 163, 184, 0.15);
  background: rgba(15, 23, 42, 0.6);
}

.login-wrapper.theme-dark .nav-link-btn:hover {
  color: #ffffff;
  border-color: rgba(99, 102, 241, 0.4);
  background: rgba(99, 102, 241, 0.15);
}

.login-wrapper.theme-dark .theme-dropdown-btn,
.login-wrapper.theme-dark .lang-dropdown-btn {
  background: rgba(15, 23, 42, 0.7) !important;
  border-color: rgba(148, 163, 184, 0.2) !important;
  color: #cbd5e1 !important;
}

.login-wrapper.theme-dark .theme-dropdown-btn:hover,
.login-wrapper.theme-dark .lang-dropdown-btn:hover {
  border-color: #6366f1 !important;
  color: #ffffff !important;
}

.login-wrapper.theme-dark .hero-badge {
  background: rgba(99, 102, 241, 0.12);
  border-color: rgba(99, 102, 241, 0.35);
  color: #a5b4fc;
}

.login-wrapper.theme-dark .hero-title {
  color: #ffffff;
  background: linear-gradient(135deg, #ffffff 0%, #cbd5e1 60%, #818cf8 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.login-wrapper.theme-dark .hero-desc {
  color: #94a3b8;
}

.login-wrapper.theme-dark .feature-card {
  background: rgba(15, 23, 42, 0.55);
  border-color: rgba(255, 255, 255, 0.08);
}

.login-wrapper.theme-dark .feature-card:hover {
  background: rgba(30, 41, 59, 0.65);
  border-color: rgba(99, 102, 241, 0.35);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.25);
}

.login-wrapper.theme-dark .feature-heading {
  color: #f1f5f9;
}

.login-wrapper.theme-dark .feature-sub {
  color: #8294ac;
}

.login-wrapper.theme-dark .hero-footer-status {
  background: rgba(15, 23, 42, 0.6);
  border-color: rgba(255, 255, 255, 0.08);
  color: #94a3b8;
}

.login-wrapper.theme-dark .status-text {
  color: #cbd5e1;
}

.login-wrapper.theme-dark .status-divider {
  color: #475569;
}

.login-wrapper.theme-dark .status-networks {
  color: #94a3b8;
}

.login-wrapper.theme-dark .login-card {
  background: #0d1322;
  border-color: #1e293b;
  box-shadow: 
    0 20px 40px -15px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(255, 255, 255, 0.08),
    0 0 30px rgba(99, 102, 241, 0.15);
}

.login-wrapper.theme-dark .card-logo-container {
  background: rgba(99, 102, 241, 0.15);
  border-color: rgba(99, 102, 241, 0.3);
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.2);
}

.login-wrapper.theme-dark .card-title {
  color: #f8fafc;
}

.login-wrapper.theme-dark .card-subtitle {
  color: #94a3b8;
}

.login-wrapper.theme-dark .auth-tabs :deep(.el-tabs__item) {
  color: #94a3b8;
}

.login-wrapper.theme-dark .auth-tabs :deep(.el-tabs__item.is-active) {
  color: #818cf8;
}

.login-wrapper.theme-dark .auth-tabs :deep(.el-tabs__active-bar) {
  background-color: #818cf8;
}

.login-wrapper.theme-dark .auth-tabs :deep(.el-tabs__nav-wrap::after) {
  background-color: #1e293b;
}

.login-wrapper.theme-dark .auth-form :deep(.el-form-item__label) {
  color: #cbd5e1;
}

.login-wrapper.theme-dark .auth-form :deep(.el-input__wrapper) {
  background-color: #111827;
  box-shadow: 0 0 0 1px #334155 inset;
}

.login-wrapper.theme-dark .auth-form :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #64748b inset;
}

.login-wrapper.theme-dark .auth-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 2px #6366f1 inset;
}

.login-wrapper.theme-dark .auth-form :deep(.el-input__inner) {
  color: #f8fafc;
}

.login-wrapper.theme-dark .auth-form :deep(.el-input__inner::placeholder) {
  color: #64748b;
}

.login-wrapper.theme-dark .divider-text {
  color: #64748b;
}

.login-wrapper.theme-dark .role-btn-item {
  background: #111827;
  border-color: #334155;
}

.login-wrapper.theme-dark .role-btn-item:hover {
  background: #1e293b;
  border-color: #818cf8;
}

.login-wrapper.theme-dark .role-title {
  color: #f1f5f9;
}

.login-wrapper.theme-dark .role-desc {
  color: #94a3b8;
}

.login-wrapper.theme-dark .bottom-footer {
  border-top-color: rgba(255, 255, 255, 0.05);
  background: rgba(7, 11, 20, 0.6);
}

.login-wrapper.theme-dark .footer-inner {
  color: #64748b;
}

.login-wrapper.theme-dark .footer-links a {
  color: #94a3b8;
}

.login-wrapper.theme-dark .footer-links a:hover {
  color: #ffffff;
}

.login-wrapper.theme-dark .footer-links .dot {
  color: #334155;
}

/* ================= LIGHT THEME STYLES ================= */
.login-wrapper.theme-light {
  background-color: #f8fafc;
  color: #0f172a;
}

.login-wrapper.theme-light .bg-grid-overlay {
  background-image: 
    linear-gradient(to right, rgba(148, 163, 184, 0.12) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(148, 163, 184, 0.12) 1px, transparent 1px);
  mask-image: radial-gradient(ellipse at center, rgba(0, 0, 0, 0.8) 0%, rgba(0, 0, 0, 0.2) 70%, transparent 100%);
}

.login-wrapper.theme-light .ambient-glow {
  opacity: 0.45;
}

.login-wrapper.theme-light .brand-logo-wrap {
  background: rgba(99, 102, 241, 0.08);
  border-color: rgba(99, 102, 241, 0.2);
  box-shadow: 0 2px 10px rgba(99, 102, 241, 0.15);
}

.login-wrapper.theme-light .brand-name {
  color: #0f172a;
}

.login-wrapper.theme-light .console-pill {
  background: rgba(99, 102, 241, 0.1);
  color: #4f46e5;
  border-color: rgba(99, 102, 241, 0.25);
}

.login-wrapper.theme-light .nav-link-btn {
  color: #475569;
  border-color: #e2e8f0;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.login-wrapper.theme-light .nav-link-btn:hover {
  color: #4f46e5;
  border-color: #c7d2fe;
  background: #f5f3ff;
}

.login-wrapper.theme-light .theme-dropdown-btn,
.login-wrapper.theme-light .lang-dropdown-btn {
  background: #ffffff !important;
  border-color: #cbd5e1 !important;
  color: #334155 !important;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.login-wrapper.theme-light .theme-dropdown-btn:hover,
.login-wrapper.theme-light .lang-dropdown-btn:hover {
  border-color: #6366f1 !important;
  color: #4f46e5 !important;
}

.login-wrapper.theme-light .hero-badge {
  background: rgba(99, 102, 241, 0.08);
  border-color: rgba(99, 102, 241, 0.25);
  color: #4f46e5;
}

.login-wrapper.theme-light .hero-title {
  color: #0f172a;
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 60%, #4338ca 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.login-wrapper.theme-light .hero-desc {
  color: #475569;
}

.login-wrapper.theme-light .feature-card {
  background: #ffffff;
  border-color: #e2e8f0;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.04);
}

.login-wrapper.theme-light .feature-card:hover {
  background: #ffffff;
  border-color: #c7d2fe;
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(99, 102, 241, 0.1);
}

.login-wrapper.theme-light .feature-heading {
  color: #0f172a;
}

.login-wrapper.theme-light .feature-sub {
  color: #64748b;
}

.login-wrapper.theme-light .hero-footer-status {
  background: #ffffff;
  border-color: #e2e8f0;
  color: #64748b;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.login-wrapper.theme-light .status-text {
  color: #0f172a;
}

.login-wrapper.theme-light .status-divider {
  color: #cbd5e1;
}

.login-wrapper.theme-light .status-networks {
  color: #64748b;
}

.login-wrapper.theme-light .login-card {
  background: #ffffff;
  border-color: #e2e8f0;
  box-shadow: 
    0 20px 40px -15px rgba(0, 0, 0, 0.08),
    0 0 0 1px rgba(0, 0, 0, 0.02),
    0 0 30px rgba(99, 102, 241, 0.06);
}

.login-wrapper.theme-light .card-logo-container {
  background: rgba(99, 102, 241, 0.08);
  border-color: rgba(99, 102, 241, 0.2);
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.1);
}

.login-wrapper.theme-light .card-title {
  color: #0f172a;
}

.login-wrapper.theme-light .card-subtitle {
  color: #64748b;
}

.login-wrapper.theme-light .auth-tabs :deep(.el-tabs__item) {
  color: #64748b;
}

.login-wrapper.theme-light .auth-tabs :deep(.el-tabs__item.is-active) {
  color: #4f46e5;
}

.login-wrapper.theme-light .auth-tabs :deep(.el-tabs__active-bar) {
  background-color: #4f46e5;
}

.login-wrapper.theme-light .auth-tabs :deep(.el-tabs__nav-wrap::after) {
  background-color: #e2e8f0;
}

.login-wrapper.theme-light .auth-form :deep(.el-form-item__label) {
  color: #334155;
}

.login-wrapper.theme-light .auth-form :deep(.el-input__wrapper) {
  background-color: #ffffff;
  box-shadow: 0 0 0 1px #cbd5e1 inset;
}

.login-wrapper.theme-light .auth-form :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #94a3b8 inset;
}

.login-wrapper.theme-light .auth-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 2px #4f46e5 inset;
}

.login-wrapper.theme-light .auth-form :deep(.el-input__inner) {
  color: #0f172a;
}

.login-wrapper.theme-light .auth-form :deep(.el-input__inner::placeholder) {
  color: #94a3b8;
}

.login-wrapper.theme-light .divider-text {
  color: #94a3b8;
}

.login-wrapper.theme-light .role-btn-item {
  background: #f8fafc;
  border-color: #cbd5e1;
}

.login-wrapper.theme-light .role-btn-item:hover {
  background: #eef2ff;
  border-color: #6366f1;
}

.login-wrapper.theme-light .role-title {
  color: #1e293b;
}

.login-wrapper.theme-light .role-desc {
  color: #64748b;
}

.login-wrapper.theme-light .bottom-footer {
  border-top-color: #e2e8f0;
  background: #ffffff;
}

.login-wrapper.theme-light .footer-inner {
  color: #64748b;
}

.login-wrapper.theme-light .footer-links a {
  color: #64748b;
}

.login-wrapper.theme-light .footer-links a:hover {
  color: #0f172a;
}

.login-wrapper.theme-light .footer-links .dot {
  color: #cbd5e1;
}

/* ================= RESPONSIVE LAYOUT ================= */
@media (max-width: 1024px) {
  .login-split-layout {
    flex-direction: column;
    align-items: center;
    gap: 32px;
  }

  .hero-showcase {
    max-width: 100%;
    text-align: center;
  }

  .hero-badge {
    margin-left: auto;
    margin-right: auto;
  }

  .hero-title {
    font-size: 28px;
  }

  .hero-desc {
    margin-bottom: 24px;
  }

  .hero-footer-status {
    margin: 0 auto;
  }

  .auth-card-section {
    width: 100%;
    max-width: 460px;
  }
}

@media (max-width: 640px) {
  .top-nav {
    padding: 14px 16px;
  }

  .nav-link-btn {
    display: none;
  }

  .feature-cards-grid {
    grid-template-columns: 1fr;
  }

  .login-card {
    padding: 24px 18px;
    border-radius: 14px;
  }

  .footer-inner {
    flex-direction: column;
    gap: 8px;
    text-align: center;
  }
}
</style>
