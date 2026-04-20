## 1. Prerequisites

- [x] 1.1 `harvester-scheduler-consumer` change가 적용되어 `harvester` capability와 consumer 루프 정의가 존재함을 확인한다.
- [x] 1.2 `harvester-snapshot-first-fetch` change가 적용되어 `Fetcher`/`CompositeFetcher` 의미론과 스냅샷 키 내부 계산 규약이 존재함을 확인한다.
- [x] 1.3 `scheduler.Dequeue(QueueHarvester)`의 기존 반환 형태(`url string, err error`)가 그대로 유지됨을 확인한다. 본 change는 이 시그니처를 변경하지 않는다.

## 2. Spec delta 교차 검토

- [x] 2.1 본 change `specs/harvester/spec.md`의 ADDED 5개 requirement가 `harvester-scheduler-consumer/specs/harvester/spec.md`의 기존 requirement와 충돌하지 않는지(동일 이름 없음, 모순 없음) 두 spec을 병렬로 비교한다.
- [x] 2.2 "fetch 단 errorKind는 4종으로 한정된다" requirement가 `harvester-scheduler-consumer` "실패 시 SetStatus + RecordHarvestError를 둘 다 호출한다" requirement의 6종 errorKind 집합과 정합함을 확인한다(진입점 반환 4종 ⊆ RecordHarvestError 6종).
- [x] 2.3 "스냅샷 내부 실패 종류는 consumer의 `errorKind`에 노출되지 않는다" requirement가 `harvester-snapshot-first-fetch` "실패 유형은 로그로만 구분된다" Scenario와 의미적으로 중복되지 않고 경계(capability)만 재확인하는 형태인지 검토한다.
- [x] 2.4 본 change의 어떤 requirement도 `harvester-scheduler-consumer`의 `SetStatus`/`RecordHarvestError` 이중 호출 규약을 변경하거나 축소하지 않음을 확인한다.

## 3. 수용 기준(Acceptance)

- [x] 3.1 `cd <repo> && openspec validate harvester-snapshot-first-fetchconsumer --strict`가 통과한다.
- [x] 3.2 `openspec show harvester-snapshot-first-fetchconsumer --json --deltas-only`의 결과가 `specs/harvester/spec.md`에 대해 ADDED 5건만 보고하며(각 delta의 `operation: "ADDED"`, `spec: "harvester"`), MODIFIED/REMOVED/RENAMED가 발생하지 않는다.
- [x] 3.3 archive 전 정적 검사: 본 spec의 5개 ADDED 제목과 `harvester-scheduler-consumer/specs/harvester/spec.md`의 기존 requirement 제목 사이에 중복·모순이 없음을 확인한다(공존 가능성 사전 검증). archive 시점에는 `openspec show harvester --json`의 requirement 목록에 본 spec의 5개 제목이 모두 포함되는지 다시 확인한다. (`harvester-snapshot-first-fetch`는 `bot` capability 대상이므로 본 공존 검사에서 제외한다.)
- [x] 3.4 후속 구현 change가 본 spec의 5개 requirement(진입점 경유, 입력 `(ctx, url)`, 반환 3-tuple, 4종 errorKind, 스냅샷 내부 실패 은닉)를 그대로 수용 기준(assertion)으로 가져갈 수 있는 형태로 작성되어 있다.
