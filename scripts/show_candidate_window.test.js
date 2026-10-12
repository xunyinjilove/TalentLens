const test = require('node:test');
const assert = require('node:assert/strict');
const { restoreCandidateWindow } = require('./show_candidate_window');

test('manual candidate opening restores the exact tab window and brings it forward', async () => {
  const actions = [];
  const candidatePage = { bringToFront: async () => actions.push('foreground') };
  const otherPage = { bringToFront: async () => { throw Error('wrong tab'); } };
  const browser = { targets: () => [
    { _targetId: 'other', page: async () => otherPage },
    { _targetId: 'candidate', page: async () => candidatePage }
  ] };
  await restoreCandidateWindow(browser, 'candidate', async (page, state) => {
    assert.equal(page, candidatePage);
    actions.push(state);
    return true;
  });
  assert.deepEqual(actions, ['normal', 'foreground']);
});
