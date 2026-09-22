<template>
  <el-dialog
    v-model="visible"
    title="📧 发送正式录用通知书 (Offer Letter)"
    width="720px"
    :close-on-click-modal="false"
    append-to-body
    class="offer-dialog"
  >
    <div class="offer-dialog-content">
      <!-- 提示条 -->
      <div class="tip-bar">
        <el-icon class="tip-icon"><InfoFilled /></el-icon>
        <div class="tip-text">
          系统将通过网易企业通道 <strong>{{ form.senderEmail }}</strong> 向候选人 <strong>{{ form.recipientEmail }}</strong> 发送正式排版的 HTML 录用通知书。
        </div>
      </div>

      <el-tabs v-model="activeTab" class="offer-tabs">
        <!-- 标签页 1: 录用信息编辑 -->
        <el-tab-pane label="📝 录用核心条款" name="edit">
          <div class="form-grid">
            <div class="form-item">
              <label class="form-label">候选人姓名</label>
              <el-input v-model="form.candidateName" placeholder="候选人姓名" />
            </div>

            <div class="form-item">
              <label class="form-label">候选人接收邮箱</label>
              <el-input v-model="form.recipientEmail" placeholder="候选人接收邮箱">
                <template #prefix><el-icon><Message /></el-icon></template>
              </el-input>
            </div>

            <div class="form-item">
              <label class="form-label">聘用单位名称</label>
              <el-input v-model="form.companyName" placeholder="聘用单位名称" />
            </div>

            <div class="form-item">
              <label class="form-label">录用职位</label>
              <el-input v-model="form.jobTitle" placeholder="录用职位" />
            </div>

            <div class="form-item span-2">
              <label class="form-label">薪酬福利待遇 (Package)</label>
              <el-input v-model="form.salaryPackage" placeholder="如：20,000 - 25,000 元/月 (14薪) + 绩效奖金 + 五险一金" />
            </div>

            <div class="form-item">
              <label class="form-label">期望报到入职日期</label>
              <el-input v-model="form.reportDate" placeholder="如：2026年10月15日" />
            </div>

            <div class="form-item">
              <label class="form-label">工作地点</label>
              <el-input v-model="form.workLocation" placeholder="工作地点" />
            </div>

            <div class="form-item span-2">
              <label class="form-label">HR 专属补充说明 / 备注</label>
              <el-input
                v-model="form.customNotes"
                type="textarea"
                :rows="2"
                placeholder="如：请于入职首日携带体检报告及证件原件，报到时间为上午 09:30。"
              />
            </div>
          </div>

          <!-- 发件邮箱与 SMTP 配置抽屉 -->
          <div class="smtp-config-box">
            <div class="smtp-header" @click="showSmtpConfig = !showSmtpConfig">
              <span>⚙️ 官方发件人邮箱与鉴权配置</span>
              <span class="toggle-btn">{{ showSmtpConfig ? '收起 ▲' : '展开修改 ▼' }}</span>
            </div>
            <div v-show="showSmtpConfig" class="smtp-body">
              <div class="form-grid">
                <div class="form-item">
                  <label class="form-label">发件人 163 邮箱</label>
                  <el-input v-model="form.senderEmail" placeholder="15194921527@163.com" />
                </div>
                <div class="form-item">
                  <label class="form-label">客户端授权密码 (非登录密码)</label>
                  <el-input
                    v-model="form.senderPassword"
                    type="password"
                    show-password
                    placeholder="163 邮箱授权密码"
                  />
                </div>
              </div>
              <div class="smtp-note">
                ℹ️ 说明：网易 163 邮箱连接第三方系统时，要求在【mail.163.com -> 设置 -> POP3/SMTP/IMAP】中开启服务并使用「客户端授权密码」。
              </div>
            </div>
          </div>
        </el-tab-pane>

        <!-- 标签页 2: 邮件实时效果预览 -->
        <el-tab-pane label="👁️ 邮件效果预览" name="preview">
          <div class="email-preview-wrap">
            <div class="email-header-banner">
              <h2>🎉 录 用 通 知 书 (OFFER LETTER)</h2>
              <p>{{ form.companyName }} · 人力资源与人才发展中心</p>
            </div>
            <div class="email-body">
              <p><strong>尊敬的 {{ form.candidateName }}：</strong></p>
              <p>
                非常高兴地通知您，经过我司严格而细致的简历评审与专业面试评估，您的专业能力与职业素养高度契合我司的发展规划。现正式向您发出录用邀请！
              </p>

              <table class="preview-table">
                <tr><td>🎯 录用岗位</td><td><strong>{{ form.jobTitle }}</strong></td></tr>
                <tr><td>🏢 聘用单位</td><td>{{ form.companyName }}</td></tr>
                <tr><td>💰 薪酬待遇</td><td class="highlight-green"><strong>{{ form.salaryPackage }}</strong></td></tr>
                <tr><td>📅 报到日期</td><td class="highlight-blue"><strong>{{ form.reportDate }}</strong></td></tr>
                <tr><td>📍 工作地点</td><td>{{ form.workLocation }}</td></tr>
              </table>

              <div class="preview-box checklist">
                <h4>📋 入职报到准备材料清单：</h4>
                <ul>
                  <li>身份证原件及正反面复印件（2份）；</li>
                  <li>最高学历学位证书原件及学信网在线验证报告；</li>
                  <li>原用人单位开具的正式解除劳动关系证明（离职证明原件）；</li>
                  <li>近期二寸蓝底免冠证件照片（2张）；</li>
                  <li>近3个月内三甲医院体检合格报告一份。</li>
                </ul>
              </div>

              <div v-if="form.customNotes" class="preview-box notes">
                <strong>💡 HR 补充说明：</strong> {{ form.customNotes }}
              </div>

              <p style="margin-top: 18px;">
                收到本通知后，请于 <strong>3个工作日内</strong> 直接回复本邮件予以确认。期待与您携手同行，共创未来！
              </p>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <!-- 结果反馈区 -->
      <div v-if="resultMessage" class="result-alert" :class="isSuccess ? 'success' : 'error'">
        <el-icon><CircleCheck v-if="isSuccess" /><WarningFilled v-else /></el-icon>
        <div class="result-text">{{ resultMessage }}</div>
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="visible = false">关闭</el-button>
        <el-button
          type="primary"
          :loading="sending"
          @click="handleSendOffer"
          class="send-btn"
        >
          <el-icon><Promotion /></el-icon> 🚀 一键发送正式录用 Offer
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { InfoFilled, Message, CircleCheck, WarningFilled, Promotion } from '@element-plus/icons-vue'

