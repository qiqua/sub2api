import { keysAPI } from '@/api/keys'
import { buildGatewayUrl } from '@/api/client'
import {
  getBatchImageItemContent,
  getBatchImageJob,
  listBatchImageItems,
  listBatchImageModels,
  submitBatchImageJob,
  type BatchImageSubmitItem,
} from '@/api/batchImage'
import type { ApiKey } from '@/types'
import { IMAGE_TYPES, MAX_IMAGE_BYTES, type CanvasJob } from './document'

export type CanvasAspectRatio = '1:1' | '3:2' | '2:3'

export interface SubmitCanvasImageOptions {
  key: ApiKey
  model: string
  prompt: string
  reference?: Blob
  requestId: string
  signal?: AbortSignal
  aspectRatio?: CanvasAspectRatio
}

export type SubmitCanvasImageResult = { image: Blob } | { job: CanvasJob }

export type CanvasJobStatus =
  | { status: 'pending' }
  | { status: 'complete'; image: Blob }
  | { status: 'failed'; message: string }

const CANVAS_KEY_PAGE_SIZE = 100
const CANVAS_CUSTOM_ID = 'canvas_image'
const TERMINAL_BATCH_STATUSES = new Set(['failed', 'cancelled', 'output_deleted'])

function isGeminiKey(key: ApiKey): boolean {
  return key.status === 'active' && key.group?.platform === 'gemini' && key.group.allow_batch_image_generation === true
}

function isNativeKey(key: ApiKey): boolean {
  return (
    key.status === 'active' &&
    (key.group?.platform === 'openai' || key.group?.platform === 'grok') &&
    key.group.allow_image_generation === true
  )
}

function isCanvasKey(key: ApiKey): boolean {
  return isGeminiKey(key) || isNativeKey(key)
}

function errorMessage(body: unknown, fallback: string): string {
  if (typeof body === 'string' && body.trim()) return body.trim()
  if (!body || typeof body !== 'object') return fallback
  const candidate = body as Record<string, unknown>
  const nested = candidate.error
  if (nested && typeof nested === 'object' && typeof (nested as Record<string, unknown>).message === 'string') {
    return String((nested as Record<string, unknown>).message)
  }
  if (typeof candidate.message === 'string' && candidate.message.trim()) return candidate.message
  return fallback
}

async function requestError(response: Response): Promise<Error> {
  let body: unknown
  try {
    body = await response.json()
  } catch {
    body = undefined
  }
  return new Error(errorMessage(body, response.statusText || `HTTP ${response.status}`))
}

function gatewayHeaders(key: ApiKey, extra?: HeadersInit): Headers {
  const headers = new Headers(extra)
  headers.set('Authorization', `Bearer ${key.key}`)
  return headers
}

async function gatewayFetch(key: ApiKey, path: string, init: RequestInit = {}): Promise<Response> {
  return fetch(buildGatewayUrl(path), { ...init, headers: gatewayHeaders(key, init.headers), redirect: 'error' })
}

function assertCanvasKey(key: ApiKey): void {
  if (!isCanvasKey(key)) throw new Error('This API key is not enabled for image generation')
}

export async function listCanvasKeys(): Promise<ApiKey[]> {
  const keys: ApiKey[] = []
  let page = 1
  while (true) {
    const response = await keysAPI.list(page, CANVAS_KEY_PAGE_SIZE, {
      status: 'active',
      sort_by: 'created_at',
      sort_order: 'desc',
    })
    const pageItems = response.items || []
    keys.push(...pageItems.filter(isCanvasKey))
    if (!pageItems.length || page >= response.pages) break
    page += 1
  }
  return keys
}

export async function listCanvasModels(key: ApiKey): Promise<string[]> {
  assertCanvasKey(key)
  if (isGeminiKey(key)) {
    const response = await listBatchImageModels(key.key)
    return [...new Set(response.data.map(model => model.id.trim()).filter(Boolean))]
  }

  const response = await gatewayFetch(key, '/v1/models')
  if (!response.ok) throw await requestError(response)
  const body = await response.json() as { data?: Array<{ id?: unknown }> }
  if (!Array.isArray(body.data)) return []
  const candidates = body.data.map(model => typeof model.id === 'string' ? model.id.trim() : '')
  return [...new Set(candidates.filter(model => /image|imagine|dall[-_ ]?e/i.test(model) && !/video/i.test(model)))]
}

