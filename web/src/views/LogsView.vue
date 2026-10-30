<script setup lang="ts">
import { computed, nextTick, onActivated, onDeactivated, ref, watch } from 'vue'
import { NButton, NInput, NRadioButton, NRadioGroup, NSwitch, NTooltip } from 'naive-ui'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import type { LogEntry } from '../types'

// 实时日志（参考 MoviePilot）：页面打开时每 2 秒增量拉取，按级别着色，可筛选、暂停、自动滚动、下载
const MAX = 2000
const entries = ref<LogEntry[]>([])
const last = ref(0)
const error = ref('')
const paused = ref(false)
const follow = ref(true)
const level = ref<'all' | 'WARN' | 'ERROR'>('all')
const q = ref('')
const box = ref<HTMLElement | null>(null)

async function pull() {
  try {
    const r = await api.logs(last.value)
    error.value = ''
    if (r.last < last.value) entries.value = [] // 服务重启后 ID 重新计数
    last.value = r.last
    if (!r.items.length) return
    entries.value = [...entries.value, ...r.items].slice(-MAX)
    if (follow.value) {
      await nextTick()
      toBottom()
    }
  } catch (e) {
    error.value = (e as Error).message
  }
}

let timer: ReturnType<typeof setInterval> | null = null
function start() {
  stop()
  void pull()
  timer = setInterval(() => !paused.value && pull(), 2000)
}
function stop() {
  if (timer) clearInterval(timer)
  timer = null
}
onActivated(start)
onDeactivated(stop)

const levelRank = { INFO: 0, WARN: 1, ERROR: 2 } as const
const shown = computed(() => {
  const min = level.value === 'all' ? 0 : levelRank[level.value]
  const kw = q.value.trim().toLowerCase()
  return entries.value.filter((e) => levelRank[e.level] >= min && (!kw || e.msg.toLowerCase().includes(kw)))
})
const counts = computed(() => ({
  WARN: entries.value.filter((e) => e.level === 'WARN').length,
  ERROR: entries.value.filter((e) => e.level === 'ERROR').length,
}))
watch([level, q], async () => {
  await nextTick()
  if (follow.value) toBottom()
})

function toBottom() {
  const el = box.value
  if (el) el.scrollTop = el.scrollHeight
}
// 手动向上滚动时暂停跟随，回到底部时恢复
function onScroll() {
  const el = box.value
  if (!el) return
  follow.value = el.scrollHeight - el.scrollTop - el.clientHeight < 24
}

const pad = (n: number) => String(n).padStart(2, '0')
function fmt(ms: number) {
  const d = new Date(ms)
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
const levelLabel = { INFO: '信息', WARN: '警告', ERROR: '错误' } as const

function clearView() {
  entries.value = []
}
function download() {
  const text = entries.value.map((e) => `${new Date(e.time).toLocaleString('zh-CN', { hour12: false })} [${e.level}] ${e.msg}`).join('\n')
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([text + '\n'], { type: 'text/plain;charset=utf-8' }))
  a.download = `linkscope-${new Date().toISOString().slice(0, 10)}.log`
  a.click()
  URL.revokeObjectURL(a.href)
}
</script>

