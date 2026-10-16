<template>
  <Transition name="card-slide" appear>
    <div
      class="resume-card"
      :class="{
        'is-selected': selected,
        'is-done': resume.status === 'done',
        'is-pending': resume.status === 'pending',
        'is-analyzing': resume.status === 'analyzing',
        'is-error': resume.status === 'error'
      }"
      @click="$emit('click')"
    >
      <div class="card-header">
        <span class="sequence-badge" :aria-label="`列表第 ${sequence} 位`">{{ sequence }}</span>
        <div class="card-heading">
          <span class="file-name" :title="resume.fileName">{{ resume.fileName }}</span>
          <span class="file-meta">{{ sourceText }} · {{ fileTypeText }} · {{ statusText }}</span>
          <span v-if="resume.snapshot_copy_ids?.length" class="file-meta" title="原始文件与历史评测保留；同文副本不重复参与排名。平台 ID 冲突的记录不会自动折叠。">已折叠 {{ resume.snapshot_copy_ids.length }} 份相同快照</span>
        </div>
      </div>

      <div v-if="showTags" class="card-tags">
        <span v-if="(resume as any).is_merged_analysis || (resume as any).has_attachment" class="merged-badge" title="已完成微简历与完整附件结合终审">
          已合并终审
        </span>
        <span v-if="resume.analysis?.redLineViolations && resume.analysis.redLineViolations.length > 0" class="redline-pill" title="AI 发现红线线索，请由 HR 核实">
          红线线索待核实
        </span>
        <span v-else-if="resume.analysis?.redLineStatus === 'pending'" class="redline-pill" title="红线材料不足，需人工核实">
          红线待核实
        </span>
        <span v-else-if="resume.analysis?.bonusMatches && resume.analysis.bonusMatches.length > 0" class="bonus-pill" title="命中用人部门优先加分项">
          加分达成
        </span>
        <span v-if="(resume as any).source_keyword" class="source-kw-badge" :title="'派生检索词: ' + (resume as any).source_keyword">
          {{ (resume as any).source_keyword }}
        </span>
      </div>

      <div class="card-body">
        <!-- 分析完成状态 -->
        <div v-if="resume.status === 'done'" class="result">
          <div class="score-badge" :class="scoreClass">
            <span class="score-number">{{ displayScore }}</span>
          </div>
          <div class="result-copy">
            <span class="result-caption">能力评估</span>
            <span class="recommendation" :class="recommendationClass">{{ recommendationText }}</span>
          </div>
        </div>

        <!-- 分析中状态 - 带实时进度条 -->
        <div v-else-if="resume.status === 'analyzing'" class="analyzing">
          <div class="analyzing-content">
            <el-icon class="loading"><Loading /></el-icon>
            <span>{{ progressLabel }}</span>
          </div>
          <div class="progress-bar">
            <div class="progress-fill" :style="{ width: actualProgress + '%' }"></div>
          </div>
        </div>

        <!-- 错误状态 -->
        <div v-else-if="resume.status === 'error'" class="error">
          <el-icon><CircleClose /></el-icon>
          <span>{{ failureSummary }}</span>
        </div>

        <!-- 待分析状态 -->
        <div v-else class="pending">
          <el-icon><Clock /></el-icon>
          <span>{{ $t('analysis.pending') }}</span>
        </div>
      </div>

      <div class="card-footer">
        <span class="footer-hint">点击查看评测详情</span>
        <div class="card-actions">
          <button
            v-if="resume.status === 'done' || resume.status === 'error'"
            class="action-btn reanalyze-btn"
            @click.stop="$emit('re-analyze', resume.id)"
          >
            <el-icon><RefreshRight /></el-icon>
            {{ $t('analysis.reAnalyze') }}
          </button>
          <button
            class="action-btn delete-btn"
            @click.stop="$emit('delete', resume.id)"
          >
            {{ $t('common.delete') }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Loading,
  CircleClose,
  Clock,
  RefreshRight
} from '@element-plus/icons-vue'
import type { Resume } from '../composables/useResumeStore'

