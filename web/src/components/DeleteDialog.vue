<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NCheckbox, NInput, NModal, NSpin, useMessage } from 'naive-ui'
import Icon from './Icon.vue'
import { api } from '../api'
import { fmtCount, fmtSize } from '../format'
import type { DeletePreview, DeleteResult, DeleteTarget } from '../types'

const props = defineProps<{ show: boolean; target: DeleteTarget | null; title?: string }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'done', r: DeleteResult): void }>()
const message = useMessage()

const preview = ref<DeletePreview | null>(null)
const loading = ref(false)
const deleting = ref(false)
const error = ref('')
const permanent = ref(false)
const confirmText = ref('')

watch(
  () => props.show,
  async (v) => {
    if (!v || !props.target) return
    preview.value = null
    error.value = ''
    permanent.value = false
    confirmText.value = ''
    loading.value = true
    try {
      preview.value = await api.deletePreview(props.target)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  },
)

const pv = computed(() => preview.value?.preview)
const canConfirm = computed(
  () => !!pv.value && pv.value.count > pv.value.errors && (!permanent.value || confirmText.value.trim() === '删除'),
)
const kindLabel = { file: '文件', dir: '文件夹', symlink: '软链接', '': '' } as const

async function run() {
  if (!props.target) return
  deleting.value = true
  try {
    const r = await api.deletePaths(props.target, permanent.value)
    if (r.failed) message.warning(`已删除 ${r.deleted} 项，${r.failed} 项失败：${r.errors[0]?.error ?? ''}`)
    else message.success(permanent.value ? `已永久删除 ${r.deleted} 项` : `已移到回收站 ${r.deleted} 项`)
    emit('done', r)
    emit('update:show', false)
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="title ?? '删除'"
    style="width: min(600px, calc(100vw - 24px))"
    :bordered="false"
    :mask-closable="!deleting"
    :closable="!deleting"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <NSpin :show="loading">
      <div class="del">
        <div v-if="error" class="notice notice-err">{{ error }}</div>
        <template v-if="pv">
          <div class="sum">
            <div><span>数量</span><b>{{ fmtCount(pv.count - pv.errors) }} 项</b></div>
            <div><span>包含文件</span><b>{{ fmtCount(pv.files) }} 个</b></div>
            <div><span>总大小</span><b>{{ fmtSize(pv.total_size) }}</b></div>
            <div><span>实际释放</span><b :class="pv.freed ? 'c-green' : 'muted'">{{ fmtSize(pv.freed) }}</b></div>
          </div>

          <div v-if="preview!.seeding.files" class="notice notice-err">
            <Icon name="seed" :size="18" />
            <span>
              其中 <b>{{ preview!.seeding.files }}</b> 个文件仍在做种（{{ preview!.seeding.sites.join('、') }}）。
              删除后种子会出错，可能被站点判为 H&amp;R；建议先在下载器里停止或删除种子。
            </span>
          </div>
          <div v-if="pv.kept" class="notice notice-warn">
            <Icon name="link" :size="18" />
            <span>
              {{ pv.kept }} 个文件在其他位置还有硬链接（共 {{ fmtSize(pv.kept_size) }}），删除这里的路径不会释放空间，
              其他位置的文件不受影响。
            </span>
          </div>
          <div v-if="!permanent" class="muted small">
            移到回收站后可以恢复；空间要在清空回收站后才释放，{{ preview!.trash_days }} 天后自动清除。
          </div>

          <div class="list">
            <div v-for="it in pv.items" :key="it.path" class="row" :class="{ bad: it.error }">
              <span class="badge" :class="it.error ? 'badge-red' : 'badge-gray'">{{ it.error ? '跳过' : kindLabel[it.kind] }}</span>
              <span class="mono path">{{ it.path }}</span>
              <span class="meta">
                <template v-if="it.error">{{ it.error }}</template>
                <template v-else-if="it.kind === 'dir'">{{ it.files }} 个文件 · {{ fmtSize(it.size) }}</template>
                <template v-else-if="it.kind === 'file'">
                  {{ fmtSize(it.size) }}<template v-if="!it.freed"> · 其他位置还有链接</template>
                </template>
              </span>
            </div>
            <div v-if="pv.count > pv.items.length" class="muted small more">
              还有 {{ fmtCount(pv.count - pv.items.length) }} 项未列出
            </div>
          </div>

          <div class="perm">
            <NCheckbox v-model:checked="permanent">永久删除（不进回收站，无法恢复）</NCheckbox>
            <NInput
              v-if="permanent"
              v-model:value="confirmText"
              placeholder="输入「删除」确认"
              :input-props="{ 'aria-label': '输入删除确认' }"
              style="max-width: 220px"
            />
          </div>
        </template>
      </div>
    </NSpin>
    <template #footer>
      <div class="foot">
        <NButton :disabled="deleting" @click="emit('update:show', false)">取消</NButton>
        <NButton type="error" :disabled="!canConfirm" :loading="deleting" @click="run">
          <template #icon><Icon name="trash" /></template>
          {{ permanent ? '永久删除' : '移到回收站' }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.del { display: flex; flex-direction: column; gap: 12px; min-height: 80px; }
.sum { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; padding: 14px; border-radius: 8px; background: var(--soft); }
.sum div { display: flex; flex-direction: column; gap: 2px; }
.sum span { font-size: 12px; color: var(--ink-2); }
.sum b { font-size: 15px; }
.c-green { color: var(--green-ink); }
.small { font-size: 13px; }
.list { max-height: 260px; overflow-y: auto; border: 1px solid var(--border); border-radius: 8px; }
.row { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; padding: 8px 12px; border-bottom: 1px solid var(--border); }
.row:last-child { border-bottom: 0; }
.path { flex: 1 1 260px; min-width: 0; font-size: 12px; word-break: break-all; }
.meta { font-size: 12px; color: var(--ink-2); }
.row.bad .path { color: var(--ink-2); text-decoration: line-through; }
.more { padding: 8px 12px; }
.perm { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
.foot { display: flex; justify-content: flex-end; gap: 8px; }
@media (max-width: 640px) { .sum { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
