package main

import (
	"github.com/chungsanghwa/fugue/apps/api/internal/config"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// buildHostRateLimiter constructs a HostRateLimiter from operator-supplied
// scheduler host config, with no fallback applied here — operator defaults vs.
// factory defaults are resolved by config.LoadSchedulerHostConfig before this
// is called.
func buildHostRateLimiter(cfg config.SchedulerHostConfig) *scheduler.HostRateLimiter {
	return scheduler.NewHostRateLimiter(cfg.DefaultRatePerSec, cfg.DefaultBurst, cfg.Enabled)
}
