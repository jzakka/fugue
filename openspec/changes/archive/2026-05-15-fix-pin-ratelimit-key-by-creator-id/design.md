## Context

직전 사이클에서 도입된 `ratelimit` capability는 미들웨어가 보장해야 할 cross-cutting 행위 계약(원자성·fixed-window·fail-open)을 한 곳에 모았다. 그 capability의 Purpose는 명시적으로 다음을 선언한다.

> 본 capability는 라우트별 limit 값·윈도우 길이·적용 매트릭스를 규정하지 않으며 그것들은 `docs/architecture.md`의 Rate Limit 섹션이 소유한다.

architecture.md의 Rate Limit 섹션은 두 SHALL을 짧게 명시한다.

- "핀 생성: 30/분/유저"
- "OG fetch: 20/분/IP"

여기서 "/유저"·"/IP" 접미사는 limit의 분모(어떤 식별자로 카운터를 분리하느냐)를 분명히 다른 두 단위로 규정한다. 그러나 현재 미들웨어는 두 라우트 모두 `path + IP`로만 키를 만든다. 라우트 적용 매트릭스를 architecture.md가 소유하더라도, 그 매트릭스를 만족시키려면 미들웨어 surface가 두 식별자(IP / creator)를 모두 표현할 수 있어야 한다. 본 change는 그 surface를 보강한다.

## Goals / Non-Goals

### Goals

- `RateLimiter`에 creator 단위로 카운터를 분리하는 미들웨어 surface를 추가한다. 그 surface는 path + creator-id를 키로 사용한다.
- `/api/pins POST` 라우트를 새 surface로 교체해 architecture.md의 "핀 생성: 30/분/유저" SHALL이 production에서 의도된 per-user 단위로 enforce되게 한다.
- 직전 사이클의 fixed-window 원자성 invariant는 surface 변경과 무관하게 그대로 보존한다(동일한 `rateLimitScript`를 공유).

### Non-Goals

- limit 값·윈도우 길이·라우트 적용 매트릭스 그 자체는 변경하지 않는다. 30/min·1min·"`/api/pins`만 per-user, 나머지는 per-IP"는 architecture.md가 계속 소유한다.
- sliding-window 의미로의 변경, 라우트 추가/삭제, Redis 외 storage 도입, IP 추출 알고리즘 변경(`extractIP`는 그대로 유지), Retry-After 헤더 정책 변경은 본 change 범위 밖이다.
- 새 spec capability 도입 없음. `ratelimit` capability에 Requirement 1건을 ADDED로 추가한다.

## Decisions

### Decision 1: 두 surface를 같은 `RateLimiter` 인스턴스가 모두 제공한다 (옵션 A 채택)

세 가지 옵션을 검토했다.

- **(A) 같은 인스턴스의 두 메서드 — `Middleware()`(IP) + `MiddlewareByCreatorID()`(creator)** ← 채택.
  - 장점: 호출부가 명시적으로 surface를 선택. 한 라우트의 의도가 wiring 한 줄에 드러난다. 같은 인스턴스가 같은 limit/window를 공유하므로 30/min·1min 등 상수 중복이 없다. 기존 `pinRL`/`ogRL`/`authRL`/`callbackRL` 인스턴스 변수는 그대로 재사용된다.
  - 단점: `RateLimiter`가 두 메서드를 노출한다. 호출부가 잘못된 surface를 선택할 경우 doc SHALL이 다시 어긋날 수 있다(이건 review 단계에서 보장).
- (B) 별도 타입 `UserRateLimiter` 신설.
  - 장점: 두 surface가 타입으로 분리되어 잘못 선택 위험이 더 낮다.
  - 단점: limit/window를 두 곳에 복제. main.go의 인스턴스 변수가 두 배가 되고, 같은 라우트에 두 인스턴스를 헷갈리게 부착할 위험이 새로 생긴다.
- (C) `Middleware()`를 "smart" 자동 판별로 변경 — 인증 컨텍스트가 있으면 creator, 없으면 IP.
  - 장점: 호출부 변경 0.
  - 단점: 라우트의 의도가 wiring에서 사라진다. `/api/og/fetch`가 인증 유저에게 한해 갑자기 per-user로 동작하는 식의 행위 변동을 doc SHALL이 막을 수 없다. surface가 호출부의 의도를 보존하는 게 더 안전하다.

(A)는 호출부가 surface 선택을 명시적으로 책임지므로 doc SHALL과 wiring을 1:1 mapping할 수 있다.

### Decision 2: 인증 컨텍스트 부재 시 IP fallback

`MiddlewareByCreatorID`는 `auth.CreatorIDFromContext`로 식별자를 꺼낸다. 정상 wiring(`r.With(JWTMiddleware, pinRL.MiddlewareByCreatorID)`)에서는 항상 식별자가 존재한다. 그러나 wiring 사고로 JWTMiddleware가 누락되면 식별자가 없다. 세 대안:

- **(A) `creator:` prefix가 붙은 IP fallback** ← 채택.
  - `"ip:" + extractIP(r)`를 키 버킷으로 사용한다. 카운터는 여전히 분리되어 무제한 통과를 막는다.
  - 단점: doc의 "/유저" 의미가 잠시 어긋나지만, 이는 wiring 사고에 대한 안전망이지 정상 경로의 행위 변경이 아니다.
- (B) 식별자가 없으면 fail-open(throttle 없이 통과).
  - 단점: 인증이 빠진 wiring을 사실상 무제한 핀 생성으로 만든다. 보안적으로 더 나쁘다.
- (C) 식별자가 없으면 401/500 반환.
  - 단점: 미들웨어가 인증 책임을 일부 떠안는다. surface는 빈도 제한에만 책임지고 인증은 `JWTMiddleware`가 책임지는 SoC를 깨뜨린다.

