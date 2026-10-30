<script setup lang="ts">
import { computed } from 'vue'
import { NRadioButton, NRadioGroup, NTab, NTabs } from 'naive-ui'
import Icon from '../components/Icon.vue'
import UpdatePanel from '../components/UpdatePanel.vue'
import { navigate, route } from '../router'
import { themePref, viewPref } from '../prefs'
import { cfg, hasUpdate } from '../store'

// 设置拆成标签页：外观与显示 / 运行配置 / 版本与更新（#/settings/update 可直接打开）
type Tab = 'general' | 'runtime' | 'update'
const tab = computed<Tab>(() => (['general', 'runtime', 'update'].includes(route.value.sub) ? (route.value.sub as Tab) : 'general'))
function setTab(t: string) {
  navigate('settings', t === 'general' ? '' : t)
}

const envs = computed(() => {
  const c = cfg.value
  if (!c) return []
  return [
    { name: 'LS_ROOTS', label: '允许访问的目录', value: c.roots.length ? c.roots.join('、') : '不限制', mono: true },
    { name: 'LS_DEFAULT_SOURCE', label: '默认下载目录', value: c.default_source || '未设置', mono: true },
    { name: 'LS_DEFAULT_TARGETS', label: '默认媒体库目录', value: c.default_targets.length ? c.default_targets.join('、') : '未设置', mono: true },
    {
      name: 'LS_DELETE',
      label: '清理功能',
      value: c.can_delete ? `可用 · 回收站保留 ${c.trash_days} 天（LS_TRASH_DAYS）` : `不可用：${c.delete_reason}`,
      mono: false,
    },
  ]
})
</script>

<template>
  <div class="page">
    <NTabs :value="tab" type="line" class="tabs" @update:value="setTab">
      <NTab name="general">
        <span class="tab"><Icon name="palette" :size="18" />外观与显示</span>
      </NTab>
      <NTab name="runtime">
        <span class="tab"><Icon name="tune" :size="18" />运行配置</span>
      </NTab>
      <NTab name="update">
        <span class="tab"><Icon name="update" :size="18" />版本与更新<span v-if="hasUpdate" class="dot" /></span>
      </NTab>
    </NTabs>

    <template v-if="tab === 'general'">
      <section class="card">
        <div class="card-head">
          <h2 class="card-title">外观</h2>
          <span class="card-sub">保存在当前浏览器</span>
        </div>
        <div class="card-body">
          <div class="themes" role="radiogroup" aria-label="主题">
            <button
              v-for="t in [
                { v: 'system', label: '跟随系统', icon: 'system' },
                { v: 'light', label: '浅色', icon: 'light' },
                { v: 'dark', label: '深色', icon: 'dark' },
              ] as const"
              :key="t.v"
              type="button"
              role="radio"
              class="theme"
              :class="[t.v, { on: themePref === t.v }]"
              :aria-checked="themePref === t.v"
              @click="themePref = t.v"
            >
              <span class="preview" aria-hidden="true">
                <span class="pv-nav" />
                <span class="pv-main"><span class="pv-card" /><span class="pv-card short" /></span>
              </span>
              <span class="theme-label"><Icon :name="t.icon" :size="18" />{{ t.label }}</span>
            </button>
          </div>
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <h2 class="card-title">显示</h2>
        </div>
        <div class="card-body">
          <div class="set-row">
            <div class="set-label"><b>清理结果默认显示方式</b><span>打开页面时的结果视图，浏览器会记住上次的选择</span></div>
            <NRadioGroup v-model:value="viewPref" aria-label="默认显示方式">
              <NRadioButton value="folders">按文件夹</NRadioButton>
              <NRadioButton value="files">按文件</NRadioButton>
            </NRadioGroup>
          </div>
        </div>
      </section>
    </template>

    <template v-else-if="tab === 'runtime'">
      <section v-if="cfg" class="card">
        <div class="card-head">
          <h2 class="card-title">运行配置</h2>
          <span class="card-sub">由 docker-compose 中的环境变量决定，修改后重建容器生效</span>
        </div>
        <div class="card-body">
          <div v-for="e in envs" :key="e.name" class="set-row env">
            <div class="set-label">
              <b>{{ e.label }}</b>
              <span class="mono">{{ e.name }}</span>
            </div>
            <span class="env-val" :class="{ mono: e.mono }">{{ e.value }}</span>
          </div>
        </div>
      </section>
      <section class="card">
        <div class="card-head">
          <h2 class="card-title">其他说明</h2>
        </div>
        <div class="card-body notes">
          <p><b>登录保护：</b>设置 <span class="mono">LS_AUTH_USER</span> / <span class="mono">LS_AUTH_PASS</span> 后访问页面需要登录（浏览器弹窗）。</p>
          <p><b>只读挂载：</b>媒体目录以 <span class="mono">:ro</span> 挂载或 <span class="mono">LS_DELETE=false</span> 时，清理与回收站会自动隐藏。</p>
          <p><b>出站代理：</b>环境变量 <span class="mono">PROXY_HOST</span>，目前只用于查询 Docker Hub 上的最新版本。</p>
          <p><b>数据目录：</b><span class="mono">/app/data</span>，保存扫描结果、下载器配置、回收站位置和更新状态。</p>
        </div>
      </section>
    </template>

    <UpdatePanel v-else />
  </div>
