export interface Point {
  x: number
  y: number
}

export interface Viewport extends Point {
  zoom: number
}

export interface CanvasJob {
  id: string
  keyId: number
}

export interface CanvasNode extends Point {
  id: string
  kind: 'prompt' | 'image'
  text: string
  parentId?: string
  image?: Blob
  model?: string
  job?: CanvasJob
  error?: string
}

export interface CanvasBoard {
  id: string
  name: string
  nodes: CanvasNode[]
  viewport: Viewport
}

export const IMAGE_TYPES: readonly string[] = ['image/png', 'image/jpeg', 'image/webp']
export const MAX_IMAGE_BYTES = 10 * 1024 * 1024
export const MAX_TOTAL_IMAGE_BYTES = 32 * 1024 * 1024
export const MAX_NODES = 256

export function newId(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  // getRandomValues is also available on non-secure HTTP origins.
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export function createBoard(name: string): CanvasBoard {
  return { id: newId(), name, nodes: [], viewport: { x: 0, y: 0, zoom: 1 } }
}
