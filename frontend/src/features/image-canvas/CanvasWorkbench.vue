<template>
  <main class="image-canvas" @keydown="onKeydown">
    <header class="canvas-header">
      <RouterLink class="canvas-icon" to="/dashboard" :aria-label="t('canvas.back')" :title="t('canvas.back')"><Icon name="chevronLeft" /></RouterLink>
      <h1>{{ t('nav.imageWorkbench') }}</h1>
      <button v-if="board" type="button" class="board-switch" :title="t('canvas.projects')" @click="projects?.show()"><Icon name="folder" size="sm" /><span>{{ board.name }}</span><Icon name="chevronDown" size="xs" /></button>
      <span v-if="board" class="save-state" role="status">{{ t(`canvas.save.${saveState}`) }}</span>
      <button v-if="board" type="button" class="canvas-control library-toggle" @click="library?.show()"><Icon name="image" size="sm" />{{ t('canvas.library') }}</button>
    </header>
    <template v-if="board">
      <div class="canvas-desk" :class="{ 'has-inspector': selected }" :inert="busy || undefined">
        <CanvasScene ref="scene" :key="board.id" :nodes="board.nodes" :viewport="board.viewport" :selected-id="selectedId" @select="select" @move="moveNode" @view="setViewport" @add="addPrompt" @files="addFiles">
          <template #empty>
            <div class="canvas-start">
              <h2>{{ t('canvas.emptyTitle') }}</h2>
              <p>{{ t('canvas.emptyHint') }}</p>
              <div class="inline-actions"><button type="button" class="canvas-control canvas-primary" @click.stop="addPrompt()"><Icon name="edit" size="sm" />{{ t('canvas.writePrompt') }}</button><button type="button" class="canvas-control" @click.stop="imageInput?.click()"><Icon name="upload" size="sm" />{{ t('canvas.addImage') }}</button></div>
            </div>
          </template>
        </CanvasScene>
        <CanvasInspector v-if="selected" :key="selected.id" :node="selected" :references="references" :keys="keys" :models="models" :loading-keys="loadingKeys" :loading-models="loadingModels" :key-error="keyError" :model-error="modelError" :generating="selectedBusy" :poll-error="jobMessage" :has-owner="!!owner" :request-missing="!!missing[selected.id]" :can-retry="!!owner && isGeminiKey(owner)" v-model:key-id="keyId" v-model:model="model" v-model:aspect-ratio="aspectRatio" @close="select(null)" @text="setText" @finish-edit="finishEdit" @reference="setReference" @branch="derive" @reuse="reusePrompt" @remove="removeNode" @poll="selected.intent ? checkIntent(selected) : poll(selected)" @retry="retry(selected)" @generate="generate(selected)" @reload-keys="loadKeys" @reload-models="loadModels" />
      </div>
      <footer class="canvas-tools">
        <div class="tool-group">
          <button type="button" class="canvas-control add-control" :disabled="busy" @click="addPrompt()"><Icon name="plus" size="md" /><span>{{ t('canvas.addPrompt') }}</span></button>
          <button type="button" class="canvas-icon" :disabled="busy" :aria-label="t('canvas.addImage')" :title="t('canvas.addImage')" @click="imageInput?.click()"><Icon name="upload" /></button>
          <button type="button" class="canvas-icon" :disabled="!canUndo || busy" :aria-label="t('canvas.undo')" :title="t('canvas.undo')" @click="undo"><Icon name="history" /></button>
          <button type="button" class="canvas-icon" :disabled="!canRedo || busy" :aria-label="t('canvas.redo')" :title="t('canvas.redo')" @click="redo"><Icon class="mirror" name="history" /></button>
        </div>
        <div class="tool-group viewport-tools">
          <button type="button" class="canvas-icon" :disabled="board.viewport.zoom <= 0.2" :aria-label="t('canvas.zoomOut')" :title="t('canvas.zoomOut')" @click="scene?.zoomBy(0.8)">−</button>
          <output class="zoom-value">{{ Math.round(board.viewport.zoom * 100) }}%</output>
          <button type="button" class="canvas-icon" :disabled="board.viewport.zoom >= 3" :aria-label="t('canvas.zoomIn')" :title="t('canvas.zoomIn')" @click="scene?.zoomBy(1.25)"><Icon name="plus" size="sm" /></button>
          <button type="button" class="canvas-control" :title="t('canvas.fit')" @click="scene?.fit()">{{ t('canvas.fit') }}</button>
          <button type="button" class="canvas-icon" :disabled="board.nodes.length < 2 || busy" :aria-label="t('canvas.arrange')" :title="t('canvas.arrange')" @click="arrange"><Icon name="grid" /></button>
        </div>
        <button type="button" class="canvas-icon help-toggle" :aria-label="t('canvas.help')" :title="t('canvas.help')" @click="help?.showModal()"><Icon name="questionCircle" /></button>
      </footer>
      <input ref="imageInput" type="file" multiple accept="image/png,image/jpeg,image/webp" hidden @change="chooseImages" />
      <div v-if="message || recoveredBoard || saveState === 'failed'" class="canvas-notice" role="alert">
        <span>{{ message || (saveState === 'failed' ? t('canvas.saveError') : t('canvas.conflictSaved', { name: recoveredBoard })) }}</span>
        <button v-if="saveState === 'failed'" type="button" class="canvas-link" @click="retrySave">{{ t('canvas.retrySave') }}</button>
        <button v-if="message || recoveredBoard" type="button" class="canvas-icon" :aria-label="t('common.close')" @click="message = ''; recoveredBoard = ''"><Icon name="x" size="sm" /></button>
      </div>
      <CanvasProjects ref="projects" :board="board" :boards="boards" :busy="busy" :locked="!!submitting" :error="projectError" @clear-error="projectError = ''" @open="id => projectAction(() => open(id))" @create="projectAction(newBoard)" @import="file => projectAction(() => importFile(file))" @rename="name => projectAction(async () => { rename(name); await flush() })" @remove="projectAction(remove)" @export="exportFile" />
      <CanvasLibrary ref="library" :user-id="userId" :board="board" :boards="boards" @use="useAsset" />
      <dialog ref="help" class="canvas-dialog" :aria-label="t('canvas.help')">
        <header class="dialog-head"><h2>{{ t('canvas.help') }}</h2><form method="dialog"><button class="canvas-icon" :aria-label="t('common.close')"><Icon name="x" /></button></form></header>
        <dl class="canvas-shortcuts"><template v-for="item in gestures" :key="item"><dt>{{ t(`canvas.shortcuts.${item}.key`) }}</dt><dd>{{ t(`canvas.shortcuts.${item}.action`) }}</dd></template></dl>
      </dialog>
    </template>
    <div v-else class="canvas-load" role="status"><p>{{ t(loadFailed ? 'canvas.loadError' : 'canvas.loading') }}</p><button v-if="loadFailed" type="button" class="canvas-control" @click="initialize">{{ t('canvas.refresh') }}</button></div>
  </main>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { saveBlob } from '@/api/batchImage'
