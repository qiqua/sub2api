<template>
  <BaseDialog
    :show="show"
    title="Auth Files 巡检"
    width="full"
    :close-on-click-outside="false"
    @close="handleClose"
  >
    <div class="inspection-shell">
      <section class="inspection-hero">
        <div>
          <div class="text-sm font-medium text-gray-500 dark:text-gray-400">可用账号总数</div>
          <div class="mt-2 text-4xl font-semibold text-gray-950 dark:text-white">{{ displayTotal }}</div>
          <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">会自动巡检当前筛选范围内全部符合条件账号</div>
        </div>

        <div class="min-w-0 flex-1">
          <div class="flex items-center justify-between gap-3">
            <div class="text-sm font-semibold text-gray-700 dark:text-gray-200">已巡检 / 总数</div>
            <div class="text-lg font-semibold text-gray-950 dark:text-white">
              {{ summary.checked }} / {{ displayTotal }} ({{ progressPercent }}%)
            </div>
          </div>
          <div class="mt-3 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div class="h-full rounded-full bg-emerald-500 transition-all duration-500" :style="{ width: `${progressPercent}%` }" />
          </div>
          <div class="mt-3 flex flex-wrap items-center justify-between gap-2 text-sm text-gray-500 dark:text-gray-400">
            <span>巡检时间：{{ runTimeLabel }}</span>
            <span>{{ runStateLabel }}</span>
          </div>
        </div>

        <div class="flex w-full flex-col gap-2 sm:w-44">
          <button class="btn btn-primary w-full" :disabled="loading || saving || starting || running" @click="start">
            <span v-if="starting">启动中...</span>
            <span v-else-if="running">巡检中</span>
            <span v-else>开始巡检</span>
          </button>
          <button class="btn btn-secondary w-full" :disabled="loading || stopping || !running" @click="stop">
            {{ stopping ? '取消中...' : '取消巡检' }}
          </button>
        </div>
      </section>

      <section class="grid gap-3 md:grid-cols-3 xl:grid-cols-6">
        <div v-for="card in metricCards" :key="card.key" class="metric-card">
          <div class="text-sm font-semibold text-gray-600 dark:text-gray-300">{{ card.label }}</div>
          <div :class="['mt-2 text-3xl font-bold', card.valueClass]">{{ card.value }}</div>
          <div class="mt-1 text-sm text-gray-400">{{ card.percent }}%</div>
        </div>
      </section>

      <section class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_24rem]">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">巡检范围与自动处置</div>
              <div class="panel-subtitle">
                点击开始后会自动扫完整个当前筛选范围，不受列表 50 条分页限制。
                单批数量、并发和间隔只是限速保护，用来防止 2H2G 服务器被瞬间打满。
              </div>
            </div>
            <button class="btn btn-secondary btn-sm" :disabled="saving || running" @click="saveSettings">
              {{ saving ? '保存中...' : '保存配置' }}
            </button>
          </div>

          <div class="grid gap-4 lg:grid-cols-4">
            <label class="field">
              <span>单批限速数量</span>
              <input v-model.number="form.batch_limit" type="number" min="1" :max="form.low_resource_mode ? 50 : 500" :disabled="running" />
              <small>不是只扫这一批；后台会自动继续直到扫完整个范围。低配模式最多 50。</small>
            </label>
            <label class="field">
              <span>检测并发</span>
              <input v-model.number="form.concurrency" type="number" min="1" :max="form.low_resource_mode ? 1 : 5" :disabled="running" />
              <small>2H2G 建议保持 1，低配模式会强制为 1。</small>
            </label>
            <label class="field">
              <span>限速间隔秒</span>
              <input v-model.number="form.batch_sleep_seconds" type="number" :min="form.low_resource_mode ? 30 : 1" max="3600" :disabled="running" />
              <small>每个后台小批次之间休眠，低配模式最少 30 秒。</small>
            </label>
            <label class="field">
              <span>跳过已扫小时</span>
              <input v-model.number="form.recheck_after_hours" type="number" min="1" max="2160" :disabled="running" />
              <small>默认 168 小时，扫过的下次跳过。</small>
            </label>
          </div>

          <div class="mt-4 protection-grid">
            <label class="toggle-row">
              <input v-model="form.low_resource_mode" type="checkbox" :disabled="running" />
              <span>
                <strong>低配服务器保护模式</strong>
                <small>默认开启：并发 1、单批最多 50、批次间隔至少 30 秒，适合 2H2G。</small>
              </span>
            </label>
            <label class="field">
              <span>单次最多巡检账号</span>
              <input v-model.number="form.max_accounts_per_run" type="number" min="1" max="100000" :disabled="running" />
              <small>达到后自动暂停，点开始会从 cursor 继续，不会重扫已扫账号。</small>
            </label>
            <label class="field">
              <span>连续异常批次自动暂停</span>
              <input v-model.number="form.auto_pause_consecutive_errors" type="number" min="1" max="100" :disabled="running" />
              <small>遇到 Redis/Postgres 超时或拒绝连接时保护服务器。</small>
            </label>
          </div>

          <div class="mt-4 grid gap-3 lg:grid-cols-2">
            <label class="toggle-row">
              <input v-model="form.include_unschedulable" type="checkbox" :disabled="running" />
              <span>
                <strong>包含已关闭调度账号</strong>
                <small>用于检查之前被关掉的账号是否恢复。</small>
              </span>
            </label>
            <label class="toggle-row">
              <input v-model="form.disable_quota_exhausted" type="checkbox" :disabled="running" />
              <span>
                <strong>扫出额度不足后关闭调度</strong>
                <small>额度不足归为“限流”，不算不可用。</small>
              </span>
            </label>
            <label class="toggle-row">
              <input v-model="form.restore_auto_disabled" type="checkbox" :disabled="running" />
              <span>
                <strong>额度恢复后自动开放</strong>
                <small>只恢复本巡检自动关闭的账号，不动你手动关闭的账号。</small>
              </span>
            </label>
          </div>

          <div class="mt-4">
            <div class="panel-title">删除规则</div>
            <div class="panel-subtitle">可配置“连续扫出几次 + 保留观察多久”才删除；未到规则前会显示保留或待删除。</div>
            <div class="mt-3 grid gap-3 xl:grid-cols-3">
              <div class="rule-card">
                <label class="toggle-row rule-toggle">
                  <input v-model="form.delete_auth_invalid" type="checkbox" :disabled="running" />
                  <span>
                    <strong>401 / refresh token 失效</strong>
                    <small>默认开启。明确认证失效才进入删除规则。</small>
                  </span>
                </label>
                <div class="rule-fields">
                  <label class="field">
                    <span>连续次数</span>
                    <input v-model.number="form.delete_auth_invalid_min_consecutive" type="number" min="1" max="20" :disabled="running || !form.delete_auth_invalid" />
                  </label>
                  <label class="field">
                    <span>保留小时</span>
                    <input v-model.number="form.delete_auth_invalid_after_hours" type="number" min="0" max="2160" :disabled="running || !form.delete_auth_invalid" />
                  </label>
                </div>
              </div>

              <div class="rule-card">
                <label class="toggle-row rule-toggle">
                  <input v-model="form.delete_quota_exhausted" type="checkbox" :disabled="running" />
                  <span>
                    <strong>额度不足 / usage limit</strong>
                    <small>默认只关闭调度。开启后可按规则观察后删除。</small>
                  </span>
                </label>
                <div class="rule-fields">
                  <label class="field">
                    <span>连续次数</span>
                    <input v-model.number="form.delete_quota_exhausted_min_consecutive" type="number" min="1" max="20" :disabled="running || !form.delete_quota_exhausted" />
                  </label>
                  <label class="field">
                    <span>保留小时</span>
                    <input v-model.number="form.delete_quota_exhausted_after_hours" type="number" min="0" max="2160" :disabled="running || !form.delete_quota_exhausted" />
                  </label>
                </div>
              </div>

              <div class="rule-card">
                <label class="toggle-row rule-toggle">
                  <input v-model="form.delete_payment_required" type="checkbox" :disabled="running" />
                  <span>
                    <strong>402 / payment required</strong>
                    <small>默认不删除。确认要清理欠费类账号时再开启。</small>
                  </span>
                </label>
                <div class="rule-fields">
                  <label class="field">
                    <span>连续次数</span>
                    <input v-model.number="form.delete_payment_required_min_consecutive" type="number" min="1" max="20" :disabled="running || !form.delete_payment_required" />
                  </label>
                  <label class="field">
                    <span>保留小时</span>
                    <input v-model.number="form.delete_payment_required_after_hours" type="number" min="0" max="2160" :disabled="running || !form.delete_payment_required" />
                  </label>
                </div>
              </div>
            </div>
          </div>

          <div class="mt-4 rounded-xl bg-gray-50 p-3 text-sm text-gray-600 dark:bg-dark-700/60 dark:text-gray-300">
            当前筛选：{{ filterSummary }}。后台会从上次进度继续自动推进；进度游标 cursor={{ form.cursor || 0 }}。
          </div>
        </div>

        <div class="panel">
          <div class="panel-title">自动处置结果</div>
          <div class="mt-4 grid grid-cols-2 gap-3">
            <div class="action-stat">
              <span>已保留</span>
              <strong>{{ summary.retained }}</strong>
            </div>
            <div class="action-stat">
              <span>待删除</span>
              <strong>{{ summary.pending_delete }}</strong>
            </div>
            <div class="action-stat">
              <span>已删除</span>
              <strong>{{ summary.deleted }}</strong>
            </div>
            <div class="action-stat">
              <span>已关闭</span>
              <strong>{{ summary.disabled }}</strong>
            </div>
            <div class="action-stat">
              <span>已恢复</span>
              <strong>{{ summary.restored }}</strong>
            </div>
            <div class="action-stat">
              <span>失败</span>
              <strong>{{ summary.action_failed }}</strong>
            </div>
          </div>
          <div v-if="summary.action_failed > 0" class="mt-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-200">
            有 {{ summary.action_failed }} 个账号处置失败，请在最近结果里查看原因。
          </div>
        </div>
      </section>

      <section class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_32rem]">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">最近结果</div>
              <div class="panel-subtitle">展示最新 30 条巡检结果，删除账号后仍保留本次结果记录。</div>
            </div>
          </div>

          <div class="result-list">
            <div v-if="recentResults.length === 0" class="empty-state">暂无结果，点击开始巡检后这里会刷新。</div>
            <div v-for="item in recentResults" :key="item.id || `${item.run_id}-${item.account_id}`" class="result-row">
              <div class="min-w-0">
                <div class="truncate font-semibold text-gray-900 dark:text-white">{{ item.name || item.account_id }}</div>
                <div class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ item.platform }} / {{ item.type }} · {{ item.error_code || item.message || 'ok' }}
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span :class="categoryPillClass(item.category)">{{ categoryLabel(item.category, item.http_status) }}</span>
                <span v-if="item.action !== 'none'" class="action-pill">{{ actionLabel(item.action) }}</span>
              </div>
              <div class="text-right text-xs text-gray-500 dark:text-gray-400">{{ formatTime(item.checked_at) }}</div>
            </div>
          </div>
        </div>

        <div class="panel">
          <div class="panel-title">实时日志</div>
          <div class="log-console">
            <div v-if="logs.length === 0" class="text-gray-400">等待巡检日志...</div>
            <div v-for="item in logs" :key="item.id" :class="logLineClass(item.level)">
              [{{ formatClock(item.created_at) }}] {{ item.message }}
            </div>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" @click="handleClose">关闭</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch, onUnmounted } from 'vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import type {
  AccountHealthCheckFilters,
  AccountInspectionResult,
  AccountInspectionSummary,
  AccountInspectionSettings,
  AccountInspectionStatus
} from '@/api/admin/accounts'

