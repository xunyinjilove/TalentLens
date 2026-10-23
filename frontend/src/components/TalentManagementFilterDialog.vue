<template>
  <el-dialog
    v-model="visible"
    title="🎯 前程无忧 (51job)「人才管理」自动筛选与不合适淘汰"
    width="860px"
    :close-on-click-modal="!running"
    :close-on-press-escape="!running"
    destroy-on-close
    @close="handleClose"
  >
    <div class="tm-filter-dialog">
      <!-- 顶部关联职位与安全准则说明 -->
      <div class="tm-context-banner">
        <div class="tm-context-row">
          <div class="tm-context-left">
            <span class="tm-label">目标招聘项目：</span>
            <strong class="tm-project-name">{{ projectName || '当前招聘项目' }}</strong>
            <el-tag size="small" type="primary" effect="plain" style="margin-left: 8px;">
              51job 待处理投递池
            </el-tag>
          </div>
          <div class="tm-context-right">
            <el-button size="small" link type="primary" @click="loadHistoryRuns">
              <el-icon><Clock /></el-icon>
              <span>{{ showHistory ? '返回筛选配置' : '历史审计记录' }}</span>
            </el-button>
          </div>
        </div>
        <div class="tm-safety-notice">
          🛡️ <strong>零误伤安全准则：</strong>系统<strong>只自动点击「不合适」并勾选原因，绝不自动点「合适」</strong>。资料未披露字段一律按「未知不误筛」保留在待处理；符合条件与资料未知的候选人将在执行后自动导入当前职位待 AI 深度初筛。
        </div>
      </div>

      <!-- 历史筛选审计记录面板 -->
      <div v-if="showHistory" class="tm-history-panel">
        <div class="tm-section-title">最近筛选与淘汰审计批次</div>
        <div v-if="historyRuns.length === 0" class="tm-empty-box">暂无历史筛选记录</div>
        <div v-else class="tm-history-list">
          <div v-for="h in historyRuns" :key="h.id" class="tm-history-card">
            <div class="tm-history-main">
              <span class="tm-mode-badge" :class="h.mode">{{ h.mode === 'execute' ? '正式执行' : '只读试算' }}</span>
              <strong>{{ h.jobTitle }}</strong>
              <span class="tm-history-stats">
                扫描 {{ h.summary?.totalScanned || 0 }} 人 ·
                淘汰 <b class="c-danger">{{ h.summary?.unfitCount || 0 }}</b> 人 ·
                保留导入 <b class="c-success">{{ (h.summary?.keepMatchedCount || 0) + (h.summary?.keepUnknownCount || 0) }}</b> 人
              </span>
              <span class="tm-history-time">{{ formatTime(h.createdAt) }}</span>
            </div>
            <div class="tm-history-actions">
              <el-button size="small" @click="viewRunDetail(h.id)">查看明细</el-button>
              <el-button size="small" type="primary" plain @click="exportRunExcel(h.id)">导出 Excel</el-button>
            </div>
          </div>
        </div>
      </div>

      <!-- 主工作区：1. 临时筛选条件表单 -->
      <div v-else>
        <div class="tm-filter-form-card">
          <div class="tm-form-header">
            <span class="tm-section-title">本次临时硬性筛选条件（留空或 0 表示该项不限）</span>
            <el-button size="small" link @click="resetForm">重置条件</el-button>
          </div>

          <div class="tm-grid-row two-cols">
            <div class="tm-field">
              <label>51job 对应职位名称（用于核对顶部职位 Tab）</label>
              <el-input v-model="form.jobTitle" placeholder="如：AI赋能开发程序员" :disabled="running" />
            </div>
            <div class="tm-field">
              <label>单次扫描上限</label>
              <el-select v-model="form.maxCandidates" style="width: 100%" :disabled="running">
                <el-option :value="50" label="最多处理前 50 位待处理候选人" />
                <el-option :value="100" label="最多处理前 100 位待处理候选人" />
                <el-option :value="200" label="最多处理前 200 位待处理候选人（推荐）" />
              </el-select>
            </div>
          </div>

          <div class="tm-grid-row three-cols">
            <div class="tm-field">
              <label>最低学历要求</label>
              <el-select v-model="form.minEducation" clearable placeholder="不限学历" style="width: 100%" :disabled="running">
                <el-option label="不限学历" value="" />
                <el-option label="大专及以上" value="大专" />
                <el-option label="本科及以上" value="本科" />
                <el-option label="硕士及以上" value="硕士" />
                <el-option label="博士" value="博士" />
              </el-select>
            </div>

            <div class="tm-field">
              <label>最低工作年限（年）</label>
              <el-input-number
                v-model="form.minWorkYears"
                :min="0"
                :max="20"
                :step="1"
                style="width: 100%"
                :disabled="running"
              />
            </div>

            <div class="tm-field tm-switch-field">
              <label>排除在校生 / 未毕业人选</label>
              <div class="tm-switch-box">
                <el-switch v-model="form.excludeStudents" :disabled="running" />
                <span class="tm-switch-hint">识别“在校生”或毕业时间晚于当前月</span>
              </div>
            </div>
          </div>

          <div class="tm-grid-row three-cols">
            <div class="tm-field">
              <label>允许的现居地城市（多个用逗号分隔）</label>
              <el-input
                v-model="form.allowedResidences"
                clearable
                placeholder="如：上海, 苏州（留空不限）"
                :disabled="running"
              />
            </div>

            <div class="tm-field">
              <label>允许的求职意向城市</label>
              <el-input
                v-model="form.allowedIntentCities"
                clearable
                placeholder="如：上海（留空不限）"
                :disabled="running"
              />
            </div>

            <div class="tm-field">
              <label>期望月薪上限（K/月，最低期望超此值才淘汰）</label>
              <el-input-number
                v-model="form.maxSalaryK"
                :min="0"
                :max="200"
                :step="1"
                style="width: 100%"
                placeholder="0 表示不限"
                :disabled="running"
              />
            </div>
          </div>

          <div class="tm-grid-row one-col">
            <div class="tm-field">
              <label>允许的所学专业关键词（任一匹配即通过，留空不限）</label>
              <el-input
                v-model="form.allowedMajors"
                clearable
                placeholder="如：计算机, 软件, 数字媒体, 电子, 信息, 自动化, 数学（留空不限专业）"
                :disabled="running"
              />
            </div>
          </div>

          <!-- 合规受控项：年龄与性别 -->
          <div class="tm-compliance-box">
            <div class="tm-compliance-toggles">
              <el-checkbox v-model="form.enableAgeFilter" :disabled="running">启用年龄区间筛选（受控项）</el-checkbox>
              <el-checkbox v-model="form.enableGenderFilter" :disabled="running">启用性别筛选（受控项）</el-checkbox>
            </div>

            <div v-if="form.enableAgeFilter || form.enableGenderFilter" class="tm-compliance-inner">
              <div class="tm-grid-row two-cols" style="margin-top: 8px;">
                <div v-if="form.enableAgeFilter" class="tm-field">
                  <label>允许年龄区间（岁，0 表示不设限）</label>
                  <div class="tm-age-range">
                    <el-input-number v-model="form.minAge" :min="0" :max="65" placeholder="最小" :disabled="running" />
                    <span class="tm-range-sep">至</span>
                    <el-input-number v-model="form.maxAge" :min="0" :max="65" placeholder="最大" :disabled="running" />
                  </div>
                </div>

                <div v-if="form.enableGenderFilter" class="tm-field">
                  <label>性别要求（根据头像角标 ♂/♀ 或称谓识别）</label>
                  <el-select v-model="form.allowedGender" placeholder="请选择" style="width: 100%" :disabled="running">
                    <el-option label="男" value="男" />
                    <el-option label="女" value="女" />
                  </el-select>
                </div>
              </div>

              <div class="tm-compliance-confirm">
                <el-checkbox v-model="form.complianceConfirmed" :disabled="running">
                  <span class="c-warning">
                    我已确认该岗位对年龄/性别存在合法特殊职务要求（命中时在 51job 对外统一勾选「其他原因」，仅在本地审计表记录内部规则）
                  </span>
                </el-checkbox>
              </div>
            </div>
          </div>
        </div>

        <!-- 实时执行进度条 -->
        <div v-if="running" class="tm-progress-banner">
          <div class="tm-progress-top">
            <el-icon class="is-loading"><Loading /></el-icon>
            <strong>{{ runningMode === 'execute' ? '正在 51job 全自动执行筛选与勾选不合适原因…' : '正在只读扫描 51job 人才管理待处理卡片…' }}</strong>
            <span v-if="liveProgressCount > 0">已处理 {{ liveProgressCount }} 人</span>
          </div>
          <div v-if="liveLastItem" class="tm-progress-sub">
            最新处理：<strong>{{ liveLastItem.candidateName }}</strong> — {{ liveLastItem.actionMessage || liveLastItem.reasonLabel }}
          </div>
        </div>

        <!-- Phase 1 / Phase 2 结果汇总与预览表格 -->
        <div v-if="currentRun" class="tm-result-section">
          <div class="tm-summary-cards">
            <div class="tm-stat-card">
              <span class="tm-stat-num">{{ currentRun.summary?.totalScanned || 0 }}</span>
              <span class="tm-stat-label">扫描待处理总数</span>
            </div>
            <div class="tm-stat-card danger" :class="{ active: filterTab === 'UNFIT' }" @click="filterTab = filterTab === 'UNFIT' ? 'ALL' : 'UNFIT'">
              <span class="tm-stat-num">{{ currentRun.summary?.unfitCount || 0 }}</span>
              <span class="tm-stat-label">{{ currentRun.mode === 'execute' ? '已标「不合适」' : '预计标「不合适」' }}</span>
            </div>
            <div class="tm-stat-card success" :class="{ active: filterTab === 'KEEP_MATCHED' }" @click="filterTab = filterTab === 'KEEP_MATCHED' ? 'ALL' : 'KEEP_MATCHED'">
              <span class="tm-stat-num">{{ currentRun.summary?.keepMatchedCount || 0 }}</span>
              <span class="tm-stat-label">符合条件保留</span>
            </div>
            <div class="tm-stat-card warning" :class="{ active: filterTab === 'KEEP_UNKNOWN' }" @click="filterTab = filterTab === 'KEEP_UNKNOWN' ? 'ALL' : 'KEEP_UNKNOWN'">
              <span class="tm-stat-num">{{ currentRun.summary?.keepUnknownCount || 0 }}</span>
              <span class="tm-stat-label">资料未知保留</span>
            </div>
            <div class="tm-stat-card info" :class="{ active: filterTab === 'WHITELIST_SKIP' }" @click="filterTab = filterTab === 'WHITELIST_SKIP' ? 'ALL' : 'WHITELIST_SKIP'">
              <span class="tm-stat-num">{{ currentRun.summary?.whitelistSkipCount || 0 }}</span>
              <span class="tm-stat-label">白名单保护豁免</span>
            </div>
          </div>

          <div v-if="currentRun.message" class="tm-run-message" :class="currentRun.status">
            <span>{{ currentRun.message }}</span>
            <el-button v-if="currentRun.id" size="small" type="primary" plain @click="exportRunExcel(currentRun.id)">
              <el-icon><Download /></el-icon>
              <span>导出 Excel 审计表</span>
            </el-button>
          </div>

          <!-- 候选人判定明细表格 -->
          <div class="tm-table-wrapper">
            <table class="tm-audit-table">
              <thead>
                <tr>
                  <th>候选人</th>
                  <th>基本画像 (年龄/年限/学历/现居地)</th>
                  <th>院校与专业</th>
                  <th>求职意向与薪资</th>
                  <th>判定结果</th>
                  <th>命中原因 / 51job 动作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="filteredItems.length === 0">
                  <td colspan="6" class="tm-empty-cell">当前分类下无候选人记录</td>
                </tr>
                <tr v-for="item in filteredItems" :key="item.candidateKey" :class="'row-' + item.decision">
                  <td class="col-name">
                    <strong>{{ item.candidateName }}</strong>
                    <span v-if="item.gender" class="tm-gender-tag">{{ item.gender }}</span>
                  </td>
                  <td>
                    {{ item.age !== null && item.age !== undefined ? item.age + '岁' : '年龄未知' }} ·
                    {{ formatWorkYears(item.workYears) }} ·
                    {{ item.education || '学历未知' }} ·
                    {{ item.residence || '现居未知' }}
                  </td>
                  <td>
                    <div class="tm-ellipsis" :title="`${item.school || ''} · ${item.major || ''} (${item.eduGradRange || ''})`">
                      {{ item.school || '院校未知' }} · {{ item.major || '专业未知' }}
                    </div>
                    <div v-if="item.eduGradRange" class="tm-sub-line">{{ item.eduGradRange }}</div>
                  </td>
                  <td>
                    <div class="tm-ellipsis">{{ item.intentRole || '—' }} · {{ item.intentCity || '城市未知' }}</div>
                    <div class="tm-sub-line">{{ item.intentSalaryRaw || '薪资未披露' }}</div>
                  </td>
                  <td>
                    <el-tag v-if="item.decision === 'UNFIT'" type="danger" size="small" effect="dark">不合适</el-tag>
                    <el-tag v-else-if="item.decision === 'KEEP_MATCHED'" type="success" size="small" effect="plain">符合保留</el-tag>
                    <el-tag v-else-if="item.decision === 'KEEP_UNKNOWN'" type="warning" size="small" effect="plain">资料未知保留</el-tag>
                    <el-tag v-else type="info" size="small" effect="plain">白名单豁免</el-tag>
                  </td>
                  <td class="col-reason">
                    <div>{{ item.reasonLabel }}</div>
                    <div v-if="item.clickedReasonButton" class="tm-clicked-btn">
                      已勾选原因：{{ item.clickedReasonButton }}
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="tm-dialog-footer">
        <div class="tm-footer-left">
          <span v-if="currentRun && currentRun.mode === 'dry_run'" class="tm-dryrun-tip">
            💡 试算仅读取页面未做任何修改。核对无误后点击右侧「确认并全自动执行」。
          </span>
        </div>
        <div class="tm-footer-right">
          <el-button v-if="running" type="danger" @click="handleStop">
            ⏹️ 停止任务
          </el-button>
          <el-button v-else @click="visible = false">关闭</el-button>

          <el-button
            v-if="!showHistory"
            type="primary"
            plain
            :loading="running && runningMode === 'dry_run'"
            :disabled="running"
            @click="handleDryRun"
          >
            🔍 第一步：只读试算 (Dry-Run)
          </el-button>

          <el-button
            v-if="!showHistory"
            type="danger"
            :loading="running && runningMode === 'execute'"
            :disabled="running || !canExecute"
            @click="handleExecute"
          >
            ⚡ 第二步：确认并全自动执行 ({{ currentRun?.summary?.unfitCount ?? '需先试算' }} 人标不合适)
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Clock, Loading, Download } from '@element-plus/icons-vue'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  projectId: { type: String, default: '' },
  projectName: { type: String, default: '' },
  jobTitle: { type: String, default: '' },
  defaultEducation: { type: String, default: '' },
  defaultCity: { type: String, default: '' }
})

