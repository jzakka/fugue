## Context

`scheduler-frontier-table` change가 `harvester_frontier` 테이블과 Harvester용 partial index(`WHERE harvested_at IS NULL AND harvest_error_count < 5 ORDER BY score DESC, next_harvest_at ASC`)를 정의하고, `scheduler-claim-api` change가 `URLScheduler` 인터페이스(`Enqueue`, `Dequeue(QueueType)`, `SetStatus(key, status, pinIDs)`, `RecordHarvestError(key, errorKind)`)와 `FOR UPDATE SKIP LOCKED` 기반 linearizable claim을 정의한다. 본 change는 그 위에서 동작하는 **Harvester consumer**의 메인 루프와 상태 전이를 확정한다.

현재 `apps/api/internal/bot/harvester.go`의 `harvestBFS`는 다음 인메모리 자료구조에 의존한다:
- `BFSQueue` (level별 큐)
- `visited map[uuid.UUID]bool`
- `nodeMap map[uuid.UUID]db.BotGraphNode` (사이트 전체 노드 사전 적재)
- `findRootNode` 기반 단일 사이트 진입점

이 구조는 (1) 단일 프로세스만 동작 가능하고, (2) 사이트 단위로만 실행되며, (3) 재시작 시 진행 상태가 휘발된다. 요구는 명확히 "Harvester도 우선순위 큐 기반 (BFS 아님)"이고, 그 우선순위 큐는 `harvester_frontier` partial index가 구현하는 영속 큐다.

본 design은 이 의사코드의 정합성을 frontier 모델 위에서 확정한다.

## Goals / Non-Goals

**Goals:**
- Harvester를 `URLScheduler` consumer로 재정의: `Dequeue(QueueHarvester) → fetchSnapshotOrLive → harvestPipeline.Process → createPins → SetStatus(url, "harvested", pinIDs)`.
- Harvester의 진행 상태는 100% `harvester_frontier` row에 보관 (인메모리 큐/visited/nodeMap 금지).
- 다중 워커 정확성(중복 dequeue 금지, at-most-once Pin 생성)은 `URLScheduler` 책임에 위임. Harvester는 임의 워커 수에서 안전하게 돌 수 있음을 명시한다.
- 사이트 경계와 무관하게 동작: 한 워커가 여러 host의 row를 섞어 처리할 수 있다. "사이트 단위 BFS" 개념을 Harvester에서 제거한다.
- 다중 Pin 처리: 한 URL이 여러 Pin을 생성하면 `[]uuid.UUID` pinIDs 전체를 `SetStatus`가 한 트랜잭션에서 `harvester_frontier_pins`에 일괄 INSERT.
- 실패 시 `SetStatus("harvest_failed", nil)` + `RecordHarvestError(url, errorKind)` 이중 호출 규약 준수.

**Non-Goals:**
- `URLScheduler` 인터페이스 구현/SQL (`scheduler-claim-api` 책임).
- `harvester_frontier`/`harvester_frontier_pins` 스키마 (`scheduler-frontier-table` 책임).
- 백오프 수식 (`scheduler-retry-backoff`).
- Pin 문서/메타데이터 스키마 (`harvester-pin-document`).
- 워커 종료 조건/예산 (`harvester-worker-budget`).
- Snapshot storage 키 포맷 (`pioneer-snapshot-storage`).
- Snapshot miss 시 HTTP fallback 세부 (`harvester-snapshot-first-fetch`).
- Pioneer 측 동작.

## Decisions

### Decision 1: URL 수급은 오직 `scheduler.Dequeue(scheduler.QueueHarvester)`

**선택**: consumer 루프의 한 iteration은 항상 `scheduler.Dequeue(scheduler.QueueHarvester)` 호출로 시작한다. 인자 `QueueHarvester`는 `scheduler-claim-api`가 정의하는 `QueueType` enum의 한 값이며, scheduler는 이 값에 따라 `harvester_frontier`만을 대상으로 partial index scan + `FOR UPDATE SKIP LOCKED`를 수행한다.

**근거**:
- 요구 사항 "Harvester도 우선순위 큐 기반"의 핵심은 "다음 URL 결정이 한 자리(DB partial index)에서만 이루어진다"는 것이다.
- Pioneer/Harvester 공용 `Dequeue` 시그니처 하나로 두 consumer의 진입점이 동형이 된다(본 저장소 내 `pioneer-scheduler-consumer`와 대칭).

**Trade-off**: 매 iteration DB 왕복 1회. partial index scan은 소폭 비용이나 워커 수가 한정적이므로 허용 가능.

### Decision 2: 빈 큐 polling은 scheduler 내부 책임

