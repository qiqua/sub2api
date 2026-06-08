<template>
  <BaseDialog
    :show="show"
    title="批量检测账号"
    width="full"
    :close-on-click-outside="false"
    @close="handleClose"
  >
    <div class="space-y-5">
      <div class="rounded-xl border border-blue-100 bg-blue-50 p-4 text-sm text-blue-800 dark:border-blue-900/50 dark:bg-blue-900/20 dark:text-blue-100">
        后端会按并发批量检测账号可用性，并把结果分为可用、限流、不可用。检测不会自动删除账号，删除/禁用等操作需要你二次确认。
      </div>

      <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
        <div class="space-y-4 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div class="text-base font-semibold text-gray-900 dark:text-white">检测范围</div>
              <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                当前选中 {{ selectedIds.length }} 个账号，当前筛选会覆盖整个列表结果。
              </div>
            </div>
            <button class="btn btn-primary" :disabled="starting || isRunning" @click="startJob">
              <span v-if="starting">创建中...</span>
              <span v-else-if="isRunning">检测中</span>
              <span v-else>开始检测</span>
            </button>
          </div>

          <div class="grid gap-3 md:grid-cols-2">
            <label class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors"
              :class="scope === 'selected' ? 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-600'"
            >
              <input v-model="scope" type="radio" value="selected" class="mt-1" :disabled="selectedIds.length === 0 || isRunning" />
              <span>
                <span class="block text-sm font-medium text-gray-900 dark:text-white">检测选中账号</span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">适合先手动框选一批账号再处理。</span>
              </span>
            </label>
            <label class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors"
              :class="scope === 'filtered' ? 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-600'"
            >
              <input v-model="scope" type="radio" value="filtered" class="mt-1" :disabled="isRunning" />
              <span>
                <span class="block text-sm font-medium text-gray-900 dark:text-white">检测当前筛选</span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">会检测当前分组、平台、状态、搜索条件下的所有账号。</span>
              </span>
            </label>
          </div>

          <div class="grid gap-3 md:grid-cols-3">
            <label class="block">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">并发数</span>
              <input
                v-model.number="concurrency"
                type="number"
                min="1"
                max="30"
                class="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white"
                :disabled="isRunning"
              />
            </label>
            <label class="block md:col-span-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">测试模型（可选）</span>
              <input
                v-model.trim="modelId"
                type="text"
                placeholder="留空使用后端默认测试模型"
                class="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white"
                :disabled="isRunning"
              />
            </label>
          </div>

          <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="includeUnschedulable" type="checkbox" class="rounded border-gray-300 text-primary-600" :disabled="isRunning" />
            包含已禁用调度账号
          </label>
        </div>

        <div class="space-y-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
          <div class="text-sm font-semibold text-gray-900 dark:text-white">任务状态</div>
          <div class="text-2xl font-semibold text-gray-900 dark:text-white">{{ jobStatusLabel }}</div>
          <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${progressPercent}%` }"></div>
          </div>
          <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400">
            <span>已完成 {{ completedCount }}</span>
            <span>总计 {{ summary.total }}</span>
          </div>
          <button v-if="isRunning" class="btn btn-secondary w-full" @click="cancelJob">取消检测</button>
        </div>
      </div>

      <div class="grid gap-3 md:grid-cols-4">
        <div class="health-card">
          <div class="text-xs text-gray-500 dark:text-gray-400">可用</div>
          <div class="mt-1 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">{{ summary.available }}</div>
        </div>
        <div class="health-card">
          <div class="text-xs text-gray-500 dark:text-gray-400">限流</div>
          <div class="mt-1 text-2xl font-semibold text-amber-600 dark:text-amber-300">{{ summary.rate_limited }}</div>
        </div>
        <div class="health-card">
          <div class="text-xs text-gray-500 dark:text-gray-400">不可用</div>
          <div class="mt-1 text-2xl font-semibold text-rose-600 dark:text-rose-300">{{ summary.unavailable }}</div>
        </div>
        <div class="health-card">
          <div class="text-xs text-gray-500 dark:text-gray-400">待检测/检测中</div>
          <div class="mt-1 text-2xl font-semibold text-gray-700 dark:text-gray-200">{{ summary.pending + summary.checking }}</div>
        </div>
      </div>

      <div v-if="categoryRows.length > 0" class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
        <div class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">错误分类</div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="row in categoryRows"
            :key="row.category"
            class="rounded-full border px-3 py-1 text-xs transition-colors"
            :class="resultCategory === row.category ? 'border-primary-400 bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-200' : 'border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
            @click="resultCategory = resultCategory === row.category ? '' : row.category"
          >
            {{ categoryLabel(row.category) }}: {{ row.count }}
          </button>
        </div>
      </div>

      <div class="rounded-xl border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-4 dark:border-dark-600">
          <div>
            <div class="text-sm font-semibold text-gray-900 dark:text-white">检测结果</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">已选中 {{ checkedResultIds.length }} 个结果</div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <select v-model="resultStatus" class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white">
              <option value="">全部状态</option>
              <option value="available">可用</option>
              <option value="rate_limited">限流</option>
              <option value="unavailable">不可用</option>
            </select>
            <button class="btn btn-secondary btn-sm" :disabled="filteredResults.length === 0" @click="toggleAllFilteredResults">
              {{ allFilteredChecked ? '取消当前筛选' : '选择当前筛选' }}
            </button>
            <button class="btn btn-warning btn-sm" :disabled="checkedResultIds.length === 0 || actionLoading" @click="disableSelected">
              禁用调度
            </button>
            <button class="btn btn-secondary btn-sm" :disabled="checkedResultIds.length === 0 || actionLoading" @click="clearErrorSelected">
              清除错误
            </button>
            <button class="btn btn-secondary btn-sm" :disabled="checkedResultIds.length === 0 || actionLoading" @click="refreshSelected">
              刷新 token
            </button>
            <button class="btn btn-danger btn-sm" :disabled="checkedResultIds.length === 0 || actionLoading" @click="deleteSelected">
              删除
            </button>
          </div>
        </div>

        <div class="max-h-[48vh] overflow-auto">
          <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-600">
            <thead class="sticky top-0 bg-gray-50 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:bg-dark-700 dark:text-gray-300">
              <tr>
                <th class="w-10 px-4 py-3"></th>
                <th class="px-4 py-3">账号</th>
                <th class="px-4 py-3">平台/类型</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3">分类</th>
                <th class="px-4 py-3">HTTP</th>
                <th class="px-4 py-3">耗时</th>
                <th class="px-4 py-3">信息</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
              <tr v-if="filteredResults.length === 0">
                <td colspan="8" class="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  暂无结果。开始检测后这里会实时刷新。
                </td>
              </tr>
              <tr v-for="item in filteredResults" :key="item.account_id" class="hover:bg-gray-50 dark:hover:bg-dark-700/60">
                <td class="px-4 py-3">
                  <input
                    type="checkbox"
                    class="rounded border-gray-300 text-primary-600"
                    :checked="checkedResultIds.includes(item.account_id)"
                    @change="toggleResult(item.account_id)"
                  />
                </td>
                <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ item.name || item.account_id }}</td>
                <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ item.platform }} / {{ item.type }}</td>
                <td class="px-4 py-3">
                  <span :class="statusBadgeClass(item.status)">{{ statusLabel(item.status) }}</span>
                </td>
                <td class="px-4 py-3 text-gray-700 dark:text-gray-300">{{ categoryLabel(item.category) }}</td>
                <td class="px-4 py-3 text-gray-500 dark:text-gray-400">{{ item.http_status || '-' }}</td>
                <td class="px-4 py-3 text-gray-500 dark:text-gray-400">{{ typeof item.latency_ms === 'number' ? `${item.latency_ms}ms` : '-' }}</td>
                <td class="max-w-md px-4 py-3 text-gray-500 dark:text-gray-400">
                  <span class="line-clamp-2" :title="item.message || item.error_code || ''">
                    {{ item.error_code || item.message || '-' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" @click="handleClose">关闭</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type {
  AccountHealthCheckFilters,
  AccountHealthCheckJob,
  AccountHealthCheckResult
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
const scope = ref<'selected' | 'filtered'>('filtered')
const concurrency = ref(10)
const modelId = ref('')
const includeUnschedulable = ref(true)
const job = ref<AccountHealthCheckJob | null>(null)
const results = ref<AccountHealthCheckResult[]>([])
const resultStatus = ref('')
const resultCategory = ref('')
const checkedResultIds = ref<number[]>([])
const starting = ref(false)
const actionLoading = ref(false)
let pollTimer: number | undefined

const emptySummary = {
  total: 0,
  pending: 0,
  checking: 0,
  available: 0,
  rate_limited: 0,
  unavailable: 0,
  by_category: {}
}

const summary = computed(() => job.value?.summary ?? emptySummary)
const isRunning = computed(() => ['queued', 'running'].includes(job.value?.status || ''))
const completedCount = computed(() => summary.value.available + summary.value.rate_limited + summary.value.unavailable)
const progressPercent = computed(() => {
  if (!summary.value.total) return 0
  return Math.min(100, Math.round((completedCount.value / summary.value.total) * 100))
})
const jobStatusLabel = computed(() => {
  switch (job.value?.status) {
    case 'queued': return '排队中'
    case 'running': return '检测中'
    case 'completed': return '已完成'
    case 'canceled': return '已取消'
    case 'failed': return '失败'
    default: return '未开始'
  }
})
const categoryRows = computed(() => Object.entries(summary.value.by_category || {})
  .map(([category, count]) => ({ category, count: Number(count) || 0 }))
  .filter(row => row.count > 0)
  .sort((a, b) => b.count - a.count)
)
const filteredResults = computed(() => {
  return results.value.filter(item => {
    if (resultStatus.value && item.status !== resultStatus.value) return false
    if (resultCategory.value && item.category !== resultCategory.value) return false
    return true
  })
})
const allFilteredChecked = computed(() => {
  return filteredResults.value.length > 0 && filteredResults.value.every(item => checkedResultIds.value.includes(item.account_id))
})

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      scope.value = props.selectedIds.length > 0 ? 'selected' : 'filtered'
    } else {
      stopPolling()
    }
  }
)

onUnmounted(() => {
  stopPolling()
})

const startPolling = () => {
  stopPolling()
  pollTimer = window.setInterval(() => {
    refreshJob().catch((error) => {
      console.error('Failed to poll account health check job:', error)
    })
  }, 1500)
}

const stopPolling = () => {
  if (pollTimer !== undefined) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
}

const refreshJob = async () => {
  if (!job.value) return
  job.value = await adminAPI.accounts.getHealthCheckJob(job.value.id)
  await refreshResults()
  if (!isRunning.value) {
    stopPolling()
  }
}

const refreshResults = async () => {
  if (!job.value) return
  const response = await adminAPI.accounts.listHealthCheckJobResults(job.value.id)
  results.value = response.items ?? []
}

const startJob = async () => {
  if (scope.value === 'selected' && props.selectedIds.length === 0) {
    appStore.showError('请先选择账号，或改用当前筛选范围')
    return
  }
  starting.value = true
  checkedResultIds.value = []
  results.value = []
  try {
    job.value = await adminAPI.accounts.createHealthCheckJob({
      account_ids: scope.value === 'selected' ? props.selectedIds : undefined,
      filters: scope.value === 'filtered' ? props.filters : undefined,
      model_id: modelId.value || undefined,
      concurrency: concurrency.value,
      include_unschedulable: includeUnschedulable.value
    })
    await refreshResults()
    startPolling()
  } catch (error) {
    console.error('Failed to create account health check job:', error)
    appStore.showError(String(error))
  } finally {
    starting.value = false
  }
}

const cancelJob = async () => {
  if (!job.value) return
  try {
    job.value = await adminAPI.accounts.cancelHealthCheckJob(job.value.id)
    stopPolling()
  } catch (error) {
    console.error('Failed to cancel account health check job:', error)
    appStore.showError(String(error))
  }
}

const handleClose = () => {
  stopPolling()
  emit('close')
}

const toggleResult = (accountId: number) => {
  if (checkedResultIds.value.includes(accountId)) {
    checkedResultIds.value = checkedResultIds.value.filter(id => id !== accountId)
  } else {
    checkedResultIds.value = [...checkedResultIds.value, accountId]
  }
}

const toggleAllFilteredResults = () => {
  const ids = filteredResults.value.map(item => item.account_id)
  if (allFilteredChecked.value) {
    const remove = new Set(ids)
    checkedResultIds.value = checkedResultIds.value.filter(id => !remove.has(id))
  } else {
    checkedResultIds.value = Array.from(new Set([...checkedResultIds.value, ...ids]))
  }
}

const runAction = async (runner: (ids: number[]) => Promise<{ success: number; failed: number }>, successText: string) => {
  const ids = [...checkedResultIds.value]
  if (ids.length === 0) return
  actionLoading.value = true
  try {
    const result = await runner(ids)
    if (result.failed > 0) {
      appStore.showError(`操作部分成功：成功 ${result.success}，失败 ${result.failed}`)
    } else {
      appStore.showSuccess(successText.replace('{count}', String(result.success)))
      checkedResultIds.value = []
    }
    emit('changed')
  } catch (error) {
    console.error('Failed to run account health check action:', error)
    appStore.showError(String(error))
  } finally {
    actionLoading.value = false
  }
}

const disableSelected = async () => {
  await runAction(
    ids => adminAPI.accounts.bulkUpdate(ids, { schedulable: false }),
    '已禁用 {count} 个账号调度'
  )
}

const clearErrorSelected = async () => {
  await runAction(
    ids => adminAPI.accounts.batchClearError(ids),
    '已清除 {count} 个账号错误'
  )
}

const refreshSelected = async () => {
  await runAction(
    ids => adminAPI.accounts.batchRefresh(ids),
    '已刷新 {count} 个账号 token'
  )
}

const deleteSelected = async () => {
  const ids = [...checkedResultIds.value]
  if (!window.confirm(`确认删除 ${ids.length} 个账号？此操作不可恢复。`)) return
  await runAction(
    ids => adminAPI.accounts.batchDelete(ids),
    '已删除 {count} 个账号'
  )
}

const statusLabel = (status: string) => {
  switch (status) {
    case 'available': return '可用'
    case 'rate_limited': return '限流'
    case 'unavailable': return '不可用'
    case 'checking': return '检测中'
    case 'pending': return '待检测'
    default: return status || '-'
  }
}

const categoryLabel = (category: string) => {
  switch (category) {
    case 'available': return '可用'
    case 'rate_limited': return '限流'
    case 'quota_exhausted': return '额度不足'
    case 'auth_invalid': return '认证失效'
    case 'proxy_error': return '代理/网络'
    case 'model_error': return '模型错误'
    case 'upstream_error': return '上游错误'
    case 'config_error': return '配置缺失'
    case 'unknown_error': return '未知错误'
    default: return category || '-'
  }
}

const statusBadgeClass = (status: string) => {
  const base = 'inline-flex rounded-full px-2 py-0.5 text-xs font-medium'
  switch (status) {
    case 'available':
      return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300`
    case 'rate_limited':
      return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300`
    case 'unavailable':
      return `${base} bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300`
    case 'checking':
      return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300`
    default:
      return `${base} bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300`
  }
}
</script>

<style scoped>
.health-card {
  @apply rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800;
}
</style>
