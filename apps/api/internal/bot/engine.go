package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

// FuguebotUUID is the fixed system account UUID for the fuguebot.
var FuguebotUUID = uuid.MustParse("00000000-0000-0000-0000-00000000f096")

// CrawlStats tracks statistics for a single crawl run.
type CrawlStats struct {
	CrawledCount int `json:"crawled_count"`
	SkippedCount int `json:"skipped_count"`
	FailedCount  int `json:"failed_count"`
}

// Engine orchestrates the crawl pipeline:
// load sources -> crawl -> download -> dedup -> tag -> create pin.
type Engine struct {
	q          *db.Queries
	store      *storage.Client
	dedup      *Deduplicator
	tagger     *Tagger
	downloader *Downloader
	sources    map[string]Source // platform name -> source implementation
}

// NewEngine creates a new crawl engine.
func NewEngine(q *db.Queries, store *storage.Client) *Engine {
	return &Engine{
		q:          q,
		store:      store,
		dedup:      NewDeduplicator(q),
		tagger:     NewTagger(q),
		downloader: NewDownloader(store),
		sources:    make(map[string]Source),
	}
}

// RegisterSource adds a source implementation to the engine.
func (e *Engine) RegisterSource(s Source) {
	e.sources[s.Name()] = s
}

// Run executes the full crawl pipeline for all active sources.
func (e *Engine) Run(ctx context.Context) error {
	activeSources, err := e.q.ListActiveBotSources(ctx)
	if err != nil {
		return fmt.Errorf("engine: list active sources: %w", err)
	}

	if len(activeSources) == 0 {
		log.Println("engine: no active sources found")
		return nil
	}

	for _, bs := range activeSources {
		src, ok := e.sources[bs.Platform]
		if !ok {
			log.Printf("engine: no implementation for platform %q (source: %s)", bs.Platform, bs.Name)
			continue
		}

		log.Printf("engine: crawling source %q (platform: %s)", bs.Name, bs.Platform)
		stats := e.crawlSource(ctx, src, bs)

		// Update stats in DB
		statsJSON, _ := json.Marshal(stats)
		if err := e.q.UpdateBotSourceStats(ctx, db.UpdateBotSourceStatsParams{
			ID:    bs.ID,
			Stats: pqtype.NullRawMessage{RawMessage: statsJSON, Valid: true},
		}); err != nil {
			log.Printf("engine: failed to update stats for source %q: %v", bs.Name, err)
		}

		log.Printf("engine: source %q done — crawled: %d, skipped: %d, failed: %d",
			bs.Name, stats.CrawledCount, stats.SkippedCount, stats.FailedCount)
	}

	return nil
}

// crawlSource runs the pipeline for a single source.
func (e *Engine) crawlSource(ctx context.Context, src Source, bs db.BotSource) CrawlStats {
	var stats CrawlStats

	items, err := src.Crawl(ctx)
	if err != nil {
		log.Printf("engine: crawl error for %q: %v", src.Name(), err)
		stats.FailedCount++
		return stats
	}

	for _, item := range items {
		if err := e.processItem(ctx, item); err != nil {
			if err == errSkipped {
				stats.SkippedCount++
			} else {
				log.Printf("engine: process item %q error: %v", item.Title, err)
				stats.FailedCount++
			}
			continue
		}
		stats.CrawledCount++
	}

	return stats
}

var errSkipped = fmt.Errorf("skipped")

// processItem handles a single raw item through the pipeline:
// dedup -> download -> tag -> create pin.
func (e *Engine) processItem(ctx context.Context, item RawItem) error {
	// 1. Dedup: check if URL already exists
	if item.SourceURL != "" && e.dedup.Exists(ctx, item.SourceURL) {
		log.Printf("engine: dedup skip %q (url: %s)", item.Title, item.SourceURL)
		return errSkipped
	}

	// 2. Tag matching
	tagIDs := e.tagger.MatchTags(ctx, item.Title, item.Description)
	if len(tagIDs) == 0 {
		log.Printf("engine: no tags matched for %q, skipping", item.Title)
		return errSkipped
	}
	// Limit to 10 tags max
	if len(tagIDs) > 10 {
		tagIDs = tagIDs[:10]
	}

	// 3. Download media and upload to S3
	dlResult, err := e.downloader.DownloadAndUpload(ctx, item.MediaURL, item.MediaType)
	if err != nil {
		return fmt.Errorf("download %q: %w", item.Title, err)
	}

	// 4. Create pin
	pin, err := e.q.CreatePin(ctx, db.CreatePinParams{
		CreatorID:   FuguebotUUID,
		MediaUrl:    dlResult.MediaURL,
		MediaType:   dlResult.MediaType,
		Url:         toNullString(item.SourceURL),
		Title:       item.Title,
		Description: toNullString(item.Description),
	})
	if err != nil {
		return fmt.Errorf("create pin %q: %w", item.Title, err)
	}

	// 5. Link tags
	for _, tagID := range tagIDs {
		if err := e.q.LinkPinTag(ctx, db.LinkPinTagParams{
			PinID: pin.ID,
			TagID: tagID,
		}); err != nil {
			log.Printf("engine: link tag error (pin=%s, tag=%s): %v", pin.ID, tagID, err)
		}
	}

	log.Printf("engine: created pin %s for %q (%d tags)", pin.ID, item.Title, len(tagIDs))
	return nil
}