import CanvasInspector from './CanvasInspector.vue'
import CanvasLibrary from './CanvasLibrary.vue'
import CanvasProjects from './CanvasProjects.vue'
import CanvasScene from './CanvasScene.vue'
import { boardImageBytes, MAX_NODES, MAX_TOTAL_IMAGE_BYTES, newId, type CanvasNode, type Point } from './document'
import { arrangeNodes, branchPoint, referenceChoices, vacantPoint } from './layout'
import { CanvasArchiveError, exportBoard, validateImage } from './storage'
import { isGeminiKey } from './gateway'
import { useCanvasGeneration } from './useCanvasGeneration'
import { useCanvasWorkspace } from './useCanvasWorkspace'
import './workbench.css'

const props = defineProps<{ userId: number }>()
const { t } = useI18n()
const { board, boards, busy, saveState, recoveredBoard, loadFailed, initialize, open, newBoard, importFile, rename, remove, flush,
  setNodes, setViewport, finishEdit, startResult, patchResult, canUndo, canRedo, undo, redo } = useCanvasWorkspace(props.userId, () => t('canvas.untitled'), name => t('canvas.conflictName', { name }))
const scene = ref<InstanceType<typeof CanvasScene>>()
const projects = ref<InstanceType<typeof CanvasProjects>>()
const library = ref<InstanceType<typeof CanvasLibrary>>()
const help = ref<HTMLDialogElement>()
const imageInput = ref<HTMLInputElement>()
const selectedId = ref<string | null>(null)
const selected = computed(() => board.value?.nodes.find(n => n.id === selectedId.value))
const references = computed(() => selected.value?.kind === 'prompt' ? referenceChoices(board.value!.nodes, selected.value.id) : [])
const message = ref('')
const projectError = ref('')
const gestures = ['pan', 'zoom', 'create', 'move', 'undo', 'paste'] as const
const { keys, recoveryKeys, keyId, models, model, aspectRatio, loadingKeys, loadingModels, keyError, modelError, loadKeys, loadModels, submitting, generate, retry, poll, checkIntent, missing, pollErrors } = useCanvasGeneration({
  board, startResult, patchResult, flush, reveal, notify: msg => { message.value = msg },
})
const selectedBusy = computed(() => {
  const node = selected.value
  if (!node) return false
  if (node.kind === 'image') return submitting.value === node.id
  return !!submitting.value || !!board.value?.nodes.some(n => n.parentId === node.id && (n.job || n.intent))
})
const owner = computed(() => recoveryKeys.value.find(k => k.id === (selected.value?.job ?? selected.value?.intent)?.keyId))
const jobMessage = computed(() => {
  const n = selected.value
  if (!n || (!n.job && !n.intent)) return ''
  if (!loadingKeys.value && !owner.value) return t('canvas.jobKeyMissing')
  return pollErrors.value[n.id] || ''
})

