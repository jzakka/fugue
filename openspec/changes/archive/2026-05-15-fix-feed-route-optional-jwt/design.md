## Context

`openspec/specs/feed/spec.md`의 `개인화된 추천 피드를 제공한다` Requirement는 인증 상태와 보유 핀 수에 따라 세 가지 피드 분기(개인화·콜드스타트·비인증 최신)를 SHALL로 묶는다. 행위 자체의 분기 로직은 `apps/api/internal/feed/handler.go`의 `Handler.GetFeed` / `buildPersonalizedFeed` / `buildLatestFeed`에 이미 구현되어 있다. 갭은 그 핸들러에 도달하는 라우트가 인증 컨텍스트를 전달하지 않는다는 점뿐이다.

`apps/api/internal/auth/middleware.go`는 토큰 부재 시 401을 반환하는 강제 `JWTMiddleware` 한 종류만 노출한다. 이 미들웨어를 `/api/feed`에 부착하면 spec의 "비인증 유저" Scenario가 깨져 401이 반환된다. 부착하지 않으면 인증 유저가 늘 비인증으로 분류되어 "충분한 핀이 있는 유저" Scenario가 깨진다. 두 시나리오 모두 통과하려면 토큰 존재 시 선택적으로 컨텍스트를 채우고 그렇지 않으면 그대로 통과시키는 별도 미들웨어가 필요하다.

본 change는 그 미들웨어 surface를 auth 도메인에 추가하고 `/api/feed` 라우트 한 곳에만 적용한다. 같은 surface를 다른 엔드포인트(예: `/api/pins`, `/api/boards/{id}`)에 광범위 적용하는 결정은 본 change 범위 밖이다.

## Goals / Non-Goals

**Goals:**
- production `/api/feed` 라우트가 인증 토큰 존재 시 인증 컨텍스트를 핸들러에 전달한다(SHALL).
- 토큰 부재·만료·파싱 실패 어느 경우에도 401을 반환하지 않고 핸들러를 호출한다(SHALL).
- auth 패키지에 선택적 인증 미들웨어 surface가 존재하고, 다른 비인증·인증 혼합 엔드포인트가 같은 surface를 재사용할 수 있다.
- 회귀 방지를 위해 단위 테스트가 두 분기(토큰 있음·없음·잘못된 토큰)를 모두 검증한다.

**Non-Goals:**
- `JWTMiddleware`의 기존 SHALL(`인증이 필요한 요청을 보호한다`) 동작 변경.
- 다른 라우트(`/api/pins`, `/api/boards/...` 등)에 선택적 인증 미들웨어를 광범위하게 부착.
- 피드 분기 로직(`buildPersonalizedFeed`/`buildLatestFeed`)의 추천 알고리즘·캐싱 정책 변경.
- 토큰 만료를 클라이언트에 알리기 위한 `X-Token-Expired` 헤더 노출(기존 `JWTMiddleware`만 사용; 선택적 미들웨어는 토큰 만료를 비인증과 동등하게 취급한다).

## Decisions

### Decision 1: 선택적 미들웨어는 `auth` 패키지 안에 `OptionalJWTMiddleware`로 신설한다

**선택**: `apps/api/internal/auth/middleware.go`에 `func OptionalJWTMiddleware(jwtSvc *JWTService) func(http.Handler) http.Handler`를 추가한다. 기존 `JWTMiddleware`와 동일하게 쿠키(`fugue_access`)·`Authorization: Bearer` 순으로 토큰을 추출하지만, 토큰이 비어 있거나 `ValidateToken`이 실패하거나 `claims.Subject` 파싱이 실패하면 401을 내지 않고 다음 핸들러를 그대로 호출한다.

**대안**:
- (a) `JWTMiddleware`에 `optional bool` 인자를 추가해 두 모드를 한 함수로 분기 → 호출 측이 분기 의도를 인자 boolean으로만 노출해 라우터 정의 한눈에 의도가 드러나지 않는다.
- (b) 핸들러 안에서 `extractToken`을 직접 호출하고 `jwtSvc`를 의존성 주입 → 핸들러가 토큰 추출·검증 책임을 떠안아 단일 책임을 깨고, 다른 혼합 인증 엔드포인트가 동일한 패턴을 복제하게 된다.

**근거**: (b)는 라우터 단에서 인증 처리를 모듈화한 기존 설계를 깨뜨린다. (a)는 함수 signature에 boolean 인자가 더해져 호출 측 가독성이 떨어진다. 별도 함수 신설은 한 라우터 정의에서 `JWTMiddleware`와 `OptionalJWTMiddleware`를 텍스트로 구분 가능하게 하므로 라우트 표 자체가 인증 정책을 표현하게 된다. 신설 비용은 30라인 미만이며 회귀 위험이 가장 작다.

