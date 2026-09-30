<template>
  <el-dialog v-model="opened" title="新增职位 · 四平台" width="760px" :close-on-click-modal="false" :close-on-press-escape="!saving" :show-close="!saving">
    <p class="intro">招聘项目：<strong>{{ project?.name }}</strong>。前程无忧可保存草稿或立即发布，其他平台的职位发布入口尚未接入。</p>
    <div class="platforms">
      <div class="platform active"><strong>前程无忧 51job</strong><span>草稿 / 立即发布</span></div>
      <div class="platform"><strong>BOSS 直聘</strong><span>待接入</span></div>
      <div class="platform"><strong>智联招聘</strong><span>待接入</span></div>
      <div class="platform"><strong>猎聘</strong><span>待接入</span></div>
    </div>
    <el-form label-position="top" class="posting-form">
      <el-form-item label="招聘类型"><el-radio-group v-model="form.jobType"><el-radio label="社会招聘">社会招聘</el-radio><el-radio label="校园招聘">校园招聘</el-radio></el-radio-group></el-form-item>
      <el-form-item label="职位名称（必填）"><el-input v-model="form.title" maxlength="80" /></el-form-item>
      <el-form-item label="职位描述（必填）"><el-input v-model="form.description" type="textarea" :rows="5" maxlength="40000" show-word-limit /></el-form-item>
      <div class="form-grid">
        <el-form-item label="51job 职能路径（必填，填写到末级）"><el-input v-model="form.functionPath" placeholder="例如：互联网技术 > 测试 > 软件测试" /></el-form-item>
        <el-form-item label="职能关键词">
          <el-button :loading="loadingKeywords" @click="loadKeywordSuggestions">按职能加载 51job 推荐词</el-button>
        </el-form-item>
        <el-form-item label="最低月薪（元）"><el-input-number v-model="form.minSalary" :min="1" :max="99999999" :step="1000" style="width: 100%" /></el-form-item>
        <el-form-item label="最高月薪（元）"><el-input-number v-model="form.maxSalary" :min="1" :max="99999999" :step="1000" style="width: 100%" /></el-form-item>
        <el-form-item label="年薪发放月数"><el-select v-model="form.salaryMonths" placeholder="请选择" style="width: 100%"><el-option v-for="month in 13" :key="month" :label="`${month + 11}薪`" :value="month + 11" /></el-select></el-form-item>
        <el-form-item label="招聘人数"><el-input-number v-model="form.headcount" :min="1" :max="9999" style="width: 100%" /></el-form-item>
        <el-form-item v-if="form.jobType === '社会招聘'" label="最低工作经验（年）"><el-input-number v-model="form.experienceYears" :min="0" :max="10" style="width: 100%" /></el-form-item>
        <el-form-item label="最低学历"><el-select v-model="form.education" style="width: 100%"><el-option v-for="item in educationOptions" :key="item" :label="item" :value="item" /></el-select></el-form-item>
      </div>
      <p v-if="form.jobType === '校园招聘'" class="location-note">校招按 51job 的应届生条件填写，可能同步到应届生求职网并消耗职位配额；请以平台页面显示的实时规则为准。</p>
      <div v-if="keywordGroups.length" class="keyword-panel">
        <p>按“{{ loadedFunctionPath }}”选择关键词（含自定义已选 {{ keywordCount }} / 10）</p>
        <div v-for="group in keywordGroups" :key="group.name" class="keyword-group">
          <strong>{{ group.name }}</strong>
          <div class="keyword-options"><button v-for="word in group.keywords" :key="word" type="button" class="keyword-option" :class="{ selected: selectedKeywords.includes(word) }" @click="toggleKeyword(word)">{{ word }}</button></div>
        </div>
      </div>
      <el-form-item label="补充自定义关键词（可选，逗号分隔）"><el-input v-model="form.keywords" placeholder="例如：接口测试,Postman；与上方勾选词合计不超过 10 个" /></el-form-item>
      <p v-if="project?.job_config?.required_skills?.length" class="skill-hint">项目技能参考：{{ project.job_config.required_skills.slice(0, 5).join('、') }}</p>
      <el-alert v-if="keywordMessage" :closable="false" :title="keywordMessage" type="info" />
      <p class="location-note">工作地址和发布城市沿用已登录 51job 企业账号当前选定的地址与城市。请先在 51job 页面核对；软件不会猜测或改写这两项。</p>
      <el-checkbox v-model="locationConfirmed">我已在 51job 页面核对工作地址、发布城市及公司账号</el-checkbox>
    </el-form>
    <el-alert v-if="result" :type="['saved', 'published'].includes(result.status) ? 'success' : 'warning'" :closable="false" :title="result.message" show-icon />
    <template #footer>
      <el-button :disabled="saving" @click="opened = false">关闭</el-button>
      <el-button @click="openLogin">查看 51job 页面</el-button>
      <el-button :loading="saving" :disabled="saving || !locationConfirmed || completed" @click="submit('draft')">保存到 51job 草稿</el-button>
      <el-button type="primary" :loading="saving" :disabled="saving || !locationConfirmed || completed" @click="submit('publish')">立即发布到 51job</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { Project } from '../composables/useProjectStore'

