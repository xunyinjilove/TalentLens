<template>
  <div class="app-container">
    <!-- macOS 风格标题栏 -->
    <TitleBar :title="projectName || $t('app.title')" :show-back="!!projectId">
      <div class="titlebar-actions">
        <span v-if="jobTitle && jobTitle !== projectName" class="job-badge">
          <el-icon><Briefcase /></el-icon>
          {{ jobTitle }}
        </span>
        <LanguageSwitcher />
      </div>
    </TitleBar>

    <!-- 全局分析进度条 -->
    <Transition name="progress-slide">
      <div v-if="resumeStore.isAnalyzing && (resumeStore.batchProgress.total > 0 || resumeStore.resumes.length > 0)" class="global-progress">
        <div class="progress-info">
          <span class="progress-text">
            {{ $t('analysis.batchProgress', { current: resumeStore.batchProgress.current || 0, total: resumeStore.batchProgress.total || resumeStore.resumes.length }) }}
          </span>
          <span class="progress-pct">{{ Math.round(((resumeStore.batchProgress.current || 0) / (resumeStore.batchProgress.total || resumeStore.resumes.length || 1)) * 100) }}%</span>
        </div>
        <div class="progress-track">
          <div class="progress-fill" :style="{ width: (((resumeStore.batchProgress.current || 0) / (resumeStore.batchProgress.total || resumeStore.resumes.length || 1)) * 100) + '%' }"></div>
        </div>
      </div>
    </Transition>

    <el-alert v-if="resumeStore.loadError || routeResumeError" type="warning" :closable="false" show-icon
      style="margin: 8px 20px" :title="resumeStore.loadError || routeResumeError">
      <el-button size="small" @click="retryAssessmentLoad">重新加载测评</el-button>
    </el-alert>

    <main class="app-main">
      <!-- 左侧面板 -->
      <aside
        class="left-panel"
        :class="{ 'is-panel-dragging': isPanelDragging }"
        :style="{ width: leftPanelWidth + 'px' }"
        @dragenter.prevent="handlePanelDragEnter"
        @dragleave.prevent="handlePanelDragLeave"
        @dragover.prevent
        @drop.prevent="handlePanelDrop"
      >
        <!-- 拖拽覆盖层（拖动外部文件悬停在左侧面板时展示） -->
        <Transition name="fade">
          <div v-if="isPanelDragging" class="panel-drag-overlay">
            <el-icon class="drag-overlay-icon"><UploadFilled /></el-icon>
            <div class="drag-overlay-title">松开鼠标即可添加简历</div>
            <div class="drag-overlay-sub">支持 PDF / Word / 图片格式，自动开始分析</div>
          </div>
        </Transition>

        <div class="resume-section">
          <div class="resume-toolbar">
            <!-- 第一行：标题、待分析状态徽章、更多操作下拉菜单 (Q1, Q4) -->
            <div class="resume-toolbar-heading">
              <div class="heading-left">
                <span class="section-label">
                  <el-icon><Document /></el-icon>
                  <span>{{ $t('home.resumeList') }}</span>
                  <span class="count-badge">({{ resumeStore.resumes.length }})</span>
                </span>
                <span v-if="resumeStore.resumes.length > 0 && resumeStore.pendingCount > 0" class="status-pill pending">
                  {{ resumeStore.pendingCount }} 待分析
                </span>
                <span v-else-if="resumeStore.failedCount > 0" class="status-pill pending">{{ resumeStore.failedCount }} 分析失败</span>
                <span v-else-if="resumeStore.resumes.some(r => r.status === 'analyzing')" class="status-pill pending">分析中</span>
                <span v-else-if="resumeStore.resumes.length > 0" class="status-pill done">
                  已全部分析
                </span>
              </div>

              <div class="heading-right">
                <el-dropdown trigger="click" @command="handleResumeMenuCommand">
                  <button class="more-menu-btn" type="button" :aria-label="$t('home.moreActions')" title="更多操作">
                    <el-icon><MoreFilled /></el-icon>
                    <span>更多</span>
                  </button>
                  <template #dropdown>
                    <el-dropdown-menu class="resume-more-dropdown">
                      <el-dropdown-item command="filter51" :disabled="!projectId">
                        <el-icon><Filter /></el-icon>
                        <span>51job 人才管理自动筛选</span>
                      </el-dropdown-item>
                      <el-dropdown-item command="import-local">
                        <el-icon><UploadFilled /></el-icon>
                        <span>导入本地简历（PDF/Word/图片）</span>
                      </el-dropdown-item>
                      <el-dropdown-item command="export" :disabled="resumeStore.doneCount === 0">
                        <el-icon><Download /></el-icon>
                        <span>{{ $t('home.exportReport') }}</span>
                      </el-dropdown-item>
                      <el-dropdown-item
                        command="clear"
                        divided
                        :disabled="!projectId || resumeStore.resumes.length === 0 || resumeStore.isAnalyzing || isClearingProject"
                        class="danger-item"
                      >
                        <el-icon><Delete /></el-icon>
                        <span>{{ $t('home.clearProject') }}</span>
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </div>

            <!-- 第二行：三项核心操作按钮（矩阵寻才、继续寻才、批量分析） -->
            <div class="resume-toolbar-actions">
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
              <button
                class="action-btn primary analysis-action"
                @click="handleStartAnalysis"
                :disabled="resumeStore.isAnalyzing || resumeStore.resumes.length === 0"
                :title="resumeStore.analysisReadyCount === 0 && resumeStore.doneCount > 0 ? '重新分析当前职位分库的全部已完成简历' : ''"
              >
                <el-icon v-if="resumeStore.isAnalyzing" class="spin"><Loading /></el-icon>
                <el-icon v-else-if="resumeStore.analysisReadyCount > 0"><VideoPlay /></el-icon>
                <el-icon v-else><RefreshRight /></el-icon>
                <span v-if="resumeStore.isAnalyzing">{{ (resumeStore.batchProgress.total > 0 || resumeStore.resumes.length > 0) ? `分析中 (${resumeStore.batchProgress.current || 0}/${resumeStore.batchProgress.total || resumeStore.resumes.length})` : $t('home.analyzing') }}</span>
                <span v-else-if="resumeStore.analysisReadyCount > 0">分析 ({{ resumeStore.analysisReadyCount }})</span>
                <span v-else>重新分析全部 ({{ resumeStore.doneCount }})</span>
              </button>
            </div>
          </div>

          <div class="resume-list">
            <ResumeCard
              v-for="(resume, index) in sortedResumes"
              :key="resume.id"
              :resume="resume"
              :sequence="index + 1"
              :selected="resumeStore.selectedId === resume.id"
              :profile-confirmed="hasFixedRadarProfile"
              @click="resumeStore.selectResume(resume.id)"
              @delete="resumeStore.deleteResume"
            />
            <div v-if="resumeStore.resumes.length === 0" class="empty-dropzone-box">
              <DropZone @select-files="handleSelectFiles" />
            </div>
          </div>
        </div>
      </aside>

      <!-- 左右面板拖拽调整手柄 (Splitter Resizer) -->
      <div
        class="panel-resizer"
        :class="{ 'is-resizing': isResizing }"
        @mousedown="startResize"
        @dblclick="resetPanelWidth"
        title="按住鼠标左键拖动调整左右分栏宽度，双击恢复默认宽度"
      >
        <div class="resizer-handle"></div>
      </div>

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
              <button class="action-btn primary" :disabled="!projectId || !rankedResumes.length" @click="showOutreach = true">
                <el-icon><Message /></el-icon>
                批量打招呼
              </button>
              <button class="action-btn" @click="handleExport">
                <el-icon><Document /></el-icon>
                {{ $t('project.exportExcel') }}
              </button>
            </div>
          </div>

          <!-- 详情 Tab -->
          <div v-show="rightTab === 'detail'" class="preview-content">
            <div v-if="resumeStore.selectedResume" class="resume-detail">
              <!-- 1. 常驻决策与画像总览横幅 (Hero Card) -->
              <div class="resume-hero-header">
                <!-- 上排：画像 + 决策看板 (左右分立，绝不重叠挤压) -->
                <div class="hero-top-row">
                  <div class="hero-left-section">
                    <div class="hero-avatar">
                      <span>{{ getCandidateSurname(resumeStore.selectedResume.analysis?.candidateName || resumeStore.selectedResume.fileName) }}</span>
                    </div>
                    <div class="hero-candidate-info">
                      <div class="hero-title-row">
                        <h3 class="hero-candidate-name">
                          {{ resumeStore.selectedResume.analysis?.candidateName || '候选人' }}
                        </h3>
                        <span v-if="resumeStore.selectedResume.is_merged_analysis || resumeStore.selectedResume.has_attachment" class="hero-badge merged">
                          ⭐ 深度终审版
                        </span>
                        <span class="hero-file-name" :title="resumeStore.selectedResume.fileName">
                          {{ resumeStore.selectedResume.fileName }}
                        </span>
                      </div>

                      <div class="hero-meta-row">
                        <span v-if="resumeStore.selectedResume.analysis?.currentRole" class="meta-tag role">
                          <el-icon><Briefcase /></el-icon> {{ resumeStore.selectedResume.analysis.currentRole }}
                        </span>
                        <span v-if="resumeStore.selectedResume.analysis?.workYears" class="meta-tag exp">
                          <el-icon><Calendar /></el-icon> {{ resumeStore.selectedResume.analysis.workYears }}经验
                        </span>
                        <span v-if="resumeStore.selectedResume.analysis?.education" class="meta-tag edu">
                          {{ resumeStore.selectedResume.analysis.education }}
                        </span>
                        <span v-if="resumeStore.selectedResume.source_keyword" class="meta-tag source" title="配额调度派生词条">
                          <el-icon><Filter /></el-icon> 来源词: {{ resumeStore.selectedResume.source_keyword }}
                        </span>
                        <span v-if="resumeStore.selectedResume.activity_label" class="meta-tag activity">
                          <el-icon><Clock /></el-icon> 活跃: {{ resumeStore.selectedResume.activity_label }}
                        </span>
                        <span v-if="resumeStore.selectedResume.email" class="meta-tag email clickable" title="点击复制候选人邮箱" @click="copyCandidateEmail">
                          <el-icon><Message /></el-icon> {{ resumeStore.selectedResume.email }}
                          <el-icon class="copy-sub-icon"><DocumentCopy /></el-icon>
                        </span>
                        <span v-else class="meta-tag email disabled" title="在线直聘平台默认隐私保护">
                          <el-icon><Message /></el-icon> 邮箱未公开
                        </span>
                      </div>
                    </div>
                  </div>

                  <!-- 右上角：综合评分 -->
                  <div v-if="resumeStore.selectedResume.status === 'done' && resumeStore.selectedResume.analysis" class="hero-score-section">
                    <div class="hero-score-box" :class="!resumeStore.selectedResume.analysis.radarStale ? getScoreClass(resumeStore.selectedResume.score) : ''">
                      <strong class="hero-score-num">{{ !resumeStore.selectedResume.analysis.radarStale ? (animatedDetailScore || resumeStore.selectedResume.score) : (resumeStore.selectedResume.analysis.abilityScore ?? resumeStore.selectedResume.score ?? '—') }}</strong>
                      <span class="hero-score-label">{{ resumeStore.selectedResume.analysis.radarStale ? '待复核 · 旧分参考' : '综合评分' }}</span>
                    </div>
                    <div class="hero-decision-badge-wrap">
                      <span class="hero-rec-badge" :class="!resumeStore.selectedResume.analysis.radarStale ? resumeStore.selectedResume.analysis.recommendation : ''">
                        {{ resumeStore.selectedResume.analysis.radarStale ? `待复核 · 原结论：${$t(`analysis.recommendations.${resumeStore.selectedResume.analysis.recommendation || 'consider'}`)}` : $t(`analysis.recommendations.${resumeStore.selectedResume.analysis.recommendation || 'consider'}`) }}
                      </span>
                    </div>
                  </div>
                </div>

                <!-- 下排：快捷操作组与沟通提示 (横向平铺，自适应换行) -->
                <div class="hero-bottom-row">
                  <div class="hero-action-buttons">
                    <button
                      v-if="resumeStore.selectedResume.status === 'done' && resumeStore.selectedResume.analysis"
                      class="hero-act-btn highlight"
                      @click="handleOpenPitchDialog"
                      title="一键生成/复制极简推介卡，推给用人部门业务主管"
                    >
                      <el-icon><Share /></el-icon>
                      <span>推介卡</span>
                    </button>
                    <button
                      class="hero-act-btn secondary"
                      @click="handleAttachResume(resumeStore.selectedResume.id)"
                      :title="resumeStore.selectedResume.has_attachment ? '更新完整附件' : '补充完整附件简历'"
                    >
                      <el-icon><Paperclip /></el-icon>
                      <span>{{ resumeStore.selectedResume.has_attachment ? '更新附件' : '补充附件' }}</span>
                    </button>
                    <button
                      v-if="resumeStore.selectedResume.url"
                      class="hero-act-btn link"
                      @click="handleOpenCandidateUrl"
                      title="在浏览器中直接打开候选人主页"
                    >
                      <el-icon><Compass /></el-icon>
                      <span>主页直达</span>
                    </button>
                    <!-- 51job 快捷打招呼/索要简历 -->
                    <button v-if="resumeStore.selectedResume.snapshot_copy_ids?.length" class="hero-act-btn secondary" @click="showSnapshotHistory">快照历史（{{ resumeStore.selectedResume.snapshot_copy_ids.length + 1 }}）</button>
                    <template v-if="is51JobSource">
                      <button v-if="!is51JobCandidate" class="hero-act-btn secondary" :disabled="contactBusy" @click="ensure51JobCandidateDetail">关联候选人网页</button>
                      <button
                        class="hero-act-btn platform"
                        :disabled="contactBusy || jobPickerVisible || !!resumeStore.selectedResume.contact_status"
                        @click="handle51JobGreeting"
                      >
                        51job打招呼
                      </button>
                      <button
                        class="hero-act-btn platform"
                        :disabled="contactBusy || resumeStore.selectedResume.contact_status !== 'greeted'"
                        @click="handle51JobContact"
                      >
                        索要简历
                      </button>
                      <button
                        class="hero-act-btn platform"
                        :disabled="contactBusy || !is51JobCandidate"
                        @click="check51JobReply(resumeStore.selectedResume.id, true)"
                      >
                        检查回复
                      </button>
                    </template>
                  </div>
                  <div v-if="is51JobSource" class="hero-contact-tip">
                    {{ is51JobCandidate ? contactStatusText : '旧快照缺少可核验的详情链接。请关联此人的 51job 网页后打招呼；完成后可索要完整简历。' }}
                  </div>
                </div>
              </div>

              <!-- 2. 二级 Sub-Tabs 导航栏 -->
              <div v-if="resumeStore.selectedResume.status === 'done' && resumeStore.selectedResume.analysis" class="detail-subtabs-nav">
                <button
                  class="subtab-btn"
                  :class="{ active: detailSubTab === 'summary' }"
                  @click="detailSubTab = 'summary'"
                >
                  <el-icon><Finished /></el-icon>
                  <span>综合评估结论</span>
                </button>

                <button
                  class="subtab-btn"
                  :class="{ active: detailSubTab === 'competency' }"
                  @click="detailSubTab = 'competency'"
                >
                  <el-icon><Histogram /></el-icon>
                  <span>能力维度拆解</span>
                  <span class="subtab-badge info">
                    {{ !resumeStore.selectedResume.analysis.radarStale ? `综合 ${resumeStore.selectedResume.analysis.abilityScore ?? resumeStore.selectedResume.score} 分` : '待复核' }}
                  </span>
                </button>

                <button
                  class="subtab-btn"
                  :class="{ active: detailSubTab === 'interview' }"
                  @click="detailSubTab = 'interview'"
                >
                  <el-icon><ChatLineSquare /></el-icon>
                  <span>初试提纲与记录</span>
                  <span class="subtab-badge count">
                    {{ currentHROutline?.questions?.length || 10 }} 题
                  </span>
                </button>
              </div>

              <!-- 3. 4 大子栏目内容容器 -->
              <div v-if="resumeStore.selectedResume.status === 'done' && resumeStore.selectedResume.analysis" class="detail-subtab-container">
                <!-- ==================== SUB-TAB 1: 综合结论与红线 ==================== -->
                <div v-show="detailSubTab === 'summary'" class="subtab-pane summary-pane">
                  <!-- AI 综合评审卡片 -->
                  <div class="summary-card">
                    <div class="card-head">
                      <div class="card-title">
                        <el-icon><InfoFilled /></el-icon>
                        <span>AI 综合评价总结</span>
                      </div>
                      <span class="file-size-tag">{{ formatFileSize(resumeStore.selectedResume.fileSize) }} · {{ getFileTypeLabel(resumeStore.selectedResume.fileType) }}</span>
                    </div>
                    <p class="summary-body">{{ resumeStore.selectedResume.analysis.summary || '暂无分析总结' }}</p>
                  </div>

                  <!-- 三维匹配度环形图看板 (SVG Donut Rings) -->
                  <div class="match-donuts-row">
                    <SvgDonutRing
                      :percentage="resumeStore.selectedResume.analysis.skillMatch"
                      title="核心技能匹配"
                      subtitle="技术栈与专业度"
                    />
                    <SvgDonutRing
                      :percentage="resumeStore.selectedResume.analysis.experienceMatch"
                      title="工作经验匹配"
                      subtitle="年限与业务场景"
                    />
                    <SvgDonutRing
                      :percentage="resumeStore.selectedResume.analysis.educationMatch"
                      title="学历背景对口"
                      subtitle="院校与专业资质"
                    />
                  </div>

                  <!-- 核心优势与不足并列卡片 -->
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
                    <h4><el-icon><Warning /></el-icon> 潜在风险与注意事项</h4>
                    <ul>
                      <li v-for="(item, idx) in resumeStore.selectedResume.analysis.risks" :key="idx">{{ item }}</li>
                    </ul>
                  </div>
                </div>

                <!-- ==================== SUB-TAB 2: 能力维度拆解 ==================== -->
                <div v-show="detailSubTab === 'competency'" class="subtab-pane competency-pane">
                  <!-- 双源结合分析：初筛 ➔ 终审演进与一致性核验看板（仅深度终审展现） -->
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

                  <!-- 岗位统一技能指标雷达 -->
                  <div class="score-section-card modern-competency-card">
                    <div class="section-card-header">
                      <div class="header-title">
                        <el-icon><Histogram /></el-icon>
                        <span>岗位统一技能指标雷达</span>
                        <span v-if="isGeneratingMetrics" class="ai-generating-tag">
                          <el-icon class="is-loading"><Loading /></el-icon>
                          AI正在自动推导岗位技能画像...
                        </span>
                      </div>
                    </div>

                    <el-alert v-if="resumeStore.selectedResume.analysis.competencyMessage" :title="resumeStore.selectedResume.analysis.competencyMessage" :type="resumeStore.selectedResume.analysis.competencyStatus === 'error' ? 'warning' : 'info'" :closable="false" show-icon style="margin:0 0 16px" />

                    <div class="radar-header-summary">
                      <p class="score-explanation">基于当前岗位配置的【技能要求与核心能力】构建统一雷达坐标，所有候选人以相同标准横向对比评估。</p>
                      <el-button size="small" type="primary" plain :loading="isGeneratingMetrics" @click="openRadarDraft">{{ unifiedJobMetrics.length < 5 ? 'AI 补充岗位雷达指标' : '核对岗位雷达指标' }}</el-button>
                      <el-alert v-if="resumeStore.selectedResume.analysis.radarStale" title="岗位指标已变更：此份旧测评待复核，旧分仅供参考，不参与正式排名。" type="warning" :closable="false" style="margin:12px 0" />
                      <div class="radar-meta-row">
                        <span class="coverage-pill" :class="{ 'is-eligible': isRankEligible }">
                          已知指标覆盖率: <strong>{{ (radarCoverageRatio * 100).toFixed(1) }}%</strong>
                          <span v-if="radarCoverageRatio >= 0.8" class="eligible-badge">（达核实门槛 ≥80%）</span>
                          <span v-else class="eligible-badge">（含待核查项评分，已计入总分与排名）</span>
                        </span>
                        <span class="bounds-text" v-if="radarScoreBounds">技能维度区间: {{ radarScoreBounds }}</span>
                        <span v-if="resumeStore.selectedResume.analysis.holisticMatchScore" class="bounds-text">综合匹配参考：{{ resumeStore.selectedResume.analysis.holisticMatchScore }} 分</span>
                      </div>
                      <p v-if="!resumeStore.selectedResume.analysis.radarStale && missingRadarMetrics.length" class="score-explanation">待核查指标：{{ missingRadarMetrics.join('、') }}</p>
                    </div>

                    <div class="competency-split-layout">
                      <!-- 左侧：原生 SVG 蜘蛛网雷达图 -->
                      <div class="radar-visual-col">
                        <SvgRadarChart
                          v-if="radarDisplayChecks.length >= 3"
                          :indicators="radarIndicators"
                          :candidate-values="radarCandidateValues"
                          :show-benchmark="false"
                          :selected-index="selectedRadarIndex"
                          @select="handleSelectRadarIndex"
                        />
                        <div v-else class="radar-generating-box">
                          <span>{{ radarDisplayChecks.length > 0 ? `当前岗位按 ${radarDisplayChecks.length} 项指标评分；雷达图至少需要 3 项。` : '当前岗位尚无技能指标，点击分析后将自动生成。' }}</span>
                        </div>
                      </div>

                      <!-- 右侧：技能能力与原文线索 -->
                      <div class="score-breakdown-box">
                        <div
                          v-for="(item, cIdx) in activeRadarChecks"
                          :key="item.criterion"
                          class="criterion-item-row clickable"
                          :class="{ highlighted: selectedRadarIndex === cIdx }"
                          @click="handleSelectRadarIndex(cIdx)"
                        >
                          <div class="criterion-name-col">
                            <span class="criterion-name">{{ item.criterion }}</span>
                            <span class="criterion-level-badge" :class="item.isUnknown ? 'level-unknown' : ('level-' + item.level)">
                              {{ item.isUnknown ? `${item.level}/4 · ${item.level > 0 ? '背景线索 (待核查)' : '简历未提及 (待核查)'}` : (item.level + '/4 · ' + (['明确未达标', '相关线索', '参与', '独立完成', '主导并可复盘'][item.level] || '已核实')) }}
                            </span>
                          </div>
                          <div v-if="item.evidence" class="criterion-evidence-col">
                            {{ item.evidence }}
                          </div>
                          <p v-if="item.assessment" class="criterion-assessment">{{ item.assessment }}</p>
                        </div>
                        <p v-if="!activeRadarChecks.length" class="score-explanation empty">
                          暂无岗位技能指标。请先在岗位配置中设置技能要求或核心能力。
                        </p>
                      </div>
                    </div>

                      <!-- 三维进度条 -->
                      <div class="progress-bars-group">
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

                  <!-- 评分详解 (技能、经验、学历文本解析) -->
                  <div v-if="resumeStore.selectedResume.analysis.skillDetail" class="detail-cards-grid">
                    <div class="detail-card">
                      <h4><el-icon><Tickets /></el-icon> {{ $t('analysis.skillMatch') }}详解</h4>
                      <p>{{ resumeStore.selectedResume.analysis.skillDetail }}</p>
                    </div>
                    <div class="detail-card">
                      <h4><el-icon><Calendar /></el-icon> {{ $t('analysis.experienceMatch') }}详解</h4>
                      <p>{{ resumeStore.selectedResume.analysis.experienceDetail }}</p>
                    </div>
                    <div class="detail-card">
                      <h4><el-icon><Document /></el-icon> {{ $t('analysis.educationMatch') }}详解</h4>
                      <p>{{ resumeStore.selectedResume.analysis.educationDetail }}</p>
                    </div>
                  </div>
                </div>

                <!-- ==================== SUB-TAB 3: HR 初试提纲与记录 ==================== -->
                <div v-show="detailSubTab === 'interview'" class="subtab-pane interview-pane">
                  <!-- 本次初试优先关注点 Banner -->
                  <div v-if="currentHROutline?.priority_focus && currentHROutline.priority_focus.length > 0" class="priority-focus-banner">
                    <div class="banner-title">
                      <el-icon><Aim /></el-icon>
                      <span>🎯 本次初试关注点与把关优先级</span>
                    </div>
                    <ul class="focus-list">
                      <li v-for="(f, fIdx) in currentHROutline.priority_focus" :key="fIdx">{{ f }}</li>
                    </ul>
                  </div>

                  <!-- 提纲卡片容器 (始终展示，不被空 interviewQA 隐藏) -->
                  <div class="qa-section-card">
                    <div class="qa-section-head">
                      <div class="qa-title-wrap">
                        <el-icon><QuestionFilled /></el-icon>
                        <span class="qa-head-title">📋 HR 初试提纲与记录 ({{ currentHROutline?.questions?.length || 10 }}题)</span>
                      </div>
                      <div class="qa-actions-row">
                        <button class="copy-qa-btn" @click="handleCopyAllOutline" title="一键复制全套提纲与观察要点">
                          <el-icon><DocumentCopy /></el-icon> 复制全套提纲
                        </button>
                        <button class="export-qa-btn" @click="handleExportInterviewRecords" title="导出已记录的初试原话与判断">
                          <el-icon><Download /></el-icon> 导出初试记录
                        </button>
                        <el-button size="small" type="primary" @click="showInterviewDialog = true">
                          <el-icon><ChatDotRound /></el-icon> 发起初试记录
                        </el-button>
                      </div>
                    </div>

                    <InterviewAccordionCard
                      :outline="currentHROutline"
                      :qa-list="resumeStore.selectedResume?.analysis?.interviewQA"
                      :resume-id="resumeStore.selectedResume?.id"
                      @refresh="loadHROutline"
                    />
                  </div>

                  <!-- 本次初试把关要点与专业交接建议 -->
                  <div class="interview-section-card">
                    <div class="section-card-header">
                      <div class="header-title">
                        <el-icon><ChatLineSquare /></el-icon>
                        <span>本次初试把关要点与专业交接建议</span>
                      </div>
                    </div>
                    <ul class="interview-sug-list">
                      <li><el-icon><CircleCheck /></el-icon><span><strong>意向先行</strong>：优先沟通薪资待遇、工作地点与到岗时间，存在不可调和差异时及时向候选人坦诚说明，节省双方时间。</span></li>
                      <li><el-icon><CircleCheck /></el-icon><span><strong>事例核查</strong>：询问过往工作职责时，要求候选人说明“本人独立负责的环节与交付结果”，区分个人贡献与团队成果。</span></li>
                      <li><el-icon><CircleCheck /></el-icon><span><strong>专业交接</strong>：涉及深度技术原理、代码实现或算法细节的问题，记下线索并在本轮标记待确认，转交下一轮专业/业务面试官深度把关。</span></li>
                      <li><el-icon><CircleCheck /></el-icon><span><strong>客观记录</strong>：尽量记录候选人回答原意与关键事实数据，避免仅凭语气或流畅度直接下断言。</span></li>
                    </ul>
                  </div>
                </div>
              </div>

              <!-- 状态提示 -->
              <div v-else-if="resumeStore.selectedResume.status === 'pending'" class="status-hint pending">
                <el-icon><Clock /></el-icon>
                <span>这份简历待分析。点击下方按钮使用当前配置的大模型生成测评。</span>
                <el-button size="small" type="primary" :disabled="resumeStore.isAnalyzing" @click="handleStartAnalysis">开始分析</el-button>
              </div>
              <div v-else-if="resumeStore.selectedResume.status === 'pending_document'" class="status-hint pending">
                <el-icon><Clock /></el-icon>
                <span>资料待补全：{{ resumeStore.selectedResume.errorMessage || '完整简历或附件尚未取得，暂不分析。' }}</span>
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
                <div class="content-heading">
                  <span class="content-filename">{{ resumeStore.selectedResume.fileName }}</span>
                  <span class="content-stats">{{ $t('home.contentLength', { count: activeRawContent.length }) }}</span>
                  <span v-if="translatedSections.length" class="translation-status-pill">已生成中文对照译文</span>
                </div>
                <div class="content-actions">
                  <el-button
                    v-if="!translatedSections.length"
                    size="small"
                    type="primary"
                    plain
                    :loading="translationBusy"
                    @click="handleTranslateToChinese(false)"
                  >
                    <el-icon><RefreshRight /></el-icon>
                    一键翻译为中文
                  </el-button>
                  <template v-else>
                    <el-button size="small" plain @click="toggleAllTranslations">
                      {{ allTranslationsCollapsed ? '展开全部译文' : '折叠全部译文' }}
                    </el-button>
                    <el-button size="small" plain :loading="translationBusy" @click="handleTranslateToChinese(true)">
                      重新翻译
                    </el-button>
                  </template>
                  <el-button size="small" plain @click="showRawContent = !showRawContent">
                    {{ showRawContent ? '查看分区排版' : '查看完整原文' }}
                  </el-button>
                </div>
              </div>
              <div class="content-hint">
                {{ translatedSections.length ? '已在对应分区模块下方生成中文译文，保留完整原文以便中英核对。' : '以下内容只调整排版，不改写简历原文。' }}
              </div>
              <el-alert v-if="translationError" :title="translationError" type="warning" :closable="true" @close="translationError = ''" />

              <div v-if="showRawContent" class="raw-content-view">
                <pre class="content-text">{{ activeRawContent }}</pre>
              </div>

              <div v-else class="parsed-sections">
                <section v-for="(section, index) in originalParsedSections" :key="index" class="parsed-section">
                  <div class="parsed-section-heading">
                    <span class="parsed-section-number">{{ String(index + 1).padStart(2, '0') }}</span>
                    <h3>{{ section.title }}</h3>
                  </div>
                  <div class="parsed-section-body">
                    <!-- 如果是【在线微简历完整正文】等在线卡片分区，采用高仿真网站卡片UI优化展示 -->
                    <template v-if="isOnlineMicroResumeSection(section.title)">
                      <MicroResumeWebView
                        :raw-content="getSectionRawText(section)"
                        :full-content="activeRawContent"
                        :candidate-name="resumeStore.selectedResume.fileName"
                      />
                    </template>
                    <template v-else>
                      <template v-for="(line, lineIndex) in section.lines" :key="lineIndex">
                        <div v-if="isLineWorkSnapshot(line)" class="work-snapshot">
                          <h4>{{ line.label || '任职履历快照' }}</h4>
                          <div v-if="parseWorkSnapshot(getSnapshotRaw(line)).facts.length" class="snapshot-facts">
                            <span v-for="(fact, factIndex) in parseWorkSnapshot(getSnapshotRaw(line)).facts" :key="factIndex">{{ fact }}</span>
                          </div>
                          <template v-for="(experience, expIndex) in parseWorkSnapshot(getSnapshotRaw(line)).experiences" :key="expIndex">
                            <!-- 原文任职履历小卡片 -->
                            <div class="snapshot-experience">
                              <span class="snapshot-period">{{ experience.period }}</span>
                              <strong v-if="experience.company">{{ experience.company }}</strong>
                              <span v-if="experience.role" class="snapshot-role">{{ experience.role }}</span>
                              <p v-if="experience.details.length">{{ experience.details.join(' · ') }}</p>
                            </div>

                            <!-- 对应小模块正下方紧跟生成的中文翻译 -->
                            <div
                              v-if="getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)"
                              class="snapshot-experience translated-subcard"
                            >
                              <div class="translated-subcard-header">
                                <span class="subcard-badge">
                                  <el-icon><Reading /></el-icon>
                                  <span>中文译文</span>
                                </span>
                                <span v-if="getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)?.period" class="subcard-period">
                                  {{ getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)!.period }}
                                </span>
                              </div>
                              <strong v-if="getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)?.company">
                                {{ getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)!.company }}
                              </strong>
                              <span v-if="getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)?.role" class="snapshot-role">
                                {{ getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)!.role }}
                              </span>
                              <p v-if="getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)?.details?.length">
                                {{ getMatchedTranslatedExperience(index, lineIndex, experience, expIndex)!.details.join(' · ') }}
                              </p>
                            </div>
                          </template>
                        </div>
                        <div v-else-if="line.kind === 'field'" class="parsed-field">
                          <span class="parsed-field-label">{{ line.label }}</span>
                          <span class="parsed-field-value">{{ line.value }}</span>
                        </div>
                        <div v-else-if="line.kind === 'bullet'" class="parsed-bullet"><span aria-hidden="true">•</span>{{ line.text }}</div>
                        <p v-else class="parsed-paragraph">{{ line.text }}</p>
                      </template>
                    </template>
                  </div>

                  <!-- 对应模块下方的中文译文卡片 -->
                  <div v-if="parsedTranslatedSections[index]" class="section-translation-container">
                    <div class="translation-card-header" @click="toggleSectionTranslation(index)">
                      <div class="translation-badge">
                        <el-icon><Reading /></el-icon>
                        <span>中文译文</span>
                        <span class="translated-section-title">【{{ parsedTranslatedSections[index].title }}】</span>
                      </div>
                      <el-button link size="small" type="primary">
                        {{ isTranslationCollapsed(index) ? '展开译文' : '收起译文' }}
                      </el-button>
                    </div>

                    <div v-show="!isTranslationCollapsed(index)" class="translation-card-body">
                      <template v-for="(tLine, tIdx) in parsedTranslatedSections[index].lines" :key="tIdx">
                        <div v-if="isLineWorkSnapshot(tLine)" class="work-snapshot translation-snapshot">
                          <div class="inline-trans-note">
                            <el-icon><CircleCheck /></el-icon>
                            <span>任职履历快照各段经历已在上方对应小卡片正下方生成就地中文对照</span>
                          </div>
                        </div>
                        <div v-else-if="tLine.kind === 'field'" class="parsed-field">
                          <span class="parsed-field-label">{{ tLine.label }}</span>
                          <span class="parsed-field-value">{{ tLine.value }}</span>
                        </div>
                        <div v-else-if="tLine.kind === 'bullet'" class="parsed-bullet"><span aria-hidden="true">•</span>{{ tLine.text }}</div>
                        <p v-else class="parsed-paragraph">{{ tLine.text }}</p>
                      </template>
                    </div>
                  </div>
                </section>
              </div>
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
                  <span v-if="!r.analysis" class="rank-status-pending">待分析</span>
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
      :city="projectStore.currentProject?.job_config?.city || '上海'"
      :job-description="projectStore.currentProject?.job_config?.description || projectStore.currentJobConfig?.description"
      :exp-years="projectStore.currentProject?.job_config?.experience_years"
      :edu-level="projectStore.currentProject?.job_config?.education_level"
      :is-continue="bossDialogContinueMode"
      @refresh="handleRefreshProject"
    />
    <TalentManagementFilterDialog
      v-model="showTalentFilterDialog"
      :project-id="projectId"
      :project-name="projectName"
      :job-title="projectStore.currentProject?.job_config?.title || jobTitle"
      :default-education="projectStore.currentProject?.job_config?.education_level || ''"
      :default-city="projectStore.currentProject?.job_config?.city || ''"
      @completed="handleRefreshProject"
    />
    <AIInterviewDialog v-model="showInterviewDialog" :resume-id="resumeStore.selectedResume?.id || ''" />
    <el-dialog v-model="radarDraftOpen" title="确认岗位统一技能指标" width="min(680px,92vw)">
      <p>这组指标将固定用于当前岗位。确认后，指标不一致的旧测评会标为待复核。</p>
      <el-alert v-if="radarDraftOrigin" :title="radarDraftOrigin === 'ai_inferred' ? 'AI 推定草稿，请核对岗位相关性' : radarDraftOrigin === 'template_inferred' ? '职业模板草稿，请核对岗位相关性' : '现有岗位指标'" type="info" :closable="false" />
      <p v-if="radarDraftNote">{{ radarDraftNote }}</p>
      <el-input v-model="radarDraftText" type="textarea" :rows="8" placeholder="每行一项岗位技能指标，1 至 8 项" />
      <template #footer><el-button @click="radarDraftOpen=false">取消</el-button><el-button type="primary" :loading="isApplyingRadar" @click="applyRadarDraft">确认并启用</el-button></template>
    </el-dialog>
    <el-dialog v-model="replyDialogOpen" title="51job 候选人回复" width="min(640px,92vw)">
      <p>{{ replyResult.message }}</p>
      <p v-if="!replyResult.replies?.length">当前已加载会话中暂无文字回复。</p>
      <article v-for="(text,index) in replyResult.replies || []" :key="index" style="white-space:pre-wrap;overflow-wrap:anywhere;padding:16px;margin:12px 0;border:1px solid var(--wb-border);border-radius:12px">{{ text }}</article>
      <div v-if="replyResult.attachmentNames?.length"><strong>会话中的简历附件</strong><p v-for="(name,index) in replyResult.attachmentNames" :key="index">{{ name }}</p></div>
      <template #footer><el-button type="primary" @click="replyDialogOpen=false">关闭</el-button></template>
    </el-dialog>
    <TopTenOutreachDialog v-model="showOutreach" :project-id="projectId" @updated="resumeStore.loadProjectResumes(projectId)" />
    <el-dialog v-model="snapshotHistoryOpen" title="相同简历快照 · 历史评测" width="min(760px,94vw)">
      <p>相同文本曾被分别采集和评测，模型输出或当时岗位配置可能不同。列表已折叠相同快照；原文件、历史分数及沟通记录保留。</p>
      <el-table :data="snapshotHistory" max-height="420">
        <el-table-column label="档案" min-width="160"><template #default="scope">{{ scope.row.fileName }}<el-tag v-if="scope.row.current" size="small">当前展示</el-tag></template></el-table-column>
        <el-table-column prop="score" label="保存时得分" width="105" />
        <el-table-column prop="status" label="原状态" width="100" />
        <el-table-column prop="analyzedAt" label="评测时间" min-width="170" />
      </el-table>
    </el-dialog>

    <el-dialog v-model="jobPickerVisible" title="选择 51job 沟通职位" width="640px" destroy-on-close>
      <p class="job-picker-hint">招聘项目岗位：{{ projectStore.currentProject?.job_config?.title || '未设置' }}。请从当前 51job 企业账号返回的职位中选择；点数会在下一步单独确认。</p>
      <div class="job-picker-search">
        <el-input v-model="jobPickerKeyword" clearable placeholder="输入职位名称搜索 51job" @keyup.enter="load51JobOptions" />
        <el-button :loading="jobPickerLoading" @click="load51JobOptions">搜索</el-button>
      </div>
      <el-alert v-if="jobPickerError" :title="jobPickerError" type="warning" :closable="false" show-icon />
      <div v-loading="jobPickerLoading" class="job-picker-list">
        <button v-for="job in jobPickerJobs" :key="job.id" type="button" class="job-picker-row"
          :class="{ selected: jobPickerSelectedID === job.id }" @click="jobPickerSelectedID = job.id">
          <span class="job-picker-row-main"><strong>{{ job.title }}</strong><small>ID {{ job.id }}</small></span>
          <span class="job-picker-row-meta">{{ [job.company, job.workArea, job.publishStatus].filter(Boolean).join(' · ') || job.source }}</span>
          <span v-if="job.title === projectStore.currentProject?.job_config?.title" class="job-picker-match">同名岗位</span>
        </button>
        <p v-if="!jobPickerLoading && !jobPickerJobs.length && jobPickerError" class="job-picker-empty">职位列表加载失败，尚不能判断 51job 是否有职位。请点击“搜索”重试。</p>
        <p v-else-if="!jobPickerLoading && !jobPickerJobs.length" class="job-picker-empty">当前没有返回职位。可输入更短的职位关键词搜索，或检查 51job 企业账号的在招职位。</p>
      </div>
      <p v-if="jobPickerHasMore" class="job-picker-more">51job 还有更多职位未列出，请输入职位名称搜索；当前列表不是全部职位。</p>
      <template #footer>
        <el-button @click="jobPickerVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!jobPickerSelectedID || jobPickerLoading" @click="confirm51JobSelectedJob">核对点数并继续</el-button>
      </template>
    </el-dialog>

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
  Setting, Briefcase, Document, VideoPlay, View,
  CircleCheck, Warning, ChatLineSquare, Clock, Loading, CircleClose,
  RefreshRight, MoreFilled, Tickets, QuestionFilled, DocumentCopy, InfoFilled, Search,
  ChatDotRound, DocumentAdd, Connection, CloseBold, Paperclip, Compass, Check, Message,
  Link, Plus, Share, Aim, Filter, Download, Delete, Finished, Histogram, Calendar,
  Reading, UploadFilled
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import TitleBar from '../components/TitleBar.vue'
import DropZone from '../components/DropZone.vue'
import ResumeCard from '../components/ResumeCard.vue'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import AIConfigGuide from '../components/AIConfigGuide.vue'
import DevPanel from '../components/DevPanel.vue'
import BossSearchDialog from '../components/BossSearchDialog.vue'
import TalentManagementFilterDialog from '../components/TalentManagementFilterDialog.vue'
import AIInterviewDialog from '../components/AIInterviewDialog.vue'
import TopTenOutreachDialog from '../components/TopTenOutreachDialog.vue'
import SvgDonutRing from '../components/charts/SvgDonutRing.vue'
import SvgRadarChart from '../components/charts/SvgRadarChart.vue'
import InterviewAccordionCard from '../components/InterviewAccordionCard.vue'
import MicroResumeWebView from '../components/MicroResumeWebView.vue'
import { useResumeStore } from '../composables/useResumeStore'
import { useProjectStore } from '../composables/useProjectStore'
import { runWith51JobBrowser } from '../composables/51jobBrowserRecovery'
import { getCandidateSurname } from '../utils/candidate'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const routeResumeError = ref('')
const resumeStore = useResumeStore()
const projectStore = useProjectStore()

