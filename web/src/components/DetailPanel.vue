<script setup lang="ts">
import { computed } from 'vue'
import { NButton, useMessage } from 'naive-ui'
import Icon from './Icon.vue'
import { badgesOf, torrentBadge } from '../badges'
import { copyText, fmtSize, fmtTime } from '../format'
import type { Item } from '../types'

const props = defineProps<{ item: Item | null; canDelete: boolean }>()
const emit = defineEmits<{ (e: 'delete', paths: string[], title: string): void }>()
const message = useMessage()

interface PathRow {
  tag: string
  cls: string
  path: string
  deletable: boolean
}

const paths = computed<PathRow[]>(() => {
  const it = props.item
  if (!it) return []
  if (it.type !== 'file') {
    const rows: PathRow[] = [
      { tag: it.type === 'strm' ? 'STRM 文件' : '软链接', cls: 'badge-gray', path: it.path, deletable: true },
    ]
    if (it.target) rows.push({ tag: '指向', cls: it.broken ? 'badge-red' : 'badge-teal', path: it.target, deletable: false })
    if (it.type === 'symlink' && it.resolved && it.resolved !== it.target) {
      rows.push({ tag: '实际路径', cls: 'badge-gray', path: it.resolved, deletable: false })
    }
    return rows
  }
  return [
    { tag: '源文件', cls: 'badge-gray', path: it.path, deletable: true },
    ...(it.links ?? []).map((p) => ({ tag: '硬链接', cls: 'badge-blue', path: p, deletable: true })),
  ]
})

const dups = computed(() => props.item?.dups ?? [])
const torrents = computed(() => props.item?.torrents ?? [])
const allLinks = computed(() => (props.item ? [props.item.path, ...(props.item.links ?? [])] : []))

async function copyAll() {
  const all = [...paths.value.map((p) => p.path), ...dups.value].join('\n')
  if (await copyText(all)) message.success('已复制到剪贴板')
  else message.error('复制失败，请手动选择文本')
}
</script>

