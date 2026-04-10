package ai

import (
	"context"
)

// Client defines the interface for AI model interactions.
// This abstraction allows for future model swaps or multi-provider support.
type Client interface {
	// Call sends a prompt to the AI model and returns the response.
	// Context can be used for timeout and cancellation.
	Call(ctx context.Context, prompt string) (string, error)
}
