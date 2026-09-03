// 岗位模板预设数据
export interface JobPreset {
  id: string
  name: string
  nameEn: string
  category: string
  requiredSkills: string[]
  preferredSkills?: string[]
  experienceYears: number
  educationLevel: string
  description?: string
}

export interface JobCategory {
  id: string
  labelKey: string // i18n key
  presets: JobPreset[]
}

// 技术类岗位
const techJobs: JobPreset[] = [
  {
    id: 'frontend-senior',
    name: '高级前端工程师',
    nameEn: 'Senior Frontend Engineer',
    category: 'tech',
    requiredSkills: ['JavaScript', 'TypeScript', 'React', 'Vue', 'CSS', 'HTML'],
    preferredSkills: ['Node.js', 'Webpack', 'Vite', 'GraphQL', '性能优化'],
    experienceYears: 5,
    educationLevel: '本科',
    description: '负责公司核心产品的前端架构设计与开发'
  },
  {
    id: 'frontend-junior',
    name: '前端工程师',
    nameEn: 'Frontend Engineer',
    category: 'tech',
    requiredSkills: ['JavaScript', 'HTML', 'CSS', 'Vue'],
    preferredSkills: ['TypeScript', 'React', 'Element Plus'],
    experienceYears: 2,
    educationLevel: '本科'
  },
  {
    id: 'backend-go',
    name: '高级Go开发工程师',
    nameEn: 'Senior Go Developer',
    category: 'tech',
    requiredSkills: ['Go', 'MySQL', 'Redis', 'Docker', 'Linux'],
    preferredSkills: ['Kubernetes', 'gRPC', 'Kafka', '微服务', '分布式系统'],
    experienceYears: 5,
    educationLevel: '本科',
    description: '负责后端服务架构设计与核心功能开发'
  },
  {
    id: 'backend-java',
    name: '高级Java开发工程师',
    nameEn: 'Senior Java Developer',
    category: 'tech',
    requiredSkills: ['Java', 'Spring Boot', 'MySQL', 'Redis', 'MyBatis'],
    preferredSkills: ['微服务', 'Dubbo', 'RocketMQ', 'Kubernetes'],
    experienceYears: 5,
    educationLevel: '本科'
  },
  {
    id: 'backend-python',
    name: 'Python开发工程师',
    nameEn: 'Python Developer',
    category: 'tech',
    requiredSkills: ['Python', 'Django', 'Flask', 'MySQL', 'Redis'],
    preferredSkills: ['FastAPI', 'Celery', 'Docker', '机器学习'],
    experienceYears: 3,
    educationLevel: '本科'
  },
  {
    id: 'fullstack',
    name: '全栈工程师',
    nameEn: 'Full Stack Engineer',
    category: 'tech',
    requiredSkills: ['JavaScript', 'Node.js', 'Vue', 'MySQL', 'Docker'],
    preferredSkills: ['TypeScript', 'React', 'MongoDB', 'AWS'],
    experienceYears: 4,
    educationLevel: '本科'
  },
  {
    id: 'devops',
    name: '运维工程师',
    nameEn: 'DevOps Engineer',
    category: 'tech',
    requiredSkills: ['Linux', 'Docker', 'Kubernetes', 'CI/CD', 'Shell'],
    preferredSkills: ['Terraform', 'Ansible', 'Prometheus', 'ELK'],
    experienceYears: 3,
    educationLevel: '本科'
  },
  {
    id: 'ai-engineer',
    name: 'AI算法工程师',
    nameEn: 'AI Engineer',
    category: 'tech',
    requiredSkills: ['Python', '机器学习', '深度学习', 'PyTorch', 'TensorFlow'],
    preferredSkills: ['NLP', 'CV', 'LLM', '模型部署', 'CUDA'],
    experienceYears: 3,
    educationLevel: '硕士'
  },
  {
    id: 'data-engineer',
    name: '数据工程师',
    nameEn: 'Data Engineer',
    category: 'tech',
    requiredSkills: ['SQL', 'Python', 'Spark', 'Hive', 'ETL'],
    preferredSkills: ['Flink', 'Kafka', 'Airflow', '数据仓库'],
    experienceYears: 3,
    educationLevel: '本科'
  },
  {
    id: 'mobile-ios',
    name: 'iOS开发工程师',
    nameEn: 'iOS Developer',
    category: 'tech',
    requiredSkills: ['Swift', 'Objective-C', 'iOS SDK', 'Xcode'],
    preferredSkills: ['SwiftUI', 'RxSwift', '性能优化', 'App Store发布'],
    experienceYears: 3,
    educationLevel: '本科'
  },
  {
    id: 'mobile-android',
    name: 'Android开发工程师',
    nameEn: 'Android Developer',
    category: 'tech',
    requiredSkills: ['Kotlin', 'Java', 'Android SDK', 'Android Studio'],
    preferredSkills: ['Jetpack Compose', 'Flutter', 'Gradle'],
    experienceYears: 3,
    educationLevel: '本科'
  }
]

