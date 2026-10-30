<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NButton, NInput, NTooltip } from 'naive-ui'
import Icon from './Icon.vue'
import { currentPage, navigate, pages, route, type PageKey } from '../router'
import { themePref } from '../prefs'
import { canDelete, cfg, downloaderCount, hasUpdate, job, q, running, totals } from '../store'
import { fmtCount } from '../format'
import type { ThemeMode } from '../types'

// 布局参考 MoviePilot：左侧导航抽屉（桌面可折叠为图标栏，手机为浮层）+ 顶部应用栏
const collapsed = ref(false)
try {
  collapsed.value = localStorage.getItem('ls.nav') === 'rail'
} catch {
  /* 忽略 */
}
watch(collapsed, (v) => {
  try {
    localStorage.setItem('ls.nav', v ? 'rail' : 'full')
  } catch {
    /* 忽略 */
  }
})

const mobile = ref(false)
const drawerOpen = ref(false)
const mq = window.matchMedia('(max-width: 1023px)')
function syncMq() {
  mobile.value = mq.matches
  if (!mq.matches) drawerOpen.value = false
}
onMounted(() => {
  syncMq()
  mq.addEventListener('change', syncMq)
})
onBeforeUnmount(() => mq.removeEventListener('change', syncMq))
watch(() => route.value.page, () => (drawerOpen.value = false))

const rail = computed(() => collapsed.value && !mobile.value)
function toggleNav() {
  if (mobile.value) drawerOpen.value = !drawerOpen.value
  else collapsed.value = !collapsed.value
}

const visiblePages = computed(() => pages.filter((p) => p.key !== 'trash' || canDelete.value))
const groups = computed(() => {
  const out: { name: string; items: typeof pages }[] = []
  for (const p of visiblePages.value) {
    const g = out.find((x) => x.name === p.group)
    if (g) g.items.push(p)
    else out.push({ name: p.group, items: [p] })
  }
  return out
})

/** 导航项右侧的计数 / 状态 */
function navBadge(key: PageKey): { text: string; cls: string } | null {
  if (key === 'scan' && running.value) return { text: '扫描中', cls: 'badge-primary' }
  if (key === 'results' && totals.value?.pending_folders) return { text: fmtCount(totals.value.pending_folders), cls: 'badge-orange' }
  if (key === 'downloaders' && downloaderCount.value) return { text: String(downloaderCount.value), cls: 'badge-gray' }
  if (key === 'settings' && hasUpdate.value) return { text: '新版本', cls: 'badge-primary' }
  return null
}

// 主题：跟随系统 → 浅色 → 深色 循环
const themeNext: Record<ThemeMode, ThemeMode> = { system: 'light', light: 'dark', dark: 'system' }
const themeLabel: Record<ThemeMode, string> = { system: '跟随系统', light: '浅色', dark: '深色' }
function cycleTheme() {
  themePref.value = themeNext[themePref.value]
}

// 应用栏搜索：写入结果页的搜索词并跳转
function onSearch(v: string) {
  q.value = v
  if (route.value.page !== 'results') navigate('results')
}

const scanPct = computed(() => {
  const j = job.value
  return j.phase === 'dups' && j.total ? Math.min(100, Math.round((j.done / j.total) * 100)) : null
})
</script>