const { t } = useI18n()

const props = defineProps<{
  resume: Resume
  sequence: number
  selected?: boolean
}>()

const failureSummary = computed(() => {
  const message = props.resume.errorMessage || ''
  if (message.includes('解析AI返回失败')) return 'AI 返回格式错误 · 点击查看详情'
  if (message.includes('余额不足')) return 'AI 服务余额不足 · 点击查看详情'
  if (message.includes('429') || message.includes('请求过于频繁')) return 'AI 服务限流 · 点击查看详情'
  if (message.includes('认证失败') || message.includes('API Key')) return 'AI 配置或认证错误 · 点击查看详情'
  return '分析失败 · 点击查看原因'
})

defineEmits<{
  (e: 'click'): void
  (e: 're-analyze', id: string): void
  (e: 'delete', id: string): void
}>()

// 分数动画显示
const displayScore = ref(0)
let animationFrame: number | null = null

// 监听分数变化，触发动画
watch(
  () => props.resume.score,
  (newScore) => {
    if (newScore && props.resume.status === 'done') {
      animateScore(newScore)
    }
  },
  { immediate: true }
)

// 分数跳动动画
function animateScore(targetScore: number) {
  const startScore = displayScore.value
  const startTime = performance.now()
  const duration = 800 // 动画时长 800ms

  function update(currentTime: number) {
    const elapsed = currentTime - startTime
    const progress = Math.min(elapsed / duration, 1)
    
    // 使用 easeOutQuart 缓动函数
    const easeProgress = 1 - Math.pow(1 - progress, 4)
    displayScore.value = Math.round(startScore + (targetScore - startScore) * easeProgress)
    
    if (progress < 1) {
      animationFrame = requestAnimationFrame(update)
    }
  }

  if (animationFrame) {
    cancelAnimationFrame(animationFrame)
  }
  animationFrame = requestAnimationFrame(update)
}

onMounted(() => {
  if (props.resume.score && props.resume.status === 'done') {
    displayScore.value = props.resume.score
  }
})

// 文件类型文本
const fileTypeText = computed(() => {
  const ext = props.resume.fileType.toLowerCase()
  if (ext === '.pdf') return t('file.pdf')
  if (ext === '.docx' || ext === '.doc') return t('file.word')
  if (['.jpg', '.jpeg', '.png', '.bmp', '.gif', '.webp'].includes(ext)) return t('file.image')
  if (ext === '.txt') return '微简历 / TXT'
  return t('file.unknown')
})

const statusText = computed(() => {
  if (props.resume.status === 'done') return '已完成'
  if (props.resume.status === 'analyzing') return '分析中'
  if (props.resume.status === 'error') return '失败'
  return '待分析'
})

const sourceText = computed(() => {
  const match = props.resume.fileName.match(/^【([^】]+)】/)
  if (match) return match[1]
  try {
    const host = new URL(props.resume.url || '').hostname
    if (host.endsWith('51job.com')) return '前程无忧'
    if (host.endsWith('zhipin.com')) return 'BOSS直聘'
    if (host.endsWith('zhaopin.com')) return '智联招聘'
    if (host.endsWith('liepin.com')) return '猎聘'
  } catch {}
  return '本地导入'
})

const showTags = computed(() => Boolean(
  props.resume.is_merged_analysis || props.resume.has_attachment ||
  props.resume.analysis?.redLineViolations?.length ||
  props.resume.analysis?.redLineStatus === 'pending' ||
  props.resume.analysis?.bonusMatches?.length ||
  props.resume.source_keyword
))

// 分数等级
const scoreClass = computed(() => {
  const score = props.resume.score || 0
  if (score >= 80) return 'high'
  if (score >= 60) return 'medium'
  return 'low'
})

