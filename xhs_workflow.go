package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const xhsWorkflowPageCount = 6

type xhsWorkflowCard struct {
	Badge    string
	Heading  string
	Blocks   []string
	Footer   string
	Accent   string
	Template string
}

func BuildXHSWorkflowPackage(ctx context.Context, content GeneratedContent, outputRoot string) (XHSWorkflowPackage, error) {
	if strings.TrimSpace(outputRoot) == "" {
		outputRoot = os.ExpandEnv("$HOME/Documents/red/xhs-output")
	}
	slug := safeSlug(content.Title)
	if slug == "" {
		slug = fmt.Sprintf("content-%d", content.ID)
	}
	dir := filepath.Join(outputRoot, fmt.Sprintf("%03d-%s", content.ID, slug))
	exportsDir := filepath.Join(dir, "exports")
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		return XHSWorkflowPackage{}, err
	}
	cards := buildXHSWorkflowCards(content)
	contentPath := filepath.Join(dir, "content.md")
	htmlPath := filepath.Join(dir, "cards.html")
	if err := os.WriteFile(contentPath, []byte(renderXHSWorkflowMarkdown(content, cards)), 0o644); err != nil {
		return XHSWorkflowPackage{}, err
	}
	if err := os.WriteFile(htmlPath, []byte(renderXHSWorkflowHTML(content, cards)), 0o644); err != nil {
		return XHSWorkflowPackage{}, err
	}

	exportPaths := existingXHSExports(exportsDir)
	pkg := XHSWorkflowPackage{
		ContentID:   content.ID,
		Title:       content.Title,
		Dir:         dir,
		ContentPath: contentPath,
		HTMLPath:    htmlPath,
		ExportPaths: exportPaths,
		PreviewURL:  "/xhs-output/" + url.PathEscape(filepath.Base(dir)) + "/cards.html",
		ExportsURL:  "/xhs-output/" + url.PathEscape(filepath.Base(dir)) + "/exports/",
		Exported:    len(exportPaths) == len(cards),
		PageCount:   len(cards),
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
	if len(exportPaths) < len(cards) {
		pkg.ExportError = "PNG 导出未完成；可先使用预览页查看图文包"
	}
	_ = saveXHSWorkflowPackageIndex(outputRoot, pkg)
	return pkg, nil
}

func dynamicXHSWorkflowPackage(content GeneratedContent) XHSWorkflowPackage {
	return XHSWorkflowPackage{
		ContentID:   content.ID,
		Title:       content.Title,
		PreviewURL:  fmt.Sprintf("/api/contents/%d/xhs-preview", content.ID),
		ExportsURL:  "",
		Exported:    false,
		PageCount:   xhsWorkflowPageCount,
		GeneratedAt: time.Now().Format(time.RFC3339),
		ExportError: "当前为平台动态预览；PNG 导出将作为后台任务接入",
	}
}

func existingXHSExports(exportsDir string) []string {
	exports := make([]string, 0, xhsWorkflowPageCount)
	for i := 1; i <= xhsWorkflowPageCount; i++ {
		p := filepath.Join(exportsDir, fmt.Sprintf("page-%d.png", i))
		if info, err := os.Stat(p); err == nil && info.Size() > 0 {
			exports = append(exports, p)
		}
	}
	return exports
}

func ListXHSWorkflowPackages(outputRoot string) ([]XHSWorkflowPackage, error) {
	if strings.TrimSpace(outputRoot) == "" {
		outputRoot = os.ExpandEnv("$HOME/Documents/red/xhs-output")
	}
	data, err := os.ReadFile(filepath.Join(outputRoot, "packages.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return []XHSWorkflowPackage{}, nil
		}
		return nil, err
	}
	var items []XHSWorkflowPackage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func saveXHSWorkflowPackageIndex(outputRoot string, pkg XHSWorkflowPackage) error {
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return err
	}
	items, err := ListXHSWorkflowPackages(outputRoot)
	if err != nil {
		items = []XHSWorkflowPackage{}
	}
	next := []XHSWorkflowPackage{pkg}
	for _, item := range items {
		if item.Dir != pkg.Dir {
			next = append(next, item)
		}
	}
	if len(next) > 100 {
		next = next[:100]
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputRoot, "packages.json"), append(data, '\n'), 0o644)
}

