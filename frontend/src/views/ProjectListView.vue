<template>
  <div class="project-page">
    <TitleBar title="TalentLens">
      <div class="titlebar-actions">
        <LanguageSwitcher />
        <button class="toolbar-btn" @click="router.push('/settings')" title="系统设置">
          <el-icon><Setting /></el-icon>
        </button>
      </div>
    </TitleBar>

    <main class="project-main">
      <!-- 顶部：岗位需求与快速选择模板看板 -->
      <section class="job-dashboard-section">
        <div class="section-top-bar">
          <div class="section-title-wrap">
            <h2 class="section-title">
              <el-icon class="title-icon"><Briefcase /></el-icon>
              {{ $t('project.jobPanelTitle') }}
            </h2>
            <span class="section-subtitle">{{ $t('project.jobPanelDesc') }}</span>
          </div>
          <div class="section-actions">
            <el-button size="small" :icon="showJobDetails ? ArrowUp : ArrowDown" @click="showJobDetails = !showJobDetails" round>
              {{ showJobDetails ? '收起配置详情' : '查看/编辑岗位要求' }}
            </el-button>
            <el-button size="small" type="primary" plain @click="router.push('/settings')" round>
              <el-icon><Setting /></el-icon>
              更多配置
            </el-button>
          </div>
        </div>

        <!-- 快速选择模板组件 -->
        <JobPresetPicker ref="presetPickerRef" :selected-id="selectedPresetId" @select="handlePresetSelect" />

        <!-- 当前选中岗位信息概览条 -->
        <div class="current-job-strip">
          <div class="strip-left">
            <div class="job-badge-title">
              <span class="tag-title">{{ currentJobConfig.title || '自定义岗位' }}</span>
              <el-tag size="small" type="info" effect="plain" class="exp-tag">
                {{ currentJobConfig.experienceYears }}{{ $t('common.year') }}经验 · {{ currentJobConfig.educationLevel || '学历不限' }}
              </el-tag>
            </div>
            <div class="skills-wrap">
              <span class="skill-label">核心技能:</span>
              <el-tag
                v-for="skill in (currentJobConfig.requiredSkills || []).slice(0, 8)"
                :key="skill"
                size="small"
                type="primary"
                effect="light"
                class="skill-tag"
              >
                {{ skill }}
              </el-tag>
              <span v-if="(currentJobConfig.requiredSkills || []).length > 8" class="more-skills">
                +{{ (currentJobConfig.requiredSkills || []).length - 8 }}项
              </span>
            </div>
          </div>

          <div class="strip-right">
            <el-button type="success" size="small" plain @click="saveCurrentAsNewTemplate" round>
              <el-icon><DocumentAdd /></el-icon>
              存为新模板
            </el-button>
            <el-button type="primary" size="small" @click="openCreateDialogWithCurrentJob" round>
              <el-icon><Plus /></el-icon>
              以当前岗位新建项目
            </el-button>
          </div>
        </div>

        <!-- 展开的岗位职责与要求详情抽屉/卡片 -->
        <el-collapse-transition>
          <div v-show="showJobDetails" class="job-details-expand">
            <div class="expand-grid">
              <div class="expand-col">
                <label class="expand-label">岗位名称</label>
                <el-input v-model="currentJobConfig.title" placeholder="岗位名称" @change="saveJobConfigToStorage" />
              </div>
              <div class="expand-col">
                <label class="expand-label">最低工作经验 (年)</label>
                <el-input-number v-model="currentJobConfig.experienceYears" :min="0" :max="30" @change="saveJobConfigToStorage" />
              </div>
              <div class="expand-col">
                <label class="expand-label">最低学历要求</label>
                <el-select v-model="currentJobConfig.educationLevel" @change="saveJobConfigToStorage" style="width: 100%">
                  <el-option label="不限" value="" />
                  <el-option label="大专" value="大专" />
                  <el-option label="本科" value="本科" />
                  <el-option label="硕士" value="硕士" />
                  <el-option label="博士" value="博士" />
                </el-select>
              </div>
            </div>

            <div class="expand-full" style="margin-top: 12px;">
              <label class="expand-label">核心必备技能点 (可回车添加)</label>
              <el-select
                v-model="currentJobConfig.requiredSkills"
                multiple
                filterable
                allow-create
                default-first-option
                placeholder="输入技能并回车添加"
                style="width: 100%"
                @change="saveJobConfigToStorage"
              >
                <el-option v-for="s in commonSkillsList" :key="s" :label="s" :value="s" />
              </el-select>
            </div>

            <div class="expand-full" style="margin-top: 12px;">
              <div class="label-row">
                <label class="expand-label">岗位技能与职责要求详情</label>
                <span class="label-tip">大模型分析简历与出题时将重点对照此处</span>
              </div>
              <el-input
                v-model="currentJobConfig.jobDescription"
                type="textarea"
                :rows="5"
                placeholder="详细输入岗位职责、任职资格与技能要求..."
                @change="saveJobConfigToStorage"
              />
            </div>
            <div class="expand-footer">
              <span class="save-hint">修改将自动同步保存并在新建项目中生效</span>
              <el-button size="small" type="primary" @click="saveJobConfigToStorage(true)">
                <el-icon><Check /></el-icon>
                保存当前岗位配置
              </el-button>
            </div>
          </div>
        </el-collapse-transition>
      </section>

      <!-- 下方：招聘项目列表区域 -->
      <section class="projects-section">
        <div class="top-bar">
          <div class="title-with-count">
            <h2 class="page-title">{{ $t('project.title') }}</h2>
            <span class="project-count-badge">{{ projectStore.projects.length }} 个项目</span>
          </div>
          <button class="create-btn" @click="openCreateDialogWithCurrentJob">
            <svg viewBox="0 0 16 16" fill="currentColor"><path d="M8 2a.5.5 0 0 1 .5.5v5h5a.5.5 0 0 1 0 1h-5v5a.5.5 0 0 1-1 0v-5h-5a.5.5 0 0 1 0-1h5v-5A.5.5 0 0 1 8 2z"/></svg>
            {{ $t('project.create') }}
          </button>
        </div>

        <!-- 项目卡片网格 -->
        <div class="project-grid">
          <div
            v-for="project in projectStore.projects"
            :key="project.id"
            class="project-card"
            @click="openProject(project.id)"
          >
            <div class="card-top">
              <h3 class="card-title" :title="project.name">{{ project.name }}</h3>
              <button class="card-menu" @click.stop="handleDelete(project.id)" title="删除项目">
                <svg viewBox="0 0 16 16" fill="currentColor"><path d="M5.5 5.5A.5.5 0 0 1 6 6v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm2.5 0a.5.5 0 0 1 .5.5v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm3 .5a.5.5 0 0 0-1 0v6a.5.5 0 0 0 1 0V6z"/><path fill-rule="evenodd" d="M14.5 3a1 1 0 0 1-1 1H13v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V4h-.5a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1H6a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1h3.5a1 1 0 0 1 1 1v1zM4.118 4L4 4.059V13a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V4.059L11.882 4H4.118z"/></svg>
              </button>
            </div>

            <!-- 部门与岗位标签 -->
            <div class="card-tags-row">
              <span v-if="project.department" class="dept-tag">
                🏢 {{ project.department }}
              </span>
              <span class="job-tag">
                💼 {{ project.job_config?.title || '未设置岗位' }}
              </span>
            </div>

            <!-- 招聘人数与负责人 -->
            <div class="card-info-row">
              <span class="info-cell">
                👥 招聘 <strong>{{ project.headcount || 1 }}</strong> 人
              </span>
              <span class="info-cell" v-if="project.recruiter">
                👤 HR: <strong>{{ project.recruiter }}</strong>
              </span>
            </div>

            <!-- 备注说明 -->
            <div v-if="project.remark" class="card-remark-row" :title="project.remark">
              <span class="remark-text">📝 {{ project.remark }}</span>
            </div>

            <!-- 统计指标与状态 -->
            <div class="card-stats">
              <span class="stat">📄 {{ project.resume_ids?.length || 0 }} 份简历</span>
              <span class="stat" v-if="projectStats[project.id]">已分析 {{ projectStats[project.id].analyzed }}/{{ projectStats[project.id].total }}</span>
              <span class="stat max-score" v-if="projectStats[project.id]?.maxScore">最高分 {{ projectStats[project.id].maxScore }}</span>
              <span class="stat-badge" :class="project.status">{{ statusText(project.status) }}</span>
            </div>

            <!-- 创建时间 -->
            <div class="card-time">📅 {{ formatDate(project.created_at) }}</div>
          </div>

          <!-- 空状态 -->
          <div v-if="projectStore.projects.length === 0 && !projectStore.loading" class="empty-state">
            <div class="empty-icon">
              <svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="6" y="10" width="36" height="28" rx="4"/><path d="M6 18h36"/><circle cx="14" cy="14" r="1.5" fill="currentColor"/><circle cx="20" cy="14" r="1.5" fill="currentColor"/></svg>
            </div>
            <p>{{ $t('project.empty') }}</p>
            <button class="create-btn" @click="openCreateDialogWithCurrentJob">
              {{ $t('project.createFirst') }}
            </button>
          </div>
        </div>
      </section>
    </main>

    <!-- 新建项目对话框（重构字段：主题、关联岗位、部门、人数、负责人、备注） -->
    <el-dialog v-model="showCreateDialog" :title="$t('project.createDialogTitle')" width="540px" :close-on-click-modal="false">
      <div class="create-form">
        <!-- 项目名称/招聘主题 -->
        <div class="form-item">
          <label class="required">{{ $t('project.projectName') }}</label>
          <el-input v-model="newProject.name" :placeholder="$t('project.projectNamePlaceholder')" />
        </div>

        <!-- 关联岗位绑定提示 -->
        <div class="form-item linked-job-box">
          <div class="linked-job-header">
            <span class="linked-title">{{ $t('project.linkedJob') }}</span>
            <el-tag size="small" type="success" effect="plain">{{ currentJobConfig.title || '自定义岗位' }}</el-tag>
          </div>
          <p class="linked-desc">
            {{ currentJobConfig.experienceYears }}{{ $t('common.year') }}经验 · {{ currentJobConfig.educationLevel || '学历不限' }} | 技能: {{ (currentJobConfig.requiredSkills || []).slice(0, 5).join(', ') }}
          </p>
          <span class="linked-tip">💡 该项目将自动继承当前岗位设置中的技能与职责要求</span>
        </div>

        <!-- 招聘部门 -->
        <div class="form-item">
          <label>{{ $t('project.department') }}</label>
          <el-select
            v-model="newProject.department"
            filterable
            allow-create
            default-first-option
            :placeholder="$t('project.departmentPlaceholder')"
            style="width: 100%"
          >
            <el-option v-for="dept in departmentOptions" :key="dept" :label="dept" :value="dept" />
          </el-select>
        </div>

        <!-- 计划招聘人数 与 招聘负责人 -->
        <div class="form-row">
          <div class="form-item flex-1">
            <label>{{ $t('project.headcount') }}</label>
            <el-input-number v-model="newProject.headcount" :min="1" :max="100" style="width: 100%" />
          </div>
          <div class="form-item flex-1">
            <label>{{ $t('project.recruiter') }}</label>
            <el-input v-model="newProject.recruiter" :placeholder="$t('project.recruiterPlaceholder')" />
          </div>
        </div>

        <!-- 项目说明/备注 -->
        <div class="form-item">
          <label>{{ $t('project.remark') }}</label>
          <el-input
            v-model="newProject.remark"
            type="textarea"
            :rows="3"
            :placeholder="$t('project.remarkPlaceholder')"
          />
        </div>
      </div>

      <template #footer>
        <el-button @click="showCreateDialog = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleCreate" :disabled="!newProject.name.trim()">
          <el-icon><Check /></el-icon>
          {{ $t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>

    <DevPanel />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Setting,
  Briefcase,
  Plus,
  ArrowDown,
  ArrowUp,
  DocumentAdd,
  Check
} from '@element-plus/icons-vue'
import TitleBar from '../components/TitleBar.vue'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import DevPanel from '../components/DevPanel.vue'
import JobPresetPicker from '../components/JobPresetPicker.vue'
import { useProjectStore } from '../composables/useProjectStore'
import { saveCustomJobPreset, type JobPreset } from '../data/jobPresets'

const router = useRouter()
const { t } = useI18n()
const projectStore = useProjectStore()

const showCreateDialog = ref(false)
const showJobDetails = ref(false)
const selectedPresetId = ref<string>('')
const presetPickerRef = ref<InstanceType<typeof JobPresetPicker> | null>(null)
const projectStats = ref<Record<string, any>>({})

// 当前主界面活跃的岗位配置
const currentJobConfig = reactive({
  title: '临床项目经理',
  requiredSkills: ['临床试验方案设计', '质量控制', 'SOP标准操作', '药监核查', '生物/医学背景'] as string[],
  experienceYears: 3,
  educationLevel: '本科',
  jobDescription: '岗位职责：\n1. 制定项目管理计划，按计划完成所负责的项目启动、开展及结束工作，确保通过药监部门核查；\n2. 根据项目研发资料、指导原则、行业标准等要求，设计临床试验方案；\n3. 对所负责的临床试验项目进行全面的质量控制与管理，确保所有试验严格按照临床试验方案、标准操作程序和相关法规进行；\n4. 根据临床试验相关规范要求，完成临床研究的各阶段报批文件包括但不限于临床报告；\n5. 作为公司对外代表，建立并保持与机构的良好关系。\n\n任职要求：\n1. 生物学、检验学、医学或药学等相关专业背景，本科及以上学历；\n2. 高度责任心和抗压能力，可适应出差；\n3. 优秀的沟通表达能力、发现和解决问题的能力。'
})

const departmentOptions = [
  '研发中心',
  '临床部',
  '国际营销中心',
  '国内销售部',
  '质量管理部',
  '应用技术部',
  '人力资源部',
  '医学检验所'
]

const commonSkillsList = [
  '化学发光', '免疫分析仪', '体外诊断(IVD)', '临床试验', 'SOP标准操作',
  '肿瘤标志物', '甲功/激素', '质量控制', 'GMP规范', '质谱平台', 'PCR上岗证',
  '检验士/师', '科室会/学术讲座', '海外销售', '客户开拓', '自身免疫', '试剂研发',
  'Go', 'Python', 'Java', 'JavaScript', 'TypeScript', 'React', 'Vue', 'MySQL', 'Redis'
]

// 新建项目表单（包含业务描述字段）
const newProject = reactive({
  name: '',
  department: '临床部',
  headcount: 1,
  recruiter: '杨孝荣',
  remark: ''
})

// 从 LocalStorage 加载当前岗位配置
function loadJobConfigFromStorage() {
  const saved = localStorage.getItem('goresume_settings')
  if (saved) {
    try {
      const s = JSON.parse(saved)
      if (s.job) {
        currentJobConfig.title = s.job.title || currentJobConfig.title
        currentJobConfig.requiredSkills = s.job.requiredSkills || currentJobConfig.requiredSkills
        currentJobConfig.experienceYears = s.job.experienceYears ?? currentJobConfig.experienceYears
        currentJobConfig.educationLevel = s.job.educationLevel || currentJobConfig.educationLevel
        currentJobConfig.jobDescription = s.job.jobDescription || currentJobConfig.jobDescription
      }
    } catch {}
  }
}

// 保存当前岗位配置到 LocalStorage
function saveJobConfigToStorage(showMessage = false) {
  try {
    const saved = localStorage.getItem('goresume_settings')
    let s: any = {}
    if (saved) {
      try { s = JSON.parse(saved) } catch {}
    }
    s.job = {
      title: currentJobConfig.title,
      requiredSkills: [...currentJobConfig.requiredSkills],
      experienceYears: currentJobConfig.experienceYears,
      educationLevel: currentJobConfig.educationLevel,
      jobDescription: currentJobConfig.jobDescription
    }
    localStorage.setItem('goresume_settings', JSON.stringify(s))
    if (showMessage) {
      ElMessage.success('岗位配置已保存并在新建项目中生效')
    }
  } catch (e) {
    if (showMessage) {
      ElMessage.error('保存失败')
    }
  }
}

// 选择预设模板
function handlePresetSelect(preset: JobPreset | null) {
  if (preset) {
    selectedPresetId.value = preset.id
    currentJobConfig.title = preset.name
    currentJobConfig.requiredSkills = [...preset.requiredSkills]
    currentJobConfig.experienceYears = preset.experienceYears
    currentJobConfig.educationLevel = preset.educationLevel
    currentJobConfig.jobDescription = preset.jobDescription || preset.description || ''
    saveJobConfigToStorage(false)
    ElMessage.success(`已切换为岗位：${preset.name}`)
  } else {
    selectedPresetId.value = 'custom'
  }
}

// 保存当前岗位为新模板
function saveCurrentAsNewTemplate() {
  if (!currentJobConfig.title.trim()) {
    ElMessage.warning('请输入岗位名称')
    return
  }
  const newPreset: JobPreset = {
    id: 'custom-' + Date.now(),
    name: currentJobConfig.title.trim(),
    nameEn: currentJobConfig.title.trim(),
    category: 'custom',
    requiredSkills: [...currentJobConfig.requiredSkills],
    experienceYears: currentJobConfig.experienceYears,
    educationLevel: currentJobConfig.educationLevel,
    jobDescription: currentJobConfig.jobDescription,
    description: currentJobConfig.jobDescription ? currentJobConfig.jobDescription.slice(0, 60) + '...' : '',
    isCustom: true
  }
  saveCustomJobPreset(newPreset)
  selectedPresetId.value = newPreset.id
  presetPickerRef.value?.refresh()
  saveJobConfigToStorage(false)
  ElMessage.success(`已成功保存「${newPreset.name}」到自定义模板！`)
}

function openCreateDialogWithCurrentJob() {
  newProject.name = `${currentJobConfig.title} - ${new Date().toLocaleDateString('zh-CN')}招聘`
  showCreateDialog.value = true
}

// 加载项目统计
async function loadStats() {
  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch { return }
  if (!WailsApp) return
  for (const p of projectStore.projects) {
    try {
      const stats = await WailsApp.GetProjectStats(p.id)
      projectStats.value[p.id] = stats
    } catch {}
  }
}

function statusText(status: string): string {
  const map: Record<string, string> = {
    draft: t('project.statusDraft'),
    analyzing: t('project.statusAnalyzing'),
    completed: t('project.statusCompleted')
  }
  return map[status] || status
}

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  try {
    return new Date(dateStr).toLocaleDateString('zh-CN')
  } catch { return dateStr }
}

