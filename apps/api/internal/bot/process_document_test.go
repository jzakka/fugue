package bot

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// safeBotDB is a thread-safe BotDB stub for concurrency testing.
type safeBotDB struct {
	mu        sync.Mutex
	existing  map[string]bool
	callCount int
}

func newSafeBotDB() *safeBotDB { return &safeBotDB{existing: map[string]bool{}} }

func (s *safeBotDB) BotPinExistsByURL(_ context.Context, arg db.BotPinExistsByURLParams) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.existing[arg.Url.String], nil
}

func (s *safeBotDB) CreatePin(_ context.Context, _ db.CreatePinParams) (db.Pin, error) {
	return db.Pin{ID: uuid.New()}, nil
}

func (s *safeBotDB) UpsertBotPinByURL(_ context.Context, arg db.UpsertBotPinByURLParams) (db.UpsertBotPinByURLRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
	inserted := !s.existing[arg.Url.String]
	s.existing[arg.Url.String] = true
	return db.UpsertBotPinByURLRow{
		ID:        uuid.New(),
		CreatorID: arg.CreatorID,
		Url:       arg.Url,
		MediaUrl:  arg.MediaUrl,
		MediaType: arg.MediaType,
		Inserted:  inserted,
	}, nil
}

// TestProcessDocument_PopulatesMediaURLFromThumbnail is a regression guard
// for the `media_url NOT NULL` constraint: ProcessDocument must pick a
// non-empty media_url from thumbnail_url or first media candidate.
func TestProcessDocument_PopulatesMediaURLFromThumbnail(t *testing.T) {
	mockDB := newSafeBotDB()
	p := NewHarvestPipeline(mockDB, NewMockStorage(), WithImageCacheEnabled(false))

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: "https://example.com/a",
		ThumbnailURL: "https://cdn.example.com/img.jpg",
	}
	created, pinID, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if !created {
		t.Errorf("created = false, want true on first insert")
	}
	if pinID == uuid.Nil {
		t.Errorf("pinID is Nil")
	}
}

func TestProcessDocument_PopulatesMediaURLFromCandidateWhenNoThumbnail(t *testing.T) {
	mockDB := newSafeBotDB()
	p := NewHarvestPipeline(mockDB, NewMockStorage(), WithImageCacheEnabled(false))

	doc := PinDocument{
		Title:        "t",
		CanonicalURL: "https://example.com/a",
		MediaCandidates: []MediaCandidate{
			{Type: "video", URL: "https://cdn.example.com/v.mp4"},
		},
	}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
}

func TestProcessDocument_ErrorsWhenMediaURLAbsent(t *testing.T) {
	mockDB := newSafeBotDB()
	p := NewHarvestPipeline(mockDB, NewMockStorage(), WithImageCacheEnabled(false))

	doc := PinDocument{Title: "t", CanonicalURL: "https://example.com/a"}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err == nil {
		t.Fatal("expected error when no thumbnail and no media candidates")
	}
}

func TestProcessDocument_ErrorsWhenCanonicalMissing(t *testing.T) {
	mockDB := newSafeBotDB()
	p := NewHarvestPipeline(mockDB, NewMockStorage(), WithImageCacheEnabled(false))

	doc := PinDocument{Title: "t", ThumbnailURL: "https://cdn.example.com/i.jpg"}
	_, _, err := p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
	if err == nil {
		t.Fatal("expected error on missing canonical_url")
	}
}

// TestProcessDocument_ConcurrentCallsAreRaceSafe verifies the harvester
// code path through UpsertBotPinByURL is free of data races. The actual
// "exactly one row" guarantee is provided by the Postgres partial unique
// index and is covered by integration tests; this test only exercises the
// Go code under -race.
func TestProcessDocument_ConcurrentCallsAreRaceSafe(t *testing.T) {
	mockDB := newSafeBotDB()
	p := NewHarvestPipeline(mockDB, NewMockStorage(), WithImageCacheEnabled(false))
	doc := PinDocument{
		Title:        "t",
		CanonicalURL: "https://example.com/a",
		ThumbnailURL: "https://cdn.example.com/img.jpg",
	}

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, _, _ = p.ProcessDocument(context.Background(), db.BotGraphNode{}, doc)
		}()
	}
	wg.Wait()

	if mockDB.callCount != workers {
		t.Errorf("callCount = %d, want %d", mockDB.callCount, workers)
	}
}
