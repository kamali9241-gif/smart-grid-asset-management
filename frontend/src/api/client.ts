import type { Asset, AssetDetail, ImportMode, ImportReport, Meta } from '../types'

// Vite proxies /api to the backend in dev; in production the app is served
// behind the same origin as the API (see deploy/nginx.conf), so a relative
// base keeps a single build working in both environments.
const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api'

export class ApiError extends Error {
  status: number
  code: string
  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`${API_BASE}${path}`, init)
  } catch {
    throw new ApiError(0, 'network_error', 'Could not reach the server. Check your connection and try again.')
  }
  if (!res.ok) {
    let code = 'unknown_error'
    let message = `Request failed with status ${res.status}`
    try {
      const body = await res.json()
      code = body?.error?.code ?? code
      message = body?.error?.message ?? message
    } catch {
      // response was not JSON; fall back to the generic message
    }
    throw new ApiError(res.status, code, message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export function getMeta(): Promise<Meta> {
  return request('/meta')
}

export function getRoots(): Promise<{ assets: Asset[] }> {
  return request('/assets/roots')
}

export function getAsset(assetId: string): Promise<AssetDetail> {
  return request(`/assets/${encodeURIComponent(assetId)}`)
}

export function getChildren(assetId: string): Promise<{ assets: Asset[] }> {
  return request(`/assets/${encodeURIComponent(assetId)}/children`)
}

export function getAncestors(assetId: string): Promise<{ assets: Asset[] }> {
  return request(`/assets/${encodeURIComponent(assetId)}/ancestors`)
}

export function searchAssets(
  query: string,
  assetType?: string,
  limit = 25,
): Promise<{ assets: Asset[]; count: number }> {
  const params = new URLSearchParams({ q: query, limit: String(limit) })
  if (assetType) params.set('type', assetType)
  return request(`/assets/search?${params.toString()}`)
}

export function uploadImport(file: File, mode: ImportMode): Promise<ImportReport> {
  const form = new FormData()
  form.append('file', file)
  form.append('mode', mode)
  return request('/imports', { method: 'POST', body: form })
}
