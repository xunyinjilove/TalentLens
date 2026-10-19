/** 51job candidate-specific chat and inbound attachment bridge. Never infer success from a click. */
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const puppeteer = require('puppeteer-core');

function parseCandidateUrl(raw) {
  const url = new URL(raw);
  const resumeId = url.searchParams.get('resumeId');
  if (url.protocol !== 'https:' || url.hostname !== 'ehire.51job.com' ||
      url.pathname !== '/Revision/talent/resume/detail' || !/^\d+$/.test(resumeId || '')) {
    throw new Error('不是可核对的 51job 候选人详情地址');
  }
  return { url: url.href, resumeId };
}

function sameCandidatePage(pageUrl, expected) {
  try { return parseCandidateUrl(pageUrl).resumeId === expected.resumeId; } catch { return false; }
}

function attachmentFingerprint(link) {
  const parsed = new URL(link.href);
  return crypto.createHash('sha256').update(parsed.origin + parsed.pathname).digest('hex');
}

function selectInboundAttachment(messages, known = []) {
  const excluded = new Set(known);
  for (let i = messages.length - 1; i >= 0; i--) {
    const message = messages[i];
    if (message.mine) continue;
    for (const link of message.links || []) {
      let parsed;
      try { parsed = new URL(link.href); } catch { continue; }
      if (parsed.protocol !== 'https:' || !(parsed.hostname === '51job.com' || parsed.hostname.endsWith('.51job.com'))) continue;
      const hintedName = String(link.download || link.text || path.basename(parsed.pathname)).trim();
      const match = hintedName.match(/[^\\/\s<>:"|?*]+\.(pdf|docx|doc|txt)\b/i) ||
        decodeURIComponent(parsed.pathname).match(/[^\\/\s<>:"|?*]+\.(pdf|docx|doc|txt)\b/i);
      if (match && !excluded.has(attachmentFingerprint(link))) {
        return { href: parsed.href, fileName: match[0], messageIndex: i };
      }
    }
  }
  return null;
}

function validResumeBytes(buffer, extension) {
  if (buffer.length < 40 || buffer.length > 15 * 1024 * 1024) return false;
  if (extension === 'pdf') return buffer.subarray(0, 5).toString() === '%PDF-';
  if (extension === 'docx') return buffer.subarray(0, 2).toString() === 'PK';
  if (extension === 'doc') return buffer.subarray(0, 8).equals(Buffer.from('d0cf11e0a1b11ae1', 'hex'));
  if (extension === 'txt') return !buffer.includes(0) && !/^\s*</.test(buffer.toString('utf8', 0, 80));
  return false;
}

async function candidatePage(browser, expected, name) {
  let pages = (await browser.pages()).filter(page => sameCandidatePage(page.url(), expected));
  if (!pages.length) {
    const source = (await browser.pages()).find(page => page.url().startsWith('https://ehire.51job.com/'));
    if (!source) {
      const error = new Error('51job 专用浏览器尚未登录或未打开');
      error.code = '51JOB_BROWSER_UNAVAILABLE';
      throw error;
    }
    const session = await source.target().createCDPSession();
    try {
      const { targetId } = await session.send('Target.createTarget', { url: expected.url, background: true });
      const target = await browser.waitForTarget(item => item._targetId === targetId, { timeout: 10000 });
      const page = await target.page();
      if (!page) throw new Error('候选人详情页未就绪');
      pages = [page];
    } finally { await session.detach().catch(() => {}); }
  }
  let page = pages[0];
  if (name && pages.length > 1) {
    for (const candidate of pages) {
      if ((await candidate.title()).trim() === name) { page = candidate; break; }
    }
  }
  // Ads and long-lived requests can keep readyState below complete. Wait for
  // this candidate's actual UI, retry only the read, and preserve all tabs.
  await waitCandidateReady(page, expected, name);
  const title = (await page.title()).trim();
  if (name && title !== name) throw new Error('51job 页面姓名与本地候选人不一致，已停止发送');
  return page;
}

async function waitCandidateReady(page, expected, name) {
  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      await page.waitForFunction((resumeID, expectedName) => {
        const current = new URL(location.href);
        if (current.pathname !== '/Revision/talent/resume/detail' || current.searchParams.get('resumeId') !== resumeID) return false;
        if (expectedName && document.title.trim() !== expectedName) return false;
        const chat = document.querySelector('#chatAreaEmbed');
        if (chat?.closest('[data-type="embed"]')?.id === 'im-sdk-container' &&
            chat.querySelector('.input-textarea_self')?.getClientRects().length &&
            (!expectedName || (chat.querySelector('.username-text')?.textContent || '').trim().replace(/小姐$/, '女士') === expectedName.replace(/小姐$/, '女士'))) return true;
        return Array.from(document.querySelectorAll('button.store_resume_button, .btn_item_chat .chat_btn'))
          .some(el => ['立即Hi聊', '继续沟通', 'Hi聊'].includes((el.innerText || '').trim().replace(/\s+/g, '')) && el.getClientRects().length > 0);
      }, { polling: 250, timeout: 12000 }, expected.resumeId, name || '');
      return;
    } catch {
      if (!sameCandidatePage(page.url(), expected)) throw new Error('候选人详情地址已变化，请检查平台登录状态后重试；未发送');
      if (attempt === 1) {
        const title = (await page.title()).trim();
        if (name && title && title !== name) throw new Error('51job 页面姓名与本地候选人不一致，已停止发送');
        throw new Error('候选人页面沟通入口未就绪，已自动重试；请检查登录或页面加载后继续，本次未发送');
      }
    }
  }
}

function sameDisplayedName(left, right) {
  return String(left || '').trim().replace(/小姐$/, '女士') === String(right || '').trim().replace(/小姐$/, '女士');
}

function findCandidateChatEntry(doc) {
  return Array.from(doc.querySelectorAll('button.store_resume_button, .btn_item_chat .chat_btn'))
    .find(el => ['立即Hi聊', '继续沟通', 'Hi聊'].includes((el.innerText || '').trim().replace(/\s+/g, '')) &&
      el.getClientRects().length > 0) || null;
}

function platformChatGateVisible() {
  return Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
    .some(dialog => dialog.getClientRects().length > 0 &&
      !!dialog.querySelector('.eh_forwarder_searcher input') &&
      !!dialog.querySelector('.el-dialog__footer'));
}

function searchCommunicationJob(title) {
  const dialog = Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
    .find(element => element.getClientRects().length > 0);
  const input = dialog?.querySelector('.eh_forwarder_searcher input');
  if (!input) return false;
  input.focus();
  if (!dialog.querySelector('.list_box_container')?.getClientRects().length) {
    input.dispatchEvent(new FocusEvent('focus'));
  }
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
  if (setter) setter.call(input, title);
  else input.value = title;
  input.dispatchEvent(new Event('input', { bubbles: true }));
  input.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', code: 'Enter', keyCode: 13, which: 13, bubbles: true }));
  return true;
}

function clickExactCommunicationJob(title) {
  const dialog = Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
    .find(element => element.getClientRects().length > 0);
  if (!dialog) return { status: 'dialog_missing' };
  const clean = value => String(value || '').trim().replace(/\s+/g, ' ');
  const rows = Array.from(dialog.querySelectorAll('.list_box_container .user_info'))
    .filter(row => row.getClientRects().length > 0);
  const matches = rows.filter(row => clean(row.querySelector('.name')?.textContent) === clean(title));
  if (matches.length === 0) {
    return { status: 'not_found', available: rows.map(row => clean(row.querySelector('.name')?.textContent)).slice(0, 5) };
  }
  if (matches.length > 1) return { status: 'ambiguous', count: matches.length };
  matches[0].click();
  return { status: 'clicked' };
}

async function selectCommunicationJob(page, title) {
  const requested = String(title || '').trim();
  if (!requested) {
    return { status: 'needs_job_selection', message: '当前招聘项目未配置岗位名称，无法自动选择 51job 沟通职位；本次未发送消息' };
  }
  const session = await page.createCDPSession();
  try {
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    const alreadySelected = await page.evaluate(job => {
      const picker = document.querySelector('.el-dialog[aria-label="立即Hi聊"] .eh_forwarder_searcher')?.parentElement?.__vue__;
      return picker?.jobName?.trim() === job && !!picker?.jobId;
    }, requested);
    if (alreadySelected) {
      return { status: 'needs_confirmation', message: `51job 已选中沟通职位“${requested}”。请核对平台显示的消耗点数并决定是否发起Hi聊；本次软件未发送消息` };
    }
    if (!await page.evaluate(searchCommunicationJob, requested)) {
      return { status: 'needs_job_selection', message: '51job 职位选择框未就绪；本次未发送消息' };
    }
    const resultsReady = await page.waitForFunction(job => {
      const dialog = document.querySelector('.el-dialog[aria-label="立即Hi聊"]');
      const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
      if (!picker || picker.loading) return false;
      const names = Array.from(dialog?.querySelectorAll('.list_box_container .user_info .name') || [])
        .filter(element => element.getClientRects().length > 0)
        .map(element => (element.textContent || '').trim());
      const noMatch = dialog?.querySelector('.list_box_container .note');
      return (names.length > 0 && Number(picker.totalcount) <= names.length && names.every(name => name.includes(job))) ||
        (!!noMatch?.getClientRects().length && (noMatch.textContent || '').includes('未找到'));
    }, { polling: 100, timeout: 5000 }, requested).then(() => true).catch(() => false);
    if (!resultsReady) {
      return { status: 'needs_job_selection', message: '51job 职位搜索结果未能完整加载，未自动选择；本次未发送消息' };
    }
    const selection = await page.evaluate(clickExactCommunicationJob, requested);
    if (selection.status !== 'clicked') {
      const reason = selection.status === 'ambiguous' ? `发现 ${selection.count} 个同名职位，无法判定应选哪一个`
        : selection.status === 'not_found' ? `未找到与“${requested}”完全一致的已发布职位`
          : '51job 职位选择窗口已关闭';
      return { status: 'needs_job_selection', message: `${reason}；本次未发送消息，请在平台核对职位` };
    }
    const selected = await page.waitForFunction(job => {
      const dialog = document.querySelector('.el-dialog[aria-label="立即Hi聊"]');
      const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
      return picker?.jobName?.trim() === job && !!picker?.jobId;
    }, { polling: 100, timeout: 3000 }, requested).then(() => true).catch(() => false);
    if (!selected) {
      return { status: 'needs_job_selection', message: '已点击同名职位，但 51job 未确认选中状态；本次未发送消息，请在平台核对' };
    }
    return { status: 'needs_confirmation', message: `已自动选择 51job 沟通职位“${requested}”。请核对平台显示的消耗点数并决定是否发起Hi聊；本次软件未发送消息` };
  } finally {
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: false }).catch(() => {});
    await session.detach().catch(() => {});
  }
}

