<template>
  <el-dialog v-model="visible" :title="dialogTitle" width="min(940px, 94vw)" :close-on-click-modal="!sendingBatch" :close-on-press-escape="!sendingBatch" :show-close="!sendingBatch">
    <el-alert title="核对名单和文案后，确认本批沟通职位与点数预算。软件在预算内逐人实时核点发送；超预算、职位变化或回执不明时暂停。未初筛或红线待核实者须主动勾选。" type="info" :closable="false" style="margin-bottom:16px" />
    <div v-if="loading" class="wb-empty">正在生成沟通名单快照与招呼文案…</div>
    <div v-else-if="error" class="wb-empty">{{ error }}</div>
    <div v-else-if="batch">
      <div v-if="batch.status === 'confirmed'" class="outreach-run-state">
        <span>{{ sendingBatch ? '正在逐人处理发送队列…' : '本批已确认；逐人结果如下' }}</span>
        <el-button v-if="!sendingBatch && batch.items.some(item => item.status === 'queued' || item.status === 'awaiting_confirmation')" size="small" type="primary" @click="openBudget">确认预算并继续</el-button>
        <el-button v-if="sendingBatch" size="small" :disabled="stopRequested" @click="stopRequested=true">{{ stopRequested ? '当前人完成后暂停' : '暂停后续发送' }}</el-button>
        <el-button v-if="!sendingBatch" size="small" @click="preview(true)">重新生成预览</el-button>
      </div>
      <div v-if="batch.authorization" class="budget-summary" role="status" aria-live="polite">
        <strong>{{ batch.authorization.jobTitle }}</strong>
        <span>默认消耗 Hi 点发起沟通</span>
        <span>已发送/处理 {{ batch.authorization.reservedPoints }} 人</span>
        <p v-if="batch.authorization.message">{{ batch.authorization.message }}</p>
      </div>
      <div v-if="batch.status === 'draft'" class="outreach-toolbar">
        <div class="toolbar-left">
          <el-checkbox
            :model-value="allSelected"
            :indeterminate="isIndeterminate"
            @change="handleSelectAll"
          >
            一键全选 / 纳入本批
          </el-checkbox>
          <span class="selected-badge">已选 {{ selectedCount }} / {{ batch.items.length }} 人</span>
          <el-radio-group v-model="templateMode" size="small" @change="() => preview(true)" style="margin-left: 8px">
            <el-radio-button label="template">⚡ 极速企业模板 (0 AI消耗)</el-radio-button>
            <el-radio-button label="ai">✨ AI 深度定制</el-radio-button>
          </el-radio-group>
        </div>
        <div class="toolbar-right">
          <el-button
            size="small"
            type="primary"
            plain
            :loading="regeneratingAll"
            @click="regenerateAll"
          >
            ✨ 全员 AI 润色
          </el-button>
          <el-button
            size="small"
            type="primary"
            :disabled="selectedCount === 0"
            :loading="confirming || sendingBatch"
            @click="openBudget"
          >
            🚀 开始打招呼
          </el-button>
        </div>
      </div>

      <article v-for="item in batch.items" :key="item.id" class="outreach-row">
        <div class="outreach-head">
          <strong>#{{ item.rank }} {{ item.candidateName }}</strong>
          <span class="wb-pill" v-if="item.abilityScore > 0">能力 {{ item.abilityScore }}</span>
          <span class="wb-pill wb-muted" v-else>待初筛</span>
          <span class="wb-pill" :class="item.redLineStatus === 'pending' ? 'warn' : ''">红线：{{ item.redLineStatus === 'pending' ? '待核实' : '已核实／未配置' }}</span>
          <span class="wb-muted">{{ item.platform || '平台未确认' }} · {{ item.platformCandidateId || '缺少稳定 ID' }}</span>
          <el-button v-if="item.profileUrl" size="small" link @click="openProfile(item.profileUrl)">打开候选人网页</el-button>
          <div class="head-actions" v-if="batch.status === 'draft'">
            <el-button
              size="small"
              type="primary"
              link
              :loading="regenerating[item.id]"
              @click="regenerateSingle(item)"
            >
              ✨ AI 重新生成
            </el-button>
          </div>
        </div>
        <p class="wb-muted evidence-line" v-if="item.evidence">
          <span class="evidence-tag">履历与项目依据</span>
          <span>{{ item.evidence }}</span>
        </p>
        <p class="wb-muted" v-else>无可核实的个人经历片段，已使用通用岗位招呼语。</p>
        <template v-if="batch.status === 'draft'">
          <div class="wb-actions" style="margin-bottom:8px">
            <el-checkbox v-model="selected[item.id]">纳入本批</el-checkbox>
            <el-checkbox v-if="item.redLineStatus === 'pending'" v-model="pendingConfirmed[item.id]">我已确认纳入红线待核实人选</el-checkbox>
          </div>
          <el-input v-model="messages[item.id]" type="textarea" :rows="3" :maxlength="500" show-word-limit :disabled="!selected[item.id]" />
        </template>
        <template v-else>
          <p class="outreach-message">{{ item.message }}</p>
          <div class="wb-actions">
            <span class="wb-pill" :class="item.status === 'failed' ? 'danger' : item.status === 'sent' ? 'ok' : 'warn'">{{ statusLabel(item.status) }}</span>
            <span class="wb-muted">{{ item.result }}</span>
            <el-button v-if="['needs_manual','partial','uncertain','sending','failed'].includes(item.status)" size="small" @click="copyMessage(item.message)">复制文案</el-button>
            <el-button v-if="item.status === 'needs_manual' && item.platformCandidateId && /Waiting failed|timeout|timed out|超时|页面|职位/i.test(item.result)" size="small" :disabled="sendingBatch" @click="retryRead(item)">重新核点</el-button>
            <el-button v-if="item.status === 'needs_manual' && item.profileUrl?.startsWith('https://ehire.51job.com/') && !item.platformCandidateId" size="small" @click="verifyIdentity(item)">核验新版详情链接</el-button>
            <el-button v-if="['needs_manual','partial','uncertain','sending','failed'].includes(item.status)" size="small" :disabled="sendingBatch" @click="record(item)">登记平台结果</el-button>
          </div>
        </template>
      </article>
      <div v-if="batch.status === 'draft'" class="wb-actions" style="justify-content:space-between;align-items:center;margin-top:18px">
        <el-checkbox
          :model-value="allSelected"
          :indeterminate="isIndeterminate"
          @change="handleSelectAll"
        >
          全选本批候选人
        </el-checkbox>
        <el-button type="primary" :disabled="selectedCount === 0" :loading="confirming" @click="openBudget">
          确认名单并开始打招呼 (已选 {{ selectedCount }} 人)
        </el-button>
      </div>
    </div>
    <el-dialog v-model="budgetOpen" title="确认沟通职位并开始打招呼" width="min(560px, 92vw)" append-to-body :close-on-click-modal="false" :close-on-press-escape="!confirming" :show-close="!confirming">
      <p class="budget-hint">招聘岗位：{{ batch?.projectTitle }}。请确认本批沟通职位，系统将默认消耗 51job 平台 Hi 点数发起沟通（不设点数上限）。</p>
      <el-form label-position="top" @submit.prevent>
        <el-alert v-if="jobsError" :title="jobsError" type="warning" :closable="false" style="margin-bottom:12px">
          <div style="margin-top: 8px; display: flex; gap: 8px;">
            <el-button size="small" type="primary" plain @click="open51JobBrowser">打开 51job 浏览器</el-button>
            <el-button size="small" :loading="jobsLoading" @click="loadJobs">刷新职位列表</el-button>
          </div>
        </el-alert>
        <el-form-item label="沟通职位（来自当前企业账号）">
          <el-select v-model="selectedJobID" placeholder="请选择沟通职位" style="width:100%" :loading="jobsLoading">
            <el-option v-for="job in jobOptions" :key="job.id" :value="job.id" :label="`${job.title} · ${job.workArea || job.company || ''} · ID ${job.id}`" />
          </el-select>
        </el-form-item>
        <div class="hi-budget-tip">
          <el-icon><InfoFilled /></el-icon>
          <span>已取消点数上限限制，将默认消耗平台 Hi 点数逐人发送已定制的招呼文案。</span>
        </div>
      </el-form>
      <template #footer>
        <el-button :disabled="confirming" @click="budgetOpen=false">取消</el-button>
        <el-button type="primary" :loading="confirming" :disabled="!selectedJobID || jobsLoading" @click="confirm">
          立即开始打招呼
        </el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="receiptOpen" title="登记平台发送结果" width="460px" append-to-body>
      <el-select v-model="receiptStatus" style="width:100%;margin-bottom:12px">
        <el-option label="平台已核实发送成功" value="sent" />
        <el-option label="发送失败" value="failed" />
        <el-option label="结果不明，待核查" value="uncertain" />
      </el-select>
      <el-input v-model="receiptText" type="textarea" placeholder="平台会话 ID、发送时间或可核对的回执" />
      <el-input v-model="receiptActor" placeholder="登记人" style="margin-top:12px" />
      <template #footer>
        <el-button @click="receiptOpen = false">取消</el-button>
        <el-button type="primary" @click="saveReceipt">保存逐人结果</el-button>
      </template>
    </el-dialog>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

