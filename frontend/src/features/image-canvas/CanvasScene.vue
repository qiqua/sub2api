<template>
  <section
    ref="stageRef"
    class="canvas-scene"
    :style="gridStyle"
    tabindex="0"
    role="region"
    :aria-label="t('canvas.sceneLabel')"
    @pointerdown="onStagePointerDown"
    @pointermove="onStagePointerMove"
    @pointerup="onStagePointerEnd"
    @pointercancel="onStagePointerEnd"
    @lostpointercapture="onStagePointerEnd"
    @dblclick="onStageDoubleClick"
    @wheel="onWheel"
    @dragover.prevent
    @drop.prevent="onDrop"
  >
    <div class="canvas-world" :style="worldStyle">
      <svg class="canvas-connections" aria-hidden="true">
        <path
          v-for="connection in connections"
          :key="connection.id"
          :d="connection.path"
          class="connection-path"
        />
      </svg>

      <article
        v-for="node in nodes"
        :key="node.id"
        class="canvas-node"
        :class="[`canvas-node--${node.kind}`, { 'is-selected': selectedId === node.id }]"
        :style="nodeStyle(node)"
        :tabindex="0"
        :aria-label="nodeLabel(node)"
        role="group"
        @click.stop="selectNode(node.id)"
        @keydown.enter.prevent="selectNode(node.id)"
        @keydown.space.prevent="selectNode(node.id)"
      >
        <button
          type="button"
          class="node-handle"
          :aria-label="t('canvas.moveNode', { label: node.text || t(`canvas.${node.kind}`) })"
          @pointerdown.stop="startNodeDrag($event, node)"
          @pointermove.stop="onNodeDrag"
          @pointerup.stop="finishNodeDrag"
          @pointercancel.stop="finishNodeDrag"
          @lostpointercapture.stop="finishNodeDrag"
          @keydown="onNodeHandleKeydown($event, node)"
        >
          <span aria-hidden="true">⋮⋮</span>
        </button>

        <div class="node-content">
          <div v-if="node.kind === 'image'" class="node-thumbnail">
            <img
              v-if="imageUrl(node)"
              :src="imageUrl(node)"
              :alt="node.text || t('canvas.imageNode')"
              width="172"
              height="72"
              draggable="false"
            />
            <span v-else class="node-thumbnail-empty" aria-hidden="true">▧</span>
          </div>
          <div class="node-kicker">{{ t(`canvas.${node.kind}`) }}</div>
          <p class="node-text">{{ node.text || t(`canvas.${node.kind}Node`) }}</p>
          <p v-if="node.kind === 'image' && (!imageUrl(node) || node.error || node.job)" class="node-status">{{ node.error ? t('canvas.interrupted') : node.job ? t('canvas.generating') : t('canvas.pending') }}</p>
        </div>
      </article>
    </div>

    <button
      v-if="minimapBounds"
      type="button"
      class="canvas-minimap"
      :aria-label="t('canvas.minimap')"
      @click="onMinimapClick"
      @keydown.enter.prevent="fit"
      @keydown.space.prevent="fit"
    >
      <span
        v-for="node in nodes"
        :key="`mini-${node.id}`"
        class="minimap-node"
        :class="{ 'is-selected': selectedId === node.id }"
        :style="minimapNodeStyle(node)"
      />
      <span class="minimap-viewport" :style="minimapViewportStyle" aria-hidden="true" />
    </button>

    <div v-if="nodes.length === 0" class="canvas-empty">
      <slot name="empty" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CanvasNode, Point, Viewport } from './document'

const CARD_WIDTH = 220
const CARD_HEIGHT = 164
const MIN_ZOOM = 0.2
const MAX_ZOOM = 3
const GRID_SIZE = 24
const MINIMAP_WIDTH = 180
const MINIMAP_HEIGHT = 112
const MINIMAP_PADDING = 10

type Props = {
  nodes: CanvasNode[]
  viewport: Viewport
  selectedId: string | null
}

