## Why

`docs/architecture.md`의 "### Rate Limit" 섹션은 두 SHALL을 한 줄씩 명시한다.

- "핀 생성: 30/분/유저"
- "OG fetch: 20/분/IP"

두 SHALL은 enforce 단위(per-user vs per-IP)까지 분명히 구분한다. 그러나 `apps/api/internal/auth/ratelimit.go:48-69`의 `RateLimiter.Middleware`는 모든 라우트에서 동일하게 `path + extractIP(r)`로 키를 만든다. creator/유저 식별자는 키 구성에 사용되지 않으며, 미들웨어는 `auth.CreatorIDFromContext`를 호출하지도 않는다.

`apps/api/cmd/server/main.go:138`에서 `r.With(auth.JWTMiddleware(jwtSvc), pinRL.Middleware).Post("/api/pins", pinHandler.Create)` 순서로 wiring되어 있어 미들웨어 시점에 인증 컨텍스트는 이미 존재하지만, 현재 `pinRL.Middleware`는 그 정보를 무시한다. 결과적으로 `/api/pins POST`의 카운터는 doc이 요구한 per-user 30/분이 아니라 per-IP 30/분으로 enforce된다.

영향 시나리오:

- **공유 NAT throttling**: 인증 유저 A·B가 같은 NAT 뒤(공유 wifi, 사내망, 모바일 캐리어 grade NAT)에 있을 때 A가 30개를 만들면 B는 자신의 쿼터를 한 번도 안 쓰고 429를 받는다.
- **IP rotation으로 per-user 상한 우회**: 단일 유저가 Wi-Fi → 카페 → 모바일 등 IP를 바꿔가며 각각 30개씩 핀을 만들 수 있어 doc의 per-user 상한도 enforce되지 않는다.

`/api/og/fetch`(OG fetch)는 doc이 명시한 대로 per-IP 의미가 맞고 현행 구현과 일치하므로 변경 대상이 아니다. `/api/auth/{provider}/login`·`/callback`·`/logout`의 limit는 doc에 별도 SHALL이 없고, 인증 전·중 라우트라 per-IP가 자연스러우므로 그대로 둔다.

본 change는 `RateLimiter` 미들웨어에 유저 단위 키 surface를 추가하고, `/api/pins POST` 라우트만 그 surface로 바꾼다. limit 값(30)·윈도우 길이(1분)·다른 라우트 적용 매트릭스는 변경하지 않는다.

## What Changes

- `RateLimiter`에 creator(유저 식별자) 기반으로 키를 만드는 미들웨어 surface를 추가한다.
- 그 surface는 요청 컨텍스트의 인증된 식별자가 존재할 때 그 식별자를 카운터 버킷 키로 사용해야 한다. 같은 식별자에 대한 요청은 클라이언트 IP가 달라도 같은 버킷을 공유해야 한다.
- 같은 surface가 인증 컨텍스트 없이 도달했을 때(상위 미들웨어 누락 같은 wiring 오류)는 클라이언트 IP를 fallback 키로 사용해 요청을 무제한 통과시키지 말아야 한다.
- 기존 IP 기반 surface는 보존된다. `/api/og/fetch`·`/api/auth/{provider}/login`·`/api/auth/{provider}/callback`·`/api/auth/logout`의 키 의미는 변경하지 않는다.
- `apps/api/cmd/server/main.go:138`의 `/api/pins POST` wiring을 새 surface로 교체한다. 이 라우트의 limit 값(30)·윈도우(1분)·미들웨어 순서(JWTMiddleware → RateLimiter → handler)는 변경하지 않는다.
- 카운터 윈도우는 fixed-window 의미를 유지한다(직전 사이클의 `ratelimit` capability Requirement). 매 요청마다 TTL을 리셋하는 sliding-window 의미는 채택하지 않는다.

## Capabilities

### Modified Capabilities

- `ratelimit`: 기존 fixed-window 원자성 Requirement에 더해, "유저 단위 빈도 제한 surface를 노출한다" Requirement 1건을 추가한다. 본 Requirement는 surface 행위 계약만 다루며 어떤 라우트가 그 surface를 사용해야 하는지는 `docs/architecture.md`의 Rate Limit 섹션이 계속 소유한다.

### New Capabilities

<!-- 없음. ratelimit capability는 직전 사이클에서 도입되었다. -->

## Impact

- 영향 코드: `apps/api/internal/auth/ratelimit.go`(creator-keyed surface 추가, 기존 IP-keyed surface 유지), `apps/api/cmd/server/main.go:138`(`/api/pins` POST wiring 1줄 교체), 신규 단위 테스트.
- 운영 지표: Redis 키 prefix가 `rl:/api/pins:<IP>` → `rl:/api/pins:creator:<uuid>` 형태로 바뀐다(같은 path 안에서도 분리). 응답 본문·헤더·상태 코드·limit 값·윈도우 길이는 변경되지 않는다. 인증된 정상 유저의 분당 30개 상한은 유지되며, 단지 그 카운트가 NAT 동거인이 아닌 본인의 행동만을 누적한다.
- 의존성·인프라·DB 마이그레이션 없음. Redis 서버 측 변경 없음.
- 롤백: 단일 라우트 wiring 1줄을 `.MiddlewareByCreatorID` → `.Middleware`로 되돌리면 즉시 직전 동작으로 복귀(이 경우 doc SHALL 위반이 다시 발생하지만 가용성에는 영향 없음). 변경 전후로 라우트 외부 컨트랙트가 동일하다.
