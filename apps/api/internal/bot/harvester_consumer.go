// harvester_consumer.go implements the harvester-scheduler-consumer OpenSpec
// change: Harvester as a thin consumer of harvester_frontier.
//
// The per-iteration loop is:
//
//	Dequeue(QueueHarvester) → fetcher.Fetch(url) → extract → classify →
//	  createPins → SetStatus(harvested, pinIDs)
//
// In-memory BFS state (BFSQueue, visited map, site-scoped nodeMap) is
// deliberately absent; the URLScheduler owns dedup, ordering, and lease
// semantics via the partial index on harvester_frontier and
// FOR UPDATE SKIP LOCKED. Multiple HarvesterConsumer instances may run
// concurrently against the same scheduler; duplicate-URL prevention is
// scheduler's responsibility (no consumer-side advisory lock).
//
// Spec SSoT: openspec/changes/harvester-scheduler-consumer/specs/harvester/spec.md
// Scheduler contract SSoT: internal/scheduler/url_scheduler.go (URLScheduler)

package bot

import (
	"context"
	"errors"
	"log"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// genericExtractorIface is the minimal surface HarvesterConsumer needs from
// its fallback extractor. Declared as an interface (not the concrete
// *GenericExtractor) so parse-failure tests can inject a stub that errors
// — the real GenericExtractor is defensive and almost never returns an
// error, which would make the "parse → network" mapping untestable.
type genericExtractorIface interface {
	Extract(htmlBytes []byte, fetchURL string) (PinDocument, error)
}

// DocumentPipeline persists a PinDocument. The concrete implementation is
// *HarvestPipeline (harvest_pipeline.go); *MockPipeline (mocks.go) is used
// in tests. Declared as an interface here so HarvesterConsumer can swap
// implementations without a hard dependency on HarvestPipeline's private
// wiring (DB, storage, http.Client).
//
// MarkSkipped is kept on the interface because HarvestPipeline exposes it
// for non-pinnable documents; the consumer itself only calls
// ProcessDocument and delegates "skip" semantics to scheduler.SetStatus.
type DocumentPipeline interface {
	ProcessDocument(ctx context.Context, node db.BotGraphNode, doc PinDocument) (created bool, pinID uuid.UUID, err error)
	MarkSkipped(ctx context.Context, node db.BotGraphNode) error
}

// harvesterDequeueBudget caps the number of successful URLScheduler.Dequeue
// calls a single Harvester worker process performs before exiting (spec
// `harvester-worker-budget`). The constant is intentionally not exposed via
// env/config/CLI: budget changes happen by editing this value and rebuilding.
// Symmetric to Pioneer's WorkerBudget so operators reason about both workers
// with one mental model.
const harvesterDequeueBudget = 100

// HarvesterConsumer implements the Dequeue → fetch → extract → classify →
// createPins → SetStatus loop defined in the harvester-scheduler-consumer
// change. It holds only the dependencies needed by that loop and carries no
// site-scoped in-memory state.
type HarvesterConsumer struct {
	scheduler  scheduler.URLScheduler
	fetcher    Fetcher
	registry   AdapterRegistry
	extractor  genericExtractorIface
	classifier *Classifier
	pipeline   DocumentPipeline
	// budget overrides harvesterDequeueBudget for in-process tests only
	// (spec `harvester-worker-budget`, symmetric with Pioneer's
	// WorkerBudget test seam). Zero (the default) means "use
	// harvesterDequeueBudget". Production code MUST NOT set this field —
	// it is package-private and has no env/config/CLI surface, satisfying
	// the spec's "build-time constant" rule.
	budget int
}

// NewHarvesterConsumer wires the consumer with the five mandatory
// dependencies listed in the spec's Consumer loop pseudo-code. Nil registry/
// extractor/classifier fall back to zero-config defaults so CLI entry points
// and tests can construct a usable consumer without threading every knob.
func NewHarvesterConsumer(
	sched scheduler.URLScheduler,
	fetcher Fetcher,
	registry AdapterRegistry,
	extractor *GenericExtractor,
	classifier *Classifier,
	pipeline DocumentPipeline,
) *HarvesterConsumer {
	if registry == nil {
		registry = NewInMemoryAdapterRegistry()
	}
	if extractor == nil {
		extractor = NewGenericExtractor()
	}
	if classifier == nil {
		classifier = NewClassifier()
	}
	return &HarvesterConsumer{
		scheduler:  sched,
		fetcher:    fetcher,
		registry:   registry,
		extractor:  extractor,
		classifier: classifier,
		pipeline:   pipeline,
	}
}

// withExtractor swaps the fallback extractor. Used by tests to install a
// stub that errors, exercising the "parse → RecordHarvestError(network)"
// mapping. Not exported because production code always uses the
// *GenericExtractor injected via the constructor.
func (h *HarvesterConsumer) withExtractor(e genericExtractorIface) *HarvesterConsumer {
	if e != nil {
		h.extractor = e
	}
	return h
}

// Run blocks, processing URLs from harvester_frontier until ctx is cancelled
// or the worker budget is exhausted (spec `harvester-worker-budget`). Per
// spec the consumer does not sleep; empty-queue polling is internal to
// Dequeue. Duplicate-URL prevention for N>1 workers is delegated to
// scheduler's FOR UPDATE SKIP LOCKED contract.
//
// Budget accounting (spec `harvester-worker-budget`):
//   - Counter increments only after Dequeue returns a non-empty URL with no
//     error. Empty results and errors are NOT counted; the loop logs the
//     error and retries so a transient scheduler hiccup cannot kill the
//     worker before its budget is spent.
//   - The 100th URL is processed to completion — success path waits for
//     SetStatus(harvested, pinIDs) to return; failure path waits for the
//     dual-call (SetStatus(harvest_failed, nil) + RecordHarvestError) to
//     finish — only then does Run emit the structured budget-exhausted log
//     and return nil so the supervisor can restart a fresh process.
//   - ctx cancellation is independent of budget: it can exit the loop at
//     any time without violating the budget contract (budget is an upper
//     bound, not a lower bound).
func (h *HarvesterConsumer) Run(ctx context.Context) error {
	budget := h.budget
	if budget <= 0 {
		budget = harvesterDequeueBudget
	}
	dequeues := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rawURL, err := h.scheduler.Dequeue(scheduler.QueueHarvester)
		if err != nil {
			// Spec "Dequeue 자체 오류는 카운트되지 않는다": log and retry
			// instead of returning, so a transient scheduler/DB hiccup does
			// not consume any budget or kill the worker prematurely.
			// Key=value format; Pioneer symmetry is required only for the
			// budget-exhausted log below (tasks.md §2.4).
			log.Printf("WARN harvester_consumer: component=harvester_worker reason=dequeue_error err=%v", err)
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
		h.processOne(ctx, rawURL)
		if dequeues >= budget {
			log.Printf(
				`msg="harvester worker: work budget exhausted" component=harvester_worker reason=budget_exhausted dequeues=%d`,
				dequeues,
			)
			return nil
		}
	}
}

// processOne runs the per-URL pipeline. Errors are handled in-place (report
// to scheduler, log) rather than returning, so a single bad URL cannot kill
// the Run loop — the spec treats the consumer as an always-on daemon.
//
// Status transitions on every exit path:
//   - success, Pinnable=true : SetStatus(harvested, pinIDs) — single call
//   - success, Pinnable=false: SetStatus(harvested, nil)     — single call
//   - any failure            : SetStatus(harvest_failed, nil) +
//                              RecordHarvestError(errorKind) — dual call
//
// ctx is threaded to extractDocument and createPins (which carry it into
// adapter scripts and the DocumentPipeline) but the fetch and scheduler
// calls use their own internal timeouts; SIGTERM therefore takes effect at
// the next loop iteration rather than mid-URL. This matches the spec's
// graceful-shutdown requirement that the in-flight URL completes its final
// status transition before Run returns.
func (h *HarvesterConsumer) processOne(ctx context.Context, rawURL string) {
	body, fetchErr := h.fetcher.Fetch(rawURL)
	if fetchErr != nil {
		kind := classifyHarvestFetchError(fetchErr)
		h.reportFailure(rawURL, kind, "fetch", fetchErr)
		return
	}

	// Extract + classify. Per spec Decision 6, pipeline.Process failure is
	// tagged internally as "parse" for logs/metrics and reported to
	// scheduler as "network" (retriable). A fresh empty BotGraphNode is
	// passed because the scheduler-consumer model has no bot_graph_nodes
	// row: HarvestPipeline.ProcessDocument's node parameter is unused in
	// production (harvest_pipeline.go).
	doc, fellBack, extractErr := h.extractDocument(ctx, body, rawURL)
	if extractErr != nil {
		h.reportFailure(rawURL, scheduler.ErrorNetwork, "parse", extractErr)
		return
	}
	if fellBack {
		log.Printf("harvester_consumer: adapter fell back to generic for url=%q", rawURL)
	}

	linkStats := ComputeLinkStats(body)
	pinnable, reason := h.classifier.Classify(doc, linkStats)
	doc.OGData.Classifier = &ClassifierVerdict{Pinnable: pinnable, Reason: reason}

	if !pinnable {
		// Pinnable=false: no Pin created, but the URL is still "done".
		// SetStatus marks harvested_at and resets harvest_error_count to 0
		// (scheduler Decision 5) so the row exits the partial index.
		if err := h.scheduler.SetStatus(rawURL, scheduler.StatusHarvested, nil); err != nil {
			log.Printf("WARN harvester_consumer: set_status_harvested_skipped url=%q err=%v", rawURL, err)
		}
		return
	}

	// Rune-safe truncation to fit pins.description (500 runes). Must happen
	// before ProcessDocument because the pipeline writes BodyText verbatim.
	doc.BodyText = truncateRunes(doc.BodyText, 500)

	pinIDs, createErr := h.createPins(ctx, doc)
	if createErr != nil {
		// Pipeline persistence failure — retriable per Decision 6.
		h.reportFailure(rawURL, scheduler.ErrorNetwork, "pin_create", createErr)
		return
	}

	// Success path: harvested_at UPDATE + harvest_error_count=0 reset +
	// harvester_frontier_pins bulk INSERT all in one transaction on the
	// scheduler side. A nil/empty pinIDs slice is valid; the pin mapping
	// INSERT is a no-op in that case (e.g. if a pipeline ever returns a
	// zero pinID without error, though createPins currently only returns
	// len==1 on success).
	if err := h.scheduler.SetStatus(rawURL, scheduler.StatusHarvested, pinIDs); err != nil {
		log.Printf("WARN harvester_consumer: set_status_harvested url=%q pin_count=%d err=%v",
			rawURL, len(pinIDs), err)
	}
}

// extractDocument resolves the domain's adapter (if any) and runs it. On
// adapter error falls back to the generic extractor. The (fellBack bool)
// second return is observability-only; the first failure-class branch in
// spec tasks §3.2 ("parse") covers both adapter-fail-then-generic-fail and
// generic-fail alone.
func (h *HarvesterConsumer) extractDocument(ctx context.Context, body []byte, fetchURL string) (PinDocument, bool, error) {
	domain := hostnameOf(fetchURL)
	adapter, hasAdapter := h.registry.Resolve(domain)

	if hasAdapter {
		// NodeType was previously threaded through from bot_graph_nodes.
		// Consumer model has no per-node type context so we pass an empty
		// string; script adapters that rely on node_type will no-op and
		// fall back to generic.
		adapterCtx := WithNodeType(ctx, "")
		doc, err := adapter.Extract(adapterCtx, body, fetchURL)
		if err == nil {
			FillExtractorIdentity(&doc, adapter.Name())
			return doc, false, nil
		}
		log.Printf("harvester_consumer: adapter %s failed (%v); falling back to generic", adapter.Name(), err)
		genericDoc, gErr := h.extractor.Extract(body, fetchURL)
		if gErr != nil {
			return genericDoc, true, gErr
		}
		FillExtractorIdentity(&genericDoc, "generic")
		return genericDoc, true, nil
	}

	doc, err := h.extractor.Extract(body, fetchURL)
	if err != nil {
		return doc, false, err
	}
	FillExtractorIdentity(&doc, "generic")
	return doc, false, nil
}

// createPins persists the PinDocument as one or more Pin rows and returns
// the resulting pin IDs. The current DocumentPipeline.ProcessDocument
// contract returns a single pinID per document (one URL → one Pin), so the
// slice always has length 1 on success. Keeping the return as []uuid.UUID
// matches scheduler.SetStatus and leaves room for a future N-Pin fanout
// without changing the loop shape (spec design.md Decision 4).
func (h *HarvesterConsumer) createPins(ctx context.Context, doc PinDocument) ([]uuid.UUID, error) {
	_, pinID, err := h.pipeline.ProcessDocument(ctx, db.BotGraphNode{}, doc)
	if err != nil {
		return nil, err
	}
	return []uuid.UUID{pinID}, nil
}

// reportFailure is the spec-mandated dual call site (Decision 6):
// SetStatus(harvest_failed, nil) + RecordHarvestError(errorKind) MUST both
// fire on any failure. Wrapping them in one helper removes the risk of
// forgetting one half. `stage` and `cause` are for observability only —
// errorKind is the single source of truth the scheduler reads.
func (h *HarvesterConsumer) reportFailure(rawURL string, kind scheduler.ErrorKind, stage string, cause error) {
	log.Printf("WARN harvester_consumer: stage=%s url=%q kind=%q err=%v", stage, rawURL, string(kind), cause)
	if err := h.scheduler.SetStatus(rawURL, scheduler.StatusHarvestFailed, nil); err != nil {
		log.Printf("WARN harvester_consumer: set_status_harvest_failed url=%q err=%v", rawURL, err)
	}
	if err := h.scheduler.RecordHarvestError(rawURL, kind); err != nil {
		log.Printf("WARN harvester_consumer: record_harvest_error url=%q kind=%q err=%v", rawURL, string(kind), err)
	}
}

// classifyHarvestFetchError maps a Fetcher.Fetch error to scheduler's
// ErrorKind enum. Restricted to the 4 values scheduler accepts (spec
// Decision 6): http_4xx, http_5xx, network, timeout. Callers MUST NOT pass
// any other string to RecordHarvestError — scheduler returns
// ErrUnknownErrorKind and the row is not updated, which leaves the URL
// stuck until lease expiry.
//
// The structural patterns mirror pioneer_consumer.classifyFetchError but
// are kept separate because Fetcher.Fetch does not expose an HTTP
// statusCode separately; we recover it from the error message via regex.
func classifyHarvestFetchError(err error) scheduler.ErrorKind {
	if err == nil {
		return scheduler.ErrorNetwork
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return scheduler.ErrorTimeout
	}
	statusCode := harvestStatusCodeFromErr(err)
	switch {
	case statusCode >= 400 && statusCode < 500:
		return scheduler.ErrorHTTP4xx
	case statusCode >= 500 && statusCode < 600:
		return scheduler.ErrorHTTP5xx
	default:
		return scheduler.ErrorNetwork
	}
}

// harvestHTTPStatusErrorPattern matches fetchHTMLShared's
// "HTTP error: status code N" phrasing. Tolerant of leading/trailing text
// because CompositeFetcher's fallback may prepend additional context.
var harvestHTTPStatusErrorPattern = regexp.MustCompile(`status code (\d{3})`)

func harvestStatusCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	m := harvestHTTPStatusErrorPattern.FindStringSubmatch(err.Error())
	if m == nil {
		return 0
	}
	code, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return 0
	}
	return code
}

// hostnameOf returns the lowercase hostname of rawURL, or "" if invalid.
// Shared helper retained after the BFS Harvester removal because the
// adapter registry (`Resolve(domain)`) keys on the bare hostname.
func hostnameOf(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// truncateRunes returns the first n runes of s. Cuts on rune boundaries so
// multi-byte characters are never split. Used to fit PinDocument.BodyText
// into the pins.description column's 500-rune limit before persistence.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