(A)는 surface의 책임 경계를 깨지 않으면서, wiring 사고가 가용성과 보안 모두를 더 나쁘게 만들지 않게 한다.

### Decision 3: 키 prefix 분리(`creator:<uuid>` vs `ip:<addr>`)

같은 라우트(`/api/pins`)의 두 surface(`Middleware` vs `MiddlewareByCreatorID`)는 키 prefix(`ip:`·`creator:`)로 분리한다. 이론적으로 `/api/pins`에 두 surface가 동시에 부착될 일은 없지만, surface 선택이 잘못 바뀌었을 때 직전 카운터가 그대로 누적되어 spike하는 회귀를 막는다. 또한 디버깅 시 키 이름만으로 어떤 surface가 카운팅 중인지 식별 가능하다.

기존 `Middleware`의 키 포맷(`rl:<path>:<ip>`)을 깨지 않으려면, IP 경로는 `ip:` prefix 없이 그대로 두는 선택지도 있다. 하지만 `creator:` prefix만 도입해도 두 surface의 키는 충돌하지 않고(creator UUID는 hex가 아닌 raw UUID 표현이라 IP 표기와 형식이 다르다), 직관적인 분리가 가능하므로 IP 경로의 prefix는 유지하지 않는다.

요약:

- `Middleware()` 키: `rl:<path>:<ip>` (변경 없음)
- `MiddlewareByCreatorID()` 키: `rl:<path>:creator:<uuid>` (정상 경로) 또는 `rl:<path>:ip:<addr>` (wiring fallback)

## Risks / Trade-offs

- **R1 (이 change의 핵심 회귀 가능성)**: `/api/pins POST` 라우트를 새 surface로 교체하면 그 라우트의 Redis 키 prefix가 바뀐다. 배포 직후 기존 IP-keyed 카운터(`rl:/api/pins:<ip>`)와 새 creator-keyed 카운터(`rl:/api/pins:creator:<uuid>`)는 다른 키를 사용하므로, 직전 윈도우의 카운트가 새 윈도우로 자동 이월되지 않는다. 다음 1분 동안 같은 유저가 60개를 만들 수 있다.
  - 완화: 본 프로젝트의 배포 정책은 `AGENTS.md`에 "로컬 개발만 하므로 상용에서 카나리 배포, 무중단 같은건 고려 안해도 된다"로 명시되어 있다(`AGENTS.md:152`). 운영 SHALL이 아니므로 1분 경계의 일회성 카운터 손실은 수용 범위.
- **R2**: `MiddlewareByCreatorID`에서 인증 컨텍스트가 부재한 wiring 사고 시 IP fallback이 발동하면 doc SHALL이 일시적으로 IP 단위로 되돌아간다. 모니터링 측면에서 키 prefix(`ip:`)를 통해 식별 가능. 운영 절차는 본 change 범위 밖이지만, 키 형식이 사고를 가린다 → 드러낸다로 바뀌는 점은 안전 쪽 변화이다.
- **R3**: 새 메서드 추가로 `RateLimiter`의 surface가 두 개로 늘어 호출부가 잘못된 surface를 고를 가능성이 생긴다. 완화: 라우트 wiring에 spec comment(`// spec: docs/architecture.md 핀 생성: 30/분/유저`)를 부여해 review 시 의도를 검증한다(`/api/feed`·`/api/boards/{id}` optional JWT wiring 사이클에서 적용한 패턴과 같다).
- **R4**: 신규 Requirement는 surface 추상화를 명시한다. 향후 라우트 매트릭스가 architecture.md에서 변동되어도 surface 자체가 두 단위를 표현할 수 있으므로 spec 자체는 안정적이다.

## Alternatives Considered

- "creator-id 사용 여부를 인스턴스 생성 시점에 결정"(`NewRateLimiterByCreatorID(...)`로 별도 인스턴스): 같은 라우트에 두 surface를 동시 부착할 일이 없고 limit/window가 같다면 인스턴스를 두 배로 복제할 이유가 없다. 옵션 (A) 채택과 같은 이유로 기각.
- IP 추출 함수 자체를 "extractIdentity"로 일반화하고 라우트별 함수를 주입하는 구조: 한 라우트가 두 surface를 모두 노출할 일이 없어 일반화 비용이 효익보다 크다. 두 메서드 노출이 더 명료.

## Migration Plan

1. `RateLimiter` 본문에 surface 두 개(`Middleware` 기존, `MiddlewareByCreatorID` 신규)를 공통 헬퍼로 분리. 헬퍼는 `rateLimitScript`(직전 사이클의 Lua EVAL)를 그대로 사용한다.
2. `/api/pins POST` 라우트 wiring 1줄을 `pinRL.MiddlewareByCreatorID`로 교체한다. 다른 라우트(`/api/og/fetch`·`/api/auth/*`)는 그대로 둔다.
3. miniredis 기반 단위 테스트를 추가해 (a) 같은 유저가 다른 IP에서도 같은 버킷을 공유, (b) 같은 IP의 두 유저가 서로 다른 버킷을 가짐, (c) 인증 컨텍스트가 없을 때 IP fallback이 작동, (d) 기존 IP-keyed `Middleware`의 행위가 변하지 않음 4개를 보장한다. 기존 6개 fixed-window/fail-open 테스트는 그대로 통과해야 한다.
4. `openspec validate --specs --strict`로 ratelimit capability의 신규 Requirement가 main spec과 정합한지 검증한다.
5. archive 시점에 새 Requirement를 main spec에 머지한다. capability의 Purpose는 변경하지 않는다.

## Open Questions

없음. doc SHALL의 enforce 단위, 미들웨어 surface API 선택, fallback 정책 모두 본 design에서 확정.