<template>
  <div class="layout" :class="{ rail, mobile }">
    <!-- 左侧导航 -->
    <aside class="nav" :class="{ open: drawerOpen }" aria-label="导航">
      <div class="nav-head">
        <a href="#/dashboard" class="brand" aria-label="LinkScope 仪表盘">
          <span class="logo"><Icon name="link" :size="22" /></span>
          <span class="brand-text">
            <span class="brand-name">LinkScope</span>
            <span class="brand-cn">链接透视</span>
          </span>
        </a>
      </div>
      <nav class="nav-body">
        <template v-for="g in groups" :key="g.name">
          <div class="nav-group">
            <span class="nav-group-text">{{ g.name }}</span>
            <span class="nav-group-line" />
          </div>
          <a
            v-for="p in g.items"
            :key="p.key"
            :href="`#/${p.key}`"
            class="nav-item"
            :class="{ active: route.page === p.key }"
            :aria-current="route.page === p.key ? 'page' : undefined"
            :title="rail ? p.title : undefined"
          >
            <Icon :name="p.icon" :size="22" />
            <span class="nav-text">{{ p.title }}</span>
            <span v-if="navBadge(p.key)" class="badge nav-badge" :class="navBadge(p.key)!.cls">{{ navBadge(p.key)!.text }}</span>
            <span v-if="rail && navBadge(p.key)" class="nav-dot" />
          </a>
        </template>
      </nav>
    </aside>
    <div v-if="mobile && drawerOpen" class="scrim" @click="drawerOpen = false" />

    <!-- 内容区 -->
    <div class="content">
      <header class="appbar">
        <div class="appbar-in">
          <NButton quaternary circle :aria-label="mobile ? '打开导航' : rail ? '展开导航' : '收起导航'" @click="toggleNav">
            <template #icon><Icon :name="!mobile && !rail ? 'menuOpen' : 'menu'" :size="22" /></template>
          </NButton>
          <h1 class="page-title">{{ currentPage.title }}</h1>
          <span class="spacer" />
          <NInput
            v-if="!mobile"
            :value="q"
            clearable
            round
            class="appbar-search"
            placeholder="搜索文件夹或文件名"
            :input-props="{ 'aria-label': '搜索结果' }"
            @update:value="onSearch"
          >
            <template #prefix><Icon name="search" /></template>
          </NInput>
          <a v-if="running" href="#/scan" class="scan-chip" title="查看扫描进度">
            <span class="pulse" />
            扫描中<template v-if="scanPct !== null"> {{ scanPct }}%</template>
          </a>
          <NTooltip>
            <template #trigger>
              <NButton quaternary circle :aria-label="`主题：${themeLabel[themePref]}，点击切换`" @click="cycleTheme">
                <template #icon><Icon :name="themePref" :size="22" /></template>
              </NButton>
            </template>
            主题：{{ themeLabel[themePref] }}
          </NTooltip>
          <a
            v-if="cfg"
            href="#/settings/update"
            class="ver-chip"
            :title="`提交 ${cfg.version.commit}`"
            :aria-label="hasUpdate ? `版本 ${cfg.version.version}，有新版本` : `版本 ${cfg.version.version}`"
          >
            v{{ cfg.version.version }}
            <span v-if="hasUpdate" class="dot" />
          </a>
        </div>
      </header>
      <main class="main">
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
.layout { --nav-w: 260px; min-height: 100vh; }
.layout.rail { --nav-w: 76px; }

/* ---- 导航 ---- */
.nav {
  position: fixed; top: 0; bottom: 0; left: 0; z-index: 40; width: var(--nav-w);
  display: flex; flex-direction: column; background: var(--bg);
  transition: width 0.2s ease, transform 0.2s ease;
  overflow: hidden;
}
.mobile .nav { width: 260px; transform: translateX(-100%); background: var(--surface); box-shadow: var(--shadow-lg); }
.mobile .nav.open { transform: none; }
.scrim { position: fixed; inset: 0; z-index: 39; background: rgba(20, 18, 33, 0.45); }

.nav-head { height: 64px; display: flex; align-items: center; padding: 0 22px; flex-shrink: 0; }
.brand { display: flex; align-items: center; gap: 12px; text-decoration: none; color: var(--ink); min-width: 0; }
.logo {
  width: 34px; height: 34px; border-radius: 9px; flex-shrink: 0; display: flex; align-items: center; justify-content: center;
  color: var(--on-primary); background: linear-gradient(135deg, var(--primary-2), var(--primary));
  box-shadow: 0 3px 8px rgba(145, 85, 253, 0.4);
}
.brand-text { display: flex; flex-direction: column; line-height: 1.15; white-space: nowrap; }
.brand-name { font-size: 19px; font-weight: 700; letter-spacing: 0.2px; }
.brand-cn { font-size: 12px; color: var(--ink-2); }

