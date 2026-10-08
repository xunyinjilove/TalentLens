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

      <!-- 搜索配置表单（先选择平台与确认参数） -->
      <div class="search-form" v-if="!isSearchingStarted">
        <!-- 继续寻才模式专属提示横幅 -->
        <div v-if="isContinue" class="continue-mode-banner">
          <div class="banner-badge">
            <el-icon><RefreshRight /></el-icon>
            <span>增量继续寻才模式</span>
          </div>
          <div class="banner-desc">
            已继承首次寻才要求（<strong>{{ form.keyword || jobTitle }} · {{ form.city }}</strong>），已自动开启<strong>跨批次全局排重</strong>。请勾选本次要继续深挖的招聘平台：
          </div>
        </div>

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
            <div class="label-with-action">
              <label class="form-label">搜索岗位关键词</label>
              <button
                type="button"
                class="ai-synonym-trigger-btn"
                :class="{ active: isQuotaMatrixActive }"
                @click="handleToggleQuotaMatrix"
              >
                <el-icon v-if="isGeneratingSynonyms" class="spin"><Loading /></el-icon>
                <el-icon v-else><Opportunity /></el-icon>
                <span>{{ isQuotaMatrixActive ? '收起配额调度矩阵' : '⚡ AI 智能拓词与配额调度 (突破茧房)' }}</span>
              </button>
            </div>
            <el-input v-model="form.keyword" placeholder="输入搜索关键词，如：临床项目经理" />
          </div>
          <div class="form-item flex-1">
            <label class="form-label">目标城市</label>
            <el-select v-model="form.city" filterable allow-create default-first-option style="width: 100%">
              <el-option v-for="c in cityOptions" :key="c" :label="c" :value="c" />
            </el-select>
          </div>
        </div>

        <!-- 模块二：全网人才拓扑与配额调度矩阵面板 (Quota Matrix) -->
        <div class="quota-matrix-panel" v-if="isQuotaMatrixActive">
          <div class="quota-header">
            <div class="quota-title-wrap">
              <span class="quota-title">🧬 全网人才拓扑与配额调度矩阵 (Quota Matrix)</span>
              <span class="quota-subtitle">
                自动派生多维度行业同义词，按比例动态分配寻才配额，全局排重归集
              </span>
            </div>
            <div class="quota-actions">
              <el-button
                size="small"
                type="primary"
                plain
                :loading="isGeneratingSynonyms"
                @click="handleGenerateSynonyms"
              >
                <el-icon><Refresh /></el-icon> AI 重新拓词
              </el-button>
              <el-button size="small" @click="addCustomSynonym">
                <el-icon><Plus /></el-icon> 添加维度词
              </el-button>
              <el-button size="small" @click="normalizeRatios">
                <el-icon><ScaleToOriginal /></el-icon> 均摊/归一比例
              </el-button>
            </div>
          </div>

          <!-- 配额公式汇总卡 -->
          <div class="quota-formula-banner">
            <el-icon class="formula-icon"><Histogram /></el-icon>
            <div class="formula-content">
              <span class="formula-label">当前调度公式：</span>
              <strong class="formula-text">{{ quotaFormulaText }}</strong>
              <span v-if="totalRatio !== 100" class="formula-warn">
                (当前比例和为 {{ totalRatio }}%，建议归一)
              </span>
            </div>
          </div>

          <!-- 词条矩阵列表 -->
          <div class="synonym-cards-list">
            <div
              v-for="(item, idx) in synonymList"
              :key="item.id || idx"
              class="synonym-item-card"
              :class="{ disabled: !item.selected }"
            >
              <div class="synonym-col-check">
                <el-checkbox v-model="item.selected" @change="normalizeRatios" />
              </div>

              <div class="synonym-col-category">
                <el-tag size="small" :type="getCategoryTagType(item.category)" effect="dark">
                  {{ item.category_name }}
                </el-tag>
              </div>

              <div class="synonym-col-keyword">
                <el-input
                  v-model="item.keyword"
                  size="small"
                  placeholder="分流词条"
                  :disabled="!item.selected"
                />
                <span class="synonym-desc" :title="item.description">{{ item.description }}</span>
              </div>

              <div class="synonym-col-slider">
                <div class="slider-header">
                  <span class="slider-ratio">{{ item.ratio }}%</span>
                  <span class="slider-count">{{ getKeywordAllocatedCount(item) }} 人 / 全渠道</span>
                </div>
                <el-slider
                  v-model="item.ratio"
                  :min="0"
                  :max="90"
                  :step="5"
                  :disabled="!item.selected"
                  size="small"
                />
              </div>

              <div class="synonym-col-del">
                <el-button
                  type="danger"
                  link
                  size="small"
                  @click="removeSynonym(idx)"
                  title="移除该词条"
                >
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
            </div>
          </div>
        </div>

        <div class="form-row" style="margin-top: 8px;">
          <div class="form-item flex-1">
            <div class="label-with-badge">
              <label class="form-label">全渠道新增目标人数</label>
              <span class="safe-badge">🛡️ 水库保护</span>
            </div>
            <el-select v-model="form.countPerPlatform" style="width: 100%">
              <el-option v-for="n in [5, 10, 15, 20, 30, 50]" :key="n" :label="`${n} 人 / 全渠道`" :value="n" />
            </el-select>
            <span class="reservoir-hint" v-if="form.countPerPlatform > 10">
              🛡️ 自动阶梯拆解：单批上限锁定 10 人，批次间注入高斯拟人微步呼吸停顿 (2.5s ± 800ms) 防风控
            </span>
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
            检索分批执行；识别到安全验证时暂停并提示人工处理。平台限制仍可能变化，请留意实际页面状态。
          </span>
        </div>
      </div>

      <!-- 搜索进行中与日志展示区 -->
      <div v-else class="searching-dashboard">
        <!-- 🚨 验证码拦截紧急唤醒横幅 -->
        <div v-if="captchaAlert.active" class="captcha-emergency-card">
          <div class="emergency-left">
            <el-icon class="emergency-icon pulse"><WarningFilled /></el-icon>
            <div class="emergency-text">
              <div class="emergency-title">
                ⚠️ 【{{ captchaAlert.platformName }}】检测到平台安全验证 · 自动化引擎已安全挂起
              </div>
              <div class="emergency-sub">
                系统已将该平台 Edge 浏览器窗口激活置顶，请在浏览器中完成滑动拼图或点选。滑动通过后系统将<strong>无缝自动恢复抓取</strong>！
              </div>
            </div>
          </div>
          <div class="emergency-actions">
            <el-button size="small" type="warning" @click="handleActivateBrowser(captchaAlert.platform)">
              唤出浏览器窗口
            </el-button>
          </div>
        </div>

        <div class="progress-wrap">
          <div class="progress-info">
            <span class="status-title">{{ currentStatusText }}</span>
            <span class="progress-num">{{ candidateCount }} / {{ expectedTotalCount }} 人</span>
          </div>
          <el-progress
            :percentage="Math.min(100, Math.round((candidateCount / Math.max(1, expectedTotalCount)) * 100))"
            :status="isFinished && candidateCount >= expectedTotalCount ? 'success' : ''"
            :indeterminate="searching && candidateCount === 0"
            :stroke-width="10"
            striped
            striped-flow
          />
          <div class="silent-sandbox-badge" v-if="searching">
            <span class="silent-dot"></span>
            <span>🛡️ 常用 Edge 后台检索中 · 详情页读取后会再次最小化窗口；需要登录或验证时会显示浏览器</span>
          </div>
        </div>

        <!-- 当前各招聘平台状态实时指示条 -->
        <div class="active-platforms-bar">
          <div
            v-for="p in platformList.filter(p => p.selected)"
            :key="p.code"
            class="active-plat-pill"
            :class="{
              'is-active': activePlatformCode === p.code,
              'is-done': isFinished && (platformCounts[p.code] || 0) > 0
            }"
          >
            <span class="plat-pill-icon">{{ p.icon }}</span>
            <span class="plat-pill-name">{{ p.name }}</span>
            <span class="plat-pill-count">
              <el-icon v-if="activePlatformCode === p.code && searching" class="spin"><Loading /></el-icon>
              <el-icon v-else-if="isFinished && (platformCounts[p.code] || 0) > 0" class="icon-done"><CircleCheck /></el-icon>
              {{ platformCounts[p.code] || 0 }} 人
            </span>
          </div>
        </div>

        <!-- 运行记录默认收起，主界面只显示进度和异常提醒 -->
        <details class="search-log-details">
          <summary>查看运行记录</summary>
          <div class="search-log-list" ref="logBoxRef">
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
        </details>
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <!-- 阶段一：表单配置与平台选择阶段 -->
        <template v-if="!isSearchingStarted">
          <el-button @click="visible = false">取消</el-button>
          <el-button
            :type="isContinue ? 'warning' : 'primary'"
            @click="handleStartSearch"
            :disabled="!form.keyword.trim() || selectedPlatformCodes.length === 0"
          >
            <el-icon><component :is="isContinue ? Plus : Search" /></el-icon>
            {{ isContinue ? `立即启动继续寻才 (已选 ${selectedPlatformCodes.length} 个平台 · 自动排重)` : `启动矩阵并发检索 (已选 ${selectedPlatformCodes.length} 个平台)` }}
          </el-button>
        </template>

        <!-- 阶段二：检索进行中或已完成阶段 -->
        <template v-else>
          <el-button v-if="searching" type="danger" plain @click="handleStop">停止检索</el-button>
          <el-button v-if="!searching && !isFinished" @click="handleBackToForm">
            <el-icon><Back /></el-icon> 返回修改配置
          </el-button>
          <el-button
            v-if="!searching && !isFinished"
            type="primary"
            @click="handleStartSearch"
            :disabled="!form.keyword.trim() || selectedPlatformCodes.length === 0"
          >
            <el-icon><Search /></el-icon>
            重新启动检索
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
        </template>
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
  RefreshRight,
  Plus,
  Back,
  Opportunity,
  Histogram,
  ScaleToOriginal,
  Delete,
  Refresh
} from '@element-plus/icons-vue'

