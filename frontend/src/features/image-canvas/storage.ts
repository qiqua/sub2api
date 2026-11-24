import { boardImageBytes, IMAGE_TYPES, MAX_IMAGE_BYTES, MAX_NODES, MAX_TOTAL_IMAGE_BYTES, newId, type CanvasBoard, type CanvasNode, type CanvasSubmitIntent, type Viewport } from './document'

const MAX_ARCHIVE_BYTES = 48 * 1024 * 1024
const MAX_IMAGE_PIXELS = 32 * 1024 * 1024
const FORMAT = 'sub2api-image-canvas'

export class CanvasArchiveError extends Error {
  constructor(
    readonly code: 'unfinished-node' | 'invalid-document' | 'limit-exceeded' | 'invalid-image',
    message: string,
    readonly nodeId?: string,
  ) {
    super(message)
    this.name = 'CanvasArchiveError'
  }
}

export class CanvasConflictError extends Error {
  constructor(readonly committed?: CanvasBoard, readonly version = committed?.version) {
    super('Canvas changed in another tab.')
    this.name = 'CanvasConflictError'
  }
}

function invalid(message: string): never {
  throw new CanvasArchiveError('invalid-document', message)
}

function limit(message: string): never {
  throw new CanvasArchiveError('limit-exceeded', message)
}

function invalidImage(): never {
  throw new CanvasArchiveError('invalid-image', 'Expected a readable PNG, JPEG, or WebP image.')
}

function record(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) invalid('Expected a document object.')
  return value as Record<string, unknown>
}

function onlyFields(value: Record<string, unknown>, allowed: readonly string[]): void {
  if (Object.keys(value).some(key => !allowed.includes(key))) invalid('Unexpected document field.')
}

function string(value: unknown, max: number, field: string): string {
  if (typeof value !== 'string') invalid(`Expected a string for ${field}.`)
  if (value.length > max) limit(`${field} exceeds ${max} characters.`)
  return value
}

function id(value: unknown): string {
  const result = string(value, 128, 'ID')
  if (!/^[a-zA-Z0-9_-]+$/.test(result) || result in Object.prototype) invalid('Invalid canvas ID.')
  return result
}

function boardName(value: unknown): string {
  const result = string(value, 120, 'Board name')
  if (!result.trim()) invalid('The board needs a name.')
  return result
}

function coordinate(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) invalid('Expected a finite coordinate.')
  if (Math.abs(value) > 10_000_000) limit('Coordinate exceeds the canvas bounds.')
  return value
}

function viewport(value: unknown): Viewport {
  const view = record(value)
  const { zoom } = view
  if (typeof zoom !== 'number' || !Number.isFinite(zoom)) invalid('Expected a finite zoom.')
  if (zoom < 0.05 || zoom > 16) limit('Zoom must be between 0.05 and 16.')
  return { x: coordinate(view.x), y: coordinate(view.y), zoom }
}

function nodeList(value: unknown): unknown[] {
  if (!Array.isArray(value)) invalid('Expected a node list.')
  if (value.length > MAX_NODES) limit(`A canvas can have at most ${MAX_NODES} nodes.`)
  return value
}

// Explicitly project every archive field. Local errors and job/key references must stay local.
function nodeFields(value: Record<string, unknown>): CanvasNode {
  const { kind } = value
  if (kind !== 'prompt' && kind !== 'image') invalid('Unknown canvas node kind.')
  const node: CanvasNode = {
    id: id(value.id), kind, x: coordinate(value.x), y: coordinate(value.y), text: string(value.text, 32_000, 'Node text'),
  }
  if (value.parentId !== undefined) node.parentId = id(value.parentId)
  if (value.model !== undefined) node.model = string(value.model, 256, 'Model')
  return node
}

