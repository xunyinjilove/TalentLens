<template>
  <header class="title-bar" data-wails-drag>
    <!-- 左侧：返回按钮 + 页面标题/面包屑 -->
    <div class="title-bar-left" data-wails-no-drag>
      <button v-if="showBack" class="back-btn" @click="handleBack" title="返回上一页">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M19 12H5M12 19l-7-7 7-7" />
        </svg>
        <span v-if="backText" class="back-text">{{ backText }}</span>
      </button>

      <div class="title-info">
        <span class="title-text">{{ title }}</span>
        <span v-if="subtitle" class="title-subtitle">{{ subtitle }}</span>
      </div>
    </div>

    <!-- 中间：宽敞的可拖拽区 -->
    <div class="title-bar-center" data-wails-drag>
      <!-- 拖拽手柄区域，保持空置以供双击最大化或拖拽窗口 -->
    </div>

    <!-- 右侧：页面动作插槽 + 窗口控制按钮组 -->
    <div class="title-bar-right" data-wails-no-drag>
      <!-- 业务插槽区（如岗位标签、语言切换等） -->
      <div class="actions-slot">
        <slot></slot>
      </div>

      <div class="divider-line"></div>

      <!-- 右上角 Windows / VS Code 风格窗口控制按钮组 -->
      <div class="window-controls">
        <button
          class="win-btn minimize"
          @click="minimizeWindow"
          title="最小化"
          type="button"
        >
          <svg viewBox="0 0 12 12">
            <line x1="2" y1="6" x2="10" y2="6" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
          </svg>
        </button>

        <button
          class="win-btn maximize"
          @click="maximizeWindow"
          :title="isMaximized ? '向下还原' : '最大化'"
          type="button"
        >
          <svg v-if="!isMaximized" viewBox="0 0 12 12">
            <rect x="2.5" y="2.5" width="7" height="7" fill="none" stroke="currentColor" stroke-width="1.1" rx="0.5" />
          </svg>
          <svg v-else viewBox="0 0 12 12">
            <path d="M3.5 4.5V2.5H9.5V8.5H7.5" fill="none" stroke="currentColor" stroke-width="1.1" />
            <rect x="2.5" y="4.5" width="5" height="5" fill="none" stroke="currentColor" stroke-width="1.1" />
          </svg>
        </button>

        <button
          class="win-btn close"
          @click="closeWindow"
          title="关闭"
          type="button"
        >
          <svg viewBox="0 0 12 12">
            <path d="M2.5 2.5l7 7M9.5 2.5l-7 7" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
          </svg>
        </button>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
const props = withDefaults(defineProps<{
  title?: string
  subtitle?: string
  showBack?: boolean
  backText?: string
}>(), {
  title: 'Tellgen',
  subtitle: '',
  showBack: false,
  backText: '返回'
})

const emit = defineEmits<{
  (e: 'back'): void
}>()

const router = useRouter()
const isMaximized = ref(false)

// Wails 绑定
let WailsApp: any = null

onMounted(async () => {
  try {
    WailsApp = await import('../../wailsjs/go/main/App')
  } catch {
    // 非 Wails 环境下静默降级
  }
})

function handleBack() {
  emit('back')
  router.back()
}

async function minimizeWindow() {
  if (WailsApp) {
    await WailsApp.WindowMinimize()
  }
}

async function maximizeWindow() {
  if (WailsApp) {
    await WailsApp.WindowMaximize()
    isMaximized.value = !isMaximized.value
  }
}

async function closeWindow() {
  if (WailsApp) {
    await WailsApp.WindowClose()
  }
}
</script>

<style scoped lang="scss">
.title-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: var(--topbar-height);
  background: var(--topbar-bg);
  border-bottom: 1px solid var(--topbar-border);
  user-select: none;
  -webkit-app-region: drag;
  --wails-draggable: drag;
  position: relative;
  z-index: 90;
  transition: background var(--transition-normal), border-color var(--transition-normal);
  flex-shrink: 0;
}

// 左侧区域
.title-bar-left {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-left: 16px;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;

  .back-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 5px 8px;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all var(--transition-fast);

    svg {
      width: 14px;
      height: 14px;
    }

    &:hover {
      background: var(--bg-hover);
      color: var(--text-primary);
      border-color: var(--border-color-light);
    }
  }

  .title-info {
    display: flex;
    align-items: baseline;
    gap: 8px;

    .title-text {
      font-size: 13px;
      font-weight: 600;
      color: var(--topbar-text);
      letter-spacing: -0.01em;
    }

    .title-subtitle {
      font-size: 11px;
      color: var(--text-muted);
    }
  }
}

// 中间拖拽区
.title-bar-center {
  flex: 1;
  height: 100%;
  -webkit-app-region: drag;
  --wails-draggable: drag;
}

// 右侧操作与窗口控制
.title-bar-right {
  display: flex;
  align-items: center;
  height: 100%;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;

  .actions-slot {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-right: 12px;
  }

  .divider-line {
    width: 1px;
    height: 18px;
    background: var(--topbar-border);
    margin-right: 4px;
  }

  // Windows 原生级高可见性窗口控制按钮组
  .window-controls {
    display: flex;
    align-items: center;
    height: 100%;

    .win-btn {
      width: 46px;
      height: 100%;
      border: none;
      background: transparent;
      color: var(--text-secondary);
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 0;
      transition: background var(--transition-fast), color var(--transition-fast);

      svg {
        width: 11px;
        height: 11px;
      }

      &:hover {
        background: var(--window-btn-hover);
        color: var(--topbar-text);
      }

      &:active {
        background: var(--bg-active);
      }

      // 关闭按钮悬浮高亮红底，与 Windows / VS Code 体验完全一致
      &.close:hover {
        background: var(--window-close-hover);
        color: #FFFFFF;
      }

      &.close:active {
        background: #B71C1C;
        color: #FFFFFF;
      }
    }
  }
}
</style>
