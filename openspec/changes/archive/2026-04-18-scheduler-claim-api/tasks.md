## 1. 패키지 스캐폴딩

- [x] 1.1 `apps/api/internal/scheduler/` 디렉터리 생성
- [x] 1.2 `scheduler.go` 에 `QueueType` enum 정의 — `type QueueType string`, 상수 `QueuePioneer = "pioneer"`, `QueueHarvester = "harvester"`
- [x] 1.3 `scheduler.go` 에 `Status string`, `ErrorKind string` alias 추가 및 `URLScheduler` interface 선언 — 메서드: `Enqueue(queueType QueueType, urls ...string) error`, `Dequeue(queueType QueueType) (url string, err error)`, `SetStatus(key string, status Status, pinIDs []uuid.UUID) error`, `RecordFetchError(key string, errorKind ErrorKind) error`, `RecordHarvestError(key string, errorKind ErrorKind) error`
- [x] 1.4 status 상수 정의 — `StatusFetched Status = "fetched"`, `StatusFetchFailed Status = "fetch_failed"`, `StatusHarvested Status = "harvested"`, `StatusHarvestFailed Status = "harvest_failed"`
- [x] 1.5 errorKind 상수 정의 — `ErrorHTTP4xx ErrorKind = "http_4xx"`, `ErrorHTTP5xx ErrorKind = "http_5xx"`, `ErrorNetwork ErrorKind = "network"`, `ErrorTimeout ErrorKind = "timeout"`
- [x] 1.6 `HostRateLimiter` 인터페이스 의존성 선언 — `Allow(host string) bool` 메서드만 사용. 실제 구현은 `scheduler-host-token-bucket` change가 제공

## 2. sqlc 쿼리 작성

- [x] 2.1 `enqueue_pioneer` — `INSERT INTO pioneer_frontier (normalized_url, url, url_hash, host, depth, score) VALUES (..., 0, 0.0) ON CONFLICT (url_hash) DO NOTHING`. URL batch insert 지원. `depth=0`, `score=0.0` 기본값 적용 (design.md Decision 6 참조)
- [x] 2.2 `enqueue_harvester` — URL 레벨 UPSERT (snapshot_key 제외): `ON CONFLICT (url_hash) DO UPDATE SET next_harvest_at = now(), harvest_error_count = 0, last_updated_at = now() WHERE harvester_frontier.harvested_at IS NULL`. snapshot_key 초기/갱신은 후속 change(`harvester-scheduler-consumer` / `harvester-pin-document`)의 구조화된 enqueue 경로 책임이며 본 change에서 설정하지 않는다
- [x] 2.3 `claim_pioneer_candidates` — `SELECT id, url, host FROM pioneer_frontier WHERE fetch_error_count < 5 AND next_fetch_at <= now() ORDER BY score DESC, next_fetch_at ASC LIMIT :n FOR UPDATE SKIP LOCKED`
- [x] 2.4 `claim_harvester_candidates` — `SELECT id, url, host FROM harvester_frontier WHERE harvested_at IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now() ORDER BY score DESC, next_harvest_at ASC LIMIT :n FOR UPDATE SKIP LOCKED`
- [x] 2.5 `mark_pioneer_in_flight` — `UPDATE pioneer_frontier SET next_fetch_at = now() + interval '10 minutes', last_updated_at = now() WHERE id = :id`
- [x] 2.6 `mark_harvester_in_flight` — `UPDATE harvester_frontier SET next_harvest_at = now() + interval '10 minutes', last_updated_at = now() WHERE id = :id`
- [x] 2.7 `set_status_fetched` — `UPDATE pioneer_frontier SET last_fetched_at = now(), next_fetch_at = now() + interval '365 days', fetch_error_count = 0, last_updated_at = now() WHERE url_hash = :url_hash` (Go 구현체가 `sha256(normalized_url)` 결과를 BYTEA로 바인딩)
- [x] 2.8 `set_status_fetch_failed` — `UPDATE pioneer_frontier SET last_updated_at = now() WHERE url_hash = :url_hash` (error_count/next_fetch_at 갱신은 RecordFetchError 책임이므로 여기서는 마킹만)
- [x] 2.9 `set_status_harvested` — `UPDATE harvester_frontier SET harvested_at = now(), harvest_error_count = 0, last_updated_at = now() WHERE url_hash = :url_hash RETURNING id`
- [x] 2.10 `insert_harvester_frontier_pins` — `INSERT INTO harvester_frontier_pins (frontier_id, pin_id) SELECT :frontier_id, UNNEST(:pin_ids::uuid[])` 또는 동등 batch INSERT
- [x] 2.11 `set_status_harvest_failed` — `UPDATE harvester_frontier SET last_updated_at = now() WHERE url_hash = :url_hash`
- [x] 2.12 RecordFetchError의 http_4xx 경로 — `scheduler-retry-backoff`의 `UpdateFetchErrorDead`(이미 존재)를 재사용. 별도 쿼리를 추가하지 않는다.
- [x] 2.13 RecordFetchError의 transient(http_5xx/network/timeout) 경로 — `scheduler-retry-backoff`의 `UpdateFetchErrorBackoff`(이미 존재: CASE WHEN LEAST(count+1,5)로 5개 candidate timestamp 중 하나 선택)를 재사용. Go 측이 `(count_after=1..5)`에 대한 5개 jittered next_fetch_at을 계산해 한 번의 UPDATE로 전달.
- [x] 2.14 RecordHarvestError의 http_4xx 경로 — `UpdateHarvestErrorDead`(이미 존재) 재사용.
- [x] 2.15 RecordHarvestError의 transient 경로 — `UpdateHarvestErrorBackoff`(이미 존재) 재사용.

