/**
 * 51job 企业端「人才管理」(https://ehire.51job.com/Revision/talent/management)
 * 自动筛选与「不合适」原因勾选适配器
 *
 * 核心安全守则：
 * 1. 只自动点「不合适」，绝不自动点「合适」。
 * 2. 三值逻辑（PASS / FAIL / UNKNOWN）：资料未披露或无法解析的字段一律记为 UNKNOWN，绝不误筛。
 * 3. 白名单豁免：已在 TalentLens 标记合适/面试/Offer 的候选人自动跳过（WHITELIST_SKIP）。
 * 4. 点后核验闭环：点击「不合适 -> 选择原因」后验证卡片从当前列表移除或顶部计数变化，异常立即熔断暂停。
 */

const fs = require('fs');
const crypto = require('crypto');
const puppeteer = require('puppeteer-core');

const EDU_RANK_MAP = {
  初中: 1,
  中技: 1,
  高中: 1,
  中专: 1,
  大专: 2,
  专科: 2,
  本科: 3,
  学士: 3,
  硕士: 4,
  研究生: 4,
  MBA: 4,
  EMBA: 4,
  博士: 5,
  博士后: 5
};

function educationRank(raw) {
  const text = String(raw || '').trim();
  if (!text || text === '不限' || text === '学历不限') return 0;
  let best = 0;
  for (const [label, rank] of Object.entries(EDU_RANK_MAP)) {
    if (text.toUpperCase().includes(label)) {
      if (rank > best) best = rank;
    }
  }
  return best;
}

function splitFilterTokens(raw) {
  if (Array.isArray(raw)) {
    return raw.map(item => String(item || '').trim()).filter(Boolean);
  }
  return String(raw || '')
    .split(/[,，、;；\s/]+/)
    .map(item => item.trim())
    .filter(Boolean);
}

/**
 * 将 51job 期望薪资字符串统一换算为以 K/月 为单位的区间 { minK, maxK }
 * 例如：
 * - "5-8千/月" -> { minK: 5, maxK: 8 }
 * - "1-1.5万/月" -> { minK: 10, maxK: 15 }
 * - "8000-12000元/月" -> { minK: 8, maxK: 12 }
 * - "12-18K/月" -> { minK: 12, maxK: 18 }
 * - "面议" / "" -> null (UNKNOWN)
 */
function parseSalaryRangeK(raw) {
  const text = String(raw || '').replace(/\s+/g, '');
  if (!text || /面议|保密/.test(text)) return null;

  let m = text.match(/(\d+(?:\.\d+)?)\s*[-~至]\s*(\d+(?:\.\d+)?)\s*(千|万|k|K|元)(?:\/?(月|年))?/);
  if (m) {
    let minVal = parseFloat(m[1]);
    let maxVal = parseFloat(m[2]);
    const unit = m[3].toLowerCase();
    const period = m[4] || '月';
    let factor = 1;
    if (unit === '万') factor = 10;
    else if (unit === '千' || unit === 'k') factor = 1;
    else if (unit === '元') factor = 0.001;
    minVal *= factor;
    maxVal *= factor;
    if (period === '年') {
      minVal = Number((minVal / 12).toFixed(2));
      maxVal = Number((maxVal / 12).toFixed(2));
    }
    return { minK: minVal, maxK: maxVal, raw: text };
  }

  m = text.match(/(\d+(?:\.\d+)?)\s*(千|万|k|K|元)\/?(月|年)?(?:以下|以上)?/);
  if (m) {
    let val = parseFloat(m[1]);
    const unit = m[2].toLowerCase();
    const period = m[3] || '月';
    if (unit === '万') val *= 10;
    else if (unit === '元') val *= 0.001;
    if (period === '年') val = Number((val / 12).toFixed(2));
    return { minK: val, maxK: val, raw: text };
  }

  return null;
}

/**
 * 解析工作年限文本
 * - "1年" / "3年工作经验" -> { years: 1, isStudentOrFresh: false }
 * - "在校生" / "应届生" / "无工作经验" -> { years: 0, isStudentOrFresh: true }
 */
function parseWorkYears(raw) {
  const text = String(raw || '').trim();
  if (!text) return { years: null, isStudentOrFresh: null };
  if (/在校生|在读|应届|无工作经验|无经验/.test(text)) {
    return {
      years: 0,
      isStudentOrFresh: true,
      explicitlyStudent: /在校生|在读/.test(text)
    };
  }
  const m = text.match(/(\d+(?:\.\d+)?)\s*年/);
  if (m) {
    return {
      years: parseFloat(m[1]),
      isStudentOrFresh: false,
      explicitlyStudent: false
    };
  }
  if (/(\d+)\s*个月/.test(text)) {
    return {
      years: 0.5,
      isStudentOrFresh: false,
      explicitlyStudent: false
    };
  }
  return { years: null, isStudentOrFresh: null, explicitlyStudent: false };
}

/**
 * 根据教育经历行判断是否为未毕业/在校生
 * 例如 "2023.09-2026.06 张家口学院 · 数字媒体技术 · 本科"
 * 若参考年月为 "2026.03"，则毕业年月 2026.06 > 2026.03 -> 返回 true (在读)
 */
function isGradAfterMonth(eduText, refYearMonth) {
  const text = String(eduText || '').trim();
  if (!text) return null;
  const nowYM = normalizeYearMonth(refYearMonth) || currentYearMonth();
  if (/\d{4}[./-]\d{1,2}\s*[-~至]\s*至今/.test(text)) {
    return true;
  }
  const rangeMatch = text.match(/(\d{4})[./-](\d{1,2})\s*[-~至]\s*(\d{4})[./-](\d{1,2})/);
  if (!rangeMatch) return null;
  const endYM = `${rangeMatch[3]}.${String(parseInt(rangeMatch[4], 10)).padStart(2, '0')}`;
  return endYM > nowYM;
}

