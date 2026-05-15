package main

import (
	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// spec: bot "Pioneer 부트스트랩은 RobotsFilter에 HostRateLimiter를 wire한다"
//
// buildPioneerConsumer is the single production bootstrap path that
// assembles a *bot.PioneerConsumer for the pioneer subcommand. It MUST wire
// the same HostRateSetter instance into the FilterChain's RobotsFilter that
// the scheduler uses for its host token bucket, so that the four Scenarios of
// the existing Requirement "RobotsFilter는 Crawl-delay를 호스트 bucket에
// 반영한다" (openspec/specs/bot/spec.md) are observed in production.
//
// The RobotsFilter is also returned alongside the consumer so that the
// regression test in pioneer_consumer_builder_test.go can assert the wiring
// deterministically via RobotsFilter.RateSetter().
//
// If this function ever passes nil to bot.NewRobotsFilter, or constructs a
// RobotsFilter whose HostRateSetter differs from the one owned by the
// scheduler, the spec is being violated.
func buildPioneerConsumer(
	sched scheduler.URLScheduler,
	store snapshot.SnapshotStore,
	rl bot.HostRateSetter,
) (*bot.PioneerConsumer, *bot.RobotsFilter) {
	robots := bot.NewRobotsFilter(rl)
	chain := bot.NewFilterChain(
		&bot.DomainFilter{},
		&bot.ExtensionFilter{},
		&bot.PathPatternFilter{},
		robots,
		bot.NewCanonicalDedupFilter(nil),
	)
	consumer := bot.NewPioneerConsumer(sched, store, chain, bot.NewDefaultConsumerFetcher())
	return consumer, robots
}
