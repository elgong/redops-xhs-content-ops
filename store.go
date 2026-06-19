package main

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	Close() error
	Seed(context.Context) error
	ListAccounts(context.Context) ([]Account, error)
	ListKeywords(context.Context) ([]KeywordTask, error)
	CreateKeyword(context.Context, KeywordTask) (KeywordTask, error)
	GetKeyword(context.Context, int64) (KeywordTask, error)
	UpdateKeyword(context.Context, KeywordTask) error
	AddSourcePosts(context.Context, []SourcePost) error
	ListSourcePosts(context.Context, int64, int) ([]SourcePost, error)
	SaveInsight(context.Context, InsightReport) (InsightReport, error)
	LatestInsight(context.Context, int64) (InsightReport, error)
	CreateContent(context.Context, GeneratedContent) (GeneratedContent, error)
	GetContent(context.Context, int64) (GeneratedContent, error)
	ListContents(context.Context, string) ([]GeneratedContent, error)
	UpdateContent(context.Context, GeneratedContent) error
	AddReview(context.Context, ReviewRecord) error
	CreatePublishTask(context.Context, PublishTask) (PublishTask, error)
	ListPublishTasks(context.Context) ([]PublishTask, error)
	DuePublishTasks(context.Context, time.Time) ([]PublishTask, error)
	UpdatePublishTask(context.Context, PublishTask) error
	Dashboard(context.Context) (Dashboard, error)
	GetRules(context.Context) (AppRules, error)
	UpdateRules(context.Context, AppRules) (AppRules, error)
}

type MemoryStore struct {
	mu           sync.RWMutex
	nextID       int64
	accounts     map[int64]Account
	keywords     map[int64]KeywordTask
	posts        map[int64]SourcePost
	insights     map[int64]InsightReport
	contents     map[int64]GeneratedContent
	reviews      map[int64]ReviewRecord
	publishTasks map[int64]PublishTask
	rules        AppRules
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextID:       1,
		accounts:     map[int64]Account{},
		keywords:     map[int64]KeywordTask{},
		posts:        map[int64]SourcePost{},
		insights:     map[int64]InsightReport{},
		contents:     map[int64]GeneratedContent{},
		reviews:      map[int64]ReviewRecord{},
		publishTasks: map[int64]PublishTask{},
	}
}

func (s *MemoryStore) Close() error { return nil }

func (s *MemoryStore) id() int64 {
	id := s.nextID
	s.nextID++
	return id
}

func (s *MemoryStore) Seed(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.accounts) > 0 {
		return nil
	}
	now := time.Now()
	for _, account := range []Account{
		{Name: "品牌号 A", AccountType: "品牌号", AuthStatus: "正常", DailyLimit: 8, TodayScheduled: 3, ExceptionStatus: "无异常"},
		{Name: "品牌号 B", AccountType: "企业号", AuthStatus: "正常", DailyLimit: 6, TodayScheduled: 1, ExceptionStatus: "无异常"},
		{Name: "达人号 C", AccountType: "个人号", AuthStatus: "待确认", DailyLimit: 3, TodayScheduled: 0, ExceptionStatus: "需登录"},
	} {
		account.ID = s.id()
		account.CreatedAt = now
		s.accounts[account.ID] = account
	}
	for _, task := range []KeywordTask{
		{Keyword: "护肤早C晚A", Category: "美妆", FrequencyMinutes: 30, SampleLimit: 30, Status: StatusRunning, CreatedBy: "运营"},
		{Keyword: "通勤包收纳", Category: "时尚", FrequencyMinutes: 60, SampleLimit: 20, Status: StatusRunning, CreatedBy: "运营"},
		{Keyword: "低卡早餐", Category: "食品", FrequencyMinutes: 120, SampleLimit: 20, Status: StatusPaused, CreatedBy: "运营"},
	} {
		task.ID = s.id()
		task.CreatedAt = now
		s.keywords[task.ID] = task
	}
	s.rules = AppRules{
		ID:              s.id(),
		BrandVoice:      "真实分享,轻专业,少广告感,朋友式提醒",
		BannedPhrases:   "保证见效,最有效,立刻变白,关注领取,私信购买,微信联系",
		ReviewRules:     "高风险内容必须二审,驳回必须填写原因,发布前再次检查重复度,商业合作内容必须标记",
		DefaultReviewer: "小林",
		UpdatedAt:       now,
	}
	return nil
}

