<template>
  <el-dialog
    v-model="visible"
    :title="'🌐 4合1 全渠道智能矩阵寻才与 AI 评测系统'"
    width="680px"
    :close-on-click-modal="!searching"
    :close-on-press-escape="!searching"
    @close="handleClose"
  >
    <div class="boss-dialog-content">
      <!-- 关联项目与岗位提示卡片 -->
      <div class="job-context-card">
        <div class="context-top">
          <span class="context-label">目标招聘项目:</span>
          <span class="context-project">{{ projectName || '当前项目' }}</span>
        </div>
        <div class="context-job">
          <el-tag size="small" type="success" effect="dark" class="job-tag">
            💼 {{ jobTitle || '未指定岗位' }}
          </el-tag>
          <span class="job-reqs">{{ expYears ? expYears + '年经验' : '经验不限' }} · {{ eduLevel || '学历不限' }}</span>
        </div>
      </div>

      <!-- 搜索配置表单 -->
      <div class="search-form" v-if="!searching && searchLogs.length === 0">
        <!-- 4 大平台矩阵选择与独立登录管理面板 -->
        <div class="platform-grid-section">
          <div class="section-title-row">
            <label class="form-label">选择调度招聘渠道（支持多选并发）</label>
            <span class="channel-hint">已勾选 {{ selectedPlatformCodes.length }} / 4 个渠道</span>
          </div>

          <div class="platform-grid">
            <div
              v-for="p in platformList"
              :key="p.code"
              class="platform-card"
              :class="{ active: p.selected }"
              @click="togglePlatform(p)"
            >
              <div class="platform-top">
                <el-checkbox v-model="p.selected" @click.stop />
                <span class="platform-icon">{{ p.icon }}</span>
                <span class="platform-name">{{ p.name }}</span>
                <el-tag size="small" :type="p.tagType" effect="plain" class="platform-tag">
                  {{ p.badge }}
                </el-tag>
              </div>
              <div class="platform-desc">{{ p.desc }}</div>
              <div class="platform-action">
                <button
                  class="login-test-btn"
                  @click.stop="handleTestPlatform(p.code, p.name)"
                  title="单独在 Edge 浏览器中扫码或登录该平台企业端"
                >
                  <el-icon><Key /></el-icon> 扫码 / 登录
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="form-row" style="margin-top: 6px;">
          <div class="form-item flex-2">
            <label class="form-label">搜索岗位关键词</label>
            <el-input v-model="form.keyword" placeholder="输入搜索关键词，如：临床项目经理" />
          </div>
          <div class="form-item flex-1">
            <label class="form-label">目标城市</label>
            <el-select v-model="form.city" filterable allow-create default-first-option style="width: 100%">
              <el-option v-for="c in cityOptions" :key="c" :label="c" :value="c" />
            </el-select>
          </div>
        </div>

        <div class="form-row" style="margin-top: 8px;">
          <div class="form-item flex-1">
            <label class="form-label">单渠道目标采集数</label>
            <el-select v-model="form.countPerPlatform" style="width: 100%">
              <el-option :label="'5 人 / 平台'" :value="5" />
              <el-option :label="'10 人 / 平台 (推荐)'" :value="10" />
              <el-option :label="'15 人 / 平台'" :value="15" />
              <el-option :label="'20 人 / 平台'" :value="20" />
            </el-select>
          </div>
          <div class="form-item flex-1">
            <label class="form-label">自动 AI 深度评估</label>
            <div class="switch-box">
              <el-switch v-model="form.autoAnalyze" />
              <span class="switch-text">抓取后即刻初筛与出题</span>
            </div>
          </div>
        </div>

        <div class="safety-tip">
          <el-icon><InfoFilled /></el-icon>
          <span>
            【全渠道直连说明】：系统将通过隔离浏览器直连已勾选平台的企业后台，若某平台未登录将引导扫码。跨平台重合的候选人将自动识别聚合。
          </span>
        </div>
      </div>

      <!-- 搜索进行中与日志展示区 -->
      <div v-else class="searching-dashboard">
        <div class="progress-wrap">
          <div class="progress-info">
            <span class="status-title">{{ currentStatusText }}</span>
            <span class="progress-num">{{ candidateCount }} / {{ expectedTotalCount }} 人</span>
          </div>
          <el-progress
            :percentage="Math.min(100, Math.round((candidateCount / Math.max(1, expectedTotalCount)) * 100))"
            :status="isFinished ? 'success' : ''"
            :indeterminate="searching && candidateCount === 0"
            :stroke-width="10"
            striped
            striped-flow
          />
        </div>

        <!-- 实时抓取流水日志 -->
        <div class="log-stream-box" ref="logBoxRef">
          <div
            v-for="(log, idx) in searchLogs"
            :key="idx"
            class="log-item"
            :class="log.type"
          >
            <span class="log-time">{{ formatTime(log.time) }}</span>
            <span class="log-icon">
              <el-icon v-if="log.type === 'candidate'"><User /></el-icon>
              <el-icon v-else-if="log.type === 'done'"><CircleCheck /></el-icon>
              <el-icon v-else-if="log.type === 'error'"><WarningFilled /></el-icon>
              <el-icon v-else><Loading class="spin" /></el-icon>
            </span>
            <span class="log-msg">{{ log.message }}</span>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button v-if="!searching" @click="visible = false">取消</el-button>
        <el-button v-if="searching" type="danger" plain @click="handleStop">停止检索</el-button>
        <el-button
          v-if="!searching && searchLogs.length === 0"
          type="primary"
          @click="handleStartSearch"
          :disabled="!form.keyword.trim() || selectedPlatformCodes.length === 0"
        >
          <el-icon><Search /></el-icon>
          启动矩阵并发检索 (已选 {{ selectedPlatformCodes.length }} 个平台)
        </el-button>
        <el-button
          v-if="isFinished"
          type="primary"
          @click="handleContinueSearch"
        >
          <el-icon><RefreshRight /></el-icon>
          继续寻找下一批 (增量 {{ form.countPerPlatform }} 人 · 自动排重)
        </el-button>
        <el-button
          v-if="isFinished"
          type="success"
          @click="handleCompleteAndClose"
        >
          <el-icon><Check /></el-icon>
          完成并查看 (本批 {{ candidateCount }} 人)
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Search,
  User,
  Check,
  CircleCheck,
  WarningFilled,
  Loading,
  InfoFilled,
  Key,
  RefreshRight
} from '@element-plus/icons-vue'