function openProject(id: string) {
  projectStore.currentProjectId = id
  router.push(`/project/${id}`)
}

async function handleCreate() {
  if (!newProject.name.trim()) return
  const jobCfg = {
    title: currentJobConfig.title,
    requirements: [],
    required_skills: [...currentJobConfig.requiredSkills],
    experience_years: currentJobConfig.experienceYears,
    education_level: currentJobConfig.educationLevel,
    job_description: currentJobConfig.jobDescription
  }
  const result = await projectStore.createProject(
    newProject.name.trim(),
    newProject.department || '',
    newProject.headcount || 1,
    newProject.recruiter || '',
    newProject.remark || '',
    jobCfg
  )
  if (result) {
    showCreateDialog.value = false
    ElMessage.success(t('project.createSuccess'))
    newProject.name = ''
    newProject.remark = ''
  }
}

async function handleDelete(id: string) {
  try {
    await ElMessageBox.confirm(t('project.deleteConfirm'), t('common.warning'), { type: 'warning' })
    await projectStore.deleteProject(id)
    ElMessage.success(t('project.deleteSuccess'))
  } catch {}
}

onMounted(async () => {
  loadJobConfigFromStorage()
  await projectStore.loadProjects()
  if (projectStore.projects.length === 0) {
    await projectStore.migrateExisting()
  }
  await loadStats()
})
</script>

