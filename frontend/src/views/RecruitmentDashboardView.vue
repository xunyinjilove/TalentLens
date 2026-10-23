<template>
  <div class="workbench-page">
    <header class="wb-header">
      <div><h1>招聘概览</h1><div class="wb-crumb">工作台 / 整体招聘态势</div></div>
    </header>
    <main class="wb-content">
      <el-alert v-if="recruitment.error" type="warning" :title="recruitment.error" show-icon :closable="false" style="margin-bottom:18px" />
      <div class="wb-kpi-grid">
        <div v-for="card in kpis" :key="card.label" class="wb-card wb-kpi" role="button" tabindex="0" @click="router.push(card.route)" @keydown.enter="router.push(card.route)">
          <div class="wb-kpi-icon"><el-icon v-if="card.iconComponent"><component :is="card.iconComponent" /></el-icon><span v-else>{{ card.icon }}</span></div>
          <div class="wb-kpi-value">{{ card.value }}</div>
          <div class="wb-kpi-label">{{ card.label }}</div>
        </div>
      </div>
      <section class="wb-card overview-board" :class="{ 'is-collapsed': !boardExpanded }" aria-labelledby="overview-board-title">
        <div class="overview-board-heading">
          <div>
            <h2 id="overview-board-title" class="wb-section-title">候选人跟进看板</h2>
          </div>
          <div class="overview-board-controls">
            <button type="button" class="overview-board-toggle" :aria-expanded="boardExpanded" aria-controls="overview-board-content" @click="boardExpanded = !boardExpanded">
              {{ boardExpanded ? '收起看板' : '展开看板' }} <span aria-hidden="true">{{ boardExpanded ? '⌃' : '⌄' }}</span>
            </button>
            <el-select v-if="boardExpanded" v-model="selectedProjectId" aria-label="筛选招聘职位" class="overview-project-select" placeholder="全部职位">
              <el-option label="全部职位" value="" />
              <el-option v-for="project in sortedProjects" :key="project.id" :label="project.job_config?.title && project.job_config.title !== project.name ? `${project.job_config.title} · ${project.name}` : project.name" :value="project.id" />
            </el-select>
          </div>
        </div>
        <div id="overview-board-content" v-show="boardExpanded">
        <div v-if="!sortedProjects.length" class="overview-board-empty">
          <p>尚未创建职位，创建后即可在这里查看该职位的候选人进展。</p>
          <el-button type="primary" @click="router.push('/jobs')">前往职位管理</el-button>
        </div>
        <div v-else class="overview-board-scroll" tabindex="0" aria-label="候选人阶段看板，横向滚动查看更多阶段">
          <div class="overview-board-columns">
            <section v-for="column in boardColumns" :key="column.key" class="overview-board-column" :class="`overview-board-column-${column.key}`">
              <header class="overview-column-heading">
                <span class="overview-column-name"><span class="overview-column-dot" aria-hidden="true" />{{ column.label }}</span>
                <span class="overview-column-count" :aria-label="`${column.total} 位候选人`">{{ column.total }}</span>
              </header>
              <div class="overview-column-cards">
                <button v-for="item in column.items" :key="item.projectCandidateId" class="overview-candidate-card" type="button" :aria-label="`查看${item.name}的跟进记录`" @click="openFollowUp(item)">
                  <span class="overview-candidate-main">
                    <span class="overview-candidate-avatar" aria-hidden="true">{{ getCandidateSurname(item) }}</span>
                    <span class="overview-candidate-identity"><strong>{{ item.name }}</strong><small>{{ item.currentRole || item.jobTitle || item.projectName }}<template v-if="item.workYears"> · {{ item.workYears }}</template></small></span>
                  </span>
                  <span v-if="!selectedProjectId" class="overview-candidate-project">{{ item.jobTitle || item.projectName }}</span>
                  <span v-if="item.skills?.length" class="overview-skill-list"><span v-for="skill in item.skills.slice(0, 2)" :key="skill" class="overview-skill">{{ skill }}</span></span>
                  <span class="overview-candidate-footer">
                    <span class="overview-candidate-score">{{ item.rankEligible ? `岗位 ${item.abilityScore}` : item.radarStale ? '待复核' : '待核实' }}</span>
                    <span class="overview-candidate-time" :title="item.lastFollowUpAt || item.createdAt">{{ activityLabel(item) }}</span>
                  </span>
                  <span class="overview-candidate-stage">{{ stageLabels[item.stage] || item.stage }}</span>
                </button>
                <p v-if="!column.total" class="overview-column-empty">暂无人选</p>
              </div>
              <button v-if="column.total" class="overview-column-more" type="button" @click="openStage(column.firstStage)">查看全部 {{ column.total }} 人</button>
            </section>
          </div>
        </div>
        </div>
      </section>
      <div class="wb-two-col">
        <section class="wb-card">
          <h2 class="wb-section-title">招聘漏斗</h2>
          <div v-if="funnel.length" class="funnel-list">
            <div v-for="step in funnel" :key="step.label" class="funnel-line">
              <span>{{ step.label }}</span>
              <div class="funnel-track"><div class="funnel-fill" :style="{ width: step.width + '%' }" /></div>
              <strong>{{ step.count }}</strong>
            </div>
          </div>
          <div v-else class="wb-empty">暂无候选人跟进数据</div>
        </section>
        <section class="wb-card">
          <h2 class="wb-section-title">近期动态</h2>
          <div v-if="recruitment.recentActivities.length" class="recent-activity-list">
            <button v-for="activity in recruitment.recentActivities" :key="`${activity.kind}-${activity.id}`" type="button" class="recent-activity-item" @click="openActivity(activity)">
              <span class="recent-activity-icon" :class="{ 'is-interview': activity.kind === 'interview' }" aria-hidden="true"><el-icon><component :is="activity.kind === 'interview' ? Calendar : ChatDotRound" /></el-icon></span>
              <span class="recent-activity-copy"><strong>{{ activity.candidateName }} · {{ activityTitle(activity) }}</strong><small>{{ activity.projectName }} · {{ activityDetail(activity) }}</small></span>
              <time class="recent-activity-time" :datetime="activity.occurredAt">{{ activityTime(activity.occurredAt) }}</time>
            </button>
          </div>
          <div v-else class="wb-empty">暂无面试或跟进动态</div>
        </section>
      </div>
      <section class="wb-card today-todo-card" aria-labelledby="today-todo-title">
        <div class="today-todo-heading">
          <div><h2 id="today-todo-title" class="wb-section-title">今日待办</h2><span class="wb-muted">{{ pendingTodoCount }} 项未完成</span></div>
          <span class="wb-muted">完成后勾选，未完成的任务会保留到明天</span>
        </div>
        <form class="today-todo-add" @submit.prevent="addTodo">
          <el-input v-model="newTodoTitle" maxlength="200" placeholder="添加一项招聘待办，例如联系候选人或准备面试" aria-label="新增待办内容" />
          <el-button type="primary" native-type="submit" :loading="todoBusy" :disabled="!newTodoTitle.trim()">添加待办</el-button>
        </form>
        <div v-if="recruitment.todayTodos.length" class="today-todo-list">
          <div v-for="todo in recruitment.todayTodos" :key="todo.id" class="today-todo-row" :class="{ 'today-todo-row-done': todo.completed }">
            <el-checkbox :model-value="todo.completed" :disabled="todoBusy" :aria-label="`${todo.completed ? '取消完成' : '完成'}：${todo.title}`" @change="toggleTodo(todo)">{{ todo.title }}</el-checkbox>
            <el-button link type="danger" :disabled="todoBusy" :aria-label="`删除待办：${todo.title}`" @click="removeTodo(todo.id)">删除</el-button>
          </div>
        </div>
        <div v-else class="today-todo-empty">今天还没有待办，添加一项开始安排工作。</div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Calendar, ChatDotRound, UserFilled } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { useRecruitmentStore, stageLabels, type TalentCandidate, type TodayTodo, type RecruitmentActivity } from '../composables/useRecruitmentStore'
