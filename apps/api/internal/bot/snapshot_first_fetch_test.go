// snapshot_first_fetch_test.go covers the 4-way errorKind mapping and
// ObjectStorage→HTTP fallback semantics required by harvester spec L563-657.
// These are the unit-layer tests for SnapshotFirstFetcher; the Harvester-loop
// integration assertions live in harvester_consumer_test.go.

package bot

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// fixedFetcher returns canned (body, err) for every URL. Used in tests that
// only need one URL.
type fixedFetcher struct {
	calls int
	body  []byte
	err   error
}

func (f *fixedFetcher) Fetch(_ string) ([]byte, error) {
	f.calls++
	return f.body, f.err
}

// netTimeoutErr implements net.Error with Timeout()==true so tests can
// exercise the transport-timeout precedence branch of classifyFetchFailure.
type netTimeoutErr struct{}

func (netTimeoutErr) Error() string   { return "i/o timeout" }
func (netTimeoutErr) Timeout() bool   { return true }
func (netTimeoutErr) Temporary() bool { return true }

// Sanity: netTimeoutErr satisfies the net.Error interface so errors.As inside
// classifyFetchFailure can unwrap it.
var _ net.Error = netTimeoutErr{}

// TestSnapshotFirstFetcher_HTTP4xx pins spec "HTTP 4xx 응답은 'http_4xx'로
// 매핑된다": when the HTTP fallback surfaces an HTTPStatusError with a 4xx
// code, the entry point MUST return kind=http_4xx, html=nil, err=non-nil.
func TestSnapshotFirstFetcher_HTTP4xx(t *testing.T) {
	snap := &fixedFetcher{err: errors.New("snapshot miss")}
	http := &fixedFetcher{err: &HTTPStatusError{Code: 404}}

	f := NewSnapshotFirstFetcher(snap, http)
	body, kind, err := f.Fetch(context.Background(), "https://a.example/missing")

	if err == nil {
		t.Fatalf("expected non-nil err on 4xx, got nil")
	}
	if kind != scheduler.ErrorHTTP4xx {
		t.Fatalf("kind = %q, want %q", kind, scheduler.ErrorHTTP4xx)
	}
	if body != nil {
		t.Fatalf("body must be nil on failure, got %q", body)
	}
}

// TestSnapshotFirstFetcher_HTTP5xx pins spec "HTTP 5xx 응답은 'http_5xx'로
// 매핑된다".
func TestSnapshotFirstFetcher_HTTP5xx(t *testing.T) {
	snap := &fixedFetcher{err: errors.New("snapshot miss")}
	http := &fixedFetcher{err: &HTTPStatusError{Code: 502}}

	f := NewSnapshotFirstFetcher(snap, http)
	_, kind, err := f.Fetch(context.Background(), "https://a.example/oops")

	if err == nil {
		t.Fatalf("expected non-nil err on 5xx, got nil")
	}
	if kind != scheduler.ErrorHTTP5xx {
		t.Fatalf("kind = %q, want %q", kind, scheduler.ErrorHTTP5xx)
	}
}

// TestSnapshotFirstFetcher_Network pins spec "DNS/connect/TLS 실패 등 transport
// 단계 에러는 'network'로 매핑된다": a transport-level error that is neither a
// timeout nor an HTTPStatusError MUST map to kind=network.
func TestSnapshotFirstFetcher_Network(t *testing.T) {
	snap := &fixedFetcher{err: errors.New("snapshot miss")}
	http := &fixedFetcher{err: errors.New("dial tcp 1.2.3.4:443: no such host")}

	f := NewSnapshotFirstFetcher(snap, http)
	_, kind, err := f.Fetch(context.Background(), "https://a.example/dns")

	if err == nil {
		t.Fatalf("expected non-nil err on transport failure, got nil")
	}
	if kind != scheduler.ErrorNetwork {
		t.Fatalf("kind = %q, want %q", kind, scheduler.ErrorNetwork)
	}
}

// TestSnapshotFirstFetcher_Timeout_NetError pins spec "transport timeout은
// 'timeout'으로 매핑된다": a net.Error with Timeout()==true MUST map to
// kind=timeout (not network).
func TestSnapshotFirstFetcher_Timeout_NetError(t *testing.T) {
	snap := &fixedFetcher{err: errors.New("snapshot miss")}
	http := &fixedFetcher{err: netTimeoutErr{}}

	f := NewSnapshotFirstFetcher(snap, http)
	_, kind, err := f.Fetch(context.Background(), "https://a.example/slow")

	if err == nil {
		t.Fatalf("expected non-nil err on timeout, got nil")
	}
	if kind != scheduler.ErrorTimeout {
		t.Fatalf("kind = %q, want %q", kind, scheduler.ErrorTimeout)
	}
}

// TestSnapshotFirstFetcher_CtxCanceledShortCircuits pins spec "ctx 취소/
// deadline은 스냅샷 경로 내부 실패로 분류되지 않고 'timeout'으로 귀결된다":
// a pre-cancelled ctx MUST short-circuit before any Fetcher call and return
// kind=timeout. The ObjectStorage Fetcher must NOT be invoked (no
// classification of internal snapshot errors as snapshot-specific kinds).
func TestSnapshotFirstFetcher_CtxCanceledShortCircuits(t *testing.T) {
	snap := &fixedFetcher{body: []byte("<html>snap</html>")}
	http := &fixedFetcher{body: []byte("<html>http</html>")}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f := NewSnapshotFirstFetcher(snap, http)
	body, kind, err := f.Fetch(ctx, "https://a.example/x")

	if err == nil {
		t.Fatalf("expected non-nil err on cancelled ctx, got nil")
	}
	if kind != scheduler.ErrorTimeout {
		t.Fatalf("kind = %q, want %q", kind, scheduler.ErrorTimeout)
	}
	if body != nil {
		t.Fatalf("body must be nil on cancelled ctx, got %q", body)
	}
	if snap.calls != 0 {
		t.Errorf("ObjectStorage MUST NOT be called on cancelled ctx, got %d calls", snap.calls)
	}
	if http.calls != 0 {
		t.Errorf("HTTP MUST NOT be called on cancelled ctx, got %d calls", http.calls)
	}
}