function readCommunicationJobs(keyword = '') {
  const dialog = Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
    .find(element => element.getClientRects().length > 0);
  const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
  if (!picker) return { status: 'error', message: '51job 职位列表未打开，未发起Hi聊' };
  const sources = keyword
    ? [['搜索结果', picker.jobList || []]]
    : [['我的职位', picker.myJobList || []], ['公司职位', picker.companJobList || []]];
  const seen = new Set();
  const jobs = [];
  for (const [source, items] of sources) {
    for (const item of items) {
      const id = String(item.jobid || '').trim();
      const title = String(item.jobname || item.job_name || '').trim();
      if (!id || !title || seen.has(id)) continue;
      seen.add(id);
      jobs.push({ id, title, company: String(item.coName || item.coname || '').trim(),
        workArea: String(item.workArea || item.workarea || '').trim(), source,
        publishStatus: String(item.publishStatus || item.publish_status || '').trim() });
    }
  }
  const hasMore = keyword ? Number(picker.totalcount) > jobs.length
    : String(picker.moreMyJob) === '1' || String(picker.moreCompanyJob) === '1';
  return { status: 'jobs', jobs, hasMore,
    message: jobs.length ? `已从 51job 加载 ${jobs.length} 个可选沟通职位${hasMore ? '；还有更多职位，请输入名称搜索' : ''}`
      : '51job 暂未返回可选沟通职位，请输入职位关键词搜索或检查企业账号' };
}