interface Item { id:string; rank:number; candidateName:string; platform:string; platformCandidateId:string; profileUrl:string; redLineStatus:string; abilityScore:number; evidence:string; message:string; status:string; result:string }
interface JobOption { id:string; title:string; company?:string; workArea?:string }
interface Batch { id:string; status:string; projectTitle:string; items:Item[]; authorization?:{jobId:string; jobTitle:string; perPersonLimit:number; totalLimit:number; reservedPoints:number; status:string; message:string} }
const props = withDefaults(defineProps<{
  modelValue: boolean
  projectId: string
  candidateIds?: string[]
}>(), {
  candidateIds: () => []
})
const emit = defineEmits<{ 'update:modelValue':[value:boolean]; updated:[] }>()
const visible = computed({ get:()=>props.modelValue, set:value=>emit('update:modelValue',value) })
const templateMode = ref<'template' | 'ai'>('template')
const batch=ref<Batch|null>(null)
const loading=ref(false)
const confirming=ref(false)
const sendingBatch=ref(false)
const stopRequested=ref(false)
const budgetOpen=ref(false)
const jobsLoading=ref(false)
const jobsError=ref('')
const jobOptions=ref<JobOption[]>([])
const jobsHasMore=ref(false)
const selectedJobID=ref('')
const regenerating=ref<Record<string, boolean>>({})
const regeneratingAll=ref(false)
const error=ref('')
const selected=ref<Record<string,boolean>>({})
const pendingConfirmed=ref<Record<string,boolean>>({})
const messages=ref<Record<string,string>>({})
const receiptOpen=ref(false)
const receiptItem=ref<Item|null>(null)
const receiptStatus=ref('sent')
const receiptText=ref('')
const receiptActor=ref('HR')

