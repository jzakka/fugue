package snapshot

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// fakeS3 is an in-memory S3PutObjectAPI used by tests.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	calls   int
	err     error
}

func newFakeS3() *fakeS3 {
	return &fakeS3{objects: make(map[string][]byte)}
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.objects[*in.Key] = body
	return &s3.PutObjectOutput{}, nil
}

func TestGzipRoundTripPreservesBytes(t *testing.T) {
	src := []byte("<html><body>hello fugue</body></html>")

	enc, err := gzipBytes(src)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if bytes.Equal(enc, src) {
		t.Fatalf("expected compressed bytes to differ from input")
	}

	dec, err := gunzipBytes(enc)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if !bytes.Equal(dec, src) {
		t.Fatalf("round trip mismatch:\n  got:  %q\n  want: %q", dec, src)
	}
}

func TestGzipCorruptionDetected(t *testing.T) {
	enc, err := gzipBytes([]byte("hello"))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	// Flip a byte in the middle to corrupt CRC.
	enc[len(enc)/2] ^= 0xFF

	if _, err := gunzipBytes(enc); err == nil {
		t.Fatalf("expected gunzip to detect corruption, got nil error")
	}
}

func TestS3StorePutUploadsGzippedToCorrectKey(t *testing.T) {
	fake := newFakeS3()
	store := NewS3Store(fake, "fugue-media")
	store.now = func() time.Time {
		return time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	}

	const url = "https://example.com/page"
	const body = "<html>raw</html>"
	if err := store.Put(context.Background(), url, []byte(body)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	wantKey := SnapshotKey(url, time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC))
	stored, ok := fake.objects[wantKey]
	if !ok {
		t.Fatalf("expected object at key %q, have keys: %v", wantKey, mapKeys(fake.objects))
	}

	dec, err := gunzipBytes(stored)
	if err != nil {
		t.Fatalf("uploaded body is not valid gzip: %v", err)
	}
	if string(dec) != body {
		t.Fatalf("decompressed body mismatch: %q vs %q", dec, body)
	}
}

func TestS3StorePutEmptyBodySkipsUpload(t *testing.T) {
	fake := newFakeS3()
	store := NewS3Store(fake, "fugue-media")

	err := store.Put(context.Background(), "https://example.com/x", nil)
	if !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("expected ErrEmptyBody, got %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("expected zero PutObject calls for empty body, got %d", fake.calls)
	}
}

func TestS3StorePutPropagatesUploadError(t *testing.T) {
	fake := newFakeS3()
	fake.err = errors.New("network down")
	store := NewS3Store(fake, "fugue-media")

	err := store.Put(context.Background(), "https://example.com/x", []byte("body"))
	if err == nil {
		t.Fatalf("expected error from failing PutObject")
	}
}

// TestS3StorePutConcurrentSameKeyLastWriteWins simulates the
// "동시 쓰기 idempotent 확인" integration scenario (tasks 4.6).
//
// Two goroutines upload distinct bodies for the same URL on the same UTC
// day. Both target the same key; the fake's map assignment under a single
// mutex models the object store's atomic-PUT contract. After both
// complete, exactly one body remains and it is one of the two inputs
// (last-write-wins semantics).
func TestS3StorePutConcurrentSameKeyLastWriteWins(t *testing.T) {
	fake := newFakeS3()
	store := NewS3Store(fake, "fugue-media")
	store.now = func() time.Time {
		return time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	}

	const url = "https://example.com/race"
	bodyA := []byte("body-A")
	bodyB := []byte("body-B")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = store.Put(context.Background(), url, bodyA)
	}()
	go func() {
		defer wg.Done()
		_ = store.Put(context.Background(), url, bodyB)
	}()
	wg.Wait()

	if fake.calls != 2 {
		t.Fatalf("expected 2 PutObject calls, got %d", fake.calls)
	}
	if len(fake.objects) != 1 {
		t.Fatalf("expected exactly 1 object key (overwrite), got %d: %v",
			len(fake.objects), mapKeys(fake.objects))
	}

	wantKey := SnapshotKey(url, time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC))
	stored, ok := fake.objects[wantKey]
	if !ok {
		t.Fatalf("missing key %q", wantKey)
	}
	dec, err := gunzipBytes(stored)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if !bytes.Equal(dec, bodyA) && !bytes.Equal(dec, bodyB) {
		t.Fatalf("final body must be one of the inputs, got %q", dec)
	}
}

func mapKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
