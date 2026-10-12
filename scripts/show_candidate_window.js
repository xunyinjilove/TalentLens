/** Restore the dedicated recruiting browser after HR explicitly opens a candidate. */
const puppeteer = require('puppeteer-core');
const { setBrowserWindowState } = require('./multi_platform_agent');

const args = process.argv.slice(2);
const port = Number(args[args.indexOf('--port') + 1]);
const targetID = args[args.indexOf('--target') + 1];

async function main() {
  if (!Number.isInteger(port) || port < 1 || !targetID) throw new Error('browser port or target missing');
  const browser = await puppeteer.connect({ browserURL: `http://127.0.0.1:${port}`, defaultViewport: null });
  try {
    await restoreCandidateWindow(browser, targetID);
  } finally {
    await browser.disconnect();
  }
}

async function restoreCandidateWindow(browser, id, setWindowState = setBrowserWindowState) {
  const target = browser.targets().find(item => item._targetId === id) ||
    await browser.waitForTarget(item => item._targetId === id, { timeout: 5000 });
  const page = await target.page();
  if (!page || !await setWindowState(page, 'normal')) throw new Error('could not restore candidate browser window');
  await page.bringToFront();
}

if (require.main === module) {
  main().catch(error => {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  });
}

module.exports = { main, restoreCandidateWindow };
