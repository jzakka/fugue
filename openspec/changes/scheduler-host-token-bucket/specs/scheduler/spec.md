## ADDED Requirements

### Requirement: 스케줄러는 호스트별 token bucket으로 politeness를 적용한다
스케줄러는 URL claim 시점에 대상 URL의 호스트(`bot_frontier.host`)에 대응하는 token bucket을 검사하여, 사용 가능한 token이 있을 때만 해당 URL을 dequeue해야 한다(SHALL). token bucket은 프로세스별 인메모리 자료구조로 유지되어야 하며(SHALL), 프로세스 간 token 상태를 공유하지 않아야 한다(SHALL NOT).

#### Scenario: token이 충분한 호스트는 즉시 claim된다
- **WHEN** 스케줄러가 호스트 `H`의 URL을 claim 시도하고 호스트 `H`의 bucket에 token이 1개 이상 남아 있을 때
- **THEN** 해당 URL이 dequeue되고 호스트 `H`의 bucket에서 token 1개가 소비된다.

#### Scenario: 프로세스 재시작 후 token 상태는 초기화된다
- **WHEN** 스케줄러 프로세스가 중단되었다가 재시작될 때
- **THEN** 모든 호스트의 token bucket은 burst 한도까지 가득 찬 초기 상태에서 시작한다(인메모리 상태이므로 보존되지 않는다).

#### Scenario: 두 스케줄러 프로세스의 token 상태는 독립이다
- **WHEN** 동일 호스트 `H`에 대해 두 개의 스케줄러 프로세스가 동시에 실행될 때
- **THEN** 각 프로세스가 독립된 token bucket을 가지며, 한 프로세스의 token 소비가 다른 프로세스의 가용 token에 영향을 주지 않는다.

---

### Requirement: 호스트의 token이 부족하면 다른 호스트의 후보로 fallback한다
스케줄러가 frontier에서 가져온 claim 후보 중 어떤 후보의 호스트 bucket에 token이 없으면, 그 후보를 건너뛰고 같은 후보 집합에서 다른 호스트의 후보를 시도해야 한다(SHALL). token이 가용한 첫 후보를 claim해야 한다(SHALL).

#### Scenario: 최상위 score 후보가 blocked이면 차상위 후보가 claim된다
- **WHEN** 스케줄러가 score 내림차순으로 후보 N개(예: 16)를 가져왔고, 최상위 후보의 호스트 bucket이 비어 있고, 차상위 후보의 호스트 bucket에는 token이 있을 때
- **THEN** 차상위 후보가 claim되고 최상위 후보는 다음 dequeue 호출에서 다시 평가된다.

#### Scenario: 동일 호스트로 연속된 후보 중 burst 초과분은 건너뛴다
- **WHEN** 후보 집합의 상위 K개가 모두 동일 호스트 `H`이고 호스트 `H`의 bucket에 token이 J개(J < K) 남아 있을 때
- **THEN** 상위 J개의 후보는 claim되고 나머지 K-J개 후보는 이 dequeue 사이클에서 건너뛰어진다.

---

### Requirement: 모든 후보가 blocked이면 짧게 sleep 후 재시도한다
스케줄러가 가져온 후보 N개가 모두 호스트 bucket에 의해 blocked이면, 짧은 시간 sleep 후 frontier를 다시 조회해야 한다(SHALL). sleep 시간은 설정 가능해야 하며(SHALL), 기본값은 100ms이고 상한은 1초여야 한다(SHALL).

#### Scenario: 모든 후보가 blocked이면 sleep한다
- **WHEN** 스케줄러가 가져온 후보 N개의 모든 호스트 bucket이 token 부족 상태일 때
- **THEN** 스케줄러는 설정된 sleep 시간(기본 100ms)만큼 대기한 뒤 frontier를 재조회한다.

#### Scenario: sleep 후 token이 충전되어 후보가 claim된다
- **WHEN** sleep 직후 호스트 `H`의 bucket이 token을 1개 이상 회복했을 때
- **THEN** 그 다음 frontier 재조회 사이클에서 호스트 `H`의 후보가 claim된다.

