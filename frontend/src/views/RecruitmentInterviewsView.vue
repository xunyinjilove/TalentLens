<template>
  <div class="workbench-page">
    <header class="wb-header">
      <div><h1>面试安排</h1><div class="wb-crumb">日程 / 预约与改期</div></div>
    </header>
    <main class="wb-content">
      <div class="wb-toolbar">
        <el-select v-model="projectId" clearable placeholder="全部岗位" style="width:300px" @change="reload">
          <el-option v-for="project in projects.projects" :key="project.id" :label="`${project.job_config.title || project.name} · ${project.name}`" :value="project.id" />
        </el-select>
      </div>
      <el-alert v-if="recruitment.error" :title="recruitment.error" type="warning" :closable="false" style="margin-bottom:16px" />
      <div class="interview-layout">
        <section class="interview-agenda" aria-labelledby="interview-agenda-title">
          <div class="interview-agenda-heading">
            <div><h2 id="interview-agenda-title" class="wb-section-title">面试日程</h2><p v-if="selectedDate" class="wb-muted">{{ formatSelectedDate(selectedDate) }}</p></div>
            <div class="interview-agenda-toolbar">
              <el-button v-if="selectedDate" link @click="selectedDate = ''">显示近期</el-button>
              <el-button type="primary" @click="create">预约面试</el-button>
            </div>
          </div>
          <div v-if="agendaGroups.length" class="interview-day-groups">
            <section v-for="group in agendaGroups" :key="group.date" class="interview-day-group">
              <div class="interview-day-heading"><span class="interview-day-number">{{ group.day }}</span><div><h3>{{ group.label }}</h3><span class="wb-muted">共 {{ group.items.length }} 场面试</span></div></div>
              <article v-for="item in group.items" :key="item.id" class="interview-agenda-card">
                <div class="interview-agenda-time"><strong>{{ formatTime(item.startsAt) }}</strong><span>{{ durationMinutes(item) }} 分钟</span></div>
                <div class="interview-agenda-detail"><strong>{{ item.candidateName }} · {{ candidateRole(item) }}</strong><div class="interview-agenda-meta">{{ item.projectName }} · 面试官：{{ item.interviewer || '待指定' }} · {{ item.method || '方式待定' }}<template v-if="item.location"> · {{ item.location }}</template></div></div>
                <div class="interview-agenda-actions"><span class="wb-pill" :class="item.status === 'cancelled' ? 'danger' : item.status === 'completed' ? 'ok' : ''">{{ statusLabel(item.status) }}</span><el-button size="small" @click="edit(item)">{{ item.status === 'cancelled' ? '查看' : '改期 / 编辑' }}</el-button></div>
              </article>
            </section>
          </div>
          <div v-else class="wb-card wb-empty">{{ selectedDate ? '这一天没有面试安排。' : '近期没有面试安排。可预约面试。' }}</div>
        </section>
        <aside class="wb-card interview-calendar" aria-label="面试日历">
          <div class="interview-calendar-heading"><strong>{{ calendarMonthLabel }}</strong><div><button type="button" aria-label="上个月" @click="changeMonth(-1)">‹</button><button type="button" aria-label="下个月" @click="changeMonth(1)">›</button></div></div>
          <div class="interview-calendar-grid interview-calendar-weekdays"><span v-for="weekday in weekdays" :key="weekday">{{ weekday }}</span></div>
          <div class="interview-calendar-grid interview-calendar-days">
            <button v-for="day in calendarDays" :key="day.key" type="button" :class="{ 'is-outside': !day.inMonth, 'is-today': day.key === todayKey, 'is-selected': day.key === selectedDate, 'has-interview': interviewCountOn(day.key) > 0 }" :aria-label="`${day.key}，${interviewCountOn(day.key)} 场面试`" @click="selectDate(day.key)">{{ day.day }}<span v-if="interviewCountOn(day.key)" class="interview-calendar-dot" /></button>
          </div>
          <div class="interview-coming"><h3>即将到来</h3><button v-for="item in upcomingInterviews" :key="item.id" type="button" @click="selectDate(dateKey(item.startsAt))"><span class="interview-coming-icon">✓</span><span><strong>{{ item.candidateName }} · {{ candidateRole(item) }}</strong><small>{{ formatDay(item.startsAt) }} {{ formatTime(item.startsAt) }} · {{ item.method || '面试' }}</small></span></button><p v-if="!upcomingInterviews.length" class="wb-muted">暂无即将举行的面试</p></div>
        </aside>
      </div>
    </main>
    <el-dialog v-model="dialogOpen" :title="form.id ? '修改面试' : '预约面试'" width="600px">
      <div class="wb-form-grid">
        <div class="full"><label>候选人及项目</label><el-select v-model="form.projectCandidateId" filterable clearable :disabled="!!form.id" placeholder="输入姓名、简历文件名或职位搜索" style="width:100%" :filter-method="filterBookingCandidates"><el-option v-for="person in bookingCandidates" :key="person.projectCandidateId" :value="person.projectCandidateId" :label="`${person.name} · ${person.jobTitle || person.projectName} · ${person.projectName} · ${person.resumeFileName || '无文件名'}`" /></el-select></div>
        <div><label>开始时间</label><input v-model="form.startsAt" type="datetime-local" /></div>
        <div><label>结束时间</label><input v-model="form.endsAt" type="datetime-local" /></div>
        <div><label>面试官</label><input v-model="form.interviewer" placeholder="输入面试官" /></div>
        <div><label>面试方式</label><select v-model="form.method"><option value="">请选择</option><option value="视频面试">视频面试</option><option value="现场面试">现场面试</option><option value="电话面试">电话面试</option></select></div>
        <div class="full"><label>地点或会议链接</label><input v-model="form.location" /></div>
        <div class="full"><label>备注</label><textarea v-model="form.note" rows="3" /></div>
      </div>
      <p class="wb-muted" style="font-size:12px">日程保存在本机；保存不会自动通知候选人或面试官。</p>
      <template #footer>
        <el-button v-if="form.id && form.status !== 'cancelled'" type="danger" plain @click="cancel">取消面试</el-button>
        <el-button @click="dialogOpen = false">关闭</el-button>
        <el-button v-if="form.status !== 'cancelled'" type="primary" :loading="saving" @click="save">保存日程</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute } from 'vue-router'
