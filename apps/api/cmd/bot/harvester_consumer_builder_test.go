package main

import (
	"testing"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
)

// Regression test for openspec change fix-harvester-wire-media-validator.
// Verifies that the single production bootstrap path used by the harvester
// subcommand returns a consumer with a MediaValidator installed. Removing
// the WithMediaValidator(...) call from buildHarvesterConsumer would
// silently re-introduce the production wiring gap; this test prevents that.
func TestBuildHarvesterConsumer_WiresMediaValidator(t *testing.T) {
	c := buildHarvesterConsumer(nil, nil, nil, nil, nil, bot.NewMockPipeline())
	if !c.HasMediaValidator() {
		t.Fatal("buildHarvesterConsumer must return a consumer with HasMediaValidator()==true; production wiring is missing")
	}
}