type Emits = {
  (event: 'select', id: string | null): void
  (event: 'move', id: string, x: number, y: number): void
  (event: 'view', viewport: Viewport): void
  (event: 'add', point: Point): void
  (event: 'files', files: File[], point: Point): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()
const nodes = computed(() => props.nodes)
const selectedId = computed(() => props.selectedId)
const stageRef = ref<HTMLElement | null>(null)
const localViewport = ref<Viewport>({ x: props.viewport.x, y: props.viewport.y, zoom: props.viewport.zoom })
const dragPoint = ref<Point | null>(null)
const dragNodeId = ref<string | null>(null)
const dragOffset = ref<Point>({ x: 0, y: 0 })
const panLast = ref<Point | null>(null)
const activePointers = new Map<number, { x: number; y: number }>()
const pinchStart = ref<{ distance: number; midpoint: Point; viewport: Viewport } | null>(null)
const objectUrls = new Map<string, string>()
let viewChanged = false
let dragPointer: number | null = null
let tapStart: Point | null = null


const minimapBounds = computed(() => {
  if (!props.nodes.length) return null
  const minX = Math.min(...props.nodes.map((node) => node.x))
  const minY = Math.min(...props.nodes.map((node) => node.y))
  const maxX = Math.max(...props.nodes.map((node) => node.x + CARD_WIDTH))
  const maxY = Math.max(...props.nodes.map((node) => node.y + CARD_HEIGHT))
  return { minX, minY, width: Math.max(CARD_WIDTH, maxX - minX), height: Math.max(CARD_HEIGHT, maxY - minY) }
})

const minimapScale = computed(() => {
  const bounds = minimapBounds.value
  if (!bounds) return 1
  return Math.min(
    (MINIMAP_WIDTH - MINIMAP_PADDING * 2) / bounds.width,
    (MINIMAP_HEIGHT - MINIMAP_PADDING * 2) / bounds.height,
  )
})

const minimapNodeStyle = (node: CanvasNode) => {
  const bounds = minimapBounds.value
  if (!bounds) return {}
  const scale = minimapScale.value
  return {
    left: `${MINIMAP_PADDING + (node.x - bounds.minX) * scale}px`,
    top: `${MINIMAP_PADDING + (node.y - bounds.minY) * scale}px`,
    width: `${CARD_WIDTH * scale}px`,
    height: `${CARD_HEIGHT * scale}px`,
  }
}

const minimapViewportStyle = computed(() => {
  const bounds = minimapBounds.value
  const rect = stageRef.value?.getBoundingClientRect()
  if (!bounds || !rect) return {}
  const scale = minimapScale.value
  const worldLeft = -localViewport.value.x / localViewport.value.zoom
  const worldTop = -localViewport.value.y / localViewport.value.zoom
  return {
    left: `${MINIMAP_PADDING + (worldLeft - bounds.minX) * scale}px`,
    top: `${MINIMAP_PADDING + (worldTop - bounds.minY) * scale}px`,
    width: `${rect.width / localViewport.value.zoom * scale}px`,
    height: `${rect.height / localViewport.value.zoom * scale}px`,
  }
})

const worldStyle = computed(() => ({
  transform: `translate3d(${localViewport.value.x}px, ${localViewport.value.y}px, 0) scale(${localViewport.value.zoom})`,
}))

const gridStyle = computed(() => ({
  backgroundSize: `${GRID_SIZE * localViewport.value.zoom}px ${GRID_SIZE * localViewport.value.zoom}px`,
  backgroundPosition: `${localViewport.value.x}px ${localViewport.value.y}px`,
}))

const connections = computed(() => {
  const byId = new Map(props.nodes.map((node) => [node.id, node]))
  return props.nodes.flatMap((node) => {
    if (!node.parentId) return []
    const parent = byId.get(node.parentId)
    if (!parent) return []
    const from = displayedPoint(parent)
    const to = displayedPoint(node)
    const start = connectionPoint(from, to)
    const end = connectionPoint(to, from)
    const curve = Math.max(40, Math.abs(end.x - start.x) * 0.32)
    return [{
      id: `${parent.id}-${node.id}`,
      path: `M ${start.x} ${start.y} C ${start.x + curve} ${start.y}, ${end.x - curve} ${end.y}, ${end.x} ${end.y}`,
    }]
  })
})

function connectionPoint(from: Point, to: Point): Point {
  const fromCenter = { x: from.x + CARD_WIDTH / 2, y: from.y + CARD_HEIGHT / 2 }
  const toCenter = { x: to.x + CARD_WIDTH / 2, y: to.y + CARD_HEIGHT / 2 }
  if (Math.abs(toCenter.x - fromCenter.x) > Math.abs(toCenter.y - fromCenter.y)) {
    return { x: from.x + (toCenter.x > fromCenter.x ? CARD_WIDTH : 0), y: fromCenter.y }
  }
  return { x: fromCenter.x, y: from.y + (toCenter.y > fromCenter.y ? CARD_HEIGHT : 0) }
}

function displayedPoint(node: CanvasNode): Point {
  return dragNodeId.value === node.id && dragPoint.value ? dragPoint.value : node
}

function nodeStyle(node: CanvasNode) {
  const point = displayedPoint(node)
  return { left: `${point.x}px`, top: `${point.y}px` }
}

function nodeLabel(node: CanvasNode) {
  const kind = t(`canvas.${node.kind}`)
  return node.text ? `${kind}: ${node.text}` : kind
}

function imageUrl(node: CanvasNode) {
  return objectUrls.get(node.id) || ''
}

function updateObjectUrls(nodes: CanvasNode[]) {
  const activeIds = new Set(nodes.filter((node) => node.image).map((node) => node.id))
  for (const [id, url] of objectUrls) {
    if (!activeIds.has(id)) {
      URL.revokeObjectURL(url)
      objectUrls.delete(id)
    }
  }
  for (const node of nodes) {
    if (node.image && !objectUrls.has(node.id)) objectUrls.set(node.id, URL.createObjectURL(node.image))
  }
}

watch(() => props.nodes, updateObjectUrls, { immediate: true })
watch(() => props.viewport, (viewport) => {
  if (dragNodeId.value || activePointers.size) return
  localViewport.value = { x: viewport.x, y: viewport.y, zoom: viewport.zoom }
}, { deep: true })

function stagePoint(event: { clientX: number; clientY: number }): Point {
  const rect = stageRef.value?.getBoundingClientRect()
  if (!rect) return { x: 0, y: 0 }
  return {
    x: (event.clientX - rect.left - localViewport.value.x) / localViewport.value.zoom,
    y: (event.clientY - rect.top - localViewport.value.y) / localViewport.value.zoom,
  }
}

function center(): Point {
  const rect = stageRef.value?.getBoundingClientRect()
  if (!rect) return { x: -CARD_WIDTH / 2, y: -CARD_HEIGHT / 2 }
  return {
    x: (rect.width / 2 - localViewport.value.x) / localViewport.value.zoom - CARD_WIDTH / 2,
    y: (rect.height / 2 - localViewport.value.y) / localViewport.value.zoom - CARD_HEIGHT / 2,
  }
}

function centerOn(node: Point) {
  const rect = stageRef.value?.getBoundingClientRect()
  if (!rect) return
  localViewport.value = {
    ...localViewport.value,
    x: rect.width / 2 - (node.x + CARD_WIDTH / 2) * localViewport.value.zoom,
    y: rect.height / 2 - (node.y + CARD_HEIGHT / 2) * localViewport.value.zoom,
  }
  viewChanged = true
  emitView()
}

function fit() {
  const rect = stageRef.value?.getBoundingClientRect()
  if (!rect || props.nodes.length === 0) {
    localViewport.value = { ...localViewport.value, x: 0, y: 0, zoom: 1 }
    viewChanged = true
    emitView()
    return
  }
  const minX = Math.min(...props.nodes.map((node) => node.x))
  const minY = Math.min(...props.nodes.map((node) => node.y))
  const maxX = Math.max(...props.nodes.map((node) => node.x + CARD_WIDTH))
  const maxY = Math.max(...props.nodes.map((node) => node.y + CARD_HEIGHT))
  const padding = 64
  const zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.min(
    (rect.width - padding * 2) / Math.max(1, maxX - minX),
    (rect.height - padding * 2) / Math.max(1, maxY - minY),
  )))
  localViewport.value = {
    zoom,
    x: (rect.width - (maxX - minX) * zoom) / 2 - minX * zoom,
    y: (rect.height - (maxY - minY) * zoom) / 2 - minY * zoom,
  }
  viewChanged = true
  emitView()
}

