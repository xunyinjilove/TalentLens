// 连接用户已打开的日常 Edge。用户须先在 edge://inspect 中允许本机远程调试。
const fs = require('fs');
const path = require('path');
const http = require('http');
const puppeteer = require('puppeteer-core');

const SETUP = '请在常用 Edge 打开 edge://inspect，进入 Remote debugging 并勾选 Allow remote debugging for this browser instance，然后重试';

function parseActivePort(contents) {
  const port = Number(String(contents).trim().split(/\r?\n/)[0]);
  return Number.isInteger(port) && port > 0 && port <= 65535 ? port : null;
}

function readActivePort() {
  try {
    if (!process.env.LOCALAPPDATA) return null;
    const file = path.join(process.env.LOCALAPPDATA, 'Microsoft', 'Edge', 'User Data', 'DevToolsActivePort');
    const [port, socketPath] = fs.readFileSync(file, 'utf8').trim().split(/\r?\n/);
    const parsedPort = parseActivePort(port);
    return parsedPort && /^\/devtools\/browser\/[\w-]+$/.test(socketPath || '')
      ? { port: parsedPort, socketPath } : null;
  } catch { return null; }
}

async function getEdgePort() {
  const active = readActivePort();
  if (!active) throw new Error(SETUP);
  const { port, socketPath } = active;
  const version = await new Promise((resolve, reject) => {
    const request = http.get(`http://127.0.0.1:${port}/json/version`, { timeout: 2000 }, response => {
      let body = '';
      response.setEncoding('utf8');
      response.on('data', chunk => { body += chunk; });
      response.on('end', () => {
        try { resolve(JSON.parse(body)); } catch (error) { reject(error); }
      });
    });
    request.on('timeout', () => request.destroy(new Error('连接超时')));
    request.on('error', reject);
  }).catch(() => null);
  if (!version || !/Edg|Edge/i.test(version.Browser || '') ||
      !String(version.webSocketDebuggerUrl || '').endsWith(socketPath)) {
    throw new Error(`常用 Edge 连接不可用。${SETUP}`);
  }
  return port;
}

async function connectEverydayEdge() {
  const port = await getEdgePort();
  return puppeteer.connect({ browserURL: `http://127.0.0.1:${port}`, defaultViewport: null, protocolTimeout: 12000 });
}

module.exports = { SETUP, parseActivePort, readActivePort, getEdgePort, connectEverydayEdge };
