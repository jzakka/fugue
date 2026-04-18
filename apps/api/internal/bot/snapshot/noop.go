package snapshot

import "context"

// NoopStore is used when the feature flag PIONEER_SNAPSHOT_ENABLED is
// off. Its Put is a no-op so Pioneer can call SaveRawContent
// unconditionally without extra flag checks at each call site.
type NoopStore struct{}

func (NoopStore) Put(ctx context.Context, normalizedURL string, body []byte) error {
	return nil
}