const props = defineProps<{
  modelValue: boolean
  projectId: string
  projectName?: string
  jobTitle?: string
  expYears?: number
  eduLevel?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'refresh'): void
}>()

const visible = ref(false)
const searching = ref(false)
const isFinished = ref(false)
const currentStatusText = ref('正在就绪...')
const candidateCount = ref(0)
const logBoxRef = ref<HTMLElement | null>(null)

interface SearchLog {
  time: number
  type: 'status' | 'candidate' | 'done' | 'error'
  message: string
}

const searchLogs = ref<SearchLog[]>([])

const cityOptions = ['上海', '北京', '深圳', '广州', '杭州', '南京', '武汉', '成都', '苏州', '全国']

// 4 大招聘平台配置
const platformList = reactive([
  {
    code: 'boss',
    name: 'BOSS直聘',
    icon: '🏢',
    desc: '企业直连推荐牛人，微简历与在线沟通',
    selected: true,
    badge: '直聘热门',
    tagType: 'success' as const
  },
  {
    code: 'zhaopin',
    name: '智联招聘',
    icon: '💼',
    desc: 'iHR 人才搜索库与智能推荐人才',
    selected: true,
    badge: '白领大盘',
    tagType: 'primary' as const
  },
  {
    code: '51job',
    name: '前程无忧',
    icon: '📑',
    desc: 'eHire 招聘管理系统与简历库检索',
    selected: true,
    badge: '老牌综合',
    tagType: 'warning' as const
  },
  {
    code: 'liepin',
    name: '猎聘网',
    icon: '🎯',
    desc: 'LPT 企业中高端人才库与精准搜索',
    selected: true,
    badge: '中高端',
    tagType: 'danger' as const
  }
])

const selectedPlatformCodes = computed(() => {
  return platformList.filter(p => p.selected).map(p => p.code)
})

function togglePlatform(p: any) {
  p.selected = !p.selected
}

