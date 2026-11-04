<template>
  <div class="workbench-page">
    <header class="wb-header"><div><h1>Offer 管理</h1><div class="wb-crumb">候选人 / 软件内 Offer 卡</div></div></header>
    <main class="wb-content">
      <div class="wb-toolbar">
        <el-select v-model="projectId" clearable placeholder="全部岗位" style="width:300px" @change="load">
          <el-option v-for="project in projects.projects" :key="project.id" :label="project.job_config.title || project.name" :value="project.id" />
        </el-select>
        <el-button @click="load">刷新</el-button>
      </div>
      <el-alert title="Offer 卡仅保存在本机，不会向候选人发送邮件或平台消息。" type="info" :closable="false" style="margin-bottom:16px" />
      <div v-if="cards.length" class="offer-grid">
        <article v-for="card in cards" :key="card.projectCandidateId" class="wb-card offer-card">
          <div class="offer-head"><div><h2>{{ card.candidateName }}</h2><p>{{ card.jobTitle || card.projectName }} · {{ card.projectName }}</p></div><span class="wb-pill" :class="card.status === 'accepted' ? 'ok' : card.status === 'declined' || card.status === 'withdrawn' ? 'danger' : ''">{{ statusText(card.status) }}</span></div>
          <dl><div><dt>薪酬方案</dt><dd>{{ card.salaryPackage || '待填写' }}</dd></div><div><dt>工作地点</dt><dd>{{ card.workLocation || '待填写' }}</dd></div><div><dt>拟入职日</dt><dd>{{ card.plannedStartDate || '待填写' }}</dd></div><div><dt>有效期</dt><dd>{{ card.validUntil || '待填写' }}</dd></div></dl>
          <div class="offer-actions"><el-button @click="open(card)">编辑 / 预览</el-button><el-button v-if="card.status === 'draft'" type="primary" plain @click="changeStatus(card,'confirmed')">确认 Offer</el-button><template v-if="card.status === 'confirmed'"><el-button type="success" plain @click="changeStatus(card,'accepted')">已接受</el-button><el-button type="danger" plain @click="changeStatus(card,'declined')">已拒绝</el-button></template><el-button v-if="card.status === 'draft' || card.status === 'confirmed'" plain @click="changeStatus(card,'withdrawn')">撤回</el-button><el-button v-if="card.status === 'declined' || card.status === 'withdrawn'" plain @click="changeStatus(card,'draft')">重新起草</el-button><el-button v-if="card.status === 'accepted'" type="success" @click="markHired(card)">进入入职</el-button></div>
        </article>
      </div>
      <div v-else class="wb-card wb-empty">暂无 Offer 阶段人选。请先在候选人跟进中调整到 Offer 阶段。</div>
    </main>
    <el-dialog v-model="dialogOpen" :title="`Offer 卡 · ${form.candidateName}`" width="720px">
      <div class="wb-form-grid">
        <div><label>候选人</label><input :value="form.candidateName" disabled /></div><div><label>岗位</label><input :value="form.jobTitle || form.projectName" disabled /></div>
        <div><label>薪酬方案</label><input v-model="form.salaryPackage" placeholder="如：月薪、年终奖及补贴" /></div><div><label>工作地点</label><input v-model="form.workLocation" /></div>
        <div><label>拟入职日</label><input v-model="form.plannedStartDate" type="date" /></div><div><label>有效期</label><input v-model="form.validUntil" type="date" /></div>
        <div><label>负责人</label><input v-model="form.owner" /></div><div class="full"><label>备注</label><textarea v-model="form.note" rows="3" /></div>
      </div>
      <section class="offer-preview"><h3>预览</h3><p><strong>{{ form.candidateName }}</strong> · {{ form.jobTitle || form.projectName }}</p><p>薪酬：{{ form.salaryPackage || '待填写' }}　工作地点：{{ form.workLocation || '待填写' }}</p><p>拟入职日：{{ form.plannedStartDate || '待填写' }}　有效期：{{ form.validUntil || '待填写' }}</p><p v-if="form.note">备注：{{ form.note }}</p></section>
      <section><h3>变更记录</h3><p v-if="!history.length" class="wb-muted">暂无记录</p><ul v-else class="offer-history"><li v-for="entry in history" :key="entry.id">{{ new Date(entry.createdAt).toLocaleString('zh-CN') }} · {{ historyText(entry.action) }} · {{ entry.actor || 'HR' }}</li></ul></section>
      <template #footer><el-button @click="dialogOpen=false">关闭</el-button><el-button type="primary" :loading="saving" @click="save">保存 Offer 卡</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute } from 'vue-router'
