package snapshot

import (
	"context"
	"log"
	"time"
)

// Logger is a minimal structured-log sink. The standard library *log.Logger
// is not a natural fit for structured fields, so we model the emit points
// directly. The default implementation formats with log.Printf.
type Logger interface {
	// UploadFailed is called when an object-storage PUT fails. It is
	// invoked AFTER the Pioneer loop has already decided to continue, so
	// implementations must not panic.
	UploadFailed(ctx context.Context, normalizedURL, key string, err error)
}

type stdLogger struct{}

func (stdLogger) UploadFailed(_ context.Context, normalizedURL, key string, err error) {
	log.Printf("snapshot upload failed url=%q key=%q err=%v", normalizedURL, key, err)
}

// Saver encapsulates the Pioneer-side "save raw content after successful
// fetch" policy. It does three things:
//
//  1. Refuses to upload when the fetch was not a success — the caller
//     passes an explicit *FetchOutcome with the 2xx + non-empty-body check
//     already performed, and SaveRawContent only runs for Ok outcomes.
//  2. Times the upload and records success/failure metrics.
//  3. Swallows upload errors so Pioneer's crawl is never blocked by a
//     storage-side outage (fail-open).
//
// Concurrency: Saver is safe for concurrent use as long as the underlying
// Store, Metrics and Logger are. The default Store implementation (S3Store)
// is goroutine-safe via the AWS SDK client.
type Saver struct {
	store   Store
	metrics Metrics
	logger  Logger
	now     func() time.Time
}

// NewSaver wraps a Store with the Pioneer save policy. Passing a NoopStore
// produces a Saver whose SaveRawContent is a no-op — this is how the
// feature flag PIONEER_SNAPSHOT_ENABLED=false is wired at startup.
func NewSaver(store Store, metrics Metrics, logger Logger) *Saver {
	if metrics == nil {
		metrics = nopMetrics{}
	}
	if logger == nil {
		logger = stdLogger{}
	}
	return &Saver{
		store:   store,
		metrics: metrics,
		logger:  logger,
		now:     time.Now,
	}
}

// SaveRawContent uploads body for normalizedURL. It is a no-op when body is
// empty — callers should also skip calling this method for non-2xx fetch
// outcomes, but the empty-body check here is a belt-and-suspenders guard
// that enforces the spec's "fetch success = 2xx + body > 0" contract.
//
// The returned error is informational; Pioneer must continue crawling
// regardless (fail-open).
func (s *Saver) SaveRawContent(ctx context.Context, normalizedURL string, body []byte) error {
	if len(body) == 0 {
		// Defense in depth: caller should already have filtered this.
		return nil
	}

	start := s.now()
	err := s.store.Put(ctx, normalizedURL, body)
	s.metrics.ObserveUploadDuration(s.now().Sub(start))

	if err != nil {
		s.metrics.IncUpload(ResultFailure)
		key := SnapshotKey(normalizedURL, start)
		s.logger.UploadFailed(ctx, normalizedURL, key, err)
		return err
	}
	s.metrics.IncUpload(ResultSuccess)
	return nil
}
