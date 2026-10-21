<script setup lang="ts">
import { useI18n } from '@/i18n';

const { elLocale } = useI18n();
</script>

<template>
  <el-config-provider :locale="elLocale">
    <router-view />
  </el-config-provider>
</template>

<style>
html,
body,
#app {
  margin: 0;
  padding: 0;
  width: 100%;
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
</style>
