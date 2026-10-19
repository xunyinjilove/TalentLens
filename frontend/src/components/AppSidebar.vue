<template>
  <aside class="app-sidebar">
    <!-- 顶部品牌区域（可拖拽窗口） -->
    <div class="sidebar-brand" data-wails-drag>
      <div class="brand-logo-wrap">
        <div class="brand-icon">
          <img src="../assets/tellgen-logo.png" alt="Tellgen" class="brand-logo-img" />
        </div>
        <div class="brand-text">
          <h1 class="brand-title">透景</h1>
          <span class="brand-subtitle">Tellgen</span>
        </div>
      </div>
      <span class="brand-badge">SaaS</span>
    </div>

    <!-- 中部主导航区 -->
    <nav class="sidebar-nav">
      <div class="nav-group-title">工作台</div>

      <router-link to="/" class="nav-item" :class="{ active: route.path === '/' }">
        <el-icon class="nav-icon"><DataBoard /></el-icon>
        <span class="nav-label">招聘概览</span>
      </router-link>

      <router-link to="/jobs" class="nav-item" :class="{ active: isProjectsActive }">
        <el-icon class="nav-icon"><Briefcase /></el-icon>
        <span class="nav-label">职位管理</span>
        <span v-if="projectCount > 0" class="nav-badge">{{ projectCount }}</span>
      </router-link>

      <router-link to="/talent" class="nav-item" :class="{ active: route.path === '/talent' }">
        <el-icon class="nav-icon"><Document /></el-icon>
        <span class="nav-label">简历人才库</span>
      </router-link>

      <div class="nav-group-title">智能助手</div>

      <div
        class="nav-item cursor-pointer"
        :class="{ active: assistantStore.isOpen }"
        @click="assistantStore.toggleDrawer"
      >
        <el-icon class="nav-icon"><ChatDotRound /></el-icon>
        <span class="nav-label">AI 招聘助理</span>
        <span class="nav-badge ai-badge">Copilot</span>
      </div>

      <div class="nav-group-title">系统</div>

      <router-link
        to="/settings"
        class="nav-item"
        :class="{ active: route.path === '/settings' }"
      >
        <el-icon class="nav-icon"><Setting /></el-icon>
        <span class="nav-label">系统设置</span>
      </router-link>
    </nav>

    <!-- 底部功能区域：主题切换 + 状态 -->
    <div class="sidebar-footer">
      <button
        class="theme-toggle-btn"
        @click="toggleTheme"
        :title="isDark ? '切换到浅色模式' : '切换到深色模式'"
      >
        <div class="theme-icon-wrap">
          <el-icon v-if="isDark" class="theme-icon sun"><Sunny /></el-icon>
          <el-icon v-else class="theme-icon moon"><Moon /></el-icon>
        </div>
        <span class="theme-label">{{ isDark ? '深色模式' : '浅色模式' }}</span>
        <span class="theme-switch-indicator" :class="{ 'is-dark': isDark }"></span>
      </button>

      <div class="app-status-bar">
        <span class="status-dot online"></span>
        <span class="status-text">本地 AI 引擎就绪</span>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useTheme } from '../composables/useTheme'
import { useProjectStore } from '../composables/useProjectStore'
import { useAIAssistantStore } from '../composables/useAIAssistantStore'

const route = useRoute()
const { isDark, toggleTheme } = useTheme()
const projectStore = useProjectStore()
const assistantStore = useAIAssistantStore()

const projectCount = computed(() => projectStore.projects.length)

// 判断项目列表是否处于激活态（包含详情页）
const isProjectsActive = computed(() => {
  return route.path.startsWith('/jobs') || route.path.startsWith('/project/')
})
</script>

<style scoped lang="scss">
.app-sidebar {
  width: var(--sidebar-width);
  height: 100vh;
  background: var(--sidebar-bg);
  border-right: 1px solid var(--sidebar-border);
  display: flex;
  flex-direction: column;
  user-select: none;
  flex-shrink: 0;
  transition: background var(--transition-normal), border-color var(--transition-normal);
  z-index: 100;
}

