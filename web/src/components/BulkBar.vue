<script setup lang="ts">
import { NButton } from 'naive-ui'
import Icon from './Icon.vue'
import { fmtCount, fmtSize } from '../format'

defineProps<{ count: number; size: number; unit: string }>()
const emit = defineEmits<{ (e: 'clear'): void; (e: 'delete'): void }>()
</script>

<template>
  <!-- 勾选后显示在表格上方的批量操作条（不占用页面底部、不引起外部滚动） -->
  <div v-if="count" class="bulk" role="region" aria-label="批量操作">
    <Icon name="check" :size="18" />
    <span class="info">
      已选 <b>{{ fmtCount(count) }}</b> {{ unit }}<template v-if="size"> · 共 <b>{{ fmtSize(size) }}</b></template>
    </span>
    <span class="spacer" />
    <NButton size="small" quaternary @click="emit('clear')">取消选择</NButton>
    <NButton size="small" type="error" @click="emit('delete')">
      <template #icon><Icon name="trash" /></template>
      删除所选
    </NButton>
  </div>
</template>

<style scoped>
.bulk {
  display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 8px 20px;
  border-bottom: 1px solid var(--border); background: var(--primary-row); color: var(--primary-ink); font-size: 14px;
}
.info { color: var(--ink); }
@media (max-width: 640px) { .bulk { padding: 8px 14px; } }
</style>
