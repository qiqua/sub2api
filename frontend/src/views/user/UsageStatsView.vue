<template>
  <AppLayout>
    <div class="app-page-scroll mx-auto max-w-[1050px] space-y-5">
      <header class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="page-title text-2xl font-semibold text-gray-950 dark:text-white">使用量统计</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">查看 Token 使用量和成本统计</p>
        </div>
        <button type="button" class="btn btn-secondary" @click="refreshedAt = new Date().toLocaleTimeString()">
          <Icon name="refresh" size="sm" :class="refreshedAt ? '' : 'animate-spin'" />
          <span>刷新</span>
        </button>
      </header>

      <section class="card p-4 sm:p-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
            <Icon name="filter" size="sm" class="text-primary-600" />
            筛选器
            <span class="rounded-full bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">已选{{ activeFilters }}</span>
          </div>
          <button type="button" class="text-xs font-medium text-gray-500 hover:text-primary-600" @click="resetFilters">清除筛选</button>
        </div>
        <div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <label v-for="field in filterFields" :key="field.label" class="space-y-1.5">
            <span class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ field.label }}</span>
            <select v-model="filters[field.key]" class="input w-full py-2 text-sm">
              <option v-for="option in field.options" :key="option" :value="option">{{ option }}</option>
            </select>
          </label>
          <label class="space-y-1.5">
            <span class="text-xs font-medium text-gray-500 dark:text-dark-400">从</span>
            <input v-model="filters.start" type="datetime-local" class="input w-full py-2 text-sm" />
          </label>
          <label class="space-y-1.5">
            <span class="text-xs font-medium text-gray-500 dark:text-dark-400">到</span>
            <input v-model="filters.end" type="datetime-local" class="input w-full py-2 text-sm" />
          </label>
        </div>
        <div class="mt-4 flex flex-wrap items-center gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
          <span class="mr-1 text-xs text-gray-500 dark:text-dark-400">快捷时间</span>
          <button v-for="range in ranges" :key="range" type="button" class="btn btn-secondary btn-sm" @click="selectedRange = range">{{ range }}</button>
        </div>
      </section>

      <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <article v-for="stat in stats" :key="stat.label" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ stat.label }}</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight" :class="stat.tone === 'success' ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-950 dark:text-white'">{{ stat.value }}</p>
          <p v-if="stat.detail" class="mt-2 text-xs text-gray-400 dark:text-dark-500">{{ stat.detail }}</p>
        </article>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-2 border-b border-gray-100 px-5 py-4 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">使用量明细</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">按时间或模型分组的 Token 使用量</p>
          </div>
          <div class="flex items-center gap-2">
            <select v-model="groupBy" class="input py-2 text-sm"><option>按日期</option><option>按模型</option></select>
            <label class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400"><input v-model="hourly" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" /> 小时粒度</label>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-[760px] w-full text-left text-sm">
            <thead class="bg-gray-50/80 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-400"><tr><th v-for="head in headers" :key="head" class="px-5 py-3 font-medium">{{ head }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="row in rows" :key="row.date" class="text-gray-700 dark:text-dark-200"><td class="px-5 py-3 font-medium">{{ row.date }}</td><td class="px-5 py-3">{{ row.requests }}</td><td class="px-5 py-3">{{ row.input }}</td><td class="px-5 py-3">{{ row.output }}</td><td class="px-5 py-3">{{ row.cacheRead }}</td><td class="px-5 py-3">{{ row.cacheWrite }}</td><td class="px-5 py-3">{{ row.total }}</td><td class="px-5 py-3 text-emerald-600">{{ row.rate }}</td><td class="px-5 py-3 font-medium">{{ row.cost }}</td></tr>
            </tbody>
          </table>
        </div>
        <div class="flex items-center justify-between border-t border-gray-100 px-5 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400"><span>第 1-2 条，共 2 条</span><span>1 / 1</span></div>
      </section>
      <p v-if="refreshedAt" class="text-right text-xs text-gray-400">最近刷新 {{ refreshedAt }} · {{ selectedRange }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

type FilterKey = 'key' | 'model' | 'route' | 'billing' | 'compression'
const filters = reactive<Record<FilterKey | 'start' | 'end', string>>({ key: '所有 Key', model: '所有模型', route: '所有渠道路由', billing: '全部计费来源', compression: '全部请求', start: '', end: '' })
const filterFields: Array<{ key: FilterKey; label: string; options: string[] }> = [
  { key: 'key', label: 'Key', options: ['所有 Key', '2123'] },
  { key: 'model', label: '模型', options: ['所有模型', 'gpt-5.6-sol', 'gpt-5.6-luna'] },
  { key: 'route', label: '渠道路由', options: ['所有渠道路由', 'QQ邀请通道'] },
  { key: 'billing', label: '计费来源', options: ['全部计费来源', '余额', '订阅'] },
  { key: 'compression', label: '压缩请求', options: ['全部请求', '压缩', '未压缩'] },
]
const ranges = ['最近 1 小时', '最近 6 小时', '最近 24 小时', '最近 7 天', '最近 30 天']
const selectedRange = ref('最近 30 天')
const groupBy = ref('按日期')
const hourly = ref(false)
const refreshedAt = ref('')
const activeFilters = computed(() => Object.entries(filters).filter(([key, value]) => key !== 'start' && key !== 'end' && value && !value.startsWith('所有') && !value.startsWith('全部')).length || 1)
const resetFilters = () => { filters.key = '所有 Key'; filters.model = '所有模型'; filters.route = '所有渠道路由'; filters.billing = '全部计费来源'; filters.compression = '全部请求'; filters.start = ''; filters.end = '' }
const stats = [
  { label: '总请求数', value: '477' }, { label: '输入 Tokens（去缓存）', value: '11.5592M' }, { label: '输出 Tokens', value: '277.237K' }, { label: '缓存读取', value: '69.0698M' },
  { label: '缓存写入', value: '29.06K' }, { label: '总 Token', value: '80.9353M' }, { label: '总成本', value: '$100.623729', tone: 'success' as const, detail: '余额消耗 $0.000000 · 订阅消耗 $100.623729' },
]
const headers = ['日期', '请求数', '输入（去缓存）', '输出', '缓存读', '缓存写', '总 Token', '缓存率', '成本']
const rows = [
  { date: '2026-09-19', requests: '399', input: '9.35022M', output: '219.269K', cacheRead: '59.2082M', cacheWrite: '29.06K', total: '68.8068M', rate: '86.4%', cost: '$82.910532' },
  { date: '2026-09-18', requests: '78', input: '2.20901M', output: '57.968K', cacheRead: '9.8615M', cacheWrite: '0', total: '12.1285M', rate: '81.7%', cost: '$17.713197' },
]
</script>
