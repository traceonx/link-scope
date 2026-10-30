<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { NButton, NEmpty, NInput, NModal, NSelect, NSpin, NSwitch, useDialog, useMessage } from 'naive-ui'
import Icon from './Icon.vue'
import { api } from '../api'
import { fmtTime } from '../format'
import type { Downloader, TorrentSummary } from '../types'

const props = defineProps<{ show: boolean; summary?: TorrentSummary | null; scanned: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'refreshed'): void; (e: 'changed', n: number): void }>()
const message = useMessage()
const dialog = useDialog()

const list = ref<Downloader[]>([])
const loading = ref(false)
const refreshing = ref(false)
const editing = ref(false)
const testing = ref(false)
const saving = ref(false)

const blank = (): Downloader => ({
  name: '',
  type: 'qbittorrent',
  url: 'http://',
  username: '',
  password: '',
  mappings: [{ from: '', to: '' }],
  enabled: true,
})
const form = reactive<Downloader>(blank())
const typeOptions = [
  { label: 'qBittorrent', value: 'qbittorrent' },
  { label: 'Transmission', value: 'transmission' },
]

async function load() {
  loading.value = true
  try {
    list.value = await api.downloaders()
    emit('changed', list.value.filter((d) => d.enabled).length)
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}
watch(
  () => props.show,
  (v) => {
    if (v) {
      editing.value = false
      load()
    }
  },
)

function edit(d?: Downloader) {
  Object.assign(form, blank(), d ? JSON.parse(JSON.stringify(d)) : {}, { password: '' })
  if (!form.mappings.length) form.mappings.push({ from: '', to: '' })
  editing.value = true
}

function payload(): Downloader {
  return { ...form, mappings: form.mappings.filter((m) => m.from.trim() || m.to.trim()) }
}

async function test() {
  testing.value = true
  try {
    const r = await api.testDownloader(payload())
    message.success(`连接成功，共 ${r.torrents} 个种子`)
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    testing.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await api.saveDownloader(payload())
    message.success('已保存')
    editing.value = false
    await load()
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}

function remove(d: Downloader) {
  dialog.warning({
    title: '删除下载器',
    content: `删除「${d.name}」的连接配置？不会影响下载器里的种子。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      await api.deleteDownloader(d.id!)
      await load()
    },
  })
}

async function toggle(d: Downloader, v: boolean) {
  try {
    await api.saveDownloader({ ...d, enabled: v, password: '' })
    await load()
  } catch (e) {
    message.error((e as Error).message)
  }
}

async function refresh() {
  refreshing.value = true
  try {
    const s = await api.refreshTorrents()
    message.success(`已更新：做种 ${s.seeding} 个文件，已删种 ${s.unregistered}，孤儿 ${s.orphans}`)
    emit('refreshed')
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    refreshing.value = false
  }
}

function clientError(name: string) {
  return props.summary?.clients.find((c) => c.name === name && !c.ok)?.error
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="editing ? (form.id ? '编辑下载器' : '添加下载器') : '下载器'"
    style="width: min(640px, calc(100vw - 24px))"
    :bordered="false"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <!-- 列表 -->
    <div v-if="!editing" class="dl">
      <p class="muted small intro">
        连接 qBittorrent / Transmission 后，扫描结束会自动读取种子，标出每个文件来自哪个站点、是否在做种、是否已被站点删种，以及下载目录里不属于任何种子的孤儿文件。
      </p>
      <NSpin :show="loading">
        <div class="list">
          <div v-for="d in list" :key="d.id" class="row">
            <div class="body">
              <div class="name">
                <span>{{ d.name }}</span>
                <span class="badge badge-gray">{{ d.type === 'qbittorrent' ? 'qBittorrent' : 'Transmission' }}</span>
              </div>
              <span class="mono small muted">{{ d.url }}</span>
              <span v-for="m in d.mappings" :key="m.from" class="mono small muted">{{ m.from }} → {{ m.to }}</span>
              <span v-if="clientError(d.name)" class="small c-red">上次读取失败：{{ clientError(d.name) }}</span>
            </div>
            <NSwitch :value="d.enabled" :aria-label="`启用 ${d.name}`" @update:value="(v: boolean) => toggle(d, v)" />
            <NButton size="small" quaternary @click="edit(d)">编辑</NButton>
            <NButton size="small" quaternary type="error" @click="remove(d)">删除</NButton>
          </div>
          <NEmpty v-if="!loading && !list.length" description="还没有添加下载器" style="padding: 32px 0" />
        </div>
      </NSpin>
      <div v-if="summary" class="clients">
        <span class="muted small">上次读取种子：{{ fmtTime(summary.fetched_at) }}</span>
        <div v-for="c in summary.clients" :key="c.name" class="client small">
          <b>{{ c.name }}</b>
          <template v-if="!c.ok"><span class="c-red">读取失败：{{ c.error }}</span></template>
          <template v-else>
            <span>{{ c.torrents }} 个种子 · 对上 {{ c.matched ?? 0 }} 个扫描到的文件</span>
            <span v-if="c.torrents && !c.matched" class="c-orange">
              一个文件也没对上，请检查路径映射。下载器里的文件路径示例：<span class="mono">{{ c.sample || '（无）' }}</span>，它应该位于扫描的下载目录内。
            </span>
          </template>
        </div>
      </div>
    </div>

    <!-- 编辑 -->
    <div v-else class="form">
      <div class="grid">
        <label class="f">
          <span class="label">名称</span>
          <NInput v-model:value="form.name" placeholder="如 qB-主力" />
        </label>
        <label class="f">
          <span class="label">类型</span>
          <NSelect v-model:value="form.type" :options="typeOptions" />
        </label>
      </div>
      <label class="f">
        <span class="label">地址</span>
        <NInput
          v-model:value="form.url"
          class="mono-input"
          :placeholder="form.type === 'qbittorrent' ? 'http://192.168.1.10:8080' : 'http://192.168.1.10:9091'"
        />
        <span class="muted small">WebUI 地址。容器内访问 NAS 本机服务请填 NAS 的局域网 IP，不要填 127.0.0.1。</span>
      </label>
      <div class="grid">
        <label class="f">
          <span class="label">用户名</span>
          <NInput v-model:value="form.username" :input-props="{ autocomplete: 'off' }" />
        </label>
        <label class="f">
          <span class="label">密码</span>
          <NInput
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            :placeholder="form.id ? '留空表示不修改' : ''"
            :input-props="{ autocomplete: 'new-password' }"
          />
        </label>
      </div>
      <div class="f">
        <span class="label">路径映射 <span class="muted">（左：下载器里的保存路径，如 /video；右：LinkScope 里的路径，如 /vol2/1000/video）</span></span>
        <div v-for="(m, i) in form.mappings" :key="i" class="map">
          <NInput v-model:value="m.from" class="mono-input" placeholder="下载器里的路径，如 /video" :input-props="{ 'aria-label': '下载器中的路径' }" />
          <Icon name="arrow" />
          <NInput v-model:value="m.to" class="mono-input" placeholder="LinkScope 里的路径，如 /vol2/1000/video" :input-props="{ 'aria-label': '本工具中的路径' }" />
          <NButton quaternary size="small" aria-label="删除这条映射" @click="form.mappings.splice(i, 1)">×</NButton>
        </div>
        <NButton dashed size="small" @click="form.mappings.push({ from: '', to: '' })">
          <template #icon><Icon name="plus" /></template>
          添加映射
        </NButton>
        <span class="muted small">下载器也运行在 Docker 里时，它看到的保存路径（如 /downloads）通常和这里不同，需要映射后才能对上文件。路径一致则不用填。两边填反时会自动纠正。</span>
      </div>
    </div>

    <template #footer>
      <div class="foot">
        <template v-if="!editing">
          <NButton :disabled="!scanned || !list.length" :loading="refreshing" @click="refresh">
            <template #icon><Icon name="refresh" /></template>
            重新读取种子
          </NButton>
          <span class="spacer" />
          <NButton type="primary" @click="edit()">
            <template #icon><Icon name="plus" /></template>
            添加下载器
          </NButton>
        </template>
        <template v-else>
          <NButton @click="editing = false">返回</NButton>
          <span class="spacer" />
          <NButton :loading="testing" @click="test">测试连接</NButton>
          <NButton type="primary" :loading="saving" @click="save">保存</NButton>
        </template>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.clients { display: flex; flex-direction: column; gap: 6px; }
.client { display: flex; flex-wrap: wrap; gap: 4px 8px; }
.client .mono { word-break: break-all; }
.c-red { color: var(--red-ink); }
.c-orange { color: var(--orange-ink); }
.dl, .form { display: flex; flex-direction: column; gap: 12px; }
.intro { margin: 0; line-height: 20px; }
.small { font-size: 12px; }
.c-red { color: var(--red-ink); }
.list { border: 1px solid var(--border); border-radius: 8px; min-height: 60px; }
.row { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border-bottom: 1px solid var(--border); }
.row:last-child { border-bottom: 0; }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; word-break: break-all; }
.name { display: flex; align-items: center; gap: 6px; font-size: 14px; font-weight: 600; }
.grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.f { display: flex; flex-direction: column; gap: 6px; }
.label { font-size: 13px; font-weight: 600; }
.label .muted { font-weight: 400; }
.map { display: flex; align-items: center; gap: 8px; }
.mono-input :deep(input) { font-family: var(--mono); }
.foot { display: flex; flex-wrap: wrap; gap: 8px; }
.spacer { flex: 1; }
@media (max-width: 560px) { .grid { grid-template-columns: 1fr; } }
</style>
