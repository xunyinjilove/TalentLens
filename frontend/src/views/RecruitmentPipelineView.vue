<template>
  <div class="workbench-page">
    <header class="wb-header">
      <div>
        <h1>候选人跟进</h1>
        <div class="wb-crumb">Offer 跟进与入职确认看板</div>
      </div>
      <div class="wb-actions">
        <el-button @click=" router.push('/talent') ">前往简历人才库</el-button>
        <el-button type="primary" plain @click="recruitment.loadCandidates()">刷新数据</el-button>
      </div>
    </header>

    <main class="wb-content">
      <el-alert
        v-if="recruitment.error"
        :title="recruitment.error"
        type="warning"
        :closable="false"
        style="margin-bottom: 16px"
      />

      <!-- 顶部统计与筛选工具栏 -->
      <section class="wb-card pipeline-toolbar-card">
        <div class="pipeline-stats-row">
          <div class="pipeline-stat-pill is-offer">
            <span class="stat-dot" aria-hidden="true" />
            <span class="stat-label">Offer 跟进中</span>
            <strong class="stat-value">{{ offerCandidates.length }}</strong>
          </div>
          <div class="pipeline-stat-pill is-month-hired">
            <span class="stat-dot" aria-hidden="true" />
            <span class="stat-label">本月已入职</span>
            <strong class="stat-value">{{ currentMonthHiredCount }}</strong>
          </div>
          <div class="pipeline-stat-pill is-total-hired">
            <span class="stat-dot" aria-hidden="true" />
            <span class="stat-label">累计已入职</span>
            <strong class="stat-value">{{ totalHiredCount }}</strong>
          </div>
        </div>

        <div class="pipeline-filter-row">
          <el-select
            v-model="projectId"
            clearable
            placeholder="全部招聘岗位"
            class="pipeline-project-select"
            @change="syncRoute"
          >
            <el-option
              v-for="project in projects.projects"
              :key="project.id"
              :label="`${project.job_config.title || project.name} · ${project.name}`"
              :value="project.id"
            />
          </el-select>

          <el-input
            v-model="keyword"
            clearable
            placeholder="搜索候选人姓名、岗位或负责人..."
            class="pipeline-search-input"
          />

          <div class="pipeline-scope-switch" role="group" aria-label="入职展示范围">
            <button
              type="button"
              class="scope-btn"
              :class="{ active: hiredMonthFilter !== 'current' }"
              @click="setHiredMonthFilter('all')"
            >
              全部入职 ({{ totalHiredCount }})
            </button>
            <button
              type="button"
              class="scope-btn"
              :class="{ active: hiredMonthFilter === 'current' }"
              @click="setHiredMonthFilter('current')"
            >
              本月入职 ({{ currentMonthHiredCount }})
            </button>
          </div>
        </div>
      </section>

      <!-- 双栏宽屏看板：仅展示 Offer 与 入职 -->
      <div ref="kanbanEl" class="pipeline-two-col-board">
        <section
          v-for="col in boardColumns"
          :key="col.stage"
          class="pipeline-column"
          :class="`is-${col.stage}`"
          :data-stage="col.stage"
        >
          <header class="pipeline-column-header">
            <div class="pipeline-column-title-wrap">
              <span class="pipeline-column-dot" aria-hidden="true" />
              <div>
                <div class="pipeline-column-title-row">
                  <h2>{{ col.title }}</h2>
                  <span v-if="col.stage === 'hired' && hiredMonthFilter === 'current'" class="pipeline-filter-tag">
                    仅看本月首次入职
                  </span>
                </div>
                <p class="pipeline-column-subtitle">{{ col.subtitle }}</p>
              </div>
            </div>
            <span class="pipeline-column-count" :aria-label="`${col.items.length} 位候选人`">
              {{ col.items.length }} 人
            </span>
          </header>

          <div v-if="col.items.length" class="pipeline-card-list">
            <article
              v-for="item in col.items"
              :key="item.projectCandidateId"
              class="pipeline-card"
              :class="`is-${item.stage}`"
            >
              <header class="pipeline-card-header">
                <div class="pipeline-card-identity">
                  <span class="pipeline-avatar" aria-hidden="true">{{ getCandidateSurname(item) }}</span>
                  <div class="pipeline-name-block">
                    <div class="pipeline-name-row">
                      <h3 :title="item.name">{{ item.name }}</h3>
                      <span class="pipeline-stage-badge" :class="`is-${item.stage}`">
                        {{ item.stage === 'hired' ? '已入职' : 'Offer 待入职' }}
                      </span>
                    </div>
                    <p class="pipeline-role-meta" :title="item.jobTitle || item.projectName">
                      {{ item.jobTitle || item.projectName }}
                      <template v-if="item.workYears"> · {{ item.workYears }}</template>
                    </p>
                  </div>
                </div>
                <span class="pipeline-score-pill">
                  {{
                    item.analysisStatus === 'done' && !item.radarStale
                      ? `综合 ${item.abilityScore} 分`
                      : item.radarStale
                        ? '待复核'
                        : '待分析'
                  }}
                </span>
              </header>

              <div class="pipeline-card-body">
                <div class="pipeline-meta-tags">
                  <span class="meta-chip">
                    <strong>负责人：</strong>{{ item.owner || '未分配' }}
                  </span>
                  <span v-if="item.stage === 'hired'" class="meta-chip is-highlight">
                    <strong>入职时间：</strong>{{ formatShortDate(item.hiredAt || item.lastFollowUpAt || item.createdAt) }}
                  </span>
                  <span v-else-if="item.nextActionAt" class="meta-chip is-highlight">
                    <strong>下一步跟进：</strong>{{ formatShortDate(item.nextActionAt) }}
                  </span>
                  <span v-else class="meta-chip">
                    <strong>最近更新：</strong>{{ formatShortDate(item.lastFollowUpAt || item.createdAt) }}
                  </span>
                </div>

                <p v-if="item.lastFollowUpNote" class="pipeline-note-snippet" :title="item.lastFollowUpNote">
                  备注：{{ item.lastFollowUpNote }}
                </p>
              </div>

              <footer class="pipeline-card-footer">
                <button
                  v-if="item.stage === 'offer'"
                  type="button"
                  class="pipeline-action-btn is-primary"
                  :disabled="quickBusyId === item.projectCandidateId"
                  @click="quickMarkHired(item)"
                >
                  {{ quickBusyId === item.projectCandidateId ? '处理中...' : '确认入职' }}
                </button>
                <button
                  type="button"
                  class="pipeline-action-btn"
                  @click="edit(item)"
                >
                  {{ item.stage === 'offer' ? '跟进 / 备注' : '更新跟进详情' }}
                </button>
              </footer>
            </article>
          </div>

          <!-- 空状态 -->
          <div v-else class="pipeline-empty-state">
            <div class="pipeline-empty-icon" aria-hidden="true">
              {{ col.stage === 'offer' ? '📋' : '🎉' }}
            </div>
            <h3>{{ col.emptyTitle }}</h3>
            <p>{{ col.emptyDesc }}</p>
            <div class="pipeline-empty-actions">
              <el-button
                v-if="col.stage === 'hired' && hiredMonthFilter === 'current' && totalHiredCount > 0"
                size="small"
                type="primary"
                plain
                @click="setHiredMonthFilter('all')"
              >
                查看全部已入职 ({{ totalHiredCount }} 人)
              </el-button>
              <el-button
                v-else
                size="small"
                @click="router.push('/talent')"
              >
                去简历人才库挑选人选
              </el-button>
            </div>
          </div>
        </section>
      </div>
    </main>

    <!-- 更新候选人跟进弹窗（已移除 Offer 管理跳转按钮） -->
    <el-dialog v-model="dialogOpen" title="更新候选人跟进与入职状态" width="560px">
      <template v-if="editing">
        <div class="dialog-candidate-banner">
          <span class="pipeline-avatar" aria-hidden="true">{{ getCandidateSurname(editing) }}</span>
          <div>
            <strong>{{ editing.name }}</strong>
            <div class="wb-muted" style="font-size: 12px">{{ editing.jobTitle || editing.projectName }}</div>
          </div>
        </div>

        <div class="wb-form-grid">
          <div>
            <label>当前阶段</label>
            <select v-model="form.stage">
              <option v-for="stage in allStages" :key="stage" :value="stage">
                {{ stageLabels[stage] }}
              </option>
            </select>
          </div>
          <div>
            <label>负责人</label>
            <input v-model="form.owner" placeholder="输入 HR 或负责人姓名" />
          </div>
          <div class="full">
            <label>下一步跟进 / 预计报到时间</label>
            <input v-model="form.nextActionAt" type="datetime-local" />
          </div>
          <div class="full">
            <label>跟进与入职备注</label>
            <textarea
              v-model="form.note"
              rows="3"
              placeholder="可直接记录薪资意向、拟入职日期、报到材料准备情况或阶段调整依据"
            />
          </div>
        </div>

        <h3 style="font-size: 14px; margin-top: 20px">记录沟通</h3>
        <div class="wb-actions">
          <el-select v-model="communicationChannel" style="width: 130px">
            <el-option label="平台" value="platform" />
            <el-option label="电话" value="phone" />
            <el-option label="微信" value="wechat" />
            <el-option label="邮件" value="email" />
            <el-option label="其他" value="other" />
          </el-select>
          <el-input v-model="communicationText" placeholder="填写实际沟通内容" />
          <el-button :disabled="!communicationText.trim()" @click="saveCommunication">记录</el-button>
        </div>

        <h3 style="font-size: 14px; margin-top: 22px">历史跟进记录</h3>
        <div v-if="events.length" class="event-list">
          <div v-for="event in events" :key="event.id" class="event-item">
            <div>
              <strong>
                {{
                  event.type.startsWith('communication:')
                    ? '沟通记录'
                    : event.type === 'note'
                      ? '备注'
                      : historyStageLabel(event.toStage)
                }}
              </strong>
              <span class="wb-muted"> · {{ event.actor || 'HR' }} · {{ formatDate(event.createdAt) }}</span>
            </div>
            <div v-if="event.note">{{ event.note }}</div>
          </div>
        </div>
        <div v-else class="wb-muted">暂无跟进记录</div>
      </template>
      <template #footer>
        <el-button @click="dialogOpen = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存跟进</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { useProjectStore } from '../composables/useProjectStore'
