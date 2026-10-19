<template>
  <div class="interview-accordion-container">
    <div v-for="(item, idx) in qaList" :key="idx" class="qa-accordion-item" :class="{ 'is-open': openMap[idx] }">
      <!-- 考题头部 (点击展开/折叠) -->
      <div class="qa-item-head" @click="toggleItem(idx)">
        <div class="head-left">
          <span class="qa-order-pill">Q{{ idx + 1 }}</span>
          <span class="qa-category-tag" :class="getCategoryClass(item.category)">
            {{ item.category || '技术提问' }}
          </span>
          <span class="qa-question-title">{{ item.question }}</span>
        </div>
        <div class="head-right" @click.stop>
          <!-- 快捷评价状态 -->
          <div class="eval-status-group">
            <button
              class="eval-btn pass"
              :class="{ active: evalMap[idx] === 'pass' }"
              @click="setEval(idx, 'pass')"
              title="标记为合格"
            >
              ✓
            </button>
            <button
              class="eval-btn pending"
              :class="{ active: evalMap[idx] === 'pending' }"
              @click="setEval(idx, 'pending')"
              title="标记为待定"
            >
              ?
            </button>
            <button
              class="eval-btn fail"
              :class="{ active: evalMap[idx] === 'fail' }"
              @click="setEval(idx, 'fail')"
              title="标记为不达标"
            >
              ✕
            </button>
          </div>

          <!-- 一键复制单题 -->
          <button class="action-icon-btn" @click="handleCopySingleQA(item, idx)" title="复制该道考题与参考回答">
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
          <div v-if="item.reference_answer" class="reference-answer-block">
            <div class="block-title">
              <el-icon><InfoFilled /></el-icon>
              <span>基于候选人履历事实的参考回答要点：</span>
            </div>
            <div class="block-text">{{ item.reference_answer }}</div>
          </div>

          <div class="evaluation-guide-row">
            <span class="guide-tag">🎯 面试评判建议：</span>
            <span class="guide-desc">重点核查项目中是否独立负责关键设计，反问候选人在该方案下的瓶颈取舍。</span>
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { DocumentCopy, InfoFilled, ArrowDown } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

interface QAItem {
  question: string
  reference_answer?: string
  category?: string
}

const props = defineProps<{
  qaList: QAItem[]
}>()

// 默认第一题展开，其余题折叠
const openMap = reactive<Record<number, boolean>>({
  0: true
})

// 考官实时评价状态 ('pass' | 'pending' | 'fail')
const evalMap = reactive<Record<number, 'pass' | 'pending' | 'fail'>>({})

function toggleItem(idx: number) {
  openMap[idx] = !openMap[idx]
}

function setEval(idx: number, status: 'pass' | 'pending' | 'fail') {
  if (evalMap[idx] === status) {
    delete evalMap[idx]
  } else {
    evalMap[idx] = status
  }
}

function getCategoryClass(category?: string): string {
  if (!category) return 'cat-tech'
  if (category.includes('能力') || category.includes('综合') || category.includes('沟通') || category.includes('协作') || category.includes('自驱') || category.includes('动机') || category.includes('抗压') || category.includes('规划')) {
    return 'cat-ability'
  }
  if (category.includes('技术') || category.includes('专业')) return 'cat-tech'
  if (category.includes('项目') || category.includes('业务')) return 'cat-project'
  if (category.includes('真实') || category.includes('防伪') || category.includes('疑点') || category.includes('短板')) return 'cat-gap'
  return 'cat-tech'
}

function handleCopySingleQA(item: QAItem, idx: number) {
  const text = `【面试题目 Q${idx + 1} (${item.category || '技术'})】\n${item.question}\n\n💡 参考回答要点：\n${item.reference_answer || '围绕核心实战经验展开'}`
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success(`第 ${idx + 1} 题已复制到剪贴板`)
  }).catch(() => {
    ElMessage.error('复制失败，请手动选取')
  })
}
</script>

<style scoped lang="scss">
.interview-accordion-container {
  display: flex;
  flex-direction: column;
  gap: 10px;

  .qa-accordion-item {
    background: var(--bg-card, #ffffff);
    border: 1px solid var(--border-color, #e2e8f0);
    border-radius: 10px;
    overflow: hidden;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);

    &:hover {
      border-color: #cbd5e1;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
    }

    &.is-open {
      border-color: rgba(37, 99, 235, 0.3);
      box-shadow: 0 3px 10px rgba(37, 99, 235, 0.05);

      .qa-item-head {
        background: rgba(248, 250, 252, 0.85);
      }
    }

    .qa-item-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 12px 14px;
      cursor: pointer;
      user-select: none;
      transition: background 0.15s ease;

      .head-left {
        display: flex;
        align-items: center;
        gap: 8px;
        min-width: 0;
        flex: 1;

        .qa-order-pill {
          font-size: 11px;
          font-weight: 700;
          color: #2563eb;
          background: #eff6ff;
          border: 1px solid #dbeafe;
          padding: 1px 6px;
          border-radius: 6px;
          flex-shrink: 0;
        }

        .qa-category-tag {
          font-size: 11px;
          font-weight: 600;
          padding: 2px 7px;
          border-radius: 6px;
          white-space: nowrap;
          flex-shrink: 0;

          &.cat-ability { background: #fdf2f8; color: #be185d; border: 1px solid #fbcfe8; }
          &.cat-tech { background: #eff6ff; color: #1d4ed8; border: 1px solid #dbeafe; }
          &.cat-project { background: #f0fdf4; color: #15803d; border: 1px solid #bbf7d0; }
          &.cat-gap { background: #fef2f2; color: #b91c1c; border: 1px solid #fecaca; }
        }

        .qa-question-title {
          font-size: 13.5px;
          font-weight: 600;
          color: #1e293b;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
      }

      .head-right {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-shrink: 0;

        .eval-status-group {
          display: flex;
          align-items: center;
          gap: 3px;
          background: #f1f5f9;
          padding: 2px;
          border-radius: 6px;

          .eval-btn {
            width: 20px;
            height: 20px;
            border-radius: 4px;
            border: none;
            background: transparent;
            font-size: 11px;
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
            &.fail.active { background: #ef4444; color: #ffffff; }
          }
        }

        .action-icon-btn {
          display: flex;
          align-items: center;
          justify-content: center;
          width: 26px;
          height: 26px;
          border-radius: 6px;
          border: 1px solid #e2e8f0;
          background: #ffffff;
          color: #64748b;
          cursor: pointer;
          transition: all 0.15s ease;

          &:hover {
            color: #2563eb;
            border-color: #2563eb;
            background: #eff6ff;
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
            color: #2563eb;
          }
        }
      }
    }

    .qa-item-body {
      padding: 12px 16px 14px;
      border-top: 1px solid #f1f5f9;
      background: #ffffff;

      .reference-answer-block {
        background: #f8fafc;
        border: 1px solid #e2e8f0;
        border-radius: 8px;
        padding: 10px 14px;
        margin-bottom: 8px;

        .block-title {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 12px;
          font-weight: 600;
          color: #2563eb;
          margin-bottom: 6px;
        }

        .block-text {
          font-size: 13px;
          line-height: 1.6;
          color: #334155;
          white-space: pre-wrap;
        }
      }

      .evaluation-guide-row {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 11.5px;
        color: #64748b;

        .guide-tag {
          font-weight: 600;
          color: #475569;
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
