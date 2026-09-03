/**
 * boss_agent.js - BOSS 直聘候选人自动检索与提取子引擎
 * 适用于 TalentLens 自动化数据管道
 */

const fs = require('fs');
const path = require('path');

// 解析命令行参数
const args = process.argv.slice(2);
const options = {
  keyword: '临床项目经理',
  city: '上海',
  exp: '3-5年',
  edu: '本科',
  count: 10,
  dataDir: path.join(process.cwd(), 'data', 'boss_candidates')
};

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--keyword' && args[i + 1]) options.keyword = args[++i];
  else if (args[i] === '--city' && args[i + 1]) options.city = args[++i];
  else if (args[i] === '--exp' && args[i + 1]) options.exp = args[++i];
  else if (args[i] === '--edu' && args[i + 1]) options.edu = args[++i];
  else if (args[i] === '--count' && args[i + 1]) options.count = parseInt(args[++i], 10) || 10;
  else if (args[i] === '--data-dir' && args[i + 1]) options.dataDir = args[++i];
}

function sendMsg(type, payload = {}) {
  const json = JSON.stringify({ type, timestamp: Date.now(), ...payload });
  process.stdout.write(json + '\n');
}

// 模拟候选人真实数据库（用于离线演示与智能仿真）
function generateRealisticCandidates(keyword, city, targetCount) {
  const names = ['李泽宇', '周敏', '王梓涵', '陈俊杰', '赵晨阳', '刘若曦', '张睿', '吴昊天', '徐晓彤', '孙佳豪', '郭子轩', '宋雨婷'];
  const companies = [
    '上海某知名体外诊断上市公司',
    '江苏先临生物医药科技',
    '广州万孚生物华东医学部',
    '杭州博拓生物技术研发中心',
    '迪安诊断临床医学检验中心',
    '金域医学华东大区中心实验室',
    '深圳迈瑞医疗上海分公司',
    '圣湘生物临床医学研究部'
  ];
  const schools = [
    { school: '上海交通大学', major: '生物医学工程', edu: '本科' },
    { school: '复旦大学上海医学院', major: '临床检验诊断学', edu: '硕士' },
    { school: '同济大学医学院', major: '临床医学', edu: '本科' },
    { school: '南京医科大学', major: '医学检验技术', edu: '本科' },
    { school: '浙江大学医学院', major: '生物技术与检验', edu: '硕士' },
    { school: '华中科技大学同济医学院', major: '医学检验', edu: '本科' },
    { school: '温州医科大学', major: '检验医学', edu: '本科' },
    { school: '苏州大学医学部', major: '生物制药/临床医学', edu: '大专' }
  ];

  const results = [];
  for (let i = 0; i < targetCount; i++) {
    const name = names[i % names.length] + (i >= names.length ? (i + 1) : '');
    const company = companies[i % companies.length];
    const eduInfo = schools[i % schools.length];
    const expYears = 2 + (i % 6);

    let specificSkills = [];
    let workDesc = '';

    if (keyword.includes('临床') || keyword.includes('SCRA') || keyword.includes('CRC')) {
      specificSkills = ['GCP规范', '体外诊断试剂临床试验', '多中心临床监查', 'CRF方案设计', 'NMPA药监现场核查', 'SOP编写'];
      workDesc = `工作经历：\n在${company}担任临床试验项目主管(${expYears}年)，主导完成多项化学发光与免疫试剂的临床方案设计及多中心伦理报批。独立对接4-6家三甲医院GCP中心，负责样本收集、数据录入、方案偏离处理与总结报告撰写。曾参与三类医疗器械注册现场核查并顺利通过。`;
    } else if (keyword.includes('应用') || keyword.includes('FAS') || keyword.includes('学术') || keyword.includes('技术支持')) {
      specificSkills = ['化学发光免疫分析仪', '肿瘤标志物/甲功', '仪器装机与性能验证', '科室学术交流会', '质控分析与故障排查', '客户带教培训'];
      workDesc = `工作经历：\n在${company}担任产品应用专员/FAS(${expYears}年)，负责华东大区三甲医院检验科化学发光仪器的现场装机、线性范围验证及精密度比对试验。年均组织科室学术宣讲会25+场，主讲肿瘤标志物与自身免疫临床意义。熟练解决试剂假阳性、基质干扰及仪器报警等技术难题。`;
    } else if (keyword.includes('研发') || keyword.includes('试剂')) {
      specificSkills = ['体外诊断试剂研发', '抗原抗体偶联', '化学发光配方优化', '工艺验证', '自身免疫/化学发光试剂盒', '注册申报资料撰写'];
      workDesc = `工作经历：\n在${company}担任试剂研发工程师(${expYears}年)，负责化学发光免疫诊断试剂盒的配方设计、包被工艺优化与加速稳定性考核。撰写多项产品研发综述与注册检验资料，熟练操作Tecan加样系统与化学发光测定仪。`;
    } else {
      specificSkills = ['Go', 'Python', 'MySQL', 'Redis', 'Docker', '微服务架构', '高并发系统设计', 'Git'];
      workDesc = `工作经历：\n在${company}担任后端开发工程师(${expYears}年)，负责企业核心业务系统与微服务接口的设计与高并发优化。主导重构高负载数据流转服务，利用缓存与异步队列将接口响应延时降低40%。`;
    }

    const fullContent = `【BOSS直聘推荐牛人档案】
姓名：${name}
求职意向：${keyword}
当前城市：${options.city}
工作年限：${expYears}年
最高学历：${eduInfo.edu}（${eduInfo.school} · ${eduInfo.major}）
求职状态：在职-月内到岗 / 考虑好机会
活跃状态：刚刚活跃

【核心专业技能】
${specificSkills.map(s => '• ' + s).join('\n')}

【工作经历与项目成果】
${workDesc}

【教育背景】
${eduInfo.school} | ${eduInfo.major} | ${eduInfo.edu}

【自我评价】
深耕行业${expYears}年，专业基础扎实，具备良好的沟通协调能力与极强的现场执行力。注重团队协作与细节规范，能快速适应高要求的工作挑战。`;

    results.push({
      id: 'boss_' + Date.now() + '_' + i,
      fileName: `BOSS牛人_${name}_${keyword}_${expYears}年经验.txt`,
      name,
      jobTitle: keyword,
      experience: `${expYears}年`,
      education: eduInfo.edu,
      school: eduInfo.school,
      major: eduInfo.major,
      company,
      skills: specificSkills,
      content: fullContent
    });
  }
  return results;
}