func buildXHSWorkflowCards(content GeneratedContent) []xhsWorkflowCard {
	title := strings.TrimSpace(content.Title)
	if title == "" {
		title = "小红书图文素材"
	}
	blocks := splitXHSBlocks(content.Body)
	if len(blocks) == 0 {
		blocks = []string{"把核心观点拆成更容易保存和转发的图文卡片。"}
	}
	tags := strings.TrimSpace(content.Tags)
	if tags == "" {
		tags = "#真实分享 #经验总结"
	}
	cards := []xhsWorkflowCard{
		{Badge: "01 / 06", Heading: trimRunes(title, 28), Blocks: []string{pickBlock(blocks, 0), "适合先收藏，实际用一次会更有感觉。"}, Footer: tags, Accent: "teal", Template: "cover"},
		{Badge: "02 / 06", Heading: "先抓住重点", Blocks: []string{pickBlock(blocks, 0), pickBlock(blocks, 1)}, Footer: "先理解，再执行", Accent: "coral", Template: "normal"},
		{Badge: "03 / 06", Heading: "1 先看场景", Blocks: []string{pickBlock(blocks, 2), pickBlock(blocks, 3)}, Footer: "把抽象工具放进真实任务", Accent: "teal", Template: "normal"},
		{Badge: "04 / 06", Heading: "2 给出标准", Blocks: []string{pickBlock(blocks, 4), "目标、边界、验收标准越清楚，结果越稳。"}, Footer: "标准越明确，返工越少", Accent: "yellow", Template: "check"},
		{Badge: "05 / 06", Heading: "3 避免跑偏", Blocks: []string{pickBlock(blocks, 5), pickBlock(blocks, 6)}, Footer: "先 review，再动手", Accent: "blue", Template: "normal"},
		{Badge: "06 / 06", Heading: "最后这句最有用", Blocks: []string{pickBlock(blocks, len(blocks)-1), "给背景、给边界、给验收。别把它当搜索框，把它当协作开发。"}, Footer: "保存这张，下次直接套", Accent: "coral", Template: "summary"},
	}
	return cards
}

func splitXHSBlocks(body string) []string {
	clean := strings.ReplaceAll(strings.TrimSpace(body), "\r\n", "\n")
	parts := strings.Split(clean, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasPrefix(part, "本次生成要求") || strings.HasPrefix(part, "本次根据审核意见调整") {
			continue
		}
		if len([]rune(part)) > 90 {
			sentences := regexp.MustCompile(`[。！？!?]`).Split(part, -1)
			for _, sentence := range sentences {
				sentence = strings.TrimSpace(sentence)
				if sentence != "" {
					out = append(out, trimRunes(sentence, 62))
				}
			}
			continue
		}
		out = append(out, trimRunes(part, 76))
	}
	return out
}

func pickBlock(blocks []string, index int) string {
	if len(blocks) == 0 {
		return "把一个具体场景讲清楚，比堆功能更有用。"
	}
	if index < 0 {
		index = 0
	}
	if index >= len(blocks) {
		index = len(blocks) - 1
	}
	return blocks[index]
}

func renderXHSWorkflowMarkdown(content GeneratedContent, cards []xhsWorkflowCard) string {
	var b strings.Builder
	b.WriteString("# " + content.Title + "\n\n")
	b.WriteString("## Assumptions\n")
	b.WriteString("- Audience: 小红书上的工具效率、AI 编程和内容运营用户。\n")
	b.WriteString("- Goal: 从平台生成文案一键转成可发布图文包。\n")
	b.WriteString("- Tone: 真实分享、轻专业、少广告感。\n")
	b.WriteString("- Page count: 6 页。\n")
	b.WriteString("- Format: Checklist + step-by-step tutorial。\n\n")
	b.WriteString("## Title Options\n")
	b.WriteString("1. " + content.Title + "\n")
	b.WriteString("2. " + content.CoverText + "\n")
	b.WriteString("3. 这套方法我想早点知道\n\n")
	b.WriteString("## Card Script\n\n")
	for i, card := range cards {
		b.WriteString("### Page " + strconv.Itoa(i+1) + "\n")
		b.WriteString("- Heading: " + card.Heading + "\n")
		b.WriteString("- Body:\n")
		for _, block := range card.Blocks {
			b.WriteString("  - " + block + "\n")
		}
		b.WriteString("- Visual: " + card.Template + " card with " + card.Accent + " accent.\n\n")
	}
	b.WriteString("## Caption\n\n")
	b.WriteString(content.Body + "\n\n")
	b.WriteString("## Hashtags\n\n")
	b.WriteString(content.Tags + "\n\n")
	b.WriteString("## Alt Text\n")
	for i, card := range cards {
		b.WriteString("- page-" + strconv.Itoa(i+1) + ": " + card.Heading + "。\n")
	}
	return b.String()
}

