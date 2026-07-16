package pin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/interaction"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

const maxVideoDurationSeconds = 15
const maxBytes int64 = 100 << 20 // 100MB server-side video size limit

// requestBodyCap is the per-request body upper bound for POST /api/pins.
// spec: pin `서버가 본문을 디스크에 스풀하기 전에 본문 상한으로 거절한다` — the
// multipart body is wrapped with http.MaxBytesReader before ParseMultipartForm
// so abusive bodies are rejected before any bytes spool to disk.
// Declared as var (not const) so unit tests can lower the cap to a small value
// that exercises the rejection path without allocating cap-sized buffers;
// production code never mutates this value.
var requestBodyCap int64 = 500 << 20

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
	FallbackRelatedByMediaType(ctx context.Context, arg db.FallbackRelatedByMediaTypeParams) ([]db.FallbackRelatedByMediaTypeRow, error)
	FallbackRelatedLatest(ctx context.Context, arg db.FallbackRelatedLatestParams) ([]db.FallbackRelatedLatestRow, error)
	GetTagsByIDs(ctx context.Context, ids []uuid.UUID) ([]db.Tag, error)
	CreateInteraction(ctx context.Context, arg db.CreateInteractionParams) error
}

// mediaStore is the minimal storage surface Create needs; *storage.Client
// satisfies it. Narrowing the type lets tests inject a fake to verify the
// compensating-delete contract without a live object store.
type mediaStore interface {
	Upload(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (*storage.UploadResult, error)
	Delete(ctx context.Context, key string) error
}

type Handler struct {
	q     PinQuerier
	store mediaStore
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

	// Parse multipart: max 500MB (video originals before server-side trim).
	// spec: pin `서버가 본문을 디스크에 스풀하기 전에 본문 상한으로 거절한다` — wrap r.Body
	// with MaxBytesReader so the multipart parser cannot spool unlimited bytes to disk
	// when client validation is bypassed.
	r.Body = http.MaxBytesReader(w, r.Body, requestBodyCap)
	if err := r.ParseMultipartForm(requestBodyCap); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusBadRequest, "파일 크기가 제한을 초과했습니다")
			return
		}
		writeError(w, http.StatusBadRequest, "잘못된 요청 형식입니다")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		writeError(w, http.StatusBadRequest, "제목은 필수입니다")
		return
	}
	if utf8.RuneCountInString(title) > 200 {
		writeError(w, http.StatusBadRequest, "제목은 200자 이내여야 합니다")
		return
	}

	// Parse tag IDs (optional, max 10)
	tagIDStrs := r.Form["tag_ids"]
	if len(tagIDStrs) == 0 {
		// Also try comma-separated
		if csv := r.FormValue("tag_ids"); csv != "" {
			tagIDStrs = strings.Split(csv, ",")
		}
	}
	if len(tagIDStrs) > 10 {
		writeError(w, http.StatusBadRequest, "태그는 최대 10개까지 가능합니다")
		return
	}

	var tagIDs []uuid.UUID
	for _, s := range tagIDStrs {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			writeError(w, http.StatusBadRequest, "유효하지 않은 태그 ID입니다: "+s)
			return
		}
		tagIDs = append(tagIDs, id)
	}

	// Validate tag IDs exist (skip if no tags provided)
	if len(tagIDs) > 0 {
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
	}

	// Optional field validation happens before any storage write so a 4xx
	// rejection never creates an orphan object (spec: pin `핀 생성 실패 시
	// 업로드된 미디어를 보상 삭제한다` — 검증 실패 시 객체 미생성).
	description := sql.NullString{}
	if d := strings.TrimSpace(r.FormValue("description")); d != "" {
		if utf8.RuneCountInString(d) > 500 {
			writeError(w, http.StatusBadRequest, "설명은 500자 이내여야 합니다")
			return
		}
		description = sql.NullString{String: d, Valid: true}
	}

	urlField := sql.NullString{}
	if u := strings.TrimSpace(r.FormValue("url")); u != "" {
		if utf8.RuneCountInString(u) > 1000 {
			writeError(w, http.StatusBadRequest, "URL은 1000자 이내여야 합니다")
			return
		}
		urlField = sql.NullString{String: u, Valid: true}
	}

	ogImage := sql.NullString{}
	if o := strings.TrimSpace(r.FormValue("og_image")); o != "" {
		if utf8.RuneCountInString(o) > 1000 {
			writeError(w, http.StatusBadRequest, "og_image URL은 1000자 이내여야 합니다")
			return
		}
		ogImage = sql.NullString{String: o, Valid: true}
	}

	// Media file (required)
	file, header, err := r.FormFile("media")
	if err != nil {
		writeError(w, http.StatusBadRequest, "미디어 파일은 필수입니다")
		return
	}
	defer func() { _ = file.Close() }()

	contentType := header.Header.Get("Content-Type")

	// Server-side video processing: trim if requested, reject untrimmed > 15s
	var uploadBody io.Reader = file
	uploadSize := header.Size
	if strings.HasPrefix(contentType, "video/") {
		tmpFile, err := os.CreateTemp("", "fugue-video-*.tmp")
		if err != nil {
			log.Printf("pin.Create: temp file error: %v", err)
			writeError(w, http.StatusInternalServerError, "파일 처리에 실패했습니다")
			return
		}
		origTmpPath := tmpFile.Name()
		defer func() { _ = os.Remove(origTmpPath) }()

		if _, err := io.Copy(tmpFile, file); err != nil {
			_ = tmpFile.Close()
			log.Printf("pin.Create: temp copy error: %v", err)
			writeError(w, http.StatusInternalServerError, "파일 처리에 실패했습니다")
			return
		}
		// spec: pin `구간 정보 없이 비디오 길이를 확인할 수 없는 경우` 도 거부 SHALL 을
		// 명시한다 (openspec/specs/pin/spec.md L205-207). *os.File.Close() 는 io.Copy
		// 성공 후에도 ENOSPC/EIO/NFS write-back error 등으로 실패할 수 있고, 그 경우
		// buffered data 가 디스크에 flush 되지 않았음을 의미한다. 이어지는
		// probeDuration(origTmpPath) 가 부분 파일을 ffprobe 로 측정하면 16s 실제 비디오가
		// 12-14s 로 보고되어 "15초 초과 거부" SHALL 이 침묵 우회될 수 있으므로,
		// L247-251 의 probeErr fail-closed 와 동일하게 Close 실패도 "결정적으로 길이를
		// 확인할 수 없는 경우" 로 보고 400 으로 거부한다.
		if err := tmpFile.Close(); err != nil {
			log.Printf("pin.Create: tmpfile close error: %v (path=%s)", err, origTmpPath)
			writeError(w, http.StatusBadRequest, "비디오 길이를 확인할 수 없습니다")
			return
		}

		uploadPath := origTmpPath

		// Parse trim parameters from client
		trimStartStr := strings.TrimSpace(r.FormValue("trim_start"))
		trimEndStr := strings.TrimSpace(r.FormValue("trim_end"))

		if trimStartStr != "" && trimEndStr != "" {
			// Client specified trim range
			trimStart, err1 := strconv.ParseFloat(trimStartStr, 64)
			trimEnd, err2 := strconv.ParseFloat(trimEndStr, 64)
			if err1 != nil || err2 != nil {
				writeError(w, http.StatusBadRequest, "유효하지 않은 트리밍 값입니다")
				return
			}

			// Validate trim values
			duration, _ := probeDuration(r.Context(), origTmpPath)
			trimDuration := trimEnd - trimStart
			if trimStart < 0 || trimStart >= trimEnd || trimDuration > float64(maxVideoDurationSeconds)+0.5 {
				writeError(w, http.StatusBadRequest, "유효하지 않은 트리밍 구간입니다")
				return
			}
			if duration > 0 && trimEnd > duration+1.0 {
				writeError(w, http.StatusBadRequest, "트리밍 구간이 비디오 길이를 초과합니다")
				return
			}

			// Try -c copy first (fast), fall back to re-encode for non-MP4 (e.g. WebM)
			trimCopyPath := origTmpPath + ".trimmed.mp4"
			defer func() { _ = os.Remove(trimCopyPath) }()
			trimmedPath := trimCopyPath

			copyErr := trimVideoRange(r.Context(), origTmpPath, trimmedPath, trimStart, trimDuration)
			needsReencode := copyErr != nil // -c copy failed (e.g. WebM → MP4 remux)
			if copyErr != nil {
				log.Printf("pin.Create: -c copy trim failed (will re-encode): %v", copyErr)
			}

			// Also check duration accuracy and size if copy succeeded
			if !needsReencode {
				resultDur, probeErr := probeDuration(r.Context(), trimmedPath)
				fi, statErr := os.Stat(trimmedPath)
				if probeErr == nil && (resultDur-trimDuration > 2.0 || resultDur-trimDuration < -2.0) {
					needsReencode = true
					log.Printf("pin.Create: trim duration mismatch: requested %.1fs, got %.1fs, re-encoding", trimDuration, resultDur)
				}
				if statErr == nil && fi.Size() > maxBytes {
					needsReencode = true
					log.Printf("pin.Create: trimmed file too large (%d bytes), re-encoding", fi.Size())
				}
			}

			if needsReencode {
				reencodedPath := origTmpPath + ".reencoded.mp4"
				defer func() { _ = os.Remove(reencodedPath) }()
				if err := reencodeVideoRange(r.Context(), origTmpPath, reencodedPath, trimStart, trimDuration); err != nil {
					log.Printf("pin.Create: ffmpeg re-encode error: %v", err)
					writeError(w, http.StatusInternalServerError, "비디오 처리에 실패했습니다")
					return
				}
				trimmedPath = reencodedPath
			}

			uploadPath = trimmedPath
			contentType = "video/mp4"

			fi2, err := os.Stat(trimmedPath)
			if err != nil {
				log.Printf("pin.Create: stat trimmed file error: %v", err)
				writeError(w, http.StatusInternalServerError, "파일 처리에 실패했습니다")
				return
			}
			uploadSize = fi2.Size()
			log.Printf("pin.Create: trimmed video %.1fs-%.1fs, size %d -> %d bytes",
				trimStart, trimEnd, header.Size, uploadSize)
		} else {
			// No trim params: reject if > 15s (server defense).
			// spec: pin `구간 정보 없이 15초 초과 비디오 업로드` 는 거부 SHALL을 명시하며
			// `구간 정보 없이 비디오 길이를 확인할 수 없는 경우` 도 거부 SHALL을 명시한다 —
			// probe 실패 시 fail-closed 로 처리해 15초 초과 거부 SHALL이 길이 측정 실패라는
			// 우연으로 침묵 우회되지 않게 한다.
			duration, probeErr := probeDuration(r.Context(), origTmpPath)
			if probeErr != nil {
				log.Printf("pin.Create: probe duration failed: %v", probeErr)
				writeError(w, http.StatusBadRequest, "비디오 길이를 확인할 수 없습니다")
				return
			}
			if duration > float64(maxVideoDurationSeconds) {
				writeError(w, http.StatusBadRequest, "15초 초과 비디오는 트리밍이 필요합니다")
				return
			}
		}

		// Re-open the (possibly trimmed) file for storage upload
		reopened, err := os.Open(uploadPath)
		if err != nil {
			log.Printf("pin.Create: reopen temp file error: %v", err)
			writeError(w, http.StatusInternalServerError, "파일 처리에 실패했습니다")
			return
		}
		defer func() { _ = reopened.Close() }()
		uploadBody = reopened
	}

	result, err := h.store.Upload(r.Context(), header.Filename, contentType, uploadSize, uploadBody)
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

	// Track every object written to storage in this request so failure paths
	// below can compensate-delete them instead of leaving orphans.
	uploadedKeys := []string{result.Key}

	// Upload video thumbnail if provided
	if thumbFile, thumbHeader, err := r.FormFile("thumbnail"); err == nil {
		defer func() { _ = thumbFile.Close() }()
		thumbResult, err := h.store.Upload(r.Context(), thumbHeader.Filename, thumbHeader.Header.Get("Content-Type"), thumbHeader.Size, thumbFile)
		if err != nil {
			log.Printf("pin.Create: thumbnail upload warning: %v", err)
		} else {
			ogImage = sql.NullString{String: thumbResult.URL, Valid: true}
			uploadedKeys = append(uploadedKeys, thumbResult.Key)
		}
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
		h.deleteUploadedMedia(r.Context(), uploadedKeys)
		writeError(w, http.StatusInternalServerError, "핀 등록에 실패했습니다")
		return
	}

	// Link tags — all must succeed
	for _, tagID := range tagIDs {
		if err := h.q.LinkPinTag(r.Context(), db.LinkPinTagParams{PinID: p.ID, TagID: tagID}); err != nil {
			log.Printf("pin.Create: LinkPinTag error: %v (pin=%s tag=%s)", err, p.ID, tagID)
			// Rollback: delete the orphan pin. log both DB error and rowsAffected==0
			// so an orphan that survives this cleanup leaves an operator-visible trail
			// (otherwise pins.creator_id rows + S3 media linger silently).
			// context.WithoutCancel: when the LinkPinTag failure was caused by
			// client cancellation, r.Context() is already canceled and would make
			// both rollbacks fail in cascade.
			rollbackCtx := context.WithoutCancel(r.Context())
			if rowsAffected, delErr := h.q.DeletePin(rollbackCtx, db.DeletePinParams{ID: p.ID, CreatorID: creatorID}); delErr != nil {
				log.Printf("pin.Create: rollback DeletePin error: %v (pin=%s creator=%s)", delErr, p.ID, creatorID)
			} else if rowsAffected == 0 {
				log.Printf("pin.Create: rollback DeletePin matched 0 rows (pin=%s creator=%s)", p.ID, creatorID)
			}
			h.deleteUploadedMedia(rollbackCtx, uploadedKeys)
			writeError(w, http.StatusInternalServerError, "태그 연결에 실패했습니다")
			return
		}
	}

	// spec: interaction `인증된 호출자의 핀 조회·핀 생성·보드 추가에 interaction row가 piggyback된다`
	interaction.Record(r.Context(), h.q, creatorID, p.ID, "pin")

	writeJSON(w, http.StatusCreated, toCreatedResponse(p))
}

