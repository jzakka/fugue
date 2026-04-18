// Package snapshot provides Pioneer's raw-HTML snapshot storage.
//
// Per OpenSpec change "pioneer-snapshot-storage", Pioneer uploads each
// successfully-fetched raw HTML response to object storage so that
// downstream stages (Harvester, retries, debugging) can re-read the
// exact same bytes without re-fetching the origin.
//
// The package exposes:
//   - SnapshotKey: the canonical S3 key derived from the normalized URL
//     and UTC fetch date. Pioneer and Harvester MUST share this function
//     so the harvester change ("harvester-snapshot-first-fetch") can
//     reconstruct the same key from a normalized URL alone.
//   - SnapshotStore: the storage interface (Put-only here; reads land in
//     the harvester change).
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// SnapshotKeyPattern is the canonical S3 object key format.
//
// Format: snapshots/<sha256_hex>/<yyyymmdd>.html.gz
//   - sha256_hex: lowercase hex of sha256(normalized_url), exactly 64 chars
//   - yyyymmdd:   UTC fetch date
//   - .html.gz:   content type and gzip compression
//
// The key format is part of the external behavior contract (see
// design.md Decision 1a). Changing the hash function or layout requires
// migrating every stored object plus all consumers (Pioneer + Harvester).
const SnapshotKeyPattern = "snapshots/%s/%s.html.gz"

// HashNormalizedURL returns the lowercase hex sha256 digest of the
// normalized URL string. Output is exactly 64 hex characters.
//
// Pioneer and Harvester MUST call this with the same normalization rules
// (the existing bot URL normalization) so they derive identical keys.
func HashNormalizedURL(normalizedURL string) string {
	sum := sha256.Sum256([]byte(normalizedURL))
	return hex.EncodeToString(sum[:])
}

// SnapshotKey builds the S3 object key for a snapshot of the given
// normalized URL captured at time t. The date segment is always rendered
// in UTC so identical normalized URLs fetched on the same UTC day produce
// the same key (idempotent overwrite).
func SnapshotKey(normalizedURL string, t time.Time) string {
	return "snapshots/" + HashNormalizedURL(normalizedURL) + "/" +
		t.UTC().Format("20060102") + ".html.gz"
}
