package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

type XHSAdapter interface {
	Collect(ctx context.Context, task KeywordTask) ([]SourcePost, error)
	SaveDraft(ctx context.Context, account Account, content GeneratedContent) (string, error)
	Publish(ctx context.Context, account Account, content GeneratedContent) (string, error)
}

type MockXHSAdapter struct{}

func (MockXHSAdapter) Collect(ctx context.Context, task KeywordTask) ([]SourcePost, error) {
	now := time.Now()
	templates := []string{
		"%s真的别乱做，我踩过坑",
		"%s新手最容易忽略的顺序",
		"%s这样整理后轻松很多",
		"后悔没早点知道的%s清单",
		"%s普通人也能照着做",
	}
	n := task.SampleLimit
	if n <= 0 || n > 30 {
		n = 12
	}
	posts := make([]SourcePost, 0, n)
	for i := 0; i < n; i++ {
		views := 1800 + rand.Intn(160000)
		likes := int(float64(views) * (0.018 + rand.Float64()*0.08))
		comments := int(float64(views) * (0.002 + rand.Float64()*0.012))
		favorites := int(float64(views) * (0.006 + rand.Float64()*0.03))
		score := hotScore(views, likes, comments, favorites, now.Add(-time.Duration(i)*time.Hour*6))
		title := fmt.Sprintf(templates[i%len(templates)], task.Keyword)
		posts = append(posts, SourcePost{
			KeywordTaskID:  task.ID,
			Title:          title,
			ContentSummary: "围绕用户痛点、个人体验和清单步骤展开，评论关注适用人群、预算、操作顺序和避坑。",
			URL:            fmt.Sprintf("mock://xhs/%d/%d", task.ID, time.Now().UnixNano()+int64(i)),
			Views:          views,
			Likes:          likes,
			Comments:       comments,
			Favorites:      favorites,
			HotScore:       score,
			PublishedAt:    now.Add(-time.Duration(i+1) * time.Hour * time.Duration(2+rand.Intn(8))),
			CapturedAt:     now,
		})
	}
	return posts, nil
}

func (MockXHSAdapter) SaveDraft(ctx context.Context, account Account, content GeneratedContent) (string, error) {
	if !accountReady(account.AuthStatus) {
		return "", fmt.Errorf("账号授权状态为%s，无法保存草稿", account.AuthStatus)
	}
	return fmt.Sprintf("draft_%d_%d", content.ID, time.Now().Unix()), nil
}

func (MockXHSAdapter) Publish(ctx context.Context, account Account, content GeneratedContent) (string, error) {
	if !accountReady(account.AuthStatus) {
		return "", fmt.Errorf("账号授权状态为%s，无法发布", account.AuthStatus)
	}
	if content.DraftID == "" {
		return "", fmt.Errorf("内容尚未保存草稿")
	}
	return fmt.Sprintf("https://www.xiaohongshu.com/mock/%d", content.ID), nil
}

func accountReady(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "正常", "normal", "ok", "active", "authorized", "verified":
		return true
	default:
		return false
	}
}

type AppService struct {
	store   Store
	adapter XHSAdapter
	ai      ContentAI
}

func NewAppService(store Store, adapter XHSAdapter, ai ContentAI) *AppService {
	return &AppService{store: store, adapter: adapter, ai: ai}
}

func (s *AppService) Collect(ctx context.Context, taskID int64) ([]SourcePost, error) {
	task, err := s.store.GetKeyword(ctx, taskID)
	if err != nil {
		return nil, err
	}
	posts, err := s.adapter.Collect(ctx, task)
	if err != nil {
		task.Status = StatusError
		_ = s.store.UpdateKeyword(ctx, task)
		return nil, err
	}
	now := time.Now()
	for i := range posts {
		posts[i].KeywordTaskID = taskID
		if posts[i].CapturedAt.IsZero() {
			posts[i].CapturedAt = now
		}
		if posts[i].PublishedAt.IsZero() {
			posts[i].PublishedAt = now
		}
		if posts[i].HotScore == 0 {
			posts[i].HotScore = hotScore(posts[i].Views, posts[i].Likes, posts[i].Comments, posts[i].Favorites, posts[i].PublishedAt)
		}
	}
	task.LastCollectedAt = &now
	task.Status = StatusRunning
	if err := s.store.AddSourcePosts(ctx, posts); err != nil {
		return nil, err
	}
	return posts, s.store.UpdateKeyword(ctx, task)
}