**Key lookup 규약**: 모든 `set_status_*` / `record_*_error` 쿼리는 `url_hash` 컬럼으로만 row를 찾는다. `key` 인자는 `normalized_url` 문자열이며, Go 구현체가 SQL 호출 직전에 `sha256(normalized_url)` BYTEA로 변환해 바인딩한다. `normalized_url` 컬럼 자체를 WHERE 절로 사용하지 않는다(BYTEA index가 더 선택적).
- [x] 2.16 `make sqlc` (또는 동등 명령)으로 Go 코드 생성 확인

## 3. URLScheduler Postgres 구현

- [x] 3.1 `postgres_scheduler.go` 에 `URLScheduler` interface 구현체 추가 — `pgxpool.Pool`(또는 프로젝트 표준 DB 핸들), `HostRateLimiter`, 그리고 `scheduler-retry-backoff`가 이미 도입한 `Clock` 의존성을 생성자에서 주입 (테스트에서 `fakeClock`으로 lease 만료를 시뮬레이션할 수 있도록)
- [x] 3.2 `Enqueue(queueType, urls...)` 구현 — queueType에 따라 `enqueue_pioneer` 또는 `enqueue_harvester` 쿼리 호출. 각 URL을 정규화·url_hash 계산 후 batch upsert
- [x] 3.3 `Dequeue(queueType)` 구현 — 내부 `tryClaim(queueType)` 호출 → 성공 시 URL 반환, 실패 시 `time.Sleep(1 * time.Second)` 후 재시도하는 무한 루프. 빈 큐/throttle 구분 없이 동일 sleep
- [x] 3.4 `tryClaim(queueType)` 구현 — **단일 Postgres 트랜잭션** 내에서 다음을 순차 실행: (1) `BEGIN`, (2) `SCHEDULER_CLAIM_CANDIDATE_N`(env, default 1) 만큼 `claim_*_candidates` 쿼리(`FOR UPDATE SKIP LOCKED`)로 row 잠금, (3) **같은 트랜잭션** 안에서 각 row의 host에 대해 `HostRateLimiter.Allow(host)` 호출, (4) 첫 true row에 대해 **같은 트랜잭션** 안에서 `mark_*_in_flight` 쿼리로 `next_*_at = now() + 10min` UPDATE, (5) `COMMIT` 후 winner URL 반환. 모두 false면 `ROLLBACK` 후 "not claimed" 반환. 중요: `FOR UPDATE SKIP LOCKED` 락은 트랜잭션 종료 시에만 해제되므로 SELECT와 mark UPDATE 사이에 **다른 트랜잭션에서의 재claim이 불가능**해야 한다. 두 쿼리를 별도 트랜잭션으로 분리해서는 안 된다.
- [x] 3.5 `SetStatus(key, status, pinIDs)` 구현 — status로 분기:
  - `StatusFetched`: `set_status_fetched` 호출
  - `StatusFetchFailed`: `set_status_fetch_failed` 호출 (마킹만)
  - `StatusHarvested`: 트랜잭션 내에서 `set_status_harvested` (RETURNING frontier_id) → `insert_harvester_frontier_pins(frontier_id, pinIDs)` 순차 실행. `pinIDs`가 비어 있으면 INSERT 스킵
  - `StatusHarvestFailed`: `set_status_harvest_failed` 호출
  - 알 수 없는 status: 에러 반환
