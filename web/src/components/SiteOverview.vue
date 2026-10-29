<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import { fmtSize, fmtTime } from '../format'
import type { TorrentSummary } from '../types'

const props = defineProps<{ torrents: TorrentSummary; site: string }>()
const emit = defineEmits<{ (e: 'update:site', v: string): void }>()
const open = ref(false)

const max = computed(() => props.torrents.sites[0]?.size || 1)
const failed = computed(() => props.torrents.clients.filter((c) => !c.ok))
const width = (size: number) => `${Math.max(4, Math.round((size / max.value) * 100))}%`
</script>

<template>
  <section class="card sites" aria-label="下载器与站点">
    <button type="button" class="head" :aria-expanded="open" @click="open = !open">
      <Icon name="server" />
      <span class="title">下载器与 PT 站点</span>
      <span class="muted small">
        {{ torrents.clients.length }} 个下载器 · {{ torrents.sites.length }} 个站点 · 读取于 {{ fmtTime(torrents.fetched_at) }}
      </span>
      <span v-if="failed.length" class="badge badge-red">{{ failed.length }} 个读取失败</span>
      <span v-if="site" class="badge badge-blue">已筛选：{{ site }}</span>
      <Icon name="chevronDown" :size="14" class="chev" :class="{ rot: open }" />
    </button>
    <div v-if="open" class="body">
      <div v-for="c in failed" :key="c.name" class="notice notice-err">下载器「{{ c.name }}」读取失败：{{ c.error }}</div>
      <div v-if="torrents.sites.length" class="grid">
        <button
          v-for="s in torrents.sites"
          :key="s.site"
          type="button"
          class="site"
          :class="{ on: site === s.site }"
          :aria-pressed="site === s.site"
          @click="emit('update:site', site === s.site ? '' : s.site)"
        >
          <span class="name">{{ s.site }}</span>
          <span class="bar"><span :style="{ width: width(s.size) }" /></span>
          <span class="meta muted">
            {{ s.files }} 个文件 · {{ fmtSize(s.size) }}
            <template v-if="s.unregistered"> · <span class="c-red">{{ s.unregistered }} 已删种</span></template>
          </span>
        </button>
      </div>
      <p v-else class="muted small">下载目录中的文件没有对应到任何种子，请检查下载器的路径映射。</p>
    </div>
  </section>
</template>

<style scoped>
.sites { overflow: hidden; }
.head {
  width: 100%; display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 12px 16px; border: 0;
  background: transparent; color: inherit; font: inherit; text-align: left; cursor: pointer;
}
.head:focus-visible { outline: 2px solid var(--blue); outline-offset: -2px; }
.title { font-size: 14px; font-weight: 600; }
.small { font-size: 12px; }
.chev { margin-left: auto; color: var(--ink-2); }
.chev.rot { transform: rotate(180deg); }
.body { display: flex; flex-direction: column; gap: 10px; padding: 0 16px 16px; }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 8px; }
.site { display: flex; flex-direction: column; gap: 4px; padding: 8px 10px; border-radius: 8px; border: 1px solid var(--border); background: transparent; color: inherit; font: inherit; text-align: left; cursor: pointer; }
.site:hover { border-color: var(--border-strong); }
.site.on { border-color: var(--blue); background: var(--blue-row); }
.site:focus-visible { outline: 2px solid var(--blue); outline-offset: 2px; }
.name { font-size: 13px; font-weight: 600; }
.bar { height: 4px; border-radius: 2px; background: var(--chip); overflow: hidden; }
.bar span { display: block; height: 4px; background: var(--green-ink); }
.meta { font-size: 12px; }
.c-red { color: var(--red-ink); }
p { margin: 0; }
</style>
