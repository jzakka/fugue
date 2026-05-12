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
	"log"
	"net/url"
	"strings"
	"sync/atomic"
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
	scheduler scheduler.URLScheduler
	// fetcher is the snapshot-first fetch entry point. The consumer is
	// forbidden from calling the low-level Fetcher interface directly
	// (spec "Consumer는 snapshot-first 진입점만 경유하여 fetch를 수행한다"),
	// so the field type is the entry-point interface, not Fetcher.
	fetcher  SnapshotFirstFetch
	registry AdapterRegistry
	extractor  genericExtractorIface
	classifier *Classifier
	pipeline   DocumentPipeline
	// validator filters invalid media candidates from PinDocument before
	// the classifier runs (harvester-media-validation change, design.md D2).
	// nil means validation is disabled (legacy behavior, retained for tests
	// that don't need the network round to ffprobe/HTTP).
	validator MediaValidator
	// validationMetrics is the process-local counter set fed by the
	// validator + classifier. nil means metric collection is disabled
	// (tasks.md §5 — operational guidance, not a spec contract).
	validationMetrics *MediaValidationMetrics
	// budget overrides harvesterDequeueBudget for in-process tests only
	// (spec `harvester-worker-budget`, symmetric with Pioneer's
	// WorkerBudget test seam). Zero (the default) means "use
	// harvesterDequeueBudget". Production code MUST NOT set this field —
	// it is package-private and has no env/config/CLI surface, satisfying
	// the spec's "build-time constant" rule.
	budget int

	// fetchFailureCount is the in-memory worker-stat counter required by
	// harvester-snapshot-first-fetch tasks §3.2 and design.md Decision 3.
	// It is explicitly distinct from harvester_frontier.harvest_error_count
	// (DB column): the DB counter tracks per-URL retry state for scheduler
	// backoff, while this counter aggregates per-process fetch failures
	// (CompositeFetcher dual-miss: ObjectStorage + HTTP both failed) for
	// worker-level observability.
	fetchFailureCount atomic.Uint64

	// stats holds the 5 per-node category counters defined by harvester spec
	// "Harvester 노드 단위 통계 정의" (PinsCreated/Deduped/Skipped/Failed/
	// AdapterFallback). Incremented exclusively at processOne exit paths so
	// the "exactly one primary category per node" invariant is enforced in
	// one place (fix-harvester-node-stats design Decision 3).
	stats nodeStats
}

// nodeStats holds the 5 per-node category counters required by harvester
// spec. Each counter is updated independently via atomic.Add; the 5-tuple
// snapshot is NOT atomic (see NodeStats doc).
type nodeStats struct {
	pinsCreated     atomic.Uint64
	deduped         atomic.Uint64
	skipped         atomic.Uint64
	failed          atomic.Uint64
	adapterFallback atomic.Uint64
}

// NodeStatsSnapshot is a plain-value view of nodeStats at a moment in time.
// The snapshot is NOT atomic: each field is read with an independent
// atomic.Load, so a concurrent counter increment can leave the snapshot
// internally inconsistent (e.g. PinsCreated reflects the increment but
// AdapterFallback does not). This is acceptable because node-level stats
// are for trend observation, not exact invariants.
type NodeStatsSnapshot struct {
	PinsCreated     uint64
	Deduped         uint64
	Skipped         uint64
	Failed          uint64
	AdapterFallback uint64
}

// NodeStats returns a snapshot of the 5 per-node category counters for this
// consumer instance. The snapshot is non-atomic (see NodeStatsSnapshot doc).
// Counters are process-local and zero at construction time; they are
// discarded on worker exit and not shared across workers (spec
// "Dequeue 카운터는 워커 간 공유 상태가 아니다").
func (h *HarvesterConsumer) NodeStats() NodeStatsSnapshot {
	return NodeStatsSnapshot{
		PinsCreated:     h.stats.pinsCreated.Load(),
		Deduped:         h.stats.deduped.Load(),
		Skipped:         h.stats.skipped.Load(),
		Failed:          h.stats.failed.Load(),
		AdapterFallback: h.stats.adapterFallback.Load(),
	}
}

