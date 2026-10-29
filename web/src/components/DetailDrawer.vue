<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NDrawer, NDrawerContent } from 'naive-ui'
import Icon from './Icon.vue'
import FolderDetail from './FolderDetail.vue'
import FileDetail from './FileDetail.vue'
import type { Folder, Item } from '../types'

const props = defineProps<{
  folder: Folder | null
  item: Item | null
  canDelete: boolean
  deleteBlocked: string
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'delete', paths: string[], title: string): void
  (e: 'open-files', f: Folder): void
}>()

// 从文件夹点进某个文件时，在抽屉内显示文件详情，可返回文件夹
const drill = ref<Item | null>(null)
watch(
  () => props.folder?.path,
  () => (drill.value = null),
)

const show = computed(() => !!props.folder || !!props.item)
const width = ref(Math.min(560, window.innerWidth))
window.addEventListener('resize', () => (width.value = Math.min(560, window.innerWidth)))

const current = computed(() => drill.value ?? props.item)
</script>

<template>
  <NDrawer
    :show="show"
    :width="width"
    placement="right"
    :auto-focus="false"
    @update:show="(v: boolean) => !v && emit('close')"
  >
    <NDrawerContent :native-scrollbar="false" body-content-style="padding: 16px 20px 0">
      <template #header>
        <div class="dh">
          <NButton v-if="folder && drill" text size="small" @click="drill = null">
            <template #icon><Icon name="back" /></template>
            返回 {{ folder.name }}
          </NButton>
          <span v-else class="dh-title">{{ folder ? '文件夹详情' : '文件详情' }}</span>
          <NButton quaternary circle size="small" aria-label="关闭详情" @click="emit('close')">
            <template #icon><Icon name="close" /></template>
          </NButton>
        </div>
      </template>
      <FileDetail
        v-if="current"
        :item="current"
        :can-delete="canDelete"
        :delete-blocked="deleteBlocked"
        @delete="(p: string[], t: string) => emit('delete', p, t)"
      />
      <FolderDetail
        v-else-if="folder"
        :folder="folder"
        :can-delete="canDelete"
        :delete-blocked="deleteBlocked"
        @delete="(p: string[], t: string) => emit('delete', p, t)"
        @open-files="(f: Folder) => emit('open-files', f)"
        @open-file="(it: Item) => (drill = it)"
      />
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.dh { display: flex; align-items: center; justify-content: space-between; gap: 8px; width: 100%; }
.dh-title { font-size: 15px; font-weight: 600; }
</style>
