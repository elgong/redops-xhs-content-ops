package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type XHSOpenAPIAdapter struct {
	BaseURL         string
	AccessToken     string
	DraftEndpoint   string
	PublishEndpoint string
	Client          *http.Client
	fallback        MockXHSAdapter
}

func NewConfiguredXHSAdapter(cfg Config) XHSAdapter {
	switch cfg.XHSAdapterMode {
	case "web", "browser":
		return XHSWebAdapter{
			ProfileDir:  cfg.XHSWebProfileDir,
			BrowserPath: cfg.XHSWebBrowserPath,
			RemoteURL:   cfg.XHSWebRemoteURL,
			Headless:    cfg.XHSWebHeadless,
			Timeout:     45 * time.Second,
		}
	case "openapi":
		return XHSOpenAPIAdapter{
			BaseURL:         strings.TrimRight(cfg.XHSBaseURL, "/"),
			AccessToken:     cfg.XHSAccessToken,
			DraftEndpoint:   cfg.XHSDraftEndpoint,
			PublishEndpoint: cfg.XHSPublishEndpoint,
			Client:          &http.Client{Timeout: 15 * time.Second},
		}
	default:
		return MockXHSAdapter{}
	}
}

func (a XHSOpenAPIAdapter) Collect(ctx context.Context, task KeywordTask) ([]SourcePost, error) {
	return nil, fmt.Errorf("当前官方开放平台目录未提供通用关键词笔记搜索接口；请使用授权数据源/人工导入，或配置第三方合规数据供应商")
}

func (a XHSOpenAPIAdapter) SaveDraft(ctx context.Context, account Account, content GeneratedContent) (string, error) {
	if strings.TrimSpace(a.DraftEndpoint) == "" {
		return "", fmt.Errorf("未配置 XHS_DRAFT_ENDPOINT；官方公开目录未确认通用草稿保存接口")
	}
	var resp struct {
		DraftID string `json:"draft_id"`
		ID      string `json:"id"`
	}
	if err := a.postJSON(ctx, a.DraftEndpoint, map[string]any{
		"account": account.Name,
		"title":   content.Title,
		"body":    content.Body,
		"cover":   content.CoverText,
		"tags":    content.Tags,
	}, &resp); err != nil {
		return "", err
	}
	if resp.DraftID != "" {
		return resp.DraftID, nil
	}
	if resp.ID != "" {
		return resp.ID, nil
	}
	return "", fmt.Errorf("草稿接口未返回 draft_id")
}

func (a XHSOpenAPIAdapter) Publish(ctx context.Context, account Account, content GeneratedContent) (string, error) {
	if strings.TrimSpace(a.PublishEndpoint) == "" {
		return "", fmt.Errorf("未配置 XHS_PUBLISH_ENDPOINT；官方公开目录未确认通用笔记发布接口")
	}
	var resp struct {
		URL string `json:"url"`
		ID  string `json:"id"`
	}
	if err := a.postJSON(ctx, a.PublishEndpoint, map[string]any{
		"account":  account.Name,
		"draft_id": content.DraftID,
		"title":    content.Title,
		"body":     content.Body,
		"tags":     content.Tags,
	}, &resp); err != nil {
		return "", err
	}
	if resp.URL != "" {
		return resp.URL, nil
	}
	if resp.ID != "" {
		return "xhs://note/" + resp.ID, nil
	}
	return "", fmt.Errorf("发布接口未返回 url 或 id")
}

type MaterialUploadRequest struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	MaterialContent []string `json:"materialContent"`
}

type MaterialUploadResponse struct {
	MaterialID string `json:"materialId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	URL        string `json:"url"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Duration   int    `json:"duration"`
	Status     int    `json:"status"`
	CreateTime int64  `json:"createTime"`
	UpdateTime int64  `json:"updateTime"`
}

func (a XHSOpenAPIAdapter) UploadMaterial(ctx context.Context, req MaterialUploadRequest) (MaterialUploadResponse, error) {
	var resp MaterialUploadResponse
	err := a.postJSON(ctx, "/ark/open_api/v3/common_controller", req, &resp)
	return resp, err
}

func (a XHSOpenAPIAdapter) postJSON(ctx context.Context, endpoint string, payload any, dst any) error {
	if a.Client == nil {
		a.Client = &http.Client{Timeout: 15 * time.Second}
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return fmt.Errorf("endpoint 不能为空")
	}
	url := endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		url = strings.TrimRight(a.BaseURL, "/") + "/" + strings.TrimLeft(endpoint, "/")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.AccessToken)
		req.Header.Set("X-Access-Token", a.AccessToken)
	}
	res, err := a.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("小红书 OpenAPI 返回 HTTP %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(dst)
}