const props = defineProps<{
  modelValue: boolean
  candidateName?: string
  candidateEmail?: string
  jobTitle?: string
  companyName?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'success'): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const activeTab = ref('edit')
const showSmtpConfig = ref(false)
const sending = ref(false)
const isSuccess = ref(false)
const resultMessage = ref('')

// 计算默认报到日期为 14 天后
function getDefaultReportDate(): string {
  const d = new Date()
  d.setDate(d.getDate() + 14)
  return `${d.getFullYear()}年${String(d.getMonth() + 1).padStart(2, '0')}月${String(d.getDate()).padStart(2, '0')}日`
}

const form = reactive({
  recipientEmail: 'qn3366271573@163.com',
  candidateName: '陈思远',
  jobTitle: '临床项目经理',
  companyName: '上海泰尔生物医药科技有限公司',
  salaryPackage: '20,000 - 25,000 元/月 (14薪) + 绩效奖金 + 五险一金',
  reportDate: getDefaultReportDate(),
  workLocation: '上海市张江高科技园区 / 核心研发中心',
  customNotes: '请于入职首日携带相关证件原件及离职证明，报到时间为上午 09:30。',
  senderEmail: '15194921527@163.com',
  senderPassword: '2247633190Zz.'
})

watch(() => props.modelValue, (val) => {
  if (val) {
    if (props.candidateName) form.candidateName = props.candidateName
    if (props.candidateEmail) form.recipientEmail = props.candidateEmail
    if (props.jobTitle) form.jobTitle = props.jobTitle
    if (props.companyName) form.companyName = props.companyName
    resultMessage.value = ''
    isSuccess.value = false
    activeTab.value = 'edit'
  }
})