function normalizeYearMonth(ym) {
  const m = String(ym || '').trim().match(/^(\d{4})[./-](\d{1,2})$/);
  if (!m) return '';
  return `${m[1]}.${String(parseInt(m[2], 10)).padStart(2, '0')}`;
}

function currentYearMonth() {
  const now = new Date();
  return `${now.getFullYear()}.${String(now.getMonth() + 1).padStart(2, '0')}`;
}

/**
 * 解析 51job「人才管理」单张卡片文本与属性
 */
function parse51JobManagementCardText(input = {}, options = {}) {
  const rawText = String(input.rawText || input.text || '').trim();
  const lines = rawText
    .split(/\r?\n/)
    .map(l => l.trim())
    .filter(Boolean);

  // 1. 顶部投递与求职意向栏：如 "已看 2026.09.01 投递 · 求职意向：电气工程师助理 5-8千/月 上海"
  let appliedDate = input.appliedDate || '';
  let intentRole = input.intentRole || '';
  let intentSalaryRaw = input.intentSalaryRaw || '';
  let intentCity = input.intentCity || '';

  const headerLine = lines.find(l => /求职意向[：:]/.test(l)) || rawText;
  const dateMatch = headerLine.match(/(\d{4}[./-]\d{2}[./-]\d{2})\s*(?:投递|收藏|推荐|浏览)?/);
  if (!appliedDate && dateMatch) appliedDate = dateMatch[1];

  const intentMatch = headerLine.match(/求职意向[：:]\s*([^\r\n]+)/);
  if (intentMatch) {
    const intentBody = intentMatch[1].trim();
    const salaryTokenMatch = intentBody.match(/(\d+(?:\.\d+)?(?:\s*[-~至]\s*\d+(?:\.\d+)?)?\s*(?:千|万|k|K|元)\/?(?:月|年)?|面议)/);
    if (salaryTokenMatch) {
      if (!intentSalaryRaw) intentSalaryRaw = salaryTokenMatch[1].trim();
      const idx = intentBody.indexOf(salaryTokenMatch[1]);
      if (!intentRole) intentRole = intentBody.slice(0, idx).trim();
      const afterSalary = intentBody.slice(idx + salaryTokenMatch[1].length).trim();
      if (!intentCity && afterSalary) {
        intentCity = afterSalary.split(/\s+/)[0].trim();
      }
    } else {
      const parts = intentBody.split(/\s+/).filter(Boolean);
      if (!intentRole && parts.length >= 1) intentRole = parts[0];
      if (!intentCity && parts.length >= 2) intentCity = parts[parts.length - 1];
    }
  }

  const salaryParsed = parseSalaryRangeK(intentSalaryRaw);

  // 2. 基本画像行：如 "21岁 | 1年 | 大专 | 上海-松江区" 或 "23岁 | 在校生 | 本科 | 北京"
  let age = typeof input.age === 'number' ? input.age : null;
  let workYears = typeof input.workYears === 'number' ? input.workYears : null;
  let isStudent = typeof input.isStudent === 'boolean' ? input.isStudent : null;
  let education = input.education || '';
  let residence = input.residence || '';

  const profileLine = lines.find(l => l.includes('|') && (/\d{2}\s*岁/.test(l) || /本科|大专|硕士|博士|中专|高中/.test(l))) || '';
  if (profileLine) {
    const segments = profileLine.split('|').map(s => s.trim()).filter(Boolean);
    for (const seg of segments) {
      const ageM = seg.match(/^(\d{2})\s*岁$/);
      if (ageM && age === null) {
        age = parseInt(ageM[1], 10);
        continue;
      }
      if (/^(?:在校生|应届生|无经验|无工作经验|\d+(?:\.\d+)?\s*年(?:工作经验)?|\d+\s*个月)$/.test(seg) && workYears === null) {
        const wy = parseWorkYears(seg);
        workYears = wy.years;
        if (wy.explicitlyStudent) isStudent = true;
        else if (wy.years !== null && wy.years > 0 && isStudent === null) isStudent = false;
        continue;
      }
      if (educationRank(seg) > 0 && !education) {
        education = seg;
        continue;
      }
      if (!residence && !/\d{2}\s*岁/.test(seg) && educationRank(seg) === 0 && !/年|在校|应届/.test(seg)) {
        residence = seg;
      }
    }
  } else {
    const ageFallback = rawText.match(/(\d{2})\s*岁/);
    if (age === null && ageFallback) age = parseInt(ageFallback[1], 10);
  }

  // 3. 教育经历行：如 "2023.09-2026.06 张家口学院 · 数字媒体技术（电子与信息大类） · 本科"
  let school = input.school || '';
  let major = input.major || '';
  let eduGradRange = input.eduGradRange || '';

  const eduLine = lines.find(l => {
    if (!l.includes('·')) return false;
    return /学院|大学|学校|专科|本科|硕士|博士/.test(l) && /\d{4}[./-]\d{1,2}/.test(l);
  }) || lines.find(l => l.includes('·') && /大学|学院|本科|大专|硕士|博士/.test(l)) || '';

  if (eduLine) {
    const rangeM = eduLine.match(/(\d{4}[./-]\d{1,2}\s*[-~至]\s*(?:\d{4}[./-]\d{1,2}|至今))/);
    let rest = eduLine;
    if (rangeM) {
      eduGradRange = rangeM[1].replace(/\s+/g, '');
      rest = eduLine.slice(eduLine.indexOf(rangeM[1]) + rangeM[1].length).trim();
    }
    const dotParts = rest.split('·').map(s => s.trim()).filter(Boolean);
    if (dotParts.length >= 3) {
      if (!school) school = dotParts[0];
      if (!major) major = dotParts[1];
      const eduFromLine = dotParts[dotParts.length - 1];
      if (educationRank(eduFromLine) > educationRank(education)) {
        education = eduFromLine;
      }
    } else if (dotParts.length === 2) {
      if (!school) school = dotParts[0];
      if (educationRank(dotParts[1]) > 0 && !major) {
        if (educationRank(dotParts[1]) > educationRank(education)) education = dotParts[1];
      } else if (!major) {
        major = dotParts[1];
      }
    }
    const gradStatus = isGradAfterMonth(eduGradRange || eduLine, options.refYearMonth);
    if (gradStatus === true) {
      isStudent = true;
    } else if (gradStatus === false && isStudent === null) {
      isStudent = false;
    }
  }

  // 4. 工作经历行：如 "2025.12-2026.07 (7个月) 北京恒兴仁业科技 · 大模型应用工程师"
  let recentCompany = input.recentCompany || '';
  let recentRole = input.recentRole || '';
  const workLine = lines.find(l => l !== eduLine && l.includes('·') && /\d{4}[./-]\d{1,2}/.test(l)) || '';
  if (workLine) {
    const cleanedWork = workLine.replace(/^\d{4}[./-]\d{1,2}\s*[-~至]\s*(?:\d{4}[./-]\d{1,2}|至今)\s*(?:\([^)]*\))?\s*/, '').trim();
    const workParts = cleanedWork.split('·').map(s => s.trim()).filter(Boolean);
    if (workParts.length >= 2) {
      if (!recentCompany) recentCompany = workParts[0];
      if (!recentRole) recentRole = workParts[1];
    }
  }

  // 5. 姓名与性别
  let candidateName = String(input.candidateName || '').trim();
  if (!candidateName) {
    const ignored = new Set(['已看', '精选', '刚刚活跃', '今日活跃', '拨打电话', '回复', '合适', '不合适']);
    for (const line of lines) {
      if (/求职意向|技能&经验|\||·|\d{4}[./-]/.test(line)) continue;
      const cleaned = line.replace(/\s*(?:精选|已看|刚刚活跃|\d+个月内活跃|\d+天内活跃|拨打电话).*$/, '').trim();
      if (/^[\u4e00-\u9fa5·]{2,6}(?:先生|女士|小姐)?$/.test(cleaned) && !ignored.has(cleaned)) {
        candidateName = cleaned;
        break;
      }
    }
  }

  let gender = normalizeGender(input.gender);
  if (!gender) {
    if (/先生|♂/.test(rawText) || /先生$/.test(candidateName)) gender = '男';
    else if (/女士|小姐|♀/.test(rawText) || /(?:女士|小姐)$/.test(candidateName)) gender = '女';
  }

  // 6. 技能标签行
  let skills = Array.isArray(input.skills) ? input.skills : [];
  if (!skills.length) {
    const skillLine = lines.find(l => /技能\s*&\s*经验[：:]/.test(l));
    if (skillLine) {
      const body = skillLine.replace(/^.*技能\s*&\s*经验[：:]\s*/, '').trim();
      skills = body.split(/\s+/).map(s => s.trim()).filter(Boolean);
    }
  }

  const eduRank = educationRank(education);
  const resumeId = String(input.resumeId || input.platformCandidateId || '').trim();
  const profileUrl = String(input.profileUrl || (resumeId ? `https://ehire.51job.com/Revision/talent/resume/detail?resumeId=${resumeId}` : '')).trim();

  const fingerprintSource = [
    candidateName,
    resumeId,
    age !== null ? `${age}岁` : '',
    education,
    residence,
    intentRole,
    school,
    major
  ].join('|');
  const candidateKey = resumeId
    ? `51job_${resumeId}`
    : `51job_fp_${crypto.createHash('sha1').update(fingerprintSource).digest('hex').slice(0, 12)}`;

  return {
    candidateKey,
    resumeId,
    profileUrl,
    candidateName,
    gender: gender || null,
    age,
    workYears,
    isStudent,
    education: education || null,
    eduRank: eduRank > 0 ? eduRank : null,
    residence: residence || null,
    intentRole: intentRole || null,
    intentCity: intentCity || null,
    intentSalaryRaw: intentSalaryRaw || null,
    salaryMinK: salaryParsed ? salaryParsed.minK : null,
    salaryMaxK: salaryParsed ? salaryParsed.maxK : null,
    school: school || null,
    major: major || null,
    eduGradRange: eduGradRange || null,
    recentCompany: recentCompany || null,
    recentRole: recentRole || null,
    skills,
    appliedDate: appliedDate || null,
    rawText
  };
}

