## Context

`scheduler-frontier-table` change가 `bot_frontier` 테이블과 Harvester용 partial index(`pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now()`)를 정의했고, `scheduler-claim-api` change가 `URLScheduler` 인터페이스(`Enqueue`, `Dequeue`, `SetStatus`)와 `FOR UPDATE SKIP LOCKED` 기반 linearizable claim을 정의한다(예정). 본 change는 그 위에서 동작하는 **Harvester consumer**의 메인 루프와 상태 전이를 확정한다.

현재 `apps/api/internal/bot/harvester.go`의 `harvestBFS`는 다음 인메모리 자료구조에 의존한다:
- `BFSQueue` (level별 큐)
- `visited map[uuid.UUID]bool`
- `nodeMap map[uuid.UUID]db.BotGraphNode` (사이트 전체 노드 사전 적재)
- `findRootNode` 기반 단일 사이트 진입점

이 구조는 (1) 단일 프로세스만 동작 가능하고, (2) 사이트 단위로만 실행되며, (3) 재시작 시 진행 상태가 휘발된다. 사용자 요구는 명확히 "Harvester도 우선순위 큐 기반 (BFS 아님)"이고, 그 우선순위 큐는 `bot_frontier` partial index가 구현하는 영속 큐다.

`apps/api/fuguebot_pseudo.go`의 `Harvester.Run`은 이미 의도된 형태를 보여준다:

```
current := pq.Dequeue("not-parsed")
for {
    content := fetch(current)
    document := ParseDocument(content)
    Index(document)
    current = pq.Dequeue("not-parsed")
}
```

본 design은 이 의사코드의 정합성을 frontier 모델 위에서 확정한다.

## Goals / Non-Goals

**Goals:**
- Harvester를 `URLScheduler` consumer로 재정의: `Dequeue → fetch → ParseDocument → Index → SetStatus(pin_id 또는 error)`.
- Harvester의 진행 상태는 100% `bot_frontier` row에 보관 (인메모리 큐/visited/nodeMap 금지).
- 다중 워커 정확성(중복 dequeue 금지, at-most-once Pin 생성)은 `URLScheduler` 책임에 위임. Harvester는 임의 워커 수에서 안전하게 돌 수 있음을 명시한다.
- 사이트 경계와 무관하게 동작 (한 워커가 여러 사이트 노드를 섞어 처리해도 됨). "사이트 단위 BFS"라는 개념을 Harvester에서 제거한다.
- 성공/실패 시 frontier row 갱신 의미를 명확히 정의 (성공 = `pin_id` 채움, 실패 = `harvest_error_count++` + `next_harvest_at` 백오프).

**Non-Goals:**
- `URLScheduler` 인터페이스 정의 (`scheduler-claim-api` 책임).
- `bot_frontier` 테이블/인덱스 정의 (`scheduler-frontier-table` 책임).
- 백오프 정책의 구체 수식 (`scheduler-retry-backoff` 같은 후속 change 책임).
- Pin 문서/메타데이터 스키마 (`harvester-pin-document` 책임).
- 워커 종료 조건/예산 (`harvester-worker-budget` 책임).
- 이미지/HTML 캐시, snapshot first-fetch.
- Pioneer 측 동작.

## Decisions

### Decision 1: Harvester의 "다음 노드"는 항상 `URLScheduler.Dequeue`가 결정한다

**선택**: 메인 루프에서 어떤 인메모리 자료구조도 "다음에 처리할 노드 후보"를 보관하지 않는다. 한 iteration의 시작은 항상 `scheduler.Dequeue(ctx)` 호출이고, 이 호출이 block/return하는 단일 row만 처리한다.

**대안**: (a) Harvester가 사이트별로 작은 prefetch 버퍼를 유지하여 latency를 낮춘다. (b) 기존 `BFSQueue`를 유지하고 frontier는 보조로만 쓴다.

