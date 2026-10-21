<template>
  <div class="micro-resume-web-view">
    <!-- 顶部平台来源与真实性标识横幅 -->
    <div class="web-platform-banner" :class="'platform-' + resumeData.platform">
      <div class="banner-left">
        <span class="platform-logo-badge">
          <span v-if="resumeData.platform === '51job'" class="platform-icon job51-icon">51</span>
          <span v-else-if="resumeData.platform === 'boss'" class="platform-icon boss-icon">BOSS</span>
          <el-icon v-else class="platform-icon"><User /></el-icon>
          <strong class="platform-name">{{ resumeData.platformName }}</strong>
        </span>
        <span class="platform-card-type">在线微简历 · 平台真实抽屉</span>
      </div>
      <div class="banner-right">
        <span v-if="resumeData.activityLabel" class="activity-pulse-tag">
          <span class="pulse-dot"></span>
          {{ resumeData.activityLabel }}
        </span>
        <a
          v-if="resumeData.url"
          :href="resumeData.url"
          target="_blank"
          class="direct-link-btn"
          title="在浏览器中直接打开该候选人页面"
        >
          <el-icon><Link /></el-icon>
          打开原网页
        </a>
      </div>
    </div>

    <!-- 候选人主卡片 Hero -->
    <div class="candidate-hero-card">
      <div class="hero-main-row">
        <!-- 头像区 -->
        <div class="hero-avatar" :class="'platform-' + resumeData.platform">
          {{ avatarChar }}
        </div>

        <!-- 姓名与状态标签 -->
        <div class="hero-info">
          <div class="name-status-row">
            <h2 class="candidate-name">{{ resumeData.name || '候选人' }}</h2>
            <div class="status-tags-group">
              <span
                v-for="(tag, idx) in resumeData.statusTags"
                :key="idx"
                class="hero-status-pill"
                :class="getStatusClass(tag)"
              >
                {{ tag }}
              </span>
            </div>
          </div>

          <div class="hero-target-job">
            <span class="target-job-label">期望求职：</span>
            <strong class="target-job-value">{{ resumeData.targetRole || '对应专业岗位' }}</strong>
            <span v-if="resumeData.targetCity" class="dot-sep">·</span>
            <span v-if="resumeData.targetCity" class="target-city">{{ resumeData.targetCity }}</span>
            <span v-if="resumeData.targetSalary" class="dot-sep">·</span>
            <span v-if="resumeData.targetSalary" class="target-salary">{{ resumeData.targetSalary }}</span>
          </div>
        </div>
      </div>

      <!-- 核心指标栅格 (年龄 / 工作年限 / 学历 / 所在城市 / 期望薪资) -->
      <div class="metrics-grid">
        <div class="metric-item">
          <div class="metric-icon-wrap"><el-icon><Calendar /></el-icon></div>
          <div class="metric-content">
            <span class="metric-label">年龄</span>
            <strong class="metric-val">{{ resumeData.age || '未披露' }}</strong>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-icon-wrap"><el-icon><Briefcase /></el-icon></div>
          <div class="metric-content">
            <span class="metric-label">工作年限</span>
            <strong class="metric-val">{{ resumeData.experience || '在职经验' }}</strong>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-icon-wrap"><el-icon><School /></el-icon></div>
          <div class="metric-content">
            <span class="metric-label">最高学历</span>
            <strong class="metric-val">{{ resumeData.education || '统招大专及以上' }}</strong>
          </div>
        </div>
        <div class="metric-item">
          <div class="metric-icon-wrap"><el-icon><Location /></el-icon></div>
          <div class="metric-content">
            <span class="metric-label">城市 / 区域</span>
            <strong class="metric-val">{{ resumeData.location || resumeData.targetCity || '工作地' }}</strong>
          </div>
        </div>
        <div v-if="resumeData.targetSalary" class="metric-item highlight-salary">
          <div class="metric-icon-wrap"><el-icon><Money /></el-icon></div>
          <div class="metric-content">
            <span class="metric-label">期望薪资</span>
            <strong class="metric-val">{{ resumeData.targetSalary }}</strong>
          </div>
        </div>
      </div>
    </div>

    <!-- 核心专业技能标签云 -->
    <div v-if="resumeData.skills.length" class="web-section-card">
      <div class="section-title-row">
        <div class="title-with-bar">
          <span class="accent-bar"></span>
          <span class="section-title">核心专业技能与标签 ({{ resumeData.skills.length }})</span>
        </div>
      </div>
      <div class="skills-cloud">
        <span
          v-for="(skill, sIdx) in resumeData.skills"
          :key="sIdx"
          class="skill-chip"
        >
          <span class="skill-dot"></span>
          {{ skill }}
        </span>
      </div>
    </div>

    <!-- 工作履历时间线 -->
    <div v-if="resumeData.workExperiences.length" class="web-section-card">
      <div class="section-title-row">
        <div class="title-with-bar">
          <span class="accent-bar"></span>
          <span class="section-title">工作经历与履历时间线</span>
        </div>
        <span class="section-sub-badge">{{ resumeData.workExperiences.length }} 段从业履历</span>
      </div>

      <div class="timeline-container">
        <div
          v-for="(exp, expIdx) in resumeData.workExperiences"
          :key="expIdx"
          class="timeline-item"
        >
          <div class="timeline-dot-wrapper">
            <div class="timeline-dot"></div>
            <div v-if="expIdx < resumeData.workExperiences.length - 1" class="timeline-line"></div>
          </div>
          <div class="timeline-content">
            <div class="timeline-header">
              <div class="company-role-group">
                <span class="company-name">{{ exp.company }}</span>
                <span v-if="exp.role" class="role-badge">{{ exp.role }}</span>
              </div>
              <span class="timeline-period-pill">{{ exp.period }}</span>
            </div>
            <div v-if="exp.details && exp.details.length" class="timeline-details">
              <p v-for="(detail, dIdx) in exp.details" :key="dIdx" class="detail-line">
                {{ detail }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 教育背景 -->
    <div v-if="resumeData.educations.length" class="web-section-card">
      <div class="section-title-row">
        <div class="title-with-bar">
          <span class="accent-bar"></span>
          <span class="section-title">教育背景</span>
        </div>
      </div>

      <div class="education-grid">
        <div
          v-for="(edu, eduIdx) in resumeData.educations"
          :key="eduIdx"
          class="education-card"
        >
          <div class="school-icon-circle">
            <el-icon><School /></el-icon>
          </div>
          <div class="school-info">
            <div class="school-name-row">
              <strong class="school-name">{{ edu.school }}</strong>
              <span v-if="edu.degree" class="degree-pill">{{ edu.degree }}</span>
            </div>
            <div class="major-period-row">
              <span v-if="edu.major" class="major-name">{{ edu.major }}</span>
              <span v-if="edu.major && edu.period" class="dot-sep">·</span>
              <span v-if="edu.period" class="period-text">{{ edu.period }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 个人综合优势 / 自我评价 -->
    <div v-if="resumeData.advantage" class="web-section-card advantage-card">
      <div class="section-title-row">
        <div class="title-with-bar">
          <span class="accent-bar"></span>
          <span class="section-title">个人综合优势 / 评价</span>
        </div>
      </div>
      <div class="advantage-quote-box">
        <div class="quote-icon">“</div>
        <p class="advantage-text">{{ resumeData.advantage }}</p>
      </div>
    </div>

    <!-- 原始抓取数据流折叠面板 (供 HR 核对源字段) -->
    <div class="raw-data-expander">
      <el-collapse>
        <el-collapse-item title="🔍 查看原始卡片抓取数据流 (Raw Text)">
          <pre class="raw-dump-pre">{{ rawContent }}</pre>
        </el-collapse-item>
      </el-collapse>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  User,
  School,
  Briefcase,
  Location,
  Money,
  Calendar,
  Link
} from '@element-plus/icons-vue'

interface WorkExperience {
  period: string
  company: string
  role: string
  details: string[]
}

interface Education {
  period: string
  school: string
  major: string
  degree: string
}

interface MicroResumeData {
  platform: '51job' | 'boss' | 'other'
  platformName: string
  name: string
  activityLabel: string
  statusTags: string[]
  targetRole: string
  targetCity: string
  targetSalary: string
  age: string
  experience: string
  education: string
  location: string
  skills: string[]
  workExperiences: WorkExperience[]
  educations: Education[]
  advantage: string
  url: string
}

const props = defineProps<{
  rawContent: string
  candidateName?: string
  candidateUrl?: string
}>()

const resumeData = computed<MicroResumeData>(() => {
  return parseMicroResume(props.rawContent, props.candidateName, props.candidateUrl)
})

const avatarChar = computed(() => {
  const n = resumeData.value.name.trim()
  if (!n) return '牛'
  // 若以先生/女士结尾，取首字
  return n.charAt(0)
})

function getStatusClass(tag: string): string {
  if (tag.includes('急切') || tag.includes('随时到岗') || tag.includes('离职')) return 'status-urgent'
  if (tag.includes('活跃') || tag.includes('刚刚') || tag.includes('在线')) return 'status-active'
  if (tag.includes('回复快') || tag.includes('沟通')) return 'status-reply'
  return 'status-normal'
}

function parseMicroResume(content: string, propName?: string, propUrl?: string): MicroResumeData {
  const res: MicroResumeData = {
    platform: 'other',
    platformName: '在线微简历',
    name: propName ? propName.replace(/^【.*?】/, '').replace(/_.*$/, '') : '',
    activityLabel: '',
    statusTags: [],
    targetRole: '',
    targetCity: '',
    targetSalary: '',
    age: '',
    experience: '',
    education: '',
    location: '',
    skills: [],
    workExperiences: [],
    educations: [],
    advantage: '',
    url: propUrl || ''
  }

  if (!content) return res

  // 1. 平台判定
  if (content.includes('前程无忧') || content.includes('51job')) {
    res.platform = '51job'
    res.platformName = '前程无忧 51job'
  } else if (content.includes('BOSS') || content.includes('boss')) {
    res.platform = 'boss'
    res.platformName = 'BOSS 直聘'
  }

  // 2. URL 提取
  const urlMatch = content.match(/在线直达网址[：:]\s*(https?:\/\/[^\s\r\n]+)/)
  if (urlMatch) res.url = urlMatch[1].trim()

  // 3. 姓名 / 称谓提取
  const nameMatch = content.match(/姓名(?:\s*\/\s*称谓)?[：:]\s*([^\s\r\n]+)/)
  if (nameMatch) {
    res.name = nameMatch[1].trim()
  }

  // 4. 活跃度提取
  const actMatch = content.match(/(?:平台活跃标记|活跃状态)[：:]\s*([^\s\r\n]+)/)
  if (actMatch) res.activityLabel = actMatch[1].trim()

  // 5. 求职状态 / 意向标签
  const statusMatch = content.match(/求职状态[：:]\s*([^\r\n]+)/)
  if (statusMatch) {
    res.statusTags.push(...statusMatch[1].split(/[/,，、|]/).map(s => s.trim()).filter(Boolean))
  }

  // 扫描全文常见标签词（急切求职、回复快、电话沟通等）
  const commonTags = ['急切求职', '回复快', '电话沟通', '在职-月内到岗', '随时到岗', '离职-随时到岗', '在职-考虑好机会']
  for (const t of commonTags) {
    if (content.includes(t) && !res.statusTags.includes(t)) {
      res.statusTags.push(t)
    }
  }

  // 6. 期望岗位 / 期望城市 / 期望薪资
  const jobMatch = content.match(/(?:检索岗位|求职意向)[：:]\s*([^\r\n]+)/)
  if (jobMatch) {
    const val = jobMatch[1].trim()
    if (val.includes('万') || val.includes('K') || val.includes('k') || val.includes('元')) {
      res.targetSalary = val
    } else if (['上海', '北京', '深圳', '广州', '杭州', '南昌', '宁波', '南京', '武汉', '成都', '苏州', '重庆', '西安'].includes(val)) {
      res.targetCity = val
    } else {
      res.targetRole = val
    }
  }

  const cityMatch = content.match(/(?:目标城市|当前城市)[：:]\s*([^\s\r\n]+)/)
  if (cityMatch) res.targetCity = cityMatch[1].trim()

  // 7. 技能列表提取
  const skillsBlock = content.match(/【核心专业技能】([\s\S]*?)(?=【|$)/)
  if (skillsBlock) {
    const lines = skillsBlock[1]
      .split('\n')
      .map(s => s.trim().replace(/^[•●·*\-]\s*/, ''))
      .filter(Boolean)
    res.skills = lines.filter(s => s !== '岗位专业技能' && !s.includes('核心专业技能'))
  }

  // 8. 优势与自我评价提取
  const advBlock = content.match(/【(?:个人综合优势|自我评价|综合优势|自我介绍)】([\s\S]*?)(?=【|$)/)
  if (advBlock) {
    res.advantage = advBlock[1].trim()
  }

  // 9. 年龄、工龄、学历、地点提取
  const portraitMatch = content.match(/基本画像[：:]\s*([^\r\n]+)/)
  const textToScan = (portraitMatch ? portraitMatch[1] : '') + '\n' + content

  const ageM = textToScan.match(/(\d{2})\s*岁/)
  if (ageM) res.age = ageM[1] + ' 岁'

  const expM = textToScan.match(/(?:工作年限[：:]\s*)?(\d+年(?:以上|及以上|经验|工作经验)?)/)
  if (expM) {
    res.experience = expM[1].includes('年') && !expM[1].includes('经验') ? expM[1] + '经验' : expM[1]
  }

  const eduM = textToScan.match(/(博士|硕士|本科|大专|高中|中专)/)
  if (eduM) res.education = eduM[1]

  const locM = textToScan.match(/([^\s·|：:\r\n]+-[^\s·|\r\n]+)/)
  if (locM) res.location = locM[1]

  const salM = content.match(/(\d+(?:\.\d+)?-\d+(?:\.\d+)?(?:万|千|K|k)(?:\/[月年])?)/)
  if (salM) res.targetSalary = salM[1]

  // 10. 解析工作履历与教育经历 (任职履历快照)
  const snapshotMatch = content.match(/任职履历快照[：:]\s*([^\r\n]+)/)
  if (snapshotMatch) {
    const rawSegs = snapshotMatch[1].split('|').map(s => s.trim().replace(/^[•●·*\-]\s*/, '')).filter(Boolean)
    
    // 如果快照前几个是画像标签，跳过它们
    for (let i = 0; i < rawSegs.length; i++) {
      const seg = rawSegs[i]
      // 检测时间范围：形如 2020.09-至今 (6年) 或 2012.09-2016.06
      if (/(?:19|20)\d{2}[./-]\d{1,2}/.test(seg) && (seg.includes('至今') || seg.includes('-') || seg.includes('~'))) {
        const next1 = rawSegs[i + 1] || ''
        const next2 = rawSegs[i + 2] || ''
        const next3 = rawSegs[i + 3] || ''

        // 若 next1 是学校或者 next2 是学历
        const isEdu = next1.includes('大学') || next1.includes('学院') || ['本科', '硕士', '大专', '博士'].includes(next2)
        if (isEdu) {
          res.educations.push({
            period: seg,
            school: next1,
            major: !['本科', '硕士', '大专', '博士'].includes(next2) ? next2 : next3,
            degree: ['本科', '硕士', '大专', '博士'].includes(next2) ? next2 : (['本科', '硕士', '大专', '博士'].includes(next3) ? next3 : (res.education || '本科'))
          })
          i += 2
        } else {
          res.workExperiences.push({
            period: seg,
            company: next1 || '生物医药企业',
            role: next2 || res.targetRole || '专业岗位',
            details: []
          })
          i += 2
        }
      }
    }
  }

  // 兼容 BOSS 风格的【工作经历与项目成果】
  const workBlock = content.match(/【工作经历与项目成果】([\s\S]*?)(?=【|$)/)
  if (workBlock && res.workExperiences.length === 0) {
    const text = workBlock[1].replace(/工作经历[：:]/, '').trim()
    res.workExperiences.push({
      period: res.experience || '历年任职',
      company: '相关生物医药/医疗器械企业',
      role: res.targetRole || '项目主管',
      details: [text]
    })
  }

  // 兼容 BOSS 风格的【教育背景】
  const eduBlock = content.match(/【教育背景】([\s\S]*?)(?=【|$)/)
  if (eduBlock && res.educations.length === 0) {
    const parts = eduBlock[1].trim().split('|').map(s => s.trim())
    if (parts.length >= 2) {
      res.educations.push({
        period: '最高学历院校',
        school: parts[0],
        major: parts[1] || '',
        degree: parts[2] || res.education || '本科'
      })
    }
  }

  // 若没有从快照提取到姓名，从第一行提取
  if (!res.name) {
    const lines = content.split('\n').map(s => s.trim()).filter(Boolean)
    for (const l of lines) {
      if ((l.includes('先生') || l.includes('女士')) && l.length <= 6) {
        res.name = l
        break
      }
    }
  }

  return res
}
</script>

<style scoped>
.micro-resume-web-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px;
}

