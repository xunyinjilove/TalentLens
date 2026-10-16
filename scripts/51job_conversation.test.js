const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { parseCandidateUrl, sameCandidatePage, sameDisplayedName, attachmentFingerprint, selectInboundAttachment,
  validResumeBytes, sendCandidateMessage, inspectReply, findCandidateChatEntry, openCandidateChat,
  searchCommunicationJob, clickExactCommunicationJob, selectCommunicationJob, platformChatGateVisible,
  readHiChatQuote, previewHiChat, clickQuotedHiChat, confirmHiChat,
  readCommunicationJobs, listCommunicationJobs, setCommunicationJobByID, selectCommunicationJobByID,
  waitCandidateReady, candidatePage } = require('./51job_conversation');

test('51job detail opens chat through its visible div.chat_btn entry', () => {
  const entry = { innerText: '立即Hi聊', className: 'chat_btn', getClientRects: () => [1] };
  const doc = { querySelectorAll: selector => selector.includes('.btn_item_chat .chat_btn') ? [entry] : [] };
  assert.equal(findCandidateChatEntry(doc), entry);
});

test('51job job search remains available when the input placeholder becomes the selected job name', () => {
  const oldDocument = global.document;
  const oldInput = global.HTMLInputElement;
  const oldKeyboard = global.KeyboardEvent;
  const input = { value: '', focus: () => {}, dispatchEvent: () => {} };
  const dialog = {
    getClientRects: () => [1],
    querySelector: selector => selector === '.eh_forwarder_searcher input' ? input
      : selector === '.el-dialog__footer' ? {}
        : selector === '.list_box_container' ? { getClientRects: () => [1] } : null
  };
  global.document = { querySelectorAll: () => [dialog] };
  global.HTMLInputElement = class {};
  global.KeyboardEvent = class {};
  try {
    assert.equal(platformChatGateVisible(), true);
    assert.equal(searchCommunicationJob('临床项目经理'), true);
    assert.equal(input.value, '临床项目经理');
  } finally {
    global.document = oldDocument;
    global.HTMLInputElement = oldInput;
    global.KeyboardEvent = oldKeyboard;
  }
});

test('opening 51job chat clicks the visible entry and verifies the chat identity', async () => {
  let clicks = 0;
  let reads = 0;
  const entry = { innerText: '立即Hi聊', getClientRects: () => [1], click: () => { clicks++; } };
  const oldDocument = global.document;
  global.document = {
    querySelectorAll: selector => selector.startsWith('.el-dialog') ? [] : [entry],
    querySelector: selector => selector === '#chatAreaEmbed .username-text'
      ? { textContent: '测试候选人' }
      : selector === '#chatAreaEmbed'
        ? { closest: () => ({ id: 'im-sdk-container' }) }
        : null
  };
  try {
    const page = {
      evaluate: async callback => ++reads === 1
        ? { visible: false, embedded: false, name: '' }
        : callback(),
      waitForSelector: async () => {}
    };
    await openCandidateChat(page, '测试候选人');
    assert.equal(clicks, 1);
  } finally {
    global.document = oldDocument;
  }
});

test('51job only clicks a unique exact communication job', () => {
  const oldDocument = global.document;
  let clicks = 0;
  const row = name => ({ getClientRects: () => [1], querySelector: () => ({ textContent: name }), click: () => { clicks++; } });
  let rows = [row('临床项目经理（IVD行业）')];
  const dialog = { getClientRects: () => [1], querySelectorAll: () => rows };
  global.document = { querySelectorAll: () => [dialog] };
  try {
    assert.equal(clickExactCommunicationJob('临床项目经理').status, 'not_found');
    assert.equal(clicks, 0);
    rows = [row('临床项目经理'), row('临床项目经理')];
    assert.equal(clickExactCommunicationJob('临床项目经理').status, 'ambiguous');
    assert.equal(clicks, 0);
    rows = [row('临床项目经理')];
    assert.equal(clickExactCommunicationJob('临床项目经理').status, 'clicked');
    assert.equal(clicks, 1);
  } finally {
    global.document = oldDocument;
  }
});

