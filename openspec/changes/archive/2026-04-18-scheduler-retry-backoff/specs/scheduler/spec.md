## ADDED Requirements

### Requirement: URLScheduler는 실패 보고 API RecordFetchError / RecordHarvestError를 제공한다
URLScheduler는 실패 경로를 위한 두 개의 실패 보고 API를 제공해야 한다(SHALL):

- Pioneer가 `key`(= `normalized_url`)에 대한 fetch 실패를 보고하는 `RecordFetchError`.
- Harvester가 동일 형식으로 harvest 실패를 보고하는 `RecordHarvestError`.

두 API의 구체 시그니처(Go 타입, context 파라미터 등)는 `scheduler-claim-api`의 `URLScheduler` interface 정의에 따른다. 본 capability는 실패 보고 시 발생해야 하는 관찰 가능한 행위만 정의한다.

`errorKind`는 다음 enum 중 하나여야 한다(SHALL): `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`. 열거 외 값은 에러를 반환해야 하며(SHALL), row를 변경해서는 안 된다(SHALL NOT).

성공 경로의 `fetch_error_count` / `harvest_error_count` reset은 본 API가 아니라 `scheduler-claim-api`의 `SetStatus`가 담당한다(참조: `scheduler-claim-api` spec의 `SetStatus` 요구사항). 본 capability는 실패 경로만 정의한다.

#### Scenario: fetch 실패 보고
- **WHEN** Pioneer가 `"https://example.com/x"` 키와 `"http_5xx"` errorKind로 `RecordFetchError`를 호출할 때
- **THEN** scheduler가 해당 row에 본 capability가 정의한 backoff 공식을 적용한다.

#### Scenario: harvest 실패 보고
- **WHEN** Harvester가 `"https://example.com/x"` 키와 `"timeout"` errorKind로 `RecordHarvestError`를 호출할 때
- **THEN** scheduler가 해당 row에 동일한 backoff 공식을 적용한다.

#### Scenario: 알 수 없는 errorKind 거부 (fetch)
- **WHEN** `RecordFetchError`가 `"unknown"` errorKind로 호출될 때
- **THEN** API가 에러를 반환하고, 해당 row의 `fetch_error_count` / `next_fetch_at`은 변경되지 않는다.

#### Scenario: 알 수 없는 errorKind 거부 (harvest)
- **WHEN** `RecordHarvestError`가 `"unknown"` errorKind로 호출될 때
- **THEN** API가 에러를 반환하고, 해당 row의 `harvest_error_count` / `next_harvest_at`은 변경되지 않는다.

---

### Requirement: http_4xx 에러는 즉시 dead 처리된다
`errorKind == "http_4xx"`로 `RecordFetchError`가 호출되면 scheduler는 해당 row의 `fetch_error_count`를 **공식 적용 없이 즉시 5로 설정**해야 한다(SHALL). backoff 공식(`30s * 2^n`)은 이 경로에서 적용되지 않아야 한다(SHALL NOT). Harvester 측(`RecordHarvestError`, `harvest_error_count`)도 동일하다(SHALL).

이유: 4xx(404/410/401/403 등)는 재시도해도 회복 가능성이 없는 결정적 실패이므로 5회 재시도 비용을 소비하지 않는다.

4xx 경로에서 `next_fetch_at` / `next_harvest_at`은 갱신되지 않아야 한다(SHALL NOT) — 해당 row는 dead로 전환되어 partial index에서 제외되므로 backoff 타임스탬프는 의미가 없으며, `next_fetch_at` / `next_harvest_at`은 기존 값을 그대로 유지한다. (본 "기존 값 유지" 규칙은 `next_*_at` 컬럼에만 한정된다. `last_updated_at`은 본 capability의 별도 요구사항에 따라 갱신된다.)

#### Scenario: 4xx 첫 호출에 즉시 dead
- **WHEN** `fetch_error_count = 0`인 row에 대해 `"http_4xx"` errorKind로 `RecordFetchError`가 호출될 때
- **THEN** 같은 row의 `fetch_error_count`가 5로 설정되고, 이후 Pioneer claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: 4xx는 backoff 공식을 건너뛴다
- **WHEN** `fetch_error_count = 2`인 row에 대해 `"http_4xx"` errorKind로 `RecordFetchError`가 호출될 때
- **THEN** `fetch_error_count`가 3이 아니라 5로 설정된다(공식의 `+=1` 증가가 아님).

