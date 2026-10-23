const test = require('node:test');
const assert = require('node:assert/strict');
const agent = require('./multi_platform_agent');

test('repeated anonymous document copies do not consume additional capture quota', () => {
  const seen = new Set();
  assert.equal(agent.claimResumeSnapshot('王女士\n完整的合成项目经历', '', seen), true);
  assert.equal(agent.claimResumeSnapshot('王女士\r\n完整的合成项目经历', '', seen), false);
  assert.equal(agent.claimResumeSnapshot('王女士\n另一份项目经历', '', seen), true);
  assert.equal(agent.claimResumeSnapshot('王女士\n完整的合成项目经历', '123', seen), true);
  assert.equal(agent.claimResumeSnapshot('王女士\n完整的合成项目经历', '456', seen), true);
  const first = '【前程无忧 真实推荐牛人档案】\n姓名：王女士\n分流同义词：测试开发\n目标城市：上海\n实际项目经历';
  const next = first.replace('测试开发', '自动化测试').replace('上海', '北京');
  assert.equal(agent.claimResumeSnapshot(first, '', seen), true);
  assert.equal(agent.claimResumeSnapshot(next, '', seen), false);
});

test('global channel quota honors balanced and HR priority modes', () => {
  assert.equal(agent.planChannelQuota(30, 4, 'balanced'), 8);
  assert.equal(agent.planChannelQuota(22, 3, 'balanced'), 8);
  assert.equal(agent.planChannelQuota(30, 4, 'priority'), 30);
});

test('activity filter only accepts explicitly labeled recent activity', async () => {
  const makeCard = (label) => ({
    offsetParent: {}, getClientRects: () => [1],
    innerText: `王女士\n上海 本科 4年经验\n${label}\n负责临床项目交付`,
    querySelector: selector => selector.includes('.firstline .name')
      ? { innerText: '王女士' }
      : selector.includes('.userinfo') ? { innerText: '上海 本科 4年经验' } : null,
    querySelectorAll: () => [], getAttribute: () => null
  });
  const page = { evaluate: async (fn, ...args) => fn(...args), frames: () => [] };
  try {
    global.document = { querySelectorAll: () => [makeCard('3日内活跃')] };
    assert.equal((await agent.extractCandidatesAcrossFrames(page, 10, '', '', [], [], { city: '上海', activeDays: 3 })).length, 1);
    assert.equal((await agent.extractCandidatesAcrossFrames(page, 10, '', '', [], [], { city: '上海', activeDays: 1 })).length, 0);
    global.document = { querySelectorAll: () => [makeCard('未披露')] };
    assert.equal((await agent.extractCandidatesAcrossFrames(page, 10, '', '', [], [], { activeDays: 3 })).length, 0);
  } finally { delete global.document; }
});

test('51job 7-tier recent active time filter and extra keywords task builder', async () => {
  assert.equal(agent.activeDaysTo51JobLabel(7), '近1周');
  assert.equal(agent.activeDaysTo51JobLabel(14), '近2周');
  assert.equal(agent.activeDaysTo51JobLabel(30), '近1个月');
  assert.equal(agent.activeDaysTo51JobLabel(60), '近2个月');
  assert.equal(agent.activeDaysTo51JobLabel(180), '近6个月');
  assert.equal(agent.activeDaysTo51JobLabel(365), '近1年');
  assert.equal(agent.activeDaysTo51JobLabel(366), '1年及以上');

  assert.equal(agent.normalize51JobOption('最近活跃时间', '近6个月'), '近6个月');
  assert.equal(agent.normalize51JobOption('最近活跃时间', '1年及以上'), '1年及以上');
  assert.equal(agent.normalize51JobOption('最近活跃时间', '不限'), '不限');
  assert.equal(agent.normalize51JobOption('最近活跃时间', 180), '近6个月');

  const tasks = agent.build51JobQuotaTasks({
    title: '体外诊断研发工程师',
    extra_keywords: 'PCR 微流控',
    skills: ['引物设计', '冻干工艺']
  }, '', 15);
  assert.equal(tasks.length, 3);
  assert.equal(tasks[0].keyword, '体外诊断研发工程师 PCR 微流控');
  assert.equal(tasks[1].keyword, '体外诊断研发工程师 PCR 微流控 引物设计');
  assert.equal(tasks[2].keyword, '体外诊断研发工程师 PCR 微流控 冻干工艺');

  const makeCard = (label) => ({
    offsetParent: {}, getClientRects: () => [1],
    innerText: `张先生\n上海 本科 5年经验\n${label}\n负责PCR试剂开发`,
    querySelector: selector => selector.includes('.firstline .name')
      ? { innerText: '张先生' }
      : selector.includes('.userinfo') ? { innerText: '上海 本科 5年经验' } : null,
    querySelectorAll: () => [], getAttribute: () => null
  });
  const page = { evaluate: async (fn, ...args) => fn(...args), frames: () => [] };
  try {
    global.document = { querySelectorAll: () => [makeCard('近2个月活跃')] };
    assert.equal((await agent.extractCandidatesAcrossFrames(page, 10, '', '前程无忧', [], [], { recentActiveTime: '近6个月' })).length, 1);
    assert.equal((await agent.extractCandidatesAcrossFrames(page, 10, '', '前程无忧', [], [], { recentActiveTime: '近1个月' })).length, 0);
    assert.equal((await agent.extractCandidatesAcrossFrames(page, 10, '', '前程无忧', [], [], { recentActiveTime: '1年及以上' })).length, 0);

    global.document = { querySelectorAll: () => [makeCard('1年及以上活跃')] };
    assert.equal((await agent.extractCandidatesAcrossFrames(page, 10, '', '前程无忧', [], [], { recentActiveTime: '1年及以上' })).length, 1);
    assert.equal((await agent.extractCandidatesAcrossFrames(page, 10, '', '前程无忧', [], [], { recentActiveTime: '近1年' })).length, 0);
  } finally {
    delete global.document;
  }
});
