<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { NButton, NInput, NModal, NProgress, NSelect, NSwitch, useMessage } from 'naive-ui'
import Icon from '../components/Icon.vue'
import FolderPicker from '../components/FolderPicker.vue'
import { fmtCount, fmtTime } from '../format'
import { presetsPref, scanDraftPref } from '../prefs'
import { navigate } from '../router'
import { canDelete, cancelScan, cfg, deletePaths, job, pickerReload, running, startScan, summary } from '../store'
import type { DupMode, ScanPreset } from '../types'

const message = useMessage()

const form = reactive({
  source: '',
  targets: [] as string[],
  recursive: true,
  skipSmall: true,
  dupMode: 'quick' as DupMode,
})

// 首次拿到配置时填入表单（之后不再覆盖用户的修改）。
// 优先级：比上次扫描更新的浏览器草稿 > 上次扫描的设置 > 环境变量默认值
let inited = false
watch(
  () => [cfg.value, summary.value] as const,
  ([c, last]) => {
    if (inited || !c) return
    inited = true
    const draft = scanDraftPref.value
    if (draft && (!last || draft.saved_at > last.scanned_at * 1000)) {
      Object.assign(form, { source: draft.source, targets: [...draft.targets], recursive: draft.recursive, skipSmall: draft.skipSmall, dupMode: draft.dupMode })
      return
    }
    form.source = last?.source || c.default_source
    form.targets = last?.targets?.length ? [...last.targets] : [...c.default_targets]
    if (last) {
      form.recursive = last.recursive
      form.skipSmall = last.min_size > 0
      form.dupMode = last.dup_mode
    }
  },
  { immediate: true },
)
// 用户修改后保存草稿，刷新页面不丢失
watch(
  form,
  (f) => {
    if (!inited) return
    scanDraftPref.value = { source: f.source, targets: [...f.targets], recursive: f.recursive, skipSmall: f.skipSmall, dupMode: f.dupMode, saved_at: Date.now() }
  },
  { deep: true },
)

const dupOptions = [
  { label: '抽样比对（快）', value: 'quick' },
  { label: '完整比对（慢，最准确）', value: 'full' },
  { label: '不检测重复副本', value: 'off' },
]

function start() {
  const source = form.source.trim()
  if (!source) {
    message.warning('请先选择要检测的下载目录')
    return
  }
  startScan({
    source,
    targets: form.targets,
    recursive: form.recursive,
    min_size_mb: form.skipSmall ? 1 : 0,
    dup_mode: form.dupMode,
  })
}

