<script setup lang="ts">
import { computed } from 'vue';
import { useAuthStore } from '@/stores/auth';
import type { UserRole } from '@/types/auth';

const props = defineProps<{
  roles?: UserRole[];
}>();

const authStore = useAuthStore();
const hasPermission = computed(() => {
  if (!props.roles || props.roles.length === 0) return true;
  return authStore.hasRole(props.roles);
});
</script>

<template>
  <slot v-if="hasPermission" />
  <slot v-else name="fallback" />
</template>
