## 1. 선행 의존성 확인

- [x] 1.1 `scheduler-frontier-table` change가 반영되어 `pioneer_frontier` / `harvester_frontier` 두 테이블과 Pioneer claim용 partial index(`WHERE fetch_error_count < 5 ORDER BY score DESC, next_fetch_at ASC`)가 존재하는지 확인
- [x] 1.2 `scheduler-claim-api` change가 반영되어 `URLScheduler` 인터페이스에 baseline 5개 메서드 `Dequeue(QueueType)`, `Enqueue(QueueType, urls...)`, `SetStatus(key, status, pinIDs)`, `RecordFetchError(key, errorKind)`, `RecordHarvestError(key, errorKind)` 시그니처가 확정되었는지 확인 (`EnqueueHarvester`는 baseline에 없으며 §2에서 본 change가 추가)
- [x] 1.3 `pioneer-snapshot-storage`가 반영되어 `snapshots/<sha256_hex>/<yyyymmdd>.html.gz` 키 규약과 `SnapshotStore.Put(ctx, normalizedURL, body) error` 저장 API 및 `snapshot.SnapshotKey(normalizedURL, t) string` 키 계산 공개 함수가 제공되는지 확인
- [x] 1.4 `pioneer-link-filter-policy`가 반영되어 `FilterChain` / `DomainFilter` / `ExtensionFilter` / `PathPatternFilter` / `RobotsFilter` / `DedupFilter` 체인 및 `Apply(links) []Link` 메서드가 존재하는지 확인
- [x] 1.5 정식 구현의 참조 모델로 본 change `design.md` Decision 1 pseudo-code(`func (p *Pioneer) Run` 루프)를 사용한다 — `apps/api/fuguebot_pseudo.go`는 부가 참고 자료일 뿐 라인 번호에 의존하지 않는다

## 2. Scheduler 인터페이스 확장 (EnqueueHarvester)

- [x] 2.1 `URLScheduler` 인터페이스에 `EnqueueHarvester(url string, snapshotKey string) error` 메서드를 추가한다
- [x] 2.2 `EnqueueHarvester`에 대응하는 sqlc 쿼리/UPSERT SQL(`ON CONFLICT (url_hash) DO UPDATE ... WHERE harvested_at IS NULL`)을 구현하여 본 change `specs/scheduler/spec.md` ADDED Requirement의 행위 계약(이미 harvested인 row에 대한 no-op, 미완료 row에 대한 snapshot_key/next_harvest_at/harvest_error_count 갱신)을 충족시킨다
- [x] 2.3 baseline `Enqueue(QueueHarvester, urls...)` 경로는 snapshot_key를 여전히 건드리지 않음을 회귀 테스트로 확인한다 (`EnqueueHarvester` 경로와의 분리 유지)

## 3. Pioneer consumer 신규 구현 (feature flag 하)

