## Context

`docs/architecture.md`의 Rate Limit 섹션은 두 라우트의 분당 빈도 제한을 두 줄 문장으로 명시한다("핀 생성: 30/분/유저", "OG fetch: 20/분/IP"). `apps/api/cmd/server/main.go:97-100`에서 정의되는 `authRL`(10/분), `callbackRL`(5/분), `pinRL`(30/분), `ogRL`(20/분)이 각각 해당 라우트에 부착된다. 모든 limiter는 `apps/api/internal/auth/ratelimit.go`의 단일 `RateLimiter` 타입으로 구현되어 있다.

`RateLimiter.Middleware`(L27-52)는 키별 카운터를 Redis `INCR`로 누적하고, 카운터가 1일 때만 `EXPIRE`를 호출하는 fixed-window 패턴이다. 그러나 두 명령은 별개의 Redis 왕복으로 발행되고 `EXPIRE`의 결과 `.Err()`가 무시된다. 두 명령 사이에서 네트워크/프로세스 단절이 발생하면 키는 TTL=-1 상태로 영구 잔존하고, 후속 요청의 `INCR`는 `count == 1` 분기를 다시 통과하지 못해 `EXPIRE`가 재발행되지 않는다. `count > limit`를 넘어선 순간부터 해당 (IP, path) 쌍은 무한 429를 받는다.

본 change는 그 비원자성을 Lua `EVAL` 한 번으로 결합해 닫는다. limit 값·윈도우 길이·라우트 매트릭스·fail-open 정책·키 포맷은 변경하지 않는다.

## Goals / Non-Goals

**Goals:**
- 첫 INCR이 성공하면 같은 원자 단위에서 EXPIRE가 적용된다(SHALL).
- 카운터는 fixed-window 의미를 유지한다 — 첫 INCR 시점에 TTL이 설정되고 후속 요청은 TTL을 리셋하지 않는다(SHALL).
- Redis 명령 실패는 fail-open으로 처리한다(요청을 throttle하지 않는다)(SHALL).
- 회귀 방지를 위해 단위 테스트가 다음 두 경로를 검증한다.
  - 정상 경로: 윈도우 안에서 limit개 요청은 허용, limit+1번째는 429.
  - 회귀 경로: 첫 요청 후 키가 TTL=-1로 남는 시나리오를 사전에 차단(즉, 첫 INCR 직후 키의 TTL이 음수여서는 안 된다).

**Non-Goals:**
- 라우트별 limit 값·윈도우 길이 변경.
- 추가 라우트에 rate limit 부착(다른 라우트의 미부착은 별 change에서 다룬다).
- sliding-window·token bucket·distributed leaky bucket 등 다른 알고리즘 채택.
- `extractIP` 로직 변경(`r.RemoteAddr` + `middleware.RealIP` 신뢰).
- Redis pool·timeout·재시도 정책 변경.
- 응답 본문/`Retry-After` 헤더 포맷 변경.

## Decisions

### Decision 1: 원자성 확보는 Redis Lua `EVAL`로 INCR+EXPIRE를 묶는다

**선택**: `apps/api/internal/auth/ratelimit.go`에 패키지 변수 `rateLimitScript = redis.NewScript(...)`를 도입하고, 본문에서 `rateLimitScript.Run(ctx, rl.rdb, []string{key}, int(rl.window.Seconds())).Int64()` 한 번으로 카운터를 받는다. 스크립트 본문은

```lua
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return n
```

이며, `n`은 그대로 limit 비교에 사용된다.

**대안**:
- (a) `pipeline`으로 INCR+EXPIRE를 한 왕복에 묶기 — pipeline은 한 번의 네트워크 왕복이지만 Redis 서버 측에서 원자 실행이 아니다. 다른 클라이언트가 두 명령 사이에 끼어들 수는 없지만 첫 명령만 성공하고 두 번째가 실패하는 case는 여전히 가능하다. 또한 매 요청마다 EXPIRE를 재발행하면 TTL이 윈도우 전체로 리셋되어 fixed-window가 sliding-window로 의미가 바뀐다. fixed-window를 유지하려면 첫 INCR에서만 EXPIRE를 발행해야 하는데, pipeline 내부에서 첫 INCR의 결과로 조건분기는 불가능하다.
- (b) `SET key 0 EX <window> NX`로 키를 선점한 뒤 `INCR`을 별도 호출 — 두 왕복으로 늘어나고, NX 실패 case 처리가 복잡해진다. EVAL이 더 단순하다.
- (c) `IncrEX(key, 1, window)` 같은 명령 직접 사용 — go-redis는 그런 헬퍼를 제공하지 않는다.

