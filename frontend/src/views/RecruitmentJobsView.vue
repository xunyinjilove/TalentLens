<template>
  <div class="job-management-view">
    <!-- 顶部导航栏 -->
    <header class="top-nav-bar">
      <div class="nav-left">
        <h1 class="page-title">职位管理</h1>
        <div class="page-subtitle">招聘信息 / 职位发布与维护</div>
      </div>
      <div class="nav-right">
        <div class="global-search-wrapper">
          <el-icon class="search-icon"><Search /></el-icon>
          <input
            v-model="globalSearch"
            type="text"
            class="global-search-input"
            placeholder="搜索职位、候选人、简历..."
            @keyup.enter="handleGlobalSearch"
          />
        </div>
      </div>
    </header>

    <!-- 工具筛选栏 -->
    <div class="table-toolbar">
      <div class="search-filter-box">
        <el-input
          v-model="keyword"
          placeholder="搜索职位名称..."
          clearable
          class="custom-search-input"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>

      <div class="filter-controls">
        <el-select
          v-model="selectedDepartment"
          placeholder="全部部门"
          clearable
          class="filter-select"
        >
          <el-option label="全部部门" value="" />
          <el-option
            v-for="dept in departmentOptions"
            :key="dept"
            :label="dept"
            :value="dept"
          />
        </el-select>

        <el-select
          v-model="selectedStatus"
          placeholder="全部状态"
          clearable
          class="filter-select"
        >
          <el-option label="全部状态" value="" />
          <el-option label="招聘中" value="publishing" />
          <el-option label="草稿" value="draft" />
          <el-option label="已暂停" value="paused" />
        </el-select>

        <el-button type="primary" class="create-job-btn" @click="openCreateModal">
          <el-icon style="margin-right: 4px;"><Plus /></el-icon>
          新建职位
        </el-button>
      </div>
    </div>

    <!-- 现代表格主体卡片 -->
    <div class="modern-table-card">
      <div ref="tableResponsiveRef" class="table-responsive">
        <table class="saas-table">
          <thead>
            <tr>
              <th class="col-sourcing">寻才</th>
              <th class="col-title">职位名称</th>
              <th class="col-dept">部门</th>
              <th class="col-headcount">招聘人数</th>
              <th class="col-stats">投递 / 合适</th>
              <th class="col-status">状态</th>
              <th class="col-time">更新时间</th>
              <th class="col-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading" class="empty-row">
              <td colspan="8">
                <div class="empty-state">
                  <el-icon class="is-loading" style="font-size: 24px;"><Loading /></el-icon>
                  <p>正在载入职位数据…</p>
                </div>
              </td>
            </tr>
            <tr v-else-if="!filteredProjects.length" class="empty-row">
              <td colspan="8">
                <div class="empty-state">
                  <p>未找到符合条件的职位</p>
                  <el-button size="small" type="primary" plain @click="openCreateModal">立即新建职位</el-button>
                </div>
              </td>
            </tr>
            <tr
              v-for="project in filteredProjects"
              :key="project.id"
              class="table-row"
            >
              <!-- 快捷寻才（最左侧第一列） -->
              <td class="col-sourcing">
                <button
                  class="sourcing-pill-btn"
                  title="立即为该岗位检索人才"
                  @click.stop="openSourcing(project)"
                >
                  <el-icon><Position /></el-icon>
                  <span>寻才</span>
                </button>
              </td>

              <!-- 职位名称与次行信息 -->
              <td class="col-title">
                <div class="job-title-wrapper">
                  <span class="job-name" :title="project.job_config?.title || project.name">
                    {{ project.job_config?.title || project.name }}
                  </span>
                  <div class="job-subinfo">
                    <span>{{ project.job_config?.city || '上海' }}</span>
                    <span class="dot-sep">·</span>
                    <span>{{ project.job_config?.employment_type || '全职' }}</span>
                    <span class="dot-sep">·</span>
                    <span class="salary-text">{{ project.job_config?.salary_range || '15-25K' }}</span>
                  </div>
                </div>
              </td>

              <!-- 部门 -->
              <td class="col-dept">
                <span class="dept-text">{{ project.department || '未分配' }}</span>
              </td>

              <!-- 招聘人数 -->
              <td class="col-headcount">
                <span class="headcount-text">{{ project.headcount || 1 }}</span>
              </td>

              <!-- 投递 / 合适 -->
              <td class="col-stats">
                <span class="stats-text">
                  {{ getStats(project.id).totalApplications }} / {{ getStats(project.id).suitableCount }}
                </span>
              </td>

              <!-- 状态胶囊标签 -->
              <td class="col-status">
                <span class="status-badge" :class="normalizeStatusClass(project.status)">
                  <span class="status-dot"></span>
                  {{ normalizeStatusLabel(project.status) }}
                </span>
              </td>

              <!-- 更新时间 -->
              <td class="col-time">
                <span class="time-text">{{ formatRelativeTime(project.updated_at || project.created_at) }}</span>
              </td>

              <!-- 操作图标组 -->
              <td class="col-actions">
                <div class="action-btn-group">
                  <!-- 快捷寻才 -->
                  <button
                    class="action-icon-btn highlight"
                    title="立即为该岗位检索人才"
                    @click.stop="openSourcing(project)"
                  >
                    <el-icon><Position /></el-icon>
                  </button>

                  <!-- 编辑 -->
                  <button
                    class="action-icon-btn"
                    title="编辑职位配置与规则"
                    @click="openEditModal(project)"
                  >
                    <el-icon><EditPen /></el-icon>
                  </button>

                  <!-- 穿透进入专属人才分库 -->
                  <button
                    class="action-icon-btn"
                    title="查看专属人才库分库"
                    @click="goToTalentPool(project.id)"
                  >
                    <el-icon><View /></el-icon>
                  </button>

                  <!-- 更多下拉操作 -->
                  <el-dropdown trigger="click" @command="cmd => handleActionCommand(cmd, project)">
                    <button class="action-icon-btn" title="更多操作">
                      <el-icon><MoreFilled /></el-icon>
                    </button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="sourcing">
                          <el-icon><Position /></el-icon>全渠道矩阵寻才
                        </el-dropdown-item>
                        <el-dropdown-item command="mapping51">
                          <el-icon><Link /></el-icon>关联 51job 平台职位
                        </el-dropdown-item>
                        <el-dropdown-item command="evaluation">
                          <el-icon><Document /></el-icon>项目评测详情
                        </el-dropdown-item>
                        <el-dropdown-item divided command="toggle_status">
                          <el-icon><Refresh /></el-icon>
                          {{ project.status === 'paused' ? '恢复为招聘中' : '暂停该职位' }}
                        </el-dropdown-item>
                        <el-dropdown-item command="delete" style="color: #ef4444;">
                          <el-icon><Delete /></el-icon>删除职位
                        </el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 新建 / 编辑职位模态弹窗 -->
    <el-dialog
      v-model="editModalVisible"
      :title="isEditing ? '编辑职位配置' : '新建招聘职位'"
      width="680px"
      destroy-on-close
      class="job-form-dialog"
    >
      <el-form :model="jobForm" label-position="top" class="job-dialog-form">
        <div class="form-row two-cols">
          <el-form-item label="职位名称 *" required>
            <el-input v-model="jobForm.title" placeholder="如：高级前端工程师" />
          </el-form-item>
          <el-form-item label="所属部门">
            <el-input v-model="jobForm.department" placeholder="如：研发中心、产品部" />
          </el-form-item>
        </div>

        <div class="form-row three-cols">
          <el-form-item label="招聘人数">
            <el-input-number v-model="jobForm.headcount" :min="1" :max="99" style="width: 100%" />
          </el-form-item>
          <el-form-item label="工作城市">
            <el-select v-model="jobForm.city" placeholder="选择城市" allow-create filterable style="width: 100%">
              <el-option label="上海" value="上海" />
              <el-option label="北京" value="北京" />
              <el-option label="深圳" value="深圳" />
              <el-option label="杭州" value="杭州" />
              <el-option label="广州" value="广州" />
              <el-option label="宁波" value="宁波" />
              <el-option label="南昌" value="南昌" />
              <el-option label="苏州" value="苏州" />
              <el-option label="南京" value="南京" />
              <el-option label="武汉" value="武汉" />
              <el-option label="成都" value="成都" />
              <el-option label="远程" value="远程" />
            </el-select>
            <div v-if="suggested51JobCity && suggested51JobCity !== jobForm.city" class="city-sync-suggestion">
              💡 51job [职位管理] 匹配城市：<strong>{{ suggested51JobCity }}</strong>
              <el-button link type="primary" size="small" @click="jobForm.city = suggested51JobCity">一键应用</el-button>
            </div>
          </el-form-item>
          <el-form-item label="工作性质">
            <el-select v-model="jobForm.employment_type" placeholder="性质">
              <el-option label="全职" value="全职" />
              <el-option label="兼职" value="兼职" />
              <el-option label="实习" value="实习" />
              <el-option label="远程" value="远程" />
            </el-select>
          </el-form-item>
        </div>

        <div class="form-row two-cols">
          <el-form-item label="薪资范围">
            <el-select v-model="jobForm.salary_range" placeholder="薪资" allow-create filterable>
              <el-option label="10-18K" value="10-18K" />
              <el-option label="15-25K" value="15-25K" />
              <el-option label="25-40K" value="25-40K" />
              <el-option label="40-60K" value="40-60K" />
              <el-option label="面议" value="面议" />
            </el-select>
          </el-form-item>
          <el-form-item label="当前状态">
            <el-select v-model="jobForm.status">
              <el-option label="招聘中" value="publishing" />
              <el-option label="草稿" value="draft" />
              <el-option label="已暂停" value="paused" />
            </el-select>
          </el-form-item>
        </div>

        <div class="form-row two-cols">
          <el-form-item label="经验要求">
            <el-select
              v-model="jobForm.experience_years"
              placeholder="输入或选择经验要求（如：3-5年、应届生）"
              allow-create
              filterable
              default-first-option
              clearable
            >
              <el-option label="经验不限" value="经验不限" />
              <el-option label="应届毕业生" value="应届毕业生" />
              <el-option label="1 年以上" value="1 年以上" />
              <el-option label="1-3 年" value="1-3 年" />
              <el-option label="3-5 年" value="3-5 年" />
              <el-option label="5-8 年" value="5-8 年" />
              <el-option label="5 年以上" value="5 年以上" />
              <el-option label="8-10 年" value="8-10 年" />
              <el-option label="10 年以上" value="10 年以上" />
            </el-select>
          </el-form-item>
          <el-form-item label="最低学历">
            <el-select v-model="jobForm.education_level">
              <el-option label="学历不限" value="不限" />
              <el-option label="大专" value="大专" />
              <el-option label="本科" value="本科" />
              <el-option label="硕士" value="硕士" />
              <el-option label="博士" value="博士" />
            </el-select>
          </el-form-item>
        </div>

        <el-form-item>
          <template #label>
            <div class="job-description-label-row">
              <span>岗位职责与要求 (AI 画像核心参考)</span>
              <el-dropdown trigger="click" @command="handleAiDraftCommand">
                <el-button class="ai-description-action" type="primary" link :loading="aiDraftGenerating">
                  ✨ AI 一键帮写 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="fresh">按当前职位全新起草 (推荐)</el-dropdown-item>
                    <el-dropdown-item command="refine" :disabled="!jobForm.job_description.trim()">基于现有描述优化润色</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>

          <!-- 岗位方向确认卡片（针对器械/硬件等涉及多个细分领域的岗位） -->
          <div v-if="directionPromptVisible" class="direction-prompt-card">
            <div class="direction-prompt-title">
              <el-icon color="#4f46e5"><Guide /></el-icon>
              <span>职位【{{ jobForm.title }}】涉及多个专业方向，请指定具体的细分方向：</span>
            </div>
            <div class="direction-tags">
              <el-button
                v-for="dir in suggestedDirections"
                :key="dir"
                size="small"
                plain
                type="primary"
                :loading="aiDraftGenerating"
                @click="() => selectDirectionAndGenerate(dir)"
              >
                {{ dir }}
              </el-button>
            </div>
            <div class="direction-custom-row">
              <el-input
                v-model="customDirectionInput"
                size="small"
                placeholder="或输入自定义专业方向，如：医疗器械机械结构设计"
                style="max-width: 320px;"
                @keyup.enter="() => selectDirectionAndGenerate(customDirectionInput)"
              />
              <el-button
                size="small"
                type="primary"
                :disabled="!customDirectionInput.trim()"
                @click="() => selectDirectionAndGenerate(customDirectionInput)"
              >
                按此方向生成
              </el-button>
              <el-button size="small" @click="directionPromptVisible = false">取消</el-button>
            </div>
          </div>

          <!-- 跨领域历史遗留技能冲突警告 -->
          <el-alert
            v-if="activeConflicts.length"
            type="warning"
            show-icon
            :closable="false"
            style="margin-bottom: 12px"
          >
            <template #title>
              <span>跨领域技能冲突提示</span>
            </template>
            <div style="font-size: 12px; margin-top: 4px;">
              <div v-for="(c, idx) in activeConflicts" :key="idx">{{ c }}</div>
              <div style="margin-top: 8px;">
                <el-button size="small" type="danger" plain @click="removeConflictingSkills">
                  一键移除表单中不匹配的遗留技能
                </el-button>
              </div>
            </div>
          </el-alert>

          <el-input
            v-model="jobForm.job_description"
            type="textarea"
            :rows="5"
            placeholder="填写职位描述、工作职责以及基本技能要求…"
          />
          <div class="description-help">AI 草稿只供参考；核对后点击“保存职位”才会更新岗位配置。</div>
        </el-form-item>

        <div v-if="aiDraft" class="ai-description-preview">
          <div class="ai-preview-heading">
            <div>
              <strong>✨ AI 起草预览</strong>
              <span v-if="currentDraftDirection" class="direction-badge">{{ currentDraftDirection }}</span>
            </div>
            <div style="display: flex; gap: 8px;">
              <el-button size="small" @click="aiDraft = ''">放弃草稿</el-button>
              <el-button type="primary" size="small" @click="applyJobDescriptionDraft">应用到编辑框</el-button>
            </div>
          </div>
          <pre>{{ aiDraft }}</pre>
        </div>

        <el-form-item>
          <template #label>
            <div class="skills-label-wrapper">
              <div class="skills-label-title">必备关键技能 / 评分项 (回车分隔)</div>
              <div class="skills-label-tip">（关键技能用于生成技能指标雷达，可分析八项以内的技能，若不足则AI自动匹配技能）</div>
            </div>
          </template>
          <el-input
            v-model="skillsInput"
            placeholder="输入核心专业技能，多个技能以逗号或回车隔开 (选填，留空由 AI 智能拟定)"
            @blur="syncSkills"
          />
        </el-form-item>

        <el-form-item label="初试重点核查题纲（进入初试环节时自动生成重点核验题目，以逗号或分号分隔）">
          <el-input
            v-model="interviewQuestionsInput"
            placeholder="如：能独立完成核心架构设计；具备大型复杂系统性能调优经验"
            @blur="syncInterviewQuestions"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="editModalVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="saveJob">保存职位</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 关联 51job 平台职位对话框 -->
    <el-dialog v-model="mappingOpen" title="关联 51job 平台职位" width="460px">
      <p class="wb-muted">只按平台职位 ID 归档主动投递。请从 51job 企业端核对该 ID。</p>
      <el-input v-model="platformJobId" placeholder="51job 职位 ID" style="margin-top: 12px;" />
      <template #footer>
        <el-button @click="mappingOpen = false">取消</el-button>
        <el-button type="primary" @click="saveMapping">保存关联</el-button>
      </template>
    </el-dialog>

    <!-- 矩阵寻才对话框 -->
    <BossSearchDialog
      v-if="sourcingProject"
      v-model="showSourcingDialog"
      :project-id="sourcingProject.id"
      :project-name="sourcingProject.name"
      :job-title="sourcingProject.job_config?.title || sourcingProject.name"
      :city="sourcingProject.job_config?.city || '上海'"
      :exp-years="sourcingProject.job_config?.experience_years || 0"
      :edu-level="sourcingProject.job_config?.education_level || ''"
      @refresh="load"
      @update:model-value="val => { showSourcingDialog = val; if (!val) sourcingProject = null }"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Search,
  Plus,
  Loading,
  EditPen,
  View,
  MoreFilled,
  Position,
  SetUp,
  Link,
  Document,
  Refresh,
  Delete,
  ArrowDown,
  Guide,
  WarningFilled
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useProjectStore, type Project } from '../composables/useProjectStore'
import { useRecruitmentStore } from '../composables/useRecruitmentStore'
import BossSearchDialog from '../components/BossSearchDialog.vue'

