/**
 * multi_platform_agent.js - 4合1 多招聘平台（BOSS直聘、智联招聘、前程无忧、猎聘）企业端直连与自动化抓取引擎
 */

const fs = require('fs');
const path = require('path');
const { spawn, execSync } = require('child_process');
const puppeteer = require('puppeteer-core');

// 命令行参数解析
const args = process.argv.slice(2);
const options = {
  platforms: ['boss'], // 支持 boss, zhaopin, 51job, liepin
  keyword: '临床项目经理',
  city: '上海',
  exp: '3-5年',
  edu: '本科',
  count: 30,
  testLoginPlatform: '', // 单独测试某平台登录
  action: '',            // 'greet', 'ask_resume', 'exchange_wechat', 'mark_unfit'
  candidateName: '',
  candidateUrl: '',
  message: '',
  excludeFile: '',       // 跨批次排重凭据文件
  quotaMatrix: null,     // 全网同义词与配额调度矩阵
  autoAnalyze: true,
  dataDir: path.join(process.cwd(), 'data', 'candidates_multi')
};

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--platforms' && args[i + 1]) {
    options.platforms = args[++i].split(',').map(s => s.trim()).filter(Boolean);
  } else if (args[i] === '--keyword' && args[i + 1]) {
    options.keyword = args[++i];
  } else if (args[i] === '--city' && args[i + 1]) {
    options.city = args[++i];
  } else if (args[i] === '--exp' && args[i + 1]) {
    options.exp = args[++i];
  } else if (args[i] === '--edu' && args[i + 1]) {
    options.edu = args[++i];
  } else if (args[i] === '--count' && args[i + 1]) {
    options.count = parseInt(args[++i], 10) || 30;
  } else if (args[i] === '--test-login' && args[i + 1]) {
    options.testLoginPlatform = args[++i];
  } else if (args[i] === '--action' && args[i + 1]) {
    options.action = args[++i];
  } else if (args[i] === '--candidate-name' && args[i + 1]) {
    options.candidateName = args[++i];
  } else if (args[i] === '--candidate-url' && args[i + 1]) {
    options.candidateUrl = args[++i];
  } else if (args[i] === '--message' && args[i + 1]) {
    options.message = args[++i];
  } else if (args[i] === '--exclude-file' && args[i + 1]) {
    options.excludeFile = args[++i];
  } else if (args[i] === '--quota-matrix' && args[i + 1]) {
    try {
      options.quotaMatrix = JSON.parse(args[++i]);
    } catch (e) {
      options.quotaMatrix = null;
    }
  } else if (args[i] === '--auto-analyze' && args[i + 1]) {
    options.autoAnalyze = args[++i] === 'true';
  } else if (args[i] === '--data-dir' && args[i + 1]) {
    options.dataDir = args[++i];
  }
}

// 标准输出 JSON 协议
function sendMsg(type, payload = {}) {
  const json = JSON.stringify({ type, timestamp: Date.now(), ...payload });
  process.stdout.write(json + '\n');
}

// 寻找系统 Edge 或 Chrome
function findBrowserExecutable() {
  const candidates = [
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    (process.env.LOCALAPPDATA || '') + '\\Microsoft\\Edge SxS\\Application\\msedge.exe',
    (process.env.LOCALAPPDATA || '') + '\\Google\\Chrome\\Application\\chrome.exe'
  ];

  for (const p of candidates) {
    if (p && fs.existsSync(p)) return p;
  }
  return null;
}

// 平台配置定义 — 每个平台分配固定 debugPort 避免冲突
const PLATFORM_CONFIGS = {
  boss: {
    name: 'BOSS直聘',
    code: 'boss',
    icon: '🏢',
    loginUrl: 'https://www.zhipin.com/web/user/',
    homeUrl: 'https://www.zhipin.com/web/chat/index',
    profileFolder: 'boss_isolated_profile',
    debugPort: 9501
  },
  zhaopin: {
    name: '智联招聘',
    code: 'zhaopin',
    icon: '💼',
    loginUrl: 'https://passport.zhaopin.com/login',
    homeUrl: 'https://rd6.zhaopin.com/app/recommend',
    fallbackHomeUrl: 'https://ihr.zhaopin.com/',
    profileFolder: 'zhaopin_isolated_profile',
    debugPort: 9502
  },
  '51job': {
    name: '前程无忧',
    code: '51job',
    icon: '📑',
    loginUrl: 'https://ehire.51job.com/MainLogin.aspx',
    homeUrl: 'https://ehire.51job.com/Revision/talent/search',
    fallbackHomeUrl: 'https://ehire.51job.com/Revision/navigate/',
    profileFolder: '51job_isolated_profile',
    debugPort: 9503
  },
  liepin: {
    name: '猎聘网',
    code: 'liepin',
    icon: '🎯',
    loginUrl: 'https://lpt.liepin.com/user/login',
    homeUrl: 'https://lpt.liepin.com/recommend',
    fallbackHomeUrl: 'https://lpt.liepin.com/',
    profileFolder: 'liepin_isolated_profile',
    debugPort: 9504
  }
};

