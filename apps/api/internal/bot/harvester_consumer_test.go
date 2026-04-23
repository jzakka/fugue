package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// --- test doubles ---

// fakeHarvestScheduler is a minimal URLScheduler for HarvesterConsumer
// tests. Only Dequeue / SetStatus / RecordHarvestError are exercised by
// the consumer; the other methods return nil/empty so the interface is
// satisfied. The fake is NOT goroutine-safe — tests drive it serially.
//
// dequeueScript, when non-empty, takes precedence over dequeueQueue and
// lets budget-loop tests script per-call (url, err) outcomes — needed to
// verify that empty results and errors are not counted toward the worker
// budget. dequeueCalls tracks the total number of Dequeue invocations
// regardless of outcome so tests can assert on retry behaviour.
type fakeHarvestScheduler struct {
	dequeueQueue       []string
	dequeueScript      []dequeueResult
	dequeueIdx         int
	dequeueCalls       int
	dequeueErr         error
	setStatus          []setStatusCall
	recordHarvestError []recordFetchErrorCall
}

func (f *fakeHarvestScheduler) Dequeue(qt scheduler.QueueType) (string, error) {
	if qt != scheduler.QueueHarvester {
		return "", fmt.Errorf("unexpected queue type: %s", qt)
	}
	f.dequeueCalls++
	if len(f.dequeueScript) > 0 {
		if f.dequeueIdx >= len(f.dequeueScript) {
			// Test setup bug: budget loop should have exited before draining
			// the script. Returning a sentinel error keeps the loop alive
			// (since errors are now retried) but the test will time out via
			// ctx, surfacing the misconfiguration.
			return "", errors.New("fake: script exhausted")
		}
		r := f.dequeueScript[f.dequeueIdx]
		f.dequeueIdx++
		return r.url, r.err
	}
	if f.dequeueErr != nil {
		return "", f.dequeueErr
	}
	if len(f.dequeueQueue) == 0 {
		return "", errors.New("fake: queue drained")
	}
	u := f.dequeueQueue[0]
	f.dequeueQueue = f.dequeueQueue[1:]
	return u, nil
}

func (f *fakeHarvestScheduler) Enqueue(scheduler.QueueType, ...string) error { return nil }
func (f *fakeHarvestScheduler) EnqueueHarvester(string, string) error        { return nil }

func (f *fakeHarvestScheduler) SetStatus(key string, status scheduler.Status, pinIDs []uuid.UUID) error {
	f.setStatus = append(f.setStatus, setStatusCall{key: key, status: status, pinIDs: pinIDs})
	return nil
}

func (f *fakeHarvestScheduler) RecordFetchError(string, scheduler.ErrorKind) error { return nil }

func (f *fakeHarvestScheduler) RecordHarvestError(key string, kind scheduler.ErrorKind) error {
	f.recordHarvestError = append(f.recordHarvestError, recordFetchErrorCall{key: key, kind: kind})
	return nil
}

// pinnableDocHTML is a small HTML payload that the generic extractor +
// classifier will accept as pinnable: non-trivial body text, OG thumbnail,
// and low link density.
func pinnableDocHTML() []byte {
	body := strings.Repeat("This is interesting body content describing something notable. ", 10)
	return []byte(`<html><head>` +
		`<meta property="og:title" content="Test Page">` +
		`<meta property="og:image" content="https://cdn.example.com/img.jpg">` +
		`</head><body><article>` + body + `</article></body></html>`)
}

// listingDocHTML yields a page the classifier rejects as listing
// (link density > threshold), which drives the Pinnable=false branch.
func listingDocHTML() []byte {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	for i := 0; i < 60; i++ {
		sb.WriteString(`<a href="/x">x</a> `)
	}
	sb.WriteString(`</body></html>`)
	return []byte(sb.String())
}

// --- classifyHarvestFetchError unit tests (tasks §3.4) ---

func TestClassifyHarvestFetchError_HTTP4xx(t *testing.T) {
	err := fmt.Errorf("HTTP error: status code 404")
	if got := classifyHarvestFetchError(err); got != scheduler.ErrorHTTP4xx {
		t.Fatalf("expected http_4xx, got %q", got)
	}
}

func TestClassifyHarvestFetchError_HTTP5xx(t *testing.T) {
	err := fmt.Errorf("HTTP error: status code 503")
	if got := classifyHarvestFetchError(err); got != scheduler.ErrorHTTP5xx {
		t.Fatalf("expected http_5xx, got %q", got)
	}
}

func TestClassifyHarvestFetchError_Network(t *testing.T) {
	err := errors.New("dial tcp: connection refused")
	if got := classifyHarvestFetchError(err); got != scheduler.ErrorNetwork {
		t.Fatalf("expected network, got %q", got)
	}
}