interface JobStats {
  totalApplications: number
  suitableCount: number
}

const router = useRouter()
const route = useRoute()
const projectStore = useProjectStore()
const recruitmentStore = useRecruitmentStore()

// 状态
const loading = ref(false)
const saving = ref(false)
const aiDraftGenerating = ref(false)
const aiDraft = ref('')
const currentDraftDirection = ref('')
const currentFormInstanceId = ref('')
const directionPromptVisible = ref(false)
const suggestedDirections = ref<string[]>([])
const customDirectionInput = ref('')
const activeConflicts = ref<string[]>([])
let activeRequestSeq = 0
const keyword = ref('')
const globalSearch = ref('')
const selectedDepartment = ref('')
const selectedStatus = ref('')
const jobStatsMap = ref<Record<string, JobStats>>({})
const cached51JobJobs = ref<Array<{ title: string; city: string; salary?: string }>>([])

// 表格横向滚轮滚动
const tableResponsiveRef = ref<HTMLElement | null>(null)

function handleTableWheel(e: WheelEvent) {
  const el = tableResponsiveRef.value
  if (!el) return
  if (el.scrollWidth > el.clientWidth) {
    if (Math.abs(e.deltaY) >= Math.abs(e.deltaX)) {
      el.scrollLeft += e.deltaY
      e.preventDefault()
    }
  }
}