function isBrowserStartPage(url) {
  return !url || url === 'about:blank' || /^(?:edge|chrome):\/\/(?:newtab|new-tab-page)\/?(?:[?#].*)?$/i.test(url)
    || /^https?:\/\/ntp\.msn\.cn\/edge\/ntp(?:[/?#]|$)/i.test(url);
}

function browserLaunchArgs(cfg, profileDir, isLoginTest) {
  return [
    `--remote-debugging-port=${cfg.debugPort}`,
    `--user-data-dir=${profileDir}`,
    '--no-first-run',
    '--no-default-browser-check',
    isLoginTest ? '--start-maximized' : '--start-minimized',
    cfg.homeUrl
  ];
}

function isPlatformPageUrl(url, cfg) {
  try {
    const host = new URL(url).hostname.toLowerCase();
    const homeHost = new URL(cfg.homeUrl).hostname.toLowerCase();
    const domain = homeHost.split('.').slice(-2).join('.');
    return host === domain || host.endsWith(`.${domain}`);
  } catch (e) {
    return false;
  }
}

async function ensurePlatformPage(page, cfg) {
  if (isPlatformPageUrl(page.url(), cfg)) return { ok: true };
  let lastError = '';
  for (const url of [cfg.homeUrl, cfg.loginUrl]) {
    try {
      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
    } catch (error) {
      lastError = error.message;
    }
    if (isPlatformPageUrl(page.url(), cfg)) return { ok: true };
  }
  return { ok: false, reason: lastError || '浏览器仍停留在非招聘网站页面' };
}

async function submit51jobSearch(page, keyword) {
  if (typeof page.url === 'function' && !/ehire\.51job\.com\/Revision\/talent\/search/i.test(page.url())) {
    return { ok: false, reason: '当前页面不是前程无忧人才搜索页' };
  }
  try {
    await page.waitForSelector('.talent_search_head_right button.search_button', { timeout: 10000 });
  } catch (error) {
    return { ok: false, reason: '人才搜索页未加载出搜索按钮' };
  }
  const filled = await page.evaluate(targetKeyword => {
    const inputs = Array.from(document.querySelectorAll('input'));
    const input = document.querySelector('.talent_search_keywords_input input, .talent_search_keywords input') || inputs.find(item => {
      const placeholder = (item.placeholder || '').trim();
      return placeholder.includes('搜索职位名') || placeholder.includes('职位名') || placeholder.includes('关键词') || placeholder.includes('搜索');
    });
    if (!input) return false;
    input.focus();
    input.value = targetKeyword;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
    return true;
  }, keyword);
  if (!filled) return { ok: false, reason: '没有找到岗位关键词输入框' };

  for (let attempt = 0; attempt < 3; attempt++) {
    await new Promise(resolve => setTimeout(resolve, 500));
    const clicked = await page.evaluate(() => {
      // 外层 div 的 innerText 同样是“搜索”，只能点击真正的按钮。
      const button = document.querySelector('.talent_search_head_right button.search_button') ||
        Array.from(document.querySelectorAll('button')).find(item => (item.innerText || '').trim() === '搜索');
      if (!button || button.disabled) return false;
      button.click();
      return true;
    });
    if (!clicked) return { ok: false, reason: '没有找到可点击的搜索按钮' };

    // 单纯调用 click() 不等于平台已经开始查询；必须等初始提示消失或结果卡片出现。
    for (let poll = 0; poll < 10; poll++) {
      await new Promise(resolve => setTimeout(resolve, 500));
      const state = await page.evaluate(() => ({
        cards: document.querySelectorAll('.talent-search-container .card').length,
        waiting: (document.body?.innerText || '').includes('输入关键词搜索，寻找匹配人才')
      }));
      if (state.cards > 0 || !state.waiting) return { ok: true };
    }
  }
  return { ok: false, reason: '搜索按钮已点击，但页面仍显示“输入关键词搜索”，检索未生效' };
}

async function closeUnusedStartPages(browser, selectedPage) {
  const pages = await browser.pages();
  for (const page of pages) {
    if (page !== selectedPage && isBrowserStartPage(page.url())) {
      await page.close().catch(() => {});
    }
  }
}

// 跨批次与全局排重缓存
const seenCandidateKeys = new Set();
let safetyStopped = false;
const projectExcludedNames = new Set();
const projectExcludedUrls = new Set();

if (options.excludeFile && fs.existsSync(options.excludeFile)) {
  try {
    const raw = fs.readFileSync(options.excludeFile, 'utf8');
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed.names)) {
      parsed.names.forEach(n => {
        const c = (n || '').trim();
        if (c) {
          projectExcludedNames.add(c);
          seenCandidateKeys.add(c);
        }
      });
    }
    if (Array.isArray(parsed.urls)) {
      parsed.urls.forEach(u => {
        const c = (u || '').trim();
        if (c) projectExcludedUrls.add(c);
      });
    }
    if (Array.isArray(parsed.cards)) {
      parsed.cards.forEach(card => {
        if (card && card.name && (card.infoText || card.workText)) {
          seenCandidateKeys.add(candidateCardKey(card));
        }
      });
    }
  } catch (e) {}
}

function isDuplicateCandidate(name, company, exp) {
  const cleanName = (name || '').replace(/[\s\*]/g, '');
  const cleanComp = (company || '').replace(/[\s\(\)（）某]/g, '');
  const key = `${cleanName}_${cleanComp}_${(exp || '').replace(/\s/g, '')}`;
  if (seenCandidateKeys.has(key)) return true;
  seenCandidateKeys.add(key);
  return false;
}

function candidateCardKey(candidate) {
  if (candidate.url) return `url:${String(candidate.url).trim()}`;
  const clean = value => String(value || '')
    .replace(/已读|未读|刚刚活跃|今日活跃|3日内活跃|半年前活跃/g, '')
    .replace(/[\s\*()（）某]/g, '');
  return `${clean(candidate.name)}_${clean(candidate.workText)}_${clean(candidate.infoText)}`;
}

function selectFreshCandidates(candidates, limit, excludedUrls = new Set(), seenKeys = new Set()) {
  const selected = [];
  const batchKeys = new Set();
  for (const candidate of candidates || []) {
    const key = candidateCardKey(candidate);
    if (!candidate.name || seenKeys.has(key) || batchKeys.has(key)) continue;
    if (candidate.url && excludedUrls.has(candidate.url)) continue;
    candidate.cardKey = key;
    batchKeys.add(key);
    selected.push(candidate);
    if (selected.length >= limit) break;
  }
  return selected;
}

// 杀死占用指定 profile 目录的 Edge/Chrome 进程
function killBrowserByProfile(profileDir) {
  const normalizedDir = profileDir.replace(/\\/g, '\\\\');
  for (const procName of ['msedge.exe', 'chrome.exe']) {
    try {
      const cmd = `wmic process where "name='${procName}' and CommandLine like '%${normalizedDir}%'" call terminate 2>nul`;
      execSync(cmd, { stdio: 'ignore', timeout: 5000, windowsHide: true });
    } catch (e) {
      // 静默 — WMIC 在没有匹配进程时返回非零退出码
    }
  }
}

// 清除浏览器 profile 目录下的锁文件
function clearProfileLocks(profileDir) {
  for (const lockName of ['SingletonLock', 'SingletonSocket', 'SingletonCookie']) {
    try {
      const f = path.join(profileDir, lockName);
      if (fs.existsSync(f)) fs.unlinkSync(f);
    } catch (e) {}
  }
}

// 模拟真人分段随机打字（汲取 GoodHR 拟人输入设计）
async function humanType(page, selectorOrElement, text) {
  try {
    const el = typeof selectorOrElement === 'string' ? await page.$(selectorOrElement) : selectorOrElement;
    if (!el) return false;
    await el.click().catch(() => {});
    await page.keyboard.down('Control');
    await page.keyboard.press('KeyA');
    await page.keyboard.up('Control');
    await page.keyboard.press('Backspace');

    const chars = Array.from(String(text || ''));
    let offset = 0;
    while (offset < chars.length) {
      const chunkSize = Math.floor(Math.random() * 2) + 1; // 1~2 个汉字/字符
      const chunk = chars.slice(offset, offset + chunkSize).join('');
      const delay = Math.floor(Math.random() * 65) + 25; // 25~90ms 单字敲击延时
      await page.keyboard.type(chunk, { delay });
      offset += chunk.length;
      if (offset < chars.length) {
        const pause = Math.floor(Math.random() * 140) + 80; // 80~220ms 拟人思考停顿
        await new Promise(r => setTimeout(r, pause));
      }
    }
    return true;
  } catch (e) {
    return false;
  }
}

// 分段滚动页面内的真实列表容器；最小化窗口时 CDP 鼠标滚轮可能一直等待绘制确认。
async function smoothScroll(page, distance = 480, step = 80) {
  let scrolled = 0;
  const dir = distance > 0 ? 1 : -1;
  const absDist = Math.abs(distance);
  while (scrolled < absDist) {
    const currentStep = Math.min(step, absDist - scrolled);
    const delta = currentStep * dir;
    const frames = typeof page.frames === 'function' ? page.frames() : [page];
    let foundList = false;
    for (const frame of frames) {
      const result = await frame.evaluate(amount => {
        const card = document.querySelector('.talent-search-container .card, .eh-talent-search .card, .candidate-card-wrap, .geek-item, .res-list tr, .candidate-box, .resume-item, .talent-item');
        if (!card) return false;
        let target = card.parentElement;
        while (target && target !== document.body && target.scrollHeight <= target.clientHeight + 100) target = target.parentElement;
        if (!target || target === document.body) target = document.scrollingElement || document.documentElement;
        target.scrollTop += amount;
        return true;
      }, delta).catch(() => false);
      if (result) {
        foundList = true;
        break;
      }
    }
    if (!foundList) {
      await page.evaluate(amount => {
        const target = document.scrollingElement || document.documentElement;
        if (target) target.scrollTop += amount;
      }, delta).catch(() => {});
    }
    scrolled += currentStep;
    await new Promise(r => setTimeout(r, 40 + Math.floor(Math.random() * 50)));
  }
}

async function keepPageActiveInBackground(page) {
  let session = null;
  try {
    session = await page.target().createCDPSession();
    await session.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    return session;
  } catch (e) {
    if (session) await session.detach().catch(() => {});
    return null;
  }
}

// 基于 Box-Muller 变换的高斯正态分布拟人随机停顿（模拟真实人类心智呼吸停顿）
function gaussianRandom(mean = 2500, stdev = 800, min = 1200, max = 4500) {
  let u = 0, v = 0;
  while (u === 0) u = Math.random();
  while (v === 0) v = Math.random();
  const z = Math.sqrt(-2.0 * Math.log(u)) * Math.cos(2.0 * Math.PI * v);
  const val = mean + z * stdev;
  return Math.max(min, Math.min(max, Math.round(val)));
}

function allocateQuota(total, items) {
  const weights = items.map(item => Math.max(0, Number(item.ratio) || 0));
  const sum = weights.reduce((a, b) => a + b, 0);
  if (!sum || total <= 0) return items.map(() => 0);
  const exact = weights.map(weight => total * weight / sum);
  const counts = exact.map(Math.floor);
  let left = total - counts.reduce((a, b) => a + b, 0);
  const order = exact.map((value, index) => ({ index, fraction: value - counts[index] }))
    .sort((a, b) => b.fraction - a.fraction || a.index - b.index);
  for (let i = 0; i < left; i++) counts[order[i].index]++;
  return counts;
}

// 设置浏览器窗口状态（'minimized' 最小化至任务栏沙盒 | 'normal' 正常还原窗口 | 'maximized' 最大化）
async function setBrowserWindowState(browserOrPage, state = 'minimized') {
  for (let attempt = 0; attempt < 3; attempt++) {
    let session = null;
    try {
      let target = null;
      if (browserOrPage && typeof browserOrPage.pages === 'function') {
        const pages = await browserOrPage.pages();
        if (pages && pages.length > 0) target = pages[0].target();
      } else if (browserOrPage && typeof browserOrPage.target === 'function') {
        target = browserOrPage.target();
      }
      if (!target) return false;
      session = await target.createCDPSession();
      const { windowId } = await session.send('Browser.getWindowForTarget');
      if (windowId !== undefined) {
        await session.send('Browser.setWindowBounds', { windowId, bounds: { windowState: state } });
        return true;
      }
    } catch (e) {
      // 新标签页可能暂时没有窗口 ID，等待后重试。
    } finally {
      if (session) await session.detach().catch(() => {});
    }
    await new Promise(resolve => setTimeout(resolve, 120));
  }
  return false;
}

// CDP 在已有 Edge 窗口内创建后台标签页；browser.newPage() 会激活窗口并抢走 HR 的焦点。
async function createBackgroundPage(browser, sourcePage, url) {
  const session = await sourcePage.target().createCDPSession();
  let targetId;
  try {
    ({ targetId } = await session.send('Target.createTarget', {
      url: new URL(url, sourcePage.url()).href,
      background: true
    }));
    const target = await browser.waitForTarget(item => item._targetId === targetId, { timeout: 8000 });
    const page = await target.page();
    if (!page) throw new Error('后台详情标签页未就绪');
    // 详情页加载可能重新唤起专用浏览器，新页就绪后再次最小化。
    await setBrowserWindowState(page, 'minimized');
    return page;
  } catch (error) {
    if (targetId) await session.send('Target.closeTarget', { targetId }).catch(() => {});
    throw error;
  } finally {
    await session.detach().catch(() => {});
  }
}

// 前程无忧用 window.open 打开详情。仅在本次卡片点击的同步调用中截取 URL，不真正弹出前台标签页。
async function capture51jobDetailUrl(page, candidate) {
  const result = await page.evaluate((name, expectedText) => {
    const cards = Array.from(document.querySelectorAll('.talent-search-container .card'));
    const sameName = cards.filter(card => {
      const nameEl = card.querySelector('.firstline .name, span.name, .name');
      return (nameEl?.innerText || '').trim() === name;
    });
    const card = sameName.find(item => (item.innerText || '').trim() === expectedText.trim()) ||
      (sameName.length === 1 ? sameName[0] : null);
    if (!card) return { clicked: false, url: '' };
    const clickTarget = card.querySelector('.firstline .name, span.name, .name') || card;
    const originalOpen = window.open;
    let openedUrl = '';
    try {
      window.open = function(url) { openedUrl = String(url || ''); return null; };
      clickTarget.click();
    } finally {
      window.open = originalOpen;
    }
    return { clicked: true, url: openedUrl };
  }, candidate.name, candidate.rawCardText || '');
  if (!result.clicked || !result.url) return { ...result, url: '' };
  try {
    const url = new URL(result.url, page.url());
    if (url.hostname !== 'ehire.51job.com' || !url.pathname.includes('/talent/resume/detail')) {
      return { clicked: true, url: '' };
    }
    return { clicked: true, url: url.href };
  } catch (error) {
    return { clicked: true, url: '' };
  }
}

// 全渠道安全验证码深度多模态感知函数（覆盖 BOSS直聘、猎聘网、智联招聘、前程无忧）
async function detectCaptcha(page) {
  try {
    const curUrl = page.url() || '';
    if (curUrl.includes('security-check.html') || curUrl.includes('/verify') || curUrl.includes('baxia-dialog') || curUrl.includes('waf_nc') || curUrl.includes('captcha')) {
      return { detected: true, type: 'url_intercept', url: curUrl };
    }

    const hasDomCaptcha = await page.evaluate(() => {
      // 常见滑块、拼图、点选、极验、易盾、阿里WAF元素特征
      const captchaSelectors = [
        '#nc_1_wrapper', '.nc_wrapper', '#nc_1_n1z', '.btn_slide', '.verify-slider',
        '.baxia-dialog', '[class*="dialog-wrap"][class*="verify"]', '.geetest_radar_tip',
        '.geetest_slider', '.geetest_window', '.geetest_holder', '.geetest_popup_ghost',
        '.captcha-modal', '.verify-container', '.t-sec-dialog', '#captcha-box',
        '.yidun_slider', '.yidun_modal', '.dx_captcha', '#waf_nc_h5_block', '.slider-check',
        'iframe[src*="captcha"]', 'iframe[src*="verify"]', 'iframe[src*="sec"]', 'iframe[src*="waf"]'
      ];

      for (const sel of captchaSelectors) {
        const el = document.querySelector(sel);
        if (el && (el.offsetParent !== null || el.getClientRects().length > 0)) {
          return true;
        }
      }

      const bodyText = document.body ? document.body.innerText : '';
      const keywords = [
        '请完成安全验证', '安全验证', '拖动滑块完成拼图', '请向右滑动滑块',
        '请滑动验证', '行为验证', '操作异常，请完成验证', '验证通过后继续',
        '访问过于频繁，请输入验证码', '请向右拖动滑块'
      ];
      for (const kw of keywords) {
        if (bodyText.includes(kw)) {
          const dialogLike = document.querySelector('.modal, .dialog, .popup, [class*="verify"], [class*="captcha"], [class*="modal"], [class*="mask"]');
          if (dialogLike) return true;
        }
      }
      return false;
    });

    if (hasDomCaptcha) {
      return { detected: true, type: 'dom_modal', url: curUrl };
    }

    // 检查所有子 Frame
    for (const frame of page.frames()) {
      if (frame === page.mainFrame()) continue;
      const fUrl = frame.url() || '';
      if (fUrl.includes('captcha') || fUrl.includes('verify') || fUrl.includes('sec') || fUrl.includes('waf')) {
        return { detected: true, type: 'frame_intercept', url: fUrl };
      }
    }

    return { detected: false };
  } catch (e) {
    // 页面无法检查时不得把未知状态当作安全状态继续自动化。
    return { detected: true, type: 'detection_error', message: e.message };
  }
}

// 验证码挂起拦截与声光唤醒自旋轮询
async function waitForCaptchaResolved(page, platformKey, cfg) {
  const check = await detectCaptcha(page);
  if (!check.detected) return true;

  sendMsg('captcha', {
    platform: platformKey,
    platformName: cfg.name,
    message: `⚠️ 检测到【${cfg.name}】平台安全验证，自动化引擎已安全挂起！请在已打开的 Edge 浏览器中完成滑动拼图...`
  });

  try {
    // 恢复窗口为正常可见状态并激活置顶
    await setBrowserWindowState(page, 'normal');
    await page.bringToFront();
  } catch (e) {}

  const startWait = Date.now();
  const maxWaitMs = 180000; // 最长等待3分钟

  while (Date.now() - startWait < maxWaitMs) {
    await new Promise(r => setTimeout(r, 1200));
    try {
      if (!page.browser().isConnected()) { safetyStopped = true; return false; }
    } catch (e) {}

    const reCheck = await detectCaptcha(page);
    if (!reCheck.detected) {
      sendMsg('captcha_resolved', {
        platform: platformKey,
        platformName: cfg.name,
        message: `🎉 【${cfg.name}】安全验证已通过！自动化引擎已自动无缝恢复运转！`
      });
      // 验证通过，重新将浏览器缩入后台静默任务栏沙盒
      await setBrowserWindowState(page, 'minimized');
      await new Promise(r => setTimeout(r, 1200));
      return true;
    }
  }

  sendMsg('error', {
    platform: platformKey,
    message: `⚠️ 【${cfg.name}】安全验证等待超时，已停止当前渠道抓取以保护账号安全。`
  });
  safetyStopped = true;
  return false;
}

// 跨 Frame 深度穿透提取候选人卡片（支持 51job 新版/老版、BOSS 直聘、智联、猎聘等全渠道，并支持全局跨批次排重）
async function extractCandidatesAcrossFrames(page, targetCount, keyword, platformName, excludedNames = [], excludedUrls = [], filters = {}) {
  const evaluateCardFn = (targetCount, kw, pName, exclNames, exclUrls, activeFilters) => {
    const results = [];
    const seenNames = new Set(exclNames || []);
    const seenUrls = new Set(exclUrls || []);

    const selectors = [
      // 前程无忧 (51job) 最新版 Revision 容器
      '.talent-search-container .card',
      '.eh-talent-search .card',
      'div.card',
      // BOSS 直聘与通用选择器
      '.card-list .candidate-card-wrap', '.recommend-card-list .candidate-card-wrap',
      '.candidate-card-wrap', '.candidate-card', '.card-inner', '.recommend-card',
      '.geek-item', '.candidate-item', '.user-card', '.resume-item', '.resume-list-item',
      '.search-result-item', '.search-item', '.list-item', '[class*="candidate"]', '[class*="resume-card"]',
      '.talent-card', '.user-item', '.chat-user-item', '.resume-card-exp',
      // 前程无忧老版 / 智联 / 猎聘 表格行与列表项
      '.res-list tr', '.resume-list tr', '.table-candidate tr', 'tr.tr-resume', 'tr[class*="resume"]',
      '.candidate-box', '.talent-item', '.item-box', '[class*="resumeItem"]', '[class*="searchItem"]',
      '[class*="talentItem"]', '.resume_detail', '.res_list_box', '.resume-item-wrap', '.el-table__row'
    ];

    const elements = document.querySelectorAll(selectors.join(', '));
    for (let i = 0; i < elements.length && results.length < targetCount; i++) {
      const el = elements[i];
      // 过滤不可见节点
      if (el.offsetParent === null && el.getClientRects().length === 0) continue;
      const text = el.innerText || '';
      if (text.length < 20) continue;
      // 平台页面的筛选控件不统一，以卡片中可验证的字段做最终准入。
      // 未披露的城市、年限或学历一律跳过，避免把未知当作满足。
      const city = (activeFilters.city || '').trim();
      if (city && city !== '不限' && city !== '全国' && !text.includes(city)) continue;
      const minYears = parseInt(activeFilters.exp, 10) || 0;
      if (minYears > 0) {
        const years = Array.from(text.matchAll(/(\d+(?:\.\d+)?)\s*年(?:以上|工作经验|经验|及以上|以内|-\d+年)?/g))
          .map(m => Number(m[1]));
        if (!years.some(y => y >= minYears)) continue;
      }
      const educationRank = { '中专': 1, '高中': 1, '大专': 2, '专科': 2, '本科': 3, '学士': 3, '硕士': 4, '研究生': 4, '博士': 5 };
      const requiredRank = educationRank[activeFilters.edu] || 0;
      if (requiredRank > 0) {
        const foundRanks = Object.entries(educationRank).filter(([label]) => text.includes(label)).map(([, rank]) => rank);
        if (!foundRanks.some(rank => rank >= requiredRank)) continue;
      }

      // 提取姓名（兼容 51job .firstline .name, span.name 等）
      const nameEl = el.querySelector('.firstline .name, span.name, h3, h4, .name, .user-name, .geek-name, .title-text, .c-name, .title, .candidate-name, td.name, td a[href*="resume"], a[href*="Resume"], a[href*="detail"], td:first-child a');
      if (!nameEl) continue;

      let name = nameEl.innerText.trim();
      name = name.split('\n')[0].trim();
      if (!name || name.length > 10) continue;

      // 纯净姓名与去重判断（跳过项目已有候选人）
      const cleanName = name.replace(/^【.*?】/, '').replace(/^BOSS牛人_/, '').split('_')[0].trim();
      // 同名候选人可能是不同人，必须依赖独立主页或复合履历信息排重。

      // 提取直达链接
      let candUrl = '';
      const aTag = el.querySelector('a[href*="resume"], a[href*="Resume"], a[href*="detail"], a[href*="geek"], a[href*="talent"], a');
      if (aTag && aTag.href && !aTag.href.startsWith('javascript:')) {
        candUrl = aTag.href;
      }
      if (!candUrl) {
        const seq = el.getAttribute('data-seq') || el.getAttribute('data-resumeid') || el.getAttribute('data-id') || el.getAttribute('data-userid') || el.getAttribute('data-geekid');
        if (seq) {
          candUrl = `${window.location.origin}${window.location.pathname}?seq=${seq}`;
        }
      }

      // 关键判定：只有独立的候选人详情 URL 才参与 URL 排重，绝对不能把搜索列表页 URL 当作排重依据！
      const isCandDetailUrl = (u) => {
        if (!u) return false;
        const low = u.toLowerCase();
        if (low.includes('/talent/search') || low.includes('/search') || low.includes('/recommend') || low.includes('/navigate')) return false;
        return low.includes('id=') || low.includes('seq=') || low.includes('user') || low.includes('resume') || low.includes('detail') || low.includes('geek');
      };

      if (isCandDetailUrl(candUrl)) {
        if (seenUrls.has(candUrl)) continue;
        seenUrls.add(candUrl);
      } else {
        candUrl = ''; // 保持空，等待详情穿透提取真实 ID 链接
      }

      // 基本画像
      const infoEl = el.querySelector('.userinfo, .detail, .firstline, .base-info.join-text-wrap, .info, .labels, .base-info, .desc, .user-desc, .exp-edu, .info-labels, td.exp, td.edu');
      let infoText = infoEl ? infoEl.innerText.trim().replace(/\n+/g, ' · ') : '';
      infoText = infoText.replace(name, '').replace(/^[\s·]+/, '');

      // 任职履历与详细经历
      const workEl = el.querySelector('.info_content, .work, .work-exp, .company, .company-name, .position, .experience, .resume-card-exp, td.company');
      const workText = workEl ? workEl.innerText.trim().replace(/\n+/g, ' | ') : '';

      // 核心专业技能标签
      const tags = Array.from(new Set(
        Array.from(el.querySelectorAll('.skill_label, .content_tag_item, .tag, .skill-tag, .tag-item, span.label, .skill-label, .label-item, span[class*="tag"], span[class*="label"], .match-tag'))
          .map(t => t.innerText.trim())
          .filter(t => t && t.length < 20 && !t.includes('电话') && !t.includes('聊') && !t.includes('活跃') && !t.includes('求职意向'))
      ));

      results.push({
        name,
        infoText,
        workText,
        skills: tags,
        url: candUrl,
        rawCardText: text
      });
    }
    return results;
  };

  // 1. 优先在主文档查找
  let items = await page.evaluate(evaluateCardFn, targetCount, keyword, platformName, excludedNames, excludedUrls, filters).catch(() => []);
  if (items && items.length > 0) return items;

  // 2. 主文档无结果时穿透遍历子 iframe
  for (const frame of page.frames()) {
    if (frame === page.mainFrame()) continue;
    try {
      const fItems = await frame.evaluate(evaluateCardFn, targetCount, keyword, platformName, excludedNames, excludedUrls, filters);
      if (fItems && fItems.length > 0) return fItems;
    } catch (e) {}
  }
  return [];
}

async function searchPageFingerprint(page) {
  const frames = typeof page.frames === 'function' ? page.frames() : [page];
  const parts = [];
  for (const frame of frames) {
    const part = await frame.evaluate(() => {
      const cardSelector = '.talent-search-container .card, .eh-talent-search .card, .candidate-card-wrap, .geek-item, .res-list tr, .candidate-box, .resume-item, .talent-item';
      const cards = Array.from(document.querySelectorAll(cardSelector))
        .filter(el => el.offsetParent !== null || el.getClientRects().length > 0);
      if (cards.length === 0) return '';
      const activePage = document.querySelector('.el-pagination .is-active, .pagination .active, li.number.active')?.textContent?.trim() || '';
      const identities = cards.slice(0, 60).map(card => {
        const name = card.querySelector('.firstline .name, span.name, .name, h3, h4, .user-name')?.textContent?.trim() || '';
        const link = card.querySelector('a[href]')?.getAttribute('href') || '';
        const id = card.getAttribute('data-resumeid') || card.getAttribute('data-id') || '';
        return `${name}:${link}:${id}`;
      });
      let scrollState = '';
      for (let el = cards[0]?.parentElement; el && el !== document.body; el = el.parentElement) {
        if (el.scrollHeight > el.clientHeight + 100) {
          scrollState = `${el.scrollTop}/${el.scrollHeight}/${el.clientHeight}`;
          break;
        }
      }
      return JSON.stringify([location.pathname, activePage, cards.length, identities, scrollState]);
    }).catch(() => '');
    if (part) parts.push(part);
  }
  return parts.join('||');
}

async function scrollSearchResults(page) {
  await smoothScroll(page, 600, 100).catch(() => {});
}

async function advanceSearchResults(page) {
  const before = await searchPageFingerprint(page);
  const clickedNext = await page.evaluate(() => {
    const selectors = 'button.btn-next, .btn-next, .next-page, a.next, .el-pagination .btn-next, li.number.active + li.number';
    const buttons = Array.from(document.querySelectorAll(selectors));
    const button = buttons.find(el =>
      (el.offsetParent !== null || el.getClientRects().length > 0) &&
      !el.disabled && el.getAttribute('aria-disabled') !== 'true' &&
      !String(el.className).includes('is-disabled'));
    if (!button) return false;
    button.click();
    return true;
  }).catch(() => false);
  if (!clickedNext) await scrollSearchResults(page);
  for (let attempt = 0; attempt < 4; attempt++) {
    await new Promise(resolve => setTimeout(resolve, 600));
    const after = await searchPageFingerprint(page);
    if (after && after !== before) return true;
  }
  if (clickedNext) {
    await scrollSearchResults(page);
    const after = await searchPageFingerprint(page);
    if (after && after !== before) return true;
  }
  return false;
}

// 深度净化候选人简历正文：剔除防泄密水印网格、举报按钮、平台免责声明与动态操作框
function cleanCandidateResumeText(rawText) {
  if (!rawText) return '';
  let cleaned = rawText;

  // 1. 剔除末尾平台免责声明、输入框与操作动态面板
  cleaned = cleaned.replace(/声明[：:][\s\S]*?终止服务.*?$/is, '');
  cleaned = cleaned.replace(/与人才沟通[\s\S]*?$/is, '');
  cleaned = cleaned.replace(/操作动态[\s\S]*?$/is, '');
  cleaned = cleaned.replace(/仅看评价[\s\S]*?$/is, '');
  cleaned = cleaned.replace(/0\s*\/\s*500[\s\S]*?$/is, '');
  cleaned = cleaned.replace(/您可对候选人的在职状态进行相关询问[～~]/g, '');

  // 2. 剔除独立的“举报”及“虚假简历举报”等行
  cleaned = cleaned.replace(/^\s*(?:举报|侵权举报|虚假简历举报|举报该简历)\s*$/mgi, '');

  // 3. 核心水印清洗算法：检测并剔除横向或纵向重复网格化出现的企业防泄密水印
  // 水印特征：同一行内出现多次相同的公司名称，或者连续多行只包含同一公司名称
  cleaned = cleaned.replace(/(上海透景生命科技股份有限公司[\s\t]*)+/g, '');

  // 通用规则：检测任何公司名称（以股份有限公司、有限公司、科技、集团结尾的4~30字短语）在局部大量重复
  cleaned = cleaned.replace(/(?:[\t ]*([^\n\r]{4,30}?(?:公司|企业|集团|机构))[\t ]*){2,}/g, (match, word) => {
    const escaped = word.replace(/[-/\\^$*+?.()|[\]{}]/g, '\\$&');
    const count = (match.match(new RegExp(escaped, 'g')) || []).length;
    return count >= 2 ? '' : match;
  });

  // 去除只包含重复水印的独立行
  const lines = cleaned.split('\n');
  const validLines = [];
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) {
      validLines.push('');
      continue;
    }
    if (trimmed === '举报' || trimmed.startsWith('声明：以上人才信息仅供')) continue;
    // 检查单行内是否由相同单词/短语重复填充
    const words = trimmed.split(/\s{2,}|\t+/);
    if (words.length >= 2 && words.every(w => w === words[0])) continue;
    validLines.push(line);
  }
  cleaned = validLines.join('\n');

  // 4. 清理冗余空行
  cleaned = cleaned.replace(/\n{3,}/g, '\n\n').trim();
  return cleaned;
}