**근거**:
- (a) prefetch 버퍼는 워커가 죽었을 때 해당 row가 다른 워커에게 재할당되지 못하는 시간을 만든다. `FOR UPDATE SKIP LOCKED` 트랜잭션 경계와도 어긋난다. latency가 실제로 문제가 되면 `Dequeue` 내부에서 batch claim으로 해결할 수 있다 (scheduler 책임).
- (b) "Harvester는 BFS 아니라 우선순위 큐 기반"이라는 사용자 요구의 핵심은 "한 자리에서 우선순위가 결정된다"는 것이다. 이중 큐는 두 곳에서 우선순위가 결정되므로 의미를 깬다.

**Trade-off**: 매 iteration마다 DB 왕복이 1회 추가된다. 단, claim 쿼리는 partial index 단독 스캔이고 워커 수가 한정적이므로 허용 가능.

### Decision 2: BFS 그래프 순회 자체를 Harvester에서 제거한다

**선택**: `findRootNode`, edge 순회, child 노드 sorting 모두 제거한다. Harvester는 자기가 받은 한 row가 그래프상 어디에 있는지 알 필요가 없다.

**근거**:
- 그래프 순회 경로는 Pioneer가 enqueue 단계에서 frontier에 미리 표현했다 (각 row의 `score`로). Harvester가 다시 그래프를 따라가는 것은 중복 책임.
- "사이트 단위" 개념이 사라지면 한 워커가 여러 사이트의 고우선순위 노드를 섞어 처리할 수 있다 → 워커 활용도 ↑.
- `nodeMap`을 메모리에 적재하던 비용(사이트가 커지면 OOM 위험)이 사라진다.

**Trade-off**: 사이트 단위 진행률 같은 메트릭은 frontier 쿼리(`SELECT count(*) FROM bot_frontier WHERE host = ? AND pin_id IS NOT NULL`)로 대체해야 한다. 본 spec 범위 외.

### Decision 3: 성공 시 `pin_id` 갱신을 트랜잭션으로 묶지 않는다 (현 단계)

**선택**: Pin INSERT와 `bot_frontier.pin_id` UPDATE를 같은 트랜잭션에 강제하지 않는다. Pin 생성 후 별도 `scheduler.SetStatus(key, pin_id)` 호출로 갱신한다.

**대안**: 한 트랜잭션에 묶어서 "Pin은 있는데 frontier row가 미갱신" 상태를 원천 차단.

**근거**:
- Pin INSERT 경로는 기존 `pipeline.Process`가 담당하며 자체 트랜잭션 경계가 있다. Harvester가 이를 frontier 트랜잭션과 묶으면 두 도메인(콘텐츠 / 크롤 frontier)을 강결합하게 된다.
- "Pin은 있는데 frontier 미갱신" 상태는 다음 워커가 같은 row를 다시 claim해서 처리할 때 콘텐츠 중복 체크(이미 본 spec과 무관하게 bot spec에 정의됨: "봇이 이미 수집한 sourceURL은 중복 스킵")가 막는다. 즉 멱등성으로 흡수 가능.
- 백압이 누적되면 `harvester-pin-document` change에서 트랜잭션 통합을 재검토할 수 있다.

**Trade-off**: 동일 sourceURL Pin이 짧게 한 번 더 시도될 수 있음 (실제 INSERT는 중복 스킵).

### Decision 4: 실패 분류는 단순화 — 모두 `harvest_error_count++` + `next_harvest_at` 백오프

**선택**: fetch 실패, 스크립트 실행 실패, pipeline 실패를 구분하지 않고 한 카운터(`harvest_error_count`)에 누적한다. `harvest_error_count >= 5` 도달 row는 partial index에서 자동 제외된다.

**대안**: 실패 종류별로 별도 카운터/별도 백오프 곡선.

**근거**:
- 본 spec은 "consumer 루프"의 골격을 정의하는 것이 목적. 실패 분류는 운영 데이터를 본 뒤 후속 change에서 도입하는 것이 합리적.
- 5회 한도와 백오프 수식 자체는 `scheduler-frontier-table`/`scheduler-retry-backoff` 책임. Harvester는 "실패 시 카운터를 올리고 백오프 시각을 갱신한다"는 행위만 책임진다.

