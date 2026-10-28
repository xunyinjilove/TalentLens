<template>
  <el-dialog
    v-model="visible"
    :title="`人才库一键智能筛选 · ${jobTitle || '职位专属库'}`"
    width="680px"
    :close-on-click-modal="!filtering"
    append-to-body
  >
    <div class="one-click-filter-content" v-loading="loading">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      >
        <template #default>
          根据设定门槛智能排查当前专属库在库候选人。被排除人选将自动归档移入【已淘汰/不合适】，并记录淘汰审计日志。
        </template>
      </el-alert>

      <el-form label-position="top" size="default">
        <!-- 第 1 行：学历与工龄 -->
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="最低学历要求">
              <el-select v-model="criteria.minEducation" style="width: 100%" placeholder="选择学历门槛">
                <el-option label="不限" value="" />
                <el-option label="大专及以上" value="大专" />
                <el-option label="本科及以上" value="本科" />
                <el-option label="硕士及以上" value="硕士" />
                <el-option label="博士" value="博士" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="最低工作年限（年）">
              <el-input-number v-model="criteria.minWorkYears" :min="0" :max="50" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 第 2 行：年龄区间与薪资期望 -->
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="年龄区间要求（岁）">
              <div style="display: flex; align-items: center; gap: 8px">
                <el-input-number v-model="criteria.minAge" :min="0" :max="80" placeholder="最小" style="width: 48%" />
                <span style="color: #94a3b8">至</span>
                <el-input-number v-model="criteria.maxAge" :min="0" :max="80" placeholder="最大" style="width: 48%" />
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="期望薪资上限（千元/月）">
              <el-input-number v-model="criteria.maxExpectedSalary" :min="0" :max="200" placeholder="如 15 代表 15k/月，0 为不限" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 第 3 行：性别与求职状态 -->
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="性别要求">
              <el-select v-model="criteria.gender" style="width: 100%" placeholder="性别要求">
                <el-option label="不限" value="" />
                <el-option label="仅限男性" value="男" />
                <el-option label="仅限女性" value="女" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="求职状态限制">
              <el-select v-model="criteria.jobSeekingStatus" style="width: 100%" placeholder="求职状态">
                <el-option label="不限" value="" />
                <el-option label="仅离职（随时到岗）" value="离职" />
                <el-option label="在职（考虑机会）" value="在职" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 第 4 行：专业背景关键词匹配 -->
        <el-form-item label="专业背景关键词匹配（满足其一即可通过）">
          <el-input
            v-model="criteria.majorKeywords"
            placeholder="如：医学, 药学, 市场营销, 生物制药（逗号分隔，留空表示不限专业）"
            clearable
          />
        </el-form-item>

        <!-- 第 5 行：目标城市工作意愿限制 -->
        <el-form-item label="目标城市工作意愿">
          <div style="display: flex; align-items: center; gap: 12px; width: 100%">
            <el-checkbox v-model="criteria.rejectNonLocalWillingness">排除明确不愿在目标城市工作的候选人</el-checkbox>
            <el-input
              v-if="criteria.rejectNonLocalWillingness"
              v-model="criteria.targetCity"
              placeholder="目标城市名称，如：上海"
              style="width: 180px"
            />
          </div>
        </el-form-item>

        <!-- 双向联动与保存设置 -->
        <div style="background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; padding: 12px 16px; margin-top: 8px">
          <div style="font-weight: 600; color: #1e293b; font-size: 13px; margin-bottom: 8px">双向联动与规则保存</div>
          <div class="linkage-options">
            <el-checkbox v-model="autoMarkLocalRejected" disabled>
              <span style="color: #475569">本地人才库：将排除人选招聘阶段自动转为【已淘汰/不合适】并归档</span>
            </el-checkbox>
            <el-checkbox v-model="criteria.autoRejectPlatform">
              <span style="color: #1e293b; font-weight: 500">平台自动化：对来源于 51job 的人选，自动在 51job 企业端点击标记【不合适】并回填对应淘汰理由</span>
            </el-checkbox>
            <el-checkbox v-model="saveAsDefault">
              <span style="color: #2563eb; font-weight: 500">同步保存为本职位的长期硬性条件（后续自动同步导入时将默认以此规则初筛）</span>
            </el-checkbox>
          </div>
        </div>

        <!-- 筛选结果卡片 -->
        <el-alert
          v-if="filterResult"
          type="success"
          :closable="false"
          show-icon
          style="margin-top: 16px"
        >
          <template #title>
            <b>筛选执行完成：</b>核验 {{ filterResult.totalChecked }} 人 · 排除不合格 {{ filterResult.unfitCount }} 人（其中 51job 平台点击不合适 {{ filterResult.platformRejectedCount }} 人） · 保留合格 {{ filterResult.activeRemainingCount }} 人
          </template>
        </el-alert>
      </el-form>
    </div>

    <template #footer>
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span style="font-size: 12px; color: #64748b">
          {{ filterResult ? '已完成筛选，点击确定关闭并刷新' : '点击下方按钮即刻启动智能筛选' }}
        </span>
        <div style="display: flex; gap: 8px">
          <el-button @click="visible = false" :disabled="filtering">取消</el-button>
          <el-button
            type="primary"
            :loading="filtering"
            @click="executeFilter"
          >
            <el-icon style="margin-right: 4px"><Filter /></el-icon>
            {{ filterResult ? '再次执行筛选' : '立即执行一键筛选' }}
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { ElMessage, ElNotification } from 'element-plus'
import { Filter } from '@element-plus/icons-vue'