const props = defineProps<{
  show: boolean
  selectedIds: number[]
  filters: AccountHealthCheckFilters
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'changed'): void
}>()

const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const starting = ref(false)
const stopping = ref(false)
const status = ref<AccountInspectionStatus | null>(null)
let pollTimer: number | undefined

const form = reactive<AccountInspectionSettings>({
  enabled: false,
  filters: {},
  model_id: '',
  batch_limit: 50,
  concurrency: 1,
  batch_sleep_seconds: 30,
  recheck_after_hours: 168,
  low_resource_mode: true,
  max_accounts_per_run: 1000,
  auto_pause_consecutive_errors: 3,
  include_unschedulable: true,
  delete_auth_invalid: true,
  delete_auth_invalid_min_consecutive: 1,
  delete_auth_invalid_after_hours: 0,
  delete_quota_exhausted: false,
  delete_quota_exhausted_min_consecutive: 1,
  delete_quota_exhausted_after_hours: 168,
  delete_payment_required: false,
  delete_payment_required_min_consecutive: 1,
  delete_payment_required_after_hours: 168,
  disable_quota_exhausted: true,
  restore_auto_disabled: true,
  cursor: 0
})

const emptySummary: AccountInspectionSummary = {
  total: 0,
  checked: 0,
  available: 0,
  rate_limited: 0,
  unavailable: 0,
  quota_exhausted: 0,
  auth_401: 0,
  payment_402: 0,
  other_failure: 0,
  unknown: 0,
  deleted: 0,
  retained: 0,
  pending_delete: 0,
  disabled: 0,
  restored: 0,
  action_failed: 0,
  by_category: {}
}