import { useProjectStore } from '../composables/useProjectStore'
import { getCandidateSurname } from '../utils/candidate'

const router = useRouter()
const recruitment = useRecruitmentStore()
const projects = useProjectStore()
const boardExpanded = ref(false)
const selectedProjectId = ref('')
const newTodoTitle = ref('')
const todoBusy = ref(false)
const pendingTodoCount = computed(() => recruitment.todayTodos.filter(item => !item.completed).length)
const sortedProjects = computed(() => [...projects.projects].sort((a, b) => (b.created_at || '').localeCompare(a.created_at || '')))
const activeColumns = [
  { key: 'new', label: '新简历', stages: ['new', 'pending_analysis'] },
  { key: 'screening', label: '筛选中', stages: ['pending_contact', 'communicating'] },
  { key: 'interview', label: '面试中', stages: ['first_interview', 'second_interview'] },
  { key: 'offer', label: 'Offer', stages: ['offer'] },
  { key: 'hired', label: '入职', stages: ['hired'] }
]
const boardColumns = computed(() => {
  const candidates = recruitment.candidates.filter(item => !selectedProjectId.value || item.projectId === selectedProjectId.value)
  return activeColumns.map(definition => {
    const matching = candidates.filter(item => definition.stages.includes(item.stage))
      .sort((a, b) => activityTimestamp(b) - activityTimestamp(a) || (b.createdAt || '').localeCompare(a.createdAt || ''))
    return { ...definition, firstStage: definition.stages[0], total: matching.length, items: matching.slice(0, 2) }
  })
})
const kpis = computed(() => [
  { label: '招聘项目', value: recruitment.dashboard.openProjects, icon: '▣', route: '/jobs' },
  { label: '近七天新增人才', value: recruitment.dashboard.candidatesThisWeek, icon: '▤', route: '/talent' },
  { label: '待面试安排', value: recruitment.dashboard.pendingInterviewArrangements, iconComponent: Calendar, route: '/interviews' },
  { label: '本月入职', value: recruitment.dashboard.hiresThisMonth, iconComponent: UserFilled, route: '/pipeline?stage=hired&hiredMonth=current' }
])
const funnel = computed(() => {
  const counts = recruitment.dashboard.stageCounts || {}
  const rows = [
    { label: '入库', count: Object.values(counts).reduce((a, b) => a + b, 0) },
    { label: '沟通', count: (counts.communicating || 0) + (counts.first_interview || 0) + (counts.second_interview || 0) + (counts.offer || 0) + (counts.hired || 0) },
    { label: '面试', count: (counts.first_interview || 0) + (counts.second_interview || 0) + (counts.offer || 0) + (counts.hired || 0) },
    { label: 'Offer', count: (counts.offer || 0) + (counts.hired || 0) },
    { label: '入职', count: counts.hired || 0 }
  ]
  return rows[0].count ? rows.map(row => ({ ...row, width: Math.max(4, Math.round(row.count / rows[0].count * 100)) })) : []
})

