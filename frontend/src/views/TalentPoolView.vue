<template>
  <div class="workbench-page talent-pool-page">
    <!-- 顶部全局工具头 -->
    <header class="wb-header">
      <div>
        <h1>简历人才库</h1>
        <div class="wb-crumb">人才检索 / 按职位分库查看与调配</div>
      </div>
      <div class="wb-actions">
        <el-button @click="refresh" :loading="recruitment.loading">
          <el-icon style="margin-right: 4px;"><Refresh /></el-icon>刷新
        </el-button>
        <el-button :loading="importing" @click="import51Job">一键导入 51job 投递</el-button>
        <el-button @click="showInbox">
          待分配投递<span v-if="pendingApplications.length">（{{ pendingApplications.length }}）</span>
        </el-button>
        <el-button @click="showDuplicates">
          疑似重复<span v-if="duplicates.length">（{{ duplicates.length }}）</span>
        </el-button>
        <el-button :disabled="!selectedProjectId" @click="showOutreach = true">向前十名打招呼</el-button>
        <el-button type="primary" :disabled="!selectedProjectId" @click="showSearch = true">
          <el-icon style="margin-right: 4px;"><Position /></el-icon>矩阵寻才
        </el-button>
      </div>
    </header>

    <!-- 主体分栏：左侧职位分库导航 + 右侧候选人看板 -->
    <div class="talent-workbench-layout">
      <!-- 左侧：职位分库导航侧栏 -->
      <aside class="job-repositories-sidebar">
        <div class="repo-header">
          <div class="repo-title">职位分库导航</div>
          <el-button
            link
            type="primary"
            class="add-repo-link"
            @click="router.push('/jobs')"
            title="管理或新建职位"
          >
            <el-icon><Plus /></el-icon>新建分库
          </el-button>
        </div>

        <div class="repo-search">
          <el-input
            v-model="repoFilterKeyword"
            placeholder="筛选职位分库…"
            clearable
            size="small"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </div>

        <div class="repo-list">
          <!-- 全部人才库 (总览汇总) -->
          <div
            class="repo-item"
            :class="{ active: !selectedProjectId }"
            @click="selectRepository('')"
          >
            <div class="repo-icon all-repo-icon">
              <el-icon><Collection /></el-icon>
            </div>
            <div class="repo-info">
              <span class="repo-name">全部人才库 (汇总)</span>
              <span class="repo-sub">跨职位全部人选</span>
            </div>
            <span class="repo-badge">{{ recruitment.candidates.length }}</span>
          </div>

          <div class="repo-divider">
            <span>各职位专属分库 ({{ filteredProjectRepos.length }})</span>
          </div>

          <!-- 各职位专属分库项 -->
          <div
            v-for="proj in filteredProjectRepos"
            :key="proj.id"
            class="repo-item"
            :class="{ active: selectedProjectId === proj.id }"
            @click="selectRepository(proj.id)"
          >
            <div class="repo-status-indicator" :class="normalizeRepoStatusClass(proj.status)"></div>
            <div class="repo-info">
              <span class="repo-name" :title="proj.job_config?.title || proj.name">
                {{ proj.job_config?.title || proj.name }}
              </span>
              <span class="repo-sub">
                {{ proj.department || '综合' }} · {{ proj.job_config?.city || '全国' }}
              </span>
            </div>
            <span class="repo-badge" :class="{ 'has-count': countFor(proj.id) > 0 }">
              {{ countFor(proj.id) }}
            </span>
          </div>

          <div v-if="!filteredProjectRepos.length" class="empty-repos">
            无匹配职位分库
          </div>
        </div>
      </aside>

      <!-- 右侧：候选人看板主内容区 -->
      <main class="talent-content-area">
        <!-- 错误提示（过滤掉旧简历脏数据干扰） -->
        <el-alert
          v-if="recruitment.error && !recruitment.error.includes('引用缺失的旧简历')"
          type="warning"
          :title="recruitment.error"
          show-icon
          closable
          @close="recruitment.error = ''"
          style="margin-bottom: 16px; margin-right: 8px;"
        />

        <!-- 当前分库信息横幅 (选定职位分库时展示) -->
        <div v-if="selectedProject" class="selected-repo-banner">
          <div class="repo-banner-info">
            <div class="repo-banner-title">
              <h2>{{ selectedProject.job_config?.title || selectedProject.name }} 专属人才库</h2>
              <span class="banner-status-tag" :class="normalizeRepoStatusClass(selectedProject.status)">
                {{ normalizeStatusLabel(selectedProject.status) }}
              </span>
            </div>
            <div class="repo-banner-meta">
              <span>部门：{{ selectedProject.department || '未分配' }}</span>
              <span class="sep">·</span>
              <span>城市：{{ selectedProject.job_config?.city || '上海' }}</span>
              <span class="sep">·</span>
              <span>性质：{{ selectedProject.job_config?.employment_type || '全职' }}</span>
              <span class="sep">·</span>
              <span>薪资：{{ selectedProject.job_config?.salary_range || '15-25K' }}</span>
              <span class="sep">·</span>
              <span>计划编制：{{ selectedProject.headcount || 1 }} 人</span>
              <span class="sep">·</span>
              <span>库内人选：<strong>{{ countFor(selectedProject.id) }}</strong> 位</span>
            </div>
          </div>
          <div class="repo-banner-actions">
            <el-button size="small" @click="router.push('/jobs')">职位设置</el-button>
            <el-button size="small" type="primary" plain @click="router.push('/project/' + selectedProject.id)">
              项目完整评测
            </el-button>
          </div>
        </div>

        <!-- 筛选与搜索工具条 -->
        <div class="wb-toolbar talent-filter-toolbar">
          <el-input
            v-model="keyword"
            clearable
            placeholder="搜索姓名、岗位或项目…"
            style="width: 240px"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>

          <el-select v-model="statusFilter" placeholder="分析状态" clearable style="width: 140px">
            <el-option label="已抓取／待分析" value="pending" />
            <el-option label="待补全简历" value="pending_document" />
            <el-option label="分析中" value="analyzing" />
            <el-option label="已完成" value="done" />
            <el-option label="失败" value="error" />
          </el-select>

          <el-select v-model="sourceFilter" placeholder="全部来源" clearable style="width: 130px">
            <el-option v-for="source in sources" :key="source" :label="source" :value="source" />
          </el-select>

          <el-select v-model="stageFilter" placeholder="招聘阶段" clearable style="width: 130px">
            <el-option
              v-for="(label, key) in stageLabels"
              :key="key"
              :label="label"
              :value="key"
            />
          </el-select>

          <span class="wb-muted count-summary">
            当前分库呈现 <strong>{{ filtered.length }}</strong> 位候选人
          </span>
        </div>

        <!-- 候选人看板网格 -->
        <div v-if="filtered.length" class="wb-candidate-grid">
          <article
            v-for="(item, index) in filtered"
            :key="item.projectCandidateId"
            class="wb-card wb-candidate talent-card"
          >
            <div class="wb-candidate-top">
              <span class="wb-avatar candidate-sequence" :title="`当前列表第 ${index + 1} 位`">{{ index + 1 }}</span>
              <div class="candidate-headline">
                <div class="wb-candidate-name">{{ item.name }}</div>
                <div class="wb-candidate-role">
                  {{ item.jobTitle || item.projectName }} · {{ item.projectName }}
                </div>
              </div>
              <span v-if="item.analysisStatus === 'done'" class="wb-score" title="核心能力分">
                {{ item.abilityScore }}
              </span>
            </div>

            <div class="wb-candidate-meta">
              <span class="wb-pill">{{ item.source }}</span>
              <span class="wb-pill" :class="statusClass(item.analysisStatus)">
                {{ statusLabel(item.analysisStatus) }}
              </span>
              <span class="wb-pill" :class="redLineClass(item.redLineStatus)">
                红线：{{ redLineLabel(item.redLineStatus) }}
              </span>
              <span class="wb-pill stage-pill">{{ stageLabels[item.stage] || item.stage }}</span>
            </div>

            <div class="wb-muted candidate-owner-line">
              <span>负责人：{{ item.owner || '未分配' }}</span>
              <span v-if="item.nextActionAt"> · 下次跟进 {{ formatDate(item.nextActionAt) }}</span>
            </div>

            <div class="wb-candidate-foot">
              <span class="wb-muted入库时间">入库 {{ formatDate(item.createdAt) }}</span>
              <div class="wb-actions">
                <el-button size="small" @click="selected = item">查看档案</el-button>
                <el-button size="small" @click="openMoveDialog(item)">调配分库</el-button>
                <el-button
                  v-if="item.analysisStatus === 'error'"
                  size="small"
                  type="warning"
                  plain
                  @click="retryAnalysis(item.resumeId)"
                >
                  重试
                </el-button>
                <el-button
                  size="small"
                  type="primary"
                  plain
                  @click="router.push('/project/' + item.projectId)"
                >
                  评测
                </el-button>
              </div>
            </div>
          </article>
        </div>

        <!-- 空数据提示 -->
        <div v-else class="wb-card wb-empty empty-candidate-panel">
          <p v-if="recruitment.loading" class="empty-loading">
            <el-icon class="is-loading" style="font-size: 24px; margin-bottom: 8px;"><Loading /></el-icon>
            <span>正在载入人才库…</span>
          </p>
          <div v-else class="empty-detail-box">
            <div class="empty-illustration">
              <el-icon><Search /></el-icon>
            </div>
            <h3 class="empty-title">
              {{ selectedProject ? `「${selectedProject.job_config?.title || selectedProject.name}」分库中暂无符合条件的候选人` : '当前筛选条件下暂无候选人' }}
            </h3>
            <p class="empty-subtitle">
              该职位专属分库已就绪。点击下方按钮即可一键调起多平台矩阵寻才，或导入 51job 投递简历
            </p>
            <el-button
              v-if="selectedProject"
              type="primary"
              size="large"
              class="empty-action-btn"
              @click="showSearch = true"
            >
              <el-icon style="margin-right: 6px;"><Position /></el-icon>
              立即为该岗位检索人才
            </el-button>
          </div>
        </div>
      </main>
    </div>

    <!-- 候选人档案抽屉 -->
    <el-drawer v-model="drawerOpen" title="候选人档案" size="480px">
      <template v-if="selected">
        <h2>{{ selected.name }}</h2>
        <p class="wb-muted">{{ selected.projectName }} · {{ selected.jobTitle }}</p>
        <div class="wb-candidate-meta" style="margin: 18px 0">
          <span class="wb-pill">{{ selected.source }}</span>
          <span class="wb-pill">{{ stageLabels[selected.stage] || selected.stage }}</span>
          <span class="wb-pill" :class="redLineClass(selected.redLineStatus)">
            红线：{{ redLineLabel(selected.redLineStatus) }}
          </span>
        </div>
        <p>能力分：<strong>{{ selected.analysisStatus === 'done' ? selected.abilityScore : '待评估' }}</strong></p>
        <p>已确认加分项：{{ selected.confirmedBonus }}</p>
        <p>负责人：{{ selected.owner || '未分配' }}</p>
        <p v-if="selected.nextActionAt">下次跟进：{{ formatDate(selected.nextActionAt) }}</p>
        <div class="wb-actions" style="margin-top: 22px">
          <el-button v-if="selected.profileUrl" @click="openProfile(selected.profileUrl)">平台主页</el-button>
          <el-button type="primary" plain @click="openMoveDialog(selected)">调配至其他职位库</el-button>
          <el-button type="primary" @click="router.push('/project/' + selected!.projectId)">打开完整评测</el-button>
        </div>
        <div class="wb-card" style="margin-top: 22px">
          <strong>核实联系方式</strong>
          <p class="wb-muted">仅在 HR 已核对该联系方式属于本人时填写；相同已核实联系方式会合并统一档案。</p>
          <div class="wb-actions">
            <el-select v-model="contactKind" style="width: 100px">
              <el-option label="邮箱" value="email" />
              <el-option label="电话" value="phone" />
            </el-select>
            <el-input v-model="verifiedContact" placeholder="已核实的联系方式" />
            <el-button @click="verifyContact">核实</el-button>
          </div>
        </div>
        <section v-if="profile" style="margin-top: 22px">
          <h3>关联岗位与独立评测</h3>
          <div v-for="link in profile.projects" :key="link.projectCandidateId" class="wb-card" style="margin-bottom: 8px">
            <strong>{{ link.projectName }}</strong><span class="wb-muted"> · {{ link.jobTitle }}</span>
            <p class="wb-muted">
              能力 {{ link.analysisStatus === 'done' ? link.abilityScore : '待评估' }} ·
              红线 {{ redLineLabel(link.redLineStatus) }} ·
              {{ stageLabels[link.stage] || link.stage }}
            </p>
          </div>
          <h3>来源与简历版本</h3>
          <p v-for="source in profile.sources" :key="source.platform + source.platformCandidateId" class="wb-muted">
            {{ source.platform === 'legacy' ? '本地档案' : source.platform }} · {{ source.platformCandidateId }}
          </p>
          <p v-for="resume in profile.resumes" :key="resume.id" class="wb-muted">
            {{ resume.fileName }} · {{ formatDate(resume.createdAt) }}
          </p>
          <h3>跨岗位沟通与跟进</h3>
          <p v-if="!profile.timeline.length" class="wb-muted">暂无记录</p>
          <div v-for="event in profile.timeline" :key="event.id" class="wb-card" style="margin-bottom: 8px">
            <strong>{{ event.projectName }}</strong>
            <span class="wb-muted"> · {{ event.actor || 'HR' }} · {{ formatDate(event.createdAt) }}</span>
            <p class="wb-muted">{{ event.note || event.type }}</p>
          </div>
        </section>
      </template>
    </el-drawer>

    <!-- 调配至其他职位库弹窗 -->
    <el-dialog
      v-model="moveDialogOpen"
      :title="`候选人调配 · ${candidateToMove?.name || ''}`"
      width="480px"
    >
      <p class="wb-muted" style="margin-bottom: 16px;">
        将候选人<strong>【{{ candidateToMove?.name }}】</strong>从当前职位库流转至新的目标职位库。候选人的评测记录、简历版本和跟进轨迹将完整保留。
      </p>
      <el-form label-position="top">
        <el-form-item label="选择目标职位分库 *" required>
          <el-select
            v-model="targetProjectId"
            placeholder="请选择要调配到的目标职位"
            style="width: 100%"
          >
            <el-option
              v-for="proj in targetProjectOptions"
              :key="proj.id"
              :label="`${proj.job_config?.title || proj.name} · ${proj.department || '未分配'}`"
              :value="proj.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="moveDialogOpen = false">取消</el-button>
        <el-button type="primary" :disabled="!targetProjectId" :loading="moving" @click="confirmMoveCandidate">
          确认调配
        </el-button>
      </template>
    </el-dialog>

    <!-- 寻才与打招呼对话框 -->
    <TopTenOutreachDialog v-model="showOutreach" :project-id="selectedProjectId" />
    <BossSearchDialog
      v-if="selectedProject"
      v-model="showSearch"
      :project-id="selectedProject.id"
      :project-name="selectedProject.name"
      :job-title="selectedProject.job_config?.title || selectedProject.name"
      :exp-years="selectedProject.job_config?.experience_years || 0"
      :edu-level="selectedProject.job_config?.education_level || ''"
      @refresh="refresh"
    />

    <!-- 待分配投递抽屉 -->
    <el-drawer v-model="inboxOpen" title="主动投递 · 待分配" size="500px">
      <div v-if="!pendingApplications.length" class="wb-empty">暂无待分配投递</div>
      <article
        v-for="application in pendingApplications"
        :key="application.id"
        class="wb-card"
        style="margin-bottom: 12px"
      >
        <strong>{{ application.candidateName }}</strong>
        <p class="wb-muted">
          51job 职位 ID：{{ application.platformJobId || '平台未返回' }} · 投递 ID：{{ application.applicationId }}
        </p>
        <div class="wb-actions">
          <el-select
            v-model="assignment[application.id]"
            placeholder="选择正确项目"
            style="width: 230px"
          >
            <el-option
              v-for="project in projects.projects"
              :key="project.id"
              :label="`${project.job_config.title || project.name} · ${project.name}`"
              :value="project.id"
            />
          </el-select>
          <el-button
            type="primary"
            :disabled="!assignment[application.id]"
            @click="assign(application.id)"
          >
            确认分配
          </el-button>
        </div>
      </article>
    </el-drawer>

    <!-- 疑似重复抽屉 -->
    <el-drawer v-model="duplicatesOpen" title="疑似重复 · HR 核对" size="500px">
      <div v-if="!duplicates.length" class="wb-empty">暂无疑似重复档案</div>
      <article
        v-for="pair in duplicates"
        :key="pair.candidateId + pair.otherCandidateId"
        class="wb-card"
        style="margin-bottom: 12px"
      >
        <strong>{{ pair.name }}</strong>
        <p class="wb-muted">{{ pair.reason }}</p>
        <p class="wb-muted">
          档案 A：{{ pair.candidateId }}<br />
          档案 B：{{ pair.otherCandidateId }}
        </p>
        <el-input
          v-model="mergeReasons[pair.candidateId + pair.otherCandidateId]"
          placeholder="请填写已核实的合并依据"
        />
        <el-button
          style="margin-top: 8px"
          :disabled="!mergeReasons[pair.candidateId + pair.otherCandidateId]?.trim()"
          @click="merge(pair)"
        >
          确认将 B 合并至 A
        </el-button>
      </article>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Refresh,
  Position,
  Search,
  Plus,
  Collection
} from '@element-plus/icons-vue'
import { useProjectStore, type Project } from '../composables/useProjectStore'
import { useRecruitmentStore, stageLabels, type TalentCandidate } from '../composables/useRecruitmentStore'
import BossSearchDialog from '../components/BossSearchDialog.vue'
import TopTenOutreachDialog from '../components/TopTenOutreachDialog.vue'
import { ElMessage } from 'element-plus'