// 产品类岗位
const productJobs: JobPreset[] = [
  {
    id: 'product-manager',
    name: '产品经理',
    nameEn: 'Product Manager',
    category: 'product',
    requiredSkills: ['需求分析', 'PRD撰写', '原型设计', '数据分析', '用户调研'],
    preferredSkills: ['SQL', 'Axure', 'Figma', 'Jira'],
    experienceYears: 3,
    educationLevel: '本科'
  },
  {
    id: 'product-director',
    name: '产品总监',
    nameEn: 'Product Director',
    category: 'product',
    requiredSkills: ['产品规划', '团队管理', '战略思维', '商业分析', '项目管理'],
    preferredSkills: ['行业洞察', 'OKR', '跨部门协作'],
    experienceYears: 8,
    educationLevel: '本科'
  },
  {
    id: 'product-operation',
    name: '产品运营',
    nameEn: 'Product Operations',
    category: 'product',
    requiredSkills: ['用户运营', '数据分析', '活动策划', '内容运营'],
    preferredSkills: ['增长黑客', 'A/B测试', 'SQL'],
    experienceYears: 2,
    educationLevel: '本科'
  }
]

// 运营类岗位
const operationJobs: JobPreset[] = [
  {
    id: 'operation-manager',
    name: '运营经理',
    nameEn: 'Operations Manager',
    category: 'operation',
    requiredSkills: ['用户运营', '活动策划', '数据分析', '内容策划'],
    preferredSkills: ['社群运营', 'SEO/SEM', '增长黑客'],
    experienceYears: 5,
    educationLevel: '本科'
  },
  {
    id: 'content-operation',
    name: '内容运营',
    nameEn: 'Content Operations',
    category: 'operation',
    requiredSkills: ['内容策划', '文案撰写', '新媒体运营', '数据分析'],
    preferredSkills: ['视频剪辑', 'SEO', '热点追踪'],
    experienceYears: 2,
    educationLevel: '本科'
  },
  {
    id: 'user-operation',
    name: '用户运营',
    nameEn: 'User Operations',
    category: 'operation',
    requiredSkills: ['用户增长', '用户分层', '活动策划', '数据分析'],
    preferredSkills: ['社群运营', 'CRM', '用户画像'],
    experienceYears: 3,
    educationLevel: '本科'
  },
  {
    id: 'growth-hacker',
    name: '增长运营',
    nameEn: 'Growth Hacker',
    category: 'operation',
    requiredSkills: ['增长策略', '数据分析', 'A/B测试', '渠道运营'],
    preferredSkills: ['SQL', 'Python', '裂变营销'],
    experienceYears: 3,
    educationLevel: '本科'
  }
]