async function listCommunicationJobs(page, keyword = '') {
  const query = String(keyword || '').trim();
  const session = await page.createCDPSession();
  try {
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    const endpoint = query ? '/jobj/list/v1/get_talent_joblist' : '/jobj/list/get_combine_joblist';
    const response = page.waitForResponse(item => {
      try { return new URL(item.url()).pathname === endpoint && item.status() === 200; }
      catch { return false; }
    }, { timeout: 8000 }).then(() => true).catch(() => false);
    if (!await page.evaluate(searchCommunicationJob, query)) {
      return { status: 'error', message: '51job 职位搜索框未就绪，未发起Hi聊' };
    }
    if (!await response) return { status: 'error', message: '51job 职位查询未返回，请重试或更换搜索词' };
    const loaded = await page.waitForFunction(search => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
        .find(element => element.getClientRects().length > 0);
      const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
      const input = dialog?.querySelector('.eh_forwarder_searcher input');
      if (!picker || picker.loading || input?.value.trim() !== search ||
          String(picker.value || '').trim() !== search) return false;
      if (!search) return Array.isArray(picker.myJobList) && Array.isArray(picker.companJobList);
      return Array.isArray(picker.jobList) &&
        (picker.jobList.length > 0 || Number(picker.totalcount) === 0);
    }, { polling: 100, timeout: 8000 }, query).then(() => true).catch(() => false);
    if (!loaded) return { status: 'error', message: '51job 职位列表未加载完成，请重试或更换搜索词' };
    return await page.evaluate(readCommunicationJobs, query);
  } finally {
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: false }).catch(() => {});
    await session.detach().catch(() => {});
  }
}

