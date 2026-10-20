<template>
  <el-dialog :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" title="HR 初试记录与问答" width="760px" destroy-on-close>
    <div v-if="loading && !session" class="loading-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>正在读取初试会话...</span>
    </div>
    <div v-else-if="session" class="interview-content">
      <p class="hint">
        <strong>{{ session.candidate_name }}</strong> · 围绕工作意向、工作能力与双向确认进行初试把关，回答逐题保存，深度技术由后续专业面试核验。
      </p>

      <!-- 题目步骤指示器（支持点击自由跳转） -->
      <div class="steps-wrapper">
        <el-steps :active="activeTurnIndex" finish-status="success" simple class="interactive-steps">
          <el-step
            v-for="(turn, i) in session.turns"
            :key="i"
            :title="`第 ${i + 1} 题`"
            :status="turn.evaluated ? 'success' : (activeTurnIndex === i ? 'process' : 'wait')"
            @click="activeTurnIndex = i"
          />
        </el-steps>
      </div>

      <!-- 当前题目答题卡 -->
      <template v-if="currentTurn">
        <div class="current-turn-card">
          <div class="turn-meta-bar">
            <el-tag size="small" :type="getCategoryTagType(currentTurn.category)">{{ currentTurn.category || '初试问题' }}</el-tag>
            <span v-if="currentTurn.criterion" class="turn-criterion">【{{ currentTurn.criterion }}】</span>
            <span class="turn-progress-text">第 {{ activeTurnIndex + 1 }} / {{ session.turns.length }} 题</span>
          </div>

          <p class="question-text">
            <strong>{{ isFollowUp ? '💬 追问：' : '❓ 问题：' }}</strong>
            {{ isFollowUp ? currentTurn.follow_up : currentTurn.question }}
          </p>

          <el-input
            v-model="answer"
            type="textarea"
            :rows="5"
            maxlength="4000"
            show-word-limit
            :placeholder="isFollowUp ? '请记录候选人对追问的回答原话...' : '请记录候选人的回答要点或原话；不确定的内容如实记录...'"
          />

          <div class="turn-bottom-actions">
            <div class="nav-btns">
              <el-button size="small" :disabled="activeTurnIndex <= 0" @click="activeTurnIndex--">上一题</el-button>
              <el-button size="small" :disabled="activeTurnIndex >= session.turns.length - 1" @click="activeTurnIndex++">下一题</el-button>
            </div>
            <div class="submit-btns">
              <el-button type="primary" :loading="loading" :disabled="!answer.trim()" @click="submitAnswer">
                {{ isFollowUp ? '提交追问回答并评分' : '提交本题回答' }}
              </el-button>
            </div>
          </div>
        </div>
      </template>

      <!-- 全部回答完毕或中途生成总结操作区 -->
      <div v-if="session.status === 'in_progress'" class="summary-action-box">
        <span class="summary-tip">所有问题均可随时记录；完成后可生成初试 AI 复核与总结。</span>
        <el-button type="success" :loading="loading" @click="complete">
          生成初试总结
        </el-button>
      </div>

      <!-- 初面已完成：展示 AI 总结与 HR 复核做决定 -->
      <div v-else class="completed-summary-box">
        <el-alert title="AI 初试分析已完成，最终决定由 HR 做出" type="success" :closable="false" show-icon />
        <div class="score-summary-row">
          <span><strong>初试能力得分：</strong><span class="score-num">{{ session.overall_score }}</span> 分（工作意向不计入能力得分）</span>
          <span><strong>AI 建议：</strong><el-tag :type="session.ai_recommendation === 'advance' ? 'success' : (session.ai_recommendation === 'hold' ? 'warning' : 'danger')">{{ session.ai_recommendation }}</el-tag></span>
        </div>
        <div class="summary-text-box">
          {{ session.ai_summary }}
        </div>

        <div class="hr-decision-section">
          <h4>HR 初试结论与交接备注：</h4>
          <el-input v-model="hrNote" type="textarea" :rows="2" placeholder="记录 HR 复核结论、需向下一轮专业面试官交接的技术关注点等" />
          <div class="review-actions">
            <el-button type="primary" :loading="loading" @click="review('advance')">推进下一轮专业面试</el-button>
            <el-button type="warning" :loading="loading" @click="review('hold')">存疑待核实</el-button>
            <el-button type="danger" :loading="loading" @click="review('decline')">初试不推进</el-button>
          </div>
          <p v-if="session.hr_decision" class="hr-decision-result">
            当前 HR 决定：<strong>{{ session.hr_decision }}</strong>{{ session.hr_note ? ` · 备注：${session.hr_note}` : '' }}
          </p>
        </div>
      </div>

      <el-divider />

      <!-- 已回答列表折叠区 -->
      <details class="history-details">
        <summary>查看已提交的问答记录与单题评价 ({{ answeredCount }}/{{ session.turns.length }})</summary>
        <div v-for="(turn, i) in session.turns" :key="i" class="history-item">
          <div class="history-head">
            <span class="history-idx">第 {{ i + 1 }} 题</span>
            <el-tag size="small" :type="getCategoryTagType(turn.category)">{{ turn.category }}</el-tag>
            <span class="history-q">{{ turn.question }}</span>
          </div>
          <p class="history-a"><strong>候选人回答：</strong>{{ turn.answer || '未回答' }}</p>
          <p v-if="turn.follow_up" class="history-follow">
            <strong>追问：</strong>{{ turn.follow_up }}<br />
            <strong>追问回答：</strong>{{ turn.follow_up_answer || '未回答' }}
          </p>
          <p v-if="turn.evaluated" class="history-eval">
            <strong>AI 评价（得分 {{ turn.score }}）：</strong>{{ turn.assessment }}
          </p>
          <p v-if="turn.evidence" class="history-ev">
            <strong>回答线索依据：</strong>{{ turn.evidence }}
          </p>
        </div>
      </details>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import * as App from '../../wailsjs/go/main/App'