const form = reactive({
  keyword: '',
  city: '上海',
  countPerPlatform: 5,
  autoAnalyze: true
})

const expectedTotalCount = computed(() => {
  return selectedPlatformCodes.value.length * form.countPerPlatform
})

watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    // 重置状态与表单初始值
    form.keyword = props.jobTitle || '临床项目经理'
    form.city = '上海'
    form.countPerPlatform = 5
    searching.value = false
    isFinished.value = false
    candidateCount.value = 0
    searchLogs.value = []
    currentStatusText.value = '准备就绪'
  }
})

watch(visible, (val) => {
  emit('update:modelValue', val)
})

function formatTime(ts: number) {
  const d = new Date(ts)
  return d.toTimeString().split(' ')[0]
}

function addLog(type: SearchLog['type'], message: string) {
  searchLogs.value.push({ time: Date.now(), type, message })
  nextTick(() => {
    if (logBoxRef.value) {
      logBoxRef.value.scrollTop = logBoxRef.value.scrollHeight
    }
  })
}

// 独立测试某平台登录
async function handleTestPlatform(platformCode: string, platformName: string) {
  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch {}

  searching.value = true
  isFinished.value = false
  candidateCount.value = 0
  searchLogs.value = []
  currentStatusText.value = `正在唤起 Edge 打开【${platformName}】企业后台...`
  addLog('status', `🔑 正在启动 Edge 浏览器连接【${platformName}】鉴权页...`)

  if (WailsApp && WailsApp.TestPlatformLogin) {
    try {
      await WailsApp.TestPlatformLogin(platformCode)
    } catch (err: any) {
      searching.value = false
      addLog('error', `启动测试失败: ${err.message || err}`)
    }
  } else if (WailsApp && WailsApp.TestBossLogin && platformCode === 'boss') {
    try {
      await WailsApp.TestBossLogin()
    } catch (err: any) {
      searching.value = false
      addLog('error', `启动测试失败: ${err.message || err}`)
    }
  } else {
    setTimeout(() => {
      addLog('status', `✅ 【${platformName}】登录测试通道已就绪！`)
      searching.value = false
      isFinished.value = true
    }, 1500)
  }
}

// 启动多平台并发搜索
async function handleStartSearch() {
  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch {
    ElMessage.warning('当前运行在开发预览模式')
  }

  const plats = selectedPlatformCodes.value
  if (plats.length === 0) {
    ElMessage.warning('请至少勾选一个招聘平台！')
    return
  }

  searching.value = true
  isFinished.value = false
  candidateCount.value = 0
  seenCandidateIds.clear()
  lastStatusMsg = ''
  doneTriggered = false
  searchLogs.value = []
  currentStatusText.value = '正在启动矩阵式检索引擎...'
  addLog('status', `🚀 启动多平台聚合寻才：[${form.city}] 岗位「${form.keyword}」，调度平台：${plats.join('、')}，单平台目标 ${form.countPerPlatform} 人`)

  if (WailsApp && WailsApp.StartMultiPlatformSearch) {
    try {
      await WailsApp.StartMultiPlatformSearch(
        props.projectId,
        form.keyword.trim(),
        form.city,
        props.expYears || 3,
        props.eduLevel || '本科',
        form.countPerPlatform,
        plats
      )
    } catch (err: any) {
      searching.value = false
      addLog('error', `启动失败: ${err.message || err}`)
      ElMessage.error('启动搜寻失败')
    }
  } else if (WailsApp && WailsApp.StartBossSearch) {
    try {
      await WailsApp.StartBossSearch(
        props.projectId,
        form.keyword.trim(),
        form.city,
        props.expYears || 3,
        props.eduLevel || '本科',
        form.countPerPlatform
      )
    } catch (err: any) {
      searching.value = false
      addLog('error', `启动失败: ${err.message || err}`)
      ElMessage.error('启动搜寻失败')
    }
  } else {
    ElMessage.info('开发模式下需在客户端中运行多平台直连引擎')
    searching.value = false
  }
}

// 停止搜索
async function handleStop() {
  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch {}
  if (WailsApp && WailsApp.StopBossSearch) {
    await WailsApp.StopBossSearch()
  }
  searching.value = false
  currentStatusText.value = '已停止搜寻'
  addLog('status', '⏹️ 已手动停止搜寻任务')
}