function select(id: string | null) {
  if (selectedId.value !== id) finishEdit()
  selectedId.value = id
}
async function reveal(node: CanvasNode) {
  select(node.id)
  await nextTick()
  scene.value?.centerOn(node)
}
function canAdd(count = 1) {
  if (!board.value || busy.value) return false
  if (board.value.nodes.length + count <= MAX_NODES) return true
  message.value = t('canvas.nodeLimit', { count: MAX_NODES })
  return false
}
function addPrompt(point?: Point, text = '', parentId?: string) {
  if (!board.value || !canAdd()) return
  const nodes = board.value.nodes
  const node: CanvasNode = { id: newId(), kind: 'prompt', text, parentId, ...vacantPoint(nodes, point ?? scene.value!.center()) }
  setNodes([...nodes, node])
  void reveal(node)
}
function derive() {
  const node = selected.value
  if (node?.image && board.value) addPrompt(branchPoint(board.value.nodes, node), '', node.id)
}
function reusePrompt() {
  const node = selected.value
  if (!node || !board.value) return
  const source = board.value.nodes.find(n => n.id === node.parentId)
  addPrompt(branchPoint(board.value.nodes, node), node.text, source?.parentId)
}
function setText(text: string) {
  if (!board.value || !selected.value) return
  const id = selected.value.id
  setNodes(board.value.nodes.map(n => n.id === id ? { ...n, text } : n), `text:${id}`)
}
function setReference(id: string) {
  if (!board.value || (id && !references.value.some(n => n.id === id))) return
  setNodes(board.value.nodes.map(n => n.id === selectedId.value ? { ...n, parentId: id || undefined } : n))
}
function moveNode(id: string, x: number, y: number) {
  const nodes = board.value?.nodes
  const node = nodes?.find(n => n.id === id)
  if (nodes && node && (node.x !== x || node.y !== y)) setNodes(nodes.map(n => n.id === id ? { ...n, x, y } : n))
}
function removeNode() {
  const node = selected.value
  if (!node || !board.value || node.id === submitting.value) return
  if ((node.job || node.intent) && !window.confirm(t('canvas.removeJob'))) return
  setNodes(board.value.nodes.filter(n => n.id !== node.id).map(n => n.parentId === node.id ? { ...n, parentId: undefined } : n))
  select(null)
}
async function arrange() {
  if (!board.value) return
  setNodes(arrangeNodes(board.value.nodes))
  await nextTick()
  scene.value?.fit()
}
function errorLabel(error: unknown) {
  return error instanceof CanvasArchiveError ? t(`canvas.archive.${error.code}`) : t('canvas.fileError')
}
async function addFiles(files: File[], at?: Point) {
  const target = board.value?.id
  if (!target || !files.length || !canAdd(files.length)) return
  try {
    for (const file of files) await validateImage(file)
    const current = board.value
    if (!current || current.id !== target || !canAdd(files.length)) return
    const total = boardImageBytes(current.nodes) + files.reduce((size, f) => size + f.size, 0)
    if (total > MAX_TOTAL_IMAGE_BYTES) { message.value = t('canvas.imageSpace'); return }
    const nodes = [...current.nodes]
    const point = at ?? scene.value!.center()
    for (const file of files) nodes.push({ id: newId(), kind: 'image', text: file.name, image: file, ...vacantPoint(nodes, point) })
    setNodes(nodes)
    void reveal(nodes[nodes.length - files.length])
  } catch (error) { message.value = errorLabel(error) }
}
function chooseImages(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  void addFiles(files)
}
function onPaste(event: ClipboardEvent) {
  if (event.target instanceof HTMLElement && event.target.closest('textarea,input,[contenteditable=true],dialog')) return
  const files = Array.from(event.clipboardData?.files ?? [])
  if (files.length) { event.preventDefault(); void addFiles(files) }
}
function useAsset(source: CanvasNode) {
  if (!board.value || !canAdd()) return
  const image = source.image
  if (image && boardImageBytes(board.value.nodes) + image.size > MAX_TOTAL_IMAGE_BYTES) {
    library.value?.close()
    message.value = t('canvas.imageSpace')
    return
  }
  const node: CanvasNode = { id: newId(), kind: source.kind, text: source.text, model: source.model, image, ...vacantPoint(board.value.nodes, scene.value!.center()) }
  setNodes([...board.value.nodes, node])
  library.value?.close()
  void reveal(node)
}
function onKeydown(event: KeyboardEvent) {
  if (event.defaultPrevented || event.isComposing || !(event.target instanceof HTMLElement) || event.target.closest('textarea,input,select,dialog,[contenteditable=true]')) return
  const key = event.key.toLowerCase()
  if ((event.ctrlKey || event.metaKey) && (key === 'z' || key === 'y')) {
    event.preventDefault()
    if (key === 'y' || event.shiftKey) redo(); else undo()
  } else if (key === 'escape') select(null)
  else if (key === 'delete' && selected.value && !selectedBusy.value) removeNode()
}
async function projectAction(action: () => Promise<void>) {
  projectError.value = ''
  try { await action(); projects.value?.close() } catch (error) { projectError.value = errorLabel(error) }
}
async function exportFile() {
  if (!board.value) return
  const current = board.value
  projectError.value = ''
  try { saveBlob(await exportBoard(current), `${current.name}.canvas.json`) } catch (error) { projectError.value = errorLabel(error) }
}
async function retrySave() {
  try { await flush(); message.value = '' } catch { message.value = t('canvas.saveError') }
}
function saveOnHide() { if (document.hidden) void flush().catch(() => {}) }
function warnBeforeUnload(event: BeforeUnloadEvent) {
  if (submitting.value || saveState.value !== 'saved') { event.preventDefault(); event.returnValue = '' }
}
watch(() => board.value?.id, () => { select(null); message.value = '' })
onBeforeRouteLeave(async () => {
  if (submitting.value && !window.confirm(t('canvas.leaveRequest'))) return false
  try { await flush() } catch { return window.confirm(t('canvas.leaveUnsaved')) }
})
onMounted(() => {
  void initialize()
  void loadKeys()
  document.addEventListener('visibilitychange', saveOnHide)
  document.addEventListener('paste', onPaste)
  window.addEventListener('beforeunload', warnBeforeUnload)
})
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', saveOnHide)
  document.removeEventListener('paste', onPaste)
  window.removeEventListener('beforeunload', warnBeforeUnload)
})
</script>
