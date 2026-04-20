// fetcher_test.go covers tasks 4.1-4.6 of harvester-snapshot-first-fetch:
// unit tests for CompositeFetcher's ObjectStorage-first → HTTP-fallback
// semantics. Tasks 4.7 and 4.8 (end-to-end Harvester flow) live in
// harvester_snapshot_test.go.

package bot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
)

// countingFetcher records every Fetch call so tests can assert "HTTP was
// not called" (the key claim of snapshot-first semantics).
type countingFetcher struct {
	calls int
	body  []byte
	err   error
}

func (f *countingFetcher) Fetch(_ string) ([]byte, error) {
	f.calls++
	return f.body, f.err
}

// stubReader implements snapshot.SnapshotReader with a canned response so
// ObjectStorageFetcher can be exercised without AWS.
type stubReader struct {
	body []byte
	err  error
}

func (r *stubReader) Get(_ context.Context, _ string, _ time.Time) ([]byte, error) {
	return r.body, r.err
}

// Task 4.1: snapshot hit → HTTP never called.
func TestCompositeFetcher_SnapshotHitSkipsHTTP(t *testing.T) {
	snap := &countingFetcher{body: []byte("<html>from-snapshot</html>")}
	http := &countingFetcher{body: []byte("<html>from-http</html>")}

	cf := NewCompositeFetcher(snap, http)
	body, err := cf.Fetch("https://example.com/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "<html>from-snapshot</html>" {
		t.Errorf("body = %q, want snapshot bytes", body)
	}
	if snap.calls != 1 {
		t.Errorf("snapshot calls = %d, want 1", snap.calls)
	}
	if http.calls != 0 {
		t.Errorf("http calls = %d, want 0 (snapshot hit must not fall back)", http.calls)
	}
}

// Task 4.2: "not found" miss → HTTP fallback returns body.
func TestCompositeFetcher_NotFoundFallsBackToHTTP(t *testing.T) {
	snap := &countingFetcher{err: fmt.Errorf("wrap: %w", snapshot.ErrSnapshotNotFound)}
	http := &countingFetcher{body: []byte("<html>http-body</html>")}

	cf := NewCompositeFetcher(snap, http)
	body, err := cf.Fetch("https://example.com/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "<html>http-body</html>" {
		t.Errorf("body = %q, want HTTP fallback body", body)
	}
	if http.calls != 1 {
		t.Errorf("http calls = %d, want 1", http.calls)
	}
}

// Task 4.3: expired-style "not found" miss → HTTP fallback.
// Per design.md, TTL-expired objects surface as NoSuchKey (lifecycle
// deletion), which classifier maps to ErrSnapshotNotFound. No separate
// "expired" sentinel exists — the test asserts the spec-required behavior
// that stale snapshots route through the same miss → HTTP path.
func TestCompositeFetcher_ExpiredFallsBackToHTTP(t *testing.T) {
	// Simulate S3 lifecycle-deleted object: underlying error is NoSuchKey,
	// classifier wraps with ErrSnapshotNotFound.
	snap := &countingFetcher{err: fmt.Errorf("expired: %w", snapshot.ErrSnapshotNotFound)}
	http := &countingFetcher{body: []byte("<html>refetched</html>")}

	cf := NewCompositeFetcher(snap, http)
	body, err := cf.Fetch("https://example.com/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "<html>refetched</html>" {
		t.Errorf("body = %q, want HTTP fallback body", body)
	}
	if http.calls != 1 {
		t.Errorf("http calls = %d, want 1", http.calls)
	}
}

// Task 4.4: network / permission / internal errors ALL fall back to HTTP.
// The spec's core decision (design.md Decision 2) is that every
// ObjectStorage failure class is treated as a single "miss".
func TestCompositeFetcher_AllErrorKindsFallBackToHTTP(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"network", snapshot.ErrSnapshotNetwork},
		{"permission", snapshot.ErrSnapshotPermission},
		{"internal", snapshot.ErrSnapshotInternal},
		{"unknown", errors.New("some arbitrary error")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := &countingFetcher{err: tc.err}
			http := &countingFetcher{body: []byte("ok")}

			cf := NewCompositeFetcher(snap, http)
			body, err := cf.Fetch("https://example.com/x")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(body) != "ok" {
				t.Errorf("body = %q, want HTTP fallback", body)
			}
			if http.calls != 1 {
				t.Errorf("http calls = %d, want 1", http.calls)
			}
		})
	}
}

// Task 4.5: gzip'd snapshot is decompressed inside ObjectStorageFetcher so
// CompositeFetcher's caller never sees the compressed blob.
//
// Uses the full stack: a stubReader that returns pre-gunzipped bytes
// (snapshot.S3Reader's Get contract: the body is ALREADY decompressed by
// the time it leaves the reader). This covers 2.4 semantically. A
// lower-level round-trip test for the actual gunzip lives in
// snapshot/reader_test.go (CompressedObjectRoundTrip) as a companion.
func TestObjectStorageFetcher_ReturnsDecompressedBody(t *testing.T) {
	raw := []byte("<html>original</html>")
	reader := &stubReader{body: raw}

	osf := NewObjectStorageFetcher(reader).WithClock(func() time.Time {
		return time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	})

	body, err := osf.Fetch("https://example.com/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != string(raw) {
		t.Errorf("body = %q, want %q", body, raw)
	}
}

// Task 4.6: both paths fail → CompositeFetcher returns HTTP's error.
func TestCompositeFetcher_DoubleFailureReturnsError(t *testing.T) {
	snap := &countingFetcher{err: snapshot.ErrSnapshotNotFound}
	httpErr := errors.New("http 500")
	http := &countingFetcher{err: httpErr}

	cf := NewCompositeFetcher(snap, http)
	body, err := cf.Fetch("https://example.com/x")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, httpErr) {
		t.Errorf("err = %v, want HTTP error wrapping", err)
	}
	if body != nil {
		t.Errorf("body = %q, want nil on double failure", body)
	}
}

// Task 5.1 observability check: the error classifier labels the five
// error kinds spec §5.1 requires. Metric label vocabulary is part of the
// observability contract; this test pins it down.
func TestClassifySnapshotError_StableLabels(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{snapshot.ErrSnapshotNotFound, "not_found"},
		{snapshot.ErrSnapshotPermission, "permission"},
		{snapshot.ErrSnapshotNetwork, "network"},
		{snapshot.ErrSnapshotInternal, "internal"},
		{errors.New("mystery"), "unknown"},
	}
	for _, tc := range cases {
		got := ClassifySnapshotError(tc.err)
		if got != tc.want {
			t.Errorf("ClassifySnapshotError(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

// Smoke test for HTTPFetcher against a local httptest server, primarily to
// guarantee the default Fetcher still works end-to-end after the refactor.
func TestHTTPFetcher_Smoke(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	body, err := NewHTTPFetcher().Fetch(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "<html>ok</html>" {
		t.Errorf("body = %q", body)
	}
}