func renderXHSWorkflowHTML(content GeneratedContent, cards []xhsWorkflowCard) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>`)
	b.WriteString(html.EscapeString(content.Title))
	b.WriteString(`</title><style>
:root{--ink:#172027;--muted:#5d6872;--paper:#fbfcf8;--line:#dde6df;--teal:#13a6a0;--coral:#f0644f;--yellow:#ffd166;--blue:#2f80ed;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",Arial,sans-serif}
*{box-sizing:border-box}body{margin:0;background:#e8eee9;color:var(--ink)}.stage{display:grid;gap:32px;padding:32px;justify-content:center}.card{position:relative;width:1080px;height:1440px;overflow:hidden;background:linear-gradient(90deg,rgba(19,166,160,.08) 1px,transparent 1px),linear-gradient(180deg,rgba(19,166,160,.08) 1px,transparent 1px),var(--paper);background-size:48px 48px;border:1px solid var(--line);padding:82px 76px}.card:before{content:"";position:absolute;inset:28px;border:2px solid rgba(23,32,39,.08);border-radius:28px;pointer-events:none}.hide{display:none}.topbar{display:flex;align-items:center;justify-content:space-between;margin-bottom:54px;font-size:26px;font-weight:800;color:var(--muted)}.badge{min-height:48px;padding:8px 24px;border-radius:999px;background:var(--ink);color:white;font-size:24px;font-weight:900}h1,h2,p{margin:0}h1{font-size:96px;line-height:1.08;letter-spacing:0;max-width:880px}h2{font-size:70px;line-height:1.12;letter-spacing:0;margin-bottom:42px}.lead{margin-top:32px;color:var(--muted);font-size:38px;line-height:1.5;font-weight:700}.blocks{display:grid;gap:26px;margin-top:36px}.block{border-radius:22px;padding:30px 34px;background:white;border:2px solid var(--line);font-size:36px;line-height:1.48;font-weight:700}.accent-teal{color:var(--teal)}.accent-coral{color:var(--coral)}.accent-yellow{color:#d99a00}.accent-blue{color:var(--blue)}.terminal{position:absolute;left:76px;right:76px;bottom:190px;border-radius:24px;background:#172027;color:white;padding:38px;font-size:32px;line-height:1.55;font-weight:800}.minirow{position:absolute;left:76px;right:76px;bottom:82px;display:grid;grid-template-columns:repeat(3,1fr);gap:18px}.mini{border-radius:18px;padding:22px;background:white;border:2px solid var(--line);font-size:27px;font-weight:900;text-align:center}.checklist{display:grid;gap:22px;margin-top:38px}.check{display:grid;grid-template-columns:62px 1fr;gap:22px;align-items:start;padding:28px 30px;border-radius:22px;background:white;border:2px solid var(--line)}.num{width:62px;height:62px;border-radius:18px;display:grid;place-items:center;background:var(--yellow);font-size:31px;font-weight:900}.check h3{margin:0 0 8px;font-size:36px}.check p{color:var(--muted);font-size:30px;line-height:1.42;font-weight:700}.quote{margin-top:44px;border-radius:26px;padding:36px;background:#fff8df;border:2px solid rgba(255,209,102,.75);font-size:36px;line-height:1.48;font-weight:850}.footer{position:absolute;left:76px;right:76px;bottom:64px;display:flex;justify-content:space-between;color:var(--muted);font-size:25px;font-weight:800}.footer span:last-child{color:var(--teal)}.summary{display:grid;gap:26px;margin-top:38px}.summary .block{font-size:38px}.tagline{position:absolute;left:76px;right:76px;bottom:170px;border-radius:26px;padding:34px;background:#172027;color:white;font-size:38px;line-height:1.45;font-weight:900}
</style></head><body><main class="stage">`)
	for i, card := range cards {
		b.WriteString(renderXHSWorkflowCard(i+1, card))
	}
	b.WriteString(`<script>const page=new URLSearchParams(location.search).get("page");if(page){document.querySelectorAll(".card").forEach(c=>c.classList.toggle("hide",c.dataset.page!==page));document.querySelector(".stage").style.padding="0";document.body.style.background="#fbfcf8";}</script></main></body></html>`)
	return b.String()
}

func renderXHSWorkflowCard(page int, card xhsWorkflowCard) string {
	accent := "accent-" + card.Accent
	if card.Template == "cover" {
		return fmt.Sprintf(`<section class="card" data-page="%d"><div class="topbar"><span>小红书图文工作流</span><span class="badge">%s</span></div><h1>%s</h1><p class="lead">%s</p><div class="terminal">$ workflow generate<br>$ export png<br><span style="color:#ffd166">content.md / cards.html / page-*.png</span></div><div class="minirow"><div class="mini">脚本</div><div class="mini">设计</div><div class="mini">导出</div></div><div class="footer"><span>%s</span><span>先收藏</span></div></section>`, page, html.EscapeString(card.Badge), html.EscapeString(card.Heading), html.EscapeString(pickBlock(card.Blocks, 0)), html.EscapeString(card.Footer))
	}
	var body strings.Builder
	if card.Template == "check" || card.Template == "summary" {
		body.WriteString(`<div class="checklist">`)
		for i, block := range card.Blocks {
			body.WriteString(fmt.Sprintf(`<div class="check"><div class="num">%d</div><div><h3>%s</h3><p>%s</p></div></div>`, i+1, html.EscapeString(shortHeading(block)), html.EscapeString(block)))
		}
		body.WriteString(`</div>`)
	} else {
		body.WriteString(`<div class="blocks">`)
		for _, block := range card.Blocks {
			body.WriteString(`<p class="block">` + html.EscapeString(block) + `</p>`)
		}
		body.WriteString(`</div>`)
	}
	return fmt.Sprintf(`<section class="card" data-page="%d"><div class="topbar"><span>%s</span><span class="badge">%s</span></div><h2><span class="%s">%s</span></h2>%s<div class="quote">%s</div><div class="footer"><span>%s</span><span>%02d</span></div></section>`, page, html.EscapeString(card.Footer), html.EscapeString(card.Badge), accent, html.EscapeString(card.Heading), body.String(), html.EscapeString(pickBlock(card.Blocks, len(card.Blocks)-1)), html.EscapeString(card.Footer), page)
}

func shortHeading(text string) string {
	r := []rune(strings.TrimSpace(text))
	if len(r) <= 10 {
		return string(r)
	}
	return string(r[:10])
}

func exportXHSWorkflowPNGs(ctx context.Context, htmlPath, exportsDir string, pageCount int) ([]string, error) {
	browser, err := findHeadlessBrowser()
	if err != nil {
		return nil, err
	}
	profileDir, err := os.MkdirTemp("", "redops-xhs-export-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(profileDir)
	paths := make([]string, 0, pageCount)
	allocatorOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browser),
		chromedp.UserDataDir(profileDir),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.WindowSize(1080, 1440),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocatorOptions...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	for i := 1; i <= pageCount; i++ {
		out := filepath.Join(exportsDir, fmt.Sprintf("page-%d.png", i))
		pageURL := (&url.URL{Scheme: "file", Path: htmlPath}).String() + "?page=" + strconv.Itoa(i)
		pageCtx, cancel := context.WithTimeout(browserCtx, 20*time.Second)
		var png []byte
		err := chromedp.Run(pageCtx,
			chromedp.EmulateViewport(1080, 1440),
			chromedp.Navigate(pageURL),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Sleep(250*time.Millisecond),
			chromedp.CaptureScreenshot(&png),
		)
		cancel()
		if err != nil {
			return paths, fmt.Errorf("export page %d: %w", i, err)
		}
		if pageCtx.Err() != nil {
			return paths, fmt.Errorf("export page %d timeout: %w", i, pageCtx.Err())
		}
		if len(png) == 0 {
			return paths, fmt.Errorf("export page %d: empty screenshot", i)
		}
		if err := os.WriteFile(out, png, 0o644); err != nil {
			return paths, err
		}
		paths = append(paths, out)
	}
	return paths, nil
}

func findHeadlessBrowser() (string, error) {
	candidates := []string{
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到可用于导出 PNG 的 Chrome/Edge")
}

func safeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "，", "-", "。", "", "！", "", "？", "")
	s = replacer.Replace(s)
	s = regexp.MustCompile(`[^a-z0-9\p{Han}-]+`).ReplaceAllString(s, "-")
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return trimRunes(s, 46)
}
