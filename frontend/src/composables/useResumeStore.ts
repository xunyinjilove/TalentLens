import { defineStore } from 'pinia'
import { ref, computed, onMounted } from 'vue'

// 尝试导入 Wails 运行时（可能不存在于纯前端开发模式）
let WailsRuntime: any = null
let WailsApp: any = null
let isWailsEnv = false

// 异步加载 Wails 绑定
async function loadWailsBindings() {
  try {
    WailsRuntime = await import('../../wailsjs/runtime/runtime')
    WailsApp = await import('../../wailsjs/go/main/App')
    isWailsEnv = true
    console.log('✅ Wails 环境已检测到')
  } catch (e) {
    console.log('⚠️ 非 Wails 环境，使用 Mock 模式')
    isWailsEnv = false
  }
}

// 文件信息接口（从 DropZone 传入）
export interface FileInfo {
  name: string
  path: string
  size: number
  type: string
  lastModified: number
}

// 简历类型定义
export interface Resume {
  id: string
  fileName: string
  filePath: string
  fileType: string
  fileSize: number
  content?: string
  status: 'pending' | 'analyzing' | 'done' | 'error'
  score?: number
  errorMessage?: string
  analysis?: {
    overallScore: number
    experienceMatch: number
    skillMatch: number
    educationMatch: number
    skillDetail: string
    experienceDetail: string
    educationDetail: string
    candidateName: string
    workYears: string
    education: string
    currentRole: string
    summary: string
    strengths: string[]
    weaknesses: string[]
    risks: string[]
    recommendation: string
    interviewSuggestions: string[]
    interviewQA?: Array<{ category: string; question: string; reference_answer: string }>
  }
  createdAt: string
  analyzedAt?: string
}

// 开发者日志条目
export interface DevLogEntry {
  time: string
  level: 'info' | 'warn' | 'error'
  message: string
}

// 开发者日志缓冲区（全局共享）
const MAX_LOG_ENTRIES = 200
const _devLogs: DevLogEntry[] = []
let _devLogListeners: Array<() => void> = []

export function getDevLogs(): DevLogEntry[] {
  return _devLogs
}

export function onDevLogChange(fn: () => void) {
  _devLogListeners.push(fn)
  return () => { _devLogListeners = _devLogListeners.filter(f => f !== fn) }
}

export function devLog(level: DevLogEntry['level'], message: string) {
  const entry: DevLogEntry = {
    time: new Date().toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 } as any),
    level,
    message
  }
  _devLogs.push(entry)
  if (_devLogs.length > MAX_LOG_ENTRIES) _devLogs.shift()
  _devLogListeners.forEach(fn => fn())
  
  // 同时输出到控制台
  const prefix = level === 'error' ? '❌' : level === 'warn' ? '⚠️' : '📋'
  console.log(`${prefix} [${entry.time}] ${message}`)
}

// 批量分析进度
export interface BatchProgress {
  current: number
  total: number
  currentResumeId: string | null
}

