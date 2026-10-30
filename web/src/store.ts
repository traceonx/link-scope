// 全局状态（单例）：配置、扫描任务、结果筛选、勾选、详情、删除。各页面共享，切换页面不丢失。
import { computed, reactive, ref, watch } from 'vue'
import type { MessageApi } from 'naive-ui'
import { api } from './api'
import { navigate } from './router'
import { viewPref } from './prefs'
import type {
  AppConfig,
  DeleteResult,
  DeleteTarget,
  FileSort,
  Filter,
  Folder,
  FolderFilter,
  FolderSort,
  FolderTotals,
  Item,
  JobState,
  ScanRequest,
  SortOrder,
  Summary,
  UpdateStatus,
  ViewMode,
} from './types'

let message: MessageApi | null = null
/** 由根组件在 NMessageProvider 内注入 */
export function bindMessage(m: MessageApi) {
  message = m
}

// ---- 全局状态 ----
export const cfg = ref<AppConfig | null>(null)
export const job = ref<JobState>({ status: 'idle', done: 0, total: 0 })
export const summary = ref<Summary | null>(null)
export const totals = ref<FolderTotals | null>(null)
export const loadError = ref('')
export const ready = ref(false)
export const running = computed(() => job.value.status === 'running')
export const canDelete = computed(() => !!cfg.value?.can_delete)
export const deleteBlocked = computed(() => (running.value ? '扫描进行中，扫描结束后才能清理' : ''))
export const hasTorrents = computed(() => !!summary.value?.torrents)
export const downloaderCount = ref(0)
export const updateStatus = ref<UpdateStatus | null>(null)
export const hasUpdate = computed(() => !!updateStatus.value?.hasUpdate)

// ---- 结果区 ----
export const view = viewPref
export const fFilter = ref<FolderFilter>('unlinked')
export const filter = ref<Filter>('unlinked')
export const site = ref('')
export const q = ref('')
export const dirFilter = ref('')
export const page = ref(1)
export const pageSize = ref(50)
export const fSort = reactive<{ by: FolderSort; order: SortOrder }>({ by: 'pending', order: 'desc' })
export const iSort = reactive<{ by: FileSort; order: SortOrder }>({ by: 'size', order: 'desc' })
export const folders = ref<Folder[]>([])
export const items = ref<Item[]>([])
export const count = ref(0)
export const loading = ref(false)
export const fCounts = ref<Record<string, number>>({})
export const counts = ref<Record<string, number>>({})

export const folderTabs = computed(() => {
  const list: { key: FolderFilter; label: string }[] = [
    { key: 'unlinked', label: '待清理' },
    { key: 'partial', label: '部分游离' },
    { key: 'full', label: '已整理' },
    { key: 'dup', label: '重复副本' },
  ]
  if (hasTorrents.value) {
    list.push({ key: 'seeding', label: '做种中' }, { key: 'unregistered', label: '已删种' }, { key: 'orphan', label: '孤儿文件' })
  }
  list.push({ key: 'all', label: '全部' })
  return list
})
export const fileTabs = computed(() => {
  const list: { key: Filter; label: string }[] = [
    { key: 'unlinked', label: '待清理' },
    { key: 'linked', label: '已整理' },
    { key: 'dup', label: '重复副本' },
    { key: 'symlink', label: '软链接' },
    { key: 'strm', label: 'STRM' },
    { key: 'broken', label: '失效' },
  ]
  if (hasTorrents.value) {
    list.push({ key: 'seeding', label: '做种中' }, { key: 'unregistered', label: '已删种' }, { key: 'orphan', label: '孤儿文件' })
  }
  list.push({ key: 'all', label: '全部' })
  return list
})
export const currentFilter = computed<string>(() => (view.value === 'folders' ? fFilter.value : filter.value))
export const currentTabLabel = computed(() => {
  const tabs: { key: string; label: string }[] = view.value === 'folders' ? folderTabs.value : fileTabs.value
  return tabs.find((t) => t.key === currentFilter.value)?.label ?? '全部'
})
export const hasFilters = computed(
  () => currentFilter.value !== 'all' || !!site.value || !!q.value.trim() || !!dirFilter.value,
)

