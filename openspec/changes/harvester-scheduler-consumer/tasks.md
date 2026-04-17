## 1. 사전 검증 (Prerequisites)

- [ ] 1.1 `scheduler-frontier-table` change가 적용되어 `bot_frontier` 테이블과 Harvester partial index(`pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now()`)가 존재하는지 확인
- [ ] 1.2 `scheduler-claim-api` change에서 정의되는 `URLScheduler` 인터페이스(`Enqueue`, `Dequeue`, `SetStatus`)가 사용 가능한지 확인 (없다면 본 change 적용 보류)
- [ ] 1.3 기존 `apps/api/internal/bot/harvester.go`의 외부 사용처(CLI, 테스트, 잡 스케줄러) 인벤토리 작성

## 2. 새 Harvester consumer 루프 구현

- [ ] 2.1 `apps/api/internal/bot/harvester.go`에 새 `Run(ctx context.Context) error` 시그니처 추가 (사이트 ID 인자 제거)
- [ ] 2.2 메인 루프 골격: `for { row, err := scheduler.Dequeue(ctx); ... }` 작성, ctx 취소 시 안전 종료
- [ ] 2.3 `Dequeue` 빈 결과 처리: 짧은 백오프(예: 100ms~1s) 후 재호출, 자체 큐/그래프 순회 금지
- [ ] 2.4 fetch 단계: 기존 `fetchHTMLShared(ctx, row.Url)` 또는 동등 함수 호출, 실패 시 후속 단계 스킵
- [ ] 2.5 ParseDocument 단계: 기존 `executor.Execute(ctx, scriptCode, html, fetchURL)` 호출, 실패/0건 시 후속 단계 스킵
- [ ] 2.6 Index 단계: 기존 `pipeline.Process(ctx, items)` 호출
- [ ] 2.7 성공 시 `scheduler.SetStatus`(또는 동등 호출)로 frontier row의 `pin_id` 갱신 (대표 Pin ID 1건)
- [ ] 2.8 실패 시 `scheduler.SetStatus` 또는 동등 호출로 `harvest_error_count++` 및 `next_harvest_at` 백오프 갱신, `pin_id`는 NULL 유지

## 3. 인메모리 상태 제거

- [ ] 3.1 `harvestBFS`, `BFSQueue` 사용 경로 삭제
- [ ] 3.2 `visited map[uuid.UUID]bool`, `nodeMap map[uuid.UUID]db.BotGraphNode` 삭제
- [ ] 3.3 `findRootNode`, `sortNodesByPriority` 등 사이트 단위 BFS 헬퍼 삭제 (또는 unexport 후 미사용 표시)
- [ ] 3.4 사이트 ID 기반 사전 적재 코드(`graphRepo.ListNodesBySite`) Harvester 경로에서 제거
- [ ] 3.5 정적 점검: 새 Harvester 구조체 필드에 큐/visited/nodeMap이 존재하지 않음을 확인

## 4. 다중 워커 안전성 위임

- [ ] 4.1 Harvester 자체 advisory lock/분산 락/조정 채널이 도입되지 않았는지 코드 리뷰로 확인
- [ ] 4.2 동일 row 중복 dequeue 정확성은 `URLScheduler.Dequeue` 계약(scheduler-claim-api의 FOR UPDATE SKIP LOCKED)에 위임함을 주석으로 명시

## 5. 사이트 경계 무관 동작

- [ ] 5.1 한 워커가 여러 사이트 row를 임의 순서로 처리할 수 있는지 통합 테스트 작성 (사이트 A/B row 혼재 시 우선순위대로 dequeue)
- [ ] 5.2 사이트가 바뀔 때 별도 셋업/티어다운이 호출되지 않음을 확인

## 6. CLI / 진입점 정리

- [ ] 6.1 기존 Harvester CLI 진입점에서 "사이트 ID 1건 → BFS 1회" 인터페이스 제거 (worker exit 정책은 후속 change 책임)
- [ ] 6.2 임시 종료 수단으로 SIGTERM 처리만 보장 (예산 기반 종료는 `harvester-worker-budget`)
- [ ] 6.3 사이트 단위 종료 통계 출력 제거 (워커 메트릭은 후속 작업)

## 7. 테스트

- [ ] 7.1 `Dequeue → fetch → ParseDocument → Index → SetStatus(pin_id)` 정상 흐름 단위 테스트 (Harvester `URLScheduler` mock 사용)
- [ ] 7.2 fetch 실패 → ParseDocument/Index 미호출 + `harvest_error_count++` 검증
- [ ] 7.3 ParseDocument 실패 → Index 미호출 + `harvest_error_count++` 검증
- [ ] 7.4 Index 전체 실패 → `harvest_error_count++`, `pin_id` NULL 유지 검증
- [ ] 7.5 모든 항목 중복 스킵 시 frontier row가 partial index에서 제외되는지 검증 (대표 pin_id 또는 종료 마커)
- [ ] 7.6 빈 큐일 때 polling 동작과 ctx 취소 시 안전 종료 검증
- [ ] 7.7 통합 테스트: 워커 2개 동시 실행 시 동일 row가 한 번만 처리되는지 검증 (scheduler-claim-api와 함께)

## 8. bot spec REMOVED 정합성 확인

- [ ] 8.1 기존 bot spec의 "Harvester 실행 완료 시 전체 통계를 집계한다" 의존 코드/문서 제거 확인
- [ ] 8.2 기존 bot spec의 "Harvester CLI가 실제 모드를 지원한다" 의존 코드/문서 정리 (CLI는 진입점만 유지, BFS 트리거 의미 제거)
- [ ] 8.3 `openspec validate harvester-scheduler-consumer --strict` 통과 확인

## 9. 문서 업데이트

- [ ] 9.1 `docs/architecture.md`에 Harvester가 URLScheduler consumer임을 반영
- [ ] 9.2 `CLAUDE.md`의 Harvester 설명을 "BFS 순회" → "scheduler consumer"로 업데이트 (필요 시)
- [ ] 9.3 본 change archive 시 `openspec/specs/bot/spec.md`에서 REMOVED requirement 2건이 실제로 제거되는지 확인
