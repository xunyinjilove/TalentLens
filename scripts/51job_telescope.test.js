const test = require('node:test');
const assert = require('node:assert/strict');
const { normalizeTelescopeCandidate, telescopeActionSnapshot, validateTelescopeDetailURL } = require('./51job_telescope');

test('telescope uses stable userid when resume_key is an empty object', () => {
  const raw = {
    userid: 'user-1', resume_key: {}, base_info: { resume_name: '候选人' },
    active_type: '3日内活跃', resume_interactive: { is_hichat: '0' }
  };
  const candidate = normalizeTelescopeCandidate(raw, 'job-1');
  assert.equal(candidate.candidateId, 'user-1');
  assert.equal(candidate.recordId, 'user-1');
  assert.equal(candidate.jobId, 'job-1');
  assert.equal(candidate.alreadyHi, false);
  assert.equal(normalizeTelescopeCandidate({ ...raw, userid: '' }, 'job-1'), null);
});

test('telescope detects a previously sent Hi', () => {
  const candidate = normalizeTelescopeCandidate({ userid: 'user-1', is_hi_chat: '1' }, 'job-1');
  assert.equal(candidate.alreadyHi, true);
});

test('platform not-match is distinct from removal from the full telescope list', () => {
  const previousDocument = global.document;
  const previousLocation = global.location;
  const recommend = { $options: { name: 'TalentRecommend' }, $data: {
    activeJobInfo: { jobid: 'job-1', coid: 'company-1' }, hruId: 'hr-1'
  } };
  const list = { $options: { name: 'EhFossicksList' }, $props: { list: [{
    matchList: [{ userid: 'user-1' }],
    notMatchList: [{ userid: 'user-2', isDislikedInFossicks: '1' }]
  }] } };
  try {
    global.location = { pathname: '/Revision/talent/search-recommend' };
    global.document = { querySelectorAll: () => [{ __vue__: recommend }, { __vue__: list }] };
    assert.deepEqual(telescopeActionSnapshot('user-1'), {
      jobId: 'job-1', accountScope: 'co:company-1:hr:hr-1', inMatch: true, inNotMatch: false, disliked: false
    });
    assert.deepEqual(telescopeActionSnapshot('user-2'), {
      jobId: 'job-1', accountScope: 'co:company-1:hr:hr-1', inMatch: false, inNotMatch: true, disliked: true
    });
  } finally {
    global.document = previousDocument;
    global.location = previousLocation;
  }
});

test('telescope opens only the platform detail for the selected job and candidate', () => {
  const base = 'https://ehire.51job.com/Revision/talent/search-recommend';
  const path = '/Revision/talent/resume/detail?jobId=job-1&resumeId=user-1';
  assert.equal(validateTelescopeDetailURL(path, base, 'job-1', 'user-1'),
    `https://ehire.51job.com${path}`);
  for (const invalid of [
    '/Revision/talent/resume/detail?jobId=job-2&resumeId=user-1',
    '/Revision/talent/resume/detail?jobId=job-1&resumeId=user-2',
    'https://example.com/Revision/talent/resume/detail?jobId=job-1&resumeId=user-1',
    '/Revision/talent/search-recommend?jobId=job-1&resumeId=user-1'
  ]) assert.throws(() => validateTelescopeDetailURL(invalid, base, 'job-1', 'user-1'));
});