<style scoped lang="scss">
@import '../styles/macos-theme.scss';
$text-muted: #86868b;

.project-page {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: $bg-secondary;
  font-family: $font-family;
}

.project-main {
  flex: 1;
  overflow-y: auto;
  padding: 24px 36px 48px;
}

/* 顶部岗位看板样式 */
.job-dashboard-section {
  background: $bg-primary;
  border: 1px solid $separator;
  border-radius: $radius-lg;
  padding: 18px 22px;
  margin-bottom: 24px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.03);

  .section-top-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 14px;
  }

  .section-title-wrap {
    display: flex;
    align-items: baseline;
    gap: 10px;

    .section-title {
      margin: 0;
      font-size: 16px;
      font-weight: 700;
      color: $text-primary;
      display: flex;
      align-items: center;
      gap: 6px;

      .title-icon {
        color: $system-blue;
      }
    }

    .section-subtitle {
      font-size: 12px;
      color: $text-muted;
    }
  }

  .section-actions {
    display: flex;
    gap: 8px;
  }
}

.current-job-strip {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  background: #f4f7fb;
  border: 1px solid #e2e8f0;
  border-radius: $radius-md;
  margin-top: 10px;

  .strip-left {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 14px;
  }

  .job-badge-title {
    display: flex;
    align-items: center;
    gap: 8px;

    .tag-title {
      font-size: 14.5px;
      font-weight: 700;
      color: #1e3a5f;
    }

    .exp-tag {
      font-weight: 500;
    }
  }

  .skills-wrap {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 5px;

    .skill-label {
      font-size: 12px;
      color: $text-muted;
      margin-right: 2px;
    }

    .skill-tag {
      font-size: 11px;
      border-radius: 4px;
    }

    .more-skills {
      font-size: 11px;
      color: $text-muted;
    }
  }

  .strip-right {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
  }
}

