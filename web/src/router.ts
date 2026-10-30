// 极简 hash 路由（#/results、#/settings/update），不引入 vue-router
import { computed, ref } from 'vue'

export type PageKey = 'dashboard' | 'scan' | 'results' | 'downloaders' | 'trash' | 'logs' | 'settings'

export interface PageMeta {
  key: PageKey
  title: string
  icon: string
  group: string
}

export const pages: PageMeta[] = [
  { key: 'dashboard', title: '仪表盘', icon: 'dashboard', group: '清理' },
  { key: 'scan', title: '扫描', icon: 'scan', group: '清理' },
  { key: 'results', title: '清理结果', icon: 'results', group: '清理' },
  { key: 'downloaders', title: '下载器', icon: 'downloader', group: '管理' },
  { key: 'trash', title: '回收站', icon: 'trash', group: '管理' },
  { key: 'logs', title: '日志', icon: 'logs', group: '系统' },
  { key: 'settings', title: '设置', icon: 'settings', group: '系统' },
]

function parse(): { page: PageKey; sub: string } {
  const [p = '', sub = ''] = location.hash.replace(/^#\/?/, '').split('/')
  const page = pages.find((x) => x.key === p)?.key ?? 'dashboard'
  return { page, sub: decodeURIComponent(sub) }
}

export const route = ref(parse())
window.addEventListener('hashchange', () => {
  route.value = parse()
  window.scrollTo({ top: 0 })
})

export const currentPage = computed(() => pages.find((p) => p.key === route.value.page)!)

export function navigate(page: PageKey, sub = '') {
  const hash = `#/${page}${sub ? `/${encodeURIComponent(sub)}` : ''}`
  if (location.hash === hash) return
  location.hash = hash
}
