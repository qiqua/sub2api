<template>
  <aside class="canvas-inspector" :aria-label="t('canvas.inspector')">
    <header class="inspector-head">
      <h2><Icon :name="node.kind === 'prompt' ? 'edit' : 'image'" size="md" />{{ t(node.kind === 'prompt' ? 'canvas.promptNode' : 'canvas.imageNode') }}</h2>
      <button type="button" class="canvas-icon" :aria-label="t('common.close')" @click="emit('close')"><Icon name="x" /></button>
    </header>
    <form v-if="node.kind === 'prompt'" class="inspector-body" @submit.prevent="emit('generate')">
      <label class="canvas-field">{{ t('canvas.prompt') }}
        <textarea :value="node.text" rows="5" maxlength="32000" :placeholder="t('canvas.promptHint')" @input="emit('text', ($event.target as HTMLTextAreaElement).value)" @blur="emit('finishEdit')" />
      </label>
      <label class="canvas-field">{{ t('canvas.reference') }}
        <select :value="node.parentId ?? ''" @change="emit('reference', ($event.target as HTMLSelectElement).value)">
          <option value="">{{ t('canvas.noReference') }}</option>
          <option v-for="item in references" :key="item.id" :value="item.id">{{ item.text.slice(0, 64) || t('canvas.imageNode') }}</option>
        </select>
      </label>
      <div class="generation-fields">
        <p v-if="loadingKeys" role="status" class="canvas-note">{{ t('canvas.loadingKeys') }}</p>
        <template v-else-if="!keys.length">
          <p class="canvas-note">{{ t(keyError ? 'canvas.keyError' : 'canvas.noKey') }}</p>
          <div class="inline-actions"><RouterLink class="canvas-link" to="/keys">{{ t('nav.apiKeys') }}</RouterLink><button type="button" class="canvas-link" @click="emit('reloadKeys')">{{ t('canvas.refresh') }}</button></div>
        </template>
        <template v-else>
          <label class="canvas-field">{{ t('canvas.billingKey') }}
            <select v-model.number="keyId"><option v-for="entry in keys" :key="entry.id" :value="entry.id">{{ entry.name || `#${entry.id}` }} · {{ entry.group?.name }}</option></select>
          </label>
          <label class="canvas-field">{{ t('canvas.model') }}
            <select v-if="batch" v-model="model" :disabled="loadingModels || !models.length"><option v-for="name in models" :key="name" :value="name">{{ name }}</option></select>
            <input v-else v-model="model" :list="modelListId" :disabled="loadingModels" maxlength="256" :placeholder="t('canvas.exactModel')" />
          </label>
          <datalist v-if="!batch" :id="modelListId"><option v-for="name in models" :key="name" :value="name" /></datalist>
          <p v-if="loadingModels" role="status" class="canvas-note">{{ t('canvas.loadingModels') }}</p>
          <p v-else-if="modelError || !models.length" class="canvas-note">{{ t(batch ? 'canvas.noModels' : 'canvas.exactModel') }} <button type="button" class="canvas-link" @click="emit('reloadModels')">{{ t('canvas.refresh') }}</button></p>
          <label class="canvas-field">{{ t('canvas.aspect') }}
            <select v-model="aspectRatio"><option value="1:1">{{ t('canvas.square') }} · 1:1</option><option value="3:2">{{ t('canvas.wide') }} · 3:2</option><option value="2:3">{{ t('canvas.tall') }} · 2:3</option></select>
          </label>
          <button type="submit" class="canvas-control canvas-primary generate-button" :disabled="generating || !node.text.trim() || !model.trim() || loadingModels">
            <Icon name="play" size="sm" />{{ t(generating ? 'canvas.generating' : 'canvas.generate') }}
          </button>
          <p class="canvas-note">{{ t(batch ? 'canvas.batchBilling' : 'canvas.billing') }}</p>
        </template>
      </div>
    </form>
    <div v-else class="inspector-body">
      <template v-if="node.image">
        <img :src="imageUrl" :alt="node.text || t('canvas.imageNode')" class="inspector-image" width="360" height="240" />
        <div class="inline-actions">
          <button type="button" class="canvas-control canvas-primary" @click="emit('branch')"><Icon name="link" size="sm" />{{ t('canvas.derive') }}</button>
          <button type="button" class="canvas-control" @click="download"><Icon name="download" size="sm" />{{ t('canvas.download') }}</button>
        </div>
      </template>
      <template v-else>
        <p v-if="node.error" role="alert" class="canvas-error">{{ node.error }}</p>
        <p v-else role="status" class="canvas-note">{{ t(node.job ? 'canvas.jobRunning' : generating ? 'canvas.requestRunning' : node.intent ? (requestMissing ? 'canvas.submitMissing' : 'canvas.requestUnknown') : 'canvas.noResult') }}</p>
        <p v-if="pollError" role="alert" class="canvas-error">{{ pollError }}</p>
        <button v-if="(node.job || node.intent) && !hasOwner" type="button" class="canvas-control" :disabled="loadingKeys" @click="emit('reloadKeys')">{{ t('canvas.refresh') }}</button>
        <template v-else-if="node.job || node.intent">
          <button type="button" class="canvas-control" :disabled="generating" @click="emit('poll')"><Icon name="refresh" size="sm" />{{ t(node.job ? 'canvas.checkJob' : 'canvas.checkSubmission') }}</button>
          <button v-if="node.intent && requestMissing && canRetry" type="button" class="canvas-control" :disabled="generating" @click="emit('retry')">{{ t('canvas.retryRequest') }}</button>
        </template>
        <p v-else-if="!generating" class="canvas-note">{{ t('canvas.checkUsage') }} <RouterLink to="/usage" class="canvas-link">{{ t('nav.usage') }}</RouterLink></p>
      </template>
      <div v-if="node.text" class="canvas-field"><strong>{{ t(node.model ? 'canvas.originalPrompt' : 'canvas.fileName') }}</strong><p class="image-caption">{{ node.text }}</p></div>
      <div v-if="node.model" class="canvas-field"><strong>{{ t('canvas.model') }}</strong><p>{{ node.model }}</p></div>
      <button v-if="node.model && node.text && !generating && !node.job && !node.intent" class="canvas-link" type="button" @click="emit('reuse')">{{ t('canvas.reusePrompt') }}</button>
    </div>
    <footer class="inspector-foot"><button type="button" class="canvas-link" :disabled="generating" @click="emit('remove')"><Icon name="trash" size="sm" />{{ t('canvas.removeNode') }}</button></footer>
  </aside>
