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
      <section class="wb-card pending-interviews" aria-labelledby="pending-interviews-title">
        <h2 id="pending-interviews-title" class="wb-section-title">待面试安排（{{ pendingCandidates.length }}）</h2>
        <p class="wb-muted">招聘阶段为初面或复面，且没有未来已安排的面试时自动列入。可在候选人卡片调整阶段。</p>
        <div v-if="pendingCandidates.length" class="pending-interview-list">
          <div v-for="person in pendingCandidates" :key="person.projectCandidateId" class="pending-interview-row">
            <span><strong>{{ person.candidateName }}</strong> · {{ person.projectName }} · {{ person.stage === 'second_interview' ? '复面' : '初面' }}</span>
            <el-button size="small" type="primary" @click="bookCandidate(person.projectCandidateId)">预约面试</el-button>
          </div>
        </div>
        <p v-else class="wb-muted">当前没有待安排的人选。</p>
      </section>
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
              <article v-for="item in group.items" :key="item.id" class="interview-agenda-card" :class="{ 'has-conflict': getItemConflicts(item).length > 0 }">
                <div class="interview-agenda-time"><strong>{{ formatTime(item.startsAt) }}</strong><span>{{ durationMinutes(item) }} 分钟</span></div>
                <div class="interview-agenda-detail">
                  <strong>{{ item.candidateName }} · {{ candidateRole(item) }}</strong>
                  <div class="interview-agenda-meta">{{ item.projectName }} · 面试官：{{ item.interviewer || '待指定' }} · {{ item.method || '方式待定' }}<template v-if="item.location"> · {{ item.location }}</template></div>
                  <div v-if="getItemConflicts(item).length" class="interview-conflict-inline" role="status">
                    <span class="conflict-inline-icon">⚠️</span>
                    <div class="conflict-inline-body">
                      <div v-for="conf in getItemConflicts(item)" :key="conf.interview.id" class="conflict-inline-row">
                        <span class="conflict-tag" :class="`is-${conf.kind}`">{{ conf.labels.join(' / ') }}</span>
                        <span>与 <strong>{{ conf.interview.candidateName }}</strong>（{{ conf.interview.projectName || '未指定岗位' }} · 面试官：{{ conf.interview.interviewer || '待指定' }} · {{ conf.timeRangeText }}）时段重叠</span>
                      </div>
                    </div>
                  </div>
                  <div v-if="item.meetingSyncStatus && item.meetingSyncStatus !== 'synced'" class="wb-muted">腾讯会议：{{ item.meetingError || item.meetingSyncStatus }}</div>
                  <div v-if="item.meetingUrl" class="interview-meeting-bar">
                    <a :href="item.meetingUrl" target="_blank" rel="noopener noreferrer" @click.prevent="openMeetingUrl(item.meetingUrl)">{{ item.meetingUrl }}</a>
                    <span v-if="item.meetingCode" class="interview-meeting-code">#腾讯会议：{{ formatMeetingCode(item.meetingCode) }}</span>
                    <el-button size="small" type="primary" plain @click="copyMeetingInvite(item)">一键复制链接</el-button>
                  </div>
                  <div v-if="item.note || item.strengthsNote" class="interview-note-preview">
                    <span v-if="item.note">备注：{{ item.note }}</span>
                    <span v-if="item.strengthsNote">优势亮点：{{ item.strengthsNote }}</span>
                    <el-button size="small" link type="primary" aria-label="复制备注和优势亮点备注" @click="copyInterviewNotes(item)">复制全部备注</el-button>
                  </div>
                </div>
                <div class="interview-agenda-actions">
                  <div class="interview-agenda-pills">
                    <span v-if="getItemConflicts(item).length" class="wb-pill conflict-pill">⚠️ 时间冲突 ({{ getItemConflicts(item).length }})</span>
                    <span class="wb-pill" :class="item.status === 'cancelled' ? 'danger' : item.status === 'completed' ? 'ok' : ''">{{ statusLabel(item.status) }}</span>
                  </div>
                  <div class="interview-agenda-btn-group">
                    <el-button size="small" @click="edit(item)">{{ item.status === 'cancelled' ? '查看' : '改期 / 编辑' }}</el-button>
                    <el-button size="small" type="danger" plain @click="removeInterview(item)">删除</el-button>
                  </div>
                </div>
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
        <div class="full">
          <label>候选人及项目</label>
          <el-select v-model="form.projectCandidateId" filterable clearable :disabled="!!form.id" placeholder="输入姓名、简历文件名或职位搜索" style="width:100%" @change="onCandidateSelected">
            <el-option v-for="person in bookingCandidates" :key="person.projectCandidateId" :value="person.projectCandidateId" :label="`${person.name} · ${person.jobTitle || person.projectName} · ${person.projectName} · ${person.resumeFileName || '无文件名'}`" />
          </el-select>
        </div>
        <div><label>开始时间</label><input v-model="form.startsAt" type="datetime-local" /></div>
        <div><label>结束时间</label><input v-model="form.endsAt" type="datetime-local" /></div>
        <div v-if="formTimeRangeInvalid" class="full">
          <div class="conflict-warning-banner is-error" role="alert">
            <div class="conflict-warning-title">⛔ 结束时间必须晚于开始时间</div>
          </div>
        </div>
        <div v-else-if="formConflicts.length" class="full">
          <div class="conflict-warning-banner" role="alert">
            <div class="conflict-warning-title">⚠️ 检测到当前时间段与 {{ formConflicts.length }} 场已安排面试存在时间冲突：</div>
            <ul class="conflict-warning-list">
              <li v-for="conf in formConflicts" :key="conf.interview.id">
                <span class="conflict-tag" :class="`is-${conf.kind}`">{{ conf.labels.join(' / ') }}</span>
                <span><strong>{{ conf.interview.candidateName }}</strong> · {{ conf.interview.projectName || '未指定岗位' }}（面试官：{{ conf.interview.interviewer || '待指定' }}，{{ conf.timeRangeText }}）</span>
              </li>
            </ul>
            <div class="conflict-warning-foot">建议调整开始/结束时间，或确认面试官与腾讯会议账号不冲突后再保存。</div>
          </div>
        </div>
        <div><label>面试官</label><input v-model="form.interviewer" placeholder="输入面试官" /></div>
        <div><label>面试方式</label><select v-model="form.method"><option value="">请选择</option><option value="腾讯会议自动生成">腾讯会议自动生成</option><option value="其他视频会议">其他视频会议（手填链接）</option><option value="视频面试">视频面试（历史记录）</option><option value="现场面试">现场面试</option><option value="电话面试">电话面试</option></select></div>
        <div class="full" v-if="form.method === '腾讯会议自动生成'"><label>会议标题</label><input v-model="form.meetingSubject" :placeholder="defaultMeetingSubjectPlaceholder" /></div>
        <div class="full" v-else><label>地点或会议链接</label><input v-model="form.location" /></div>
        <div class="full" v-if="form.meetingSyncStatus">
          <label>腾讯会议状态</label>
          <p>{{ form.meetingError || (form.meetingSyncStatus === 'synced' ? '远端会议已同步' : form.meetingSyncStatus) }}</p>
          <p v-if="form.remoteStartsAt && form.meetingSyncStatus !== 'synced'">远端仍有效时间：{{ formatDay(form.remoteStartsAt) }} {{ formatTime(form.remoteStartsAt) }}</p>
          <p v-if="form.meetingUrl">
            会议链接：<a :href="form.meetingUrl" target="_blank" rel="noopener noreferrer" @click.prevent="openMeetingUrl(form.meetingUrl)">{{ form.meetingUrl }}</a>
            <template v-if="form.meetingCode"> · #腾讯会议：{{ formatMeetingCode(form.meetingCode) }}</template>
            <el-button link type="primary" @click="copyMeetingInvite(form)">一键复制链接</el-button>
          </p>
          <el-button v-if="form.id && form.meetingSyncStatus !== 'synced'" size="small" @click="retryMeetingSync">重试同步／对账</el-button>
        </div>
        <div class="full">
          <div class="interview-note-heading"><label for="interview-regular-note">备注</label><el-button size="small" link :disabled="!form.note?.trim()" @click="copyNote(form.note, '备注')">复制备注</el-button></div>
          <textarea id="interview-regular-note" v-model="form.note" rows="3" />
        </div>
        <div class="full">
          <div class="interview-note-heading"><label for="interview-strengths-note">优势亮点备注</label><div><el-button size="small" link :disabled="!form.projectCandidateId || syncingStrengths" :loading="syncingStrengths" @click="syncAssessmentStrengths">同步测评亮点</el-button><el-button size="small" link :disabled="!form.strengthsNote?.trim()" @click="copyNote(form.strengthsNote, '优势亮点备注')">复制亮点</el-button></div></div>
          <textarea id="interview-strengths-note" v-model="form.strengthsNote" rows="3" placeholder="选定候选人后可同步测评中的优势亮点，也可手动修改" />
          <el-button size="small" type="primary" plain :disabled="!form.note?.trim() && !form.strengthsNote?.trim()" @click="copyInterviewNotes(form)">一键复制两项备注</el-button>
        </div>
      </div>
      <p class="wb-muted" style="font-size:12px">时区：{{ localTimeZone }}。日程保存在本机；保存不会自动通知候选人或面试官。</p>
      <template #footer>
        <el-button v-if="form.id" type="danger" @click="removeInterview(form)">删除日程</el-button>
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
import { buildConflictConfirmMessage, detectInterviewConflicts, type InterviewConflictItem } from '../utils/interviewConflict'

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
type PendingPerson = { projectCandidateId: string; projectId: string; candidateName: string; projectName: string; stage: string }
const pendingCandidates = ref<PendingPerson[]>([])
const saving = ref(false)
const form = reactive<RecruitmentInterview>({ id:'', projectCandidateId:'', candidateName:'', projectName:'', startsAt:'', endsAt:'', interviewer:'', method:'', location:'', note:'', strengthsNote:'', status:'scheduled', meetingSubject:'', meetingId:'', meetingCode:'', meetingUrl:'', meetingSyncStatus:'' })
const syncingStrengths = ref(false)
const lastAutoSubject = ref('')
const localTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone
const bookingCandidates = computed(() => {
  const seen = new Set<string>()
  const list: Array<{ projectCandidateId: string; projectId: string; name: string; jobTitle: string; projectName: string; resumeFileName: string; stage: string }> = []
  for (const person of recruitment.candidates) {
    if (projectId.value && person.projectId !== projectId.value && person.projectCandidateId !== form.projectCandidateId) continue
    seen.add(person.projectCandidateId)
    list.push({
      projectCandidateId: person.projectCandidateId,
      projectId: person.projectId,
      name: person.name,
      jobTitle: person.jobTitle || person.projectName,
      projectName: person.projectName,
      resumeFileName: person.resumeFileName || '无文件名',
      stage: person.stage
    })
  }
  for (const pending of pendingCandidates.value) {
    if (seen.has(pending.projectCandidateId)) continue
    if (projectId.value && pending.projectId !== projectId.value && pending.projectCandidateId !== form.projectCandidateId) continue
    seen.add(pending.projectCandidateId)
    list.push({
      projectCandidateId: pending.projectCandidateId,
      projectId: pending.projectId,
      name: pending.candidateName,
      jobTitle: pending.projectName,
      projectName: pending.projectName,
      resumeFileName: '待安排人选',
      stage: pending.stage
    })
  }
  return list
})

