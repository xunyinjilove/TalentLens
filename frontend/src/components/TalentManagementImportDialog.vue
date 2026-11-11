<template>
  <el-dialog
    v-model="visible"
    title="51job 人才管理与人才望远镜 · 项目筛选"
    width="760px"
    top="4vh"
    :close-on-click-modal="!running"
    :close-on-press-escape="!running"
  >
    <div class="dialog-scroll-body" style="max-height: 68vh; overflow-y: auto; overflow-x: hidden; padding-right: 8px;">
      <el-tabs v-model="activeTab" style="margin-top: -10px">
      <!-- Tab 1: 四类人才管理同步 -->
      <el-tab-pane label="职位接入与人才管理同步" name="sync">
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
          <template #default>
            待处理和待定按项目硬性条件筛选，明确不符者在整轮同步后自动逐人标记「不合适」，平台成功后归档本地旧简历。合适、面试直接导入；资料不足者保留供补读，不自动拒绝。
          </template>
        </el-alert>

        <el-alert v-if="!jobs.length && !loading" type="warning" :closable="false" show-icon style="margin-bottom: 16px"
          title="未检测到已登录的 51job 企业职位。请点击下方【打开 51job 浏览器】登录企业账号，登录完成后点击【重新读取职位】即可。" />
        <el-alert v-if="catalogStale && jobs.length" type="warning" :closable="false" show-icon style="margin-bottom: 16px"
          title="当前显示上次读取的职位，尚未核实当前登录账号；重新读取成功前不能导入或启动同步。" />
        <p v-if="catalogStatus" class="catalog-status" :class="{ stale: catalogStale }">{{ catalogStatus }}</p>

        <el-form label-position="top">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="51job 企业职位">
                <el-select v-model="jobId" filterable style="width: 100%" :disabled="running || scanning" placeholder="选择已登录企业的职位">
                  <el-option v-for="job in jobs" :key="job.jobId" :value="job.jobId" :label="formatJobOptionLabel(job)" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="对应本地职位专属库">
                <el-select v-model="projectId" filterable style="width: 100%" :disabled="running || scanning" placeholder="首次绑定请确认项目">
                  <el-option v-for="project in projects" :key="project.id" :value="project.id"
                    :label="`${project.job_config?.title || project.name} · ${project.name}`" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-alert v-if="selectedJob && selectedJob.degree === '大专' && strictCriteria.minEducation === '本科'"
            type="warning" :closable="false" show-icon style="margin-bottom: 16px"
            title="注意：当前 51job 职位的招聘要求为【大专】，但您在【岗位专属硬性条件】中设置了【本科及以上】。本次扫描的大专候选人将被拦截排除。若需录入大专候选人，请前往【岗位专属硬性条件】将最低学历调整为【大专及以上】。" />

          <div style="display: flex; gap: 12px; align-items: center; margin-bottom: 16px; flex-wrap: wrap">
            <el-button type="success" :loading="running" :disabled="!jobId || !projectId || catalogStale || scanning" @click="start">
              同步人才管理四类简历到本地
            </el-button>
            <el-button v-if="running" type="warning" plain @click="requestStopImport">中止同步</el-button>
            <span style="font-size: 12px; color: #64748b">
              待处理、待定仅合格者入库；合适、面试直接同步。每个职位专属库只绑定一个当前 51job 职位。
            </span>
          </div>
		  <div v-if="attachmentTask" style="padding:10px 12px;margin-bottom:14px;border:1px solid #dbeafe;border-radius:8px;background:#f8fbff;font-size:13px;line-height:1.8;color:#334155">
		    <strong>附件简历后台同步：</strong>
		    {{ attachmentTask.checked || 0 }}/{{ attachmentTask.queue?.length || 0 }} 人已检查 ·
		    抓取 {{ attachmentTask.downloaded || 0 }} · 跳过 {{ attachmentTask.skipped || 0 }} · 失败 {{ attachmentTask.failed || 0 }}
		    <span v-if="attachmentTask.currentName"> · 正在检查 {{ attachmentTask.currentName }}</span>
		    <span v-if="attachmentTask.state === 'paused'"> · 已中止，可继续</span>
		    <span v-if="attachmentTask.state === 'completed'"> · 已完成</span>
		    <el-button v-if="attachmentTask.state === 'running'" size="small" type="warning" plain @click="stopAttachmentSync">中止附件同步</el-button>
		    <el-button v-else-if="attachmentTask.state === 'paused'" size="small" type="primary" plain @click="resumeAttachmentSync">继续附件同步</el-button>
		    <p v-if="attachmentTask.lastError" class="catalog-status">{{ attachmentTask.lastError }}</p>
		  </div>

        </el-form>

        <p v-if="progress" style="color: #606f89; font-size: 13px">{{ progress }}</p>
        <p v-if="summary" style="color: #0f766e; font-size: 13px; font-weight: 500">{{ summary }}</p>
        <el-table v-if="manualUnfitReviews.length" :data="manualUnfitReviews" size="small" max-height="240" style="margin-top:12px">
          <el-table-column prop="candidateName" label="自动不合适待处理" width="160" />
          <el-table-column prop="platformStatus" label="平台状态" width="100" />
          <el-table-column label="明确不符原因" min-width="260">
            <template #default="{ row }">{{ row.reasons?.filter((item: any) => item.status === 'fail').map((item: any) => item.evidence).join('；') }}</template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="人才望远镜每日筛选" name="telescope">
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px"
          title="读取 51job「AI淘金 → 人才望远镜」对应职位，按项目条件筛选；整轮完成后自动对明确不符者点「不感兴趣」。资料不足者仅记录供补读，筛选不会发送消息。" />

        <el-alert v-if="!telescopeJobs.length && !telescopeLoading" type="warning" :closable="false" show-icon style="margin-bottom: 16px"
          title="未读取到职位。若您已在 51job 浏览器打开「AI淘金 → 人才望远镜」，请点击下方【重新读取望远镜职位】。" />
        <p v-if="telescopeCatalogStatus" class="catalog-status" :class="{ stale: telescopeCatalogStale }">{{ telescopeCatalogStatus }}</p>

        <el-form label-position="top">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="51job 人才望远镜职位">
                <el-select v-model="telescopeJobId" filterable style="width: 100%" :disabled="telescopeScanning"
                  placeholder="选择与望远镜页面当前标签一致的职位">
                  <el-option v-for="job in telescopeJobs" :key="job.jobId" :value="job.jobId" :label="formatJobOptionLabel(job)" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="共用条件的本地专属库">
                <el-select v-model="projectId" filterable style="width: 100%" :disabled="telescopeScanning"
                  placeholder="选择已绑定的项目">
                  <el-option v-for="project in projects" :key="project.id" :value="project.id"
                    :label="`${project.job_config?.title || project.name} · ${project.name}`" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>

        <p class="catalog-status">已保存项目条件：{{ savedCriteria?.criteriaVersion || '读取中' }} · 活跃度{{ savedCriteria?.activityDays ? `近 ${savedCriteria.activityDays} 天` : '不限' }} · 最低学历{{ savedCriteria?.minEducation || '不限' }}</p>
        <div v-if="projectId" class="screening-preview-card" style="padding:14px;margin:12px 0;border:1px solid #e2e8f0;border-radius:8px">
          <strong>望远镜技能与其他要求</strong>
          <p class="catalog-status">岗位必备技能已预填；每项可设为必须或参考。资料未写明时标为证据不足，不据此拒绝。筛选只生成预览，绝不自动入库。</p>
          <el-alert v-if="telescopeConditionConfig?.jobSkillsChanged" type="warning" :closable="false" title="岗位技能已变化。自定义条件已保留，可手动同步新的岗位技能。" style="margin-bottom:10px" />
          <div v-for="(condition, index) in telescopeConditions" :key="index" style="display:flex;gap:8px;margin:8px 0;align-items:center">
            <el-select v-model="condition.kind" style="width:110px"><el-option label="技能" value="skill" /><el-option label="其他要求" value="other" /></el-select>
            <el-input v-model="condition.text" placeholder="一行一项，例如：有医院渠道开发经历" style="flex:1" />
            <el-select v-model="condition.required" style="width:110px"><el-option label="必须满足" :value="true" /><el-option label="加分参考" :value="false" /></el-select>
            <el-button @click="telescopeConditions.splice(index, 1)">删除</el-button>
          </div>
          <div style="display:flex;gap:8px;margin-top:10px">
            <el-button @click="telescopeConditions.push({ text:'', kind:'skill', required:true, source:'custom' })">添加技能</el-button>
            <el-button @click="telescopeConditions.push({ text:'', kind:'other', required:true, source:'custom' })">添加其他要求</el-button>
            <el-button v-if="telescopeConditionConfig?.jobSkillsChanged" @click="syncTelescopeJobSkills">同步岗位技能</el-button>
            <el-button type="primary" plain @click="saveTelescopeConditions">保存望远镜条件</el-button>
          </div>
        </div>
        <p class="catalog-status">计划时间：{{ savedCriteria?.scheduleEnabled ? savedCriteria.scheduleTime : '未启用' }}。软件运行期间按项目时间自动筛选；关闭期间不补跑。</p>
        <div style="display: flex; gap: 10px; align-items: center; margin: 16px 0; flex-wrap: wrap">
          <el-button type="warning" plain @click="openTelescope">打开 51job 人才望远镜</el-button>
          <el-button type="info" plain :loading="telescopeLoading" @click="loadTelescopeCatalog">
            <el-icon><Refresh /></el-icon> 重新读取望远镜职位
          </el-button>
          <el-button type="primary" :loading="telescopeScanning" :disabled="!telescopeJobId || !projectId || telescopeCatalogStale || running" @click="scanTelescopePreview">
            立即筛选当前项目
          </el-button>
        </div>
        <p v-if="telescopeProgress" class="catalog-status">{{ telescopeProgress }}</p>

        <div v-if="telescopePreview" class="screening-preview-card" style="border: 1px solid #e2e8f0; border-radius: 8px; padding: 14px; background: #fff">
          <div style="font-weight: 600; margin-bottom: 12px">
            望远镜预览 · {{ telescopePreview.businessDate }} · 条件 {{ telescopePreview.criteriaVersion }} · 共 {{ telescopePreview.totalCount }} 人
          </div>
          <el-radio-group v-model="telescopeSubTab" size="small" style="margin-bottom: 12px">
            <el-radio-button value="qualified">合格 {{ telescopePreview.qualifiedCount }}</el-radio-button>
            <el-radio-button value="unfit">不合格 {{ telescopePreview.unfitCount }}</el-radio-button>
            <el-radio-button value="pending">技能／资料证据不足 {{ telescopePreview.pendingCount }}</el-radio-button>
          </el-radio-group>
          <el-table :data="telescopeSubTab === 'qualified' ? telescopePreview.qualifiedList : telescopeSubTab === 'unfit' ? telescopePreview.unfitList : telescopePreview.pendingList"
            size="small" max-height="300" empty-text="本类暂无人选">
            <el-table-column prop="candidateName" label="候选人" width="110" />
            <el-table-column label="逐项核验" min-width="300">
              <template #default="{ row }">
                <div v-for="item in row.ruleEvidence" :key="item.rule" style="font-size: 12px; line-height: 1.6">
                  <el-tag :type="item.status === 'pass' ? 'success' : item.status === 'fail' ? 'danger' : 'info'" size="small">{{ item.status === 'pass' ? '通过' : item.status === 'fail' ? '不符' : item.status === 'skipped' ? '不作依据' : '资料不足' }}</el-tag>
                  {{ item.label }}：{{ item.evidence }}
                  <template v-if="item.rule?.startsWith('TELESCOPE_') && item.rule !== 'TELESCOPE_DETAIL'">
                    <el-button link size="small" type="success" @click="confirmTelescopeCondition(row.candidateId, item.rule, 'pass')">确认符合</el-button>
                    <el-button link size="small" type="danger" @click="confirmTelescopeCondition(row.candidateId, item.rule, 'fail')">确认不符</el-button>
                  </template>
                </div>
              </template>
            </el-table-column>
          </el-table>
          <div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-top:14px">
            <span>本批 Hi聊点数上限</span>
            <el-input-number v-model="telescopePointsLimit" :min="1" :max="100000" :step="100" size="small" />
            <el-button type="primary" :loading="telescopeActionBusy" :disabled="!telescopePreview.qualifiedCount || telescopeActionBusy || telescopeScanning" @click="runTelescopeHi">一键 Hi聊已入库合格者</el-button>
            <el-button type="danger" :loading="telescopeActionBusy" :disabled="!telescopePreview.unfitCount || telescopeActionBusy || telescopeScanning" @click="runTelescopeReject">检查未处理的不感兴趣</el-button>
          </div>
          <p v-if="telescopeActionSummary" class="catalog-status">{{ telescopeActionSummary }}</p>
          <el-table v-if="telescopeActionItems.length" :data="telescopeActionItems" size="small" max-height="240" style="margin-top:10px">
            <el-table-column prop="name" label="候选人" width="120" />
            <el-table-column label="状态" width="130">
              <template #default="{ row }">
                {{ getTelescopeActionItemLabel(row.status) }}
              </template>
            </el-table-column>
            <el-table-column label="平台回执或跳过原因" min-width="300">
              <template #default="{ row }">
                {{ row.reason }}
                <el-button v-if="row.reason?.includes('项目文案待重试')" link type="primary" @click="retryTelescopeProjectMessage(row)">重试项目文案</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <!-- Tab 2: 岗位专属硬性淘汰条件 -->
      <el-tab-pane label="岗位专属硬性条件" name="criteria">
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
          <template #default>
            仅在完整资料分析后，有可定位确凿证据违反硬性条件时才执行淘汰。信息缺失、矛盾或低分绝不自动拒绝，年龄与性别仅展示不用于淘汰。
          </template>
        </el-alert>

        <el-form label-position="top">
          <el-form-item label="当前配置针对的本地职位专属库">
            <el-select v-model="projectId" filterable style="width: 100%" placeholder="请选择需要配置硬性条件的职位专属库">
              <el-option v-for="project in projects" :key="project.id" :value="project.id"
                :label="`${project.job_config?.title || project.name} · ${project.name}`" />
            </el-select>
          </el-form-item>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="最低学历硬性要求">
                <el-select v-model="strictCriteria.minEducation" style="width: 100%">
                  <el-option label="不限" value="" />
                  <el-option label="大专及以上" value="大专" />
                  <el-option label="本科及以上" value="本科" />
                  <el-option label="硕士及以上" value="硕士" />
                  <el-option label="博士及以上" value="博士" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="最低工作年限要求（年）">
                <el-input-number v-model="strictCriteria.minWorkYears" :min="0" :max="40" :step="0.5" style="width: 100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="年龄区间要求（岁）">
                <div style="display: flex; align-items: center; gap: 8px">
                  <el-input-number v-model="strictCriteria.minAge" :min="0" :max="80" placeholder="最小" style="width: 48%" />
                  <span style="color: #94a3b8">至</span>
                  <el-input-number v-model="strictCriteria.maxAge" :min="0" :max="80" placeholder="最大" style="width: 48%" />
                </div>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="期望薪资上限（千元/月）">
                <el-input-number v-model="strictCriteria.maxExpectedSalary" :min="0" :max="200" placeholder="如 15 代表 15k/月，0 为不限" style="width: 100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="性别要求">
                <el-select v-model="strictCriteria.gender" style="width: 100%" placeholder="性别要求">
                  <el-option label="不限" value="" />
                  <el-option label="仅限男性" value="男" />
                  <el-option label="仅限女性" value="女" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="求职状态限制">
                <el-select v-model="strictCriteria.jobSeekingStatus" style="width: 100%" placeholder="求职状态">
                  <el-option label="不限" value="" />
                  <el-option label="仅离职（随时到岗）" value="离职" />
                  <el-option label="在职（考虑机会）" value="在职" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="人才活跃度要求">
                <el-select v-model="strictCriteria.activityDays" style="width: 100%">
                  <el-option label="不限活跃时间" :value="0" />
                  <el-option label="近 3 天活跃" :value="3" />
                  <el-option label="近 7 天活跃" :value="7" />
                  <el-option label="近 30 天活跃" :value="30" />
                  <el-option label="近 3 个月（90 天，默认）" :value="90" />
                </el-select>
                <div v-if="strictCriteria.activityDays === 7" style="font-size: 11px; color: #d97706; margin-top: 4px; line-height: 1.4">
                  提示：当前项目使用近 7 天活跃标准（条件版本：{{ strictCriteria.criteriaVersion }}）。若希望放宽筛选池，可切换为【近 3 个月（90 天）】后保存，新版本将重评未处理名单。
                </div>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="条件版本标识">
                <el-input :model-value="strictCriteria.criteriaVersion || 'v1'" disabled placeholder="保存后自动递增版本号" style="width: 100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="项目专属推介文案（平台打招呼后发送）">
            <el-input
              v-model="strictCriteria.projectIntroMessage"
              type="textarea"
              :rows="3"
              placeholder="如：您好！关注到您的经历与我们目前急聘的【研发专家】岗位非常契合，欢迎了解职位详情并投递沟通！"
              clearable
            />
          </el-form-item>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="每日定时计划时间">
                <el-input v-model="strictCriteria.scheduleTime" placeholder="例如 09:30" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="定时自动初筛状态" style="display: flex; align-items: center; margin-top: 28px">
                <el-switch v-model="strictCriteria.scheduleEnabled" active-text="启用每日定时初筛" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="专业背景关键词匹配（满足其一即可通过）">
            <el-input
              v-model="strictCriteria.majorKeywords"
              placeholder="如：医学, 药学, 市场营销, 生物制药（逗号分隔，留空表示不限专业）"
              clearable
            />
          </el-form-item>

          <el-form-item label="工作城市意向限制">
            <div style="display: flex; align-items: center; gap: 12px; width: 100%">
              <el-checkbox v-model="strictCriteria.rejectNonLocalWillingness">
                排除明确表示不愿在目标城市工作的候选人
              </el-checkbox>
              <el-input v-model="strictCriteria.targetCity" placeholder="目标城市（如：上海）" style="width: 160px"
                :disabled="!strictCriteria.rejectNonLocalWillingness" />
            </div>
            <div style="font-size: 12px; color: #94a3b8; margin-top: 4px">
              仅当候选人求职意向明确排除目标城市时触发；异地现居地但有意向者不触发。
            </div>
          </el-form-item>

          <el-form-item>
            <div style="background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; padding: 12px 16px; width: 100%">
              <el-checkbox v-model="strictCriteria.autoRejectPlatform">
                <span style="font-weight: 500; color: #1e293b">联动 51job 网页端自动标记【不合适】</span>
                <span style="font-size: 12px; color: #64748b; display: block; margin-top: 2px">
                  对来源于 51job 的候选人，在识别为不符合硬性条件时，自动在 51job 企业后台点击标记不合适并提交淘汰理由。
                </span>
              </el-checkbox>
            </div>
          </el-form-item>

          <el-form-item style="margin-top: 16px">
            <div style="background: #fef2f2; border: 1px solid #fee2e2; border-radius: 6px; padding: 12px 16px; width: 100%">
              <el-checkbox v-model="strictCriteria.complianceConfirmed">
                <span style="font-weight: 600; color: #991b1b">HR 合规授权确认</span>
                <span style="font-size: 12px; color: #b91c1c; display: block; margin-top: 2px">
                  确认已人工核实本岗位的硬性门槛，授权仅对完整资料且证据确凿的「待处理」人选执行平台不合适标记。
                </span>
              </el-checkbox>
            </div>
          </el-form-item>

          <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 16px; border-top: 1px dashed #cbd5e1; padding-top: 12px">
            <div style="display: flex; gap: 12px; align-items: center">
              <el-button type="primary" :disabled="!projectId" :loading="savingCriteria" @click="saveCriteria">
                保存岗位硬性条件
              </el-button>
              <span v-if="!projectId" style="font-size: 12px; color: #f59e0b">请先选择上方职位专属库后保存</span>
              <span v-if="saveStatus" class="catalog-status">{{ saveStatus }}</span>
            </div>
            <el-button type="warning" plain :disabled="!projectId" :loading="rescreening" @click="rescreenCurrentProject">
              一键旧库全量重筛
            </el-button>
          </div>
        </el-form>
      </el-tab-pane>

    </el-tabs>
    </div>

    <template #footer>
      <div style="display: flex; justify-content: space-between; align-items: center; width: 100%">
        <div style="display: flex; gap: 8px">
          <el-button type="warning" plain :disabled="running || scanning || executingAction" @click="openBrowser">打开 51job 浏览器</el-button>
          <el-button :loading="loading" :disabled="running || scanning || executingAction" @click="load">重新读取职位</el-button>
        </div>
        <div style="display: flex; gap: 8px">
          <el-button :disabled="running || scanning || executingAction" @click="visible = false">关闭</el-button>
          <el-button v-if="activeTab === 'criteria'" type="primary" :disabled="!projectId" :loading="savingCriteria"
            @click="saveCriteria">保存硬性条件</el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh } from '@element-plus/icons-vue'