const dialogTitle = computed(() => {
  if (props.candidateIds && props.candidateIds.length > 0) {
    return `批量沟通 · 定制打招呼 (已选 ${props.candidateIds.length} 人)`
  }
  return '批量沟通 · 定制打招呼 (排名前十)'
})

const selectedCount = computed(() => {
  if (!batch.value) return 0
  return batch.value.items.filter(item => selected.value[item.id]).length
})
const allSelected = computed(() => {
  if (!batch.value || !batch.value.items.length) return false
  return batch.value.items.every(item => selected.value[item.id])
})
const isIndeterminate = computed(() => {
  if (!batch.value || !batch.value.items.length) return false
  const count = selectedCount.value
  return count > 0 && count < batch.value.items.length
})

function handleSelectAll(val: any) {
  if (!batch.value) return
  const isChecked = Boolean(val)
  for (const item of batch.value.items) {
    selected.value[item.id] = isChecked
    if (item.redLineStatus === 'pending') {
      pendingConfirmed.value[item.id] = isChecked
    }
  }
}

function statusLabel(status:string){return ({queued:'待发送',awaiting_confirmation:'已核点／待发送',sending:'发送中或中断待核查',partial:'部分完成／待核查',needs_manual:'需人工处理',excluded:'未纳入',sent:'已核实发送',failed:'失败',uncertain:'待核查'} as Record<string,string>)[status]||status}
async function preview(forceNew=false){
  if(!props.projectId)return
  loading.value=true;error.value='';batch.value=null
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    const ids = props.candidateIds && props.candidateIds.length > 0 ? props.candidateIds : []
    const storageKey=`talentlens-outreach-batch:${props.projectId}`
    const savedID=localStorage.getItem(storageKey)
    if(ids.length === 0 && savedID && !forceNew){
      try{
        const saved:Batch=await api.GetOutreachBatch(savedID)
        if(saved.status==='confirmed' && saved.items.some(item=>['queued','awaiting_confirmation','needs_manual','partial','uncertain','sending'].includes(item.status))){
          batch.value=saved
          return
        }
      }catch{}
      localStorage.removeItem(storageKey)
    }
    if (api.PreviewBatchOutreach) {
      batch.value = await api.PreviewBatchOutreach(props.projectId, ids, templateMode.value)
    } else {
      batch.value = await api.PreviewTopTenOutreach(props.projectId)
    }
    if(batch.value?.id && ids.length === 0)localStorage.setItem(storageKey,batch.value.id)
    for(const item of batch.value?.items||[]){
      selected.value[item.id]=item.redLineStatus!=='pending'
      pendingConfirmed.value[item.id]=false
      messages.value[item.id]=item.message
      regenerating.value[item.id]=false
    }
  }catch(err:any){error.value=err?.message||String(err)}finally{loading.value=false}
}
async function regenerateSingle(item: Item) {
  regenerating.value[item.id] = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const newMsg = await api.GenerateCandidateAIOutreachMessage(props.projectId, item.id)
    if (newMsg) {
      messages.value[item.id] = newMsg
      ElMessage.success(`已结合【${item.candidateName}】的真实项目与工作经历重新定制招呼语`)
    }
  } catch (err: any) {
    ElMessage.error(`AI 生成失败: ${err?.message || String(err)}`)
  } finally {
    regenerating.value[item.id] = false
  }
}
async function regenerateAll() {
  if (!batch.value) return
  regeneratingAll.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    let successCount = 0
    for (const item of batch.value.items) {
      if (!selected.value[item.id]) continue
      regenerating.value[item.id] = true
      try {
        const newMsg = await api.GenerateCandidateAIOutreachMessage(props.projectId, item.id)
        if (newMsg) {
          messages.value[item.id] = newMsg
          successCount++
        }
      } catch {}
      regenerating.value[item.id] = false
    }
    ElMessage.success(`已完成 ${successCount} 位候选人的 AI 深度定制招呼语`)
  } finally {
    regeneratingAll.value = false
  }
}
async function open51JobBrowser() {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    if (api.EnsurePlatformBrowser) {
      await api.EnsurePlatformBrowser('51job')
    } else {
      await api.OpenURL('https://ehire.51job.com')
    }
    ElMessage.success('已启动 51job 企业浏览器窗口，请在窗口中登录企业账号')
  } catch (err: any) {
    ElMessage.error(`启动浏览器失败: ${err?.message || err}`)
  }
}