// 项目上下文
const projectId = computed(() => (route.params.id as string) || projectStore.currentProjectId || '')
const projectName = computed(() => projectStore.currentProject?.name || '')
const showOutreach = ref(false)
const snapshotHistoryOpen = ref(false)
const snapshotHistory = ref<any[]>([])
interface RadarCheckItem {
  criterion: string
  level: number
  evidence: string
  assessment: string
  isCovered: boolean
  isUnknown?: boolean
}

const isGeneratingMetrics = ref(false)
const radarDraftOpen = ref(false)
const radarDraftText = ref('')
const radarDraftOrigin = ref('')
const radarDraftNote = ref('')
const radarDraftSourceHash = ref('')
const isApplyingRadar = ref(false)

async function openRadarDraft() {
  if (!projectId.value || isGeneratingMetrics.value) return
  isGeneratingMetrics.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const draft = await api.PreviewProjectJobRadar(projectId.value)
    radarDraftText.value = (draft.metrics || []).join('\n')
    radarDraftOrigin.value = draft.origin || ''
    radarDraftNote.value = draft.note || ''
    radarDraftSourceHash.value = draft.sourceHash || ''
    radarDraftOpen.value = true
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { isGeneratingMetrics.value = false }
}

async function applyRadarDraft() {
  const metrics = [...new Set(radarDraftText.value.split(/\r?\n/).map(s => s.trim()).filter(Boolean))]
  if (metrics.length < 1 || metrics.length > 8) { ElMessage.warning('请确认 1 至 8 项相关指标'); return }
  isApplyingRadar.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.ApplyProjectJobRadar({ projectId: projectId.value, sourceHash: radarDraftSourceHash.value, metrics, origin: radarDraftOrigin.value, note: radarDraftNote.value })
    await projectStore.refreshProject(projectId.value)
    await resumeStore.loadProjectResumes(projectId.value)
    radarDraftOpen.value = false
    ElMessage.success('岗位雷达指标已保存；旧测评如不一致将等待 HR 复核')
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
  finally { isApplyingRadar.value = false }
}