const props = defineProps<{ modelValue: boolean; projects: any[]; defaultProjectId?: string; initialTab?: 'sync' | 'telescope' }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; completed: [] }>()
const visible = computed({ get: () => props.modelValue, set: value => emit('update:modelValue', value) })

const activeTab = ref('sync')
const jobs = ref<Array<{ jobId: string; title: string; workArea?: string; degree?: string; workYear?: string; salary?: string }>>([])
const telescopeJobs = ref<Array<{ jobId: string; title: string; workArea?: string; degree?: string; workYear?: string; salary?: string }>>([])
const mappings = ref<any[]>([])
const accountScope = ref('')
const telescopeAccountScope = ref('')
const jobId = ref('')
const telescopeJobId = ref('')
const projectId = ref('')
const selectedJob = computed(() => jobs.value.find(j => j.jobId === jobId.value))
const statuses = ref<number[]>([1, 2, 3, 4, 5])
const loading = ref(false)
const telescopeLoading = ref(false)
const catalogStale = ref(true)
const telescopeCatalogStale = ref(true)
const catalogStatus = ref('')
const telescopeCatalogStatus = ref('')
const saveStatus = ref('')
let loadGeneration = 0
const running = ref(false)
const stopRequested = ref(false)
const progress = ref('')
const summary = ref('')
const pendingReviews = ref<any[]>([])
const manualUnfitReviews = computed(() => pendingReviews.value.filter((item: any) => item.verdict === 'unfit' && ['待处理', '待定'].includes(item.platformStatus)))

