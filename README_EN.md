<p align="center">
  <h1 align="center">TalentLens</h1>
  <p align="center">AI-Powered Resume Screening Tool for HR</p>
</p>

<p align="center">
  English | <a href="README.md">中文</a>
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

## What is TalentLens?

**TalentLens** is a lightweight, elegant desktop application that helps HR professionals screen resumes efficiently using AI. Simply drag-and-drop resumes, and TalentLens will analyze, score, and rank candidates based on your job requirements.

Available on **Windows** and **macOS**.

---

## Features

| Feature | Description |
|---------|-------------|
| **Drag & Drop** | Upload resumes in PDF / Word / Image formats |
| **AI Analysis** | Deep analysis via OpenAI-compatible APIs |
| **Multi-dimensional Scoring** | Skills match, experience match, education match |
| **Recommendations** | AI provides detailed recommendation with reasoning |
| **Job Templates** | Built-in templates for common job positions |
| **Provider Presets** | Pre-configured for DeepSeek / OpenAI / Zhipu / Moonshot / SiliconFlow |
| **i18n** | Simplified Chinese / Traditional Chinese / English |
| **macOS-style UI** | Frosted glass titlebar, traffic light buttons, polished interface |
| **Real-time Progress** | Live visualization of analysis progress |
| **Dev Console** | Built-in developer debug panel (F12) |

---

## 🌟 Core Improvements & Key Features

Compared to the baseline version, this project has undergone extensive enterprise-grade feature enhancements:

### 1. 🎯 AI-Powered Interview Assistant (5 Targeted Interview Questions)
- **Dynamic Questioning**: Generates **5 tailored interview questions** based on candidate's real project experience, skill gaps, and strengths;
- **Reference Answers & Assessment Points**: Provides standard reference answers and HR evaluation criteria for each question.

### 2. 🧬 Biomedical & IVD (In Vitro Diagnostics) Job Library
- Added dedicated **【Biomedical / IVD】** category with 13 real-world IVD position presets (`Clinical Project Manager`, `SCRA`, `Autoimmune R&D Manager`, `Product Application Specialist`, `IVD Product Manager`, `Field Service Engineer`, `QC Inspector`, etc.);
- Preloaded with authentic job descriptions, qualifications, and core skill sets.

### 3. 📊 Main Screen Job Requirements Dashboard
- Embedded job requirements and template selector directly on the top of the main screen;
- Quick-switch between 13 IVD presets or custom roles, with expandable inline editing for skill tags and detailed responsibilities.

### 4. 💾 Custom Job Template Storage & Management
- Save any configured job setup as a new template with one click (**【💾 Save as New Template】**);
- Dedicated **【Custom Templates】** tab with active count badge and delete management.

### 5. 📝 Deep Job Description Injection in AI Prompts
- Added multi-line job description module (up to 3,000 characters);
- Injected as core evaluation criteria in backend Go prompts to guarantee deep business alignment.

### 6. 🏢 Comprehensive Recruitment Project Management
- Rebuilt **【+ New Project】** workflow to include **Project Theme**, **Automatic Job Inheritance**, **Department**, **Target Headcount**, **Recruiter / HR**, and **Project Notes**;
- Project cards upgraded to display department/job badges, headcount, recruiters, and real-time score statistics.

### 7. ⚡ Custom Model Name Input & Latest LLM Support
- Allows direct text input for custom model names;
- Support for streaming models and latest provider integrations.

---

## Quick Start

### Download

Download the latest release from the [Releases](https://github.com/xunyinjilove/TalentLens/releases) page:

- **Windows**: `TalentLens-windows-amd64.exe`
- **macOS (Intel)**: `TalentLens-macos-amd64.zip`
- **macOS (Apple Silicon)**: `TalentLens-macos-arm64.zip`

Just download and run. No installation required.

### First-time Setup

1. Open the app and click the **gear icon** in the top-right to open Settings
2. Select an AI provider (DeepSeek recommended - free quota for new users)
3. Follow the guide to obtain and enter your API Key
4. Configure job requirements (use built-in templates for quick setup)
5. Return to the home page, drag in resumes and start analyzing

---

## Supported AI Providers

| Provider | Recommended Model | Notes |
|----------|------------------|-------|
| DeepSeek | DeepSeek-V3.2 | Recommended, excellent cost-performance ratio |
| OpenAI | GPT-5 Mini | Latest GPT-5 family |
| Zhipu AI | GLM-4.7-Flash | Chinese LLM, free tier available |
| Moonshot (Kimi) | Kimi K2.5 | Latest multimodal agent model |
| SiliconFlow | DeepSeek-V3.2 | Model aggregator, 100+ models |
| Custom | - | Any OpenAI-compatible API |

---

## Build from Source

### Prerequisites

- [Go](https://go.dev/) >= 1.21
- [Node.js](https://nodejs.org/) >= 18
- [Wails CLI](https://wails.io/) v2

### Build

```bash
# Clone the repository
git clone https://github.com/xunyinjilove/TalentLens.git
cd TalentLens

# Install frontend dependencies
cd frontend && npm install && cd ..

# Development mode (hot reload)
wails dev

# Production build
wails build

# Cross-platform builds
wails build -platform darwin/arm64   # macOS Apple Silicon
wails build -platform darwin/amd64   # macOS Intel
wails build -platform windows/amd64  # Windows
```

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23, Wails v2 |
| Frontend | Vue 3, TypeScript, Vite |
| UI Library | Element Plus |
| State Management | Pinia |
| i18n | vue-i18n |
| Styling | SCSS, macOS Design System |

---

## Project Structure

```
TalentLens/
├── app.go                 # Go backend (AI client, file processing, API)
├── wails.json             # Wails configuration
├── frontend/
│   ├── src/
│   │   ├── views/         # Page views (HomeView, SettingsView)
│   │   ├── components/    # Components (TitleBar, ResumeCard, DropZone...)
│   │   ├── composables/   # Pinia stores
│   │   ├── locales/       # i18n translations
│   │   ├── styles/        # macOS theme variables
│   │   └── data/          # Job presets, provider configs
│   └── index.html
├── build/                 # Build assets & installers
├── docs/                  # Documentation
├── .github/workflows/     # CI/CD auto-build
├── CHANGELOG.md           # Version changelog
└── CONTRIBUTING.md        # Contribution guide
```

---

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines and submission process.

---

## License

This project is licensed under the **GNU General Public License v3.0** - see the [LICENSE](LICENSE) file for details.

