<template>
  <AppLayout>
    <div class="app-page-scroll mx-auto max-w-[1050px] space-y-5">
      <header class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div><h1 class="page-title text-2xl font-semibold text-gray-950 dark:text-white">排行榜</h1><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">查看用户、模型和密钥维度的用量排行。</p></div>
        <div class="flex items-center gap-2"><select v-model="period" class="input py-2 text-sm"><option>今天</option><option>近 7 天</option><option>近 30 天</option></select><button class="btn btn-secondary" type="button"><Icon name="refresh" size="sm" /><span>刷新</span></button></div>
      </header>
      <div class="flex gap-5 overflow-x-auto border-b border-gray-200 dark:border-dark-700">
        <button v-for="tab in tabs" :key="tab" type="button" class="shrink-0 border-b-2 px-1 pb-3 text-sm font-medium" :class="activeTab === tab ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'" @click="activeTab = tab">{{ tab }}</button>
      </div>
      <div class="flex items-start gap-3 rounded-xl border border-primary-200 bg-primary-50 px-4 py-3 text-sm text-primary-800 dark:border-primary-900/50 dark:bg-primary-950/30 dark:text-primary-200"><Icon name="infoCircle" size="md" class="mt-0.5 shrink-0" /><div><p class="font-semibold">默认加入用户排行榜</p><p class="mt-1 text-xs opacity-80">排行榜参与默认开启。如需退出，请打开账户设置的安全区域并关闭“加入排行榜”。</p></div><router-link to="/profile?tab=security" class="ml-auto shrink-0 text-xs font-semibold underline">打开账户设置</router-link></div>
      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-5 py-5 dark:border-dark-700"><div class="flex items-center gap-2"><Icon name="trophy" size="md" class="text-primary-600" /><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ activeTab }}</h2></div><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">当前显示 {{ rows.length }} 条</p></div>
        <div class="grid gap-3 border-b border-gray-100 p-5 sm:grid-cols-2 lg:grid-cols-4 dark:border-dark-700">
          <div v-for="stat in summary" :key="stat.label" class="rounded-xl border border-gray-200 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-900/40"><p class="text-xs text-gray-500 dark:text-dark-400">{{ stat.label }}</p><p class="mt-2 text-xl font-semibold text-gray-950 dark:text-white">{{ stat.value }}</p><p v-if="stat.note" class="mt-1 text-xs text-gray-400">{{ stat.note }}</p></div>
        </div>
        <div class="overflow-x-auto"><table class="w-full min-w-[520px] text-left text-sm"><thead class="bg-gray-50/80 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-400"><tr><th class="px-5 py-3 font-medium">排名</th><th class="px-5 py-3 font-medium">{{ activeTab === '模型' ? '模型' : activeTab === 'API 密钥' ? '密钥' : '用户' }}</th><th class="px-5 py-3 text-right font-medium">消费</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="(row, index) in rows" :key="row.name"><td class="px-5 py-3"><span class="inline-flex min-w-8 justify-center rounded-full px-2 py-1 text-xs font-semibold" :class="index === 0 ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-200'">{{ index + 1 }}</span></td><td class="px-5 py-3 font-medium text-gray-800 dark:text-dark-100">{{ row.name }}</td><td class="px-5 py-3 text-right font-semibold text-emerald-600 dark:text-emerald-400">{{ row.cost }}</td></tr></tbody></table></div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const period = ref('今天')
const tabs = ['余额用户', '订阅用户', '模型', 'API 密钥']
const activeTab = ref(tabs[0])
const allRows = {
  '余额用户': [{ name: '10***09', cost: '$289.301848' }, { name: '94***73', cost: '$93.580423' }, { name: '77***16', cost: '$85.848854' }, { name: '25***55', cost: '$83.822871' }, { name: '10***57', cost: '$79.364422' }],
  '订阅用户': [{ name: '2123', cost: '$100.623729' }, { name: '15***24', cost: '$52.290100' }, { name: '87***41', cost: '$49.516188' }],
  模型: [{ name: 'gpt-5.6-sol', cost: '$62.183420' }, { name: 'gpt-5.6-luna', cost: '$21.504100' }, { name: 'claude-fable-5', cost: '$16.936209' }],
  'API 密钥': [{ name: 'sk-1f33****af18', cost: '$100.623729' }, { name: 'sk-9d20****c341', cost: '$28.622190' }],
} as const
const rows = ref([...allRows[activeTab.value as keyof typeof allRows]])
watch(activeTab, (value) => { rows.value = [...allRows[value as keyof typeof allRows]] })
const summary = [
  { label: '总消费', value: '$868.866827', note: '10 个用户' },
  { label: '显示条目', value: '10' },
  { label: '最高消费', value: '$289.301848' },
  { label: '我的排名', value: '-', note: '当前范围内暂无排名' },
]
</script>
