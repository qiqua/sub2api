<template>
  <AppLayout>
    <div class="app-page-scroll mx-auto max-w-[1050px] space-y-5">
      <nav class="flex gap-6 border-b border-gray-200 text-sm dark:border-dark-700"><button v-for="tab in tabs" :key="tab" type="button" class="border-b-2 px-1 pb-3 font-medium" :class="activeTab === tab ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500 dark:text-dark-400'" @click="activeTab = tab">{{ tab }}</button></nav>
      <header class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between"><div><h1 class="page-title text-2xl font-semibold text-gray-950 dark:text-white">余额</h1><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">购买余额充值包或订阅套餐。</p></div><div class="flex gap-2"><router-link to="/redeem" class="btn btn-secondary"><Icon name="gift" size="sm" />兑换中心</router-link><button type="button" class="btn btn-secondary"><Icon name="refresh" size="sm" />刷新</button></div></header>
      <div class="grid gap-5 xl:grid-cols-[1fr_380px]">
        <section class="card p-5"><div class="flex items-center justify-between"><div><h2 class="text-base font-semibold text-gray-900 dark:text-white">余额充值包</h2><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">购买余额充值包或试用套餐。</p></div><span class="rounded-lg bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-200">{{ packages.length }} 个项目</span></div><div class="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3"><button v-for="(item, index) in packages" :key="item.amount" type="button" class="relative min-h-36 rounded-xl border p-4 text-left transition hover:-translate-y-0.5 hover:shadow-md" :class="selected === index ? 'border-primary-500 ring-2 ring-primary-100 dark:ring-primary-900/40' : 'border-gray-200 dark:border-dark-700'" :style="{ background: item.tint }" @click="selected = index"><Icon v-if="selected === index" name="checkCircle" size="sm" class="absolute right-3 top-3 text-primary-600" /><p class="text-xs text-gray-500">到账余额</p><p class="mt-1 text-2xl font-bold text-gray-950">${{ item.amount }}</p><p class="mt-4 text-xs text-gray-500">支付金额</p><p class="text-sm font-semibold text-gray-900">¥{{ item.price }}</p></button></div></section>
        <aside class="card p-5"><div class="rounded-xl border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40"><div class="flex items-center gap-3"><div class="flex h-10 w-10 items-center justify-center rounded-lg bg-gray-900 text-white"><Icon name="creditCard" size="md" /></div><div><h2 class="font-semibold text-gray-900 dark:text-white">确认订单</h2><p class="text-xs text-gray-500">¥{{ current.price }} / ${{ current.amount }}</p></div></div></div><h3 class="mt-5 text-sm font-semibold text-gray-900 dark:text-white">支付方式</h3><label class="mt-3 flex cursor-pointer items-center justify-between rounded-xl border-2 border-emerald-500 bg-emerald-50/50 p-3 dark:bg-emerald-950/20"><span class="flex items-center gap-3"><span class="flex h-8 w-8 items-center justify-center rounded-lg bg-white text-emerald-600 shadow-sm"><Icon name="check" size="sm" /></span><span class="text-sm font-medium">微信支付</span></span><input type="radio" checked class="h-4 w-4 text-emerald-600" /></label><dl class="mt-5 divide-y divide-gray-100 rounded-xl border border-gray-200 px-4 dark:divide-dark-700 dark:border-dark-700"><div class="flex justify-between py-3 text-sm"><dt class="text-gray-500">到账余额</dt><dd class="font-semibold">${{ current.amount }}</dd></div><div class="flex justify-between py-3 text-sm"><dt class="text-gray-500">原价</dt><dd>¥{{ current.price }}</dd></div><div class="flex justify-between py-3 text-sm"><dt class="text-gray-500">支付金额</dt><dd class="text-xl font-bold text-primary-600">¥{{ current.price }}</dd></div></dl><button type="button" class="btn btn-primary mt-4 w-full" @click="notice = '订单已准备，接入支付后即可继续'">确认支付</button><p v-if="notice" class="mt-3 rounded-lg bg-primary-50 px-3 py-2 text-center text-xs text-primary-700">{{ notice }}</p></aside>
      </div>
      <section class="card p-5"><h2 class="text-base font-semibold text-gray-900 dark:text-white">余额说明</h2><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">充值余额即时到账，可在账户设置中查看并发与 RPM 限制。</p><div class="mt-4 grid gap-3 sm:grid-cols-3"><div v-for="tip in tips" :key="tip.title" class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900/40"><Icon :name="tip.icon" size="sm" class="text-primary-600" /><p class="mt-2 text-sm font-medium text-gray-800 dark:text-dark-100">{{ tip.title }}</p><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ tip.description }}</p></div></div></section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const tabs = ['余额', '订阅', '我的订单', '统计']
const activeTab = ref('余额')
const selected = ref(0)
const notice = ref('')
const packages = [
  { amount: '1.00', price: '1.00', tint: 'linear-gradient(135deg,#eff6ff,#fff)' },
  { amount: '10.00', price: '10.00', tint: 'linear-gradient(135deg,#ecfeff,#fff)' },
  { amount: '20.00', price: '20.00', tint: 'linear-gradient(135deg,#f5f3ff,#fff)' },
  { amount: '50.00', price: '50.00', tint: 'linear-gradient(135deg,#fff7ed,#fff)' },
  { amount: '100.00', price: '100.00', tint: 'linear-gradient(135deg,#ecfdf5,#fff)' },
  { amount: '200.00', price: '200.00', tint: 'linear-gradient(135deg,#fff1f2,#fff)' },
]
const current = computed(() => packages[selected.value])
const tips = [
  { icon: 'bolt', title: '即时到账', description: '支付成功后余额自动更新。' },
  { icon: 'shield', title: '安全支付', description: '订单由站点支付配置统一处理。' },
  { icon: 'chart', title: '透明用量', description: '可在使用量统计中查看每次消耗。' },
] as const
</script>
