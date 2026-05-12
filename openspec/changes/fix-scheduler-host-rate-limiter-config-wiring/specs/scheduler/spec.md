## ADDED Requirements

### Requirement: 봇 워커는 부팅 시 운영자 설정값을 host rate limiter에 전달한다

봇 워커의 엔트리포인트(`apps/api/cmd/bot`)는 호스트 token bucket을 생성할 때 운영자가 환경변수로 지정한 다음 세 설정값을 그대로 host rate limiter 생성자에 전달해야 한다(SHALL):

- `scheduler.host_default_rate_per_sec` — 환경변수 `SCHEDULER_HOST_DEFAULT_RATE_PER_SEC`
- `scheduler.host_default_burst` — 환경변수 `SCHEDULER_HOST_DEFAULT_BURST`
- `scheduler.host_token_bucket_enabled` — 환경변수 `SCHEDULER_HOST_TOKEN_BUCKET_ENABLED`

봇 워커는 호스트 rate limiter 생성자에 공장 기본값 상수(`FactoryDefaultRatePerSec`/`FactoryDefaultBurst`)나 하드코딩된 boolean을 전달해서는 안 된다(SHALL NOT). 운영자가 환경변수를 지정하지 않은 경우의 기본값(rate=1, burst=5, enabled=true)은 Config 로딩 계층의 책임이며(`apps/api/internal/config/config.go`), 와이어링 계층은 그 값을 그대로 위임한다.

본 요구사항은 기존의 `기본 rate와 burst는 설정 가능하다` 요구사항(특히 "운영자가 기본 rate/burst를 변경한다" 시나리오)과 `token bucket 검사를 비활성화할 수 있다` 요구사항이 봇 워커 프로세스에서도 실제로 적용 가능하다는 것을 시스템 엔트리포인트 레벨에서 보장한다.

#### Scenario: pioneer 워커가 운영자 기본 rate/burst로 부팅한다

- **WHEN** 운영자가 `SCHEDULER_HOST_DEFAULT_RATE_PER_SEC=0.5`, `SCHEDULER_HOST_DEFAULT_BURST=3`, `SCHEDULER_HOST_TOKEN_BUCKET_ENABLED=true`로 환경을 구성한 뒤 `fuguebot pioneer <site>`를 기동할 때
- **THEN** pioneer 워커가 사용하는 host rate limiter는 처음 등장하는 호스트의 bucket을 rate=0.5 req/sec, burst=3으로 lazy 생성한다(공장 기본값 1/5가 적용되지 않는다)

#### Scenario: harvester 워커가 token bucket 비활성화 설정을 따른다

- **WHEN** 운영자가 `SCHEDULER_HOST_TOKEN_BUCKET_ENABLED=false`로 환경을 구성한 뒤 `fuguebot harvester <site>`를 기동할 때
- **THEN** harvester 워커가 사용하는 host rate limiter의 허용-여부 질의는 임의 호스트에 대해 항상 `true`를 반환한다(token bucket 상태와 무관)

#### Scenario: 환경변수 미지정 시 공장 기본값과 동일하게 동작한다

- **WHEN** 운영자가 위 세 환경변수를 모두 지정하지 않은 상태에서 봇 워커를 기동할 때
- **THEN** host rate limiter는 rate=1 req/sec, burst=5, enabled=true로 동작하여 기존 운영 동작과 비트 단위로 일치한다
