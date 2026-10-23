const test = require('node:test');
const assert = require('node:assert/strict');
const {
  educationRank,
  parseSalaryRangeK,
  parseWorkYears,
  isGradAfterMonth,
  parse51JobManagementCardText,
  evaluateTalentManagementCard,
  pick51JobUnfitReasonOption,
  processPageCardsInPlace,
  summarizeFilterItems
} = require('./51job_talent_management_filter');

test('parse51JobManagementCardText accurately extracts fields from 51job Talent Management cards', () => {
  const rawZhou = `
    已看 2026.09.01 投递 · 求职意向：电气工程师助理 5-8千/月 上海
    周瑞闯
    拨打电话 1个月内活跃
    21岁 | 1年 | 大专 | 上海-松江区
    2025.03-2025.06 (3个月) 某公司 · 实习数据标注员
    2023.09-2026.06 上海科创职业技术学院 · 智能机电技术 · 大专
    技能&经验： excel 数据清洗 审核报告 数据汇总
    回复 合适 不合适
  `;

  const parsedZhou = parse51JobManagementCardText(
    { rawText: rawZhou, gender: '男', resumeId: '900101' },
    { refYearMonth: '2026.03' }
  );

  assert.equal(parsedZhou.candidateName, '周瑞闯');
  assert.equal(parsedZhou.gender, '男');
  assert.equal(parsedZhou.age, 21);
  assert.equal(parsedZhou.workYears, 1);
  assert.equal(parsedZhou.education, '大专');
  assert.equal(parsedZhou.eduRank, 2);
  assert.equal(parsedZhou.residence, '上海-松江区');
  assert.equal(parsedZhou.intentRole, '电气工程师助理');
  assert.equal(parsedZhou.intentCity, '上海');
  assert.equal(parsedZhou.salaryMinK, 5);
  assert.equal(parsedZhou.salaryMaxK, 8);
  assert.equal(parsedZhou.school, '上海科创职业技术学院');
  assert.equal(parsedZhou.major, '智能机电技术');
  assert.equal(parsedZhou.isStudent, true); // 2026.06 > 2026.03

  const rawShen = `
    2026.09.08 投递 · 求职意向：大模型应用工程师 1-1.5万/月 北京
    申佳怡 精选 刚刚活跃
    21岁 | 1年 | 本科 | 北京
    2025.12-2026.07 (7个月) 北京恒兴仁业科技 · 大模型应用工程师
    2022.09-2026.06 张家口学院 · 数字媒体技术（电子与信息大类） · 本科
    技能&经验： 精排 agent 知识库构建 数据增强
    回复 合适 不合适
  `;

  const parsedShen = parse51JobManagementCardText(
    { rawText: rawShen, gender: '女', resumeId: '900102' },
    { refYearMonth: '2026.09' }
  );

  assert.equal(parsedShen.candidateName, '申佳怡');
  assert.equal(parsedShen.gender, '女');
  assert.equal(parsedShen.education, '本科');
  assert.equal(parsedShen.eduRank, 3);
  assert.equal(parsedShen.residence, '北京');
  assert.equal(parsedShen.intentCity, '北京');
  assert.equal(parsedShen.salaryMinK, 10);
  assert.equal(parsedShen.salaryMaxK, 15);
  assert.equal(parsedShen.major, '数字媒体技术（电子与信息大类）');
  assert.equal(parsedShen.isStudent, false); // 2026.06 <= 2026.09
});

test('evaluateTalentManagementCard enforces Tri-State logic (PASS / FAIL / UNKNOWN) and never misfilters unknown fields', () => {
  const filter = {
    minEducation: '本科',
    minWorkYears: 1,
    excludeStudents: true,
    allowedResidences: '上海, 苏州',
    allowedIntentCities: '上海',
    allowedMajors: '计算机, 数字媒体, 软件',
    maxSalaryK: 12
  };

  // Case 1: 明确大专学历 + 专业不符 -> UNFIT
  const zhouCard = {
    candidateKey: '51job_1',
    candidateName: '周瑞闯',
    eduRank: educationRank('大专'),
    education: '大专',
    workYears: 1,
    isStudent: true,
    eduGradRange: '2023.09-2026.06',
    residence: '上海-松江区',
    intentCity: '上海',
    major: '智能机电技术',
    salaryMinK: 5,
    salaryMaxK: 8
  };
  const res1 = evaluateTalentManagementCard(zhouCard, filter);
  assert.equal(res1.decision, 'UNFIT');
  assert.equal(res1.primaryReasonCode, 'education');
  assert.ok(res1.failedRules.some(r => r.code === 'education'));
  assert.ok(res1.failedRules.some(r => r.code === 'exclude_students'));
  assert.ok(res1.failedRules.some(r => r.code === 'major'));

  // Case 2: 候选人期望 10-15K，HR 设定上限 12K -> 最低期望 10K <= 12K，双方有交集，不应按薪资淘汰！
  const salaryOverlapCard = {
    candidateKey: '51job_2',
    candidateName: '李工',
    eduRank: educationRank('本科'),
    education: '本科',
    workYears: 2,
    isStudent: false,
    residence: '上海-浦东新区',
    intentCity: '上海',
    major: '软件工程',
    salaryMinK: 10,
    salaryMaxK: 15,
    intentSalaryRaw: '1-1.5万/月'
  };
  const res2 = evaluateTalentManagementCard(salaryOverlapCard, filter);
  assert.equal(res2.decision, 'KEEP_MATCHED');

  // Case 3: 候选人专业与期望薪资均未披露 (null) -> KEEP_UNKNOWN（未知不误筛）
  const unknownFieldsCard = {
    candidateKey: '51job_3',
    candidateName: '王工',
    eduRank: educationRank('硕士'),
    education: '硕士',
    workYears: 3,
    isStudent: false,
    residence: '上海',
    intentCity: '上海',
    major: null,
    salaryMinK: null,
    salaryMaxK: null
  };
  const res3 = evaluateTalentManagementCard(unknownFieldsCard, filter);
  assert.equal(res3.decision, 'KEEP_UNKNOWN');
  assert.equal(res3.failedRules.length, 0);
  assert.equal(res3.unknownRules.length, 2);

  // Case 4: 白名单候选人即使学历是中专也豁免跳过
  const whitelistedCard = {
    ...zhouCard,
    resumeId: 'protected_999'
  };
  const res4 = evaluateTalentManagementCard(whitelistedCard, filter, new Set(['protected_999']));
  assert.equal(res4.decision, 'WHITELIST_SKIP');
});

