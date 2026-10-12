# TalentLens 测试说明

## 已安装

- VS Code 官方 Cline 扩展：`saoudrizwan.claude-dev`，版本 4.1.23。
- 本项目开发依赖：`@playwright/test` 1.64.0、`@playwright/mcp` 0.0.83。
- Chromium 浏览器：`D:\HR\agent-resources\playwright-browsers`。
- Cline MCP 服务名：`talentlens-playwright`。本机配置位于 `C:\Users\Tellgen\.cline\data\settings\cline_mcp_settings.json`。
- MCP 初始化、工具列表（25 项）、浏览器启动与空白页导航已验证。

Cline 模型尚未配置。本轮测试由当前助手直接运行，不是由已登录的 Cline 模型执行。请在 VS Code 打开本项目，在 Cline 中选择模型服务并自行完成登录/API Key 配置；不要将密钥写进仓库。

## 运行

在 PowerShell 中进入本项目，按顺序执行，不要在前端构建清理 dist 的同时运行 Go 测试：

```powershell
cd D:\HR\TalentLens-main
npm run test:e2e
npm run test:go
npm test
```

`test:e2e` 自动构建前端和专用 Go 测试可执行文件，再启动 Wails 后端并运行 Playwright。当前浏览器测试仍有一项因 Wails 开发调试层异常失败，详见 TEST-REPORT.md。

依赖 Go、Node.js、Windows WebView2，以及已有 Wails 生成绑定。全新检出项目时，先按 README 的 Wails 构建步骤生成 `frontend/wailsjs`。在另一台机器上需安装 npm 依赖和 `npx playwright install chromium`；配置会优先使用环境变量指定的浏览器路径，其次使用本机安装目录（存在时），否则使用 Playwright 标准缓存。

报告命令：

```powershell
npm run test:e2e:report
```

报告位于 `playwright-report/index.html`；失败截图、调用轨迹和上下文位于 `test-results`。这两个目录已忽略，不提交 Git。

## Cline 探索检查

模型配置完成后，Cline 可读取 `.clinerules/testing.md`。可给它这样的任务：

> 阅读 TESTING.md 和 TEST-REPORT.md，使用现有隔离测试环境检查 TalentLens。先运行测试，复现失败；修复业务缺陷后重跑相关用例，保留失败证据。不要删除失败断言或跳过测试。区分 Wails 框架开发工具错误与业务功能错误。只使用虚构候选人和本地模拟 AI 服务。

需要让 Cline 通过 MCP 手动探索页面时，先运行 `npm run test:e2e:prepare`，再运行 `node scripts/e2e-server.js`。浏览器入口是 `http://127.0.0.1:34116`。结束后用 Ctrl+C 关闭服务。自动化测试要求独占此端口，不能同时启动手动环境。

## 覆盖范围与隔离

- 后端使用真实 App、Wails 绑定、文件解析及磁盘保存逻辑。
- 每次运行创建独立 `.e2e-data/run-*`，不使用用户的 Documents/TalentLens 数据。
- 浏览器使用独立上下文；原生 WebView2 同样使用测试目录中的配置。
- DOCX 简历为程序生成的虚构内容。AI 服务是本地 HTTP 模拟服务，验证请求、返回解析、评分、事件更新和持久化，不衡量真实大模型的分析质量。
- 招聘平台自动化由现有 Node 测试验证逻辑；没有调用真实平台、发消息、发布职位或消耗点数。
- 原生文件选择框、拖拽、打包程序安装、真实平台页面兼容性、旧版 DOC 和图片 OCR 尚未做端到端验证。

测试数据按次保留在 `.e2e-data` 供排查；Go 测试可执行文件位于 `bin`。它们不会进入生产构建或 Git 提交。
