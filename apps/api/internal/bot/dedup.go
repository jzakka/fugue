package bot

import (
	"context"
	"log"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Deduplicator checks whether a pin URL already exists in the database.
type Deduplicator struct {
	q *db.Queries
}

// NewDeduplicator creates a new Deduplicator.
func NewDeduplicator(q *db.Queries) *Deduplicator {
	return &Deduplicator{q: q}
}

// Exists returns true if the given URL already has a pin in the database.
func (d *Deduplicator) Exists(ctx context.Context, url string) bool {
	exists, err := d.q.PinURLExists(ctx, toNullString(url))
	if err != nil {
		log.Printf("dedup: error checking URL %q: %v", url, err)
		return false // on error, allow the item through
	}
	return exists
}
