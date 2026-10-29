<script setup lang="ts">
import { ref, watch } from 'vue'
import { NButton, NSpin } from 'naive-ui'
import Icon from './Icon.vue'
import { api } from '../api'
import { badgesOf, torrentSummaryOf } from '../badges'
import { fmtSize, fmtTime } from '../format'
import type { Folder, Item } from '../types'

const props = defineProps<{ folder: Folder | null; canDelete: boolean }>()
const emit = defineEmits<{
  (e: 'delete', paths: string[], title: string): void
  (e: 'open-files', f: Folder): void
  (e: 'open-file', it: Item): void
}>()

const files = ref<Item[]>([])
const total = ref(0)
const loading = ref(false)
const LIMIT = 100

watch(
  () => props.folder?.path,
  async (p) => {
    files.value = []
    total.value = 0
    if (!p || !props.folder) return
    loading.value = true
    try {
      // 单个文件：直接查它自己；文件夹：查其中的文件
      const r = props.folder.is_file
        ? await api.result('all', '', p, 0, 1)
        : await api.result('all', '', '', 0, LIMIT, p)
      files.value = props.folder.is_file ? r.items.filter((x) => x.path === p) : r.items
      total.value = props.folder.is_file ? files.value.length : r.count
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)

const statusText = { linked: '全部链接', partial: '部分链接', unlinked: '未链接' } as const
const statusCls = { linked: 'badge-blue', partial: 'badge-violet', unlinked: 'badge-orange' } as const
</script>

<template>
  <aside class="card detail" aria-label="文件夹详情" aria-live="polite">
    <div v-if="!folder" class="detail-empty">点击列表中的文件夹，查看其中每个文件的链接情况</div>
    <template v-else>
      <div class="head">
        <span class="eyebrow">{{ folder.is_file ? '文件' : '文件夹' }}</span>
        <h2 class="name">{{ folder.name }}</h2>
        <span class="mono path">{{ folder.path }}</span>
        <div class="badges">
          <span class="badge" :class="statusCls[folder.status]">{{ statusText[folder.status] }}</span>
          <span v-if="folder.dups" class="badge badge-violet">重复副本 {{ folder.dups }}</span>
          <span v-if="folder.seeding" class="badge badge-green">做种中 {{ folder.seeding }}</span>
          <span v-if="folder.unregistered" class="badge badge-red">已删种 {{ folder.unregistered }}</span>
          <span v-if="folder.orphans" class="badge badge-orange">孤儿文件 {{ folder.orphans }}</span>
        </div>
      </div>

      <div class="meta">
        <div><span>文件</span><b>{{ folder.files }} 个</b></div>
        <div><span>已链接</span><b>{{ folder.linked }} / {{ folder.files }}</b></div>
        <div><span>总大小</span><b>{{ fmtSize(folder.size) }}</b></div>
        <div><span>未链接部分</span><b>{{ fmtSize(folder.size - folder.linked_size) }}</b></div>
        <div v-if="folder.sites.length" class="wide"><span>站点</span><b>{{ folder.sites.join('、') }}</b></div>
        <div class="wide"><span>最近修改</span><b>{{ fmtTime(folder.mtime) }}</b></div>
      </div>

      <div v-if="folder.status === 'unlinked' && !folder.seeding" class="notice notice-warn">
        <Icon name="alert" :size="18" />
        <span>这个{{ folder.is_file ? '文件' : '文件夹' }}里没有任何被硬链接的文件，可能还没整理，或者整理时被复制了。</span>
      </div>
      <div v-else-if="folder.status === 'partial'" class="notice notice-info">
        <Icon name="link" :size="18" />
        <span>有 {{ folder.unlinked }} 个文件没有被链接（共 {{ fmtSize(folder.size - folder.linked_size) }}），常见于没被整理的花絮、样片或字幕。</span>
      </div>

      <div class="section">
        <span class="section-title">文件 <span class="muted hint">（点击查看链接详情）</span><template v-if="total > files.length">（显示前 {{ files.length }} 个，共 {{ total }}）</template></span>
        <NSpin :show="loading">
          <div class="files">
            <button
              v-for="it in files"
              :key="it.path"
              type="button"
              class="file"
              :aria-label="`查看 ${it.name} 的链接详情`"
              @click="emit('open-file', it)"
            >
              <div class="file-badges">
                <span v-for="b in badgesOf(it)" :key="b.label" class="badge" :class="b.cls">{{ b.label }}</span>
                <span v-if="torrentSummaryOf(it).badge" class="badge" :class="torrentSummaryOf(it).badge!.cls">
                  {{ torrentSummaryOf(it).badge!.label }}
                </span>
                <span class="spacer" />
                <span class="muted small">{{ it.size ? fmtSize(it.size) : '' }}</span>
                <Icon name="chevron" :size="14" class="muted" />
              </div>
              <span class="file-name">{{ it.path.slice(folder.path.length + 1) || it.name }}</span>
              <span v-for="l in it.links ?? []" :key="l" class="mono small muted link">↳ {{ l }}</span>
            </button>
          </div>
        </NSpin>
      </div>

      <div class="actions">
        <NButton v-if="!folder.is_file" block @click="emit('open-files', folder)">
          <template #icon><Icon name="file" /></template>
          在文件视图中查看
        </NButton>
        <NButton
          v-if="canDelete"
          block
          type="error"
          secondary
          @click="emit('delete', [folder.path], folder.is_file ? '删除文件' : '删除文件夹')"
        >
          <template #icon><Icon name="trash" /></template>
          删除{{ folder.is_file ? '文件' : '整个文件夹' }}
        </NButton>
      </div>
    </template>
  </aside>
</template>

<style scoped>
.detail { padding: 20px; display: flex; flex-direction: column; gap: 16px; }
.detail-empty { padding: 40px 8px; text-align: center; font-size: 14px; color: var(--ink-2); }
.head { display: flex; flex-direction: column; gap: 6px; }
.eyebrow { font-size: 12px; font-weight: 600; color: var(--ink-2); }
.name { margin: 0; font-size: 16px; font-weight: 600; line-height: 24px; word-break: break-all; }
.path { font-size: 12px; color: var(--ink-2); word-break: break-all; }
.badges { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 2px; }
.meta { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 14px; border-radius: 8px; background: var(--soft); }
.meta div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.meta .wide { grid-column: span 2; }
.meta span { font-size: 12px; color: var(--ink-2); }
.meta b { font-size: 14px; font-weight: 500; word-break: break-all; }
.section { display: flex; flex-direction: column; gap: 8px; }
.section-title { font-size: 13px; font-weight: 600; }
.hint { font-weight: 400; font-size: 12px; }
.files { max-height: 420px; overflow-y: auto; border: 1px solid var(--border); border-radius: 8px; min-height: 48px; }
.file {
  width: 100%; display: flex; flex-direction: column; gap: 4px; padding: 8px 12px; border: 0;
  border-bottom: 1px solid var(--border); background: transparent; color: inherit; font: inherit; text-align: left; cursor: pointer;
}
.file:hover { background: var(--soft); }
.file:focus-visible { outline: 2px solid var(--blue); outline-offset: -2px; }
.file:last-child { border-bottom: 0; }
.file-badges { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; }
.spacer { flex: 1; }
.file-name { font-size: 13px; word-break: break-all; }
.link { word-break: break-all; }
.small { font-size: 12px; }
.actions { display: flex; flex-direction: column; gap: 8px; }
</style>