function zoomBy(factor: number) {
  const rect = stageRef.value?.getBoundingClientRect()
  if (!rect) return
  setZoom(localViewport.value.zoom * factor, { x: rect.width / 2, y: rect.height / 2 })
  viewChanged = true
  emitView()
}

function setZoom(nextZoom: number, screenPoint: Point) {
  const zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, nextZoom))
  const worldPoint = {
    x: (screenPoint.x - localViewport.value.x) / localViewport.value.zoom,
    y: (screenPoint.y - localViewport.value.y) / localViewport.value.zoom,
  }
  localViewport.value = {
    zoom,
    x: screenPoint.x - worldPoint.x * zoom,
    y: screenPoint.y - worldPoint.y * zoom,
  }
}

function onWheel(event: WheelEvent) {
  event.preventDefault()
  const rect = stageRef.value?.getBoundingClientRect()
  if (!rect) return
  const factor = Math.exp(-event.deltaY * 0.0015)
  setZoom(localViewport.value.zoom * factor, { x: event.clientX - rect.left, y: event.clientY - rect.top })
  viewChanged = true
  emitView()
}

function selectNode(id: string) {
  emit('select', id)
}

function onStageDoubleClick(event: MouseEvent) {
  if (event.target !== stageRef.value) return
  emit('add', stagePoint(event))
}