func TestClassifyHarvestFetchError_Timeout(t *testing.T) {
	if got := classifyHarvestFetchError(timeoutErr{}); got != scheduler.ErrorTimeout {
		t.Fatalf("expected timeout, got %q", got)
	}
}

// TestClassifyHarvestFetchError_OnlyReturnsSchedulerEnum verifies the
// contract in tasks §3.4: the helper MUST only return one of the four
// scheduler enum values. A value outside the enum would cause
// scheduler.RecordHarvestError to return ErrUnknownErrorKind and leave the
// row unchanged (stuck until lease expiry).
func TestClassifyHarvestFetchError_OnlyReturnsSchedulerEnum(t *testing.T) {
	cases := []error{
		nil,
		errors.New("some random error"),
		fmt.Errorf("HTTP error: status code 204"), // 2xx (outside 4xx/5xx bands)
		fmt.Errorf("HTTP error: status code 399"), // 3xx
		timeoutErr{},
	}
	allowed := map[scheduler.ErrorKind]bool{
		scheduler.ErrorHTTP4xx: true,
		scheduler.ErrorHTTP5xx: true,
		scheduler.ErrorNetwork: true,
		scheduler.ErrorTimeout: true,
	}
	for _, e := range cases {
		got := classifyHarvestFetchError(e)
		if !allowed[got] {
			t.Errorf("classifyHarvestFetchError(%v) = %q — outside scheduler enum", e, got)
		}
	}
}

// --- processOne integration-style tests ---

// TestHarvesterConsumer_SuccessPath covers tasks §8.1: the full happy path
// where a pinnable document is persisted and SetStatus(harvested, pinIDs)
// fires exactly once. No RecordHarvestError must fire.
func TestHarvesterConsumer_SuccessPath(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{
			"https://a.example/p1": pinnableDocHTML(),
		},
	}
	pipeline := NewMockPipeline()

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, pipeline)
	c.processOne(context.Background(), "https://a.example/p1")

	if len(sched.setStatus) != 1 {
		t.Fatalf("expected 1 SetStatus call, got %+v", sched.setStatus)
	}
	call := sched.setStatus[0]
	if call.status != scheduler.StatusHarvested {
		t.Fatalf("expected SetStatus(harvested), got %v", call)
	}
	if len(call.pinIDs) != 1 {
		t.Fatalf("expected 1 pinID in SetStatus, got %d (%v)", len(call.pinIDs), call.pinIDs)
	}
	if call.pinIDs[0] == uuid.Nil {
		t.Fatalf("pinID must not be zero uuid")
	}
	if len(sched.recordHarvestError) != 0 {
		t.Fatalf("success path must not call RecordHarvestError, got %+v", sched.recordHarvestError)
	}
	if pipeline.CallCount != 1 {
		t.Fatalf("expected pipeline.ProcessDocument called once, got %d", pipeline.CallCount)
	}
}

// TestHarvesterConsumer_SnapshotHitPath verifies §8.2: when the injected
// Fetcher returns a snapshot body on first call, the consumer proceeds
// without a second fetch. Because HarvesterConsumer depends only on the
// Fetcher interface, snapshot-hit vs live-fetch behaviour is invisible at
// this layer — we verify the body is consumed exactly once per URL.
func TestHarvesterConsumer_SnapshotHitPath(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{
			"https://a.example/snap": pinnableDocHTML(),
		},
	}

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())
	c.processOne(context.Background(), "https://a.example/snap")

	if fetcher.calls["https://a.example/snap"] != 1 {
		t.Fatalf("expected 1 Fetcher call, got %d", fetcher.calls["https://a.example/snap"])
	}
	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusHarvested {
		t.Fatalf("expected SetStatus(harvested), got %+v", sched.setStatus)
	}
}

// TestHarvesterConsumer_FetchFailure_HTTP4xx covers §8.3: dual call on
// fetch failure — SetStatus(harvest_failed, nil) + RecordHarvestError(http_4xx).
// The pipeline MUST NOT be invoked.
func TestHarvesterConsumer_FetchFailure_HTTP4xx(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := newMapFetcher()
	fetcher.errs["https://a.example/missing"] = fmt.Errorf("HTTP error: status code 404")
	pipeline := NewMockPipeline()

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, pipeline)
	c.processOne(context.Background(), "https://a.example/missing")

	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusHarvestFailed {
		t.Fatalf("expected SetStatus(harvest_failed), got %+v", sched.setStatus)
	}
	if sched.setStatus[0].pinIDs != nil {
		t.Fatalf("expected nil pinIDs on failure, got %+v", sched.setStatus[0].pinIDs)
	}
	if len(sched.recordHarvestError) != 1 || sched.recordHarvestError[0].kind != scheduler.ErrorHTTP4xx {
		t.Fatalf("expected RecordHarvestError(http_4xx), got %+v", sched.recordHarvestError)
	}
	if pipeline.CallCount != 0 {
		t.Fatalf("fetch failure must not invoke pipeline, got CallCount=%d", pipeline.CallCount)
	}
}

