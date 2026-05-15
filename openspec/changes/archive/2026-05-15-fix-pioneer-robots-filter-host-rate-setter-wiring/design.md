## Context

`bot` capability의 `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다` SHALL은 RobotsFilter의 외부 관찰 가능 행위(`SetHostRate` 호출)를 contract로 못박는다. `scheduler` capability의 대응 Requirement는 robots.txt 파싱·호출 타이밍을 `pioneer-link-filter-policy` capability에 위임한다고 명시한다.

코드 측 wiring 현황:

- `apps/api/internal/bot/robots_filter.go:72-80` `NewRobotsFilter(rateSetter HostRateSetter)` — rateSetter는 nil 허용. nil일 때 fetch·파싱은 동작하지만 SHALL이 요구하는 `SetHostRate` 호출 분기(L161 `f.rateSetter != nil` 가드)에서 빠진다.
- `apps/api/internal/scheduler/host_rate_limiter.go:78-` `SetHostRate(host string, ratePerSec float64, burst int)` — 메서드 시그니처가 `bot.HostRateSetter` 인터페이스(같은 시그니처)와 구조적으로 정확히 일치. 별도 어댑터 불필요.
- `apps/api/cmd/bot/main.go:475-509` `runPioneerConsumer` — `buildHostRateLimiter(config.LoadSchedulerHostConfig())`를 `WithRateLimiter`에 inline 호출로만 사용하고 변수에 보존하지 않는다. L486에서 별개로 `bot.NewRobotsFilter(nil)`이 호출된다.

이 결과 production에서는 RobotsFilter의 cache miss/TTL 만료 경로에서 `SetHostRate`가 절대 호출되지 않아, scheduler가 보유한 HostRateLimiter의 호스트 bucket은 항상 기본 rate(1 req/sec, 5 burst)로 lazy 생성된다.

## Goals

- `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다` SHALL이 production pioneer 워커에서 enforce되도록 RobotsFilter와 PGURLScheduler가 **같은** `*scheduler.HostRateLimiter` 인스턴스를 공유하게 한다.
- 이 공유 관계를 외부에서 결정적으로 검증할 수 있도록 wiring 함수를 surface로 노출한다.

## Non-Goals

- RobotsFilter의 robots.txt 파싱 로직, single-flight 구조, 캐시 TTL 24h 정책은 변경하지 않는다.
- `HostRateLimiter.SetHostRate`의 입력 검증·기본값 substitute 로직은 변경하지 않는다.
- harvester 부트스트랩의 wiring은 본 SHALL의 책임 분담(`pioneer-link-filter-policy`)에 따라 손대지 않는다.
- `bot.HostRateSetter` 인터페이스 정의나 `NewRobotsFilter` 시그니처는 변경하지 않는다. nil 허용 계약은 테스트 fixtures(이미 nil로 RobotsFilter를 생성하는 단위 테스트 다수)와 호환되어야 한다.
- 진행 중 change `fix-scheduler-host-rate-limiter-config-wiring`이 `buildHostRateLimiter`에 전달하는 Config 값을 다루므로, 본 change는 그 결과 인스턴스를 재사용하는 부분만 책임진다.

## Decisions

### Decision 1: 동일 인스턴스 공유 (옵션 A 채택)

`runPioneerConsumer` 안에서 `buildHostRateLimiter(...)` 결과를 지역 변수 `rl`로 추출하고, `sched.WithRateLimiter(rl)`과 `bot.NewRobotsFilter(rl)`에 **둘 다 전달**한다.

이유:
- spec의 두 Scenario("Crawl-delay 파싱 및 호스트 rate 갱신", "캐시 TTL 내 중복 호출 방지")가 enforce되려면 RobotsFilter가 호출하는 `SetHostRate`의 효과가 sched가 dequeue 시점에 호스트 bucket을 조회하는 동일 인스턴스에 반영되어야 한다. 다른 인스턴스를 두 곳에 넘기면 SHALL이 코드 상 호출은 되지만 dequeue 측에 반영되지 않아 enforce되지 않는다.
- 인터페이스 surface 변경 없음. `*scheduler.HostRateLimiter`가 `bot.HostRateSetter`를 구조적으로 만족하므로 어댑터 불필요.