import { useProjectStore } from '../composables/useProjectStore'
import { useRecruitmentStore } from '../composables/useRecruitmentStore'

type OfferCard = { projectCandidateId:string; projectId:string; candidateName:string; projectName:string; jobTitle:string; salaryPackage:string; workLocation:string; plannedStartDate:string; validUntil:string; note:string; owner:string; status:string; createdAt:string; updatedAt:string }
type History = { id:string; action:string; actor:string; createdAt:string }
const route = useRoute()
const projects = useProjectStore()
const recruitment = useRecruitmentStore()
const projectId = ref(String(route.query.project || ''))
const cards = ref<OfferCard[]>([])
const history = ref<History[]>([])
const dialogOpen = ref(false)
const saving = ref(false)
const form = reactive<OfferCard>({ projectCandidateId:'',projectId:'',candidateName:'',projectName:'',jobTitle:'',salaryPackage:'',workLocation:'',plannedStartDate:'',validUntil:'',note:'',owner:'',status:'draft',createdAt:'',updatedAt:'' })
const statusText = (status:string) => ({ draft:'草稿',confirmed:'已确认',accepted:'候选人已接受',declined:'候选人已拒绝',withdrawn:'已撤回' } as Record<string,string>)[status] || status
const historyText = (action:string) => action === 'saved' ? '保存修改' : `状态变更：${statusText(action.slice(7))}`
async function api():Promise<any> { return await import('../../wailsjs/go/main/App') }
async function load() { try { cards.value = await (await api()).ListOfferCards(projectId.value) || [] } catch (err:any) { ElMessage.error(err?.message || String(err)) } }
async function open(card:OfferCard) { Object.assign(form,card); dialogOpen.value=true; try { history.value = await (await api()).ListOfferHistory(card.projectCandidateId) || [] } catch { history.value=[] } }
async function save() { saving.value=true; try { await (await api()).SaveOfferCard({ ...form },'HR'); await load(); history.value=await (await api()).ListOfferHistory(form.projectCandidateId) || []; ElMessage.success('Offer 卡已保存') } catch(err:any) { ElMessage.error(err?.message || String(err)) } finally { saving.value=false } }
async function changeStatus(card:OfferCard,status:string) { try { await ElMessageBox.confirm(`确定将 ${card.candidateName} 的 Offer 标记为“${statusText(status)}”？`,'确认状态'); await (await api()).SetOfferStatus(card.projectCandidateId,status,'HR'); await load(); ElMessage.success('Offer 状态已更新') } catch(err:any) { if(err!=='cancel') ElMessage.error(err?.message || String(err)) } }
async function markHired(card:OfferCard) { try { await ElMessageBox.confirm(`确认 ${card.candidateName} 已进入入职阶段？`,'确认入职'); await recruitment.loadCandidates(); const person = recruitment.candidates.find(item=>item.projectCandidateId===card.projectCandidateId); if(!person) throw new Error('候选人档案未找到'); await recruitment.updateFollowUp(person,'hired',person.owner || 'HR',person.nextActionAt || '','Offer 已接受，进入入职'); await load(); ElMessage.success('已进入入职阶段') } catch(err:any) { if(err!=='cancel') ElMessage.error(err?.message || String(err)) } }
onMounted(async () => { await Promise.all([projects.loadProjects(),load()]); if(typeof route.query.candidate==='string') { const card=cards.value.find(item=>item.projectCandidateId===route.query.candidate); if(card) void open(card) } })
</script>

<style scoped>
.offer-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(340px,1fr)); gap:16px; }
.offer-card { min-width:0; }
.offer-head { display:flex; justify-content:space-between; align-items:start; gap:12px; }
.offer-head h2 { margin:0; font-size:18px; }
.offer-head p { margin:5px 0 14px; color:var(--wb-muted); font-size:13px; }
.offer-card dl { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:10px; margin:0 0 16px; }
.offer-card dt { color:var(--wb-muted); font-size:12px; }.offer-card dd { margin:3px 0 0; overflow-wrap:anywhere; }
.offer-actions { display:flex; flex-wrap:wrap; gap:8px; }.offer-actions .el-button { margin:0; }
.offer-preview { margin:20px 0; padding:16px; border:1px solid var(--wb-border); border-radius:12px; background:var(--wb-surface); }
.offer-preview h3 { margin:0 0 12px; }.offer-preview p { margin:7px 0; overflow-wrap:anywhere; }
.offer-history { max-height:150px; overflow:auto; padding-left:18px; color:var(--wb-muted); }
@media(max-width:600px) { .offer-grid { grid-template-columns:1fr; } }
</style>
