const test = require('node:test');
const assert = require('node:assert/strict');
const frontendCatalog = require('../frontend/src/data/51job_majors.json');
const scriptCatalog = require('./51job_majors.json');
const { validate } = require('./51job_job_publisher');

test('51job 校招专业目录完整且前后端一致', () => {
  assert.deepEqual(frontendCatalog, scriptCatalog);
  assert.equal(frontendCatalog.maxSelections, 5);
  assert.ok(frontendCatalog.majors.length >= 10);
  const branches = frontendCatalog.majors.flatMap(root => root.children || []);
  const leaves = branches.flatMap(branch => branch.children || []);
  assert.ok(branches.length >= 20);
  assert.ok(leaves.length >= 100);
  assert.equal(new Set(leaves.map(item => item.value)).size, leaves.length);
});

test('校招请求只接受本地目录中的专业，限制五项并保留实习性质', () => {
  const root = frontendCatalog.majors[0];
  const branch = root.children[0];
  const leaf = branch.children[0];
  const input = {
    jobType: '校园招聘', employmentType: '实习',
    majors: [{ value: leaf.value, path: [root.name, branch.name, leaf.name] }],
    title: '软件测试实习生', description: '岗位职责与任职要求。'.repeat(8),
    functionPath: '互联网技术 > 测试 > 软件测试', keywords: ['软件测试'],
    minSalary: 6000, maxSalary: 9000, salaryMonths: 12, headcount: 2,
    experienceYears: 0, education: '本科'
  };
  assert.equal(validate(input).majors[0].value, leaf.value);
  assert.equal(validate(input).employmentType, '实习');
  assert.throws(() => validate({ ...input, majors: [{ value: 'fake', path: [root.name, branch.name, leaf.name] }] }), /不在本地/);
  assert.throws(() => validate({ ...input, majors: Array(6).fill(input.majors[0]) }), /最多选择 5 项/);
  assert.throws(() => validate({ ...input, jobType: '社会招聘', employmentType: '全职' }), /专业要求仅适用于校园招聘/);
});
