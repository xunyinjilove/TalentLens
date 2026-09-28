<template>
  <div class="app-container">
    <!-- macOS 风格标题栏 -->
    <TitleBar :title="projectName || $t('app.title')" :show-back="!!projectId">
      <div class="titlebar-actions">
        <span class="job-badge">
          <el-icon><Briefcase /></el-icon>
          {{ jobTitle }}
        </span>
        <LanguageSwitcher />
        <button class="toolbar-btn" @click="router.push('/settings')">
          <el-icon><Setting /></el-icon>
        </button>
      </div>
    </TitleBar>

    <!-- 全局分析进度条 -->
    <Transition name="progress-slide">
      <div v-if="resumeStore.isAnalyzing && resumeStore.batchProgress.total > 0" class="global-progress">
        <div class="progress-info">
          <span class="progress-text">
            {{ $t('analysis.batchProgress', { current: resumeStore.batchProgress.current, total: resumeStore.batchProgress.total }) }}
          </span>
          <span class="progress-pct">{{ Math.round((resumeStore.batchProgress.current / resumeStore.batchProgress.total) * 100) }}%</span>
        </div>
        <div class="progress-track">
          <div class="progress-fill" :style="{ width: (resumeStore.batchProgress.current / resumeStore.batchProgress.total * 100) + '%' }"></div>
        </div>
      </div>
    </Transition>

    <main class="app-main">
      <!-- 左侧面板 -->
      <aside class="left-panel">
        <DropZone @select-files="handleSelectFiles" />

        <div class="resume-section">
          <div class="section-bar">
            <span class="section-label">
              <el-icon><Document /></el-icon>
              {{ $t('home.resumeList') }} ({{ resumeStore.resumes.length }})
            </span>
            <div class="section-btns">
              <button
                class="action-btn boss-btn"
                @click="openBossDialog(false)"
                title="4合1 全渠道矩阵寻才（BOSS直聘、智联、前程无忧、猎聘）"
              >
                <el-icon><Search /></el-icon>
                <span>矩阵寻才</span>
              </button>
              <button
                class="action-btn continue-btn"
                @click="openBossDialog(true)"
                :disabled="resumeStore.resumes.length === 0"
                title="在当前已抓取结果基础上，继续向下抓取更多候选人（自动排重）"
              >
                <el-icon><Plus /></el-icon>
                <span>继续寻才</span>
              </button>
              <button class="action-btn primary" @click="handleStartAnalysis" :disabled="resumeStore.isAnalyzing">
                <el-icon v-if="!resumeStore.isAnalyzing"><VideoPlay /></el-icon>
                <el-icon v-else class="spin"><Loading /></el-icon>
                {{ $t('home.startAnalysis') }}
              </button>
              <button
                class="action-btn"
                @click="handleReAnalyzeAll"
                :disabled="resumeStore.isAnalyzing || resumeStore.doneCount === 0"
                :title="$t('home.reAnalyzeAll')"
              >
                <el-icon><RefreshRight /></el-icon>
              </button>
              <button
                class="action-btn"
                @click="handleExport"
                :disabled="resumeStore.doneCount === 0"
                :title="$t('home.exportReport')"
              >
                <el-icon><Download /></el-icon>
              </button>
              <button class="action-btn" @click="handleClearAll" :title="$t('home.clear')">
                <el-icon><Delete /></el-icon>
              </button>
            </div>
          </div>

          <div class="resume-list">
            <ResumeCard
              v-for="resume in sortedResumes"
              :key="resume.id"
              :resume="resume"
              :selected="resumeStore.selectedId === resume.id"
              @click="resumeStore.selectResume(resume.id)"
              @re-analyze="handleReAnalyze"
              @delete="resumeStore.deleteResume"
            />
            <el-empty v-if="resumeStore.resumes.length === 0" :description="$t('home.dragHint')" />
          </div>
        </div>
      </aside>

      <!-- 右侧面板 -->
      <section class="right-panel">
        <div class="preview-section">
          <!-- 右侧 Tab 切换 -->
          <div class="section-bar">
            <div class="tab-bar">
              <button :class="['tab-btn', { active: rightTab === 'detail' }]" @click="rightTab = 'detail'">
                <el-icon><View /></el-icon>
                {{ $t('home.preview') }}
              </button>
              <button :class="['tab-btn', { active: rightTab === 'ranking' }]" @click="rightTab = 'ranking'">
                <el-icon><Document /></el-icon>
                {{ $t('project.ranking') }}
              </button>
              <button :class="['tab-btn', { active: rightTab === 'content' }]" @click="rightTab = 'content'">
                <el-icon><Tickets /></el-icon>
                {{ $t('home.parsedContent') }}
              </button>
            </div>
            <div class="tab-actions" v-if="rightTab === 'ranking'">
              <button class="action-btn" @click="handleExport">
                <el-icon><Document /></el-icon>
                {{ $t('project.exportExcel') }}
              </button>
            </div>
          </div>

          <!-- 详情 Tab -->
          <div v-show="rightTab === 'detail'" class="preview-content">
            <div v-if="resumeStore.selectedResume" class="resume-detail">
              <!-- 基本信息 -->
              <!-- 基本信息 -->
              <div class="detail-header">
                <el-icon class="file-icon"><Document /></el-icon>
                <div class="file-info">
                  <div class="file-title-row">
                    <h3>{{ resumeStore.selectedResume.fileName }}</h3>
                    <span v-if="resumeStore.selectedResume.is_merged_analysis || resumeStore.selectedResume.has_attachment" class="header-merged-tag">
                      ⭐ 深度终审版
                    </span>
                  </div>
                  <span class="file-meta">
                    {{ formatFileSize(resumeStore.selectedResume.fileSize) }} · {{ getFileTypeLabel(resumeStore.selectedResume.fileType) }}
                    <span v-if="resumeStore.selectedResume.attachment_file_name" class="attach-meta">
                      · 📎 已绑定附件: {{ resumeStore.selectedResume.attachment_file_name }}
                    </span>
                  </span>
                </div>
                <div class="header-attach-btn-wrap">
                  <button
                    v-if="resumeStore.selectedResume.status === 'done' && resumeStore.selectedResume.analysis"
                    class="pitch-card-top-btn"
                    @click="handleOpenPitchDialog"
                    title="一键生成/复制极简推介卡，推给用人部门业务主管"
                  >
                    <el-icon><Share /></el-icon> 📋 微信/钉钉推介卡
                  </button>
                  <button class="attach-upload-btn" @click="handleAttachResume(resumeStore.selectedResume.id)" title="选择或更新该候选人的完整 PDF/Word 简历并执行双源深度终审">
                    <el-icon><Paperclip /></el-icon> {{ resumeStore.selectedResume.has_attachment ? '更新完整附件' : '📎 补充完整附件简历' }}
                  </button>
                </div>
              </div>

              <!-- 分析结果 -->
              <div v-if="resumeStore.selectedResume.status === 'done' && resumeStore.selectedResume.analysis" class="analysis-result">
                <!-- 候选人信息卡片 -->
                <div v-if="resumeStore.selectedResume.analysis.candidateName" class="candidate-card">
                  <div class="candidate-name">{{ resumeStore.selectedResume.analysis.candidateName }}</div>
                  <div class="candidate-meta">
                    <span v-if="resumeStore.selectedResume.analysis.currentRole">{{ resumeStore.selectedResume.analysis.currentRole }}</span>
                    <span v-if="resumeStore.selectedResume.analysis.workYears">{{ resumeStore.selectedResume.analysis.workYears }}经验</span>
                    <span v-if="resumeStore.selectedResume.analysis.education">{{ resumeStore.selectedResume.analysis.education }}</span>
                    <span v-if="resumeStore.selectedResume.email" class="candidate-email-badge" title="点击复制候选人联系邮箱" @click="copyCandidateEmail">
                      <el-icon><Message /></el-icon> {{ resumeStore.selectedResume.email }}
                      <el-icon class="copy-icon"><DocumentCopy /></el-icon>
                    </span>
                    <span v-else class="candidate-email-badge disabled" title="在线直聘平台默认隐私保护，需在线打招呼沟通获取">
                      <el-icon><Message /></el-icon> 邮箱未公开
                    </span>
                    <span v-if="resumeStore.selectedResume.url" class="candidate-url-badge" title="点击复制候选人在线直达网址" @click="copyCandidateUrl">
                      <el-icon><Link /></el-icon> 在线直达主页
                      <el-icon class="copy-icon"><DocumentCopy /></el-icon>
                    </span>
                    <a v-if="resumeStore.selectedResume.url" :href="resumeStore.selectedResume.url" target="_blank" class="candidate-link-btn" title="在浏览器中直接打开候选人在线页面">
                      <el-icon><TopRight /></el-icon> 直达联系
                    </a>
                  </div>
                </div>

                <!-- 🚫 触碰用人部门一票否决红线警告 -->
                <div v-if="resumeStore.selectedResume.analysis?.redLineViolations && resumeStore.selectedResume.analysis.redLineViolations.length > 0" class="redline-alert-card">
                  <div class="redline-alert-header">
                    <div class="redline-title-group">
                      <div class="redline-badge-icon">
                        <el-icon><CircleClose /></el-icon>
                      </div>
                      <div class="redline-title-text">
                        <span class="redline-title">🚫 触碰用人部门一票否决红线 (Deal Breakers)</span>
                        <span class="redline-sub">该候选人违反岗位硬性考核红线，已被系统一票否决淘汰，综合评分已强制降级锁定（≤50分）</span>
                      </div>
                    </div>
                  </div>
                  <ul class="redline-violations-list">
                    <li v-for="(violation, vIdx) in resumeStore.selectedResume.analysis.redLineViolations" :key="vIdx" class="redline-violation-item">
                      <el-icon><Warning /></el-icon>
                      <span>{{ violation }}</span>
                    </li>
                  </ul>
                </div>

                <!-- ⭐ 达成用人部门优先加分项 -->
                <div v-if="resumeStore.selectedResume.analysis?.bonusMatches && resumeStore.selectedResume.analysis.bonusMatches.length > 0" class="bonus-match-card">
                  <div class="bonus-header">
                    <div class="bonus-title-group">
                      <div class="bonus-badge-icon">
                        <el-icon><Check /></el-icon>
                      </div>
                      <div class="bonus-title-text">
                        <span class="bonus-title">⭐ 达成用人部门优先加分项 (Bonus Points)</span>
                        <span class="bonus-sub">该候选人具备用人部门高优先级青睐特征，建议重点推介与优先安排面试</span>
                      </div>
                    </div>
                  </div>
                  <ul class="bonus-matches-list">
                    <li v-for="(bonus, bIdx) in resumeStore.selectedResume.analysis.bonusMatches" :key="bIdx" class="bonus-match-item">
                      <el-icon><Check /></el-icon>
                      <span>{{ bonus }}</span>
                    </li>
                  </ul>
                </div>

                <!-- 双源结合分析：初筛 ➔ 终审演进与一致性核验看板 -->
                <div v-if="resumeStore.selectedResume.is_merged_analysis || resumeStore.selectedResume.analysis?.consistency_check" class="merged-evolution-card">
                  <div class="evo-header">
                    <div class="evo-title">
                      <el-icon><Compass /></el-icon>
                      <span>初筛 ➔ 终审演进看板</span>
                    </div>
                    <div class="evo-scores" v-if="resumeStore.selectedResume.initial_score">
                      <span class="evo-init-score">初筛: {{ resumeStore.selectedResume.initial_score }}分</span>
                      <span class="evo-arrow">➔</span>
                      <span class="evo-final-score">终审: {{ resumeStore.selectedResume.score }}分</span>
                      <span class="evo-diff" :class="(resumeStore.selectedResume.score - resumeStore.selectedResume.initial_score) >= 0 ? 'up' : 'down'">
                        {{ (resumeStore.selectedResume.score - resumeStore.selectedResume.initial_score) >= 0 ? '+' : '' }}{{ resumeStore.selectedResume.score - resumeStore.selectedResume.initial_score }}分
                      </span>
                    </div>
                  </div>

                  <!-- 真实度与一致性核验 (Reality Check) -->
                  <div v-if="resumeStore.selectedResume.analysis?.consistency_check" class="consistency-box" :class="resumeStore.selectedResume.analysis.consistency_check.status">
                    <div class="consistency-head">
                      <span class="consistency-status-tag" :class="resumeStore.selectedResume.analysis.consistency_check.status">
                        {{ getConsistencyStatusText(resumeStore.selectedResume.analysis.consistency_check.status) }}
                      </span>
                      <span class="consistency-summary">{{ resumeStore.selectedResume.analysis.consistency_check.summary }}</span>
                    </div>
                    <ul v-if="resumeStore.selectedResume.analysis.consistency_check.details && resumeStore.selectedResume.analysis.consistency_check.details.length > 0" class="consistency-details">
                      <li v-for="(detail, dIdx) in resumeStore.selectedResume.analysis.consistency_check.details" :key="dIdx">
                        <el-icon><Check /></el-icon> {{ detail }}
                      </li>
                    </ul>
                  </div>

                  <!-- 终审评分依据说明 -->
                  <div v-if="resumeStore.selectedResume.analysis?.score_change_reason" class="score-reason-box">
                    <span class="reason-label">💡 终审评分依据：</span>
                    <span class="reason-text">{{ resumeStore.selectedResume.analysis.score_change_reason }}</span>
                  </div>
                </div>

                <!-- 🛡️ 简历防伪注水雷达与断层侦测器 (WaterCheck Radar) -->
                <div class="water-radar-card" :class="getWaterRiskClass(resumeStore.selectedResume.analysis?.water_check?.risk_level)">
                  <div class="water-radar-header">
                    <div class="radar-title-group">
                      <div class="radar-icon-shield" :class="resumeStore.selectedResume.analysis?.water_check?.risk_level || 'low'">
                        <el-icon><Warning v-if="resumeStore.selectedResume.analysis?.water_check?.risk_level === 'high'" /><Check v-else /></el-icon>
                      </div>
                      <div class="radar-title-text-wrap">
                        <div class="radar-main-title">
                          <span class="title-bold">🛡️ 简历防伪注水与破绽雷达</span>
                          <span class="radar-risk-pill" :class="resumeStore.selectedResume.analysis?.water_check?.risk_level || 'low'">
                            {{ getWaterRiskTitle(resumeStore.selectedResume.analysis?.water_check) }}
                          </span>
                        </div>
                        <div class="radar-sub-desc">
                          AI 深度侦测：职场时间线断层空窗、假大空缺乏量化描述、外包驻场与跳槽动荡隐患
                        </div>
                      </div>
                    </div>
                    <div class="radar-score-box" :class="resumeStore.selectedResume.analysis?.water_check?.risk_level || 'low'">
                      <div class="radar-score-num">{{ resumeStore.selectedResume.analysis?.water_check?.water_score ?? 15 }}</div>
                      <div class="radar-score-caption">注水破绽分 (越低越真实)</div>
                    </div>
                  </div>

                  <div class="water-radar-grid">
                    <!-- 1. 职场时间断层侦测 -->
                    <div class="radar-grid-item gaps-box">
                      <div class="grid-item-title">
                        <el-icon><Clock /></el-icon> 职场时间断层与空窗侦测
                      </div>
                      <div class="grid-item-content">
                        <div v-if="hasGaps(resumeStore.selectedResume.analysis?.water_check)" class="gap-list">
                          <div v-for="(gap, gIdx) in resumeStore.selectedResume.analysis?.water_check?.gaps" :key="gIdx" class="gap-item alert">
                            <span class="gap-dot">⚠️</span>
                            <span class="gap-text">{{ gap }}</span>
                          </div>
                        </div>
                        <div v-else class="gap-item safe">
                          <el-icon><CircleCheck /></el-icon> 履历时间线连贯，未侦测到异常离职断层
                        </div>
                      </div>
                    </div>

                    <!-- 2. 假大空与欠缺量化佐证 -->
                    <div class="radar-grid-item vague-box">
                      <div class="grid-item-title">
                        <el-icon><Aim /></el-icon> 欠缺量化佐证 / 夸大嫌疑
                      </div>
                      <div class="grid-item-content">
                        <div v-if="hasVagueClaims(resumeStore.selectedResume.analysis?.water_check)" class="vague-list">
                          <div v-for="(claim, cIdx) in resumeStore.selectedResume.analysis?.water_check?.vague_claims" :key="cIdx" class="vague-item alert">
                            <span class="claim-dot">🔍</span>
                            <span class="claim-text">{{ claim }}</span>
                          </div>
                        </div>
                        <div v-else class="vague-item safe">
                          <el-icon><CircleCheck /></el-icon> 项目职责与业绩成果具备具体量化指标，扎实度高
                        </div>
                      </div>
                    </div>

                    <!-- 3. 外包派遣与履历动荡风险 -->
                    <div class="radar-grid-item stability-box">
                      <div class="grid-item-title">
                        <el-icon><Briefcase /></el-icon> 外包驻场 & 稳定性研判
                      </div>
                      <div class="grid-item-content">
                        <div v-if="resumeStore.selectedResume.analysis?.water_check?.outsourcing_warning && !resumeStore.selectedResume.analysis?.water_check?.outsourcing_warning.includes('无')" class="alert-pill outsourcing">
                          <span class="pill-label">🏢 外包/驻场提醒:</span>
                          <span class="pill-msg">{{ resumeStore.selectedResume.analysis?.water_check?.outsourcing_warning }}</span>
                        </div>
                        <div v-if="resumeStore.selectedResume.analysis?.water_check?.frequent_hop_warning && !resumeStore.selectedResume.analysis?.water_check?.frequent_hop_warning.includes('合理') && !resumeStore.selectedResume.analysis?.water_check?.frequent_hop_warning.includes('无') && !resumeStore.selectedResume.analysis?.water_check?.frequent_hop_warning.includes('稳定')" class="alert-pill hopping">
                          <span class="pill-label">⏳ 稳定性提醒:</span>
                          <span class="pill-msg">{{ resumeStore.selectedResume.analysis?.water_check?.frequent_hop_warning }}</span>
                        </div>
                        <div v-if="isStabilityClean(resumeStore.selectedResume.analysis?.water_check)" class="stability-safe">
                          <el-icon><CircleCheck /></el-icon> 无明显外包挂靠痕迹，跳槽频率在健康合理区间
                        </div>
                      </div>
                    </div>

                    <!-- 4. 初试防伪一针见血反问 -->
                    <div class="radar-grid-item interview-probe-box" v-if="resumeStore.selectedResume.analysis?.water_check?.advise_questions?.length">
                      <div class="grid-item-title">
                        <el-icon><QuestionFilled /></el-icon> 初试防伪反问建议（破水核验）
                      </div>
                      <div class="grid-item-content">
                        <div class="probe-list">
                          <div v-for="(q, qIdx) in resumeStore.selectedResume.analysis?.water_check?.advise_questions" :key="qIdx" class="probe-item">
                            <span class="probe-num">追问{{ qIdx + 1 }}</span>
                            <span class="probe-text">{{ q }}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 候选人在线直达与沟通专区 -->
                <div class="candidate-portal-bar">
                  <div class="portal-header">
                    <span class="portal-title">🌐 候选人在线直达与沟通</span>
                    <span class="portal-sub">直连招聘平台候选人详情页，可直接在线联系、查看最新动态或加收藏</span>
                  </div>
                  <div class="portal-actions">
                    <button
                      class="portal-act-btn highlight"
                      @click="handleOpenPitchDialog"
                      title="生成微信/钉钉极简推介卡，支持一键复制与微调"
                    >
                      <el-icon><Share /></el-icon> 极简推介卡 (微信/钉钉)
                    </button>
                    <button
                      class="portal-act-btn primary"
                      :disabled="!resumeStore.selectedResume.url"
                      @click="handleOpenCandidateUrl"
                    >
                      <el-icon><Compass /></el-icon> 在浏览器中直接打开候选人主页
                    </button>
                    <button
                      class="portal-act-btn secondary"
                      :disabled="!resumeStore.selectedResume.url"
                      @click="copyCandidateUrl"
                    >
                      <el-icon><DocumentCopy /></el-icon> 复制直达网址
                    </button>
                    <span v-if="!resumeStore.selectedResume.url" class="portal-empty-tip">
                      （本地导入简历，暂无在线直达链接）
                    </span>
                  </div>
                </div>

                <!-- 评分面板 -->
                <div class="score-section">
                  <div class="main-score" :class="getScoreClass(resumeStore.selectedResume.score)">
                    <span class="score-value" :key="resumeStore.selectedResume.id">{{ animatedDetailScore }}</span>
                    <span class="score-label">{{ $t('analysis.totalScore') }}</span>
                  </div>
                  <div class="score-breakdown">
                    <div class="score-item">
                      <span class="label">{{ $t('analysis.skillMatch') }}</span>
                      <el-progress :percentage="resumeStore.selectedResume.analysis.skillMatch" :color="getProgressColor(resumeStore.selectedResume.analysis.skillMatch)" />
                    </div>
                    <div class="score-item">
                      <span class="label">{{ $t('analysis.experienceMatch') }}</span>
                      <el-progress :percentage="resumeStore.selectedResume.analysis.experienceMatch" :color="getProgressColor(resumeStore.selectedResume.analysis.experienceMatch)" />
                    </div>
                    <div class="score-item">
                      <span class="label">{{ $t('analysis.educationMatch') }}</span>
                      <el-progress :percentage="resumeStore.selectedResume.analysis.educationMatch" :color="getProgressColor(resumeStore.selectedResume.analysis.educationMatch)" />
                    </div>
                  </div>
                </div>

                <!-- 推荐结论 -->
                <div class="recommendation-section">
                  <div class="recommendation-badge" :class="resumeStore.selectedResume.analysis.recommendation">
                    {{ $t(`analysis.recommendations.${resumeStore.selectedResume.analysis.recommendation}`) }}
                  </div>
                </div>

                <!-- 评分详解 -->
                <div v-if="resumeStore.selectedResume.analysis.skillDetail" class="detail-cards">
                  <div class="detail-card">
                    <h4>{{ $t('analysis.skillMatch') }}详解</h4>
                    <p>{{ resumeStore.selectedResume.analysis.skillDetail }}</p>
                  </div>
                  <div class="detail-card">
                    <h4>{{ $t('analysis.experienceMatch') }}详解</h4>
                    <p>{{ resumeStore.selectedResume.analysis.experienceDetail }}</p>
                  </div>
                  <div class="detail-card">
                    <h4>{{ $t('analysis.educationMatch') }}详解</h4>
                    <p>{{ resumeStore.selectedResume.analysis.educationDetail }}</p>
                  </div>
                </div>

                <!-- 优势与不足 -->
                <div class="strength-weakness">
                  <div class="sw-block strengths">
                    <h4><el-icon><CircleCheck /></el-icon> {{ $t('analysis.strengths') }}</h4>
                    <ul>
                      <li v-for="(item, idx) in resumeStore.selectedResume.analysis.strengths" :key="idx">{{ item }}</li>
                    </ul>
                  </div>
                  <div class="sw-block weaknesses">
                    <h4><el-icon><Warning /></el-icon> {{ $t('analysis.weaknesses') }}</h4>
                    <ul>
                      <li v-for="(item, idx) in resumeStore.selectedResume.analysis.weaknesses" :key="idx">{{ item }}</li>
                    </ul>
                  </div>
                </div>

                <!-- 风险提示 -->
                <div v-if="resumeStore.selectedResume.analysis.risks && resumeStore.selectedResume.analysis.risks.length > 0" class="risk-section">
                  <h4><el-icon><Warning /></el-icon> 风险提示</h4>
                  <ul>
                    <li v-for="(item, idx) in resumeStore.selectedResume.analysis.risks" :key="idx">{{ item }}</li>
                  </ul>
                </div>

                <!-- 面试建议 -->
                <div v-if="resumeStore.selectedResume.analysis.interviewSuggestions && resumeStore.selectedResume.analysis.interviewSuggestions.length > 0" class="interview-section">
                  <h4><el-icon><ChatLineSquare /></el-icon> 面试建议</h4>
                  <ul>
                    <li v-for="(item, idx) in resumeStore.selectedResume.analysis.interviewSuggestions" :key="idx">{{ item }}</li>
                  </ul>
                </div>

                <!-- AI 初试提纲与参考回答（5道定制问题） -->
                <div v-if="resumeStore.selectedResume.analysis.interviewQA && resumeStore.selectedResume.analysis.interviewQA.length > 0" class="qa-section">
                  <div class="qa-section-head">
                    <h4><el-icon><QuestionFilled /></el-icon> AI 预面试提纲与参考回答 ({{ resumeStore.selectedResume.analysis.interviewQA.length }}题)</h4>
                    <button class="copy-qa-btn" @click="handleCopyAllQA" title="一键复制全套提纲">
                      <el-icon><DocumentCopy /></el-icon> 复制全套提纲
                    </button>
                  </div>
                  <div class="qa-list">
                    <div v-for="(item, idx) in resumeStore.selectedResume.analysis.interviewQA" :key="idx" class="qa-card">
                      <div class="qa-card-header">
                        <span class="qa-badge" :class="getCategoryClass(item.category)">{{ item.category || '技术提问' }}</span>
                        <span class="qa-tag">Q{{ idx + 1 }}</span>
                        <span class="qa-question-text">{{ item.question }}</span>
                      </div>
                      <div v-if="item.reference_answer" class="qa-card-body">
                        <div class="qa-answer-title">
                          <el-icon><InfoFilled /></el-icon> 基于简历事实的参考回答要点：
                        </div>
                        <div class="qa-answer-text">{{ item.reference_answer }}</div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- AI 总结 -->
                <div class="summary-section">
                  <h4><el-icon><ChatLineSquare /></el-icon> {{ $t('analysis.summary') }}</h4>
                  <p>{{ resumeStore.selectedResume.analysis.summary }}</p>
                </div>
              </div>

              <!-- 状态提示 -->
              <div v-else-if="resumeStore.selectedResume.status === 'pending'" class="status-hint pending">
                <el-icon><Clock /></el-icon>
                <span>{{ $t('analysis.pendingHint') }}</span>
              </div>
              <div v-else-if="resumeStore.selectedResume.status === 'analyzing'" class="status-hint analyzing">
                <el-icon class="spin"><Loading /></el-icon>
                <span>{{ $t('analysis.analyzingHint') }}</span>
              </div>
              <div v-else-if="resumeStore.selectedResume.status === 'error'" class="status-hint error-box">
                <div class="error-header">
                  <el-icon class="error-icon"><CircleClose /></el-icon>
                  <span class="error-title">分析出错</span>
                </div>
                <div class="error-detail-text" v-if="resumeStore.selectedResume.errorMessage">
                  {{ resumeStore.selectedResume.errorMessage }}
                </div>
                <div class="error-actions">
                  <el-button size="small" type="danger" plain @click="resumeStore.reAnalyze(resumeStore.selectedResume.id)">
                    <el-icon><RefreshRight /></el-icon> 重新分析
                  </el-button>
                  <el-button size="small" @click="handleCopyError(resumeStore.selectedResume)">
                    <el-icon><DocumentCopy /></el-icon> 复制错误信息
                  </el-button>
                  <el-button size="small" @click="handleOpenLogDir">
                    <el-icon><Tickets /></el-icon> 导出诊断日志
                  </el-button>
                </div>
              </div>
            </div>

            <el-empty v-else :description="$t('home.selectToPreview')" />
          </div>

          <!-- 解析内容 Tab -->
          <div v-show="rightTab === 'content'" class="parsed-content-tab">
            <div v-if="!resumeStore.selectedResume" class="content-empty">
              <el-empty :description="$t('home.selectToPreview')" />
            </div>
            <div v-else-if="contentLoading" class="content-loading">
              <el-icon class="spin"><Loading /></el-icon>
              <span>{{ $t('home.loadingContent') }}</span>
            </div>
            <div v-else-if="!parsedContent" class="content-empty">
              <el-empty :description="$t('home.noContent')" />
            </div>
            <div v-else class="content-viewer">
              <div class="content-header">
                <span class="content-filename">{{ resumeStore.selectedResume.fileName }}</span>
                <span class="content-stats">{{ $t('home.contentLength', { count: parsedContent.length }) }}</span>
              </div>
              <div class="content-hint">{{ $t('home.parsedContentHint') }}</div>
              <pre class="content-text">{{ parsedContent }}</pre>
            </div>
          </div>

          <!-- 排名 Tab -->
          <div v-show="rightTab === 'ranking'" class="ranking-content">
            <div v-if="rankedResumes.length > 0" class="ranking-list">
              <div v-for="(r, idx) in rankedResumes" :key="r.id" class="ranking-item" :class="{ 'top-3': idx < 3 }">
                <div class="rank-num" :class="'rank-' + (idx < 3 ? idx + 1 : 'other')">{{ idx + 1 }}</div>
                <div class="rank-info">
                  <div class="rank-name">{{ r.analysis?.candidateName || r.fileName }}</div>
                  <div class="rank-meta">
                    <span v-if="r.analysis?.currentRole">{{ r.analysis.currentRole }}</span>
                    <span v-if="r.analysis?.workYears">{{ r.analysis.workYears }}</span>
                  </div>
                </div>
                <div class="rank-scores">
                  <span class="rank-score-main" :class="getScoreClass(r.score)">{{ r.score }}</span>
                  <div class="rank-score-detail">
                    <span>技能 {{ r.analysis?.skillMatch || 0 }}</span>
                    <span>经验 {{ r.analysis?.experienceMatch || 0 }}</span>
                    <span>学历 {{ r.analysis?.educationMatch || 0 }}</span>
                  </div>
                </div>
                <div class="rank-rec">
                  <span class="rec-badge" :class="r.analysis?.recommendation">
                    {{ $t(`analysis.recommendations.${r.analysis?.recommendation || 'consider'}`) }}
                  </span>
                </div>
              </div>
            </div>
            <el-empty v-else :description="'暂无已分析的简历'" />
          </div>
        </div>
      </section>
    </main>

    <AIConfigGuide v-model="showConfigGuide" />

    <!-- BOSS 直聘自动搜寻与智能评测弹窗 -->
    <BossSearchDialog
      v-model="showBossDialog"
      :project-id="projectId"
      :project-name="projectName"
      :job-title="projectStore.currentProject?.job_config?.title || jobTitle"
      :exp-years="projectStore.currentProject?.job_config?.experience_years"
      :edu-level="projectStore.currentProject?.job_config?.education_level"
      :is-continue="bossDialogContinueMode"
      @refresh="handleRefreshProject"
    />

    <!-- 业务主管极简推介卡弹窗 -->
    <el-dialog
      v-model="showPitchDialog"
      title="📢 业务主管极简推介卡（微信 / 钉钉一键转发）"
      width="600px"
      destroy-on-close
      class="pitch-card-dialog"
    >
      <div class="pitch-dialog-body">
        <div class="pitch-tip-banner">
          💡 专为向业务主管/用人部门推人设计。提炼 3 条核心硬核亮点与关键把关提醒，主管在手机微信/钉钉 10 秒即可拍板是否初试！
        </div>
        <div class="pitch-preview-card">
          <div class="pitch-preview-header">
            <span class="pitch-tag">推介卡文案预览（可直接编辑微调）</span>
            <div class="pitch-card-tools">
              <el-button size="small" text type="primary" @click="resetPitchText">
                <el-icon><RefreshRight /></el-icon> 恢复默认
              </el-button>
            </div>
          </div>
          <el-input
            v-model="currentPitchText"
            type="textarea"
            :rows="12"
            placeholder="推介卡生成中..."
            class="pitch-textarea"
          />
        </div>
      </div>
      <template #footer>
        <div class="pitch-dialog-footer">
          <el-button @click="showPitchDialog = false">关闭</el-button>
          <el-button type="primary" @click="copyPitchText">
            <el-icon><DocumentCopy /></el-icon> 一键复制到剪贴板
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 开发者调试面板 (F12 切换) -->
    <DevPanel />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Setting, Briefcase, Document, VideoPlay, Delete, View,
  CircleCheck, Warning, ChatLineSquare, Clock, Loading, CircleClose,
  RefreshRight, Download, Tickets, QuestionFilled, DocumentCopy, InfoFilled, Search,
  ChatDotRound, DocumentAdd, Connection, CloseBold, Paperclip, Compass, Check, Message,
  Link, TopRight, Plus, Share, Aim
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import TitleBar from '../components/TitleBar.vue'
import DropZone from '../components/DropZone.vue'
import ResumeCard from '../components/ResumeCard.vue'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import AIConfigGuide from '../components/AIConfigGuide.vue'
import DevPanel from '../components/DevPanel.vue'
import BossSearchDialog from '../components/BossSearchDialog.vue'
import { useResumeStore } from '../composables/useResumeStore'
import { useProjectStore } from '../composables/useProjectStore'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const resumeStore = useResumeStore()
const projectStore = useProjectStore()

