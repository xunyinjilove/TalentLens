<template>
  <div class="workbench-page talent-pool-page">
    <!-- 顶部全局工具头 -->
    <header class="wb-header talent-page-header">
      <div class="header-title-box">
        <div class="title-with-badge">
          <h1>简历人才库</h1>
          <span class="pool-count-tag" :title="`人才库内共有 ${recruitment.candidates.length} 位人选`">
            {{ recruitment.candidates.length }} 位人选
          </span>
        </div>
        <div class="wb-crumb">人才检索 / 按职位分库多维归档、查验与协同调配</div>
      </div>

      <div class="header-actions-bar" aria-label="人才库操作">
        <el-button
          type="primary"
          class="matrix-search-btn"
          :disabled="!selectedProjectId"
          @click="showSearch = true"
          :title="selectedProjectId ? '唤起 4合1 多平台矩阵式主动寻才' : '请先选择左侧具体职位分库'"
        >
          <el-icon><Position /></el-icon><span>矩阵寻才</span>
        </el-button>

        <el-button
          class="outreach-action-btn"
          :disabled="!selectedProjectId"
          @click="startBatchOutreach()"
          :title="selectedProjectId ? '一键向当前分库筛选候选人或前十名发起批量沟通' : '请先选择左侧具体职位分库'"
        >
          <el-icon><ChatRound /></el-icon><span>批量打招呼</span>
        </el-button>

        <el-dropdown trigger="click" @command="handleLibraryAction">
          <el-button class="library-tools-btn" aria-label="导入与核对，展开更多操作">
            <el-icon><FolderOpened /></el-icon><span>导入与核对</span>
            <span v-if="pendingReviewCount" class="library-tools-count">{{ pendingReviewCount > 99 ? '99+' : pendingReviewCount }}</span>
            <el-icon class="dropdown-chevron"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="import" :disabled="importing">
                <el-icon><Download /></el-icon>导入 51job 投递
              </el-dropdown-item>
              <el-dropdown-item command="inbox">
                <el-icon><Tickets /></el-icon>待分配投递
                <span v-if="pendingApplications.length" class="dropdown-item-count">{{ pendingApplications.length }}</span>
              </el-dropdown-item>
              <el-dropdown-item command="duplicates">
                <el-icon><CopyDocument /></el-icon>疑似重复
                <span v-if="duplicates.length" class="dropdown-item-count">{{ duplicates.length }}</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <el-button
          class="refresh-tool-btn"
          :loading="recruitment.loading"
          aria-label="刷新人才库数据"
          title="刷新人才库数据"
          @click="refresh"
        >
          <el-icon v-if="!recruitment.loading"><Refresh /></el-icon>
        </el-button>
      </div>
    </header>

    <!-- 主体分栏：左侧职位分库导航 + 右侧候选人看板 -->
    <div class="talent-workbench-layout">
      <!-- 左侧：职位分库导航侧栏 -->
      <aside class="job-repositories-sidebar">
        <div class="repo-header">
          <div class="repo-title">职位分库导航</div>
          <el-button
            link
            type="primary"
            class="add-repo-link"
            @click="router.push('/jobs')"
            title="管理或新建职位"
          >
            <el-icon><Plus /></el-icon>新建分库
          </el-button>
        </div>

        <div class="repo-search">
          <el-input
            v-model="repoFilterKeyword"
            placeholder="筛选职位分库…"
            clearable
            size="small"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </div>

        <div class="repo-list">
          <!-- 全部人才库 (总览汇总) -->
          <div
            class="repo-item"
            :class="{ active: !selectedProjectId }"
            @click="selectRepository('')"
          >
            <div class="repo-icon all-repo-icon">
              <el-icon><Collection /></el-icon>
            </div>
            <div class="repo-info">
              <span class="repo-name">全部人才库 (汇总)</span>
              <span class="repo-sub">跨职位全部人选</span>
            </div>
            <span class="repo-badge">{{ recruitment.candidates.length }}</span>
          </div>

          <div class="repo-divider">
            <span>各职位专属分库 ({{ filteredProjectRepos.length }})</span>
          </div>

          <!-- 各职位专属分库项 -->
          <div
            v-for="proj in filteredProjectRepos"
            :key="proj.id"
            class="repo-item"
            :class="{ active: selectedProjectId === proj.id }"
            @click="selectRepository(proj.id)"
          >
            <div class="repo-status-indicator" :class="normalizeRepoStatusClass(proj.status)"></div>
            <div class="repo-info">
              <span class="repo-name" :title="proj.job_config?.title || proj.name">
                {{ proj.job_config?.title || proj.name }}
              </span>
              <span class="repo-sub">
                {{ proj.department || '综合' }} · {{ proj.job_config?.city || '全国' }}
              </span>
            </div>
            <span class="repo-badge" :class="{ 'has-count': countFor(proj.id) > 0 }">
              {{ countFor(proj.id) }}
            </span>
          </div>

          <div v-if="!filteredProjectRepos.length" class="empty-repos">
            无匹配职位分库
          </div>
        </div>
      </aside>

      <!-- 右侧：候选人看板主内容区 -->
      <main class="talent-content-area">
        <!-- 错误提示（过滤掉旧简历脏数据干扰） -->
        <el-alert
          v-if="recruitment.error && !recruitment.error.includes('引用缺失的旧简历')"
          type="warning"
          :title="recruitment.error"
          show-icon
          closable
          @close="recruitment.error = ''"
          style="margin-bottom: 16px; margin-right: 8px;"
        />

        <!-- 当前分库信息横幅 (选定职位分库时展示) -->
        <div v-if="selectedProject" class="selected-repo-banner">
          <div class="repo-banner-info">
            <div class="repo-banner-title">
              <h2>{{ selectedProject.job_config?.title || selectedProject.name }} 专属人才库</h2>
              <span class="banner-status-tag" :class="normalizeRepoStatusClass(selectedProject.status)">
                {{ normalizeStatusLabel(selectedProject.status) }}
              </span>
            </div>
            <div class="repo-banner-meta">
              <span>部门：{{ selectedProject.department || '未分配' }}</span>
              <span class="sep">·</span>
              <span>城市：{{ selectedProject.job_config?.city || '上海' }}</span>
              <span class="sep">·</span>
              <span>性质：{{ selectedProject.job_config?.employment_type || '全职' }}</span>
              <span class="sep">·</span>
              <span>薪资：{{ selectedProject.job_config?.salary_range || '15-25K' }}</span>
              <span class="sep">·</span>
              <span>计划编制：{{ selectedProject.headcount || 1 }} 人</span>
              <span class="sep">·</span>
              <span>库内人选：<strong>{{ countFor(selectedProject.id) }}</strong> 位</span>
            </div>
          </div>
          <div class="repo-banner-actions">
            <el-button size="small" @click="router.push({ path: '/jobs', query: { edit: selectedProject.id } })">职位设置</el-button>
            <el-button size="small" type="primary" plain @click="router.push('/project/' + selectedProject.id)">
              项目完整评测
            </el-button>
          </div>
        </div>

        <!-- 筛选与搜索工具条（含卡片/表格双视图切换） -->
        <div class="wb-toolbar talent-filter-toolbar">
          <div class="toolbar-left-filters">
            <el-input
              v-model="keyword"
              clearable
              placeholder="搜索姓名、岗位或项目…"
              class="search-filter-input"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>

            <el-select v-model="statusFilter" placeholder="分析状态" clearable style="width: 140px">
              <el-option label="已抓取／待分析" value="pending" />
              <el-option label="待补全简历" value="pending_document" />
              <el-option label="分析中" value="analyzing" />
              <el-option label="已完成" value="done" />
              <el-option label="失败" value="error" />
            </el-select>

            <el-select v-model="sourceFilter" placeholder="全部来源" clearable style="width: 130px">
              <el-option v-for="source in sources" :key="source" :label="source" :value="source" />
            </el-select>

            <el-select v-model="stageFilter" placeholder="招聘阶段" clearable style="width: 130px">
              <el-option
                v-for="(label, key) in stageLabels"
                :key="key"
                :label="label"
                :value="key"
              />
            </el-select>

            <el-button
              v-if="hasActiveFilters"
              link
              type="primary"
              class="reset-filters-btn"
              @click="resetFilters"
            >
              重置筛选
            </el-button>
          </div>

          <div class="toolbar-right-controls">
            <span class="count-summary">
              共 <strong>{{ filtered.length }}</strong> 位候选人
            </span>

            <el-select v-model="sortOrder" aria-label="按得分排序候选人" class="candidate-sort-select" size="small">
              <el-option label="综合评分：高到低" value="score_desc" />
              <el-option label="综合评分：低到高" value="score_asc" />
              <el-option label="入库时间：最新" value="newest" />
            </el-select>

            <el-button size="small" @click="manageMode = !manageMode; selectedCandidateIds = []">
              {{ manageMode ? '退出管理' : '管理简历' }}
            </el-button>
            <template v-if="manageMode">
              <el-button size="small" @click="selectVisibleCandidates">全选当前结果</el-button>
              <el-button size="small" type="primary" plain :disabled="!selectedCandidateIds.length" @click="startBatchOutreach(selectedCandidateIds)">
                批量打招呼（{{ selectedCandidateIds.length }}）
              </el-button>
              <el-button size="small" type="danger" plain :disabled="!selectedCandidateIds.length" @click="archiveSelectedCandidates">
                批量删除（{{ selectedCandidateIds.length }}）
              </el-button>
            </template>

            <!-- 双视图模式切换 (Card / Table) -->
            <el-radio-group v-model="viewMode" size="small" class="view-mode-toggle">
              <el-radio-button label="grid" title="卡片看板视图">
                <el-icon><Grid /></el-icon>
                <span class="toggle-text">卡片</span>
              </el-radio-button>
              <el-radio-button label="table" title="紧凑数据表格视图">
                <el-icon><Tickets /></el-icon>
                <span class="toggle-text">表格</span>
              </el-radio-button>
            </el-radio-group>
          </div>
        </div>

        <!-- 视图形态一：候选人卡片看板网格 (Grid View) -->
        <div v-if="filtered.length && viewMode === 'grid'" class="wb-candidate-grid">
          <article
            v-for="(item, index) in filtered"
            :key="item.projectCandidateId"
            class="wb-card wb-candidate talent-card"
          >
            <div class="wb-candidate-top">
              <el-checkbox
                v-if="manageMode"
                :model-value="selectedCandidateIds.includes(item.projectCandidateId)"
                :aria-label="`选择 ${item.name}`"
                @change="toggleCandidateSelection(item.projectCandidateId)"
              />
              <div class="candidate-avatar-col">
                <span class="wb-avatar candidate-sequence" :title="`当前列表第 ${index + 1} 位 · ${item.name}`">
                  {{ getCandidateSurname(item) }}
                </span>
                <span class="candidate-seq-badge">#{{ index + 1 }}</span>
              </div>
              <div class="candidate-headline">
                <div class="candidate-name-row">
                  <span class="wb-candidate-name" :title="item.name">{{ item.name }}</span>
                  <span v-if="isFreshGradCandidate(item)" class="fresh-grad-tag">应届生</span>
                </div>

                <!-- 有工作经历：岗位与经历各分一行 -->
                <template v-if="!isFreshGradCandidate(item)">
                  <div class="candidate-info-row role-row" :title="`岗位：${getCandidateExperience(item).role}`">
                    <span class="row-label">岗位：</span>
                    <span class="row-val role-val">{{ getCandidateExperience(item).role }}</span>
                  </div>
                  <div class="candidate-info-row exp-row" :title="`经历：${getCandidateExperience(item).company}`">
                    <span class="row-label">经历：</span>
                    <span class="row-val company-val">{{ getCandidateExperience(item).company }}</span>
                  </div>
                </template>

                <!-- 没有经历（应届生）：专业和学校各分一行，标注应届生 -->
                <template v-else>
                  <div class="candidate-info-row major-row" :title="`专业：${getCandidateEducation(item).major}`">
                    <span class="row-label">专业：</span>
                    <span class="row-val major-val">{{ getCandidateEducation(item).major }}</span>
                  </div>
                  <div class="candidate-info-row school-row" :title="`学校：${getCandidateEducation(item).school}${getCandidateEducation(item).degree ? ' (' + getCandidateEducation(item).degree + ')' : ''}`">
                    <span class="row-label">学校：</span>
                    <span class="row-val school-val">{{ getCandidateEducation(item).school }}</span>
                    <span v-if="getCandidateEducation(item).degree" class="edu-degree-tag">{{ getCandidateEducation(item).degree }}</span>
                  </div>
                </template>

                <div class="candidate-target-role">目标：{{ item.jobTitle || item.projectName }}</div>
              </div>
              <div class="candidate-score-wrap">
                <div v-if="item.analysisStatus === 'done' && (item.abilityScore > 0 || item.matchScore > 0)"
                  class="candidate-match-ring" :style="{ '--match': `${item.abilityScore || item.matchScore}%` }"
                  role="img" :aria-label="`综合评分 ${item.abilityScore || item.matchScore} 分`"
                  :title="`综合评分：${item.abilityScore || item.matchScore} 分`">
                  <span>{{ item.abilityScore || item.matchScore }}</span>
                </div>
                <div v-else class="candidate-match-pending">待评</div>
                <small>综合评分</small>
              </div>
            </div>

            <div class="wb-candidate-meta">
              <span class="wb-pill source-pill">{{ item.source }}</span>
              <span class="wb-pill" :class="statusClass(item.analysisStatus)">
                {{ statusLabel(item.analysisStatus) }}
              </span>
              <span class="wb-pill stage-pill">{{ stageLabels[item.stage] || item.stage }}</span>
            </div>

            <div class="candidate-skill-list" aria-label="已核对的能力标签">
              <span v-for="skill in item.skills || []" :key="skill" class="candidate-skill-chip" :title="skill">{{ skill }}</span>
              <span v-if="!item.skills?.length" class="candidate-skill-empty">技能证据待补充</span>
            </div>

            <div class="wb-candidate-foot">
              <span class="candidate-created-time">入库 {{ formatDate(item.createdAt) }}<span v-if="item.analysisStatus === 'done' && (item.abilityScore > 0 || item.matchScore > 0)"> · 综合评分 {{ item.abilityScore || item.matchScore }}</span></span>
              <div class="candidate-actions">
                <el-button class="candidate-primary-action" size="small" type="primary" plain @click="navigateToEvaluation(item)">查看测评</el-button>
                <el-button class="candidate-secondary-action" size="small" type="primary" @click="openInterviewInvite(item)">
                  <el-icon style="margin-right: 3px;"><Calendar /></el-icon>邀约面试
                </el-button>
                <el-dropdown trigger="click" @command="(command: string) => handleCandidateAction(command, item)">
                  <el-button size="small" class="candidate-more-action" aria-label="更多候选人操作">更多<el-icon><ArrowDown /></el-icon></el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="move">调配分库</el-dropdown-item>
                      <el-dropdown-item v-if="item.analysisStatus === 'error'" command="retry">重新分析</el-dropdown-item>
                      <el-dropdown-item command="archive" divided>删除岗位简历</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </div>
          </article>
        </div>

        <!-- 视图形态二：高密数据表格视图 (Table View) -->
        <div v-else-if="filtered.length && viewMode === 'table'" class="talent-table-container">
          <el-table
            :data="filtered"
            stripe
            highlight-current-row
            class="talent-data-table"
            @row-click="handleTableRowClick"
          >
            <el-table-column v-if="manageMode" label="选择" width="58" align="center">
              <template #default="{ row }">
                <el-checkbox
                  :model-value="selectedCandidateIds.includes(row.projectCandidateId)"
                  :aria-label="`选择 ${row.name}`"
                  @click.stop
                  @change="toggleCandidateSelection(row.projectCandidateId)"
                />
              </template>
            </el-table-column>
            <el-table-column type="index" label="#" width="60" align="center">
              <template #default="{ $index }">
                <span class="table-seq">#{{ $index + 1 }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="name" label="姓名" min-width="130">
              <template #default="{ row }">
                <div class="table-name-cell">
                  <span class="table-candidate-name">{{ row.name }}</span>
                  <span v-if="isFreshGradCandidate(row)" class="fresh-grad-tag table-fresh-tag">应届生</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="背景履历" min-width="200">
              <template #default="{ row }">
                <div v-if="!isFreshGradCandidate(row)" class="table-exp-box">
                  <span class="table-role-title">岗位：{{ getCandidateExperience(row).role }}</span>
                  <span class="table-company-name">经历：{{ getCandidateExperience(row).company }}</span>
                </div>
                <div v-else class="table-exp-box">
                  <span class="table-role-title">专业：{{ getCandidateEducation(row).major }}</span>
                  <span class="table-company-name">学校：{{ getCandidateEducation(row).school }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="意向岗位 / 分库" min-width="180">
              <template #default="{ row }">
                <div class="table-job-box">
                  <span class="table-job-title">{{ row.jobTitle || row.projectName }}</span>
                  <span class="table-job-dept">{{ row.projectName }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="综合评分" width="110" align="center">
              <template #default="{ row }">
                <span
                  v-if="row.analysisStatus === 'done' && (row.abilityScore > 0 || row.matchScore > 0)"
                  class="score-badge"
                  :class="scoreLevelClass(row.abilityScore || row.matchScore)"
                >
                  {{ row.abilityScore || row.matchScore }} 分
                </span>
                <span v-else class="score-badge pending">待评</span>
              </template>
            </el-table-column>

            <el-table-column label="招聘阶段" width="110" align="center">
              <template #default="{ row }">
                <span class="wb-pill stage-pill" style="font-size: 11px;">
                  {{ stageLabels[row.stage] || row.stage }}
                </span>
              </template>
            </el-table-column>

            <el-table-column prop="source" label="来源渠道" width="110" align="center">
              <template #default="{ row }">
                <span class="table-source-tag">{{ row.source }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="owner" label="负责人" width="110">
              <template #default="{ row }">
                <span class="table-owner-text">{{ row.owner || '未分配' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="createdAt" label="入库时间" width="150">
              <template #default="{ row }">
                <span class="table-date-text">{{ formatDate(row.createdAt) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="250" fixed="right" align="center">
              <template #default="{ row }">
                <div class="table-row-actions" @click.stop>
                  <el-button link type="primary" size="small" @click="navigateToEvaluation(row)">查看测评</el-button>
                  <el-button link type="primary" size="small" @click="openInterviewInvite(row)">邀约面试</el-button>
                  <el-button link type="primary" size="small" @click="openMoveDialog(row)">调配</el-button>
                  <el-button link type="danger" size="small" @click="archiveOneCandidate(row)">删除</el-button>
                  <el-button
                    v-if="row.analysisStatus === 'error'"
                    link
                    type="warning"
                    size="small"
                    @click="retryAnalysis(row.resumeId)"
                  >
                    重试
                  </el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 空数据提示 -->
        <div v-else class="wb-card wb-empty empty-candidate-panel">
          <p v-if="recruitment.loading" class="empty-loading">
            <el-icon class="is-loading" style="font-size: 26px; margin-bottom: 10px;"><Loading /></el-icon>
            <span>正在载入人才库数据…</span>
          </p>
          <div v-else class="empty-detail-box">
            <div class="empty-illustration">
              <el-icon><Search /></el-icon>
            </div>
            <h3 class="empty-title">
              {{ selectedProject ? `「${selectedProject.job_config?.title || selectedProject.name}」分库中暂无符合条件的候选人` : '当前筛选条件下暂无候选人' }}
            </h3>
            <p class="empty-subtitle">
              该职位专属分库已就绪。点击下方按钮即可一键调起多平台矩阵寻才，或直接导入 51job 投递简历
            </p>
            <el-button
              v-if="selectedProject"
              type="primary"
              size="large"
              class="empty-action-btn"
              @click="showSearch = true"
            >
              <el-icon style="margin-right: 6px;"><Position /></el-icon>
              立即为该岗位检索人才
            </el-button>
          </div>
        </div>
      </main>
    </div>

    <!-- 邀约面试与沟通文案对话框 -->
    <el-dialog
      v-model="inviteDialogOpen"
      title="邀约面试 · 沟通意向与日程安排"
      width="min(720px, 94vw)"
      class="interview-invite-dialog"
      :close-on-click-modal="!savingInterview && !generatingGreeting"
    >
      <div v-if="inviteCandidate" class="invite-dialog-body">
        <!-- 头部候选人概览卡片 -->
        <div class="invite-candidate-header">
          <div class="invite-avatar">{{ getCandidateSurname(inviteCandidate) }}</div>
          <div class="invite-info">
            <div class="invite-name-row">
              <span class="invite-name">{{ inviteCandidate.name }}</span>
              <span class="wb-pill source-pill">{{ inviteCandidate.source }}</span>
              <span class="wb-pill stage-pill">{{ stageLabels[inviteCandidate.stage] || inviteCandidate.stage }}</span>
            </div>
            <div class="invite-sub-text">
              <span>应聘分库：<strong>{{ inviteCandidate.jobTitle || inviteCandidate.projectName }}</strong></span>
              <span v-if="(inviteCandidate.abilityScore || inviteCandidate.matchScore) > 0"> · 综合评分：<strong>{{ inviteCandidate.abilityScore || inviteCandidate.matchScore }}分</strong></span>
            </div>
          </div>
          <el-button
            v-if="inviteCandidate.profileUrl"
            size="small"
            link
            type="primary"
            @click="openProfile(inviteCandidate.profileUrl)"
          >
            打开平台主页
          </el-button>
        </div>

        <!-- 模块一：✨ 一键打招呼与 AI 深度定制沟通文案 -->
        <div class="invite-section-box">
          <div class="invite-section-header">
            <div class="section-title">
              <span class="icon">✨</span>
              <strong>一键打招呼 / 定制沟通意向</strong>
            </div>
            <div class="section-actions">
              <el-button
                size="small"
                type="primary"
                plain
                :loading="generatingGreeting"
                @click="generateGreeting(inviteCandidate)"
              >
                {{ greetingMessage ? '重新生成招呼语' : 'AI 生成招呼语' }}
              </el-button>
              <el-button
                v-if="greetingMessage"
                size="small"
                @click="copyGreeting"
              >
                复制招呼文案
              </el-button>
            </div>
          </div>
          <p class="section-hint">
            基于候选人真实项目与工作经历，由接入的 AI 模型深度定制契合岗位方向的真诚招呼文案；可直接复制后在平台对话窗口发送。
          </p>
          <el-input
            v-model="greetingMessage"
            type="textarea"
            :rows="3"
            :maxlength="600"
            show-word-limit
            placeholder="点击上方按钮生成沟通文案，或在此手动输入..."
          />
        </div>

        <!-- 模块二：📅 面试日程预约表单 -->
        <div class="invite-section-box">
          <div class="invite-section-header">
            <div class="section-title">
              <span class="icon">📅</span>
              <strong>预约面试日程</strong>
            </div>
          </div>
          <div class="wb-form-grid" style="margin-top: 10px;">
            <div>
              <label>面试开始时间 <span class="required">*</span></label>
              <input v-model="interviewForm.startsAt" type="datetime-local" class="invite-input" />
            </div>
            <div>
              <label>预计结束时间 <span class="required">*</span></label>
              <input v-model="interviewForm.endsAt" type="datetime-local" class="invite-input" />
            </div>
            <div>
              <label>面试形式</label>
              <select v-model="interviewForm.method" class="invite-select">
                <option value="视频面试">视频面试 (腾讯会议/飞书)</option>
                <option value="现场面试">现场面试 (公司职场)</option>
                <option value="电话面试">电话初试</option>
              </select>
            </div>
            <div>
              <label>主要面试官</label>
              <input v-model="interviewForm.interviewer" placeholder="如：技术总监 / HRBP" class="invite-input" />
            </div>
            <div class="full">
              <label>面试地点 / 会议链接</label>
              <input v-model="interviewForm.location" placeholder="填写会议号/链接或职场具体会议室" class="invite-input" />
            </div>
            <div class="full">
              <label>邀约备注 (考察重点)</label>
              <textarea v-model="interviewForm.note" rows="2" placeholder="填写本次面试重点核实的专业问题或备忘事项..." class="invite-textarea" />
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="inviteDialogOpen = false">取消</el-button>
        <el-button
          type="primary"
          :loading="savingInterview"
          @click="submitInterviewInvite"
        >
          保存面试安排
        </el-button>
      </template>
    </el-dialog>

    <!-- 调配至其他职位库弹窗 -->
    <el-dialog
      v-model="moveDialogOpen"
      :title="`候选人调配 · ${candidateToMove?.name || ''}`"
      width="480px"
    >
      <p class="wb-muted" style="margin-bottom: 16px;">
        将候选人<strong>【{{ candidateToMove?.name }}】</strong>从当前职位库流转至新的目标职位库。候选人的评测记录、简历版本和跟进轨迹将完整保留。
      </p>
      <el-form label-position="top">
        <el-form-item label="选择目标职位分库 *" required>
          <el-select
            v-model="targetProjectId"
            placeholder="请选择要调配到的目标职位"
            style="width: 100%"
          >
            <el-option
              v-for="proj in targetProjectOptions"
              :key="proj.id"
              :label="`${proj.job_config?.title || proj.name} · ${proj.department || '未分配'}`"
              :value="proj.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="moveDialogOpen = false">取消</el-button>
        <el-button type="primary" :disabled="!targetProjectId" :loading="moving" @click="confirmMoveCandidate">
          确认调配
        </el-button>
      </template>
    </el-dialog>

    <!-- 寻才与打招呼对话框 -->
    <TopTenOutreachDialog
      v-model="showOutreach"
      :project-id="selectedProjectId"
      :candidate-ids="outreachCandidateIds"
      @updated="refresh"
    />
    <BossSearchDialog
      v-if="selectedProject"
      v-model="showSearch"
      :project-id="selectedProject.id"
      :project-name="selectedProject.name"
      :job-title="selectedProject.job_config?.title || selectedProject.name"
      :exp-years="selectedProject.job_config?.experience_years || 0"
      :edu-level="selectedProject.job_config?.education_level || ''"
      @refresh="refresh"
    />

    <!-- 待分配投递抽屉 -->
    <el-drawer v-model="inboxOpen" title="主动投递 · 待分配" size="500px">
      <div v-if="!pendingApplications.length" class="wb-empty">暂无待分配投递</div>
      <article
        v-for="application in pendingApplications"
        :key="application.id"
        class="wb-card"
        style="margin-bottom: 12px"
      >
        <strong>{{ application.candidateName }}</strong>
        <p class="wb-muted">
          51job 职位 ID：{{ application.platformJobId || '平台未返回' }} · 投递 ID：{{ application.applicationId }}
        </p>
        <div class="wb-actions">
          <el-select
            v-model="assignment[application.id]"
            placeholder="选择正确项目"
            style="width: 230px"
          >
            <el-option
              v-for="project in projects.projects"
              :key="project.id"
              :label="`${project.job_config.title || project.name} · ${project.name}`"
              :value="project.id"
            />
          </el-select>
          <el-button
            type="primary"
            :disabled="!assignment[application.id]"
            @click="assign(application.id)"
          >
            确认分配
          </el-button>
        </div>
      </article>
    </el-drawer>

    <!-- 疑似重复抽屉 -->
    <el-drawer v-model="duplicatesOpen" title="疑似重复 · HR 核对" size="500px">
      <div class="duplicate-drawer-toolbar">
        <span>仅清理核对提示，候选人档案与简历保留。</span>
        <el-button size="small" type="danger" plain :disabled="!duplicates.length" @click="clearDuplicateHints">一键清空</el-button>
      </div>
      <div v-if="!duplicates.length" class="wb-empty">暂无疑似重复档案</div>
      <article
        v-for="pair in duplicates"
        :key="pair.candidateId + pair.otherCandidateId"
        class="wb-card"
        style="margin-bottom: 12px"
      >
        <strong>{{ pair.name }}</strong>
        <p class="wb-muted">{{ pair.reason }}</p>
        <p class="wb-muted">
          档案 A：{{ pair.candidateId }}<br />
          档案 B：{{ pair.otherCandidateId }}
        </p>
        <el-input
          v-model="mergeReasons[pair.candidateId + pair.otherCandidateId]"
          placeholder="请填写已核实的合并依据"
        />
        <el-button
          style="margin-top: 8px"
          :disabled="!mergeReasons[pair.candidateId + pair.otherCandidateId]?.trim()"
          @click="merge(pair)"
        >
          确认将 B 合并至 A
        </el-button>
        <el-button style="margin-top: 8px" type="danger" plain @click="dismissDuplicateHint(pair)">删除提示</el-button>
      </article>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Refresh,
  Position,
  Search,
  Plus,
  Collection,
  Grid,
  Tickets,
  Download,
  FolderOpened,
  CopyDocument,
  ArrowDown,
  ChatRound,
  CircleCheck,
  CircleClose,
  Warning,
  Loading,
  Document,
  Calendar
} from '@element-plus/icons-vue'
import { useProjectStore, type Project } from '../composables/useProjectStore'
import { useRecruitmentStore, stageLabels, type TalentCandidate } from '../composables/useRecruitmentStore'
import BossSearchDialog from '../components/BossSearchDialog.vue'
import TopTenOutreachDialog from '../components/TopTenOutreachDialog.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getCandidateSurname } from '../utils/candidate'

interface PendingApplication { id: string; applicationId: string; candidateName: string; platformJobId: string }
interface DuplicatePair { candidateId: string; otherCandidateId: string; name: string; reason: string }
const route = useRoute()
const router = useRouter()
const projects = useProjectStore()
const recruitment = useRecruitmentStore()

const selectedProjectId = ref(String(route.query.project || ''))
const repoFilterKeyword = ref('')
const keyword = ref('')
const statusFilter = ref('')
const sourceFilter = ref('')
const stageFilter = ref('')
const viewMode = ref<'grid' | 'table'>('grid')
const sortOrder = ref<'score_desc' | 'score_asc' | 'newest'>('score_desc')
const manageMode = ref(false)
const selectedCandidateIds = ref<string[]>([])

function toggleCandidateSelection(id: string) {
  selectedCandidateIds.value = selectedCandidateIds.value.includes(id)
    ? selectedCandidateIds.value.filter(item => item !== id)
    : [...selectedCandidateIds.value, id]
}

function selectVisibleCandidates() {
  selectedCandidateIds.value = filtered.value.map(item => item.projectCandidateId)
}

async function archiveCandidates(ids: string[]) {
  if (!ids.length) return
  try {
    await ElMessageBox.confirm(
      `确认从人才库删除 ${ids.length} 条岗位简历？统一候选人档案、原始简历和沟通记录会保留。`,
      '删除简历',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  try {
    const removed = await recruitment.archiveCandidates(ids)
    selectedCandidateIds.value = selectedCandidateIds.value.filter(id => !ids.includes(id))
    ElMessage.success(`已从人才库删除 ${removed} 条岗位简历`)
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

function archiveOneCandidate(item: TalentCandidate) {
  void archiveCandidates([item.projectCandidateId])
}

function archiveSelectedCandidates() {
  void archiveCandidates(selectedCandidateIds.value)
}

function handleCandidateAction(command: string, item: TalentCandidate) {
  if (command === 'move') openMoveDialog(item)
  if (command === 'retry') void retryAnalysis(item.resumeId)
  if (command === 'archive') archiveOneCandidate(item)
}

const hasActiveFilters = computed(() => {
  return !!(keyword.value || statusFilter.value || sourceFilter.value || stageFilter.value)
})

function resetFilters() {
  keyword.value = ''
  statusFilter.value = ''
  sourceFilter.value = ''
  stageFilter.value = ''
}

const showSearch = ref(false)
const showOutreach = ref(false)
const outreachCandidateIds = ref<string[]>([])

function startBatchOutreach(ids?: string[]) {
  if (ids && ids.length > 0) {
    outreachCandidateIds.value = [...ids]
  } else if (hasActiveFilters.value && filtered.value.length > 0) {
    outreachCandidateIds.value = filtered.value.map(item => item.projectCandidateId || item.id)
  } else {
    outreachCandidateIds.value = []
  }
  showOutreach.value = true
}
const importing = ref(false)
const inboxOpen = ref(false)
const pendingApplications = ref<PendingApplication[]>([])
const assignment = ref<Record<string, string>>({})
const duplicates = ref<DuplicatePair[]>([])
const duplicatesOpen = ref(false)
const mergeReasons = ref<Record<string, string>>({})
const pendingReviewCount = computed(() => pendingApplications.value.length + duplicates.value.length)

function handleLibraryAction(command: string) {
  if (command === 'import') void import51Job()
  if (command === 'inbox') showInbox()
  if (command === 'duplicates') showDuplicates()
}

// 点击查看评测跳转到对应候选人的简历预览界面
function navigateToEvaluation(item: TalentCandidate) {
  router.push({
    path: '/project/' + item.projectId,
    query: { resume: item.resumeId || '' }
  })
}

// 邀约面试与一键打招呼弹窗状态
const inviteDialogOpen = ref(false)
const inviteCandidate = ref<TalentCandidate | null>(null)
const greetingMessage = ref('')
const generatingGreeting = ref(false)
const savingInterview = ref(false)

const interviewForm = reactive({
  startsAt: '',
  endsAt: '',
  method: '视频面试',
  interviewer: '',
  location: '',
  note: ''
})

function formatDateTimeInput(date: Date): string {
  const offset = date.getTimezoneOffset() * 60000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

async function openInterviewInvite(candidate: TalentCandidate) {
  inviteCandidate.value = candidate
  greetingMessage.value = ''
  generatingGreeting.value = false
  savingInterview.value = false

  // 默认面试时间：明天上午 10:00 - 11:00
  const tomorrow = new Date()
  tomorrow.setDate(tomorrow.getDate() + 1)
  tomorrow.setHours(10, 0, 0, 0)
  const tomorrowEnd = new Date(tomorrow)
  tomorrowEnd.setHours(11, 0, 0, 0)

  Object.assign(interviewForm, {
    startsAt: formatDateTimeInput(tomorrow),
    endsAt: formatDateTimeInput(tomorrowEnd),
    method: '视频面试',
    interviewer: candidate.owner || 'HRBP',
    location: '',
    note: ''
  })

  inviteDialogOpen.value = true

  // 自动为候选人生成一份 AI 定制打招呼语
  void generateGreeting(candidate)
}

async function generateGreeting(candidate: TalentCandidate) {
  if (generatingGreeting.value) return
  generatingGreeting.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const msg = await api.GenerateCandidateGreetingMessage(candidate.projectId, candidate.resumeId)
    if (msg) {
      greetingMessage.value = msg
    }
  } catch (err: any) {
    const surname = getCandidateSurname(candidate)
    const targetJob = candidate.jobTitle || candidate.projectName || '目标岗位'
    greetingMessage.value = `您好${surname}，我们正在招聘【${targetJob}】。看到了您提供的经历，想进一步了解您是否愿意交流这个机会，方便聊聊吗？`
  } finally {
    generatingGreeting.value = false
  }
}

async function copyGreeting() {
  if (!greetingMessage.value.trim()) return
  try {
    await navigator.clipboard.writeText(greetingMessage.value.trim())
    ElMessage.success('打招呼文案已复制到剪贴板，可直接在平台聊天窗口粘贴发送！')
  } catch {
    ElMessage.info('请手动选中文本框复制文案')
  }
}

async function submitInterviewInvite() {
  if (!inviteCandidate.value) return
  if (!interviewForm.startsAt || !interviewForm.endsAt) {
    ElMessage.warning('请选择面试开始与结束时间')
    return
  }
  if (new Date(interviewForm.endsAt) <= new Date(interviewForm.startsAt)) {
    ElMessage.warning('面试结束时间需晚于开始时间')
    return
  }
  savingInterview.value = true
  let scheduleSaved = false
  try {
    const candidate = inviteCandidate.value
    // 1. 保存面试日程
    await recruitment.saveInterview({
      id: '',
      projectCandidateId: candidate.projectCandidateId,
      candidateName: candidate.name,
      projectName: candidate.jobTitle || candidate.projectName,
      startsAt: new Date(interviewForm.startsAt).toISOString(),
      endsAt: new Date(interviewForm.endsAt).toISOString(),
      interviewer: interviewForm.interviewer.trim(),
      method: interviewForm.method,
      location: interviewForm.location.trim(),
      note: interviewForm.note.trim() ? `${interviewForm.note.trim()}\n[招呼文案]: ${greetingMessage.value}` : greetingMessage.value,
      status: 'scheduled'
    })
    scheduleSaved = true

    // 已安排首轮面试，使用招聘流程内有效的阶段值。
    await recruitment.updateFollowUp(
      candidate,
      'first_interview',
      interviewForm.interviewer.trim() || candidate.owner || 'HR',
      interviewForm.startsAt,
      `已保存面试安排：${interviewForm.method}，时间：${interviewForm.startsAt}；请与候选人确认。`
    )

    ElMessage.success('面试安排已保存，候选人已进入初面阶段；请在平台确认邀约。')
    inviteDialogOpen.value = false
    await refresh()
  } catch (err: any) {
    ElMessage.error(scheduleSaved ? `面试安排已保存，但阶段更新失败：${err?.message || String(err)}` : err?.message || String(err))
    if (scheduleSaved) {
      inviteDialogOpen.value = false
      await refresh()
    }
  } finally {
    savingInterview.value = false
  }
}

// 调配分库逻辑
const moveDialogOpen = ref(false)
const candidateToMove = ref<TalentCandidate | null>(null)
const targetProjectId = ref('')
const moving = ref(false)

const selectedProject = computed(() =>
  projects.projects.find(p => p.id === selectedProjectId.value)
)

const filteredProjectRepos = computed(() => {
  return projects.projects.filter(p => {
    if (!repoFilterKeyword.value) return true
    const kw = repoFilterKeyword.value.toLowerCase()
    const name = (p.job_config?.title || p.name || '').toLowerCase()
    const dept = (p.department || '').toLowerCase()
    return name.includes(kw) || dept.includes(kw)
  })
})

const targetProjectOptions = computed(() => {
  if (!candidateToMove.value) return projects.projects
  return projects.projects.filter(p => p.id !== candidateToMove.value?.projectId)
})

const sources = computed(() => [...new Set(recruitment.candidates.map(c => c.source))].sort())

const filtered = computed(() => recruitment.candidates.filter(item =>
  (!selectedProjectId.value || item.projectId === selectedProjectId.value) &&
  (!statusFilter.value || item.analysisStatus === statusFilter.value) &&
  (!sourceFilter.value || item.source === sourceFilter.value) &&
  (!stageFilter.value || item.stage === stageFilter.value) &&
  (!keyword.value || `${item.name} ${item.projectName} ${item.jobTitle}`.toLowerCase().includes(keyword.value.toLowerCase()))
).sort((a, b) => {
  const recent = (b.createdAt || '').localeCompare(a.createdAt || '')
  if (sortOrder.value === 'newest') return recent || a.projectCandidateId.localeCompare(b.projectCandidateId)
  if (a.analysisStatus !== b.analysisStatus) return a.analysisStatus === 'done' ? -1 : b.analysisStatus === 'done' ? 1 : 0
  const scoreA = a.abilityScore || a.matchScore || 0
  const scoreB = b.abilityScore || b.matchScore || 0
  const score = sortOrder.value === 'score_desc'
    ? scoreB - scoreA
    : scoreA - scoreB
  return score || recent || a.projectCandidateId.localeCompare(b.projectCandidateId)
}))

watch(() => route.query.project, value => {
  selectedProjectId.value = String(value || '')
})



function countFor(id: string): number {
  return recruitment.candidates.filter(item => item.projectId === id).length
}

function selectRepository(id: string) {
  selectedProjectId.value = id
  syncRoute()
}

function syncRoute() {
  void router.replace({
    path: '/talent',
    query: selectedProjectId.value ? { project: selectedProjectId.value } : {}
  })
}

function openMoveDialog(candidate: TalentCandidate) {
  candidateToMove.value = candidate
  targetProjectId.value = ''
  moveDialogOpen.value = true
}

async function confirmMoveCandidate() {
  if (!candidateToMove.value || !targetProjectId.value) return
  moving.value = true
  try {
    await recruitment.moveCandidate(candidateToMove.value.projectCandidateId, targetProjectId.value)
    ElMessage.success('候选人已成功调配至目标职位分库！')
    moveDialogOpen.value = false
    candidateToMove.value = null
    await refresh()
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    moving.value = false
  }
}

function handleTableRowClick(row: TalentCandidate) {
  void navigateToEvaluation(row)
}

function scoreLevelClass(score: number): string {
  if (score >= 80) return 'score-high'
  if (score >= 60) return 'score-mid'
  return 'score-low'
}

function normalizeRepoStatusClass(status?: string): string {
  if (!status || status === 'publishing' || status === 'analyzing' || status === 'completed') return 'status-dot-publishing'
  if (status === 'draft') return 'status-dot-draft'
  if (status === 'paused') return 'status-dot-paused'
  return 'status-dot-publishing'
}

function normalizeStatusLabel(status?: string): string {
  if (!status || status === 'publishing' || status === 'analyzing' || status === 'completed') return '招聘中'
  if (status === 'draft') return '草稿'
  if (status === 'paused') return '已暂停'
  return status
}

function statusLabel(status: string) {
  return ({
    pending: '已抓取／待分析',
    pending_document: '待补全简历',
    analyzing: '分析中',
    done: '已完成',
    error: '失败'
  } as Record<string, string>)[status] || status
}

function statusClass(status: string) {
  return status === 'error' ? 'danger' : status === 'done' ? 'ok' : status === 'analyzing' ? 'warn' : ''
}

function redLineLabel(status: string) {
  return ({
    pending: '待核实',
    failed: '不满足',
    passed: '已满足',
    not_configured: '未配置'
  } as Record<string, string>)[status] || '待核实'
}

function redLineClass(status: string) {
  return status === 'failed' ? 'danger' : status === 'passed' ? 'ok' : 'warn'
}

function formatDate(value: string) {
  return value ? new Date(value).toLocaleString('zh-CN') : ''
}

function formatRecentRole(role: string) {
  return role.replace(/\s*[@＠]\s*/, ' · ')
}

// 判断候选人是否为应届生或无工作经历（有工作经历优先显示经历）
function isFreshGradCandidate(item: any): boolean {
  if (!item) return false
  const role = (item.currentRole || '').trim()
  const workYears = (item.workYears || '').trim()

  // 1. 如果有明确的岗位或经历（且不是“无”、“暂无”、“待补充”等占位词），优先视为有工作经历！
  if (role && !/^(无|暂无|最近经历待补充|无工作经历|待补充|无经历)$/i.test(role)) {
    if (!/^(应届生?|在校生?|学生|实习生|全日制在读)$/i.test(role)) {
      return false // 明确有工作岗位/经历，优先显示经历，绝不判断为应届生！
    }
  }

  // 2. 如果工作年限明确大于0年（如 1年、3年、10年），绝不能判断为应届生（注意避免 /0年/ 误伤 10年、20年）
  const yearsMatch = workYears.match(/^(\d+(?:\.\d+)?)\s*年?$/)
  if (yearsMatch && parseFloat(yearsMatch[1]) > 0) {
    return false
  }

  // 3. 只有在没有经历或岗位为空/无经历的情况下，且明确包含“应届/在校/毕业”等关键词，才判定为应届生
  if (/应届|在校|在读|学生|毕业|fresh/i.test(workYears) || /应届|在校生|在读/i.test(role)) {
    return true
  }

  return false
}

// 提取往期经历：岗位与经历/公司分别提取
function getCandidateExperience(item: any): { role: string; company: string } {
  const rawRole = (item?.currentRole || '').trim()
  if (!rawRole || isFreshGradCandidate(item)) {
    return { role: '暂无岗位记录', company: '暂无公司经历' }
  }

  // 1. 如果包含 @ 或 ＠ 分隔符（如 "高级前端工程师 @ 阿里巴巴"）
  if (/[@＠]/.test(rawRole)) {
    const parts = rawRole.split(/[@＠]/).map((s: string) => s.trim())
    return {
      role: parts[0] || '岗位待核对',
      company: parts.slice(1).join(' @ ') || '公司待核对'
    }
  }

  // 2. 如果包含斜杠 / 或 ／
  if (/[/／]/.test(rawRole)) {
    const parts = rawRole.split(/[/／]/).map((s: string) => s.trim())
    const isCompany = (s: string) => /公司|集团|科技|药业|有限|事务所|研究所|厂|银行|证券|中心|学校|医院/i.test(s)
    if (parts.length >= 2) {
      if (isCompany(parts[1]) && !isCompany(parts[0])) {
        return { role: parts[0], company: parts[1] }
      }
      if (isCompany(parts[0]) && !isCompany(parts[1])) {
        return { role: parts[1], company: parts[0] }
      }
      return { role: parts[0], company: parts.slice(1).join(' / ') }
    }
  }

  // 3. 如果包含竖线 | 或 ｜
  if (/[|｜]/.test(rawRole)) {
    const parts = rawRole.split(/[|｜]/).map((s: string) => s.trim())
    if (parts.length >= 2) {
      return { role: parts[0], company: parts.slice(1).join(' | ') }
    }
  }

  // 4. 如果包含显式连字符（如 "腾讯科技 - 前端架构师" 或 "前端架构师 - 腾讯科技"）
  if (/\s+[-–—]\s+/.test(rawRole)) {
    const parts = rawRole.split(/\s+[-–—]\s+/).map((s: string) => s.trim())
    const isCompany = (s: string) => /公司|集团|科技|药业|有限|事务所|研究所|厂|银行|证券|中心|学校|医院/i.test(s)
    if (parts.length >= 2) {
      if (isCompany(parts[0]) && !isCompany(parts[1])) {
        return { role: parts[1], company: parts[0] }
      }
      return { role: parts[0], company: parts.slice(1).join(' - ') }
    }
  }

  // 5. 如果是括号形式（如 "高级前端工程师 (腾讯科技)"）
  const parenMatch = rawRole.match(/^(.*?)[（(](.*?)[)）]$/)
  if (parenMatch) {
    return {
      role: parenMatch[1].trim() || '岗位待核对',
      company: parenMatch[2].trim() || '公司待核对'
    }
  }

  // 6. 纯岗位文本，公司标注为待标注
  return {
    role: rawRole,
    company: '往期任职公司未标注'
  }
}

// 提取应届生/无经历候选人的专业与学校
function getCandidateEducation(item: any): { major: string; school: string; degree: string } {
  const rawEdu = (item?.education || '').trim()
  if (!rawEdu) {
    return { major: '专业信息待补充', school: '院校信息待补充', degree: '' }
  }

  // 常见学历关键词
  const degrees = ['博士后', '博士', '硕士', '研究生', 'MBA', 'EMBA', '本科', '学士', '大专', '专科', '中专', '高中']
  let detectedDegree = ''
  for (const deg of degrees) {
    if (rawEdu.includes(deg)) {
      detectedDegree = deg
      break
    }
  }

  // 按常见分隔符拆分：-, ·, /, |, 空格, 逗号
  const parts = rawEdu.split(/[-·／/|｜,，\s]+/).map((s: string) => s.trim()).filter(Boolean)

  let school = ''
  let major = ''

  for (const part of parts) {
    // 过滤纯学历关键词
    if (degrees.includes(part)) {
      continue
    }
    // 识别学校（含大学、学院、学校、分校、研究院、研究所、分院、高职、中专、附中）
    if (/大学|学院|学校|分校|研究院|研究所|分院|高职|附中/i.test(part) && !school) {
      school = part
      continue
    }
    // 识别专业
    if (!major) {
      major = part
    }
  }

  // 如果未能识别出学校或专业，按位置兜底
  if (!school && parts.length > 0) {
    school = parts.find((p: string) => !degrees.includes(p)) || '院校待核对'
  }
  if (!major && parts.length > 1) {
    major = parts.filter((p: string) => !degrees.includes(p) && p !== school)[0] || '专业待核对'
  }

  return {
    major: major || '专业待补充',
    school: school || '院校待补充',
    degree: detectedDegree
  }
}

function openProfile(url: string) {
  window.open(url, '_blank', 'noopener,noreferrer')
}

async function refresh() {
  await Promise.all([projects.loadProjects(), recruitment.loadCandidates()])
}

async function loadInbox(interactive = false) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    pendingApplications.value = (await api.ListImportedApplications('unassigned')) || []
  } catch (err: any) {
    console.warn('loadInbox error:', err)
    if (interactive) ElMessage.error(err?.message || String(err))
  }
}

function showInbox() {
  inboxOpen.value = true
  void loadInbox(true)
}

async function import51Job() {
  importing.value = true
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const result = await api.ImportApplications('51job')
    ElMessage.success(`新增 ${result.addedCount}，重复 ${result.duplicateCount}，待分配 ${result.unassignedCount}，失败 ${result.failedCount}`)
    await Promise.all([refresh(), loadInbox()])
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  } finally {
    importing.value = false
  }
}

async function assign(id: string) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.AssignApplication(id, assignment.value[id])
    ElMessage.success('投递已分配')
    await Promise.all([refresh(), loadInbox()])
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

async function loadDuplicates(interactive = false) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    duplicates.value = (await api.ListSuspectedDuplicates()) || []
  } catch (err: any) {
    console.warn('loadDuplicates error:', err)
    if (interactive) ElMessage.error(err?.message || String(err))
  }
}

function showDuplicates() {
  duplicatesOpen.value = true
  void loadDuplicates(true)
}

async function merge(pair: DuplicatePair) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.MergeCandidates(pair.candidateId, pair.otherCandidateId, 'HR', mergeReasons.value[pair.candidateId + pair.otherCandidateId])
    ElMessage.success('档案已合并')
    await Promise.all([loadDuplicates(), refresh()])
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

async function dismissDuplicateHint(pair: DuplicatePair) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.DismissSuspectedDuplicate(pair.candidateId, pair.otherCandidateId)
    await loadDuplicates()
    ElMessage.success('已删除这条核对提示，档案和简历仍保留')
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

async function clearDuplicateHints() {
  if (!duplicates.value.length) return
  try {
    await ElMessageBox.confirm(`清空当前 ${duplicates.value.length} 条疑似重复提示？候选人档案、简历和评测会保留。`,
      '清空疑似重复提示', { confirmButtonText: '确认清空', cancelButtonText: '取消', type: 'warning' })
  } catch { return }
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    const removed: number = await api.ClearSuspectedDuplicates()
    await loadDuplicates()
    ElMessage.success(`已清理 ${removed} 条核对提示`)
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}



async function retryAnalysis(resumeId: string) {
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.RetryCandidateAnalysis(resumeId)
    ElMessage.success('已重新加入分析队列')
    await refresh()
  } catch (err: any) {
    ElMessage.error(err?.message || String(err))
  }
}

onMounted(() => {
  void projects.loadProjects()
  void refresh()
  void loadInbox()
  void loadDuplicates()
})
</script>

<style scoped>
/* 全局页面容器：保证右侧保留充裕的 32px 呼吸间距，支持极端分辨率横向自适应 */
.talent-pool-page {
  box-sizing: border-box;
  width: 100%;
  padding: 0 32px 60px 24px;
  overflow-x: hidden;
}

/* 顶栏随内容增高，避免继承工作台固定高度后覆盖筛选区 */
.talent-page-header {
  height: auto;
  min-height: 88px;
  box-sizing: border-box;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px 20px;
  padding: 16px 142px 16px 0;
}

.header-title-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1 1 310px;
  min-width: 0;
}

.title-with-badge {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-with-badge h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 800;
  color: var(--wb-text);
  letter-spacing: -0.02em;
}

.pool-count-tag {
  font-size: 12px;
  font-weight: 600;
  color: var(--wb-brand);
  background: var(--wb-brand-soft);
  padding: 2px 10px;
  border-radius: 12px;
}

.header-actions-bar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
  max-width: 100%;
  margin-left: auto;
}

.header-actions-bar :deep(.el-button + .el-button) {
  margin-left: 0;
}

.header-actions-bar .el-button {
  height: 38px;
  padding: 0 14px;
  border-radius: 9px;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.header-actions-bar .el-button .el-icon:first-child {
  margin-right: 6px;
  font-size: 16px;
}

.header-actions-bar .el-button:focus-visible {
  outline: 2px solid var(--wb-brand);
  outline-offset: 2px;
}

.matrix-search-btn {
  box-shadow: none;
  font-weight: 700;
}

.matrix-search-btn:hover:not(:disabled) {
  box-shadow: 0 3px 10px rgba(79, 70, 229, 0.16);
}

.outreach-action-btn {
  color: var(--wb-brand);
  border-color: var(--wb-border);
  background-color: var(--wb-brand-soft);
}

.outreach-action-btn:hover:not(:disabled) {
  color: var(--wb-brand);
  border-color: var(--wb-brand);
  background-color: var(--wb-brand-soft);
}

.library-tools-btn,
.refresh-tool-btn {
  color: var(--wb-text);
  border-color: var(--wb-border);
  background-color: var(--wb-surface);
}

.library-tools-btn:hover,
.refresh-tool-btn:hover {
  color: var(--wb-brand);
  border-color: var(--wb-brand);
  background-color: var(--wb-surface-soft);
}

.library-tools-count,
.dropdown-item-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 20px;
  padding: 0 5px;
  border-radius: 10px;
  background: var(--wb-brand-soft);
  color: var(--wb-brand);
  font-size: 11px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.library-tools-count { margin-left: 6px; }
.dropdown-item-count { margin-left: auto; }
.dropdown-chevron { margin-left: 8px; font-size: 12px; }

.header-actions-bar .refresh-tool-btn {
  width: 38px;
  padding: 0;
}

.header-actions-bar .refresh-tool-btn .el-icon {
  margin: 0;
}

/* 主体分栏布局 */
.talent-workbench-layout {
  display: flex;
  gap: 20px;
  align-items: flex-start;
  margin-top: 16px;
  width: 100%;
  box-sizing: border-box;
}

/* 左侧职位分库导航侧栏 */
.job-repositories-sidebar {
  width: 256px;
  min-width: 256px;
  max-width: 256px;
  background-color: var(--wb-surface);
  border: 1px solid var(--wb-border);
  border-radius: 12px;
  padding: 14px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: calc(100vh - 140px);
  position: sticky;
  top: 20px;
  box-sizing: border-box;
}

.repo-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.repo-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--wb-text);
}

.add-repo-link {
  font-size: 12px;
}

.repo-search {
  margin-bottom: 2px;
}

.repo-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow-y: auto;
  padding-right: 2px;
}

