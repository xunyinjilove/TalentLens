import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ElNotification } from 'element-plus'

export interface TalentCandidate {
  id: string
  projectCandidateId: string
  projectId: string
  projectName: string
  jobTitle: string
  name: string
  source: string
  resumeId: string
  resumeFileName: string
  profileUrl: string
  analysisStatus: string
  abilityScore: number
  matchScore: number
  rankEligible: boolean
  radarStale: boolean
  currentRole: string
  education?: string
  workYears?: string
  skills: string[]
  redLineStatus: string
  confirmedBonus: number
  stage: string
  owner: string
  nextActionAt: string
  createdAt: string
  lastFollowUpAt: string
  hiredAt: string
}

export interface RecruitmentDashboard {
  openProjects: number
  candidatesThisWeek: number
  pendingFollowUps: number
  upcomingInterviews: number
  pendingInterviewArrangements: number
  hiresThisMonth: number
  stageCounts: Record<string, number>
}

export interface TodayTodo {
  id: string
  title: string
  completed: boolean
  createdAt: string
  completedAt: string
}

export interface RecruitmentActivity {
  id: string
  projectCandidateId: string
  projectId: string
  candidateName: string
  projectName: string
  kind: 'followup' | 'interview'
  eventType: string
  toStage: string
  note: string
  occurredAt: string
  startsAt: string
  method: string
}

export interface RecruitmentInterview {
  id: string
  projectCandidateId: string
  candidateName: string
  projectName: string
  startsAt: string
  endsAt: string
  interviewer: string
  method: string
  location: string
  note: string
  status: string
}

export interface SourcingTask {
  id: string
  projectId: string
  projectName: string
  status: string
  targetCount: number
  capturedCount: number
  analyzedCount: number
  failedCount: number
  currentPlatform: string
  message: string
  createdAt: string
  updatedAt: string
  config: { keyword: string; platforms: string[]; autoAnalyze: boolean }
}

export const stageLabels: Record<string, string> = {
  new: '新入库', pending_analysis: '待评估', pending_contact: '待联系',
  communicating: '沟通中', first_interview: '初面', second_interview: '复面',
  offer: 'Offer', hired: '入职', on_hold: '暂缓', rejected: '淘汰'
}

const emptyDashboard = (): RecruitmentDashboard => ({
  openProjects: 0, candidatesThisWeek: 0, pendingFollowUps: 0,
  upcomingInterviews: 0, pendingInterviewArrangements: 0, hiresThisMonth: 0, stageCounts: {}
})

async function recruitmentAPI(): Promise<any> {
  if (!(window as any).go?.main?.App) {
    throw new Error('当前为浏览器界面预览；请在 TalentLens 桌面客户端查看本地招聘数据')
  }
  return await import('../../wailsjs/go/main/App')
}