// 项目上下文
const projectId = computed(() => (route.params.id as string) || '')
const projectName = computed(() => projectStore.currentProject?.name || '')
const rightTab = ref<'detail' | 'ranking' | 'content'>('detail')
const parsedContent = ref('')
const contentLoading = ref(false)

const jobTitle = ref('高级Go开发工程师')
const showConfigGuide = ref(false)
const showBossDialog = ref(false)
const bossDialogContinueMode = ref(false) // true=继续寻才模式（跳过已有候选人）
const showPitchDialog = ref(false) // 业务主管推介卡弹窗
const currentPitchText = ref('')   // 当前推介卡文案
const animatedDetailScore = ref(0)
let scoreAnimationFrame: number | null = null

// 打开矩阵寻才弹窗（isContinue=true 时为继续寻才模式）
function openBossDialog(isContinue: boolean) {
  bossDialogContinueMode.value = isContinue
  showBossDialog.value = true
}

async function handleRefreshProject() {
  if (projectId.value) {
    await projectStore.refreshProject(projectId.value)
    await resumeStore.loadProjectResumes(projectId.value)
  }
}

function getCategoryClass(category?: string): string {
  if (!category) return 'cat-tech'
  if (category.includes('技术')) return 'cat-tech'
  if (category.includes('项目')) return 'cat-project'
  if (category.includes('真实') || category.includes('短板') || category.includes('疑点')) return 'cat-gap'
  return 'cat-tech'
}