const syncTask = ref<any>(null)
const attachmentTask = ref<any>(null)
const autoSyncEnabled = ref(false)
const savingCriteria = ref(false)

// 51job 共用初筛框架状态
const batchPreview = ref<any>(null)
const scanning = ref(false)
const previewSubTab = ref<'qualified' | 'unfit' | 'pending'>('qualified')
const selectedUnfitCandidates = ref<string[]>([])
const executingAction = ref(false)
const rescreening = ref(false)
const telescopePreview = ref<any>(null)
const telescopeScanning = ref(false)
const telescopeSubTab = ref<'qualified' | 'unfit' | 'pending'>('qualified')
type TelescopeCondition = { text: string; kind: 'skill' | 'other'; required: boolean; source: string }
const telescopeConditions = ref<TelescopeCondition[]>([])
const telescopeConditionConfig = ref<any>(null)
async function loadTelescopeConditions() {
  if (!projectId.value) { telescopeConditions.value = []; telescopeConditionConfig.value = null; return }
  try { const api: any = await import('../../wailsjs/go/backend/App'); const config = await api.GetTelescopeConditionConfig(projectId.value); telescopeConditionConfig.value = config; telescopeConditions.value = (config.conditions || []).map((item: TelescopeCondition) => ({ ...item })) }
  catch (error: any) { ElMessage.error(error?.message || String(error)) }
}
async function saveTelescopeConditions() {
  if (!projectId.value) return
  const api: any = await import('../../wailsjs/go/backend/App')
  const config = await api.SaveTelescopeConditionConfig({ projectId: projectId.value, conditions: telescopeConditions.value, version: '', jobSkillsChanged: false, jobSkills: [] })
  telescopeConditionConfig.value = config
  telescopeConditions.value = (config.conditions || []).map((item: TelescopeCondition) => ({ ...item }))
  ElMessage.success('望远镜条件已保存')
}
async function syncTelescopeJobSkills() { const api: any = await import('../../wailsjs/go/backend/App'); const config = await api.SyncTelescopeJobSkills(projectId.value); telescopeConditionConfig.value = config; telescopeConditions.value = (config.conditions || []).map((item: TelescopeCondition) => ({ ...item })); ElMessage.success('岗位技能已同步，自定义条件已保留') }
async function confirmTelescopeCondition(candidateId: string, rule: string, status: string) {
  if (!telescopePreview.value?.batchId) return
  try { const api: any = await import('../../wailsjs/go/backend/App'); telescopePreview.value = await api.ConfirmTelescopeCondition(telescopePreview.value.batchId, candidateId, rule, status); ElMessage.success('人工结论已保存；明确不符者已按平台回执处理') }
  catch (error: any) { ElMessage.error(error?.message || String(error)) }
}
const telescopeProgress = ref('')
const telescopeActionBusy = ref(false)
const telescopePointsLimit = ref(1000)
const telescopeActionSummary = ref('')
const telescopeActionItems = ref<any[]>([])
const savedCriteria = ref<any>(null)