function normalizeGender(val) {
  const s = String(val || '').trim().toLowerCase();
  if (!s || s === '不限' || s === 'unknown') return null;
  if (s === '男' || s === 'male' || s === 'm' || s === '♂') return '男';
  if (s === '女' || s === 'female' || s === 'f' || s === '♀') return '女';
  return null;
}

/**
 * 9 项筛选维度的三值逻辑（PASS / FAIL / UNKNOWN）评估器
 */
function evaluateTalentManagementCard(card, filterConfig = {}, whitelistKeys = new Set()) {
  const wl = whitelistKeys instanceof Set ? whitelistKeys : new Set(Array.isArray(whitelistKeys) ? whitelistKeys : []);
  const nameKey = card.candidateName ? `name:${card.candidateName}` : '';
  if (
    (card.resumeId && wl.has(card.resumeId)) ||
    (card.candidateKey && wl.has(card.candidateKey)) ||
    (nameKey && wl.has(nameKey)) ||
    (card.candidateName && wl.has(card.candidateName))
  ) {
    return {
      decision: 'WHITELIST_SKIP',
      failedRules: [],
      unknownRules: [],
      primaryReasonCode: '',
      reasonLabel: 'TalentLens 白名单保护（已合适/面试/入库保护）'
    };
  }

  const failedRules = [];
  const unknownRules = [];

  // 1. 最低学历 (minEducation)
  const requiredEduRank = educationRank(filterConfig.minEducation);
  if (requiredEduRank > 0) {
    if (!card.eduRank) {
      unknownRules.push({ code: 'education', label: '学历未披露' });
    } else if (card.eduRank < requiredEduRank) {
      failedRules.push({
        code: 'education',
        label: `学历(${card.education || '低学历'})低于要求(${filterConfig.minEducation})`
      });
    }
  }

  // 2. 工作年限下限 (minWorkYears)
  const minWorkYears = Number(filterConfig.minWorkYears);
  if (Number.isFinite(minWorkYears) && minWorkYears > 0) {
    if (card.workYears === null || card.workYears === undefined) {
      unknownRules.push({ code: 'work_years', label: '工作年限未披露' });
    } else if (card.workYears < minWorkYears) {
      failedRules.push({
        code: 'work_years',
        label: `工作年限(${card.workYears === 0 ? '在校/无经验' : card.workYears + '年'})低于要求(${minWorkYears}年)`
      });
    }
  }

  // 3. 在校生/应届未毕业排除 (excludeStudents)
  if (filterConfig.excludeStudents) {
    if (card.isStudent === null || card.isStudent === undefined) {
      unknownRules.push({ code: 'exclude_students', label: '毕业时间/在校状态未披露' });
    } else if (card.isStudent === true) {
      failedRules.push({
        code: 'exclude_students',
        label: `在校生/未毕业(${card.eduGradRange || '在读'})`
      });
    }
  }

  // 4. 专业关键词限制 (allowedMajors)
  const allowedMajors = splitFilterTokens(filterConfig.allowedMajors);
  if (allowedMajors.length > 0) {
    if (!card.major) {
      unknownRules.push({ code: 'major', label: '所学专业未披露' });
    } else {
      const majorLower = card.major.toLowerCase();
      const matched = allowedMajors.some(kw => majorLower.includes(kw.toLowerCase()));
      if (!matched) {
        failedRules.push({
          code: 'major',
          label: `专业(${card.major})不在允许范围(${allowedMajors.join('/')})`
        });
      }
    }
  }

  // 5. 现居地限制 (allowedResidences)
  const allowedResidences = splitFilterTokens(filterConfig.allowedResidences);
  if (allowedResidences.length > 0) {
    if (!card.residence) {
      unknownRules.push({ code: 'residence', label: '现居地未披露' });
    } else {
      const matched = allowedResidences.some(city => card.residence.includes(city) || city.includes(card.residence.split('-')[0]));
      if (!matched) {
        failedRules.push({
          code: 'residence',
          label: `现居地(${card.residence})不在允许城市(${allowedResidences.join('/')})`
        });
      }
    }
  }

  // 6. 求职意向城市限制 (allowedIntentCities)
  const allowedIntentCities = splitFilterTokens(filterConfig.allowedIntentCities);
  if (allowedIntentCities.length > 0) {
    if (!card.intentCity) {
      unknownRules.push({ code: 'intent_city', label: '求职意向城市未披露' });
    } else {
      const matched = allowedIntentCities.some(city => card.intentCity.includes(city) || city.includes(card.intentCity.split('-')[0]));
      if (!matched) {
        failedRules.push({
          code: 'intent_city',
          label: `求职意向城市(${card.intentCity})不在允许城市(${allowedIntentCities.join('/')})`
        });
      }
    }
  }

  // 7. 期望月薪上限 (maxSalaryK)：仅当候选人最低期望 salaryMinK > maxSalaryK 时才淘汰
  const maxSalaryK = Number(filterConfig.maxSalaryK);
  if (Number.isFinite(maxSalaryK) && maxSalaryK > 0) {
    if (card.salaryMinK === null || card.salaryMinK === undefined) {
      unknownRules.push({ code: 'salary', label: '期望月薪未披露或面议' });
    } else if (card.salaryMinK > maxSalaryK) {
      failedRules.push({
        code: 'salary',
        label: `最低期望月薪(${card.intentSalaryRaw || card.salaryMinK + 'K'})高于上限(${maxSalaryK}K)`
      });
    }
  }

  // 8. 年龄区间 (minAge / maxAge) —— 合规受控项
  const complianceOk = Boolean(filterConfig.complianceConfirmed);
  if (filterConfig.enableAgeFilter && complianceOk) {
    const minAge = Number(filterConfig.minAge);
    const maxAge = Number(filterConfig.maxAge);
    const hasMin = Number.isFinite(minAge) && minAge > 0;
    const hasMax = Number.isFinite(maxAge) && maxAge > 0;
    if (hasMin || hasMax) {
      if (card.age === null || card.age === undefined) {
        unknownRules.push({ code: 'age', label: '年龄未披露' });
      } else if ((hasMin && card.age < minAge) || (hasMax && card.age > maxAge)) {
        const rangeDesc = `${hasMin ? minAge : '不限'}~${hasMax ? maxAge : '不限'}岁`;
        failedRules.push({
          code: 'age',
          label: `年龄(${card.age}岁)超出设定区间(${rangeDesc})`
        });
      }
    }
  }

  // 9. 性别限制 (allowedGender) —— 合规受控项
  if (filterConfig.enableGenderFilter && complianceOk) {
    const requiredGender = normalizeGender(filterConfig.allowedGender);
    if (requiredGender) {
      if (!card.gender) {
        unknownRules.push({ code: 'gender', label: '性别未披露' });
      } else if (card.gender !== requiredGender) {
        failedRules.push({
          code: 'gender',
          label: `性别(${card.gender})与设定(${requiredGender})不符`
        });
      }
    }
  }

  if (failedRules.length > 0) {
    return {
      decision: 'UNFIT',
      failedRules,
      unknownRules,
      primaryReasonCode: failedRules[0].code,
      reasonLabel: failedRules.map(r => r.label).join('；')
    };
  }

  if (unknownRules.length > 0) {
    return {
      decision: 'KEEP_UNKNOWN',
      failedRules: [],
      unknownRules,
      primaryReasonCode: '',
      reasonLabel: `资料未知保留（${unknownRules.map(r => r.label).join('、')}）`
    };
  }

  return {
    decision: 'KEEP_MATCHED',
    failedRules: [],
    unknownRules: [],
    primaryReasonCode: '',
    reasonLabel: '硬性条件全部符合，保留并导入待分析'
  };
}

