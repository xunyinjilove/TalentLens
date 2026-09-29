import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import zhTW from './zh-TW'
import enUS from './en-US'

// 语言配置
export const LOCALE_OPTIONS = [
  { value: 'zh-CN', label: '简体中文', flag: '🇨🇳' },
  { value: 'zh-TW', label: '繁體中文', flag: '🇹🇼' },
  { value: 'en-US', label: 'English', flag: '🇺🇸' }
]

type LocaleCode = 'zh-CN' | 'zh-TW' | 'en-US'

// 获取默认语言
function getDefaultLocale(): LocaleCode {
  // 优先从 localStorage 读取
  const saved = localStorage.getItem('goresume_locale')
  if (saved && LOCALE_OPTIONS.some(opt => opt.value === saved)) {
    return saved as LocaleCode
  }
  
  // 其次根据浏览器语言判断
  const browserLang = navigator.language
  if (browserLang.startsWith('zh')) {
    // 简体/繁体判断
    if (browserLang === 'zh-TW' || browserLang === 'zh-HK') {
      return 'zh-TW'
    }
    return 'zh-CN'
  }
  
  // 默认英语
  if (browserLang.startsWith('en')) {
    return 'en-US'
  }
  
  // 最终默认简体中文
  return 'zh-CN'
}

// 创建 i18n 实例
const i18n = createI18n({
  legacy: false, // 使用 Composition API 模式
  locale: getDefaultLocale(),
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'zh-TW': zhTW,
    'en-US': enUS
  }
})

// 切换语言函数
export function setLocale(locale: LocaleCode) {
  if (LOCALE_OPTIONS.some(opt => opt.value === locale)) {
    i18n.global.locale.value = locale
    localStorage.setItem('goresume_locale', locale)
    // 同时更新 Element Plus 语言
    document.documentElement.lang = locale
  }
}

// 获取当前语言
export function getLocale(): string {
  return i18n.global.locale.value
}

export default i18n
