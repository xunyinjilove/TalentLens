import { defineStore } from 'pinia'
import { ref } from 'vue'
import router from '../router'

export interface AssistantMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: number
  error?: boolean
}

export interface JobApplyData {
  title: string
  department?: string
  headcount?: number
  experience_years?: number
  education_level?: string
  job_description?: string
  required_skills?: string[]
  interview_questions?: string[]
  red_lines?: string[]
  bonus_points?: string[]
}

const STORAGE_KEY = 'talentlens_ai_assistant_chat_history'

// 尝试异步导入 Wails 绑定
let WailsApp: any = null
let isWailsEnv = false

async function loadWails() {
  if (WailsApp) return
  try {
    WailsApp = await import('../../wailsjs/go/main/App')
    isWailsEnv = true
  } catch {
    isWailsEnv = false
  }
}

export const useAIAssistantStore = defineStore('aiAssistant', () => {
  const isOpen = ref(false)
  const loading = ref(false)
  const messages = ref<AssistantMessage[]>([])
  const pendingJobToApply = ref<JobApplyData | null>(null)

  // 初始化从 localStorage 恢复聊天记录
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      messages.value = JSON.parse(saved)
    }
  } catch {
    messages.value = []
  }

  function saveMessages() {
    try {
      // 最多持久化最近 40 条对话
      const slice = messages.value.slice(-40)
      localStorage.setItem(STORAGE_KEY, JSON.stringify(slice))
    } catch {}
  }

  function toggleDrawer() {
    isOpen.value = !isOpen.value
  }

  function openDrawer() {
    isOpen.value = true
  }

  function closeDrawer() {
    isOpen.value = false
  }

  function clearMessages() {
    messages.value = []
    try {
      localStorage.removeItem(STORAGE_KEY)
    } catch {}
  }

  // 获取系统设置中保存的大模型配置
  function getAIConfig() {
    const saved = localStorage.getItem('goresume_settings')
    if (!saved) return null
    try {
      const settings = JSON.parse(saved)
      if (!settings.ai) return null
      return {
        provider: settings.ai.provider || 'deepseek',
        base_url: settings.ai.baseURL || settings.ai.base_url || 'https://api.deepseek.com/v1',
        api_key: settings.ai.apiKey || settings.ai.api_key || '',
        model: settings.ai.model || 'deepseek-chat',
        max_retries: 3,
        timeout: 60
      }
    } catch {
      return null
    }
  }

  async function sendMessage(userContent: string, currentContext: string = '') {
    const trimmed = userContent.trim()
    if (!trimmed || loading.value) return

    const userMsg: AssistantMessage = {
      id: 'msg_' + Date.now() + '_' + Math.random().toString(36).slice(2, 7),
      role: 'user',
      content: trimmed,
      timestamp: Date.now()
    }
    messages.value.push(userMsg)
    saveMessages()

    loading.value = true

    try {
      await loadWails()

      let replyContent = ''
      if (isWailsEnv && WailsApp && typeof WailsApp.ChatWithAIAssistant === 'function') {
        // 构建转换格式给 Go 后端
        const reqPayload = messages.value.filter(m => !m.error).map(m => ({
          role: m.role,
          content: m.content
        }))
        const aiConfig = getAIConfig()
        replyContent = await WailsApp.ChatWithAIAssistant(reqPayload, currentContext, aiConfig)
      } else {
        // 非 Wails 浏览器预览 Mock 模式
        await new Promise(r => setTimeout(r, 600))
        if (trimmed.includes('设置') || trimmed.includes('配置')) {
          replyContent = '### 💡 AI 配置指南\n\n您可以在「系统设置」中配置 API Key 与 Base URL。\n\n[ACTION:NAVIGATE|/settings|前往系统设置]'
        } else {
          replyContent = `### 📋 岗位招聘方案\n\n已为您规划 **${trimmed}** 的核心职责与要求：\n\n` +
            '```json:talentlens-job\n' +
            '{\n' +
            `  "title": "${trimmed}",\n` +
            '  "department": "研发中心",\n' +
            '  "headcount": 1,\n' +
            '  "experience_years": 3,\n' +
            '  "education_level": "本科",\n' +
            '  "job_description": "负责该岗位的方案制定与落地执行，保障项目高质量交付。",\n' +
            '  "required_skills": ["专业技能", "沟通协作", "方案落地"],\n' +
            '  "red_lines": ["工作年限不达标淘汰", "学历不符淘汰"],\n' +
            '  "bonus_points": ["有标杆企业经验优先"]\n' +
            '}\n' +
            '```\n\n' +
            '[ACTION:OPEN_CREATE_PROJECT|新建招聘项目] [ACTION:NAVIGATE|/|查看项目列表]'
        }
      }

      const assistantMsg: AssistantMessage = {
        id: 'msg_' + Date.now() + '_' + Math.random().toString(36).slice(2, 7),
        role: 'assistant',
        content: replyContent,
        timestamp: Date.now()
      }
      messages.value.push(assistantMsg)
      saveMessages()
    } catch (err: any) {
      const reason = err?.message || String(err)
      const errMsg: AssistantMessage = {
        id: 'msg_err_' + Date.now(),
        role: 'assistant',
        content: reason,
        timestamp: Date.now(),
        error: true
      }
      messages.value.push(errMsg)
      saveMessages()
    } finally {
      loading.value = false
    }
  }

  // 一键将职位数据应用到新建项目中
  function applyJobToProject(jobData: JobApplyData) {
    pendingJobToApply.value = jobData
    // 路由跳转到项目列表页
    if (router.currentRoute.value.path !== '/') {
      router.push('/')
    }
    // 触发全局自定义事件通知 ProjectListView
    window.dispatchEvent(new CustomEvent('talentlens:apply-ai-job', { detail: jobData }))
  }

  return {
    isOpen,
    loading,
    messages,
    pendingJobToApply,
    toggleDrawer,
    openDrawer,
    closeDrawer,
    clearMessages,
    sendMessage,
    applyJobToProject
  }
})
