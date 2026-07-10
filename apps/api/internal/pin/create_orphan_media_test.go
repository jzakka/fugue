package pin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	"github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

// spec: pin `핀 생성 실패 시 업로드된 미디어를 보상 삭제한다` /
// `보상 삭제 실패는 요청 결과와 독립적으로 기록된다` (change
// fix-pin-create-orphan-media, NAV-1247). 검증 실패(4xx)는 저장소 쓰기 자체가
// 없어야 하고, 업로드 이후 실패(핀 insert·태그 연결)는 업로드된 객체(썸네일
// 포함)를 보상 삭제해야 하며, 보상 삭제 실패는 사용자 응답을 바꾸지 않아야
// 한다.

// PNG magic bytes so a real *storage.Client would sniff image/png; the fake
// store below doesn't sniff, but keeping realistic bytes documents intent.
var testPNGBytes = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}

// fakeStore implements mediaStore, recording uploaded and deleted keys.
type fakeStore struct {
	uploads   []string
	deletes   []string
	deleteErr error
}

func (f *fakeStore) Upload(_ context.Context, _ string, _ string, _ int64, body io.Reader) (*storage.UploadResult, error) {
	_, _ = io.Copy(io.Discard, body)
	key := fmt.Sprintf("image/fake-%d.png", len(f.uploads))
	f.uploads = append(f.uploads, key)
	return &storage.UploadResult{Key: key, URL: "http://fake.test/" + key, MediaType: storage.MediaImage}, nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.deletes = append(f.deletes, key)
	return f.deleteErr
}

type createForm struct {
	fields        map[string]string
	withMedia     bool
	withThumbnail bool
}

func buildCreateRequest(t *testing.T, form createForm) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range form.fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("WriteField(%s): %v", k, err)
		}
	}
	writeFile := func(field, filename string) {
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
		hdr.Set("Content-Type", "image/png")
		pw, err := mw.CreatePart(hdr)
		if err != nil {
			t.Fatalf("CreatePart(%s): %v", field, err)
		}
		if _, err := pw.Write(testPNGBytes); err != nil {
			t.Fatalf("write %s bytes: %v", field, err)
		}
	}
	if form.withMedia {
		writeFile("media", "a.png")
	}
	if form.withThumbnail {
		writeFile("thumbnail", "thumb.png")
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/pins", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req.WithContext(auth.WithCreatorID(req.Context(), uuid.New()))
}

// 검증 실패로 거절되는 요청은 저장소에 아무 객체도 생성하지 않아야 한다.
func TestCreate_ValidationFailureDoesNotUpload(t *testing.T) {
	cases := []struct {
		name    string
		fields  map[string]string
		wantMsg string
	}{
		{"설명 길이 초과", map[string]string{"title": "t", "description": strings.Repeat("가", 501)}, "설명은 500자 이내여야 합니다"},
		{"URL 길이 초과", map[string]string{"title": "t", "url": "https://" + strings.Repeat("a", 1000)}, "URL은 1000자 이내여야 합니다"},
		{"og_image 길이 초과", map[string]string{"title": "t", "og_image": "https://" + strings.Repeat("a", 1000)}, "og_image URL은 1000자 이내여야 합니다"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{}
			h := &Handler{q: &mockQuerier{}, store: store}

			rec := httptest.NewRecorder()
			h.Create(rec, buildCreateRequest(t, createForm{fields: tc.fields, withMedia: true}))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (body=%q)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantMsg) {
				t.Fatalf("expected %q, got: %q", tc.wantMsg, rec.Body.String())
			}
			if len(store.uploads) != 0 {
				t.Fatalf("validation failure must not upload; got uploads=%v", store.uploads)
			}
		})
	}
}

// 핀 기록 실패 시 업로드된 미디어가 보상 삭제된다.
func TestCreate_CreatePinFailureDeletesUploadedMedia(t *testing.T) {
	store := &fakeStore{}
	h := &Handler{q: &mockQuerier{createPinErr: errors.New("insert failed")}, store: store}

	rec := httptest.NewRecorder()
	h.Create(rec, buildCreateRequest(t, createForm{fields: map[string]string{"title": "t"}, withMedia: true}))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if len(store.uploads) != 1 {
		t.Fatalf("expected 1 upload, got %v", store.uploads)
	}
	assertSameKeys(t, store.uploads, store.deletes)
}

// 썸네일도 함께 업로드된 경우 미디어·썸네일 객체가 모두 보상 삭제된다.
func TestCreate_CreatePinFailureDeletesThumbnailToo(t *testing.T) {
	store := &fakeStore{}
	h := &Handler{q: &mockQuerier{createPinErr: errors.New("insert failed")}, store: store}

	rec := httptest.NewRecorder()
	h.Create(rec, buildCreateRequest(t, createForm{fields: map[string]string{"title": "t"}, withMedia: true, withThumbnail: true}))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if len(store.uploads) != 2 {
		t.Fatalf("expected 2 uploads (media+thumbnail), got %v", store.uploads)
	}
	assertSameKeys(t, store.uploads, store.deletes)
}

// 태그 연결 실패 시 핀 row 롤백과 함께 업로드된 객체(미디어·썸네일)도 모두
// 보상 삭제된다.
func TestCreate_LinkPinTagFailureDeletesUploadedMediaAndPinRow(t *testing.T) {
	tagID := uuid.New()
	store := &fakeStore{}
	q := &mockQuerier{
		linkPinTagErr: errors.New("link failed"),
		tagsByIDs:     []db.Tag{{ID: tagID, Name: "art", Slug: "art", Category: "genre"}},
		deletePinRows: 1,
	}
	h := &Handler{q: q, store: store}

	rec := httptest.NewRecorder()
	h.Create(rec, buildCreateRequest(t, createForm{
		fields:        map[string]string{"title": "t", "tag_ids": tagID.String()},
		withMedia:     true,
		withThumbnail: true,
	}))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "태그 연결에 실패했습니다") {
		t.Fatalf("expected tag-link error, got: %q", rec.Body.String())
	}
	if q.deletePinCalls != 1 {
		t.Fatalf("expected pin-row rollback (DeletePin) exactly once, got %d", q.deletePinCalls)
	}
	if len(store.uploads) != 2 {
		t.Fatalf("expected 2 uploads (media+thumbnail), got %v", store.uploads)
	}
	assertSameKeys(t, store.uploads, store.deletes)
}

// 보상 삭제 실패는 사용자 응답을 바꾸지 않는다 (기록만 남는다).
func TestCreate_CompensatingDeleteFailureKeepsOriginalResponse(t *testing.T) {
	store := &fakeStore{deleteErr: errors.New("storage down")}
	h := &Handler{q: &mockQuerier{createPinErr: errors.New("insert failed")}, store: store}

	rec := httptest.NewRecorder()
	h.Create(rec, buildCreateRequest(t, createForm{fields: map[string]string{"title": "t"}, withMedia: true}))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected original 500 despite delete failure, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "핀 등록에 실패했습니다") {
		t.Fatalf("expected original insert-failure message, got: %q", rec.Body.String())
	}
	if len(store.deletes) != 1 {
		t.Fatalf("expected delete to be attempted once, got %v", store.deletes)
	}
}

func assertSameKeys(t *testing.T, uploads, deletes []string) {
	t.Helper()
	deleted := make(map[string]bool, len(deletes))
	for _, k := range deletes {
		deleted[k] = true
	}
	for _, k := range uploads {
		if !deleted[k] {
			t.Errorf("uploaded key %q was not compensating-deleted (deletes=%v)", k, deletes)
		}
	}
}
