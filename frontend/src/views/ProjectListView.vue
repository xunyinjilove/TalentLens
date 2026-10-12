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
      <!-- 顶部：岗位配置快捷状态条（可折叠预设看板） -->
      <section class="job-dashboard-section" :class="{ 'is-expanded': isPresetExpanded }">
        <!-- 紧凑常驻条 (Ribbon) -->
        <div class="job-ribbon-bar">
          <div class="ribbon-left">
            <div class="job-badge">
              <el-icon class="job-icon"><Briefcase /></el-icon>
              <span class="job-title">{{ currentJobConfig.title || '自定义岗位' }}</span>
              <span class="job-meta">
                {{ currentJobConfig.experienceYears }}{{ $t('common.year') }}经验 · {{ currentJobConfig.educationLevel || '学历不限' }}
              </span>
            </div>

            <!-- 核心技能微标签 -->
            <div class="skills-summary">
              <span
                v-for="skill in (currentJobConfig.requiredSkills || []).slice(0, 4)"
                :key="skill"
                class="skill-chip"
              >
                {{ skill }}
              </span>
              <el-tooltip
                v-if="(currentJobConfig.requiredSkills || []).length > 4"
                :content="(currentJobConfig.requiredSkills || []).slice(4).join('、')"
                placement="top"
              >
                <span class="skill-more">+{{ (currentJobConfig.requiredSkills || []).length - 4 }}</span>
              </el-tooltip>
            </div>

            <!-- 红线与加分微徽章（若有） -->
            <div class="criteria-badges" v-if="(currentJobConfig.redLines || []).length || (currentJobConfig.bonusPoints || []).length">
              <span v-if="(currentJobConfig.redLines || []).length" class="badge-item danger" :title="(currentJobConfig.redLines || []).join('\n')">
                <span class="badge-dot"></span>
                {{ (currentJobConfig.redLines || []).length }}条红线
              </span>
              <span v-if="(currentJobConfig.bonusPoints || []).length" class="badge-item warning" :title="(currentJobConfig.bonusPoints || []).join('\n')">
                <span class="badge-dot"></span>
                {{ (currentJobConfig.bonusPoints || []).length }}项加分
              </span>
            </div>
          </div>

          <div class="ribbon-right">
            <el-button
              size="small"
              class="btn-toggle-drawer"
              :class="{ 'is-active': isPresetExpanded }"
              @click="isPresetExpanded = !isPresetExpanded"
              round
            >
              <el-icon><Operation /></el-icon>
              <span>{{ isPresetExpanded ? '收起模板库' : '切换模板与要求' }}</span>
              <el-icon class="arrow-indicator" :class="{ 'is-rotated': isPresetExpanded }"><ArrowDown /></el-icon>
            </el-button>

            <el-button
              size="small"
              class="btn-edit-config"
              @click="showJobDetails = true"
              round
            >
              <el-icon><Edit /></el-icon>
              <span>微调要求</span>
            </el-button>

            <el-button
              type="primary"
              size="small"
              class="btn-new-project-ribbon"
              @click="openCreateDialogWithCurrentJob"
              round
            >
              <el-icon><Plus /></el-icon>
              <span>以此岗位新建项目</span>
            </el-button>
          </div>
        </div>

        <!-- 折叠面板内容：模板选择器与快捷保存 -->
        <Transition name="expand-slide">
          <div v-show="isPresetExpanded" class="preset-drawer-content">
            <div class="drawer-header">
              <div class="drawer-title-wrap">
                <span class="drawer-title">岗位预设模板库</span>
                <span class="drawer-subtitle">选择行业预设模板可一键填入岗位要求与职责详情</span>
              </div>
              <div class="drawer-actions">
                <el-button type="success" size="small" plain @click="saveCurrentAsNewTemplate" round>
                  <el-icon><DocumentAdd /></el-icon>
                  存为新模板
                </el-button>
                <el-button size="small" type="primary" plain @click="router.push('/settings')" round>
                  <el-icon><Setting /></el-icon>
                  系统设置
                </el-button>
                <el-button size="small" text @click="isPresetExpanded = false">
                  收起
                </el-button>
              </div>
            </div>

            <!-- 快速选择模板组件 -->
            <JobPresetPicker ref="presetPickerRef" :selected-id="selectedPresetId" @select="handlePresetSelect" />
          </div>
        </Transition>

        <!-- 居中弹窗常驻排版；仅切换可见性，避免点击时重新计算整张表单。 -->
        <Teleport to="body">
          <div class="job-details-backdrop" :class="{ 'is-open': showJobDetails }" aria-hidden="true" @click="showJobDetails = false" />
          <div ref="jobDetailsPanel" class="job-details-expand" :class="{ 'is-open': showJobDetails }" :aria-hidden="!showJobDetails" :aria-modal="showJobDetails ? 'true' : undefined" :inert="!showJobDetails" role="dialog" aria-label="岗位要求" tabindex="-1" @keydown.esc.stop.prevent="showJobDetails = false">
            <div class="expand-panel-header">
              <strong>岗位要求</strong>
              <el-button text aria-label="关闭岗位要求" @click="showJobDetails = false">关闭</el-button>
            </div>
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

            <!-- 🚫 一票否决红线 -->
            <div class="expand-full" style="margin-top: 14px;">
              <label class="expand-label text-danger" style="margin-bottom: 8px; display: block;">
                🚫 岗位必备条件（HR 核实后确认准入）
              </label>
              <TagInput
                v-model="currentJobConfig.redLines"
                tag-type="danger"
                placeholder="输入与岗位职责直接相关、可核验的必备条件"
                quick-title="快捷填入:"
                :preset-list="commonRedLines"
                storage-key="talentlens_custom_redlines"
                add-prompt-title="新增一票否决红线快捷项"
                @change="saveJobConfigToStorage"
              />
              <p class="save-hint">建议仅保留 1–3 条，并与用人经理确认业务理由和等价经历；AI 只给核实线索，不自动淘汰。</p>
            </div>

            <!-- ⭐ 核心优先加分项 -->
            <div class="expand-full" style="margin-top: 14px;">
              <label class="expand-label text-warning" style="margin-bottom: 8px; display: block;">
                ⭐ 优先加分项（HR 确认后仅用于同档排序）
              </label>
              <TagInput
                v-model="currentJobConfig.bonusPoints"
                tag-type="warning"
                placeholder="输入有事实依据的额外项目经验、成果或认证"
                quick-title="快捷填入:"
                :preset-list="commonBonusPoints"
                storage-key="talentlens_custom_bonuses"
                add-prompt-title="新增优先加分项快捷项"
                @change="saveJobConfigToStorage"
              />
              <p class="save-hint">AI 线索标为待确认；未填写人工核实依据前，不计入已确认优势。</p>
            </div>

            <div class="expand-full" style="margin-top: 14px;">
              <label class="expand-label" style="margin-bottom: 8px; display: block;">核心能力评分卡（逐项 0–4 级，等权汇总）</label>
              <TagInput v-model="currentJobConfig.coreCriteria" placeholder="输入需要评估的岗位核心能力" @change="saveJobConfigToStorage" />
              <p class="save-hint">0 未体现、1 了解、2 参与、3 独立完成、4 主导并可复盘；为空时使用上方核心技能。</p>
            </div>

            <div class="expand-footer">
              <span class="save-hint">修改将自动同步保存并在新建项目中生效</span>
              <el-button size="small" type="primary" @click="saveJobConfigToStorage(true)">
                <el-icon><Check /></el-icon>
                保存当前岗位配置
              </el-button>
            </div>
          </div>
        </Teleport>
      </section>

      <!-- 下方：招聘项目列表区域 -->
      <section class="projects-section">
        <div class="top-bar">
          <div class="title-with-count">
            <h2 class="page-title">{{ $t('project.title') }}</h2>
            <span class="project-count-badge">{{ projectStore.projects.length }} 个项目</span>
          </div>
          <button class="create-btn" @click="openCreateDialogWithCurrentJob">
            <el-icon><Plus /></el-icon>
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
            <!-- 卡片头部：标题、状态胶囊、删除 -->
            <div class="card-header">
              <div class="card-title-group">
                <h3 class="card-title" :title="project.name">{{ project.name }}</h3>
                <span class="status-pill" :class="project.status">
                  <span class="status-dot"></span>
                  {{ statusText(project.status) }}
                </span>
              </div>
              <button class="card-delete-btn" @click.stop="handleDelete(project.id)" title="删除项目">
                <el-icon><Delete /></el-icon>
              </button>
            </div>

            <!-- 岗位与部门标签栏 -->
            <div class="card-meta-tags">
              <span class="meta-tag job-tag" :title="project.job_config?.title || '未设置岗位'">
                <el-icon><Briefcase /></el-icon>
                <span>{{ project.job_config?.title || '未设置岗位' }}</span>
              </span>
              <span v-if="project.department" class="meta-tag dept-tag" :title="project.department">
                <el-icon><OfficeBuilding /></el-icon>
                <span>{{ project.department }}</span>
              </span>
            </div>

            <!-- 核心招聘指标行：人数、HR负责人、时间 -->
            <div class="card-info-grid">
              <div class="info-item">
                <el-icon><User /></el-icon>
                <span class="info-label">需求:</span>
                <span class="info-val"><strong>{{ project.headcount || 1 }}</strong> 人</span>
              </div>
              <div class="info-item" v-if="project.recruiter">
                <el-icon><UserFilled /></el-icon>
                <span class="info-label">HR:</span>
                <span class="info-val">{{ project.recruiter }}</span>
              </div>
              <div class="info-item date-item">
                <el-icon><Calendar /></el-icon>
                <span class="info-val">{{ formatDate(project.created_at) }}</span>
              </div>
            </div>

            <!-- 备注说明（若有） -->
            <div v-if="project.remark" class="card-remark" :title="project.remark">
              <span class="remark-text">{{ project.remark }}</span>
            </div>

            <!-- 简历分析进度与最高分卡条 -->
            <div class="card-progress-bar-wrap">
              <div class="progress-labels">
                <span class="resume-count">
                  <el-icon><Document /></el-icon>
                  {{ project.resume_ids?.length || 0 }} 份简历
                </span>
                <span class="analyzed-ratio" v-if="projectStats[project.id]">
                  已分析 {{ projectStats[project.id].analyzed }}/{{ projectStats[project.id].total }}
                </span>
                <span class="max-score-badge" v-if="projectStats[project.id]?.maxScore">
                  <el-icon><Trophy /></el-icon> 最高 {{ projectStats[project.id].maxScore }}分
                </span>
              </div>
              <div class="progress-track" v-if="projectStats[project.id]?.total">
                <div
                  class="progress-fill"
                  :style="{
                    width: Math.min(100, Math.round(((projectStats[project.id].analyzed || 0) / (projectStats[project.id].total || 1)) * 100)) + '%'
                  }"
                ></div>
              </div>
            </div>

            <!-- 底部快捷动作：发布招聘与进入详情 -->
            <div class="card-footer-actions">
              <button class="post-job-btn" @click.stop="openJobPosting(project)">
                <el-icon><Promotion /></el-icon>
                <span>四平台发布</span>
              </button>
              <div class="card-enter-link">
                <span>进入项目</span>
                <el-icon><ArrowRight /></el-icon>
              </div>
            </div>
          </div>

          <!-- 空状态 -->
          <div v-if="projectStore.projects.length === 0 && !projectStore.loading" class="empty-state">
            <div class="empty-icon">
              <el-icon :size="48"><Document /></el-icon>
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

    <JobPostingDialog v-model="showPostingDialog" :project="postingProject" />
    <DevPanel />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, nextTick, watch } from 'vue'
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
  Check,
  OfficeBuilding,
  User,
  UserFilled,
  Calendar,
  Document,
  Delete,
  Trophy,
  Promotion,
  ArrowRight,
  Edit,
  Operation
} from '@element-plus/icons-vue'
import TitleBar from '../components/TitleBar.vue'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import DevPanel from '../components/DevPanel.vue'
import JobPresetPicker from '../components/JobPresetPicker.vue'
import TagInput from '../components/TagInput.vue'
import JobPostingDialog from '../components/JobPostingDialog.vue'
import { useProjectStore, type Project } from '../composables/useProjectStore'
import {
  saveCustomJobPreset,
  getMergedPreset,
  savePresetCustomization,
  type JobPreset
} from '../data/jobPresets'

