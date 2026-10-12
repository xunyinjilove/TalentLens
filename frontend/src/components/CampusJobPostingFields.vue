<template>
  <section class="campus-fields">
    <div class="campus-heading">
      <div><strong>校园招聘专属设置</strong><span>根据 51job 校招表单填写</span></div>
      <span class="campus-badge">在校生 / 应届生</span>
    </div>
    <div class="campus-grid">
      <el-form-item label="岗位性质">
        <el-radio-group :model-value="employmentType" @update:model-value="emit('update:employmentType', $event)">
          <el-radio-button label="全职">全职</el-radio-button>
          <el-radio-button label="兼职">兼职</el-radio-button>
          <el-radio-button label="实习">实习</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="学历">
        <el-select :model-value="education" style="width: 100%" @update:model-value="emit('update:education', $event)">
          <el-option v-for="item in educationOptions" :key="item" :label="item" :value="item" />
        </el-select>
      </el-form-item>
    </div>
    <div class="major-heading">
      <strong>专业要求（可选，最多 {{ catalog.maxSelections }} 项）</strong>
      <span>已选 {{ majors.length }} / {{ catalog.maxSelections }} · 本地目录</span>
    </div>
    <div v-if="majors.length" class="selected-majors">
      <el-tag v-for="item in majors" :key="item.value" closable :title="item.path.join(' > ')" @close="removeMajor(item.value)">{{ item.path.at(-1) }}</el-tag>
    </div>
    <el-input v-model="search" clearable placeholder="搜索专业名称，例如：计算机科学与技术" class="major-search" />
    <div v-if="search.trim()" class="major-search-results">
      <span class="column-title">匹配 {{ matchingMajors.length }} 项<span v-if="matchingMajors.length > 100">，显示前 100 项</span></span>
      <button v-for="item in matchingMajors.slice(0, 100)" :key="item.value" type="button" class="major-option" :class="{ selected: majors.some(selected => selected.value === item.value) }" @click="toggleMajor(item)">
        <strong>{{ item.path.at(-1) }}</strong><small>{{ item.path.slice(0, -1).join(' > ') }}</small>
      </button>
      <p v-if="!matchingMajors.length" class="empty">本地目录没有匹配项，请尝试其他专业名称。</p>
    </div>
    <div v-else class="major-columns">
      <div class="major-column">
        <span class="column-title">一级学科</span>
        <button v-for="root in catalog.majors" :key="root.value" type="button" class="major-option" :class="{ active: root.value === selectedRoot }" @click="selectRoot(root.value)">{{ root.name }}<span>›</span></button>
      </div>
      <div class="major-column">
        <span class="column-title">二级学科</span>
        <button v-for="branch in branches" :key="branch.value" type="button" class="major-option" :class="{ active: branch.value === selectedBranch }" @click="selectedBranch = branch.value">{{ branch.name }}<span>›</span></button>
        <span v-if="!branches.length" class="empty">请选择一级学科</span>
      </div>
      <div class="major-column">
        <span class="column-title">具体专业</span>
        <button v-for="leaf in leaves" :key="leaf.value" type="button" class="major-option" :class="{ selected: majors.some(item => item.value === leaf.value) }" @click="toggleMajor({ value: leaf.value, path: [currentRoot!.name, currentBranch!.name, leaf.name] })">{{ leaf.name }}<span v-if="majors.some(item => item.value === leaf.value)">✓</span></button>
        <span v-if="!leaves.length" class="empty">请选择二级学科</span>
      </div>
    </div>
    <p class="catalog-note">专业目录于 {{ catalog.capturedAt.slice(0, 10) }} 从 51job 校招页面抓取；提交前仍会核对平台当前选项。</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import catalog from '../data/51job_majors.json'

export interface CampusMajor { value: string, path: string[] }

const props = defineProps<{
  employmentType: string
  education: string
  educationOptions: string[]
  majors: CampusMajor[]
}>()
const emit = defineEmits<{
  'update:employmentType': [value: string]
  'update:education': [value: string]
  'update:majors': [value: CampusMajor[]]
}>()

const selectedRoot = ref('')
const selectedBranch = ref('')
const search = ref('')
const currentRoot = computed(() => catalog.majors.find(item => item.value === selectedRoot.value))
const branches = computed(() => currentRoot.value?.children || [])
const currentBranch = computed(() => branches.value.find(item => item.value === selectedBranch.value))
const leaves = computed(() => currentBranch.value?.children || [])
const allMajors: CampusMajor[] = catalog.majors.flatMap(root => (root.children || []).flatMap(branch =>
  (branch.children || []).map(leaf => ({ value: leaf.value, path: [root.name, branch.name, leaf.name] }))))
const matchingMajors = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return query ? allMajors.filter(item => item.path.join(' ').toLocaleLowerCase().includes(query)) : []
})

function selectRoot(value: string) {
  selectedRoot.value = value
  selectedBranch.value = ''
}

function toggleMajor(major: CampusMajor) {
  if (props.majors.some(item => item.value === major.value)) {
    removeMajor(major.value)
  } else if (props.majors.length >= catalog.maxSelections) {
    ElMessage.warning(`51job 最多选择 ${catalog.maxSelections} 项专业`)
  } else if (props.majors.some(item => item.path.at(-1) === major.path.at(-1))) {
    ElMessage.warning('同名专业不能重复选择')
  } else {
    emit('update:majors', [...props.majors, major])
  }
}

function removeMajor(value: string) {
  emit('update:majors', props.majors.filter(item => item.value !== value))
}
</script>

<style scoped>
.campus-fields { margin: 2px 0 16px; padding: 14px; border: 1px solid #bfdbfe; border-radius: 10px; background: #f8fbff; }
.campus-heading, .major-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.campus-heading { margin-bottom: 12px; }
.campus-heading strong, .major-heading strong { display: block; color: #1e3a5f; }
.campus-heading span, .major-heading span, .catalog-note { color: #64748b; font-size: 12px; }
.campus-heading strong + span { display: block; margin-top: 3px; }
.campus-badge { padding: 5px 9px; border-radius: 20px; background: #dbeafe; color: #1d4ed8 !important; white-space: nowrap; }
.campus-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0 16px; }
.major-heading { margin-bottom: 8px; }
.selected-majors { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
.major-search { margin-bottom: 8px; }
.major-columns { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
.major-column, .major-search-results { min-height: 180px; max-height: 220px; overflow-y: auto; padding: 7px; border-radius: 6px; background: white; border: 1px solid #e2e8f0; }
.column-title { display: block; margin: 3px 6px 7px; color: #64748b; font-size: 12px; }
.major-option { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; width: 100%; padding: 7px 8px; border: 0; border-radius: 5px; background: transparent; color: #334155; text-align: left; cursor: pointer; }
.major-option:hover, .major-option.active, .major-option.selected { background: #dbeafe; color: #1d4ed8; }
.major-option small { color: #64748b; }
.major-search-results .major-option { border-bottom: 1px solid #f1f5f9; }
.empty { display: block; margin: 10px 7px; color: #94a3b8; font-size: 12px; }
.catalog-note { margin: 9px 0 0; line-height: 1.4; }
@media (max-width: 620px) { .campus-grid { grid-template-columns: 1fr; } }
</style>
