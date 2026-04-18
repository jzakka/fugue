## MODIFIED Requirements

### Requirement: 스케줄러는 호스트별 token bucket으로 politeness를 적용한다
스케줄러는 호스트별 token bucket을 유지하고, 외부 소비자가 특정 호스트의 token 가용성을 질의할 수 있는 `Allow(host) bool` 동작을 제공해야 한다(SHALL). Token bucket은 프로세스별 인메모리 자료구조로 유지되어야 하며(SHALL), 프로세스 간 token 상태를 공유하지 않아야 한다(SHALL NOT).

호스트 키의 형식은 `scheduler-frontier-table`의 `host` 컬럼 값과 동일하다: 호스트명만(포트 제외), 대소문자 원본 유지, `www.` prefix 유지. 정규화 책임은 호출부(Pioneer)에 있으며, scheduler는 전달된 문자열을 그대로 키로 사용한다(SHALL).

claim 시점의 `Allow` 호출 타이밍(예: `SELECT FOR UPDATE SKIP LOCKED` 후 각 후보 row의 host에 대해 `Allow` 검사)은 `scheduler-claim-api` capability의 Claim 프로토콜에서 정의되며, 본 capability는 메서드의 행위 계약만 제공한다.

#### Scenario: token이 있는 호스트에 대한 Allow는 true를 반환한다
- **WHEN** 호출자가 호스트 `H`에 대해 `Allow("H")`를 호출하고 호스트 `H`의 bucket에 token이 1개 이상 남아 있을 때
- **THEN** `Allow`는 `true`를 반환하고 호스트 `H`의 bucket에서 token 1개가 소비된다.

#### Scenario: token이 없는 호스트에 대한 Allow는 false를 반환한다
- **WHEN** 호스트 `H`의 bucket이 token 0개 상태에서 `Allow("H")`가 호출될 때
- **THEN** `Allow`는 `false`를 반환하고 token은 소비되지 않는다.

#### Scenario: 프로세스 재시작 후 token 상태는 초기화된다
- **WHEN** 스케줄러 프로세스가 중단되었다가 재시작될 때
- **THEN** 모든 호스트의 token bucket은 burst 한도까지 가득 찬 초기 상태에서 시작한다(인메모리 상태이므로 보존되지 않는다).

#### Scenario: 두 스케줄러 프로세스의 token 상태는 독립이다
- **WHEN** 동일 호스트 `H`에 대해 두 개의 스케줄러 프로세스가 동시에 실행될 때
- **THEN** 각 프로세스가 독립된 token bucket을 가지며, 한 프로세스의 token 소비가 다른 프로세스의 가용 token에 영향을 주지 않는다.

#### Scenario: 호스트 키는 frontier host 컬럼 값 그대로 사용된다
- **WHEN** 호출자가 `Allow("www.Example.COM")`을 호출하고 scheduler 내부에 동일 문자열 키로 bucket이 존재할 때
- **THEN** 해당 bucket이 조회되며 scheduler는 대소문자/`www.` prefix를 변형하지 않는다.

---

### Requirement: 기본 rate와 burst는 설정 가능하다
스케줄러는 호스트별 기본 token 충전율(rate)과 최대 누적 token 수(burst)를 설정값으로 노출해야 한다(SHALL). 기본값은 호스트당 1 req/sec rate와 burst 5여야 한다(SHALL). 처음 조회되는 호스트의 bucket은 이 기본값으로 lazy 생성되어야 한다(SHALL).

#### Scenario: 기본값으로 새 호스트 bucket이 생성된다
- **WHEN** 처음 본 호스트 `H`에 대해 `Allow("H")`가 호출될 때
- **THEN** 호스트 `H`의 bucket이 rate=1 req/sec, burst=5로 생성되고 첫 `Allow`는 true를 반환한다.

#### Scenario: 운영자가 기본 rate/burst를 변경한다
- **WHEN** 운영자가 설정값 `scheduler.host_default_rate_per_sec`을 0.5로, `scheduler.host_default_burst`를 3으로 변경하고 스케줄러를 재기동할 때
- **THEN** 그 이후 처음 등장하는 호스트의 bucket은 rate=0.5 req/sec, burst=3으로 생성된다.

---

