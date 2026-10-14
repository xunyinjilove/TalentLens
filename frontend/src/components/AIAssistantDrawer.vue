<template>
  <div>
    <!-- 右下角常驻悬浮唤醒按钮 -->
    <div
      v-if="!assistantStore.isOpen"
      class="ai-float-trigger"
      @click="assistantStore.openDrawer"
      title="唤起透景 AI 招聘助理"
    >
      <div class="ai-trigger-pulse"></div>
      <div class="ai-trigger-icon-wrap">
        <el-icon class="ai-icon"><ChatDotRound /></el-icon>
      </div>
      <span class="ai-trigger-text">AI 助手</span>
    </div>

    <!-- 侧边滑出抽屉面板 -->
    <el-drawer
      v-model="assistantStore.isOpen"
      direction="rtl"
      size="540px"
      :with-header="false"
      class="talentlens-ai-drawer"
      destroy-on-close="false"
    >
      <div class="drawer-container">
        <!-- 抽屉顶部头部 -->
        <header class="drawer-header">
          <div class="header-left">
            <div class="assistant-avatar">
              <el-icon><Cpu /></el-icon>
            </div>
            <div class="header-titles">
              <div class="title-row">
                <h3 class="main-title">透景 AI 招聘助理</h3>
                <span class="badge-copilot">Copilot</span>
              </div>
              <p class="sub-title">HR 专家顾问 · 系统功能向导</p>
            </div>
          </div>

          <div class="header-actions">
            <el-button
              link
              class="header-btn"
              title="清空聊天记录"
              :disabled="assistantStore.messages.length === 0"
              @click="handleClearChat"
            >
              <el-icon><Delete /></el-icon>
            </el-button>
            <el-button
              link
              class="header-btn"
              title="收起助理面板"
              @click="assistantStore.closeDrawer"
            >
              <el-icon><Close /></el-icon>
            </el-button>
          </div>
        </header>

        <!-- 聊天消息展示区域 -->
        <main class="chat-viewport" ref="chatViewportRef">
          <!-- 空状态欢迎指引 -->
          <div v-if="assistantStore.messages.length === 0" class="welcome-guide">
            <div class="hero-card">
              <div class="hero-icon">
                <el-icon><ChatDotRound /></el-icon>
              </div>
              <h4>您好！我是透景 AI 招聘助理</h4>
              <p class="hero-desc">
                我可以帮您梳理职位需求并起草标准化 JD、提炼红线过滤标准，也能为您解答软件各模块的使用方法。
              </p>
            </div>

            <div class="prompt-section">
              <div class="section-title">
                <span class="title-tag">💡 点击快速提问</span>
              </div>
              <div class="prompt-chips">
                <button
                  v-for="(prompt, idx) in quickPrompts"
                  :key="idx"
                  class="prompt-chip"
                  @click="handleQuickPrompt(prompt.text)"
                >
                  <span class="chip-icon">{{ prompt.icon }}</span>
                  <span class="chip-text">{{ prompt.text }}</span>
                </button>
              </div>
            </div>
          </div>

          <!-- 消息历史流 -->
          <div v-else class="message-stream">
            <div
              v-for="msg in assistantStore.messages"
              :key="msg.id"
              class="message-row"
              :class="{ 'is-user': msg.role === 'user', 'is-assistant': msg.role === 'assistant' }"
            >
              <!-- 助手头像 -->
              <div v-if="msg.role === 'assistant'" class="msg-avatar assistant">
                <el-icon><Cpu /></el-icon>
              </div>

              <!-- 消息气泡正文 -->
              <div class="msg-bubble" :class="{ 'has-error': msg.error }">
                <div class="msg-meta">
                  <span class="sender-name">{{ msg.role === 'user' ? '我' : '透景 AI 助手' }}</span>
                  <span class="msg-time">{{ formatTime(msg.timestamp) }}</span>
                </div>

                <!-- 格式化渲染的内容（文本 + 职位卡片 + 快捷操作按钮） -->
                <div class="msg-content">
                  <!-- 解析后的文本部分 -->
                  <div class="rendered-markdown" v-html="renderMessageText(msg.content)"></div>

                  <button v-if="msg.error" class="error-settings-btn" type="button" @click="router.push('/settings')">
                    前往 AI 设置
                  </button>

                  <!-- 如果消息中包含 talentlens-job 结构化数据，渲染交互卡片 -->
                  <div
                    v-if="extractJobData(msg.content)"
                    class="job-blueprint-card"
                  >
                    <div class="blueprint-header">
                      <div class="header-tag">📋 岗位立项蓝图</div>
                      <h4 class="job-name">{{ extractJobData(msg.content)?.title || '未命名岗位' }}</h4>
                    </div>

                    <div class="job-meta-grid">
                      <div class="meta-item">
                        <span class="label">建议部门:</span>
                        <span class="val">{{ extractJobData(msg.content)?.department || '默认部门' }}</span>
                      </div>
                      <div class="meta-item">
                        <span class="label">经验学历:</span>
                        <span class="val">
                          {{ extractJobData(msg.content)?.experience_years || 1 }}年 ·
                          {{ extractJobData(msg.content)?.education_level || '学历不限' }}
                        </span>
                      </div>
                      <div class="meta-item">
                        <span class="label">编制人数:</span>
                        <span class="val">{{ extractJobData(msg.content)?.headcount || 1 }} 人</span>
                      </div>
                    </div>

                    <!-- 技能标签 -->
                    <div
                      v-if="extractJobData(msg.content)?.required_skills?.length"
                      class="blueprint-tags"
                    >
                      <span class="tag-title">必备技能:</span>
                      <div class="tags-wrap">
                        <span
                          v-for="skill in extractJobData(msg.content)!.required_skills"
                          :key="skill"
                          class="skill-tag"
                        >
                          {{ skill }}
                        </span>
                      </div>
                    </div>

                    <!-- 淘汰红线 -->
                    <div
                      v-if="extractJobData(msg.content)?.red_lines?.length"
                      class="blueprint-redlines"
                    >
                      <span class="redline-title">❌ 淘汰红线标准:</span>
                      <ul class="redline-list">
                        <li v-for="(rl, rIdx) in extractJobData(msg.content)!.red_lines" :key="rIdx">
                          {{ rl }}
                        </li>
                      </ul>
                    </div>

                    <!-- 加分项 -->
                    <div
                      v-if="extractJobData(msg.content)?.bonus_points?.length"
                      class="blueprint-bonus"
                    >
                      <span class="bonus-title">⭐ 加分项:</span>
                      <ul class="bonus-list">
                        <li v-for="(bp, bIdx) in extractJobData(msg.content)!.bonus_points" :key="bIdx">
                          {{ bp }}
                        </li>
                      </ul>
                    </div>

                    <!-- 卡片动作栏 -->
                    <div class="blueprint-actions">
                      <el-button
                        type="primary"
                        size="small"
                        class="apply-btn"
                        @click="handleApplyJob(extractJobData(msg.content)!)"
                      >
                        <el-icon><Check /></el-icon>
                        一键填入新建项目
                      </el-button>

                      <el-button
                        size="small"
                        class="copy-btn"
                        @click="handleCopyJD(extractJobData(msg.content)!)"
                      >
                        <el-icon><DocumentCopy /></el-icon>
                        复制完整JD
                      </el-button>
                    </div>
                  </div>

                  <!-- 如果消息中包含 [ACTION:...] 动作标签，渲染快捷跳转药丸按钮 -->
                  <div
                    v-if="extractActions(msg.content).length > 0"
                    class="action-buttons-wrap"
                  >
                    <button
                      v-for="(act, aIdx) in extractActions(msg.content)"
                      :key="aIdx"
                      class="action-pill-btn"
                      @click="handleExecuteAction(act)"
                    >
                      <span class="pill-icon">🚀</span>
                      <span class="pill-label">{{ act.label }}</span>
                    </button>
                  </div>
                </div>
              </div>

              <!-- 用户头像 -->
              <div v-if="msg.role === 'user'" class="msg-avatar user">
                <el-icon><User /></el-icon>
              </div>
            </div>

            <!-- 加载态（思考中） -->
            <div v-if="assistantStore.loading" class="message-row is-assistant">
              <div class="msg-avatar assistant">
                <el-icon><Cpu /></el-icon>
              </div>
              <div class="msg-bubble thinking-bubble">
                <div class="thinking-dots">
                  <span></span>
                  <span></span>
                  <span></span>
                </div>
                <span class="thinking-text">AI 正在深度分析并制定招聘建议...</span>
              </div>
            </div>
          </div>
        </main>

        <!-- 底部输入栏 -->
        <footer class="drawer-footer">
          <div class="input-card">
            <el-input
              v-model="inputContent"
              type="textarea"
              :rows="2"
              :autosize="{ minRows: 2, maxRows: 6 }"
              placeholder="请输入您的问题或招聘需求（Enter 发送，Shift+Enter 换行）..."
              resize="none"
              class="chat-textarea"
              :disabled="assistantStore.loading"
              @keydown.enter="handleEnterKey"
            />

            <div class="input-toolbar">
              <span class="input-hint">💡 支持多轮需求追问与软件功能直达</span>

              <el-button
                type="primary"
                class="send-btn"
                :loading="assistantStore.loading"
                :disabled="!inputContent.trim()"
                @click="handleSend"
              >
                <el-icon><Promotion /></el-icon>
                <span>发送</span>
              </el-button>
            </div>
          </div>
        </footer>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ChatDotRound,
  Cpu,
  Delete,
  Close,
  User,
  Promotion,
  Check,
  DocumentCopy
} from '@element-plus/icons-vue'
import { useAIAssistantStore, type JobApplyData } from '../composables/useAIAssistantStore'

