## 1. 패키지 스캐폴딩

- [ ] 1.1 `apps/api/internal/scheduler/` 디렉터리 생성 (또는 합의 시 `apps/api/internal/bot/scheduler/`)
- [ ] 1.2 `scheduler.go` 에 `URLScheduler` interface 선언 (메서드: `Enqueue(urls ...string)`, `Dequeue(cond queryCondition) string`, `SetStatus(key string, status string)`)
- [ ] 1.3 `queryCondition` 타입 결정 — design.md Decision 4의 옵션 1~3 중 PR 리뷰로 확정. 기본값은 옵션 3(미리 정의된 const `PioneerClaimable`, `HarvesterClaimable`)으로 시작
- [ ] 1.4 `pioneerCondition`, `harvesterCondition` 헬퍼/상수 정의 — Pioneer는 `last_fetched_at IS NULL AND fetch_error_count < 5 AND next_fetch_at <= now()`, Harvester는 `pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now()`

## 2. sqlc 쿼리 작성

- [ ] 2.1 `enqueue` — `INSERT INTO bot_frontier (...) VALUES (...) ON CONFLICT (normalized_url) DO NOTHING`. URL list batch insert 지원
- [ ] 2.2 `claim_pioneer` — `SELECT ... WHERE last_fetched_at IS NULL AND fetch_error_count < 5 AND next_fetch_at <= now() ORDER BY score DESC, next_fetch_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`
- [ ] 2.3 `claim_harvester` — `SELECT ... WHERE pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now() ORDER BY score DESC, next_harvest_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`
- [ ] 2.4 `set_fetched` — `UPDATE bot_frontier SET last_fetched_at = now(), last_updated_at = now() WHERE normalized_url = $1`
- [ ] 2.5 `set_fetch_failed` — `UPDATE bot_frontier SET fetch_error_count = fetch_error_count + 1, next_fetch_at = now(), last_updated_at = now() WHERE normalized_url = $1` (backoff 곡선은 후속 change)
- [ ] 2.6 `set_harvested` — `UPDATE bot_frontier SET pin_id = $2, last_updated_at = now() WHERE normalized_url = $1`
- [ ] 2.7 `set_harvest_failed` — `UPDATE bot_frontier SET harvest_error_count = harvest_error_count + 1, next_harvest_at = now(), last_updated_at = now() WHERE normalized_url = $1`
- [ ] 2.8 `make sqlc` (또는 동등 명령)으로 Go 코드 생성 확인

## 3. URLScheduler Postgres 구현

- [ ] 3.1 `postgres_scheduler.go` 에 `URLScheduler` interface 구현체 추가 — `pgxpool.Pool`(또는 프로젝트 표준 DB 핸들) 의존성 주입
- [ ] 3.2 `Enqueue(urls ...string)` 구현 — 각 URL을 정규화한 뒤 batch upsert. 정규화 로직은 기존 helper 재사용 또는 placeholder
- [ ] 3.3 `Dequeue(cond)` 구현 — `tryClaim(cond)` 헬퍼로 단발 SELECT 시도, 0 row 반환 시 `time.Sleep(1 * time.Second)` 후 재시도하는 무한 루프
- [ ] 3.4 `tryClaim` 내부에서 트랜잭션 시작 → `FOR UPDATE SKIP LOCKED` SELECT → 즉시 in-flight marker UPDATE(예: `last_fetched_at = now()` for Pioneer, 또는 design.md의 권고에 따라 처리) → COMMIT → 잠긴 row의 URL 반환
- [ ] 3.5 `SetStatus(key, status)` 구현 — status 문자열을 파싱하여 적절한 sqlc 호출로 분기 (`fetched`, `fetch_failed`, `harvested:<pin_id>`, `harvest_failed`). 알 수 없는 status는 에러 로깅 후 noop
- [ ] 3.6 알 수 없는 `key`(존재하지 않는 normalized_url)에 대한 SetStatus 처리 — UPDATE의 RowsAffected 확인하여 0이면 warn 로그, panic 금지

## 4. 단위 테스트

- [ ] 4.1 `Enqueue` 멱등성 테스트 — 동일 URL 두 번 enqueue 후 frontier에 row 1개만 존재
- [ ] 4.2 `Enqueue` batch 테스트 — 가변인자 다중 URL 등록, 일부가 conflict여도 나머지 성공
- [ ] 4.3 `Dequeue` linearizability 테스트 — goroutine 2개가 동일 조건으로 동시 호출 시 동일 normalized_url이 두 번 반환되지 않음
- [ ] 4.4 `Dequeue` block-on-empty 테스트 — 빈 frontier에서 호출 시 즉시 반환하지 않고, 별도 goroutine이 enqueue하면 1~2초 이내에 반환
- [ ] 4.5 `Dequeue` Pioneer/Harvester 조건 분리 테스트 — 같은 row가 Pioneer 조건엔 잡히고 Harvester 조건엔 안 잡히는 상태(예: fetched but not harvested)에서 정확한 분기 확인
- [ ] 4.6 `SetStatus("fetched")` 후 동일 row가 Pioneer 조건의 다음 Dequeue에서 제외되는지 확인
- [ ] 4.7 `SetStatus("harvested:<pin_id>")` 후 동일 row가 Harvester 조건의 다음 Dequeue에서 제외되는지 확인
- [ ] 4.8 `SetStatus("fetch_failed")` 5회 호출 후 partial index에서 제외되는지 확인
- [ ] 4.9 `SetStatus`에 알 수 없는 key 전달 시 panic 없이 noop/warn 처리되는지 확인

## 5. 통합 테스트

- [ ] 5.1 docker-compose Postgres 인스턴스에서 마이그레이션(`bot_frontier`) 적용 후 위 단위 테스트 통과
- [ ] 5.2 EXPLAIN으로 `claim_pioneer` 쿼리가 `bot_frontier_pioneer_claimable_idx`를 사용하는지 확인
- [ ] 5.3 EXPLAIN으로 `claim_harvester` 쿼리가 `bot_frontier_harvester_claimable_idx`를 사용하는지 확인

## 6. 의사코드 정리 (호출부 미수정 원칙 유지)

- [ ] 6.1 `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 타입 옆에 `// Deprecated: use scheduler.URLScheduler` 주석 추가
- [ ] 6.2 `URLPriorityQueue.Dequeue(string)` 호출 부분(`p.pq.Dequeue("not-visited")` 등)에 후속 change에서 `URLScheduler.Dequeue(PioneerClaimable)` 등으로 교체될 예정임을 주석으로 표기
- [ ] 6.3 본 change에서는 Pioneer/Harvester worker 진입점(`Run()`) 코드를 변경하지 않음을 PR 설명에 명시

## 7. 문서

- [ ] 7.1 `docs/architecture.md`의 bot 섹션에 `URLScheduler` interface 1단락 추가 (메서드, linearizable, block-on-empty, queryCondition 의미 요약)
- [ ] 7.2 후속 change 로드맵에 `harvester-scheduler-consumer`, Pioneer 측 호출부 마이그레이션, `scheduler-retry-backoff`, `scheduler-host-token-bucket` 연결 명시

## 8. 검증

- [ ] 8.1 `openspec validate scheduler-claim-api --strict` 통과
- [ ] 8.2 `go test ./apps/api/internal/scheduler/...` 통과
- [ ] 8.3 PR 설명에 본 change가 contract 정의 + Postgres 구현체 + 테스트만 포함하며, Pioneer/Harvester 호출부 마이그레이션은 후속 change임을 명시
