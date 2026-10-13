<template>
  <div class="workbench-page">
    <header class="wb-header">
      <div><h1>简历人才库</h1><div class="wb-crumb">人才检索 / 按岗位查看候选人</div></div>
      <div class="wb-actions">
        <el-button @click="refresh" :loading="recruitment.loading">刷新</el-button>
        <el-button :loading="importing" @click="import51Job">一键导入 51job 投递</el-button>
        <el-button @click="showInbox">待分配投递<span v-if="pendingApplications.length">（{{ pendingApplications.length }}）</span></el-button>
        <el-button @click="showDuplicates">疑似重复<span v-if="duplicates.length">（{{ duplicates.length }}）</span></el-button>
        <el-button :disabled="!selectedProjectId" @click="showOutreach = true">向前十名打招呼</el-button>
        <el-button type="primary" :disabled="!selectedProjectId" @click="showSearch = true">矩阵寻才</el-button>
      </div>
    </header>
    <main class="wb-content">
      <el-alert v-if="recruitment.error" type="warning" :title="recruitment.error" show-icon :closable="false" style="margin-bottom:18px" />
      <div class="wb-toolbar">
        <el-select v-model="selectedProjectId" placeholder="全部岗位" clearable @change="syncRoute">
          <el-option v-for="project in projects.projects" :key="project.id" :label="`${project.job_config.title || project.name} · ${project.name}`" :value="project.id" />
        </el-select>
        <el-input v-model="keyword" clearable placeholder="搜索姓名、岗位或项目" style="width:230px" />
        <el-select v-model="statusFilter" placeholder="分析状态" clearable>
          <el-option label="已抓取／待分析" value="pending" /><el-option label="待补全简历" value="pending_document" /><el-option label="分析中" value="analyzing" />
          <el-option label="已完成" value="done" /><el-option label="失败" value="error" />
        </el-select>
        <el-select v-model="sourceFilter" placeholder="全部来源" clearable>
          <el-option v-for="source in sources" :key="source" :label="source" :value="source" />
        </el-select>
        <span class="wb-muted" style="margin-left:auto">{{ filtered.length }} 位人选</span>
      </div>
      <div v-if="filtered.length" class="wb-candidate-grid">
        <article v-for="item in filtered" :key="item.projectCandidateId" class="wb-card wb-candidate">
          <div class="wb-candidate-top">
            <span class="wb-avatar">{{ item.name.slice(0, 1) }}</span>
            <div><div class="wb-candidate-name">{{ item.name }}</div><div class="wb-candidate-role">{{ item.jobTitle || item.projectName }} · {{ item.projectName }}</div></div>
            <span v-if="item.analysisStatus === 'done'" class="wb-score">{{ item.abilityScore }}</span>
          </div>
          <div class="wb-candidate-meta">
            <span class="wb-pill">{{ item.source }}</span>
            <span class="wb-pill" :class="statusClass(item.analysisStatus)">{{ statusLabel(item.analysisStatus) }}</span>
            <span class="wb-pill" :class="redLineClass(item.redLineStatus)">红线：{{ redLineLabel(item.redLineStatus) }}</span>
            <span class="wb-pill">{{ stageLabels[item.stage] || item.stage }}</span>
          </div>
          <div class="wb-muted" style="font-size:12px">负责人：{{ item.owner || '未分配' }}<span v-if="item.nextActionAt"> · 下次跟进 {{ formatDate(item.nextActionAt) }}</span></div>
          <div class="wb-candidate-foot">
            <span class="wb-muted" style="font-size:12px">入库 {{ formatDate(item.createdAt) }}</span>
            <div class="wb-actions">
              <el-button size="small" @click="selected = item">查看档案</el-button>
              <el-button v-if="item.analysisStatus === 'error'" size="small" type="warning" plain @click="retryAnalysis(item.resumeId)">重试分析</el-button>
              <el-button size="small" type="primary" plain @click="router.push('/project/' + item.projectId)">项目评测</el-button>
            </div>
          </div>
        </article>
      </div>
      <div v-else class="wb-card wb-empty">{{ recruitment.loading ? '正在载入人才库…' : '当前筛选条件下暂无候选人' }}</div>
    </main>
    <el-drawer v-model="drawerOpen" title="候选人档案" size="480px">
      <template v-if="selected">
        <h2>{{ selected.name }}</h2>
        <p class="wb-muted">{{ selected.projectName }} · {{ selected.jobTitle }}</p>
        <div class="wb-candidate-meta" style="margin:18px 0">
          <span class="wb-pill">{{ selected.source }}</span>
          <span class="wb-pill">{{ stageLabels[selected.stage] || selected.stage }}</span>
          <span class="wb-pill" :class="redLineClass(selected.redLineStatus)">红线：{{ redLineLabel(selected.redLineStatus) }}</span>
        </div>
        <p>能力分：<strong>{{ selected.analysisStatus === 'done' ? selected.abilityScore : '待评估' }}</strong></p>
        <p>已确认加分项：{{ selected.confirmedBonus }}</p>
        <p>负责人：{{ selected.owner || '未分配' }}</p>
        <p v-if="selected.nextActionAt">下次跟进：{{ formatDate(selected.nextActionAt) }}</p>
        <div class="wb-actions" style="margin-top:22px">
          <el-button v-if="selected.profileUrl" @click="openProfile(selected.profileUrl)">平台主页</el-button>
          <el-button type="primary" @click="router.push('/project/' + selected!.projectId)">打开完整评测</el-button>
        </div>
        <div class="wb-card" style="margin-top:22px">
          <strong>核实联系方式</strong>
          <p class="wb-muted">仅在 HR 已核对该联系方式属于本人时填写；相同已核实联系方式会合并统一档案。</p>
          <div class="wb-actions"><el-select v-model="contactKind" style="width:100px"><el-option label="邮箱" value="email" /><el-option label="电话" value="phone" /></el-select><el-input v-model="verifiedContact" placeholder="已核实的联系方式" /><el-button @click="verifyContact">核实</el-button></div>
        </div>
        <section v-if="profile" style="margin-top:22px">
          <h3>关联岗位与独立评测</h3>
          <div v-for="link in profile.projects" :key="link.projectCandidateId" class="wb-card" style="margin-bottom:8px">
            <strong>{{ link.projectName }}</strong><span class="wb-muted"> · {{ link.jobTitle }}</span>
            <p class="wb-muted">能力 {{ link.analysisStatus === 'done' ? link.abilityScore : '待评估' }} · 红线 {{ redLineLabel(link.redLineStatus) }} · {{ stageLabels[link.stage] || link.stage }}</p>
          </div>
          <h3>来源与简历版本</h3>
          <p v-for="source in profile.sources" :key="source.platform + source.platformCandidateId" class="wb-muted">{{ source.platform === 'legacy' ? '本地档案' : source.platform }} · {{ source.platformCandidateId }}</p>
          <p v-for="resume in profile.resumes" :key="resume.id" class="wb-muted">{{ resume.fileName }} · {{ formatDate(resume.createdAt) }}</p>
          <h3>跨岗位沟通与跟进</h3>
          <p v-if="!profile.timeline.length" class="wb-muted">暂无记录</p>
          <div v-for="event in profile.timeline" :key="event.id" class="wb-card" style="margin-bottom:8px">
            <strong>{{ event.projectName }}</strong><span class="wb-muted"> · {{ event.actor || 'HR' }} · {{ formatDate(event.createdAt) }}</span>
            <p class="wb-muted">{{ event.note || event.type }}</p>
          </div>
        </section>
      </template>
    </el-drawer>
    <TopTenOutreachDialog v-model="showOutreach" :project-id="selectedProjectId" />
    <BossSearchDialog v-if="selectedProject" v-model="showSearch" :project-id="selectedProject.id"
      :project-name="selectedProject.name" :job-title="selectedProject.job_config.title"
      :exp-years="selectedProject.job_config.experience_years" :edu-level="selectedProject.job_config.education_level"
      @refresh="refresh" />
    <el-drawer v-model="inboxOpen" title="主动投递 · 待分配" size="500px">
      <div v-if="!pendingApplications.length" class="wb-empty">暂无待分配投递</div>
      <article v-for="application in pendingApplications" :key="application.id" class="wb-card" style="margin-bottom:12px">
        <strong>{{ application.candidateName }}</strong>
        <p class="wb-muted">51job 职位 ID：{{ application.platformJobId || '平台未返回' }} · 投递 ID：{{ application.applicationId }}</p>
        <div class="wb-actions"><el-select v-model="assignment[application.id]" placeholder="选择正确项目" style="width:230px"><el-option v-for="project in projects.projects" :key="project.id" :label="`${project.job_config.title || project.name} · ${project.name}`" :value="project.id" /></el-select><el-button type="primary" :disabled="!assignment[application.id]" @click="assign(application.id)">确认分配</el-button></div>
      </article>
    </el-drawer>
    <el-drawer v-model="duplicatesOpen" title="疑似重复 · HR 核对" size="500px">
      <div v-if="!duplicates.length" class="wb-empty">暂无疑似重复档案</div>
      <article v-for="pair in duplicates" :key="pair.candidateId + pair.otherCandidateId" class="wb-card" style="margin-bottom:12px">
        <strong>{{ pair.name }}</strong>
        <p class="wb-muted">{{ pair.reason }}</p>
        <p class="wb-muted">档案 A：{{ pair.candidateId }}<br />档案 B：{{ pair.otherCandidateId }}</p>
        <el-input v-model="mergeReasons[pair.candidateId + pair.otherCandidateId]" placeholder="请填写已核实的合并依据" />
        <el-button style="margin-top:8px" :disabled="!mergeReasons[pair.candidateId + pair.otherCandidateId]?.trim()" @click="merge(pair)">确认将 B 合并至 A</el-button>
      </article>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useProjectStore } from '../composables/useProjectStore'