func (s *MemoryStore) ListAccounts(ctx context.Context) ([]Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Account, 0, len(s.accounts))
	for _, v := range s.accounts {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *MemoryStore) ListKeywords(ctx context.Context) ([]KeywordTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]KeywordTask, 0, len(s.keywords))
	for _, v := range s.keywords {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *MemoryStore) CreateKeyword(ctx context.Context, task KeywordTask) (KeywordTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task.ID = s.id()
	task.CreatedAt = time.Now()
	if task.Status == "" {
		task.Status = StatusRunning
	}
	if task.CreatedBy == "" {
		task.CreatedBy = "运营"
	}
	s.keywords[task.ID] = task
	return task, nil
}

func (s *MemoryStore) GetKeyword(ctx context.Context, id int64) (KeywordTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.keywords[id]
	if !ok {
		return KeywordTask{}, ErrNotFound
	}
	return v, nil
}

func (s *MemoryStore) UpdateKeyword(ctx context.Context, task KeywordTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keywords[task.ID]; !ok {
		return ErrNotFound
	}
	s.keywords[task.ID] = task
	return nil
}

func (s *MemoryStore) AddSourcePosts(ctx context.Context, posts []SourcePost) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, post := range posts {
		post.ID = s.id()
		if post.CapturedAt.IsZero() {
			post.CapturedAt = time.Now()
		}
		s.posts[post.ID] = post
	}
	return nil
}

func (s *MemoryStore) ListSourcePosts(ctx context.Context, taskID int64, limit int) ([]SourcePost, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []SourcePost{}
	for _, v := range s.posts {
		if v.KeywordTaskID == taskID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].HotScore > out[j].HotScore })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *MemoryStore) SaveInsight(ctx context.Context, report InsightReport) (InsightReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	report.ID = s.id()
	report.CreatedAt = time.Now()
	s.insights[report.ID] = report
	return report, nil
}

func (s *MemoryStore) LatestInsight(ctx context.Context, taskID int64) (InsightReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out InsightReport
	for _, v := range s.insights {
		if v.KeywordTaskID == taskID && v.ID > out.ID {
			out = v
		}
	}
	if out.ID == 0 {
		return InsightReport{}, ErrNotFound
	}
	return out, nil
}

func (s *MemoryStore) CreateContent(ctx context.Context, content GeneratedContent) (GeneratedContent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	content.ID = s.id()
	content.CreatedAt = now
	content.UpdatedAt = now
	if content.Status == "" {
		content.Status = ContentPendingReview
	}
	if content.Version == 0 {
		content.Version = 1
	}
	s.contents[content.ID] = content
	return s.decorateContent(content), nil
}

func (s *MemoryStore) GetContent(ctx context.Context, id int64) (GeneratedContent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.contents[id]
	if !ok {
		return GeneratedContent{}, ErrNotFound
	}
	return s.decorateContent(v), nil
}

