<template>
  <el-dialog
    v-model="visible"
    title="51job 人才管理 · 职位接入与合规初筛"
    width="760px"
    top="4vh"
    :close-on-click-modal="!running"
    :close-on-press-escape="!running"
  >
    <div class="dialog-scroll-body" style="max-height: 68vh; overflow-y: auto; overflow-x: hidden; padding-right: 8px;">
      <el-tabs v-model="activeTab" style="margin-top: -10px">
      <!-- Tab 1: 职位接入与初筛预览 -->
      <el-tab-pane label="职位接入与初筛预览" name="sync">
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
          <template #default>
            项目是条件的唯一归属。仅扫描平台「待处理」人选生成三类名单预览；HR 确认后，合格人选入库，不合格人选可批量标记「不感兴趣」，待核实人选受完整性保护不入库也不触发平台操作。
          </template>
        </el-alert>

        <el-alert v-if="!jobs.length && !loading" type="warning" :closable="false" show-icon style="margin-bottom: 16px"
          title="未检测到已登录的 51job 企业职位。请点击下方【打开 51job 浏览器】登录企业账号，登录完成后点击【重新读取职位】即可。" />
        <el-alert v-if="catalogStale && jobs.length" type="warning" :closable="false" show-icon style="margin-bottom: 16px"
          title="当前显示上次读取的职位，尚未核实当前登录账号；重新读取成功前不能导入或启动同步。" />
        <p v-if="catalogStatus" class="catalog-status" :class="{ stale: catalogStale }">{{ catalogStatus }}</p>

        <el-form label-position="top">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="51job 企业职位">
                <el-select v-model="jobId" filterable style="width: 100%" :disabled="running || scanning" placeholder="选择已登录企业的职位">
                  <el-option v-for="job in jobs" :key="job.jobId" :value="job.jobId" :label="formatJobOptionLabel(job)" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="对应本地职位专属库">
                <el-select v-model="projectId" filterable style="width: 100%" :disabled="running || scanning" placeholder="首次绑定请确认项目">
                  <el-option v-for="project in projects" :key="project.id" :value="project.id"
                    :label="`${project.job_config?.title || project.name} · ${project.name}`" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-alert v-if="selectedJob && selectedJob.degree === '大专' && strictCriteria.minEducation === '本科'"
            type="warning" :closable="false" show-icon style="margin-bottom: 16px"
            title="注意：当前 51job 职位的招聘要求为【大专】，但您在【岗位专属硬性条件】中设置了【本科及以上】。本次扫描的大专候选人将被拦截排除。若需录入大专候选人，请前往【岗位专属硬性条件】将最低学历调整为【大专及以上】。" />

          <div style="display: flex; gap: 12px; align-items: center; margin-bottom: 16px">
            <el-button type="primary" :loading="scanning" :disabled="!jobId || !projectId || catalogStale" @click="scanBatchPreview">
              <el-icon><Search /></el-icon> 扫描待处理人选并生成初筛预览 (单页50人)
            </el-button>
            <span style="font-size: 12px; color: #64748b">
              扫描仅收集名单与核验证据，绝不静默触碰或修改平台状态
            </span>
          </div>

          <!-- 初筛预览看板 -->
          <div v-if="batchPreview" class="screening-preview-card" style="border: 1px solid #e2e8f0; border-radius: 8px; padding: 14px; background: #fff; margin-bottom: 16px">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid #f1f5f9; padding-bottom: 8px">
              <div style="font-weight: 600; font-size: 14px; color: #1e293b">
                本轮初筛核验报告（条件版本：{{ batchPreview.criteriaVersion }} · 共检索 {{ batchPreview.totalCount }} 人）
              </div>
              <el-radio-group v-model="previewSubTab" size="small">
                <el-radio-button value="qualified">合格 ({{ batchPreview.qualifiedCount }})</el-radio-button>
                <el-radio-button value="unfit">不合格 ({{ batchPreview.unfitCount }})</el-radio-button>
                <el-radio-button value="pending">待核实 ({{ batchPreview.pendingCount }})</el-radio-button>
              </el-radio-group>
            </div>

            <!-- 子看板 1: 合格人选 -->
            <div v-if="previewSubTab === 'qualified'">
              <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
                <span style="font-size: 12px; color: #166534">各项启用条件均通过，可一键导入至当前职位专属分库</span>
                <el-button type="success" size="small" :loading="executingAction" :disabled="batchPreview.qualifiedCount === 0" @click="confirmImportQualified">
                  一键导入合格人选入库 ({{ batchPreview.qualifiedCount }})
                </el-button>
              </div>
              <el-table :data="batchPreview.qualifiedList" size="small" max-height="240" empty-text="当前批次无合格人选">
                <el-table-column prop="candidateName" label="姓名" width="100" />
                <el-table-column label="核验说明" min-width="260">
                  <template #default="{ row }">
                    <div v-for="item in row.ruleEvidence" :key="item.rule" style="font-size: 11px; color: #15803d">
                      ✔ {{ item.label }}：{{ item.evidence }}
                    </div>
                  </template>
                </el-table-column>
                <el-table-column prop="actionStatus" label="状态" width="90">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.actionStatus === 'imported' ? 'success' : 'info'">
                      {{ row.actionStatus === 'imported' ? '已入库' : '待导入' }}
                    </el-tag>
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <!-- 子看板 2: 不合格人选 -->
            <div v-if="previewSubTab === 'unfit'">
              <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
                <span style="font-size: 12px; color: #991b1b">有确凿不符合项，可批量联动 51job 平台标记「不感兴趣」</span>
                <el-button type="danger" size="small" :loading="executingAction" :disabled="!selectedUnfitCandidates.length" @click="confirmRejectUnfit">
                  批量标记不合适 (已选 {{ selectedUnfitCandidates.length }} 人)
                </el-button>
              </div>
              <el-table :data="batchPreview.unfitList" size="small" max-height="240" @selection-change="handleUnfitSelectionChange" empty-text="当前批次无不合格人选">
                <el-table-column type="selection" width="40" />
                <el-table-column prop="candidateName" label="姓名" width="100" />
                <el-table-column label="不合格确凿证据" min-width="260">
                  <template #default="{ row }">
                    <div v-for="item in row.ruleEvidence.filter((e: any) => e.status === 'fail')" :key="item.rule" style="font-size: 11px; color: #b91c1c; font-weight: 500">
                      ✖ {{ item.label }}：{{ item.evidence }}
                    </div>
                  </template>
                </el-table-column>
                <el-table-column prop="actionStatus" label="状态" width="90">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.actionStatus === 'rejected' ? 'danger' : 'warning'">
                      {{ row.actionStatus === 'rejected' ? '已标记' : '待处理' }}
                    </el-tag>
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <!-- 子看板 3: 待核实人选 -->
            <div v-if="previewSubTab === 'pending'">
              <div style="margin-bottom: 8px; font-size: 12px; color: #854d0e">
                资料缺失、活跃度未公开或关键字段未知。保护机制已生效：绝不自动入库，也绝不触发平台淘汰。
              </div>
              <el-table :data="batchPreview.pendingList" size="small" max-height="240" empty-text="当前批次无待核实人选">
                <el-table-column prop="candidateName" label="姓名" width="100" />
                <el-table-column label="待核实证据说明" min-width="260">
                  <template #default="{ row }">
                    <div v-for="item in row.ruleEvidence.filter((e: any) => e.status === 'unknown')" :key="item.rule" style="font-size: 11px; color: #d97706">
                      ？{{ item.label }}：{{ item.evidence }}
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="100">
                  <template #default="{ row }">
                    <span style="font-size: 12px; color: #94a3b8">保留待人工复核</span>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </el-form>

        <p v-if="progress" style="color: #606f89; font-size: 13px">{{ progress }}</p>
        <p v-if="summary" style="color: #0f766e; font-size: 13px; font-weight: 500">{{ summary }}</p>
      </el-tab-pane>

      <!-- Tab 2: 岗位专属硬性淘汰条件 -->
      <el-tab-pane label="岗位专属硬性条件" name="criteria">
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
          <template #default>
            仅在完整资料分析后，有可定位确凿证据违反硬性条件时才执行淘汰。信息缺失、矛盾或低分绝不自动拒绝，年龄与性别仅展示不用于淘汰。
          </template>
        </el-alert>

        <el-form label-position="top">
          <el-form-item label="当前配置针对的本地职位专属库">
            <el-select v-model="projectId" filterable style="width: 100%" placeholder="请选择需要配置硬性条件的职位专属库">
              <el-option v-for="project in projects" :key="project.id" :value="project.id"
                :label="`${project.job_config?.title || project.name} · ${project.name}`" />
            </el-select>
          </el-form-item>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="最低学历硬性要求">
                <el-select v-model="strictCriteria.minEducation" style="width: 100%">
                  <el-option label="不限" value="" />
                  <el-option label="大专及以上" value="大专" />
                  <el-option label="本科及以上" value="本科" />
                  <el-option label="硕士及以上" value="硕士" />
                  <el-option label="博士及以上" value="博士" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="最低工作年限要求（年）">
                <el-input-number v-model="strictCriteria.minWorkYears" :min="0" :max="40" :step="0.5" style="width: 100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="年龄区间要求（岁）">
                <div style="display: flex; align-items: center; gap: 8px">
                  <el-input-number v-model="strictCriteria.minAge" :min="0" :max="80" placeholder="最小" style="width: 48%" />
                  <span style="color: #94a3b8">至</span>
                  <el-input-number v-model="strictCriteria.maxAge" :min="0" :max="80" placeholder="最大" style="width: 48%" />
                </div>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="期望薪资上限（千元/月）">
                <el-input-number v-model="strictCriteria.maxExpectedSalary" :min="0" :max="200" placeholder="如 15 代表 15k/月，0 为不限" style="width: 100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="性别要求">
                <el-select v-model="strictCriteria.gender" style="width: 100%" placeholder="性别要求">
                  <el-option label="不限" value="" />
                  <el-option label="仅限男性" value="男" />
                  <el-option label="仅限女性" value="女" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="求职状态限制">
                <el-select v-model="strictCriteria.jobSeekingStatus" style="width: 100%" placeholder="求职状态">
                  <el-option label="不限" value="" />
                  <el-option label="仅离职（随时到岗）" value="离职" />
                  <el-option label="在职（考虑机会）" value="在职" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="人才活跃度要求">
                <el-select v-model="strictCriteria.activityDays" style="width: 100%">
                  <el-option label="不限活跃时间" :value="0" />
                  <el-option label="近 3 天活跃" :value="3" />
                  <el-option label="近 7 天活跃（推荐）" :value="7" />
                  <el-option label="近 30 天活跃" :value="30" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="条件版本标识">
                <el-input :model-value="strictCriteria.criteriaVersion || 'v1'" disabled placeholder="保存后自动递增版本号" style="width: 100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="项目专属推介文案（平台打招呼后发送）">
            <el-input
              v-model="strictCriteria.projectIntroMessage"
              type="textarea"
              :rows="3"
              placeholder="如：您好！关注到您的经历与我们目前急聘的【研发专家】岗位非常契合，欢迎了解职位详情并投递沟通！"
              clearable
            />
          </el-form-item>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="每日定时计划时间">
                <el-input v-model="strictCriteria.scheduleTime" placeholder="例如 09:30" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="定时自动初筛状态" style="display: flex; align-items: center; margin-top: 28px">
                <el-switch v-model="strictCriteria.scheduleEnabled" active-text="启用每日定时初筛" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="专业背景关键词匹配（满足其一即可通过）">
            <el-input
              v-model="strictCriteria.majorKeywords"
              placeholder="如：医学, 药学, 市场营销, 生物制药（逗号分隔，留空表示不限专业）"
              clearable
            />
          </el-form-item>

          <el-form-item label="工作城市意向限制">
            <div style="display: flex; align-items: center; gap: 12px; width: 100%">
              <el-checkbox v-model="strictCriteria.rejectNonLocalWillingness">
                排除明确表示不愿在目标城市工作的候选人
              </el-checkbox>
              <el-input v-model="strictCriteria.targetCity" placeholder="目标城市（如：上海）" style="width: 160px"
                :disabled="!strictCriteria.rejectNonLocalWillingness" />
            </div>
            <div style="font-size: 12px; color: #94a3b8; margin-top: 4px">
              仅当候选人求职意向明确排除目标城市时触发；异地现居地但有意向者不触发。
            </div>
          </el-form-item>

          <el-form-item>
            <div style="background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; padding: 12px 16px; width: 100%">
              <el-checkbox v-model="strictCriteria.autoRejectPlatform">
                <span style="font-weight: 500; color: #1e293b">联动 51job 网页端自动标记【不合适】</span>
                <span style="font-size: 12px; color: #64748b; display: block; margin-top: 2px">
                  对来源于 51job 的候选人，在识别为不符合硬性条件时，自动在 51job 企业后台点击标记不合适并提交淘汰理由。
                </span>
              </el-checkbox>
            </div>
          </el-form-item>

          <el-form-item style="margin-top: 16px">
            <div style="background: #fef2f2; border: 1px solid #fee2e2; border-radius: 6px; padding: 12px 16px; width: 100%">
              <el-checkbox v-model="strictCriteria.complianceConfirmed">
                <span style="font-weight: 600; color: #991b1b">HR 合规授权确认</span>
                <span style="font-size: 12px; color: #b91c1c; display: block; margin-top: 2px">
                  确认已人工核实本岗位的硬性门槛，授权仅对完整资料且证据确凿的「待处理」人选执行平台不合适标记。
                </span>
              </el-checkbox>
            </div>
          </el-form-item>

          <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 16px; border-top: 1px dashed #cbd5e1; padding-top: 12px">
            <div style="display: flex; gap: 12px; align-items: center">
              <el-button type="primary" :disabled="!projectId" :loading="savingCriteria" @click="saveCriteria">
                保存岗位硬性条件
              </el-button>
              <span v-if="!projectId" style="font-size: 12px; color: #f59e0b">请先选择上方职位专属库后保存</span>
              <span v-if="saveStatus" class="catalog-status">{{ saveStatus }}</span>
            </div>
            <el-button type="warning" plain :disabled="!projectId" :loading="rescreening" @click="rescreenCurrentProject">
              一键旧库全量重筛
            </el-button>
          </div>
        </el-form>
      </el-tab-pane>

      <!-- Tab 3: 平台动作台账与审计 -->
      <el-tab-pane label="平台动作台账与审计" name="audit">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
          <span style="font-size: 12px; color: #64748b">
            留存逐人身份、判定规则、执行动作、平台回执与失败原因，支持审计追溯。
          </span>
          <div style="display: flex; gap: 8px">
            <el-button size="small" type="danger" plain :disabled="!auditLogs.length" @click="confirmClearAuditLogs">清空审计记录</el-button>
            <el-button size="small" :loading="loadingAudit || loadingLedger" @click="refreshAuditAndLedger">刷新记录</el-button>
          </div>
        </div>

        <el-tabs type="border-card">
          <el-tab-pane label="平台动作台账">
            <el-table :data="actionLedgerLogs" size="small" max-height="300" empty-text="暂无平台动作台账记录">
              <el-table-column prop="candidateName" label="候选人" width="100" />
              <el-table-column prop="actionType" label="动作" width="90">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.actionType === 'reject' ? 'danger' : (row.actionType === 'import' ? 'success' : 'primary')">
                    {{ row.actionType === 'reject' ? '标记不合适' : (row.actionType === 'import' ? '专属库入库' : row.actionType) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="80">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.status === 'success' ? 'success' : 'danger'">
                    {{ row.status === 'success' ? '成功' : '失败' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="receipt" label="平台回执 / 结果" min-width="200" show-overflow-tooltip />
              <el-table-column prop="requestedAt" label="执行时间" width="140" />
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="淘汰审计记录">
            <el-table :data="auditLogs" size="small" max-height="300" empty-text="暂无淘汰审计记录">
              <el-table-column prop="candidateName" label="候选人" width="100" />
              <el-table-column prop="reasonLabel" label="淘汰原因" width="110">
                <template #default="{ row }">
                  <el-tag type="danger" size="small">{{ row.reasonLabel }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="evidence" label="确凿证据说明" min-width="190" show-overflow-tooltip />
              <el-table-column prop="platformReceipt" label="平台回执" min-width="140" show-overflow-tooltip />
              <el-table-column prop="createdAt" label="执行时间" width="140" />
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </el-tab-pane>
    </el-tabs>
    </div>

    <template #footer>
      <div style="display: flex; justify-content: space-between; align-items: center; width: 100%">
        <div style="display: flex; gap: 8px">
          <el-button type="warning" plain :disabled="running || scanning || executingAction" @click="openBrowser">打开 51job 浏览器</el-button>
          <el-button :loading="loading" :disabled="running || scanning || executingAction" @click="load">重新读取职位</el-button>
        </div>
        <div style="display: flex; gap: 8px">
          <el-button :disabled="running || scanning || executingAction" @click="visible = false">关闭</el-button>
          <el-button v-if="activeTab === 'sync'" type="primary" :loading="scanning" :disabled="!jobId || !projectId || catalogStale"
            @click="scanBatchPreview">扫描生成初筛预览 (50人)</el-button>
          <el-button v-if="activeTab === 'criteria'" type="primary" :disabled="!projectId" :loading="savingCriteria"
            @click="saveCriteria">保存硬性条件</el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'

const props = defineProps<{ modelValue: boolean; projects: any[]; defaultProjectId?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; completed: [] }>()
const visible = computed({ get: () => props.modelValue, set: value => emit('update:modelValue', value) })

const activeTab = ref('sync')
const jobs = ref<Array<{ jobId: string; title: string; workArea?: string; degree?: string; workYear?: string; salary?: string }>>([])
const mappings = ref<any[]>([])
const accountScope = ref('')
const jobId = ref('')
const projectId = ref('')
const selectedJob = computed(() => jobs.value.find(j => j.jobId === jobId.value))
const statuses = ref<number[]>([1])
const loading = ref(false)
const catalogStale = ref(true)
const catalogStatus = ref('')
const saveStatus = ref('')
let loadGeneration = 0
const running = ref(false)
const stopRequested = ref(false)
const progress = ref('')
const summary = ref('')

const syncTask = ref<any>(null)
const autoSyncEnabled = ref(false)
const savingCriteria = ref(false)
const loadingAudit = ref(false)
const auditLogs = ref<any[]>([])

// 51job 共用初筛框架状态
const batchPreview = ref<any>(null)
const scanning = ref(false)
const previewSubTab = ref<'qualified' | 'unfit' | 'pending'>('qualified')
const selectedUnfitCandidates = ref<string[]>([])
const executingAction = ref(false)
const actionLedgerLogs = ref<any[]>([])
const loadingLedger = ref(false)
const rescreening = ref(false)

const strictCriteria = ref({
  projectId: '',
  minEducation: '',
  minWorkYears: 0,
  minAge: 0,
  maxAge: 0,
  maxExpectedSalary: 0,
  gender: '',
  jobSeekingStatus: '',
  majorKeywords: '',
  rejectNonLocalWillingness: false,
  targetCity: '',
  activityDays: 7,
  criteriaVersion: 'v1',
  projectIntroMessage: '',
  scheduleTime: '09:30',
  scheduleEnabled: false,
  complianceConfirmed: false,
  autoRejectPlatform: true,
  updatedAt: ''
})

const currentStatusName = computed(() => {
  const map: Record<number, string> = { 1: '待处理', 2: '合适', 3: '面试', 4: '待定' }
  return map[syncTask.value?.statusIndex || 1] || '待处理'
})

const syncTaskStatusLabel = computed(() => {
  const map: Record<string, string> = {
    idle: '空闲待命',
    running: '同步中',
    paused: '已暂停',
    completed: '已完成一轮检查',
    error: '异常退避'
  }
  return map[syncTask.value?.state || 'idle'] || '空闲'
})

const syncTaskTagType = computed(() => {
  const map: Record<string, any> = {
    idle: 'info',
    running: 'primary',
    paused: 'warning',
    completed: 'success',
    error: 'danger'
  }
  return map[syncTask.value?.state || 'idle'] || 'info'
})

async function load() {
  const generation = ++loadGeneration
  loading.value = true
  catalogStale.value = true
  catalogStatus.value = '正在核验 51job 登录账号和职位…'
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const fresh = await withTimeout((async () => {
      const catalog = await api.Get51JobTalentManagementCatalog()
      const nextMappings = await api.ListPlatformJobMappings() || []
      const nextJobs = catalog.jobs || []
      const nextJobID = nextJobs.some((job: any) => job.jobId === jobId.value) ? jobId.value : (nextJobs[0]?.jobId || '')
      const mapping = nextMappings.find((item: any) => item.platform === '51job' &&
        item.platformAccount === catalog.accountScope && item.platformJobId === nextJobID)
      const nextProjectID = mapping?.projectId || props.defaultProjectId || ''
      const [task, criteria, audit] = await Promise.all([
        nextJobID ? api.GetTalentManagementSyncTask('51job', catalog.accountScope, nextJobID) : Promise.resolve(null),
        nextProjectID ? api.GetTalentManagementStrictCriteria(nextProjectID) : Promise.resolve(null),
        nextProjectID ? api.ListTalentManagementAuditLogs(nextProjectID) : Promise.resolve([])
      ])
      return { catalog, nextMappings, nextJobs, nextJobID, nextProjectID, task, criteria, audit }
    })(), 25000, '重新读取职位')
    if (generation !== loadGeneration) return
    mappings.value = fresh.nextMappings
    jobs.value = fresh.nextJobs
    accountScope.value = fresh.catalog.accountScope || ''
    jobId.value = fresh.nextJobID
    projectId.value = fresh.nextProjectID
    syncTask.value = fresh.task
    autoSyncEnabled.value = Boolean(fresh.task?.autoSyncEnabled)
    if (fresh.criteria) applyCriteria(fresh.criteria, fresh.nextProjectID)
    auditLogs.value = fresh.audit || []
    catalogStale.value = false
    catalogStatus.value = `已核验 ${jobs.value.length} 个在线职位 · ${new Date().toLocaleTimeString('zh-CN', { hour12: false })}`
    ElMessage.success(`成功读取 ${jobs.value.length} 个 51job 在线职位`)
  } catch (error: any) {
    if (generation !== loadGeneration) return
    jobs.value = []
    jobId.value = ''
    catalogStale.value = true
    catalogStatus.value = classifyCatalogError(error)
    ElMessage.error(catalogStatus.value)
  } finally {
    if (generation === loadGeneration) loading.value = false
  }
}

function withTimeout<T>(promise: Promise<T>, timeoutMS: number, action: string): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => reject(new Error(`${action}超过 ${Math.round(timeoutMS / 1000)} 秒，请检查浏览器登录或本地数据服务`)), timeoutMS)
    promise.then(value => { window.clearTimeout(timer); resolve(value) }, error => { window.clearTimeout(timer); reject(error) })
  })
}