interface PendingApplication { id: string; applicationId: string; candidateName: string; platformJobId: string }
interface DuplicatePair { candidateId: string; otherCandidateId: string; name: string; reason: string }
interface CandidateProfile {
  projects: TalentCandidate[]
  sources: Array<{ platform: string; platformCandidateId: string }>
  resumes: Array<{ id: string; fileName: string; createdAt: string }>
  timeline: Array<{ id: string; projectName: string; type: string; note: string; actor: string; createdAt: string }>
}

const route = useRoute()
const router = useRouter()
const projects = useProjectStore()
const recruitment = useRecruitmentStore()

const selectedProjectId = ref(String(route.query.project || ''))
const repoFilterKeyword = ref('')
const keyword = ref('')
const statusFilter = ref('')
const sourceFilter = ref('')
const stageFilter = ref('')

const showSearch = ref(false)
const showOutreach = ref(false)
const importing = ref(false)
const inboxOpen = ref(false)
const pendingApplications = ref<PendingApplication[]>([])
const assignment = ref<Record<string, string>>({})
const duplicates = ref<DuplicatePair[]>([])
const duplicatesOpen = ref(false)
const mergeReasons = ref<Record<string, string>>({})

const contactKind = ref('email')
const verifiedContact = ref('')
const selected = ref<TalentCandidate | null>(null)
const profile = ref<CandidateProfile | null>(null)
const drawerOpen = computed({
  get: () => !!selected.value,
  set: value => { if (!value) selected.value = null }
})

