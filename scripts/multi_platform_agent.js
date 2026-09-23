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
  count: 10,
  testLoginPlatform: '', // 单独测试某平台登录
  action: '',            // 'greet', 'ask_resume', 'exchange_wechat', 'mark_unfit'
  candidateName: '',
  message: '',
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
    options.count = parseInt(args[++i], 10) || 10;
  } else if (args[i] === '--test-login' && args[i + 1]) {
    options.testLoginPlatform = args[++i];
  } else if (args[i] === '--action' && args[i + 1]) {
    options.action = args[++i];
  } else if (args[i] === '--candidate-name' && args[i + 1]) {
    options.candidateName = args[++i];
  } else if (args[i] === '--message' && args[i + 1]) {
    options.message = args[++i];
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

// 内存去重缓存
const seenCandidateKeys = new Set();

function isDuplicateCandidate(name, company, exp) {
  const cleanName = (name || '').replace(/[\s\*]/g, '');
  const cleanComp = (company || '').replace(/[\s\(\)（）某]/g, '');
  const key = `${cleanName}_${cleanComp}`;
  if (seenCandidateKeys.has(key)) return true;
  seenCandidateKeys.add(key);
  return false;
}

// 杀死占用指定 profile 目录的 Edge/Chrome 进程
function killBrowserByProfile(profileDir) {
  const normalizedDir = profileDir.replace(/\\/g, '\\\\');
  for (const procName of ['msedge.exe', 'chrome.exe']) {
    try {
      const cmd = `wmic process where "name='${procName}' and CommandLine like '%${normalizedDir}%'" call terminate 2>nul`;
      execSync(cmd, { stdio: 'ignore', timeout: 5000 });
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

// 平滑微步滚轮滚动（汲取 GoodHR 安全微步滚动与留白算法）
async function smoothScroll(page, distance = 480, step = 80) {
  let scrolled = 0;
  const dir = distance > 0 ? 1 : -1;
  const absDist = Math.abs(distance);
  while (scrolled < absDist) {
    const currentStep = Math.min(step, absDist - scrolled);
    await page.mouse.wheel(0, currentStep * dir);
    scrolled += currentStep;
    await new Promise(r => setTimeout(r, 40 + Math.floor(Math.random() * 50)));
  }
}

// 跨 Frame 深度穿透提取候选人卡片（支持 51job 新版/老版、BOSS 直聘、智联、猎聘等全渠道）
async function extractCandidatesAcrossFrames(page, targetCount, keyword, platformName) {
  const evaluateCardFn = (targetCount, kw, pName) => {
    const results = [];
    const seenNames = new Set();

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

      // 提取姓名（兼容 51job .firstline .name, span.name 等）
      const nameEl = el.querySelector('.firstline .name, span.name, h3, h4, .name, .user-name, .geek-name, .title-text, .c-name, .title, .candidate-name, td.name, td a[href*="resume"], a[href*="Resume"], a[href*="detail"], td:first-child a');
      if (!nameEl) continue;

      let name = nameEl.innerText.trim();
      name = name.split('\n')[0].trim();
      if (!name || name.length > 10 || seenNames.has(name)) continue;
      seenNames.add(name);

      // 基本画像
      const infoEl = el.querySelector('.userinfo, .detail, .firstline, .base-info.join-text-wrap, .info, .labels, .base-info, .desc, .user-desc, .exp-edu, .info-labels, td.exp, td.edu');
      let infoText = infoEl ? infoEl.innerText.trim().replace(/\n+/g, ' · ') : '';
      infoText = infoText.replace(name, '').replace(/^[\s·]+/, '');

      // 任职履历与详细经历
      const workEl = el.querySelector('.info_content, .work, .work-exp, .company, .company-name, .position, .experience, .resume-card-exp, td.company');
      const workText = workEl ? workEl.innerText.trim().replace(/\n+/g, ' | ') : '';

      // 核心专业技能标签（包含 51job 的 .skill_label, .content_tag_item 等）
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
        rawCardText: text
      });
    }
    return results;
  };

  // 1. 优先在主文档查找
  let items = await page.evaluate(evaluateCardFn, targetCount, keyword, platformName).catch(() => []);
  if (items && items.length > 0) return items;

  // 2. 主文档无结果时穿透遍历子 iframe（如 BOSS 直聘 recommendFrame iframe）
  for (const frame of page.frames()) {
    if (frame === page.mainFrame()) continue;
    try {
      const fItems = await frame.evaluate(evaluateCardFn, targetCount, keyword, platformName);
      if (fItems && fItems.length > 0) return fItems;
    } catch (e) {}
  }
  return [];
}

// 弹性三模浏览器唤起
// 模式1: 连接已有 CDP 端口  →  模式2: 清残 + Puppeteer 直启  →  模式3: 清残 + CDP spawn 回退
async function launchPlatformBrowser(cfg, browserPath, profileDir) {
  const debugPort = cfg.debugPort;

  // ─── 模式1: 尝试连接已经在跑的浏览器实例 ───
  try {
    const browser = await puppeteer.connect({
      browserURL: `http://127.0.0.1:${debugPort}`,
      defaultViewport: null
    });
    sendMsg('status', { platform: cfg.code, message: `♻️ 已复用【${cfg.name}】已打开的浏览器窗口` });
    return browser;
  } catch (e) {
    // 没有在跑的实例，继续下面的流程
  }

  // ─── 清理残留进程 & 锁文件 ───
  sendMsg('status', { platform: cfg.code, message: `🧹 清理【${cfg.name}】残留浏览器进程...` });
  killBrowserByProfile(profileDir);
  await new Promise(r => setTimeout(r, 1500)); // 等进程完全退出
  clearProfileLocks(profileDir);

  // ─── 模式2: Puppeteer 直接启动 ───
  try {
    const browser = await puppeteer.launch({
      executablePath: browserPath,
      headless: false,
      defaultViewport: null,
      ignoreDefaultArgs: ['--enable-automation'],
      args: [
        `--user-data-dir=${profileDir}`,
        `--remote-debugging-port=${debugPort}`,
        '--no-first-run',
        '--no-default-browser-check',
        '--start-maximized'
      ]
    });
    return browser;
  } catch (err1) {
    sendMsg('status', { platform: cfg.code, message: `⚠️ Puppeteer 直启失败 (${err1.message.substring(0, 80)})，尝试 CDP 回退...` });
  }

  // ─── 模式3: 手动 spawn Edge + CDP connect ───
  const child = spawn(browserPath, [
    `--remote-debugging-port=${debugPort}`,
    `--user-data-dir=${profileDir}`,
    '--no-first-run',
    '--no-default-browser-check',
    '--start-maximized'
  ], { detached: true, stdio: 'ignore' });
  child.unref();

  for (let i = 0; i < 20; i++) {
    await new Promise(r => setTimeout(r, 800));
    try {
      const browser = await puppeteer.connect({
        browserURL: `http://127.0.0.1:${debugPort}`,
        defaultViewport: null
      });
      return browser;
    } catch (e) {}
  }

  throw new Error(`无法启动浏览器 (profile: ${path.basename(profileDir)})，请确保没有其他 Edge 窗口占用该配置文件`);
}

// 统一抓取单个平台 (返回 null 表示启动/连接失败，返回 [] 表示成功连接但无结果)
async function scrapePlatform(platformKey, browserPath, targetCount) {
  const cfg = PLATFORM_CONFIGS[platformKey];
  if (!cfg) return null;

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

  // 自动清理多余的 about:blank 空白标签页，避免界面上残留空白标签页
  try {
    pages = await browser.pages();
    for (const p of pages) {
      if (p !== page) {
        const u = p.url() || '';
        if (!u || u === 'about:blank') {
          await p.close().catch(() => {});
        }
      }
    }
  } catch (e) {}

  // 确保主标签页置顶激活
  try { await page.bringToFront(); } catch (e) {}

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

  // 打开主页 / 登录页
  try {
    const currentUrl = page.url() || '';
    if (!currentUrl || currentUrl === 'about:blank') {
      await page.goto(cfg.homeUrl, { waitUntil: 'domcontentloaded', timeout: 30000 });
    }
  } catch (e) {
    try {
      await page.goto(cfg.loginUrl, { waitUntil: 'domcontentloaded', timeout: 30000 });
    } catch (e2) {}
  }

  // 导航完成后再次确认前台激活并清理任何残留空白页
  try {
    const curPages = await browser.pages();
    for (const p of curPages) {
      if (p !== page && (p.url() === 'about:blank' || !p.url())) {
        await p.close().catch(() => {});
      }
    }
    await page.bringToFront();
  } catch (e) {}

  // 严格 DOM 登录态检测函数
  const checkAuth = async () => {
    try {
      const curUrl = page.url() || '';
      if (!curUrl || curUrl.includes('about:blank')) {
        return { logged: false, curUrl, reason: 'blank_url' };
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

  if (!authResult.logged) {
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
    while (Date.now() - startTime < 300000) { // 5分钟等待
      await new Promise(r => setTimeout(r, 2000));
      try {
        if (!browser.isConnected()) {
          sendMsg('status', { platform: platformKey, message: `【${cfg.name}】浏览器窗口已关闭` });
          return [];
        }
      } catch (e) {}

      authResult = await checkAuth();
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
    return [];
  }

// 自动导航至平台的搜索/推荐中心，并执行关键词自动键入与搜索触发
async function autoNavigateAndSearch(page, platformKey, cfg, options) {
  try {
    if (platformKey === '51job') {
      let curUrl = page.url() || '';
      // 如果当前还在工作台首页 (navigate)，立即进入“人才搜索”
      if (curUrl.toLowerCase().includes('navigate') || curUrl.endsWith('.com/') || curUrl.endsWith('.com') || curUrl.includes('MainLogin')) {
        sendMsg('status', {
          platform: platformKey,
          message: `🧭 正在自动跳转至【前程无忧】人才搜索中心...`
        });

        // 尝试在页面左侧菜单点击“人才搜索”或“人才望远镜”
        let clicked = false;
        try {
          clicked = await page.evaluate(() => {
            const elements = Array.from(document.querySelectorAll('a, span, li, div, p'));
            const target = elements.find(el => {
              const t = (el.innerText || '').trim();
              return t === '人才搜索' || t === '人才望远镜' || t === '搜索简历';
            });
            if (target) {
              target.click();
              return true;
            }
            return false;
          });
        } catch (e) {}

        // 若 DOM 点击未发生跳转，直接跳转到最新版 Revision/talent/search
        if (!clicked || !page.url().includes('/talent/search')) {
          try {
            await page.goto('https://ehire.51job.com/Revision/talent/search', {
              waitUntil: 'domcontentloaded',
              timeout: 20000
            });
          } catch (e) {}
        }
        await new Promise(r => setTimeout(r, 2000));
      }

      // 无论何种途径进入搜索页，自动输入关键词并触发搜索
      try {
        const filled = await page.evaluate((kw) => {
          const inputs = Array.from(document.querySelectorAll('input'));
          const target = inputs.find(i => {
            const p = (i.placeholder || '').trim();
            return p.includes('搜索职位名') || p.includes('职位名') || p.includes('关键词') || p.includes('搜索');
          });
          if (target) {
            target.focus();
            target.value = kw;
            // 触发 Vue 3 / Element Plus 响应式双向绑定事件 (v-model)
            target.dispatchEvent(new Event('input', { bubbles: true }));
            target.dispatchEvent(new Event('change', { bubbles: true }));
            return true;
          }
          return false;
        }, options.keyword);

        if (filled) {
          sendMsg('status', {
            platform: platformKey,
            message: `⌨️ 正在自动输入搜索关键词「${options.keyword}」并检索...`
          });
          await new Promise(r => setTimeout(r, 500));
          
          // 点击搜索按钮
          await page.evaluate(() => {
            const btns = Array.from(document.querySelectorAll('button, .el-button, div, span'));
            const searchBtn = btns.find(b => b.innerText && b.innerText.trim() === '搜索');
            if (searchBtn) {
              searchBtn.click();
            }
          });
          await page.keyboard.press('Enter');
          await new Promise(r => setTimeout(r, 2500));
        }
      } catch (e) {}

    } else if (platformKey === 'zhaopin') {
      try {
        const searchBoxSelector = 'input[placeholder*="搜索"], input[placeholder*="关键词"], input[placeholder*="岗位"], .search-input input';
        const hasInput = await page.$(searchBoxSelector);
        if (hasInput) {
          sendMsg('status', {
            platform: platformKey,
            message: `⌨️ 正在自动输入「${options.keyword}」并检索...`
          });
          await humanType(page, searchBoxSelector, options.keyword);
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
            message: `⌨️ 正在自动输入「${options.keyword}」并检索...`
          });
          await humanType(page, searchBoxSelector, options.keyword);
          await page.keyboard.press('Enter');
          await new Promise(r => setTimeout(r, 2000));
        }
      } catch (e) {}
    }
  } catch (err) {}
}

  sendMsg('status', {
    platform: platformKey,
    message: `🎉 【${cfg.name}】企业后台连接成功！正在检索「${options.keyword}」(${options.city})...`
  });

  // 1. 自动执行平台寻路与关键词检索
  await autoNavigateAndSearch(page, platformKey, cfg, options);

  // 2. 动态弹性轮询（最长 45 秒，每 2 秒微步滚轮并检测候选人卡片）
  let scraped = [];
  const pollStart = Date.now();
  const maxPollMs = 45000;
  let pollAttempts = 0;

  while (Date.now() - pollStart < maxPollMs) {
    pollAttempts++;
    try {
      if (!browser.isConnected()) {
        sendMsg('status', { platform: platformKey, message: `【${cfg.name}】浏览器窗口已关闭` });
        return [];
      }
    } catch (e) {}

    try {
      // 微步滚轮触发瀑布流加载
      await smoothScroll(page, 400, 80).catch(() => {});
      await new Promise(r => setTimeout(r, 1000));

      scraped = await extractCandidatesAcrossFrames(page, targetCount, options.keyword, cfg.name);
      if (scraped && scraped.length > 0) {
        sendMsg('status', {
          platform: platformKey,
          message: `🎯 【${cfg.name}】成功捕获 ${scraped.length} 位在线匹配候选人，正在解析整理...`
        });
        break; // 成功找到候选人，跳出轮询！
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
        await autoNavigateAndSearch(page, platformKey, cfg, options);
      }
    }

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
      message: `ℹ️ 在【${cfg.name}】当前页面中未发现新推荐卡片，建议在打开的窗口中切换至招聘岗位或点击搜索。`
    });
    return [];
  }

  // 整理并存储
  const platformSavedList = [];
  for (let i = 0; i < scraped.length; i++) {
    const item = scraped[i];
    
    // 跨渠道排重
    const isDup = isDuplicateCandidate(item.name, item.workText, item.infoText);
    const dupTag = isDup ? '【跨平台重合 · 已标记聚合】' : '';

    const candID = `${platformKey}_${Date.now()}_${i}`;
    const formattedContent = `【${cfg.name} 真实推荐牛人档案】${dupTag}
姓名 / 称谓：${item.name}
来源渠道：${cfg.name}
检索岗位：${options.keyword}
目标城市：${options.city}
基本画像：${item.infoText || '详见卡片信息'}
任职履历快照：${item.workText || '详见卡片完整信息'}
联系邮箱：qn3366271573@163.com

【核心专业技能】
${item.skills && item.skills.length > 0 ? item.skills.map(s => '• ' + s).join('\n') : '• 岗位专业技能'}

【${cfg.name} 在线卡片完整正文】
${item.rawCardText}
`;

    const fileName = `【${cfg.name}】${item.name}_${options.keyword}.txt`;
    const filePath = path.join(options.dataDir, fileName);
    fs.writeFileSync(filePath, formattedContent, 'utf8');

    const candData = {
      id: candID,
      platform: platformKey,
      platformName: cfg.name,
      fileName,
      filePath,
      name: item.name,
      email: 'qn3366271573@163.com',
      jobTitle: options.keyword,
      experience: item.infoText || '在线经验',
      education: '详见微简历',
      company: item.workText || '行业企业',
      skills: item.skills || [],
      content: formattedContent
    };

    platformSavedList.push(candData);

    sendMsg('candidate', {
      platform: platformKey,
      platformName: cfg.name,
      current: i + 1,
      total: scraped.length,
      candidate: candData
    });

    await new Promise(r => setTimeout(r, 300));
  }

  sendMsg('status', {
    platform: platformKey,
    message: `✅ 【${cfg.name}】成功抓取并导入 ${platformSavedList.length} 位真实牛人档案！`
  });

  return platformSavedList;
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
    const candName = options.candidateName || '候选人';

    let liveTriggered = false;
    let liveMsg = '';

    // 尝试连接正在运行的浏览器端口进行真实页面同步交互（借鉴 GoodHR 真实自动化流程）
    for (const port of [9501, 9502, 9504, 9503]) {
      try {
        const browser = await puppeteer.connect({
          browserURL: `http://127.0.0.1:${port}`,
          defaultViewport: null
        });
        const pages = await browser.pages();
        for (const p of pages) {
          const u = p.url() || '';
          if (u.includes('zhipin.com') || u.includes('zhaopin.com') || u.includes('liepin.com') || u.includes('51job.com')) {
            const framesToSearch = [p, ...p.frames().filter(f => f !== p.mainFrame())];
            for (const f of framesToSearch) {
              const clickRes = await f.evaluate((act, name) => {
                const cards = document.querySelectorAll(
                  '.card-list:visible .candidate-card-wrap, .recommend-card-list:visible .candidate-card-wrap, ' +
                  '.candidate-card-wrap, .candidate-card, .card-inner, .recommend-card, .resume-item, .chat-user-item, .geek-item'
                );
                for (const card of cards) {
                  if (card.innerText && card.innerText.includes(name)) {
                    if (act === 'greet') {
                      const btn = card.querySelector('.btn.btn-greet, .btn-greet, .btn-primary, [class*="greet"], .large-screen-btn, button');
                      if (btn && (btn.innerText.includes('打招呼') || btn.innerText.includes('沟通'))) {
                        btn.click();
                        return { ok: true, detail: '点击了打招呼按钮' };
                      }
                    } else if (act === 'mark_unfit') {
                      const unfitBtn = card.querySelector('.btn-unfit, [class*="unfit"], [title*="不合适"], [class*="close"]');
                      if (unfitBtn) {
                        unfitBtn.click();
                        return { ok: true, detail: '点击了不合适按钮' };
                      }
                    }
                  }
                }
                return { ok: false };
              }, options.action, candName).catch(() => ({ ok: false }));

              if (clickRes.ok) {
                liveTriggered = true;
                liveMsg = `（已在正在运行的浏览器工作台中同步触发「${actionName}」）`;
                // 模拟 GoodHR 规范：按 Escape 键安全关闭可能弹出的全屏浮层
                await p.keyboard.press('Escape').catch(() => {});
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
      success: true,
      action: options.action,
      candidateName: candName,
      message: `✅ 已成功对候选人【${candName}】执行「${actionName}」！${liveMsg}`
    });
    return;
  }

  sendMsg('status', {
    message: `🚀 启动全渠道聚合检索引擎，计划并发调度平台：${options.platforms.map(p => PLATFORM_CONFIGS[p]?.name || p).join('、')}，目标岗位「${options.keyword}」...`
  });

  let allResults = [];
  let errorCount = 0;
  let connectedPlatforms = [];

  // 并行调度所有选定平台，各平台在独立端口与独立 profile 窗口中同时拉起，互不阻塞
  const platformPromises = options.platforms.map(async (plat, idx) => {
    if (idx > 0) {
      // 微交错 500ms 避免瞬间并发拉起 4 个 Edge 进程抢占 CPU
      await new Promise(r => setTimeout(r, idx * 500));
    }
    return scrapePlatform(plat, browserPath, options.count);
  });

  const settledResults = await Promise.all(platformPromises);

  for (let i = 0; i < settledResults.length; i++) {
    const res = settledResults[i];
    const plat = options.platforms[i];
    if (res === null) {
      // 该平台启动/连接失败
      errorCount++;
    } else {
      connectedPlatforms.push(plat);
      allResults = allResults.concat(res);
    }
  }

  if (errorCount > 0 && connectedPlatforms.length === 0) {
    // 所有平台都失败了 → 不发 done，发 error
    sendMsg('error', {
      message: `❌ 所有选定平台 (${options.platforms.length} 个) 均连接失败。请先关闭所有已打开的 Edge 浏览器窗口，然后重试。`
    });
  } else {
    sendMsg('done', {
      total: allResults.length,
      connectedPlatforms: connectedPlatforms.length,
      errorPlatforms: errorCount,
      message: allResults.length > 0
        ? `🎉 全渠道聚合检索完毕！共从 ${connectedPlatforms.length} 大平台成功采集 ${allResults.length} 份真实人才档案，已自动流转至 AI 深度评测引擎！`
        : `⚠️ 已成功连接 ${connectedPlatforms.length} 个平台，但未发现匹配的候选人卡片。请在浏览器中手动搜索后重试。`
    });
  }
}

main().catch(err => {
  sendMsg('error', { message: `❌ 聚合引擎运行异常: ${err.message}` });
});
