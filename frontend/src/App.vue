<template>
  <el-config-provider :locale="locale">
    <div class="app-layout-frame" :data-theme="theme">
      <!-- 全局常驻深色侧边栏 -->
      <AppSidebar />

      <!-- 主视图区域 -->
      <div class="app-main-viewport">
        <WorkbenchWindowControls v-if="workbenchRoute" />
        <router-view />
      </div>

      <!-- 全局 AI 招聘助理抽屉 & 悬浮按钮 -->
      <AIAssistantDrawer />

      <!-- 全局八方原生无边框窗口缩放手柄 -->
      <WindowResizeOverlay />
    </div>
  </el-config-provider>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import AppSidebar from './components/AppSidebar.vue'
import AIAssistantDrawer from './components/AIAssistantDrawer.vue'
import WorkbenchWindowControls from './components/WorkbenchWindowControls.vue'
import WindowResizeOverlay from './components/WindowResizeOverlay.vue'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useTheme } from './composables/useTheme'
import { useProjectStore } from './composables/useProjectStore'
import { useRecruitmentStore } from './composables/useRecruitmentStore'
import './styles/recruitment-workbench.scss'

const locale = ref(zhCn)
const { theme, initTheme } = useTheme()
const projectStore = useProjectStore()
const recruitmentStore = useRecruitmentStore()
const route = useRoute()
const workbenchRoute = computed(() => ['/', '/jobs', '/talent', '/interviews', '/pipeline'].includes(route.path))

onMounted(async () => {
  initGlobalWindowDrag()
  initTheme()
  void projectStore.loadProjects()
  void recruitmentStore.initEvents()
  // 启动时静默将本地存储的 AI 模型配置对齐同步到 Go 后端
  try {
    const saved = localStorage.getItem('goresume_settings')
    if (saved) {
      const s = JSON.parse(saved)
      if (s.ai && (s.ai.apiKey || s.ai.api_key)) {
        const WailsApp = await import('../wailsjs/go/main/App')
        if (WailsApp && typeof WailsApp.SaveAIConfig === 'function') {
          await WailsApp.SaveAIConfig({
            provider: s.ai.provider || 'deepseek',
            base_url: s.ai.baseURL || s.ai.base_url || 'https://api.deepseek.com/v1',
            api_key: s.ai.apiKey || s.ai.api_key || '',
            model: s.ai.model || 'deepseek-chat',
            max_retries: 3,
            timeout: 60
          })
        }
      }
    }
  } catch {}
})

// 全局无死角窗口拖拽移动与双击最大化分发器
function initGlobalWindowDrag() {
  window.addEventListener('mousedown', async (e: MouseEvent) => {
    if (e.button !== 0) return

    const target = e.target as HTMLElement | null
    if (!target) return

    // 交互型组件严格排除，确保按钮/输入框/下拉菜单点击行为 100% 正常
    if (target.closest('[data-wails-no-drag], button, input, textarea, select, a, .el-button, .el-input, .el-dropdown, .el-dialog, .workbench-window-controls, .resize-handle')) {
      return
    }

    // 命中可拖拽头部或品牌区域
    const dragContainer = target.closest('[data-wails-drag], .title-bar, .wb-header, .sidebar-brand, .wails-draggable')
    if (dragContainer) {
      try {
        const WailsApp = await import('../wailsjs/go/main/App')
        if (WailsApp && typeof WailsApp.StartWindowDrag === 'function') {
          await WailsApp.StartWindowDrag()
          return
        }
      } catch {}

      if (typeof (window as any).WailsInvoke === 'function') {
        (window as any).WailsInvoke('drag')
      }
    }
  })

  // 双击顶部空白区域切换最大化 / 还原
  window.addEventListener('dblclick', async (e: MouseEvent) => {
    const target = e.target as HTMLElement | null
    if (!target) return
    if (target.closest('[data-wails-no-drag], button, input, textarea, select, a, .el-button, .el-dropdown, .workbench-window-controls, .resize-handle')) {
      return
    }
    const dragContainer = target.closest('[data-wails-drag], .title-bar, .wb-header, .wails-draggable')
    if (dragContainer) {
      try {
        const WailsApp = await import('../wailsjs/go/main/App')
        if (WailsApp && typeof WailsApp.WindowMaximize === 'function') {
          await WailsApp.WindowMaximize()
        }
      } catch {}
    }
  })
}
</script>

<style lang="scss">
.app-layout-frame {
  width: 100vw;
  height: 100vh;
  display: flex;
  overflow: hidden;
  background-color: var(--bg-app);
  color: var(--text-primary);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
  transition: background-color var(--transition-normal), color var(--transition-normal);
}

.app-main-viewport {
  flex: 1;
  height: 100vh;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: var(--bg-app);
  position: relative;
}
</style>
