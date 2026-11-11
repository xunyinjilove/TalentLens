<p align="center">
  <h1 align="center">TalentLens</h1>
  <p align="center">AI 驱动的智能简历筛选工具，专为 HR 设计</p>
</p>

<p align="center">
  <a href="README_EN.md">English</a> | 中文 | <a href="COMMITS_ZH.md">📜 提交记录与中文对照</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js" alt="Vue 3">
  <img src="https://img.shields.io/badge/Wails-v2-red" alt="Wails">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue" alt="License">
  <img src="https://img.shields.io/badge/Windows-0078D6?logo=windows&logoColor=white" alt="Windows">
  <img src="https://img.shields.io/badge/macOS-000000?logo=apple&logoColor=white" alt="macOS">
</p>

---

## 简介

**TalentLens** 是一款轻量级桌面应用，帮助 HR 利用 AI 高效筛选简历。只需拖入简历文件，即可自动分析、打分、排序，根据岗位需求给出专业推荐建议。

支持 **Windows** 和 **macOS** 双平台。

---

## 功能特性

| 功能 | 说明 |
|------|------|
| **拖拽上传** | 支持 PDF / Word / 图片格式简历及前程无忧结构化微简历解析 |
| **AI 智能分析** | 基于 OpenAI 兼容接口，岗位红线硬性准入核验 + 多维度能力深度评估 |
| **模块化简历与就地翻译** | 工作经历与项目经验按 `#01`、`#02` 独立卡片拆解，每一段经历正下方内嵌中文对照译文；个人优势智能划分为【工作与专业能力】与【生活与个人特质】 |
| **人才管理与人才望远镜** | 51job 主动投递与平台推荐人才池硬性条件自动初筛（学历、经验、活跃度、年龄等）、合格自动打招呼入库、不合适自动婉拒与附件简历同步 |
| **面试安排与腾讯会议** | 内置面试日历看板，支持腾讯会议自动建会/改期/取消同步，并支持一键复制标准腾讯会议邀请全文（含主题、时间、链接、会议号） |
| **全链路招聘工作台** | 涵盖招聘概览、职位管理、简历人才库、候选人跟进漏斗、面试安排、Offer 管理完整闭环 |
| **岗位模板** | 内置生物医药 / IVD 及通用技术岗位模板，一键配置需求 |
| **发布招聘** | 招聘项目卡片提供四平台入口；51job 支持社招/校招、从平台全部职能逐级选到末级并选择对应关键词、保存草稿或立即发布 |
| **多服务商** | 预设 DeepSeek / OpenAI / 智谱 / Moonshot / SiliconFlow |
| **国际化** | 支持简体中文 / 繁体中文 / English |
| **现代 SaaS 工作台 UI** | 支持浅色 / 深色主题切换、无边框八方自由缩放与分辨率预设 |
| **实时进度** | 分析过程实时可视化进度显示 |
| **开发者工具** | 内置调试面板 (F12) |

---

## 🌟 核心升级与特色功能

相比初始版本，本项目进行了深度的业务级与技术级重构升级：

### 1. 🗓️ 面试安排工作台与腾讯会议自动化协同（v1.8.99 ~ v1.8.102）
- **腾讯会议自动建会与状态对账**：预约面试选择「腾讯会议自动生成」即可自动创建远端会议、同步改期与取消，并自动生成 `候选人－岗位－初面／复面` 标准会议主题；
- **一键复制完整腾讯会议邀请信息**：在面试日程卡片与编辑弹窗中点击「一键复制链接」，即可复制包含会议主题、GMT+08:00 标准时间区间、入会链接、`#腾讯会议：XXX-XXX-XXX` 会议号及手机入会指引的完整文本。

