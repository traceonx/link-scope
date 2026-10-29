<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { NButton, NCheckbox, NInput, NProgress, NRadioButton, NRadioGroup, NSelect, NTabs, NTab, useMessage } from 'naive-ui'
import Icon from '../components/Icon.vue'
import FolderPicker from '../components/FolderPicker.vue'
import ResultTable from '../components/ResultTable.vue'
import DetailPanel from '../components/DetailPanel.vue'
import FolderTable from '../components/FolderTable.vue'
import FolderDetail from '../components/FolderDetail.vue'
import UpdateModal from '../components/UpdateModal.vue'
import DeleteDialog from '../components/DeleteDialog.vue'
import TrashModal from '../components/TrashModal.vue'
import DownloaderModal from '../components/DownloaderModal.vue'
import { api } from '../api'
import { fmtCount, fmtSize, fmtTime } from '../format'
import type {
  AppConfig,
  DeleteResult,
  DeleteTarget,
  DupMode,
  Filter,
  Folder,
  FolderFilter,
  Item,
  JobState,
  Summary,
  UpdateStatus,
  ViewMode,
} from '../types'

const message = useMessage()

// ---- 扫描设置 ----
const cfg = ref<AppConfig | null>(null)
const form = reactive({
  source: '',
  targets: [] as string[],
  recursive: true,
  skipSmall: true,
  dupMode: 'quick' as DupMode,
})
const dupOptions = [
  { label: '重复检测：抽样比对（快）', value: 'quick' },
  { label: '重复检测：完整比对（慢）', value: 'full' },
  { label: '不检测重复文件', value: 'off' },
]

// ---- 任务与结果 ----
const job = ref<JobState>({ status: 'idle', done: 0, total: 0 })
const summary = ref<Summary | null>(null)
const emptyCounts: Record<Filter, number> = {
  all: 0,
  linked: 0,
  unlinked: 0,
  dup: 0,
  symlink: 0,
  strm: 0,
  broken: 0,
  torrent: 0,
  seeding: 0,
  unregistered: 0,
  orphan: 0,
}
const counts = ref<Record<Filter, number>>({ ...emptyCounts })
const filter = ref<Filter>('all')
const q = ref('')
const site = ref('')
const checked = ref<string[]>([])
const page = ref(1)
const pageSize = ref(50)
const items = ref<Item[]>([])
const count = ref(0)
const loading = ref(false)
const selected = ref<Item | null>(null)

// ---- 视图：按文件夹 / 按文件 ----
function loadView(): ViewMode {
  try {
    return localStorage.getItem('ls.view') === 'files' ? 'files' : 'folders'
  } catch {
    return 'folders'
  }
}
const view = ref<ViewMode>(loadView())
const dirFilter = ref('') // 文件视图中只看某个文件夹
const emptyFolderCounts: Record<FolderFilter, number> = {
  all: 0,
  linked: 0,
  full: 0,
  partial: 0,
  unlinked: 0,
  dup: 0,
  seeding: 0,
  unregistered: 0,
  orphan: 0,
}
const fFilter = ref<FolderFilter>('all')
const fCounts = ref<Record<FolderFilter, number>>({ ...emptyFolderCounts })
const folders = ref<Folder[]>([])
const selFolder = ref<Folder | null>(null)
const drillItem = ref<Item | null>(null) // 文件夹视图中点开的单个文件
const detailEl = ref<HTMLElement | null>(null)

const running = computed(() => job.value.status === 'running')

const hasTorrents = computed(() => !!summary.value?.torrents)
const filters = computed(() => {
  const list: { key: Filter; label: string }[] = [
    { key: 'all', label: '全部' },
    { key: 'linked', label: '已链接' },
    { key: 'unlinked', label: '未链接' },
    { key: 'dup', label: '重复副本' },
    { key: 'symlink', label: '软链接' },
    { key: 'strm', label: 'STRM' },
    { key: 'broken', label: '失效' },
  ]
  if (hasTorrents.value) {
    list.push(
      { key: 'seeding', label: '做种中' },
      { key: 'unregistered', label: '已删种' },
      { key: 'orphan', label: '孤儿文件' },
    )
  }
  return list
})
const folderFilters = computed(() => {
  const list: { key: FolderFilter; label: string }[] = [
    { key: 'all', label: '全部' },
    { key: 'linked', label: '已链接' },
    { key: 'partial', label: '部分链接' },
    { key: 'unlinked', label: '未链接' },
    { key: 'dup', label: '含重复副本' },
  ]
  if (hasTorrents.value) {
    list.push(
      { key: 'seeding', label: '做种中' },
      { key: 'unregistered', label: '已删种' },
      { key: 'orphan', label: '含孤儿文件' },
    )
  }
  return list
})
const siteOptions = computed(() => [
  { label: '全部站点', value: '' },
  ...(summary.value?.torrents?.sites ?? []).map((s) => ({ label: `${s.site}（${s.files}）`, value: s.site })),
])
const canDelete = computed(() => !!cfg.value?.can_delete)
const failedClients = computed(() => (summary.value?.torrents?.clients ?? []).filter((c) => !c.ok))

