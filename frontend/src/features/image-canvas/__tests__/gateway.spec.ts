// @vitest-environment node
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '@/types'
import { checkCanvasJob, submitCanvasImage } from '../gateway'

vi.mock('@/api/keys', () => ({ keysAPI: {} }))
vi.mock('@/api/client', () => ({ buildGatewayUrl: (path: string) => `https://relay.example${path}` }))
vi.mock('@/dev/previewMode', () => ({ isPreviewMode: false }))

const key = { id: 7, key: 'test-credential', status: 'active', group: { platform: 'gemini', allow_batch_image_generation: true } } as ApiKey
const request = { key, model: 'gemini-3-pro-image-preview', prompt: 'A lighthouse', requestId: 'request-1' }
const owner = { id: 'existing-batch', keyId: key.id }
const png = new Blob([Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='), c => c.charCodeAt(0))], { type: 'image/png' })

afterEach(() => vi.unstubAllGlobals())

describe('image canvas gateway', () => {
  it('sends the chosen aspect and reference in a single billable batch item', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>().mockResolvedValue(Response.json({ id: owner.id }))
    vi.stubGlobal('fetch', fetch)
    expect(await submitCanvasImage({ ...request, aspectRatio: '2:3', reference: png })).toEqual({ job: owner })
    expect(fetch).toHaveBeenCalledTimes(1)
    const [url, init] = fetch.mock.calls[0]
    expect(url).toBe('https://relay.example/v1/images/batches')
    expect(new Headers(init?.headers).get('Idempotency-Key')).toBe('request-1')
    const payload = JSON.parse(String(init?.body))
    expect(payload).toMatchObject({ aspect_ratio: '2:3', model: request.model, items: [{ prompt: request.prompt, reference_images: [{ mime_type: 'image/png' }] }] })
    expect(atob(payload.items[0].reference_images[0].data).length).toBe(png.size)
  })

  it('can retrieve the same job after a lost status response without creating a second job', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>()
      .mockRejectedValueOnce(new TypeError('Network offline'))
      .mockResolvedValueOnce(Response.json({ status: 'completed' }))
      .mockResolvedValueOnce(Response.json({ data: [{ custom_id: 'canvas_image', status: 'succeeded', image_count: 1 }] }))
      .mockResolvedValueOnce(new Response(png))
    vi.stubGlobal('fetch', fetch)
    const { signal } = new AbortController()
    await expect(checkCanvasJob(key, owner, signal)).rejects.toThrow('Network offline')
    const result = await checkCanvasJob(key, owner, signal)
    expect(result.status).toBe('complete')
    expect(fetch.mock.calls.every(([url, init]) => String(url).includes('/batches/existing-batch') && (init?.method ?? 'GET') === 'GET' && init?.signal === signal)).toBe(true)
  })

  it('uses multipart edits and retrieves URL results without disclosing the billing key', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>()
      .mockResolvedValueOnce(Response.json({ data: [{ url: 'https://images.example/result.png' }] }))
      .mockResolvedValueOnce(new Response(png))
    vi.stubGlobal('fetch', fetch)
    const nativeKey = { ...key, group: { platform: 'openai', allow_image_generation: true } } as ApiKey
    const result = await submitCanvasImage({ ...request, key: nativeKey, model: 'gpt-image-2', reference: png, aspectRatio: '3:2' })
    const [url, init] = fetch.mock.calls[0]
    expect(url).toBe('https://relay.example/v1/images/edits')
    expect(new Headers(init?.headers).get('Authorization')).toBe('Bearer test-credential')
    expect((init?.body as FormData).get('size')).toBe('1536x1024')
    expect(((init?.body as FormData).get('image') as Blob).size).toBe(png.size)
    expect(new Headers(fetch.mock.calls[1][1]?.headers).has('Authorization')).toBe(false)
    expect(fetch.mock.calls[1][1]?.credentials).toBe('omit')
    expect('image' in result && result.image.type).toBe('image/png')
  })
})