async function handleSendOffer() {
  if (!form.recipientEmail || !form.recipientEmail.includes('@')) {
    ElMessage.warning('请输入合法的候选人接收邮箱！')
    return
  }

  sending.value = true
  resultMessage.value = ''
  isSuccess.value = false

  let WailsApp: any = null
  try {
    WailsApp = await import('../../wailsjs/go/main/App')
  } catch {}

  if (WailsApp && WailsApp.SendOfferEmail) {
    try {
      const res = await WailsApp.SendOfferEmail(
        form.recipientEmail.trim(),
        form.candidateName.trim(),
        form.jobTitle.trim(),
        form.companyName.trim(),
        form.salaryPackage.trim(),
        form.reportDate.trim(),
        form.customNotes.trim(),
        form.senderEmail.trim(),
        form.senderPassword.trim()
      )

      if (res && res.success) {
        isSuccess.value = true
        resultMessage.value = res.message || `🎉 录用通知书已成功送达 ${form.recipientEmail}！`
        ElMessage.success('🎉 录用 Offer 发送成功！')
        emit('success')
      } else {
        isSuccess.value = false
        resultMessage.value = res.error || '发送失败，请检查 SMTP 配置'
        ElMessage.error('Offer 发送未完成')
      }
    } catch (err: any) {
      isSuccess.value = false
      resultMessage.value = `执行异常: ${err.message || err}`
      ElMessage.error('发送出现异常')
    } finally {
      sending.value = false
    }
  } else {
    // 预览环境回退
    setTimeout(() => {
      sending.value = false
      isSuccess.value = true
      resultMessage.value = `[开发模拟] 录用 Offer 已模拟投递至 ${form.recipientEmail}`
      ElMessage.success('模拟发送成功！')
    }, 1200)
  }
}
</script>

<style scoped>
.offer-dialog-content {
  padding: 4px 8px;
}

.tip-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: #e6f7ff;
  border: 1px solid #91d5ff;
  border-radius: 6px;
  padding: 10px 14px;
  margin-bottom: 16px;
  font-size: 13px;
  color: #0050b3;
}

.tip-icon {
  font-size: 16px;
  color: #1890ff;
  flex-shrink: 0;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 16px;
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-item.span-2 {
  grid-column: span 2;
}

.form-label {
  font-size: 13px;
  font-weight: 500;
  color: #4a5568;
}

.smtp-config-box {
  margin-top: 18px;
  background: #f8fafc;
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  padding: 12px 14px;
}

.smtp-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: #475569;
}

.toggle-btn {
  font-size: 12px;
  color: #3b82f6;
}

.smtp-body {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #e2e8f0;
}

.smtp-note {
  margin-top: 8px;
  font-size: 12px;
  color: #64748b;
  line-height: 1.5;
}

/* 邮件预览样式 */
.email-preview-wrap {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  overflow: hidden;
  background: #ffffff;
}

.email-header-banner {
  background: linear-gradient(135deg, #2563eb 0%, #06b6d4 100%);
  padding: 20px;
  text-align: center;
  color: #ffffff;
}

.email-header-banner h2 {
  margin: 0;
  font-size: 18px;
  letter-spacing: 1px;
}

.email-header-banner p {
  margin: 6px 0 0;
  font-size: 12px;
  opacity: 0.9;
}

.email-body {
  padding: 20px;
  font-size: 14px;
  line-height: 1.7;
  color: #334155;
}

.preview-table {
  width: 100%;
  border-collapse: collapse;
  margin: 14px 0;
  background: #f8fafc;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
}

.preview-table td {
  padding: 8px 12px;
  border-bottom: 1px solid #f1f5f9;
  font-size: 13px;
}

.preview-table td:first-child {
  width: 100px;
  color: #64748b;
}

.highlight-green {
  color: #16a34a;
}

.highlight-blue {
  color: #2563eb;
}

.preview-box {
  padding: 12px 14px;
  border-radius: 6px;
  margin: 12px 0;
  font-size: 13px;
}

.preview-box.checklist {
  background: #f0f9ff;
  border-left: 3px solid #0284c7;
}

.preview-box.checklist h4 {
  margin: 0 0 6px;
  color: #0369a1;
}

.preview-box.checklist ul {
  margin: 0;
  padding-left: 18px;
  line-height: 1.6;
}

.preview-box.notes {
  background: #fffbeb;
  border-left: 3px solid #d97706;
  color: #92400e;
}

/* 反馈提示条 */
.result-alert {
  margin-top: 16px;
  padding: 12px 14px;
  border-radius: 6px;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.result-alert.success {
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  color: #065f46;
}

.result-alert.error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #991b1b;
}

.send-btn {
  background: linear-gradient(135deg, #2563eb 0%, #0d9488 100%);
  border: none;
  font-weight: 600;
}
</style>
