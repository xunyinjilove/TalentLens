/** Read only: import visible 51job employer applications from an authenticated Edge tab.
 * No message, status, or platform record is changed here. Unknown IDs are rejected.
 */
const puppeteer = require('puppeteer-core');

async function main() {
  let browser;
  try {
    const response = await fetch('http://127.0.0.1:9503/json/list', { signal: AbortSignal.timeout(5000) });
    const targets = await response.json();
    const likelyPages = targets.filter(target => {
      if (target.type !== 'page') return false;
      try {
        const u = new URL(target.url);
        return u.hostname === 'ehire.51job.com' && (/apply|application|resume|talent/i.test(u.pathname) || /主动投递|收到的简历|应聘管理/.test(target.title || ''));
      } catch { return false; }
    });
    if (!likelyPages.length) {
      process.stdout.write(JSON.stringify({ status: 'needs_action', message: '请先在已登录的 51job 企业端打开主动投递列表页，再点击导入' }));
      return;
    }
    browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 10000 });
    const pages = (await browser.pages()).filter(page => /^https:\/\/ehire\.51job\.com\//i.test(page.url()));
    const candidates = [];
    for (const page of pages) {
      const text = await page.evaluate(() => document.body?.innerText?.slice(0, 3000) || '').catch(() => '');
      if (/主动投递|收到的简历|应聘管理|投递记录/.test(text) || /apply|application|receive/i.test(page.url())) candidates.push(page);
    }
    if (candidates.length !== 1) {
      process.stdout.write(JSON.stringify({ status: 'needs_action', message: candidates.length ? '检测到多个投递页，请只保留一个 51job 主动投递列表页后重试' : '请先在已登录的 51job 企业端打开主动投递列表页，再点击导入' }));
      return;
    }
    const records = await candidates[0].evaluate(() => {
      const rows = Array.from(document.querySelectorAll('[data-application-id], [data-apply-id], [data-applyid], tr[data-applicationid]'));
      const output = [];
      for (const row of rows) {
        const applicationId = row.getAttribute('data-application-id') || row.getAttribute('data-apply-id') || row.getAttribute('data-applyid') || row.getAttribute('data-applicationid') || '';
        if (!/^[\w-]{4,100}$/.test(applicationId)) continue;
        const link = row.querySelector('a[href*="/talent/resume/detail"]');
        const profileUrl = link?.href || '';
        let platformCandidateId = '';
        try {
          const u = new URL(profileUrl);
          if (u.hostname === 'ehire.51job.com' && u.pathname === '/Revision/talent/resume/detail' && /^\d+$/.test(u.searchParams.get('resumeId') || '')) platformCandidateId = u.searchParams.get('resumeId');
        } catch {}
        const platformJobId = row.getAttribute('data-job-id') || row.getAttribute('data-jobid') || row.querySelector('[data-job-id]')?.getAttribute('data-job-id') || '';
        const candidateName = row.querySelector('[data-candidate-name], .candidate-name, .resume-name, .name')?.textContent?.trim() || '';
        const appliedAt = row.querySelector('[data-applied-at], .apply-time, .delivery-time')?.textContent?.trim() || '';
        if (!candidateName || !platformCandidateId) continue;
        output.push({ platform: '51job', platformAccount: '', applicationId, platformCandidateId, platformJobId, candidateName, profileUrl, appliedAt, resumeText: '' });
      }
      return { records: output, rowCount: rows.length };
    });
    if (records.rowCount > 0 && records.records.length === 0) {
      process.stdout.write(JSON.stringify({ status: 'needs_action', message: '投递列表缺少可核实的申请 ID 或候选人详情 ID，未导入任何记录；请检查当前页面或更新平台适配器' }));
      return;
    }
    if (records.rowCount === 0) {
      process.stdout.write(JSON.stringify({ status: 'needs_action', message: '当前 51job 页面没有可核实的主动投递行；请打开投递列表，或更新平台适配器' }));
      return;
    }
    for (const record of records.records) {
      const detail = await browser.newPage();
      try {
        await detail.goto(record.profileUrl, { waitUntil: 'domcontentloaded', timeout: 12000 });
        record.resumeText = await detail.evaluate(name => {
          const section = document.querySelector('.resume-detail, .resume-content, [class*="resumeDetail"], [class*="resume-detail"]');
          const text = section?.innerText?.trim() || '';
          return text.includes(name) && text.length >= 100 ? text.slice(0, 50000) : '';
        }, record.candidateName);
      } catch { record.resumeText = ''; }
      finally { await detail.close().catch(() => {}); }
    }
    const rejectedCount = records.rowCount - records.records.length;
    process.stdout.write(JSON.stringify({ status: 'ok', message: `已读取 ${records.records.length} 条可核实投递；${rejectedCount} 条因缺少稳定身份未导入；缺少完整简历的记录需后续补全`, rejectedCount, records: records.records }));
  } catch (err) {
    process.stdout.write(JSON.stringify({ status: 'needs_action', message: `无法读取已登录的 51job 浏览器：${err.message || err}` }));
  } finally {
    if (browser) await browser.disconnect().catch(() => {});
  }
}

main();