import { useRecruitmentStore, stageLabels, type TalentCandidate } from '../composables/useRecruitmentStore'
import { getCandidateSurname } from '../utils/candidate'

const boardStages = ['offer', 'hired'] as const
const allStages = ['offer', 'hired', 'second_interview', 'first_interview', 'pending_contact', 'new']

const route = useRoute()
const router = useRouter()
const projects = useProjectStore()
const recruitment = useRecruitmentStore()

const projectId = ref(String(route.query.project || ''))
const keyword = ref('')
const hiredMonthFilter = ref<'all' | 'current'>(route.query.hiredMonth === 'current' ? 'current' : 'all')
const kanbanEl = ref<HTMLElement | null>(null)
let openedCandidateId = ''

const dialogOpen = ref(false)
const editing = ref<TalentCandidate | null>(null)
const saving = ref(false)
const quickBusyId = ref('')
const events = ref<Array<{ id: string; type: string; toStage: string; note: string; actor: string; createdAt: string }>>([])
const form = reactive({ stage: 'offer', owner: '', nextActionAt: '', note: '' })
const communicationChannel = ref('platform')
const communicationText = ref('')

watch(
  () => route.query.project,
  value => {
    projectId.value = String(value || '')
  }
)
watch(
  () => route.query.hiredMonth,
  value => {
    hiredMonthFilter.value = value === 'current' ? 'current' : 'all'
  }
)
watch(
  () => route.query.candidate,
  () => {
    void openLinkedCandidate()
  }
)
watch(
  () => route.query.stage,
  () => {
    void scrollToLinkedStage()
  }
)