function getConsistencyStatusText(status?: string): string {
  if (status === 'consistent') return '✅ 双源吻合（真实度高）'
  if (status === 'warning') return '⚠️ 存在疑点或细节出入'
  if (status === 'conflict') return '❌ 存在明显冲突'
  return '🔍 真实度核验完成'
}

async function handleAttachResume(resumeId: string) {
  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch {}

  if (WailsApp && WailsApp.SelectAndAttachResumeFile) {
    try {
      const selected = await WailsApp.SelectAndAttachResumeFile(resumeId)
      if (selected) {
        ElMessage.success(`已绑定完整附件：${selected.split('\\').pop() || selected}，正在启动双源交叉核验与深度终审...`)
        if (projectId.value) {
          await resumeStore.loadProjectResumes(projectId.value)
        }
      }
    } catch (err: any) {
      ElMessage.error(`绑定附件失败: ${err.message || err}`)
    }
  } else {
    ElMessage.info('开发模式：模拟绑定完整附件简历并启动终审')
  }
}

async function handleOpenCandidateUrl() {
  const url = resumeStore.selectedResume?.url
  if (!url) {
    ElMessage.warning('当前候选人暂无在线主页网址')
    return
  }

  let WailsApp: any = null
  try { WailsApp = await import('../../wailsjs/go/main/App') } catch {}

  if (WailsApp && WailsApp.OpenURL) {
    try {
      await WailsApp.OpenURL(url)
      ElMessage.success('✅ 已调用默认浏览器打开候选人在线主页！')
      return
    } catch (e: any) {
      ElMessage.error(`打开浏览器失败: ${e.message || e}`)
    }
  }

  // 开发预览模式回退
  window.open(url, '_blank')
  ElMessage.success('已在新标签页中打开候选人主页！')
}

