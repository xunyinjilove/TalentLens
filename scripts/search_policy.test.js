const test = require('node:test');
const assert = require('node:assert/strict');
const agent = require('./multi_platform_agent');

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
