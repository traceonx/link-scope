<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { NButton, NEmpty, NInput, NRadioButton, NRadioGroup, NSelect, NTab, NTabs, useMessage } from 'naive-ui'
import Icon from '../components/Icon.vue'
import AppHeader from '../components/AppHeader.vue'
import ScanPanel from '../components/ScanPanel.vue'
import StatsOverview from '../components/StatsOverview.vue'
import SiteOverview from '../components/SiteOverview.vue'
import FolderTable from '../components/FolderTable.vue'
import ResultTable from '../components/ResultTable.vue'
import BulkBar from '../components/BulkBar.vue'
import DetailDrawer from '../components/DetailDrawer.vue'
import DeleteDialog from '../components/DeleteDialog.vue'
import TrashModal from '../components/TrashModal.vue'
import DownloaderModal from '../components/DownloaderModal.vue'
import UpdateModal from '../components/UpdateModal.vue'
import SettingsModal from '../components/SettingsModal.vue'
import { api } from '../api'
import { fmtCount, fmtSize } from '../format'
import { viewPref } from '../prefs'
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
} from '../types'

const message = useMessage()

// ---- 全局状态 ----
const cfg = ref<AppConfig | null>(null)
const job = ref<JobState>({ status: 'idle', done: 0, total: 0 })
const summary = ref<Summary | null>(null)
const totals = ref<FolderTotals | null>(null)
const loadError = ref('')
const running = computed(() => job.value.status === 'running')
const canDelete = computed(() => !!cfg.value?.can_delete)
const deleteBlocked = computed(() => (running.value ? '扫描进行中，扫描结束后才能清理' : ''))
const hasTorrents = computed(() => !!summary.value?.torrents)

// ---- 结果区 ----
const view = viewPref
const fFilter = ref<FolderFilter>('unlinked')
const filter = ref<Filter>('unlinked')
const site = ref('')
const q = ref('')
const dirFilter = ref('')
const page = ref(1)
const pageSize = ref(50)
const fSort = reactive<{ by: FolderSort; order: SortOrder }>({ by: 'pending', order: 'desc' })
const iSort = reactive<{ by: FileSort; order: SortOrder }>({ by: 'size', order: 'desc' })
const folders = ref<Folder[]>([])
const items = ref<Item[]>([])
const count = ref(0)
const loading = ref(false)
const fCounts = ref<Record<string, number>>({})
const counts = ref<Record<string, number>>({})
const resultsEl = ref<HTMLElement | null>(null)

