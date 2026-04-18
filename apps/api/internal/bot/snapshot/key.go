// Package snapshot defines the shared conventions for Pioneer raw-HTML
// snapshots stored in object storage. Both Pioneer (writer) and Harvester
// (reader, see harvester-snapshot-first-fetch) import this package so the
// two stages derive the same object key from a normalized URL.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// SnapshotKeyPattern is the object-storage key template for Pioneer snapshots.
//
//	snapshots/<sha256_hex_64>/<yyyymmdd>.html.gz
//
// The first %s placeholder is the sha256 hex digest of the normalized URL
// (exactly 64 lowercase hex chars). The second %s placeholder is the UTC
// fetch date in yyyymmdd form.
const SnapshotKeyPattern = "snapshots/%s/%s.html.gz"

// HashNormalizedURL returns the sha256 hex digest (64 lowercase chars) of
// the given normalized URL string. Pioneer and Harvester share this function
// so they always agree on a snapshot's key segment for the same URL.
//
// The input MUST already be canonicalized by the bot's URL normalization
// rules (lowercased scheme/host, stripped fragment, default ports removed,
// etc.). This function does not re-normalize.
func HashNormalizedURL(normalizedURL string) string {
	sum := sha256.Sum256([]byte(normalizedURL))
	return hex.EncodeToString(sum[:])
}

// SnapshotKey returns the full object-storage key for a snapshot of the
// given normalized URL taken at time t. The date segment is derived from
// t.UTC() so two workers that fetch the same URL on the same calendar day
// produce the same key regardless of local time zone.
func SnapshotKey(normalizedURL string, t time.Time) string {
	return fmt.Sprintf(
		SnapshotKeyPattern,
		HashNormalizedURL(normalizedURL),
		t.UTC().Format("20060102"),
	)
}
