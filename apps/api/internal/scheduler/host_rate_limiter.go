// Package scheduler provides URL scheduling primitives for crawler workers.
//
// HostRateLimiter implements per-host token bucket politeness for the crawler
// dequeue path. The behavior contract (Allow / SetHostRate, default fallbacks,
// validity substitution, disable flag) is defined by the OpenSpec change
// "scheduler-host-token-bucket". The claim-time call pattern (which row to
// pass to Allow, what to do when all candidates are blocked) is defined by
// "scheduler-claim-api" and is intentionally NOT implemented here.
package scheduler

import (
	"log"
	"sync"

	"golang.org/x/time/rate"
)

// Factory defaults applied when the operator has not configured otherwise.
const (
	FactoryDefaultRatePerSec = 1.0
	FactoryDefaultBurst      = 5
)

// HostRateLimiter holds a per-host rate.Limiter map guarded by an RWMutex.
// Process-local; no cross-process coordination.
//
// Bootstrapping note: Config (apps/api/internal/config/config.go) loads
// SCHEDULER_HOST_DEFAULT_RATE_PER_SEC / SCHEDULER_HOST_DEFAULT_BURST /
// SCHEDULER_HOST_TOKEN_BUCKET_ENABLED. The actual instantiation of
// HostRateLimiter and its injection into the dequeue path happens in the
// scheduler-claim-api change (which defines the URLScheduler interface and
// claim transaction). This change deliberately ships only the contract
// implementation so that claim-api can wire it in next.
type HostRateLimiter struct {
	mu           sync.RWMutex
	limiters     map[string]*rate.Limiter
	defaultRate  float64
	defaultBurst int
	enabled      bool
}

// NewHostRateLimiter constructs a limiter with operator-configured defaults.
// Defensive normalization: if a caller (e.g. a test or a future code path
// that doesn't go through Config) passes non-positive defaults, fall back to
// the factory values so the limiter can never be constructed in an unusable
// state. Production config.go guarantees positive defaults via env fallbacks,
// so this branch is dead in normal bootstrap.
func NewHostRateLimiter(defaultRate float64, defaultBurst int, enabled bool) *HostRateLimiter {
	if defaultRate <= 0 {
		defaultRate = FactoryDefaultRatePerSec
	}
	if defaultBurst <= 0 {
		defaultBurst = FactoryDefaultBurst
	}
	return &HostRateLimiter{
		limiters:     make(map[string]*rate.Limiter),
		defaultRate:  defaultRate,
		defaultBurst: defaultBurst,
		enabled:      enabled,
	}
}

// Allow reports whether a request to host may proceed and consumes one token
// when it returns true. A previously unseen host gets a bucket lazy-created
// from the operator-configured defaults. When the limiter is disabled, Allow
// always returns true and never mutates bucket state.
func (l *HostRateLimiter) Allow(host string) bool {
	if !l.enabled {
		return true
	}
	limiter := l.getOrCreate(host)
	return limiter.Allow()
}

// SetHostRate replaces the host's bucket with one running at the given rate
// and burst. Non-positive inputs trigger fallback to the operator-configured
// defaults with a WARN log; the call never returns an error or panics.
func (l *HostRateLimiter) SetHostRate(host string, ratePerSec float64, burst int) {
	effRate := ratePerSec
	effBurst := burst
	substituted := false
	if effRate <= 0 || effBurst <= 0 {
		substituted = true
		effRate = l.defaultRate
		effBurst = l.defaultBurst
	}
	l.mu.Lock()
	l.limiters[host] = rate.NewLimiter(rate.Limit(effRate), effBurst)
	l.mu.Unlock()
	if substituted {
		log.Printf("WARN scheduler.host_rate_limiter: invalid SetHostRate(host=%q, rate=%v, burst=%v); substituted defaults rate=%v burst=%v",
			host, ratePerSec, burst, effRate, effBurst)
	}
}

// getOrCreate returns the existing limiter for host, or lazily creates one
// using a double-checked locking pattern to keep the read path lock-free
// for the common case.
func (l *HostRateLimiter) getOrCreate(host string) *rate.Limiter {
	l.mu.RLock()
	limiter, ok := l.limiters[host]
	l.mu.RUnlock()
	if ok {
		return limiter
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if limiter, ok = l.limiters[host]; ok {
		return limiter
	}
	limiter = rate.NewLimiter(rate.Limit(l.defaultRate), l.defaultBurst)
	l.limiters[host] = limiter
	return limiter
}