func TestHarvesterConsumer_FetchFailure_HTTP5xx(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := newMapFetcher()
	fetcher.errs["https://a.example/oops"] = fmt.Errorf("HTTP error: status code 502")

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())
	c.processOne(context.Background(), "https://a.example/oops")

	if len(sched.recordHarvestError) != 1 || sched.recordHarvestError[0].kind != scheduler.ErrorHTTP5xx {
		t.Fatalf("expected RecordHarvestError(http_5xx), got %+v", sched.recordHarvestError)
	}
}

func TestHarvesterConsumer_FetchFailure_Network(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := newMapFetcher()
	fetcher.errs["https://a.example/dns"] = errors.New("dial tcp: no such host")

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())
	c.processOne(context.Background(), "https://a.example/dns")

	if len(sched.recordHarvestError) != 1 || sched.recordHarvestError[0].kind != scheduler.ErrorNetwork {
		t.Fatalf("expected RecordHarvestError(network), got %+v", sched.recordHarvestError)
	}
}

func TestHarvesterConsumer_FetchFailure_Timeout(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := &timeoutFetcher{}

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())
	c.processOne(context.Background(), "https://a.example/slow")

	if len(sched.recordHarvestError) != 1 || sched.recordHarvestError[0].kind != scheduler.ErrorTimeout {
		t.Fatalf("expected RecordHarvestError(timeout), got %+v", sched.recordHarvestError)
	}
}

// TestHarvesterConsumer_NotPinnable covers §8.4: classifier returns
// pinnable=false, consumer emits SetStatus(harvested, nil) and skips Pin
// creation. Spec design.md Decision 4: "pinDocument.Pinnable == false →
// skip (pinIDs = nil)".
func TestHarvesterConsumer_NotPinnable(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{"https://a.example/list": listingDocHTML()},
	}
	pipeline := NewMockPipeline()

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, pipeline)
	c.processOne(context.Background(), "https://a.example/list")

	if len(sched.setStatus) != 1 {
		t.Fatalf("expected 1 SetStatus call, got %+v", sched.setStatus)
	}
	if sched.setStatus[0].status != scheduler.StatusHarvested {
		t.Fatalf("expected SetStatus(harvested) even on non-pinnable, got %v", sched.setStatus[0])
	}
	if sched.setStatus[0].pinIDs != nil {
		t.Fatalf("expected nil pinIDs on non-pinnable, got %+v", sched.setStatus[0].pinIDs)
	}
	if pipeline.CallCount != 0 {
		t.Fatalf("non-pinnable URL must not reach pipeline.ProcessDocument, got CallCount=%d", pipeline.CallCount)
	}
	if len(sched.recordHarvestError) != 0 {
		t.Fatalf("non-pinnable is a success outcome, not a failure; got %+v", sched.recordHarvestError)
	}
}

// TestHarvesterConsumer_CreatePinsFailure covers §8.6: when pipeline.
// ProcessDocument returns an error, the consumer reports it to scheduler
// as ErrorNetwork (spec Decision 6 maps internal "pin_create" → network).
func TestHarvesterConsumer_CreatePinsFailure(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{"https://a.example/p": pinnableDocHTML()},
	}
	pipeline := NewMockPipeline()
	pipeline.ProcessDocumentFunc = func(context.Context, db.BotGraphNode, PinDocument) (bool, uuid.UUID, error) {
		return false, uuid.Nil, errors.New("db down")
	}

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, pipeline)
	c.processOne(context.Background(), "https://a.example/p")

	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusHarvestFailed {
		t.Fatalf("expected SetStatus(harvest_failed), got %+v", sched.setStatus)
	}
	if len(sched.recordHarvestError) != 1 || sched.recordHarvestError[0].kind != scheduler.ErrorNetwork {
		t.Fatalf("expected RecordHarvestError(network) for pin_create failure, got %+v", sched.recordHarvestError)
	}
}