// ---- 预设（保存在浏览器） ----
const presetOptions = computed(() => presetsPref.value.map((p) => ({ label: p.name, value: p.name })))
const presetName = ref<string | null>(null)
function applyPreset(name: string | null) {
  presetName.value = name
  const p = presetsPref.value.find((x) => x.name === name)
  if (!p) return
  Object.assign(form, { source: p.source, targets: [...p.targets], recursive: p.recursive, skipSmall: p.skipSmall, dupMode: p.dupMode })
}
const saveDlg = reactive({ show: false, name: '' })
function openSave() {
  saveDlg.name = presetName.value ?? ''
  saveDlg.show = true
}
function savePreset() {
  const name = saveDlg.name.trim()
  if (!name) return
  const p: ScanPreset = { name, source: form.source.trim(), targets: [...form.targets], recursive: form.recursive, skipSmall: form.skipSmall, dupMode: form.dupMode }
  presetsPref.value = [...presetsPref.value.filter((x) => x.name !== name), p]
  presetName.value = name
  saveDlg.show = false
  message.success(`已保存预设「${name}」`)
}
function removePreset() {
  if (!presetName.value) return
  presetsPref.value = presetsPref.value.filter((x) => x.name !== presetName.value)
  presetName.value = null
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

// ---- 进度 ----
const phases = [
  { key: 'source', label: '扫描下载目录' },
  { key: 'targets', label: '扫描媒体库' },
  { key: 'dups', label: '比对重复' },
  { key: 'torrents', label: '读取种子' },
] as const
const phaseIndex = computed(() => phases.findIndex((p) => p.key === job.value.phase))
const progressPct = computed(() => {
  const j = job.value
  if (j.phase === 'dups' && j.total) return Math.min(100, Math.round((j.done / j.total) * 100))
  return undefined
})
const progressText = computed(() => {
  const j = job.value
  if (j.phase === 'dups') return `已比对 ${fmtCount(j.done)} / ${fmtCount(j.total)}`
  if (j.phase === 'torrents') return '正在读取下载器中的种子…'
  return `已读取 ${fmtCount(j.done)} 个条目`
})
// 本次打开页面后扫描结束：给出去看结果的入口
const justDone = ref(false)
watch(running, (now, before) => {
  if (before && !now && job.value.status === 'done') justDone.value = true
  if (now) justDone.value = false
})
</script>

<template>
  <div class="page">
    <!-- 扫描进度 -->
    <section v-if="running" class="card progress" role="status" aria-label="扫描进度">
      <div class="card-head">
        <span class="avatar primary"><Icon name="scan" :size="24" /></span>
        <div class="ph-title">
          <h2 class="card-title">正在扫描</h2>
          <span class="card-sub">{{ progressText }} · 扫描期间不能删除文件</span>
        </div>
        <span class="spacer" />
        <NButton secondary type="error" @click="cancelScan">
          <template #icon><Icon name="stop" /></template>
          取消扫描
        </NButton>
      </div>
      <div class="card-body">
        <ol class="phases">
          <li v-for="(p, i) in phases" :key="p.key" :class="{ done: i < phaseIndex, now: i === phaseIndex }">
            <span class="num">
              <Icon v-if="i < phaseIndex" name="check" :size="14" />
              <template v-else>{{ i + 1 }}</template>
            </span>
            {{ p.label }}
          </li>
        </ol>
        <NProgress
          type="line"
          :percentage="progressPct ?? 100"
          :processing="progressPct === undefined"
          :show-indicator="false"
          :height="8"
        />
      </div>
    </section>
    <div v-else-if="justDone" class="notice notice-info done-note">
      <Icon name="check" />
      <span>扫描完成。</span>
      <NButton size="tiny" type="primary" @click="navigate('results')">查看清理结果</NButton>
      <NButton size="tiny" quaternary @click="navigate('dashboard')">查看仪表盘</NButton>
    </div>

    <div class="layout">
      <!-- 扫描目录 -->
      <section class="card" aria-label="扫描目录">
        <div class="card-head">
          <h2 class="card-title">扫描目录</h2>
          <span class="card-sub">
            <template v-if="summary">上次扫描 {{ fmtTime(summary.scanned_at) }} · 用时 {{ summary.duration }} 秒</template>
            <template v-else>还没有扫描过</template>
          </span>
        </div>
        <div class="card-body fields">
          <div class="field">
            <label for="src" class="label">下载目录 <span class="muted">（qBittorrent 的保存位置，在这里找游离文件）</span></label>
            <div class="input-row">
              <NInput
                v-model:value="form.source"
                :input-props="{ id: 'src' }"
                class="mono-input"
                placeholder="/vol2/1000/video/download/下载目录"
                :disabled="running"
                @keyup.enter="start"
              >
                <template #prefix><Icon name="folder" /></template>
              </NInput>
              <NButton :disabled="running" @click="pickSource">浏览</NButton>
            </div>
          </div>

          <div class="field">
            <span class="label">媒体库目录 <span class="muted">（在这里查找硬链接，例如电视剧、电影、动漫）</span></span>
            <div class="chips">
              <span v-for="t in form.targets" :key="t" class="chip mono" :title="t">
                <Icon name="folder" :size="16" />
                <span class="chip-text">{{ t }}</span>
                <button type="button" class="chip-x" :aria-label="`移除 ${t}`" :disabled="running" @click="removeTarget(t)">
                  <Icon name="close" :size="14" />
                </button>
              </span>
              <button type="button" class="chip-add" :disabled="running" @click="pickTarget">
                <Icon name="plus" :size="16" />添加目录
              </button>
            </div>
          </div>
        </div>
        <div class="card-foot">
          <span class="muted small">没有被硬链接到媒体库的文件会列为「待清理」。</span>
          <span class="spacer" />
          <NButton v-if="!running" type="primary" size="large" class="start" @click="start">
            <template #icon><Icon name="play" /></template>
            开始扫描
          </NButton>
          <NButton v-else size="large" class="start" @click="cancelScan">取消扫描</NButton>
        </div>
      </section>

      <div class="side">
        <!-- 扫描选项 -->
        <section class="card" aria-label="扫描选项">
          <div class="card-head">
            <h2 class="card-title">扫描选项</h2>
          </div>
          <div class="card-body opts">
            <div class="set-row">
              <div class="set-label"><b>包含子文件夹</b><span>递归检查下载目录中的所有层级</span></div>
              <NSwitch v-model:value="form.recursive" :disabled="running" aria-label="包含子文件夹" />
            </div>
            <div class="set-row">
              <div class="set-label"><b>忽略小文件</b><span>跳过小于 1 MB 的 .nfo、封面图等</span></div>
              <NSwitch v-model:value="form.skipSmall" :disabled="running" aria-label="忽略小于 1 MB 的文件" />
            </div>
            <div class="set-row col">
              <div class="set-label"><b>重复副本检测</b><span>找出内容相同但不是硬链接的文件</span></div>
              <NSelect v-model:value="form.dupMode" :options="dupOptions" :disabled="running" aria-label="重复副本检测" />
            </div>
          </div>
        </section>

        <!-- 预设 -->
        <section class="card" aria-label="扫描预设">
          <div class="card-head">
            <h2 class="card-title">扫描预设</h2>
            <span class="card-sub">仅保存在这个浏览器</span>
          </div>
          <div class="card-body presets">
            <NSelect
              :value="presetName"
              :options="presetOptions"
              :placeholder="presetOptions.length ? '选择预设' : '还没有保存预设'"
              clearable
              :disabled="running || !presetOptions.length"
              @update:value="applyPreset"
            />
            <div class="preset-actions">
              <NButton size="small" secondary type="primary" :disabled="running" @click="openSave">
                <template #icon><Icon name="preset" /></template>
                保存当前设置
              </NButton>
              <NButton v-if="presetName" size="small" quaternary type="error" :disabled="running" @click="removePreset">删除预设</NButton>
            </div>
          </div>
        </section>
      </div>
    </div>

    <FolderPicker
      v-model:show="picker.show"
      :title="picker.mode === 'source' ? '选择下载目录' : '添加媒体库目录'"
      :start="picker.start"
      :can-delete="canDelete && !running"
      :reload-key="pickerReload"
      @select="onPicked"
      @delete="(p: string) => deletePaths([p], '删除文件夹')"
    />

    <NModal
      v-model:show="saveDlg.show"
      preset="card"
      title="保存扫描预设"
      style="width: min(400px, calc(100vw - 24px))"
      :bordered="false"
    >
      <label class="field">
        <span class="label">预设名称</span>
        <NInput v-model:value="saveDlg.name" placeholder="例如：日常清理" @keyup.enter="savePreset" />
        <span class="muted small">保存当前的下载目录、媒体库目录和扫描选项，仅保存在这个浏览器中。</span>
      </label>
      <template #footer>
        <div class="dlg-foot">
          <NButton @click="saveDlg.show = false">取消</NButton>
          <NButton type="primary" :disabled="!saveDlg.name.trim()" @click="savePreset">保存</NButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.layout { display: grid; grid-template-columns: minmax(0, 8fr) minmax(300px, 4fr); gap: 22px; align-items: start; }
@media (max-width: 1100px) { .layout { grid-template-columns: minmax(0, 1fr); } }
.side { display: flex; flex-direction: column; gap: 22px; }
.ph-title { display: flex; flex-direction: column; }
.done-note { align-items: center; flex-wrap: wrap; }

.fields { display: flex; flex-direction: column; gap: 20px; }
.field { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.label { font-size: 14px; font-weight: 600; }
.label .muted { font-weight: 400; font-size: 13px; }
.input-row { display: flex; gap: 8px; }
.mono-input :deep(input) { font-family: var(--mono); }
.chips { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; min-height: 34px; }
.chip {
  height: 34px; max-width: 100%; display: inline-flex; align-items: center; gap: 6px; padding: 0 4px 0 10px; border-radius: 17px;
  background: var(--primary-bg); color: var(--primary-ink); font-size: 13px;
}
.chip-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.chip-x { width: 26px; height: 26px; display: flex; align-items: center; justify-content: center; border: 0; border-radius: 13px; background: transparent; color: inherit; cursor: pointer; }
.chip-x:hover { background: rgba(145, 85, 253, 0.16); }
.chip-add {
  height: 34px; display: inline-flex; align-items: center; gap: 4px; padding: 0 14px; border-radius: 17px;
  border: 1px dashed var(--primary); background: transparent; color: var(--primary-ink); font: inherit; font-size: 13px; cursor: pointer;
}
.chip-add:hover { background: var(--primary-row); }
.chip-add:disabled, .chip-x:disabled { opacity: 0.5; cursor: not-allowed; }
.chip-x:focus-visible, .chip-add:focus-visible { outline: 2px solid var(--primary); outline-offset: 2px; }

.opts { padding-top: 0; }
.set-row.col { flex-direction: column; align-items: stretch; }
.presets { display: flex; flex-direction: column; gap: 10px; }
.preset-actions { display: flex; flex-wrap: wrap; gap: 6px; }

.phases { display: flex; flex-wrap: wrap; gap: 8px 22px; margin: 0 0 14px; padding: 0; list-style: none; font-size: 14px; color: var(--ink-2); }
.phases li { display: flex; align-items: center; gap: 8px; }
.phases .num { width: 24px; height: 24px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; font-size: 12px; background: var(--chip); }
.phases .done { color: var(--ink); }
.phases .done .num { background: var(--primary-bg); color: var(--primary-ink); }
.phases .now { color: var(--primary-ink); font-weight: 600; }
.phases .now .num { background: var(--primary); color: var(--on-primary); box-shadow: 0 2px 6px rgba(145, 85, 253, 0.45); }
.dlg-foot { display: flex; justify-content: flex-end; gap: 8px; }
@media (max-width: 640px) {
  .start { width: 100%; }
}
</style>
