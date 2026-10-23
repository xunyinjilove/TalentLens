<template>
  <div class="workbench-window-controls" data-wails-no-drag>
    <button title="最小化" @click="invoke('WindowMinimize')">─</button>
    <button title="最大化或还原" @click="invoke('WindowMaximize')">□</button>
    <button title="关闭" class="close" @click="invoke('WindowClose')">×</button>
  </div>
</template>

<script setup lang="ts">
async function invoke(method: string) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api[method]()
  } catch { /* Browser preview has no desktop window. */ }
}
</script>

<style scoped>
.workbench-window-controls { position:absolute;right:0;top:0;height:42px;display:flex;align-items:stretch;z-index:80;color:var(--wb-muted); }
.workbench-window-controls button { width:42px;border:0;background:transparent;color:inherit;cursor:pointer;font-size:16px; }
.workbench-window-controls button:hover { background:var(--wb-brand-soft);color:var(--wb-brand); }
.workbench-window-controls button.close:hover { background:#ef4444;color:white; }
</style>
