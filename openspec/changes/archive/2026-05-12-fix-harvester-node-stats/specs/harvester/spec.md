## ADDED Requirements

### Requirement: 노드 단위 통계는 워커 lifetime 동안 관찰 가능하다

Harvester 워커 프로세스는 "Harvester 노드 단위 통계 정의" Requirement가 정의한 5개 카운터(`PinsCreated`, `Deduped`, `Skipped`, `Failed`, `AdapterFallback`) 각각의 누적 값을 워커 lifetime 동안 외부에서 읽을 수 있는 상태로 보유해야 한다(SHALL). 카운터 값은 워커 종료 시 폐기되며 워커 간 공유되지 않는다(SHALL NOT, "Dequeue 카운터는 워커 간 공유 상태가 아니다" Requirement와 동일 정책).

#### Scenario: 노드 처리 직후 카운터 변화가 관찰된다
- **WHEN** 어떤 노드 처리가 끝난 직후 카운터 값을 읽을 때
- **THEN** 해당 노드의 분류에 해당하는 주 카테고리 카운터가 처리 전 대비 1 증가한 값으로 관찰되며, `AdapterFallback`은 어댑터 실패가 발생한 경우에 한해 함께 1 증가한 값으로 관찰된다

#### Scenario: 다중 노드 처리 시 카운터가 누적된다
- **WHEN** 같은 워커가 N개의 노드를 처리한 직후 카운터 값을 읽을 때
- **THEN** 5개 카운터의 합(주 카테고리 4개의 합 + `AdapterFallback`은 별도)이 노드 수와 어댑터 fallback 발생 횟수에 정합하게 누적되어 있다 (주 카테고리 4개 합 = 처리 노드 수)

#### Scenario: 카운터는 워커 종료 시 폐기된다
- **WHEN** Harvester 워커 프로세스가 종료될 때
- **THEN** in-memory 카운터 값은 어떤 외부 저장소(DB/Redis/파일)에도 보관되지 않으며, 새 워커는 모든 카운터를 0에서 시작한다