옵션 B(`PGURLScheduler`에 HostRateLimiter getter surface 추가)는 간접성·surface 면적을 늘리는 만큼의 이득이 없어 기각.

### Decision 2: 결정적 검증을 위한 wiring 함수 노출

`runPioneerConsumer`에서 PioneerConsumer 의존 조립부(`sched`, `chain`, `store`, `consumer fetcher`, RobotsFilter wiring 포함)를 별도 함수 `buildPioneerConsumer(infra, rl)`로 분리한다. 테스트에서는 `rl`을 가짜 HostRateSetter로 치환해 동일 인스턴스가 RobotsFilter에 전파되었는지 결정적으로 확인할 수 있다.

이유:
- 기존 `buildHarvesterConsumer`(`apps/api/cmd/bot/harvester_consumer_builder.go`) 패턴과 일관성. 봇 워커 부트스트랩 함수들의 surface 면적을 통일.
- `runPioneerConsumer` 내부에 wiring과 lifecycle(`signal.NotifyContext`)이 섞여 있어 단위 테스트가 어려웠던 문제를 해소.

### Decision 3: 멱등 호출과 캐시 TTL의 상호작용 보존

spec Scenario "캐시 TTL 내 중복 호출 방지"는 같은 호스트 24h 캐시 내에서 `SetHostRate`가 갱신 시점에만 호출됨을 요구한다. RobotsFilter `refresh()` 경로가 이미 single-flight winner만 `SetHostRate`를 호출하도록 짜여 있으므로(`robots_filter.go:156-164`), 본 change는 nil → 실제 인스턴스로 바꾸는 것만 한다. 호출 빈도 정책은 RobotsFilter 자체가 소유.

### Decision 4: nil 허용 계약 보존

`NewRobotsFilter`는 여전히 nil rateSetter를 허용한다(테스트·embeddable 용도). 본 change는 production 부트스트랩 한 곳을 nil → 실제 인스턴스로 바꾸는 wiring만 변경한다. 단위 테스트 다수가 `bot.NewRobotsFilter(nil)`로 RobotsFilter를 생성하고 있으므로 호환성 유지.

## Risks / Trade-offs

- 호스트 bucket이 새 rate로 교체되면 진행 중인 토큰 잔량이 리셋된다(`SetHostRate`의 정의된 행위, `host_rate_limiter.go:78-`). spec Scenario "Crawl-delay 파싱 및 호스트 rate 갱신"이 이 동작을 의도된 결과로 정의하므로 추가 처리 불필요.
- production에서 RobotsFilter가 `Crawl-delay`가 큰 호스트(예: 10초)를 만나면 그 호스트의 throughput이 의도된 대로 1/N req/sec로 떨어진다(현재는 기본 1 req/sec). 이는 spec이 목표하는 정중한 크롤 행위이며 회귀가 아니다.
- 시그니처 변경이 없어 사용처 일괄 변경 위험은 없다. 단위 테스트는 새 wiring을 fake HostRateSetter로 검증.

## Migration Plan

마이그레이션 없음(코드 wiring만). 무중단 배포 가능.

## Open Questions

- (없음)

## Test Plan

- `apps/api/cmd/bot/pioneer_consumer_builder_test.go`(신규): `buildPioneerConsumer`가 PioneerConsumer의 RobotsFilter에 인자 `rl`과 동일 인스턴스를 전파하는지 fake `HostRateSetter`로 결정적 검증.
- 기존 `apps/api/internal/bot/robots_filter_test.go`의 SetHostRate 회귀 케이스(단위 테스트가 이미 보유): 변경 영향 없음(시그니처/가드 변경 없음).
- 기존 `apps/api/cmd/bot/host_rate_limiter_test.go`·`harvester_consumer_builder_test.go`: 변경 영향 없음.