.job-details-expand {
  margin-top: 14px;
  padding: 16px;
  background: #ffffff;
  border: 1px solid $separator;
  border-radius: $radius-md;

  .expand-grid {
    display: grid;
    grid-template-columns: 2fr 1fr 1fr;
    gap: 14px;
  }

  .expand-label {
    display: block;
    font-size: 12.5px;
    font-weight: 600;
    color: $text-secondary;
    margin-bottom: 6px;
  }

  .label-row {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 6px;

    .label-tip {
      font-size: 11px;
      color: $text-muted;
    }
  }

  .expand-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 12px;
    padding-top: 10px;
    border-top: 1px dashed $separator;

    .save-hint {
      font-size: 11.5px;
      color: $text-muted;
    }
  }
}

/* 项目列表区域样式 */
.projects-section {
  .top-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 18px;

    .title-with-count {
      display: flex;
      align-items: center;
      gap: 10px;

      .page-title {
        margin: 0;
        font-size: 20px;
        font-weight: 700;
        color: $text-primary;
        letter-spacing: -0.02em;
      }

      .project-count-badge {
        font-size: 12px;
        padding: 3px 8px;
        border-radius: 10px;
        background: rgba(0, 122, 255, 0.1);
        color: $system-blue;
        font-weight: 600;
      }
    }
  }
}