test('age and gender filters require explicit compliance confirmation and map to 其他原因', () => {
  const card = {
    candidateKey: '51job_age',
    candidateName: '张先生',
    age: 38,
    gender: '男',
    eduRank: 3,
    education: '本科'
  };

  // 未勾选合规确认时，年龄/性别条件不生效
  const resUnconfirmed = evaluateTalentManagementCard(card, {
    enableAgeFilter: true,
    maxAge: 35,
    complianceConfirmed: false
  });
  assert.equal(resUnconfirmed.decision, 'KEEP_MATCHED');

  // 勾选合规确认后生效
  const resConfirmed = evaluateTalentManagementCard(card, {
    enableAgeFilter: true,
    maxAge: 35,
    complianceConfirmed: true
  });
  assert.equal(resConfirmed.decision, 'UNFIT');
  assert.equal(resConfirmed.primaryReasonCode, 'age');

  const btn = pick51JobUnfitReasonOption(resConfirmed.failedRules, [
    '最近工作职能(数据标注师)不匹配',
    '学历(大专)不匹配',
    '工作年限(在校生)不匹配',
    '过往经历不匹配',
    '其他原因'
  ]);
  assert.equal(btn, '其他原因');
});

test('pick51JobUnfitReasonOption selects dynamic 51job popover buttons by priority and falls back safely', () => {
  const popoverButtons = [
    '最近工作职能(数据标注师)不匹配',
    '学历(大专)不匹配',
    '工作年限(在校生)不匹配',
    '过往经历不匹配',
    '其他原因'
  ];

  // 同时命中大专学历与异地时，优先点弹窗中存在的 "学历(大专)不匹配"
  assert.equal(
    pick51JobUnfitReasonOption([{ code: 'residence' }, { code: 'education' }], popoverButtons),
    '学历(大专)不匹配'
  );

  // 命中在校生排除时，匹配 "工作年限(在校生)不匹配"
  assert.equal(
    pick51JobUnfitReasonOption([{ code: 'exclude_students' }], popoverButtons),
    '工作年限(在校生)不匹配'
  );

  // 命中居住地不符但弹窗无居住地按钮时，回退到 "其他原因"
  assert.equal(
    pick51JobUnfitReasonOption([{ code: 'residence' }], popoverButtons),
    '其他原因'
  );
});

test('processPageCardsInPlace handles in-place DOM removal without skipping adjacent candidates', async () => {
  // 模拟页面上有 3 张卡片：第 1、2 张不合适（点完会从 DOM 数组 splice 移除），第 3 张合适保留
  const liveDomCards = [
    {
      resumeId: '101',
      candidateName: '候选人A',
      rawText: '2026.09.01 投递 · 求职意向：开发 6-8千/月 上海\n候选人A\n21岁 | 1年 | 大专 | 上海'
    },
    {
      resumeId: '102',
      candidateName: '候选人B',
      rawText: '2026.09.02 投递 · 求职意向：开发 6-8千/月 北京\n候选人B\n22岁 | 1年 | 大专 | 北京'
    },
    {
      resumeId: '103',
      candidateName: '候选人C',
      rawText: '2026.09.03 投递 · 求职意向：开发 1-1.5万/月 上海\n候选人C\n25岁 | 3年 | 本科 | 上海'
    }
  ];

  const clickedNames = [];
  const adapter = {
    async scanVisibleCards() {
      return [...liveDomCards];
    },
    async markCardUnfit(targetCard, failedRules) {
      clickedNames.push(targetCard.candidateName);
      const idx = liveDomCards.findIndex(c => c.resumeId === targetCard.resumeId);
      if (idx !== -1) liveDomCards.splice(idx, 1);
      return {
        status: 'verified',
        clickedReasonButton: pick51JobUnfitReasonOption(failedRules, ['学历(大专)不匹配', '其他原因'])
      };
    },
    async waitBetweenActions() {}
  };

  const { results, haltedReason } = await processPageCardsInPlace(
    adapter,
    { minEducation: '本科' },
    new Set(),
    { mode: 'execute', minDelayMs: 1, maxDelayMs: 2 }
  );

  assert.equal(haltedReason, '');
  assert.deepEqual(clickedNames, ['候选人A', '候选人B']);
  assert.equal(results.length, 3);
  const summary = summarizeFilterItems(results);
  assert.equal(summary.unfitCount, 2);
  assert.equal(summary.keepMatchedCount, 1);
  assert.equal(liveDomCards.length, 1);
  assert.equal(liveDomCards[0].candidateName, '候选人C');
});