const folderTabs = computed(() => {
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
const fileTabs = computed(() => {
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
const siteOptions = computed(() => [
  { label: '全部站点', value: '' },
  ...(summary.value?.torrents?.sites ?? []).map((s) => ({ label: `${s.site}（${s.files}）`, value: s.site })),
])
const currentFilter = computed<string>(() => (view.value === 'folders' ? fFilter.value : filter.value))
const currentTabLabel = computed(() => {
  const tabs: { key: string; label: string }[] = view.value === 'folders' ? folderTabs.value : fileTabs.value
  return tabs.find((t) => t.key === currentFilter.value)?.label ?? '全部'
})
const hasFilters = computed(() => currentFilter.value !== 'all' || !!site.value || !!q.value.trim() || !!dirFilter.value)
const dirLabel = computed(() => {
  const src = summary.value?.source ?? ''
  return dirFilter.value.startsWith(src + '/') ? dirFilter.value.slice(src.length + 1) : dirFilter.value
})

async function loadResult() {
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
function onFolderSort(by: FolderSort, order: SortOrder) {
  Object.assign(fSort, { by, order })
  page.value = 1
  loadResult()
}
function onFileSort(by: FileSort, order: SortOrder) {
  Object.assign(iSort, { by, order })
  page.value = 1
  loadResult()
}
function onPageSize(s: number) {
  pageSize.value = s
  page.value = 1
}
function clearFilters() {
  site.value = ''
  q.value = ''
  dirFilter.value = ''
  if (view.value === 'folders') fFilter.value = 'all'
  else filter.value = 'all'
}
async function go(v: ViewMode, f: FolderFilter | Filter) {
  dirFilter.value = ''
  if (v === 'folders') fFilter.value = f as FolderFilter
  else filter.value = f as Filter
  view.value = v
  await nextTick()
  resultsEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// ---- 勾选（跨页保留，记录大小用于显示） ----
const checked = ref<string[]>([])
const checkedSize = new Map<string, number>()
const checkedTotal = ref(0)
function onChecked(keys: string[]) {
  const set = new Set(keys)
  for (const k of [...checkedSize.keys()]) if (!set.has(k)) checkedSize.delete(k)
  const sizeOf = (k: string) =>
    view.value === 'folders' ? folders.value.find((f) => f.path === k)?.size : items.value.find((i) => i.path === k)?.size
  for (const k of keys) if (!checkedSize.has(k)) checkedSize.set(k, sizeOf(k) ?? 0)
  checked.value = keys
  checkedTotal.value = [...checkedSize.values()].reduce((a, b) => a + b, 0)
}
function clearChecked() {
  checked.value = []
  checkedSize.clear()
  checkedTotal.value = 0
}

// ---- 详情抽屉 ----
const openFolder = ref<Folder | null>(null)
const openItem = ref<Item | null>(null)
function showFolder(f: Folder) {
  openItem.value = null
  openFolder.value = f
}
function showItem(it: Item) {
  openFolder.value = null
  openItem.value = it
}
function closeDrawer() {
  openFolder.value = null
  openItem.value = null
}
function onlyFolder(f: Folder) {
  closeDrawer()
  dirFilter.value = f.path
  filter.value = 'all'
  view.value = 'files'
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
        message.success(`扫描完成，用时 ${s.summary?.duration ?? 0} 秒`)
      } else if (s.state.status === 'error') {
        message.error(`扫描失败：${s.state.error || '未知错误'}`)
      } else if (s.state.status === 'cancelled') {
        message.info('已取消扫描')
      }
    } catch {
      /* 网络抖动时下次再试 */
    }
  }, 700)
}
async function startScan(req: ScanRequest) {
  try {
    await api.scan(req)
    job.value = { status: 'running', phase: 'source', done: 0, total: 0 }
    startPolling()
  } catch (e) {
    message.error((e as Error).message)
  }
}
async function cancelScan() {
  await api.cancel().catch(() => undefined)
}

// ---- 删除 ----
const del = reactive({ show: false, target: null as DeleteTarget | null, title: '' })
const pickerReload = ref(0)
function askDelete(target: DeleteTarget, title: string) {
  if (deleteBlocked.value) {
    message.warning(deleteBlocked.value)
    return
  }
  Object.assign(del, { show: true, target, title })
}
function deletePaths(paths: string[], title = '清理') {
  askDelete({ paths }, title)
}
function deleteChecked() {
  deletePaths([...checked.value], `清理所选 ${checked.value.length} ${view.value === 'folders' ? '个文件夹' : '项'}`)
}
function deleteAllFiltered() {
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
async function onDeleted(r: DeleteResult) {
  if (!r.deleted) return
  clearChecked()
  closeDrawer()
  pickerReload.value++
  await loadResult()
}

// ---- 其他弹窗 ----
const showTrash = ref(false)
const showDownloaders = ref(false)
const showUpdate = ref(false)
const showSettings = ref(false)
const downloaderCount = ref(0)
const hasUpdate = ref(false)
function onUpdateStatus(s: UpdateStatus) {
  hasUpdate.value = s.hasUpdate
}
function openUpdateFromSettings() {
  showSettings.value = false
  showUpdate.value = true
}

onMounted(async () => {
  try {
    cfg.value = await api.config()
    const st = await api.status()
    job.value = st.state
    summary.value = st.summary
    if (st.state.status === 'running') startPolling()
    if (st.summary) await loadResult()
  } catch (e) {
    loadError.value = (e as Error).message
  }
  api
    .downloaders()
    .then((l) => (downloaderCount.value = l.filter((d) => d.enabled).length))
    .catch(() => undefined)
  api
    .updateStatus()
    .then(onUpdateStatus)
    .catch(() => undefined)
})
onBeforeUnmount(stopPolling)

const exportUrl = computed(() =>
  view.value === 'folders'
    ? api.foldersExportUrl(fFilter.value, site.value, q.value)
    : api.exportUrl(filter.value, site.value, q.value, dirFilter.value),
)
const emptyText = computed(() => {
  if (q.value.trim()) return `没有名称包含「${q.value.trim()}」的结果`
  if (view.value === 'folders' && fFilter.value === 'unlinked') return '没有待清理的文件夹，下载目录里的内容都已被媒体库用上'
  if (view.value === 'files' && filter.value === 'unlinked') return '没有待清理的文件'
  return `「${currentTabLabel.value}」中没有内容`
})
</script>

<template>
  <AppHeader
    :cfg="cfg"
    :downloaders="downloaderCount"
    :has-update="hasUpdate"
    @downloaders="showDownloaders = true"
    @trash="showTrash = true"
    @settings="showSettings = true"
    @update="showUpdate = true"
  />

  <main class="wrap main" :class="{ 'has-bulk': checked.length }">
    <ScanPanel
      :cfg="cfg"
      :summary="summary"
      :job="job"
      :can-delete="canDelete && !running"
      :picker-reload="pickerReload"
      @start="startScan"
      @cancel="cancelScan"
      @delete-folder="(p: string) => deletePaths([p], '删除文件夹')"
    >
      <div v-if="summary?.warnings?.length" class="notice notice-warn">
        <Icon name="alert" :size="18" />
        <div><div v-for="w in summary.warnings" :key="w">{{ w }}</div></div>
      </div>
      <div v-if="summary?.error_count" class="notice notice-err">
        <Icon name="alert" :size="18" />
        <div>
          有 {{ summary.error_count }} 个路径无法读取，通常是权限不足（compose 中不设置 PUID 即以 root 运行）。
          <details>
            <summary>查看路径</summary>
            <div v-for="e in summary.errors" :key="e" class="mono small">{{ e }}</div>
          </details>
        </div>
      </div>
      <div v-if="summary?.stale" class="notice notice-info">
        <Icon name="refresh" :size="18" />
        <span>上次扫描后有文件被清理或恢复，统计为估算值，建议重新扫描。</span>
      </div>
    </ScanPanel>

    <div v-if="loadError && !summary" class="notice notice-err">
      <Icon name="alert" :size="18" />
      <span>无法读取数据：{{ loadError }}。请确认容器正在运行，然后刷新页面。</span>
    </div>

    <!-- 首次使用 -->
    <section v-if="!summary && !running && cfg" class="card onboarding">
      <Icon name="broom" :size="28" />
      <h2>找出下载目录里没被媒体库用上的文件</h2>
      <p class="muted">
        填好下载目录（qBittorrent 的保存位置）和媒体库目录（MoviePilot 整理的目标，如电视剧、电影、动漫），点「开始扫描」。
        没有被硬链接到媒体库的文件会列为「待清理」。
      </p>
      <p v-if="!downloaderCount" class="muted small">
        可选：在右上角「下载器」中添加 qBittorrent，扫描时会同时显示做种状态和 PT 站点，清理前提醒仍在做种的文件。
      </p>
    </section>

    <template v-if="summary">
      <StatsOverview :summary="summary" :totals="totals" :has-torrents="hasTorrents" @go="go" />
      <SiteOverview v-if="summary.torrents" v-model:site="site" :torrents="summary.torrents" />

      <section ref="resultsEl" class="card results" aria-label="结果">
        <div class="r-head">
          <NRadioGroup v-model:value="view" size="small" aria-label="显示方式">
            <NRadioButton value="folders">按文件夹</NRadioButton>
            <NRadioButton value="files">按文件</NRadioButton>
          </NRadioGroup>
          <NInput
            v-model:value="q"
            clearable
            class="search"
            :placeholder="view === 'folders' ? '搜索文件夹名' : '搜索文件名或路径'"
            :input-props="{ 'aria-label': '搜索' }"
          >
            <template #prefix><Icon name="search" /></template>
          </NInput>
          <NSelect
            v-if="hasTorrents"
            v-model:value="site"
            :options="siteOptions"
            class="site-select"
            :consistent-menu-width="false"
          />
          <NButton v-if="hasFilters" quaternary size="small" @click="clearFilters">清除筛选</NButton>
          <span class="spacer" />
          <NButton tag="a" :href="exportUrl" size="small" quaternary>
            <template #icon><Icon name="download" /></template>
            导出 CSV
          </NButton>
        </div>

        <NTabs v-if="view === 'folders'" v-model:value="fFilter" type="line" size="small" class="tabs">
          <NTab v-for="t in folderTabs" :key="t.key" :name="t.key">
            {{ t.label }}<span class="tab-count">{{ fmtCount(fCounts[t.key] ?? 0) }}</span>
          </NTab>
        </NTabs>
        <NTabs v-else v-model:value="filter" type="line" size="small" class="tabs">
          <NTab v-for="t in fileTabs" :key="t.key" :name="t.key">
            {{ t.label }}<span class="tab-count">{{ fmtCount(counts[t.key] ?? 0) }}</span>
          </NTab>
        </NTabs>

        <div v-if="view === 'files' && dirFilter" class="strip">
          <Icon name="folder" />
          <span>当前仅显示文件夹 <b>{{ dirLabel }}</b> 中的文件</span>
          <NButton size="tiny" quaternary @click="dirFilter = ''">查看全部文件</NButton>
        </div>
        <div v-if="canDelete && currentFilter !== 'all' && count && !checked.length" class="strip">
          <span class="muted">「{{ currentTabLabel }}」共 {{ fmtCount(count) }} {{ view === 'folders' ? '个文件夹' : '项' }}</span>
          <NButton size="tiny" type="error" secondary :disabled="!!deleteBlocked" @click="deleteAllFiltered">
            <template #icon><Icon name="broom" /></template>
            清理全部筛选结果
          </NButton>
        </div>
        <div v-if="cfg && !cfg.can_delete" class="strip muted">
          <Icon name="alert" />
          <span>清理功能不可用：{{ cfg.delete_reason }}</span>
        </div>
        <div v-if="loadError" class="strip c-red">
          <Icon name="alert" />
          <span>加载失败：{{ loadError }}</span>
          <NButton size="tiny" @click="loadResult">重试</NButton>
        </div>

        <FolderTable
          v-if="view === 'folders'"
          :checked="checked"
          :selectable="canDelete"
          :show-torrents="hasTorrents"
          :items="folders"
          :count="count"
          :page="page"
          :page-size="pageSize"
          :loading="loading"
          :selected="openFolder?.path ?? null"
          :sort="fSort.by"
          :order="fSort.order"
          @open="showFolder"
          @update:checked="onChecked"
          @update:page="(p: number) => (page = p)"
          @update:page-size="onPageSize"
          @sort="onFolderSort"
        />
        <ResultTable
          v-else
          :checked="checked"
          :selectable="canDelete"
          :show-torrents="hasTorrents"
          :items="items"
          :count="count"
          :page="page"
          :page-size="pageSize"
          :loading="loading"
          :selected="openItem?.path ?? null"
          :sort="iSort.by"
          :order="iSort.order"
          :base-dir="summary.source"
          @open="showItem"
          @update:checked="onChecked"
          @update:page="(p: number) => (page = p)"
          @update:page-size="onPageSize"
          @sort="onFileSort"
        />

        <div v-if="!loading && !count && !loadError" class="empty">
          <NEmpty :description="emptyText">
            <template #extra>
              <div class="empty-actions">
                <NButton v-if="view === 'folders' && fFilter === 'unlinked'" size="small" @click="fFilter = 'partial'">
                  查看部分游离
                </NButton>
                <NButton v-if="hasFilters" size="small" @click="clearFilters">清除筛选</NButton>
              </div>
            </template>
          </NEmpty>
        </div>
      </section>
    </template>
  </main>

  <BulkBar
    v-if="canDelete"
    :count="checked.length"
    :size="checkedTotal"
    :unit="view === 'folders' ? '个文件夹' : '项'"
    @clear="clearChecked"
    @delete="deleteChecked"
  />

  <DetailDrawer
    :folder="openFolder"
    :item="openItem"
    :can-delete="canDelete"
    :delete-blocked="deleteBlocked"
    @close="closeDrawer"
    @delete="deletePaths"
    @open-files="onlyFolder"
  />

  <DeleteDialog v-model:show="del.show" :target="del.target" :title="del.title" @done="onDeleted" />
  <TrashModal v-model:show="showTrash" @changed="loadResult" />
  <DownloaderModal
    v-model:show="showDownloaders"
    :summary="summary?.torrents"
    :scanned="!!summary"
    @refreshed="loadResult"
    @changed="(n: number) => (downloaderCount = n)"
  />
  <UpdateModal v-model:show="showUpdate" @status="onUpdateStatus" />
  <SettingsModal v-model:show="showSettings" :cfg="cfg" @open-update="openUpdateFromSettings" />

  <span class="sr-only" aria-live="polite">{{ checked.length ? `已选 ${checked.length} 项，共 ${fmtSize(checkedTotal)}` : '' }}</span>
</template>

<style scoped>
.main { display: flex; flex-direction: column; gap: 16px; padding-top: 18px; padding-bottom: 40px; }
.main.has-bulk { padding-bottom: 96px; }
.small { font-size: 12px; }
.c-red { color: var(--red-ink); }
details summary { cursor: pointer; text-decoration: underline; }
.onboarding { padding: 32px 24px; display: flex; flex-direction: column; align-items: center; text-align: center; gap: 10px; color: var(--blue-ink); }
.onboarding h2 { margin: 0; font-size: 18px; color: var(--ink); }
.onboarding p { margin: 0; max-width: 640px; font-size: 14px; line-height: 22px; }
.results { overflow: hidden; scroll-margin-top: 72px; }
.r-head { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 10px; padding: 12px 16px 4px; }
.search { width: 240px; max-width: 100%; }
.site-select { width: 180px; }
.spacer { flex: 1; }
.tabs { padding: 0 16px; }
.tabs :deep(.n-tabs-nav) { border-bottom: 1px solid var(--border); }
.tab-count { margin-left: 4px; color: var(--ink-2); font-size: 12px; }
.strip { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 8px 16px; border-bottom: 1px solid var(--border); background: var(--soft); font-size: 13px; }
.empty { padding: 40px 16px 48px; }
.empty-actions { display: flex; gap: 8px; justify-content: center; }
@media (max-width: 640px) {
  .search, .site-select { width: 100%; }
}
</style>
