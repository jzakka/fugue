## Context

Pioneer는 지금까지 단일 프로세스 BFS 크롤러였다. 큐/visited/카운터가 전부 인메모리 상태이고, 사이트 루트에서 BFS로 하강하여 링크를 추출하고 노드/엣지를 기록한 뒤, 세션 종료 시 stale edge를 정리했다. 운영 관점에서 두 가지 근본 문제가 있다.

- **수평 확장 불가**: Pioneer 프로세스가 여러 개면 인메모리 `visited` 맵을 공유할 수 없으므로 동일 URL을 중복 fetch한다. 사이트당 quota(`MaxNodesPerSite`)도 프로세스별로 따로 세어 총량이 의미를 잃는다.
- **재시작 시 상태 소실**: 크롤 도중 프로세스가 죽으면 큐 전체가 휘발하고, 재시작 시 사이트 루트부터 다시 탐색해야 한다.

선행 change들이 이 기반을 이미 만들어뒀다.
- `scheduler-frontier-table`: `pioneer_frontier` / `harvester_frontier` 두 독립 테이블, `UNIQUE(url_hash)` 제약, Pioneer claim용 partial index(`WHERE fetch_error_count < 5 ORDER BY score DESC, next_fetch_at ASC`) 정의.
- `scheduler-claim-api`: `URLScheduler` 인터페이스. `Dequeue(QueueType)` 단일 인자, 내부에서 `FOR UPDATE SKIP LOCKED` + host token bucket으로 linearizable claim, 빈 큐면 1초 sleep 후 재시도. `SetStatus(key, status, pinIDs)` / `RecordFetchError(key, errorKind)` / `EnqueueHarvester(url, snapshotKey)`.
- `pioneer-snapshot-storage`: `snapshots/<sha256_hex>/<yyyymmdd>.html.gz` 키 규약, 365일 TTL, gzip 압축.
- `pioneer-link-filter-policy`: `FilterChain`의 필터 구성(Domain → Extension → PathPattern → Robots → Dedup) 정의.

본 change는 이 기반 위에 **Pioneer의 동작 모델**을 확정한다. Pioneer는 `pioneer_frontier` consumer이면서 `pioneer_frontier`(새 링크) + `harvester_frontier`(원본 + snapshot_key) 양쪽의 producer — 즉 DECISIONS.md §2의 **fanout B**다.

## Goals / Non-Goals

**Goals:**
- Pioneer의 동작을 `Dequeue(QueuePioneer) → fetch → saveSnapshot → parseLinks → FilterChain.Apply → Enqueue(QueuePioneer, filtered) → EnqueueHarvester(url, snapshotKey) → SetStatus(url, "fetched", nil)` 루프로 정의.
- 실패 시 `SetStatus(url, "fetch_failed", nil)` + `RecordFetchError(url, errorKind)` 둘 다 호출하는 규약을 확정.
- `errorKind` 분류 규칙(HTTP 4xx/5xx, timeout, network)을 consumer가 결정함을 명시.
- Pioneer가 `FilterChain.Apply()`를 호출하는 **타이밍**(Enqueue 직전)을 명시. 필터 내용은 `pioneer-link-filter-policy`가 정의.
- Pioneer는 producer이자 consumer임을 명시. fanout B의 두 경로를 모두 규범화.
- Pioneer 코드에 인메모리 크롤 상태(큐/visited/세션 카운터)가 존재하지 않음을 규범화.
- 링크 추출과 콘텐츠 추출의 책임 경계를 명확히: Pioneer는 링크 + snapshot + harvester_frontier fanout만, 콘텐츠/Pin 생성은 Harvester.
- 전환 기간 feature flag + 롤백 경로를 명시.
- `bot` spec에서 인메모리 BFS를 전제하는 마지막 requirement 1건 제거.

**Non-Goals:**
- fetch 에러 backoff 공식 자체(`scheduler-retry-backoff`에서 정의. 본 change는 `RecordFetchError` 호출만 책임).
- host별 속도 제한(`scheduler-host-token-bucket`에서. Dequeue 내부에서 적용됨).
- Pioneer 워커 종료 조건과 전체 예산(`pioneer-worker-budget`에서).
- 링크 필터 **구성/정책** 자체(`pioneer-link-filter-policy`에서. 본 change는 호출 타이밍만 확정).
- snapshot 키 포맷/압축/TTL(`pioneer-snapshot-storage`에서).
- `URLScheduler` 인터페이스 시그니처와 Postgres claim 쿼리(`scheduler-claim-api`에서).
- `pioneer_frontier` / `harvester_frontier` 테이블 스키마(`scheduler-frontier-table`에서).
- harvester_frontier UPSERT SQL의 세부 컬럼 구성(`scheduler-claim-api` / `scheduler-frontier-table`에서. 본 design.md는 참고 예시로만 포함).