function computeCandidateDefaultSubject(projectCandidateId: string, fallbackName = '', fallbackProject = ''): string {
  if (!projectCandidateId && !fallbackName) return ''
  const person = recruitment.candidates.find(c => c.projectCandidateId === projectCandidateId)
  const pending = pendingCandidates.value.find(c => c.projectCandidateId === projectCandidateId)
  const name = person?.name || pending?.candidateName || fallbackName || '候选人'
  const pid = person?.projectId || pending?.projectId || ''
  const proj = projects.projects.find(p => p.id === pid)
  const job = proj?.job_config?.title || person?.jobTitle || pending?.projectName || fallbackProject || person?.projectName || '面试岗位'
  const stage = person?.stage || pending?.stage || ''
  const round = stage === 'second_interview' ? '复面' : '初面'
  return `${name}－${job}－${round}`
}

const defaultMeetingSubjectPlaceholder = computed(() => {
  const auto = computeCandidateDefaultSubject(form.projectCandidateId, form.candidateName, form.projectName)
  return auto ? `默认：${auto}` : '默认：姓名－岗位－初面／复面'
})

function onCandidateSelected(val: string) {
  if (!val) {
    form.id = ''
    form.candidateName = ''
    form.projectName = ''
    form.strengthsNote = ''
    if (form.meetingSubject === lastAutoSubject.value) {
      form.meetingSubject = ''
      lastAutoSubject.value = ''
    }
    return
  }
  const matched = bookingCandidates.value.find(c => c.projectCandidateId === val)
  if (matched) {
    form.candidateName = matched.name
    form.projectName = matched.projectName
  }
  const existing = recruitment.interviews.find(i => i.projectCandidateId === val && i.status === 'scheduled')
  if (existing) {
    form.id = existing.id
    if (!form.interviewer && existing.interviewer) form.interviewer = existing.interviewer
    if (existing.method) form.method = existing.method
    if (!form.location && existing.location) form.location = existing.location
    form.note = existing.note || ''
    form.strengthsNote = existing.strengthsNote || ''
    if (existing.meetingSubject) form.meetingSubject = existing.meetingSubject
    form.meetingId = existing.meetingId || ''
    form.meetingCode = existing.meetingCode || ''
    form.meetingUrl = existing.meetingUrl || ''
    form.meetingSyncStatus = existing.meetingSyncStatus || ''
  } else {
    form.id = ''
    form.note = ''
    form.strengthsNote = ''
    void syncAssessmentStrengths(false)
  }
  if (!form.method) {
    form.method = '腾讯会议自动生成'
  }
  const nextAuto = computeCandidateDefaultSubject(val, form.candidateName, form.projectName)
  if (nextAuto && (!form.meetingSubject || form.meetingSubject === lastAutoSubject.value)) {
    form.meetingSubject = nextAuto
    lastAutoSubject.value = nextAuto
  }
}

