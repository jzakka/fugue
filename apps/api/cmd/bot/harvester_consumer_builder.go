package main

import (
	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
	"github.com/chungsanghwa/fugue/apps/api/internal/scheduler"
)

// spec: harvester "Harvester 워커 부트스트랩은 미디어 후보 유효성 검증기를 wire한다"
//
// buildHarvesterConsumer is the single production bootstrap path that
// constructs a *bot.HarvesterConsumer for the harvester subcommand. It MUST
// install a MediaValidator so the four media-validation SHALL Requirements
// in openspec/specs/harvester/spec.md ("미디어 후보 유효성 검증", "정본 키 영속
// 제한", "검증 실패 사유의 og_data 기록", "Pin primary media invariant") are
// enforced in production. The wiring is observable via the returned
// consumer's HasMediaValidator() accessor, which the regression test in
// harvester_consumer_builder_test.go asserts.
//
// If this function ever returns a consumer with HasMediaValidator()==false,
// the spec is being violated.
func buildHarvesterConsumer(
	sched scheduler.URLScheduler,
	fetcher bot.Fetcher,
	registry bot.AdapterRegistry,
	extractor *bot.GenericExtractor,
	classifier *bot.Classifier,
	pipeline bot.DocumentPipeline,
) *bot.HarvesterConsumer {
	return bot.NewHarvesterConsumer(
		sched, fetcher, registry, extractor, classifier, pipeline,
	).WithMediaValidator(bot.NewDefaultMediaValidator())
}
