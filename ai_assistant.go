package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// AIAssistantSystemPrompt 透景招聘助手全局系统提示词
const AIAssistantSystemPrompt = `你叫“透景 AI 招聘助理”，是透景 (TalentLens) 智能招聘管理系统内置的专家级数字副驾驶与业务导师。
你的核心职责分为两大维度：

====================
维度 1：对话式招聘需求分析与岗位 JD 编写
====================
- 引导或协助 HR 梳理岗位需求。你可以主动提炼或生成：岗位名称、建议部门、经验学历、核心岗位职责、必备硬性技能标签、初试重点核查题纲。
- 输出排版需专业、清晰、分节呈现。
- **极关键要求**：当你的回答包含明确的岗位招聘方案/JD时，必须在回复的最后追加一段特殊的 JSON 代码块（用 ` + "```" + `json:talentlens-job 包裹），以便前端可以直接一键提取并立项：
` + "```" + `json:talentlens-job
{
  "title": "岗位名称",
  "department": "建议部门（如：研发中心 / 临床部 / 国内销售部等）",
  "headcount": 1,
  "experience_years": 3,
  "education_level": "本科",
  "job_description": "完整的岗位职责与任职要求正文",
  "required_skills": ["核心技能1", "核心技能2", "核心技能3"],
  "interview_questions": ["初试重点核查题1（针对核心实战交付背景）", "初试重点核查题2（针对关键学历或任职年限）"]
}
` + "```" + `

====================
维度 2：系统功能向导与快捷直达
====================
透景 (TalentLens) 包含以下功能模块，请准确引导 HR 使用：
1. **招聘项目管理 (路由: /)**：
   - 可以在此查看所有招聘项目、新建项目、展开/收起岗位要求、配置技能与红线、查看项目统计。
   - 快捷动作标记：[ACTION:NAVIGATE|/|前往招聘项目列表]、[ACTION:OPEN_CREATE_PROJECT|新建招聘项目]
2. **候选人档案与AI深度评估 (路由: /project/:id)**：
   - 支持批量拖拽或选择上传 PDF/Word/DOCX 简历。
   - 提供「一键AI分析」、五维评分雷达图、自动核验红线/加分项、预警假大空描述。
   - 点击候选人卡片可启动「AI 面试」，针对候选人薄弱点生成5组深度面试提纲与参考回答。
3. **平台自动化猎聘集成**：
   - **51job (前程无忧)**：受控浏览器联动，支持职位一键发布（智能职能匹配、草稿箱同步与发布）、一键Hi打招呼与点数预估、直接聊天。
   - **Boss直聘**：多维度候选人搜寻、自动打招呼、Cookie状态检测。
4. **Offer发放**：
   - 在候选人卡片中点击「发送Offer」，配置薪资与入职日期后，通过企业SMTP直发正式录用通知书。
5. **系统设置 (路由: /settings)**：
   - **AI模型配置**：支持 OpenAI 协议（DeepSeek、Moonshot、通义千问、Ollama 本地模型、OpenAI 等），填入 API Key、Base URL 与 Model，支持连通性测试。
   - **平台账号**：51job 登录状态（一键唤起浏览器登录/最小化）、Boss直聘 Cookie。
   - **邮件设置**：SMTP 服务器与授权码。
   - **数据与日志**：快捷动作标记：[ACTION:NAVIGATE|/settings|前往系统设置]、[ACTION:OPEN_DATA_DIR|打开本地数据目录]

回答系统功能时，如果涉及具体页面或操作，请务必在回复中附带上述对应格式的动作标记（如 [ACTION:NAVIGATE|/settings|前往系统设置]），系统前端会自动将其渲染为可点击的快捷直达按钮。
态度请保持专业、亲切、循循善诱。`

