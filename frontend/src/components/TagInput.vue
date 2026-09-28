<template>
  <div class="tag-input-wrapper">
    <!-- 快捷填入区 -->
    <div v-if="presetList.length > 0 || customList.length > 0 || allowCustom" class="quick-tags-bar">
      <span class="quick-title">{{ quickTitle }}</span>
      
      <!-- 预设内置快捷标签 -->
      <button
        v-for="preset in presetList"
        :key="'preset-' + preset"
        type="button"
        class="quick-pill"
        :class="[
          tagType,
          { 'is-active': modelValue.includes(preset) }
        ]"
        :title="modelValue.includes(preset) ? '已添加此项（再次点击取消）' : '点击快速填入'"
        @click="toggleTag(preset)"
      >
        <span class="pill-prefix">{{ modelValue.includes(preset) ? '✓' : '+' }}</span>
        <span class="pill-text">{{ preset }}</span>
      </button>

      <!-- 用户自定义快捷标签 -->
      <div
        v-for="(custom, cIdx) in customList"
        :key="'custom-' + cIdx"
        class="custom-pill-wrapper"
      >
        <button
          type="button"
          class="quick-pill custom"
          :class="[
            tagType,
            { 'is-active': modelValue.includes(custom) }
          ]"
          :title="modelValue.includes(custom) ? '已添加此项（再次点击取消）' : '点击快速填入'"
          @click="toggleTag(custom)"
        >
          <span class="pill-prefix">{{ modelValue.includes(custom) ? '✓' : '+' }}</span>
          <span class="pill-text">{{ custom }}</span>
        </button>
        <button
          type="button"
          class="custom-pill-del"
          title="从快捷填入库中永久删除该自定义项"
          @click.stop="removeCustomPreset(custom)"
        >
          ×
        </button>
      </div>

      <!-- 添加自定义快捷项按钮 -->
      <button
        v-if="allowCustom"
        type="button"
        class="quick-pill add-custom-btn"
        @click="openAddCustomDialog"
        title="添加常用快捷项到快捷库"
      >
        <span class="pill-prefix">+</span> 自定义快捷项
      </button>
    </div>

    <!-- 标签输入框容器 (点击任意空白处自动聚焦输入框) -->
    <div
      class="tag-input-box"
      :class="[
        tagType,
        { 'is-focused': isInputFocused }
      ]"
      @click="focusInput"
    >
      <!-- 已添加的标签卡片 -->
      <TransitionGroup name="tag-zoom">
        <span
          v-for="(tag, idx) in modelValue"
          :key="tag"
          class="tag-badge"
          :class="tagType"
        >
          <span class="tag-icon">{{ tagType === 'danger' ? '🚫' : tagType === 'warning' ? '⭐' : '•' }}</span>
          <span class="tag-text">{{ tag }}</span>
          <span
            class="tag-close-btn"
            title="移除此项"
            @click.stop="removeTag(idx)"
          >
            ×
          </span>
        </span>
      </TransitionGroup>

      <!-- 实时原生输入框：按回车立即转为要求标签，清空文本，不留任何残留残影 -->
      <input
        ref="inputRef"
        v-model="currentText"
        type="text"
        class="tag-native-input"
        :placeholder="modelValue.length === 0 ? placeholder : '输入要求并按回车直接添加...'"
        @focus="isInputFocused = true"
        @blur="handleInputBlur"
        @keydown.enter.prevent="addCurrentTag"
        @keydown.backspace="handleBackspace"
        @paste="handlePaste"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const props = withDefaults(
  defineProps<{
    modelValue?: string[]
    tagType?: 'danger' | 'warning' | 'primary' | 'info'
    placeholder?: string
    quickTitle?: string
    presetList?: string[]
    storageKey?: string
    allowCustom?: boolean
    addPromptTitle?: string
  }>(),
  {
    modelValue: () => [],
    tagType: 'danger',
    placeholder: '输入要求并回车添加',
    quickTitle: '快捷填入:',
    presetList: () => [],
    storageKey: '',
    allowCustom: true,
    addPromptTitle: '新增自定义快捷项'
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: string[]): void
  (e: 'change', value: string[]): void
}>()

const inputRef = ref<HTMLInputElement | null>(null)
const currentText = ref('')
const isInputFocused = ref(false)
const customList = ref<string[]>([])