async function openBudget(){
  if(!batch.value || sendingBatch.value)return
  if(batch.value.status==='draft' && !batch.value.items.some(item=>selected.value[item.id])){ElMessage.warning('请至少选择一位候选人');return}
  budgetOpen.value=true
  const auth=batch.value.authorization
  selectedJobID.value=auth?.jobId||''
  await loadJobs()
}
async function loadJobs(){
  if(!batch.value || jobsLoading.value)return
  jobsLoading.value=true; jobsError.value=''
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    if (api.EnsurePlatformBrowser) {
      await api.EnsurePlatformBrowser('51job')
    }
    const result=await api.ListOutreachCommunicationJobs(batch.value.id,'')
    if(result.status==='needs_login' || (result.status!=='jobs' && String(result.message).includes('未启动'))){
      if(api.EnsurePlatformBrowser) await api.EnsurePlatformBrowser('51job')
      jobsError.value='已为您自动打开 51job 企业浏览器，请在弹出的窗口中登录企业账号后点击【刷新职位列表】。'
      jobOptions.value=[]
      selectedJobID.value=''
      return
    }
    if(result.status!=='jobs')throw new Error(result.message||'无法读取平台职位')
    jobOptions.value=result.jobs||[]; jobsHasMore.value=!!result.hasMore
    if(!jobOptions.value.some(job=>job.id===selectedJobID.value))selectedJobID.value=result.suggestedJobId||''
    if(!jobOptions.value.length)jobsError.value='当前账号未找到在招职位，请确认 51job 后台是否有在招职位'
  }catch(err:any){jobsError.value=err?.message||String(err);jobOptions.value=[];selectedJobID.value=''}
  finally{jobsLoading.value=false}
}
async function confirm(){
  if(!batch.value || !selectedJobID.value)return
  const job=jobOptions.value.find(job=>job.id===selectedJobID.value)
  if(!job)return
  confirming.value=true
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    if(batch.value.status==='draft'){
      batch.value=await api.ConfirmOutreachBatch(batch.value.id,batch.value.items.map(item=>({itemId:item.id,message:messages.value[item.id],include:!!selected.value[item.id],confirmPending:!!pendingConfirmed.value[item.id]})))
    }
    // 默认消耗 Hi 点数，不设上限限制
    const pLimit = 100
    const tLimit = 100000
    batch.value=await api.AuthorizeOutreachBudget(batch.value!.id,job.id,job.title,pLimit,tLimit)
    budgetOpen.value=false
    ElMessage.info('沟通职位已确认，开始默认消耗 Hi 点数逐人发送打招呼')
    await runQueuedOutreach()
  }catch(err:any){ElMessage.error(err?.message||String(err))}finally{confirming.value=false}
}
async function runQueuedOutreach(){
  if(!batch.value || sendingBatch.value)return
  sendingBatch.value=true;stopRequested.value=false
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    const pending=batch.value.items.filter(item=>item.status==='queued'||item.status==='awaiting_confirmation')
    for(const item of pending){
      if(stopRequested.value)break
      try{
        const quote=await api.PrepareOutreachItem(item.id)
        batch.value=await api.GetOutreachBatch(batch.value!.id)
        if(quote.status!=='needs_confirmation'){
          ElMessage.warning(item.candidateName+'：'+(quote.message||'核点未完成，已暂停'))
          break
        }
        const outcome:Item=await api.ConfirmOutreachItem(item.id,quote.quoteId)
        batch.value=await api.GetOutreachBatch(batch.value!.id)
        if(outcome.status!=='sent'){
          ElMessage.warning(item.candidateName+'：'+outcome.result)
          break
        }
      }catch(err:any){
        ElMessage.error(item.candidateName+'：'+(err?.message||String(err))+'；队列已暂停')
        batch.value=await api.GetOutreachBatch(batch.value!.id)
        break
      }
    }
  }finally{sendingBatch.value=false;emit('updated')}
}
async function retryRead(item:Item){
  if(!batch.value)return
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    await api.RequeueOutreachReadFailure(item.id)
    batch.value=await api.GetOutreachBatch(batch.value.id)
    await openBudget()
  }catch(err:any){ElMessage.error(err?.message||String(err))}
}
async function verifyIdentity(item:Item){
  try{
    const response=await ElMessageBox.prompt(
      '请先打开此人的 51job 网页，复制地址栏中包含 resumeId 的新版详情链接。软件会再次打开该页并核对候选人姓名。',
      '核验候选人详情链接',
      { confirmButtonText:'核验并加入发送队列', cancelButtonText:'取消', inputPlaceholder:'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=…' }
    )
    const api:any=await import('../../wailsjs/go/main/App')
    await api.VerifyOutreachCandidateDetail(item.id,response.value.trim())
    batch.value=await api.GetOutreachBatch(batch.value!.id)
    ElMessage.success('已核对平台页面与候选人姓名；请点击“继续处理待发送”核对职位和点数')
  }catch(err:any){
    if(err==='cancel'||err==='close')return
    ElMessage.error(err?.message||String(err))
  }
}
function record(item:Item){receiptItem.value=item;receiptStatus.value='sent';receiptText.value='';receiptOpen.value=true}
async function copyMessage(message:string){try{await navigator.clipboard.writeText(message);ElMessage.success('文案已复制')}catch{ElMessage.warning('复制失败，请手动选择文字')}}
function openProfile(url:string){window.open(url,'_blank','noopener,noreferrer')}
async function saveReceipt(){
  if(!receiptItem.value||!batch.value)return
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    await api.RecordOutreachResult(receiptItem.value.id,receiptStatus.value,receiptText.value,receiptActor.value)
    batch.value=await api.GetOutreachBatch(batch.value.id)
    receiptOpen.value=false
  }catch(err:any){ElMessage.error(err?.message||String(err))}
}
watch(()=>props.modelValue,value=>{if(value)void preview()})
watch(()=>props.candidateIds,()=>{if(props.modelValue)void preview(true)},{deep:true})
</script>

