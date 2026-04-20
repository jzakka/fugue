package bot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// togglePipeline is a DocumentPipeline used by Harvester stats tests that
// lets each test inject per-node decisions via onProcess.
type togglePipeline struct {
	onProcess      func(node db.BotGraphNode, doc PinDocument) (bool, uuid.UUID, error)
	onSkip         func(node db.BotGraphNode) error
	processCnt     int
	markSkippedCnt int
}

func (p *togglePipeline) ProcessDocument(_ context.Context, node db.BotGraphNode, doc PinDocument) (bool, uuid.UUID, error) {
	p.processCnt++
	if p.onProcess == nil {
		return true, uuid.New(), nil
	}
	return p.onProcess(node, doc)
}

func (p *togglePipeline) MarkSkipped(_ context.Context, node db.BotGraphNode) error {
	p.markSkippedCnt++
	if p.onSkip == nil {
		return nil
	}
	return p.onSkip(node)
}

// failingAdapter always errors so the Harvester takes the generic fallback
// path and AdapterFallback increments.
type failingAdapter struct{ domain string }

func (a *failingAdapter) Domain() string { return a.domain }
func (a *failingAdapter) Name() string   { return "failing" }
func (a *failingAdapter) Extract(_ context.Context, _ []byte, _ string) (PinDocument, error) {
	return PinDocument{}, errors.New("adapter boom")
}

func pinnableHTML(title string) string {
	body := strings.Repeat("This is rich body text explaining something interesting. ", 10)
	return `<html><head>` +
		`<meta property="og:title" content="` + title + `">` +
		`<meta property="og:image" content="https://cdn.example.com/img.jpg">` +
		`</head><body><article>` + body + `</article></body></html>`
}

