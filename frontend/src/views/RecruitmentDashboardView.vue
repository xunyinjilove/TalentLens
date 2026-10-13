<template>
  <div class="workbench-page">
    <header class="wb-header">
      <div><h1>招聘概览</h1><div class="wb-crumb">工作台 / 整体招聘态势</div></div>
      <div class="wb-actions">
        <el-button @click="refresh" :loading="recruitment.loading">刷新数据</el-button>
        <el-button type="primary" @click="router.push('/jobs')">查看职位</el-button>
      </div>
    </header>
    <main class="wb-content">
      <el-alert v-if="recruitment.error" type="warning" :title="recruitment.error" show-icon :closable="false" style="margin-bottom:18px" />
      <div class="wb-kpi-grid">
        <div v-for="card in kpis" :key="card.label" class="wb-card wb-kpi" role="button" tabindex="0" @click="router.push(card.route)" @keydown.enter="router.push(card.route)">
          <div class="wb-kpi-icon">{{ card.icon }}</div>
          <div class="wb-kpi-value">{{ card.value }}</div>
          <div class="wb-kpi-label">{{ card.label }}</div>
        </div>
      </div>
      <div class="wb-two-col">
        <section class="wb-card">
          <h2 class="wb-section-title">招聘漏斗</h2>
          <div v-if="funnel.length" class="funnel-list">
            <div v-for="step in funnel" :key="step.label" class="funnel-line">
              <span>{{ step.label }}</span>
              <div class="funnel-track"><div class="funnel-fill" :style="{ width: step.width + '%' }" /></div>
              <strong>{{ step.count }}</strong>
            </div>
          </div>
          <div v-else class="wb-empty">暂无候选人跟进数据</div>
        </section>
        <section class="wb-card">
          <h2 class="wb-section-title">近期入库</h2>
          <div v-if="recentCandidates.length" class="recent-list">
            <div v-for="item in recentCandidates" :key="item.projectCandidateId" class="recent-item" @click="router.push('/talent?project=' + item.projectId)">
              <span class="wb-avatar">{{ item.name.slice(0, 1) }}</span>
              <div><strong>{{ item.name }}</strong><div class="wb-muted">{{ item.projectName }} · {{ item.source }}</div></div>
              <span class="wb-muted">{{ formatDate(item.createdAt) }}</span>
            </div>
          </div>
          <div v-else class="wb-empty">暂无入库记录</div>
        </section>
      </div>
      <section class="wb-card" style="margin-top:18px">
        <h2 class="wb-section-title">待跟进</h2>
        <div v-if="dueCandidates.length" class="due-grid">
          <div v-for="item in dueCandidates" :key="item.projectCandidateId" class="due-item">
            <div><strong>{{ item.name }}</strong><span class="wb-muted"> · {{ item.projectName }}</span></div>
            <div class="wb-muted">负责人：{{ item.owner || '未分配' }} · {{ stageLabels[item.stage] || item.stage }}</div>
            <el-button size="small" @click="router.push('/pipeline?project=' + item.projectId)">查看跟进</el-button>
          </div>
        </div>
        <div v-else class="wb-empty">当前没有到期的跟进事项</div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useRecruitmentStore, stageLabels } from '../composables/useRecruitmentStore'

const router = useRouter()
const recruitment = useRecruitmentStore()
const kpis = computed(() => [
  { label: '招聘项目', value: recruitment.dashboard.openProjects, icon: '▣', route: '/jobs' },
  { label: '近七天新增人才', value: recruitment.dashboard.candidatesThisWeek, icon: '▤', route: '/talent' },
  { label: '待跟进', value: recruitment.dashboard.pendingFollowUps, icon: '✓', route: '/pipeline' },
  { label: '待举行面试', value: recruitment.dashboard.upcomingInterviews, icon: '◷', route: '/interviews' }
])
const funnel = computed(() => {
  const counts = recruitment.dashboard.stageCounts || {}
  const rows = [
    { label: '入库', count: Object.values(counts).reduce((a, b) => a + b, 0) },
    { label: '沟通', count: (counts.communicating || 0) + (counts.first_interview || 0) + (counts.second_interview || 0) + (counts.offer || 0) + (counts.hired || 0) },
    { label: '面试', count: (counts.first_interview || 0) + (counts.second_interview || 0) + (counts.offer || 0) + (counts.hired || 0) },
    { label: 'Offer', count: (counts.offer || 0) + (counts.hired || 0) },
    { label: '入职', count: counts.hired || 0 }
  ]
  return rows[0].count ? rows.map(row => ({ ...row, width: Math.max(4, Math.round(row.count / rows[0].count * 100)) })) : []
})
const recentCandidates = computed(() => recruitment.candidates.slice(0, 5))
const dueCandidates = computed(() => recruitment.candidates.filter(item => item.nextActionAt && new Date(item.nextActionAt).getTime() <= Date.now() && !['hired', 'rejected'].includes(item.stage)).slice(0, 6))

function formatDate(value: string) { return value ? new Date(value).toLocaleDateString('zh-CN') : '' }
async function refresh() { await Promise.all([recruitment.loadDashboard(), recruitment.loadCandidates()]) }
onMounted(() => { void refresh() })
</script>

<style scoped>
.funnel-list { display:flex; flex-direction:column; gap:17px; }
.funnel-line { display:grid; grid-template-columns:50px 1fr 36px; gap:12px; align-items:center; font-size:13px; }
.funnel-track { height:24px; border-radius:7px; background:var(--wb-surface-soft); overflow:hidden; }
.funnel-fill { height:100%; border-radius:7px; background:linear-gradient(90deg,#6366f1,#8b5cf6); }
.recent-list { display:flex; flex-direction:column; }
.recent-item { display:flex; gap:12px; align-items:center; padding:11px 0; border-bottom:1px solid var(--wb-border); cursor:pointer; }
.recent-item:last-child { border-bottom:none; }
.recent-item > div { flex:1; }
.recent-item .wb-muted { font-size:12px; }
.due-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:12px; }
.due-item { padding:13px; border:1px solid var(--wb-border); border-radius:10px; display:flex; flex-direction:column; align-items:flex-start; gap:7px; }
@media(max-width:760px) { .due-grid { grid-template-columns:1fr; } }
</style>