// 弹窗状态
const editModalVisible = ref(false)
const isEditing = ref(false)
const currentEditProjectId = ref<string | null>(null)
const sourcingProject = ref<Project | null>(null)
const showSourcingDialog = ref(false)

// 51job 关联
const mapProject = ref<Project | null>(null)
const platformJobId = ref('')
const mappings = ref<Array<{ projectId: string; platform: string; platformJobId: string }>>([])
const mappingOpen = computed({
  get: () => !!mapProject.value,
  set: val => { if (!val) mapProject.value = null }
})

// 表单数据
const jobForm = reactive({
  title: '',
  direction: '',
  department: '研发中心',
  headcount: 1,
  recruiter: '',
  city: '上海',
  employment_type: '全职',
  salary_range: '25-40K',
  status: 'publishing',
  experience_years: '3-5 年',
  education_level: '本科',
  job_description: ''
})

function formatExpRequirement(jobConfig: any): string {
  if (jobConfig?.experience_text) return String(jobConfig.experience_text)
  if (jobConfig?.experience_years !== undefined && jobConfig?.experience_years !== null) {
    return jobConfig.experience_years === 0 ? '经验不限' : `${jobConfig.experience_years} 年以上`
  }
  return '3-5 年'
}

function parseExperienceYears(val: any): number {
  if (typeof val === 'number') return isNaN(val) ? 0 : Math.max(0, val)
  if (!val) return 0
  const match = String(val).match(/\d+/)
  return match ? parseInt(match[0], 10) : 0
}