// deleteUploadedMedia best-effort deletes objects uploaded during a failed
// Create so they don't linger as orphans (spec: pin `핀 생성 실패 시 업로드된
// 미디어를 보상 삭제한다`). Delete failures must not change the user-facing
// error response; they are logged with the object key so operators can
// identify surviving orphans (spec: pin `보상 삭제 실패는 요청 결과와
// 독립적으로 기록된다`). context.WithoutCancel: the failure that triggered
// compensation may itself be a client cancellation, which would otherwise
// cancel the compensating deletes too.
func (h *Handler) deleteUploadedMedia(ctx context.Context, keys []string) {
	ctx = context.WithoutCancel(ctx)
	for _, key := range keys {
		if err := h.store.Delete(ctx, key); err != nil {
			log.Printf("pin.Create: rollback storage delete error: %v (key=%s)", err, key)
		}
	}
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

	// spec: interaction `미인증 호출자의 핀 조회에는 interaction이 기록되지 않는다`
	if creatorID, ok := auth.CreatorIDFromContext(r.Context()); ok {
		interaction.Record(r.Context(), h.q, creatorID, id, "view")
	}

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

	const maxRelated = 10
	pins := make([]PinResponse, 0, maxRelated)
	excludeIDs := []uuid.UUID{id} // always exclude self

	// Stage 1: tag-based matching (skip if no tags)
	pinTags := h.loadPinTags(r.Context(), id)
	if len(pinTags) > 0 {
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
			log.Printf("pin.Related: tag query error: %v (id=%s)", err, idStr)
			writeError(w, http.StatusInternalServerError, "연관 핀을 불러올 수 없습니다")
			return
		}
		for _, r := range related {
			pins = append(pins, toRelatedPinResponse(r))
			excludeIDs = append(excludeIDs, r.ID)
		}
	}

	// Stage 2: same media type fallback
	if len(pins) < maxRelated {
		remaining := int32(maxRelated - len(pins))
		fbRows, err := h.q.FallbackRelatedByMediaType(r.Context(), db.FallbackRelatedByMediaTypeParams{
			MediaType: row.MediaType,
			Column2:   excludeIDs,
			Limit:     remaining,
		})
		if err != nil {
			log.Printf("pin.Related: media type fallback error: %v (id=%s)", err, idStr)
		} else {
			for _, r := range fbRows {
				pins = append(pins, toFallbackMediaTypePinResponse(r))
				excludeIDs = append(excludeIDs, r.ID)
			}
		}
	}

	// Stage 3: latest pins fallback
	if len(pins) < maxRelated {
		remaining := int32(maxRelated - len(pins))
		fbRows, err := h.q.FallbackRelatedLatest(r.Context(), db.FallbackRelatedLatestParams{
			Column1: excludeIDs,
			Limit:   remaining,
		})
		if err != nil {
			log.Printf("pin.Related: latest fallback error: %v (id=%s)", err, idStr)
		} else {
			for _, r := range fbRows {
				pins = append(pins, toFallbackLatestPinResponse(r))
			}
		}
	}

	h.hydrateListTags(r.Context(), pins)
	writeJSON(w, http.StatusOK, map[string]any{"pins": pins})
}