const REASON_MATCHERS = [
  { codes: ['education'], regex: /^学历.*不匹配/ },
  { codes: ['work_years', 'exclude_students'], regex: /^工作年限.*不匹配/ },
  { codes: ['major'], regex: /^专业.*不匹配/ },
  { codes: ['residence', 'intent_city'], regex: /^(?:居住地|现居地|地点|城市).*不匹配/ },
  { codes: ['recent_function'], regex: /^最近工作职能.*不匹配/ },
  { codes: ['past_experience'], regex: /^过往经历.*不匹配/ }
];

/**
 * 根据命中的淘汰规则，在 51job「选择原因，为您优化推荐」浮层按钮列表中选择最佳按钮文字
 * - 若命中年龄/性别/薪资，或当前弹窗无对应专属按钮，一律兜底选择 "其他原因"
 */
function pick51JobUnfitReasonOption(failedRules = [], availableOptions = []) {
  const cleanedOptions = availableOptions
    .map(opt => String(opt || '').trim())
    .filter(Boolean);

  const failedCodes = new Set(
    (failedRules || []).map(r => (typeof r === 'string' ? r : r.code)).filter(Boolean)
  );

  for (const matcher of REASON_MATCHERS) {
    if (matcher.codes.some(c => failedCodes.has(c))) {
      const hit = cleanedOptions.find(opt => matcher.regex.test(opt));
      if (hit) return hit;
    }
  }

  const fallback = cleanedOptions.find(opt => /^其他原因/.test(opt));
  return fallback || '其他原因';
}