function classifyCatalogError(error: any): string {
  const message = String(error?.message || error || '')
  if (/超时|timeout|超过/.test(message)) return `读取超时：${message}`
  if (/登录|账号|企业身份/.test(message)) return `51job 登录状态待核实：${message}`
  if (/浏览器|9503|连接/.test(message)) return `51job 专用浏览器未就绪：${message}`
  if (/database|sqlite|locked|busy|数据库/.test(message)) return `本地数据库繁忙：${message}`
  return `职位读取失败：${message || '平台页面尚未加载完成'}`
}

function syncMappedProject() {
  const mapped = mappings.value.find(item => item.platform === '51job' &&
    item.platformAccount === accountScope.value && item.platformJobId === jobId.value)
  projectId.value = mapped?.projectId || props.defaultProjectId || ''
}

function formatJobOptionLabel(job: any) {
  if (!job) return ''
  const loc = job.workArea ? ` · ${job.workArea}` : ''
  const details = [job.degree, job.workYear, job.salary].filter(Boolean).join(' | ')
  const extra = details ? ` | ${details}` : ''
  return `${job.title}${loc} (ID: ${job.jobId}${extra})`
}

async function openBrowser() {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.Open51JobManagementBrowser()
    ElMessage.success('已为您呼出 51job 专用浏览器，请在浏览器中登录企业账号')
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
  }
}