// 基于当前岗位配置的【技能要求 + 核心能力】构建统一雷达指标
const unifiedJobMetrics = computed<string[]>(() => {
  const jobCfg = projectStore.currentProject?.job_config
  const set = new Set<string>()
  if (jobCfg?.radar_metrics?.length) return jobCfg.radar_metrics
  if (jobCfg?.required_skills && Array.isArray(jobCfg.required_skills)) {
    for (const s of jobCfg.required_skills) {
      const trimmed = (s || '').trim()
      if (trimmed) set.add(trimmed)
    }
  }
  if (jobCfg?.core_criteria && Array.isArray(jobCfg.core_criteria)) {
    for (const c of jobCfg.core_criteria) {
      const trimmed = (c || '').trim()
      if (trimmed) set.add(trimmed)
    }
  }
  return Array.from(set)
})

const hasFixedRadarProfile = computed(() => !!projectStore.currentProject?.job_config?.radar_profile_hash)

// 将当前候选人的评估记录对齐到岗位统一指标
const activeRadarChecks = computed<RadarCheckItem[]>(() => {
  const analysis = resumeStore.selectedResume?.analysis
  const metrics = unifiedJobMetrics.value
  if (!analysis) return []
  if (analysis.radarStale) return metrics.map(criterion => ({ criterion, level: 0, evidence: '', assessment: '旧测评使用了不同的岗位指标，待 AI 复核', isCovered: false, isUnknown: true }))

  const expComps: any[] = analysis.experienceCompetencies || []
  const coreChecks: any[] = (analysis as any).coreCriterionChecks || []
  const allExisting: any[] = [...expComps, ...coreChecks]

  return metrics.map(metric => {
    // 优先精确匹配
    let matched = allExisting.find(item => item.criterion === metric || item.criterion?.trim().toLowerCase() === metric.trim().toLowerCase())
    if (matched) {
      const lvl = typeof matched.level === 'number' ? Math.max(0, Math.min(4, matched.level)) : 0
      const hasEv = !!(matched.evidence && matched.evidence.trim())
      const isKnown = matched.evidence_status !== 'unknown' && matched.evidence_status !== 'conflict' && hasEv
      return {
        criterion: metric,
        level: lvl,
        evidence: matched.evidence || '',
        assessment: matched.assessment || (isKnown ? (lvl === 0 ? '经核验明确未达到最低锚点' : '') : '材料未提及该项技能或证据不足'),
        isCovered: true,
        isUnknown: !isKnown
      }
    }

    return {
      criterion: metric,
      level: 0,
      evidence: '',
      assessment: '材料未提及该项技能或证据不足',
      isCovered: true,
      isUnknown: true
    }
  })
})

const missingRadarMetrics = computed(() => activeRadarChecks.value.filter(item => item.isUnknown).map(item => item.criterion))

const radarCoverageRatio = computed(() => {
  const a = resumeStore.selectedResume?.analysis
  if (a?.radarStale) return 0
  if (a && typeof a.radarCoverage === 'number') return a.radarCoverage
  const checks = activeRadarChecks.value
  if (!checks.length) return 0
  const knownCount = checks.filter(c => !c.isUnknown).length
  return knownCount / checks.length
})

