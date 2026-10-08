<template>
  <el-dialog :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" title="AI 文字初面" width="700px" destroy-on-close>
    <div v-if="loading && !session">正在读取初面会话...</div>
    <div v-else-if="session" class="interview-content">
      <p class="hint">{{ session.candidate_name }} · 回答会逐题保存。AI 的追问和结论供 HR 复核。</p>
      <el-steps :active="currentIndex" finish-status="success" simple>
        <el-step v-for="(_, i) in session.turns" :key="i" :title="`第 ${i + 1} 题`" />
      </el-steps>
      <template v-if="currentTurn">
        <p class="question"><strong>{{ currentTurn.category }}：</strong>{{ isFollowUp ? currentTurn.follow_up : currentTurn.question }}</p>
        <el-input v-model="answer" type="textarea" :rows="6" maxlength="4000" show-word-limit placeholder="请记录候选人的原话；不确定的内容可以明确写不清楚" />
        <el-button type="primary" :loading="loading" :disabled="!answer.trim()" @click="submitAnswer">提交回答{{ isFollowUp ? '并完成本题' : '' }}</el-button>
      </template>
      <template v-else-if="session.status === 'in_progress'">
        <p>全部问题已回答，可以生成初面总结。</p>
        <el-button type="primary" :loading="loading" @click="complete">生成初面总结</el-button>
      </template>
      <template v-else>
        <el-alert title="AI 初面已完成，最终决定由 HR 做出" type="info" :closable="false" />
        <p><strong>AI 分数：</strong>{{ session.overall_score }}　<strong>建议：</strong>{{ session.ai_recommendation }}</p>
        <p>{{ session.ai_summary }}</p>
        <div v-if="redLineTurns.length" class="redline-review">
          <strong>逐条核实岗位红线</strong>
          <p class="hint">AI 只提供问答线索。请依据候选人原话确认；未核实前不能选择“进入下一轮”。</p>
          <div v-for="turn in redLineTurns" :key="turn.rule_id" class="redline-item">
            <strong>{{ turn.criterion }}</strong>
            <p>回答：{{ turn.answer }}{{ turn.follow_up_answer ? `；追问：${turn.follow_up_answer}` : '' }}</p>
            <p v-if="reviewedRules[turn.rule_id]">HR 已记录：{{ reviewedRules[turn.rule_id] === 'met' ? '满足' : reviewedRules[turn.rule_id] === 'violated' ? '不满足' : '待核实' }}</p>
            <el-button size="small" type="success" plain :loading="loading" @click="reviewRedLine(turn, 'met')">确认满足</el-button>
            <el-button size="small" type="danger" plain :loading="loading" @click="reviewRedLine(turn, 'violated')">确认不满足</el-button>
            <el-button size="small" :loading="loading" @click="reviewRedLine(turn, 'unknown')">仍待核实</el-button>
          </div>
        </div>
        <el-input v-model="hrNote" type="textarea" :rows="2" placeholder="HR 复核备注" />
        <div class="review-actions">
          <el-button :loading="loading" @click="review('advance')">进入下一轮</el-button>
          <el-button :loading="loading" @click="review('hold')">待核实</el-button>
          <el-button :loading="loading" @click="review('decline')">不推进</el-button>
        </div>
        <p v-if="session.hr_decision">HR 决定：{{ session.hr_decision }}{{ session.hr_note ? ` · ${session.hr_note}` : '' }}</p>
      </template>
      <el-divider />
      <details>
        <summary>查看已提交的问答与单题评价</summary>
        <div v-for="(turn, i) in session.turns" :key="i" class="history">
          <strong>{{ i + 1 }}. {{ turn.question }}</strong>
          <p>候选人：{{ turn.answer || '未回答' }}</p>
          <p v-if="turn.follow_up">追问：{{ turn.follow_up }}<br />候选人：{{ turn.follow_up_answer || '未回答' }}</p>
          <p v-if="turn.evaluated">AI 评价（{{ turn.score }}）：{{ turn.assessment }}</p>
        </div>
      </details>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as App from '../../wailsjs/go/main/App'