export async function loadResult() {
  loading.value = true
  loadError.value = ''
  const offset = (page.value - 1) * pageSize.value
  try {
    if (view.value === 'folders') {
      const r = await api.folders(fFilter.value, site.value, q.value, offset, pageSize.value, fSort.by, fSort.order)
      folders.value = r.items
      count.value = r.count
      fCounts.value = r.counts
      totals.value = r.totals
      summary.value = r.summary
    } else {
      const r = await api.result(filter.value, site.value, q.value, offset, pageSize.value, dirFilter.value, iSort.by, iSort.order)
      items.value = r.items
      count.value = r.count
      counts.value = r.counts
      summary.value = r.summary
      if (!totals.value && r.summary) {
        api
          .folders('all', '', '', 0, 1)
          .then((f) => (totals.value = f.totals))
          .catch(() => undefined)
      }
    }
  } catch (e) {
    loadError.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function resetPage() {
  page.value = 1
  clearChecked()
}
watch([view, fFilter, filter, site, dirFilter], () => {
  resetPage()
  loadResult()
})
watch([page, pageSize], loadResult)
let qTimer: ReturnType<typeof setTimeout> | null = null
watch(q, () => {
  if (qTimer) clearTimeout(qTimer)
  qTimer = setTimeout(() => {
    resetPage()
    loadResult()
  }, 300)
})
export function onFolderSort(by: FolderSort, order: SortOrder) {
  Object.assign(fSort, { by, order })
  page.value = 1
  loadResult()
}
export function onFileSort(by: FileSort, order: SortOrder) {
  Object.assign(iSort, { by, order })
  page.value = 1
  loadResult()
}
export function onPageSize(s: number) {
  pageSize.value = s
  page.value = 1
}
export function clearFilters() {
  site.value = ''
  q.value = ''
  dirFilter.value = ''
  if (view.value === 'folders') fFilter.value = 'all'
  else filter.value = 'all'
}
/** 跳到清理结果页并应用筛选 */
export function go(v: ViewMode, f: FolderFilter | Filter, s?: string) {
  dirFilter.value = ''
  if (s !== undefined) site.value = s
  if (v === 'folders') fFilter.value = f as FolderFilter
  else filter.value = f as Filter
  view.value = v
  navigate('results')
}

// ---- 勾选（跨页保留，记录大小用于显示） ----
export const checked = ref<string[]>([])
const checkedSize = new Map<string, number>()
export const checkedTotal = ref(0)
export function onChecked(keys: string[]) {
  const set = new Set(keys)
  for (const k of [...checkedSize.keys()]) if (!set.has(k)) checkedSize.delete(k)
  const sizeOf = (k: string) =>
    view.value === 'folders' ? folders.value.find((f) => f.path === k)?.size : items.value.find((i) => i.path === k)?.size
  for (const k of keys) if (!checkedSize.has(k)) checkedSize.set(k, sizeOf(k) ?? 0)
  checked.value = keys
  checkedTotal.value = [...checkedSize.values()].reduce((a, b) => a + b, 0)
}
export function clearChecked() {
  checked.value = []
  checkedSize.clear()
  checkedTotal.value = 0
}

// ---- 详情抽屉 ----
export const openFolder = ref<Folder | null>(null)
export const openItem = ref<Item | null>(null)
export function showFolder(f: Folder) {
  openItem.value = null
  openFolder.value = f
}
export function showItem(it: Item) {
  openFolder.value = null
  openItem.value = it
}
export function closeDrawer() {
  openFolder.value = null
  openItem.value = null
}
export function onlyFolder(f: Folder) {
  closeDrawer()
  dirFilter.value = f.path
  filter.value = 'all'
  view.value = 'files'
  navigate('results')
}

// ---- 扫描 ----
let timer: ReturnType<typeof setInterval> | null = null
function stopPolling() {
  if (timer) clearInterval(timer)
  timer = null
}
function startPolling() {
  stopPolling()
  timer = setInterval(async () => {
    try {
      const s = await api.status()
      job.value = s.state
      if (s.state.status === 'running') return
      stopPolling()
      if (s.state.status === 'done') {
        summary.value = s.summary
        closeDrawer()
        resetPage()
        await loadResult()
        message?.success(`扫描完成，用时 ${s.summary?.duration ?? 0} 秒`)
      } else if (s.state.status === 'error') {
        message?.error(`扫描失败：${s.state.error || '未知错误'}`)
      } else if (s.state.status === 'cancelled') {
        message?.info('已取消扫描')
      }
    } catch {
      /* 网络抖动时下次再试 */
    }
  }, 700)
}
export async function startScan(req: ScanRequest) {
  try {
    await api.scan(req)
    job.value = { status: 'running', phase: 'source', done: 0, total: 0 }
    startPolling()
  } catch (e) {
    message?.error((e as Error).message)
  }
}
export async function cancelScan() {
  await api.cancel().catch(() => undefined)
}

// ---- 删除（对话框挂在根组件，任何页面都可发起） ----
export const del = reactive({ show: false, target: null as DeleteTarget | null, title: '' })
export const pickerReload = ref(0)
/** 删除或恢复后递增，回收站等页面据此刷新 */
export const changedKey = ref(0)
export function askDelete(target: DeleteTarget, title: string) {
  if (deleteBlocked.value) {
    message?.warning(deleteBlocked.value)
    return
  }
  Object.assign(del, { show: true, target, title })
}
export function deletePaths(paths: string[], title = '清理') {
  askDelete({ paths }, title)
}
export function deleteChecked() {
  deletePaths([...checked.value], `清理所选 ${checked.value.length} ${view.value === 'folders' ? '个文件夹' : '项'}`)
}
export function deleteAllFiltered() {
  const label = currentTabLabel.value
  if (view.value === 'folders') {
    askDelete(
      { view: 'folders', filter: fFilter.value, site: site.value, q: q.value },
      `清理「${label}」中的全部 ${count.value} 个文件夹`,
    )
  } else {
    askDelete(
      { filter: filter.value, site: site.value, q: q.value, dir: dirFilter.value },
      `清理「${label}」中的全部 ${count.value} 项`,
    )
  }
}
export async function onDeleted(r: DeleteResult) {
  if (!r.deleted) return
  clearChecked()
  closeDrawer()
  pickerReload.value++
  changedKey.value++
  await loadResult()
}

export function refreshDownloaderCount() {
  api
    .downloaders()
    .then((l) => (downloaderCount.value = l.filter((d) => d.enabled).length))
    .catch(() => undefined)
}

// ---- 启动 ----
export async function init() {
  try {
    cfg.value = await api.config()
    const st = await api.status()
    job.value = st.state
    summary.value = st.summary
    if (st.state.status === 'running') startPolling()
    if (st.summary) await loadResult()
  } catch (e) {
    loadError.value = (e as Error).message
  } finally {
    ready.value = true
  }
  refreshDownloaderCount()
  api
    .updateStatus()
    .then((s) => (updateStatus.value = s))
    .catch(() => undefined)
}
