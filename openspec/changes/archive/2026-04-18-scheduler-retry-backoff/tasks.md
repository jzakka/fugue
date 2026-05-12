## 1. URLScheduler 실패 보고 메서드 정의

- [x] 1.1 URLScheduler 인터페이스/구현체에 `RecordFetchError` 메서드를 추가한다. 시그니처는 `scheduler-claim-api`의 `URLScheduler` interface 정의(`RecordFetchError(key string, errorKind string) error`)와 정확히 일치해야 한다. `key`는 `normalized_url`, `errorKind`는 `"http_4xx" | "http_5xx" | "network" | "timeout"` 중 하나.
- [x] 1.2 동일 형태로 `RecordHarvestError`를 추가한다. 시그니처는 `scheduler-claim-api`의 `RecordHarvestError(key string, errorKind string) error`와 일치.
- [x] 1.3 성공 경로(`RecordFetchSuccess` / `RecordHarvestSuccess`)는 **본 change에서 추가하지 않는다**. 성공 시 `fetch_error_count = 0` / `harvest_error_count = 0` reset은 `scheduler-claim-api`의 `SetStatus("fetched")` / `SetStatus("harvested", pinIDs)`가 담당함을 코드 주석으로 명시한다.
- [x] 1.4 본 capability가 정의한 backoff/dead 규칙을 위 2개 메서드 외부에서 우회하지 않도록 Pioneer/Harvester **런타임 코드 경로**를 점검한다. `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼은 SetStatus 또는 RecordXxxError 경로 밖에서 직접 UPDATE되지 않는다. (운영자의 수동 psql 개입은 범위 외.)

## 2. Backoff 공식 구현 (Go 측 계산)

- [x] 2.1 공통 헬퍼 `computeBackoff(errorCountAfter int) time.Duration`를 작성: `base=30s`, `delay = base * time.Duration(1 << (errorCountAfter - 1))` (Go에서 `int`→`time.Duration` 명시 캐스트 필요). `errorCountAfter`는 이번 실패를 반영한 "후"의 값(1..5).
- [x] 2.2 `applyJitter(delay time.Duration) time.Duration` 헬퍼를 작성: 내부에서 `math/rand` 기반 PRNG로 `uniform[-0.1, +0.1]` 값을 추출해 `delay`에 곱해 더한다. PRNG 소스는 **scheduler 구현체 생성자 레벨에서 캡슐화**하는 것을 권장하며(design.md 결정), 헬퍼 내부 static PRNG도 허용한다. 어느 쪽이든 시그니처에 PRNG 타입을 노출하지 않는다. 정규분포 금지.
- [x] 2.3 `T_report + delay + jitter` 합성은 scheduler 구현체가 `clock.Now().Add(computeBackoff(n) + jitter(delay))` 형태로 **인라인 수행**한다. 별도 공용 `nextAt` 헬퍼는 두지 않는다(단일 UPDATE 분기에서 5개 후보를 루프로 생성하므로 헬퍼가 루프 안에서만 쓰이게 되어 추출 이득이 없음). `time.Now()`는 `Clock` 인터페이스로, PRNG는 `Jitterer` 함수 타입으로 scheduler 생성자에 주입 가능하여 테스트 결정성을 확보한다.
- [x] 2.4 최대 delay가 `30s * 2^4 = 480s`임을 단위 테스트로 확인. 호출 측(RecordFetchError/RecordHarvestError 구현부)이 `1 <= errorCountAfter <= 5`를 보장한다(spec 요구사항). 헬퍼 `computeBackoff`는 경계 밖 입력에도 안전하게 동작하도록 양방향 clamp를 둔다: `errorCountAfter < 1`은 `1`로, `errorCountAfter > 5`는 `5`로 clamp하여 `1 << (n-1)`의 음수 시프트 panic을 방지한다. 단 이 방어는 호출부 계약 위반의 안전망이며, clamp가 발동하면 경고 로그를 남겨 호출부 버그로 다룬다.

## 3. RecordFetchError 구현

- [x] 3.1 `errorKind == "http_4xx"` 분기: 단일 UPDATE로 `fetch_error_count = 5`, `last_updated_at = now()`를 set (공식 **미적용**). `next_fetch_at`은 **갱신하지 않는다**(기존 값 유지, NOT NULL 제약은 이미 기존 값으로 충족).
- [x] 3.2 그 외 `errorKind` 분기: 단일 UPDATE로 `fetch_error_count = fetch_error_count + 1`, `next_fetch_at = $2`, `last_updated_at = now()`. 호출부는 `nextAt(clock.Now(), newErrorCount)` 결과를 `$2`에 바인딩한다(= `T_report + delay + jitter`).
- [x] 3.3 위 두 UPDATE를 sqlc 쿼리로 작성한다 (`UpdateFetchErrorDead`, `UpdateFetchErrorBackoff`). 호출부 Go 코드에서 `errorKind`에 따라 분기하여 둘 중 하나를 실행.
- [x] 3.4 `errorKind`가 enum에 없는 값이면 에러 반환(panic 금지). 기본 폴백 없이 명시적으로 거부.

## 4. RecordHarvestError 구현

- [x] 4.1 `errorKind == "http_4xx"` 분기: `harvest_error_count = 5`, `last_updated_at = now()`. `next_harvest_at`은 갱신하지 않는다(기존 값 유지).
- [x] 4.2 그 외 분기: `harvest_error_count = harvest_error_count + 1`, `next_harvest_at = $2`, `last_updated_at = now()`. 호출부는 `nextAt(clock.Now(), newErrorCount)` 결과를 `$2`에 바인딩한다(= `T_report + delay + jitter`).
- [x] 4.3 sqlc 쿼리 작성 (`UpdateHarvestErrorDead`, `UpdateHarvestErrorBackoff`).
- [x] 4.4 4xx 분기와 기타 분기의 errorKind enum 검증은 fetch 측과 공유 헬퍼로.

## 5. Dead 동작 검증 (cleanup은 범위 밖)

- [x] 5.1 `RecordFetchError`가 `fetch_error_count`를 5 이상으로 만든 경우 별도 cleanup/삭제를 수행하지 않는지 확인한다.
- [x] 5.2 `scheduler-claim-api` claim 쿼리가 `fetch_error_count < 5` 조건으로 dead row를 자연스럽게 제외하는지 통합 테스트로 검증한다. 별도 `is_dead` 컬럼 없이 partial index만으로 충분함을 확인.
- [x] 5.3 Harvester claim도 동일하게 `harvest_error_count < 5`로 dead row를 제외하는지 검증한다.

## 6. 테스트

- [x] 6.1 단위 테스트: `computeBackoff(1..5)`가 30s, 60s, 120s, 240s, 480s를 반환하는지 검증.
- [x] 6.2 단위 테스트: PRNG 시드를 고정하지 않은 기본 설정에서 `applyJitter`를 고정 delay로 1000회 호출 시 (1) 모든 결과가 `[delay*0.9, delay*1.1]` 범위에 속하고, (2) 표본 분산이 0이 아님(결과가 변동함)을 검증한다. uniform 분포임을 강제하는 경계 검증이다(정규분포 구현은 꼬리 값이 ±10% 경계를 넘을 수 있어 조건 (1)을 위반하게 된다).
- [x] 6.3 통합 테스트: `errorKind="http_4xx"`로 `RecordFetchError`를 1회 호출하면 `fetch_error_count = 5`로 즉시 set되고, 이후 Pioneer claim이 0건을 반환한다.
- [x] 6.4 통합 테스트: `errorKind="http_5xx"`로 `RecordFetchError`를 5회 연속 호출하면 각 호출 후 count가 1,2,3,4,5로 증가하고, `next_fetch_at`이 각각 30s/60s/120s/240s/480s (±10%) 범위 내에서 `time.Now()`를 기준으로 갱신되며, 5번째 호출 후에는 Pioneer claim이 0건을 반환한다.
- [x] 6.5 통합 테스트: `errorKind="network"`, `errorKind="timeout"`도 `http_5xx`와 동일하게 공식 적용됨을 확인.
- [x] 6.6 통합 테스트(`scheduler-claim-api` change 구현이 선행된 이후 수행): `fetch_error_count = 3`인 row에 `SetStatus("fetched")`를 호출하면 base scheduler spec의 성공 경로 요구사항에 따라 `fetch_error_count = 0`이 되고 `last_fetched_at`이 비-NULL이 됨을 본 change 영향 영역 경계로 검증(실제 구현은 claim-api 소관).
    - Covered by `apps/api/internal/scheduler/postgres_scheduler_test.go:TestIntegration_SetStatus_FetchedExcludesFromQueue` (본 change `scheduler-retry-backoff-closeout` 에서 `last_fetched_at` 비-NULL 단언 보강).
- [x] 6.7 통합 테스트: harvest 측에 대해 6.3, 6.4, 6.5와 동일 시나리오.
- [x] 6.8 단위 테스트: `RecordFetchError`와 `RecordHarvestError` 양쪽 모두에 대해 알 수 없는 errorKind(`"unknown"`)로 호출 시 에러 반환, 대응 row의 카운트/`next_*_at` 무변경을 검증한다.

## 7. 문서

- [x] 7.1 `docs/architecture.md` 또는 scheduler 섹션에 backoff 공식(`30s * 2^(n-1) + ±10% uniform jitter, cap=5`), 4xx 즉시 dead 정책, 계산 위치(Go app, `time.Now()`)를 명시한다.
- [x] 7.2 운영 노트: dead row 수동 재활성화 SQL 예시(`UPDATE pioneer_frontier SET fetch_error_count = 0, next_fetch_at = now() WHERE url_hash = decode($1, 'hex')`)를 운영 가이드에 추가한다. `decode($1, 'hex')`는 운영자 psql 세션에서 hex 문자열을 바인딩하기 위한 편의 표기이며, Go 런타임 쿼리는 raw BYTEA 바인딩(`WHERE url_hash = $1`, `$1 = sha256(key)`)을 사용한다는 점을 주석으로 명시.
