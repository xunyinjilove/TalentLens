<template>
  <el-dialog v-model="visible" title="项目排名前十 · 定制招呼预览" width="min(920px, 94vw)" :close-on-click-modal="!sendingBatch" :close-on-press-escape="!sendingBatch" :show-close="!sendingBatch">
    <el-alert title="先逐人核对文案，再确认名单。51job 会逐人核对职位与点数；默认 Hi聊和定制文案分别核实回执。红线待核实者须主动勾选。" type="warning" :closable="false" style="margin-bottom:16px" />
    <div v-if="loading" class="wb-empty">正在生成排名快照与 AI 深度定制招呼语…</div>
    <div v-else-if="error" class="wb-empty">{{ error }}</div>
    <div v-else-if="batch">
      <div v-if="batch.status === 'confirmed'" class="outreach-run-state">
        <span>{{ sendingBatch ? '正在逐人处理发送队列…' : '本批已确认；逐人结果如下' }}</span>
        <el-button v-if="!sendingBatch && batch.items.some(item => item.status === 'queued' || item.status === 'awaiting_confirmation')" size="small" type="primary" @click="runQueuedOutreach">继续处理待发送</el-button>
        <el-button v-if="!sendingBatch" size="small" @click="preview(true)">重新生成排名预览</el-button>
      </div>
      <div v-if="batch.status === 'draft'" class="outreach-toolbar">
        <el-button
          size="small"
          type="primary"
          plain
          :loading="regeneratingAll"
          @click="regenerateAll"
        >
          ✨ 全员 AI 深度定制招呼语
        </el-button>
        <span class="wb-muted" style="font-size:12px;margin-left:8px">自动结合已接入大模型分析每位候选人的真实代表性项目与工作经历</span>
      </div>

      <article v-for="item in batch.items" :key="item.id" class="outreach-row">
        <div class="outreach-head">
          <strong>#{{ item.rank }} {{ item.candidateName }}</strong>
          <span class="wb-pill">能力 {{ item.abilityScore }}</span>
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
            <el-button v-if="['needs_manual','partial','uncertain'].includes(item.status)" size="small" @click="copyMessage(item.message)">复制文案</el-button>
            <el-button v-if="item.status === 'needs_manual' && item.profileUrl?.startsWith('https://ehire.51job.com/') && !item.platformCandidateId" size="small" @click="verifyIdentity(item)">核验新版详情链接</el-button>
            <el-button v-if="['needs_manual','partial','uncertain'].includes(item.status)" size="small" @click="record(item)">登记平台结果</el-button>
          </div>
        </template>
      </article>
      <div v-if="batch.status === 'draft'" class="wb-actions" style="justify-content:flex-end;margin-top:18px">
        <el-button type="primary" :loading="confirming" @click="confirm">一次确认本批名单与文案</el-button>
      </div>
    </div>
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
import { ElMessage, ElMessageBox } from 'element-plus'