const router = useRouter()
const assistantStore = useAIAssistantStore()

const inputContent = ref('')
const chatViewportRef = ref<HTMLElement | null>(null)

// 快捷提示卡片
const quickPrompts = [
  { icon: '📝', text: '帮我起草一个临床试验项目经理(CPM)的岗位JD与淘汰红线' },
  { icon: '💻', text: '生成一份高级全栈工程师的招聘要求与必备技能' },
  { icon: '🌐', text: '怎么在软件里把职位发布到 51job 并向候选人打招呼？' },
  { icon: '📄', text: '如何在候选人档案中设置自动淘汰红线规则？' },
  { icon: '⚙️', text: '如何配置大模型 API Key (DeepSeek / OpenAI / Ollama)？' },
  { icon: '✉️', text: '怎么向通过终审的候选人发送正式 Offer 通知？' }
]

// 自动滚动至最新消息
async function scrollToBottom() {
  await nextTick()
  if (chatViewportRef.value) {
    chatViewportRef.value.scrollTop = chatViewportRef.value.scrollHeight
  }
}

watch(
  () => assistantStore.messages.length,
  () => scrollToBottom()
)

watch(
  () => assistantStore.loading,
  () => scrollToBottom()
)

// 发送消息
async function handleSend() {
  const content = inputContent.value.trim()
  if (!content || assistantStore.loading) return

  inputContent.value = ''
  scrollToBottom()

  // 附带当前页面上下文
  const currentPath = router.currentRoute.value.path
  const contextDesc = `当前用户所在页面: ${currentPath}`

  await assistantStore.sendMessage(content, contextDesc)
  if (assistantStore.messages.at(-1)?.error) inputContent.value = content
  scrollToBottom()
}