#### Scenario: sleep 시간은 상한을 초과하지 않는다
- **WHEN** 운영자가 sleep 설정값을 1초보다 큰 값으로 지정할 때
- **THEN** 스케줄러는 sleep 시간을 1초로 캡(cap)한다.

---

### Requirement: 기본 rate와 burst는 설정 가능하다
스케줄러는 호스트별 기본 token 충전율(rate)과 최대 누적 token 수(burst)를 설정값으로 노출해야 한다(SHALL). 기본값은 호스트당 1 req/sec rate와 burst 5여야 한다(SHALL). 신규 호스트의 bucket은 이 기본값으로 생성되어야 한다(SHALL).

#### Scenario: 기본값으로 새 호스트 bucket이 생성된다
- **WHEN** 처음 본 호스트 `H`의 URL이 claim 후보로 평가될 때
- **THEN** 호스트 `H`의 bucket이 rate=1 req/sec, burst=5로 생성된다.

#### Scenario: 운영자가 기본 rate/burst를 변경한다
- **WHEN** 운영자가 설정값 `scheduler.host_default_rate_per_sec`을 0.5로, `scheduler.host_default_burst`를 3으로 변경하고 스케줄러를 재기동할 때
- **THEN** 그 이후 처음 등장하는 호스트의 bucket은 rate=0.5 req/sec, burst=3으로 생성된다.

---

### Requirement: 호스트별 rate/burst를 외부에서 override할 수 있다
스케줄러는 특정 호스트의 rate와 burst를 런타임에 변경할 수 있는 인터페이스(예: `SetHostRate(host, rate, burst)`)를 제공해야 한다(SHALL). 호출 후 즉시 해당 호스트의 bucket이 새 설정으로 동작해야 한다(SHALL).

#### Scenario: 호스트 override가 즉시 반영된다
- **WHEN** Pioneer가 `SetHostRate("example.com", 0.1, 1)`을 호출한 직후 스케줄러가 `example.com`의 URL을 claim 시도할 때
- **THEN** 호스트 `example.com`의 bucket은 rate=0.1 req/sec, burst=1로 동작한다.

#### Scenario: 미본 호스트에 대한 override는 신규 bucket을 생성한다
- **WHEN** 처음 보는 호스트 `new.example.org`에 대해 `SetHostRate`가 호출될 때
- **THEN** 해당 호스트의 bucket이 지정된 rate/burst로 생성되며 기본값을 사용하지 않는다.

---

### Requirement: robots.txt Crawl-delay를 호스트 rate로 반영한다
스케줄러는 robots.txt에서 추출된 Crawl-delay 값을 호스트의 token bucket rate로 반영할 수 있는 경로를 제공해야 한다(SHALL). 본 capability는 robots.txt 자체의 fetch/파싱은 수행하지 않으며(SHALL NOT), Pioneer 측이 파싱한 결과를 `SetHostRate`로 전달하는 통합 지점만 정의한다(SHALL).

#### Scenario: Crawl-delay가 rate로 환산되어 적용된다
- **WHEN** Pioneer가 robots.txt에서 호스트 `slow.example.com`의 Crawl-delay = 10초를 파싱하고 스케줄러에 `SetHostRate("slow.example.com", 0.1, 1)`(=1/10 req/sec)을 호출할 때
- **THEN** 스케줄러는 호스트 `slow.example.com`의 token bucket을 rate=0.1 req/sec, burst=1로 운영한다.

#### Scenario: Crawl-delay가 없는 호스트는 기본값으로 동작한다
- **WHEN** robots.txt에 Crawl-delay 지시가 없거나 robots.txt가 없어서 Pioneer가 `SetHostRate`를 호출하지 않을 때
- **THEN** 해당 호스트의 bucket은 기본 rate(1 req/sec)와 기본 burst(5)로 동작한다.