const linkedPct = computed(() => {
  const s = summary.value
  return s && s.total ? Math.round((s.linked / s.total) * 100) : 0
})

const progressText = computed(() => {
  const j = job.value
  if (j.phase === 'dups') return `正在比对重复文件… ${fmtCount(j.done)} / ${fmtCount(j.total)}`
  if (j.phase === 'torrents') return '正在读取下载器中的种子…'
  if (j.phase === 'targets') return `正在扫描查找目录… 已读取 ${fmtCount(j.done)} 个条目`
  return `正在扫描文件夹… 已读取 ${fmtCount(j.done)} 个条目`
})
const progressPct = computed(() => {
  const j = job.value
  if (j.phase === 'dups' && j.total) return Math.min(100, Math.round((j.done / j.total) * 100))
  return undefined
})

// ---- 数据加载 ----
async function loadResult() {
  loading.value = true
  const offset = (page.value - 1) * pageSize.value
  try {
    if (view.value === 'folders') {
      const r = await api.folders(fFilter.value, site.value, q.value, offset, pageSize.value)
      folders.value = r.items
      count.value = r.count
      fCounts.value = { ...emptyFolderCounts, ...r.counts }
      summary.value = r.summary
      if (selFolder.value) {
        selFolder.value = r.items.find((f) => f.path === selFolder.value?.path) ?? selFolder.value
      }
    } else {
      const r = await api.result(filter.value, site.value, q.value, offset, pageSize.value, dirFilter.value)
      items.value = r.items
      count.value = r.count
      counts.value = { ...emptyCounts, ...r.counts }
      summary.value = r.summary
      if (selected.value) {
        selected.value = r.items.find((it) => it.path === selected.value?.path) ?? selected.value
      }
    }
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

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
        page.value = 1
        selected.value = null
        selFolder.value = null
        drillItem.value = null
        await loadResult()
        message.success(`扫描完成，用时 ${s.summary?.duration ?? 0} 秒`)
      } else if (s.state.status === 'error') {
        message.error(s.state.error || '扫描失败')
      } else if (s.state.status === 'cancelled') {
        message.info('已取消扫描')
      }
    } catch {
      /* 网络抖动时下次再试 */
    }
  }, 600)
}

async function startScan() {
  const source = form.source.trim()
  if (!source) {
    message.warning('请先选择要检测的文件夹')
    return
  }
  try {
    await api.scan({
      source,
      targets: form.targets,
      recursive: form.recursive,
      min_size_mb: form.skipSmall ? 1 : 0,
      dup_mode: form.dupMode,
    })
    job.value = { status: 'running', phase: 'source', done: 0, total: 0 }
    startPolling()
  } catch (e) {
    message.error((e as Error).message)
  }
}

async function cancelScan() {
  await api.cancel().catch(() => undefined)
}

onMounted(async () => {
  try {
    cfg.value = await api.config()
    const st = await api.status()
    job.value = st.state
    const last = st.summary
    form.source = last?.source || cfg.value.default_source
    form.targets = last?.targets?.length ? [...last.targets] : [...cfg.value.default_targets]
    if (last) {
      form.recursive = last.recursive
      form.skipSmall = last.min_size > 0
      form.dupMode = last.dup_mode
    }
    if (st.state.status === 'running') startPolling()
    await loadResult()
  } catch (e) {
    message.error((e as Error).message)
  }
})
onBeforeUnmount(stopPolling)

watch(view, (v) => {
  try {
    localStorage.setItem('ls.view', v)
  } catch {
    /* 无痕模式等情况下忽略 */
  }
  page.value = 1
  checked.value = []
  drillItem.value = null
  if (v === 'folders') dirFilter.value = ''
  loadResult()
})
watch([filter, fFilter, site, dirFilter], () => {
  page.value = 1
  checked.value = []
  loadResult()
})
watch([page, pageSize], loadResult)
let qTimer: ReturnType<typeof setTimeout> | null = null
watch(q, () => {
  if (qTimer) clearTimeout(qTimer)
  qTimer = setTimeout(() => {
    page.value = 1
    checked.value = []
    loadResult()
  }, 300)
})
function onPageSize(s: number) {
  pageSize.value = s
  page.value = 1
}