function matchesFilter(item: TalentCandidate) {
  if (projectId.value && item.projectId !== projectId.value) return false
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return true
  const hay = `${item.name || ''} ${item.jobTitle || ''} ${item.projectName || ''} ${item.owner || ''}`.toLowerCase()
  return hay.includes(kw)
}

function isHiredInCurrentMonth(item: TalentCandidate) {
  const raw = item.hiredAt || item.lastFollowUpAt || item.createdAt
  if (!raw) return false
  const hiredAt = new Date(raw)
  if (Number.isNaN(hiredAt.getTime())) return false
  const now = new Date()
  return hiredAt.getFullYear() === now.getFullYear() && hiredAt.getMonth() === now.getMonth()
}

const offerCandidates = computed(() =>
  recruitment.candidates.filter(item => item.stage === 'offer' && matchesFilter(item))
)

const allHiredCandidates = computed(() =>
  recruitment.candidates.filter(item => item.stage === 'hired' && matchesFilter(item))
)

const currentMonthHiredCandidates = computed(() =>
  allHiredCandidates.value.filter(item => isHiredInCurrentMonth(item))
)

const totalHiredCount = computed(() => allHiredCandidates.value.length)
const currentMonthHiredCount = computed(() => currentMonthHiredCandidates.value.length)

