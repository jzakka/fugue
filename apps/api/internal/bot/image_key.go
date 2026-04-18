package bot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// normalizeImageURL applies the normalization rules used as SHA-256 input for
// the cache key:
//  1. Resolve as absolute URL against pageURL (if provided).
//  2. Strip fragment (#...).
//  3. Lower-case host.
//  4. Preserve query parameters as-is (no sort/remove).
//
// Returns the normalized URL string, or the input raw URL when parsing fails.
func normalizeImageURL(rawURL, pageURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if !u.IsAbs() && pageURL != "" {
		if base, err := url.Parse(pageURL); err == nil {
			u = base.ResolveReference(u)
		}
	}

	u.Fragment = ""
	u.RawFragment = ""
	u.Host = strings.ToLower(u.Host)

	return u.String()
}

// sha256Hex returns lowercase hex SHA-256 of s.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// extensionForImage picks an extension for the cache key.
// Priority:
//  1. Content-Type mapping (image/jpeg→.jpg, image/png→.png, image/webp→.webp, image/gif→.gif).
//  2. Fallback: URL path extension (if known image extension).
//  3. Fallback: ".bin".
func extensionForImage(contentType, rawURL string) string {
	if ext := extensionFromContentType(contentType); ext != "" {
		return ext
	}
	if ext := extensionFromURLPath(rawURL); ext != "" {
		return ext
	}
	return ".bin"
}

func extensionFromContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	// Strip parameters e.g. "image/jpeg; charset=..."
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	return ""
}

func extensionFromURLPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(path.Ext(u.Path))
	switch ext {
	case ".jpg", ".jpeg":
		return ".jpg"
	case ".png", ".webp", ".gif":
		return ext
	}
	return ""
}

// imageCacheKey builds the object storage key for an image cache entry.
// Format: images/<sha256_hex>/<unix_ts>.<ext>
func imageCacheKey(hash string, unixTS int64, ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("images/%s/%d%s", hash, unixTS, ext)
}