// 详情穿透引擎：针对搜索列表仅展示精简摘要的特性，点击候选人穿透提取抽屉/新标签页中的全量履历与优势
async function enrichCandidatesWithFullDetail(page, browser, candidates, platformKey, cfg) {
  if (!candidates || candidates.length === 0) return candidates;

  sendMsg('status', {
    platform: platformKey,
    message: `🔍 【${cfg.name}】正在进行简历深度穿透，逐一提取全量工作经历与完整优势...`
  });

  const enriched = [];

  for (let i = 0; i < candidates.length; i++) {
    if (!await waitForCaptchaResolved(page, platformKey, cfg)) return null;
    const cand = candidates[i];
    sendMsg('status', {
      platform: platformKey,
      message: `📑 正在穿透读取【${cand.name}】的完整微简历档案 (${i + 1}/${candidates.length})...`
    });

    let fullDetailText = '';

    try {
      // 1. 若已有独立详情 URL，用 CDP 后台标签页提取，不激活 Edge 窗口。
      const isDirectDetail = cand.url && (cand.url.includes('id=') || cand.url.includes('seq=') || cand.url.includes('ResumeView') || cand.url.includes('geek'));
      if (isDirectDetail) {
        let directPage = null;
        let directFocusSession = null;
        try {
          directPage = await createBackgroundPage(browser, page, cand.url);
          directFocusSession = await keepPageActiveInBackground(directPage);
          await directPage.waitForFunction(
            () => document.body && document.body.innerText.length > 200,
            { timeout: 5000 }
          ).catch(() => {});
          if (!await waitForCaptchaResolved(directPage, platformKey, cfg)) return null;
          fullDetailText = await directPage.evaluate(() => document.body.innerText).catch(() => '');
        } catch (e) {
        } finally {
          if (directFocusSession) {
            await directFocusSession.send('Emulation.setFocusEmulationEnabled', { enabled: false }).catch(() => {});
            await directFocusSession.detach().catch(() => {});
          }
          if (directPage) await directPage.close().catch(() => {});
        }
      }

      // 2. 前程无忧卡片会同步调用 window.open；截取详情 URL 后再用后台标签页加载。
      if (platformKey === '51job' && (!fullDetailText || fullDetailText.length < 200)) {
        const captured = await capture51jobDetailUrl(page, cand);
        if (captured.url) {
          let detailPage = null;
          let detailFocusSession = null;
          try {
            detailPage = await createBackgroundPage(browser, page, captured.url);
            detailFocusSession = await keepPageActiveInBackground(detailPage);
            await detailPage.waitForFunction(
              () => document.body && document.body.innerText.length > 200,
              { timeout: 5000 }
            ).catch(() => {});
            if (!await waitForCaptchaResolved(detailPage, platformKey, cfg)) return null;
            fullDetailText = await detailPage.evaluate(() => document.body.innerText).catch(() => '');
            if (fullDetailText.length > 200) cand.url = captured.url;
          } finally {
            if (detailFocusSession) {
              await detailFocusSession.send('Emulation.setFocusEmulationEnabled', { enabled: false }).catch(() => {});
              await detailFocusSession.detach().catch(() => {});
            }
            if (detailPage) await detailPage.close().catch(() => {});
          }
        } else {
          sendMsg('status', { platform: platformKey, message: `【前程无忧】未取得 ${cand.name} 的独立详情地址，保留已核实的列表信息。` });
        }
      }

      // 3. 其他平台仍使用卡片点击和抽屉监听。
      if (platformKey !== '51job' && (!fullDetailText || fullDetailText.length < 200)) {
        let newPagePromise = new Promise(resolve => {
          const handler = async target => {
            try {
              const p = await target.page();
              if (p) {
                browser.off('targetcreated', handler);
                resolve(p);
              }
            } catch (e) { resolve(null); }
          };
          browser.on('targetcreated', handler);
          setTimeout(() => {
            browser.off('targetcreated', handler);
            resolve(null);
          }, 1500);
        });

        // 拟人点击候选人卡片或姓名
        const clicked = await page.evaluate((candName, idx) => {
          const cards = document.querySelectorAll(
            '.talent-search-container .card, div.card, .eh-talent-search .card, .candidate-card-wrap, .geek-item, .res-list tr, .candidate-box'
          );
          let targetCard = null;
          for (const c of cards) {
            const nameEl = c.querySelector('.firstline .name, span.name, .name, h3, h4, .user-name');
             if (nameEl && (nameEl.innerText || '').trim() === candName) {
              targetCard = c;
              break;
            }
          }

          if (targetCard) {
            const clickTarget = targetCard.querySelector('.firstline .name, span.name, .name, a, h3, h4') || targetCard;
            try { clickTarget.scrollIntoView({ behavior: 'smooth', block: 'center' }); } catch (e) {}
            clickTarget.click();
            return true;
          }
          return false;
        }, cand.name, i);

        if (clicked) {
          const newPage = await newPagePromise;
          if (newPage) {
            await newPage.waitForFunction(
              () => document.body && document.body.innerText.length > 200,
              { timeout: 3500 }
            ).catch(() => {});

            cand.url = newPage.url() || cand.url;
            fullDetailText = await newPage.evaluate(() => document.body.innerText).catch(() => '');
            await newPage.close().catch(() => {});
          } else {
            // 抽屉/模态弹层快速轮询 (最长等待 1.5 秒，每 150ms 轮询一次)
            const waitStart = Date.now();
            while (Date.now() - waitStart < 1500) {
              fullDetailText = await page.evaluate(() => {
                const drawerSelectors = [
                  '.el-drawer__body', '.el-drawer',
                  '.resume-detail', '.resume-detail-drawer', '.detail-box',
                  '.candidate-detail', '.user-detail', '.dialog-resume',
                  '[class*="resume-detail"]', '[class*="ResumeDetail"]',
                  '.chat-detail'
                ];
                for (const sel of drawerSelectors) {
                  const els = document.querySelectorAll(sel);
                  for (const el of els) {
                    if ((el.offsetParent !== null || el.getClientRects().length > 0) && (el.innerText || '').length > 200) {
                      return el.innerText;
                    }
                  }
                }
                const all = Array.from(document.querySelectorAll('div, section, aside'));
                const containers = all.filter(el => {
                  const t = el.innerText || '';
                  const isVis = el.offsetParent !== null || el.getClientRects().length > 0;
                  return isVis && t.includes('工作经历') && (t.includes('个人优势') || t.includes('项目经验') || t.includes('教育经历')) && el.children.length >= 2;
                });
                if (containers.length > 0) {
                  containers.sort((a, b) => b.innerText.length - a.innerText.length);
                  return containers[0].innerText;
                }
                return '';
              }).catch(() => '');

              if (fullDetailText && fullDetailText.length > 200) break;
              await new Promise(r => setTimeout(r, 150));
            }

            // 抓取完毕快速关闭抽屉
            await page.keyboard.press('Escape');
            await page.evaluate(() => {
              const closeBtns = document.querySelectorAll('.el-drawer__close-btn, .close-btn, .icon-close, [class*="close"]');
              for (const b of closeBtns) {
                if (b.offsetParent !== null) { b.click(); break; }
              }
            }).catch(() => {});
          }
        }
      }
    } catch (err) {
      console.warn(`[TalentLens] 穿透提取候选人【${cand.name}】详情异常: ${err.message}`);
    }

    // 执行文本深度净化（剔除水印网格、举报、免责声明等噪音）
    if (fullDetailText) {
      fullDetailText = cleanCandidateResumeText(fullDetailText);
      const idMatch = fullDetailText.match(/人才ID[：:\s]*(\d+)/i) || fullDetailText.match(/ID[：:\s]*(\d{6,})/i);
      if (idMatch && (!cand.url || cand.url.includes('/talent/search'))) {
        cand.url = `https://ehire.51job.com/Candidate/ResumeView.aspx?hidUserID=${idMatch[1]}`;
      }
    }

    // 若成功提取到全量详情正文，则融合并升级候选人信息
    if (fullDetailText && fullDetailText.length > (cand.rawCardText || '').length) {
      cand.rawCardText = fullDetailText;

      // 提取全量工作经历
      const workSectionMatch = fullDetailText.match(/工作经历[\s\S]*?(?=项目经验|教育经历|证书|求职意向|$)/i);
      if (workSectionMatch && workSectionMatch[0].length > 20) {
        cand.workText = cleanCandidateResumeText(workSectionMatch[0].trim().replace(/\n+/g, ' | '));
      }

      // 提取核心优势
      const advMatch = fullDetailText.match(/个人优势[\s\S]*?(?=工作经历|项目经验|教育经历|证书|$)/i);
      if (advMatch && advMatch[0].length > 10) {
        cand.advantage = cleanCandidateResumeText(advMatch[0].trim());
      }
    }

    // 51job 的异步详情脚本可能稍后才唤起窗口；每人读取完再次最小化。
    if (options.testLoginPlatform !== platformKey) await setBrowserWindowState(page, 'minimized');
    enriched.push(cand);
    await new Promise(r => setTimeout(r, 150));
  }

  return enriched;
}

