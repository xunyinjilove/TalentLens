/** 51job 企业端职位填写：复用已登录浏览器，不重复启动 Edge。 */
const puppeteer = require('puppeteer-core');
const { createBackgroundPage, keepPageActiveInBackground, setBrowserWindowState } = require('./multi_platform_agent');

const JOB_URL = 'https://ehire.51job.com/Revision/job?mark=new';
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

function validate(input) {
  const action = input.action || 'draft';
  const jobType = input.jobType || '社会招聘';
  if (!['draft', 'publish'].includes(action)) throw new Error('职位操作只能是保存草稿或立即发布');
  if (!['社会招聘', '校园招聘'].includes(jobType)) throw new Error('职位类型只能是社会招聘或校园招聘');
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
  return { action, jobType, title, description, functionPath, keywords, minSalary, maxSalary, salaryMonths, headcount, experienceYears, education: String(input.education).trim() };
}

function parseFunctionPath(value) {
  const path = String(value || '').split('>').map(item => item.trim()).filter(Boolean);
  if (path.length < 2) throw new Error('51job 职能路径需填写到末级，例如“互联网技术 > 测试 > 软件测试”');
  return path;
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
  for (let level = 0; level < path.length; level++) {
    const name = path[level];
    const found = await page.evaluate(({ value, level, last }) => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.innerText.includes('选择职能'));
      const menu = dialog?.querySelectorAll('.cascader_panel_menu')[level];
      const item = Array.from(menu?.querySelectorAll('.func-item') || []).find(el => el.title === value);
      if (item?.parentElement?.classList.contains('leaf') !== last) return false;
      if (item) item.click();
      return Boolean(item);
    }, { value: name, level, last: level === path.length - 1 });
    if (!found) throw new Error(`51job 职能路径第 ${level + 1} 级“${name}”不存在或不是末级，请重新选择`);
    await pause(200);
  }
  const selected = await page.evaluate(() => document.querySelector('[data-id="funcTypeInput"] input')?.value || '');
  if (selected !== path[path.length - 1]) throw new Error('51job 职能未选中，请在平台页面检查');
}

// 按已选上级读取下一列；只返回当前页面实际展示的职能，不缓存猜测分类。
async function getFunctionOptions(raw) {
  const path = Array.isArray(raw.path) ? raw.path.map(item => String(item).trim()) : [];
  if (path.length > 2 || path.some(item => !item)) throw new Error('职能层级无效');
  let browser;
  try { browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 }); }
  catch { return { status: 'needs_login', message: '请先打开并登录 51job 企业浏览器', options: [] }; }
  let page;
  let session;
  try {
    const source = (await browser.pages()).find(item => item.url().includes('ehire.51job.com'));
    if (!source) return { status: 'needs_login', message: '未检测到 51job 企业页面', options: [] };
    page = await createBackgroundPage(browser, source, JOB_URL);
    session = await keepPageActiveInBackground(page);
    await page.waitForSelector('.func-dropdown .all_func_tips', { timeout: 20000 });
    await click(page, '.func-dropdown .all_func_tips', '职能入口');
    await page.waitForFunction(() => Array.from(document.querySelectorAll('.el-dialog')).some(el => el.getBoundingClientRect().height > 0 && el.innerText.includes('选择职能') && el.querySelector('.cascader_panel_menu')), { timeout: 5000 });
    for (let level = 0; level < path.length; level++) {
      const selection = await page.evaluate(({ level, value }) => {
        const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.innerText.includes('选择职能'));
        const menu = dialog?.querySelectorAll('.cascader_panel_menu')[level];
        const item = Array.from(menu?.querySelectorAll('.func-item') || []).find(el => el.title === value && !el.parentElement?.classList.contains('leaf'));
        if (!item) return { found: false };
        const nextMenu = dialog?.querySelectorAll('.cascader_panel_menu')[level + 1];
        const before = Array.from(nextMenu?.querySelectorAll('.func-item') || []).map(el => el.title).join('|');
        const wasActive = item.parentElement?.classList.contains('active');
        item?.click();
        return { found: true, before, wasActive };
      }, { level, value: path[level] });
      if (!selection.found) throw new Error(`51job 职能“${path[level]}”已变化，请重新选择`);
      if (!selection.wasActive) {
        await page.waitForFunction(({ nextLevel, before }) => {
          const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.innerText.includes('选择职能'));
          const menu = dialog?.querySelectorAll('.cascader_panel_menu')[nextLevel];
          const current = Array.from(menu?.querySelectorAll('.func-item') || []).map(el => el.title).join('|');
          return Boolean(current) && current !== before;
        }, { timeout: 5000 }, { nextLevel: level + 1, before: selection.before });
      }
    }
    const options = await page.evaluate(level => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.innerText.includes('选择职能'));
      const menu = dialog?.querySelectorAll('.cascader_panel_menu')[level];
      return Array.from(menu?.querySelectorAll('.func-item') || []).map(item => ({
        name: item.title,
        leaf: item.parentElement?.classList.contains('leaf') || false
      })).filter(item => item.name);
    }, path.length);
    if (!options.length) throw new Error('51job 未返回这一层的职能，请在平台页面检查');
    return { status: 'ready', message: `已读取 51job 第 ${path.length + 1} 级职能`, options };
  } catch (error) {
    return { status: 'error', message: error.message, options: [] };
  } finally {
    if (session) await session.detach().catch(() => {});
    if (page) await page.close().catch(() => {});
    await browser.disconnect().catch(() => {});
  }
}

