const test = require('node:test');
const assert = require('node:assert/strict');
const esbuild = require('../frontend/node_modules/esbuild');

async function loadRecovery() {
  const built = await esbuild.build({ entryPoints: ['frontend/src/composables/51jobBrowserRecovery.ts'], platform: 'node', format: 'cjs', bundle: true, write: false });
  const module = { exports: {} };
  new Function('module', 'exports', built.outputFiles[0].text)(module, module.exports);
  return module.exports;
}

test('浏览器关闭时先启动并等待登录，再继续原来的只读操作', async () => {
  const { runWith51JobBrowser } = await loadRecovery();
  const events = [];
  const app = {
    Show51JobPage: async () => { events.push('show'); return { status: 'ready' }; },
    Check51JobLogin: async () => { events.push('check'); return { status: 'ready' }; },
    Minimize51JobBrowser: async () => { events.push('minimize'); return { status: 'ready' }; }
  };
  let attempts = 0;
  const result = await runWith51JobBrowser(app, async () => {
    events.push('action');
    return ++attempts === 1 ? { status: 'needs_login' } : { status: 'ready', options: ['地址'] };
  });
  assert.deepEqual(events, ['action', 'show', 'check', 'minimize', 'action']);
  assert.deepEqual(result.options, ['地址']);
});

test('不确定或已发送的操作绝不自动重放', async () => {
  const { runWith51JobBrowser } = await loadRecovery();
  let count = 0;
  const app = { Show51JobPage: () => { throw new Error('must not open'); } };
  const result = await runWith51JobBrowser(app, async () => { count++; return { status: 'uncertain' }; });
  assert.equal(result.status, 'uncertain');
  assert.equal(count, 1);
});