#### Scenario: 4xx는 next_fetch_at을 변경하지 않는다
- **WHEN** 호출 직전 `next_fetch_at = T0`인 row에 대해 `"http_4xx"` errorKind로 `RecordFetchError`가 호출될 때 (`T0`는 보통 `Dequeue`가 설정한 lease marker 값 `T_claim + 10분`이지만 어떤 값이든 무방)
- **THEN** 호출 직후 `next_fetch_at`은 여전히 `T0`이다(dead로 claim되지 않으므로 무의미하지만 기존 값 유지).

#### Scenario: harvest 측 4xx도 동일
- **WHEN** `harvest_error_count = 1`인 row에 대해 `"http_4xx"` errorKind로 `RecordHarvestError`가 호출될 때
- **THEN** `harvest_error_count`가 5로 설정되고, `next_harvest_at`은 기존 값을 유지한다.

---

### Requirement: http_5xx / network / timeout 에러는 exponential backoff 공식을 적용한다
`errorKind`가 `"http_5xx"`, `"network"`, `"timeout"` 중 하나인 실패 보고에 대해 URLScheduler는 대상 row의 `fetch_error_count`(또는 `harvest_error_count`)를 1 증가시키고, `next_fetch_at`(또는 `next_harvest_at`)을 다음 공식으로 갱신해야 한다(SHALL):

```
delay      = 30s * 2^(error_count_after - 1)
jitter     = uniform[-0.1 * delay, +0.1 * delay]   (uniform 분포, 정규분포 아님)
next_*_at  = T_report + delay + jitter
```

여기서 `error_count_after`는 이번 실패를 반영한 후의 카운트 값(1..5)이며, `T_report`는 실패 보고 시점에 워커가 관측한 현재 시각이다. 공식은 단일 UPDATE로 반영되어야 한다(SHALL). `error_count_after` 증가와 `next_*_at` 갱신은 같은 row에서 찢어져서 관측되지 않아야 한다(SHALL NOT).

구현은 `error_count_after`가 5를 초과하지 않도록 보장해야 한다(SHALL). 결과적으로 최대 delay는 `30s * 2^4 = 480s` (8분)이며, `int64` nanosecond 범위 안에서 산술 overflow가 발생하지 않는다.

#### Scenario: 첫 fetch 실패 (non-4xx) backoff
- **WHEN** `fetch_error_count = 0`인 row에 대해 `"http_5xx"` errorKind로 `RecordFetchError`가 호출될 때 (호출 시각 = `T_report`)
- **THEN** `fetch_error_count`가 1로 증가하고, `next_fetch_at`이 `T_report + 30s ± 3s` (30s의 ±10% uniform jitter) 범위로 갱신된다.

#### Scenario: 두 번째 fetch 실패 backoff
- **WHEN** `fetch_error_count = 1`인 row에 대해 `"network"` errorKind로 `RecordFetchError`가 호출될 때 (호출 시각 = `T_report`)
- **THEN** `fetch_error_count`가 2가 되고, `next_fetch_at`이 `T_report + 60s ± 6s` 범위로 갱신된다.

#### Scenario: 네 번째 fetch 실패 backoff
- **WHEN** `fetch_error_count = 3`인 row에 대해 `"timeout"` errorKind로 `RecordFetchError`가 호출될 때 (호출 시각 = `T_report`)
- **THEN** `fetch_error_count`가 4가 되고, `next_fetch_at`이 `T_report + 240s ± 24s` 범위로 갱신된다.