// ChatWithAIAssistant 处理来自前端 AI 助手的多轮对话
func (a *App) ChatWithAIAssistant(messages []ChatMessage, currentContext string, customCfg *AIConfig) (string, error) {
	if len(messages) == 0 {
		return "您好！我是透景 AI 招聘助理。我可以帮您撰写岗位JD与红线标准，也可以为您解答系统中的各项功能操作。请问有什么可以帮您？", nil
	}

	lastMsg := messages[len(messages)-1].Content
	trimmedLast := strings.TrimSpace(lastMsg)

	// 1. 优先使用前端传入的 AI 配置；若传入了有效配置，同步更新并持久化后端内存
	effectiveCfg := &a.config.AI
	if customCfg != nil && strings.TrimSpace(customCfg.APIKey) != "" {
		effectiveCfg = customCfg
		a.config.AI = *customCfg
		_ = a.SaveAIConfig(customCfg)
	}

	// 2. 检查有效配置中的 API Key 是否已配置
	if strings.TrimSpace(effectiveCfg.APIKey) == "" {
		// 未配置 API Key，启用内置智能离线知识库应答，不直接抛错
		return a.handleOfflineKnowledgeQuery(trimmedLast), nil
	}

	// 3. 已配置 API Key：组装对话上下文，调用大模型
	systemContent := AIAssistantSystemPrompt
	if currentContext != "" {
		systemContent += fmt.Sprintf("\n\n当前客户端运行上下文环境：\n%s", currentContext)
	}

	fullMessages := make([]ChatMessage, 0, len(messages)+1)
	fullMessages = append(fullMessages, ChatMessage{
		Role:    "system",
		Content: systemContent,
	})
	fullMessages = append(fullMessages, messages...)

	// 调用底层多轮对话接口
	reply, err := a.callAIChatMessages(effectiveCfg, fullMessages, 0.4)
	if err != nil {
		log.Printf("[ChatWithAIAssistant] 调用 AI 出错: %v", err)
		// 已配置云端模型时，不把通用离线模板伪装成该问题的 AI 回答。
		return "", err
	}

	return reply, nil
}

// callAIChatMessages 执行通用的多轮 Chat 请求
func (a *App) callAIChatMessages(cfg *AIConfig, messages []ChatMessage, temperature float64) (string, error) {
	content, _, err := a.callAIChatMessagesDetailed(cfg, messages, temperature)
	return content, err
}

// callAIChatMessagesDetailed 执行通用的多轮 Chat 请求并返回结果与 Token 用量
func (a *App) callAIChatMessagesDetailed(cfg *AIConfig, messages []ChatMessage, temperature float64) (string, *AIUsage, error) {
	if cfg.APIKey == "" {
		return "", nil, fmt.Errorf("API Key 未配置")
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gpt-4o"
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	reqBody := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   4000,
	}
	if isGLM53Flash(model) {
		// This model defaults to maximum reasoning. A resume can exhaust the
		// output budget before it produces any final answer.
		reqBody.MaxTokens = 16384
		reqBody.ReasoningEffort = "low"
	}

	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"

	var lastErr error
	maxRetries := cfg.MaxRetries
	if maxRetries < 1 {
		maxRetries = 2
	}
	if isGLM53Flash(model) && maxRetries < 2 {
		maxRetries = 2
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if isGLM53Flash(model) && attempt > 0 {
			reqBody.MaxTokens = 32768
		}
		body, err := json.Marshal(reqBody)
		if err != nil {
			return "", nil, fmt.Errorf("序列化请求失败: %w", err)
		}
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 1500 * time.Millisecond)
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

		timeoutSec := cfg.Timeout
		if timeoutSec < 180 {
			timeoutSec = 180
		}
		client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
		log.Printf("[callAIChatMessages] 正在请求 AI 接口 (尝试 %d/%d, 超时=%d秒): %s", attempt+1, maxRetries, timeoutSec, url)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("请求过于频繁 (HTTP 429)")
			continue
		}

		if resp.StatusCode >= 400 {
			switch resp.StatusCode {
			case http.StatusPaymentRequired:
				return "", nil, fmt.Errorf("AI 服务账户余额不足。请到模型服务商平台充值或更换可用模型，再重试")
			case http.StatusUnauthorized:
				return "", nil, fmt.Errorf("AI 服务认证失败。请检查系统设置中的 API Key 和模型配置")
			case http.StatusForbidden:
				return "", nil, fmt.Errorf("AI 服务拒绝访问。请检查模型权限与账户状态")
			case http.StatusBadRequest:
				return "", nil, fmt.Errorf("AI 服务请求配置不正确。请检查模型名称和接口地址")
			}
			lastErr = fmt.Errorf("AI 服务暂时不可用 (HTTP %d)", resp.StatusCode)
			if resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests {
				return "", nil, lastErr
			}
			continue
		}

		var chatResp ChatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			lastErr = fmt.Errorf("解析响应失败: %w", err)
			continue
		}

		if chatResp.Error != nil {
			lastErr = fmt.Errorf("AI 错误: %s", chatResp.Error.Message)
			continue
		}

		if len(chatResp.Choices) == 0 {
			lastErr = fmt.Errorf("AI 未返回任何回答")
			continue
		}

		answer := strings.TrimSpace(chatResp.Choices[0].Message.Content)
		if answer == "" {
			lastErr = fmt.Errorf("模型只返回思考内容，没有生成可用回答；输出长度可能不足")
			if isGLM53Flash(model) && attempt+1 < maxRetries {
				continue
			}
			return "", nil, lastErr
		}

		var usage *AIUsage
		if chatResp.Usage != nil {
			usage = &AIUsage{
				PromptTokens:     chatResp.Usage.PromptTokens,
				CompletionTokens: chatResp.Usage.CompletionTokens,
				TotalTokens:      chatResp.Usage.TotalTokens,
			}
			if chatResp.Usage.CompletionTokensDetails != nil {
				usage.ReasoningTokens = chatResp.Usage.CompletionTokensDetails.ReasoningTokens
			}
			log.Printf("[callAIChatMessagesDetailed] Token消耗: prompt=%d, completion=%d, total=%d", usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
		}

		return answer, usage, nil
	}

	return "", nil, fmt.Errorf("重试 %d 次后失败: %v", maxRetries, lastErr)
}