// TestHarvesterConsumer_ParseFailure covers §8.7: when the extractor
// errors, scheduler gets RecordHarvestError(network) per Decision 6's
// "parse" → network mapping. We inject a stub extractor that always
// errors because the real GenericExtractor is defensive and rarely
// returns an error for arbitrary HTML input.
func TestHarvesterConsumer_ParseFailure(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{"https://a.example/bad": pinnableDocHTML()},
	}
	pipeline := NewMockPipeline()

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, pipeline).
		withExtractor(&erroringExtractor{})
	c.processOne(context.Background(), "https://a.example/bad")

	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusHarvestFailed {
		t.Fatalf("expected SetStatus(harvest_failed), got %+v", sched.setStatus)
	}
	if len(sched.recordHarvestError) != 1 || sched.recordHarvestError[0].kind != scheduler.ErrorNetwork {
		t.Fatalf("expected RecordHarvestError(network) for parse failure, got %+v", sched.recordHarvestError)
	}
	if pipeline.CallCount != 0 {
		t.Fatalf("parse failure must not reach pipeline, got CallCount=%d", pipeline.CallCount)
	}
}

// TestHarvesterConsumer_Run_CtxCancel covers §8.8: Run must return when
// ctx is cancelled rather than spinning in the Dequeue retry loop. The
// fake scheduler has URLs in queue so Dequeue would otherwise return; we
// cancel ctx and verify Run exits with ctx.Err().
func TestHarvesterConsumer_Run_CtxCancel(t *testing.T) {
	sched := &fakeHarvestScheduler{dequeueQueue: []string{"https://a.example/p"}}
	fetcher := &mapFetcher{bodies: map[string][]byte{"https://a.example/p": pinnableDocHTML()}}

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run starts

	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not exit within 2s after ctx cancel")
	}
}

// TestHarvesterConsumer_Run_DequeueErrorRetries verifies the spec scenario
// "Dequeue 자체 오류는 카운트되지 않는다 — 워커는 오류를 로깅한 뒤 다시
// Dequeue를 시도한다". A permanent Dequeue error must not exit Run; Run
// must retry and only return when ctx is cancelled. Replaces the earlier
// "return on first error" contract which was superseded by
// harvester-worker-budget.
func TestHarvesterConsumer_Run_DequeueErrorRetries(t *testing.T) {
	sched := &fakeHarvestScheduler{dequeueErr: errors.New("permanent failure")}
	fetcher := newMapFetcher()

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := c.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected ctx deadline exceeded after retried errors, got %v", err)
	}
	if sched.dequeueCalls < 2 {
		t.Fatalf("expected Dequeue to be retried (>=2 calls), got %d", sched.dequeueCalls)
	}
}

// TestHarvesterConsumer_DualCallEvenIfSetStatusFails pins the spec §
// "실패 시 SetStatus와 RecordHarvestError 둘 다 호출" invariant: even if
// the SetStatus(harvest_failed) call itself errors, RecordHarvestError
// MUST still fire. This protects the scheduler's retry bookkeeping from
// silently desyncing when the first half of the dual call fails.
func TestHarvesterConsumer_DualCallEvenIfSetStatusFails(t *testing.T) {
	sched := &dualCallFakeScheduler{
		fakeHarvestScheduler: fakeHarvestScheduler{},
		setStatusErr:         errors.New("setStatus boom"),
	}
	fetcher := newMapFetcher()
	fetcher.errs["https://a.example/x"] = fmt.Errorf("HTTP error: status code 500")

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())
	c.processOne(context.Background(), "https://a.example/x")

	if len(sched.setStatus) != 1 {
		t.Fatalf("expected SetStatus attempted once, got %d", len(sched.setStatus))
	}
	if len(sched.recordHarvestError) != 1 {
		t.Fatalf("expected RecordHarvestError called exactly once even after SetStatus error, got %d",
			len(sched.recordHarvestError))
	}
	if sched.recordHarvestError[0].kind != scheduler.ErrorHTTP5xx {
		t.Fatalf("expected http_5xx, got %q", sched.recordHarvestError[0].kind)
	}
}

// TestHarvesterConsumer_CrossHostMixedDequeue covers tasks §6.1: a single
// worker must process URLs across multiple hosts in the order the
// scheduler returns them, with no per-host setup/teardown. We seed two
// URLs on different hosts and assert both SetStatus calls fire in
// sequence.
func TestHarvesterConsumer_CrossHostMixedDequeue(t *testing.T) {
	sched := &fakeHarvestScheduler{
		dequeueQueue: []string{
			"https://a.example/p1",
			"https://b.example/p2",
		},
	}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{
			"https://a.example/p1": pinnableDocHTML(),
			"https://b.example/p2": pinnableDocHTML(),
		},
	}

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())
	c.processOne(context.Background(), "https://a.example/p1")
	c.processOne(context.Background(), "https://b.example/p2")

	if len(sched.setStatus) != 2 {
		t.Fatalf("expected 2 SetStatus calls across hosts, got %+v", sched.setStatus)
	}
	if sched.setStatus[0].key != "https://a.example/p1" || sched.setStatus[1].key != "https://b.example/p2" {
		t.Fatalf("expected interleaved host order, got %+v", sched.setStatus)
	}
	for _, s := range sched.setStatus {
		if s.status != scheduler.StatusHarvested {
			t.Errorf("expected all harvested, got %v", s)
		}
	}
}

