import type { Folder, Item, TorrentRef } from './types'

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
    it.linked ? { label: '已整理', cls: 'badge-blue' } : { label: '待清理', cls: 'badge-orange' },
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

/** 文件夹状态（文件夹内没有任何已链接文件即为待清理）。 */
export const folderStatus: Record<Folder['status'], Badge> = {
  unlinked: { label: '待清理', cls: 'badge-orange' },
  partial: { label: '部分游离', cls: 'badge-violet' },
  linked: { label: '已整理', cls: 'badge-blue' },
}

/** 文件夹的附加标记（与状态可同时存在）。 */
export function folderExtras(f: Folder): Badge[] {
  const out: Badge[] = []
  if (f.dups) out.push({ label: `重复 ${f.dups}`, cls: 'badge-violet' })
  if (f.broken) out.push({ label: `失效 ${f.broken}`, cls: 'badge-red' })
  return out
}

/** 文件夹的种子摘要。 */
export function folderTorrent(f: Folder): Badge | null {
  if (f.unregistered) return { label: `已删种 ${f.unregistered}`, cls: 'badge-red' }
  if (f.seeding) return { label: f.seeding === f.files ? '做种中' : `做种 ${f.seeding}/${f.files}`, cls: 'badge-green' }
  if (f.torrents) return { label: '未做种', cls: 'badge-gray' }
  if (f.orphans) return { label: `孤儿 ${f.orphans}`, cls: 'badge-orange' }
  return null
}