### Decision 5: 사이트 단위 종료 통계 → 워커 단위 누적 통계로 대체

**선택**: 기존 `HarvestStats{NodesProcessed, PinsCreated, Deduped, Failed}`는 "한 사이트 BFS 종료" 시점의 누적이었다. consumer 루프에는 종료 시점이 없으므로(데몬), 통계는 워커 프로세스 라이프사이클 동안 누적되어 메트릭/로그로 노출된다.

**근거**: BFS 모델이 사라지면 "Run() 종료 시점에 통계 반환"이라는 시그니처가 어색해진다. metrics는 별도 채널.

**Trade-off**: CLI에서 "한 번 돌려서 결과 보기"가 자연스럽지 않다. `harvester-worker-budget` change가 "N개 처리 후 종료" 같은 mode를 정의할 때 다시 표현 가능.

## Risks / Trade-offs

- **[Risk] Pioneer가 enqueue를 충분히 채우지 못하면 Harvester 워커가 idle**: Harvester는 자체 enqueue를 하지 않는다 (그래프 노드 그대로). → Mitigation: `Dequeue`가 비어 있을 때의 polling/blocking 동작은 `URLScheduler` 계약에 위임. Harvester는 빈 결과 시 짧게 sleep 후 재시도.
- **[Risk] `SetStatus` 직후 워커 크래시 → frontier는 갱신됐지만 메트릭 누락**: 가벼운 누락이며 row 자체는 일관됨. → Mitigation: 영구 크래시가 아니면 다음 사이클 메트릭에 반영. 정확한 회계가 필요하면 별도 audit log.
- **[Risk] 동일 row가 두 워커에 동시 dequeue될 가능성**: scheduler 계약이 깨지면 동일 Pin 중복 시도. → Mitigation: 본 spec은 scheduler 계약을 신뢰한다고 명시. 실제 락 정확성은 `scheduler-claim-api` 책임.
- **[Trade-off] 매 iteration DB 왕복**: 위 Decision 1 참조. 워커 수 × QPS 견적이 인덱스 스캔 한도 안임을 운영 단계에서 모니터링.
- **[Trade-off] 사이트별 진행률을 코드 내부 변수로 알 수 없음**: frontier 쿼리로 대체. 본 spec 범위 외.

## Migration Plan

1. (선결) `scheduler-frontier-table` 적용되어 `bot_frontier` 존재.
2. (선결) `scheduler-claim-api` 적용되어 `URLScheduler` 인터페이스와 Harvester용 `Dequeue` 동작이 정의됨.
3. 신규 Harvester `Run(ctx)` 구현: 본 spec의 ADDED requirement를 만족하는 consumer 루프.
4. 기존 `harvestBFS`, `BFSQueue` 의존, `nodeMap`, `findRootNode` 경로 제거.
5. CLI 진입점은 유지하되 인자 의미 변경: "사이트 ID 1건 BFS"가 아니라 "consumer 워커 1개 시작" (worker exit 정책은 후속 change).
6. **Rollback**: 본 change 자체는 frontier가 채워져 있어야 의미가 있다. 롤백이 필요하면 신규 Harvester를 비활성화하고 기존 `harvestBFS`를 임시로 다시 호출 가능 (단, 인메모리 BFS는 frontier와 분리되어 있어 데이터 일관성에 영향 없음).

## Open Questions

- `Dequeue`가 비었을 때의 정확한 동작(blocking vs polling) — `scheduler-claim-api` spec에서 결정.
- 워커 종료 조건(`harvester-worker-budget`)이 정의되기 전까지 CLI는 SIGTERM 외 종료 수단이 없음. 운영상 허용 가능한지 확인 필요.
- 사이트 단위 메트릭(현재 `HarvestStats`로 노출)이 외부에서 소비되고 있는지 확인 — 있다면 frontier 쿼리 기반 대체 메트릭 정의가 별도 작업으로 필요할 수 있음.