async function handleContinueSearch() {
  emit('refresh')
  ElMessage.info(`正在为您启动下一批检索（每平台 ${form.countPerPlatform} 人，自动跳过已有简历）...`)
  await handleStartSearch()
}

function handleCompleteAndClose() {
  visible.value = false
  emit('refresh')
}

function handleClose() {
  if (searching.value) {
    handleStop()
  }
}

// 绑定 Wails 事件监听 (统一监听 platform:* 并引入候选人唯一排重，根除计数翻倍问题)
let unsubscribeList: Array<() => void> = []
const seenCandidateIds = new Set<string>()
let lastStatusMsg = ''
let doneTriggered = false

onMounted(async () => {
  let WailsRuntime: any = null
  try { WailsRuntime = await import('../../wailsjs/runtime/runtime') } catch {}
  if (!WailsRuntime) return

  const handleStatus = (evt: any) => {
    if (evt && evt.message) {
      if (lastStatusMsg === evt.message) return // 防重打印
      lastStatusMsg = evt.message
      currentStatusText.value = evt.message
      addLog('status', evt.message)
    }
  }

  const handleCandidate = (evt: any) => {
    if (evt && evt.candidate) {
      const c = evt.candidate
      const candKey = c.id || `${c.platform}_${c.name}_${c.fileName}`
      if (seenCandidateIds.has(candKey)) return // 排重，根绝翻倍
      seenCandidateIds.add(candKey)

      candidateCount.value = seenCandidateIds.size
      const pName = c.platformName || evt.platformName || '招聘平台'
      addLog('candidate', `👤 [${pName}] 成功提取牛人: ${c.name}（${c.experience} · ${c.company || '在线履历'}）`)
      emit('refresh')
    }
  }

  const handleDone = (evt: any) => {
    if (doneTriggered) return
    doneTriggered = true
    searching.value = false
    isFinished.value = true
    currentStatusText.value = '全渠道检索完成，已启动 AI 分析！'
    addLog('done', evt.message || '🎉 候选人已全部采集并导入！')
    ElMessage.success(`多平台候选人采集完成 (共 ${candidateCount.value} 人)，正在进行 AI 智能打分！`)
    emit('refresh')
  }

  const handleError = (evt: any) => {
    searching.value = false
    currentStatusText.value = '检索提示'
    addLog('error', evt.message || '检索过程中发生提示')
    ElMessage.error(evt.message || '操作未完成')
  }

  // 仅监听统一平台事件，彻底切断与旧 boss:* 兼容事件的双发重叠
  const offPlatformStatus = WailsRuntime.EventsOn('platform:status', handleStatus)
  const offPlatformCandidate = WailsRuntime.EventsOn('platform:candidate_found', handleCandidate)
  const offPlatformDone = WailsRuntime.EventsOn('platform:done', handleDone)
  const offPlatformError = WailsRuntime.EventsOn('platform:error', handleError)

  unsubscribeList = [
    offPlatformStatus, offPlatformCandidate, offPlatformDone, offPlatformError
  ]
})

onUnmounted(() => {
  unsubscribeList.forEach(un => {
    try { un() } catch {}
  })
})
</script>

<style scoped lang="scss">
@import '../styles/macos-theme.scss';

.boss-dialog-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.job-context-card {
  background: #f0f7ff;
  border: 1px solid #bae0ff;
  border-radius: $radius-md;
  padding: 10px 14px;
  display: flex;
  justify-content: space-between;
  align-items: center;

  .context-top {
    display: flex;
    align-items: center;
    gap: 8px;

    .context-label {
      font-size: 12.5px;
      color: #0050b3;
    }

    .context-project {
      font-size: 13.5px;
      font-weight: 700;
      color: #0958d9;
    }
  }

  .context-job {
    display: flex;
    align-items: center;
    gap: 8px;

    .job-tag {
      font-weight: 600;
      border-radius: 4px;
    }

    .job-reqs {
      font-size: 12px;
      color: #595959;
    }
  }
}

