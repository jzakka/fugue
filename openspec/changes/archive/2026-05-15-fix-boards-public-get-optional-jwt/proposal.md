## Why

`openspec/specs/board/spec.md`는 다음 두 SHALL Scenario를 묶고 있다.

- Requirement `보드를 조회한다` Scenario "소유자는 비공개 보드 조회 가능": 소유자가 자신의 비공개 보드를 조회하면 보드 정보와 핀 목록이 반환된다.
- Requirement `유저의 보드 목록을 조회한다` Scenario "본인의 보드 목록": 인증된 유저가 자신의 보드 목록을 조회하면 공개 + 비공개 모든 보드가 반환된다.

`apps/api/internal/boards/handler.go`의 `GetByID`(L144-170)와 `ListByCreator`(L364-405)는 `auth.CreatorIDFromContext`로 호출자 인증 상태를 분기한다. `GetByID`는 비공개 보드에 한해 owner 일치를 확인하고, `ListByCreator`는 owner인 경우 `ListBoardsByCreator`(공개+비공개 전체)로, 그렇지 않으면 `ListPublicBoardsByCreator`(공개만)로 분기한다.

그러나 `apps/api/cmd/server/main.go:156`(`r.Get("/", boardsHandler.ListByCreator)`)와 `apps/api/cmd/server/main.go:158`(`r.Get("/{id}", boardsHandler.GetByID)`)는 어떤 인증 미들웨어도 부착하고 있지 않다. 결과적으로 인증 토큰을 제시한 호출자도 `creatorIDKey`가 컨텍스트에 세팅되지 않아 항상 `authenticated == false`로 평가된다.

- `GetByID`: 비공개 보드 소유자가 자신의 보드를 조회해도 owner 일치 분기가 도달 불가하여 404가 반환된다 = "소유자는 비공개 보드 조회 가능" SHALL 위반.
- `ListByCreator`: 소유자가 본인을 `creator_id`로 지정해 조회해도 owner 분기가 도달 불가하여 공개 보드만 반환된다 = "본인의 보드 목록" SHALL 위반.

본 change는 직전 머지된 `OptionalJWTMiddleware`(`apps/api/internal/auth/middleware.go`)를 두 라우트에 부착해서 동일한 패턴으로 두 SHALL을 production에서 enforce한다.

## What Changes

- production 서버 라우터에서 `GET /api/boards/{id}`와 `GET /api/boards`는 인증 토큰이 존재하면 인증 컨텍스트를 핸들러로 전달하고, 토큰이 없거나 유효하지 않으면 그대로 통과한다.
- 토큰이 있어도 401을 반환하지 않으며, 비인증 호출자가 공개 보드(목록)를 받는 기존 행위를 유지한다.
- 핸들러 본문 로직(공개/비공개 분기, owner 일치 검사)은 변경하지 않는다. 본 change는 wiring 한 점만 닫는다.

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities

- `board`: 기존 Requirement `보드를 조회한다`·`유저의 보드 목록을 조회한다`의 SHALL 본문과 Scenario는 변경하지 않는다. ADDED Requirement `공개 보드 조회 라우트는 선택적 인증 미들웨어로 보호된다`(SHALL)를 추가해 production 라우팅 wiring을 spec 수준에서 enforce한다. 본 wiring Requirement는 기존 Requirement의 의미를 좁히거나 넓히지 않는다.

## Impact

- 영향 코드: `apps/api/cmd/server/main.go:156`·`apps/api/cmd/server/main.go:158`(라우트 wiring), 신규 wiring 회귀 테스트.
- 운영 지표: 인증 토큰을 제시한 호출자가 처음으로 자기 비공개 보드 응답을 받게 되어 `ListBoardsByCreator`/owner 분기 호출량이 증가한다. 비인증 호출자의 응답은 변하지 않는다(공개 보드만 반환).
- 의존성·인프라·DB 마이그레이션 없음. `OptionalJWTMiddleware`는 직전 사이클에서 도입돼 이미 main에 머지된 상태이므로 추가 도입 없이 재사용한다.
