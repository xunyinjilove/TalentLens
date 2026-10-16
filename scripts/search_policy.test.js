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