// 推荐等级文本
const recommendationText = computed(() => {
  if (!props.resume.analysis) return ''
  const rec = props.resume.analysis.recommendation
  return t(`analysis.recommendations.${rec}`)
})

// 实际分析进度 (从 resume 对象动态读取)
const actualProgress = computed(() => {
  const p = (props.resume as any).progress
  return typeof p === 'number' ? Math.min(p, 100) : 0
})

// 进度阶段文本
const progressLabel = computed(() => {
  const p = actualProgress.value
  if (p <= 10) return t('analysis.analyzing') + '...'
  if (p <= 30) return t('analysis.preparingPrompt') || '准备分析...'
  if (p <= 70) return t('analysis.callingAI') || 'AI 分析中...'
  if (p <= 84) return t('analysis.parsingResult') || '解析结果...'
  if (p < 100) return t('analysis.extractingCompetencies') || '提炼履历能力雷达...'
  return t('analysis.done') || '已完成'
})

// 推荐等级样式
const recommendationClass = computed(() => {
  if (!props.resume.analysis) return ''
  const rec = props.resume.analysis.recommendation
  if (rec === 'strong_recommend' || rec === 'recommend') return 'positive'
  if (rec === 'not_recommend') return 'negative'
  return 'neutral'
})
</script>

<style scoped lang="scss">
// ==========================================
// 设计系统变量
// ==========================================
$primary: #1e3a5f;
$primary-light: #2d5a87;
$primary-lighter: #e8f0f7;
$accent: #0ea5e9;
$accent-light: #7dd3fc;

$bg-card: #ffffff;
$border: #e2e7ed;
$divider: #eef1f5;

$text-primary: #1a2332;
$text-secondary: #5a6678;
$text-muted: #8b95a5;

$success: #16a34a;
$success-dark: #15803d;
$success-light: #dcfce7;
$warning: #d97706;
$warning-dark: #b45309;
$warning-light: #fef3c7;
$error: #dc2626;
$error-dark: #b91c1c;
$error-light: #fee2e2;

$shadow-sm: 0 1px 3px rgba(30, 58, 95, 0.06);
$shadow-md: 0 4px 12px rgba(30, 58, 95, 0.08);

$radius-sm: 6px;
$radius-md: 10px;

// ==========================================
// 卡片入场动画
// ==========================================
.card-slide-enter-active {
  animation: slideIn 0.4s ease-out;
}

