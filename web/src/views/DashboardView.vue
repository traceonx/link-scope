<script setup lang="ts">
import { computed } from 'vue'
import { NButton } from 'naive-ui'
import Icon from '../components/Icon.vue'
import { fmtCount, fmtSize, fmtTime } from '../format'
import { navigate } from '../router'
import { cfg, downloaderCount, go, hasTorrents, loadError, ready, running, site, summary, totals } from '../store'
import type { Filter, FolderFilter, ViewMode } from '../types'

const s = computed(() => summary.value)
const pendingPct = computed(() => {
  const x = s.value
  return x && x.total_size ? Math.round((x.unlinked_size / x.total_size) * 100) : 0
})

interface Stat {
  key: string
  label: string
  value: string
  sub: string
  tone: string
  icon: string
  go: [ViewMode, FolderFilter | Filter]
}

// 清理概览：四项核心统计
const stats = computed<Stat[]>(() => {
  const x = s.value
  const t = totals.value
  if (!x) return []
  return [
    { key: 'pending', label: '待清理文件', value: fmtCount(x.unlinked), sub: fmtSize(x.unlinked_size), tone: 'orange', icon: 'broom', go: ['files', 'unlinked'] },
    { key: 'done', label: '已整理文件', value: fmtCount(x.linked), sub: fmtSize(x.linked_size), tone: 'blue', icon: 'link', go: ['folders', 'full'] },
    {
      key: 'partial',
      label: '部分游离文件夹',
      value: fmtCount(t ? t.pending_folders - t.unlinked_folders : 0),
      sub: '有文件没被整理',
      tone: 'violet',
      icon: 'folder',
      go: ['folders', 'partial'],
    },
    {
      key: 'dup',
      label: '重复副本',
      value: fmtCount(x.dup_files),
      sub: x.dup_wasted ? `多占 ${fmtSize(x.dup_wasted)}` : '未发现',
      tone: 'violet',
      icon: 'copies',
      go: ['folders', 'dup'],
    },
  ]
})

// 种子相关统计（连接下载器后才有）
const torrentStats = computed<Stat[]>(() => {
  const t = s.value?.torrents
  if (!t) return []
  return [
    { key: 'seeding', label: '做种中', value: fmtCount(t.seeding), sub: '删除前请留意', tone: 'green', icon: 'seed', go: ['folders', 'seeding'] },
    { key: 'orphan', label: '孤儿文件', value: fmtCount(t.orphans), sub: fmtSize(t.orphan_size), tone: 'orange', icon: 'file', go: ['folders', 'orphan'] },
    { key: 'unreg', label: '站点已删种', value: fmtCount(t.unregistered), sub: '可放心清理', tone: 'red', icon: 'alert', go: ['folders', 'unregistered'] },
  ]
})

// 空间构成环形图：已整理 / 待清理
const R = 54
const C = 2 * Math.PI * R
const linkedArc = computed(() => {
  const x = s.value
  if (!x || !x.total_size) return 0
  return (x.linked_size / x.total_size) * C
})

const sites = computed(() => s.value?.torrents?.sites ?? [])
const siteMax = computed(() => sites.value[0]?.size || 1)
const failedClients = computed(() => s.value?.torrents?.clients.filter((c) => !c.ok) ?? [])
const siteWidth = (size: number) => `${Math.max(3, Math.round((size / siteMax.value) * 100))}%`
function openSite(name: string) {
  go('folders', 'all', site.value === name ? '' : name)
}
</script>

