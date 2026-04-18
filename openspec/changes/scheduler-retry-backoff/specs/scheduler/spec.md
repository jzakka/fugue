## MODIFIED Requirements

### Requirement: URLScheduler는 실패 보고 메서드 RecordFetchError / RecordHarvestError를 제공한다
URLScheduler는 실패 경로를 위해 다음 두 메서드를 제공해야 한다(SHALL):

- `RecordFetchError(ctx context.Context, key string, errorKind string) error` — Pioneer가 `key`(= `normalized_url`) 에 대한 fetch 실패를 보고한다.
- `RecordHarvestError(ctx context.Context, key string, errorKind string) error` — Harvester가 동일 형식으로 harvest 실패를 보고한다.

`errorKind`는 다음 enum 중 하나여야 한다(SHALL): `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`. 열거 외 값은 에러를 반환해야 하며(SHALL), row를 변경해서는 안 된다(SHALL NOT).

성공 경로의 `fetch_error_count` / `harvest_error_count` reset은 본 메서드가 아니라 `scheduler-claim-api`의 `SetStatus`가 담당한다(참조: `scheduler-claim-api` spec의 `SetStatus` 요구사항). 본 capability는 실패 경로만 정의한다.

#### Scenario: fetch 실패 보고
- **WHEN** Pioneer가 `RecordFetchError(ctx, "https://example.com/x", "http_5xx")`를 호출할 때
- **THEN** scheduler가 해당 row에 본 capability가 정의한 backoff 공식을 적용한다.

#### Scenario: harvest 실패 보고
- **WHEN** Harvester가 `RecordHarvestError(ctx, "https://example.com/x", "timeout")`를 호출할 때
- **THEN** scheduler가 해당 row에 동일한 backoff 공식을 적용한다.

#### Scenario: 알 수 없는 errorKind 거부
- **WHEN** `RecordFetchError(ctx, key, "unknown")`가 호출될 때
- **THEN** 메서드가 에러를 반환하고, 해당 row의 `fetch_error_count` / `next_fetch_at`은 변경되지 않는다.

---

### Requirement: http_4xx 에러는 즉시 dead 처리된다
`errorKind == "http_4xx"`로 `RecordFetchError`가 호출되면 scheduler는 해당 row의 `fetch_error_count`를 **공식 적용 없이 즉시 5로 설정**해야 한다(SHALL). backoff 공식(`30s * 2^n`)은 이 경로에서 적용되지 않아야 한다(SHALL NOT). Harvester 측(`RecordHarvestError`, `harvest_error_count`)도 동일하다(SHALL).

이유: 4xx(404/410/401/403 등)는 재시도해도 회복 가능성이 없는 결정적 실패이므로 5회 재시도 비용을 소비하지 않는다.

#### Scenario: 4xx 첫 호출에 즉시 dead
- **WHEN** `fetch_error_count = 0`인 row에 대해 `RecordFetchError(ctx, key, "http_4xx")`가 호출될 때
- **THEN** 같은 row의 `fetch_error_count`가 5로 설정되고, 이후 Pioneer claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: 4xx는 backoff 공식을 건너뛴다
- **WHEN** `fetch_error_count = 2`인 row에 대해 `RecordFetchError(ctx, key, "http_4xx")`가 호출될 때
- **THEN** `fetch_error_count`가 3이 아니라 5로 설정된다(공식의 `+=1` 증가가 아님).

#### Scenario: harvest 측 4xx도 동일
- **WHEN** `harvest_error_count = 1`인 row에 대해 `RecordHarvestError(ctx, key, "http_4xx")`가 호출될 때
- **THEN** `harvest_error_count`가 5로 설정된다.

---

### Requirement: http_5xx / network / timeout 에러는 exponential backoff 공식을 적용한다
`errorKind`가 `"http_5xx"`, `"network"`, `"timeout"` 중 하나인 실패 보고에 대해 URLScheduler는 대상 row의 `fetch_error_count`(또는 `harvest_error_count`)를 1 증가시키고, `next_fetch_at`(또는 `next_harvest_at`)을 다음 공식으로 갱신해야 한다(SHALL):

```
delay      = 30s * 2^(error_count_after - 1)
jitter     = uniform[-0.1 * delay, +0.1 * delay]   (uniform 분포, 정규분포 아님)
next_*_at  = time.Now() + delay + jitter
```

여기서 `error_count_after`는 이번 실패를 반영한 후의 카운트 값(1..5)이며, 공식은 **Go 애플리케이션에서 `time.Now()` 기준으로 계산**되어 단일 UPDATE로 반영되어야 한다(SHALL). DB의 `now()` / `random()`을 공식 계산에 사용해서는 안 된다(SHALL NOT). 단 `last_updated_at`과 같은 비-backoff 타임스탬프는 DB `now()` 사용 가능.

최대 delay는 `30s * 2^4 = 480s` (8분)이며, `error_count <= 5`가 보장되므로 overflow는 발생하지 않아야 한다(SHALL NOT).

#### Scenario: 첫 fetch 실패 (non-4xx) backoff
- **WHEN** `fetch_error_count = 0`인 row에 대해 `RecordFetchError(ctx, key, "http_5xx")`가 호출될 때
- **THEN** `fetch_error_count`가 1로 증가하고, `next_fetch_at`이 `time.Now() + 30s ± 3s` (30s의 ±10% uniform jitter) 범위로 갱신된다.