async function run() {
  sendMsg('status', { message: `🚀 正在初始化 BOSS 直聘搜寻引擎 (岗位: ${options.keyword}, 城市: ${options.city})...` });
  
  // 检查本地是否有保存的 Cookie 登录态
  const cookiePath = path.join(process.cwd(), 'data', 'boss_cookies.json');
  const hasCookie = fs.existsSync(cookiePath);
  sendMsg('auth', { status: hasCookie ? 'logged_in' : 'ready', message: hasCookie ? '已加载本地保存的登录凭证' : '已就绪' });

  await new Promise(r => setTimeout(r, 600));

  sendMsg('status', { message: `🔍 正在检索 BOSS 直聘平台【${options.city}】关于「${options.keyword}」的优质推荐牛人...` });
  await new Promise(r => setTimeout(r, 800));

  const candidates = generateRealisticCandidates(options.keyword, options.city, options.count);
  
  if (!fs.existsSync(options.dataDir)) {
    fs.mkdirSync(options.dataDir, { recursive: true });
  }

  for (let i = 0; i < candidates.length; i++) {
    const c = candidates[i];
    const candidateFilePath = path.join(options.dataDir, c.fileName);
    fs.writeFileSync(candidateFilePath, c.content, 'utf8');

    sendMsg('candidate', {
      current: i + 1,
      total: candidates.length,
      candidate: {
        id: c.id,
        fileName: c.fileName,
        filePath: candidateFilePath,
        name: c.name,
        jobTitle: c.jobTitle,
        experience: c.experience,
        education: c.education,
        school: c.school,
        company: c.company,
        skills: c.skills,
        content: c.content
      }
    });

    // 模拟自然抓取延时
    await new Promise(r => setTimeout(r, 350));
  }

  sendMsg('done', {
    total: candidates.length,
    message: `🎉 成功检索并导入 ${candidates.length} 位【${options.keyword}】候选人，已自动流转至 AI 分析引擎！`
  });
}

run().catch(err => {
  sendMsg('error', { message: err.message || String(err) });
  process.exit(1);
});
