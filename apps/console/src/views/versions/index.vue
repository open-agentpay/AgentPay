<script setup lang="ts">
import { ref, watch, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useTenantStore } from '@/stores/tenant';
import { configApi } from '@/api/client';
import type { ValidationResult, ConfigDraftDiff, ConfigVersion } from '@/types/config';
import ValidationDiagnosticModal from '@/components/ValidationDiagnosticModal.vue';
import DraftDiffDrawer from '@/components/DraftDiffDrawer.vue';
import { t } from '@/i18n';

const authStore = useAuthStore();
const tenantStore = useTenantStore();

const loading = ref(false);
const versions = ref<ConfigVersion[]>([]);

const validating = ref(false);
const lastValidationResult = ref<ValidationResult | null>(null);
const validationModalVisible = ref(false);

const diffLoading = ref(false);
const draftDiff = ref<ConfigDraftDiff | null>(null);
const diffDrawerVisible = ref(false);

// Load Versions from API
async function loadVersions() {
  if (!tenantStore.currentTenantId) return;
  loading.value = true;
  try {
    const list = await configApi.getVersions(tenantStore.currentTenantId);
    versions.value = list;
  } catch (err: unknown) {
    const msg = err instanceof Error ? t(err.message) : t('加载版本历史失败');
    ElMessage.error(msg);
  } finally {
    loading.value = false;
  }
}

// Pre-publication check: Enforce static validation matrix and Publish
async function handlePublishClick() {
  if (!tenantStore.currentTenantId) return;

  validating.value = true;
  try {
    const result = await configApi.validateDraft(tenantStore.currentTenantId);
    lastValidationResult.value = result;

    if (!result.valid) {
      ElMessage.error(`${t('草稿组合校验未通过')} (${t('发现')} ${result.summary.error_count} ${t('处错误')})，${t('系统已阻止发布！')}`);
      validationModalVisible.value = true;
      return;
    }

    // Validation succeeded -> Prompt for release summary
    const { value: changeSummary } = await ElMessageBox.prompt(
      t('配置草稿已通过完整静态校验矩阵。请输入本次发布变更摘要：'),
      t('发布配置快照版本'),
      {
        confirmButtonText: t('确认发布并热加载'),
        cancelButtonText: t('取消'),
        inputPlaceholder: t('例如：添加金融资讯路由并调整限流策略'),
        inputValue: t('发布路由与策略配置快照')
      }
    );

    const newVersion = await configApi.publishConfig(tenantStore.currentTenantId, {
      change_summary: changeSummary
    });

    ElMessage.success(`${t('版本')} v${newVersion.version_number} ${t('发布成功，不可变快照已热加载至网关！')}`);
    await loadVersions();
  } catch (err: unknown) {
    if (err !== 'cancel') {
      const msg = err instanceof Error ? t(err.message) : t('发布流程失败');
      ElMessage.error(msg);
    }
  } finally {
    validating.value = false;
  }
}

// Rollback to specific target version
async function handleRollbackClick(target: ConfigVersion) {
  if (!tenantStore.currentTenantId) return;

  try {
    const { value: reason } = await ElMessageBox.prompt(
      `${t('确定要将网关配置秒级回滚到版本')} v${target.version_number} (${target.version_id}) ${t('吗？回滚将自增生成新版本并瞬间原子切换生效。请输入回滚原因：')}`,
      t('秒级一键回滚确认'),
      {
        confirmButtonText: t('立即回滚'),
        cancelButtonText: t('取消'),
        type: 'warning',
        inputPlaceholder: t('例如：线上上游接口异常应急处置'),
        inputValue: t('故障应急回滚')
      }
    );

    const restoredVer = await configApi.rollbackConfig(tenantStore.currentTenantId, {
      target_version_id: target.version_id,
      reason: reason || t('故障应急回滚')
    });

    ElMessage.success(`${t('回滚成功！已生成生效版本')} v${restoredVer.version_number}，${t('指纹与历史版本严格一致。')}`);
    await loadVersions();
  } catch (err: unknown) {
    if (err !== 'cancel') {
      const msg = err instanceof Error ? t(err.message) : t('回滚失败');
      ElMessage.error(msg);
    }
  }
}

async function handleViewDiff() {
  if (!tenantStore.currentTenantId) return;
  diffLoading.value = true;
  try {
    const diff = await configApi.getDraftDiff(tenantStore.currentTenantId);
    draftDiff.value = diff;
    diffDrawerVisible.value = true;
  } catch (err: unknown) {
    const msg = err instanceof Error ? t(err.message) : t('获取版本差异失败');
    ElMessage.error(msg);
  } finally {
    diffLoading.value = false;
  }
}

