<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NEmpty, NInput, NRadioButton, NRadioGroup, NSelect, NTab, NTabs } from 'naive-ui'
import Icon from '../components/Icon.vue'
import FolderTable from '../components/FolderTable.vue'
import ResultTable from '../components/ResultTable.vue'
import BulkBar from '../components/BulkBar.vue'
import DetailDrawer from '../components/DetailDrawer.vue'
import { api } from '../api'
import { fmtCount, fmtSize } from '../format'
import { navigate } from '../router'
import {
  canDelete,
  cfg,
  checked,
  checkedTotal,
  clearChecked,
  clearFilters,
  closeDrawer,
  count,
  counts,
  currentFilter,
  currentTabLabel,
  deleteAllFiltered,
  deleteBlocked,
  deleteChecked,
  deletePaths,
  dirFilter,
  fCounts,
  fFilter,
  fileTabs,
  filter,
  folderTabs,
  folders,
  fSort,
  hasFilters,
  hasTorrents,
  iSort,
  items,
  loadError,
  loadResult,
  loading,
  onChecked,
  onFileSort,
  onFolderSort,
  onlyFolder,
  onPageSize,
  openFolder,
  openItem,
  page,
  pageSize,
  q,
  ready,
  running,
  showFolder,
  showItem,
  site,
  summary,
  view,
} from '../store'

const siteOptions = computed(() => [
  { label: '全部站点', value: '' },
  ...(summary.value?.torrents?.sites ?? []).map((s) => ({ label: `${s.site}（${s.files}）`, value: s.site })),
])
const dirLabel = computed(() => {
  const src = summary.value?.source ?? ''
  return dirFilter.value.startsWith(src + '/') ? dirFilter.value.slice(src.length + 1) : dirFilter.value
})
const exportUrl = computed(() =>
  view.value === 'folders'
    ? api.foldersExportUrl(fFilter.value, site.value, q.value)
    : api.exportUrl(filter.value, site.value, q.value, dirFilter.value),
)
const emptyText = computed(() => {
  if (q.value.trim()) return `没有名称包含「${q.value.trim()}」的结果`
  if (view.value === 'folders' && fFilter.value === 'unlinked') return '没有待清理的文件夹，下载目录里的内容都已被媒体库用上'
  if (view.value === 'files' && filter.value === 'unlinked') return '没有待清理的文件'
  return `「${currentTabLabel.value}」中没有内容`
})
</script>

<template>
  <div class="page">
    <section v-if="ready && !summary" class="card empty-card">
      <NEmpty :description="running ? '正在扫描，完成后这里会列出结果' : '还没有扫描结果'">
        <template #extra>
          <NButton type="primary" @click="navigate('scan')">{{ running ? '查看扫描进度' : '去扫描' }}</NButton>
        </template>
      </NEmpty>
    </section>

    <section v-if="summary" class="card results" aria-label="清理结果">
      <div class="r-head">
        <NRadioGroup v-model:value="view" aria-label="显示方式">
          <NRadioButton value="folders">
            <span class="rb"><Icon name="folders" :size="16" />按文件夹</span>
          </NRadioButton>
          <NRadioButton value="files">
            <span class="rb"><Icon name="file" :size="16" />按文件</span>
          </NRadioButton>
        </NRadioGroup>
        <NInput
          v-model:value="q"
          clearable
          class="search"
          :placeholder="view === 'folders' ? '搜索文件夹名' : '搜索文件名或路径'"
          :input-props="{ 'aria-label': '搜索' }"
        >
          <template #prefix><Icon name="search" /></template>
        </NInput>
        <NSelect
          v-if="hasTorrents"
          v-model:value="site"
          :options="siteOptions"
          class="site-select"
          :consistent-menu-width="false"
          aria-label="按站点筛选"
        />
        <NButton v-if="hasFilters" quaternary @click="clearFilters">清除筛选</NButton>
        <span class="spacer" />
        <NButton tag="a" :href="exportUrl" secondary>
          <template #icon><Icon name="download" /></template>
          导出 CSV
        </NButton>
      </div>

      <NTabs v-if="view === 'folders'" v-model:value="fFilter" type="line" class="tabs">
        <NTab v-for="t in folderTabs" :key="t.key" :name="t.key">
          {{ t.label }}<span class="tab-count">{{ fmtCount(fCounts[t.key] ?? 0) }}</span>
        </NTab>
      </NTabs>
      <NTabs v-else v-model:value="filter" type="line" class="tabs">
        <NTab v-for="t in fileTabs" :key="t.key" :name="t.key">
          {{ t.label }}<span class="tab-count">{{ fmtCount(counts[t.key] ?? 0) }}</span>
        </NTab>
      </NTabs>

      <BulkBar
        v-if="canDelete"
        :count="checked.length"
        :size="checkedTotal"
        :unit="view === 'folders' ? '个文件夹' : '项'"
        @clear="clearChecked"
        @delete="deleteChecked"
      />
      <div v-if="view === 'files' && dirFilter" class="strip">
        <Icon name="folder" />
        <span>当前仅显示文件夹 <b>{{ dirLabel }}</b> 中的文件</span>
        <NButton size="tiny" quaternary @click="dirFilter = ''">查看全部文件</NButton>
      </div>
      <div v-if="canDelete && currentFilter !== 'all' && count && !checked.length" class="strip">
        <span class="muted">「{{ currentTabLabel }}」共 {{ fmtCount(count) }} {{ view === 'folders' ? '个文件夹' : '项' }}</span>
        <NButton size="tiny" type="error" secondary :disabled="!!deleteBlocked" @click="deleteAllFiltered">
          <template #icon><Icon name="broom" :size="14" /></template>
          清理全部筛选结果
        </NButton>
      </div>
      <div v-if="cfg && !cfg.can_delete" class="strip muted">
        <Icon name="alert" />
        <span>清理功能不可用：{{ cfg.delete_reason }}</span>
      </div>
      <div v-if="loadError" class="strip c-red">
        <Icon name="alert" />
        <span>加载失败：{{ loadError }}</span>
        <NButton size="tiny" @click="loadResult">重试</NButton>
      </div>

      <FolderTable
        v-if="view === 'folders' && (loading || count)"
        :checked="checked"
        :selectable="canDelete"
        :show-torrents="hasTorrents"
        :items="folders"
        :count="count"
        :page="page"
        :page-size="pageSize"
        :loading="loading"
        :selected="openFolder?.path ?? null"
        :sort="fSort.by"
        :order="fSort.order"
        @open="showFolder"
        @update:checked="onChecked"
        @update:page="(p: number) => (page = p)"
        @update:page-size="onPageSize"
        @sort="onFolderSort"
      />
      <ResultTable
        v-else-if="view === 'files' && (loading || count)"
        :checked="checked"
        :selectable="canDelete"
        :show-torrents="hasTorrents"
        :items="items"
        :count="count"
        :page="page"
        :page-size="pageSize"
        :loading="loading"
        :selected="openItem?.path ?? null"
        :sort="iSort.by"
        :order="iSort.order"
        :base-dir="summary.source"
        @open="showItem"
        @update:checked="onChecked"
        @update:page="(p: number) => (page = p)"
        @update:page-size="onPageSize"
        @sort="onFileSort"
      />

      <div v-if="!loading && !count && !loadError" class="empty">
        <NEmpty :description="emptyText">
          <template #extra>
            <div class="empty-actions">
              <NButton v-if="view === 'folders' && fFilter === 'unlinked'" size="small" @click="fFilter = 'partial'">
                查看部分游离
              </NButton>
              <NButton v-if="hasFilters" size="small" @click="clearFilters">清除筛选</NButton>
            </div>
          </template>
        </NEmpty>
      </div>
    </section>

    <DetailDrawer
      :folder="openFolder"
      :item="openItem"
      :can-delete="canDelete"
      :delete-blocked="deleteBlocked"
      @close="closeDrawer"
      @delete="deletePaths"
      @open-files="onlyFolder"
    />

    <span class="sr-only" aria-live="polite">{{ checked.length ? `已选 ${checked.length} 项，共 ${fmtSize(checkedTotal)}` : '' }}</span>
  </div>