**선택**: Harvester consumer 루프는 `time.Sleep`/backoff를 호출하지 않는다. `Dequeue`는 내부에서 "빈 큐 → 1초 sleep → 재시도"를 blocking 형태로 수행하고(DECISIONS §3), 반드시 URL을 반환한 뒤에야 return한다. consumer는 "반환된 URL을 받아 처리"만 한다.

**근거**:
- 폴링 정책을 두 곳(scheduler + consumer)에서 관리하면 sleep 간격이 어긋나 낭비가 생긴다.
- scheduler 내부에서 polling하면 Pioneer consumer와 Harvester consumer의 sleep 정책이 자동으로 통일된다.

### Decision 3: Snapshot-first fetch 경유

**선택**: 루프의 fetch 단계는 `harvester-snapshot-first-fetch` capability가 정의한 `Fetcher.Fetch(url) ([]byte, error)` 인터페이스를 호출한다(해당 change의 design.md §Decision 1). 주입되는 구현체는 `CompositeFetcher`로 내부에서 다음을 수행한다:
- `harvester_frontier.snapshot_key`가 non-NULL이면 object storage에서 snapshot을 읽는다.
- snapshot이 miss(key 없음/만료/네트워크/권한/내부 에러)이면 HTTP live fetch로 폴백한다(단일 "miss"로 취급).

`Fetcher.Fetch`가 반환한 에러는 consumer가 `classifyFetchError(err)` 헬퍼로 scheduler enum(`"http_4xx" | "http_5xx" | "network" | "timeout"`)으로 매핑한다(Pioneer consumer의 `classifyError` 패턴과 동형). 본 spec은 "snapshot-first 경로를 경유한다"는 호출 규약과 consumer 측 에러 분류 책임만 정의하고, storage 클라이언트/HTTP 리트라이는 `harvester-snapshot-first-fetch`에 위임한다.

**근거**: Harvester가 snapshot 키 포맷을 직접 알아야 할 이유는 없다. 책임 분리가 명확.

### Decision 4: 파싱 → `PinDocument` → Pin 생성 흐름

**선택**: fetch된 HTML을 `harvestPipeline.Process(html)`에 넘기면 `PinDocument`(핀 0~N개로 materialize 가능한 구조)를 반환한다. consumer는:
- `pinDocument.Pinnable == true` → `createPins(pinDocument) → pinIDs []uuid.UUID`
- `pinDocument.Pinnable == false` → skip (pinIDs = nil)

다음 구문은 `harvester-pin-document` capability에서 정의된 스키마를 그대로 사용하며, 본 spec은 흐름만 정의한다.

**근거**: Pin 스키마/분류 규칙은 별도 책임. Harvester consumer는 "Pinnable 체크 → Pin 생성 → pinIDs 취합" 세 단계만 수행.

### Decision 5: 성공 상태 전이는 `SetStatus(url, "harvested", pinIDs)` 단일 호출

**선택**: `scheduler.SetStatus`가 다음을 한 트랜잭션으로 처리한다(`scheduler/spec.md` 의 `SetStatus` requirement 참조):
1. `UPDATE harvester_frontier SET harvested_at = now(), last_updated_at = now() WHERE url_hash = sha256(url)`
2. `UPDATE harvester_frontier SET harvest_error_count = 0` — 이전 harvest 시도에서 누적된 실패 카운터를 리셋한다(scheduler SSOT: `scheduler/spec.md` `"harvested"` status 요구사항).
3. `INSERT INTO harvester_frontier_pins (frontier_id, pin_id) VALUES (..., $1), (..., $2), ...` (pinIDs 일괄 INSERT)

pinIDs가 `nil` 또는 빈 슬라이스면 3단계는 no-op. row는 여전히 harvested로 표시되어 partial index(`WHERE harvested_at IS NULL`)에서 자동 제외된다.

**근거**: `harvested_at`과 `harvester_frontier_pins` INSERT가 분리되면 "매핑 없는 harvested row" 또는 "harvested_at이 NULL인데 pin 매핑이 있는 row"가 생길 수 있다. 한 트랜잭션으로 묶어 이 불일치를 원천 차단한다.

### Decision 6: 실패 시 SetStatus + RecordHarvestError 이중 호출

**선택**: fetch/파싱/Pin 생성 어느 단계든 실패하면:
```
scheduler.SetStatus(url, "harvest_failed", nil)
scheduler.RecordHarvestError(url, errorKind)
```
두 호출을 순서대로 수행한다. `errorKind`는 `scheduler-claim-api`가 허용하는 enum 4종으로 제한된다(`scheduler/spec.md` `RecordHarvestError` requirement): `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`. 그 외 값은 scheduler가 에러를 반환하며 row를 변경하지 않으므로 consumer가 임의로 확장해서는 안 된다.

