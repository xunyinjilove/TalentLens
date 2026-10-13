<template>
  <div class="window-size-menu-wrapper" data-wails-no-drag>
    <el-dropdown trigger="click" @command="handleCommand">
      <button class="size-menu-btn" title="调整窗口大小与分辨率" type="button">
        <svg viewBox="0 0 16 16" fill="currentColor">
          <path d="M1.5 2.5A1.5 1.5 0 0 1 3 1h10a1.5 1.5 0 0 1 1.5 1.5v9a1.5 1.5 0 0 1-1.5 1.5H9.5v1.25H11a.75.75 0 0 1 0 1.5H5a.75.75 0 0 1 0-1.5h1.5V13H3a1.5 1.5 0 0 1-1.5-1.5v-9zm1.5 0v9h10v-9H3z"/>
        </svg>
      </button>
      <template #dropdown>
        <el-dropdown-menu class="window-size-dropdown">
          <div class="menu-header">
            <span>窗口尺寸预设</span>
            <small class="current-dim">{{ currentDimText }}</small>
          </div>
          <el-dropdown-item command="1200x800">
            <span class="dim-title">1200 × 800</span>
            <span class="dim-tag">默认标准</span>
          </el-dropdown-item>
          <el-dropdown-item command="1366x768">
            <span class="dim-title">1366 × 768</span>
            <span class="dim-tag">轻薄本</span>
          </el-dropdown-item>
          <el-dropdown-item command="1440x900">
            <span class="dim-title">1440 × 900</span>
            <span class="dim-tag">舒适宽屏</span>
          </el-dropdown-item>
          <el-dropdown-item command="1600x1000">
            <span class="dim-title">1600 × 1000</span>
            <span class="dim-tag">高清大屏</span>
          </el-dropdown-item>
          <el-dropdown-item command="1920x1080">
            <span class="dim-title">1920 × 1080</span>
            <span class="dim-tag">1080P</span>
          </el-dropdown-item>
          <el-dropdown-item divided command="custom">
            <span>⚙️ 自定义分辨率...</span>
          </el-dropdown-item>
          <el-dropdown-item command="center">
            <span>🎯 窗口居中显示</span>
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>

    <!-- 自定义分辨率弹窗 -->
    <el-dialog
      v-model="customDialogVisible"
      title="自定义窗口分辨率"
      width="380px"
      append-to-body
      :close-on-click-modal="false"
    >
      <div class="custom-dim-form">
        <div class="form-row">
          <label>宽度 (px)：</label>
          <el-input-number
            v-model="customWidth"
            :min="900"
            :max="3840"
            :step="50"
            controls-position="right"
          />
        </div>
        <div class="form-row">
          <label>高度 (px)：</label>
          <el-input-number
            v-model="customHeight"
            :min="600"
            :max="2160"
            :step="50"
            controls-position="right"
          />
        </div>
        <p class="form-tip">提示：最小支持 900 × 600，调整后将自动居中并记忆保存。</p>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="customDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="applyCustomDimensions">应用并保存</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'

const customDialogVisible = ref(false)
const customWidth = ref(1200)
const customHeight = ref(800)
const winW = ref(window.innerWidth)
const winH = ref(window.innerHeight)

const currentDimText = computed(() => `${winW.value} × ${winH.value}`)

function updateCurrentDim() {
  winW.value = window.innerWidth
  winH.value = window.innerHeight
}

onMounted(() => {
  window.addEventListener('resize', updateCurrentDim)
})

onUnmounted(() => {
  window.removeEventListener('resize', updateCurrentDim)
})

async function handleCommand(cmd: string) {
  if (cmd === 'custom') {
    customWidth.value = Math.max(900, window.innerWidth)
    customHeight.value = Math.max(600, window.innerHeight)
    customDialogVisible.value = true
    return
  }

  if (cmd === 'center') {
    try {
      const WailsApp = await import('../../wailsjs/go/main/App')
      if (WailsApp && typeof WailsApp.CenterWindow === 'function') {
        await WailsApp.CenterWindow()
        ElMessage.success('窗口已在当前屏幕居中')
      }
    } catch {}
    return
  }

  const parts = cmd.split('x')
  if (parts.length === 2) {
    const w = parseInt(parts[0], 10)
    const h = parseInt(parts[1], 10)
    await applySize(w, h)
  }
}

async function applySize(w: number, h: number) {
  try {
    const WailsApp = await import('../../wailsjs/go/main/App')
    if (WailsApp && typeof WailsApp.SetWindowSize === 'function') {
      await WailsApp.SetWindowSize(w, h)
      ElMessage.success(`窗口大小已调整为 ${w} × ${h}`)
    } else {
      // 纯网页降级提示
      ElMessage.info(`预览环境已模拟尺寸：${w} × ${h}`)
    }
  } catch (err) {
    console.warn('SetWindowSize error:', err)
  }
}

async function applyCustomDimensions() {
  customDialogVisible.value = false
  await applySize(customWidth.value, customHeight.value)
}
</script>

<style scoped lang="scss">
.window-size-menu-wrapper {
  display: flex;
  align-items: center;
  height: 100%;
}

.size-menu-btn {
  width: 36px;
  height: 100%;
  border: none;
  background: transparent;
  color: var(--text-secondary, #6b7280);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: all 0.15s ease;

  svg {
    width: 14px;
    height: 14px;
  }

  &:hover {
    background: var(--bg-hover, rgba(0, 0, 0, 0.05));
    color: var(--text-primary, #111827);
  }
}

.window-size-dropdown {
  min-width: 200px;

  .menu-header {
    padding: 8px 16px 6px;
    font-size: 11px;
    font-weight: 600;
    color: var(--el-text-color-secondary, #909399);
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--el-border-color-lighter, #ebeef5);
    margin-bottom: 4px;

    .current-dim {
      font-size: 10px;
      color: var(--el-color-primary, #409eff);
      background: var(--el-color-primary-light-9, #ecf5ff);
      padding: 1px 6px;
      border-radius: 4px;
    }
  }

  .el-dropdown-menu__item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 13px;
    padding: 7px 16px;

    .dim-title {
      font-weight: 500;
    }

    .dim-tag {
      font-size: 11px;
      color: var(--el-text-color-placeholder, #a8abb2);
      margin-left: 12px;
    }
  }
}

.custom-dim-form {
  padding: 8px 0;

  .form-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;

    label {
      font-size: 13px;
      color: var(--text-primary, #303133);
      font-weight: 500;
    }
  }

  .form-tip {
    font-size: 12px;
    color: var(--text-muted, #909399);
    margin-top: 10px;
    line-height: 1.5;
  }
}
</style>