func (s *MemoryStore) ListContents(ctx context.Context, status string) ([]GeneratedContent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []GeneratedContent{}
	for _, v := range s.contents {
		if status == "" || v.Status == status {
			out = append(out, s.decorateContent(v))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *MemoryStore) UpdateContent(ctx context.Context, content GeneratedContent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.contents[content.ID]; !ok {
		return ErrNotFound
	}
	content.UpdatedAt = time.Now()
	s.contents[content.ID] = content
	return nil
}

func (s *MemoryStore) AddReview(ctx context.Context, record ReviewRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record.ID = s.id()
	record.ReviewedAt = time.Now()
	s.reviews[record.ID] = record
	return nil
}

func (s *MemoryStore) CreatePublishTask(ctx context.Context, task PublishTask) (PublishTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	task.ID = s.id()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = PublishPending
	}
	s.publishTasks[task.ID] = task
	return s.decoratePublishTask(task), nil
}

func (s *MemoryStore) ListPublishTasks(ctx context.Context) ([]PublishTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []PublishTask{}
	for _, v := range s.publishTasks {
		out = append(out, s.decoratePublishTask(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScheduledAt.Before(out[j].ScheduledAt) })
	return out, nil
}

func (s *MemoryStore) DuePublishTasks(ctx context.Context, now time.Time) ([]PublishTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []PublishTask{}
	for _, v := range s.publishTasks {
		if v.Status == PublishPending && !v.ScheduledAt.After(now) {
			out = append(out, s.decoratePublishTask(v))
		}
	}
	return out, nil
}

func (s *MemoryStore) UpdatePublishTask(ctx context.Context, task PublishTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.publishTasks[task.ID]; !ok {
		return ErrNotFound
	}
	task.UpdatedAt = time.Now()
	s.publishTasks[task.ID] = task
	return nil
}

func (s *MemoryStore) Dashboard(ctx context.Context) (Dashboard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var d Dashboard
	for _, p := range s.posts {
		if p.CapturedAt.After(start) {
			d.CollectedToday++
		}
	}
	for _, c := range s.contents {
		switch c.Status {
		case ContentPendingReview:
			d.PendingReview++
		case ContentScheduled:
			d.Scheduled++
		case ContentPublishFailed:
			d.PublishFailed++
		}
	}
	for _, k := range s.keywords {
		d.Keywords = append(d.Keywords, k)
	}
	for _, c := range s.contents {
		d.Contents = append(d.Contents, s.decorateContent(c))
	}
	for _, p := range s.publishTasks {
		d.PublishTasks = append(d.PublishTasks, s.decoratePublishTask(p))
	}
	sort.Slice(d.Keywords, func(i, j int) bool { return d.Keywords[i].ID < d.Keywords[j].ID })
	sort.Slice(d.Contents, func(i, j int) bool { return d.Contents[i].UpdatedAt.After(d.Contents[j].UpdatedAt) })
	sort.Slice(d.PublishTasks, func(i, j int) bool { return d.PublishTasks[i].ScheduledAt.Before(d.PublishTasks[j].ScheduledAt) })
	if len(d.Contents) > 6 {
		d.Contents = d.Contents[:6]
	}
	if len(d.PublishTasks) > 8 {
		d.PublishTasks = d.PublishTasks[:8]
	}
	return d, nil
}

func (s *MemoryStore) GetRules(ctx context.Context) (AppRules, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.rules.ID == 0 {
		return defaultRules(), nil
	}
	return s.rules, nil
}

func (s *MemoryStore) UpdateRules(ctx context.Context, rules AppRules) (AppRules, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rules.ID == 0 {
		rules.ID = s.id()
	}
	rules.UpdatedAt = time.Now()
	s.rules = rules
	return rules, nil
}

func (s *MemoryStore) decorateContent(content GeneratedContent) GeneratedContent {
	if k, ok := s.keywords[content.KeywordTaskID]; ok {
		content.Keyword = k.Keyword
	}
	if a, ok := s.accounts[content.AccountID]; ok {
		content.AccountName = a.Name
	}
	return content
}

func (s *MemoryStore) decoratePublishTask(task PublishTask) PublishTask {
	if c, ok := s.contents[task.ContentID]; ok {
		task.Title = c.Title
	}
	if a, ok := s.accounts[task.AccountID]; ok {
		task.AccountName = a.Name
	}
	return task
}

func defaultRules() AppRules {
	return AppRules{
		ID:              1,
		BrandVoice:      "真实分享,轻专业,少广告感,朋友式提醒",
		BannedPhrases:   "保证见效,最有效,立刻变白,关注领取,私信购买,微信联系",
		ReviewRules:     "高风险内容必须二审,驳回必须填写原因,发布前再次检查重复度,商业合作内容必须标记",
		DefaultReviewer: "小林",
		UpdatedAt:       time.Now(),
	}
}

func containsAny(text, csv string) bool {
	for _, item := range strings.Split(csv, ",") {
		item = strings.TrimSpace(item)
		if item != "" && strings.Contains(text, item) {
			return true
		}
	}
	return false
}
