// 浏览器本地偏好（仅保存在当前浏览器，读写失败时使用默认值）
import { ref, watch, type Ref } from 'vue'
import type { ScanPreset, ThemeMode, ViewMode } from './types'

function read<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? fallback : (JSON.parse(raw) as T)
  } catch {
    return fallback
  }
}

function write(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    /* 无痕模式或存储被禁用时忽略 */
  }
}

/** 与 localStorage 同步的 ref */
export function stored<T>(key: string, fallback: T): Ref<T> {
  const r = ref(read(key, fallback)) as Ref<T>
  watch(r, (v) => write(key, v), { deep: true })
  return r
}

export const viewPref = stored<ViewMode>('ls.view2', 'folders')
export const presetsPref = stored<ScanPreset[]>('ls.presets', [])

/** 扫描表单草稿：未扫描就刷新页面时恢复填写的目录（saved_at 为毫秒时间戳） */
export type ScanDraft = Omit<ScanPreset, 'name'> & { saved_at: number }
export const scanDraftPref = stored<ScanDraft | null>('ls.scanDraft', null)

// 主题单独以纯字符串保存，index.html 中的脚本在首屏前读取它
function readTheme(): ThemeMode {
  try {
    const t = localStorage.getItem('ls.theme')
    return t === 'light' || t === 'dark' ? t : 'system'
  } catch {
    return 'system'
  }
}
export const themePref = ref<ThemeMode>(readTheme())
watch(themePref, (t) => {
  try {
    if (t === 'system') localStorage.removeItem('ls.theme')
    else localStorage.setItem('ls.theme', t)
  } catch {
    /* 忽略 */
  }
  if (t === 'system') document.documentElement.removeAttribute('data-theme')
  else document.documentElement.setAttribute('data-theme', t)
})
