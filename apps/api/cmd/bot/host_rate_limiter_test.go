package main

import (
	"testing"

	"github.com/chungsanghwa/fugue/apps/api/internal/config"
)

// TestBuildHostRateLimiter_OperatorOverriddenDefaults checks that operator
// defaults (rate=0.5, burst=3) reach the limiter through buildHostRateLimiter
// — this is the wiring guarantee the bot worker entrypoint owes the scheduler
// spec.
func TestBuildHostRateLimiter_OperatorOverriddenDefaults(t *testing.T) {
	cfg := config.SchedulerHostConfig{DefaultRatePerSec: 0.5, DefaultBurst: 3, Enabled: true}
	l := buildHostRateLimiter(cfg)
	if l == nil {
		t.Fatal("limiter should not be nil")
	}

	host := "operator-default.example.com"
	for i := 0; i < 3; i++ {
		if !l.Allow(host) {
			t.Fatalf("call #%d: expected Allow=true under burst=3", i+1)
		}
	}
	if l.Allow(host) {
		t.Fatal("call #4: expected Allow=false once burst=3 is exhausted")
	}
}

// TestBuildHostRateLimiter_DisabledShortCircuit checks that the enabled=false
// operator toggle reaches the limiter and short-circuits Allow().
func TestBuildHostRateLimiter_DisabledShortCircuit(t *testing.T) {
	cfg := config.SchedulerHostConfig{DefaultRatePerSec: 0.5, DefaultBurst: 3, Enabled: false}
	l := buildHostRateLimiter(cfg)

	host := "disabled.example.com"
	for i := 0; i < 100; i++ {
		if !l.Allow(host) {
			t.Fatalf("call #%d: expected Allow=true while disabled", i+1)
		}
	}
}

// TestBuildHostRateLimiter_FactoryDefaultsWhenEnvUnset checks that when no
// SCHEDULER_HOST_* env var is set, LoadSchedulerHostConfig + buildHostRateLimiter
// reproduces factory defaults (rate=1, burst=5, enabled=true) bit-for-bit —
// the legacy hardcoded behavior is preserved.
func TestBuildHostRateLimiter_FactoryDefaultsWhenEnvUnset(t *testing.T) {
	t.Setenv("SCHEDULER_HOST_DEFAULT_RATE_PER_SEC", "")
	t.Setenv("SCHEDULER_HOST_DEFAULT_BURST", "")
	t.Setenv("SCHEDULER_HOST_TOKEN_BUCKET_ENABLED", "")

	cfg := config.LoadSchedulerHostConfig()
	if cfg.DefaultRatePerSec != 1.0 {
		t.Errorf("DefaultRatePerSec: got %v, want 1.0", cfg.DefaultRatePerSec)
	}
	if cfg.DefaultBurst != 5 {
		t.Errorf("DefaultBurst: got %d, want 5", cfg.DefaultBurst)
	}
	if !cfg.Enabled {
		t.Error("Enabled: got false, want true")
	}

	l := buildHostRateLimiter(cfg)
	host := "factory.example.com"
	for i := 0; i < 5; i++ {
		if !l.Allow(host) {
			t.Fatalf("call #%d: expected Allow=true under factory burst=5", i+1)
		}
	}
	if l.Allow(host) {
		t.Fatal("call #6: expected Allow=false once factory burst=5 is exhausted")
	}
}

// TestLoadSchedulerHostConfig_OperatorEnvOverrides checks the env-to-config
// path that the operator scenario in the spec depends on. Operator sets
// SCHEDULER_HOST_DEFAULT_RATE_PER_SEC=0.5 and SCHEDULER_HOST_DEFAULT_BURST=3;
// LoadSchedulerHostConfig MUST return those values verbatim.
func TestLoadSchedulerHostConfig_OperatorEnvOverrides(t *testing.T) {
	t.Setenv("SCHEDULER_HOST_DEFAULT_RATE_PER_SEC", "0.5")
	t.Setenv("SCHEDULER_HOST_DEFAULT_BURST", "3")
	t.Setenv("SCHEDULER_HOST_TOKEN_BUCKET_ENABLED", "true")

	cfg := config.LoadSchedulerHostConfig()
	if cfg.DefaultRatePerSec != 0.5 {
		t.Errorf("DefaultRatePerSec: got %v, want 0.5", cfg.DefaultRatePerSec)
	}
	if cfg.DefaultBurst != 3 {
		t.Errorf("DefaultBurst: got %d, want 3", cfg.DefaultBurst)
	}
	if !cfg.Enabled {
		t.Error("Enabled: got false, want true")
	}
}

// TestLoadSchedulerHostConfig_DisabledFromEnv checks that operators can turn
// the host bucket off via env, satisfying "token bucket 검사를 비활성화할 수 있다"
// Requirement.
func TestLoadSchedulerHostConfig_DisabledFromEnv(t *testing.T) {
	t.Setenv("SCHEDULER_HOST_TOKEN_BUCKET_ENABLED", "false")

	cfg := config.LoadSchedulerHostConfig()
	if cfg.Enabled {
		t.Error("Enabled: got true, want false (SCHEDULER_HOST_TOKEN_BUCKET_ENABLED=false)")
	}
}
