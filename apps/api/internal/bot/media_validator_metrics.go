package bot

import (
	"sync"
	"sync/atomic"
)

// MediaValidationMetrics is a dependency-free counter set the consumer can
// emit while running. It is a process-local snapshot consumed by the existing
// snapshot/metrics observability seam (apps/api/internal/bot/snapshot/) and
// is structured so a future Prometheus exporter can read its public methods
// without locking on internals.
//
// Spec note: tasks.md §5 explicitly carves metrics out as "운영 가이드" — they
// are not part of the spec contract. This struct provides a minimal home so
// validator integrations don't reach for ad-hoc package-level globals.
type MediaValidationMetrics struct {
	mu       sync.RWMutex
	rejected map[string]uint64
	totalRej uint64
	// noPrimaryMediaCount counts how many PinDocuments classified as
	// no_primary_media after validation ran (tasks 5.2). Co-locating this
	// with rejection counts keeps the observability surface tight.
	noPrimaryMediaCount atomic.Uint64
	// pinnableCount counts pinnable documents seen after validation. Together
	// with noPrimaryMediaCount it produces the "no_primary_media 분류 비율"
	// trend metric.
	pinnableCount atomic.Uint64
}

// NewMediaValidationMetrics returns an empty metrics counter set.
func NewMediaValidationMetrics() *MediaValidationMetrics {
	return &MediaValidationMetrics{rejected: make(map[string]uint64)}
}

// RecordRejection increments the per-reason rejection counter and the total
// by 1. Safe for concurrent use.
func (m *MediaValidationMetrics) RecordRejection(reason MediaValidationReason) {
	m.RecordRejectionN(reason, 1)
}

// RecordRejectionN adds n to the per-reason rejection counter and the total
// in a single critical section. Used by the consumer to fold an entire
// MediaValidationRecord (which already aggregates per-reason counts) into
// the metric in O(1) per reason instead of O(n) calls. Safe for concurrent
// use; n<=0 is a no-op.
func (m *MediaValidationMetrics) RecordRejectionN(reason MediaValidationReason, n int) {
	if m == nil || n <= 0 {
		return
	}
	m.mu.Lock()
	if m.rejected == nil {
		m.rejected = make(map[string]uint64)
	}
	m.rejected[string(reason)] += uint64(n)
	m.totalRej += uint64(n)
	m.mu.Unlock()
}

// RecordClassification increments either pinnable or no_primary_media
// counters based on the classifier verdict. Other reasons are not tracked
// here; this is a focused signal for the validation change's tuning loop.
func (m *MediaValidationMetrics) RecordClassification(pinnable bool, reason ClassifierReason) {
	if m == nil {
		return
	}
	if pinnable {
		m.pinnableCount.Add(1)
		return
	}
	if reason == ClassifierReasonNoPrimaryMedia {
		m.noPrimaryMediaCount.Add(1)
	}
}

// Reset clears all counters back to zero. Intended for tests that share an
// instance across cases and for operational scenarios where a manual reset
// is desired (e.g. between metric scrape windows during incident response).
// Counters are otherwise process-lifetime-cumulative; a process restart
// also resets them implicitly.
func (m *MediaValidationMetrics) Reset() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.rejected = make(map[string]uint64)
	m.totalRej = 0
	m.mu.Unlock()
	m.pinnableCount.Store(0)
	m.noPrimaryMediaCount.Store(0)
}

// Snapshot returns a point-in-time copy of all counters. The map is a
// defensive copy and may be mutated freely by the caller.
func (m *MediaValidationMetrics) Snapshot() (totalRejections uint64, perReason map[string]uint64, pinnable uint64, noPrimaryMedia uint64) {
	if m == nil {
		return 0, nil, 0, 0
	}
	m.mu.RLock()
	perReason = make(map[string]uint64, len(m.rejected))
	for k, v := range m.rejected {
		perReason[k] = v
	}
	totalRejections = m.totalRej
	m.mu.RUnlock()
	return totalRejections, perReason, m.pinnableCount.Load(), m.noPrimaryMediaCount.Load()
}