const skillsInput = ref('')
const interviewQuestionsInput = ref('')

function splitTags(input: string): string[] {
  return input
    .split(/[,，;；\n]/)
    .map(s => s.trim())
    .filter(Boolean)
}

function syncSkills() {
  // synced when saving
}
function syncInterviewQuestions() {}

// 部门选项
const departmentOptions = computed(() => {
  const depts = new Set<string>()
  for (const p of projectStore.projects) {
    if (p.department) depts.add(p.department)
  }
  return Array.from(depts)
})

// 过滤后的项目列表
const filteredProjects = computed(() => {
  return projectStore.projects.filter(project => {
    const title = project.job_config?.title || project.name || ''
    const dept = project.department || ''
    const matchKw = !keyword.value ||
      title.toLowerCase().includes(keyword.value.toLowerCase()) ||
      dept.toLowerCase().includes(keyword.value.toLowerCase())
    const matchDept = !selectedDepartment.value || dept === selectedDepartment.value
    const matchStatus = !selectedStatus.value ||
      (selectedStatus.value === 'publishing' && (!project.status || project.status === 'publishing' || project.status === 'analyzing' || project.status === 'completed')) ||
      project.status === selectedStatus.value
    return matchKw && matchDept && matchStatus
  })
})

function getStats(projectId: string): JobStats {
  if (jobStatsMap.value[projectId]) {
    return jobStatsMap.value[projectId]
  }
  // 备用计算：从 recruitmentStore.candidates 提取
  const candidates = recruitmentStore.candidates.filter(c => c.projectId === projectId)
  const total = candidates.length
  const suitable = candidates.filter(c => {
    if (c.stage === 'rejected') return false
    if (c.abilityScore >= 60) return true
    return ['first_interview', 'second_interview', 'offer', 'hired', 'communicating'].includes(c.stage)
  }).length
  return { totalApplications: total, suitableCount: suitable }
}

function normalizeStatusClass(status?: string): string {
  if (!status || status === 'publishing' || status === 'analyzing' || status === 'completed') {
    return 'status-publishing'
  }
  if (status === 'draft') return 'status-draft'
  if (status === 'paused') return 'status-paused'
  return 'status-publishing'
}

