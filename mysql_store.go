package main

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

func (s *MySQLStore) Close() error { return s.db.Close() }

func (s *MySQLStore) Seed(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		for _, a := range []Account{
			{Name: "品牌号 A", AccountType: "品牌号", AuthStatus: "正常", DailyLimit: 8, TodayScheduled: 3, ExceptionStatus: "无异常"},
			{Name: "品牌号 B", AccountType: "企业号", AuthStatus: "正常", DailyLimit: 6, TodayScheduled: 1, ExceptionStatus: "无异常"},
			{Name: "达人号 C", AccountType: "个人号", AuthStatus: "待确认", DailyLimit: 3, TodayScheduled: 0, ExceptionStatus: "需登录"},
		} {
			if _, err := s.db.ExecContext(ctx, `INSERT INTO accounts (name, account_type, auth_status, daily_limit, today_scheduled, exception_status) VALUES (?,?,?,?,?,?)`, a.Name, a.AccountType, a.AuthStatus, a.DailyLimit, a.TodayScheduled, a.ExceptionStatus); err != nil {
				return err
			}
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM keyword_tasks`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		for _, k := range []KeywordTask{
			{Keyword: "护肤早C晚A", Category: "美妆", FrequencyMinutes: 30, SampleLimit: 30, Status: StatusRunning, CreatedBy: "运营"},
			{Keyword: "通勤包收纳", Category: "时尚", FrequencyMinutes: 60, SampleLimit: 20, Status: StatusRunning, CreatedBy: "运营"},
			{Keyword: "低卡早餐", Category: "食品", FrequencyMinutes: 120, SampleLimit: 20, Status: StatusPaused, CreatedBy: "运营"},
		} {
			if _, err := s.db.ExecContext(ctx, `INSERT INTO keyword_tasks (keyword, category, frequency_minutes, sample_limit, status, created_by) VALUES (?,?,?,?,?,?)`, k.Keyword, k.Category, k.FrequencyMinutes, k.SampleLimit, k.Status, k.CreatedBy); err != nil {
				return err
			}
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_rules`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		r := defaultRules()
		_, err := s.db.ExecContext(ctx, `INSERT INTO app_rules (brand_voice, banned_phrases, review_rules, default_reviewer) VALUES (?,?,?,?)`, r.BrandVoice, r.BannedPhrases, r.ReviewRules, r.DefaultReviewer)
		return err
	}
	return nil
}

