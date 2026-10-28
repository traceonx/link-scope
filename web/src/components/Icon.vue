<script setup lang="ts">
import { computed } from 'vue'
import { iconPaths, type IconName } from '../icons'

// 简单的线性图标，避免额外依赖
const props = defineProps<{ name: string; size?: number }>()
const ds = computed(() => iconPaths[props.name as IconName] ?? [])
</script>

<template>
  <svg
    :width="size ?? 16"
    :height="size ?? 16"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    class="icon"
  >
    <path v-for="(d, i) in ds" :key="i" :d="d" />
  </svg>
</template>
