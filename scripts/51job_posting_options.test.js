const test = require('node:test');
const assert = require('node:assert/strict');
const snapshot = require('../frontend/src/data/51job_posting_options.json');

test('本地 51job 目录覆盖每个语言熟练程度和证书分类', () => {
  assert.equal(snapshot.source, '51job 企业职位表单');
  assert.ok(snapshot.languages.length > 0);
  assert.ok(snapshot.certificateCategories.length > 0);
  assert.equal(new Set(snapshot.languages).size, snapshot.languages.length);
  assert.equal(new Set(snapshot.certificateCategories).size, snapshot.certificateCategories.length);
  for (const language of snapshot.languages) {
    assert.ok(snapshot.languageLevels[language]?.length > 0, `${language} 缺少熟练程度`);
  }
  for (const category of snapshot.certificateCategories) {
    const options = snapshot.certificates[category];
    assert.ok(options?.length > 0, `${category} 缺少证书`);
    assert.equal(new Set(options).size, options.length, `${category} 包含重复证书`);
  }
});