.card-slide-leave-active {
  animation: slideOut 0.3s ease-in;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateY(-20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes slideOut {
  from {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
  to {
    opacity: 0;
    transform: translateY(10px) scale(0.95);
  }
}

// ==========================================
// 简历卡片
// ==========================================
.resume-card {
  padding: 14px 16px;
  border: 1px solid $border;
  border-radius: $radius-md;
  margin-bottom: 10px;
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  background-color: $bg-card;
  position: relative;
  overflow: hidden;

  &:hover {
    box-shadow: $shadow-md;
    border-color: rgba(30, 58, 95, 0.15);
    transform: translateY(-2px);
  }

  &.is-selected {
    border-color: $primary;
    background-color: $primary-lighter;
    box-shadow: 0 0 0 3px rgba(30, 58, 95, 0.08);

    .file-name {
      color: $primary;
    }
  }

  // 左侧状态指示条
  &.is-done {
    border-left: 4px solid $success;

    &::before {
      content: '';
      position: absolute;
      top: 0;
      left: -1px;
      bottom: 0;
      width: 4px;
      background: linear-gradient(180deg, $success, $success-dark);
      border-radius: 4px 0 0 4px;
    }
  }

  &.is-pending {
    border-left: 4px solid $text-muted;
  }

  &.is-error {
    border-left: 4px solid $error;
  }

  &.is-analyzing {
    border-left: 4px solid $accent;

    // 呼吸灯边框效果
    &::after {
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      border: 2px solid $accent;
      border-radius: $radius-md;
      animation: breathing 2s ease-in-out infinite;
      pointer-events: none;
    }
  }

  // 分析中的发光效果层
  .analyzing-glow {
    position: absolute;
    top: -50%;
    left: -50%;
    right: -50%;
    bottom: -50%;
    background: linear-gradient(
      45deg,
      transparent 30%,
      rgba(14, 165, 233, 0.1) 50%,
      transparent 70%
    );
    animation: glowSweep 2s ease-in-out infinite;
    pointer-events: none;
  }

  // ==========================================
  // 卡片头部
  // ==========================================
  .card-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 10px;
    position: relative;
    z-index: 1;

    .file-icon {
      font-size: 22px;
      color: $primary-light;
      transition: transform 0.2s ease;
    }

    .file-name {
      flex: 1;
      font-size: 14px;
      font-weight: 600;
      color: $text-primary;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      letter-spacing: 0.2px;
    }

    .merged-badge {
      font-size: 10px;
      font-weight: 600;
      color: #b45309;
      background: #fef3c7;
      border: 1px solid #fde68a;
      padding: 1px 6px;
      border-radius: 10px;
      flex-shrink: 0;
    }

    .redline-pill {
      font-size: 10px;
      font-weight: 600;
      color: #dc2626;
      background: #fee2e2;
      border: 1px solid #fca5a5;
      padding: 1px 6px;
      border-radius: 10px;
      flex-shrink: 0;
    }

    .bonus-pill {
      font-size: 10px;
      font-weight: 600;
      color: #ca8a04;
      background: #fef9c3;
      border: 1px solid #fde047;
      padding: 1px 6px;
      border-radius: 10px;
      flex-shrink: 0;
    }

    .source-kw-badge {
      font-size: 10px;
      font-weight: 600;
      color: #0369a1;
      background: #e0f2fe;
      border: 1px solid #bae6fd;
      padding: 1px 6px;
      border-radius: 10px;
      flex-shrink: 0;
      max-width: 90px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  &:hover .file-icon {
    transform: scale(1.1);
  }

  // ==========================================
  // 卡片主体
  // ==========================================
  .card-body {
    margin-bottom: 10px;
    position: relative;
    z-index: 1;

    .result {
      display: flex;
      align-items: center;
      gap: 12px;

      .score-badge {
        width: 44px;
        height: 44px;
        border-radius: 12px;
        display: flex;
        align-items: center;
        justify-content: center;
        color: white;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
        position: relative;
        overflow: hidden;

        // 闪光效果
        &::after {
          content: '';
          position: absolute;
          top: -50%;
          left: -50%;
          width: 200%;
          height: 200%;
          background: linear-gradient(
            45deg,
            transparent 30%,
            rgba(255, 255, 255, 0.3) 50%,
            transparent 70%
          );
          animation: shimmer 3s ease-in-out infinite;
        }

        .score-number {
          font-size: 18px;
          font-weight: 700;
          position: relative;
          z-index: 1;
        }

        &.high {
          background: linear-gradient(135deg, $success, $success-dark);
        }

        &.medium {
          background: linear-gradient(135deg, $warning, $warning-dark);
        }

        &.low {
          background: linear-gradient(135deg, $error, $error-dark);
        }
      }

      .recommendation {
        font-size: 13px;
        font-weight: 600;
        padding: 5px 12px;
        border-radius: $radius-sm;
        animation: fadeIn 0.3s ease-out;

        &.positive {
          color: $success;
          background-color: $success-light;
        }

        &.negative {
          color: $error;
          background-color: $error-light;
        }

        &.neutral {
          color: $warning;
          background-color: $warning-light;
        }
      }
    }

    .analyzing {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 10px 12px;
      background: linear-gradient(135deg, rgba(14, 165, 233, 0.08), rgba(14, 165, 233, 0.04));
      border-radius: $radius-sm;
      border: 1px solid rgba(14, 165, 233, 0.15);

      .analyzing-content {
        display: flex;
        align-items: center;
        gap: 10px;
        color: $accent;
        font-size: 13px;
        font-weight: 500;

        .loading {
          animation: spin 1s linear infinite;
        }
      }

      .progress-bar {
        height: 4px;
        background: rgba(14, 165, 233, 0.15);
        border-radius: 2px;
        overflow: hidden;

        .progress-fill {
          height: 100%;
          min-width: 4%;
          background: linear-gradient(90deg, $accent, $accent-light);
          border-radius: 2px;
          transition: width 0.6s cubic-bezier(0.4, 0, 0.2, 1);
        }
      }
    }

    .error {
      display: flex;
      align-items: center;
      gap: 10px;
      color: $error;
      font-size: 13px;
      font-weight: 500;
      padding: 10px 12px;
      background-color: $error-light;
      border-radius: $radius-sm;
      animation: shake 0.5s ease-out;
    }

    .pending {
      display: flex;
      align-items: center;
      gap: 10px;
      color: $text-muted;
      font-size: 13px;
      padding: 10px 12px;
      background-color: rgba(139, 149, 165, 0.08);
      border-radius: $radius-sm;
    }
  }

  // ==========================================
  // 卡片底部
  // ==========================================
  .card-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
    color: $text-muted;
    padding-top: 10px;
    border-top: 1px solid $divider;
    position: relative;
    z-index: 1;

    .file-type {
      background-color: rgba(30, 58, 95, 0.06);
      padding: 3px 10px;
      border-radius: 4px;
      font-weight: 500;
      color: $text-secondary;
      letter-spacing: 0.3px;
    }

    .card-actions {
      display: flex;
      align-items: center;
      gap: 10px;

      .action-btn {
        border: none;
        background: none;
        cursor: pointer;
        font-size: 12px;
        font-weight: 500;
        font-family: inherit;
        transition: all 0.2s ease;
        border-radius: 4px;
        padding: 3px 8px;
      }

      .reanalyze-btn {
        display: flex;
        align-items: center;
        gap: 4px;
        color: $primary;
        background: rgba(30, 58, 95, 0.06);
        border: 1px solid rgba(30, 58, 95, 0.15);

        .el-icon { font-size: 13px; }

        &:hover {
          background: rgba(30, 58, 95, 0.12);
          border-color: $primary-light;
        }
        &:active { transform: scale(0.96); }
      }

      .delete-btn {
        color: $error;

        &:hover { background: $error-light; }
        &:active { transform: scale(0.96); }
      }
    }
  }
}

// ==========================================
// 动画
// ==========================================
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes breathing {
  0%, 100% {
    opacity: 0.3;
    transform: scale(1);
  }
  50% {
    opacity: 0.6;
    transform: scale(1.01);
  }
}

@keyframes glowSweep {
  0% {
    transform: translateX(-100%) rotate(45deg);
  }
  100% {
    transform: translateX(100%) rotate(45deg);
  }
}

@keyframes shimmer {
  0% {
    transform: translateX(-100%) rotate(45deg);
  }
  50%, 100% {
    transform: translateX(100%) rotate(45deg);
  }
}

@keyframes progressWave {
  0% {
    width: 20%;
    transform: translateX(-100%);
  }
  50% {
    width: 50%;
  }
  100% {
    width: 20%;
    transform: translateX(400%);
  }
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateX(-10px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

// 紧凑评测卡片：用主题变量保持浅色、深色模式一致。
.resume-card {
  padding: 14px;
  margin-bottom: 9px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-left: 3px solid var(--text-muted);
  border-radius: 12px;
  box-shadow: var(--shadow-xs);
  color: var(--text-primary);

  &:hover {
    background: var(--bg-card);
    border-color: var(--accent-blue-subtle);
    box-shadow: var(--shadow-card-hover);
    transform: translateY(-1px);
  }

  &.is-selected {
    background: var(--bg-card);
    border-color: var(--accent-blue);
    box-shadow: 0 0 0 2px var(--accent-blue-subtle);
  }

  &.is-done { border-left-color: var(--color-success); }
  &.is-analyzing { border-left-color: var(--color-info); }
  &.is-error { border-left-color: var(--color-danger); }
  &.is-done::before, &.is-analyzing::after { display: none; }

  .card-header {
    display: flex;
    align-items: center;
    gap: 11px;
    margin-bottom: 8px;
  }

  .sequence-badge {
    width: 34px;
    height: 34px;
    flex: 0 0 34px;
    display: grid;
    place-items: center;
    border-radius: 9px;
    background: var(--accent-blue-light);
    color: var(--accent-blue);
    font-size: 14px;
    font-weight: 750;
    font-variant-numeric: tabular-nums;
  }

  .card-heading {
    display: flex;
    flex: 1;
    min-width: 0;
    flex-direction: column;
    gap: 3px;
  }

  .card-header .file-name {
    display: block;
    width: 100%;
    color: var(--text-primary);
    font-size: 13px;
    font-weight: 700;
    line-height: 1.35;
  }

  .file-meta {
    color: var(--text-muted);
    font-size: 11px;
    line-height: 1.25;
  }

  .card-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
    margin: 0 0 10px 45px;
  }

  .card-tags:empty { display: none; }

  .card-tags > span {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    border-radius: 5px;
    padding: 2px 6px;
    font-size: 10px;
    font-weight: 650;
  }

  .card-tags .redline-pill { color: var(--color-danger); background: var(--color-danger-bg); border: 0; }
  .card-tags .bonus-pill { color: var(--color-warning); background: var(--color-warning-bg); border: 0; }
  .card-tags .merged-badge { color: var(--color-success); background: var(--color-success-bg); border: 0; }
  .card-tags .source-kw-badge { color: var(--accent-blue); background: var(--accent-blue-light); border: 0; max-width: 125px; }

  .card-body {
    margin-bottom: 10px;
  }

  .card-body .result { gap: 10px; }

  .card-body .result .score-badge {
    width: 43px;
    height: 43px;
    border-radius: 9px;
    box-shadow: none;
  }

  .card-body .result .score-badge::after { display: none; }

  .result-copy {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 3px;
  }

  .result-caption { color: var(--text-muted); font-size: 10px; }

  .card-body .result .recommendation {
    padding: 0;
    background: transparent;
    font-size: 12px;
  }

  .card-body .result .recommendation.positive { color: var(--color-success); }
  .card-body .result .recommendation.neutral { color: var(--color-warning); }
  .card-body .result .recommendation.negative { color: var(--color-danger); }

  .card-body .analyzing, .card-body .error, .card-body .pending {
    padding: 8px 10px;
    border-radius: 7px;
    font-size: 12px;
  }

  .card-body .analyzing { background: var(--accent-blue-light); border-color: var(--accent-blue-subtle); }
  .card-body .error { background: var(--color-danger-bg); color: var(--color-danger); }
  .card-body .pending { background: var(--bg-card-secondary); color: var(--text-secondary); }

  .card-footer {
    padding-top: 9px;
    border-top-color: var(--border-color-light);
  }

  .footer-hint { color: var(--text-muted); font-size: 11px; }

  .card-footer .card-actions .reanalyze-btn {
    color: var(--accent-blue);
    background: var(--accent-blue-light);
    border-color: var(--accent-blue-subtle);
  }

  .card-footer .card-actions .delete-btn { color: var(--color-danger); }
  .card-footer .card-actions .delete-btn:hover { background: var(--color-danger-bg); }
}

@keyframes shake {
  0%, 100% { transform: translateX(0); }
  20% { transform: translateX(-4px); }
  40% { transform: translateX(4px); }
  60% { transform: translateX(-4px); }
  80% { transform: translateX(4px); }
}
</style>
