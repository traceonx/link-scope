<script setup lang="ts">
import { computed, onActivated, ref, watch } from 'vue'
import { NButton, NCheckbox, NEmpty, NSpin, useDialog, useMessage } from 'naive-ui'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import { fmtSize, fmtTime } from '../format'
import { canDelete, cfg, changedKey, loadResult } from '../store'
import type { TrashEntry } from '../types'

const message = useMessage()
const dialog = useDialog()

const items = ref<TrashEntry[]>([])
const days = ref(30)
const loading = ref(false)
const busy = ref(false)
const checked = ref<Set<string>>(new Set())

async function load() {
  if (!canDelete.value) return
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
onActivated(load)
// 其他页面删除后回收站内容会变
watch(changedKey, load)
watch(canDelete, load)

const total = computed(() => items.value.reduce((n, x) => n + x.size, 0))
const allChecked = computed(() => items.value.length > 0 && checked.value.size === items.value.length)
const someChecked = computed(() => checked.value.size > 0 && !allChecked.value)
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
    void loadResult()
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
const kindIcon = { file: 'file', dir: 'folder', symlink: 'link' } as const
</script>

<template>
  <div class="page">
    <section v-if="cfg && !canDelete" class="card blocked">
      <NEmpty :description="`清理功能不可用：${cfg.delete_reason}`" />
    </section>

    <template v-else>
      <div class="sum">
        <section class="card sum-card">
          <span class="avatar red"><Icon name="trashCan" :size="24" /></span>
          <div><b>{{ items.length }}</b><span>回收站中的项目</span></div>
        </section>
        <section class="card sum-card">
          <span class="avatar orange"><Icon name="disk" :size="24" /></span>
          <div><b>{{ fmtSize(total) }}</b><span>占用空间</span></div>
        </section>
        <section class="card sum-card">
          <span class="avatar primary"><Icon name="restore" :size="24" /></span>
          <div><b>{{ days }} 天</b><span>保留期限，到期自动清除</span></div>
        </section>
      </div>

      <section class="card list-card" aria-label="回收站">
        <div class="card-head">
          <NCheckbox
            :checked="allChecked"
            :indeterminate="someChecked"
            :disabled="!items.length"
            @update:checked="toggleAll"
          >
            全选
          </NCheckbox>
          <span v-if="checked.size" class="muted small">已选 {{ checked.size }} 项</span>
          <span class="spacer" />
          <NButton size="small" quaternary :disabled="busy || !items.length" @click="purge([], true)">清空回收站</NButton>
          <NButton size="small" secondary type="error" :disabled="busy || !checked.size" @click="purge([...checked])">永久删除所选</NButton>
          <NButton size="small" type="primary" :disabled="busy || !checked.size" @click="restore([...checked])">
            <template #icon><Icon name="restore" /></template>
            恢复所选
          </NButton>
        </div>
        <NSpin :show="loading">
          <div class="list">
            <div v-for="it in items" :key="it.id" class="row" :class="{ on: checked.has(it.id) }">
              <NCheckbox
                :checked="checked.has(it.id)"
                :aria-label="`选择 ${it.name}`"
                @update:checked="(v: boolean) => toggle(it.id, v)"
              />
              <span class="avatar kind"><Icon :name="kindIcon[it.kind]" :size="20" /></span>
              <div class="body">
                <div class="name">
                  <span>{{ it.name }}</span>
                  <span class="badge badge-gray">{{ kindLabel[it.kind] }}</span>
                </div>
                <span class="mono path">{{ it.path }}</span>
                <span class="muted small">
                  {{ fmtSize(it.size) }}<template v-if="it.kind === 'dir'"> · {{ it.files }} 个文件</template>
                  · 删除于 {{ fmtTime(it.deleted_at) }} · {{ fmtTime(it.expires_at) }} 清除
                </span>
              </div>
              <NButton size="small" secondary :disabled="busy" :aria-label="`恢复 ${it.name}`" @click="restore([it.id])">
                <template #icon><Icon name="restore" /></template>
                恢复
              </NButton>
            </div>
            <NEmpty v-if="!loading && !items.length" description="回收站是空的" class="empty" />
          </div>
        </NSpin>
      </section>
    </template>
  </div>
</template>

<style scoped>
.blocked { padding: 56px 16px; }
.sum { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 22px; }
@media (max-width: 720px) { .sum { grid-template-columns: 1fr; gap: 12px; } }
.sum-card { display: flex; align-items: center; gap: 14px; padding: 18px 20px; }
.sum-card div { display: flex; flex-direction: column; }
.sum-card b { font-size: 20px; line-height: 28px; }
.sum-card span:not(.avatar) { font-size: 13px; color: var(--ink-2); }
.list-card { overflow: hidden; }
.list-card .card-head { padding-bottom: 14px; border-bottom: 1px solid var(--border); }
.list { min-height: 80px; }
.row { display: flex; align-items: center; gap: 12px; padding: 12px 20px; border-bottom: 1px solid var(--border); }
.row:last-child { border-bottom: 0; }
.row.on { background: var(--primary-row); }
.kind { width: 38px; height: 38px; background: var(--chip); color: var(--ink-2); }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
.name { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; font-size: 14px; font-weight: 600; word-break: break-all; }
.path { font-size: 12px; color: var(--ink-2); word-break: break-all; }
.empty { padding: 48px 0; }
@media (max-width: 560px) {
  .kind { display: none; }
  .row { padding: 12px 14px; align-items: flex-start; }
}
</style>