const router = useRouter()
const { t } = useI18n()
const projectStore = useProjectStore()

const showCreateDialog = ref(false)
const showPostingDialog = ref(false)
const postingProject = ref<Project | null>(null)

function openJobPosting(project: Project) {
  postingProject.value = project
  showPostingDialog.value = true
}
const showJobDetails = ref(false)
const jobDetailsPanel = ref<HTMLElement | null>(null)
watch(showJobDetails, async open => {
  await nextTick()
  if (open) jobDetailsPanel.value?.focus()
  else document.getElementById('job-details-toggle')?.focus()
})
const selectedPresetId = ref<string>('')
const presetPickerRef = ref<InstanceType<typeof JobPresetPicker> | null>(null)
const projectStats = ref<Record<string, any>>({})

// 顶部岗位预设抽屉展开状态（默认收起以突出招聘项目，记忆用户偏好）
const isPresetExpanded = ref(localStorage.getItem('talentlens_preset_expanded') === 'true')
watch(isPresetExpanded, val => {
  localStorage.setItem('talentlens_preset_expanded', String(val))
})

// 当前主界面活跃的岗位配置
const currentJobConfig = reactive({
  title: '临床项目经理',
  requiredSkills: ['临床试验方案设计', '质量控制', 'SOP标准操作', '药监核查', '生物/医学背景'] as string[],
  experienceYears: 3,
  educationLevel: '本科',
  jobDescription: '岗位职责：\n1. 制定项目管理计划，按计划完成所负责的项目启动、开展及结束工作，确保通过药监部门核查；\n2. 根据项目研发资料、指导原则、行业标准等要求，设计临床试验方案；\n3. 对所负责的临床试验项目进行全面的质量控制与管理，确保所有试验严格按照临床试验方案、标准操作程序和相关法规进行；\n4. 根据临床试验相关规范要求，完成临床研究的各阶段报批文件包括但不限于临床报告；\n5. 作为公司对外代表，建立并保持与机构的良好关系。\n\n任职要求：\n1. 生物学、检验学、医学或药学等相关专业背景，本科及以上学历；\n2. 高度责任心和抗压能力，可适应出差；\n3. 优秀的沟通表达能力、发现和解决问题的能力。',
  redLines: ['具备临床试验项目落地经验，并能说明本人负责的关键环节'] as string[],
  bonusPoints: ['有可核验的多中心临床试验统筹成果', '具备与岗位相关的项目管理认证'] as string[],
  coreCriteria: [] as string[]
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

const commonRedLines = [
  '能说明岗位核心项目中本人负责的具体工作',
  '具备岗位要求的有效执业或上岗资格（如适用）',
  '能提供核心必备技能的实际应用案例'
]

const commonBonusPoints = [
  '有与岗位相关、可量化的项目改善成果',
  '具备跨团队协作与带教的具体案例',
  '持有与岗位直接相关的专业认证',
  '有可核验的开源贡献或技术成果'
]

function addQuickRedLine(line: string) {
  if (!currentJobConfig.redLines) currentJobConfig.redLines = []
  if (!currentJobConfig.redLines.includes(line)) {
    currentJobConfig.redLines.push(line)
    saveJobConfigToStorage(false)
    ElMessage.success(`已添加红线：${line}`)
  }
}

function addQuickBonus(bonus: string) {
  if (!currentJobConfig.bonusPoints) currentJobConfig.bonusPoints = []
  if (!currentJobConfig.bonusPoints.includes(bonus)) {
    currentJobConfig.bonusPoints.push(bonus)
    saveJobConfigToStorage(false)
    ElMessage.success(`已添加加分项：${bonus}`)
  }
}

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
        currentJobConfig.redLines = s.job.redLines || currentJobConfig.redLines || []
        currentJobConfig.bonusPoints = s.job.bonusPoints || currentJobConfig.bonusPoints || []
        currentJobConfig.coreCriteria = s.job.coreCriteria || []
      }
    } catch {}
  }
}