const props = defineProps<{ modelValue: boolean; resumeId: string }>()
defineEmits<{ (e: 'update:modelValue', value: boolean): void }>()
const session = ref<any>(null)
const loading = ref(false)
const answer = ref('')
const hrNote = ref('')
const currentIndex = computed(() => session.value?.turns?.findIndex((turn: any) => !turn.evaluated) ?? -1)
const currentTurn = computed(() => currentIndex.value >= 0 ? session.value?.turns[currentIndex.value] : null)
const isFollowUp = computed(() => !!currentTurn.value?.follow_up && !!currentTurn.value?.answer)
const redLineTurns = computed(() => (session.value?.turns || []).filter((turn: any) => !!turn.rule_id))
const reviewedRules = ref<Record<string, string>>({})

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
    const unfinished = session.value?.turns?.find((turn: any) => !turn.evaluated)
    answer.value = unfinished?.follow_up ? unfinished.follow_up_answer || '' : unfinished?.answer || ''
  } catch (err: any) { ElMessage.error(`无法开始初面：${err.message || err}`) }
  finally { loading.value = false }
}, { immediate: true })

async function submitAnswer() {
  if (!currentTurn.value || !answer.value.trim()) return
  loading.value = true
  try {
    session.value = isFollowUp.value
      ? await App.SubmitInterviewFollowUp(props.resumeId, currentIndex.value, answer.value.trim())
      : await App.SubmitInterviewAnswer(props.resumeId, currentIndex.value, answer.value.trim())
    answer.value = ''
  } catch (err: any) { ElMessage.error(`回答已保留；AI 处理失败：${err.message || err}`) }
  finally { loading.value = false }
}

async function complete() {
  loading.value = true
  try { session.value = await App.CompleteInterview(props.resumeId) }
  catch (err: any) { ElMessage.error(`生成总结失败：${err.message || err}`) }
  finally { loading.value = false }
}

async function review(decision: string) {
  loading.value = true
  try { session.value = await App.ReviewInterview(props.resumeId, decision, hrNote.value) }
  catch (err: any) { ElMessage.error(`保存复核失败：${err.message || err}`) }
  finally { loading.value = false }
}

async function reviewRedLine(turn: any, status: 'met' | 'violated' | 'unknown') {
  let evidence = ''
  if (status !== 'unknown') {
    try {
      const result = await ElMessageBox.prompt(`请从「${turn.criterion}」的候选人回答中选取能支持结论的原话`, 'HR 红线核实', {
        inputValue: (turn.follow_up_answer || turn.answer || '').slice(0, 400),
        inputValidator: value => !!value?.trim() &&
          ((turn.answer || '').includes(value.trim()) || (turn.follow_up_answer || '').includes(value.trim()))
          ? true : '依据必须是该题候选人的原话'
      })
      evidence = result.value.trim()
    } catch { return }
  }
  loading.value = true
  try {
    const resume = await App.ReviewInterviewRedLine(props.resumeId, turn.rule_id, status, evidence)
    const check = resume.analysis?.red_line_checks?.find((item: any) => item.rule_id === turn.rule_id)
    reviewedRules.value[turn.rule_id] = check?.status || status
    ElMessage.success('红线核实已保存，候选人详情中的结论同步更新')
  } catch (err: any) { ElMessage.error(`红线核实失败：${err.message || err}`) }
  finally { loading.value = false }
}
</script>

<style scoped>
.interview-content { display: flex; flex-direction: column; gap: 14px; }
.hint { color: #64748b; margin: 0; }
.question { font-size: 16px; line-height: 1.6; }
.review-actions { display: flex; gap: 8px; }
.history { padding: 10px 0; border-bottom: 1px solid #eee; }
.history p { margin: 5px 0; white-space: pre-wrap; }
.redline-review { padding: 12px; border: 1px solid #f5c2c2; border-radius: 8px; }
.redline-item { padding: 10px 0; border-top: 1px solid #eee; }
.redline-item p { white-space: pre-wrap; }
</style>