const isRankEligible = computed(() => {
  const a = resumeStore.selectedResume?.analysis
  if (!a || a.radarStale) return false
  return true
})

const radarScoreBounds = computed(() => {
  const a = resumeStore.selectedResume?.analysis
  if (a && typeof a.scoreLowerBound === 'number' && typeof a.scoreUpperBound === 'number') {
    return `${a.scoreLowerBound.toFixed(1)} ~ ${a.scoreUpperBound.toFixed(1)} 分`
  }
  return ''
})

const hasHistoricalUncovered = computed(() => false)

// 雷达图渲染前 8 项指标，以保证视觉多边形饱满清晰
const radarDisplayChecks = computed(() => {
  const checks = activeRadarChecks.value
  if (checks.length <= 8) return checks
  return checks.slice(0, 8)
})

const rightTab = ref<'detail' | 'ranking' | 'content'>('detail')
const detailSubTab = ref<'summary' | 'competency' | 'interview'>('summary')

// 蜘蛛网雷达图维度与数值提取
const selectedRadarIndex = ref<number | null>(null)
function handleSelectRadarIndex(idx: number) {
  selectedRadarIndex.value = selectedRadarIndex.value === idx ? null : idx
}

const radarIndicators = computed(() => {
  return radarDisplayChecks.value.map(c => ({ name: c.criterion, max: 4 }))
})

const radarCandidateValues = computed(() => {
  return radarDisplayChecks.value.map(c => c.level)
})

// 左右面板拖拽调整宽度 (Splitter Resizer)
const DEFAULT_LEFT_PANEL_WIDTH = 340
const savedLeftWidth = Number(localStorage.getItem('talentlens_left_panel_width'))
const leftPanelWidth = ref(savedLeftWidth && savedLeftWidth >= 260 && savedLeftWidth <= 600 ? savedLeftWidth : DEFAULT_LEFT_PANEL_WIDTH)
const isResizing = ref(false)

let activeMouseMoveHandler: ((e: MouseEvent) => void) | null = null
let activeMouseUpHandler: (() => void) | null = null

function cleanupResizeListeners() {
  if (activeMouseMoveHandler) {
    window.removeEventListener('mousemove', activeMouseMoveHandler)
    activeMouseMoveHandler = null
  }
  if (activeMouseUpHandler) {
    window.removeEventListener('mouseup', activeMouseUpHandler)
    activeMouseUpHandler = null
  }
  isResizing.value = false
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

function startResize(e: MouseEvent) {
  e.preventDefault()
  cleanupResizeListeners()
  isResizing.value = true
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'

  const startX = e.clientX
  const startWidth = leftPanelWidth.value

  activeMouseMoveHandler = (moveEvent: MouseEvent) => {
    const deltaX = moveEvent.clientX - startX
    const maxWidth = Math.max(380, Math.min(window.innerWidth * 0.55, 620))
    const minWidth = 260
    const newWidth = Math.round(Math.max(minWidth, Math.min(maxWidth, startWidth + deltaX)))
    leftPanelWidth.value = newWidth
  }

  activeMouseUpHandler = () => {
    cleanupResizeListeners()
    localStorage.setItem('talentlens_left_panel_width', String(leftPanelWidth.value))
  }

  window.addEventListener('mousemove', activeMouseMoveHandler)
  window.addEventListener('mouseup', activeMouseUpHandler)
}

function resetPanelWidth() {
  leftPanelWidth.value = DEFAULT_LEFT_PANEL_WIDTH
  localStorage.setItem('talentlens_left_panel_width', String(DEFAULT_LEFT_PANEL_WIDTH))
  ElMessage.info('已恢复默认分栏比例 (340px)')
}

// 待核实红线项数量
const unverifiedRedLineCount = computed(() => {
  const checks = resumeStore.selectedResume?.analysis?.redLineChecks || []
  return checks.filter(c => c.status === 'unknown' || !c.status).length
})

// HR 初试提纲与记录
const currentHROutline = ref<any>(null)
const hrOutlineLoading = ref(false)

async function loadHROutline() {
  const resumeId = resumeStore.selectedResume?.id
  if (!resumeId) {
    currentHROutline.value = null
    return
  }
  hrOutlineLoading.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    currentHROutline.value = await api.GetHROutline(resumeId)
  } catch (e) {
    console.warn('获取初试提纲失败:', e)
  } finally {
    hrOutlineLoading.value = false
  }
}

watch(
  () => resumeStore.selectedResume?.id,
  newId => {
    if (newId) {
      loadHROutline()
    } else {
      currentHROutline.value = null
    }
  },
  { immediate: true }
)

const parsedContent = ref('')
const contentLoading = ref(false)
const showRawContent = ref(false)

function isOnlineMicroResumeSection(title: string): boolean {
  if (!title) return false
  return title.includes('在线微简历') ||
    title.includes('在线卡片') ||
    title.includes('微简历完整正文') ||
    title.includes('推荐牛人档案')
}

function getSectionRawText(section: ParsedSection): string {
  return section.lines.map(line => {
    if (line.kind === 'field') return `${line.label}：${line.value}`
    if (line.kind === 'bullet') return `• ${line.text}`
    return line.text || ''
  }).join('\n')
}
const isChineseView = ref(false)
const translatedSections = ref<string[]>([])
const translationBusy = ref(false)
const translationError = ref('')
let translationRequest = 0

type ParsedLine = { kind: 'field' | 'bullet' | 'text'; text?: string; label?: string; value?: string }
type ParsedSection = { title: string; lines: ParsedLine[] }

function parseContentToSections(content: string): ParsedSection[] {
  const sections: ParsedSection[] = []
  let current: ParsedSection = { title: '基本信息', lines: [] }
  for (const rawLine of content.replace(/\r\n?/g, '\n').split('\n')) {
    const line = rawLine.trim()
    if (!line) continue
    const heading = line.match(/^[【\[]([^】\]]{2,60})[】\]]$/) || line.match(/^#{1,4}\s+(.+)$/)
    if (heading) {
      if (current.lines.length) sections.push(current)
      current = { title: heading[1].trim(), lines: [] }
      continue
    }
    const field = line.match(/^([^：:]{2,20})[：:]\s*(.+)$/)
    if (field) current.lines.push({ kind: 'field', label: field[1].trim(), value: field[2].trim() })
    else if (/^[•●·*\-]\s*/.test(line)) current.lines.push({ kind: 'bullet', text: line.replace(/^[•●·*\-]\s*/, '') })
    else current.lines.push({ kind: 'text', text: line })
  }
  if (current.lines.length) sections.push(current)
  return sections.length ? sections : [{ title: '原文内容', lines: [{ kind: 'text', text: content }] }]
}

function isWorkSnapshot(label?: string, value?: string, text?: string): boolean {
  if (label && (label.includes('任职履历快照') || label.includes('工作经历') || label.includes('履历快照') || label.includes('工作背景') || label.includes('履历') || label.includes('经历') || /work\s*experience/i.test(label) || /experience/i.test(label))) {
    return true
  }
  const candidate = (value || '') + ' ' + (text || '')
  return candidate.includes('|') && /(?:19|20)\d{2}[./-]\d{1,2}/.test(candidate)
}

function isLineWorkSnapshot(line: ParsedLine): boolean {
  if (line.kind === 'field' && isWorkSnapshot(line.label, line.value)) return true
  const raw = line.value || line.text || ''
  return raw.includes('|') && /(?:19|20)\d{2}[./-]\d{1,2}/.test(raw)
}

function getSnapshotRaw(line: ParsedLine): string {
  return line.value || line.text || ''
}

function parseSectionText(text: string, defaultTitle: string): ParsedSection {
  const lines = text.replace(/\r\n?/g, '\n').split('\n')
  let title = defaultTitle
  const parsedLines: ParsedLine[] = []
  let activeSnapshotLine: ParsedLine | null = null

  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line) continue
    const heading = line.match(/^[【\[]([^】\]]{2,60})[】\]]$/) || line.match(/^#{1,4}\s+(.+)$/)
    if (heading && parsedLines.length === 0) {
      title = heading[1].trim()
      continue
    }
    const field = line.match(/^([^：:]{2,20})[：:]\s*(.+)$/)
    if (field) {
      const label = field[1].trim()
      const value = field[2].trim()
      const newLine: ParsedLine = { kind: 'field', label, value }
      parsedLines.push(newLine)
      activeSnapshotLine = isWorkSnapshot(label, value) ? newLine : null
    } else if (activeSnapshotLine && (line.includes('|') || /(?:19|20)\d{2}/.test(line) || /^[•●·*\-o]\s*/.test(line))) {
      activeSnapshotLine.value = (activeSnapshotLine.value ? activeSnapshotLine.value + ' | ' : '') + line
    } else if (line.includes('|') && /(?:19|20)\d{2}[./-]\d{1,2}/.test(line)) {
      activeSnapshotLine = { kind: 'field', label: '任职履历快照', value: line }
      parsedLines.push(activeSnapshotLine)
    } else if (/^[•●·*\-]\s*/.test(line)) {
      activeSnapshotLine = null
      parsedLines.push({ kind: 'bullet', text: line.replace(/^[•●·*\-]\s*/, '') })
    } else {
      activeSnapshotLine = null
      parsedLines.push({ kind: 'text', text: line })
    }
  }
  return { title, lines: parsedLines.length ? parsedLines : [{ kind: 'text', text }] }
}

const originalParsedSections = computed<ParsedSection[]>(() => {
  return parseContentToSections(parsedContent.value)
})

const collapsedTranslations = ref<Record<number, boolean>>({})

function isTranslationCollapsed(index: number): boolean {
  return !!collapsedTranslations.value[index]
}

function toggleSectionTranslation(index: number) {
  collapsedTranslations.value[index] = !collapsedTranslations.value[index]
}

const allTranslationsCollapsed = computed(() => {
  if (!translatedSections.value.length) return false
  return originalParsedSections.value.every((_, idx) => !!collapsedTranslations.value[idx])
})

function toggleAllTranslations() {
  const willCollapse = !allTranslationsCollapsed.value
  const newMap: Record<number, boolean> = {}
  originalParsedSections.value.forEach((_, idx) => {
    newMap[idx] = willCollapse
  })
  collapsedTranslations.value = newMap
}

const parsedTranslatedSections = computed<ParsedSection[]>(() => {
  if (!translatedSections.value.length) return []
  return originalParsedSections.value.map((origSec, idx) => {
    const secText = translatedSections.value[idx] || ''
    if (!secText) return { title: origSec.title, lines: [] }
    const parsed = parseSectionText(secText, origSec.title)
    parsed.title = origSec.title
    return parsed
  })
})

const activeSections = computed<ParsedSection[]>(() => {
  return originalParsedSections.value
})

const activeRawContent = computed<string>(() => {
  if (isChineseView.value && translatedSections.value.length) {
    return translatedSections.value.join('\n\n')
  }
  return parsedContent.value
})

interface SnapshotExperience { period: string; company: string; role: string; details: string[] }
function parseWorkSnapshot(value: string): { facts: string[]; experiences: SnapshotExperience[] } {
  const cleanedValue = value.replace(/\r?\n+/g, ' | ')
  const tokens = cleanedValue.split(/\s*\|\s*/).map(part => {
    let p = part.trim()
    p = p.replace(/^[•●·*\-o]\s*/, '').trim()
    return p
  }).filter(part => part && part !== '•' && part !== '·' && part !== 'o' && part !== '-')
  const facts: string[] = []
  const experiences: SnapshotExperience[] = []
  const periodPattern = /(?:19|20)\d{2}[./-]\d{1,2}\s*(?:[-—–~至]|到)\s*(?:(?:19|20)\d{2}[./-]\d{1,2}|至今|现在|今)/
  const companyPattern = /公司|集团|研究院|实验室|医院|事务所|科技|大学|学院|中心$|控股|电子商务|银行|研究所|生物|医疗|tech|corp|inc|ltd/i
  const rolePattern = /工程师|经理|总监|主管|专员|顾问|负责人|设计师|研究员|助理|分析师|架构师|测试员|开发者|开发|专家|采购员|技术员|代表|manager|lead|director|developer/i
  let current: SnapshotExperience | null = null
  for (const token of tokens) {
    if (periodPattern.test(token)) {
      current = { period: token, company: '', role: '', details: [] }
      experiences.push(current)
    } else if (!current) {
      facts.push(token)
    } else if (!current.company && companyPattern.test(token)) {
      current.company = token
    } else if (!current.role && rolePattern.test(token)) {
      current.role = token
    } else {
      current.details.push(token)
    }
  }
  return { facts, experiences }
}

// 精准匹配任职履历快照中每段工作经历对应的中文译文小模块
function getMatchedTranslatedExperience(
  sectionIndex: number,
  lineIndex: number,
  origExp: SnapshotExperience,
  expIndex: number
): SnapshotExperience | null {
  const transSec = parsedTranslatedSections.value[sectionIndex]
  if (!transSec || !transSec.lines?.length) return null

  // 1. 查找对应的翻译快照行
  let targetTLine: ParsedLine | null = null
  if (transSec.lines[lineIndex] && isLineWorkSnapshot(transSec.lines[lineIndex])) {
    targetTLine = transSec.lines[lineIndex]
  } else {
    targetTLine = transSec.lines.find(l => isLineWorkSnapshot(l)) || null
  }

  if (!targetTLine) return null

  const parsedSnapshot = parseWorkSnapshot(getSnapshotRaw(targetTLine))
  const tExperiences = parsedSnapshot.experiences
  if (!tExperiences || tExperiences.length === 0) {
    const rawText = getSnapshotRaw(targetTLine).trim()
    if (rawText) {
      return {
        period: origExp.period,
        company: origExp.company,
        role: origExp.role,
        details: [rawText]
      }
    }
    return null
  }

  // 2. 匹配规则：优先按时间段 (period) 匹配
  const cleanPeriod = (p: string) => (p || '').replace(/\s+/g, '')
  const origP = cleanPeriod(origExp.period)
  if (origP) {
    const matchedByPeriod = tExperiences.find(te => {
      const tp = cleanPeriod(te.period)
      return tp && (tp === origP || tp.includes(origP) || origP.includes(tp))
    })
    if (matchedByPeriod) return matchedByPeriod
  }

  // 3. 次优匹配：按公司名关键词模糊匹配
  if (origExp.company) {
    const matchedByCompany = tExperiences.find(te => {
      return te.company && (te.company.includes(origExp.company) || origExp.company.includes(te.company))
    })
    if (matchedByCompany) return matchedByCompany
  }

  // 4. 下标对齐兜底
  if (expIndex < tExperiences.length) {
    return tExperiences[expIndex]
  }

  return null
}

function translationSourceSections(): string[] {
  return originalParsedSections.value.map(section => [
    `【${section.title}】`,
    ...section.lines.map(line => {
      if (line.kind === 'field') return `${line.label}：${line.value}`
      if (line.kind === 'bullet') return `• ${line.text}`
      return line.text || ''
    })
  ].join('\n'))
}

