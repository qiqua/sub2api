<template>
  <dialog ref="dialog" class="canvas-dialog asset-shelf" :aria-labelledby="titleId" @close="onClosed">
    <header class="dialog-head">
      <h2 :id="titleId">{{ t('canvasLibrary.title') }}</h2>
      <form method="dialog"><button class="canvas-icon" :aria-label="t('common.close')"><Icon name="x" /></button></form>
    </header>
    <div class="shelf-fields">
      <label class="canvas-field">{{ t('canvasLibrary.source') }}
        <select v-model="sourceId" autofocus>
          <option :value="board.id">{{ t('canvasLibrary.current', { name: board.name }) }}</option>
          <option v-for="item in otherBoards" :key="item.id" :value="item.id">{{ item.name }}</option>
        </select>
      </label>
      <label class="canvas-field">{{ t('canvasLibrary.search') }}
        <input v-model="query" type="search" :placeholder="t('canvasLibrary.placeholder')" />
      </label>
    </div>
    <div class="shelf-filters" role="group" :aria-label="t('canvasLibrary.kind')">
      <button v-for="kind in kinds" :key="kind" type="button" :aria-pressed="filter === kind" @click="filter = kind">
        {{ t(`canvasLibrary.${kind}`) }}
      </button>
    </div>
    <p v-if="loading" role="status" class="shelf-message">{{ t('canvasLibrary.loading') }}</p>
    <div v-else-if="failed" class="shelf-message">
      <p role="alert">{{ t('canvasLibrary.unavailable') }}</p>
      <button type="button" class="canvas-control" @click="attempt++">{{ t('canvasLibrary.retry') }}</button>
    </div>
    <p v-else-if="!matches.length" role="status" class="shelf-message">
      {{ t(assets.length ? 'canvasLibrary.noMatches' : 'canvasLibrary.empty') }}
    </p>
    <ul v-else class="shelf-list">
      <li v-for="{ node, label } in matches" :key="node.id">
        <button class="shelf-item" type="button" @click="useAsset(node)">
          <img v-if="node.image" v-preview="node.image" alt="" width="64" height="56" loading="lazy" decoding="async" />
          <span class="shelf-description">
            <span class="shelf-kind">{{ t(`canvas.${node.kind}`) }}<template v-if="node.model"> · {{ node.model }}</template></span>
            <span class="shelf-text">{{ label }}</span>
          </span>
          <span class="shelf-action">{{ t('canvasLibrary.add') }}</span>
        </button>
      </li>
    </ul>
  </dialog>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, useId, watch, type ObjectDirective } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { newId, type CanvasBoard, type CanvasNode } from './document'
import { loadBoard } from './storage'

const props = defineProps<{ userId: number; board: CanvasBoard; boards: { id: string; name: string }[] }>()
const emit = defineEmits<{ use: [node: CanvasNode] }>()
const { t } = useI18n()
const titleId = useId()
const dialog = ref<HTMLDialogElement>()
const open = ref(false)
const sourceId = ref('')
const query = ref('')
const kinds = ['all', 'prompt', 'image'] as const
const filter = ref<typeof kinds[number]>('all')
const loaded = shallowRef<CanvasBoard>()
const loading = ref(false)
const failed = ref(false)
const attempt = ref(0)
const otherBoards = computed(() => props.boards.filter(item => item.id !== props.board.id))
const selectedIsAvailable = computed(() => otherBoards.value.some(item => item.id === sourceId.value))

function show() {
  if (!dialog.value || dialog.value.open || !Number.isSafeInteger(props.userId) || props.userId <= 0) return
  sourceId.value = props.board.id
  query.value = ''
  filter.value = 'all'
  open.value = true
  dialog.value.showModal()
}
function close() {
  open.value = false
  dialog.value?.close()
}
function onClosed() {
  if (!dialog.value?.open) open.value = false
}
watch([() => props.userId, () => props.board.id], close, { flush: 'sync' })

watch([open, sourceId, () => props.userId, selectedIsAvailable, attempt], async ([isOpen, id, userId, available], _, onCleanup) => {
  let stale = false
  onCleanup(() => { stale = true })
  loaded.value = undefined
  failed.value = false
  loading.value = false
  if (!isOpen || id === props.board.id) return
  if (!available) { failed.value = true; return }
  loading.value = true
  try {
    const result = await loadBoard(userId, id)
    if (!stale) {
      loaded.value = result
      failed.value = !result
    }
  } catch {
    if (!stale) failed.value = true
  } finally {
    if (!stale) loading.value = false
  }
})