const summary = computed(() => status.value?.summary ?? emptySummary)
const candidateTotal = computed(() => status.value?.candidate_total ?? 0)
const displayTotal = computed(() => summary.value.total || candidateTotal.value)
const running = computed(() => Boolean(status.value?.running))
const recentResults = computed<AccountInspectionResult[]>(() => status.value?.recent_results ?? [])
const logs = computed(() => status.value?.logs ?? [])
const progressPercent = computed(() => {
  const total = displayTotal.value
  if (!total) return 0
  return Math.min(100, Math.round((summary.value.checked / total) * 100))
})
const runStateLabel = computed(() => {
  const run = status.value?.run
  if (!run) return '尚未开始'
  if (run.status === 'running') return '巡检中'
  if (run.status === 'stopping') return '停止中'
  if (run.status === 'paused') return '已暂停，可继续'
  if (run.status === 'completed') return run.has_more ? '本批完成，可继续' : '已完成'
  if (run.status === 'stopped') return '已取消'
  if (run.status === 'failed') return '失败'
  return run.status
})
const runTimeLabel = computed(() => {
  const run = status.value?.run
  if (!run?.started_at) return '尚未开始'
  const start = new Date(run.started_at).getTime()
  const end = run.finished_at ? new Date(run.finished_at).getTime() : Date.now()
  if (!Number.isFinite(start) || !Number.isFinite(end)) return '计算中'
  const seconds = Math.max(0, Math.round((end - start) / 1000))
  if (seconds < 60) return `${seconds}s`
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
})
const metricCards = computed(() => {
  const total = displayTotal.value || 1
  const pct = (value: number) => Math.round((value / total) * 100)
  return [
    { key: 'available', label: '正常', value: summary.value.available, percent: pct(summary.value.available), valueClass: 'text-emerald-600 dark:text-emerald-300' },
    { key: 'quota', label: '达到限额', value: summary.value.quota_exhausted, percent: pct(summary.value.quota_exhausted), valueClass: 'text-amber-600 dark:text-amber-300' },
    { key: '401', label: '401', value: summary.value.auth_401, percent: pct(summary.value.auth_401), valueClass: 'text-rose-600 dark:text-rose-300' },
    { key: '402', label: '402', value: summary.value.payment_402, percent: pct(summary.value.payment_402), valueClass: 'text-orange-600 dark:text-orange-300' },
    { key: 'other', label: '其它失败', value: summary.value.other_failure, percent: pct(summary.value.other_failure), valueClass: 'text-red-600 dark:text-red-300' },
    { key: 'unknown', label: '未知', value: summary.value.unknown, percent: pct(summary.value.unknown), valueClass: 'text-gray-600 dark:text-gray-300' }
  ]
})
const filterSummary = computed(() => {
  const f = form.filters || {}
  const parts = [
    f.platform ? `平台=${f.platform}` : '',
    f.type ? `类型=${f.type}` : '',
    f.status ? `状态=${f.status}` : '',
    f.group ? `分组=${f.group}` : '',
    f.search ? `搜索=${f.search}` : '',
    f.privacy_mode ? `隐私=${f.privacy_mode}` : ''
  ].filter(Boolean)
  return parts.length > 0 ? parts.join('，') : '全部账号'
})

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      void loadStatus(true).then(() => {
        if (running.value) {
          startPolling()
        }
      })
    } else {
      stopPolling()
    }
  }
)

