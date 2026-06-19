package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type XHSWebAdapter struct {
	ProfileDir  string
	BrowserPath string
	RemoteURL   string
	Headless    bool
	Timeout     time.Duration
}

type xhsWebNote struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Date   string `json:"date"`
	Likes  string `json:"likes"`
	URL    string `json:"url"`
}

func (a XHSWebAdapter) Collect(ctx context.Context, task KeywordTask) ([]SourcePost, error) {
	if strings.TrimSpace(a.ProfileDir) == "" {
		a.ProfileDir = os.ExpandEnv("$HOME/.redops/xhs-browser-profile")
	}
	if a.Timeout <= 0 {
		a.Timeout = 45 * time.Second
	}
	if err := os.MkdirAll(a.ProfileDir, 0o755); err != nil {
		return nil, err
	}
	browserCtx, cancelBrowser, err := a.newBrowserContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancelBrowser()
	runCtx, cancelRun := context.WithTimeout(browserCtx, a.Timeout)
	defer cancelRun()

	searchURL := "https://www.xiaohongshu.com/search_result/?keyword=" + url.QueryEscape(task.Keyword) + "&type=51"
	var bodyText string
	var rawNotes string
	err = chromedp.Run(runCtx,
		chromedp.Navigate(searchURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(6*time.Second),
		chromedp.Text("body", &bodyText, chromedp.ByQuery),
		chromedp.Evaluate(xhsExtractNotesJS(), &rawNotes),
	)
	if err != nil {
		return nil, err
	}
	if strings.Contains(bodyText, "登录后查看搜索结果") || strings.Contains(bodyText, "扫码") {
		return nil, fmt.Errorf("小红书网页登录态未就绪：请先运行 ./xhs-login.sh 登录一次，再重新采集")
	}
	var notes []xhsWebNote
	if err := json.Unmarshal([]byte(rawNotes), &notes); err != nil {
		return nil, err
	}
	limit := task.SampleLimit
	if limit <= 0 || limit > 30 {
		limit = 20
	}
	now := time.Now()
	posts := make([]SourcePost, 0, limit)
	seen := map[string]bool{}
	for _, note := range notes {
		if len(posts) >= limit {
			break
		}
		title := strings.TrimSpace(note.Title)
		if title == "" || seen[note.URL+"|"+title] {
			continue
		}
		seen[note.URL+"|"+title] = true
		likes := parseMetricNumber(note.Likes)
		publishedAt := parseXHSPublishedAt(note.Date, now)
		posts = append(posts, SourcePost{
			Title:          trimRunes(title, 120),
			ContentSummary: trimRunes(fmt.Sprintf("作者：%s；搜索结果公开互动数：%s", note.Author, note.Likes), 360),
			URL:            strings.TrimSpace(note.URL),
			Likes:          likes,
			PublishedAt:    publishedAt,
			CapturedAt:     now,
			HotScore:       hotScore(0, likes, 0, 0, publishedAt),
		})
	}
	if len(posts) == 0 {
		return nil, fmt.Errorf("未从小红书搜索页解析到笔记卡片，请确认账号已登录且搜索页有结果")
	}
	return posts, nil
}

func (a XHSWebAdapter) newBrowserContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if remoteURL := strings.TrimSpace(a.RemoteURL); remoteURL != "" {
		allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, remoteURL)
		browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
		return browserCtx, func() {
			cancelBrowser()
			cancelAlloc()
		}, nil
	}
	allocatorOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(a.ProfileDir),
		chromedp.Flag("headless", a.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.WindowSize(1280, 900),
	)
	if strings.TrimSpace(a.BrowserPath) != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(a.BrowserPath))
	} else if browserPath := detectMacBrowserPath(); browserPath != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(browserPath))
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocatorOptions...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	return browserCtx, func() {
		cancelBrowser()
		cancelAlloc()
	}, nil
}

func detectMacBrowserPath() string {
	for _, path := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func (a XHSWebAdapter) SaveDraft(ctx context.Context, account Account, content GeneratedContent) (string, error) {
	return "", fmt.Errorf("网页登录态适配器当前只负责搜索采集；保存草稿请配置官方 XHS_DRAFT_ENDPOINT")
}

func (a XHSWebAdapter) Publish(ctx context.Context, account Account, content GeneratedContent) (string, error) {
	return "", fmt.Errorf("网页登录态适配器当前只负责搜索采集；发布请配置官方 XHS_PUBLISH_ENDPOINT")
}

func xhsExtractNotesJS() string {
	return `(() => {
  const clean = (s) => String(s || '').trim().replace(/\s+/g, ' ');
  const isDate = (s) => /^(\d{4}-\d{2}-\d{2}|\d{2}-\d{2}|昨天\s+\d{2}:\d{2}|今天\s+\d{2}:\d{2})$/.test(clean(s));
  const sections = Array.from(document.querySelectorAll('section.note-item'));
  const notes = sections.map((section) => {
    const text = clean(section.innerText || section.textContent || '');
    const href = (section.querySelector('a[href*="/explore/"]') || section.querySelector('a[href*="/search_result/"]') || {}).href || '';
    const parts = text.split(' ').filter(Boolean);
    let dateIdx = parts.findIndex(isDate);
    if (dateIdx < 0 || dateIdx === 0) return null;
    const like = parts[dateIdx + 1] || '';
    const author = parts[dateIdx - 1] || '';
    const title = parts.slice(0, Math.max(1, dateIdx - 1)).join(' ');
    return { title, author, date: parts[dateIdx], likes: like, url: href };
  }).filter(Boolean);
  return JSON.stringify(notes);
})()`
}

func parseXHSPublishedAt(raw string, now time.Time) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return now
	}
	if strings.HasPrefix(raw, "昨天") {
		return now.AddDate(0, 0, -1)
	}
	if strings.HasPrefix(raw, "今天") {
		return now
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, time.Local); err == nil {
		return t
	}
	if t, err := time.ParseInLocation("01-02", raw, time.Local); err == nil {
		return time.Date(now.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.Local)
	}
	return now
}