<template>
  <div class="page">
    <div v-if="loadError && !summary" class="notice notice-err">
      <Icon name="alert" />
      <span>无法读取数据：{{ loadError }}。请确认容器正在运行，然后刷新页面。</span>
    </div>

    <!-- 首次使用 -->
    <section v-if="ready && !summary && !running && cfg" class="card onboarding">
      <span class="avatar primary big"><Icon name="broom" :size="34" /></span>
      <h2>找出下载目录里没被媒体库用上的文件</h2>
      <p class="muted">
        填好下载目录（qBittorrent 的保存位置）和媒体库目录（MoviePilot 整理的目标，如电视剧、电影、动漫），开始扫描。
        没有被硬链接到媒体库的文件会列为「待清理」。
      </p>
      <div class="ob-actions">
        <NButton type="primary" size="large" @click="navigate('scan')">
          <template #icon><Icon name="scan" /></template>
          去扫描
        </NButton>
        <NButton v-if="!downloaderCount" size="large" secondary @click="navigate('downloaders')">
          <template #icon><Icon name="downloader" /></template>
          添加下载器（可选）
        </NButton>
      </div>
    </section>
    <section v-else-if="!summary && running" class="card onboarding">
      <span class="avatar primary big"><Icon name="scan" :size="34" /></span>
      <h2>正在进行第一次扫描…</h2>
      <NButton secondary @click="navigate('scan')">查看进度</NButton>
    </section>

    <template v-if="s">
      <div class="row-a">
        <!-- 待清理主卡片 -->
        <section class="card hero" aria-label="待清理">
          <div class="hero-text">
            <span class="hero-label">待清理 · 没被媒体库用上的文件</span>
            <b class="hero-num">{{ fmtSize(s.unlinked_size) }}</b>
            <span class="muted">
              占下载目录的 <b class="c-orange">{{ pendingPct }}%</b> · {{ fmtCount(s.unlinked) }} 个文件
              <template v-if="totals"> · {{ fmtCount(totals.pending_folders) }} 个文件夹</template>
            </span>
            <div class="hero-actions">
              <NButton type="primary" :disabled="!s.unlinked" @click="go('folders', 'unlinked', '')">
                <template #icon><Icon name="broom" /></template>
                查看待清理
              </NButton>
              <NButton quaternary :disabled="!s.unlinked" @click="go('files', 'unlinked', '')">按文件查看</NButton>
            </div>
          </div>
          <Icon name="broom" :size="120" class="hero-art" />
        </section>

        <!-- 清理概览 -->
        <section class="card overview" aria-label="清理概览">
          <div class="card-head">
            <h2 class="card-title">清理概览</h2>
            <span class="card-sub">共扫描 {{ fmtCount(s.total) }} 个文件 · {{ fmtSize(s.total_size) }}</span>
          </div>
          <div class="card-body stat-row">
            <button v-for="c in stats" :key="c.key" type="button" class="stat" @click="go(c.go[0], c.go[1], '')">
              <span class="avatar" :class="c.tone"><Icon :name="c.icon" :size="24" /></span>
              <span class="stat-body">
                <span class="stat-label">{{ c.label }}</span>
                <b class="stat-val">{{ c.value }}</b>
                <span class="stat-sub">{{ c.sub }}</span>
              </span>
            </button>
          </div>
        </section>
      </div>

      <div class="grid-2">
        <!-- 空间构成 -->
        <section class="card" aria-label="空间构成">
          <div class="card-head">
            <h2 class="card-title">空间构成</h2>
            <span class="card-sub">下载目录 {{ fmtSize(s.total_size) }}</span>
          </div>
          <div class="card-body donut-wrap">
            <svg viewBox="0 0 140 140" class="donut" role="img" :aria-label="`已整理 ${100 - pendingPct}%，待清理 ${pendingPct}%`">
              <circle cx="70" cy="70" :r="R" class="ring-bg" />
              <circle
                v-if="s.total_size"
                cx="70"
                cy="70"
                :r="R"
                class="ring-linked"
                :stroke-dasharray="`${linkedArc} ${C}`"
                transform="rotate(-90 70 70)"
              />
              <text x="70" y="66" text-anchor="middle" class="donut-num">{{ pendingPct }}%</text>
              <text x="70" y="86" text-anchor="middle" class="donut-sub">待清理</text>
            </svg>
            <ul class="legend">
              <li>
                <i class="lg blue" />
                <span class="lg-label">已整理</span>
                <b>{{ fmtSize(s.linked_size) }}</b>
              </li>
              <li>
                <i class="lg orange" />
                <span class="lg-label">待清理</span>
                <b>{{ fmtSize(s.unlinked_size) }}</b>
              </li>
              <li v-if="s.saved">
                <i class="lg primary" />
                <span class="lg-label">硬链接节省</span>
                <b>{{ fmtSize(s.saved) }}</b>
              </li>
              <li class="others">
                <button type="button" class="link-btn" @click="go('files', 'symlink', '')">
                  软链接 {{ fmtCount(s.symlinks) }}<template v-if="s.broken_links">（失效 {{ s.broken_links }}）</template>
                </button>
                <button type="button" class="link-btn" @click="go('files', 'strm', '')">
                  STRM {{ fmtCount(s.strm) }}<template v-if="s.broken_strm">（失效 {{ s.broken_strm }}）</template>
                </button>
              </li>
            </ul>
          </div>
        </section>

        <!-- 站点占用 -->
        <section class="card" aria-label="站点占用">
          <div class="card-head">
            <h2 class="card-title">站点占用</h2>
            <span v-if="s.torrents" class="card-sub">
              {{ s.torrents.clients.length }} 个下载器 · {{ sites.length }} 个站点 · 读取于 {{ fmtTime(s.torrents.fetched_at) }}
            </span>
          </div>
          <div class="card-body">
            <template v-if="hasTorrents && s.torrents">
              <div class="tstats">
                <button v-for="c in torrentStats" :key="c.key" type="button" class="tstat" @click="go(c.go[0], c.go[1], '')">
                  <span class="avatar" :class="c.tone"><Icon :name="c.icon" :size="20" /></span>
                  <span class="stat-body">
                    <b class="tstat-val">{{ c.value }}</b>
                    <span class="stat-label">{{ c.label }}</span>
                  </span>
                </button>
              </div>
              <div v-for="c in failedClients" :key="c.name" class="notice notice-err">下载器「{{ c.name }}」读取失败：{{ c.error }}</div>
              <ul v-if="sites.length" class="sites">
                <li v-for="x in sites.slice(0, 8)" :key="x.site">
                  <button type="button" class="site" :class="{ on: site === x.site }" :aria-pressed="site === x.site" @click="openSite(x.site)">
                    <span class="site-top">
                      <b>{{ x.site }}</b>
                      <span class="muted small">
                        {{ x.files }} 个文件 · {{ fmtSize(x.size) }}
                        <template v-if="x.unregistered"> · <span class="c-red">{{ x.unregistered }} 已删种</span></template>
                      </span>
                    </span>
                    <span class="bar"><span :style="{ width: siteWidth(x.size) }" /></span>
                  </button>
                </li>
              </ul>
              <p v-else class="muted small">下载目录中的文件没有对应到任何种子，请检查下载器的路径映射。</p>
              <p v-if="sites.length > 8" class="muted small">另有 {{ sites.length - 8 }} 个站点，可在清理结果中按站点筛选。</p>
            </template>
            <div v-else class="empty-mini">
              <span class="avatar green"><Icon name="seed" :size="24" /></span>
              <div>
                <b>还没有种子信息</b>
                <p class="muted small">
                  添加 qBittorrent / Transmission 后，扫描结束会显示每个文件来自哪个站点、是否在做种、是否已被站点删种。
                </p>
                <NButton size="small" secondary type="primary" @click="navigate('downloaders')">前往下载器</NButton>
              </div>
            </div>
          </div>
        </section>
      </div>

      <!-- 最近扫描 -->
      <section class="card" aria-label="最近扫描">
        <div class="card-head">
          <h2 class="card-title">最近扫描</h2>
          <span class="card-sub">{{ fmtTime(s.scanned_at) }} · 用时 {{ s.duration }} 秒</span>
          <span class="spacer" />
          <NButton size="small" secondary type="primary" @click="navigate('scan')">
            <template #icon><Icon name="refresh" /></template>
            {{ running ? '查看进度' : '重新扫描' }}
          </NButton>
        </div>
        <div class="card-body last">
          <dl>
            <dt>下载目录</dt>
            <dd class="mono">{{ s.source }}</dd>
            <dt>媒体库目录</dt>
            <dd class="mono">{{ s.targets.length ? s.targets.join('、') : '未设置' }}</dd>
          </dl>
          <div v-if="s.stale" class="notice notice-info">
            <Icon name="refresh" />
            <span>上次扫描后有文件被清理或恢复，统计为估算值，建议重新扫描。</span>
          </div>
          <div v-if="s.warnings?.length" class="notice notice-warn">
            <Icon name="alert" />
            <div><div v-for="w in s.warnings" :key="w">{{ w }}</div></div>
          </div>
          <div v-if="s.error_count" class="notice notice-err">
            <Icon name="alert" />
            <div>
              有 {{ s.error_count }} 个路径无法读取，通常是权限不足（compose 中不设置 PUID 即以 root 运行）。
              <details>
                <summary>查看路径</summary>
                <div v-for="e in s.errors" :key="e" class="mono small">{{ e }}</div>
              </details>
            </div>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.c-orange { color: var(--orange-ink); }
