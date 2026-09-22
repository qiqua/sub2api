<template>
  <section class="request-log-shell">
    <div class="request-log-filter" :class="{ 'is-open': filtersOpen }">
      <button type="button" class="filter-trigger" @click="filtersOpen = !filtersOpen">
        <Icon name="filter" size="sm" />
        <span>{{ t('usage.requestLog.filters') }}</span>
        <span class="filter-count">{{ t('usage.requestLog.selected', { count: activeFilterCount }) }}</span>
        <Icon name="chevronDown" size="sm" :class="filtersOpen ? 'rotate-180' : ''" />
      </button>
      <div v-if="filtersOpen" class="filter-fields">
        <slot name="filters" />
      </div>
    </div>

    <div class="request-log-card">
      <div class="request-log-heading">
        <div>
          <h2>{{ t('usage.requestLog.title') }}</h2>
          <p>{{ t('usage.requestLog.range', { start: rangeStart, end: rangeEnd, total }) }}</p>
        </div>
        <div class="heading-actions">
          <button type="button" class="outline-action" @click="$emit('reset')">{{ t('common.reset') }}</button>
          <button type="button" class="outline-action primary" :disabled="loading" @click="$emit('refresh')">
            <Icon name="refresh" size="sm" />{{ t('common.refresh') }}
          </button>
        </div>
      </div>

      <div class="request-log-table-wrap">
        <table class="request-log-table">
          <thead>
            <tr>
              <th>
                <button type="button" class="sort-button" @click="sort('created_at')">
                  {{ t('usage.time') }} <span class="sort-arrow">{{ sortOrder === 'desc' ? '↓' : '↑' }}</span>
                </button>
              </th>
              <th>{{ t('usage.apiKeyFilter') }}</th><th>{{ t('usage.requestLog.requestId') }}</th><th>{{ t('usage.model') }}</th><th>{{ t('usage.channel') }}</th><th>{{ t('usage.reasoningEffort') }}</th>
              <th>{{ t('usage.requestLog.method') }}</th><th>{{ t('usage.requestLog.status') }}</th><th>{{ t('usage.requestLog.attempts') }}</th><th>{{ t('usage.latencyFirstToken') }}</th><th>TPS</th><th>{{ t('usage.duration') }}</th>
              <th>{{ t('usage.in') }}</th><th>{{ t('usage.out') }}</th><th>{{ t('usage.cacheRead') }}</th><th>{{ t('usage.cacheHitRate') }}</th><th>{{ t('usage.cacheWrite') }}</th><th>{{ t('usage.cost') }}</th><th>{{ t('usage.specialBilling') }}</th><th>{{ t('usage.rate') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading"><td colspan="20" class="table-empty">{{ t('common.loading') }}</td></tr>
            <tr v-else-if="!data.length"><td colspan="20" class="table-empty">{{ t('usage.requestLog.empty') }}</td></tr>
            <tr v-for="row in data" v-else :key="row.id">
              <td class="time-cell">{{ formatDate(row.created_at) }}</td>
              <td class="key-cell">{{ keyName(row) }}</td>
              <td class="request-id" :title="row.request_id">{{ shortId(row.request_id) }}</td>
              <td class="model-cell">{{ row.model || '-' }}</td>
              <td><span class="channel-badge"><span class="channel-dot">◉</span>{{ groupName(row) }}</span><span :class="billingBadgeClass(row)">{{ billingLabel(row) }}</span></td>
              <td><span class="effort-badge">{{ formatReasoningEffort(row.reasoning_effort) }}</span></td>
              <td><span class="method-badge">{{ methodLabel(row) }}</span></td>
              <td><span class="status-badge" :class="statusBadgeClass(row)">{{ statusLabel(row) }}</span></td>
              <td><span class="attempt-badge">{{ attemptLabel(row) }}</span></td>
              <td :class="latencyClass(row.first_token_ms)">{{ seconds(row.first_token_ms) }}</td>
              <td class="metric-cell">{{ tps(row) }}</td>
              <td :class="latencyClass(row.duration_ms)">{{ seconds(row.duration_ms) }}</td>
              <td class="numeric-cell">{{ compact(row.input_tokens) }}</td>
              <td class="numeric-cell">{{ compact(row.output_tokens) }}</td>
              <td class="numeric-cell">{{ compact(row.cache_read_tokens) }}</td>
              <td class="cache-rate">{{ cacheRate(row) }}</td>
              <td class="numeric-cell">{{ compact(row.cache_creation_tokens) }}</td>
              <td class="cost-cell"><span class="cost-badge">${{ Number(row.actual_cost || 0).toFixed(6) }}</span></td>
              <td class="muted-cell">{{ row.long_context_billing_applied ? t('usage.requestLog.longContext') : '-' }}</td>
              <td><span class="multiplier-badge">{{ Number(row.rate_multiplier || 1).toFixed(2) }}x</span></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="request-log-footer">
        <div class="page-size-control">
          <span>{{ t('usage.requestLog.page', { page }) }}</span>
          <span>{{ t('usage.requestLog.perPage') }}</span>
          <div class="page-size-options" :aria-label="t('usage.requestLog.perPage')">
            <button
              v-for="size in pageSizes"
              :key="size"
              type="button"
              :class="{ active: pageSize === size }"
              @click="$emit('update:pageSize', size)"
            >
              {{ size }}
            </button>
          </div>
          <input
            class="page-size-slider"
            type="range"
            min="1"
            max="100"
            step="1"
            :value="pageSize"
            :aria-label="t('usage.requestLog.perPage')"
            @change="updatePageSizeFromSlider"
          />
          <span class="page-size-value">{{ pageSize }}</span>
        </div>
        <div class="pager">
          <button type="button" :disabled="page <= 1" @click="$emit('update:page', page - 1)">‹ {{ t('usage.requestLog.previous') }}</button>
          <template v-for="item in pageItems" :key="item">
            <span v-if="typeof item === 'string'" class="pager-ellipsis">…</span>
            <button v-else type="button" :class="{ active: item === page }" @click="$emit('update:page', item)">{{ item }}</button>
          </template>
          <button type="button" :disabled="page >= pages" @click="$emit('update:page', page + 1)">{{ t('usage.requestLog.next') }} ›</button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UsageLog } from '@/types'
import { formatReasoningEffort } from '@/utils/format'

type RequestLogRow = UsageLog & {
  http_method?: string | null
  method?: string | null
  status_code?: number | null
  attempt_count?: number | null
}

const { t } = useI18n()

const props = withDefaults(defineProps<{
  data: UsageLog[]
  loading?: boolean
  total?: number
  page?: number
  pageSize?: number
  activeFilterCount?: number
  apiKeyNames?: Record<number, string>
  groupNames?: Record<number, string>
}>(), {
  loading: false,
  total: 0,
  page: 1,
  pageSize: 20,
  activeFilterCount: 1,
  apiKeyNames: () => ({}),
  groupNames: () => ({}),
})
const emit = defineEmits<{ (e: 'sort', key: string, order: 'asc' | 'desc'): void; (e: 'refresh'): void; (e: 'reset'): void; (e: 'update:page', page: number): void; (e: 'update:pageSize', size: number): void }>()
const filtersOpen = ref(false)
const sortOrder = ref<'asc' | 'desc'>('desc')
const pageSizes = [10, 20, 50, 100]
const pages = computed(() => Math.max(1, Math.ceil((props.total || 0) / (props.pageSize || 20))))
const pageItems = computed<Array<number | string>>(() => {
  const last = pages.value
  if (last <= 5) return Array.from({ length: last }, (_, i) => i + 1)
  if (props.page <= 3) return [1, 2, 3, 'ellipsis-right', last]
  if (props.page >= last - 2) return [1, 'ellipsis-left', last - 2, last - 1, last]
  return [1, 'ellipsis-left', props.page, 'ellipsis-right', last]
})
const rangeStart = computed(() => props.total ? (props.page - 1) * props.pageSize + 1 : 0)
const rangeEnd = computed(() => Math.min(props.page * props.pageSize, props.total))
const sort = (key: string) => {
  sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'
  emit('sort', key, sortOrder.value)
}
const updatePageSizeFromSlider = (event: Event) => {
  const value = Number((event.target as HTMLInputElement).value)
  if (Number.isFinite(value)) emit('update:pageSize', value)
}
function formatDate(value: string) { const d = new Date(value); return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}` }
function shortId(value: string) { return value ? `${value.slice(0, 10)}...` : '-' }
function compact(value: number | null | undefined) { const n = Number(value || 0); return n >= 1000 ? `${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}K` : String(n) }
function seconds(value: number | null | undefined) { return value == null ? '-' : `${(value / 1000).toFixed(2)}s` }
function tps(row: UsageLog) { const duration = (Number(row.duration_ms || 0) - Number(row.first_token_ms || 0)) / 1000; return duration > 0 ? `${(Number(row.output_tokens || 0) / duration).toFixed(1)}/s` : '-' }
function cacheRate(row: UsageLog) { const input = Number(row.input_tokens || 0); const read = Number(row.cache_read_tokens || 0); return input + read ? `${((read / (input + read)) * 100).toFixed(1)}%` : '0.0%' }
function latencyClass(value: number | null | undefined) { return Number(value || 0) > 10000 ? 'latency-slow' : Number(value || 0) > 5000 ? 'latency-warn' : 'latency-good' }
function keyName(row: UsageLog) { return row.api_key?.name || props.apiKeyNames[row.api_key_id] || row.api_key_id || '-' }
function groupName(row: UsageLog) { return row.group?.name || (row.group_id != null ? props.groupNames[row.group_id] : '') || '-' }
function billingLabel(row: UsageLog) { return row.billing_type === 1 || row.subscription_id != null ? t('usage.requestLog.subscription') : t('usage.requestLog.balance') }
function billingBadgeClass(row: UsageLog) { return row.billing_type === 1 || row.subscription_id != null ? 'subscription-badge' : 'balance-badge' }
function requestMeta(row: UsageLog) { return row as RequestLogRow }
function methodLabel(row: UsageLog) { return requestMeta(row).http_method || requestMeta(row).method || '-' }
function statusLabel(row: UsageLog) { return requestMeta(row).status_code ?? '-' }
function statusBadgeClass(row: UsageLog) {
  const status = requestMeta(row).status_code
  if (status == null) return 'is-unknown'
  if (status >= 200 && status < 300) return 'is-success'
  if (status >= 400 && status < 500) return 'is-warning'
  return 'is-error'
}
function attemptLabel(row: UsageLog) {
  const count = requestMeta(row).attempt_count
  return count == null ? '-' : t('usage.requestLog.attemptCount', { count })
}
</script>

<style scoped>
.request-log-shell { color: #12213d; }
.request-log-filter, .request-log-card { border: 1px solid #dfe7f2; border-radius: 14px; background: #fff; box-shadow: 0 2px 6px rgba(30, 65, 110, .04); }
.request-log-card { min-width: 0; overflow: hidden; }
.request-log-filter { margin-bottom: 14px; overflow: hidden; }
.filter-trigger { display: flex; width: 100%; align-items: center; gap: 8px; padding: 14px 18px; color: #243453; font-weight: 600; font-size: 13px; background: transparent; border: 0; cursor: pointer; }
.filter-count { padding: 2px 8px; border-radius: 6px; color: #2664ea; background: #edf3ff; font-size: 12px; }
.filter-fields { padding: 0 18px 16px; border-top: 1px solid #edf1f7; }
.request-log-heading { display: flex; align-items: center; justify-content: space-between; padding: 18px 24px 14px; }
.request-log-heading h2 { margin: 0; font-size: 19px; font-weight: 700; }
.request-log-heading p { margin: 4px 0 0; color: #69809f; font-size: 13px; }
.heading-actions { display: flex; gap: 9px; }
.outline-action { display: inline-flex; align-items: center; gap: 5px; border: 1px solid #d8e1ed; border-radius: 8px; padding: 7px 13px; color: #263750; background: white; font-size: 13px; white-space: nowrap; cursor: pointer; }
.outline-action.primary { border-color: #9dbdff; color: #2664ea; }
.outline-action:disabled { opacity: .5; cursor: wait; }
.request-log-table-wrap { overflow: auto; border-top: 1px solid #e5ebf3; }
.request-log-table { width: 100%; min-width: 1500px; border-collapse: collapse; font-size: 12px; white-space: nowrap; }
.request-log-table th { position: sticky; top: 0; z-index: 1; padding: 5px 10px; color: #667b98; background: #f9fbfe; text-align: left; font-weight: 600; border-bottom: 1px solid #e5ebf3; }
.request-log-table td { padding: 5px 10px; border-bottom: 1px solid #edf1f6; color: #183154; vertical-align: middle; }
.request-log-table tbody tr:hover { background: #f8fbff; }
.sort-button { display: inline-flex; align-items: center; gap: 3px; border: 0; padding: 0; color: inherit; background: transparent; font: inherit; cursor: pointer; }.sort-arrow { color: #1f65e8; font-size: 13px; }
.time-cell, .request-id { color: #284a79 !important; }.request-id { max-width: 104px; overflow: hidden; text-overflow: ellipsis; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.key-cell, .model-cell { font-weight: 600; }.model-cell { max-width: 130px; overflow: hidden; text-overflow: ellipsis; }
.channel-badge { display: inline-flex; align-items: center; gap: 4px; padding: 3px 7px; border-radius: 6px; color: #17894e; background: #eaf9f0; font-size: 11px; }.channel-dot { color: #20a764; }
.subscription-badge { margin-left: 4px; padding: 2px 4px; border-radius: 4px; color: #7446e5; background: #f0eaff; font-size: 10px; }
.balance-badge { margin-left: 4px; padding: 2px 4px; border-radius: 4px; color: #2563eb; background: #eaf2ff; font-size: 10px; }
.effort-badge, .method-badge, .attempt-badge { display: inline-flex; padding: 4px 8px; border-radius: 6px; color: #23324a; background: #f1f4f8; font-weight: 600; }.method-badge { border: 1px solid #d7dfeb; background: #fff; }.attempt-badge { background: #f6f7fa; }
.status-badge { display: inline-flex; padding: 4px 8px; border-radius: 5px; color: #fff; font-weight: 700; }.status-badge.is-success { background: #18ae72; }.status-badge.is-warning { background: #d99a20; }.status-badge.is-error { background: #dc5252; }.status-badge.is-unknown { color: #73849c; background: #eef2f6; }.metric-cell, .numeric-cell { color: #183154; }.cache-rate { color: #138bd4 !important; text-decoration: underline dotted; }.cost-cell { color: #1e304e !important; font-weight: 700; }.cost-badge { display: inline-flex; padding: 3px 7px; border: 1px solid #d5dfec; border-radius: 6px; background: #fff; }.muted-cell { color: #8da0bb !important; }.multiplier-badge { display: inline-flex; padding: 3px 7px; border: 1px solid #9ee6c3; border-radius: 5px; color: #179257; background: #ecfff5; font-size: 11px; }.latency-good { color: #0e9a68 !important; }.latency-warn { color: #dd8a1a !important; }.latency-slow { color: #ec6c47 !important; }.table-empty { padding: 42px !important; color: #7890ad !important; text-align: center; }
.request-log-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 18px; color: #7085a0; font-size: 12px; }.page-size-control, .page-size-options, .pager { display: flex; align-items: center; gap: 6px; }.page-size-options { gap: 0; overflow: hidden; border: 1px solid #dce5f0; border-radius: 7px; }.page-size-control button, .pager button { border: 0; border-radius: 6px; padding: 5px 8px; color: #4d6687; background: transparent; cursor: pointer; }.page-size-options button { border-radius: 0; }.page-size-options button.active { color: #2563eb; background: #edf4ff; box-shadow: inset 0 0 0 1px #bcd0f8; }.page-size-slider { width: 82px; height: 16px; margin-left: 6px; accent-color: #2563eb; cursor: pointer; }.page-size-value { min-width: 20px; color: #243b5a; font-weight: 700; }.pager button.active { color: #fff; background: #2563eb; }.pager button:disabled { opacity: .35; cursor: default; }.pager button { border: 1px solid #dce5f0; background: #fff; }
.pager-ellipsis { min-width: 18px; text-align: center; }
@media (min-width: 701px) { .request-log-table-wrap { max-height: calc(100dvh - 320px); } }
@media (max-width: 700px) { .request-log-heading { padding: 15px; }.request-log-heading h2 { font-size: 17px; }.request-log-footer { align-items: flex-start; flex-direction: column; }.page-size-control { max-width: 100%; flex-wrap: wrap; }.page-size-slider { flex: 1 1 70px; }.pager { width: 100%; justify-content: flex-end; }.request-log-table { min-width: 1300px; } }
@media (max-width: 420px) { .request-log-heading { align-items: flex-start; flex-direction: column; gap: 12px; }.request-log-heading p { line-height: 1.45; }.heading-actions { width: 100%; justify-content: flex-end; } }
</style>
