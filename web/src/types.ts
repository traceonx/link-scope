export type ItemType = 'file' | 'symlink' | 'strm'
export type Filter = 'all' | 'linked' | 'unlinked' | 'dup' | 'symlink' | 'strm' | 'broken'
export type DupMode = 'off' | 'quick' | 'full'

export interface Item {
  type: ItemType
  name: string
  dir: string
  path: string
  size: number
  mtime: number
  nlink?: number
  inode?: number
  linked: boolean
  links?: string[]
  missing?: number
  dups?: string[]
  target?: string
  resolved?: string
  remote?: boolean
  broken: boolean
  tags: string[]
}

export interface Summary {
  source: string
  targets: string[]
  recursive: boolean
  min_size: number
  dup_mode: DupMode
  total: number
  total_size: number
  linked: number
  unlinked: number
  saved: number
  dup_files: number
  dup_groups: number
  dup_wasted: number
  symlinks: number
  broken_links: number
  strm: number
  broken_strm: number
  scanned_at: number
  duration: number
  warnings: string[]
  errors: string[]
  error_count: number
}

export interface JobState {
  status: 'idle' | 'running' | 'done' | 'error' | 'cancelled'
  phase?: 'source' | 'targets' | 'dups'
  done: number
  total: number
  started?: number
  error?: string
}

export interface AppConfig {
  version: string
  roots: string[]
  default_source: string
  default_targets: string[]
}

export interface BrowseResult {
  path: string
  parent: string | null
  dirs: { name: string; path: string }[]
}

export interface ResultPage {
  summary: Summary | null
  counts: Record<Filter, number>
  count: number
  items: Item[]
}

export interface ScanRequest {
  source: string
  targets: string[]
  recursive: boolean
  min_size_mb: number
  dup_mode: DupMode
}
