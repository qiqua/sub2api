import { computed, onScopeDispose, ref, shallowRef, watch, type ShallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApiKey } from '@/types'
import { MAX_IMAGE_BYTES, MAX_NODES, MAX_TOTAL_IMAGE_BYTES, newId, type CanvasBoard, type CanvasNode } from './document'
import { checkCanvasJob, listCanvasKeys, listCanvasModels, submitCanvasImage } from './gateway'
import { validateImage } from './storage'
import { branchPoint } from './layout'

interface GenerationOptions {
  board: ShallowRef<CanvasBoard | null>
  startResult(node: CanvasNode): void
  patchResult(id: string, patch: Pick<CanvasNode, 'image' | 'job' | 'error'>): void
  flush(): Promise<void>
  reveal(node: CanvasNode): void
  notify(message: string): void
}

export function useCanvasGeneration(options: GenerationOptions) {
  const { t } = useI18n()
  const keys = shallowRef<ApiKey[]>([])
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
  const polling = new Set<string>()
  const controller = new AbortController()
  let modelRequest = 0

  async function loadKeys() {
    if (loadingKeys.value) return
    loadingKeys.value = true
    keyError.value = false
    try {
      const entries = await listCanvasKeys()
      if (controller.signal.aborted) return
      keys.value = entries
      if (!key.value) keyId.value = entries[0]?.id ?? null
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
    const total = board.nodes.reduce((sum, n) => sum + (n.image?.size ?? 0), image.size)
    if (image.size > MAX_IMAGE_BYTES || total > MAX_TOTAL_IMAGE_BYTES) throw new Error(t('canvas.imageSpace'))
  }

  async function generate(promptNode: CanvasNode) {
    const board = options.board.value
    const selectedKey = key.value
    const selectedModel = model.value.trim()
    if (!board || !selectedKey || submitting.value || promptNode.kind !== 'prompt' || !selectedModel || !promptNode.text.trim()) return
    if (board.nodes.length >= MAX_NODES) return options.notify(t('canvas.nodeLimit', { count: MAX_NODES }))
    if (board.nodes.some(n => n.parentId === promptNode.id && n.job)) return
    const reference = board.nodes.find(n => n.id === promptNode.parentId)?.image
    if (promptNode.parentId && !reference) return options.notify(t('canvas.missingReference'))
    const result: CanvasNode = {
      id: newId(), kind: 'image', text: promptNode.text.trim(), model: selectedModel,
      parentId: promptNode.id, ...branchPoint(board.nodes, promptNode),
    }
    submitting.value = result.id
    options.startResult(result)
    options.reveal(result)
    try {
      const response = await submitCanvasImage({
        key: selectedKey, model: selectedModel, prompt: result.text, reference,
        aspectRatio: aspectRatio.value, requestId: result.id, signal: controller.signal,
      })
      if (controller.signal.aborted || options.board.value?.id !== board.id) return
      if ('job' in response) {
        options.patchResult(result.id, { job: response.job })
        await options.flush().catch(() => {}) // The workspace keeps a retryable local-save error.
      } else {
        await validateImage(response.image)
        if (controller.signal.aborted || options.board.value?.id !== board.id) return
        checkSize(response.image, options.board.value)
        options.patchResult(result.id, { image: response.image })
      }
    } catch (error) {
      if (!controller.signal.aborted && options.board.value?.id === board.id) {
        options.patchResult(result.id, { error: error instanceof Error ? error.message : t('canvas.requestFailed') })
      }
    } finally { submitting.value = null }
  }

  async function poll(node: CanvasNode) {
    const job = node.job
    const id = options.board.value?.id
    if (!job || polling.has(node.id) || controller.signal.aborted) return
    const owner = keys.value.find(k => k.id === job.keyId)
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
        options.patchResult(node.id, { job: undefined, image: result.image, error: undefined })
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

  const pending = computed(() => (options.board.value?.nodes ?? []).filter(n => n.job))
  const pollAll = () => { for (const node of pending.value) if (!pollErrors.value[node.id]) void poll(node) }
  watch([() => JSON.stringify(pending.value.map(n => n.job?.id)), keys], pollAll)
  const timer = window.setInterval(() => { if (!document.hidden) pollAll() }, 5000)
  onScopeDispose(() => { controller.abort(); window.clearInterval(timer) })

  return { keys, keyId, key, models, model, aspectRatio, loadingKeys, loadingModels, keyError, modelError, loadKeys, loadModels, submitting, generate, poll, pollErrors }
}
