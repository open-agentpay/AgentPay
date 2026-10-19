<script setup lang="ts">
import { computed } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import PermissionGate from '@/components/PermissionGate.vue';

const authStore = useAuthStore();
const tenantStore = useTenantStore();

const rolePermissionsList = computed(() => {
  const role = authStore.currentRole;
  switch (role) {
    case 'owner':
      return [
        { name: '租户与组织管理', allowed: true, detail: '创建项目、邀请成员、转让所有权' },
        { name: '网关路由与策略配置 (T4.2/T4.3)', allowed: true, detail: '创建/编辑路由、上游地址、定价、策略' },
        { name: '版本发布与即时回滚 (T4.4)', allowed: true, detail: '生成配置快照、发布到生产网关、一键回滚' },
        { name: '钱包调账与充值 (T4.6)', allowed: true, detail: '调整信用额度、查看内部流水、主链出入金' },
        { name: 'Agent 熔断与 Kill Switch (T4.7)', allowed: true, detail: '一键熔断失控 Agent、查看全量审计时间线' }
      ];
    case 'admin':
      return [
        { name: '租户与组织管理', allowed: false, detail: '只读租户基本信息，不能转让租户' },
        { name: '网关路由与策略配置 (T4.2/T4.3)', allowed: true, detail: '创建/编辑路由、上游地址、定价、策略' },
        { name: '版本发布与即时回滚 (T4.4)', allowed: true, detail: '生成配置快照、发布到生产网关、一键回滚' },
        { name: '钱包调账与充值 (T4.6)', allowed: false, detail: '仅可查看余额与流水，不可直接手动调账' },
        { name: 'Agent 熔断与 Kill Switch (T4.7)', allowed: true, detail: '一键熔断失控 Agent、查看全量审计时间线' }
      ];
    case 'operator':
      return [
        { name: '租户与组织管理', allowed: false, detail: '无权限' },
        { name: '网关路由与策略配置 (T4.2/T4.3)', allowed: true, detail: '编辑草稿配置，但不可直接生效发布' },
        { name: '版本发布与即时回滚 (T4.4)', allowed: false, detail: '仅可查看版本差异，无权触发发布与回滚' },
        { name: '钱包调账与充值 (T4.6)', allowed: false, detail: '只读钱包流水与余额' },
        { name: 'Agent 熔断与 Kill Switch (T4.7)', allowed: false, detail: '只读 Agent 时间线，无权触发紧急停用' }
      ];
    case 'viewer':
    default:
      return [
        { name: '租户与组织管理', allowed: false, detail: '无权限' },
        { name: '网关路由与策略配置 (T4.2/T4.3)', allowed: false, detail: '只读已有路由配置，禁止修改' },
        { name: '版本发布与即时回滚 (T4.4)', allowed: false, detail: '无权进入发布管理页面 (403 阻断)' },
        { name: '钱包调账与充值 (T4.6)', allowed: false, detail: '只读钱包余额与流水' },
        { name: 'Agent 熔断与 Kill Switch (T4.7)', allowed: false, detail: '只读 Agent 列表，无权修改状态' }
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
          <h2 class="welcome-title">欢迎使用 AgentPay 管理控制台</h2>
          <p class="welcome-desc">
            面向自主 AI Agent 的开源 HTTP 402 / x402 V2 支付基础设施与财务防火墙。
          </p>
        </div>
        <div class="current-scope-tags">
          <el-tag size="large" effect="plain" type="primary">
            当前租户: {{ authStore.currentTenant?.name || tenantStore.currentTenant?.name || 'AgentPay Default' }} ({{ authStore.currentTenant?.tenant_id || tenantStore.currentTenantId }})
          </el-tag>
          <el-tag size="large" effect="plain" type="success" style="margin-left: 10px">
            当前项目: {{ tenantStore.currentProject?.name || '默认项目' }} ({{ tenantStore.currentProjectId || 'prj_default' }})
          </el-tag>
        </div>
      </div>
    </el-card>

    <!-- Metrics Grid -->
    <el-row :gutter="16" class="metrics-row">
      <el-col :span="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-title">生效代理路由</div>
          <div class="metric-val">12 <span class="metric-unit">条</span></div>
          <div class="metric-sub">对应任务: T4.2</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-title">项目预算上限</div>
          <div class="metric-val">5,000 <span class="metric-unit">USDC</span></div>
          <div class="metric-sub">已用 320.50 USDC (6.4%)</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-title">已发布配置版本</div>
          <div class="metric-val">v2026.10.18-04</div>
          <div class="metric-sub">对应任务: T4.4 (快照不可变)</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-title">注册 AI Agent 实例</div>
          <div class="metric-val">8 <span class="metric-unit">个</span></div>
          <div class="metric-sub">对应任务: T4.7 (行为审计)</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Role RBAC Authorization Overview -->
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="14">
        <el-card shadow="never">
          <template #header>
            <div class="card-header">
              <span>当前会话 RBAC 权限矩阵 (T4.1 验收基准)</span>
              <el-tag size="small" type="warning">
                角色: {{ authStore.currentRole?.toUpperCase() }}
              </el-tag>
            </div>
          </template>
          <el-table :data="rolePermissionsList" style="width: 100%">
            <el-table-column prop="name" label="功能模块" width="220" />
            <el-table-column label="执行授权" width="100">
              <template #default="{ row }">
                <el-tag :type="row.allowed ? 'success' : 'danger'" size="small">
                  {{ row.allowed ? '允许' : '受限 / 拒绝' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="detail" label="权限说明与边界" />
          </el-table>
        </el-card>
      </el-col>

      <el-col :span="10">
        <el-card shadow="never">
          <template #header>
            <span>快速导航与功能入口</span>
          </template>
          <div class="quick-nav-list">
            <router-link to="/members" class="nav-item">
              <el-icon><User /></el-icon>
              <div>
                <div class="nav-title">组织团队与成员权限 (T4.1)</div>
                <div class="nav-desc">查看团队成员、分配 RBAC 角色与开户授权</div>
              </div>
            </router-link>

            <router-link to="/routes" class="nav-item">
              <el-icon><Connection /></el-icon>
              <div>
                <div class="nav-title">路由与上游管理 (T4.2)</div>
                <div class="nav-desc">配置 API 端点、上游目标与 x402 价格</div>
              </div>
            </router-link>

            <PermissionGate :roles="['owner', 'admin']">
              <router-link to="/versions" class="nav-item">
                <el-icon><Files /></el-icon>
                <div>
                  <div class="nav-title">配置发布与回滚 (T4.4)</div>
                  <div class="nav-desc">生成配置快照，安全推送到网关或一键回退</div>
                </div>
              </router-link>
              <template #fallback>
                <div class="nav-item disabled">
                  <el-icon><Lock /></el-icon>
                  <div>
                    <div class="nav-title">配置发布与回滚 (T4.4) [已锁定]</div>
                    <div class="nav-desc">当前角色 ({{ authStore.currentRole }}) 无权执行发布</div>
                  </div>
                </div>
              </template>
            </PermissionGate>

            <router-link to="/wallet" class="nav-item">
              <el-icon><Wallet /></el-icon>
              <div>
                <div class="nav-title">虚拟钱包与对账 (T4.6)</div>
                <div class="nav-desc">可用/冻结余额展示，多维度流水记账</div>
              </div>
            </router-link>

            <router-link to="/agents" class="nav-item">
              <el-icon><Cpu /></el-icon>
              <div>
                <div class="nav-title">Agent 管理与审计时间线 (T4.7)</div>
                <div class="nav-desc">会话级请求下钻，紧急失控 Kill Switch</div>
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
  margin: 0;
  font-size: 20px;
  color: #1a1a1a;
}

.welcome-desc {
  margin: 6px 0 0;
  color: #606266;
  font-size: 13px;
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
}

.metric-val {
  font-size: 26px;
  font-weight: 700;
  color: #303133;
  margin: 8px 0 4px;
}

.metric-unit {
  font-size: 14px;
  font-weight: normal;
  color: #606266;
}

.metric-sub {
  font-size: 12px;
  color: #909399;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
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
  padding: 12px 14px;
  background-color: #f8f9fa;
  border-radius: 6px;
  text-decoration: none;
  color: inherit;
  border: 1px solid #ebeef5;
  transition: all 0.2s;
}

.nav-item:not(.disabled):hover {
  border-color: #409eff;
  background-color: #ecf5ff;
}

.nav-item.disabled {
  opacity: 0.6;
  cursor: not-allowed;
  background-color: #f2f3f5;
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
