package main

import (
	"context"
	"errors"
	"testing"
)

type failingDraftAdapter struct {
	MockXHSAdapter
}

func (failingDraftAdapter) SaveDraft(ctx context.Context, account Account, content GeneratedContent) (string, error) {
	return "", errors.New("draft endpoint unavailable")
}

func TestSaveDraftFailureRestoresPreviousStatus(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	account, err := store.CreateAccount(ctx, Account{Name: "验收账号", AuthStatus: "正常", DailyLimit: 8})
	if err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	content, err := store.CreateContent(ctx, GeneratedContent{
		AccountID: account.ID,
		Title:     "待保存草稿内容",
		Body:      "body",
		Status:    ContentApproved,
	})
	if err != nil {
		t.Fatalf("CreateContent returned error: %v", err)
	}
	service := NewAppService(store, failingDraftAdapter{}, LocalContentAI{})

	if _, err := service.SaveDraft(ctx, content.ID); err == nil {
		t.Fatalf("SaveDraft expected error")
	}
	updated, err := store.GetContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("GetContent returned error: %v", err)
	}
	if updated.Status != ContentApproved {
		t.Fatalf("expected status %q after draft failure, got %q", ContentApproved, updated.Status)
	}
	if updated.DraftID != "" {
		t.Fatalf("expected empty draft id, got %q", updated.DraftID)
	}
}

func TestSaveDraftSuccessMarksDraftSaved(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	account, err := store.CreateAccount(ctx, Account{Name: "验收账号", AuthStatus: "正常", DailyLimit: 8})
	if err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}
	content, err := store.CreateContent(ctx, GeneratedContent{
		AccountID: account.ID,
		Title:     "待保存草稿内容",
		Body:      "body",
		Status:    ContentApproved,
	})
	if err != nil {
		t.Fatalf("CreateContent returned error: %v", err)
	}
	service := NewAppService(store, MockXHSAdapter{}, LocalContentAI{})

	updated, err := service.SaveDraft(ctx, content.ID)
	if err != nil {
		t.Fatalf("SaveDraft returned error: %v", err)
	}
	if updated.Status != ContentDraftSaved {
		t.Fatalf("expected status %q, got %q", ContentDraftSaved, updated.Status)
	}
	if updated.DraftID == "" {
		t.Fatalf("expected non-empty draft id")
	}
}