// TestSnapshotFirstFetcher_SnapshotHit_SuccessAndNoHTTPCall pins spec "성공
// 반환 형태": on ObjectStorage hit, html is non-empty, kind is the zero
// ErrorKind, err is nil, and the HTTP fallback MUST NOT be invoked.
func TestSnapshotFirstFetcher_SnapshotHit_SuccessAndNoHTTPCall(t *testing.T) {
	snap := &fixedFetcher{body: []byte("<html>from-snap</html>")}
	http := &fixedFetcher{body: []byte("<html>from-http</html>")}

	f := NewSnapshotFirstFetcher(snap, http)
	body, kind, err := f.Fetch(context.Background(), "https://a.example/x")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != scheduler.ErrorKind("") {
		t.Fatalf("kind on success must be zero, got %q", kind)
	}
	if string(body) != "<html>from-snap</html>" {
		t.Fatalf("body = %q, want snap bytes", body)
	}
	if http.calls != 0 {
		t.Errorf("HTTP MUST NOT be called on snapshot hit, got %d calls", http.calls)
	}
}

// TestSnapshotFirstFetcher_SnapshotMissThenHTTPSuccess pins spec "스냅샷 키
// 부재는 consumer에 snapshot 전용 kind로 노출되지 않는다", "스냅샷 경로
// 네트워크/권한/내부 에러도 동일하게 HTTP fallback": ANY ObjectStorage error
// MUST be absorbed and the HTTP fallback's success surfaces as a clean
// success (no kind, no err) to the consumer.
func TestSnapshotFirstFetcher_SnapshotMissThenHTTPSuccess(t *testing.T) {
	cases := []struct {
		name   string
		snapEr error
	}{
		{"not_found", snapshot.ErrSnapshotNotFound},
		{"permission", snapshot.ErrSnapshotPermission},
		{"network", snapshot.ErrSnapshotNetwork},
		{"internal", snapshot.ErrSnapshotInternal},
		{"unknown", errors.New("opaque snapshot error")},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			snap := &fixedFetcher{err: tc.snapEr}
			http := &fixedFetcher{body: []byte("<html>from-http</html>")}

			f := NewSnapshotFirstFetcher(snap, http)
			body, kind, err := f.Fetch(context.Background(), "https://a.example/x")

			if err != nil {
				t.Fatalf("HTTP fallback success must absorb snapshot error %v, got err=%v", tc.snapEr, err)
			}
			if kind != scheduler.ErrorKind("") {
				t.Fatalf("kind on fallback success must be zero, got %q", kind)
			}
			if string(body) != "<html>from-http</html>" {
				t.Fatalf("body = %q, want http bytes", body)
			}
		})
	}
}

// TestSnapshotFirstFetcher_SnapshotMissThenHTTP5xx pins spec "스냅샷 경로
// 내부 실패 종류는 consumer의 errorKind에 노출되지 않는다": when both stages
// fail, only the HTTP fallback's 4-way kind is surfaced; the ObjectStorage
// error class is never propagated.
func TestSnapshotFirstFetcher_SnapshotMissThenHTTP5xx(t *testing.T) {
	snap := &fixedFetcher{err: snapshot.ErrSnapshotPermission}
	http := &fixedFetcher{err: &HTTPStatusError{Code: 503}}

	f := NewSnapshotFirstFetcher(snap, http)
	_, kind, err := f.Fetch(context.Background(), "https://a.example/x")

	if err == nil {
		t.Fatalf("expected non-nil err on dual miss, got nil")
	}
	if kind != scheduler.ErrorHTTP5xx {
		t.Fatalf("kind = %q, want %q (snapshot permission must NOT leak as a snapshot-specific kind)",
			kind, scheduler.ErrorHTTP5xx)
	}
}

// TestSnapshotFirstFetcher_CtxDeadlineDuringFallback covers the precedence
// rule: if the ctx is cancelled while the ObjectStorage stage is running and
// returns an error, the entry point MUST return kind=timeout and MUST NOT
// invoke the HTTP fallback. The ObjectStorage stub blocks until ctx fires.
func TestSnapshotFirstFetcher_CtxDeadlineDuringFallback(t *testing.T) {
	snap := &ctxAwareFetcher{}
	http := &fixedFetcher{body: []byte("<html>should-not-reach-http</html>")}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	f := NewSnapshotFirstFetcher(snap, http)
	_, kind, err := f.Fetch(ctx, "https://a.example/x")

	if err == nil {
		t.Fatalf("expected non-nil err when ctx fires during snapshot, got nil")
	}
	if kind != scheduler.ErrorTimeout {
		t.Fatalf("kind = %q, want %q", kind, scheduler.ErrorTimeout)
	}
	if http.calls != 0 {
		t.Errorf("HTTP MUST NOT be called when ctx fires; got %d calls", http.calls)
	}
}

// ctxAwareFetcher blocks for ~50ms then returns a generic error; the test
// wraps it with a 20ms ctx so the ctx fires while the stage runs.
type ctxAwareFetcher struct{ calls int }

func (f *ctxAwareFetcher) Fetch(_ string) ([]byte, error) {
	f.calls++
	time.Sleep(50 * time.Millisecond)
	return nil, errors.New("snapshot internal error")
}
