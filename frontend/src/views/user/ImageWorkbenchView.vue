<template>
  <div class="image-workbench" @dblclick="addNode">
    <div class="workbench-grid" aria-hidden="true"></div>
    <header class="workbench-topbar"><button class="workbench-icon" type="button" aria-label="返回" @click="router.back()"><Icon name="chevronLeft" size="md" /></button><button class="canvas-pill" type="button"><Icon name="document" size="sm" /><span>未命名画布</span><b>{{ nodes.length }}</b></button><button class="workbench-icon ml-auto" type="button" aria-label="设置"><Icon name="cog" size="md" /></button></header>
    <div class="workbench-hint"><Icon name="infoCircle" size="md" /><strong>双击画布</strong><span>自由生成节点，也可以直接拖入或粘贴图片</span></div>
    <div v-for="node in nodes" :key="node.id" class="workbench-node" :style="{ left: `${node.x}%`, top: `${node.y}%` }"><Icon name="document" size="md" /><span>{{ node.label }}</span></div>
    <div class="workbench-tools"><button type="button" class="tool-active" @click="assetsOpen = !assetsOpen"><Icon name="document" size="sm" /><span>资产管理</span></button><button type="button" aria-label="整理画布" @click="nodes = []"><Icon name="grid" size="sm" /></button><button type="button" aria-label="画布小地图"><Icon name="document" size="sm" /></button><button type="button" aria-label="网格吸附"><Icon name="link" size="sm" /></button><button type="button" class="zoom-pill">100%</button></div>
    <div class="workbench-actions"><button type="button" class="action-primary" aria-label="添加节点" @click="addNode"><Icon name="plus" size="md" /></button><button type="button" aria-label="历史记录"><Icon name="refresh" size="md" /></button><button type="button" aria-label="快捷键"><Icon name="terminal" size="md" /></button></div>
    <div v-if="assetsOpen" class="assets-popover"><p class="text-sm font-semibold">资产管理</p><p class="mt-1 text-xs text-gray-400">拖入图片或从本地选择文件</p><button type="button" class="btn btn-secondary btn-sm mt-3">选择文件</button></div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'

const router = useRouter()
const assetsOpen = ref(false)
const nodes = ref<Array<{ id: number; x: number; y: number; label: string }>>([])
const addNode = (event?: MouseEvent) => {
  const x = event ? Math.min(82, Math.max(12, (event.clientX / window.innerWidth) * 100)) : 42
  const y = event ? Math.min(78, Math.max(18, (event.clientY / window.innerHeight) * 100)) : 42
  nodes.value.push({ id: Date.now(), x, y, label: `生成节点 ${nodes.value.length + 1}` })
}
</script>

<style scoped>
.image-workbench { position: relative; height: 100dvh; overflow: hidden; background: #101010; color: #f5f5f5; }
.workbench-grid { position: absolute; inset: 0; opacity: .72; background-image: radial-gradient(circle, rgba(255,255,255,.26) 1px, transparent 1px); background-size: 22px 22px; }
.workbench-topbar { position: absolute; z-index: 2; top: 18px; left: 20px; right: 20px; display: flex; align-items: center; gap: 10px; }
.workbench-icon, .canvas-pill, .workbench-tools, .workbench-actions, .tool-active, .zoom-pill { border: 1px solid rgba(255,255,255,.12); background: rgba(40,40,40,.86); color: #f5f5f5; box-shadow: 0 10px 30px rgba(0,0,0,.2); }
.workbench-icon { display:flex; align-items:center; justify-content:center; width:42px; height:42px; border-radius:12px; }
.canvas-pill { display:flex; align-items:center; gap:10px; min-height:42px; padding:0 14px; border-radius:12px; font-size:13px; font-weight:600; }
.canvas-pill b { display:grid; place-items:center; width:22px; height:22px; border-radius:999px; background:#333; font-size:11px; }
.workbench-hint { position:absolute; left:50%; top:50%; transform:translate(-50%,-50%); display:flex; align-items:center; gap:9px; color:#999; font-size:13px; white-space:nowrap; }
.workbench-hint strong { color:#f2f2f2; }
.workbench-tools { position:absolute; z-index:2; left:28px; bottom:18px; display:flex; align-items:center; gap:8px; padding:8px; border-radius:14px; }
.workbench-tools > button { display:flex; align-items:center; justify-content:center; width:38px; height:34px; border-radius:9px; color:#cfcfcf; }
.workbench-tools .tool-active { width:auto; gap:8px; padding:0 11px; }
.workbench-tools .zoom-pill { width:auto; padding:0 11px; font-weight:700; }
.workbench-actions { position:absolute; z-index:2; left:50%; bottom:18px; transform:translateX(-50%); display:flex; gap:4px; padding:6px; border-radius:14px; }
.workbench-actions button { display:grid; place-items:center; width:38px; height:36px; border-radius:10px; color:#cfcfcf; }
.workbench-actions .action-primary { background:#fff; color:#111; }
.workbench-node { position:absolute; z-index:1; display:flex; align-items:center; gap:8px; transform:translate(-50%,-50%); padding:12px 14px; border:1px solid rgba(255,255,255,.2); border-radius:12px; background:#242424; font-size:12px; box-shadow:0 12px 30px rgba(0,0,0,.3); }
.assets-popover { position:absolute; z-index:3; left:28px; bottom:76px; width:220px; padding:14px; border:1px solid rgba(255,255,255,.14); border-radius:12px; background:#242424; box-shadow:0 18px 45px rgba(0,0,0,.4); }
@media (max-width: 640px) { .workbench-hint { font-size:11px; } .workbench-tools { left:12px; bottom:12px; } .workbench-actions { bottom:12px; } .workbench-topbar { top:12px; left:12px; right:12px; } }
</style>