</template>

<style scoped>
p { margin: 0; }
.tabs :deep(.n-tabs-nav) { border-bottom: 1px solid var(--border); }
.tab { position: relative; display: inline-flex; align-items: center; gap: 6px; }
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--orange); }

.themes { display: grid; grid-template-columns: repeat(3, minmax(0, 160px)); gap: 16px; }
@media (max-width: 560px) { .themes { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; } }
.theme {
  display: flex; flex-direction: column; gap: 10px; padding: 8px; border-radius: 10px;
  border: 2px solid var(--border); background: transparent; color: var(--ink); font: inherit; cursor: pointer;
}
.theme:hover { border-color: var(--border-strong); }
.theme.on { border-color: var(--primary); }
.theme:focus-visible { outline: 2px solid var(--primary); outline-offset: 2px; }
.theme-label { display: inline-flex; align-items: center; justify-content: center; gap: 6px; font-size: 14px; }
.theme.on .theme-label { color: var(--primary-ink); font-weight: 600; }
/* 主题缩略图：固定展示浅色 / 深色样例，不随当前主题变化 */
.preview { display: flex; height: 64px; border-radius: 6px; overflow: hidden; }
.pv-nav { width: 26%; }
.pv-main { flex: 1; display: flex; flex-direction: column; gap: 6px; padding: 8px; }
.pv-card { height: 18px; border-radius: 4px; }
.pv-card.short { width: 60%; }
.light .preview, .light .pv-nav { background: #f4f5fa; }
.light .pv-nav { box-shadow: inset -1px 0 0 #e7e6eb; }
.light .pv-card { background: #ffffff; box-shadow: 0 1px 4px rgba(58, 53, 65, 0.15); }
.dark .preview, .dark .pv-nav { background: #28243d; }
.dark .pv-nav { box-shadow: inset -1px 0 0 #46425f; }
.dark .pv-card { background: #312d4b; }
.system .preview { background: linear-gradient(135deg, #f4f5fa 50%, #28243d 50%); }
.system .pv-nav { background: transparent; }
.system .pv-card { background: rgba(145, 85, 253, 0.35); }
.light .pv-card:first-child, .dark .pv-card:first-child { box-shadow: inset 3px 0 0 #9155fd; }

.env { align-items: flex-start; }
.env-val { max-width: 60%; text-align: right; font-size: 13px; word-break: break-all; }
@media (max-width: 640px) { .env-val { max-width: 100%; text-align: left; } }
.notes { display: flex; flex-direction: column; gap: 10px; font-size: 14px; line-height: 22px; }
</style>
