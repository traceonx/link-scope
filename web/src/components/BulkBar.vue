<script setup lang="ts">
import { NButton } from 'naive-ui'
import Icon from './Icon.vue'
import { fmtCount, fmtSize } from '../format'

defineProps<{ count: number; size: number; unit: string }>()
const emit = defineEmits<{ (e: 'clear'): void; (e: 'delete'): void }>()
</script>

<template>
  <!-- 悬浮在内容区底部的批量操作卡片（随页面滚动吸底） -->
  <Transition name="bulk">
    <div v-if="count" class="bulk card" role="region" aria-label="批量操作">
      <span class="avatar primary"><Icon name="check" :size="20" /></span>
      <span class="info">
        已选 <b>{{ fmtCount(count) }}</b> {{ unit }}<template v-if="size"> · 共 <b>{{ fmtSize(size) }}</b></template>
      </span>
      <span class="spacer" />
      <NButton quaternary @click="emit('clear')">取消选择</NButton>
      <NButton type="error" @click="emit('delete')">
        <template #icon><Icon name="trash" /></template>
        删除所选
      </NButton>
    </div>
  </Transition>
</template>

<style scoped>
.bulk {
  position: sticky; bottom: 16px; z-index: 25;
  display: flex; flex-wrap: wrap; align-items: center; gap: 8px 10px; padding: 10px 14px;
  box-shadow: var(--shadow-lg); border: 1px solid var(--border);
}
.avatar { width: 34px; height: 34px; }
.info { font-size: 14px; }
.bulk-enter-active, .bulk-leave-active { transition: transform 0.15s ease, opacity 0.15s ease; }
.bulk-enter-from, .bulk-leave-to { transform: translateY(20px); opacity: 0; }
@media (prefers-reduced-motion: reduce) {
  .bulk-enter-active, .bulk-leave-active { transition: none; }
}
</style>