// “查看页面”由用户主动触发：恢复专用浏览器窗口，且不新建后台抓取标签页。
async function show51JobPage() {
  let browser;
  try { browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 }); }
  catch { return { status: 'needs_login', message: '51job 专用浏览器已关闭' }; }
  try {
    const pages = await browser.pages();
    const page = pages.find(item => item.url().includes('ehire.51job.com/Revision/job'))
      || pages.find(item => item.url().includes('ehire.51job.com'));
    if (!page) return { status: 'needs_login', message: '专用浏览器中没有 51job 页面' };
    const restored = await setBrowserWindowState(page, 'normal');
    if (!restored) return { status: 'error', message: '无法还原 51job 浏览器窗口' };
    await page.bringToFront();
    return { status: 'ready', message: '已显示 51job 专用浏览器', url: page.url() };
  } catch (error) {
    return { status: 'error', message: `无法显示 51job 浏览器：${error.message}` };
  } finally {
    await browser.disconnect().catch(() => {});
  }
}

async function chooseKeywords(page, keywords) {
  await click(page, '#job-keywords input', '关键词入口');
  const dialogVisible = () => Array.from(document.querySelectorAll('.el-dialog')).some(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
  try { await page.waitForFunction(dialogVisible, { timeout: 1800 }); }
  catch {
    await click(page, '#job-keywords input', '关键词入口');
    await page.waitForFunction(dialogVisible, { timeout: 5000 });
  }
  // 51job 可能根据描述自动预选词；先清理本次未选择的词，避免占满 10 个名额。
  for (let attempt = 0; attempt < 15; attempt++) {
    const removed = await page.evaluate(expected => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
      const tag = Array.from(dialog?.querySelectorAll('#_selectedKeywordsRef .el-tag') || []).find(el => !expected.includes(el.textContent.trim()));
      if (tag) tag.querySelector('.el-tag__close')?.click();
      return Boolean(tag);
    }, keywords);
    if (!removed) break;
    await pause(100);
  }
  for (const keyword of keywords) {
    const alreadySelected = await page.evaluate(value => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
      return Array.from(dialog?.querySelectorAll('#_selectedKeywordsRef .el-tag') || []).some(el => el.textContent.trim() === value);
    }, keyword);
    if (alreadySelected) continue;
    const recommended = await page.evaluate(value => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
      const item = Array.from(dialog?.querySelectorAll('.option-item') || []).find(el => el.textContent.trim() === value);
      if (item) item.click();
      return Boolean(item);
    }, keyword);
    if (recommended) { await pause(120); continue; }
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
  const selected = await page.evaluate(() => {
    const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
    return Array.from(dialog?.querySelectorAll('#_selectedKeywordsRef .el-tag') || []).map(el => el.textContent.trim());
  });
  if (selected.length !== keywords.length || keywords.some(keyword => !selected.includes(keyword))) {
    throw new Error(`51job 关键词选择不一致：期望 ${keywords.join('、')}，实际 ${selected.join('、') || '无'}`);
  }
  const confirmed = await page.evaluate(() => {
    const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
    const button = dialog?.querySelector('.keyword-footer_wrapper .confirm-btn');
    if (button) button.click();
    return Boolean(button);
  });
  if (!confirmed) throw new Error('51job 关键词确认按钮不可用');
}

