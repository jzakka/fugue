## REMOVED Requirements

### Requirement: BFS로 사이트를 탐색한다 (Pioneer)
**Reason**: "BFS로 사이트를 탐색"한다는 표현은 Pioneer가 사이트 루트를 시작점으로 사이트 내부를 너비 우선으로 하강한다는 **단일 프로세스/인메모리 세션** 모델을 전제한다. Pioneer는 이제 `URLScheduler`의 얇은 consumer로 재정의되며, 다음 URL은 frontier에서 `Dequeue`로 받고 링크를 `Enqueue`로 되돌려놓는다. 큐 자료구조와 순회 전략은 `URLScheduler` 구현체와 frontier의 우선순위 스코어가 결정하므로, Pioneer 자체의 요구사항으로 "BFS"를 고정하는 것은 타당하지 않다.

기존 6개 scenario의 처리 내역은 다음과 같다.

| Scenario | 처리 | 근거 |
|----------|------|------|
| DOM 기반 링크 추출 | **보존**(무영향) | `bot` spec의 독립 requirement "DOM ancestor selector를 포함하여 링크를 추출한다"가 그대로 담당 |
| 필터 체인을 통한 링크 필터링 | **보존**(무영향) | `bot` spec의 독립 requirement "필터 체인이 순서대로 필터를 적용한다"가 그대로 담당 (필터 정책 세부는 `pioneer-link-filter-policy`에서 이관·확장) |
| 복합 우선순위 계산 | **이관**(후속) | `pioneer-link-filter-policy`에서 score 보정 책임으로 재정의(선행 change) |
| 이미 방문한 링크에 대한 엣지 생성 | **영구 삭제** | 새 모델에는 "사이트 세션" 개념이 없고 graph edge를 유지하지 않는다. DECISIONS.md §2의 frontier-only 모델에 따라 의도적 폐기 |
| 부모 관계 추적 및 엣지 생성 | **영구 삭제** | 동일 근거(의도적 폐기). 노드 간 부모-자식 관계는 URLScheduler 모델의 책임이 아니다 |
| 최대 노드 수 제한 준수 | **부분 대체** | 사이트 단위 quota 자체는 폐기된다. 대신 워커 단위 예산은 `pioneer-worker-budget`(성공 Dequeue 100회 후 종료)이, host 단위 속도 제한은 `scheduler-host-token-bucket`이 각각 다른 관점에서 담당 |

**Migration**: 본 change가 정의하는 `pioneer` capability의 "Pioneer 메인 루프는 Dequeue → fetch → snapshot → parse → filter → Enqueue(pioneer) + EnqueueHarvester → SetStatus 반복이다" 및 관련 requirement 집합이 Pioneer의 탐색 모델을 대체한다. "DOM 기반 링크 추출"과 "필터 체인을 통한 링크 필터링"은 `bot` spec의 독립 requirement가 계속 담당한다. "복합 우선순위 계산"은 `pioneer-link-filter-policy`로 이관된다. "최대 노드 수 제한"은 사이트 단위 quota가 의도적으로 폐기되고 워커 단위 예산(`pioneer-worker-budget`)/host 단위 속도 제한(`scheduler-host-token-bucket`)으로 관점이 전환된다. "이미 방문한 링크에 대한 엣지 생성"과 "부모 관계 추적 및 엣지 생성"은 graph edge 유지 자체를 폐기하므로 복원되지 않는다(의도적 trade-off).