const props = defineProps<{ modelValue: boolean, project: Project | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const opened = ref(false)
const saving = ref(false)
const result = ref<{ status: string, message: string, url?: string } | null>(null)
const locationConfirmed = ref(false)
const loadingKeywords = ref(false)
const keywordMessage = ref('')
const keywordGroups = ref<Array<{ name: string, keywords: string[] }>>([])
const selectedKeywords = ref<string[]>([])
const loadedFunctionPath = ref('')
const completed = computed(() => ['saved', 'published', 'uncertain'].includes(result.value?.status || ''))
const currentKeywords = () => [...new Set([...selectedKeywords.value, ...form.keywords.split(/[,，、]/).map(s => s.trim()).filter(Boolean)])]
const keywordCount = computed(() => currentKeywords().length)
const educationOptions = ['本科', '大专', '硕士', '博士', '中技/中专', '高中', '初中及以下', '无学历要求']
const form = reactive({ jobType: '社会招聘', title: '', description: '', functionPath: '', keywords: '', minSalary: 0, maxSalary: 0, salaryMonths: 0, headcount: 1, experienceYears: 0, education: '本科' })

watch(() => props.modelValue, value => { opened.value = value })
watch(opened, value => emit('update:modelValue', value))
watch(() => props.project, project => {
  if (!project) return
  form.jobType = '社会招聘'
  form.title = project.job_config?.title || ''
  form.description = project.job_config?.job_description || (project.job_config?.requirements || []).join('\n')
  // 仅为已在 51job 页面核验过的岗位预填分类；其他岗位不猜测平台职能。
  form.functionPath = /软件测试/.test(form.title) ? '互联网技术 > 测试 > 软件测试' : ''
  form.keywords = ''
  form.minSalary = 0
  form.maxSalary = 0
  form.salaryMonths = 0
  form.headcount = project.headcount || 1
  form.experienceYears = Math.min(10, Math.max(0, project.job_config?.experience_years || 0))
  form.education = educationOptions.includes(project.job_config?.education_level) ? project.job_config.education_level : '本科'
  result.value = null
  locationConfirmed.value = false
  keywordGroups.value = []
  selectedKeywords.value = []
  keywordMessage.value = ''
})

watch(() => form.functionPath, () => {
  keywordGroups.value = []
  selectedKeywords.value = []
  loadedFunctionPath.value = ''
  keywordMessage.value = ''
})

function toggleKeyword(word: string) {
  if (selectedKeywords.value.includes(word)) {
    selectedKeywords.value = selectedKeywords.value.filter(item => item !== word)
  } else if (keywordCount.value < 10 || form.keywords.split(/[,，、]/).map(s => s.trim()).includes(word)) {
    selectedKeywords.value = [...selectedKeywords.value, word]
  } else {
    ElMessage.warning('51job 最多选择 10 个关键词')
  }
}

async function loadKeywordSuggestions() {
  const path = form.functionPath.trim()
  if (path.split('>').filter(Boolean).length < 2) { ElMessage.warning('请先填写到末级的 51job 职能路径'); return }
  loadingKeywords.value = true
  keywordMessage.value = ''
  try {
    const app: any = await import('../../wailsjs/go/main/App')
    const response = await app.Get51JobKeywordSuggestions(path)
    if (path !== form.functionPath.trim()) return
    if (response.status !== 'ready') { keywordMessage.value = response.message; return }
    keywordGroups.value = response.groups || []
    loadedFunctionPath.value = path
    keywordMessage.value = response.message
  } catch (error: any) { keywordMessage.value = `读取 51job 关键词失败：${error.message || error}` }
  finally { loadingKeywords.value = false }
}

async function submit(action: 'draft' | 'publish') {
  if (!props.project) return
  const keywords = currentKeywords()
  const errors: string[] = []
  if (!form.title.trim()) errors.push('请填写职位名称')
  if (form.description.trim().length < 50) errors.push(`职位描述还差 ${50 - form.description.trim().length} 字`)
  if (form.functionPath.split('>').filter(s => s.trim()).length < 2) errors.push('51job 职能路径需填写到末级，例如“互联网技术 > 测试 > 软件测试”')
  if (!keywords.length) errors.push('请填写至少一个关键词')
  if (keywords.length > 10) errors.push('51job 关键词最多 10 个')
  if (!form.minSalary || !form.maxSalary || form.maxSalary < form.minSalary) errors.push('请填写有效的月薪范围')
  if (!form.salaryMonths) errors.push('请选择年薪发放月数')
  if (errors.length) {
    ElMessage.warning(errors.join('；'))
    return
  }
  saving.value = true
  result.value = null
  try {
    const app: any = await import('../../wailsjs/go/main/App')
    const request = {
      projectId: props.project.id,
      jobType: form.jobType,
      title: form.title.trim(), description: form.description.trim(), functionPath: form.functionPath.trim(),
      keywords, minSalary: form.minSalary, maxSalary: form.maxSalary, salaryMonths: form.salaryMonths,
      headcount: form.headcount, experienceYears: form.jobType === '校园招聘' ? 0 : form.experienceYears, education: form.education
    }
    result.value = action === 'publish' ? await app.Publish51Job(request) : await app.Save51JobDraft(request)
    if (['saved', 'published'].includes(result.value?.status || '')) ElMessage.success(result.value!.message)
  } catch (error: any) {
    result.value = { status: 'error', message: `无法调用职位发布引擎：${error.message || error}` }
  } finally { saving.value = false }
}

async function openLogin() {
  try {
    const app: any = await import('../../wailsjs/go/main/App')
    if (result.value?.url && result.value.status !== 'needs_login') {
      await app.ActivatePlatformBrowser('51job')
    } else {
      await app.OpenURL('https://ehire.51job.com/Revision/job?mark=new')
    }
  } catch (error: any) { ElMessage.error(error.message || String(error)) }
}
</script>

<style scoped>
.intro { margin: 0 0 16px; color: #475569; }
.platforms { display: grid; grid-template-columns: repeat(4, 1fr); gap: 9px; margin-bottom: 18px; }
.platform { display: flex; flex-direction: column; gap: 4px; border: 1px solid #e2e8f0; border-radius: 9px; padding: 10px; color: #64748b; }
.platform.active { color: #155e75; border-color: #67e8f9; background: #ecfeff; }
.platform span { font-size: 12px; }
.posting-form { max-height: 55vh; overflow-y: auto; padding-right: 8px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 16px; }
.keyword-panel { border: 1px solid #bae6fd; background: #f0f9ff; border-radius: 8px; padding: 12px; margin: 0 0 14px; }
.keyword-panel p { color: #075985; font-weight: 600; margin: 0 0 10px; }
.keyword-group { margin-top: 10px; }
.keyword-group strong { display: block; color: #334155; font-size: 13px; margin-bottom: 6px; }
.keyword-options { display: flex; flex-wrap: wrap; gap: 6px; }
.keyword-option { border: 1px solid #cbd5e1; border-radius: 5px; background: white; color: #475569; cursor: pointer; padding: 4px 8px; }
.keyword-option.selected { background: #dbeafe; border-color: #3b82f6; color: #1d4ed8; }
.skill-hint { color: #64748b; font-size: 12px; margin: -8px 0 14px; }
.location-note { color: #92400e; background: #fffbeb; border-radius: 8px; padding: 10px; margin: 0 0 15px; }
@media (max-width: 620px) { .platforms, .form-grid { grid-template-columns: repeat(2, 1fr); } }
</style>
