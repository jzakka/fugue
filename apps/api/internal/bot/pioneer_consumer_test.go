package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
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
//
// dequeueScript, when non-empty, takes precedence over dequeueQueue and lets
// budget-loop tests script per-call (url, err) outcomes — needed to verify
// that empty results and errors are not counted toward the worker budget.
// dequeueCalls tracks the total number of Dequeue invocations regardless of
// outcome so tests can assert on retry behaviour.
type fakeScheduler struct {
	dequeueQueue       []string
	dequeueScript      []dequeueResult
	dequeueIdx         int
	dequeueCalls       int
	enqueuePioneer     [][]string
	enqueueHarvester   []enqueueHarvesterCall
	setStatus          []setStatusCall
	recordFetchError   []recordFetchErrorCall
	recordHarvestError []recordFetchErrorCall
}

type dequeueResult struct {
	url string
	err error
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

	// tasks §5.1 requires verifying the cycle "수 회 정상 반복". Drive processOne
	// three times to assert the consumer is stateless — every iteration must
	// emit the full Enqueue + EnqueueHarvester + SetStatus(fetched) triple.
	const iterations = 3
	for i := 0; i < iterations; i++ {
		c.processOne(context.Background(), "https://a.example/root")
	}

	if len(sched.enqueuePioneer) != iterations {
		t.Fatalf("expected %d Enqueue(pioneer) calls, got %d: %+v",
			iterations, len(sched.enqueuePioneer), sched.enqueuePioneer)
	}
	for i, batch := range sched.enqueuePioneer {
		if len(batch) != 2 {
			t.Fatalf("iter %d: expected 2 URLs in Enqueue payload, got %+v", i, batch)
		}
	}
	if len(sched.enqueueHarvester) != iterations {
		t.Fatalf("expected %d EnqueueHarvester, got %d", iterations, len(sched.enqueueHarvester))
	}
	for i, call := range sched.enqueueHarvester {
		if call.url != "https://a.example/root" {
			t.Fatalf("iter %d enqueueHarvester url: got %q", i, call.url)
		}
		if call.snapshotKey == "" {
			t.Fatalf("iter %d enqueueHarvester snapshotKey should be non-empty", i)
		}
	}
	if len(sched.setStatus) != iterations {
		t.Fatalf("expected %d SetStatus calls, got %+v", iterations, sched.setStatus)
	}
	for i, s := range sched.setStatus {
		if s.status != scheduler.StatusFetched {
			t.Fatalf("iter %d: expected SetStatus(fetched), got %+v", i, s)
		}
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
		"https://a.example/same":  true,
		"https://other.example/x": true,
		"https://third.example/y": true,
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

// TestPioneerConsumer_FetchFailure_EmptyBody verifies tasks §5.9 / pioneer
// spec scenario "HTTP 2xx + 빈 body는 fetch 실패로 분류": even though the
// HTTP status is 200, a zero-byte body must be treated as a fetch failure
// and classified as network. No snapshot is written, no harvester fanout.
func TestPioneerConsumer_FetchFailure_EmptyBody(t *testing.T) {
	sched := &fakeScheduler{}
	// fetcher surfaces the empty-body failure via err — matches the
	// fetcher.go:301 contract ("empty response body: status=..."). The
	// consumer must see fetchErr != nil and route through reportFailure
	// before the snapshot step is reached.
	fetcher := &fakeFetcher{
		body:       nil,
		finalURL:   "https://a.example/empty",
		statusCode: 200,
		err:        fmt.Errorf("pioneer_consumer: empty response body: status=200 url=https://a.example/empty"),
	}
	store := &fakeSnapshotStore{}
	chain := NewFilterChain()

	c := NewPioneerConsumer(sched, store, chain, fetcher)
	c.processOne(context.Background(), "https://a.example/empty")

	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusFetchFailed {
		t.Fatalf("expected SetStatus(fetch_failed), got %+v", sched.setStatus)
	}
	if len(sched.recordFetchError) != 1 || sched.recordFetchError[0].kind != scheduler.ErrorNetwork {
		t.Fatalf("expected RecordFetchError(network), got %+v", sched.recordFetchError)
	}
	if len(store.puts) != 0 {
		t.Fatalf("empty-body failure must not Put a snapshot, got %+v", store.puts)
	}
	if len(sched.enqueueHarvester) != 0 {
		t.Fatalf("empty-body failure must not fanout to harvester, got %+v", sched.enqueueHarvester)
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

// --- pioneer-worker-budget: Run loop budget tests ---

// newBudgetConsumer wires a PioneerConsumer with a small budget so loop
// tests don't have to dequeue 100 URLs to exercise the exhaustion path.
// Production callers go through NewPioneerConsumer and inherit WorkerBudget;
// the budget field is package-private precisely because the spec forbids
// runtime exposure.
func newBudgetConsumer(t *testing.T, sched scheduler.URLScheduler, body []byte, budget int) *PioneerConsumer {
	t.Helper()
	fetcher := &fakeFetcher{body: body, finalURL: "https://a.example/root", statusCode: 200}
	store := &fakeSnapshotStore{}
	chain := NewFilterChain()
	c := NewPioneerConsumer(sched, store, chain, fetcher)
	c.budget = budget
	return c
}

// TestPioneerConsumer_Run_BudgetExhaustionExitsZero covers the canonical
// "100회째 처리 완료 후 exit 0" scenario at a smaller budget. After exactly
// `budget` successful Dequeues + processOne cycles, Run returns nil and the
// extra queued URL is never dequeued — verifying the spec's "100회 완료 후
// 추가 Dequeue 호출이 발생하지 않음".
func TestPioneerConsumer_Run_BudgetExhaustionExitsZero(t *testing.T) {
	sched := &fakeScheduler{
		dequeueQueue: []string{
			"https://a.example/u1",
			"https://a.example/u2",
			"https://a.example/u3",
			// u4 must not be dequeued — Run should exit after u3.
			"https://a.example/u4",
		},
	}
	c := newBudgetConsumer(t, sched, []byte("<html></html>"), 3)

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

// TestPioneerConsumer_Run_BudgetExhaustedLogOnce verifies the spec scenario
// "종료 사유 로그": on budget exhaustion the consumer must emit exactly one
// key=value log line containing reason=budget_exhausted, component=pioneer_worker,
// and the actual dequeue count.
func TestPioneerConsumer_Run_BudgetExhaustedLogOnce(t *testing.T) {
	var buf bytes.Buffer
	origOutput := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
	})

	sched := &fakeScheduler{
		dequeueQueue: []string{
			"https://a.example/u1",
			"https://a.example/u2",
		},
	}
	c := newBudgetConsumer(t, sched, []byte("<html></html>"), 2)

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
		`msg="pioneer worker: work budget exhausted"`,
		"component=pioneer_worker",
		"reason=budget_exhausted",
		"dequeues=2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("budget-exhausted log missing %q\nlog output:\n%s", want, out)
		}
	}
}

// TestPioneerConsumer_Run_DoesNotExitBeforeBudget verifies "99회까지는
// 종료하지 않는다": with budget=3 and 2 URLs queued + a context cancel after
// the 2nd processOne, Run must NOT have returned via budget exhaustion.
// We assert that all 2 URLs were processed (Run kept consuming) and the
// loop only exits via ctx — proving no early exit at <budget.
func TestPioneerConsumer_Run_DoesNotExitBeforeBudget(t *testing.T) {
	sched := &fakeScheduler{
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
	c := newBudgetConsumer(t, sched, []byte("<html></html>"), 3)

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

// TestPioneerConsumer_Run_DequeueErrorNotCounted verifies the spec scenario
// "Dequeue 자체 오류는 카운트되지 않는다 ... 워커는 오류를 로깅한 뒤 다시
// Dequeue를 시도한다". With budget=1, 2 errors precede 1 success: Run must
// retry past the errors, count only the successful Dequeue, and exit
// cleanly via budget exhaustion.
func TestPioneerConsumer_Run_DequeueErrorNotCounted(t *testing.T) {
	sched := &fakeScheduler{
		dequeueScript: []dequeueResult{
			{err: errors.New("transient: connection reset")},
			{err: errors.New("transient: deadline")},
			{url: "https://a.example/u1"},
		},
	}
	c := newBudgetConsumer(t, sched, []byte("<html></html>"), 1)

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

// TestPioneerConsumer_Run_EmptyDequeueNotCounted defends the spec scenario
// "빈 Dequeue는 카운트되지 않는다". Even though production Dequeue is
// internally blocking and is not expected to surface an empty result, the
// loop must still treat ("", nil) as "skip & retry" so the contract holds
// if the scheduler implementation ever loosens that invariant.
func TestPioneerConsumer_Run_EmptyDequeueNotCounted(t *testing.T) {
	sched := &fakeScheduler{
		dequeueScript: []dequeueResult{
			{url: ""}, // empty: must be skipped without incrementing budget
			{url: "https://a.example/u1"},
		},
	}
	c := newBudgetConsumer(t, sched, []byte("<html></html>"), 1)

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

// TestPioneerConsumer_Run_IndependentBudgetsPerInstance verifies the spec
// requirement "복수 워커는 각자 독립 카운터를 갖는다": two PioneerConsumer
// instances each spend their own budget without affecting the other's.
func TestPioneerConsumer_Run_IndependentBudgetsPerInstance(t *testing.T) {
	makeSched := func() *fakeScheduler {
		return &fakeScheduler{dequeueQueue: []string{"https://a.example/x", "https://a.example/y"}}
	}
	schedA := makeSched()
	schedB := makeSched()
	cA := newBudgetConsumer(t, schedA, []byte("<html></html>"), 2)
	cB := newBudgetConsumer(t, schedB, []byte("<html></html>"), 2)

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

// TestPioneerConsumer_Run_BudgetExhaustionOnFetchFailure verifies the spec
// scenario "100회째 처리 실패도 정상 종료": when the budget-completing URL's
// fetch fails, Run still exits with nil (exit 0) — the failure is recorded
// via SetStatus(fetch_failed)+RecordFetchError, but does NOT bubble up to
// Run's return value. Symmetric to harvester's "100회째 작업이 실패해도 종료는
// 정상" scenario.
func TestPioneerConsumer_Run_BudgetExhaustionOnFetchFailure(t *testing.T) {
	sched := &fakeScheduler{
		dequeueQueue: []string{"https://a.example/will-404"},
	}
	fetcher := &fakeFetcher{
		statusCode: 404,
		err:        fmt.Errorf("HTTP error: status code 404"),
	}
	store := &fakeSnapshotStore{}
	chain := NewFilterChain()
	c := NewPioneerConsumer(sched, store, chain, fetcher)
	c.budget = 1

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run must return nil even when the budget-completing URL fails fetch, got %v", err)
	}
	if len(sched.setStatus) != 1 || sched.setStatus[0].status != scheduler.StatusFetchFailed {
		t.Fatalf("expected SetStatus(fetch_failed), got %+v", sched.setStatus)
	}
	if len(sched.recordFetchError) != 1 || sched.recordFetchError[0].kind != scheduler.ErrorHTTP4xx {
		t.Fatalf("expected RecordFetchError(http_4xx), got %+v", sched.recordFetchError)
	}
}

// TestPioneerConsumer_Run_DefaultBudgetIsWorkerBudget guards against
// regressions where the default budget drifts from the spec-mandated 100.
func TestPioneerConsumer_Run_DefaultBudgetIsWorkerBudget(t *testing.T) {
	if WorkerBudget != 100 {
		t.Fatalf("spec violation: WorkerBudget must be 100, got %d", WorkerBudget)
	}
	c := NewPioneerConsumer(&fakeScheduler{}, &fakeSnapshotStore{}, NewFilterChain(), &fakeFetcher{})
	// Default budget field is zero, which Run interprets as "use WorkerBudget".
	if c.budget != 0 {
		t.Fatalf("constructor must leave budget at zero (defaults to WorkerBudget at Run time), got %d", c.budget)
	}
}