async function loadSyncTask() {
  if (!accountScope.value || !jobId.value) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const task = await api.GetTalentManagementSyncTask('51job', accountScope.value, jobId.value)
    syncTask.value = task
    autoSyncEnabled.value = Boolean(task?.autoSyncEnabled)
  } catch (e) {
    syncTask.value = null
  }
}

async function toggleAutoSync(val: boolean) {
  if (!accountScope.value || !jobId.value) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.SetTalentManagementAutoSync('51job', accountScope.value, jobId.value, val)
    ElMessage.success(val ? '已开启每小时自动检查' : '已关闭自动检查')
  } catch (e: any) {
    autoSyncEnabled.value = !val
    ElMessage.error(e?.message || '设置自动同步失败')
  }
}

async function resetTask() {
  if (!accountScope.value || !jobId.value) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.ResetTalentManagementSyncTask('51job', accountScope.value, jobId.value)
    await loadSyncTask()
    ElMessage.success('断点已重置至第 1 状态第 1 页')
  } catch (e: any) {
    ElMessage.error(e?.message || '重置失败')
  }
}

async function loadCriteria() {
  if (!projectId.value) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const c = await api.GetTalentManagementStrictCriteria(projectId.value)
    if (c) applyCriteria(c, projectId.value)
  } catch (e) {}
}