// 使用 Pinia 创建单例 Store
export const useResumeStore = defineStore('resume', () => {
  // 状态
  const resumes = ref<Resume[]>([])
  const isAnalyzing = ref(false)
  const selectedId = ref<string | null>(null)
  
  // 分析进度状态
  const batchProgress = ref<BatchProgress>({ current: 0, total: 0, currentResumeId: null })

  // 计算属性
  const selectedResume = computed(() => {
    if (!selectedId.value) return null
    return resumes.value.find(r => r.id === selectedId.value) || null
  })

  const pendingCount = computed(() => {
    return resumes.value.filter(r => r.status === 'pending').length
  })

  const doneCount = computed(() => {
    return resumes.value.filter(r => r.status === 'done').length
  })

  // 加载项目下的简历
  async function loadProjectResumes(projectId: string) {
    await loadWailsBindings()
    if (!isWailsEnv || !WailsApp) return
    try {
      const result = await WailsApp.GetProjectResumes(projectId)
      if (result && Array.isArray(result)) {
        resumes.value = result.map((r: any) => ({
          id: r.id,
          projectId: r.project_id || projectId,
          fileName: r.file_name,
          filePath: r.file_path,
          fileType: r.file_type,
          fileSize: r.file_size,
          content: r.content,
          status: r.status as Resume['status'],
          score: r.score,
          analysis: r.analysis ? {
            overallScore: r.analysis.overall_score,
            experienceMatch: r.analysis.experience_match,
            skillMatch: r.analysis.skill_match,
            educationMatch: r.analysis.education_match,
            skillDetail: r.analysis.skill_detail || '',
            experienceDetail: r.analysis.experience_detail || '',
            educationDetail: r.analysis.education_detail || '',
            candidateName: r.analysis.candidate_name || '',
            workYears: r.analysis.work_years || '',
            education: r.analysis.education || '',
            currentRole: r.analysis.current_role || '',
            summary: r.analysis.summary,
            strengths: r.analysis.strengths || [],
            weaknesses: r.analysis.weaknesses || [],
            risks: r.analysis.risks || [],
            recommendation: r.analysis.recommendation,
            interviewSuggestions: r.analysis.interview_suggestions || [],
            interviewQA: r.analysis.interview_qa || []
          } : undefined,
          createdAt: r.created_at
        }))
        devLog('info', `加载项目简历: ${resumes.value.length} 份`)
      }
    } catch (err: any) {
      devLog('error', `加载项目简历失败: ${err.message || err}`)
    }
  }

  // 添加简历（接受 FileInfo 数组）
  async function addResumes(files: FileInfo[], projectId?: string) {
    for (const file of files) {
      // 检查是否已存在相同文件名的简历
      if (resumes.value.some(r => r.fileName === file.name && r.fileSize === file.size)) {
        devLog('warn', `跳过重复文件: ${file.name}`)
        continue
      }

      const ext = '.' + file.name.split('.').pop()?.toLowerCase()
      const id = Date.now().toString() + Math.random().toString(36).substring(2, 11)

      const newResume: Resume = {
        id,
        fileName: file.name,
        filePath: file.path,
        fileType: ext,
        fileSize: file.size,
        status: 'pending',
        createdAt: new Date().toISOString()
      }

      resumes.value.unshift(newResume)
      devLog('info', `添加简历: ${file.name} (id: ${id})`)

      // 在 Wails 环境下同步到后端磁盘
      if (isWailsEnv && WailsApp) {
        try {
          if (projectId) {
            // 注册到项目
            await WailsApp.RegisterResumeToProject(projectId, id, file.name, file.path, ext, file.size)
            devLog('info', `注册到项目: ${projectId}`)
          } else {
            await WailsApp.RegisterResume(id, file.name, file.path, ext, file.size)
            devLog('info', `后端注册完成`)
          }
        } catch (err: any) {
          devLog('error', `后端注册失败: ${err.message || err}`)
        }
      }
    }
  }

  // 删除简历（前端 + 后端同步删除）
  async function deleteResume(id: string) {
    const index = resumes.value.findIndex(r => r.id === id)
    if (index !== -1) {
      resumes.value.splice(index, 1)
      if (selectedId.value === id) {
        selectedId.value = null
      }
    }
    // 同步删除后端磁盘文件
    if (isWailsEnv && WailsApp) {
      try {
        await WailsApp.DeleteResume(id)
        devLog('info', `后端已删除简历: ${id}`)
      } catch (err: any) {
        devLog('error', `后端删除失败: ${err.message || err}`)
      }
    }
  }

  // 选择简历
  function selectResume(id: string | null) {
    selectedId.value = id
  }

  // 重新分析（需要 AI 配置，返回是否成功启动）
  async function reAnalyze(id: string): Promise<boolean> {
    const resume = resumes.value.find(r => r.id === id)
    if (!resume) return false

    // 检查 AI 配置
    const aiConfig = getAIConfig()
    if (!aiConfig || !aiConfig.api_key) {
      devLog('error', '重新分析失败: 未配置 AI Key')
      return false
    }

    resume.status = 'pending'
    resume.score = undefined
    resume.analysis = undefined

    // Wails 环境：调用后端分析
    if (isWailsEnv && WailsApp) {
      const jobConfig = getJobConfig()
      try {
        resume.status = 'analyzing'
        devLog('info', `重新分析简历: ${resume.fileName}`)
        await WailsApp.AnalyzeResume(id, aiConfig, jobConfig)
        // 结果通过 analysis:completed 事件回调更新
      } catch (err: any) {
        devLog('error', `重新分析失败: ${err.message || err}`)
        resume.status = 'error'
      }
      return true
    }

    // 非 Wails 环境 Mock（仅用于开发调试）
    devLog('warn', '非 Wails 环境，使用 Mock 重新分析')
    resume.status = 'analyzing'
    setTimeout(() => {
      resume.status = 'done'
      resume.score = Math.floor(Math.random() * 30) + 70
      resume.analysis = {
        overallScore: resume.score,
        experienceMatch: Math.floor(Math.random() * 20) + 80,
        skillMatch: Math.floor(Math.random() * 25) + 75,
        educationMatch: Math.floor(Math.random() * 15) + 85,
        skillDetail: 'Mock 模式',
        experienceDetail: 'Mock 模式',
        educationDetail: 'Mock 模式',
        candidateName: resume.fileName.replace(/\.[^/.]+$/, ''),
        workYears: '3年',
        education: '本科',
        currentRole: '开发工程师',
        summary: 'Mock 模式分析完成',
        strengths: ['技能匹配', '经验丰富'],
        weaknesses: ['可提升空间'],
        risks: [],
        recommendation: 'recommend',
        interviewSuggestions: []
      }
      resume.analyzedAt = new Date().toISOString()
    }, 1500)
    return true
  }

  // 模拟的分析结果模板
  const mockStrengths = [
    '技术栈与岗位要求高度匹配',
    '有丰富的项目经验',
    '学历背景符合要求',
    '具备良好的沟通能力',
    '有团队管理经验',
    '掌握核心技能',
    '工作经验丰富',
    '有相关行业背景'
  ]

  const mockWeaknesses = [
    '部分技能需要加强',
    '项目经验略显不足',
    '缺少某些加分技能',
    '工作年限稍短',
    '缺乏大型项目经验',
    '需要补充相关认证'
  ]

  // 获取 AI 配置
  function getAIConfig() {
    const saved = localStorage.getItem('goresume_settings')
    if (!saved) return null
    try {
      const settings = JSON.parse(saved)
      return {
        provider: settings.ai?.provider || 'deepseek',
        base_url: settings.ai?.baseURL || 'https://api.deepseek.com/v1',
        api_key: settings.ai?.apiKey || '',
        model: settings.ai?.model || 'deepseek-chat',
        max_retries: 3,
        timeout: 60
      }
    } catch {
      return null
    }
  }

  // 获取岗位配置
  function getJobConfig() {
    const saved = localStorage.getItem('goresume_settings')
    if (!saved) {
      return {
        title: '高级开发工程师',
        requirements: ['熟悉相关技术栈', '有团队协作经验'],
        required_skills: ['开发', '设计', '测试'],
        experience_years: 3,
        education_level: '本科',
        job_description: ''
      }
    }
    try {
      const settings = JSON.parse(saved)
      return {
        title: settings.job?.title || '高级开发工程师',
        requirements: settings.job?.requirements || ['熟悉相关技术栈'],
        required_skills: settings.job?.requiredSkills || ['开发'],
        experience_years: settings.job?.experienceYears || 3,
        education_level: settings.job?.educationLevel || '本科',
        job_description: settings.job?.jobDescription || ''
      }
    } catch {
      return {
        title: '高级开发工程师',
        requirements: [],
        required_skills: [],
        experience_years: 3,
        education_level: '本科',
        job_description: ''
      }
    }
  }

  // 开始分析所有待分析的简历
  async function startAnalysis() {
    if (isAnalyzing.value) return

    isAnalyzing.value = true

    const pendingResumes = resumes.value.filter(r => r.status === 'pending')

    // 如果在 Wails 环境下，使用后端 API
    if (isWailsEnv && WailsApp) {
      const aiConfig = getAIConfig()
      const jobConfig = getJobConfig()

      if (!aiConfig || !aiConfig.api_key) {
        devLog('error', 'AI 配置无效或缺少 API Key')
        isAnalyzing.value = false
        return
      }

      // 使用 Wails 后端批量分析
      const resumeIds = pendingResumes.map(r => r.id)
      devLog('info', `启动后端批量分析: ${resumeIds.length} 份简历`)
      try {
        // 如果有项目上下文，使用项目分析（自动使用项目岗位配置）
        const firstResume = pendingResumes[0]
        const projId = (firstResume as any).projectId
        if (projId) {
          await WailsApp.StartProjectAnalysis(projId, aiConfig)
          devLog('info', `项目分析已启动: ${projId}`)
        } else {
          await WailsApp.StartBatchAnalysis(resumeIds, aiConfig, jobConfig)
          devLog('info', 'StartBatchAnalysis 调用成功')
        }
      } catch (err: any) {
        devLog('error', `启动批量分析失败: ${err.message || err}`)
        isAnalyzing.value = false
      }
      // 注意：isAnalyzing 会在接收到 batch:completed 事件时设置为 false
      return
    }

    // 非 Wails 环境，使用 Mock 模式
    const total = pendingResumes.length
    for (let i = 0; i < pendingResumes.length; i++) {
      const resume = pendingResumes[i]
      
      // 更新批量进度
      batchProgress.value = { current: i + 1, total, currentResumeId: resume.id }
      
      resume.status = 'analyzing'
      ;(resume as any).progress = 10

      // 模拟进度阶段
      await new Promise(resolve => setTimeout(resolve, 500))
      ;(resume as any).progress = 30
      
      await new Promise(resolve => setTimeout(resolve, 800))
      ;(resume as any).progress = 70
      
      await new Promise(resolve => setTimeout(resolve, 500))
      ;(resume as any).progress = 100

      resume.status = 'done'
      resume.score = Math.floor(Math.random() * 35) + 60
      
      const expMatch = Math.floor(Math.random() * 25) + 70
      const skillMatch = Math.floor(Math.random() * 30) + 65
      const eduMatch = Math.floor(Math.random() * 20) + 75

      // 随机选择优势和不足
      const shuffledStrengths = [...mockStrengths].sort(() => Math.random() - 0.5)
      const shuffledWeaknesses = [...mockWeaknesses].sort(() => Math.random() - 0.5)

      resume.analysis = {
        overallScore: resume.score,
        experienceMatch: expMatch,
        skillMatch: skillMatch,
        educationMatch: eduMatch,
        skillDetail: '技能匹配分析详情 (Mock 模式)',
        experienceDetail: '经验匹配分析详情 (Mock 模式)',
        educationDetail: '学历匹配分析详情 (Mock 模式)',
        candidateName: resume.fileName.replace(/\.[^/.]+$/, ''),
        workYears: '3年',
        education: '本科',
        currentRole: '开发工程师',
        summary: `该候选人（${resume.fileName.replace(/\.[^/.]+$/, '')}）整体表现${resume.score >= 80 ? '优秀' : resume.score >= 70 ? '良好' : '一般'}。技能匹配度${skillMatch}%，工作经验匹配度${expMatch}%。${resume.score >= 75 ? '建议安排面试进一步了解。' : '可根据实际情况考虑是否进入下一轮。'}`,
        strengths: shuffledStrengths.slice(0, 2 + Math.floor(Math.random() * 2)),
        weaknesses: shuffledWeaknesses.slice(0, 1 + Math.floor(Math.random() * 2)),
        risks: ['Mock 模式暂无风险分析'],
        recommendation: resume.score >= 85 ? 'strong_recommend' : 
                       resume.score >= 70 ? 'recommend' : 
                       resume.score >= 55 ? 'consider' : 'not_recommend',
        interviewSuggestions: ['Mock 模式暂无面试建议']
      }
      resume.analyzedAt = new Date().toISOString()
    }

    isAnalyzing.value = false
    batchProgress.value = { current: 0, total: 0, currentResumeId: null }
  }

  // 批量重新分析所有简历（将 done/error 状态重置为 pending，然后启动分析）
  async function reAnalyzeAll(): Promise<boolean> {
    if (isAnalyzing.value) return false

    const aiConfig = getAIConfig()
    if (!aiConfig || !aiConfig.api_key) {
      devLog('error', '批量重分析失败: 未配置 AI Key')
      return false
    }

    // 将所有 done/error 简历重置为 pending
    let resetCount = 0
    for (const resume of resumes.value) {
      if (resume.status === 'done' || resume.status === 'error') {
        resume.status = 'pending'
        resume.score = undefined
        resume.analysis = undefined
        resetCount++
      }
    }

    if (resetCount === 0) {
      devLog('warn', '没有需要重新分析的简历')
      return false
    }

    devLog('info', `已重置 ${resetCount} 份简历，准备重新分析`)

    // 启动分析（startAnalysis 会处理所有 pending 简历）
    await startAnalysis()
    return true
  }

  // 清空所有简历（前端 + 后端同步清空）
  async function clearAll() {
    resumes.value = []
    selectedId.value = null
    // 同步清空后端磁盘
    if (isWailsEnv && WailsApp) {
      try {
        await WailsApp.ClearResumes()
        devLog('info', '后端已清空所有简历')
      } catch (err: any) {
        devLog('error', `后端清空失败: ${err.message || err}`)
      }
    }
  }

  // 从后端数据构建简历并添加到列表
  function addResumeFromBackend(data: any) {
    const newResume: Resume = {
      id: data.id,
      fileName: data.file_name,
      filePath: data.file_path,
      fileType: data.file_type,
      fileSize: data.file_size,
      content: data.content,
      status: (data.status as Resume['status']) || 'pending',
      score: data.score,
      createdAt: data.created_at
    }
    if (!resumes.value.some(r => r.id === newResume.id)) {
      resumes.value.unshift(newResume)
    }
  }

  // 初始化 Wails 事件监听
  async function initWailsEvents() {
    await loadWailsBindings()
    
    if (!isWailsEnv || !WailsRuntime) {
      console.log('⚠️ 跳过 Wails 事件监听初始化')
      return
    }

    // 监听简历添加事件（后端 processFile 或 OnFileDrop）
    WailsRuntime.EventsOn('resume:added', (data: any) => {
      devLog('info', `收到后端简历添加事件: ${data.file_name}`)
      addResumeFromBackend(data)
    })

    // 监听原生拖拽/文件选择添加事件
    WailsRuntime.EventsOn('resume:dropped', (data: any) => {
      devLog('info', `收到原生文件事件: ${data.file_name}, 内容长度=${(data.content || '').length}`)
      addResumeFromBackend(data)
    })

    // 监听分析进度事件（含进度百分比）
    WailsRuntime.EventsOn('analysis:progress', (data: any) => {
      devLog('info', `分析进度: id=${data.id}, progress=${data.progress}%`)
      const resume = resumes.value.find(r => r.id === data.id)
      if (resume) {
        resume.status = data.status || 'analyzing'
        // 存储进度到 resume 对象上（动态属性）
        ;(resume as any).progress = data.progress || 0
      }
    })

    // 监听分析完成事件
    WailsRuntime.EventsOn('analysis:completed', (data: any) => {
      devLog('info', `分析完成: id=${data.id}, score=${data.score}`)
      const resume = resumes.value.find(r => r.id === data.id)
      if (resume) {
        resume.status = 'done'
        resume.score = data.score
        const a = data.analysis
        resume.analysis = {
          overallScore: a.overall_score,
          experienceMatch: a.experience_match,
          skillMatch: a.skill_match,
          educationMatch: a.education_match,
          skillDetail: a.skill_detail || '',
          experienceDetail: a.experience_detail || '',
          educationDetail: a.education_detail || '',
          candidateName: a.candidate_name || '',
          workYears: a.work_years || '',
          education: a.education || '',
          currentRole: a.current_role || '',
          summary: a.summary,
          strengths: a.strengths || [],
          weaknesses: a.weaknesses || [],
          risks: a.risks || [],
          recommendation: a.recommendation,
          interviewSuggestions: a.interview_suggestions || [],
          interviewQA: a.interview_qa || []
        }
        resume.analyzedAt = data.analysis.analyzed_at
      }
    })

    // 监听分析错误事件
    WailsRuntime.EventsOn('analysis:error', (data: any) => {
      devLog('error', `分析失败: id=${data.id}, error=${data.error}`)
      const resume = resumes.value.find(r => r.id === data.id)
      if (resume) {
        resume.status = 'error'
        resume.errorMessage = data.error || '未知错误'
      }
    })

    // 监听批量分析进度（更新全局进度状态）
    WailsRuntime.EventsOn('batch:progress', (data: any) => {
      devLog('info', `批量进度: ${data.current}/${data.total}`)
      batchProgress.value = {
        current: data.current,
        total: data.total,
        currentResumeId: data.resumeId || null
      }
    })

    // 监听批量分析完成
    WailsRuntime.EventsOn('batch:completed', (data: any) => {
      devLog('info', `批量分析完成, 共 ${data.total} 份`)
      isAnalyzing.value = false
      batchProgress.value = { current: 0, total: 0, currentResumeId: null }
    })

    console.log('✅ Wails 事件监听已初始化')
  }

  // 获取简历解析内容（AI 实际看到的文本）
  async function getResumeContent(id: string): Promise<string> {
    if (isWailsEnv && WailsApp) {
      try {
        const content = await WailsApp.GetFreshResumeContent(id)
        // GetFreshResumeContent 返回 [string, error]，处理两种情况
        if (Array.isArray(content)) {
          return content[0] || ''
        }
        return content || ''
      } catch (err: any) {
        devLog('error', `获取简历内容失败: ${err.message || err}`)
        return ''
      }
    }
    // 回退到前端已有内容
    const resume = resumes.value.find(r => r.id === id)
    return resume?.content || '(非 Wails 环境，无法获取解析内容)'
  }

  // 检查是否为 Wails 环境
  function isWailsEnvironment() {
    return isWailsEnv
  }

  return {
    // 状态
    resumes,
    isAnalyzing,
    selectedId,
    batchProgress,
    // 计算属性
    selectedResume,
    pendingCount,
    doneCount,
    // 方法
    addResumes,
    loadProjectResumes,
    deleteResume,
    selectResume,
    reAnalyze,
    reAnalyzeAll,
    startAnalysis,
    clearAll,
    getResumeContent,
    initWailsEvents,
    isWailsEnvironment
  }
})
