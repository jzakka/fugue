package bot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// recordingSaver captures every SaveRawContent call and optionally injects
// an error. Concurrent-safe so several goroutines may record at once.
type recordingSaver struct {
	mu     sync.Mutex
	err    error
	urls   []string
	bodies [][]byte
}

func (r *recordingSaver) SaveRawContent(_ context.Context, url string, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls = append(r.urls, url)
	r.bodies = append(r.bodies, append([]byte(nil), body...))
	return r.err
}

func (r *recordingSaver) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.urls)
}

// TestPioneer_SnapshotHook_CalledOn2xx verifies that SaveRawContent is
// invoked once per successful fetch and receives the fetched body.
func TestPioneer_SnapshotHook_CalledOn2xx(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><p>ok</p></body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	siteID := uuid.New()
	siteRepo := NewMockSiteRepository()
	siteRepo.Sites[siteID] = db.BotSite{
		ID: siteID, Domain: "127.0.0.1", RootUrl: ts.URL + "/", Active: true,
	}
	saver := &recordingSaver{}
	pioneer := NewPioneer(
		siteRepo,
		NewMockGraphRepository(),
		NewMockScriptRepository(),
		NewMockAIClient(),
		NewMockScriptExecutor(),
		PioneerConfig{MaxNodesPerSite: 1, RateLimitMs: 0, SuccessThreshold: 0.7},
	).WithSnapshotSaver(saver)

	if err := pioneer.Run(context.Background(), siteID); err != nil {
		t.Fatalf("pioneer.Run: %v", err)
	}
	if saver.Count() == 0 {
		t.Fatal("expected SaveRawContent to be called for successful fetch")
	}
	// Non-empty body recorded on every call.
	saver.mu.Lock()
	for i, b := range saver.bodies {
		if len(b) == 0 {
			t.Fatalf("recorded body #%d is empty", i)
		}
	}
	saver.mu.Unlock()
}

// TestPioneer_SnapshotHook_NotCalledOnFetchError verifies the spec
// "fetch failure → no snapshot" rule (4xx/5xx, network error).
func TestPioneer_SnapshotHook_NotCalledOnFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	siteID := uuid.New()
	siteRepo := NewMockSiteRepository()
	siteRepo.Sites[siteID] = db.BotSite{
		ID: siteID, Domain: "127.0.0.1", RootUrl: ts.URL + "/", Active: true,
	}
	saver := &recordingSaver{}
	pioneer := NewPioneer(
		siteRepo,
		NewMockGraphRepository(),
		NewMockScriptRepository(),
		NewMockAIClient(),
		NewMockScriptExecutor(),
		PioneerConfig{MaxNodesPerSite: 1, RateLimitMs: 0, SuccessThreshold: 0.7},
	).WithSnapshotSaver(saver)

	_ = pioneer.Run(context.Background(), siteID)
	if saver.Count() != 0 {
		t.Fatalf("expected 0 SaveRawContent calls on 404, got %d", saver.Count())
	}
}

// TestPioneer_SnapshotHook_FailOpen verifies that a saver error does not
// abort the crawl: Pioneer still creates the root node (proof the loop
// continued past the snapshot hook).
func TestPioneer_SnapshotHook_FailOpen(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>ok</body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	siteID := uuid.New()
	siteRepo := NewMockSiteRepository()
	siteRepo.Sites[siteID] = db.BotSite{
		ID: siteID, Domain: "127.0.0.1", RootUrl: ts.URL + "/", Active: true,
	}
	graphRepo := NewMockGraphRepository()
	saver := &recordingSaver{err: errors.New("storage down")}
	pioneer := NewPioneer(
		siteRepo,
		graphRepo,
		NewMockScriptRepository(),
		NewMockAIClient(),
		NewMockScriptExecutor(),
		PioneerConfig{MaxNodesPerSite: 1, RateLimitMs: 0, SuccessThreshold: 0.7},
	).WithSnapshotSaver(saver)

	if err := pioneer.Run(context.Background(), siteID); err != nil {
		t.Fatalf("pioneer.Run should not surface saver error, got %v", err)
	}
	if saver.Count() == 0 {
		t.Fatal("saver should have been invoked at least once")
	}
	if len(graphRepo.Nodes) == 0 {
		t.Fatal("crawl must continue past a failed snapshot save (graph nodes should exist)")
	}
}

// TestPioneer_SnapshotHook_DefaultNoop ensures Pioneer works out of the
// box without a saver wired (feature flag off / not configured).
func TestPioneer_SnapshotHook_DefaultNoop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>ok</body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	siteID := uuid.New()
	siteRepo := NewMockSiteRepository()
	siteRepo.Sites[siteID] = db.BotSite{
		ID: siteID, Domain: "127.0.0.1", RootUrl: ts.URL + "/", Active: true,
	}
	pioneer := NewPioneer(
		siteRepo,
		NewMockGraphRepository(),
		NewMockScriptRepository(),
		NewMockAIClient(),
		NewMockScriptExecutor(),
		PioneerConfig{MaxNodesPerSite: 1, RateLimitMs: 0, SuccessThreshold: 0.7},
	)
	if err := pioneer.Run(context.Background(), siteID); err != nil {
		t.Fatalf("pioneer.Run without saver: %v", err)
	}
}
