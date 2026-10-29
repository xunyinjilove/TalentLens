/**
 * Read the logged-in 51job talent-management page. This adapter never guesses
 * an identity from a card's text or clicks a status action.
 */
const puppeteer = require('puppeteer-core');

function normalizeCatalog(state) {
  const scope = String(state?.hruid || '').trim();
  if (!scope) throw new Error('无法核实当前 51job 操作员身份');
  const company = String(state?.coid || '').trim();
  if (!company) throw new Error('无法核实当前 51job 企业身份');
  const seen = new Set();
  const jobs = [];
  for (const raw of state?.positions || []) {
    const jobId = String(raw?.jobid || '').trim();
    const title = String(raw?.jobname || '').trim();
    if (!jobId || !title || seen.has(jobId)) continue;
    seen.add(jobId);
    const workArea = String(raw?.workArea || raw?.jobArea || '').trim();
    const degree = String(raw?.degree || '').trim();
    const workYear = String(raw?.workYear || '').trim();
    const salary = String(raw?.salary || '').trim();
    jobs.push({ jobId, title, workArea, degree, workYear, salary });
  }
  if (!jobs.length) throw new Error('当前企业账号没有可核验 ID 的人才管理职位');
  return { accountScope: `co:${company}:hr:${scope}`, jobs };
}

function normalizeCandidate(raw, expectedJobId) {
  const candidateId = String(raw?.userid || raw?.user_id || '').trim();
  const rawJobId = String(raw?.jobid || raw?.job_id || '').trim();
  const jobId = rawJobId || expectedJobId;
  const recordId = String(raw?.cvlogid || raw?.cv_log_id || '').trim();
  if (!candidateId || !jobId || !recordId || jobId !== expectedJobId) return null;
  const hasAttachment = String(raw?.resume_has_enclosure || '') === '1';
  const name = String(raw?.username || raw?.base_info?.resume_name || raw?.base_info?.name || '').trim();
  const resumeText = JSON.stringify(raw);
  return {
    candidateId,
    jobId,
    recordId,
    companyId: String(raw?.coid || '').trim(),
    name,
    hasAttachment,
    platformStatus: String(raw?.recruit_stage || ''),
    resumeText,
    documentsComplete: true,
    documentStatus: hasAttachment ? '在线简历（附带平台附件）' : '在线结构化简历完整',
    raw
  };
}

async function connectManagementPage() {
  let browser;
  try {
    browser = await puppeteer.connect({
      browserURL: 'http://127.0.0.1:9503',
      defaultViewport: null,
      protocolTimeout: 30000
    });
  } catch (err) {
    throw new Error('51job 专用浏览器端口 (9503) 未连接，请先点击【打开 51job 浏览器】并登录企业账号');
  }
  // Avoid calling browser.pages() directly because idle/sleeping tabs or background
  // web-workers can trigger CDP Network.enable timeouts.
  let target = browser.targets().find(t => t.type() === 'page' &&
    t.url().startsWith('https://ehire.51job.com/Revision/talent/management'));
  if (!target) target = await findManagementPageViaMenu(browser);
  let page = null;
  let isTempPage = false;
  if (target) {
    page = await target.page().catch(() => null);
  }
  if (!page) {
    await browser.disconnect().catch(() => {});
    throw new Error('未找到可读取的人才管理页面。请从 51job 企业首页菜单进入「人才管理」后重试');
  } else {
    // 若页面已经存在，等待 Vue 组件就绪（最多 5 秒）
    try {
      await page.waitForFunction(() => {
        const u = window.location.href.toLowerCase();
        if (u.includes('login') || u.includes('navigate.aspx') || u.includes('passport') ||
            u.includes('default.aspx') || (!u.includes('/talent/') && !u.includes('/revision/'))) {
          return 'LOGIN_REDIRECT';
        }
        const components = Array.from(document.querySelectorAll('*')).map(element => element.__vue__).filter(Boolean);
        const management = components.find(vm => vm.$options?.name === 'TalentManagement');
        const menu = components.find(vm => vm.$options?.name === 'EhMenuPositions');
        return Boolean(management?.$data?.hruid && menu?.$props?.positionListsAll?.length);
      }, { timeout: 5000, polling: 250 });
    } catch {
      // 容错继续
    }
  }
  return { browser, page, isTempPage };
}