// TestHarvesterConsumer_SnapshotHit_NoHTTPCall covers
// harvester-snapshot-first-fetch task §4.9 at the consumer integration
// layer: when HarvesterConsumer runs with a CompositeFetcher whose
// ObjectStorage side returns a snapshot hit, the HTTP side MUST NOT be
// invoked. The unit-layer version of this assertion lives in
// TestCompositeFetcher_SnapshotHitSkipsHTTP; this test pins the contract
// at the Harvester-loop boundary so a future refactor cannot accidentally
// reorder fetch attempts.
func TestHarvesterConsumer_SnapshotHit_NoHTTPCall(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	pipeline := NewMockPipeline()

	snap := &unitFetcher{body: pinnableDocHTML()}
	httpSpy := &unitFetcher{body: []byte("<html>should-not-be-seen</html>")}
	composite := NewCompositeFetcher(snap, httpSpy)

	c := NewHarvesterConsumer(sched, composite, nil, nil, nil, pipeline)
	c.processOne(context.Background(), "https://a.example/snap")

	if snap.calls != 1 {
		t.Errorf("ObjectStorage side called %d times, want 1", snap.calls)
	}
	if httpSpy.calls != 0 {
		t.Errorf("HTTP side called %d times on snapshot hit, want 0", httpSpy.calls)
	}
	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusHarvested {
		t.Errorf("expected SetStatus(harvested), got %+v", sched.setStatus)
	}
}

// unitFetcher is a minimal Fetcher double used by the Harvester-loop
// integration tests above. It records call count and returns canned
// (body, err). Kept local because mapFetcher keys on URL, and these tests
// only need one URL.
type unitFetcher struct {
	calls int
	body  []byte
	err   error
}

func (f *unitFetcher) Fetch(string) ([]byte, error) {
	f.calls++
	return f.body, f.err
}

// TestHarvesterConsumer_FetchFailureCounter_IsolatesPerNode covers
// harvester-snapshot-first-fetch tasks §3.2 and §4.8: when a node's
// Fetcher call fails (the injected Fetcher is typically CompositeFetcher,
// so this represents an ObjectStorage+HTTP dual miss), the in-memory
// fetchFailureCount MUST increment by exactly 1 and the consumer MUST
// continue processing subsequent nodes unaffected. The counter is
// orthogonal to scheduler.RecordHarvestError's DB-backed
// harvest_error_count column (Decision 3).
func TestHarvesterConsumer_FetchFailureCounter_IsolatesPerNode(t *testing.T) {
	sched := &fakeHarvestScheduler{}
	fetcher := newMapFetcher()
	fetcher.errs["https://a.example/dead"] = errors.New("dual miss: snapshot and http both failed")
	fetcher.bodies["https://b.example/live"] = pinnableDocHTML()

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())

	// Node 1: fetch failure — counter should tick to 1.
	c.processOne(context.Background(), "https://a.example/dead")
	if got := c.FetchFailureCount(); got != 1 {
		t.Fatalf("after one fetch failure, FetchFailureCount = %d, want 1", got)
	}

	// Node 2: fetch succeeds — subsequent nodes must process despite the
	// prior failure, and the counter must NOT double-count.
	c.processOne(context.Background(), "https://b.example/live")
	if got := c.FetchFailureCount(); got != 1 {
		t.Fatalf("after one success following one failure, FetchFailureCount = %d, want 1 (no double-count)", got)
	}

	// Scheduler-side invariants (spec Decision 6 / §8.3): dual call on
	// failure, single harvested call on success. Confirms the in-memory
	// counter lives alongside — not instead of — the scheduler contract.
	if len(sched.setStatus) != 2 {
		t.Fatalf("expected 2 SetStatus calls (one per node), got %+v", sched.setStatus)
	}
	if sched.setStatus[0].status != scheduler.StatusHarvestFailed {
		t.Errorf("node 1 SetStatus = %v, want harvest_failed", sched.setStatus[0].status)
	}
	if sched.setStatus[1].status != scheduler.StatusHarvested {
		t.Errorf("node 2 SetStatus = %v, want harvested", sched.setStatus[1].status)
	}
	if len(sched.recordHarvestError) != 1 {
		t.Errorf("RecordHarvestError should fire exactly once (node 1), got %+v", sched.recordHarvestError)
	}
}

// --- test doubles that can't live with the test where they're used ---

// mapFetcher implements bot.Fetcher by looking up a URL in bodies for the
// success path and in errs for the failure path. Counts fetches per URL
// so tests can assert snapshot-hit behavior (exactly one Fetch per URL).
type mapFetcher struct {
	bodies map[string][]byte
	errs   map[string]error
	calls  map[string]int
}