export const useRecruitmentStore = defineStore('recruitment', () => {
  const candidates = ref<TalentCandidate[]>([])
  const interviews = ref<RecruitmentInterview[]>([])
  const sourcingTasks = ref<SourcingTask[]>([])
  const taskCenterOpen = ref(false)
  const dismissedSourcingTaskIds = new Set<string>()
  const dashboard = ref<RecruitmentDashboard>(emptyDashboard())
  const todayTodos = ref<TodayTodo[]>([])
  const recentActivities = ref<RecruitmentActivity[]>([])
  const loading = ref(false)
  const error = ref('')
  let subscribed = false
  let refreshTimer: ReturnType<typeof setTimeout> | undefined

  async function loadCandidates(projectId = '') {
    loading.value = true
    error.value = ''
    try {
      const api: any = await recruitmentAPI()
      candidates.value = await api.ListTalentCandidates(projectId) || []
    } catch (err: any) {
      error.value = err?.message || String(err)
    } finally {
      loading.value = false
    }
  }

  async function loadDashboard() {
    try {
      const api: any = await recruitmentAPI()
      dashboard.value = await api.GetRecruitmentDashboard() || emptyDashboard()
    } catch (err: any) {
      error.value = err?.message || String(err)
    }
  }

  async function loadTodayTodos() {
    try {
      const api: any = await recruitmentAPI()
      todayTodos.value = await api.ListTodayTodos() || []
    } catch (err: any) {
      error.value = err?.message || String(err)
    }
  }

  async function loadRecentActivities() {
    try {
      const api: any = await recruitmentAPI()
      recentActivities.value = await api.ListRecentRecruitmentActivities(8) || []
    } catch (err: any) {
      error.value = err?.message || String(err)
    }
  }

  async function addTodayTodo(title: string) {
    const api: any = await recruitmentAPI()
    await api.AddTodayTodo(title)
    await loadTodayTodos()
  }

  async function setTodayTodoCompleted(id: string, completed: boolean) {
    const api: any = await recruitmentAPI()
    await api.SetTodayTodoCompleted(id, completed)
    await loadTodayTodos()
  }

  async function deleteTodayTodo(id: string) {
    const api: any = await recruitmentAPI()
    await api.DeleteTodayTodo(id)
    await loadTodayTodos()
  }

  async function loadInterviews(projectId = '') {
    try {
      const api: any = await recruitmentAPI()
      interviews.value = await api.ListRecruitmentInterviews(projectId) || []
    } catch (err: any) {
      error.value = err?.message || String(err)
    }
  }

  async function loadSourcingTasks() {
    try {
      const api: any = await recruitmentAPI()
      sourcingTasks.value = (await api.ListSourcingTasks('') || []).filter((task: SourcingTask) => !dismissedSourcingTaskIds.has(task.id))
    } catch (err: any) {
      error.value = err?.message || String(err)
    }
  }

  async function resumeSourcingTask(id: string) {
    const api: any = await recruitmentAPI()
    await api.ResumeSourcingTask(id)
    await loadSourcingTasks()
  }

  async function stopSourcingTask() {
    const api: any = await recruitmentAPI()
    await api.StopBossSearch()
  }

  async function deleteSourcingTask(id: string) {
    const api: any = await recruitmentAPI()
    await api.DeleteSourcingTask(id)
    dismissedSourcingTaskIds.add(id)
    sourcingTasks.value = sourcingTasks.value.filter(task => task.id !== id)
  }

  async function clearFinishedSourcingTasks() {
    const api: any = await recruitmentAPI()
    const removed: number = await api.ClearFinishedSourcingTasks()
    for (const task of sourcingTasks.value) {
      if (!['queued', 'running'].includes(task.status)) dismissedSourcingTaskIds.add(task.id)
    }
    await loadSourcingTasks()
    return removed
  }

  async function updateFollowUp(item: TalentCandidate, stage: string, owner: string, nextActionAt: string, note: string) {
    const api: any = await recruitmentAPI()
    await api.UpdateCandidateFollowUp(item.projectCandidateId, stage, owner, nextActionAt, note, 'HR')
    await Promise.all([loadCandidates(), loadDashboard(), loadRecentActivities()])
  }

  async function batchUpdateStage(projectCandidateIds: string[], stage: string, note = '') {
    const api: any = await recruitmentAPI()
    if (typeof api.BatchUpdateCandidateStage === 'function') {
      await api.BatchUpdateCandidateStage(projectCandidateIds, stage, note, 'HR')
    } else {
      for (const id of projectCandidateIds) {
        await api.UpdateCandidateFollowUp(id, stage, '', '', note, 'HR')
      }
    }
    await Promise.all([loadCandidates(), loadDashboard(), loadRecentActivities()])
  }

  async function saveInterview(item: RecruitmentInterview) {
    const api: any = await recruitmentAPI()
    await api.SaveRecruitmentInterview(item)
    await Promise.all([loadInterviews(), loadDashboard(), loadRecentActivities()])
  }

  async function cancelInterview(id: string) {
    const api: any = await recruitmentAPI()
    await api.CancelRecruitmentInterview(id)
    await Promise.all([loadInterviews(), loadDashboard(), loadRecentActivities()])
  }

  async function moveCandidate(projectCandidateId: string, targetProjectId: string) {
    const api: any = await recruitmentAPI()
    await api.MoveCandidateToProject(projectCandidateId, targetProjectId)
    await Promise.all([loadCandidates(), loadDashboard()])
  }

  async function archiveCandidates(projectCandidateIds: string[]) {
    const api: any = await recruitmentAPI()
    const removed: number = await api.ArchiveTalentCandidates(projectCandidateIds)
    await Promise.all([loadCandidates(), loadDashboard()])
    return removed
  }

  async function initEvents() {
    if (subscribed) return
    subscribed = true
    try {
      const runtime = await import('../../wailsjs/runtime/runtime')
      const schedule = () => {
        if (refreshTimer) clearTimeout(refreshTimer)
        refreshTimer = setTimeout(() => {
          void loadCandidates()
          void loadDashboard()
        }, 200)
      }
      for (const event of ['resume:dropped', 'resume:added', 'resume:updated', 'analysis:completed', 'analysis:error', 'analysis:updated', 'platform:candidate_found']) {
        runtime.EventsOn(event, schedule)
      }
      runtime.EventsOn('sourcing:updated', (task: SourcingTask) => {
        if (dismissedSourcingTaskIds.has(task.id)) return
        const index = sourcingTasks.value.findIndex(item => item.id === task.id)
        const priorStatus = index >= 0 ? sourcingTasks.value[index].status : ''
        if (index < 0) sourcingTasks.value.unshift(task)
        else sourcingTasks.value[index] = task
        if (priorStatus === 'running' && ['completed', 'partial', 'failed', 'stopped'].includes(task.status)) {
          ElNotification({ title: `${task.projectName} · 寻才任务更新`, message: task.message || task.status,
            type: task.status === 'completed' ? 'success' : task.status === 'failed' ? 'error' : 'warning', duration: 6000 })
        }
      })
    } catch {
      // The standalone Vite preview has no Wails runtime.
    }
  }

  return { candidates, interviews, dashboard, todayTodos, recentActivities, sourcingTasks, taskCenterOpen, loading, error, loadCandidates,
    loadDashboard, loadTodayTodos, loadRecentActivities, addTodayTodo, setTodayTodoCompleted, deleteTodayTodo,
    loadInterviews, loadSourcingTasks, resumeSourcingTask, stopSourcingTask,
    deleteSourcingTask, clearFinishedSourcingTasks,
    updateFollowUp, batchUpdateStage, saveInterview, cancelInterview, moveCandidate, archiveCandidates, initEvents }
})
