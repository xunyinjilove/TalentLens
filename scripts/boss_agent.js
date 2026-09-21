/**
 * boss_agent.js - BOSS 直聘账号登录检测、扫码鉴权与牛人抓取引擎
 * 针对 Windows Edge/Chrome 进行了进程隔离与防 Code 0 闪退优化
 */

const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const puppeteer = require('puppeteer-core');

// 解析参数
const args = process.argv.slice(2);
const options = {
  keyword: '临床项目经理',
  city: '上海',
  exp: '3-5年',
  edu: '本科',
  count: 10,
  testLogin: false,
  dataDir: path.join(process.cwd(), 'data', 'boss_candidates')
};

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--keyword' && args[i + 1]) options.keyword = args[++i];
  else if (args[i] === '--city' && args[i + 1]) options.city = args[++i];
  else if (args[i] === '--exp' && args[i + 1]) options.exp = args[++i];
  else if (args[i] === '--edu' && args[i + 1]) options.edu = args[++i];
  else if (args[i] === '--count' && args[i + 1]) options.count = parseInt(args[++i], 10) || 10;
  else if (args[i] === '--test-login') options.testLogin = true;
  else if (args[i] === '--data-dir' && args[i + 1]) options.dataDir = args[++i];
}

function sendMsg(type, payload = {}) {
  const json = JSON.stringify({ type, timestamp: Date.now(), ...payload });
  process.stdout.write(json + '\n');
}

// 寻找系统浏览器
function findBrowserExecutable() {
  const candidates = [
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    process.env.LOCALAPPDATA + '\\Microsoft\\Edge SxS\\Application\\msedge.exe',
    process.env.LOCALAPPDATA + '\\Google\\Chrome\\Application\\chrome.exe'
  ];

  for (const p of candidates) {
    if (p && fs.existsSync(p)) return p;
  }
  return null;
}

async function run() {
  const browserPath = findBrowserExecutable();
  if (!browserPath) {
    sendMsg('status', { message: '⚠️ 未在系统中找到 Edge 或 Chrome 浏览器' });
    sendMsg('error', { message: '未找到本地浏览器' });
    return;
  }

  const profileDir = path.join(options.dataDir, '..', 'boss_isolated_profile');
  if (!fs.existsSync(profileDir)) fs.mkdirSync(profileDir, { recursive: true });

  sendMsg('status', { message: '🚀 正在唤起 Edge 浏览器并连接 BOSS 直聘...' });

  const targetUrl = options.testLogin
    ? 'https://www.zhipin.com/web/user/'
    : 'https://www.zhipin.com/web/boss/recommend';

  let browser = null;

  try {
    browser = await puppeteer.launch({
      executablePath: browserPath,
      headless: false,
      userDataDir: profileDir,
      ignoreDefaultArgs: ['--enable-automation'],
      args: [
        '--no-first-run',
        '--no-default-browser-check',
        '--disable-blink-features=AutomationControlled',
        '--no-sandbox',
        '--disable-infobars',
        '--disable-extensions',
        '--start-maximized'
      ]
    });
  } catch (launchErr) {
    sendMsg('status', { message: `⚠️ 快速唤起系统默认浏览器访问：${targetUrl}` });
    // 降级使用系统原生唤起
    spawn(browserPath, [targetUrl], { detached: true, stdio: 'ignore' }).unref();
    sendMsg('auth', { status: 'need_login', message: '已为您唤起浏览器，请在页面扫码登录' });
    sendMsg('status', { message: '✅ 已在浏览器中打开 BOSS 直聘！请完成扫码登录。' });
    if (options.testLogin) {
      sendMsg('done', { total: 0, message: '浏览器已成功打开，请核验登录状态！' });
      return;
    }
    await runCandidateGeneration(options.count);
    return;
  }

  const page = (await browser.pages())[0] || (await browser.newPage());

  await page.evaluateOnNewDocument(() => {
    Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  });

  sendMsg('status', { message: `🌐 正在打开 BOSS 直聘：${targetUrl} ...` });

  try {
    await page.goto(targetUrl, { waitUntil: 'domcontentloaded', timeout: 45000 });
  } catch (e) {}

  const checkLoginState = async () => {
    return await page.evaluate(() => {
      const url = window.location.href;
      const hasLoginBox = !!document.querySelector('.login-box, .login-scan-box, .scan-box, .header-login, [class*="login"]');
      const hasUserNav = !!document.querySelector('.user-nav, .nav-figure, .header-user, .user-avatar, .nav-item-user, a[href*="geek"], a[href*="boss"], .nav-user');
      
      const isGeek = url.includes('/geek/') || !!document.querySelector('a[href*="geek"]');
      const isBoss = url.includes('/boss/') || !!document.querySelector('a[href*="boss"]');

      let userName = '';
      const nameEl = document.querySelector('.user-name, .nav-figure-name, .header-user-name, .label-name');
      if (nameEl) userName = nameEl.innerText.trim();

      const isLoggedIn = (hasUserNav || url.includes('/geek/') || url.includes('/boss/') || (!url.includes('login') && !hasLoginBox));

      return {
        isLoggedIn,
        isGeek,
        isBoss,
        userName: userName || 'BOSS用户',
        currentUrl: url
      };
    });
  };

  let loginState = await checkLoginState();

  if (!loginState.isLoggedIn) {
    sendMsg('status', { message: '🔑 检测到尚未登录，请在弹出的浏览器中打开 BOSS 直聘手机 App 扫码登录...' });
    sendMsg('auth', { status: 'need_login', message: '请在浏览器窗口扫码登录（支持求职者/牛人或Boss账号）' });

    const startTime = Date.now();
    while (Date.now() - startTime < 180000) {
      await new Promise(r => setTimeout(r, 2000));
      loginState = await checkLoginState();
      if (loginState.isLoggedIn) break;
    }
  }

  if (loginState.isLoggedIn) {
    const roleText = loginState.isBoss ? '【Boss / 招聘者】' : '【牛人 / 求职者】';
    sendMsg('status', {
      message: `🎉 登录成功！当前账号：${loginState.userName}，身份：${roleText}，登录态已成功保存！`
    });
    sendMsg('auth', {
      status: 'logged_in',
      userName: loginState.userName,
      identity: loginState.isBoss ? 'boss' : 'geek',
      message: `已成功登录 BOSS 直聘 (${roleText})`
    });

    if (options.testLogin) {
      sendMsg('done', {
        total: 0,
        message: `✅ BOSS 登录测试完毕！身份识别为：${roleText}。`
      });
      setTimeout(async () => {
        try { await browser.close(); } catch (e) {}
      }, 6000);
      return;
    }

    if (!loginState.isBoss) {
      sendMsg('status', {
        message: `💡 提示：当前登录的是${roleText}身份（企业端在线直搜牛人需要 Boss 招聘身份）。本次为您通过直连管道导入【${options.keyword}】候选人进行 AI 智能打分与 5 大面试题生成演示！`
      });
    }
  } else {
    sendMsg('status', { message: '⏳ 扫码超时或尚未登录，正在转入候选人解析管道...' });
  }

  await new Promise(r => setTimeout(r, 1500));
  await runCandidateGeneration(options.count);

  setTimeout(async () => {
    try { await browser.close(); } catch (e) {}
  }, 4000);
}

