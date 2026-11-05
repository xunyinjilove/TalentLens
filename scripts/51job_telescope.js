/** Read the currently opened 51job Talent Telescope page without changing it. */
const puppeteer = require('puppeteer-core');

function normalizeTelescopeCandidate(raw, jobId) {
  const candidateId = String(raw?.userid || '').trim();
  // Telescope has no cvlogid; userid is its stable candidate identity.
  const recordId = candidateId;
  if (!candidateId || !recordId) return null;
  const name = String(raw?.base_info?.resume_name || '').trim();
  return {
    candidateId,
    recordId,
    jobId,
    name,
    resumeText: JSON.stringify(raw),
    activityLabel: String(raw?.active_type || '').trim(),
    alreadyHi: [true, 1, '1'].includes(raw?.resume_interactive?.is_hichat) ||
      [true, 1, '1'].includes(raw?.is_hi_chat)
  };
}

function telescopeSnapshot() {
  if (location.pathname !== '/Revision/talent/search-recommend') return null;
  const components = Array.from(document.querySelectorAll('*')).map(el => el.__vue__).filter(Boolean);
  const recommend = components.find(vm => vm.$options?.name === 'TalentRecommend');
  const menu = components.find(vm => vm.$options?.name === 'EhMenuPositionsRecommend');
  const list = components.find(vm => vm.$options?.name === 'EhFossicksList');
  // 职位标签可先于候选人列表加载，目录读取不应依赖列表组件。
  if (!recommend) return null;

  const job = recommend.$data?.activeJobInfo || {};
  const groups = list?.$props?.list || list?.$data?.list || [];
  const hruid = String(recommend.$data?.hruId || '').trim();
  const coid = String(job.coid || groups[0]?.coid || list?.$props?.jobParams?.coid || '').trim();

  const rawPositions = menu?.$props?.positionListsAll || menu?.$data?.positionListsAll || [];
  const seen = new Set();
  const jobs = [];
  for (const p of rawPositions) {
    const pid = String(p.jobid || '').trim();
    if (!pid || seen.has(pid)) continue;
    seen.add(pid);
    jobs.push({
      jobId: pid,
      title: String(p.jobname || '').trim(),
      workArea: String(p.workArea || p.jobArea || '').trim(),
      degree: String(p.degree || '').trim(),
      workYear: String(p.workYear || '').trim(),
      salary: String(p.salary || p.newSalary || '').trim(),
      status: String(p.statusStr || p.publishStatus || '').trim()
    });
  }

  const activeJobId = String(job.jobid || menu?.$props?.activeJobid || list?.$props?.jobParams?.jobid || '').trim();
  const activeJobTitle = String(job.jobname || '').trim();
  if (activeJobId && !seen.has(activeJobId)) {
    jobs.unshift({
      jobId: activeJobId,
      title: activeJobTitle,
      workArea: String(job.workArea || '').trim(),
      degree: String(job.degree || '').trim(),
      workYear: String(job.workYear || '').trim(),
      salary: String(job.salary || '').trim(),
      status: String(job.publishStatus || '').trim()
    });
  }

  return {
    jobId: activeJobId,
    jobTitle: activeJobTitle,
    companyId: coid,
    hruid,
    accountScope: (coid && hruid) ? `co:${coid}:hr:${hruid}` : '',
    jobs,
    listLoaded: Boolean(list),
    candidates: groups.flatMap(group => group.dataList || []).map(item => JSON.parse(JSON.stringify(item)))
  };
}

async function readTelescopePage(target) {
  // target.page() enables Network on a large, sleeping telescope tab and can
  // time out even while its Vue data is available. Runtime.evaluate is enough.
  const session = await target.createCDPSession();
  try {
    const response = await session.send('Runtime.evaluate', {
      expression: `JSON.stringify((${telescopeSnapshot.toString()})())`,
      returnByValue: true
    });
    if (response.exceptionDetails) throw new Error('人才望远镜页面数据读取失败');
    return JSON.parse(response.result?.value || 'null');
  } finally {
    await session.detach().catch(() => {});
  }
}

