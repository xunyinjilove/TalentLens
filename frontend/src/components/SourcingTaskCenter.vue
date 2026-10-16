<template>
  <div v-if="tasks.length" class="sourcing-task-center">
    <button class="sourcing-task-trigger" @click="store.taskCenterOpen = true">
      <span class="task-dot" :class="{ running: runningCount > 0 }"></span>
      寻才任务 <strong>{{ runningCount > 0 ? `${runningCount} 运行中` : `${tasks.length} 条记录` }}</strong>
    </button>
    <el-drawer v-model="store.taskCenterOpen" size="min(430px, 100%)" class="sourcing-drawer">
      <template #header>
        <div class="sourcing-drawer-header">
          <strong>寻才任务与分析进度</strong>
          <el-button size="small" text @click="openAIAssistant">AI 招聘助理</el-button>
        </div>
      </template>
      <div class="task-toolbar">
        <p class="wb-muted">寻才只抓取入库，分析由你手动启动。人数不足时定时补抓，关闭面板不会停止任务。</p>
        <el-button size="small" plain type="danger" :disabled="finishedCount === 0" @click="clearTasks">清空已结束</el-button>
      </div>
      <article v-for="task in tasks" :key="task.id" class="wb-card sourcing-task-card">
        <div class="task-title"><strong>{{ task.projectName }}</strong><span class="wb-pill" :class="task.status === 'failed' ? 'danger' : task.status === 'running' ? 'warn' : ''">{{ statusText(task.status) }}</span></div>
        <div class="wb-muted">{{ task.config.keyword }} · {{ (task.config.platforms || []).join('、') }}</div>
        <div class="task-metrics"><span>已抓取 <b>{{ task.capturedCount }}/{{ task.targetCount }}</b></span><template v-if="task.config.autoAnalyze"><span>历史已分析 <b>{{ task.analyzedCount }}</b></span><span>历史失败 <b>{{ task.failedCount }}</b></span></template><span v-else>分析方式 <b>手动</b></span></div>
        <el-progress :percentage="task.targetCount ? Math.min(100, Math.round(task.capturedCount / task.targetCount * 100)) : 0" :show-text="false" />
        <p class="wb-muted task-message">{{ task.message || '等待更新' }}</p>
        <div class="wb-actions">
          <el-button size="small" @click="viewTalent(task.projectId)">查看已抓取简历</el-button>
          <el-button v-if="task.status === 'running'" size="small" type="danger" plain @click="stopTask">停止</el-button>
          <el-button v-if="['interrupted','stopped','partial','failed'].includes(task.status) && task.capturedCount < task.targetCount" size="small" type="primary" @click="resumeTask(task.id)">手动继续</el-button>
          <el-button class="task-delete" size="small" text type="danger" :disabled="['queued','running'].includes(task.status)" :title="['queued','running'].includes(task.status) ? '请先停止任务并等待结果保存' : '删除此任务记录'" @click="deleteTask(task.id)">删除</el-button>
        </div>
      </article>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRecruitmentStore } from '../composables/useRecruitmentStore'
import { useAIAssistantStore } from '../composables/useAIAssistantStore'

const store = useRecruitmentStore()
const assistantStore = useAIAssistantStore()
const router = useRouter()
const tasks = computed(() => store.sourcingTasks)
const runningCount = computed(() => tasks.value.filter(task => task.status === 'running').length)
const finishedCount = computed(() => tasks.value.filter(task => !['queued', 'running'].includes(task.status)).length)
const labels: Record<string, string> = { queued: '排队中', running: '检索中', completed: '检索完成', partial: '部分完成', failed: '检索失败', stopped: '已停止', interrupted: '任务已中断' }
function statusText(status: string) { return labels[status] || status }
function viewTalent(projectId: string) { store.taskCenterOpen = false; void router.push({ path: '/talent', query: { project: projectId } }) }
function openAIAssistant() { store.taskCenterOpen = false; assistantStore.openDrawer() }
async function resumeTask(id: string) {
  try { await store.resumeSourcingTask(id); ElMessage.success('已启动剩余目标的检索') }
  catch (err: any) { ElMessage.error(err?.message || String(err)) }
}
async function stopTask() {
  try { await store.stopSourcingTask(); ElMessage.info('已请求停止，正在保存当前结果') }
  catch (err: any) { ElMessage.error(err?.message || String(err)) }
}
async function deleteTask(id: string) {
  try {
    await ElMessageBox.confirm('只删除这条任务记录，已抓取简历和分析结果会保留。', '删除寻才任务', { type: 'warning', confirmButtonText: '删除记录', cancelButtonText: '取消' })
    await store.deleteSourcingTask(id)
    ElMessage.success('任务记录已删除')
  } catch (err: any) {
    if (err !== 'cancel' && err !== 'close') ElMessage.error(err?.message || String(err))
  }
}
async function clearTasks() {
  try {
    await ElMessageBox.confirm('清空全部已结束任务记录；运行中的任务会保留。已抓取简历和分析结果不受影响。', '清空任务记录', { type: 'warning', confirmButtonText: '清空记录', cancelButtonText: '取消' })
    const removed = await store.clearFinishedSourcingTasks()
    ElMessage.success(`已清空 ${removed} 条任务记录`)
  } catch (err: any) {
    if (err !== 'cancel' && err !== 'close') ElMessage.error(err?.message || String(err))
  }
}
onMounted(() => { void store.loadSourcingTasks() })
</script>

<style scoped>
.sourcing-task-center { position: fixed; right: 18px; bottom: 22px; z-index: 1000; }
.sourcing-task-trigger { border: 1px solid var(--wb-border); background: var(--wb-surface); color: var(--wb-text); box-shadow: 0 12px 32px rgba(31,41,80,.16); border-radius: 999px; padding: 11px 17px; cursor: pointer; display: flex; align-items: center; gap: 8px; }
.sourcing-task-trigger strong { color: var(--wb-brand); }
.sourcing-drawer-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; }
.task-dot { width: 8px; height: 8px; border-radius: 50%; background: #94a3b8; }
.task-dot.running { background: #10b981; box-shadow: 0 0 0 4px rgba(16,185,129,.15); }
.sourcing-task-card { margin-top: 12px; display: flex; flex-direction: column; gap: 12px; }
.task-toolbar { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.task-toolbar p { margin: 0; flex: 1; }
.task-toolbar button { flex-shrink: 0; }
.task-title { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.task-metrics { display: flex; justify-content: space-between; font-size: 12px; }
.task-metrics b { color: var(--wb-brand); }
.task-message { font-size: 12px; margin: 0; }
.task-delete { margin-left: auto; }
</style>
