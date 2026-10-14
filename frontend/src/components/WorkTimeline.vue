<template>
  <div class="work-timeline-card">
    <div class="timeline-header">
      <div class="timeline-title-wrap">
        <el-icon class="timeline-icon"><Clock /></el-icon>
        <span class="timeline-title">职场履历时序与时间断层侦测</span>
        <span class="timeline-years-badge">{{ workYears || '多' }}年总工龄</span>
      </div>
      <div class="timeline-status-pill" :class="hasGapAlert ? 'alert' : 'safe'">
        <el-icon><Warning v-if="hasGapAlert" /><CircleCheck v-else /></el-icon>
        <span>{{ hasGapAlert ? '侦测到离职空窗或时间断层' : '履历时序连贯·无异常断层' }}</span>
      </div>
    </div>

    <!-- 可视化时间轴轨道 (Timeline Track) -->
    <div class="timeline-visual-track">
      <!-- 起点节点：职业生涯开启 -->
      <div class="track-node start">
        <div class="node-dot start-dot"></div>
        <div class="node-label">进入职场</div>
      </div>

      <!-- 中间主轴连线 -->
      <div class="track-line" :class="{ 'has-gaps': hasGapAlert }">
        <div class="line-progress"></div>
        <!-- 如果侦测到空窗断层，在线条中间插上断层预警旗帜 -->
        <div v-if="hasGapAlert" class="gap-marker" title="点击查看断层线索">
          <span class="marker-pulse"></span>
          <span class="marker-tag">⚠️ 空窗断层</span>
        </div>
      </div>

      <!-- 当前/最近任职节点 -->
      <div class="track-node current">
        <div class="node-dot current-dot"></div>
        <div class="node-label">{{ currentRole || '当前岗位' }}</div>
      </div>
    </div>

    <!-- 断层与连贯性详情线索列表 -->
    <div class="timeline-details-box">
      <div v-if="hasGapAlert" class="gap-alert-items">
        <div v-for="(gap, gIdx) in validGaps" :key="gIdx" class="gap-alert-item">
          <span class="alert-icon">⚠️</span>
          <div class="alert-content">
            <span class="alert-text">{{ gap }}</span>
            <span class="alert-sub-tip">建议初面核实：确认离职空窗期是否处于考证、进修或自由职业状态。</span>
          </div>
        </div>
      </div>
      <div v-else class="gap-safe-box">
        <div class="safe-tag">
          <el-icon><CircleCheck /></el-icon>
          <span>履历各段任职时间无缝衔接，未发现超过 3 个月的无记录离职空窗期。</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Clock, Warning, CircleCheck } from '@element-plus/icons-vue'

const props = withDefaults(
  defineProps<{
    gaps?: string[]
    workYears?: string
    currentRole?: string
  }>(),
  {
    gaps: () => [],
    workYears: '',
    currentRole: ''
  }
)

const validGaps = computed(() => {
  return (props.gaps || []).filter(
    (g) => g && !g.includes('连贯') && !g.includes('未侦测到') && !g.includes('无显著')
  )
})

const hasGapAlert = computed(() => {
  return validGaps.value.length > 0
})
</script>