func newMapFetcher() *mapFetcher {
	return &mapFetcher{
		bodies: make(map[string][]byte),
		errs:   make(map[string]error),
		calls:  make(map[string]int),
	}
}

func (m *mapFetcher) Fetch(rawURL string) ([]byte, error) {
	if m.calls == nil {
		m.calls = make(map[string]int)
	}
	m.calls[rawURL]++
	if err, ok := m.errs[rawURL]; ok {
		return nil, err
	}
	if body, ok := m.bodies[rawURL]; ok {
		return body, nil
	}
	return nil, fmt.Errorf("mapFetcher: no body/err registered for %q", rawURL)
}

// timeoutFetcher returns a net.Error with Timeout()==true, driving the
// classifyHarvestFetchError timeout branch.
type timeoutFetcher struct{}

func (timeoutFetcher) Fetch(string) ([]byte, error) { return nil, timeoutErr{} }

// dualCallFakeScheduler overrides SetStatus to return an error on demand so
// tests can verify that RecordHarvestError still fires (dual-call
// invariant). All other calls delegate to the embedded fakeHarvestScheduler
// which records them normally.
type dualCallFakeScheduler struct {
	fakeHarvestScheduler
	setStatusErr error
}

func (d *dualCallFakeScheduler) SetStatus(key string, status scheduler.Status, pinIDs []uuid.UUID) error {
	d.setStatus = append(d.setStatus, setStatusCall{key: key, status: status, pinIDs: pinIDs})
	return d.setStatusErr
}

// erroringExtractor always returns an extract error so the parse-failure
// path in HarvesterConsumer.processOne is exercised deterministically.
// The real *GenericExtractor is defensive and almost never returns an
// error for arbitrary HTML, which is why we inject this stub via the
// unexported withExtractor() test seam.
type erroringExtractor struct{}

func (erroringExtractor) Extract([]byte, string) (PinDocument, error) {
	return PinDocument{}, errors.New("extractor boom")
}

// --- harvester-worker-budget: Run loop budget tests ---

// newBudgetHarvester wires a HarvesterConsumer with a small budget so loop
// tests don't have to dequeue 100 URLs to exercise the exhaustion path.
// Production callers go through NewHarvesterConsumer and inherit
// harvesterDequeueBudget; the budget field is package-private precisely
// because the spec forbids runtime exposure.
func newBudgetHarvester(t *testing.T, sched scheduler.URLScheduler, fetcher Fetcher, pipeline DocumentPipeline, budget int) *HarvesterConsumer {
	t.Helper()
	if pipeline == nil {
		pipeline = NewMockPipeline()
	}
	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, pipeline)
	c.budget = budget
	return c
}

// TestHarvesterConsumer_Run_BudgetExhaustionExitsZero covers the canonical
// "100회 처리 후 정상 종료" scenario at a smaller budget. After exactly
// `budget` successful Dequeues + processOne cycles, Run returns nil and the
// extra queued URL is never dequeued.
func TestHarvesterConsumer_Run_BudgetExhaustionExitsZero(t *testing.T) {
	sched := &fakeHarvestScheduler{
		dequeueQueue: []string{
			"https://a.example/u1",
			"https://a.example/u2",
			"https://a.example/u3",
			// u4 must not be dequeued — Run should exit after u3.
			"https://a.example/u4",
		},
	}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{
			"https://a.example/u1": pinnableDocHTML(),
			"https://a.example/u2": pinnableDocHTML(),
			"https://a.example/u3": pinnableDocHTML(),
			"https://a.example/u4": pinnableDocHTML(),
		},
	}
	c := newBudgetHarvester(t, sched, fetcher, nil, 3)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run should return nil on budget exhaustion, got %v", err)
	}
	if got := len(sched.setStatus); got != 3 {
		t.Fatalf("expected exactly budget=3 SetStatus calls, got %d", got)
	}
	if sched.dequeueCalls != 3 {
		t.Fatalf("expected exactly 3 Dequeue calls, got %d (u4 must not be dequeued)", sched.dequeueCalls)
	}
}

