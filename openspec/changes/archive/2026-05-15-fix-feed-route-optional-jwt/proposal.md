## Why

`openspec/specs/feed/spec.md`의 Requirement `개인화된 추천 피드를 제공한다`는 3개 Scenario를 SHALL로 묶고 있다.

- "충분한 핀이 있는 유저" — 10개 이상의 핀을 가진 인증 유저는 개인화 추천 + 최신 혼합을 받는다.
- "핀이 부족한 유저(콜드 스타트)" — 10개 미만의 핀을 가진 인증 유저는 최신순을 받는다.
- "비인증 유저" — 인증되지 않은 요청자는 최신순을 받는다.

`apps/api/internal/feed/handler.go:54-112`의 `Handler.GetFeed`는 `auth.CreatorIDFromContext(r.Context())`로 인증 상태를 분기한다. 그러나 `apps/api/cmd/server/main.go:169`의 라우트 정의 `r.Get("/api/feed", feedHandler.GetFeed)`에는 어떤 JWT 미들웨어도 부착되어 있지 않다. 기존 `auth.JWTMiddleware`(`apps/api/internal/auth/middleware.go:24-52`)는 토큰 부재 시 401을 반환하는 강제 인증 미들웨어이므로 비인증 시나리오를 위해 부착할 수 없다. 결과적으로 production에서 인증 토큰을 제시한 호출자도 `creatorIDKey`가 컨텍스트에 세팅되지 않아 `authenticated==false`로 평가되고, `buildPersonalizedFeed` 분기와 `RecommendByTags`/`RecommendByMediaType` 호출이 도달 불가 코드가 된다. spec이 보장한 "충분한 핀이 있는 유저"·"핀이 부족한 유저(콜드 스타트)" 두 시나리오의 SHALL이 production에서 enforce되지 않는다.

본 change는 그 라우팅·미들웨어 갭만 한정해서 닫는다.

## What Changes

- production 서버 라우터에서 `/api/feed`는 인증 토큰이 존재하면 인증 컨텍스트를 핸들러로 전달하고, 토큰이 없거나 유효하지 않으면 그대로 통과한다.
- 토큰이 있어도 401을 반환하지 않으며, 비인증 호출자가 200으로 최신 피드를 받는 기존 행위를 유지한다.
- auth 도메인에 선택적 인증 미들웨어가 도입되어 라우트 단에서 인증·비인증 분기를 모두 지원한다.

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities

- `auth`: 기존 Requirement `인증이 필요한 요청을 보호한다`(토큰 부재 시 401 반환)는 변경하지 않는다. ADDED Requirement `토큰이 존재할 때 선택적으로 인증 컨텍스트를 노출한다`(SHALL)을 추가해 인증·비인증을 모두 허용하는 엔드포인트가 사용할 미들웨어 surface를 명시한다.
- `feed`: 기존 Requirement `개인화된 추천 피드를 제공한다`의 SHALL 본문과 5개 Scenario는 그대로 유지한다. ADDED Requirement `피드 라우트는 선택적 인증 미들웨어로 보호된다`(SHALL)을 추가해 production 라우팅 wiring을 spec 수준에서 enforce한다. 본 wiring Requirement는 기존 Requirement의 의미를 좁히거나 넓히지 않는다.

## Impact

- 영향 코드: `apps/api/internal/auth/middleware.go`(선택적 미들웨어 추가), `apps/api/cmd/server/main.go:169`(라우트 wiring), 신규 단위 테스트.
- 운영 지표: 인증 토큰을 제시한 호출자가 처음으로 개인화 피드를 받게 되어 `RecommendByTags`/`RecommendByMediaType` 호출량과 Redis 캐시 키(`feed:<uuid>:<limit>:<offset>`) 트래픽이 증가한다. 비인증 호출자의 응답 본문은 변하지 않는다.
- 의존성·인프라·DB 마이그레이션 없음.