## Decisions

### Decision 1: Pioneer는 `pioneer_frontier`의 얇은 consumer로 정의한다
Pioneer는 자체 큐/BFS/visited 상태를 보유하지 않는다. 메인 루프 pseudo-code는 다음과 같이 최소화된다.

```go
for {
    url, err := scheduler.Dequeue(scheduler.QueuePioneer) // 내부 blocking (빈 큐 → 1초 sleep 재시도)
    if err != nil {
        log.Error("dequeue", err)
        continue
    }

    html, fetchErr := fetcher.Fetch(url)
    if fetchErr != nil {
        kind := classifyError(fetchErr) // "http_4xx" | "http_5xx" | "timeout" | "network"
        scheduler.SetStatus(url, "fetch_failed", nil)
        scheduler.RecordFetchError(url, kind)
        continue
    }

    snapshotKey, snapErr := snapshotStore.Save(url, html) // pioneer-snapshot-storage 규약
    if snapErr != nil {
        // snapshot 저장 실패는 network 분류로 기록
        scheduler.SetStatus(url, "fetch_failed", nil)
        scheduler.RecordFetchError(url, "network")
        continue
    }

    newLinks := extractor.ExtractLinks(html)              // <a href> 집합 + 메타데이터
    filteredLinks := filterChain.Apply(newLinks)          // pioneer-link-filter-policy

    scheduler.Enqueue(scheduler.QueuePioneer, filteredLinks) // 새 링크 → pioneer_frontier
    scheduler.EnqueueHarvester(url, snapshotKey)              // 원본 URL → harvester_frontier (UPSERT)

    scheduler.SetStatus(url, "fetched", nil)              // next_fetch_at = now() + 365d
}
```

**대안**: Pioneer에 "사이트 세션" 개념을 유지하고 scheduler는 URL 저장소로만 쓰는 설계. → 거부. 복수 워커에서 "세션" 경계가 정의되지 않으며, 이는 기존 인메모리 모델의 한계를 그대로 가져온다.

**근거**: scheduler가 이미 `FOR UPDATE SKIP LOCKED` 기반 linearizable claim을 제공하므로, Pioneer는 claim의 정확성을 재구현할 필요가 없다. "Pioneer가 가벼운 consumer"라는 모델은 수평 확장, 프로세스 재시작 복구, 테스트 용이성을 동시에 얻는다.

### Decision 2: Dequeue는 `QueueType` 단일 인자이며 폴링은 scheduler 내부 책임이다
Pioneer 루프는 `scheduler.Dequeue(scheduler.QueuePioneer)` 하나로 URL을 받는다. 과거 스케치에 있던 `queryCondition string`(`"not-visited"` 등) 인자는 **제거**한다. 이유: partial index 정의(`fetch_error_count < 5` 조건 등)가 이미 frontier 테이블에 포함되어 있어 consumer가 조건을 넘길 필요가 없다.

빈 큐일 때의 sleep(1초 고정 간격), `FOR UPDATE SKIP LOCKED` 후 host token bucket 실패 시 재시도 로직은 모두 **scheduler 내부**에서 처리한다. Pioneer 루프는 `Dequeue` 호출 후 어떤 sleep/backoff도 수행하지 않는다.

**대안**: Pioneer가 "다음 URL이 없으면 짧게 sleep 후 재시도" 로직을 갖는 설계. → 거부. 동일 정책이 Harvester에도 필요하며 scheduler 구현체에 두는 것이 DRY하다.

### Decision 3: 성공 시 SetStatus("fetched", nil)로만 보고한다
Pioneer는 frontier row를 직접 UPDATE하지 않는다. 성공 시 `scheduler.SetStatus(url, "fetched", nil)` 한 번만 호출하면 scheduler 구현체가 `next_fetch_at = now() + 365 days`로 갱신하고 `fetch_error_count`를 0으로 리셋한다(DECISIONS.md §3, §8). `pinIDs` 파라미터는 Harvester 전용이므로 Pioneer는 `nil`을 전달한다.

### Decision 4: 실패 시 `SetStatus` + `RecordFetchError` 둘 다 호출한다
DECISIONS.md §3 "Consumer 호출 규약"에 따라, fetch 실패 시 Pioneer는 다음 두 호출을 순서대로 수행한다.

1. `scheduler.SetStatus(url, "fetch_failed", nil)` — status 마킹
2. `scheduler.RecordFetchError(url, errorKind)` — `fetch_error_count` 증가 + `next_fetch_at` backoff 계산(공식은 `scheduler-retry-backoff`)