test('51job selects the project job but never starts HiChat or sends a message', async () => {
  const operations = [];
  const page = {
    createCDPSession: async () => ({ send: async method => { operations.push(method); }, detach: async () => {} }),
    evaluate: async (callback, title) => {
      operations.push(callback.name);
      assert.equal(title, '软件测试工程师');
      if (callback.name === '') return false;
      if (callback === searchCommunicationJob) return true;
      if (callback === clickExactCommunicationJob) return { status: 'clicked' };
      throw new Error('unexpected page operation');
    },
    waitForFunction: async () => true
  };
  const result = await selectCommunicationJob(page, '软件测试工程师');
  assert.equal(result.status, 'needs_confirmation');
  assert.match(result.message, /本次软件未发送消息/);
  assert.deepEqual(operations.filter(x => x === 'clickExactCommunicationJob'), ['clickExactCommunicationJob']);
});

test('opening chat stops at the 51job points gate after selecting the project job', async () => {
  let anonymousCalls = 0;
  const page = {
    createCDPSession: async () => ({ send: async () => {}, detach: async () => {} }),
    evaluate: async callback => {
      if (callback === platformChatGateVisible) return true;
      if (callback === searchCommunicationJob) return true;
      if (callback === clickExactCommunicationJob) return { status: 'clicked' };
      return ++anonymousCalls === 1 ? { visible: false, embedded: false, name: '' }
        : anonymousCalls === 2 ? true : false;
    },
    waitForFunction: async () => true,
    waitForSelector: async () => { throw new Error('must not wait for a chat box before platform confirmation'); }
  };
  const result = await openCandidateChat(page, '测试候选人', '软件测试工程师');
  assert.equal(result.status, 'needs_confirmation');
});

test('a missing project job cannot select a 51job position by guesswork', async () => {
  const result = await selectCommunicationJob({}, '');
  assert.equal(result.status, 'needs_job_selection');
});

test('51job job picker lists platform IDs and chooses the HR-selected ID even for identical titles', async () => {
  const oldDocument = global.document;
  const chosen = [];
  const picker = {
    myJobList: [{ jobid: '101', job_name: '测试工程师', coid: '1', coname: '甲公司', workarea: '武汉', publish_status: '发布中' }],
    companJobList: [{ jobid: '202', job_name: '测试工程师', coid: '2', coname: '乙公司', workarea: '上海', publish_status: '发布中' }],
    jobList: [], moreMyJob: '0', moreCompanyJob: '1',
    setJob: (...args) => { chosen.push(args); }
  };
  const dialog = {
    getClientRects: () => [1],
    querySelector: () => ({ parentElement: { __vue__: picker } })
  };
  global.document = { querySelectorAll: () => [dialog] };
  try {
    const list = readCommunicationJobs('');
    assert.equal(list.status, 'jobs');
    assert.deepEqual(list.jobs.map(job => job.id), ['101', '202']);
    assert.equal(list.jobs[1].company, '乙公司');
    assert.equal(list.hasMore, true);
    assert.equal(setCommunicationJobByID('202', '测试工程师'), true);
    assert.deepEqual(chosen, [['2', '乙公司', '202', '测试工程师']]);
    assert.equal(setCommunicationJobByID('999', '测试工程师'), false);
    assert.equal(chosen.length, 1);
    const page = {
      createCDPSession: async () => ({ send: async () => {}, detach: async () => {} }),
      evaluate: async (callback, ...args) => callback(...args),
      waitForFunction: async () => true
    };
    assert.equal((await selectCommunicationJobByID(page, '101', '测试工程师')).status, 'needs_confirmation');
    assert.equal(chosen.at(-1)[2], '101');
  } finally { global.document = oldDocument; }
});

test('51job keyword search waits for its platform response instead of returning stale jobs', async () => {
  let responseFilter;
  const page = {
    createCDPSession: async () => ({ send: async () => {}, detach: async () => {} }),
    waitForResponse: async predicate => {
      responseFilter = predicate;
      throw new Error('network timeout');
    },
    evaluate: async callback => callback === searchCommunicationJob ? true : { status: 'jobs', jobs: [{ id: 'old' }] },
    waitForFunction: async () => true
  };
  const result = await listCommunicationJobs(page, '软件测试');
  assert.equal(responseFilter({ url: () => 'https://ehire.51job.com/jobj/list/v1/get_talent_joblist', status: () => 200 }), true);
  assert.equal(responseFilter({ url: () => 'https://ehire.51job.com/jobj/list/get_combine_joblist', status: () => 200 }), false);
  assert.equal(result.status, 'error');
  assert.equal(result.jobs, undefined);
});

