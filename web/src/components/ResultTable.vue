<script setup lang="ts">
import { computed, h } from 'vue'
import { NDataTable, type DataTableColumns, type PaginationProps } from 'naive-ui'
import { badgesOf } from '../badges'
import { fmtSize, fmtTime } from '../format'
import type { Item } from '../types'

const props = defineProps<{
  items: Item[]
  count: number
  page: number
  pageSize: number
  loading: boolean
  selected: string | null
}>()
const emit = defineEmits<{
  (e: 'select', it: Item): void
  (e: 'update:page', p: number): void
  (e: 'update:pageSize', s: number): void
}>()

const columns: DataTableColumns<Item> = [
  {
    title: '状态',
    key: 'status',
    width: 120,
    render: (it) =>
      h(
        'div',
        { class: 'cell-badges' },
        badgesOf(it).map((b) => h('span', { class: ['badge', b.cls] }, b.label)),
      ),
  },
  {
    title: '文件',
    key: 'path',
    minWidth: 280,
    render: (it) =>
      h('div', { class: 'cell-file' }, [
        h('span', { class: 'cell-name', title: it.name }, it.name),
        it.type === 'file'
          ? h('span', { class: 'cell-dir mono', title: it.dir }, it.dir)
          : h('span', { class: 'cell-dir mono', title: it.target }, `→ ${it.target || '（空）'}`),
      ]),
  },
  {
    title: '链接数',
    key: 'nlink',
    width: 76,
    align: 'right',
    render: (it) =>
      it.type === 'file'
        ? h('span', { class: it.linked ? 'cell-nlink on' : 'cell-nlink' }, String(it.nlink ?? 1))
        : h('span', { class: 'muted' }, '—'),
  },
  { title: '大小', key: 'size', width: 96, align: 'right', render: (it) => (it.size ? fmtSize(it.size) : '—') },
  { title: '修改时间', key: 'mtime', width: 148, render: (it) => h('span', { class: 'muted' }, fmtTime(it.mtime)) },
]

const pagination = computed<PaginationProps>(() => ({
  page: props.page,
  pageSize: props.pageSize,
  itemCount: props.count,
  pageSizes: [50, 100, 200],
  showSizePicker: true,
  prefix: ({ itemCount }) => `共 ${itemCount ?? 0} 条`,
  'onUpdate:page': (p: number) => emit('update:page', p),
  'onUpdate:pageSize': (s: number) => emit('update:pageSize', s),
}))

function rowProps(it: Item) {
  return {
    style: 'cursor: pointer',
    tabindex: 0,
    onClick: () => emit('select', it),
    onKeydown: (e: KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault()
        emit('select', it)
      }
    },
  }
}

const rowClass = (it: Item) => (it.path === props.selected ? 'row-selected' : '')
</script>

<template>
  <NDataTable
    remote
    :columns="columns"
    :data="items"
    :loading="loading"
    :pagination="pagination"
    :row-key="(it: Item) => it.path"
    :row-props="rowProps"
    :row-class-name="rowClass"
    :scroll-x="760"
    :bordered="false"
    size="small"
  />
</template>

<style scoped>
:deep(.cell-badges) { display: flex; flex-wrap: wrap; gap: 4px; }
:deep(.cell-file) { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
:deep(.cell-name) { font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.cell-dir) { font-size: 12px; color: var(--ink-2); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.cell-nlink) { font-weight: 600; color: var(--ink-2); }
:deep(.cell-nlink.on) { color: var(--blue-ink); }
:deep(.row-selected td) { background: var(--blue-row) !important; }
:deep(.row-selected td:first-child) { box-shadow: inset 3px 0 0 var(--blue); }
:deep(tr:focus-visible td) { outline: 2px solid var(--blue); outline-offset: -2px; }
</style>
