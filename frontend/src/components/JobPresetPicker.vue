<template>
  <div class="job-preset-picker">
    <div class="picker-header">
      <span class="picker-title">{{ $t('job.quickSelect') }}</span>
      <span class="picker-desc">选择预设模板可一键填入岗位要求与职责详情</span>
    </div>

    <div class="category-tabs">
      <el-scrollbar>
        <div class="tab-wrapper">
          <div
            v-for="category in displayCategories"
            :key="category.id"
            class="category-tab"
            :class="{ active: activeCategory === category.id }"
            @click="activeCategory = category.id"
          >
            {{ $t(category.labelKey) }}
            <span v-if="category.id === 'custom' && customPresets.length" class="badge-count">
              {{ customPresets.length }}
            </span>
          </div>
        </div>
      </el-scrollbar>
    </div>

    <div class="preset-list">
      <div
        v-for="preset in currentPresets"
        :key="preset.id"
        class="preset-item"
        :class="{ selected: selectedId === preset.id, 'is-custom': preset.isCustom }"
        @click="selectPreset(preset)"
      >
        <div class="preset-top">
          <span class="preset-name">{{ preset.name }}</span>
          <el-button
            v-if="preset.isCustom"
            link
            type="danger"
            size="small"
            class="delete-btn"
            title="删除此自定义模板"
            @click.stop="handleDeleteCustom(preset)"
          >
            <el-icon><Delete /></el-icon>
          </el-button>
        </div>
        <span class="preset-info">
          {{ preset.experienceYears }}{{ $t('common.year') }} · {{ preset.educationLevel || '学历不限' }}
        </span>
        <span v-if="preset.requiredSkills && preset.requiredSkills.length" class="preset-skills">
          {{ preset.requiredSkills.slice(0, 3).join(' / ') }}
        </span>
      </div>

      <div
        v-if="activeCategory === 'custom' && currentPresets.length === 0"
        class="empty-custom-hint"
      >
        <span>暂无自定义模板。在下方配置岗位后，点击「保存为新模板」即可收录到此处。</span>
      </div>

      <div
        class="preset-item custom"
        :class="{ selected: selectedId === 'custom' }"
        @click="selectCustom"
      >
        <el-icon><Plus /></el-icon>
        <span>{{ $t('job.customJob') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { Plus, Delete } from '@element-plus/icons-vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import {
  jobCategories,
  getCustomJobPresets,
  deleteCustomJobPreset,
  type JobPreset,
  type JobCategory
} from '../data/jobPresets'

const props = defineProps<{
  selectedId?: string
}>()

const emit = defineEmits<{
  (e: 'select', preset: JobPreset | null): void
}>()

const activeCategory = ref('medical')
const customPresets = ref<JobPreset[]>(getCustomJobPresets())

function refresh() {
  customPresets.value = getCustomJobPresets()
}

defineExpose({
  refresh
})

const displayCategories = computed<JobCategory[]>(() => {
  return [
    ...jobCategories,
    {
      id: 'custom',
      labelKey: 'job.categories.custom',
      presets: []
    }
  ]
})

const currentPresets = computed(() => {
  if (activeCategory.value === 'custom') {
    return customPresets.value
  }
  const category = jobCategories.find(c => c.id === activeCategory.value)
  return category?.presets || []
})

function selectPreset(preset: JobPreset) {
  emit('select', preset)
}

function selectCustom() {
  emit('select', null)
}

function handleDeleteCustom(preset: JobPreset) {
  ElMessageBox.confirm(`确定要删除自定义模板「${preset.name}」吗？`, '提示', {
    confirmButtonText: '确定删除',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    deleteCustomJobPreset(preset.id)
    refresh()
    ElMessage.success('已删除自定义模板')
    if (props.selectedId === preset.id) {
      emit('select', null)
    }
  }).catch(() => {})
}
</script>

<style scoped lang="scss">
$primary: #1e3a5f;
$primary-light: #2d5a87;
$primary-lighter: #e8f0f7;
$accent: #0ea5e9;

$bg-section: #fafbfc;
$border: #e2e7ed;
$divider: #eef1f5;

$text-primary: #1a2332;
$text-secondary: #5a6678;
$text-muted: #8b95a5;

$radius-sm: 6px;
$radius-md: 10px;

.job-preset-picker {
  background-color: $bg-section;
  border: 1px solid $divider;
  border-radius: $radius-md;
  overflow: hidden;
  margin-bottom: 20px;
}

.picker-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 18px;
  border-bottom: 1px solid $divider;

  .picker-title {
    font-size: 14px;
    font-weight: 600;
    color: $text-primary;
  }

  .picker-desc {
    font-size: 12px;
    color: $text-muted;
  }
}

.category-tabs {
  background-color: white;
  border-bottom: 1px solid $divider;

  .tab-wrapper {
    display: flex;
    gap: 6px;
    padding: 10px 14px;
  }

  .category-tab {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 14px;
    font-size: 13px;
    color: $text-secondary;
    border-radius: $radius-sm;
    cursor: pointer;
    transition: all 0.2s ease;
    white-space: nowrap;

    .badge-count {
      display: inline-block;
      font-size: 10.5px;
      line-height: 1;
      padding: 2px 5px;
      border-radius: 8px;
      background-color: #0ea5e9;
      color: #fff;
    }

    &:hover {
      background-color: $primary-lighter;
      color: $primary;
    }

    &.active {
      background-color: $primary;
      color: white;
      font-weight: 500;

      .badge-count {
        background-color: rgba(255, 255, 255, 0.3);
      }
    }
  }
}

.preset-list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding: 14px;
  background-color: white;
}

.preset-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 14px;
  background-color: $bg-section;
  border: 1px solid $divider;
  border-radius: $radius-sm;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 150px;
  max-width: 220px;
  position: relative;

  &:hover {
    border-color: $primary-light;
    background-color: $primary-lighter;

    .delete-btn {
      opacity: 1;
    }
  }

  &.selected {
    border-color: $primary;
    background-color: $primary-lighter;
    box-shadow: 0 0 0 2px rgba(30, 58, 95, 0.12);

    .preset-name {
      color: $primary;
    }
  }

  .preset-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 4px;
  }

  .delete-btn {
    opacity: 0.3;
    padding: 0 2px;
    height: auto;
    font-size: 13px;
    transition: opacity 0.2s;

    &:hover {
      opacity: 1;
    }
  }

  .preset-name {
    font-size: 13px;
    font-weight: 600;
    color: $text-primary;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .preset-info {
    font-size: 11px;
    color: $text-muted;
  }

  .preset-skills {
    font-size: 10.5px;
    color: $text-secondary;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0.85;
  }

  &.custom {
    flex-direction: row;
    align-items: center;
    justify-content: center;
    gap: 6px;
    color: $text-secondary;
    font-size: 13px;
    border-style: dashed;
    min-height: 52px;

    .el-icon {
      font-size: 14px;
    }

    &:hover {
      color: $primary;
    }
  }
}

.empty-custom-hint {
  width: 100%;
  padding: 16px 12px;
  font-size: 12.5px;
  color: $text-muted;
  background-color: #f8fafc;
  border-radius: $radius-sm;
  border: 1px dashed $border;
}
</style>
