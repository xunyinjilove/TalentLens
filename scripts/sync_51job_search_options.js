/** Snapshot the logged-in 51job talent-search option trees for offline job setup. */
const fs = require('fs');
const path = require('path');
const WebSocket = require('ws');

const endpoint = 'http://127.0.0.1:9503';
const output = path.join(__dirname, '..', 'frontend', 'src', 'data', '51job_search_options.json');
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

async function connect(url) {
  const socket = new WebSocket(url);
  await new Promise((resolve, reject) => { socket.once('open', resolve); socket.once('error', reject); });
  let sequence = 0;
  const waiting = new Map();
  socket.on('message', data => {
    const message = JSON.parse(data.toString());
    const entry = waiting.get(message.id);
    if (!entry) return;
    waiting.delete(message.id);
    message.error ? entry.reject(new Error(message.error.message)) : entry.resolve(message.result);
  });
  return {
    call(method, params = {}) {
      return new Promise((resolve, reject) => {
        const id = ++sequence;
        waiting.set(id, { resolve, reject });
        socket.send(JSON.stringify({ id, method, params }));
      });
    },
    close() { socket.close(); }
  };
}

async function evaluate(target, expression) {
  const response = await target.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
  if (response.exceptionDetails) throw new Error(response.exceptionDetails.text || '页面脚本执行失败');
  return response.result?.value;
}

async function collectTree(target, selector, title) {
  const clicked = await evaluate(target, `Boolean(document.querySelector(${JSON.stringify(selector)})?.click() || document.querySelector(${JSON.stringify(selector)}))`);
  if (!clicked) throw new Error(`51job 未显示 ${title} 选择器`);
  await pause(200);
  const raw = await evaluate(target, `JSON.stringify((() => {
    const dialog = Array.from(document.querySelectorAll('.el-dialog__wrapper'))
      .filter(item => item.querySelector('.jbs_cascader_dialog_title')?.textContent.trim() === ${JSON.stringify(title)} &&
        getComputedStyle(item).display !== 'none' && !item.className.includes('dialog-fade-leave')).at(-1);
    const source = dialog?.querySelector('.jbs_cascader_panel')?.__vue__?.$props?.options || [];
    const copy = nodes => nodes.map(node => ({ label: String(node.label || '').trim(),
      children: Array.isArray(node.children) ? copy(node.children) : [] })).filter(node => node.label);
    return copy(source);
  })())`);
  const tree = JSON.parse(raw || '[]');
  if (!tree.length || tree.some(item => !item.children.length)) throw new Error(`51job ${title} 目录不完整`);
  return tree;
}

async function collectJobStatus(target) {
  const popoverID = await evaluate(target, `(() => {
    const nodes = Array.from(document.querySelectorAll('*')).filter(item => item.textContent?.trim().startsWith('求职状态'));
    const text = nodes.sort((a, b) => a.textContent.length - b.textContent.length)[0];
    const popover = text?.closest('.el-popover__reference, .el-popover__reference-wrapper') || text?.parentElement;
    popover?.click();
    return popover?.getAttribute('aria-describedby') || popover?.querySelector('[aria-describedby]')?.getAttribute('aria-describedby') || '';
  })()`);
  if (!popoverID) return [];
  await pause(120);
  const raw = await evaluate(target, `JSON.stringify((() => {
    return Array.from(document.getElementById(${JSON.stringify(popoverID)})?.querySelectorAll('.option-item-label') || [])
      .map(item => item.textContent.trim()).filter(Boolean);
  })())`);
  return JSON.parse(raw || '[]');
}

async function main() {
  const version = await (await fetch(`${endpoint}/json/version`, { signal: AbortSignal.timeout(5000) })).json();
  const browser = await connect(version.webSocketDebuggerUrl);
  try {
    const tabInfo = (await (await fetch(`${endpoint}/json/list`)).json())
      .find(tab => tab.type === 'page' && tab.url.includes('/Revision/talent/search'));
    if (!tabInfo?.webSocketDebuggerUrl) throw new Error('请先打开并登录 51job 人才搜索页');
    const tab = await connect(tabInfo.webSocketDebuggerUrl);
    try {
      let ready = false;
      for (let attempt = 0; attempt < 60; attempt++) {
        ready = await evaluate(tab, `!!document.querySelector('.candidate-living-area-picker button.base-select-button')`);
        if (ready) break;
        await pause(250);
      }
      if (!ready) throw new Error('51job 人才搜索筛选项尚未加载');
      const residence = await collectTree(tab, '.candidate-living-area-picker button.base-select-button', '选择现居地');
      const industry = await collectTree(tab, '.candidate-expected-industry-picker button.base-select-button', '选择行业');
      const functions = await collectTree(tab, '.candidate-expected-function-picker button.base-select-button', '选择职能');
      const jobStatuses = await collectJobStatus(tab);
      const snapshot = { source: '51job 企业人才搜索', capturedAt: new Date().toISOString(), residence, industry, functions, jobStatuses };
      fs.mkdirSync(path.dirname(output), { recursive: true });
      fs.writeFileSync(output, `${JSON.stringify(snapshot, null, 2)}\n`, 'utf8');
      process.stdout.write(JSON.stringify({ output, residence: residence.length, industry: industry.length, functions: functions.length, jobStatuses: jobStatuses.length }) + '\n');
    } finally { tab.close(); }
  } finally { browser.close(); }
}

main().catch(error => { process.stderr.write(`${error.message}\n`); process.exitCode = 1; });