function copyCandidateEmail() {
  const email = resumeStore.selectedResume?.email
  if (email) {
    try {
      navigator.clipboard.writeText(email)
      ElMessage.success(`✅ 已复制候选人联系邮箱：${email}`)
    } catch (e) {
      ElMessage.info(`候选人联系邮箱：${email}`)
    }
  } else {
    ElMessage.warning(`该候选人暂未公开个人邮箱，可直接通过在线打招呼沟通。`)
  }
}

function copyCandidateUrl() {
  const url = resumeStore.selectedResume?.url
  if (url) {
    try {
      navigator.clipboard.writeText(url)
      ElMessage.success(`✅ 已复制候选人在线直达网址，可粘贴到浏览器中联系！`)
    } catch (e) {
      ElMessage.info(`候选人网址：${url}`)
    }
  }
}

async function handleCopyAllQA() {
  const resume = resumeStore.selectedResume
  if (!resume || !resume.analysis?.interviewQA || resume.analysis.interviewQA.length === 0) return

  const candidateName = resume.analysis.candidateName || resume.fileName
  const lines = [
    `【AI 预面试提纲与参考回答】`,
    `候选人：${candidateName}`,
    `综合得分：${resume.score || 0}分`,
    `========================================`
  ]

  resume.analysis.interviewQA.forEach((item, idx) => {
    lines.push(`\nQ${idx + 1}【${item.category || '技术问题'}】: ${item.question}`)
    if (item.reference_answer) {
      lines.push(`参考回答（基于简历事实）: ${item.reference_answer}`)
    }
  })

  try {
    await navigator.clipboard.writeText(lines.join('\n'))
    ElMessage.success('已复制 5 组面试问题与参考回答到剪贴板！')
  } catch (err) {
    ElMessage.error('复制失败，请手动选择复制')
  }
}

// ---------------------------------------------------------------------
// 业务主管极简推介卡（微信 / 钉钉一键推送）
// ---------------------------------------------------------------------
function getPitchCardText(resume: any): string {
  if (resume.analysis?.managerPitch) {
    return resume.analysis.managerPitch
  }
  const a = resume.analysis
  const candidateName = a?.candidateName || resume.fileName.replace(/\.[^/.]+$/, '')
  const role = a?.currentRole || '开发工程师'
  const exp = a?.workYears || '具备工作经验'
  const edu = a?.education || '学历符合'
  const score = resume.score || a?.overallScore || 0
  const topStrengths = (a?.strengths || []).slice(0, 3).map((s: string, idx: number) => `  ${idx + 1}. ${s}`).join('\n') || '  - 具备核心开发技能与实战背景'
  const risk = (a?.risks && a.risks[0]) || (a?.weaknesses && a.weaknesses[0]) || '建议初试深入核实项目真实职责与实操深度'

  let redLineNotice = ''
  if (a?.redLineViolations && a.redLineViolations.length > 0) {
    redLineNotice = `\n🚨 触碰用人红线：${a.redLineViolations.join('；')}`
  }

  let bonusNotice = ''
  if (a?.bonusMatches && a.bonusMatches.length > 0) {
    bonusNotice = `\n⭐ 命中优先加分：${a.bonusMatches.join('；')}`
  }

  let text = `【候选人极简推介卡】\n👤 候选人：${candidateName} | 现任：${role}\n📌 背景画像：${exp} | ${edu}\n🎯 综合匹配：${Math.round(score)}分${redLineNotice}${bonusNotice}\n✨ 核心亮点：\n${topStrengths}\n⚠️ 关注提示：\n  - ${risk}`
  if (resume.url) {
    text += `\n🔗 在线主页：${resume.url}`
  }
  return text
}

function handleOpenPitchDialog() {
  const resume = resumeStore.selectedResume
  if (!resume || !resume.analysis) {
    ElMessage.warning('候选人尚未完成分析，无法生成推介卡')
    return
  }
  currentPitchText.value = getPitchCardText(resume)
  showPitchDialog.value = true
}

function resetPitchText() {
  const resume = resumeStore.selectedResume
  if (resume) {
    currentPitchText.value = getPitchCardText(resume)
    ElMessage.info('已恢复默认推介卡内容')
  }
}

async function copyPitchText() {
  if (!currentPitchText.value) return
  try {
    await navigator.clipboard.writeText(currentPitchText.value)
    ElMessage.success('✅ 已复制主管推介卡！可直接在微信/钉钉中粘贴给业务主管')
    showPitchDialog.value = false
  } catch (err: any) {
    ElMessage.error(`复制失败: ${err.message || err}`)
  }
}

// ---------------------------------------------------------------------
// 简历防伪注水雷达与断层侦测器
// ---------------------------------------------------------------------
function getWaterRiskClass(riskLevel?: string): string {
  if (riskLevel === 'high') return 'risk-high'
  if (riskLevel === 'medium') return 'risk-medium'
  return 'risk-low'
}

function getWaterRiskTitle(waterCheck?: any): string {
  if (!waterCheck) return '真实度高 · 扎实可信'
  if (waterCheck.risk_level === 'high') return '注水嫌疑高 · 高风险'
  if (waterCheck.risk_level === 'medium') return '存在疑点 · 中风险'
  return '真实度高 · 扎实可信'
}

function hasGaps(waterCheck?: any): boolean {
  if (!waterCheck || !waterCheck.gaps || waterCheck.gaps.length === 0) return false
  const first = waterCheck.gaps[0]
  if (first.includes('时间线连贯') || first.includes('无显著断层')) return false
  return true
}

function hasVagueClaims(waterCheck?: any): boolean {
  if (!waterCheck || !waterCheck.vague_claims || waterCheck.vague_claims.length === 0) return false
  const first = waterCheck.vague_claims[0]
  if (first.includes('量化明确') || first.includes('详实') || first.includes('无')) return false
  return true
}

function isStabilityClean(waterCheck?: any): boolean {
  if (!waterCheck) return true
  const hasOutsourcing = waterCheck.outsourcing_warning && !waterCheck.outsourcing_warning.includes('无')
  const hasHopping = waterCheck.frequent_hop_warning && !waterCheck.frequent_hop_warning.includes('合理') && !waterCheck.frequent_hop_warning.includes('无') && !waterCheck.frequent_hop_warning.includes('稳定')
  return !hasOutsourcing && !hasHopping
}

