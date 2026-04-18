package scheduler

import (
	"testing"
	"time"
)

// 6.1: computeBackoff(1..5) == 30s, 60s, 120s, 240s, 480s.
func TestComputeBackoff_TableOfFive(t *testing.T) {
	cases := []struct {
		n    int
		want time.Duration
	}{
		{1, 30 * time.Second},
		{2, 60 * time.Second},
		{3, 120 * time.Second},
		{4, 240 * time.Second},
		{5, 480 * time.Second},
	}
	for _, c := range cases {
		got := computeBackoff(c.n)
		if got != c.want {
			t.Errorf("computeBackoff(%d) = %s, want %s", c.n, got, c.want)
		}
	}
}

// 2.4 defensive clamp: out-of-range inputs must not panic and must map to the
// nearest in-range value.
func TestComputeBackoff_ClampLow(t *testing.T) {
	got := computeBackoff(0)
	if got != 30*time.Second {
		t.Errorf("computeBackoff(0) clamped = %s, want 30s", got)
	}
	got = computeBackoff(-3)
	if got != 30*time.Second {
		t.Errorf("computeBackoff(-3) clamped = %s, want 30s", got)
	}
}

func TestComputeBackoff_ClampHigh(t *testing.T) {
	got := computeBackoff(6)
	if got != 480*time.Second {
		t.Errorf("computeBackoff(6) clamped = %s, want 480s", got)
	}
	got = computeBackoff(100)
	if got != 480*time.Second {
		t.Errorf("computeBackoff(100) clamped = %s, want 480s", got)
	}
}

// 6.2: default jitterer over 1000 samples on a fixed delay must (a) stay
// inside [-10%, +10%] and (b) produce variance (not constant). This is the
// deterministic boundary check required by the spec; the ±10% envelope
// distinguishes uniform from normal distributions (normal has unbounded tails
// that would violate (a)).
func TestDefaultJitterer_WithinBoundsAndVaries(t *testing.T) {
	j := defaultJitterer()
	const delay = 100 * time.Second
	const lo = -int64(delay) / 10
	const hi = int64(delay) / 10

	var samples [1000]time.Duration
	for i := range samples {
		samples[i] = j(delay)
		if int64(samples[i]) < lo || int64(samples[i]) > hi {
			t.Fatalf("sample %d = %s out of [-10%%, +10%%] = [%s, %s]",
				i, samples[i], time.Duration(lo), time.Duration(hi))
		}
	}
	// Variance sanity: at least two distinct values.
	first := samples[0]
	distinct := false
	for _, s := range samples[1:] {
		if s != first {
			distinct = true
			break
		}
	}
	if !distinct {
		t.Fatalf("jitterer produced constant output over 1000 samples: %s", first)
	}
}

// Task 2.3's T_report + delay + jitter composition is exercised end-to-end by
// the url_scheduler integration tests (gap = next - callStart assertions in
// TestIntegration_RecordFetchError_FiveConsecutiveBackoffs /
// _NetworkAndTimeoutFormula / _RecordHarvestError_4xxAndBackoff). No standalone
// helper is kept here because the production path in recordError composes the
// three components inline per error-count candidate and exposing a separate
// helper would duplicate that logic.
