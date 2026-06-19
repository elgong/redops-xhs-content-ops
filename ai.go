package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ContentAI interface {
	Analyze(ctx context.Context, task KeywordTask, posts []SourcePost) (InsightReport, error)
	Generate(ctx context.Context, task KeywordTask, insight InsightReport, rules AppRules, account Account, instruction string) (GeneratedContent, error)
}

func NewConfiguredContentAI(cfg Config) ContentAI {
	return NewReloadableContentAI(cfg)
}

type ReloadableContentAI struct {
	mu    sync.RWMutex
	cfg   Config
	inner ContentAI
}

func NewReloadableContentAI(cfg Config) *ReloadableContentAI {
	return &ReloadableContentAI{cfg: cfg, inner: newContentAI(cfg)}
}

func (ai *ReloadableContentAI) Analyze(ctx context.Context, task KeywordTask, posts []SourcePost) (InsightReport, error) {
	ai.mu.RLock()
	inner := ai.inner
	ai.mu.RUnlock()
	return inner.Analyze(ctx, task, posts)
}

func (ai *ReloadableContentAI) Generate(ctx context.Context, task KeywordTask, insight InsightReport, rules AppRules, account Account, instruction string) (GeneratedContent, error) {
	ai.mu.RLock()
	inner := ai.inner
	ai.mu.RUnlock()
	return inner.Generate(ctx, task, insight, rules, account, instruction)
}

func (ai *ReloadableContentAI) Update(cfg Config) {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	ai.cfg = cfg
	ai.inner = newContentAI(cfg)
}

func (ai *ReloadableContentAI) Status() map[string]any {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	provider := strings.ToLower(strings.TrimSpace(ai.cfg.AIProvider))
	hasKey := strings.TrimSpace(ai.cfg.OpenAIAPIKey) != ""
	configured := provider == "local" || hasKey
	baseURL := ai.cfg.OpenAIBaseURL
	if provider == "local" {
		baseURL = ""
	}
	return map[string]any{
		"ai_provider":         ai.cfg.AIProvider,
		"api_configured":      configured,
		"openai_configured":   provider == "openai" && hasKey,
		"deepseek_configured": provider == "deepseek" && hasKey,
		"openai_model":        ai.cfg.OpenAIModel,
		"openai_base_url":     baseURL,
	}
}

func newContentAI(cfg Config) ContentAI {
	switch strings.ToLower(strings.TrimSpace(cfg.AIProvider)) {
	case "local":
		return LocalContentAI{}
	case "deepseek":
		model := cfg.OpenAIModel
		if model == "" || model == "local" || strings.HasPrefix(model, "gpt-") {
			model = "deepseek-v4-flash"
		}
		baseURL := cfg.OpenAIBaseURL
		if baseURL == "" || strings.Contains(baseURL, "api.openai.com") {
			baseURL = "https://api.deepseek.com"
		}
		return OpenAIContentAI{
			client: DeepSeekClient{
				APIKey:  cfg.OpenAIAPIKey,
				Model:   model,
				BaseURL: baseURL,
				Client:  &http.Client{Timeout: 60 * time.Second},
			},
		}
	default:
		model := cfg.OpenAIModel
		if model == "" || model == "local" || strings.HasPrefix(model, "deepseek-") {
			model = "gpt-4o-mini"
		}
		baseURL := cfg.OpenAIBaseURL
		if baseURL == "" || strings.Contains(baseURL, "deepseek.com") {
			baseURL = "https://api.openai.com"
		}
		return OpenAIContentAI{
			client: OpenAIClient{
				APIKey:  cfg.OpenAIAPIKey,
				Model:   model,
				BaseURL: baseURL,
				Client:  &http.Client{Timeout: 60 * time.Second},
			},
		}
	}
}

type LocalContentAI struct{}

func (LocalContentAI) Analyze(ctx context.Context, task KeywordTask, posts []SourcePost) (InsightReport, error) {
	var total float64
	for _, post := range posts {
		total += post.HotScore
	}
	avg := total / float64(len(posts))
	return InsightReport{
		KeywordTaskID:      task.ID,
		AverageHotScore:    round(avg),
		TitlePatterns:      "痛点提醒 + 真实体验 + 结果承诺弱化；常见词：真的、别乱、后悔没早知道",
		StyleTags:          "真实分享,朋友式提醒,清单化,少广告感",
		AudiencePainPoints: fmt.Sprintf("%s相关用户主要关注适用人群、预算、操作顺序和避坑。", task.Keyword),
		RiskHints:          "避免绝对化功效、诱导互动、站外导流和大面积复制爆文原句。",
	}, nil
}

