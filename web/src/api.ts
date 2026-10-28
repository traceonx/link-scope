import type {
  AppConfig,
  AutoUpdate,
  BrowseResult,
  Filter,
  JobState,
  ResultPage,
  ScanRequest,
  Summary,
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
  result: (filter: Filter, q: string, offset: number, limit: number) =>
    request<ResultPage>(
      `api/result?filter=${filter}&q=${encodeURIComponent(q)}&offset=${offset}&limit=${limit}`,
    ),
  exportUrl: (filter: Filter, q: string) => `api/export.csv?filter=${filter}&q=${encodeURIComponent(q)}`,
}