// 设计类岗位
const designJobs: JobPreset[] = [
  {
    id: 'ui-designer',
    name: 'UI设计师',
    nameEn: 'UI Designer',
    category: 'design',
    requiredSkills: ['Figma', 'Sketch', '视觉设计', '设计规范', 'UI组件'],
    preferredSkills: ['动效设计', '设计系统', 'Principle'],
    experienceYears: 3,
    educationLevel: '本科'
  },
  {
    id: 'ux-designer',
    name: 'UX设计师',
    nameEn: 'UX Designer',
    category: 'design',
    requiredSkills: ['用户研究', '交互设计', '原型设计', '可用性测试'],
    preferredSkills: ['数据分析', 'Design Thinking', 'Figma'],
    experienceYears: 3,
    educationLevel: '本科'
  },
  {
    id: 'visual-designer',
    name: '视觉设计师',
    nameEn: 'Visual Designer',
    category: 'design',
    requiredSkills: ['Photoshop', 'Illustrator', '品牌设计', '平面设计'],
    preferredSkills: ['C4D', 'After Effects', '插画'],
    experienceYears: 3,
    educationLevel: '本科'
  }
]

// 销售类岗位
const salesJobs: JobPreset[] = [
  {
    id: 'sales-manager',
    name: '销售经理',
    nameEn: 'Sales Manager',
    category: 'sales',
    requiredSkills: ['销售技巧', '客户管理', '商务谈判', '团队管理'],
    preferredSkills: ['CRM系统', '行业资源', '大客户销售'],
    experienceYears: 5,
    educationLevel: '本科'
  },
  {
    id: 'sales-rep',
    name: '销售代表',
    nameEn: 'Sales Representative',
    category: 'sales',
    requiredSkills: ['销售技巧', '客户开发', '沟通能力', '抗压能力'],
    preferredSkills: ['行业知识', 'CRM'],
    experienceYears: 1,
    educationLevel: '大专'
  },
  {
    id: 'bd-manager',
    name: '商务拓展经理',
    nameEn: 'Business Development Manager',
    category: 'sales',
    requiredSkills: ['商务谈判', '渠道开发', '合作洽谈', '市场分析'],
    preferredSkills: ['行业资源', '战略合作', '商业模式'],
    experienceYears: 5,
    educationLevel: '本科'
  }
]

// 人力资源类岗位
const hrJobs: JobPreset[] = [
  {
    id: 'hr-manager',
    name: '人力资源经理',
    nameEn: 'HR Manager',
    category: 'hr',
    requiredSkills: ['招聘管理', '绩效管理', '员工关系', '薪酬福利'],
    preferredSkills: ['HRBP', '组织发展', '人才盘点'],
    experienceYears: 5,
    educationLevel: '本科'
  },
  {
    id: 'recruiter',
    name: '招聘专员',
    nameEn: 'Recruiter',
    category: 'hr',
    requiredSkills: ['招聘流程', '简历筛选', '面试技巧', '渠道管理'],
    preferredSkills: ['猎头合作', 'ATS系统', '雇主品牌'],
    experienceYears: 2,
    educationLevel: '本科'
  },
  {
    id: 'hrbp',
    name: 'HRBP',
    nameEn: 'HR Business Partner',
    category: 'hr',
    requiredSkills: ['业务理解', '组织诊断', '人才发展', '员工关系'],
    preferredSkills: ['OD', '数据分析', '变革管理'],
    experienceYears: 5,
    educationLevel: '本科'
  }
]

// 财务类岗位
const financeJobs: JobPreset[] = [
  {
    id: 'finance-manager',
    name: '财务经理',
    nameEn: 'Finance Manager',
    category: 'finance',
    requiredSkills: ['财务分析', '预算管理', '成本控制', '财务报表'],
    preferredSkills: ['ERP系统', '税务筹划', '内控管理'],
    experienceYears: 5,
    educationLevel: '本科'
  },
  {
    id: 'accountant',
    name: '会计',
    nameEn: 'Accountant',
    category: 'finance',
    requiredSkills: ['账务处理', '报表编制', '税务申报', '财务软件'],
    preferredSkills: ['成本核算', 'Excel', '审计配合'],
    experienceYears: 2,
    educationLevel: '本科'
  }
]