<style scoped lang="scss">
.work-timeline-card {
  padding: 14px 16px;
  background: var(--bg-card, #ffffff);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03);

  .timeline-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 14px;

    .timeline-title-wrap {
      display: flex;
      align-items: center;
      gap: 7px;

      .timeline-icon {
        font-size: 15px;
        color: #2563eb;
      }

      .timeline-title {
        font-size: 13.5px;
        font-weight: 700;
        color: #1e293b;
      }

      .timeline-years-badge {
        font-size: 11px;
        font-weight: 600;
        padding: 1px 7px;
        border-radius: 6px;
        background: #eff6ff;
        color: #2563eb;
        border: 1px solid #dbeafe;
      }
    }

    .timeline-status-pill {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      font-size: 11.5px;
      font-weight: 600;
      padding: 3px 10px;
      border-radius: 9999px;

      &.safe {
        background: #ecfdf5;
        color: #047857;
        border: 1px solid #a7f3d0;
      }

      &.alert {
        background: #fffbeb;
        color: #b45309;
        border: 1px solid #fde68a;
      }
    }
  }

  // 时间轴轨道
  .timeline-visual-track {
    display: flex;
    align-items: center;
    padding: 12px 16px;
    background: #f8fafc;
    border-radius: 10px;
    border: 1px solid #f1f5f9;
    margin-bottom: 12px;

    .track-node {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 4px;
      flex-shrink: 0;

      .node-dot {
        width: 14px;
        height: 14px;
        border-radius: 50%;
        background: #2563eb;
        border: 3px solid #ffffff;
        box-shadow: 0 0 0 2px #93c5fd;

        &.start-dot {
          background: #64748b;
          box-shadow: 0 0 0 2px #cbd5e1;
        }
        &.current-dot {
          background: #10b981;
          box-shadow: 0 0 0 2px #86efac;
        }
      }

      .node-label {
        font-size: 11px;
        font-weight: 600;
        color: #475569;
        white-space: nowrap;
        max-width: 130px;
        overflow: hidden;
        text-overflow: ellipsis;
      }
    }

    .track-line {
      flex: 1;
      height: 4px;
      background: #cbd5e1;
      border-radius: 2px;
      margin: 0 12px;
      position: relative;
      display: flex;
      align-items: center;
      justify-content: center;

      .line-progress {
        position: absolute;
        left: 0;
        top: 0;
        bottom: 0;
        width: 100%;
        background: linear-gradient(90deg, #64748b 0%, #2563eb 50%, #10b981 100%);
        border-radius: 2px;
      }

      &.has-gaps {
        .line-progress {
          background: linear-gradient(90deg, #64748b 0%, #f59e0b 50%, #10b981 100%);
        }
      }

      .gap-marker {
        position: relative;
        z-index: 2;
        display: flex;
        align-items: center;
        gap: 4px;
        padding: 2px 8px;
        background: #fef2f2;
        border: 1px solid #f87171;
        border-radius: 10px;
        box-shadow: 0 2px 5px rgba(239, 68, 68, 0.2);

        .marker-tag {
          font-size: 10.5px;
          font-weight: 700;
          color: #dc2626;
        }

        .marker-pulse {
          width: 6px;
          height: 6px;
          border-radius: 50%;
          background: #ef4444;
          animation: pulse-dot 1.5s infinite;
        }
      }
    }
  }

  // 线索详情
  .timeline-details-box {
    .gap-alert-items {
      display: flex;
      flex-direction: column;
      gap: 6px;

      .gap-alert-item {
        display: flex;
        align-items: flex-start;
        gap: 8px;
        padding: 8px 12px;
        background: #fffbeb;
        border: 1px solid #fef3c7;
        border-radius: 8px;

        .alert-icon {
          font-size: 14px;
          flex-shrink: 0;
          margin-top: 1px;
        }

        .alert-content {
          display: flex;
          flex-direction: column;
          gap: 2px;

          .alert-text {
            font-size: 12px;
            font-weight: 600;
            color: #92400e;
          }

          .alert-sub-tip {
            font-size: 11px;
            color: #b45309;
            opacity: 0.9;
          }
        }
      }
    }

    .gap-safe-box {
      .safe-tag {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 12px;
        color: #047857;
        font-weight: 500;
        padding: 6px 10px;
        background: #f0fdf4;
        border-radius: 6px;

        .el-icon {
          font-size: 14px;
          color: #10b981;
        }
      }
    }
  }
}

@keyframes pulse-dot {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(239, 68, 68, 0.7); }
  70% { transform: scale(1.1); box-shadow: 0 0 0 5px rgba(239, 68, 68, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(239, 68, 68, 0); }
}
</style>
