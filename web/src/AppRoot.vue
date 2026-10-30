<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import AppLayout from './components/AppLayout.vue'
import DeleteDialog from './components/DeleteDialog.vue'
import DashboardView from './views/DashboardView.vue'
import ScanView from './views/ScanView.vue'
import ResultsView from './views/ResultsView.vue'
import DownloadersView from './views/DownloadersView.vue'
import TrashView from './views/TrashView.vue'
import SettingsView from './views/SettingsView.vue'
import { route } from './router'
import { bindMessage, del, init, onDeleted } from './store'

bindMessage(useMessage())
onMounted(init)

const views = {
  dashboard: DashboardView,
  scan: ScanView,
  results: ResultsView,
  downloaders: DownloadersView,
  trash: TrashView,
  settings: SettingsView,
}
const current = computed(() => views[route.value.page])
</script>

<template>
  <AppLayout>
    <!-- 保留各页面状态（筛选、表单、更新进度轮询），切换页面不重置 -->
    <KeepAlive>
      <component :is="current" />
    </KeepAlive>
  </AppLayout>
  <DeleteDialog v-model:show="del.show" :target="del.target" :title="del.title" @done="onDeleted" />
</template>