// NewHarvesterConsumer wires the consumer with the five mandatory
// dependencies listed in the spec's Consumer loop pseudo-code. Nil registry/
// extractor/classifier fall back to zero-config defaults so CLI entry points
// and tests can construct a usable consumer without threading every knob.
func NewHarvesterConsumer(
	sched scheduler.URLScheduler,
	fetcher SnapshotFirstFetch,
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

// WithMediaValidator installs a MediaValidator on the consumer. When set the
// consumer applies FilterValidMedia between extractDocument and the
// classifier so invalid candidates (1x1 placeholders, undecodable bytes,
// sub-threshold video/audio) never reach Pin canonical storage. Pass nil to
// disable validation (legacy behavior). See harvester-media-validation
// design.md D2.
func (h *HarvesterConsumer) WithMediaValidator(v MediaValidator) *HarvesterConsumer {
	h.validator = v
	return h
}

// HasMediaValidator reports whether a MediaValidator has been installed on the
// consumer. Used by bootstrap regression tests to assert that production
// wiring did not silently drop the validator. See
// fix-harvester-wire-media-validator design.md D2.
func (h *HarvesterConsumer) HasMediaValidator() bool {
	return h.validator != nil
}

// WithValidationMetrics installs a metrics sink for the validator and
// classifier signals (tasks.md §5). nil disables collection.
func (h *HarvesterConsumer) WithValidationMetrics(m *MediaValidationMetrics) *HarvesterConsumer {
	h.validationMetrics = m
	return h
}

// ValidationMetrics returns the metrics sink installed on this consumer or
// nil when none is wired. Exposed for in-process observability and tests.
func (h *HarvesterConsumer) ValidationMetrics() *MediaValidationMetrics {
	return h.validationMetrics
}

// FetchFailureCount returns the in-memory total of fetch failures
// observed by this consumer instance since construction. Exposed for
// worker-level observability and tests (tasks §3.2, §4.8). This counter
// is process-local and independent of the scheduler's per-URL
// harvest_error_count DB column.
func (h *HarvesterConsumer) FetchFailureCount() uint64 {
	return h.fetchFailureCount.Load()
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
//     RecordHarvestError(errorKind) — dual call
//
// ctx is threaded to extractDocument and createPins (which carry it into
// adapter scripts and the DocumentPipeline) but the fetch and scheduler
// calls use their own internal timeouts; SIGTERM therefore takes effect at
// the next loop iteration rather than mid-URL. This matches the spec's
// graceful-shutdown requirement that the in-flight URL completes its final
// status transition before Run returns.
func (h *HarvesterConsumer) processOne(ctx context.Context, rawURL string) {
	// spec: harvester "Consumer는 snapshot-first 진입점만 경유하여 fetch를
	// 수행한다" + "fetch 단 errorKind는 4종으로 한정된다" — consumer passes
	// the entry point's ctx and rawURL through unchanged (Dequeue URL) and
	// uses the returned kind verbatim; no re-classification on this side.
	body, fetchKind, fetchErr := h.fetcher.Fetch(ctx, rawURL)
	if fetchErr != nil {
		h.fetchFailureCount.Add(1)
		h.stats.failed.Add(1)
		h.reportFailure(rawURL, fetchKind, "fetch", fetchErr)
		return
	}

	// Extract + classify. Per spec Decision 6, pipeline.Process failure is
	// tagged internally as "parse" for logs/metrics and reported to
	// scheduler as "network" (retriable). A fresh empty BotGraphNode is
	// passed because the scheduler-consumer model has no bot_graph_nodes
	// row: HarvestPipeline.ProcessDocument's node parameter is unused in
	// production (harvest_pipeline.go).
	doc, fellBack, extractErr := h.extractDocument(ctx, body, rawURL)
	// spec: harvester "Harvester 노드 단위 통계 정의" — 본문 SHALL 보강
	// (어댑터 실패가 발생하면 generic 성공/실패와 무관하게 AdapterFallback이
	// 1 증가) + Scenario "어댑터 실패 후 generic 실패" enforce. 이 분기는
	// extractErr 조기 반환보다 반드시 앞에 있어야 한다.
	if fellBack {
		h.stats.adapterFallback.Add(1)
		log.Printf("harvester_consumer: adapter fell back to generic for url=%q", rawURL)
	}
	if extractErr != nil {
		h.stats.failed.Add(1)
		h.reportFailure(rawURL, scheduler.ErrorNetwork, "parse", extractErr)
		return
	}

	// Filter invalid media candidates BEFORE classification. After this
	// step, doc.MediaCandidates / doc.ThumbnailURL contain only validated
	// references; rejection counts/reasons are recorded on
	// doc.OGData.MediaValidation. The classifier's existing "no_primary_media"
	// rule (empty thumbnail + empty candidates) then naturally handles
	// pages where every candidate was invalid. See harvester-media-validation
	// design.md D2.
	if h.validator != nil {
		FilterValidMedia(ctx, h.validator, &doc)
		if h.validationMetrics != nil && doc.OGData.MediaValidation != nil {
			for reasonKey, count := range doc.OGData.MediaValidation.Reasons {
				h.validationMetrics.RecordRejectionN(MediaValidationReason(reasonKey), count)
			}
		}
	}

	linkStats := ComputeLinkStats(body)
	pinnable, reason := h.classifier.Classify(doc, linkStats)
	doc.OGData.Classifier = &ClassifierVerdict{Pinnable: pinnable, Reason: reason}
	if h.validationMetrics != nil {
		h.validationMetrics.RecordClassification(pinnable, reason)
	}

	if !pinnable {
		// Pinnable=false: no Pin created, but the URL is still "done".
		// SetStatus marks harvested_at and resets harvest_error_count to 0
		// (scheduler Decision 5) so the row exits the partial index.
		h.stats.skipped.Add(1)
		if err := h.scheduler.SetStatus(rawURL, scheduler.StatusHarvested, nil); err != nil {
			log.Printf("WARN harvester_consumer: set_status_harvested_skipped url=%q err=%v", rawURL, err)
		}
		return
	}

	// Rune-safe truncation to fit pins.description (500 runes). Must happen
	// before ProcessDocument because the pipeline writes BodyText verbatim.
	doc.BodyText = truncateRunes(doc.BodyText, 500)

	pinIDs, created, createErr := h.createPins(ctx, doc)
	if createErr != nil {
		// Pipeline persistence failure — retriable per Decision 6.
		h.stats.failed.Add(1)
		h.reportFailure(rawURL, scheduler.ErrorNetwork, "pin_create", createErr)
		return
	}
	if created {
		h.stats.pinsCreated.Add(1)
	} else {
		h.stats.deduped.Add(1)
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
// the resulting pin IDs along with the pipeline's `created` flag (true =
// new insert, false = idempotent update). The current
// DocumentPipeline.ProcessDocument contract returns a single pinID per
// document (one URL → one Pin), so the slice always has length 1 on
// success. Keeping the return as []uuid.UUID matches scheduler.SetStatus
// and leaves room for a future N-Pin fanout without changing the loop
// shape (spec design.md Decision 4). The `created` bool is surfaced so
// processOne can split the success path into PinsCreated vs Deduped
// counters (fix-harvester-node-stats design Decision 4).
func (h *HarvesterConsumer) createPins(ctx context.Context, doc PinDocument) ([]uuid.UUID, bool, error) {
	created, pinID, err := h.pipeline.ProcessDocument(ctx, db.BotGraphNode{}, doc)
	if err != nil {
		return nil, false, err
	}
	return []uuid.UUID{pinID}, created, nil
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
