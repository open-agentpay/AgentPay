<script setup lang="ts">
import { computed } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import PermissionGate from '@/components/PermissionGate.vue';
import { useI18n } from '@/i18n';

const authStore = useAuthStore();
const tenantStore = useTenantStore();
const { t } = useI18n();

const rolePermissionsList = computed(() => {
  const role = authStore.currentRole;
  switch (role) {
    case 'owner':
      return [
        { name: t('租户与组织管理'), allowed: true, detail: t('创建项目、邀请成员、转让所有权') },
        { name: t('dashboard.routesModule'), allowed: true, detail: t('dashboard.routesDetail') },
        { name: t('dashboard.versionsModule'), allowed: true, detail: t('dashboard.versionsDetail') },
        { name: t('dashboard.walletModule'), allowed: true, detail: t('dashboard.walletDetail') },
        { name: t('dashboard.agentsModule'), allowed: true, detail: t('dashboard.agentsDetail') }
      ];
    case 'admin':
      return [
        { name: t('租户与组织管理'), allowed: false, detail: t('只读租户基本信息，不能转让租户') },
        { name: t('dashboard.routesModule'), allowed: true, detail: t('dashboard.routesDetail') },
        { name: t('dashboard.versionsModule'), allowed: true, detail: t('dashboard.versionsDetail') },
        { name: t('dashboard.walletModule'), allowed: false, detail: t('仅可查看余额与流水，不可直接手动调账') },
        { name: t('dashboard.agentsModule'), allowed: true, detail: t('dashboard.agentsDetail') }
      ];
    case 'operator':
      return [
        { name: t('租户与组织管理'), allowed: false, detail: t('无权限') },
        { name: t('dashboard.routesModule'), allowed: true, detail: t('dashboard.routesDetail') },
        { name: t('dashboard.versionsModule'), allowed: false, detail: t('仅可查看版本差异，无权触发发布与回滚') },
        { name: t('dashboard.walletModule'), allowed: false, detail: t('只读钱包流水与余额') },
        { name: t('dashboard.agentsModule'), allowed: false, detail: t('只读 Agent 时间线，无权触发紧急停用') }
      ];
    case 'viewer':
    default:
      return [
        { name: t('租户与组织管理'), allowed: false, detail: t('无权限') },
        { name: t('dashboard.routesModule'), allowed: false, detail: t('只读已有路由配置，禁止修改') },
        { name: t('dashboard.versionsModule'), allowed: false, detail: t('无权进入发布管理页面 (403 阻断)') },
        { name: t('dashboard.walletModule'), allowed: false, detail: t('只读钱包余额与流水') },
        { name: t('dashboard.agentsModule'), allowed: false, detail: t('只读 Agent 列表，无权修改状态') }
      ];
  }
});
</script>

