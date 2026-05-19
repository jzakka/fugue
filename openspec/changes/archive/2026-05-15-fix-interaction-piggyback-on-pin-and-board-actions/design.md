# Design

## Context

`interaction/spec.md` Requirement `유저 행동을 기록한다`의 SHALL 4건이 production에서 enforce되지 않는 결함을 해결한다. 코드 상 `CreateInteraction`의 유일한 호출자는 별도 endpoint `POST /api/interactions`(`internal/interaction/handler.go:55`)이며, 핀 조회·핀 생성·보드 추가의 "원래 요청" 핸들러는 어떤 interaction도 기록하지 않는다.

spec text의 "유저의 원래 요청(조회, 핀, 보드 추가)은 정상적으로 처리된다"라는 표현은 piggyback 모델 — 원래 요청 핸들러가 부수효과로 기록 — 을 가정한다. `AGENTS.md`의 "암묵적 행동 기록" 표현도 같은 모델을 시사한다.

## Decision 1: 동기 best-effort piggyback INSERT 모델 채택

원래 요청 핸들러가 응답 직전에 `db.Queries.CreateInteraction`을 best-effort로 호출한다. 에러는 log만 하고 응답에는 영향을 주지 않는다. 별도 트랜잭션이며 같은 `r.Context()`를 공유한다.

대안으로 `docs/architecture.md`에 명시된 비동기 이벤트 파이프라인(Go channel → Event Worker → Firehose → S3)이 있다. 그러나:

- 비동기 파이프라인 도입은 channel 도입·Worker goroutine·외부 의존성(Firehose) 설계가 필요한 큰 변경이다.
- 본 결함은 "interactions 테이블에 row가 생성되지 않는다"라는 좁은 문제이다. 동기 INSERT로 충분히 SHALL을 enforce할 수 있다.
- 비동기 파이프라인은 별도 design 결정으로 미루고, 본 change는 최소 해결만 한다.

## Decision 2: 헬퍼 함수 `interaction.Record`로 추출

세 호출 지점(`pin.GetByID`, `pin.Create`, `boards.AddPin`)이 동일 패턴(인증 분기 → INSERT → 에러 log)이므로 헬퍼 1개로 추출한다.