#### Scenario: 두 번째 fetch 실패 backoff
- **WHEN** `fetch_error_count = 1`인 row에 대해 `RecordFetchError(ctx, key, "network")`가 호출될 때
- **THEN** `fetch_error_count`가 2가 되고, `next_fetch_at`이 `time.Now() + 60s ± 6s` 범위로 갱신된다.

#### Scenario: 네 번째 fetch 실패 backoff
- **WHEN** `fetch_error_count = 3`인 row에 대해 `RecordFetchError(ctx, key, "timeout")`가 호출될 때
- **THEN** `fetch_error_count`가 4가 되고, `next_fetch_at`이 `time.Now() + 240s ± 24s` 범위로 갱신된다.

#### Scenario: 다섯 번째 fetch 실패가 dead를 만든다
- **WHEN** `fetch_error_count = 4`인 row에 대해 `RecordFetchError(ctx, key, "http_5xx")`가 호출될 때
- **THEN** `fetch_error_count`가 5가 되고, `next_fetch_at`이 `time.Now() + 480s ± 48s` 범위로 갱신되지만, 이후 Pioneer claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: jitter가 uniform 분포에서 표집된다
- **WHEN** 동일 `error_count_after`로 다수의 실패를 보고하여 산출된 jitter 값을 관찰할 때
- **THEN** 표집된 jitter 값들이 `[-0.1*delay, +0.1*delay]` 구간의 **균일 분포**(정규분포 아님)에 따라 분포하며, 두 보고가 항상 동일한 `next_fetch_at`을 갖지 않는다.

#### Scenario: harvest 측도 동일 공식 적용
- **WHEN** 동일한 `error_count_before` 값으로 `RecordHarvestError`의 delay 공식을 비교할 때
- **THEN** base(30s), 지수(`2^(n-1)`), jitter 비율(uniform ±10%) 모두 `RecordFetchError`와 동일하다.

#### Scenario: 계산 위치는 Go app
- **WHEN** `RecordFetchError` 구현의 UPDATE 쿼리를 확인할 때
- **THEN** `next_fetch_at`은 Go 측이 계산한 값(`time.Time` 파라미터)을 받으며, SQL 식 안에서 `now() + interval '...' * random()` 같은 DB 계산을 사용하지 않는다.

---

### Requirement: 재시도 한도 5에 도달한 row는 dead로 취급되어 claim되지 않는다
`pioneer_frontier` row의 `fetch_error_count`가 5 이상이면 Pioneer claim 대상에서 영구적으로 제외되어야 한다(SHALL). `harvester_frontier` row의 `harvest_error_count`가 5 이상이면 Harvester claim 대상에서 영구적으로 제외되어야 한다(SHALL).

dead 상태는 `scheduler-frontier-table`이 정의한 partial index (`WHERE fetch_error_count < 5` / `WHERE harvest_error_count < 5`)가 자동으로 제외하는 것으로 성립한다. 별도 `is_dead` boolean 컬럼이나 별도 상태 플래그를 도입해서는 안 된다(SHALL NOT).

URLScheduler는 dead 상태에 도달한 row에 대해 별도의 cleanup(삭제·아카이브)을 수행하지 않아야 한다(SHALL NOT) — cleanup은 본 capability의 책임이 아니다.

#### Scenario: dead row는 next_fetch_at이 도래해도 claim되지 않는다
- **WHEN** `fetch_error_count = 5`이고 `next_fetch_at <= now()`인 row가 존재할 때
- **THEN** Pioneer가 claim을 시도해도 해당 row는 반환되지 않는다(partial index의 `fetch_error_count < 5` 조건 위배).

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

#### Scenario: RecordFetchError는 reset을 수행하지 않는다
- **WHEN** `RecordFetchError` 구현의 SQL / 코드를 확인할 때
- **THEN** `fetch_error_count = 0`으로 set하는 경로가 존재하지 않는다(오직 `+= 1` 또는 `= 5`만 존재).

#### Scenario: 성공 시 reset은 SetStatus가 담당
- **WHEN** Pioneer가 fetch 성공을 보고할 때
- **THEN** 호출되는 메서드는 `SetStatus(key, "fetched")`이며(`scheduler-claim-api` 참조), 이 호출이 `fetch_error_count = 0`과 `last_fetched_at = now()`를 갱신한다. 본 capability의 메서드는 이 경로에 관여하지 않는다.

---

### Requirement: 실패 보고 경로 외부에서 backoff 컬럼을 직접 수정하지 않는다
Pioneer/Harvester 및 기타 코드 경로는 `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼을 `RecordFetchError` / `RecordHarvestError` / `SetStatus`(claim-api) 외부에서 직접 UPDATE해서는 안 된다(SHALL NOT). 한 번의 실패 보고는 단일 트랜잭션 안에서 일관되게 반영되어야 한다(SHALL).

#### Scenario: 실패 보고 한 번의 호출이 단일 UPDATE로 반영된다
- **WHEN** 워커가 fetch 실패를 한 번 scheduler에 보고할 때
- **THEN** 해당 row의 `fetch_error_count`와 `next_fetch_at`이 단일 UPDATE(단일 트랜잭션) 안에서 함께 갱신된다.

#### Scenario: 워커가 직접 컬럼을 수정하지 않는다
- **WHEN** Pioneer/Harvester 코드 경로를 검토할 때
- **THEN** `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼은 `RecordFetchError` / `RecordHarvestError` / `SetStatus` 외부에서 직접 UPDATE되지 않는다.
