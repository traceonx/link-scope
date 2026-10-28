import type { Item, TorrentRef } from './types'

export interface Badge {
  label: string
  cls: string
}

/** 一条结果对应的状态标签。 */
export function badgesOf(it: Item): Badge[] {
  if (it.type === 'symlink') {
    return it.broken
      ? [{ label: '软链接·失效', cls: 'badge-red' }]
      : [{ label: '软链接', cls: 'badge-gray' }]
  }
  if (it.type === 'strm') {
    if (it.broken) return [{ label: 'STRM·失效', cls: 'badge-red' }]
    return [{ label: it.remote ? 'STRM·远程' : 'STRM', cls: 'badge-teal' }]
  }
  const out: Badge[] = [
    it.linked ? { label: '已链接', cls: 'badge-blue' } : { label: '未链接', cls: 'badge-orange' },
  ]
  if (it.dups?.length) out.push({ label: '重复副本', cls: 'badge-violet' })
  return out
}

const stateLabel: Record<string, string> = {
  seeding: '做种中',
  downloading: '下载中',
  paused: '已暂停',
  checking: '校验中',
  queued: '排队中',
  error: '出错',
}

/** 单个种子的状态标签。 */
export function torrentBadge(t: TorrentRef): Badge {
  if (t.unregistered) return { label: '站点已删种', cls: 'badge-red' }
  const label = stateLabel[t.state] ?? t.state
  if (t.state === 'seeding') return { label, cls: 'badge-green' }
  if (t.state === 'error') return { label, cls: 'badge-red' }
  if (t.state === 'downloading' || t.state === 'checking' || t.state === 'queued') return { label, cls: 'badge-blue' }
  return { label, cls: 'badge-gray' }
}

/** 条目的种子摘要：站点 + 最重要的状态。 */
export function torrentSummaryOf(it: Item): { sites: string[]; badge: Badge | null } {
  const ts = it.torrents ?? []
  if (!ts.length) {
    return { sites: [], badge: it.tags.includes('orphan') ? { label: '孤儿文件', cls: 'badge-orange' } : null }
  }
  const sites = [...new Set(ts.map((t) => t.site))]
  const pick =
    ts.find((t) => t.unregistered) ?? ts.find((t) => t.state === 'seeding') ?? ts.find((t) => t.state !== 'paused') ?? ts[0]
  return { sites, badge: torrentBadge(pick) }
}
