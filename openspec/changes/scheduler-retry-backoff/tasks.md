## 1. URLScheduler 실패 보고 메서드 정의

- [ ] 1.1 URLScheduler 인터페이스/구현체에 `RecordFetchError(ctx context.Context, key string, errorKind string) error` 메서드를 추가한다. `key`는 `normalized_url`, `errorKind`는 `"http_4xx" | "http_5xx" | "network" | "timeout"` 중 하나.
- [ ] 1.2 동일 형태로 `RecordHarvestError(ctx context.Context, key string, errorKind string) error` 메서드를 추가한다.
- [ ] 1.3 성공 경로(`RecordFetchSuccess` / `RecordHarvestSuccess`)는 **본 change에서 추가하지 않는다**. 성공 시 `fetch_error_count = 0` / `harvest_error_count = 0` reset은 `scheduler-claim-api`의 `SetStatus("fetched")` / `SetStatus("harvested:<pin_id>")`가 담당함을 코드 주석으로 명시한다.
- [ ] 1.4 본 capability가 정의한 backoff/dead 규칙을 위 2개 메서드 외부에서 우회하지 않도록 Pioneer/Harvester 코드 경로를 점검한다. `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼은 SetStatus 또는 RecordXxxError 경로 밖에서 직접 UPDATE되지 않는다.

## 2. Backoff 공식 구현 (Go 측 계산)

- [ ] 2.1 공통 헬퍼 `computeBackoff(errorCountAfter int) time.Duration`를 작성: `base=30s`, `delay = base * (1 << (errorCountAfter - 1))`. `errorCountAfter`는 이번 실패를 반영한 "후"의 값(1..5).
- [ ] 2.2 `applyJitter(delay time.Duration, rng *rand.Rand) time.Duration` 헬퍼: `math/rand`(또는 `crypto/rand` 기반 래퍼)로 `uniform[-0.1, +0.1]` 추출 후 `delay`에 곱해 더한다. 정규분포 금지.
- [ ] 2.3 `nextAt(now time.Time, errorCountAfter int) time.Time`: `now.Add(computeBackoff(errorCountAfter) + applyJitter(...))`. `time.Now()`는 구현체가 inject 가능한 clock 인터페이스로 통해 호출(테스트 용이).
- [ ] 2.4 최대 delay가 `30s * 2^4 = 480s`임을 단위 테스트로 확인. `errorCountAfter > 5`인 입력은 호출되지 않아야 하며, 방어적으로 `errorCountAfter = 5`로 clamp한다.

## 3. RecordFetchError 구현

- [ ] 3.1 `errorKind == "http_4xx"` 분기: 단일 UPDATE로 `fetch_error_count = 5`, `last_updated_at = now()`를 set (공식 **미적용**). `next_fetch_at`은 `time.Now()` 또는 기존 값 유지(dead라 무의미하지만 NOT NULL 유지).
- [ ] 3.2 그 외 `errorKind` 분기: 단일 UPDATE로 `fetch_error_count = fetch_error_count + 1`, `next_fetch_at = $2` (Go가 계산한 `time.Now() + delay + jitter`), `last_updated_at = now()`.
- [ ] 3.3 위 두 UPDATE를 sqlc 쿼리로 작성한다 (`UpdateFetchErrorDead`, `UpdateFetchErrorBackoff`). 호출부 Go 코드에서 `errorKind`에 따라 분기하여 둘 중 하나를 실행.
- [ ] 3.4 `errorKind`가 enum에 없는 값이면 에러 반환(panic 금지). 기본 폴백 없이 명시적으로 거부.

## 4. RecordHarvestError 구현

- [ ] 4.1 `errorKind == "http_4xx"` 분기: `harvest_error_count = 5`, `last_updated_at = now()`.
- [ ] 4.2 그 외 분기: `harvest_error_count = harvest_error_count + 1`, `next_harvest_at = $2` (Go 계산값), `last_updated_at = now()`.
- [ ] 4.3 sqlc 쿼리 작성 (`UpdateHarvestErrorDead`, `UpdateHarvestErrorBackoff`).
- [ ] 4.4 4xx 분기와 기타 분기의 errorKind enum 검증은 fetch 측과 공유 헬퍼로.

## 5. Dead 동작 검증 (cleanup은 범위 밖)

- [ ] 5.1 `RecordFetchError`가 `fetch_error_count`를 5 이상으로 만든 경우 별도 cleanup/삭제를 수행하지 않는지 확인한다.
- [ ] 5.2 `scheduler-claim-api` claim 쿼리가 `fetch_error_count < 5` 조건으로 dead row를 자연스럽게 제외하는지 통합 테스트로 검증한다. 별도 `is_dead` 컬럼 없이 partial index만으로 충분함을 확인.
- [ ] 5.3 Harvester claim도 동일하게 `harvest_error_count < 5`로 dead row를 제외하는지 검증한다.

## 6. 테스트

- [ ] 6.1 단위 테스트: `computeBackoff(1..5)`가 30s, 60s, 120s, 240s, 480s를 반환하는지 검증.
- [ ] 6.2 단위 테스트: `applyJitter`를 고정 delay로 1000회 호출 시 모든 결과가 `[delay*0.9, delay*1.1]` 범위에 속하고, 동일 입력에 항상 같은 값을 반환하지 않음을 검증. 정규분포 꼬리 값이 범위를 벗어나지 않음도 확인(uniform 강제).
- [ ] 6.3 통합 테스트: `errorKind="http_4xx"`로 `RecordFetchError`를 1회 호출하면 `fetch_error_count = 5`로 즉시 set되고, 이후 Pioneer claim이 0건을 반환한다.
- [ ] 6.4 통합 테스트: `errorKind="http_5xx"`로 `RecordFetchError`를 5회 연속 호출하면 각 호출 후 count가 1,2,3,4,5로 증가하고, `next_fetch_at`이 각각 30s/60s/120s/240s/480s (±10%) 범위 내에서 `time.Now()`를 기준으로 갱신되며, 5번째 호출 후에는 Pioneer claim이 0건을 반환한다.
- [ ] 6.5 통합 테스트: `errorKind="network"`, `errorKind="timeout"`도 `http_5xx`와 동일하게 공식 적용됨을 확인.
- [ ] 6.6 통합 테스트: `fetch_error_count = 3`인 row에 `SetStatus("fetched")`(claim-api spec)를 호출하면 `fetch_error_count = 0`이 되고 `last_fetched_at`이 비-NULL이 됨을 본 change 영향 영역 경계로 검증(실제 구현은 claim-api).
- [ ] 6.7 통합 테스트: harvest 측에 대해 6.3, 6.4, 6.5와 동일 시나리오.
- [ ] 6.8 단위 테스트: 알 수 없는 errorKind(`"unknown"`)로 호출 시 에러 반환, row 무변경.

## 7. 문서

- [ ] 7.1 `docs/architecture.md` 또는 scheduler 섹션에 backoff 공식(`30s * 2^(n-1) + ±10% uniform jitter, cap=5`), 4xx 즉시 dead 정책, 계산 위치(Go app, `time.Now()`)를 명시한다.
- [ ] 7.2 운영 노트: dead row 수동 재활성화 SQL 예시(`UPDATE pioneer_frontier SET fetch_error_count = 0, next_fetch_at = now() WHERE url_hash = decode($1, 'hex')`)를 운영 가이드에 추가한다.