const emit = defineEmits(['update:modelValue', 'completed'])

const visible = computed({
  get: () => props.modelValue,
  set: val => emit('update:modelValue', val)
})

const running = ref(false)
const runningMode = ref('dry_run')
const liveProgressCount = ref(0)
const liveLastItem = ref(null)
const currentRun = ref(null)
const filterTab = ref('ALL')
const showHistory = ref(false)
const historyRuns = ref([])

const createDefaultForm = () => ({
  jobTitle: props.jobTitle || props.projectName || '',
  minEducation: ['大专', '本科', '硕士', '博士'].includes(props.defaultEducation) ? props.defaultEducation : '',
  minWorkYears: 0,
  excludeStudents: false,
  allowedResidences: '',
  allowedIntentCities: props.defaultCity || '',
  allowedMajors: '',
  maxSalaryK: 0,
  enableAgeFilter: false,
  minAge: 0,
  maxAge: 0,
  enableGenderFilter: false,
  allowedGender: '',
  complianceConfirmed: false,
  maxCandidates: 200
})

const form = ref(createDefaultForm())

watch(
  () => [props.modelValue, props.projectId, props.jobTitle],
  ([open]) => {
    if (open) {
      form.value.jobTitle = props.jobTitle || props.projectName || form.value.jobTitle
      showHistory.value = false
    }
  }
)

