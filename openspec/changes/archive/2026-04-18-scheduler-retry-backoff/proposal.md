## Why

`apps/api/fuguebot_pseudo.go`의 Pioneer/Harvester는 fetch 실패 시 즉시 3회 in-process 재시도만 수행하고 영구적인 backoff 정보를 남기지 않는다. URLScheduler를 Postgres frontier(archived change `scheduler-frontier-table`이 도입하여 현재 `openspec/specs/scheduler` spec에 반영된 `pioneer_frontier`/`harvester_frontier`) 기반으로 옮긴 후에는 워커가 다수로 분산되므로, 같은 실패 row를 짧은 간격으로 무한히 재claim하지 않도록 **영속적인 exponential backoff 정책**이 필요하다. 또한 일정 횟수 이상 실패한 row는 frontier에서 영원히 dead 처리되어 정상 트래픽을 점유하지 않아야 한다.

추가로 **에러 종류별 차등 정책**이 필요하다. HTTP 4xx는 재시도해도 회복 가능성이 없는(콘텐츠가 사라지거나 권한 자체가 없는) 결정적 실패이므로 즉시 dead 처리하고, 5xx / network / timeout 등 일시적 실패만 공식에 따라 5회까지 재시도한다.

## What Changes

- `scheduler` capability에 **재시도/backoff 동작 규칙**을 추가:
  - 에러 종류(`errorKind`)를 `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"` 4종으로 표준화한다.
  - fetch 실패 시 `errorKind != "http_4xx"`이면 `next_fetch_at = T_report + 30s * 2^(error_count_after - 1) + jitter(±10%)`로 갱신하고 `fetch_error_count`를 1 증가시킨다. 여기서 `error_count_after = fetch_error_count_before + 1`(본 실패를 반영한 후의 값, 1..5)이며 design.md/spec.md의 수식과 동일하다.
  - `errorKind == "http_4xx"`인 경우 `fetch_error_count = 5`로 즉시 설정하여 backoff 공식을 건너뛰고 dead 처리한다.
  - harvest 실패도 동일한 분기 규칙(4xx 즉시 dead 포함)을 `next_harvest_at`/`harvest_error_count`에 적용한다.
  - 성공 시 error_count reset은 `scheduler-claim-api`의 `SetStatus`가 담당한다(본 change 밖). 본 change의 `RecordFetchError` / `RecordHarvestError`는 실패 경로(non-4xx의 backoff 공식 적용 및 카운트 증가, 또는 4xx 즉시 dead 처리)만 담당한다.
  - `fetch_error_count >= 5`인 row는 Pioneer claim 대상에서 영구 제외(dead)되며, `harvest_error_count >= 5`인 row는 Harvester claim 대상에서 영구 제외된다 — `scheduler-frontier-table`이 정의한 partial index (`… < 5`)가 자동으로 제외하며 별도 `is_dead` 컬럼은 두지 않는다.
- 공식 계산 책임 분리: `next_fetch_at` / `next_harvest_at` 값(= `T_report + delay + jitter`)은 워커 프로세스 측이 계산해 UPDATE 파라미터로 바인딩한다. 공식 밖의 `last_updated_at` 타임스탬프는 DB `now()`를 사용할 수 있다. DB의 `random()`은 어디에서도 사용하지 않는다(테스트/관찰 용이성, 워커 간 시간 기준 통일). 상세 근거와 설계는 `design.md` 참조.
- 본 change는 정책/공식만 정의하고, dead row의 별도 cleanup이나 알림은 다루지 않는다(out of scope).

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `scheduler`: `scheduler-frontier-table`에서 도입된 `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼의 의미를 구체화하는 동작 규칙(에러 종류별 backoff 분기, dead 임계값 5, 4xx 즉시 dead)을 추가한다. 성공 시 reset 규칙은 `scheduler-claim-api`의 `SetStatus`가 담당하며 본 change는 실패 경로만 다룬다.

## Impact

- **코드**: URLScheduler 구현체에 `RecordFetchError(key, errorKind)` / `RecordHarvestError(key, errorKind)` 메서드 추가 및 Go 측 backoff 계산 헬퍼 추가. Pioneer/Harvester는 in-process 3회 재시도 루프 대신 실패를 1회 보고하고 다음 claim을 신뢰한다.
- **DB**: 스키마 변경 없음(`scheduler-frontier-table`이 도입한 컬럼만 사용).
- **운영**: 죽은 도메인/URL이 frontier를 점유하는 시간이 제한된다. 4xx로 판명된 URL은 즉시 dead되어 5회의 재시도 비용을 아낀다.
- **관련 change**: `scheduler-frontier-table`(컬럼 도입), `scheduler-claim-api`(claim 인터페이스 및 `SetStatus` 성공 경로). 본 change는 이 둘이 정의한 컬럼/인터페이스 위에서 실패 경로 동작 규칙을 채운다.