function applyCriteria(c: any, selectedProjectID: string) {
  strictCriteria.value = {
    projectId: selectedProjectID,
    minEducation: c.minEducation || '',
    minWorkYears: c.minWorkYears || 0,
    minAge: c.minAge || 0,
    maxAge: c.maxAge || 0,
    maxExpectedSalary: c.maxExpectedSalary || 0,
    gender: c.gender || '',
    jobSeekingStatus: c.jobSeekingStatus || '',
    majorKeywords: c.majorKeywords || '',
    rejectNonLocalWillingness: Boolean(c.rejectNonLocalWillingness),
    targetCity: c.targetCity || '',
    activityDays: c.activityDays !== undefined ? c.activityDays : 7,
    criteriaVersion: c.criteriaVersion || 'v1',
    projectIntroMessage: c.projectIntroMessage || '',
    scheduleTime: c.scheduleTime || '09:30',
    scheduleEnabled: Boolean(c.scheduleEnabled),
    complianceConfirmed: Boolean(c.complianceConfirmed),
    autoRejectPlatform: c.autoRejectPlatform !== false,
    updatedAt: c.updatedAt || ''
  }
}

function handleUnfitSelectionChange(selection: any[]) {
  selectedUnfitCandidates.value = selection.map(item => item.candidateId)
}