- [x] 3.6 `RecordFetchError(key, errorKind)` 구현 — errorKind로 분기:
  - `ErrorHTTP4xx`: `record_fetch_error_http_4xx` 호출 (즉시 error_count=5)
  - `ErrorHTTP5xx` / `ErrorNetwork` / `ErrorTimeout`: 현재 error_count를 읽어 backoff 공식(`scheduler-retry-backoff`의 공식, 본 change 범위에서는 placeholder로 `now()` 또는 stub)으로 next_fetch_at 계산 후 `record_fetch_error_transient` 호출
  - 알 수 없는 errorKind: 에러 반환
- [x] 3.7 `RecordHarvestError(key, errorKind)` 구현 — 3.6과 동일 구조, harvester 테이블 대상
- [x] 3.8 알 수 없는 `key`(존재하지 않는 url_hash)에 대한 SetStatus/RecordXxxError 처리 — UPDATE의 RowsAffected 확인하여 0이면 warn 로그, panic 금지

## 4. 단위 테스트

- [x] 4.1 `Enqueue` 멱등성 테스트 — 동일 URL 두 번 enqueue 후 frontier에 row 1개만 존재 (pioneer/harvester 양쪽)
- [x] 4.2 `Enqueue` batch 테스트 — 가변인자 다중 URL 등록, 일부가 conflict여도 나머지 성공
- [x] 4.3 `Enqueue` harvester UPSERT 테스트 — `harvested_at IS NOT NULL`인 row에 대한 재enqueue는 no-op, `harvested_at IS NULL`인 row는 snapshot_key·next_harvest_at 갱신
- [x] 4.4 `Dequeue` linearizability 테스트 — goroutine 2개가 동일 QueueType으로 동시 호출 시 동일 url_hash가 두 번 반환되지 않음
- [x] 4.5 `Dequeue` block-on-empty 테스트 — 빈 frontier에서 호출 시 즉시 반환하지 않고, 별도 goroutine이 enqueue하면 약 1~2초 이내 반환
- [x] 4.6 `Dequeue` host throttle 테스트 — `HostRateLimiter.Allow`가 항상 false를 반환하도록 mock하면 Dequeue가 block, true로 전환되면 다음 폴링에서 반환
- [x] 4.7 `Dequeue` top N 후보 테스트 — `SCHEDULER_CLAIM_CANDIDATE_N=3`으로 설정, head row host만 throttle 시 두 번째 row가 claim되는지 확인
- [x] 4.8 `Dequeue` Pioneer/Harvester enum 분리 테스트 — 같은 URL이 pioneer만 claimable한 상태에서 `QueueHarvester` Dequeue는 block, `QueuePioneer`는 즉시 반환
- [x] 4.9 `Dequeue` in-flight marker 테스트 — claim 직후 동일 row가 partial index에서 제외(`next_*_at > now()`)됨을 확인
- [x] 4.10 `Dequeue` lease 만료 회수 테스트 — `scheduler-retry-backoff`의 `Clock` interface를 scheduler 구현체에 주입하고, `tryClaim`의 in-flight marker 계산을 `clock.Now() + leaseDuration`으로 바꾼다. 테스트는 `fakeClock`을 11분 앞당긴 뒤 두 번째 Dequeue를 호출하여 동일 row가 재claim되는지 확인 (실제 10분 대기 없이)
- [x] 4.11 `SetStatus("fetched")` 후 동일 row가 Pioneer Dequeue에서 제외(`next_fetch_at = now() + 365 days`) 확인
- [x] 4.12 `SetStatus("harvested", pinIDs)` 원자성 테스트 — `harvester_frontier.harvested_at` 갱신과 `harvester_frontier_pins` INSERT가 동일 트랜잭션에서 실행, 중간 실패 시 둘 다 롤백
- [x] 4.13 `SetStatus("harvested", [])` 빈 pinIDs 테스트 — harvested_at만 갱신되고 pins INSERT는 스킵
- [x] 4.14 `RecordFetchError("http_4xx")` 테스트 — `fetch_error_count`가 즉시 5로 설정, partial index에서 제외
- [x] 4.15 `RecordFetchError("http_5xx"|"network"|"timeout")` 테스트 — `fetch_error_count`가 1 증가, `next_fetch_at`가 갱신 (backoff 공식 stub은 허용)
- [x] 4.16 Consumer 호출 규약 테스트 — 실패 시 `SetStatus(fetch_failed, nil)` + `RecordFetchError(errorKind)` 둘 다 호출 시 error_count 증가가 1회만 발생하는지 확인(SetStatus가 count 증가시키지 않음)
- [x] 4.17 `SetStatus`/`RecordXxxError`에 알 수 없는 key 전달 시 panic 없이 warn 로깅
- [x] 4.18 알 수 없는 status 또는 errorKind 전달 시 에러 반환