// 调配分库逻辑
const moveDialogOpen = ref(false)
const candidateToMove = ref<TalentCandidate | null>(null)
const targetProjectId = ref('')
const moving = ref(false)

const selectedProject = computed(() =>
  projects.projects.find(p => p.id === selectedProjectId.value)
)

const filteredProjectRepos = computed(() => {
  return projects.projects.filter(p => {
    if (!repoFilterKeyword.value) return true
    const kw = repoFilterKeyword.value.toLowerCase()
    const name = (p.job_config?.title || p.name || '').toLowerCase()
    const dept = (p.department || '').toLowerCase()
    return name.includes(kw) || dept.includes(kw)
  })
})

const targetProjectOptions = computed(() => {
  if (!candidateToMove.value) return projects.projects
  return projects.projects.filter(p => p.id !== candidateToMove.value?.projectId)
})

const sources = computed(() => [...new Set(recruitment.candidates.map(c => c.source))].sort())

const filtered = computed(() => recruitment.candidates.filter(item =>
  (!selectedProjectId.value || item.projectId === selectedProjectId.value) &&
  (!statusFilter.value || item.analysisStatus === statusFilter.value) &&
  (!sourceFilter.value || item.source === sourceFilter.value) &&
  (!stageFilter.value || item.stage === stageFilter.value) &&
  (!keyword.value || `${item.name} ${item.projectName} ${item.jobTitle}`.toLowerCase().includes(keyword.value.toLowerCase()))
))