async function handleCopyError(resume: any) {
  if (!resume) return
  const text = `【简历分析失败诊断】\n文件名: ${resume.fileName}\n错误详情: ${resume.errorMessage || '未知错误'}\n时间: ${new Date().toLocaleString()}`
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('错误诊断信息已复制到剪贴板！')
  } catch (err) {
    ElMessage.error('复制失败')
  }
}

async function handleOpenLogDir() {
  try {
    const WailsApp = await import('../../wailsjs/go/main/App')
    if (WailsApp && WailsApp.ExportErrorReportFile) {
      await WailsApp.ExportErrorReportFile()
      ElMessage.success('已打开日志文件夹')
    } else {
      ElMessage.info('日志保存在系统“文档/TalentLens/logs”目录')
    }
  } catch (e) {
    ElMessage.info('日志保存在系统“文档/TalentLens/logs”目录')
  }
}

watch(
  () => resumeStore.selectedResume?.score,
  (newScore) => {
    if (newScore && resumeStore.selectedResume?.status === 'done') {
      animateDetailScore(newScore)
    } else {
      animatedDetailScore.value = 0
    }
  },
  { immediate: true }
)

function animateDetailScore(targetScore: number) {
  const startScore = 0
  const startTime = performance.now()
  const duration = 1000
  function update(currentTime: number) {
    const elapsed = currentTime - startTime
    const progress = Math.min(elapsed / duration, 1)
    const easeProgress = progress === 1 ? 1 : 1 - Math.pow(2, -10 * progress)
    animatedDetailScore.value = Math.round(startScore + (targetScore - startScore) * easeProgress)
    if (progress < 1) scoreAnimationFrame = requestAnimationFrame(update)
  }
  if (scoreAnimationFrame) cancelAnimationFrame(scoreAnimationFrame)
  scoreAnimationFrame = requestAnimationFrame(update)
}

function checkAIConfig(): boolean {
  const saved = localStorage.getItem('goresume_settings')
  if (!saved) return false
  try {
    const settings = JSON.parse(saved)
    return settings.ai?.apiKey && settings.ai.apiKey.length >= 10
  } catch { return false }
}

const sortedResumes = computed(() => {
  return [...resumeStore.resumes].sort((a, b) => {
    if (a.status === 'done' && b.status !== 'done') return -1
    if (a.status !== 'done' && b.status === 'done') return 1
    if (a.status === 'done' && b.status === 'done') return (b.score || 0) - (a.score || 0)
    if (a.status === 'analyzing' && b.status === 'pending') return -1
    if (a.status === 'pending' && b.status === 'analyzing') return 1
    return 0
  })
})

// 排名列表（仅已完成分析的，按分数降序）
const rankedResumes = computed(() => {
  return resumeStore.resumes
    .filter(r => r.status === 'done' && r.score)
    .sort((a, b) => (b.score || 0) - (a.score || 0))
})

// 导出报告
async function handleExport() {
  if (!projectId.value) {
    ElMessage.warning(t('project.exportFailed') + ' - ' + t('project.createFirst'))
    return
  }
  if (resumeStore.doneCount === 0) {
    ElMessage.warning(t('home.noAnalyzedResumes'))
    return
  }
  try {
    const filePath = await projectStore.exportReport(projectId.value)
    if (filePath) {
      ElMessage.success(t('project.exportSuccess') + '：' + filePath)
      // 自动打开导出目录
      try {
        const WailsApp = await import('../../wailsjs/go/main/App')
        await WailsApp.OpenExportDir()
      } catch {}
    } else {
      ElMessage.error(t('project.exportFailed'))
    }
  } catch {
    ElMessage.error(t('project.exportFailed'))
  }
}

function loadJobTitle() {
  const saved = localStorage.getItem('goresume_settings')
  if (saved) {
    try {
      const settings = JSON.parse(saved)
      if (settings.job?.title) jobTitle.value = settings.job.title
    } catch {}
  }
}

// 点击选择文件 → 调用 Wails 原生文件对话框
async function handleSelectFiles() {
  try {
    const WailsApp = await import('../../wailsjs/go/main/App')
    const count = await WailsApp.SelectResumeFiles(projectId.value || '')
    if (count > 0) {
      ElMessage.success(t('guide.analysisStarted').replace('...', `: ${count} 份`))
    }
  } catch (err: any) {
    console.error('选择文件失败:', err)
  }
}

function handleStartAnalysis() {
  const pendingCount = resumeStore.resumes.filter(r => r.status === 'pending').length
  if (pendingCount === 0) { ElMessage.warning(t('guide.noResumesToAnalyze')); return }
  if (!checkAIConfig()) { showConfigGuide.value = true; return }
  resumeStore.startAnalysis()
}

// 加载解析内容
async function loadParsedContent() {
  if (!resumeStore.selectedId) { parsedContent.value = ''; return }
  contentLoading.value = true
  try {
    parsedContent.value = await resumeStore.getResumeContent(resumeStore.selectedId)
  } catch {
    parsedContent.value = ''
  } finally {
    contentLoading.value = false
  }
}

// 切换到解析内容 Tab 时自动加载
watch(rightTab, (tab) => {
  if (tab === 'content' && resumeStore.selectedId) { loadParsedContent() }
})

// 选中简历变化时，如果在内容 Tab 则刷新
watch(() => resumeStore.selectedId, () => {
  if (rightTab.value === 'content') { loadParsedContent() }
})

async function handleReAnalyze(id: string) {
  if (!checkAIConfig()) { showConfigGuide.value = true; return }
  await resumeStore.reAnalyze(id)
}

async function handleReAnalyzeAll() {
  if (resumeStore.doneCount === 0) { ElMessage.warning(t('home.noAnalyzedResumes')); return }
  if (!checkAIConfig()) { showConfigGuide.value = true; return }
  const ok = await resumeStore.reAnalyzeAll()
  if (!ok) { ElMessage.warning(t('home.noAnalyzedResumes')) }
}

function handleClearAll() { resumeStore.clearAll() }

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function getFileTypeLabel(fileType: string): string {
  const ext = fileType.toLowerCase()
  if (ext === '.pdf') return 'PDF'
  if (ext === '.docx' || ext === '.doc') return 'Word'
  if (['.jpg', '.jpeg', '.png', '.bmp', '.gif', '.webp'].includes(ext)) return t('file.image')
  return t('file.unknown')
}

function getScoreClass(score: number | undefined): string {
  if (!score) return 'low'
  if (score >= 80) return 'high'
  if (score >= 60) return 'medium'
  return 'low'
}

function getProgressColor(value: number): string {
  if (value >= 80) return '#34C759'
  if (value >= 60) return '#FF9500'
  return '#FF3B30'
}

onMounted(async () => {
  loadJobTitle()
  await resumeStore.initWailsEvents()
  
  // 通知后端当前活跃项目（用于原生拖拽时关联项目）
  try {
    const WailsApp = await import('../../wailsjs/go/main/App')
    await WailsApp.SetActiveProject(projectId.value || '')
  } catch {}

  // 如果有项目上下文，加载项目信息和岗位
  if (projectId.value) {
    projectStore.currentProjectId = projectId.value
    await projectStore.refreshProject(projectId.value)
    if (projectStore.currentProject) {
      jobTitle.value = projectStore.currentProject.job_config?.title || jobTitle.value
    }
    // 加载项目下的简历
    await resumeStore.loadProjectResumes(projectId.value)
  }
})

onUnmounted(() => {
  resumeStore.cleanupWailsEvents()
})
</script>

<style scoped lang="scss">
@import '../styles/macos-theme.scss';

// 主容器
.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: $bg-secondary;
  font-family: $font-family;
}

// 标题栏内的操作区
.titlebar-actions {
  display: flex;
  align-items: center;
  gap: 8px;

  .job-badge {
    display: flex;
    align-items: center;
    gap: 5px;
    font-size: 12px;
    color: $text-secondary;
    padding: 3px 10px;
    background: $bg-hover;
    border-radius: 4px;
    border: 1px solid $separator;

    .el-icon { font-size: 12px; color: $gray-1; }
  }

  .toolbar-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: none;
    border-radius: $radius-sm;
    background: transparent;
    color: $text-secondary;
    cursor: pointer;
    transition: background $transition-fast;

    .el-icon { font-size: 16px; }

    &:hover { background: $bg-hover; color: $text-primary; }
  }
}

// 全局分析进度条
.global-progress {
  padding: 8px 18px;
  background: $bg-primary;
  border-bottom: 1px solid $separator;

  .progress-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 6px;
  }

  .progress-text {
    font-size: 12px;
    font-weight: 500;
    color: $system-blue;
  }

  .progress-pct {
    font-size: 11px;
    font-weight: 600;
    color: $text-secondary;
  }

  .progress-track {
    height: 3px;
    background: rgba(0, 122, 255, 0.1);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: $system-blue;
    border-radius: 2px;
    transition: width 0.5s ease;
  }
}

.progress-slide-enter-active, .progress-slide-leave-active {
  transition: all 0.3s ease;
}
.progress-slide-enter-from, .progress-slide-leave-to {
  opacity: 0;
  max-height: 0;
  padding-top: 0;
  padding-bottom: 0;
}

// 主体
.app-main {
  display: flex;
  flex: 1;
  overflow: hidden;
}

// 左侧面板
.left-panel {
  width: 380px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid $separator;
  background: $bg-primary;
  flex-shrink: 0;

  .resume-section {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
}

// 右侧面板
.right-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: $bg-primary;
}

// 区块头部
.section-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  border-bottom: 1px solid $separator;
  background: rgba(246, 246, 246, 0.5);

  .section-label {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 600;
    color: $text-primary;

    .el-icon { color: $gray-1; font-size: 15px; }
  }

  .section-btns {
    display: flex;
    gap: 6px;
  }
}