async function findManagementPageViaMenu(browser) {
  const siteTargets = browser.targets().filter(t => t.type() === 'page' &&
    t.url().startsWith('https://ehire.51job.com/') && !t.url().toLowerCase().includes('/error.aspx'));
  for (const target of siteTargets) {
    const page = await target.page().catch(() => null);
    if (!page) continue;
    const hasMenu = await page.evaluate(() => Boolean(document.querySelector('#sensor_Talentmanagement'))).catch(() => false);
    if (!hasMenu) continue;
    await page.evaluate(() => document.querySelector('#sensor_Talentmanagement').click());
    const deadline = Date.now() + 8000;
    while (Date.now() < deadline) {
      const found = browser.targets().find(t => t.type() === 'page' &&
        t.url().startsWith('https://ehire.51job.com/Revision/talent/management'));
      if (found) return found;
      await new Promise(resolve => setTimeout(resolve, 200));
    }
    throw new Error('已点击 51job「人才管理」菜单，但页面未在 8 秒内打开；请检查登录状态或刷新企业首页');
  }
  throw new Error('未找到已登录且带「人才管理」菜单的 51job 页面，请先在专用浏览器登录企业账号');
}

async function readState(page) {
  return page.evaluate(() => {
    const components = Array.from(document.querySelectorAll('*'))
      .map(element => element.__vue__)
      .filter(Boolean);
    const find = name => components.find(component => component.$options?.name === name);
    const management = find('TalentManagement');
    const menu = find('EhMenuPositions');
    const status = find('EhJobStatus');
    if (!management || !menu) return null;
    return {
      hruid: management.$data?.hruid,
      coid: management.$data?.jobParams?.coid,
      positions: menu.$props?.positionListsAll || [],
      activeJobId: menu.$props?.activeJobid,
      statuses: status?.$data?.resumeTabList || status?.$props?.resumeTabList || [],
      items: management.$data?.items || [],
      total: management.$data?.total,
      pagination: management.$data?.pagination
    };
  });
}

async function openBackgroundManagementPage(browser, readyTimeout = 12000) {
  let existing = browser.targets().find(target => target.type() === 'page' &&
    target.url().startsWith('https://ehire.51job.com/Revision/talent/management'));
  if (!existing) existing = await findManagementPageViaMenu(browser);
  if (!existing) throw new Error('未找到已打开的人才管理页面。请从 51job 企业首页菜单进入「人才管理」后重试');
  const session = await browser.target().createCDPSession();
  let page;
  try {
    const { targetId } = await session.send('Target.createTarget', {
      url: existing.url(), background: true
    });
    const target = await browser.waitForTarget(item => item._targetId === targetId, { timeout: 10000 });
    page = await target.page();
  } finally {
    await session.detach().catch(() => {});
  }
  if (!page) throw new Error('51job 后台读取页未就绪');

  try {
    await page.waitForFunction(() => {
      const u = window.location.href.toLowerCase();
      if (u === 'about:blank' || u === '') {
        return false;
      }
      if (u.includes('login') || u.includes('navigate.aspx') || u.includes('passport') ||
          (u.includes('default.aspx') && !u.includes('/talent/'))) {
        return 'LOGIN_REDIRECT';
      }
      const components = Array.from(document.querySelectorAll('*')).map(element => element.__vue__).filter(Boolean);
      const management = components.find(vm => vm.$options?.name === 'TalentManagement');
      const menu = components.find(vm => vm.$options?.name === 'EhMenuPositions');
      if (Boolean(management?.$data?.hruid && management?.$data?.jobParams?.coid &&
        menu?.$props?.positionListsAll?.length)) {
        return 'READY';
      }
      if (document.readyState === 'complete' && components.length > 20 && (!management || !menu)) {
        return 'COMPONENT_CHANGED';
      }
      return false;
    // Background tabs may not schedule animation frames; use interval polling.
    }, { timeout: readyTimeout, polling: 250 }).then(async (handle) => {
      const val = await handle.jsonValue().catch(() => null);
      if (val === 'LOGIN_REDIRECT') {
        throw new Error('51job 尚未登录或会话已过期，请先点击【打开 51job 浏览器】登录企业账号');
      }
      if (val === 'COMPONENT_CHANGED') {
        throw new Error('51job 页面结构可能已改变（未找到 TalentManagement 或 EhMenuPositions 组件）');
      }
    });
    return page;
  } catch (error) {
    const currentURL = page.url();
    await page.close().catch(() => {});
    if (currentURL.toLowerCase().includes('error.aspx')) {
      throw new Error('51job 人才管理地址已失效。请从企业首页菜单重新进入「人才管理」，软件不会继续打开失效链接');
    }
    if (currentURL.toLowerCase().includes('login') || currentURL.toLowerCase().includes('navigate.aspx') || currentURL.toLowerCase().includes('passport')) {
      throw new Error('51job 尚未登录或会话已过期，请先点击【打开 51job 浏览器】登录企业账号');
    }
    if (String(error?.message).includes('尚未登录') || String(error?.message).includes('页面结构可能已改变')) {
      throw error;
    }
    throw new Error(`51job 人才管理后台页加载超时（超过${Math.ceil(readyTimeout / 1000)}秒），稍后自动补读`);
  }
}

