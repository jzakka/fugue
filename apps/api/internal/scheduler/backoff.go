package scheduler

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

// Backoff policy constants defined by OpenSpec change "scheduler-retry-backoff":
//
//	delay      = base * 2^(errorCountAfter - 1)   where base = 30s
//	jitter     = uniform[-0.1 * delay, +0.1 * delay]
//	next_*_at  = T_report + delay + jitter
//
// Maximum errorCountAfter is 5 (enforced by clamp in computeBackoff), so the
// largest delay is 30s * 2^4 = 480s and int64 nanosecond arithmetic cannot
// overflow within the documented regime.
const (
	backoffBase       = 30 * time.Second
	backoffMaxErrorN  = 5
	backoffJitterFrac = 0.10
)

// Clock abstracts time.Now so tests can inject deterministic time without
// taking the parameter through public scheduler APIs. Production code uses
// realClock.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// RealClock returns a Clock backed by time.Now. Exposed so other packages in
// the same module can construct a default URLScheduler without building their
// own wrapper.
func RealClock() Clock { return realClock{} }

// Jitterer returns a random offset inside [-frac*delay, +frac*delay] for a
// given delay. The default implementation uses a PRNG captured in its closure;
// tests may substitute a deterministic function via the scheduler constructor.
type Jitterer func(delay time.Duration) time.Duration

// defaultJitterer seeds a non-cryptographic PRNG once per process and returns
// a Jitterer that samples uniform[-0.1, +0.1]. math/rand is sufficient: jitter
// is a herd-reduction mechanism, not a security primitive.
//
// The *rand.Rand is intentionally kept out of the function signature so that
// callers never depend on a specific PRNG type. Concurrent access is guarded
// by a mutex because *rand.Rand is not safe for concurrent use.
func defaultJitterer() Jitterer {
	src := rand.New(rand.NewSource(time.Now().UnixNano()))
	var mu sync.Mutex
	return func(delay time.Duration) time.Duration {
		mu.Lock()
		defer mu.Unlock()
		// Uniform in [-1, +1), scaled by backoffJitterFrac.
		// rand.Float64() returns [0, 1); 2x - 1 yields [-1, 1).
		frac := (src.Float64()*2 - 1) * backoffJitterFrac
		return time.Duration(float64(delay) * frac)
	}
}

// computeBackoff returns the pre-jitter delay for a given post-increment error
// count. Callers must pass 1..5; values outside this range are clamped and a
// warning log is emitted because clamping indicates a caller contract
// violation (the spec guarantees 1..5 at the API boundary). The clamp exists
// solely to prevent a negative-shift panic from `1 << (n-1)`.
func computeBackoff(errorCountAfter int) time.Duration {
	n := errorCountAfter
	if n < 1 {
		log.Printf("WARN scheduler.backoff: computeBackoff called with errorCountAfter=%d; clamped to 1 (caller bug)", n)
		n = 1
	}
	if n > backoffMaxErrorN {
		log.Printf("WARN scheduler.backoff: computeBackoff called with errorCountAfter=%d; clamped to %d (caller bug)", n, backoffMaxErrorN)
		n = backoffMaxErrorN
	}
	// Explicit int -> time.Duration cast required by Go for the shift result.
	return backoffBase * time.Duration(1<<(n-1))
}
