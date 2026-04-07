package pin

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

type PinQuerier interface {
	ListPinsWithCreator(ctx context.Context, arg db.ListPinsWithCreatorParams) ([]db.ListPinsWithCreatorRow, error)
	ListPinsByCreator(ctx context.Context, arg db.ListPinsByCreatorParams) ([]db.ListPinsByCreatorRow, error)
	CountPins(ctx context.Context, arg db.CountPinsParams) (int64, error)
	CountPinsByCreatorFiltered(ctx context.Context, arg db.CountPinsByCreatorFilteredParams) (int64, error)
	CreatePin(ctx context.Context, arg db.CreatePinParams) (db.Pin, error)
	LinkPinTag(ctx context.Context, arg db.LinkPinTagParams) error
	DeletePin(ctx context.Context, arg db.DeletePinParams) (int64, error)
	GetPinWithCreator(ctx context.Context, id uuid.UUID) (db.GetPinWithCreatorRow, error)
	GetPinTags(ctx context.Context, pinID uuid.UUID) ([]db.GetPinTagsRow, error)
	GetTagsForPins(ctx context.Context, pinIDs []uuid.UUID) ([]db.GetTagsForPinsRow, error)
	RelatedPins(ctx context.Context, arg db.RelatedPinsParams) ([]db.RelatedPinsRow, error)
	GetTagsByIDs(ctx context.Context, ids []uuid.UUID) ([]db.Tag, error)
}

type Handler struct {
	q     PinQuerier
	store *storage.Client
}

func NewHandler(database *sql.DB, store *storage.Client) *Handler {
	return &Handler{q: db.New(database), store: store}
}

func NewHandlerWithQuerier(q PinQuerier) *Handler {
	return &Handler{q: q}
}

// Create handles POST /api/pins [auth] — multipart/form-data
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse multipart: max 110MB (video limit + overhead)
	if err := r.ParseMultipartForm(110 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		writeError(w, http.StatusBadRequest, "제목은 필수입니다")
		return
	}

	// Parse tag IDs
	tagIDStrs := r.Form["tag_ids"]
	if len(tagIDStrs) == 0 {
		// Also try comma-separated
		if csv := r.FormValue("tag_ids"); csv != "" {
			tagIDStrs = strings.Split(csv, ",")
		}
	}
	if len(tagIDStrs) == 0 {
		writeError(w, http.StatusBadRequest, "태그는 1개 이상 선택해야 합니다")
		return
	}
	if len(tagIDStrs) > 10 {
		writeError(w, http.StatusBadRequest, "태그는 최대 10개까지 가능합니다")
		return
	}

	tagIDs := make([]uuid.UUID, 0, len(tagIDStrs))
	for _, s := range tagIDStrs {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			writeError(w, http.StatusBadRequest, "유효하지 않은 태그 ID입니다: "+s)
			return
		}
		tagIDs = append(tagIDs, id)
	}

	// Validate tag IDs exist
	existingTags, err := h.q.GetTagsByIDs(r.Context(), tagIDs)
	if err != nil {
		log.Printf("pin.Create: GetTagsByIDs error: %v", err)
		writeError(w, http.StatusInternalServerError, "태그를 확인할 수 없습니다")
		return
	}
	if len(existingTags) != len(tagIDs) {
		writeError(w, http.StatusBadRequest, "존재하지 않는 태그가 포함되어 있습니다")
		return
	}

	// Media file (required)
	file, header, err := r.FormFile("media")
	if err != nil {
		writeError(w, http.StatusBadRequest, "미디어 파일은 필수입니다")
		return
	}
	defer file.Close()

	result, err := h.store.Upload(r.Context(), header.Filename, header.Header.Get("Content-Type"), header.Size, file)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported file type") {
			writeError(w, http.StatusBadRequest, "지원하지 않는 파일 형식입니다")
			return
		}
		if strings.Contains(err.Error(), "file too large") {
			writeError(w, http.StatusBadRequest, "파일 크기가 제한을 초과했습니다")
			return
		}
		log.Printf("pin.Create: upload error: %v", err)
		writeError(w, http.StatusInternalServerError, "파일 업로드에 실패했습니다")
		return
	}

	// Optional fields
	description := sql.NullString{}
	if d := strings.TrimSpace(r.FormValue("description")); d != "" {
		description = sql.NullString{String: d, Valid: true}
	}

	urlField := sql.NullString{}
	if u := strings.TrimSpace(r.FormValue("url")); u != "" {
		urlField = sql.NullString{String: u, Valid: true}
	}

	ogImage := sql.NullString{}
	if o := strings.TrimSpace(r.FormValue("og_image")); o != "" {
		ogImage = sql.NullString{String: o, Valid: true}
	}

	p, err := h.q.CreatePin(r.Context(), db.CreatePinParams{
		CreatorID:   creatorID,
		MediaUrl:    result.URL,
		MediaType:   string(result.MediaType),
		Url:         urlField,
		Title:       title,
		Description: description,
		OgImage:     ogImage,
	})
	if err != nil {
		log.Printf("pin.Create: insert error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀 등록에 실패했습니다")
		return
	}

	// Link tags — all must succeed
	for _, tagID := range tagIDs {
		if err := h.q.LinkPinTag(r.Context(), db.LinkPinTagParams{PinID: p.ID, TagID: tagID}); err != nil {
			log.Printf("pin.Create: LinkPinTag error: %v (pin=%s tag=%s)", err, p.ID, tagID)
			// Rollback: delete the orphan pin
			_, _ = h.q.DeletePin(r.Context(), db.DeletePinParams{ID: p.ID, CreatorID: creatorID})
			writeError(w, http.StatusInternalServerError, "태그 연결에 실패했습니다")
			return
		}
	}

	writeJSON(w, http.StatusCreated, toCreatedResponse(p))
}