async function reloadSkeletonPage(target) {
  const session = await target.createCDPSession();
  try { await session.send('Page.enable'); await session.send('Page.reload', { ignoreCache: false }); }
  finally { await session.detach().catch(() => {}); }
}

async function selectTelescopeJob(target, jobId) {
  const current = await readTelescopePage(target);
  if (current?.jobId === jobId) return current;
  if (!current?.jobs?.some(job => job.jobId === jobId)) return current;
  const page = await target.page();
  const selected = await page.evaluate(id => {
    const menu = [...document.querySelectorAll('*')].map(el => el.__vue__)
      .find(vm => vm?.$options?.name === 'EhMenuPositionsRecommend');
    const jobs = menu?.$props?.positionListsAll || [];
    const index = jobs.findIndex(job => String(job.jobid) === id);
    const cards = menu?.$el?.querySelectorAll('.menu-item');
    if (index < 0 || !cards || cards.length !== jobs.length) return false;
    const card = cards[index];
    if (card.querySelector('[title]')?.title !== String(jobs[index].jobname || '')) return false;
    const button = card.querySelector('.menu-item_content');
    if (!button) return false;
    button.click();
    return true;
  }, jobId);
  if (!selected) throw new Error('无法从人才望远镜职位标签精确定位所选职位');
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    const state = await readTelescopePage(target).catch(() => null);
    if (state?.jobId === jobId && state.listLoaded) return state;
    await new Promise(resolve => setTimeout(resolve, 300));
  }
  throw new Error('已切换人才望远镜职位，但候选人列表未在 15 秒内就绪');
}

async function findTelescopeTargets(browser) {
  const matching = () => browser.targets().filter(target => target.type() === 'page' &&
    target.url().startsWith('https://ehire.51job.com/Revision/talent/search-recommend'));
  if (matching().length) return matching();
  const siteTargets = browser.targets().filter(target => target.type() === 'page' &&
    target.url().startsWith('https://ehire.51job.com/') && !target.url().toLowerCase().includes('/error.aspx'));
  for (const target of siteTargets) {
    const page = await target.page().catch(() => null);
    if (!page) continue;
    const hasMenu = await page.evaluate(() => Boolean(document.querySelector('#sensor_recommand_menu'))).catch(() => false);
    if (!hasMenu) continue;
    await page.evaluate(() => document.querySelector('#sensor_recommand_menu').click());
    const deadline = Date.now() + 8000;
    while (Date.now() < deadline) {
      if (matching().length) return matching();
      await new Promise(resolve => setTimeout(resolve, 200));
    }
    throw new Error('已点击 51job「人才望远镜」菜单，但页面未在 8 秒内打开；请检查登录状态');
  }
  throw new Error('未找到已登录且带「人才望远镜」菜单的 51job 页面，请先在专用浏览器登录企业账号');
}

async function readTelescopeCatalog() {
  const browser = await puppeteer.connect({
    browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 8000
  });
  try {
    const targets = await findTelescopeTargets(browser);
    if (!targets.length) {
      throw new Error('未找到已打开的人才望远镜页面，请先在 51job 专用浏览器打开「AI淘金 → 人才望远镜」');
    }
    let lastError = null;
    for (const target of targets) {
      for (let attempt = 0; attempt < 20; attempt++) {
        try {
          const state = await readTelescopePage(target);
          if (state && (state.jobs?.length > 0 || state.jobId)) {
            return {
              status: 'ok',
              accountScope: state.accountScope,
              activeJobId: state.jobId,
              activeJobTitle: state.jobTitle,
              companyId: state.companyId,
              jobs: state.jobs
            };
          }
        } catch (err) {
          lastError = err;
        }
        await new Promise(resolve => setTimeout(resolve, 400));
      }
    }
    // An old telescope tab can remain on a Vue loading skeleton after login
    // expires. Refresh that tab once without activating its window.
    for (const target of targets) {
      await reloadSkeletonPage(target).catch(() => {});
      for (let attempt = 0; attempt < 20; attempt++) {
        await new Promise(resolve => setTimeout(resolve, 400));
        const state = await readTelescopePage(target).catch(() => null);
        if (state && (state.jobs?.length > 0 || state.jobId)) {
          return { status: 'ok', accountScope: state.accountScope, activeJobId: state.jobId,
            activeJobTitle: state.jobTitle, companyId: state.companyId, jobs: state.jobs };
        }
      }
    }
    throw lastError || new Error('未能从人才望远镜页面解析出职位列表');
  } finally {
    await browser.disconnect();
  }
}

