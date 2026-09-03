<p align="center">
  <h1 align="center">TalentLens</h1>
  <p align="center">AI 驱动的智能简历筛选工具，专为 HR 设计</p>
</p>

<p align="center">
  <a href="README_EN.md">English</a> | 中文
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
| **拖拽上传** | 支持 PDF / Word / 图片格式简历 |
| **AI 智能分析** | 基于 OpenAI 兼容接口，多维度深度分析 |
| **综合评分** | 技能匹配度、经验匹配度、学历匹配度 |
| **推荐结论** | AI 给出推荐/不推荐结论及详细理由 |
| **岗位模板** | 内置多种岗位模板，一键配置需求 |
| **多服务商** | 预设 DeepSeek / OpenAI / 智谱 / Moonshot / SiliconFlow |
| **国际化** | 支持简体中文 / 繁体中文 / English |
| **macOS 风格 UI** | 毛玻璃标题栏、交通灯按钮、精致界面 |
| **实时进度** | 分析过程实时可视化进度显示 |
| **开发者工具** | 内置调试面板 (F12) |

---

## 🌟 核心升级与特色功能

相比初始版本，本项目进行了深度的业务级与技术级重构升级：

### 1. 🎯 深度 AI 面试辅助系统（5 大针对性面试提问）
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

- **Windows**: `TalentLens-windows-amd64.exe`
- **macOS (Intel)**: `TalentLens-macos-amd64.zip`
- **macOS (Apple Silicon)**: `TalentLens-macos-arm64.zip`

下载后双击运行即可，无需安装。

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

# 开发模式 (热重载)
wails dev

# 生产构建
wails build

# 跨平台构建
wails build -platform darwin/arm64   # macOS Apple Silicon
wails build -platform darwin/amd64   # macOS Intel
wails build -platform windows/amd64  # Windows
```

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
├── app.go                 # Go 后端 (AI 客户端、文件处理、API)
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

---

## 参与贡献

欢迎参与贡献！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解开发规范和提交流程。

---

## 许可证

本项目采用 **GNU General Public License v3.0** 开源协议，详见 [LICENSE](LICENSE)。