watch(() => route.query.project, value => {
  selectedProjectId.value = String(value || '')
})

watch(selected, async item => {
  profile.value = null
  if (!item) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    profile.value = await api.GetCandidateProfile(item.id)
  } catch (err: any) {
    ElMessage.warning(err?.message || String(err))
  }
})

function countFor(id: string): number {
  return recruitment.candidates.filter(item => item.projectId === id).length
}

function selectRepository(id: string) {
  selectedProjectId.value = id
  syncRoute()
}

function syncRoute() {
  void router.replace({
    path: '/talent',
    query: selectedProjectId.value ? { project: selectedProjectId.value } : {}
  })
}

function openMoveDialog(candidate: TalentCandidate) {
  candidateToMove.value = candidate
  targetProjectId.value = ''
  moveDialogOpen.value = true
}

async function confirmMoveCandidate() {
  if (!candidateToMove.value || !targetProjectId.value) return
  moving.value = true
  try {
    await recruitment.moveCandidate(candidateToMove.value.projectCandidateId, targetProjectId.value)
    ElMessage.success('候选人已成功调配至目标职位分库！')
    moveDialogOpen.value = false
    candidateToMove.value = null
    await refresh()
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    moving.value = false
  }
}

function normalizeRepoStatusClass(status?: string): string {
  if (!status || status === 'publishing' || status === 'analyzing' || status === 'completed') return 'status-dot-publishing'
  if (status === 'draft') return 'status-dot-draft'
  if (status === 'paused') return 'status-dot-paused'
  return 'status-dot-publishing'
}

