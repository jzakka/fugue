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

// fakeS3 is an in-memory S3PutObjectAPI used to verify the store's
// upload behavior (key path, gzip round-trip, last-write-wins).
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	err     error
	calls   int
}

func newFakeS3() *fakeS3 { return &fakeS3{objects: map[string][]byte{}} }

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

func TestS3Store_Put_UsesExpectedKeyAndGzip(t *testing.T) {
	t.Parallel()
	fake := newFakeS3()
	store := NewS3Store(fake, "bucket")
	store.now = func() time.Time { return time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC) }

	payload := []byte("<html>hello</html>")
	if err := store.Put(context.Background(), "https://ex.com/", payload); err != nil {
		t.Fatalf("Put: %v", err)
	}

	wantKey := SnapshotKey("https://ex.com/", time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC))
	got, ok := fake.objects[wantKey]
	if !ok {
		t.Fatalf("expected key %q not found, have: %v", wantKey, keysOf(fake.objects))
	}
	decoded, err := gunzipForTest(got)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("roundtrip mismatch: %q vs %q", decoded, payload)
	}
}

func TestS3Store_Put_GzipCRCDetectsCorruption(t *testing.T) {
	t.Parallel()
	compressed, err := gzipCompress([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt the CRC-32 trailer (last 8 bytes contain CRC + size).
	compressed[len(compressed)-5] ^= 0xff
	if _, err := gunzipForTest(compressed); err == nil {
		t.Fatal("expected gunzip to detect corruption via CRC, got nil error")
	}
}

func TestS3Store_Put_PropagatesErrors(t *testing.T) {
	t.Parallel()
	fake := newFakeS3()
	fake.err = errors.New("s3 down")
	store := NewS3Store(fake, "bucket")
	if err := store.Put(context.Background(), "https://ex.com/", []byte("x")); err == nil {
		t.Fatal("expected error to propagate from fake S3")
	}
}

func TestS3Store_Put_LastWriteWins(t *testing.T) {
	t.Parallel()
	fake := newFakeS3()
	store := NewS3Store(fake, "bucket")
	store.now = func() time.Time { return time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC) }

	// Two sequential PUTs on the same URL + same UTC date.
	if err := store.Put(context.Background(), "https://ex.com/p", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "https://ex.com/p", []byte("second")); err != nil {
		t.Fatal(err)
	}

	key := SnapshotKey("https://ex.com/p", time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC))
	decoded, err := gunzipForTest(fake.objects[key])
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "second" {
		t.Fatalf("expected last-write-wins (second), got %q", decoded)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 PutObject calls, got %d", fake.calls)
	}
	if len(fake.objects) != 1 {
		t.Fatalf("expected single object key (overwrite), got %d keys", len(fake.objects))
	}
}

func TestS3Store_Put_ConcurrentSameKeyOverwrites(t *testing.T) {
	t.Parallel()
	fake := newFakeS3()
	store := NewS3Store(fake, "bucket")
	store.now = func() time.Time { return time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC) }

	var wg sync.WaitGroup
	const n = 8
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.Put(context.Background(), "https://ex.com/p", []byte("x"))
		}()
	}
	wg.Wait()

	// Regardless of interleaving, exactly one key should exist.
	if len(fake.objects) != 1 {
		t.Fatalf("expected 1 final key, got %d", len(fake.objects))
	}
	if fake.calls != n {
		t.Fatalf("expected %d PutObject calls (all accepted), got %d", n, fake.calls)
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
