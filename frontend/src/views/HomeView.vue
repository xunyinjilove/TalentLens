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
      <aside class="left-panel" :style="{ width: leftPanelWidth + 'px' }">
        <DropZone @select-files="handleSelectFiles" />

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
                      <el-dropdown-item command="reanalyze" :disabled="resumeStore.isAnalyzing || !hasReanalyzableResumes">
                        <el-icon><RefreshRight /></el-icon>
                        <span>{{ $t('home.reAnalyzeAll') }}</span>
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

            <!-- 第二行：三项核心操作按钮（矩阵寻才、继续寻才、开始分析/已全部分析）(Q1, Q3) -->
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
                :class="{ 'is-all-done': resumeStore.analysisReadyCount === 0 && !resumeStore.isAnalyzing && resumeStore.resumes.length > 0 }"
                @click="handleStartAnalysis"
                :disabled="resumeStore.isAnalyzing || resumeStore.analysisReadyCount === 0 || resumeStore.resumes.length === 0"
                :title="resumeStore.analysisReadyCount === 0 && resumeStore.resumes.length > 0 ? '所有简历已全部分析完毕。如需重新分析，请从右上角「更多」菜单选择「全部重新分析」' : ''"
              >
                <el-icon v-if="resumeStore.isAnalyzing" class="spin"><Loading /></el-icon>
                <el-icon v-else-if="resumeStore.analysisReadyCount > 0"><VideoPlay /></el-icon>
                <el-icon v-else><CircleCheck /></el-icon>
                <span v-if="resumeStore.isAnalyzing">{{ $t('home.analyzing') }}</span>
                <span v-else-if="resumeStore.analysisReadyCount > 0">手动分析（{{ resumeStore.analysisReadyCount }}）</span>
                <span v-else>{{ $t('home.allAnalyzed') }}</span>
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
              @click="resumeStore.selectResume(resume.id)"
              @re-analyze="handleReAnalyze"
              @delete="resumeStore.deleteResume"
            />
            <el-empty v-if="resumeStore.resumes.length === 0" :description="$t('home.dragHint')" />
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
                前十名打招呼
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
                      <span>{{ (resumeStore.selectedResume.analysis?.candidateName || resumeStore.selectedResume.fileName || '人').slice(0, 1) }}</span>
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

                  <!-- 右上角：综合评分与推荐结论 (固定在右上，flex-shrink: 0，永远醒目) -->
                  <div v-if="resumeStore.selectedResume.status === 'done' && resumeStore.selectedResume.analysis" class="hero-score-section">
                    <div class="hero-score-box" :class="getScoreClass(resumeStore.selectedResume.score)">
                      <strong class="hero-score-num">{{ animatedDetailScore || resumeStore.selectedResume.score }}</strong>
                      <span class="hero-score-label">{{ $t('analysis.totalScore') }}</span>
                    </div>
                    <div class="hero-decision-badge-wrap">
                      <span class="hero-rec-badge" :class="resumeStore.selectedResume.analysis.recommendation">
                        {{ $t(`analysis.recommendations.${resumeStore.selectedResume.analysis.recommendation || 'consider'}`) }}
                      </span>
                      <span class="hero-redline-status" :class="resumeStore.selectedResume.analysis.redLineStatus || 'none'">
                        红线: {{ !resumeStore.selectedResume.analysis.redLineChecks?.length ? '尚未配置' : resumeStore.selectedResume.analysis.redLineStatus === 'failed' ? '不满足' : resumeStore.selectedResume.analysis.redLineStatus === 'pending' ? '待 HR 核实' : '已满足' }}
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
                  <span>综合结论与红线</span>
                  <span v-if="unverifiedRedLineCount > 0" class="subtab-badge alert" title="有待HR核实的红线项">
                    {{ unverifiedRedLineCount }} 项待核
                  </span>
                </button>

                <button
                  class="subtab-btn"
                  :class="{ active: detailSubTab === 'competency' }"
                  @click="detailSubTab = 'competency'"
                >
                  <el-icon><Histogram /></el-icon>
                  <span>能力维度拆解</span>
                  <span class="subtab-badge info">
                    核心 {{ resumeStore.selectedResume.analysis.abilityScore ?? resumeStore.selectedResume.score }}分
                  </span>
                </button>

                <button
                  class="subtab-btn"
                  :class="{ active: detailSubTab === 'water' }"
                  @click="detailSubTab = 'water'"
                >
                  <el-icon><Aim /></el-icon>
                  <span>防伪与注水雷达</span>
                  <span class="subtab-badge" :class="waterRiskLevel === 'high' ? 'danger' : waterRiskLevel === 'medium' ? 'warning' : 'safe'">
                    {{ waterRiskLevel === 'high' ? '高风险' : waterRiskLevel === 'medium' ? '中风险' : '低风险' }} · {{ waterScore }}分
                  </span>
                </button>

                <button
                  class="subtab-btn"
                  :class="{ active: detailSubTab === 'interview' }"
                  @click="detailSubTab = 'interview'"
                >
                  <el-icon><ChatLineSquare /></el-icon>
                  <span>初试提纲与建议</span>
                  <span v-if="interviewQACount > 0" class="subtab-badge count">
                    {{ interviewQACount }} 题
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

                  <!-- 岗位红线逐项核验卡 -->
                  <div class="redline-alert-card">
                    <div class="redline-alert-header">
                      <div class="redline-title-group">
                        <div class="redline-badge-icon">
                          <el-icon><CircleClose /></el-icon>
                        </div>
                        <div class="redline-title-text">
                           <span class="redline-title">岗位红线逐项核验：{{ !resumeStore.selectedResume.analysis.redLineChecks?.length ? '尚未配置' : resumeStore.selectedResume.analysis.redLineStatus === 'failed' ? '不满足' : resumeStore.selectedResume.analysis.redLineStatus === 'pending' ? '待核实' : '满足' }}</span>
                           <span class="redline-sub">AI 线索不能直接淘汰；HR 确认不满足时不推荐，能力原始分仍保留。</span>
                        </div>
                      </div>
                      <el-button size="small" @click="handleAddProjectReviewCriterion('redline')">添加项目红线</el-button>
                    </div>
                    <p v-if="!resumeStore.selectedResume.analysis.redLineChecks?.length" class="empty-review-help">本项目尚未设置红线。添加后，该项目的候选人会先标记为待 HR 核实。</p>
                    <ul class="redline-violations-list">
                       <li v-for="(check, vIdx) in resumeStore.selectedResume.analysis.redLineChecks" :key="vIdx" class="redline-violation-item">
                        <span class="redline-stamp-badge" :class="check.status || 'unknown'">
                          {{ check.status === 'met' ? '✓ 合规满足' : check.status === 'violated' ? '✕ 触碰红线' : '？ 待HR核验' }}
                        </span>
                        <span class="redline-item-desc">{{ check.criterion }}{{ check.suggested_status && check.status === 'unknown' ? `（AI提示：${check.suggested_status === 'met' ? '可能满足' : '可能不满足'}）` : '' }}{{ check.evidence ? `（原文线索：${check.evidence}）` : '（暂无可核对原文）' }}</span>
                        <div class="redline-btn-actions">
                          <el-button size="small" text type="success" @click="handleReviewRedLine(check.criterion, 'met')">确认满足</el-button>
                          <el-button size="small" text type="danger" @click="handleReviewRedLine(check.criterion, 'violated')">确认不满足</el-button>
                          <el-button size="small" text @click="handleReviewRedLine(check.criterion, 'unknown')">待核实</el-button>
                        </div>
                      </li>
                    </ul>
                  </div>

                  <!-- ⭐ 优先加分项卡 -->
                  <div class="bonus-match-card">
                    <div class="bonus-header">
                      <div class="bonus-title-group">
                        <div class="bonus-badge-icon">
                          <el-icon><Check /></el-icon>
                        </div>
                        <div class="bonus-title-text">
                          <span class="bonus-title">⭐ 优先加分项：{{ !resumeStore.selectedResume.analysis.bonusChecks?.length ? '尚未配置' : `${resumeStore.selectedResume.analysis.confirmedBonusCount || 0} 项已确认` }}</span>
                          <span class="bonus-sub">待确认项不计入排序；已确认项只在同一能力档内比较，不改变能力分。</span>
                        </div>
                      </div>
                      <el-button size="small" @click="handleAddProjectReviewCriterion('bonus')">添加项目加分项</el-button>
                    </div>
                    <p v-if="!resumeStore.selectedResume.analysis.bonusChecks?.length" class="empty-review-help">本项目尚未设置加分项。添加后，由 HR 逐人选择待确认、确认满足或确认不满足。</p>
                    <ul class="bonus-matches-list">
                      <li v-for="(bonus, bIdx) in resumeStore.selectedResume.analysis.bonusChecks" :key="bIdx" class="bonus-match-item">
                        <el-icon><Check /></el-icon>
                        <span>{{ bonus.status === 'met' ? 'HR 已确认' : bonus.status === 'violated' ? 'HR 确认不满足' : '待 HR 确认' }} · {{ bonus.criterion }}{{ bonus.suggested_status && bonus.status === 'unknown' ? `（AI 线索：${bonus.suggested_status === 'met' ? '可能满足' : '可能不满足'}）` : '' }}{{ bonus.evidence ? `（依据：${bonus.evidence}）` : '' }}</span>
                        <el-button size="small" text type="success" @click="handleReviewBonus(bonus.criterion, 'met')">确认满足</el-button>
                        <el-button size="small" text type="danger" @click="handleReviewBonus(bonus.criterion, 'violated')">确认不满足</el-button>
                        <el-button size="small" text @click="handleReviewBonus(bonus.criterion, 'unknown')">待确认</el-button>
                      </li>
                    </ul>
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

                  <!-- 评分面板与逐项评分卡 -->
                  <div class="score-section-card modern-competency-card">
                    <div class="section-card-header">
                      <div class="header-title">
                        <el-icon><Histogram /></el-icon>
                        <span>核心能力维度全景雷达 ({{ resumeStore.selectedResume.analysis.abilityScore ?? resumeStore.selectedResume.score }} 分)</span>
                      </div>
                      <el-button size="small" type="primary" plain :loading="competencyBusy" @click="supplementCompetencies">
                        <el-icon style="margin-right: 4px"><RefreshRight /></el-icon>
                        {{ competencyButtonText }}
                      </el-button>
                    </div>

                    <el-alert v-if="resumeStore.selectedResume.analysis.competencyMessage" :title="resumeStore.selectedResume.analysis.competencyMessage" :type="resumeStore.selectedResume.analysis.competencyStatus === 'error' ? 'warning' : 'info'" :closable="false" show-icon style="margin:0 0 16px" />

                    <div class="competency-mode-bar">
                      <el-radio-group v-model="radarMode" size="small">
                        <el-radio-button label="job">岗位评分卡</el-radio-button>
                        <el-radio-button label="experience" :disabled="!resumeStore.selectedResume.analysis.experienceCompetencies?.length">
                          履历能力补充
                          <span
                            v-if="resumeStore.selectedResume.analysis.experienceCompetencies?.length"
                            class="competency-count-badge"
                          >
                            ({{ resumeStore.selectedResume.analysis.experienceCompetencies.length }})
                          </span>
                        </el-radio-button>
                      </el-radio-group>
                      <span class="header-tip">{{ radarMode === 'experience' ? '根据实际经历提炼，逐项展示依据；补充维度不计入岗位能力分。' : '对照岗位要求，未取得原文依据的项目保留为未体现。' }}</span>
                    </div>

                    <div class="competency-split-layout">
                      <!-- 左侧：原生 SVG 蜘蛛网雷达图 -->
                      <div class="radar-visual-col">
                        <SvgRadarChart
                          v-if="radarMode === 'job' || activeRadarChecks.length >= 3"
                          :indicators="radarIndicators"
                          :candidate-values="radarCandidateValues"
                          :show-benchmark="radarMode === 'job'"
                          :selected-index="selectedRadarIndex"
                          @select="handleSelectRadarIndex"
                        />
                        <p v-else class="score-explanation">目前仅有 {{ activeRadarChecks.length }} 项可引用的履历能力，详见右侧；材料不足时不补画无依据的维度。</p>
                      </div>

                      <!-- 右侧：逐项评分卡与原文线索 -->
                      <div class="score-breakdown-box">
                        <p v-if="radarMode === 'job' && resumeStore.selectedResume.analysis.scoreBasis === 'provisional'" class="score-explanation warning">
                          ⚠️ 暂估分：AI 未提供足够可定位的逐项原文，系统改用技能与经验匹配估计；请 HR 核对后再推进。
                        </p>
                        <div
                          v-for="(item, cIdx) in activeRadarChecks"
                          :key="item.criterion"
                          class="criterion-item-row clickable"
                          :class="{ highlighted: selectedRadarIndex === cIdx }"
                          @click="handleSelectRadarIndex(cIdx)"
                        >
                          <div class="criterion-name-col">
                            <span class="criterion-name">{{ item.criterion }}</span>
                            <span class="criterion-level-badge" :class="'level-' + item.level">
                              {{ item.level }}/4 · {{ ['未体现', '相关经历线索', '参与', '独立完成', '主导并可复盘'][item.level] || '待核实' }}
                            </span>
                          </div>
                          <div v-if="item.evidence" class="criterion-evidence-col">
                            {{ item.evidence }}
                          </div>
                          <p v-if="item.assessment" class="criterion-assessment">{{ item.assessment }}</p>
                        </div>
                        <p v-if="!activeRadarChecks.length" class="score-explanation empty">
                          暂无逐项评分卡；当前显示核心匹配估计，建议重新分析以取得可核对的逐项依据。
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

                <!-- ==================== SUB-TAB 3: 防伪与注水雷达 ==================== -->
                <div v-show="detailSubTab === 'water'" class="subtab-pane water-pane">
                  <div class="water-radar-card modern-card" :class="getWaterRiskClass(resumeStore.selectedResume.analysis?.water_check?.risk_level)">
                    <div class="water-radar-header-bar">
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
                          <span class="radar-sub-desc">基于履历时间轴断层、跳槽频率及量化指标真实度检测</span>
                        </div>
                      </div>
                      <div class="radar-score-box" :class="resumeStore.selectedResume.analysis?.water_check?.risk_level || 'low'">
                        <div class="radar-score-num">{{ resumeStore.selectedResume.analysis?.water_check?.water_score ?? 15 }}</div>
                        <div class="radar-score-caption">注水破绽分 (越低越真实)</div>
                      </div>
                    </div>

                    <!-- 可视化履历时间轴与空窗断层追踪 (Work Timeline) -->
                    <WorkTimeline
                      :gaps="resumeStore.selectedResume.analysis?.water_check?.gaps"
                      :work-years="resumeStore.selectedResume.analysis?.workYears"
                      :current-role="resumeStore.selectedResume.analysis?.currentRole"
                      class="water-timeline-embed"
                    />

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
                </div>

                <!-- ==================== SUB-TAB 4: 初试提纲与建议 ==================== -->
                <div v-show="detailSubTab === 'interview'" class="subtab-pane interview-pane">
                  <!-- AI 初试提纲与参考回答 -->
                  <div v-if="resumeStore.selectedResume.analysis.interviewQA && resumeStore.selectedResume.analysis.interviewQA.length > 0" class="qa-section-card">
                    <div class="qa-section-head">
                      <div class="qa-title-wrap">
                        <el-icon><QuestionFilled /></el-icon>
                        <span class="qa-head-title">AI 预面试提纲与参考回答 ({{ resumeStore.selectedResume.analysis.interviewQA.length }}题)</span>
                      </div>
                      <div class="qa-actions-row">
                        <button class="copy-qa-btn" @click="handleCopyAllQA" title="一键复制全套提纲">
                          <el-icon><DocumentCopy /></el-icon> 复制全套提纲
                        </button>
                        <el-button size="small" type="primary" @click="showInterviewDialog = true">
                          <el-icon><ChatDotRound /></el-icon> 发起 AI 文字初面
                        </el-button>
                      </div>
                    </div>

                    <InterviewAccordionCard :qa-list="resumeStore.selectedResume.analysis.interviewQA" />
                  </div>

                  <!-- 面试重点建议 -->
                  <div v-if="resumeStore.selectedResume.analysis.interviewSuggestions && resumeStore.selectedResume.analysis.interviewSuggestions.length > 0" class="interview-section-card">
                    <div class="section-card-header">
                      <div class="header-title">
                        <el-icon><ChatLineSquare /></el-icon>
                        <span>业务主管初面把关建议</span>
                      </div>
                    </div>
                    <ul class="interview-sug-list">
                      <li v-for="(item, idx) in resumeStore.selectedResume.analysis.interviewSuggestions" :key="idx">
                        <el-icon><CircleCheck /></el-icon>
                        <span>{{ item }}</span>
                      </li>
                    </ul>
                  </div>
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
                <div class="content-heading">
                  <span class="content-filename">{{ resumeStore.selectedResume.fileName }}</span>
                  <span class="content-stats">{{ $t('home.contentLength', { count: parsedContent.length }) }}</span>
                </div>
                <el-button size="small" plain @click="showRawContent = !showRawContent">
                  {{ showRawContent ? '查看分区排版' : '查看完整原文' }}
                </el-button>
              </div>
              <div class="content-hint">以下内容只调整排版，不改写简历原文；原文可随时切换查看。</div>
              <pre v-if="showRawContent" class="content-text">{{ parsedContent }}</pre>
              <div v-else class="parsed-sections">
                <section v-for="(section, index) in parsedSections" :key="index" class="parsed-section">
                  <div class="parsed-section-heading">
                    <span class="parsed-section-number">{{ String(index + 1).padStart(2, '0') }}</span>
                    <h3>{{ section.title }}</h3>
                  </div>
                  <div class="parsed-section-body">
                    <template v-for="(line, lineIndex) in section.lines" :key="lineIndex">
                      <div v-if="line.kind === 'field'" class="parsed-field">
                        <span class="parsed-field-label">{{ line.label }}</span>
                        <span class="parsed-field-value">{{ line.value }}</span>
                      </div>
                      <div v-else-if="line.kind === 'bullet'" class="parsed-bullet"><span aria-hidden="true">•</span>{{ line.text }}</div>
                      <p v-else class="parsed-paragraph">{{ line.text }}</p>
                    </template>
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
                  <span>{{ !r.analysis ? '待分析' : r.analysis.redLineStatus === 'failed' ? '必备条件不满足' : r.analysis.redLineStatus === 'pending' ? '必备条件待核实' : r.analysis.redLineStatus === 'not_configured' ? '未配置红线' : '红线已核实' }} · 已确认优势 {{ r.analysis?.confirmedBonusCount || 0 }}</span>
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
      :job-description="projectStore.currentProject?.job_config?.description || projectStore.currentJobConfig?.description"
      :exp-years="projectStore.currentProject?.job_config?.experience_years"
      :edu-level="projectStore.currentProject?.job_config?.education_level"
      :is-continue="bossDialogContinueMode"
      @refresh="handleRefreshProject"
    />
    <AIInterviewDialog v-model="showInterviewDialog" :resume-id="resumeStore.selectedResume?.id || ''" />
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
  Link, Plus, Share, Aim, Filter, Download, Delete, Finished, Histogram, Calendar
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import TitleBar from '../components/TitleBar.vue'
import DropZone from '../components/DropZone.vue'
import ResumeCard from '../components/ResumeCard.vue'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import AIConfigGuide from '../components/AIConfigGuide.vue'
import DevPanel from '../components/DevPanel.vue'
import BossSearchDialog from '../components/BossSearchDialog.vue'
import AIInterviewDialog from '../components/AIInterviewDialog.vue'
import TopTenOutreachDialog from '../components/TopTenOutreachDialog.vue'
import SvgDonutRing from '../components/charts/SvgDonutRing.vue'
import SvgRadarChart from '../components/charts/SvgRadarChart.vue'
import WorkTimeline from '../components/WorkTimeline.vue'
import InterviewAccordionCard from '../components/InterviewAccordionCard.vue'
import { useResumeStore } from '../composables/useResumeStore'
import { useProjectStore } from '../composables/useProjectStore'
import { runWith51JobBrowser } from '../composables/51jobBrowserRecovery'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const resumeStore = useResumeStore()
const projectStore = useProjectStore()