// 优先复用已有调试会话；没有会话时只启动一次 Edge，避免直启失败后再次启动产生第二个窗口。
async function launchPlatformBrowser(cfg, browserPath, profileDir) {
  const debugPort = cfg.debugPort;
  const isLoginTest = options.testLoginPlatform === cfg.code;

  // 先连接已经在跑的浏览器实例。
  try {
    const browser = await puppeteer.connect({
      browserURL: `http://127.0.0.1:${debugPort}`,
      defaultViewport: null
    });
    sendMsg('status', { platform: cfg.code, message: `♻️ 已复用【${cfg.name}】已打开的浏览器窗口` });
    if (!isLoginTest) {
      await setBrowserWindowState(browser, 'minimized');
    }
    return browser;
  } catch (e) {
    // 没有在跑的实例，继续下面的流程
  }

  // 清理仅属于当前平台隔离 profile 的残留进程与锁文件。
  sendMsg('status', { platform: cfg.code, message: `🧹 清理【${cfg.name}】残留浏览器进程...` });
  killBrowserByProfile(profileDir);
  await new Promise(r => setTimeout(r, 1500)); // 等进程完全退出
  clearProfileLocks(profileDir);

  // Edge 只启动一次，并直接打开平台网址；不先打开新标签页再启动第二个 Edge。
  const child = spawn(browserPath, browserLaunchArgs(cfg, profileDir, isLoginTest),
    { detached: true, stdio: 'ignore', windowsHide: true });
  child.unref();

  for (let i = 0; i < 20; i++) {
    await new Promise(r => setTimeout(r, 800));
    try {
      const browser = await puppeteer.connect({
        browserURL: `http://127.0.0.1:${debugPort}`,
        defaultViewport: null
      });
      if (!isLoginTest) {
        await setBrowserWindowState(browser, 'minimized');
      }
      return browser;
    } catch (e) {}
  }

  throw new Error(`无法连接刚启动的浏览器 (profile: ${path.basename(profileDir)})，请检查该配置文件是否被其他 Edge 窗口占用`);
}