.create-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: none;
  border-radius: $radius-md;
  background: $system-blue;
  color: white;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  font-family: $font-family;
  transition: background $transition-fast;

  svg { width: 14px; height: 14px; }

  &:hover { background: $system-blue-hover; }
  &:active { transform: scale(0.97); }
}

.project-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(330px, 1fr));
  gap: 18px;
}

.project-card {
  background: $bg-primary;
  border: 1px solid $separator;
  border-radius: $radius-lg;
  padding: 18px 20px;
  cursor: pointer;
  transition: all $transition-normal;
  display: flex;
  flex-direction: column;
  gap: 10px;

  &:hover {
    border-color: $system-blue;
    box-shadow: 0 6px 20px rgba(0, 122, 255, 0.12);
    transform: translateY(-2px);
  }

  .card-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
  }

  .card-title {
    margin: 0;
    font-size: 16px;
    font-weight: 700;
    color: $text-primary;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 260px;
  }

  .card-menu {
    width: 28px;
    height: 28px;
    border: none;
    border-radius: $radius-sm;
    background: transparent;
    color: $text-muted;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all $transition-fast;

    svg { width: 14px; height: 14px; }

    &:hover {
      background: rgba(255, 59, 48, 0.1);
      color: $system-red;
    }
  }

  .card-tags-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;

    .dept-tag {
      font-size: 11.5px;
      font-weight: 600;
      color: #0284c7;
      background: #e0f2fe;
      padding: 2px 7px;
      border-radius: 4px;
    }

    .job-tag {
      font-size: 11.5px;
      font-weight: 500;
      color: #334155;
      background: #f1f5f9;
      padding: 2px 7px;
      border-radius: 4px;
    }
  }

  .card-info-row {
    display: flex;
    align-items: center;
    gap: 14px;
    font-size: 12px;
    color: $text-secondary;

    .info-cell strong {
      color: $text-primary;
    }
  }

  .card-remark-row {
    font-size: 11.5px;
    color: $text-muted;
    background: #f8fafc;
    padding: 4px 8px;
    border-radius: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .card-stats {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding-top: 6px;
    border-top: 1px solid $separator;

    .stat {
      font-size: 12px;
      color: $text-muted;

      &.max-score {
        color: #d97706;
        font-weight: 600;
      }
    }

    .stat-badge {
      font-size: 11px;
      padding: 2px 8px;
      border-radius: 10px;
      font-weight: 600;
      margin-left: auto;

      &.draft {
        background: rgba(142, 142, 147, 0.12);
        color: $text-secondary;
      }
      &.analyzing {
        background: rgba(0, 122, 255, 0.12);
        color: $system-blue;
      }
      &.completed {
        background: rgba(52, 199, 89, 0.12);
        color: $system-green;
      }
    }
  }

  .card-time {
    font-size: 11px;
    color: $text-muted;
  }
}

