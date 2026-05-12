package bot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// imageCacheKeyPrefix is the object storage namespace for cached primary
// images. It is intentionally separate from the body-media prefix (bot/<uuid>)
// so monitoring and lifecycle policies can be applied independently
// (Decision 7).
const imageCacheKeyPrefix = "images"

// defaultImageExt is used when neither Content-Type nor URL path yield a
// known image extension (Decision 9).
const defaultImageExt = ".bin"

// normalizeImageURL applies Decision 3's normalization rules to a candidate
// URL prior to hashing:
//  1. Resolve relative URLs against pageURL.
//  2. Strip fragment.
//  3. Lower-case the scheme.
//  4. Lower-case the host.
//  5. Preserve query parameters as-is.
//  6. Preserve path, userinfo, etc. as-is (RFC 3986 path is case-sensitive).
//
// Returns the normalized absolute URL string. Returns an error if the URL
// cannot be parsed or resolved to an absolute URL.
func normalizeImageURL(rawURL, pageURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	if !u.IsAbs() {
		if pageURL == "" {
			return "", fmt.Errorf("cannot resolve relative URL without page URL")
		}
		base, err := url.Parse(pageURL)
		if err != nil {
			return "", fmt.Errorf("parse page URL: %w", err)
		}
		u = base.ResolveReference(u)
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

// hashImageURL returns the lower-case hex SHA-256 (64 chars) of the input
// string. The input is expected to be a normalized URL from
// normalizeImageURL; hashing a non-normalized URL will produce a different
// hash for otherwise-equivalent candidates.
func hashImageURL(normalizedURL string) string {
	sum := sha256.Sum256([]byte(normalizedURL))
	return hex.EncodeToString(sum[:])
}

// contentTypeToExt maps known image MIME types to canonical file extensions
// per Decision 9. Returns the extension including the leading dot, or empty
// string when the Content-Type is unknown/unmapped.
func contentTypeToExt(contentType string) string {
	// Trim any parameters (e.g. "image/jpeg; charset=binary").
	ct := contentType
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ""
	}
}

// urlPathExt returns the lower-cased extension (including leading dot) of
// the URL path, or empty string if there is none.
func urlPathExt(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := path.Ext(u.Path)
	return strings.ToLower(ext)
}

// resolveImageExt applies Decision 9's fallback chain:
//  1. Content-Type → extension.
//  2. URL path extension (lower-cased).
//  3. Default ".bin".
func resolveImageExt(contentType, rawURL string) string {
	if ext := contentTypeToExt(contentType); ext != "" {
		return ext
	}
	if ext := urlPathExt(rawURL); ext != "" {
		return ext
	}
	return defaultImageExt
}

// buildImageCacheKey assembles the object storage key for a cached image.
// Format: images/<hash>/<unix_ts>.<ext>
//
//   - hash: 64-char lower-case hex SHA-256 of the normalized URL.
//   - unix_ts: seconds since epoch at cache time (Decision 3 / Decision re-cache).
//   - ext: extension chosen by resolveImageExt.
//
// Re-caching the same URL at a different second yields a different key, so
// prior objects are not overwritten (spec: SHALL NOT overwrite).
func buildImageCacheKey(normalizedURL, contentType string, unixTS int64) string {
	hash := hashImageURL(normalizedURL)
	ext := resolveImageExt(contentType, normalizedURL)
	return fmt.Sprintf("%s/%s/%d%s", imageCacheKeyPrefix, hash, unixTS, ext)
}
