<script setup lang="ts">
import { ref, watch } from 'vue'
import { NModal, NButton, NSpin, NEmpty } from 'naive-ui'
import Icon from './Icon.vue'
import { api } from '../api'
import type { BrowseResult } from '../types'

const props = defineProps<{ show: boolean; title: string; start?: string }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'select', path: string): void }>()

const data = ref<BrowseResult>({ path: '', parent: null, dirs: [] })
const loading = ref(false)
const error = ref('')

async function open(path: string) {
  loading.value = true
  error.value = ''
  try {
    data.value = await api.browse(path)
  } catch (e) {
    error.value = (e as Error).message
    if (path) {
      // 起始目录不可用时回到根目录列表
      data.value = await api.browse('').catch(() => data.value)
    }
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (v) => {
    if (v) open(props.start ?? '')
  },
)

function choose() {
  if (!data.value.path) return
  emit('select', data.value.path)
  emit('update:show', false)
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="title"
    style="width: min(560px, calc(100vw - 24px))"
    :bordered="false"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <div class="picker">
      <div class="picker-bar">
        <NButton
          size="small"
          quaternary
          :disabled="data.parent === null"
          aria-label="上一级"
          @click="open(data.parent ?? '')"
        >
          <template #icon><Icon name="up" /></template>
        </NButton>
        <span class="picker-path mono">{{ data.path || '选择起始位置' }}</span>
      </div>
      <div v-if="error" class="notice notice-err">{{ error }}</div>
      <NSpin :show="loading">
        <div class="picker-list">
          <button v-for="d in data.dirs" :key="d.path" type="button" class="picker-item" @click="open(d.path)">
            <Icon name="folder" />
            <span class="picker-name">{{ d.name }}</span>
            <Icon name="chevron" :size="14" />
          </button>
          <NEmpty v-if="!loading && !data.dirs.length" description="没有子文件夹" style="padding: 32px 0" />
        </div>
      </NSpin>
    </div>
    <template #footer>
      <div class="picker-foot">
        <NButton @click="emit('update:show', false)">取消</NButton>
        <NButton type="primary" :disabled="!data.path" @click="choose">选择此文件夹</NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.picker { display: flex; flex-direction: column; gap: 10px; }
.picker-bar { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-radius: 8px; background: var(--soft); }
.picker-path { font-size: 13px; word-break: break-all; }
.picker-list { height: min(360px, 50vh); overflow-y: auto; border: 1px solid var(--border); border-radius: 8px; }
.picker-item {
  width: 100%; display: flex; align-items: center; gap: 10px; padding: 0 12px; min-height: 44px;
  border: 0; border-bottom: 1px solid var(--border); background: transparent; color: var(--ink);
  font: inherit; font-size: 14px; text-align: left; cursor: pointer;
}
.picker-item:hover { background: var(--soft); }
.picker-item:focus-visible { outline: 2px solid var(--blue); outline-offset: -2px; }
.picker-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.picker-foot { display: flex; justify-content: flex-end; gap: 8px; }
</style>
