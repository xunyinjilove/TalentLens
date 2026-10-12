/** 从已登录的 51job 企业职位表单抓取语言、熟练程度及完整证书目录。 */
const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer-core');
const { createBackgroundPage, keepPageActiveInBackground } = require('./multi_platform_agent');

const output = path.join(__dirname, '..', 'frontend', 'src', 'data', '51job_posting_options.json');
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

async function main() {
  const browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 12000 });
  let page;
  let session;
  try {
    const source = (await browser.pages()).find(item => item.url().includes('ehire.51job.com'));
    if (!source) throw new Error('请先打开并登录 51job 企业浏览器');
    page = await createBackgroundPage(browser, source, 'https://ehire.51job.com/Revision/job?mark=new');
    session = await keepPageActiveInBackground(page);
    await page.waitForSelector('.language_form .el-select:first-child .el-select-dropdown__item', { timeout: 20000 });

    const languages = await page.evaluate(() => Array.from(document.querySelectorAll('.language_form .el-select:first-child .el-select-dropdown__item'))
      .map(item => item.textContent.trim()).filter(text => text && !text.startsWith('请选择')));
    if (!languages.length) throw new Error('51job 语言目录为空，请确认职位表单已加载');
    const languageLevels = {};
    for (const language of languages) {
      const selected = await page.evaluate(value => {
        const input = document.querySelector('.language_form .el-select:first-child input');
        input?.click();
        const item = Array.from(document.querySelectorAll('.language_form .el-select:first-child .el-select-dropdown__item'))
          .find(el => el.textContent.trim() === value && !el.classList.contains('is-disabled'));
        item?.click();
        return Boolean(item);
      }, language);
      if (!selected) throw new Error(`51job 语言未能选择：${language}`);
      await pause(100);
      const levels = await page.evaluate(() => Array.from(document.querySelectorAll('.language_form .el-select[prop="flevel1"] .el-select-dropdown__item'))
        .filter(el => !el.classList.contains('is-disabled')).map(el => el.textContent.trim()).filter(text => text && !text.startsWith('请选择')));
      if (!levels.length) throw new Error(`51job 未返回“${language}”熟练程度`);
      languageLevels[language] = [...new Set(levels)];
    }

    await page.evaluate(() => document.querySelector('[data-id="certificateInput"] input')?.click());
    await page.waitForFunction(() => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
      return Boolean(dialog?.querySelector('.cascader_panel_menu .func-item'));
    }, { timeout: 5000 });
    const categories = await page.evaluate(() => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
      return Array.from(dialog?.querySelectorAll('.cascader_panel_menu:first-child .func-item') || []).map(el => el.title || el.textContent.trim());
    });
    if (!categories.length) throw new Error('51job 证书分类为空');
    const certificates = {};
    for (const category of categories) {
      const selected = await page.evaluate(value => {
        const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
        const item = Array.from(dialog?.querySelectorAll('.cascader_panel_menu:first-child .func-item') || [])
          .find(el => (el.title || el.textContent.trim()) === value);
        item?.click();
        return Boolean(item);
      }, category);
      if (!selected) throw new Error(`51job 证书分类未能选择：${category}`);
      await pause(100);
      const options = await page.evaluate(() => {
        const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === '选择证书');
        return Array.from(dialog?.querySelectorAll('.cascader_panel_menu:nth-child(2) .func-item') || []).map(el => el.title || el.textContent.trim());
      });
      if (!options.length) throw new Error(`51job 证书分类“${category}”为空，请检查抓取`);
      certificates[category] = [...new Set(options)];
    }

    const snapshot = { source: '51job 企业职位表单', capturedAt: new Date().toISOString(), languages: [...new Set(languages)], languageLevels, certificateCategories: [...new Set(categories)], certificates };
    fs.writeFileSync(output, `${JSON.stringify(snapshot, null, 2)}\n`, 'utf8');
    process.stdout.write(JSON.stringify({ output, languages: snapshot.languages.length, categories: snapshot.certificateCategories.length, certificates: Object.values(certificates).reduce((sum, items) => sum + items.length, 0) }) + '\n');
  } finally {
    if (session) await session.detach().catch(() => {});
    if (page) await page.close().catch(() => {});
    await browser.disconnect().catch(() => {});
  }
}

main().catch(error => { process.stderr.write(`${error.message}\n`); process.exitCode = 1; });
