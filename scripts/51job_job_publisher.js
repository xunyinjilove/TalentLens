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
  const address = String(input.address || '').trim();
  const language = String(input.language || '').trim();
  const languageLevel = String(input.languageLevel || '').trim();
  const certificates = (Array.isArray(input.certificates) ? input.certificates : []).map(item => ({
    category: String(item.category || '').trim(), name: String(item.name || '').trim()
  }));
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
  if (Boolean(language) !== Boolean(languageLevel)) throw new Error('选择语言时还需选择熟练程度');
  if (certificates.length > 10 || certificates.some(item => !item.category || !item.name)) throw new Error('证书最多选择 10 项，且须从 51job 证书分类中选择');
  if (new Set(certificates.map(item => item.name)).size !== certificates.length) throw new Error('证书不能重复选择');
  return { action, jobType, title, description, functionPath, keywords, minSalary, maxSalary, salaryMonths, headcount, experienceYears, education: String(input.education).trim(), address, language, languageLevel, certificates };
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

async function waitForJobForm(page) {
  await page.waitForFunction(() =>
    Boolean(document.querySelector('.func-dropdown .all_func_tips'))
    || /账号密码登录|扫码登录|企业账号登录/.test(document.body?.innerText || ''),
  { timeout: 20000 });
  return Boolean(await page.$('.func-dropdown .all_func_tips'));
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
    if (!await waitForJobForm(page)) return { status: 'needs_login', message: '请在打开的 51job 企业浏览器完成登录，再点击重试加载', options: [] };
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
      || pages.find(item => item.url().includes('ehire.51job.com'))
      || pages.find(item => {
        try { return new URL(item.url()).hostname.endsWith('.51job.com'); }
        catch { return false; }
      });
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

// 重新加载职位表单验证服务端会话；此步骤只读，不改变窗口状态。
async function check51JobLogin() {
  let browser;
  try { browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 }); }
  catch { return { status: 'waiting', message: '等待 51job 专用浏览器启动' }; }
  try {
    const pages = await browser.pages();
    let page = pages.filter(item => item.url().includes('ehire.51job.com/Revision/job')).at(-1);
    if (!page) {
      const source = pages.filter(item => item.url().includes('ehire.51job.com/Revision/')).at(-1);
      if (source) {
        const loggedInShell = await source.evaluate(() =>
          !/账号密码登录|扫码登录|企业账号登录/.test(document.body?.innerText || '')
          && /职位管理|我的工作台|人才管理/.test(document.body?.innerText || '')).catch(() => false);
        if (loggedInShell) {
          await source.goto(JOB_URL, { waitUntil: 'domcontentloaded', timeout: 12000 });
          page = source;
        }
      }
    }
    if (!page) return { status: 'waiting', message: '请在 51job 浏览器完成登录' };
    const visible = await page.evaluate(() => {
      const el = document.querySelector('.func-dropdown .all_func_tips');
      return Boolean(el && el.getBoundingClientRect().height > 0)
        && !/账号密码登录|扫码登录|企业账号登录/.test(document.body?.innerText || '');
    }).catch(() => false);
    if (!visible) return { status: 'waiting', message: '请在 51job 浏览器完成登录' };
    // 重新加载验证服务端会话，避免把登录前残留的表单 DOM 误判为已登录。
    await page.reload({ waitUntil: 'domcontentloaded', timeout: 12000 });
    await page.waitForFunction(() => {
      const el = document.querySelector('.func-dropdown .all_func_tips');
      return Boolean(el && el.getBoundingClientRect().height > 0)
        && !/账号密码登录|扫码登录|企业账号登录/.test(document.body?.innerText || '');
    }, { timeout: 8000 });
    return { status: 'ready', message: '51job 登录已确认' };
  } catch (error) {
    return { status: 'waiting', message: `等待 51job 登录完成：${error.message}` };
  } finally {
    await browser.disconnect().catch(() => {});
  }
}