// handleOfflineKnowledgeQuery 本地离线智能知识库应答器
func (a *App) handleOfflineKnowledgeQuery(query string) string {
	q := strings.ToLower(query)

	// 1. 询问 AI 配置 / Key 设置
	if strings.Contains(q, "key") || strings.Contains(q, "配置") || strings.Contains(q, "模型") || strings.Contains(q, "deepseek") || strings.Contains(q, "ollama") || strings.Contains(q, "openai") {
		return "### 💡 如何配置 AI 模型 (DeepSeek / OpenAI / Ollama 等)\n\n" +
			"透景支持兼容 OpenAI 接口标准的任何主流大模型（如 DeepSeek、Moonshot、阿里云通义千问、MiniMax 以及本地 Ollama）：\n\n" +
			"1. 点击左侧导航栏的 **「系统设置」**，进入 **「AI 配置」** 选项卡；\n" +
			"2. 选择或输入模型供应商，填入 **API Key** 与 **Base URL**（例如 DeepSeek 为 `https://api.deepseek.com/v1`）；\n" +
			"3. 填写模型名称（例如 `deepseek-chat` 或 `deepseek-reasoner`）；\n" +
			"4. 点击 **「测试连接」**，提示连接成功后点击 **「保存配置」** 即可激活全功能 AI 能力！\n\n" +
			"[ACTION:NAVIGATE|/settings|🚀 前往系统设置配置 AI 模型]"
	}

	// 2. 询问 51job / 前程无忧功能
	if strings.Contains(q, "51job") || strings.Contains(q, "前程无忧") || strings.Contains(q, "打招呼") || strings.Contains(q, "发布职位") {
		return "### 🌐 51job (前程无忧) 平台联动功能指南\n\n" +
			"透景系统内置了与 51job eHire 平台的无缝联动功能：\n\n" +
			"1. **平台登录与鉴权**：在「系统设置」中点击「唤起 51job 登录」，在弹出窗口中完成企业登录；系统会自动接管会话，随后可点击「最小化窗口」保持后台运行；\n" +
			"2. **一键发布职位**：在招聘项目列表中，点击卡片上的「51job发布」按钮，系统会自动智能映射职能分类、生成草稿并发布；\n" +
			"3. **一键打招呼 (Hi 沟通)**：在候选人卡片中，可预估所需点数，一键向 51job 候选人发起沟通打招呼。\n\n" +
			"[ACTION:NAVIGATE|/settings|前往系统设置检查 51job 状态] [ACTION:NAVIGATE|/|查看招聘项目]"
	}

	// 3. 询问 Boss 直聘功能
	if strings.Contains(q, "boss") || strings.Contains(q, "直聘") || strings.Contains(q, "搜寻") || strings.Contains(q, "猎聘") {
		return "### 💼 Boss 直聘多维度搜索与主动沟通\n\n" +
			"透景支持对 Boss 直聘候选人进行多维度自动化检索：\n\n" +
			"1. **登录凭证检测**：在系统设置中核验 Boss 直聘 Cookie 是否有效；\n" +
			"2. **多词矩阵搜索**：在项目页面中唤起 Boss 搜寻弹窗，输入岗位与专业关键词矩阵，系统将自动爬取匹配简历并打招呼；\n" +
			"3. **批量归档**：搜寻到的意向候选人可一键归档到当前招聘项目中进行后续 AI 深度终审。\n\n" +
			"[ACTION:NAVIGATE|/settings|前往系统设置]"
	}

	// 4. 询问简历筛选 / 导入 / AI 评估 / 淘汰红线
	if strings.Contains(q, "简历") || strings.Contains(q, "导入") || strings.Contains(q, "红线") || strings.Contains(q, "打分") || strings.Contains(q, "筛选") {
		return "### 📄 简历批量导入与 AI 深度评估流程\n\n" +
			"1. **创建或选择项目**：进入「招聘项目列表」，点击进入具体项目；\n" +
			"2. **导入候选人简历**：直接将 PDF、Word (.docx) 或图片格式的简历文件**拖拽**至项目主界面，或点击「选择文件导入」；\n" +
			"3. **设定淘汰红线**：在项目设置中可自定义负向过滤规则（例如：\"学历低于本科直接淘汰\"、\"无相关临床经验淘汰\"）；\n" +
			"4. **启动 AI 分析**：点击「一键AI分析」，系统将结合能力模型与红线规则输出五维雷达打分，并预警简历中的虚假量化描述；\n" +
			"5. **AI 预审面试**：点击候选人卡片上的「AI 面试」，系统会自动针对候选人的短板生成5组针对性面试提纲。\n\n" +
			"[ACTION:NAVIGATE|/|前往招聘项目列表] [ACTION:OPEN_CREATE_PROJECT|新建招聘项目]"
	}

	// 5. 询问 发送 Offer / 录用通知
	if strings.Contains(q, "offer") || strings.Contains(q, "录用") || strings.Contains(q, "邮件") || strings.Contains(q, "smtp") {
		return "### ✉️ 候选人 Offer 录用通知书直发指南\n\n" +
			"透景支持直接向候选人邮箱一键直发正式录用 Offer：\n\n" +
			"1. **前置配置**：在「系统设置」的「邮件设置」中配置发件 SMTP 服务器（如 163、QQ 或企业邮箱）及授权码；\n" +
			"2. **发放 Offer**：在项目候选人卡片中，点击「发送 Offer」，填写录用岗位、薪资包、入职报到日期及补充说明；\n" +
			"3. **一键直发**：系统会自动生成排版精美的 Offer 邮件模板并直达候选人邮箱。\n\n" +
			"[ACTION:NAVIGATE|/settings|前往系统设置配置邮件服务]"
	}

	// 6. 询问数据目录 / 备份 / 日志
	if strings.Contains(q, "目录") || strings.Contains(q, "文件") || strings.Contains(q, "数据") || strings.Contains(q, "日志") || strings.Contains(q, "存储") {
		return "### 📂 本地数据存储与系统维护\n\n" +
			"透景的所有项目配置、简历解析缓存与日志均安全保存在您的本地磁盘中：\n\n" +
			"- 您可随时点击下方按钮快速在文件资源管理器中打开系统数据目录；\n" +
			"- 如遇到分析异常，可导出诊断错误报告以供排查。\n\n" +
			"[ACTION:OPEN_DATA_DIR|打开本地数据存储目录]"
	}

	// 7. 如果用户询问撰写某岗位 JD，且未配置 Key，给出专业结构化模版
	return "### 📋 智能招聘需求参考模板\n\n" +
		"（💡 **提示**：您当前尚未在系统设置中配置大模型 API Key。以下为您展示系统内置标准化岗位模板。若需根据您的个性化诉求进行多轮智能定制与润色，请前往系统设置配置 API Key）\n\n" +
		"#### 岗位职责：\n" +
		"1. 负责核心业务板块的规划设计、方案编制与日常推进；\n" +
		"2. 深度协同跨职能团队，按计划保质保量达成业务指标；\n" +
		"3. 沉淀业务经验，持续优化操作流程与标准化制度。\n\n" +
		"#### 任职要求：\n" +
		"1. 本科及以上学历，对口专业背景，具备3年以上相关领域工作经验；\n" +
		"2. 具备良好的自驱力、抗压能力及扎实的专业实践功底；\n" +
		"3. 具备成熟行业标杆案例或大型项目统筹经验者优先。\n\n" +
		"```json:talentlens-job\n" +
		"{\n" +
		"  \"title\": \"" + query + "\",\n" +
		"  \"department\": \"业务部门\",\n" +
		"  \"headcount\": 1,\n" +
		"  \"experience_years\": 3,\n" +
		"  \"education_level\": \"本科\",\n" +
		"  \"job_description\": \"负责核心业务板块的规划设计、方案编制与日常推进；深度协同跨职能团队，按计划保质保量达成业务指标。沉淀业务经验，持续优化操作流程与标准化制度。\",\n" +
		"  \"required_skills\": [\"专业领域技能\", \"方案设计\", \"跨部门协同\", \"质量管理\"],\n" +
		"  \"interview_questions\": [\"请说明过去3年在对口业务领域的代表性实战成果\", \"请介绍在跨职能协同中主导解决的最复杂业务难题\"]\n" +
		"}\n" +
		"```\n\n" +
		"[ACTION:OPEN_CREATE_PROJECT|一键新建项目] [ACTION:NAVIGATE|/settings|前往系统设置配置 API Key]"
}