// 键盘快捷键监听
function handleEnterKey(e: KeyboardEvent) {
  if (e.shiftKey) {
    // 允许 Shift+Enter 换行
    return
  }
  e.preventDefault()
  handleSend()
}

// 点击预设快捷提示
function handleQuickPrompt(text: string) {
  inputContent.value = text
  handleSend()
}

// 清空对话
function handleClearChat() {
  ElMessageBox.confirm('确定要清空当前的 AI 对话记录吗？', '清空确认', {
    confirmButtonText: '确定清空',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    assistantStore.clearMessages()
    ElMessage.success('已清空对话记录')
  }).catch(() => {})
}

// 时间戳格式化
function formatTime(timestamp: number): string {
  const date = new Date(timestamp)
  return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`
}

// 提取职位 JSON 数据
function extractJobData(content: string): JobApplyData | null {
  const regex = /```json:talentlens-job\s*([\s\S]*?)\s*```/
  const match = content.match(regex)
  if (!match) return null

  try {
    return JSON.parse(match[1])
  } catch {
    return null
  }
}

interface ActionCommand {
  type: 'NAVIGATE' | 'OPEN_CREATE_PROJECT' | 'OPEN_DATA_DIR'
  path?: string
  label: string
}

// 提取系统动作标签 [ACTION:...]
function extractActions(content: string): ActionCommand[] {
  const actions: ActionCommand[] = []
  const regex = /\[ACTION:(NAVIGATE|OPEN_CREATE_PROJECT|OPEN_DATA_DIR)(?:\|([^|\]]+))?\|([^\]]+)\]/g
  let match: RegExpExecArray | null

  while ((match = regex.exec(content)) !== null) {
    const actType = match[1] as ActionCommand['type']
    if (actType === 'NAVIGATE') {
      actions.push({
        type: 'NAVIGATE',
        path: match[2],
        label: match[3]
      })
    } else if (actType === 'OPEN_CREATE_PROJECT') {
      actions.push({
        type: 'OPEN_CREATE_PROJECT',
        label: match[3] || match[2] || '新建项目'
      })
    } else if (actType === 'OPEN_DATA_DIR') {
      actions.push({
        type: 'OPEN_DATA_DIR',
        label: match[3] || match[2] || '打开数据目录'
      })
    }
  }

  return actions
}

// 过滤掉内部代码块与动作标记，进行轻量级 Markdown 渲染
function renderMessageText(raw: string): string {
  // 1. 移除 json:talentlens-job 代码块，避免重复显示丑陋的 JSON
  let cleaned = raw.replace(/```json:talentlens-job[\s\S]*?```/g, '')

  // 2. 移除 [ACTION:...] 标记，因为已有专用药丸按钮展示
  cleaned = cleaned.replace(/\[ACTION:[^\]]+\]/g, '')

  // 3. 处理转义与简单 Markdown 格式化
  const escaped = cleaned
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  // 标题
  let html = escaped
    .replace(/^### (.*$)/gim, '<h3 class="md-h3">$1</h3>')
    .replace(/^#### (.*$)/gim, '<h4 class="md-h4">$1</h4>')
    // 粗体
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    // 行内代码
    .replace(/`([^`]+)`/g, '<code class="md-code">$1</code>')
    // 无序列表
    .replace(/^\s*[-*]\s+(.*$)/gim, '<li class="md-li">$1</li>')
    // 有序列表
    .replace(/^\s*(\d+)\.\s+(.*$)/gim, '<li class="md-oli"><span class="oli-num">$1.</span> $2</li>')
    // 换行
    .replace(/\n\n/g, '<p class="md-p"></p>')
    .replace(/\n/g, '<br/>')

  return html
}

// 执行动作指令
async function handleExecuteAction(action: ActionCommand) {
  if (action.type === 'NAVIGATE' && action.path) {
    router.push(action.path)
    ElMessage.success(`已前往：${action.label}`)
  } else if (action.type === 'OPEN_CREATE_PROJECT') {
    if (router.currentRoute.value.path !== '/') {
      router.push('/')
    }
    // 派发创建项目事件
    window.dispatchEvent(new CustomEvent('talentlens:trigger-create-project'))
    ElMessage.success('已为您打开新建项目对话框')
  } else if (action.type === 'OPEN_DATA_DIR') {
    try {
      const WailsApp = await import('../../wailsjs/go/main/App')
      if (WailsApp && typeof WailsApp.OpenDataDir === 'function') {
        await WailsApp.OpenDataDir()
        ElMessage.success('已在文件管理器中打开本地数据目录')
      }
    } catch {
      ElMessage.info('当前环境无法直接唤起资源管理器')
    }
  }
}

// 一键填入新建项目
function handleApplyJob(jobData: JobApplyData) {
  assistantStore.applyJobToProject(jobData)
  ElMessage.success(`已自动填充「${jobData.title}」的岗位需求并准备立项！`)
}

// 复制岗位 JD
async function handleCopyJD(jobData: JobApplyData) {
  const fullJD = `${jobData.title} 招聘要求\n\n` +
    `所属部门: ${jobData.department || '未定'}\n` +
    `工作经验: ${jobData.experience_years || 1}年 | 学历要求: ${jobData.education_level || '本科'}\n\n` +
    `【岗位描述】:\n${jobData.job_description || ''}\n\n` +
    `【必备技能】:\n${(jobData.required_skills || []).join('、')}\n\n` +
    `【淘汰红线】:\n${(jobData.red_lines || []).join('\n')}\n\n` +
    `【加分项】:\n${(jobData.bonus_points || []).join('\n')}`

  try {
    await navigator.clipboard.writeText(fullJD)
    ElMessage.success('已成功复制完整岗位 JD 到剪贴板！')
  } catch {
    ElMessage.warning('复制失败，请手动选取复制')
  }
}
</script>

<style scoped lang="scss">
/* 常驻右下角悬浮按钮 */
.ai-float-trigger {
  position: fixed;
  right: 28px;
  bottom: 28px;
  height: 44px;
  padding: 0 16px 0 12px;
  border-radius: 22px;
  background: linear-gradient(135deg, #2563EB 0%, #1D4ED8 100%);
  box-shadow: 0 6px 18px rgba(37, 99, 235, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.15) inset;
  color: #FFFFFF;
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  z-index: 990;
  user-select: none;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);

  &:hover {
    transform: translateY(-2px) scale(1.03);
    box-shadow: 0 8px 24px rgba(37, 99, 235, 0.55), 0 0 0 1px rgba(255, 255, 255, 0.2) inset;
  }

  &:active {
    transform: translateY(0) scale(0.98);
  }

  .ai-trigger-icon-wrap {
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.2);
    display: flex;
    align-items: center;
    justify-content: center;

    .ai-icon {
      font-size: 16px;
    }
  }

  .ai-trigger-text {
    font-size: 13px;
    font-weight: 600;
    letter-spacing: 0.5px;
  }
}