const props = defineProps<{
  modelValue: boolean
  projectId: string
  projectName?: string
  jobTitle?: string
  jobDescription?: string
  expYears?: number
  eduLevel?: string
  isContinue?: boolean  // true=继续寻才模式，自动跳过已有候选人
}>()

interface SynonymMatrixItem {
  id?: string
  keyword: string
  category: string
  category_name: string
  description: string
  ratio: number
  selected: boolean
}

const isQuotaMatrixActive = ref(false)
const isGeneratingSynonyms = ref(false)
const synonymList = ref<SynonymMatrixItem[]>([])

function getCategoryTagType(cat: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  switch (cat) {
    case 'standard': return ''
    case 'high_level': return 'danger'
    case 'derived': return 'success'
    case 'abbreviation': return 'warning'
    case 'custom': return 'info'
    default: return ''
  }
}

async function handleToggleQuotaMatrix() {
  isQuotaMatrixActive.value = !isQuotaMatrixActive.value
  if (isQuotaMatrixActive.value && synonymList.value.length === 0) {
    await handleGenerateSynonyms()
  }
}

async function handleGenerateSynonyms() {
  const kw = form.keyword.trim() || props.jobTitle || '临床项目经理'
  isGeneratingSynonyms.value = true
  isQuotaMatrixActive.value = true
  try {
    let WailsApp: any = null
    try { WailsApp = await import('../../wailsjs/go/main/App') } catch {}
    if (WailsApp && WailsApp.GenerateJobSynonyms) {
      const items = await WailsApp.GenerateJobSynonyms(kw, props.jobDescription || '')
      if (Array.isArray(items) && items.length > 0) {
        synonymList.value = items.map((it: any, idx: number) => ({
          id: `syn_${Date.now()}_${idx}`,
          keyword: it.keyword,
          category: it.category || 'standard',
          category_name: it.category_name || '维度词',
          description: it.description || '',
          ratio: it.ratio || (idx === 0 ? 40 : 30),
          selected: true
        }))
        normalizeRatios()
        ElMessage.success(`AI 成功派生 ${items.length} 个行业同义词维度！`)
        return
      }
    }
    // 回退默认派生
    synonymList.value = [
      { id: 'syn_1', keyword: kw, category: 'standard', category_name: '标准称谓', description: '行业通用标准岗位称呼', ratio: 40, selected: true },
      { id: 'syn_2', keyword: `${kw}总监`, category: 'high_level', category_name: '高阶下探', description: '资深高职级从业者', ratio: 30, selected: true },
      { id: 'syn_3', keyword: `${kw}主管`, category: 'derived', category_name: '业务衍生', description: '具备一线攻坚能力的业务骨干', ratio: 30, selected: true }
    ]
    normalizeRatios()
  } catch (err: any) {
    ElMessage.error(`AI 拓词失败: ${err.message || err}`)
  } finally {
    isGeneratingSynonyms.value = false
  }
}