// 顶部品牌区
.sidebar-brand {
  height: 56px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--sidebar-border);
  cursor: default;
  -webkit-app-region: drag;
  --wails-draggable: drag;

  .brand-logo-wrap {
    display: flex;
    align-items: center;
    gap: 8px;
    -webkit-app-region: drag;
    --wails-draggable: drag;
  }

  .brand-icon {
    width: 30px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.05);
    padding: 2px;
    transition: transform var(--transition-fast);

    &:hover {
      transform: scale(1.05);
    }

    .brand-logo-img {
      width: 100%;
      height: 100%;
      object-fit: contain;
      filter: drop-shadow(0 2px 6px rgba(229, 57, 53, 0.35));
    }
  }

  .brand-text {
    display: flex;
    flex-direction: column;
    justify-content: center;

    .brand-title {
      margin: 0;
      font-size: 16px;
      font-weight: 800;
      color: var(--brand-red);
      line-height: 1.15;
      letter-spacing: 1px;
      text-shadow: 0 0 12px var(--brand-red-glow);
    }

    .brand-subtitle {
      font-size: 9px;
      color: var(--sidebar-text-muted);
      font-weight: 600;
      letter-spacing: 0.5px;
      text-transform: uppercase;
      line-height: 1.2;
    }
  }

  .brand-badge {
    font-size: 9px;
    padding: 1px 5px;
    background: rgba(37, 99, 235, 0.2);
    color: #60A5FA;
    border-radius: 4px;
    font-weight: 700;
    letter-spacing: 0.5px;
    -webkit-app-region: no-drag;
  }
}

// 中间导航项
.sidebar-nav {
  flex: 1;
  padding: 12px 6px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;

  .nav-group-title {
    font-size: 10px;
    font-weight: 600;
    color: var(--sidebar-text-muted);
    padding: 10px 8px 3px;
    letter-spacing: 0.5px;
    text-transform: uppercase;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border-radius: 8px;
    color: var(--sidebar-text);
    text-decoration: none;
    font-size: 13px;
    font-weight: 500;
    position: relative;
    transition: all var(--transition-fast);
    background: transparent;

    &.sidebar-action {
      width: 100%;
      min-height: 44px;
      border: 0;
      text-align: left;
      cursor: pointer;
      font-family: inherit;
    }

    .nav-icon {
      font-size: 16px;
      color: var(--sidebar-text-muted);
      transition: color var(--transition-fast);
    }

    .nav-label {
      flex: 1;
    }

    .nav-badge {
      font-size: 11px;
      padding: 1px 7px;
      background: rgba(255, 255, 255, 0.1);
      color: var(--sidebar-text-muted);
      border-radius: 10px;
      font-weight: 600;
    }

    &:hover {
      background: var(--sidebar-bg-hover);
      color: #FFFFFF;

      .nav-icon {
        color: #FFFFFF;
      }
    }

    &.active {
      background: var(--sidebar-bg-active);
      color: #FFFFFF;
      font-weight: 600;

      .nav-icon {
        color: var(--accent-blue);
      }

      .nav-badge {
        background: var(--accent-blue);
        color: #FFFFFF;
      }

      &::before {
        content: '';
        position: absolute;
        left: 0;
        top: 6px;
        bottom: 6px;
        width: 3px;
        background: var(--sidebar-active-indicator);
        border-radius: 0 3px 3px 0;
      }
    }

    &.cursor-pointer {
      cursor: pointer;
    }

    .ai-badge {
      background: rgba(37, 99, 235, 0.25);
      color: #60A5FA;
      font-size: 10px;
    }
  }
}

.posting-picker-hint { margin: 0 0 12px; color: #64748b; line-height: 1.5; font-size: 13px; }

// 底部区域
.sidebar-footer {
  padding: 12px 8px;
  border-top: 1px solid var(--sidebar-border);
  display: flex;
  flex-direction: column;
  gap: 8px;

  .theme-toggle-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 10px;
    border: 1px solid var(--sidebar-border);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.04);
    color: var(--sidebar-text);
    cursor: pointer;
    font-size: 12px;
    font-weight: 500;
    transition: all var(--transition-fast);

    &:hover {
      background: var(--sidebar-bg-hover);
      border-color: rgba(255, 255, 255, 0.15);
      color: #FFFFFF;
    }

    .theme-icon-wrap {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;

      .theme-icon {
        font-size: 15px;

        &.sun {
          color: #FBBF24;
        }

        &.moon {
          color: #818CF8;
        }
      }
    }

    .theme-label {
      flex: 1;
      text-align: left;
    }

    .theme-switch-indicator {
      width: 28px;
      height: 16px;
      border-radius: 10px;
      background: rgba(255, 255, 255, 0.15);
      position: relative;
      transition: background var(--transition-fast);

      &::after {
        content: '';
        position: absolute;
        width: 12px;
        height: 12px;
        border-radius: 50%;
        background: #FFFFFF;
        top: 2px;
        left: 2px;
        transition: transform var(--transition-fast);
      }

      &.is-dark {
        background: var(--accent-blue);

        &::after {
          transform: translateX(12px);
        }
      }
    }
  }

  .app-status-bar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 4px;

    .status-dot {
      width: 6px;
      height: 6px;
      border-radius: 50%;

      &.online {
        background: var(--color-success);
        box-shadow: 0 0 6px rgba(16, 185, 129, 0.6);
      }
    }

    .status-text {
      font-size: 11px;
      color: var(--sidebar-text-muted);
    }
  }
}
</style>