interface Item { id:string; rank:number; candidateName:string; platform:string; platformCandidateId:string; profileUrl:string; redLineStatus:string; abilityScore:number; evidence:string; message:string; status:string; result:string }
interface Batch { id:string; status:string; items:Item[] }
const props = defineProps<{ modelValue:boolean; projectId:string }>()
const emit = defineEmits<{ 'update:modelValue':[value:boolean] }>()
const visible = computed({ get:()=>props.modelValue, set:value=>emit('update:modelValue',value) })
const batch=ref<Batch|null>(null)
const loading=ref(false)
const confirming=ref(false)
const sendingBatch=ref(false)
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
function statusLabel(status:string){return ({queued:'待发送',awaiting_confirmation:'待确认点数',partial:'部分完成／待核查',needs_manual:'需人工处理',excluded:'未纳入',sent:'已核实发送',failed:'失败',uncertain:'待核查'} as Record<string,string>)[status]||status}
async function preview(forceNew=false){
  if(!props.projectId)return
  loading.value=true;error.value='';batch.value=null
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    const storageKey=`talentlens-outreach-batch:${props.projectId}`
    const savedID=localStorage.getItem(storageKey)
    if(savedID && !forceNew){
      try{
        const saved:Batch=await api.GetOutreachBatch(savedID)
        if(saved.status==='confirmed' && saved.items.some(item=>['queued','awaiting_confirmation','needs_manual','partial','uncertain'].includes(item.status))){
          batch.value=saved
          return
        }
      }catch{}
      localStorage.removeItem(storageKey)
    }
    batch.value=await api.PreviewTopTenOutreach(props.projectId)
    if(batch.value?.id)localStorage.setItem(storageKey,batch.value.id)
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
async function confirm(){
  if(!batch.value)return
  confirming.value=true
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    batch.value=await api.ConfirmOutreachBatch(batch.value.id,batch.value.items.map(item=>({itemId:item.id,message:messages.value[item.id],include:!!selected.value[item.id],confirmPending:!!pendingConfirmed.value[item.id]})))
    ElMessage.info('名单与文案已确认，开始逐人核对平台职位与点数')
    await runQueuedOutreach()
  }catch(err:any){ElMessage.error(err?.message||String(err))}finally{confirming.value=false}
}
async function runQueuedOutreach(){
  if(!batch.value || sendingBatch.value)return
  sendingBatch.value=true
  try{
    const api:any=await import('../../wailsjs/go/main/App')
    const pending=batch.value.items.filter(item=>item.status==='queued'||item.status==='awaiting_confirmation')
    for(const item of pending){
      let quote:any
      try{
        quote=await api.PrepareOutreachItem(item.id)
        batch.value=await api.GetOutreachBatch(batch.value!.id)
      }catch(err:any){ElMessage.error(`${item.candidateName} 报价失败：${err?.message||String(err)}`);continue}
      if(quote.status==='needs_login'){
        ElMessage.warning('51job 企业浏览器需要先登录；待发送项目已保留，请登录后点击“继续处理待发送”')
        break
      }
      if(quote.status!=='needs_confirmation'){
        ElMessage.warning(`${item.candidateName}：${quote.message||'无法核对平台信息，已转人工处理'}`)
        continue
      }
      try{
        await ElMessageBox.confirm(
          `候选人：${item.candidateName}\n沟通职位：${quote.jobTitle}\n预计消耗：${quote.points} 个 Hi聊点数。确认后将先发平台默认 Hi聊，再发送此人的定制文案。`,
          '逐人核对 51job 职位与点数',
          { confirmButtonText:'确认并发送此人', cancelButtonText:'转人工处理', type:'warning', dangerouslyUseHTMLString:false }
        )
      }catch{
        await api.DeferOutreachItem(item.id,'HR 暂未确认平台职位或点数；此人未发送，转人工处理')
        batch.value=await api.GetOutreachBatch(batch.value!.id)
        continue
      }
      try{
        const outcome:Item=await api.ConfirmOutreachItem(item.id,quote.quoteId)
        if(outcome.status==='sent')ElMessage.success(`${item.candidateName}：定制文案已核实发送`)
        else ElMessage.warning(`${item.candidateName}：${outcome.result}`)
      }catch(err:any){ElMessage.error(`${item.candidateName}：${err?.message||String(err)}；请核对平台会话`)}
      batch.value=await api.GetOutreachBatch(batch.value!.id)
    }
  }finally{sendingBatch.value=false}
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
</script>

<style scoped>
.outreach-row{padding:16px 0;border-bottom:1px solid var(--wb-border)}
.outreach-head{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.outreach-head strong{margin-right:4px}
.outreach-row p{font-size:12px;margin:9px 0}
.outreach-message{padding:10px;border-radius:8px;background:var(--wb-surface-soft);color:var(--wb-text);font-size:13px!important}
.outreach-toolbar{display:flex;align-items:center;margin-bottom:14px;padding:8px 12px;background:var(--wb-surface-soft);border-radius:8px}
.outreach-run-state{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-bottom:12px;padding:10px 12px;border-radius:8px;background:var(--wb-brand-soft);color:var(--wb-text);font-size:13px;font-weight:600}
.head-actions{margin-left:auto}
.evidence-line{display:flex;align-items:center;gap:6px;margin:6px 0!important}
.evidence-tag{display:inline-block;padding:1px 6px;border-radius:4px;background:rgba(79, 70, 229, 0.1);color:#4f46e5;font-weight:600;font-size:11px}
</style>
