const test = require('node:test');
const assert = require('node:assert/strict');
const { normalizeTelescopeCandidate, validateTelescopeDetailURL } = require('./51job_telescope');

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