const strictCriteria = ref({
  projectId: '',
  minEducation: '',
  minWorkYears: 0,
  minAge: 0,
  maxAge: 0,
  maxExpectedSalary: 0,
  gender: '',
  jobSeekingStatus: '',
  majorKeywords: '',
  rejectNonLocalWillingness: false,
  targetCity: '',
  activityDays: 90,
  criteriaVersion: 'v1',
  projectIntroMessage: '',
  scheduleTime: '09:30',
  scheduleEnabled: false,
  complianceConfirmed: false,
  autoRejectPlatform: true,
  updatedAt: ''
})

const currentStatusName = computed(() => {
  const map: Record<number, string> = { 1: '待处理', 2: '合适', 3: '面试', 4: '待定', 5: '不合适对账' }
  return map[syncTask.value?.statusIndex || 1] || '待处理'
})

const syncTaskStatusLabel = computed(() => {
  const map: Record<string, string> = {
    idle: '空闲待命',
    running: '同步中',
    paused: '已暂停',
    completed: '已完成一轮检查',
    error: '异常退避'
  }
  return map[syncTask.value?.state || 'idle'] || '空闲'
})

const syncTaskTagType = computed(() => {
  const map: Record<string, any> = {
    idle: 'info',
    running: 'primary',
    paused: 'warning',
    completed: 'success',
    error: 'danger'
  }
  return map[syncTask.value?.state || 'idle'] || 'info'
})

