package bot

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Env variable names for Harvester-wide tuning (Section 9 of
// harvester-pin-document). Values are read at process start; changes
// require a restart.
const (
	// EnvHarvesterBodyTextMinBytes overrides the classifier body-text
	// threshold (in bytes). Default 200.
	EnvHarvesterBodyTextMinBytes = "HARVESTER_BODY_TEXT_MIN_BYTES"

	// EnvHarvesterLinkDensity overrides the classifier link-density
	// threshold. Default 0.5 (links/words).
	EnvHarvesterLinkDensity = "HARVESTER_LINK_DENSITY_THRESHOLD"

	// EnvHarvesterMediaCandidatesMax overrides the media-candidates cap in
	// the generic extractor. Default 50.
	EnvHarvesterMediaCandidatesMax = "HARVESTER_MEDIA_CANDIDATES_MAX"

	// EnvFugueBotCreatorID overrides the bot creator UUID. IMMUTABLE-sync
	// policy (source.go) means this value MUST match the migration predicate
	// and the upsert ON CONFLICT literal; a different value will cause
	// arbiter inference to miss the partial unique index. The override
	// exists only for test/staging environments that have a coordinated
	// migration.
	EnvFugueBotCreatorID = "FUGUE_BOT_CREATOR_ID"
)

// NewClassifierFromEnv returns a Classifier with thresholds overridden by
// the HARVESTER_BODY_TEXT_MIN_BYTES and HARVESTER_LINK_DENSITY_THRESHOLD
// env vars. Invalid values are ignored with a warning.
func NewClassifierFromEnv() *Classifier {
	c := NewClassifier()
	if v := strings.TrimSpace(os.Getenv(EnvHarvesterBodyTextMinBytes)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.BodyTextMinBytes = n
		} else {
			log.Printf("harvester: invalid %s=%q, keeping default %d", EnvHarvesterBodyTextMinBytes, v, c.BodyTextMinBytes)
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvHarvesterLinkDensity)); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			c.LinkDensityThreshold = n
		} else {
			log.Printf("harvester: invalid %s=%q, keeping default %v", EnvHarvesterLinkDensity, v, c.LinkDensityThreshold)
		}
	}
	return c
}

// NewGenericExtractorFromEnv returns a GenericExtractor with the
// media-candidates cap overridden via env var.
func NewGenericExtractorFromEnv() *GenericExtractor {
	e := NewGenericExtractor()
	if v := strings.TrimSpace(os.Getenv(EnvHarvesterMediaCandidatesMax)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			e.MediaCandidatesLimit = n
		} else {
			log.Printf("harvester: invalid %s=%q, keeping default %d", EnvHarvesterMediaCandidatesMax, v, e.MediaCandidatesLimit)
		}
	}
	return e
}

// ApplyBotCreatorIDFromEnv overrides the compiled-in BotCreatorID if the
// FUGUE_BOT_CREATOR_ID env var is set to a valid UUID. Logs a warning
// describing the IMMUTABLE-sync policy violation risk. Call exactly once at
// process start, before any pin upserts.
func ApplyBotCreatorIDFromEnv() {
	v := strings.TrimSpace(os.Getenv(EnvFugueBotCreatorID))
	if v == "" {
		return
	}
	parsed, err := uuid.Parse(v)
	if err != nil {
		log.Printf("harvester: invalid %s=%q (not a UUID), keeping default %s", EnvFugueBotCreatorID, v, BotCreatorID)
		return
	}
	if parsed == BotCreatorID {
		return
	}
	log.Printf("harvester: %s overridden to %s (default %s); partial unique index predicate MUST match this UUID literal or upserts will fail silently", EnvFugueBotCreatorID, parsed, BotCreatorID)
	BotCreatorID = parsed
}
