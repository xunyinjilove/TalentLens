/** 51job 企业端职位草稿：只连接已有登录浏览器，不重复启动 Edge。 */
const puppeteer = require('puppeteer-core');
const { createBackgroundPage, keepPageActiveInBackground } = require('./multi_platform_agent');

const JOB_URL = 'https://ehire.51job.com/Revision/job?mark=new';
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

function validate(input) {
  const title = String(input.title || '').trim();
  const description = String(input.description || '').trim();
  const functionPath = String(input.functionPath || '').split('>').map(s => s.trim()).filter(Boolean);
  const keywords = (Array.isArray(input.keywords) ? input.keywords : []).map(s => String(s).trim()).filter(Boolean);
  const minSalary = Number(input.minSalary);
  const maxSalary = Number(input.maxSalary);
  const salaryMonths = Number(input.salaryMonths);
  const headcount = Number(input.headcount);
  const experienceYears = Number(input.experienceYears);
  if (!title) throw new Error('请填写职位名称');
  if (description.length < 50) throw new Error(`职位描述还差 ${50 - description.length} 字`);
  if (functionPath.length < 2) throw new Error('51job 职能路径需填写到末级，例如“互联网技术 > 测试 > 软件测试”');
  if (!keywords.length) throw new Error('请填写至少一个关键词');
  if (!Number.isInteger(minSalary) || !Number.isInteger(maxSalary) || minSalary <= 0 || maxSalary < minSalary) throw new Error('月薪范围应为正整数，最高月薪不能低于最低月薪');
  if (!Number.isInteger(salaryMonths) || salaryMonths < 12 || salaryMonths > 24) throw new Error('请选择 12 至 24 薪');
  if (keywords.length > 10) throw new Error('51job 关键词最多 10 个，请减少后重试');
  if (!Number.isInteger(headcount) || headcount < 1 || headcount > 9999) throw new Error('招聘人数须在 1 至 9999 之间');
  if (!Number.isInteger(experienceYears) || experienceYears < 0 || experienceYears > 10) throw new Error('经验年限须在 0 至 10 年之间');
  if (!String(input.education || '').trim()) throw new Error('请选择学历要求');
  return { title, description, functionPath, keywords, minSalary, maxSalary, salaryMonths, headcount, experienceYears, education: String(input.education).trim() };
}

async function click(page, selector, label) {
  const found = await page.evaluate(sel => { const el = document.querySelector(sel); if (el) el.click(); return Boolean(el); }, selector);
  if (!found) throw new Error(`51job 页面未找到${label}，请在浏览器中检查页面`);
  await pause(120);
}

async function choose(page, root, value, label) {
  const selectOption = () => page.evaluate(({ root, value }) => {
    const target = document.querySelector(root);
    const scope = target?.matches('input') ? target.closest('.el-select') : target;
    const items = Array.from(scope?.querySelectorAll('.el-select-dropdown__item') || []);
    const item = items.find(el => el.textContent.trim() === value);
    if (item) item.click();
    return Boolean(item);
  }, { root, value });
  let found = await selectOption();
  if (!found) {
    await click(page, root.startsWith('input[') ? root : `${root} input`, label);
    await pause(300);
    found = await selectOption();
  }
  if (!found) {
    const options = await page.evaluate(root => {
      const target = document.querySelector(root);
      const scope = target?.matches('input') ? target.closest('.el-select') : target;
      return Array.from(scope?.querySelectorAll('.el-select-dropdown__item') || []).map(el => el.textContent.trim()).slice(0, 12);
    }, root);
    throw new Error(`51job 的${label}没有“${value}”选项（当前可选：${options.join('、') || '无'}）`);
  }
  await pause(150);
}

async function fill(page, selector, value, label) {
  const actual = await page.evaluate(({ selector, value }) => {
    const el = document.querySelector(selector);
    if (!el) return null;
    const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, String(value));
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
    el.blur();
    return el.value;
  }, { selector, value });
  if (actual !== String(value)) throw new Error(`无法填写${label}`);
  await pause(100);
}

