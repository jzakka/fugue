// pioneer_consumer.go implements the pioneer-scheduler-consumer OpenSpec
// change: Pioneer as a thin consumer of pioneer_frontier that fans out to
// both pioneer_frontier (new links) and harvester_frontier (original URL +
// snapshot_key). In-memory BFS queue / visited map are deliberately absent;
// the URLScheduler owns dedup and ordering.
//
// Spec SSoT: openspec/changes/pioneer-scheduler-consumer/specs/pioneer/spec.md
// Scheduler contract SSoT: internal/scheduler/url_scheduler.go (URLScheduler)

package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
	"github.com/chungsanghwa/fugue/apps/api/internal/httpclient"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// ConsumerFetcher is the small fetch surface PioneerConsumer depends on.
// Production uses DefaultConsumerFetcher (HTTP); tests substitute in-memory
// implementations. Declared as an interface (not a function type) so mocks
// can carry state (e.g. a URL→fixture map) without closure gymnastics.
type ConsumerFetcher interface {
	// Fetch returns the response body, the final URL after redirects, the
	// HTTP status code, and any error. statusCode is 0 for non-HTTP errors
	// (network/timeout/body-read failure). The body is returned as []byte,
	// not a stream, because the consumer needs it twice (snapshot + parse)
	// and re-opening the stream isn't possible after read.
	Fetch(ctx context.Context, rawURL string) (body []byte, finalURL string, statusCode int, err error)
}

// WorkerBudget caps the number of successful URLScheduler.Dequeue calls a
// single Pioneer worker process performs before exiting (spec
// `pioneer-worker-budget`). The constant is intentionally not exposed via
// env/config/CLI: budget changes happen by editing this value and rebuilding.
// Symmetric to harvester's worker budget so operators reason about both
// workers with one mental model.
const WorkerBudget = 100

// dequeueErrorBackoff bounds how often the consumer Run loops may re-enter
// scheduler.URLScheduler.DequeueCtx after the previous call returned an
// error. Without this sleep, the spec rule "Dequeue 자체 오류는 카운트되지
// 않는다" (errors are not counted toward the worker budget) collapses into a
// busy-wait: PGURLScheduler.DequeueCtx's own poll sleep only fires on the
// (claimed=false, no-err) path (postgres_scheduler.go:236-243), so a real
// error from tryClaim — most commonly BeginTx failing in microseconds when
// the DB connection pool is down — returns to the caller immediately, the
// Run loop logs+continues, and the cycle repeats thousands of times per
// second per worker. The intent of the spec rule was "do not consume the
// budget", not "skip the backoff"; without a sleep here, a sustained DB
// outage burns one CPU core per worker, drowns log-shipping pipelines, and
// is invisible to k8s liveness probes (consumer pods expose no HTTP).
//
// Pinned to 1s to match scheduler.defaultPollInterval — the same value
// PGURLScheduler.DequeueCtx uses on the (empty queue / all-throttled) path
// (postgres_scheduler.go:27). Operators reason about both backoffs with the
// same number, and "spec pins this to 1 second" applies symmetrically: the
// quiet-but-healthy and quiet-because-broken backoffs are deliberately the
// same. Not surfaced via env/config/CLI for the same reason WorkerBudget
// isn't — recovery cadence is a build-time tuning knob, not a per-deploy
// override.
const dequeueErrorBackoff = 1 * time.Second

// PioneerConsumer implements the new Dequeue → fetch → snapshot → parse →
// filter → Enqueue(pioneer) + EnqueueHarvester → SetStatus loop.
//
// No in-memory queue, no visited map, no site-bound state: everything lives
// in URLScheduler. Multiple PioneerConsumer instances may run concurrently
// against the same scheduler; FOR UPDATE SKIP LOCKED in ClaimPioneerCandidates
// guarantees each URL is processed exactly once.
type PioneerConsumer struct {
	scheduler     scheduler.URLScheduler
	snapshotStore snapshot.SnapshotStore
	filterChain   *FilterChain
	fetcher       ConsumerFetcher
	// now is injected so tests can freeze the snapshot-key date segment.
	// Production: time.Now.
	now func() time.Time
	// budget overrides WorkerBudget for in-process tests only. Zero (the
	// default) means "use WorkerBudget". This field is package-private and
	// has no env/config/CLI surface, satisfying the spec's "build-time
	// constant" rule for production callers.
	budget int
	// errorBackoff overrides dequeueErrorBackoff for in-process tests only.
	// Zero (the default) means "use dequeueErrorBackoff". Same package-private
	// test seam shape as `budget` above — production code paths cannot reach
	// it because no exported setter exists.
	errorBackoff time.Duration
}