<template>
  <div class="dashboard-page">
    <!-- Top Welcome Banner -->
    <el-card shadow="never" class="welcome-card">
      <div class="welcome-inner">
        <div>
          <h2 class="welcome-title">{{ $t('欢迎使用 AgentPay 管理控制台') }}</h2>
          <p class="welcome-desc">
            {{ $t('面向自主 AI Agent 的开源 HTTP 402 / x402 V2 支付基础设施与财务防火墙。') }}
          </p>
        </div>
        <div class="current-scope-tags">
          <el-tag size="large" effect="plain" type="primary">
            {{ $t('当前租户:') }} {{ authStore.currentTenant?.name || tenantStore.currentTenant?.name || 'AgentPay Default' }} ({{ authStore.currentTenant?.tenant_id || tenantStore.currentTenantId }})
          </el-tag>
          <el-tag size="large" effect="plain" type="success" style="margin-left: 10px">
            {{ $t('当前项目:') }} {{ tenantStore.currentProject?.name || $t('默认项目') }} ({{ tenantStore.currentProjectId || 'prj_default' }})
          </el-tag>
        </div>
      </div>
    </el-card>

    <!-- Metrics Grid -->
    <el-row :gutter="16" class="metrics-row">
      <el-col :span="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-title">{{ t('dashboard.activeRoutes') }}</div>
          <div class="metric-val">12 <span class="metric-unit">{{ t('dashboard.routesUnit') }}</span></div>
          <div class="metric-sub">{{ t('dashboard.gatewayRunning') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-title">{{ t('dashboard.projectBudgetCap') }}</div>
          <div class="metric-val">{{ tenantStore.currencySymbol }}5,000 <span class="metric-unit">{{ tenantStore.currentCurrency }}</span></div>
          <div class="metric-sub">{{ t('dashboard.budgetUsed') }} {{ tenantStore.currencySymbol }}320.50 {{ tenantStore.currentCurrency }} (6.4%)</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-title">{{ t('dashboard.publishedVersion') }}</div>
          <div class="metric-val">v2026.10.18-04</div>
          <div class="metric-sub">{{ t('dashboard.immutableSnapshot') }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-title">{{ t('dashboard.registeredAgents') }}</div>
          <div class="metric-val">8 <span class="metric-unit">{{ t('dashboard.agentsUnit') }}</span></div>
          <div class="metric-sub">{{ t('dashboard.behaviorAudit') }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Role RBAC Authorization Overview -->
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="14">
        <el-card shadow="never">
          <template #header>
            <div class="card-header">
              <span>{{ t('dashboard.rbacMatrix') }}</span>
              <el-tag size="small" type="warning">
                {{ t('layout.role') }}: {{ authStore.currentRole?.toUpperCase() }}
              </el-tag>
            </div>
          </template>
          <el-table :data="rolePermissionsList" style="width: 100%">
            <el-table-column prop="name" :label="t('dashboard.module')" width="220" />
            <el-table-column :label="t('dashboard.permission')" width="100">
              <template #default="{ row }">
                <el-tag :type="row.allowed ? 'success' : 'danger'" size="small">
                  {{ row.allowed ? t('dashboard.allowed') : t('dashboard.forbidden') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="detail" :label="t('dashboard.permissionDetail')" />
          </el-table>
        </el-card>
      </el-col>

      <el-col :span="10">
        <el-card shadow="never">
          <template #header>
            <span>{{ t('dashboard.quickNav') }}</span>
          </template>
          <div class="quick-nav-list">
            <router-link to="/members" class="nav-item">
              <el-icon><User /></el-icon>
              <div>
                <div class="nav-title">{{ t('menu.members') }}</div>
                <div class="nav-desc">{{ $t('查看团队成员、分配 RBAC 角色与开户授权') }}</div>
              </div>
            </router-link>

            <router-link to="/routes" class="nav-item">
              <el-icon><Connection /></el-icon>
              <div>
                <div class="nav-title">{{ t('menu.routes') }}</div>
                <div class="nav-desc">{{ $t('配置 API 端点、上游目标与 x402 价格') }}</div>
              </div>
            </router-link>

            <PermissionGate :roles="['owner', 'admin']">
              <router-link to="/versions" class="nav-item">
                <el-icon><Files /></el-icon>
                <div>
                  <div class="nav-title">{{ t('menu.versions') }}</div>
                  <div class="nav-desc">{{ $t('生成配置快照，安全推送到网关或一键回退') }}</div>
                </div>
              </router-link>
              <template #fallback>
                <div class="nav-item disabled">
                  <el-icon><Lock /></el-icon>
                  <div>
                    <div class="nav-title">{{ t('menu.versions') }} [{{ t('dashboard.locked') }}]</div>
                    <div class="nav-desc">{{ $t('当前角色') }} ({{ authStore.currentRole }}) {{ $t('无权执行发布') }}</div>
                  </div>
                </div>
              </template>
            </PermissionGate>

            <router-link to="/wallet" class="nav-item">
              <el-icon><Wallet /></el-icon>
              <div>
                <div class="nav-title">{{ t('menu.wallet') }}</div>
                <div class="nav-desc">{{ $t('可用/冻结余额展示，多维度流水记账') }}</div>
              </div>
            </router-link>

            <router-link to="/agents" class="nav-item">
              <el-icon><Cpu /></el-icon>
              <div>
                <div class="nav-title">{{ t('menu.agents') }}</div>
                <div class="nav-desc">{{ $t('会话级请求下钻，紧急失控 Kill Switch') }}</div>
              </div>
            </router-link>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.welcome-card {
  margin-bottom: 16px;
  background: #ffffff;
  border-radius: 8px;
}
.welcome-inner {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.welcome-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 6px 0;
  color: #303133;
}
.welcome-desc {
  font-size: 13px;
  color: #909399;
  margin: 0;
}
.metrics-row {
  margin-bottom: 0;
}
.metric-card {
  border-radius: 8px;
}
.metric-title {
  font-size: 13px;
  color: #909399;
  margin-bottom: 8px;
}
.metric-val {
  font-size: 24px;
  font-weight: 700;
  color: #303133;
  line-height: 1.2;
}
.metric-unit {
  font-size: 14px;
  font-weight: 400;
  color: #909399;
  margin-left: 4px;
}
.metric-sub {
  font-size: 12px;
  color: #67c23a;
  margin-top: 8px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}
.quick-nav-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid #e4e7ed;
  text-decoration: none;
  color: inherit;
  transition: all 0.2s;
}
.nav-item:hover {
  background-color: #f5f7fa;
  border-color: #dcdfe6;
}
.nav-item.disabled {
  opacity: 0.6;
  cursor: not-allowed;
  background-color: #f5f7fa;
}
.nav-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}
.nav-desc {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}
</style>