**errorKind 분류 규칙 (Pioneer 책임)**:
| 조건 | errorKind |
|------|-----------|
| HTTP 응답 status 400-499 | `"http_4xx"` |
| HTTP 응답 status 500-599 | `"http_5xx"` |
| `net.Error` 이며 `Timeout() == true` | `"timeout"` |
| 그 외 네트워크/IO 에러 | `"network"` |
| snapshot 저장 실패 | `"network"` (서버 원인 분류 없음 → 일반 IO로 취급) |

`"http_4xx"`는 scheduler 쪽에서 즉시 `fetch_error_count = 5`(dead)로 설정한다(DECISIONS.md §3).

### Decision 5: Pioneer는 fanout B의 producer이다
Pioneer는 동일한 `URLScheduler` 인스턴스에 대해 두 큐 모두에 쓴다.

- **새 링크**: `scheduler.Enqueue(scheduler.QueuePioneer, filteredLinks)` — `pioneer_frontier`에 INSERT ON CONFLICT (url_hash) DO NOTHING.
- **원본 URL + snapshot_key**: `scheduler.EnqueueHarvester(url, snapshotKey)` — `harvester_frontier`에 UPSERT. `ON CONFLICT (url_hash) DO UPDATE`에서 `WHERE harvester_frontier.harvested_at IS NULL` 가드를 걸어, 이미 harvest가 끝난 URL은 no-op으로 처리한다(DECISIONS.md §8).

참고 UPSERT SQL(실제 쿼리는 `scheduler-claim-api` / sqlc에서 관리):
```sql
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, snapshot_key, score, next_harvest_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (url_hash) DO UPDATE
  SET snapshot_key = EXCLUDED.snapshot_key,
      next_harvest_at = now(),
      harvest_error_count = 0
  WHERE harvester_frontier.harvested_at IS NULL;
```

**대안**: Pioneer가 harvester_frontier에는 쓰지 않고 별도 indexer가 snapshot 이벤트를 소비. → 거부. 컴포넌트와 실패 지점이 늘어나는 반면 이익이 없다. frontier가 이미 dedup / re-harvest 가드를 담당한다.

### Decision 6: FilterChain.Apply 호출은 Pioneer consumer의 책임이다 (DECISIONS.md §13)
Pioneer consumer가 링크 추출 후 `filterChain.Apply(newLinks)`를 호출한다. 이 호출은 `scheduler.Enqueue(QueuePioneer, ...)` **직전**에 위치하여, frontier에는 필터를 통과한 링크만 들어간다.

- 필터 **구성**(DomainFilter/ExtensionFilter/PathPatternFilter/RobotsFilter/DedupFilter 순서 등)은 `pioneer-link-filter-policy`가 정의.
- 필터 **호출 타이밍**(Enqueue 직전, Dequeue 이후)은 본 change(`pioneer-scheduler-consumer`)가 정의.

이 분리는 Pioneer 코드 변경 없이 필터 정책만 갱신하는 배포를 가능하게 한다.

### Decision 7: Pioneer는 링크 + snapshot 저장 + harvester fanout만 담당한다
Pioneer의 책임 경계:
- **포함**: HTTP fetch, snapshot 저장(`pioneer-snapshot-storage` 규약), `<a href>` 추출, FilterChain.Apply 호출, `pioneer_frontier` / `harvester_frontier` 쓰기, scheduler 상태 보고.
- **제외**: JavaScript 스크립트 실행, 미디어 다운로드(`harvester-image-cache`), Pin 생성(`harvester-pin-document`), OG/semantic 파싱, 콘텐츠 분류.

### Decision 8: 교차 사이트 크롤을 허용한다
Pioneer는 사이트/도메인 경계를 알지 않는다. 도메인 제한이 필요하면 `pioneer-link-filter-policy`의 link filter에서 정책으로 표현한다. 본 change의 스펙은 "Pioneer가 도메인 경계를 인지해야 한다"고 명시하지 않는다.

### Decision 9: Pioneer는 인메모리 크롤 상태를 가져서는 안 된다
"인메모리 크롤 상태"란 URL 큐, visited/방문 집합, 사이트/세션 카운터를 말한다. 프로세스 로컬 캐시(예: HTTP keep-alive pool, DNS 캐시, FilterChain 내부의 robots.txt 캐시)는 포함하지 않는다. 이 구분은 "재시작 후 frontier 상태만으로 동작이 복구되는가"로 테스트 가능하다.

### Decision 10: 다중 워커 정확성은 scheduler가 보장한다
"정확히-한 번 claim", "동일 URL 동시 처리 방지" 같은 정합성은 scheduler의 `FOR UPDATE SKIP LOCKED` + `next_fetch_at` lease(10분 in-flight marker) claim에서 나온다. Pioneer 코드에는 mutex/semaphore/advisory lock 같은 동시성 제어를 두지 않는다.

