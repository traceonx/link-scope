<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NCheckbox, NInput, NModal, NSpin } from 'naive-ui'
import Icon from './Icon.vue'
import { api } from '../api'
import { fmtCount, fmtSize } from '../format'
import type { DeletePreview, DeleteResult, DeleteTarget } from '../types'

const props = defineProps<{ show: boolean; target: DeleteTarget | null; title?: string }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'done', r: DeleteResult): void }>()

const preview = ref<DeletePreview | null>(null)
const loading = ref(false)
const deleting = ref(false)
const error = ref('')
const permanent = ref(false)
const confirmText = ref('')
const expanded = ref(false)
const result = ref<DeleteResult | null>(null)

watch(
  () => props.show,
  async (v) => {
    if (!v || !props.target) return
    preview.value = null
    error.value = ''
    permanent.value = false
    confirmText.value = ''
    expanded.value = false
    result.value = null
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
const valid = computed(() => (pv.value ? pv.value.count - pv.value.errors : 0))
const canConfirm = computed(
  () => !!pv.value && valid.value > 0 && !deleting.value && (!permanent.value || confirmText.value.trim() === '删除'),
)
const shown = computed(() => (pv.value ? (expanded.value ? pv.value.items : pv.value.items.slice(0, 20)) : []))
const skipped = computed(() => pv.value?.items.filter((x) => x.error) ?? [])
const kindLabel = { file: '文件', dir: '文件夹', symlink: '软链接', '': '' } as const

async function run() {
  if (!props.target || !canConfirm.value) return
  deleting.value = true
  try {
    result.value = await api.deletePaths(props.target, permanent.value)
    emit('done', result.value)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    deleting.value = false
  }
}

function close() {
  if (!deleting.value) emit('update:show', false)
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="result ? '清理完成' : (title ?? '清理')"
    style="width: min(620px, calc(100vw - 16px))"
    :bordered="false"
    :mask-closable="!deleting"
    :closable="!deleting"
    @update:show="(v: boolean) => !v && close()"
  >
    <!-- 结果 -->
    <div v-if="result" class="del">
      <div class="sum">
        <div><span>成功</span><b class="c-blue">{{ fmtCount(result.deleted) }} 项</b></div>
        <div><span>失败</span><b :class="result.failed ? 'c-red' : ''">{{ fmtCount(result.failed) }} 项</b></div>
        <div><span>跳过</span><b>{{ fmtCount(skipped.length) }} 项</b></div>
        <div>
          <span>{{ permanent ? '已释放' : '清空回收站后释放' }}</span>
          <b>{{ fmtSize(pv?.freed ?? 0) }}</b>
        </div>
      </div>
      <div v-if="result.errors.length" class="notice notice-err col">
        <b>失败原因</b>
        <span v-for="e in result.errors.slice(0, 10)" :key="e.path" class="small">
          <span class="mono">{{ e.path }}</span>：{{ e.error }}
        </span>
      </div>
      <div v-if="!permanent && result.deleted" class="muted small">
        已移入回收站，可以在右上角「回收站」中恢复；{{ preview?.trash_days }} 天后自动清除。
      </div>
      <div class="muted small">统计数字已按删除结果估算，建议稍后重新扫描。</div>
    </div>

    <!-- 预览 -->
    <NSpin v-else :show="loading">
      <div class="del">
        <div v-if="error" class="notice notice-err">{{ error }}</div>
        <template v-if="pv">
          <div class="sum">
            <div><span>清理项目</span><b>{{ fmtCount(valid) }} 项</b></div>
            <div><span>包含文件</span><b>{{ fmtCount(pv.files) }} 个</b></div>
            <div><span>总大小</span><b>{{ fmtSize(pv.total_size) }}</b></div>
            <div><span>预计释放</span><b class="c-green">{{ fmtSize(pv.freed) }}</b></div>
          </div>

          <div v-if="preview!.seeding.files" class="notice notice-seed">
            <Icon name="seed" :size="18" />
            <span>
              {{ preview!.seeding.files }} 个文件仍在做种（{{ preview!.seeding.sites.join('、') }}）。
              如未满足保种要求，删除后可能被站点判为 H&amp;R。
            </span>
          </div>
          <div v-if="pv.kept" class="muted small">
            其中 {{ pv.kept }} 个文件在媒体库中仍有硬链接（{{ fmtSize(pv.kept_size) }}），删除这里不影响媒体库，也不释放这部分空间。
          </div>
          <div v-if="skipped.length" class="notice notice-warn col">
            <b>{{ skipped.length }} 项将被跳过</b>
            <span v-for="s in skipped.slice(0, 5)" :key="s.path" class="small"><span class="mono">{{ s.path }}</span>：{{ s.error }}</span>
          </div>

          <div class="list">
            <div v-for="it in shown" :key="it.path" class="row" :class="{ bad: it.error }">
              <span class="badge" :class="it.error ? 'badge-red' : 'badge-gray'">{{ it.error ? '跳过' : kindLabel[it.kind] }}</span>
              <span class="mono path">{{ it.path }}</span>
              <span class="meta">
                <template v-if="it.kind === 'dir'">{{ it.files }} 个文件 · </template>{{ it.size ? fmtSize(it.size) : '' }}
              </span>
            </div>
            <button
              v-if="pv.items.length > 20 && !expanded"
              type="button"
              class="more"
              @click="expanded = true"
            >
              展开全部 {{ pv.items.length }} 项
            </button>
            <div v-if="pv.count > pv.items.length" class="muted small more-note">
              另有 {{ fmtCount(pv.count - pv.items.length) }} 项未列出
            </div>
          </div>

          <div class="mode">
            <div class="mode-text">
              <b>{{ permanent ? '永久删除' : '移入回收站' }}</b>
              <span class="muted small">
                <template v-if="permanent">立即删除，无法恢复。</template>
                <template v-else>可在回收站恢复；清空回收站后才真正释放空间，{{ preview!.trash_days }} 天后自动清除。</template>
              </span>
            </div>
            <NCheckbox v-model:checked="permanent">永久删除</NCheckbox>
          </div>
          <NInput
            v-if="permanent"
            v-model:value="confirmText"
            placeholder="输入「删除」确认永久删除"
            :input-props="{ 'aria-label': '输入删除确认' }"
            status="error"
          />
        </template>
      </div>
    </NSpin>

    <template #footer>
      <div class="foot">
        <template v-if="result">
          <NButton type="primary" @click="close">完成</NButton>
        </template>
        <template v-else>
          <NButton :disabled="deleting" @click="close">取消</NButton>
          <NButton type="error" :secondary="!permanent" :disabled="!canConfirm" :loading="deleting" @click="run">
            <template #icon><Icon name="trash" /></template>
            {{ permanent ? '永久删除' : '移入回收站' }}{{ valid ? ` ${fmtCount(valid)} 项` : '' }}
          </NButton>
        </template>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.del { display: flex; flex-direction: column; gap: 12px; min-height: 80px; }
.sum { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; padding: 14px; border-radius: 8px; background: var(--soft); }
.sum div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.sum span { font-size: 12px; color: var(--ink-2); }
.sum b { font-size: 15px; }
.c-green { color: var(--green-ink); }
.c-blue { color: var(--blue-ink); }
.c-red { color: var(--red-ink); }
.notice-seed { background: var(--green-bg); color: var(--green-ink); }
.col { flex-direction: column; gap: 4px; }
.small { font-size: 12px; }
.list { max-height: 280px; overflow-y: auto; border: 1px solid var(--border); border-radius: 8px; }
.row { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; padding: 8px 12px; border-bottom: 1px solid var(--border); }
.path { flex: 1 1 260px; min-width: 0; font-size: 12px; word-break: break-all; }
.meta { font-size: 12px; color: var(--ink-2); }
.row.bad .path { color: var(--ink-2); text-decoration: line-through; }
.more { width: 100%; padding: 8px; border: 0; background: transparent; color: var(--blue); font: inherit; font-size: 13px; cursor: pointer; }
.more:focus-visible { outline: 2px solid var(--blue); outline-offset: -2px; }
.more-note { padding: 8px 12px; }
.mode { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 16px; padding: 12px 14px; border: 1px solid var(--border); border-radius: 8px; }
.mode-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; flex: 1 1 260px; }
.foot { display: flex; justify-content: flex-end; gap: 8px; }
@media (max-width: 560px) { .sum { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