// 从 localStorage 读取自定义快捷项
function loadCustomList() {
  if (!props.storageKey) return
  try {
    const raw = localStorage.getItem(props.storageKey)
    if (raw) {
      customList.value = JSON.parse(raw)
    }
  } catch (e) {
    customList.value = []
  }
}

// 保存自定义快捷项到 localStorage
function saveCustomList() {
  if (!props.storageKey) return
  try {
    localStorage.setItem(props.storageKey, JSON.stringify(customList.value))
  } catch (e) {}
}

function focusInput() {
  inputRef.value?.focus()
}

function handleInputBlur() {
  isInputFocused.value = false
  // 如果失焦时输入框内有未敲回车的文本，自动帮助用户创建为要求标签
  if (currentText.value.trim()) {
    addCurrentTag()
  }
}

function addCurrentTag() {
  const text = currentText.value.trim()
  if (!text) return

  if (props.modelValue.includes(text)) {
    ElMessage.warning(`「${text}」已在要求列表中`)
    currentText.value = ''
    return
  }

  const next = [...props.modelValue, text]
  emit('update:modelValue', next)
  emit('change', next)
  currentText.value = '' // 核心：瞬间清空文本，绝无残留文字！
  focusInput()
}

function removeTag(idx: number) {
  const next = [...props.modelValue]
  next.splice(idx, 1)
  emit('update:modelValue', next)
  emit('change', next)
}

function handleBackspace(e: KeyboardEvent) {
  if (currentText.value === '' && props.modelValue.length > 0) {
    const next = props.modelValue.slice(0, -1)
    emit('update:modelValue', next)
    emit('change', next)
  }
}

function handlePaste(e: ClipboardEvent) {
  const pasteText = e.clipboardData?.getData('text')
  if (pasteText && (pasteText.includes('\n') || pasteText.includes(',') || pasteText.includes('，') || pasteText.includes('、') || pasteText.includes('；') || pasteText.includes(';'))) {
    e.preventDefault()
    const items = pasteText
      .split(/[\n,，、;；]/)
      .map(s => s.trim())
      .filter(Boolean)
    if (items.length > 0) {
      const next = [...props.modelValue]
      for (const item of items) {
        if (!next.includes(item)) next.push(item)
      }
      emit('update:modelValue', next)
      emit('change', next)
      currentText.value = ''
    }
  }
}

function toggleTag(tag: string) {
  const next = [...props.modelValue]
  const idx = next.indexOf(tag)
  if (idx > -1) {
    next.splice(idx, 1)
  } else {
    next.push(tag)
  }
  emit('update:modelValue', next)
  emit('change', next)
}

async function openAddCustomDialog() {
  try {
    const { value } = await ElMessageBox.prompt(
      '请输入常用快捷要求名称（例如：具备海外项目经验、持有PMP证书等）：',
      props.addPromptTitle,
      {
        confirmButtonText: '确定添加',
        cancelButtonText: '取消',
        inputPlaceholder: '例如：统招硕士硬卡 / 5人以上管理经验',
        inputValidator: (val) => {
          if (!val || !val.trim()) return '请输入快捷项内容'
          if (props.presetList.includes(val.trim()) || customList.value.includes(val.trim())) {
            return '该快捷项已存在'
          }
          return true
        }
      }
    )
    if (value && value.trim()) {
      const cleanVal = value.trim()
      customList.value.push(cleanVal)
      saveCustomList()

      // 自动把新加的项也应用到当前要求中
      if (!props.modelValue.includes(cleanVal)) {
        const next = [...props.modelValue, cleanVal]
        emit('update:modelValue', next)
        emit('change', next)
      }
      ElMessage.success(`已添加并保存自定义快捷项「${cleanVal}」！`)
    }
  } catch {
    // 用户点击取消
  }
}

function removeCustomPreset(custom: string) {
  const idx = customList.value.indexOf(custom)
  if (idx > -1) {
    customList.value.splice(idx, 1)
    saveCustomList()
    ElMessage.info(`已移除快捷项「${custom}」`)
  }
}

onMounted(() => {
  loadCustomList()
})
</script>

<style scoped lang="scss">
.tag-input-wrapper {
  width: 100%;
}

