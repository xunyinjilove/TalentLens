const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer-core');

async function main() {
  const browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null });
  const pages = await browser.pages();
  const source = pages.find(p => p.url().includes('ehire.51job.com'));
  if (!source) {
    throw new Error('请先打开并登录 51job 企业浏览器');
  }
  const page = await browser.newPage();
  try {
    await page.goto('https://ehire.51job.com/Revision/job?mark=new', { waitUntil: 'domcontentloaded', timeout: 15000 });
    await new Promise(r => setTimeout(r, 2000));

    // Open dialog
    await page.evaluate(() => {
      const btn = document.querySelector('.func-dropdown .all_func_tips');
      if (btn) btn.click();
    });
    await new Promise(r => setTimeout(r, 1500));

    // Extract options
    const rawOptions = await page.evaluate(() => {
      const dialog = Array.from(document.querySelectorAll('.el-dialog')).find(el => el.getBoundingClientRect().height > 0 && el.innerText.includes('选择职能'));
      if (!dialog) return null;
      const elements = [dialog, ...Array.from(dialog.querySelectorAll('*'))];
      for (const el of elements) {
        if (el.__vue__ && el.__vue__.options && Array.isArray(el.__vue__.options) && el.__vue__.options.length > 5) {
          return el.__vue__.options;
        }
      }
      return null;
    });

    if (!rawOptions) {
      throw new Error('未能读取 51job 职能目录');
    }

    console.log(`Found ${rawOptions.length} Level 1 categories`);

    // Let's transform rawOptions into clean format:
    // [{ name: string, id: string, children: [{ name: string, id: string, children: [{ name: string, id: string }] }] }]
    function cleanNode(node) {
      const name = (node.label || node.name || node.title || '').trim();
      const id = node.id || node.value || '';
      const children = Array.isArray(node.children) ? node.children.map(cleanNode).filter(c => c.name) : [];
      const res = { name, id };
      if (children.length > 0) {
        res.children = children;
      } else {
        res.leaf = true;
      }
      return res;
    }

    const cleanTree = rawOptions.map(cleanNode).filter(n => n.name);
    console.log(`Cleaned ${cleanTree.length} categories.`);
    let totalL2 = 0;
    let totalL3 = 0;
    for (const l1 of cleanTree) {
      totalL2 += l1.children?.length || 0;
      for (const l2 of l1.children || []) {
        totalL3 += l2.children?.length || 0;
      }
    }
    console.log(`Summary: Level 1: ${cleanTree.length}, Level 2: ${totalL2}, Level 3 (Specific Jobs): ${totalL3}`);

    const targetFrontend = path.join(__dirname, '..', 'frontend', 'src', 'data', '51job_functions.json');
    const targetScripts = path.join(__dirname, '51job_functions.json');

    fs.writeFileSync(targetFrontend, JSON.stringify(cleanTree, null, 2), 'utf8');
    fs.writeFileSync(targetScripts, JSON.stringify(cleanTree, null, 2), 'utf8');
    console.log(`Saved to ${targetFrontend} and ${targetScripts}`);

    // Print first 3 categories
    console.log('Sample Level 1:', cleanTree.slice(0, 3).map(c => ({ name: c.name, subCount: c.children?.length })));
  } finally {
    await page.close().catch(() => {});
    await browser.disconnect().catch(() => {});
  }
}

main().catch(error => { console.error(error.message); process.exitCode = 1; });
