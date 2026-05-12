package bot

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

type stubScriptExecutor struct {
	items []RawItem
	err   error
}

func (s *stubScriptExecutor) Execute(_ context.Context, _ string, _ string, _ string) ([]RawItem, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

type stubScriptRepo struct {
	script db.BotScript
	err    error
}

func (s *stubScriptRepo) Create(_ context.Context, _ db.CreateScriptParams) (db.BotScript, error) {
	return db.BotScript{}, errors.New("not implemented")
}
func (s *stubScriptRepo) GetBySiteType(_ context.Context, _ db.GetScriptBySiteTypeParams) (db.BotScript, error) {
	return s.script, s.err
}

func TestScriptAdapter_FirstItemBecomesPrimary(t *testing.T) {
	repo := &stubScriptRepo{script: db.BotScript{ScriptCode: "// noop"}}
	exec := &stubScriptExecutor{
		items: []RawItem{
			{Title: "Primary", Description: "primary body", MediaURL: "https://cdn.example.com/p.jpg", MediaType: "image", SourceURL: "https://example.com/post/1"},
			{Title: "Extra1", MediaURL: "https://cdn.example.com/x1.jpg", MediaType: "image"},
			{Title: "Extra2", MediaURL: "https://cdn.example.com/x2.jpg", MediaType: "image"},
		},
	}
	a := NewScriptAdapter(uuid.New(), "example.com", repo, exec)

	ctx := WithNodeType(context.Background(), "detail")
	doc, err := a.Extract(ctx, []byte("<html></html>"), "https://example.com/post/1")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if doc.Title != "Primary" {
		t.Errorf("title = %q", doc.Title)
	}
	if doc.BodyText != "primary body" {
		t.Errorf("body = %q", doc.BodyText)
	}
	if doc.ThumbnailURL != "https://cdn.example.com/p.jpg" {
		t.Errorf("thumb = %q", doc.ThumbnailURL)
	}
	if len(doc.MediaCandidates) != 3 {
		t.Fatalf("expected 3 media candidates (primary + 2 extras), got %d", len(doc.MediaCandidates))
	}
	if doc.OGData.Source != "https://example.com/post/1" {
		t.Errorf("source = %q", doc.OGData.Source)
	}
}

func TestScriptAdapter_ExtractorIdentitySet(t *testing.T) {
	siteID := uuid.New()
	repo := &stubScriptRepo{script: db.BotScript{ScriptCode: "// noop"}}
	exec := &stubScriptExecutor{items: []RawItem{{Title: "x", MediaURL: "https://e/x.jpg", MediaType: "image"}}}
	a := NewScriptAdapter(siteID, "example.com", repo, exec)

	ctx := WithNodeType(context.Background(), "detail")
	doc, err := a.Extract(ctx, nil, "https://example.com/x")
	if err != nil {
		t.Fatal(err)
	}
	want := "script:" + siteID.String()
	if doc.OGData.Extractor != want {
		t.Errorf("extractor = %q, want %q", doc.OGData.Extractor, want)
	}
}

func TestScriptAdapter_ZeroItemsErrors(t *testing.T) {
	repo := &stubScriptRepo{script: db.BotScript{ScriptCode: "// noop"}}
	exec := &stubScriptExecutor{items: nil}
	a := NewScriptAdapter(uuid.New(), "example.com", repo, exec)
	ctx := WithNodeType(context.Background(), "detail")
	if _, err := a.Extract(ctx, nil, "https://example.com"); err == nil {
		t.Fatal("expected error on zero items")
	}
}

func TestScriptAdapter_ExecutorErrorPropagates(t *testing.T) {
	repo := &stubScriptRepo{script: db.BotScript{ScriptCode: "// noop"}}
	exec := &stubScriptExecutor{err: errors.New("boom")}
	a := NewScriptAdapter(uuid.New(), "example.com", repo, exec)
	ctx := WithNodeType(context.Background(), "detail")
	if _, err := a.Extract(ctx, nil, "https://example.com"); err == nil {
		t.Fatal("expected error from executor")
	}
}

func TestScriptAdapter_NodeTypeMissing(t *testing.T) {
	repo := &stubScriptRepo{}
	exec := &stubScriptExecutor{}
	a := NewScriptAdapter(uuid.New(), "example.com", repo, exec)
	if _, err := a.Extract(context.Background(), nil, "https://example.com"); err == nil {
		t.Fatal("expected error when node type is missing from context")
	}
}

func TestScriptAdapter_ScriptLookupErrorPropagates(t *testing.T) {
	repo := &stubScriptRepo{err: errors.New("not found")}
	exec := &stubScriptExecutor{}
	a := NewScriptAdapter(uuid.New(), "example.com", repo, exec)
	ctx := WithNodeType(context.Background(), "detail")
	if _, err := a.Extract(ctx, nil, "https://example.com"); err == nil {
		t.Fatal("expected error when script lookup fails")
	}
}
