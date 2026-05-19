# Why

`openspec/specs/interaction/spec.md` Requirement `유저 행동을 기록한다`는 다음 세 Scenario에서 시스템이 유저의 작품 조회·핀 생성·보드 추가 행위를 기록해야 한다고 명시한다(SHALL):

- Scenario "작품 조회 기록": WHEN 유저가 작품 상세 페이지를 열면 THEN 조회 행동이 기록된다
- Scenario "핀 생성 기록": WHEN 유저가 작품을 핀하면 THEN 핀 행동이 기록된다
- Scenario "보드 추가 기록": WHEN 유저가 핀을 보드에 추가하면 THEN 보드 추가 행동이 기록된다

또한 Scenario "기록 실패가 유저 경험에 영향을 주지 않는다"는 "유저의 원래 요청(조회, 핀, 보드 추가)은 정상적으로 처리된다"고 명시한다. "원래 요청"이라는 표현과 `AGENTS.md`의 "interaction: 암묵적 행동 기록 (view, pin, board_add)" 표현이 일관되게 piggyback 모델 — 원래 요청 핸들러가 부수효과로 기록을 함께 수행 — 을 가정한다.

현재 코드는 별도 endpoint `POST /api/interactions`(`internal/interaction/handler.go`)만 가지며, `pin.Handler.GetByID`·`pin.Handler.Create`·`boards.Handler.AddPin` 어느 곳에서도 `CreateInteraction`을 호출하지 않는다. 즉 세 Scenario의 SHALL이 production에서 enforce되지 않는다. `grep -rn "CreateInteraction" apps/api/internal/ --include="*.go"`로 호출자를 확인하면 `internal/interaction/handler.go:55`가 유일하다.

본 변경은 piggyback 모델로 spec SHALL을 enforce하도록 wiring을 보충한다. 별도 endpoint(`POST /api/interactions`)는 본 변경의 범위 밖이며 변경하지 않는다.

# What Changes

- **interaction capability에 Requirement 추가**: `시스템은 핀 조회·핀 생성·보드 추가 핸들러 진입 시 인증된 호출자에 한해 interaction 행동을 best-effort로 기록한다` (3 Scenarios). 기존 Requirement `유저 행동을 기록한다`는 변경하지 않고 본 Requirement가 그 SHALL의 enforce 계약을 보장하는 wiring 계약이 된다.
- **`GET /api/pins/{id}` 라우트에 `auth.OptionalJWTMiddleware` 부착**: 현재 미들웨어 미부착 상태라 인증 컨텍스트가 절대 노출되지 않아 view 기록 분기에 도달할 수 없다. 401을 도입하지 않기 위해 `OptionalJWTMiddleware`를 사용한다(미인증 호출자는 그대로 통과).
- **interaction 패키지에 best-effort 헬퍼 도입**: `interaction.Record(ctx, q, userID, pinID, type)`. 내부적으로 `db.Queries.CreateInteraction`을 호출하고 에러는 log만 — 호출자에게 에러를 반환하지 않는다.
- **세 핸들러에서 헬퍼 호출**:
  - `pin.Handler.GetByID`: 응답 직전에 `auth.CreatorIDFromContext`로 인증 분기 후 `interaction.Record(..., 'view')`.
  - `pin.Handler.Create`: 핀 생성 트랜잭션 성공 후 201 응답 직전 `interaction.Record(..., 'pin')`.
  - `boards.Handler.AddPin`: 응답 직전 `interaction.Record(..., 'board_add')`.
- **단위 테스트**: `interaction.Record`의 best-effort 동작(인증된 호출자만 INSERT, type 화이트리스트 외 reject, DB 에러 시 panic·return 없음)을 검증한다. 핸들러 통합 테스트는 가능한 범위에서 호출 여부를 검증한다(testdb 의존성이 없으면 본 change 범위 밖으로 표시).

# Out of Scope

- 별도 endpoint `POST /api/interactions`(`internal/interaction/handler.go`) 동작 변경·삭제.
- `docs/architecture.md`의 비동기 이벤트 파이프라인(Go channel → Event Worker → Firehose → S3) 도입. 본 결함의 최소 해결은 동기 best-effort INSERT로 충분하며, 비동기 파이프라인은 별도 design 결정이다.
- `interactions` 테이블 스키마 변경·인덱스 추가.
- 기존 Requirement `유저 행동을 기록한다`의 Scenarios 변경(본 change는 그 SHALL을 enforce하는 wiring 계약을 추가할 뿐).