async function scanBatchPreview() {
  if (!jobId.value || !projectId.value || !accountScope.value) return
  scanning.value = true
  summary.value = ''
  progress.value = '正在扫描平台待处理人选并执行三态合规核验...'
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.Bind51JobTalentManagementJob(accountScope.value, jobId.value, projectId.value)
    const preview = await api.Scan51JobTalentManagementBatch(accountScope.value, jobId.value, projectId.value, 1)
    batchPreview.value = preview
    selectedUnfitCandidates.value = (preview.unfitList || []).map((c: any) => c.candidateId)
    progress.value = `扫描完成：合格 ${preview.qualifiedCount} 人，不合格 ${preview.unfitCount} 人，待核实 ${preview.pendingCount} 人`
    ElMessage.success(`成功生成初筛预览：共核验 ${preview.totalCount} 位候选人`)
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
    progress.value = '扫描生成预览失败'
  } finally {
    scanning.value = false
  }
}

async function confirmImportQualified() {
  if (!batchPreview.value?.batchId) return
  executingAction.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const count = await api.ConfirmScreeningImportQualified(batchPreview.value.batchId)
    ElMessage.success(`成功将 ${count} 位合格人选导入本地专属分库！`)
    batchPreview.value = await api.GetScreeningBatchPreview(batchPreview.value.batchId)
    emit('completed')
    void loadActionLedger()
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    executingAction.value = false
  }
}

