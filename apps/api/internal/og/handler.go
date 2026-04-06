package og

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
)

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
	Title         string `json:"title"`
	Description   string `json:"description"`
	Image         string `json:"image"`
	SiteName      string `json:"site_name"`
	URL           string `json:"url"`
	DetectedField string `json:"detected_field"`
	Error         string `json:"error,omitempty"`
}

// Fetch handles POST /api/og/fetch.
// It validates the incoming URL, fetches OG metadata, and returns the result.
// On partial failure it returns whatever data was collected along with an error message.
func (h *Handler) Fetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST 메서드만 허용됩니다")
		return
	}

	var req fetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "URL이 필요합니다")
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
				Error:         err.Error(),
			})
			return
		}

		// No partial result — return a pure error with the URL and detected field.
		writeJSON(w, http.StatusOK, fetchResponse{
			URL:           req.URL,
			DetectedField: detectField(parsed.Hostname()),
			Error:         err.Error(),
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
