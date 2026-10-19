<template>
  <div class="workbench-page">
    <header class="wb-header">
      <div><h1>候选人跟进</h1><div class="wb-crumb">招聘漏斗 / 阶段看板</div></div>
      <div class="wb-actions"><el-button @click="recruitment.loadCandidates()">刷新</el-button></div>
    </header>
    <main class="wb-content">
      <div class="wb-toolbar">
        <el-select v-model="projectId" clearable placeholder="全部岗位" style="width:300px" @change="syncRoute">
          <el-option v-for="project in projects.projects" :key="project.id" :label="`${project.job_config.title || project.name} · ${project.name}`" :value="project.id" />
        </el-select>
        <span class="wb-muted">点击候选人卡片，更新阶段与下一步</span>
      </div>
      <el-alert v-if="recruitment.error" :title="recruitment.error" type="warning" :closable="false" style="margin-bottom:16px" />
      <div class="wb-kanban">
        <section v-for="stage in stages" :key="stage" class="wb-kanban-col">
          <div class="wb-kanban-head"><span>{{ stageLabels[stage] }}</span><span class="wb-pill">{{ inStage(stage).length }}</span></div>
          <div v-for="item in inStage(stage)" :key="item.projectCandidateId" class="wb-kanban-card" role="button" tabindex="0" @click="edit(item)" @keydown.enter="edit(item)">
            <div style="display:flex;gap:9px;align-items:center"><span class="wb-avatar" style="width:31px;height:31px">{{ getCandidateSurname(item) }}</span><strong>{{ item.name }}</strong></div>
            <div class="wb-muted" style="font-size:12px;margin:8px 0">{{ item.jobTitle || item.projectName }}</div>
            <div style="display:flex;justify-content:space-between;align-items:center"><span class="wb-pill">{{ item.analysisStatus === 'done' ? `能力 ${item.abilityScore}` : '待评估' }}</span><span class="wb-muted" style="font-size:11px">{{ item.owner || '未分配' }}</span></div>
            <div v-if="item.nextActionAt" class="wb-muted" style="font-size:11px;margin-top:8px">下次：{{ formatDate(item.nextActionAt) }}</div>
          </div>
          <div v-if="!inStage(stage).length" class="wb-muted" style="font-size:12px;padding:8px 4px">暂无人选</div>
        </section>
      </div>
    </main>
    <el-dialog v-model="dialogOpen" title="更新候选人跟进" width="560px">
      <template v-if="editing">
        <p style="margin-top:0"><strong>{{ editing.name }}</strong> · {{ editing.projectName }}</p>
        <div class="wb-form-grid">
          <div><label>招聘阶段</label><select v-model="form.stage"><option v-for="stage in stages" :key="stage" :value="stage">{{ stageLabels[stage] }}</option></select></div>
          <div><label>负责人</label><input v-model="form.owner" placeholder="输入 HR 或负责人姓名" /></div>
          <div class="full"><label>下一步日期</label><input v-model="form.nextActionAt" type="datetime-local" /></div>
          <div class="full"><label>跟进备注</label><textarea v-model="form.note" rows="3" placeholder="记录本次沟通或阶段调整依据" /></div>
        </div>
        <h3 style="font-size:14px;margin-top:20px">记录沟通</h3>
        <div class="wb-actions"><el-select v-model="communicationChannel" style="width:130px"><el-option label="平台" value="platform" /><el-option label="电话" value="phone" /><el-option label="微信" value="wechat" /><el-option label="邮件" value="email" /><el-option label="其他" value="other" /></el-select><el-input v-model="communicationText" placeholder="填写实际沟通内容" /><el-button :disabled="!communicationText.trim()" @click="saveCommunication">记录</el-button></div>
        <h3 style="font-size:14px;margin-top:22px">跟进记录</h3>
        <div v-if="events.length" class="event-list">
          <div v-for="event in events" :key="event.id" class="event-item">
            <div><strong>{{ event.type.startsWith('communication:') ? '沟通记录' : event.type === 'note' ? '备注' : stageLabels[event.toStage] || event.toStage }}</strong><span class="wb-muted"> · {{ event.actor || 'HR' }} · {{ formatDate(event.createdAt) }}</span></div>
            <div v-if="event.note">{{ event.note }}</div>
          </div>
        </div>
        <div v-else class="wb-muted">暂无跟进记录</div>
      </template>
      <template #footer><el-button @click="dialogOpen = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存跟进</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { useProjectStore } from '../composables/useProjectStore'
import { useRecruitmentStore, stageLabels, type TalentCandidate } from '../composables/useRecruitmentStore'
import { getCandidateSurname } from '../utils/candidate'

const stages = ['new', 'pending_analysis', 'pending_contact', 'communicating', 'first_interview', 'second_interview', 'offer', 'hired', 'on_hold', 'rejected']
const route = useRoute()
const router = useRouter()
const projects = useProjectStore()
const recruitment = useRecruitmentStore()
const projectId = ref(String(route.query.project || ''))
const dialogOpen = ref(false)
const editing = ref<TalentCandidate | null>(null)
const saving = ref(false)
const events = ref<Array<{ id:string; type:string; toStage:string; note:string; actor:string; createdAt:string }>>([])
const form = reactive({ stage: 'new', owner: '', nextActionAt: '', note: '' })
const communicationChannel = ref('platform')
const communicationText = ref('')

watch(() => route.query.project, value => { projectId.value = String(value || '') })
function syncRoute() { void router.replace({ path: '/pipeline', query: projectId.value ? { project: projectId.value } : {} }) }
function inStage(stage: string) { return recruitment.candidates.filter(item => item.stage === stage && (!projectId.value || item.projectId === projectId.value)) }
function formatDate(value: string) { return value ? new Date(value).toLocaleString('zh-CN') : '' }
async function edit(item: TalentCandidate) {
  editing.value = item
  form.stage = item.stage
  form.owner = item.owner
  form.nextActionAt = item.nextActionAt ? localDateTime(item.nextActionAt) : ''
  form.note = ''
  events.value = []
  dialogOpen.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    events.value = await api.ListFollowUpEvents(item.projectCandidateId) || []
  } catch { /* Existing card remains editable if history cannot load. */ }
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
    dialogOpen.value = false
    ElMessage.success('跟进记录已保存')
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { saving.value = false }
}
async function saveCommunication() {
  if (!editing.value || !communicationText.value.trim()) return
  try {
    const api:any = await import('../../wailsjs/go/main/App')
    await api.AddCandidateCommunication(editing.value.projectCandidateId,communicationChannel.value,communicationText.value,'HR')
    events.value = await api.ListFollowUpEvents(editing.value.projectCandidateId) || []
    communicationText.value = ''
    ElMessage.success('沟通记录已保存')
  } catch (err:any) { ElMessage.error(err?.message || String(err)) }
}
onMounted(() => { void projects.loadProjects(); void recruitment.loadCandidates() })
</script>

<style scoped>
.event-list { display:flex; flex-direction:column; gap:8px; max-height:180px; overflow:auto; }
.event-item { border-left:2px solid var(--wb-brand); padding:3px 10px; font-size:12px; }
</style>