/* 顶部平台来源与真实性标识横幅 */
.web-platform-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-radius: 10px;
  background: var(--wb-surface-soft, #f8fafc);
  border: 1px solid var(--wb-border, #e2e8f0);
}

.web-platform-banner.platform-51job {
  border-left: 4px solid #ff6000;
}

.web-platform-banner.platform-boss {
  border-left: 4px solid #00bebd;
}

.banner-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.platform-logo-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--wb-text, #1e293b);
}

.platform-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 800;
  color: #fff;
}

.job51-icon {
  background: linear-gradient(135deg, #ff7a18 0%, #ff5200 100%);
}

.boss-icon {
  background: linear-gradient(135deg, #00d2c9 0%, #009d9c 100%);
}

.platform-card-type {
  font-size: 12px;
  color: var(--wb-muted, #64748b);
  border-left: 1px solid var(--wb-border, #cbd5e1);
  padding-left: 10px;
}

.banner-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.activity-pulse-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
  color: #10b981;
  background: rgba(16, 185, 129, 0.1);
  padding: 3px 10px;
  border-radius: 12px;
}

.pulse-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background-color: #10b981;
  box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7);
  animation: pulse 1.8s infinite;
}

@keyframes pulse {
  0% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7);
  }
  70% {
    transform: scale(1);
    box-shadow: 0 0 0 6px rgba(16, 185, 129, 0);
  }
  100% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 rgba(16, 185, 129, 0);
  }
}

