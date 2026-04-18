// Package snapshot defines the shared conventions for Pioneer raw-HTML
// snapshots stored in object storage. Both Pioneer (writer) and Harvester
// (reader, see harvester-snapshot-first-fetch) import this package so the
// two stages derive the same object key from a normalized URL.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
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

// NormalizeURL applies the canonicalization that Pioneer (writer) and
// Harvester (reader, see harvester-snapshot-first-fetch) share when
// deriving a snapshot key. Keeping both consumers on this one function
// guarantees they agree on the sha256 hex segment for the same URL.
//
// Normalization rules (kept intentionally minimal — identical to the
// scheduler's own normalization so url_hash and snapshot key stay aligned):
//   - trim surrounding whitespace
//   - lowercase scheme and host
//   - strip fragment
//   - drop the default port (:80 for http, :443 for https)
//
// Malformed URLs (missing scheme/host) are returned unchanged so the caller
// still gets a deterministic key; the resulting hash is irrelevant because
// such URLs never reach the fetch path in practice.
func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) ||
		(u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) {
		u.Host = strings.SplitN(u.Host, ":", 2)[0]
	}
	return u.String()
}