const assets = computed(() => {
  if (!open.value) return []
  const source = sourceId.value === props.board.id ? props.board : loaded.value
  if (!source || source.id !== sourceId.value) return []
  return source.nodes.flatMap((node, index) => {
    if (node.job || (node.kind === 'image' ? !node.image : !node.text.trim())) return []
    const label = node.text.trim() || t('canvasLibrary.unnamedImage', { number: index + 1 })
    return [{ node, label }]
  })
})
const matches = computed(() => {
  const term = query.value.trim().toLocaleLowerCase()
  return assets.value.filter(({ node, label }) =>
    (filter.value === 'all' || filter.value === node.kind) &&
    (!term || label.toLocaleLowerCase().includes(term) || node.model?.toLocaleLowerCase().includes(term)),
  )
})

function useAsset(node: CanvasNode) {
  if (!open.value || !matches.value.some(asset => asset.node === node)) return
  const copy: CanvasNode = { id: newId(), x: 0, y: 0, kind: node.kind, text: node.text }
  if (node.kind === 'image') copy.image = node.image
  if (node.model) copy.model = node.model
  close()
  emit('use', copy)
}

const vPreview: ObjectDirective<HTMLImageElement, Blob> = {
  mounted(el, { value }) { el.src = URL.createObjectURL(value) },
  updated(el, { value, oldValue }) {
    if (value === oldValue) return
    URL.revokeObjectURL(el.src)
    el.src = URL.createObjectURL(value)
  },
  unmounted(el) { URL.revokeObjectURL(el.src) },
}

defineExpose({ show, close })
</script>

<style scoped>
.asset-shelf:modal { width: min(36rem, calc(100% - 1.5rem)); max-height: calc(100dvh - 2rem); overflow-y: auto; overscroll-behavior: contain; }
.asset-shelf::backdrop { overflow: hidden; overscroll-behavior: contain; }
.shelf-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 13rem), 1fr)); gap: .75rem; }
.shelf-fields :is(label, input, select) { min-width: 0; width: 100%; }
.shelf-filters { display: flex; flex-wrap: wrap; gap: .375rem; border-bottom: 1px solid var(--canvas-line); }
.shelf-filters button { min-height: 44px; padding: 0 .6rem; border-bottom: 2px solid transparent; color: var(--canvas-muted); }
.shelf-filters button[aria-pressed='true'] { color: var(--canvas-text); font-weight: 650; border-color: var(--canvas-accent); }
.asset-shelf :is(button, select) { cursor: pointer; }
.asset-shelf :is(button, input, select):focus-visible { outline: 2px solid var(--canvas-accent); outline-offset: 1px; }
.shelf-list { margin: 0; padding: 0; list-style: none; }
.shelf-list li + li { border-top: 1px solid var(--canvas-line); }
.shelf-item { width: 100%; min-height: 64px; display: flex; align-items: center; gap: .875rem; padding: .625rem .25rem; text-align: start; }
.shelf-item img { width: 4rem; height: 3.5rem; object-fit: contain; border-radius: .3rem; background: var(--canvas-soft); }
.shelf-item:hover, .shelf-filters button:hover { background: var(--canvas-soft); }
.shelf-item:active, .shelf-filters button:active { background: var(--canvas-line); }
.shelf-description { min-width: 0; flex: 1; display: grid; gap: .2rem; overflow-wrap: anywhere; }
.shelf-kind, .shelf-action { font-size: .75rem; }
.shelf-kind, .shelf-message { color: var(--canvas-muted); }
.shelf-action { color: var(--canvas-accent); font-weight: 650; }
.shelf-text { display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
.shelf-message { padding: 1rem 0; }
.shelf-message button { margin-top: .75rem; }
@media (max-width: 30rem) {
  .shelf-item { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: .3rem .75rem; }
  .shelf-item img { grid-row: span 2; }
  .shelf-description, .shelf-action { grid-column: 2; }
  .shelf-fields input { font-size: 1rem; }
}
</style>
