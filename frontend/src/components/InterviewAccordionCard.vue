<template>
  <div class="interview-accordion-container">
    <div
      v-for="(item, idx) in questions"
      :key="item.question_id || idx"
      class="qa-accordion-item"
      :class="{ 'is-open': openMap[idx] }"
    >
      <!-- 考题头部 (点击展开/折叠) -->
      <div class="qa-item-head" @click="toggleItem(idx)">
        <div class="head-left">
          <span class="qa-order-pill">Q{{ idx + 1 }}</span>
          <span class="qa-category-tag" :class="getCategoryClass(item.category)">
            {{ item.category_label || item.category || '初试提问' }}
          </span>
          <span v-if="item.topic" class="qa-topic-pill">{{ item.topic }}</span>
          <span class="qa-question-title">{{ item.question }}</span>
        </div>
        <div class="head-right" @click.stop>
          <!-- 快捷评价状态 -->
          <div class="eval-status-group">
            <button
              class="eval-btn pass"
              :class="{ active: item.hr_verdict === 'pass' }"
              @click="setVerdict(item, 'pass')"
              title="基本一致 / 有明确事例"
            >
              ✓
            </button>
            <button
              class="eval-btn pending"
              :class="{ active: item.hr_verdict === 'pending' }"
              @click="setVerdict(item, 'pending')"
              title="待确认 / 需进一步了解"
            >
              ?
            </button>
            <button
              class="eval-btn gap"
              :class="{ active: item.hr_verdict === 'gap' }"
              @click="setVerdict(item, 'gap')"
              title="存在差异 / 欠缺事例"
            >
              ✕
            </button>
          </div>

          <!-- 一键复制单题与考察要点 -->
          <button class="action-icon-btn" @click="handleCopySingleQA(item, idx)" title="复制该道题目与 HR 观察要点">
            <el-icon><DocumentCopy /></el-icon>
          </button>

          <!-- 折叠展开箭头 -->
          <span class="expand-arrow" :class="{ 'is-expanded': openMap[idx] }" @click="toggleItem(idx)">
            <el-icon><ArrowDown /></el-icon>
          </span>
        </div>
      </div>

      <!-- 展开的考题详情内容 (手风琴主体) -->
      <Transition name="accordion-slide">
        <div v-if="openMap[idx]" class="qa-item-body">
          <!-- 1. 为什么问 & 简历线索 -->
          <div class="info-meta-row">
            <div class="meta-item">
              <span class="meta-label">🎯 为什么问：</span>
              <span class="meta-value">{{ item.reason_to_ask || '核实实际工作经历与期望' }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-label">📄 简历线索：</span>
              <span class="meta-value clue">{{ item.source_refs || '本次沟通现场确认' }}</span>
            </div>
          </div>

          <!-- 2. 条件追问 -->
          <div v-if="item.follow_up_hints && item.follow_up_hints.length > 0" class="hints-box">
            <div class="box-title">
              <el-icon><ChatLineSquare /></el-icon>
              <span>💬 条件追问（视回答情况选用 1~2 条）：</span>
            </div>
            <ul class="box-list">
              <li v-for="(hint, hIdx) in item.follow_up_hints" :key="hIdx">{{ hint }}</li>
            </ul>
          </div>

          <!-- 3. 重点听什么（HR 观察点） -->
          <div v-if="item.hr_observe_points && item.hr_observe_points.length > 0" class="observe-box">
            <div class="box-title">
              <el-icon><Aim /></el-icon>
              <span>👂 重点听什么（HR 观察点与评判建议）：</span>
            </div>
            <ul class="box-list">
              <li v-for="(pt, pIdx) in item.hr_observe_points" :key="pIdx">{{ pt }}</li>
            </ul>
          </div>

          <!-- 4. 关联岗位重点关注项 -->
          <div v-if="item.related_checks && item.related_checks.length > 0" class="related-checks-box">
            <div class="box-title">
              <el-icon><Briefcase /></el-icon>
              <span>📌 关联岗位重点关注项（建议结合本题案例核验，专业深度交接下一轮）：</span>
            </div>
            <div class="tag-group">
              <span v-for="(check, cIdx) in item.related_checks" :key="cIdx" class="check-pill">
                {{ check }}
              </span>
            </div>
          </div>

          <!-- 5. 初试现场记录与候选人原话摘要 -->
          <div class="record-section">
            <div class="record-head">
              <span class="record-title">📝 初试现场记录与候选人原话摘要</span>
              <span v-if="item.saved_at" class="saved-time-badge">已保存于 {{ item.saved_at }}</span>
            </div>
            <el-input
              v-model="item.hr_notes"
              type="textarea"
              :rows="3"
              maxlength="3000"
              show-word-limit
              placeholder="请客观记录候选人作答要点或原话；不确定的内容可以明确标记“待确认”。"
              class="notes-textarea"
            />
            <div class="record-actions">
              <div class="verdict-radios">
                <span class="radios-label">本次判断：</span>
                <el-radio-group v-model="item.hr_verdict" size="small">
                  <el-radio-button value="pass">基本一致 / 有事例</el-radio-button>
                  <el-radio-button value="pending">待确认 / 需了解</el-radio-button>
                  <el-radio-button value="gap">存在差异 / 欠缺</el-radio-button>
                  <el-radio-button value="unanswered">未涉及</el-radio-button>
                </el-radio-group>
              </div>
              <el-button
                type="primary"
                size="small"
                :loading="savingMap[item.question_id || idx]"
                @click="saveQuestion(item, idx)"
              >
                保存本题记录
              </el-button>
            </div>
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive } from 'vue'
import { DocumentCopy, ArrowDown, Aim, ChatLineSquare, Briefcase } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

