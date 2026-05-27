package snapshot

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// SnapshotReader reads previously stored snapshots back out.
//
// Implementations MUST:
//   - Compute the object key via SnapshotKey(normalizedURL, t) so Pioneer's
//     Put and Harvester's Get agree bit-for-bit on the same input.
//   - Return the gunzipped body. Callers receive the original HTML bytes,
//     not the on-wire .gz blob.
//   - Return one of the ErrSnapshot* sentinels for recognizable failure
//     modes so callers can classify (not_found / network / permission /
//     internal) for observability. ErrEmptyBody is also possible if the
//     upstream stored object is empty.
type SnapshotReader interface {
	Get(ctx context.Context, normalizedURL string, t time.Time) ([]byte, error)
}

// S3GetObjectAPI is the minimal S3 client surface a reader needs.
// Lets tests inject a fake without depending on the full s3.Client.
type S3GetObjectAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// Sentinel errors emitted by S3Reader so callers can distinguish the five
// failure categories required by pioneer-snapshot-storage observability
// (not_found / expired / network / permission / internal).
//
// Note: the fetcher contract of harvester-snapshot-first-fetch collapses
// all of these into a single "miss" for the fallback decision; these
// sentinels exist purely for logging/metrics.
var (
	ErrSnapshotNotFound   = errors.New("snapshot: not found")
	ErrSnapshotPermission = errors.New("snapshot: permission denied")
	ErrSnapshotNetwork    = errors.New("snapshot: network error")
	ErrSnapshotInternal   = errors.New("snapshot: internal error")
)

// S3Reader implements SnapshotReader against an S3-compatible object store.
type S3Reader struct {
	client S3GetObjectAPI
	bucket string
}

// NewS3Reader builds an S3-backed SnapshotReader.
//
// bucket must match the one used by NewS3Store on the write side so
// Pioneer's Put and Harvester's Get hit the same object.
func NewS3Reader(client S3GetObjectAPI, bucket string) *S3Reader {
	return &S3Reader{client: client, bucket: bucket}
}

// Get fetches the gzip-compressed snapshot under SnapshotKey(normalizedURL, t)
// and returns the uncompressed HTML bytes.
//
// On success: returns decompressed body, nil.
// On failure: returns nil and one of ErrSnapshot* (classified from the S3
// error) wrapping the underlying cause. The caller should treat every
// error case identically for fallback purposes, using the sentinel only
// for logging/metrics.
func (r *S3Reader) Get(ctx context.Context, normalizedURL string, t time.Time) ([]byte, error) {
	key := SnapshotKey(normalizedURL, t)

	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, classifyGetError(err, key)
	}
	defer func() { _ = out.Body.Close() }()

	const maxCompressedSize = 10 * 1024 * 1024 // 10MB cap on the gz blob
	compressed, err := io.ReadAll(io.LimitReader(out.Body, maxCompressedSize))
	if err != nil {
		return nil, fmt.Errorf("%w: read body %q: %v", ErrSnapshotNetwork, key, err)
	}
	if len(compressed) == 0 {
		return nil, fmt.Errorf("%w: empty object %q", ErrSnapshotInternal, key)
	}

	body, err := Gunzip(compressed)
	if err != nil {
		return nil, fmt.Errorf("%w: gunzip %q: %v", ErrSnapshotInternal, key, err)
	}
	return body, nil
}

// MaxDecompressedSnapshotBytes caps the gunzipped snapshot body to bound
// the gzip-bomb blast radius on the Harvester worker.
//
// Sister convention: every other external-body read in this codebase wraps
// `io.ReadAll` with `io.LimitReader` — og/service.go:44 (1MB HTML head),
// bot/robots_filter.go:51 (512KB), bot/snapshot/reader.go:87 (10MB on the
// compressed gz blob), bot/helpers.go, bot/pioneer_consumer.go,
// bot/media_validator.go, auth/provider.go:35 (64KB userinfo, cycle 108).
// The compressed-input cap above is meaningless against a gzip bomb: a 1KB
// gz with a 1000:1 ratio still fits under the 10MB input cap and explodes
// to ~1GB on Gunzip. OWASP zip-bomb is a well-known category; we close it
// by capping the decompressed output too.
//
// 100MB rationale: snapshot bodies are the full HTML page (not just <head>
// like og), so the og cap (1MB) would be too tight. Pioneer-side typical
// snapshots are ~100KB-1MB compressed with normal HTML compression ratio
// 5:1-10:1 → decompressed ~500KB-10MB. 100MB gives ~10x safety margin over
// the largest realistic page and matches the input cap's 10:1 ratio
// (10MB input × 10:1 normal compression = 100MB normal output). A 1000:1
// bomb at the 10MB input cap would try to write 10GB and is now rejected
// instead of OOM-killing the Harvester worker — CompositeFetcher's
// fail-open path (collapses any snapshot error into "miss" and falls back
// to HTTP) handles the rejection gracefully without service degradation.
const MaxDecompressedSnapshotBytes = 100 * 1024 * 1024

// Gunzip decompresses a gzip member produced by gzipBytes (the storage
// side's compression). Exposed for production Harvester use; the test
// helper in testhelpers_test.go pre-dates this and remains unchanged so
// its round-trip assertions stay self-contained.
//
// Output is capped at MaxDecompressedSnapshotBytes. If the decompressed
// stream exceeds the cap, returns an error (rather than silent truncation)
// so the caller's existing fail-open path (CompositeFetcher) retries via
// HTTP instead of feeding a truncated, syntactically invalid HTML body to
// downstream parsers.
func Gunzip(src []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	body, err := io.ReadAll(io.LimitReader(zr, MaxDecompressedSnapshotBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > MaxDecompressedSnapshotBytes {
		return nil, fmt.Errorf("decompressed body exceeded %d byte cap (suspected gzip bomb)", MaxDecompressedSnapshotBytes)
	}
	return body, nil
}

// classifyGetError maps AWS SDK errors into the ErrSnapshot* sentinels.
// Order matters: we check the most specific types first.
func classifyGetError(err error, key string) error {
	// Missing object: NoSuchKey or NotFound.
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return fmt.Errorf("%w: key %q: %v", ErrSnapshotNotFound, key, err)
	}
	// AWS SDK v2 may also surface a bare types.NotFound for HEAD-style misses.
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return fmt.Errorf("%w: key %q: %v", ErrSnapshotNotFound, key, err)
	}
	// String fallback for providers (MinIO, etc.) that don't map to the
	// typed error. "NoSuchKey" / "NotFound" appear in the error message.
	msg := err.Error()
	if strings.Contains(msg, "NoSuchKey") || strings.Contains(msg, "status code: 404") || strings.Contains(msg, "NotFound") {
		return fmt.Errorf("%w: key %q: %v", ErrSnapshotNotFound, key, err)
	}
	if strings.Contains(msg, "AccessDenied") || strings.Contains(msg, "status code: 403") || strings.Contains(msg, "Forbidden") {
		return fmt.Errorf("%w: key %q: %v", ErrSnapshotPermission, key, err)
	}
	// 5xx or body-read issues → internal.
	if strings.Contains(msg, "status code: 5") || strings.Contains(msg, "InternalError") {
		return fmt.Errorf("%w: key %q: %v", ErrSnapshotInternal, key, err)
	}
	// Network/timeout default: covers connection refused, DNS, ctx timeout.
	return fmt.Errorf("%w: key %q: %v", ErrSnapshotNetwork, key, err)
}
