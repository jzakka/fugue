## 1. 마이그레이션 스크립트 작성

- [ ] 1.1 `apps/api/internal/db/migrations/`에 `bot_frontier` 테이블 생성 SQL 추가 (컬럼: `normalized_url`, `url`, `host`, `depth`, `score`, `last_fetched_at`, `next_fetch_at`, `fetch_error_count`, `pin_id`, `next_harvest_at`, `harvest_error_count`, `last_updated_at`)
- [ ] 1.2 `normalized_url`에 unique constraint 추가
- [ ] 1.3 nullable 정책 적용: `last_fetched_at`, `pin_id`만 NULL 허용; 나머지는 NOT NULL + 적절한 DEFAULT
- [ ] 1.4 down 마이그레이션(테이블 DROP) 작성

## 2. 인덱스 정의

- [ ] 2.1 Pioneer claim partial index: `WHERE last_fetched_at IS NULL AND fetch_error_count < 5`, 정렬 `(score DESC, next_fetch_at)`
- [ ] 2.2 Harvester claim partial index: `WHERE pin_id IS NULL AND harvest_error_count < 5`, 정렬 `(score DESC, next_harvest_at)`
- [ ] 2.3 `host` 보조 인덱스
- [ ] 2.4 `score` 보조 인덱스 (Pioneer/Harvester partial과 별도)

## 3. sqlc / 모델 스캐폴딩

- [ ] 3.1 `bot_frontier` 테이블에 대한 sqlc 모델 자동 생성을 위한 query 파일에 placeholder query 1개 추가 (예: 단순 `SELECT count(*) FROM bot_frontier`) — claim/enqueue 쿼리는 `scheduler-claim-api` change 범위
- [ ] 3.2 `make sqlc` 또는 동등 명령으로 Go 구조체 생성 확인

## 4. 검증

- [ ] 4.1 로컬 Postgres에 마이그레이션 적용 후 `\d bot_frontier`로 컬럼/제약/인덱스 존재 확인
- [ ] 4.2 EXPLAIN으로 Pioneer claim 쿼리(`WHERE last_fetched_at IS NULL AND fetch_error_count < 5 AND next_fetch_at <= now() ORDER BY score DESC LIMIT 10`)가 partial index를 사용하는지 확인
- [ ] 4.3 EXPLAIN으로 Harvester claim 쿼리(`WHERE pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now() ORDER BY score DESC LIMIT 10`)가 partial index를 사용하는지 확인
- [ ] 4.4 동일 `normalized_url` 두 번 INSERT 시 unique violation 발생 확인
- [ ] 4.5 `openspec validate scheduler-frontier-table --strict` 통과 확인

## 5. 문서 업데이트

- [ ] 5.1 `docs/erd.md`에 `bot_frontier` 테이블 추가 (컬럼/인덱스/제약)
- [ ] 5.2 `docs/architecture.md`에 frontier 도입 배경과 후속 change(`scheduler-claim-api`, `scheduler-retry-backoff`, `scheduler-host-token-bucket`) 로드맵 1단락 추가

## 6. 후속 change를 위한 정리 (본 change 범위 외이지만 메모)

- [ ] 6.1 `apps/api/internal/bot/priority_queue.go` 및 `bfs_queue.go`는 본 change에서 삭제하지 않음을 README/PR 설명에 명시 (`scheduler-claim-api`에서 제거)
- [ ] 6.2 `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue`는 후속 change에서 `URLScheduler`로 rename됨을 PR 설명에 명시