const displayedHiredCandidates = computed(() =>
  hiredMonthFilter.value === 'current' ? currentMonthHiredCandidates.value : allHiredCandidates.value
)

const boardColumns = computed(() => [
  {
    stage: 'offer',
    title: 'Offer 阶段',
    subtitle: '已发放意向，待候选人确认或办理入职',
    items: offerCandidates.value,
    emptyTitle: '暂无 Offer 阶段人选',
    emptyDesc: '可在简历人才库或面试安排中将通过面试的候选人推进至 Offer 阶段'
  },
  {
    stage: 'hired',
    title: '已入职',
    subtitle: hiredMonthFilter.value === 'current' ? '展示本月完成入职确认的人选' : '展示全部已完成入职确认的人选',
    items: displayedHiredCandidates.value,
    emptyTitle: hiredMonthFilter.value === 'current' ? '本月暂无已入职人选' : '暂无已入职人选',
    emptyDesc:
      hiredMonthFilter.value === 'current'
        ? '本月尚无候选人进入入职阶段，您可点击上方切换查看「全部入职」或将 Offer 人选确认入职'
        : '当候选人完成报到后，在左侧点击「确认入职」即可归档至本列'
  }
])

function syncRoute() {
  const nextQuery: Record<string, string> = {}
  if (projectId.value) nextQuery.project = projectId.value
  if (hiredMonthFilter.value === 'current') {
    nextQuery.stage = 'hired'
    nextQuery.hiredMonth = 'current'
  }
  void router.replace({ path: '/pipeline', query: nextQuery })
}

function setHiredMonthFilter(mode: 'all' | 'current') {
  hiredMonthFilter.value = mode
  syncRoute()
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString('zh-CN') : ''
}

function formatShortDate(value: string) {
  if (!value) return '暂无记录'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function historyStageLabel(value: string) {
  return (
    stageLabels[value] ||
    ({
      communicating: '沟通中（历史）',
      on_hold: '暂缓（历史）',
      rejected: '淘汰（历史）'
    } as Record<string, string>)[value] ||
    value
  )
}

async function openLinkedCandidate() {
  const id = String(route.query.candidate || '')
  if (!id || id === openedCandidateId) return
  const item = recruitment.candidates.find(
    candidate => candidate.projectCandidateId === id && (!projectId.value || candidate.projectId === projectId.value)
  )
  if (!item) return
  openedCandidateId = id
  await edit(item)
}

async function scrollToLinkedStage() {
  const stage = String(route.query.stage || '')
  if (!boardStages.includes(stage as any)) return
  await nextTick()
  const column = Array.from(kanbanEl.value?.querySelectorAll<HTMLElement>('[data-stage]') || []).find(
    element => element.dataset.stage === stage
  )
  column?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
}

async function quickMarkHired(item: TalentCandidate) {
  if (quickBusyId.value) return
  quickBusyId.value = item.projectCandidateId
  try {
    await recruitment.updateFollowUp(
      item,
      'hired',
      item.owner || 'HR',
      '',
      item.lastFollowUpNote || '已确认入职'
    )
    await recruitment.loadDashboard()
    ElMessage.success(`已将「${item.name}」标记为已入职`)
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    quickBusyId.value = ''
  }
}

async function edit(item: TalentCandidate) {
  editing.value = item
  form.stage = item.stage
  form.owner = item.owner
  form.nextActionAt = item.nextActionAt ? localDateTime(item.nextActionAt) : ''
  form.note = item.lastFollowUpNote || ''
  events.value = []
  dialogOpen.value = true
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    events.value = (await api.ListFollowUpEvents(item.projectCandidateId)) || []
  } catch {
    /* Existing card remains editable if history cannot load. */
  }
}

