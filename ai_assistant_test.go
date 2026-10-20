package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestChatWithAIAssistantPaymentRequiredDoesNotRetryOrInventAnswer(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"code":30001,"message":"Sorry, your account balance is insufficient"}`))
	}))
	defer server.Close()

	app := &App{
		dataDirOverride: t.TempDir(),
		config: Config{AI: AIConfig{
			APIKey: "test-key", BaseURL: server.URL, Model: "test-model", MaxRetries: 3,
		}},
	}
	reply, err := app.ChatWithAIAssistant([]ChatMessage{{Role: "user", Content: "你好"}}, "", nil)
	if err == nil || !strings.Contains(err.Error(), "余额不足") {
		t.Fatalf("期望清晰的余额不足错误，实际回答=%q 错误=%v", reply, err)
	}
	if reply != "" || strings.Contains(err.Error(), "系统内置知识库") || strings.Contains(err.Error(), "30001") {
		t.Fatalf("不应回退到不相关模板或展示原始 API 错误，回答=%q 错误=%v", reply, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("HTTP 402 不应重试，实际调用 %d 次", calls.Load())
	}
}

func TestGLMFlashConnectionUsesWorkspaceChatEndpointAndThinkingBudget(t *testing.T) {
	type requestDetails struct {
		path, authorization, model, effort string
		maxTokens                          int
	}
	seen := make(chan requestDetails, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model           string `json:"model"`
			MaxTokens       int    `json:"max_tokens"`
			ReasoningEffort string `json:"reasoning_effort"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen <- requestDetails{r.URL.Path, r.Header.Get("Authorization"), body.Model, body.ReasoningEffort, body.MaxTokens}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"OK"}}]}`))
	}))
	defer server.Close()

	app := NewApp()
	ok, message := app.TestAIConnection(&AIConfig{Provider: "aliyun-glm", BaseURL: server.URL + "/compatible-mode/v1", APIKey: "workspace-test-key", Model: "ZHIPU/GLM-5.3-Flash", Timeout: 30})
	if !ok {
		t.Fatalf("GLM Flash connection test failed: %s", message)
	}
	request := <-seen
	if request.path != "/compatible-mode/v1/chat/completions" || request.authorization != "Bearer workspace-test-key" || request.model != "ZHIPU/GLM-5.3-Flash" || request.effort != "low" || request.maxTokens < 2048 {
		t.Fatalf("unexpected GLM Flash request: %+v", request)
	}
}

func TestGLMFlashConnectionDoesNotAcceptThinkingOnlyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","reasoning_content":"thinking","content":""}}]}`))
	}))
	defer server.Close()
	app := NewApp()
	ok, message := app.TestAIConnection(&AIConfig{BaseURL: server.URL, APIKey: "test-key", Model: "ZHIPU/GLM-5.3-Flash", Timeout: 30})
	if ok || !strings.Contains(message, "回答") {
		t.Fatalf("thinking-only response should not pass connection test: ok=%v message=%q", ok, message)
	}
}

func TestGLMFlashConnectionExplainsInsufficientBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"code":30001,"message":"Sorry, your account balance is insufficient"}`))
	}))
	defer server.Close()
	app := NewApp()
	ok, message := app.TestAIConnection(&AIConfig{BaseURL: server.URL, APIKey: "test-key", Model: "ZHIPU/GLM-5.3-Flash", Timeout: 30})
	if ok || !strings.Contains(message, "余额不足") || strings.Contains(message, "30001") {
		t.Fatalf("HTTP 402 should explain the next action without raw details: ok=%v message=%q", ok, message)
	}
}

func TestGLMFlashChatReservesOutputBeyondThinking(t *testing.T) {
	seen := make(chan int, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen <- body.MaxTokens
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"可以继续分析简历"}}]}`))
	}))
	defer server.Close()
	app := NewApp()
	reply, err := app.callAIChatMessages(&AIConfig{BaseURL: server.URL, APIKey: "test-key", Model: "ZHIPU/GLM-5.3-Flash", MaxRetries: 1}, []ChatMessage{{Role: "user", Content: "你好"}}, 0.4)
	if err != nil || reply != "可以继续分析简历" || <-seen < 8192 {
		t.Fatalf("GLM Flash chat response=%q error=%v", reply, err)
	}
}