// 保存当前岗位配置到 LocalStorage 及对应模板定制库中
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
      jobDescription: currentJobConfig.jobDescription,
      redLines: [...(currentJobConfig.redLines || [])],
      bonusPoints: [...(currentJobConfig.bonusPoints || [])],
      coreCriteria: [...currentJobConfig.coreCriteria]
    }
    localStorage.setItem('goresume_settings', JSON.stringify(s))

    // 核心持久化：同时以当前选中的模板 ID 或岗位标题为 Key，持久化保存到模板定制库中，绝不丢失！
    const currentKey = selectedPresetId.value || currentJobConfig.title
    if (currentKey && currentKey !== 'custom') {
      savePresetCustomization(currentKey, {
        title: currentJobConfig.title,
        requiredSkills: [...currentJobConfig.requiredSkills],
        experienceYears: currentJobConfig.experienceYears,
        educationLevel: currentJobConfig.educationLevel,
        jobDescription: currentJobConfig.jobDescription,
        redLines: [...(currentJobConfig.redLines || [])],
        bonusPoints: [...(currentJobConfig.bonusPoints || [])],
        coreCriteria: [...currentJobConfig.coreCriteria]
      })
      if (currentJobConfig.title && currentJobConfig.title !== currentKey) {
        savePresetCustomization(currentJobConfig.title, {
          redLines: [...(currentJobConfig.redLines || [])],
          bonusPoints: [...(currentJobConfig.bonusPoints || [])]
        })
      }
    }

    if (showMessage) {
      ElMessage.success('岗位配置与红线机制已保存并在新建项目中生效')
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
    // 自动融入用户已为该模板保存的个性化配置（包含红线与加分项！）
    const merged = getMergedPreset(preset)
    currentJobConfig.title = merged.name
    currentJobConfig.requiredSkills = [...merged.requiredSkills]
    currentJobConfig.experienceYears = merged.experienceYears
    currentJobConfig.educationLevel = merged.educationLevel
    currentJobConfig.jobDescription = merged.jobDescription || merged.description || ''
    currentJobConfig.redLines = merged.redLines ? [...merged.redLines] : []
    currentJobConfig.bonusPoints = merged.bonusPoints ? [...merged.bonusPoints] : []
    currentJobConfig.coreCriteria = merged.coreCriteria ? [...merged.coreCriteria] : [...merged.requiredSkills]

    // 同步到当前的全局设置，但保留各模板已存储的定制数据
    saveJobConfigToStorage(false)
    ElMessage.success(`已切换为岗位：${merged.name}`)
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
    redLines: [...(currentJobConfig.redLines || [])],
    bonusPoints: [...(currentJobConfig.bonusPoints || [])],
    coreCriteria: [...currentJobConfig.coreCriteria],
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
    job_description: currentJobConfig.jobDescription,
    red_lines: [...(currentJobConfig.redLines || [])],
    bonus_points: [...(currentJobConfig.bonusPoints || [])],
    core_criteria: [...currentJobConfig.coreCriteria]
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

/* 顶部岗位快捷看板样式 */
.job-dashboard-section {
  background: $bg-primary;
  border: 1px solid $separator;
  border-radius: $radius-lg;
  padding: 10px 16px;
  margin-bottom: 24px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.03);
  transition: all 0.25s ease;

  &.is-expanded {
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06);
    border-color: #cbd5e1;
    padding-bottom: 16px;
  }

  .job-ribbon-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px;
    min-height: 40px;
  }

  .ribbon-left {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px;
  }

  .job-badge {
    display: flex;
    align-items: center;
    gap: 7px;

    .job-icon {
      color: #0284c7;
      font-size: 15px;
    }

    .job-title {
      font-size: 14.5px;
      font-weight: 700;
      color: #0f172a;
    }

    .job-meta {
      font-size: 11.5px;
      color: #64748b;
      background: #f1f5f9;
      padding: 2px 8px;
      border-radius: 10px;
      font-weight: 500;
    }
  }

  .skills-summary {
    display: flex;
    align-items: center;
    gap: 5px;
    flex-wrap: wrap;

    .skill-chip {
      font-size: 11.5px;
      color: #0369a1;
      background: #e0f2fe;
      padding: 2px 7px;
      border-radius: 4px;
      font-weight: 500;
    }

    .skill-more {
      font-size: 11px;
      color: #64748b;
      cursor: pointer;
      padding: 2px 6px;
      background: #f1f5f9;
      border-radius: 4px;
      font-weight: 500;
      transition: background 0.15s ease;

      &:hover {
        background: #e2e8f0;
      }
    }
  }

  .criteria-badges {
    display: flex;
    align-items: center;
    gap: 6px;

    .badge-item {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      font-size: 11px;
      font-weight: 600;
      padding: 2px 8px;
      border-radius: 10px;

      &.danger {
        background: #fef2f2;
        color: #dc2626;
        border: 1px solid #fecaca;
        .badge-dot { background: #ef4444; }
      }

      &.warning {
        background: #fffbeb;
        color: #d97706;
        border: 1px solid #fde68a;
        .badge-dot { background: #f59e0b; }
      }

      .badge-dot {
        width: 5px;
        height: 5px;
        border-radius: 50%;
      }
    }
  }

  .ribbon-right {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;

    .btn-toggle-drawer {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      transition: all 0.2s ease;

      .arrow-indicator {
        transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
        font-size: 11px;

        &.is-rotated {
          transform: rotate(180deg);
        }
      }

      &.is-active {
        background: #f1f5f9;
        color: #0284c7;
        border-color: #cbd5e1;
      }
    }

    .btn-edit-config {
      display: inline-flex;
      align-items: center;
      gap: 4px;
    }

    .btn-new-project-ribbon {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-weight: 600;
    }
  }

  .preset-drawer-content {
    margin-top: 14px;
    padding-top: 14px;
    border-top: 1px solid $separator;

    .drawer-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
      flex-wrap: wrap;
      gap: 8px;

      .drawer-title-wrap {
        display: flex;
        align-items: baseline;
        gap: 8px;

        .drawer-title {
          font-size: 14px;
          font-weight: 700;
          color: #1e293b;
        }

        .drawer-subtitle {
          font-size: 12px;
          color: $text-muted;
        }
      }

      .drawer-actions {
        display: flex;
        align-items: center;
        gap: 8px;
      }
    }
  }
}