async function scanJobStatus(browser, accountScope, jobId, statusIndex, pageIndex = 1) {
  if (![1, 2, 3, 4].includes(statusIndex)) throw new Error('仅支持读取待处理、合适、面试、待定');
  if (!Number.isInteger(pageIndex) || pageIndex < 1) throw new Error('页码无效');
  const page = await openBackgroundManagementPage(browser);
  try {
    const initial = await readState(page);
    const catalog = normalizeCatalog(initial);
    if (catalog.accountScope !== accountScope) throw new Error('51job 账号在读取期间发生变化');
    if (!catalog.jobs.some(job => job.jobId === jobId)) throw new Error('平台职位 ID 不属于当前账号');
    const outcome = await page.evaluate(async ({ selectedJobId, selectedStatus, selectedPage }) => {
      const components = Array.from(document.querySelectorAll('*'))
        .map(element => element.__vue__).filter(Boolean);
      const management = components.find(vm => vm.$options?.name === 'TalentManagement');
      const menu = components.find(vm => vm.$options?.name === 'EhMenuPositions');
      const status = components.find(vm => vm.$options?.name === 'EhJobStatus');
      if (!management || !menu || !status) throw new Error('51job 人才管理组件未就绪');
      const job = (menu.$props?.positionListsAll || []).find(item => String(item.jobid) === selectedJobId);
      if (!job) throw new Error('平台职位未找到');
      management.getJobParams({ ...job, activeIndex: (menu.$props.positionListsAll || []).indexOf(job) });
      status.changeStatus(selectedStatus);
      await new Promise(resolve => management.$nextTick(resolve));
      management.$data.pagination.page_index = selectedPage;
      management.updateList();
      const deadline = Date.now() + 8000;
      while (Date.now() < deadline) {
        if (!management.$data.loading) {
          const items = management.$data.items || [];
          if (items.length === 0 || String(items[0]?.jobid || items[0]?.job_id || '') === selectedJobId) {
            break;
          }
        }
        await new Promise(resolve => setTimeout(resolve, 150));
      }
      return {
        jobId: String(management.$data.jobParams?.jobid || ''),
        stage: Number(management.getParams()?.resume_process_state),
        total: Number(management.$data.total || 0),
        items: management.$data.items || [],
        pageSize: Number(management.$data.pagination?.page_size || 0),
        pageIndex: Number(management.$data.pagination?.page_index || 0)
      };
    }, { selectedJobId: jobId, selectedStatus: statusIndex, selectedPage: pageIndex });
    if (outcome.jobId !== jobId || outcome.stage !== statusIndex || outcome.pageIndex !== pageIndex) {
      throw new Error('平台职位或处理状态回执不一致，已停止读取');
    }
    const candidates = outcome.items.map(item => normalizeCandidate(item, jobId));
    if (candidates.some(item => item === null)) throw new Error('候选人身份或职位无法逐条核验，已停止读取');
    if (outcome.pageSize < 1 || candidates.length > outcome.pageSize) throw new Error('平台分页回执异常');
    const hasMore = pageIndex * outcome.pageSize < outcome.total;
    return { status: 'ok', accountScope, jobId, statusIndex, pageIndex,
      pageSize: outcome.pageSize, total: outcome.total, hasMore, candidates };
  } finally {
    await page.close().catch(() => {});
  }
}

