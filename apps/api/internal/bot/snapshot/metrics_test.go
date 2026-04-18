package snapshot

import (
	"sync"
	"testing"
	"time"
)

func TestMetricsCountersAreThreadSafe(t *testing.T) {
	m := NewMetrics(0)

	const N = 1000
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			m.IncSuccess()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			m.IncFailure()
		}
	}()
	wg.Wait()

	s, f, _ := m.Snapshot()
	if s != N || f != N {
		t.Fatalf("counters wrong: success=%d failure=%d (want %d each)", s, f, N)
	}
}

func TestMetricsObserveDurationRingBounded(t *testing.T) {
	m := NewMetrics(4)
	for i := 0; i < 10; i++ {
		m.ObserveDuration(time.Duration(i) * time.Millisecond)
	}
	_, _, ds := m.Snapshot()
	if len(ds) != 4 {
		t.Fatalf("expected ring size 4, got %d (%v)", len(ds), ds)
	}
}

func TestMetricsNilReceiverIsSafe(t *testing.T) {
	var m *Metrics
	m.IncSuccess()
	m.IncFailure()
	m.ObserveDuration(time.Second)
	s, f, _ := m.Snapshot()
	if s != 0 || f != 0 {
		t.Fatalf("nil-receiver Snapshot should be zero, got %d/%d", s, f)
	}
}