function setCommunicationJobByID(jobID, title) {
  const dialog = Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
    .find(element => element.getClientRects().length > 0);
  const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
  if (!picker || typeof picker.setJob !== 'function') return false;
  const items = [...(picker.jobList || []), ...(picker.myJobList || []), ...(picker.companJobList || [])];
  const item = items.find(job => String(job.jobid || '') === jobID &&
    String(job.jobname || job.job_name || '').trim() === title);
  if (!item) return false;
  picker.setJob(String(item.coid || ''), String(item.coName || item.coname || ''), jobID, title);
  return true;
}

async function selectCommunicationJobByID(page, jobID, title) {
  if (!jobID || !title) return { status: 'needs_job_selection', message: '请先在软件中选择 51job 沟通职位' };
  const session = await page.createCDPSession();
  try {
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    let chosen = await page.evaluate(setCommunicationJobByID, jobID, title);
    if (!chosen) {
      const search = await listCommunicationJobs(page, title);
      if (search.status !== 'jobs' || !search.jobs.some(job => job.id === jobID && job.title === title)) {
        return { status: 'needs_job_selection', message: `51job 当前未返回所选职位“${title}”（ID ${jobID}），未发起Hi聊；请刷新职位列表后重选` };
      }
      chosen = await page.evaluate(setCommunicationJobByID, jobID, title);
    }
    if (!chosen) return { status: 'needs_job_selection', message: '51job 所选职位已从列表消失，未发起Hi聊' };
    const selected = await page.waitForFunction((id, name) => {
      const dialog = document.querySelector('.el-dialog[aria-label="立即Hi聊"]');
      const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
      return String(picker?.jobId || '') === id && String(picker?.jobName || '').trim() === name;
    }, { polling: 100, timeout: 3000 }, jobID, title).then(() => true).catch(() => false);
    return selected
      ? { status: 'needs_confirmation', message: `已选择 51job 沟通职位“${title}”，请核对点数；尚未发起Hi聊` }
      : { status: 'needs_job_selection', message: '51job 未确认选中 HR 指定的职位，未发起Hi聊' };
  } finally {
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: false }).catch(() => {});
    await session.detach().catch(() => {});
  }
}

function readHiChatQuote() {
  const dialog = Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
    .find(element => element.getClientRects().length > 0);
  const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
  const gate = dialog?.parentElement?.parentElement?.__vue__;
  const text = (dialog?.querySelector('.msg')?.textContent || '').replace(/\s+/g, '');
  const match = text.match(/需消耗(\d+)个Hi聊数/);
  const button = Array.from(dialog?.querySelectorAll('.el-dialog__footer button') || [])
    .find(element => (element.textContent || '').trim() === '发起Hi聊' && element.getClientRects().length > 0);
  return {
    jobId: String(gate?.jobId || ''), jobTitle: String(gate?.jobName || '').trim(),
    points: match ? Number(match[1]) : null,
    ready: !!dialog && !!button && !button.disabled && !picker?.loading && !gate?.loading &&
      !!gate?.jobId && String(gate.jobId) === String(picker?.jobId) &&
      String(gate.jobName || '').trim() === String(picker?.jobName || '').trim()
  };
}

async function previewHiChat(page, title) {
  await page.waitForFunction(() => {
    const dialog = document.querySelector('.el-dialog[aria-label="立即Hi聊"]');
    const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
    const gate = dialog?.parentElement?.parentElement?.__vue__;
    const text = (dialog?.querySelector('.msg')?.textContent || '').replace(/\s+/g, '');
    return !picker?.loading && !gate?.loading && /需消耗\d+个Hi聊数/.test(text);
  }, { polling: 100, timeout: 5000 }).catch(() => {});
  const quote = await page.evaluate(readHiChatQuote);
  if (!quote.ready || quote.jobTitle !== title || !quote.jobId || !Number.isSafeInteger(quote.points) || quote.points <= 0) {
    return { status: 'needs_manual_confirmation', message: '51job 未明确显示可核对的Hi聊点数；未点击发起Hi聊，请在平台核对' };
  }
  return { status: 'quote', jobId: quote.jobId, jobTitle: quote.jobTitle,
    targetId: page.target()._targetId, points: quote.points,
    message: `51job 预计消耗 ${quote.points} 个Hi聊数` };
}

