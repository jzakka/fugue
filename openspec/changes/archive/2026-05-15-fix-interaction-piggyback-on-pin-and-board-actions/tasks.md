# Tasks

## 1. Spec delta

- [ ] 1.1 `openspec/changes/fix-interaction-piggyback-on-pin-and-board-actions/specs/interaction/spec.md`에 `## ADDED Requirements` + 1개 Requirement(3 Scenarios) 작성.
- [ ] 1.2 `openspec validate fix-interaction-piggyback-on-pin-and-board-actions --strict` 통과.

## 2. Helper

- [ ] 2.1 `apps/api/internal/interaction/recorder.go` 신규: `Recorder` interface + `Record(ctx, r, userID, pinID, kind)` 함수. 내부에서 `CreateInteraction` best-effort 호출, type 화이트리스트, 에러 log.
- [ ] 2.2 `apps/api/internal/interaction/handler.go`의 `isValidInteractionType`을 패키지 내에서 헬퍼와 공유 가능하도록 그대로 두고 `recorder.go`에서 재사용한다. handler.go의 import·로직은 변경하지 않는다.

## 3. Routing

- [ ] 3.1 `apps/api/cmd/server/main.go`의 `GET /api/pins/{id}`에 `auth.OptionalJWTMiddleware(jwtSvc)`를 부착한다(`r.Get` → `r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get`).

## 4. Handler wiring

- [ ] 4.1 `apps/api/internal/pin/handler.go` `GetByID`: 응답 직전에 `if creatorID, ok := auth.CreatorIDFromContext(r.Context()); ok { interaction.Record(r.Context(), h.queries(), creatorID, pinID, "view") }` 분기 추가.
- [ ] 4.2 `apps/api/internal/pin/handler.go` `Create`: 핀 생성 성공 후 응답 직전에 `interaction.Record(r.Context(), h.queries(), creatorID, newPinID, "pin")` 호출. creatorID는 이미 핸들러 본문에서 추출되어 있음.
- [ ] 4.3 `apps/api/internal/boards/handler.go` `AddPin`: board-pin 관계 INSERT 성공 후 응답 직전에 `interaction.Record(r.Context(), h.queries(), creatorID, pinID, "board_add")` 호출.
- [ ] 4.4 위 세 곳에서 사용할 `h.queries()`(또는 동등한 *db.Queries 접근자)는 핸들러 구조체의 기존 sqlc 접근 패턴을 따른다. 새 surface 추가가 필요하면 가장 작은 형태로만 추가.

## 5. Tests

- [ ] 5.1 `apps/api/internal/interaction/recorder_test.go` 신규: `Record` 헬퍼의 (a) 정상 INSERT(mock Recorder의 호출 인자 검증), (b) DB 에러 시 panic·return 없음, (c) 잘못된 type 시 INSERT 호출 안 함을 검증한다.

## 6. Verification

- [ ] 6.1 `cd apps/api && go build ./...`가 성공한다.
- [ ] 6.2 `cd apps/api && go test ./internal/interaction/... ./internal/pin/... ./internal/boards/...`가 회귀 없이 통과한다.
- [ ] 6.3 `openspec validate fix-interaction-piggyback-on-pin-and-board-actions --strict`가 다시 통과한다.
- [ ] 6.4 `openspec archive fix-interaction-piggyback-on-pin-and-board-actions --yes` 후 `openspec/specs/interaction/spec.md`에 신규 Requirement가 반영됐는지 확인한다.
