package bot

import (
	"encoding/json"
	"time"
)

// MediaCandidate is a single piece of media discovered on a page that may be
// used as a Pin's primary media or surfaced as a secondary candidate in
// og_data.media_candidates.
type MediaCandidate struct {
	Type   string `json:"type"`             // "image" | "video" | "audio"
	URL    string `json:"url"`              // absolute URL
	Width  int    `json:"width,omitempty"`  // intrinsic width if known
	Height int    `json:"height,omitempty"` // intrinsic height if known
}

// ClassifierReason enumerates the reasons a page may be deemed non-pinnable.
type ClassifierReason string

const (
	ClassifierReasonListing        ClassifierReason = "listing"
	ClassifierReasonEmptyBody      ClassifierReason = "empty_body"
	ClassifierReasonNoPrimaryMedia ClassifierReason = "no_primary_media"
)

// PinDocument is the canonical, single-Pin representation of an HTML page
// produced by the Harvester pipeline. It is the contract between extractors
// (generic + per-site adapters) and the upsert step.
//
// Invariants:
//   - Always a non-nil value (even for non-pinnable pages — the classifier
//     decides whether to upsert).
//   - CanonicalURL is the value that will be written to pins.url; the
//     extractor is responsible for any cross-domain fallback so the
//     Harvester can trust this field directly.
//   - BodyText is raw text BEFORE the description-length cut; the Harvester
//     is responsible for the rune-safe truncation when persisting.
type PinDocument struct {
	Title           string           `json:"title,omitempty"`
	BodyText        string           `json:"body_text,omitempty"`
	CanonicalURL    string           `json:"canonical_url,omitempty"`
	ThumbnailURL    string           `json:"thumbnail_url,omitempty"`
	MediaCandidates []MediaCandidate `json:"media_candidates,omitempty"`
	Lang            string           `json:"lang,omitempty"`
	Author          string           `json:"author,omitempty"`
	PublishedAt     *time.Time       `json:"published_at,omitempty"`
	OGData          OGData           `json:"og_data,omitempty"`
}

// OGData is the structured payload persisted to pins.og_data. It holds
// metadata that does NOT belong in dedicated columns: extractor identity,
// classifier verdict, source URL back-reference, and (optionally) secondary
// media candidates.
//
// Notably ABSENT (do not add):
//   - body_text  → stored in pins.description (rune-truncated)
//   - canonical_url → stored in pins.url (single source of truth)
type OGData struct {
	// Source is the URL the Harvester actually fetched (back-reference).
	Source string `json:"source,omitempty"`
	// Extractor identifies which extractor produced this document, e.g.
	// "generic", "script:<site_id>", or a per-site adapter name.
	Extractor string `json:"extractor,omitempty"`
	// Classifier records the verdict from the content classifier.
	Classifier *ClassifierVerdict `json:"classifier,omitempty"`
	// MediaCandidates duplicates PinDocument.MediaCandidates for persistence.
	MediaCandidates []MediaCandidate `json:"media_candidates,omitempty"`
	// Lang is the detected/declared language of the page (BCP-47).
	Lang string `json:"lang,omitempty"`
	// Author is the page author when discoverable (OG/JSON-LD/byline).
	Author string `json:"author,omitempty"`
	// PublishedAt is the page publication time when discoverable.
	PublishedAt *time.Time `json:"published_at,omitempty"`
	// MediaValidation records media-candidate validation rejections that
	// were applied before the classifier ran. It satisfies the spec's
	// "검증 실패 사유의 og_data 기록" requirement: the externally observable
	// minimum is (a) rejected_count and (b) per-reason counts.
	MediaValidation *MediaValidationRecord `json:"media_validation,omitempty"`
}

// MediaValidationRecord aggregates the count and per-reason breakdown of
// media candidates rejected by the validator. The schema is observable
// (count + reason map) without locking specific reason strings; reason keys
// match MediaValidationReason values.
type MediaValidationRecord struct {
	RejectedCount int            `json:"rejected_count"`
	Reasons       map[string]int `json:"reasons,omitempty"`
}

// ClassifierVerdict is what the content classifier returns and what is
// persisted under og_data.classifier.
type ClassifierVerdict struct {
	Pinnable bool             `json:"pinnable"`
	Reason   ClassifierReason `json:"reason,omitempty"`
}

// MarshalOGData serialises an OGData value to JSON bytes suitable for
// writing into the pins.og_data jsonb column.
func MarshalOGData(d OGData) ([]byte, error) {
	return json.Marshal(d)
}

// UnmarshalOGData decodes a pins.og_data jsonb payload back into OGData.
// A nil/empty payload yields a zero-value OGData and a nil error.
func UnmarshalOGData(raw []byte) (OGData, error) {
	if len(raw) == 0 {
		return OGData{}, nil
	}
	var d OGData
	if err := json.Unmarshal(raw, &d); err != nil {
		return OGData{}, err
	}
	return d, nil
}