import { useProjectStore } from '../composables/useProjectStore'
import { useRecruitmentStore, type RecruitmentInterview } from '../composables/useRecruitmentStore'

const route = useRoute()
const projects = useProjectStore()
const recruitment = useRecruitmentStore()
const projectId = ref('')
const initialDate = typeof route.query.date === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(route.query.date) && !Number.isNaN(Date.parse(`${route.query.date}T12:00:00`)) ? route.query.date : ''
const selectedDate = ref(initialDate)
const initialCalendarDate = initialDate ? new Date(`${initialDate}T12:00:00`) : new Date()
const calendarCursor = ref(new Date(initialCalendarDate.getFullYear(), initialCalendarDate.getMonth(), 1))
const weekdays = ['一', '二', '三', '四', '五', '六', '日']
const dialogOpen = ref(false)
const saving = ref(false)
const form = reactive<RecruitmentInterview>({ id:'', projectCandidateId:'', candidateName:'', projectName:'', startsAt:'', endsAt:'', interviewer:'', method:'', location:'', note:'', status:'scheduled' })
const bookingQuery = ref('')
const bookingCandidates = computed(() => recruitment.candidates.filter(person => {
  if (projectId.value && person.projectId !== projectId.value && person.projectCandidateId !== form.projectCandidateId) return false
  const query = bookingQuery.value.trim().toLocaleLowerCase()
  return !query || [person.name, person.resumeFileName, person.jobTitle, person.projectName].some(value => (value || '').toLocaleLowerCase().includes(query))
}))
function filterBookingCandidates(value: string) { bookingQuery.value = value }
const visibleInterviews = computed(() => recruitment.interviews.filter(item => !projectId.value || recruitment.candidates.some(person => person.projectCandidateId === item.projectCandidateId && person.projectId === projectId.value)).sort((a, b) => a.startsAt.localeCompare(b.startsAt)))
const todayKey = dateKey(new Date())
const calendarMonthLabel = computed(() => `${calendarCursor.value.getFullYear()} 年 ${calendarCursor.value.getMonth() + 1} 月`)
const calendarDays = computed(() => {
  const start = new Date(calendarCursor.value.getFullYear(), calendarCursor.value.getMonth(), 1)
  const mondayOffset = (start.getDay() + 6) % 7
  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(start.getFullYear(), start.getMonth(), index - mondayOffset + 1)
    return { key: dateKey(date), day: date.getDate(), inMonth: date.getMonth() === start.getMonth() }
  })
})
const agendaGroups = computed(() => {
  const todayStart = new Date(new Date().getFullYear(), new Date().getMonth(), new Date().getDate()).getTime()
  const rows = visibleInterviews.value.filter(item => selectedDate.value
    ? dateKey(item.startsAt) === selectedDate.value
    : item.status !== 'cancelled' && Date.parse(item.startsAt) >= todayStart)
  const grouped = new Map<string, RecruitmentInterview[]>()
  for (const item of rows) {
    const key = dateKey(item.startsAt)
    if (!grouped.has(key)) grouped.set(key, [])
    grouped.get(key)!.push(item)
  }
  return Array.from(grouped, ([date, items]) => ({
    date, items, day: Number(date.slice(-2)), label: date === todayKey ? `今天 · ${formatSelectedDate(date)}` : formatSelectedDate(date)
  }))
})
const upcomingInterviews = computed(() => visibleInterviews.value.filter(item => item.status === 'scheduled' && Date.parse(item.startsAt) >= Date.now()).slice(0, 3))
watch(() => route.query.date, value => { if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(`${value}T12:00:00`))) selectDate(value) })
function dateKey(value: string | Date) {
  const date = new Date(value)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}