function addCustomSynonym() {
  synonymList.value.push({
    id: `syn_${Date.now()}`,
    keyword: '',
    category: 'custom',
    category_name: '自定义维度',
    description: 'HR 自定义补充词条',
    ratio: 20,
    selected: true
  })
  normalizeRatios()
}

function removeSynonym(index: number) {
  synonymList.value.splice(index, 1)
  normalizeRatios()
}

function normalizeRatios() {
  const selected = synonymList.value.filter(s => s.selected)
  if (selected.length === 0) return
  const currentTotal = selected.reduce((sum, s) => sum + (Number(s.ratio) || 0), 0)
  if (currentTotal <= 0) {
    const avg = Math.floor(100 / selected.length)
    selected.forEach((s, idx) => {
      s.ratio = idx === selected.length - 1 ? 100 - avg * (selected.length - 1) : avg
    })
  } else if (currentTotal !== 100) {
    let acc = 0
    selected.forEach((s, idx) => {
      if (idx === selected.length - 1) {
        s.ratio = Math.max(1, 100 - acc)
      } else {
        const scaled = Math.max(1, Math.round((s.ratio / currentTotal) * 100))
        s.ratio = scaled
        acc += scaled
      }
    })
  }
}

const activeQuotaList = computed(() => {
  return synonymList.value.filter(s => s.selected && s.keyword.trim())
})