function intentFields(value: unknown, node: CanvasNode): CanvasSubmitIntent {
  const raw = record(value)
  onlyFields(raw, ['requestId', 'keyId', 'prompt', 'model', 'aspectRatio', 'reference'])
  if (node.kind !== 'image') invalid('Only image nodes may contain a generation intent.')
  const keyId = raw.keyId
  if (typeof keyId !== 'number' || !Number.isSafeInteger(keyId) || keyId < 1) invalid('Invalid intent key ID.')
  const aspectRatio = raw.aspectRatio
  if (aspectRatio !== '1:1' && aspectRatio !== '3:2' && aspectRatio !== '2:3') invalid('Invalid intent aspect ratio.')
  const intent: CanvasSubmitIntent = {
    requestId: string(raw.requestId, 256, 'Intent request ID'),
    keyId,
    prompt: string(raw.prompt, 32_000, 'Intent prompt'),
    model: string(raw.model, 256, 'Intent model'),
    aspectRatio,
  }
  if (!intent.requestId.trim()) invalid('Missing intent request ID.')
  if (raw.reference !== undefined) {
    checkBlob(raw.reference)
    return { ...intent, reference: raw.reference }
  }
  return intent
}

function checkGraph(nodes: CanvasNode[]): void {
  const byId = new Map(nodes.map(node => [node.id, node]))
  if (byId.size !== nodes.length) invalid('Duplicate node ID.')
  const checked = new Set<string>()
  for (const start of nodes) {
    let current = start
    const path = new Set<string>()
    while (current.parentId && !checked.has(current.id)) {
      if (path.has(current.id)) invalid('Cycle in the canvas graph.')
      path.add(current.id)
      const parent = byId.get(current.parentId)
      if (!parent || parent.kind === current.kind) invalid('Invalid parent node.')
      current = parent
    }
    for (const key of path) checked.add(key)
  }
}

function checkBlob(blob: unknown): asserts blob is Blob {
  if (!(blob instanceof Blob) || !IMAGE_TYPES.includes(blob.type) || !blob.size) invalidImage()
  if (blob.size > MAX_IMAGE_BYTES) limit('Each image must be 10 MiB or smaller.')
}

function checkImageBudget(total: number): void {
  if (total > MAX_TOTAL_IMAGE_BYTES) limit('Board images must total 32 MiB or less.')
}

const verifiedImages = new WeakSet<Blob>()

export async function validateImage(blob: Blob): Promise<void> {
  checkBlob(blob)
  if (verifiedImages.has(blob)) return
  const bytes = new Uint8Array(await blob.slice(0, 16).arrayBuffer())
  const starts = (offset: number, ...signature: number[]) => signature.every((byte, i) => bytes[offset + i] === byte)
  const matches = blob.type === 'image/png' ? starts(0, 137, 80, 78, 71, 13, 10, 26, 10)
    : blob.type === 'image/jpeg' ? starts(0, 255, 216, 255)
    : starts(0, 82, 73, 70, 70) && starts(8, 87, 69, 66, 80)
  if (!matches) invalidImage()

  const url = URL.createObjectURL(blob)
  try {
    const image = new Image()
    image.decoding = 'async'
    await new Promise<void>((resolve, reject) => {
      image.onload = () => resolve()
      image.onerror = () => reject(new CanvasArchiveError('invalid-image', 'The image header could not be read.'))
      image.src = url
    })
    if (!image.naturalWidth || !image.naturalHeight) invalidImage()
    if (image.naturalWidth * image.naturalHeight > MAX_IMAGE_PIXELS) limit('Image exceeds 33,554,432 pixels.')
    try {
      await image.decode()
    } catch {
      invalidImage()
    }
  } finally {
    URL.revokeObjectURL(url)
  }
  verifiedImages.add(blob)
}