async function handleTranslateToChinese(force: boolean) {
  const resumeID = resumeStore.selectedId
  if (!resumeID || !parsedContent.value || translationBusy.value) return
  if (!force && translatedSections.value.length === originalParsedSections.value.length) {
    isChineseView.value = true
    return
  }
  const request = ++translationRequest
  translationBusy.value = true
  translationError.value = ''
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const sections = translationSourceSections()
    const result: string[] = await api.TranslateResumeSections(resumeID, 'zh', force, sections)
    if (request !== translationRequest || resumeStore.selectedId !== resumeID) return
    if (result.length !== sections.length) throw new Error('译文分区与原文不一致，请重新翻译')
    translatedSections.value = result
    isChineseView.value = true
    ElMessage.success('已一键翻译为中文')
  } catch (err: any) {
    if (request === translationRequest) translationError.value = err?.message || String(err)
  } finally {
    if (request === translationRequest) translationBusy.value = false
  }
}
const contactBusy = ref(false)
interface JobCommunicationOption {
  id: string
  title: string
  company?: string
  workArea?: string
  source?: string
  publishStatus?: string
}
const jobPickerVisible = ref(false)
const jobPickerLoading = ref(false)
const jobPickerResumeID = ref('')
const jobPickerKeyword = ref('')
const jobPickerJobs = ref<JobCommunicationOption[]>([])
const jobPickerSelectedID = ref('')
const jobPickerHasMore = ref(false)
const jobPickerError = ref('')
const is51JobCandidate = computed(() => {
  try {
    const u = new URL(resumeStore.selectedResume?.url || '')
    return u.protocol === 'https:' && u.hostname === 'ehire.51job.com' && u.pathname === '/Revision/talent/resume/detail' && /^\d+$/.test(u.searchParams.get('resumeId') || '')
  } catch { return false }
})
const is51JobSource = computed(() => is51JobCandidate.value || resumeStore.selectedResume?.source_platform === '51job' || /【前程无忧】/.test(resumeStore.selectedResume?.fileName || ''))
const contactStatusText = computed(() => ({
  greeted: '已打招呼', resume_requested: '已索要简历；页面打开时后台轮询回复', resume_received: '已收到完整简历'
}[resumeStore.selectedResume?.contact_status || ''] || '尚未联系'))

const jobTitle = ref('高级Go开发工程师')
const showConfigGuide = ref(false)
const showBossDialog = ref(false)
const showTalentFilterDialog = ref(false)
const showInterviewDialog = ref(false)
const isClearingProject = ref(false)
const isAnalysisBusy = computed(() => resumeStore.isAnalyzing || resumeStore.resumes.some(r => r.status === 'analyzing'))

async function handleReviewRedLine(criterion: string, status: 'met' | 'violated' | 'unknown') {
  const resume = resumeStore.selectedResume
  if (!resume) return
  let evidence = ''
  if (status !== 'unknown') {
    try {
      const result = await ElMessageBox.prompt(`请输入核实「${criterion}」的简历原文或初面回答依据`, 'HR 红线复核', { inputValidator: value => !!value?.trim() || '请填写核实依据' })
      evidence = result.value.trim()
    } catch { return }
  }
  try {
    const App = await import('../../wailsjs/go/main/App')
    await App.SetReviewedRedLine(resume.id, criterion, status, evidence)
    if (projectId.value) await resumeStore.loadProjectResumes(projectId.value)
    ElMessage.success('红线核实结果已保存')
  } catch (err: any) { ElMessage.error(`保存红线复核失败：${err.message || err}`) }
}

async function handleReviewBonus(criterion: string, status: 'met' | 'violated' | 'unknown') {
  const resume = resumeStore.selectedResume
  if (!resume) return
  let evidence = ''
  if (status !== 'unknown') {
    try {
      const result = await ElMessageBox.prompt(`请输入人工核实「${criterion}」的事实依据`, 'HR 加分项复核', { inputValidator: value => !!value?.trim() || '请填写核实依据' })
      evidence = result.value.trim()
    } catch { return }
  }
  try {
    const App = await import('../../wailsjs/go/main/App')
    await App.SetReviewedBonus(resume.id, criterion, status, evidence)
    if (projectId.value) await resumeStore.loadProjectResumes(projectId.value)
    ElMessage.success('加分项人工核实结果已保存')
  } catch (err: any) { ElMessage.error(`保存加分项复核失败：${err.message || err}`) }
}

async function handleAddProjectReviewCriterion(kind: 'redline' | 'bonus') {
  if (!projectId.value) return
  const label = kind === 'redline' ? '岗位红线' : '加分项'
  let criterion = ''
  try {
    const result = await ElMessageBox.prompt(`填写适用于当前项目所有候选人的${label}，由 HR 逐人核实`, `添加${label}`, {
      inputPlaceholder: kind === 'redline' ? '例如：独立负责过临床项目' : '例如：有多中心项目管理经验',
      inputValidator: value => !!value?.trim() && value.trim().length <= 120 || '请填写 1–120 字的具体要求'
    })
    criterion = result.value.trim()
  } catch { return }
  try {
    const App = await import('../../wailsjs/go/main/App')
    await App.AddProjectReviewCriterion(projectId.value, kind, criterion)
    await projectStore.refreshProject(projectId.value)
    await resumeStore.loadProjectResumes(projectId.value)
    ElMessage.success(`${label}已加入当前项目，候选人默认待 HR 核实`)
  } catch (err: any) { ElMessage.error(`添加${label}失败：${err.message || err}`) }
}
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
      ElMessage.success('已在招聘专用浏览器中显示候选人主页')
      return
    } catch (e: any) {
      ElMessage.error(`打开浏览器失败: ${e.message || e}`)
      return
    }
  }

  // 开发预览模式回退
  window.open(url, '_blank')
  ElMessage.success('已在新标签页中打开候选人主页！')
}

async function handle51JobGreeting() {
  if (contactBusy.value || jobPickerVisible.value || !await ensure51JobCandidateDetail()) return
  const resume = resumeStore.selectedResume
  if (!resume) return
  jobPickerResumeID.value = resume.id
  jobPickerKeyword.value = ''
  jobPickerJobs.value = []
  jobPickerSelectedID.value = ''
  jobPickerHasMore.value = false
  jobPickerError.value = ''
  jobPickerVisible.value = true
  await load51JobOptions()
}

async function ensure51JobCandidateDetail(): Promise<boolean> {
  if (is51JobCandidate.value) return true
  const resume = resumeStore.selectedResume
  if (!resume || !is51JobSource.value || contactBusy.value) return false
  try {
    const response = await ElMessageBox.prompt('请在 51job 企业工作台找到此人，核对其姓名和经历，再复制包含 resumeId 的新版详情链接。软件会再次核对页面姓名。', '关联候选人网页', { inputPlaceholder: 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=…', confirmButtonText: '已核对此人，关联链接', cancelButtonText: '取消' })
    contactBusy.value = true
    const api: any = await import('../../wailsjs/go/main/App')
    const result = await runWith51JobBrowser(api, () => api.Bind51JobCandidateDetail(resume.id, response.value.trim()), { isActive: () => resumeStore.selectedResume?.id === resume.id })
    if (result.status !== 'verified') { ElMessage.warning(result.message || '关联未完成'); return false }
    await resumeStore.loadProjectResumes(projectId.value)
    ElMessage.success('候选人网页已关联，可继续打招呼')
    return resumeStore.selectedResume?.id === resume.id && is51JobCandidate.value
  } catch (err: any) {
    if (err !== 'cancel' && err !== 'close') ElMessage.error(err?.message || String(err))
    return false
  } finally { contactBusy.value = false }
}

async function showSnapshotHistory() {
  const id = resumeStore.selectedResume?.id
  if (!id) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    snapshotHistory.value = await api.GetResumeSnapshotHistory(id)
    snapshotHistoryOpen.value = true
  } catch (err: any) { ElMessage.error(err?.message || String(err)) }
}

async function load51JobOptions() {
  if (!jobPickerVisible.value || jobPickerLoading.value || !jobPickerResumeID.value) return
  jobPickerLoading.value = true
  jobPickerError.value = ''
  jobPickerJobs.value = []
  jobPickerSelectedID.value = ''
  jobPickerHasMore.value = false
  const requestedKeyword = jobPickerKeyword.value
  const requestedResumeID = jobPickerResumeID.value
  try {
    const App = await import('../../wailsjs/go/main/App')
    const result = await runWith51JobBrowser(App,
      () => App.List51JobCommunicationJobs(requestedResumeID, requestedKeyword),
      { isActive: () => jobPickerVisible.value && jobPickerResumeID.value === requestedResumeID,
        onWaiting: message => { jobPickerError.value = message } })
    if (!jobPickerVisible.value || jobPickerResumeID.value !== requestedResumeID || jobPickerKeyword.value !== requestedKeyword) return
    if (result.status !== 'jobs') {
      jobPickerError.value = result.message || '无法读取 51job 沟通职位'
      return
    }
    jobPickerJobs.value = result.jobs || []
    jobPickerHasMore.value = !!result.hasMore
    const title = projectStore.currentProject?.job_config?.title || ''
    const exact = jobPickerJobs.value.filter(job => job.title === title)
    if (exact.length === 1) jobPickerSelectedID.value = exact[0].id
  } catch (err: any) { jobPickerError.value = `读取 51job 职位失败：${err.message || err}` }
  finally { jobPickerLoading.value = false }
}

async function confirm51JobSelectedJob() {
  const resume = resumeStore.selectedResume
  const job = jobPickerJobs.value.find(item => item.id === jobPickerSelectedID.value)
  if (!resume || resume.id !== jobPickerResumeID.value || !job || contactBusy.value) {
    ElMessage.warning('候选人或沟通职位已变化，请重新选择')
    return
  }
  jobPickerVisible.value = false
  contactBusy.value = true
  try {
    const App = await import('../../wailsjs/go/main/App')
    const preview = await runWith51JobBrowser(App,
      () => App.Prepare51JobHiChatForJob(resume.id, job.id, job.title),
      { isActive: () => resumeStore.selectedResume?.id === resume.id,
        onWaiting: message => { ElMessage.info(message) } })
    if (preview.status !== 'needs_confirmation' || !preview.quoteId) {
      ElMessage.warning({ message: preview.message || '无法核实 51job 职位和点数，未发起Hi聊', duration: 10000, showClose: true })
      return
    }
    const name = resume.analysis?.candidateName || resume.fileName
    try {
      await ElMessageBox.confirm(
        `候选人：${name}。招聘项目岗位：${projectStore.currentProject?.job_config?.title || '未设置'}。HR 选择的 51job 沟通职位：${preview.jobTitle}（ID ${job.id}）。51job 显示本次预计消耗 ${preview.points} 个Hi聊数。确认后软件将点击平台“发起Hi聊”，由 51job 发送平台招呼语；软件不会再额外发送一条自定义招呼。`,
        '确认 51job Hi聊点数',
        { confirmButtonText: '确认消耗并发起Hi聊', cancelButtonText: '取消', type: 'warning' }
      )
    } catch {
      ElMessage.info('已取消，未点击发起Hi聊')
      return
    }
    const result = await App.Confirm51JobHiChat(resume.id, preview.quoteId)
    if (result.status !== 'sent') {
      ElMessage.warning({ message: result.message || '平台未确认发送成功，请先查看聊天记录', duration: 12000, showClose: true })
      return
    }
    if (projectId.value) await resumeStore.loadProjectResumes(projectId.value)
    ElMessage.success(result.message)
  } catch (err: any) { ElMessage.error(`51job 打招呼未确认：${err.message || err}`) }
  finally { contactBusy.value = false }
}

async function handle51JobContact() {
  if (contactBusy.value || !await ensure51JobCandidateDetail()) return
  const resume = resumeStore.selectedResume
  if (!resume || !is51JobCandidate.value || contactBusy.value) return
  const preset = '您好，方便发一份最新的完整简历（PDF 或 Word）吗？收到后我们会结合岗位要求进一步评估。'
  let message = ''
  try {
    const reply = await ElMessageBox.prompt('请确认发给这位候选人的索要简历内容；发送后将在 51job 聊天记录中核验。', '51job 索要简历', {
      inputType: 'textarea', inputValue: preset,
      inputValidator: value => !!value?.trim() && value.trim().length <= 500 || '请输入 1–500 字'
    })
    message = reply.value.trim()
  } catch { return }
  contactBusy.value = true
  try {
    const App = await import('../../wailsjs/go/main/App')
    const result = await runWith51JobBrowser(App,
      () => App.Start51JobConversation(resume.id, 'ask_resume', message),
      { isActive: () => resumeStore.selectedResume?.id === resume.id,
        onWaiting: status => { ElMessage.info(status) } })
    if (result.status === 'needs_confirmation' || result.status === 'needs_job_selection') {
      ElMessage.warning({ message: result.message, duration: 10000, showClose: true })
      return
    }
    if (result.status !== 'sent') throw new Error(result.message || '平台未确认发送成功')
    if (projectId.value) await resumeStore.loadProjectResumes(projectId.value)
    ElMessage.success(result.message)
  } catch (err: any) { ElMessage.error(`51job 沟通未确认：${err.message || err}`) }
  finally { contactBusy.value = false }
}

async function check51JobReply(resumeId: string, interactive = false) {
  if (contactBusy.value) return
  contactBusy.value = true
  try {
    const App = await import('../../wailsjs/go/main/App')
    const result = interactive
      ? await runWith51JobBrowser(App, () => App.Check51JobResumeReply(resumeId),
        { onWaiting: message => { ElMessage.info(message) } })
      : await App.Check51JobResumeReply(resumeId)
    if (interactive && ['reply','waiting','downloaded'].includes(result.status)) {
      replyResult.value=result
      replyDialogOpen.value=true
    }
    if (interactive && projectId.value) await resumeStore.loadProjectResumes(projectId.value)
    if (result.status === 'downloaded') {
      if (projectId.value) await resumeStore.loadProjectResumes(projectId.value)
      ElMessage.success(result.message)
    } else if (interactive) {
      if (result.status === 'waiting' || result.status === 'reply') { /* 展示已核验会话的回复面板 */ }
      else ElMessage.warning(result.message || '尚未取得完整简历')
    }
  } catch (err: any) { if (interactive) ElMessage.error(`检查 51job 回复失败：${err.message || err}`) }
  finally { contactBusy.value = false }
}

const replyDialogOpen=ref(false)
const replyResult=ref<{message?:string; replies?:string[]; attachmentNames?:string[]}>({})
let contactPollTimer: number | null = null
let contactPollCursor = 0
let contactPollRunning = false
async function poll51JobReplies() {
  if (contactBusy.value || contactPollRunning) return
  const pending = resumeStore.resumes.filter(r => r.contact_status === 'resume_requested' && r.url?.startsWith('https://ehire.51job.com/'))
  if (!pending.length) return
  contactPollRunning = true
  try {
    for (let i = 0; i < Math.min(3, pending.length); i++) {
      if (contactBusy.value) break
      const next = pending[contactPollCursor % pending.length]
      contactPollCursor++
      await check51JobReply(next.id)
    }
  } finally { contactPollRunning = false }
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

async function handleCopyAllOutline() {
  const resume = resumeStore.selectedResume
  const outline = currentHROutline.value
  const candidateName = outline?.candidate_name || resume?.analysis?.candidateName || resume?.fileName || '候选人'
  const jobTitle = outline?.job_title || '目标岗位'
  const questions = outline?.questions || []

  if (questions.length === 0) {
    ElMessage.warning('暂无可用提纲')
    return
  }

  const lines = [
    `【HR 初试提纲与记录 · ${jobTitle}】`,
    `候选人：${candidateName}`,
    `推荐题量：共 ${questions.length} 题（工作意向 / 工作能力 / 双向确认）`,
    `========================================`
  ]

  if (outline?.priority_focus && outline.priority_focus.length > 0) {
    lines.push('\n🎯 本次初试优先关注点：')
    outline.priority_focus.forEach((f: string, i: number) => {
      lines.push(`  ${i + 1}. ${f}`)
    })
    lines.push('----------------------------------------')
  }

  questions.forEach((q: any, idx: number) => {
    lines.push(`\nQ${idx + 1}【${q.category_label || q.category} · ${q.topic}】`)
    lines.push(`主问题：${q.question}`)
    lines.push(`为什么问：${q.reason_to_ask}`)
    if (q.source_refs) {
      lines.push(`简历线索：${q.source_refs}`)
    }
    if (q.follow_up_hints && q.follow_up_hints.length > 0) {
      lines.push('条件追问：')
      q.follow_up_hints.forEach((h: string) => lines.push(`  - ${h}`))
    }
    if (q.hr_observe_points && q.hr_observe_points.length > 0) {
      lines.push('重点听什么（HR 观察点）：')
      q.hr_observe_points.forEach((o: string) => lines.push(`  - ${o}`))
    }
    if (q.related_checks && q.related_checks.length > 0) {
      lines.push(`关联岗位要求：${q.related_checks.join('；')}`)
    }
  })

  try {
    await navigator.clipboard.writeText(lines.join('\n'))
    ElMessage.success(`已复制全套 ${questions.length} 道初试题纲与观察要点到剪贴板！`)
  } catch (err) {
    ElMessage.error('复制失败，请手动选取复制')
  }
}

async function handleExportInterviewRecords() {
  const outline = currentHROutline.value
  const candidateName = outline?.candidate_name || '候选人'
  const jobTitle = outline?.job_title || '目标岗位'
  const questions = outline?.questions || []

  const recorded = questions.filter((q: any) => (q.hr_notes && q.hr_notes.trim()) || (q.hr_verdict && q.hr_verdict !== 'unanswered'))
  if (recorded.length === 0) {
    ElMessage.info('当前尚未记录初试回答或判定，请先在下方题目中记录')
    return
  }

  const lines = [
    `【HR 初试现场记录 · ${jobTitle}】`,
    `候选人：${candidateName}`,
    `记录时间：${new Date().toLocaleString()}`,
    `已记录题数：${recorded.length} / ${questions.length} 题`,
    `========================================`
  ]

  recorded.forEach((q: any, i: number) => {
    const verdictLabel = q.hr_verdict === 'pass' ? '✓ 基本一致/有事例' : q.hr_verdict === 'pending' ? '? 待确认/需了解' : q.hr_verdict === 'gap' ? '✕ 存在差异/欠缺' : '未评定'
    lines.push(`\n【Q${q.question_id || i + 1} · ${q.category_label} · ${q.topic}】`)
    lines.push(`考题：${q.question}`)
    lines.push(`HR 判定：${verdictLabel}`)
    lines.push(`现场记录/原话摘要：${q.hr_notes || '（无手写摘要）'}`)
    if (q.saved_at) lines.push(`记录时间：${q.saved_at}`)
  })

  try {
    await navigator.clipboard.writeText(lines.join('\n'))
    ElMessage.success(`已导出 ${recorded.length} 项初试记录至剪贴板，可直接粘贴到初面评估报告中！`)
  } catch (err) {
    ElMessage.error('导出失败，请重试')
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
  if (a?.redLineStatus === 'failed') redLineNotice = '\n🚨 HR 已确认红线不满足'
  else if (a?.redLineStatus === 'pending') redLineNotice = '\n⚠️ 岗位红线待 HR 核实'

  let bonusNotice = ''
  const confirmedBonuses = (a?.bonusChecks || []).filter((check: any) => check.status === 'met' && check.reviewed_at)
  if (confirmedBonuses.length > 0) bonusNotice = `\n⭐ HR 已确认附加优势：${confirmedBonuses.map((check: any) => check.criterion).join('；')}`

  let text = `【候选人极简推介卡】\n👤 候选人：${candidateName} | 现任：${role}\n📌 背景画像：${exp} | ${edu}\n🎯 核心能力：${Math.round(score)}分${redLineNotice}${bonusNotice}\n✨ 核心亮点：\n${topStrengths}\n⚠️ 关注提示：\n  - ${risk}`
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
    if (a.status === 'done' && b.status === 'done') {
      if (!!a.analysis?.radarStale !== !!b.analysis?.radarStale) return a.analysis?.radarStale ? 1 : -1
      return (b.score || 0) - (a.score || 0)
    }
    if (a.status === 'analyzing' && b.status === 'pending') return -1
    if (a.status === 'pending' && b.status === 'analyzing') return 1
    return 0
  })
})

