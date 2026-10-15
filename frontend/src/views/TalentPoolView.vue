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

      <div class="header-actions-bar">
        <!-- 次级数据录入与分流维护工具组 -->
        <div class="action-triage-group">
          <el-button
            class="action-triage-btn"
            @click="refresh"
            :loading="recruitment.loading"
            title="刷新人才库数据"
          >
            <el-icon><Refresh /></el-icon>刷新
          </el-button>

          <el-button
            class="action-triage-btn"
            :loading="importing"
            @click="import51Job"
            title="一键拉取 51job 最新投递简历"
          >
            <el-icon><Download /></el-icon>51job 投递导入
          </el-button>

          <el-badge
            :value="pendingApplications.length"
            :hidden="!pendingApplications.length"
            type="primary"
            class="triage-badge"
          >
            <el-button class="action-triage-btn" @click="showInbox">
              待分配投递
            </el-button>
          </el-badge>

          <el-badge
            :value="duplicates.length"
            :hidden="!duplicates.length"
            type="warning"
            class="triage-badge"
          >
            <el-button class="action-triage-btn" @click="showDuplicates">
              疑似重复
            </el-button>
          </el-badge>
        </div>

        <div class="header-action-divider"></div>

        <!-- 主业务动作组 (重点突出) -->
        <div class="action-primary-group">
          <el-button
            class="outreach-action-btn"
            :disabled="!selectedProjectId"
            @click="showOutreach = true"
            :title="selectedProjectId ? '一键向当前分库前十名匹配候选人打招呼' : '请先选择左侧具体职位分库'"
          >
            <el-icon><ChatRound /></el-icon>向前十名打招呼
          </el-button>

          <el-button
            type="primary"
            class="matrix-search-btn"
            :disabled="!selectedProjectId"
            @click="showSearch = true"
            :title="selectedProjectId ? '唤起 4合1 多平台矩阵式主动寻才' : '请先选择左侧具体职位分库'"
          >
            <el-icon><Position /></el-icon>矩阵寻才
          </el-button>
        </div>
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
            <el-button size="small" @click="router.push('/jobs')">职位设置</el-button>
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
              <span class="wb-avatar candidate-sequence" :title="`当前列表第 ${index + 1} 位`">
                #{{ index + 1 }}
              </span>
              <div class="candidate-headline">
                <div class="wb-candidate-name">{{ item.name }}</div>
                <div class="wb-candidate-role" :title="`${item.jobTitle || item.projectName} · ${item.projectName}`">
                  {{ item.jobTitle || item.projectName }} · {{ item.projectName }}
                </div>
              </div>
              <div class="candidate-score-wrap">
                <span
                  v-if="item.analysisStatus === 'done'"
                  class="score-badge"
                  :class="scoreLevelClass(item.abilityScore)"
                  title="AI 核心能力评分"
                >
                  {{ item.abilityScore }} 分
                </span>
                <span v-else class="score-badge pending" title="尚未完成 AI 评测">待评</span>
              </div>
            </div>

            <div class="wb-candidate-meta">
              <span class="wb-pill source-pill">{{ item.source }}</span>
              <span class="wb-pill" :class="statusClass(item.analysisStatus)">
                {{ statusLabel(item.analysisStatus) }}
              </span>
              <span class="wb-pill" :class="redLineClass(item.redLineStatus)">
                <el-icon style="margin-right: 2px;">
                  <CircleCheck v-if="item.redLineStatus === 'passed'" />
                  <CircleClose v-else-if="item.redLineStatus === 'failed'" />
                  <Warning v-else />
                </el-icon>
                红线：{{ redLineLabel(item.redLineStatus) }}
              </span>
              <span class="wb-pill stage-pill">{{ stageLabels[item.stage] || item.stage }}</span>
            </div>

            <div class="candidate-owner-line">
              <span>👤 负责人：<strong>{{ item.owner || '未分配' }}</strong></span>
              <span v-if="item.nextActionAt"> · 下次跟进 {{ formatDate(item.nextActionAt) }}</span>
            </div>

            <div class="wb-candidate-foot">
              <span class="candidate-created-time">入库 {{ formatDate(item.createdAt) }}</span>
              <div class="candidate-actions">
                <el-button size="small" @click="selected = item">查看档案</el-button>
                <el-button size="small" @click="openMoveDialog(item)">调配分库</el-button>
                <el-button
                  v-if="item.analysisStatus === 'error'"
                  size="small"
                  type="warning"
                  plain
                  @click="retryAnalysis(item.resumeId)"
                >
                  重试
                </el-button>
                <el-button
                  size="small"
                  type="primary"
                  plain
                  @click="router.push('/project/' + item.projectId)"
                >
                  评测
                </el-button>
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
            <el-table-column type="index" label="#" width="60" align="center">
              <template #default="{ $index }">
                <span class="table-seq">#{{ $index + 1 }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="name" label="姓名" min-width="120">
              <template #default="{ row }">
                <span class="table-candidate-name">{{ row.name }}</span>
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

            <el-table-column label="能力评分" width="110" align="center">
              <template #default="{ row }">
                <span
                  v-if="row.analysisStatus === 'done'"
                  class="score-badge"
                  :class="scoreLevelClass(row.abilityScore)"
                >
                  {{ row.abilityScore }} 分
                </span>
                <span v-else class="score-badge pending">待评</span>
              </template>
            </el-table-column>

            <el-table-column label="红线状态" width="110" align="center">
              <template #default="{ row }">
                <span class="wb-pill" :class="redLineClass(row.redLineStatus)" style="font-size: 11px;">
                  {{ redLineLabel(row.redLineStatus) }}
                </span>
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

            <el-table-column label="操作" width="180" fixed="right" align="center">
              <template #default="{ row }">
                <div class="table-row-actions" @click.stop>
                  <el-button link type="primary" size="small" @click="selected = row">档案</el-button>
                  <el-button link type="primary" size="small" @click="openMoveDialog(row)">调配</el-button>
                  <el-button link type="primary" size="small" @click="router.push('/project/' + row.projectId)">评测</el-button>
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

    <!-- 候选人档案抽屉 (现代流式模块化设计) -->
    <el-drawer
      v-model="drawerOpen"
      title="候选人综合档案"
      size="540px"
      class="candidate-profile-drawer"
    >
      <template v-if="selected">
        <!-- 抽屉顶部 Hero 卡片 -->
        <div class="profile-hero-card">
          <div class="hero-avatar">
            {{ selected.name.charAt(0) }}
          </div>
          <div class="hero-info">
            <div class="hero-name-row">
              <h2 class="hero-name">{{ selected.name }}</h2>
              <span class="hero-source-tag">{{ selected.source }}</span>
            </div>
            <p class="hero-job-text">
              {{ selected.jobTitle || selected.projectName }} · {{ selected.projectName }}
            </p>
            <div class="hero-pills-row">
              <span class="wb-pill stage-pill">{{ stageLabels[selected.stage] || selected.stage }}</span>
              <span class="wb-pill" :class="redLineClass(selected.redLineStatus)">
                红线：{{ redLineLabel(selected.redLineStatus) }}
              </span>
            </div>
          </div>
          <div class="hero-score-box">
            <div class="hero-score-val" :class="scoreLevelClass(selected.abilityScore)">
              {{ selected.analysisStatus === 'done' ? selected.abilityScore : '—' }}
            </div>
            <div class="hero-score-lbl">综合评分</div>
          </div>
        </div>

        <!-- 快速流转与平台主页操作条 -->
        <div class="profile-quick-actions">
          <el-button
            v-if="selected.profileUrl"
            @click="openProfile(selected.profileUrl)"
            title="在浏览器中打开原平台候选人主页"
          >
            打开平台主页
          </el-button>
          <el-button type="primary" plain @click="openMoveDialog(selected)">
            调配至其他职位库
          </el-button>
          <el-button type="primary" @click="router.push('/project/' + selected!.projectId)">
            查看完整评测
          </el-button>
        </div>

        <!-- 档案流式卡片一：基本指标与跟进责任人 -->
        <div class="profile-section-card">
          <h3 class="section-card-title">📌 岗位评测与跟进状态</h3>
          <div class="section-grid">
            <div class="section-grid-item">
              <span class="grid-label">当前阶段</span>
              <span class="grid-value font-semibold">{{ stageLabels[selected.stage] || selected.stage }}</span>
            </div>
            <div class="section-grid-item">
              <span class="grid-label">红线核验</span>
              <span class="grid-value" :class="redLineClass(selected.redLineStatus)">
                {{ redLineLabel(selected.redLineStatus) }}
              </span>
            </div>
            <div class="section-grid-item">
              <span class="grid-label">已确认加分项</span>
              <span class="grid-value">{{ selected.confirmedBonus || 0 }} 项</span>
            </div>
            <div class="section-grid-item">
              <span class="grid-label">负责顾问</span>
              <span class="grid-value">{{ selected.owner || '未分配' }}</span>
            </div>
            <div class="section-grid-item full-width" v-if="selected.nextActionAt">
              <span class="grid-label">下次计划跟进</span>
              <span class="grid-value text-blue">{{ formatDate(selected.nextActionAt) }}</span>
            </div>
          </div>
        </div>

        <!-- 档案流式卡片二：核实联系方式 -->
        <div class="profile-section-card">
          <div class="section-title-with-desc">
            <h3 class="section-card-title">📞 核实联系方式</h3>
            <p class="section-desc">仅在 HR 已核实该联系方式属于本人时录入；相同联系方式将自动合并统一档案。</p>
          </div>
          <div class="verify-contact-form">
            <el-select v-model="contactKind" style="width: 100px">
              <el-option label="邮箱" value="email" />
              <el-option label="电话" value="phone" />
            </el-select>
            <el-input
              v-model="verifiedContact"
              placeholder="填写已核实的手机号或邮箱"
              clearable
              class="verify-input"
            />
            <el-button type="primary" @click="verifyContact" :disabled="!verifiedContact.trim()">
              核实
            </el-button>
          </div>
        </div>

        <!-- 档案流式卡片三：关联岗位与独立评测 -->
        <section v-if="profile" class="profile-section-card">
          <h3 class="section-card-title">💼 关联岗位与独立评测</h3>
          <div v-if="!profile.projects?.length" class="empty-sub-text">暂无跨岗位评测</div>
          <div
            v-for="link in profile.projects"
            :key="link.projectCandidateId"
            class="nested-sub-card"
          >
            <div class="nested-sub-top">
              <strong>{{ link.projectName }}</strong>
              <span class="wb-muted"> · {{ link.jobTitle }}</span>
            </div>
            <div class="nested-sub-meta">
              <span>评分：<strong>{{ link.analysisStatus === 'done' ? link.abilityScore : '待评估' }}</strong></span>
              <span class="sep">·</span>
              <span>红线：<strong :class="redLineClass(link.redLineStatus)">{{ redLineLabel(link.redLineStatus) }}</strong></span>
              <span class="sep">·</span>
              <span>阶段：{{ stageLabels[link.stage] || link.stage }}</span>
            </div>
          </div>

          <h3 class="section-card-title" style="margin-top: 20px;">📄 来源渠道与简历版本</h3>
          <div class="source-list">
            <div
              v-for="source in profile.sources"
              :key="source.platform + source.platformCandidateId"
              class="source-row-item"
            >
              <span class="source-badge-tag">{{ source.platform === 'legacy' ? '本地档案' : source.platform }}</span>
              <span class="source-candidate-id">ID: {{ source.platformCandidateId }}</span>
            </div>
          </div>
          <div class="resume-files-list" style="margin-top: 10px;">
            <div
              v-for="resume in profile.resumes"
              :key="resume.id"
              class="resume-file-item"
            >
              <el-icon><Document /></el-icon>
              <span class="file-name">{{ resume.fileName }}</span>
              <span class="file-date">{{ formatDate(resume.createdAt) }}</span>
            </div>
          </div>

          <h3 class="section-card-title" style="margin-top: 20px;">🕒 跨岗位沟通与跟进时间线</h3>
          <div v-if="!profile.timeline?.length" class="empty-sub-text">暂无跟进记录</div>
          <div v-else class="profile-timeline">
            <div
              v-for="event in profile.timeline"
              :key="event.id"
              class="timeline-item-card"
            >
              <div class="timeline-header">
                <span class="timeline-project">{{ event.projectName }}</span>
                <span class="timeline-meta">{{ event.actor || 'HR' }} · {{ formatDate(event.createdAt) }}</span>
              </div>
              <p class="timeline-body">{{ event.note || event.type }}</p>
            </div>
          </div>
        </section>
      </template>
    </el-drawer>

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
    <TopTenOutreachDialog v-model="showOutreach" :project-id="selectedProjectId" />
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
      </article>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
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
  ChatRound,
  CircleCheck,
  CircleClose,
  Warning,
  Loading,
  Document
} from '@element-plus/icons-vue'
import { useProjectStore, type Project } from '../composables/useProjectStore'
import { useRecruitmentStore, stageLabels, type TalentCandidate } from '../composables/useRecruitmentStore'
import BossSearchDialog from '../components/BossSearchDialog.vue'
import TopTenOutreachDialog from '../components/TopTenOutreachDialog.vue'
import { ElMessage } from 'element-plus'