### 2. 🧩 结构化简历模块拆解与逐段就地内嵌翻译（v1.8.91 ~ v1.8.93）
- **工作经历与项目经历卡片化**：自动将每段工作经历、任职履历快照与项目经验拆解为 `#01`、`#02` 独立编号卡片，清晰分离公司、岗位、起止时间、行业规模标签与职责列表；
- **逐段就地中文译文对照**：点击「一键翻译为中文」后，在每一段工作经历、项目经验与个人优势子模块正下方直接内嵌展示对应的中文译文卡片（`中文译文 · #01`），无需跨屏对照；
- **个人优势双维提纯**：自动剥离平台界面噪音，将个人优势划分为 **【工作与专业能力】** 与 **【生活与个人特质】** 两大模块。

### 3. 🔭 人才管理与人才望远镜自动化初筛（v1.8.76 ~ v1.8.99）
- **自投简历硬性条件筛选**：针对前程无忧主动投递（人才管理）与平台推荐（人才望远镜），支持按最低学历、工作年限、年龄区间、院校/专业及活跃度执行三态裁决（合格 / 待核实 / 不合格）；
- **自动化动作闭环**：合格人选自动导入对应职位分库并同步真实附件简历，不合格人选支持一键或定时自动礼貌回绝。

### 4. 🎯 深度 AI 面试辅助系统（5 大针对性面试提问）
- **动态命题**：在简历解析与打分后，AI 会紧扣候选人的真实项目经历、核心技能掌握度及潜在薄弱项，自动生成 **5 个深度面试问题**；
- **参考回答与考察点**：针对每个提问，AI 提供基于该候选人简历背景的标准参考回答与 HR 考察关注点，大幅提升面试筛选效率。

### 2. 🧬 生物医药 / IVD（体外诊断）专业岗位库
- 新增独立的 **【生物医药 / IVD】** 分类标签，收录 13 个体外诊断企业的真实岗位标准预设（涵盖 `临床项目经理`、`国际销售经理`、`研发项目经理-自身免疫`、`临床SCRA`、`产品应用专员`、`IVD产品经理`、`器械工程师`、`QC质检` 等）；
- 各岗位均预设了行业真实的工作职责、任职资格与关键技能标签。

### 3. 📊 主界面常驻【岗位需求看板】与快捷模板
- 将岗位需求与模板切换直接移至软件首页顶部，用户打开即可一览当前岗位配置；
- 支持在首页一键切换 13 个 IVD 预设或技术类岗位，支持展开即时编辑技能标签与详细职责要求并实时同步。

### 4. 💾 自定义岗位模板保存与管理
- 支持在编辑任意岗位配置后一键 **【💾 存为新模板】**；
- 模板栏新增 **【自定义模板】** 分类页，带实时数量计数，支持随时一键套用或删除。

### 5. 📝 岗位职责与技能要求深度注入 Prompt
- 增加了独立的 **【岗位技能与职责要求】** 富文本/多行描述模块（支持最大 3000 字）；
- 后端在组装大模型 Prompt 时，将详细职责与技能要求作为核心基准注入，确保 AI 评估与出题深度对齐企业真实业务。

### 6. 🏢 招聘项目多维度业务管理体系
- 重构 **【+ 新建项目】** 对话框，移除重复的技能输入，升级为包含 **【项目名称/招聘主题】**、**【关联岗位自动继承】**、**【招聘部门】**、**【计划招聘人数】**、**【招聘负责人/HR】**、**【项目备注说明】** 的完整架构；
- 主界面项目卡片升级：清晰呈现部门与岗位标签、招聘人数、HR 负责人、简历分析进度及最高分徽章。

### 7. ⚡ 自由模型输入与最新大模型支持
- 支持模型名称直接自由打字输入，兼容任意自定义 OpenAI 格式模型；
- 新增主流模型流式分析与最新服务商预设。

---

## 快速开始

### 下载安装

