import { flushPromises } from '@vue/test-utils'
import { effectScope, type EffectScope } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CanvasBoard, CanvasNode } from '../document'
import { useCanvasWorkspace } from '../useCanvasWorkspace'

const storage = vi.hoisted(() => {
  const boards = new Map<string, CanvasBoard>()

  function key(userId: number, boardId: string) {
    return `${userId}:${boardId}`
  }

  function copy(value: CanvasBoard): CanvasBoard {
    return {
      ...value,
      nodes: value.nodes.map(node => ({
        ...node,
        intent: node.intent && { ...node.intent, reference: node.intent.reference },
      })),
    }
  }

  class CanvasConflictError extends Error {
    constructor(readonly current?: CanvasBoard) {
      super('Canvas changed in another workspace.')
      this.name = 'CanvasConflictError'
    }
  }

  const listBoards = vi.fn(async (userId: number) => [...boards]
    .filter(([storedKey]) => storedKey.startsWith(`${userId}:`))
    .map(([, board]) => ({ id: board.id, name: board.name })))

  const loadBoard = vi.fn(async (userId: number, boardId: string) => {
    const board = boards.get(key(userId, boardId))
    return board && copy(board)
  })

  const saveBoard = vi.fn(async (userId: number, board: CanvasBoard, expectedVersion: number | undefined) => {
    const storedKey = key(userId, board.id)
    const current = boards.get(storedKey)
    if (expectedVersion !== current?.version) throw new CanvasConflictError(current && copy(current))
    const version = (expectedVersion ?? 0) + 1
    boards.set(storedKey, { ...copy(board), version })
    return version
  })

  const deleteBoard = vi.fn(async (userId: number, boardId: string, expectedVersion: number) => {
    const storedKey = key(userId, boardId)
    const current = boards.get(storedKey)
    if (expectedVersion !== current?.version) throw new CanvasConflictError(current && copy(current))
    boards.delete(storedKey)
  })

  return {
    CanvasConflictError,
    listBoards,
    loadBoard,
    saveBoard,
    deleteBoard,
    read: (userId: number, boardId: string) => {
      const board = boards.get(key(userId, boardId))
      return board && copy(board)
    },
    reset: () => boards.clear(),
  }
})

vi.mock('../storage', () => storage)

const scopes: EffectScope[] = []

function makeEditor(userId = 12, name = 'Draft', conflictName?: (currentName: string) => string) {
  const scope = effectScope()
  scopes.push(scope)
  return scope.run(() => useCanvasWorkspace(userId, () => name, conflictName))!
}

async function openPair(conflictName?: (currentName: string) => string) {
  const owner = makeEditor()
  await owner.initialize()
  const peer = makeEditor(12, 'Draft', conflictName)
  await peer.initialize()
  return { owner, peer }
}

afterEach(async () => {
  for (const scope of scopes) scope.stop()
  scopes.length = 0
  // Disposing an editor flushes outstanding writes. Drain them before resetting the shared store.
  await flushPromises()
  storage.reset()
  localStorage.clear()
  sessionStorage.clear()
  vi.clearAllMocks()
})

describe('canvas editing around a paid request', () => {
  it('keeps the finished image when unrelated edits are undone and redone', async () => {
    const workspace = makeEditor()
    await workspace.initialize()
    const prompt: CanvasNode = { id: 'prompt', text: 'A bridge', kind: 'prompt', x: 0, y: 0 }
    const output: CanvasNode = { id: 'result', text: prompt.text, kind: 'image', parentId: prompt.id, x: 286, y: 0 }
    workspace.setNodes([prompt])
    workspace.startResult(output)
    workspace.patchResult(output.id, { job: { id: 'job', keyId: 19 } })
    workspace.setNodes([{ ...prompt, text: 'A new idea' }, workspace.board.value!.nodes[1]])
    const image = new Blob(['result'], { type: 'image/png' })
    workspace.patchResult(output.id, { image, job: undefined })
    workspace.undo()
    expect(workspace.board.value?.nodes).toEqual([prompt, { ...output, image, job: undefined }])
    expect(workspace.canUndo.value).toBe(false)
    workspace.redo()
    expect(workspace.board.value?.nodes[0].text).toBe('A new idea')
    expect(workspace.board.value?.nodes[1].image).toBe(image)
    expect(workspace.board.value?.nodes[1].job).toBeUndefined()
  })

  it('forks a stale update with its image and intent, then saves later edits on the fork', async () => {
    const { owner, peer } = await openPair(currentName => `${currentName} (fork)`)
    const originalId = peer.board.value!.id
    owner.rename('Remote edit')
    await owner.flush()

    const image = new Blob(['finished'], { type: 'image/png' })
    const reference = new Blob(['reference'], { type: 'image/png' })
    const pending: CanvasNode = {
      id: 'pending',
      kind: 'image',
      text: 'A mountain',
      x: 240,
      y: 80,
      image,
      job: { id: 'job-1', keyId: 19 },
      intent: {
        requestId: 'request-1',
        keyId: 19,
        prompt: 'A mountain',
        model: 'image-model',
        aspectRatio: '1:1',
        reference,
      },
    }
    peer.startResult(pending)
    await peer.flush()

    const fork = peer.board.value!
    expect(fork.id).not.toBe(originalId)
    expect(fork.name).toBe('Draft (fork)')
    expect(fork.version).toBe(1)
    expect(fork.nodes).toHaveLength(1)
    expect(fork.nodes[0]).toMatchObject({ id: 'pending', image, job: pending.job, intent: pending.intent })
    expect(storage.read(12, originalId)?.name).toBe('Remote edit')
    expect(storage.read(12, fork.id)?.nodes[0]).toMatchObject({ id: 'pending', image, intent: pending.intent })

    peer.rename('Recovered mountain')
    await peer.flush()
    const calls = storage.saveBoard.mock.calls as Array<[number, CanvasBoard, number | undefined]>
    const lastCall = calls[calls.length - 1]
    expect(lastCall[1].id).toBe(fork.id)
    expect(lastCall[2]).toBe(1)
    expect(peer.board.value?.version).toBe(2)
    expect(storage.read(12, fork.id)?.name).toBe('Recovered mountain')
  })

  it('keeps a fork as the active workspace when a stale delete conflicts after a remote edit', async () => {
    const { owner, peer } = await openPair(currentName => `${currentName} (delete fork)`)
    const originalId = peer.board.value!.id
    owner.rename('Remote edit')
    await owner.flush()

    await expect(peer.remove()).resolves.toBeUndefined()

    const fork = peer.board.value!
    expect(fork.id).not.toBe(originalId)
    expect(fork.name).toBe('Draft (delete fork)')
    expect(peer.recoveredBoard.value).toBe(fork.name)
    expect(storage.read(12, originalId)?.name).toBe('Remote edit')
    expect(storage.read(12, fork.id)?.version).toBe(1)
    expect(peer.boards.value).toContainEqual({ id: fork.id, name: fork.name })
  })

  it('forks a pending write after the remote workspace deletes the board', async () => {
    const { owner, peer } = await openPair(currentName => `${currentName} (write fork)`)
    const originalId = peer.board.value!.id
    await owner.remove()
    expect(storage.read(12, originalId)).toBeUndefined()

    peer.rename('Local edit')
    await peer.flush()

    const fork = peer.board.value!
    expect(fork.id).not.toBe(originalId)
    expect(fork.name).toBe('Local edit (write fork)')
    expect(fork.version).toBe(1)
    expect(storage.read(12, fork.id)?.name).toBe('Local edit (write fork)')
    expect(storage.read(12, fork.id)?.version).toBe(1)
  })
})
