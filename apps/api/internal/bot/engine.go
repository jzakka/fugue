package bot

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

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

	// TODO: platform 필드 제거됨 - Source 플러그인 매핑 재설계 필요
	log.Printf("engine: found %d active sources but platform mapping removed", len(activeSources))
	_ = activeSources
	return nil
}