async function chooseFunction(page, path) {
  await click(page, '.func-dropdown .all_func_tips', '职能入口');
  for (const name of path) {
    const found = await page.evaluate(value => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.innerText.includes('选择职能'));
      const item = Array.from(dialog?.querySelectorAll('.func-item') || []).find(el => el.title === value && el.getBoundingClientRect().height > 0);
      if (item) item.click();
      return Boolean(item);
    }, name);
    if (!found) throw new Error(`51job 职能分类“${name}”不存在，请在平台页面手动选择`);
    await pause(200);
  }
  const selected = await page.evaluate(() => document.querySelector('[data-id="funcTypeInput"] input')?.value || '');
  if (selected !== path[path.length - 1]) throw new Error('51job 职能未选中，请在平台页面检查');
}

async function chooseKeywords(page, keywords) {
  await click(page, '#job-keywords input', '关键词入口');
  await page.waitForSelector('.keyword-footer_wrapper .confirm-btn', { timeout: 5000 });
  for (const keyword of keywords.slice(0, 10)) {
    const recommended = await page.evaluate(value => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
      const item = Array.from(dialog?.querySelectorAll('.option-item') || []).find(el => el.textContent.trim() === value);
      if (item) item.click();
      return Boolean(item);
    }, keyword);
    if (recommended) continue;
    if (keyword.length > 12) throw new Error(`关键词“${keyword}”过长，51job 自定义关键词最多 12 字符，请缩短后重试`);
    await click(page, '.el-dialog .button-new-tag', '自定义关键词');
    await fill(page, '.el-dialog .custome-input input', keyword, '自定义关键词');
    const added = await page.evaluate(() => {
      const panel = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
      const button = Array.from(panel?.querySelectorAll('.custome-input span') || []).find(el => el.textContent.trim() === '添加');
      if (button) button.click();
      return Boolean(button);
    });
    if (!added) throw new Error(`无法添加关键词“${keyword}”`);
    await pause(150);
  }
  await click(page, '.keyword-footer_wrapper .confirm-btn', '关键词确认按钮');
}

async function chooseExperience(page, years) {
  const value = years === 0 ? '无需经验' : `${years}年`;
  const selectYear = () => page.evaluate(text => {
    const popover = document.querySelector('.eh_work_year_require .el-popover');
    const item = Array.from(popover?.querySelectorAll('.work-year-item') || []).find(el => el.textContent.trim() === text);
    if (item) item.click();
    return Boolean(item);
  }, value);
  let found = await selectYear();
  if (!found) {
    await click(page, '.eh_work_year_require button', '工作经验');
    found = await selectYear();
  }
  if (!found) throw new Error(`51job 没有“${value}”经验选项`);
  await pause(200);
}

async function fillForm(page, job) {
  await page.waitForFunction(() => document.querySelectorAll('.job_type_select .el-select-dropdown__item').length >= 2 && document.body.innerText.includes('保存'), { timeout: 20000 });
  await pause(1600);
  const state = await page.evaluate(() => ({ loggedIn: !document.body.innerText.includes('账号密码登录'), address: Boolean(document.querySelector('#job_work_location .address_label.is-checked')) }));
  if (!state.loggedIn) throw new Error('请先在 51job 浏览器中登录企业账号');
  if (!state.address) throw new Error('51job 账号尚未选定上班地址，请在平台页面设置后重试');
  await choose(page, '.job_type_select', '社会招聘', '职位类型');
  await fill(page, '[data-id="jobName"] input', job.title, '职位名称');
  await fill(page, '[data-id="jobInfo"] textarea', job.description, '职位描述');
  await chooseFunction(page, job.functionPath);
  await chooseKeywords(page, job.keywords);
  await chooseExperience(page, job.experienceYears);
  await choose(page, '.salary_type', '月薪', '薪资单位');
  await fill(page, 'input[placeholder="最低月薪"]', job.minSalary, '最低月薪');
  await fill(page, 'input[placeholder="最高月薪"]', job.maxSalary, '最高月薪');
  await fill(page, '[data-id="jobnum"] input', job.headcount, '招聘人数');
  await choose(page, '.salay_factor', `${job.salaryMonths}薪`, '薪资月数');
  await choose(page, 'input[placeholder="选择最低学历"]', job.education, '学历');
  return page.evaluate(() => ({
    title: document.querySelector('[data-id="jobName"] input')?.value,
    description: document.querySelector('[data-id="jobInfo"] textarea')?.value,
    functionName: document.querySelector('[data-id="funcTypeInput"] input')?.value,
    keywords: document.querySelector('#job-keywords')?.innerText || '',
    minSalary: document.querySelector('input[placeholder="最低月薪"]')?.value,
    maxSalary: document.querySelector('input[placeholder="最高月薪"]')?.value,
    headcount: document.querySelector('[data-id="jobnum"] input')?.value,
    education: document.querySelector('input[placeholder="选择最低学历"]')?.value,
    address: document.querySelector('#job_work_location .address_label.is-checked .address_name')?.innerText || '',
    city: document.querySelector('[for="formattedCitys"]')?.parentElement?.innerText || ''
  }));
}

