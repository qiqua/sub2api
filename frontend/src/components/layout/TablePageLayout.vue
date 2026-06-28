<template>
  <div class="table-page-layout" :class="{ 'mobile-mode': isMobile }">
    <div v-if="$slots.actions" class="layout-section-fixed">
      <slot name="actions" />
    </div>

    <div v-if="$slots.filters" class="layout-section-fixed">
      <slot name="filters" />
    </div>

    <div class="layout-section-scrollable">
      <div class="card table-scroll-container">
        <slot name="table" />
      </div>
    </div>

    <div v-if="$slots.pagination" class="layout-section-fixed">
      <slot name="pagination" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMobile = ref(false)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 1024
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.table-page-layout {
  @apply flex h-full min-h-0 flex-col gap-6 overflow-hidden;
}

.layout-section-fixed {
  @apply flex-shrink-0;
}

.layout-section-scrollable {
  @apply flex-1 min-h-0 flex flex-col overflow-hidden;
}

.table-scroll-container {
  @apply flex h-full flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800;
}

.table-scroll-container :deep(.table-wrapper) {
  @apply flex-1 overflow-x-auto overflow-y-auto;
  scrollbar-gutter: stable;
}

.table-scroll-container :deep(table) {
  @apply w-full;
  min-width: max-content;
  display: table;
}

.table-scroll-container :deep(thead) {
  @apply bg-gray-50/80 backdrop-blur-sm dark:bg-dark-800/80;
}

.table-scroll-container :deep(th) {
  @apply border-b border-gray-200 px-5 py-4 text-left text-sm font-medium text-gray-600 dark:border-dark-700 dark:text-dark-300;
}

.table-scroll-container :deep(td) {
  @apply border-b border-gray-100 px-5 py-4 text-sm text-gray-700 dark:border-dark-800 dark:text-gray-300;
}

.table-page-layout.mobile-mode {
  @apply h-auto overflow-visible;
}

.table-page-layout.mobile-mode .table-scroll-container {
  @apply h-auto overflow-visible border-none bg-transparent shadow-none;
}

.table-page-layout.mobile-mode .layout-section-scrollable {
  @apply flex-none min-h-fit;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(.table-wrapper) {
  @apply overflow-visible;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(table) {
  @apply flex-none;
  display: table;
  min-width: 100%;
}
</style>