async function scanTelescope(jobId, limit = 100) {
  if (!jobId) throw new Error('请先选择 51job 职位');
  const browser = await puppeteer.connect({
    browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 8000
  });
  try {
    const targets = await findTelescopeTargets(browser);
    if (!targets.length) throw new Error('未找到已打开的人才望远镜页面，请先在 51job 专用浏览器打开「AI淘金 → 人才望远镜」');
    const visibleJobs = [];
    const readErrors = [];
    for (const target of targets) {
      let state;
      for (let attempt = 0; attempt < 20; attempt++) {
        try {
          state = await readTelescopePage(target);
          if (state?.jobId && state.listLoaded) break;
        } catch (error) {
          readErrors.push(error.message);
        }
        await new Promise(resolve => setTimeout(resolve, 400));
      }
      if (state?.jobId !== jobId && state?.jobs?.some(job => job.jobId === jobId)) {
        state = await selectTelescopeJob(target, jobId);
      }
      if (!state?.jobId || !state.listLoaded) continue;
      visibleJobs.push(state.jobTitle ? `${state.jobTitle} (${state.jobId})` : state.jobId);
      if (state.jobId !== jobId) continue;
      if (!state.companyId) throw new Error('人才望远镜未返回可核验的企业 ID');
      const seen = new Set();
      const candidates = [];
      for (const [index, raw] of state.candidates.entries()) {
        const candidate = normalizeTelescopeCandidate(raw, jobId);
        if (!candidate) throw new Error(`人才望远镜第 ${index + 1} 条候选人缺少稳定的用户 ID，已停止扫描`);
        const key = `${candidate.candidateId}:${candidate.recordId}`;
        if (seen.has(key)) continue;
        seen.add(key);
        candidates.push(candidate);
        if (candidates.length >= limit) break;
      }
      return {
        status: 'ok', source: 'telescope', jobId, jobTitle: state.jobTitle,
        companyId: state.companyId, accountScope: state.accountScope,
        total: state.candidates.length, candidates
      };
    }
    if (!visibleJobs.length && readErrors.length) {
      throw new Error('人才望远镜标签页没有响应。请在 51job 浏览器切换到「人才望远镜」，等待候选人列表出现后重试；若仍失败，请刷新该标签页。');
    }
    throw new Error(`人才望远镜当前展示的职位与所选职位不一致。请在 51job 页面切换到对应职位后重试。当前页面：${visibleJobs.join('、') || '未加载职位'}`);
  } finally {
    await browser.disconnect();
  }
}

