import type { Item } from './types'

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
