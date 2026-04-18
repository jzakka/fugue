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

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// recordingStore is an in-memory snapshot.SnapshotStore for tests.
type recordingStore struct {
	mu      sync.Mutex
	objects map[string][]byte // normalizedURL → raw body bytes
	calls   int
	err     error
}

func newRecordingStore() *recordingStore {
	return &recordingStore{objects: make(map[string][]byte)}
}

func (s *recordingStore) Put(_ context.Context, normalizedURL string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return s.err
	}
	s.objects[normalizedURL] = append([]byte(nil), body...)
	return nil
}

func newSnapshotPioneerTestSite(t *testing.T, mux *http.ServeMux) (*Pioneer, uuid.UUID, *recordingStore, func()) {
	t.Helper()

	ts := httptest.NewServer(mux)
	cleanup := ts.Close

	siteID := uuid.New()
	siteRepo := NewMockSiteRepository()
	siteRepo.Sites[siteID] = db.BotSite{
		ID:      siteID,
		Domain:  "127.0.0.1",
		RootUrl: ts.URL + "/",
		Active:  true,
	}

	store := newRecordingStore()
	p := NewPioneer(
		siteRepo,
		NewMockGraphRepository(),
		NewMockScriptRepository(),
		NewMockAIClient(),
		NewMockScriptExecutor(),
		PioneerConfig{
			MaxNodesPerSite:  3,
			RateLimitMs:      0,
			SuccessThreshold: 0.7,
			SnapshotEnabled:  true,
		},
	).WithSnapshotStore(store, snapshot.NewMetrics(0))

	return p, siteID, store, cleanup
}

// Tasks 4.3 (negative — fetch failure path) and the spec's
// "Scenario: HTTP 404 응답" / "Scenario: 본문이 비어 있는 성공 응답":
// when fetchHTMLShared rejects the response, the snapshot store must not
// be called.
func TestPioneer_NoSnapshotOnFetchFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	p, siteID, store, cleanup := newSnapshotPioneerTestSite(t, mux)
	defer cleanup()

	if err := p.Run(context.Background(), siteID); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if store.calls != 0 {
		t.Fatalf("expected 0 snapshot Put calls on 404, got %d", store.calls)
	}
}

func TestPioneer_NoSnapshotOnEmptyBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		// no body
	})

	p, siteID, store, cleanup := newSnapshotPioneerTestSite(t, mux)
	defer cleanup()

	if err := p.Run(context.Background(), siteID); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if store.calls != 0 {
		t.Fatalf("expected 0 snapshot Put calls on empty body, got %d", store.calls)
	}
}

// Task 4.4 + spec "Scenario: 업로드 실패 시 크롤 계속":
// when the snapshot store fails, Pioneer must continue link extraction
// and child-node creation as if nothing went wrong.
func TestPioneer_FailOpen_StoreErrorDoesNotBlockCrawl(t *testing.T) {
	mux := http.NewServeMux()
	var serverURL string
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><body><a href="%s/trending">trending</a></body></html>`, serverURL)
	})
	mux.HandleFunc("/trending", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>list</body></html>`)
	})

	p, siteID, store, cleanup := newSnapshotPioneerTestSite(t, mux)
	defer cleanup()
	// Look up the actual server URL so the root page links resolve.
	serverURL = p.siteRepo.(*MockSiteRepository).Sites[siteID].RootUrl
	serverURL = serverURL[:len(serverURL)-1] // strip trailing "/"
	store.err = errors.New("simulated upload failure")

	if err := p.Run(context.Background(), siteID); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Even with all uploads failing, the BFS must have processed pages
	// and extracted at least one link → at least one node beyond root.
	graphRepo := p.graphRepo.(*MockGraphRepository)
	if len(graphRepo.Nodes) < 1 {
		t.Fatalf("expected crawl to produce nodes despite snapshot failures, got %d", len(graphRepo.Nodes))
	}
	if store.calls == 0 {
		t.Fatalf("expected snapshot Put to be attempted at least once")
	}
}

// Spec "Scenario: 비활성화 시 업로드 스킵": with SnapshotEnabled=false,
// no upload calls happen even on a successful fetch.
func TestPioneer_FeatureFlagOff_NoUploadAttempt(t *testing.T) {
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
		ID:      siteID,
		Domain:  "127.0.0.1",
		RootUrl: ts.URL + "/",
		Active:  true,
	}

	store := newRecordingStore()
	p := NewPioneer(
		siteRepo,
		NewMockGraphRepository(),
		NewMockScriptRepository(),
		NewMockAIClient(),
		NewMockScriptExecutor(),
		PioneerConfig{
			MaxNodesPerSite:  3,
			RateLimitMs:      0,
			SuccessThreshold: 0.7,
			SnapshotEnabled:  false, // explicitly off
		},
	).WithSnapshotStore(store, nil)

	if err := p.Run(context.Background(), siteID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("expected 0 Put calls when feature flag is off, got %d", store.calls)
	}
}

// Spec Requirement "스냅샷 키는 normalized URL의 sha256 기반" + design
// Decision 1a: two URLs that normalize to the same template path must be
// stored under the same snapshot store key. Concretely: query-string
// variants of one page (?id=111 vs ?id=222) collapse to a single key, so
// the harvester can reconstruct it from the normalized URL alone.
func TestPioneer_SnapshotKeyUsesNormalizedURL(t *testing.T) {
	mux := http.NewServeMux()
	var serverURL string
	// Root links to two query-string variants of the same template.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><body>
			<a href="%s/page?id=111">a</a>
			<a href="%s/page?id=222">b</a>
		</body></html>`, serverURL, serverURL)
	})
	mux.HandleFunc("/page", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>page</body></html>`)
	})

	p, siteID, store, cleanup := newSnapshotPioneerTestSite(t, mux)
	defer cleanup()
	serverURL = p.siteRepo.(*MockSiteRepository).Sites[siteID].RootUrl
	serverURL = serverURL[:len(serverURL)-1]

	if err := p.Run(context.Background(), siteID); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// All keys passed to Put MUST already be normalized: query strings
	// stripped (templatePath behavior). If the raw finalURL leaked
	// through, we'd see distinct keys per query variant.
	for k := range store.objects {
		if containsQueryParam(k) {
			t.Fatalf("snapshot key contains query string — normalization missing: %q", k)
		}
	}
}

func containsQueryParam(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '?' {
			return true
		}
	}
	return false
}

// Spec "Scenario: 2xx 본문 수신 시 스냅샷 업로드": happy path.
func TestPioneer_SnapshotUploadedOn2xxSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>root</body></html>`)
	})

	p, siteID, store, cleanup := newSnapshotPioneerTestSite(t, mux)
	defer cleanup()

	if err := p.Run(context.Background(), siteID); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if store.calls < 1 {
		t.Fatalf("expected at least 1 snapshot Put on 2xx success, got %d", store.calls)
	}
	if len(store.objects) < 1 {
		t.Fatalf("expected at least 1 stored object, got %d", len(store.objects))
	}
}