async function run(raw, { dryRun = false } = {}) {
  const job = validate(raw);
  let browser;
  try { browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 }); }
  catch { return { status: 'needs_login', message: '前程无忧企业浏览器未启动，请先打开并登录 51job，再重试' }; }
  let page;
  let session;
  try {
    const pages = await browser.pages();
    const source = pages.find(item => item.url().includes('ehire.51job.com'));
    if (!source) return { status: 'needs_login', message: '未检测到已登录的 51job 企业页面，请先登录' };
    page = await createBackgroundPage(browser, source, JOB_URL);
    session = await keepPageActiveInBackground(page);
    const fields = await fillForm(page, job);
    if (fields.title !== job.title || fields.description !== job.description || fields.functionName !== job.functionPath.at(-1) || fields.minSalary !== String(job.minSalary) || fields.maxSalary !== String(job.maxSalary) || fields.headcount !== String(job.headcount) || fields.education !== job.education) {
      throw new Error('51job 表单回读与输入不一致，请在浏览器中核对');
    }
    if (dryRun) {
      await page.evaluate(() => Array.from(document.querySelectorAll('button')).find(el => el.textContent.trim() === '预览')?.click());
      await pause(700);
      const errors = await page.evaluate(() => Array.from(document.querySelectorAll('.el-form-item__error')).map(el => el.textContent.trim()).filter(Boolean));
      const preview = await page.evaluate(() => ({ url: location.href, dialogs: Array.from(document.querySelectorAll('.el-dialog')).filter(el => el.getBoundingClientRect().height > 0).map(el => el.innerText.slice(0, 120)) }));
      return { status: errors.length ? 'needs_review' : 'prepared', message: errors.length ? `预览校验失败：${errors.join('；')}` : '表单已填写且通过预览校验，未点击保存', fields, preview };
    }
    const saveClicked = await page.evaluate(() => {
      const button = Array.from(document.querySelectorAll('button, .el-button')).find(el => el.textContent.trim() === '保存' && el.getBoundingClientRect().height > 0);
      if (button) button.click();
      return Boolean(button);
    });
    if (!saveClicked) throw new Error('51job 页面未找到“保存”按钮，草稿未提交');
    let result;
    for (let attempt = 0; attempt < 20; attempt++) {
      await pause(350);
      result = await page.evaluate(() => ({
        url: location.href,
        errors: Array.from(document.querySelectorAll('.el-form-item__error')).map(el => el.textContent.trim()).filter(Boolean),
        notifications: Array.from(document.querySelectorAll('.el-message, .el-notification, .el-dialog')).filter(el => el.getBoundingClientRect().height > 0).map(el => el.innerText.trim()).filter(Boolean)
      }));
      if (result.errors.length || result.notifications.some(text => /保存成功|草稿.*成功|职位已保存/.test(text))) break;
    }
    if (result.errors.length) return { status: 'needs_review', message: `51job 未接受表单：${result.errors.join('；')}`, url: result.url };
    if (result.notifications.some(text => /保存成功|草稿.*成功|职位已保存/.test(text))) return { status: 'saved', message: '51job 已确认保存职位草稿', url: result.url };
    return { status: 'uncertain', message: '已向 51job 提交保存请求，但未收到明确成功回执；请到职位管理核对，暂勿重复提交', url: result.url };
  } catch (error) {
    return { status: 'needs_review', message: error.message, url: page?.url() || '' };
  } finally {
    if (session) await session.detach().catch(() => {});
    if (dryRun && page) await page.close().catch(() => {});
    await browser.disconnect().catch(() => {});
  }
}

if (require.main === module) {
  let input = '';
  process.stdin.setEncoding('utf8');
  process.stdin.on('data', chunk => { input += chunk; });
  process.stdin.on('end', async () => {
    try { process.stdout.write(JSON.stringify(await run(JSON.parse(input), { dryRun: process.argv.includes('--dry-run') })) + '\n'); }
    catch (error) { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); }
  });
}

module.exports = { validate, run };