function onStagePointerDown(event: PointerEvent) {
  if (event.target !== stageRef.value || event.button !== 0 || activePointers.size >= 2 || dragPointer !== null) return
  activePointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
  stageRef.value?.setPointerCapture(event.pointerId)
  if (activePointers.size === 1) {
    panLast.value = { x: event.clientX, y: event.clientY }
    tapStart = panLast.value
  } else if (activePointers.size === 2) {
    tapStart = null
    panLast.value = null
    pinchStart.value = makePinchStart()
  }
}

function onStagePointerMove(event: PointerEvent) {
  if (!activePointers.has(event.pointerId)) return
  if (tapStart && Math.hypot(event.clientX - tapStart.x, event.clientY - tapStart.y) > 4) tapStart = null
  activePointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
  if (activePointers.size >= 2 && pinchStart.value) {
    updatePinch()
    return
  }
  if (activePointers.size !== 1 || !panLast.value) return
  localViewport.value = {
    ...localViewport.value,
    x: localViewport.value.x + event.clientX - panLast.value.x,
    y: localViewport.value.y + event.clientY - panLast.value.y,
  }
  panLast.value = { x: event.clientX, y: event.clientY }
  viewChanged = true
}

function onStagePointerEnd(event: PointerEvent) {
  if (!activePointers.has(event.pointerId)) return
  activePointers.delete(event.pointerId)
  if (activePointers.size === 1) panLast.value = [...activePointers.values()][0]
  if (activePointers.size < 2) pinchStart.value = null
  if (activePointers.size === 0) {
    panLast.value = null
    if (tapStart && event.type === 'pointerup') emit('select', null)
    tapStart = null
    emitView()
  }
}

function makePinchStart() {
  const points = [...activePointers.values()]
  return {
    distance: Math.max(1, Math.hypot(points[1].x - points[0].x, points[1].y - points[0].y)),
    midpoint: { x: (points[0].x + points[1].x) / 2, y: (points[0].y + points[1].y) / 2 },
    viewport: { ...localViewport.value },
  }
}

function updatePinch() {
  if (!pinchStart.value || !stageRef.value) return
  const points = [...activePointers.values()]
  const rect = stageRef.value.getBoundingClientRect()
  const distance = Math.max(1, Math.hypot(points[1].x - points[0].x, points[1].y - points[0].y))
  const midpoint = {
    x: (points[0].x + points[1].x) / 2 - rect.left,
    y: (points[0].y + points[1].y) / 2 - rect.top,
  }
  const startMidpoint = {
    x: pinchStart.value.midpoint.x - rect.left,
    y: pinchStart.value.midpoint.y - rect.top,
  }
  const zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, pinchStart.value.viewport.zoom * distance / pinchStart.value.distance))
  const worldAtStart = {
    x: (startMidpoint.x - pinchStart.value.viewport.x) / pinchStart.value.viewport.zoom,
    y: (startMidpoint.y - pinchStart.value.viewport.y) / pinchStart.value.viewport.zoom,
  }
  localViewport.value = {
    zoom,
    x: midpoint.x - worldAtStart.x * zoom,
    y: midpoint.y - worldAtStart.y * zoom,
  }
  viewChanged = true
}

