<template>
  <el-dialog v-model="visible" :title="`${project?.name || '岗位'} · 岗位规则审批`" width="720px">
    <p class="wb-muted">模板和 AI 建议均先成为草稿。只有 HR 逐条确认后，新版本才用于后续评测；历史评测和人工核验记录保留。</p>
    <div class="wb-actions" style="margin:16px 0">
      <el-button :loading="busy" @click="suggest(false)">载入岗位模板建议</el-button>
      <el-button :loading="busy" type="primary" plain @click="suggest(true)">AI 补充建议</el-button>
    </div>
    <el-alert v-if="error" type="warning" :title="error" :closable="false" style="margin-bottom:12px" />
    <div v-if="drafts.length">
      <p class="wb-muted">请选择要生效的规则。红线只用于记录 HR 必须核实的准入条件；AI 不能做红线结论。</p>
      <div v-for="rule in drafts" :key="rule.id" class="rule-item">
        <el-checkbox v-model="approved[rule.id]">确认</el-checkbox>
        <span class="wb-pill" :class="rule.kind === 'redline' ? 'danger' : rule.kind === 'bonus' ? 'warn' : ''">{{ kindLabel(rule.kind) }}</span>
        <el-input v-model="edits[rule.id]" :maxlength="220" show-word-limit />
        <span class="wb-muted">{{ rule.source === 'ai' ? 'AI' : '模板' }}</span>
      </div>
      <div class="wb-actions" style="margin-top:16px"><el-input v-model="actor" placeholder="审批人" style="width:160px" /><el-button type="primary" :loading="busy" @click="approve">确认并生效新版本</el-button></div>
    </div>
    <div v-else class="wb-empty">当前没有待审批规则。可从岗位模板或 AI 生成建议。</div>
    <div v-if="effective.length" style="margin-top:20px"><strong>已审批版本</strong><p v-for="rule in effective" :key="rule.id" class="wb-muted">V{{ rule.version }} · {{ kindLabel(rule.kind) }} · {{ rule.criterion }}</p></div>
    <div v-if="legacyRules.length" style="margin-top:20px"><strong>历史项目规则 · 来源待确认</strong><p v-for="rule in legacyRules" :key="rule.id" class="wb-muted">{{ kindLabel(rule.kind) }} · {{ rule.criterion }}</p></div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { Project } from '../composables/useProjectStore'
import { useProjectStore } from '../composables/useProjectStore'

interface Rule { id: string; kind: string; criterion: string; source: string; status: string; version: number }
const props = defineProps<{ modelValue: boolean; project: Project | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const visible = computed({ get: () => props.modelValue, set: value => emit('update:modelValue', value) })
const projects = useProjectStore()
const rules = ref<Rule[]>([])
const edits = ref<Record<string,string>>({})
const approved = ref<Record<string,boolean>>({})
const actor = ref('HR')
const error = ref('')
const busy = ref(false)
const drafts = computed(() => rules.value.filter(rule => rule.status === 'draft'))
const effective = computed(() => rules.value.filter(rule => rule.status === 'approved'))
const legacyRules = computed(() => rules.value.filter(rule => rule.status === 'source_unconfirmed'))
function kindLabel(kind: string) { return ({ redline: '红线待核实', core: '核心能力', bonus: '加分项' } as Record<string,string>)[kind] || kind }
async function load() {
  if (!props.project) return
  const api: any = await import('../../wailsjs/go/main/App')
  rules.value = await api.ListJobRules(props.project.id) || []
  for (const rule of rules.value) {
    if (!(rule.id in edits.value)) edits.value[rule.id] = rule.criterion
    if (!(rule.id in approved.value)) approved.value[rule.id] = false
  }
}
async function suggest(withAI: boolean) {
  if (!props.project) return
  busy.value = true; error.value = ''
  try {
    const previous = new Set(rules.value.map(rule => rule.id))
    const api: any = await import('../../wailsjs/go/main/App')
    await api.SuggestJobRules(props.project.id, withAI)
    await load()
    if (withAI && !rules.value.some(rule => rule.source === 'ai' && !previous.has(rule.id))) {
      ElMessage.info('没有生成新的有岗位依据的规则，请补充职位职责或必需技能后重试')
    }
  } catch (err: any) { error.value = err?.message || String(err) }
  finally { busy.value = false }
}
async function approve() {
  if (!props.project) return
  busy.value = true; error.value = ''
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const version = await api.ApproveJobRules(props.project.id, actor.value, drafts.value.map(rule => ({ id: rule.id, criterion: edits.value[rule.id], approved: !!approved.value[rule.id] })))
    ElMessage.success(`岗位规则 V${version} 已生效`)
    await Promise.all([load(), projects.loadProjects()])
  } catch (err: any) { error.value = err?.message || String(err) }
  finally { busy.value = false }
}
watch(() => [props.modelValue, props.project?.id], () => { if (props.modelValue) void load() })
</script>

<style scoped>
.rule-item { display:flex; align-items:center; gap:8px; padding:9px 0; border-bottom:1px solid var(--wb-border); }
.rule-item .el-input { flex:1; }
</style>
