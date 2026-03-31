package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	statePrefix = "oauth_state:"
	stateTTL    = 10 * time.Minute
)

type StateData struct {
	Provider string `json:"provider"`
	ReturnTo string `json:"return_to"`
}

type StateManager struct {
	rdb *redis.Client
}

func NewStateManager(rdb *redis.Client) *StateManager {
	return &StateManager{rdb: rdb}
}

// CreateState generates a random state token and stores it in Redis.
func (m *StateManager) CreateState(ctx context.Context, provider, returnTo string) (string, error) {
	returnTo = validateReturnTo(returnTo)

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	state := base64.URLEncoding.EncodeToString(b)

	data, err := json.Marshal(StateData{
		Provider: provider,
		ReturnTo: returnTo,
	})
	if err != nil {
		return "", fmt.Errorf("marshal state: %w", err)
	}

	key := statePrefix + state
	if err := m.rdb.Set(ctx, key, data, stateTTL).Err(); err != nil {
		return "", fmt.Errorf("store state: %w", err)
	}

	return state, nil
}

// VerifyState atomically reads and deletes the state (GETDEL).
func (m *StateManager) VerifyState(ctx context.Context, state string) (*StateData, error) {
	key := statePrefix + state
	val, err := m.rdb.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("invalid or expired state")
	}
	if err != nil {
		return nil, fmt.Errorf("verify state: %w", err)
	}

	var data StateData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &data, nil
}

// validateReturnTo ensures the return URL is a safe relative path.
func validateReturnTo(returnTo string) string {
	if returnTo == "" {
		return "/"
	}
	// Must start with /
	if !strings.HasPrefix(returnTo, "/") {
		return "/"
	}
	// Reject protocol-relative URLs and backslash tricks
	if strings.HasPrefix(returnTo, "//") || strings.Contains(returnTo, "\\") {
		return "/"
	}
	return returnTo
}
