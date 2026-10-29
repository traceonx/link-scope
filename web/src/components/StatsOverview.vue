<script setup lang="ts">
import { computed } from 'vue'
import { NButton } from 'naive-ui'
import Icon from './Icon.vue'
import { fmtCount, fmtSize } from '../format'
import type { Filter, FolderFilter, FolderTotals, Summary, ViewMode } from '../types'

const props = defineProps<{ summary: Summary | null; totals: FolderTotals | null; hasTorrents: boolean }>()
const emit = defineEmits<{ (e: 'go', view: ViewMode, filter: FolderFilter | Filter): void }>()

const s = computed(() => props.summary)
const pendingPct = computed(() => {
  const x = s.value
  return x && x.total_size ? Math.round((x.unlinked_size / x.total_size) * 100) : 0
})

interface Card {
  key: string
  label: string
  value: string
  sub: string
  tone: string
  icon: string
  go: [ViewMode, FolderFilter | Filter]
}

const cards = computed<Card[]>(() => {
  const x = s.value
  const t = props.totals
  if (!x) return []
  const list: Card[] = [
    {
      key: 'done',
      label: '已整理文件',
      value: fmtCount(x.linked),
      sub: fmtSize(x.linked_size),
      tone: 'blue',
      icon: 'link',
      go: ['folders', 'full'],
    },
    {
      key: 'partial',
      label: '部分游离文件夹',
      value: fmtCount(t ? t.pending_folders - t.unlinked_folders : 0),
      sub: '有文件没被整理',
      tone: 'violet',
      icon: 'folder',
      go: ['folders', 'partial'],
    },
    {
      key: 'dup',
      label: '重复副本',
      value: fmtCount(x.dup_files),
      sub: x.dup_wasted ? `多占 ${fmtSize(x.dup_wasted)}` : '未发现',
      tone: 'violet',
      icon: 'copies',
      go: ['folders', 'dup'],
    },
  ]
  if (props.hasTorrents && x.torrents) {
    list.push(
      {
        key: 'seeding',
        label: '做种中文件',
        value: fmtCount(x.torrents.seeding),
        sub: '删除前请留意',
        tone: 'green',
        icon: 'seed',
        go: ['folders', 'seeding'],
      },
      {
        key: 'orphan',
        label: '孤儿文件',
        value: fmtCount(x.torrents.orphans),
        sub: fmtSize(x.torrents.orphan_size),
        tone: 'orange',
        icon: 'file',
        go: ['folders', 'orphan'],
      },
      {
        key: 'unreg',
        label: '站点已删种',
        value: fmtCount(x.torrents.unregistered),
        sub: '可放心清理',
        tone: 'red',
        icon: 'alert',
        go: ['folders', 'unregistered'],
      },
    )
  }
  return list
})
</script>

<template>
  <section v-if="s" class="stats" aria-label="清理概览">
    <div class="card hero">
      <div class="hero-main">
        <span class="hero-label"><span class="dot" />待清理（没被媒体库用上的文件）</span>
        <div class="hero-num">
          <b>{{ fmtSize(s.unlinked_size) }}</b>
          <span class="muted">占下载目录的 {{ pendingPct }}%</span>
        </div>
        <span class="hero-sub">
          {{ fmtCount(s.unlinked) }} 个文件
          <template v-if="totals"> · 分布在 {{ fmtCount(totals.pending_folders) }} 个文件夹</template>
        </span>
      </div>
      <div class="hero-actions">
        <NButton type="primary" :disabled="!s.unlinked" @click="emit('go', 'folders', 'unlinked')">
          <template #icon><Icon name="broom" /></template>
          查看待清理
        </NButton>
        <NButton quaternary size="small" :disabled="!s.unlinked" @click="emit('go', 'files', 'unlinked')">按文件查看</NButton>
      </div>
      <div class="hero-bar" aria-hidden="true">
        <span :style="{ width: `${100 - pendingPct}%` }" />
      </div>
      <div class="hero-legend muted">
        <span><i class="lg blue" />已整理 {{ fmtSize(s.linked_size) }}</span>
        <span><i class="lg orange" />待清理 {{ fmtSize(s.unlinked_size) }}</span>
      </div>
    </div>

    <div class="grid">
      <button
        v-for="c in cards"
        :key="c.key"
        type="button"
        class="card mini"
        @click="emit('go', c.go[0], c.go[1])"
      >
        <span class="mini-icon" :class="c.tone"><Icon :name="c.icon" :size="18" /></span>
        <span class="mini-body">
          <span class="mini-label">{{ c.label }}</span>
          <span class="mini-val"><b>{{ c.value }}</b></span>
          <span class="mini-sub muted">{{ c.sub }}</span>
        </span>
      </button>
    </div>
  </section>
</template>

<style scoped>
.stats { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); gap: 12px; }
@media (max-width: 960px) { .stats { grid-template-columns: 1fr; } }
.hero { padding: 18px 20px; display: grid; grid-template-columns: 1fr auto; gap: 12px 16px; align-content: start; }
.hero-main { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.hero-label { display: flex; align-items: center; gap: 6px; font-size: 13px; font-weight: 600; color: var(--orange-ink); }
.dot { width: 8px; height: 8px; border-radius: 4px; background: var(--orange); }
.hero-num { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 10px; }
.hero-num b { font-size: 34px; line-height: 42px; font-weight: 700; }
.hero-num .muted { font-size: 13px; }
.hero-sub { font-size: 13px; }
.hero-actions { display: flex; flex-direction: column; align-items: flex-end; gap: 6px; }
.hero-bar { grid-column: 1 / -1; height: 8px; border-radius: 4px; background: var(--orange); overflow: hidden; }
.hero-bar span { display: block; height: 8px; background: var(--blue); }
.hero-legend { grid-column: 1 / -1; display: flex; gap: 16px; font-size: 12px; }
.lg { display: inline-block; width: 8px; height: 8px; border-radius: 2px; margin-right: 4px; }
.lg.blue { background: var(--blue); }
.lg.orange { background: var(--orange); }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); gap: 12px; align-content: start; }
.mini { display: flex; align-items: flex-start; gap: 12px; padding: 14px 16px; font: inherit; color: inherit; text-align: left; cursor: pointer; }
.mini:hover { border-color: var(--border-strong); }
.mini:focus-visible { outline: 2px solid var(--blue); outline-offset: 2px; }
.mini-icon { width: 34px; height: 34px; border-radius: 8px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.mini-icon.blue { background: var(--blue-bg); color: var(--blue-ink); }
.mini-icon.violet { background: var(--violet-bg); color: var(--violet-ink); }
.mini-icon.green { background: var(--green-bg); color: var(--green-ink); }
.mini-icon.orange { background: var(--orange-bg); color: var(--orange-ink); }
.mini-icon.red { background: var(--red-bg); color: var(--red-ink); }
.mini-body { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.mini-label { font-size: 12px; color: var(--ink-2); }
.mini-val b { font-size: 20px; line-height: 26px; }
.mini-sub { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 640px) {
  .hero { grid-template-columns: 1fr; }
  .hero-actions { flex-direction: row; align-items: center; }
  .hero-num b { font-size: 28px; line-height: 36px; }
  .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
