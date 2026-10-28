<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { NButton, NModal, NProgress, NSwitch, NTimePicker, useDialog, useMessage } from 'naive-ui'
import { api } from '../api'
import type { UpdateStatus } from '../types'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'status', s: UpdateStatus): void }>()
const message = useMessage()
const dialog = useDialog()

const status = ref<UpdateStatus | null>(null)
const loading = ref(false)
const restarting = ref(false)
const autoTime = ref<string | null>('04:00')
let restartAt = 0
const autoEnabled = ref(false)

async function refresh() {
  loading.value = true
  try {
    status.value = await api.updateStatus()
    autoEnabled.value = status.value.autoUpdate.enabled
    autoTime.value = status.value.autoUpdate.time
    emit('status', status.value)
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

let timer: ReturnType<typeof setInterval> | null = null
function stopPoll() {
  if (timer) clearInterval(timer)
  timer = null
}

// 更新中轮询状态；进入重建阶段后改为等待服务以新版本重启，然后刷新页面
function poll() {
  stopPoll()
  const from = status.value?.current.version
  timer = setInterval(async () => {
    try {
      if (restarting.value) {
        const h = await api.health()
        if (h.version !== from || Date.now() - restartAt > 15000) {
          stopPoll()
          location.reload()
        }
        return
      }
      const s = await api.updateStatus()
      status.value = s
      if (s.state.phase === 'restarting') {
        restarting.value = true
        restartAt = Date.now()
      } else if (!s.state.running) {
        stopPoll()
        if (s.state.success) message.success(s.state.message || '更新完成')
        else message.error(s.state.message || '更新失败')
      }
    } catch {
      // 服务重启期间请求会失败，继续等待
      if (!restarting.value) {
        restarting.value = true
        restartAt = Date.now()
      }
    }
  }, 1500)
}

function confirmUpdate() {
  const s = status.value
  dialog.warning({
    title: '一键更新',
    content: `将拉取 ${s?.docker.image ?? '最新镜像'} 并重建容器，期间服务会重启约 10–30 秒。正在进行的扫描会中断。`,
    positiveText: '开始更新',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        if (status.value) status.value.state = await api.startUpdate()
        poll()
      } catch (e) {
        message.error((e as Error).message)
      }
    },
  })
}

async function saveAuto() {
  try {
    const a = await api.setAutoUpdate(autoEnabled.value, autoTime.value ?? '04:00')
    if (status.value) status.value.autoUpdate = a
    message.success(a.enabled ? `已开启，每天 ${a.time} 检查` : '已关闭自动更新')
  } catch (e) {
    message.error((e as Error).message)
    autoEnabled.value = status.value?.autoUpdate.enabled ?? false
  }
}

watch(
  () => props.show,
  async (v) => {
    if (!v) return
    await refresh()
    if (status.value?.state.running) poll()
  },
)
onBeforeUnmount(stopPoll)

const running = computed(() => !!status.value?.state.running || restarting.value)
const percent = computed(() => status.value?.state.progress?.percent ?? 0)
const fmt = (iso?: string) => (iso ? new Date(iso).toLocaleString('zh-CN', { hour12: false }) : '')
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="版本与更新"
    style="width: min(520px, calc(100vw - 24px))"
    :bordered="false"
    :mask-closable="!running"
    :closable="!running"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <div v-if="status" class="upd">
      <div class="rows">
        <div><span>当前版本</span><b>v{{ status.current.version }}</b></div>
        <div><span>提交</span><b class="mono">{{ status.current.commit }}</b></div>
        <div v-if="status.current.build_time"><span>构建时间</span><b>{{ status.current.build_time }}</b></div>
        <div>
          <span>最新版本</span>
          <b v-if="status.latest">v{{ status.latest }}</b>
          <b v-else class="muted">未知</b>
        </div>
      </div>

      <div v-if="status.hasUpdate && !running" class="notice notice-info">
        发现新{{ status.latest && status.latest !== status.current.version ? `版本 v${status.latest}` : '镜像' }}，可以一键更新。
      </div>
      <div v-else-if="!running && status.latest && !status.hasUpdate" class="muted small">已是最新版本。</div>
      <div v-if="status.latestError" class="muted small">{{ status.latestError }}</div>
      <div v-if="!status.docker.available" class="notice notice-warn">{{ status.docker.reason }}</div>

      <div v-if="running" class="progress">
        <NProgress
          type="line"
          :percentage="restarting ? 100 : percent"
          :processing="restarting || status.state.phase !== 'pull'"
          :height="8"
        />
        <span class="small">{{ restarting ? '正在重启服务，完成后页面会自动刷新…' : status.state.message }}</span>
      </div>
      <div
        v-else-if="status.state.message && status.state.finishedAt"
        class="notice"
        :class="status.state.success ? 'notice-info' : 'notice-err'"
      >
        {{ status.state.message }}（{{ fmt(status.state.finishedAt) }}）
      </div>
      <pre v-if="status.state.log && !running" class="log mono">{{ status.state.log }}</pre>

      <div class="auto">
        <div class="auto-head">
          <span class="label">自动更新</span>
          <NSwitch v-model:value="autoEnabled" :disabled="!status.docker.available" @update:value="saveAuto" />
        </div>
        <div class="auto-body">
          每天
          <NTimePicker
            v-model:formatted-value="autoTime"
            value-format="HH:mm"
            format="HH:mm"
            size="small"
            :disabled="!status.docker.available"
            style="width: 110px"
            @update:formatted-value="autoEnabled && saveAuto()"
          />
          检查一次，有新版本就自动更新
        </div>
        <div v-if="status.autoUpdate.enabled && status.autoUpdate.next" class="muted small">
          下次检查：{{ fmt(status.autoUpdate.next) }}
        </div>
      </div>
    </div>
    <template #footer>
      <div class="foot">
        <NButton :loading="loading" :disabled="running" @click="refresh">检查更新</NButton>
        <NButton
          type="primary"
          :disabled="!status?.docker.available || !status?.hasUpdate || running"
          @click="confirmUpdate"
        >
          一键更新
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.upd { display: flex; flex-direction: column; gap: 14px; }
.rows { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 14px; border-radius: 8px; background: var(--soft); }
.rows div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.rows span { font-size: 12px; color: var(--ink-2); }
.rows b { font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; }
.small { font-size: 13px; }
.progress { display: flex; flex-direction: column; gap: 6px; }
.log { margin: 0; padding: 10px 12px; border-radius: 8px; background: var(--soft); font-size: 12px; white-space: pre-wrap; word-break: break-all; max-height: 180px; overflow: auto; }
.auto { display: flex; flex-direction: column; gap: 8px; padding-top: 12px; border-top: 1px solid var(--border); }
.auto-head { display: flex; align-items: center; justify-content: space-between; }
.label { font-size: 14px; font-weight: 600; }
.auto-body { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-size: 14px; }
.foot { display: flex; justify-content: flex-end; gap: 8px; }
</style>