// macOS 风格操作按钮
.action-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  border: 1px solid $gray-4;
  border-radius: $radius-sm;
  background: $bg-primary;
  color: $text-primary;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all $transition-fast;
  font-family: $font-family;

  .el-icon { font-size: 13px; }

  &:hover { background: $bg-hover; border-color: $gray-3; }
  &:active { transform: scale(0.97); }

  &.primary {
    background: $system-blue;
    border-color: $system-blue;
    color: white;

    &:hover { background: $system-blue-hover; }
    &:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }
  }

  &.boss-btn {
    background: linear-gradient(135deg, #00b96b 0%, #059669 100%);
    border-color: #059669;
    color: #ffffff;
    font-weight: 600;
    box-shadow: 0 2px 6px rgba(0, 185, 107, 0.25);

    &:hover {
      background: linear-gradient(135deg, #059669 0%, #047857 100%);
      transform: translateY(-1px);
      box-shadow: 0 4px 10px rgba(0, 185, 107, 0.35);
    }
    &:active {
      transform: translateY(0);
    }
  }

  &.continue-btn {
    background: linear-gradient(135deg, #f59e0b 0%, #d97706 100%);
    border-color: #d97706;
    color: #ffffff;
    font-weight: 600;
    box-shadow: 0 2px 6px rgba(245, 158, 11, 0.25);

    &:hover:not(:disabled) {
      background: linear-gradient(135deg, #d97706 0%, #b45309 100%);
      transform: translateY(-1px);
      box-shadow: 0 4px 10px rgba(245, 158, 11, 0.35);
    }
    &:active {
      transform: translateY(0);
    }
    &:disabled {
      opacity: 0.45;
      cursor: not-allowed;
      transform: none;
    }
  }
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

// 简历列表
.resume-list {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
  background: $bg-secondary;

  &::-webkit-scrollbar { width: 6px; }
  &::-webkit-scrollbar-track { background: transparent; }
  &::-webkit-scrollbar-thumb { background: rgba(0,0,0,0.12); border-radius: 3px; }

  :deep(.el-empty) {
    padding: 40px 20px;
    .el-empty__description { color: $text-tertiary; font-size: 13px; }
  }
}

// 预览区
.preview-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.preview-content {
  flex: 1;
  overflow: auto;
  padding: 20px;
  background: $bg-secondary;

  :deep(.el-empty) {
    padding: 60px 20px;
    .el-empty__description { color: $text-tertiary; }
  }
}

// 简历详情
.resume-detail {
  .detail-header {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 16px 20px;
    background: $bg-primary;
    border-radius: $radius-lg;
    margin-bottom: 14px;
    border: 1px solid $separator;

    .file-icon { font-size: 36px; color: $system-blue; }

    .file-info {
      h3 { margin: 0 0 3px 0; font-size: 15px; font-weight: 600; color: $text-primary; }
      .file-meta { font-size: 12px; color: $text-secondary; }
    }
  }

  .analysis-result {
    .score-section {
      display: flex;
      gap: 20px;
      padding: 20px;
      background: $bg-primary;
      border-radius: $radius-lg;
      margin-bottom: 14px;
      border: 1px solid $separator;

      .main-score {
        width: 90px;
        height: 90px;
        border-radius: 50%;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        color: white;
        flex-shrink: 0;

        &.high { background: linear-gradient(135deg, $system-green, darken($system-green, 10%)); }
        &.medium { background: linear-gradient(135deg, $system-orange, darken($system-orange, 10%)); }
        &.low { background: linear-gradient(135deg, $system-red, darken($system-red, 10%)); }

        .score-value { font-size: 28px; font-weight: 700; line-height: 1; animation: scorePopIn 0.5s ease-out; }
        .score-label { font-size: 10px; opacity: 0.9; margin-top: 3px; }
      }

      .score-breakdown {
        flex: 1;
        display: flex;
        flex-direction: column;
        justify-content: center;
        gap: 10px;

        .score-item {
          .label { display: block; font-size: 12px; color: $text-secondary; margin-bottom: 3px; }
          :deep(.el-progress__text) { font-size: 11px; font-weight: 600; }
        }
      }
    }

    .recommendation-section {
      text-align: center;
      margin-bottom: 14px;

      .recommendation-badge {
        display: inline-block;
        padding: 8px 20px;
        border-radius: 16px;
        font-size: 13px;
        font-weight: 600;

        &.strong_recommend { background: linear-gradient(135deg, $system-green, darken($system-green, 10%)); color: white; }
        &.recommend { background: rgba(52, 199, 89, 0.12); color: $system-green; }
        &.consider { background: rgba(255, 149, 0, 0.12); color: $system-orange; }
        &.not_recommend { background: rgba(255, 59, 48, 0.12); color: $system-red; }
      }
    }

    .strength-weakness {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 14px;
      margin-bottom: 14px;

      .sw-block {
        padding: 16px;
        border-radius: $radius-lg;
        border: 1px solid $separator;

        h4 { display: flex; align-items: center; gap: 6px; margin: 0 0 10px 0; font-size: 13px; font-weight: 600; }
        ul { margin: 0; padding-left: 18px; }
        li { font-size: 12px; color: $text-secondary; margin-bottom: 5px; &:last-child { margin-bottom: 0; } }

        &.strengths { background: rgba(52, 199, 89, 0.06); h4 { color: $system-green; } }
        &.weaknesses { background: rgba(255, 149, 0, 0.06); h4 { color: $system-orange; } }
      }
    }

    // 候选人信息卡片
    .candidate-card {
      padding: 14px 18px;
      background: linear-gradient(135deg, rgba(0,122,255,0.06), rgba(0,122,255,0.02));
      border: 1px solid rgba(0,122,255,0.15);
      border-radius: $radius-lg;
      margin-bottom: 14px;

      .candidate-name { font-size: 16px; font-weight: 700; color: $text-primary; margin-bottom: 6px; }
      .candidate-meta {
        display: flex; flex-wrap: wrap; gap: 8px;
        span {
          font-size: 12px; color: $text-secondary;
          padding: 2px 8px; background: rgba(0,0,0,0.04); border-radius: 4px;
        }
      }
    }

    // 评分详解
    .detail-cards {
      display: flex; flex-direction: column; gap: 10px; margin-bottom: 14px;

      .detail-card {
        padding: 14px 16px;
        background: $bg-primary;
        border: 1px solid $separator;
        border-radius: $radius-lg;

        h4 { margin: 0 0 8px 0; font-size: 13px; font-weight: 600; color: $text-primary; }
        p { margin: 0; font-size: 12px; color: $text-secondary; line-height: 1.7; }
      }
    }

    // 风险提示
    .risk-section {
      padding: 14px 16px;
      background: rgba(255, 149, 0, 0.04);
      border: 1px solid rgba(255, 149, 0, 0.15);
      border-radius: $radius-lg;
      margin-bottom: 14px;

      h4 { display: flex; align-items: center; gap: 6px; margin: 0 0 10px 0; font-size: 13px; font-weight: 600; color: $system-orange; }
      ul { margin: 0; padding-left: 18px; }
      li { font-size: 12px; color: $text-secondary; margin-bottom: 5px; line-height: 1.6; }
    }

    // 面试建议
    .interview-section {
      padding: 14px 16px;
      background: rgba(0, 122, 255, 0.04);
      border: 1px solid rgba(0, 122, 255, 0.12);
      border-radius: $radius-lg;
      margin-bottom: 14px;

      h4 { display: flex; align-items: center; gap: 6px; margin: 0 0 10px 0; font-size: 13px; font-weight: 600; color: $system-blue; }
      ul { margin: 0; padding-left: 18px; }
      li { font-size: 12px; color: $text-secondary; margin-bottom: 5px; line-height: 1.6; }
    }

    // AI 问答提纲卡片
    .qa-section {
      padding: 16px;
      background: linear-gradient(135deg, rgba(88, 86, 214, 0.04), rgba(0, 122, 255, 0.03));
      border: 1px solid rgba(88, 86, 214, 0.2);
      border-radius: $radius-lg;
      margin-bottom: 14px;

      .qa-section-head {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 12px;

        h4 {
          display: flex;
          align-items: center;
          gap: 6px;
          margin: 0;
          font-size: 13.5px;
          font-weight: 700;
          color: #5856D6;
        }

        .copy-qa-btn {
          display: inline-flex;
          align-items: center;
          gap: 4px;
          padding: 4px 10px;
          font-size: 11.5px;
          font-weight: 500;
          color: $system-blue;
          background: rgba(0, 122, 255, 0.08);
          border: 1px solid rgba(0, 122, 255, 0.2);
          border-radius: 12px;
          cursor: pointer;
          transition: all 0.2s;

          &:hover {
            background: rgba(0, 122, 255, 0.16);
            border-color: $system-blue;
          }
        }
      }

      .qa-list {
        display: flex;
        flex-direction: column;
        gap: 10px;

        .qa-card {
          padding: 12px 14px;
          background: $bg-primary;
          border: 1px solid $separator;
          border-radius: $radius-md;
          box-shadow: 0 1px 3px rgba(0,0,0,0.03);

          .qa-card-header {
            display: flex;
            align-items: flex-start;
            gap: 8px;
            margin-bottom: 8px;

            .qa-tag {
              font-size: 11px;
              font-weight: 700;
              color: #5856D6;
              background: rgba(88, 86, 214, 0.1);
              padding: 1px 5px;
              border-radius: 4px;
              flex-shrink: 0;
            }

            .qa-badge {
              font-size: 10.5px;
              font-weight: 600;
              padding: 1px 6px;
              border-radius: 4px;
              flex-shrink: 0;

              &.cat-tech { background: rgba(0, 122, 255, 0.1); color: $system-blue; }
              &.cat-project { background: rgba(52, 199, 89, 0.1); color: $system-green; }
              &.cat-gap { background: rgba(255, 149, 0, 0.12); color: $system-orange; }
            }

            .qa-question-text {
              font-size: 12.5px;
              font-weight: 600;
              color: $text-primary;
              line-height: 1.5;
            }
          }

          .qa-card-body {
            padding: 8px 10px;
            background: #f8fafc;
            border-radius: 4px;
            border-left: 3px solid $system-blue;

            .qa-answer-title {
              display: flex;
              align-items: center;
              gap: 4px;
              font-size: 11px;
              font-weight: 600;
              color: $text-secondary;
              margin-bottom: 4px;
            }

            .qa-answer-text {
              font-size: 12px;
              color: #4a5568;
              line-height: 1.6;
              white-space: pre-wrap;
            }
          }
        }
      }
    }

    // AI 总结
    .summary-section {
      padding: 16px;
      background: $bg-primary;
      border-radius: $radius-lg;
      border: 1px solid $separator;

      h4 { display: flex; align-items: center; gap: 6px; margin: 0 0 10px 0; font-size: 13px; font-weight: 600; color: $system-blue; }
      p { margin: 0; font-size: 13px; color: $text-secondary; line-height: 1.7; }
    }
  }

  .status-hint {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    text-align: center;

    .el-icon { font-size: 40px; margin-bottom: 12px; }
    span { font-size: 13px; }

    &.pending { .el-icon, span { color: $text-tertiary; } }
    &.analyzing { .el-icon, span { color: $system-blue; } }
    &.error { .el-icon, span { color: $system-red; } }

    &.error-box {
      max-width: 540px;
      margin: 40px auto;
      padding: 24px;
      background: #fff5f5;
      border: 1px solid rgba(255, 59, 48, 0.25);
      border-radius: $radius-lg;
      box-shadow: 0 4px 12px rgba(255, 59, 48, 0.06);

      .error-header {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 12px;

        .error-icon {
          font-size: 24px;
          color: $system-red;
          margin-bottom: 0;
        }

        .error-title {
          font-size: 15px;
          font-weight: 700;
          color: $system-red;
        }
      }

      .error-detail-text {
        font-size: 12px;
        font-family: monospace;
        color: #742a2a;
        background: #fed7d7;
        padding: 10px 12px;
        border-radius: 6px;
        line-height: 1.6;
        word-break: break-all;
        margin-bottom: 16px;
        text-align: left;
        max-height: 160px;
        overflow-y: auto;
      }

      .error-actions {
        display: flex;
        gap: 10px;
        justify-content: center;
      }
    }
  }
}

@keyframes scorePopIn {
  0% { transform: scale(0.5); opacity: 0; }
  50% { transform: scale(1.1); }
  100% { transform: scale(1); opacity: 1; }
}

:deep(.el-progress-bar__inner) {
  transition: width 0.5s ease-out !important;
}

// Tab 栏
.tab-bar {
  display: flex;
  gap: 4px;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  border: none;
  border-radius: $radius-sm;
  background: transparent;
  color: $text-secondary;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  font-family: $font-family;
  transition: all $transition-fast;

  .el-icon { font-size: 13px; }

  &:hover { background: $bg-hover; }

  &.active {
    background: $system-blue-light;
    color: $system-blue;
  }
}

.tab-actions {
  display: flex;
  gap: 6px;
}

// 解析内容面板
.parsed-content-tab {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  background: $bg-secondary;

  .content-loading {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    color: $text-tertiary;

    .el-icon { font-size: 32px; margin-bottom: 10px; color: $system-blue; }
    span { font-size: 13px; }
  }

  .content-empty {
    padding: 40px 20px;
  }

  .content-viewer {
    .content-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 12px 16px;
      background: $bg-primary;
      border: 1px solid $separator;
      border-radius: $radius-lg $radius-lg 0 0;

      .content-filename {
        font-size: 13px;
        font-weight: 600;
        color: $text-primary;
      }

      .content-stats {
        font-size: 11px;
        color: $text-tertiary;
        padding: 2px 8px;
        background: rgba(0,122,255,0.08);
        border-radius: 4px;
        color: $system-blue;
      }
    }

    .content-hint {
      padding: 8px 16px;
      font-size: 11px;
      color: $text-tertiary;
      background: rgba(255, 149, 0, 0.06);
      border-left: 1px solid $separator;
      border-right: 1px solid $separator;
    }

    .content-text {
      margin: 0;
      padding: 16px;
      background: $bg-primary;
      border: 1px solid $separator;
      border-top: none;
      border-radius: 0 0 $radius-lg $radius-lg;
      font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
      font-size: 12px;
      line-height: 1.8;
      color: $text-primary;
      white-space: pre-wrap;
      word-break: break-all;
      max-height: calc(100vh - 260px);
      overflow-y: auto;

      &::-webkit-scrollbar { width: 6px; }
      &::-webkit-scrollbar-track { background: transparent; }
      &::-webkit-scrollbar-thumb { background: rgba(0,0,0,0.12); border-radius: 3px; }
    }
  }
}

// 排名面板
.ranking-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  background: $bg-secondary;
}

.ranking-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ranking-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  background: $bg-primary;
  border: 1px solid $separator;
  border-radius: $radius-lg;
  transition: all $transition-fast;

  &:hover { border-color: $system-blue; box-shadow: $shadow-md; }

  &.top-3 { border-left: 3px solid $system-green; }

  .rank-num {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: 700;
    flex-shrink: 0;

    &.rank-1 { background: linear-gradient(135deg, #FFD700, #FFA500); color: white; }
    &.rank-2 { background: linear-gradient(135deg, #C0C0C0, #A0A0A0); color: white; }
    &.rank-3 { background: linear-gradient(135deg, #CD7F32, #A0522D); color: white; }
    &.rank-other { background: $bg-hover; color: $text-secondary; }
  }

  .rank-info {
    flex: 1;

    .rank-name { font-size: 14px; font-weight: 600; color: $text-primary; }
    .rank-meta {
      display: flex; gap: 8px; margin-top: 2px;
      span { font-size: 11px; color: $text-tertiary; }
    }
  }

  .rank-scores {
    text-align: center;

    .rank-score-main {
      font-size: 20px; font-weight: 700; display: block;
      &.high { color: $system-green; }
      &.medium { color: $system-orange; }
      &.low { color: $system-red; }
    }

    .rank-score-detail {
      display: flex; gap: 8px; margin-top: 2px;
      span { font-size: 10px; color: $text-tertiary; }
    }
  }

  .rank-rec {
    .rec-badge {
      font-size: 11px; font-weight: 500; padding: 3px 10px; border-radius: 12px;
      &.strong_recommend { background: rgba(52,199,89,0.12); color: $system-green; }
      &.recommend { background: rgba(52,199,89,0.08); color: $system-green; }
      &.consider { background: rgba(255,149,0,0.1); color: $system-orange; }
      &.not_recommend { background: rgba(255,59,48,0.1); color: $system-red; }
    }
  }
}

// 头部终审标签与附件信息
.file-title-row {
  display: flex;
  align-items: center;
  gap: 10px;

  .header-merged-tag {
    font-size: 11px;
    font-weight: 700;
    color: #b45309;
    background: #fef3c7;
    border: 1px solid #fde68a;
    padding: 2px 8px;
    border-radius: 12px;
  }
}

.attach-meta {
  color: #0284c7;
  font-weight: 500;
}

.header-attach-btn-wrap {
  margin-left: auto;

  .attach-upload-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
    background: #f0fdf4;
    border: 1px solid #bbf7d0;
    color: #16a34a;
    font-size: 12px;
    font-weight: 600;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s ease;

    &:hover {
      background: #dcfce7;
      border-color: #86efac;
      transform: translateY(-1px);
    }
  }
}

// 红线一票否决警告卡片
.redline-alert-card {
  margin: 12px 0 16px 0;
  padding: 14px 18px;
  background: #fef2f2;
  border: 1.5px solid #f87171;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(220, 38, 38, 0.08);

  .redline-alert-header {
    margin-bottom: 8px;

    .redline-title-group {
      display: flex;
      align-items: flex-start;
      gap: 10px;

      .redline-badge-icon {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        background: #ef4444;
        color: #ffffff;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 16px;
        flex-shrink: 0;
      }

      .redline-title-text {
        display: flex;
        flex-direction: column;
        gap: 2px;

        .redline-title {
          font-size: 13.5px;
          font-weight: 700;
          color: #991b1b;
        }

        .redline-sub {
          font-size: 11.5px;
          color: #b91c1c;
        }
      }
    }
  }

  .redline-violations-list {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 6px;

    .redline-violation-item {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 12px;
      font-weight: 600;
      color: #b91c1c;
      background: #fee2e2;
      padding: 6px 10px;
      border-radius: 6px;
      border-left: 3px solid #dc2626;

      .el-icon {
        color: #dc2626;
        font-size: 14px;
        flex-shrink: 0;
      }
    }
  }
}

// 加分达成看板
.bonus-match-card {
  margin: 12px 0 16px 0;
  padding: 14px 18px;
  background: #fefce8;
  border: 1.5px solid #fde047;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(202, 138, 4, 0.08);

  .bonus-header {
    margin-bottom: 8px;

    .bonus-title-group {
      display: flex;
      align-items: flex-start;
      gap: 10px;

      .bonus-badge-icon {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        background: #eab308;
        color: #ffffff;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 16px;
        flex-shrink: 0;
      }

      .bonus-title-text {
        display: flex;
        flex-direction: column;
        gap: 2px;

        .bonus-title {
          font-size: 13.5px;
          font-weight: 700;
          color: #854d0e;
        }

        .bonus-sub {
          font-size: 11.5px;
          color: #a16207;
        }
      }
    }
  }

  .bonus-matches-list {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 6px;

    .bonus-match-item {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 12px;
      font-weight: 600;
      color: #854d0e;
      background: #fef9c3;
      padding: 6px 10px;
      border-radius: 6px;
      border-left: 3px solid #ca8a04;

      .el-icon {
        color: #ca8a04;
        font-size: 14px;
        flex-shrink: 0;
      }
    }
  }
}

// 初筛 ➔ 终审阶段演进看板
.merged-evolution-card {
  margin: 12px 0 16px 0;
  padding: 14px 18px;
  background: linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%);
  border: 1px solid #fde68a;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(217, 119, 6, 0.08);

  .evo-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;

    .evo-title {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 13px;
      font-weight: 700;
      color: #92400e;
    }

    .evo-scores {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;

      .evo-init-score { color: #64748b; font-weight: 500; }
      .evo-arrow { color: #94a3b8; font-size: 11px; }
      .evo-final-score { color: #b45309; font-weight: 700; }
      .evo-diff {
        font-size: 11px;
        font-weight: 700;
        padding: 2px 8px;
        border-radius: 10px;
        &.up { background: #dcfce7; color: #16a34a; }
        &.down { background: #fee2e2; color: #dc2626; }
      }
    }
  }

  .consistency-box {
    padding: 10px 14px;
    background: #ffffff;
    border-radius: 8px;
    border: 1px solid rgba(217, 119, 6, 0.2);
    margin-bottom: 8px;

    &.consistent { border-left: 4px solid #16a34a; }
    &.warning { border-left: 4px solid #d97706; }
    &.conflict { border-left: 4px solid #dc2626; }

    .consistency-head {
      display: flex;
      align-items: center;
      gap: 10px;
      margin-bottom: 6px;

      .consistency-status-tag {
        font-size: 11px;
        font-weight: 700;
        padding: 2px 8px;
        border-radius: 4px;
        &.consistent { background: #dcfce7; color: #15803d; }
        &.warning { background: #fef3c7; color: #b45309; }
        &.conflict { background: #fee2e2; color: #b91c1c; }
      }

      .consistency-summary {
        font-size: 12px;
        font-weight: 600;
        color: #1e293b;
      }
    }

    .consistency-details {
      list-style: none;
      padding: 0;
      margin: 4px 0 0 0;
      display: flex;
      flex-direction: column;
      gap: 4px;

      li {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 11px;
        color: #475569;
        .el-icon { color: #16a34a; font-size: 12px; }
      }
    }
  }

  .score-reason-box {
    font-size: 11px;
    color: #78350f;
    line-height: 1.5;
    .reason-label { font-weight: 600; }
  }
}

// 候选人在线直达专区
.candidate-portal-bar {
  margin: 14px 0 18px 0;
  padding: 14px 18px;
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.06) 0%, rgba(14, 165, 233, 0.08) 100%);
  border: 1px solid rgba(37, 99, 235, 0.22);
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(37, 99, 235, 0.04);

  .portal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;

    .portal-title {
      font-size: 13.5px;
      font-weight: 700;
      color: #1e3a8a;
      display: inline-flex;
      align-items: center;
      gap: 6px;
    }

    .portal-sub {
      font-size: 11px;
      color: #64748b;
    }
  }

  .portal-actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;

    .portal-act-btn {
      display: inline-flex;
      align-items: center;
      gap: 7px;
      padding: 8px 18px;
      font-size: 12.5px;
      font-weight: 600;
      border-radius: 8px;
      cursor: pointer;
      transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
      border: 1px solid transparent;

      &.primary {
        background: linear-gradient(135deg, #2563eb 0%, #0284c7 100%);
        color: #ffffff;
        box-shadow: 0 3px 10px rgba(37, 99, 235, 0.25);

        &:hover:not(:disabled) {
          background: linear-gradient(135deg, #1d4ed8 0%, #0369a1 100%);
          transform: translateY(-1px);
          box-shadow: 0 5px 14px rgba(37, 99, 235, 0.35);
        }

        &:disabled {
          background: #cbd5e1;
          color: #94a3b8;
          cursor: not-allowed;
          box-shadow: none;
        }
      }

      &.secondary {
        background: #ffffff;
        color: #1e293b;
        border-color: #cbd5e1;

        &:hover:not(:disabled) {
          background: #f8fafc;
          border-color: #94a3b8;
          color: #0f172a;
          transform: translateY(-1px);
        }

        &:disabled {
          background: #f1f5f9;
          color: #cbd5e1;
          border-color: #e2e8f0;
          cursor: not-allowed;
        }
      }
    }

    .portal-empty-tip {
      font-size: 11px;
      color: #94a3b8;
      margin-left: 4px;
    }
  }
}

.candidate-email-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: #eff6ff;
  color: #2563eb;
  border: 1px solid #bfdbfe;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;

  &.disabled {
    background: #f1f5f9;
    color: #94a3b8;
    border-color: #e2e8f0;
    cursor: default;
  }
}

.candidate-url-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: #ecfdf5;
  color: #059669;
  border: 1px solid #a7f3d0;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;

  &:hover {
    background: #d1fae5;
    border-color: #6ee7b7;
    transform: translateY(-1px);
  }
}

.candidate-link-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  background: #f8fafc;
  color: #475569;
  border: 1px solid #cbd5e1;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  font-weight: 500;
  text-decoration: none;
  transition: all 0.2s;

  &:hover {
    background: #0284c7;
    color: #ffffff;
    border-color: #0284c7;
    transform: translateY(-1px);
  }
}

.pitch-card-top-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
  color: #ffffff;
  border: none;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 2px 6px rgba(16, 185, 129, 0.25);
  transition: all 0.2s ease;

  &:hover {
    background: linear-gradient(135deg, #059669 0%, #047857 100%);
    transform: translateY(-1px);
    box-shadow: 0 4px 10px rgba(16, 185, 129, 0.35);
  }
}

.portal-act-btn.highlight {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
  color: #ffffff;
  border: 1px solid #059669;
  font-weight: 600;
  box-shadow: 0 2px 6px rgba(16, 185, 129, 0.25);

  &:hover {
    background: linear-gradient(135deg, #059669 0%, #047857 100%);
    color: #ffffff;
    transform: translateY(-1px);
  }
}

// 🛡️ 简历防伪注水雷达与断层侦测器
.water-radar-card {
  margin: 14px 0 16px 0;
  padding: 16px 18px;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.03);
  transition: all 0.2s ease;

  &.risk-low {
    border-color: #a7f3d0;
    background: linear-gradient(180deg, #f0fdf4 0%, #ffffff 50px);
  }

  &.risk-medium {
    border-color: #fed7aa;
    background: linear-gradient(180deg, #fffbeb 0%, #ffffff 50px);
  }

  &.risk-high {
    border-color: #fecaca;
    background: linear-gradient(180deg, #fef2f2 0%, #ffffff 50px);
  }

  .water-radar-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-bottom: 12px;
    border-bottom: 1px solid rgba(226, 232, 240, 0.8);
    margin-bottom: 14px;

    .radar-title-group {
      display: flex;
      align-items: center;
      gap: 12px;

      .radar-icon-shield {
        width: 38px;
        height: 38px;
        border-radius: 10px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 20px;

        &.low {
          background: #dcfce7;
          color: #15803d;
        }
        &.medium {
          background: #fef3c7;
          color: #d97706;
        }
        &.high {
          background: #fee2e2;
          color: #b91c1c;
        }
      }

      .radar-title-text-wrap {
        display: flex;
        flex-direction: column;
        gap: 3px;

        .radar-main-title {
          display: flex;
          align-items: center;
          gap: 8px;

          .title-bold {
            font-size: 14.5px;
            font-weight: 700;
            color: #0f172a;
          }

          .radar-risk-pill {
            font-size: 11px;
            font-weight: 600;
            padding: 2px 8px;
            border-radius: 9999px;

            &.low {
              background: #dcfce7;
              color: #166534;
              border: 1px solid #86efac;
            }
            &.medium {
              background: #fef3c7;
              color: #92400e;
              border: 1px solid #fde68a;
            }
            &.high {
              background: #fee2e2;
              color: #991b1b;
              border: 1px solid #fca5a5;
            }
          }
        }

        .radar-sub-desc {
          font-size: 11.5px;
          color: #64748b;
        }
      }
    }

    .radar-score-box {
      text-align: right;
      padding: 4px 12px;
      border-radius: 8px;

      &.low { background: rgba(34, 197, 94, 0.08); }
      &.medium { background: rgba(245, 158, 11, 0.08); }
      &.high { background: rgba(239, 68, 68, 0.08); }

      .radar-score-num {
        font-size: 20px;
        font-weight: 800;
        font-family: 'SF Mono', Monaco, Menlo, Consolas, monospace;
        line-height: 1.1;

        & { color: #0f172a; }
      }

      .radar-score-caption {
        font-size: 10px;
        color: #64748b;
        margin-top: 2px;
      }
    }
  }

  .water-radar-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 12px;

    .radar-grid-item {
      background: #f8fafc;
      border: 1px solid #e2e8f0;
      border-radius: 8px;
      padding: 10px 12px;

      .grid-item-title {
        font-size: 12px;
        font-weight: 600;
        color: #334155;
        display: flex;
        align-items: center;
        gap: 6px;
        margin-bottom: 8px;
        padding-bottom: 5px;
        border-bottom: 1px dashed #cbd5e1;

        .el-icon {
          font-size: 13px;
          color: #64748b;
        }
      }

      .grid-item-content {
        font-size: 11.5px;
        line-height: 1.5;

        .safe {
          display: flex;
          align-items: center;
          gap: 6px;
          color: #16a34a;
          font-weight: 500;
          padding: 4px 0;

          .el-icon { font-size: 13px; color: #16a34a; }
        }

        .alert {
          display: flex;
          align-items: flex-start;
          gap: 5px;
          margin-bottom: 5px;
          color: #b91c1c;

          &:last-child { margin-bottom: 0; }
          .gap-dot, .claim-dot { flex-shrink: 0; font-size: 12px; }
          .gap-text, .claim-text { flex: 1; word-break: break-all; }
        }

        .alert-pill {
          display: flex;
          flex-direction: column;
          gap: 2px;
          padding: 6px 8px;
          border-radius: 6px;
          margin-bottom: 6px;

          &:last-child { margin-bottom: 0; }

          &.outsourcing {
            background: #fff7ed;
            border: 1px solid #fed7aa;
            .pill-label { font-weight: 600; color: #c2410c; }
            .pill-msg { color: #9a3412; }
          }

          &.hopping {
            background: #fef2f2;
            border: 1px solid #fecaca;
            .pill-label { font-weight: 600; color: #b91c1c; }
            .pill-msg { color: #991b1b; }
          }
        }

        .stability-safe {
          display: flex;
          align-items: center;
          gap: 6px;
          color: #16a34a;
          font-weight: 500;
          padding: 4px 0;
          .el-icon { font-size: 13px; color: #16a34a; }
        }

        .probe-list {
          display: flex;
          flex-direction: column;
          gap: 6px;

          .probe-item {
            display: flex;
            align-items: flex-start;
            gap: 6px;
            background: #eff6ff;
            border: 1px solid #bfdbfe;
            padding: 6px 8px;
            border-radius: 6px;

            .probe-num {
              flex-shrink: 0;
              font-size: 10px;
              font-weight: 700;
              background: #2563eb;
              color: #ffffff;
              padding: 1px 5px;
              border-radius: 3px;
            }

            .probe-text {
              flex: 1;
              color: #1e3a8a;
              font-size: 11px;
            }
          }
        }
      }
    }
  }
}

// 业务主管极简推介卡弹窗
:deep(.pitch-card-dialog) {
  border-radius: 14px;
  overflow: hidden;

  .el-dialog__header {
    margin: 0;
    padding: 16px 20px;
    background: #f8fafc;
    border-bottom: 1px solid #e2e8f0;

    .el-dialog__title {
      font-size: 15px;
      font-weight: 700;
      color: #0f172a;
    }
  }

  .el-dialog__body {
    padding: 18px 20px;
  }

  .pitch-tip-banner {
    background: #f0fdf4;
    border: 1px solid #bbf7d0;
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 12px;
    color: #166534;
    line-height: 1.5;
    margin-bottom: 14px;
  }

  .pitch-preview-card {
    border: 1px solid #e2e8f0;
    border-radius: 10px;
    overflow: hidden;

    .pitch-preview-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      background: #f8fafc;
      padding: 8px 12px;
      border-bottom: 1px solid #e2e8f0;

      .pitch-tag {
        font-size: 12px;
        font-weight: 600;
        color: #475569;
      }
    }

    .pitch-textarea {
      :deep(textarea) {
        border: none;
        box-shadow: none;
        font-family: 'SF Mono', Monaco, Menlo, Consolas, monospace;
        font-size: 12.5px;
        line-height: 1.6;
        color: #1e293b;
        padding: 12px;
        background: #ffffff;

        &:focus {
          box-shadow: none;
        }
      }
    }
  }

  .pitch-dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
  }
}

// 语言切换器样式
.titlebar-actions {
  :deep(.language-trigger) {
    background: transparent;
    border: 1px solid $gray-4;
    color: $text-primary;
    font-size: 12px;
    padding: 3px 8px;
    border-radius: 4px;

    &:hover { background: $bg-hover; border-color: $gray-3; }
  }
}
</style>