- [x] 3.1 `apps/api/internal/bot/pioneer_consumer.go`(또는 동등 경로) 생성. 루프 본문을 `Dequeue(QueuePioneer) → fetch → saveSnapshot → extractLinks → FilterChain.Apply → Enqueue(QueuePioneer, filteredURLs...) → EnqueueHarvester(url, snapshotKey) → SetStatus(url, "fetched", nil)` 순으로 구현 (기존 BFS/PriorityQueue 로직 교체). `filteredURLs`는 `FilterChain.Apply` 결과 `[]Link`에서 URL 문자열만 추출한 `[]string`
- [x] 3.2 Pioneer 생성자에서 `URLScheduler`, `SnapshotStore`, `FilterChain`, `LinkExtractor`, `Fetcher`를 주입받도록 시그니처 정리(컴포지션 포인트). 인메모리 큐/visited 필드는 **추가하지 않는다**
- [x] 3.3 `Enqueue(QueuePioneer, filteredURLs...)` 호출 경로: 각 링크별로 normalized URL 계산 → `url_hash = sha256(normalized_url)` 생성 → scheduler가 `pioneer_frontier`에 `INSERT ... ON CONFLICT (url_hash) DO NOTHING`. URL 정규화 규칙(scheme 소문자, default port 제거, query 이름순 정렬 등)은 baseline `bot` spec의 canonicalURL requirement를 SSOT로 따르며, `pioneer-link-filter-policy`/`DedupFilter`가 이를 소비한다
- [x] 3.4 `EnqueueHarvester(url, snapshotKey)` 호출 경로 검증: §2에서 구현된 `EnqueueHarvester` 메서드를 Pioneer consumer가 fetch 성공 직후에 정확히 1회 호출하는지 확인한다. UPSERT SQL 세부(컬럼/가드)는 본 change `specs/scheduler/spec.md` ADDED Requirement가 SSOT이며, 참고 예시는 본 change `design.md` Decision 5를 참조. 이미 harvest된 URL(`harvested_at IS NOT NULL`)은 no-op이어야 함을 통합 테스트로 검증
- [x] 3.5 성공 시 `scheduler.SetStatus(url, "fetched", nil)` 호출. `pinIDs`는 Pioneer에서 `nil`로 고정 (Pin 생성은 Harvester 책임)
- [x] 3.6 실패 시 에러 분류 → `scheduler.SetStatus(url, "fetch_failed", nil)` + `scheduler.RecordFetchError(url, errorKind)` **둘 다** 호출. 호출 누락이 없도록 defer/helper로 묶을 것
- [x] 3.7 `classifyError(err)` helper 구현: HTTP 4xx → `"http_4xx"`, HTTP 5xx → `"http_5xx"`, `net.Error` with `Timeout()==true` → `"timeout"`, 그 외 → `"network"`. snapshot 저장 실패도 `"network"`로 분류
- [x] 3.8 `FilterChain.Apply(links)` 호출 위치: `extractLinks` 직후, `scheduler.Enqueue(QueuePioneer, ...)` 직전. 필터 내용은 수정하지 않고 `pioneer-link-filter-policy`가 구성한 체인을 그대로 호출
- [x] 3.9 루프 sleep/backoff 코드 제거: 빈 큐 폴링은 scheduler 내부 책임이므로 Pioneer 루프에 `time.Sleep` 등을 **추가하지 않음**
- [x] 3.10 feature flag 추가: `BOT_PIONEER_SCHEDULER` 환경변수 (기본 `false`). `true`일 때만 신규 consumer 경로 실행, `false`이면 기존 BFS 경로 유지 — **BFS fallback 병행 가능 기간** 확보

## 4. 인메모리 상태 제거 검증

- [x] 4.1 Pioneer 신규 구현에 URL 큐/스택/슬라이스/채널 상태가 없음을 코드 리뷰 체크리스트에 반영
- [x] 4.2 Pioneer 신규 구현에 visited 맵/사이트 카운터/세션 변수가 없음을 확인
- [x] 4.3 Pioneer가 `pioneer_frontier` / `harvester_frontier` 테이블 컬럼을 직접 UPDATE/INSERT하지 않음을 `grep`/정적 검사로 확인 (모든 접근은 `URLScheduler` 메서드 경유)
- [x] 4.4 Pioneer 코드에 분산 락/advisory lock/mutex 기반 조율 코드가 없음을 확인

## 5. 다중 워커 동작 검증

