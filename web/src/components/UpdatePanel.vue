<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onMounted, ref } from 'vue'
import { NButton, NProgress, NSwitch, NTimePicker, useDialog, useMessage } from 'naive-ui'
import Icon from './Icon.vue'
import { api } from '../api'
import { updateStatus as status } from '../store'

const message = useMessage()
const dialog = useDialog()

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

// 打开时刷新一次；页面被缓存，再次进入时若没有在更新就重新检查
async function enter() {
  if (timer) return
  await refresh()
  if (status.value?.state.running) poll()
}
onMounted(enter)
onActivated(enter)
onBeforeUnmount(stopPoll)

const running = computed(() => !!status.value?.state.running || restarting.value)
const percent = computed(() => status.value?.state.progress?.percent ?? 0)
const fmt = (iso?: string) => (iso ? new Date(iso).toLocaleString('zh-CN', { hour12: false }) : '')
</script>

<template>
  <div class="upd">
    <section class="card">
      <div class="card-head">
        <h2 class="card-title">版本信息</h2>
        <span class="spacer" />
        <NButton size="small" secondary :loading="loading" :disabled="running" @click="refresh">
          <template #icon><Icon name="refresh" /></template>
          检查更新
        </NButton>
        <NButton
          size="small"
          type="primary"
          :disabled="!status?.docker.available || !status?.hasUpdate || running"
          @click="confirmUpdate"
        >
          <template #icon><Icon name="update" /></template>
          一键更新
        </NButton>
      </div>
      <div v-if="status" class="card-body body">
        <div class="rows">
          <div>
            <span class="avatar primary"><Icon name="info" :size="22" /></span>
            <span class="kv"><span>当前版本</span><b>v{{ status.current.version }}</b></span>
          </div>
          <div>
            <span class="avatar" :class="status.hasUpdate ? 'orange' : 'green'"><Icon name="update" :size="22" /></span>
            <span class="kv">
              <span>最新版本</span>
              <b v-if="status.latest">v{{ status.latest }}</b>
              <b v-else class="muted">未知</b>
            </span>
          </div>
          <div>
            <span class="avatar blue"><Icon name="link" :size="22" /></span>
            <span class="kv"><span>提交</span><b class="mono">{{ status.current.commit }}</b></span>
          </div>
          <div v-if="status.current.build_time">
            <span class="avatar teal"><Icon name="settings" :size="22" /></span>
            <span class="kv"><span>构建时间</span><b>{{ status.current.build_time }}</b></span>
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
      </div>
    </section>

    <section v-if="status" class="card">
      <div class="card-head">
        <h2 class="card-title">自动更新</h2>
        <span class="card-sub">需要挂载 docker.sock</span>
      </div>
      <div class="card-body">
        <div class="set-row">
          <div class="set-label">
            <b>每天自动检查并更新</b>
            <span>有新版本就自动拉取镜像并重建容器；失败后 6 小时内不重试</span>
          </div>
          <NSwitch v-model:value="autoEnabled" :disabled="!status.docker.available" aria-label="自动更新" @update:value="saveAuto" />
        </div>
        <div class="set-row">
          <div class="set-label">
            <b>检查时间</b>
            <span v-if="status.autoUpdate.enabled && status.autoUpdate.next">下次检查：{{ fmt(status.autoUpdate.next) }}</span>
            <span v-else>建议选在没有下载任务的深夜</span>
          </div>
          <NTimePicker
            v-model:formatted-value="autoTime"
            value-format="HH:mm"
            format="HH:mm"
            :disabled="!status.docker.available"
            style="width: 120px"
            @update:formatted-value="autoEnabled && saveAuto()"
          />
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.upd { display: flex; flex-direction: column; gap: 22px; }
.body { display: flex; flex-direction: column; gap: 14px; }
.rows { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px; }
.rows > div { display: flex; align-items: center; gap: 12px; min-width: 0; }
.kv { display: flex; flex-direction: column; min-width: 0; }
.kv span { font-size: 12px; color: var(--ink-2); }
.kv b { font-size: 15px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.small { font-size: 13px; }
.progress { display: flex; flex-direction: column; gap: 6px; }
.log { margin: 0; padding: 10px 12px; border-radius: 8px; background: var(--soft); font-size: 12px; white-space: pre-wrap; word-break: break-all; max-height: 220px; overflow: auto; }
</style>