/**
 * Box-Muller 正态分布拟人随机等待毫秒数（默认 [2000, 5000] ms）
 */
function gaussianRandomDelayMs(minMs = 2000, maxMs = 5000) {
  const mean = (minMs + maxMs) / 2;
  const stdDev = (maxMs - minMs) / 6;
  let u = 0;
  let v = 0;
  while (u === 0) u = Math.random();
  while (v === 0) v = Math.random();
  const z = Math.sqrt(-2.0 * Math.log(u)) * Math.cos(2.0 * Math.PI * v);
  const val = Math.round(mean + z * stdDev);
  return Math.max(minMs, Math.min(maxMs, val));
}

/**
 * 单页原地消化控制器（防止点击「不合适」后卡片从 DOM 移除导致下标偏移）
 */
async function processPageCardsInPlace(adapter, filterConfig, whitelistKeys, runOptions = {}) {
  const mode = runOptions.mode === 'execute' ? 'execute' : 'dry_run';
  const maxCandidates = Number(runOptions.maxCandidates) || 200;
  const processedKeys = runOptions.processedKeys || new Set();
  const results = [];
  let haltedReason = '';

  const initialCards = await adapter.scanVisibleCards();
  if (mode === 'dry_run') {
    for (const rawCard of initialCards) {
      if (results.length >= maxCandidates) break;
      const parsed = parse51JobManagementCardText(rawCard, { refYearMonth: runOptions.refYearMonth });
      if (!parsed.candidateName || processedKeys.has(parsed.candidateKey)) continue;
      processedKeys.add(parsed.candidateKey);
      const evalResult = evaluateTalentManagementCard(parsed, filterConfig, whitelistKeys);
      results.push({
        ...parsed,
        ...evalResult,
        actionStatus: 'dry_run_preview',
        clickedReasonButton: ''
      });
    }
    return { results, haltedReason };
  }

  // Execute 模式：每次扫描当前 DOM，优先找到第一个尚未处理的卡片
  let safetyLoops = 0;
  const maxLoops = Math.max(initialCards.length * 3 + 10, 60);
  while (safetyLoops++ < maxLoops && results.length < maxCandidates) {
    if (runOptions.shouldStop && runOptions.shouldStop()) {
      haltedReason = '用户手动停止任务';
      break;
    }

    const currentCards = await adapter.scanVisibleCards();
    let targetParsed = null;
    for (const rawCard of currentCards) {
      const parsed = parse51JobManagementCardText(rawCard, { refYearMonth: runOptions.refYearMonth });
      if (!parsed.candidateName || processedKeys.has(parsed.candidateKey)) continue;
      targetParsed = parsed;
      break;
    }

    if (!targetParsed) {
      break; // 当前页所有卡片均已处理完毕
    }

    processedKeys.add(targetParsed.candidateKey);
    const evalResult = evaluateTalentManagementCard(targetParsed, filterConfig, whitelistKeys);

    if (evalResult.decision === 'UNFIT') {
      const clickOutcome = await adapter.markCardUnfit(targetParsed, evalResult.failedRules);
      if (!clickOutcome || clickOutcome.status !== 'verified') {
        haltedReason = (clickOutcome && clickOutcome.message) || `候选人 ${targetParsed.candidateName} 点击不合适后未通过页面核验，已安全暂停`;
        results.push({
          ...targetParsed,
          ...evalResult,
          actionStatus: 'failed_halted',
          actionMessage: haltedReason,
          clickedReasonButton: (clickOutcome && clickOutcome.clickedReasonButton) || ''
        });
        break;
      }
      results.push({
        ...targetParsed,
        ...evalResult,
        actionStatus: 'marked_unfit',
        actionMessage: `已在 51job 标记不合适（勾选：${clickOutcome.clickedReasonButton}）`,
        clickedReasonButton: clickOutcome.clickedReasonButton
      });
      if (runOptions.onProgress) {
        runOptions.onProgress(results[results.length - 1], results.length);
      }
      if (adapter.waitBetweenActions) {
        await adapter.waitBetweenActions(gaussianRandomDelayMs(runOptions.minDelayMs || 2000, runOptions.maxDelayMs || 5000));
      }
    } else {
      // KEEP_MATCHED / KEEP_UNKNOWN / WHITELIST_SKIP：绝不点「合适」或「不合适」，原样保留在待处理
      results.push({
        ...targetParsed,
        ...evalResult,
        actionStatus: evalResult.decision === 'WHITELIST_SKIP' ? 'skipped_whitelist' : 'kept_pending',
        actionMessage: evalResult.reasonLabel,
        clickedReasonButton: ''
      });
      if (runOptions.onProgress) {
        runOptions.onProgress(results[results.length - 1], results.length);
      }
    }
  }

  return { results, haltedReason };
}