// maxListTagIDs caps `?tag_ids=` count on /api/pins (unauthenticated,
// not rate-limited). Mirrors the sibling search.Search cap at
// internal/search/handler.go:115 (same GET parameter name, same CSV
// split, same filter semantic) — without this guard a single request
// can pass an unbounded UUID array into ListPinsWithCreator +
// CountPins (or ListPinsByCreator + CountPinsByCreatorFiltered on the
// creator_id path), and the Count query has no LIMIT/OFFSET so the
// `tag_id = ANY($N::uuid[])` EXISTS subquery is evaluated against
// every row in `pins`.
const maxListTagIDs = 5

// List handles GET /api/pins?media_type=&tag_ids=&limit=&offset=&creator_id=
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	mediaType := r.URL.Query().Get("media_type")

	var tagIDs []uuid.UUID
	if tagsParam := r.URL.Query().Get("tag_ids"); tagsParam != "" {
		parts := strings.Split(tagsParam, ",")
		if len(parts) > maxListTagIDs {
			writeError(w, http.StatusBadRequest, "태그 필터는 최대 5개까지 가능합니다")
			return
		}
		for _, s := range parts {
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

// trimVideoRange trims a video using stream copy (fast, keyframe boundary).
//
// ctx 는 호출 핸들러의 r.Context() 를 받아 exec.CommandContext 로 ffmpeg
// 자식 프로세스에 묶는다. 클라이언트 disconnect / SIGTERM 등으로 ctx 가
// cancel 되면 ffmpeg 가 즉시 SIGKILL 되어 http.Server.Shutdown 의 25s 드레인
// 예산 안에 핸들러 goroutine 이 cmd.CombinedOutput 에서 unblock 된다.
// Sister convention: bot/media_validator.go:284 probeDuration 도 동일하게
// exec.CommandContext(probeCtx, ...) 사용. 참고 PR #353 (GoogleProvider
// FetchProfile ctx 전파, cycle 120).
func trimVideoRange(ctx context.Context, inputPath, outputPath string, start, duration float64) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", fmt.Sprintf("%.3f", start),
		"-i", inputPath,
		"-t", fmt.Sprintf("%.3f", duration),
		"-c", "copy",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg copy: %w: %s", err, string(out))
	}
	return nil
}

// reencodeVideoRange trims + re-encodes (frame-accurate, smaller output).
// ctx 는 trimVideoRange 와 동일한 이유로 ffmpeg 자식 프로세스에 묶는다.
func reencodeVideoRange(ctx context.Context, inputPath, outputPath string, start, duration float64) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.3f", start),
		"-t", fmt.Sprintf("%.3f", duration),
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", "fast",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg reencode: %w: %s", err, string(out))
	}
	return nil
}

// probeDuration runs ffprobe to extract video duration in seconds.
// ctx 는 trimVideoRange 와 동일한 이유로 ffprobe 자식 프로세스에 묶는다.
func probeDuration(ctx context.Context, path string) (float64, error) {
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}