const props = defineProps<{ modelValue: boolean; resumeId: string }>()
defineEmits<{ (e: 'update:modelValue', value: boolean): void }>()

const session = ref<any>(null)
const loading = ref(false)
const answer = ref('')
const hrNote = ref('')
const activeTurnIndex = ref(0)
const reviewedRules = ref<Record<string, string>>({})

const answeredCount = computed(() => {
  if (!session.value?.turns) return 0
  return session.value.turns.filter((t: any) => t.evaluated || t.answer).length
})

const currentTurn = computed(() => {
  if (!session.value?.turns || activeTurnIndex.value < 0 || activeTurnIndex.value >= session.value.turns.length) {
    return null
  }
  return session.value.turns[activeTurnIndex.value]
})

const isFollowUp = computed(() => !!currentTurn.value?.follow_up && !!currentTurn.value?.answer)

function getCategoryTagType(category: string): string {
  if (category === '工作意向') return 'info'
  if (category === '工作能力') return 'primary'
  if (category === '双向确认') return 'warning'
  return ''
}

function syncAnswerForActiveTurn() {
  const turn = currentTurn.value
  if (!turn) {
    answer.value = ''
    return
  }
  if (turn.follow_up && turn.answer) {
    answer.value = turn.follow_up_answer || ''
  } else {
    answer.value = turn.answer || ''
  }
}

watch(activeTurnIndex, () => {
  syncAnswerForActiveTurn()
})

watch(() => [props.modelValue, props.resumeId], async () => {
  if (!props.modelValue || !props.resumeId) return
  loading.value = true
  session.value = null
  reviewedRules.value = {}
  try {
    session.value = await App.StartInterview(props.resumeId)
    const checks = await App.GetResumeRedLineChecks(props.resumeId)
    reviewedRules.value = Object.fromEntries((checks || [])
      .filter((check: any) => !!check.reviewed_at)
      .map((check: any) => [check.rule_id, check.status]))
    hrNote.value = session.value?.hr_note || ''

    // 默认定位到首个未评估的题目
    const firstUnfinishedIdx = session.value?.turns?.findIndex((turn: any) => !turn.evaluated)
    if (firstUnfinishedIdx !== undefined && firstUnfinishedIdx >= 0) {
      activeTurnIndex.value = firstUnfinishedIdx
    } else {
      activeTurnIndex.value = 0
    }
    syncAnswerForActiveTurn()
  } catch (err: any) {
    ElMessage.error(`无法读取初面信息：${err.message || err}`)
  } finally {
    loading.value = false
  }
}, { immediate: true })

