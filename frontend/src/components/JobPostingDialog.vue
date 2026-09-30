<template>
  <el-dialog v-model="opened" title="新增职位 · 四平台" width="760px" :close-on-click-modal="false">
    <p class="intro">招聘项目：<strong>{{ project?.name }}</strong>。前程无忧可保存职位草稿，其他平台的职位发布入口尚未接入。</p>
    <div class="platforms">
      <div class="platform active"><strong>前程无忧 51job</strong><span>可保存草稿</span></div>
      <div class="platform"><strong>BOSS 直聘</strong><span>待接入</span></div>
      <div class="platform"><strong>智联招聘</strong><span>待接入</span></div>
      <div class="platform"><strong>猎聘</strong><span>待接入</span></div>
    </div>
    <el-form label-position="top" class="posting-form">
      <el-form-item label="职位名称（必填）"><el-input v-model="form.title" maxlength="80" /></el-form-item>
      <el-form-item label="职位描述（必填）"><el-input v-model="form.description" type="textarea" :rows="5" maxlength="40000" show-word-limit /></el-form-item>
      <div class="form-grid">
        <el-form-item label="51job 职能路径（必填）"><el-input v-model="form.functionPath" placeholder="例如：互联网技术 > 测试 > 软件测试" /></el-form-item>
        <el-form-item label="关键词（必填，逗号分隔）"><el-input v-model="form.keywords" placeholder="例如：功能测试,Postman" /></el-form-item>
        <el-form-item label="最低月薪（元）"><el-input-number v-model="form.minSalary" :min="1" :max="99999999" :step="1000" style="width: 100%" /></el-form-item>
        <el-form-item label="最高月薪（元）"><el-input-number v-model="form.maxSalary" :min="1" :max="99999999" :step="1000" style="width: 100%" /></el-form-item>
        <el-form-item label="年薪发放月数"><el-select v-model="form.salaryMonths" placeholder="请选择" style="width: 100%"><el-option v-for="month in 13" :key="month" :label="`${month + 11}薪`" :value="month + 11" /></el-select></el-form-item>
        <el-form-item label="招聘人数"><el-input-number v-model="form.headcount" :min="1" :max="9999" style="width: 100%" /></el-form-item>
        <el-form-item label="最低工作经验（年）"><el-input-number v-model="form.experienceYears" :min="0" :max="10" style="width: 100%" /></el-form-item>
        <el-form-item label="最低学历"><el-select v-model="form.education" style="width: 100%"><el-option v-for="item in educationOptions" :key="item" :label="item" :value="item" /></el-select></el-form-item>
      </div>
      <p class="location-note">工作地址和发布城市沿用已登录 51job 企业账号当前选定的地址与城市。请先在 51job 页面核对；软件不会猜测或改写这两项。</p>
      <el-checkbox v-model="locationConfirmed">我已在 51job 页面核对工作地址、发布城市及公司账号</el-checkbox>
    </el-form>
    <el-alert v-if="result" :type="result.status === 'saved' ? 'success' : 'warning'" :closable="false" :title="result.message" show-icon />
    <template #footer>
      <el-button @click="opened = false">关闭</el-button>
      <el-button @click="openLogin">查看 51job 页面</el-button>
      <el-button type="primary" :loading="saving" :disabled="!locationConfirmed || result?.status === 'saved' || result?.status === 'uncertain'" @click="saveDraft">一键保存到 51job 草稿</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { Project } from '../composables/useProjectStore'

const props = defineProps<{ modelValue: boolean, project: Project | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const opened = ref(false)
const saving = ref(false)
const result = ref<{ status: string, message: string, url?: string } | null>(null)
const locationConfirmed = ref(false)
const educationOptions = ['本科', '大专', '硕士', '博士', '中技/中专', '高中', '初中及以下', '无学历要求']
const form = reactive({ title: '', description: '', functionPath: '', keywords: '', minSalary: 0, maxSalary: 0, salaryMonths: 0, headcount: 1, experienceYears: 0, education: '本科' })

watch(() => props.modelValue, value => { opened.value = value })
watch(opened, value => emit('update:modelValue', value))
watch(() => props.project, project => {
  if (!project) return
  form.title = project.job_config?.title || ''
  form.description = project.job_config?.job_description || (project.job_config?.requirements || []).join('\n')
  form.functionPath = ''
  form.keywords = (project.job_config?.required_skills || []).slice(0, 3).join(',')
  form.minSalary = 0
  form.maxSalary = 0
  form.salaryMonths = 0
  form.headcount = project.headcount || 1
  form.experienceYears = Math.min(10, Math.max(0, project.job_config?.experience_years || 0))
  form.education = educationOptions.includes(project.job_config?.education_level) ? project.job_config.education_level : '本科'
  result.value = null
  locationConfirmed.value = false
})

async function saveDraft() {
  if (!props.project) return
  const keywords = form.keywords.split(/[,，、]/).map(s => s.trim()).filter(Boolean)
  if (!form.title.trim() || form.description.trim().length < 50 || form.functionPath.split('>').filter(s => s.trim()).length < 2 || !keywords.length || !form.minSalary || !form.maxSalary || form.maxSalary < form.minSalary || !form.salaryMonths) {
    ElMessage.warning('请填写名称、至少 50 字的描述、职能路径、关键词、月薪范围及发放月数')
    return
  }
  saving.value = true
  result.value = null
  try {
    const app: any = await import('../../wailsjs/go/main/App')
    result.value = await app.Save51JobDraft({
      projectId: props.project.id,
      title: form.title.trim(), description: form.description.trim(), functionPath: form.functionPath.trim(),
      keywords, minSalary: form.minSalary, maxSalary: form.maxSalary, salaryMonths: form.salaryMonths,
      headcount: form.headcount, experienceYears: form.experienceYears, education: form.education
    })
    if (result.value?.status === 'saved') ElMessage.success(result.value.message)
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
.location-note { color: #92400e; background: #fffbeb; border-radius: 8px; padding: 10px; margin: 0 0 15px; }
@media (max-width: 620px) { .platforms, .form-grid { grid-template-columns: repeat(2, 1fr); } }
</style>
