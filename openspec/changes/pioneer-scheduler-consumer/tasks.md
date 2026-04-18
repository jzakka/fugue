## 1. 선행 의존성 확인

- [ ] 1.1 `scheduler-frontier-table` change가 반영되어 `pioneer_frontier` / `harvester_frontier` 두 테이블과 Pioneer claim용 partial index(`WHERE fetch_error_count < 5 ORDER BY score DESC, next_fetch_at ASC`)가 존재하는지 확인
- [ ] 1.2 `scheduler-claim-api` change가 반영되어 `URLScheduler` 인터페이스에 `Dequeue(QueueType)`, `Enqueue(QueueType, urls)`, `EnqueueHarvester(url, snapshotKey)`, `SetStatus(key, status, pinIDs)`, `RecordFetchError(key, errorKind)` 시그니처가 확정되었는지 확인
- [ ] 1.3 `pioneer-snapshot-storage`가 반영되어 `snapshots/<sha256_hex>/<yyyymmdd>.html.gz` 키 규약과 `SnapshotStore.Save(url, html) (snapshotKey string, err error)` API가 제공되는지 확인
- [ ] 1.4 `pioneer-link-filter-policy`가 반영되어 `FilterChain` / `DomainFilter` / `ExtensionFilter` / `PathPatternFilter` / `RobotsFilter` / `DedupFilter` 체인 및 `Apply(links) []Link` 메서드가 존재하는지 확인
- [ ] 1.5 `apps/api/fuguebot_pseudo.go` Pioneer.Run(라인 33-68)을 정식 구현의 참조 모델로 재확인

## 2. Pioneer consumer 신규 구현 (feature flag 하)

- [ ] 2.1 `apps/api/internal/bot/pioneer_consumer.go`(또는 동등 경로) 생성. 루프 본문을 `Dequeue(QueuePioneer) → fetch → saveSnapshot → extractLinks → FilterChain.Apply → Enqueue(QueuePioneer, filtered) → EnqueueHarvester(url, snapshotKey) → SetStatus(url, "fetched", nil)` 순으로 구현 (기존 BFS/PriorityQueue 로직 교체)
- [ ] 2.2 Pioneer 생성자에서 `URLScheduler`, `SnapshotStore`, `FilterChain`, `LinkExtractor`, `Fetcher`를 주입받도록 시그니처 정리(컴포지션 포인트). 인메모리 큐/visited 필드는 **추가하지 않는다**
- [ ] 2.3 `Enqueue(QueuePioneer, filteredLinks)` 호출 경로: 각 링크별로 normalized URL 계산 → `url_hash = sha256(normalized_url)` 생성 → scheduler가 `pioneer_frontier`에 `INSERT ... ON CONFLICT (url_hash) DO NOTHING`. url_hash 계산은 `pioneer-link-filter-policy`의 canonicalURL 규칙(scheme 소문자, default port 제거, query 이름순 정렬)을 따른 뒤 수행
- [ ] 2.4 `EnqueueHarvester(url, snapshotKey)` 호출 경로: 원본 URL + `snapshotKey`를 `harvester_frontier`에 UPSERT. 참고 SQL:
      ```sql
      INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, snapshot_key, score, next_harvest_at)
      VALUES ($1, $2, $3, $4, $5, $6, now())
      ON CONFLICT (url_hash) DO UPDATE
        SET snapshot_key = EXCLUDED.snapshot_key,
            next_harvest_at = now(),
            harvest_error_count = 0
        WHERE harvester_frontier.harvested_at IS NULL;
      ```
      이미 harvest된 URL(`harvested_at IS NOT NULL`)은 no-op이어야 함을 통합 테스트로 검증
- [ ] 2.5 성공 시 `scheduler.SetStatus(url, "fetched", nil)` 호출. `pinIDs`는 Pioneer에서 `nil`로 고정 (Pin 생성은 Harvester 책임)
- [ ] 2.6 실패 시 에러 분류 → `scheduler.SetStatus(url, "fetch_failed", nil)` + `scheduler.RecordFetchError(url, errorKind)` **둘 다** 호출. 호출 누락이 없도록 defer/helper로 묶을 것
- [ ] 2.7 `classifyError(err)` helper 구현: HTTP 4xx → `"http_4xx"`, HTTP 5xx → `"http_5xx"`, `net.Error` with `Timeout()==true` → `"timeout"`, 그 외 → `"network"`. snapshot 저장 실패도 `"network"`로 분류
- [ ] 2.8 `FilterChain.Apply(links)` 호출 위치: `extractLinks` 직후, `scheduler.Enqueue(QueuePioneer, ...)` 직전. 필터 내용은 수정하지 않고 `pioneer-link-filter-policy`가 구성한 체인을 그대로 호출
- [ ] 2.9 루프 sleep/backoff 코드 제거: 빈 큐 폴링은 scheduler 내부 책임이므로 Pioneer 루프에 `time.Sleep` 등을 **추가하지 않음**
- [ ] 2.10 feature flag 추가: `BOT_PIONEER_SCHEDULER` 환경변수 (기본 `false`). `true`일 때만 신규 consumer 경로 실행, `false`이면 기존 BFS 경로 유지 — **BFS fallback 병행 가능 기간** 확보

