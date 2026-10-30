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
import Icon from './Icon.vue'
import { folderExtras, folderStatus, folderTorrent } from '../badges'
import { fmtSize, fmtTime } from '../format'
import type { Folder, FolderSort, SortOrder } from '../types'

const props = defineProps<{
  items: Folder[]
  count: number
  page: number
  pageSize: number
  loading: boolean
  selected: string | null
  checked: string[]
  selectable: boolean
  showTorrents: boolean
  sort: FolderSort
  order: SortOrder
}>()
const emit = defineEmits<{
  (e: 'open', f: Folder): void
  (e: 'update:page', p: number): void
  (e: 'update:pageSize', s: number): void
  (e: 'update:checked', keys: string[]): void
  (e: 'sort', by: FolderSort, order: SortOrder): void
}>()

function sortOrder(key: FolderSort) {
  return props.sort === key ? (props.order === 'desc' ? 'descend' : 'ascend') : false
}

const columns = computed<DataTableColumns<Folder>>(() => {
  const cols: DataTableColumns<Folder> = []
  if (props.selectable) cols.push({ type: 'selection', width: 56 })
  cols.push(
    {
      title: '状态',
      key: 'status',
      width: 100,
      render: (f) => {
        const b = folderStatus[f.status]
        return h('span', { class: ['badge', b.cls] }, b.label)
      },
    },
    {
      title: '名称',
      key: 'name',
      minWidth: 240,
      sorter: true,
      sortOrder: sortOrder('name'),
      render: (f) =>
        h('div', { class: 'cell-name-row' }, [
          h(Icon, { name: f.is_file ? 'file' : 'folder', size: 16, class: 'cell-icon' }),
          h('span', { class: 'cell-name', title: f.path }, f.name),
          ...folderExtras(f).map((b) => h('span', { class: ['badge', b.cls] }, b.label)),
        ]),
    },
    {
      title: '已整理',
      key: 'files',
      width: 130,
      sorter: true,
      sortOrder: sortOrder('files'),
      render: (f) => {
        if (!f.files) return h('span', { class: 'muted' }, '—')
        const pct = Math.round((f.linked / f.files) * 100)
        return h('div', { class: 'cell-progress', title: `${f.linked} / ${f.files} 个文件已整理` }, [
          h('span', { class: 'bar' }, [h('span', { style: `width: ${pct}%` })]),
          h('span', { class: 'muted' }, `${f.linked}/${f.files}`),
        ])
      },
    },
    {
      title: '游离文件',
      key: 'unlinked',
      width: 96,
      align: 'right',
      sorter: true,
      sortOrder: sortOrder('unlinked'),
      render: (f) => (f.unlinked ? h('b', { class: 'c-orange' }, String(f.unlinked)) : h('span', { class: 'muted' }, '0')),
    },
    {
      title: '游离大小',
      key: 'pending',
      width: 110,
      align: 'right',
      sorter: true,
      sortOrder: sortOrder('pending'),
      render: (f) => {
        const n = f.size - f.linked_size
        return n ? h('b', { class: 'c-orange' }, fmtSize(n)) : h('span', { class: 'muted' }, '—')
      },
    },
  )
  if (props.showTorrents) {
    cols.push({
      title: '种子 / 站点',
      key: 'sites',
      width: 160,
      render: (f) => {
        const b = folderTorrent(f)
        if (!b) return h('span', { class: 'muted' }, '—')
        return h('div', { class: 'cell-col' }, [
          h('span', { class: ['badge', b.cls], style: 'align-self: flex-start' }, b.label),
          f.sites.length ? h('span', { class: 'cell-sub', title: f.sites.join('、') }, f.sites.join('、')) : null,
        ])
      },
    })
  }
  cols.push(
    {
      title: '最近修改',
      key: 'mtime',
      width: 140,
      sorter: true,
      sortOrder: sortOrder('mtime'),
      render: (f) => h('span', { class: 'muted' }, fmtTime(f.mtime)),
    },
    {
      title: '',
      key: 'actions',
      width: 88,
      render: (f) =>
        h(
          NButton,
          { size: 'small', quaternary: true, 'aria-label': `查看 ${f.name} 的详情`, onClick: () => emit('open', f) },
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
  prefix: ({ itemCount }) => `共 ${itemCount ?? 0} 项`,
  'onUpdate:page': (p: number) => emit('update:page', p),
  'onUpdate:pageSize': (s: number) => emit('update:pageSize', s),
}))

function rowProps(f: Folder) {
  return {
    style: 'cursor: pointer',
    tabindex: 0,
    onClick: (e: MouseEvent) => {
      if ((e.target as HTMLElement).closest('.n-checkbox, .n-button')) return
      emit('open', f)
    },
    onKeydown: (e: KeyboardEvent) => {
      if (e.key === 'Enter' && e.target === e.currentTarget) {
        e.preventDefault()
        emit('open', f)
      }
    },
  }
}
const rowClass = (f: Folder) => (f.path === props.selected ? 'row-selected' : '')
const onChecked = (keys: DataTableRowKey[]) => emit('update:checked', keys.map(String))
function onSorter(s: DataTableSortState | DataTableSortState[] | null) {
  const st = Array.isArray(s) ? s[0] : s
  if (!st || !st.order) {
    emit('sort', 'pending', 'desc')
    return
  }
  emit('sort', String(st.columnKey) as FolderSort, st.order === 'descend' ? 'desc' : 'asc')
}
</script>

<template>
  <NDataTable
    remote
    :columns="columns"
    :data="items"
    :loading="loading"
    :pagination="pagination"
    :row-key="(f: Folder) => f.path"
    :row-props="rowProps"
    :row-class-name="rowClass"
    :checked-row-keys="checked"
    :scroll-x="showTorrents ? 1060 : 900"
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
:deep(.cell-name-row) { display: flex; align-items: center; gap: 6px; min-width: 0; overflow: hidden; }
:deep(.cell-name-row .badge), :deep(.cell-icon) { flex-shrink: 0; }
:deep(.cell-icon) { color: var(--ink-2); }
:deep(.cell-name) { font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
:deep(.cell-col) { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
:deep(.cell-sub) { font-size: 12px; color: var(--ink-2); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.cell-progress) { display: flex; align-items: center; gap: 8px; font-size: 12px; }
:deep(.cell-progress .bar) { flex: 1; height: 6px; border-radius: 3px; background: var(--orange-bg); overflow: hidden; }
:deep(.cell-progress .bar span) { display: block; height: 6px; background: var(--blue); }
:deep(.c-orange) { color: var(--orange-ink); }
:deep(.row-selected td) { background: var(--primary-row) !important; }
:deep(.row-selected td:first-child) { box-shadow: inset 3px 0 0 var(--primary); }
:deep(tr:focus-visible td) { outline: 2px solid var(--primary); outline-offset: -2px; }
</style>