/* 抽屉容器 */
.drawer-container {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-app);
  color: var(--text-primary);
  font-size: 14px;
}

/* 抽屉头部 */
.drawer-header {
  height: 64px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-card);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;

    .assistant-avatar {
      width: 38px;
      height: 38px;
      border-radius: 10px;
      background: linear-gradient(135deg, #3B82F6, #1D4ED8);
      color: #FFFFFF;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 20px;
      box-shadow: 0 2px 8px rgba(37, 99, 235, 0.3);
    }

    .header-titles {
      display: flex;
      flex-direction: column;

      .title-row {
        display: flex;
        align-items: center;
        gap: 8px;

        .main-title {
          margin: 0;
          font-size: 16px;
          font-weight: 700;
          color: var(--text-primary);
        }

        .badge-copilot {
          font-size: 10px;
          font-weight: 700;
          padding: 1px 6px;
          border-radius: 4px;
          background: rgba(37, 99, 235, 0.12);
          color: var(--accent-blue);
          text-transform: uppercase;
        }
      }

      .sub-title {
        margin: 2px 0 0;
        font-size: 11px;
        color: var(--sidebar-text-muted);
      }
    }
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 6px;

    .header-btn {
      font-size: 17px;
      color: var(--sidebar-text-muted);
      padding: 6px;

      &:hover {
        color: var(--text-primary);
      }
    }
  }
}