async function load() {
  const generation = ++loadGeneration
  loading.value = true
  catalogStale.value = true
  catalogStatus.value = '正在核验 51job 登录账号和职位…'
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const fresh = await withTimeout((async () => {
      const catalog = await api.Get51JobTalentManagementCatalog()
      const nextMappings = await api.ListPlatformJobMappings() || []
      const nextJobs = catalog.jobs || []
      const nextProjectID = props.defaultProjectId || projectId.value || ''
      const bound = nextMappings.filter((item: any) => item.platform === '51job' && item.projectId === nextProjectID)
      const nextJobID = bound.length === 1 && bound[0].platformAccount === catalog.accountScope &&
        nextJobs.some((job: any) => job.jobId === bound[0].platformJobId) ? bound[0].platformJobId : ''
      const [task, criteria] = await Promise.all([
        nextJobID ? api.GetTalentManagementSyncTask('51job', catalog.accountScope, nextJobID).catch(() => null) : Promise.resolve(null),
        nextProjectID ? api.GetTalentManagementStrictCriteria(nextProjectID).catch(() => null) : Promise.resolve(null)
      ])
      return { catalog, nextMappings, nextJobs, nextJobID, nextProjectID, task, criteria }
    })(), 25000, '重新读取职位')
    if (generation !== loadGeneration) return
    mappings.value = fresh.nextMappings
    jobs.value = fresh.nextJobs
    accountScope.value = fresh.catalog.accountScope || ''
    jobId.value = fresh.nextJobID
    projectId.value = fresh.nextProjectID
    syncTask.value = fresh.task
	void loadAttachmentTask()
    autoSyncEnabled.value = Boolean(fresh.task?.autoSyncEnabled)
    if (fresh.criteria) applyCriteria(fresh.criteria, fresh.nextProjectID)
    catalogStale.value = false
    catalogStatus.value = jobId.value
      ? `已核验当前分库绑定的职位 ID ${jobId.value} · ${new Date().toLocaleTimeString('zh-CN', { hour12: false })}`
      : `已核验 ${jobs.value.length} 个在线职位；本分库尚无有效的唯一绑定，请核对账号并选定职位一次`
    ElMessage.success(`成功读取 ${jobs.value.length} 个 51job 在线职位`)
  } catch (error: any) {
    if (generation !== loadGeneration) return
    jobs.value = []
    jobId.value = ''
    catalogStale.value = true
    catalogStatus.value = classifyCatalogError(error)
    ElMessage.error(catalogStatus.value)
  } finally {
    if (generation === loadGeneration) loading.value = false
  }
}

async function loadTelescopeCatalog() {
  telescopeLoading.value = true
  telescopeCatalogStale.value = true
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const catalog = await api.Get51JobTelescopeCatalog()
    if (catalog && catalog.jobs && catalog.jobs.length > 0) {
      telescopeJobs.value = catalog.jobs
      telescopeAccountScope.value = catalog.accountScope || ''
      telescopeCatalogStale.value = false
      if (!telescopeJobId.value || !telescopeJobs.value.some((j: any) => j.jobId === telescopeJobId.value)) {
        telescopeJobId.value = catalog.jobs[0]?.jobId || ''
      }
      if (!mappings.value.length) mappings.value = await api.ListPlatformJobMappings() || []
      const mapped = mappings.value.find(item => item.platform === '51job' &&
        item.platformAccount === telescopeAccountScope.value && item.platformJobId === telescopeJobId.value)
      if (mapped?.projectId && !projectId.value) projectId.value = mapped.projectId
      telescopeCatalogStatus.value = `已从人才望远镜读取 ${catalog.jobs.length} 个职位 · ${new Date().toLocaleTimeString('zh-CN', { hour12: false })}`
      ElMessage.success(`已从人才望远镜读取 ${catalog.jobs.length} 个职位`)
    } else {
      telescopeCatalogStatus.value = '望远镜页面尚未显示职位，请从 51job 菜单进入并等待职位标签加载后重试'
    }
  } catch (e: any) {
    telescopeJobs.value = []
    telescopeJobId.value = ''
    telescopeCatalogStatus.value = e?.message || '读取人才望远镜职位失败，请确认已打开望远镜页面'
    ElMessage.error(telescopeCatalogStatus.value)
  } finally {
    telescopeLoading.value = false
  }
}

function withTimeout<T>(promise: Promise<T>, timeoutMS: number, action: string): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => reject(new Error(`${action}超过 ${Math.round(timeoutMS / 1000)} 秒，请检查浏览器登录或本地数据服务`)), timeoutMS)
    promise.then(value => { window.clearTimeout(timer); resolve(value) }, error => { window.clearTimeout(timer); reject(error) })
  })
}

function classifyCatalogError(error: any): string {
  const message = String(error?.message || error || '')
  if (/超时|timeout|超过/.test(message)) return `读取超时：${message}`
  if (/登录|账号|企业身份/.test(message)) return `51job 登录状态待核实：${message}`
  if (/浏览器|9503|连接/.test(message)) return `51job 专用浏览器未就绪：${message}`
  if (/database|sqlite|locked|busy|数据库/.test(message)) return `本地数据库繁忙：${message}`
  return `职位读取失败：${message || '平台页面尚未加载完成'}`
}

function syncMappedProject() {
  const mapped = mappings.value.find(item => item.platform === '51job' &&
    item.platformAccount === accountScope.value && item.platformJobId === jobId.value)
  if (mapped?.projectId && mapped.projectId !== projectId.value) {
    ElMessage.warning('此职位已绑定其他专属库，请选择本项目的职位 ID')
  }
}

async function bindSelectedJob(api: any): Promise<void> {
  const used = mappings.value.find(item => item.platform === '51job' &&
    item.platformAccount === accountScope.value && item.platformJobId === jobId.value)
  if (used?.projectId && used.projectId !== projectId.value) throw new Error('该 51job 职位已绑定其他专属库，不能串库导入')
  const previous = mappings.value.filter(item => item.platform === '51job' && item.projectId === projectId.value)
  if (previous.length === 1 && (previous[0].platformAccount !== accountScope.value || previous[0].platformJobId !== jobId.value)) {
    await ElMessageBox.confirm(`此分库已绑定职位 ID ${previous[0].platformJobId}。确定改绑为 ${jobId.value} 吗？原职位简历关联将归档。`, '确认改绑职位', { type: 'warning' })
  }
  await api.Bind51JobTalentManagementJob(accountScope.value, jobId.value, projectId.value)
  mappings.value = await api.ListPlatformJobMappings() || []
}

function formatJobOptionLabel(job: any) {
  if (!job) return ''
  const loc = job.workArea ? ` · ${job.workArea}` : ''
  const details = [job.degree, job.workYear, job.salary].filter(Boolean).join(' | ')
  const extra = details ? ` | ${details}` : ''
  return `${job.title}${loc} (ID: ${job.jobId}${extra})`
}

async function openBrowser() {
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await api.Open51JobManagementBrowser()
    ElMessage.info('已呼出 51job 专用浏览器；若当前不是人才管理页，请从企业首页菜单进入「人才管理」后重新读取')
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
  }
}

async function loadSyncTask() {
  if (!accountScope.value || !jobId.value) return
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const task = await api.GetTalentManagementSyncTask('51job', accountScope.value, jobId.value)
    syncTask.value = task
    autoSyncEnabled.value = Boolean(task?.autoSyncEnabled)
  } catch (e) {
    syncTask.value = null
  }
}

async function loadAttachmentTask() {
  if (!projectId.value) { attachmentTask.value = null; return }
  const requestedProject = projectId.value
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const task = await api.Get51JobAttachmentSyncTask(requestedProject)
    if (projectId.value === requestedProject) attachmentTask.value = task
  } catch { if (projectId.value === requestedProject) attachmentTask.value = null }
}

async function stopAttachmentSync() {
  if (!projectId.value) return
  const api: any = await import('../../wailsjs/go/backend/App')
  await api.Stop51JobAttachmentSync(projectId.value)
  ElMessage.info('正在中止附件检查，已保存的文件会保留')
}

