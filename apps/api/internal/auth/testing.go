package auth

import (
	"context"

	"github.com/google/uuid"
)

// SetCreatorIDForTest injects a creator ID into context for testing.
// This should only be used in test code.
func SetCreatorIDForTest(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, creatorIDKey, id)
}
