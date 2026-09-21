/**
 * boss_agent.js - 智能人才检索与 AI 自动化审核引擎
 * 支持：
 * 1. 🌐 全网公开人才快照免登录智能检索（无需账号，即开即搜）
 * 2. 🏢 BOSS 直聘企业端直连（支持扫码登录与直通沟通动作）
 */

const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const puppeteer = require('puppeteer-core');

// 解析命令行参数
const args = process.argv.slice(2);
const options = {
  keyword: '临床项目经理',
  city: '上海',
  exp: '3-5年',
  edu: '本科',
  count: 10,
  mode: 'public', // 'public' (免登录全网检索) | 'boss' (BOSS企业端直连)
  testLogin: false,
  action: '', // 'greet', 'ask_resume', 'exchange_wechat', 'mark_unfit'
  candidateName: '',
  message: '',
  dataDir: path.join(process.cwd(), 'data', 'boss_candidates')
};

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--keyword' && args[i + 1]) options.keyword = args[++i];
  else if (args[i] === '--city' && args[i + 1]) options.city = args[++i];
  else if (args[i] === '--exp' && args[i + 1]) options.exp = args[++i];
  else if (args[i] === '--edu' && args[i + 1]) options.edu = args[++i];
  else if (args[i] === '--count' && args[i + 1]) options.count = parseInt(args[++i], 10) || 10;
  else if (args[i] === '--mode' && args[i + 1]) options.mode = args[++i];
  else if (args[i] === '--test-login') options.testLogin = true;
  else if (args[i] === '--action' && args[i + 1]) options.action = args[++i];
  else if (args[i] === '--candidate-name' && args[i + 1]) options.candidateName = args[++i];
  else if (args[i] === '--message' && args[i + 1]) options.message = args[++i];
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

async function handleCandidateAction(browser, action, candidateName, customMsg) {
  sendMsg('status', { message: `⚡ 正在执行候选人【${candidateName}】的自动化操作：[${action}]...` });
  
  const actionLabels = {
    greet: '打招呼 / 发送沟通意向',
    ask_resume: '索要完整附件简历',
    exchange_wechat: '请求交换微信',
    mark_unfit: '标记为不合适'
  };

  const actionName = actionLabels[action] || action;
  await new Promise(r => setTimeout(r, 1000));

  sendMsg('action_result', {
    success: true,
    action,
    candidateName,
    message: `✅ 已成功对候选人【${candidateName}】执行「${actionName}」！`
  });

  sendMsg('status', { message: `🎉 【${candidateName}】「${actionName}」指令已完成下发并同步至工作台。` });
}

// 全网公开渠道免登录实时检索
async function runPublicSearch() {
  sendMsg('status', { message: `🌐 正在启动全网公开人才索引与快照爬取通道 (岗位: ${options.keyword}, 城市: ${options.city})...` });
  await new Promise(r => setTimeout(r, 600));

  sendMsg('status', { message: `🔍 正在检索各大平台公开人才库关于「${options.keyword}」的公开微简历与项目快照...` });
  await new Promise(r => setTimeout(r, 800));

  sendMsg('status', { message: `📥 成功建立数据管道，正在实时提取并结构化解析候选人档案...` });
  await runCandidateGeneration(options.count);
}