watch(
  () => props.filters,
  () => {
    if (!props.show || running.value) return
    applyCurrentFilters()
  },
  { deep: true }
)

onUnmounted(() => stopPolling())

const loadStatus = async (syncFilters = false) => {
  loading.value = true
  try {
    const next = await adminAPI.accounts.getInspectionStatus()
    status.value = next
    Object.assign(form, next.settings)
    if (syncFilters && !next.running) {
      applyCurrentFilters()
      const saved = await adminAPI.accounts.updateInspectionSettings(normalizedSettings())
      Object.assign(form, saved)
      const refreshed = await adminAPI.accounts.getInspectionStatus()
      status.value = refreshed
      Object.assign(form, refreshed.settings)
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载巡检状态失败'))
  } finally {
    loading.value = false
  }
}

const applyCurrentFilters = () => {
  const nextFilters = sanitizeFilters(props.filters)
  const sameFilters = filtersEqual(form.filters, nextFilters)
  form.filters = nextFilters
  form.cursor = sameFilters ? (status.value?.settings?.cursor ?? 0) : 0
}

const saveSettings = async () => {
  saving.value = true
  try {
    const saved = await adminAPI.accounts.updateInspectionSettings(normalizedSettings())
    Object.assign(form, saved)
    appStore.showSuccess('巡检配置已保存')
    await loadStatus(false)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '保存巡检配置失败'))
  } finally {
    saving.value = false
  }
}

