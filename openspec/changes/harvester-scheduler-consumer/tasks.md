## 1. Prerequisites

본 change의 tasks는 아래 change가 먼저 적용되어야 의미가 있다. 적용 전에는 본 change 구현을 보류한다.

- [ ] 1.1 `scheduler-frontier-table` 적용: `harvester_frontier`, `harvester_frontier_pins` 테이블과 Harvester partial index(`WHERE harvested_at IS NULL AND harvest_error_count < 5 ORDER BY score DESC, next_harvest_at ASC`) 존재 확인
- [ ] 1.2 `scheduler-claim-api` 적용: `scheduler.Dequeue(QueueType)`, `scheduler.SetStatus(key, status, pinIDs)`, `scheduler.RecordHarvestError(key, errorKind)` 사용 가능 확인
- [ ] 1.3 `harvester-snapshot-first-fetch` 적용: `fetchSnapshotOrLive(ctx, url)` 헬퍼(또는 동등 함수) 사용 가능 확인
- [ ] 1.4 `harvester-pin-document` 적용: `harvestPipeline.Process(ctx, html, url) → PinDocument` 스키마와 `Pinnable` 판정 규칙 확정 확인
- [ ] 1.5 기존 `apps/api/internal/bot/harvester.go`의 외부 사용처(CLI, 테스트, 잡 스케줄러) 인벤토리 작성

## 2. 새 Harvester consumer 루프 구현

- [ ] 2.1 `apps/api/internal/bot/harvester.go`에 새 `Run(ctx context.Context) error` 시그니처 추가 (사이트 ID 인자 제거)
- [ ] 2.2 메인 루프 골격 작성: `for { url, err := scheduler.Dequeue(scheduler.QueueHarvester); ... }` — ctx 취소 시 안전 종료
- [ ] 2.3 **자체 sleep/backoff 금지 확인**: 빈 큐 polling은 `Dequeue` 내부 책임(scheduler-claim-api). consumer 루프에서 `time.Sleep`/backoff 호출이 없는지 정적 점검
- [ ] 2.4 fetch 단계: `fetchSnapshotOrLive(ctx, url)` 호출로 snapshot 우선, miss 시 HTTP live fetch 폴백 (실제 구현은 `harvester-snapshot-first-fetch`)
- [ ] 2.5 fetch 실패 시 `errorKind` 분류: `"http_4xx" | "http_5xx" | "network" | "timeout"` 중 하나로 분류하고, `SetStatus(url, "harvest_failed", nil)` + `RecordHarvestError(url, errorKind)` 둘 다 호출
- [ ] 2.6 파싱 단계: `harvestPipeline.Process(ctx, html, url) → PinDocument` 호출. 실패 시 `errorKind = "parse"`로 위 2.5 동일 실패 처리
- [ ] 2.7 `PinDocument.Pinnable == false` 시: `SetStatus(url, "harvested", nil)` 호출하고 다음 iteration (매핑 없이 완료 표기)
- [ ] 2.8 `Pinnable == true` 시: `createPins(ctx, pinDocument) → pinIDs []int64` 호출. 실패 시 `errorKind = "pin_create"`로 실패 처리
- [ ] 2.9 성공 시 `scheduler.SetStatus(url, "harvested", pinIDs)` 단일 호출 — `harvested_at` UPDATE와 `harvester_frontier_pins` 일괄 INSERT가 한 트랜잭션에서 수행됨(scheduler 책임)
- [ ] 2.10 다중 Pin 처리 검증: `pinIDs`가 `[]int64` 슬라이스로 전달되며, 길이 0/1/N 모두에서 올바른 동작 확인

## 3. errorKind 분류 규칙

- [ ] 3.1 fetch 실패 분류 헬퍼 작성:
  - HTTP 4xx → `"http_4xx"`
  - HTTP 5xx → `"http_5xx"`
  - DNS/connect/TLS 오류 → `"network"`
  - 타임아웃 → `"timeout"`
- [ ] 3.2 파싱 실패는 일괄 `"parse"`로 분류 (스크립트 구문/런타임/타임아웃 세분화는 후속 change)
- [ ] 3.3 Pin INSERT 실패는 일괄 `"pin_create"`로 분류
- [ ] 3.4 분류 헬퍼 단위 테스트: 각 상황별 반환값 검증
- [ ] 3.5 `http_4xx` 전달 시 `RecordHarvestError`가 즉시 `harvest_error_count = 5`로 설정하여 partial index에서 제외됨을 통합 테스트로 확인 (이 동작 자체는 `scheduler-claim-api`)

## 4. 인메모리 상태 제거 (BFS → scheduler 기반 교체)

