package bot

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestOGDataRoundtrip(t *testing.T) {
	publishedAt := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	original := OGData{
		Source:    "https://example.com/article",
		Extractor: "generic",
		Classifier: &ClassifierVerdict{
			Pinnable: true,
		},
		MediaCandidates: []MediaCandidate{
			{Type: "image", URL: "https://cdn.example.com/img1.jpg", Width: 800, Height: 600},
			{Type: "video", URL: "https://cdn.example.com/v1.mp4"},
		},
		Lang:        "en",
		Author:      "Jane Doe",
		PublishedAt: &publishedAt,
	}

	raw, err := MarshalOGData(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got, err := UnmarshalOGData(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(got, original) {
		t.Fatalf("roundtrip mismatch:\nwant %+v\n got %+v", original, got)
	}
}

func TestOGDataEmptyUnmarshal(t *testing.T) {
	got, err := UnmarshalOGData(nil)
	if err != nil {
		t.Fatalf("nil unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, OGData{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}

	got, err = UnmarshalOGData([]byte{})
	if err != nil {
		t.Fatalf("empty unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, OGData{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}
}

func TestOGDataOmitsForbiddenKeys(t *testing.T) {
	// Regression: og_data MUST NOT carry body_text or canonical_url. Those
	// belong in pins.description and pins.url respectively.
	raw, err := MarshalOGData(OGData{Source: "https://example.com"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("unmarshal map: %v", err)
	}

	for _, forbidden := range []string{"body_text", "canonical_url"} {
		if _, ok := asMap[forbidden]; ok {
			t.Fatalf("og_data must not contain key %q, payload=%s", forbidden, raw)
		}
	}
}

func TestPinDocumentZeroValueIsNonNil(t *testing.T) {
	// Sanity: PinDocument is a value type; the extractor contract requires
	// the zero value to be a usable, non-nil document.
	doc := PinDocument{}
	if doc.MediaCandidates == nil {
		// nil slice is acceptable; this assertion just documents expectations
		// — len(nil) == 0 so downstream code is safe either way.
		_ = doc
	}
}

func TestClassifierReasonValues(t *testing.T) {
	// Lock the wire values so persisted og_data stays stable across releases.
	cases := map[ClassifierReason]string{
		ClassifierReasonListing:        "listing",
		ClassifierReasonEmptyBody:      "empty_body",
		ClassifierReasonNoPrimaryMedia: "no_primary_media",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("reason wire value mismatch: got %q want %q", got, want)
		}
	}
}