## 5. 통합 테스트

- [x] 5.1 docker-compose Postgres 인스턴스에서 `scheduler-frontier-table` 마이그레이션 적용 후 위 단위 테스트 통과
- [x] 5.2 EXPLAIN으로 `claim_pioneer_candidates` 쿼리가 pioneer 쪽 partial index를 사용하는지 확인 (DECISIONS §1의 `WHERE fetch_error_count < 5 ORDER BY score DESC, next_fetch_at ASC`)
- [x] 5.3 EXPLAIN으로 `claim_harvester_candidates` 쿼리가 harvester 쪽 partial index를 사용하는지 확인 (DECISIONS §1의 `WHERE harvested_at IS NULL AND harvest_error_count < 5 ORDER BY score DESC, next_harvest_at ASC`)
- [x] 5.4 두 프로세스(또는 goroutine 풀) 동시 Dequeue 부하 테스트 — claim 중복 없음, host throttle 준수, lease 회수 정상 동작

## 6. 의사코드 정리 (호출부 미수정 원칙 유지)

- [x] 6.1 `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 타입 옆에 `// Deprecated: use scheduler.URLScheduler` 주석 추가
- [x] 6.2 `URLPriorityQueue.Dequeue(string)` 호출 부분에 후속 change에서 `URLScheduler.Dequeue(QueuePioneer|QueueHarvester)` 등으로 교체될 예정임을 주석으로 표기
- [x] 6.3 본 change에서는 Pioneer/Harvester worker 진입점(`Run()`) 코드를 변경하지 않음을 PR 설명에 명시

## 7. 문서

- [x] 7.1 `docs/architecture.md`의 bot 섹션에 `URLScheduler` interface 1단락 추가 (메서드, QueueType, linearizable, block-on-empty, host throttle 통합 요약)
- [x] 7.2 후속 change 로드맵에 `harvester-scheduler-consumer`, Pioneer 측 호출부 마이그레이션, `scheduler-retry-backoff`, `scheduler-host-token-bucket` 연결 명시

## 8. 검증

- [x] 8.1 `openspec validate scheduler-claim-api --strict` 통과
- [x] 8.2 `go test ./apps/api/internal/scheduler/...` 통과
- [x] 8.3 PR 설명에 본 change가 contract 정의 + Postgres 구현체 + 테스트만 포함하며, Pioneer/Harvester 호출부 마이그레이션은 후속 change임을 명시