.repo-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s ease;
  background-color: transparent;
  border-left: 3px solid transparent;
}

.repo-item:hover {
  background-color: var(--wb-surface-soft);
}

.repo-item.active {
  background-color: var(--wb-brand-soft);
  border-left-color: var(--wb-brand);
}

.all-repo-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 6px;
  background-color: #ede9fe;
  color: #6366f1;
  font-size: 14px;
}

.repo-status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-dot-publishing {
  background-color: #22c55e;
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.2);
}
.status-dot-draft {
  background-color: #f59e0b;
}
.status-dot-paused {
  background-color: #94a3b8;
}

.repo-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.repo-name {
  font-size: 13px;
  font-weight: 700;
  color: var(--wb-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.repo-sub {
  font-size: 11px;
  font-weight: 500;
  color: var(--wb-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.repo-badge {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 10px;
  background-color: var(--wb-surface-soft);
  color: var(--wb-muted);
  flex-shrink: 0;
}

.repo-badge.has-count {
  background-color: #e0e7ff;
  color: #3730a3;
}

.repo-divider {
  font-size: 11px;
  font-weight: 700;
  color: #64748b;
  padding: 8px 4px 4px;
  letter-spacing: 0.02em;
}

.empty-repos {
  text-align: center;
  font-size: 12px;
  color: #94a3b8;
  padding: 20px 0;
}

/* 右侧主看板区 */
.talent-content-area {
  flex: 1;
  min-width: 0;
  box-sizing: border-box;
}

/* 选定职位分库横幅 */
.selected-repo-banner {
  background: linear-gradient(135deg, #ffffff 0%, #f8faff 100%);
  border: 1px solid #e0e7ff;
  border-radius: 12px;
  padding: 16px 20px;
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  box-sizing: border-box;
  box-shadow: 0 1px 3px rgba(37, 99, 235, 0.04);
}

.repo-banner-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.repo-banner-title h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 800;
  color: #0f172a;
}

.banner-status-tag {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 600;
}

.banner-status-tag.status-dot-publishing {
  background-color: #dcfce7;
  color: #15803d;
}
.banner-status-tag.status-dot-draft {
  background-color: #fef3c7;
  color: #b45309;
}
.banner-status-tag.status-dot-paused {
  background-color: #f1f5f9;
  color: #475569;
}

.repo-banner-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  font-size: 12px;
  color: #334155;
}

.repo-banner-meta .meta-tag {
  font-weight: 500;
}

.repo-banner-meta .sep {
  color: #cbd5e1;
}

.repo-banner-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

/* 筛选与搜索工具条 */
.talent-filter-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 18px;
  box-sizing: border-box;
}

.toolbar-left-filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.search-filter-input {
  width: 240px;
}

.reset-filters-btn {
  font-size: 12px;
  margin-left: 2px;
}

.toolbar-right-controls {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-left: auto;
}

.candidate-sort-select {
  width: 152px;
}

.count-summary {
  font-size: 13px;
  color: var(--wb-muted);
}

.count-summary strong {
  color: var(--wb-text);
}

.view-mode-toggle {
  display: inline-flex;
}

.toggle-text {
  margin-left: 4px;
  font-size: 12px;
}

/* 网格卡片看板视图 (Grid View) */
.wb-candidate-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  box-sizing: border-box;
}

.talent-card {
  transition: transform 0.15s ease, box-shadow 0.15s ease;
  min-width: 0; /* 核心：防止卡片内部文字溢出把 CSS Grid 轨道撑爆 */
  background: var(--wb-surface);
  border: 1px solid var(--wb-border);
  border-radius: 12px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.talent-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.06);
  border-color: #cbd5e1;
}

