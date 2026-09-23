/**
 * test_greet.js - 独立候选人打招呼 / Hi聊功能测试与诊断模块
 * 
 * 用法：
 *   node scripts/test_greet.js [--platform 51job|boss|zhaopin|liepin] [--name 候选人姓名] [--dry-run]
 * 
 * 功能：
 *   1. 自动连接正在运行的浏览器或启动对应平台后台
 *   2. 准确定位候选人卡片或微简历详情抽屉中的打招呼/Hi聊按钮
 *   3. 测试点击触发并监测后续反馈（弹窗、IM 对话窗口、按钮状态变更）
 *   4. 输出完备的测试诊断日志
 */

const puppeteer = require('puppeteer-core');
const path = require('path');
const fs = require('fs');

const PLATFORM_PORTS = {
  boss: 9501,
  zhaopin: 9502,
  '51job': 9503,
  liepin: 9504
};

const PLATFORM_NAMES = {
  boss: 'BOSS直聘',
  zhaopin: '智联招聘',
  '51job': '前程无忧',
  liepin: '猎聘网'
};

// 解析命令行参数
const args = process.argv.slice(2);
let targetPlatform = '51job';
let targetName = '';
let dryRun = false; // 若为 true，则只探测定位（不真实点击）

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--platform' && args[i + 1]) {
    targetPlatform = args[++i];
  } else if (args[i] === '--name' && args[i + 1]) {
    targetName = args[++i];
  } else if (args[i] === '--dry-run') {
    dryRun = true;
  }
}

