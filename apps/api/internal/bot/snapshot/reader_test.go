package snapshot

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeGetS3 is an in-memory S3GetObjectAPI for reader tests.
type fakeGetS3 struct {
	objects map[string][]byte
	err     error
}

func (f *fakeGetS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	body, ok := f.objects[*in.Key]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(body)),
	}, nil
}

// TestS3ReaderRoundTripThroughStore exercises the full write→read cycle:
// Pioneer's S3Store gzips and uploads, Harvester's S3Reader downloads and
// gunzips. The shared SnapshotKey guarantees both hit the same object.
// This is the single authoritative test for the harvester-snapshot-first-fetch
// spec §1 guarantee that "snapshot hit → decompressed bytes identical to
// what Pioneer wrote".
func TestS3ReaderRoundTripThroughStore(t *testing.T) {
	fake := &fakeGetS3{objects: map[string][]byte{}}
	// Write path: use the real S3Store to produce a gzip'd object.
	store := NewS3Store(putAdapter{fake}, "fugue-media")
	store.now = func() time.Time { return time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC) }

	const url = "https://example.com/page"
	raw := []byte("<html>original body</html>")
	if err := store.Put(context.Background(), url, raw); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Read path: new S3Reader against the same fake, same key.
	reader := NewS3Reader(fake, "fugue-media")
	got, err := reader.Get(context.Background(), url, time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("round-trip bytes mismatch: got %q, want %q", got, raw)
	}
}

// TestS3ReaderTypedNoSuchKey asserts the typed NoSuchKey mapping.
func TestS3ReaderTypedNoSuchKey(t *testing.T) {
	fake := &fakeGetS3{objects: map[string][]byte{}} // empty
	reader := NewS3Reader(fake, "fugue-media")

	_, err := reader.Get(context.Background(), "https://nope.test", time.Now())
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Errorf("err = %v, want ErrSnapshotNotFound", err)
	}
}

// TestS3ReaderStringErrorClassification pins the MinIO-style string
// fallback used when providers return plain error strings instead of typed
// errors.
func TestS3ReaderStringErrorClassification(t *testing.T) {
	cases := []struct {
		rawErr string
		want   error
	}{
		{"something NoSuchKey happened", ErrSnapshotNotFound},
		{"HTTP status code: 404", ErrSnapshotNotFound},
		{"NotFound: absent", ErrSnapshotNotFound},
		{"AccessDenied on bucket", ErrSnapshotPermission},
		{"HTTP status code: 403 Forbidden", ErrSnapshotPermission},
		{"HTTP status code: 503 bad gateway", ErrSnapshotInternal},
		{"connection refused", ErrSnapshotNetwork},
	}
	for _, tc := range cases {
		t.Run(tc.rawErr, func(t *testing.T) {
			fake := &fakeGetS3{err: errors.New(tc.rawErr)}
			reader := NewS3Reader(fake, "fugue-media")

			_, err := reader.Get(context.Background(), "https://x.test", time.Now())
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestS3ReaderRejectsCorruptedGzip covers CRC-based integrity detection.
func TestS3ReaderRejectsCorruptedGzip(t *testing.T) {
	// Compress a body, then flip a middle byte.
	compressed, err := gzipBytes([]byte("hello world"))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	compressed[len(compressed)/2] ^= 0xFF

	key := SnapshotKey("https://corrupt.test", time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC))
	fake := &fakeGetS3{objects: map[string][]byte{key: compressed}}
	reader := NewS3Reader(fake, "fugue-media")

	_, err = reader.Get(context.Background(), "https://corrupt.test", time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected gunzip to detect CRC corruption")
	}
	if !errors.Is(err, ErrSnapshotInternal) {
		t.Errorf("err = %v, want ErrSnapshotInternal (corruption classified as internal)", err)
	}
	// And the message should not leak implementation details beyond the
	// sentinel + key.
	if !strings.Contains(err.Error(), "gunzip") {
		t.Errorf("err = %q, want mention of gunzip context", err)
	}
}

// putAdapter exposes the fakeGetS3 as an S3PutObjectAPI so the write test
// above can use the real S3Store without a second fake.
type putAdapter struct{ f *fakeGetS3 }

func (a putAdapter) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	if a.f.objects == nil {
		a.f.objects = map[string][]byte{}
	}
	a.f.objects[*in.Key] = body
	return &s3.PutObjectOutput{}, nil
}
