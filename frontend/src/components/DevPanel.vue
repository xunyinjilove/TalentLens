<template>
  <!-- 调试面板 -->
  <Transition name="panel-slide">
    <div v-if="visible" class="dev-panel">
      <div class="panel-header">
        <span class="panel-title">Developer Console</span>
        <div class="panel-actions">
          <button class="panel-btn" @click="clearLogs" title="清空">
            <svg viewBox="0 0 16 16" fill="currentColor"><path d="M5.5 5.5A.5.5 0 0 1 6 6v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm2.5 0a.5.5 0 0 1 .5.5v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm3 .5a.5.5 0 0 0-1 0v6a.5.5 0 0 0 1 0V6z"/><path fill-rule="evenodd" d="M14.5 3a1 1 0 0 1-1 1H13v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V4h-.5a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1H6a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1h3.5a1 1 0 0 1 1 1v1zM4.118 4L4 4.059V13a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V4.059L11.882 4H4.118z"/></svg>
          </button>
          <button class="panel-btn close" @click="visible = false" title="关闭 (F12)">
            <svg viewBox="0 0 12 12"><path d="M2 2L10 10M10 2L2 10" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linecap="round"/></svg>
          </button>
        </div>
      </div>
      <div class="panel-body" ref="logContainer">
        <div
          v-for="(log, idx) in logs"
          :key="idx"
          :class="['log-entry', `log-${log.level}`]"
        >
          <span class="log-time">{{ log.time }}</span>
          <span class="log-level">{{ log.level.toUpperCase() }}</span>
          <span class="log-msg">{{ log.message }}</span>
        </div>
        <div v-if="logs.length === 0" class="log-empty">暂无日志，操作后将显示实时日志...</div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { getDevLogs, onDevLogChange, type DevLogEntry } from '../composables/useResumeStore'

const visible = ref(false)
const logs = ref<DevLogEntry[]>([...getDevLogs()])
const logContainer = ref<HTMLElement | null>(null)

// 监听日志变化
let unsubscribe: (() => void) | null = null

onMounted(() => {
  unsubscribe = onDevLogChange(() => {
    logs.value = [...getDevLogs()]
    nextTick(() => {
      if (logContainer.value) {
        logContainer.value.scrollTop = logContainer.value.scrollHeight
      }
    })
  })

  // F12 切换
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  if (unsubscribe) unsubscribe()
  window.removeEventListener('keydown', handleKeydown)
})

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'F12') {
    e.preventDefault()
    visible.value = !visible.value
  }
}

function clearLogs() {
  getDevLogs().length = 0
  logs.value = []
}
</script>

<style scoped lang="scss">
// 调试面板
.dev-panel {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  height: 240px;
  background: rgba(30, 30, 30, 0.95);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  z-index: 9999;
  display: flex;
  flex-direction: column;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 12px;
  background: rgba(255, 255, 255, 0.04);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;

  .panel-title {
    font-size: 11px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.7);
    letter-spacing: 0.3px;
  }

  .panel-actions {
    display: flex;
    gap: 4px;
  }
}

.panel-btn {
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: rgba(255, 255, 255, 0.5);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.1s;

  svg { width: 12px; height: 12px; }

  &:hover {
    background: rgba(255, 255, 255, 0.1);
    color: rgba(255, 255, 255, 0.8);
  }

  &.close:hover {
    background: rgba(255, 59, 48, 0.3);
    color: #FF3B30;
  }
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 6px 0;
  font-size: 11px;
  line-height: 1.6;

  &::-webkit-scrollbar { width: 6px; }
  &::-webkit-scrollbar-track { background: transparent; }
  &::-webkit-scrollbar-thumb { background: rgba(255, 255, 255, 0.1); border-radius: 3px; }
}

.log-entry {
  display: flex;
  gap: 8px;
  padding: 1px 12px;

  &:hover { background: rgba(255, 255, 255, 0.03); }

  .log-time {
    color: rgba(255, 255, 255, 0.3);
    flex-shrink: 0;
    min-width: 72px;
  }

  .log-level {
    flex-shrink: 0;
    min-width: 40px;
    font-weight: 600;
    font-size: 10px;
  }

  .log-msg {
    color: rgba(255, 255, 255, 0.85);
    word-break: break-all;
  }

  &.log-info .log-level { color: #007AFF; }
  &.log-warn .log-level { color: #FF9500; }
  &.log-warn .log-msg { color: #FF9500; }
  &.log-error .log-level { color: #FF3B30; }
  &.log-error .log-msg { color: #FF6961; }
}

.log-empty {
  color: rgba(255, 255, 255, 0.3);
  text-align: center;
  padding: 40px 20px;
  font-size: 12px;
}

// 面板滑入动画
.panel-slide-enter-active { transition: transform 0.2s ease-out; }
.panel-slide-leave-active { transition: transform 0.15s ease-in; }
.panel-slide-enter-from { transform: translateY(100%); }
.panel-slide-leave-to { transform: translateY(100%); }
</style>