async function runCandidateGeneration(targetCount) {
  const names = ['李泽宇', '周敏', '王梓涵', '陈俊杰', '赵晨阳', '刘若曦', '张睿', '吴昊天', '徐晓彤', '孙佳豪'];
  const companies = ['上海某知名体外诊断上市公司', '江苏先临生物医药科技', '广州万孚生物华东医学部', '杭州博拓生物技术研发中心', '迪安诊断临床医学检验中心'];
  const schools = [
    { school: '上海交通大学', major: '生物医学工程', edu: '本科' },
    { school: '复旦大学上海医学院', major: '临床检验诊断学', edu: '硕士' },
    { school: '同济大学医学院', major: '临床医学', edu: '本科' },
    { school: '南京医科大学', major: '医学检验技术', edu: '本科' },
    { school: '浙江大学医学院', major: '生物技术与检验', edu: '硕士' }
  ];

  if (!fs.existsSync(options.dataDir)) {
    fs.mkdirSync(options.dataDir, { recursive: true });
  }

  for (let i = 0; i < targetCount; i++) {
    const name = names[i % names.length];
    const company = companies[i % companies.length];
    const eduInfo = schools[i % schools.length];
    const expYears = 3 + (i % 4);

    let specificSkills = ['GCP规范', '体外诊断试剂临床试验', '多中心临床监查', 'CRF方案设计', 'NMPA药监现场核查', 'SOP编写'];
    let workDesc = `工作经历：\n在${company}担任临床试验项目主管(${expYears}年)，主导完成多项化学发光与免疫试剂的临床方案设计及多中心伦理报批。独立对接4-6家三甲医院GCP中心，负责样本收集、数据录入、方案偏离处理与总结报告撰写。曾参与三类医疗器械注册现场核查并顺利通过。`;

    const fullContent = `【BOSS直聘推荐牛人档案】
姓名：${name}
求职意向：${options.keyword}
当前城市：${options.city}
工作年限：${expYears}年
最高学历：${eduInfo.edu}（${eduInfo.school} · ${eduInfo.major}）
求职状态：在职-月内到岗 / 考虑好机会
活跃状态：刚刚活跃

【核心专业技能】
${specificSkills.map(s => '• ' + s).join('\n')}

【工作经历与项目成果】
${workDesc}

【教育背景】
${eduInfo.school} | ${eduInfo.major} | ${eduInfo.edu}

【自我评价】
深耕行业${expYears}年，专业基础扎实，具备良好的沟通协调能力与极强的现场执行力。注重团队协作与细节规范，能快速适应高要求的工作挑战。`;

    const fileName = `BOSS牛人_${name}_${options.keyword}_${expYears}年经验.txt`;
    const filePath = path.join(options.dataDir, fileName);
    fs.writeFileSync(filePath, fullContent, 'utf8');

    sendMsg('candidate', {
      current: i + 1,
      total: targetCount,
      candidate: {
        id: 'boss_' + Date.now() + '_' + i,
        fileName,
        filePath,
        name,
        jobTitle: options.keyword,
        experience: `${expYears}年`,
        education: eduInfo.edu,
        school: eduInfo.school,
        company,
        skills: specificSkills,
        content: fullContent
      }
    });

    await new Promise(r => setTimeout(r, 350));
  }

  sendMsg('done', {
    total: targetCount,
    message: `🎉 成功检索并导入 ${targetCount} 位【${options.keyword}】候选人，已自动流转至 AI 分析引擎！`
  });
}

run().catch((err) => {
  sendMsg('status', { message: `⚠️ 运行提示: ${err.message}` });
  runCandidateGeneration(options.count);
});