.wb-candidate-top {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.candidate-avatar-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  width: 44px;
}

.candidate-sequence {
  font-variant-numeric: tabular-nums;
  font-size: 18px;
  font-weight: 700;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 0;
  background: linear-gradient(135deg, #8375ff, #4f46e5);
  color: #fff;
  flex-shrink: 0;
}

.candidate-seq-badge {
  font-size: 11px;
  font-weight: 700;
  color: #4f46e5;
  background: #eef2ff;
  border: 1px solid #e0e7ff;
  border-radius: 4px;
  padding: 0 4px;
  line-height: 1.3;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.2px;
}

.candidate-headline {
  flex: 1;
  min-width: 0;
}

.candidate-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  line-height: 1.2;
}

.wb-candidate-name {
  font-size: 15px;
  font-weight: 800;
  color: #0f172a;
  line-height: 1.2;
}

.fresh-grad-tag {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 700;
  color: #047857;
  background: #d1fae5;
  border: 1px solid #6ee7b7;
  border-radius: 999px;
  padding: 0 6px;
  height: 18px;
  line-height: 18px;
  letter-spacing: 0.2px;
  flex-shrink: 0;
}

.edu-degree-tag {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 600;
  color: #0369a1;
  background: #e0f2fe;
  border-radius: 4px;
  padding: 0 4px;
  margin-left: 4px;
  height: 16px;
  line-height: 16px;
  flex-shrink: 0;
}