/**
 * 在浏览器页面中读取当前激活职位、待处理/不合适计数及可见卡片列表
 */
async function createPuppeteerPageAdapter(page, expectedJobTitle = '') {
  return {
    async verifyAndSelectJobAndPendingTab() {
      return page.evaluate(targetJob => {
        const clean = s => String(s || '').replace(/\s+/g, ' ').trim();
        const bodyText = document.body ? document.body.innerText : '';
        if (/滑动验证|安全验证|请完成验证|验证码/.test(bodyText)) {
          return { ok: false, message: '检测到 51job 页面出现安全验证，请先在浏览器完成验证后再试' };
        }

        // 如果传入了目标职位名，尝试核对或点击顶部职位胶囊
        if (targetJob) {
          const allJobItems = Array.from(document.querySelectorAll('[class*="job"], .el-tabs__item, .tab-item, span, div'))
            .filter(el => el.children.length === 0 && clean(el.textContent) === clean(targetJob) && el.getClientRects().length > 0);
          if (allJobItems.length > 0) {
            const targetEl = allJobItems[0].closest('[class*="item"], [class*="tab"], button, li') || allJobItems[0];
            if (targetEl && !/active|is-active|selected|cur/i.test(targetEl.className || '')) {
              targetEl.click();
            }
          }
        }

        // 确保处于「待处理」二级 Tab
        const pendingTabs = Array.from(document.querySelectorAll('div, span, li, a'))
          .filter(el => /^待处理(?:\s*[·•]\s*\d+)?$/.test(clean(el.textContent)) && el.getClientRects().length > 0);
        if (pendingTabs.length > 0) {
          const tabEl = pendingTabs[0].closest('[class*="tab"], [class*="item"], li') || pendingTabs[0];
          if (tabEl && !/active|is-active|selected|cur/i.test(tabEl.className || '')) {
            tabEl.click();
          }
        }

        return { ok: true };
      }, expectedJobTitle);
    },

    async scanVisibleCards() {
      return page.evaluate(() => {
        const clean = s => String(s || '').trim();
        // 通过右侧「不合适」按钮反向向上定位单张候选人卡片容器
        const unfitButtons = Array.from(document.querySelectorAll('button, span, div, a')).filter(el => {
          const t = clean(el.textContent).replace(/^[×xX✕]\s*/, '');
          return t === '不合适' && el.children.length <= 2 && el.getClientRects().length > 0;
        });

        const containers = [];
        const seenNodes = new Set();
        for (const btn of unfitButtons) {
          let curr = btn.parentElement;
          let matchedCard = null;
          for (let depth = 0; depth < 10 && curr && curr !== document.body; depth++) {
            const txt = curr.innerText || '';
            if ((txt.includes('求职意向') || /\d{2}\s*岁\s*\|/.test(txt)) && txt.includes('不合适')) {
              matchedCard = curr;
              break;
            }
            curr = curr.parentElement;
          }
          if (matchedCard && !seenNodes.has(matchedCard)) {
            seenNodes.add(matchedCard);
            containers.push(matchedCard);
          }
        }

        return containers.map(card => {
          const rawText = card.innerText || '';
          const link = card.querySelector('a[href*="resumeId="], a[href*="/talent/resume/detail"]');
          let resumeId = card.getAttribute('data-resume-id') || card.getAttribute('data-resumeid') || '';
          let profileUrl = link ? link.href : '';
          if (!resumeId && profileUrl) {
            try {
              const u = new URL(profileUrl, location.origin);
              resumeId = u.searchParams.get('resumeId') || '';
            } catch {}
          }

          // 检测头像角标性别
          let gender = '';
          const html = card.innerHTML || '';
          if (/icon-male|gender-male|sex-male|boy|male\.|♂/i.test(html)) gender = '男';
          else if (/icon-female|gender-female|sex-female|girl|female\.|♀/i.test(html)) gender = '女';

          // 提取候选人姓名节点
          const nameEl = card.querySelector('.name, .user-name, .candidate-name, [class*="userName"], [class*="name"]');
          let candidateName = nameEl ? clean(nameEl.textContent).split(/\s+/)[0] : '';
          if (candidateName && /求职意向|投递|合适/.test(candidateName)) candidateName = '';

          return {
            resumeId,
            profileUrl,
            candidateName,
            gender,
            rawText
          };
        });
      });
    },

    async markCardUnfit(targetCard, failedRules) {
      // Step 1: 在最新 DOM 中根据候选人姓名与特征指纹锁定卡片并点击「不合适」
      const clickedUnfit = await page.evaluate(target => {
        const clean = s => String(s || '').trim();
        const readCounters = () => {
          const body = document.body ? document.body.innerText : '';
          const pMatch = body.match(/待处理\s*[·•]?\s*(\d+)/);
          const uMatch = body.match(/不合适\s*[·•]?\s*(\d+)/);
          return {
            pendingCount: pMatch ? parseInt(pMatch[1], 10) : null,
            unfitCount: uMatch ? parseInt(uMatch[1], 10) : null
          };
        };

        const beforeCounters = readCounters();
        const unfitButtons = Array.from(document.querySelectorAll('button, span, div, a')).filter(el => {
          const t = clean(el.textContent).replace(/^[×xX✕]\s*/, '');
          return t === '不合适' && el.children.length <= 2 && el.getClientRects().length > 0;
        });

        for (const btn of unfitButtons) {
          let curr = btn.parentElement;
          let card = null;
          for (let depth = 0; depth < 10 && curr && curr !== document.body; depth++) {
            const txt = curr.innerText || '';
            if ((txt.includes('求职意向') || /\d{2}\s*岁\s*\|/.test(txt)) && txt.includes('不合适')) {
              card = curr;
              break;
            }
            curr = curr.parentElement;
          }
          if (!card) continue;
          const cardText = card.innerText || '';
          if (!cardText.includes(target.candidateName)) continue;
          if (target.school && !cardText.includes(target.school)) continue;
          if (target.residence && !cardText.includes(target.residence)) continue;

          const clickTarget = btn.closest('button, a, [role="button"]') || btn;
          clickTarget.click();
          return { ok: true, beforeCounters };
        }
        return { ok: false, message: `未能在当前页面唯一定位候选人 ${target.candidateName} 的「不合适」按钮` };
      }, {
        candidateName: targetCard.candidateName,
        school: targetCard.school,
        residence: targetCard.residence
      });

      if (!clickedUnfit || !clickedUnfit.ok) {
        return { status: 'error', message: (clickedUnfit && clickedUnfit.message) || '未能点击不合适按钮' };
      }

      // Step 2: 等待「选择原因，为您优化推荐」浮层弹出并读取可选原因按钮
      const popoverOptions = await page.waitForFunction(() => {
        const popovers = Array.from(document.querySelectorAll('.el-popover, .el-dialog, [role="tooltip"], [class*="popover"], [class*="reason"]'))
          .filter(el => el.getClientRects().length > 0 && /选择原因|优化推荐|其他原因/.test(el.innerText || ''));
        if (!popovers.length) return null;
        const pop = popovers[popovers.length - 1];
        const btns = Array.from(pop.querySelectorAll('button, span, li, div'))
          .filter(el => el.children.length === 0 && el.getClientRects().length > 0)
          .map(el => (el.textContent || '').trim())
          .filter(t => /不匹配$|^其他原因$/.test(t));
        return btns.length ? btns : null;
      }, { polling: 150, timeout: 4000 }).then(handle => handle.jsonValue()).catch(() => null);

      if (!popoverOptions || !popoverOptions.length) {
        return {
          status: 'error',
          message: `点击 ${targetCard.candidateName} 的「不合适」后未弹出「选择原因」浮层，已暂停以防误操作`
        };
      }

      const chosenReasonText = pick51JobUnfitReasonOption(failedRules, popoverOptions);

      // Step 3: 点击匹配到的原因按钮
      const reasonClicked = await page.evaluate(targetReason => {
        const popovers = Array.from(document.querySelectorAll('.el-popover, .el-dialog, [role="tooltip"], [class*="popover"], [class*="reason"]'))
          .filter(el => el.getClientRects().length > 0 && /选择原因|优化推荐|其他原因/.test(el.innerText || ''));
        if (!popovers.length) return false;
        const pop = popovers[popovers.length - 1];
        const items = Array.from(pop.querySelectorAll('button, span, li, div'))
          .filter(el => el.children.length === 0 && el.getClientRects().length > 0);
        const match = items.find(el => (el.textContent || '').trim() === targetReason) ||
          items.find(el => (el.textContent || '').trim() === '其他原因');
        if (!match) return false;
        const clickable = match.closest('button, li, [role="button"]') || match;
        clickable.click();
        return true;
      }, chosenReasonText);

      if (!reasonClicked) {
        return {
          status: 'error',
          message: `未能点击原因按钮「${chosenReasonText}」，已安全暂停`
        };
      }

      // Step 4: 验证该候选人卡片从「待处理」列表消失，或顶部「不合适 · N」计数增加
      const verified = await page.waitForFunction((target, before) => {
        const body = document.body ? document.body.innerText : '';
        const uMatch = body.match(/不合适\s*[·•]?\s*(\d+)/);
        const pMatch = body.match(/待处理\s*[·•]?\s*(\d+)/);
        const currentUnfit = uMatch ? parseInt(uMatch[1], 10) : null;
        const currentPending = pMatch ? parseInt(pMatch[1], 10) : null;

        if (before.unfitCount !== null && currentUnfit !== null && currentUnfit > before.unfitCount) {
          return true;
        }
        if (before.pendingCount !== null && currentPending !== null && currentPending < before.pendingCount) {
          return true;
        }

        // 检查卡片是否已从当前 DOM 中卸载
        const clean = s => String(s || '').trim();
        const unfitButtons = Array.from(document.querySelectorAll('button, span, div, a')).filter(el => {
          const t = clean(el.textContent).replace(/^[×xX✕]\s*/, '');
          return t === '不合适' && el.children.length <= 2 && el.getClientRects().length > 0;
        });
        for (const btn of unfitButtons) {
          let curr = btn.parentElement;
          for (let depth = 0; depth < 10 && curr && curr !== document.body; depth++) {
            const txt = curr.innerText || '';
            if ((txt.includes('求职意向') || /\d{2}\s*岁\s*\|/.test(txt)) && txt.includes('不合适')) {
              if (txt.includes(target.candidateName) && (!target.school || txt.includes(target.school))) {
                return false;
              }
              break;
            }
            curr = curr.parentElement;
          }
        }
        return true;
      }, { polling: 200, timeout: 4500 }, {
        candidateName: targetCard.candidateName,
        school: targetCard.school
      }, clickedUnfit.beforeCounters || {}).then(() => true).catch(() => false);

      if (!verified) {
        return {
          status: 'error',
          clickedReasonButton: chosenReasonText,
          message: `点击原因「${chosenReasonText}」后，页面未确认卡片移除或计数变化，已暂停`
        };
      }

      return {
        status: 'verified',
        clickedReasonButton: chosenReasonText
      };
    },

    async goToNextPage() {
      return page.evaluate(() => {
        const nextBtn = document.querySelector('.el-pagination .btn-next:not([disabled]):not(.is-disabled), .pagination .next:not(.disabled)');
        if (!nextBtn || nextBtn.getClientRects().length === 0) return false;
        if (nextBtn.getAttribute('disabled') !== null || nextBtn.classList.contains('disabled') || nextBtn.classList.contains('is-disabled')) {
          return false;
        }
        nextBtn.click();
        return true;
      });
    },

    async waitBetweenActions(ms) {
      await new Promise(resolve => setTimeout(resolve, ms));
    }
  };
}