// Snapshot synchronously so a drag, account switch, or edit during validation cannot alter this write.
function snapshot(board: CanvasBoard, portable: boolean): CanvasBoard {
  const nodes = nodeList(board.nodes).map(value => {
    const raw = record(value)
    const node = nodeFields(raw)
    if (portable && (raw.intent !== undefined || raw.job !== undefined || (node.kind === 'image' && !raw.image))) {
      throw new CanvasArchiveError('unfinished-node', 'Finish or remove the unfinished node before exporting.', node.id)
    }
    if (raw.image !== undefined) {
      if (node.kind !== 'image') invalid('Only image nodes may contain an image.')
      checkBlob(raw.image)
      node.image = raw.image
    }
    if (!portable && raw.intent !== undefined) {
      node.intent = intentFields(raw.intent, node)
    }
    if (!portable && raw.job !== undefined) {
      const job = record(raw.job)
      if (node.kind !== 'image' || typeof job.keyId !== 'number' || !Number.isSafeInteger(job.keyId) || job.keyId < 1) invalid('Invalid local job.')
      const jobId = string(job.id, 256, 'Job ID')
      if (!jobId.trim()) invalid('Missing local job ID.')
      node.job = { id: jobId, keyId: job.keyId }
    }
    if (!portable && raw.error !== undefined) node.error = string(raw.error, 32_000, 'Node error')
    return node
  })
  checkImageBudget(boardImageBytes(nodes))
  checkGraph(nodes)
  return { id: id(board.id), version: board.version, name: boardName(board.name), viewport: viewport(board.viewport), nodes }
}

function encodeImage(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const url = reader.result as string
      resolve(url.slice(url.indexOf(',') + 1))
    }
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(blob)
  })
}

export async function exportBoard(board: CanvasBoard): Promise<Blob> {
  const copy = snapshot(board, true)
  const nodes = []
  for (const { image, ...node } of copy.nodes) {
    if (!image) nodes.push(node)
    else {
      await validateImage(image)
      nodes.push({ ...node, image: { type: image.type, data: await encodeImage(image) } })
    }
  }
  const archive = { format: FORMAT, version: 1, name: copy.name, viewport: copy.viewport, nodes }
  const file = new Blob([JSON.stringify(archive)], { type: 'application/json' })
  if (file.size > MAX_ARCHIVE_BYTES) limit('Canvas archive exceeds 48 MiB.')
  return file
}

function decodeImage(value: unknown, used: number): Blob {
  const raw = record(value)
  onlyFields(raw, ['type', 'data'])
  const { type, data } = raw
  if (typeof type !== 'string' || !IMAGE_TYPES.includes(type) || typeof data !== 'string' || !data.length) invalidImage()
  if (data.length > Math.ceil(MAX_IMAGE_BYTES / 3) * 4) limit('Each image must be 10 MiB or smaller.')
  if (data.length % 4 || /[^a-zA-Z0-9+/=]/.test(data)) invalidImage()
  const padding = data.endsWith('==') ? 2 : data.endsWith('=') ? 1 : 0
  const size = data.length / 4 * 3 - padding
  if (size > MAX_IMAGE_BYTES) limit('Each image must be 10 MiB or smaller.')
  checkImageBudget(used + size)
  let binary: string
  try {
    binary = atob(data)
  } catch {
    invalidImage()
  }
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < bytes.length; i++) bytes[i] = binary.charCodeAt(i)
  return new Blob([bytes], { type })
}

export async function importBoard(file: Blob): Promise<CanvasBoard> {
  if (!(file instanceof Blob)) invalid('Expected a canvas archive.')
  if (file.size > MAX_ARCHIVE_BYTES) limit('Canvas archive exceeds 48 MiB.')
  const buffer = await file.arrayBuffer()
  let parsed: unknown
  try {
    parsed = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(buffer))
  } catch {
    invalid('Canvas archive must be valid UTF-8 JSON.')
  }
  const raw = record(parsed)
  onlyFields(raw, ['format', 'version', 'name', 'viewport', 'nodes'])
  if (raw.format !== FORMAT || raw.version !== 1) invalid('Unsupported canvas archive format or version.')
  const name = boardName(raw.name)
  onlyFields(record(raw.viewport), ['x', 'y', 'zoom'])
  const view = viewport(raw.viewport)
  let total = 0
  const nodes = nodeList(raw.nodes).map(value => {
    const entry = record(value)
    onlyFields(entry, ['id', 'kind', 'x', 'y', 'text', 'parentId', 'model', 'image'])
    const node = nodeFields(entry)
    if (node.kind === 'image') {
      node.image = decodeImage(entry.image, total)
      total += node.image.size
    } else if (entry.image !== undefined) invalid('Only image nodes may contain an image.')
    return node
  })
  checkGraph(nodes)
  for (const node of nodes) if (node.image) await validateImage(node.image)
  return { id: newId(), name, nodes, viewport: view }
}

