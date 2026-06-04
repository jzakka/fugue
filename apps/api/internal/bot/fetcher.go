// fetcher.go implements the harvester-snapshot-first-fetch OpenSpec change.
//
// Spec SSoT: openspec/changes/harvester-snapshot-first-fetch/specs/bot/spec.md
// Pseudo reference: apps/api/fuguebot_pseudo.go lines 97-112
//
// The single public surface Harvester (and future Pioneer migration) depends
// on is the Fetcher interface. Production wires three concrete types:
//
//	CompositeFetcher{ o: ObjectStorageFetcher, h: HTTPFetcher }
//
// so the ObjectStorage-first → HTTP-fallback semantic is encapsulated
// outside Harvester's per-node loop.

package bot

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
)

// Fetcher is the single fetch surface Harvester depends on.
//
// The signature is deliberately context-less to match the spec
// (harvester-snapshot-first-fetch §1 and pseudo-code
// fuguebot_pseudo.go:94-96). Concrete implementations are responsible for
// enforcing their own timeouts — HTTPFetcher uses the 10s deadline baked
// into fetchHTMLShared; ObjectStorageFetcher inherits the SDK's request
// timeout.
//
// Implementations MUST:
//   - Return the original HTML bytes. Callers never observe gzip blobs;
//     decompression lives inside ObjectStorageFetcher (spec Decision 4).
//   - Return a non-nil error on any failure. CompositeFetcher treats any
//     error from the ObjectStorage path as a single "miss" regardless of
//     the underlying cause (spec Decision 2).
type Fetcher interface {
	Fetch(url string) ([]byte, error)
}

// HTTPFetcher is the network-side Fetcher. It delegates to fetchHTMLShared,
// which fetches caller-untrusted harvester_frontier URLs through the
// SSRF-safe HTTP client (ConnectTimeout 5s, TotalTimeout 10s, 5-redirect
// cap, 5MB body limit, FugueBot User-Agent) — the same policy Pioneer's
// DefaultConsumerFetcher uses, so both fetch stages share one SSRF guard.
type HTTPFetcher struct{}

// NewHTTPFetcher builds the default HTTP Fetcher. The struct is stateless
// so a zero value works too, but the constructor keeps call sites uniform.
func NewHTTPFetcher() *HTTPFetcher { return &HTTPFetcher{} }

// Fetch performs a GET against rawURL using fetchHTMLShared's bounded HTTP
// client. A fresh context.Background() is used because the Fetcher
// interface intentionally doesn't propagate ctx; cancellation is bounded
// by fetchHTMLShared's 10s deadline instead.
func (f *HTTPFetcher) Fetch(rawURL string) ([]byte, error) {
	htmlStr, _, err := fetchHTMLShared(context.Background(), nil, rawURL)
	if err != nil {
		return nil, err
	}
	return []byte(htmlStr), nil
}

// ObjectStorageFetcher reads the most recent same-UTC-day snapshot for a
// URL and returns the decompressed HTML.
//
// The reader is injected so callers can swap in a fake S3 client for
// tests. URL normalization uses canonicalURL — the same normalization
// Pioneer feeds into snapshot.SnapshotKey on the write side
// (pioneer_consumer.go:168), guaranteeing bit-identical keys across
// stages (spec Decision 5).
type ObjectStorageFetcher struct {
	reader snapshot.SnapshotReader
	now    func() time.Time
}

// NewObjectStorageFetcher wires a SnapshotReader. The clock defaults to
// time.Now; tests override via WithClock to freeze the UTC-date component
// of the snapshot key.
func NewObjectStorageFetcher(reader snapshot.SnapshotReader) *ObjectStorageFetcher {
	return &ObjectStorageFetcher{reader: reader, now: time.Now}
}

// WithClock overrides the time source used to date-segment the snapshot
// key. Returns the receiver so the call can be chained from the
// constructor.
func (f *ObjectStorageFetcher) WithClock(now func() time.Time) *ObjectStorageFetcher {
	if now != nil {
		f.now = now
	}
	return f
}

// Fetch looks up the gzipped snapshot under
// snapshot.SnapshotKey(canonicalURL(rawURL), now().UTC()) and returns the
// decompressed HTML. Any error — ErrSnapshotNotFound, permission, network,
// internal — propagates unchanged so CompositeFetcher can log the class;
// the sentinel class is observability-only (spec §5.1) and does NOT
// change the fallback decision.
func (f *ObjectStorageFetcher) Fetch(rawURL string) ([]byte, error) {
	normalized := canonicalURL(rawURL)
	return f.reader.Get(context.Background(), normalized, f.now())
}

// CompositeFetcher implements the ObjectStorage-first → HTTP-fallback
// semantics from harvester-snapshot-first-fetch design.md Decision 2.
//
// Any non-nil error from the ObjectStorage path is collapsed into a single
// "miss" and the request is retried against HTTP. The original error class
// is emitted as a log field for observability (§5.1) but does not branch
// the control flow.
type CompositeFetcher struct {
	o Fetcher
	h Fetcher
}

// NewCompositeFetcher composes an object-storage Fetcher with an HTTP
// Fetcher. Both arguments are required; passing nil will panic on first
// Fetch, which surfaces wiring mistakes loudly at startup rather than
// silently falling back.
func NewCompositeFetcher(objectStorage Fetcher, http Fetcher) *CompositeFetcher {
	return &CompositeFetcher{o: objectStorage, h: http}
}

// Fetch tries ObjectStorage first; on any error, falls back to HTTP.
//
// Behavior (spec §2):
//   - Snapshot hit → returns snapshot bytes, no HTTP call.
//   - Snapshot miss of any kind → HTTP fetch; whatever HTTP returns is
//     propagated (success or final error).
//
// Observability (spec §5.1): each branch emits a single-line log tagged
// with fetch source and, on miss, the ObjectStorage error classification.
func (f *CompositeFetcher) Fetch(rawURL string) ([]byte, error) {
	body, err := f.o.Fetch(rawURL)
	if err == nil {
		log.Printf("fetcher: source=snapshot url=%s bytes=%d", rawURL, len(body))
		return body, nil
	}

	log.Printf("fetcher: source=snapshot_miss reason=%s url=%s err=%v; falling back to http",
		ClassifySnapshotError(err), rawURL, err)

	body, herr := f.h.Fetch(rawURL)
	if herr != nil {
		log.Printf("fetcher: source=http_error url=%s err=%v", rawURL, herr)
		return nil, herr
	}
	log.Printf("fetcher: source=http url=%s bytes=%d", rawURL, len(body))
	return body, nil
}

// ClassifySnapshotError maps ObjectStorage sentinel errors to short
// metric-friendly labels (not_found / permission / network / internal /
// unknown). Exported so dashboards and tests can share the same label
// vocabulary. Observability only (spec §5.1) — the label never changes
// fetch behavior.
func ClassifySnapshotError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, snapshot.ErrSnapshotNotFound):
		return "not_found"
	case errors.Is(err, snapshot.ErrSnapshotPermission):
		return "permission"
	case errors.Is(err, snapshot.ErrSnapshotNetwork):
		return "network"
	case errors.Is(err, snapshot.ErrSnapshotInternal):
		return "internal"
	default:
		return "unknown"
	}
}
