<script setup lang="ts">
import { computed, h } from 'vue'
import {
  NButton,
  NDataTable,
  type DataTableColumns,
  type DataTableRowKey,
  type DataTableSortState,
  type PaginationProps,
} from 'naive-ui'
import { badgesOf, torrentSummaryOf } from '../badges'
import { fmtSize, fmtTime } from '../format'
import type { FileSort, Item, SortOrder } from '../types'

const props = defineProps<{
  items: Item[]
  count: number
  page: number
  pageSize: number
  loading: boolean
  selected: string | null
  checked: string[]
  selectable: boolean
  showTorrents: boolean
  sort: FileSort
  order: SortOrder
  baseDir: string
}>()
const emit = defineEmits<{
  (e: 'open', it: Item): void
  (e: 'update:page', p: number): void
  (e: 'update:pageSize', s: number): void
  (e: 'update:checked', keys: string[]): void
  (e: 'sort', by: FileSort, order: SortOrder): void
}>()

function sortOrder(key: FileSort) {
  return props.sort === key ? (props.order === 'desc' ? 'descend' : 'ascend') : false
}
// 目录显示为相对下载目录的路径，更短更易读
function relDir(dir: string) {
  const b = props.baseDir
  if (b && dir === b) return '（下载目录根）'
  return b && dir.startsWith(b + '/') ? dir.slice(b.length + 1) : dir
}

const columns = computed<DataTableColumns<Item>>(() => {
  const cols: DataTableColumns<Item> = []
  if (props.selectable) cols.push({ type: 'selection', width: 56 })
  cols.push(
    {
      title: '状态',
      key: 'status',
      width: 112,
      render: (it) =>
        h(
          'div',
          { class: 'cell-badges' },
          badgesOf(it).map((b) => h('span', { class: ['badge', b.cls] }, b.label)),
        ),
    },
    {
      title: '文件',
      key: 'name',
      minWidth: 280,
      sorter: true,
      sortOrder: sortOrder('name'),
      render: (it) =>
        h('div', { class: 'cell-file' }, [
          h('span', { class: 'cell-name', title: it.path }, it.name),
          it.type === 'file'
            ? h('span', { class: 'cell-dir', title: it.dir }, relDir(it.dir))
            : h('span', { class: 'cell-dir mono', title: it.target }, `→ ${it.target || '（空）'}`),
        ]),
    },
    {
      title: '大小',
      key: 'size',
      width: 100,
      align: 'right',
      sorter: true,
      sortOrder: sortOrder('size'),
      render: (it) => (it.size ? fmtSize(it.size) : '—'),
    },
    {
      title: '链接数',
      key: 'nlink',
      width: 84,
      align: 'right',
      sorter: true,
      sortOrder: sortOrder('nlink'),
      render: (it) =>
        it.type === 'file'
          ? h('span', { class: it.linked ? 'cell-nlink on' : 'cell-nlink' }, String(it.nlink ?? 1))
          : h('span', { class: 'muted' }, '—'),
    },
  )
  if (props.showTorrents) {
    cols.push(
      {
        title: '种子',
        key: 'torrent',
        width: 104,
        render: (it) => {
          const b = torrentSummaryOf(it).badge
          return b ? h('span', { class: ['badge', b.cls] }, b.label) : h('span', { class: 'muted' }, '—')
        },
      },
      {
        title: '站点',
        key: 'site',
        width: 130,
        ellipsis: { tooltip: true },
        render: (it) => torrentSummaryOf(it).sites.join('、') || '—',
      },
    )
  }
  cols.push(
    {
      title: '修改时间',
      key: 'mtime',
      width: 140,
      sorter: true,
      sortOrder: sortOrder('mtime'),
      render: (it) => h('span', { class: 'muted' }, fmtTime(it.mtime)),
    },
    {
      title: '',
      key: 'actions',
      width: 88,
      render: (it) =>
        h(
          NButton,
          { size: 'small', quaternary: true, 'aria-label': `查看 ${it.name} 的详情`, onClick: () => emit('open', it) },
          { default: () => '详情' },
        ),
    },
  )
  return cols
})

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
    onClick: (e: MouseEvent) => {
      if ((e.target as HTMLElement).closest('.n-checkbox, .n-button')) return
      emit('open', it)
    },
    onKeydown: (e: KeyboardEvent) => {
      if (e.key === 'Enter' && e.target === e.currentTarget) {
        e.preventDefault()
        emit('open', it)
      }
    },
  }
}

const rowClass = (it: Item) => (it.path === props.selected ? 'row-selected' : '')
const onChecked = (keys: DataTableRowKey[]) => emit('update:checked', keys.map(String))
function onSorter(s: DataTableSortState | DataTableSortState[] | null) {
  const st = Array.isArray(s) ? s[0] : s
  if (!st || !st.order) {
    emit('sort', 'path', 'asc')
    return
  }
  emit('sort', String(st.columnKey) as FileSort, st.order === 'descend' ? 'desc' : 'asc')
}
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
    :checked-row-keys="checked"
    :scroll-x="showTorrents ? 1080 : 840"
    table-layout="fixed"
    flex-height
    class="ls-table"
    :scrollbar-props="{ trigger: 'none' }"
    :bordered="false"
    size="small"
    @update:checked-row-keys="onChecked"
    @update:sorter="onSorter"
  />
</template>

<style scoped>
:deep(.cell-badges) { display: flex; flex-wrap: wrap; gap: 4px; }
:deep(.cell-file) { display: flex; flex-direction: column; gap: 2px; min-width: 0; overflow: hidden; }
:deep(.cell-name) { font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.cell-dir) { font-size: 12px; color: var(--ink-2); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.cell-nlink) { font-weight: 600; color: var(--ink-2); }
:deep(.cell-nlink.on) { color: var(--blue-ink); }
:deep(.row-selected td) { background: var(--primary-row) !important; }
:deep(.row-selected td:first-child) { box-shadow: inset 3px 0 0 var(--primary); }
:deep(tr:focus-visible td) { outline: 2px solid var(--primary); outline-offset: -2px; }
</style>
