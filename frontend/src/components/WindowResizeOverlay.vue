<template>
  <div v-if="!isMaximized" class="window-resize-overlay" data-wails-no-drag>
    <div class="resize-handle top" @mousedown="startResize('n-resize', $event)" />
    <div class="resize-handle bottom" @mousedown="startResize('s-resize', $event)" />
    <div class="resize-handle left" @mousedown="startResize('w-resize', $event)" />
    <div class="resize-handle right" @mousedown="startResize('e-resize', $event)" />
    <div class="resize-handle top-left" @mousedown="startResize('nw-resize', $event)" />
    <div class="resize-handle top-right" @mousedown="startResize('ne-resize', $event)" />
    <div class="resize-handle bottom-left" @mousedown="startResize('sw-resize', $event)" />
    <div class="resize-handle bottom-right" @mousedown="startResize('se-resize', $event)" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMaximized = ref(false)
let checkTimer: number | null = null
let saveTimer: number | null = null

// 检查窗口是否处于最大化或全屏状态
async function checkMaximizedState() {
  try {
    const runtime = (window as any).runtime
    if (runtime && typeof runtime.WindowIsMaximised === 'function') {
      const max = await runtime.WindowIsMaximised()
      const full = typeof runtime.WindowIsFullscreen === 'function' ? await runtime.WindowIsFullscreen() : false
      isMaximized.value = Boolean(max || full)
      return
    }
  } catch {}

  // 纯浏览器降级兜底检测
  const isNearScreen = Math.abs(window.innerWidth - screen.availWidth) < 10 &&
                       Math.abs(window.innerHeight - screen.availHeight) < 20
  isMaximized.value = isNearScreen
}

// 启动原生拉伸逻辑
async function startResize(edge: string, event: MouseEvent) {
  // 必须阻止默认行为，避免选中文本或触发 TitleBar 拖拽
  event.preventDefault()
  event.stopPropagation()

  try {
    const WailsApp = await import('../../wailsjs/go/main/App')
    if (WailsApp && typeof WailsApp.StartWindowResize === 'function') {
      await WailsApp.StartWindowResize(edge)
      return
    }
  } catch {}

  const wailsObj = (window as any).wails
  if (wailsObj && wailsObj.flags) {
    wailsObj.flags.resizeEdge = edge
  }

  // 触发 Wails 后端 Windows 原生 Win32 WM_NCLBUTTONDOWN 处理管道
  if (typeof (window as any).WailsInvoke === 'function') {
    (window as any).WailsInvoke(`resize:${edge}`)
  } else {
    // 浏览器预览环境降级（纯演示）
    console.debug('[WindowResizeOverlay] Wails runtime not present, edge:', edge)
  }
}

// 窗口尺寸发生变动时，防抖同步保存至持久化配置
function onWindowResize() {
  void checkMaximizedState()
  if (saveTimer) {
    window.clearTimeout(saveTimer)
  }
  saveTimer = window.setTimeout(async () => {
    try {
      const WailsApp = await import('../../wailsjs/go/main/App')
      if (WailsApp && typeof WailsApp.SaveWindowSettings === 'function') {
        const curW = window.innerWidth
        const curH = window.innerHeight
        if (curW >= 900 && curH >= 600 && !isMaximized.value) {
          await WailsApp.SaveWindowSettings(curW, curH, isMaximized.value)
        }
      }
    } catch {}
  }, 400)
}

onMounted(() => {
  void checkMaximizedState()
  window.addEventListener('resize', onWindowResize)
  // 定时检查状态，应对还原/最大化等变化
  checkTimer = window.setInterval(() => {
    void checkMaximizedState()
  }, 800)
})

onUnmounted(() => {
  window.removeEventListener('resize', onWindowResize)
  if (checkTimer) {
    clearInterval(checkTimer)
  }
  if (saveTimer) {
    clearTimeout(saveTimer)
  }
})
</script>

<style scoped>
.window-resize-overlay {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 999999;
}

.resize-handle {
  position: absolute;
  pointer-events: auto;
  user-select: none;
  -webkit-user-select: none;
  -webkit-app-region: no-drag;
}

/* 顶部与底部边缘：厚度 7px，两端各预留 14px 给角落 */
.resize-handle.top {
  top: 0;
  left: 14px;
  right: 14px;
  height: 7px;
  cursor: n-resize;
}

.resize-handle.bottom {
  bottom: 0;
  left: 14px;
  right: 14px;
  height: 7px;
  cursor: s-resize;
}

/* 左侧与右侧边缘：厚度 7px，两端各预留 14px 给角落 */
.resize-handle.left {
  left: 0;
  top: 14px;
  bottom: 14px;
  width: 7px;
  cursor: w-resize;
}

.resize-handle.right {
  right: 0;
  top: 14px;
  bottom: 14px;
  width: 7px;
  cursor: e-resize;
}

/* 四个角落手柄：14px × 14px */
.resize-handle.top-left {
  top: 0;
  left: 0;
  width: 14px;
  height: 14px;
  cursor: nw-resize;
}

.resize-handle.top-right {
  top: 0;
  right: 0;
  width: 14px;
  height: 14px;
  cursor: ne-resize;
}

.resize-handle.bottom-left {
  bottom: 0;
  left: 0;
  width: 14px;
  height: 14px;
  cursor: sw-resize;
}

.resize-handle.bottom-right {
  bottom: 0;
  right: 0;
  width: 14px;
  height: 14px;
  cursor: se-resize;
}
</style>