func (LocalContentAI) Generate(ctx context.Context, task KeywordTask, insight InsightReport, rules AppRules, account Account, instruction string) (GeneratedContent, error) {
	if strings.Contains(strings.ToLower(task.Keyword), "codex") || strings.Contains(task.Category, "AI") {
		return generateLocalAIToolContent(task, rules, instruction), nil
	}
	version := 1
	title := fmt.Sprintf("%s真的别乱做，我踩过坑", task.Keyword)
	if strings.Contains(instruction, "标题") || strings.Contains(instruction, "夸张") {
		title = fmt.Sprintf("%s这套顺序我想早点知道", task.Keyword)
		version = 2
	}
	body := fmt.Sprintf("最近看了不少%s相关内容，发现高热笔记都不是硬讲道理，而是先说真实场景。我的建议是先把步骤拆简单，再根据自己的预算和使用频率慢慢调整。\n\n%s\n\n适合想提高效率、又不想被复杂流程劝退的人。", task.Keyword, insight.AudiencePainPoints)
	if instruction != "" {
		body += "\n\n本次根据审核意见调整：" + instruction
	}
	tags := fmt.Sprintf("#%s #%s经验 #运营灵感 #真实分享", strings.ReplaceAll(task.Keyword, " ", ""), task.Category)
	risk := "low"
	if containsAny(title+body+tags, rules.BannedPhrases) {
		risk = "high"
	}
	return GeneratedContent{
		KeywordTaskID:  task.ID,
		AccountID:      account.ID,
		Title:          title,
		Body:           body,
		CoverText:      fmt.Sprintf("%s避坑清单", task.Keyword),
		Tags:           tags,
		RiskLevel:      risk,
		DuplicateScore: 0.16 + rand.Float64()*0.18,
		Status:         ContentPendingReview,
		Version:        version,
	}, nil
}