.c-red { color: var(--red-ink); }
p { margin: 0; }
details summary { cursor: pointer; text-decoration: underline; }

.onboarding { padding: 40px 24px; display: flex; flex-direction: column; align-items: center; text-align: center; gap: 12px; }
.onboarding h2 { margin: 6px 0 0; font-size: 20px; }
.onboarding p { max-width: 640px; font-size: 14px; line-height: 22px; }
.avatar.big { width: 64px; height: 64px; border-radius: 16px; }
.ob-actions { display: flex; flex-wrap: wrap; justify-content: center; gap: 10px; margin-top: 6px; }

.row-a { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); gap: 22px; }
@media (max-width: 1100px) { .row-a { grid-template-columns: minmax(0, 1fr); } }

.hero { position: relative; overflow: hidden; padding: 22px 24px; display: flex; }
.hero-text { position: relative; z-index: 1; display: flex; flex-direction: column; gap: 6px; font-size: 14px; }
.hero-label { font-size: 15px; font-weight: 600; }
.hero-num { font-size: 36px; line-height: 46px; font-weight: 700; color: var(--orange-ink); }
.hero-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
.hero-art { position: absolute; right: -8px; bottom: -14px; color: var(--orange); opacity: 0.16; transform: rotate(-12deg); }

.stat-row { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
@media (max-width: 720px) { .stat-row { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
.stat, .tstat {
  display: flex; align-items: center; gap: 12px; padding: 8px; margin: -8px; border: 0; border-radius: 8px;
  background: transparent; color: inherit; font: inherit; text-align: left; cursor: pointer; min-width: 0;
}
.stat:hover, .tstat:hover { background: var(--soft); }
.stat:focus-visible, .tstat:focus-visible, .site:focus-visible, .link-btn:focus-visible { outline: 2px solid var(--primary); outline-offset: 2px; }
.stat-body { display: flex; flex-direction: column; min-width: 0; }
.stat-label { font-size: 13px; color: var(--ink-2); line-height: 18px; }
.stat-val { font-size: 20px; line-height: 28px; font-weight: 600; }
.stat-sub { font-size: 12px; color: var(--ink-3); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.donut-wrap { display: flex; flex-wrap: wrap; align-items: center; gap: 20px 32px; }
.donut { width: 160px; height: 160px; flex-shrink: 0; }
.ring-bg { fill: none; stroke: var(--orange); stroke-width: 16; }
.ring-linked { fill: none; stroke: var(--blue); stroke-width: 16; stroke-linecap: butt; }
.donut-num { font-size: 24px; font-weight: 700; fill: var(--ink); }
.donut-sub { font-size: 12px; fill: var(--ink-2); }
.legend { flex: 1; min-width: 200px; margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 12px; font-size: 14px; }
.legend li { display: flex; align-items: center; gap: 10px; }
.lg { width: 10px; height: 10px; border-radius: 3px; flex-shrink: 0; }
.lg.blue { background: var(--blue); }
.lg.orange { background: var(--orange); }
.lg.primary { background: var(--primary); }
.lg-label { flex: 1; color: var(--ink-2); }
.legend .others { flex-wrap: wrap; gap: 4px 14px; padding-top: 10px; border-top: 1px solid var(--border); }
.link-btn { border: 0; padding: 0; background: transparent; color: var(--primary-ink); font: inherit; font-size: 13px; cursor: pointer; }
.link-btn:hover { text-decoration: underline; }

.tstats { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-bottom: 16px; }
.tstat .avatar { width: 38px; height: 38px; }
.tstat-val { font-size: 18px; line-height: 24px; }
.sites { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 4px; }
.site {
  width: 100%; display: flex; flex-direction: column; gap: 6px; padding: 8px 10px; border: 0; border-radius: 8px;
  background: transparent; color: inherit; font: inherit; text-align: left; cursor: pointer;
}
.site:hover { background: var(--soft); }
.site.on { background: var(--primary-row); box-shadow: inset 3px 0 0 var(--primary); }
.site-top { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 2px 10px; font-size: 14px; }
.bar { height: 6px; border-radius: 3px; background: var(--chip); overflow: hidden; }
.bar span { display: block; height: 6px; border-radius: 3px; background: var(--green); }
.empty-mini { display: flex; gap: 14px; align-items: flex-start; }
.empty-mini > div { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; }
.empty-mini p { line-height: 18px; }

.last { display: flex; flex-direction: column; gap: 10px; }
dl { display: grid; grid-template-columns: auto 1fr; gap: 6px 16px; margin: 0; font-size: 13px; }
dt { color: var(--ink-2); }
dd { margin: 0; word-break: break-all; }

@media (max-width: 640px) {
  .hero-num { font-size: 30px; line-height: 38px; }
  .tstats { grid-template-columns: 1fr; gap: 14px; }
  .donut-wrap { justify-content: center; }
}
</style>