- [x] 5.1 단일 프로세스 테스트: 신규 Pioneer가 seed URL 하나로 시작하여 `Dequeue → fetch → snapshot → Enqueue(pioneer) + EnqueueHarvester → SetStatus("fetched")` 사이클을 수 회 정상 반복하는지 확인 — `TestPioneerConsumer_SuccessPath`가 fake scheduler로 호출 시퀀스(Enqueue(pioneer) + EnqueueHarvester + SetStatus(fetched), RecordFetchError 미호출)를 검증
- [x] 5.2 복수 인스턴스 테스트: 동일 scheduler에 2개 이상의 Pioneer를 띄웠을 때 동일 URL이 한 번만 처리되는지 검증 (`FOR UPDATE SKIP LOCKED` + host token bucket 재현) — `TestIntegration_Dequeue_ConcurrentClaimsAreDistinct`가 3개 워커가 8개 seeded row에 경쟁할 때 각 URL이 정확히 1회 claim됨을 검증
- [x] 5.3 재시작 복구 테스트: 크롤 도중 Pioneer를 kill했다가 다시 띄우면 frontier 현재 상태에서 즉시 이어받는지 확인 (lease timeout 10분 경과 후 재claim) — `TestIntegration_Dequeue_ExpiredLeaseIsReclaimable`가 `next_fetch_at`을 과거로 되돌려 lease 만료를 시뮬레이션하고 re-claim이 성공함을 검증
- [x] 5.4 교차 사이트 Enqueue 확인: 외부 도메인 링크가 필터를 통과하면 Pioneer가 거르지 않고 `Enqueue(QueuePioneer, ...)`하는지 검증 — `TestPioneerConsumer_CrossSiteEnqueue`가 동일 도메인 + 두 외부 도메인 링크가 모두 Enqueue payload에 포함됨을 검증
- [x] 5.5 실패 경로 테스트: HTTP 404 응답 시 `SetStatus("fetch_failed", nil)` + `RecordFetchError(url, "http_4xx")` 둘 다 호출되는지 검증 (scheduler가 `fetch_error_count = 5`로 즉시 dead 처리) — `TestPioneerConsumer_FetchFailure_HTTP404`가 dual-call과 harvester fanout 미발생을 검증
- [x] 5.6 실패 경로 테스트: 네트워크 timeout 시 `errorKind = "timeout"`으로 기록되고 다음 `next_fetch_at`이 backoff 공식에 따라 계산되는지 검증 — `TestClassifyFetchError_Timeout`이 classification을 검증하고 `TestIntegration_RecordFetchError_NetworkAndTimeoutFormula`가 timeout kind에 대해 30s×2^(n-1) backoff 공식이 적용됨을 DB 레벨에서 검증
- [x] 5.7 재크롤 가드 테스트: 이미 `harvested_at IS NOT NULL`인 URL을 재fetch하면 `EnqueueHarvester` 호출이 no-op이 되는지(snapshot_key/next_harvest_at 미변경) SQL 레벨에서 검증

## 6. 레거시 코드 제거 (배포 전이므로 즉시 일원화)

- [x] 6.1 feature flag(`BOT_PIONEER_SCHEDULER`) 코드 제거 — 프로덕션 배포 전 단계이므로 카나리/병행 운영 단계 없이 신규 경로를 유일 경로로 만든다
- [x] 6.2 `apps/api/internal/bot/priority_queue.go` 제거
- [x] 6.3 ~~`apps/api/internal/bot/bfs_queue.go` 제거~~ — Harvester가 여전히 사용하므로 유지 (Pioneer-only가 아님)
- [x] 6.4 기존 `pioneer.go`의 BFS 본문/visited 맵/세션 카운터 제거 (재사용 헬퍼 `hashURL`/`templatePath`/`urlPathContains`/`hasExcludedExtension`/`isNumeric`은 `url_helpers.go`로 이동, 나머지 Pioneer-only 코드와 `pioneer_test.go`/`pioneer_snapshot_test.go`는 삭제)
- [x] 6.5 (6.1과 통합됨)
- [x] 6.6 제거된 코드를 참조하던 테스트·진단 도구(`show-map` 등) 동작 재확인 — `go build ./...` + `go test ./...` 통과 (375 tests)

## 7. 스펙/문서 정리

- [x] 7.1 `bot` spec의 "BFS로 사이트를 탐색한다 (Pioneer)" requirement가 본 change의 `specs/bot/spec.md` REMOVED delta로 제거되는지 `openspec validate pioneer-scheduler-consumer --strict` 통과 확인
- [x] 7.2 `docs/architecture.md`의 Pioneer-Scheduler 관계 다이어그램/서술 갱신: Pioneer가 scheduler consumer이자 fanout B의 producer(새 링크 → `pioneer_frontier`, 원본+snapshot_key → `harvester_frontier`)임을 명시
- [x] 7.3 AGENTS.md에 Pioneer 동작 모델 한 줄 요약 업데이트 (Dequeue → fetch → snapshot → Enqueue(pioneer) + EnqueueHarvester → SetStatus)
- [x] 7.4 아카이브: 구현 + 레거시 코드 제거가 완료된 시점에 **아카이브 당일 날짜**를 디렉토리 prefix로 사용하여 `openspec/changes/archive/YYYY-MM-DD-pioneer-scheduler-consumer/` 하위로 이동(프로젝트 archive 디렉토리 명명 규칙: 아카이브 시점 기준). `.openspec.yaml:created`의 생성 시점 날짜와 혼동하지 말 것
