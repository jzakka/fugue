package snapshot

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics is a small thread-safe counter+histogram surface for Pioneer
// snapshot uploads. Names mirror the proposed Prometheus series
// (`pioneer_snapshot_uploads_total{result=...}` and
// `pioneer_snapshot_upload_duration_seconds`) but the implementation is
// intentionally dependency-free; a future change can wire these counters
// into the project's chosen metrics backend.
type Metrics struct {
	successCount uint64
	failureCount uint64

	mu         sync.Mutex
	durations  []time.Duration // bounded ring; observation window
	ringSize   int
	ringCursor int
}

// NewMetrics returns a Metrics instance with the given ring size for
// duration observations (use 0 for default).
func NewMetrics(ringSize int) *Metrics {
	if ringSize <= 0 {
		ringSize = 1024
	}
	return &Metrics{
		ringSize:  ringSize,
		durations: make([]time.Duration, 0, ringSize),
	}
}

// IncSuccess increments pioneer_snapshot_uploads_total{result="success"}.
func (m *Metrics) IncSuccess() {
	if m == nil {
		return
	}
	atomic.AddUint64(&m.successCount, 1)
}

// IncFailure increments pioneer_snapshot_uploads_total{result="failure"}.
func (m *Metrics) IncFailure() {
	if m == nil {
		return
	}
	atomic.AddUint64(&m.failureCount, 1)
}

// ObserveDuration records pioneer_snapshot_upload_duration_seconds.
func (m *Metrics) ObserveDuration(d time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.durations) < m.ringSize {
		m.durations = append(m.durations, d)
		return
	}
	m.durations[m.ringCursor] = d
	m.ringCursor = (m.ringCursor + 1) % m.ringSize
}

// Snapshot returns a point-in-time view of the counters and observed
// durations. Used by tests and any future scrape adapter.
func (m *Metrics) Snapshot() (success, failure uint64, durations []time.Duration) {
	if m == nil {
		return 0, 0, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]time.Duration, len(m.durations))
	copy(cp, m.durations)
	return atomic.LoadUint64(&m.successCount),
		atomic.LoadUint64(&m.failureCount),
		cp
}
