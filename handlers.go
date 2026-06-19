package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	store   Store
	service *AppService
	cfg     Config
}

func NewServer(store Store, service *AppService, cfg Config) *Server {
	return &Server{store: store, service: service, cfg: cfg}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/bootstrap", s.handleBootstrap)
	mux.HandleFunc("/api/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/accounts", s.handleAccounts)
	mux.HandleFunc("/api/keywords", s.handleKeywords)
	mux.HandleFunc("/api/keywords/", s.handleKeywordAction)
	mux.HandleFunc("/api/generate", s.handleGenerate)
	mux.HandleFunc("/api/contents", s.handleContents)
	mux.HandleFunc("/api/contents/", s.handleContentAction)
	mux.HandleFunc("/api/publish-tasks", s.handlePublishTasks)
	mux.HandleFunc("/api/publish/run-due", s.handleRunDue)
	mux.HandleFunc("/api/rules", s.handleRules)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/xhs/status", s.handleXHSStatus)
	mux.HandleFunc("/api/xhs/materials", s.handleXHSMaterials)
	return logMiddleware(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx := r.Context()
	d, err := s.store.Dashboard(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	accounts, _ := s.store.ListAccounts(ctx)
	contents, _ := s.store.ListContents(ctx, "")
	rules, _ := s.store.GetRules(ctx)
	writeJSON(w, http.StatusOK, map[string]any{"dashboard": d, "accounts": accounts, "contents": contents, "rules": rules})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	d, err := s.store.Dashboard(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		accounts, err := s.store.ListAccounts(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, accounts)
	case http.MethodPost:
		var req Account
		if !decodeJSON(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			writeError(w, errors.New("账号名称不能为空"))
			return
		}
		account, err := s.store.CreateAccount(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, account)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleKeywords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.store.ListKeywords(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var req struct {
			Keyword          string `json:"keyword"`
			Category         string `json:"category"`
			FrequencyMinutes int    `json:"frequency_minutes"`
			SampleLimit      int    `json:"sample_limit"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Keyword) == "" {
			writeError(w, errors.New("关键词不能为空"))
			return
		}
		if req.Category == "" {
			req.Category = "通用"
		}
		if req.FrequencyMinutes <= 0 {
			req.FrequencyMinutes = 60
		}
		if req.SampleLimit <= 0 {
			req.SampleLimit = 20
		}
		task, err := s.store.CreateKeyword(r.Context(), KeywordTask{
			Keyword:          req.Keyword,
			Category:         req.Category,
			FrequencyMinutes: req.FrequencyMinutes,
			SampleLimit:      req.SampleLimit,
			Status:           StatusRunning,
			CreatedBy:        "运营",
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, task)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleKeywordAction(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseActionPath(r.URL.Path, "/api/keywords/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch action {
	case "collect":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		posts, err := s.service.Collect(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, posts)
	case "analyze":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		report, err := s.service.Analyze(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, report)
	case "import":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req struct {
			Posts []SourcePost `json:"posts"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		posts, err := s.service.ImportPosts(r.Context(), id, req.Posts)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, posts)
	case "import-text":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req struct {
			RawText string `json:"raw_text"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		parsed, err := ParseXHSWebText(req.RawText)
		if err != nil {
			writeError(w, err)
			return
		}
		posts, err := s.service.ImportPosts(r.Context(), id, parsed)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, posts)
	case "posts":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		posts, err := s.store.ListSourcePosts(r.Context(), id, 50)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, posts)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		KeywordTaskID int64  `json:"keyword_task_id"`
		AccountID     int64  `json:"account_id"`
		Instruction   string `json:"instruction"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.KeywordTaskID == 0 || req.AccountID == 0 {
		writeError(w, errors.New("keyword_task_id 和 account_id 必填"))
		return
	}
	content, err := s.service.Generate(r.Context(), req.KeywordTaskID, req.AccountID, req.Instruction)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, content)
}

func (s *Server) handleContents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := s.store.ListContents(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleContentAction(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseActionPath(r.URL.Path, "/api/contents/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch action {
	case "approve":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req struct {
			Reviewer string `json:"reviewer"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Reviewer == "" {
			req.Reviewer = "小林"
		}
		content, err := s.service.Approve(r.Context(), id, req.Reviewer)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, content)
	case "reject":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req struct {
			Reviewer    string `json:"reviewer"`
			Reason      string `json:"reason"`
			Instruction string `json:"instruction"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Reviewer == "" {
			req.Reviewer = "小林"
		}
		content, err := s.service.RejectAndRegenerate(r.Context(), id, req.Reviewer, req.Reason, req.Instruction)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, content)
	case "save-draft":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		content, err := s.service.SaveDraft(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, content)
	case "schedule":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var req struct {
			ScheduledAt string `json:"scheduled_at"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		t, err := parseTime(req.ScheduledAt)
		if err != nil {
			writeError(w, err)
			return
		}
		task, err := s.service.Schedule(r.Context(), id, t)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, task)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handlePublishTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	tasks, err := s.store.ListPublishTasks(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleRunDue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	count, err := s.service.RunDuePublishes(r.Context(), time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"published": count})
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules, err := s.store.GetRules(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rules)
	case http.MethodPut:
		var rules AppRules
		if !decodeJSON(w, r, &rules) {
			return
		}
		updated, err := s.store.UpdateRules(r.Context(), rules)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.settingsPayload())
	case http.MethodPut:
		var req struct {
			AIProvider    string `json:"ai_provider"`
			OpenAIAPIKey  string `json:"openai_api_key"`
			OpenAIModel   string `json:"openai_model"`
			OpenAIBaseURL string `json:"openai_base_url"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		cfg := s.cfg
		previousProvider := cfg.AIProvider
		if strings.TrimSpace(req.AIProvider) != "" {
			cfg.AIProvider = strings.ToLower(strings.TrimSpace(req.AIProvider))
		}
		if strings.TrimSpace(req.OpenAIAPIKey) != "" {
			cfg.OpenAIAPIKey = strings.TrimSpace(req.OpenAIAPIKey)
		}
		if strings.TrimSpace(req.OpenAIModel) != "" {
			cfg.OpenAIModel = strings.TrimSpace(req.OpenAIModel)
		}
		if strings.TrimSpace(req.OpenAIBaseURL) != "" {
			cfg.OpenAIBaseURL = strings.TrimRight(strings.TrimSpace(req.OpenAIBaseURL), "/")
		}
		if cfg.AIProvider == "" {
			cfg.AIProvider = "openai"
		}
		providerChanged := previousProvider != cfg.AIProvider
		if providerChanged && strings.TrimSpace(req.OpenAIAPIKey) == "" {
			cfg.OpenAIAPIKey = storedProviderAPIKey(cfg.AIProvider)
		}
		switch cfg.AIProvider {
		case "deepseek":
			if cfg.OpenAIModel == "" || cfg.OpenAIModel == "local" || strings.HasPrefix(cfg.OpenAIModel, "gpt-") || providerChanged && strings.TrimSpace(req.OpenAIModel) == "" {
				cfg.OpenAIModel = "deepseek-v4-flash"
			}
			if cfg.OpenAIBaseURL == "" || strings.Contains(cfg.OpenAIBaseURL, "api.openai.com") {
				cfg.OpenAIBaseURL = "https://api.deepseek.com"
			}
		case "local":
			cfg.OpenAIModel = "local"
			cfg.OpenAIBaseURL = ""
		default:
			cfg.AIProvider = "openai"
			if cfg.OpenAIModel == "" || cfg.OpenAIModel == "local" || strings.HasPrefix(cfg.OpenAIModel, "deepseek-") || providerChanged && strings.TrimSpace(req.OpenAIModel) == "" {
				cfg.OpenAIModel = "gpt-4o-mini"
			}
			if cfg.OpenAIBaseURL == "" || strings.Contains(cfg.OpenAIBaseURL, "deepseek.com") {
				cfg.OpenAIBaseURL = "https://api.openai.com"
			}
		}
		if ai, ok := s.service.ai.(*ReloadableContentAI); ok {
			ai.Update(cfg)
		} else {
			s.service.ai = NewConfiguredContentAI(cfg)
		}
		s.cfg = cfg
		_ = os.Setenv("AI_PROVIDER", cfg.AIProvider)
		_ = os.Setenv("OPENAI_MODEL", cfg.OpenAIModel)
		_ = os.Setenv("OPENAI_BASE_URL", cfg.OpenAIBaseURL)
		updates := map[string]string{
			"AI_PROVIDER":     cfg.AIProvider,
			"OPENAI_MODEL":    cfg.OpenAIModel,
			"OPENAI_BASE_URL": cfg.OpenAIBaseURL,
		}
		if cfg.AIProvider != "local" {
			keyEnv := providerAPIKeyEnv(cfg.AIProvider)
			_ = os.Setenv(keyEnv, cfg.OpenAIAPIKey)
			updates[keyEnv] = cfg.OpenAIAPIKey
		}
		if err := updateDotEnv(".env", updates); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.settingsPayload())
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) settingsPayload() map[string]any {
	configured := s.cfg.AIProvider == "local" || strings.TrimSpace(s.cfg.OpenAIAPIKey) != ""
	aiStatus := map[string]any{
		"ai_provider":       s.cfg.AIProvider,
		"api_configured":    configured,
		"openai_configured": configured,
		"openai_model":      s.cfg.OpenAIModel,
		"openai_base_url":   s.cfg.OpenAIBaseURL,
	}
	if ai, ok := s.service.ai.(*ReloadableContentAI); ok {
		aiStatus = ai.Status()
	}
	return map[string]any{
		"ai": aiStatus,
		"xhs": map[string]any{
			"adapter":        s.cfg.XHSAdapterMode,
			"web_remote_url": s.cfg.XHSWebRemoteURL,
		},
	}
}

func (s *Server) handleXHSStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	adapter := "mock"
	keywordSearch := false
	materialUpload := false
	switch s.service.adapter.(type) {
	case XHSOpenAPIAdapter:
		adapter = "openapi"
		materialUpload = true
	case XHSWebAdapter:
		adapter = "web"
		keywordSearch = true
	}
	aiProvider := "local"
	apiConfigured := false
	openAIModel := ""
	if ai, ok := s.service.ai.(*ReloadableContentAI); ok {
		status := ai.Status()
		aiProvider, _ = status["ai_provider"].(string)
		apiConfigured, _ = status["api_configured"].(bool)
		openAIModel, _ = status["openai_model"].(string)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"adapter":                   adapter,
		"keyword_search_web":        keywordSearch,
		"keyword_search_official":   false,
		"material_upload":           materialUpload,
		"draft_endpoint_required":   true,
		"publish_endpoint_required": true,
		"ai_provider":               aiProvider,
		"api_configured":            apiConfigured,
		"openai_configured":         apiConfigured,
		"openai_model":              openAIModel,
	})
}

func (s *Server) handleXHSMaterials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	adapter, ok := s.service.adapter.(XHSOpenAPIAdapter)
	if !ok {
		writeError(w, errors.New("当前为 mock adapter，请设置 XHS_ADAPTER=openapi 后再调用素材接口"))
		return
	}
	var req MaterialUploadRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Type) == "" {
		writeError(w, errors.New("name 和 type 必填"))
		return
	}
	resp, err := adapter.UploadMaterial(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseActionPath(path, prefix string) (int64, string, bool) {
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return 0, "", false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return id, parts[1], true
}

func parseTime(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, errors.New("scheduled_at 不能为空")
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02T15:04:05.999999"} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("时间格式应为 RFC3339 或 2006-01-02 15:04")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, ErrNotFound) {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