const props = defineProps<{
  modelValue: boolean
  projectId: string
  jobTitle?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'filtered': [result: any]
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const loading = ref(false)
const filtering = ref(false)
const autoMarkLocalRejected = ref(true)
const saveAsDefault = ref(true)
const filterResult = ref<any>(null)

const criteria = ref({
  projectId: '',
  minEducation: '',
  minWorkYears: 0,
  minAge: 0,
  maxAge: 0,
  maxExpectedSalary: 0,
  gender: '',
  jobSeekingStatus: '',
  majorKeywords: '',
  rejectNonLocalWillingness: false,
  targetCity: '',
  complianceConfirmed: true,
  autoRejectPlatform: true,
  updatedAt: ''
})

watch(() => props.modelValue, async (val) => {
  if (val && props.projectId) {
    filterResult.value = null
    await loadCriteria()
  }
})

async function loadCriteria() {
  loading.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const res = await api.GetTalentManagementStrictCriteria(props.projectId)
    if (res) {
      criteria.value = {
        projectId: props.projectId,
        minEducation: res.minEducation || '',
        minWorkYears: res.minWorkYears || 0,
        minAge: res.minAge || 0,
        maxAge: res.maxAge || 0,
        maxExpectedSalary: res.maxExpectedSalary || 0,
        gender: res.gender || '',
        jobSeekingStatus: res.jobSeekingStatus || '',
        majorKeywords: res.majorKeywords || '',
        rejectNonLocalWillingness: Boolean(res.rejectNonLocalWillingness),
        targetCity: res.targetCity || '',
        complianceConfirmed: true,
        autoRejectPlatform: res.autoRejectPlatform !== false,
        updatedAt: res.updatedAt || ''
      }
    }
  } catch (err: any) {
    ElMessage.warning('未能加载岗位历史配置，将使用默认选项')
  } finally {
    loading.value = false
  }
}

async function executeFilter() {
  if (!props.projectId) {
    ElMessage.error('未选定职位分库')
    return
  }

  filtering.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    criteria.value.projectId = props.projectId
    criteria.value.complianceConfirmed = true

    const result = await api.OneClickFilterTalentPoolWithConfig(
      props.projectId,
      criteria.value,
      saveAsDefault.value
    )

    filterResult.value = result

    ElNotification({
      title: '一键筛选执行完毕',
      message: `共核验 ${result.totalChecked} 人，排除并标记不合适 ${result.unfitCount} 人（平台点击 ${result.platformRejectedCount} 人），剩余活跃 ${result.activeRemainingCount} 人。`,
      type: result.unfitCount > 0 ? 'success' : 'info',
      duration: 5000
    })

    emit('filtered', result)
  } catch (err: any) {
    ElMessage.error(err?.message || '一键筛选失败')
  } finally {
    filtering.value = false
  }
}
</script>

<style scoped>
.one-click-filter-content {
  padding: 4px 0;
}
.linkage-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
.linkage-options :deep(.el-checkbox) {
  display: flex;
  align-items: flex-start;
  height: auto;
  min-width: 0;
  margin-right: 0;
  white-space: normal;
}
.linkage-options :deep(.el-checkbox__input) {
  margin-top: 4px;
}
.linkage-options :deep(.el-checkbox__label) {
  min-width: 0;
  white-space: normal;
  overflow-wrap: anywhere;
  line-height: 1.6;
}
</style>
