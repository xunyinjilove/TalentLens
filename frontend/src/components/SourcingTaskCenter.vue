<template>
  <div v-if="tasks.length" class="sourcing-task-center">
    <button class="sourcing-task-trigger" @click="open = true">
      <span class="task-dot" :class="{ running: runningCount > 0 }"></span>
      寻才任务 <strong>{{ runningCount > 0 ? `${runningCount} 运行中` : `${tasks.length} 条记录` }}</strong>
    </button>
    <el-drawer v-model="open" title="寻才任务与分析进度" size="430px" class="sourcing-drawer">
      <p class="wb-muted">检索和 AI 分析分别计数。关闭面板或切换页面不会停止任务。</p>
      <article v-for="task in tasks" :key="task.id" class="wb-card sourcing-task-card">
        <div class="task-title"><strong>{{ task.projectName }}</strong><span class="wb-pill" :class="task.status === 'failed' ? 'danger' : task.status === 'running' ? 'warn' : ''">{{ statusText(task.status) }}</span></div>
        <div class="wb-muted">{{ task.config.keyword }} · {{ (task.config.platforms || []).join('、') }}</div>
        <div class="task-metrics"><span>已抓取 <b>{{ task.capturedCount }}/{{ task.targetCount }}</b></span><span>已分析 <b>{{ task.analyzedCount }}</b></span><span>失败 <b>{{ task.failedCount }}</b></span></div>
        <el-progress :percentage="task.targetCount ? Math.min(100, Math.round(task.capturedCount / task.targetCount * 100)) : 0" :show-text="false" />
        <p class="wb-muted task-message">{{ task.message || '等待更新' }}</p>
        <div class="wb-actions">
          <el-button size="small" @click="viewTalent(task.projectId)">查看已抓取简历</el-button>
          <el-button v-if="task.status === 'running'" size="small" type="danger" plain @click="stopTask">停止</el-button>
          <el-button v-if="['interrupted','stopped','partial','failed'].includes(task.status) && task.capturedCount < task.targetCount" size="small" type="primary" @click="resumeTask(task.id)">手动继续</el-button>
        </div>
      </article>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useRecruitmentStore } from '../composables/useRecruitmentStore'

const store = useRecruitmentStore()
const router = useRouter()
const open = ref(false)
const tasks = computed(() => store.sourcingTasks.slice(0, 20))
const runningCount = computed(() => tasks.value.filter(task => task.status === 'running').length)
const labels: Record<string, string> = { queued: '排队中', running: '检索中', completed: '检索完成', partial: '部分完成', failed: '失败', stopped: '已停止', interrupted: '任务已中断' }
function statusText(status: string) { return labels[status] || status }
function viewTalent(projectId: string) { open.value = false; void router.push({ path: '/talent', query: { project: projectId } }) }
async function resumeTask(id: string) {
  try { await store.resumeSourcingTask(id); ElMessage.success('已启动剩余目标的检索') }
  catch (err: any) { ElMessage.error(err?.message || String(err)) }
}
async function stopTask() {
  try { await store.stopSourcingTask(); ElMessage.info('已请求停止，正在保存当前结果') }
  catch (err: any) { ElMessage.error(err?.message || String(err)) }
}
onMounted(() => { void store.loadSourcingTasks() })
</script>

<style scoped>
.sourcing-task-center { position: fixed; right: 18px; bottom: 22px; z-index: 1000; }
.sourcing-task-trigger { border: 1px solid var(--wb-border); background: var(--wb-surface); color: var(--wb-text); box-shadow: 0 12px 32px rgba(31,41,80,.16); border-radius: 999px; padding: 11px 17px; cursor: pointer; display: flex; align-items: center; gap: 8px; }
.sourcing-task-trigger strong { color: var(--wb-brand); }
.task-dot { width: 8px; height: 8px; border-radius: 50%; background: #94a3b8; }
.task-dot.running { background: #10b981; box-shadow: 0 0 0 4px rgba(16,185,129,.15); }
.sourcing-task-card { margin-top: 12px; display: flex; flex-direction: column; gap: 12px; }
.task-title { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.task-metrics { display: flex; justify-content: space-between; font-size: 12px; }
.task-metrics b { color: var(--wb-brand); }
.task-message { font-size: 12px; margin: 0; }
</style>
