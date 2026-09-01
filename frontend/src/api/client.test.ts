import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { ApiError, getAsset, searchAssets, uploadImport } from './client'

const originalFetch = global.fetch

function mockFetchOnce(status: number, body: unknown) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  }) as unknown as typeof fetch
}

describe('api client', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })
  afterEach(() => {
    global.fetch = originalFetch
  })

  it('returns parsed JSON on success', async () => {
    mockFetchOnce(200, { asset: { assetId: 'SUB-1' } })
    const result = await getAsset('SUB-1')
    expect(result).toEqual({ asset: { assetId: 'SUB-1' } })
  })

  it('throws an ApiError with the server-provided message on failure', async () => {
    mockFetchOnce(404, { error: { code: 'asset_not_found', message: 'no asset with id SUB-9' } })
    await expect(getAsset('SUB-9')).rejects.toMatchObject({
      code: 'asset_not_found',
      message: 'no asset with id SUB-9',
    })
  })

  it('builds the search query string with an optional type filter', async () => {
    mockFetchOnce(200, { assets: [], count: 0 })
    await searchAssets('panel', 'SWITCHBOARD_PANEL', 10)
    const calledUrl = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string
    expect(calledUrl).toContain('q=panel')
    expect(calledUrl).toContain('type=SWITCHBOARD_PANEL')
    expect(calledUrl).toContain('limit=10')
  })

  it('sends the file and mode as multipart form data on upload', async () => {
    mockFetchOnce(201, { committed: true, rejections: [] })
    const file = new File(['a,b'], 'assets.csv', { type: 'text/csv' })
    await uploadImport(file, 'partial')
    const [, init] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(init.method).toBe('POST')
    expect(init.body).toBeInstanceOf(FormData)
  })

  it('treats a 422 import response as a valid report, not an error', async () => {
    const report = { committed: false, totalRows: 222, importedRows: 0, rejectedRows: 17, rejections: [] }
    mockFetchOnce(422, report)
    const file = new File(['a,b'], 'assets.csv', { type: 'text/csv' })
    await expect(uploadImport(file, 'all_or_nothing')).resolves.toEqual(report)
  })

  it('wraps network failures in an ApiError', async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error('offline')) as unknown as typeof fetch
    await expect(getAsset('SUB-1')).rejects.toBeInstanceOf(ApiError)
  })
})