// 统一抓取单个平台 (返回 null 表示启动/连接失败，返回 [] 表示成功连接但无结果)
async function scrapePlatform(platformKey, browserPath, targetCount, keywordPlan = null) {
  const cfg = PLATFORM_CONFIGS[platformKey];
  if (!cfg) return null;
  const outcome = (candidates, reason = '') => ({ candidates, reason });

  sendMsg('status', {
    platform: platformKey,
    message: `${cfg.icon} 正在连接【${cfg.name}】企业端后台通道...`
  });

  const profileDir = path.join(options.dataDir, '..', cfg.profileFolder);
  if (!fs.existsSync(profileDir)) fs.mkdirSync(profileDir, { recursive: true });

  let browser = null;
  try {
    browser = await launchPlatformBrowser(cfg, browserPath, profileDir);
  } catch (err) {
    sendMsg('error', {
      platform: platformKey,
      message: `❌ 无法启动 ${cfg.name} 浏览器直连: ${err.message}`
    });
    return null; // null = 启动失败（区别于 [] 即成功但无结果）
  }

  let backgroundSession = null;
  // 断开自动化连接后，后台浏览器仍保留给 HR 使用，Node 进程才能正常退出。
  try {
  // 等待浏览器进程就绪
  await new Promise(r => setTimeout(r, 1500));

  let pages = await browser.pages();

  // 优先选择已经在平台域名上的 tab（避免选错 about:blank tab）
  const domainHint = cfg.code === '51job' ? '51job.com' : cfg.code;
  let page = pages.find(p => {
    const u = p.url() || '';
    return u.includes(domainHint) && !u.includes('about:blank');
  });

  if (!page) {
    page = pages[0] || (await browser.newPage());
  }

  if (options.testLoginPlatform !== cfg.code) {
    backgroundSession = await keepPageActiveInBackground(page);
    if (!backgroundSession) {
      sendMsg('status', { platform: platformKey, message: `【${cfg.name}】浏览器不支持后台活动模式，最小化后页面可能暂停，请保持窗口可见。` });
    }
  }

  // 自动清理隔离 profile 中多余的 Edge 新标签页，避免额外窗口留在前台。
  try {
    await closeUnusedStartPages(browser, page);
  } catch (e) {}

  // 仅在扫码登录测试时激活置顶窗口，常规寻才检索时维持后台沙盒最小化
  const isLoginTest = options.testLoginPlatform === cfg.code;
  if (isLoginTest) {
    try { await page.bringToFront(); } catch (e) {}
  } else {
    await setBrowserWindowState(page, 'minimized');
  }

  // 注入反检测防护：隐藏 webdriver，并拦截任何脚本企图把页面强制跳转至 about:blank 的行为
  try {
    await page.evaluateOnNewDocument(() => {
      Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
      window.chrome = window.chrome || { runtime: {} };

      // 阻止 Location.prototype.replace / assign 跳转至 about:blank
      try {
        const origReplace = Location.prototype.replace;
        Location.prototype.replace = function(url) {
          if (typeof url === 'string' && url.includes('about:blank')) {
            console.warn('[TalentLens] 已拦截第三方脚本强制跳转至 about:blank');
            return;
          }
          return origReplace.call(this, url);
        };
      } catch (e) {}

      try {
        const origAssign = Location.prototype.assign;
        Location.prototype.assign = function(url) {
          if (typeof url === 'string' && url.includes('about:blank')) {
            console.warn('[TalentLens] 已拦截第三方脚本 assign 跳转至 about:blank');
            return;
          }
          return origAssign.call(this, url);
        };
      } catch (e) {}
    });
  } catch (e) {}

  // 拦截并阻断猎聘等平台的反爬崩溃探针脚本 (security.min.js)
  try {
    await page.setRequestInterception(true);
    page.on('request', req => {
      const u = req.url() || '';
      // 猎聘 security.min.js 检测到 DevTools/自动化后会强行清空页面跳转 about:blank
      if (u.includes('security.min.js')) {
        req.abort();
      } else {
        req.continue();
      }
    });
  } catch (e) {}

  // Edge/Chrome 首次启动常打开内置新标签页，必须先进入招聘网站再判断登录态。
  const navigation = await ensurePlatformPage(page, cfg);
  if (!navigation.ok) {
    sendMsg('error', {
      platform: platformKey,
      message: `【${cfg.name}】招聘网站未能打开：${navigation.reason}`
    });
    return outcome([], '招聘网站未能打开');
  }

  // 导航完成后再次清理新标签页；常规检索保持最小化。
  try {
    await closeUnusedStartPages(browser, page);
    if (isLoginTest) await page.bringToFront();
    else await setBrowserWindowState(page, 'minimized');
  } catch (e) {}

  // 严格 DOM 登录态检测函数
  const checkAuth = async () => {
    try {
      const curUrl = page.url() || '';
      if (!isPlatformPageUrl(curUrl, cfg)) {
        return { logged: false, curUrl, reason: 'wrong_page' };
      }

      const domAuth = await page.evaluate((code) => {
        const url = window.location.href;
        const text = document.body ? document.body.innerText : '';

        // 404 检测
        if (text.includes('当前页面未找到') || text.includes('页面不存在') || text.includes('404') || document.title.includes('404')) {
          return { logged: false, reason: '404_error' };
        }

        // 猎聘网
        if (code === 'liepin') {
          const isLoginUrl = url.includes('/user/login') || url.includes('/login') || url.includes('/passport');
          const hasLoginForm = !!document.querySelector('input[name*="user_login"], input[type="password"], input[placeholder*="手机号"], input[placeholder*="验证码"], .ant-lpt-input, .login-container, .login-box, .scan-box, .login-form');
          if (isLoginUrl || hasLoginForm) {
            return { logged: false, reason: 'login_form_present' };
          }
          const hasUserInfo = !!document.querySelector('.header-user-info, .user-name, .company-name, a[href*="logout"], .nav-user, .lpt-header-user, .user-avatar, .user-nav, .enterprise-info');
          const hasRecNav = (url.includes('lpt.liepin.com') || url.includes('h.liepin.com')) && (text.includes('职位管理') || text.includes('人才搜索') || text.includes('沟通') || text.includes('候选人') || text.includes('推荐'));
          return { logged: hasUserInfo || hasRecNav, reason: 'ok' };
        }

        // BOSS直聘
        if (code === 'boss') {
          const isLoginUrl = url.includes('/login') || url.includes('/web/user') || url.includes('intent=');
          const hasLoginForm = !!document.querySelector('input[type="tel"], input[placeholder*="手机号"], .login-box, .login-scan-box, .btn-sure, .dialog-login');
          if (isLoginUrl || hasLoginForm) {
            return { logged: false, reason: 'login_form_present' };
          }
          const hasUserInfo = !!document.querySelector('.user-nav, .nav-figure, .header-user, .user-avatar, .nav-item-user, a[href*="logout"], .nav-user, .user-name, .chat-user');
          const hasRecNav = (url.includes('/boss/') || url.includes('recommend') || url.includes('/web/chat/')) && (text.includes('推荐牛人') || text.includes('职位管理') || text.includes('沟通'));
          return { logged: hasUserInfo || hasRecNav, reason: 'ok' };
        }

        // 智联招聘
        if (code === 'zhaopin') {
          const isLoginUrl = url.includes('passport.zhaopin.com') || url.includes('/login');
          const hasLoginForm = !!document.querySelector('input[type="password"], .login-box, .passport-login, .login-form');
          if (isLoginUrl || hasLoginForm) {
            return { logged: false, reason: 'login_form_present' };
          }
          const hasUserInfo = !!document.querySelector('.user-info, .header-user, .c-user-name, a[href*="logout"], .user-avatar, .header-user-name');
          const hasRecNav = (url.includes('rd6.zhaopin.com') || url.includes('ihr.zhaopin.com') || url.includes('rd5.zhaopin.com')) && (text.includes('简历管理') || text.includes('人才搜索') || text.includes('职位管理') || text.includes('推荐') || text.includes('候选人'));
          return { logged: hasUserInfo || hasRecNav, reason: 'ok' };
        }

        // 前程无忧
        if (code === '51job') {
          const isLoginUrl = url.includes('MainLogin.aspx') || url.includes('login');
          const hasLoginForm = !!document.querySelector('#txtMemberName, #txtUserName, #txtPassword, input[name*="password"], .login-box');
          if (isLoginUrl || hasLoginForm) {
            return { logged: false, reason: 'login_form_present' };
          }
          const hasUserInfo = !!document.querySelector('#lblUserName, #divHead, .user-name, a[href*="Logout"], #spanCompanyName, .header-user');
          const hasRecNav = url.includes('ehire.51job.com') && (text.includes('简历管理') || text.includes('搜索简历') || text.includes('职位管理'));
          return { logged: hasUserInfo || hasRecNav, reason: 'ok' };
        }

        return { logged: false, reason: 'unknown_code' };
      }, cfg.code);

      return { logged: domAuth.logged, curUrl, reason: domAuth.reason };
    } catch (e) {
      return { logged: false, curUrl: '', reason: e.message };
    }
  };

  sendMsg('status', {
    platform: platformKey,
    message: `🔑 正在验证【${cfg.name}】企业端登录态...`
  });

  let authResult = await checkAuth();
  if (!authResult.logged && authResult.reason === 'ok') {
    await new Promise(r => setTimeout(r, 1500));
    authResult = await checkAuth();
  }
  for (let attempt = 0; attempt < 3 && !authResult.logged && authResult.reason === 'wrong_page'; attempt++) {
    sendMsg('status', { platform: platformKey, message: `【${cfg.name}】浏览器停留在新标签页，正在重新进入招聘网站（${attempt + 1}/3）...` });
    const recovery = await ensurePlatformPage(page, cfg);
    if (!recovery.ok) break;
    await new Promise(r => setTimeout(r, 1200));
    authResult = await checkAuth();
  }
  if (authResult.reason === 'wrong_page') {
    sendMsg('error', { platform: platformKey, message: `【${cfg.name}】浏览器仍未进入招聘网站，检索未启动。请查看 Edge 是否停在新标签页。` });
    return outcome([], '浏览器未进入招聘网站');
  }
  if (!authResult.logged) {
    await setBrowserWindowState(page, 'normal');
    await page.bringToFront().catch(() => {});
    sendMsg('auth', {
      platform: platformKey,
      status: 'need_login',
      message: `请在打开的浏览器中，扫码或账号登录【${cfg.name}】企业端`
    });
    sendMsg('status', {
      platform: platformKey,
      message: `👉 等待用户在打开的 Edge 浏览器中登录【${cfg.name}】（检测到登录成功后将自动继续抓取）...`
    });

    const startTime = Date.now();
    let pageRecoveries = 0;
    while (Date.now() - startTime < 300000) { // 5分钟等待
      await new Promise(r => setTimeout(r, 2000));
      try {
        if (!browser.isConnected()) {
          sendMsg('status', { platform: platformKey, message: `【${cfg.name}】浏览器窗口已关闭` });
          return outcome([], '浏览器窗口已关闭');
        }
      } catch (e) {}

      authResult = await checkAuth();
      if (authResult.reason === 'wrong_page') {
        if (++pageRecoveries > 3) {
          sendMsg('error', { platform: platformKey, message: `【${cfg.name}】浏览器反复返回新标签页，已停止当前渠道检索。` });
          return outcome([], '浏览器反复返回新标签页');
        }
        const recovery = await ensurePlatformPage(page, cfg);
        if (!recovery.ok) {
          sendMsg('error', { platform: platformKey, message: `【${cfg.name}】重新进入招聘网站失败：${recovery.reason}` });
          return outcome([], '重新进入招聘网站失败');
        }
        await new Promise(r => setTimeout(r, 1200));
        authResult = await checkAuth();
      }
      if (authResult.logged) {
        break;
      }
    }
  }

  if (!authResult.logged) {
    sendMsg('error', {
      platform: platformKey,
      message: `⚠️ 未检测到【${cfg.name}】企业登录态（或超时未登录），已跳过该渠道。`
    });
    return outcome([], '未检测到企业登录态');
  }

  if (!isLoginTest) await setBrowserWindowState(page, 'minimized');

// 自动导航至平台的搜索/推荐中心，并执行关键词自动键入与搜索触发
async function autoNavigateAndSearch(page, platformKey, cfg, searchKeyword) {
  const kw = searchKeyword || options.keyword;
  try {
    if (platformKey === '51job') {
      let curUrl = page.url() || '';
      // 不在人才搜索页时直接进入目标页，避免点击同名的菜单父容器。
      if (!/\/Revision\/talent\/search/i.test(curUrl)) {
        sendMsg('status', {
          platform: platformKey,
          message: `🧭 正在自动跳转至【前程无忧】人才搜索中心...`
        });
        await page.goto(cfg.homeUrl, { waitUntil: 'domcontentloaded', timeout: 20000 });
        await new Promise(r => setTimeout(r, 2000));
      }

      // 无论何种途径进入搜索页，自动输入关键词并触发搜索
      try {
        const submitted = await submit51jobSearch(page, kw);
        if (!submitted.ok) {
          sendMsg('error', { platform: platformKey, message: `【前程无忧】未提交关键词搜索：${submitted.reason}` });
          return false;
        }
        sendMsg('status', { platform: platformKey, message: `【前程无忧】关键词「${kw}」已触发检索，正在读取候选人列表...` });
      } catch (e) {
        sendMsg('error', { platform: platformKey, message: `【前程无忧】提交关键词搜索失败：${e.message}` });
        return false;
      }

    } else if (platformKey === 'zhaopin') {
      try {
        const searchBoxSelector = 'input[placeholder*="搜索"], input[placeholder*="关键词"], input[placeholder*="岗位"], .search-input input';
        const hasInput = await page.$(searchBoxSelector);
        if (hasInput) {
          sendMsg('status', {
            platform: platformKey,
            message: `⌨️ 正在自动输入「${kw}」并检索...`
          });
          await humanType(page, searchBoxSelector, kw);
          await page.keyboard.press('Enter');
          await new Promise(r => setTimeout(r, 2000));
        }
      } catch (e) {}

    } else if (platformKey === 'liepin') {
      try {
        const searchBoxSelector = 'input[placeholder*="搜索"], input[placeholder*="关键词"], input[placeholder*="职位"], .search-box input';
        const hasInput = await page.$(searchBoxSelector);
        if (hasInput) {
          sendMsg('status', {
            platform: platformKey,
            message: `⌨️ 正在自动输入「${kw}」并检索...`
          });
          await humanType(page, searchBoxSelector, kw);
          await page.keyboard.press('Enter');
          await new Promise(r => setTimeout(r, 2000));
        }
      } catch (e) {}
    } else if (platformKey === 'boss') {
      try {
        const searchBoxSelector = 'input[placeholder*="搜索"], input[placeholder*="牛人"], .search-input input, input.search-input';
        const hasInput = await page.$(searchBoxSelector);
        if (hasInput) {
          sendMsg('status', {
            platform: platformKey,
            message: `⌨️ 正在自动输入「${kw}」并检索...`
          });
          await humanType(page, searchBoxSelector, kw);
          await page.keyboard.press('Enter');
          await new Promise(r => setTimeout(r, 2000));
        }
      } catch (e) {}
    }
    return true;
  } catch (err) {
    sendMsg('error', { platform: platformKey, message: `【${cfg.name}】打开检索页失败：${err.message}` });
    return false;
  }
}

  sendMsg('status', {
    platform: platformKey,
    message: `🎉 【${cfg.name}】企业后台连接成功！正在执行阶梯水库矩阵式寻才(${options.city})...`
  });

  // 单批最多 10 人；前一批缺额会滚动到后续批次。
  const MAX_BATCH_SIZE = 10;
  const maxBatches = Math.max(Math.ceil(targetCount / MAX_BATCH_SIZE) + 3, 8);
  const allPlatformSavedList = [];
  const savedPerKeyword = Array.isArray(keywordPlan) ? keywordPlan.map(() => 0) : [];
  const exhaustedKeywords = new Set();
  let activeKeyword = '';
  let batchIdx = 0;
  let emptyBatches = 0;

  while (allPlatformSavedList.length < targetCount && batchIdx < maxBatches) {
    const batchStartCount = allPlatformSavedList.length;
    const currentBatchQuota = Math.min(MAX_BATCH_SIZE, targetCount - batchStartCount);

    if (targetCount > MAX_BATCH_SIZE) {
      sendMsg('status', {
        platform: platformKey,
        message: `🛡️ 启动第 ${batchIdx + 1} 批检索（本批最多 ${currentBatchQuota} 人，累计 ${allPlatformSavedList.length}/${targetCount} 人）...`
      });
    }

    // 检查验证码
    const captchaOk = await waitForCaptchaResolved(page, platformKey, cfg);
    if (!captchaOk) return outcome(allPlatformSavedList, '安全验证未完成');

    // 解析当前子批次的配额调度任务矩阵
    const allocations = Array.isArray(keywordPlan)
      ? allocateQuota(currentBatchQuota, keywordPlan.map((quota, i) => ({ ratio: Math.max(0, quota - savedPerKeyword[i]) })))
      : Array.isArray(options.quotaMatrix) ? allocateQuota(currentBatchQuota, options.quotaMatrix) : [];
    const quotaTasks = (options.quotaMatrix && Array.isArray(options.quotaMatrix) && options.quotaMatrix.length > 0)
      ? options.quotaMatrix.map((item, idx) => {
          return {
            keyword: item.keyword || options.keyword,
            categoryName: item.category_name || item.category || '推荐维度',
            ratio: item.ratio,
            targetCount: Array.isArray(keywordPlan) ? Math.min(allocations[idx], Math.max(0, keywordPlan[idx] - savedPerKeyword[idx])) : allocations[idx]
          };
        })
      : [
          {
            keyword: options.keyword,
            categoryName: '目标岗位',
            ratio: 100,
            targetCount: currentBatchQuota
          }
        ];

    // 按当前子批次的配额矩阵依次执行多维度词条轮转检索
    for (let taskIdx = 0; taskIdx < quotaTasks.length; taskIdx++) {
      if (allPlatformSavedList.length >= targetCount) break;

      const task = quotaTasks[taskIdx];
      if (task.targetCount <= 0 || exhaustedKeywords.has(task.keyword)) continue;
      const currentKeyword = task.keyword;
      const currentTarget = task.targetCount;
      const taskStartCount = allPlatformSavedList.length;
      let pageAttempts = 1;
      const maxPageAttempts = 5;

      sendMsg('status', {
        platform: platformKey,
        message: `🎯 【${cfg.name}】[批次 ${batchIdx + 1} · 维度 ${taskIdx + 1}/${quotaTasks.length}] 正在检索「${currentKeyword}」（本批目标: ${currentTarget} 人）...`
      });

      // 验证码检测
      const preCheck = await waitForCaptchaResolved(page, platformKey, cfg);
      if (!preCheck) return outcome(allPlatformSavedList, '安全验证未完成');

      // 1. 自动执行平台寻路与关键词检索
      if (activeKeyword !== currentKeyword) {
        if (!await autoNavigateAndSearch(page, platformKey, cfg, currentKeyword)) {
          return outcome(allPlatformSavedList, '未能提交关键词搜索');
        }
        activeKeyword = currentKeyword;
      }

      // 已读或已收录的卡片不占新增配额；不足时继续翻页/滚动补抓。
      while (allPlatformSavedList.length - taskStartCount < currentTarget && pageAttempts <= maxPageAttempts) {
        const remainingForTask = currentTarget - (allPlatformSavedList.length - taskStartCount);

      // 2. 动态弹性轮询（最长 35 秒，每 1.5 秒微步滚轮并检测候选人卡片）
      let scraped = [];
      const pollStart = Date.now();
      const maxPollMs = quotaTasks.length > 1 ? 30000 : 45000;
      let pollAttempts = 0;

      while (Date.now() - pollStart < maxPollMs) {
        pollAttempts++;
        try {
          if (!browser.isConnected()) {
            sendMsg('status', { platform: platformKey, message: `【${cfg.name}】浏览器窗口已关闭` });
            return outcome(allPlatformSavedList, '浏览器窗口已关闭');
          }
        } catch (e) {}

        // 每次轮询探测验证码
        const inPollCheck = await waitForCaptchaResolved(page, platformKey, cfg);
        if (!inPollCheck) return outcome(allPlatformSavedList, '安全验证未完成');

        try {
          // 微步滚轮触发瀑布流加载
          await smoothScroll(page, 450, 80).catch(() => {});
          await new Promise(r => setTimeout(r, 800));

          scraped = await extractCandidatesAcrossFrames(
            page,
            Math.max(remainingForTask * 5, 60),
            currentKeyword,
            cfg.name,
            Array.from(projectExcludedNames),
            Array.from(projectExcludedUrls),
            { city: options.city, exp: options.exp, edu: options.edu }
          );
          scraped = selectFreshCandidates(scraped, remainingForTask, projectExcludedUrls, seenCandidateKeys);

          if (scraped && scraped.length > 0) {
            if (scraped.length >= remainingForTask || pollAttempts >= 2) {
              sendMsg('status', {
                platform: platformKey,
                message: `🎯 【${cfg.name}】关键词「${currentKeyword}」捕获 ${scraped.length} 位全新在线候选人（已自动排重），正在解析整理...`
              });
              break; // 成功找到该关键词的候选人，跳出轮询
            }
          }

        } catch (evalErr) {}

        // 如果仍在 51job 工作台且过了 6 秒仍未进入搜索页，自动重试跳转
        if (platformKey === '51job' && pollAttempts === 3) {
          const curUrl = page.url() || '';
          if (curUrl.toLowerCase().includes('navigate') || curUrl.endsWith('.com/') || curUrl.endsWith('.com')) {
            sendMsg('status', {
              platform: platformKey,
              message: `🔄 【前程无忧】正在从工作台自动直跳「人才搜索」中心...`
            });
            await page.goto('https://ehire.51job.com/Revision/talent/search', {
              waitUntil: 'domcontentloaded',
              timeout: 20000
            }).catch(() => {});
            await new Promise(r => setTimeout(r, 2000));
            if (!await autoNavigateAndSearch(page, platformKey, cfg, currentKeyword)) {
              return outcome(allPlatformSavedList, '重试时未能提交关键词搜索');
            }
          }
        }
        if (pollAttempts >= 3) break;

        // 周期性提醒用户后台正在持续守候
        if (pollAttempts % 5 === 0) {
          sendMsg('status', {
            platform: platformKey,
            message: `⏳ 【${cfg.name}】正在实时守候候选人数据渲染...（您也可在打开的 Edge 窗口中切换岗位或点击搜索）`
          });
        }

        await new Promise(r => setTimeout(r, 1500));
      }

      if (!scraped || scraped.length === 0) {
        sendMsg('status', {
          platform: platformKey,
          message: `ℹ️ 【${cfg.name}】当前列表无新候选人，继续向下加载（本轮 ${pageAttempts}/${maxPageAttempts}）。`
        });
        if (pageAttempts >= maxPageAttempts) break;
        if (!await advanceSearchResults(page)) {
          exhaustedKeywords.add(currentKeyword);
          sendMsg('status', { platform: platformKey, message: `【${cfg.name}】「${currentKeyword}」列表及滚动位置均未变化，停止重复扫描。` });
          break;
        }
        pageAttempts++;
        continue;
      }

      // 执行详情穿透：逐一提取抽屉/新标签页中的全量工作经历与完整个人优势
      if (!await waitForCaptchaResolved(page, platformKey, cfg)) return outcome(allPlatformSavedList, '安全验证未完成');
      scraped = await enrichCandidatesWithFullDetail(page, browser, scraped, platformKey, cfg);
      if (scraped === null) return outcome(allPlatformSavedList, '详情页安全验证未完成');

      // 整理并存储
      for (let i = 0; i < scraped.length; i++) {
        if (allPlatformSavedList.length >= targetCount) break;

        const item = scraped[i];
        item.sourceKeyword = currentKeyword;

        // 跨渠道排重
        const urlText = String(item.url || '');
        const urlLower = urlText.toLowerCase();
        const candidateUrl = urlText && !/\/talent\/search|\/search|\/recommend|\/navigate/.test(urlLower) &&
          /id=|seq=|user|resume|detail|geek/.test(urlLower) ? urlText : '';
        const isDup = candidateUrl
          ? projectExcludedUrls.has(candidateUrl) || seenCandidateKeys.has(`url:${candidateUrl}`)
          : isDuplicateCandidate(item.name, item.workText, item.infoText);
        if (isDup) {
          if (item.cardKey) seenCandidateKeys.add(item.cardKey);
          continue;
        }
        const dupTag = '';

        const candID = `${platformKey}_${Date.now()}_${allPlatformSavedList.length}_${i}`;

        // 净化正文与经历（剔除水印网格、举报、免责声明等）
        const cleanedRawText = cleanCandidateResumeText(item.rawCardText);
        const cleanedWorkText = cleanCandidateResumeText(item.workText);
        const cleanedAdvantage = cleanCandidateResumeText(item.advantage);

        // 智能提取候选人真实邮箱
        const emailRegex = /([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})/;
        const emailMatch = (cleanedRawText || '').match(emailRegex);
        const candidateEmail = emailMatch ? emailMatch[1] : '';

        const formattedContent = `【${cfg.name} 真实推荐牛人档案】${dupTag}
姓名 / 称谓：${item.name}
来源渠道：${cfg.name}
检索岗位：${options.keyword}
分流同义词：${currentKeyword}（${task.categoryName}）
目标城市：${options.city}
基本画像：${item.infoText || '详见卡片信息'}
任职履历快照：${cleanedWorkText || '详见卡片完整信息'}
在线直达网址：${candidateUrl || '（平台内嵌卡片推荐，可于企业工作台直接联系）'}
联系方式：${candidateEmail ? candidateEmail : '平台默认隐私保护（需通过在线打招呼或索取完整简历获取）'}
${cleanedAdvantage ? `\n【个人综合优势】\n${cleanedAdvantage}\n` : ''}
【核心专业技能】
${item.skills && item.skills.length > 0 ? item.skills.map(s => '• ' + s).join('\n') : '• 岗位专业技能'}

【${cfg.name} 在线微简历完整正文】
${cleanedRawText}
`;

        const fileName = `【${cfg.name}】${item.name}_${currentKeyword}.txt`;
        const filePath = path.join(options.dataDir, fileName);
        fs.writeFileSync(filePath, formattedContent, 'utf8');

        const candData = {
          id: candID,
          platform: platformKey,
          platformName: cfg.name,
          fileName,
          filePath,
          name: item.name,
          url: candidateUrl,
          email: candidateEmail,
          jobTitle: options.keyword,
          sourceKeyword: currentKeyword,
          experience: item.infoText || '在线经验',
          education: '详见微简历',
          company: cleanedWorkText || '行业企业',
          skills: item.skills || [],
          content: formattedContent
        };

        allPlatformSavedList.push(candData);
        if (savedPerKeyword.length) savedPerKeyword[taskIdx]++;

        // 立即记录排重集合，防止后续关键词重复抓取同一人
        if (candData.name) {
          projectExcludedNames.add(candData.name.trim());
          seenCandidateKeys.add(candData.name.trim());
        }
        if (item.cardKey) seenCandidateKeys.add(item.cardKey);
        if (candData.url) {
          projectExcludedUrls.add(candData.url.trim());
          seenCandidateKeys.add(`url:${candData.url.trim()}`);
        }

        sendMsg('candidate', {
          platform: platformKey,
          platformName: cfg.name,
          current: allPlatformSavedList.length,
          total: targetCount,
          candidate: candData
        });

        await new Promise(r => setTimeout(r, 300));
      }
      if (allPlatformSavedList.length - taskStartCount < currentTarget) {
        sendMsg('status', {
          platform: platformKey,
          message: `↪️ 【${cfg.name}】「${currentKeyword}」已新增 ${allPlatformSavedList.length - taskStartCount}/${currentTarget} 人，继续翻找缺额。`
        });
      }
      }
      if (allPlatformSavedList.length - taskStartCount < currentTarget) {
        sendMsg('status', {
          platform: platformKey,
          message: `ℹ️ 【${cfg.name}】「${currentKeyword}」本轮已检查 ${pageAttempts} 次，实际新增 ${allPlatformSavedList.length - taskStartCount}/${currentTarget} 人；其余卡片重复或不满足筛选。`
        });
      }
    }

    if (allPlatformSavedList.length === batchStartCount) emptyBatches++;
    else emptyBatches = 0;
    batchIdx++;
    if (quotaTasks.every(task => task.targetCount <= 0 || exhaustedKeywords.has(task.keyword))) break;

    // 子批次间短暂停顿；缺额由下一批继续搜索。
    if (batchIdx < maxBatches && allPlatformSavedList.length < targetCount) {
      if (!await waitForCaptchaResolved(page, platformKey, cfg)) return outcome(allPlatformSavedList, '安全验证未完成');
      const pauseMs = gaussianRandom(2500, 800, 1500, 4500);
      sendMsg('status', {
        platform: platformKey,
        message: `第 ${batchIdx} 批结束，当前累计 ${allPlatformSavedList.length}/${targetCount} 人；稍后继续补足缺额 (${(pauseMs / 1000).toFixed(1)}s)...`
      });
      // 模拟人类回看与轻微滚动
      await smoothScroll(page, -140, 40).catch(() => {});
      await new Promise(r => setTimeout(r, Math.floor(pauseMs * 0.4)));
      await smoothScroll(page, 200, 50).catch(() => {});
      await new Promise(r => setTimeout(r, Math.floor(pauseMs * 0.6)));
      if (!await waitForCaptchaResolved(page, platformKey, cfg)) return outcome(allPlatformSavedList, '安全验证未完成');
    }
  }

  const stopReason = allPlatformSavedList.length >= targetCount ? ''
    : exhaustedKeywords.size > 0 ? '列表与滚动位置均未变化，可能已到结果末尾'
    : batchIdx >= maxBatches ? `已达到本次安全批次数上限（连续 ${emptyBatches} 批无新增）`
    : '当前筛选条件下没有更多可确认的新候选人';
  sendMsg('status', {
    platform: platformKey,
    message: `【${cfg.name}】检索结束，实际抓取并导入 ${allPlatformSavedList.length}/${targetCount} 位候选人档案${stopReason ? `；原因：${stopReason}` : ''}。`
  });

  return outcome(allPlatformSavedList, stopReason);
  } finally {
    if (backgroundSession) {
      await backgroundSession.send('Emulation.setFocusEmulationEnabled', { enabled: false }).catch(() => {});
      await backgroundSession.detach().catch(() => {});
    }
    try { await browser.disconnect(); } catch (e) {}
  }
}