/* 抽屉平滑展开动画 */
.expand-slide-enter-active,
.expand-slide-leave-active {
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  overflow: hidden;
}
.expand-slide-enter-from,
.expand-slide-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.job-details-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1500;
  background: rgba(15, 23, 42, 0.28);
  visibility: hidden;
  pointer-events: none;

  &.is-open {
    visibility: visible;
    pointer-events: auto;
  }
}

.job-details-expand {
  position: fixed;
  top: 50%;
  left: 50%;
  z-index: 1501;
  box-sizing: border-box;
  width: min(700px, calc(100vw - 32px));
  height: min(560px, calc(100vh - 64px));
  transform: translate(-50%, -50%);
  visibility: hidden;
  pointer-events: none;
  overflow-y: auto;
  overscroll-behavior: contain;
  contain: layout paint;
  padding: 0 20px 24px;
  background: #ffffff;
  border: 1px solid $separator;
  border-radius: $radius-md;
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.22);
  outline: none;

  &.is-open {
    visibility: visible;
    pointer-events: auto;
  }

  .expand-panel-header {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 52px;
    margin-bottom: 14px;
    background: #ffffff;
    border-bottom: 1px solid $separator;
  }

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
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 6px;

    .label-tip {
      font-size: 11px;
      color: $text-muted;
    }

    .quick-tags {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-wrap: wrap;

      .quick-title {
        font-size: 11px;
        color: #64748b;
      }

      .quick-pill {
        border: none;
        padding: 2px 7px;
        border-radius: 4px;
        font-size: 11px;
        cursor: pointer;
        transition: all 0.15s ease;

        &.danger {
          background: #fee2e2;
          color: #dc2626;
          &:hover { background: #fecaca; }
        }

        &.warning {
          background: #fef3c7;
          color: #d97706;
          &:hover { background: #fde68a; }
        }
      }
    }
  }

  .text-danger { color: #dc2626 !important; font-weight: 600; }
  .text-warning { color: #d97706 !important; font-weight: 600; }

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

@media (max-width: 720px) {
  .job-details-expand {
    width: calc(100vw - 20px);
    height: calc(100vh - 40px);
    .expand-grid { grid-template-columns: 1fr; }
    .expand-footer { flex-wrap: wrap; gap: 8px; }
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
  transition: all $transition-fast;

  &:hover {
    background: $system-blue-hover;
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 122, 255, 0.25);
  }
  &:active { transform: scale(0.97); }
}

.project-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 20px;
}

.project-card {
  background: $bg-primary;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  padding: 18px 20px;
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  position: relative;

  &:hover {
    border-color: #38bdf8;
    box-shadow: 0 10px 24px -4px rgba(2, 132, 199, 0.12), 0 4px 8px -2px rgba(0, 0, 0, 0.04);
    transform: translateY(-3px);

    .card-enter-link {
      color: $system-blue;
      transform: translateX(2px);
    }

    .card-delete-btn {
      opacity: 1;
    }
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 10px;

    .card-title-group {
      display: flex;
      align-items: center;
      gap: 8px;
      flex: 1;
      min-width: 0;

      .card-title {
        margin: 0;
        font-size: 16px;
        font-weight: 700;
        color: #0f172a;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .status-pill {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        font-size: 11px;
        font-weight: 600;
        padding: 2px 8px;
        border-radius: 12px;
        flex-shrink: 0;

        &.draft {
          background: #f1f5f9;
          color: #64748b;
          .status-dot { background: #94a3b8; }
        }
        &.analyzing {
          background: #e0f2fe;
          color: #0284c7;
          .status-dot {
            background: #0284c7;
            animation: pulse-dot 1.5s infinite;
          }
        }
        &.completed {
          background: #dcfce7;
          color: #15803d;
          .status-dot { background: #16a34a; }
        }

        .status-dot {
          width: 6px;
          height: 6px;
          border-radius: 50%;
        }
      }
    }

    .card-delete-btn {
      width: 26px;
      height: 26px;
      border: none;
      border-radius: 6px;
      background: transparent;
      color: #94a3b8;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      opacity: 0.5;
      transition: all $transition-fast;
      flex-shrink: 0;

      &:hover {
        background: #fee2e2;
        color: #dc2626;
      }
    }
  }

  .card-meta-tags {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;

    .meta-tag {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      font-size: 12px;
      font-weight: 500;
      padding: 3px 8px;
      border-radius: 6px;

      &.job-tag {
        background: #f8fafc;
        color: #334155;
        border: 1px solid #e2e8f0;
      }

      &.dept-tag {
        background: #f0fdf4;
        color: #166534;
        border: 1px solid #bbf7d0;
      }
    }
  }

  .card-info-grid {
    display: flex;
    align-items: center;
    gap: 14px;
    font-size: 12px;
    color: #64748b;

    .info-item {
      display: flex;
      align-items: center;
      gap: 4px;

      .info-label {
        color: #94a3b8;
      }

      .info-val strong {
        color: #0f172a;
      }

      &.date-item {
        margin-left: auto;
        font-size: 11.5px;
      }
    }
  }

  .card-remark {
    font-size: 11.5px;
    color: #64748b;
    background: #f8fafc;
    padding: 5px 8px;
    border-radius: 6px;
    border-left: 3px solid #cbd5e1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .card-progress-bar-wrap {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px 10px;
    background: #f8fafc;
    border-radius: 8px;
    border: 1px solid #f1f5f9;

    .progress-labels {
      display: flex;
      align-items: center;
      justify-content: space-between;
      font-size: 12px;

      .resume-count {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        font-weight: 600;
        color: #334155;
      }

      .analyzed-ratio {
        font-size: 11.5px;
        color: #64748b;
      }

      .max-score-badge {
        display: inline-flex;
        align-items: center;
        gap: 3px;
        font-size: 11px;
        font-weight: 700;
        color: #d97706;
        background: #fef3c7;
        padding: 1px 6px;
        border-radius: 4px;
      }
    }

    .progress-track {
      height: 4px;
      background: #e2e8f0;
      border-radius: 2px;
      overflow: hidden;

      .progress-fill {
        height: 100%;
        background: linear-gradient(90deg, #38bdf8, #0284c7);
        border-radius: 2px;
        transition: width 0.4s ease;
      }
    }
  }

  .card-footer-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: 6px;
    border-top: 1px solid #f1f5f9;

    .post-job-btn {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      border: 1px solid #bae6fd;
      border-radius: 6px;
      background: #f0f9ff;
      color: #0284c7;
      font-size: 11.5px;
      font-weight: 600;
      padding: 5px 10px;
      cursor: pointer;
      transition: all 0.2s ease;

      &:hover {
        background: #e0f2fe;
        border-color: #7dd3fc;
      }
    }

    .card-enter-link {
      display: inline-flex;
      align-items: center;
      gap: 3px;
      font-size: 12px;
      font-weight: 500;
      color: #94a3b8;
      transition: all 0.2s ease;
    }
  }
}

@keyframes pulse-dot {
  0% { transform: scale(0.95); opacity: 0.8; }
  50% { transform: scale(1.2); opacity: 1; }
  100% { transform: scale(0.95); opacity: 0.8; }
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