function formatSelectedDate(value: string) { return new Date(`${value}T12:00:00`).toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }) }
function interviewCountOn(key: string) { return visibleInterviews.value.filter(item => item.status !== 'cancelled' && dateKey(item.startsAt) === key).length }
function selectDate(key: string) {
  selectedDate.value = key
  const date = new Date(`${key}T12:00:00`)
  calendarCursor.value = new Date(date.getFullYear(), date.getMonth(), 1)
}
function changeMonth(delta: number) { calendarCursor.value = new Date(calendarCursor.value.getFullYear(), calendarCursor.value.getMonth() + delta, 1) }
function candidateRole(item: RecruitmentInterview) {
  const person = recruitment.candidates.find(candidate => candidate.projectCandidateId === item.projectCandidateId)
  return person?.currentRole || person?.jobTitle || item.projectName
}
function durationMinutes(item: RecruitmentInterview) { return Math.max(0, Math.round((Date.parse(item.endsAt) - Date.parse(item.startsAt)) / 60000)) }
function localInput(value: string) {
  if (!value) return ''
  const d = new Date(value)
  const offset = d.getTimezoneOffset() * 60000
  return new Date(d.getTime() - offset).toISOString().slice(0, 16)
}
function toISO(value: string) { return value ? new Date(value).toISOString() : '' }
function formatDay(value: string) { return new Date(value).toLocaleDateString('zh-CN', { month:'short', day:'numeric' }) }
function formatTime(value: string) { return new Date(value).toLocaleTimeString('zh-CN', { hour:'2-digit', minute:'2-digit' }) }
function statusLabel(status: string) { return ({ scheduled:'已安排', cancelled:'已取消', completed:'已完成' } as Record<string,string>)[status] || status }
function create() {
  bookingQuery.value = ''
  Object.assign(form, { id:'', projectCandidateId:'', candidateName:'', projectName:'', startsAt:'', endsAt:'', interviewer:'', method:'', location:'', note:'', status:'scheduled' })
  if (selectedDate.value) {
    form.startsAt = `${selectedDate.value}T10:00`
    form.endsAt = `${selectedDate.value}T11:00`
  }
  dialogOpen.value = true
}
function edit(item: RecruitmentInterview) { bookingQuery.value = ''; Object.assign(form, item, { startsAt: localInput(item.startsAt), endsAt: localInput(item.endsAt) }); dialogOpen.value = true }
async function reload() { await Promise.all([recruitment.loadInterviews(), recruitment.loadCandidates()]) }
async function save() {
  if (!form.projectCandidateId || !form.startsAt || !form.endsAt) { ElMessage.warning('请选择候选人并填写时间'); return }
  saving.value = true
  try {
    await recruitment.saveInterview({ ...form, startsAt: toISO(form.startsAt), endsAt: toISO(form.endsAt) })
    dialogOpen.value = false
    selectDate(dateKey(form.startsAt))
    ElMessage.success('面试日程已保存至本机')
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { saving.value = false }
}
async function cancel() {
  if (!form.id) return
  try {
    await ElMessageBox.confirm('确定取消这场面试吗？', '取消面试', { type:'warning' })
    await recruitment.cancelInterview(form.id)
    dialogOpen.value = false
    ElMessage.success('面试已取消')
  } catch (err: any) { if (err !== 'cancel') ElMessage.error(err?.message || String(err)) }
}
onMounted(() => { void projects.loadProjects(); void reload() })
</script>

<style scoped>
.interview-layout { display: grid; grid-template-columns: minmax(0, 1.65fr) minmax(290px, .9fr); gap: 18px; align-items: start; }
.interview-agenda-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 14px; margin-bottom: 17px; }
.interview-agenda-toolbar { display: flex; align-items: center; gap: 8px; }
.interview-agenda-heading .wb-section-title { margin-bottom: 4px; }
.interview-agenda-heading p { margin: 0; font-size: 13px; }
.interview-day-groups { display: flex; flex-direction: column; gap: 20px; }
.interview-day-group { display: flex; flex-direction: column; gap: 10px; }
.interview-day-heading { display: flex; align-items: center; gap: 11px; margin-bottom: 2px; }
.interview-day-number { display: grid; place-items: center; width: 46px; height: 46px; flex: none; border-radius: 12px; background: var(--wb-brand); color: #fff; font-size: 20px; font-weight: 800; }
.interview-day-heading h3 { margin: 0 0 3px; font-size: 17px; }
.interview-day-heading .wb-muted { font-size: 12px; }
.interview-agenda-card { display: grid; grid-template-columns: 78px minmax(0, 1fr) auto; gap: 14px; align-items: center; min-height: 112px; padding: 16px; border: 1px solid var(--wb-border); border-left: 3px solid var(--wb-brand); border-radius: 14px; background: var(--wb-surface); box-shadow: 0 2px 10px rgba(15, 23, 42, .04); }
.interview-agenda-time { display: flex; flex-direction: column; gap: 3px; }
.interview-agenda-time strong { font-size: 17px; }
.interview-agenda-time span, .interview-agenda-meta { color: var(--wb-muted); font-size: 12px; }
.interview-agenda-detail { min-width: 0; }
.interview-agenda-detail > strong { display: block; margin-bottom: 7px; overflow-wrap: anywhere; }
.interview-agenda-meta { line-height: 1.6; overflow-wrap: anywhere; }
.interview-agenda-actions { display: flex; flex-direction: column; align-items: flex-end; gap: 9px; }
.interview-calendar { position: sticky; top: 96px; }
.interview-calendar-heading { display: flex; justify-content: space-between; align-items: center; gap: 8px; margin-bottom: 14px; }
.interview-calendar-heading strong { font-size: 16px; }
.interview-calendar-heading button { width: 32px; height: 32px; border: 0; border-radius: 8px; background: transparent; color: var(--wb-muted); font-size: 23px; cursor: pointer; }
.interview-calendar-heading button:hover { background: var(--wb-brand-soft); color: var(--wb-brand); }
.interview-calendar-grid { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 4px; text-align: center; }
.interview-calendar-weekdays { margin-bottom: 8px; color: var(--wb-muted); font-size: 12px; font-weight: 700; }
.interview-calendar-days button { display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 39px; padding: 4px 0; border: 0; border-radius: 9px; background: transparent; color: var(--wb-text); font: inherit; font-size: 13px; cursor: pointer; }
.interview-calendar-days button:hover, .interview-calendar-days button.is-today { background: var(--wb-brand-soft); color: var(--wb-brand); }
.interview-calendar-days button.is-selected { background: var(--wb-brand); color: #fff; font-weight: 750; }
.interview-calendar-days button.is-outside { opacity: .42; }
.interview-calendar-dot { width: 4px; height: 4px; margin-top: 2px; border-radius: 50%; background: currentColor; }
.interview-calendar button:focus-visible, .interview-coming button:focus-visible { outline: 2px solid var(--wb-brand); outline-offset: 2px; }
.interview-coming { margin-top: 24px; }
.interview-coming h3 { margin: 0 0 12px; font-size: 14px; }
.interview-coming > button { display: flex; width: 100%; align-items: center; gap: 10px; padding: 10px 0; border: 0; border-bottom: 1px solid var(--wb-border); background: transparent; color: var(--wb-text); text-align: left; cursor: pointer; }
.interview-coming > button > span:last-child { min-width: 0; }
.interview-coming strong, .interview-coming small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.interview-coming strong { font-size: 12px; }
.interview-coming small { margin-top: 3px; color: var(--wb-muted); font-size: 11px; }
.interview-coming-icon { display: grid; place-items: center; width: 28px; height: 28px; flex: none; border-radius: 9px; background: var(--wb-brand-soft); color: var(--wb-brand); }
.interview-coming .wb-muted { font-size: 12px; }
@media (max-width: 1050px) { .interview-layout { grid-template-columns: 1fr; } .interview-calendar { position: static; max-width: 480px; } }
@media (max-width: 650px) { .interview-agenda-card { grid-template-columns: 65px minmax(0, 1fr); } .interview-agenda-actions { grid-column: 2; flex-direction: row; align-items: center; justify-content: space-between; } }
</style>