// 主入口
async function main() {
  const browserPath = findBrowserExecutable();
  if (!browserPath) {
    sendMsg('error', { message: '❌ 未在系统中检测到 Edge 或 Chrome 浏览器，无法启动直连引擎。' });
    return;
  }

  if (!fs.existsSync(options.dataDir)) {
    fs.mkdirSync(options.dataDir, { recursive: true });
  }

  // 独立测试某平台登录
  if (options.testLoginPlatform) {
    const cfg = PLATFORM_CONFIGS[options.testLoginPlatform];
    if (!cfg) {
      sendMsg('error', { message: `未知平台: ${options.testLoginPlatform}` });
      return;
    }
    await scrapePlatform(options.testLoginPlatform, browserPath, 0);
    sendMsg('done', { total: 0, message: `【${cfg.name}】登录态测试完毕` });
    return;
  }

  // 候选人自动化交互动作 (打招呼/索要简历/交换微信/标为不合适)
  if (options.action) {
    const actionLabels = {
      greet: '打招呼 / 发送沟通意向',
      ask_resume: '索要完整附件简历',
      exchange_wechat: '请求交换微信',
      mark_unfit: '标记为不合适'
    };
    const actionName = actionLabels[options.action] || options.action;
    // 纯净化候选人姓名，去掉【前程无忧】等外包前缀
    const rawName = options.candidateName || '';
    const cleanName = rawName.replace(/^【.*?】/, '').replace(/^BOSS牛人_/, '').split('_')[0].trim();

    if (!cleanName || !['greet', 'ask_resume', 'exchange_wechat', 'mark_unfit'].includes(options.action)) {
      sendMsg('action_result', { success: false, action: options.action, candidateName: cleanName, message: '缺少明确候选人姓名或操作类型不受支持' });
      return;
    }

    let liveTriggered = false;
    let liveMsg = '';

    // 尝试连接正在运行的浏览器端口进行真实页面同步交互（优先 51job 9503 与 BOSS 9501）
    for (const port of [9503, 9501, 9502, 9504]) {
      try {
        const browser = await puppeteer.connect({
          browserURL: `http://127.0.0.1:${port}`,
          defaultViewport: null
        });
        let pages = await browser.pages();

        // 若提供了 candidateUrl 且当前没有打开该简历页面，主动在该平台浏览器中打开
        if (options.candidateUrl && (
          (port === 9503 && options.candidateUrl.includes('51job')) ||
          (port === 9501 && options.candidateUrl.includes('zhipin')) ||
          (port === 9502 && options.candidateUrl.includes('zhaopin')) ||
          (port === 9504 && options.candidateUrl.includes('liepin'))
        )) {
          const hasUrl = pages.some(p => p.url() === options.candidateUrl);
          if (!hasUrl) {
            try {
              const newP = await browser.newPage();
              await newP.goto(options.candidateUrl, { waitUntil: 'domcontentloaded', timeout: 15000 });
              await new Promise(r => setTimeout(r, 1500));
              pages = await browser.pages();
            } catch (e) {}
          }
        }

        for (const p of pages) {
          const u = p.url() || '';
          if (u.includes('zhipin.com') || u.includes('zhaopin.com') || u.includes('liepin.com') || u.includes('51job.com')) {
            const framesToSearch = [p, ...p.frames().filter(f => f !== p.mainFrame())];
            for (const f of framesToSearch) {
              const clickRes = await f.evaluate((act, name, expectedUrl, pageUrl) => {
                const triggerClick = (targetEl) => {
                  if (typeof targetEl.click === 'function') {
                    targetEl.click();
                  }
                };

                // 1. 查找匹配候选人的卡片
                const cardSelectors = [
                  '.talent-search-container .card',
                  'div.card',
                  '.eh-talent-search .card',
                  '.candidate-card-wrap',
                  '.candidate-card',
                  '.card-inner',
                  '.recommend-card',
                  '.resume-item',
                  '.chat-user-item',
                  '.geek-item',
                  '.candidate-box'
                ];
                const cards = Array.from(document.querySelectorAll(cardSelectors.join(', ')));
                const normalize = value => { try { const u = new URL(value); u.hash = ''; return u.href; } catch { return ''; } };
                const exactName = card => {
                  const el = card.querySelector('.firstline .name, .candidate-name, .geek-name, .user-name, span.name, td.name, h3, h4');
                  return el && (el.innerText || '').trim().split('\n')[0].trim() === name;
                };
                const matches = cards.filter(card => {
                  if (!exactName(card)) return false;
                  if (!expectedUrl) return true;
                  const links = Array.from(card.querySelectorAll('a[href]'));
                  return links.some(link => normalize(link.href) === normalize(expectedUrl));
                });
                let targetContainer = matches.length === 1 ? matches[0] : null;
                if (!targetContainer && expectedUrl && normalize(pageUrl) === normalize(expectedUrl)) {
                  const detail = document.querySelector('.el-drawer, .resume-detail, [class*="resume-detail"], [class*="candidate-detail"]') || document.body;
                  const names = Array.from(detail.querySelectorAll('.candidate-name, .geek-name, .user-name, span.name, h1, h2, h3'));
                  if (names.some(el => (el.innerText || '').trim() === name)) targetContainer = detail;
                }
                if (!targetContainer) return { ok: false, detail: '未能唯一确认候选人身份' };

                if (targetContainer) {
                  if (act === 'greet' || act === 'ask_resume' || act === 'exchange_wechat') {
                    // 全渠道打招呼关键词匹配
                    // 51job: 立即Hi聊, Hi聊, .talk_btn
                    // Boss: 打招呼, 继续沟通, .btn-greet
                    // 智联: 聊一聊, .btn-chat
                    // 猎聘: 立即沟通, 打招呼, .btn-contact
                    const clickables = Array.from(targetContainer.querySelectorAll('button, div, span, a'));
                    const actionKeywords = {
                      greet: ['立即hi聊', 'hi聊', '打招呼', '聊一聊', '立即沟通'],
                      ask_resume: ['索要简历', '请求简历', '索取简历'],
                      exchange_wechat: ['交换微信', '请求微信', '索取微信']
                    };
                    const keywords = actionKeywords[act] || [];
                    
                    const btn = clickables.find(el => {
                      const t = (el.innerText || '').trim().toLowerCase();
                      const cls = (el.className || '').toLowerCase();
                      const isVis = el.offsetParent !== null || el.getClientRects().length > 0;
                      if (!isVis) return false;
                      if (t.length > 15) return false; // 排除长段落
                      return keywords.some(kw => t === kw) || (act === 'greet' && (cls.includes('talk_btn') || cls.includes('btn-greet')));
                    });

                    if (btn) {
                      btn.scrollIntoView({ behavior: 'smooth', block: 'center' });
                      triggerClick(btn);

                      return { ok: true, detail: `已定位本人并点击「${btn.innerText.trim()}」，请核对平台后续确认结果` };
                    }
                  } else if (act === 'mark_unfit') {
                    const unfitBtn = Array.from(targetContainer.querySelectorAll('button, a, [title]'))
                      .find(el => (el.innerText || '').trim() === '不合适' || (el.getAttribute('title') || '') === '不合适');
                    if (unfitBtn) {
                      triggerClick(unfitBtn);
                      return { ok: true, detail: '已定位本人并点击不合适按钮，请核对平台状态' };
                    }
                  }
                }
                return { ok: false };
              }, options.action, cleanName, options.candidateUrl, u).catch(() => ({ ok: false }));

              if (clickRes.ok) {
                liveTriggered = true;
                liveMsg = `（已在正在运行的浏览器工作台中${clickRes.detail}）`;
                // 将页面置顶激活，让用户能直观看到弹出的沟通窗口/对话输入框
                await p.bringToFront().catch(() => {});
                await new Promise(r => setTimeout(r, 1200));
                break;
              }
            }
          }
          if (liveTriggered) break;
        }
        if (liveTriggered) break;
      } catch (e) {}
    }

    sendMsg('action_result', {
      success: liveTriggered,
      action: options.action,
      candidateName: cleanName,
      message: liveTriggered
        ? `已对候选人【${cleanName}】执行「${actionName}」的页面点击。${liveMsg}请核对平台最终状态。`
        : `未能唯一确认【${cleanName}】或定位对应按钮，操作未执行。请打开本人详情页后重试。`
    });
    return;
  }

  sendMsg('status', {
    message: `🚀 启动全渠道聚合检索引擎，计划并发调度平台：${options.platforms.map(p => PLATFORM_CONFIGS[p]?.name || p).join('、')}，目标岗位「${options.keyword}」...`
  });

  let allResults = [];
  let errorCount = 0;
  let connectedPlatforms = [];
  let shortfallReasons = [];
  const globalKeywordTargets = Array.isArray(options.quotaMatrix) && options.quotaMatrix.length
    ? allocateQuota(options.count, options.quotaMatrix) : null;

  // 全渠道共用一个总额。串行分配可依据前一平台实际产出补足后续额度。
  for (let i = 0; i < options.platforms.length && allResults.length < options.count && !safetyStopped; i++) {
    const plat = options.platforms[i];
    const remaining = options.count - allResults.length;
    const planned = Math.ceil(remaining / (options.platforms.length - i));
    const remainingKeywords = globalKeywordTargets && globalKeywordTargets.map((quota, idx) =>
      Math.max(0, quota - allResults.filter(item => item.sourceKeyword === options.quotaMatrix[idx].keyword).length));
    const keywordPlan = remainingKeywords && allocateQuota(planned, remainingKeywords.map(ratio => ({ ratio })));
    const res = await scrapePlatform(plat, browserPath, planned, keywordPlan);
    if (res === null) {
      // 该平台启动/连接失败
      errorCount++;
    } else {
      connectedPlatforms.push(plat);
      allResults = allResults.concat(res.candidates.slice(0, remaining));
      if (res.reason) shortfallReasons.push(`${PLATFORM_CONFIGS[plat]?.name || plat}：${res.reason}`);
    }
  }

  if (safetyStopped) {
    sendMsg('error', { message: '安全验证未完成，检索已暂停。已采集的候选人保留，请处理验证后重新搜索。' });
  } else if (errorCount > 0 && connectedPlatforms.length === 0) {
    // 所有平台都失败了 → 不发 done，发 error
    sendMsg('error', {
      message: `❌ 所有选定平台 (${options.platforms.length} 个) 均连接失败。请先关闭所有已打开的 Edge 浏览器窗口，然后重试。`
    });
  } else {
    const shortfallReason = allResults.length < options.count ? shortfallReasons.join('；') : '';
    sendMsg('done', {
      total: allResults.length,
      connectedPlatforms: connectedPlatforms.length,
      errorPlatforms: errorCount,
      shortfallReason,
      message: allResults.length > 0
        ? `全渠道检索结束，采集 ${allResults.length}/${options.count} 份人才档案${shortfallReason ? `；未达目标原因：${shortfallReason}` : ''}${options.autoAnalyze ? '，将启动 AI 评估' : '，等待人工启动评估'}。`
        : `⚠️ 已成功连接 ${connectedPlatforms.length} 个平台，但未发现匹配的候选人卡片。请在浏览器中手动搜索后重试。`
    });
  }
}

if (require.main === module) {
  main().catch(err => {
    sendMsg('error', { message: `❌ 聚合引擎运行异常: ${err.message}` });
  });
}

module.exports = { allocateQuota, smoothScroll, keepPageActiveInBackground, setBrowserWindowState, createBackgroundPage, capture51jobDetailUrl, isBrowserStartPage, browserLaunchArgs, closeUnusedStartPages, isPlatformPageUrl, ensurePlatformPage, submit51jobSearch, detectCaptcha, extractCandidatesAcrossFrames, selectFreshCandidates, advanceSearchResults, main };