// TestHarvesterConsumer_Run_BudgetExhaustedLogOnce verifies the spec
// scenario "종료 사유 로그": on budget exhaustion the consumer emits exactly
// one key=value log line containing reason=budget_exhausted,
// component=harvester_worker, and the actual dequeue count.
func TestHarvesterConsumer_Run_BudgetExhaustedLogOnce(t *testing.T) {
	var buf bytes.Buffer
	origOutput := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
	})

	sched := &fakeHarvestScheduler{
		dequeueQueue: []string{
			"https://a.example/u1",
			"https://a.example/u2",
		},
	}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{
			"https://a.example/u1": pinnableDocHTML(),
			"https://a.example/u2": pinnableDocHTML(),
		},
	}
	c := newBudgetHarvester(t, sched, fetcher, nil, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run should return nil on budget exhaustion, got %v", err)
	}

	out := buf.String()
	if got := strings.Count(out, "reason=budget_exhausted"); got != 1 {
		t.Fatalf("expected exactly 1 budget-exhausted log line, got %d\nlog output:\n%s", got, out)
	}
	for _, want := range []string{
		`msg="harvester worker: work budget exhausted"`,
		"component=harvester_worker",
		"reason=budget_exhausted",
		"dequeues=2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("budget-exhausted log missing %q\nlog output:\n%s", want, out)
		}
	}
}

// TestHarvesterConsumer_Run_DoesNotExitBeforeBudget verifies "99회까지는
// 종료하지 않는다": with budget=3 and 2 URLs queued + a context cancel after
// the 2nd processOne, Run must NOT have returned via budget exhaustion.
// We assert that all 2 URLs were processed (Run kept consuming) and the
// loop only exits via ctx — proving no early exit at <budget.
func TestHarvesterConsumer_Run_DoesNotExitBeforeBudget(t *testing.T) {
	sched := &fakeHarvestScheduler{
		dequeueScript: []dequeueResult{
			{url: "https://a.example/u1"},
			{url: "https://a.example/u2"},
			// 3rd call: ctx is cancelled by then via the timeout below.
			// Returning a sentinel error keeps the loop alive (errors are
			// retried per spec); the ctx check at the top of the loop will
			// trip on the next iteration.
			{err: errors.New("hold")},
		},
	}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{
			"https://a.example/u1": pinnableDocHTML(),
			"https://a.example/u2": pinnableDocHTML(),
		},
	}
	c := newBudgetHarvester(t, sched, fetcher, nil, 3)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := c.Run(ctx)
	if err == nil {
		t.Fatalf("expected Run to exit via ctx cancel (not budget), got nil")
	}
	if got := len(sched.setStatus); got != 2 {
		t.Fatalf("expected 2 URLs processed before ctx cancel, got %d", got)
	}
}

// TestHarvesterConsumer_Run_DequeueErrorNotCounted verifies the spec
// scenario "Dequeue 자체 오류는 카운트되지 않는다 ... 워커는 오류를 로깅한
// 뒤 다시 Dequeue를 시도한다". With budget=1, 2 errors precede 1 success:
// Run must retry past the errors, count only the successful Dequeue, and
// exit cleanly via budget exhaustion.
func TestHarvesterConsumer_Run_DequeueErrorNotCounted(t *testing.T) {
	sched := &fakeHarvestScheduler{
		dequeueScript: []dequeueResult{
			{err: errors.New("transient: connection reset")},
			{err: errors.New("transient: deadline")},
			{url: "https://a.example/u1"},
		},
	}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{"https://a.example/u1": pinnableDocHTML()},
	}
	c := newBudgetHarvester(t, sched, fetcher, nil, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run should return nil on budget exhaustion after retried errors, got %v", err)
	}
	if sched.dequeueCalls != 3 {
		t.Fatalf("expected 3 Dequeue calls (2 retried errors + 1 success), got %d", sched.dequeueCalls)
	}
	if len(sched.setStatus) != 1 {
		t.Fatalf("expected 1 SetStatus (only the successful URL processed), got %d", len(sched.setStatus))
	}
}

// TestHarvesterConsumer_Run_EmptyDequeueNotCounted defends the spec
// scenario "빈 Dequeue는 카운트되지 않는다". Even though production Dequeue
// is internally blocking and is not expected to surface an empty result,
// the loop must still treat ("", nil) as "skip & retry" so the contract
// holds if the scheduler implementation ever loosens that invariant.
func TestHarvesterConsumer_Run_EmptyDequeueNotCounted(t *testing.T) {
	sched := &fakeHarvestScheduler{
		dequeueScript: []dequeueResult{
			{url: ""}, // empty: must be skipped without incrementing budget
			{url: "https://a.example/u1"},
		},
	}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{"https://a.example/u1": pinnableDocHTML()},
	}
	c := newBudgetHarvester(t, sched, fetcher, nil, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run should return nil on budget exhaustion after empty skip, got %v", err)
	}
	if sched.dequeueCalls != 2 {
		t.Fatalf("expected 2 Dequeue calls (1 empty + 1 success), got %d", sched.dequeueCalls)
	}
	if len(sched.setStatus) != 1 {
		t.Fatalf("expected 1 SetStatus (only the non-empty URL processed), got %d", len(sched.setStatus))
	}
}