func (s *AppService) ImportPosts(ctx context.Context, taskID int64, posts []SourcePost) ([]SourcePost, error) {
	task, err := s.store.GetKeyword(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(posts) == 0 {
		return nil, fmt.Errorf("导入样本不能为空")
	}
	now := time.Now()
	for i := range posts {
		posts[i].KeywordTaskID = taskID
		if strings.TrimSpace(posts[i].Title) == "" {
			return nil, fmt.Errorf("第%d条样本标题不能为空", i+1)
		}
		if strings.TrimSpace(posts[i].ContentSummary) == "" {
			posts[i].ContentSummary = "人工导入样本"
		}
		if strings.TrimSpace(posts[i].URL) == "" {
			posts[i].URL = fmt.Sprintf("manual://keyword/%d/%d", taskID, now.UnixNano()+int64(i))
		}
		if posts[i].PublishedAt.IsZero() {
			posts[i].PublishedAt = now
		}
		posts[i].CapturedAt = now
		posts[i].HotScore = hotScore(posts[i].Views, posts[i].Likes, posts[i].Comments, posts[i].Favorites, posts[i].PublishedAt)
	}
	task.LastCollectedAt = &now
	task.Status = StatusRunning
	if err := s.store.AddSourcePosts(ctx, posts); err != nil {
		return nil, err
	}
	return posts, s.store.UpdateKeyword(ctx, task)
}

func (s *AppService) Analyze(ctx context.Context, taskID int64) (InsightReport, error) {
	posts, err := s.store.ListSourcePosts(ctx, taskID, 50)
	if err != nil {
		return InsightReport{}, err
	}
	if len(posts) == 0 {
		if _, err := s.Collect(ctx, taskID); err != nil {
			return InsightReport{}, err
		}
		posts, err = s.store.ListSourcePosts(ctx, taskID, 50)
		if err != nil {
			return InsightReport{}, err
		}
	}
	task, err := s.store.GetKeyword(ctx, taskID)
	if err != nil {
		return InsightReport{}, err
	}
	report, err := s.ai.Analyze(ctx, task, posts)
	if err != nil {
		return InsightReport{}, err
	}
	report.KeywordTaskID = taskID
	return s.store.SaveInsight(ctx, report)
}

func (s *AppService) Generate(ctx context.Context, taskID, accountID int64, instruction string) (GeneratedContent, error) {
	task, err := s.store.GetKeyword(ctx, taskID)
	if err != nil {
		return GeneratedContent{}, err
	}
	insight, err := s.store.LatestInsight(ctx, taskID)
	if err != nil {
		insight, err = s.Analyze(ctx, taskID)
		if err != nil {
			return GeneratedContent{}, err
		}
	}
	rules, _ := s.store.GetRules(ctx)
	accounts, err := s.store.ListAccounts(ctx)
	if err != nil {
		return GeneratedContent{}, err
	}
	account, ok := findAccount(accounts, accountID)
	if !ok {
		return GeneratedContent{}, fmt.Errorf("账号不存在，请先绑定账号")
	}
	content, err := s.ai.Generate(ctx, task, insight, rules, account, instruction)
	if err != nil {
		return GeneratedContent{}, err
	}
	content.KeywordTaskID = taskID
	content.AccountID = accountID
	content.Status = ContentPendingReview
	return s.store.CreateContent(ctx, content)
}

func (s *AppService) Approve(ctx context.Context, contentID int64, reviewer string) (GeneratedContent, error) {
	content, err := s.store.GetContent(ctx, contentID)
	if err != nil {
		return GeneratedContent{}, err
	}
	content.Status = ContentApproved
	if err := s.store.UpdateContent(ctx, content); err != nil {
		return GeneratedContent{}, err
	}
	_ = s.store.AddReview(ctx, ReviewRecord{ContentID: contentID, ReviewerID: reviewer, Decision: ReviewDecisionApprove})
	return content, nil
}

func (s *AppService) RejectAndRegenerate(ctx context.Context, contentID int64, reviewer, reason, instruction string) (GeneratedContent, error) {
	if strings.TrimSpace(reason) == "" {
		return GeneratedContent{}, fmt.Errorf("驳回原因不能为空")
	}
	content, err := s.store.GetContent(ctx, contentID)
	if err != nil {
		return GeneratedContent{}, err
	}
	content.Status = ContentRejected
	if err := s.store.UpdateContent(ctx, content); err != nil {
		return GeneratedContent{}, err
	}
	_ = s.store.AddReview(ctx, ReviewRecord{
		ContentID:             contentID,
		ReviewerID:            reviewer,
		Decision:              ReviewDecisionReject,
		RejectReason:          reason,
		RegenerateInstruction: instruction,
	})
	return s.Generate(ctx, content.KeywordTaskID, content.AccountID, strings.TrimSpace(reason+" "+instruction))
}

func (s *AppService) SaveDraft(ctx context.Context, contentID int64) (GeneratedContent, error) {
	content, err := s.store.GetContent(ctx, contentID)
	if err != nil {
		return GeneratedContent{}, err
	}
	if content.Status != ContentApproved && content.Status != ContentDraftSaved {
		return GeneratedContent{}, fmt.Errorf("内容状态为%s，必须审核通过后才能保存草稿", content.Status)
	}
	accounts, err := s.store.ListAccounts(ctx)
	if err != nil {
		return GeneratedContent{}, err
	}
	account, ok := findAccount(accounts, content.AccountID)
	if !ok {
		return GeneratedContent{}, ErrNotFound
	}
	previousStatus := content.Status
	content.Status = ContentDraftSaving
	_ = s.store.UpdateContent(ctx, content)
	draftID, err := s.adapter.SaveDraft(ctx, account, content)
	if err != nil {
		content.Status = previousStatus
		_ = s.store.UpdateContent(ctx, content)
		return GeneratedContent{}, err
	}
	content.DraftID = draftID
	content.Status = ContentDraftSaved
	return content, s.store.UpdateContent(ctx, content)
}

func (s *AppService) Schedule(ctx context.Context, contentID int64, scheduledAt time.Time) (PublishTask, error) {
	content, err := s.store.GetContent(ctx, contentID)
	if err != nil {
		return PublishTask{}, err
	}
	if content.Status != ContentDraftSaved && content.Status != ContentScheduled {
		return PublishTask{}, fmt.Errorf("内容必须已保存草稿才能排期")
	}
	content.Status = ContentScheduled
	content.ScheduledAt = &scheduledAt
	if err := s.store.UpdateContent(ctx, content); err != nil {
		return PublishTask{}, err
	}
	return s.store.CreatePublishTask(ctx, PublishTask{
		ContentID:   contentID,
		AccountID:   content.AccountID,
		ScheduledAt: scheduledAt,
		Status:      PublishPending,
	})
}

func (s *AppService) RunDuePublishes(ctx context.Context, now time.Time) (int, error) {
	tasks, err := s.store.DuePublishTasks(ctx, now)
	if err != nil {
		return 0, err
	}
	accounts, err := s.store.ListAccounts(ctx)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, task := range tasks {
		task.Status = PublishRunning
		_ = s.store.UpdatePublishTask(ctx, task)
		content, err := s.store.GetContent(ctx, task.ContentID)
		if err != nil {
			task.Status = PublishFailed
			task.FailureReason = err.Error()
			_ = s.store.UpdatePublishTask(ctx, task)
			continue
		}
		account, ok := findAccount(accounts, task.AccountID)
		if !ok {
			task.Status = PublishFailed
			task.FailureReason = "账号不存在"
			_ = s.store.UpdatePublishTask(ctx, task)
			continue
		}
		url, err := s.adapter.Publish(ctx, account, content)
		if err != nil {
			task.Status = PublishFailed
			task.FailureReason = err.Error()
			content.Status = ContentPublishFailed
			_ = s.store.UpdateContent(ctx, content)
			_ = s.store.UpdatePublishTask(ctx, task)
			continue
		}
		task.Status = PublishSuccess
		task.PublishedURL = url
		content.Status = ContentPublished
		content.PublishedURL = url
		_ = s.store.UpdateContent(ctx, content)
		_ = s.store.UpdatePublishTask(ctx, task)
		done++
	}
	return done, nil
}

func findAccount(accounts []Account, id int64) (Account, bool) {
	for _, account := range accounts {
		if account.ID == id {
			return account, true
		}
	}
	return Account{}, false
}

func hotScore(views, likes, comments, favorites int, publishedAt time.Time) float64 {
	if views <= 0 {
		engagements := likes + comments*3 + favorites*2
		if engagements <= 0 {
			return 0
		}
		hours := math.Max(time.Since(publishedAt).Hours(), 1)
		freshness := math.Max(0, 1-hours/168)
		score := math.Log(float64(engagements)+1)*9 + freshness*18
		return round(math.Min(score, 100))
	}
	likeRate := float64(likes) / float64(views)
	commentRate := float64(comments) / float64(views)
	favoriteRate := float64(favorites) / float64(views)
	hours := math.Max(time.Since(publishedAt).Hours(), 1)
	freshness := math.Max(0, 1-hours/168)
	score := math.Log(float64(views))*7 + likeRate*220 + commentRate*360 + favoriteRate*260 + freshness*12
	return round(math.Min(score, 100))
}

func round(v float64) float64 {
	return math.Round(v*100) / 100
}
