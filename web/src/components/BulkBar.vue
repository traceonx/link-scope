<script setup lang="ts">
import { NButton } from 'naive-ui'
import Icon from './Icon.vue'
import { fmtCount, fmtSize } from '../format'

defineProps<{ count: number; size: number; unit: string }>()
const emit = defineEmits<{ (e: 'clear'): void; (e: 'delete'): void }>()
</script>

<template>
  <Transition name="bulk">
    <div v-if="count" class="bulk" role="region" aria-label="批量操作">
      <div class="wrap bulk-in">
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
    </div>
  </Transition>
</template>

<style scoped>
.bulk {
  position: fixed; left: 0; right: 0; bottom: 0; z-index: 30;
  background: var(--surface); border-top: 1px solid var(--border-strong);
  box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.08);
}
.bulk-in { display: flex; align-items: center; gap: 8px; min-height: 60px; padding-top: 8px; padding-bottom: 8px; }
.info { font-size: 14px; }
.spacer { flex: 1; }
.bulk-enter-active, .bulk-leave-active { transition: transform 0.15s ease; }
.bulk-enter-from, .bulk-leave-to { transform: translateY(100%); }
@media (prefers-reduced-motion: reduce) {
  .bulk-enter-active, .bulk-leave-active { transition: none; }
}
</style>