function startNodeDrag(event: PointerEvent, node: CanvasNode) {
  if (event.button !== 0 || dragPointer !== null || activePointers.size) return
  dragPointer = event.pointerId
  dragNodeId.value = node.id
  dragPoint.value = { x: node.x, y: node.y }
  const point = stagePoint(event)
  dragOffset.value = { x: point.x - node.x, y: point.y - node.y }
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function onNodeDrag(event: PointerEvent) {
  if (event.pointerId !== dragPointer) return
  const point = stagePoint(event)
  dragPoint.value = { x: point.x - dragOffset.value.x, y: point.y - dragOffset.value.y }
}

function finishNodeDrag(event?: PointerEvent) {
  if (!dragNodeId.value || !dragPoint.value || (event && event.pointerId !== dragPointer)) return
  dragPointer = null
  const id = dragNodeId.value
  const point = dragPoint.value
  dragNodeId.value = null
  dragPoint.value = null
  if (event?.type === 'pointerup') emit('move', id, point.x, point.y)
}

function onNodeHandleKeydown(event: KeyboardEvent, node: CanvasNode) {
  const direction: Point = { x: 0, y: 0 }
  if (event.key === 'ArrowLeft') direction.x = -1
  if (event.key === 'ArrowRight') direction.x = 1
  if (event.key === 'ArrowUp') direction.y = -1
  if (event.key === 'ArrowDown') direction.y = 1
  if (!direction.x && !direction.y) return
  event.preventDefault()
  const step = event.shiftKey ? 50 : 10
  emit('move', node.id, node.x + direction.x * step, node.y + direction.y * step)
}

function onMinimapClick(event: MouseEvent) {
  const bounds = minimapBounds.value
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  if (!bounds) return
  const worldX = bounds.minX + ((event.clientX - rect.left) * MINIMAP_WIDTH / rect.width - MINIMAP_PADDING) / minimapScale.value
  const worldY = bounds.minY + ((event.clientY - rect.top) * MINIMAP_HEIGHT / rect.height - MINIMAP_PADDING) / minimapScale.value
  centerOn({ x: worldX - CARD_WIDTH / 2, y: worldY - CARD_HEIGHT / 2 })
}

function onDrop(event: DragEvent) {
  const files = [...(event.dataTransfer?.files || [])].filter((file) => file.type.startsWith('image/'))
  if (files.length) emit('files', files, stagePoint(event))
}

function emitView() {
  if (!viewChanged) return
  viewChanged = false
  emit('view', { ...localViewport.value })
}

function handleWindowBlur() {
  activePointers.clear()
  tapStart = null
  pinchStart.value = null
  finishNodeDrag()
  emitView()
}

onMounted(() => window.addEventListener('blur', handleWindowBlur))
onBeforeUnmount(() => {
  window.removeEventListener('blur', handleWindowBlur)
  for (const url of objectUrls.values()) URL.revokeObjectURL(url)
  objectUrls.clear()
})

defineExpose({ center, centerOn, fit, zoomBy })
</script>

<style scoped>
.canvas-minimap {
  position: absolute;
  z-index: 3;
  top: 16px;
  right: 16px;
  width: 180px;
  height: 112px;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--canvas-line);
  border-radius: 8px;
  background: var(--canvas-panel);
  cursor: pointer;
}

@media (max-width: 640px) {
  .canvas-minimap { top: 8px; right: 8px; transform: scale(.66); transform-origin: top right; }
}

.canvas-minimap:focus-visible {
  outline: 2px solid var(--canvas-accent);
  outline-offset: 3px;
}

.minimap-node,
.minimap-viewport {
  position: absolute;
  box-sizing: border-box;
  pointer-events: none;
}

