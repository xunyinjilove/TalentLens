const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const agent = require('./multi_platform_agent');

test('quota preserves total and zero weight', () => {
  assert.deepEqual(agent.allocateQuota(20, [{ ratio: 40 }, { ratio: 30 }, { ratio: 30 }]), [8, 6, 6]);
  assert.deepEqual(agent.allocateQuota(10, [{ ratio: 99 }, { ratio: 1 }, { ratio: 0 }]), [10, 0, 0]);
});

test('minimized search scrolls the list without mouse wheel events', async () => {
  const body = {};
  const scroller = { parentElement: body, scrollTop: 0, scrollHeight: 2000, clientHeight: 500 };
  const card = { parentElement: scroller };
  global.document = { body, querySelector: () => card };
  const page = {
    evaluate: async (fn, ...args) => fn(...args),
    mouse: { wheel: async () => { throw Error('mouse wheel must not be used'); } }
  };
  await agent.smoothScroll(page, 450, 80);
  assert.equal(scroller.scrollTop, 450);
  delete global.document;
});

test('background search enables page focus emulation', async () => {
  const commands = [];
  const session = { send: async (command, args) => commands.push({ command, args }) };
  const page = { target: () => ({ createCDPSession: async () => session }) };
  assert.equal(await agent.keepPageActiveInBackground(page), session);
  assert.deepEqual(commands, [{ command: 'Emulation.setFocusEmulationEnabled', args: { enabled: true } }]);
});

test('existing browser connection retries a CDP timeout without replacing the browser', async () => {
  let attempts = 0;
  const browser = { pages: async () => [] };
  const result = await agent.connectToExistingBrowser(
    { debugPort: 9503, name: '前程无忧' },
    {
      probe: async () => true,
      connect: async config => {
        attempts++;
        assert.equal(config.protocolTimeout, 120000);
        if (attempts === 1) throw new Error('Network.enable timed out');
        return browser;
      },
      wait: async () => {}
    }
  );
  assert.equal(result, browser);
  assert.equal(attempts, 2);
});

test('busy browser debug endpoint is probed again before reporting failure', async () => {
  let probes = 0;
  const browser = { pages: async () => [] };
  const result = await agent.connectToExistingBrowser(
    { debugPort: 9503, name: '前程无忧' },
    {
      probe: async () => { if (++probes === 1) throw Error('endpoint timed out'); return true; },
      connect: async () => browser,
      wait: async () => {}
    }
  );
  assert.equal(result, browser);
  assert.equal(probes, 2);
});

test('an unavailable debug endpoint does not attach to an unrelated browser', async () => {
  const result = await agent.connectToExistingBrowser(
    { debugPort: 9503, name: '前程无忧' },
    { probe: async () => false, connect: async () => { throw Error('must not connect'); } }
  );
  assert.equal(result, null);
});

test('persistent CDP timeout reports a resumable error without closing existing pages', async () => {
  let attempts = 0;
  await assert.rejects(
    agent.connectToExistingBrowser(
      { debugPort: 9503, name: '前程无忧' },
      {
        probe: async () => true,
        connect: async () => { attempts++; throw Error('Network.enable timed out'); },
        wait: async () => {}
      }
    ),
    /窗口和网页已保留；可稍后手动继续/
  );
  assert.equal(attempts, 2);
});

test('search opens its own background tab and leaves existing pages untouched', async () => {
  let detached = false;
  let navigations = 0;
  let closes = 0;
  const searchPage = { url: () => 'https://ehire.51job.com/Revision/talent/search' };
  const session = {
    send: async (command, args) => {
      assert.equal(command, 'Target.createTarget');
      assert.deepEqual(args, { url: searchPage.url(), background: true });
      return { targetId: 'search-target' };
    },
    detach: async () => { detached = true; }
  };
  const existingPage = {
    url: () => 'https://example.test/important',
    goto: async () => { navigations++; },
    close: async () => { closes++; },
    target: () => ({ createCDPSession: async () => session })
  };
  const browser = {
    pages: async () => [existingPage],
    waitForTarget: async predicate => {
      const target = { _targetId: 'search-target', page: async () => searchPage };
      assert.equal(predicate(target), true);
      return target;
    }
  };
  assert.equal(await agent.openDedicatedSearchPage(browser, { homeUrl: searchPage.url() }), searchPage);
  assert.equal(navigations, 0);
  assert.equal(closes, 0);
  assert.equal(detached, true);
});