export interface JobPreset {
  id: string
  name: string
  nameEn: string
  category: string
  requiredSkills: string[]
  preferredSkills?: string[]
  experienceYears: number
  educationLevel: string
  description?: string
  jobDescription?: string // 详细岗位职责与技能要求
  isCustom?: boolean
}

export interface JobCategory {
  id: string
  labelKey: string // i18n key
  presets: JobPreset[]
}

// 生物医药 / IVD 体外诊断类岗位 (来自透景生命实聘)
const medicalJobs: JobPreset[] = [
  {
    id: 'ivd-clin-pm-1',
    name: '临床项目经理',
    nameEn: 'Clinical Project Manager',
    category: 'medical',
    requiredSkills: ['临床试验方案设计', '质量控制', 'SOP标准操作', '药监核查', '生物/医学背景'],
    preferredSkills: ['沟通协调', '抗压能力', '出差适应', '团队管理'],
    experienceYears: 3,
    educationLevel: '本科',
    description: '负责体外诊断试剂临床试验全流程方案设计、质量管理与药监申报',
    jobDescription: '岗位职责：\n1. 制定项目管理计划，按计划完成所负责的项目启动、开展及结束工作，确保通过药监部门核查；\n2. 根据项目研发资料、指导原则、行业标准等要求，设计临床试验方案；\n3. 对所负责的临床试验项目进行全面的质量控制与管理，确保所有试验严格按照临床试验方案、标准操作程序和相关法规进行；\n4. 根据临床试验相关规范要求，完成临床研究的各阶段报批文件包括但不限于临床报告；\n5. 作为公司对外代表，建立并保持与机构的良好关系；\n6. 领导交办的其他工作。\n\n任职要求：\n1. 生物学、检验学、医学或药学等相关专业背景，本科及以上学历；\n2. 高度责任心和抗压能力，可适应出差；\n3. 优秀的沟通表达能力、发现和解决问题的能力。'
  },
  {
    id: 'ivd-intl-sales',
    name: '国际销售经理',
    nameEn: 'International Sales Manager',
    category: 'medical',
    requiredSkills: ['体外诊断海外销售', '客户开拓', '英语流利沟通', '产品推广', '国际市场调研'],
    preferredSkills: ['好学勤奋', '稳重踏实', '外贸跟单', '海外展会'],
    experienceYears: 2,
    educationLevel: '本科',
    description: '负责公司体外诊断(IVD)产品在海外市场的销售与客户开拓',
    jobDescription: '岗位职责：\n1. 负责公司体外诊断产品在海外市场的销售；\n2. 开拓海外客户；\n3. 负责产品推广、信息收集、工作汇报。\n\n任职资格：\n1. 本科及以上学历，检验、生物等相关专业；\n2. 二年以上相关工作经验；\n3. 英语沟通能力好；\n4. 好学、勤奋、踏实、稳重；\n5. 可以驻点在上海或者深圳。\n\n薪酬福利：基本工资+销售提成+各类福利+各类补贴+年终奖'
  },
  {
    id: 'ivd-rd-pm-autoimmunity',
    name: '研发项目经理-自身免疫',
    nameEn: 'R&D Project Manager (Autoimmunity)',
    category: 'medical',
    requiredSkills: ['自身免疫诊断试剂', '试剂研发', '项目管理', '实验室操作', '文献检索与撰写'],
    preferredSkills: ['技术方案改进', '质量性能优化', '化学发光', '酶免实验'],
    experienceYears: 3,
    educationLevel: '本科',
    description: '负责自身免疫诊断试剂的研发、优化与上市前后技术支持',
    jobDescription: '工作内容：\n1. 负责自身免疫诊断试剂的研发工作，包括产品上市前后的开发、优化、改进；\n2. 根据市场反馈与要求，不断调整优化产品质量与性能；\n3. 密切关注市场需求与技术动态，设计产品优化改进方案，完成对现有产品的技术改进。\n\n任职要求：\n1. 生物学、医学、检验学专业毕业，本科以上学历，有3年自身免疫诊断试剂研发相关经验、具备项目管理经验；\n2. 热爱实验室工作，动手能力强，工作有条理；\n3. 有良好的科学文献阅读能力和较强的文字表达能力。'
  },
  {
    id: 'ivd-scra',
    name: '临床SCRA (IVD行业)',
    nameEn: 'Senior Clinical Research Associate (IVD)',
    category: 'medical',
    requiredSkills: ['体外诊断试剂监查', '化学发光', '质谱平台', '临床数据处理', '监查计划与报告'],
    preferredSkills: ['定量试剂', '出差适应', 'GCP规范', '医院机构协调'],
    experienceYears: 3,
    educationLevel: '本科',
    description: '负责体外诊断试剂临床验证全流程监查、质量跟进与机构协调',
    jobDescription: '岗位职责：\n1. 协调解决临床试验过程中出现的问题，协助处理临床数据；\n2. 制定监查计划，制作监查报告，确保临床研究按进度完成；\n3. 根据临床试验要求，及时回收、录入数据；\n4. 负责临床研究质量、研究进度的监查及临床试验工作的协调等；\n5. 领导交办的其他工作。\n\n任职要求：\n1. 生物、药学、医学等相关专业，本科以上学历；有体外诊断试剂临床验证监查实习经验者优先；\n2. 有较强的沟通表达能力，能经常出差；\n3. 诚恳踏实、敬业进取。'
  },
  {
    id: 'ivd-app-specialist',
    name: '产品应用专员',
    nameEn: 'Product Application Specialist',
    category: 'medical',
    requiredSkills: ['化学发光分析仪', '肿瘤标志物/甲功/激素', '科室会学术宣讲', '性能验证比对', '临床检验解读'],
    preferredSkills: ['罗氏/雅培/贝克曼/新产业仪器', '售前售后技术支持', '客情维护', '大专以上'],
    experienceYears: 2,
    educationLevel: '大专',
    description: '负责化学发光免疫分析平台仪器及配套试剂的售前、售中及售后应用技术支持',
    jobDescription: '【岗位职责】\n1. 产品应用支持：负责公司化学发光免疫分析平台（仪器及配套试剂）的售前、售中及售后应用支持，解决客户在产品使用过程中遇到的临床应用及技术问题。\n2. 临床解读与推广：具备扎实的免疫学知识，能够对临床检验结果进行解读与答疑。协助销售团队向临床医生、检验科老师推广产品的临床优势与应用场景。\n3. 客户培训与科室会：独立完成针对终端医院检验科、临床科室的产品科室会/学术会讲解；负责对代理商团队进行产品原理、操作及维护的系统性培训。\n4. 性能验证与比对：配合客户完成化学发光试剂的性能验证（精密度、线性、参考区间建立等）、仪器校准及临床比对实验。\n5. 市场信息反馈：收集并分析竞品信息及临床需求，及时向研发或市场部反馈产品改进建议。\n\n【任职资格】\n1. 本科及以上学历，医学检验、临床医学、生物技术、免疫学或相关专业（条件优秀者可放宽至大专）；\n2. 具有2-4年体外诊断(IVD)行业工作经验，其中至少1-2年化学发光产品（如肿瘤标志物、甲功、激素、心肌标志物等）应用推广或技术支持经验；\n3. 熟悉IVD试剂研发/生产原理，深刻理解化学发光反应机理，熟练操作主流化学发光分析仪（如罗氏、雅培、贝克曼、新产业、安图等）；\n4. 具备优秀的沟通表达能力与逻辑思维能力，能适应经常性短期出差，有较强的课堂掌控力和科室会专业演讲能力。'
  },
  {
    id: 'ivd-clin-pm-clia',
    name: '临床项目经理 (IVD化学发光)',
    nameEn: 'Clinical PM (IVD Chemiluminescence)',
    category: 'medical',
    requiredSkills: ['化学发光', '质谱试剂盒2类证', '临床试验方案设计', '临床监查管理', '药监申报'],
    preferredSkills: ['项目统筹协作', '跨部门沟通', '试验质量把控'],
    experienceYears: 3,
    educationLevel: '本科',
    description: '负责IVD化学发光与质谱试剂盒临床试验项目方案设计与质量控制',
    jobDescription: '岗位职责：\n1. 制定项目管理计划，按计划完成所负责的项目启动、开展及结束工作，确保通过药监部门核查；\n2. 根据项目研发资料、指导原则、行业标准等要求，设计临床试验方案；\n3. 对所负责的临床试验项目进行全面的质量控制与管理，确保所有试验严格按照临床试验方案、标准操作程序和相关法规进行；\n4. 根据临床试验相关规范要求，完成临床研究的各阶段报批文件包括但不限于临床报告；\n5. 作为公司对外代表，建立并保持与机构的良好关系；\n6. 领导交办的其他工作。\n\n任职要求：\n1. 生物学、检验学、医学或药学等相关专业背景，本科及以上学历；曾独立承担临床监查项目，质谱试剂盒2类证经验优先；\n2. 高度责任心和抗压能力，可适应出差；\n3. 优秀的沟通表达能力、发现和解决问题的能力，具有很强的项目管理能力、统筹协作能力及敬业精神。'
  },
  {
    id: 'ivd-product-manager',
    name: 'IVD产品经理',
    nameEn: 'IVD Product Manager',
    category: 'medical',
    requiredSkills: ['设备售后维护与维修', '售前售后技术咨询', '展会学术交流', '老客户上量加项', '竞品信息收集'],
    preferredSkills: ['体外诊断仪器', '客户关怀', '大连驻点'],
    experienceYears: 3,
    educationLevel: '本科',
    description: '负责区域内客户售后维护、设备维修、售前技术咨询与老客户上量',
    jobDescription: '岗位职责：\n1. 负责区域内客户的售后维护工作，处理客户使用过程中的问题和投诉；\n2. 负责区域内公司设备的日常维护与维修；\n3. 负责区域内公司产品的售前和售后技术相关问题咨询；\n4. 参与公司开展的展会及各类活动，与客户交流相关技术问题；\n5. 收集和反馈区域内相关竞品及其它有关信息；\n6. 负责老客户上量、加项工作。\n\n任职要求：本科及以上学历，生物医药、检验相关专业，3年以上IVD设备技术支持与售后维护经验。'
  },
  {
    id: 'ivd-sales-zhejiang',
    name: '区域销售经理-浙江',
    nameEn: 'Regional Sales Manager (Zhejiang)',
    category: 'medical',
    requiredSkills: ['体外诊断销售', '渠道开拓与拜访', '终端客户开发', '销售任务达成', '浙江/宁波驻点'],
    preferredSkills: ['好学勤奋', '稳重踏实', '医药生物背景'],
    experienceYears: 2,
    educationLevel: '大专',
    description: '负责公司体外诊断产品在浙江区域的销售与渠道终端开拓',
    jobDescription: '岗位职责：\n1. 负责公司体外诊断产品的销售，完成销售任务；\n2. 开拓并协助渠道拜访终端客户；\n3. 负责区域产品推广、信息收集、工作汇报；\n4. 驻点在宁波。\n\n任职资格：\n1. 专科学历，检验、医药、生物、营销等相关专业；\n2. 二年以上该区域同岗位工作经验；\n3. 好学、勤奋、踏实、稳重。\n\n薪酬待遇：基本工资+销售提成+季度奖金+各类福利+各类补贴+年终奖'
  },
  {
    id: 'ivd-sales-lanzhou',
    name: '区域销售经理',
    nameEn: 'Regional Sales Manager (Lanzhou)',
    category: 'medical',
    requiredSkills: ['体外诊断销售', '渠道开拓', '终端客户拜访', '兰州驻点', '市场信息收集'],
    preferredSkills: ['检验医药背景', '销售指标达成', '客户关系维护'],
    experienceYears: 2,
    educationLevel: '大专',
    description: '负责公司体外诊断产品在兰州及西北区域的销售与渠道开拓',
    jobDescription: '岗位职责：\n1. 负责公司体外诊断产品的销售，完成销售任务；\n2. 开拓并协助渠道拜访终端客户；\n3. 负责区域产品推广、信息收集、工作汇报；\n4. 驻点在兰州。\n\n任职资格：\n1. 专科学历，检验、医药、生物、营销等相关专业；\n2. 二年以上该区域同岗位工作经验；\n3. 好学、勤奋、踏实、稳重。\n\n薪酬：基本工资+销售提成+各类福利+各类补贴+年终奖'
  },
  {
    id: 'ivd-academic-manager',
    name: '应用学术经理',
    nameEn: 'Application Academic Manager',
    category: 'medical',
    requiredSkills: ['学术推广与宣讲', '售后技术维护', '设备维修与保养', '产品技术咨询', '老客户上量加项'],
    preferredSkills: ['大连驻点', '展会技术交流', '竞品分析'],
    experienceYears: 3,
    educationLevel: '本科',
    description: '负责区域内产品的学术推广、售后维护、仪器维修与售前技术支持',
    jobDescription: '岗位职责：\n1. 负责区域内客户的售后维护工作，处理客户使用过程中的问题和投诉；\n2. 负责区域内公司设备的日常维护与维修；\n3. 负责区域内公司产品的售前和售后技术相关问题咨询；\n4. 参与公司开展的展会及各类活动，与客户交流相关技术问题；\n5. 收集和反馈区域内相关竞品及其它有关信息；\n6. 负责老客户上量、加项工作。\n\n任职资格：本科及以上学历，医学、检验、生物等相关背景，3-5年体外诊断行业学术支持与设备维护经验。'
  },
  {
    id: 'ivd-crc-intern',
    name: '临床CRC实习生-广州',
    nameEn: 'Clinical CRC Intern (Guangzhou)',
    category: 'medical',
    requiredSkills: ['医院临床样本收样', '临床试验协助', '数据表格处理', '办公软件操作', '广州驻点'],
    preferredSkills: ['性格外向善于沟通', '生物医药检验专业', '踏实细心'],
    experienceYears: 0,
    educationLevel: '大专',
    description: '在广州各大医院协助完成体外诊断临床样本收样与试验协助工作',
    jobDescription: '岗位职责及任职要求：\n1. 学历大专及以上，生物医药或检验类相关专业优先考虑；\n2. 能够熟练使用办公软件，简单的数据表格处理；\n3. 性格外向，善于与人沟通交流；\n4. 在医院协助完成临床样本收样工作；\n5. 工作地：广州（海珠区/荔湾区等合作医院）。\n\n待遇：150-200元/天，提供实习证明。'
  },
  {
    id: 'ivd-field-engineer',
    name: '器械工程师 (驻点)',
    nameEn: 'Field Service Engineer (Medical Device)',
    category: 'medical',
    requiredSkills: ['体外诊断仪器维护与维修', '检验产品实验操作', '检验士/师证', 'PCR上岗资格证', '杭州驻点'],
    preferredSkills: ['医学检验/生物工程', '客户售后培训', '故障排除'],
    experienceYears: 1,
    educationLevel: '大专',
    description: '负责驻点医院体外诊断仪器正常运行维护、故障维修与技术支持',
    jobDescription: '工作职责：\n1. 协助所负责客户正常使用本公司产品；\n2. 及时反馈和协助处理客户处问题和投诉；\n3. 维护仪器的正常运行；\n4. 公司安排的其他相关工作。\n\n任职要求：\n1. 医学检验、生物技术或生物工程等相关专业大学专科及以上学历；\n2. 有体外诊断试剂技术(售后)服务工作经验或临床检验工作（实习）经验，熟练掌握检验产品实验操作技能者优先；\n3. 有检验士（师）证和PCR上岗资格证；\n4. 欢迎有相关实习经验应届生投递简历。\n\n工作地点：杭州市拱墅区及周边驻点医院。'
  },
  {
    id: 'ivd-qc-nanchang',
    name: 'QC(南昌进贤)',
    nameEn: 'Quality Control Specialist (Nanchang)',
    category: 'medical',
    requiredSkills: ['GMP质量规范', '化学发光产品检验', '免疫分析仪操作', '中间品/成品/留样检验', '新产品注册报批质检'],
    preferredSkills: ['室间质评', '质控品/标准品校准', '实验记录书写', '南昌驻点'],
    experienceYears: 1,
    educationLevel: '本科',
    description: '负责化学发光诊断试剂中间品、半成品、成品的质量检验与GMP记录',
    jobDescription: '岗位职责：\n1. 负责化学发光产品中间品、半成品、成品、留样稳定性检验的日常检验工作；\n2. 负责新产品注册报批、体系核查质检相关工作；\n3. 负责免疫分析仪使用，维护和保养，及时填写相关记录；\n4. 负责室间质评、售后反馈问题的试剂检测和评估；\n5. 负责实验室日常样本、质控盘、标准品、校准、质控品及相关记录的填写；\n6. 及时完成上级领导安排的其它任务。\n\n岗位技能要求：\n1. 医学检验、生物学、制药类、医疗器械类等专业，本科以上学历；\n2. 会操作主流免疫分析仪，熟练掌握化学发光产品质检流程，熟悉新项目注册报批质检相关工作；\n3. 有较强的责任心、良好的学习能力与独立思考的能力、良好的团队合作意识，善于沟通；\n4. 从事质检工作1年或以上。'
  }
]