**consumer 측 실패 분류 → scheduler enum 매핑:**
| 상황 | 내부 분류(로그/메트릭용) | scheduler errorKind |
|------|-------------------------|---------------------|
| HTTP 4xx 응답 (snapshot miss 후 live fetch) | `fetch_http_4xx` | `"http_4xx"` |
| HTTP 5xx 응답 | `fetch_http_5xx` | `"http_5xx"` |
| DNS/connect/TLS 오류 | `fetch_network` | `"network"` |
| fetch/스크립트 실행 타임아웃 | `fetch_timeout` | `"timeout"` |
| `harvestPipeline.Process` 자체 실패 (스크립트 런타임/구문 에러 등) | `parse` | `"network"` — 내부적 일시 실패로 간주, 백오프 후 재시도 |
| DB 에러 등으로 Pin INSERT 실패 | `pin_create` | `"network"` — 내부적 일시 실패로 간주, 백오프 후 재시도 |

`parse`/`pin_create`는 consumer 내부 로그·메트릭 라벨로만 사용하고, scheduler 보고 시에는 4종 enum 중 하나로 매핑한다. 내부 분류를 `network`로 집약하는 근거: 두 실패 모두 "특정 입력이나 일시적 리소스 상태로 인해 실패했을 수 있다"는 점에서 retriable로 취급하는 것이 안전하다. 실제로 같은 입력에 대해 계속 실패하면 `harvest_error_count`가 5에 도달해 partial index에서 자동으로 제외된다(`scheduler-claim-api`의 dead 경계). `http_4xx`는 `RecordHarvestError` 쪽에서 즉시 `harvest_error_count = 5`로 설정(Pioneer 규칙을 Harvester에도 동일 적용).

**근거**: DECISIONS §3 "Consumer 호출 규약: 실패 시 SetStatus + RecordXxxError 둘 다 호출". 상태 갱신과 카운터 누적은 책임이 다르므로 별도 API로 분리되어 있다.

### Decision 7: 재harvest 없음 (UPSERT 가드에 의존)

**선택**: Harvester는 "이 URL은 이미 harvest했나?"를 코드에서 체크하지 않는다. Pioneer가 재크롤하여 UPSERT할 때(DECISIONS §8) `WHERE harvester_frontier.harvested_at IS NULL` 가드가 적용되므로 harvested URL은 `next_harvest_at` 리셋 없이 no-op. 따라서 partial index에 다시 나타나지 않아 `Dequeue`가 반환하지 않는다.

**근거**: idempotency를 스키마 수준 guard로 구현하면 Harvester 코드가 단순해진다(조회 없음).

### Decision 8: BFS/nodeMap/그래프 순회 제거

**선택**: `findRootNode`, edge 순회, child 노드 sorting, 사이트별 `nodeMap` 사전 적재 전부 제거한다. Harvester는 자기가 받은 한 URL이 그래프상 어디에 있는지 알 필요가 없다.

**근거**:
- 그래프 순회는 Pioneer가 enqueue 단계에서 `score`에 이미 반영.
- "사이트 단위" 개념이 사라지면 한 워커가 여러 host의 고우선순위 URL을 섞어 처리할 수 있다 → 워커 활용도 ↑.
- `nodeMap`을 메모리에 적재하던 비용(사이트가 커지면 OOM 위험)이 사라진다.

### Decision 9: 다중 워커 정확성은 scheduler 계약에 위임

**선택**: Harvester는 자체 advisory lock/분산 락/워커 간 조정 채널을 갖지 않는다. 동일 row 중복 claim 방지는 `scheduler-claim-api`의 `FOR UPDATE SKIP LOCKED`에 전적으로 위임한다.

## Consumer 루프 pseudo-code

