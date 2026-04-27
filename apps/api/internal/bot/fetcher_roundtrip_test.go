// fetcher_roundtrip_test.go covers task 4.10 of
// harvester-snapshot-first-fetch: Pioneer-write → Harvester-read
// round-trip integrity.
//
// The test puts a snapshot through the Pioneer write path (canonicalURL
// → snapshot.S3Store.Put) and then reads it back through the Harvester
// read path (ObjectStorageFetcher.Fetch → snapshot.S3Reader.Get). It
// asserts the body is byte-identical and that no HTTP fallback fires,
// which behaviorally guarantees both sides use the same URL
// normalization function (spec design.md Decision 5).

package bot

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
)

// sharedS3 implements both S3PutObjectAPI (writes) and S3GetObjectAPI
// (reads) against a single in-memory bucket so the write and read sides
// of the round-trip see the same storage.
type sharedS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newSharedS3() *sharedS3 {
	return &sharedS3{objects: map[string][]byte{}}
}

func (s *sharedS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[*in.Key] = body
	return &s3.PutObjectOutput{}, nil
}

func (s *sharedS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	s.mu.Lock()
	body, ok := s.objects[*in.Key]
	s.mu.Unlock()
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(body))}, nil
}

// Task 4.10: Pioneer-write → Harvester-read round-trip. Asserts the
// composite fetcher resolves the object from snapshot storage alone (no
// HTTP fallback) for URLs whose normalization rules differ between the
// two URL helpers — canonicalURL vs templatePath. A mismatched normalizer
// on the read side would cause the reader to look at a different key and
// force an HTTP fallback.
func TestRoundTrip_PioneerWriteHarvesterRead(t *testing.T) {
	// URL chosen so that canonicalURL and templatePath produce different
	// normalized strings: canonicalURL (urlcanon.Canonical) keeps the
	// numeric path segment "42", while templatePath rewrites it to "{id}"
	// and strips the query. A bug that reintroduces templatePath on the
	// read side would miss this object and trip the httpSpy below.
	const rawURL = "https://example.com/posts/42?utm_source=rss"
	raw := []byte("<html>round-trip body</html>")
	fixed := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)

	shared := newSharedS3()

	// Pioneer write path: canonicalURL → S3Store.Put (mirrors
	// pioneer_consumer.go:168).
	store := snapshot.NewS3Store(shared, "fugue-media")
	store.SetClock(func() time.Time { return fixed })
	canonical := canonicalURL(rawURL)
	if err := store.Put(context.Background(), canonical, raw); err != nil {
		t.Fatalf("Pioneer Put: %v", err)
	}

	// Harvester read path: ObjectStorageFetcher → S3Reader. CompositeFetcher
	// wraps an HTTP fetcher that MUST NOT be called on a snapshot hit.
	reader := snapshot.NewS3Reader(shared, "fugue-media")
	osf := NewObjectStorageFetcher(reader).WithClock(func() time.Time { return fixed })
	httpSpy := &countingFetcher{body: []byte("<html>should-not-be-seen</html>")}
	cf := NewCompositeFetcher(osf, httpSpy)

	got, err := cf.Fetch(rawURL)
	if err != nil {
		t.Fatalf("Harvester Fetch: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("round-trip body mismatch:\n  got:  %q\n  want: %q", got, raw)
	}
	if httpSpy.calls != 0 {
		t.Errorf("HTTP fallback called %d times; snapshot hit was expected (URL normalization drift between Pioneer write and Harvester read)", httpSpy.calls)
	}
}

// Task 4.7 (cross-day miss half): a snapshot written on day D must NOT be
// returned on a read performed on day D+1, because the Harvester key
// includes the read-time UTC date (design.md Decision 5a). A read-side
// clock advanced by 24h MUST miss the snapshot and fall through to the
// HTTP fetcher. The hit half of §4.7 is covered by
// TestRoundTrip_PioneerWriteHarvesterRead above (same-day write+read).
func TestRoundTrip_CrossUTCDayMissesAndFallsBackToHTTP(t *testing.T) {
	const rawURL = "https://example.com/posts/42?utm_source=rss"
	raw := []byte("<html>day-1 body</html>")
	httpBody := []byte("<html>http-fallback body</html>")

	writeClock := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	readClock := writeClock.Add(24 * time.Hour) // day D+1

	shared := newSharedS3()

	// Pioneer write on day D.
	store := snapshot.NewS3Store(shared, "fugue-media")
	store.SetClock(func() time.Time { return writeClock })
	if err := store.Put(context.Background(), canonicalURL(rawURL), raw); err != nil {
		t.Fatalf("Pioneer Put: %v", err)
	}

	// Harvester read on day D+1: the reader computes a different
	// SnapshotKey (different date segment) and must miss, falling back to
	// HTTP.
	reader := snapshot.NewS3Reader(shared, "fugue-media")
	osf := NewObjectStorageFetcher(reader).WithClock(func() time.Time { return readClock })
	httpSpy := &countingFetcher{body: httpBody}
	cf := NewCompositeFetcher(osf, httpSpy)

	got, err := cf.Fetch(rawURL)
	if err != nil {
		t.Fatalf("Harvester Fetch: %v", err)
	}
	if !bytes.Equal(got, httpBody) {
		t.Errorf("expected HTTP-fallback body, got %q", got)
	}
	if httpSpy.calls != 1 {
		t.Errorf("HTTP fallback called %d times; want exactly 1 (cross-UTC-day miss)", httpSpy.calls)
	}
}