- [ ] 4.1 `harvestBFS`, `BFSQueue` 사용 경로 삭제
- [ ] 4.2 `visited map[uuid.UUID]bool`, `nodeMap map[uuid.UUID]db.BotGraphNode` 삭제
- [ ] 4.3 `findRootNode`, `sortNodesByPriority` 등 사이트 단위 BFS 헬퍼 삭제 (또는 unexport 후 미사용 표시)
- [ ] 4.4 사이트 ID 기반 사전 적재 코드(`graphRepo.ListNodesBySite`)를 Harvester 경로에서 제거
- [ ] 4.5 정적 점검: 새 Harvester 구조체 필드에 큐/visited/nodeMap이 존재하지 않음 확인
- [ ] 4.6 정적 점검: Harvester에 advisory lock/분산 락/워커 간 조정 채널이 없고, 중복 dequeue 방지는 전적으로 `scheduler.Dequeue` 계약에 위임되었음을 주석으로 명시

## 5. 재harvest 방지 확인

- [ ] 5.1 Pioneer가 동일 URL을 재크롤하여 UPSERT해도 `WHERE harvester_frontier.harvested_at IS NULL` 가드에 막혀 partial index에 다시 나타나지 않음을 통합 테스트로 확인 (스키마 가드는 `scheduler-frontier-table` 책임)
- [ ] 5.2 Harvester 코드 내에서 "이미 harvested인지" 직접 체크하는 경로가 없음 확인

## 6. 사이트 경계 무관 동작

- [ ] 6.1 한 워커가 여러 host의 row를 임의 순서로 처리할 수 있는지 통합 테스트 작성 (host A/B row 혼재 시 score 우선순위대로 dequeue)
- [ ] 6.2 host가 바뀔 때 별도 셋업/티어다운이 호출되지 않음을 확인

## 7. CLI / 진입점 정리

- [ ] 7.1 기존 Harvester CLI 진입점에서 "사이트 ID 1건 → BFS 1회" 인터페이스 제거 (worker exit 정책은 `harvester-worker-budget`)
- [ ] 7.2 임시 종료 수단으로 SIGTERM 처리만 보장 (예산 기반 종료는 `harvester-worker-budget`)
- [ ] 7.3 사이트 단위 종료 통계 출력 제거 (워커 메트릭은 후속 작업)

## 8. 테스트

- [ ] 8.1 정상 흐름 단위 테스트: `Dequeue → fetchSnapshotOrLive → harvestPipeline.Process → createPins → SetStatus(url, "harvested", pinIDs)` (scheduler/pipeline mock 사용)
- [ ] 8.2 snapshot hit 경로 vs live fetch 폴백 경로 모두 커버 (`harvester-snapshot-first-fetch` mock)
- [ ] 8.3 fetch 실패 단위 테스트: 4xx/5xx/network/timeout 각각에 대해 `SetStatus("harvest_failed", nil)` + `RecordHarvestError(url, errorKind)` 호출 검증
- [ ] 8.4 `PinDocument.Pinnable == false` → `SetStatus(url, "harvested", nil)` 호출 검증 (매핑 없음)
- [ ] 8.5 다중 Pin 생성(N>=2) → `SetStatus(url, "harvested", pinIDs)`로 한 번에 전달되는지 검증
- [ ] 8.6 `createPins` 실패 → `errorKind = "pin_create"`로 실패 처리 검증
- [ ] 8.7 파싱 실패 → `errorKind = "parse"`로 실패 처리 검증
- [ ] 8.8 ctx 취소 시 현재 iteration 완료 후 안전 종료 검증
- [ ] 8.9 통합 테스트: 워커 2개 동시 실행 시 동일 URL이 한 번만 처리됨 (scheduler-claim-api의 `FOR UPDATE SKIP LOCKED`에 의존)
- [ ] 8.10 통합 테스트: Pioneer 재크롤 후 Harvester가 동일 URL을 다시 처리하지 않음 (UPSERT 가드 확인)

## 9. bot spec REMOVED delta 정합성

- [ ] 9.1 본 change의 `specs/bot/spec.md` REMOVED delta에 포함된 2개 requirement가 실제로 기존 bot spec과 매칭됨 확인:
  - "Harvester 실행 완료 시 전체 통계를 집계한다"
  - "Harvester CLI가 실제 모드를 지원한다"
- [ ] 9.2 `Harvester가 실제 HTML을 가져온다` requirement는 본 change에서 **삭제하지 않음**: HTML fetch 자체는 `harvester-snapshot-first-fetch`가 재정의하므로 본 change의 책임 범위 외
- [ ] 9.3 위 REMOVED 대상 requirement를 참조하던 문서/코드 경로 인벤토리 작성 및 업데이트 계획
- [ ] 9.4 `openspec validate harvester-scheduler-consumer --strict` 통과 확인

## 10. 문서 업데이트

- [ ] 10.1 `docs/architecture.md`에 Harvester가 `scheduler.Dequeue(QueueHarvester)` consumer임을 반영 (BFS 서술 제거)
- [ ] 10.2 `CLAUDE.md`의 Harvester 설명을 "BFS 순회" → "scheduler consumer"로 업데이트 (필요 시)
- [ ] 10.3 본 change archive 시 `openspec/specs/bot/spec.md`에서 REMOVED requirement 2건이 실제로 제거되는지 확인
