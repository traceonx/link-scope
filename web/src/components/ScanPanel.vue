<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { NButton, NCheckbox, NInput, NModal, NProgress, NSelect, useMessage } from 'naive-ui'
import Icon from './Icon.vue'
import FolderPicker from './FolderPicker.vue'
import { fmtCount, fmtTime } from '../format'
import { presetsPref } from '../prefs'
import type { AppConfig, DupMode, JobState, ScanPreset, ScanRequest, Summary } from '../types'

const props = defineProps<{
  cfg: AppConfig | null
  summary: Summary | null
  job: JobState
  canDelete: boolean
  pickerReload: number
}>()
const emit = defineEmits<{
  (e: 'start', req: ScanRequest): void
  (e: 'cancel'): void
  (e: 'delete-folder', path: string): void
}>()
const message = useMessage()

const form = reactive({
  source: '',
  targets: [] as string[],
  recursive: true,
  skipSmall: true,
  dupMode: 'quick' as DupMode,
})
const advanced = ref(false)
const running = computed(() => props.job.status === 'running')

// 首次拿到配置 / 上次扫描结果时填入表单（之后不再覆盖用户的修改）
let inited = false
watch(
  () => [props.cfg, props.summary] as const,
  ([cfg, last]) => {
    if (inited || !cfg) return
    inited = true
    form.source = last?.source || cfg.default_source
    form.targets = last?.targets?.length ? [...last.targets] : [...cfg.default_targets]
    if (last) {
      form.recursive = last.recursive
      form.skipSmall = last.min_size > 0
      form.dupMode = last.dup_mode
    }
  },
  { immediate: true },
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
  emit('start', {
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
const phaseIndex = computed(() => phases.findIndex((p) => p.key === props.job.phase))
const progressPct = computed(() => {
  const j = props.job
  if (j.phase === 'dups' && j.total) return Math.min(100, Math.round((j.done / j.total) * 100))
  return undefined
})
const progressText = computed(() => {
  const j = props.job
  if (j.phase === 'dups') return `已比对 ${fmtCount(j.done)} / ${fmtCount(j.total)}`
  if (j.phase === 'torrents') return '正在读取下载器中的种子…'
  return `已读取 ${fmtCount(j.done)} 个条目`
})
</script>

<template>
  <section class="card scan" aria-label="扫描设置">
    <div class="main-row">
      <div class="field grow">
        <label for="src" class="label">下载目录</label>
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
      <div class="scan-actions">
        <NButton v-if="!running" type="primary" size="large" @click="start">
          <template #icon><Icon name="search" /></template>
          开始扫描
        </NButton>
        <NButton v-else size="large" @click="emit('cancel')">取消扫描</NButton>
      </div>
    </div>

    <div class="field">
      <span class="label">媒体库目录 <span class="muted">（在这里查找硬链接，例如电视剧、电影、动漫）</span></span>
      <div class="chips">
        <span v-for="t in form.targets" :key="t" class="chip mono" :title="t">
          <span class="chip-text">{{ t }}</span>
          <button type="button" class="chip-x" :aria-label="`移除 ${t}`" :disabled="running" @click="removeTarget(t)">
            <Icon name="close" :size="12" />
          </button>
        </span>
        <button type="button" class="chip-add" :disabled="running" @click="pickTarget">
          <Icon name="plus" :size="14" />添加目录
        </button>
      </div>
    </div>

    <div class="foot">
      <button type="button" class="adv-toggle" :aria-expanded="advanced" @click="advanced = !advanced">
        <Icon name="chevronDown" :size="14" :class="{ rot: advanced }" />高级设置
      </button>
      <div class="presets">
        <NSelect
          :value="presetName"
          :options="presetOptions"
          placeholder="扫描预设"
          size="small"
          clearable
          :disabled="running || !presetOptions.length"
          class="preset-select"
          @update:value="applyPreset"
        />
        <NButton size="small" quaternary :disabled="running" @click="openSave">保存为预设</NButton>
        <NButton v-if="presetName" size="small" quaternary :disabled="running" @click="removePreset">删除预设</NButton>
      </div>
      <span class="last muted">
        <template v-if="summary">上次扫描 {{ fmtTime(summary.scanned_at) }} · 用时 {{ summary.duration }} 秒</template>
        <template v-else>还没有扫描过</template>
      </span>
    </div>

    <div v-if="advanced" class="advanced">
      <NCheckbox v-model:checked="form.recursive" :disabled="running">包含子文件夹</NCheckbox>
      <NCheckbox v-model:checked="form.skipSmall" :disabled="running">忽略小于 1 MB 的文件（.nfo、封面图等）</NCheckbox>
      <label class="inline">
        <span>重复副本检测</span>
        <NSelect v-model:value="form.dupMode" :options="dupOptions" :disabled="running" size="small" class="dup-select" />
      </label>
    </div>

    <div v-if="running" class="progress" role="status">
      <ol class="phases">
        <li
          v-for="(p, i) in phases"
          :key="p.key"
          :class="{ done: i < phaseIndex, now: i === phaseIndex }"
        >
          <span class="num">
            <Icon v-if="i < phaseIndex" name="check" :size="12" />
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
        :height="6"
      />
      <span class="muted small">{{ progressText }} · 扫描期间不能删除文件</span>
    </div>

    <slot />

    <FolderPicker
      v-model:show="picker.show"
      :title="picker.mode === 'source' ? '选择下载目录' : '添加媒体库目录'"
      :start="picker.start"
      :can-delete="canDelete"
      :reload-key="pickerReload"
      @select="onPicked"
      @delete="(p: string) => emit('delete-folder', p)"
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
        <span class="muted small">保存当前的下载目录、媒体库目录和高级设置，仅保存在这个浏览器中。</span>
      </label>
      <template #footer>
        <div class="dlg-foot">
          <NButton @click="saveDlg.show = false">取消</NButton>
          <NButton type="primary" :disabled="!saveDlg.name.trim()" @click="savePreset">保存</NButton>
        </div>
      </template>
    </NModal>
  </section>
</template>

<style scoped>
.scan { padding: 18px 20px; display: flex; flex-direction: column; gap: 14px; }
.main-row { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 12px 16px; }
.field { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.grow { flex: 1 1 420px; }
.label { font-size: 13px; font-weight: 600; }
.label .muted { font-weight: 400; }
.input-row { display: flex; gap: 8px; }
.mono-input :deep(input) { font-family: var(--mono); }
.scan-actions { display: flex; }
.chips { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; min-height: 32px; }
.chip { height: 30px; max-width: 100%; display: inline-flex; align-items: center; gap: 2px; padding: 0 4px 0 10px; border-radius: 6px; background: var(--chip); font-size: 13px; }
.chip-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.chip-x { width: 24px; height: 24px; display: flex; align-items: center; justify-content: center; border: 0; border-radius: 4px; background: transparent; color: var(--ink-2); cursor: pointer; }
.chip-x:hover { background: var(--border); }
.chip-add { height: 30px; display: inline-flex; align-items: center; gap: 4px; padding: 0 10px; border-radius: 6px; border: 1px dashed var(--border-strong); background: transparent; color: var(--blue); font: inherit; font-size: 13px; cursor: pointer; }
.chip-add:disabled, .chip-x:disabled { opacity: 0.5; cursor: not-allowed; }
.chip-x:focus-visible, .chip-add:focus-visible, .adv-toggle:focus-visible { outline: 2px solid var(--blue); outline-offset: 2px; }
.foot { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; padding-top: 12px; border-top: 1px solid var(--border); }
.adv-toggle { display: inline-flex; align-items: center; gap: 4px; border: 0; background: transparent; color: var(--ink-2); font: inherit; font-size: 13px; cursor: pointer; padding: 4px 0; }
.adv-toggle .rot { transform: rotate(180deg); }
.presets { display: flex; align-items: center; gap: 4px; }
.preset-select { width: 160px; }
.last { margin-left: auto; font-size: 12px; }
.advanced { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 24px; padding: 12px 14px; border-radius: 8px; background: var(--soft); }
.inline { display: flex; align-items: center; gap: 8px; font-size: 14px; }
.dup-select { width: 200px; }
.progress { display: flex; flex-direction: column; gap: 8px; }
.phases { display: flex; flex-wrap: wrap; gap: 6px 18px; margin: 0; padding: 0; list-style: none; font-size: 13px; color: var(--ink-2); }
.phases li { display: flex; align-items: center; gap: 6px; }
.phases .num { width: 18px; height: 18px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; font-size: 11px; background: var(--chip); }
.phases .done { color: var(--ink); }
.phases .done .num { background: var(--blue-bg); color: var(--blue-ink); }
.phases .now { color: var(--blue-ink); font-weight: 600; }
.phases .now .num { background: var(--blue); color: #fff; }
.small { font-size: 12px; }
.dlg-foot { display: flex; justify-content: flex-end; gap: 8px; }
@media (max-width: 640px) {
  .scan { padding: 14px; }
  .scan-actions, .scan-actions :deep(.n-button) { width: 100%; }
  .last { margin-left: 0; }
}
</style>