export interface HROutlineQuestion {
  question_id: string
  category: string
  category_label: string
  topic: string
  question: string
  reason_to_ask: string
  source_refs?: string
  follow_up_hints?: string[]
  hr_observe_points?: string[]
  record_fields?: string
  related_checks?: string[]
  hr_verdict?: string
  hr_notes?: string
  saved_at?: string
}

export interface InterviewOutline {
  resume_id: string
  project_id: string
  candidate_name: string
  job_title: string
  priority_focus: string[]
  questions: HROutlineQuestion[]
  version: number
}

const props = defineProps<{
  outline?: InterviewOutline | null
  qaList?: any[]
  resumeId?: string
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
}>()

// 展开状态映射（默认展开第 1 题）
const openMap = reactive<Record<number, boolean>>({
  0: true
})

// 单题保存 loading 状态
const savingMap = reactive<Record<string | number, boolean>>({})

// 计算问题列表：优先使用 outline.questions，若无则平滑转换 legacy qaList
const questions = computed<HROutlineQuestion[]>(() => {
  if (props.outline?.questions && props.outline.questions.length > 0) {
    return props.outline.questions
  }
  if (props.qaList && props.qaList.length > 0) {
    return props.qaList.map((qa, i) => ({
      question_id: `legacy_${i}`,
      category: 'competence',
      category_label: qa.category || '能力实战',
      topic: qa.category || '核心经历',
      question: qa.question,
      reason_to_ask: '基于简历经历核查实战真实性与独立贡献',
      source_refs: qa.reference_answer ? `参考线索：${qa.reference_answer}` : '本次沟通现场确认',
      follow_up_hints: ['请举出具体工作事例说明本人承担的实际职责'],
      hr_observe_points: ['重点关注职责范围、交付成果与复盘改进'],
      hr_verdict: 'unanswered',
      hr_notes: ''
    }))
  }
  return []
})

function toggleItem(idx: number) {
  openMap[idx] = !openMap[idx]
}

function getCategoryClass(cat?: string): string {
  if (!cat) return 'cat-competence'
  if (cat === 'intention' || cat.includes('意向')) return 'cat-intention'
  if (cat === 'confirmation' || cat.includes('确认')) return 'cat-confirmation'
  return 'cat-competence'
}

async function setVerdict(item: HROutlineQuestion, verdict: string) {
  if (item.hr_verdict === verdict) {
    item.hr_verdict = 'unanswered'
  } else {
    item.hr_verdict = verdict
  }
  if (props.resumeId && item.question_id) {
    try {
      const api: any = await import('../../wailsjs/go/main/App')
      await api.SaveHROutlineQuestion(props.resumeId, item.question_id, item.hr_verdict, item.hr_notes || '')
    } catch {}
  }
}

