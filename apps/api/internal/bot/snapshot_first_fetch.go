// snapshot_first_fetch.go implements the snapshot-first fetch entry point
// required by the harvester spec L563-657 ("Consumer는 snapshot-first 진입점만
// 경유하여 fetch를 수행한다" plus the 4 related Requirements).
//
// The consumer's only fetch surface is SnapshotFirstFetch.Fetch(ctx, url).
// All status/error classification happens inside this entry point, so the
// consumer just passes the returned ErrorKind through to
// scheduler.RecordHarvestError without re-parsing HTTP responses or error
// strings (spec "consumer는 fetch 실패 경로에서 errorKind를 재분류하지 않는다").
package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// SnapshotFirstFetch is the single fetch surface the harvester consumer is
// allowed to call (spec "Consumer는 snapshot-first 진입점만 경유하여 fetch를
// 수행한다"). The (ctx, url) input matches scheduler.Dequeue's URL contract
// (spec "snapshot-first 진입점의 입력은 (ctx, url)이며 scheduler.Dequeue 반환
// 형태와 정합한다"). The 3-tuple return follows the semantics of spec
// "snapshot-first 진입점의 반환은 3-tuple (html, errorKind, err) 의미론을
// 따른다": on success html is non-empty and err is nil; on failure html is
// nil, err is non-nil, and errorKind is one of the four values defined by
// "fetch 단 errorKind는 4종으로 한정된다".
type SnapshotFirstFetch interface {
	Fetch(ctx context.Context, url string) (html []byte, kind scheduler.ErrorKind, err error)
}

// HTTPStatusError carries the HTTP status code returned by the live fetch
// path in a structured form so the entry point can classify 4xx/5xx without
// re-parsing the error message (design.md Decision 7). The Error() string
// keeps the legacy "HTTP error: status code N" phrasing so other consumers
// (Pioneer's httpStatusErrorPattern regex) that read the message verbatim
// continue to work unchanged.
type HTTPStatusError struct {
	Code int
}

// Error returns the legacy phrasing kept for backwards compatibility with
// callers that still pattern-match on the message body.
func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP error: status code %d", e.Code)
}

// SnapshotFirstFetcher is the production implementation of SnapshotFirstFetch.
// It owns an ObjectStorage Fetcher and an HTTP Fetcher; the consumer never
// sees either, satisfying spec "consumer 모듈에 ObjectStorage/HTTP 클라이언트
// 의존 부재" because the concrete implementations are injected here, not at
// the consumer boundary.
//
// Behavior:
//   - ctx.Err() is checked before each stage; a cancelled or deadline-expired
//     ctx short-circuits to errorKind = "timeout" without attempting fallback
//     (spec "ctx 취소/deadline은 스냅샷 경로 내부 실패로 분류되지 않고
//     'timeout'으로 귀결된다").
//   - The ObjectStorage stage is tried first. ANY non-ctx error from this
//     stage is absorbed and the HTTP fallback runs; the specific failure
//     class is emitted to logs only and never propagated to the consumer
//     (spec "스냅샷 내부 실패 종류는 consumer의 errorKind에 노출되지
//     않는다", "스냅샷 키 부재는 consumer에 snapshot 전용 kind로 노출되지
//     않는다", "스냅샷 경로 네트워크/권한/내부 에러도 동일하게 HTTP fallback").
//   - The HTTP fallback's result (success or 4-way errorKind) is the final
//     return value.
type SnapshotFirstFetcher struct {
	objectStorage Fetcher
	http          Fetcher
}

// NewSnapshotFirstFetcher composes an ObjectStorage Fetcher with an HTTP
// Fetcher. Both arguments are required; passing nil will panic on the first
// Fetch call, which surfaces wiring mistakes loudly at startup.
func NewSnapshotFirstFetcher(objectStorage Fetcher, http Fetcher) *SnapshotFirstFetcher {
	return &SnapshotFirstFetcher{objectStorage: objectStorage, http: http}
}

// Fetch runs the snapshot-first sequence (ObjectStorage then HTTP fallback)
// and returns the 3-tuple (html, errorKind, err) that the consumer reads.
//
// Success: html is non-empty, err is nil, kind is the zero ErrorKind. The
// consumer MUST NOT branch on kind in the success path (spec Scenario "성공
// 경로에서 consumer는 errorKind를 사용하지 않는다").
//
// Failure: html is nil, err is non-nil, and kind is one of
// scheduler.ErrorHTTP4xx, scheduler.ErrorHTTP5xx, scheduler.ErrorNetwork,
// scheduler.ErrorTimeout (spec "fetch 단 errorKind는 4종으로 한정된다").
func (s *SnapshotFirstFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, scheduler.ErrorKind, error) {
	if err := ctx.Err(); err != nil {
		return nil, scheduler.ErrorTimeout, err
	}

	body, osErr := s.objectStorage.Fetch(rawURL)
	if osErr == nil {
		log.Printf("snapshot_first_fetch: source=snapshot url=%s bytes=%d", rawURL, len(body))
		return body, "", nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, scheduler.ErrorTimeout, ctxErr
	}

	log.Printf("snapshot_first_fetch: source=snapshot_miss reason=%s url=%s err=%v; falling back to http",
		ClassifySnapshotError(osErr), rawURL, osErr)

	body, httpErr := s.http.Fetch(rawURL)
	if httpErr == nil {
		log.Printf("snapshot_first_fetch: source=http url=%s bytes=%d", rawURL, len(body))
		return body, "", nil
	}

	kind := classifyFetchFailure(ctx, httpErr)
	log.Printf("snapshot_first_fetch: source=http_error url=%s kind=%s err=%v",
		rawURL, string(kind), httpErr)
	return nil, kind, httpErr
}

// classifyFetchFailure maps a live HTTP fetch error to one of the four
// scheduler.ErrorKind values defined by spec "fetch 단 errorKind는 4종으로
// 한정된다". Precedence:
//  1. ctx cancellation/deadline → timeout
//  2. net.Error with Timeout()==true → timeout
//  3. *HTTPStatusError 4xx → http_4xx
//  4. *HTTPStatusError 5xx → http_5xx
//  5. anything else → network (DNS, TCP connect, TLS handshake, EOF, etc.)
//
// This is the ONLY place fetch-time classification happens; the consumer
// passes the returned kind verbatim to scheduler.RecordHarvestError.
func classifyFetchFailure(ctx context.Context, err error) scheduler.ErrorKind {
	// Defensive guard: the only current caller passes a non-nil err, but
	// keeping this branch lets future callers (e.g., adapter-side wrappers)
	// reuse the function without re-implementing the precedence table.
	if err == nil {
		return scheduler.ErrorNetwork
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return scheduler.ErrorTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return scheduler.ErrorTimeout
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.Code >= 400 && statusErr.Code < 500:
			return scheduler.ErrorHTTP4xx
		case statusErr.Code >= 500 && statusErr.Code < 600:
			return scheduler.ErrorHTTP5xx
		}
	}
	return scheduler.ErrorNetwork
}
