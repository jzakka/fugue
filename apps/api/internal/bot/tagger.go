package bot

import (
	"context"
	"log"
	"strings"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Tagger matches raw item text against existing tags in the database.
type Tagger struct {
	q *db.Queries
}

// NewTagger creates a new Tagger.
func NewTagger(q *db.Queries) *Tagger {
	return &Tagger{q: q}
}

// MatchTags finds tags whose names appear (case-insensitive) in the title or description.
// Returns matched tag UUIDs. Returns nil if no tags match.
func (t *Tagger) MatchTags(ctx context.Context, title, description string) []uuid.UUID {
	allTags, err := t.q.ListAllTags(ctx)
	if err != nil {
		log.Printf("tagger: failed to list tags: %v", err)
		return nil
	}

	text := strings.ToLower(title + " " + description)
	var matched []uuid.UUID

	for _, tag := range allTags {
		tagName := strings.ToLower(tag.Name)
		if strings.Contains(text, tagName) {
			matched = append(matched, tag.ID)
		}
	}

	return matched
}