async function saveQuestion(item: HROutlineQuestion, idx: number) {
  if (!props.resumeId) {
    ElMessage.warning('缺少简历 ID，无法保存')
    return
  }
  const key = item.question_id || idx
  savingMap[key] = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const updated = await api.SaveHROutlineQuestion(
      props.resumeId,
      item.question_id,
      item.hr_verdict || 'unanswered',
      item.hr_notes || ''
    )
    if (updated) {
      item.saved_at = new Date().toLocaleTimeString()
      ElMessage.success(`第 ${idx + 1} 题记录已成功保存！`)
      emit('refresh')
    }
  } catch (err: any) {
    ElMessage.error(`保存失败：${err.message || err}`)
  } finally {
    savingMap[key] = false
  }
}

function handleCopySingleQA(item: HROutlineQuestion, idx: number) {
  const followUps = (item.follow_up_hints || []).map(h => `  - ${h}`).join('\n')
  const observes = (item.hr_observe_points || []).map(o => `  - ${o}`).join('\n')
  const text = [
    `【HR 初试题目 Q${idx + 1} · ${item.category_label || '初试提问'}】`,
    `主题：${item.topic || '综合考察'}`,
    `为什么问：${item.reason_to_ask || '了解实际经历与意向'}`,
    `主问题：${item.question}`,
    followUps ? `条件追问：\n${followUps}` : '',
    observes ? `重点听什么（HR 观察点）：\n${observes}` : ''
  ].filter(Boolean).join('\n')

  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success(`第 ${idx + 1} 题与观察要点已复制到剪贴板`)
  }).catch(() => {
    ElMessage.error('复制失败，请手动选取')
  })
}
</script>