// A detail is opened through background CDP navigation, avoiding window.open
// which pulls the browser window to the foreground. Intercepts /resumedtl/getresume.
async function readCandidateDetail(browser, accountScope, options) {
  const { jobId, candidateId, recordId } = options;
  const statusIndex = Number(options.statusIndex);
  const pageIndex = Number(options.pageIndex);
  if (![1, 2, 3, 4].includes(statusIndex) || !Number.isInteger(pageIndex) || pageIndex < 1 ||
      !jobId || !candidateId || !recordId) throw new Error('读取简历所需的平台标识不完整');
  const page = await openBackgroundManagementPage(browser, 8000);
  let detailPage;
  let detailTargetId;
  try {
    const catalog = normalizeCatalog(await readState(page));
    if (catalog.accountScope !== accountScope || !catalog.jobs.some(job => job.jobId === jobId)) {
      throw new Error('51job 账号或职位已变化');
    }
    const selected = await page.evaluate(async ({ jobId, statusIndex, pageIndex, candidateId, recordId }) => {
      const components = Array.from(document.querySelectorAll('*')).map(el => el.__vue__).filter(Boolean);
      const management = components.find(vm => vm.$options?.name === 'TalentManagement');
      const menu = components.find(vm => vm.$options?.name === 'EhMenuPositions');
      const status = components.find(vm => vm.$options?.name === 'EhJobStatus');
      if (!management || !menu || !status) throw new Error('人才管理页面尚未就绪');
      const job = (menu.$props.positionListsAll || []).find(item => String(item.jobid) === jobId);
      if (!job) throw new Error('平台职位不存在');
      management.getJobParams({ ...job, activeIndex: menu.$props.positionListsAll.indexOf(job) });
      status.changeStatus(statusIndex);
      await new Promise(resolve => management.$nextTick(resolve));
      management.$data.pagination.page_index = pageIndex;
      management.updateList();
      const deadline = Date.now() + 8000;
      while (Date.now() < deadline) {
        if (!management.$data.loading) {
          const items = management.$data.items || [];
          if (items.length === 0 || String(items[0]?.jobid || items[0]?.job_id || '') === jobId) {
            break;
          }
        }
        await new Promise(resolve => setTimeout(resolve, 150));
      }
      const item = (management.$data.items || []).find(value => String(value.userid) === candidateId &&
        String(value.cvlogid) === recordId && String(value.jobid) === jobId);
      if (!item || String(management.$data.jobParams.jobid) !== jobId ||
          Number(management.getParams().resume_process_state) !== statusIndex) throw new Error('候选人身份或处理状态已变化');
      const card = Array.from(document.querySelectorAll('*')).map(el => el.__vue__)
        .find(vm => vm?.$options?.name === 'EhResumeCard' &&
          String(vm.$props?.source?.userid) === candidateId && String(vm.$props?.source?.cvlogid) === recordId);
      if (!card) throw new Error('无法定位候选人的简历卡片');

      // 平台先异步校验权限，回调后才打开详情。拦截必须持续到回调完成。
      let detailUrl = '';
      window.open = function(url) {
        detailUrl = String(url || '');
        return { focus: () => {}, close: () => {} };
      };
      if (typeof card.toResumeDetail !== 'function') throw new Error('平台简历详情入口不可用');
      card.toResumeDetail(candidateId, jobId);
      for (let attempt = 0; attempt < 20 && !detailUrl; attempt++) {
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      if (!detailUrl) throw new Error('平台未返回真实简历详情地址，已留待核实，不使用猜测链接');

      return {
        name: String(item.username || item.base_info?.resume_name || item.base_info?.name || '').trim(),
        hasAttachment: String(item.resume_has_enclosure || '') === '1',
        detailUrl
      };
    }, { jobId, statusIndex, pageIndex, candidateId, recordId });

    // 只使用平台真实提供的地址，并先建立监听再后台导航，避免丢失详情响应。
    const fullUrl = new URL(selected.detailUrl, 'https://ehire.51job.com');
    if (fullUrl.protocol !== 'https:' || fullUrl.hostname !== 'ehire.51job.com' ||
        fullUrl.pathname.toLowerCase().endsWith('/error.aspx')) {
      throw new Error('平台返回的简历详情地址无效，已留待核实');
    }
    const session = await browser.target().createCDPSession();
    try {
      const created = await session.send('Target.createTarget', { url: 'about:blank', background: true });
      detailTargetId = created.targetId;
    } finally {
      await session.detach().catch(() => {});
    }
    const detailTarget = await browser.waitForTarget(target => target._targetId === detailTargetId, { timeout: 2500 });
    detailPage = await detailTarget.page();
    if (!detailPage) throw new Error('平台简历详情后台页未就绪');
    const responsePromise = new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('简历详情回执超时，已留待核实')), 14000);
      detailPage.on('response', async response => {
        if (!response.url().includes('/resumedtl/getresume') || response.request().method() === 'OPTIONS') return;
        try {
          const payload = await response.json();
          clearTimeout(timer);
          resolve(payload);
        } catch { /* Duplicate or cancelled response. */ }
      });
    });
    void detailPage.goto(fullUrl.toString(), { waitUntil: 'domcontentloaded', timeout: 14000 }).catch(() => {});
    const response = await responsePromise;
    const data = response?.data;
    if (!data || String(data.resumeid) !== candidateId || String(data.cvlogid) !== recordId ||
        String(data.jobid) !== jobId || `co:${data.coid}:hr:${(await readState(page)).hruid}` !== accountScope) {
      throw new Error('简历详情回执身份不一致，已停止导入');
    }
    const candidateName = selected.name || String(data.username || data.base_info?.resume_name || data.base_info?.name || '').trim();
    const hasAttachment = selected.hasAttachment || String(data.hasenclosure || '') === '1' ||
      String(data.hasattachpdf || '') === '1' || (Array.isArray(data.attachment) && data.attachment.length > 0);
    const content = JSON.stringify(data);
    if (content.length < 400) throw new Error('简历正文不完整');
    return { status: 'ok', accountScope, jobId, candidateId, recordId, name: candidateName,
      resumeText: content, hasAttachment, documentsComplete: true,
      documentStatus: hasAttachment ? '在线结构化简历完整（附带平台附件）' : '在线结构化简历完整' };
  } finally {
    if (detailPage && !detailPage.isClosed()) await detailPage.close().catch(() => {});
    if (detailTargetId) {
      try {
        const s = await browser.target().createCDPSession();
        await s.send('Target.closeTarget', { targetId: detailTargetId });
        await s.detach().catch(() => {});
      } catch (_) {}
    }
    await page.close().catch(() => {});
  }
}