function parseCliArgs(argv = process.argv.slice(2)) {
  const opts = {
    mode: 'dry_run',
    jobTitle: '',
    maxCandidates: 200,
    filterConfig: {},
    whitelist: [],
    stopFile: ''
  };
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === '--mode' && argv[i + 1]) opts.mode = argv[++i];
    else if (arg === '--job-title' && argv[i + 1]) opts.jobTitle = argv[++i];
    else if (arg === '--max-candidates' && argv[i + 1]) opts.maxCandidates = parseInt(argv[++i], 10) || 200;
    else if (arg === '--stop-file' && argv[i + 1]) opts.stopFile = argv[++i];
    else if (arg === '--filter-json' && argv[i + 1]) {
      try { opts.filterConfig = JSON.parse(argv[++i]); } catch {}
    } else if (arg === '--whitelist-json' && argv[i + 1]) {
      try { opts.whitelist = JSON.parse(argv[++i]); } catch {}
    }
  }
  return opts;
}

async function runCli(argv = process.argv.slice(2)) {
  const opts = parseCliArgs(argv);
  const whitelistSet = new Set(Array.isArray(opts.whitelist) ? opts.whitelist : []);
  let browser;
  try {
    browser = await puppeteer.connect({
      browserURL: 'http://127.0.0.1:9503',
      defaultViewport: null,
      protocolTimeout: 15000
    });
  } catch (err) {
    process.stdout.write(JSON.stringify({
      type: 'result',
      status: 'needs_login',
      message: '无法连接 51job 专用浏览器（端口 9503），请先在软件中打开或登录 51job 企业端'
    }) + '\n');
    return;
  }

  try {
    const pages = await browser.pages();
    let page = pages.find(p => /ehire\.51job\.com\/Revision\/talent\/management/i.test(p.url()));
    if (!page) {
      const ehirePage = pages.find(p => /ehire\.51job\.com/i.test(p.url()));
      if (!ehirePage) {
        process.stdout.write(JSON.stringify({
          type: 'result',
          status: 'needs_action',
          message: '请先在 51job 企业端打开「人才管理」(https://ehire.51job.com/Revision/talent/management) 页面'
        }) + '\n');
        return;
      }
      page = ehirePage;
      await page.goto('https://ehire.51job.com/Revision/talent/management', {
        waitUntil: 'domcontentloaded',
        timeout: 15000
      });
      await new Promise(r => setTimeout(r, 1500));
    }

    const adapter = await createPuppeteerPageAdapter(page, opts.jobTitle);
    const navCheck = await adapter.verifyAndSelectJobAndPendingTab();
    if (!navCheck || !navCheck.ok) {
      process.stdout.write(JSON.stringify({
        type: 'result',
        status: 'needs_action',
        message: (navCheck && navCheck.message) || '未能切换到 51job 人才管理「待处理」列表'
      }) + '\n');
      return;
    }

    await new Promise(r => setTimeout(r, 800));

    const processedKeys = new Set();
    const allItems = [];
    let haltedReason = '';
    let pageNo = 1;

    while (allItems.length < opts.maxCandidates && pageNo <= 20) {
      if (opts.stopFile && fs.existsSync(opts.stopFile)) {
        haltedReason = '用户手动停止任务';
        break;
      }

      const { results, haltedReason: pageHalted } = await processPageCardsInPlace(
        adapter,
        opts.filterConfig,
        whitelistSet,
        {
          mode: opts.mode,
          maxCandidates: opts.maxCandidates - allItems.length,
          processedKeys,
          shouldStop: () => Boolean(opts.stopFile && fs.existsSync(opts.stopFile)),
          onProgress: (item) => {
            process.stdout.write(JSON.stringify({
              type: 'progress',
              item,
              processedCount: allItems.length + 1
            }) + '\n');
          }
        }
      );

      allItems.push(...results);
      if (pageHalted) {
        haltedReason = pageHalted;
        break;
      }
      if (results.length === 0 || allItems.length >= opts.maxCandidates) {
        break;
      }

      const hasNext = await adapter.goToNextPage();
      if (!hasNext) break;
      pageNo++;
      await new Promise(r => setTimeout(r, 1400));
    }

    const summary = summarizeFilterItems(allItems);
    process.stdout.write(JSON.stringify({
      type: 'result',
      status: haltedReason ? 'halted' : 'ok',
      mode: opts.mode,
      message: haltedReason || (allItems.length ? `已完成 ${allItems.length} 位待处理候选人${opts.mode === 'execute' ? '自动筛选处理' : '只读试算'}` : '当前页面未扫描到待处理候选人卡片'),
      summary,
      items: allItems
    }) + '\n');
  } catch (err) {
    process.stdout.write(JSON.stringify({
      type: 'result',
      status: 'error',
      message: `51job 人才管理筛选异常: ${err.message || String(err)}`
    }) + '\n');
  } finally {
    if (browser) await browser.disconnect().catch(() => {});
  }
}

function summarizeFilterItems(items = []) {
  let unfitCount = 0;
  let keepMatchedCount = 0;
  let keepUnknownCount = 0;
  let whitelistSkipCount = 0;
  for (const item of items) {
    if (item.decision === 'UNFIT') unfitCount++;
    else if (item.decision === 'KEEP_MATCHED') keepMatchedCount++;
    else if (item.decision === 'KEEP_UNKNOWN') keepUnknownCount++;
    else if (item.decision === 'WHITELIST_SKIP') whitelistSkipCount++;
  }
  return {
    totalScanned: items.length,
    unfitCount,
    keepMatchedCount,
    keepUnknownCount,
    whitelistSkipCount
  };
}

if (require.main === module) {
  runCli();
}

module.exports = {
  educationRank,
  splitFilterTokens,
  parseSalaryRangeK,
  parseWorkYears,
  isGradAfterMonth,
  parse51JobManagementCardText,
  evaluateTalentManagementCard,
  pick51JobUnfitReasonOption,
  gaussianRandomDelayMs,
  processPageCardsInPlace,
  summarizeFilterItems,
  parseCliArgs
};