// 在同一次浏览器脚本执行中复核并点击，避免预览后切换人选、职位或点数。
function clickQuotedHiChat(candidateID, name, jobID, jobTitle, points) {
  const current = new URL(document.URL);
  if (current.protocol !== 'https:' || current.hostname !== 'ehire.51job.com' ||
      current.pathname !== '/Revision/talent/resume/detail' ||
      current.searchParams.get('resumeId') !== candidateID || document.title.trim() !== name) {
    return { clicked: false, reason: 'candidate' };
  }
  const dialog = Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
    .find(element => element.getClientRects().length > 0);
  if (!dialog) return { clicked: false, reason: 'dialog' };
  const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
  const gate = dialog?.parentElement?.parentElement?.__vue__;
  const cost = (dialog?.querySelector('.msg')?.textContent || '').replace(/\s+/g, '').match(/需消耗(\d+)个Hi聊数/);
  const button = Array.from(dialog?.querySelectorAll('.el-dialog__footer button') || [])
    .find(element => (element.textContent || '').trim() === '发起Hi聊' && element.getClientRects().length > 0);
  if (picker?.loading || gate?.loading) return { clicked: false, reason: 'loading' };
  if (String(picker?.jobId || '') !== jobID || String(gate?.jobId || '') !== jobID ||
      String(picker?.jobName || '').trim() !== jobTitle ||
      String(gate?.jobName || '').trim() !== jobTitle) return { clicked: false, reason: 'job' };
  if (Number(cost?.[1]) !== points) return { clicked: false, reason: 'points' };
  if (!button || button.disabled) return { clicked: false, reason: 'button' };
  button.click();
  return { clicked: true };
}

async function confirmHiChat(browser, args, expected) {
  const target = browser.targets().find(item => item._targetId === args.targetID);
  if (!target) return { status: 'uncertain', message: '预览时的 51job 标签页已关闭；请检查聊天记录，暂勿重复发起' };
  const page = await target.page();
  if (!page || !sameCandidatePage(page.url(), expected) || (await page.title()).trim() !== args.name) {
    return { status: 'uncertain', message: '候选人页面已变化，未点击发起Hi聊；请检查平台记录' };
  }
  const quote = await page.evaluate(readHiChatQuote);
  if (!quote.ready || quote.jobId !== args.expectedJobID || quote.jobTitle !== args.jobTitle ||
      quote.points !== Number(args.expectedPoints)) {
    return { status: 'uncertain', message: '51job 职位或点数已变化，未点击发起Hi聊；请重新核对平台页面' };
  }
  const session = await page.createCDPSession();
  try {
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    // 51job 获得焦点会重新加载职位选择器；等待这次核价结束，再重新核对报价。
    const settled = await page.waitForFunction(() => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog[aria-label="立即Hi聊"]'))
        .find(element => element.getClientRects().length > 0);
      const picker = dialog?.querySelector('.eh_forwarder_searcher')?.parentElement?.__vue__;
      const gate = dialog?.parentElement?.parentElement?.__vue__;
      return !!dialog && !!picker && !!gate && !picker.loading && !gate.loading;
    }, { polling: 100, timeout: 5000 }).then(() => true).catch(() => false);
    if (!settled) return { status: 'error', message: '51job 职位点数仍在重新核算，未点击发起Hi聊；请稍后重新预览点数' };
    const refreshed = await page.evaluate(readHiChatQuote);
    if (!refreshed.ready || refreshed.jobId !== args.expectedJobID ||
        refreshed.jobTitle !== args.jobTitle || refreshed.points !== Number(args.expectedPoints)) {
      return { status: 'error', message: '51job 重新核算后的职位或点数与 HR 确认的不一致，未点击发起Hi聊；请重新预览' };
    }
    const before = await page.evaluate(() => document.querySelectorAll('#chatAreaEmbed .message-item.mine').length);
    let clickResult;
    try {
      clickResult = await page.evaluate(clickQuotedHiChat, expected.resumeId, args.name,
        args.expectedJobID, args.jobTitle, Number(args.expectedPoints));
    } catch {
      return { status: 'uncertain', message: '点击发起Hi聊时页面状态不明；请检查聊天记录，暂勿重复操作' };
    }
    if (!clickResult?.clicked) {
      const reasons = {
        candidate: '候选人页面已变化', dialog: 'Hi聊窗口已关闭', loading: '职位点数再次进入核算',
        job: '沟通职位已变化', points: 'Hi聊点数已变化', button: '发起Hi聊按钮不可用'
      };
      return { status: 'error', message: `51job ${reasons[clickResult?.reason] || '页面状态已变化'}，未点击发起Hi聊；请重新预览核对` };
    }
    try {
      await page.waitForFunction((name, count) => {
        const root = document.querySelector('#chatAreaEmbed');
        const normalize = value => String(value || '').trim().replace(/小姐$/, '女士');
        const currentName = root?.querySelector('.username-text')?.textContent || '';
        const outgoing = Array.from(root?.querySelectorAll('.message-item.mine') || []);
        return !!root?.getClientRects().length &&
          root.closest('[data-type="embed"]')?.id === 'im-sdk-container' &&
          normalize(currentName) === normalize(name) && outgoing.length > count &&
          !!outgoing.at(-1)?.querySelector('.im-message-success');
      }, { polling: 100, timeout: 12000 }, args.name, before);
    } catch {
      return { status: 'uncertain', message: '已点击 51job 发起Hi聊，但未核实到该候选人的新消息发送成功；请查看聊天记录，暂勿重复操作' };
    }
  } finally {
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: false }).catch(() => {});
    await session.detach().catch(() => {});
  }
  return { status: 'sent', message: '51job 已显示该候选人的新招呼消息发送成功' };
}