async function handleViewVersionDiff(ver: ConfigVersion) {
  if (!tenantStore.currentTenantId) return;
  diffLoading.value = true;
  try {
    const diff = await configApi.getVersionDiff(tenantStore.currentTenantId, undefined, ver.version_id);
    draftDiff.value = diff;
    diffDrawerVisible.value = true;
  } catch (err: unknown) {
    const msg = err instanceof Error ? t(err.message) : t('获取版本差异失败');
    ElMessage.error(msg);
  } finally {
    diffLoading.value = false;
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text);
  ElMessage.success(t('已复制到剪贴板'));
}

function formatDate(val: string | number) {
  if (!val) return '-';
  if (typeof val === 'number') {
    return new Date(val * 1000).toLocaleString();
  }
  return new Date(val).toLocaleString();
}

watch(
  () => tenantStore.currentTenantId,
  () => {
    loadVersions();
  }
);

onMounted(() => {
  loadVersions();
});
</script>

<template>
  <div class="versions-page">
    <el-card shadow="never">
      <template #header>
        <div class="header-box">
          <div>
            <span class="title">{{ $t('配置快照发布与版本回滚') }}</span>
            <span class="subtitle">{{ $t('租户:') }} {{ tenantStore.currentTenant?.name }}</span>
          </div>
          <div class="header-actions">
            <el-button
              type="info"
              plain
              icon="Operation"
              :loading="diffLoading"
              @click="handleViewDiff"
            >
              {{ $t('比对草稿差异') }}
            </el-button>
            <el-button
              type="success"
              icon="Upload"
              :loading="validating"
              :disabled="!authStore.hasRole(['owner', 'admin'])"
              @click="handlePublishClick"
            >
              {{ $t('发布当前草稿为新版本') }}
            </el-button>
          </div>
        </div>
      </template>

      <el-alert
        :title="$t('发布门禁与不可变快照约束: 发布前强制执行静态组合规则校验。发布成功后生成确定性 SHA-256 内容指纹并写入 ap_config_version，网关执行零中断 RCU 原子替换。支持随时一键回滚。')"
        type="info"
        show-icon
        :closable="false"
        style="margin-bottom: 16px"
      />

      <el-table v-loading="loading" :data="versions" style="width: 100%" stripe>
        <el-table-column prop="version_number" :label="$t('版本号')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'" effect="plain">
              v{{ row.version_number }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="version_id" :label="$t('版本快照 ID')" width="160">
          <template #default="{ row }">
            <code class="code-id">{{ row.version_id }}</code>
          </template>
        </el-table-column>

        <el-table-column prop="change_summary" :label="$t('发布变更描述')" min-width="240" show-overflow-tooltip />

        <el-table-column prop="snapshot_hash" :label="$t('快照哈希 (SHA-256)')" width="180">
          <template #default="{ row }">
            <div class="hash-cell" v-if="row.snapshot_hash">
              <span class="code-hash">{{ row.snapshot_hash.substring(0, 16) }}...</span>
              <el-button
                link
                icon="CopyDocument"
                size="small"
                @click="copyToClipboard(row.snapshot_hash)"
                :title="$t('复制完整哈希')"
              />
            </div>
            <span v-else>-</span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('路由/策略数')" width="140">
          <template #default="{ row }">
            <span>{{ row.routes_count ?? 0 }} {{ $t('路由') }} / {{ row.policies_count ?? 0 }} {{ $t('策略') }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="publisher_email" :label="$t('发布人')" width="180">
          <template #default="{ row }">
            <span>{{ row.publisher_email || row.published_by }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="created_at" :label="$t('发布时间')" width="180">
          <template #default="{ row }">
            <span>{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>

        <el-table-column :label="$t('生效状态')" width="110">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'" size="small">
              {{ row.is_active ? $t('正在生效') : $t('历史版本') }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="$t('操作')" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="handleViewVersionDiff(row)">
              {{ $t('查看差异') }}
            </el-button>
            <el-button
              v-if="!row.is_active"
              link
              type="warning"
              size="small"
              :disabled="!authStore.hasRole(['owner', 'admin'])"
              @click="handleRollbackClick(row)"
            >
              {{ $t('回滚至此版本') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

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
.header-box {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
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
.code-id {
  font-family: monospace;
  font-size: 12px;
  color: #409eff;
}
.hash-cell {
  display: flex;
  align-items: center;
  gap: 4px;
}
.code-hash {
  font-family: monospace;
  font-size: 12px;
  color: #67c23a;
}
</style>
