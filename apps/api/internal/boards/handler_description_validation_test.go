package boards_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/chungsanghwa/fugue/apps/api/internal/auth"
	"github.com/chungsanghwa/fugue/apps/api/internal/boards"
)

// These tests verify the ADDED Requirement "보드 description 입력은
// boards.description 컬럼 cap에 맞춰 사전 길이 검증된다" of the change
// `fix-boards-handler-description-input-length-validation`. The Create
// handler runs the length validation BEFORE issuing any query, so a
// Handler built with a nil *sql.DB can be driven end-to-end for the
// reject path: when the validation fires, the 400 response is produced
// without touching the DB.
//
// Update path coverage is intentionally omitted at the unit-test layer
// because Update reads the current board via GetBoard BEFORE the
// description merge, which requires DB scaffolding outside the scope
// of this change. The Update path is exercised by the real-environment
// QA in this change's tasks (5.6–5.9).

func newAuthedRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.WithCreatorID(req.Context(), uuid.New())
	return req.WithContext(ctx)
}

func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v (raw=%q)", err, rec.Body.String())
	}
	return body["error"]
}

func TestCreate_RejectsDescriptionOverRuneCap(t *testing.T) {
	h := boards.NewHandler(nil)

	over := strings.Repeat("A", 501)
	body, err := json.Marshal(map[string]any{
		"name":        "test",
		"description": over,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := newAuthedRequest(t, http.MethodPost, "/api/boards", string(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if msg := decodeErrorBody(t, rec); msg != "보드 설명은 500자 이내여야 합니다" {
		t.Fatalf("error message: got %q, want %q", msg, "보드 설명은 500자 이내여야 합니다")
	}
}

func TestCreate_RejectsDescriptionOverRuneCapMultibyte(t *testing.T) {
	h := boards.NewHandler(nil)

	// 한국어 501 rune = ~1503 byte. byte-count로 cap을 비교하면 정상 입력
	// (가*167 = 501 byte)을 잘못 거부하거나 본 케이스를 정상으로 통과시킨다.
	// rune-count(`utf8.RuneCountInString`)만이 PostgreSQL VARCHAR(500)
	// cap 규칙과 일치한다.
	over := strings.Repeat("가", 501)
	body, err := json.Marshal(map[string]any{
		"name":        "테스트",
		"description": over,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := newAuthedRequest(t, http.MethodPost, "/api/boards", string(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
	if msg := decodeErrorBody(t, rec); msg != "보드 설명은 500자 이내여야 합니다" {
		t.Fatalf("error message: got %q, want %q", msg, "보드 설명은 500자 이내여야 합니다")
	}
}

func TestCreate_AcceptsDescriptionAtRuneCap(t *testing.T) {
	// description 500 rune은 검증을 통과해 CreateBoard 쿼리 호출로
	// 진행한다. nil *sql.DB 기반 querier라 이 호출은 panic 한다 — 그
	// panic이 발생하는 것 자체가 "검증이 description 길이로 reject 하지
	// 않았다"는 증거다. 정상 입력(cap 이하)이 reject 되지 않음을 확인하는
	// 회귀 방지 케이스.
	h := boards.NewHandler(nil)

	atCap := strings.Repeat("A", 500)
	body, err := json.Marshal(map[string]any{
		"name":        "test",
		"description": atCap,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := newAuthedRequest(t, http.MethodPost, "/api/boards", string(body))
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r == nil {
			// nil DB 경로가 panic 없이 빠져나갔다면 검증이 reject 했거나
			// 핸들러 흐름이 변경된 것. 명세상 cap=500은 정상 입력이므로
			// reject 면 회귀. 명시적으로 실패.
			if rec.Code == http.StatusBadRequest {
				t.Fatalf("cap boundary input was rejected: status=%d body=%q", rec.Code, rec.Body.String())
			}
			t.Fatalf("expected nil-DB CreateBoard call to panic, got no panic and status=%d", rec.Code)
		}
		// panic 발생 = 검증 통과 후 DB 호출 진입 = 정상 흐름. test 통과.
	}()

	h.Create(rec, req)
}

func TestCreate_AcceptsDescriptionOmitted(t *testing.T) {
	// description 미제공도 검증 분기를 건너뛰고 DB 호출로 진행해야 한다.
	// optional 필드 보존 정책 확인.
	h := boards.NewHandler(nil)

	body, err := json.Marshal(map[string]any{
		"name": "test",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := newAuthedRequest(t, http.MethodPost, "/api/boards", string(body))
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r == nil {
			if rec.Code == http.StatusBadRequest {
				t.Fatalf("description-omitted input was rejected: status=%d body=%q", rec.Code, rec.Body.String())
			}
			t.Fatalf("expected nil-DB CreateBoard call to panic, got no panic and status=%d", rec.Code)
		}
	}()

	h.Create(rec, req)
}

// Sanity: name length validation still works (adjacent regression).
// fix-boards-handler-description-input-length-validation는 description만
// 다루지만 같은 핸들러 안 name 검증과의 비대칭을 해소하는 작업이므로
// name 검증이 회귀하지 않았음을 한 케이스로 확인한다.
func TestCreate_NameValidationStillWorks(t *testing.T) {
	h := boards.NewHandler(nil)

	overName := strings.Repeat("A", 101)
	body, err := json.Marshal(map[string]any{
		"name": overName,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := newAuthedRequest(t, http.MethodPost, "/api/boards", string(body))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("name 101-rune: status got %d, want 400", rec.Code)
	}
	if msg := decodeErrorBody(t, rec); msg != "보드 이름은 100자 이내여야 합니다" {
		t.Fatalf("name 101-rune: error got %q, want %q", msg, "보드 이름은 100자 이내여야 합니다")
	}
}

// Silence unused-import warning if bytes accidentally drops out during edits.
var _ = bytes.NewReader
