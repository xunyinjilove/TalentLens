<template>
  <div class="workbench-page">
    <header class="wb-header">
      <div><h1>面试安排</h1><div class="wb-crumb">日程 / 预约与改期</div></div>
      <div class="wb-actions"><el-button type="primary" @click="create">预约面试</el-button></div>
    </header>
    <main class="wb-content">
      <div class="wb-toolbar">
        <el-select v-model="projectId" clearable placeholder="全部岗位" style="width:300px" @change="reload">
          <el-option v-for="project in projects.projects" :key="project.id" :label="`${project.job_config.title || project.name} · ${project.name}`" :value="project.id" />
        </el-select>
        <el-button @click="reload">刷新日程</el-button>
      </div>
      <el-alert v-if="recruitment.error" :title="recruitment.error" type="warning" :closable="false" style="margin-bottom:16px" />
      <div v-if="visibleInterviews.length" class="wb-interview-list">
        <article v-for="item in visibleInterviews" :key="item.id" class="wb-card wb-interview">
          <div class="wb-interview-time">{{ formatDay(item.startsAt) }}<div>{{ formatTime(item.startsAt) }}</div></div>
          <div><strong>{{ item.candidateName }}</strong> · {{ item.projectName }}
            <div class="wb-muted" style="font-size:12px;margin-top:5px">面试官：{{ item.interviewer || '未指定' }} · {{ item.method || '未指定方式' }}<span v-if="item.location"> · {{ item.location }}</span></div>
          </div>
          <div class="wb-actions"><span class="wb-pill" :class="item.status === 'cancelled' ? 'danger' : 'ok'">{{ statusLabel(item.status) }}</span>
            <el-button size="small" @click="edit(item)">{{ item.status === 'cancelled' ? '查看' : '修改' }}</el-button>
          </div>
        </article>
      </div>
      <div v-else class="wb-card wb-empty">暂无面试安排。预约后会显示在这里，并同步到招聘概览。</div>
    </main>
    <el-dialog v-model="dialogOpen" :title="form.id ? '修改面试' : '预约面试'" width="600px">
      <div class="wb-form-grid">
        <div class="full"><label>候选人及项目</label><select v-model="form.projectCandidateId" :disabled="!!form.id"><option value="">请选择</option><option v-for="person in recruitment.candidates" :key="person.projectCandidateId" :value="person.projectCandidateId">{{ person.name }} · {{ person.projectName }}</option></select></div>
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
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useProjectStore } from '../composables/useProjectStore'
import { useRecruitmentStore, type RecruitmentInterview } from '../composables/useRecruitmentStore'

const projects = useProjectStore()
const recruitment = useRecruitmentStore()
const projectId = ref('')
const dialogOpen = ref(false)
const saving = ref(false)
const form = reactive<RecruitmentInterview>({ id:'', projectCandidateId:'', candidateName:'', projectName:'', startsAt:'', endsAt:'', interviewer:'', method:'', location:'', note:'', status:'scheduled' })
const visibleInterviews = computed(() => recruitment.interviews.filter(item => !projectId.value || recruitment.candidates.some(person => person.projectCandidateId === item.projectCandidateId && person.projectId === projectId.value)))
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
  Object.assign(form, { id:'', projectCandidateId:'', candidateName:'', projectName:'', startsAt:'', endsAt:'', interviewer:'', method:'', location:'', note:'', status:'scheduled' })
  dialogOpen.value = true
}
function edit(item: RecruitmentInterview) { Object.assign(form, item, { startsAt: localInput(item.startsAt), endsAt: localInput(item.endsAt) }); dialogOpen.value = true }
async function reload() { await Promise.all([recruitment.loadInterviews(), recruitment.loadCandidates()]) }
async function save() {
  if (!form.projectCandidateId || !form.startsAt || !form.endsAt) { ElMessage.warning('请选择候选人并填写时间'); return }
  saving.value = true
  try {
    await recruitment.saveInterview({ ...form, startsAt: toISO(form.startsAt), endsAt: toISO(form.endsAt) })
    dialogOpen.value = false
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