async function resumeAttachmentSync() {
  if (!projectId.value) return
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    attachmentTask.value = await api.Start51JobAttachmentSync(projectId.value)
  } catch (error: any) { ElMessage.error(error?.message || String(error)) }
}

async function toggleAutoSync(val: boolean) {
  if (!accountScope.value || !jobId.value) return
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await api.SetTalentManagementAutoSync('51job', accountScope.value, jobId.value, val)
    ElMessage.success(val ? '已开启每小时自动检查' : '已关闭自动检查')
  } catch (e: any) {
    autoSyncEnabled.value = !val
    ElMessage.error(e?.message || '设置自动同步失败')
  }
}

async function resetTask() {
  if (!accountScope.value || !jobId.value) return
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await api.ResetTalentManagementSyncTask('51job', accountScope.value, jobId.value)
    await loadSyncTask()
    ElMessage.success('断点已重置至第 1 状态第 1 页')
  } catch (e: any) {
    ElMessage.error(e?.message || '重置失败')
  }
}

async function loadCriteria() {
  if (!projectId.value) return
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const c = await api.GetTalentManagementStrictCriteria(projectId.value)
    if (c) applyCriteria(c, projectId.value)
  } catch (e) {}
}

function applyCriteria(c: any, selectedProjectID: string) {
  strictCriteria.value = {
    projectId: selectedProjectID,
    minEducation: c.minEducation || '',
    minWorkYears: c.minWorkYears || 0,
    minAge: c.minAge || 0,
    maxAge: c.maxAge || 0,
    maxExpectedSalary: c.maxExpectedSalary || 0,
    gender: c.gender || '',
    jobSeekingStatus: c.jobSeekingStatus || '',
    majorKeywords: c.majorKeywords || '',
    rejectNonLocalWillingness: Boolean(c.rejectNonLocalWillingness),
    targetCity: c.targetCity || '',
    activityDays: c.activityDays !== undefined ? c.activityDays : 90,
    criteriaVersion: c.criteriaVersion || 'v1',
    projectIntroMessage: c.projectIntroMessage || '',
    scheduleTime: c.scheduleTime || '09:30',
    scheduleEnabled: Boolean(c.scheduleEnabled),
    complianceConfirmed: Boolean(c.complianceConfirmed),
    autoRejectPlatform: c.autoRejectPlatform !== false,
    updatedAt: c.updatedAt || ''
  }
  savedCriteria.value = { ...strictCriteria.value }
}

function handleUnfitSelectionChange(selection: any[]) {
  selectedUnfitCandidates.value = selection.map(item => item.candidateId)
}

function getActionStatusLabel(status: string): string {
  switch (status) {
    case 'pending': return '待处理'
    case 'executing': return '执行中'
    case 'receipt_pending': return '待核实回执'
    case 'retrying': return '待重试'
    case 'completed': return '已标记不合适'
    case 'verified_on_platform': return '平台已处理／已核对'
    case 'manual_required': return '需人工处理'
    case 'failed': return '执行失败'
    case 'imported': return '已入库'
    default: return status || '待处理'
  }
}

function getTelescopeActionItemLabel(status: string): string {
  switch (status) {
    case 'rejected': return '已转未匹配'
    case 'sent': return '已打招呼'
    case 'uncertain': return '回执待核查'
    case 'skipped': return '已跳过'
    default: return status || '待处理'
  }
}

function getActionStatusTagType(status: string): any {
  switch (status) {
    case 'completed':
    case 'verified_on_platform':
    case 'imported':
      return 'success'
    case 'executing':
      return 'primary'
    case 'receipt_pending':
    case 'retrying':
      return 'warning'
    case 'failed':
    case 'manual_required':
      return 'danger'
    default:
      return 'info'
  }
}

async function checkAndResumeProject() {
  if (!projectId.value) return
  scanning.value = true
  progress.value = '正在核对任务检查点并进行平台状态对账…'
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const preview = await api.CheckAndResumeProjectScreening(projectId.value)
    batchPreview.value = preview
    selectedUnfitCandidates.value = (preview.unfitList || []).filter((c: any) => c.actionStatus !== 'completed' && c.actionStatus !== 'verified_on_platform').map((c: any) => c.candidateId)
    progress.value = `任务核查完成：合格 ${preview.qualifiedCount} 人，不合格 ${preview.unfitCount} 人，资料不足待补读 ${preview.pendingCount} 人`
    ElMessage.success('任务检查与状态对账完成')
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
  } finally {
    scanning.value = false
  }
}

async function scanBatchPreview() {
  if (!jobId.value || !projectId.value || !accountScope.value) return
  scanning.value = true
  summary.value = ''
  progress.value = '正在扫描平台待处理人选并执行三态合规核验...'
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await bindSelectedJob(api)
    await api.BeginProjectScreening(projectId.value)
    const preview = await api.Scan51JobTalentManagementBatch(accountScope.value, jobId.value, projectId.value, 0)
    batchPreview.value = preview
    selectedUnfitCandidates.value = (preview.unfitList || []).filter((c: any) => c.actionStatus !== 'completed' && c.actionStatus !== 'verified_on_platform').map((c: any) => c.candidateId)
    progress.value = preview.scanStatus === 'manual_required' ? '已中止扫描，断点已保留；可手动继续' : `扫描结果：合格 ${preview.qualifiedCount} 人，不合格 ${preview.unfitCount} 人，资料不足待补读 ${preview.pendingCount} 人`
    if (preview.scanStatus === 'complete') ElMessage.success(`初筛预览就绪：共核验 ${preview.totalCount} 位候选人`)
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
    progress.value = '扫描生成预览失败'
  } finally {
    scanning.value = false
  }
}

onMounted(() => {
  const runtime: any = (window as any).runtime
  if (runtime && runtime.EventsOn) {
    runtime.EventsOn('screening_candidate_updated', (event: any) => {
      if (!batchPreview.value || batchPreview.value.batchId !== event.batchId) return
      const cand = (batchPreview.value.unfitList || []).find((c: any) => c.candidateId === event.candidateId)
      if (cand) {
        cand.actionStatus = event.actionStatus
        cand.actionReceiptAt = event.actionReceiptAt || ''
        cand.actionError = event.actionError || ''
      }
    })
    runtime.EventsOn('screening_scan_progress', (event: any) => {
      if (event && event.statusText) {
        progress.value = event.statusText
        if (batchPreview.value && (!event.batchId || batchPreview.value.batchId === event.batchId)) {
          batchPreview.value.scanStatus = 'scanning'
          batchPreview.value.lastError = ''
          if (event.page) batchPreview.value.currentPage = event.page
          if (event.totalPages) batchPreview.value.totalPagesEstimated = event.totalPages
        }
      }
    })
    runtime.EventsOn('talent_management_import_progress', (event: any) => {
      if (running.value && event?.statusText) progress.value = event.statusText
    })
	 runtime.EventsOn('telescope_scan_progress', (event: any) => {
	   if (telescopeScanning.value) telescopeProgress.value = `正在读取望远镜详情：${event.current}/${event.total} · ${event.name || '候选人'}`
	 })
	 runtime.EventsOn('attachment_sync_progress', (task: any) => {
	   if (task?.projectId === projectId.value) attachmentTask.value = task
	 })
  }
})