const canExecute = computed(() => {
  return Boolean(currentRun.value && currentRun.value.Status !== 'error' && (currentRun.value.summary?.totalScanned || 0) > 0)
})

const filteredItems = computed(() => {
  const list = currentRun.value?.items || []
  if (filterTab.value === 'ALL') return list
  return list.filter(i => i.decision === filterTab.value)
})

function resetForm() {
  form.value = createDefaultForm()
  currentRun.value = null
  filterTab.value = 'ALL'
}

function formatWorkYears(wy) {
  if (wy === null || wy === undefined) return '年限未知'
  if (Number(wy) === 0) return '在校/无经验'
  return `${wy}年`
}

function formatTime(iso) {
  if (!iso) return ''
  try {
    return new Date(iso).toLocaleString('zh-CN', { hour12: false })
  } catch {
    return iso
  }
}

function validateBeforeRun() {
  if (!props.projectId) {
    ElMessage.warning('请先选择一个招聘职位项目')
    return false
  }
  if ((form.value.enableAgeFilter || form.value.enableGenderFilter) && !form.value.complianceConfirmed) {
    ElMessage.warning('已开启年龄或性别筛选，请先勾选合规确认声明')
    return false
  }
  return true
}

async function handleDryRun() {
  if (!validateBeforeRun()) return
  running.value = true
  runningMode.value = 'dry_run'
  liveProgressCount.value = 0
  liveLastItem.value = null
  filterTab.value = 'ALL'
  try {
    const res = await window.go.main.App.Preview51JobTalentManagementFilter(props.projectId, { ...form.value })
    currentRun.value = res
    ElMessage.success(res.message || '只读试算完成')
  } catch (err) {
    ElMessage.error(typeof err === 'string' ? err : err?.message || '只读试算失败')
  } finally {
    running.value = false
  }
}