async function rejectCandidate(browser, accountScope, options) {
  const { jobId, candidateId, recordId, reasonLabel } = options;
  if (!jobId || !candidateId || !recordId) {
    throw new Error('标记不合适所需的平台标识不完整');
  }
  const page = await connectManagementPage().then(r => r.page);
  try {
    const catalog = normalizeCatalog(await readState(page));
    if (catalog.accountScope !== accountScope || !catalog.jobs.some(job => job.jobId === jobId)) {
      throw new Error('51job 账号或职位已变化');
    }
    const outcome = await page.evaluate(async ({ jobId, candidateId, recordId, reasonLabel }) => {
      const components = Array.from(document.querySelectorAll('*')).map(el => el.__vue__).filter(Boolean);
      const management = components.find(vm => vm.$options?.name === 'TalentManagement');
      const menu = components.find(vm => vm.$options?.name === 'EhMenuPositions');
      const status = components.find(vm => vm.$options?.name === 'EhJobStatus');
      if (!management || !menu || !status) throw new Error('人才管理页面尚未就绪');

      const job = (menu.$props.positionListsAll || []).find(item => String(item.jobid) === jobId);
      if (!job) throw new Error('平台职位不存在');
      management.getJobParams({ ...job, activeIndex: menu.$props.positionListsAll.indexOf(job) });
      status.changeStatus(1);
      await new Promise(resolve => management.$nextTick(resolve));

      const card = Array.from(document.querySelectorAll('*')).map(el => el.__vue__)
        .find(vm => vm?.$options?.name === 'EhResumeCard' &&
          String(vm.$props?.source?.userid) === candidateId && String(vm.$props?.source?.cvlogid) === recordId);
      if (!card) {
        // 先只读复查是否已在「不合适」(statusIndex = 4)
        try {
          status.changeStatus(4);
          await new Promise(resolve => setTimeout(resolve, 1200));
          const unfitCard = Array.from(document.querySelectorAll('*')).map(el => el.__vue__)
            .find(vm => vm?.$options?.name === 'EhResumeCard' &&
              String(vm.$props?.source?.userid) === candidateId && String(vm.$props?.source?.cvlogid) === recordId);
          status.changeStatus(1);
          if (unfitCard) {
            return {
              alreadyUnfit: true,
              clickedReason: '平台已处于不合适状态',
              cvlogid: recordId,
              userid: candidateId
            };
          }
        } catch (_) {}
        throw new Error('无法在待处理列表中定位该候选人（可能已被处理或转移）');
      }

      const btn = document.getElementById(`unfit_reason_btn${recordId}`);
      if (!btn) throw new Error('无法找到候选人的「不合适」操作按钮');

      btn.click();
      await new Promise(resolve => setTimeout(resolve, 800));

      const popovers = Array.from(document.querySelectorAll('.el-popover, .el-dialog, [role="tooltip"], [class*="popover"], [class*="reason"]'))
        .filter(el => el.getClientRects().length > 0 && /选择原因|优化推荐|其他原因|不匹配/.test(el.innerText || ''));
      if (!popovers.length) throw new Error('点击不合适后未弹出「选择原因」浮层');

      const pop = popovers[popovers.length - 1];
      const items = Array.from(pop.querySelectorAll('button, span, li, div'))
        .filter(el => el.children.length === 0 && el.getClientRects().length > 0);

      const targetText = reasonLabel || '学历不匹配';
      let match = items.find(el => (el.textContent || '').trim() === targetText);
      if (!match) match = items.find(el => (el.textContent || '').trim().includes(targetText));
      if (!match) match = items.find(el => /其他原因/.test(el.textContent || ''));
      if (!match) throw new Error('未能匹配到原因选择项');

      const clickable = match.closest('button, li, [role="button"]') || match;
      clickable.click();
      await new Promise(resolve => setTimeout(resolve, 1200));

      const confirmBtns = Array.from(document.querySelectorAll('.el-dialog__footer button, .el-message-box__btns button'))
        .filter(b => /确定|确认|提交/.test(b.innerText || ''));
      if (confirmBtns.length) {
        confirmBtns[confirmBtns.length - 1].click();
        await new Promise(resolve => setTimeout(resolve, 1000));
      }

      // 逐人真实回执复核：等待列表刷新，验证卡片是否从待处理中移出
      if (typeof management.updateList === 'function') {
        try {
          const p = management.updateList();
          if (p && typeof p.then === 'function') await p;
        } catch (_) {}
      }
      await new Promise(resolve => setTimeout(resolve, 1500));

      const remainingCard = Array.from(document.querySelectorAll('*')).map(el => el.__vue__)
        .find(vm => vm?.$options?.name === 'EhResumeCard' &&
          String(vm.$props?.source?.userid) === candidateId && String(vm.$props?.source?.cvlogid) === recordId);

      return {
        clickedReason: (match.textContent || '').trim(),
        cvlogid: recordId,
        userid: candidateId,
        verified: !remainingCard
      };
    }, { jobId, candidateId, recordId, reasonLabel });

    if (outcome.alreadyUnfit) {
      return {
        status: 'ok',
        verified: true,
        alreadyProcessed: true,
        receipt: `经核验平台已处于不合适状态，已逐人核对确认（保留来源，不重复点击）`,
        accountScope,
        jobId,
        candidateId,
        recordId
      };
    }
    if (!outcome.verified) {
      return {
        status: 'receipt_pending',
        verified: false,
        receipt: `已在平台提交不合适操作，但待处理列表未即时移出该卡片，待核实回执`,
        accountScope,
        jobId,
        candidateId,
        recordId
      };
    }
    return {
      status: 'ok',
      verified: true,
      receipt: `已在 51job 平台成功标记不合适并经回执核验（勾选原因：${outcome.clickedReason}）`,
      accountScope,
      jobId,
      candidateId,
      recordId
    };
  } finally {
    // Keep management page intact
  }
}

