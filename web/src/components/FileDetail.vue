<script setup lang="ts">
import { computed } from 'vue'
import { NButton, useMessage } from 'naive-ui'
import Icon from './Icon.vue'
import { badgesOf, torrentBadge } from '../badges'
import { copyText, fmtSize, fmtTime } from '../format'
import type { Item } from '../types'

const props = defineProps<{ item: Item; canDelete: boolean; deleteBlocked: string }>()
const emit = defineEmits<{ (e: 'delete', paths: string[], title: string): void }>()
const message = useMessage()

const torrents = computed(() => props.item.torrents ?? [])
const dups = computed(() => props.item.dups ?? [])
const seeding = computed(() => torrents.value.some((t) => t.state === 'seeding'))
const unreg = computed(() => torrents.value.some((t) => t.unregistered))
const orphan = computed(() => props.item.tags.includes('orphan'))

/** 结论：是否属于清理候选 */
const verdict = computed(() => {
  const it = props.item
  if (it.type === 'symlink') {
    return it.broken
      ? { cls: 'notice-err', icon: 'alert', text: '软链接指向的文件已不存在，可以清理。' }
      : { cls: 'notice-info', icon: 'link', text: '这是一个软链接，删除只会删掉链接本身。' }
  }
  if (it.type === 'strm') {
    if (it.broken) return { cls: 'notice-err', icon: 'alert', text: it.target ? 'STRM 指向的本地文件不存在，可以清理。' : 'STRM 文件是空的，可以清理。' }
    return { cls: 'notice-info', icon: 'link', text: it.remote ? 'STRM 指向远程地址，不检测是否可访问。' : 'STRM 指向的文件存在。' }
  }
  if (it.linked) return { cls: 'notice-ok', icon: 'check', text: '已整理到媒体库，不是清理对象。' }
  if (seeding.value)
    return { cls: 'notice-seed', icon: 'seed', text: `没被媒体库用上（${fmtSize(it.size)}），但仍在做种。满足保种要求后再清理。` }
  if (unreg.value) return { cls: 'notice-warn', icon: 'broom', text: `没被媒体库用上，且站点已删种，可以放心清理（${fmtSize(it.size)}）。` }
  return { cls: 'notice-warn', icon: 'broom', text: `没被媒体库用上，属于清理候选（${fmtSize(it.size)}）。` }
})

const allPaths = computed(() => [props.item.path, ...(props.item.links ?? [])])

async function copyPath() {
  if (await copyText(props.item.path)) message.success('已复制路径')
  else message.error('复制失败，请手动选择文本')
}
</script>