// NewPioneerConsumer wires the consumer with its four mandatory dependencies
// (spec: "Pioneer 생성자에서 URLScheduler, SnapshotStore, FilterChain,
// LinkExtractor, Fetcher를 주입받도록"). LinkExtractor is not a separate
// dependency because the crawler package already exposes ExtractLinksWithSelectors
// as a pure function; injecting it would only add indirection.
func NewPioneerConsumer(
	sched scheduler.URLScheduler,
	store snapshot.SnapshotStore,
	chain *FilterChain,
	fetcher ConsumerFetcher,
) *PioneerConsumer {
	return &PioneerConsumer{
		scheduler:     sched,
		snapshotStore: store,
		filterChain:   chain,
		fetcher:       fetcher,
		now:           time.Now,
	}
}

// WithClock overrides the time source. Tests use this to make snapshot keys
// deterministic (SnapshotKey bakes the UTC date into the path).
func (p *PioneerConsumer) WithClock(now func() time.Time) *PioneerConsumer {
	if now != nil {
		p.now = now
	}
	return p
}

// Run blocks, processing URLs from pioneer_frontier until ctx is cancelled,
// or the worker budget is exhausted (spec `pioneer-worker-budget`). Per spec
// "루프 sleep/backoff 코드 제거": no sleeps here — block-on-empty polling is
// internal to URLScheduler.Dequeue.
//
// Budget accounting (spec `pioneer-worker-budget`):
//   - Counter increments only after Dequeue returns a non-empty URL with no
//     error. Empty results and errors are NOT counted; the loop logs the
//     error and retries so a transient scheduler hiccup cannot kill the
//     worker before its budget is spent.
//   - The 100th URL is processed to completion (fetch → snapshot → parse →
//     enqueue → set_status); only then does Run emit the structured
//     budget-exhausted log and return nil so the supervisor can restart a
//     fresh process.
func (p *PioneerConsumer) Run(ctx context.Context) error {
	budget := p.budget
	if budget <= 0 {
		budget = WorkerBudget
	}
	backoff := p.errorBackoff
	if backoff <= 0 {
		backoff = dequeueErrorBackoff
	}
	dequeues := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rawURL, err := p.scheduler.DequeueCtx(ctx, scheduler.QueuePioneer)
		if err != nil {
			// Spec "Dequeue 자체 오류는 카운트되지 않는다": log and retry
			// instead of returning, so a transient scheduler/DB hiccup does
			// not consume any budget or kill the worker prematurely.
			log.Printf("WARN pioneer_consumer: dequeue err=%v", err)
			// Match the (empty queue / all-throttled) sleep cadence to avoid
			// a hot-spin when tryClaim's BeginTx fails immediately on DB
			// outage — see dequeueErrorBackoff doc above. ctx.Done arms the
			// select so SIGTERM still unblocks within the same nanosecond.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}
		if rawURL == "" {
			// Spec "빈 Dequeue는 카운트되지 않는다": URLScheduler.Dequeue is
			// internally blocking and is not expected to return an empty
			// string in production, but defensive handling here keeps the
			// counter contract correct if the contract ever loosens.
			continue
		}
		dequeues++
		p.processOne(ctx, rawURL)
		if dequeues >= budget {
			log.Printf(
				`msg="pioneer worker: work budget exhausted" component=pioneer_worker reason=budget_exhausted dequeues=%d`,
				dequeues,
			)
			return nil
		}
	}
}