const BOARDS = 'boards'
const SUMMARIES = 'summaries'
const ASSETS = 'assets'
type StoredIntent = Omit<CanvasSubmitIntent, 'reference'> & { referenceAssetId?: string }
type StoredNode = Omit<CanvasNode, 'image' | 'intent'> & { assetId?: string; intent?: StoredIntent }
type StoredBoard = Omit<CanvasBoard, 'nodes'> & { nodes: StoredNode[] }
interface Asset { id: string; blob: Blob }
// Asset IDs are local to a single board's compound key range, including those read from legacy rows.
const assetIds = new WeakMap<Blob, { userId: number; boardId: string; assetId: string }>()
let connection: Promise<IDBDatabase> | undefined

function database(): Promise<IDBDatabase> {
  if (connection) return connection
  const pending = new Promise<IDBDatabase>((resolve, reject) => {
    if (!globalThis.indexedDB) throw new Error('IndexedDB is unavailable for canvas storage.')
    const request = indexedDB.open('sub2api-image-canvas', 1)
    let blocked = false
    request.onupgradeneeded = () => {
      request.result.createObjectStore(BOARDS)
      request.result.createObjectStore(SUMMARIES)
      request.result.createObjectStore(ASSETS)
    }
    request.onblocked = () => {
      blocked = true
      reject(new Error('Canvas storage is blocked by another tab.'))
    }
    request.onerror = () => reject(request.error)
    request.onsuccess = () => {
      const db = request.result
      if (blocked) { db.close(); return }
      db.onversionchange = () => { db.close(); connection = undefined }
      db.onclose = () => { connection = undefined }
      resolve(db)
    }
  })
  connection = pending
  void pending.catch(() => { if (connection === pending) connection = undefined })
  return pending
}

function user(userId: number): void {
  if (!Number.isSafeInteger(userId) || userId < 1) throw new TypeError('Canvas storage requires a signed-in user ID.')
}

function readRequest<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

interface Summary {
  id: string
  name: string
  savedAt: number
}

export async function listBoards(userId: number): Promise<{ id: string; name: string }[]> {
  user(userId)
  const db = await database()
  const range = IDBKeyRange.bound([userId], [userId, []])
  const rows: Summary[] = await readRequest(db.transaction(SUMMARIES).objectStore(SUMMARIES).getAll(range))
  // Rows already arrive in ascending primary-key order. Stable sort preserves descending ID for ties.
  rows.reverse().sort((a, b) => b.savedAt - a.savedAt)
  return rows.map(({ id, name }) => ({ id, name }))
}

function restoreBoard(userId: number, stored: StoredBoard, assets: Asset[]): CanvasBoard {
  const images = new Map(assets.map(asset => [asset.id, asset.blob]))
  function resolveAsset(assetId: string): Blob {
    const blob = images.get(assetId)
    if (!blob) invalid('Stored canvas image is missing.')
    assetIds.set(blob, { userId, boardId: stored.id, assetId })
    return blob
  }
  const nodes = stored.nodes.map(({ assetId, intent, ...node }) => {
    const restored: CanvasNode = { ...node }
    if (assetId !== undefined) restored.image = resolveAsset(assetId)
    if (intent) {
      const { referenceAssetId, ...fields } = intent
      restored.intent = referenceAssetId === undefined ? fields : { ...fields, reference: resolveAsset(referenceAssetId) }
    }
    return restored
  })
  return snapshot({ ...stored, version: stored.version ?? 0, nodes }, false)
}

export async function loadBoard(userId: number, boardId: string): Promise<CanvasBoard | undefined> {
  user(userId)
  const key = [userId, id(boardId)]
  const db = await database()
  const tx = db.transaction([BOARDS, ASSETS])
  const range = IDBKeyRange.bound(key, [...key, []])
  const [board, assets]: [StoredBoard | undefined, Asset[]] = await Promise.all([
    readRequest(tx.objectStore(BOARDS).get(key)),
    readRequest(tx.objectStore(ASSETS).getAll(range)),
  ])
  return board ? restoreBoard(userId, board, assets) : undefined
}

