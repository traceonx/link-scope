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
import AppRoot from './AppRoot.vue'
import { themePref } from './prefs'

const osTheme = useOsTheme()
const dark = computed(() => (themePref.value === 'system' ? osTheme.value === 'dark' : themePref.value === 'dark'))
const theme = computed(() => (dark.value ? darkTheme : null))

// Naive UI 主题对齐 MoviePilot（Materio）配色；颜色值与 style.css 中的变量保持一致
const overrides = computed<GlobalThemeOverrides>(() => {
  const d = dark.value
  const primary = '#9155FD'
  return {
    common: {
      primaryColor: primary,
      primaryColorHover: '#A272FD',
      primaryColorPressed: '#804BDF',
      primaryColorSuppl: primary,
      infoColor: '#16B1FF',
      infoColorHover: '#45C1FF',
      infoColorPressed: '#149CE0',
      infoColorSuppl: '#16B1FF',
      successColor: '#56CA00',
      successColorHover: '#78D533',
      successColorPressed: '#4CB200',
      successColorSuppl: '#56CA00',
      warningColor: '#FFB400',
      warningColorHover: '#FFC333',
      warningColorPressed: '#E09E00',
      warningColorSuppl: '#FFB400',
      errorColor: '#FF4C51',
      errorColorHover: '#FF7074',
      errorColorPressed: '#E04347',
      errorColorSuppl: '#FF4C51',
      borderRadius: '6px',
      borderRadiusSmall: '4px',
      fontFamily: "'Noto Sans SC', 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif",
      fontFamilyMono: "'JetBrains Mono', ui-monospace, Menlo, Consolas, monospace",
      ...(d
        ? {
            bodyColor: '#28243D',
            cardColor: '#312D4B',
            modalColor: '#312D4B',
            popoverColor: '#3A3556',
            tableColor: '#312D4B',
            tableHeaderColor: '#3A3556',
            inputColor: 'rgba(231, 227, 252, 0.05)',
            textColorBase: '#E7E3FC',
            textColor1: 'rgba(231, 227, 252, 0.92)',
            textColor2: 'rgba(231, 227, 252, 0.82)',
            textColor3: 'rgba(231, 227, 252, 0.6)',
            dividerColor: '#46425F',
            borderColor: '#5F5A7E',
            hoverColor: 'rgba(231, 227, 252, 0.07)',
          }
        : {
            bodyColor: '#F4F5FA',
            textColorBase: '#3A3541',
            textColor1: 'rgba(58, 53, 65, 0.92)',
            textColor2: 'rgba(58, 53, 65, 0.8)',
            textColor3: 'rgba(58, 53, 65, 0.56)',
            dividerColor: '#E7E6EB',
            borderColor: '#D8D6DE',
          }),
    },
    Card: { borderRadius: '10px', titleFontSizeMedium: '17px', titleFontWeight: '600' },
    Button: { fontWeight: '500' },
    DataTable: {
      thColor: d ? '#3A3556' : '#F9F8FC',
      thFontWeight: '600',
      tdColor: d ? '#312D4B' : '#FFFFFF',
      borderColor: d ? '#46425F' : '#EDECF0',
    },
    Tabs: { tabFontWeightActive: '600' },
    Drawer: { color: d ? '#312D4B' : '#FFFFFF' },
  }
})
</script>

<template>
  <NConfigProvider :theme="theme" :theme-overrides="overrides" :locale="zhCN" :date-locale="dateZhCN">
    <NMessageProvider>
      <NDialogProvider>
        <AppRoot />
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