interface PendingApplication { id: string; applicationId: string; candidateName: string; platformJobId: string }
interface DuplicatePair { candidateId: string; otherCandidateId: string; name: string; reason: string }
interface CandidateProfile {
  projects: TalentCandidate[]
  sources: Array<{ platform: string; platformCandidateId: string }>
  resumes: Array<{ id: string; fileName: string; createdAt: string }>
  timeline: Array<{ id: string; projectName: string; type: string; note: string; actor: string; createdAt: string }>
}

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
const importing = ref(false)
const inboxOpen = ref(false)
const pendingApplications = ref<PendingApplication[]>([])
const assignment = ref<Record<string, string>>({})
const duplicates = ref<DuplicatePair[]>([])
const duplicatesOpen = ref(false)
const mergeReasons = ref<Record<string, string>>({})

const contactKind = ref('email')
const verifiedContact = ref('')
const selected = ref<TalentCandidate | null>(null)
const profile = ref<CandidateProfile | null>(null)
const drawerOpen = computed({
  get: () => !!selected.value,
  set: value => { if (!value) selected.value = null }
})

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
))

watch(() => route.query.project, value => {
  selectedProjectId.value = String(value || '')
})

watch(selected, async item => {
  profile.value = null
  if (!item) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    profile.value = await api.GetCandidateProfile(item.id)
  } catch (err: any) {
    ElMessage.warning(err?.message || String(err))
  }
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
  selected.value = row
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

async function verifyContact() {
  if (!selected.value || !verifiedContact.value.trim()) return
  try {
    const api: any = await import('../../wailsjs/go/main/App')
    await api.VerifyCandidateContact(selected.value.id, contactKind.value, verifiedContact.value.trim(), 'HR')
    ElMessage.success('联系方式已核实，统一档案已更新')
    verifiedContact.value = ''
    selected.value = null
    await Promise.all([loadDuplicates(), refresh()])
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
  padding: 20px 32px 60px 24px;
  overflow-x: auto;
}

/* 顶部全局标题栏与动作分组 */
.talent-page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  padding-bottom: 4px;
}

.header-title-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
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
  color: #0f172a;
  letter-spacing: -0.02em;
}

