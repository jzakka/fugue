// Package urlcanon provides a single canonical URL normalizer shared by
// every component that must agree on url_hash / snapshot_key identity.
//
// Why it lives here: the pioneer crawler derives snapshot_key from a
// canonicalized URL, while the scheduler derives url_hash from a
// canonicalized URL. If the two canonicalizers differ, the same input URL
// maps to different frontier rows vs snapshot files — the two-table fanout
// silently desynchronizes. This package is the single source of truth.
//
// Rules applied by Canonical:
//   - scheme lowercased
//   - host lowercased, "www." prefix stripped, default port removed
//     (":80" for http, ":443" for https; non-default ports preserved)
//   - tracking query parameters removed; remaining keys sorted ascending
//   - trailing slash removed for non-root paths ("/" preserved)
//   - path case preserved
//   - fragment removed
package urlcanon

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// trackingParams lists query parameters stripped during canonicalization.
// These are marketing/attribution tokens that do not change page identity.
var trackingParams = map[string]struct{}{
	"utm_source":   {},
	"utm_medium":   {},
	"utm_campaign": {},
	"utm_term":     {},
	"utm_content":  {},
	"ref":          {},
	"fbclid":       {},
	"gclid":        {},
}

// Canonical returns the canonical form of raw. On parse failure it returns
// raw unchanged (best-effort semantics used by the crawler's link extractor
// where malformed hrefs should still flow through downstream filters).
func Canonical(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	apply(u)
	return u.String()
}

// CanonicalWithHost canonicalizes raw and also returns the canonicalized
// host. Unlike Canonical, this variant errors on empty input, unparseable
// input, or missing scheme/host — the scheduler uses it to reject ambiguous
// keys before they reach the url_hash unique index.
func CanonicalWithHost(raw string) (normalized, host string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("urlcanon: empty url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("urlcanon: url missing scheme or host: %q", raw)
	}
	apply(u)
	return u.String(), u.Hostname(), nil
}

// apply mutates u in place with the canonicalization rules. Extracted so
// Canonical and CanonicalWithHost stay in lockstep — any rule change lands
// in exactly one place.
func apply(u *url.URL) {
	u.Scheme = strings.ToLower(u.Scheme)

	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	host = stripDefaultPort(u.Scheme, host)
	u.Host = host

	q := u.Query()
	for param := range trackingParams {
		q.Del(param)
	}
	u.RawQuery = q.Encode()

	if u.Path != "/" && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}

	u.Fragment = ""
}

// stripDefaultPort removes ":80" from http hosts and ":443" from https hosts.
// Input host must already be lowercased by the caller.
func stripDefaultPort(scheme, host string) string {
	switch scheme {
	case "http":
		return strings.TrimSuffix(host, ":80")
	case "https":
		return strings.TrimSuffix(host, ":443")
	default:
		return host
	}
}
