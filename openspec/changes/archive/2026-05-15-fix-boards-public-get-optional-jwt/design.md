## Context

`openspec/specs/board/spec.md`는 보드 조회를 두 Requirement로 나눈다. `보드를 조회한다`의 Scenario "소유자는 비공개 보드 조회 가능"은 비공개 보드 + 소유자 일치일 때 200을 요구하고, `유저의 보드 목록을 조회한다`의 Scenario "본인의 보드 목록"은 호출자가 본인을 조회할 때 공개+비공개 전체를 요구한다. 두 Scenario는 모두 호출자 인증 상태에 따라 분기를 요구한다.

`apps/api/internal/boards/handler.go`의 `GetByID`(L144-170)와 `ListByCreator`(L364-405)는 이미 그 분기 로직을 구현해 두었다. 갭은 production 라우터(`apps/api/cmd/server/main.go:156`, `158`)가 두 라우트에 어떤 인증 미들웨어도 부착하지 않아 핸들러가 항상 비인증 분기로 들어간다는 점뿐이다.

직전 change `fix-feed-route-optional-jwt`(2026-05-15-fix-feed-route-optional-jwt로 아카이브됨)에서 `auth.OptionalJWTMiddleware`를 신설했고, 그 surface는 본 change에서 재사용된다. 본 change는 추가 surface를 신설하지 않으며 두 라우트 wiring만 닫는다.

## Goals / Non-Goals

**Goals:**
- production `GET /api/boards/{id}` 라우트가 인증 토큰 존재 시 인증 컨텍스트를 핸들러에 전달한다(SHALL).
- production `GET /api/boards` 라우트가 인증 토큰 존재 시 인증 컨텍스트를 핸들러에 전달한다(SHALL).
- 토큰 부재·만료·파싱 실패 어느 경우에도 401을 반환하지 않고 핸들러를 호출한다(SHALL).
- 회귀 방지를 위해 단위 테스트가 두 라우트의 두 분기(토큰 있음·없음)를 모두 검증한다.

**Non-Goals:**
- `OptionalJWTMiddleware` 자체의 행위 변경. 그 행위는 auth `토큰이 존재할 때 선택적으로 인증 컨텍스트를 노출한다` Requirement에 이미 묶여 있다.
- 핸들러 본문의 공개/비공개 분기 로직, owner 일치 검사 조건 변경.
- 다른 board 라우트(`POST /`, `PUT /{id}`, `DELETE /{id}`, `POST /{id}/pins`, `DELETE /{id}/pins/{pin_id}`)의 미들웨어 변경. 그 라우트들은 이미 `JWTMiddleware`로 보호된다.
- `GET /api/pins/{id}/boards`(`boardsHandler.ListByPin`)의 미들웨어 변경. 그 핸들러는 공개 보드만 반환하며 owner 분기가 없다.

## Decisions

### Decision 1: `OptionalJWTMiddleware`를 두 라우트에 각각 부착한다 — 공통 사전 미들웨어 그룹은 도입하지 않는다

**선택**: `apps/api/cmd/server/main.go:156`·`158`을 각각 `r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get(...)`로 교체한다.

**대안**:
- (a) `r.Route("/api/boards", ...)` 블록 전체에 `r.Use(auth.OptionalJWTMiddleware(jwtSvc))`를 적용 → 같은 블록의 `POST`·`PUT`·`DELETE` 라우트는 이미 `JWTMiddleware`로 보호되며, 두 미들웨어를 동시 부착하면 인증 토큰이 두 번 파싱되고 강제 미들웨어가 401을 내는 의도가 흐려진다.
- (b) `Route` 블록을 둘로 분리해 GET만 별도 그룹으로 묶기 → 라우트 표 가독성이 떨어진다.