function aspectSize(aspectRatio: CanvasAspectRatio): string {
  if (aspectRatio === '3:2') return '1536x1024'
  if (aspectRatio === '2:3') return '1024x1536'
  return '1024x1024'
}

function mimeForBlob(blob: Blob): string {
  const mime = blob.type.toLowerCase().split(';', 1)[0]
  if (!IMAGE_TYPES.includes(mime)) throw new Error('References must be PNG, JPEG, or WebP images')
  if (blob.size > MAX_IMAGE_BYTES) throw new Error('Reference image exceeds 10 MiB')
  return mime
}

async function blobToBase64(blob: Blob): Promise<string> {
  const bytes = new Uint8Array(await blob.arrayBuffer())
  let binary = ''
  const chunkSize = 0x8000
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize))
  }
  return btoa(binary)
}

function extensionForMime(mime: string): string {
  if (mime === 'image/jpeg') return 'jpg'
  if (mime === 'image/webp') return 'webp'
  return 'png'
}

async function parseDataUrl(value: string): Promise<Blob | undefined> {
  if (!value.startsWith('data:')) return undefined
  const match = /^data:(image\/(?:png|jpeg|webp));base64,([A-Za-z0-9+/=\s]+)$/i.exec(value)
  if (!match) throw new Error('The image response contains an invalid data URL')
  if (match[2].length > Math.ceil(MAX_IMAGE_BYTES / 3) * 4) throw new Error('The image response exceeds 10 MiB')
  const binary = atob(match[2].replace(/\s/g, ''))
  const bytes = Uint8Array.from(binary, character => character.charCodeAt(0))
  return checkedImageBlob(new Blob([bytes], { type: match[1].toLowerCase() }))
}

async function checkedImageBlob(blob: Blob): Promise<Blob> {
  if (blob.size === 0 || blob.size > MAX_IMAGE_BYTES) throw new Error('The image response is empty or exceeds 10 MiB')
  const prefix = new Uint8Array(await blob.slice(0, 12).arrayBuffer())
  const png = prefix.length >= 8 && prefix[0] === 0x89 && prefix[1] === 0x50 && prefix[2] === 0x4e && prefix[3] === 0x47
  const jpeg = prefix.length >= 3 && prefix[0] === 0xff && prefix[1] === 0xd8 && prefix[2] === 0xff
  const webp = prefix.length >= 12 && String.fromCharCode(...prefix.slice(0, 4)) === 'RIFF' && String.fromCharCode(...prefix.slice(8, 12)) === 'WEBP'
  if (!png && !jpeg && !webp) throw new Error('The image response is not a supported raster image')
  const type = png ? 'image/png' : jpeg ? 'image/jpeg' : 'image/webp'
  return type === blob.type ? blob : blob.slice(0, blob.size, type)
}