// processOne runs the per-URL pipeline. Errors are handled in-place (report
// to scheduler, log) rather than returning, so a single bad URL cannot kill
// the Run loop — the spec treats the consumer as an always-on daemon.
func (p *PioneerConsumer) processOne(ctx context.Context, rawURL string) {
	body, finalURL, status, fetchErr := p.fetcher.Fetch(ctx, rawURL)
	if fetchErr != nil {
		kind := classifyFetchError(fetchErr, status)
		p.reportFailure(rawURL, kind)
		return
	}

	// snapshot_key is derived from the canonicalized dequeued URL to match
	// design.md Decision 1 pseudo-code (`normalized := canonicalURL(url)`).
	// canonicalURL is the link_filter.go helper; it's the SSOT normalizer per
	// tasks §3.3.
	canonical := canonicalURL(rawURL)
	snapKey := snapshot.SnapshotKey(canonical, p.now())

	if err := p.snapshotStore.Put(ctx, canonical, body); err != nil {
		// Snapshot store is mandatory in this loop — without a snapshot there
		// is no snapshotKey to pass to EnqueueHarvester. Classify as network
		// per tasks §3.7 ("snapshot 저장 실패도 network로 분류").
		log.Printf("WARN pioneer_consumer: snapshot put failed url=%q err=%v", canonical, err)
		p.reportFailure(rawURL, scheduler.ErrorNetwork)
		return
	}

	links, extractErr := crawler.ExtractLinksWithSelectors(strings.NewReader(string(body)), finalURL)
	if extractErr != nil {
		// Per design.md Risks: the three success calls (Enqueue pioneer +
		// EnqueueHarvester + SetStatus fetched) form an atomic-like block; an
		// upstream failure before all three complete must surface as
		// fetch_failed + RecordFetchError so the URL is retried via the
		// scheduler's backoff path instead of being silently marked fetched.
		log.Printf("WARN pioneer_consumer: extract_links url=%q err=%v", finalURL, extractErr)
		p.reportFailure(rawURL, scheduler.ErrorNetwork)
		return
	}

	filtered := p.filterChain.Apply(links)
	urls := linkURLs(filtered)
	if len(urls) > 0 {
		if err := p.scheduler.Enqueue(scheduler.QueuePioneer, urls...); err != nil {
			// See above: mid-block failures must report fetch_failed to keep
			// the three-call success block atomic per design.md Risks.
			log.Printf("WARN pioneer_consumer: enqueue pioneer url=%q count=%d err=%v",
				rawURL, len(urls), err)
			p.reportFailure(rawURL, scheduler.ErrorNetwork)
			return
		}
	}

	if err := p.scheduler.EnqueueHarvester(rawURL, snapKey); err != nil {
		// EnqueueHarvester failure means the URL's snapshot never reaches
		// harvester_frontier and no Pin will be generated. Report fetch_failed
		// so the scheduler retries per design.md Risks "EnqueueHarvester 누락".
		log.Printf("WARN pioneer_consumer: enqueue_harvester url=%q err=%v", rawURL, err)
		p.reportFailure(rawURL, scheduler.ErrorNetwork)
		return
	}

	if err := p.scheduler.SetStatus(rawURL, scheduler.StatusFetched, nil); err != nil {
		// Note: SetStatus(fetched) failure is NOT routed through reportFailure.
		// The dual-call contract (SetStatus + RecordFetchError) applies to
		// fetch failures, not to a bookkeeping failure after a successful
		// fetch. If this write fails the row's next_fetch_at is not advanced,
		// so the scheduler's 10-min lease expiry naturally re-claims the URL
		// and the whole pipeline re-runs. EnqueueHarvester and snapshotStore.Put
		// are both idempotent (url_hash dedup + same-key overwrite), so the
		// retry is safe. Log is sufficient observability here.
		log.Printf("WARN pioneer_consumer: set_status_fetched url=%q err=%v", rawURL, err)
	}
}

// reportFailure is the spec-mandated dual call site: SetStatus(fetch_failed)
// + RecordFetchError(kind) MUST both fire on a fetch failure (tasks §3.6).
// Wrapping them in one helper removes the risk of forgetting one half.
func (p *PioneerConsumer) reportFailure(rawURL string, kind scheduler.ErrorKind) {
	if err := p.scheduler.SetStatus(rawURL, scheduler.StatusFetchFailed, nil); err != nil {
		log.Printf("WARN pioneer_consumer: set_status_fetch_failed url=%q err=%v", rawURL, err)
	}
	if err := p.scheduler.RecordFetchError(rawURL, kind); err != nil {
		log.Printf("WARN pioneer_consumer: record_fetch_error url=%q kind=%q err=%v", rawURL, string(kind), err)
	}
}

// linkURLs flattens []crawler.Link to []string for Enqueue. Order-preserving
// because downstream URLScheduler treats the array position as a tie-breaker
// for same-priority rows (depends on DB insert order).
func linkURLs(links []crawler.Link) []string {
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, l.URL)
	}
	return out
}

