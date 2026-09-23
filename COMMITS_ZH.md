# TalentLens Git 提交历史与版本变更记录 (Commit History & Changelog)

本文档整理了 **TalentLens** 项目自创建以来的全量 Git 提交记录（共 28 次及最新提交），附带完整的中文对照翻译、提交哈希（Commit Hash）、日期以及具体更新功能说明。

---

## 📌 快速导航对照表

| 序号 | 提交哈希 | 提交日期 | 类别 | 中文提交说明 | 原始英文提交信息 (Original Message) |
| :---: | :---: | :---: | :---: | :--- | :--- |
| **01** | `b9451ed` | 2026-09-22 | 功能 (feat) | **融合 GoodHR 架构设计模式**：跨 Frame 穿透提取候选人卡片、平滑微步滚轮防风控、拟人打字随机延迟、Esc 键快捷关闭弹窗 | `feat: integrate GoodHR architectural patterns (cross-frame extraction, smooth scroll, human typing, escape close)` |
| **02** | `f819a63` | 2026-09-22 | 修复 (fix) | **彻底消除 about:blank 空白标签页**：移除被 Chromium 废弃的 AutomationControlled 启动参数，修正首标签复用逻辑 | `fix: eliminate about:blank tab and remove deprecated AutomationControlled CLI flag` |
| **03** | `9e84cc5` | 2026-09-22 | 功能 (feat) | **移除候选人详情页的“平台反查”按钮**：简化候选人信息展示与操作流程 | `feat: remove 平台反查 button and related function from candidate detail view` |
| **04** | `60a8e29` | 2026-09-22 | 功能 (feat) | **四平台并发调度开启**：改为通过 Promise.all 并行启动，支持 BOSS、智联、51job、猎聘 4 个浏览器窗口同时拉起 | `feat: run multi-platform scrapers concurrently via Promise.all so all 4 platforms open simultaneously` |
| **05** | `166ffe0` | 2026-09-22 | 修复 (fix) | **修复脚本语法错误并重新编译**：解决 multi_platform_agent.js 中的闭合大括号语法错误，更新 build 产物 | `fix: resolve syntax error in multi_platform_agent.js closing brace and rebuild` |
| **06** | `a909feb` | 2026-09-22 | 功能 (feat) | **测试数据适配**：向所有生成与存储的候选人注入模拟邮箱 `qn3366271573@163.com`；移除冗余的发送录用 Offer 功能 | `feat: inject qn3366271573@163.com into all generated and stored resumes for simulation test; remove send offer feature` |
| **07** | `9a33de5` | 2026-09-22 | 功能 (feat) | **邮件发信集成与 BOSS 链接修正**：新增 163 邮箱 SMTP 自动发信服务，修复已废弃的 BOSS 网址入口 | `feat: add Offer letter email sending feature via 163 SMTP, attach candidate email qn3366271573@163.com, fix deprecated BOSS URL` |
| **08** | `0cb7e78` | 2026-09-22 | 修复 (fix) | **猎聘反爬探针拦截**：拦截猎聘 `security.min.js` 反爬脚本，消除 about:blank 重定向死循环 | `fix: block Liepin security.min.js anti-bot script and neutralize about:blank redirect loop` |
| **09** | `c426346` | 2026-09-22 | 修复 (fix) | **浏览器启动重置修复**：解决 Edge 启动时页面重置为空白页的问题，新增标签页智能探测与自动恢复机制 | `fix: resolve about:blank reset issue in browser launch - add tab smart detection and auto-recovery` |
| **10** | `ae99e11` | 2026-09-22 | 修复 (fix) | **浏览器 Profile 冲突与进程管理优化**：解决配置锁冲突导致任务直接跳过完成的 Bug；实现三模弹性启动机制、分平台独立调试端口与残留进程清理 | `fix: resolve browser profile lock conflict causing skip-to-done bug. Add 3-mode launch (CDP reuse, kill+Puppeteer, kill+CDP spawn), per-platform debug ports, stale process cleanup, and proper error-vs-done event distinction` |
| **11** | `fe31fcf` | 2026-09-22 | 修复 (fix) | **登录态严格检测**：强化页面内真实 DOM 登录态检测，防止用户尚未完成扫码就过早触发抓取 | `fix: enforce strict in-page DOM login detection to prevent premature scraping` |
| **12** | `c120b3b` | 2026-09-21 | 修复 (fix) | **修复 Edge 启动异常退出 (Code 0)**：修正猎聘企业端入口地址并解决部分 Windows 环境下浏览器启动崩溃问题 | `fix: resolve Edge Code 0 launch crash and correct Liepin enterprise URL` |
| **13** | `e838c3b` | 2026-09-21 | 版本 (v1.6.0) | **4 合 1 矩阵寻才控制面板**：全量上线 BOSS、智联招聘、前程无忧、猎聘四大招聘平台多选并发抓取与矩阵看板 | `feat(v1.6.0): implement 4-in-1 multi-platform sourcing (BOSS, Zhaopin, 51job, Liepin) with matrix dashboard` |
| **14** | `497a57f` | 2026-09-21 | 功能 (feat) | **纯真实数据直连**：彻底剥离任何 mock 模拟假数据，全面切换为 BOSS 直聘企业后台纯真实直连与交互 | `feat: switch to pure real BOSS enterprise direct connection without mock data` |
| **15** | `2a545cc` | 2026-09-21 | 功能 (feat) | **双模人才检索支持**：实现公开免登录检索（外网公开库）与 BOSS 企业端直连检索（深度匹配）双通道运作 | `feat: implement dual-mode candidate retrieval (open public no-login & BOSS direct connect)` |
| **16** | `2e8a1cb` | 2026-09-21 | 功能 (feat) | **双源简历交叉核验与演进分析**：实现前端微简历与后端完整附件简历的内容交叉比对、时间线演进分析与真伪核查 | `feat: implement dual-source resume cross-validation and evolution analysis (micro-resume + full attachment)` |
| **17** | `09c4954` | 2026-09-21 | 功能 (feat) | **全自动化 BOSS 交互动作**：集成批量打招呼、索要完整简历、交换微信/手机号、标记不合适等全流程候选人管理 | `feat: integrated full BOSS automation actions (greet, ask resume, exchange wechat, mark unfit) and candidate workflow` |
| **18** | `84120b0` | 2026-09-21 | 修复 (fix) | **登录流程直连优化**：优化浏览器首开路径，直连平台扫码登录页，防止出现 about:blank 假死 | `fix: launch directly into login page and prevent about:blank state` |
| **19** | `7c4c934` | 2026-09-21 | 修复 (fix) | **常驻登录窗口保持**：保持登录浏览器窗口持续开启，直至用户手机端确认扫码完成，避免提前关闭 | `fix: keep login browser window open until user completes login` |
| **20** | `805532d` | 2026-09-21 | 修复 (fix) | **Edge Code 0 错误专修与登录测试流**：适配 Windows 平台 Edge 进程管理，新增独立的登录连接测试链路 | `fix: resolve Windows Edge Code 0 launch error and add dedicated login test flow` |
| **21** | `2485c24` | 2026-09-21 | 功能 (feat) | **牛人与 Boss 双端身份识别**：支持对登录账号进行实时身份判定（求职端 Geek vs 招聘端 Boss），避免账号权限错误 | `feat: support Geek (job seeker) and Boss account login testing with live identity recognition` |
| **22** | `4f8cc8b` | 2026-09-03 | 修复 (fix) | **Go 原生内置搜索引擎**：在 Go 后端中内置底层搜索解析逻辑，保障程序零外部依赖稳定运行 | `fix: embed native BOSS search engine in Go to guarantee zero-dependency execution` |
| **23** | `5303eb5` | 2026-09-03 | 工程 (ci) | **CI 模板维护**：更新 Release 发布配置模板中的 GitHub 仓库地址链接 | `ci: update repository URL in release notes template` |
| **24** | `b0fae2f` | 2026-09-03 | 功能 (feat) | **内嵌 BOSS 直聘自动化人选检索与实时 AI 筛选**：结合大语言模型对抓取候选人执行全维度背景与岗位匹配评分 | `feat: integrate embedded BOSS Zhipin automated candidate sourcing with live AI screening` |
| **25** | `3ff3f8b` | 2026-09-03 | 文档 (docs) | **README 升级**：新增核心技术升级亮点、生物医药/IVD 岗位库与面试提问系统说明 | `docs: add core improvements and feature highlights section to README` |
| **26** | `706109e` | 2026-09-03 | 工程 (ci) | **精简 CI 工作流**：移除不再使用的 GitHub Pages 部署脚本 | `ci: remove github pages deployment workflow` |
| **27** | `5730a6e` | 2026-09-03 | 维护 (chore) | **品牌标识脱敏与更新**：移除第三方公司商标与信息，更新项目官方开源仓库地址 | `chore: remove third-party company branding and update repo links` |
| **28** | `b438728` | 2026-09-03 | 初始提交 (feat) | **TalentLens 项目初始化**：构建支持 IVD 专业岗位模板、多模型 AI 简历智能深度解析、评分与项目管理的桌面端系统 | `feat: initial commit for TalentLens with IVD job presets, AI analysis and project management` |