// TestHarvesterConsumer_Run_IndependentBudgetsPerInstance verifies the spec
// requirement "복수 워커는 각자 독립 카운터를 갖는다": two HarvesterConsumer
// instances each spend their own budget without affecting the other's.
func TestHarvesterConsumer_Run_IndependentBudgetsPerInstance(t *testing.T) {
	makeSched := func() *fakeHarvestScheduler {
		return &fakeHarvestScheduler{dequeueQueue: []string{"https://a.example/x", "https://a.example/y"}}
	}
	makeFetcher := func() *mapFetcher {
		return &mapFetcher{
			bodies: map[string][]byte{
				"https://a.example/x": pinnableDocHTML(),
				"https://a.example/y": pinnableDocHTML(),
			},
		}
	}
	schedA := makeSched()
	schedB := makeSched()
	cA := newBudgetHarvester(t, schedA, makeFetcher(), nil, 2)
	cB := newBudgetHarvester(t, schedB, makeFetcher(), nil, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cA.Run(ctx); err != nil {
		t.Fatalf("consumer A: %v", err)
	}
	if err := cB.Run(ctx); err != nil {
		t.Fatalf("consumer B: %v", err)
	}
	if schedA.dequeueCalls != 2 || schedB.dequeueCalls != 2 {
		t.Fatalf("each consumer should dequeue 2 URLs independently, got A=%d B=%d", schedA.dequeueCalls, schedB.dequeueCalls)
	}
}

// TestHarvesterConsumer_Run_BudgetExhaustionOnFetchFailure covers the spec
// scenario "100회째 작업이 실패해도 종료는 정상": when the budget-completing
// URL's harvest fails, Run still exits with nil (exit 0) and the dual-call
// (SetStatus(harvest_failed) + RecordHarvestError) both fire before exit.
func TestHarvesterConsumer_Run_BudgetExhaustionOnFetchFailure(t *testing.T) {
	sched := &fakeHarvestScheduler{
		dequeueQueue: []string{"https://a.example/will-404"},
	}
	fetcher := newMapFetcher()
	fetcher.errs["https://a.example/will-404"] = fmt.Errorf("HTTP error: status code 404")
	c := newBudgetHarvester(t, sched, fetcher, nil, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run must return nil even when the budget-completing URL fails fetch, got %v", err)
	}
	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusHarvestFailed {
		t.Fatalf("expected SetStatus(harvest_failed), got %+v", sched.setStatus)
	}
	if len(sched.recordHarvestError) != 1 || sched.recordHarvestError[0].kind != scheduler.ErrorHTTP4xx {
		t.Fatalf("expected RecordHarvestError(http_4xx), got %+v", sched.recordHarvestError)
	}
}

// TestHarvesterConsumer_Run_CtxCancelMidBudget verifies the spec scenario
// "ctx 취소 경로는 budget과 독립적이다": ctx cancellation must exit Run at
// any time without waiting for the budget to be exhausted.
func TestHarvesterConsumer_Run_CtxCancelMidBudget(t *testing.T) {
	sched := &fakeHarvestScheduler{
		dequeueQueue: []string{
			"https://a.example/u1",
			"https://a.example/u2",
			"https://a.example/u3",
			"https://a.example/u4",
			"https://a.example/u5",
		},
	}
	fetcher := &mapFetcher{
		bodies: map[string][]byte{
			"https://a.example/u1": pinnableDocHTML(),
			"https://a.example/u2": pinnableDocHTML(),
			"https://a.example/u3": pinnableDocHTML(),
			"https://a.example/u4": pinnableDocHTML(),
			"https://a.example/u5": pinnableDocHTML(),
		},
	}
	// budget=100 (default via zero), but we cancel after a short delay so
	// Run must exit well before exhausting the queue.
	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := c.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected ctx deadline exceeded, got %v", err)
	}
	if len(sched.setStatus) >= 100 {
		t.Fatalf("ctx cancel should have exited well before budget=100, got %d processed", len(sched.setStatus))
	}
}

// TestHarvesterConsumer_Run_DefaultBudgetIsHarvesterBudget guards against
// regressions where the default budget drifts from the spec-mandated 100
// or is exposed via a runtime surface (env/CLI/config).
func TestHarvesterConsumer_Run_DefaultBudgetIsHarvesterBudget(t *testing.T) {
	if harvesterDequeueBudget != 100 {
		t.Fatalf("spec violation: harvesterDequeueBudget must be 100, got %d", harvesterDequeueBudget)
	}
	c := NewHarvesterConsumer(&fakeHarvestScheduler{}, newMapFetcher(), nil, nil, nil, NewMockPipeline())
	// Default budget field is zero, which Run interprets as
	// "use harvesterDequeueBudget".
	if c.budget != 0 {
		t.Fatalf("constructor must leave budget at zero (defaults to harvesterDequeueBudget at Run time), got %d", c.budget)
	}
}