```go
// in apps/api/internal/interaction/recorder.go

type Recorder interface {
    CreateInteraction(ctx context.Context, arg db.CreateInteractionParams) error
}

// Record performs a best-effort interaction insert. Errors are logged and
// never returned, so callers can call it as a side effect of their main
// request handler without affecting the response.
//
// spec: interaction `시스템은 핀 조회·핀 생성·보드 추가 핸들러 진입 시 인증된
// 호출자에 한해 interaction 행동을 best-effort로 기록한다`
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
```

`Recorder` 인터페이스로 mockable. `db.Queries`가 이 인터페이스를 만족한다(이미 `CreateInteraction(ctx, params) error` 시그니처를 가진다).

`isValidInteractionType`은 기존 `internal/interaction/handler.go`의 동명 함수를 재활용한다(현재 unexported. 패키지 내부에서 그대로 사용).

## Decision 3: `GET /api/pins/{id}` 라우트에 `OptionalJWTMiddleware` 부착

현재 라우트(`apps/api/cmd/server/main.go:141`)는 미들웨어 미부착 상태라 `auth.CreatorIDFromContext`가 항상 false를 반환한다. view 기록 분기에 도달하려면 인증 컨텍스트가 노출되어야 한다.

401을 도입하지 않기 위해(공개 핀 상세는 미인증에도 열려 있어야 함) `OptionalJWTMiddleware`를 사용한다. 이는 직전 change `fix-feed-route-optional-jwt`·`fix-boards-public-get-optional-jwt`와 동일한 패턴이다.

미인증 호출자는 `OptionalJWTMiddleware`를 통과하지만 `auth.CreatorIDFromContext` ok=false라 view 기록 분기에 진입하지 않는다(spec Scenario "미인증 유저는 기록하지 않는다" 보존).

`POST /api/pins`와 `POST /api/boards/{id}/pins`는 이미 `JWTMiddleware`로 보호되므로 인증 컨텍스트가 보장된다. 라우팅 변경 불필요.

## Decision 4: 호출 위치는 응답 직전

`pin.Create`은 핀 생성 트랜잭션이 성공한 후 응답을 만들고 `writeJSON` 직전에 `interaction.Record` 호출. 핀 생성 실패 시(202 이전 return) 기록 분기에 진입하지 않는다.

`pin.GetByID`는 핀을 DB에서 조회한 후 응답 직전에 호출. 404·500 분기에서는 호출하지 않는다(존재하지 않는 핀에 대한 view는 무의미).

`boards.AddPin`은 board-pin 관계 INSERT 성공 후 응답 직전에 호출. board 권한 검사·핀 존재 검사 실패 분기에서는 호출하지 않는다.

이는 "원래 요청"이 정상적으로 성공한 경우에만 piggyback 기록이 발생한다는 자연스러운 의미이다. 실패한 행위까지 기록하면 추천 엔진 입력에 노이즈가 섞인다.

## Decision 5: 트랜잭션 분리 및 컨텍스트 공유

`interaction.Record`는 호출자의 `r.Context()`를 그대로 사용한다(별도 트랜잭션, 같은 context). 호출자가 cancel 시 INSERT도 중단된다.

핀 생성/조회/board 추가의 트랜잭션과 분리되어 있어 interaction INSERT 실패가 원래 동작을 rollback하지 않는다. 이는 Scenario "기록 실패가 유저 경험에 영향을 주지 않는다"의 SHALL과 일치.

## Decision 6: 기존 endpoint `POST /api/interactions`는 변경하지 않는다

별도 endpoint(`internal/interaction/handler.go:31` `Create`)는 client-side에서 명시적으로 type을 보낼 때 사용된다(예: 작품 상세 페이지 진입 후 일정 시간 머문 경우의 "engaged-view" 같은 신호). 본 change에서는 deprecate 여부·동작 변경을 결정하지 않는다.

자동 기록과 명시적 기록이 같은 `interactions` 테이블에 row를 만들 수 있어 중복 가능성이 있지만:

- 추천 엔진의 입력은 `GetUserTagFrequency`(현재는 user의 핀 작품 태그 빈도)로 interaction 테이블을 아직 직접 입력으로 쓰지 않는다.
- 미래에 interaction을 직접 입력으로 쓸 때 중복 처리는 그 분석/모델링 단계의 책임이다.
- 본 change의 최소 해결 원칙에 따라 dedup 정책은 범위 밖.

## Decision 7: 단위 테스트 범위

- `interaction.Record` 헬퍼 단위 테스트: (a) 정상 INSERT(mock Recorder의 호출 인자 검증), (b) DB 에러 시 panic·return 없음, (c) 잘못된 type 시 INSERT 호출 안 함, (d) `pinID`가 zero uuid라도 그대로 전달(검증은 caller 책임).
- `pin.Create`·`pin.GetByID`·`boards.AddPin`의 통합 테스트는 본 change 범위에서 별도 인터페이스 도입을 요구하므로, 본 change에서는 헬퍼 단위 테스트로 SHALL 보장의 최소 단위를 확보한다. 핸들러 측 호출 여부 검증은 후속 change에서 핸들러를 mockable한 형태로 refactor할 때 추가한다.

## Risk

- **회귀**: best-effort INSERT라 원래 요청 실패로 이어지지 않는다. 헬퍼 호출이 응답 직전에 위치하므로 응답 timing에 약간의 latency만 추가된다(단일 INSERT ms 단위).
- **DB 부하**: 핀 조회·생성·보드 추가 빈도만큼 INSERT 증가. interactions 테이블에는 이미 `(user_id, created_at DESC)`·`pin_id`·`type` 인덱스가 있어 쓰기 비용은 제한적.
- **마이그레이션**: 스키마 변경 없음. interactions 테이블·인덱스·sqlc 쿼리·db.Queries 코드 모두 그대로.

## Rollback

문제 발생 시 `pin/handler.go`·`boards/handler.go`에서 추가된 `interaction.Record` 호출 라인을 제거하고, `GET /api/pins/{id}` 라우트의 `OptionalJWTMiddleware`를 떼면 직전 상태로 복원된다. `interaction/recorder.go` 헬퍼 파일은 dead code로 남지만 동작에는 영향 없다(다음 정리에서 제거).
