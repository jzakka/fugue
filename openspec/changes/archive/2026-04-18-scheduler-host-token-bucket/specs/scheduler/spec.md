## ADDED Requirements

### Requirement: 스케줄러는 호스트별 token bucket으로 politeness를 적용한다
스케줄러는 호스트별 token bucket을 유지하고, 외부 소비자가 특정 호스트의 token 가용성을 질의할 수 있는 허용-여부 질의 동작을 제공해야 한다(SHALL). 처음 보는 호스트에 대한 허용-여부 질의는 기본 rate/burst로 bucket을 lazy 생성한 뒤 판정해야 한다(SHALL). Token bucket은 프로세스별 인메모리 자료구조로 유지되어야 하며(SHALL), 프로세스 간 token 상태를 공유하지 않아야 한다(SHALL NOT).

호스트 키의 형식은 `scheduler-frontier-table`의 `host` 컬럼 값과 동일하다: 호스트명만(포트 제외), 대소문자 원본 유지, `www.` prefix 유지. 정규화 책임은 호출부(Pioneer)에 있으며, scheduler는 전달된 문자열을 그대로 키로 사용한다(SHALL).

claim 시점의 허용-여부 질의 호출 타이밍(예: `SELECT FOR UPDATE SKIP LOCKED` 후 각 후보 row의 host에 대한 검사)은 `scheduler-claim-api` capability의 Claim 프로토콜에서 정의되며, 본 capability는 동작의 행위 계약만 제공한다.

#### Scenario: token이 있는 호스트에 대한 허용-여부 질의는 true를 반환한다
- **WHEN** 호출자가 호스트 `H`에 대해 허용-여부 질의를 호출하고 호스트 `H`의 bucket에 token이 1개 이상 남아 있을 때
- **THEN** 허용-여부 질의는 `true`를 반환하고 호스트 `H`의 bucket에서 token 1개가 소비된다.

#### Scenario: token이 없는 호스트에 대한 허용-여부 질의는 false를 반환한다
- **WHEN** 호스트 `H`의 bucket이 token 0개 상태에서 허용-여부 질의가 호출될 때
- **THEN** 허용-여부 질의는 `false`를 반환하고 token은 소비되지 않는다.

#### Scenario: 프로세스 재시작 후 token 상태는 초기화된다
- **WHEN** 스케줄러 프로세스가 중단되었다가 재시작될 때
- **THEN** 모든 호스트의 token bucket은 burst 한도까지 가득 찬 초기 상태에서 시작한다(인메모리 상태이므로 보존되지 않는다).

#### Scenario: 두 스케줄러 프로세스의 token 상태는 독립이다
- **WHEN** 동일 호스트 `H`에 대해 두 개의 스케줄러 프로세스가 동시에 실행될 때
- **THEN** 각 프로세스가 독립된 token bucket을 가지며, 한 프로세스의 token 소비가 다른 프로세스의 가용 token에 영향을 주지 않는다.

#### Scenario: 호스트 키는 frontier host 컬럼 값 그대로 사용된다
- **WHEN** 호출자가 `www.Example.COM` 호스트에 대해 허용-여부 질의를 호출하고 scheduler 내부에 동일 문자열 키로 bucket이 존재할 때
- **THEN** 해당 bucket이 조회되며 scheduler는 대소문자/`www.` prefix를 변형하지 않는다.

---

### Requirement: 기본 rate와 burst는 설정 가능하다
스케줄러는 호스트별 기본 token 충전율(rate)과 최대 누적 token 수(burst)를 설정값으로 노출해야 한다(SHALL). 운영자가 별도 설정하지 않았을 때의 공장 기본값은 호스트당 1 req/sec rate와 burst 5여야 한다(SHALL). 처음 조회되는 호스트의 bucket은 운영자 설정 기본값(미설정 시 공장 기본값)으로 lazy 생성되어야 한다(SHALL).