async function readTelescopeDetail(jobId, candidateId) {
  if (!jobId || !candidateId) throw new Error('缺少职位或人才 ID');
  const browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 });
  let browserSession = null;
  let backgroundTargetId = null;
  let openedTarget = null;
  try {
    const targets = await findTelescopeTargets(browser);
    for (const target of targets) {
      let state = await readTelescopePage(target).catch(() => null);
      if (state?.jobId !== jobId && state?.jobs?.some(job => job.jobId === jobId)) state = await selectTelescopeJob(target, jobId);
      if (!state || state.jobId !== jobId) continue;
      const matches = state.candidates.filter(raw => String(raw.userid || '') === candidateId);
      if (matches.length !== 1) throw new Error('望远镜候选人身份无法唯一核对');
      const page = await target.page();
      const platformURL = await page.evaluate(async id => {
        const list = [...document.querySelectorAll('*')].map(el => el.__vue__).find(vm => vm?.$options?.name === 'EhFossicksList');
        const rows = (list?.list || []).flatMap(group => [...(group.matchList || []), ...(group.notMatchList || [])]);
        const index = rows.findIndex(raw => String(raw.userid || '') === id);
        if (index < 0 || !list?.resumeViewHandler) return '';
        // Let 51job authorize and construct the detail URL, but capture its window.open
        // so screening does not switch the user's browser to a visible detail tab.
        if (window.__talentLensDetailCapture) throw new Error('另一项详情读取仍在进行');
        const originalOpen = window.open;
        window.__talentLensDetailCapture = true;
        try {
          return await new Promise((resolve, reject) => {
            const timer = setTimeout(() => reject(new Error('平台未返回详情入口')), 9000);
            window.open = function(url, ...args) {
              const openedURL = url ? new URL(String(url), location.href) : null;
              if (openedURL?.hostname === 'ehire.51job.com' &&
                  openedURL.pathname === '/Revision/talent/resume/detail' &&
                  openedURL.searchParams.get('resumeId') === id) {
                clearTimeout(timer);
                resolve(openedURL.href);
                return null;
              }
              return originalOpen.call(this, url, ...args);
            };
            try { list.resumeViewHandler([index, id, '', rows[index]], 0); }
            catch (error) { clearTimeout(timer); reject(error); }
          });
        } finally {
          window.open = originalOpen;
          delete window.__talentLensDetailCapture;
        }
      }, candidateId);
      if (!platformURL) throw new Error('平台未提供该人才的详情入口');
      const detailURL = validateTelescopeDetailURL(platformURL, page.url(), jobId, candidateId);
      const before = new Set(browser.targets());
      browserSession = await page.createCDPSession();
      const created = await browserSession.send('Target.createTarget', { url: detailURL, background: true });
      backgroundTargetId = created.targetId;
      const deadline = Date.now() + 18000;
      while (Date.now() < deadline) {
        openedTarget = browser.targets().find(t => !before.has(t) && t.type() === 'page' &&
          t.url().includes('/Revision/talent/resume/detail') && new URL(t.url()).searchParams.get('resumeId') === candidateId);
        if (openedTarget) break;
        await new Promise(resolve => setTimeout(resolve, 250));
      }
      if (!openedTarget) throw new Error('后台详情页未加载');
      const detailPage = await openedTarget.page();
      while (Date.now() < deadline) {
        const detail = await detailPage.evaluate(() => {
          const vm = [...document.querySelectorAll('*')].map(el => el.__vue__).find(x => x?.$options?.name === 'TalentResumeDetailUpgrades');
          const data = vm?.$data?.resumeDetail;
          const query = new URLSearchParams(location.search);
          if (!data || String(data.resumeid || '') !== query.get('resumeId') || String(data.jobid || '') !== query.get('jobId')) return null;
          return { candidateId: String(data.resumeid), jobId: String(data.jobid), name: String(data.username || ''),
            resumeText: JSON.stringify(data), profileUrl: location.href };
        }).catch(() => null);
        if (detail && detail.candidateId === candidateId && detail.jobId === jobId && detail.resumeText.length > 100) {
          return { status: 'ok', ...detail };
        }
        await new Promise(resolve => setTimeout(resolve, 300));
      }
      throw new Error('平台详情页未返回可核验的简历资料');
    }
    throw new Error('当前没有与所选职位一致的望远镜页面');
  } finally {
    let closed = false;
    if (backgroundTargetId && browserSession) {
      closed = await browserSession.send('Target.closeTarget', { targetId: backgroundTargetId })
        .then(result => Boolean(result.success)).catch(() => false);
    }
    if (!closed && openedTarget) { const page = await openedTarget.page().catch(() => null); if (page) await page.close().catch(() => {}); }
    if (browserSession) await browserSession.detach().catch(() => {});
    await browser.disconnect();
  }
}