**근거**: `/api/feed` 라우트도 동일한 이유로 한 줄에 `r.With(...)`를 부착했다(직전 change Decision 1). 본 change는 그 패턴을 그대로 따른다. 두 라우트에 동일한 텍스트가 나타나는 비용보다 라우트별 인증 정책이 라우트 표에서 직접 보이는 가독성 이득이 크다.

### Decision 2: spec delta는 board capability에 ADDED Requirement 1개를 추가하고 auth capability는 손대지 않는다

**선택**:
- `board` spec: ADDED Requirement `공개 보드 조회 라우트는 선택적 인증 미들웨어로 보호된다`(SHALL) — `GET /{id}`·`GET /` 두 라우트의 wiring 계약.

**대안**:
- (a) Requirement를 둘로 쪼개 라우트별로 각 1개씩 → 같은 wiring 계약을 두 곳에 나누면 회귀 시 한쪽만 누락되는 비대칭이 spec 단에서 잡히지 않는다.
- (b) auth capability에 추가 Requirement를 더 추가 → auth `토큰이 존재할 때 선택적으로 인증 컨텍스트를 노출한다`가 이미 미들웨어 surface 계약을 묶고 있으므로 중복이다.

**근거**: auth surface는 이미 SHALL로 묶여 있고, 본 change는 그 surface의 부착 위치만 추가한다. 따라서 spec delta는 board capability에 wiring Requirement 1개로 충분하다. 두 라우트는 같은 미들웨어를 같은 의도로 부착하는 한 묶음이므로 단일 Requirement가 더 응집도 있다.

### Decision 3: 핸들러 본문 변경 없음 — owner 검사·공개/비공개 분기는 그대로 둔다

**선택**: `boards/handler.go`의 `GetByID`·`ListByCreator`는 코드 변경 없이 유지한다.

**대안**:
- (a) `GetByID`에서 비공개 보드 404 응답에 owner-시 200 분기를 명시적으로 분리해 가독성을 올리기 → 행위 변경 없이 코드 형태만 바뀌므로 본 change 범위를 벗어난다.
- (b) `ListByCreator`에 `creator_id` 미지정 시 본인 목록으로 해석하는 편의 분기 추가 → 외부 컨트랙트(`creator_id` 필수) 변경이며 본 change 범위를 벗어난다.

**근거**: 본 change는 wiring 한 점만 닫는다. 핸들러 본문 변경은 회귀 위험을 증가시키고 spec 본문도 늘어난다.

## Risks / Trade-offs

- **[Risk] 캐시·서드파티 CDN 캐시 키 충돌**: `GET /api/boards/{id}`는 핸들러 내부에서 Redis 캐시를 사용하지 않으므로 이번 변경으로 캐시 키 의미가 바뀌지 않는다. CDN 레이어가 동일 URL에 대해 본문 변형(인증 헤더 유무에 따른) 캐시 분리를 하지 않는 환경이라면 응답 본문 변형이 캐시될 수 있다. **Mitigation**: 운영 환경에 CDN이 도입되면 별도 change에서 `Vary: Cookie` 또는 캐시 우회 규칙을 정의한다. 본 change는 로컬·dev 한정 영향(AGENTS.md "배포 정책: 로컬 개발만").
- **[Trade-off] board 응답 본문이 인증 호출자에 대해 변경**: 동일 호출자가 동일 쿠키로 동일 URL을 호출했을 때, 본 change 적용 후 비공개 보드가 응답에 추가될 수 있다(`GET /api/boards`) 또는 404→200으로 응답이 바뀔 수 있다(`GET /api/boards/{id}`). 이는 spec의 의도된 행위이며 외부 컨트랙트(`BoardResponse`/`BoardDetailResponse` 스키마)는 변하지 않는다.
- **[Trade-off] 라우트 표 한 줄 길이 증가**: `r.Get("/{id}", ...)` → `r.With(auth.OptionalJWTMiddleware(jwtSvc)).Get("/{id}", ...)`. 가독성 측면에서 라우트별 인증 정책이 표로 드러나는 이점이 더 크다.
