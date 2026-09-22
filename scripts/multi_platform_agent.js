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
    homeUrl: 'https://www.zhipin.com/web/boss/recommend',
    profileFolder: 'boss_isolated_profile',
    debugPort: 9501
  },
  zhaopin: {
    name: '智联招聘',
    code: 'zhaopin',
    icon: '💼',
    loginUrl: 'https://passport.zhaopin.com/login',
    homeUrl: 'https://ihr.zhaopin.com/',
    profileFolder: 'zhaopin_isolated_profile',
    debugPort: 9502
  },
  '51job': {
    name: '前程无忧',
    code: '51job',
    icon: '📑',
    loginUrl: 'https://ehire.51job.com/MainLogin.aspx',
    homeUrl: 'https://ehire.51job.com/',
    profileFolder: '51job_isolated_profile',
    debugPort: 9503
  },
  liepin: {
    name: '猎聘网',
    code: 'liepin',
    icon: '🎯',
    loginUrl: 'https://lpt.liepin.com/user/login',
    homeUrl: 'https://lpt.liepin.com/',
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
      ignoreDefaultArgs: ['--enable-automation'],
      args: [
        `--user-data-dir=${profileDir}`,
        `--remote-debugging-port=${debugPort}`,
        '--no-first-run',
        '--no-default-browser-check',
        '--disable-infobars',
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
    '--start-maximized',
    cfg.homeUrl
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

  const pages = await browser.pages();
  const page = pages[0] || (await browser.newPage());

  try {
    const currentUrl = page.url() || '';
    if (!currentUrl.includes(cfg.code === '51job' ? '51job.com' : cfg.code) || currentUrl.includes('about:blank')) {
      await page.goto(cfg.homeUrl, { waitUntil: 'domcontentloaded', timeout: 30000 });
    }
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
          const hasLoginForm = !!document.querySelector('input[name*="user_login"], input[type="password"], .login-container, .login-box, .scan-box, .login-form');
          if (isLoginUrl || hasLoginForm) {
            return { logged: false, reason: 'login_form_present' };
          }
          const hasUserInfo = !!document.querySelector('.header-user-info, .user-name, .company-name, a[href*="logout"], .nav-user, .lpt-header-user, .user-avatar, .user-nav, .enterprise-info');
          const hasRecNav = text.includes('职位管理') || text.includes('人才搜索') || text.includes('沟通') || text.includes('候选人');
          return { logged: hasUserInfo || (url.includes('lpt.liepin.com') && hasRecNav), reason: 'ok' };
        }

        // BOSS直聘
        if (code === 'boss') {
          const isLoginUrl = url.includes('/login') || url.includes('/web/user');
          const hasLoginForm = !!document.querySelector('input[type="tel"], input[placeholder*="手机号"], .login-box, .login-scan-box, .btn-sure');
          if (isLoginUrl || hasLoginForm) {
            return { logged: false, reason: 'login_form_present' };
          }
          const hasUserInfo = !!document.querySelector('.user-nav, .nav-figure, .header-user, .user-avatar, .nav-item-user, a[href*="logout"], .nav-user, .user-name');
          const hasRecNav = url.includes('/boss/') || url.includes('recommend') || text.includes('推荐牛人') || text.includes('职位管理');
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
          const hasRecNav = (url.includes('ihr.zhaopin.com') || url.includes('rd5.zhaopin.com')) && (text.includes('简历管理') || text.includes('人才搜索') || text.includes('职位管理'));
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

  sendMsg('status', {
    platform: platformKey,
    message: `🎉 【${cfg.name}】企业后台连接成功！正在检索「${options.keyword}」(${options.city})...`
  });

  // DOM 元素提取逻辑
  let scraped = [];
  try {
    await new Promise(r => setTimeout(r, 2500));

    scraped = await page.evaluate((targetCount, kw, pName) => {
      const results = [];
      const selectors = [
        '.candidate-card-wrap', '.candidate-card', '.card-inner', '.recommend-card',
        '.geek-item', '.candidate-item', '.user-card', '.resume-item', '.resume-list-item',
        '.search-result-item', '.search-item', '.list-item', '[class*="candidate"]', '[class*="resume-card"]',
        '.talent-card', '.user-item'
      ];

      const elements = document.querySelectorAll(selectors.join(', '));
      for (let i = 0; i < elements.length && results.length < targetCount; i++) {
        const el = elements[i];
        const text = el.innerText || '';
        if (text.length < 25) continue;

        const nameEl = el.querySelector('h3, h4, .name, .user-name, .geek-name, .title-text, .c-name, .title');
        const name = nameEl ? nameEl.innerText.trim() : `${pName}候选人_${i + 1}`;

        const infoEl = el.querySelector('.info, .labels, .base-info, .desc, .user-desc, .exp-edu, .info-labels');
        const infoText = infoEl ? infoEl.innerText.trim().replace(/\n+/g, ' · ') : '';

        const workEl = el.querySelector('.work, .work-exp, .company, .company-name, .position, .experience');
        const workText = workEl ? workEl.innerText.trim() : '';

        const tags = Array.from(el.querySelectorAll('.tag, .skill-tag, .tag-item, span.label, .skill-label'))
          .map(t => t.innerText.trim())
          .filter(Boolean);

        results.push({
          name,
          infoText,
          workText,
          skills: tags,
          rawCardText: text
        });
      }
      return results;
    }, targetCount, options.keyword, cfg.name);
  } catch (evalErr) {
    sendMsg('status', { platform: platformKey, message: `⚠️ DOM 解析提示: ${evalErr.message}` });
  }

  if (!scraped || scraped.length === 0) {
    sendMsg('status', {
      platform: platformKey,
      message: `ℹ️ 在【${cfg.name}】当前工作台视图中未发现新推荐卡片，建议在打开的窗口中切换至招聘岗位。`
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

  sendMsg('status', {
    message: `🚀 启动全渠道聚合检索引擎，计划调度平台：${options.platforms.map(p => PLATFORM_CONFIGS[p]?.name || p).join('、')}，目标岗位「${options.keyword}」...`
  });

  let allResults = [];
  let errorCount = 0;
  let connectedPlatforms = [];

  for (const plat of options.platforms) {
    const res = await scrapePlatform(plat, browserPath, options.count);
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