function validateTelescopeDetailURL(rawURL, baseURL, jobId, candidateId) {
  const url = new URL(rawURL, baseURL);
  if (url.protocol !== 'https:' || url.hostname !== 'ehire.51job.com' ||
      url.pathname !== '/Revision/talent/resume/detail' ||
      url.searchParams.get('jobId') !== jobId || url.searchParams.get('resumeId') !== candidateId) {
    throw new Error('平台返回的详情地址与职位或人才不一致，未打开页面');
  }
  return url.href;
}

async function rejectTelescopeCandidate(jobId, candidateId, accountScope, reasonLabel) {
  if (!jobId || !candidateId || !accountScope) throw new Error('缺少可核对的职位、人才或账号 ID');
  const browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 10000 });
  try {
    const targets = await findTelescopeTargets(browser);
    for (const target of targets) {
      let state = await readTelescopePage(target).catch(() => null);
      if (state?.jobId !== jobId && state?.jobs?.some(job => job.jobId === jobId)) {
        state = await selectTelescopeJob(target, jobId);
      }
      if (!state || state.jobId !== jobId || state.accountScope !== accountScope) continue;
      if (!state.candidates.some(raw => String(raw.userid || '') === candidateId)) {
        throw new Error('目标人才已不在当前望远镜名单，未重复点击');
      }
      const page = await target.page();
      const outcome = await page.evaluate(async ({ candidateId, reasonLabel }) => {
        const visible = el => !!el && el.getClientRects().length > 0;
        const components = [...document.querySelectorAll('*')].map(el => el.__vue__).filter(Boolean);
        const list = components.find(vm => vm.$options?.name === 'EhFossicksList');
        const hiddenMismatch = (list?.$props?.list || []).some(group =>
          (group.notMatchList || []).some(raw => String(raw.userid || '') === candidateId) && group.notMatchListCollapse);
        if (hiddenMismatch) {
          const toggles = [...list.$el.querySelectorAll('.not-match .collapse-btn')]
            .filter(el => visible(el) && /展开/.test(el.innerText || ''));
          if (toggles.length !== 1) throw new Error('无法展开平台未推荐人才列表，未点击平台');
          toggles[0].click();
          await new Promise(resolve => setTimeout(resolve, 250));
        }
        const cards = [...document.querySelectorAll('*')].map(el => el.__vue__).filter(vm => vm?.$options?.name === 'EhFossicksResumeCard' &&
          String(vm.$props?.source?.userid || '') === candidateId && visible(vm.$el));
        if (cards.length !== 1) {
          throw new Error('无法唯一定位该人才的望远镜卡片，未点击平台');
        }
        const button = document.getElementById(`no_interested_${candidateId}`);
        if (!visible(button) || !cards[0].$el.contains(button) || !button.classList.contains('no_interested_div')) {
          throw new Error('无法核对该人才的不感兴趣按钮，未点击平台');
        }
        button.click();
        await new Promise(resolve => setTimeout(resolve, 500));
        const dialogs = [...document.querySelectorAll('.el-popover,.el-dialog,[role="dialog"]')]
          .filter(el => visible(el) && /选择不感兴趣原因|其他原因/.test(el.innerText || ''));
        if (!dialogs.length) throw new Error('平台未显示不感兴趣原因，回执待核查');
        const dialog = dialogs[dialogs.length - 1];
        const options = [...dialog.querySelectorAll('button,li,[role="button"],span,div')]
          .filter(el => visible(el) && el.children.length === 0);
        let option = options.find(el => (el.textContent || '').trim() === reasonLabel);
        if (!option) option = options.find(el => (el.textContent || '').trim() === '其他原因');
        if (!option) throw new Error('平台未显示可选原因，回执待核查');
        const other = /其他原因/.test(option.textContent || '');
        (option.closest('button,li,[role="button"]') || option).click();
        await new Promise(resolve => setTimeout(resolve, 350));
        if (other) {
          const input = [...document.querySelectorAll('textarea,input[type="text"]')].find(el => visible(el) && !el.disabled &&
            /原因|说明|补充|其他/.test(el.placeholder || el.closest('.el-dialog,.el-popover')?.innerText || ''));
          if (!input) throw new Error('已选其他原因，但没有说明输入框，回执待核查');
          const setter = Object.getOwnPropertyDescriptor(input.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype, 'value')?.set;
          if (!setter) throw new Error('无法填写其他原因，回执待核查');
          setter.call(input, '不匹配');
          input.dispatchEvent(new Event('input', { bubbles: true }));
        }
        const submit = [...document.querySelectorAll('.el-dialog__footer button,.el-popover button')]
          .filter(el => visible(el) && /确定|确认|提交/.test(el.innerText || ''));
        if (submit.length === 1) submit[0].click();
        return { selectedReason: other ? '其他原因：不匹配' : reasonLabel };
      }, { candidateId, reasonLabel });
      let verified = false;
      for (let i = 0; i < 8; i++) {
        await new Promise(resolve => setTimeout(resolve, 400));
        const after = await readTelescopePage(target).catch(() => null);
        if (after && after.jobId === jobId && after.accountScope === accountScope &&
            !after.candidates.some(raw => String(raw.userid || '') === candidateId)) { verified = true; break; }
      }
      return { status: verified ? 'ok' : 'receipt_pending', verified, candidateId, jobId, accountScope,
        receipt: verified ? `平台已移除该人才（${outcome.selectedReason}）` : '已点击不感兴趣，但候选人仍在列表；仅核查回执，不重复点击' };
    }
    throw new Error('当前没有与所选账号及职位一致的望远镜页面');
  } finally { await browser.disconnect(); }
}