test('51job point quote is read from the selected job dialog without sending', async () => {
  const oldDocument = global.document;
  const picker = { jobId: 'job-123', jobName: '软件测试工程师', loading: false };
  const gate = { jobId: 'job-123', jobName: '软件测试工程师', loading: false };
  const button = { textContent: '发起Hi聊', disabled: false, getClientRects: () => [1] };
  const message = { textContent: '本次已选择 1 位人才，需消耗 2 个Hi聊数' };
  const dialog = {
    getClientRects: () => [1],
    parentElement: { parentElement: { __vue__: gate } },
    querySelector: selector => selector === '.eh_forwarder_searcher' ? { parentElement: { __vue__: picker } }
      : selector === '.msg' ? message : null,
    querySelectorAll: () => [button]
  };
  global.document = { querySelectorAll: () => [dialog] };
  try {
    assert.deepEqual(readHiChatQuote(), { jobId: 'job-123', jobTitle: '软件测试工程师', points: 2, ready: true });
    const page = {
      waitForFunction: async () => true,
      evaluate: async callback => callback(),
      target: () => ({ _targetId: 'tab-123' })
    };
    const result = await previewHiChat(page, '软件测试工程师');
    assert.equal(result.status, 'quote');
    assert.equal(result.points, 2);
    gate.jobId = 'another-job';
    assert.equal((await previewHiChat(page, '软件测试工程师')).status, 'needs_manual_confirmation');
    gate.jobId = 'job-123';
    message.textContent = '选择职位查看消耗点数';
    assert.equal((await previewHiChat(page, '软件测试工程师')).status, 'needs_manual_confirmation');
  } finally { global.document = oldDocument; }
});

test('51job final click rechecks the candidate, job and points together', () => {
  const oldDocument = global.document;
  let clicks = 0;
  const picker = { jobId: 'job-123', jobName: '软件测试工程师', loading: false };
  const gate = { jobId: 'job-123', jobName: '软件测试工程师', loading: false };
  const button = { textContent: '发起Hi聊', disabled: false, getClientRects: () => [1], click: () => { clicks++; } };
  const cost = { textContent: '本次需消耗 2 个Hi聊数' };
  const dialog = {
    getClientRects: () => [1], parentElement: { parentElement: { __vue__: gate } },
    querySelector: selector => selector === '.eh_forwarder_searcher' ? { parentElement: { __vue__: picker } }
      : selector === '.msg' ? cost : null,
    querySelectorAll: () => [button]
  };
  global.document = {
    URL: 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123',
    title: '测试候选人', querySelectorAll: () => [dialog]
  };
  const click = () => clickQuotedHiChat('123', '测试候选人', 'job-123', '软件测试工程师', 2);
  try {
    assert.deepEqual(click(), { clicked: true });
    assert.equal(clicks, 1);
    cost.textContent = '本次需消耗 3 个Hi聊数';
    assert.deepEqual(click(), { clicked: false, reason: 'points' });
    cost.textContent = '本次需消耗 2 个Hi聊数';
    gate.jobId = 'job-456';
    assert.deepEqual(click(), { clicked: false, reason: 'job' });
    gate.jobId = 'job-123';
    global.document.URL = 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=456';
    assert.deepEqual(click(), { clicked: false, reason: 'candidate' });
    assert.equal(clicks, 1);
  } finally { global.document = oldDocument; }
});

test('51job HiChat confirmation clicks only after job and point quote still match', async () => {
  const url = 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123';
  let clicks = 0;
  const page = {
    url: () => url,
    title: async () => '测试候选人',
    createCDPSession: async () => ({ send: async () => {}, detach: async () => {} }),
    evaluate: async callback => {
      if (callback === readHiChatQuote) return { ready: true, jobId: 'job-123', jobTitle: '软件测试工程师', points: 2 };
      if (callback === clickQuotedHiChat) { clicks++; return { clicked: true }; }
      return 0;
    },
    waitForFunction: async () => true
  };
  const browser = { targets: () => [{ _targetId: 'tab-123', page: async () => page }] };
  const args = { targetID: 'tab-123', name: '测试候选人', jobTitle: '软件测试工程师', expectedJobID: 'job-123', expectedPoints: '3' };
  const expected = parseCandidateUrl(url);
  assert.equal((await confirmHiChat(browser, args, expected)).status, 'uncertain');
  assert.equal(clicks, 0);
  args.expectedPoints = '2';
  assert.equal((await confirmHiChat(browser, args, expected)).status, 'sent');
  assert.equal(clicks, 1);
  page.waitForFunction = async () => { throw new Error('timeout'); };
  assert.equal((await confirmHiChat(browser, args, expected)).status, 'error');
  assert.equal(clicks, 1);
});