function activityTitle(activity: RecruitmentActivity) {
  if (activity.kind === 'interview') return ({ scheduled: '面试已安排', rescheduled: '面试已更新', cancelled: '面试已取消', completed: '面试已完成' } as Record<string, string>)[activity.eventType] || '面试动态'
  if (activity.eventType === 'stage_change') return `进入${stageLabels[activity.toStage] || activity.toStage}阶段`
  if (activity.eventType.startsWith('communication:')) return '新增沟通记录'
  return '新增跟进备注'
}
function activityDetail(activity: RecruitmentActivity) {
  if (activity.kind === 'interview') {
    const date = activity.startsAt ? new Date(activity.startsAt).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : '时间待定'
    return `${date} · ${activity.method || '面试'}`
  }
  return activity.note || '查看跟进详情'
}
function activityTime(value: string) {
  const timestamp = Date.parse(value)
  if (!timestamp) return ''
  const hours = Math.floor(Math.max(0, Date.now() - timestamp) / 3600000)
  if (hours < 1) return '刚刚'
  if (hours < 24) return `${hours} 小时前`
  return new Date(timestamp).toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' })
}
function openActivity(activity: RecruitmentActivity) {
  if (activity.kind === 'interview') {
    const date = activity.startsAt ? new Date(activity.startsAt) : new Date(activity.occurredAt)
    const day = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
    void router.push({ path: '/interviews', query: { date: day } })
  } else {
    void router.push({ path: '/pipeline', query: { project: activity.projectId, candidate: activity.projectCandidateId } })
  }
}
function activityTimestamp(item: TalentCandidate) { return Date.parse(item.lastFollowUpAt || item.createdAt || '') || 0 }
function activityLabel(item: TalentCandidate) {
  const value = item.lastFollowUpAt || item.createdAt
  const timestamp = Date.parse(value || '')
  if (!timestamp) return '暂无时间'
  const prefix = item.lastFollowUpAt ? '跟进' : '入库'
  const elapsed = Math.max(0, Date.now() - timestamp)
  if (elapsed < 60 * 60 * 1000) return `${prefix}${Math.max(1, Math.floor(elapsed / 60000))} 分钟前`
  if (elapsed < 24 * 60 * 60 * 1000) return `${prefix}${Math.floor(elapsed / 3600000)} 小时前`
  return `${prefix}${new Date(timestamp).toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' })}`
}
function openFollowUp(item: TalentCandidate) {
  void router.push({ path: '/pipeline', query: { project: item.projectId, candidate: item.projectCandidateId } })
}
function openStage(stage: string) {
  void router.push({ path: '/pipeline', query: { ...(selectedProjectId.value ? { project: selectedProjectId.value } : {}), stage } })
}
async function addTodo() {
  const title = newTodoTitle.value.trim()
  if (!title || todoBusy.value) return
  todoBusy.value = true
  try {
    await recruitment.addTodayTodo(title)
    newTodoTitle.value = ''
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { todoBusy.value = false }
}
async function toggleTodo(todo: TodayTodo) {
  if (todoBusy.value) return
  todoBusy.value = true
  try { await recruitment.setTodayTodoCompleted(todo.id, !todo.completed) }
  catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { todoBusy.value = false }
}
async function removeTodo(id: string) {
  if (todoBusy.value) return
  todoBusy.value = true
  try { await recruitment.deleteTodayTodo(id) }
  catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { todoBusy.value = false }
}
async function refresh() { await Promise.all([projects.loadProjects(), recruitment.loadDashboard(), recruitment.loadCandidates(), recruitment.loadTodayTodos(), recruitment.loadRecentActivities()]) }
onMounted(() => { void refresh() })
</script>

<style scoped>
.overview-board { margin-bottom: 18px; padding: 20px; }
.overview-board-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; margin-bottom: 18px; }
.overview-board-heading .wb-section-title { margin-bottom: 4px; }
.overview-board-controls { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 10px; }
.overview-board-toggle { min-height: 34px; padding: 5px 11px; border: 1px solid var(--wb-border); border-radius: 8px; background: var(--wb-surface); color: var(--wb-brand); font: inherit; cursor: pointer; }
.overview-board-toggle:hover { background: var(--wb-brand-soft); }
.overview-board-toggle:focus-visible { outline: 2px solid var(--wb-brand); outline-offset: 2px; }
.overview-board.is-collapsed .overview-board-heading { margin-bottom: 0; }
.overview-project-select { width: 240px; }
.overview-board-scroll { overflow-x: auto; padding-bottom: 6px; border-radius: 12px; }
.overview-board-scroll:focus-visible, .overview-candidate-card:focus-visible, .overview-column-more:focus-visible { outline: 2px solid var(--wb-brand); outline-offset: 2px; }
.overview-board-columns { display: grid; grid-template-columns: repeat(5, minmax(220px, 1fr)); gap: 12px; min-width: 1148px; }
.overview-board-column { min-width: 0; min-height: 206px; padding: 13px; background: var(--wb-surface-soft); border: 1px solid var(--wb-border); border-radius: 14px; }
.overview-column-heading { display: flex; justify-content: space-between; align-items: center; gap: 8px; min-height: 35px; margin-bottom: 12px; }
.overview-column-name { display: inline-flex; align-items: center; gap: 8px; min-width: 0; font-size: 14px; font-weight: 700; }
.overview-column-dot { width: 9px; height: 9px; flex: none; border-radius: 50%; background: #3b82f6; }
.overview-board-column-screening .overview-column-dot { background: #6366f1; }
.overview-board-column-interview .overview-column-dot { background: #f59e0b; }
.overview-board-column-offer .overview-column-dot { background: #8b5cf6; }
.overview-board-column-hired .overview-column-dot { background: #10b981; }
.overview-column-count { min-width: 27px; padding: 3px 8px; border-radius: 999px; border: 1px solid var(--wb-border); background: var(--wb-surface); color: var(--wb-muted); text-align: center; font-size: 12px; font-weight: 700; }
.overview-column-cards { display: flex; flex-direction: column; gap: 10px; }
.overview-candidate-card { display: flex; flex-direction: column; width: 100%; min-height: 137px; gap: 9px; padding: 12px; border: 1px solid var(--wb-border); border-radius: 12px; background: var(--wb-surface); color: var(--wb-text); font: inherit; text-align: left; cursor: pointer; transition: border-color .15s ease, box-shadow .15s ease; }
.overview-candidate-card:hover { border-color: var(--wb-brand); box-shadow: 0 4px 14px rgba(15,23,42,.08); }
.overview-candidate-main { display: flex; align-items: center; gap: 9px; min-width: 0; }
.overview-candidate-avatar { display: grid; place-items: center; width: 32px; height: 32px; flex: none; border-radius: 50%; background: var(--wb-brand); color: #fff; font-size: 13px; font-weight: 700; }
.overview-candidate-identity { display: flex; flex-direction: column; min-width: 0; gap: 2px; }
.overview-candidate-identity strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; }
.overview-candidate-identity small, .overview-candidate-project { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--wb-muted); font-size: 12px; }
.overview-candidate-project { display: block; max-width: 100%; }
.overview-skill-list { display: flex; gap: 5px; min-height: 22px; overflow: hidden; }
.overview-skill { display: inline-block; max-width: 48%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: 2px 7px; border: 1px solid var(--wb-border); border-radius: 999px; color: var(--wb-muted); background: var(--wb-surface-soft); font-size: 11px; }
.overview-candidate-footer { display: flex; justify-content: space-between; align-items: center; gap: 6px; margin-top: auto; padding-top: 8px; border-top: 1px solid var(--wb-border); font-size: 11px; }
.overview-candidate-score { color: var(--wb-brand); font-weight: 700; }
.overview-candidate-time { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--wb-muted); }
.overview-candidate-stage { align-self: flex-start; padding: 2px 7px; border-radius: 999px; color: var(--wb-brand); background: var(--wb-brand-soft); font-size: 11px; }
.overview-column-empty { margin: 18px 0; color: var(--wb-muted); text-align: center; font-size: 12px; }
.overview-column-more { width: 100%; min-height: 36px; margin-top: 9px; border: 0; background: transparent; color: var(--wb-brand); font-size: 12px; font-weight: 700; cursor: pointer; }
.overview-column-more:hover { text-decoration: underline; }
.overview-board-empty { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 30px; color: var(--wb-muted); text-align: center; }
.overview-board-empty p { margin: 0; }
.funnel-list { display:flex; flex-direction:column; gap:17px; }
.funnel-line { display:grid; grid-template-columns:50px 1fr 36px; gap:12px; align-items:center; font-size:13px; }
.funnel-track { height:24px; border-radius:7px; background:var(--wb-surface-soft); overflow:hidden; }
.funnel-fill { height:100%; border-radius:7px; background:linear-gradient(90deg,#6366f1,#8b5cf6); }
.recent-activity-list { display: flex; flex-direction: column; max-height: 440px; overflow-y: auto; }
.recent-activity-item { display: flex; width: 100%; align-items: center; gap: 11px; min-height: 72px; padding: 10px 2px; border: 0; border-bottom: 1px solid var(--wb-border); background: transparent; color: var(--wb-text); font: inherit; text-align: left; cursor: pointer; }
.recent-activity-item:last-child { border-bottom: 0; }
.recent-activity-item:hover { background: var(--wb-surface-soft); }
.recent-activity-item:focus-visible { outline: 2px solid var(--wb-brand); outline-offset: -2px; }
.recent-activity-icon { display: grid; place-items: center; width: 34px; height: 34px; flex: none; border-radius: 10px; color: var(--wb-brand); background: var(--wb-brand-soft); }
.recent-activity-icon.is-interview { color: #b45309; background: #fef3c7; }
.recent-activity-copy { display: flex; flex: 1; flex-direction: column; gap: 4px; min-width: 0; }
.recent-activity-copy strong, .recent-activity-copy small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.recent-activity-copy strong { font-size: 13px; }
.recent-activity-copy small, .recent-activity-time { color: var(--wb-muted); font-size: 11px; }
.recent-activity-time { flex: none; white-space: nowrap; }
.today-todo-card { margin-top: 18px; }
.today-todo-heading { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 15px; font-size: 12px; }
.today-todo-heading .wb-section-title { margin-bottom: 3px; }
.today-todo-add { display: flex; gap: 9px; margin-bottom: 13px; }
.today-todo-add .el-input { max-width: 680px; }
.today-todo-list { display: flex; flex-direction: column; }
.today-todo-row { display: flex; justify-content: space-between; align-items: center; gap: 12px; min-height: 47px; padding: 7px 2px; border-top: 1px solid var(--wb-border); }
.today-todo-row :deep(.el-checkbox) { min-width: 0; height: auto; margin-right: 0; }
.today-todo-row :deep(.el-checkbox__label) { overflow-wrap: anywhere; white-space: normal; color: var(--wb-text); }
.today-todo-row-done :deep(.el-checkbox__label) { color: var(--wb-muted); text-decoration: line-through; }
.today-todo-empty { padding: 20px 4px; color: var(--wb-muted); font-size: 13px; }
@media(max-width:760px) { .overview-board-heading { flex-direction: column; } .overview-board-controls { width: 100%; justify-content: flex-start; } .overview-project-select { width: min(100%, 300px); } .today-todo-heading { flex-direction: column; } .today-todo-add { flex-direction: column; } .today-todo-add .el-button { align-self: flex-start; } }
</style>