func generateLocalAIToolContent(task KeywordTask, rules AppRules, instruction string) GeneratedContent {
	version := 1
	title := "Codex 用浅了真亏，我现在主要用这 5 个场景"
	cover := "Codex 实用场景"
	body := "这两天看了不少 Codex 相关笔记，发现高赞内容基本不是单纯夸 AI 多强，而是把「怎么用在真实工作里」讲清楚。\n\n我目前觉得最实用的是这 5 个场景：\n\n1. 接手陌生项目\n先让 Codex 读目录结构、启动方式、核心接口和数据库表，省掉自己翻半天文件的时间。\n\n2. 改小需求\n不要只说「帮我改一下」，我会补一句：先读相关文件，按现有代码风格改，改完跑测试。效果会稳很多。\n\n3. 查 bug\n把报错、复现步骤、期望结果一起给它，让它先定位可能文件，再动代码。比直接贴一句报错靠谱。\n\n4. 写验收用例\n让它按「正常流程、异常流程、边界条件」列清单，再转成 curl 或测试代码，特别适合补漏。\n\n5. 做代码评审\n我会让它先只读评审，列阻塞项和风险点，再决定要不要改，能避免一上来乱动代码。\n\n我的使用顺序是：先让它理解项目，再让它给计划，最后才允许改文件。这样 Codex 更像一个能一起干活的工程搭子，而不是一个只会回答问题的聊天框。"
	tags := "#Codex #AI工具 #效率工具 #程序员日常 #真实分享"
	lowerInstruction := strings.ToLower(instruction)
	if strings.Contains(instruction, "避坑") || strings.Contains(instruction, "踩") || strings.Contains(lowerInstruction, "pitfall") {
		version = 2
		title = "Codex 不是许愿机，这几个坑我踩完才会用"
		cover = "Codex 避坑清单"
		body = "如果你刚开始用 Codex，千万别只丢一句「帮我做完」。我一开始也是这么用，结果改得快，但返工也快。\n\n几个真实避坑点：\n\n1. 需求别太空\n差一点的说法：帮我优化页面。\n更稳的说法：保持现有风格，修复审核列表只展示待审核，非待审核不要显示审核按钮。\n\n2. 先评审，再改代码\n复杂需求我会先让它开评审或列验收标准。标准清楚以后再动手，最后能少很多返工。\n\n3. 明确哪些不能碰\n比如不要改数据库结构、不要重置用户数据、不要提交密钥。Codex 很听话，但前提是边界说清楚。\n\n4. 让它自己跑测试\n我现在都会要求：改完自己跑测试，跑不通继续修。这样它不只是写代码，还能把闭环跑完。\n\n5. 大任务拆小\n像采集、分析、生成、审核、发布这种链路，最好一段一段验收。每段都过，再看全流程。\n\n我的结论：Codex 最强的地方不是替你写几行代码，而是能把「读代码、改代码、跑测试、复盘问题」串起来。你越会给验收标准，它越像真正的开发同事。"
	}
	if strings.Contains(instruction, "3 个场景") || strings.Contains(instruction, "三个场景") {
		title = "Codex 真正提效的 3 个场景，新手可以直接抄"
		cover = "Codex 3 个用法"
		body = "最近刷到很多 Codex 教程，我自己试下来，最适合新手先用的不是复杂自动化，而是这 3 个高频场景。\n\n场景 1：读懂一个陌生项目\n让它先回答：项目怎么启动、核心目录是什么、主要接口在哪、数据从哪里来。这个动作能把上手时间压得很低。\n\n场景 2：带验收标准改需求\n我会直接写：改完请自己跑测试，不合格继续修。比如「审核列表只显示待审核，驳回必须填原因，保存草稿失败不能污染状态」。这种标准越明确，结果越稳。\n\n场景 3：让它做第二双眼睛\n不是每次都让它马上改。先让它只读 review，按阻塞项、次要问题、建议修复项输出，很多隐藏问题会提前暴露。\n\n一个小技巧：别把 Codex 当搜索框，要把它当协作开发。给背景、给边界、给验收标准，它就能从回答问题变成真的推进项目。"
	}
	if instruction != "" {
		body += "\n\n本次生成要求：" + instruction
	}
	risk := "low"
	if containsAny(title+body+tags, rules.BannedPhrases) {
		risk = "high"
	}
	return GeneratedContent{
		KeywordTaskID:  task.ID,
		Title:          title,
		Body:           body,
		CoverText:      cover,
		Tags:           tags,
		RiskLevel:      risk,
		DuplicateScore: 0.12 + rand.Float64()*0.12,
		Status:         ContentPendingReview,
		Version:        version,
	}
}

type OpenAIContentAI struct {
	client TextAIClient
}

type TextAIClient interface {
	Respond(ctx context.Context, instructions, input string) (string, error)
}

func (ai OpenAIContentAI) Analyze(ctx context.Context, task KeywordTask, posts []SourcePost) (InsightReport, error) {
	payload := map[string]any{"keyword": task.Keyword, "category": task.Category, "posts": posts}
	input, _ := json.Marshal(payload)
	out, err := ai.client.Respond(ctx, "你是资深小红书内容运营分析师。基于输入的真实搜索样本，输出严格 JSON，不要 Markdown。JSON 字段：average_hot_score(number), title_patterns(string), style_tags(string), audience_pain_points(string), risk_hints(string)。", string(input))
	if err != nil {
		return InsightReport{}, err
	}
	var parsed struct {
		AverageHotScore    float64 `json:"average_hot_score"`
		TitlePatterns      string  `json:"title_patterns"`
		StyleTags          string  `json:"style_tags"`
		AudiencePainPoints string  `json:"audience_pain_points"`
		RiskHints          string  `json:"risk_hints"`
	}
	if err := decodeJSONObject(out, &parsed); err != nil {
		return InsightReport{}, err
	}
	if parsed.AverageHotScore == 0 {
		var total float64
		for _, post := range posts {
			total += post.HotScore
		}
		parsed.AverageHotScore = round(total / float64(len(posts)))
	}
	return InsightReport{
		KeywordTaskID:      task.ID,
		AverageHotScore:    round(parsed.AverageHotScore),
		TitlePatterns:      parsed.TitlePatterns,
		StyleTags:          parsed.StyleTags,
		AudiencePainPoints: parsed.AudiencePainPoints,
		RiskHints:          parsed.RiskHints,
	}, nil
}