</template>

<style scoped>
.c-red { color: var(--red-ink); }
.empty-card { padding: 56px 16px; }
.results { overflow: hidden; display: flex; flex-direction: column; }
/* 桌面端：卡片占满一屏，只有表格内部滚动，页面本身不出现滚动条 */
@media (min-width: 641px) {
  .page { height: calc(100dvh - 112px); min-height: 520px; }
  .results { flex: 1; min-height: 0; }
}
.results :deep(.ls-table) { flex: 1; min-height: 0; }
.results :deep(.ls-table .n-data-table-th:first-child),
.results :deep(.ls-table .n-data-table-td:first-child) { padding-left: 20px; }
.results :deep(.ls-table .n-data-table-th:last-child),
.results :deep(.ls-table .n-data-table-td:last-child) { padding-right: 20px; }
.results :deep(.ls-table .n-data-table__pagination) { margin: 0; padding: 12px 20px; border-top: 1px solid var(--border); }
.r-head { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 12px; padding: 18px 20px 8px; }
.rb { display: inline-flex; align-items: center; gap: 6px; }
.search { width: 260px; max-width: 100%; }
.site-select { width: 190px; }
.tabs { padding: 0 20px; }
.tabs :deep(.n-tabs-nav) { border-bottom: 1px solid var(--border); }
.tab-count { margin-left: 6px; padding: 0 6px; border-radius: 9px; background: var(--chip); color: var(--ink-2); font-size: 12px; line-height: 18px; }
.tabs :deep(.n-tabs-tab--active) .tab-count { background: var(--primary-bg); color: var(--primary-ink); }
.strip { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 10px 20px; border-bottom: 1px solid var(--border); background: var(--soft); font-size: 13px; }
.empty { flex: 1; display: flex; align-items: center; justify-content: center; padding: 40px 16px 48px; }
.empty-actions { display: flex; gap: 8px; justify-content: center; }
@media (max-width: 640px) {
  .r-head { padding: 14px 14px 6px; }
  .tabs { padding: 0 8px; }
  .strip { padding: 8px 14px; }
  .results :deep(.ls-table) { height: 70vh; flex: none; }
  .results :deep(.ls-table .n-data-table-th:first-child),
  .results :deep(.ls-table .n-data-table-td:first-child) { padding-left: 14px; }
  .results :deep(.ls-table .n-data-table__pagination) { padding: 10px 14px; }
  .search, .site-select { width: 100%; }
}
</style>