.pool-count-tag {
  font-size: 12px;
  font-weight: 600;
  color: #4338ca;
  background: #e0e7ff;
  padding: 2px 10px;
  border-radius: 12px;
}

.header-actions-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.action-triage-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.action-triage-btn {
  font-size: 13px;
  font-weight: 500;
  color: #334155;
  border-color: #cbd5e1;
  background-color: #ffffff;
  transition: all 0.15s ease;
}

.action-triage-btn:hover {
  color: #2563eb;
  border-color: #93c5fd;
  background-color: #f8faff;
}

.triage-badge :deep(.el-badge__content) {
  font-size: 11px;
  font-weight: 700;
  height: 18px;
  line-height: 18px;
  padding: 0 5px;
}

.header-action-divider {
  width: 1px;
  height: 24px;
  background-color: #e2e8f0;
}

.action-primary-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.outreach-action-btn {
  font-weight: 600;
  color: #4338ca;
  border-color: #c7d2fe;
  background-color: #eef2ff;
}

.outreach-action-btn:hover:not(:disabled) {
  color: #3730a3;
  border-color: #a5b4fc;
  background-color: #e0e7ff;
}

.matrix-search-btn {
  font-weight: 700;
  box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);
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

/* 左侧职位分库导航侧栏 (紧凑 256px，黑字高对比) */
.job-repositories-sidebar {
  width: 256px;
  min-width: 256px;
  max-width: 256px;
  background-color: #ffffff;
  border: 1px solid #e2e8f0;
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
  color: #0f172a;
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
  background-color: #ffffff;
  border-left: 3px solid transparent;
}