async function openCandidateChat(page, name, jobTitle = '', autoSelect = true) {
  const existing = await page.evaluate(() => ({
    visible: !!document.querySelector('#chatAreaEmbed .input-textarea_self[contenteditable="true"]')?.getClientRects().length,
    embedded: document.querySelector('#chatAreaEmbed')?.closest('[data-type="embed"]')?.id === 'im-sdk-container',
    name: (document.querySelector('#chatAreaEmbed .username-text')?.textContent || '').trim()
  }));
  if (existing.visible && existing.embedded && sameDisplayedName(existing.name, name)) return existing;
  if (await page.evaluate(platformChatGateVisible)) {
    return autoSelect ? selectCommunicationJob(page, jobTitle)
      : { status: 'needs_job_selection', message: '51job 沟通职位选择窗口已打开；本次未发送消息' };
  }
  const opened = await page.evaluate(() => {
    const entry = Array.from(document.querySelectorAll('button.store_resume_button, .btn_item_chat .chat_btn'))
      .find(el => ['立即Hi聊', '继续沟通', 'Hi聊'].includes((el.innerText || '').trim().replace(/\s+/g, '')) &&
        el.getClientRects().length > 0);
    if (!entry) return false;
    entry.click();
    return true;
  });
  if (!opened) throw new Error('未找到当前候选人的「立即Hi聊」入口');
  if (await page.evaluate(platformChatGateVisible)) {
    return autoSelect ? selectCommunicationJob(page, jobTitle)
      : { status: 'needs_job_selection', message: '51job 沟通职位选择窗口已打开；本次未发送消息' };
  }
  try {
    await page.waitForSelector('#chatAreaEmbed .input-textarea_self[contenteditable="true"]', { visible: true, timeout: 8000 });
  } catch {
    if (await page.evaluate(platformChatGateVisible).catch(() => false)) {
      return autoSelect ? selectCommunicationJob(page, jobTitle)
        : { status: 'needs_job_selection', message: '51job 沟通职位选择窗口已打开；本次未发送消息' };
    }
    throw new Error('51job 聊天输入框未出现；本次软件未发送消息。请查看候选人页面是否有登录、权限或平台提示');
  }
  const identity = await page.evaluate(() => ({
    embedded: document.querySelector('#chatAreaEmbed')?.closest('[data-type="embed"]')?.id === 'im-sdk-container',
    name: (document.querySelector('#chatAreaEmbed .username-text')?.textContent || '').trim()
  }));
  if (!identity.embedded || !identity.name || !sameDisplayedName(identity.name, name)) {
    throw new Error('聊天窗口姓名与本地候选人不一致，已停止操作');
  }
  return identity;
}

async function conversationMessages(page) {
  return page.evaluate(() => Array.from(
    document.querySelectorAll('#chatAreaEmbed .message-list .message-item')
  ).map(item => ({
    mine: item.classList.contains('mine'),
    text: (item.querySelector('.text-content')?.textContent || '').trim().slice(0, 2000),
    links: Array.from(item.querySelectorAll('a[href]')).map(link => ({
      href: link.href, text: (link.textContent || '').trim(), download: link.getAttribute('download') || ''
    }))
  })));
}

async function sendCandidateMessage(page, message, includeBaseline = false) {
  const text = String(message || '').trim();
  if (!text || text.length > 500) throw new Error('沟通内容需为 1–500 字');
  const before = await page.evaluate(msg => {
    const root = document.querySelector('#chatAreaEmbed');
    const draft = root?.querySelector('.input-textarea_self');
    if (!draft || draft.textContent?.trim()) return { ready: false, count: 0 };
    const count = Array.from(root.querySelectorAll('.message-item.mine'))
      .filter(el => el.querySelector('.text-content')?.textContent?.trim() === msg).length;
    return { ready: true, count };
  }, text);
  if (!before.ready) throw new Error('聊天框已有未发送草稿，请先在 51job 页面处理');
  if (before.count > 0) throw new Error('聊天记录中已有相同内容，未重复发送；请先核对平台记录');
  const baseline = includeBaseline ? (await conversationMessages(page))
    .filter(item => !item.mine)
    .flatMap(item => item.links || [])
    .filter(link => { try { return !!selectInboundAttachment([{ mine: false, links: [link] }]); } catch { return false; } })
    .map(attachmentFingerprint) : [];
  await page.type('#chatAreaEmbed .input-textarea_self', text, { delay: 12 });
  const sendButton = await page.$('#chatAreaEmbed button.new-send-button');
  if (!sendButton) throw new Error('未找到 51job 发送按钮，消息未发送');
  await sendButton.click();
  try {
    await page.waitForFunction((msg, count) => {
      const root = document.querySelector('#chatAreaEmbed');
      const matches = Array.from(root?.querySelectorAll('.message-item.mine') || [])
        .filter(el => el.querySelector('.text-content')?.textContent?.trim() === msg);
      return matches.length > count && matches.at(-1)?.querySelector('.im-message-success');
    }, { timeout: 12000 }, text, before.count);
  } catch { throw new Error('消息点击后未取得平台已发送状态，请人工查看聊天记录，暂勿重复发送'); }
  return { status: 'sent', baseline, message: '51job 聊天记录已显示这条消息发送成功' };
}