<template>
  <aside class="card detail" aria-label="详情" aria-live="polite">
    <div v-if="!item" class="detail-empty">点击列表中的条目，查看所有链接路径</div>
    <template v-else>
      <div class="detail-head">
        <span class="eyebrow">{{ item.type === 'file' ? '文件详情' : item.type === 'strm' ? 'STRM 详情' : '软链接详情' }}</span>
        <h2 class="detail-name">{{ item.name }}</h2>
        <div class="detail-badges">
          <span v-for="b in badgesOf(item)" :key="b.label" class="badge" :class="b.cls">{{ b.label }}</span>
          <span v-if="item.type === 'file' && item.linked" class="badge badge-gray">共 {{ item.nlink }} 个路径</span>
          <span v-if="item.tags.includes('orphan')" class="badge badge-orange">孤儿文件</span>
        </div>
      </div>

      <div class="meta">
        <div><span>大小</span><b>{{ item.size ? fmtSize(item.size) : '—' }}</b></div>
        <div><span>修改时间</span><b>{{ fmtTime(item.mtime) }}</b></div>
        <template v-if="item.type === 'file'">
          <div><span>链接数</span><b>{{ item.nlink }}</b></div>
          <div><span>inode</span><b class="mono">{{ item.inode }}</b></div>
        </template>
      </div>

      <div v-if="torrents.length" class="section">
        <span class="section-title">种子</span>
        <div v-for="t in torrents" :key="t.client + t.hash" class="path-row">
          <div class="tor-head">
            <span class="badge" :class="torrentBadge(t).cls">{{ torrentBadge(t).label }}</span>
            <span class="tor-site">{{ t.site }}</span>
            <span class="muted small">{{ t.client }}</span>
          </div>
          <span class="tor-name">{{ t.name }}</span>
          <span v-if="t.message" class="small" :class="t.unregistered ? 'c-red' : 'muted'">Tracker：{{ t.message }}</span>
        </div>
      </div>
      <div v-else-if="item.tags.includes('orphan')" class="notice notice-warn">
        <Icon name="seed" :size="18" />
        <span>这个文件在下载目录里，但不属于下载器中的任何种子（种子可能已被删除）。</span>
      </div>

      <div class="section">
        <span class="section-title">{{ item.type === 'file' ? '所有路径' : '路径' }}</span>
        <div v-for="p in paths" :key="p.tag + p.path" class="path-row">
          <div class="path-head">
            <span class="badge" :class="p.cls">{{ p.tag }}</span>
            <span class="spacer" />
            <NButton
              v-if="canDelete && p.deletable"
              size="tiny"
              quaternary
              type="error"
              :aria-label="`删除 ${p.path}`"
              @click="emit('delete', [p.path], '删除这个路径')"
            >
              <template #icon><Icon name="trash" :size="14" /></template>
            </NButton>
          </div>
          <span class="mono path">{{ p.path }}</span>
        </div>
      </div>

      <div v-if="item.type === 'file' && item.missing" class="notice notice-warn">
        <Icon name="alert" :size="18" />
        <span>另有 {{ item.missing }} 个硬链接不在查找目录中（可能在其他文件夹里）。</span>
      </div>

      <div v-if="dups.length" class="section">
        <span class="section-title">重复副本 <span class="muted">（内容相同，但是独立文件）</span></span>
        <div v-for="p in dups" :key="p" class="path-row">
          <div class="path-head">
            <span class="badge badge-violet">副本</span>
            <span class="spacer" />
            <NButton
              v-if="canDelete"
              size="tiny"
              quaternary
              type="error"
              :aria-label="`删除副本 ${p}`"
              @click="emit('delete', [p], '删除重复副本')"
            >
              <template #icon><Icon name="trash" :size="14" /></template>
            </NButton>
          </div>
          <span class="mono path">{{ p }}</span>
        </div>
        <div class="notice notice-info">
          <Icon name="copies" :size="18" />
          <span>这些副本多占用了 {{ fmtSize(item.size * dups.length) }}。整理时它们被复制而不是硬链接，可以删掉副本后重新用硬链接整理。</span>
        </div>
      </div>

      <div v-if="item.type === 'file' && !item.linked && !dups.length && !item.tags.includes('orphan')" class="notice notice-warn">
        <Icon name="alert" :size="18" />
        <span>这个文件只有它自己。可能还没被整理；如果已经整理过，副本可能不在查找目录里，或者跨存储空间被复制了。</span>
      </div>
      <div v-if="item.type === 'symlink' && item.broken" class="notice notice-err">
        <Icon name="alert" :size="18" />
        <span>软链接指向的文件已经不存在。若在 Docker 中运行，也可能是容器里没有挂载这个路径。</span>
      </div>
      <div v-if="item.type === 'strm' && item.broken" class="notice notice-err">
        <Icon name="alert" :size="18" />
        <span>{{ item.target ? 'STRM 指向的本地文件不存在。' : 'STRM 文件是空的。' }}</span>
      </div>
      <div v-if="item.type === 'strm' && item.remote" class="notice notice-info">
        <Icon name="link" :size="18" />
        <span>这是远程地址，不会检测是否可访问。</span>
      </div>

      <div class="actions">
        <NButton block @click="copyAll">
          <template #icon><Icon name="copy" /></template>
          复制全部路径
        </NButton>
        <template v-if="canDelete">
          <NButton
            v-if="item.type === 'file' && allLinks.length > 1"
            block
            type="error"
            secondary
            @click="emit('delete', allLinks, '删除全部硬链接（释放空间）')"
          >
            <template #icon><Icon name="trash" /></template>
            删除全部 {{ allLinks.length }} 个硬链接
          </NButton>
          <NButton block secondary @click="emit('delete', [item.dir], '删除所在文件夹')">
            <template #icon><Icon name="folder" /></template>
            删除所在文件夹
          </NButton>
        </template>
      </div>
    </template>
  </aside>
</template>

<style scoped>
.detail { padding: 20px; display: flex; flex-direction: column; gap: 16px; }
.detail-empty { padding: 40px 8px; text-align: center; font-size: 14px; color: var(--ink-2); }
.detail-head { display: flex; flex-direction: column; gap: 8px; }
.eyebrow { font-size: 12px; font-weight: 600; color: var(--ink-2); }
.detail-name { margin: 0; font-size: 16px; font-weight: 600; line-height: 24px; word-break: break-all; }
.detail-badges { display: flex; flex-wrap: wrap; gap: 6px; }
.meta {
  display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px;
  padding: 14px; border-radius: 8px; background: var(--soft);
}
.meta div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.meta span { font-size: 12px; color: var(--ink-2); }
.meta b { font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; }
.section { display: flex; flex-direction: column; gap: 8px; }
.section-title { font-size: 13px; font-weight: 600; }
.section-title .muted { font-weight: 400; }
.path-row {
  display: flex; flex-direction: column; align-items: stretch; gap: 6px;
  padding: 10px 12px; border: 1px solid var(--border); border-radius: 8px;
}
.path-head, .tor-head { display: flex; align-items: center; gap: 8px; min-height: 22px; }
.spacer { flex: 1; }
.path { font-size: 12px; line-height: 18px; word-break: break-all; user-select: all; }
.tor-site { font-size: 13px; font-weight: 600; }
.tor-name { font-size: 12px; word-break: break-all; }
.small { font-size: 12px; }
.c-red { color: var(--red-ink); }
.actions { display: flex; flex-direction: column; gap: 8px; }
</style>