// 项目上下文
const projectId = computed(() => (route.params.id as string) || projectStore.currentProjectId || '')
const projectName = computed(() => projectStore.currentProject?.name || '')
const showOutreach = ref(false)
const competencyBusy = ref(false)
const snapshotHistoryOpen = ref(false)
const snapshotHistory = ref<any[]>([])
const radarMode = ref<'job' | 'experience'>('job')
const competencyButtonText = computed(() => {
  const analysis = resumeStore.selectedResume?.analysis
  if (!analysis) return '按经历补全雷达'
  if (analysis.competencyStatus === 'error') return '重试雷达补全'
  if (analysis.experienceCompetencies && analysis.experienceCompetencies.length > 0) {
    return '重新核验经历雷达'
  }
  return '补充提炼经历雷达'
})
const activeRadarChecks = computed(() => radarMode.value === 'experience'
  ? resumeStore.selectedResume?.analysis?.experienceCompetencies || []
  : resumeStore.selectedResume?.analysis?.coreCriterionChecks || [])
const rightTab = ref<'detail' | 'ranking' | 'content'>('detail')
const detailSubTab = ref<'summary' | 'competency' | 'water' | 'interview'>('summary')

// 蜘蛛网雷达图维度与数值提取
const selectedRadarIndex = ref<number | null>(null)
function handleSelectRadarIndex(idx: number) {
  selectedRadarIndex.value = selectedRadarIndex.value === idx ? null : idx
}