/* 对话框内样式 */
.create-form {
  display: flex;
  flex-direction: column;
  gap: 14px;

  .form-item {
    display: flex;
    flex-direction: column;
    gap: 6px;

    label {
      font-size: 13px;
      font-weight: 600;
      color: $text-secondary;

      &.required::after {
        content: ' *';
        color: $system-red;
      }
    }
  }

  .form-row {
    display: flex;
    gap: 12px;
  }

  .linked-job-box {
    background: #f0fdf4;
    border: 1px solid #bbf7d0;
    border-radius: $radius-md;
    padding: 12px;

    .linked-job-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 4px;

      .linked-title {
        font-size: 12.5px;
        font-weight: 700;
        color: #166534;
      }
    }

    .linked-desc {
      margin: 0 0 6px 0;
      font-size: 11.5px;
      color: #15803d;
    }

    .linked-tip {
      font-size: 11px;
      color: #65a30d;
    }
  }
}

.empty-state {
  grid-column: 1 / -1;
  text-align: center;
  padding: 60px 20px;

  .empty-icon {
    width: 64px;
    height: 64px;
    margin: 0 auto 16px;
    color: $text-muted;
    opacity: 0.5;

    svg { width: 100%; height: 100%; }
  }

  p {
    font-size: 14px;
    color: $text-muted;
    margin-bottom: 20px;
  }
}
</style>