func (s *MySQLStore) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, account_type, auth_status, daily_limit, today_scheduled, last_published_at, exception_status, created_at FROM accounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		var a Account
		var last sql.NullTime
		if err := rows.Scan(&a.ID, &a.Name, &a.AccountType, &a.AuthStatus, &a.DailyLimit, &a.TodayScheduled, &last, &a.ExceptionStatus, &a.CreatedAt); err != nil {
			return nil, err
		}
		if last.Valid {
			a.LastPublishedAt = &last.Time
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateAccount(ctx context.Context, a Account) (Account, error) {
	if a.AccountType == "" {
		a.AccountType = "品牌号"
	}
	if a.AuthStatus == "" {
		a.AuthStatus = "待绑定"
	}
	if a.DailyLimit == 0 {
		a.DailyLimit = 8
	}
	if a.ExceptionStatus == "" {
		a.ExceptionStatus = "未验证"
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO accounts (name, account_type, auth_status, daily_limit, today_scheduled, exception_status) VALUES (?,?,?,?,?,?)`, a.Name, a.AccountType, a.AuthStatus, a.DailyLimit, a.TodayScheduled, a.ExceptionStatus)
	if err != nil {
		return Account{}, err
	}
	a.ID, _ = res.LastInsertId()
	return s.getAccount(ctx, a.ID)
}

func (s *MySQLStore) getAccount(ctx context.Context, id int64) (Account, error) {
	var a Account
	var last sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id, name, account_type, auth_status, daily_limit, today_scheduled, last_published_at, exception_status, created_at FROM accounts WHERE id=?`, id).Scan(&a.ID, &a.Name, &a.AccountType, &a.AuthStatus, &a.DailyLimit, &a.TodayScheduled, &last, &a.ExceptionStatus, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if last.Valid {
		a.LastPublishedAt = &last.Time
	}
	return a, err
}

func (s *MySQLStore) ListKeywords(ctx context.Context) ([]KeywordTask, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, keyword, category, frequency_minutes, sample_limit, status, last_collected_at, created_by, created_at FROM keyword_tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KeywordTask{}
	for rows.Next() {
		task, err := scanKeyword(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateKeyword(ctx context.Context, task KeywordTask) (KeywordTask, error) {
	if task.Status == "" {
		task.Status = StatusRunning
	}
	if task.CreatedBy == "" {
		task.CreatedBy = "运营"
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO keyword_tasks (keyword, category, frequency_minutes, sample_limit, status, created_by) VALUES (?,?,?,?,?,?)`, task.Keyword, task.Category, task.FrequencyMinutes, task.SampleLimit, task.Status, task.CreatedBy)
	if err != nil {
		return KeywordTask{}, err
	}
	task.ID, _ = res.LastInsertId()
	return s.GetKeyword(ctx, task.ID)
}

func (s *MySQLStore) GetKeyword(ctx context.Context, id int64) (KeywordTask, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, keyword, category, frequency_minutes, sample_limit, status, last_collected_at, created_by, created_at FROM keyword_tasks WHERE id=?`, id)
	task, err := scanKeyword(row)
	if errors.Is(err, sql.ErrNoRows) {
		return KeywordTask{}, ErrNotFound
	}
	return task, err
}

func (s *MySQLStore) UpdateKeyword(ctx context.Context, task KeywordTask) error {
	_, err := s.db.ExecContext(ctx, `UPDATE keyword_tasks SET keyword=?, category=?, frequency_minutes=?, sample_limit=?, status=?, last_collected_at=?, created_by=? WHERE id=?`, task.Keyword, task.Category, task.FrequencyMinutes, task.SampleLimit, task.Status, task.LastCollectedAt, task.CreatedBy, task.ID)
	return err
}

func (s *MySQLStore) AddSourcePosts(ctx context.Context, posts []SourcePost) error {
	for _, p := range posts {
		_, err := s.db.ExecContext(ctx, `INSERT INTO source_posts (keyword_task_id, title, content_summary, url, views, likes, comments, favorites, hot_score, published_at, captured_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, p.KeywordTaskID, p.Title, p.ContentSummary, p.URL, p.Views, p.Likes, p.Comments, p.Favorites, p.HotScore, p.PublishedAt, p.CapturedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) ListSourcePosts(ctx context.Context, taskID int64, limit int) ([]SourcePost, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, keyword_task_id, title, content_summary, url, views, likes, comments, favorites, hot_score, published_at, captured_at FROM source_posts WHERE keyword_task_id=? ORDER BY hot_score DESC LIMIT ?`, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SourcePost{}
	for rows.Next() {
		var p SourcePost
		if err := rows.Scan(&p.ID, &p.KeywordTaskID, &p.Title, &p.ContentSummary, &p.URL, &p.Views, &p.Likes, &p.Comments, &p.Favorites, &p.HotScore, &p.PublishedAt, &p.CapturedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *MySQLStore) SaveInsight(ctx context.Context, r InsightReport) (InsightReport, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO insight_reports (keyword_task_id, average_hot_score, title_patterns, style_tags, audience_pain_points, risk_hints) VALUES (?,?,?,?,?,?)`, r.KeywordTaskID, r.AverageHotScore, r.TitlePatterns, r.StyleTags, r.AudiencePainPoints, r.RiskHints)
	if err != nil {
		return InsightReport{}, err
	}
	r.ID, _ = res.LastInsertId()
	r.CreatedAt = time.Now()
	return r, nil
}

func (s *MySQLStore) LatestInsight(ctx context.Context, taskID int64) (InsightReport, error) {
	var r InsightReport
	err := s.db.QueryRowContext(ctx, `SELECT id, keyword_task_id, average_hot_score, title_patterns, style_tags, audience_pain_points, risk_hints, created_at FROM insight_reports WHERE keyword_task_id=? ORDER BY id DESC LIMIT 1`, taskID).Scan(&r.ID, &r.KeywordTaskID, &r.AverageHotScore, &r.TitlePatterns, &r.StyleTags, &r.AudiencePainPoints, &r.RiskHints, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return InsightReport{}, ErrNotFound
	}
	return r, err
}

func (s *MySQLStore) CreateContent(ctx context.Context, c GeneratedContent) (GeneratedContent, error) {
	if c.Status == "" {
		c.Status = ContentPendingReview
	}
	if c.Version == 0 {
		c.Version = 1
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO generated_contents (keyword_task_id, account_id, title, body, cover_text, tags, risk_level, duplicate_score, status, version, draft_id, published_url, scheduled_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.KeywordTaskID, c.AccountID, c.Title, c.Body, c.CoverText, c.Tags, c.RiskLevel, c.DuplicateScore, c.Status, c.Version, c.DraftID, c.PublishedURL, c.ScheduledAt)
	if err != nil {
		return GeneratedContent{}, err
	}
	c.ID, _ = res.LastInsertId()
	return s.GetContent(ctx, c.ID)
}

func (s *MySQLStore) GetContent(ctx context.Context, id int64) (GeneratedContent, error) {
	query := contentSelect() + ` WHERE c.id=?`
	row := s.db.QueryRowContext(ctx, query, id)
	c, err := scanContent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return GeneratedContent{}, ErrNotFound
	}
	return c, err
}

func (s *MySQLStore) ListContents(ctx context.Context, status string) ([]GeneratedContent, error) {
	query := contentSelect()
	args := []any{}
	if status != "" {
		query += ` WHERE c.status=?`
		args = append(args, status)
	}
	query += ` ORDER BY c.updated_at DESC LIMIT 100`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GeneratedContent{}
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *MySQLStore) UpdateContent(ctx context.Context, c GeneratedContent) error {
	_, err := s.db.ExecContext(ctx, `UPDATE generated_contents SET keyword_task_id=?, account_id=?, title=?, body=?, cover_text=?, tags=?, risk_level=?, duplicate_score=?, status=?, version=?, draft_id=?, published_url=?, scheduled_at=? WHERE id=?`, c.KeywordTaskID, c.AccountID, c.Title, c.Body, c.CoverText, c.Tags, c.RiskLevel, c.DuplicateScore, c.Status, c.Version, c.DraftID, c.PublishedURL, c.ScheduledAt, c.ID)
	return err
}

func (s *MySQLStore) AddReview(ctx context.Context, r ReviewRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO review_records (content_id, reviewer_id, decision, reject_reason, regenerate_instruction) VALUES (?,?,?,?,?)`, r.ContentID, r.ReviewerID, r.Decision, r.RejectReason, r.RegenerateInstruction)
	return err
}

func (s *MySQLStore) CreatePublishTask(ctx context.Context, p PublishTask) (PublishTask, error) {
	if p.Status == "" {
		p.Status = PublishPending
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO publish_tasks (content_id, account_id, scheduled_at, status, failure_reason, published_url) VALUES (?,?,?,?,?,?)`, p.ContentID, p.AccountID, p.ScheduledAt, p.Status, p.FailureReason, p.PublishedURL)
	if err != nil {
		return PublishTask{}, err
	}
	p.ID, _ = res.LastInsertId()
	return p, nil
}

func (s *MySQLStore) ListPublishTasks(ctx context.Context) ([]PublishTask, error) {
	return s.queryPublishTasks(ctx, ` ORDER BY p.scheduled_at ASC LIMIT 100`)
}

func (s *MySQLStore) DuePublishTasks(ctx context.Context, now time.Time) ([]PublishTask, error) {
	return s.queryPublishTasks(ctx, ` WHERE p.status='pending' AND p.scheduled_at<=? ORDER BY p.scheduled_at ASC`, now)
}

func (s *MySQLStore) UpdatePublishTask(ctx context.Context, p PublishTask) error {
	_, err := s.db.ExecContext(ctx, `UPDATE publish_tasks SET status=?, failure_reason=?, published_url=? WHERE id=?`, p.Status, p.FailureReason, p.PublishedURL, p.ID)
	return err
}

func (s *MySQLStore) Dashboard(ctx context.Context) (Dashboard, error) {
	var d Dashboard
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM source_posts WHERE captured_at >= CURDATE()`).Scan(&d.CollectedToday)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM generated_contents WHERE status=?`, ContentPendingReview).Scan(&d.PendingReview)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM generated_contents WHERE status=?`, ContentScheduled).Scan(&d.Scheduled)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM generated_contents WHERE status=?`, ContentPublishFailed).Scan(&d.PublishFailed)
	d.Keywords, _ = s.ListKeywords(ctx)
	d.Contents, _ = s.ListContents(ctx, "")
	d.PublishTasks, _ = s.ListPublishTasks(ctx)
	if len(d.Contents) > 6 {
		d.Contents = d.Contents[:6]
	}
	if len(d.PublishTasks) > 8 {
		d.PublishTasks = d.PublishTasks[:8]
	}
	return d, nil
}

func (s *MySQLStore) GetRules(ctx context.Context) (AppRules, error) {
	var r AppRules
	err := s.db.QueryRowContext(ctx, `SELECT id, brand_voice, banned_phrases, review_rules, default_reviewer, updated_at FROM app_rules ORDER BY id DESC LIMIT 1`).Scan(&r.ID, &r.BrandVoice, &r.BannedPhrases, &r.ReviewRules, &r.DefaultReviewer, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultRules(), nil
	}
	return r, err
}

func (s *MySQLStore) UpdateRules(ctx context.Context, r AppRules) (AppRules, error) {
	if r.ID == 0 {
		_, err := s.db.ExecContext(ctx, `INSERT INTO app_rules (brand_voice, banned_phrases, review_rules, default_reviewer) VALUES (?,?,?,?)`, r.BrandVoice, r.BannedPhrases, r.ReviewRules, r.DefaultReviewer)
		if err != nil {
			return AppRules{}, err
		}
		return s.GetRules(ctx)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE app_rules SET brand_voice=?, banned_phrases=?, review_rules=?, default_reviewer=? WHERE id=?`, r.BrandVoice, r.BannedPhrases, r.ReviewRules, r.DefaultReviewer, r.ID)
	if err != nil {
		return AppRules{}, err
	}
	return s.GetRules(ctx)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanKeyword(row rowScanner) (KeywordTask, error) {
	var task KeywordTask
	var last sql.NullTime
	err := row.Scan(&task.ID, &task.Keyword, &task.Category, &task.FrequencyMinutes, &task.SampleLimit, &task.Status, &last, &task.CreatedBy, &task.CreatedAt)
	if last.Valid {
		task.LastCollectedAt = &last.Time
	}
	return task, err
}

func contentSelect() string {
	return `SELECT c.id, c.keyword_task_id, c.account_id, c.title, c.body, c.cover_text, c.tags, c.risk_level, c.duplicate_score, c.status, c.version, c.draft_id, c.published_url, c.scheduled_at, c.created_at, c.updated_at, k.keyword, a.name
	FROM generated_contents c
	JOIN keyword_tasks k ON k.id=c.keyword_task_id
	JOIN accounts a ON a.id=c.account_id`
}

func scanContent(row rowScanner) (GeneratedContent, error) {
	var c GeneratedContent
	var scheduled sql.NullTime
	err := row.Scan(&c.ID, &c.KeywordTaskID, &c.AccountID, &c.Title, &c.Body, &c.CoverText, &c.Tags, &c.RiskLevel, &c.DuplicateScore, &c.Status, &c.Version, &c.DraftID, &c.PublishedURL, &scheduled, &c.CreatedAt, &c.UpdatedAt, &c.Keyword, &c.AccountName)
	if scheduled.Valid {
		c.ScheduledAt = &scheduled.Time
	}
	return c, err
}

func (s *MySQLStore) queryPublishTasks(ctx context.Context, suffix string, args ...any) ([]PublishTask, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.content_id, p.account_id, p.scheduled_at, p.status, p.failure_reason, p.published_url, p.created_at, p.updated_at, c.title, a.name
	FROM publish_tasks p
	JOIN generated_contents c ON c.id=p.content_id
	JOIN accounts a ON a.id=p.account_id`+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublishTask{}
	for rows.Next() {
		var p PublishTask
		if err := rows.Scan(&p.ID, &p.ContentID, &p.AccountID, &p.ScheduledAt, &p.Status, &p.FailureReason, &p.PublishedURL, &p.CreatedAt, &p.UpdatedAt, &p.Title, &p.AccountName); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
