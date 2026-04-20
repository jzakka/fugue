package bot

import "strings"

// Classifier defaults. The Harvester wires these to env-configurable values
// in production but tests construct Classifier values directly.
const (
	DefaultBodyTextMinBytes     = 200
	DefaultLinkDensityThreshold = 0.5 // links per word
)

// Classifier decides whether a PinDocument is worth persisting as a Pin and,
// if not, why. It depends only on the contents of PinDocument — never on
// node_type or any external state.
type Classifier struct {
	BodyTextMinBytes     int
	LinkDensityThreshold float64
}

// NewClassifier returns a Classifier with the default thresholds.
func NewClassifier() *Classifier {
	return &Classifier{
		BodyTextMinBytes:     DefaultBodyTextMinBytes,
		LinkDensityThreshold: DefaultLinkDensityThreshold,
	}
}

// LinkStats describes the link/word counts the classifier needs to evaluate
// the listing rule. The extractor or the Harvester is responsible for
// counting; the classifier only consumes the numbers.
//
// Why this is not on PinDocument: link/word counts are derived from the
// raw HTML, not the canonical Pin shape. Keeping them off PinDocument keeps
// the Pin contract clean.
type LinkStats struct {
	Links int
	Words int
}

// Classify returns whether the document is pinnable and, when not, the
// single canonical reason. Reason priority is `listing` > `empty_body` >
// `no_primary_media` — evaluated in that exact order, first match wins.
// When pinnable=true the reason is empty.
//
// The signature explicitly does not accept a node type — the classifier
// must depend on the document alone so it remains testable in isolation
// and behaves identically across crawler entry points.
func (c *Classifier) Classify(doc PinDocument, stats LinkStats) (bool, ClassifierReason) {
	// 1. listing: high link density (with division-by-zero guard).
	if stats.Words > 0 && float64(stats.Links)/float64(stats.Words) > c.LinkDensityThreshold {
		return false, ClassifierReasonListing
	}

	// `body_text` length is measured in bytes.
	bodyBytes := len(strings.TrimSpace(doc.BodyText))

	// 2. empty_body: body text below the byte threshold.
	if bodyBytes < c.BodyTextMinBytes {
		return false, ClassifierReasonEmptyBody
	}

	// 3. no_primary_media: no thumbnail and no media candidates. Body
	//    length is sufficient (else empty_body would have matched first).
	if doc.ThumbnailURL == "" && len(doc.MediaCandidates) == 0 {
		return false, ClassifierReasonNoPrimaryMedia
	}

	return true, ""
}