.repo-item:hover {
  background-color: #f8fafc;
}

.repo-item.active {
  background-color: #eff6ff;
  border-left-color: #2563eb;
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
  color: #0f172a;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.repo-sub {
  font-size: 11px;
  font-weight: 500;
  color: #475569;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.repo-badge {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 10px;
  background-color: #f1f5f9;
  color: #475569;
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
  gap: 16px;
  margin-left: auto;
}

.count-summary {
  font-size: 13px;
  color: #475569;
}

.count-summary strong {
  color: #0f172a;
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
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
  box-sizing: border-box;
}

.talent-card {
  transition: transform 0.15s ease, box-shadow 0.15s ease;
  min-width: 0; /* 核心：防止卡片内部文字溢出把 CSS Grid 轨道撑爆 */
  background: #ffffff;
  border: 1px solid #e2e8f0;
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
  align-items: center;
  gap: 10px;
}

.candidate-sequence {
  font-variant-numeric: tabular-nums;
  font-size: 13px;
  font-weight: 700;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  color: #475569;
  flex-shrink: 0;
}

.candidate-headline {
  flex: 1;
  min-width: 0;
}

.wb-candidate-name {
  font-size: 15px;
  font-weight: 800;
  color: #0f172a;
  line-height: 1.2;
}

.wb-candidate-role {
  font-size: 12px;
  font-weight: 500;
  color: #475569;
  margin-top: 3px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.candidate-score-wrap {
  flex-shrink: 0;
}

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
  justify-content: space-between;
  align-items: center;
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
  gap: 6px;
}

