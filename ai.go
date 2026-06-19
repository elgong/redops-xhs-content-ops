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
	return map[string]any{
		"ai_provider":       ai.cfg.AIProvider,
		"openai_configured": strings.TrimSpace(ai.cfg.OpenAIAPIKey) != "",
		"openai_model":      ai.cfg.OpenAIModel,
		"openai_base_url":   ai.cfg.OpenAIBaseURL,
	}
}

func newContentAI(cfg Config) ContentAI {
	if cfg.AIProvider == "local" {
		return LocalContentAI{}
	}
	return OpenAIContentAI{
		client: OpenAIClient{
			APIKey:  cfg.OpenAIAPIKey,
			Model:   cfg.OpenAIModel,
			BaseURL: cfg.OpenAIBaseURL,
			Client:  &http.Client{Timeout: 60 * time.Second},
		},
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

type OpenAIContentAI struct {
	client OpenAIClient
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
		c.Model = "gpt-5.5"
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