### Decision 11: 전환은 feature flag로, 롤백 경로는 BFS fallback 병행
전환 기간 동안 `BOT_PIONEER_SCHEDULER` 환경변수로 신규 scheduler 경로와 구 BFS 경로를 분기한다.

- `BOT_PIONEER_SCHEDULER=true`: 본 change의 consumer 루프 사용.
- `BOT_PIONEER_SCHEDULER=false`: 기존 BFS/PriorityQueue 경로 사용.

두 경로는 스테이징에서 병행 가능해야 하며, 롤백은 플래그 토글만으로 즉시 이루어져야 한다. `pioneer_frontier` / `harvester_frontier` 테이블은 롤백 시에도 삭제하지 않는다(재전환 시 재사용). 신규 경로 안정화(수 주 운영) 확인 후 기존 코드와 플래그를 함께 제거한다.

## Risks / Trade-offs

- **Risk: `SetStatus` 또는 `RecordFetchError` 호출 누락 시 URL이 영원히 dead되거나 반복 claim된다** → scheduler의 lease timeout(10분, DECISIONS.md §3)이 claim을 자동 회수한다. 본 change는 "실패 시 둘 다 호출" 규약을 requirement로 못박아 코드 리뷰 단계에서 차단한다.
- **Risk: `EnqueueHarvester` 누락 시 Pin이 생성되지 않는다** → pipeline 누락은 재크롤(365일 후) 때 자연 복구되지만 실질적으로는 관측 누락이다. 구조적 방지: consumer 루프의 성공 경로는 `Enqueue(pioneer)` + `EnqueueHarvester` + `SetStatus("fetched", nil)` 세 호출을 원자적 블록처럼 묶고, 중간 실패 시 `SetStatus("fetch_failed", ...)` + `RecordFetchError`로 처리한다.
- **Risk: "인메모리 상태 금지"는 강한 제약이라 리팩터 비용이 크다** → 기존 `apps/api/internal/bot/pioneer.go`와 `priority_queue.go`/`bfs_queue.go`가 삭제·재작성된다. tasks.md에서 feature flag 단계적 전환으로 완화한다.
- **Trade-off: 교차 사이트 크롤 허용으로 quota/속도 제어를 scheduler가 전부 떠안는다** → `scheduler-host-token-bucket`(host별 속도)과 `pioneer-worker-budget`(전체 예산)이 각각 맡는다. Pioneer는 정책을 신경 쓰지 않아 심플해진다.
- **Trade-off: UPSERT `WHERE harvested_at IS NULL` 가드로 재크롤 시 snapshot_key가 갱신되지 않는다** → 이미 harvest된 URL은 pins가 이미 만들어져 있으므로 재harvest하지 않는 것이 §8의 정책이다. snapshot 파일 자체는 오브젝트 스토리지에 덮어써 최신 상태를 유지한다(last-write-wins, DECISIONS.md §7).

## Migration Plan

1. 선행 change 확인: `scheduler-frontier-table`(두 테이블), `scheduler-claim-api`(`Dequeue(QueueType)` / `EnqueueHarvester` / `RecordFetchError` 포함), `pioneer-snapshot-storage`, `pioneer-link-filter-policy`가 반영되어야 한다.
2. 새 Pioneer consumer 구현을 `apps/api/internal/bot/` 하위에 추가. `BOT_PIONEER_SCHEDULER=false`가 기본값이며 스테이징에서 `true`로 전환해 병행 운영.
3. 신규 경로 안정화(스테이징 + 일부 프로덕션 워커)가 확인되면 플래그 기본값을 `true`로 올리고, 이후 `priority_queue.go` / `bfs_queue.go` / `pioneer.go`의 BFS 본문을 제거.
4. 롤백 전략: 플래그만 토글. `pioneer_frontier` / `harvester_frontier` 테이블은 삭제하지 않는다.

## Open Questions

없음. 본 change 범위의 모든 결정은 DECISIONS.md §2, §3, §8, §13 및 상기 Decisions 섹션에서 확정되었다.

- ~~Pioneer가 링크 추출 시 DOM selector 메타데이터를 frontier에 함께 적재할지~~ → `pioneer-link-filter-policy`가 필터 내부에서 소비. Pioneer는 `score` 계산을 하지 않고 scheduler 기본 score(0)로 Enqueue.
- ~~`SetStatus`의 status 문자열 enum~~ → `scheduler-claim-api`에서 `"fetched"` / `"fetch_failed"` / `"harvested"` / `"harvest_failed"`로 확정(DECISIONS.md §3).
- ~~filterLinks 호출 책임 주체~~ → DECISIONS.md §13: Pioneer consumer가 `FilterChain.Apply()` 호출.
