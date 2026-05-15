package interaction

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Tests cover:
//   interaction `시스템은 핀 조회·핀 생성·보드 추가 핸들러 진입 시 인증된
//   호출자에 한해 interaction 행동을 best-effort로 기록한다`
//
// Record is the piggyback path callers use. The Recorder must observe the
// expected user/pin/type, errors must not propagate, and invalid types must
// short-circuit before any DB call.

type fakeRecorder struct {
	calls    []db.CreateInteractionParams
	err      error
	panicErr any
}

func (f *fakeRecorder) CreateInteraction(_ context.Context, arg db.CreateInteractionParams) error {
	if f.panicErr != nil {
		panic(f.panicErr)
	}
	f.calls = append(f.calls, arg)
	return f.err
}

func TestRecord_HappyPathPropagatesParams(t *testing.T) {
	r := &fakeRecorder{}
	userID := uuid.New()
	pinID := uuid.New()

	Record(context.Background(), r, userID, pinID, "view")

	if len(r.calls) != 1 {
		t.Fatalf("expected exactly 1 CreateInteraction call, got %d", len(r.calls))
	}
	got := r.calls[0]
	if got.UserID != userID {
		t.Errorf("UserID: got %s, want %s", got.UserID, userID)
	}
	if !got.PinID.Valid {
		t.Errorf("PinID.Valid: got false, want true")
	}
	if got.PinID.UUID != pinID {
		t.Errorf("PinID.UUID: got %s, want %s", got.PinID.UUID, pinID)
	}
	if got.Type != "view" {
		t.Errorf("Type: got %q, want %q", got.Type, "view")
	}
}

func TestRecord_AcceptsAllThreeKnownTypes(t *testing.T) {
	for _, kind := range []string{"view", "pin", "board_add"} {
		t.Run(kind, func(t *testing.T) {
			r := &fakeRecorder{}
			Record(context.Background(), r, uuid.New(), uuid.New(), kind)
			if len(r.calls) != 1 {
				t.Fatalf("kind=%s: expected 1 call, got %d", kind, len(r.calls))
			}
			if r.calls[0].Type != kind {
				t.Errorf("kind=%s: Type recorded as %q", kind, r.calls[0].Type)
			}
		})
	}
}

// Scenario "기록 실패가 유저 경험에 영향을 주지 않는다" — DB 에러가 호출자에게 전파되어선 안 됨.
// Record는 반환값이 없으므로 panic이 발생하지 않고 정상 return 하는 것 자체가 best-effort 보장이다.
func TestRecord_DBErrorDoesNotPropagate(t *testing.T) {
	r := &fakeRecorder{err: errors.New("db down")}

	// Should not panic and should not block.
	Record(context.Background(), r, uuid.New(), uuid.New(), "pin")

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call attempt despite error, got %d", len(r.calls))
	}
}

// 잘못된 type은 DB 호출 전에 차단되어야 한다. 그렇지 않으면 CHECK 제약이 없는
// VARCHAR(20) 컬럼에 임의 문자열이 들어가 추천 엔진 입력이 오염된다.
func TestRecord_InvalidTypeShortCircuits(t *testing.T) {
	r := &fakeRecorder{}

	Record(context.Background(), r, uuid.New(), uuid.New(), "click")
	Record(context.Background(), r, uuid.New(), uuid.New(), "")
	Record(context.Background(), r, uuid.New(), uuid.New(), "VIEW") // case-sensitive

	if len(r.calls) != 0 {
		t.Fatalf("expected 0 calls for invalid types, got %d", len(r.calls))
	}
}
