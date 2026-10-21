<script setup lang="ts">
import { useRouter } from 'vue-router';

const router = useRouter();
</script>

<template>
  <div class="error-page">
    <el-result
      icon="error"
      :title="$t('404 页面未找到')"
      :sub-title="$t('您访问的页面不存在或已被移除。')"
    >
      <template #extra>
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
</style>
