package interaction

import (
	"context"
	"log"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// Recorder is the minimal surface Record depends on. *db.Queries satisfies it
// out of the box, and tests inject a mock.
type Recorder interface {
	CreateInteraction(ctx context.Context, arg db.CreateInteractionParams) error
}

// Record performs a best-effort interaction insert. Errors are logged and
// never returned so callers can invoke it as a side effect of their main
// request handler without affecting the response.
//
// spec: interaction `시스템은 핀 조회·핀 생성·보드 추가 핸들러 진입 시 인증된
// 호출자에 한해 interaction 행동을 best-effort로 기록한다`
//
// Callers must guard the call with an authenticated-only branch (Scenario
// "미인증 호출자의 핀 조회에는 interaction이 기록되지 않는다").
func Record(ctx context.Context, r Recorder, userID, pinID uuid.UUID, kind string) {
	if !isValidInteractionType(kind) {
		log.Printf("interaction.Record: invalid type %q (user=%s pin=%s)", kind, userID, pinID)
		return
	}
	if err := r.CreateInteraction(ctx, db.CreateInteractionParams{
		UserID: userID,
		PinID:  uuid.NullUUID{UUID: pinID, Valid: true},
		Type:   kind,
	}); err != nil {
		log.Printf("interaction.Record: db error: %v (user=%s pin=%s type=%s)", err, userID, pinID, kind)
	}
}
