import { createBoard, type CanvasBoard, type CanvasNode } from '../document'
import { CanvasArchiveError, CanvasConflictError, deleteBoard, exportBoard, importBoard, listBoards, loadBoard, saveBoard } from '../storage'
import { saveBoard as saveFromOtherTab } from '../storage?second-tab'

function check(condition: unknown, message: string): asserts condition {
  if (!condition) throw new Error(message)
}

async function conflict(operation: Promise<unknown>, version?: number) {
  try { await operation } catch (error) {
    check(error instanceof CanvasConflictError, `Expected a storage conflict, received ${error}`)
    check(error.version === version, `Wrong committed version ${error.version}, expected ${version}`)
    return error.committed
  }
  throw new Error('Stale operation unexpectedly succeeded')
}

function result(id: string, image?: Blob): CanvasNode {
  return { id, kind: 'image', text: '', x: 0, y: 0, ...(image && { image }) }
}

const png = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='), c => c.charCodeAt(0))

function image(bytes = png.length) {
  return new Blob([png, new Uint8Array(bytes - png.length)], { type: 'image/png' })
}

function request<T>(req: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function run() {
  const user = 13
  const draft = createBoard('Local')
  const key = [user, draft.id]
  await listBoards(user) // Create the production schema before seeding a row written by the old client.
  const db = await request(indexedDB.open('sub2api-image-canvas', 1))
  const tx = db.transaction(['boards', 'summaries', 'assets'], 'readwrite')
  const parent = result('original')
  const prompt: CanvasNode = { id: 'prompt', kind: 'prompt', x: 1, y: 2, parentId: parent.id, text: '旧' }
  const pending: CanvasNode = { ...result('submitted'), parentId: prompt.id, job: { id: 'pending-job', keyId: 12 }, error: 'offline' }
  tx.objectStore('boards').put({ ...draft, nodes: [{ ...parent, assetId: 'legacy-img' }, prompt, pending] }, key)
  tx.objectStore('summaries').put({ id: draft.id, name: draft.name, savedAt: 7 }, key)
  tx.objectStore('assets').put({ id: 'legacy-img', blob: image() }, [...key, 'legacy-img'])
  await new Promise<void>((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onabort = () => reject(tx.error)
  })

  const legacy = await loadBoard(user, draft.id)
  check(legacy?.version === 0, 'Existing unversioned row must load as version zero')
  check(legacy.nodes[0].image?.size === png.length && legacy.nodes[2].job?.id === 'pending-job' && legacy.nodes[2].error === 'offline', 'Legacy assets and pending data changed during restore')
  await conflict(saveBoard(user, draft, undefined), 0)
  check(await saveBoard(user, legacy, legacy.version) === 1, 'First legacy update must commit version one')
  await conflict(deleteBoard(user, draft.id, 0), 1)
  const afterUpdate = await conflict(saveBoard(user, { ...legacy, nodes: [] }, 0), 1)
  check(afterUpdate?.nodes[0].image?.size === png.length && afterUpdate.nodes[2].job?.keyId === 12, 'Conflict must expose the winning board with its assets')

  // A separate module instance opens a second IDB connection with its own in-tab write queue.
  const race = await Promise.allSettled([
    saveBoard(user, { ...legacy, name: 'First tab' }, 1),
    saveFromOtherTab(user, { ...legacy, name: 'Second tab' }, 1),
  ])
  const winner = race.findIndex(entry => entry.status === 'fulfilled')
  check(winner !== -1 && race[1 - winner].status === 'rejected', 'Exactly one concurrent CAS should succeed')
  const persisted = await loadBoard(user, draft.id)
  check(persisted?.version === 2 && persisted.name === (winner === 0 ? 'First tab' : 'Second tab'), 'Winning board and version must match')
  const summary = (await listBoards(user)).find(entry => entry.id === draft.id)
  check(summary?.name === persisted.name, 'Summary and winning board diverged')
  await deleteBoard(user, draft.id, 2)
  await conflict(saveBoard(user, legacy, 0))
  await conflict(saveBoard(user, persisted, 2))
  await conflict(deleteBoard(user, draft.id, 2))
  check(!await loadBoard(user, draft.id) && (await listBoards(user)).length === 0, 'A stale operation resurrected the deleted board')

  const shared = image(9 * 1024 * 1024)
  const intent = { requestId: 'request-42', keyId: 7, prompt: '  初雪 🌨  ', model: 'gemini', aspectRatio: '2:3' as const, reference: shared }
  const board: CanvasBoard = { ...createBoard('Pending'), nodes: [
    { ...parent, image: shared }, prompt,
    { ...result('recover'), parentId: prompt.id, intent, error: 'response lost' },
    result('second', image(shared.size)), result('third', image(shared.size)),
  ] }
  const saving = saveBoard(user, board, undefined)
  intent.prompt = 'edited after save was invoked'
  check(await saving === 1, 'New board must commit version one')
  const saved = await loadBoard(user, board.id)
  check(saved?.version === 1, 'New board version was not restored')
  const recovered = saved.nodes[2].intent
  check(recovered?.reference && recovered.reference === saved.nodes[0].image, 'Shared reference and parent must restore as one Blob')
  check(recovered.prompt === '  初雪 🌨  ' && recovered.model === 'gemini' && recovered.aspectRatio === '2:3' && recovered.keyId === 7 && recovered.requestId === 'request-42' && saved.nodes[2].error === 'response lost', 'Generation intent or error changed')
  check(recovered.reference.size === shared.size && recovered.reference.type === shared.type, 'Reference size or MIME changed')
  const hash = async (blob: Blob) => Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', await blob.arrayBuffer()))).join(',')
  check(await hash(recovered.reference) === await hash(shared), 'Reference bytes changed')
  const repeated = { ...saved, nodes: [...saved.nodes, result('duplicate-image', recovered.reference)] }
  try { await saveBoard(user, repeated, 1); throw new Error('Saved more than 32 MiB of visible images') } catch (error) {
    check(error instanceof CanvasArchiveError && error.code === 'limit-exceeded', `Unexpected repeated-image budget result: ${error}`)
  }
  const fork = { ...saved, ...createBoard('Fork'), nodes: saved.nodes }
  check(await saveBoard(user, fork, undefined) === 1, 'Fork must create independent asset rows')
  await deleteBoard(user, board.id, 1)
  const surviving = await loadBoard(user, fork.id)
  check(surviving?.nodes[2].intent?.reference && await hash(surviving.nodes[2].intent.reference) === await hash(shared), 'Deleting the source removed the fork reference')
  check(surviving.nodes[0].image === surviving.nodes[2].intent.reference, 'Fork reference did not reuse its own parent asset')

  // The captured reference must survive changes to its parent, even if it is the only remaining owner.
  surviving.nodes[0] = { ...parent, image: image() }
  check(await saveBoard(user, surviving, 1) === 2, 'Replacing the parent image failed')
  const detached = await loadBoard(user, surviving.id)
  check(detached?.nodes[2].intent?.reference?.size === shared.size && detached.nodes[0].image?.size === png.length, 'Captured reference followed the replacement parent')
  const tooLarge = { ...detached, nodes: [...detached.nodes, result('extra', image(6 * 1024 * 1024))] }
  try { await saveBoard(user, tooLarge, 2); throw new Error('Saved more than 32 MiB') } catch (error) {
    check(error instanceof CanvasArchiveError && error.code === 'limit-exceeded', `Unexpected budget result: ${error}`)
  }
  check((await loadBoard(user, surviving.id))?.version === 2, 'Rejected write advanced the version')
  try { await exportBoard(detached); throw new Error('Exported an unfinished intent') } catch (error) {
    check(error instanceof CanvasArchiveError && error.code === 'unfinished-node', `Unexpected export result: ${error}`)
  }
  const archive = { format: 'sub2api-image-canvas', version: 1, name: 'Bad', viewport: draft.viewport, nodes: [{ ...prompt, parentId: undefined, intent }] }
  try { await importBoard(new Blob([JSON.stringify(archive)])); throw new Error('Imported a local generation intent') } catch (error) {
    check(error instanceof CanvasArchiveError && error.code === 'invalid-document', `Unexpected import result: ${error}`)
  }
  db.close()
}

void run().then(
  () => fetch('/__storage_result', { method: 'POST', body: 'pass' }),
  error => fetch('/__storage_result', { method: 'POST', body: String(error?.stack ?? error) }),
)
