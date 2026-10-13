import { ref, computed } from 'vue'

export type ThemeMode = 'light' | 'dark'

const THEME_KEY = 'talentlens_theme'

// 全局单例响应式状态
const theme = ref<ThemeMode>('light')
const isDark = computed(() => theme.value === 'dark')

/**
 * 应用主题到 DOM
 */
function applyThemeToDOM(current: ThemeMode) {
  const root = document.documentElement
  root.setAttribute('data-theme', current)
  // Element Plus 官方黑暗模式切换规范：html 标签添加 .dark class
  if (current === 'dark') {
    root.classList.add('dark')
  } else {
    root.classList.remove('dark')
  }
}

/**
 * 初始化主题设置（从 localStorage 或系统首选颜色获取）
 */
function initTheme(): ThemeMode {
  const saved = localStorage.getItem(THEME_KEY) as ThemeMode | null
  if (saved === 'light' || saved === 'dark') {
    theme.value = saved
  } else if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    theme.value = 'dark'
  } else {
    theme.value = 'light'
  }
  applyThemeToDOM(theme.value)
  return theme.value
}

/**
 * 切换 Light / Dark 主题
 */
function toggleTheme(): ThemeMode {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
  localStorage.setItem(THEME_KEY, theme.value)
  applyThemeToDOM(theme.value)
  return theme.value
}

/**
 * 显式设定主题
 */
function setTheme(mode: ThemeMode) {
  theme.value = mode
  localStorage.setItem(THEME_KEY, mode)
  applyThemeToDOM(mode)
}

export function useTheme() {
  return {
    theme,
    isDark,
    initTheme,
    toggleTheme,
    setTheme
  }
}
