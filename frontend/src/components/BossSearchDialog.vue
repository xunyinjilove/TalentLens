<template>
  <el-dialog
    v-model="visible"
    :title="$t('boss.dialogTitle') || '双模智能寻才与 AI 评测系统'"
    width="640px"
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
        <!-- 检索渠道双模切换卡片 -->
        <div class="channel-section">
          <label class="form-label">选择人才搜寻通道</label>
          <div class="channel-selector">
            <div
              class="channel-card"
              :class="{ active: form.mode === 'public' }"
              @click="form.mode = 'public'"
            >
              <div class="channel-header">
                <span class="channel-icon">🌐</span>
                <span class="channel-title">全网公开渠道实时检索</span>
                <el-tag size="small" type="success" effect="plain" class="channel-badge">免登录 · 推荐</el-tag>
              </div>
              <div class="channel-desc">
                无需企业账号，实时抓取行业公开人才档案快照，支持任意岗位即搜即测并流转 AI 审核。
              </div>
            </div>

            <div
              class="channel-card"
              :class="{ active: form.mode === 'boss' }"
              @click="form.mode = 'boss'"
            >
              <div class="channel-header">
                <span class="channel-icon">🏢</span>
                <span class="channel-title">BOSS 直聘企业端直连</span>
                <el-tag size="small" type="primary" effect="plain" class="channel-badge">需企业账号</el-tag>
              </div>
              <div class="channel-desc">
                直连 BOSS 直聘企业后台，支持扫码鉴权、在线牛人检索、打招呼及在线索取完整附件简历。
              </div>
            </div>
          </div>
        </div>

        <div class="form-row" style="margin-top: 4px;">
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
            <label class="form-label">计划搜寻人才数</label>
            <el-select v-model="form.count" style="width: 100%">
              <el-option :label="'5 位优质候选人'" :value="5" />
              <el-option :label="'10 位优质候选人 (推荐)'" :value="10" />
              <el-option :label="'15 位优质候选人'" :value="15" />
              <el-option :label="'20 位优质候选人'" :value="20" />
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
          <span v-if="form.mode === 'public'">
            【全网公开通道】：已开启自动化数据管道，检索结果将实时存入本地并启动多维度 AI 匹配与真实度核验。
          </span>
          <span v-else>
            【BOSS直聘直连】：将唤起 Edge 浏览器连接 BOSS 招聘者后台，若未登录请点击下方「扫码登录」完成鉴权。
          </span>
        </div>
      </div>

      <!-- 搜索进行中与日志展示区 -->
      <div v-else class="searching-dashboard">
        <div class="progress-wrap">
          <div class="progress-info">
            <span class="status-title">{{ currentStatusText }}</span>
            <span class="progress-num">{{ candidateCount }} / {{ form.count }} 人</span>
          </div>
          <el-progress
            :percentage="Math.min(100, Math.round((candidateCount / form.count) * 100))"
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
        <el-button
          v-if="!searching && searchLogs.length === 0 && form.mode === 'boss'"
          type="warning"
          plain
          @click="handleTestLogin"
        >
          <el-icon><Key /></el-icon>
          测试/扫码企业登录
        </el-button>
        <el-button v-if="searching" type="danger" plain @click="handleStop">停止搜寻</el-button>
        <el-button
          v-if="!searching && searchLogs.length === 0"
          type="primary"
          @click="handleStartSearch"
          :disabled="!form.keyword.trim()"
        >
          <el-icon><Search /></el-icon>
          {{ form.mode === 'public' ? '开始全网智能检索' : '连接 BOSS 直聘寻才' }}
        </el-button>
        <el-button
          v-if="isFinished"
          type="success"
          @click="handleCompleteAndClose"
        >
          <el-icon><Check /></el-icon>
          完成 ({{ candidateCount }} 人)
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Search,
  User,
  Check,
  CircleCheck,
  WarningFilled,
  Loading,
  InfoFilled,
  Key
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

const form = reactive({
  mode: 'public' as 'public' | 'boss',
  keyword: '',
  city: '上海',
  count: 10,
  autoAnalyze: true
})

watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    // 重置状态与表单初始值
    form.mode = 'public'
    form.keyword = props.jobTitle || '临床项目经理'
    form.city = '上海'
    form.count = 10
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

// 独立测试扫码登录与鉴权 (针对 BOSS 直聘企业端)
async function handleTestLogin() {
  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch {}

  searching.value = true
  isFinished.value = false
  candidateCount.value = 0
  searchLogs.value = []
  currentStatusText.value = '正在唤起浏览器进行登录测试...'
  addLog('status', '🔑 正在启动 Edge 浏览器打开 BOSS 直聘企业端鉴权页...')

  if (WailsApp && WailsApp.TestBossLogin) {
    try {
      await WailsApp.TestBossLogin()
    } catch (err: any) {
      searching.value = false
      addLog('error', `启动测试失败: ${err.message || err}`)
    }
  } else {
    setTimeout(() => {
      addLog('status', '✅ 登录态测试通道正常连接！')
      searching.value = false
      isFinished.value = true
    }, 1500)
  }
}