async function fetchImageUrl(value: string, signal?: AbortSignal): Promise<Blob> {
  const dataUrl = await parseDataUrl(value)
  if (dataUrl) return dataUrl
  let url: URL
  try {
    url = new URL(value)
  } catch {
    throw new Error('The image response contains an invalid URL')
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') throw new Error('The image response URL is not supported')
  const response = await fetch(url, {
    credentials: 'omit',
    redirect: 'follow',
    referrerPolicy: 'no-referrer',
    signal,
  })
  if (!response.ok) throw new Error(`Image download failed with HTTP ${response.status}`)
  return checkedImageBlob(await response.blob())
}

async function nativeImageResponse(response: Response, signal?: AbortSignal): Promise<Blob> {
  const contentType = (response.headers.get('content-type') || '').toLowerCase()
  if (contentType.startsWith('image/')) return checkedImageBlob(await response.blob())
  let body: unknown
  try {
    body = await response.json()
  } catch {
    throw new Error('The image gateway returned an invalid response')
  }
  const data = body && typeof body === 'object' && Array.isArray((body as Record<string, unknown>).data)
    ? (body as Record<string, unknown>).data as Array<Record<string, unknown>>
    : []
  const first = data[0]
  if (!first) throw new Error(errorMessage(body, 'The image gateway returned no image'))
  if (typeof first.b64_json === 'string') {
    if (first.b64_json.length > Math.ceil(MAX_IMAGE_BYTES / 3) * 4) throw new Error('The image response exceeds 10 MiB')
    const bytes = Uint8Array.from(atob(first.b64_json), character => character.charCodeAt(0))
    return checkedImageBlob(new Blob([bytes], { type: typeof first.mime_type === 'string' ? first.mime_type : '' }))
  }
  if (typeof first.url === 'string') return fetchImageUrl(first.url, signal)
  throw new Error('The image gateway returned no usable image')
}

export async function submitCanvasImage(options: SubmitCanvasImageOptions): Promise<SubmitCanvasImageResult> {
  const { key, model, prompt, reference, requestId, signal, aspectRatio } = options
  assertCanvasKey(key)
  if (!model.trim() || !prompt.trim()) throw new Error('Choose an image model and enter a prompt')

  if (isGeminiKey(key)) {
    const item: BatchImageSubmitItem = { custom_id: CANVAS_CUSTOM_ID, prompt, output_count: 1 }
    if (reference) {
      const mime = mimeForBlob(reference)
      item.reference_images = [{ id: 'canvas-reference', type: 'reference', mime_type: mime, data: await blobToBase64(reference) }]
    }
    const batch = await submitBatchImageJob(key.key, {
      model,
      aspect_ratio: aspectRatio,
      response_mime_type: 'image/png',
      image_size: '1K',
      items: [item],
    }, requestId, signal)
    return { job: { id: batch.id, keyId: key.id } }
  }

  const isGrok = key.group?.platform === 'grok'
  const headers: HeadersInit = { 'X-Request-ID': requestId }
  let response: Response
  if (reference) {
    const mime = mimeForBlob(reference)
    const form = new FormData()
    form.append('model', model)
    form.append('prompt', prompt)
    form.append('image', reference, `reference.${extensionForMime(mime)}`)
    if (aspectRatio) {
      form.append(isGrok ? 'aspect_ratio' : 'size', isGrok ? aspectRatio : aspectSize(aspectRatio))
    }
    response = await gatewayFetch(key, '/v1/images/edits', { method: 'POST', headers, body: form, signal })
  } else {
    const payload: Record<string, string | number> = { model, prompt, n: 1 }
    if (aspectRatio) payload[isGrok ? 'aspect_ratio' : 'size'] = isGrok ? aspectRatio : aspectSize(aspectRatio)
    if (isGrok) payload.response_format = 'b64_json'
    response = await gatewayFetch(key, '/v1/images/generations', {
      method: 'POST',
      headers: { ...headers, 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
      signal,
    })
  }
  if (!response.ok) throw await requestError(response)
  return { image: await nativeImageResponse(response, signal) }
}

export async function checkCanvasJob(key: ApiKey, job: CanvasJob, signal?: AbortSignal): Promise<CanvasJobStatus> {
  if (key.id !== job.keyId) return { status: 'failed', message: 'This canvas job belongs to another API key' }
  assertCanvasKey(key)
  // Keep the job handle on transport errors so the user can retrieve the same result later.
  const batch = await getBatchImageJob(key.key, job.id, signal)
  if (TERMINAL_BATCH_STATUSES.has(batch.status)) return { status: 'failed', message: `Batch ${job.id}: ${batch.status}` }
  if (batch.status !== 'completed') return { status: 'pending' }
  const items = await listBatchImageItems(key.key, job.id, '', signal)
  const item = items.data.find(entry => entry.custom_id === CANVAS_CUSTOM_ID)
  if (!item || !['completed', 'succeeded'].includes(item.status) || !item.image_count) {
    return { status: 'failed', message: item?.error?.message || 'The task finished without an image' }
  }
  const image = await getBatchImageItemContent(key.key, job.id, item.custom_id, 0, signal)
  return { status: 'complete', image: await checkedImageBlob(image) }
}