const start = async () => {
  starting.value = true
  try {
    await adminAPI.accounts.updateInspectionSettings(normalizedSettings())
    await adminAPI.accounts.startInspection()
    appStore.showSuccess('巡检已启动')
    await loadStatus(false)
    startPolling()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '启动巡检失败'))
  } finally {
    starting.value = false
  }
}

const stop = async () => {
  stopping.value = true
  try {
    await adminAPI.accounts.stopInspection()
    appStore.showSuccess('已发送取消巡检指令')
    await loadStatus(false)
    if (running.value) {
      startPolling()
    } else {
      stopPolling()
      emit('changed')
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '取消巡检失败'))
  } finally {
    stopping.value = false
  }
}

const startPolling = () => {
  stopPolling()
  pollTimer = window.setInterval(async () => {
    try {
      const next = await adminAPI.accounts.getInspectionStatus()
      const wasRunning = running.value
      status.value = next
      if (next.running || wasRunning) {
        Object.assign(form, next.settings)
      }
      if (wasRunning && !next.running) {
        stopPolling()
        emit('changed')
      }
    } catch (error) {
      console.error('Failed to poll account inspection status:', error)
    }
  }, 2000)
}

const stopPolling = () => {
  if (pollTimer !== undefined) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
}

const normalizedSettings = (): AccountInspectionSettings => ({
  ...form,
  filters: sanitizeFilters(props.filters),
  model_id: String(form.model_id || '').trim(),
  low_resource_mode: Boolean(form.low_resource_mode),
  batch_limit: clampNumber(form.batch_limit, 1, form.low_resource_mode ? 50 : 500, 50),
  concurrency: clampNumber(form.concurrency, 1, form.low_resource_mode ? 1 : 5, 1),
  batch_sleep_seconds: clampNumber(form.batch_sleep_seconds, form.low_resource_mode ? 30 : 1, 3600, 30),
  recheck_after_hours: clampNumber(form.recheck_after_hours, 1, 2160, 168),
  max_accounts_per_run: clampNumber(form.max_accounts_per_run, 1, 100000, 1000),
  auto_pause_consecutive_errors: clampNumber(form.auto_pause_consecutive_errors, 1, 100, 3),
  include_unschedulable: Boolean(form.include_unschedulable),
  delete_auth_invalid: Boolean(form.delete_auth_invalid),
  delete_auth_invalid_min_consecutive: clampNumber(form.delete_auth_invalid_min_consecutive, 1, 20, 1),
  delete_auth_invalid_after_hours: clampNumber(form.delete_auth_invalid_after_hours, 0, 2160, 0),
  delete_quota_exhausted: Boolean(form.delete_quota_exhausted),
  delete_quota_exhausted_min_consecutive: clampNumber(form.delete_quota_exhausted_min_consecutive, 1, 20, 1),
  delete_quota_exhausted_after_hours: clampNumber(form.delete_quota_exhausted_after_hours, 0, 2160, 168),
  delete_payment_required: Boolean(form.delete_payment_required),
  delete_payment_required_min_consecutive: clampNumber(form.delete_payment_required_min_consecutive, 1, 20, 1),
  delete_payment_required_after_hours: clampNumber(form.delete_payment_required_after_hours, 0, 2160, 168),
  disable_quota_exhausted: Boolean(form.disable_quota_exhausted),
  restore_auto_disabled: Boolean(form.restore_auto_disabled),
  cursor: Number(form.cursor || 0)
})