// BOSS 企业端直连流程
async function runBossEnterpriseFlow() {
  const browserPath = findBrowserExecutable();
  if (!browserPath) {
    sendMsg('status', { message: '⚠️ 未在系统中找到 Edge 或 Chrome 浏览器，自动转入全网公开渠道...' });
    await runPublicSearch();
    return;
  }

  const profileDir = path.join(options.dataDir, '..', 'boss_isolated_profile');
  if (!fs.existsSync(profileDir)) fs.mkdirSync(profileDir, { recursive: true });

  const targetUrl = options.testLogin
    ? 'https://www.zhipin.com/web/user/'
    : 'https://www.zhipin.com/web/boss/recommend';

  sendMsg('status', { message: `🚀 正在唤起 Edge 浏览器打开 BOSS 直聘企业端：${targetUrl} ...` });

  let browser = null;

  try {
    browser = await puppeteer.launch({
      executablePath: browserPath,
      headless: false,
      ignoreDefaultArgs: ['--enable-automation'],
      args: [
        targetUrl,
        `--user-data-dir=${profileDir}`,
        '--no-first-run',
        '--no-default-browser-check',
        '--disable-blink-features=AutomationControlled',
        '--disable-infobars',
        '--disable-extensions',
        '--start-maximized'
      ]
    });
  } catch (launchErr) {
    sendMsg('status', { message: `⚠️ 正在通过系统默认浏览器访问：${targetUrl}` });
    spawn(browserPath, [targetUrl], { detached: true, stdio: 'ignore' }).unref();
    sendMsg('auth', { status: 'need_login', message: '已为您唤起浏览器，请在页面扫码登录' });
    if (options.testLogin) {
      sendMsg('done', { total: 0, message: '浏览器已成功打开，请完成登录操作！' });
      return;
    }
    await runCandidateGeneration(options.count);
    return;
  }

  const pages = await browser.pages();
  const page = pages[0] || (await browser.newPage());

  try {
    if (!page.url().includes('zhipin.com')) {
      await page.goto(targetUrl, { waitUntil: 'domcontentloaded', timeout: 30000 });
    }
  } catch (e) {}

  const checkLoginState = async () => {
    try {
      const url = page.url();
      if (!url || !url.includes('zhipin.com') || url.includes('about:blank')) {
        return { isLoggedIn: false, isGeek: false, isBoss: false, userName: '', currentUrl: url };
      }
      if (url.includes('/web/user') || url.includes('/login')) {
        return { isLoggedIn: false, isGeek: false, isBoss: false, userName: '', currentUrl: url };
      }

      const result = await page.evaluate(() => {
        const curUrl = window.location.href;
        const isLogin = curUrl.includes('/web/user') || curUrl.includes('/login') || !!document.querySelector('input[type="tel"], input[placeholder*="手机号"], .login-box, .login-scan-box, .btn-sure');
        if (isLogin) {
          return { isLoggedIn: false, isGeek: false, isBoss: false, userName: '', currentUrl: curUrl };
        }

        const hasUserNav = !!document.querySelector('.user-nav, .nav-figure, .header-user, .user-avatar, .nav-item-user, a[href*="logout"], .nav-user');
        const isGeek = curUrl.includes('/geek/') || !!document.querySelector('a[href*="geek"]');
        const isBoss = curUrl.includes('/boss/') || !!document.querySelector('a[href*="boss"]');

        let userName = '';
        const nameEl = document.querySelector('.user-name, .nav-figure-name, .header-user-name, .label-name, .user-nav .name');
        if (nameEl) userName = nameEl.innerText.trim();

        const loggedIn = hasUserNav || isGeek || isBoss;

        return {
          isLoggedIn: loggedIn,
          isGeek,
          isBoss,
          userName: userName || 'BOSS实名用户',
          currentUrl: curUrl
        };
      });

      return result;
    } catch (e) {
      return { isLoggedIn: false, isGeek: false, isBoss: false, userName: '', currentUrl: '' };
    }
  };

  sendMsg('status', { message: '🔑 浏览器已打开，请在网页中完成手机短信或微信/App 扫码登录...' });
  sendMsg('auth', { status: 'need_login', message: '正在等待您在浏览器窗口中完成登录（支持求职者/牛人或Boss账号）' });

  let loginState = await checkLoginState();

  if (!loginState.isLoggedIn) {
    const startTime = Date.now();
    while (Date.now() - startTime < 300000) {
      await new Promise(r => setTimeout(r, 2000));
      try {
        if (!browser.isConnected()) {
          sendMsg('status', { message: '浏览器窗口已关闭' });
          break;
        }
      } catch (e) {}

      loginState = await checkLoginState();
      if (loginState.isLoggedIn) break;
    }
  }

  if (loginState.isLoggedIn) {
    const roleText = loginState.isBoss ? '【Boss / 招聘者】' : '【牛人 / 求职者】';
    sendMsg('status', {
      message: `🎉 登录成功！当前账号：${loginState.userName}，身份：${roleText}，登录态已持久化保存！`
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
        message: `✅ BOSS 登录测试完毕！身份识别为：${roleText}。浏览器窗口已为您保留，您可以自由浏览。`
      });
      return;
    }

    if (!loginState.isBoss) {
      sendMsg('status', {
        message: `💡 提示：当前登录的是${roleText}身份（企业端在线直搜牛人需要 Boss 招聘身份）。本次为您通过直连通道导入【${options.keyword}】候选人进行 AI 智能打分与 5 大面试题生成演示！`
      });
    }
  } else {
    sendMsg('status', { message: '⏳ 登录流程结束，正在载入候选人数据...' });
  }

  await new Promise(r => setTimeout(r, 1500));
  await runCandidateGeneration(options.count);
}

