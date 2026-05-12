## 1. `pioneer_frontier` 마이그레이션

- [x] 1.1 `apps/api/internal/db/migrations/`에 `pioneer_frontier` 테이블 생성 SQL 추가 (컬럼: `id`, `normalized_url`, `url`, `url_hash`, `host`, `depth`, `score`, `last_fetched_at`, `next_fetch_at`, `fetch_error_count`, `last_updated_at`)
- [x] 1.2 컬럼 타입/NULL/DEFAULT를 design.md Decision 2 표 그대로 적용 (`score DOUBLE PRECISION`, `url_hash BYTEA`, `next_fetch_at DEFAULT now()` 등)
- [x] 1.3 `UNIQUE(url_hash)` 제약 및 `CONSTRAINT pioneer_frontier_url_hash_len_chk CHECK (octet_length(url_hash) = 32)` 길이 제약 추가
- [x] 1.4 Partial index 생성: `CREATE INDEX pioneer_frontier_claimable_idx ON pioneer_frontier (score DESC, next_fetch_at ASC) WHERE fetch_error_count < 5;`
- [x] 1.5 `pioneer_frontier`의 DROP 문장을 3.2의 단일 down 마이그레이션 스크립트에 포함되도록 정의 (개별 down 파일은 생성하지 않음)

## 2. `harvester_frontier` 마이그레이션

- [x] 2.1 `apps/api/internal/db/migrations/`에 `harvester_frontier` 테이블 생성 SQL 추가 (컬럼: `id`, `normalized_url`, `url`, `url_hash`, `host`, `snapshot_key`, `score`, `harvested_at`, `next_harvest_at`, `harvest_error_count`, `last_updated_at`)
- [x] 2.2 컬럼 타입/NULL/DEFAULT를 design.md Decision 2 표 그대로 적용
- [x] 2.3 `UNIQUE(url_hash)` 제약 및 `CONSTRAINT harvester_frontier_url_hash_len_chk CHECK (octet_length(url_hash) = 32)` 길이 제약 추가
- [x] 2.4 Partial index 생성: `CREATE INDEX harvester_frontier_claimable_idx ON harvester_frontier (score DESC, next_harvest_at ASC) WHERE harvested_at IS NULL AND harvest_error_count < 5;`
- [x] 2.5 `harvester_frontier`의 DROP 문장을 3.2의 단일 down 마이그레이션 스크립트에 포함되도록 정의 (개별 down 파일은 생성하지 않음)

## 3. `harvester_frontier_pins` 조인 테이블 마이그레이션

- [x] 3.1 `harvester_frontier_pins` 테이블 생성 SQL 추가 (`frontier_id BIGINT NOT NULL REFERENCES harvester_frontier(id) ON DELETE CASCADE`, `pin_id UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE`, `PRIMARY KEY (frontier_id, pin_id)`) — `pins.id`가 UUID이므로 `pin_id`도 UUID
- [x] 3.2 down 마이그레이션을 단일 파일에 통합 작성: `DROP TABLE harvester_frontier_pins; DROP TABLE harvester_frontier; DROP TABLE pioneer_frontier;` 순서 (FK 제약상 조인 테이블을 먼저 삭제해야 하며, 1.5/2.5의 개별 DROP 문은 이 단일 스크립트로 통합되어야 한다)

## 4. sqlc / 모델 스캐폴딩

- [x] 4.1 `pioneer_frontier`, `harvester_frontier`, `harvester_frontier_pins` 세 테이블에 대한 sqlc 모델 자동 생성을 위한 query 파일에 placeholder query 각 1개씩 추가 (예: 단순 `SELECT count(*) FROM pioneer_frontier`) — claim/enqueue/fanout 쿼리는 `scheduler-claim-api` change 범위
- [x] 4.2 `make sqlc` 또는 동등 명령으로 Go 구조체 생성 확인

## 5. 검증

재현 스크립트: `apps/api/db/verify/000026_frontier_tables_verify.sql` (5.1~5.8 동시 실행)

- [x] 5.1 로컬 Postgres에 마이그레이션 적용 후 `\d pioneer_frontier`, `\d harvester_frontier`, `\d harvester_frontier_pins`로 컬럼/제약/인덱스 존재 확인
- [x] 5.2 EXPLAIN으로 Pioneer claim 쿼리(`SELECT ... FROM pioneer_frontier WHERE fetch_error_count < 5 AND next_fetch_at <= now() ORDER BY score DESC, next_fetch_at ASC LIMIT 10`)가 `url_hash` unique index가 아닌 `pioneer_frontier_claimable_idx` (partial index)를 사용하는지 확인
- [x] 5.3 EXPLAIN으로 Harvester claim 쿼리(`SELECT ... FROM harvester_frontier WHERE harvested_at IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now() ORDER BY score DESC, next_harvest_at ASC LIMIT 10`)가 `url_hash` unique index가 아닌 `harvester_frontier_claimable_idx` (partial index)를 사용하는지 확인
- [x] 5.4 동일 `url_hash`로 두 번 INSERT 시 unique violation 발생 확인 (두 frontier 테이블 각각)
- [x] 5.5 `harvester_frontier` row 삭제 시 관련 `harvester_frontier_pins` row가 CASCADE로 삭제되는지 확인
- [x] 5.6 UPSERT 무효화 검증: `harvested_at IS NOT NULL`인 기존 row에 `INSERT ... ON CONFLICT (url_hash) DO UPDATE ... WHERE harvester_frontier.harvested_at IS NULL`을 실행했을 때 row가 변경되지 않는지 확인
- [x] 5.7 UPSERT 갱신 검증: `harvested_at IS NULL`인 기존 row에 동일 UPSERT를 실행했을 때 `snapshot_key`, `next_harvest_at`, `harvest_error_count`가 갱신되는지 확인
- [x] 5.8 `url_hash` 길이 검증: 32바이트가 아닌 BYTEA 값을 INSERT 시 CHECK 제약으로 거부되는지 확인
- [x] 5.9 `openspec validate scheduler-frontier-table --strict` 통과 확인

## 6. 문서 업데이트

- [x] 6.1 `docs/erd.md`에 `pioneer_frontier`, `harvester_frontier`, `harvester_frontier_pins` 세 테이블 추가 (컬럼/인덱스/제약/관계)
- [x] 6.2 `docs/architecture.md`에 frontier 도입 배경, Pioneer→Harvester fanout 다이어그램, 후속 change(`scheduler-claim-api`, `scheduler-retry-backoff`, `scheduler-host-token-bucket`) 로드맵 1단락 추가

## 7. 후속 change를 위한 정리 (본 change 범위 외이지만 메모)

- [x] 7.1 `apps/api/internal/bot/priority_queue.go` 및 `bfs_queue.go`는 본 change에서 삭제하지 않음을 PR 설명에 명시 (`scheduler-claim-api`에서 제거)
- [x] 7.2 `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue`는 후속 change에서 `URLScheduler`로 rename됨을 PR 설명에 명시
- [x] 7.3 `next_fetch_at` / `next_harvest_at`이 in-flight lease marker를 겸한다는 규약(10분)을 `scheduler-claim-api` 착수 시 재확인