#### Scenario: 다섯 번째 fetch 실패가 dead를 만든다
- **WHEN** `fetch_error_count = 4`인 row에 대해 `"http_5xx"` errorKind로 `RecordFetchError`가 호출될 때 (호출 시각 = `T_report`)
- **THEN** `fetch_error_count`가 5가 되고, `next_fetch_at`이 `T_report + 480s ± 48s` 범위로 갱신되지만, 이후 Pioneer claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: jitter로 인해 동일 조건 보고의 next_fetch_at이 ±10% 경계 내 분산된다
- **WHEN** 동일 `error_count_after`와 동일 `T_report`를 가정한 N회(N ≥ 1000)의 실패 보고를 관찰할 때
- **THEN** 주 조건: 관측된 모든 `next_fetch_at` 값이 `[T_report + 0.9*delay, T_report + 1.1*delay]` 구간 내에 속한다 — 이 경계 조건이 uniform 분포를 강제한다(정규분포였다면 꼬리 샘플이 경계를 초과하여 이 조건을 위반). 보조 조건: 표본에 서로 다른 값이 2개 이상 관측된다(PRNG가 상수 출력이 아님을 확인하는 smoke check).

#### Scenario: harvest 측도 동일 공식 적용
- **WHEN** 동일한 `error_count_after` 값으로 `RecordHarvestError`의 delay 공식을 비교할 때
- **THEN** base(30s), 지수(`2^(n-1)`), jitter 비율(uniform ±10%) 모두 `RecordFetchError`와 동일하다.

---

### Requirement: 재시도 한도 5에 도달한 row는 dead로 취급되어 claim되지 않는다
`pioneer_frontier` row의 `fetch_error_count`가 5 이상이면 Pioneer claim 대상에서 영구적으로 제외되어야 한다(SHALL). `harvester_frontier` row의 `harvest_error_count`가 5 이상이면 Harvester claim 대상에서 영구적으로 제외되어야 한다(SHALL).

dead 상태는 archived change `scheduler-frontier-table`이 도입하여 현재 `openspec/specs/scheduler` spec에 반영된 partial index — pioneer 측은 `fetch_error_count < 5` 조건, harvester 측은 `harvested_at IS NULL AND harvest_error_count < 5` 조건 — 가 자동으로 제외하는 것으로 성립한다. 정확한 partial index 정의는 `openspec/specs/scheduler` spec이 단일 진실 원천(Single Source of Truth)이며 본 capability는 해당 index의 `… < 5` 조건에만 의존한다. 별도 `is_dead` boolean 컬럼이나 별도 상태 플래그를 도입해서는 안 된다(SHALL NOT).

URLScheduler는 dead 상태에 도달한 row에 대해 별도의 cleanup(삭제·아카이브)을 수행하지 않아야 한다(SHALL NOT) — cleanup은 본 capability의 책임이 아니다.

#### Scenario: dead row는 next_fetch_at이 도래해도 claim되지 않는다
- **WHEN** `fetch_error_count = 5`이고 `next_fetch_at <= now()`인 row가 존재할 때
- **THEN** Pioneer가 claim을 시도해도 해당 row는 반환되지 않는다(partial index가 `fetch_error_count < 5` 조건을 요구하므로 해당 row는 index에서 제외되어 있음).

#### Scenario: harvest 측도 동일하게 동작
- **WHEN** `harvest_error_count = 5`인 row가 존재할 때
- **THEN** Harvester claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: 별도 is_dead 컬럼 부재
- **WHEN** `pioneer_frontier` / `harvester_frontier` 스키마를 확인할 때
- **THEN** `is_dead`나 유사한 boolean 컬럼이 존재하지 않으며, dead 판정은 `fetch_error_count >= 5` / `harvest_error_count >= 5` 조건만으로 이루어진다.

#### Scenario: dead row는 frontier에서 삭제되지 않는다
- **WHEN** row가 dead 상태에 도달한 직후 frontier 테이블을 조회할 때
- **THEN** 해당 row가 여전히 테이블에 존재하며, scheduler는 해당 row를 자동으로 삭제하거나 다른 테이블로 이동시키지 않는다.

---

### Requirement: 성공 시 error_count reset은 SetStatus 책임이다
본 capability의 `RecordFetchError` / `RecordHarvestError`는 **실패 경로만** 다루어야 한다(SHALL). `fetch_error_count = 0` / `harvest_error_count = 0` reset은 `scheduler-claim-api` capability가 정의하는 `SetStatus`의 책임이며, 본 capability 메서드는 reset 로직을 중복 구현해서는 안 된다(SHALL NOT). (참조: `scheduler-claim-api` spec의 `SetStatus` 요구사항.)