.candidate-info-row {
  display: flex;
  align-items: center;
  font-size: 12px;
  margin-top: 3px;
  line-height: 1.3;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.candidate-info-row .row-label {
  color: #64748b;
  font-weight: 600;
  font-size: 11px;
  flex-shrink: 0;
  margin-right: 2px;
}

.candidate-info-row .row-val {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.candidate-info-row .role-val {
  color: #0f172a;
  font-weight: 600;
}

.candidate-info-row .company-val {
  color: #475569;
  font-weight: 500;
}

.candidate-info-row .major-val {
  color: #0f172a;
  font-weight: 600;
}

.candidate-info-row .school-val {
  color: #475569;
  font-weight: 500;
}

.candidate-target-role {
  margin-top: 3px;
  font-size: 11px;
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.table-name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.table-fresh-tag {
  height: 16px;
  line-height: 16px;
  padding: 0 4px;
  font-size: 9px;
}

.table-exp-box {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.25;
}

.table-role-title {
  font-size: 12px;
  font-weight: 600;
  color: #0f172a;
}

.table-company-name {
  font-size: 11px;
  color: #64748b;
}

.candidate-score-wrap {
  flex-shrink: 0;
  width: 76px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
}

.candidate-score-wrap small { color: #64748b; font-size: 10px; white-space: nowrap; }

.candidate-match-ring,
.candidate-match-pending {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 19px;
  font-weight: 800;
  color: #4f46e5;
  font-variant-numeric: tabular-nums;
}

.candidate-match-ring { background: conic-gradient(#6559ee var(--match), #e8eaf6 0); }
.candidate-match-ring span {
  width: 50px;
  height: 50px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: #fff;
}
.candidate-match-pending { background: #f1f5f9; color: #94a3b8; font-size: 13px; }

.candidate-skill-list { display: flex; flex-wrap: wrap; gap: 6px; min-height: 24px; }
.candidate-skill-chip {
  display: inline-block;
  max-width: 100%;
  padding: 3px 9px;
  border: 1px solid #e2e8f0;
  border-radius: 99px;
  background: #f8fafc;
  color: #475569;
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.candidate-skill-empty { font-size: 11px; color: #94a3b8; }

/* 能力分徽标 */
.score-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.score-badge.score-high {
  background-color: #dcfce7;
  color: #15803d;
  border: 1px solid #bbf7d0;
}

.score-badge.score-mid {
  background-color: #dbeafe;
  color: #1d4ed8;
  border: 1px solid #bfdbfe;
}

.score-badge.score-low {
  background-color: #f1f5f9;
  color: #475569;
  border: 1px solid #e2e8f0;
}

.score-badge.pending {
  background-color: #f8fafc;
  color: #94a3b8;
  font-weight: 500;
}

.wb-candidate-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.source-pill {
  background-color: #f1f5f9;
  color: #334155;
  font-weight: 600;
}

.stage-pill {
  background-color: #f1f5f9;
  color: #0f172a;
  font-weight: 600;
}

.candidate-owner-line {
  font-size: 12px;
  color: #475569;
}

.candidate-owner-line strong {
  color: #0f172a;
}

.wb-candidate-foot {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid #f1f5f9;
  margin-top: auto;
}

.candidate-created-time {
  font-size: 11px;
  color: #94a3b8;
}

.candidate-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
  min-width: 0;
}

.candidate-actions .candidate-primary-action,
.candidate-actions .candidate-secondary-action { flex: 1 1 105px; }
.candidate-actions .candidate-more-action { min-width: 60px; }

.duplicate-drawer-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.candidate-actions .el-button + .el-button {
  margin-left: 0;
}

/* 高密数据表格视图 (Table View) */
.talent-table-container {
  background: var(--wb-surface);
  border: 1px solid var(--wb-border);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
}

.talent-data-table {
  width: 100%;
  cursor: pointer;
}

.table-seq {
  font-size: 12px;
  font-weight: 700;
  color: #64748b;
  font-variant-numeric: tabular-nums;
}

.table-candidate-name {
  font-size: 14px;
  font-weight: 700;
  color: #0f172a;
}

.table-job-box {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.table-job-title {
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
}

.table-job-dept {
  font-size: 11px;
  color: #64748b;
}

.table-source-tag {
  font-size: 12px;
  color: #334155;
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
}

.table-owner-text {
  font-size: 12px;
  color: #334155;
}

.table-date-text {
  font-size: 12px;
  color: #64748b;
}

.table-row-actions {
  display: flex;
  justify-content: center;
  gap: 4px;
}

/* 空状态卡片 */
.empty-candidate-panel {
  padding: 56px 24px;
  text-align: center;
  box-sizing: border-box;
}

.empty-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  color: #94a3b8;
}

.empty-detail-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.empty-illustration {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: #eff6ff;
  color: #3b82f6;
  font-size: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
}

.empty-title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 700;
  color: var(--wb-text);
}

.empty-subtitle {
  margin: 0 0 20px;
  font-size: 13px;
  color: var(--wb-muted);
  max-width: 480px;
  line-height: 1.5;
}

.empty-action-btn {
  border-radius: 20px;
  padding: 10px 24px;
  font-weight: 700;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.25);
}


/* 邀约面试与沟通文案弹窗 */
.interview-invite-dialog {
  .invite-dialog-body {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .invite-candidate-header {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 14px 18px;
    background: #f8fafc;
    border-radius: 10px;
    border: 1px solid #e2e8f0;

    .invite-avatar {
      width: 48px;
      height: 48px;
      border-radius: 50%;
      background: linear-gradient(135deg, #6366f1, #4f46e5);
      color: #fff;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 20px;
      font-weight: 700;
      flex-shrink: 0;
      box-shadow: 0 4px 10px rgba(99, 102, 241, 0.25);
    }

    .invite-info {
      flex: 1;
      min-width: 0;

      .invite-name-row {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 4px;

        .invite-name {
          font-size: 16px;
          font-weight: 700;
          color: #1e293b;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
      }

      .invite-sub-text {
        font-size: 12px;
        color: #64748b;
      }
    }
  }

  .invite-section-box {
    padding: 14px 16px;
    background: #ffffff;
    border: 1px solid #e2e8f0;
    border-radius: 10px;

    .invite-section-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 6px;

      .section-title {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 14px;
        color: #1e293b;

        .icon {
          font-size: 16px;
        }
      }

      .section-actions {
        display: flex;
        gap: 8px;
      }
    }

    .section-hint {
      font-size: 12px;
      color: #64748b;
      margin: 0 0 10px;
      line-height: 1.5;
    }
  }

  .invite-input, .invite-select, .invite-textarea {
    width: 100%;
    padding: 8px 10px;
    border: 1px solid #dcdfe6;
    border-radius: 6px;
    font-size: 13px;
    color: #303133;
    outline: none;
    box-sizing: border-box;
    transition: border-color 0.2s;

    &:focus {
      border-color: #409eff;
    }
  }

  .required {
    color: #f56c6c;
    margin-left: 2px;
  }
}

@media (max-width: 1180px) {
  .talent-page-header { align-items: flex-start; }
  .header-actions-bar { justify-content: flex-start; margin-left: 0; }
}

@media (max-width: 900px) {
  .wb-candidate-grid { grid-template-columns: minmax(0, 1fr); }
}

@media (max-width: 760px) {
  .talent-pool-page { padding-right: 16px; padding-left: 16px; }
  .talent-page-header { padding-right: 16px; }
  .talent-workbench-layout { flex-direction: column; }
  .job-repositories-sidebar { position: static; width: 100%; min-width: 0; max-width: none; max-height: 240px; }
  .header-actions-bar { width: 100%; }
}
</style>
