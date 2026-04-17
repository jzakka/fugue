## 1. URLScheduler 보고 메서드 정의

- [ ] 1.1 URLScheduler 인터페이스/구현체에 `RecordFetchSuccess(ctx, normalizedURL)`, `RecordFetchError(ctx, normalizedURL)` 메서드를 추가한다 (이름은 변경 가능; 기존 `SetStatus` hook을 확장해도 됨).
- [ ] 1.2 동일 형태로 `RecordHarvestSuccess(ctx, normalizedURL, pinID)`, `RecordHarvestError(ctx, normalizedURL)` 메서드를 추가한다.
- [ ] 1.3 본 capability가 정의한 backoff/reset 규칙을 위 4개 메서드 외부에서 우회하지 않도록 Pioneer/Harvester 코드 경로를 점검한다.

## 2. Backoff 공식 구현

- [ ] 2.1 공통 헬퍼 `computeBackoff(errorCountBefore int) time.Duration`를 작성: `base=30s`, `delay = base * 2^errorCountBefore`.
- [ ] 2.2 `applyJitter(delay time.Duration) time.Duration` 헬퍼: 균일 분포 `[-0.1, +0.1]` * delay 적용.
- [ ] 2.3 `RecordFetchError`가 단일 UPDATE로 `fetch_error_count = fetch_error_count + 1`, `next_fetch_at = now() + computeBackoff(fetch_error_count_before) + jitter`, `last_updated_at = now()`를 반영하는 sqlc 쿼리를 작성한다.
- [ ] 2.4 `RecordHarvestError`도 동일 형태로 `harvest_error_count`/`next_harvest_at`에 적용하는 sqlc 쿼리를 작성한다.

## 3. 성공 시 reset

- [ ] 3.1 `RecordFetchSuccess` UPDATE 쿼리에 `fetch_error_count = 0`, `last_fetched_at = now()`를 포함한다.
- [ ] 3.2 `RecordHarvestSuccess` UPDATE 쿼리에 `harvest_error_count = 0`, `pin_id = $1`을 포함한다.

## 4. Dead 동작 검증 (cleanup은 범위 밖)

- [ ] 4.1 `RecordFetchError`가 `fetch_error_count`를 5 이상으로 만든 경우 별도 cleanup/삭제를 수행하지 않는지 확인한다.
- [ ] 4.2 Pioneer claim 쿼리(scheduler-claim-api에서 정의)가 `fetch_error_count < 5` 조건으로 dead row를 자연스럽게 제외하는지 통합 테스트로 검증한다.
- [ ] 4.3 Harvester claim도 동일하게 `harvest_error_count < 5`로 dead row를 제외하는지 검증한다.

## 5. 테스트

- [ ] 5.1 단위 테스트: `errorCountBefore = 0..4`에 대해 `computeBackoff`가 30s, 60s, 120s, 240s, 480s를 반환하는지 검증.
- [ ] 5.2 단위 테스트: `applyJitter`가 1000회 호출 시 모두 `[delay*0.9, delay*1.1]` 범위에 속하고, 동일 입력에 항상 같은 값을 반환하지 않음을 검증.
- [ ] 5.3 통합 테스트: `RecordFetchError`를 5회 연속 호출하면 `fetch_error_count = 5`가 되고, 이후 Pioneer claim이 0건을 반환한다.
- [ ] 5.4 통합 테스트: `fetch_error_count = 3`인 row에 `RecordFetchSuccess`를 호출하면 `fetch_error_count = 0`이 되고 `last_fetched_at`이 비-NULL이 된다.
- [ ] 5.5 통합 테스트: harvest 측에 대해 5.3, 5.4와 동일한 시나리오를 수행한다.

## 6. 문서

- [ ] 6.1 `docs/architecture.md` 또는 scheduler 섹션에 backoff 공식(`30s * 2^n + ±10% jitter, cap=5`)과 dead 정책을 명시한다.
- [ ] 6.2 운영 노트: dead row 수동 재활성화 SQL 예시(`UPDATE bot_frontier SET fetch_error_count = 0, next_fetch_at = now() WHERE normalized_url = $1`)를 운영 가이드에 추가한다.