.search-form {
  display: flex;
  flex-direction: column;
  gap: 12px;

  .platform-grid-section {
    display: flex;
    flex-direction: column;
    gap: 8px;

    .section-title-row {
      display: flex;
      justify-content: space-between;
      align-items: center;

      .channel-hint {
        font-size: 11.5px;
        color: #0958d9;
        font-weight: 600;
      }
    }

    .platform-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 10px;

      .platform-card {
        border: 1.5px solid #e2e8f0;
        border-radius: $radius-md;
        padding: 10px 12px;
        background: #f8fafc;
        cursor: pointer;
        transition: all 0.2s ease;
        display: flex;
        flex-direction: column;
        gap: 6px;

        &:hover {
          border-color: #93c5fd;
          background: #ffffff;
        }

        &.active {
          border-color: #007aff;
          background: #eff6ff;
          box-shadow: 0 2px 8px rgba(0, 122, 255, 0.12);

          .platform-name {
            color: #007aff;
            font-weight: 700;
          }
        }

        .platform-top {
          display: flex;
          align-items: center;
          gap: 6px;

          .platform-icon {
            font-size: 15px;
          }

          .platform-name {
            font-size: 13px;
            font-weight: 600;
            color: #1e293b;
            flex: 1;
          }

          .platform-tag {
            font-size: 10.5px;
            padding: 0 4px;
            height: 18px;
            line-height: 16px;
          }
        }

        .platform-desc {
          font-size: 11px;
          color: #64748b;
          line-height: 1.35;
        }

        .platform-action {
          display: flex;
          justify-content: flex-end;
          margin-top: 2px;

          .login-test-btn {
            background: #ffffff;
            border: 1px solid #cbd5e1;
            border-radius: 4px;
            padding: 2px 8px;
            font-size: 11px;
            color: #475569;
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 3px;
            transition: all 0.15s ease;

            &:hover {
              color: #007aff;
              border-color: #007aff;
              background: #f0f7ff;
            }
          }
        }
      }
    }
  }

  .form-row {
    display: flex;
    gap: 12px;
  }

  .form-item {
    display: flex;
    flex-direction: column;
    gap: 5px;

    &.flex-1 { flex: 1; }
    &.flex-2 { flex: 2; }
  }

  .form-label {
    font-size: 12px;
    font-weight: 600;
    color: $text-secondary;
  }

  .switch-box {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 32px;

    .switch-text {
      font-size: 12px;
      color: $text-secondary;
    }
  }

  .safety-tip {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    background: #fafafa;
    border: 1px solid #f0f0f0;
    padding: 8px 12px;
    border-radius: $radius-sm;
    font-size: 11.5px;
    color: #64748b;
    line-height: 1.45;

    .el-icon {
      color: $system-blue;
      margin-top: 2px;
      flex-shrink: 0;
    }
  }
}

.searching-dashboard {
  display: flex;
  flex-direction: column;
  gap: 12px;

  .progress-wrap {
    background: #ffffff;
    border: 1px solid $separator;
    border-radius: $radius-md;
    padding: 12px 14px;

    .progress-info {
      display: flex;
      justify-content: space-between;
      align-items: baseline;
      margin-bottom: 8px;

      .status-title {
        font-size: 13px;
        font-weight: 700;
        color: $text-primary;
      }

      .progress-num {
        font-size: 13px;
        font-weight: 700;
        color: $system-blue;
      }
    }
  }

  .log-stream-box {
    height: 190px;
    overflow-y: auto;
    background: #1e1e24;
    border-radius: $radius-md;
    padding: 12px 14px;
    font-family: Consolas, Monaco, "Courier New", monospace;
    font-size: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;

    .log-item {
      display: flex;
      align-items: baseline;
      gap: 8px;
      line-height: 1.4;

      .log-time {
        color: #6e7681;
        font-size: 11px;
        flex-shrink: 0;
      }

      .log-icon {
        font-size: 12px;
        flex-shrink: 0;
      }

      .log-msg {
        word-break: break-all;
      }

      &.status {
        color: #8be9fd;
      }
      &.candidate {
        color: #50fa7b;
      }
      &.done {
        color: #f1fa8c;
        font-weight: bold;
      }
      &.error {
        color: #ff5555;
      }
    }
  }
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