test('51job confirmation waits for the picker to settle after focus emulation', async () => {
  const url = 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123';
  let pickerLoading = false;
  let clicks = 0;
  const page = {
    url: () => url,
    title: async () => '测试候选人',
    createCDPSession: async () => ({
      send: async (method, options) => {
        if (method === 'Emulation.setFocusEmulationEnabled' && options.enabled) pickerLoading = true;
      },
      detach: async () => {}
    }),
    evaluate: async callback => {
      if (callback === readHiChatQuote) {
        return { ready: !pickerLoading, jobId: 'job-123', jobTitle: '软件测试工程师', points: 2 };
      }
      if (callback === clickQuotedHiChat) {
        if (pickerLoading) return { clicked: false, reason: 'loading' };
        clicks++;
        return { clicked: true };
      }
      return 0;
    },
    waitForFunction: async () => { pickerLoading = false; return true; }
  };
  const browser = { targets: () => [{ _targetId: 'tab-123', page: async () => page }] };
  const args = { targetID: 'tab-123', name: '测试候选人', jobTitle: '软件测试工程师', expectedJobID: 'job-123', expectedPoints: '2' };
  const result = await confirmHiChat(browser, args, parseCandidateUrl(url));
  assert.equal(result.status, 'sent');
  assert.equal(clicks, 1);
});

test('51job confirmation does not click when the settled point quote changes', async () => {
  const url = 'https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123';
  let focusEnabled = false;
  let clicks = 0;
  const page = {
    url: () => url,
    title: async () => '测试候选人',
    createCDPSession: async () => ({
      send: async (method, options) => {
        if (method === 'Emulation.setFocusEmulationEnabled' && options.enabled) focusEnabled = true;
      },
      detach: async () => {}
    }),
    evaluate: async callback => {
      if (callback === readHiChatQuote) {
        return { ready: true, jobId: 'job-123', jobTitle: '软件测试工程师', points: focusEnabled ? 3 : 2 };
      }
      if (callback === clickQuotedHiChat) { clicks++; return { clicked: true }; }
      return 0;
    },
    waitForFunction: async () => true
  };
  const browser = { targets: () => [{ _targetId: 'tab-123', page: async () => page }] };
  const args = { targetID: 'tab-123', name: '测试候选人', jobTitle: '软件测试工程师', expectedJobID: 'job-123', expectedPoints: '2' };
  const result = await confirmHiChat(browser, args, parseCandidateUrl(url));
  assert.equal(result.status, 'error');
  assert.match(result.message, /点数与 HR 确认的不一致/);
  assert.equal(clicks, 0);
});

test('51job actions require an exact candidate detail URL and resume ID', () => {
  const expected = parseCandidateUrl('https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123&rt=1');
  assert.equal(expected.resumeId, '123');
  assert.equal(sameCandidatePage('https://ehire.51job.com/Revision/talent/resume/detail?resumeId=124', expected), false);
  assert.throws(() => parseCandidateUrl('https://ehire.51job.com.evil.test/Revision/talent/resume/detail?resumeId=123'));
  assert.throws(() => parseCandidateUrl('https://ehire.51job.com/Revision/talent/search?resumeId=123'));
  assert.equal(sameDisplayedName('余女士', '余小姐'), true);
  assert.equal(sameDisplayedName('余女士', '王小姐'), false);
});

test('only an inbound resume attachment from 51job can be imported', () => {
  const messages = [
    { mine: false, links: [{ href: 'https://ehire.51job.com/files/old.pdf', text: 'old.pdf' }] },
    { mine: true, links: [{ href: 'https://ehire.51job.com/files/sent.docx', text: 'sent.docx' }] },
    { mine: false, links: [{ href: 'https://ehire.51job.com.evil.test/steal.pdf', text: 'steal.pdf' }] },
    { mine: false, links: [{ href: 'https://ehire.51job.com/files/latest.pdf', text: 'latest.pdf' }] }
  ];
  assert.equal(selectInboundAttachment(messages).fileName, 'latest.pdf');
  assert.equal(selectInboundAttachment(messages.slice(1, 3)), null);
  assert.equal(selectInboundAttachment(messages, [attachmentFingerprint(messages[3].links[0])]).fileName, 'old.pdf');
});