从 [Releases](https://github.com/xunyinjilove/TalentLens/releases) 页面下载最新版本：

- **Windows**: 下载对应架构的 ZIP，完整解压后运行 `TalentLens.exe`；自动寻才及 51job 草稿功能需要本机安装 Node.js
- **macOS (Intel)**: `TalentLens-macos-amd64.zip`
- **macOS (Apple Silicon)**: `TalentLens-macos-arm64.zip`

macOS 解压后即可运行；Windows 请按上方说明保留 ZIP 中的 `scripts` 与 `node_modules` 目录。

### 首次配置

1. 打开应用，点击右上角 **齿轮图标** 进入设置
2. 选择 AI 服务商（推荐 DeepSeek，新用户有免费额度）
3. 按照引导获取并填入 API Key
4. 配置岗位需求（可使用内置模板）
5. 返回主页，拖入简历开始分析

---

## 支持的 AI 服务商

| 服务商 | 推荐模型 | 说明 |
|--------|---------|------|
| DeepSeek | DeepSeek-V3.2 | 推荐，性价比高，支持对话/深度推理双模式 |
| OpenAI | GPT-5 Mini | 全球最流行，GPT-5 系列最新 |
| 智谱 AI | GLM-4.7-Flash | 国产旗舰，免费调用 |
| Moonshot (Kimi) | Kimi K2.5 | 最新多模态智能体模型 |
| 硅基流动 | DeepSeek-V3.2 | 模型聚合平台，100+ 模型一站调用 |
| 自定义 | - | 任何 OpenAI 兼容接口 |

---

## 从源码构建

### 环境要求

- [Go](https://go.dev/) >= 1.21
- [Node.js](https://nodejs.org/) >= 18
- [Wails CLI](https://wails.io/) v2

### 构建步骤

```bash
# 克隆仓库
git clone https://github.com/xunyinjilove/TalentLens.git
cd TalentLens

# 安装前端依赖
cd frontend && npm install && cd ..

# 安装浏览器自动化依赖（51job 职位草稿和多平台寻才共用）
npm install

# 开发模式 (热重载)
wails dev

# 生产构建
wails build

# 跨平台构建
wails build -platform darwin/arm64   # macOS Apple Silicon
wails build -platform darwin/amd64   # macOS Intel
wails build -platform windows/amd64  # Windows
```

Windows 本地构建后，请将仓库的 `scripts` 和根目录 `node_modules` 复制到 `build/bin`，与 `TalentLens.exe` 同级；运行前安装 Node.js。正式 Windows ZIP 已包含这两个目录。

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.23, Wails v2 |
| 前端 | Vue 3, TypeScript, Vite |
| UI 组件 | Element Plus |
| 状态管理 | Pinia |
| 国际化 | vue-i18n |
| 样式 | SCSS, macOS 设计系统 |

---

## 项目结构

```
TalentLens/
├── go.mod                 # Go 模块与依赖
├── main.go                # Wails 启动入口，嵌入前端构建资源
├── backend/               # Go 业务代码及同包测试
│   ├── app.go             # 应用生命周期与前后端绑定
│   └── *_test.go          # 后端测试
├── wails.json             # Wails 项目配置
├── frontend/
│   ├── src/
│   │   ├── views/         # 页面视图 (HomeView, SettingsView)
│   │   ├── components/    # 组件 (TitleBar, ResumeCard, DropZone...)
│   │   ├── composables/   # Pinia Store
│   │   ├── locales/       # 国际化翻译文件
│   │   ├── styles/        # macOS 主题变量
│   │   └── data/          # 岗位模板、服务商配置
│   └── index.html
├── build/                 # 构建资源和安装脚本
├── docs/                  # 项目文档
├── .github/workflows/     # CI/CD 自动构建
├── CHANGELOG.md           # 版本变更日志
└── CONTRIBUTING.md        # 贡献指南
```

Wails 构建仍从仓库根目录执行。根目录的 `main.go` 负责嵌入 `frontend/dist` 并启动 `backend.Run`；Go 业务实现和测试统一放在 `backend/`，前端调用由 Wails 生成到 `frontend/wailsjs/go/backend/`。

---

## 参与贡献

欢迎参与贡献！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解开发规范和提交流程。

---

## 许可证

本项目采用 **GNU General Public License v3.0** 开源协议，详见 [LICENSE](LICENSE)。
