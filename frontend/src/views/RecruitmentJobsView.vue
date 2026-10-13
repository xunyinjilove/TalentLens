<template>
  <div class="workbench-page">
    <header class="wb-header"><div><h1>职位管理</h1><div class="wb-crumb">岗位配置 / 规则审批 / 招聘项目</div></div><div class="wb-actions"><el-button @click="load">刷新</el-button><el-button type="primary" @click="router.push('/jobs/manage')">新建或管理岗位</el-button></div></header>
    <main class="wb-content">
      <div class="wb-toolbar"><el-input v-model="keyword" placeholder="搜索岗位或项目" clearable style="width:260px" /><span class="wb-muted">{{ filtered.length }} 个招聘项目</span></div>
      <div v-if="filtered.length" class="wb-candidate-grid">
        <article v-for="project in filtered" :key="project.id" class="wb-card job-card">
          <div class="job-head"><span class="wb-avatar">职</span><div><h2>{{ project.job_config.title || project.name }}</h2><p class="wb-muted">{{ project.name }} · {{ project.department || '未设置部门' }}</p></div></div>
          <div class="wb-candidate-meta"><span class="wb-pill">计划 {{ project.headcount || 1 }} 人</span><span class="wb-pill">负责人 {{ project.recruiter || '未分配' }}</span><span class="wb-pill">{{ countFor(project.id) }} 位候选人</span></div>
          <div class="wb-muted" style="font-size:12px">51job 职位 ID：{{ mappedJobs(project.id) || '未关联' }}</div>
          <p class="wb-muted job-desc">{{ project.job_config.job_description || '尚未填写岗位职责，可进入项目补充。' }}</p>
          <div class="wb-candidate-foot"><span class="wb-muted">{{ project.job_config.experience_years || 0 }} 年经验 · {{ project.job_config.education_level || '学历不限' }}</span><div class="wb-actions"><el-button size="small" @click="mapProject = project">51job 职位映射</el-button><el-button size="small" @click="ruleProject = project">规则审批</el-button><el-button size="small" type="primary" plain @click="router.push('/project/' + project.id)">项目详情</el-button></div></div>
        </article>
      </div>
      <div v-else class="wb-card wb-empty">{{ projects.loading ? '正在加载项目…' : '暂无招聘项目，可先新建岗位项目。' }}</div>
    </main>
    <JobRulesDialog v-model="rulesOpen" :project="ruleProject" />
    <el-dialog v-model="mappingOpen" title="关联 51job 平台职位" width="460px">
      <p class="wb-muted">只按平台职位 ID 归档主动投递。请从 51job 企业端核对该 ID。</p>
      <el-input v-model="platformJobId" placeholder="51job 职位 ID" />
      <template #footer><el-button @click="mappingOpen = false">取消</el-button><el-button type="primary" @click="saveMapping">保存关联</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useProjectStore, type Project } from '../composables/useProjectStore'
import { useRecruitmentStore } from '../composables/useRecruitmentStore'
import JobRulesDialog from '../components/JobRulesDialog.vue'
import { ElMessage } from 'element-plus'
const router = useRouter()
const projects = useProjectStore()
const recruitment = useRecruitmentStore()
const keyword = ref('')
const ruleProject = ref<Project | null>(null)
const mapProject = ref<Project | null>(null)
const platformJobId = ref('')
const mappings = ref<Array<{ projectId:string; platform:string; platformJobId:string }>>([])
const mappingOpen = computed({ get: () => !!mapProject.value, set: value => { if (!value) mapProject.value = null } })
const rulesOpen = computed({ get: () => !!ruleProject.value, set: value => { if (!value) ruleProject.value = null } })
const filtered = computed(() => projects.projects.filter(project => `${project.name} ${project.job_config.title}`.toLowerCase().includes(keyword.value.toLowerCase())))
function countFor(id: string) { return recruitment.candidates.filter(item => item.projectId === id).length }
function mappedJobs(id:string){return mappings.value.filter(item=>item.projectId===id&&item.platform==='51job').map(item=>item.platformJobId).join('、')}
async function saveMapping() {
  if (!mapProject.value || !platformJobId.value.trim()) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.SetPlatformJobMapping('51job', '', platformJobId.value.trim(), mapProject.value.id)
    ElMessage.success('平台职位 ID 已关联到项目')
    await loadMappings()
    mappingOpen.value = false
    platformJobId.value = ''
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
}
async function load() { await Promise.all([projects.loadProjects(), recruitment.loadCandidates()]) }
async function loadMappings(){try{const api:any=await import('../../wailsjs/go/main/App');mappings.value=await api.ListPlatformJobMappings()||[]}catch{mappings.value=[]}}
onMounted(() => { void load(); void loadMappings() })
</script>

<style scoped>
.job-card { display:flex; flex-direction:column; gap:15px; min-height:220px; }
.job-head { display:flex; gap:12px; align-items:center; }
.job-head h2 { margin:0; font-size:17px; }
.job-head p { margin:3px 0 0; font-size:12px; }
.job-desc { overflow:hidden; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; font-size:13px; margin:0; }
</style>
