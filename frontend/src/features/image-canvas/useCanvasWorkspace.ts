import { computed, onScopeDispose, ref, shallowRef } from 'vue'
import { createBoard, newId, type CanvasBoard, type CanvasNode, type Viewport } from './document'
import { CanvasConflictError, deleteBoard, importBoard, listBoards, loadBoard, saveBoard } from './storage'

type SaveState = 'saved' | 'pending' | 'saving' | 'failed'
const historyLimit = 40

export function useCanvasWorkspace(userId: number, defaultName: () => string, conflictName = (name: string) => `${name} (recovered)`) {
  const board = shallowRef<CanvasBoard | null>(null)
  const boards = shallowRef<{ id: string; name: string }[]>([])
  const busy = ref(false)
  const saveState = ref<SaveState>('saved')
  const loadFailed = ref(false)
  const recoveredBoard = ref('')
  const previous = shallowRef<CanvasNode[][]>([])
  const next = shallowRef<CanvasNode[][]>([])
  const activeKey = `sub2api:canvas:active:${userId}`
  let editGroup: string | undefined
  let revision = 0
  let savedRevision = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let writeQueue: Promise<void> = Promise.resolve()
  let disposed = false
  let needsFork = false

  function rememberActive(id: string) {
    try { sessionStorage.setItem(activeKey, id) } catch { /* Fall back to the last used board. */ }
    try { localStorage.setItem(activeKey, id) } catch { /* IndexedDB is the source of truth. */ }
  }

  function resetHistory() {
    previous.value = []
    next.value = []
    editGroup = undefined
  }

  function activate(value: CanvasBoard) {
    clearTimeout(timer)
    board.value = value
    revision = savedRevision = 0
    saveState.value = 'saved'
    resetHistory()
    rememberActive(value.id)
  }

  async function forkCurrent() {
    const current = board.value!
    const writing = revision
    const copy = { ...current, id: newId(), name: conflictName(current.name.slice(0, 100)).slice(0, 120), version: undefined }
    const version = await saveBoard(userId, copy, undefined)
    // Keep edits made while the fork and its assets were being saved.
    const latest = board.value!
    const name = latest.name === current.name ? copy.name : conflictName(latest.name.slice(0, 100)).slice(0, 120)
    board.value = { ...latest, id: copy.id, name, version }
    savedRevision = writing
    saveState.value = revision === writing ? 'saved' : 'pending'
    recoveredBoard.value = copy.name
    rememberActive(copy.id)
    boards.value = [{ id: copy.id, name: copy.name }, ...boards.value]
    try { boards.value = await listBoards(userId) } catch { /* The copy is already committed. */ }
  }

  function flush(): Promise<void> {
    clearTimeout(timer)
    // After a conflict, retries may only write to a new board, even if the fork hit quota.
    const pending = writeQueue.then(async () => {
      const snapshot = board.value
      if (!snapshot || savedRevision === revision) return
      const writing = revision
      saveState.value = 'saving'
      if (needsFork) {
        await forkCurrent()
        needsFork = false
        return
      }
      try {
        const version = await saveBoard(userId, snapshot, snapshot.version)
        board.value = { ...board.value!, version }
        savedRevision = writing
        saveState.value = revision === writing ? 'saved' : 'pending'
      } catch (error) {
        if (!(error instanceof CanvasConflictError)) throw error
        needsFork = true
        await forkCurrent()
        needsFork = false
      }
    }).catch(error => {
      saveState.value = 'failed'
      throw error
    })
    writeQueue = pending.catch(() => {})
    return pending
  }

  function update(value: CanvasBoard) {
    board.value = value
    revision++
    saveState.value = 'pending'
    clearTimeout(timer)
    timer = setTimeout(() => { void flush().catch(() => {}) }, 300)
  }

  function setNodes(nodes: CanvasNode[], group?: string) {
    if (!board.value || busy.value) return
    if (!group || group !== editGroup) previous.value = [...previous.value.slice(1 - historyLimit), board.value.nodes]
    next.value = []
    editGroup = group
    update({ ...board.value, nodes })
  }

  function undo() {
    const items = previous.value
    if (!board.value || busy.value || !items.length) return
    next.value = [...next.value, board.value.nodes]
    previous.value = items.slice(0, -1)
    editGroup = undefined
    update({ ...board.value, nodes: items[items.length - 1] })
  }

  function redo() {
    const items = next.value
    if (!board.value || busy.value || !items.length) return
    previous.value = [...previous.value, board.value.nodes]
    next.value = items.slice(0, -1)
    editGroup = undefined
    update({ ...board.value, nodes: items[items.length - 1] })
  }

  function startResult(node: CanvasNode) {
    if (!board.value) return
    // A submitted request is a billing boundary. Undo must never resubmit or lose its handle.
    resetHistory()
    update({ ...board.value, nodes: [...board.value.nodes, node] })
  }

  function patchResult(id: string, patch: Pick<CanvasNode, 'image' | 'job' | 'intent' | 'error'>) {
    if (!board.value) return
    const patchNodes = (nodes: CanvasNode[]) => nodes.map(n => n.id === id ? { ...n, ...patch } : n)
    previous.value = previous.value.map(patchNodes)
    next.value = next.value.map(patchNodes)
    update({ ...board.value, nodes: patchNodes(board.value.nodes) })
  }

  function setViewport(viewport: Viewport) {
    if (board.value && !busy.value) update({ ...board.value, viewport })
  }

  async function install(value: CanvasBoard) {
    const version = await saveBoard(userId, value, undefined)
    if (disposed) return
    boards.value = [{ id: value.id, name: value.name }, ...boards.value.filter(b => b.id !== value.id)]
    activate({ ...value, version })
  }

  async function initialize() {
    if (busy.value) return
    busy.value = true
    loadFailed.value = false
    try {
      await writeQueue
      const entries = await listBoards(userId)
      if (disposed) return
      boards.value = entries
      let last: string | null = null
      try { last = sessionStorage.getItem(activeKey) } catch { /* Use the shared last board. */ }
      if (!last) try { last = localStorage.getItem(activeKey) } catch { /* Use the last saved canvas. */ }
      const entry = entries.find(b => b.id === last) || entries[0]
      const existing = entry && await loadBoard(userId, entry.id)
      if (disposed) return
      if (existing) activate(existing)
      else await install(createBoard(defaultName()))
    } catch {
      loadFailed.value = true
    } finally {
      busy.value = false
    }
  }

  async function change(action: () => Promise<void>) {
    if (busy.value) return
    busy.value = true
    try {
      await flush()
      if (!disposed) await action()
    } finally { busy.value = false }
  }

  async function open(id: string) {
    if (id === board.value?.id) return
    await change(async () => {
      const value = await loadBoard(userId, id)
      if (!value) throw new Error('Canvas unavailable')
      if (!disposed) activate(value)
    })
  }

  const newBoard = () => change(() => install(createBoard(defaultName())))
  const importFile = (file: File) => change(async () => install(await importBoard(file)))

  function rename(name: string) {
    if (!board.value || !name.trim() || name.trim() === board.value.name || busy.value) return
    const renamed = { ...board.value, name: name.trim() }
    boards.value = boards.value.map(b => b.id === renamed.id ? { id: b.id, name: renamed.name } : b)
    update(renamed)
  }

  async function remove() {
    const old = board.value
    if (!old) return
    await change(async () => {
      // A conflicting pre-delete flush already preserved this tab as a separate board.
      if (board.value?.id !== old.id) return
      try {
        await deleteBoard(userId, old.id, board.value.version!)
      } catch (error) {
        if (!(error instanceof CanvasConflictError)) throw error
        // Keep the stale snapshot retryable if storage cannot hold a second copy yet.
        needsFork = true
        revision++
        await flush()
        return
      }
      if (disposed) return
      boards.value = await listBoards(userId)
      const remaining = boards.value[0] && await loadBoard(userId, boards.value[0].id)
      if (remaining) activate(remaining)
      else await install(createBoard(defaultName()))
    })
  }

  onScopeDispose(() => {
    disposed = true
    void flush().catch(() => {})
  })

  return {
    board, boards, busy, saveState, recoveredBoard, loadFailed, initialize, open, newBoard, importFile, rename, remove, flush,
    setNodes, setViewport, finishEdit: () => { editGroup = undefined }, startResult, patchResult,
    canUndo: computed(() => previous.value.length > 0), canRedo: computed(() => next.value.length > 0), undo, redo,
  }
}