/* 高密数据表格视图 (Table View) */
.talent-table-container {
  background: #ffffff;
  border: 1px solid #e2e8f0;
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
  color: #0f172a;
}

.empty-subtitle {
  margin: 0 0 20px;
  font-size: 13px;
  color: #64748b;
  max-width: 480px;
  line-height: 1.5;
}

.empty-action-btn {
  border-radius: 20px;
  padding: 10px 24px;
  font-weight: 700;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.25);
}

/* 候选人详细档案抽屉 (Modern Modular Design) */
.candidate-profile-drawer :deep(.el-drawer__body) {
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background-color: #f8fafc;
}

.profile-hero-card {
  display: flex;
  align-items: center;
  gap: 16px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 16px;
}

.hero-avatar {
  width: 50px;
  height: 50px;
  border-radius: 12px;
  background: linear-gradient(135deg, #4f46e5 0%, #3b82f6 100%);
  color: #ffffff;
  font-size: 20px;
  font-weight: 800;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.hero-info {
  flex: 1;
  min-width: 0;
}

.hero-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.hero-name {
  margin: 0;
  font-size: 18px;
  font-weight: 800;
  color: #0f172a;
}

.hero-source-tag {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  background-color: #f1f5f9;
  color: #475569;
}

.hero-job-text {
  margin: 4px 0 8px;
  font-size: 13px;
  color: #475569;
  font-weight: 500;
}

.hero-pills-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.hero-score-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  flex-shrink: 0;
}

.hero-score-val {
  font-size: 22px;
  font-weight: 900;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.hero-score-val.score-high {
  color: #15803d;
}

.hero-score-val.score-mid {
  color: #1d4ed8;
}

.hero-score-val.score-low {
  color: #475569;
}

.hero-score-lbl {
  font-size: 10px;
  color: #64748b;
  margin-top: 4px;
  font-weight: 600;
}

.profile-quick-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.profile-section-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 16px;
}

.section-card-title {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 800;
  color: #0f172a;
}

.section-title-with-desc {
  margin-bottom: 12px;
}

.section-title-with-desc .section-card-title {
  margin-bottom: 4px;
}

.section-desc {
  margin: 0;
  font-size: 12px;
  color: #64748b;
  line-height: 1.4;
}

.section-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.section-grid-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.section-grid-item.full-width {
  grid-column: 1 / -1;
}

.grid-label {
  font-size: 11px;
  color: #64748b;
  font-weight: 500;
}

.grid-value {
  font-size: 13px;
  color: #0f172a;
}

.grid-value.font-semibold {
  font-weight: 700;
}

.grid-value.text-blue {
  color: #2563eb;
  font-weight: 600;
}

.verify-contact-form {
  display: flex;
  gap: 8px;
  align-items: center;
}

.verify-input {
  flex: 1;
}

.nested-sub-card {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 8px;
}

.nested-sub-top {
  font-size: 13px;
  color: #0f172a;
}

.nested-sub-meta {
  font-size: 12px;
  color: #475569;
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.empty-sub-text {
  font-size: 12px;
  color: #94a3b8;
  padding: 8px 0;
}

.source-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.source-row-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.source-badge-tag {
  font-weight: 600;
  color: #4338ca;
  background: #ede9fe;
  padding: 2px 6px;
  border-radius: 4px;
}

.source-candidate-id {
  color: #64748b;
  font-family: monospace;
}

.resume-files-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.resume-file-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #334155;
  background: #f8fafc;
  padding: 6px 10px;
  border-radius: 6px;
}

.resume-file-item .file-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.resume-file-item .file-date {
  color: #94a3b8;
  font-size: 11px;
}

.profile-timeline {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.timeline-item-card {
  background: #f8fafc;
  border-left: 3px solid #6366f1;
  padding: 8px 12px;
  border-radius: 0 8px 8px 0;
}

.timeline-header {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  margin-bottom: 2px;
}

.timeline-project {
  font-weight: 700;
  color: #0f172a;
}

.timeline-meta {
  color: #94a3b8;
  font-size: 11px;
}

.timeline-body {
  margin: 0;
  font-size: 12px;
  color: #475569;
}
</style>
