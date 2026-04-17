## Why

`apps/api/fuguebot_pseudo.go`의 Pioneer/Harvester는 fetch 실패 시 즉시 3회 in-process 재시도만 수행하고 영구적인 backoff 정보를 남기지 않는다. URLScheduler를 Postgres frontier(`scheduler-frontier-table`) 기반으로 옮긴 후에는 워커가 다수로 분산되므로, 같은 실패 row를 짧은 간격으로 무한히 재claim하지 않도록 **영속적인 exponential backoff 정책**이 필요하다. 또한 일정 횟수 이상 실패한 row는 frontier에서 영원히 dead 처리되어 정상 트래픽을 점유하지 않아야 한다.

## What Changes

- `scheduler` capability에 **재시도/backoff 동작 규칙**을 추가:
  - fetch 실패 시 `next_fetch_at = now() + 30s * 2^fetch_error_count + jitter(±10%)`로 갱신하고 `fetch_error_count`를 1 증가시킨다.
  - harvest 실패 시 동일 공식을 `next_harvest_at`/`harvest_error_count`에 적용한다.
  - fetch 성공 시 `fetch_error_count = 0`으로 리셋한다. harvest 성공 시 `harvest_error_count = 0`으로 리셋한다.
  - `fetch_error_count >= 5`인 row는 Pioneer claim 대상에서 영구 제외(dead)되며, `harvest_error_count >= 5`인 row는 Harvester claim 대상에서 영구 제외된다 (테이블 partial index와 정합).
  - URLScheduler의 에러 보고 경로(SetStatus 또는 동등한 RecordFetchError/RecordHarvestError 호출)가 위 공식을 적용한다.
- 본 change는 정책/공식만 정의하고, dead row의 별도 cleanup이나 알림은 다루지 않는다(out of scope).

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `scheduler`: `scheduler-frontier-table`에서 도입된 `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼의 의미를 구체화하는 동작 규칙(backoff 공식, 성공 시 reset, dead 임계값 5)을 추가한다.

## Impact

- **코드**: URLScheduler 구현체에 fetch/harvest 실패·성공 보고 시 backoff 적용 로직 추가. Pioneer/Harvester는 in-process 3회 재시도 루프 대신 실패를 1회 보고하고 다음 claim을 신뢰한다.
- **DB**: 스키마 변경 없음(`scheduler-frontier-table`이 도입한 컬럼만 사용).
- **운영**: 죽은 도메인/URL이 frontier를 점유하는 시간이 제한된다. 재시도 폭주가 host token bucket과 무관하게 자체적으로 완화된다.
- **관련 change**: `scheduler-frontier-table`(컬럼 도입), `scheduler-claim-api`(claim 인터페이스). 본 change는 이 둘이 정의한 컬럼/인터페이스 위에서 동작 규칙을 채운다.
