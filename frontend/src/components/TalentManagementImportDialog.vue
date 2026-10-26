<template>
  <el-dialog v-model="visible" title="51job 人才管理 · 职位接入与合规初筛" width="760px" :close-on-click-modal="!running">
    <el-tabs v-model="activeTab" style="margin-top: -10px">
      <!-- Tab 1: 职位导入与每小时自动同步 -->
      <el-tab-pane label="职位导入与自动调度" name="sync">
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
          <template #default>
            按平台职位 ID 绑定本地专属库。只分析已取得完整资料的简历；附件受限尚未取得时保留为“待补全”。
          </template>
        </el-alert>

        <el-alert v-if="!jobs.length && !loading" type="warning" :closable="false" show-icon style="margin-bottom: 16px"
          title="未检测到已登录的 51job 企业职位。请点击下方【打开 51job 浏览器】登录企业账号，登录完成后点击【重新读取职位】即可。" />

        <el-form label-position="top">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="51job 企业职位">
                <el-select v-model="jobId" filterable style="width: 100%" :disabled="running" placeholder="选择已登录企业的职位">
                  <el-option v-for="job in jobs" :key="job.jobId" :value="job.jobId" :label="formatJobOptionLabel(job)" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="对应本地职位专属库">
                <el-select v-model="projectId" filterable style="width: 100%" :disabled="running" placeholder="首次绑定请确认项目">
                  <el-option v-for="project in projects" :key="project.id" :value="project.id"
                    :label="`${project.job_config?.title || project.name} · ${project.name}`" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-alert v-if="selectedJob && selectedJob.degree === '大专' && strictCriteria.minEducation === '本科'"
            type="warning" :closable="false" show-icon style="margin-bottom: 16px"
            title="注意：当前 51job 职位的招聘要求为【大专】，但您在【岗位专属硬性条件】中设置了【本科及以上】。本次扫描的大专候选人将被拦截排除。若需录入大专候选人，请前往【岗位专属硬性条件】将最低学历调整为【大专及以上】。" />

          <el-form-item label="导入的平台状态">
            <el-checkbox-group v-model="statuses" :disabled="running">
              <el-checkbox :value="1">待处理</el-checkbox>
              <el-checkbox :value="2">合适</el-checkbox>
              <el-checkbox :value="3">面试</el-checkbox>
              <el-checkbox :value="4">待定</el-checkbox>
            </el-checkbox-group>
          </el-form-item>

          <!-- 断点状态看板 -->
          <div v-if="syncTask" style="background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; padding: 12px 16px; margin-bottom: 16px">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
              <span style="font-weight: 600; color: #1e293b; font-size: 13px">任务断点状态</span>
              <el-tag :type="syncTaskTagType" size="small">{{ syncTaskStatusLabel }}</el-tag>
            </div>
            <div style="font-size: 12px; color: #64748b; line-height: 1.8">
              <div>当前游标：{{ currentStatusName }} · 第 {{ syncTask.pageIndex }} 页</div>
              <div>累计导入：{{ syncTask.importedCandidates }} 人 / 平台已知候选人：{{ syncTask.totalCandidates || '待统计' }}</div>
              <div v-if="syncTask.lastSyncAt">上次检查：{{ syncTask.lastSyncAt }}</div>
              <div v-if="syncTask.lastError" style="color: #ef4444">异常提示：{{ syncTask.lastError }}</div>
            </div>
            <div style="margin-top: 10px; display: flex; align-items: center; justify-content: space-between; border-top: 1px dashed #cbd5e1; padding-top: 8px">
              <div style="display: flex; align-items: center; gap: 8px">
                <el-switch v-model="autoSyncEnabled" @change="toggleAutoSync" :disabled="!jobId || running" />
                <span style="font-size: 12px; color: #334155">软件运行且当前账号登录时，每小时自动检查新增名单</span>
              </div>
              <el-button link type="info" size="small" :disabled="running" @click="resetTask">重置断点</el-button>
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

        <el-alert v-if="!jobs.length && !loading" type="warning" :closable="false" show-icon style="margin-bottom: 16px"
          title="未检测到已登录的 51job 企业职位。请点击下方【打开 51job 浏览器】登录企业账号，登录完成后点击【重新读取职位】即可。" />

        <el-form label-position="top">
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

          <el-form-item style="margin-top: 24px">
            <div style="background: #fef2f2; border: 1px solid #fee2e2; border-radius: 6px; padding: 12px 16px; width: 100%">
              <el-checkbox v-model="strictCriteria.complianceConfirmed">
                <span style="font-weight: 600; color: #991b1b">HR 合规授权确认</span>
                <span style="font-size: 12px; color: #b91c1c; display: block; margin-top: 2px">
                  确认已人工核实本岗位的硬性门槛，授权仅对完整资料且证据确凿的「待处理」人选执行平台不合适标记。
                </span>
              </el-checkbox>
            </div>
          </el-form-item>

          <el-button type="primary" :disabled="!projectId" :loading="savingCriteria" @click="saveCriteria">
            保存岗位硬性条件
          </el-button>
        </el-form>
      </el-tab-pane>

      <!-- Tab 3: 淘汰审计日志 -->
      <el-tab-pane label="淘汰审计记录" name="audit">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
          <span style="font-size: 12px; color: #64748b">
            自动淘汰人选不进入常规人才库列表，全量留存身份、规则、证据与平台回执备查。
          </span>
          <el-button size="small" :loading="loadingAudit" @click="loadAuditLogs">刷新记录</el-button>
        </div>

        <el-table :data="auditLogs" size="small" max-height="360" empty-text="暂无淘汰审计记录">
          <el-table-column prop="candidateName" label="候选人" width="110" />
          <el-table-column prop="reasonLabel" label="淘汰原因" width="120">
            <template #default="{ row }">
              <el-tag type="danger" size="small">{{ row.reasonLabel }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="evidence" label="确凿证据说明" min-width="200" show-overflow-tooltip />
          <el-table-column prop="platformReceipt" label="平台回执" min-width="150" show-overflow-tooltip />
          <el-table-column prop="createdAt" label="执行时间" width="140" />
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <el-button type="warning" plain :disabled="running" @click="openBrowser">打开 51job 浏览器</el-button>
      <el-button :loading="loading" :disabled="running" @click="load">重新读取职位</el-button>
      <el-button v-if="running" @click="stopRequested = true">本页结束后暂停</el-button>
      <el-button v-if="!running && activeTab === 'sync'" :disabled="!jobId || !projectId" @click="runSingleBatch">
        执行单批 (50人)
      </el-button>
      <el-button type="primary" :loading="running" :disabled="!jobId || !projectId || !statuses.length"
        @click="start">连续导入并同步</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'

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
const statuses = ref<number[]>([1, 2, 3, 4])
const loading = ref(false)
const running = ref(false)
const stopRequested = ref(false)
const progress = ref('')
const summary = ref('')

const syncTask = ref<any>(null)
const autoSyncEnabled = ref(false)
const savingCriteria = ref(false)
const loadingAudit = ref(false)
const auditLogs = ref<any[]>([])

const strictCriteria = ref({
  projectId: '',
  minEducation: '',
  minWorkYears: 0,
  rejectNonLocalWillingness: false,
  targetCity: '',
  complianceConfirmed: false,
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
  loading.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const catalog = await api.Get51JobTalentManagementCatalog()
    mappings.value = await api.ListPlatformJobMappings() || []
    jobs.value = catalog.jobs || []
    accountScope.value = catalog.accountScope || ''
    if (!jobs.value.some(job => job.jobId === jobId.value)) {
      jobId.value = jobs.value[0]?.jobId || ''
    }
    syncMappedProject()
    await loadSyncTask()
    await loadCriteria()
    await loadAuditLogs()
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
  } finally {
    loading.value = false
  }
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
    if (c) strictCriteria.value = c
  } catch (e) {}
}

async function saveCriteria() {
  if (!projectId.value) return
  savingCriteria.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    strictCriteria.value.projectId = projectId.value
    await api.SetTalentManagementStrictCriteria(strictCriteria.value)
    ElMessage.success('岗位专属硬性条件已保存')
  } catch (e: any) {
    ElMessage.error(e?.message || '保存硬性条件失败')
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
