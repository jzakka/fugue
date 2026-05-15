## ADDED Requirements

### Requirement: Pioneer 부트스트랩은 RobotsFilter에 HostRateLimiter를 wire한다

Pioneer 워커의 엔트리포인트(`apps/api/cmd/bot`)는 FilterChain의 RobotsFilter를 생성할 때, 동일 워커 인스턴스의 scheduler가 사용하는 `*scheduler.HostRateLimiter`와 **같은 인스턴스**를 `HostRateSetter` 인자로 전달해야 한다(SHALL). Pioneer 부트스트랩은 RobotsFilter의 `HostRateSetter` 인자로 `nil`이나 별개 인스턴스를 전달해서는 안 된다(SHALL NOT).

본 Requirement는 기존 Requirement `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다`의 Scenario "Crawl-delay 파싱 및 호스트 rate 갱신"·"캐시 TTL 내 중복 호출 방지"가 production pioneer 워커에서 enforce되도록 보장하는 wiring 계약이다. 기존 Requirement의 SHALL 본문과 4개 Scenarios의 의미는 변경하지 않는다.

#### Scenario: Pioneer 워커 부트스트랩이 RobotsFilter와 scheduler에 동일한 host rate limiter 인스턴스를 전파한다

- **WHEN** Pioneer 워커가 `runPioneerConsumer`로 부팅되어 PioneerConsumer를 조립할 때
- **THEN** FilterChain 안의 RobotsFilter는 scheduler가 dequeue 시점에 host bucket을 조회하는 인스턴스와 동일한 `*scheduler.HostRateLimiter`를 `HostRateSetter`로 보유한다. RobotsFilter가 새 호스트의 robots.txt 파싱 후 `SetHostRate`를 호출하면 그 변경이 같은 워커의 다음 dequeue부터 호스트 token bucket에 즉시 반영된다.

#### Scenario: Pioneer 워커 부트스트랩이 RobotsFilter에 nil을 전달하지 않는다

- **WHEN** Pioneer 워커가 production 부트스트랩 경로로 PioneerConsumer를 조립할 때
- **THEN** FilterChain 안의 RobotsFilter는 nil이 아닌 `HostRateSetter`를 보유한다. RobotsFilter가 `Crawl-delay: N`을 정상 파싱하면 `SetHostRate(host, 1/N, 1)`가 실제로 호출되어 기존 Requirement `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다`의 Scenario "Crawl-delay 파싱 및 호스트 rate 갱신"이 production에서 관찰된다.