// 排名按综合得分降序排列
const rankedResumes = computed(() => {
  return resumeStore.resumes
    .filter(r => r.status === 'done' && r.analysis && !r.analysis.radarStale)
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

// 拖拽文件进入左侧面板时的交互反馈
const isPanelDragging = ref(false)
let panelDragCounter = 0

function handlePanelDragEnter(e: DragEvent) {
  panelDragCounter++
  if (panelDragCounter > 0) isPanelDragging.value = true
}

function handlePanelDragLeave(e: DragEvent) {
  panelDragCounter--
  if (panelDragCounter <= 0) {
    panelDragCounter = 0
    isPanelDragging.value = false
  }
}

function handlePanelDrop(e: DragEvent) {
  panelDragCounter = 0
  isPanelDragging.value = false
  // Wails 原生 runtime.OnFileDrop 会自动捕获并处理真实文件路径
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

async function handleStartAnalysis() {
  const pendingCount = resumeStore.analysisReadyCount
  if (resumeStore.isAnalyzing || resumeStore.resumes.length === 0) return
  if (!checkAIConfig()) { showConfigGuide.value = true; return }
  if (pendingCount > 0) {
    if (await resumeStore.startAnalysis() && projectId.value) await projectStore.refreshProject(projectId.value)
    return
  }
  const completedCount = resumeStore.doneCount
  if (completedCount === 0) { ElMessage.warning(t('guide.noResumesToAnalyze')); return }
  const started = await resumeStore.reAnalyzeAll()
  if (started && projectId.value) await projectStore.refreshProject(projectId.value)
}

// 加载解析内容
async function loadParsedContent() {
  if (!resumeStore.selectedId) { parsedContent.value = ''; return }
  const requestedId = resumeStore.selectedId
  contentLoading.value = true
  try {
    const content = await resumeStore.getResumeContent(requestedId)
    if (resumeStore.selectedId === requestedId) {
      if (parsedContent.value && parsedContent.value !== content) {
        translationRequest++
        translatedSections.value = []
        collapsedTranslations.value = {}
        isChineseView.value = false
        translationBusy.value = false
      }
      parsedContent.value = content
    }
  } catch {
    if (resumeStore.selectedId === requestedId) parsedContent.value = ''
  } finally {
    if (resumeStore.selectedId === requestedId) contentLoading.value = false
  }
}

// 切换到解析内容 Tab 时自动加载
watch(rightTab, (tab) => {
  if (tab === 'content' && resumeStore.selectedId) { loadParsedContent() }
})

// 选中简历变化时，如果在内容 Tab 则刷新
watch(() => resumeStore.selectedId, () => {
  selectedRadarIndex.value = null
  translationRequest++
  translatedSections.value = []
  collapsedTranslations.value = {}
  isChineseView.value = false
  translationBusy.value = false
  translationError.value = ''
  parsedContent.value = ''
  if (rightTab.value === 'content') { loadParsedContent() }
})

function handleResumeMenuCommand(command: string) {
  if (command === 'filter51') showTalentFilterDialog.value = true
  if (command === 'import-local') void handleSelectFiles()
  if (command === 'export') void handleExport()
  if (command === 'clear') void handleClearProject()
}

async function handleClearProject() {
  const id = projectId.value
  if (!id) return
  if (resumeStore.isAnalyzing) {
    ElMessage.warning('当前项目正在执行批量分析，请等待分析结束后再执行清空')
    return
  }
  if (isClearingProject.value || resumeStore.resumes.length === 0) return
  try {
    await ElMessageBox.confirm(
      t('home.clearProjectDescription', { name: projectName.value || id, count: resumeStore.resumes.length }),
      t('home.clearProjectConfirmTitle'),
      { type: 'warning', confirmButtonText: t('home.clearProjectConfirm'), cancelButtonText: t('home.cancel'), confirmButtonClass: 'el-button--danger' }
    )
  } catch { return }
  if (resumeStore.isAnalyzing || projectId.value !== id) return
  isClearingProject.value = true
  try {
    await resumeStore.clearProjectResumes(id)
    await projectStore.refreshProject(id)
    ElMessage.success(t('home.clearProjectSuccess'))
  } catch (err: any) {
    await resumeStore.loadProjectResumes(id)
    await projectStore.refreshProject(id)
    ElMessage.error(t('home.clearProjectFailed', { error: err.message || err }))
  } finally {
    isClearingProject.value = false
  }
}

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
    // 岗位雷达指标只在 HR 确认草稿后更新。
    // 加载项目下的简历
    await resumeStore.loadProjectResumes(projectId.value)
    selectTargetResumeFromRoute()
  }
  contactPollTimer = window.setInterval(() => { void poll51JobReplies() }, 120000)
})

function selectTargetResumeFromRoute() {
  const targetId = String(route.query.resume || '').trim()
  routeResumeError.value = ''
  if (targetId) {
    const found = resumeStore.resumes.find(r => r.id === targetId)
    if (found) {
      resumeStore.selectResume(found.id)
      rightTab.value = 'detail'
      return
    }
    routeResumeError.value = '这份简历未出现在当前职位测评列表，请核对候选人是否仍在人才库中。'
    return
  }
  if (!resumeStore.selectedId && resumeStore.resumes.length > 0) {
    resumeStore.selectResume(resumeStore.resumes[0].id)
  }
}

async function retryAssessmentLoad() {
  if (!projectId.value) return
  await resumeStore.loadProjectResumes(projectId.value)
  selectTargetResumeFromRoute()
}

watch(() => route.query.resume, (newResumeId) => {
  if (newResumeId) {
    selectTargetResumeFromRoute()
  }
})

onUnmounted(() => {
  cleanupResizeListeners()
  if (contactPollTimer !== null) window.clearInterval(contactPollTimer)
  resumeStore.cleanupWailsEvents()
})
</script>

<style scoped lang="scss">
.criterion-assessment { width: 100%; margin: 6px 0 0; color: var(--wb-muted); font-size: 12px; line-height: 1.6; }
@import '../styles/macos-theme.scss';

