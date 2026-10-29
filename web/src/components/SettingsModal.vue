<script setup lang="ts">
import { NButton, NModal, NRadioButton, NRadioGroup } from 'naive-ui'
import { themePref, viewPref } from '../prefs'
import type { AppConfig } from '../types'

defineProps<{ show: boolean; cfg: AppConfig | null }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void; (e: 'open-update'): void }>()
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="设置"
    style="width: min(520px, calc(100vw - 24px))"
    :bordered="false"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <div class="settings">
      <section class="row">
        <div class="label-col">
          <span class="label">外观</span>
          <span class="hint">保存在当前浏览器</span>
        </div>
        <NRadioGroup v-model:value="themePref" size="small" aria-label="外观">
          <NRadioButton value="system">跟随系统</NRadioButton>
          <NRadioButton value="light">浅色</NRadioButton>
          <NRadioButton value="dark">深色</NRadioButton>
        </NRadioGroup>
      </section>

      <section class="row">
        <div class="label-col">
          <span class="label">默认显示方式</span>
          <span class="hint">打开页面时的结果视图</span>
        </div>
        <NRadioGroup v-model:value="viewPref" size="small" aria-label="默认显示方式">
          <NRadioButton value="folders">按文件夹</NRadioButton>
          <NRadioButton value="files">按文件</NRadioButton>
        </NRadioGroup>
      </section>

      <section class="row">
        <div class="label-col">
          <span class="label">版本与更新</span>
          <span class="hint">一键更新、每天自动更新</span>
        </div>
        <NButton size="small" @click="emit('open-update')">打开</NButton>
      </section>

      <section v-if="cfg" class="info">
        <span class="label">运行配置</span>
        <dl>
          <dt>允许访问的目录</dt>
          <dd class="mono">{{ cfg.roots.length ? cfg.roots.join('、') : '不限制' }}</dd>
          <dt>删除功能</dt>
          <dd>
            <template v-if="cfg.can_delete">可用 · 回收站保留 {{ cfg.trash_days }} 天</template>
            <template v-else>不可用：{{ cfg.delete_reason }}</template>
          </dd>
          <dt>默认检测目录</dt>
          <dd class="mono">{{ cfg.default_source || '未设置' }}</dd>
        </dl>
        <p class="hint">以上由 docker-compose 中的环境变量决定（LS_ROOTS、LS_DEFAULT_SOURCE、LS_DEFAULT_TARGETS 等），修改后重建容器生效。</p>
      </section>
    </div>
  </NModal>
</template>

<style scoped>
.settings { display: flex; flex-direction: column; gap: 18px; }
.row { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; }
.label-col { display: flex; flex-direction: column; gap: 2px; }
.label { font-size: 14px; font-weight: 600; }
.hint { font-size: 12px; color: var(--ink-2); margin: 0; line-height: 18px; }
.info { display: flex; flex-direction: column; gap: 8px; padding-top: 14px; border-top: 1px solid var(--border); }
dl { display: grid; grid-template-columns: auto 1fr; gap: 6px 12px; margin: 0; font-size: 13px; }
dt { color: var(--ink-2); }
dd { margin: 0; word-break: break-all; }
</style>