async function inspectReply(page, outputDir, recordID, known = [], download = true) {
  const messages = await conversationMessages(page);
  const replies = messages.filter(item => !item.mine && item.text).slice(-10).map(item => item.text);
  const attachmentNames = messages.filter(item => !item.mine)
    .map(item => selectInboundAttachment([item])).filter(Boolean).slice(-10).map(item => item.fileName);
  const summary = { replies, attachmentNames };
  const attachment = selectInboundAttachment(messages, known);
  if (!attachment || !download) return { ...summary, status: replies.length || attachmentNames.length ? 'reply' : 'waiting',
    message: replies.length || attachmentNames.length ? '已读取当前已加载会话中的候选人回复与附件；可在平台查看更早记录' : '当前已加载会话中尚未看到候选人的文字回复或可下载简历附件' };
  const encoded = await page.evaluate(async href => {
    const response = await fetch(href, { credentials: 'include' });
    if (!response.ok) throw new Error(`附件读取失败：HTTP ${response.status}`);
    const buffer = await response.arrayBuffer();
    if (buffer.byteLength > 15 * 1024 * 1024) throw new Error('附件超过 15MB');
    const bytes = new Uint8Array(buffer);
    let binary = '';
    for (let i = 0; i < bytes.length; i += 8192) binary += String.fromCharCode(...bytes.subarray(i, i + 8192));
    return btoa(binary);
  }, attachment.href);
  const file = Buffer.from(encoded, 'base64');
  const extension = attachment.fileName.split('.').at(-1).toLowerCase();
  if (!validResumeBytes(file, extension)) throw new Error('收到的附件格式或内容不符合简历文件要求，未导入');
  const directory = path.join(outputDir, '51job_received', recordID);
  fs.mkdirSync(directory, { recursive: true });
  const digest = crypto.createHash('sha256').update(file).digest('hex').slice(0, 20);
  const filePath = path.join(directory, `${digest}.${extension}`);
  fs.writeFileSync(filePath, file, { mode: 0o600 });
  return { ...summary, status: 'downloaded', filePath, fileName: attachment.fileName, message: '已从该候选人的回复中取得并校验简历附件' };
}

function readArgs(argv) {
  const value = flag => { const index = argv.indexOf(flag); return index < 0 ? '' : argv[index + 1] || ''; };
  return { action: value('--action'), url: value('--candidate-url'), name: value('--candidate-name'),
    message: value('--message'), jobTitle: value('--job-title'), outputDir: value('--data-dir'), recordID: value('--resume-record'),
    expectedJobID: value('--expected-job-id'), expectedPoints: value('--expected-points'), targetID: value('--target-id'),
    known: value('--known-links') };
}

async function revealJobConfirmation(browser, page, outputDir) {
  try {
    fs.mkdirSync(outputDir, { recursive: true });
    fs.writeFileSync(path.join(outputDir, 'manual_view_51job.flag'), 'visible', { mode: 0o600 });
    const { restoreCandidateWindow } = require('./show_candidate_window');
    await restoreCandidateWindow(browser, page.target()._targetId);
    return true;
  } catch { return false; }
}

