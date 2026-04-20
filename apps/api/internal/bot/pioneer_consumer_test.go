package bot

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// timeoutErr implements net.Error with Timeout() true, used to verify
// classifyFetchError picks ErrorTimeout over the generic network fallback
// even when statusCode is 0.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// Sanity check that timeoutErr is indeed seen as a net.Error — tests
// depend on errors.As resolving to a timeout-capable interface.
var _ net.Error = timeoutErr{}

func TestClassifyFetchError_Timeout(t *testing.T) {
	got := classifyFetchError(timeoutErr{}, 0)
	if got != scheduler.ErrorTimeout {
		t.Fatalf("expected %q, got %q", scheduler.ErrorTimeout, got)
	}
}

func TestClassifyFetchError_HTTP4xx(t *testing.T) {
	err := fmt.Errorf("HTTP error: status code 404")
	// Pass statusCode=0 to exercise the regex fallback path.
	if got := classifyFetchError(err, 0); got != scheduler.ErrorHTTP4xx {
		t.Fatalf("regex fallback: expected http_4xx, got %q", got)
	}
	// Pass statusCode directly to exercise the structured path.
	if got := classifyFetchError(err, 404); got != scheduler.ErrorHTTP4xx {
		t.Fatalf("structured: expected http_4xx, got %q", got)
	}
}

func TestClassifyFetchError_HTTP5xx(t *testing.T) {
	err := fmt.Errorf("HTTP error: status code 503")
	if got := classifyFetchError(err, 503); got != scheduler.ErrorHTTP5xx {
		t.Fatalf("expected http_5xx, got %q", got)
	}
}

func TestClassifyFetchError_Network(t *testing.T) {
	err := errors.New("connection refused")
	if got := classifyFetchError(err, 0); got != scheduler.ErrorNetwork {
		t.Fatalf("expected network, got %q", got)
	}
}

func TestLinkURLs_Empty(t *testing.T) {
	if got := linkURLs(nil); len(got) != 0 {
		t.Fatalf("nil input: expected empty slice, got %v", got)
	}
}

