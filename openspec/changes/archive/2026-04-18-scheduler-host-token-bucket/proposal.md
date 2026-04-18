## Why

`scheduler-frontier-table` 도입 이후 Pioneer/Harvester는 동일 frontier에서 URL을 claim하지만, 호스트별 요청률을 제한하는 메커니즘이 없어 한 사이트로 트래픽이 몰리면 상대 서버에 과부하를 주거나 차단을 유발할 수 있다. robots.txt의 Crawl-delay나 일반적인 politeness 관행을 강제할 수 있는 **호스트별 rate limit**이 claim 경로에 필요하다.

## What Changes

- `scheduler` capability에 **호스트별 token bucket 기반 politeness** 동작을 신규 요구사항으로 추가한다.
- 본 change는 호스트별 token bucket의 **행위 계약**(허용-여부 질의 동작, 호스트 rate/burst 설정 동작)만 정의한다. claim 시점의 후보 iteration과 모든 후보 blocked 시 sleep 동작은 본 change 범위가 아니며, `scheduler-claim-api`의 Claim 프로토콜에서 정의된다.
- 기본 rate는 **호스트당 1 req/sec**, 기본 burst는 **5**로 설정 가능한 값으로 정의한다.
- rate/burst 유효성 정책: `rate <= 0` 또는 `burst <= 0`이 입력되면 **기본값(1 req/sec, burst 5)으로 대체하고 경고 로그**를 남긴다. 서비스는 중단되지 않는다.
- 운영 롤백 수단으로 호스트별 token bucket 검사를 전역 비활성화하는 설정을 정의한다(비활성 시 허용-여부 질의는 항상 true).
- Pioneer가 robots.txt에서 추출한 Crawl-delay 값을 해당 호스트 bucket의 rate로 surface하는 인터페이스를 정의한다(파싱은 `pioneer-link-filter-policy` 범위).
- Token bucket은 **프로세스별 인메모리 상태**로 유지되며, 프로세스 간 조율은 하지 않는다.

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `scheduler`: 호스트별 token bucket의 허용-여부 질의 동작과 호스트 rate/burst 설정 동작, 기본 rate/burst 설정, 유효성 대체 정책, robots.txt Crawl-delay 반영 인터페이스, 전역 비활성화 설정을 추가한다.

## Impact

- **코드**: scheduler 모듈에 token bucket 관리 컴포넌트 추가. `golang.org/x/time/rate` 의존성 추가. dequeue 경로에서의 호출(후보 row의 host에 대한 허용-여부 질의 호출 패턴, 모든 후보 blocked 시 sleep)은 `scheduler-claim-api` change에서 정의.
- **운영**: 동일 사이트로의 요청률이 호스트당 1 req/sec(기본)로 제한되어 차단/항의 위험 감소. 단, 복수 scheduler 프로세스 운영 시 프로세스 수에 비례하여 실제 호스트 요청률이 증가함(프로세스 간 조율 없음) — 운영자가 프로세스 수와 호스트 rate를 함께 튜닝해야 한다.
- **의존성**: `golang.org/x/time/rate`.
- **연관 change**: `pioneer-link-filter-policy`(robots.txt 파싱), `scheduler-claim-api`(URLScheduler interface와 dequeue 흐름), `scheduler-frontier-table`(claim 후보 출처).