function normalizeStatusLabel(status?: string): string {
  if (!status || status === 'publishing' || status === 'analyzing' || status === 'completed') return '招聘中'
  if (status === 'draft') return '草稿'
  if (status === 'paused') return '已暂停'
  return status
}

function statusLabel(status: string) {
  return ({
    pending: '已抓取／待分析',
    pending_document: '待补全简历',
    analyzing: '分析中',
    done: '已完成',
    error: '失败'
  } as Record<string, string>)[status] || status
}

function statusClass(status: string) {
  return status === 'error' ? 'danger' : status === 'done' ? 'ok' : status === 'analyzing' ? 'warn' : ''
}

function redLineLabel(status: string) {
  return ({
    pending: '待核实',
    failed: '不满足',
    passed: '已满足',
    not_configured: '未配置'
  } as Record<string, string>)[status] || '待核实'
}

function redLineClass(status: string) {
  return status === 'failed' ? 'danger' : status === 'passed' ? 'ok' : 'warn'
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString('zh-CN') : ''
}

function openProfile(url: string) {
  window.open(url, '_blank', 'noopener,noreferrer')
}

async function refresh() {
  await Promise.all([projects.loadProjects(), recruitment.loadCandidates()])
}

async function loadInbox(interactive = false) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    pendingApplications.value = (await api.ListImportedApplications('unassigned')) || []
  } catch (err: any) {
    console.warn('loadInbox error:', err)
    if (interactive) ElMessage.error(err?.message || String(err))
  }
}

