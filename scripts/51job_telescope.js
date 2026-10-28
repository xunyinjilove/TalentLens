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
  if (!recommend || !list) return null;

  const job = recommend.$data?.activeJobInfo || {};
  const groups = list.$props?.list || list.$data?.list || [];
  const hruid = String(recommend.$data?.hruId || '').trim();
  const coid = String(job.coid || groups[0]?.coid || list.$props?.jobParams?.coid || '').trim();

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

  const activeJobId = String(job.jobid || menu?.$props?.activeJobid || list.$props?.jobParams?.jobid || '').trim();
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

async function readTelescopeCatalog() {
  const browser = await puppeteer.connect({
    browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 8000
  });
  try {
    const targets = browser.targets().filter(target => target.type() === 'page' &&
      target.url().startsWith('https://ehire.51job.com/Revision/talent/search-recommend'));
    if (!targets.length) {
      throw new Error('未找到已打开的人才望远镜页面，请先在 51job 专用浏览器打开「AI淘金 → 人才望远镜」');
    }
    let lastError = null;
    for (const target of targets) {
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
    const targets = browser.targets().filter(target => target.type() === 'page' &&
      target.url().startsWith('https://ehire.51job.com/Revision/talent/search-recommend'));
    if (!targets.length) throw new Error('未找到已打开的人才望远镜页面，请先在 51job 专用浏览器打开「AI淘金 → 人才望远镜」');
    const visibleJobs = [];
    const readErrors = [];
    for (const target of targets) {
      let state;
      try {
        state = await readTelescopePage(target);
      } catch (error) {
        readErrors.push(error.message);
        continue;
      }
      if (!state?.jobId) continue;
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

if (require.main === module) {
  const modeIndex = process.argv.indexOf('--mode');
  const mode = modeIndex >= 0 ? process.argv[modeIndex + 1] : 'scan';
  if (mode === 'catalog') {
    readTelescopeCatalog()
      .then(result => process.stdout.write(JSON.stringify(result) + '\n'))
      .catch(error => { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); process.exitCode = 1; });
  } else {
    const index = process.argv.indexOf('--job-id');
    const jobId = index >= 0 ? process.argv[index + 1] : '';
    scanTelescope(jobId).then(result => process.stdout.write(JSON.stringify(result) + '\n'))
      .catch(error => { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); process.exitCode = 1; });
  }
}

module.exports = { normalizeTelescopeCandidate, scanTelescope, readTelescopePage, readTelescopeCatalog };
