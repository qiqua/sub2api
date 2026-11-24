import { computed, onScopeDispose, ref, shallowRef, watch, type ShallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApiKey } from '@/types'
import { boardImageBytes, MAX_IMAGE_BYTES, MAX_NODES, MAX_TOTAL_IMAGE_BYTES, newId, type CanvasBoard, type CanvasJob, type CanvasNode, type CanvasSubmitIntent } from './document'
import { checkCanvasJob, isCanvasKey, isGeminiKey, isTerminalCanvasSubmitError, listCanvasKeys, listCanvasModels, lookupCanvasJob, submitCanvasImage, submitCanvasIntent } from './gateway'
import { validateImage } from './storage'
import { branchPoint } from './layout'

interface GenerationOptions {
  board: ShallowRef<CanvasBoard | null>
  startResult(node: CanvasNode): void
  patchResult(id: string, patch: Partial<Pick<CanvasNode, 'image' | 'job' | 'error' | 'intent'>>): void
  flush(): Promise<void>
  reveal(node: CanvasNode): void
  notify(message: string): void
}

export function useCanvasGeneration(options: GenerationOptions) {
  const { t } = useI18n()
  const recoveryKeys = shallowRef<ApiKey[]>([])
  const keys = computed(() => recoveryKeys.value.filter(isCanvasKey))
  const keyId = ref<number | null>(null)
  const key = computed(() => keys.value.find(entry => entry.id === keyId.value))
  const models = shallowRef<string[]>([])
  const model = ref('')
  const aspectRatio = ref<'1:1' | '3:2' | '2:3'>('1:1')
  const loadingKeys = ref(false)
  const loadingModels = ref(false)
  const keyError = ref(false)
  const modelError = ref(false)
  const submitting = ref<string | null>(null)
  const pollErrors = shallowRef<Record<string, string>>({})
  const missing = shallowRef<Record<string, boolean>>({})
  const polling = new Set<string>()
  const controller = new AbortController()
  let modelRequest = 0

  async function loadKeys() {
    if (loadingKeys.value) return
    loadingKeys.value = true
    keyError.value = false
    try {
      const entries = await listCanvasKeys(true)
      if (controller.signal.aborted) return
      recoveryKeys.value = entries
      if (!key.value) keyId.value = keys.value[0]?.id ?? null
    } catch { keyError.value = true } finally { loadingKeys.value = false }
  }

  async function loadModels() {
    const request = ++modelRequest
    const current = key.value
    models.value = []
    model.value = ''
    modelError.value = false
    loadingModels.value = !!current
    if (!current) return
    try {
      const entries = await listCanvasModels(current)
      if (request !== modelRequest || controller.signal.aborted) return
      models.value = entries
      model.value = entries[0] ?? ''
    } catch { if (request === modelRequest) modelError.value = true }
    finally { if (request === modelRequest) loadingModels.value = false }
  }
  watch(key, loadModels)

  function checkSize(image: Blob, board: CanvasBoard) {
    const total = boardImageBytes(board.nodes) + image.size
    if (image.size > MAX_IMAGE_BYTES || total > MAX_TOTAL_IMAGE_BYTES) throw new Error(t('canvas.imageSpace'))
  }

  async function generate(promptNode: CanvasNode) {
    const board = options.board.value
    const selectedKey = key.value
    const selectedModel = model.value.trim()
    if (!board || !selectedKey || submitting.value || promptNode.kind !== 'prompt' || !selectedModel || !promptNode.text.trim()) return
    if (board.nodes.length >= MAX_NODES) return options.notify(t('canvas.nodeLimit', { count: MAX_NODES }))
    if (board.nodes.some(n => n.parentId === promptNode.id && (n.job || n.intent))) return
    const reference = board.nodes.find(n => n.id === promptNode.parentId)?.image
    if (promptNode.parentId && !reference) return options.notify(t('canvas.missingReference'))
    const requestId = newId()
    const intent: CanvasSubmitIntent | undefined = isGeminiKey(selectedKey) ? Object.freeze({
      requestId, keyId: selectedKey.id, prompt: promptNode.text.trim(), model: selectedModel,
      aspectRatio: aspectRatio.value, ...(reference ? { reference } : {}),
    }) : undefined
    const result: CanvasNode = {
      id: newId(), kind: 'image', text: promptNode.text.trim(), model: selectedModel,
      parentId: promptNode.id, ...branchPoint(board.nodes, promptNode), ...(intent ? { intent } : {}),
    }
    submitting.value = result.id
    options.startResult(result)
    options.reveal(result)
    try {
      if (intent) {
        await processIntent(result, 'initial')
        return
      }
      const response = await submitCanvasImage({
        key: selectedKey, model: selectedModel, prompt: result.text, reference,
        aspectRatio: aspectRatio.value, requestId, signal: controller.signal,
      })
      if (controller.signal.aborted || !options.board.value?.nodes.some(n => n.id === result.id)) return
      if ('job' in response) {
        options.patchResult(result.id, { job: response.job, intent: undefined, error: undefined })
        await options.flush().catch(() => {}) // The workspace keeps a retryable local-save error.
      } else {
        await validateImage(response.image)
        if (controller.signal.aborted || !options.board.value?.nodes.some(n => n.id === result.id)) return
        checkSize(response.image, options.board.value)
        options.patchResult(result.id, { image: response.image, intent: undefined, error: undefined })
      }
    } catch (error) {
      if (!controller.signal.aborted && options.board.value?.nodes.some(n => n.id === result.id)) {
        options.patchResult(result.id, { error: error instanceof Error ? error.message : t('canvas.requestFailed') })
      }
    } finally { submitting.value = null }
  }

  async function poll(node: CanvasNode) {
    const job = node.job
    const id = options.board.value?.id
    if (!job || polling.has(node.id) || controller.signal.aborted) return
    const owner = recoveryKeys.value.find(k => k.id === job.keyId)
    if (!owner) return
    polling.add(node.id)
    const current = () => !controller.signal.aborted && options.board.value?.id === id && options.board.value?.nodes.some(n => n.id === node.id && n.job?.id === job.id)
    try {
      const result = await checkCanvasJob(owner, job, controller.signal)
      if (!current()) return
      if (result.status === 'complete' && options.board.value) {
        await validateImage(result.image)
        if (!current()) return
        checkSize(result.image, options.board.value)
        options.patchResult(node.id, { job: undefined, image: result.image, intent: undefined, error: undefined })
      } else if (result.status === 'failed') {
        options.patchResult(node.id, { job: undefined, error: result.message || t('canvas.requestFailed') })
      }
      if (pollErrors.value[node.id]) {
        const updated = { ...pollErrors.value }
        delete updated[node.id]
        pollErrors.value = updated
      }
    } catch (error) {
      if (current()) pollErrors.value = { ...pollErrors.value, [node.id]: error instanceof Error ? error.message : t('canvas.checkFailed') }
    } finally { polling.delete(node.id) }
  }

  async function processIntent(node: CanvasNode, mode: 'initial' | 'lookup' | 'retry') {
    const intent = node.intent
    const owner = recoveryKeys.value.find(entry => entry.id === intent?.keyId)
    if (!intent || !owner || polling.has(node.id) || controller.signal.aborted) return
    const current = () => !controller.signal.aborted && options.board.value?.nodes.some(n => n.id === node.id && n.intent?.requestId === intent.requestId)
    if (!current()) return
    polling.add(node.id)
    const signal = controller.signal
    let posting = false
    try {
      let job: CanvasJob | undefined
      if (mode !== 'initial') {
        job = await lookupCanvasJob(owner, intent.requestId, signal)
        if (!current()) return
        missing.value = { ...missing.value, [node.id]: !job }
      }
      if (!job && mode !== 'lookup') {
        // Both paths can start with an unsaved intent (e.g. a previous quota failure).
        const boardId = options.board.value?.id
        await options.flush()
        if (!current()) return
        // The winning tab may already have submitted a different intent for this prompt.
        // Leave this copy for explicit recovery instead of starting another billable job.
        if (options.board.value?.id !== boardId) return
        posting = true
        job = await submitCanvasIntent(owner, intent, signal)
      }
      if (!current()) return
      if (job) {
        options.patchResult(node.id, { job, intent: undefined, error: undefined })
        await options.flush().catch(() => {})
      }
      pollErrors.value = { ...pollErrors.value, [node.id]: '' }
    } catch (error) {
      if (!current()) return
      const removed = error && typeof error === 'object' && 'status' in error && error.status === 410 && 'code' in error && error.code === 'BATCH_IMAGE_SUBMISSION_REMOVED'
      if (removed || (posting && isTerminalCanvasSubmitError(error))) {
        options.patchResult(node.id, { intent: undefined, error: removed ? t('canvas.submissionRemoved') : t('canvas.submissionRejected') })
        await options.flush().catch(() => {})
      } else {
        missing.value = { ...missing.value, [node.id]: false }
        pollErrors.value = { ...pollErrors.value, [node.id]: t(posting ? 'canvas.requestUnknown' : 'canvas.checkFailed') }
      }
    } finally { polling.delete(node.id) }
    // A lost POST acknowledgement may still have created a job. Only a GET is automatic.
    if (posting && current()) await processIntent(node, 'lookup')
  }

  async function retry(node: CanvasNode) {
    if (submitting.value || polling.has(node.id)) return
    submitting.value = node.id
    try { await processIntent(node, 'retry') } finally { submitting.value = null }
  }

  const pending = computed(() => (options.board.value?.nodes ?? []).filter(n => n.job))
  const pollAll = () => {
    for (const node of pending.value) if (!pollErrors.value[node.id]) void poll(node)
  }
  function reconcile() {
    pollAll()
    for (const node of options.board.value?.nodes ?? []) {
      if (node.intent && submitting.value !== node.id) void processIntent(node, 'lookup')
    }
  }
  const pendingIdentities = computed(() => {
    const board = options.board.value
    return JSON.stringify([board?.id, board?.nodes.map(n => n.intent?.requestId ?? n.job?.id).filter(Boolean)])
  })
  watch([pendingIdentities, recoveryKeys], reconcile)
  const timer = window.setInterval(() => { if (!document.hidden) pollAll() }, 5000)
  onScopeDispose(() => { controller.abort(); window.clearInterval(timer) })

  return { keys, recoveryKeys, keyId, key, models, model, aspectRatio, loadingKeys, loadingModels, keyError, modelError, loadKeys, loadModels, submitting, generate, retry, poll, checkIntent: (node: CanvasNode) => processIntent(node, 'lookup'), missing, pollErrors }
}
