<script setup lang="ts">
import { computed, onActivated, reactive, ref } from 'vue'
import { NButton, NEmpty, NInput, NModal, NSelect, NSpin, NSwitch, useDialog, useMessage } from 'naive-ui'
import Icon from '../components/Icon.vue'
import { api } from '../api'
import { fmtTime } from '../format'
import { downloaderCount, loadResult, summary } from '../store'
import type { Downloader } from '../types'

const message = useMessage()
const dialog = useDialog()

const list = ref<Downloader[]>([])
const loading = ref(false)
const refreshing = ref(false)
const editing = ref(false)
const testing = ref(false)
const saving = ref(false)
const ts = computed(() => summary.value?.torrents)

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
    downloaderCount.value = list.value.filter((d) => d.enabled).length
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}
onActivated(load)

function edit(d?: Downloader) {
  Object.assign(form, blank(), d ? JSON.parse(JSON.stringify(d)) : {}, { password: '' })
  if (!d) delete form.id
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
    await loadResult()
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    refreshing.value = false
  }
}

function clientOf(name: string) {
  return ts.value?.clients.find((c) => c.name === name)
}
</script>

<template>
  <div class="page">
    <section class="card intro">
      <span class="avatar primary"><Icon name="downloader" :size="24" /></span>
      <div class="intro-text">
        <h2 class="card-title">连接下载器</h2>
        <p class="muted">
          连接 qBittorrent / Transmission 后，扫描结束会自动读取种子，标出每个文件来自哪个站点、是否在做种、是否已被站点删种，以及下载目录里不属于任何种子的孤儿文件。
        </p>
        <span v-if="ts" class="muted small">上次读取种子：{{ fmtTime(ts.fetched_at) }}</span>
      </div>
      <div class="intro-actions">
        <NButton :disabled="!summary || !list.length" :loading="refreshing" secondary @click="refresh">
          <template #icon><Icon name="refresh" /></template>
          重新读取种子
        </NButton>
        <NButton type="primary" @click="edit()">
          <template #icon><Icon name="plus" /></template>
          添加下载器
        </NButton>
      </div>
    </section>

    <NSpin :show="loading">
      <div v-if="list.length" class="dl-grid">
        <section v-for="d in list" :key="d.id" class="card dl" :class="{ off: !d.enabled }">
          <div class="dl-head">
            <span class="avatar" :class="d.type === 'qbittorrent' ? 'blue' : 'green'">
              <Icon name="server" :size="22" />
            </span>
            <div class="dl-title">
              <b>{{ d.name }}</b>
              <span class="badge badge-gray">{{ d.type === 'qbittorrent' ? 'qBittorrent' : 'Transmission' }}</span>
            </div>
            <span class="spacer" />
            <NSwitch :value="d.enabled" :aria-label="`启用 ${d.name}`" @update:value="(v: boolean) => toggle(d, v)" />
          </div>
          <div class="dl-body">
            <div class="kv"><span>地址</span><span class="mono">{{ d.url }}</span></div>
            <div class="kv">
              <span>路径映射</span>
              <span>
                <span v-for="m in d.mappings" :key="m.from" class="mono map-line">{{ m.from }} → {{ m.to }}</span>
                <span v-if="!d.mappings.length" class="muted">无（路径一致）</span>
              </span>
            </div>
            <template v-if="clientOf(d.name)">
              <div v-if="!clientOf(d.name)!.ok" class="notice notice-err">上次读取失败：{{ clientOf(d.name)!.error }}</div>
              <template v-else>
                <div class="kv">
                  <span>种子匹配</span>
                  <span>{{ clientOf(d.name)!.torrents }} 个种子 · 对上 {{ clientOf(d.name)!.matched ?? 0 }} 个扫描到的文件</span>
                </div>
                <div v-if="clientOf(d.name)!.torrents && !clientOf(d.name)!.matched" class="notice notice-warn">
                  <span>
                    一个文件也没对上，请检查路径映射。下载器里的文件路径示例：<span class="mono">{{ clientOf(d.name)!.sample || '（无）' }}</span>，它应该位于扫描的下载目录内。
                  </span>
                </div>
              </template>
            </template>
          </div>
          <div class="card-foot">
            <span class="spacer" />
            <NButton size="small" quaternary type="error" @click="remove(d)">删除</NButton>
            <NButton size="small" secondary type="primary" @click="edit(d)">编辑</NButton>
          </div>
        </section>
      </div>
      <section v-else-if="!loading" class="card empty">
        <NEmpty description="还没有添加下载器">
          <template #extra>
            <NButton type="primary" size="small" @click="edit()">添加下载器</NButton>
          </template>
        </NEmpty>
      </section>
    </NSpin>

    <NModal
      v-model:show="editing"
      preset="card"
      :title="form.id ? '编辑下载器' : '添加下载器'"
      style="width: min(640px, calc(100vw - 24px))"
      :bordered="false"
    >
      <div class="form">
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
            <NButton quaternary circle size="small" aria-label="删除这条映射" @click="form.mappings.splice(i, 1)">
              <template #icon><Icon name="close" /></template>
            </NButton>
          </div>
          <NButton dashed size="small" class="add-map" @click="form.mappings.push({ from: '', to: '' })">
            <template #icon><Icon name="plus" /></template>
            添加映射
          </NButton>
          <span class="muted small">下载器也运行在 Docker 里时，它看到的保存路径（如 /downloads）通常和这里不同，需要映射后才能对上文件。路径一致则不用填。两边填反时会自动纠正。</span>
        </div>
      </div>
      <template #footer>
        <div class="foot">
          <NButton @click="editing = false">取消</NButton>
          <span class="spacer" />
          <NButton :loading="testing" @click="test">测试连接</NButton>
          <NButton type="primary" :loading="saving" @click="save">保存</NButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
p { margin: 0; }
.intro { display: flex; flex-wrap: wrap; align-items: flex-start; gap: 14px 16px; padding: 20px; }
.intro-text { flex: 1 1 360px; display: flex; flex-direction: column; gap: 6px; font-size: 14px; line-height: 22px; }
.intro-actions { display: flex; flex-wrap: wrap; gap: 8px; align-self: center; }
.dl-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 22px; }
@media (max-width: 480px) { .dl-grid { grid-template-columns: 1fr; } }
.dl { display: flex; flex-direction: column; }
.dl.off { opacity: 0.7; }
.dl-head { display: flex; align-items: center; gap: 12px; padding: 18px 20px 8px; }
.dl-title { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; min-width: 0; }
.dl-title b { font-size: 16px; word-break: break-all; }
.dl-body { flex: 1; display: flex; flex-direction: column; gap: 10px; padding: 10px 20px 16px; font-size: 13px; }
.kv { display: grid; grid-template-columns: 72px 1fr; gap: 10px; }
.kv > span:first-child { color: var(--ink-2); }
.kv .mono, .map-line { word-break: break-all; }
.map-line { display: block; }
.empty { padding: 48px 16px; }
.form { display: flex; flex-direction: column; gap: 14px; }
.grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.f { display: flex; flex-direction: column; gap: 6px; }
.label { font-size: 13px; font-weight: 600; }
.label .muted { font-weight: 400; }
.map { display: flex; align-items: center; gap: 8px; color: var(--ink-2); }
.add-map { align-self: flex-start; }
.mono-input :deep(input) { font-family: var(--mono); }
.foot { display: flex; flex-wrap: wrap; gap: 8px; }
@media (max-width: 560px) { .grid { grid-template-columns: 1fr; } }
</style>
