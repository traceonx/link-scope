<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NSpin } from 'naive-ui'
import Icon from './Icon.vue'
import { api } from '../api'
import { badgesOf, folderExtras, folderStatus, folderTorrent, torrentSummaryOf } from '../badges'
import { fmtSize, fmtTime } from '../format'
import type { Folder, Item } from '../types'

const props = defineProps<{ folder: Folder; canDelete: boolean; deleteBlocked: string }>()
const emit = defineEmits<{
  (e: 'delete', paths: string[], title: string): void
  (e: 'open-files', f: Folder): void
  (e: 'open-file', it: Item): void
}>()

const files = ref<Item[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')
const LIMIT = 200

watch(
  () => props.folder.path,
  async (p) => {
    files.value = []
    total.value = 0
    error.value = ''
    loading.value = true
    try {
      // 先列出待清理的文件（按大小），方便判断
      if (props.folder.is_file) {
        const r = await api.result('all', '', p, 0, 5)
        files.value = r.items.filter((x) => x.path === p)
        total.value = files.value.length
      } else {
        const r = await api.result('all', '', '', 0, LIMIT, p, 'size', 'desc')
        files.value = [...r.items].sort((a, b) => Number(a.linked) - Number(b.linked))
        total.value = r.count
      }
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)

const pending = computed(() => props.folder.size - props.folder.linked_size)
const torrent = computed(() => folderTorrent(props.folder))
const rel = (it: Item) => it.path.slice(props.folder.path.length + 1) || it.name
</script>

<template>
  <div class="fd">
    <header class="head">
      <span class="eyebrow">{{ folder.is_file ? '文件' : '文件夹' }}</span>
      <h2 class="name">{{ folder.name }}</h2>
      <span class="path mono">{{ folder.path }}</span>
      <div class="badges">
        <span class="badge" :class="folderStatus[folder.status].cls">{{ folderStatus[folder.status].label }}</span>
        <span v-for="b in folderExtras(folder)" :key="b.label" class="badge" :class="b.cls">{{ b.label }}</span>
        <span v-if="torrent" class="badge" :class="torrent.cls">{{ torrent.label }}</span>
      </div>
    </header>

    <div class="summary">
      <div class="big">
        <span>游离文件</span>
        <b :class="{ 'c-orange': folder.unlinked }">{{ folder.unlinked }} 个 · {{ fmtSize(pending) }}</b>
      </div>
      <div><span>文件总数</span><b>{{ folder.files }}</b></div>
      <div><span>已整理</span><b>{{ folder.linked }}</b></div>
      <div><span>总大小</span><b>{{ fmtSize(folder.size) }}</b></div>
      <div><span>最近修改</span><b>{{ fmtTime(folder.mtime) }}</b></div>
      <div v-if="folder.sites.length" class="wide"><span>PT 站点</span><b>{{ folder.sites.join('、') }}</b></div>
    </div>

    <div v-if="folder.status === 'unlinked'" class="notice notice-warn">
      <Icon name="alert" :size="18" />
      <span>没有任何文件被整理到媒体库。可能是 MoviePilot 识别失败、整理失败，或者还没处理。确认不需要后可以清理。</span>
    </div>
    <div v-else-if="folder.status === 'partial'" class="notice notice-info">
      <Icon name="link" :size="18" />
      <span>已整理 {{ folder.linked }} 个文件，剩下 {{ folder.unlinked }} 个（{{ fmtSize(pending) }}）没被用上，通常是花絮、样片或没匹配上的资源。</span>
    </div>
    <div v-else class="notice notice-ok">
      <Icon name="check" :size="18" />
      <span>全部文件都已整理到媒体库。</span>
    </div>
    <div v-if="folder.seeding" class="notice notice-seed">
      <Icon name="seed" :size="18" />
      <span>{{ folder.seeding }} 个文件仍在做种，删除前请确认已满足站点的保种要求。</span>
    </div>

    <section class="section">
      <span class="section-title">
        文件
        <span class="muted hint">（未整理的排在前面，点击查看详情）</span>
        <template v-if="total > files.length"> · 显示 {{ files.length }} / {{ total }}</template>
      </span>
      <div v-if="error" class="notice notice-err">{{ error }}</div>
      <NSpin :show="loading">
        <ul class="files">
          <li v-for="it in files" :key="it.path">
            <button type="button" class="file" :aria-label="`查看 ${it.name} 的详情`" @click="emit('open-file', it)">
              <span class="file-name">{{ rel(it) }}</span>
              <span class="file-meta">
                <span v-for="b in badgesOf(it)" :key="b.label" class="badge" :class="b.cls">{{ b.label }}</span>
                <span v-if="torrentSummaryOf(it).badge" class="badge" :class="torrentSummaryOf(it).badge!.cls">
                  {{ torrentSummaryOf(it).badge!.label }}
                </span>
                <span v-if="torrentSummaryOf(it).sites.length" class="muted small">{{ torrentSummaryOf(it).sites.join('、') }}</span>
                <span class="spacer" />
                <span class="muted small">{{ it.size ? fmtSize(it.size) : '' }}</span>
                <Icon name="chevron" :size="14" class="muted" />
              </span>
            </button>
          </li>
          <li v-if="!loading && !files.length && !error" class="empty muted">没有可显示的文件</li>
        </ul>
      </NSpin>
    </section>

    <footer class="actions">
      <NButton v-if="!folder.is_file" @click="emit('open-files', folder)">
        <template #icon><Icon name="file" /></template>
        只看这个文件夹的文件
      </NButton>
      <NButton
        v-if="canDelete"
        type="error"
        :disabled="!!deleteBlocked"
        :title="deleteBlocked || undefined"
        @click="emit('delete', [folder.path], folder.is_file ? '清理这个文件' : '清理这个文件夹')"
      >
        <template #icon><Icon name="broom" /></template>
        清理{{ folder.is_file ? '这个文件' : '这个文件夹' }}
      </NButton>
    </footer>
  </div>
</template>

<style scoped>
.fd { display: flex; flex-direction: column; gap: 16px; }
.head { display: flex; flex-direction: column; gap: 6px; }
.eyebrow { font-size: 12px; font-weight: 600; color: var(--ink-2); }
.name { margin: 0; font-size: 17px; font-weight: 600; line-height: 24px; word-break: break-all; }
.path { font-size: 12px; color: var(--ink-2); word-break: break-all; }
.badges { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 2px; }
.summary { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 14px; border-radius: 8px; background: var(--soft); }
.summary div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.summary .big, .summary .wide { grid-column: span 2; }
.summary span { font-size: 12px; color: var(--ink-2); }
.summary b { font-size: 14px; font-weight: 600; word-break: break-all; }
.summary .big b { font-size: 20px; }
.c-orange { color: var(--orange-ink); }
.notice-ok { background: var(--blue-bg); color: var(--blue-ink); }
.notice-seed { background: var(--green-bg); color: var(--green-ink); }
.section { display: flex; flex-direction: column; gap: 8px; }
.section-title { font-size: 13px; font-weight: 600; }
.hint { font-weight: 400; font-size: 12px; }
.files { list-style: none; margin: 0; padding: 0; border: 1px solid var(--border); border-radius: 8px; min-height: 48px; }
.files li + li { border-top: 1px solid var(--border); }
.file { width: 100%; display: flex; flex-direction: column; gap: 4px; padding: 9px 12px; border: 0; background: transparent; color: inherit; font: inherit; text-align: left; cursor: pointer; }
.file:hover { background: var(--soft); }
.file:focus-visible { outline: 2px solid var(--blue); outline-offset: -2px; }
.file-name { font-size: 13px; word-break: break-all; }
.file-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 6px; }
.spacer { flex: 1; }
.small { font-size: 12px; }
.empty { padding: 16px; text-align: center; font-size: 13px; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; position: sticky; bottom: 0; padding: 12px 0 4px; background: var(--surface); }
.actions :deep(.n-button) { flex: 1 1 auto; }
</style>