.quick-tags-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 8px;

  .quick-title {
    font-size: 11.5px;
    font-weight: 500;
    color: #64748b;
  }

  .quick-pill {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    border: 1px solid transparent;
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 11.5px;
    font-weight: 500;
    cursor: pointer;
    background: #f1f5f9;
    color: #475569;
    transition: all 0.15s cubic-bezier(0.4, 0, 0.2, 1);
    user-select: none;

    .pill-prefix {
      font-weight: 700;
      font-size: 12px;
    }

    &.danger {
      background: #fef2f2;
      border-color: #fee2e2;
      color: #b91c1c;

      &:hover {
        background: #fee2e2;
        border-color: #fca5a5;
      }

      &.is-active {
        background: #dc2626;
        border-color: #b91c1c;
        color: #ffffff;
        box-shadow: 0 2px 6px rgba(220, 38, 38, 0.25);
      }
    }

    &.warning {
      background: #fffbeb;
      border-color: #fef3c7;
      color: #b45309;

      &:hover {
        background: #fef3c7;
        border-color: #fde68a;
      }

      &.is-active {
        background: #d97706;
        border-color: #b45309;
        color: #ffffff;
        box-shadow: 0 2px 6px rgba(217, 119, 6, 0.25);
      }
    }

    &.add-custom-btn {
      border: 1px dashed #cbd5e1;
      background: #ffffff;
      color: #64748b;

      &:hover {
        border-color: #3b82f6;
        color: #2563eb;
        background: #eff6ff;
      }
    }
  }

  .custom-pill-wrapper {
    display: inline-flex;
    align-items: center;
    position: relative;

    .quick-pill.custom {
      padding-right: 18px;
    }

    .custom-pill-del {
      position: absolute;
      right: 4px;
      top: 50%;
      transform: translateY(-50%);
      width: 14px;
      height: 14px;
      border-radius: 50%;
      background: rgba(0, 0, 0, 0.12);
      border: none;
      color: inherit;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 11px;
      line-height: 1;
      cursor: pointer;
      opacity: 0.6;
      transition: all 0.15s;

      &:hover {
        opacity: 1;
        background: rgba(0, 0, 0, 0.25);
      }
    }
  }
}

// 标签容器
.tag-input-box {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  min-height: 40px;
  background: #ffffff;
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  cursor: text;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);

  &:hover {
    border-color: #c0c4cc;
  }

  &.is-focused {
    border-color: #3b82f6;
    box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.15);
  }

  &.danger.is-focused {
    border-color: #ef4444;
    box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.15);
  }

  &.warning.is-focused {
    border-color: #f59e0b;
    box-shadow: 0 0 0 2px rgba(245, 158, 11, 0.15);
  }

  .tag-badge {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 8px;
    border-radius: 5px;
    font-size: 12px;
    font-weight: 500;
    line-height: 1.4;
    user-select: none;
    transition: all 0.15s;

    .tag-icon {
      font-size: 11px;
    }

    .tag-text {
      max-width: 280px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .tag-close-btn {
      width: 14px;
      height: 14px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      border-radius: 50%;
      cursor: pointer;
      font-size: 13px;
      font-weight: 700;
      opacity: 0.6;
      transition: all 0.15s;

      &:hover {
        opacity: 1;
        background: rgba(0, 0, 0, 0.1);
      }
    }

    &.danger {
      background: #fee2e2;
      border: 1px solid #fca5a5;
      color: #b91c1c;

      .tag-close-btn:hover {
        background: #ef4444;
        color: #ffffff;
      }
    }

    &.warning {
      background: #fef3c7;
      border: 1px solid #fde68a;
      color: #92400e;

      .tag-close-btn:hover {
        background: #f59e0b;
        color: #ffffff;
      }
    }

    &.primary {
      background: #eff6ff;
      border: 1px solid #bfdbfe;
      color: #1d4ed8;

      .tag-close-btn:hover {
        background: #3b82f6;
        color: #ffffff;
      }
    }
  }

  .tag-native-input {
    border: none;
    outline: none;
    background: transparent;
    flex: 1;
    min-width: 150px;
    font-size: 13px;
    color: #1e293b;
    padding: 2px 4px;
    height: 26px;

    &::placeholder {
      color: #94a3b8;
      font-size: 12.5px;
    }
  }
}

// 标签动画
.tag-zoom-enter-active,
.tag-zoom-leave-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
.tag-zoom-enter-from {
  opacity: 0;
  transform: scale(0.85);
}
.tag-zoom-leave-to {
  opacity: 0;
  transform: scale(0.85);
}
</style>