async function submitAnswer() {
  if (!currentTurn.value || !answer.value.trim()) return
  loading.value = true
  try {
    session.value = isFollowUp.value
      ? await App.SubmitInterviewFollowUp(props.resumeId, activeTurnIndex.value, answer.value.trim())
      : await App.SubmitInterviewAnswer(props.resumeId, activeTurnIndex.value, answer.value.trim())
    ElMessage.success('本题回答已保存')

    // 自动跳转至下一题
    if (activeTurnIndex.value < session.value.turns.length - 1) {
      activeTurnIndex.value++
    }
  } catch (err: any) {
    ElMessage.error(`回答已保留；AI 处理失败：${err.message || err}`)
  } finally {
    loading.value = false
  }
}

async function complete() {
  loading.value = true
  try {
    session.value = await App.CompleteInterview(props.resumeId)
    ElMessage.success('初试总结生成完成')
  } catch (err: any) {
    ElMessage.error(`生成总结失败：${err.message || err}`)
  } finally {
    loading.value = false
  }
}

async function review(decision: string) {
  loading.value = true
  try {
    session.value = await App.ReviewInterview(props.resumeId, decision, hrNote.value)
    ElMessage.success('HR 决定已保存')
  } catch (err: any) {
    ElMessage.error(`保存复核失败：${err.message || err}`)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 30px 0;
  color: #64748b;
}

.interview-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.hint {
  color: #64748b;
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
}

.steps-wrapper {
  overflow-x: auto;
  padding-bottom: 4px;
}

.interactive-steps {
  cursor: pointer;
}

.current-turn-card {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.turn-meta-bar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.turn-criterion {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.turn-progress-text {
  margin-left: auto;
  font-size: 12px;
  color: #64748b;
}

.question-text {
  font-size: 14.5px;
  line-height: 1.6;
  color: #1e293b;
  margin: 0;
}

.turn-bottom-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 4px;
}

.nav-btns {
  display: flex;
  gap: 8px;
}

.summary-action-box {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  padding: 12px 16px;
}

.summary-tip {
  font-size: 13px;
  color: #166534;
}

.completed-summary-box {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.score-summary-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13.5px;
  padding: 8px 12px;
  background: #f1f5f9;
  border-radius: 6px;
}

.score-num {
  font-size: 18px;
  font-weight: 700;
  color: #0284c7;
}

.summary-text-box {
  white-space: pre-wrap;
  font-size: 13.5px;
  line-height: 1.6;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 12px;
  color: #1e293b;
}

.hr-decision-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.hr-decision-section h4 {
  margin: 0;
  font-size: 13.5px;
  color: #334155;
}

.review-actions {
  display: flex;
  gap: 8px;
}

.hr-decision-result {
  font-size: 13px;
  color: #047857;
  margin: 4px 0 0 0;
}

.history-details {
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 8px 12px;
  background: #fafafa;
}

.history-details summary {
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: #475569;
}

.history-item {
  padding: 12px 0;
  border-bottom: 1px solid #f1f5f9;
}

.history-item:last-child {
  border-bottom: none;
}

.history-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.history-idx {
  font-weight: 700;
  font-size: 12.5px;
  color: #64748b;
}

.history-q {
  font-size: 13.5px;
  font-weight: 600;
  color: #1e293b;
}

.history-a, .history-follow, .history-eval, .history-ev {
  font-size: 13px;
  margin: 4px 0;
  line-height: 1.5;
}

.history-a { color: #334155; }
.history-follow { color: #475569; }
.history-eval { color: #0369a1; }
.history-ev { color: #64748b; }
</style>
