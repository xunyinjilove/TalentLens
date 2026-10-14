<template>
  <div class="radar-chart-container" :style="{ width: size + 'px' }">
    <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`" class="radar-svg">
      <!-- 1. 背景同心多边形网格 (4层：25%, 50%, 75%, 100%) -->
      <g class="radar-grid-rings">
        <polygon
          v-for="(ring, rIdx) in gridRings"
          :key="'ring-' + rIdx"
          :points="ring.points"
          fill="none"
          :stroke="rIdx === gridRings.length - 1 ? 'rgba(203, 213, 225, 0.8)' : 'rgba(226, 232, 240, 0.6)'"
          :stroke-width="rIdx === gridRings.length - 1 ? 1.5 : 1"
          :stroke-dasharray="rIdx === 2 ? '3,3' : 'none'"
        />
      </g>

      <!-- 2. 辐射轴线 (Spokes) -->
      <g class="radar-spokes">
        <line
          v-for="(spoke, sIdx) in outerVertices"
          :key="'spoke-' + sIdx"
          :x1="center"
          :y1="center"
          :x2="spoke.x"
          :y2="spoke.y"
          stroke="rgba(226, 232, 240, 0.9)"
          stroke-width="1"
        />
      </g>

      <!-- 3. 岗位期望基准层 (Benchmark Layer, 虚线浅色) -->
      <g v-if="benchmarkPoints" class="radar-benchmark-layer">
        <polygon
          :points="benchmarkPoints"
          fill="rgba(148, 163, 184, 0.12)"
          stroke="#94a3b8"
          stroke-width="1.5"
          stroke-dasharray="4,4"
        />
      </g>

      <!-- 4. 候选人实际匹配层 (Candidate Layer, 主题蓝渐变/半透实色) -->
      <g class="radar-candidate-layer">
        <polygon
          :points="candidatePoints"
          fill="rgba(37, 99, 235, 0.22)"
          stroke="#2563eb"
          stroke-width="2.5"
          class="candidate-polygon"
        />
        <!-- 顶点圆点 (带呼吸动效与点击热区) -->
        <circle
          v-for="(pt, pIdx) in candidateVertices"
          :key="'pt-' + pIdx"
          :cx="pt.x"
          :cy="pt.y"
          :r="selectedIndex === pIdx ? 6 : 4"
          :fill="selectedIndex === pIdx ? '#1d4ed8' : '#2563eb'"
          stroke="#ffffff"
          :stroke-width="selectedIndex === pIdx ? 2.5 : 2"
          class="vertex-circle"
          @click="handleSelect(pIdx)"
        />
      </g>

      <!-- 5. 轴指示标签 (Indicator Labels) -->
      <g class="radar-labels">
        <g
          v-for="(lbl, lIdx) in labelPositions"
          :key="'lbl-' + lIdx"
          class="label-group"
          :class="{ active: selectedIndex === lIdx }"
          @click="handleSelect(lIdx)"
        >
          <text
            :x="lbl.x"
            :y="lbl.y"
            :text-anchor="lbl.anchor"
            :dominant-baseline="lbl.baseline"
            class="radar-label-text"
          >
            {{ lbl.name }}
          </text>
          <text
            :x="lbl.x"
            :y="lbl.y + 13"
            :text-anchor="lbl.anchor"
            class="radar-score-text"
          >
            {{ candidateScores[lIdx] }}/4
          </text>
        </g>
      </g>
    </svg>

    <!-- 图例 (Legend) -->
    <div class="radar-legend">
      <div class="legend-item candidate">
        <span class="legend-line candidate-line"></span>
        <span>候选人实际</span>
      </div>
      <div class="legend-item benchmark">
        <span class="legend-line benchmark-line"></span>
        <span>岗位期望 (3级)</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Indicator {
  name: string
  max?: number
}

const props = withDefaults(
  defineProps<{
    indicators: Indicator[]
    candidateValues: number[]
    benchmarkValues?: number[]
    selectedIndex?: number | null
    size?: number
  }>(),
  {
    selectedIndex: null,
    size: 260
  }
)

const emit = defineEmits<{
  (e: 'select', index: number): void
}>()

const center = computed(() => props.size / 2)
const maxRadius = computed(() => props.size / 2 - 46)
const count = computed(() => Math.max(3, props.indicators.length))

// 计算多边形各顶点的极坐标转换
function getPoint(index: number, ratio: number) {
  const angle = (2 * Math.PI * index) / count.value - Math.PI / 2
  const r = maxRadius.value * Math.max(0, Math.min(1, ratio))
  return {
    x: center.value + r * Math.cos(angle),
    y: center.value + r * Math.sin(angle)
  }
}

// 4 层同心网格 (0.25, 0.5, 0.75, 1.0)
const gridRings = computed(() => {
  const steps = [0.25, 0.5, 0.75, 1.0]
  return steps.map((ratio) => {
    const pts = []
    for (let i = 0; i < count.value; i++) {
      const p = getPoint(i, ratio)
      pts.push(`${p.x.toFixed(1)},${p.y.toFixed(1)}`)
    }
    return { points: pts.join(' ') }
  })
})

// 最外层顶点
const outerVertices = computed(() => {
  const res = []
  for (let i = 0; i < count.value; i++) {
    res.push(getPoint(i, 1.0))
  }
  return res
})

// 候选人实际评分（0~4 档，归一化到 0~1）
const candidateScores = computed(() => {
  return props.indicators.map((_, idx) => {
    const val = props.candidateValues[idx]
    return typeof val === 'number' ? val : 2
  })
})

const candidateVertices = computed(() => {
  return candidateScores.value.map((score, idx) => {
    const maxVal = props.indicators[idx]?.max || 4
    return getPoint(idx, score / maxVal)
  })
})

const candidatePoints = computed(() => {
  return candidateVertices.value.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
})

// 岗位基准多边形（默认全为 3/4 = 0.75）
const benchmarkPoints = computed(() => {
  const pts = []
  for (let i = 0; i < count.value; i++) {
    const bench = props.benchmarkValues?.[i] ?? 3
    const maxVal = props.indicators[i]?.max || 4
    const p = getPoint(i, bench / maxVal)
    pts.push(`${p.x.toFixed(1)},${p.y.toFixed(1)}`)
  }
  return pts.join(' ')
})

// 轴标签位置微调
const labelPositions = computed(() => {
  const offsetR = maxRadius.value + 16
  return props.indicators.map((ind, idx) => {
    const angle = (2 * Math.PI * idx) / count.value - Math.PI / 2
    const x = center.value + offsetR * Math.cos(angle)
    const y = center.value + offsetR * Math.sin(angle)

    let anchor = 'middle'
    const cosVal = Math.cos(angle)
    if (cosVal > 0.3) anchor = 'start'
    else if (cosVal < -0.3) anchor = 'end'

    let baseline = 'middle'
    const sinVal = Math.sin(angle)
    if (sinVal < -0.7) baseline = 'auto'
    else if (sinVal > 0.7) baseline = 'hanging'

    return {
      name: ind.name.length > 7 ? ind.name.slice(0, 6) + '..' : ind.name,
      fullName: ind.name,
      x,
      y,
      anchor,
      baseline
    }
  })
})

function handleSelect(idx: number) {
  emit('select', idx)
}
</script>

<style scoped lang="scss">
.radar-chart-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  user-select: none;

  .radar-svg {
    overflow: visible;

    .candidate-polygon {
      transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
    }

    .vertex-circle {
      cursor: pointer;
      transition: all 0.2s ease;

      &:hover {
        r: 7;
        fill: #1d4ed8;
      }
    }

    .label-group {
      cursor: pointer;
      transition: all 0.2s ease;

      .radar-label-text {
        font-size: 11px;
        font-weight: 600;
        fill: #475569;
        transition: fill 0.2s ease;
      }

      .radar-score-text {
        font-size: 9.5px;
        font-weight: 700;
        fill: #2563eb;
        opacity: 0.9;
      }

      &:hover, &.active {
        .radar-label-text {
          fill: #2563eb;
          font-weight: 700;
        }
      }
    }
  }

  .radar-legend {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-top: 6px;
    font-size: 11px;
    color: #64748b;

    .legend-item {
      display: flex;
      align-items: center;
      gap: 5px;

      .legend-line {
        display: inline-block;
        width: 16px;
        height: 3px;
        border-radius: 2px;

        &.candidate-line {
          background: #2563eb;
        }

        &.benchmark-line {
          background: #94a3b8;
          border-top: 1px dashed #64748b;
          height: 0;
        }
      }
    }
  }
}
</style>
