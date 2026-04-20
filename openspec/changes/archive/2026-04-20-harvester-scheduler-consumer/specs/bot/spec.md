## REMOVED Requirements

### Requirement: Harvester 실행 완료 시 전체 통계를 집계한다
**Reason**: 본 requirement는 Harvester가 "한 사이트의 모든 노드를 BFS로 순회한 뒤 종료"하는 단일 프로세스 모델을 전제로 한다. 새 `harvester` capability에서 Harvester는 종료 시점이 정의되지 않은 데몬형 `URLScheduler` consumer로 재정의되며, "Run() 종료 시점에 누적 통계 반환"이라는 함수 시그니처가 더 이상 의미를 갖지 않는다. 사이트 단위 누적 통계가 필요하면 `bot_frontier` 쿼리(예: `SELECT count(*) FROM bot_frontier WHERE host = ? AND pin_id IS NOT NULL`)로 대체한다.

**Migration**: 본 change(`harvester-scheduler-consumer`)의 새 `harvester` spec이 메인 루프 동작과 frontier row 갱신 의미를 정의한다. 워커 단위 메트릭(처리 건수/실패 건수 등)을 외부로 노출할 필요가 있다면 후속 change에서 metrics 채널을 별도로 정의한다. 사이트 단위 진행률은 frontier 집계 쿼리로 대체한다.

---

### Requirement: Harvester CLI가 실제 모드를 지원한다
**Reason**: 기존 정의는 "사이트 ID 1건을 인자로 받아 해당 사이트의 그래프를 BFS로 한 번 순회한다"는 실행 모델을 전제로 한다. 새 `harvester` capability에서 Harvester는 사이트 경계와 무관하게 frontier에서 dequeue된 임의 row를 처리하는 데몬으로 재정의되므로, "사이트 ID 인자 + 단발 실행 + 종료 시 통계 출력"이라는 CLI 의미가 더 이상 적용되지 않는다.

**Migration**: 새 `harvester` spec의 메인 루프 정의가 데몬 동작을 규정한다. CLI 진입점 자체는 유지하되, 인자 의미와 종료 조건은 후속 change `harvester-worker-budget`에서 별도로 정의한다(예: 최대 N건 처리 후 종료, 시간 예산 도달 시 종료). 본 change는 CLI 시그니처를 새로 정의하지 않는다.
