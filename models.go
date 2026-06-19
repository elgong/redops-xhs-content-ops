package main

import "time"

const (
	StatusRunning         = "running"
	StatusPaused          = "paused"
	StatusError           = "error"
	ContentGenerated      = "generated"
	ContentPendingReview  = "pending_review"
	ContentApproved       = "approved"
	ContentRejected       = "rejected"
	ContentRegenerating   = "regenerating"
	ContentDraftSaving    = "draft_saving"
	ContentDraftSaved     = "draft_saved"
	ContentScheduled      = "scheduled"
	ContentPublished      = "published"
	ContentPublishFailed  = "publish_failed"
	PublishPending        = "pending"
	PublishRunning        = "running"
	PublishSuccess        = "success"
	PublishFailed         = "failed"
	ReviewDecisionApprove = "approve"
	ReviewDecisionReject  = "reject"
)

type Config struct {
	Addr               string
	MySQLDSN           string
	AutoMigrate        bool
	SeedData           bool
	SchedulerEnabled   bool
	XHSAdapterMode     string
	XHSBaseURL         string
	XHSAccessToken     string
	XHSDraftEndpoint   string
	XHSPublishEndpoint string
	XHSWebProfileDir   string
	XHSWebBrowserPath  string
	XHSWebRemoteURL    string
	XHSWebHeadless     bool
	AIProvider         string
	OpenAIAPIKey       string
	OpenAIModel        string
	OpenAIBaseURL      string
}

type Account struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	AccountType     string     `json:"account_type"`
	AuthStatus      string     `json:"auth_status"`
	DailyLimit      int        `json:"daily_limit"`
	TodayScheduled  int        `json:"today_scheduled"`
	LastPublishedAt *time.Time `json:"last_published_at,omitempty"`
	ExceptionStatus string     `json:"exception_status"`
	CreatedAt       time.Time  `json:"created_at"`
}

type KeywordTask struct {
	ID               int64      `json:"id"`
	Keyword          string     `json:"keyword"`
	Category         string     `json:"category"`
	FrequencyMinutes int        `json:"frequency_minutes"`
	SampleLimit      int        `json:"sample_limit"`
	Status           string     `json:"status"`
	LastCollectedAt  *time.Time `json:"last_collected_at,omitempty"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
}

type SourcePost struct {
	ID             int64     `json:"id"`
	KeywordTaskID  int64     `json:"keyword_task_id"`
	Title          string    `json:"title"`
	ContentSummary string    `json:"content_summary"`
	URL            string    `json:"url"`
	Views          int       `json:"views"`
	Likes          int       `json:"likes"`
	Comments       int       `json:"comments"`
	Favorites      int       `json:"favorites"`
	HotScore       float64   `json:"hot_score"`
	PublishedAt    time.Time `json:"published_at"`
	CapturedAt     time.Time `json:"captured_at"`
}

type InsightReport struct {
	ID                 int64     `json:"id"`
	KeywordTaskID      int64     `json:"keyword_task_id"`
	AverageHotScore    float64   `json:"average_hot_score"`
	TitlePatterns      string    `json:"title_patterns"`
	StyleTags          string    `json:"style_tags"`
	AudiencePainPoints string    `json:"audience_pain_points"`
	RiskHints          string    `json:"risk_hints"`
	CreatedAt          time.Time `json:"created_at"`
}

type GeneratedContent struct {
	ID             int64      `json:"id"`
	KeywordTaskID  int64      `json:"keyword_task_id"`
	AccountID      int64      `json:"account_id"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	CoverText      string     `json:"cover_text"`
	Tags           string     `json:"tags"`
	RiskLevel      string     `json:"risk_level"`
	DuplicateScore float64    `json:"duplicate_score"`
	Status         string     `json:"status"`
	Version        int        `json:"version"`
	DraftID        string     `json:"draft_id"`
	PublishedURL   string     `json:"published_url"`
	ScheduledAt    *time.Time `json:"scheduled_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Keyword        string     `json:"keyword,omitempty"`
	AccountName    string     `json:"account_name,omitempty"`
}

type ReviewRecord struct {
	ID                    int64     `json:"id"`
	ContentID             int64     `json:"content_id"`
	ReviewerID            string    `json:"reviewer_id"`
	Decision              string    `json:"decision"`
	RejectReason          string    `json:"reject_reason"`
	RegenerateInstruction string    `json:"regenerate_instruction"`
	ReviewedAt            time.Time `json:"reviewed_at"`
}

type PublishTask struct {
	ID            int64     `json:"id"`
	ContentID     int64     `json:"content_id"`
	AccountID     int64     `json:"account_id"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	Status        string    `json:"status"`
	FailureReason string    `json:"failure_reason"`
	PublishedURL  string    `json:"published_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Title         string    `json:"title,omitempty"`
	AccountName   string    `json:"account_name,omitempty"`
}

type AppRules struct {
	ID              int64     `json:"id"`
	BrandVoice      string    `json:"brand_voice"`
	BannedPhrases   string    `json:"banned_phrases"`
	ReviewRules     string    `json:"review_rules"`
	DefaultReviewer string    `json:"default_reviewer"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Dashboard struct {
	CollectedToday int64              `json:"collected_today"`
	PendingReview  int64              `json:"pending_review"`
	Scheduled      int64              `json:"scheduled"`
	PublishFailed  int64              `json:"publish_failed"`
	Keywords       []KeywordTask      `json:"keywords"`
	Contents       []GeneratedContent `json:"contents"`
	PublishTasks   []PublishTask      `json:"publish_tasks"`
}
