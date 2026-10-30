export type ItemType = 'file' | 'symlink' | 'strm'
export type Filter =
  | 'all'
  | 'linked'
  | 'unlinked'
  | 'dup'
  | 'symlink'
  | 'strm'
  | 'broken'
  | 'torrent'
  | 'seeding'
  | 'unregistered'
  | 'orphan'
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
  torrents?: TorrentRef[]
}

export type TorrentState = 'seeding' | 'downloading' | 'paused' | 'checking' | 'queued' | 'error'

export interface TorrentRef {
  client: string
  hash: string
  name: string
  site: string
  state: TorrentState
  unregistered?: boolean
  message?: string
}

export interface TorrentSummary {
  fetched_at: number
  clients: { name: string; ok: boolean; error?: string; torrents: number; matched?: number; sample?: string }[]
  sites: { site: string; files: number; size: number; seeding: number; unregistered: number }[]
  matched: number
  seeding: number
  unregistered: number
  orphans: number
  orphan_size: number
}

export interface Summary {
  source: string
  targets: string[]
  recursive: boolean
  min_size: number
  dup_mode: DupMode
  total: number
  total_size: number
  linked_size: number
  unlinked_size: number
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
  stale?: boolean
  torrents?: TorrentSummary
}

export interface JobState {
  status: 'idle' | 'running' | 'done' | 'error' | 'cancelled'
  phase?: 'source' | 'targets' | 'dups' | 'torrents'
  done: number
  total: number
  started?: number
  error?: string
}

export interface VersionInfo {
  version: string
  commit: string
  build_time: string
}

export interface AppConfig {
  version: VersionInfo
  can_delete: boolean
  delete_reason: string
  trash_days: number
  has_downloaders: boolean
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

export interface UpdateProgress {
  percent: number
  current: number
  total: number
  layers: number
  done: number
}

export interface UpdateState {
  running: boolean
  phase?: 'prepare' | 'pull' | 'recreate' | 'restarting'
  progress?: UpdateProgress
  fromVersion?: string
  startedAt?: string
  finishedAt?: string
  success: boolean | null
  message?: string
  log?: string
}

export interface AutoUpdate {
  enabled: boolean
  time: string
  next?: string
}

export interface UpdateStatus {
  current: VersionInfo
  latest: string
  latestError?: string
  imageChanged: boolean | null
  imageError?: string
  hasUpdate: boolean
  autoUpdate: AutoUpdate
  docker: { available: boolean; reason: string; name?: string; image?: string }
  state: UpdateState
}

/** 删除对象：指定路径，或当前筛选的全部结果（文件视图 / 文件夹视图） */
export type DeleteTarget =
  | { paths: string[] }
  | { filter: Filter; site: string; q: string; dir: string }
  | { view: 'folders'; filter: FolderFilter; site: string; q: string }

export type ViewMode = 'folders' | 'files'
export type FolderFilter = 'all' | 'linked' | 'full' | 'partial' | 'unlinked' | 'dup' | 'seeding' | 'unregistered' | 'orphan'

export interface Folder {
  name: string
  path: string
  is_file: boolean
  status: 'linked' | 'partial' | 'unlinked'
  files: number
  linked: number
  unlinked: number
  size: number
  linked_size: number
  dups: number
  symlinks: number
  broken: number
  torrents: number
  seeding: number
  unregistered: number
  orphans: number
  sites: string[]
  mtime: number
}

export interface FolderTotals {
  folders: number
  pending_folders: number
  unlinked_folders: number
  pending_files: number
  pending_size: number
  done_folders: number
}

export interface FolderPage {
  summary: Summary | null
  counts: Record<FolderFilter, number>
  totals: FolderTotals
  count: number
  items: Folder[]
}

export type SortOrder = 'asc' | 'desc'
export type FolderSort = 'name' | 'size' | 'pending' | 'files' | 'unlinked' | 'mtime'
export type FileSort = 'path' | 'name' | 'size' | 'mtime' | 'nlink'

export interface ScanPreset {
  name: string
  source: string
  targets: string[]
  recursive: boolean
  skipSmall: boolean
  dupMode: DupMode
}

export type ThemeMode = 'system' | 'light' | 'dark'

export interface DeletePreviewItem {
  path: string
  kind: 'file' | 'dir' | 'symlink' | ''
  size: number
  files: number
  nlink?: number
  freed: boolean
  error?: string
}

export interface DeletePreview {
  preview: {
    items: DeletePreviewItem[]
    count: number
    files: number
    dirs: number
    symlinks: number
    total_size: number
    freed: number
    kept: number
    kept_size: number
    errors: number
  }
  seeding: { files: number; sites: string[] }
  /** 涉及的种子数（按上次扫描的标注） */
  torrents: number
  trash_days: number
}

export interface DeleteResult {
  deleted: number
  failed: number
  errors: { path: string; error?: string }[]
  /** 勾选「同时删除种子」时返回 */
  torrents?: { removed: { client: string; name: string }[]; partial: number; errors: string[] } | null
}

export interface TrashEntry {
  id: string
  path: string
  name: string
  kind: 'file' | 'dir' | 'symlink'
  size: number
  files: number
  root: string
  deleted_at: number
  expires_at: number
}

export interface Mapping {
  from: string
  to: string
}

export interface Downloader {
  id?: string
  name: string
  type: 'qbittorrent' | 'transmission'
  url: string
  username: string
  password?: string
  mappings: Mapping[]
  enabled: boolean
}
