<script setup lang="ts">
import { computed, h } from 'vue'
import { NDataTable, type DataTableColumns, type DataTableRowKey, type PaginationProps } from 'naive-ui'
import Icon from './Icon.vue'
import { fmtSize, fmtTime } from '../format'
import type { Folder } from '../types'

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
}>()
const emit = defineEmits<{
  (e: 'select', f: Folder): void
  (e: 'update:page', p: number): void
  (e: 'update:pageSize', s: number): void
  (e: 'update:checked', keys: string[]): void
}>()

const statusBadge: Record<Folder['status'], { label: string; cls: string }> = {
  linked: { label: '全部链接', cls: 'badge-blue' },
  partial: { label: '部分链接', cls: 'badge-violet' },
  unlinked: { label: '未链接', cls: 'badge-orange' },
}

function torrentCell(f: Folder) {
  if (f.unregistered) return { label: `已删种 ${f.unregistered}`, cls: 'badge-red' }
  if (f.seeding) return { label: f.seeding === f.files ? '做种中' : `做种 ${f.seeding}/${f.files}`, cls: 'badge-green' }
  if (f.torrents) return { label: '未做种', cls: 'badge-gray' }
  if (f.orphans) return { label: '孤儿文件', cls: 'badge-orange' }
  return null
}

const columns = computed<DataTableColumns<Folder>>(() => {
  const cols: DataTableColumns<Folder> = []
  if (props.selectable) cols.push({ type: 'selection', width: 44 })
  cols.push(
    {
      title: '状态',
      key: 'status',
      width: 104,
      render: (f) => {
        const b = statusBadge[f.status]
        return h('span', { class: ['badge', b.cls] }, b.label)
      },
    },
    {
      title: '文件夹',
      key: 'name',
      minWidth: 260,
      render: (f) =>
        h('div', { class: 'cell-name-row' }, [
          h(Icon, { name: f.is_file ? 'file' : 'folder', size: 16, class: 'cell-icon' }),
          h('span', { class: 'cell-name', title: f.path }, f.name),
          f.dups ? h('span', { class: 'badge badge-violet' }, `重复 ${f.dups}`) : null,
          f.broken ? h('span', { class: 'badge badge-red' }, `失效 ${f.broken}`) : null,
        ]),
    },
    {
      title: '链接情况',
      key: 'linked',
      width: 150,
      render: (f) => {
        if (!f.files) return h('span', { class: 'muted' }, '—')
        const pct = Math.round((f.linked / f.files) * 100)
        return h('div', { class: 'cell-progress' }, [
          h('span', { class: 'bar' }, [h('span', { style: `width: ${pct}%` })]),
          h('span', { class: 'muted' }, `${f.linked}/${f.files}`),
        ])
      },
    },
  )
  if (props.showTorrents) {
    cols.push({
      title: '种子 / 站点',
      key: 'sites',
      width: 170,
      render: (f) => {
        const b = torrentCell(f)
        if (!b) return h('span', { class: 'muted' }, '—')
        return h('div', { class: 'cell-col' }, [
          h('span', { class: ['badge', b.cls], style: 'align-self: flex-start' }, b.label),
          f.sites.length ? h('span', { class: 'cell-sub', title: f.sites.join('、') }, f.sites.join('、')) : null,
        ])
      },
    })
  }
  cols.push(
    { title: '大小', key: 'size', width: 100, align: 'right', render: (f) => fmtSize(f.size) },
    { title: '最近修改', key: 'mtime', width: 148, render: (f) => h('span', { class: 'muted' }, fmtTime(f.mtime)) },
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
      if ((e.target as HTMLElement).closest('.n-checkbox')) return
      emit('select', f)
    },
    onKeydown: (e: KeyboardEvent) => {
      if (e.key === 'Enter') {
        e.preventDefault()
        emit('select', f)
      }
    },
  }
}
const rowClass = (f: Folder) => (f.path === props.selected ? 'row-selected' : '')
const onChecked = (keys: DataTableRowKey[]) => emit('update:checked', keys.map(String))
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
    :scroll-x="showTorrents ? 930 : 760"
    :bordered="false"
    size="small"
    @update:checked-row-keys="onChecked"
  />
</template>

<style scoped>
:deep(.cell-name-row) { display: flex; align-items: center; gap: 8px; min-width: 0; }
:deep(.cell-icon) { color: var(--ink-2); }
:deep(.cell-name) { font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
:deep(.cell-col) { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
:deep(.cell-sub) { font-size: 12px; color: var(--ink-2); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.cell-progress) { display: flex; align-items: center; gap: 8px; font-size: 12px; }
:deep(.cell-progress .bar) { flex: 1; height: 6px; border-radius: 3px; background: var(--orange-bg); overflow: hidden; }
:deep(.cell-progress .bar span) { display: block; height: 6px; background: var(--blue); }
:deep(.row-selected td) { background: var(--blue-row) !important; }
:deep(.row-selected td:first-child) { box-shadow: inset 3px 0 0 var(--blue); }
:deep(tr:focus-visible td) { outline: 2px solid var(--blue); outline-offset: -2px; }
</style>
