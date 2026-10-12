# TalentLens 功能检查

先阅读 CONTEXT.md 和 TESTING.md，保留工作区已有修改。

- `npm test` 执行 Node 自动化逻辑测试；不要把 scripts/test_greet.js 当作离线测试运行，它会操作真实招聘平台。
- `npm run test:go` 执行后端测试。
- `npm run test:e2e` 构建并运行带真实 Go/Wails 后端的 Playwright 测试。
- 测试环境地址为 http://127.0.0.1:34116，数据目录必须位于本项目 .e2e-data/run-*。
- 通过 talentlens-playwright MCP 探索测试环境。需要手动启动环境时先执行 `npm run test:e2e:prepare`，再执行 `node scripts/e2e-server.js`。
- 使用虚构简历与本地模拟 AI 服务。未经用户明确指示，不在检查中发起真实招聘沟通、发布职位、使用点数或发送真实候选人资料给模型。
- 发现缺陷后先保留失败证据，再修复源码并重跑相关测试。不得通过删除断言、跳过失败用例或用模拟业务结果替代真实后端来宣称修复成功。
- 报告必须区分已验证、模拟外部依赖、未覆盖的原生桌面功能和真实平台操作。