### Decision 2: 토큰이 있되 유효하지 않은(`ValidateToken` 실패) 요청도 비인증과 동등하게 통과시킨다

**선택**: 만료·서명 불일치·파싱 실패 어느 사유로든 `jwtSvc.ValidateToken(tokenString)` 또는 `uuid.Parse(claims.Subject)`가 실패하면 컨텍스트에 `creatorIDKey`를 세팅하지 않고 다음 핸들러를 호출한다.

**대안**:
- (a) 만료 토큰만 비인증으로 통과시키고 서명 불일치 등 그 외 실패는 401 → 라우트 단에서 비인증 시나리오를 보장할 수 없다. spec "비인증 유저"는 토큰 유효성과 무관하게 200을 요구한다.
- (b) 만료 토큰일 때 `X-Token-Expired` 헤더만 세팅해 클라이언트에 갱신을 유도 → 본 change 범위 밖이며 다른 라우트의 만료 처리 동작과 일관성을 깨뜨릴 위험이 있다.

**근거**: feed 응답은 인증 여부에 따라 본문이 갈리지만 두 경로 모두 200을 반환한다. 토큰 유효성과 무관하게 "토큰 컨텍스트가 없으면 비인증으로 처리"라는 단순 규칙이 spec 3개 Scenario를 모두 통과시킨다. 만료 헤더 통지는 후속 change에서 별도 결정한다.

### Decision 3: spec delta는 auth/feed 두 capability에 각 1개씩 ADDED Requirement를 추가한다

**선택**:
- `auth` spec: ADDED Requirement `토큰이 존재할 때 선택적으로 인증 컨텍스트를 노출한다`(SHALL) — 미들웨어 capability surface.
- `feed` spec: ADDED Requirement `피드 라우트는 선택적 인증 미들웨어로 보호된다`(SHALL) — production 라우트 wiring.

**대안**:
- (a) feed spec에 한 Requirement만 추가하고 auth 도메인 surface는 코드만 추가 → 다른 엔드포인트가 같은 surface를 채택할 때 spec 단에서 surface 존재를 표현할 곳이 없다.
- (b) auth spec에 한 Requirement만 추가 → feed 라우트 wiring 회귀가 spec 단에서 탐지되지 않는다(이번 갭이 정확히 그 유형이다).

**근거**: 본 change의 갭은 두 층(미들웨어 surface 부재 + feed 라우트 wiring 누락)이 동시에 존재해서 발생했다. 한 층에만 spec delta를 두면 다른 층의 회귀를 spec status로 잡을 수 없다. 두 Requirement는 각각 다른 capability에 속하고 서로의 SHALL 본문을 침범하지 않는다. 행위(분기 결과) 자체는 기존 `개인화된 추천 피드를 제공한다` Requirement가 이미 SHALL로 묶고 있으므로 추가 Requirement는 모두 wiring/surface 계약만 규정한다.

## Risks / Trade-offs

- **[Risk] 만료된 토큰을 가진 호출자가 비인증으로 분류되어 자신이 만든 핀과 무관한 추천을 받음**: 클라이언트가 만료를 인지하지 못하면 사용자 입장에서 "갑자기 추천이 비개인화로 보임"으로 관찰될 수 있다. **Mitigation**: 본 change는 만료 통지를 다루지 않는다. 클라이언트는 다른 인증 필수 엔드포인트(`/api/auth/me` 등)에서 401 + `X-Token-Expired` 헤더를 통해 만료를 감지하고 `/api/auth/refresh`를 호출하는 기존 흐름을 그대로 사용한다.
- **[Risk] 캐시 키 충돌 우려**: 인증 분기가 활성화되면 `feed:<uuid>:<limit>:<offset>` 캐시 키가 처음 사용된다. 캐시 키는 인증 유저별로 다르고 비인증은 캐시하지 않으므로 충돌 위험은 없다. **Mitigation**: 캐시 TTL은 기존 5분 유지. 캐시 일관성 결함이 관찰되면 별도 change에서 처리.
- **[Trade-off] `OptionalJWTMiddleware` surface 추가**: auth 패키지에 함수가 1개 추가되어 라이브러리 surface가 늘어난다. 다른 혼합 인증 엔드포인트가 채택할 수 있는 1차 surface이며, 본 change는 `/api/feed`에만 적용한다.
- **[Trade-off] feed 응답 본문이 인증 유저에 대해 변경**: 동일 호출자가 동일 쿠키로 동일 URL을 호출했을 때, 본 change 적용 후 응답 본문이 달라질 수 있다(개인화 분기 도달). 이는 spec의 의도된 행위이며 외부 컨트랙트(`FeedResponse` 스키마)는 변하지 않는다.
