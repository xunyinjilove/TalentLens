/** 从已登录的 51job 校园招聘表单抓取完整专业目录，供本地离线选择。 */
const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer-core');

const jobURL = 'https://ehire.51job.com/Revision/job?mark=new';
const targets = [
  path.join(__dirname, '..', 'frontend', 'src', 'data', '51job_majors.json'),
  path.join(__dirname, '51job_majors.json')
];

function cleanNode(node) {
  const name = String(node.label || '').trim();
  const value = String(node.value || '').trim();
  const children = Array.isArray(node.children) ? node.children.map(cleanNode).filter(Boolean) : [];
  if (!name || !value) return null;
  return children.length ? { name, value, children } : { name, value };
}

async function main() {
  const browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 });
  let page;
  let session;
  try {
    const source = (await browser.pages()).find(item => item.url().startsWith('https://ehire.51job.com/'));
    if (!source) throw new Error('请先打开并登录 51job 企业浏览器');
    session = await source.target().createCDPSession();
    const { targetId } = await session.send('Target.createTarget', { url: jobURL, background: true });
    const target = await browser.waitForTarget(item => item._targetId === targetId, { timeout: 8000 });
    page = await target.page();
    await page.waitForSelector('.job_type_select', { timeout: 20000 });
    const selected = await page.evaluate(() => {
      document.querySelector('.job_type_select input')?.click();
      const option = Array.from(document.querySelectorAll('.job_type_select .el-select-dropdown__item'))
        .find(item => item.textContent.trim() === '校园招聘');
      option?.click();
      return Boolean(option);
    });
    if (!selected) throw new Error('51job 未显示校园招聘选项');
    await page.waitForSelector('[data-id="major"] input', { timeout: 10000 });
    await page.evaluate(() => document.querySelector('[data-id="major"] input')?.click());
    await page.waitForFunction(() => Array.from(document.querySelectorAll('.el-dialog')).some(dialog =>
      dialog.getBoundingClientRect().height > 0 && dialog.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择专业'
      && dialog.querySelectorAll('.cascader_panel_menu:first-child .func-item').length > 5), { timeout: 10000 });
    const raw = await page.evaluate(() => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(item =>
        item.getBoundingClientRect().height > 0 && item.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择专业');
      for (const element of [dialog, ...Array.from(dialog?.querySelectorAll('*') || [])]) {
        if (Array.isArray(element?.__vue__?.options) && element.__vue__.options.length > 5) {
          return element.__vue__.options;
        }
      }
      return null;
    });
    if (!raw) throw new Error('51job 专业目录尚未加载，未覆盖原有本地文件');
    const majors = raw.map(cleanNode).filter(Boolean);
    const level2 = majors.reduce((sum, item) => sum + (item.children?.length || 0), 0);
    const leaves = majors.flatMap(item => item.children || []).reduce((sum, item) => sum + (item.children?.length || 0), 0);
    if (majors.length < 10 || level2 < 20 || leaves < 100) {
      throw new Error(`51job 专业目录疑似不完整：${majors.length}/${level2}/${leaves}，未覆盖原有本地文件`);
    }
    const snapshot = { source: '51job 企业端校园招聘专业选择器', capturedAt: new Date().toISOString(), maxSelections: 5, majors };
    for (const output of targets) fs.writeFileSync(output, `${JSON.stringify(snapshot, null, 2)}\n`, 'utf8');
    process.stdout.write(JSON.stringify({ roots: majors.length, categories: level2, majors: leaves, outputs: targets }) + '\n');
  } finally {
    if (page) await page.close().catch(() => {});
    if (session) await session.detach().catch(() => {});
    await browser.disconnect().catch(() => {});
  }
}

if (require.main === module) main().catch(error => { process.stderr.write(`${error.message}\n`); process.exitCode = 1; });
module.exports = { cleanNode };