func (ai OpenAIContentAI) Generate(ctx context.Context, task KeywordTask, insight InsightReport, rules AppRules, account Account, instruction string) (GeneratedContent, error) {
	payload := map[string]any{
		"keyword":     task.Keyword,
		"category":    task.Category,
		"account":     account,
		"insight":     insight,
		"rules":       rules,
		"instruction": instruction,
	}
	input, _ := json.Marshal(payload)
	out, err := ai.client.Respond(ctx, "你是小红书品牌内容创作者。根据真实热点洞察生成一篇可提交人工审核的小红书图文笔记文案。必须输出严格 JSON，不要 Markdown。JSON 字段：title(string), body(string), cover_text(string), tags(string), risk_level(low|high), duplicate_score(number 0-1)。要求：模仿风格但不要复制原文；少广告感；避免绝对化功效和诱导互动；标签用中文 # 标签。", string(input))
	if err != nil {
		return GeneratedContent{}, err
	}
	var parsed struct {
		Title          string  `json:"title"`
		Body           string  `json:"body"`
		CoverText      string  `json:"cover_text"`
		Tags           string  `json:"tags"`
		RiskLevel      string  `json:"risk_level"`
		DuplicateScore float64 `json:"duplicate_score"`
	}
	if err := decodeJSONObject(out, &parsed); err != nil {
		return GeneratedContent{}, err
	}
	if parsed.RiskLevel == "" {
		parsed.RiskLevel = "low"
	}
	if parsed.DuplicateScore <= 0 {
		parsed.DuplicateScore = 0.18
	}
	if containsAny(parsed.Title+parsed.Body+parsed.Tags, rules.BannedPhrases) {
		parsed.RiskLevel = "high"
	}
	return GeneratedContent{
		KeywordTaskID:  task.ID,
		AccountID:      account.ID,
		Title:          parsed.Title,
		Body:           parsed.Body,
		CoverText:      parsed.CoverText,
		Tags:           parsed.Tags,
		RiskLevel:      parsed.RiskLevel,
		DuplicateScore: parsed.DuplicateScore,
		Status:         ContentPendingReview,
		Version:        1,
	}, nil
}

type OpenAIClient struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

func (c OpenAIClient) Respond(ctx context.Context, instructions, input string) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", fmt.Errorf("未配置 OPENAI_API_KEY，无法使用 GPT 分析和生成")
	}
	if c.Model == "" {
		c.Model = "gpt-4o-mini"
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://api.openai.com"
	}
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 60 * time.Second}
	}
	payload := map[string]any{
		"model":        c.Model,
		"instructions": instructions,
		"input":        input,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	resBody, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("OpenAI API 返回 HTTP %d：%s", res.StatusCode, trimRunes(string(resBody), 600))
	}
	text := extractResponseText(resBody)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("OpenAI API 未返回可用文本")
	}
	return text, nil
}

func extractResponseText(body []byte) string {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if typ, _ := x["type"].(string); typ == "output_text" {
				if text, _ := x["text"].(string); text != "" {
					out = append(out, text)
				}
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(raw)
	if len(out) > 0 {
		return strings.Join(out, "\n")
	}
	if m, ok := raw.(map[string]any); ok {
		if text, _ := m["output_text"].(string); text != "" {
			return text
		}
	}
	return ""
}

func decodeJSONObject(text string, dst any) error {
	text = strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(text), dst); err == nil {
		return nil
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return json.Unmarshal([]byte(text[start:end+1]), dst)
	}
	return fmt.Errorf("GPT 返回内容不是 JSON：%s", trimRunes(text, 300))
}

type DeepSeekClient struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

func (c DeepSeekClient) Respond(ctx context.Context, instructions, input string) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", fmt.Errorf("未配置 API Key，无法使用 DeepSeek 分析和生成")
	}
	if c.Model == "" {
		c.Model = "deepseek-v4-flash"
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://api.deepseek.com"
	}
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 60 * time.Second}
	}
	payload := map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": instructions},
			{"role": "user", "content": input},
		},
		"temperature": 0.7,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	resBody, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("DeepSeek API 返回 HTTP %d：%s", res.StatusCode, trimRunes(string(resBody), 600))
	}
	text := extractChatCompletionText(resBody)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("DeepSeek API 未返回可用文本")
	}
	return text, nil
}

func extractChatCompletionText(body []byte) string {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	var parts []string
	for _, choice := range parsed.Choices {
		if strings.TrimSpace(choice.Message.Content) != "" {
			parts = append(parts, choice.Message.Content)
		}
	}
	return strings.Join(parts, "\n")
}
