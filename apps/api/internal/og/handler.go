package og

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"unicode/utf8"
)

// ogRequestBodyCap caps the JSON request body for POST /api/og/fetch.
// The body schema is `{"url":"..."}` where url is bounded above by
// maxOGURLRunes (500). 500 runes is at most 4 bytes/rune × 500 = 2KB,
// plus a small JSON envelope, so 8KB is a generous upper bound that
// still cuts the unbounded surface. Sister convention is
// pin/handler.go:82 (`r.Body = http.MaxBytesReader(w, r.Body,
// requestBodyCap=500<<20)`) which uses 500MB because pin uploads
// carry video originals. The done item
// `system-20260515-pin-create-no-request-body-cap` explicitly flagged
// `/api/og/fetch` as a small-body surface needing its own (smaller)
// cap as a follow-up; this is that follow-up.
const ogRequestBodyCap = 8 * 1024

// maxOGURLRunes caps the request URL length. It mirrors the
// `pins.media_url` VARCHAR(500) schema bound (migration
// 000004_pivot_pins.up.sql) and the bot URL-cap convention
// (`system-20260521-bot-process-document-media-url-no-length-cap`,
// done). Without this cap, a pathological URL flows into
// `log.Printf %q` (log bloat), `http.NewRequestWithContext` (oversize
// outbound request line), and `Service.Fetch` parsing — all
// unbounded in length cost.
const maxOGURLRunes = 500

// ogFetchErrorMessage is the generic, non-revealing message returned to the
// client when an OG fetch fails. The raw error (which can include the resolved
// internal IP from the SSRF guard, e.g. "blocked private/reserved IP 10.x.x.x",
// or upstream HTTP details) is logged server-side only — surfacing it to the
// unauthenticated /api/og/fetch caller would turn the SSRF block into an
// internal-network reconnaissance oracle (CWE-209).
const ogFetchErrorMessage = "메타데이터를 가져오지 못했습니다"

// Handler handles OG metadata fetch HTTP requests.
type Handler struct {
	svc *Service
}

// NewHandler creates a new OG handler backed by the given service.
func NewHandler() *Handler {
	return &Handler{svc: NewService()}
}

// fetchRequest is the expected JSON body for POST /api/og/fetch.
type fetchRequest struct {
	URL string `json:"url"`
}

// fetchResponse is the JSON response for a successful OG fetch.
type fetchResponse struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Image         string   `json:"image"`
	SiteName      string   `json:"site_name"`
	URL           string   `json:"url"`
	DetectedField string   `json:"detected_field"`
	SuggestedTags []string `json:"suggested_tags"`
	Error         string   `json:"error,omitempty"`
}

// Fetch handles POST /api/og/fetch.
// It validates the incoming URL, fetches OG metadata, and returns the result.
// On partial failure it returns whatever data was collected along with an error message.
func (h *Handler) Fetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST 메서드만 허용됩니다")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, ogRequestBodyCap)
	var req fetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusBadRequest, "요청 본문이 너무 큽니다")
			return
		}
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "URL이 필요합니다")
		return
	}

	if utf8.RuneCountInString(req.URL) > maxOGURLRunes {
		writeError(w, http.StatusBadRequest, "URL은 500자 이내여야 합니다")
		return
	}

	// Validate URL format: must be a valid http or https URL.
	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Host == "" {
		writeError(w, http.StatusBadRequest, "유효하지 않은 URL입니다")
		return
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		writeError(w, http.StatusBadRequest, "http 또는 https URL만 허용됩니다")
		return
	}

	result, err := h.svc.Fetch(r.Context(), req.URL)
	if err != nil {
		log.Printf("og.Fetch: error fetching %q: %v", req.URL, err)

		// If we got a partial result, return it with the error message.
		if result != nil {
			writeJSON(w, http.StatusOK, fetchResponse{
				Title:         result.Title,
				Description:   result.Description,
				Image:         result.Image,
				SiteName:      result.SiteName,
				URL:           result.URL,
				DetectedField: result.DetectedField,
				SuggestedTags: SuggestTags(result.Title, result.Description, 5),
				Error:         ogFetchErrorMessage,
			})
			return
		}

		writeJSON(w, http.StatusOK, fetchResponse{
			URL:           req.URL,
			DetectedField: detectField(parsed.Hostname()),
			SuggestedTags: []string{},
			Error:         ogFetchErrorMessage,
		})
		return
	}

	writeJSON(w, http.StatusOK, fetchResponse{
		Title:         result.Title,
		Description:   result.Description,
		Image:         result.Image,
		SiteName:      result.SiteName,
		URL:           result.URL,
		DetectedField: result.DetectedField,
		SuggestedTags: SuggestTags(result.Title, result.Description, 5),
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("og: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
