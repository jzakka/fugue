package bot

import (
	"context"
	"errors"
	"fmt"
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
type fakeHarvestScheduler struct {
	dequeueQueue       []string
	dequeueErr         error
	setStatus          []setStatusCall
	recordHarvestError []recordFetchErrorCall
}

func (f *fakeHarvestScheduler) Dequeue(qt scheduler.QueueType) (string, error) {
	if qt != scheduler.QueueHarvester {
		return "", fmt.Errorf("unexpected queue type: %s", qt)
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

// TestHarvesterConsumer_Run_DequeueError verifies the Run loop surfaces
// permanent Dequeue errors rather than hot-looping. Mirrors the Pioneer
// TestPioneerConsumer_DequeueError_NoHotLoop contract.
func TestHarvesterConsumer_Run_DequeueError(t *testing.T) {
	sched := &fakeHarvestScheduler{dequeueErr: errors.New("permanent failure")}
	fetcher := newMapFetcher()

	c := NewHarvesterConsumer(sched, fetcher, nil, nil, nil, NewMockPipeline())

	done := make(chan error, 1)
	go func() { done <- c.Run(context.Background()) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected Run to return error on permanent Dequeue failure")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not exit within 2s — hot-loop suspected")
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