async function confirmRejectUnfit() {
  if (!batchPreview.value?.batchId || !selectedUnfitCandidates.value.length) return
  try {
    await ElMessageBox.confirm(`确定要对已选的 ${selectedUnfitCandidates.value.length} 位不符合硬性条件的候选人在 51job 平台批量标记「不合适」吗？此操作将提交平台并留存审计日志。`, '平台操作确认', {
      confirmButtonText: '确认标记不合适',
      cancelButtonText: '取消',
      type: 'warning'
    })
    executingAction.value = true
    const api: any = await import('../../wailsjs/go/main/App')
    const ledgers = await api.ConfirmScreeningRejectUnfit(batchPreview.value.batchId, selectedUnfitCandidates.value)
    const successCount = (ledgers || []).filter((l: any) => l.status === 'success').length
    ElMessage.success(`已完成平台标记：成功 ${successCount} 人`)
    batchPreview.value = await api.GetScreeningBatchPreview(batchPreview.value.batchId)
    void loadAuditLogs()
    void loadActionLedger()
    emit('completed')
  } catch (err: any) {
    if (err !== 'cancel') ElMessage.error(err?.message || String(err))
  } finally {
    executingAction.value = false
  }
}

async function rescreenCurrentProject() {
  if (!projectId.value) return
  try {
    await ElMessageBox.confirm('一键旧库全量重筛将使用当前保存的岗位硬性条件，对当前专属库全部已入库人选重新进行合规排查。不合格人选将被移至待处理/归档区并留存审计日志，不会自动联动平台执行拒绝。确定继续吗？', '旧库重筛确认', {
      confirmButtonText: '确定重筛',
      cancelButtonText: '取消',
      type: 'warning'
    })
    rescreening.value = true
    const api: any = await import('../../wailsjs/go/main/App')
    const res = await api.RescreenProjectRepository(projectId.value)
    ElMessage.success(`旧库重筛完成：共核验 ${res.totalChecked} 人，排除不合规 ${res.unfitCount} 人，保留合格 ${res.activeRemainingCount} 人`)
    void loadAuditLogs()
    emit('completed')
  } catch (err: any) {
    if (err !== 'cancel') ElMessage.error(err?.message || String(err))
  } finally {
    rescreening.value = false
  }
}