test('attachment content must match its declared file type', () => {
  assert.equal(validResumeBytes(Buffer.from('%PDF-' + 'x'.repeat(50)), 'pdf'), true);
  assert.equal(validResumeBytes(Buffer.from('<html>' + 'x'.repeat(50)), 'pdf'), false);
  assert.equal(validResumeBytes(Buffer.from('PK' + 'x'.repeat(50)), 'docx'), true);
});

test('message click without a verified sent bubble is not reported as success', async () => {
  let clicked = false;
  const page = {
    evaluate: async () => ({ ready: true, count: 0 }),
    type: async () => {},
    $: async () => ({ click: async () => { clicked = true; } }),
    waitForFunction: async () => { throw new Error('timeout'); }
  };
  await assert.rejects(sendCandidateMessage(page, '请发送完整简历'), /未取得平台已发送状态/);
  assert.equal(clicked, true);
});

test('identical text already in the candidate chat is never sent twice', async () => {
  const page = { evaluate: async () => ({ ready: true, count: 1 }), type: async () => { throw new Error('must not type'); } };
  await assert.rejects(sendCandidateMessage(page, '您好'), /未重复发送/);
});

test('asking for a resume remembers prior inbound attachments before sending', async () => {
  let reads = 0;
  const old = { href: 'https://ehire.51job.com/files/old.pdf?token=1', text: 'old.pdf' };
  const page = {
    evaluate: async () => ++reads === 1 ? { ready: true, count: 0 } : [{ mine: false, links: [old] }],
    type: async () => {},
    $: async () => ({ click: async () => {} }),
    waitForFunction: async () => {}
  };
  const result = await sendCandidateMessage(page, '请发送完整简历', true);
  assert.equal(result.status, 'sent');
  assert.deepEqual(result.baseline, [attachmentFingerprint(old)]);
});

test('a candidate reply attachment is saved only after content verification', async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), '51job-reply-'));
  try {
    const bytes = Buffer.from('%PDF-' + 'x'.repeat(50));
    let calls = 0;
    const page = { evaluate: async () => (++calls === 1
      ? [{ mine: false, links: [{ href: 'https://ehire.51job.com/files/resume.pdf', text: 'resume.pdf' }] }]
      : bytes.toString('base64')) };
    const result = await inspectReply(page, root, 'resume_123');
    assert.equal(result.status, 'downloaded');
    assert.deepEqual(fs.readFileSync(result.filePath), bytes);
    assert.equal(path.dirname(result.filePath), path.join(root, '51job_received', 'resume_123'));
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test('candidate readiness retries a read without waiting for complete or reopening tabs', async () => {
  const expected=parseCandidateUrl('https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123');
  let attempts=0;
  const page={url:()=>expected.url,title:async()=> 'Synthetic',waitForFunction:async predicate=>{
    assert.doesNotMatch(predicate.toString(),/readyState/);
    if(++attempts===1)throw new Error('timeout');
  }};
  await waitCandidateReady(page,expected,'Synthetic');
  assert.equal(attempts,2);
  const browser={pages:async()=>[page]};
  assert.equal(await candidatePage(browser,expected,'Synthetic'),page);
  page.waitForFunction=async()=>{throw new Error('timeout');};
  await assert.rejects(waitCandidateReady(page,expected,'Synthetic'),/已自动重试/);
});

test('reply inspection returns inbound text and attachment names without sending or overwriting files', async () => {
  const messages=[{mine:true,text:'outgoing',links:[]},{mine:false,text:'方便下午沟通',links:[{href:'https://ehire.51job.com/files/resume.pdf',text:'resume.pdf'}]}];
  let calls=0;
  const result=await inspectReply({evaluate:async()=>{calls++;return messages;}},'unused','synthetic',[],false);
  assert.equal(calls,1);
  assert.equal(result.status,'reply');
  assert.deepEqual(result.replies,['方便下午沟通']);
  assert.deepEqual(result.attachmentNames,['resume.pdf']);
});