function localDateTime(value: string) {
  const date = new Date(value)
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

async function save() {
  if (!editing.value) return
  saving.value = true
  try {
    const next = form.nextActionAt ? new Date(form.nextActionAt).toISOString() : ''
    await recruitment.updateFollowUp(editing.value, form.stage, form.owner, next, form.note)
    await recruitment.loadDashboard()
    dialogOpen.value = false
    ElMessage.success('跟进记录已保存')
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    saving.value = false
  }
}

async function saveCommunication() {
  if (!editing.value || !communicationText.value.trim()) return
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await api.AddCandidateCommunication(
      editing.value.projectCandidateId,
      communicationChannel.value,
      communicationText.value,
      'HR'
    )
    events.value = (await api.ListFollowUpEvents(editing.value.projectCandidateId)) || []
    communicationText.value = ''
    ElMessage.success('沟通记录已保存')
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

onMounted(async () => {
  await Promise.all([projects.loadProjects(), recruitment.loadCandidates(), recruitment.loadDashboard()])
  await Promise.all([openLinkedCandidate(), scrollToLinkedStage()])
})
</script>

<style scoped>
.pipeline-toolbar-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 18px 20px;
  margin-bottom: 18px;
}

.pipeline-stats-row {
  display: flex;
  flex-wrap: wrap;
  flex-wrap: balance;
  gap: 12px;
}

.pipeline-stat-pill {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  border-radius: 12px;
  border: 1px solid var(--wb-border);
  background: var(--wb-surface-soft);
}

.pipeline-stat-pill .stat-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #8b5cf6;
}

.pipeline-stat-pill.is-month-hired .stat-dot {
  background: #10b981;
}

.pipeline-stat-pill.is-total-hired .stat-dot {
  background: #3b82f6;
}

.pipeline-stat-pill .stat-label {
  font-size: 13px;
  color: var(--wb-muted);
}

.pipeline-stat-pill .stat-value {
  font-size: 18px;
  font-weight: 800;
  color: var(--wb-text);
}

.pipeline-filter-row {
  display: flex;
  flex-wrap: wrap;
  align-items: safe center;
  gap: 12px;
}

.pipeline-project-select {
  width: 260px;
}

.pipeline-search-input {
  width: 280px;
}

.pipeline-scope-switch {
  display: inline-flex;
  margin-inline-start: auto;
  padding: 3px;
  border-radius: 10px;
  border: 1px solid var(--wb-border);
  background: var(--wb-surface-soft);
}

.scope-btn {
  padding: 6px 14px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--wb-muted);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.scope-btn.active {
  background: var(--wb-surface);
  color: var(--wb-brand);
  box-shadow: 0 1px 4px rgba(15, 23, 42, 0.08);
}

.pipeline-two-col-board {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 18px;
  align-items: start;
}

.pipeline-column {
  min-inline-size: 0;
  border-radius: 16px;
  border: 1px solid var(--wb-border);
  background: var(--wb-surface-soft);
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.pipeline-column.is-offer {
  border-top: 3px solid #8b5cf6;
}

.pipeline-column.is-hired {
  border-top: 3px solid #10b981;
}

.pipeline-column-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--wb-border);
}

.pipeline-column-title-wrap {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-inline-size: 0;
}

.pipeline-column-dot {
  width: 11px;
  height: 11px;
  margin-top: 5px;
  flex: none;
  border-radius: 50%;
  background: #8b5cf6;
}

.pipeline-column.is-hired .pipeline-column-dot {
  background: #10b981;
}

.pipeline-column-title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.pipeline-column-title-row h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 800;
  color: var(--wb-text);
}

.pipeline-filter-tag {
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(16, 185, 129, 0.14);
  color: #059669;
  font-size: 11px;
  font-weight: 700;
}

.pipeline-column-subtitle {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--wb-muted);
}

.pipeline-column-count {
  flex: none;
  padding: 4px 11px;
  border-radius: 999px;
  background: var(--wb-surface);
  border: 1px solid var(--wb-border);
  font-size: 12px;
  font-weight: 700;
  color: var(--wb-text);
}