async function loadActionLedger() {
  if (!projectId.value) return
  loadingLedger.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    actionLedgerLogs.value = (await api.ListPlatformActionLedger(projectId.value)) || []
  } catch (e) {
    actionLedgerLogs.value = []
  } finally {
    loadingLedger.value = false
  }
}

function refreshAuditAndLedger() {
  void loadAuditLogs()
  void loadActionLedger()
}

function criteriaMatches(saved: any, requested: any): boolean {
  return ['minEducation', 'minWorkYears', 'minAge', 'maxAge', 'maxExpectedSalary', 'gender',
    'jobSeekingStatus', 'majorKeywords', 'rejectNonLocalWillingness', 'targetCity',
    'complianceConfirmed', 'autoRejectPlatform'].every(key =>
      String(saved?.[key] ?? '') === String(requested?.[key] ?? ''))
}

async function confirmClearAuditLogs() {
  if (!projectId.value) return
  try {
    await ElMessageBox.confirm('确定要清空本岗位的全部淘汰审计记录吗？清空后记录不可恢复。', '清空确认', {
      confirmButtonText: '确定清空',
      cancelButtonText: '取消',
      type: 'warning'
    })
    const api: any = await import('../../wailsjs/go/main/App')
    await api.ClearTalentManagementAuditLogs(projectId.value)
    ElMessage.success('已清空本岗位的淘汰审计记录')
    await loadAuditLogs()
  } catch (e) {}
}