import { useRecruitmentStore, stageLabels, type TalentCandidate } from '../composables/useRecruitmentStore'
import BossSearchDialog from '../components/BossSearchDialog.vue'
import TopTenOutreachDialog from '../components/TopTenOutreachDialog.vue'
import { ElMessage } from 'element-plus'

interface PendingApplication { id: string; applicationId: string; candidateName: string; platformJobId: string }
interface DuplicatePair { candidateId:string; otherCandidateId:string; name:string; reason:string }
interface CandidateProfile { projects:TalentCandidate[]; sources:Array<{platform:string;platformCandidateId:string}>; resumes:Array<{id:string;fileName:string;createdAt:string}>; timeline:Array<{id:string;projectName:string;type:string;note:string;actor:string;createdAt:string}> }

const route = useRoute()
const router = useRouter()
const projects = useProjectStore()
const recruitment = useRecruitmentStore()
const selectedProjectId = ref(String(route.query.project || ''))
const keyword = ref('')
const statusFilter = ref('')
const sourceFilter = ref('')
const showSearch = ref(false)
const showOutreach = ref(false)
const importing = ref(false)
const inboxOpen = ref(false)
const pendingApplications = ref<PendingApplication[]>([])
const assignment = ref<Record<string,string>>({})
const duplicates = ref<DuplicatePair[]>([])
const duplicatesOpen = ref(false)
const mergeReasons = ref<Record<string,string>>({})
const contactKind = ref('email')
const verifiedContact = ref('')
const selected = ref<TalentCandidate | null>(null)
const profile = ref<CandidateProfile | null>(null)
const drawerOpen = computed({ get: () => !!selected.value, set: value => { if (!value) selected.value = null } })
const selectedProject = computed(() => projects.projects.find(p => p.id === selectedProjectId.value))
const sources = computed(() => [...new Set(recruitment.candidates.map(c => c.source))].sort())
const filtered = computed(() => recruitment.candidates.filter(item =>
  (!selectedProjectId.value || item.projectId === selectedProjectId.value) &&
  (!statusFilter.value || item.analysisStatus === statusFilter.value) &&
  (!sourceFilter.value || item.source === sourceFilter.value) &&
  (!keyword.value || `${item.name} ${item.projectName} ${item.jobTitle}`.toLowerCase().includes(keyword.value.toLowerCase()))
))
watch(() => route.query.project, value => { selectedProjectId.value = String(value || '') })
watch(selected, async item => {
  profile.value = null
  if (!item) return
  try { const api:any = await import('../../wailsjs/go/main/App'); profile.value = await api.GetCandidateProfile(item.id) }
  catch (err:any) { ElMessage.warning(err?.message || String(err)) }
})
function syncRoute() { void router.replace({ path: '/talent', query: selectedProjectId.value ? { project: selectedProjectId.value } : {} }) }
function statusLabel(status: string) { return ({ pending: '已抓取／待分析', pending_document: '待补全简历', analyzing: '分析中', done: '已完成', error: '失败' } as Record<string,string>)[status] || status }
function statusClass(status: string) { return status === 'error' ? 'danger' : status === 'done' ? 'ok' : status === 'analyzing' ? 'warn' : '' }
function redLineLabel(status: string) { return ({ pending: '待核实', failed: '不满足', passed: '已满足', not_configured: '未配置' } as Record<string,string>)[status] || '待核实' }
function redLineClass(status: string) { return status === 'failed' ? 'danger' : status === 'passed' ? 'ok' : 'warn' }
function formatDate(value: string) { return value ? new Date(value).toLocaleString('zh-CN') : '' }
function openProfile(url: string) { window.open(url, '_blank', 'noopener,noreferrer') }
async function refresh() { await recruitment.loadCandidates() }
async function loadInbox() {
  try { const api: any = await import('../../wailsjs/go/main/App'); pendingApplications.value = await api.ListImportedApplications('unassigned') || [] }
  catch (err: any) { ElMessage.error(err?.message || String(err)) }
}
function showInbox() { inboxOpen.value = true; void loadInbox() }
async function import51Job() {
  importing.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const result = await api.ImportApplications('51job')
    ElMessage.success(`新增 ${result.addedCount}，重复 ${result.duplicateCount}，待分配 ${result.unassignedCount}，失败 ${result.failedCount}`)
    await Promise.all([refresh(), loadInbox()])
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { importing.value = false }
}
async function assign(id: string) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.AssignApplication(id, assignment.value[id])
    ElMessage.success('投递已分配')
    await Promise.all([refresh(), loadInbox()])
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
}
async function loadDuplicates() {
  try { const api:any = await import('../../wailsjs/go/main/App'); duplicates.value = await api.ListSuspectedDuplicates() || [] }
  catch (err:any) { ElMessage.error(err?.message || String(err)) }
}
function showDuplicates() { duplicatesOpen.value = true; void loadDuplicates() }
async function merge(pair: DuplicatePair) {
  try {
    const api:any = await import('../../wailsjs/go/main/App')
    await api.MergeCandidates(pair.candidateId,pair.otherCandidateId,'HR',mergeReasons.value[pair.candidateId + pair.otherCandidateId])
    ElMessage.success('档案已合并')
    await Promise.all([loadDuplicates(), refresh()])
  } catch (err:any) { ElMessage.error(err?.message || String(err)) }
}
async function verifyContact() {
  if (!selected.value || !verifiedContact.value.trim()) return
  try {
    const api:any = await import('../../wailsjs/go/main/App')
    await api.VerifyCandidateContact(selected.value.id,contactKind.value,verifiedContact.value.trim(),'HR')
    ElMessage.success('联系方式已核实，统一档案已更新')
    verifiedContact.value = ''
    selected.value = null
    await Promise.all([loadDuplicates(),refresh()])
  } catch (err:any) { ElMessage.error(err?.message || String(err)) }
}
async function retryAnalysis(resumeId:string) {
  try { const api:any=await import('../../wailsjs/go/main/App');await api.RetryCandidateAnalysis(resumeId);ElMessage.success('已重新加入分析队列');await refresh() }
  catch(err:any){ElMessage.error(err?.message||String(err))}
}
onMounted(() => { void projects.loadProjects(); void refresh(); void loadInbox(); void loadDuplicates() })
</script>
