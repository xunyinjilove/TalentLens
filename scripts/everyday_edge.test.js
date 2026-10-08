const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const http = require('node:http');
const edge = require('./everyday_edge');

test('only accepts a valid local debugging port', () => {
  assert.equal(edge.parseActivePort('12345\n/devtools/browser/abc'), 12345);
  assert.equal(edge.parseActivePort('0'), null);
  assert.equal(edge.parseActivePort('not-a-port'), null);
});

test('attaches only when the active-port file matches this Edge endpoint', async () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'talentlens-edge-'));
  const previous = process.env.LOCALAPPDATA;
  const server = http.createServer((request, response) => {
    response.setHeader('Content-Type', 'application/json');
    response.end(JSON.stringify({ Browser: 'Microsoft Edge/140.0', webSocketDebuggerUrl: `ws://127.0.0.1:${server.address().port}/devtools/browser/expected` }));
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  try {
    process.env.LOCALAPPDATA = home;
    const dir = path.join(home, 'Microsoft', 'Edge', 'User Data');
    fs.mkdirSync(dir, { recursive: true });
    const activeFile = path.join(dir, 'DevToolsActivePort');
    fs.writeFileSync(activeFile, `${server.address().port}\n/devtools/browser/expected\n`);
    assert.equal(await edge.getEdgePort(), server.address().port);
    fs.writeFileSync(activeFile, `${server.address().port}\n/devtools/browser/wrong\n`);
    await assert.rejects(edge.getEdgePort(), /连接不可用/);
  } finally {
    if (previous === undefined) delete process.env.LOCALAPPDATA;
    else process.env.LOCALAPPDATA = previous;
    await new Promise(resolve => server.close(resolve));
    fs.rmSync(home, { recursive: true, force: true });
  }
});
