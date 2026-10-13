# 招聘工作台实施交接（2026-10-13）

对应需求和验收口径见 [recruitment-workbench-framework.md](recruitment-workbench-framework.md) 与仓库根目录 `CONTEXT.md`。本文件记录已落地的代码和仍需实测的边界，供后续实施模型继续。

## 已落地

- SQLite schema、旧项目与简历迁移、原 JSON 备份和数量校验。迁移失败时旧文件不变，旧界面仍可读取。工作台从 SQLite 取数；原项目详情与评测流程仍沿用 JSON 并同步写入 SQLite。
- 检索任务记录、重启后“任务已中断”、右侧常驻入口、缩小不停止、逐份入库即进入限并发 AI 队列。继续操作只搜索剩余目标并排除已有独立主页；旧任务保留历史进度，新批次另有任务 ID。
- 岗位模板和 AI 规则建议先存草稿，HR 逐条编辑、批准或拒绝后形成版本。旧核验结果与规则来源记录保留。排名按能力分的 5 分档优先、同档按确认加分项排序。
- 统一候选人档案、平台身份去重、已核实联系方式去重、同名疑似重复提示及 HR 确认合并。跨项目的评测和阶段独立，来源、简历版本与跟进时间线汇总展示。
- 首版招聘跟进阶段、负责人、备注、下一步日期、沟通记录、本地面试日程。五个工作台页面使用本地项目和 SQLite 数据，参考设计稿的布局、卡片、Indigo/Violet 色彩与明暗主题，并保留透景 / Tellgen 品牌。
- 51job 主动投递的统一导入契约、平台职位 ID 映射、未匹配投递待分配、增量幂等和浏览器只读适配器。适配器只接受可核实的申请 ID 与候选人 ID；缺少完整简历时标为待补全，不把列表摘要冒充完整简历。
- 项目前十名的排名快照、逐人生成和编辑招呼语、红线待核实逐人确认、一次确认批次及逐人回执登记。当前平台首次定制发送未有可验证的能力和回执时，状态明确为“需人工处理”，不记假成功。

## 已验证

- `go test ./...`：通过，包含迁移回滚、重启中断、规则审批、可信身份合并、投递幂等、批次确认等用例。
- `npm test`：52 项通过。
- `frontend/npx tsc --noEmit`：通过。
- `wails build -m -nosyncgomod -nocolour`：生成 `build/bin/TalentLens.exe`；构建过程会同步 Node 平台适配器到 `build/bin/scripts`。
- 浅色和深色工作台空数据状态截图保存在 `build/workbench-light.png` 与 `build/workbench-dark.png`。浏览器预览不读取桌面端数据，会显示明确提示。

## 尚需完成的真实平台和存储收尾

1. **51job 主动投递页面实测**：当前企业端浏览器打开的是职位页面，不是主动投递列表。适配器的保守选择器尚未在真实投递页面验证；若平台不暴露申请 ID、候选人 ID 或职位 ID，应基于脱敏 DOM / 网络响应 fixture 更新适配器，不能用姓名或职位名称猜测。运行 `node scripts/51job_application_import.js` 可先得到可操作状态。
2. **逐人定制自动发送**：现有 51job 首次 Hi聊使用平台默认招呼语；其他平台的旧通用动作只验证按钮点击，不能作为发送回执。需要针对每个平台实现稳定目标、可编辑文字、费用或职位确认与新发消息核验，才可把批次项从“需人工处理”接入自动发送。不得把默认招呼语或打开聊天窗标成定制发送。
3. **SQLite 主数据切换**：新工作台以 SQLite 查询，但旧项目与评测 API 仍写 JSON 再镜像到 SQLite。要完成框架第 1 阶段的最终口径，需将旧 API 的读写改为 SQLite 事务，并保留旧 JSON 只读回退和升级失败恢复。尤其要处理删除项目／简历、跨项目重评和同步失败的可见错误。
4. **扩展平台**：BOSS、智联、猎聘主动投递继续复用 `ApplicationRecord` 契约实现适配器，保留平台职位 ID 与账号作用域。
5. **实际端到端验证**：在已登录招聘平台、AI 配置和有代表性旧数据的隔离副本上走完框架第 8 节场景；勿直接拿用户真实数据做破坏性测试。

## 继续实施时的入口

- Go 存储与迁移：`recruitment_store.go`；检索队列：`recruitment_tasks.go`；规则：`recruitment_rules.go`；身份：`recruitment_identity.go`；导入：`recruitment_import.go`；沟通：`recruitment_outreach.go`。
- 51job 只读导入适配器：`scripts/51job_application_import.js`；运行时资源同步：`scripts/sync-runtime-assets.js`。
- Vue 工作台：`frontend/src/views/Recruitment*View.vue`、`TalentPoolView.vue`，以及 `frontend/src/components/SourcingTaskCenter.vue`、`JobRulesDialog.vue`、`TopTenOutreachDialog.vue`。
