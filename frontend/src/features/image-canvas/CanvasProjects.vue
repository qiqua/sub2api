<template>
  <dialog ref="dialog" class="canvas-dialog" :aria-labelledby="titleId" @close="confirmDelete = false">
    <header class="dialog-head">
      <h2 :id="titleId">{{ t('canvas.projects') }}</h2>
      <form method="dialog"><button class="canvas-icon" :aria-label="t('common.close')"><Icon name="x" /></button></form>
    </header>
    <p class="canvas-note">{{ t('canvas.localHint') }}</p>
    <label class="canvas-field">{{ t('canvas.board') }}
      <select :value="board.id" :disabled="busy || locked" @change="switchBoard">
        <option v-for="item in boards" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select>
    </label>
    <form class="rename-row" @submit.prevent="rename">
      <label class="canvas-field">{{ t('canvas.boardName') }}
        <input v-model="name" required maxlength="120" :disabled="busy" @keydown.enter.stop />
      </label>
      <button type="submit" class="canvas-control" :disabled="busy || !name.trim() || name.trim() === board.name">{{ t('canvas.rename') }}</button>
    </form>
    <div class="project-actions">
      <button type="button" class="canvas-control" :disabled="busy || locked" @click="emit('create')"><Icon name="plus" size="sm" />{{ t('canvas.newBoard') }}</button>
      <button type="button" class="canvas-control" :disabled="busy || locked" @click="fileInput?.click()"><Icon name="upload" size="sm" />{{ t('canvas.importBoard') }}</button>
      <button type="button" class="canvas-control" :disabled="busy" @click="emit('export')"><Icon name="download" size="sm" />{{ t('canvas.exportBoard') }}</button>
      <input ref="fileInput" type="file" accept=".json,application/json" hidden @change="readFile" />
    </div>
    <p v-if="error" role="alert" class="canvas-error">{{ error }}</p>
    <p v-if="locked" class="canvas-note">{{ t('canvas.stayOnBoard') }}</p>
    <div class="delete-board">
      <template v-if="confirmDelete">
        <p>{{ t('canvas.confirmDelete', { name: board.name }) }}</p>
        <button ref="keep" type="button" class="canvas-control" @click="confirmDelete = false">{{ t('common.cancel') }}</button>
        <button type="button" class="canvas-control" :disabled="busy || locked" @click="confirmDelete = false; emit('remove')">{{ t('canvas.deleteBoard') }}</button>
      </template>
      <button v-else type="button" class="canvas-link" :disabled="busy || locked" @click="askDelete">{{ t('canvas.deleteBoard') }}</button>
    </div>
  </dialog>
</template>

<script setup lang="ts">
import { nextTick, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CanvasBoard } from './document'

const props = defineProps<{ board: CanvasBoard; boards: { id: string; name: string }[]; busy: boolean; locked: boolean; error: string }>()
const emit = defineEmits<{
  open: [id: string]; rename: [name: string]; create: []; import: [file: File]; export: []; remove: []; clearError: []
}>()
const { t } = useI18n()
const titleId = useId()
const dialog = ref<HTMLDialogElement>()
const keep = ref<HTMLButtonElement>()
const fileInput = ref<HTMLInputElement>()
const name = ref(props.board.name)
const confirmDelete = ref(false)
watch(() => props.board.name, value => { name.value = value })

function show() {
  name.value = props.board.name
  emit('clearError')
  dialog.value?.showModal()
}
function rename() {
  if (name.value.trim()) emit('rename', name.value.trim())
}
function switchBoard(event: Event) {
  const select = event.target as HTMLSelectElement
  const id = select.value
  select.value = props.board.id
  emit('open', id)
}
function readFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) emit('import', file)
}
async function askDelete() {
  confirmDelete.value = true
  await nextTick()
  keep.value?.focus()
}
defineExpose({ show, close: () => dialog.value?.close() })
</script>
