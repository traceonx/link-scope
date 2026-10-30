<script setup lang="ts">
import { computed } from 'vue'
import { iconPaths, type IconName } from '../icons'

// MDI 图标（实心路径，跟随文字颜色）
const props = defineProps<{ name: string; size?: number }>()
const d = computed(() => iconPaths[props.name as IconName] ?? '')
</script>

<template>
  <svg
    :width="size ?? 18"
    :height="size ?? 18"
    viewBox="0 0 24 24"
    fill="currentColor"
    aria-hidden="true"
    class="icon"
  >
    <path :d="d" />
  </svg>
</template>