if (require.main === module) {
  const modeIndex = process.argv.indexOf('--mode');
  const mode = modeIndex >= 0 ? process.argv[modeIndex + 1] : 'scan';
  if (mode === 'catalog') {
    readTelescopeCatalog()
      .then(result => process.stdout.write(JSON.stringify(result) + '\n'))
      .catch(error => { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); process.exitCode = 1; });
  } else if (mode === 'detail') {
    const argument = name => { const i = process.argv.indexOf(name); return i >= 0 ? process.argv[i + 1] : ''; };
    readTelescopeDetail(argument('--job-id'), argument('--candidate-id'))
      .then(result => process.stdout.write(JSON.stringify(result) + '\n'))
      .catch(error => { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); process.exitCode = 1; });
  } else if (mode === 'reject') {
    const argument = name => { const i = process.argv.indexOf(name); return i >= 0 ? process.argv[i + 1] : ''; };
    rejectTelescopeCandidate(argument('--job-id'), argument('--candidate-id'), argument('--account-scope'), argument('--reason-label'))
      .then(result => process.stdout.write(JSON.stringify(result) + '\n'))
      .catch(error => { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); process.exitCode = 1; });
  } else {
    const index = process.argv.indexOf('--job-id');
    const jobId = index >= 0 ? process.argv[index + 1] : '';
    scanTelescope(jobId).then(result => process.stdout.write(JSON.stringify(result) + '\n'))
      .catch(error => { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); process.exitCode = 1; });
  }
}

module.exports = { normalizeTelescopeCandidate, scanTelescope, readTelescopeDetail, readTelescopePage, readTelescopeCatalog, rejectTelescopeCandidate, validateTelescopeDetailURL };