async function minimize51JobBrowser() {
  let browser;
  try { browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 }); }
  catch { return { status: 'error', message: '51job 专用浏览器已关闭，无法最小化' }; }
  try {
    const pages = await browser.pages();
    const page = pages.filter(item => item.url().includes('ehire.51job.com/Revision/job')).at(-1);
    if (!page) return { status: 'error', message: '未找到已登录的 51job 职位页面' };
    const ready = await page.evaluate(() => Boolean(document.querySelector('.func-dropdown .all_func_tips'))
      && !/账号密码登录|扫码登录|企业账号登录/.test(document.body?.innerText || ''));
    if (!ready) return { status: 'error', message: '51job 职位页面尚未就绪，未最小化浏览器' };
    const minimized = await setBrowserWindowState(page, 'minimized');
    return minimized
      ? { status: 'ready', message: '51job 已登录，专用浏览器已最小化' }
      : { status: 'error', message: '51job 已登录，但无法自动最小化浏览器窗口' };
  } catch (error) {
    return { status: 'error', message: `无法最小化 51job 浏览器：${error.message}` };
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
    if (!await waitForJobForm(page)) return { status: 'needs_login', message: '请在打开的 51job 企业浏览器完成登录，再点击重试加载', groups: [] };
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

// 选项始终从当前已登录的企业表单读取；地址搜索由 51job 自己过滤，避免内置过时的地址表。
async function getPostingOptions(raw) {
  const kind = String(raw.kind || '');
  if (!['address', 'language', 'certificate'].includes(kind)) throw new Error('职位选项类型无效');
  let browser, page, session;
  try { browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 }); }
  catch { return { status: 'needs_login', message: '请先打开并登录 51job 企业浏览器', options: [] }; }
  try {
    const source = (await browser.pages()).find(item => item.url().includes('ehire.51job.com'));
    if (!source) return { status: 'needs_login', message: '未检测到 51job 企业页面', options: [] };
    page = await createBackgroundPage(browser, source, JOB_URL);
    session = await keepPageActiveInBackground(page);
    if (!await waitForJobForm(page)) return { status: 'needs_login', message: '请先登录 51job 企业浏览器', options: [] };
    await pause(1600);
    const base = await page.evaluate(() => ({
      currentAddress: document.querySelector('#job_work_location .address_label.is-checked .address_name')?.textContent.trim() || '',
      currentCity: document.querySelector('[for="formattedCitys"]')?.parentElement?.textContent.trim() || ''
    }));
    if (kind === 'language') {
      const language = String(raw.category || '').trim();
      const options = await page.evaluate(() => Array.from(document.querySelectorAll('.language_form .el-select:first-child .el-select-dropdown__item')).map(item => item.textContent.trim()).filter(text => text && !text.startsWith('请选择')));
      if (!options.length) throw new Error('51job 语言选项尚未加载，请重试');
      if (language) {
        if (!options.includes(language)) throw new Error('51job 语言选项已变化，请重新加载');
        await click(page, '.language_form .el-select:first-child input', '语言');
        const selected = await page.evaluate(value => {
          const item = Array.from(document.querySelectorAll('.el-select-dropdown__item')).find(el => el.textContent.trim() === value && !el.classList.contains('is-disabled'));
          if (item) item.click();
          return Boolean(item);
        }, language);
        if (!selected) throw new Error('51job 语言选择器未就绪，请重试');
        await pause(200);
      }
      const levels = await page.evaluate(() => Array.from(document.querySelectorAll('.language_form .el-select[prop="flevel1"] .el-select-dropdown__item'))
        .filter(el => !el.classList.contains('is-disabled')).map(el => el.textContent.trim()).filter(text => text && !text.startsWith('请选择')));
      return { status: 'ready', message: language ? `已读取“${language}”可选的熟练程度` : '已读取 51job 语言，请选择语种', options, levels, ...base };
    }
    if (kind === 'address') {
      await click(page, '#sensor_new_addressreplace', '更换地址入口');
      const query = String(raw.query || '').trim().slice(0, 80);
      if (query) {
        await fill(page, '.address_filter_popover input[placeholder="请输入所在地区"]', query, '地址搜索');
        await pause(450);
      }
      const options = await page.evaluate(() => {
        const pop = Array.from(document.querySelectorAll('.address_filter_popover')).find(el => el.getBoundingClientRect().height > 0);
        return Array.from(pop?.querySelectorAll('.user_info .name') || []).map(el => el.getAttribute('title') || el.textContent.trim()).filter(Boolean);
      });
      return { status: 'ready', message: query ? `已读取与“${query}”匹配的 51job 已有地址` : '已读取 51job 当前可见的已有地址；可输入城市或地址搜索更多', options: [...new Set(options)], ...base };
    }
    await click(page, '[data-id="certificateInput"] input', '证书入口');
    await page.waitForFunction(() => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
      return Boolean(dialog?.querySelector('.cascader_panel_menu .func-item'));
    }, { timeout: 5000 });
    const category = String(raw.category || '').trim();
    const choices = await page.evaluate(value => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
      const menus = dialog?.querySelectorAll('.cascader_panel_menu') || [];
      const categories = Array.from(menus[0]?.querySelectorAll('.func-item') || []).map(el => el.title || el.textContent.trim());
      if (value) {
        const item = Array.from(menus[0]?.querySelectorAll('.func-item') || []).find(el => (el.title || el.textContent.trim()) === value);
        if (!item) return { categories, missing: true };
        item.click();
      }
      return { categories, missing: false };
    }, category);
    if (choices.missing) throw new Error('51job 证书分类已变化，请重新加载');
    await pause(200);
    const options = await page.evaluate(() => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
      return Array.from(dialog?.querySelectorAll('.cascader_panel_menu:nth-child(2) .func-item') || []).map(el => el.title || el.textContent.trim());
    });
    return { status: 'ready', message: '已读取 51job 证书目录', categories: choices.categories, options, ...base };
  } catch (error) { return { status: 'error', message: error.message, options: [] }; }
  finally {
    if (session) await session.detach().catch(() => {});
    if (page) await page.close().catch(() => {});
    await browser.disconnect().catch(() => {});
  }
}