.nav-body { flex: 1; overflow-y: auto; overflow-x: hidden; padding: 4px 0 12px; }
.nav-group { display: flex; align-items: center; gap: 10px; padding: 18px 22px 6px; }
.nav-group-text { font-size: 12px; color: var(--ink-3); letter-spacing: 0.4px; white-space: nowrap; }
.nav-group-line { flex: 1; height: 1px; background: var(--border); }
.rail .nav-group { padding: 18px 0 6px; justify-content: center; }
.rail .nav-group-text { display: none; }
.rail .nav-group-line { flex: 0 0 22px; }

.nav-item {
  position: relative; display: flex; align-items: center; gap: 12px;
  height: 44px; margin: 2px 16px 2px 0; padding: 0 16px 0 22px; border-radius: 0 22px 22px 0;
  color: var(--ink); text-decoration: none; font-size: 15px; white-space: nowrap;
  transition: background-color 0.15s ease;
}
.nav-item .icon { color: var(--ink-2); }
.nav-item:hover { background: var(--chip); }
.nav-item:focus-visible { outline: 2px solid var(--primary); outline-offset: -2px; }
.nav-item.active {
  color: var(--on-primary); background: linear-gradient(-72deg, var(--primary-2) 0%, var(--primary) 78%);
  box-shadow: 0 4px 8px -4px rgba(58, 53, 65, 0.42);
}
.nav-item.active .icon { color: var(--on-primary); }
.nav-text { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.nav-badge { flex: 0 0 auto; }
.nav-item.active .nav-badge { background: rgba(255, 255, 255, 0.24); color: var(--on-primary); }
.rail .nav-item { margin: 2px 12px; padding: 0; justify-content: center; border-radius: 12px; }
.rail .nav-text, .rail .nav-badge, .rail .brand-text { display: none; }
.rail .nav-head { padding: 0; justify-content: center; }
.nav-dot { position: absolute; top: 9px; right: 12px; width: 8px; height: 8px; border-radius: 50%; background: var(--orange); }


/* ---- 内容区 ---- */
.content { margin-left: var(--nav-w); min-width: 0; transition: margin-left 0.2s ease; }
.mobile .content { margin-left: 0; }
.appbar {
  position: sticky; top: 0; z-index: 30; padding: 0 24px;
  background: var(--appbar-bg); backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px);
}
.appbar-in { max-width: 1440px; margin: 0 auto; height: 64px; display: flex; align-items: center; gap: 8px; }
.page-title { margin: 0 0 0 4px; font-size: 20px; font-weight: 600; white-space: nowrap; }
.appbar-search { width: 280px; }
.appbar :deep(.n-button) { color: var(--ink-2); }
.scan-chip {
  display: inline-flex; align-items: center; gap: 6px; height: 30px; padding: 0 12px; border-radius: 15px;
  background: var(--primary-bg); color: var(--primary-ink); font-size: 13px; font-weight: 600; text-decoration: none; white-space: nowrap;
}
.pulse { width: 8px; height: 8px; border-radius: 50%; background: var(--primary); animation: pulse 1.2s ease-in-out infinite; }
@keyframes pulse { 50% { opacity: 0.3; } }
.ver-chip {
  position: relative; display: inline-flex; align-items: center; height: 30px; padding: 0 12px; border-radius: 15px;
  background: var(--surface); box-shadow: var(--shadow); color: var(--ink-2); font-size: 12px; text-decoration: none; white-space: nowrap;
}
.ver-chip:hover { color: var(--primary-ink); }
.ver-chip:focus-visible, .scan-chip:focus-visible, .brand:focus-visible { outline: 2px solid var(--primary); outline-offset: 2px; }
.dot { position: absolute; top: -2px; right: -2px; width: 10px; height: 10px; border-radius: 50%; background: var(--orange); border: 2px solid var(--surface); }
.main { max-width: 1440px; margin: 0 auto; padding: 8px 24px 40px; }

@media (max-width: 640px) {
  .appbar { padding: 0 10px; }
  .main { padding: 4px 12px 32px; }
  .page-title { font-size: 18px; }
}
@media (prefers-reduced-motion: reduce) {
  .nav, .content { transition: none; }
  .pulse { animation: none; }
}
</style>
