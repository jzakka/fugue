## Why

`openspec/specs/bot/spec.md` Requirement `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다`(L750-768)는 RobotsFilter가 robots.txt의 `Crawl-delay: N`을 파싱한 뒤 `scheduler-host-token-bucket` capability의 호스트 rate/burst 설정 동작(`SetHostRate`)을 호출해 호스트의 rate를 `1/N` req/sec, burst `1`로 갱신해야 한다(SHALL)고 명시한다. `openspec/specs/scheduler/spec.md` Requirement `robots.txt Crawl-delay를 호스트 rate로 반영한다`(L297-306)는 그 호출의 책임 소재가 `pioneer-link-filter-policy` capability라고 못박는다.

production pioneer 부트스트랩(`apps/api/cmd/bot/main.go:475-509` `runPioneerConsumer`)은 FilterChain을 구성할 때 `bot.NewRobotsFilter(nil)`로 `HostRateSetter` 인자를 **명시적 nil**로 전달한다. `apps/api/internal/bot/robots_filter.go:159-164`의 `if entry.crawlDelay != nil && *entry.crawlDelay > 0 && f.rateSetter != nil` 가드 때문에 `f.rateSetter == nil`인 production에서는 `SetHostRate`가 **단 한 번도 호출되지 않는다**. 결과: robots.txt의 `Crawl-delay`가 정상 파싱되어도 scheduler의 HostRateLimiter는 그 호스트에 대해 lazy 생성된 기본 bucket(rate=1 req/sec, burst=5)을 그대로 사용해, 위 두 SHALL이 production에서 enforce되지 않는다.

본 change는 wiring 누락만 한정해서 닫는다. RobotsFilter의 파싱 로직과 가드, HostRateLimiter의 `SetHostRate` 의미·검증 분기는 변경하지 않는다.

## What Changes

- production pioneer 부트스트랩이 PioneerConsumer의 FilterChain을 구성할 때, scheduler에 전달한 것과 **동일 인스턴스**의 `*scheduler.HostRateLimiter`를 RobotsFilter에 함께 wire한다.
- 이 wiring이 외부 코드에서 결정적으로 관찰될 수 있도록 `runPioneerConsumer`의 의존 조립을 surface 한 군데로 모아 단위 테스트를 추가한다.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `bot`: 기존 Requirement `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다`가 production pioneer 워커 부트스트랩에서 enforce되도록, 보조 Requirement "Pioneer 부트스트랩은 RobotsFilter에 HostRateLimiter를 wire한다"를 1건 추가한다(`ADDED Requirements`). 기존 Requirement의 SHALL 본문과 4개 Scenarios는 변경하지 않는다.

## Impact

- 영향 코드: `apps/api/cmd/bot/main.go`의 `runPioneerConsumer` 한 함수(`buildHostRateLimiter` 결과를 지역 변수로 추출해 sched와 RobotsFilter 두 곳에 동일 인스턴스 전달).
- 인터페이스/시그니처 변경 없음. `scheduler.HostRateLimiter`는 `SetHostRate(host string, ratePerSec float64, burst int)` 메서드로 `bot.HostRateSetter` 인터페이스를 이미 구조적으로 만족.
- 운영 지표: robots.txt에 `Crawl-delay`가 명시된 호스트에서만 변화. 그런 호스트의 throughput이 의도된 대로 감소(spec 의도된 행위). `Crawl-delay`가 없는 호스트는 기본 bucket(1 req/sec, 5 burst) 그대로 유지(Scenario "Crawl-delay 미지정 시 기본 rate 유지" 보존).
- DB/마이그레이션/외부 의존 없음. harvester 부트스트랩과 별도 endpoint는 손대지 않는다.
- 진행 중인 change `fix-scheduler-host-rate-limiter-config-wiring`는 `buildHostRateLimiter`가 받는 Config 값을 변경하는 영역으로, 본 change(`runPioneerConsumer`에서 그 결과를 추출해 RobotsFilter에도 넘기는 wiring)와는 범위가 겹치지 않는다.
