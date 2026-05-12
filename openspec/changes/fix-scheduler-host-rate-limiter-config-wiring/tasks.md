## 1. config 패키지 확장

- [x] 1.1 `apps/api/internal/config/config.go`에 신규 타입 `SchedulerHostConfig`를 export하고, 봇이 OAuth/JWT 의존 없이 호출 가능한 `LoadSchedulerHostConfig() SchedulerHostConfig` 함수를 추가한다.
- [x] 1.2 기존 `Config` 구조체의 `SchedulerHostDefaultRatePerSec`/`SchedulerHostDefaultBurst`/`SchedulerHostTokenBucketEnabled` 필드 로딩 코드도 같은 helper를 재사용하도록 정리하되, 외부 동작은 손대지 않는다.

## 2. 봇 와이어링 헬퍼 추가

- [x] 2.1 `apps/api/cmd/bot/host_rate_limiter.go`(신규 파일)에 `buildHostRateLimiter(cfg config.SchedulerHostConfig) *scheduler.HostRateLimiter` 헬퍼를 작성한다.
- [x] 2.2 헬퍼는 cfg의 세 값을 그대로 `scheduler.NewHostRateLimiter`에 위임한다(어떤 fallback도 적용하지 않음 — fallback은 config 계층의 책임).

## 3. 두 호출처 와이어링 교체

- [x] 3.1 `apps/api/cmd/bot/main.go`의 `harvesterCmd`(현재 L258) `NewHostRateLimiter(...)` 호출을 `buildHostRateLimiter(config.LoadSchedulerHostConfig())`로 교체한다.
- [x] 3.2 `apps/api/cmd/bot/main.go`의 `runPioneerConsumer`(현재 L474) `NewHostRateLimiter(...)` 호출을 동일 헬퍼로 교체한다.
- [x] 3.3 `scheduler.FactoryDefaultRatePerSec`/`FactoryDefaultBurst` import가 다른 곳에서 사용되지 않으면 import 정리한다.

## 4. 회귀 테스트 추가

- [x] 4.1 `apps/api/cmd/bot/host_rate_limiter_test.go`(신규)에 다음 케이스를 추가한다:
  - 운영자 기본값 0.5/3로 빌드된 limiter는 처음 보는 host에 대해 burst 3을 넘는 4번째 Allow에서 false를 반환한다.
  - `enabled=false`로 빌드된 limiter는 임의 host에 대해 연속 100회 Allow가 모두 true이다.
  - cfg 미지정(env unset) 시 LoadSchedulerHostConfig가 rate=1.0, burst=5, enabled=true를 반환하며, 그렇게 빌드된 limiter는 burst 5+1=6번째에서 false를 반환한다.

## 5. 빌드/테스트 검증

- [x] 5.1 `cd apps/api && go build ./...` 통과.
- [x] 5.2 `cd apps/api && go test ./...` 통과(기존 421개 + 신규 케이스).
- [x] 5.3 `cd apps/web && npm run build && npm test` 통과(웹 측 변경 없음 — 회귀 확인 목적).