#### Scenario: 공장 기본값으로 새 호스트 bucket이 생성된다
- **WHEN** 운영자가 별도 설정을 하지 않은 상태에서 처음 본 호스트 `H`에 대해 허용-여부 질의가 호출될 때
- **THEN** 호스트 `H`의 bucket이 rate=1 req/sec, burst=5로 생성되고 첫 허용-여부 질의는 true를 반환한다.

#### Scenario: 운영자가 기본 rate/burst를 변경한다
- **WHEN** 운영자가 설정값 `scheduler.host_default_rate_per_sec`을 0.5로, `scheduler.host_default_burst`를 3으로 변경하고 스케줄러를 재기동할 때
- **THEN** 그 이후 처음 등장하는 호스트의 bucket은 rate=0.5 req/sec, burst=3으로 생성된다.

---

### Requirement: 호스트별 rate/burst를 외부에서 override할 수 있다
스케줄러는 특정 호스트의 rate와 burst를 런타임에 변경할 수 있는 호스트 rate/burst 설정 동작을 제공해야 한다(SHALL). 호출 후 즉시 해당 호스트의 bucket이 새 설정으로 동작해야 한다(SHALL). 기존 호스트의 경우 새 rate/burst로 교체하고, 처음 보는 호스트의 경우 지정된 설정으로 신규 bucket을 생성해야 한다(SHALL).

#### Scenario: 호스트 override가 즉시 반영된다
- **WHEN** 호출자가 호스트 `example.com`에 rate=0.1, burst=1로 호스트 rate/burst 설정 동작을 호출한 직후 허용-여부 질의가 호출될 때
- **THEN** 호스트 `example.com`의 bucket은 rate=0.1 req/sec, burst=1로 동작하며, 연속 허용-여부 질의 시 burst 1 이후로는 false를 반환한다.

#### Scenario: 처음 보는 호스트에 대한 override는 신규 bucket을 생성한다
- **WHEN** 처음 보는 호스트 `new.example.org`에 대해 rate=2.0, burst=10으로 호스트 rate/burst 설정 동작이 호출될 때
- **THEN** 해당 호스트의 bucket이 rate=2.0 req/sec, burst=10으로 생성되며 운영자 기본값/공장 기본값을 사용하지 않는다.

---

### Requirement: rate/burst 유효성 검사와 안전한 기본값 대체
스케줄러의 호스트 rate/burst 설정 동작은 `rate <= 0` 또는 `burst <= 0`인 입력을 받으면 해당 호스트 bucket을 **현재 운영자 설정 기본값(`scheduler.host_default_rate_per_sec`, `scheduler.host_default_burst`)**으로 대체하고 `WARN` 레벨 경고 로그를 남겨야 한다(SHALL). 운영자가 별도 설정을 하지 않은 경우 공장 기본값(1 req/sec, burst 5)이 적용된다(SHALL). 이 경우 에러를 반환하거나 패닉을 발생시키지 않아야 하며(SHALL NOT), 서비스는 중단 없이 계속 동작해야 한다(SHALL). 유효성 대체 후의 허용-여부 질의 동작은 대체된 기본값 bucket 기준으로 수행되어야 한다(SHALL).

#### Scenario: rate가 0이면 운영자 기본값으로 대체된다
- **WHEN** 운영자가 별도 설정을 하지 않은 상태에서 호출자가 호스트 `bad.example.com`에 rate=0, burst=5로 호스트 rate/burst 설정 동작을 호출할 때
- **THEN** `bad.example.com`의 bucket이 공장 기본값(rate=1 req/sec, burst=5)으로 생성되고 `WARN` 로그가 남으며, 호출은 에러 없이 반환된다.

#### Scenario: burst가 음수이면 운영자가 변경한 기본값으로 대체된다
- **WHEN** 운영자가 `scheduler.host_default_rate_per_sec=0.5`, `scheduler.host_default_burst=3`으로 변경한 환경에서 호출자가 호스트 `bad.example.com`에 rate=0.5, burst=-1로 호스트 rate/burst 설정 동작을 호출할 때
- **THEN** `bad.example.com`의 bucket이 운영자 기본값(rate=0.5 req/sec, burst=3)으로 생성되고 `WARN` 로그가 남는다.