// Validation is asynchronous. Serialize same-board mutations in invocation order within this tab.
const writes = new Map<string, Promise<unknown>>()

function writeBoard<T>(userId: number, boardId: string, work: () => Promise<T>): Promise<T> {
  const key = `${userId}:${boardId}`
  const previous = writes.get(key) ?? Promise.resolve()
  const next = previous.then(work, work)
  writes.set(key, next)
  return next.finally(() => { if (writes.get(key) === next) writes.delete(key) })
}

async function updateStores(userId: number, boardId: string, expectedVersion: number | undefined, board?: CanvasBoard): Promise<number> {
  const db = await database()
  const key = [userId, boardId]
  const range = IDBKeyRange.bound(key, [...key, []])
  const images = new Map<string, Blob>()
  function storeAsset(blob: Blob): string {
    const cached = assetIds.get(blob)
    const assetId = cached?.userId === userId && cached.boardId === boardId ? cached.assetId : newId()
    assetIds.set(blob, { userId, boardId, assetId })
    images.set(assetId, blob)
    return assetId
  }
  const nodes = board?.nodes.map(({ image, intent, ...node }): StoredNode => {
    const stored: StoredNode = { ...node }
    if (image) stored.assetId = storeAsset(image)
    if (intent) {
      const { reference, ...fields } = intent
      stored.intent = reference ? { ...fields, referenceAssetId: storeAsset(reference) } : fields
    }
    return stored
  })
  const version = (expectedVersion ?? 0) + 1
  return new Promise((resolve, reject) => {
    const tx = db.transaction([BOARDS, SUMMARIES, ASSETS], 'readwrite')
    tx.oncomplete = () => resolve(version)
    tx.onabort = () => reject(tx.error ?? new Error('Canvas storage transaction aborted.'))
    const assets = tx.objectStore(ASSETS)
    const existing = tx.objectStore(BOARDS).get(key)
    const current = assets.getAllKeys(range)
    current.onsuccess = () => {
      try {
        const committed: StoredBoard | undefined = existing.result
        const currentVersion = committed === undefined ? undefined : committed.version ?? 0
        if (expectedVersion !== currentVersion || (!board && !committed)) {
          // No writes have been scheduled. Read the winning assets under the same lock.
          if (!committed) { reject(new CanvasConflictError()); return }
          const rows = assets.getAll(range)
          rows.onsuccess = () => {
            try { reject(new CanvasConflictError(restoreBoard(userId, committed, rows.result))) }
            catch (error) { reject(error) }
          }
          return
        }
        // Reconcile against committed keys inside the transaction (another tab may have saved).
        for (const assetKey of current.result) {
          const assetId = (assetKey as [number, string, string])[2]
          if (!images.delete(assetId)) assets.delete(assetKey)
        }
        for (const [assetId, blob] of images) assets.put({ id: assetId, blob }, [...key, assetId])
        if (board) {
          tx.objectStore(BOARDS).put({ ...board, version, nodes }, key)
          tx.objectStore(SUMMARIES).put({ id: boardId, name: board.name, savedAt: Date.now() }, key)
        } else {
          tx.objectStore(BOARDS).delete(key)
          tx.objectStore(SUMMARIES).delete(key)
        }
      } catch (error) {
        tx.abort()
        reject(error)
      }
    }
  })
}

export async function saveBoard(userId: number, board: CanvasBoard, expectedVersion: number | undefined): Promise<number> {
  user(userId)
  const copy = snapshot(board, false)
  return writeBoard(userId, copy.id, async () => {
    for (const node of copy.nodes) {
      if (node.image) await validateImage(node.image)
      if (node.intent?.reference) await validateImage(node.intent.reference)
    }
    return updateStores(userId, copy.id, expectedVersion, copy)
  })
}

export async function deleteBoard(userId: number, boardId: string, expectedVersion: number): Promise<void> {
  user(userId)
  id(boardId)
  await writeBoard(userId, boardId, () => updateStores(userId, boardId, expectedVersion))
}