async function saveCriteria() {
  if (!projectId.value) {
    ElMessage.warning('请先在上方选择需要保存硬性条件的本地职位专属库')
    return
  }
  savingCriteria.value = true
  saveStatus.value = '正在保存…'
  const requested = { ...strictCriteria.value, projectId: projectId.value }
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await withTimeout(api.SetTalentManagementStrictCriteria(requested), 10000, '保存硬性条件')
    const saved = await withTimeout(api.GetTalentManagementStrictCriteria(projectId.value), 5000, '回读硬性条件')
    if (saved) {
      applyCriteria(saved, projectId.value)
      saveStatus.value = `已保存 · ${saved.updatedAt || new Date().toLocaleString('zh-CN')}`
      ElMessage.success('岗位专属硬性条件已成功保存并同步')
    }
  } catch (e: any) {
    saveStatus.value = `保存异常：${e?.message || '本地数据服务未响应'}`
    ElMessage.error(saveStatus.value)
  } finally {
    savingCriteria.value = false
  }
}

async function loadAuditLogs() {
  if (!projectId.value) return
  loadingAudit.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    auditLogs.value = (await api.ListTalentManagementAuditLogs(projectId.value)) || []
  } catch (e) {
    auditLogs.value = []
  } finally {
    loadingAudit.value = false
  }
}

watch(jobId, () => {
  syncMappedProject()
  void loadSyncTask()
})

watch(projectId, () => {
  void loadCriteria()
  void loadAuditLogs()
  void loadActionLedger()
})

watch(() => props.modelValue, value => {
  if (value) void load()
})

async function runSingleBatch() {
  if (!jobId.value || !projectId.value || !accountScope.value) return
  running.value = true
  summary.value = ''
  progress.value = '正在执行单批断点导入...'
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.Bind51JobTalentManagementJob(accountScope.value, jobId.value, projectId.value)
    const result = await api.RunTalentManagementNextBatch(accountScope.value, jobId.value)
    await loadSyncTask()
    if (result.interceptedCount > 0) {
      summary.value = `单批完成：新增 ${result.addedCount || 0} · 拦截排除 ${result.interceptedCount} 人（可在【淘汰审计记录】查看） · 重复 ${result.duplicateCount || 0}`
      if (!result.addedCount) {
        ElMessage.warning(`本次扫描的候选人均不符合岗位硬性要求（${result.interceptReason || '学历不达标'}），已自动拦截并记录在【淘汰审计记录】`)
      }
    } else {
      summary.value = `单批完成：新增 ${result.addedCount || 0} · 待补全 ${result.pendingDocumentCount || 0} · 重复 ${result.duplicateCount || 0}`
    }
    progress.value = result.hasMore ? '本状态仍有下一页，可继续同步' : '本状态已无更多，已自动推进游标'
    emit('completed')
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
    progress.value = '单批导入出错'
  } finally {
    running.value = false
  }
}

async function start() {
  if (!jobId.value || !projectId.value || !accountScope.value || !statuses.value.length) return
  running.value = true
  stopRequested.value = false
  summary.value = ''
  const totals = { added: 0, duplicate: 0, pending: 0, failed: 0, intercepted: 0 }
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.Bind51JobTalentManagementJob(accountScope.value, jobId.value, projectId.value)
    for (const status of [...statuses.value]) {
      let page = 1
      while (!stopRequested.value) {
        progress.value = `正在导入 ${['', '待处理', '合适', '面试', '待定'][status]} · 第 ${page} 页`
        const result = await api.Import51JobTalentManagementPage(accountScope.value, jobId.value, status, page)
        totals.added += result.addedCount || 0
        totals.duplicate += result.duplicateCount || 0
        totals.pending += result.pendingDocumentCount || 0
        totals.failed += result.failedCount || 0
        totals.intercepted += result.interceptedCount || 0
        emit('completed')
        if (!result.hasMore) break
        page++
      }
      if (stopRequested.value) break
    }
    await loadSyncTask()
    if (totals.intercepted > 0) {
      summary.value = `新增 ${totals.added} · 拦截排除 ${totals.intercepted} · 重复 ${totals.duplicate} · 待补全 ${totals.pending}`
      if (totals.added === 0) {
        ElMessage.warning(`本次扫描的候选人均不符合岗位硬性要求，已全部拦截并存入【淘汰审计记录】中`)
      }
    } else {
      summary.value = `新增 ${totals.added} · 重复 ${totals.duplicate} · 待补全 ${totals.pending} · 失败 ${totals.failed}`
    }
    progress.value = stopRequested.value ? '已暂停，可再次点击继续同步' : '本次导入完成'
  } catch (error: any) {
    progress.value = '导入暂停'
    ElMessage.error(error?.message || String(error))
  } finally {
    running.value = false
    emit('completed')
  }
}
</script>