**근거**: Lua `EVAL`은 Redis 서버 측에서 원자 실행이 보장된다(중간에 다른 명령이 끼어들 수 없으며, 스크립트는 부분 실행되지 않는다). 첫 INCR에서만 EXPIRE를 발행하는 fixed-window 의미를 그대로 표현할 수 있고, 추가 round trip이 없다. `redis.NewScript`는 SCRIPT LOAD + EVALSHA fallback EVAL을 자동 처리한다.

### Decision 2: Redis 명령 실패 시 fail-open을 유지한다

**선택**: `rateLimitScript.Run` 결과가 에러를 반환하면 기존 동작과 동일하게 `next.ServeHTTP(w, r)`로 통과시키고 종료한다.

**대안**:
- (a) Redis 실패 시 503 반환(fail-closed) — Redis 잠시 끊김으로 사용자 트래픽 전체가 503을 받는다. 운영 위험 증가.
- (b) Redis 실패 시 in-memory fallback limiter 가동 — process-local 카운터는 분산 limit을 보장하지 못한다. 코드 복잡도가 늘어나는 데 비해 정확도 이득이 미미하다.

**근거**: 본 change는 비원자성 결함만 닫는 데 한정한다. fail-open 정책은 기존 코드(`L34-37`)에서 이미 채택된 결정이며, 본 change의 범위 밖이다. 같은 정책을 그대로 유지하는 것이 회귀 위험이 가장 낮다.

### Decision 3: spec delta는 신규 `ratelimit` capability에 1개 Requirement를 추가한다

**선택**: `openspec/specs/ratelimit/spec.md`가 부재하므로, 본 change의 spec delta는 신규 capability로 추가된다. 그 안에 단 하나의 Requirement "HTTP 요청 빈도 제한 카운터는 단일 원자 단위로 증가·만료 설정된다"(SHALL)를 둔다. 본 Requirement는 미들웨어 행위 계약만 다루고, 라우트별 limit 값·윈도우·매트릭스는 `docs/architecture.md`가 계속 소유한다.

**대안**:
- (a) `auth` capability에 추가 — `apps/api/internal/auth/ratelimit.go` 위치 때문에 자연스러워 보이지만, 라우트 매트릭스의 다수(`/api/pins`·`/api/og/fetch`)가 auth와 무관하다. auth Requirement는 식별·세션 보호에 집중되어 있다.
- (b) 각 행위(auth·pin·og)에 별도로 추가 — 한 결함에 대해 4개의 동일한 SHALL을 4개 capability에 중복으로 둬야 한다. 회귀 시 4곳 동기화 위험.

**근거**: rate limit는 cross-cutting infrastructure이며 단일 미들웨어가 모든 라우트의 한 정책을 책임진다. 별도 capability가 가장 깔끔한 표현이다. 본 change가 capability 자체를 새로 만드는 부담은 spec 1개·Scenario 4개 분량이다.

## Risks / Trade-offs

- **[Risk] Lua 스크립트 호환성**: 본 코드베이스는 go-redis v9를 사용하고 Redis는 표준 fugue 클러스터(7.x 가정)를 사용한다. `EVAL` / `SCRIPT LOAD` / `EVALSHA`는 Redis 2.6+ 표준 명령이며 호환성 문제는 사실상 없다. **Mitigation**: redis-server 버전이 의도와 다른 환경에서도 fail-open 정책 덕분에 사용자 트래픽은 차단되지 않는다(rate limit이 임시로 비활성화되는 효과).
- **[Risk] 스크립트 캐시 미스의 추가 왕복**: 첫 호출 시 EVALSHA가 NOSCRIPT를 반환하면 go-redis는 EVAL로 자동 재시도하므로 한 요청당 추가 왕복 1회가 가능하다. **Mitigation**: `redis.NewScript`가 이 fallback을 내장하며, 두 번째 호출부터는 EVALSHA 한 번으로 끝난다.
- **[Trade-off] 운영자가 직접 키를 보고 TTL을 확인할 때 변화**: 변경 전에는 첫 INCR 직후 TTL이 잠시 -1이었다가 EXPIRE에서 윈도우로 설정되는 짧은 windows가 있었다. 변경 후에는 INCR과 EXPIRE가 원자라 TTL이 항상 윈도우 안에 있다. 외부 관측에는 영향 없음.
- **[Trade-off] 테스트 환경**: 단위 테스트는 miniredis 또는 in-process Redis mock을 요구한다. 본 코드베이스는 기존 auth 테스트들에서 redis 모킹을 사용하지 않는 패턴(real connection)을 따른다. 본 change는 그 패턴을 유지하지 않고, 테스트가 가능한 좁은 행위 계약(첫 INCR 직후 TTL이 음수가 아님)만 검증하는 단위 테스트를 추가한다. 실제 redis 인스턴스가 필요한 통합 테스트는 별 cycle의 인프라 책임으로 둔다.