#### Scenario: RecordFetchError는 fetch_error_count를 감소시키지 않는다
- **WHEN** `fetch_error_count = 3`인 row에 대해 임의의 enum errorKind로 `RecordFetchError`가 호출될 때
- **THEN** 호출 직후 조회된 `fetch_error_count`는 4(`+= 1`) 또는 5(`"http_4xx"`) 둘 중 하나이며, 0 또는 3 이하의 값으로 감소하지 않는다.

#### Scenario: 성공 시 reset은 SetStatus가 담당
- **WHEN** Pioneer가 fetch 성공을 보고할 때
- **THEN** 호출되는 메서드는 `SetStatus(key, "fetched")`(signature는 `scheduler-claim-api` 참조)이며, 이 호출이 base `openspec/specs/scheduler` spec(archived `scheduler-frontier-table`이 도입한 "Pioneer fetch 성공 시" 요구사항)에 따라 `fetch_error_count = 0`과 `last_fetched_at = now()`를 갱신한다. 본 capability의 메서드는 이 경로에 관여하지 않는다.

---

### Requirement: 실패 보고 경로 외부에서 backoff 컬럼을 직접 수정하지 않는다
**런타임 애플리케이션 코드 경로**(Pioneer/Harvester 및 기타 서버 프로세스)는 `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼을 `RecordFetchError` / `RecordHarvestError` / `SetStatus`(claim-api) 외부에서 직접 UPDATE해서는 안 된다(SHALL NOT). 한 번의 실패 보고는 단일 트랜잭션 안에서 일관되게 반영되어야 한다(SHALL).

본 제약은 **운영자의 수동 개입**(psql 세션으로 dead row를 수동 재활성화하는 운영 절차 등)에는 적용되지 않는다. 수동 개입은 런타임 코드 경로가 아니며, 운영 가이드에 기술된 절차를 통해서만 수행된다.

#### Scenario: 실패 보고 한 번의 호출이 단일 UPDATE로 반영된다
- **WHEN** 워커가 fetch 실패를 한 번 scheduler에 보고할 때
- **THEN** 해당 row의 `fetch_error_count`와 `next_fetch_at`이 단일 UPDATE(단일 트랜잭션) 안에서 함께 갱신된다.

#### Scenario: 실패 보고 후 row의 상태는 공식과 정확히 일치한다
- **WHEN** 워커가 한 번의 실패 보고(단일 `RecordFetchError` 또는 `RecordHarvestError` 호출)를 수행한 직후 해당 row를 조회할 때
- **THEN** `fetch_error_count`(또는 `harvest_error_count`)와 `next_fetch_at`(또는 `next_harvest_at`)이 본 spec의 공식과 정확히 일치하는 값을 보이며, 한쪽만 갱신되고 다른 쪽이 이전 값인 중간 상태는 관측되지 않는다.

---

### Requirement: 실패 보고는 last_updated_at을 현재 시각으로 갱신한다
`RecordFetchError` / `RecordHarvestError`는 실패를 반영하는 단일 UPDATE에서 대상 row의 `last_updated_at`을 현재 시각으로 갱신해야 한다(SHALL). 이는 4xx 즉시 dead 경로와 non-4xx backoff 경로 모두에 적용된다(SHALL). 운영 디버깅에서 "마지막으로 상태가 변한 시각"을 신뢰할 수 있도록 하기 위함이다.

#### Scenario: non-4xx 경로에서 last_updated_at 갱신
- **WHEN** `"http_5xx"` errorKind로 `RecordFetchError`가 호출될 때
- **THEN** 같은 UPDATE에서 해당 row의 `last_updated_at`이 호출 시각으로 갱신된다.

#### Scenario: 4xx 경로에서도 last_updated_at 갱신
- **WHEN** `"http_4xx"` errorKind로 `RecordFetchError`가 호출될 때
- **THEN** 같은 UPDATE에서 해당 row의 `last_updated_at`이 호출 시각으로 갱신된다(`fetch_error_count = 5` 설정과 함께).
