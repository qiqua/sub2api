// @vitest-environment node
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '@/types'
import { keysAPI } from '@/api/keys'
import { checkCanvasJob, isTerminalCanvasSubmitError, listCanvasKeys, lookupCanvasJob, submitCanvasImage, submitCanvasIntent } from '../gateway'
import { previewLookupBatchImageJob, previewSubmitBatchImageJob } from '@/dev/previewBatchImage'

vi.mock('@/api/keys', () => ({ keysAPI: { list: vi.fn() } }))
vi.mock('@/api/client', () => ({ buildGatewayUrl: (path: string) => `https://relay.example${path}` }))
vi.mock('@/dev/previewMode', () => ({ isPreviewMode: false }))

const key = { id: 7, key: 'test-credential', status: 'active', group: { platform: 'gemini', allow_batch_image_generation: true } } as ApiKey
const request = { key, model: 'gemini-3-pro-image-preview', prompt: 'A lighthouse', requestId: 'request-1' }
const owner = { id: 'existing-batch', keyId: key.id }
const png = new Blob([Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='), c => c.charCodeAt(0))], { type: 'image/png' })

afterEach(() => vi.unstubAllGlobals())

describe('image canvas gateway', () => {
  const movedKey = { ...key, group: undefined }
  const intent = { requestId: request.requestId, keyId: key.id, model: request.model, prompt: request.prompt, aspectRatio: '2:3' as const, reference: png }

  it('includes active keys with changed permissions only when listing recovery keys', async () => {
    vi.mocked(keysAPI.list).mockResolvedValue({ items: [key, movedKey, { ...key, status: 'inactive' }], pages: 1, page: 1, page_size: 100, total: 3 })
    expect(await listCanvasKeys()).toEqual([key])
    expect(await listCanvasKeys(true)).toEqual([key, movedKey])
  })

  it('looks up and polls a job with its original key after a group change', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>()
      .mockResolvedValueOnce(Response.json({ id: owner.id }))
      .mockResolvedValueOnce(Response.json({ status: 'queued' }))
    vi.stubGlobal('fetch', fetch)
    const { signal } = new AbortController()
    expect(await lookupCanvasJob(movedKey, request.requestId, signal)).toEqual(owner)
    const [url, init] = fetch.mock.calls[0]
    expect(url).toBe('https://relay.example/v1/images/batches/by-idempotency-key')
    expect(init?.method ?? 'GET').toBe('GET')
    expect(init?.signal).toBe(signal)
    expect(new Headers(init?.headers).get('Authorization')).toBe(`Bearer ${key.key}`)
    expect(new Headers(init?.headers).get('Idempotency-Key')).toBe(request.requestId)
    expect(await checkCanvasJob(movedKey, owner)).toEqual({ status: 'pending' })
  })

  it('reports a missing job only for a structured job-not-found 404', async () => {
    const code = 'BATCH_IMAGE_NOT_FOUND'
    const fetch = vi.fn<typeof globalThis.fetch>()
      .mockResolvedValueOnce(Response.json({ error: { code } }, { status: 404 }))
      .mockResolvedValueOnce(new Response('route missing', { status: 404 }))
      .mockResolvedValueOnce(Response.json({ error: { code } }, { status: 503 }))
      .mockRejectedValueOnce(new TypeError('offline'))
    vi.stubGlobal('fetch', fetch)
    await expect(lookupCanvasJob(key, request.requestId)).resolves.toBeUndefined()
    await expect(lookupCanvasJob(key, request.requestId)).rejects.toMatchObject({ status: 404 })
    await expect(lookupCanvasJob(key, request.requestId)).rejects.toMatchObject({ status: 503 })
    await expect(lookupCanvasJob(key, request.requestId)).rejects.toThrow('offline')
  })

  it('replays the original POST bytes and request ID only with the original key', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>().mockImplementation(async () => Response.json({ id: owner.id }))
    vi.stubGlobal('fetch', fetch)
    await submitCanvasImage({ ...request, aspectRatio: intent.aspectRatio, reference: png })
    await expect(submitCanvasIntent({ ...key, id: key.id + 1 }, intent)).rejects.toThrow()
    const { signal } = new AbortController()
    expect(await submitCanvasIntent(key, intent, signal)).toEqual(owner)
    expect(fetch).toHaveBeenCalledTimes(2)
    const [url, init] = fetch.mock.calls[1]
    expect(url).toBe(fetch.mock.calls[0][0])
    expect(init).toMatchObject({ body: fetch.mock.calls[0][1]?.body, method: 'POST', signal })
    expect(new Headers(init?.headers).get('Idempotency-Key')).toBe(intent.requestId)
  })

  it('recognizes explicit permanent submit failures and keeps ambiguous failures recoverable', () => {
    expect(isTerminalCanvasSubmitError({ status: 400, code: 'BATCH_IMAGE_INVALID_MODEL' })).toBe(true)
    for (const err of [new TypeError('offline'), { status: 400 }, { status: 409 }, { status: 429 }, { status: 502, code: 'BATCH_IMAGE_INVALID_MODEL' }]) {
      expect(isTerminalCanvasSubmitError(err)).toBe(false)
    }
  })

  it('scopes preview lookup and replay to the credential and idempotency key', () => {
    const payload = { model: request.model, items: [{ custom_id: 'canvas_image', prompt: request.prompt }] }
    const job = previewSubmitBatchImageJob(key.key, payload, request.requestId)
    expect(previewLookupBatchImageJob(key.key, request.requestId)).toEqual(job)
    expect(previewLookupBatchImageJob('other', request.requestId)).toBeUndefined()
    expect(previewSubmitBatchImageJob(key.key, payload, request.requestId)).toEqual(job)
    expect(() => previewSubmitBatchImageJob(key.key, { ...payload, model: 'changed' }, request.requestId)).toThrow()
    expect(previewSubmitBatchImageJob('other', payload, request.requestId).id).not.toBe(job.id)
  })

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