async function getKeywordSuggestions(raw) {
  const functionPath = parseFunctionPath(raw.functionPath);
  let browser;
  try { browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 }); }
  catch { return { status: 'needs_login', message: '请先打开并登录 51job 企业浏览器', groups: [] }; }
  let page;
  let session;
  try {
    const source = (await browser.pages()).find(item => item.url().includes('ehire.51job.com'));
    if (!source) return { status: 'needs_login', message: '未检测到 51job 企业页面', groups: [] };
    page = await createBackgroundPage(browser, source, JOB_URL);
    session = await keepPageActiveInBackground(page);
    await page.waitForFunction(() => document.querySelector('.func-dropdown .all_func_tips'), { timeout: 20000 });
    await pause(1200);
    await chooseFunction(page, functionPath);
    await click(page, '#job-keywords input', '关键词入口');
    const visible = () => Array.from(document.querySelectorAll('.el-dialog')).some(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
    try { await page.waitForFunction(visible, { timeout: 1800 }); }
    catch {
      await click(page, '#job-keywords input', '关键词入口');
      await page.waitForFunction(visible, { timeout: 5000 });
    }
    const groups = await page.evaluate(() => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.keyword-footer_wrapper'));
      return Array.from(dialog?.querySelectorAll('.option-wrapper[nav-code]') || [])
        .filter(el => el.getAttribute('nav-code') !== 'other_kw')
        .map(el => ({
          name: dialog.querySelector(`.lfnav-item[code="${el.getAttribute('nav-code')}"]`)?.textContent.trim() || el.getAttribute('nav-code'),
          keywords: Array.from(el.querySelectorAll('.option-item')).map(item => item.textContent.trim()).filter(Boolean)
        })).filter(group => group.keywords.length);
    });
    if (!groups.length) return { status: 'error', message: '51job 未返回该职能的推荐关键词，请核对职能路径', groups: [] };
    return { status: 'ready', message: `已读取 ${functionPath.at(-1)} 的 51job 推荐关键词`, groups };
  } catch (error) {
    return { status: 'error', message: error.message, groups: [] };
  } finally {
    if (session) await session.detach().catch(() => {});
    if (page) await page.close().catch(() => {});
    await browser.disconnect().catch(() => {});
  }
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
  await choose(page, '.job_type_select', job.jobType, '职位类型');
  await fill(page, '[data-id="jobName"] input', job.title, '职位名称');
  await fill(page, '[data-id="jobInfo"] textarea', job.description, '职位描述');
  await chooseFunction(page, job.functionPath);
  await chooseKeywords(page, job.keywords);
  if (job.jobType === '社会招聘') await chooseExperience(page, job.experienceYears);
  await choose(page, '.salary_type', '月薪', '薪资单位');
  await fill(page, 'input[placeholder="最低月薪"]', job.minSalary, '最低月薪');
  await fill(page, 'input[placeholder="最高月薪"]', job.maxSalary, '最高月薪');
  await fill(page, '[data-id="jobnum"] input', job.headcount, '招聘人数');
  await choose(page, '.salay_factor', `${job.salaryMonths}薪`, '薪资月数');
  await choose(page, 'input[placeholder="选择最低学历"]', job.education, '学历');
  return page.evaluate(() => ({
    jobType: document.querySelector('.job_type_select input')?.value,
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
  let submitted = false;
  try {
    const pages = await browser.pages();
    const source = pages.find(item => item.url().includes('ehire.51job.com'));
    if (!source) return { status: 'needs_login', message: '未检测到已登录的 51job 企业页面，请先登录' };
    page = await createBackgroundPage(browser, source, JOB_URL);
    session = await keepPageActiveInBackground(page);
    const fields = await fillForm(page, job);
    if (fields.jobType !== job.jobType || fields.title !== job.title || fields.description !== job.description || fields.functionName !== job.functionPath.at(-1) || fields.minSalary !== String(job.minSalary) || fields.maxSalary !== String(job.maxSalary) || fields.headcount !== String(job.headcount) || fields.education !== job.education) {
      throw new Error('51job 表单回读与输入不一致，请在浏览器中核对');
    }
    if (dryRun) {
      await page.evaluate(() => Array.from(document.querySelectorAll('button')).find(el => el.textContent.trim() === '预览')?.click());
      await pause(700);
      const errors = await page.evaluate(() => Array.from(document.querySelectorAll('.el-form-item__error')).map(el => el.textContent.trim()).filter(Boolean));
      const preview = await page.evaluate(() => ({ url: location.href, dialogs: Array.from(document.querySelectorAll('.el-dialog')).filter(el => el.getBoundingClientRect().height > 0).map(el => el.innerText.slice(0, 120)) }));
      return { status: errors.length ? 'needs_review' : 'prepared', message: errors.length ? `预览校验失败：${errors.join('；')}` : '表单已填写且通过预览校验，未点击保存', fields, preview };
    }
    const label = job.action === 'publish' ? '立即发布' : '保存';
    const buttonExists = await page.evaluate(value => Array.from(document.querySelectorAll('button, .el-button')).some(el => el.textContent.trim() === value && el.getBoundingClientRect().height > 0), label);
    if (!buttonExists) throw new Error(`51job 页面未找到“${label}”按钮，职位未提交`);
    // 点击可能立即跳转，先标记“结果待核验”，避免导航中断时错误地允许重复提交。
    submitted = true;
    await page.evaluate(value => {
      Array.from(document.querySelectorAll('button, .el-button')).find(el => el.textContent.trim() === value && el.getBoundingClientRect().height > 0)?.click();
    }, label);
    const successPattern = job.action === 'publish' ? /发布(?:职位)?成功(?!后)|职位已发布/ : /保存成功|草稿.*成功|职位已保存/;
    let result;
    for (let attempt = 0; attempt < 40; attempt++) {
      await pause(350);
      result = await page.evaluate(() => ({
        url: location.href,
        errors: Array.from(document.querySelectorAll('.el-form-item__error')).map(el => el.textContent.trim()).filter(Boolean),
        notifications: Array.from(document.querySelectorAll('.el-message, .el-notification, .el-dialog')).filter(el => el.getBoundingClientRect().height > 0).map(el => el.innerText.trim()).filter(Boolean)
      }));
      if (result.errors.length || result.notifications.some(text => successPattern.test(text))) break;
    }
    if (result.errors.length) return { status: 'needs_review', message: `51job 未接受表单：${result.errors.join('；')}`, url: result.url };
    if (result.notifications.some(text => successPattern.test(text))) {
      return job.action === 'publish'
        ? { status: 'published', message: '51job 已确认职位发布成功', url: result.url }
        : { status: 'saved', message: '51job 已确认保存职位草稿', url: result.url };
    }
    return { status: 'uncertain', message: `已点击“${label}”，但未收到 51job 明确回执；请到职位管理核对，暂勿重复提交`, url: result.url };
  } catch (error) {
    const detail = /detached Frame/i.test(error.message)
      ? '51job 页面在提交后跳转，自动化连接已失效'
      : error.message;
    return submitted
      ? { status: 'uncertain', message: `已尝试向 51job 提交，但后续核验中断：${detail}；请到职位管理核对，暂勿重复提交`, url: page?.url() || '' }
      : { status: 'needs_review', message: detail, url: page?.url() || '' };
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
    try {
      const data = JSON.parse(input);
      const result = process.argv.includes('--show-page')
        ? await show51JobPage()
        : process.argv.includes('--function-options')
          ? await getFunctionOptions(data)
        : process.argv.includes('--suggest-keywords')
          ? await getKeywordSuggestions(data)
          : await run(data, { dryRun: process.argv.includes('--dry-run') });
      process.stdout.write(JSON.stringify(result) + '\n');
    }
    catch (error) { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); }
  });
}

module.exports = { validate, getFunctionOptions, getKeywordSuggestions, show51JobPage, run };