// classifyFetchError maps a fetch error + HTTP status to the scheduler's
// ErrorKind enum. Order matters: timeout is checked first because a timeout
// wrapped in a url.Error still satisfies net.Error.Timeout(), and we want
// "timeout" rather than the generic "network" in that case. Per spec tasks
// §3.7: 4xx → http_4xx, 5xx → http_5xx, timeout → timeout, else network.
//
// Note on "HTTP 2xx + empty body" (pioneer spec scenario, tasks §5.9): the
// DefaultConsumerFetcher already returns err != nil for zero-byte 2xx
// responses (fetcher.go). That error carries statusCode=200, which falls
// through both the 4xx and 5xx branches and lands in the default case →
// ErrorNetwork. This matches the spec: empty-body 2xx is classified
// "network" without needing a dedicated branch here.
func classifyFetchError(err error, statusCode int) scheduler.ErrorKind {
	if err == nil {
		return scheduler.ErrorNetwork
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return scheduler.ErrorTimeout
	}
	// Fall back to the error message for fetchers that don't propagate a
	// structured statusCode. The DefaultConsumerFetcher does populate
	// statusCode on HTTP errors, so the message scan is a safety net.
	if statusCode == 0 {
		statusCode = statusCodeFromErr(err)
	}
	switch {
	case statusCode >= 400 && statusCode < 500:
		return scheduler.ErrorHTTP4xx
	case statusCode >= 500 && statusCode < 600:
		return scheduler.ErrorHTTP5xx
	default:
		return scheduler.ErrorNetwork
	}
}

// httpStatusErrorPattern matches fetchHTMLShared's "HTTP error: status code N"
// template. Tolerant of leading/trailing text because errors.Wrap may prepend
// context.
var httpStatusErrorPattern = regexp.MustCompile(`status code (\d{3})`)

func statusCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	m := httpStatusErrorPattern.FindStringSubmatch(err.Error())
	if m == nil {
		return 0
	}
	code, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return 0
	}
	return code
}

// DefaultConsumerFetcher is the production ConsumerFetcher: a bounded-size
// HTTP GET with redirect cap and 10s timeout. Mirrors fetchHTMLShared's
// limits (5MB body, 5 redirects) but exposes the status code separately so
// the consumer can classify 4xx vs 5xx vs network without string parsing.
type DefaultConsumerFetcher struct {
	client *http.Client
	// maxBody caps the response size to prevent memory spikes on adversarial
	// servers. Zero falls back to the 5MB default.
	maxBody int64
}

// NewDefaultConsumerFetcher builds the production HTTP fetcher. The
// underlying client is `httpclient.NewSSRFSafeClient`: every outbound
// connection (and every redirect hop) re-resolves the host and refuses to
// dial private/reserved IP ranges. Pioneer fetches caller-untrusted URLs
// (a hrefs extracted from arbitrary external HTML) and stores the response
// body in SnapshotStore at pioneer_consumer.go:171
// `p.snapshotStore.Put(canonical, body)` — so the SHALL of
// `openspec/specs/harvester/spec.md` L739 ("외부 사이트로부터 추출된 ...
// URL에 대해 ... HTTP fetch는 SSRF-safe HTTP client를 경유해야 한다" +
// "외부 저장소에 사설 호스트 응답 바이트가 적재되지 않는다") applies
// here too: without this guard a 169.254.169.254 a href would land
// in S3 and propagate to harvester_frontier as a snapshot. Mirrors
// `harvest_pipeline.go:154` (HarvestPipeline.client) so both fetch stages
// of the bot pipeline share one SSRF policy.
func NewDefaultConsumerFetcher() *DefaultConsumerFetcher {
	return &DefaultConsumerFetcher{
		client: httpclient.NewSSRFSafeClient(httpclient.Options{
			ConnectTimeout: 5 * time.Second,
			TotalTimeout:   10 * time.Second,
			MaxRedirects:   5,
		}),
		maxBody: 5 * 1024 * 1024,
	}
}

// Fetch implements ConsumerFetcher. Returns (body, finalURL, statusCode, err).
// statusCode is populated even on non-2xx so classifyFetchError can pick
// http_4xx vs http_5xx without re-parsing the error string.
func (f *DefaultConsumerFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", 0, fmt.Errorf("pioneer_consumer: build request: %w", err)
	}
	req.Header.Set("User-Agent", "FugueBot/1.0 (+https://fugue.app)")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("pioneer_consumer: http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	finalURL := resp.Request.URL.String()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, finalURL, resp.StatusCode, fmt.Errorf("pioneer_consumer: HTTP error: status code %d", resp.StatusCode)
	}

	limit := f.maxBody
	if limit <= 0 {
		limit = 5 * 1024 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, finalURL, resp.StatusCode, fmt.Errorf("pioneer_consumer: read body: %w", err)
	}
	if len(body) == 0 {
		return nil, finalURL, resp.StatusCode, fmt.Errorf("pioneer_consumer: empty response body")
	}
	return body, finalURL, resp.StatusCode, nil
}