func TestLinkURLs_PreservesOrder(t *testing.T) {
	in := []crawler.Link{
		{URL: "https://a.example/x"},
		{URL: "https://a.example/y"},
		{URL: "https://b.example/z"},
	}
	got := linkURLs(in)
	want := []string{"https://a.example/x", "https://a.example/y", "https://b.example/z"}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("at %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// --- PioneerConsumer integration-style tests with a fake scheduler ---

// fakeScheduler implements scheduler.URLScheduler with in-memory slices so
// tests can assert on the exact call sequence (Enqueue / EnqueueHarvester /
// SetStatus / RecordFetchError) that PioneerConsumer produces per URL.
type fakeScheduler struct {
	dequeueQueue       []string
	enqueuePioneer     [][]string
	enqueueHarvester   []enqueueHarvesterCall
	setStatus          []setStatusCall
	recordFetchError   []recordFetchErrorCall
	recordHarvestError []recordFetchErrorCall
}

type enqueueHarvesterCall struct {
	url         string
	snapshotKey string
}

type setStatusCall struct {
	key    string
	status scheduler.Status
	pinIDs []uuid.UUID
}

type recordFetchErrorCall struct {
	key  string
	kind scheduler.ErrorKind
}

func (f *fakeScheduler) Dequeue(qt scheduler.QueueType) (string, error) {
	if len(f.dequeueQueue) == 0 {
		return "", errors.New("fake: queue drained")
	}
	u := f.dequeueQueue[0]
	f.dequeueQueue = f.dequeueQueue[1:]
	return u, nil
}

func (f *fakeScheduler) Enqueue(qt scheduler.QueueType, urls ...string) error {
	if qt == scheduler.QueuePioneer {
		f.enqueuePioneer = append(f.enqueuePioneer, append([]string{}, urls...))
	}
	return nil
}

func (f *fakeScheduler) EnqueueHarvester(url, snapshotKey string) error {
	f.enqueueHarvester = append(f.enqueueHarvester, enqueueHarvesterCall{url: url, snapshotKey: snapshotKey})
	return nil
}

func (f *fakeScheduler) SetStatus(key string, status scheduler.Status, pinIDs []uuid.UUID) error {
	f.setStatus = append(f.setStatus, setStatusCall{key: key, status: status, pinIDs: pinIDs})
	return nil
}

func (f *fakeScheduler) RecordFetchError(key string, kind scheduler.ErrorKind) error {
	f.recordFetchError = append(f.recordFetchError, recordFetchErrorCall{key: key, kind: kind})
	return nil
}

func (f *fakeScheduler) RecordHarvestError(key string, kind scheduler.ErrorKind) error {
	f.recordHarvestError = append(f.recordHarvestError, recordFetchErrorCall{key: key, kind: kind})
	return nil
}

// fakeFetcher returns a fixed body + status for every URL, and allows an
// error override for failure-path tests.
type fakeFetcher struct {
	body       []byte
	finalURL   string
	statusCode int
	err        error
}

func (f *fakeFetcher) Fetch(_ context.Context, rawURL string) ([]byte, string, int, error) {
	final := f.finalURL
	if final == "" {
		final = rawURL
	}
	return f.body, final, f.statusCode, f.err
}

// fakeSnapshotStore accepts every Put without doing I/O; callers may
// override `err` for failure-path tests.
type fakeSnapshotStore struct {
	puts []string
	err  error
}

func (s *fakeSnapshotStore) Put(_ context.Context, normalizedURL string, _ []byte) error {
	if s.err != nil {
		return s.err
	}
	s.puts = append(s.puts, normalizedURL)
	return nil
}

// TestPioneerConsumer_SuccessPath verifies the canonical success sequence:
// Enqueue(pioneer, extracted) → EnqueueHarvester(orig, snapshot_key) →
// SetStatus(fetched). No RecordFetchError should fire.
func TestPioneerConsumer_SuccessPath(t *testing.T) {
	body := []byte(`<html><body>
		<a href="https://a.example/next1">one</a>
		<a href="https://a.example/next2">two</a>
	</body></html>`)
	sched := &fakeScheduler{}
	fetcher := &fakeFetcher{body: body, finalURL: "https://a.example/root", statusCode: 200}
	store := &fakeSnapshotStore{}
	chain := NewFilterChain(&DomainFilter{}, &ExtensionFilter{})

	// Freeze time so snapshot key is deterministic.
	fixed := time.Date(2025, 11, 20, 12, 0, 0, 0, time.UTC)
	c := NewPioneerConsumer(sched, store, chain, fetcher).WithClock(func() time.Time { return fixed })

	c.processOne(context.Background(), "https://a.example/root")

	if len(sched.enqueuePioneer) != 1 || len(sched.enqueuePioneer[0]) != 2 {
		t.Fatalf("expected 1 Enqueue(pioneer) with 2 URLs, got %+v", sched.enqueuePioneer)
	}
	if len(sched.enqueueHarvester) != 1 {
		t.Fatalf("expected exactly 1 EnqueueHarvester, got %d", len(sched.enqueueHarvester))
	}
	if sched.enqueueHarvester[0].url != "https://a.example/root" {
		t.Fatalf("enqueueHarvester url: got %q", sched.enqueueHarvester[0].url)
	}
	if sched.enqueueHarvester[0].snapshotKey == "" {
		t.Fatalf("enqueueHarvester snapshotKey should be non-empty")
	}
	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusFetched {
		t.Fatalf("expected SetStatus(fetched), got %+v", sched.setStatus)
	}
	if len(sched.recordFetchError) != 0 {
		t.Fatalf("success path must not call RecordFetchError, got %+v", sched.recordFetchError)
	}
}

// TestPioneerConsumer_FetchFailure_HTTP404 verifies the dual-call contract:
// SetStatus(fetch_failed) AND RecordFetchError(http_4xx) must both fire when
// the fetcher returns a 404. EnqueueHarvester MUST NOT fire.
func TestPioneerConsumer_FetchFailure_HTTP404(t *testing.T) {
	sched := &fakeScheduler{}
	fetcher := &fakeFetcher{
		statusCode: 404,
		err:        fmt.Errorf("HTTP error: status code 404"),
	}
	store := &fakeSnapshotStore{}
	chain := NewFilterChain()

	c := NewPioneerConsumer(sched, store, chain, fetcher)
	c.processOne(context.Background(), "https://a.example/missing")

	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusFetchFailed {
		t.Fatalf("expected SetStatus(fetch_failed), got %+v", sched.setStatus)
	}
	if len(sched.recordFetchError) != 1 || sched.recordFetchError[0].kind != scheduler.ErrorHTTP4xx {
		t.Fatalf("expected RecordFetchError(http_4xx), got %+v", sched.recordFetchError)
	}
	if len(sched.enqueueHarvester) != 0 {
		t.Fatalf("failure path must not fanout to harvester, got %+v", sched.enqueueHarvester)
	}
}

// TestPioneerConsumer_CrossSiteEnqueue verifies tasks §5.4: links that point
// to external domains are NOT filtered out by Pioneer itself — the default
// DomainFilter (no AllowKeywords/DenyKeywords) passes every host through,
// and the consumer hands all of them to Enqueue(QueuePioneer, ...). This is
// the spec's "교차 사이트 기본 허용" contract: domain boundaries are the
// filter policy's business, not the consumer's.
func TestPioneerConsumer_CrossSiteEnqueue(t *testing.T) {
	body := []byte(`<html><body>
		<a href="https://a.example/same">same-site</a>
		<a href="https://other.example/x">other-site-1</a>
		<a href="https://third.example/y">other-site-2</a>
	</body></html>`)
	sched := &fakeScheduler{}
	fetcher := &fakeFetcher{body: body, finalURL: "https://a.example/root", statusCode: 200}
	store := &fakeSnapshotStore{}
	// Empty DomainFilter: no AllowKeywords/DenyKeywords → every host passes.
	chain := NewFilterChain(&DomainFilter{}, &ExtensionFilter{})

	c := NewPioneerConsumer(sched, store, chain, fetcher)
	c.processOne(context.Background(), "https://a.example/root")

	if len(sched.enqueuePioneer) != 1 {
		t.Fatalf("expected 1 Enqueue(pioneer) call, got %d", len(sched.enqueuePioneer))
	}
	got := sched.enqueuePioneer[0]
	want := map[string]bool{
		"https://a.example/same":    true,
		"https://other.example/x":   true,
		"https://third.example/y":   true,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d URLs enqueued, got %d: %v", len(want), len(got), got)
	}
	for _, u := range got {
		if !want[u] {
			t.Errorf("unexpected URL in Enqueue payload: %q", u)
		}
	}
}

// TestPioneerConsumer_SnapshotFailure_ClassifiedNetwork verifies that a
// snapshot-store Put error is reported as network-kind (tasks §3.7).
func TestPioneerConsumer_SnapshotFailure_ClassifiedNetwork(t *testing.T) {
	sched := &fakeScheduler{}
	fetcher := &fakeFetcher{body: []byte("<html></html>"), finalURL: "https://a.example/x", statusCode: 200}
	store := &fakeSnapshotStore{err: errors.New("s3 down")}
	chain := NewFilterChain()

	c := NewPioneerConsumer(sched, store, chain, fetcher)
	c.processOne(context.Background(), "https://a.example/x")

	if len(sched.recordFetchError) != 1 || sched.recordFetchError[0].kind != scheduler.ErrorNetwork {
		t.Fatalf("expected RecordFetchError(network) for snapshot failure, got %+v", sched.recordFetchError)
	}
	if len(sched.enqueueHarvester) != 0 {
		t.Fatalf("snapshot failure must not fanout to harvester, got %+v", sched.enqueueHarvester)
	}
}
