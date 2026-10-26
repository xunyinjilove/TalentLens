const test = require('node:test');
const assert = require('node:assert/strict');
const { normalizeCatalog, normalizeCandidate, rejectCandidate } = require('./51job_talent_management_import');

test('catalog requires a scoped operator and stable platform job IDs', () => {
  assert.throws(() => normalizeCatalog({ positions: [{ jobid: '7', jobname: '职位' }] }), /操作员身份/);
  assert.deepEqual(normalizeCatalog({ hruid: '42', coid: 'company1', positions: [
    { jobid: '7', jobname: '职位', workArea: '上海', degree: '本科', workYear: '2年及以上', salary: '18-36万/年' },
    { jobid: '7', jobname: '职位' },
    { jobname: '全部职位' }
  ] }), { accountScope: 'co:company1:hr:42', jobs: [{ jobId: '7', title: '职位', workArea: '上海', degree: '本科', workYear: '2年及以上', salary: '18-36万/年' }] });
});

test('candidate identity is rejected when any platform ID or selected job differs', () => {
  const raw = { userid: 'u1', cvlogid: 'c1', jobid: 'j1', coid: 'co1', resume_has_enclosure: '1' };
  assert.equal(normalizeCandidate(raw, 'j2'), null);
  assert.equal(normalizeCandidate({ ...raw, cvlogid: '' }, 'j1'), null);
  const item = normalizeCandidate(raw, 'j1');
  assert.equal(item.candidateId, 'u1');
  assert.equal(item.recordId, 'c1');
  assert.equal(item.hasAttachment, true);
});

test('candidate name correctly parses resume_name from base_info', () => {
  const raw = { userid: 'u1', cvlogid: 'c1', jobid: 'j1', coid: 'co1', base_info: { resume_name: '张三' } };
  const item = normalizeCandidate(raw, 'j1');
  assert.equal(item.name, '张三');
});

test('rejectCandidate rejects incomplete identification without touching browser', async () => {
  await assert.rejects(() => rejectCandidate(null, 'scope', {}), /平台标识不完整/);
  await assert.rejects(() => rejectCandidate(null, 'scope', { jobId: 'j1', candidateId: 'c1' }), /平台标识不完整/);
});