const radarIndicators = computed(() => {
  const coreChecks = activeRadarChecks.value
  if (coreChecks && coreChecks.length >= 3) {
    return coreChecks.map(c => ({ name: c.criterion, max: 4 }))
  }
  return [
    { name: '核心技能', max: 100 },
    { name: '业务经验', max: 100 },
    { name: '学历资质', max: 100 },
    { name: '综合胜任力', max: 100 }
  ]
})

const radarCandidateValues = computed(() => {
  const coreChecks = activeRadarChecks.value
  if (coreChecks && coreChecks.length >= 3) {
    return coreChecks.map(c => c.level ?? 2)
  }
  const a = resumeStore.selectedResume?.analysis
  return [
    a?.skillMatch ?? 75,
    a?.experienceMatch ?? 70,
    a?.educationMatch ?? 80,
    resumeStore.selectedResume?.score ?? 75
  ]
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

// 防伪风险信息
const waterRiskLevel = computed(() => {
  return resumeStore.selectedResume?.analysis?.water_check?.risk_level || 'low'
})
const waterScore = computed(() => {
  return resumeStore.selectedResume?.analysis?.water_check?.water_score ?? 15
})

// 面试题数量
const interviewQACount = computed(() => {
  return resumeStore.selectedResume?.analysis?.interviewQA?.length || 0
})

const parsedContent = ref('')
const contentLoading = ref(false)
const showRawContent = ref(false)
type ParsedLine = { kind: 'field' | 'bullet' | 'text'; text?: string; label?: string; value?: string }
type ParsedSection = { title: string; lines: ParsedLine[] }
const parsedSections = computed<ParsedSection[]>(() => {
  const sections: ParsedSection[] = []
  let current: ParsedSection = { title: '基本信息', lines: [] }
  for (const rawLine of parsedContent.value.replace(/\r\n?/g, '\n').split('\n')) {
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
  return sections.length ? sections : [{ title: '原文内容', lines: [{ kind: 'text', text: parsedContent.value }] }]
})
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
const showInterviewDialog = ref(false)
const isClearingProject = ref(false)
const hasReanalyzableResumes = computed(() => resumeStore.resumes.some(r => r.status === 'done' || r.status === 'error'))
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

async function supplementCompetencies() {
  const resume = resumeStore.selectedResume
  if (!resume || competencyBusy.value) return
  competencyBusy.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.SupplementResumeCompetencies(resume.id)
    await resumeStore.loadProjectResumes(projectId.value)
    if (resumeStore.selectedResume?.analysis?.experienceCompetencies?.length) radarMode.value = 'experience'
    selectedRadarIndex.value = null
    ElMessage.success('能力维度已按简历经历更新，请结合原文核实')
  } catch (err: any) { await resumeStore.loadProjectResumes(projectId.value); ElMessage.error(err?.message || String(err)) }
  finally { competencyBusy.value = false }
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

// 排名先看准入，再按 5 分能力档、已确认优势、能力原分比较。
const rankedResumes = computed(() => {
  const eligibility = (status?: string) => status === 'failed' ? 2 : status === 'pending' ? 1 : 0
  return resumeStore.resumes
    .filter(r => r.status === 'done' && r.score)
    .sort((a, b) => eligibility(a.analysis?.redLineStatus) - eligibility(b.analysis?.redLineStatus)
      || Math.floor((b.analysis?.abilityScore ?? b.score) / 5) - Math.floor((a.analysis?.abilityScore ?? a.score) / 5)
      || (b.analysis?.confirmedBonusCount || 0) - (a.analysis?.confirmedBonusCount || 0)
      || (b.analysis?.abilityScore ?? b.score) - (a.analysis?.abilityScore ?? a.score))
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
  const pendingCount = resumeStore.analysisReadyCount
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
  radarMode.value = 'job'
  selectedRadarIndex.value = null
  if (rightTab.value === 'content') { loadParsedContent() }
})

async function handleReAnalyze(id: string) {
  if (!checkAIConfig()) { showConfigGuide.value = true; return }
  await resumeStore.reAnalyze(id)
}

async function handleReAnalyzeAll() {
  if (!hasReanalyzableResumes.value) { ElMessage.warning(t('home.noAnalyzedResumes')); return }
  if (!checkAIConfig()) { showConfigGuide.value = true; return }
  const ok = await resumeStore.reAnalyzeAll()
  if (!ok) { ElMessage.warning(t('home.noAnalyzedResumes')) }
}

function handleResumeMenuCommand(command: string) {
  if (command === 'reanalyze') void handleReAnalyzeAll()
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
    // 加载项目下的简历
    await resumeStore.loadProjectResumes(projectId.value)
  }
  contactPollTimer = window.setInterval(() => { void poll51JobReplies() }, 120000)
})

onUnmounted(() => {
  cleanupResizeListeners()
  if (contactPollTimer !== null) window.clearInterval(contactPollTimer)
  resumeStore.cleanupWailsEvents()
})
</script>

<style scoped lang="scss">
.competency-mode-bar { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin: 12px 0; }
.competency-mode-bar .header-tip { color: var(--wb-muted); font-size: 12px; line-height: 1.6; }
.competency-count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-left: 4px;
  padding: 0 5px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 10px;
  background: rgba(22, 119, 255, 0.1);
  color: #1677ff;
  line-height: 16px;
}
:deep(.el-radio-button.is-active) .competency-count-badge {
  background: rgba(255, 255, 255, 0.25);
  color: #fff;
}
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

  .resume-section {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
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
    gap: 8px;
    width: 100%;
    min-width: 0;

    .action-btn {
      flex: 1 1 0;
      min-width: 0;
      height: 35px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 5px;
      padding: 0 8px;
      border-radius: 8px;
      font-size: 12px;
      font-weight: 600;
      white-space: nowrap;
      cursor: pointer;
      transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
      line-height: 1;

      .el-icon {
        font-size: 14px;
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

        &.is-all-done {
          background: rgba(37, 99, 235, 0.08);
          border: 1px solid rgba(37, 99, 235, 0.25);
          color: var(--accent-blue, #2563eb);
          box-shadow: none;
          cursor: not-allowed;
          opacity: 0.9;

          &:hover {
            transform: none;
            box-shadow: none;
          }
        }

        &:disabled:not(.is-all-done) {
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
        }

        .score-breakdown-box {
          flex: 1;
          min-width: 0;
          width: 100%;
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
            color: $text-primary;

            .criterion-level-badge {
              font-size: 11px;
              padding: 1px 7px;
              border-radius: 4px;
              font-weight: 600;

              &.level-4 { background: #ecfdf5; color: #047857; }
              &.level-3 { background: #eff6ff; color: #1d4ed8; }
              &.level-2 { background: #f8fafc; color: #475569; }
              &.level-1, &.level-0 { background: #fef2f2; color: #b91c1c; }
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

    .parsed-section-body { display: grid; gap: 9px; max-width: 78ch; }
    .parsed-field { display: grid; grid-template-columns: minmax(110px, 160px) minmax(0, 1fr); gap: 12px; line-height: 1.65; }
    .parsed-field-label { color: $text-secondary; font-size: 12px; font-weight: 600; }
    .parsed-field-value, .parsed-paragraph, .parsed-bullet { color: $text-primary; font-size: 13px; line-height: 1.7; overflow-wrap: anywhere; }
    .parsed-paragraph { margin: 0; }
    .parsed-bullet { display: flex; gap: 9px; }
    .parsed-bullet > span { color: $system-blue; font-weight: 700; }

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