const totalRatio = computed(() => {
  return activeQuotaList.value.reduce((sum, s) => sum + (Number(s.ratio) || 0), 0)
})

function getKeywordAllocatedCount(item: SynonymMatrixItem) {
  if (!item.selected || !item.keyword.trim()) return 0
  const items = activeQuotaList.value
  const weights = items.map(s => Math.max(0, Number(s.ratio) || 0))
  const sum = weights.reduce((a, b) => a + b, 0)
  if (!sum) return 0
  const exact = weights.map(w => form.countPerPlatform * w / sum)
  const counts = exact.map(Math.floor)
  const order = exact.map((value, index) => ({ index, fraction: value - counts[index] }))
    .sort((a, b) => b.fraction - a.fraction || a.index - b.index)
  const left = form.countPerPlatform - counts.reduce((a, b) => a + b, 0)
  for (let n = 0; n < left; n++) counts[order[n].index]++
  return counts[items.indexOf(item)] || 0
}

const quotaFormulaText = computed(() => {
  if (!isQuotaMatrixActive.value || activeQuotaList.value.length === 0) {
    return `单一岗位搜索：${form.keyword || '目标岗位'} (全渠道 ${form.countPerPlatform} 人)`
  }
  const parts = activeQuotaList.value.map(item => {
    const cnt = getKeywordAllocatedCount(item)
    return `${item.keyword} (${item.ratio}%: ${cnt}人)`
  })
  return `全渠道总计 ${form.countPerPlatform} 人 = ` + parts.join(' + ')
})

const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'refresh'): void
}>()

const visible = ref(false)
const isSearchingStarted = ref(false) // 区分表单配置页与运行监控页
const searching = ref(false)
const isFinished = ref(false)
const currentStatusText = ref('正在就绪...')
const candidateCount = ref(0)
const activePlatformCode = ref<string>('')
const platformCounts = ref<Record<string, number>>({})
const logBoxRef = ref<HTMLElement | null>(null)

// 验证码安全拦截与声光唤醒状态
const captchaAlert = reactive({
  active: false,
  platform: '',
  platformName: '',
  message: ''
})

// Web Audio API 原生双音频合成提示声（无需任何外部音频依赖，440Hz -> 660Hz 优美和弦）
function playChimeAlert() {
  try {
    const AudioCtx = window.AudioContext || (window as any).webkitAudioContext
    if (!AudioCtx) return
    const ctx = new AudioCtx()
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'sine'
    osc.frequency.setValueAtTime(440, ctx.currentTime) // A4
    osc.frequency.exponentialRampToValueAtTime(660, ctx.currentTime + 0.15) // E5

    gain.gain.setValueAtTime(0.25, ctx.currentTime)
    gain.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 0.45)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start(ctx.currentTime)
    osc.stop(ctx.currentTime + 0.5)
  } catch (e) {}
}

