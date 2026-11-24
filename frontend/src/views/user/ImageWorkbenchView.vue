<template>
  <CanvasWorkbench v-if="auth.user" :key="auth.user.id" :user-id="auth.user.id" />
</template>

<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import CanvasWorkbench from '@/features/image-canvas/CanvasWorkbench.vue'

const auth = useAuthStore()
</script>
