## Why

`docs/architecture.md`의 "### Rate Limit" 섹션은 두 SHALL을 두 줄 문장으로 명시한다.

- "핀 생성: 30/분/유저"
- "OG fetch: 20/분/IP"

추가로 `apps/api/cmd/server/main.go:97-100`에서 `auth.NewRateLimiter(rdb, 10, time.Minute)`(`authRL`), `auth.NewRateLimiter(rdb, 5, time.Minute)`(`callbackRL`)가 OAuth 로그인·콜백 라우트에 부착된다. 두 limit은 doc에 명시되어 있지 않지만 같은 fixed-window 분당 의미를 전제한다.

이 SHALL들은 "분 단위 윈도우가 끝나면 카운터가 0으로 리셋된다"는 fixed-window 의미를 전제한다. 그러나 `apps/api/internal/auth/ratelimit.go:27-52`의 `RateLimiter.Middleware`는 INCR과 EXPIRE를 별개의 Redis 왕복으로 발행한다.

```go
count, err := rl.rdb.Incr(ctx, key).Result()   // L33
if err != nil { /* fail-open */ return }
if count == 1 {
    rl.rdb.Expire(ctx, key, rl.window)         // L41 — 결과 .Err() 무시, 원자성 없음
}
```

INCR이 성공하고 EXPIRE 왕복만 실패하는 경우(네트워크 일시 단절, redis pool 재연결, 서버 프로세스 재시작)에 키는 TTL=-1로 영구 잔존한다. 후속 요청의 INCR은 count를 누적시키지만 `count == 1` 분기가 다시 실행되지 않아 EXPIRE는 재발행되지 않는다. `count > limit`가 되는 순간부터 그 (IP, path) 쌍은 무한 429를 받는다. 결과적으로 doc의 "30/분/유저"·"20/분/IP" SHALL은 한 번의 EXPIRE 미적용만으로 "30/lifetime/IP"·"20/lifetime/IP"로 무한 강화되어 위반된다.

본 change는 이 비원자성만 한정해 해결한다. limit 값·윈도우 길이·라우트 적용 매트릭스는 변경하지 않는다.

## What Changes

- HTTP 요청 빈도 제한 미들웨어는 카운터 증가와 윈도우 TTL 설정을 단일 원자 단위로 수행해야 한다.
- 첫 INCR이 성공했는데 후속 EXPIRE만 실패하는 경로가 존재해서는 안 된다.
- 라우트 적용 매트릭스(`/api/pins`·`/api/og/fetch`·`/api/auth/{provider}/login`·`/api/auth/{provider}/callback`·`/api/auth/logout`)·limit 값·윈도우 길이는 변경하지 않는다.
- 카운터 윈도우는 fixed-window 의미를 유지한다. 즉 첫 INCR 시점부터 윈도우 길이만큼 카운터가 유지되고, 윈도우 경계가 지나면 자연 만료로 0이 된다. 매 요청마다 TTL을 리셋하는 sliding-window 의미는 채택하지 않는다.

## Capabilities

### New Capabilities

- `ratelimit`: HTTP 요청 빈도 제한 미들웨어의 행위 계약을 cross-cutting infrastructure capability로 명시한다. 본 capability는 미들웨어가 보장해야 할 fixed-window 동작·원자성·fail-open·키 분리 규칙만 다루며, 개별 라우트에 적용되는 구체적 limit 값(30/분/유저 등)은 `docs/architecture.md`의 라우트 표가 계속 소유한다.

### Modified Capabilities

<!-- 없음. 기존 auth 등 capability의 SHALL은 변경하지 않는다. -->

## Impact

- 영향 코드: `apps/api/internal/auth/ratelimit.go`(`Middleware` 본문 1회 rewrite), 신규 단위 테스트.
- 운영 지표: Redis 왕복 수가 첫 요청에서 2회 → 1회(Lua EVAL)로 줄어든다. 후속 요청도 동일하게 1회 EVAL. 응답 본문·헤더·상태 코드는 변경되지 않는다. 정상 동작 중인 키의 행위는 그대로 유지된다.
- 의존성·인프라·DB 마이그레이션 없음. Redis 서버 측 변경 없음(EVAL은 Redis 2.6+ 표준 명령).
- 롤백: 단일 함수 rollback. 변경 전후로 외부 컨트랙트가 동일하므로 무중단 가능.
