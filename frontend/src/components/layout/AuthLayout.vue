<template>
  <div class="auth-shell relative flex min-h-screen items-center justify-center overflow-hidden p-4">
    <div class="auth-grid pointer-events-none absolute inset-0"></div>
    <div class="auth-signal auth-signal-one pointer-events-none absolute"></div>
    <div class="auth-signal auth-signal-two pointer-events-none absolute"></div>

    <header class="auth-topbar absolute left-4 right-4 top-4 z-20 mx-auto flex max-w-5xl items-center justify-between px-3 py-2 sm:left-6 sm:right-6">
      <router-link to="/home" class="flex items-center gap-2.5 text-sm font-bold text-gray-900 dark:text-white">
        <span class="flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-black/5">
          <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
        </span>
        <span class="max-w-[12rem] truncate">{{ siteName }}</span>
      </router-link>
      <div class="flex items-center gap-2">
        <router-link to="/home" class="auth-topbar-link hidden sm:inline-flex">首页</router-link>
        <LocaleSwitcher />
      </div>
    </header>

    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-md pt-16">
      <!-- Logo/Brand -->
      <div class="mb-7 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div
            class="auth-logo mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl"
          >
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="auth-brand-title mb-2 text-3xl font-black">
            {{ siteName }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="auth-card p-6 sm:p-8">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-7 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-shell {
  background: #f7f8fb;
  color: #121212;
}

.auth-grid {
  background-image: linear-gradient(rgba(22, 59, 242, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(22, 59, 242, 0.035) 1px, transparent 1px);
  background-size: 46px 46px;
  mask-image: linear-gradient(to bottom, transparent, #000 14%, #000 84%, transparent);
}

.auth-signal {
  left: 50%;
  top: 50%;
  width: min(680px, 92vw);
  aspect-ratio: 1;
  border: 1px solid rgba(37, 99, 235, 0.08);
  border-radius: 999px;
  transform: translate(-50%, -50%);
}

.auth-signal::after {
  position: absolute;
  inset: 12%;
  border: 1px dashed rgba(168, 85, 247, 0.12);
  border-radius: inherit;
  content: '';
}

.auth-signal-one {
  animation: auth-signal-spin 32s linear infinite;
}

.auth-signal-two {
  width: min(470px, 70vw);
  border-color: rgba(6, 182, 212, 0.1);
  animation: auth-signal-spin-reverse 24s linear infinite;
}

.auth-topbar {
  min-height: 54px;
  border: 1px solid rgba(18, 18, 18, 0.08);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 20px 50px -34px rgba(15, 23, 42, 0.5), inset 0 1px rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(22px) saturate(1.25);
  -webkit-backdrop-filter: blur(22px) saturate(1.25);
}

.auth-topbar-link {
  min-height: 36px;
  align-items: center;
  border-radius: 999px;
  padding: 0 14px;
  color: #687282;
  font-size: 12px;
  font-weight: 700;
}

.auth-topbar-link:hover {
  background: #eff6ff;
  color: #163bf2;
}

.auth-logo {
  box-shadow: 0 18px 32px -22px rgba(22, 59, 242, 0.75);
  outline: 1px solid rgba(22, 59, 242, 0.1);
}

.auth-brand-title {
  color: #121826;
  letter-spacing: -0.03em;
}

.auth-card {
  border: 1px solid rgba(18, 18, 18, 0.1);
  border-radius: 28px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 24px 80px rgba(15, 23, 42, 0.08), inset 0 1px rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
}

@keyframes auth-signal-spin {
  to { transform: translate(-50%, -50%) rotate(360deg); }
}

@keyframes auth-signal-spin-reverse {
  to { transform: translate(-50%, -50%) rotate(-360deg); }
}

:global(.dark) .auth-shell {
  background: #03050a;
  color: #f8fafc;
}

:global(.dark) .auth-topbar {
  border-color: rgba(148, 163, 184, 0.16);
  background: rgba(15, 23, 42, 0.82);
}

:global(.dark) .auth-brand-title {
  color: #f8fafc;
}

:global(.dark) .auth-card {
  border-color: rgba(71, 85, 105, 0.75);
  background: rgba(15, 23, 42, 0.88);
}

@media (prefers-reduced-motion: reduce) {
  .auth-signal-one,
  .auth-signal-two {
    animation: none;
  }
}
</style>