<template>
  <div class="page">
    <section class="card logs" aria-label="实时日志">
      <div class="toolbar">
        <NRadioGroup v-model:value="level" aria-label="日志级别">
          <NRadioButton value="all">全部</NRadioButton>
          <NRadioButton value="WARN">警告及以上<template v-if="counts.WARN + counts.ERROR"> · {{ counts.WARN + counts.ERROR }}</template></NRadioButton>
          <NRadioButton value="ERROR">错误<template v-if="counts.ERROR"> · {{ counts.ERROR }}</template></NRadioButton>
        </NRadioGroup>
        <NInput v-model:value="q" clearable class="search" placeholder="搜索日志" :input-props="{ 'aria-label': '搜索日志' }">
          <template #prefix><Icon name="search" /></template>
        </NInput>
        <span class="spacer" />
        <label class="follow">
          <NSwitch v-model:value="follow" size="small" aria-label="自动滚动" @update:value="(v: boolean) => v && toBottom()" />
          自动滚动
        </label>
        <NButton secondary :type="paused ? 'primary' : 'default'" @click="paused = !paused">
          <template #icon><Icon :name="paused ? 'play' : 'pause'" /></template>
          {{ paused ? '继续' : '暂停' }}
        </NButton>
        <NTooltip>
          <template #trigger>
            <NButton quaternary circle aria-label="清屏" @click="clearView">
              <template #icon><Icon name="clear" /></template>
            </NButton>
          </template>
          清屏（只清除页面显示）
        </NTooltip>
        <NTooltip>
          <template #trigger>
            <NButton quaternary circle aria-label="下载日志" @click="download">
              <template #icon><Icon name="download" /></template>
            </NButton>
          </template>
          下载当前日志
        </NTooltip>
      </div>

      <div v-if="error" class="notice notice-err err">无法读取日志：{{ error }}</div>

      <div ref="box" class="console mono" role="log" aria-live="off" @scroll="onScroll">
        <div v-for="e in shown" :key="e.id" class="line" :class="e.level">
          <span class="time">{{ fmt(e.time) }}</span>
          <span class="lv">{{ levelLabel[e.level] }}</span>
          <span class="msg">{{ e.msg }}</span>
        </div>
        <div v-if="!shown.length" class="empty">{{ entries.length ? '没有符合条件的日志' : '暂无日志' }}</div>
      </div>

      <div class="foot">
        <span class="muted small">
          <span class="dot" :class="{ off: paused }" />{{ paused ? '已暂停' : '实时更新中' }} · 显示 {{ shown.length }} / {{ entries.length }} 条 · 服务端保留最近 2000 条，完整日志见容器日志
        </span>
        <span class="spacer" />
        <NButton v-if="!follow" size="tiny" secondary type="primary" @click="toBottom">
          <template #icon><Icon name="toBottom" :size="14" /></template>
          回到最新
        </NButton>
      </div>
    </section>
  </div>
</template>

<style scoped>
@media (min-width: 641px) {
  .page { height: calc(100dvh - 112px); min-height: 480px; }
  .logs { flex: 1; min-height: 0; }
}
.logs { display: flex; flex-direction: column; overflow: hidden; }
.toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 12px; padding: 16px 20px; border-bottom: 1px solid var(--border); }
.search { width: 240px; max-width: 100%; }
.follow { display: inline-flex; align-items: center; gap: 8px; font-size: 13px; color: var(--ink-2); cursor: pointer; }
.err { margin: 12px 20px 0; }
.console { flex: 1; min-height: 0; overflow: auto; padding: 10px 0; background: var(--soft); font-size: 12.5px; line-height: 22px; }
.line { display: flex; gap: 12px; padding: 0 20px; }
.line:hover { background: var(--chip); }
.time { flex-shrink: 0; color: var(--ink-3); }
.lv { flex-shrink: 0; width: 40px; text-align: center; border-radius: 4px; font-size: 11px; font-weight: 600; line-height: 18px; margin-top: 2px; height: 18px; }
.INFO .lv { background: var(--blue-bg); color: var(--blue-ink); }
.WARN .lv { background: var(--orange-bg); color: var(--orange-ink); }
.ERROR .lv { background: var(--red-bg); color: var(--red-ink); }
.WARN .msg { color: var(--orange-ink); }
.ERROR .msg { color: var(--red-ink); }
.msg { flex: 1; min-width: 0; white-space: pre-wrap; word-break: break-all; }
.empty { padding: 48px 20px; text-align: center; color: var(--ink-2); font-family: var(--sans); font-size: 14px; }
.foot { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 10px 20px; border-top: 1px solid var(--border); }
.dot { display: inline-block; width: 8px; height: 8px; margin-right: 6px; border-radius: 50%; background: var(--green); }
.dot.off { background: var(--ink-3); }
@media (max-width: 640px) {
  .console { height: 65vh; flex: none; }
  .toolbar, .foot { padding-left: 14px; padding-right: 14px; }
  .line { padding: 0 14px; flex-wrap: wrap; gap: 0 8px; }
  .msg { flex-basis: 100%; }
  .search { width: 100%; }
}
</style>