watch(activeTab, (tab) => {
  if (tab === 'telescope' && !telescopeLoading.value && (!telescopeJobs.value.length || telescopeCatalogStale.value)) void loadTelescopeCatalog()
  if (tab === 'sync' && (!jobs.value.length || catalogStale.value)) void load()
})

watch(telescopeJobId, () => {
  telescopePreview.value = null
  const mapped = mappings.value.find(item => item.platform === '51job' &&
    item.platformAccount === telescopeAccountScope.value && item.platformJobId === telescopeJobId.value)
  if (mapped?.projectId) projectId.value = mapped.projectId
})

async function openTelescope() {
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await api.Open51JobTelescopeBrowser()
    ElMessage.success('已自动通过 51job 菜单进入人才望远镜')
    await loadTelescopeCatalog()
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
  }
}

async function scanTelescopePreview() {
  if (!telescopeJobId.value || !projectId.value) {
    ElMessage.warning('请先选择 51job 望远镜职位与本地职位专属库')
    return
  }
  telescopeScanning.value = true
  telescopePreview.value = null
  telescopeActionSummary.value = ''
  telescopeActionItems.value = []
  telescopeProgress.value = '正在读取人才望远镜当前职位并使用项目条件筛选…'
  try {
    await saveTelescopeConditions()
    const api: any = await import('../../wailsjs/go/backend/App')
    const preview = await api.Scan51JobTelescopeBatch(telescopeAccountScope.value, telescopeJobId.value, projectId.value)
    telescopePreview.value = preview
		telescopeProgress.value = `筛选完成：合格 ${preview.qualifiedCount} 人，不合格 ${preview.unfitCount} 人；正在自动处理明确不符者…`
    if (preview.unfitCount > 0) {
      const rejection = await api.Run51JobTelescopeReject(preview.batchId)
      telescopeActionItems.value = rejection.items || []
      telescopeActionSummary.value = `平台处理：已确认转入未匹配 ${rejection.rejected} 人，跳过 ${rejection.skipped} 人，回执待核查 ${rejection.uncertain} 人`
    }
    telescopeProgress.value = `筛选完成：合格 ${preview.qualifiedCount} 人，不合格 ${preview.unfitCount} 人；${telescopeActionSummary.value || '无须执行不感兴趣'}`
    ElMessage.success(telescopeProgress.value)
  } catch (error: any) {
    telescopeProgress.value = error?.message || String(error)
    ElMessage.error(telescopeProgress.value)
  } finally {
    telescopeScanning.value = false
  }
}

async function runTelescopeHi() {
  if (!telescopePreview.value || telescopeActionBusy.value) return
  telescopeActionBusy.value = true
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const ids = telescopePreview.value.qualifiedList.map((item: any) => item.candidateId)
    const result = await api.Run51JobOneClickHi(projectId.value, telescopeJobId.value, ids, telescopePointsLimit.value)
    telescopeActionItems.value = result.items || []
    telescopeActionSummary.value = `Hi聊：已联系 ${result.sent} 人，跳过 ${result.skipped} 人，回执待核查 ${result.uncertain} 人；已使用 ${result.pointsUsed}/${result.pointsLimit} 点`
    if (result.uncertain) ElMessage.warning(telescopeActionSummary.value)
    else ElMessage.success(telescopeActionSummary.value)
    emit('completed')
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
  } finally { telescopeActionBusy.value = false }
}

async function runTelescopeReject() {
  if (!telescopePreview.value || telescopeActionBusy.value) return
  telescopeActionBusy.value = true
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const result = await api.Run51JobTelescopeReject(telescopePreview.value.batchId)
    telescopeActionItems.value = result.items || []
    telescopeActionSummary.value = `平台处理：已确认转入未匹配 ${result.rejected} 人，跳过 ${result.skipped} 人，回执待核查 ${result.uncertain} 人`
    if (result.uncertain) ElMessage.warning(telescopeActionSummary.value)
    else ElMessage.success(telescopeActionSummary.value)
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
  } finally { telescopeActionBusy.value = false }
}

async function retryTelescopeProjectMessage(row: any) {
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    const result = await api.Retry51JobProjectMessage(projectId.value, row.candidateId)
    row.reason = result.message || row.reason
    if (result.status === 'sent') ElMessage.success(result.message)
    else ElMessage.warning(result.message)
  } catch (error: any) { ElMessage.error(error?.message || String(error)) }
}

async function confirmRejectUnfit() {
  if (!batchPreview.value?.batchId || !selectedUnfitCandidates.value.length) return
  try {
    await ElMessageBox.confirm(`确定要对已选的 ${selectedUnfitCandidates.value.length} 位不符合硬性条件的候选人在 51job 平台批量标记「不合适」吗？处理结果会自动保存，以避免重复操作。`, '平台操作确认', {
      confirmButtonText: '确认标记不合适',
      cancelButtonText: '取消',
      type: 'warning'
    })
    executingAction.value = true
    const api: any = await import('../../wailsjs/go/backend/App')
    const ledgers = await api.ConfirmScreeningRejectUnfit(batchPreview.value.batchId, selectedUnfitCandidates.value)
    const successCount = (ledgers || []).filter((l: any) => l.status === 'success').length
    ElMessage.success(`已完成平台标记：成功 ${successCount} 人`)
    batchPreview.value = await api.GetScreeningBatchPreview(batchPreview.value.batchId)
    emit('completed')
  } catch (err: any) {
    if (err !== 'cancel') ElMessage.error(err?.message || String(err))
  } finally {
    executingAction.value = false
  }
}

async function rescreenCurrentProject() {
  if (!projectId.value) return
  try {
    await ElMessageBox.confirm('一键旧库全量重筛将使用当前保存的岗位硬性条件，对当前专属库全部已入库人选重新进行合规排查。不合格人选将被移至待处理/归档区并留存审计日志，不会自动联动平台执行拒绝。确定继续吗？', '旧库重筛确认', {
      confirmButtonText: '确定重筛',
      cancelButtonText: '取消',
      type: 'warning'
    })
    rescreening.value = true
    const api: any = await import('../../wailsjs/go/backend/App')
    const res = await api.RescreenProjectRepository(projectId.value)
    ElMessage.success(`旧库重筛完成：共核验 ${res.totalChecked} 人，排除不合规 ${res.unfitCount} 人，保留合格 ${res.activeRemainingCount} 人`)
    emit('completed')
  } catch (err: any) {
    if (err !== 'cancel') ElMessage.error(err?.message || String(err))
  } finally {
    rescreening.value = false
  }
}

function criteriaMatches(saved: any, requested: any): boolean {
  return ['minEducation', 'minWorkYears', 'minAge', 'maxAge', 'maxExpectedSalary', 'gender',
    'jobSeekingStatus', 'majorKeywords', 'rejectNonLocalWillingness', 'targetCity',
    'complianceConfirmed', 'autoRejectPlatform'].every(key =>
      String(saved?.[key] ?? '') === String(requested?.[key] ?? ''))
}