/* 消息视口 */
.chat-viewport {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 欢迎英雄卡片 */
.welcome-guide {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 12px 0;

  .hero-card {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 14px;
    padding: 24px;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.03);

    .hero-icon {
      width: 48px;
      height: 48px;
      border-radius: 50%;
      background: rgba(37, 99, 235, 0.1);
      color: var(--accent-blue);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 24px;
      margin-bottom: 12px;
    }

    h4 {
      margin: 0 0 8px;
      font-size: 16px;
      font-weight: 700;
    }

    .hero-desc {
      margin: 0;
      font-size: 13px;
      line-height: 1.6;
      color: var(--sidebar-text-muted);
      max-width: 420px;
    }
  }

  .prompt-section {
    .section-title {
      margin-bottom: 12px;

      .title-tag {
        font-size: 12px;
        font-weight: 600;
        color: var(--sidebar-text-muted);
      }
    }

    .prompt-chips {
      display: flex;
      flex-direction: column;
      gap: 8px;

      .prompt-chip {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 11px 14px;
        background: var(--bg-card);
        border: 1px solid var(--border-color);
        border-radius: 10px;
        color: var(--text-primary);
        font-size: 13px;
        text-align: left;
        cursor: pointer;
        transition: all 0.2s ease;

        &:hover {
          border-color: var(--accent-blue);
          background: rgba(37, 99, 235, 0.04);
          transform: translateX(4px);
        }

        .chip-icon {
          font-size: 16px;
          flex-shrink: 0;
        }

        .chip-text {
          flex: 1;
          line-height: 1.4;
        }
      }
    }
  }
}