// 唤起指定平台 Edge 浏览器窗口至 Windows 前台（解决滑块或查看详情）
async function handleActivateBrowser(platformCode: string) {
  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch {}
  if (WailsApp && WailsApp.ActivatePlatformBrowser) {
    try {
      const ok = await WailsApp.ActivatePlatformBrowser(platformCode)
      if (ok) {
        ElMessage.success('已将 Edge 浏览器窗口激活至屏幕前台')
        return
      }
    } catch {}
  }
  handleTestPlatform(platformCode, captchaAlert.platformName)
}

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
  countPerPlatform: 30,
  autoAnalyze: true
})

const expectedTotalCount = computed(() => {
  return form.countPerPlatform
})

watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    // 始终先展示配置表单，让用户先选择/确认招聘渠道
    isSearchingStarted.value = false
    searching.value = false
    isFinished.value = false
    candidateCount.value = 0
    activePlatformCode.value = ''
    platformCounts.value = {}
    seenCandidateIds.clear()
    lastStatusMsg = ''
    doneTriggered = false
    captchaAlert.active = false
    searchLogs.value = []

    // 继承第一次寻才的岗位关键词（若已有值则保留，不进行粗暴覆盖）
    if (!form.keyword) {
      form.keyword = props.jobTitle || '临床项目经理'
    }

    if (props.isContinue) {
      currentStatusText.value = '继续寻才模式 — 请选择/确认招聘平台后启动'
    } else {
      currentStatusText.value = '准备就绪'
    }
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
  if (isQuotaMatrixActive.value && activeQuotaList.value.length > 0 && totalRatio.value <= 0) {
    ElMessage.warning('请给至少一个同义词设置大于 0 的配额比例')
    return
  }

  isSearchingStarted.value = true
  searching.value = true
  isFinished.value = false
  candidateCount.value = 0
  activePlatformCode.value = ''
  platformCounts.value = {}
  seenCandidateIds.clear()
  lastStatusMsg = ''
  doneTriggered = false
  captchaAlert.active = false
  searchLogs.value = []
  currentStatusText.value = '正在启动矩阵式检索引擎...'

  if (WailsApp?.StartSearchWithOptions) {
    const matrixPayload = isQuotaMatrixActive.value ? activeQuotaList.value.map(item => ({
      keyword: item.keyword.trim(), category: item.category,
      category_name: item.category_name, ratio: item.ratio
    })) : []
    addLog('status', `全渠道总目标 ${form.countPerPlatform} 人；城市 ${form.city}；筛选信息不明的卡片将跳过。`)
    try {
      const started = await WailsApp.StartSearchWithOptions(
        props.projectId, form.keyword.trim(), form.city,
          props.expYears ?? 0, props.eduLevel || '不限', form.countPerPlatform,
        plats, JSON.stringify(matrixPayload), form.autoAnalyze
      )
      if (!started) throw new Error('搜索进程未启动')
    } catch (err: any) {
      searching.value = false
      addLog('error', `启动失败: ${err.message || err}`)
      ElMessage.error('启动搜寻失败')
    }
    return
  }

  if (isQuotaMatrixActive.value && activeQuotaList.value.length > 0) {
    const matrixPayload = activeQuotaList.value.map(item => ({
      keyword: item.keyword.trim(),
      category: item.category,
      category_name: item.category_name,
      description: item.description,
      ratio: item.ratio
    }))

    addLog('status', `🧬 启用同义词拓扑与配额调度矩阵：${quotaFormulaText.value}`)
    addLog('status', `🚀 启动多平台配额寻才：[${form.city}]，调度平台：${plats.join('、')}，单平台总配额 ${form.countPerPlatform} 人`)

    if (WailsApp && WailsApp.StartQuotaMatrixSearch) {
      try {
        await WailsApp.StartQuotaMatrixSearch(
          props.projectId,
          form.keyword.trim(),
          form.city,
          props.expYears ?? 0,
          props.eduLevel || '不限',
          form.countPerPlatform,
          plats,
          JSON.stringify(matrixPayload)
        )
      } catch (err: any) {
        searching.value = false
        addLog('error', `启动失败: ${err.message || err}`)
        ElMessage.error('启动配额矩阵搜寻失败')
      }
      return
    }
  }

  addLog('status', `🚀 启动多平台聚合寻才：[${form.city}] 岗位「${form.keyword}」，调度平台：${plats.join('、')}，单平台目标 ${form.countPerPlatform} 人`)

  if (WailsApp && WailsApp.StartMultiPlatformSearch) {
    try {
      await WailsApp.StartMultiPlatformSearch(
        props.projectId,
        form.keyword.trim(),
        form.city,
          props.expYears ?? 0,
          props.eduLevel || '不限',
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
          props.expYears ?? 0,
          props.eduLevel || '不限',
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

// 返回修改平台与参数配置
function handleBackToForm() {
  searching.value = false
  isSearchingStarted.value = false
}

async function handleContinueSearch() {
  emit('refresh')
  ElMessage.info(`正在启动下一批检索（全渠道总目标 ${form.countPerPlatform} 人，自动跳过已有简历）...`)
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
    if (evt) {
      if (evt.platform) activePlatformCode.value = evt.platform
      if (evt.message) {
        if (lastStatusMsg === evt.message) return // 防重打印
        lastStatusMsg = evt.message
        currentStatusText.value = evt.message
        addLog('status', evt.message)
      }
    }
  }

  const handleCandidate = (evt: any) => {
    if (evt && evt.candidate) {
      const c = evt.candidate
      const candKey = c.id || `${c.platform}_${c.name}_${c.fileName}`
      if (seenCandidateIds.has(candKey)) return // 排重，根绝翻倍
      seenCandidateIds.add(candKey)

      if (c.platform) {
        platformCounts.value[c.platform] = (platformCounts.value[c.platform] || 0) + 1
      }

      candidateCount.value = seenCandidateIds.size
      const pName = c.platformName || evt.platformName || '招聘平台'
      const kwTag = c.sourceKeyword ? ` [派生: ${c.sourceKeyword}]` : ''
      addLog('candidate', `👤 [${pName}]${kwTag} 成功提取牛人: ${c.name}（${c.experience} · ${c.company || '在线履历'}）`)
      emit('refresh')
    }
  }

  const handleDone = (evt: any) => {
    if (doneTriggered) return
    doneTriggered = true
    captchaAlert.active = false
    searching.value = false
    isFinished.value = true
    const reachedTarget = candidateCount.value >= expectedTotalCount.value
    const shortfallReason = String(evt?.shortfallReason || '').trim()
    currentStatusText.value = reachedTarget
      ? (form.autoAnalyze ? '全渠道检索完成，正在启动 AI 分析' : '全渠道检索完成')
      : `检索结束，已导入 ${candidateCount.value}/${expectedTotalCount.value} 人${shortfallReason ? `；${shortfallReason}` : ''}`
    addLog(reachedTarget ? 'done' : 'status', evt.message || currentStatusText.value)
    if (reachedTarget) {
      ElMessage.success(`多平台候选人采集完成 (共 ${candidateCount.value} 人)${form.autoAnalyze ? '，正在进行 AI 智能打分' : ''}`)
    } else {
      ElMessage.warning(`本次找到 ${candidateCount.value}/${expectedTotalCount.value} 位人选。${shortfallReason || '可调整关键词或平台继续搜索'}`)
    }
    emit('refresh')
  }

  const handleError = (evt: any) => {
    if (!evt?.platform) searching.value = false
    currentStatusText.value = '检索提示'
    addLog('error', evt.message || '检索过程中发生提示')
    ElMessage.error(evt.message || '操作未完成')
  }

  const handleCaptcha = (evt: any) => {
    if (evt) {
      captchaAlert.active = true
      captchaAlert.platform = evt.platform || ''
      captchaAlert.platformName = evt.platformName || evt.platform || '招聘平台'
      captchaAlert.message = evt.message || '检测到平台安全验证，请在浏览器中完成验证'
      playChimeAlert()
      addLog('error', evt.message || '⚠️ 检测到平台安全验证，自动化已挂起')
    }
  }

  const handleCaptchaResolved = (evt: any) => {
    if (evt) {
      captchaAlert.active = false
      addLog('done', evt.message || '🎉 安全验证已通过，自动化已恢复运转')
    }
  }

  // 仅监听统一平台事件，彻底切断与旧 boss:* 兼容事件的双发重叠
  const offPlatformStatus = WailsRuntime.EventsOn('platform:status', handleStatus)
  const offPlatformCandidate = WailsRuntime.EventsOn('platform:candidate_found', handleCandidate)
  const offPlatformDone = WailsRuntime.EventsOn('platform:done', handleDone)
  const offPlatformError = WailsRuntime.EventsOn('platform:error', handleError)
  const offPlatformCaptcha = WailsRuntime.EventsOn('platform:captcha', handleCaptcha)
  const offPlatformCaptchaResolved = WailsRuntime.EventsOn('platform:captcha_resolved', handleCaptchaResolved)

  unsubscribeList = [
    offPlatformStatus, offPlatformCandidate, offPlatformDone, offPlatformError, offPlatformCaptcha, offPlatformCaptchaResolved
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

  .continue-mode-banner {
    background: linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%);
    border: 1px solid #fde68a;
    border-radius: $radius-md;
    padding: 10px 14px;
    display: flex;
    flex-direction: column;
    gap: 4px;

    .banner-badge {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      font-size: 13px;
      font-weight: 700;
      color: #b45309;
    }

    .banner-desc {
      font-size: 12px;
      color: #92400e;
      line-height: 1.5;

      strong {
        color: #78350f;
      }
    }
  }

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

  .label-with-action {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .ai-synonym-trigger-btn {
    border: none;
    background: transparent;
    color: #007aff;
    font-size: 11.5px;
    font-weight: 600;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 6px;
    border-radius: 4px;
    transition: all 0.2s ease;

    &:hover {
      background: #eff6ff;
      color: #0056b3;
    }

    &.active {
      background: #dbeafe;
      color: #1d4ed8;
    }

    .spin {
      animation: rotating 1.5s linear infinite;
    }
  }

  .quota-matrix-panel {
    margin-top: 10px;
    background: #f8fafc;
    border: 1px solid #e2e8f0;
    border-radius: 8px;
    padding: 12px;

    .quota-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      margin-bottom: 10px;

      .quota-title-wrap {
        display: flex;
        flex-direction: column;
        gap: 2px;

        .quota-title {
          font-size: 13px;
          font-weight: 700;
          color: #1e293b;
        }

        .quota-subtitle {
          font-size: 11px;
          color: #64748b;
        }
      }

      .quota-actions {
        display: flex;
        gap: 6px;
      }
    }

    .quota-formula-banner {
      display: flex;
      align-items: center;
      gap: 8px;
      background: #eff6ff;
      border: 1px solid #bfdbfe;
      border-radius: 6px;
      padding: 8px 12px;
      margin-bottom: 10px;

      .formula-icon {
        font-size: 16px;
        color: #2563eb;
        flex-shrink: 0;
      }

      .formula-content {
        font-size: 12px;
        color: #1e40af;
        line-height: 1.4;

        .formula-label {
          color: #475569;
        }

        .formula-text {
          font-weight: 700;
          color: #1d4ed8;
        }

        .formula-warn {
          color: #dc2626;
          font-size: 11px;
          margin-left: 6px;
        }
      }
    }

    .synonym-cards-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
      max-height: 220px;
      overflow-y: auto;
      padding-right: 4px;

      .synonym-item-card {
        display: flex;
        align-items: center;
        gap: 10px;
        background: #ffffff;
        border: 1px solid #e2e8f0;
        border-radius: 6px;
        padding: 8px 10px;
        transition: all 0.2s ease;

        &:hover {
          border-color: #cbd5e1;
          box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
        }

        &.disabled {
          opacity: 0.5;
          background: #f1f5f9;
        }

        .synonym-col-check {
          flex-shrink: 0;
        }

        .synonym-col-category {
          flex-shrink: 0;
          min-width: 72px;
        }

        .synonym-col-keyword {
          flex: 1.2;
          display: flex;
          flex-direction: column;
          gap: 2px;

          .synonym-desc {
            font-size: 10.5px;
            color: #94a3b8;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            max-width: 170px;
          }
        }

        .synonym-col-slider {
          flex: 1.5;
          display: flex;
          flex-direction: column;
          gap: 2px;

          .slider-header {
            display: flex;
            justify-content: space-between;
            font-size: 11px;

            .slider-ratio {
              font-weight: 700;
              color: #2563eb;
            }

            .slider-count {
              color: #64748b;
            }
          }
        }

        .synonym-col-del {
          flex-shrink: 0;
        }
      }
    }
  }

  .label-with-badge {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 4px;

    .safe-badge {
      font-size: 11px;
      color: #059669;
      background: #ecfdf5;
      border: 1px solid #a7f3d0;
      padding: 1px 6px;
      border-radius: 10px;
      font-weight: 600;
    }
  }

  .reservoir-hint {
    display: block;
    margin-top: 4px;
    font-size: 11px;
    color: #059669;
    line-height: 1.35;
    background: #f0fdf4;
    padding: 3px 8px;
    border-radius: 4px;
    border-left: 2px solid #10b981;
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

  .captcha-emergency-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: #fffbeb;
    border: 2px solid #f59e0b;
    border-radius: $radius-md;
    padding: 12px 16px;
    box-shadow: 0 4px 12px rgba(245, 158, 11, 0.2);
    animation: alert-pulse 1.8s infinite ease-in-out;

    .emergency-left {
      display: flex;
      align-items: flex-start;
      gap: 12px;

      .emergency-icon {
        font-size: 26px;
        color: #d97706;
        margin-top: 2px;
        flex-shrink: 0;
      }

      .emergency-text {
        .emergency-title {
          font-size: 13.5px;
          font-weight: 700;
          color: #92400e;
          margin-bottom: 3px;
        }

        .emergency-sub {
          font-size: 12px;
          color: #b45309;
          line-height: 1.4;

          strong {
            color: #78350f;
          }
        }
      }
    }

    .emergency-actions {
      flex-shrink: 0;
      margin-left: 12px;
    }
  }

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

    .silent-sandbox-badge {
      margin-top: 8px;
      display: inline-flex;
      align-items: center;
      gap: 6px;
      font-size: 11.5px;
      color: #0d9488;
      background: #f0fdfa;
      border: 1px solid #ccfbf1;
      padding: 3px 10px;
      border-radius: 4px;

      .silent-dot {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: #14b8a6;
        box-shadow: 0 0 6px #14b8a6;
        animation: silent-blink 1.5s infinite ease-in-out;
      }
    }
  }

  .active-platforms-bar {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;

    .active-plat-pill {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 5px 12px;
      background: #f8fafc;
      border: 1px solid #e2e8f0;
      border-radius: 20px;
      font-size: 12px;
      color: #64748b;
      transition: all 0.25s ease;

      .plat-pill-icon {
        font-size: 13px;
      }

      .plat-pill-name {
        font-weight: 600;
      }

      .plat-pill-count {
        display: inline-flex;
        align-items: center;
        gap: 3px;
        font-size: 11px;
        background: rgba(0, 0, 0, 0.05);
        padding: 1px 6px;
        border-radius: 10px;
      }

      &.is-active {
        background: #eff6ff;
        border-color: #3b82f6;
        color: #1d4ed8;
        box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.15);

        .plat-pill-count {
          background: #dbeafe;
          color: #1d4ed8;
          font-weight: 700;
        }
      }

      &.is-done {
        background: #f0fdf4;
        border-color: #22c55e;
        color: #15803d;

        .plat-pill-count {
          background: #dcfce7;
          color: #15803d;
          font-weight: 700;
        }

        .icon-done {
          color: #22c55e;
        }
      }
    }
  }

  .search-log-details {
    color: #64748b;
    font-size: 12px;
    summary {
      width: fit-content;
      cursor: pointer;
      user-select: none;
    }
  }

  .search-log-list {
    max-height: 190px;
    overflow-y: auto;
    margin-top: 8px;
    padding: 10px 12px;
    background: #f8fafc;
    border: 1px solid #e2e8f0;
    border-radius: $radius-md;
    display: flex;
    flex-direction: column;
    gap: 8px;

    .log-item {
      display: flex;
      align-items: baseline;
      gap: 8px;
      line-height: 1.4;

      .log-time {
        color: #94a3b8;
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
        color: #475569;
      }
      &.candidate {
        color: #15803d;
      }
      &.done {
        color: #1d4ed8;
        font-weight: bold;
      }
      &.error {
        color: #dc2626;
      }
    }
  }
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@keyframes alert-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(245, 158, 11, 0.45);
    border-color: #f59e0b;
  }
  50% {
    box-shadow: 0 0 0 10px rgba(245, 158, 11, 0);
    border-color: #d97706;
  }
  100% {
    box-shadow: 0 0 0 0 rgba(245, 158, 11, 0.45);
    border-color: #f59e0b;
  }
}

@keyframes silent-blink {
  0%, 100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.35;
    transform: scale(0.85);
  }
}
</style>