func TestGLMFlashRetriesThinkingOnlyAnswerWithLargerBudget(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens       int    `json:"max_tokens"`
			ReasoningEffort string `json:"reasoning_effort"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		n := calls.Add(1)
		if body.ReasoningEffort != "low" || (n == 1 && body.MaxTokens < 16384) || (n == 2 && body.MaxTokens < 32768) {
			t.Errorf("unexpected GLM request %d: %+v", n, body)
		}
		if n == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","reasoning_content":"thinking","content":""}}]}`))
		} else {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"评估完成"}}]}`))
		}
	}))
	defer server.Close()
	app := NewApp()
	reply, err := app.callAIChatMessages(&AIConfig{BaseURL: server.URL, APIKey: "test-key", Model: "ZHIPU/GLM-5.3-Flash", MaxRetries: 1},
		[]ChatMessage{{Role: "user", Content: "请评估"}}, 0.2)
	if err != nil || reply != "评估完成" || calls.Load() != 2 {
		t.Fatalf("retry answer=%q error=%v calls=%d", reply, err, calls.Load())
	}
}

func TestChatWithAIAssistantOfflineKnowledge(t *testing.T) {
	app := &App{
		dataDirOverride: t.TempDir(),
		config: Config{
			AI: AIConfig{
				APIKey: "", // 未配置 Key
			},
		},
	}

	// 1. 测试空输入
	emptyResp, err := app.ChatWithAIAssistant([]ChatMessage{}, "", nil)
	if err != nil {
		t.Fatalf("空消息测试返回错误: %v", err)
	}
	if !strings.Contains(emptyResp, "透景 AI 招聘助理") {
		t.Errorf("期望默认欢迎语，实际返回: %s", emptyResp)
	}

	// 2. 测试询问大模型配置
	configResp, err := app.ChatWithAIAssistant([]ChatMessage{
		{Role: "user", Content: "请问怎么在软件里配置大模型 API Key？"},
	}, "", nil)
	if err != nil {
		t.Fatalf("询问配置失败: %v", err)
	}
	if !strings.Contains(configResp, "/settings") {
		t.Errorf("回答中未包含系统设置跳转动作，实际返回: %s", configResp)
	}

	// 3. 测试询问 51job 功能
	jobResp, err := app.ChatWithAIAssistant([]ChatMessage{
		{Role: "user", Content: "51job怎么发布职位？"},
	}, "", nil)
	if err != nil {
		t.Fatalf("询问 51job 失败: %v", err)
	}
	if !strings.Contains(jobResp, "51job") || !strings.Contains(jobResp, "eHire") {
		t.Errorf("回答中未包含 51job 指南，实际返回: %s", jobResp)
	}

	// 4. 测试岗位 JD 模板生成
	jdResp, err := app.ChatWithAIAssistant([]ChatMessage{
		{Role: "user", Content: "帮我写一个临床试验经理的岗位要求"},
	}, "", nil)
	if err != nil {
		t.Fatalf("生成 JD 模板失败: %v", err)
	}
	if !strings.Contains(jdResp, "json:talentlens-job") {
		t.Errorf("回答中未包含 talentlens-job 结构化代码块，实际返回: %s", jdResp)
	}
	if !strings.Contains(jdResp, "interview_questions") {
		t.Errorf("回答中未包含 interview_questions 初试问题字段，实际返回: %s", jdResp)
	}
}

func TestChatWithAIAssistantOnlineMock(t *testing.T) {
	// 创建 Mock LLM 服务
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// 检查系统提示词是否已注入
		if len(req.Messages) < 2 {
			http.Error(w, "missing messages", http.StatusBadRequest)
			return
		}
		if req.Messages[0].Role != "system" {
			http.Error(w, "expected system prompt", http.StatusBadRequest)
			return
		}

		resp := ChatResponse{
			Choices: []struct {
				Message ChatMessage `json:"message"`
			}{
				{
					Message: ChatMessage{
						Role:    "assistant",
						Content: "收到，已为您制定高级前端岗位：\n\n```json:talentlens-job\n{\n  \"title\": \"高级前端工程师\",\n  \"department\": \"研发中心\",\n  \"headcount\": 2,\n  \"experience_years\": 5,\n  \"education_level\": \"本科\",\n  \"job_description\": \"负责前端架构设计与复杂交互落地\",\n  \"required_skills\": [\"Vue3\", \"TypeScript\", \"Pinia\"],\n  \"red_lines\": [\"必须精通Vue3与TS\"],\n  \"bonus_points\": [\"有跨端开发经验\"]\n}\n```\n\n[ACTION:NAVIGATE|/|查看项目列表]",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	app := &App{
		dataDirOverride: t.TempDir(),
		config: Config{
			AI: AIConfig{
				APIKey:     "test-key",
				BaseURL:    server.URL,
				Model:      "test-model",
				MaxRetries: 1,
				Timeout:    10,
			},
		},
	}

	reply, err := app.ChatWithAIAssistant([]ChatMessage{
		{Role: "user", Content: "招一个5年经验的高级前端"},
	}, "当前页面: 招聘项目", nil)
	if err != nil {
		t.Fatalf("调用 Mock AI 助手失败: %v", err)
	}

	if !strings.Contains(reply, "高级前端工程师") || !strings.Contains(reply, "json:talentlens-job") {
		t.Errorf("返回内容不符合预期: %s", reply)
	}
	if !strings.Contains(reply, "[ACTION:NAVIGATE|/|查看项目列表]") {
		t.Errorf("返回内容缺少动作标签: %s", reply)
	}
}

func TestChatWithAIAssistantWithCustomConfigOverridesEmptyBackend(t *testing.T) {
	// 创建 Mock LLM 服务
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			Choices: []struct {
				Message ChatMessage `json:"message"`
			}{
				{
					Message: ChatMessage{
						Role:    "assistant",
						Content: "已根据前端自定义配置生成建议",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 后端初始无 Key
	app := &App{
		dataDirOverride: t.TempDir(),
		config: Config{
			AI: AIConfig{
				APIKey: "",
			},
		},
	}

	customCfg := &AIConfig{
		Provider:   "deepseek",
		BaseURL:    server.URL,
		APIKey:     "custom-client-key-123",
		Model:      "deepseek-chat",
		MaxRetries: 1,
		Timeout:    10,
	}

	reply, err := app.ChatWithAIAssistant([]ChatMessage{
		{Role: "user", Content: "测试自定义前端配置传参"},
	}, "", customCfg)
	if err != nil {
		t.Fatalf("使用自定义前端配置调用失败: %v", err)
	}

	if !strings.Contains(reply, "已根据前端自定义配置生成建议") {
		t.Errorf("未能正确收到 Mock 响应: %s", reply)
	}

	// 验证后端内存中的 AI 配置已同步更新
	if app.config.AI.APIKey != "custom-client-key-123" {
		t.Errorf("后端未自动同步更新 customCfg: %+v", app.config.AI)
	}
}
