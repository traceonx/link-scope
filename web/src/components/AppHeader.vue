<script setup lang="ts">
import { NButton } from 'naive-ui'
import Icon from './Icon.vue'
import type { AppConfig } from '../types'

defineProps<{ cfg: AppConfig | null; downloaders: number; hasUpdate: boolean }>()
const emit = defineEmits<{
  (e: 'downloaders'): void
  (e: 'trash'): void
  (e: 'settings'): void
  (e: 'update'): void
}>()
</script>

<template>
  <header class="topbar">
    <div class="wrap topbar-in">
      <div class="logo" aria-hidden="true"><Icon name="link" :size="20" /></div>
      <div class="brand">
        <span class="brand-name">LinkScope <span class="brand-cn">链接透视</span></span>
        <span class="brand-sub">找出没被媒体库用上的下载文件</span>
      </div>
      <nav class="actions" aria-label="功能">
        <NButton quaternary size="small" @click="emit('downloaders')">
          <template #icon><Icon name="server" /></template>
          <span class="label">下载器<template v-if="downloaders"> · {{ downloaders }}</template></span>
        </NButton>
        <NButton v-if="cfg?.can_delete" quaternary size="small" @click="emit('trash')">
          <template #icon><Icon name="trash" /></template>
          <span class="label">回收站</span>
        </NButton>
        <NButton quaternary size="small" aria-label="设置" @click="emit('settings')">
          <template #icon><Icon name="settings" /></template>
          <span class="label">设置</span>
        </NButton>
        <button
          v-if="cfg"
          type="button"
          class="version"
          :title="`提交 ${cfg.version.commit}`"
          :aria-label="hasUpdate ? `版本 ${cfg.version.version}，有新版本` : `版本 ${cfg.version.version}`"
          @click="emit('update')"
        >
          v{{ cfg.version.version }}
          <span v-if="hasUpdate" class="dot" />
        </button>
      </nav>
    </div>
  </header>
</template>

<style scoped>
.topbar { background: var(--surface); border-bottom: 1px solid var(--border); position: sticky; top: 0; z-index: 20; }
.topbar-in { display: flex; align-items: center; gap: 12px; height: 60px; }
.logo { width: 34px; height: 34px; border-radius: 8px; background: var(--blue); color: #fff; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.brand { display: flex; flex-direction: column; flex: 1; min-width: 0; }
.brand-name { font-size: 16px; font-weight: 700; white-space: nowrap; }
.brand-cn { font-weight: 500; color: var(--ink-2); margin-left: 4px; }
.brand-sub { font-size: 12px; color: var(--ink-2); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.actions { display: flex; align-items: center; gap: 2px; }
.actions :deep(.n-button) { color: var(--ink-2); }
.version {
  position: relative; height: 30px; margin-left: 6px; padding: 0 10px; border-radius: 6px; border: 1px solid var(--border);
  background: transparent; color: var(--ink-2); font: inherit; font-size: 12px; cursor: pointer;
}
.version:hover { border-color: var(--border-strong); color: var(--ink); }
.version:focus-visible { outline: 2px solid var(--blue); outline-offset: 2px; }
.dot { position: absolute; top: -3px; right: -3px; width: 9px; height: 9px; border-radius: 50%; background: var(--orange); border: 2px solid var(--surface); }
@media (max-width: 720px) {
  .label { display: none; }
  .brand-sub { display: none; }
}
</style>
