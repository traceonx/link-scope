<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NCheckbox, NEmpty, NModal, NSpin, useDialog, useMessage } from 'naive-ui'
import Icon from './Icon.vue'
import { api } from '../api'
import { fmtSize, fmtTime } from '../format'
import type { TrashEntry } from '../types'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'changed'): void }>()
const message = useMessage()
const dialog = useDialog()

const items = ref<TrashEntry[]>([])
const days = ref(30)
const loading = ref(false)
const busy = ref(false)
const checked = ref<Set<string>>(new Set())

async function load() {
  loading.value = true
  try {
    const r = await api.trash()
    items.value = r.items
    days.value = r.days
    checked.value = new Set([...checked.value].filter((id) => r.items.some((x) => x.id === id)))
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}
watch(
  () => props.show,
  (v) => {
    if (v) {
      checked.value = new Set()
      load()
    }
  },
)

const total = computed(() => items.value.reduce((n, x) => n + x.size, 0))
const allChecked = computed(() => items.value.length > 0 && checked.value.size === items.value.length)
function toggle(id: string, v: boolean) {
  const s = new Set(checked.value)
  if (v) s.add(id)
  else s.delete(id)
  checked.value = s
}
function toggleAll(v: boolean) {
  checked.value = v ? new Set(items.value.map((x) => x.id)) : new Set()
}

async function restore(ids: string[]) {
  busy.value = true
  try {
    const r = await api.trashRestore(ids)
    const failed = r.results.filter((x) => !x.ok)
    if (failed.length) message.warning(`已恢复 ${r.restored} 项；${failed.length} 项失败：${failed[0].error}`)
    else message.success(`已恢复 ${r.restored} 项`)
    emit('changed')
    await load()
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    busy.value = false
  }
}

function purge(ids: string[], all = false) {
  const size = all ? total.value : items.value.filter((x) => ids.includes(x.id)).reduce((n, x) => n + x.size, 0)
  dialog.warning({
    title: all ? '清空回收站' : '永久删除',
    content: `将永久删除 ${all ? items.value.length : ids.length} 项（${fmtSize(size)}），无法恢复。`,
    positiveText: '永久删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      busy.value = true
      try {
        await api.trashPurge(ids, all)
        message.success('已永久删除')
        await load()
      } catch (e) {
        message.error((e as Error).message)
      } finally {
        busy.value = false
      }
    },
  })
}

const kindLabel = { file: '文件', dir: '文件夹', symlink: '软链接' } as const
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="回收站"
    style="width: min(720px, calc(100vw - 24px))"
    :bordered="false"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <div class="trash">
      <div class="head">
        <NCheckbox :checked="allChecked" :disabled="!items.length" @update:checked="toggleAll">全选</NCheckbox>
        <span class="muted small">共 {{ items.length }} 项 · {{ fmtSize(total) }} · {{ days }} 天后自动清除</span>
      </div>
      <NSpin :show="loading">
        <div class="list">
          <div v-for="it in items" :key="it.id" class="row">
            <NCheckbox
              :checked="checked.has(it.id)"
              :aria-label="`选择 ${it.name}`"
              @update:checked="(v: boolean) => toggle(it.id, v)"
            />
            <div class="body">
              <div class="name">
                <span class="badge badge-gray">{{ kindLabel[it.kind] }}</span>
                <span>{{ it.name }}</span>
              </div>
              <span class="mono path">{{ it.path }}</span>
              <span class="muted small">
                {{ fmtSize(it.size) }}<template v-if="it.kind === 'dir'"> · {{ it.files }} 个文件</template>
                · 删除于 {{ fmtTime(it.deleted_at) }} · {{ fmtTime(it.expires_at) }} 清除
              </span>
            </div>
            <NButton size="small" quaternary :disabled="busy" :aria-label="`恢复 ${it.name}`" @click="restore([it.id])">
              <template #icon><Icon name="restore" /></template>
              恢复
            </NButton>
          </div>
          <NEmpty v-if="!loading && !items.length" description="回收站是空的" style="padding: 40px 0" />
        </div>
      </NSpin>
    </div>
    <template #footer>
      <div class="foot">
        <NButton :disabled="busy || !items.length" @click="purge([], true)">清空回收站</NButton>
        <span class="spacer" />
        <NButton :disabled="busy || !checked.size" @click="purge([...checked])">永久删除所选</NButton>
        <NButton type="primary" :disabled="busy || !checked.size" @click="restore([...checked])">恢复所选</NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.trash { display: flex; flex-direction: column; gap: 10px; }
.head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; }
.small { font-size: 12px; }
.list { max-height: min(460px, 60vh); overflow-y: auto; border: 1px solid var(--border); border-radius: 8px; min-height: 80px; }
.row { display: flex; align-items: center; gap: 12px; padding: 10px 12px; border-bottom: 1px solid var(--border); }
.row:last-child { border-bottom: 0; }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
.name { display: flex; align-items: center; gap: 6px; font-size: 14px; font-weight: 500; word-break: break-all; }
.path { font-size: 12px; color: var(--ink-2); word-break: break-all; }
.foot { display: flex; flex-wrap: wrap; gap: 8px; }
.spacer { flex: 1; }
</style>