</template>

<script setup lang="ts">
import { computed, toRef, useId } from 'vue'
import { useObjectUrl } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { saveBlob } from '@/api/batchImage'
import type { ApiKey } from '@/types'
import type { CanvasNode } from './document'

const props = defineProps<{
  node: CanvasNode; references: CanvasNode[]; keys: ApiKey[]; models: string[]; loadingKeys: boolean;
  loadingModels: boolean; keyError: boolean; modelError: boolean; generating: boolean; pollError: string
  hasOwner: boolean; requestMissing: boolean; canRetry: boolean
}>()
const emit = defineEmits<{
  close: []; text: [text: string]; finishEdit: []; reference: [id: string]; generate: []; branch: []; poll: []; retry: [];
  reuse: []; remove: []; reloadKeys: []; reloadModels: []
}>()
const keyId = defineModel<number | null>('keyId', { required: true })
const model = defineModel<string>('model', { required: true })
const aspectRatio = defineModel<'1:1' | '3:2' | '2:3'>('aspectRatio', { required: true })
const { t } = useI18n()
const modelListId = useId()
const imageUrl = useObjectUrl(toRef(() => props.node.image))
const batch = computed(() => props.keys.find(k => k.id === keyId.value)?.group?.platform === 'gemini')
function download() {
  const image = props.node.image
  if (image) saveBlob(image, `canvas-${props.node.id}.${image.type === 'image/jpeg' ? 'jpg' : image.type.split('/')[1]}`)
}
</script>
