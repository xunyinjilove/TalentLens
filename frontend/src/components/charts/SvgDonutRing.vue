<template>
  <div class="donut-ring-card">
    <div class="donut-svg-wrap" :style="{ width: size + 'px', height: size + 'px' }">
      <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`" class="donut-svg">
        <!-- 底色轨道圆环 -->
        <circle
          :cx="center"
          :cy="center"
          :r="radius"
          fill="none"
          :stroke="trackColor"
          :stroke-width="strokeWidth"
        />
        <!-- 动态进度圆环 -->
        <circle
          :cx="center"
          :cy="center"
          :r="radius"
          fill="none"
          :stroke="ringColor"
          :stroke-width="strokeWidth"
          stroke-linecap="round"
          :stroke-dasharray="circumference"
          :stroke-dashoffset="dashOffset"
          class="donut-progress-ring"
        />
      </svg>
      <!-- 圆环中心文字 -->
      <div class="donut-center-content">
        <span class="donut-val" :style="{ color: ringColor }">{{ Math.round(clampedPercentage) }}<small>%</small></span>
      </div>
    </div>
    <div class="donut-meta">
      <span class="donut-title">{{ title }}</span>
      <span v-if="subtitle" class="donut-subtitle">{{ subtitle }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    percentage: number
    size?: number
    strokeWidth?: number
    color?: string
    trackColor?: string
    title: string
    subtitle?: string
  }>(),
  {
    size: 78,
    strokeWidth: 7,
    color: '',
    trackColor: 'rgba(226, 232, 240, 0.8)',
    subtitle: ''
  }
)

const clampedPercentage = computed(() => {
  if (isNaN(props.percentage) || props.percentage < 0) return 0
  return Math.min(100, props.percentage)
})

const center = computed(() => props.size / 2)
const radius = computed(() => (props.size - props.strokeWidth) / 2)
const circumference = computed(() => 2 * Math.PI * radius.value)
const dashOffset = computed(() => {
  return circumference.value * (1 - clampedPercentage.value / 100)
})

const ringColor = computed(() => {
  if (props.color) return props.color
  const pct = clampedPercentage.value
  if (pct >= 80) return '#10b981' // 绿色
  if (pct >= 60) return '#2563eb' // 蓝色
  if (pct >= 40) return '#f59e0b' // 黄色
  return '#ef4444' // 红色
})
</script>

<style scoped lang="scss">
.donut-ring-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 10px 14px;
  background: var(--bg-card, #ffffff);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03);
  min-width: 105px;
  flex: 1;
  transition: transform 0.2s ease, box-shadow 0.2s ease;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 10px rgba(0, 0, 0, 0.06);
  }

  .donut-svg-wrap {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;

    .donut-svg {
      transform: rotate(-90deg);
      display: block;

      .donut-progress-ring {
        transition: stroke-dashoffset 0.8s cubic-bezier(0.4, 0, 0.2, 1);
      }
    }

    .donut-center-content {
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      pointer-events: none;

      .donut-val {
        font-size: 17px;
        font-weight: 800;
        font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', Roboto, sans-serif;
        line-height: 1;

        small {
          font-size: 10px;
          font-weight: 600;
          opacity: 0.85;
          margin-left: 1px;
        }
      }
    }
  }

  .donut-meta {
    margin-top: 8px;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 2px;

    .donut-title {
      font-size: 12px;
      font-weight: 600;
      color: #334155;
      white-space: nowrap;
    }

    .donut-subtitle {
      font-size: 10.5px;
      color: #94a3b8;
      white-space: nowrap;
    }
  }
}
</style>
