package snapshot

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Store persists Pioneer's raw HTML responses to object storage. Pioneer
// calls Put(ctx, normalizedURL, body) after a successful fetch; the store
// derives the object key via SnapshotKey, gzip-compresses body, and uploads
// it. Errors returned here are treated as fail-open by Pioneer (log +
// metric, do not block the crawl).
type Store interface {
	Put(ctx context.Context, normalizedURL string, body []byte) error
}

// S3PutObjectAPI is the subset of the AWS S3 client API that the store
// uses. Declaring it as an interface keeps the store unit-testable with a
// small in-memory fake and lets us reuse the existing storage.Client S3
// handle without depending on its full surface.
type S3PutObjectAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Store uploads gzip-compressed snapshots to an S3-compatible bucket.
// The bucket lifecycle (TTL 365 days) is configured in infra (terraform/
// helm) on the `snapshots/` prefix; the application code does not set
// per-object expiration.
type S3Store struct {
	s3     S3PutObjectAPI
	bucket string
	now    func() time.Time // injectable for tests
}

// NewS3Store constructs an S3Store bound to the given S3 client and
// bucket. The provided s3 client is expected to already have the
// appropriate credentials and endpoint resolver.
func NewS3Store(s3 S3PutObjectAPI, bucket string) *S3Store {
	return &S3Store{
		s3:     s3,
		bucket: bucket,
		now:    time.Now,
	}
}

// Put gzip-compresses body and uploads it to
// snapshots/<sha256(normalizedURL)>/<yyyymmdd>.html.gz. Concurrent PUTs to
// the same key follow S3's last-write-wins semantics; this method does not
// use conditional writes or locks.
func (s *S3Store) Put(ctx context.Context, normalizedURL string, body []byte) error {
	key := SnapshotKey(normalizedURL, s.now())

	compressed, err := gzipCompress(body)
	if err != nil {
		return fmt.Errorf("snapshot: gzip: %w", err)
	}

	_, err = s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(s.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(compressed),
		ContentType:     aws.String("text/html"),
		ContentEncoding: aws.String("gzip"),
		ContentLength:   aws.Int64(int64(len(compressed))),
	})
	if err != nil {
		return fmt.Errorf("snapshot: s3 put: %w", err)
	}
	return nil
}

// gzipCompress writes body through a gzip.Writer. The resulting bytes are
// a complete gzip stream (magic header + deflate payload + CRC-32 trailer),
// so Harvester's gunzip naturally detects corruption via the gzip CRC.
func gzipCompress(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(body); err != nil {
		_ = gw.Close()
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// gunzipForTest decompresses a gzip stream. Exposed for tests that verify
// round-trip integrity of snapshots.
func gunzipForTest(compressed []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gr.Close() }()
	return io.ReadAll(gr)
}
