import { flushPromises } from '@vue/test-utils'
import { effectScope, shallowRef, type EffectScope } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '@/types'
import type { CanvasBoard, CanvasJob, CanvasNode } from '../document'
import * as gateway from '../gateway'
import { useCanvasGeneration } from '../useCanvasGeneration'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../gateway', () => ({
  listCanvasKeys: vi.fn(), listCanvasModels: vi.fn(), lookupCanvasJob: vi.fn(),
  submitCanvasIntent: vi.fn(), submitCanvasImage: vi.fn(), checkCanvasJob: vi.fn(),
  isCanvasKey: (key: ApiKey) => key.group?.allow_batch_image_generation === true,
  isGeminiKey: (key: ApiKey) => key.group?.platform === 'gemini',
  isTerminalCanvasSubmitError: vi.fn(),
}))

const owner = { id: 7, key: 'original-key', status: 'active', group: { platform: 'gemini', allow_batch_image_generation: true } } as ApiKey
const alternate = { ...owner, id: 8, key: 'other-key' }
const movedOwner = { ...owner, group: undefined }
const job: CanvasJob = { id: 'batch-17', keyId: owner.id }
const reference = new Blob(['lighthouse at dawn'], { type: 'image/png' })
const prompt: CanvasNode = { id: 'prompt', kind: 'prompt', text: 'A lighthouse in fog', parentId: 'source', x: 300, y: 0 }
const initialBoard: CanvasBoard = {
  id: 'board', name: 'Coast', viewport: { x: 0, y: 0, zoom: 1 },
  nodes: [{ id: 'source', kind: 'image', image: reference, text: '', x: 0, y: 0 }, prompt],
}
const scopes: EffectScope[] = []
const lookup = vi.mocked(gateway.lookupCanvasJob)
const post = vi.mocked(gateway.submitCanvasIntent)

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((ok, fail) => { resolve = ok; reject = fail })
  return { promise, resolve, reject }
}

// Copy the mutable records at each durability boundary. Blob contents are immutable.
function snapshot(board: CanvasBoard): CanvasBoard {
  return { ...board, viewport: { ...board.viewport }, nodes: board.nodes.map(node => ({
    ...node, job: node.job && { ...node.job }, intent: node.intent && { ...node.intent },
  })) }
}

function result(board: CanvasBoard) {
  const results = board.nodes.filter(node => node.parentId === prompt.id)
  expect(results).toHaveLength(1)
  return results[0]
}

function openBoard(saved = initialBoard) {
  let committed = snapshot(saved)
  const board = shallowRef(snapshot(committed))
  const flush = vi.fn(async () => { committed = snapshot(board.value) })
  const scope = effectScope()
  scopes.push(scope)
  const generation = scope.run(() => useCanvasGeneration({
    board, flush, reveal: vi.fn(), notify: vi.fn(),
    startResult(node) { board.value = { ...board.value, nodes: [...board.value.nodes, node] } },
    patchResult(id, patch) {
      board.value = { ...board.value, nodes: board.value.nodes.map(node => node.id === id ? { ...node, ...patch } : node) }
    },
  }))!
  async function load() {
    await generation.loadKeys()
    // The model and reconciliation watchers launch promises outside loadKeys().
    await flushPromises()
  }
  return { generation, board, flush, load, saved: () => committed, close: () => scope.stop() }
}

async function loseAcknowledgement() {
  const first = openBoard()
  await first.load()
  first.generation.aspectRatio.value = '2:3'
  post.mockRejectedValueOnce(new TypeError('Connection lost after sending POST'))
  lookup.mockRejectedValue(new TypeError('Offline'))
  await first.generation.generate(prompt)
  await flushPromises()
  expect(post).toHaveBeenCalledTimes(1)
  const intent = result(first.saved()).intent!
  expect(intent).toEqual({
    requestId: expect.stringMatching(/\S/), keyId: owner.id, prompt: prompt.text,
    model: 'coastal-image', aspectRatio: '2:3', reference,
  })
  expect(post).toHaveBeenCalledWith(owner, intent, expect.any(AbortSignal))
  first.close()
  lookup.mockClear()
  return { saved: first.saved(), intent }
}

beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
  vi.mocked(gateway.listCanvasKeys).mockResolvedValue([owner, alternate])
  vi.mocked(gateway.listCanvasModels).mockResolvedValue(['coastal-image'])
  vi.mocked(gateway.checkCanvasJob).mockResolvedValue({ status: 'pending' })
})

afterEach(() => {
  for (const scope of scopes.splice(0)) scope.stop()
  vi.useRealTimers()
  expect(gateway.submitCanvasImage).not.toHaveBeenCalled()
})

describe('useCanvasGeneration batch submission', () => {
  it('waits for durable intent before sending and makes no request if the save fails', async () => {
    const editor = openBoard()
    await editor.load()
    const save = deferred<void>()
    editor.flush.mockReturnValueOnce(save.promise)
    const sending = editor.generation.generate(prompt)
    await flushPromises()
    expect(editor.flush).toHaveBeenCalledTimes(1)
    expect(result(editor.board.value).intent?.keyId).toBe(owner.id)
    expect(post).not.toHaveBeenCalled()
    save.reject(new Error('Storage full'))
    await sending
    expect(post).not.toHaveBeenCalled()
    expect(lookup).not.toHaveBeenCalled()
    expect(editor.saved().nodes).toHaveLength(initialBoard.nodes.length)

    // An explicit retry may look up this unsaved request, but it still needs a successful save to POST.
    lookup.mockResolvedValueOnce(undefined)
    editor.flush.mockRejectedValueOnce(new Error('Still full'))
    const node = result(editor.board.value)
    await editor.generation.retry(node)
    expect(lookup).toHaveBeenCalledWith(owner, node.intent!.requestId, expect.any(AbortSignal))
    expect(editor.flush).toHaveBeenCalledTimes(2)
    expect(post).not.toHaveBeenCalled()
    expect(result(editor.board.value).intent).toEqual(node.intent)
  })

  it('recovers the acknowledged job on reload with its original active key after permissions change', async () => {
    const { saved, intent } = await loseAcknowledgement()
    vi.mocked(gateway.listCanvasKeys).mockResolvedValue([movedOwner, alternate])
    lookup.mockResolvedValue(job)
    const reopened = openBoard(saved)
    await reopened.load()
    expect(gateway.listCanvasKeys).toHaveBeenLastCalledWith(true)
    expect(reopened.generation.recoveryKeys.value).toEqual([movedOwner, alternate])
    expect(reopened.generation.keys.value).toEqual([alternate])
    expect(reopened.generation.key.value).toEqual(alternate)
    expect(lookup).toHaveBeenCalledTimes(1)
    expect(lookup).toHaveBeenCalledWith(movedOwner, intent.requestId, expect.any(AbortSignal))
    expect(result(reopened.saved()).job).toEqual(job)
    expect(result(reopened.saved()).intent).toBeUndefined()
    reopened.close()

    const withJob = openBoard(reopened.saved())
    await withJob.load()
    expect(gateway.checkCanvasJob).toHaveBeenLastCalledWith(movedOwner, job, expect.any(AbortSignal))
    expect(lookup).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledTimes(1)
  })

  it('only replays after an explicit retry receives fresh structured absence, using every original input', async () => {
    const { saved, intent } = await loseAcknowledgement()
    // undefined is the gateway result for a structured job-not-found response.
    lookup.mockResolvedValue(undefined)
    vi.mocked(gateway.listCanvasKeys).mockResolvedValue([alternate, owner])
    const reopened = openBoard(saved)
    await reopened.load()
    const node = result(reopened.board.value)
    expect(lookup).toHaveBeenCalledTimes(1)
    expect(lookup).toHaveBeenCalledWith(owner, intent.requestId, expect.any(AbortSignal))
    expect(reopened.generation.missing.value[node.id]).toBe(true)
    expect(post).toHaveBeenCalledTimes(1)

    reopened.board.value = { ...reopened.board.value, nodes: reopened.board.value.nodes.map(entry => {
      if (entry.id === prompt.id) return { ...entry, text: 'A different prompt' }
      if (entry.id === prompt.parentId) return { ...entry, image: new Blob(['different reference']) }
      return entry
    }) }
    reopened.generation.model.value = 'different-model'
    reopened.generation.aspectRatio.value = '3:2'
    expect(reopened.generation.keyId.value).toBe(alternate.id)
    const freshLookup = deferred<CanvasJob | undefined>()
    lookup.mockReturnValueOnce(freshLookup.promise)
    post.mockResolvedValueOnce(job)
    const replay = reopened.generation.retry(node)
    await flushPromises()
    expect(lookup).toHaveBeenCalledTimes(2)
    expect(lookup).toHaveBeenLastCalledWith(owner, intent.requestId, expect.any(AbortSignal))
    expect(post).toHaveBeenCalledTimes(1)
    freshLookup.resolve(undefined)
    await replay
    expect(post).toHaveBeenCalledTimes(2)
    expect(post).toHaveBeenLastCalledWith(owner, intent, expect.any(AbortSignal))
    expect(post.mock.calls[1][1].reference).toBe(reference)
    expect(result(reopened.saved()).job).toEqual(job)
    expect(result(reopened.saved()).intent).toBeUndefined()
  })

  it('does not replay on an ambiguous 404 or when retry discovers an existing job', async () => {
    const { saved, intent } = await loseAcknowledgement()
    lookup.mockResolvedValueOnce(undefined)
    const reopened = openBoard(saved)
    await reopened.load()
    const node = result(reopened.board.value)
    expect(reopened.generation.missing.value[node.id]).toBe(true)
    lookup.mockRejectedValueOnce(Object.assign(new Error('Proxy not found'), { status: 404 }))
    await reopened.generation.retry(node)
    expect(result(reopened.board.value).intent).toEqual(intent)
    expect(post).toHaveBeenCalledTimes(1)
    lookup.mockResolvedValueOnce(job)
    await reopened.generation.retry(node)
    expect(lookup.mock.calls.map(([key, id]) => [key.id, id])).toEqual(Array(3).fill([owner.id, intent.requestId]))
    expect(post).toHaveBeenCalledTimes(1)
    expect(result(reopened.saved()).job).toEqual(job)
    expect(result(reopened.saved()).intent).toBeUndefined()
  })

  it('persists a terminal POST rejection and clears the intent before reload', async () => {
    const rejection = { status: 400, code: 'BATCH_IMAGE_INVALID_MODEL' }
    vi.mocked(gateway.isTerminalCanvasSubmitError).mockImplementation(error => error === rejection)
    post.mockRejectedValueOnce(rejection)
    const editor = openBoard()
    await editor.load()
    await editor.generation.generate(prompt)
    expect(result(editor.saved()).error).toBe('canvas.submissionRejected')
    expect(result(editor.saved()).intent).toBeUndefined()
    expect(result(editor.saved()).job).toBeUndefined()
    editor.close()
    await openBoard(editor.saved()).load()
    expect(lookup).not.toHaveBeenCalled()
    expect(post).toHaveBeenCalledTimes(1)
  })

  it('persists removal after a structured 410 during automatic lookup', async () => {
    const { saved, intent } = await loseAcknowledgement()
    lookup.mockRejectedValueOnce({ status: 410, code: 'BATCH_IMAGE_SUBMISSION_REMOVED' })
    const reopened = openBoard(saved)
    await reopened.load()
    expect(lookup).toHaveBeenCalledWith(owner, intent.requestId, expect.any(AbortSignal))
    expect(result(reopened.saved()).error).toBe('canvas.submissionRemoved')
    expect(result(reopened.saved()).intent).toBeUndefined()
    expect(result(reopened.saved()).job).toBeUndefined()
    expect(post).toHaveBeenCalledTimes(1)
  })
})