#### Scenario: 유효하지 않은 입력이어도 서비스는 중단되지 않는다
- **WHEN** 호스트 rate/burst 설정 동작에 rate=-1, burst=0을 전달한 후 곧바로 `bad.example.com` 호스트에 대해 허용-여부 질의가 호출될 때
- **THEN** 허용-여부 질의는 운영자 기본값(미설정 시 공장 기본값) bucket을 기준으로 정상 판정해 true 또는 false를 반환한다(패닉/에러 없음).

---

### Requirement: robots.txt Crawl-delay를 호스트 rate로 반영한다
스케줄러는 robots.txt에서 추출된 Crawl-delay 값을 호스트의 token bucket rate로 반영할 수 있는 진입점으로 호스트 rate/burst 설정 동작을 제공해야 한다(SHALL). 본 capability는 robots.txt 자체의 fetch/파싱을 수행하지 않는다(SHALL NOT). robots.txt 파싱과 호스트 rate/burst 설정 동작 호출 타이밍은 `pioneer-link-filter-policy` capability의 책임이다.

#### Scenario: Crawl-delay가 rate로 환산되어 적용된다
- **WHEN** Pioneer가 robots.txt에서 호스트 `slow.example.com`의 Crawl-delay = 10초를 파싱하고 스케줄러에 rate=0.1, burst=1(=1/10 req/sec)로 호스트 rate/burst 설정 동작을 호출할 때
- **THEN** 스케줄러는 호스트 `slow.example.com`의 token bucket을 rate=0.1 req/sec, burst=1로 운영한다.

#### Scenario: 호스트 rate/burst 설정 동작이 호출되지 않은 호스트는 기본값으로 동작한다
- **WHEN** scheduler가 호스트 `H`에 대해 호스트 rate/burst 설정 동작을 한 번도 호출받지 않은 상태에서 `H`에 대한 허용-여부 질의가 호출될 때
- **THEN** 호스트 `H`의 bucket은 운영자 설정 기본값(미설정 시 공장 기본값인 rate=1 req/sec, burst=5)으로 lazy 생성되어 동작한다.

---

### Requirement: token bucket 검사를 비활성화할 수 있다
스케줄러는 호스트별 token bucket 검사를 전역적으로 비활성화할 수 있는 운영 설정을 제공해야 한다(SHALL). 비활성화된 상태에서는 모든 호스트에 대한 허용-여부 질의가 항상 `true`를 반환해야 하며(SHALL), 비활성화된 상태 동안 어떤 호스트의 bucket 내부 상태(token 잔량, 생성 시각)도 변경하지 않아야 한다(SHALL NOT). 이 설정은 운영 중 문제 발생 시 즉시 롤백 수단으로 사용된다.

#### Scenario: 비활성화 시 허용-여부 질의는 항상 true를 반환한다
- **WHEN** token bucket 검사 비활성화 설정이 켜진 상태에서 임의 호스트 `H`에 대해 허용-여부 질의가 호출될 때
- **THEN** 호스트 `H`의 bucket 상태(빈/가득/없음)와 무관하게 허용-여부 질의는 `true`를 반환한다.

#### Scenario: 비활성화 상태에서 토큰 소비가 발생하지 않는다
- **WHEN** 활성화 상태에서 호스트 `H`의 bucket이 burst 한도까지 가득 찬 후, 비활성화 상태로 전환되어 동일 호스트 `H`에 대해 연속 100회 허용-여부 질의가 호출되고, 다시 활성화 상태로 전환된 직후 호스트 `H`에 대해 허용-여부 질의가 호출될 때
- **THEN** 비활성 기간 100회 모두 `true`를 반환하며, 활성화 직후 호스트 `H`의 bucket은 여전히 burst 한도까지 가득 찬 상태이다(비활성 동안 token이 소비되지 않음).