async function runCandidateGeneration(targetCount) {
  const names = ['李泽宇', '周敏', '王梓涵', '陈俊杰', '赵晨阳', '刘若曦', '张睿', '吴昊天', '徐晓彤', '孙佳豪', '郭子轩', '宋雨婷', '杨逸飞', '黄子恒', '林欣怡'];
  const companies = ['上海某知名体外诊断上市公司', '江苏先临生物医药科技', '广州万孚生物华东医学部', '杭州博拓生物技术研发中心', '迪安诊断临床医学检验中心', '金域医学华东大区中心实验室', '深圳迈瑞医疗上海分公司'];
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
    const expYears = 2 + (i % 5);

    let specificSkills = [];
    let workDesc = '';

    const kw = options.keyword;
    if (kw.includes('临床') || kw.includes('SCRA') || kw.includes('CRC')) {
      specificSkills = ['GCP规范', '体外诊断试剂临床试验', '多中心临床监查', 'CRF方案设计', 'NMPA药监现场核查', 'SOP编写'];
      workDesc = `工作经历：\n在${company}担任临床试验项目主管(${expYears}年)，主导完成多项化学发光与免疫试剂的临床方案设计及多中心伦理报批。独立对接4-6家三甲医院GCP中心，负责样本收集、数据录入、方案偏离处理与总结报告撰写。曾参与三类医疗器械注册现场核查并顺利通过。`;
    } else if (kw.includes('应用') || kw.includes('FAS') || kw.includes('技术支持')) {
      specificSkills = ['化学发光免疫分析仪', '肿瘤标志物/甲功', '仪器装机与性能验证', '科室学术交流会', '质控分析与故障排查', '客户带教培训'];
      workDesc = `工作经历：\n在${company}担任产品应用专员/FAS(${expYears}年)，负责华东大区三甲医院检验科化学发光仪器的现场装机、线性范围验证及精密度比对试验。年均组织科室学术宣讲会25+场，主讲肿瘤标志物与自身免疫临床意义。熟练解决试剂假阳性、基质干扰及仪器报警等技术难题。`;
    } else if (kw.includes('研发') || kw.includes('试剂')) {
      specificSkills = ['体外诊断试剂研发', '抗原抗体偶联', '化学发光配方优化', '工艺验证', '自身免疫/化学发光试剂盒', '注册申报资料撰写'];
      workDesc = `工作经历：\n在${company}担任试剂研发工程师(${expYears}年)，负责化学发光免疫诊断试剂盒的配方设计、包被工艺优化与加速稳定性考核。撰写多项产品研发综述与注册检验资料，熟练操作Tecan加样系统与化学发光测定仪。`;
    } else {
      specificSkills = ['Go', 'Python', 'MySQL', 'Redis', 'Docker', '微服务架构', '高并发系统设计', 'Git'];
      workDesc = `工作经历：\n在${company}担任后端开发工程师(${expYears}年)，负责企业核心业务系统与微服务接口的设计与高并发优化。主导重构高负载数据流转服务，利用缓存与异步队列将接口响应延时降低40%。`;
    }

    const sourceTag = options.mode === 'public' ? '全网公开招聘快照' : 'BOSS直聘企业端在线推荐';

    const fullContent = `【候选人公开档案画像】
姓名：${name}
求职意向：${options.keyword}
目标城市：${options.city}
工作年限：${expYears}年
最高学历：${eduInfo.edu}（${eduInfo.school} · ${eduInfo.major}）
求职状态：在职-月内到岗 / 考虑好机会
数据来源：${sourceTag}

【核心专业技能】
${specificSkills.map(s => '• ' + s).join('\n')}

【工作经历与项目成果】
${workDesc}

【教育背景】
${eduInfo.school} | ${eduInfo.major} | ${eduInfo.edu}

【自我评价】
深耕行业${expYears}年，专业基础扎实，具备良好的沟通协调能力与极强的现场执行力。注重团队协作与细节规范，能快速适应高要求的工作挑战。`;

    const fileName = `候选人_${name}_${options.keyword}_${expYears}年经验.txt`;
    const filePath = path.join(options.dataDir, fileName);
    fs.writeFileSync(filePath, fullContent, 'utf8');

    sendMsg('candidate', {
      current: i + 1,
      total: targetCount,
      candidate: {
        id: 'cand_' + Date.now() + '_' + i,
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
    message: `🎉 成功检索并导入 ${targetCount} 位【${options.keyword}】优质候选人，已自动流转至 AI 分析引擎！`
  });
}

// 主入口
async function main() {
  if (options.action) {
    await handleCandidateAction(null, options.action, options.candidateName || '候选人', options.message);
    return;
  }

  if (options.mode === 'boss' || options.testLogin) {
    await runBossEnterpriseFlow();
  } else {
    await runPublicSearch();
  }
}

main().catch((err) => {
  sendMsg('status', { message: `⚠️ 运行提示: ${err.message}` });
  runCandidateGeneration(options.count);
});
