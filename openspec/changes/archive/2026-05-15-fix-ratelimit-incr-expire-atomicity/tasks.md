## 1. Lua EVAL 기반 원자 INCR+EXPIRE 도입

- [x] 1.1 `apps/api/internal/auth/ratelimit.go` 패키지 변수에 `rateLimitScript = redis.NewScript(...)`를 선언한다. 스크립트 본문은 design.md Decision 1에 명시된 4줄 Lua. 추가 import: `github.com/redis/go-redis/v9` 이미 있음.
- [x] 1.2 `RateLimiter.Middleware` 본문에서 기존 `rl.rdb.Incr(...).Result()` + `if count == 1 { rl.rdb.Expire(...) }` 두 호출을 `rateLimitScript.Run(ctx, rl.rdb, []string{key}, int(rl.window.Seconds())).Int64()` 단일 호출로 교체한다. 반환된 `n int64`를 그대로 limit 비교에 사용한다.
- [x] 1.3 Redis 명령 실패 시 fail-open 분기는 그대로 유지한다(`if err != nil { next.ServeHTTP(w, r); return }`).
- [x] 1.4 fixed-window 의미를 design 주석으로 함수 직위에 1~3줄 남긴다(첫 INCR에서만 TTL이 설정되고 이후 요청은 TTL을 리셋하지 않는다).

## 2. 단위 테스트

- [x] 2.1 `apps/api/internal/auth/ratelimit_test.go`(신규)를 생성하고 `miniredis`(`github.com/alicebob/miniredis/v2`)를 통한 in-process Redis로 미들웨어를 검증한다. 의존성이 go.mod에 없으면 `cd apps/api && go get github.com/alicebob/miniredis/v2`로 추가한다.
- [x] 2.2 테스트 `TestRateLimiter_AllowsUpToLimit`: limit=3, window=1s 환경에서 동일 IP가 3번 200을 받고 4번째에 429를 받는지 검증.
- [x] 2.3 테스트 `TestRateLimiter_FirstIncrSetsTTL`: 첫 요청 직후 키의 TTL이 양수(`> 0`)임을 `miniredis.TTL(key)`로 직접 관찰한다. 변경 전 코드라면 EVAL이 없으므로 TTL이 일정 시점에 음수가 되는 race를 직접 재현하기 어렵지만, 본 테스트는 "한 번의 EVAL 후 키의 TTL이 항상 양수"라는 사후 조건을 보장한다.
- [x] 2.4 테스트 `TestRateLimiter_SubsequentIncrPreservesFixedWindow`: 첫 요청 후 0.5초 대기 → 두 번째 요청. 두 번째 요청 직후 키의 남은 TTL이 1초 이하(즉 sliding이 아님)임을 확인. miniredis에서는 `FastForward`로 시간 제어 가능.
- [x] 2.5 테스트 `TestRateLimiter_RedisFailureFailsOpen`: miniredis를 `Close()`한 뒤 요청을 보내 200이 반환되는지 검증(throttle되지 않음).
- [x] 2.6 테스트 `TestRateLimiter_WindowResetsAfterExpiry`: limit=2, window=1s 환경에서 2번 요청(둘 다 200) → miniredis `FastForward(2 * time.Second)` → 3번째 요청이 200을 받는지 검증.

## 3. 검증

- [x] 3.1 `cd apps/api && go build ./...` 통과.
- [x] 3.2 `cd apps/api && go test ./internal/auth/...` 통과.
- [x] 3.3 `openspec validate fix-ratelimit-incr-expire-atomicity --strict` 통과.