async function chooseAddress(page, address) {
  if (!address) return;
  await click(page, '#sensor_new_addressreplace', '更换地址入口');
  await fill(page, '.address_filter_popover input[placeholder="请输入所在地区"]', address, '地址搜索');
  await pause(450);
  const selected = await page.evaluate(value => {
    const pop = Array.from(document.querySelectorAll('.address_filter_popover')).find(el => el.getBoundingClientRect().height > 0);
    const matches = Array.from(pop?.querySelectorAll('.user_info') || []).filter(el => (el.querySelector('.name')?.getAttribute('title') || el.querySelector('.name')?.textContent.trim()) === value);
    if (matches.length !== 1) return matches.length;
    matches[0].click();
    return 1;
  }, address);
  if (selected !== 1) throw new Error(`51job 已有地址匹配到 ${selected} 条，请重新选择唯一的上班地址`);
  await pause(250);
  const actual = await page.evaluate(() => document.querySelector('#job_work_location .address_label.is-checked .address_name')?.textContent.trim() || '');
  if (actual !== address) throw new Error('51job 上班地址回读不一致，职位未提交');
}

async function chooseLanguage(page, language, level) {
  if (!language) return;
  const chooseVisible = async (value, label) => {
    let done = false;
    for (let attempt = 0; attempt < 10 && !done; attempt++) {
      done = await page.evaluate(target => {
        const items = Array.from(document.querySelectorAll('.el-select-dropdown')).filter(el => el.style.display !== 'none')
          .flatMap(el => Array.from(el.querySelectorAll('.el-select-dropdown__item')));
        const matches = items.filter(el => el.textContent.trim() === target && !el.classList.contains('is-disabled'));
        if (matches.length !== 1) return false;
        matches[0].click();
        return true;
      }, value);
      if (!done) await pause(100);
    }
    if (!done) throw new Error(`51job 没有“${value}”${label}选项`);
    await pause(150);
  };
  await click(page, '.language_form .el-select:first-child input', '语言');
  await chooseVisible(language, '语言');
  await click(page, '.language_form .el-select[prop="flevel1"] input', '语言程度');
  await chooseVisible(level, '语言程度');
  await pause(450);
  const actual = await page.evaluate(() => Array.from(document.querySelectorAll('.language_form .el-select')).slice(0, 2).map(el => el.querySelector('input')?.value || ''));
  if (actual[0] !== language || actual[1] !== level) throw new Error('51job 语言选择回读不一致，职位未提交');
}