/* 消息流布局 */
.message-stream {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.message-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;

  &.is-user {
    justify-content: flex-end;

    .msg-bubble {
      background: #2563EB;
      color: #FFFFFF;
      border-radius: 14px 14px 2px 14px;

      .msg-meta {
        justify-content: flex-end;
        color: rgba(255, 255, 255, 0.7);
      }

      .rendered-markdown {
        color: #FFFFFF;
      }
    }
  }

  &.is-assistant {
    justify-content: flex-start;

    .msg-bubble {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      color: var(--text-primary);
      border-radius: 14px 14px 14px 2px;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.02);
    }
  }

  .msg-avatar {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 16px;
    flex-shrink: 0;
    margin-top: 2px;

    &.assistant {
      background: rgba(37, 99, 235, 0.1);
      color: var(--accent-blue);
    }

    &.user {
      background: #475569;
      color: #FFFFFF;
    }
  }

  .msg-bubble {
    max-width: 84%;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;

    &.has-error {
      border-color: #EF4444;
      background: rgba(239, 68, 68, 0.05);
    }

    .msg-meta {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 11px;
      color: var(--sidebar-text-muted);

      .sender-name {
        font-weight: 600;
      }
    }
  }
}

.error-settings-btn {
  align-self: flex-start;
  border: 1px solid var(--accent-blue);
  border-radius: 7px;
  background: var(--accent-blue-light);
  color: var(--accent-blue);
  font: inherit;
  font-size: 12px;
  font-weight: 650;
  padding: 6px 10px;
  cursor: pointer;
}

.error-settings-btn:hover { background: var(--accent-blue-subtle); }

/* 消息 Markdown 格式 */
.rendered-markdown {
  font-size: 13.5px;
  line-height: 1.65;
  word-break: break-word;

  :deep(.md-h3) {
    margin: 12px 0 6px;
    font-size: 15px;
    font-weight: 700;
    color: var(--text-primary);
  }

  :deep(.md-h4) {
    margin: 10px 0 4px;
    font-size: 14px;
    font-weight: 700;
    color: var(--text-primary);
  }

  :deep(.md-p) {
    margin: 6px 0;
  }

  :deep(.md-code) {
    font-family: monospace;
    font-size: 12px;
    padding: 2px 6px;
    border-radius: 4px;
    background: rgba(0, 0, 0, 0.06);
    color: var(--accent-blue);
  }

  :deep(.md-li) {
    list-style-type: disc;
    margin-left: 16px;
    margin-bottom: 4px;
  }

  :deep(.md-oli) {
    list-style-type: none;
    margin-left: 0;
    margin-bottom: 4px;

    .oli-num {
      font-weight: 700;
      color: var(--accent-blue);
    }
  }
}