## 3. 인메모리 상태 제거 검증

- [ ] 3.1 Pioneer 신규 구현에 URL 큐/스택/슬라이스/채널 상태가 없음을 코드 리뷰 체크리스트에 반영
- [ ] 3.2 Pioneer 신규 구현에 visited 맵/사이트 카운터/세션 변수가 없음을 확인
- [ ] 3.3 Pioneer가 `pioneer_frontier` / `harvester_frontier` 테이블 컬럼을 직접 UPDATE/INSERT하지 않음을 `grep`/정적 검사로 확인 (모든 접근은 `URLScheduler` 메서드 경유)
- [ ] 3.4 Pioneer 코드에 분산 락/advisory lock/mutex 기반 조율 코드가 없음을 확인

## 4. 다중 워커 동작 검증

- [ ] 4.1 단일 프로세스 테스트: 신규 Pioneer가 seed URL 하나로 시작하여 `Dequeue → fetch → snapshot → Enqueue(pioneer) + EnqueueHarvester → SetStatus("fetched")` 사이클을 수 회 정상 반복하는지 확인
- [ ] 4.2 복수 인스턴스 테스트: 동일 scheduler에 2개 이상의 Pioneer를 띄웠을 때 동일 URL이 한 번만 처리되는지 검증 (`FOR UPDATE SKIP LOCKED` + host token bucket 재현)
- [ ] 4.3 재시작 복구 테스트: 크롤 도중 Pioneer를 kill했다가 다시 띄우면 frontier 현재 상태에서 즉시 이어받는지 확인 (lease timeout 10분 경과 후 재claim)
- [ ] 4.4 교차 사이트 Enqueue 확인: 외부 도메인 링크가 필터를 통과하면 Pioneer가 거르지 않고 `Enqueue(QueuePioneer, ...)`하는지 검증
- [ ] 4.5 실패 경로 테스트: HTTP 404 응답 시 `SetStatus("fetch_failed", nil)` + `RecordFetchError(url, "http_4xx")` 둘 다 호출되는지 검증 (scheduler가 `fetch_error_count = 5`로 즉시 dead 처리)
- [ ] 4.6 실패 경로 테스트: 네트워크 timeout 시 `errorKind = "timeout"`으로 기록되고 다음 `next_fetch_at`이 backoff 공식에 따라 계산되는지 검증
- [ ] 4.7 재크롤 가드 테스트: 이미 `harvested_at IS NOT NULL`인 URL을 재fetch하면 `EnqueueHarvester` 호출이 no-op이 되는지(snapshot_key/next_harvest_at 미변경) SQL 레벨에서 검증

## 5. 레거시 코드 제거 (신규 경로 안정화 후)

- [ ] 5.1 feature flag(`BOT_PIONEER_SCHEDULER`)를 기본 `true`로 전환하고 스테이징 + 일부 프로덕션 워커에서 병행 운영하여 안정화
- [ ] 5.2 `apps/api/internal/bot/priority_queue.go` 제거
- [ ] 5.3 `apps/api/internal/bot/bfs_queue.go` 제거
- [ ] 5.4 기존 `pioneer.go`의 BFS 본문/visited 맵/세션 카운터 제거 (생성자·공개 API는 신규 구현으로 일원화)
- [ ] 5.5 feature flag 제거(신규 경로가 유일 경로가 된 시점)
- [ ] 5.6 제거된 코드를 참조하던 테스트·진단 도구(`show-map` 등) 동작 재확인

## 6. 스펙/문서 정리

- [ ] 6.1 `bot` spec의 "BFS로 사이트를 탐색한다 (Pioneer)" requirement가 본 change의 `specs/bot/spec.md` REMOVED delta로 제거되는지 `openspec validate pioneer-scheduler-consumer --strict` 통과 확인
- [ ] 6.2 `docs/architecture.md`의 Pioneer-Scheduler 관계 다이어그램/서술 갱신: Pioneer가 scheduler consumer이자 fanout B의 producer(새 링크 → `pioneer_frontier`, 원본+snapshot_key → `harvester_frontier`)임을 명시
- [ ] 6.3 AGENTS.md에 Pioneer 동작 모델 한 줄 요약 업데이트 (Dequeue → fetch → snapshot → Enqueue(pioneer) + EnqueueHarvester → SetStatus)
- [ ] 6.4 아카이브: 배포 완료 후 change를 `openspec/changes/archive/2026-04-17-pioneer-scheduler-consumer/` 하위로 이동하여 보관
