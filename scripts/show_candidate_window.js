/** Manage dedicated recruiting browser window state (restore or minimize). */
const puppeteer = require('puppeteer-core');
const { setBrowserWindowState } = require('./multi_platform_agent');

const args = process.argv.slice(2);
const port = Number(args[args.indexOf('--port') + 1]);
const targetID = args.includes('--target') ? args[args.indexOf('--target') + 1] : '';
const state = args.includes('--state') ? args[args.indexOf('--state') + 1] : 'normal';

async function main() {
  if (!Number.isInteger(port) || port < 1) throw new Error('browser port missing');
  const browser = await puppeteer.connect({ browserURL: `http://127.0.0.1:${port}`, defaultViewport: null });
  try {
    if (targetID) {
      await restoreCandidateWindow(browser, targetID, state);
    } else {
      await setBrowserWindowState(browser, state);
    }
  } finally {
    await browser.disconnect();
  }
}

async function restoreCandidateWindow(browser, id, stateOrSetter = 'normal', customSetter = null) {
  let state = 'normal';
  let setWindowState = setBrowserWindowState;
  if (typeof stateOrSetter === 'function') {
    setWindowState = stateOrSetter;
  } else if (typeof stateOrSetter === 'string') {
    state = stateOrSetter;
    if (typeof customSetter === 'function') {
      setWindowState = customSetter;
    }
  }

  const target = browser.targets().find(item => item._targetId === id) ||
    await browser.waitForTarget(item => item._targetId === id, { timeout: 5000 });
  const page = await target.page();
  if (!page || !await setWindowState(page, state)) throw new Error('could not update candidate browser window');
  if (state !== 'minimized') {
    await page.bringToFront();
  }
}

if (require.main === module) {
  main().catch(error => {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  });
}

module.exports = { main, restoreCandidateWindow };