async function main(argv = process.argv.slice(2)) {
  let browser;
  const args = readArgs(argv);
  try {
    const expected = parseCandidateUrl(args.url);
    if (!['verify_candidate', 'list_jobs', 'preview_hi', 'confirm_hi', 'ask_resume', 'send_outreach', 'check_resume', 'check_replies'].includes(args.action)) throw new Error('不支持的 51job 沟通操作');
    if (!/^[A-Za-z0-9_-]+$/.test(args.recordID)) throw new Error('候选人档案编号无效');
    if (args.action === 'confirm_hi' && (!args.name || !args.jobTitle || !args.expectedJobID || !args.targetID ||
        !Number.isSafeInteger(Number(args.expectedPoints)) || Number(args.expectedPoints) <= 0)) {
      throw new Error('51job 点数确认参数不完整，未点击发起Hi聊');
    }
    try {
      browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null });
    } catch (error) {
      for (let i = 0; i < 5; i++) {
        await new Promise(r => setTimeout(r, 600));
        try {
          browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null });
          if (browser) break;
        } catch (_) {}
      }
      if (!browser) {
        if (args.action === 'confirm_hi') throw error;
        process.stdout.write(JSON.stringify({ status: 'needs_login', message: '51job 企业浏览器未就绪或尚未登录，已为您打开登录窗口，请在浏览器中登录后重试' }) + '\n');
        return;
      }
    }
    if (args.action === 'confirm_hi') {
      const result = await confirmHiChat(browser, args, expected);
      if (result.status === 'uncertain') {
        const target = browser.targets().find(item => item._targetId === args.targetID);
        const page = await target?.page();
        if (page) await revealJobConfirmation(browser, page, args.outputDir);
      }
      process.stdout.write(JSON.stringify(result) + '\n');
      return;
    }
    const page = await candidatePage(browser, expected, args.name);
    if (args.action === 'verify_candidate') {
      process.stdout.write(JSON.stringify({ status: 'verified', targetId: expected.resumeId, message: '51job 详情页与候选人姓名已核对' }) + '\n');
      return;
    }
    const readingReply = args.action === 'check_resume' || args.action === 'check_replies';
    const manualJobSelection = readingReply || args.action === 'list_jobs' || (args.action === 'preview_hi' && !!args.expectedJobID);
    const chat = await openCandidateChat(page, args.name,
      readingReply ? '' : args.jobTitle, !manualJobSelection);
    if (readingReply && chat.status) {
      process.stdout.write(JSON.stringify({status:'waiting',message:'尚未发现可读取的已有会话；可打开平台核查，本次未发起Hi聊或发送消息',replies:[],attachmentNames:[]})+'\n');
      return;
    }
    if (args.action === 'list_jobs') {
      const result = chat.status === 'needs_job_selection'
        ? await listCommunicationJobs(page, args.message)
        : { status: 'error', message: '51job 已有该候选人的聊天窗口；请先核对现有沟通记录，未重复打招呼' };
      process.stdout.write(JSON.stringify(result) + '\n');
      return;
    }
    if (args.action === 'preview_hi') {
      let selection = chat;
      if (args.expectedJobID && chat.status === 'needs_job_selection') {
        selection = await selectCommunicationJobByID(page, args.expectedJobID, args.jobTitle);
      }
      const result = selection.status === 'needs_confirmation' ? await previewHiChat(page, args.jobTitle)
        : selection.status === 'needs_job_selection' ? selection
          : { status: 'error', message: '51job 已有该候选人的聊天窗口，请先核对现有沟通记录，未重复打招呼' };
      if (result.status === 'needs_job_selection' || result.status === 'needs_manual_confirmation') {
        if (!await revealJobConfirmation(browser, page, args.outputDir)) result.message += '；请手动打开候选人页面';
      }
      process.stdout.write(JSON.stringify(result) + '\n');
      return;
    }
    if (chat.status === 'needs_confirmation' || chat.status === 'needs_job_selection') {
      if (!await revealJobConfirmation(browser, page, args.outputDir)) {
        chat.message += '；请手动打开 51job 候选人页面查看';
      }
      process.stdout.write(JSON.stringify(chat) + '\n');
      return;
    }
    const result = readingReply
      ? await inspectReply(page, args.outputDir, args.recordID, JSON.parse(args.known || '[]'), args.action === 'check_resume')
      : await sendCandidateMessage(page, args.message, args.action === 'ask_resume');
    process.stdout.write(JSON.stringify(result) + '\n');
  } catch (error) {
    const status = args.action === 'confirm_hi' ? 'uncertain' : error.code === '51JOB_BROWSER_UNAVAILABLE' ? 'needs_login' : 'error';
    const message = args.action === 'confirm_hi'
      ? `51job 发起Hi聊过程未完成（${error.message}）；请核对该候选人的聊天记录，暂勿重复操作`
      : error.message;
    process.stdout.write(JSON.stringify({ status, message }) + '\n');
  } finally {
    if (browser) await browser.disconnect().catch(() => {});
  }
}

if (require.main === module) main();
module.exports = { parseCandidateUrl, sameCandidatePage, sameDisplayedName, findCandidateChatEntry,
  candidatePage, waitCandidateReady,
  attachmentFingerprint, selectInboundAttachment, validResumeBytes, platformChatGateVisible,
  searchCommunicationJob, clickExactCommunicationJob, selectCommunicationJob, readHiChatQuote,
  readCommunicationJobs, listCommunicationJobs, setCommunicationJobByID, selectCommunicationJobByID,
  previewHiChat, clickQuotedHiChat, confirmHiChat, openCandidateChat,
  sendCandidateMessage, inspectReply, main };