// GetByID handles GET /api/pins/{id}
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 핀 ID입니다")
		return
	}

	row, err := h.q.GetPinWithCreator(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "핀을 찾을 수 없습니다")
			return
		}
		log.Printf("pin.GetByID: query error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "핀을 불러올 수 없습니다")
		return
	}

	resp := toPinDetailResponse(row)
	resp.Tags = h.loadPinTags(r.Context(), id)

	writeJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /api/pins/{id} [auth]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := auth.CreatorIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 핀 ID입니다")
		return
	}

	rowsAffected, err := h.q.DeletePin(r.Context(), db.DeletePinParams{
		ID:        id,
		CreatorID: creatorID,
	})
	if err != nil {
		log.Printf("pin.Delete: delete error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "핀 삭제에 실패했습니다")
		return
	}
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "핀을 찾을 수 없습니다")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Related handles GET /api/pins/{id}/related
func (h *Handler) Related(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "유효하지 않은 핀 ID입니다")
		return
	}

	row, err := h.q.GetPinWithCreator(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "핀을 찾을 수 없습니다")
			return
		}
		log.Printf("pin.Related: get pin error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "핀을 불러올 수 없습니다")
		return
	}

	// Get this pin's tag IDs for similarity matching
	pinTags := h.loadPinTags(r.Context(), id)
	if len(pinTags) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"pins": []any{}})
		return
	}

	tagIDs := make([]uuid.UUID, len(pinTags))
	for i, t := range pinTags {
		tagIDs[i] = uuid.MustParse(t.ID)
	}

	related, err := h.q.RelatedPins(r.Context(), db.RelatedPinsParams{
		ID:        id,
		Column2:   tagIDs,
		MediaType: row.MediaType,
	})
	if err != nil {
		log.Printf("pin.Related: query error: %v (id=%s)", err, idStr)
		writeError(w, http.StatusInternalServerError, "연관 핀을 불러올 수 없습니다")
		return
	}

	pins := make([]PinResponse, 0, len(related))
	for _, r := range related {
		pins = append(pins, toRelatedPinResponse(r))
	}
	h.hydrateListTags(r.Context(), pins)

	writeJSON(w, http.StatusOK, map[string]any{"pins": pins})
}