.pipeline-card-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-block-size: 620px;
  overflow-y: auto;
  scrollbar-gutter: stable;
  overscroll-behavior: contain;
}

.pipeline-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border-radius: 14px;
  border: 1px solid var(--wb-border);
  background: var(--wb-surface);
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.pipeline-card:hover {
  border-color: var(--wb-brand);
  box-shadow: 0 6px 18px rgba(15, 23, 42, 0.07);
}

.pipeline-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.pipeline-card-identity {
  display: flex;
  align-items: center;
  gap: 12px;
  min-inline-size: 0;
}

.pipeline-avatar {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  flex: none;
  border-radius: 50%;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  color: #ffffff;
  font-size: 15px;
  font-weight: 700;
}

.pipeline-card.is-hired .pipeline-avatar {
  background: linear-gradient(135deg, #059669, #10b981);
}

.pipeline-name-block {
  min-inline-size: 0;
}

.pipeline-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-inline-size: 0;
}

.pipeline-name-row h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--wb-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipeline-stage-badge {
  flex: none;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  background: rgba(139, 92, 246, 0.12);
  color: #7c3aed;
}

.pipeline-stage-badge.is-hired {
  background: rgba(16, 185, 129, 0.14);
  color: #059669;
}

.pipeline-role-meta {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--wb-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipeline-score-pill {
  flex: none;
  padding: 3px 9px;
  border-radius: 999px;
  background: var(--wb-brand-soft);
  color: var(--wb-brand);
  font-size: 11px;
  font-weight: 700;
}

.pipeline-card-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pipeline-meta-tags {
  display: flex;
  flex-wrap: wrap;
  flex-wrap: balance;
  gap: 8px;
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 9px;
  border-radius: 8px;
  border: 1px solid var(--wb-border);
  background: var(--wb-surface-soft);
  font-size: 12px;
  color: var(--wb-muted);
}

.meta-chip strong {
  font-weight: 600;
  color: var(--wb-text);
}

.meta-chip.is-highlight {
  border-color: rgba(99, 102, 241, 0.25);
  background: var(--wb-brand-soft);
  color: var(--wb-brand);
}

.pipeline-note-snippet {
  margin: 0;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--wb-surface-soft);
  color: var(--wb-text);
  font-size: 12px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  line-clamp: 2;
  overflow: clip;
}

.pipeline-card-footer {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid var(--wb-border);
}

.pipeline-action-btn {
  padding: 6px 13px;
  border-radius: 8px;
  border: 1px solid var(--wb-border);
  background: var(--wb-surface);
  color: var(--wb-text);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.pipeline-action-btn:hover:not(:disabled) {
  border-color: var(--wb-brand);
  color: var(--wb-brand);
}

.pipeline-action-btn.is-primary {
  border-color: #10b981;
  background: #10b981;
  color: #ffffff;
}

.pipeline-action-btn.is-primary:hover:not(:disabled) {
  background: #059669;
  border-color: #059669;
  color: #ffffff;
}

.pipeline-action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.pipeline-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 38px 20px;
  border-radius: 14px;
  border: 1px dashed var(--wb-border);
  background: var(--wb-surface);
}

.pipeline-empty-icon {
  font-size: 28px;
  margin-bottom: 8px;
}

.pipeline-empty-state h3 {
  margin: 0 0 6px;
  font-size: 15px;
  font-weight: 700;
  color: var(--wb-text);
}

.pipeline-empty-state p {
  margin: 0 0 16px;
  max-width: 340px;
  font-size: 12px;
  line-height: 1.55;
  color: var(--wb-muted);
}

.dialog-candidate-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  margin-bottom: 16px;
  border-radius: 10px;
  background: var(--wb-surface-soft);
  border: 1px solid var(--wb-border);
}

.event-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-block-size: 180px;
  overflow-y: auto;
  scrollbar-gutter: stable;
  overscroll-behavior: contain;
}

.event-item {
  border-left: 2px solid var(--wb-brand);
  padding: 3px 10px;
  font-size: 12px;
}

@media (max-width: 820px) {
  .pipeline-scope-switch {
    margin-inline-start: 0;
  }
  .pipeline-project-select,
  .pipeline-search-input {
    width: 100%;
  }
}
</style>