func listingHTML() string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	for i := 0; i < 60; i++ {
		sb.WriteString(`<a href="/x">x</a> `)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

func makeServer(t *testing.T, pages map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
}

func buildHarvester(graphRepo *MockGraphRepository, siteRepo *MockSiteRepository, pipeline DocumentPipeline, registry AdapterRegistry) *Harvester {
	return NewHarvester(
		siteRepo,
		graphRepo,
		NewMockScriptRepository(),
		NewMockScriptExecutor(),
		pipeline,
		registry,
		NewGenericExtractor(),
		NewClassifier(),
		HarvesterConfig{RateLimitMs: 0},
	)
}

func seedGraph(t *testing.T, siteRepo *MockSiteRepository, graphRepo *MockGraphRepository, rootURL string, children []string) db.BotSite {
	t.Helper()
	site, err := siteRepo.Create(context.Background(), db.CreateSiteParams{
		Domain:  "example.test",
		RootUrl: rootURL,
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	root, err := graphRepo.CreateNode(context.Background(), db.CreateNodeParams{
		SiteID:  site.ID,
		Url:     rootURL,
		UrlHash: hashURL(rootURL),
	})
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	for _, u := range children {
		child, cerr := graphRepo.CreateNode(context.Background(), db.CreateNodeParams{
			SiteID:  site.ID,
			Url:     u,
			UrlHash: hashURL(u),
		})
		if cerr != nil {
			t.Fatalf("create child: %v", cerr)
		}
		if eerr := graphRepo.CreateEdge(context.Background(), db.CreateEdgeParams{
			FromNodeID: root.ID,
			ToNodeID:   child.ID,
		}); eerr != nil {
			t.Fatalf("create edge: %v", eerr)
		}
	}
	return site
}

// TestHarvester_FourPrimaryCategoriesAreMutuallyExclusive drives the Harvester
// through 4 nodes that each land in a different primary category
// (PinsCreated / Deduped / Skipped / Failed) and verifies the sum equals
// NodesProcessed — i.e. every node increments exactly one primary category.
func TestHarvester_FourPrimaryCategoriesAreMutuallyExclusive(t *testing.T) {
	pages := map[string]string{
		"/ok":      pinnableHTML("OK"),
		"/dedup":   pinnableHTML("Dedup"),
		"/listing": listingHTML(),
	}
	server := makeServer(t, pages)
	defer server.Close()

	rootURL := server.URL + "/ok"
	children := []string{
		server.URL + "/dedup",
		server.URL + "/listing",
		server.URL + "/not-found", // 404 → fetch error → Failed
	}

	siteRepo := NewMockSiteRepository()
	graphRepo := NewMockGraphRepository()
	site := seedGraph(t, siteRepo, graphRepo, rootURL, children)

	calls := 0
	pipeline := &togglePipeline{
		onProcess: func(_ db.BotGraphNode, _ PinDocument) (bool, uuid.UUID, error) {
			calls++
			return calls == 1, uuid.New(), nil // first pinnable call → created, second → deduped
		},
	}
	h := buildHarvester(graphRepo, siteRepo, pipeline, nil)

	stats, err := h.Run(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.NodesProcessed != 4 {
		t.Errorf("NodesProcessed = %d, want 4", stats.NodesProcessed)
	}
	if stats.PinsCreated != 1 {
		t.Errorf("PinsCreated = %d, want 1", stats.PinsCreated)
	}
	if stats.Deduped != 1 {
		t.Errorf("Deduped = %d, want 1", stats.Deduped)
	}
	if stats.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", stats.Skipped)
	}
	if stats.Failed != 1 {
		t.Errorf("Failed = %d, want 1", stats.Failed)
	}
	sum := stats.PinsCreated + stats.Deduped + stats.Skipped + stats.Failed
	if sum != stats.NodesProcessed {
		t.Errorf("category sum %d != NodesProcessed %d — categories not mutually exclusive", sum, stats.NodesProcessed)
	}
	if stats.AdapterFallback != 0 {
		t.Errorf("AdapterFallback = %d, want 0 (no adapter registered)", stats.AdapterFallback)
	}
}

// TestHarvester_AdapterFallbackIsIndependent confirms AdapterFallback can
// increment alongside a primary category (here PinsCreated) since it is an
// auxiliary counter, not one of the 4 mutually-exclusive primary buckets.
func TestHarvester_AdapterFallbackIsIndependent(t *testing.T) {
	pages := map[string]string{"/ok": pinnableHTML("OK")}
	server := makeServer(t, pages)
	defer server.Close()

	parsed, _ := url.Parse(server.URL)
	host := parsed.Hostname()
	rootURL := server.URL + "/ok"

	siteRepo := NewMockSiteRepository()
	graphRepo := NewMockGraphRepository()
	site := seedGraph(t, siteRepo, graphRepo, rootURL, nil)

	registry := NewInMemoryAdapterRegistry()
	registry.Register(&failingAdapter{domain: host})

	pipeline := &togglePipeline{
		onProcess: func(_ db.BotGraphNode, _ PinDocument) (bool, uuid.UUID, error) {
			return true, uuid.New(), nil
		},
	}
	h := buildHarvester(graphRepo, siteRepo, pipeline, registry)

	stats, err := h.Run(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.PinsCreated != 1 {
		t.Errorf("PinsCreated = %d, want 1 (generic fallback succeeded)", stats.PinsCreated)
	}
	if stats.AdapterFallback != 1 {
		t.Errorf("AdapterFallback = %d, want 1", stats.AdapterFallback)
	}
	if stats.NodesProcessed != 1 {
		t.Errorf("NodesProcessed = %d, want 1", stats.NodesProcessed)
	}
}

// TestHarvester_SkippedCallsMarkSkippedOnly verifies a pinnable=false node
// calls MarkSkipped and NEVER ProcessDocument — the Pin upsert must not fire
// for skipped nodes.
func TestHarvester_SkippedCallsMarkSkippedOnly(t *testing.T) {
	pages := map[string]string{"/listing": listingHTML()}
	server := makeServer(t, pages)
	defer server.Close()

	rootURL := server.URL + "/listing"
	siteRepo := NewMockSiteRepository()
	graphRepo := NewMockGraphRepository()
	site := seedGraph(t, siteRepo, graphRepo, rootURL, nil)

	pipeline := &togglePipeline{
		onProcess: func(_ db.BotGraphNode, _ PinDocument) (bool, uuid.UUID, error) {
			t.Fatalf("ProcessDocument must not be called for skipped node")
			return false, uuid.Nil, nil
		},
	}
	h := buildHarvester(graphRepo, siteRepo, pipeline, nil)

	stats, err := h.Run(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", stats.Skipped)
	}
	if pipeline.markSkippedCnt != 1 {
		t.Errorf("MarkSkipped calls = %d, want 1", pipeline.markSkippedCnt)
	}
	if pipeline.processCnt != 0 {
		t.Errorf("ProcessDocument calls = %d, want 0", pipeline.processCnt)
	}
}