// List handles GET /api/pins?media_type=&tag_ids=&limit=&offset=&creator_id=
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	mediaType := r.URL.Query().Get("media_type")

	var tagIDs []uuid.UUID
	if tagsParam := r.URL.Query().Get("tag_ids"); tagsParam != "" {
		for _, s := range strings.Split(tagsParam, ",") {
			id, err := uuid.Parse(strings.TrimSpace(s))
			if err != nil {
				continue
			}
			tagIDs = append(tagIDs, id)
		}
	}

	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		if l > 0 && l <= 50 {
			limit = l
		} else if l > 50 {
			limit = 50
		}
	}

	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o > 0 && o <= 100000 {
		offset = o
	}

	if creatorIDStr := r.URL.Query().Get("creator_id"); creatorIDStr != "" {
		creatorID, err := uuid.Parse(creatorIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "유효하지 않은 크리에이터 ID입니다")
			return
		}
		h.listByCreator(w, r, creatorID, mediaType, tagIDs, limit, offset)
		return
	}

	rows, err := h.q.ListPinsWithCreator(r.Context(), db.ListPinsWithCreatorParams{
		Column1: mediaType,
		Column2: tagIDs,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		log.Printf("pin.List: query error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀 목록을 불러올 수 없습니다")
		return
	}

	count, err := h.q.CountPins(r.Context(), db.CountPinsParams{
		Column1: mediaType,
		Column2: tagIDs,
	})
	if err != nil {
		log.Printf("pin.List: count error: %v", err)
		writeError(w, http.StatusInternalServerError, "핀 수를 확인할 수 없습니다")
		return
	}

	pins := make([]PinResponse, 0, len(rows))
	for _, row := range rows {
		pins = append(pins, toPinResponse(row))
	}
	h.hydrateListTags(r.Context(), pins)

	hasMore := (int64(offset) + int64(len(rows))) < count

	writeJSON(w, http.StatusOK, ListPinsResponse{
		Pins:    pins,
		HasMore: hasMore,
	})
}

func (h *Handler) listByCreator(w http.ResponseWriter, r *http.Request, creatorID uuid.UUID, mediaType string, tagIDs []uuid.UUID, limit, offset int) {
	rows, err := h.q.ListPinsByCreator(r.Context(), db.ListPinsByCreatorParams{
		CreatorID: creatorID,
		Column2:   mediaType,
		Column3:   tagIDs,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		log.Printf("pin.listByCreator: query error: %v (creator=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "핀 목록을 불러올 수 없습니다")
		return
	}

	count, err := h.q.CountPinsByCreatorFiltered(r.Context(), db.CountPinsByCreatorFilteredParams{
		CreatorID: creatorID,
		Column2:   mediaType,
		Column3:   tagIDs,
	})
	if err != nil {
		log.Printf("pin.listByCreator: count error: %v (creator=%s)", err, creatorID)
		writeError(w, http.StatusInternalServerError, "핀 수를 확인할 수 없습니다")
		return
	}

	pins := make([]PinResponse, 0, len(rows))
	for _, row := range rows {
		pins = append(pins, toCreatorPinResponse(row))
	}
	h.hydrateListTags(r.Context(), pins)

	hasMore := (int64(offset) + int64(len(rows))) < count

	writeJSON(w, http.StatusOK, ListPinsResponse{
		Pins:    pins,
		HasMore: hasMore,
	})
}

// hydrateListTags batch-loads tags for a slice of PinResponses.
func (h *Handler) hydrateListTags(ctx context.Context, pins []PinResponse) {
	if len(pins) == 0 {
		return
	}
	pinIDs := make([]uuid.UUID, len(pins))
	for i, p := range pins {
		pinIDs[i] = uuid.MustParse(p.ID)
	}
	rows, err := h.q.GetTagsForPins(ctx, pinIDs)
	if err != nil {
		log.Printf("pin.hydrateListTags: error: %v", err)
		return
	}
	tagMap := make(map[string][]TagResponse)
	for _, r := range rows {
		pid := r.PinID.String()
		tagMap[pid] = append(tagMap[pid], TagResponse{
			ID:       r.ID.String(),
			Name:     r.Name,
			Slug:     r.Slug,
			Category: r.Category,
		})
	}
	for i := range pins {
		if tags, ok := tagMap[pins[i].ID]; ok {
			pins[i].Tags = tags
		}
	}
}

func (h *Handler) loadPinTags(ctx context.Context, pinID uuid.UUID) []TagResponse {
	rows, err := h.q.GetPinTags(ctx, pinID)
	if err != nil {
		log.Printf("pin.loadPinTags: error: %v (pin=%s)", err, pinID)
		return []TagResponse{}
	}
	tags := make([]TagResponse, 0, len(rows))
	for _, r := range rows {
		tags = append(tags, TagResponse{
			ID:       r.ID.String(),
			Name:     r.Name,
			Slug:     r.Slug,
			Category: r.Category,
		})
	}
	return tags
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("pin: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
