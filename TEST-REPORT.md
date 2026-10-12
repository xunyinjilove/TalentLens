# TalentLens 测试报告

日期：2026-10-12。Windows；Go 1.27.0；Node.js 24.18.1；Wails 2.15.0；Playwright 1.64.0。

## 结果

| 检查 | 结果 |
| --- | --- |
| Go 后端测试 | 30 通过，0 失败 |
| Node 招聘自动化逻辑测试 | 47 通过，0 失败 |
| Vue/Vite 前端生产构建 | 通过；存在 Sass 弃用和大包体积警告 |
| Playwright 浏览器功能测试 | 4 通过，1 失败，未跳过用例 |
| Cline 安装 | 官方 VS Code 扩展 4.1.23 安装成功 |
| Cline → Playwright MCP | 25 个工具可发现，独立浏览器导航验证通过 |
| Cline 模型执行 | 尚未配置模型，未执行 Cline AI 任务 |

## 已修复

### 1. 项目详情刷新白屏

复现：创建项目、进入详情、刷新，HTTP 请求 `/project/项目ID` 返回 404，页面空白。

原因：Vue 使用 history 路由，但 Wails 本地资源服务没有该路径的 SPA 回退。

修复：`frontend/src/router/index.ts` 改用哈希路由。回归中创建、继承岗位、刷新读取和删除后的持久化断言均通过；此用例最终仍因下述独立框架 JS 异常失败。

### 2. DOCX 简历正文被解析成 ZIP 乱码

复现：导入包含中文正文的 DOCX，GetProjectResumes 返回的 content 以 `PK` 压缩包标记开头，AI 请求未包含可读的简历正文。

原因：DOCX 进入普通二进制转字符串分支，没有解压 `word/document.xml`。

修复：新增 `document_text.go`，解压并解析正文 XML，保留文本分段、表格单元格内段落、制表符和换行。限制解压正文大小；损坏文件不再返回压缩包/XML 原始内容。

验证：新增 2 项 Go 回归测试，修复前均失败、修复后通过；浏览器中真实 DOCX 导入、传入模拟 AI、显示 75 分、红线仍待 HR 核实、刷新持久化的完整流程通过。

## 浏览器用例

| 用例 | 结果 |
| --- | --- |
| DOCX 导入 → 本地模拟 AI → 显示结果 → 刷新保存结果 → 红线仍待核实 | 通过 |
| AI 服务返回 503 → 错误显示 → 恢复服务 → 单份简历重试 | 通过 |
| 未配置 AI → 配置引导 → 不发送请求 | 通过 |
| 岗位弹窗 Escape 关闭、空白项目名禁止提交 | 通过 |
| 新建项目继承岗位、刷新、删除、无未处理 JS 异常 | 失败：业务断言通过，但框架调试层抛出异常 |

## 未解决：Wails 浏览器开发调试层异常

错误：`Cannot read properties of null (reading 'nodes')`。

堆栈：`/wails/ipc.js:32` 的 `Fe` / `bn`，随后 `/wails/ipc.js:41` 的 `new In`。

已定位到本机 Wails 2.15.0 的 `internal/frontend/runtime/ipc_websocket.js`：在 DOMContentLoaded 中构造 Svelte 开发 overlay 时发生异常。同时后端记录 `Unknown message from front end: runtime:ready`。

这些是开发浏览器运行时问题；尚未通过升级/替换 Wails 运行时验证修复，也不能据此认定正式打包桌面版存在同样故障。保留测试失败，不过滤该异常、不修改 Go 模块缓存。

## 测试限制

本轮使用真实应用前后端与隔离数据，但外部 AI 为固定响应的本地模拟服务。未调用真实 AI、招聘平台，未操作真实候选人，未测试原生拖拽/文件对话框、发布版安装、旧 DOC/图片 OCR。没有生成新发布版 EXE；修复位于源码和测试构建中。

现有未提交修改被保留；本轮新增依赖、测试设施、路由修复和 DOCX 解析修复，没有提交或推送 Git。

运行方法见 TESTING.md。完整可视化结果：`playwright-report/index.html`；原始输出：`e2e-check.log`、`go-test-results.log`。