async function chooseCertificates(page, certificates) {
  if (!certificates.length) return;
  await click(page, '[data-id="certificateInput"] input', '证书入口');
  await page.waitForFunction(() => Array.from(document.querySelectorAll('.el-dialog')).some(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书' && el.querySelector('.cascader_panel_menu .func-item')), { timeout: 5000 });
  for (const certificate of certificates) {
    const found = await page.evaluate(({ category, name }) => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
      const categoryItem = Array.from(dialog?.querySelectorAll('.cascader_panel_menu')[0]?.querySelectorAll('.func-item') || []).find(el => (el.title || el.textContent.trim()) === category);
      if (!categoryItem) return false;
      categoryItem.click();
      return true;
    }, certificate);
    if (!found) throw new Error(`51job 证书分类“${certificate.category}”不存在`);
    await page.waitForFunction(name => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
      return Array.from(dialog?.querySelectorAll('.cascader_panel_menu:nth-child(2) .func-item') || []).some(el => (el.title || el.textContent.trim()) === name);
    }, { timeout: 3000 }, certificate.name).catch(() => { throw new Error(`51job 证书“${certificate.name}”已不在该分类中`); });
    const selected = await page.evaluate(name => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
      const item = Array.from(dialog?.querySelectorAll('.cascader_panel_menu:nth-child(2) .func-item') || []).find(el => (el.title || el.textContent.trim()) === name);
      if (!item) return false;
      item.click();
      return true;
    }, certificate.name);
    if (!selected) throw new Error(`51job 证书“${certificate.name}”已不在该分类中`);
    await pause(120);
  }
  const selected = await page.evaluate(() => {
    const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
    return Array.from(dialog?.querySelectorAll('#_selectedFunctypeListRef .el-tag') || []).map(el => el.textContent.trim());
  });
  if (selected.length !== certificates.length || certificates.some(item => !selected.includes(item.name))) throw new Error('51job 证书选择回读不一致，职位未提交');
  const confirmed = await page.evaluate(() => {
    const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
    const button = dialog?.querySelector('.confirm_button');
    if (button) button.click();
    return Boolean(button);
  });
  if (!confirmed) throw new Error('51job 证书确认按钮不可用');
  await pause(150);
  const actual = await page.evaluate(() => Array.from(document.querySelectorAll('[data-id="certificate"] .el-tag')).map(el => el.textContent.trim()));
  if (actual.length !== certificates.length || certificates.some(item => !actual.includes(item.name))) throw new Error('51job 证书表单回读不一致，职位未提交');
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
  if (!state.address && !job.address) throw new Error('51job 账号尚未选定上班地址，请先选择企业账号已有地址');
  await chooseAddress(page, job.address);
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
  await chooseLanguage(page, job.language, job.languageLevel);
  await chooseCertificates(page, job.certificates);
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
    city: document.querySelector('[for="formattedCitys"]')?.parentElement?.innerText || '',
    language: (document.querySelector('.language_form .el-select:first-child input')?.value || '').replace(/^请选择$/, ''),
    languageLevel: (document.querySelector('.language_form .el-select[prop="flevel1"] input')?.value || '').replace(/^请选择程度$/, ''),
    certificates: Array.from(document.querySelectorAll('[data-id="certificate"] .el-tag')).map(el => el.textContent.trim())
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
    if (fields.jobType !== job.jobType || fields.title !== job.title || fields.description !== job.description || fields.functionName !== job.functionPath.at(-1) || fields.minSalary !== String(job.minSalary) || fields.maxSalary !== String(job.maxSalary) || fields.headcount !== String(job.headcount) || fields.education !== job.education || (job.address && fields.address.trim() !== job.address) || fields.language !== job.language || fields.languageLevel !== job.languageLevel || fields.certificates.length !== job.certificates.length || job.certificates.some(item => !fields.certificates.includes(item.name))) {
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
    let result = { url: page.url(), errors: [], notifications: [] };
    for (let attempt = 0; attempt < 40; attempt++) {
      await pause(350);
      try {
        result = await page.evaluate(() => ({
          url: location.href,
          errors: Array.from(document.querySelectorAll('.el-form-item__error')).map(el => el.textContent.trim()).filter(Boolean),
          notifications: Array.from(document.querySelectorAll('.el-message, .el-notification, .el-dialog')).filter(el => el.getBoundingClientRect().height > 0).map(el => el.innerText.trim()).filter(Boolean)
        }));
      } catch (error) {
        // 提交后页面跳转会暂时销毁旧 Frame；继续读取新页面，但绝不再次点击提交。
        if (/detached Frame|Execution context was destroyed|Cannot find context/i.test(error.message)) continue;
        throw error;
      }
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
      const result = process.argv.includes('--login-status')
        ? await check51JobLogin()
        : process.argv.includes('--minimize-page')
          ? await minimize51JobBrowser()
        : process.argv.includes('--show-page')
          ? await show51JobPage()
        : process.argv.includes('--function-options')
          ? await getFunctionOptions(data)
        : process.argv.includes('--suggest-keywords')
          ? await getKeywordSuggestions(data)
        : process.argv.includes('--posting-options')
          ? await getPostingOptions(data)
          : await run(data, { dryRun: process.argv.includes('--dry-run') });
      process.stdout.write(JSON.stringify(result) + '\n');
    }
    catch (error) { process.stdout.write(JSON.stringify({ status: 'error', message: error.message }) + '\n'); }
  });
}

module.exports = { validate, getFunctionOptions, getKeywordSuggestions, getPostingOptions, show51JobPage, check51JobLogin, minimize51JobBrowser, run };
