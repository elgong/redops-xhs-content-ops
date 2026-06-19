package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	xhsURLPattern         = regexp.MustCompile(`https?://[^\s"'<>]*(?:xiaohongshu\.com|xhslink\.com)[^\s"'<>]*`)
	numberBeforeLabelRE   = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?(?:万|千|k|m)?)(浏览|观看|阅读|播放|赞|点赞|喜欢|评论|回复|收藏)`)
	labelBeforeNumberRE   = regexp.MustCompile(`(?i)(浏览量|浏览|观看量|观看|阅读量|阅读|播放量|播放|点赞数|点赞|赞|喜欢|评论数|评论|回复数|回复|收藏数|收藏)\s*[:：]?\s*([0-9]+(?:\.[0-9]+)?\s*(?:万|千|k|m)?)(?:\s|$|[,，、])`)
	commonMetricLineRE    = regexp.MustCompile(`(?i)(浏览|观看|阅读|播放|点赞|赞|喜欢|评论|回复|收藏)\s*[:：]?\s*[0-9]`)
	commonNavigationTexts = map[string]bool{
		"全部": true, "图文": true, "视频": true, "用户": true, "筛选": true, "首页": true,
		"发现": true, "直播": true, "发布": true, "通知": true, "登录": true, "扫码": true,
	}
)

func ParseXHSWebText(raw string) ([]SourcePost, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if raw == "" {
		return nil, fmt.Errorf("网页文本不能为空")
	}
	lines := cleanImportLines(raw)
	blocks := splitImportBlocks(lines)
	seen := map[string]bool{}
	now := time.Now()
	posts := make([]SourcePost, 0, len(blocks))
	for i, block := range blocks {
		post, ok := parseImportBlock(block, now, i)
		if !ok {
			continue
		}
		key := strings.TrimSpace(post.URL + "|" + post.Title)
		if seen[key] {
			continue
		}
		seen[key] = true
		posts = append(posts, post)
	}
	if len(posts) == 0 {
		return nil, fmt.Errorf("未识别到可导入的笔记，请复制包含标题、互动数或链接的搜索结果文本")
	}
	return posts, nil
}

func cleanImportLines(raw string) []string {
	items := strings.Split(raw, "\n")
	lines := make([]string, 0, len(items))
	for _, item := range items {
		line := strings.TrimSpace(strings.Join(strings.Fields(item), " "))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitImportBlocks(lines []string) [][]string {
	var blocks [][]string
	current := make([]string, 0, 10)
	push := func() {
		if len(current) >= 2 {
			blocks = append(blocks, current)
		}
		current = make([]string, 0, 10)
	}
	for _, line := range lines {
		if xhsURLPattern.MatchString(line) && len(current) > 0 {
			current = append(current, line)
			push()
			continue
		}
		if isLikelyBlockStart(line) && len(current) >= 4 {
			push()
		}
		current = append(current, line)
		if len(current) >= 12 {
			push()
		}
	}
	push()
	if len(blocks) == 0 && len(lines) > 0 {
		blocks = append(blocks, lines)
	}
	return blocks
}

func parseImportBlock(lines []string, now time.Time, index int) (SourcePost, bool) {
	var title string
	summaryParts := make([]string, 0, 3)
	url := ""
	views, likes, comments, favorites := 0, 0, 0, 0
	for _, line := range lines {
		if url == "" {
			if match := xhsURLPattern.FindString(line); match != "" {
				url = strings.TrimRight(match, "，。)")
			}
		}
		applyMetricLine(line, &views, &likes, &comments, &favorites)
		if !isUsefulContentLine(line) {
			continue
		}
		if title == "" {
			title = line
			continue
		}
		if len(summaryParts) < 3 && line != title {
			summaryParts = append(summaryParts, line)
		}
	}
	if title == "" {
		return SourcePost{}, false
	}
	if url == "" {
		url = fmt.Sprintf("webtext://xhs/%d/%d", now.UnixNano(), index)
	}
	summary := strings.Join(summaryParts, "；")
	if summary == "" {
		summary = "网页文本导入样本"
	}
	return SourcePost{
		Title:          trimRunes(title, 120),
		ContentSummary: trimRunes(summary, 360),
		URL:            url,
		Views:          views,
		Likes:          likes,
		Comments:       comments,
		Favorites:      favorites,
		PublishedAt:    now,
		CapturedAt:     now,
	}, true
}

func applyMetricLine(line string, views, likes, comments, favorites *int) {
	for _, match := range labelBeforeNumberRE.FindAllStringSubmatch(line, -1) {
		setMetric(match[1], parseMetricNumber(match[2]), views, likes, comments, favorites)
	}
	for _, match := range numberBeforeLabelRE.FindAllStringSubmatch(line, -1) {
		setMetric(match[2], parseMetricNumber(match[1]), views, likes, comments, favorites)
	}
}

func setMetric(label string, value int, views, likes, comments, favorites *int) {
	label = strings.ToLower(label)
	switch {
	case strings.Contains(label, "浏览") || strings.Contains(label, "观看") || strings.Contains(label, "阅读") || strings.Contains(label, "播放"):
		*views = maxInt(*views, value)
	case strings.Contains(label, "赞") || strings.Contains(label, "喜欢"):
		*likes = maxInt(*likes, value)
	case strings.Contains(label, "评论") || strings.Contains(label, "回复"):
		*comments = maxInt(*comments, value)
	case strings.Contains(label, "收藏"):
		*favorites = maxInt(*favorites, value)
	}
}

func parseMetricNumber(raw string) int {
	raw = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(raw, ",", "")))
	multiplier := 1.0
	switch {
	case strings.HasSuffix(raw, "万"):
		multiplier = 10000
		raw = strings.TrimSuffix(raw, "万")
	case strings.HasSuffix(raw, "千"):
		multiplier = 1000
		raw = strings.TrimSuffix(raw, "千")
	case strings.HasSuffix(raw, "k"):
		multiplier = 1000
		raw = strings.TrimSuffix(raw, "k")
	case strings.HasSuffix(raw, "m"):
		multiplier = 1000000
		raw = strings.TrimSuffix(raw, "m")
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return int(v * multiplier)
}

func isLikelyBlockStart(line string) bool {
	return isUsefulContentLine(line) && !commonMetricLineRE.MatchString(line) && utf8.RuneCountInString(line) >= 6
}

func isUsefulContentLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || xhsURLPattern.MatchString(line) || commonNavigationTexts[line] {
		return false
	}
	if commonMetricLineRE.MatchString(line) || labelBeforeNumberRE.MatchString(line) || numberBeforeLabelRE.MatchString(line) {
		return false
	}
	count := utf8.RuneCountInString(line)
	if count < 4 || count > 160 {
		return false
	}
	return true
}

func trimRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