function scrollToDetail() {
  if (window.matchMedia('(max-width: 1023px)').matches) {
    requestAnimationFrame(() => detailEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
  }
}
function selectFolder(f: Folder) {
  selFolder.value = f
  drillItem.value = null
  scrollToDetail()
}
function openFiles(f: Folder) {
  dirFilter.value = f.path
  filter.value = 'all'
  selected.value = null
  view.value = 'files'
}
function select(it: Item) {
  selected.value = it
  // 窄屏时详情在列表下方，自动滚过去
  if (window.matchMedia('(max-width: 1023px)').matches) {
    requestAnimationFrame(() => detailEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
  }
}

// ---- 目录选择 ----
const picker = reactive({ show: false, mode: 'source' as 'source' | 'target', start: '' })
function pickSource() {
  Object.assign(picker, { show: true, mode: 'source', start: form.source })
}
function pickTarget() {
  Object.assign(picker, { show: true, mode: 'target', start: form.targets[form.targets.length - 1] ?? form.source })
}
function onPicked(path: string) {
  if (picker.mode === 'source') form.source = path
  else if (!form.targets.includes(path)) form.targets.push(path)
}
function removeTarget(t: string) {
  form.targets = form.targets.filter((x) => x !== t)
}

const exportUrl = computed(() =>
  view.value === 'folders'
    ? api.foldersExportUrl(fFilter.value, site.value, q.value)
    : api.exportUrl(filter.value, site.value, q.value, dirFilter.value),
)
const dirLabel = computed(() => {
  const src = summary.value?.source ?? ''
  return dirFilter.value.startsWith(src + '/') ? dirFilter.value.slice(src.length + 1) : dirFilter.value
})

// ---- 删除 ----
const del = reactive({ show: false, target: null as DeleteTarget | null, title: '' })
const pickerReload = ref(0)
function askDelete(target: DeleteTarget, title: string) {
  if (running.value) {
    message.warning('扫描进行中，请等扫描结束后再删除')
    return
  }
  Object.assign(del, { show: true, target, title })
}
function deletePaths(paths: string[], title = '删除') {
  askDelete({ paths }, title)
}
function deleteChecked() {
  deletePaths([...checked.value], `删除所选 ${checked.value.length} 项`)
}
function deleteAllFiltered() {
  if (view.value === 'folders') {
    const f = folderFilters.value.find((x) => x.key === fFilter.value)?.label ?? '全部'
    askDelete(
      { view: 'folders', filter: fFilter.value, site: site.value, q: q.value },
      `删除「${f}」中的全部 ${count.value} 个文件夹`,
    )
    return
  }
  const f = filters.value.find((x) => x.key === filter.value)?.label ?? '全部'
  askDelete(
    { filter: filter.value, site: site.value, q: q.value, dir: dirFilter.value },
    `删除「${f}」中的全部 ${count.value} 项`,
  )
}
async function onDeleted(r: DeleteResult) {
  if (!r.deleted) return
  checked.value = []
  pickerReload.value++
  await loadResult()
  if (selected.value && !items.value.some((x) => x.path === selected.value?.path)) selected.value = null
  if (selFolder.value && !folders.value.some((x) => x.path === selFolder.value?.path)) selFolder.value = null
  drillItem.value = null
}

// ---- 回收站与下载器 ----
const showTrash = ref(false)
const showDownloaders = ref(false)
const downloaderCount = ref(0)
onMounted(() => {
  api
    .downloaders()
    .then((l) => (downloaderCount.value = l.filter((d) => d.enabled).length))
    .catch(() => undefined)
})
function siteSize(s: { size: number }) {
  const max = summary.value?.torrents?.sites[0]?.size || 1
  return `${Math.max(4, Math.round((s.size / max) * 100))}%`
}

// ---- 版本与更新 ----
const showUpdate = ref(false)
const hasUpdate = ref(false)
function onUpdateStatus(s: UpdateStatus) {
  hasUpdate.value = s.hasUpdate
}
onMounted(() => {
  // 后台检查一次是否有新版本，只用于在版本号旁显示提示点
  api.updateStatus().then(onUpdateStatus).catch(() => undefined)
})
</script>

<template>
  <header class="topbar">
    <div class="wrap topbar-in">
      <div class="logo"><Icon name="link" :size="20" /></div>
      <div class="brand">
        <span class="brand-name">LinkScope <span class="brand-cn">链接透视</span></span>
        <span class="brand-sub">硬链接 · 重复文件 · 软链接 · STRM · 种子</span>
      </div>
      <NButton quaternary size="small" class="top-btn" @click="showDownloaders = true">
        <template #icon><Icon name="server" /></template>
        <span class="top-label">下载器<template v-if="downloaderCount"> {{ downloaderCount }}</template></span>
      </NButton>
      <NButton v-if="canDelete" quaternary size="small" class="top-btn" @click="showTrash = true">
        <template #icon><Icon name="trash" /></template>
        <span class="top-label">回收站</span>
      </NButton>
      <button v-if="cfg" type="button" class="version" :title="`提交 ${cfg.version.commit}`" @click="showUpdate = true">
        v{{ cfg.version.version }}
        <span v-if="hasUpdate" class="update-dot" aria-label="有新版本" />
      </button>
    </div>
  </header>

  <main class="wrap main">
    <!-- 扫描设置 -->
    <section class="card config" aria-label="扫描设置">
      <div class="config-row">
        <div class="field">
          <label for="src" class="label">要检测的文件夹</label>
          <div class="input-row">
            <NInput
              v-model:value="form.source"
              :input-props="{ id: 'src' }"
              class="mono-input"
              placeholder="/vol1/1000/Downloads"
              :disabled="running"
              @keyup.enter="startScan"
            >
              <template #prefix><Icon name="folder" /></template>
            </NInput>
            <NButton :disabled="running" @click="pickSource">浏览…</NButton>
          </div>
        </div>
        <div class="field">
          <span class="label">在这些目录中查找链接和副本 <span class="muted">（可选，例如媒体库）</span></span>
          <div class="chips">
            <span v-for="t in form.targets" :key="t" class="chip mono">
              {{ t }}
              <button type="button" class="chip-x" :aria-label="`移除 ${t}`" :disabled="running" @click="removeTarget(t)">×</button>
            </span>
            <button type="button" class="chip-add" :disabled="running" @click="pickTarget">
              <Icon name="plus" :size="14" />添加目录
            </button>
          </div>
        </div>
      </div>

      <div class="config-foot">
        <NCheckbox v-model:checked="form.recursive" :disabled="running">包含子文件夹</NCheckbox>
        <NCheckbox v-model:checked="form.skipSmall" :disabled="running">忽略小于 1 MB 的文件（.nfo、封面图等）</NCheckbox>
        <NSelect v-model:value="form.dupMode" :options="dupOptions" :disabled="running" size="small" class="dup-select" />
        <span class="last-scan muted">
          <template v-if="summary">上次扫描 {{ fmtTime(summary.scanned_at) }} · 用时 {{ summary.duration }} 秒</template>
        </span>
        <NButton v-if="!running" type="primary" @click="startScan">
          <template #icon><Icon name="search" /></template>
          开始扫描
        </NButton>
        <NButton v-else @click="cancelScan">取消扫描</NButton>
      </div>

      <div v-if="running" class="progress">
        <NProgress
          type="line"
          :percentage="progressPct ?? 100"
          :processing="progressPct === undefined"
          :show-indicator="false"
          :height="6"
        />
        <span class="muted">{{ progressText }}</span>
      </div>

      <div v-if="summary?.warnings?.length" class="notice notice-warn">
        <Icon name="alert" :size="18" />
        <div><div v-for="w in summary.warnings" :key="w">{{ w }}</div></div>
      </div>
      <div v-if="summary?.stale" class="notice notice-info">
        <Icon name="refresh" :size="18" />
        <span>扫描后有文件被删除或恢复，统计数字为估算，建议重新扫描。</span>
      </div>
      <div v-if="failedClients.length" class="notice notice-err">
        <Icon name="server" :size="18" />
        <div>
          <div v-for="c in failedClients" :key="c.name">下载器「{{ c.name }}」读取失败：{{ c.error }}</div>
        </div>
      </div>
      <div v-if="summary?.error_count" class="notice notice-err">
        <Icon name="alert" :size="18" />
        <div>
          有 {{ summary.error_count }} 个路径无法读取（可能是权限不足）。
          <details>
            <summary>查看</summary>
            <div v-for="e in summary.errors" :key="e" class="mono err-line">{{ e }}</div>
          </details>
        </div>
      </div>
    </section>

    <!-- 统计 -->
    <section class="stats" aria-label="统计">
      <div class="card stat">
        <span class="stat-label">文件总数</span>
        <span class="stat-num">{{ summary ? fmtCount(summary.total) : '–' }}</span>
        <span class="stat-sub">{{ summary ? `共 ${fmtSize(summary.total_size)}` : ' ' }}</span>
      </div>
      <button type="button" class="card stat stat-btn" @click="filter = 'linked'">
        <span class="stat-label"><i class="dot dot-blue" />已硬链接</span>
        <span class="stat-num c-blue">{{ summary ? fmtCount(summary.linked) : '–' }}</span>
        <span class="stat-sub">{{ summary ? `占 ${linkedPct}%` : ' ' }}</span>
      </button>
      <button type="button" class="card stat stat-btn" @click="filter = 'unlinked'">
        <span class="stat-label"><i class="dot dot-orange" />未链接</span>
        <span class="stat-num c-orange">{{ summary ? fmtCount(summary.unlinked) : '–' }}</span>
        <span class="stat-sub">可能还没整理</span>
      </button>
      <div class="card stat">
        <span class="stat-label">硬链接节省空间</span>
        <span class="stat-num">{{ summary ? fmtSize(summary.saved) : '–' }}</span>
        <span class="stat-sub">相比直接复制</span>
      </div>
    </section>

    <section class="mini-stats" aria-label="其他检测">
      <button type="button" class="card mini" @click="filter = 'dup'">
        <span class="mini-icon violet"><Icon name="copies" :size="18" /></span>
        <span class="mini-body">
          <span class="mini-label">重复副本</span>
          <span class="mini-val">
            <b>{{ summary ? fmtCount(summary.dup_files) : '–' }}</b> 个文件
            <template v-if="summary?.dup_wasted"> · 多占 {{ fmtSize(summary.dup_wasted) }}</template>
          </span>
        </span>
      </button>
      <button type="button" class="card mini" @click="filter = 'symlink'">
        <span class="mini-icon gray"><Icon name="arrow" :size="18" /></span>
        <span class="mini-body">
          <span class="mini-label">软链接</span>
          <span class="mini-val">
            <b>{{ summary ? fmtCount(summary.symlinks) : '–' }}</b> 个
            <template v-if="summary?.broken_links"> · <span class="c-red">{{ summary.broken_links }} 个失效</span></template>
          </span>
        </span>
      </button>
      <button type="button" class="card mini" @click="filter = 'strm'">
        <span class="mini-icon teal"><Icon name="file" :size="18" /></span>
        <span class="mini-body">
          <span class="mini-label">STRM</span>
          <span class="mini-val">
            <b>{{ summary ? fmtCount(summary.strm) : '–' }}</b> 个
            <template v-if="summary?.broken_strm"> · <span class="c-red">{{ summary.broken_strm }} 个失效</span></template>
          </span>
        </span>
      </button>
    </section>

    <!-- 种子 / 站点 -->
    <section v-if="summary?.torrents" class="card seeds" aria-label="种子与站点">
      <div class="seeds-stats">
        <button type="button" class="seed-stat" @click="filter = 'seeding'">
          <span class="mini-icon green"><Icon name="seed" :size="18" /></span>
          <span class="mini-body">
            <span class="mini-label">做种中</span>
            <span class="mini-val"><b>{{ fmtCount(summary.torrents.seeding) }}</b> 个文件</span>
          </span>
        </button>
        <button type="button" class="seed-stat" @click="filter = 'unregistered'">
          <span class="mini-icon red"><Icon name="alert" :size="18" /></span>
          <span class="mini-body">
            <span class="mini-label">站点已删种</span>
            <span class="mini-val"><b>{{ fmtCount(summary.torrents.unregistered) }}</b> 个文件</span>
          </span>
        </button>
        <button type="button" class="seed-stat" @click="filter = 'orphan'">
          <span class="mini-icon orange"><Icon name="file" :size="18" /></span>
          <span class="mini-body">
            <span class="mini-label">孤儿文件（不属于任何种子）</span>
            <span class="mini-val">
              <b>{{ fmtCount(summary.torrents.orphans) }}</b> 个 · {{ fmtSize(summary.torrents.orphan_size) }}
            </span>
          </span>
        </button>
        <span class="muted small seeds-time">种子读取于 {{ fmtTime(summary.torrents.fetched_at) }}</span>
      </div>
      <div v-if="summary.torrents.sites.length" class="sites">
        <button
          v-for="s in summary.torrents.sites.slice(0, 12)"
          :key="s.site"
          type="button"
          class="site"
          :class="{ on: site === s.site }"
          :aria-pressed="site === s.site"
          @click="site = site === s.site ? '' : s.site"
        >
          <span class="site-name">{{ s.site }}</span>
          <span class="site-bar"><span :style="{ width: siteSize(s) }" /></span>
          <span class="site-meta muted">
            {{ s.files }} 个 · {{ fmtSize(s.size) }}<template v-if="s.unregistered"> · <span class="c-red">{{ s.unregistered }} 已删种</span></template>
          </span>
        </button>
      </div>
    </section>

    <!-- 列表 + 详情 -->
    <div class="content">
      <section class="card list" aria-label="结果列表">
        <div class="toolbar">
          <NRadioGroup v-model:value="view" size="small" class="view-switch" aria-label="显示方式">
            <NRadioButton value="folders">按文件夹</NRadioButton>
            <NRadioButton value="files">按文件</NRadioButton>
          </NRadioGroup>
          <NTabs v-if="view === 'folders'" v-model:value="fFilter" type="line" size="small" class="filter-tabs">
            <NTab v-for="f in folderFilters" :key="f.key" :name="f.key">
              {{ f.label }}<span class="tab-count">{{ fmtCount(fCounts[f.key]) }}</span>
            </NTab>
          </NTabs>
          <NTabs v-else v-model:value="filter" type="line" size="small" class="filter-tabs">
            <NTab v-for="f in filters" :key="f.key" :name="f.key">
              {{ f.label }}<span class="tab-count">{{ fmtCount(counts[f.key]) }}</span>
            </NTab>
          </NTabs>
          <div class="toolbar-right">
            <label for="q" class="sr-only">搜索文件名或路径</label>
            <NInput v-model:value="q" :input-props="{ id: 'q' }" :placeholder="view === 'folders' ? '搜索文件夹名' : '搜索文件名或路径'" clearable class="search">
              <template #prefix><Icon name="search" /></template>
            </NInput>
            <NSelect
              v-if="hasTorrents"
              v-model:value="site"
              :options="siteOptions"
              class="site-select"
              :consistent-menu-width="false"
            />
            <NButton tag="a" :href="exportUrl" :disabled="!summary">
              <template #icon><Icon name="download" /></template>
              导出 CSV
            </NButton>
          </div>
        </div>
        <div v-if="view === 'files' && dirFilter" class="bulk">
          <Icon name="folder" />
          <span>只显示 <b>{{ dirLabel }}</b> 中的文件</span>
          <NButton size="small" quaternary @click="dirFilter = ''">显示全部</NButton>
          <NButton size="small" quaternary @click="view = 'folders'">返回文件夹视图</NButton>
        </div>
        <div
          v-if="canDelete && (checked.length || ((view === 'folders' ? fFilter : filter) !== 'all' && count))"
          class="bulk"
        >
          <template v-if="checked.length">
            <span>已选 {{ checked.length }} 项</span>
            <NButton size="small" type="error" secondary @click="deleteChecked">
              <template #icon><Icon name="trash" /></template>
              删除所选
            </NButton>
            <NButton size="small" quaternary @click="checked = []">取消选择</NButton>
          </template>
          <template v-else>
            <span class="muted">当前筛选共 {{ fmtCount(count) }} 项</span>
            <NButton size="small" type="error" secondary @click="deleteAllFiltered">
              <template #icon><Icon name="trash" /></template>
              删除全部筛选结果
            </NButton>
          </template>
        </div>
        <FolderTable
          v-if="view === 'folders'"
          v-model:checked="checked"
          :selectable="canDelete"
          :show-torrents="hasTorrents"
          :items="folders"
          :count="count"
          :page="page"
          :page-size="pageSize"
          :loading="loading"
          :selected="selFolder?.path ?? null"
          @select="selectFolder"
          @update:page="(p: number) => (page = p)"
          @update:page-size="onPageSize"
        />
        <ResultTable
          v-else
          v-model:checked="checked"
          :selectable="canDelete"
          :show-torrents="hasTorrents"
          :items="items"
          :count="count"
          :page="page"
          :page-size="pageSize"
          :loading="loading"
          :selected="selected?.path ?? null"
          @select="select"
          @update:page="(p: number) => (page = p)"
          @update:page-size="onPageSize"
        />
        <div v-if="!summary && !loading" class="empty muted">还没有扫描结果。选择文件夹后点「开始扫描」。</div>
      </section>

      <div ref="detailEl" class="detail-wrap">
        <template v-if="view === 'folders'">
          <DetailPanel
            v-if="drillItem"
            :item="drillItem"
            :can-delete="canDelete"
            :back-label="`返回 ${selFolder?.name ?? '文件夹'}`"
            @back="drillItem = null"
            @delete="deletePaths"
          />
          <FolderDetail
            v-else
            :folder="selFolder"
            :can-delete="canDelete"
            @delete="deletePaths"
            @open-files="openFiles"
            @open-file="(it: Item) => (drillItem = it)"
          />
        </template>
        <DetailPanel v-else :item="selected" :can-delete="canDelete" @delete="deletePaths" />
      </div>
    </div>
  </main>

  <UpdateModal v-model:show="showUpdate" @status="onUpdateStatus" />
  <DeleteDialog v-model:show="del.show" :target="del.target" :title="del.title" @done="onDeleted" />
  <TrashModal v-model:show="showTrash" @changed="loadResult" />
  <DownloaderModal
    v-model:show="showDownloaders"
    :summary="summary?.torrents"
    :scanned="!!summary"
    @refreshed="loadResult"
    @changed="(n: number) => (downloaderCount = n)"
  />

  <FolderPicker
    v-model:show="picker.show"
    :title="picker.mode === 'source' ? '选择要检测的文件夹' : '添加查找目录'"
    :start="picker.start"
    :can-delete="canDelete"
    :reload-key="pickerReload"
    @select="onPicked"
    @delete="(p: string) => deletePaths([p], '删除文件夹')"
  />
</template>

<style scoped>
.topbar { background: var(--surface); border-bottom: 1px solid var(--border); }
.topbar-in { display: flex; align-items: center; gap: 12px; padding-top: 12px; padding-bottom: 12px; }
.logo {
  width: 36px; height: 36px; border-radius: 8px; background: var(--blue); color: #fff;
  display: flex; align-items: center; justify-content: center;
}
.brand { display: flex; flex-direction: column; flex: 1; min-width: 0; }
.brand-name { font-size: 17px; font-weight: 700; }
.brand-cn { font-weight: 500; color: var(--ink-2); margin-left: 4px; }
.brand-sub { font-size: 12px; color: var(--ink-2); }
.top-btn { color: var(--ink-2); }
@media (max-width: 640px) { .top-label { display: none; } }
.version {
  position: relative; height: 32px; padding: 0 10px; border-radius: 6px; border: 1px solid var(--border);
  background: transparent; color: var(--ink-2); font: inherit; font-size: 12px; cursor: pointer;
}
.version:hover { border-color: var(--border-strong); color: var(--ink); }
.version:focus-visible { outline: 2px solid var(--blue); outline-offset: 2px; }
.update-dot { position: absolute; top: -3px; right: -3px; width: 9px; height: 9px; border-radius: 50%; background: var(--orange); border: 2px solid var(--surface); }

.main { display: flex; flex-direction: column; gap: 16px; padding-top: 20px; padding-bottom: 32px; }

.config { padding: 20px; display: flex; flex-direction: column; gap: 16px; }
.config-row { display: flex; flex-wrap: wrap; gap: 16px; }
.field { flex: 1 1 360px; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.label { font-size: 13px; font-weight: 600; }
.label .muted { font-weight: 400; }
.input-row { display: flex; gap: 8px; }
.mono-input :deep(input) { font-family: var(--mono); }
.chips { min-height: 34px; display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.chip {
  height: 32px; display: inline-flex; align-items: center; gap: 4px; padding: 0 4px 0 10px;
  border-radius: 6px; background: var(--chip); font-size: 13px; max-width: 100%;
}
.chip-x {
  width: 24px; height: 24px; border: 0; border-radius: 4px; background: transparent; color: var(--ink-2);
  font-size: 16px; line-height: 1; cursor: pointer;
}
.chip-x:hover { background: var(--border); }
.chip-add {
  height: 32px; display: inline-flex; align-items: center; gap: 4px; padding: 0 10px; border-radius: 6px;
  border: 1px dashed var(--border-strong); background: transparent; color: var(--blue); font: inherit;
  font-size: 13px; cursor: pointer;
}
.chip-add:disabled, .chip-x:disabled { opacity: 0.5; cursor: not-allowed; }
.config-foot {
  display: flex; flex-wrap: wrap; align-items: center; gap: 12px 20px; padding-top: 14px;
  border-top: 1px solid var(--border);
}
.dup-select { width: 220px; }
.last-scan { flex: 1; font-size: 13px; text-align: right; min-width: 160px; }
.progress { display: flex; flex-direction: column; gap: 6px; font-size: 13px; }
.err-line { font-size: 12px; word-break: break-all; }
details summary { cursor: pointer; text-decoration: underline; }

.stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; }
.stat { padding: 16px 18px; display: flex; flex-direction: column; gap: 4px; text-align: left; font: inherit; color: inherit; }
.stat-btn { cursor: pointer; }
.stat-btn:hover, .mini:hover { border-color: var(--border-strong); }
.stat-btn:focus-visible, .mini:focus-visible { outline: 2px solid var(--blue); outline-offset: 2px; }
.stat-label { font-size: 13px; color: var(--ink-2); display: flex; align-items: center; gap: 6px; }
.stat-num { font-size: 30px; font-weight: 700; line-height: 38px; }
.stat-sub { font-size: 12px; color: var(--ink-2); }
.dot { width: 8px; height: 8px; border-radius: 4px; display: inline-block; }
.dot-blue { background: var(--blue); }
.dot-orange { background: var(--orange); }
.c-blue { color: var(--blue-ink); }
.c-orange { color: var(--orange-ink); }
.c-red { color: var(--red-ink); }

.mini-stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px; }
.mini {
  display: flex; align-items: center; gap: 12px; padding: 12px 16px; font: inherit; color: inherit;
  text-align: left; cursor: pointer;
}
.mini-icon { width: 36px; height: 36px; border-radius: 8px; display: flex; align-items: center; justify-content: center; }
.mini-icon.violet { background: var(--violet-bg); color: var(--violet-ink); }
.mini-icon.gray { background: var(--chip); color: var(--ink-2); }
.mini-icon.teal { background: var(--teal-bg); color: var(--teal-ink); }
.mini-body { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.mini-label { font-size: 12px; color: var(--ink-2); }
.mini-val { font-size: 14px; }
.mini-val b { font-size: 16px; }

.seeds { padding: 16px; display: flex; flex-direction: column; gap: 14px; }
.seeds-stats { display: flex; flex-wrap: wrap; align-items: center; gap: 12px 24px; }
.seed-stat { display: flex; align-items: center; gap: 10px; border: 0; background: transparent; padding: 0; font: inherit; color: inherit; text-align: left; cursor: pointer; }
.seed-stat:focus-visible, .site:focus-visible { outline: 2px solid var(--blue); outline-offset: 2px; }
.seeds-time { margin-left: auto; }
.small { font-size: 12px; }
.mini-icon.green { background: var(--green-bg); color: var(--green-ink); }
.mini-icon.red { background: var(--red-bg); color: var(--red-ink); }
.mini-icon.orange { background: var(--orange-bg); color: var(--orange-ink); }
.sites { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 8px; }
.site {
  display: flex; flex-direction: column; gap: 4px; padding: 8px 10px; border-radius: 8px;
  border: 1px solid var(--border); background: transparent; color: inherit; font: inherit; text-align: left; cursor: pointer;
}
.site:hover { border-color: var(--border-strong); }
.site.on { border-color: var(--blue); background: var(--blue-row); }
.site-name { font-size: 13px; font-weight: 600; }
.site-bar { height: 4px; border-radius: 2px; background: var(--chip); overflow: hidden; }
.site-bar span { display: block; height: 4px; background: var(--green-ink); }
.site-meta { font-size: 12px; }
.site-select { width: 180px; }
.view-switch { flex-shrink: 0; margin-bottom: 8px; }
.bulk {
  display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; padding: 8px 16px;
  border-bottom: 1px solid var(--border); background: var(--soft); font-size: 13px;
}
.content { display: flex; flex-wrap: wrap; gap: 16px; align-items: flex-start; }
.list { flex: 999 1 600px; min-width: 0; overflow: hidden; }
.detail-wrap { flex: 1 1 340px; min-width: 0; scroll-margin-top: 12px; }
@media (min-width: 1024px) { .detail-wrap { position: sticky; top: 16px; } }

.toolbar {
  display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; padding: 8px 16px 0;
  border-bottom: 1px solid var(--border);
}
.filter-tabs { flex: 1 1 420px; min-width: 0; }
.filter-tabs :deep(.n-tabs-nav) { border-bottom: 0; }
.tab-count { margin-left: 4px; color: var(--ink-2); font-size: 12px; }
.toolbar-right { display: flex; gap: 8px; flex: 1 1 300px; padding-bottom: 8px; }
.search { flex: 1; min-width: 0; }
.empty { padding: 24px 16px 32px; text-align: center; font-size: 14px; }

@media (max-width: 640px) {
  .config { padding: 16px; }
  .dup-select { width: 100%; }
  .last-scan { text-align: left; flex-basis: 100%; }
  .stat-num { font-size: 24px; line-height: 32px; }
}
</style>