<style scoped lang="scss">
.interview-accordion-container {
  display: flex;
  flex-direction: column;
  gap: 12px;

  .qa-accordion-item {
    background: var(--bg-card, #ffffff);
    border: 1px solid var(--border-color, #e2e8f0);
    border-radius: 12px;
    overflow: hidden;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);

    &:hover {
      border-color: #cbd5e1;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
    }

    &.is-open {
      border-color: rgba(99, 102, 241, 0.4);
      box-shadow: 0 4px 14px rgba(99, 102, 241, 0.06);

      .qa-item-head {
        background: rgba(248, 250, 252, 0.95);
      }
    }

    .qa-item-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 13px 16px;
      cursor: pointer;
      user-select: none;
      transition: background 0.15s ease;

      .head-left {
        display: flex;
        align-items: center;
        gap: 8px;
        min-width: 0;
        flex: 1;
        flex-wrap: wrap;

        .qa-order-pill {
          font-size: 11px;
          font-weight: 700;
          color: #4f46e5;
          background: #eef2ff;
          border: 1px solid #c7d2fe;
          padding: 2px 7px;
          border-radius: 6px;
          flex-shrink: 0;
        }

        .qa-category-tag {
          font-size: 11px;
          font-weight: 600;
          padding: 2px 8px;
          border-radius: 6px;
          white-space: nowrap;
          flex-shrink: 0;

          &.cat-intention {
            background: #eff6ff;
            color: #1d4ed8;
            border: 1px solid #bfdbfe;
          }
          &.cat-competence {
            background: #f5f3ff;
            color: #6d28d9;
            border: 1px solid #ddd6fe;
          }
          &.cat-confirmation {
            background: #ecfdf5;
            color: #047857;
            border: 1px solid #a7f3d0;
          }
        }

        .qa-topic-pill {
          font-size: 11px;
          font-weight: 600;
          color: #475569;
          background: #f1f5f9;
          border: 1px solid #e2e8f0;
          padding: 2px 8px;
          border-radius: 6px;
          white-space: nowrap;
          flex-shrink: 0;
        }

        .qa-question-title {
          font-size: 13.5px;
          font-weight: 600;
          color: #1e293b;
          line-height: 1.5;
          word-break: break-word;
          overflow-wrap: anywhere;
        }
      }

      .head-right {
        display: flex;
        align-items: center;
        gap: 10px;
        flex-shrink: 0;

        .eval-status-group {
          display: flex;
          align-items: center;
          gap: 4px;
          background: #f1f5f9;
          padding: 3px 5px;
          border-radius: 7px;

          .eval-btn {
            width: 22px;
            height: 22px;
            border-radius: 5px;
            border: none;
            background: transparent;
            font-size: 11.5px;
            font-weight: 700;
            color: #64748b;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.15s ease;

            &:hover { background: #e2e8f0; }

            &.pass.active { background: #10b981; color: #ffffff; }
            &.pending.active { background: #f59e0b; color: #ffffff; }
            &.gap.active { background: #ef4444; color: #ffffff; }
          }
        }

        .action-icon-btn {
          display: flex;
          align-items: center;
          justify-content: center;
          width: 28px;
          height: 28px;
          border-radius: 6px;
          border: 1px solid #e2e8f0;
          background: #ffffff;
          color: #64748b;
          cursor: pointer;
          transition: all 0.15s ease;

          &:hover {
            color: #4f46e5;
            border-color: #c7d2fe;
            background: #eef2ff;
          }
        }

        .expand-arrow {
          display: flex;
          align-items: center;
          color: #94a3b8;
          font-size: 12px;
          transition: transform 0.25s ease;

          &.is-expanded {
            transform: rotate(180deg);
            color: #4f46e5;
          }
        }
      }
    }

    .qa-item-body {
      padding: 14px 18px 16px;
      border-top: 1px solid #f1f5f9;
      background: #ffffff;
      display: flex;
      flex-direction: column;
      gap: 12px;

      .info-meta-row {
        display: flex;
        flex-wrap: wrap;
        gap: 16px;
        background: #f8fafc;
        border-radius: 8px;
        padding: 8px 12px;
        font-size: 12.5px;

        .meta-item {
          display: flex;
          align-items: center;
          gap: 6px;

          .meta-label {
            font-weight: 600;
            color: #475569;
          }
          .meta-value {
            color: #334155;

            &.clue {
              color: #2563eb;
              font-weight: 500;
            }
          }
        }
      }

      .hints-box, .observe-box, .related-checks-box {
        border-radius: 8px;
        padding: 10px 14px;
        font-size: 12.5px;
        line-height: 1.6;

        .box-title {
          display: flex;
          align-items: center;
          gap: 6px;
          font-weight: 600;
          margin-bottom: 6px;
        }

        .box-list {
          margin: 0;
          padding-left: 18px;
          color: #334155;
          li {
            margin-bottom: 3px;
            &:last-child { margin-bottom: 0; }
          }
        }
      }

      .hints-box {
        background: #f0fdf4;
        border: 1px solid #dcfce7;
        .box-title { color: #166534; }
      }

      .observe-box {
        background: #eff6ff;
        border: 1px solid #dbeafe;
        .box-title { color: #1e40af; }
      }

      .related-checks-box {
        background: #fffbeb;
        border: 1px solid #fef3c7;
        .box-title { color: #92400e; }

        .tag-group {
          display: flex;
          flex-wrap: wrap;
          gap: 6px;
          margin-top: 6px;

          .check-pill {
            background: #ffffff;
            border: 1px solid #fde68a;
            color: #b45309;
            padding: 3px 8px;
            border-radius: 6px;
            font-size: 12px;
            font-weight: 500;
          }
        }
      }

      .record-section {
        background: #f8fafc;
        border: 1px solid #e2e8f0;
        border-radius: 8px;
        padding: 12px;
        margin-top: 4px;

        .record-head {
          display: flex;
          justify-content: space-between;
          align-items: center;
          margin-bottom: 8px;

          .record-title {
            font-size: 12.5px;
            font-weight: 600;
            color: #1e293b;
          }

          .saved-time-badge {
            font-size: 11px;
            color: #10b981;
            background: #ecfdf5;
            padding: 2px 6px;
            border-radius: 4px;
            border: 1px solid #a7f3d0;
          }
        }

        .notes-textarea {
          margin-bottom: 10px;
        }

        .record-actions {
          display: flex;
          justify-content: space-between;
          align-items: center;
          flex-wrap: wrap;
          gap: 8px;

          .verdict-radios {
            display: flex;
            align-items: center;
            gap: 6px;

            .radios-label {
              font-size: 12px;
              color: #64748b;
            }
          }
        }
      }
    }
  }
}

.accordion-slide-enter-active, .accordion-slide-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.accordion-slide-enter-from, .accordion-slide-leave-to {
  opacity: 0;
  max-height: 0;
  padding-top: 0;
  padding-bottom: 0;
  overflow: hidden;
}
</style>