async function runGreetTest() {
  console.log(`\n======================================================`);
  console.log(`🤖 【TalentLens】多平台候选人打招呼测试诊断模块`);
  console.log(`目标平台: ${PLATFORM_NAMES[targetPlatform] || targetPlatform} (${targetPlatform})`);
  console.log(`指定候选人: ${targetName || '默认首位候选人'}`);
  console.log(`测试模式: ${dryRun ? '🔍 仅探测定位（不真实点击）' : '⚡ 真实拟人点击触发'}`);
  console.log(`======================================================\n`);

  const port = PLATFORM_PORTS[targetPlatform] || 9503;
  let browser = null;

  // 1. 连接浏览器
  try {
    browser = await puppeteer.connect({
      browserURL: `http://127.0.0.1:${port}`,
      defaultViewport: null
    });
    console.log(`✅ 成功连接到已运行的浏览器实例 (端口: ${port})`);
  } catch (e) {
    console.log(`⚠️ 未检测到端口 ${port} 上运行的浏览器，正在启动隔离浏览器...`);
    const browserPath = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
    const profileDir = path.join(process.cwd(), 'data', 'profiles', `${targetPlatform}_isolated_profile`);
    
    browser = await puppeteer.launch({
      executablePath: browserPath,
      headless: false,
      defaultViewport: null,
      args: [
        `--user-data-dir=${profileDir}`,
        `--remote-debugging-port=${port}`,
        '--no-first-run',
        '--no-default-browser-check',
        '--start-maximized'
      ]
    });
    console.log(`✅ 浏览器启动就绪`);
  }

  const pages = await browser.pages();
  const domainHint = targetPlatform === '51job' ? '51job.com' : targetPlatform;
  let page = pages.find(p => (p.url() || '').includes(domainHint) && !(p.url() || '').includes('about:blank'));

  if (!page) {
    page = pages[0] || (await browser.newPage());
    console.log(`🧭 正在导航至【${PLATFORM_NAMES[targetPlatform]}】后台...`);
    const homeUrls = {
      '51job': 'https://ehire.51job.com/Revision/talent/search',
      boss: 'https://www.zhipin.com/web/chat/index',
      zhaopin: 'https://rd6.zhaopin.com/app/recommend',
      liepin: 'https://lpt.liepin.com/recommend'
    };
    await page.goto(homeUrls[targetPlatform] || 'https://ehire.51job.com/Revision/talent/search', {
      waitUntil: 'domcontentloaded',
      timeout: 30000
    });
  }

  console.log(`📄 当前操作页面: ${page.url()}`);
  await new Promise(r => setTimeout(r, 2000));

  // 2. 诊断探测打招呼 / Hi聊按钮
  const testResult = await page.evaluate((candName, isDryRun, pPlatform) => {
    // 定义各平台候选人卡片选择器
    const cardSelectors = [
      '.talent-search-container .card',
      'div.card',
      '.eh-talent-search .card',
      '.candidate-card-wrap',
      '.geek-item',
      '.resume-item',
      '.search-result-item',
      '.candidate-box'
    ];

    const cards = Array.from(document.querySelectorAll(cardSelectors.join(', ')));
    if (cards.length === 0) {
      return {
        success: false,
        step: 'find_cards',
        message: '页面中未找到任何候选人卡片，请确认是否处于搜索结果页并已完成搜索。'
      };
    }

    // 寻找匹配的卡片（若无指定姓名则选第一张）
    let targetCard = null;
    if (candName) {
      targetCard = cards.find(c => {
        const text = c.innerText || '';
        return text.includes(candName);
      });
    } else {
      targetCard = cards[0];
    }

    if (!targetCard) {
      return {
        success: false,
        step: 'match_candidate',
        message: `在当前页面的 ${cards.length} 位候选人中未找到包含「${candName}」的卡片。`
      };
    }

    // 提取该候选人姓名
    const nameEl = targetCard.querySelector('.firstline .name, span.name, .name, h3, h4, .user-name');
    const actualName = nameEl ? nameEl.innerText.trim() : '候选人';

    // 检索卡片内的所有互动按钮
    // 51job: .talk_btn, 立即Hi聊, Hi聊
    // Boss: .btn-greet, 打招呼, 继续沟通
    // 智联: .btn-chat, 聊一聊
    // 猎聘: .btn-contact, 立即沟通, 打招呼
    const allClickables = Array.from(targetCard.querySelectorAll('button, div, span, a'));
    const greetKeywords = ['hi聊', '立即hi聊', '打招呼', '聊一聊', '立即沟通', '沟通', '发消息', '聊聊天'];

    const greetBtn = allClickables.find(el => {
      const t = (el.innerText || '').trim().toLowerCase();
      const cls = (el.className || '').toLowerCase();
      const isVisible = el.offsetParent !== null || el.getClientRects().length > 0;
      if (!isVisible) return false;

      // 关键词匹配
      const matchesKeyword = greetKeywords.some(kw => t.includes(kw));
      const matchesClass = cls.includes('greet') || cls.includes('chat') || cls.includes('talk');
      return matchesKeyword || matchesClass;
    });

    if (!greetBtn) {
      // 收集卡片上找到的所有按钮以供诊断
      const foundButtons = allClickables
        .filter(el => (el.tagName === 'BUTTON' || (el.className || '').includes('btn')) && (el.innerText || '').trim())
        .map(el => ({ tag: el.tagName, class: el.className, text: el.innerText.trim() }));

      return {
        success: false,
        step: 'find_greet_button',
        candidateName: actualName,
        message: `找到了候选人【${actualName}】卡片，但未检测到打招呼/Hi聊按钮。`,
        cardButtons: foundButtons
      };
    }

    const btnInfo = {
      tag: greetBtn.tagName,
      class: greetBtn.className,
      text: greetBtn.innerText.trim()
    };

    if (isDryRun) {
      return {
        success: true,
        dryRun: true,
        candidateName: actualName,
        button: btnInfo,
        message: `🎯 成功定位打招呼按钮: [${btnInfo.text}] (标签: <${btnInfo.tag}>, class: "${btnInfo.class}")`
      };
    }

    // 执行点击
    try {
      greetBtn.scrollIntoView({ behavior: 'smooth', block: 'center' });
      greetBtn.click();
      return {
        success: true,
        dryRun: false,
        candidateName: actualName,
        button: btnInfo,
        message: `🚀 成功触发点击【${actualName}】的「${btnInfo.text}」按钮！`
      };
    } catch (err) {
      return {
        success: false,
        step: 'click_button',
        candidateName: actualName,
        button: btnInfo,
        message: `点击按钮时发生异常: ${err.message}`
      };
    }
  }, targetName, dryRun, targetPlatform);

  console.log(`\n📋 测试诊断结果:`);
  console.log(JSON.stringify(testResult, null, 2));

  if (testResult.success) {
    if (!dryRun) {
      console.log(`\n⏳ 正在观察点击后的页面响应 (2秒)...`);
      await new Promise(r => setTimeout(r, 2000));

      // 检查页面是否出现了对话弹窗、确认框或 IM 窗口
      const postClickCheck = await page.evaluate(() => {
        const dialogs = Array.from(document.querySelectorAll('.el-dialog, .el-drawer, [class*="dialog"], [class*="chat"], [class*="modal"]'))
          .filter(d => (d.offsetParent !== null || d.getClientRects().length > 0) && (d.innerText || '').length > 10)
          .map(d => ({ class: d.className, textLen: (d.innerText || '').length, title: (d.innerText || '').substring(0, 80).replace(/\n+/g, ' ') }));

        return {
          openDialogs: dialogs.slice(0, 3)
        };
      });

      console.log(`🔔 页面后置响应状态:`, JSON.stringify(postClickCheck, null, 2));
      console.log(`\n✨ 打招呼测试执行完毕！`);
    } else {
      console.log(`\n✨ 打招呼按钮探测成功！`);
    }
  } else {
    console.log(`\n❌ 打招呼测试未完成: ${testResult.message}`);
  }

  return testResult;
}

if (require.main === module) {
  runGreetTest()
    .then(() => process.exit(0))
    .catch(err => {
      console.error('Fatal error:', err);
      process.exit(1);
    });
}

module.exports = { runGreetTest };
