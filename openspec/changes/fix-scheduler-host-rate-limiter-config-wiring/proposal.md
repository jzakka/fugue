## Why

scheduler 도메인 스펙은 운영자가 `scheduler.host_default_rate_per_sec`/`scheduler.host_default_burst` 설정값을 변경하면 그 이후 처음 등장하는 호스트의 token bucket이 운영자 기본값으로 생성되어야 한다고 요구하고(`기본 rate와 burst는 설정 가능하다`), token bucket 검사 자체를 전역 비활성화하는 운영 설정도 요구한다(`token bucket 검사를 비활성화할 수 있다`). 환경변수 `SCHEDULER_HOST_DEFAULT_RATE_PER_SEC`/`SCHEDULER_HOST_DEFAULT_BURST`/`SCHEDULER_HOST_TOKEN_BUCKET_ENABLED`는 `apps/api/internal/config/config.go`에서 Config 구조체로 이미 로드되지만, 정작 `apps/api/cmd/bot/main.go`의 두 `NewHostRateLimiter` 호출처는 공장 기본값 상수 `scheduler.FactoryDefaultRatePerSec`/`FactoryDefaultBurst`와 하드코딩된 `enabled=true`를 그대로 전달한다. 즉 운영자가 env를 변경하거나 token bucket을 끄려 해도 봇 워커는 항상 1 req/s · burst 5 · enabled=true로 기동되어, 스펙이 보장한 운영 제어가 실제로는 불가능하다.

## What Changes

- `apps/api/cmd/bot/main.go`의 `pioneer`(L258)·`harvester`(L474) 명령 두 곳 모두에서 `scheduler.NewHostRateLimiter` 호출 인자를 `config.Load()`로 얻은 `Config.SchedulerHostDefaultRatePerSec`/`SchedulerHostDefaultBurst`/`SchedulerHostTokenBucketEnabled` 값으로 변경한다.
- `Infrastructure` 초기화 단계에서 `config.Load()`를 호출하고 결과 포인터를 `Infrastructure.Config` 필드로 노출해, 워커 명령들이 동일한 Config 인스턴스를 참조하도록 한다.
- `apps/api/cmd/bot/main_test.go`(신규 또는 기존 파일에 추가)에 위 와이어링이 Config 값을 실제로 사용함을 검증하는 단위 테스트를 추가한다.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `scheduler`: 운영자 설정값(`scheduler.host_default_rate_per_sec`/`scheduler.host_default_burst`/`scheduler.host_token_bucket_enabled`)이 봇 워커 부팅 시 host rate limiter 구성에 실제로 전달된다는 요구사항을 신규 추가한다(`ADDED Requirements`). 기존 요구사항 두 건(`기본 rate와 burst는 설정 가능하다`, `token bucket 검사를 비활성화할 수 있다`)은 limiter의 행동만 규정하고 워커 엔트리포인트 와이어링 책임을 명시하지 않아 코드 갭이 가능했다.

## Impact

- 영향 범위: `apps/api/cmd/bot/main.go`, `apps/api/internal/config/config.go`(필요 시 import 정리만), 신규 봇 와이어링 테스트.
- 호환성: 환경변수 미지정 시의 동작은 동일하다. `envFloat`/`envInt`/`envBool`가 각각 1.0/5/true를 기본값으로 사용하므로 default 동작은 `FactoryDefaultRatePerSec=1`, `FactoryDefaultBurst=5`, `enabled=true`와 비트 단위로 일치한다.
- 외부 의존성/마이그레이션 없음.