const sanitizeFilters = (filters: AccountHealthCheckFilters): AccountInspectionSettings['filters'] => ({
  platform: filters.platform || '',
  type: filters.type || '',
  status: filters.status || '',
  group: filters.group || '',
  search: filters.search || '',
  privacy_mode: filters.privacy_mode || ''
})

const filtersEqual = (
  left: AccountInspectionSettings['filters'],
  right: AccountInspectionSettings['filters']
) => {
  return (left.platform || '') === (right.platform || '') &&
    (left.type || '') === (right.type || '') &&
    (left.status || '') === (right.status || '') &&
    (left.group || '') === (right.group || '') &&
    (left.search || '') === (right.search || '') &&
    (left.privacy_mode || '') === (right.privacy_mode || '')
}

const clampNumber = (value: number, min: number, max: number, fallback: number) => {
  if (!Number.isFinite(value)) return fallback
  return Math.min(max, Math.max(min, Math.floor(value)))
}

const handleClose = () => {
  stopPolling()
  emit('close')
}

const categoryLabel = (category: string, httpStatus?: number) => {
  if (category === 'auth_invalid' && httpStatus === 401) return '401'
  switch (category) {
    case 'available': return '正常'
    case 'rate_limited': return '限流'
    case 'quota_exhausted': return '额度不足'
    case 'auth_invalid': return '认证失效'
    case 'payment_required': return '402'
    case 'proxy_error': return '代理/网络'
    case 'model_error': return '模型错误'
    case 'upstream_error': return '上游错误'
    case 'config_error': return '配置缺失'
    case 'unknown_error': return '未知'
    default: return category || '-'
  }
}