const visibleInterviews = computed(() => recruitment.interviews.filter(item => !projectId.value || recruitment.candidates.some(person => person.projectCandidateId === item.projectCandidateId && person.projectId === projectId.value)).sort((a, b) => a.startsAt.localeCompare(b.startsAt)))

const itemConflictMap = computed(() => {
  const map = new Map<string, InterviewConflictItem[]>()
  for (const item of recruitment.interviews) {
    if (!item.id || item.status === 'cancelled' || item.status === 'completed') continue
    const list = detectInterviewConflicts(item, recruitment.interviews)
    if (list.length > 0) {
      map.set(item.id, list)
    }
  }
  return map
})

function getItemConflicts(item: RecruitmentInterview): InterviewConflictItem[] {
  if (!item.id) return []
  return itemConflictMap.value.get(item.id) || []
}

const formTimeRangeInvalid = computed(() => {
  if (!form.startsAt || !form.endsAt) return false
  const s = Date.parse(form.startsAt)
  const e = Date.parse(form.endsAt)
  return !Number.isNaN(s) && !Number.isNaN(e) && e <= s
})

const formConflicts = computed(() => {
  if (!form.startsAt || !form.endsAt || formTimeRangeInvalid.value) return []
  return detectInterviewConflicts(
    {
      id: form.id,
      projectCandidateId: form.projectCandidateId,
      candidateName: form.candidateName,
      interviewer: form.interviewer,
      startsAt: toISO(form.startsAt),
      endsAt: toISO(form.endsAt),
      status: form.status || 'scheduled'
    },
    recruitment.interviews
  )
})
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
    : (item.status !== 'cancelled' || item.meetingSyncStatus === 'pending_cancel') && Date.parse(item.startsAt) >= todayStart)
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
function interviewCountOn(key: string) { return visibleInterviews.value.filter(item => (item.status !== 'cancelled' || item.meetingSyncStatus === 'pending_cancel') && dateKey(item.startsAt) === key).length }
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
  lastAutoSubject.value = ''
  Object.assign(form, { id:'', projectCandidateId:'', candidateName:'', projectName:'', startsAt:'', endsAt:'', interviewer:'', method:'腾讯会议自动生成', location:'', note:'', strengthsNote:'', status:'scheduled', meetingSubject:'', meetingId:'', meetingCode:'', meetingSyncStatus:'', meetingUrl:'', meetingError:'', remoteStartsAt:'', remoteEndsAt:'' })
  const baseDate = selectedDate.value || todayKey
  form.startsAt = `${baseDate}T10:00`
  form.endsAt = `${baseDate}T11:00`
  dialogOpen.value = true
}
function bookCandidate(id: string) {
  create()
  form.projectCandidateId = id
  onCandidateSelected(id)
}
function edit(item: RecruitmentInterview) {
  lastAutoSubject.value = ''
  Object.assign(form, item, {
    strengthsNote: item.strengthsNote || '',
    method: item.method || '腾讯会议自动生成',
    startsAt: localInput(item.startsAt),
    endsAt: localInput(item.endsAt)
  })
  dialogOpen.value = true
}
async function reload() {
  await Promise.all([recruitment.loadInterviews(), recruitment.loadCandidates()])
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    pendingCandidates.value = await api.ListPendingInterviewCandidates(projectId.value) || []
  } catch (err: any) { recruitment.error = err?.message || String(err) }
}
async function save() {
  if (!form.projectCandidateId || !form.startsAt || !form.endsAt) { ElMessage.warning('请选择候选人并填写时间'); return }
  if (Date.parse(form.endsAt) <= Date.parse(form.startsAt)) {
    ElMessage.warning('结束时间必须晚于开始时间')
    return
  }
  if (formConflicts.value.length > 0) {
    try {
      await ElMessageBox.confirm(
        buildConflictConfirmMessage(formConflicts.value),
        '面试时间冲突提醒',
        {
          confirmButtonText: '仍要保存',
          cancelButtonText: '返回修改时间',
          type: 'warning'
        }
      )
    } catch {
      return
    }
  }
  saving.value = true
  try {
    await recruitment.saveInterview({ ...form, startsAt: toISO(form.startsAt), endsAt: toISO(form.endsAt) })
    await reload()
    dialogOpen.value = false
    selectDate(dateKey(form.startsAt))
    ElMessage.success('面试日程已保存；腾讯会议状态显示在日程卡片中')
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { saving.value = false }
}
async function cancel() {
  if (!form.id) return
  try {
    await ElMessageBox.confirm('确定取消这场面试吗？', '取消面试', { type:'warning' })
    await recruitment.cancelInterview(form.id)
    await reload()
    const latest = recruitment.interviews.find(item => item.id === form.id)
    dialogOpen.value = false
    if (latest?.meetingSyncStatus === 'pending_cancel') ElMessage.warning('本地安排已取消，腾讯会议仍待取消；请查看原链接并重试')
    else ElMessage.success('面试已取消')
  } catch (err: any) { if (err !== 'cancel') ElMessage.error(err?.message || String(err)) }
}

async function removeInterview(item: RecruitmentInterview) {
  if (!item?.id) return
  try {
    await ElMessageBox.confirm(
      `确定彻底删除「${item.candidateName || '该候选人'}」的这场面试安排吗？删除后不可恢复。`,
      '删除面试安排',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await recruitment.deleteInterview(item.id)
    await reload()
    if (dialogOpen.value && form.id === item.id) {
      dialogOpen.value = false
    }
    ElMessage.success('面试安排已删除')
  } catch (err: any) {
    if (err !== 'cancel') {
      ElMessage.error(err?.message || String(err))
    }
  }
}

function formatMeetingCode(code?: string): string {
  const raw = (code || '').trim()
  if (!raw) return ''
  const digits = raw.replace(/\D/g, '')
  if (digits.length === 9) {
    return `${digits.slice(0, 3)}-${digits.slice(3, 6)}-${digits.slice(6, 9)}`
  }
  if (digits.length === 10) {
    return `${digits.slice(0, 3)}-${digits.slice(3, 6)}-${digits.slice(6, 10)}`
  }
  if (digits.length === 11) {
    return `${digits.slice(0, 3)}-${digits.slice(3, 7)}-${digits.slice(7, 11)}`
  }
  return raw
}

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

function formatMeetingInviteTimeRange(startsAt: string, endsAt: string): string {
  const s = new Date(startsAt)
  const e = new Date(endsAt)
  if (Number.isNaN(s.getTime())) return startsAt || ''
  const sDate = `${s.getFullYear()}/${pad2(s.getMonth() + 1)}/${pad2(s.getDate())}`
  const sTime = `${pad2(s.getHours())}:${pad2(s.getMinutes())}`
  if (Number.isNaN(e.getTime())) {
    return `${sDate} ${sTime} (GMT+08:00) 中国标准时间 - 北京`
  }
  const eDate = `${e.getFullYear()}/${pad2(e.getMonth() + 1)}/${pad2(e.getDate())}`
  const eTime = `${pad2(e.getHours())}:${pad2(e.getMinutes())}`
  const range = sDate === eDate ? `${sDate} ${sTime}-${eTime}` : `${sDate} ${sTime}-${eDate} ${eTime}`
  return `${range} (GMT+08:00) 中国标准时间 - 北京`
}

function buildTencentMeetingInviteText(item: RecruitmentInterview): string {
  const subject = (item.meetingSubject || '').trim() || computeCandidateDefaultSubject(item.projectCandidateId, item.candidateName, item.projectName)
  const timeRange = formatMeetingInviteTimeRange(item.startsAt, item.endsAt)
  const url = (item.meetingUrl || item.location || '').trim()
  const code = formatMeetingCode(item.meetingCode)
  const lines = [
    '邀请您参加腾讯会议',
    `会议主题：${subject}`,
    `会议时间：${timeRange}`,
    '',
    '点击链接入会，或添加至会议列表：',
    url
  ]
  if (code) {
    lines.push('', `#腾讯会议：${code}`)
  }
  lines.push('', '复制该信息，打开手机腾讯会议即可参与')
  return lines.join('\n')
}

async function writeClipboardText(text: string): Promise<void> {
  try {
    const runtime: any = await import('../../wailsjs/runtime/runtime')
    if (typeof runtime?.ClipboardSetText === 'function') {
      const ok = await runtime.ClipboardSetText(text)
      if (ok) return
    }
  } catch { /* fallback to web clipboard */ }
  if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
    await navigator.clipboard.writeText(text)
    return
  }
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

async function syncAssessmentStrengths(showFeedback = true) {
  const candidateId = form.projectCandidateId
  if (!candidateId) return
  syncingStrengths.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const strengths = await api.GetRecruitmentAssessmentStrengths(candidateId) || ''
    if (form.projectCandidateId !== candidateId || (!showFeedback && form.strengthsNote?.trim())) return
    if (strengths) {
      form.strengthsNote = strengths
      if (showFeedback) ElMessage.success('已同步最新测评亮点，请保存日程')
    } else if (showFeedback) {
      ElMessage.info('该候选人当前没有已完成测评的优势亮点')
    }
  } catch (err: any) {
    if (showFeedback) ElMessage.error(err?.message || '同步测评亮点失败')
  } finally {
    syncingStrengths.value = false
  }
}

async function copyNote(value: string | undefined, label: string) {
  const content = (value || '').trim()
  if (!content) return
  try {
    await writeClipboardText(content)
    ElMessage.success(`已复制${label}`)
  } catch (err: any) {
    ElMessage.error(err?.message || '复制失败，请重试')
  }
}

async function copyInterviewNotes(item: RecruitmentInterview) {
  const sections = [
    item.note?.trim() ? `备注：\n${item.note.trim()}` : '',
    item.strengthsNote?.trim() ? `优势亮点备注：\n${item.strengthsNote.trim()}` : ''
  ].filter(Boolean)
  if (!sections.length) return
  await copyNote(sections.join('\n\n'), '全部备注')
}

async function copyMeetingInvite(item: RecruitmentInterview) {
  const url = (item.meetingUrl || item.location || '').trim()
  if (!url) {
    ElMessage.warning('暂无可复制的会议链接')
    return
  }
  const inviteText = buildTencentMeetingInviteText(item)
  try {
    await writeClipboardText(inviteText)
    ElMessage.success('已复制完整腾讯会议邀请信息')
  } catch (err: any) {
    ElMessage.error(err?.message || '复制失败，请重试')
  }
}

async function openMeetingUrl(url?: string) {
  const target = (url || '').trim()
  if (!target) return
  try {
    const runtime: any = await import('../../wailsjs/runtime/runtime')
    if (typeof runtime?.BrowserOpenURL === 'function') {
      runtime.BrowserOpenURL(target)
      return
    }
  } catch { /* fallback */ }
  window.open(target, '_blank', 'noopener,noreferrer')
}

async function retryMeetingSync() { if (!form.id) return; try { const api: any = await import('../../wailsjs/go/main/App'); await api.RetryTencentMeetingSync(form.id); await reload(); const latest = recruitment.interviews.find(item => item.id === form.id); if (latest) edit(latest) } catch (err: any) { ElMessage.error(err?.message || String(err)) } }
onMounted(() => { void projects.loadProjects(); void reload() })
</script>

<style scoped>
.interview-layout { display: grid; grid-template-columns: minmax(0, 1.65fr) minmax(290px, .9fr); gap: 18px; align-items: start; }
.pending-interviews { margin-bottom: 18px; }
.pending-interview-list { display: flex; flex-direction: column; gap: 8px; }
.pending-interview-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 8px 0; border-bottom: 1px solid var(--wb-border); }
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
.interview-agenda-card.has-conflict { border-color: #fdba74; border-left: 4px solid #ea580c; background: #fffaf5; }
.interview-note-preview { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 12px; margin-top: 8px; color: var(--wb-muted); font-size: 12px; }
.interview-note-preview span { max-width: 360px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.interview-note-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.interview-agenda-pills { display: flex; align-items: center; flex-wrap: wrap; justify-content: flex-end; gap: 6px; }
.conflict-pill { background: #ffedd5; color: #c2410c; border: 1px solid #fdba74; font-weight: 700; }
.interview-conflict-inline { display: flex; align-items: flex-start; gap: 8px; margin-top: 6px; padding: 7px 10px; border-radius: 8px; background: #fff7ed; border: 1px solid #fed7aa; color: #9a3412; font-size: 12px; line-height: 1.5; }
.conflict-inline-icon { flex: none; font-size: 13px; line-height: 1.4; }
.conflict-inline-body { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.conflict-inline-row { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
.conflict-tag { display: inline-flex; align-items: center; padding: 1px 7px; border-radius: 999px; font-size: 11px; font-weight: 700; white-space: nowrap; }
.conflict-tag.is-candidate { background: #fee2e2; color: #b91c1c; border: 1px solid #fca5a5; }
.conflict-tag.is-interviewer { background: #ffedd5; color: #c2410c; border: 1px solid #fdba74; }
.conflict-tag.is-time { background: #fef3c7; color: #b45309; border: 1px solid #fcd34d; }
.conflict-warning-banner { padding: 11px 13px; border-radius: 10px; background: #fff7ed; border: 1px solid #fdba74; color: #9a3412; font-size: 12.5px; line-height: 1.55; }
.conflict-warning-banner.is-error { background: #fef2f2; border-color: #fca5a5; color: #b91c1c; }
.conflict-warning-title { font-weight: 700; margin-bottom: 6px; }
.conflict-warning-banner.is-error .conflict-warning-title { margin-bottom: 0; }
.conflict-warning-list { margin: 0; padding-left: 0; list-style: none; display: flex; flex-direction: column; gap: 5px; }
.conflict-warning-list li { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
.conflict-warning-foot { margin-top: 7px; font-size: 11.5px; color: #c2410c; }
.interview-agenda-time { display: flex; flex-direction: column; gap: 3px; }
.interview-agenda-time strong { font-size: 17px; }
.interview-agenda-time span, .interview-agenda-meta { color: var(--wb-muted); font-size: 12px; }
.interview-agenda-detail { min-width: 0; }
.interview-agenda-detail > strong { display: block; margin-bottom: 7px; overflow-wrap: anywhere; }
.interview-agenda-meta { line-height: 1.6; overflow-wrap: anywhere; }
.interview-meeting-bar { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; margin-top: 4px; }
.interview-meeting-bar a { color: var(--wb-brand); text-decoration: underline; word-break: break-all; font-size: 13px; }
.interview-meeting-code { font-size: 12px; color: var(--wb-muted); background: var(--wb-surface-soft); padding: 2px 8px; border-radius: 6px; border: 1px solid var(--wb-border); }
.interview-agenda-actions { display: flex; flex-direction: column; align-items: flex-end; gap: 9px; }
.interview-agenda-btn-group { display: flex; align-items: center; gap: 8px; }
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
