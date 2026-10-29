import type {
  AppConfig,
  AutoUpdate,
  BrowseResult,
  DeletePreview,
  DeleteResult,
  DeleteTarget,
  Downloader,
  FolderFilter,
  FolderPage,
  Filter,
  JobState,
  ResultPage,
  ScanRequest,
  Summary,
  TorrentSummary,
  TrashEntry,
  UpdateState,
  UpdateStatus,
  VersionInfo,
} from './types'

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
  if (!res.ok) {
    let msg = `请求失败（${res.status}）`
    try {
      const body = await res.json()
      if (body?.detail) msg = body.detail
    } catch {
      /* 非 JSON 响应 */
    }
    throw new Error(msg)
  }
  return res.json() as Promise<T>
}

export const api = {
  health: () => request<VersionInfo & { status: string }>('api/health'),
  config: () => request<AppConfig>('api/config'),
  updateStatus: () => request<UpdateStatus>('api/update'),
  startUpdate: () => request<UpdateState>('api/update', { method: 'POST' }),
  setAutoUpdate: (enabled: boolean, time: string) =>
    request<AutoUpdate>('api/update/auto', { method: 'PUT', body: JSON.stringify({ enabled, time }) }),
  browse: (path = '') => request<BrowseResult>(`api/browse?path=${encodeURIComponent(path)}`),
  scan: (body: ScanRequest) => request<{ ok: boolean }>('api/scan', { method: 'POST', body: JSON.stringify(body) }),
  cancel: () => request<{ ok: boolean }>('api/scan/cancel', { method: 'POST' }),
  status: () => request<{ state: JobState; summary: Summary | null }>('api/status'),
  result: (filter: Filter, site: string, q: string, offset: number, limit: number, dir = '') =>
    request<ResultPage>(
      `api/result?filter=${filter}&site=${encodeURIComponent(site)}&q=${encodeURIComponent(q)}&dir=${encodeURIComponent(dir)}&offset=${offset}&limit=${limit}`,
    ),
  exportUrl: (filter: Filter, site: string, q: string, dir = '') =>
    `api/export.csv?filter=${filter}&site=${encodeURIComponent(site)}&q=${encodeURIComponent(q)}&dir=${encodeURIComponent(dir)}`,
  folders: (filter: FolderFilter, site: string, q: string, offset: number, limit: number) =>
    request<FolderPage>(
      `api/folders?filter=${filter}&site=${encodeURIComponent(site)}&q=${encodeURIComponent(q)}&offset=${offset}&limit=${limit}`,
    ),
  foldersExportUrl: (filter: FolderFilter, site: string, q: string) =>
    `api/folders.csv?filter=${filter}&site=${encodeURIComponent(site)}&q=${encodeURIComponent(q)}`,

  deletePreview: (t: DeleteTarget) =>
    request<DeletePreview>('api/delete/preview', { method: 'POST', body: JSON.stringify(t) }),
  deletePaths: (t: DeleteTarget, permanent: boolean) =>
    request<DeleteResult>('api/delete', { method: 'POST', body: JSON.stringify({ ...t, permanent }) }),
  trash: () => request<{ items: TrashEntry[]; days: number }>('api/trash'),
  trashRestore: (ids: string[]) =>
    request<{ restored: number; results: { path: string; ok: boolean; error?: string }[] }>('api/trash/restore', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    }),
  trashPurge: (ids: string[], all = false) =>
    request<{ results: unknown[] }>('api/trash/purge', { method: 'POST', body: JSON.stringify({ ids, all }) }),

  downloaders: () => request<Downloader[]>('api/downloaders'),
  saveDownloader: (d: Downloader) => request<Downloader>('api/downloaders', { method: 'POST', body: JSON.stringify(d) }),
  testDownloader: (d: Downloader) =>
    request<{ torrents: number }>('api/downloaders/test', { method: 'POST', body: JSON.stringify(d) }),
  deleteDownloader: (id: string) => request<{ ok: boolean }>(`api/downloaders/${id}`, { method: 'DELETE' }),
  refreshTorrents: () => request<TorrentSummary>('api/torrents/refresh', { method: 'POST' }),
}