function normalizeStatusLabel(status?: string): string {
  if (!status || status === 'publishing' || status === 'analyzing' || status === 'completed') {
    return '招聘中'
  }
  if (status === 'draft') return '草稿'
  if (status === 'paused') return '已暂停'
  return status
}

function formatRelativeTime(dateStr?: string): string {
  if (!dateStr) return '刚刚'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return '刚刚'
  const now = new Date()
  const diffSec = Math.floor((now.getTime() - d.getTime()) / 1000)
  if (diffSec < 60) return '刚刚'
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`
  if (diffSec < 86400) {
    const hours = Math.floor(diffSec / 3600)
    if (now.getDate() === d.getDate()) return `${hours} 小时前`
    return '昨天'
  }
  const days = Math.floor(diffSec / 86400)
  if (days === 1) return '昨天'
  if (days < 7) return `${days} 天前`
  if (days < 30) return `${Math.floor(days / 7)} 周前`
  return d.toLocaleDateString('zh-CN')
}

// 导航至人才库并激活对应分库
function goToTalentPool(projectId: string) {
  router.push({ path: '/talent', query: { project: projectId } })
}

// 自动静默同步 51job [职位管理] 岗位城市（后台无感同步，不打扰用户）
async function autoSync51JobCities() {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const res = await api.Sync51JobJobManagementCities()
    if (res.jobs && res.jobs.length) {
      cached51JobJobs.value = res.jobs
    }
    if (res.success && res.updatedCount > 0) {
      await projectStore.loadProjects()
    }
  } catch {
    // 静默处理网络或登录异常
  }
}

// 快速异步拉取 51job 职位管理列表缓存
async function ensure51JobJobsCache() {
  if (cached51JobJobs.value.length > 0) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const res = await api.Fetch51JobManagementJobs()
    if (res.status === 'success' && res.jobs) {
      cached51JobJobs.value = res.jobs
    }
  } catch {
    // 忽略静默网络异常
  }
}

// 智能根据表单职位名称匹配 51job 对应发布城市
const suggested51JobCity = computed(() => {
  const currentTitle = jobForm.title.trim()
  if (!currentTitle || !cached51JobJobs.value.length) return ''
  const cleanTitle = currentTitle.toLowerCase().replace(/[（）()]/g, '')
  const matched = cached51JobJobs.value.find(j => {
    const jClean = j.title.toLowerCase().replace(/[（）()]/g, '')
    return jClean === cleanTitle || jClean.includes(cleanTitle) || cleanTitle.includes(jClean)
  })
  return matched ? matched.city : ''
})

// 模态弹窗控制
function openCreateModal() {
  void ensure51JobJobsCache()
  aiDraft.value = ''
  currentDraftDirection.value = ''
  directionPromptVisible.value = false
  suggestedDirections.value = []
  customDirectionInput.value = ''
  activeConflicts.value = []
  isEditing.value = false
  currentEditProjectId.value = null
  currentFormInstanceId.value = 'form_' + Date.now()
  jobForm.title = ''
  jobForm.direction = ''
  jobForm.department = '研发中心'
  jobForm.headcount = 1
  jobForm.recruiter = ''
  jobForm.city = '上海'
  jobForm.employment_type = '全职'
  jobForm.salary_range = '25-40K'
  jobForm.status = 'publishing'
  jobForm.experience_years = '3-5 年'
  jobForm.education_level = '本科'
  jobForm.job_description = ''
  skillsInput.value = '' // 清空默认值，杜绝硬编码预填前端技能
  interviewQuestionsInput.value = ''
  editModalVisible.value = true
}

function openEditModal(project: Project) {
  void ensure51JobJobsCache()
  aiDraft.value = ''
  currentDraftDirection.value = ''
  directionPromptVisible.value = false
  suggestedDirections.value = []
  customDirectionInput.value = ''
  activeConflicts.value = []
  isEditing.value = true
  currentEditProjectId.value = project.id
  currentFormInstanceId.value = 'form_' + Date.now()
  jobForm.title = project.job_config?.title || project.name
  jobForm.direction = ''
  jobForm.department = project.department || ''
  jobForm.headcount = project.headcount || 1
  jobForm.recruiter = project.recruiter || ''
  jobForm.city = project.job_config?.city || '上海'
  jobForm.employment_type = project.job_config?.employment_type || '全职'
  jobForm.salary_range = project.job_config?.salary_range || '25-40K'
  jobForm.status = project.status || 'publishing'
  jobForm.experience_years = formatExpRequirement(project.job_config)
  jobForm.education_level = project.job_config?.education_level || '本科'
  jobForm.job_description = project.job_config?.job_description || ''

  // 修复空数组短路 bug：若 core_criteria 为 [] 则正确读取 required_skills
  const existingSkills = (project.job_config?.core_criteria && project.job_config.core_criteria.length > 0)
    ? project.job_config.core_criteria
    : (project.job_config?.required_skills || [])
  skillsInput.value = existingSkills.join(', ')

  const qList = (project.job_config?.interview_questions || []).slice()
  if (!qList.length) {
    if (project.job_config?.red_lines) qList.push(...project.job_config.red_lines)
    if (project.job_config?.bonus_points) qList.push(...project.job_config.bonus_points)
  }
  interviewQuestionsInput.value = qList.join(', ')
  editModalVisible.value = true
}

// 职位标题变化时，使旧草稿和旧方向失效
watch(() => jobForm.title, (newTitle, oldTitle) => {
  if (oldTitle && newTitle !== oldTitle) {
    aiDraft.value = ''
    currentDraftDirection.value = ''
    directionPromptVisible.value = false
    activeConflicts.value = []
  }
  // 若为新建且当前城市是默认上海，匹配到 51job 对应城市则自动带入
  if (!isEditing.value && suggested51JobCity.value && (jobForm.city === '上海' || !jobForm.city)) {
    jobForm.city = suggested51JobCity.value
  }
})

function handleAiDraftCommand(cmd: string) {
  if (cmd === 'refine') {
    void generateJobDescription('refine')
  } else {
    void generateJobDescription('fresh')
  }
}

function selectDirectionAndGenerate(dir: string) {
  const cleanDir = dir.trim()
  if (!cleanDir) return
  jobForm.direction = cleanDir
  directionPromptVisible.value = false
  customDirectionInput.value = ''
  void generateJobDescription('fresh', cleanDir)
}

function removeConflictingSkills() {
  const current = splitTags(skillsInput.value)
  const filtered = current.filter(s => {
    const sLower = s.toLowerCase()
    return !['vue', 'vue3', 'typescript', 'ts', 'react', 'angular', 'css', 'css3', 'html5', 'webpack', 'vite', '前端开发', '代码评审', '性能优化'].some(kw => sLower.includes(kw))
  })
  skillsInput.value = filtered.join(', ')
  activeConflicts.value = []
  ElMessage.success('已清理跨领域遗留技能')
}

async function generateJobDescription(mode: 'fresh' | 'refine' = 'fresh', explicitDirection?: string) {
  const title = jobForm.title.trim()
  if (!title) {
    ElMessage.warning('请先填写职位名称')
    return
  }
  const thisInstanceId = currentFormInstanceId.value
  const reqSeq = ++activeRequestSeq
  aiDraftGenerating.value = true
  aiDraft.value = ''
  directionPromptVisible.value = false

  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const dir = explicitDirection !== undefined ? explicitDirection : jobForm.direction
    const res = await api.GenerateJobDescriptionDraft({
      formInstanceId: thisInstanceId,
      projectId: currentEditProjectId.value || '',
      requestId: `req_${reqSeq}`,
      title,
      direction: dir,
      department: jobForm.department.trim(),
      city: jobForm.city,
      experienceYears: parseExperienceYears(jobForm.experience_years),
      educationLevel: jobForm.education_level,
      skills: splitTags(skillsInput.value),
      currentText: mode === 'refine' ? jobForm.job_description.trim() : '',
      mode
    })

    // 竞态保护：如果返回时表单已关闭或切换到其他职位，丢弃过期响应
    if (currentFormInstanceId.value !== thisInstanceId || jobForm.title.trim() !== title) {
      return
    }

    if (res.status === 'needs_direction') {
      suggestedDirections.value = res.suggestedDirections || []
      directionPromptVisible.value = true
      return
    }

    if (res.status === 'ready' || res.draft) {
      aiDraft.value = res.draft
      currentDraftDirection.value = res.direction || dir || title
      activeConflicts.value = res.conflicts || []
      if (res.conflicts && res.conflicts.length > 0) {
        ElMessage.warning('检测到历史遗留技能与当前岗位方向冲突，已自动隔离')
      } else {
        ElMessage.success('已生成匹配该岗位的职责与要求草稿')
      }
    } else {
      ElMessage.warning(res.message || '未能生成有效文案，请重试')
    }
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    if (activeRequestSeq === reqSeq) {
      aiDraftGenerating.value = false
    }
  }
}

function applyJobDescriptionDraft() {
  if (!aiDraft.value) return
  jobForm.job_description = aiDraft.value
  aiDraft.value = ''
  activeConflicts.value = []
  ElMessage.success('草稿已填入编辑框，核对后请点击下方的“保存职位”')
}

async function saveJob() {
  if (!jobForm.title.trim()) {
    ElMessage.warning('请填写职位名称')
    return
  }
  saving.value = true
  try {
    const skills = splitTags(skillsInput.value)
    const interviewQuestions = splitTags(interviewQuestionsInput.value)
    const expText = String(jobForm.experience_years ?? '').trim()
    const expYears = parseExperienceYears(expText)

    const jobConfig = {
      title: jobForm.title.trim(),
      requirements: [],
      required_skills: skills,
      experience_years: expYears,
      experience_text: expText,
      education_level: jobForm.education_level,
      job_description: jobForm.job_description.trim(),
      city: jobForm.city,
      employment_type: jobForm.employment_type,
      salary_range: jobForm.salary_range,
      interview_questions: interviewQuestions,
      core_criteria: skills
    }

    if (isEditing.value && currentEditProjectId.value) {
      const existing = projectStore.projects.find(p => p.id === currentEditProjectId.value)
      if (existing) {
        const updated: Project = {
          ...existing,
          name: jobForm.title.trim(),
          department: jobForm.department.trim(),
          headcount: jobForm.headcount,
          recruiter: jobForm.recruiter?.trim() || '',
          status: jobForm.status,
          job_config: jobConfig,
          updated_at: new Date().toISOString()
        }
        await projectStore.updateProject(updated)
        ElMessage.success('职位已更新')
      }
    } else {
      const created = await projectStore.createProject(
        jobForm.title.trim(),
        jobForm.department.trim(),
        jobForm.headcount,
        jobForm.recruiter?.trim() || '',
        '',
        jobConfig
      )
      if (created) {
        if (jobForm.status !== 'publishing') {
          created.status = jobForm.status
          await projectStore.updateProject(created)
        }
        ElMessage.success('新建职位成功，已为您在人才库建立专属分库！')
      }
    }

    editModalVisible.value = false
    await load()
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    saving.value = false
  }
}

async function openSourcing(project: Project) {
  sourcingProject.value = project
  await nextTick()
  showSourcingDialog.value = true
}

// 更多菜单命令处理
async function handleActionCommand(cmd: string, project: Project) {
  if (cmd === 'sourcing') {
    openSourcing(project)
  } else if (cmd === 'rules') {
    ruleProject.value = project
  } else if (cmd === 'mapping51') {
    mapProject.value = project
  } else if (cmd === 'evaluation') {
    router.push('/project/' + project.id)
  } else if (cmd === 'toggle_status') {
    const nextStatus = project.status === 'paused' ? 'publishing' : 'paused'
    const updated: Project = { ...project, status: nextStatus }
    await projectStore.updateProject(updated)
    ElMessage.success(`职位状态已调整为「${normalizeStatusLabel(nextStatus)}」`)
    await load()
  } else if (cmd === 'delete') {
    try {
      await ElMessageBox.confirm(
        `确定要删除职位「${project.job_config?.title || project.name}」吗？关联的人才分库亦将被移除。`,
        '删除确认',
        { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
      )
      await projectStore.deleteProject(project.id)
      ElMessage.success('职位已删除')
      await load()
    } catch {}
  }
}

async function saveMapping() {
  if (!mapProject.value || !platformJobId.value.trim()) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.SetPlatformJobMapping('51job', '', platformJobId.value.trim(), mapProject.value.id)
    ElMessage.success('平台职位 ID 已关联到项目')
    await loadMappings()
    mappingOpen.value = false
    platformJobId.value = ''
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

async function loadMappings() {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    mappings.value = (await api.ListPlatformJobMappings()) || []
  } catch {
    mappings.value = []
  }
}

async function loadJobStats() {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    if (api && typeof api.GetJobStatistics === 'function') {
      const stats = await api.GetJobStatistics()
      if (stats) jobStatsMap.value = stats
    }
  } catch {}
}

async function load() {
  loading.value = true
  try {
    await Promise.all([
      projectStore.loadProjects(),
      recruitmentStore.loadCandidates(),
      loadJobStats(),
      loadMappings()
    ])
  } finally {
    loading.value = false
  }
}

function handleGlobalSearch() {
  if (globalSearch.value.trim()) {
    keyword.value = globalSearch.value.trim()
  }
}

onMounted(async () => {
  if (tableResponsiveRef.value) {
    tableResponsiveRef.value.addEventListener('wheel', handleTableWheel, { passive: false })
  }
  await load()
  void autoSync51JobCities()
  const editId = String(route.query.edit || '')
  if (!editId) return
  const project = projectStore.projects.find(item => item.id === editId)
  if (project) openEditModal(project)
  else ElMessage.warning('目标职位不存在或已删除')
})

onUnmounted(() => {
  if (tableResponsiveRef.value) {
    tableResponsiveRef.value.removeEventListener('wheel', handleTableWheel)
  }
})
</script>

<style scoped>
.job-management-view {
  display: flex;
  flex-direction: column;
  padding: 24px 32px;
  background-color: #f8fafc;
  min-height: 100vh;
  box-sizing: border-box;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  color: #1e293b;
}

/* 顶部导航栏 */
.top-nav-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: -0.02em;
}

.page-subtitle {
  margin-top: 4px;
  font-size: 13px;
  color: #94a3b8;
}

.nav-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.global-search-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.global-search-input {
  width: 260px;
  height: 38px;
  padding: 0 16px 0 38px;
  background-color: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 20px;
  font-size: 13px;
  color: #334155;
  outline: none;
  transition: all 0.2s ease;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.02);
}

.global-search-input:focus {
  border-color: #6366f1;
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.12);
  width: 290px;
}

.global-search-input::placeholder {
  color: #94a3b8;
}

.global-search-wrapper .search-icon {
  position: absolute;
  left: 14px;
  color: #94a3b8;
  font-size: 15px;
  pointer-events: none;
}

.nav-icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  background-color: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 50%;
  color: #64748b;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
}

.nav-icon-btn:hover {
  background-color: #f1f5f9;
  color: #334155;
}

.notification-badge {
  position: absolute;
  top: 9px;
  right: 9px;
  width: 6px;
  height: 6px;
  background-color: #ef4444;
  border-radius: 50%;
}

.user-avatar-circle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  background: linear-space;
  background: #6366f1;
  color: #ffffff;
  font-weight: 600;
  font-size: 15px;
  border-radius: 50%;
  box-shadow: 0 2px 6px rgba(99, 102, 241, 0.3);
  cursor: pointer;
}

/* 工具栏 */
.table-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
  gap: 16px;
}

.search-filter-box {
  flex: 1;
  max-width: 480px;
}

.custom-search-input :deep(.el-input__wrapper) {
  border-radius: 10px;
  height: 40px;
  background-color: #ffffff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}

.filter-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.filter-select {
  width: 130px;
}

.filter-select :deep(.el-input__wrapper) {
  border-radius: 10px;
  height: 40px;
  background-color: #ffffff;
}

.create-job-btn {
  height: 40px;
  padding: 0 20px;
  border-radius: 10px;
  background-color: #4f46e5 !important;
  border-color: #4f46e5 !important;
  font-weight: 500;
  box-shadow: 0 2px 8px rgba(79, 70, 229, 0.25);
  transition: all 0.2s ease;
}

.create-job-btn:hover {
  background-color: #4338ca !important;
  border-color: #4338ca !important;
  transform: translateY(-1px);
}

/* 现代表格主体 */
.modern-table-card {
  background-color: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02), 0 4px 12px rgba(0, 0, 0, 0.01);
  overflow: hidden;
}

.table-responsive {
  width: 100%;
  overflow-x: auto;
}

.saas-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}

.saas-table th {
  padding: 16px 20px;
  font-size: 13px;
  font-weight: 600;
  color: #475569;
  background-color: #fafbfd;
  border-bottom: 1px solid #edf2f7;
  white-space: nowrap;
}

.saas-table td {
  padding: 16px 20px;
  vertical-align: middle;
  border-bottom: 1px solid #f1f5f9;
  transition: background-color 0.15s ease;
}

.table-row:hover td {
  background-color: #f8fafc;
}

/* 列定义与内容排版 */
.col-sourcing {
  width: 90px;
  min-width: 90px;
  text-align: center;
}

.sourcing-pill-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
  color: #ffffff;
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  border: none;
  border-radius: 20px;
  cursor: pointer;
  box-shadow: 0 2px 6px rgba(37, 99, 235, 0.25);
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  white-space: nowrap;
}

.sourcing-pill-btn:hover {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  box-shadow: 0 4px 10px rgba(37, 99, 235, 0.35);
  transform: translateY(-1px);
}

.sourcing-pill-btn:active {
  transform: translateY(0);
}

.col-title {
  min-width: 200px;
}

.job-title-wrapper {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.job-name {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.job-subinfo {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #94a3b8;
}

.dot-sep {
  color: #cbd5e1;
}

.salary-text {
  color: #64748b;
  font-weight: 500;
}

.col-dept {
  min-width: 100px;
}

.dept-text {
  font-size: 14px;
  color: #334155;
}

.col-headcount {
  min-width: 80px;
}

.headcount-text {
  font-size: 14px;
  font-weight: 500;
  color: #334155;
}

.col-stats {
  min-width: 110px;
}

.stats-text {
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
}

.col-status {
  min-width: 120px;
}

/* 彩色圆点胶囊标签 */
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 14px;
  font-size: 12px;
  font-weight: 500;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

/* 招聘中 (绿色) */
.status-publishing {
  background-color: #dcfce7;
  color: #15803d;
}
.status-publishing .status-dot {
  background-color: #22c55e;
}

/* 草稿 (黄色) */
.status-draft {
  background-color: #fef3c7;
  color: #b45309;
}
.status-draft .status-dot {
  background-color: #f59e0b;
}

/* 已暂停 (灰色) */
.status-paused {
  background-color: #f1f5f9;
  color: #64748b;
}
.status-paused .status-dot {
  background-color: #94a3b8;
}

.col-time {
  min-width: 110px;
}

.time-text {
  font-size: 13px;
  color: #64748b;
}

.col-actions {
  min-width: 110px;
  text-align: right;
}

.action-btn-group {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.action-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  background: transparent;
  border-radius: 6px;
  color: #64748b;
  cursor: pointer;
  transition: all 0.15s ease;
}

.action-icon-btn:hover {
  background-color: #f1f5f9;
  color: #1e293b;
}

.action-icon-btn.highlight {
  color: #2563eb;
  background-color: #eff6ff;
}

.action-icon-btn.highlight:hover {
  background-color: #dbeafe;
  color: #1d4ed8;
}

.empty-row td {
  padding: 48px 20px;
  text-align: center;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  color: #94a3b8;
}

/* 表单与弹窗排版 */
.job-dialog-form {
  padding-top: 8px;
}

.job-description-label-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.direction-prompt-card {
  padding: 12px 14px;
  background: rgba(79, 70, 229, 0.05);
  border: 1px dashed rgba(79, 70, 229, 0.3);
  border-radius: 8px;
  margin-bottom: 12px;
}

.direction-prompt-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: #4f46e5;
  margin-bottom: 8px;
}

.direction-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}

.direction-custom-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.direction-badge {
  display: inline-block;
  margin-left: 8px;
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(79, 70, 229, 0.1);
  color: #4f46e5;
  font-size: 11px;
  font-weight: normal;
}

.ai-description-action {
  margin-left: 12px;
}

.description-help {
  margin-top: 5px;
  font-size: 12px;
  color: var(--wb-muted);
}

.skills-label-wrapper {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
  width: 100%;
}

.skills-label-title {
  font-weight: 500;
  color: var(--wb-text);
}

.skills-label-tip {
  font-size: 12px;
  font-weight: 500;
  color: #2563eb !important;
  line-height: 1.4;
}

.city-sync-suggestion {
  margin-top: 4px;
  font-size: 12px;
  color: #059669;
  display: flex;
  align-items: center;
  gap: 6px;
}

.ai-description-preview {
  margin: -2px 0 18px;
  padding: 14px;
  border: 1px solid var(--wb-border);
  border-radius: 10px;
  background: var(--wb-surface-soft);
}

.ai-preview-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--wb-text);
}

.ai-description-preview pre {
  margin: 10px 0 0;
  color: var(--wb-text);
  font: inherit;
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.form-row {
  display: flex;
  gap: 16px;
}

.two-cols > * {
  flex: 1;
}

.three-cols > * {
  flex: 1;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