async function handleExecute() {
  if (!validateBeforeRun()) return
  const unfitNum = currentRun.value?.summary?.unfitCount || 0
  const keepNum = (currentRun.value?.summary?.keepMatchedCount || 0) + (currentRun.value?.summary?.keepUnknownCount || 0)
  try {
    await ElMessageBox.confirm(
      `本次将在 51job「待处理」列表中自动将 ${unfitNum} 位明确不符的候选人标记为「不合适」并勾选原因，同时将 ${keepNum} 位符合/资料未知候选人保留并导入当前职位待分析。确认立即开始全自动执行？`,
      '确认全自动执行 51job 投递筛选',
      {
        confirmButtonText: '确认全自动执行',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
  } catch {
    return
  }

  running.value = true
  runningMode.value = 'execute'
  liveProgressCount.value = 0
  liveLastItem.value = null
  try {
    const res = await window.go.main.App.Run51JobTalentManagementFilter(props.projectId, { ...form.value })
    currentRun.value = res
    ElMessage.success(res.message || '全自动筛选执行完成')
    emit('completed', res)
  } catch (err) {
    ElMessage.error(typeof err === 'string' ? err : err?.message || '自动筛选执行中止')
  } finally {
    running.value = false
  }
}

async function handleStop() {
  try {
    const [, msg] = await window.go.main.App.Stop51JobTalentManagementFilter()
    ElMessage.info(msg || '正在安全停止…')
  } catch (err) {
    ElMessage.error('发送停止指令失败')
  }
}

async function loadHistoryRuns() {
  showHistory.value = !showHistory.value
  if (!showHistory.value) return
  try {
    historyRuns.value = (await window.go.main.App.List51JobTalentFilterRuns(props.projectId || '')) || []
  } catch {
    historyRuns.value = []
  }
}

async function viewRunDetail(runId) {
  try {
    const detail = await window.go.main.App.Get51JobTalentFilterRunDetail(runId)
    currentRun.value = detail
    showHistory.value = false
  } catch (err) {
    ElMessage.error('读取批次详情失败')
  }
}

async function exportRunExcel(runId) {
  try {
    const path = await window.go.main.App.Export51JobTalentFilterReport(runId)
    if (path) {
      ElMessage.success(`审计报告已导出至：${path}`)
    }
  } catch (err) {
    ElMessage.error(typeof err === 'string' ? err : err?.message || '导出 Excel 失败')
  }
}

function handleClose() {
  if (!running.value) {
    visible.value = false
  }
}

onMounted(() => {
  EventsOn('51job-talent-filter-progress', payload => {
    if (!payload) return
    liveProgressCount.value = payload.processedCount || 0
    liveLastItem.value = payload.item || null
  })
})

onBeforeUnmount(() => {
  EventsOff('51job-talent-filter-progress')
})
</script>

<style scoped>
.tm-filter-dialog {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: 72vh;
  overflow-y: auto;
  padding-right: 4px;
}

.tm-context-banner {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 12px 14px;
}

.tm-context-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.tm-label {
  color: #64748b;
  font-size: 13px;
}

.tm-project-name {
  color: #0f172a;
  font-size: 14px;
}

.tm-safety-notice {
  font-size: 12.5px;
  color: #334155;
  line-height: 1.5;
  background: #eff6ff;
  border-left: 3px solid #3b82f6;
  padding: 6px 10px;
  border-radius: 4px;
}

.tm-filter-form-card {
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 14px;
  background: #ffffff;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tm-form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tm-section-title {
  font-weight: 600;
  font-size: 13.5px;
  color: #1e293b;
}

.tm-grid-row {
  display: grid;
  gap: 12px;
}

.tm-grid-row.two-cols {
  grid-template-columns: 1fr 1fr;
}

.tm-grid-row.three-cols {
  grid-template-columns: 1fr 1fr 1fr;
}

.tm-grid-row.one-col {
  grid-template-columns: 1fr;
}

.tm-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tm-field label {
  font-size: 12px;
  color: #475569;
  font-weight: 500;
}

.tm-switch-box {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
}

.tm-switch-hint {
  font-size: 12px;
  color: #64748b;
}

.tm-compliance-box {
  background: #fffbeb;
  border: 1px dashed #f59e0b;
  border-radius: 8px;
  padding: 10px 12px;
}

.tm-compliance-toggles {
  display: flex;
  gap: 20px;
}

.tm-age-range {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tm-range-sep {
  font-size: 12px;
  color: #64748b;
}

.tm-compliance-confirm {
  margin-top: 8px;
}

.c-warning {
  color: #b45309;
  font-size: 12px;
}

.c-danger {
  color: #dc2626;
}

.c-success {
  color: #16a34a;
}

.tm-progress-banner {
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 8px;
  padding: 10px 14px;
}

.tm-progress-top {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #0369a1;
  font-size: 13px;
}

.tm-progress-sub {
  margin-top: 4px;
  font-size: 12px;
  color: #334155;
}

.tm-summary-cards {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
  margin-bottom: 10px;
}

.tm-stat-card {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 10px;
  text-align: center;
  cursor: pointer;
  background: #f8fafc;
  transition: all 0.15s ease;
}

.tm-stat-card.active {
  box-shadow: 0 0 0 2px #3b82f6;
}

.tm-stat-card.danger {
  background: #fef2f2;
  border-color: #fecaca;
}

.tm-stat-card.success {
  background: #f0fdf4;
  border-color: #bbf7d0;
}

.tm-stat-card.warning {
  background: #fffbeb;
  border-color: #fde68a;
}

.tm-stat-card.info {
  background: #eff6ff;
  border-color: #bfdbfe;
}

.tm-stat-num {
  display: block;
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
}

.tm-stat-label {
  font-size: 12px;
  color: #475569;
}

.tm-run-message {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  border-radius: 6px;
  background: #f1f5f9;
  font-size: 12.5px;
  margin-bottom: 10px;
}

.tm-table-wrapper {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  max-height: 260px;
  overflow-y: auto;
}

.tm-audit-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12.5px;
}

.tm-audit-table th {
  position: sticky;
  top: 0;
  background: #f8fafc;
  color: #475569;
  text-align: left;
  padding: 8px 10px;
  border-bottom: 1px solid #e2e8f0;
  z-index: 1;
}

.tm-audit-table td {
  padding: 8px 10px;
  border-bottom: 1px solid #f1f5f9;
  color: #1e293b;
  vertical-align: top;
}

.tm-ellipsis {
  max-width: 180px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tm-sub-line {
  font-size: 11.5px;
  color: #64748b;
}

.tm-gender-tag {
  margin-left: 4px;
  font-size: 11px;
  color: #64748b;
  background: #f1f5f9;
  padding: 1px 5px;
  border-radius: 4px;
}

.tm-clicked-btn {
  margin-top: 2px;
  font-size: 11.5px;
  color: #dc2626;
  font-weight: 500;
}

.tm-empty-cell,
.tm-empty-box {
  text-align: center;
  padding: 24px;
  color: #94a3b8;
}

.tm-history-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.tm-history-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.tm-history-main {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}

.tm-mode-badge {
  font-size: 11.5px;
  padding: 2px 6px;
  border-radius: 4px;
  background: #e2e8f0;
}

.tm-mode-badge.execute {
  background: #fee2e2;
  color: #b91c1c;
}

.tm-dialog-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tm-dryrun-tip {
  font-size: 12px;
  color: #64748b;
}

.tm-footer-right {
  display: flex;
  gap: 8px;
}
</style>