.minimap-node {
  border: 1px solid var(--canvas-muted);
  border-radius: 2px;
  background: var(--canvas-soft);
}

.minimap-node.is-selected {
  border-color: var(--canvas-accent);
  background: var(--canvas-accent);
}

.minimap-viewport {
  border: 1px solid var(--canvas-accent);
  background: color-mix(in srgb, var(--canvas-accent) 12%, transparent);
}

.canvas-scene {
  position: relative;
  isolation: isolate;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background-color: var(--canvas-bg);
  background-image: radial-gradient(circle, var(--canvas-dot) 1px, transparent 1.1px);
  touch-action: none;
  -webkit-user-select: none;
  user-select: none;
  outline: none;
}

.canvas-scene:focus-visible {
  box-shadow: inset 0 0 0 2px var(--canvas-accent);
}

.canvas-world {
  position: absolute;
  left: 0;
  top: 0;
  width: 1px;
  height: 1px;
  transform-origin: 0 0;
  pointer-events: none;
}

.canvas-connections {
  position: absolute;
  z-index: 0;
  left: 0;
  top: 0;
  width: 1px;
  height: 1px;
  overflow: visible;
  pointer-events: none;
}

.connection-path {
  fill: none;
  stroke: var(--canvas-line);
  stroke-width: 2;
  vector-effect: non-scaling-stroke;
  opacity: .9;
}

.canvas-node {
  position: absolute;
  z-index: 1;
  display: flex;
  width: 220px;
  height: 164px;
  overflow: hidden;
  box-sizing: border-box;
  border: 1px solid var(--canvas-line);
  border-radius: 10px;
  background: var(--canvas-panel);
  color: var(--canvas-text);
  box-shadow: 0 8px 22px rgb(0 0 0 / 16%);
  pointer-events: auto;
  transform: translateZ(0);
  transition: border-color 120ms ease, box-shadow 120ms ease;
}

.canvas-node:hover,
.canvas-node:focus-visible,
.canvas-node.is-selected {
  border-color: var(--canvas-accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--canvas-accent) 45%, transparent), 0 10px 25px rgb(0 0 0 / 20%);
}

.canvas-node:focus-visible {
  outline: 2px solid var(--canvas-accent);
  outline-offset: 3px;
}

.node-handle {
  display: grid;
  flex: 0 0 36px;
  place-items: center;
  width: 36px;
  height: 100%;
  padding: 0;
  border: 0;
  border-right: 1px solid var(--canvas-line);
  background: var(--canvas-soft);
  color: var(--canvas-muted);
  cursor: grab;
  touch-action: none;
  -webkit-user-select: none;
  user-select: none;
}

.node-handle:active {
  cursor: grabbing;
}

.node-handle:focus-visible {
  position: relative;
  z-index: 1;
  outline: 2px solid var(--canvas-accent);
  outline-offset: -4px;
}

.node-content {
  min-width: 0;
  flex: 1;
  padding: 14px;
}

.node-thumbnail {
  width: 100%;
  height: 72px;
  margin: -2px 0 10px;
  overflow: hidden;
  border-radius: 6px;
  background: var(--canvas-soft);
}

.node-thumbnail img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.node-thumbnail-empty {
  display: grid;
  height: 100%;
  place-items: center;
  color: var(--canvas-muted);
  font-size: 24px;
}

.node-kicker {
  color: var(--canvas-accent);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: .08em;
  line-height: 1.2;
  text-transform: uppercase;
}

.node-text {
  display: -webkit-box;
  max-height: 3.8em;
  margin: 8px 0 0;
  overflow: hidden;
  color: var(--canvas-text);
  font-size: 13px;
  line-height: 1.3;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.node-status,
.node-error {
  margin: 8px 0 0;
  color: var(--canvas-muted);
  font-size: 11px;
  line-height: 1.3;
}

.node-error {
  color: var(--canvas-accent);
}

.canvas-empty {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: grid;
  place-items: center;
  pointer-events: none;
}

.canvas-empty :deep(button),
.canvas-empty :deep(a),
.canvas-empty :deep(input) {
  pointer-events: auto;
}

@media (prefers-reduced-motion: reduce) {
  .canvas-node {
    transition: none;
  }
}
</style>