```go
for {
    url, err := scheduler.Dequeue(scheduler.QueueHarvester)
    // Dequeue는 내부적으로 blocking polling. 빈 큐면 1초 sleep 후 재시도하고,
    // URL이 claim되기 전에는 return하지 않는다.
    if err != nil {
        if ctx.Err() != nil { return nil } // ctx 취소 시 안전 종료
        continue
    }

    html, fetchErr := fetcher.Fetch(url)
    // harvester-snapshot-first-fetch 책임: Fetcher.Fetch(url) ([]byte, error).
    // CompositeFetcher가 ObjectStorage 우선 → HTTP fallback을 내부에서 처리.
    if fetchErr != nil {
        errorKind := classifyFetchError(fetchErr) // "http_4xx"|"http_5xx"|"network"|"timeout"
        scheduler.SetStatus(url, "harvest_failed", nil)
        scheduler.RecordHarvestError(url, errorKind)
        continue
    }

    pinDocument, parseErr := harvestPipeline.Process(ctx, html, url)
    // harvester-pin-document 책임. Process 자체 실패는 내부 분류 `parse`로 집계하고
    // scheduler에는 `"network"`로 보고(일시 실패 간주, backoff 재시도).
    if parseErr != nil {
        scheduler.SetStatus(url, "harvest_failed", nil)
        scheduler.RecordHarvestError(url, "network")
        continue
    }

    if !pinDocument.Pinnable {
        // 리스팅/본문 없음/미디어 없음 등. Pin 생성 안 함. 완료 표기.
        scheduler.SetStatus(url, "harvested", nil)
        continue
    }

    pinIDs, createErr := createPins(ctx, pinDocument)
    // 한 문서에서 N개 Pin 가능. pinIDs: []uuid.UUID.
    if createErr != nil {
        // 내부 분류 `pin_create` → scheduler `"network"`로 보고.
        scheduler.SetStatus(url, "harvest_failed", nil)
        scheduler.RecordHarvestError(url, "network")
        continue
    }

    // 성공 경로: harvested_at UPDATE + harvest_error_count=0 리셋 + harvester_frontier_pins 일괄 INSERT가 한 트랜잭션.
    scheduler.SetStatus(url, "harvested", pinIDs)
}
```

본 pseudo-code는 의도를 드러내기 위한 것이며, 실제 구현에서는 에러 분류 헬퍼와 컨텍스트 전달을 구조적으로 정리할 수 있다. 내부 SQL은 design.md에서만 다루고, spec/tasks.md는 동작/호출 순서만 규정한다.

## Risks / Trade-offs

- **[Risk] Pioneer가 enqueue를 충분히 채우지 못하면 Harvester 워커가 idle**: Harvester는 자체 enqueue를 하지 않는다. → Mitigation: `Dequeue` 내부 polling이 1초 간격으로 재시도하므로 Pioneer가 채우는 대로 즉시 소비.
- **[Risk] `SetStatus` 직후 워커 크래시 → 워커 단위 메트릭 누락**: 가벼운 누락이며 row 자체는 일관됨(트랜잭션 단일). → Mitigation: 정확한 회계가 필요하면 별도 audit log.
- **[Risk] `errorKind` 분류 오류로 백오프가 의도와 다르게 걸림**: 예를 들어 DNS fail을 `timeout`으로 잘못 분류하면 backoff 곡선이 달라진다. → Mitigation: 분류 로직을 헬퍼 함수로 집중화하고 단위 테스트로 커버(tasks.md §7).
- **[Trade-off] 매 iteration DB 왕복 1회**: Decision 1/2 근거 참조. 워커 수 × QPS 견적이 partial index scan 한도 안임을 운영 단계에서 모니터링.
- **[Trade-off] 사이트별 진행률을 코드 내부 변수로 알 수 없음**: `SELECT count(*) FROM harvester_frontier WHERE host = ? AND harvested_at IS NOT NULL` 같은 frontier 집계 쿼리로 대체. 본 spec 범위 외.

## Migration Plan

1. (선결) `scheduler-frontier-table` 적용 → `harvester_frontier`, `harvester_frontier_pins`, partial index 존재.
2. (선결) `scheduler-claim-api` 적용 → `Dequeue(QueueType)`, `SetStatus(key, status, pinIDs)`, `RecordHarvestError(key, errorKind)` 사용 가능.
3. (선결) `harvester-snapshot-first-fetch` 적용 → `Fetcher.Fetch(url) ([]byte, error)` 인터페이스와 `CompositeFetcher` 구현체 주입 가능.
4. (선결) `harvester-pin-document` 적용 → `harvestPipeline.Process → PinDocument` 스키마 확정.
5. 신규 Harvester `Run(ctx)` 구현: 본 design의 pseudo-code대로 consumer 루프 작성.
6. 기존 `harvestBFS`, `BFSQueue` 의존, `nodeMap`, `findRootNode`, `findRootNode` 기반 초기화 경로 삭제.
7. CLI 진입점은 유지하되 인자 의미 변경: "사이트 ID 1건 BFS"가 아니라 "consumer 워커 1개 시작" (worker exit 정책은 `harvester-worker-budget`).
8. **Rollback**: 본 change 자체는 frontier가 채워져 있어야 의미가 있다. 롤백이 필요하면 신규 Harvester 진입점을 비활성화. 단, DB 스키마(prerequisite change)는 롤백 범위 외.
