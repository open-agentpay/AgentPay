<script setup lang="ts">
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';

const router = useRouter();
const authStore = useAuthStore();
</script>

<template>
  <div class="error-page">
    <el-result
      icon="warning"
      :title="$t('403 访问被拒绝 (Forbidden)')"
      :sub-title="$t('您当前的角色权限不足以访问此模块或执行该操作。')"
    >
      <template #extra>
        <div class="role-hint">
          {{ $t('当前登录角色:') }} <b>{{ authStore.currentRole?.toUpperCase() }}</b>
        </div>
        <el-button type="primary" @click="router.push('/dashboard')">{{ $t('返回工作台总览') }}</el-button>
      </template>
    </el-result>
  </div>
</template>

<style scoped>
.error-page {
  height: 80vh;
  display: flex;
  justify-content: center;
  align-items: center;
}
.role-hint {
  margin-bottom: 16px;
  color: #606266;
  font-size: 14px;
}
</style>