// 所有岗位分类
export const jobCategories: JobCategory[] = [
  { id: 'medical', labelKey: 'job.categories.medical', presets: medicalJobs },
  { id: 'tech', labelKey: 'job.categories.tech', presets: techJobs },
  { id: 'product', labelKey: 'job.categories.product', presets: productJobs },
  { id: 'operation', labelKey: 'job.categories.operation', presets: operationJobs },
  { id: 'design', labelKey: 'job.categories.design', presets: designJobs },
  { id: 'sales', labelKey: 'job.categories.sales', presets: salesJobs },
  { id: 'hr', labelKey: 'job.categories.hr', presets: hrJobs },
  { id: 'finance', labelKey: 'job.categories.finance', presets: financeJobs }
]

const CUSTOM_PRESETS_KEY = 'goresume_custom_job_presets'

// 获取用户自定义岗位模板
export function getCustomJobPresets(): JobPreset[] {
  try {
    const raw = localStorage.getItem(CUSTOM_PRESETS_KEY)
    if (!raw) return []
    return JSON.parse(raw) || []
  } catch {
    return []
  }
}

// 保存用户自定义岗位模板
export function saveCustomJobPreset(preset: JobPreset): void {
  const list = getCustomJobPresets().filter(p => p.id !== preset.id)
  preset.isCustom = true
  list.unshift(preset)
  localStorage.setItem(CUSTOM_PRESETS_KEY, JSON.stringify(list))
}

// 删除用户自定义岗位模板
export function deleteCustomJobPreset(presetId: string): void {
  const list = getCustomJobPresets().filter(p => p.id !== presetId)
  localStorage.setItem(CUSTOM_PRESETS_KEY, JSON.stringify(list))
}

// 获取所有岗位（包含系统预设 + 自定义）
export function getAllPresets(): JobPreset[] {
  return [...getCustomJobPresets(), ...jobCategories.flatMap(cat => cat.presets)]
}

// 根据ID获取岗位
export function getPresetById(id: string): JobPreset | undefined {
  return getAllPresets().find(p => p.id === id)
}

// 根据分类获取岗位
export function getPresetsByCategory(category: string): JobPreset[] {
  if (category === 'custom') {
    return getCustomJobPresets()
  }
  const cat = jobCategories.find(c => c.id === category)
  return cat?.presets || []
}