.direct-link-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--wb-primary, #2563eb);
  text-decoration: none;
  padding: 3px 8px;
  border-radius: 6px;
  border: 1px solid rgba(37, 99, 235, 0.2);
  transition: all 0.2s;
}

.direct-link-btn:hover {
  background: rgba(37, 99, 235, 0.08);
}

/* 候选人主卡片 Hero */
.candidate-hero-card {
  padding: 20px;
  border-radius: 12px;
  background: var(--wb-surface, #ffffff);
  border: 1px solid var(--wb-border, #e2e8f0);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.03);
}

.hero-main-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}

.hero-avatar {
  width: 58px;
  height: 58px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  font-weight: 700;
  color: #fff;
  flex-shrink: 0;
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.12);
}

.hero-avatar.platform-51job {
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
}

.hero-avatar.platform-boss {
  background: linear-gradient(135deg, #00bebd 0%, #059669 100%);
}

.hero-avatar.platform-other {
  background: linear-gradient(135deg, #6366f1 0%, #4338ca 100%);
}

.hero-info {
  flex: 1;
  min-width: 0;
}

.name-status-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 6px;
}

.candidate-name {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--wb-text, #0f172a);
}

.status-tags-group {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.hero-status-pill {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
}

.hero-status-pill.status-urgent {
  color: #dc2626;
  background: #fef2f2;
  border: 1px solid #fecaca;
}

.hero-status-pill.status-active {
  color: #059669;
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
}

.hero-status-pill.status-reply {
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
}

.hero-status-pill.status-normal {
  color: #475569;
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
}

.hero-target-job {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  font-size: 13px;
  color: var(--wb-muted, #64748b);
}

.target-job-label {
  color: var(--wb-muted, #94a3b8);
}

.target-job-value {
  color: var(--wb-text, #1e293b);
  font-weight: 600;
}

.dot-sep {
  margin: 0 4px;
  color: var(--wb-border, #cbd5e1);
}

.target-salary {
  color: #ea580c;
  font-weight: 700;
}

/* 核心指标栅格 */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 12px;
  padding-top: 16px;
  border-top: 1px dashed var(--wb-border, #e2e8f0);
}

.metric-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--wb-surface-soft, #f8fafc);
  border: 1px solid var(--wb-border, #f1f5f9);
}

.metric-item.highlight-salary {
  background: #fff7ed;
  border-color: #fed7aa;
}

.metric-item.highlight-salary .metric-icon-wrap {
  background: #ea580c;
  color: #fff;
}

.metric-item.highlight-salary .metric-val {
  color: #ea580c;
  font-weight: 700;
}

.metric-icon-wrap {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: var(--wb-primary-subtle, #eff6ff);
  color: var(--wb-primary, #2563eb);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;
}

.metric-content {
  display: flex;
  flex-direction: column;
}

.metric-label {
  font-size: 11px;
  color: var(--wb-muted, #94a3b8);
}

.metric-val {
  font-size: 13px;
  color: var(--wb-text, #1e293b);
}

/* 通用分区卡片 */
.web-section-card {
  padding: 18px 20px;
  border-radius: 12px;
  background: var(--wb-surface, #ffffff);
  border: 1px solid var(--wb-border, #e2e8f0);
}

.section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.title-with-bar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.accent-bar {
  width: 3px;
  height: 15px;
  border-radius: 2px;
  background: var(--wb-primary, #2563eb);
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--wb-text, #0f172a);
}

.section-sub-badge {
  font-size: 11px;
  color: var(--wb-muted, #64748b);
  background: var(--wb-surface-soft, #f1f5f9);
  padding: 2px 8px;
  border-radius: 10px;
}

/* 技能标签云 */
.skills-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.skill-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  color: #1d4ed8;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
}

.skill-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #3b82f6;
}

/* 时间线 */
.timeline-container {
  display: flex;
  flex-direction: column;
  padding-left: 6px;
}

.timeline-item {
  display: flex;
  gap: 16px;
  position: relative;
}

.timeline-dot-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex-shrink: 0;
}

.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--wb-primary, #2563eb);
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.2);
  margin-top: 5px;
}

.timeline-line {
  width: 2px;
  flex: 1;
  background: var(--wb-border, #e2e8f0);
  margin: 6px 0;
  min-height: 36px;
}

.timeline-content {
  flex: 1;
  padding-bottom: 20px;
}

.timeline-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 6px;
}

.company-role-group {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.company-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--wb-text, #1e293b);
}

.role-badge {
  font-size: 12px;
  font-weight: 500;
  color: #0369a1;
  background: #e0f2fe;
  padding: 1px 8px;
  border-radius: 4px;
}

.timeline-period-pill {
  font-size: 11px;
  color: var(--wb-muted, #64748b);
  background: var(--wb-surface-soft, #f8fafc);
  padding: 2px 8px;
  border-radius: 6px;
  border: 1px solid var(--wb-border, #f1f5f9);
}

.timeline-details {
  margin-top: 6px;
  font-size: 13px;
  color: var(--wb-muted, #475569);
  line-height: 1.6;
}

.detail-line {
  margin: 4px 0;
}

/* 教育背景卡片 */
.education-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 12px;
}

.education-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--wb-surface-soft, #f8fafc);
  border: 1px solid var(--wb-border, #e2e8f0);
}

.school-icon-circle {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background: #e0e7ff;
  color: #4338ca;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
}

.school-info {
  flex: 1;
  min-width: 0;
}

.school-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.school-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--wb-text, #1e293b);
}

.degree-pill {
  font-size: 11px;
  font-weight: 500;
  color: #4f46e5;
  background: #eef2ff;
  padding: 1px 6px;
  border-radius: 4px;
}

.major-period-row {
  font-size: 12px;
  color: var(--wb-muted, #64748b);
}

/* 优势与评价 */
.advantage-card {
  background: linear-gradient(180deg, #f0fdf4 0%, #ffffff 100%);
  border-color: #bbf7d0;
}

.advantage-quote-box {
  display: flex;
  gap: 12px;
  padding: 10px 14px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.8);
}

.quote-icon {
  font-size: 32px;
  line-height: 1;
  font-family: Georgia, serif;
  color: #16a34a;
  opacity: 0.6;
}

.advantage-text {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: #166534;
}

/* 原始数据折叠 */
.raw-data-expander {
  margin-top: 8px;
}

.raw-dump-pre {
  margin: 0;
  padding: 12px;
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 260px;
  overflow-y: auto;
}
</style>