function showInbox() {
  inboxOpen.value = true
  void loadInbox(true)
}

async function import51Job() {
  importing.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const result = await api.ImportApplications('51job')
    ElMessage.success(`新增 ${result.addedCount}，重复 ${result.duplicateCount}，待分配 ${result.unassignedCount}，失败 ${result.failedCount}`)
    await Promise.all([refresh(), loadInbox()])
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    importing.value = false
  }
}

async function assign(id: string) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.AssignApplication(id, assignment.value[id])
    ElMessage.success('投递已分配')
    await Promise.all([refresh(), loadInbox()])
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

async function loadDuplicates(interactive = false) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    duplicates.value = (await api.ListSuspectedDuplicates()) || []
  } catch (err: any) {
    console.warn('loadDuplicates error:', err)
    if (interactive) ElMessage.error(err?.message || String(err))
  }
}

function showDuplicates() {
  duplicatesOpen.value = true
  void loadDuplicates(true)
}

async function merge(pair: DuplicatePair) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.MergeCandidates(pair.candidateId, pair.otherCandidateId, 'HR', mergeReasons.value[pair.candidateId + pair.otherCandidateId])
    ElMessage.success('档案已合并')
    await Promise.all([loadDuplicates(), refresh()])
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

async function verifyContact() {
  if (!selected.value || !verifiedContact.value.trim()) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.VerifyCandidateContact(selected.value.id, contactKind.value, verifiedContact.value.trim(), 'HR')
    ElMessage.success('联系方式已核实，统一档案已更新')
    verifiedContact.value = ''
    selected.value = null
    await Promise.all([loadDuplicates(), refresh()])
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

async function retryAnalysis(resumeId: string) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.RetryCandidateAnalysis(resumeId)
    ElMessage.success('已重新加入分析队列')
    await refresh()
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

onMounted(() => {
  void projects.loadProjects()
  void refresh()
  void loadInbox()
  void loadDuplicates()
})
</script>

<style scoped>
.talent-pool-page {
  box-sizing: border-box;
  width: 100%;
  padding: 20px 36px 60px 24px;
}

.talent-workbench-layout {
  display: flex;
  gap: 20px;
  align-items: flex-start;
  margin-top: 14px;
  width: 100%;
  box-sizing: border-box;
}

/* 左侧职位分库导航侧栏 */
.job-repositories-sidebar {
  width: 280px;
  min-width: 280px;
  background-color: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: calc(100vh - 140px);
  position: sticky;
  top: 20px;
}

.repo-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.repo-title {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.add-repo-link {
  font-size: 12px;
}

.repo-search {
  margin-bottom: 4px;
}

.repo-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  overflow-y: auto;
  padding-right: 2px;
}

.repo-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s ease;
  background-color: #ffffff;
  border: 1px solid transparent;
}