async function saveCriteria() {
  if (!projectId.value) {
    ElMessage.warning('请先在上方选择需要保存硬性条件的本地职位专属库')
    return
  }
  savingCriteria.value = true
  saveStatus.value = '正在保存…'
  const requested = { ...strictCriteria.value, projectId: projectId.value }
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await withTimeout(api.SetTalentManagementStrictCriteria(requested), 10000, '保存硬性条件')
    const saved = await withTimeout(api.GetTalentManagementStrictCriteria(projectId.value), 5000, '回读硬性条件')
    if (saved) {
      applyCriteria(saved, projectId.value)
      saveStatus.value = `已保存 · ${saved.updatedAt || new Date().toLocaleString('zh-CN')}`
      ElMessage.success('岗位专属硬性条件已成功保存并同步')
    }
  } catch (e: any) {
    saveStatus.value = `保存异常：${e?.message || '本地数据服务未响应'}`
    ElMessage.error(saveStatus.value)
  } finally {
    savingCriteria.value = false
  }
}

watch(jobId, () => {
  telescopePreview.value = null
  telescopeProgress.value = ''
  syncMappedProject()
  void loadSyncTask()
})

watch(projectId, () => {
  telescopePreview.value = null
  savedCriteria.value = null
  void loadCriteria()
  void loadTelescopeConditions()
  void loadPendingReviews()
  void loadAttachmentTask()
})

async function loadPendingReviews() {
  if (!projectId.value) { pendingReviews.value = []; return }
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    pendingReviews.value = await api.ListTalentManagementPendingReviews(projectId.value) || []
  } catch { pendingReviews.value = [] }
}

async function requestStopImport() {
  stopRequested.value = true
  progress.value = '正在中止人才管理同步，已抓取结果会保留…'
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await api.Stop51JobManagementImport()
  } catch (error: any) {
    ElMessage.warning(error?.message || '中止信号未送达，当前页完成后仍会停止')
  }
}

async function requestStopScreening() {
  progress.value = '正在中止扫描并保留分页断点…'
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await api.StopProjectScreening(projectId.value)
  } catch (error: any) {
    ElMessage.error(error?.message || '中止扫描失败')
  }
}

watch(() => props.modelValue, value => {
  if (value) {
    void loadPendingReviews()
    activeTab.value = props.initialTab || 'sync'
    if (activeTab.value === 'telescope') {
      projectId.value = props.defaultProjectId || projectId.value
      void loadTelescopeCatalog()
    } else {
      void load()
    }
  }
})

async function runSingleBatch() {
  if (!jobId.value || !projectId.value || !accountScope.value) return
  running.value = true
  summary.value = ''
  progress.value = '正在执行单批断点导入...'
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await bindSelectedJob(api)
    const result = await api.RunTalentManagementNextBatch(accountScope.value, jobId.value)
    await loadSyncTask()
    if (result.interceptedCount > 0) {
      summary.value = `单批完成：新增 ${result.addedCount || 0} · 拦截排除 ${result.interceptedCount} 人（可在【淘汰审计记录】查看） · 重复 ${result.duplicateCount || 0}`
      if (!result.addedCount) {
        ElMessage.warning(`本次扫描的候选人均不符合岗位硬性要求（${result.interceptReason || '学历不达标'}），已自动拦截并记录在【淘汰审计记录】`)
      }
    } else {
      summary.value = `单批完成：新增 ${result.addedCount || 0} · 待补全 ${result.pendingDocumentCount || 0} · 重复 ${result.duplicateCount || 0}`
    }
    progress.value = result.hasMore ? '本状态仍有下一页，可继续同步' : '本状态已无更多，已自动推进游标'
    emit('completed')
  } catch (error: any) {
    ElMessage.error(error?.message || String(error))
    progress.value = '单批导入出错'
  } finally {
    running.value = false
  }
}

async function start() {
  if (!jobId.value || !projectId.value || !accountScope.value || !statuses.value.length) return
  running.value = true
  stopRequested.value = false
  summary.value = ''
  const totals = { added: 0, duplicate: 0, pending: 0, failed: 0, intercepted: 0, rejected: 0, rejectPending: 0 }
  try {
    const api: any = await import('../../wailsjs/go/backend/App')
    await api.Begin51JobManagementImport()
    await bindSelectedJob(api)
    for (const status of [...statuses.value]) {
      let page = status === 1 ? await api.CountTalentManagementPendingPages(accountScope.value, jobId.value) : 1
      while (!stopRequested.value) {
        progress.value = `正在处理 ${['', '待处理', '合适', '面试', '待定', '不合适对账'][status]} · 第 ${page} 页`
        const result = await api.Import51JobTalentManagementPage(accountScope.value, jobId.value, status, page)
        totals.added += result.addedCount || 0
        totals.duplicate += result.duplicateCount || 0
        totals.pending += result.pendingVerificationCount || 0
        totals.failed += result.failedCount || 0
        totals.intercepted += result.interceptedCount || 0
        totals.rejected += result.platformRejectedCount || 0
        totals.rejectPending += result.platformRejectPendingCount || 0
        if (result.interrupted) stopRequested.value = true
        await loadPendingReviews()
        emit('completed')
        if (stopRequested.value) break
        if (status === 1) {
          page--
          if (page < 1) break
        } else {
          if (!result.hasMore) break
          page++
        }
      }
      if (stopRequested.value) break
    }
    if (!stopRequested.value && [1, 2, 3, 4, 5].every(status => statuses.value.includes(status))) {
      progress.value = '四类名单已完整同步，正在逐人核验并自动标记明确不符者…'
      const rejection = await api.ProcessTalentManagementUnfitBacklog(accountScope.value, jobId.value)
      totals.rejected += rejection.rejected || 0
      totals.rejectPending += rejection.uncertain || 0
      await loadPendingReviews()
    }
	if (!stopRequested.value) {
	  try { attachmentTask.value = await api.Start51JobAttachmentSync(projectId.value) }
	  catch (error: any) { ElMessage.warning(`名单已同步，附件后台检查未启动：${error?.message || String(error)}`) }
	}
    await loadSyncTask()
    summary.value = `入库 ${totals.added} · 平台不合适已核实 ${totals.rejected} · 平台回执待查询 ${totals.rejectPending} · 资料不足待补读 ${totals.pending} · 已存在 ${totals.duplicate} · 失败 ${totals.failed}`
    progress.value = stopRequested.value ? '已中止；再次同步会重新核验并跳过已入库人选' : '四类状态同步及不合适对账完成'
  } catch (error: any) {
    progress.value = stopRequested.value ? '已中止；已抓取结果保留' : '同步暂停'
    if (!stopRequested.value) ElMessage.error(error?.message || String(error))
  } finally {
    try {
      const api: any = await import('../../wailsjs/go/backend/App')
      await api.End51JobManagementImport()
      await loadPendingReviews()
    } catch { /* 显示已有扫描结果，不能让清理失败锁住按钮。 */ }
    running.value = false
    emit('completed')
  }
}
</script>