<style scoped>
.outreach-row{padding:16px 0;border-bottom:1px solid var(--wb-border)}
.outreach-head{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.outreach-head strong{margin-right:4px}
.outreach-row p{font-size:12px;margin:9px 0}
.outreach-message{padding:10px;border-radius:8px;background:var(--wb-surface-soft);color:var(--wb-text);font-size:13px!important}
.outreach-toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:14px;padding:10px 14px;background:var(--wb-surface-soft);border-radius:8px;flex-wrap:wrap}
.toolbar-left{display:flex;align-items:center;gap:10px}
.toolbar-right{display:flex;align-items:center;gap:10px;margin-left:auto}
.selected-badge{font-size:12px;color:var(--wb-text-secondary);background:rgba(0,0,0,0.05);padding:2px 8px;border-radius:12px}
.outreach-run-state{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-bottom:12px;padding:10px 12px;border-radius:8px;background:var(--wb-brand-soft);color:var(--wb-text);font-size:13px;font-weight:600}
.head-actions{margin-left:auto}
.evidence-line{display:flex;align-items:center;gap:6px;margin:6px 0!important}
.evidence-tag{display:inline-block;padding:1px 6px;border-radius:4px;background:rgba(79, 70, 229, 0.1);color:#4f46e5;font-weight:600;font-size:11px}
.budget-summary{display:flex;flex-wrap:wrap;gap:8px 20px;padding:16px;margin-bottom:16px;border:1px solid var(--wb-border);border-radius:12px;background:var(--wb-surface-soft);font-size:13px;color:var(--wb-text)}
.budget-summary p{flex-basis:100%;margin:0}
.budget-hint{font-size:13px;line-height:1.6;color:var(--wb-text);margin:0 0 16px}
.hi-budget-tip{display:flex;align-items:center;gap:8px;margin-top:14px;padding:10px 14px;background:rgba(37, 99, 235, 0.08);border:1px solid rgba(37, 99, 235, 0.15);border-radius:8px;font-size:13px;color:#1d4ed8}
</style>
