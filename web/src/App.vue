<script setup lang="ts">
import { computed } from 'vue'
import {
  NConfigProvider,
  NMessageProvider,
  NDialogProvider,
  darkTheme,
  useOsTheme,
  zhCN,
  dateZhCN,
  type GlobalThemeOverrides,
} from 'naive-ui'
import Home from './views/Home.vue'

const osTheme = useOsTheme()
const dark = computed(() => osTheme.value === 'dark')
const theme = computed(() => (dark.value ? darkTheme : null))

const overrides = computed<GlobalThemeOverrides>(() => {
  const blue = dark.value ? '#5B8DEF' : '#1F5FD6'
  return {
    common: {
      primaryColor: blue,
      primaryColorHover: dark.value ? '#7AA3F2' : '#2C6BE0',
      primaryColorPressed: dark.value ? '#4A7BDB' : '#174AA8',
      primaryColorSuppl: blue,
      borderRadius: '8px',
      fontFamily: "'Noto Sans SC', 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif",
      fontFamilyMono: "'JetBrains Mono', ui-monospace, Menlo, Consolas, monospace",
    },
    DataTable: {
      thColor: dark.value ? '#23272C' : '#F8F9FB',
      thFontWeight: '600',
    },
  }
})
</script>

<template>
  <NConfigProvider :theme="theme" :theme-overrides="overrides" :locale="zhCN" :date-locale="dateZhCN">
    <NMessageProvider>
      <NDialogProvider>
        <Home />
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