// 启动搜索
async function handleStartSearch() {
  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch {
    ElMessage.warning('当前运行在开发预览模式')
  }

  const channelLabel = form.mode === 'public' ? '全网公开通道（免登录）' : 'BOSS直聘企业端'

  searching.value = true
  isFinished.value = false
  candidateCount.value = 0
  searchLogs.value = []
  currentStatusText.value = `正在启动【${channelLabel}】搜寻引擎...`
  addLog('status', `🚀 启动搜寻任务：[${form.city}] 岗位「${form.keyword}」，目标 ${form.count} 人，通道：${channelLabel}`)

  if (WailsApp && WailsApp.StartBossSearch) {
    try {
      await WailsApp.StartBossSearch(
        props.projectId,
        form.keyword.trim(),
        form.city,
        props.expYears || 3,
        props.eduLevel || '本科',
        form.count,
        form.mode
      )
    } catch (err: any) {
      searching.value = false
      addLog('error', `启动失败: ${err.message || err}`)
      ElMessage.error('启动搜寻失败')
    }
  } else {
    // Mock 模式模拟流水
    setTimeout(() => {
      addLog('status', `🔍 正在检索【${form.city}】关于「${form.keyword}」的优质人才档案...`)
    }, 600)
    for (let i = 1; i <= form.count; i++) {
      setTimeout(() => {
        candidateCount.value = i
        addLog('candidate', `👤 成功提取候选人: 候选人_${i}（${form.keyword}，${props.expYears || 3}年经验，${props.eduLevel || '本科'}）`)
        if (i === form.count) {
          searching.value = false
          isFinished.value = true
          currentStatusText.value = '搜寻完成！'
          addLog('done', `🎉 成功检索并导入 ${form.count} 位候选人，已自动启动 AI 智能评估！`)
          emit('refresh')
        }
      }, 1000 + i * 400)
    }
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

function handleCompleteAndClose() {
  visible.value = false
  emit('refresh')
}

function handleClose() {
  if (searching.value) {
    handleStop()
  }
}

// 绑定 Wails 事件监听
let unsubscribeList: Array<() => void> = []

onMounted(async () => {
  let WailsRuntime: any = null
  try { WailsRuntime = await import('../../wailsjs/runtime/runtime') } catch {}
  if (!WailsRuntime) return

  const offStatus = WailsRuntime.EventsOn('boss:status', (evt: any) => {
    if (evt && evt.message) {
      currentStatusText.value = evt.message
      addLog('status', evt.message)
    }
  })

  const offCandidate = WailsRuntime.EventsOn('boss:candidate_found', (evt: any) => {
    if (evt && evt.candidate) {
      candidateCount.value = evt.current || (candidateCount.value + 1)
      const c = evt.candidate
      addLog('candidate', `👤 成功检索到候选人: ${c.name}（${c.school || c.education} · ${c.experience} · ${c.company || 'IVD医药企业'}）`)
      emit('refresh')
    }
  })

  const offDone = WailsRuntime.EventsOn('boss:done', (evt: any) => {
    searching.value = false
    isFinished.value = true
    currentStatusText.value = '搜寻完成，已启动 AI 深度分析！'
    addLog('done', evt.message || '🎉 搜寻完成，候选人已全部导入！')
    ElMessage.success('候选人档案已全部抓取并导入，正在进行 AI 智能打分！')
    emit('refresh')
  })

  const offError = WailsRuntime.EventsOn('boss:error', (evt: any) => {
    searching.value = false
    currentStatusText.value = '搜寻出错'
    addLog('error', evt.message || '搜寻过程中发生错误')
    ElMessage.error(evt.message || '搜寻失败')
  })

  unsubscribeList = [offStatus, offCandidate, offDone, offError]
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
  gap: 10px;

  .channel-section {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .channel-selector {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;

    .channel-card {
      border: 1.5px solid #e2e8f0;
      border-radius: $radius-md;
      padding: 10px 12px;
      background: #f8fafc;
      cursor: pointer;
      transition: all 0.2s ease;
      display: flex;
      flex-direction: column;
      gap: 5px;

      &:hover {
        border-color: #93c5fd;
        background: #ffffff;
      }

      &.active {
        border-color: #007aff;
        background: #eff6ff;
        box-shadow: 0 2px 8px rgba(0, 122, 255, 0.12);

        .channel-title {
          color: #007aff;
          font-weight: 700;
        }
      }

      .channel-header {
        display: flex;
        align-items: center;
        gap: 6px;

        .channel-icon {
          font-size: 15px;
        }

        .channel-title {
          font-size: 13px;
          font-weight: 600;
          color: #1e293b;
          flex: 1;
        }

        .channel-badge {
          font-size: 10.5px;
          padding: 0 4px;
          height: 18px;
          line-height: 16px;
        }
      }

      .channel-desc {
        font-size: 11px;
        color: #64748b;
        line-height: 1.4;
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