/* 岗位立项蓝图卡片 */
.job-blueprint-card {
  margin-top: 10px;
  padding: 14px;
  background: var(--bg-app);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;

  .blueprint-header {
    display: flex;
    flex-direction: column;
    gap: 4px;

    .header-tag {
      font-size: 11px;
      font-weight: 700;
      color: var(--accent-blue);
      text-transform: uppercase;
    }

    .job-name {
      margin: 0;
      font-size: 16px;
      font-weight: 800;
      color: var(--text-primary);
    }
  }

  .job-meta-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    padding: 8px 10px;
    background: var(--bg-card);
    border-radius: 8px;
    border: 1px solid var(--border-color);

    .meta-item {
      font-size: 12px;

      .label {
        color: var(--sidebar-text-muted);
        margin-right: 4px;
      }

      .val {
        font-weight: 600;
      }
    }
  }

  .blueprint-tags {
    display: flex;
    flex-direction: column;
    gap: 4px;

    .tag-title {
      font-size: 12px;
      font-weight: 600;
      color: var(--sidebar-text-muted);
    }

    .tags-wrap {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;

      .skill-tag {
        font-size: 11px;
        padding: 2px 8px;
        border-radius: 6px;
        background: rgba(37, 99, 235, 0.1);
        color: var(--accent-blue);
        font-weight: 600;
      }
    }
  }

  .blueprint-redlines, .blueprint-bonus {
    font-size: 12px;

    .redline-title {
      color: #EF4444;
      font-weight: 700;
    }

    .bonus-title {
      color: #10B981;
      font-weight: 700;
    }

    ul {
      margin: 4px 0 0 16px;
      padding: 0;

      li {
        line-height: 1.5;
        margin-bottom: 2px;
      }
    }
  }

  .blueprint-actions {
    display: flex;
    gap: 8px;
    margin-top: 4px;

    .apply-btn {
      flex: 1;
      font-weight: 600;
    }

    .copy-btn {
      flex: 1;
    }
  }
}

/* 快捷动作跳转药丸按钮组 */
.action-buttons-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;

  .action-pill-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    background: rgba(37, 99, 235, 0.08);
    border: 1px solid rgba(37, 99, 235, 0.25);
    border-radius: 16px;
    color: var(--accent-blue);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;

    &:hover {
      background: var(--accent-blue);
      color: #FFFFFF;
      transform: translateY(-1px);
    }

    .pill-icon {
      font-size: 12px;
    }
  }
}

/* 思考加载动画 */
.thinking-bubble {
  display: flex;
  align-items: center;
  gap: 10px;

  .thinking-dots {
    display: flex;
    gap: 4px;

    span {
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: var(--accent-blue);
      animation: dotPulse 1.4s infinite ease-in-out both;

      &:nth-child(1) { animation-delay: -0.32s; }
      &:nth-child(2) { animation-delay: -0.16s; }
    }
  }

  .thinking-text {
    font-size: 12px;
    color: var(--sidebar-text-muted);
  }
}

@keyframes dotPulse {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}

/* 底部输入卡片 */
.drawer-footer {
  padding: 16px 20px;
  background: var(--bg-card);
  border-top: 1px solid var(--border-color);
  flex-shrink: 0;

  .input-card {
    border: 1px solid var(--border-color);
    border-radius: 12px;
    background: var(--bg-app);
    padding: 8px 12px 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    transition: border-color 0.2s;

    &:focus-within {
      border-color: var(--accent-blue);
      box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.12);
    }

    :deep(.el-textarea__inner) {
      background: transparent !important;
      border: none !important;
      box-shadow: none !important;
      padding: 0 !important;
      font-size: 13.5px;
      color: var(--text-primary);
    }

    .input-toolbar {
      display: flex;
      align-items: center;
      justify-content: space-between;

      .input-hint {
        font-size: 11px;
        color: var(--sidebar-text-muted);
      }

      .send-btn {
        border-radius: 8px;
        font-weight: 600;
        padding: 6px 14px;
      }
    }
  }
}
</style>
