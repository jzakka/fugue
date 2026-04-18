package snapshot

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// SnapshotStore stores Pioneer's raw fetch responses as gzip snapshots.
//
// Implementations MUST:
//   - Skip empty bodies (length 0). The spec requires "본문 길이 > 0".
//   - Compress with gzip; the on-wire object is a .html.gz blob whose CRC
//     trailer doubles as integrity check on the consumer side.
//   - Use SnapshotKey(normalizedURL, now) as the object key so Harvester
//     can reconstruct it deterministically.
//   - Honor object storage's atomic PUT semantics (last-write-wins) for
//     concurrent writes against the same key. No application-level lock,
//     versioning, or conditional headers.
type SnapshotStore interface {
	Put(ctx context.Context, normalizedURL string, body []byte) error
}

// ErrEmptyBody indicates the caller passed a zero-length body.
// Snapshot stores treat this as a no-op skip.
var ErrEmptyBody = errors.New("snapshot: empty body, nothing to store")

// S3PutObjectAPI is the minimal S3 client surface SnapshotStore needs.
// Lets tests inject a fake without depending on the full s3.Client.
type S3PutObjectAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Store implements SnapshotStore against an S3-compatible object store.
type S3Store struct {
	client S3PutObjectAPI
	bucket string
	now    func() time.Time
}

// NewS3Store builds an S3-backed SnapshotStore.
//
// bucket is the destination bucket; the snapshots/ prefix lives inside
// the key produced by SnapshotKey. The bucket lifecycle rule (TTL 365d)
// is configured at infra layer, not by this code.
func NewS3Store(client S3PutObjectAPI, bucket string) *S3Store {
	return &S3Store{client: client, bucket: bucket, now: time.Now}
}

// Put gzips body and uploads it under SnapshotKey(normalizedURL, now()).
// Returns ErrEmptyBody if body is empty (caller should treat as no-op).
// All other errors propagate so the caller can fail-open at the
// integration site.
func (s *S3Store) Put(ctx context.Context, normalizedURL string, body []byte) error {
	if len(body) == 0 {
		return ErrEmptyBody
	}

	compressed, err := gzipBytes(body)
	if err != nil {
		return fmt.Errorf("snapshot: gzip: %w", err)
	}

	key := SnapshotKey(normalizedURL, s.now())

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(s.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(compressed),
		ContentLength:   aws.Int64(int64(len(compressed))),
		ContentType:     aws.String("text/html"),
		ContentEncoding: aws.String("gzip"),
	})
	if err != nil {
		return fmt.Errorf("snapshot: s3 put %q: %w", key, err)
	}
	return nil
}

// gzipBytes compresses src with gzip default compression.
//
// The resulting blob is a complete gzip member: header + deflate-stream
// + CRC32 trailer. Harvester (or any consumer) can detect corruption via
// gunzip CRC validation; we deliberately do not add a separate checksum.
func gzipBytes(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(src); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