### Requirement: 호스트별 rate/burst를 외부에서 override할 수 있다
스케줄러는 특정 호스트의 rate와 burst를 런타임에 변경할 수 있는 `SetHostRate(host, rate, burst)` 동작을 제공해야 한다(SHALL). 호출 후 즉시 해당 호스트의 bucket이 새 설정으로 동작해야 한다(SHALL). 기존 호스트의 경우 Limiter를 재생성해 새 rate/burst로 교체하고, 미본 호스트의 경우 지정된 설정으로 신규 bucket을 생성해야 한다(SHALL).

#### Scenario: 호스트 override가 즉시 반영된다
- **WHEN** 호출자가 `SetHostRate("example.com", 0.1, 1)`을 호출한 직후 `Allow("example.com")`이 호출될 때
- **THEN** 호스트 `example.com`의 bucket은 rate=0.1 req/sec, burst=1로 동작하며, 연속 `Allow` 호출 시 burst 1 이후로는 false를 반환한다.

#### Scenario: 미본 호스트에 대한 override는 신규 bucket을 생성한다
- **WHEN** 처음 보는 호스트 `new.example.org`에 대해 `SetHostRate("new.example.org", 2.0, 10)`이 호출될 때
- **THEN** 해당 호스트의 bucket이 rate=2.0 req/sec, burst=10으로 생성되며 기본값을 사용하지 않는다.

---

### Requirement: rate/burst 유효성 검사와 안전한 기본값 대체
스케줄러의 `SetHostRate`는 `rate <= 0` 또는 `burst <= 0`인 입력을 받으면 해당 호스트 bucket을 **기본값(1 req/sec, burst 5)으로 대체**하고 `WARN` 레벨 경고 로그를 남겨야 한다(SHALL). 이 경우 에러를 반환하거나 패닉을 발생시키지 않아야 하며(SHALL NOT), 서비스는 중단 없이 계속 동작해야 한다(SHALL). 유효성 대체 후의 `Allow` 동작은 대체된 기본값 bucket 기준으로 수행되어야 한다(SHALL).

#### Scenario: rate가 0이면 기본값으로 대체된다
- **WHEN** 호출자가 `SetHostRate("bad.example.com", 0, 5)`를 호출할 때
- **THEN** `bad.example.com`의 bucket이 rate=1 req/sec, burst=5로 생성되고 `WARN` 로그가 남으며, 호출은 에러 없이 반환된다.

#### Scenario: burst가 음수이면 기본값으로 대체된다
- **WHEN** 호출자가 `SetHostRate("bad.example.com", 0.5, -1)`을 호출할 때
- **THEN** `bad.example.com`의 bucket이 rate=1 req/sec, burst=5로 생성되고 `WARN` 로그가 남는다.

#### Scenario: 유효하지 않은 입력이어도 서비스는 중단되지 않는다
- **WHEN** `SetHostRate`에 `rate=-1`, `burst=0`을 전달한 후 곧바로 `Allow("bad.example.com")`이 호출될 때
- **THEN** `Allow`는 기본값(1 req/sec, burst 5) bucket을 기준으로 정상 판정해 true 또는 false를 반환한다(패닉/에러 없음).

---

### Requirement: robots.txt Crawl-delay를 호스트 rate로 반영한다
스케줄러는 robots.txt에서 추출된 Crawl-delay 값을 호스트의 token bucket rate로 반영할 수 있는 진입점으로 `SetHostRate`를 제공해야 한다(SHALL). 본 capability는 robots.txt 자체의 fetch/파싱을 수행하지 않는다(SHALL NOT). robots.txt 파싱과 `SetHostRate` 호출 타이밍은 `pioneer-link-filter-policy` capability의 책임이다.

#### Scenario: Crawl-delay가 rate로 환산되어 적용된다
- **WHEN** Pioneer가 robots.txt에서 호스트 `slow.example.com`의 Crawl-delay = 10초를 파싱하고 스케줄러에 `SetHostRate("slow.example.com", 0.1, 1)`(=1/10 req/sec)을 호출할 때
- **THEN** 스케줄러는 호스트 `slow.example.com`의 token bucket을 rate=0.1 req/sec, burst=1로 운영한다.

#### Scenario: Crawl-delay가 없는 호스트는 기본값으로 동작한다
- **WHEN** robots.txt에 Crawl-delay 지시가 없거나 robots.txt가 없어서 Pioneer가 `SetHostRate`를 호출하지 않을 때
- **THEN** 해당 호스트의 bucket은 기본 rate(1 req/sec)와 기본 burst(5)로 동작한다.