const actionLabel = (action: string) => {
  switch (action) {
    case 'delete': return '已删除'
    case 'retain': return '已保留'
    case 'delete_pending': return '待删除'
    case 'disable': return '已关闭'
    case 'restore': return '已恢复'
    default: return action
  }
}

const categoryPillClass = (category: string) => {
  const base = 'rounded-full px-2.5 py-1 text-xs font-semibold'
  if (category === 'available') return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200`
  if (category === 'quota_exhausted' || category === 'rate_limited') return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200`
  if (category === 'auth_invalid') return `${base} bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200`
  if (category === 'payment_required') return `${base} bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200`
  return `${base} bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-200`
}

const logLineClass = (level: string) => {
  if (level === 'error') return 'text-rose-300'
  if (level === 'warn') return 'text-amber-300'
  return 'text-emerald-100'
}

const formatTime = (value?: string) => {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

const formatClock = (value?: string) => {
  if (!value) return '--:--:--'
  return new Date(value).toLocaleTimeString('zh-CN', { hour12: false })
}
</script>

<style scoped>
.inspection-shell {
  @apply space-y-5;
}

.inspection-hero {
  @apply flex flex-col gap-6 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800 lg:flex-row lg:items-center;
}

.metric-card {
  @apply rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800;
}

.panel {
  @apply rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-600 dark:bg-dark-800;
}

.panel-header {
  @apply mb-4 flex flex-wrap items-start justify-between gap-3;
}

.panel-title {
  @apply text-base font-semibold text-gray-950 dark:text-white;
}

.panel-subtitle {
  @apply mt-1 text-sm text-gray-500 dark:text-gray-400;
}

.field {
  @apply block;
}

.field span {
  @apply text-sm font-medium text-gray-700 dark:text-gray-200;
}

.field input {
  @apply mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white;
}

.field small {
  @apply mt-1 block text-xs text-gray-500 dark:text-gray-400;
}

.toggle-row {
  @apply flex items-start gap-3 rounded-xl border border-gray-200 p-3 dark:border-dark-600;
}

.toggle-row input {
  @apply mt-1 rounded border-gray-300 text-primary-600;
}

.toggle-row strong {
  @apply block text-sm text-gray-900 dark:text-white;
}

.toggle-row small {
  @apply mt-1 block text-xs text-gray-500 dark:text-gray-400;
}

.protection-grid {
  @apply grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.9fr)_minmax(0,0.9fr)];
}

.rule-card {
  @apply rounded-xl border border-gray-200 p-3 dark:border-dark-600;
}

.rule-toggle {
  @apply border-0 p-0;
}

.rule-fields {
  @apply mt-3 grid grid-cols-2 gap-3;
}

.action-stat {
  @apply rounded-xl bg-gray-50 p-3 text-center dark:bg-dark-700/60;
}

.action-stat span {
  @apply block text-xs text-gray-500 dark:text-gray-400;
}

.action-stat strong {
  @apply mt-1 block text-xl font-semibold text-gray-950 dark:text-white;
}

.result-list {
  @apply max-h-[44vh] overflow-auto rounded-xl border border-gray-100 dark:border-dark-600;
}

.result-row {
  @apply grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 dark:border-dark-600;
}

.empty-state {
  @apply px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400;
}

.action-pill {
  @apply rounded-full bg-sky-100 px-2.5 py-1 text-xs font-semibold text-sky-700 dark:bg-sky-900/30 dark:text-sky-200;
}

.log-console {
  @apply mt-4 max-h-[44vh] overflow-auto rounded-xl bg-slate-950 p-4 font-mono text-xs leading-6 shadow-inner;
}
</style>