.repo-item:hover {
  background-color: #f8fafc;
}

.repo-item.active {
  background-color: #eef2ff;
  border-color: #c7d2fe;
}

.all-repo-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background-color: #ede9fe;
  color: #6366f1;
}

.repo-status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-dot-publishing {
  background-color: #22c55e;
}
.status-dot-draft {
  background-color: #f59e0b;
}
.status-dot-paused {
  background-color: #94a3b8;
}

.repo-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.repo-name {
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.repo-sub {
  font-size: 11px;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.repo-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 10px;
  background-color: #f1f5f9;
  color: #64748b;
  flex-shrink: 0;
}

.repo-badge.has-count {
  background-color: #e0e7ff;
  color: #4f46e5;
}

.repo-divider {
  font-size: 11px;
  font-weight: 600;
  color: #94a3b8;
  padding: 8px 4px 4px;
}

.empty-repos {
  text-align: center;
  font-size: 12px;
  color: #94a3b8;
  padding: 20px 0;
}

/* 右侧主看板区 */
.talent-content-area {
  flex: 1;
  min-width: 0;
  box-sizing: border-box;
  padding-right: 12px;
}

.selected-repo-banner {
  background: linear-gradient(135deg, #ffffff 0%, #f8faff 100%);
  border: 1px solid #e0e7ff;
  border-radius: 12px;
  padding: 16px 20px;
  margin-bottom: 16px;
  margin-right: 4px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  box-sizing: border-box;
}

.repo-banner-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.repo-banner-title h2 {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
  color: #1e293b;
}

.banner-status-tag {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
}

.banner-status-tag.status-dot-publishing {
  background-color: #dcfce7;
  color: #15803d;
}
.banner-status-tag.status-dot-draft {
  background-color: #fef3c7;
  color: #b45309;
}
.banner-status-tag.status-dot-paused {
  background-color: #f1f5f9;
  color: #64748b;
}

.repo-banner-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  font-size: 12px;
  color: #64748b;
}

.repo-banner-meta .sep {
  color: #cbd5e1;
}

.repo-banner-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.talent-filter-toolbar {
  margin-bottom: 18px;
  margin-right: 4px;
  box-sizing: border-box;
}

.count-summary {
  margin-left: auto;
  font-size: 13px;
}

.wb-candidate-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
  margin-right: 4px;
}

.talent-card {
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.candidate-sequence {
  font-variant-numeric: tabular-nums;
  font-size: 15px;
  border: 1px solid var(--wb-border);
  background: var(--wb-brand-soft);
}

.talent-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.candidate-headline {
  flex: 1;
  min-width: 0;
}

.stage-pill {
  background-color: #f3f4f6;
  font-weight: 500;
}

.candidate-owner-line {
  font-size: 12px;
  margin-top: 6px;
}

.empty-candidate-panel {
  padding: 56px 24px;
  text-align: center;
  margin-right: 4px;
  box-sizing: border-box;
}

.empty-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  color: #94a3b8;
}

.empty-detail-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.empty-illustration {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: #eff6ff;
  color: #3b82f6;
  font-size: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
}

.empty-title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.empty-subtitle {
  margin: 0 0 20px;
  font-size: 13px;
  color: #64748b;
  max-width: 480px;
  line-height: 1.5;
}

.empty-action-btn {
  border-radius: 20px;
  padding: 10px 24px;
  font-weight: 600;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.25);
}
</style>