<template>
  <div class="fdt">
    <header class="head">
      <span class="eyebrow">{{ item.type === 'file' ? '文件' : item.type === 'strm' ? 'STRM' : '软链接' }}</span>
      <h2 class="name">{{ item.name }}</h2>
      <span class="path mono">{{ item.path }}</span>
      <div class="badges">
        <span v-for="b in badgesOf(item)" :key="b.label" class="badge" :class="b.cls">{{ b.label }}</span>
        <span v-if="orphan" class="badge badge-orange">孤儿文件</span>
      </div>
    </header>

    <div class="notice" :class="verdict.cls">
      <Icon :name="verdict.icon" :size="18" />
      <span>{{ verdict.text }}</span>
    </div>

    <div class="summary">
      <div><span>大小</span><b>{{ item.size ? fmtSize(item.size) : '—' }}</b></div>
      <div><span>修改时间</span><b>{{ fmtTime(item.mtime) }}</b></div>
      <div v-if="item.type === 'file'"><span>链接数</span><b>{{ item.nlink }}</b></div>
      <div v-if="item.type !== 'file' && item.target" class="wide"><span>指向</span><b class="mono">{{ item.target }}</b></div>
    </div>

    <section v-if="torrents.length" class="section">
      <span class="section-title">种子</span>
      <div v-for="t in torrents" :key="t.client + t.hash" class="box">
        <div class="row">
          <span class="badge" :class="torrentBadge(t).cls">{{ torrentBadge(t).label }}</span>
          <b class="site">{{ t.site }}</b>
          <span class="muted small">{{ t.client }}</span>
        </div>
        <span class="small">{{ t.name }}</span>
        <span v-if="t.message" class="small" :class="t.unregistered ? 'c-red' : 'muted'">Tracker：{{ t.message }}</span>
      </div>
    </section>
    <div v-else-if="orphan" class="notice notice-warn">
      <Icon name="seed" :size="18" />
      <span>不属于下载器中的任何种子（种子可能已被删除）。</span>
    </div>

    <section v-if="dups.length" class="section">
      <span class="section-title">重复副本 <span class="muted hint">内容相同，但不是硬链接，多占 {{ fmtSize(item.size * dups.length) }}</span></span>
      <div v-for="p in dups" :key="p" class="box row">
        <span class="mono small grow">{{ p }}</span>
        <NButton
          v-if="canDelete"
          size="tiny"
          quaternary
          type="error"
          :disabled="!!deleteBlocked"
          :aria-label="`删除副本 ${p}`"
          @click="emit('delete', [p], '删除重复副本')"
        >
          <template #icon><Icon name="trash" :size="14" /></template>
        </NButton>
      </div>
    </section>

    <details class="advanced">
      <summary>高级信息</summary>
      <div class="adv-body">
        <dl>
          <template v-if="item.inode"><dt>inode</dt><dd class="mono">{{ item.inode }}</dd></template>
          <template v-if="item.resolved && item.resolved !== item.target"><dt>实际路径</dt><dd class="mono">{{ item.resolved }}</dd></template>
        </dl>
        <template v-if="item.type === 'file'">
          <span class="section-title">所有路径（{{ allPaths.length }}）</span>
          <div v-for="(p, i) in allPaths" :key="p" class="box row">
            <span class="badge" :class="i === 0 ? 'badge-gray' : 'badge-blue'">{{ i === 0 ? '当前' : '硬链接' }}</span>
            <span class="mono small grow">{{ p }}</span>
            <NButton
              v-if="canDelete && i > 0"
              size="tiny"
              quaternary
              type="error"
              :disabled="!!deleteBlocked"
              :aria-label="`删除 ${p}`"
              @click="emit('delete', [p], '删除这个硬链接')"
            >
              <template #icon><Icon name="trash" :size="14" /></template>
            </NButton>
          </div>
          <span v-if="item.missing" class="muted small">另有 {{ item.missing }} 个硬链接不在媒体库目录中。</span>
        </template>
      </div>
    </details>

    <footer class="actions">
      <NButton @click="copyPath">
        <template #icon><Icon name="copy" /></template>
        复制路径
      </NButton>
      <NButton
        v-if="canDelete"
        type="error"
        :disabled="!!deleteBlocked"
        :title="deleteBlocked || undefined"
        @click="emit('delete', [item.path], '清理这个文件')"
      >
        <template #icon><Icon name="broom" /></template>
        清理这个文件
      </NButton>
    </footer>
  </div>
</template>

<style scoped>
.fdt { display: flex; flex-direction: column; gap: 16px; }
.head { display: flex; flex-direction: column; gap: 6px; }
.eyebrow { font-size: 12px; font-weight: 600; color: var(--ink-2); }
.name { margin: 0; font-size: 17px; font-weight: 600; line-height: 24px; word-break: break-all; }
.path { font-size: 12px; color: var(--ink-2); word-break: break-all; }
.badges { display: flex; flex-wrap: wrap; gap: 6px; }
.notice-ok { background: var(--blue-bg); color: var(--blue-ink); }
.notice-seed { background: var(--green-bg); color: var(--green-ink); }
.summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; padding: 14px; border-radius: 8px; background: var(--soft); }
.summary div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.summary .wide { grid-column: 1 / -1; }
.summary span { font-size: 12px; color: var(--ink-2); }
.summary b { font-size: 14px; font-weight: 600; word-break: break-all; }
.section { display: flex; flex-direction: column; gap: 8px; }
.section-title { font-size: 13px; font-weight: 600; }
.hint { font-weight: 400; font-size: 12px; }
.box { display: flex; flex-direction: column; gap: 4px; padding: 10px 12px; border: 1px solid var(--border); border-radius: 8px; }
.row { flex-direction: row; align-items: center; gap: 8px; }
.grow { flex: 1; min-width: 0; word-break: break-all; }
.site { font-size: 13px; }
.small { font-size: 12px; }
.c-red { color: var(--red-ink); }
.advanced { border: 1px solid var(--border); border-radius: 8px; }
.advanced summary { padding: 10px 12px; font-size: 13px; font-weight: 600; cursor: pointer; color: var(--ink-2); }
.advanced summary:focus-visible { outline: 2px solid var(--blue); outline-offset: -2px; }
.adv-body { display: flex; flex-direction: column; gap: 8px; padding: 0 12px 12px; }
dl { display: grid; grid-template-columns: auto 1fr; gap: 4px 12px; margin: 0; font-size: 13px; }
dt { color: var(--ink-2); }
dd { margin: 0; word-break: break-all; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; position: sticky; bottom: 0; padding: 12px 0 4px; background: var(--surface); }
.actions :deep(.n-button) { flex: 1 1 auto; }
@media (max-width: 480px) { .summary { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
