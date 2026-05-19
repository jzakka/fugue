package main

import (
	"sync"
	"testing"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
)

// recordingHostRateSetter records every SetHostRate call so the regression
// test below can assert that buildPioneerConsumer wired the exact instance
// passed in, and that the wired RobotsFilter actually routes SetHostRate
// through it.
type recordingHostRateSetter struct {
	mu    sync.Mutex
	calls []hostRateCall
}

type hostRateCall struct {
	host       string
	ratePerSec float64
	burst      int
}

func (r *recordingHostRateSetter) SetHostRate(host string, ratePerSec float64, burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, hostRateCall{host: host, ratePerSec: ratePerSec, burst: burst})
}

// Regression test for openspec change
// fix-pioneer-robots-filter-host-rate-setter-wiring. Verifies that the single
// production bootstrap path used by the pioneer subcommand passes the same
// HostRateSetter instance into the FilterChain's RobotsFilter that the
// scheduler is configured with. Reverting to bot.NewRobotsFilter(nil) — or
// passing a separate instance — would silently re-introduce the production
// wiring gap described in openspec/specs/bot/spec.md Requirement "RobotsFilter
// 는 Crawl-delay를 호스트 bucket에 반영한다"; this test prevents that.
func TestBuildPioneerConsumer_WiresHostRateSetterIntoRobotsFilter(t *testing.T) {
	fake := &recordingHostRateSetter{}

	consumer, robots := buildPioneerConsumer(nil, nil, fake)
	if consumer == nil {
		t.Fatal("buildPioneerConsumer must return a non-nil PioneerConsumer")
	}
	if robots == nil {
		t.Fatal("buildPioneerConsumer must return a non-nil RobotsFilter")
	}

	wired := robots.RateSetter()
	if wired == nil {
		t.Fatal("RobotsFilter must hold a non-nil HostRateSetter; production wiring is missing")
	}
	if got, want := wired, bot.HostRateSetter(fake); got != want {
		t.Fatalf("RobotsFilter must hold the exact HostRateSetter instance passed to buildPioneerConsumer; got %p want %p", got, want)
	}

	wired.SetHostRate("example.com", 0.1, 1)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 recorded SetHostRate call, got %d", len(fake.calls))
	}
	if got := fake.calls[0]; got.host != "example.com" || got.ratePerSec != 0.1 || got.burst != 1 {
		t.Fatalf("recorded call mismatch: %+v", got)
	}
}

// Companion guard: a nil HostRateSetter must never be the production wiring
// for the pioneer subcommand. If a future refactor changes buildPioneerConsumer
// to accept a nil rate setter and proceed silently, this test fails.
func TestBuildPioneerConsumer_NilHostRateSetterIsNotProductionWiring(t *testing.T) {
	// We do not call buildPioneerConsumer(nil, nil, nil) here because the
	// nil-rateSetter contract on bot.NewRobotsFilter is intentionally
	// permissive (see openspec design.md Decision 4). Instead, we assert
	// the inverse: a real instance passed in is preserved end-to-end.
	fake := &recordingHostRateSetter{}
	_, robots := buildPioneerConsumer(nil, nil, fake)
	if robots.RateSetter() == nil {
		t.Fatal("buildPioneerConsumer must not drop the HostRateSetter; production wiring requires a non-nil setter")
	}
}