async function run(mode, options = {}) {
  const { browser, page, isTempPage } = await connectManagementPage();
  try {
    const state = await readState(page);
    if (!state) throw new Error('51job 人才管理页面尚未加载完成');
    const catalog = normalizeCatalog(state);
    if (mode === 'catalog') return { status: 'ok', ...catalog };
    if (mode === 'current') {
      const jobId = String(state.activeJobId || '');
      if (!catalog.jobs.some(job => job.jobId === jobId)) throw new Error('当前没有选定可核验的职位');
      const candidates = (state.items || []).map(item => normalizeCandidate(item, jobId));
      if (candidates.some(item => item === null)) throw new Error('候选人身份或职位无法逐条核验，已停止读取');
      return { status: 'ok', accountScope: catalog.accountScope, jobId, total: state.total, candidates };
    }
    if (mode === 'scan') {
      return await scanJobStatus(browser, catalog.accountScope, String(options.jobId || ''),
        Number(options.statusIndex || 1), Number(options.pageIndex || 1));
    }
    if (mode === 'detail') {
      return await readCandidateDetail(browser, catalog.accountScope, options);
    }
    if (mode === 'reject') {
      return await rejectCandidate(browser, catalog.accountScope, options);
    }
    throw new Error(`不支持的读取模式: ${mode}`);
  } finally {
    if (isTempPage && page && !page.isClosed()) {
      await page.close().catch(() => {});
    }
    await browser.disconnect();
  }
}

if (require.main === module) {
  const mode = process.argv[process.argv.indexOf('--mode') + 1] || 'catalog';
  const argument = name => {
    const index = process.argv.indexOf(name);
    return index >= 0 ? process.argv[index + 1] : '';
  };
  run(mode, { jobId: argument('--job-id'), statusIndex: argument('--status-index'), pageIndex: argument('--page-index'),
    candidateId: argument('--candidate-id'), recordId: argument('--record-id'),
    reasonLabel: argument('--reason-label'), reasonCode: argument('--reason-code') })
    .then(result => process.stdout.write(JSON.stringify(result) + '\n'))
    .catch(error => {
      process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n');
      process.exitCode = 1;
    });
}

module.exports = { normalizeCatalog, normalizeCandidate, run, scanJobStatus, readCandidateDetail, rejectCandidate };