---

## 🔍 各阶段详细变更说明

### 一、GoodHR 工业级架构升级阶段 (`b9451ed`)
- **跨 Frame 候选人穿透提取 (`extractCandidatesAcrossFrames`)**：针对部分平台使用 `iframe`（如智联 `recommendFrame`）隔离卡片导致常规选择器失效的问题，引入层级 Frame 递归穿透机制，无论卡片处于主页面还是子 frame 内均可精准提取。
- **拟人微步平滑滚动 (`smoothScroll`)**：抛弃瞬时跳跃式滚动，改为模拟真实滚轮事件（deltaY 40~100px 随机微步，带 15~35ms 弹性时间间隔），有效应对各大招聘平台的反爬虫风控探针检测。
- **拟人输入打字 (`humanType`)**：模拟人类打字速度，单字符输入间隔随机波动（30~90ms），在搜索框和沟通打招呼中完全消除了自动化输入特征。
- **快捷键容错与弹窗清理**：每次候选人卡片交互后，自动向页面派发 `Escape` 键，秒级关闭可能弹出的遮罩层、推荐卡片或限流提醒。

### 二、浏览器多进程与防风控优化阶段 (`ae99e11` ~ `f819a63`)
- **消除 about:blank**：移除 Chromium 120+ 废弃且易触发异常重定向的 `--disable-blink-features=AutomationControlled`，优化页面启动逻辑，直接复用首个空白标签导航至目标地址。
- **三模弹性启动机制**：
  1. 首选复用已存在的高权限调试端口 CDP 会话；
  2. 其次采用独立 Profile 目录进行无锁 Puppeteer 启动；
  3. 兜底采用直接 spawn 子进程拉起浏览器。
- **四平台并发调度 (`60a8e29`)**：通过 `Promise.all` 将 BOSS直聘、智联招聘、前程无忧、猎聘网四方抓取任务改为全异步并行，使四家浏览器窗口同时弹出，大幅缩减总抓取耗时。

### 三、四合一矩阵寻才与真实企业后台直连 (`e838c3b` ~ `09c4954`)
- **去 Mock 化**：全面移除本地模拟人选数据，所有简历、微简历、在线状态均来自平台真实接口与企业后台 DOM。
- **候选人全流程动作链路**：打招呼、索要简历、互换微信、标为不合适，支持一键自动化批量触发。
- **双源简历一致性演进分析**：对比牛人在平台公开信息与完整 PDF 附件简历间的公司经历、任职时间线、工作产出，自动标记前后矛盾或造假存疑点。

### 四、初始化与生物医药/IVD 专业化定位 (`b438728` ~ `b0fae2f`)
- **多模型支持**：无缝对接 DeepSeek、OpenAI、智谱清言、Moonshot、SiliconFlow 等大语言模型。
- **专业 IVD 岗位预设**：内置自身免疫、化学发光、分子诊断、临床 SCRA 等体外诊断垂直行业全套岗位要求与技能画像。
