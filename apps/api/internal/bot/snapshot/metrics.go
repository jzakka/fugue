package snapshot

import (
	"sync/atomic"
	"time"
)

// Metrics is the observability surface used by the snapshot save path.
// The prod binary will bind this to a prometheus implementation
// (counter + histogram) via a small adapter; tests use the in-memory
// MetricsRecorder.
//
// Naming (per spec):
//   - IncUpload(result) → pioneer_snapshot_uploads_total{result="success|failure"}
//   - ObserveUploadDuration(d)  → pioneer_snapshot_upload_duration_seconds
type Metrics interface {
	IncUpload(result string)
	ObserveUploadDuration(d time.Duration)
}

const (
	ResultSuccess = "success"
	ResultFailure = "failure"
)

// MetricsRecorder is a tiny in-memory Metrics implementation. It is the
// default when no metrics backend is wired, and is directly useful in
// unit tests that want to assert upload counts.
type MetricsRecorder struct {
	success   atomic.Int64
	failure   atomic.Int64
	durations atomic.Int64 // sum in nanoseconds
	calls     atomic.Int64
}

func NewMetricsRecorder() *MetricsRecorder { return &MetricsRecorder{} }

func (m *MetricsRecorder) IncUpload(result string) {
	switch result {
	case ResultSuccess:
		m.success.Add(1)
	case ResultFailure:
		m.failure.Add(1)
	}
}

func (m *MetricsRecorder) ObserveUploadDuration(d time.Duration) {
	m.durations.Add(int64(d))
	m.calls.Add(1)
}

// Success returns the number of successful uploads recorded.
func (m *MetricsRecorder) Success() int64 { return m.success.Load() }

// Failure returns the number of failed uploads recorded.
func (m *MetricsRecorder) Failure() int64 { return m.failure.Load() }

// Calls returns the number of duration observations recorded.
func (m *MetricsRecorder) Calls() int64 { return m.calls.Load() }

// nopMetrics is used when the caller does not supply a Metrics backend.
type nopMetrics struct{}

func (nopMetrics) IncUpload(string)                    {}
func (nopMetrics) ObserveUploadDuration(time.Duration) {}