test('reused browser window stays visible while its search tab runs', async () => {
  const browser = {};
  agent.preserveBrowserWindow(browser);
  const page = {
    browser: () => browser,
    target: () => { throw Error('reused window must not be minimized'); }
  };
  assert.equal(await agent.setBrowserWindowState(page, 'minimized'), true);
});

test('browser minimization targets a page window and releases its CDP session', async () => {
  const commands = [];
  let detached = false;
  const session = {
    send: async (command, args) => {
      commands.push({ command, args });
      if (command === 'Browser.getWindowForTarget') return { windowId: 17 };
      return {};
    },
    detach: async () => { detached = true; }
  };
  const page = { target: () => ({ createCDPSession: async () => session }) };
  const browser = {
    pages: async () => [page],
    target: () => { throw Error('browser target must not be used'); }
  };
  assert.equal(await agent.setBrowserWindowState(browser, 'minimized'), true);
  assert.deepEqual(commands[1], {
    command: 'Browser.setWindowBounds',
    args: { windowId: 17, bounds: { windowState: 'minimized' } }
  });
  assert.equal(detached, true);
});

test('manual candidate viewing prevents the search worker from minimizing that browser', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'talentlens-view-'));
  const marker = path.join(dir, 'manual_view_51job.flag');
  fs.writeFileSync(marker, String(Date.now()));
  const commands = [];
  const session = {
    send: async (command, args) => {
      commands.push({ command, args });
      return command === 'Browser.getWindowForTarget' ? { windowId: 17 } : {};
    },
    detach: async () => {}
  };
  const page = {
    url: () => 'https://ehire.51job.com/Revision/talent/search',
    target: () => ({ createCDPSession: async () => session })
  };
  try {
    assert.equal(await agent.setBrowserWindowState(page, 'minimized', {
      manualViewFile: marker,
      searchStartedAt: Date.now() - 2000
    }), true);
    assert.equal(commands.length, 0);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('window minimization retries while a newly opened detail tab has no window ID', async () => {
  let attempts = 0;
  const page = { target: () => ({ createCDPSession: async () => ({
    send: async command => {
      if (command === 'Browser.getWindowForTarget') return attempts++ === 0 ? {} : { windowId: 23 };
      return {};
    },
    detach: async () => {}
  }) }) };
  assert.equal(await agent.setBrowserWindowState(page, 'minimized'), true);
  assert.equal(attempts, 2);
});

test('detail tab is created in the background without activating the browser', async () => {
  const commands = [];
  let detached = false;
  const detailPage = { url: () => 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=42' };
  const session = {
    send: async (command, args) => {
      commands.push({ command, args });
      return { targetId: 'detail-target' };
    },
    detach: async () => { detached = true; }
  };
  const sourcePage = {
    url: () => 'https://ehire.51job.com/Revision/talent/search',
    target: () => ({ createCDPSession: async () => session })
  };
  const browser = {
    newPage: async () => { throw Error('foreground newPage must not be used'); },
    waitForTarget: async predicate => {
      const target = { _targetId: 'detail-target', page: async () => detailPage };
      assert.equal(predicate(target), true);
      return target;
    }
  };
  assert.equal(await agent.createBackgroundPage(browser, sourcePage, '//ehire.51job.com/Revision/talent/resume/detail?resumeId=42'), detailPage);
  assert.deepEqual(commands[0], {
    command: 'Target.createTarget',
    args: { url: detailPage.url(), background: true }
  });
  assert.equal(detached, true);
});

test('51job captures the exact candidate detail URL without opening a foreground tab', async () => {
  const originalOpen = () => { throw Error('foreground popup must not open'); };
  global.window = { open: originalOpen };
  const wrongName = { innerText: '陈先生', click: () => global.window.open('//ehire.51job.com/Revision/talent/resume/detail?resumeId=1', '_blank') };
  const rightName = { innerText: '陈先生', click: () => global.window.open('//ehire.51job.com/Revision/talent/resume/detail?resumeId=2', '_blank') };
  const wrongCard = { innerText: '陈先生\n8年经验', querySelector: () => wrongName };
  const rightCard = { innerText: '陈先生\n12年经验', querySelector: () => rightName };
  global.document = { querySelectorAll: () => [wrongCard, rightCard] };
  const page = {
    url: () => 'https://ehire.51job.com/Revision/talent/search',
    evaluate: async (fn, ...args) => fn(...args)
  };
  try {
    const result = await agent.capture51jobDetailUrl(page, { name: '陈先生', rawCardText: rightCard.innerText });
    assert.equal(result.url, 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=2');
    assert.equal(global.window.open, originalOpen);
  } finally {
    delete global.window;
    delete global.document;
  }
});

test('51job delayed detail opening never reaches the foreground popup API', async () => {
  let foregroundOpens = 0;
  const originalOpen = () => { foregroundOpens++; return null; };
  const url = 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=42';
  let popupFinished;
  const popup = new Promise(resolve => { popupFinished = resolve; });
  const name = { innerText: '测试候选人', click() {
    setTimeout(() => { global.window.open(url, '_blank'); popupFinished(); }, 20);
  } };
  const card = { innerText: '测试候选人\n5年经验', querySelector: () => name };
  global.window = { open: originalOpen };
  global.document = { querySelectorAll: () => [card] };
  const page = {
    url: () => 'https://ehire.51job.com/Revision/talent/search',
    evaluate: async (fn, ...args) => fn(...args)
  };
  try {
    const result = await agent.capture51jobDetailUrl(page, { name: name.innerText, rawCardText: card.innerText });
    await popup;
    assert.equal(foregroundOpens, 0, 'delayed window.open must not activate a native browser tab');
    assert.equal(result.url, url);
    assert.equal(global.window.open, originalOpen);
  } finally {
    delete global.window;
    delete global.document;
  }
});

test('51job detail capture releases its hook on timeout or click failure', async () => {
  const originalOpen = () => null;
  const name = { innerText: '测试候选人', click() {} };
  const card = { innerText: '测试候选人', querySelector: () => name };
  global.window = { open: originalOpen };
  global.document = { querySelectorAll: () => [card] };
  const page = {
    url: () => 'https://ehire.51job.com/Revision/talent/search',
    evaluate: async (fn, ...args) => fn(...args)
  };
  try {
    assert.deepEqual(await agent.capture51jobDetailUrl(page, { name: name.innerText }, 10), { clicked: true, url: '' });
    assert.equal(global.window.open, originalOpen);
    name.click = () => { throw Error('click failed'); };
    await assert.rejects(agent.capture51jobDetailUrl(page, { name: name.innerText }, 10), /click failed/);
    assert.equal(global.window.open, originalOpen);
  } finally {
    delete global.window;
    delete global.document;
  }
});

test('Edge new tab navigates to the recruiting site before login is checked', async () => {
  let currentUrl = 'edge://newtab/';
  const visits = [];
  const page = {
    url: () => currentUrl,
    goto: async url => { visits.push(url); currentUrl = url; }
  };
  const cfg = { homeUrl: 'https://ehire.51job.com/Revision/talent/search', loginUrl: 'https://ehire.51job.com/MainLogin.aspx' };
  assert.equal(agent.isBrowserStartPage(currentUrl), true);
  assert.deepEqual(await agent.ensurePlatformPage(page, cfg), { ok: true });
  assert.deepEqual(visits, [cfg.homeUrl]);
  assert.equal(agent.isBrowserStartPage(page.url()), false);
});

test('Edge MSN start page is not mistaken for a recruiting login page', async () => {
  let currentUrl = 'https://ntp.msn.cn/edge/ntp?locale=zh-CN';
  const cfg = { homeUrl: 'https://ehire.51job.com/Revision/talent/search', loginUrl: 'https://ehire.51job.com/MainLogin.aspx' };
  const visits = [];
  const page = {
    url: () => currentUrl,
    goto: async url => { visits.push(url); currentUrl = url; }
  };
  assert.equal(agent.isBrowserStartPage(currentUrl), true);
  assert.equal(agent.isPlatformPageUrl(currentUrl, cfg), false);
  assert.deepEqual(await agent.ensurePlatformPage(page, cfg), { ok: true });
  assert.deepEqual(visits, [cfg.homeUrl]);
  assert.equal(agent.isPlatformPageUrl(page.url(), cfg), true);
});

test('Edge starts once on the recruiting URL in a minimized window', () => {
  const cfg = { debugPort: 9503, homeUrl: 'https://ehire.51job.com/Revision/talent/search' };
  assert.deepEqual(agent.browserLaunchArgs(cfg, 'C:\\TalentLens\\51job_profile', false), [
    '--remote-debugging-port=9503', '--user-data-dir=C:\\TalentLens\\51job_profile',
    '--no-first-run', '--no-default-browser-check', '--start-minimized', cfg.homeUrl
  ]);
});

test('unused Edge new-tab window is closed without closing the recruiting tab', async () => {
  let closed = 0;
  const selected = { url: () => 'https://ehire.51job.com/Revision/talent/search', close: async () => { throw Error('search tab must stay open'); } };
  const startPage = { url: () => 'edge://newtab/', close: async () => { closed++; } };
  await agent.closeUnusedStartPages({ pages: async () => [selected, startPage] }, selected);
  assert.equal(closed, 1);
});

test('51job search clicks the real button instead of its same-text parent', async () => {
  let parentClicks = 0;
  let buttonClicks = 0;
  const input = { value: '', placeholder: '搜索职位名', focus() {}, dispatchEvent() {} };
  const parent = { innerText: '搜索', click() { parentClicks++; } };
  const button = { innerText: '搜索', disabled: false, click() { buttonClicks++; global.document.body.innerText = ''; } };
  global.document = {
    body: { innerText: '输入关键词搜索，寻找匹配人才' },
    querySelector: selector => selector.includes('button.search_button') ? button : input,
    querySelectorAll: selector => selector.includes('.talent-search-container') ? [] : selector === 'input' ? [input] : selector === 'button' ? [button] : [parent, button]
  };
  const page = { url: () => 'https://ehire.51job.com/Revision/talent/search', waitForSelector: async () => {}, evaluate: async (fn, ...args) => fn(...args) };
  try {
    assert.deepEqual(await agent.submit51jobSearch(page, '软件测试工程师'), { ok: true });
    assert.equal(input.value, '软件测试工程师');
    assert.equal(buttonClicks, 1);
    assert.equal(parentClicks, 0);
  } finally {
    delete global.document;
  }
});

test('51job search reports failure when the button is missing', async () => {
  const input = { value: '', focus() {}, dispatchEvent() {} };
  global.document = {
    body: { innerText: '输入关键词搜索，寻找匹配人才' },
    querySelector: selector => selector.includes('button.search_button') ? null : input,
    querySelectorAll: selector => selector === 'button' ? [] : [input]
  };
  try {
    assert.deepEqual(await agent.submit51jobSearch({ waitForSelector: async () => {}, evaluate: async (fn, ...args) => fn(...args) }, '软件测试工程师'), {
      ok: false,
      reason: '没有找到可点击的搜索按钮'
    });
  } finally {
    delete global.document;
  }
});

test('51job search never reports success while the initial prompt remains', async () => {
  let clicks = 0;
  const input = { value: '', focus() {}, dispatchEvent() {} };
  const button = { innerText: '搜索', disabled: false, click() { clicks++; } };
  global.document = {
    body: { innerText: '输入关键词搜索，寻找匹配人才' },
    querySelector: selector => selector.includes('button.search_button') ? button : input,
    querySelectorAll: selector => selector.includes('.talent-search-container') ? [] : selector === 'button' ? [button] : [input]
  };
  const page = { waitForSelector: async () => {}, evaluate: async (fn, ...args) => fn(...args) };
  try {
    const result = await agent.submit51jobSearch(page, '软件测试工程师');
    assert.equal(result.ok, false);
    assert.match(result.reason, /检索未生效/);
    assert.equal(clicks, 3);
  } finally {
    delete global.document;
  }
});

test('captcha detection error is treated as unsafe', async () => {
  const result = await agent.detectCaptcha({ url: () => 'https://example.test', evaluate: async () => { throw Error('context lost'); }, frames: () => [] });
  assert.equal(result.detected, true);
});

test('search skips cards without verifiable filters', async () => {
  const card = {
    offsetParent: {}, getClientRects: () => [1],
    innerText: '王女士 上海 本科 4年经验 临床项目经理，负责临床项目交付',
    querySelector: selector => selector.includes('.firstline .name') ? { innerText: '王女士' } : null,
    querySelectorAll: () => [], getAttribute: () => null
  };
  global.document = { querySelectorAll: () => [card] };
  const page = { evaluate: async (fn, ...args) => fn(...args), frames: () => [] };
  const good = await agent.extractCandidatesAcrossFrames(page, 10, '临床项目经理', 'test', [], [], { city: '上海', exp: '3年', edu: '本科' });
  const badCity = await agent.extractCandidatesAcrossFrames(page, 10, '临床项目经理', 'test', [], [], { city: '北京', exp: '3年', edu: '本科' });
  const badExp = await agent.extractCandidatesAcrossFrames(page, 10, '临床项目经理', 'test', [], [], { city: '上海', exp: '5年', edu: '本科' });
  const unrestricted = await agent.extractCandidatesAcrossFrames(page, 10, '临床项目经理', 'test', [], [], { city: '全国', exp: '不限', edu: '不限' });
  assert.equal(good.length, 1);
  assert.equal(unrestricted.length, 1);
  assert.equal(badCity.length, 0);
  assert.equal(badExp.length, 0);
  delete global.document;
});

test('read or already collected cards do not consume the new-candidate quota', () => {
  const cards = Array.from({ length: 6 }, (_, index) => ({
    name: `候选人${index}`,
    workText: '临床项目经验',
    infoText: index === 0 ? '已读 · 3年经验' : '3年经验',
    url: `https://example.test/resume?id=${index}`
  }));
  const saved = agent.selectFreshCandidates(cards, 5, new Set([cards[0].url]));
  assert.equal(saved.length, 5);
  assert.equal(saved[0].name, '候选人1');
  assert.equal(saved[4].name, '候选人5');
  assert.equal(agent.selectFreshCandidates(cards.slice(0, 1), 1).length, 1);
  const sameName = [
    { name: '王女士', workText: '', infoText: '已读', url: 'https://example.test/resume?id=10' },
    { name: '王女士', workText: '', infoText: '未读', url: 'https://example.test/resume?id=11' }
  ];
  assert.equal(agent.selectFreshCandidates(sameName, 2).length, 2);
});

test('pagination only advances when candidate page actually changes', async () => {
  let pageNumber = 1;
  let canAdvance = true;
  const button = {
    offsetParent: {}, getClientRects: () => [1], disabled: false, className: '',
    getAttribute: () => null,
    click() { if (canAdvance) pageNumber++; }
  };
  const card = {
    offsetParent: {}, getClientRects: () => [1],
    querySelector: selector => selector.includes('.firstline .name') ? { textContent: `候选人${pageNumber}` } : null,
    getAttribute: () => null
  };
  global.location = { pathname: '/search' };
  global.window = { scrollBy() {} };
  global.document = {
    querySelectorAll: selector => selector.includes('button.btn-next') ? [button] : [card],
    querySelector: () => ({ textContent: String(pageNumber) }),
    body: {}
  };
  const page = { frames: () => [page], evaluate: async fn => fn(), mouse: { wheel: async () => {} } };
  assert.equal(await agent.advanceSearchResults(page, { waitStepMs: 1 }), true);
  canAdvance = false;
  assert.equal(await agent.advanceSearchResults(page, { waitStepMs: 1 }), false);
  delete global.location;
  delete global.window;
  delete global.document;
});

test('pagination waits for a slow platform render before declaring the results exhausted', async () => {
  let fingerprints = 0;
  let clicked = false;
  const page = {
    frames: () => [page],
    evaluate: async fn => {
      if (String(fn).includes('button.btn-next')) {
        clicked = true;
        return true;
      }
      if (!String(fn).includes('cardSelector')) return undefined;
      fingerprints++;
      return fingerprints >= 7 ? 'new results' : 'old results';
    },
    mouse: { wheel: async () => {} }
  };
  assert.equal(await agent.advanceSearchResults(page, { waitStepMs: 1 }), true);
  assert.equal(clicked, true);
});

test('virtual list scroll counts as progress until its real bottom', async () => {
  const body = {};
  const scroller = {
    parentElement: body, scrollHeight: 2000, clientHeight: 500,
    getBoundingClientRect: () => ({ left: 0, top: 0, width: 800, height: 500 })
  };
  let scrollTop = 0;
  Object.defineProperty(scroller, 'scrollTop', {
    get: () => scrollTop,
    set: value => { scrollTop = Math.max(0, Math.min(1500, value)); }
  });
  const card = {
    parentElement: scroller, offsetParent: {}, getClientRects: () => [1],
    querySelector: selector => selector.includes('.firstline .name') ? { textContent: '同一组卡片' } : null,
    getAttribute: () => null
  };
  global.location = { pathname: '/search' };
  global.document = {
    body,
    querySelectorAll: selector => selector.includes('button.btn-next') ? [] : [card],
    querySelector: selector => selector.includes('.talent-search-container .card') ? card : null
  };
  const page = {
    frames: () => [page], evaluate: async (fn, ...args) => fn(...args),
    mouse: { wheel: async () => { throw Error('mouse wheel must not be used'); } }
  };
  assert.equal(await agent.advanceSearchResults(page, { waitStepMs: 1 }), true);
  scroller.scrollTop = 1500;
  assert.equal(await agent.advanceSearchResults(page, { waitStepMs: 1 }), false);
  delete global.location;
  delete global.document;
});

test('candidate action never falls back to a different single card', () => {
  const source = fs.readFileSync(require.resolve('./multi_platform_agent'), 'utf8');
  const begin = source.indexOf('(act, name, expectedUrl, pageUrl) => {');
  const end = source.indexOf('}, options.action, cleanName, options.candidateUrl, u)', begin);
  assert.ok(begin >= 0 && end > begin);
  let clicks = 0;
  const button = { innerText: '打招呼', className: '', offsetParent: {}, getClientRects: () => [1], scrollIntoView() {}, click() { clicks++; } };
  const card = { innerText: '李女士', querySelector: () => ({ innerText: '李女士' }), querySelectorAll: () => [button] };
  const context = { document: { querySelectorAll: () => [card], querySelector: () => null, body: {} }, URL, window: {} };
  const action = vm.runInNewContext('(' + source.slice(begin, end + 1) + ')', context);
  const result = action('greet', '张先生', '', 'https://example.test/list');
  assert.equal(result.ok, false);
  assert.equal(clicks, 0);
});
