<template>
  <AppLayout>
    <div class="app-page-scroll mx-auto max-w-[1050px] space-y-5">
      <header><h1 class="page-title text-2xl font-semibold text-gray-950 dark:text-white">路由设置</h1><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">管理智能分组路由、模型定向规则与请求模型名称映射。</p></header>
      <section class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-5 sm:flex-row sm:items-start sm:justify-between dark:border-dark-700"><div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">智能路由</h2><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">选择同一平台的真实分组，系统会自动跳过失效上游。</p></div><div class="flex gap-2"><button class="btn btn-secondary" type="button" @click="routeAdded = true"><Icon name="plus" size="sm" /><span>添加智能路由</span></button><button class="btn btn-primary" type="button" :disabled="!routeAdded"><Icon name="check" size="sm" /><span>保存智能路由</span></button></div></div>
        <div class="p-5">
          <div class="rounded-xl border border-gray-200 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-900/40"><div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><div><p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前智能路由</p><p class="mt-1 text-sm text-gray-600 dark:text-dark-300">每条路由可绑定到不同的 API Key；使用平台和编号区分。</p></div><select class="input w-full py-2 text-sm sm:w-60"><option>选择智能路由</option><option v-if="routeAdded">新建路由 1</option></select></div></div>
          <div v-if="!routeAdded" class="mt-4 rounded-xl border border-dashed border-gray-300 px-5 py-10 text-center dark:border-dark-600"><p class="text-sm font-medium text-gray-700 dark:text-dark-200">还没有智能路由</p><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">创建一条智能路由并选择至少两个同平台分组。</p><button class="btn btn-secondary mt-4" type="button" @click="routeAdded = true"><Icon name="plus" size="sm" />添加智能路由</button></div>
          <template v-else>
            <div class="mt-5"><h3 class="text-base font-semibold text-gray-900 dark:text-white">路由策略</h3><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">没有命中模型规则或规则目标不可用时，按这里的策略选择分组。</p><div class="mt-3 grid gap-3 md:grid-cols-3"><label v-for="policy in policies" :key="policy.value" class="relative cursor-pointer rounded-xl border p-4 transition" :class="selectedPolicy === policy.value ? 'border-primary-500 bg-primary-50/60 dark:border-primary-500 dark:bg-primary-950/20' : 'border-gray-200 dark:border-dark-700'"><input v-model="selectedPolicy" type="radio" name="route-policy" :value="policy.value" class="sr-only" /><span class="text-sm font-semibold text-gray-900 dark:text-white">{{ policy.label }}</span><span class="mt-1 block text-xs leading-5 text-gray-500 dark:text-dark-400">{{ policy.description }}</span><Icon v-if="selectedPolicy === policy.value" name="checkCircle" size="sm" class="absolute right-3 top-3 text-primary-600" /></label></div></div>
            <div class="mt-5"><div class="flex items-center justify-between"><div><h3 class="text-base font-semibold text-gray-900 dark:text-white">参与分组</h3><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">手动顺序策略下会严格按此优先级尝试。</p></div><button type="button" class="btn btn-secondary btn-sm" @click="groups.push(`新分组 ${groups.length + 1}`)"><Icon name="plus" size="sm" />添加参与分组</button></div><div class="mt-3 flex flex-wrap gap-2"><span v-for="(group, index) in groups" :key="group" class="inline-flex items-center gap-2 rounded-full border border-primary-200 bg-primary-50 px-3 py-1.5 text-xs font-medium text-primary-700 dark:border-primary-900/50 dark:bg-primary-950/30 dark:text-primary-300"><span class="text-primary-400">{{ index + 1 }}</span>{{ group }}</span></div></div>
          </template>
        </div>
      </section>
      <section class="card overflow-hidden"><div class="border-b border-gray-100 px-5 py-5 dark:border-dark-700"><h2 class="text-lg font-semibold text-gray-900 dark:text-white">模型分组规则</h2><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">匹配模型映射后的名称，并将命中的请求优先交给指定成员分组。</p></div><div class="p-5"><div v-if="rules.length" class="space-y-2"><div v-for="rule in rules" :key="rule.model" class="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"><span class="font-mono">{{ rule.model }}</span><span class="text-gray-500">{{ rule.group }}</span></div></div><div v-else class="rounded-xl border border-dashed border-gray-300 px-5 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">暂无模型分组规则<button type="button" class="btn btn-secondary btn-sm ml-3" @click="rules.push({ model: 'gpt-*', group: '稳定号池' })"><Icon name="plus" size="sm" />添加模型规则</button></div></div></section>
      <section class="card overflow-hidden"><div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-5 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700"><div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">模型映射</h2><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">把客户端提交的模型名转换为实际请求的目标模型。</p></div><div class="flex items-center gap-3 text-sm text-gray-600 dark:text-dark-300"><span>启用路由映射</span><button type="button" role="switch" :aria-checked="mappingEnabled" class="relative h-6 w-11 rounded-full transition" :class="mappingEnabled ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'" @click="mappingEnabled = !mappingEnabled"><span class="absolute top-1 h-4 w-4 rounded-full bg-white shadow transition" :class="mappingEnabled ? 'left-6' : 'left-1'" /></button></div></div><div class="p-5"><div class="flex justify-end"><button type="button" class="btn btn-secondary btn-sm" @click="mappings.push({ from: 'client-model', to: 'gpt-5.6-sol' })"><Icon name="plus" size="sm" />添加映射</button></div><div v-if="mappings.length" class="mt-3 space-y-2"><div v-for="mapping in mappings" :key="mapping.from" class="grid gap-2 rounded-lg border border-gray-200 p-3 text-sm sm:grid-cols-[1fr_auto_1fr] sm:items-center dark:border-dark-700"><code class="rounded bg-gray-50 px-2 py-1 dark:bg-dark-900">{{ mapping.from }}</code><span class="text-center text-gray-400">→</span><code class="rounded bg-gray-50 px-2 py-1 dark:bg-dark-900">{{ mapping.to }}</code></div></div><p v-else class="mt-4 text-center text-sm text-gray-500 dark:text-dark-400">暂无模型映射</p></div></section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const routeAdded = ref(false)
const selectedPolicy = ref('stable')
const mappingEnabled = ref(false)
const groups = ref(['GPT 稳定号池', 'GPT 限时特价'])
const policies = [
  { value: 'stable', label: '稳定性优先', description: '优先选择当前健康可调度容量更充足的分组。' },
  { value: 'price', label: '价格优先', description: '在健康候选中按有效计费倍率从低到高选择。' },
  { value: 'manual', label: '手动顺序', description: '严格按成员顺序尝试可用分组。' },
]
const rules = ref<Array<{ model: string; group: string }>>([])
const mappings = ref<Array<{ from: string; to: string }>>([])
</script>