.job-picker-hint { margin: 0 0 12px; color: #475569; line-height: 1.6; }
.job-picker-search { display: flex; gap: 10px; margin-bottom: 12px; }
.job-picker-list { min-height: 110px; max-height: 340px; overflow-y: auto; }
.job-picker-row {
  display: block; width: 100%; margin: 0 0 8px; padding: 12px 14px;
  border: 1px solid #dbe3ee; border-radius: 8px; background: #fff;
  color: #1e293b; text-align: left; cursor: pointer;
  &:hover { border-color: #94baf5; background: #f7fbff; }
  &.selected { border-color: #1677ff; background: #edf5ff; box-shadow: inset 0 0 0 1px #1677ff; }
}
.job-picker-row-main { display: flex; justify-content: space-between; gap: 8px; align-items: baseline; }
.job-picker-row-main small { color: #64748b; white-space: nowrap; }
.job-picker-row-meta { display: block; margin-top: 5px; color: #64748b; font-size: 12px; }
.job-picker-match { display: inline-block; margin-top: 6px; color: #0a7e45; font-size: 12px; }
.job-picker-empty, .job-picker-more { color: #64748b; line-height: 1.5; }
.job-picker-more { margin: 10px 0 0; color: #a15c00; }

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
  width: 100%;
  position: relative;
}

// 左侧面板
.left-panel {
  min-width: 240px;
  max-width: 650px;
  display: flex;
  flex-direction: column;
  background: $bg-primary;
  flex-shrink: 0;
  transition: width 0.04s ease;
  position: relative;

  .panel-drag-overlay {
    position: absolute;
    inset: 8px;
    z-index: 50;
    background: rgba(240, 247, 255, 0.95);
    backdrop-filter: blur(4px);
    border: 2px dashed #2563eb;
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: #1e40af;
    pointer-events: none;
    box-shadow: 0 8px 24px rgba(37, 99, 235, 0.15);

    .drag-overlay-icon {
      font-size: 40px;
      color: #2563eb;
    }
    .drag-overlay-title {
      font-size: 15px;
      font-weight: 600;
    }
    .drag-overlay-sub {
      font-size: 12px;
      color: #64748b;
    }
  }

  .resume-section {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
}

.empty-dropzone-box {
  padding: 12px;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

// 左右拖拽手柄
.panel-resizer {
  width: 8px;
  cursor: col-resize;
  position: relative;
  z-index: 20;
  background: transparent;
  user-select: none;
  flex-shrink: 0;
  margin: 0 -4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.15s ease;

  .resizer-handle {
    width: 2px;
    height: 100%;
    background: $separator;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  }

  &:hover, &.is-resizing {
    background: rgba(37, 99, 235, 0.08);

    .resizer-handle {
      width: 4px;
      background: var(--accent-blue, #2563eb);
      box-shadow: 0 0 8px rgba(37, 99, 235, 0.45);
    }
  }
}

// 右侧面板
.right-panel {
  flex: 1;
  min-width: 0; // 核心：允许 flex 弹性缩小，防止子卡片撑爆父容器并被 overflow 裁剪
  display: flex;
  flex-direction: column;
  overflow: hidden;
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

.resume-toolbar {
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 10px 14px;
  border-bottom: 1px solid $separator;
  background: rgba(246, 246, 246, 0.6);
  user-select: none;

  .resume-toolbar-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-width: 0;

    .heading-left {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      flex: 1;

      .section-label {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        flex-shrink: 0;
        color: $text-primary;
        font-size: 13.5px;
        font-weight: 700;
        white-space: nowrap;

        .el-icon {
          color: var(--accent-blue, #2563eb);
          font-size: 15px;
        }

        .count-badge {
          color: $text-secondary;
          font-weight: 600;
          font-size: 12px;
          margin-left: 2px;
        }
      }

      .status-pill {
        display: inline-flex;
        align-items: center;
        padding: 2px 8px;
        border-radius: 10px;
        font-size: 11px;
        font-weight: 600;
        white-space: nowrap;
        flex-shrink: 0;

        &.pending {
          background: rgba(245, 158, 11, 0.12);
          color: #d97706;
          border: 1px solid rgba(245, 158, 11, 0.25);
        }

        &.done {
          background: rgba(16, 185, 129, 0.12);
          color: #059669;
          border: 1px solid rgba(16, 185, 129, 0.25);
        }
      }
    }

    .heading-right {
      flex-shrink: 0;

      .more-menu-btn {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        padding: 4px 10px;
        border-radius: 6px;
        border: 1px solid $gray-4;
        background: var(--bg-card, #ffffff);
        color: $text-secondary;
        font-size: 12px;
        font-weight: 500;
        cursor: pointer;
        transition: all 0.2s ease;
        white-space: nowrap;

        .el-icon {
          font-size: 13px;
        }

        &:hover {
          background: var(--bg-hover, rgba(0, 0, 0, 0.04));
          color: $text-primary;
          border-color: var(--accent-blue, #2563eb);
        }
      }
    }
  }

  .resume-toolbar-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    min-width: 0;

    .action-btn {
      flex: 1 1 0;
      min-width: 0;
      height: 34px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 4px;
      padding: 0 6px;
      border-radius: 8px;
      font-size: 12px;
      font-weight: 600;
      white-space: nowrap;
      cursor: pointer;
      transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
      line-height: 1;

      .el-icon {
        font-size: 13px;
        flex-shrink: 0;
      }

      span {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      &.boss-btn {
        background: linear-gradient(135deg, #00b96b 0%, #059669 100%);
        border: 1px solid #059669;
        color: #ffffff;
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
        border: 1px solid #d97706;
        color: #ffffff;
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
          box-shadow: none;
        }
      }

      &.analysis-action {
        background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
        border: 1px solid #1d4ed8;
        color: #ffffff;
        box-shadow: 0 2px 6px rgba(37, 99, 235, 0.25);

        &:hover:not(:disabled) {
          background: linear-gradient(135deg, #1d4ed8 0%, #1e40af 100%);
          transform: translateY(-1px);
          box-shadow: 0 4px 10px rgba(37, 99, 235, 0.35);
        }

        &:active {
          transform: translateY(0);
        }

        &:disabled {
          background: var(--bg-card-secondary, #f1f5f9);
          border: 1px solid $gray-4;
          color: $text-secondary;
          box-shadow: none;
          cursor: not-allowed;
          opacity: 0.55;

          &:hover {
            transform: none;
            box-shadow: none;
          }
        }
      }
    }
  }
}

:deep(.resume-more-dropdown) {
  .danger-item {
    color: #ef4444 !important;

    &:hover {
      color: #dc2626 !important;
      background: rgba(239, 68, 68, 0.08) !important;
    }
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
  background: var(--bg-app);

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
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.preview-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 16px 20px 32px 18px;
  box-sizing: border-box;
  background: $bg-secondary;

  :deep(.el-empty) {
    padding: 60px 20px;
    .el-empty__description { color: $text-tertiary; }
  }
}

// 简历详情
.resume-detail {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  width: 100%;
  box-sizing: border-box;

  // 1. 常驻决策与画像总览横幅 (Hero Card)
  .resume-hero-header {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 14px 18px;
    background: $bg-primary;
    border-radius: $radius-lg;
    border: 1px solid $separator;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
    min-width: 0;
    width: 100%;
    box-sizing: border-box;

    // 上排：左侧画像 + 右上角评分/决策
    .hero-top-row {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 16px;
      width: 100%;
      min-width: 0;

      .hero-left-section {
        display: flex;
        align-items: flex-start;
        gap: 14px;
        min-width: 0;
        flex: 1;

        .hero-avatar {
          width: 44px;
          height: 44px;
          min-width: 44px;
          border-radius: 12px;
          background: linear-gradient(135deg, $system-blue 0%, #2563eb 100%);
          color: #ffffff;
          font-size: 19px;
          font-weight: 700;
          display: flex;
          align-items: center;
          justify-content: center;
          box-shadow: 0 2px 6px rgba(37, 99, 235, 0.25);
          flex-shrink: 0;
        }

        .hero-candidate-info {
          min-width: 0;
          flex: 1;

          .hero-title-row {
            display: flex;
            align-items: center;
            flex-wrap: wrap;
            gap: 8px;
            margin-bottom: 5px;

            .hero-candidate-name {
              font-size: 17px;
              font-weight: 700;
              color: $text-primary;
              margin: 0;
            }

            .hero-badge.merged {
              padding: 2px 7px;
              border-radius: 6px;
              font-size: 11px;
              font-weight: 600;
              background: #fef3c7;
              color: #b45309;
              border: 1px solid #fde68a;
            }

            .hero-file-name {
              font-size: 12px;
              color: $text-tertiary;
              max-width: 240px;
              overflow: hidden;
              text-overflow: ellipsis;
              white-space: nowrap;
            }
          }

          .hero-meta-row {
            display: flex;
            align-items: center;
            flex-wrap: wrap;
            gap: 6px;

            .meta-tag {
              display: inline-flex;
              align-items: center;
              gap: 4px;
              font-size: 12px;
              color: $text-secondary;
              background: $bg-secondary;
              padding: 2px 7px;
              border-radius: 6px;

              &.email.clickable {
                cursor: pointer;
                transition: color 0.15s ease;
                &:hover {
                  color: $system-blue;
                  background: rgba(37, 99, 235, 0.08);
                }
              }

              &.email.disabled {
                opacity: 0.65;
              }

              .copy-sub-icon {
                font-size: 11px;
                margin-left: 2px;
              }
            }
          }
        }
      }

      // 右侧评分区：固定不缩小，永远醒目
      .hero-score-section {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 4px 14px;
        background: rgba(248, 250, 252, 0.9);
        border: 1px solid $separator;
        border-radius: 10px;
        flex-shrink: 0;

        .hero-score-box {
          width: 58px;
          min-width: 58px;
          padding: 4px 2px;
          border-radius: 8px;
          text-align: center;
          background: #f1f5f9;
          color: #475569;

          &.high { background: #ecfdf5; color: #047857; }
          &.medium { background: #fff7ed; color: #c2410c; }
          &.low { background: #fef2f2; color: #b91c1c; }

          .hero-score-num {
            display: block;
            font-size: 23px;
            font-weight: 800;
            line-height: 1.1;
          }

          .hero-score-label {
            font-size: 10px;
            opacity: 0.85;
          }
        }

        .hero-decision-badge-wrap {
          display: flex;
          flex-direction: column;
          gap: 4px;

          .hero-rec-badge {
            display: inline-block;
            padding: 3px 8px;
            border-radius: 6px;
            font-size: 12px;
            font-weight: 700;
            text-align: center;
            &.strong_recommend, &.recommend { background: #ecfdf5; color: #047857; }
            &.consider { background: #fff7ed; color: #c2410c; }
            &.not_recommend { background: #fef2f2; color: #b91c1c; }
          }

          .hero-redline-status {
            font-size: 11px;
            padding: 1px 6px;
            border-radius: 4px;
            text-align: center;
            white-space: nowrap;
            &.failed { color: #ef4444; background: #fee2e2; }
            &.pending { color: #f59e0b; background: #fef3c7; }
            &.met, &.passed { color: #10b981; background: #d1fae5; }
            &.none { color: $text-tertiary; }
          }
        }
      }
    }

    // 下排：快捷操作组与沟通提示
    .hero-bottom-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      flex-wrap: wrap;
      gap: 10px;
      padding-top: 8px;
      border-top: 1px solid rgba(0, 0, 0, 0.05);
      width: 100%;

      .hero-action-buttons {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: 6px;

        .hero-act-btn {
          display: inline-flex;
          align-items: center;
          gap: 4px;
          padding: 6px 11px;
          font-size: 12px;
          font-weight: 600;
          border-radius: 6px;
          border: 1px solid $separator;
          background: $bg-secondary;
          color: $text-primary;
          cursor: pointer;
          transition: all 0.2s ease;

          &:hover {
            background: rgba(0, 0, 0, 0.06);
          }

          &.highlight {
            background: linear-gradient(135deg, #10b981 0%, #059669 100%);
            color: #ffffff;
            border-color: #059669;
            box-shadow: 0 2px 5px rgba(5, 150, 105, 0.25);
            &:hover {
              transform: translateY(-1px);
            }
          }

          &.secondary {
            background: rgba(37, 99, 235, 0.06);
            color: $system-blue;
            border-color: rgba(37, 99, 235, 0.25);
          }

          &.link {
            background: #f8fafc;
            color: #334155;
          }

          &.platform {
            background: #f0fdf4;
            color: #166534;
            border-color: #bbf7d0;
          }

          &:disabled {
            opacity: 0.5;
            cursor: not-allowed;
          }
        }
      }

      .hero-contact-tip {
        font-size: 11px;
        color: $text-tertiary;
        margin-left: auto;
      }
    }
  }

  // 2. 二级 Sub-Tabs 导航栏
  .detail-subtabs-nav {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    background: $bg-primary;
    border-radius: 10px;
    border: 1px solid $separator;
    overflow-x: auto;
    min-width: 0;
    max-width: 100%;
    box-sizing: border-box;

    &::-webkit-scrollbar { height: 4px; }
    &::-webkit-scrollbar-thumb { background: rgba(0, 0, 0, 0.15); border-radius: 2px; }

    .subtab-btn {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 7px 14px;
      border-radius: 7px;
      font-size: 13px;
      font-weight: 600;
      color: $text-secondary;
      background: transparent;
      border: none;
      cursor: pointer;
      transition: all 0.18s ease;
      white-space: nowrap;

      &:hover {
        color: $text-primary;
        background: $bg-secondary;
      }

      &.active {
        color: #ffffff;
        background: linear-gradient(135deg, $system-blue 0%, #1d4ed8 100%);
        box-shadow: 0 2px 6px rgba(37, 99, 235, 0.28);

        .subtab-badge.info,
        .subtab-badge.count {
          background: rgba(255, 255, 255, 0.25);
          color: #ffffff;
        }
      }

      .subtab-badge {
        font-size: 11px;
        padding: 1px 6px;
        border-radius: 8px;
        font-weight: 600;

        &.alert { background: #ef4444; color: #ffffff; }
        &.info { background: rgba(0, 0, 0, 0.06); color: $text-secondary; }
        &.danger { background: #fee2e2; color: #b91c1c; }
        &.warning { background: #fef3c7; color: #b45309; }
        &.safe { background: #d1fae5; color: #047857; }
        &.count { background: rgba(0, 0, 0, 0.06); color: $text-secondary; }
      }
    }
  }

  // 3. 子栏目内容容器与各 Pane 卡片
  .detail-subtab-container {
    .subtab-pane {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    .summary-card {
      padding: 16px;
      background: $bg-primary;
      border-radius: $radius-lg;
      border: 1px solid $separator;
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03);

      .card-head {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 8px;

        .card-title {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 14px;
          font-weight: 700;
          color: $text-primary;
          .el-icon { color: $system-blue; font-size: 16px; }
        }

        .file-size-tag {
          font-size: 11.5px;
          color: $text-tertiary;
        }
      }

      .summary-body {
        margin: 0;
        font-size: 13.5px;
        line-height: 1.7;
        color: $text-primary;
        white-space: pre-wrap;
      }
    }

    .match-donuts-row {
      display: flex;
      align-items: stretch;
      gap: 12px;
      width: 100%;
      margin-bottom: 2px;
      flex-wrap: wrap;
    }

    .score-section-card {
      padding: 16px;
      background: $bg-primary;
      border-radius: $radius-lg;
      border: 1px solid $separator;

      .section-card-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 12px;

        .header-title {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 14px;
          font-weight: 700;
          color: $text-primary;
          .el-icon { color: $system-blue; }

          .ai-generating-tag {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            font-size: 11px;
            font-weight: 500;
            color: $system-blue;
            background: rgba(0, 122, 255, 0.08);
            padding: 2px 8px;
            border-radius: 4px;
            margin-left: 6px;
          }
        }

        .header-tip {
          font-size: 11.5px;
          color: $text-tertiary;
        }
      }

      .competency-split-layout {
        display: flex;
        align-items: flex-start;
        gap: 20px;
        margin-top: 10px;

        @media (max-width: 960px) {
          flex-direction: column;
          align-items: center;
        }

        .radar-visual-col {
          display: flex;
          flex-direction: column;
          align-items: center;
          justify-content: center;
          flex-shrink: 0;
          background: rgba(248, 250, 252, 0.85);
          padding: 14px 16px;
          border-radius: 12px;
          border: 1px solid $separator;
          min-height: 220px;

          .radar-generating-box {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            gap: 10px;
            color: $system-blue;
            font-size: 13px;
            padding: 30px 20px;
            text-align: center;

            .el-icon {
              font-size: 24px;
            }
          }
        }

        .radar-header-summary {
        margin-bottom: 12px;
        .radar-meta-row {
          margin-top: 6px;
          display: flex;
          align-items: center;
          flex-wrap: wrap;
          gap: 12px;
          font-size: 12px;

          .coverage-pill {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 2px 8px;
            border-radius: 4px;
            background: #f1f5f9;
            color: #475569;
            border: 1px solid #e2e8f0;

            &.is-eligible {
              background: #f0fdf4;
              color: #166534;
              border-color: #bbf7d0;
            }

            .eligible-badge {
              color: #15803d;
              font-size: 11px;
            }
            .ineligible-badge {
              color: #b45309;
              font-size: 11px;
            }
          }

          .bounds-text {
            color: #64748b;
            font-size: 12px;
          }
        }
      }

      .score-breakdown-box {
          flex: 1;
          min-width: 0;
          width: 100%;
        }
      }

      .radar-header-summary {
        margin-bottom: 12px;
        .radar-meta-row {
          margin-top: 6px;
          display: flex;
          align-items: center;
          flex-wrap: wrap;
          gap: 12px;
          font-size: 12px;

          .coverage-pill {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 2px 8px;
            border-radius: 4px;
            background: #f1f5f9;
            color: #475569;
            border: 1px solid #e2e8f0;

            &.is-eligible {
              background: #f0fdf4;
              color: #166534;
              border-color: #bbf7d0;
            }

            .eligible-badge {
              color: #15803d;
              font-size: 11px;
            }
            .ineligible-badge {
              color: #b45309;
              font-size: 11px;
            }
          }

          .bounds-text {
            color: #64748b;
            font-size: 12px;
          }
        }
      }

      .score-breakdown-box {
        display: flex;
        flex-direction: column;
        gap: 8px;

        .score-explanation.warning {
          font-size: 12px;
          color: #c2410c;
          background: #fff7ed;
          padding: 6px 10px;
          border-radius: 6px;
          margin: 0 0 6px 0;
        }

        .score-explanation.empty {
          font-size: 12px;
          color: $text-tertiary;
          margin: 0;
        }

        .criterion-item-row {
          display: flex;
          flex-direction: column;
          align-items: stretch;
          gap: 6px;
          padding: 8px 12px;
          border-radius: 6px;
          background: $bg-secondary;
          border: 1px solid transparent;

          &.clickable {
            cursor: pointer;
            transition: all 0.2s ease;

            &:hover {
              background: #f1f5f9;
              border-color: #cbd5e1;
            }

            &.highlighted {
              background: #eff6ff;
              border-color: #3b82f6;
              box-shadow: 0 0 0 1px #3b82f6;
            }
          }

          .criterion-name-col {
            display: flex;
            align-items: center;
            flex-wrap: wrap;
            gap: 8px;
            min-width: 0;
            font-size: 13px;
            font-weight: 600;

            .criterion-level-badge {
              font-size: 11px;
              padding: 1px 7px;
              border-radius: 4px;
              font-weight: 600;

              &.level-4 { background: #ecfdf5; color: #047857; }
              &.level-3 { background: #eff6ff; color: #1d4ed8; }
              &.level-2 { background: #f8fafc; color: #475569; }
              &.level-1, &.level-0 { background: #fef2f2; color: #b91c1c; }
              &.level-unknown { background: #f1f5f9; color: #64748b; border: 1px dashed #cbd5e1; }
            }
          }

          .criterion-evidence-col {
            flex: 1;
            font-size: 12px;
            color: $text-secondary;
            line-height: 1.5;
          }
        }

        .progress-bars-group {
          margin-top: 10px;
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
          gap: 12px;
          padding-top: 10px;
          border-top: 1px dashed $separator;

          .score-item {
            .label {
              display: block;
              font-size: 12px;
              color: $text-secondary;
              margin-bottom: 3px;
            }
          }
        }
      }
    }

    .detail-cards-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 12px;

      .detail-card {
        padding: 14px;
        background: $bg-primary;
        border-radius: $radius-lg;
        border: 1px solid $separator;

        h4 {
          display: flex;
          align-items: center;
          gap: 6px;
          margin: 0 0 8px 0;
          font-size: 13px;
          font-weight: 600;
          color: $text-primary;
          .el-icon { color: $system-blue; }
        }

        p {
          margin: 0;
          font-size: 12.5px;
          line-height: 1.65;
          color: $text-secondary;
        }
      }
    }

    .water-radar-header-bar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 14px;

      .radar-title-group {
        display: flex;
        align-items: center;
        gap: 12px;

        .radar-title-text-wrap {
          .radar-sub-desc {
            font-size: 11.5px;
            color: $text-tertiary;
            margin-top: 2px;
          }
        }
      }
    }

    .priority-focus-banner {
      background: linear-gradient(135deg, rgba(238, 242, 255, 0.9) 0%, rgba(245, 243, 255, 0.9) 100%);
      border: 1px solid #c7d2fe;
      border-radius: $radius-lg;
      padding: 12px 16px;
      margin-bottom: 14px;

      .banner-title {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 13.5px;
        font-weight: 700;
        color: #4338ca;
        margin-bottom: 6px;

        .el-icon {
          font-size: 15px;
        }
      }

      .focus-list {
        margin: 0;
        padding-left: 20px;
        list-style-type: disc;

        li {
          font-size: 12.5px;
          color: #374151;
          line-height: 1.6;
          margin-bottom: 2px;
          &:last-child {
            margin-bottom: 0;
          }
        }
      }
    }

    .qa-section-card {
      padding: 16px;
      background: $bg-primary;
      border-radius: $radius-lg;
      border: 1px solid $separator;

      .qa-section-head {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 12px;
        margin-bottom: 12px;
        flex-wrap: wrap;

        .qa-title-wrap {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 14px;
          font-weight: 700;
          color: #5856D6;
        }

        .qa-actions-row {
          display: flex;
          align-items: center;
          gap: 8px;

          .copy-qa-btn {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 5px 10px;
            font-size: 12px;
            color: $system-blue;
            background: #eff6ff;
            border: 1px solid #dbeafe;
            border-radius: 6px;
            cursor: pointer;
            &:hover { background: #dbeafe; }
          }

          .export-qa-btn {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 5px 10px;
            font-size: 12px;
            color: #4b5563;
            background: #f9fafb;
            border: 1px solid #e5e7eb;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.2s;
            &:hover {
              background: #f3f4f6;
              color: #111827;
            }
          }
        }
      }

      .qa-list-cards {
        display: flex;
        flex-direction: column;
        gap: 10px;

        .qa-card-item {
          padding: 12px 14px;
          background: $bg-secondary;
          border-radius: 8px;
          border: 1px solid $separator;

          .qa-card-header {
            display: flex;
            align-items: baseline;
            gap: 8px;
            margin-bottom: 6px;

            .qa-badge {
              font-size: 11px;
              padding: 1px 6px;
              border-radius: 4px;
              background: #f1f5f9;
              color: #475569;
              white-space: nowrap;
            }

            .qa-tag {
              font-size: 12px;
              font-weight: 700;
              color: $system-blue;
            }

            .qa-question-text {
              font-size: 13px;
              font-weight: 600;
              color: $text-primary;
              line-height: 1.5;
            }
          }

          .qa-card-body {
            font-size: 12.5px;
            line-height: 1.6;
            color: $text-secondary;
            padding-left: 28px;
            border-left: 2px solid $system-blue;
            margin-top: 6px;

            .qa-answer-title {
              display: flex;
              align-items: center;
              gap: 4px;
              font-weight: 600;
              color: $text-primary;
              margin-bottom: 3px;
            }
          }
        }
      }
    }

    .interview-section-card {
      padding: 16px;
      background: $bg-primary;
      border-radius: $radius-lg;
      border: 1px solid $separator;

      .section-card-header {
        .header-title {
          display: flex;
          align-items: center;
          gap: 6px;
          font-size: 14px;
          font-weight: 700;
          color: $system-blue;
        }
      }

      .interview-sug-list {
        margin: 10px 0 0 0;
        padding: 0;
        list-style: none;

        li {
          display: flex;
          align-items: flex-start;
          gap: 8px;
          font-size: 13px;
          line-height: 1.6;
          color: $text-primary;
          margin-bottom: 6px;

          .el-icon {
            color: $system-blue;
            font-size: 15px;
            margin-top: 2px;
          }
        }
      }
    }
  }

    .score-section {
      display: block;
      padding: 0;
      background: $bg-primary;
      border-radius: $radius-lg;
      margin-bottom: 10px;
      border: 1px solid $separator;

      .score-breakdown {
        display: flex;
        flex-direction: column;
        gap: 10px;
        padding: 0 16px 16px;

        .score-item {
          .label { display: block; font-size: 12px; color: $text-secondary; margin-bottom: 3px; }
          :deep(.el-progress__text) { font-size: 11px; font-weight: 600; }
        }
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
      padding: 12px 16px;
      background: $bg-primary;
      border: 1px solid $separator;
      border-radius: $radius-lg;
      margin-bottom: 10px;

      .candidate-name { font-size: 16px; font-weight: 700; color: $text-primary; margin-bottom: 4px; }
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
      display: block;
      margin-bottom: 10px;
      background: $bg-primary;
      border: 1px solid $separator;
      border-radius: $radius-lg;

      .detail-card {
        margin: 0 16px 10px;
        padding: 10px 12px;
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
      padding: 14px 16px;
      background: $bg-primary;
      border: 1px solid $separator;
      border-radius: $radius-lg;
      margin-bottom: 10px;

      .qa-section-head {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 12px;
        margin-bottom: 8px;

        h4 {
          display: flex;
          align-items: center;
          gap: 6px;
          margin: 0;
          font-size: 13.5px;
          font-weight: 700;
          color: #5856D6;
        }

        flex-wrap: wrap;
      }

      .qa-disclosure {
        border-top: 1px solid $separator;
        padding-top: 8px;

        .copy-qa-btn {
          display: inline-flex;
          align-items: center;
          gap: 4px;
          margin: 4px 0 10px;
          padding: 5px 10px;
          font-size: 11.5px;
          color: $system-blue;
          background: #eff6ff;
          border: 1px solid #dbeafe;
          border-radius: 6px;
          cursor: pointer;
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

// 浏览器原生 details 保留键盘与读屏展开语义，切换候选人时由 key 重置。
.preview-disclosure > summary,
.water-radar-card > summary,
.candidate-portal-bar > summary,
.qa-disclosure > summary {
  list-style: none;
  cursor: pointer;
  &:focus-visible { outline: 2px solid $system-blue; outline-offset: 2px; }
  &::-webkit-details-marker { display: none; }
  &::after {
    content: '⌄';
    flex-shrink: 0;
    margin-left: auto;
    color: $text-tertiary;
    font-size: 18px;
    line-height: 1;
    transition: transform 0.15s ease;
  }
}

.preview-disclosure[open] > summary::after,
.water-radar-card[open] > summary::after,
.candidate-portal-bar[open] > summary::after,
.qa-disclosure[open] > summary::after { transform: rotate(180deg); }

.preview-disclosure > summary {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  color: $text-primary;
  font-size: 13px;
  font-weight: 600;
  span { margin-left: auto; color: $text-secondary; font-size: 12px; font-weight: 400; }
  &::after { margin-left: 0; }
}

.qa-disclosure > summary {
  display: flex;
  align-items: center;
  padding: 4px 0;
  color: $system-blue;
  font-size: 12px;
}

details.water-radar-card > summary.water-radar-header { margin: 0; padding: 0; border-bottom: 0; }
details.water-radar-card[open] > summary.water-radar-header { margin-bottom: 14px; padding-bottom: 12px; border-bottom: 1px solid $separator; }
details.candidate-portal-bar > summary.portal-header { margin: 0; }
details.candidate-portal-bar[open] > summary.portal-header { margin-bottom: 12px; }

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
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 16px 20px 32px 18px;
  box-sizing: border-box;
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
      gap: 12px;
      flex-wrap: wrap;
      padding: 12px 16px;
      background: $bg-primary;
      border: 1px solid $separator;
      border-radius: $radius-lg $radius-lg 0 0;

      .content-heading { display: flex; align-items: center; gap: 10px; min-width: 0; flex-wrap: wrap; }

      .content-filename {
        font-size: 13px;
        font-weight: 600;
        color: $text-primary;
        overflow-wrap: anywhere;
      }

      .content-stats {
        font-size: 11px;
        color: $text-tertiary;
        padding: 2px 8px;
        background: rgba(0,122,255,0.08);
        border-radius: 4px;
        color: $system-blue;
      }
      .translation-status-pill {
        display: inline-flex;
        align-items: center;
        padding: 2px 8px;
        border-radius: 4px;
        background: rgba(16, 185, 129, 0.1);
        color: #059669;
        font-size: 11px;
        font-weight: 600;
      }
      .content-actions { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
      .content-actions :deep(.el-button + .el-button) { margin-left: 0; }
    }

    .content-hint {
      padding: 8px 16px;
      font-size: 12px;
      color: $text-tertiary;
      background: $bg-primary;
      border-left: 1px solid $separator;
      border-right: 1px solid $separator;
    }

    .parsed-sections {
      display: grid;
      gap: 12px;
      padding: 12px;
      background: $bg-primary;
      border: 1px solid $separator;
      border-top: none;
      border-radius: 0 0 $radius-lg $radius-lg;
    }

    .parsed-section {
      min-width: 0;
      padding: 16px 18px;
      border: 1px solid $separator;
      border-radius: $radius-lg;
      background: $bg-secondary;
    }

    .parsed-section-heading {
      display: flex;
      align-items: center;
      gap: 10px;
      margin-bottom: 12px;
      color: $text-primary;

      h3 { margin: 0; font-size: 15px; line-height: 1.4; overflow-wrap: anywhere; }
    }

    .parsed-section-number {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      flex: none;
      border-radius: 8px;
      background: rgba(79, 70, 229, 0.10);
      color: $system-blue;
      font-size: 11px;
      font-weight: 700;
    }

    .parsed-section-body { display: grid; gap: 9px; min-width: 0; max-width: 78ch; }
    .parsed-field { display: grid; grid-template-columns: minmax(110px, 160px) minmax(0, 1fr); gap: 12px; line-height: 1.65; }
    .parsed-field-label { color: $text-secondary; font-size: 12px; font-weight: 600; }
    .parsed-field-value, .parsed-paragraph, .parsed-bullet { color: $text-primary; font-size: 13px; line-height: 1.7; overflow-wrap: anywhere; }
    .parsed-paragraph { margin: 0; }
    .parsed-bullet { display: flex; gap: 9px; }
    .parsed-bullet > span { color: $system-blue; font-weight: 700; }

    .work-snapshot { display: grid; gap: 10px; min-width: 0; }
    .work-snapshot h4 { margin: 0; color: $text-secondary; font-size: 12px; }
    .snapshot-facts { display: flex; flex-wrap: wrap; gap: 6px; }
    .snapshot-facts span { padding: 4px 8px; border-radius: 6px; background: $bg-primary; border: 1px solid $separator; color: $text-secondary; font-size: 12px; overflow-wrap: anywhere; }
    .snapshot-experience {
      display: flex;
      flex-wrap: wrap;
      align-items: baseline;
      gap: 6px 10px;
      padding: 12px 14px;
      border: 1px solid $separator;
      border-left: 3px solid $system-blue;
      border-radius: 8px;
      background: $bg-primary;

      &.translated-subcard {
        margin-top: -4px;
        margin-bottom: 8px;
        border-left: 3px solid #10b981;
        background: rgba(16, 185, 129, 0.04);
        border-color: rgba(16, 185, 129, 0.22);

        .translated-subcard-header {
          width: 100%;
          display: flex;
          align-items: center;
          justify-content: space-between;
          margin-bottom: 4px;

          .subcard-badge {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            font-size: 11px;
            font-weight: 700;
            color: #059669;
            background: rgba(16, 185, 129, 0.12);
            padding: 2px 7px;
            border-radius: 4px;
          }

          .subcard-period {
            font-size: 11.5px;
            font-weight: 600;
            color: #059669;
          }
        }

        .snapshot-period {
          color: #059669;
        }
      }
    }
    .snapshot-period { color: $system-blue; font-size: 12px; font-weight: 700; }
    .snapshot-experience strong { color: $text-primary; font-size: 13px; overflow-wrap: anywhere; }
    .snapshot-role { color: $text-secondary; font-size: 12px; }
    .snapshot-experience p { flex-basis: 100%; margin: 0; color: $text-secondary; font-size: 12px; line-height: 1.6; overflow-wrap: anywhere; }

    .inline-trans-note {
      display: flex;
      align-items: center;
      gap: 6px;
      margin: 0;
      padding: 8px 12px;
      font-size: 12px;
      color: #059669;
      background: rgba(16, 185, 129, 0.08);
      border-radius: 6px;
      border: 1px solid rgba(16, 185, 129, 0.2);
    }

    .section-translation-container {
      margin-top: 14px;
      border-top: 1px dashed rgba(79, 70, 229, 0.25);
      padding: 12px 14px;
      background: rgba(79, 70, 229, 0.03);
      border-radius: $radius-md;
      border: 1px solid rgba(79, 70, 229, 0.12);

      .translation-card-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        cursor: pointer;
        user-select: none;

        .translation-badge {
          display: inline-flex;
          align-items: center;
          gap: 6px;
          font-size: 12px;
          font-weight: 600;
          color: $system-blue;

          .translated-section-title {
            color: $text-secondary;
            font-weight: normal;
          }
        }
      }

      .translation-card-body {
        margin-top: 10px;
        display: grid;
        gap: 9px;
        min-width: 0;
        max-width: 78ch;
      }

      .translation-snapshot {
        background: rgba(255, 255, 255, 0.7);
      }
    }

    .raw-content-view { display: block; width: 100%; }

    @media (max-width: 620px) {
      .parsed-field { grid-template-columns: 1fr; gap: 1px; }
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
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 16px 20px 32px 18px;
  box-sizing: border-box;
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
  margin: 0 0 10px;
  padding: 12px 16px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;

  .redline-alert-header {
    margin-bottom: 8px;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;

    .redline-title-group {
      display: flex;
      align-items: flex-start;
      gap: 10px;

      .redline-badge-icon {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        background: #fee2e2;
        color: #b91c1c;
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

  .empty-review-help {
    margin: 4px 0 10px;
    font-size: 12px;
    color: #64748b;
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
      justify-content: space-between;
      gap: 10px;
      font-size: 12px;
      color: #1e293b;
      background: #fff7f7;
      padding: 6px 12px;
      border-radius: 8px;
      border: 1px solid #fee2e2;

      .redline-stamp-badge {
        font-size: 11px;
        font-weight: 700;
        padding: 2px 8px;
        border-radius: 6px;
        white-space: nowrap;
        flex-shrink: 0;
        border: 1px solid transparent;

        &.met {
          background: #ecfdf5;
          color: #047857;
          border-color: #a7f3d0;
        }
        &.violated {
          background: #fef2f2;
          color: #b91c1c;
          border-color: #fca5a5;
        }
        &.unknown {
          background: #fffbeb;
          color: #b45309;
          border-color: #fde68a;
        }
      }

      .redline-item-desc {
        flex: 1;
        min-width: 0;
        line-height: 1.5;
        color: #334155;
      }

      .redline-btn-actions {
        display: flex;
        align-items: center;
        gap: 4px;
        flex-shrink: 0;
      }
    }
  }
}

// 加分达成看板
.bonus-match-card {
  margin: 0 0 10px;
  padding: 12px 16px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;

  .bonus-header {
    margin-bottom: 8px;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;

    .bonus-title-group {
      display: flex;
      align-items: flex-start;
      gap: 10px;

      .bonus-badge-icon {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        background: #fef3c7;
        color: #a16207;
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

  .empty-review-help {
    margin: 4px 0 10px;
    font-size: 12px;
    color: #64748b;
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
      background: #fffdf2;
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
  margin: 0 0 10px;
  padding: 12px 16px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;

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
  margin: 0 0 10px;
  padding: 12px 16px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;

  .portal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px 12px;
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

.candidate-source-kw-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: #f0fdf4;
  color: #166534;
  border: 1px solid #bbf7d0;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  font-weight: 600;
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
  margin: 0 0 10px;
  padding: 12px 16px;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
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

  .water-timeline-embed {
    margin: 12px 0 14px;
  }

  .water-radar-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
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
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 12px;
    min-width: 0;
    width: 100%;
    box-sizing: border-box;

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
